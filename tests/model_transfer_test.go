package tests

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"

	grpcregistry "github.com/kennethnrk/edgernetes-ai/internal/control-plane/api/grpc/registry"
	registrycontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/registry"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"

	deploypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/deploy"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024 // 1MB in-memory buffer for the bufconn listener

// newModelTransferClient spins up a real in-process gRPC server backed by
// the ModelTransferService and returns a client connected to it.
// The server and connection are torn down when the test ends.
func newModelTransferClient(t *testing.T, s *store.Store) deploypb.ModelTransferServiceClient {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	deploypb.RegisterModelTransferServiceServer(srv, grpcregistry.NewModelTransferServer(s))

	go func() {
		if err := srv.Serve(lis); err != nil && err != grpc.ErrServerStopped {
			t.Logf("grpc server error: %v", err)
		}
	}()

	t.Cleanup(func() {
		srv.GracefulStop()
		lis.Close()
	})

	conn, err := grpc.NewClient(
		"passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient() error = %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	return deploypb.NewModelTransferServiceClient(conn)
}

// writeTempModelFile writes deterministic bytes to a temp file, returning its path and sha256 hash.
func writeTempModelFile(t *testing.T, sizeBytes int) (path string, hash string) {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "model-*.bin")
	if err != nil {
		t.Fatalf("os.CreateTemp() error = %v", err)
	}
	defer f.Close()

	// Fill with a simple repeating pattern so the content is deterministic.
	chunk := make([]byte, 1024)
	for i := range chunk {
		chunk[i] = byte(i % 256)
	}

	h := sha256.New()
	written := 0
	for written < sizeBytes {
		n := sizeBytes - written
		if n > len(chunk) {
			n = len(chunk)
		}
		if _, err := f.Write(chunk[:n]); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
		h.Write(chunk[:n])
		written += n
	}

	return f.Name(), fmt.Sprintf("%x", h.Sum(nil))
}

// receiveAllChunks drains a DownloadModel stream and returns the reassembled bytes.
func receiveAllChunks(t *testing.T, stream deploypb.ModelTransferService_DownloadModelClient) []byte {
	t.Helper()
	var received []byte
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream.Recv() unexpected error: %v", err)
		}
		received = append(received, chunk.GetChunkData()...)
	}
	return received
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestModelTransfer_FullFile verifies that a model file is transferred
// completely and the reassembled bytes match the original file on disk.
func TestModelTransfer_FullFile(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Create a model file (~3MB to ensure more than one 2MB chunk is used).
	filePath, _ := writeTempModelFile(t, 3*1024*1024)

	modelID := "model-full"
	if err := registrycontroller.RegisterModel(s, modelID, store.ModelInfo{
		Name:      "full-model",
		FilePath:  filePath,
		ModelSize: 3 * 1024 * 1024,
	}); err != nil {
		t.Fatalf("RegisterModel() error = %v", err)
	}

	client := newModelTransferClient(t, s)
	stream, err := client.DownloadModel(context.Background(), &deploypb.ModelDownloadRequest{
		ModelId: modelID,
	})
	if err != nil {
		t.Fatalf("DownloadModel() error = %v", err)
	}

	received := receiveAllChunks(t, stream)

	original, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("os.ReadFile() error = %v", err)
	}

	if len(received) != len(original) {
		t.Fatalf("received %d bytes, want %d", len(received), len(original))
	}
	for i := range original {
		if received[i] != original[i] {
			t.Fatalf("byte mismatch at offset %d: got %02x, want %02x", i, received[i], original[i])
		}
	}
}

