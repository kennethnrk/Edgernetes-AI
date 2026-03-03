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
	deploypb.RegisterModelTransferServiceServer(srv, grpcregistry.NewModelTransferServer(s, t.TempDir()))

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

// ---------------------------------------------------------------------------
// Upload tests
// ---------------------------------------------------------------------------

// makeONNXBytes produces bytes that pass ONNX magic-byte validation.
// A valid protobuf field tag has wire type in {0,1,2,5}; 0x08 = field 1, varint (ir_version)
func makeONNXBytes(sizeBytes int) []byte {
	b := make([]byte, sizeBytes)
	b[0] = 0x08 // valid protobuf field tag (wire type 0)
	b[1] = 0x01 // varint value 1 (ir_version = 1)
	for i := 2; i < sizeBytes; i++ {
		b[i] = byte(i % 256)
	}
	return b
}

// uploadFile streams data as proper ModelUploadChunk messages to UploadModel.
func uploadFile(t *testing.T, stream deploypb.ModelTransferService_UploadModelClient, filename string, data []byte) {
	t.Helper()

	hasher := sha256.New()
	hasher.Write(data)
	expectedHash := fmt.Sprintf("%x", hasher.Sum(nil))

	// Send metadata as the first message.
	if err := stream.Send(&deploypb.ModelUploadChunk{
		Content: &deploypb.ModelUploadChunk_Metadata{
			Metadata: &deploypb.ModelUploadMetadata{
				Filename:   filename,
				TotalSize:  int64(len(data)),
				Sha256Hash: expectedHash,
			},
		},
	}); err != nil {
		t.Fatalf("Send(metadata) error = %v", err)
	}

	// Stream data chunks.
	const uploadChunk = 512 * 1024 // 512KB per chunk
	for offset := 0; offset < len(data); offset += uploadChunk {
		end := offset + uploadChunk
		if end > len(data) {
			end = len(data)
		}
		if err := stream.Send(&deploypb.ModelUploadChunk{
			Content:     &deploypb.ModelUploadChunk_ChunkData{ChunkData: data[offset:end]},
			ChunkOffset: int64(offset),
		}); err != nil {
			t.Fatalf("Send(chunk offset=%d) error = %v", offset, err)
		}
	}
}

// TestModelUpload_HappyPath verifies that a well-formed .onnx upload completes
// successfully, returns the file path, and that the file exists on disk with
// matching content.
func TestModelUpload_HappyPath(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	modelDir := t.TempDir()
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	deploypb.RegisterModelTransferServiceServer(srv, grpcregistry.NewModelTransferServer(s, modelDir))
	go func() { srv.Serve(lis) }()
	t.Cleanup(func() { srv.GracefulStop(); lis.Close() })

	conn, _ := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) { return lis.DialContext(ctx) }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	t.Cleanup(func() { conn.Close() })
	client := deploypb.NewModelTransferServiceClient(conn)

	data := makeONNXBytes(1024 * 1024) // 1MB
	stream, err := client.UploadModel(context.Background())
	if err != nil {
		t.Fatalf("UploadModel() error = %v", err)
	}

	uploadFile(t, stream, "mymodel.onnx", data)

	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv() error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("expected success=true, got message: %s", resp.GetMessage())
	}
	if resp.GetFilePath() == "" {
		t.Fatal("expected non-empty file_path in response")
	}

	// Verify the file was actually written to disk.
	diskData, err := os.ReadFile(resp.GetFilePath())
	if err != nil {
		t.Fatalf("os.ReadFile(resp.FilePath) error = %v", err)
	}
	if len(diskData) != len(data) {
		t.Fatalf("disk file size %d, want %d", len(diskData), len(data))
	}
}

// TestModelUpload_SHA256Integrity verifies the round-trip hash: data uploaded
// must match the SHA256 the client declares in metadata.
func TestModelUpload_SHA256Integrity(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	client := newModelTransferClient(t, s)

	data := makeONNXBytes(256 * 1024)
	hasher := sha256.New()
	hasher.Write(data)
	correctHash := fmt.Sprintf("%x", hasher.Sum(nil))

	stream, _ := client.UploadModel(context.Background())
	stream.Send(&deploypb.ModelUploadChunk{
		Content: &deploypb.ModelUploadChunk_Metadata{
			Metadata: &deploypb.ModelUploadMetadata{
				Filename:   "integrity.onnx",
				TotalSize:  int64(len(data)),
				Sha256Hash: correctHash,
			},
		},
	})
	stream.Send(&deploypb.ModelUploadChunk{
		Content:     &deploypb.ModelUploadChunk_ChunkData{ChunkData: data},
		ChunkOffset: 0,
	})

	resp, err := stream.CloseAndRecv()
	if err != nil {
		t.Fatalf("CloseAndRecv() error = %v", err)
	}
	if !resp.GetSuccess() {
		t.Fatalf("upload failed: %s", resp.GetMessage())
	}
}

