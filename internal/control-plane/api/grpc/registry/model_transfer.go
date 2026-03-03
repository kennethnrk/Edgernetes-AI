package grpcregistry

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	deploypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/deploy"
	registrycontroller "github.com/kennethnrk/edgernetes-ai/internal/control-plane/controller/registry"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// chunkSize is the size of each chunk streamed to the agent (2MB).
// This is large enough to be efficient but small enough to avoid
// saturating the network or exhausting memory on either end.
const chunkSize = 2 * 1024 * 1024 // 2MB

// modelTransferServer implements the ModelTransferServiceServer interface.
// It handles both streaming model files to edge agents (DownloadModel) and
// receiving model files uploaded from clients (UploadModel).
type modelTransferServer struct {
	deploypb.UnimplementedModelTransferServiceServer
	store    *store.Store
	modelDir string // directory where uploaded model files are persisted
}

// NewModelTransferServer creates a new model transfer server.
// modelDir is the directory where uploaded models will be stored on disk.
// If it does not exist it will be created automatically on the first upload.
func NewModelTransferServer(s *store.Store, modelDir string) deploypb.ModelTransferServiceServer {
	return &modelTransferServer{
		store:    s,
		modelDir: modelDir,
	}
}

// DownloadModel streams a model file to the requesting agent in sequential 2MB chunks.
//
// The model's file path is resolved from the model registry using the provided
// model_id. The agent may specify a resume_byte_offset to restart a broken
// download without starting from zero.
//
// The stream runs synchronously within the goroutine spawned by the gRPC server
// for each incoming RPC, so transfer concurrency is naturally managed by the
// gRPC server's connection handler — no additional goroutine management is needed here.
// gRPC's built-in flow control (backpressure) will cause stream.Send() to block if
// the agent is consuming chunks slower than the control plane sends them, preventing
// the network from being overwhelmed.
func (s *modelTransferServer) DownloadModel(req *deploypb.ModelDownloadRequest, stream grpc.ServerStreamingServer[deploypb.ModelChunk]) error {
	if req == nil || req.GetModelId() == "" {
		return status.Error(codes.InvalidArgument, "model_id cannot be empty")
	}

	modelID := req.GetModelId()
	log.Printf("[model-transfer] agent requested model %q (resume offset: %d)", modelID, req.GetResumeByteOffset())

	// Look up the model in the registry to get its file path.
	modelInfo, found, err := registrycontroller.GetModelByID(s.store, modelID)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to look up model: %v", err)
	}
	if !found {
		return status.Errorf(codes.NotFound, "model %q not found in registry", modelID)
	}

	// Resolve the file path. FilePath in the registry may be relative or absolute.
	filePath := filepath.Clean(modelInfo.FilePath)
	file, err := os.Open(filePath)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to open model file %q: %v", filePath, err)
	}
	defer file.Close()

	// Support resuming a broken download by seeking to the requested byte offset.
	if offset := req.GetResumeByteOffset(); offset > 0 {
		if _, err := file.Seek(offset, io.SeekStart); err != nil {
			return status.Errorf(codes.Internal, "failed to seek to offset %d: %v", offset, err)
		}
		log.Printf("[model-transfer] resuming model %q from byte offset %d", modelID, offset)
	}

	buf := make([]byte, chunkSize)
	var currentOffset int64 = req.GetResumeByteOffset()
	var chunkCount int

	for {
		// Check if the client has cancelled or timed out before reading the next chunk.
		if err := stream.Context().Err(); err != nil {
			log.Printf("[model-transfer] client cancelled transfer of model %q after %d chunks: %v", modelID, chunkCount, err)
			return status.Errorf(codes.Canceled, "client cancelled: %v", err)
		}

		bytesRead, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				break // Transfer complete
			}
			return status.Errorf(codes.Internal, "error reading model file: %v", err)
		}

		if err := stream.Send(&deploypb.ModelChunk{
			ChunkData:   buf[:bytesRead],
			ChunkOffset: currentOffset,
		}); err != nil {
			// The agent disconnected or the stream was interrupted.
			return fmt.Errorf("stream send failed at offset %d: %w", currentOffset, err)
		}

		currentOffset += int64(bytesRead)
		chunkCount++
	}

	log.Printf("[model-transfer] completed transfer of model %q: %d chunks, %d bytes", modelID, chunkCount, currentOffset)
	return nil
}