// TestModelTransfer_ChunkOffsetIsMonotonicallyIncreasing verifies that each
// chunk's offset advances sequentially, which agents rely on for integrity checks.
func TestModelTransfer_ChunkOffsetIsMonotonicallyIncreasing(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	filePath, _ := writeTempModelFile(t, 5*1024*1024)

	modelID := "model-offsets"
	if err := registrycontroller.RegisterModel(s, modelID, store.ModelInfo{
		Name:     "offset-model",
		FilePath: filePath,
	}); err != nil {
		t.Fatalf("RegisterModel() error = %v", err)
	}

	client := newModelTransferClient(t, s)
	stream, err := client.DownloadModel(context.Background(), &deploypb.ModelDownloadRequest{
		ModelId: modelID,
	})
	if err != nil {
		t.Fatalf("DownloadModel() error = %v", err)
	}

	var lastOffset int64 = -1
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream.Recv() error = %v", err)
		}
		if chunk.GetChunkOffset() <= lastOffset {
			t.Fatalf("chunk offset %d not greater than previous %d", chunk.GetChunkOffset(), lastOffset)
		}
		lastOffset = chunk.GetChunkOffset()
	}

	if lastOffset < 0 {
		t.Fatal("no chunks were received")
	}
}

// TestModelTransfer_ResumeByteOffset verifies that when an agent provides a
// non-zero resume_byte_offset, only the remaining bytes are sent.
func TestModelTransfer_ResumeByteOffset(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	filePath, _ := writeTempModelFile(t, 4*1024*1024)

	modelID := "model-resume"
	if err := registrycontroller.RegisterModel(s, modelID, store.ModelInfo{
		Name:     "resume-model",
		FilePath: filePath,
	}); err != nil {
		t.Fatalf("RegisterModel() error = %v", err)
	}

	// Simulate the agent having already received the first 2MB.
	const resumeOffset = 2 * 1024 * 1024

	client := newModelTransferClient(t, s)
	stream, err := client.DownloadModel(context.Background(), &deploypb.ModelDownloadRequest{
		ModelId:          modelID,
		ResumeByteOffset: resumeOffset,
	})
	if err != nil {
		t.Fatalf("DownloadModel() error = %v", err)
	}

	received := receiveAllChunks(t, stream)

	// The first chunk's offset should equal resumeOffset, confirming the seek worked.
	// Also confirm the number of bytes received matches the remaining file size.
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("os.Stat() error = %v", err)
	}
	expectedBytes := int(fileInfo.Size()) - resumeOffset

	if len(received) != expectedBytes {
		t.Fatalf("received %d bytes after resume, want %d (file size %d - offset %d)",
			len(received), expectedBytes, fileInfo.Size(), resumeOffset)
	}

	// Verify the reassembled tail matches the original file's tail.
	original, _ := os.ReadFile(filePath)
	tail := original[resumeOffset:]
	for i := range tail {
		if received[i] != tail[i] {
			t.Fatalf("resumed byte mismatch at offset %d: got %02x, want %02x", resumeOffset+i, received[i], tail[i])
		}
	}
}

// TestModelTransfer_SHA256Integrity verifies that the bytes streamed by the
// server, when hashed, match the SHA256 of the original file on disk.
// This mirrors exactly what the agent will do before committing the model file.
func TestModelTransfer_SHA256Integrity(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	filePath, expectedHash := writeTempModelFile(t, 2*1024*1024+512)

	modelID := "model-sha256"
	if err := registrycontroller.RegisterModel(s, modelID, store.ModelInfo{
		Name:     "sha256-model",
		FilePath: filePath,
	}); err != nil {
		t.Fatalf("RegisterModel() error = %v", err)
	}

	client := newModelTransferClient(t, s)
	stream, err := client.DownloadModel(context.Background(), &deploypb.ModelDownloadRequest{
		ModelId: modelID,
	})
	if err != nil {
		t.Fatalf("DownloadModel() error = %v", err)
	}

	hasher := sha256.New()
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream.Recv() error = %v", err)
		}
		hasher.Write(chunk.GetChunkData())
	}

	actualHash := fmt.Sprintf("%x", hasher.Sum(nil))
	if actualHash != expectedHash {
		t.Fatalf("SHA256 mismatch: got %s, want %s", actualHash, expectedHash)
	}
}

