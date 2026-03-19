package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kennethnrk/edgernetes-ai/internal/client"
	deploypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/deploy"
)

const uploadChunkSize = 2 * 1024 * 1024 // 2 MB

// uploadModel streams a local file to the control plane via UploadModel.
func uploadModel(c *client.Client, localPath, filename string) (*deploypb.ModelUploadResponse, error) {
	// Open file
	f, err := os.Open(localPath)
	if err != nil {
		return nil, fmt.Errorf("could not open %s: %w", localPath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("could not stat %s: %w", localPath, err)
	}
	totalSize := info.Size()

	// Compute SHA256
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return nil, fmt.Errorf("could not compute SHA256: %w", err)
	}
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Reset to start
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("could not seek: %w", err)
	}

	// Resolve filename
	if filename == "" || filename == localPath {
		filename = filepath.Base(localPath)
	}

	// Open stream
	ctx, cancel := c.Context()
	defer cancel()

	stream, err := c.Transfer.UploadModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not open upload stream: %w", err)
	}

	// Send metadata as first message
	err = stream.Send(&deploypb.ModelUploadChunk{
		Content: &deploypb.ModelUploadChunk_Metadata{
			Metadata: &deploypb.ModelUploadMetadata{
				Filename:   filename,
				TotalSize:  totalSize,
				Sha256Hash: hash,
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("could not send metadata: %w", err)
	}

	// Stream chunks
	buf := make([]byte, uploadChunkSize)
	var offset int64
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			err = stream.Send(&deploypb.ModelUploadChunk{
				Content: &deploypb.ModelUploadChunk_ChunkData{
					ChunkData: buf[:n],
				},
				ChunkOffset: offset,
			})
			if err != nil {
				return nil, fmt.Errorf("could not send chunk at offset %d: %w", offset, err)
			}
			offset += int64(n)

			// Progress
			pct := float64(offset) / float64(totalSize) * 100
			fmt.Fprintf(os.Stderr, "\rUploading %s: %.1f%% (%d / %d bytes)", filename, pct, offset, totalSize)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read error: %w", readErr)
		}
	}

	fmt.Fprintln(os.Stderr) // newline after progress

	resp, err := stream.CloseAndRecv()
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}
	return resp, nil
}
