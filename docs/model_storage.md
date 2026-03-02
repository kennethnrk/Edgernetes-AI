# Model Storage and Transfer Architecture

This document details the architecture and implementation strategy for transferring AI models from the Edgernetes-AI control plane to edge agents. 

## Overview
Given the potentially large size of AI models (e.g., LLMs and CV models can be gigabytes in size), it is crucial to implement a reliable and network-efficient transfer mechanism. The system will use a **hybrid transfer strategy**:
1. **Primary Method (S3-Compatible Object Storage):** If configured, the control plane will provide a pre-signed URL to a blob storage service (like AWS S3, MinIO, or Cloudflare R2), offloading the bandwidth completely.
2. **Fallback Method (gRPC Streaming):** If no external storage is configured, the control plane will securely transfer the model file via Server-Streaming gRPC chunks over a dedicated connection, ensuring no external dependencies are strictly strictly required.

---

## 1. Storage Configuration (Control Plane)
The control plane configuration should support an optional section for external storage:

```yaml
# config.yaml (Control Plane)
storage:
  type: "s3" # Options: "local" (default), "s3"
  s3:
    endpoint: "s3.amazonaws.com" # or minio endpoint
    bucket: "edgernetes-models"
    access_key: "..."
    secret_key: "..."
    region: "us-east-1"
```

If `type` is set to `local`, models are stored on the control plane's local disk, and the gRPC streaming fallback must be used.

---

## 2. Protobuf Definitions

The gRPC definitions must support both external URLs and the streaming fallback.

```protobuf
// api/proto/deploy.proto
syntax = "proto3";
package deploy;

message ModelDeployment {
    string model_id = 1;
    string model_name = 2;
    int64 size_bytes = 3;
    string sha256_hash = 4; // Crucial for data integrity validation

    // If populated, the agent downloads directly from this URL.
    // If empty, the agent MUST use the DownloadModel gRPC stream.
    string download_url = 5; 
}

message ModelDownloadRequest {
    string model_id = 1;
    // For advanced implementations: support resuming broken downloads
    int64 resume_byte_offset = 2; 
}

message ModelChunk {
    bytes chunk_data = 1;
    // Using chunk index/offset helps the agent verify sequential delivery
    int64 chunk_offset = 2; 
}

service ModelTransferService {
    // Server-streaming RPC to prevent Out-Of-Memory (OOM) errors
    rpc DownloadModel(ModelDownloadRequest) returns (stream ModelChunk);
}
```

---

## 3. Implementation: gRPC Streaming Fallback (Control Plane)

To prevent overwhelming the network or the control plane's memory, the control plane must send the file in manageable chunks (e.g., 2MB - 4MB). This should be run in a separate Go routine so as to not block other control plane operations.

```go
const ChunkSize = 2 * 1024 * 1024 // 2MB chunks

func (s *ModelTransferServer) DownloadModel(req *pb.ModelDownloadRequest, stream pb.ModelTransferService_DownloadModelServer) error {
    filePath := filepath.Join(s.ModelDir, req.ModelId)
    file, err := os.Open(filePath)
    if err != nil {
        return fmt.Errorf("failed to open model file: %v", err)
    }
    defer file.Close()

    // Seek to the requested offset if resuming
    if req.ResumeByteOffset > 0 {
        _, err = file.Seek(req.ResumeByteOffset, io.SeekStart)
        if err != nil {
            return err
        }
    }

    buffer := make([]byte, ChunkSize)
    var currentOffset int64 = req.ResumeByteOffset

    for {
        bytesRead, err := file.Read(buffer)
        if err != nil {
            if err == io.EOF {
                // Transfer complete
                break
            }
            return fmt.Errorf("error reading chunk: %v", err)
        }

        chunk := &pb.ModelChunk{
            ChunkData:    buffer[:bytesRead],
            ChunkOffset: currentOffset,
        }

        // Send blocks if the client is slow (gRPC handles backpressure natively)
        if err := stream.Send(chunk); err != nil {
            return fmt.Errorf("stream failed: %v", err)
        }

        currentOffset += int64(bytesRead)
    }

    return nil
}
```

---

## 4. Implementation: Agent Download Strategy (Edge Node)

On the edge agent, downloading must be handled with care to prevent corrupting existing models or crashing the process.

### Workflow:
1. **Receive Deployment Request:** Check if the model already exists locally and if its SHA256 hash matches.
2. **Determine Download Method:** Revert to gRPC strictly if `download_url` is empty.
3. **Execute Download (Asynchronous):** Launch a dedicated goroutine for the network IO.
4. **Temporary File Pattern:** Write chunks to a `.downloading` temp file. Do NOT overwrite an actively running model.
5. **Verify and Commit:** Upon stream completion, calculate the SHA256 of the temp file. If it matches the expected hash, atomically rename it to the final filename.

### gRPC Client Example:

```go
func (a *Agent) downloadModelViaGRPC(ctx context.Context, modelID string, expectedHash string) error {
    req := &pb.ModelDownloadRequest{
        ModelId:          modelID,
        ResumeByteOffset: 0,
    }

    stream, err := a.modelTransferClient.DownloadModel(ctx, req)
    if err != nil {
        return err
    }

    tempFilePath := filepath.Join(a.modelDir, modelID+".downloading")
    file, err := os.Create(tempFilePath)
    if err != nil {
        return err
    }
    // Clean up temp file on failure
    defer func() {
        if file != nil {
            file.Close()
            os.Remove(tempFilePath)
        }
    }()

    hasher := sha256.New()
    writer := io.MultiWriter(file, hasher)

    for {
        chunk, err := stream.Recv()
        if err == io.EOF {
            break // Success
        }
        if err != nil {
            return fmt.Errorf("download interrupted: %v", err)
        }

        if _, err := writer.Write(chunk.ChunkData); err != nil {
            return fmt.Errorf("failed to write chunk to disk: %v", err)
        }
    }

    // Verify Integrity
    actualHash := fmt.Sprintf("%x", hasher.Sum(nil))
    if actualHash != expectedHash {
        return fmt.Errorf("hash mismatch: expected %s, got %s", expectedHash, actualHash)
    }

    file.Close()
    file = nil // Prevent defer from deleting it

    // Atomic Rename
    finalFilePath := filepath.Join(a.modelDir, modelID)
    if err := os.Rename(tempFilePath, finalFilePath); err != nil {
        return fmt.Errorf("failed to finalize model file: %v", err)
    }

    return nil
}
```

## 5. Next Steps / Future Enhancements
* **Resumable Downloads:** Implement a local state tracker that saves the last written byte offset so broken downloads can restart without starting from zero.
* **Bandwidth Throttling:** Add a `time.Sleep` mechanism or use Go's `golang.org/x/time/rate` package on the client or server side to prevent the model transfer from completely saturating the agent's WAN link, which could drop heartbeat packets.
* **Disk Space Checks:** Always compute `fs.Statfs` before beginning the download to ensure the agent has enough disk space for the `size_bytes` declared in the metadata.