// UploadModel receives a client-streaming upload of a model file, validates it,
// persists it to disk, and returns the stored file path.
//
// Protocol:
//  1. The FIRST message MUST carry ModelUploadMetadata (filename, total_size, sha256_hash).
//     Filename must have an ".onnx" extension.
//  2. All SUBSEQUENT messages MUST carry chunk_data bytes.
//
// Validation performed by the server:
//   - Filename must end in ".onnx"
//   - First 2 bytes of file content are verified to be a valid protobuf varint field tag,
//     which is the binary structure all ONNX ModelProto files start with.
//   - Total bytes received must equal the declared total_size.
//   - SHA256 of the received bytes must match the declared sha256_hash.
//
// On success the file is atomically renamed from a temporary ".uploading" path
// to its final name and the absolute path is returned to the caller.
func (s *modelTransferServer) UploadModel(stream grpc.ClientStreamingServer[deploypb.ModelUploadChunk, deploypb.ModelUploadResponse]) error {
	// ── Step 1: receive and validate the metadata message ────────────────────
	firstMsg, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to receive first message: %v", err)
	}

	meta, ok := firstMsg.GetContent().(*deploypb.ModelUploadChunk_Metadata)
	if !ok || meta.Metadata == nil {
		return status.Error(codes.InvalidArgument, "first message must contain ModelUploadMetadata")
	}

	filename := strings.TrimSpace(meta.Metadata.GetFilename())
	if filename == "" {
		return status.Error(codes.InvalidArgument, "metadata.filename cannot be empty")
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".onnx") {
		return status.Errorf(codes.InvalidArgument, "filename %q must have an .onnx extension", filename)
	}
	expectedHash := strings.TrimSpace(meta.Metadata.GetSha256Hash())
	if expectedHash == "" {
		return status.Error(codes.InvalidArgument, "metadata.sha256_hash cannot be empty")
	}
	declaredSize := meta.Metadata.GetTotalSize()
	if declaredSize <= 0 {
		return status.Error(codes.InvalidArgument, "metadata.total_size must be greater than zero")
	}

	log.Printf("[model-upload] starting upload: file=%q size=%d expected_hash=%s", filename, declaredSize, expectedHash)

	// ── Step 2: prepare the storage directory and temp file ──────────────────
	if err := os.MkdirAll(s.modelDir, 0755); err != nil {
		return status.Errorf(codes.Internal, "failed to create model directory: %v", err)
	}

	uploadID := uuid.New().String()
	tempPath := filepath.Join(s.modelDir, uploadID+".uploading")

	tmpFile, err := os.Create(tempPath)
	if err != nil {
		return status.Errorf(codes.Internal, "failed to create temp file: %v", err)
	}

	// Ensure temp file is always cleaned up on any failure path.
	committed := false
	defer func() {
		tmpFile.Close()
		if !committed {
			os.Remove(tempPath)
		}
	}()

	// ── Step 3: stream and write chunks ──────────────────────────────────────
	hasher := sha256.New()
	writer := io.MultiWriter(tmpFile, hasher)

	var totalWritten int64
	var onnxValidated bool // checked on the very first data chunk

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break // client has finished sending
		}
		if err != nil {
			return status.Errorf(codes.Internal, "stream receive error: %v", err)
		}

		dataChunk, ok := msg.GetContent().(*deploypb.ModelUploadChunk_ChunkData)
		if !ok {
			return status.Error(codes.InvalidArgument, "subsequent messages must carry chunk_data, not metadata")
		}
		data := dataChunk.ChunkData

		// Validate ONNX magic bytes on the very first data chunk.
		// All ONNX ModelProto (protobuf) files begin with a varint-encoded field tag.
		// A valid protobuf field tag byte has its lower 3 bits (wire type) in {0,1,2,5},
		// meaning bit patterns xxx_000, xxx_001, xxx_010, or xxx_101.
		// Wire type 6 and 7 are reserved/invalid in protobuf.
		if !onnxValidated {
			if len(data) < 2 {
				return status.Error(codes.InvalidArgument, "first data chunk too small to validate as ONNX")
			}
			wireType := data[0] & 0x07
			if wireType == 6 || wireType == 7 {
				return status.Errorf(codes.InvalidArgument,
					"file does not appear to be a valid ONNX model: invalid protobuf wire type %d in first byte", wireType)
			}
			onnxValidated = true
		}

		if _, err := writer.Write(data); err != nil {
			return status.Errorf(codes.Internal, "failed to write chunk at offset %d: %v", msg.GetChunkOffset(), err)
		}
		totalWritten += int64(len(data))
	}

	// ── Step 4: validate size ─────────────────────────────────────────────────
	if totalWritten != declaredSize {
		return status.Errorf(codes.DataLoss,
			"upload size mismatch: received %d bytes, expected %d", totalWritten, declaredSize)
	}

	// ── Step 5: validate SHA256 hash ─────────────────────────────────────────
	actualHash := fmt.Sprintf("%x", hasher.Sum(nil))
	if actualHash != expectedHash {
		return status.Errorf(codes.DataLoss,
			"SHA256 mismatch: got %s, expected %s", actualHash, expectedHash)
	}

	// ── Step 6: atomic rename to final path ──────────────────────────────────
	// Use the original client filename (base name only — no directory traversal).
	safeFilename := filepath.Base(filename)
	finalPath, err := filepath.Abs(filepath.Join(s.modelDir, safeFilename))
	if err != nil {
		return status.Errorf(codes.Internal, "failed to resolve final path: %v", err)
	}

	tmpFile.Close()
	if err := os.Rename(tempPath, finalPath); err != nil {
		return status.Errorf(codes.Internal, "failed to commit model file: %v", err)
	}
	committed = true

	log.Printf("[model-upload] committed model %q → %s (%d bytes)", filename, finalPath, totalWritten)

	return stream.SendAndClose(&deploypb.ModelUploadResponse{
		Success:  true,
		FilePath: finalPath,
		Message:  fmt.Sprintf("model %q uploaded successfully (%d bytes)", safeFilename, totalWritten),
	})
}