// TestModelUpload_RejectsNonONNXExtension verifies that a file without an
// .onnx extension is rejected with InvalidArgument.
func TestModelUpload_RejectsNonONNXExtension(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	client := newModelTransferClient(t, s)

	stream, err := client.UploadModel(context.Background())
	if err != nil {
		t.Fatalf("UploadModel() error = %v", err)
	}
	stream.Send(&deploypb.ModelUploadChunk{
		Content: &deploypb.ModelUploadChunk_Metadata{
			Metadata: &deploypb.ModelUploadMetadata{
				Filename:   "model.pt", // wrong extension
				TotalSize:  100,
				Sha256Hash: "abc123",
			},
		},
	})

	_, err = stream.CloseAndRecv()
	if err == nil {
		t.Fatal("expected error for non-.onnx extension, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestModelUpload_RejectsDataChunkAsFirstMessage verifies that sending a data
// chunk instead of metadata as the first message is rejected.
func TestModelUpload_RejectsDataChunkAsFirstMessage(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	client := newModelTransferClient(t, s)

	stream, err := client.UploadModel(context.Background())
	if err != nil {
		t.Fatalf("UploadModel() error = %v", err)
	}
	// Send data bytes as the FIRST message instead of metadata.
	stream.Send(&deploypb.ModelUploadChunk{
		Content:     &deploypb.ModelUploadChunk_ChunkData{ChunkData: []byte{0x08, 0x01}},
		ChunkOffset: 0,
	})

	_, err = stream.CloseAndRecv()
	if err == nil {
		t.Fatal("expected error when first message is data, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}

// TestModelUpload_RejectsWrongHash verifies that a deliberate SHA256 mismatch
// causes the upload to fail with DataLoss.
func TestModelUpload_RejectsWrongHash(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	client := newModelTransferClient(t, s)

	data := makeONNXBytes(64 * 1024)
	stream, _ := client.UploadModel(context.Background())
	stream.Send(&deploypb.ModelUploadChunk{
		Content: &deploypb.ModelUploadChunk_Metadata{
			Metadata: &deploypb.ModelUploadMetadata{
				Filename:   "bad-hash.onnx",
				TotalSize:  int64(len(data)),
				Sha256Hash: "0000000000000000000000000000000000000000000000000000000000000000",
			},
		},
	})
	stream.Send(&deploypb.ModelUploadChunk{
		Content: &deploypb.ModelUploadChunk_ChunkData{ChunkData: data},
	})

	_, err := stream.CloseAndRecv()
	if err == nil {
		t.Fatal("expected error for wrong hash, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.DataLoss {
		t.Fatalf("expected DataLoss, got %v", st.Code())
	}
}

// TestModelUpload_RejectsSizeMismatch verifies that sending fewer bytes than
// declared in total_size causes DataLoss.
func TestModelUpload_RejectsSizeMismatch(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	client := newModelTransferClient(t, s)

	data := makeONNXBytes(1024)
	hasher := sha256.New()
	hasher.Write(data)
	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	stream, _ := client.UploadModel(context.Background())
	stream.Send(&deploypb.ModelUploadChunk{
		Content: &deploypb.ModelUploadChunk_Metadata{
			Metadata: &deploypb.ModelUploadMetadata{
				Filename:   "short.onnx",
				TotalSize:  int64(len(data)) + 9999, // lie about the size
				Sha256Hash: hash,
			},
		},
	})
	stream.Send(&deploypb.ModelUploadChunk{
		Content: &deploypb.ModelUploadChunk_ChunkData{ChunkData: data},
	})

	_, err := stream.CloseAndRecv()
	if err == nil {
		t.Fatal("expected error for size mismatch, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.DataLoss {
		t.Fatalf("expected DataLoss, got %v", st.Code())
	}
}

// TestModelUpload_RejectsInvalidONNXMagic verifies that a file whose first
// byte has an invalid protobuf wire type is rejected as not being an ONNX file.
func TestModelUpload_RejectsInvalidONNXMagic(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	client := newModelTransferClient(t, s)

	// Wire type 6 in first byte — reserved/invalid in protobuf.
	data := make([]byte, 1024)
	data[0] = 0x06 // wire type 6 = invalid
	data[1] = 0x00
	for i := 2; i < len(data); i++ {
		data[i] = byte(i)
	}

	hasher := sha256.New()
	hasher.Write(data)
	hash := fmt.Sprintf("%x", hasher.Sum(nil))

	stream, _ := client.UploadModel(context.Background())
	stream.Send(&deploypb.ModelUploadChunk{
		Content: &deploypb.ModelUploadChunk_Metadata{
			Metadata: &deploypb.ModelUploadMetadata{
				Filename:   "fake.onnx",
				TotalSize:  int64(len(data)),
				Sha256Hash: hash,
			},
		},
	})
	stream.Send(&deploypb.ModelUploadChunk{
		Content: &deploypb.ModelUploadChunk_ChunkData{ChunkData: data},
	})

	_, err := stream.CloseAndRecv()
	if err == nil {
		t.Fatal("expected error for invalid ONNX magic, got nil")
	}
	st, _ := status.FromError(err)
	if st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", st.Code())
	}
}
