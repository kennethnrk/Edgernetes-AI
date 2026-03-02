package grpcregistry

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

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
// It serves raw model files to edge agents over a gRPC server-streaming RPC.
// This is the fallback path used when no external blob storage (e.g., S3) is configured.
type modelTransferServer struct {
	deploypb.UnimplementedModelTransferServiceServer
	store *store.Store
}

// NewModelTransferServer creates a new model transfer server.
func NewModelTransferServer(s *store.Store) deploypb.ModelTransferServiceServer {
	return &modelTransferServer{
		store: s,
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