// TestModelTransfer_EmptyModelID verifies that the server returns an
// InvalidArgument error when no model_id is provided.
func TestModelTransfer_EmptyModelID(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	client := newModelTransferClient(t, s)
	stream, err := client.DownloadModel(context.Background(), &deploypb.ModelDownloadRequest{
		ModelId: "",
	})
	if err != nil {
		t.Fatalf("DownloadModel() unexpected dial error = %v", err)
	}

	_, err = stream.Recv()
	if err == nil {
		t.Fatal("expected error for empty model_id, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected code %v, got %v", codes.InvalidArgument, st.Code())
	}
}

// TestModelTransfer_ModelNotInRegistry verifies that requesting a model that
// does not exist in the store returns a NotFound error.
func TestModelTransfer_ModelNotInRegistry(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	client := newModelTransferClient(t, s)
	stream, err := client.DownloadModel(context.Background(), &deploypb.ModelDownloadRequest{
		ModelId: "nonexistent-model",
	})
	if err != nil {
		t.Fatalf("DownloadModel() unexpected dial error = %v", err)
	}

	_, err = stream.Recv()
	if err == nil {
		t.Fatal("expected error for unknown model, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.NotFound {
		t.Fatalf("expected code %v, got %v", codes.NotFound, st.Code())
	}
}

// TestModelTransfer_FileDeletedAfterRegistration verifies that if a model's
// file is missing from disk (e.g., manually deleted after registration),
// the server returns an Internal error instead of panicking or blocking.
func TestModelTransfer_FileDeletedAfterRegistration(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Create a file and immediately delete it to simulate a missing file.
	dir := t.TempDir()
	ghostPath := filepath.Join(dir, "ghost-model.bin")
	if err := os.WriteFile(ghostPath, []byte("temporary"), 0644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	modelID := "model-ghost"
	if err := registrycontroller.RegisterModel(s, modelID, store.ModelInfo{
		Name:     "ghost-model",
		FilePath: ghostPath,
	}); err != nil {
		t.Fatalf("RegisterModel() error = %v", err)
	}

	// Delete the file after registration.
	if err := os.Remove(ghostPath); err != nil {
		t.Fatalf("os.Remove() error = %v", err)
	}

	client := newModelTransferClient(t, s)
	stream, err := client.DownloadModel(context.Background(), &deploypb.ModelDownloadRequest{
		ModelId: modelID,
	})
	if err != nil {
		t.Fatalf("DownloadModel() unexpected dial error = %v", err)
	}

	_, err = stream.Recv()
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}

	st, ok := status.FromError(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got: %v", err)
	}
	if st.Code() != codes.Internal {
		t.Fatalf("expected code %v, got %v", codes.Internal, st.Code())
	}
}

// TestModelTransfer_ClientCancellation verifies that the server exits cleanly
// when a client cancels mid-stream rather than blocking or leaking resources.
func TestModelTransfer_ClientCancellation(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	// Use a large file so the stream is definitely in progress when we cancel.
	filePath, _ := writeTempModelFile(t, 10*1024*1024)

	modelID := "model-cancel"
	if err := registrycontroller.RegisterModel(s, modelID, store.ModelInfo{
		Name:     "cancel-model",
		FilePath: filePath,
	}); err != nil {
		t.Fatalf("RegisterModel() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := newModelTransferClient(t, s)
	stream, err := client.DownloadModel(ctx, &deploypb.ModelDownloadRequest{
		ModelId: modelID,
	})
	if err != nil {
		t.Fatalf("DownloadModel() error = %v", err)
	}

	// Receive one chunk to ensure the stream has started, then cancel.
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("first Recv() error = %v", err)
	}
	cancel()

	// Drain remaining messages — we expect a Canceled error.
	for {
		_, err := stream.Recv()
		if err != nil {
			st, ok := status.FromError(err)
			if ok && (st.Code() == codes.Canceled) {
				return // Correct — server respected the cancellation
			}
			if err == io.EOF {
				// Small file finished before cancel took effect, which is also fine.
				return
			}
			t.Fatalf("unexpected error after cancel: %v", err)
		}
	}
}
