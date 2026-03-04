# Model Storage and Transfer Architecture

This document details the architecture and implementation strategy for transferring AI models from the Edgernetes-AI control plane to edge agents.

## Overview

The `file_path` field in model registration accepts **both** local filesystem paths and remote blob URLs:

| Path Type | Example | Transfer Method |
|-----------|---------|-----------------|
| **Local** | `/models/model.onnx`, `C:\models\model.onnx` | gRPC streaming (`DownloadModel` / `UploadModel`) |
| **Network** | `https://s3.amazonaws.com/bucket/model.onnx`, `s3://...`, `gs://...` | Agent downloads directly from the URL |

Use `modelpath.IsNetworkPath(filePath)` (from `internal/common/modelpath`) to determine which flow to use. This utility is shared by both the control plane and edge agents.

---

## 1. Path Detection (`internal/common/modelpath`)

```go
import "github.com/kennethnrk/edgernetes-ai/internal/common/modelpath"

if modelpath.IsNetworkPath(filePath) {
    // Download directly from the URL (HTTP GET, S3 SDK, etc.)
} else {
    // Use gRPC DownloadModel stream from the control plane
}
```

Recognized network schemes: `http://`, `https://`, `s3://`, `gs://`, `az://`.

---

## 2. Protobuf Definitions

```protobuf
// api/proto/deploy.proto

message DeployModelRequest {
    string model_id = 1;
    string name = 2;
    string version = 3;
    string file_path = 4;     // Local path OR network URL
    string model_type = 5;
    int64 model_size = 6;
    int32 instance_count = 7;
    // Field 8 reserved
    string sha256_hash = 9;
}

service ModelTransferService {
    rpc DownloadModel(ModelDownloadRequest) returns (stream ModelChunk);
    rpc UploadModel(stream ModelUploadChunk) returns (ModelUploadResponse);
}
```

---

## 3. gRPC Streaming (Local Files)

When `file_path` is a **local** path:

### Control Plane → Agent (DownloadModel)
- Server reads the file in **2MB chunks** and streams them to the agent.
- Supports **resume** via `resume_byte_offset`.
- gRPC backpressure prevents network saturation.

### Client → Control Plane (UploadModel)
- Client sends `ModelUploadMetadata` as the first message (filename, total_size, sha256_hash).
- Subsequent messages carry `chunk_data`.
- Server validates: `.onnx` extension, ONNX magic bytes, size match, SHA256 match.
- Written to `<MODEL_STORE_DIR>/<filename>` via atomic rename.

---

## 4. Agent Download Strategy

When the agent receives a `DeployModelRequest`:

1. Call `modelpath.IsNetworkPath(req.FilePath)`:
   - **Network?** → Download directly from the URL using HTTP/SDK.
   - **Local?** → Request the file via `DownloadModel` gRPC stream from the control plane.
2. Write to a **temporary file** (`<model_id>.downloading`).
3. **Verify SHA256** hash matches `req.Sha256Hash`.
4. **Atomic rename** to the final path.
5. Load the model into the inference runtime.

---

## 5. Future Enhancements
* **Resumable Downloads:** Track last written byte offset so broken downloads restart without starting from zero.
* **Bandwidth Throttling:** Use `golang.org/x/time/rate` to prevent model transfer from saturating the agent's WAN link.
* **Disk Space Checks:** Check available storage before starting the download.
