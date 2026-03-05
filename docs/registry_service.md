# Registry Service

The Registry Service is the central component of the Edgernetes-AI control plane responsible for tracking all models and edge nodes in the cluster. It exposes two gRPC APIs — **ModelRegistryAPI** and **NodeRegistryAPI** — and persists state through a WAL-backed key–value store.

## Architecture

```
┌──────────────────────────────────────────────────┐
│                  gRPC Server                     │
│                                                  │
│  ┌──────────────────┐  ┌──────────────────────┐  │
│  │ ModelRegistryAPI  │  │  NodeRegistryAPI     │  │
│  │ (model_registry)  │  │  (node_registry)     │  │
│  └────────┬─────────┘  └──────────┬───────────┘  │
│           │                       │              │
│  ┌────────▼───────────────────────▼───────────┐  │
│  │          Registry Controller Layer         │  │
│  │  model_registry.go  │  node_registry.go    │  │
│  └────────────────────┬───────────────────────┘  │
│                       │                          │
│              ┌────────▼────────┐                 │
│              │   Store (KV)    │                 │
│              │   WAL-backed    │                 │
│              └─────────────────┘                 │
└──────────────────────────────────────────────────┘
```

The service is split into two layers:

| Layer | Package | Responsibility |
|---|---|---|
| **gRPC API** | `grpcregistry` | Proto ↔ store type conversion, input validation, gRPC error codes |
| **Controller** | `registrycontroller` | Business logic, store reads/writes, uniqueness constraints |

---

## Model Registration

### Proto Definition (`model.proto`)

```protobuf
service ModelRegistryAPI {
    rpc RegisterModel(ModelInfo)         returns (BoolResponse);
    rpc DeRegisterModel(ModelID)         returns (BoolResponse);
    rpc UpdateModel(UpdateModelRequest)  returns (BoolResponse);
    rpc GetModel(ModelID)                returns (ModelInfo);
    rpc ListModels(None)                 returns (ListModelsResponse);
}
```

### RegisterModel

Registers a new model in the control plane. The server generates a UUID for the model; any `id` field in the request is ignored.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `string` | **Yes** | Human-readable model name. **Must be unique** across all registered models. |
| `version` | `string` | No | Semantic version string (e.g. `"v1.2"`) |
| `file_path` | `string` | No | Local path or network blob URL for the model file |
| `model_type` | `string` | No | Model type (`cnn`, `linear`, `decision_tree`, `llm`) |
| `model_size` | `int64` | No | Size of the model file in bytes |
| `replicas` | `int32` | No | Desired number of replicas to deploy |
| `input_format` | `string` | No | JSON schema describing the expected inference input |

**Response:** `BoolResponse { success: true }` on success.

**Error Codes:**

| Code | Condition |
|---|---|
| `INVALID_ARGUMENT` | Request is `nil`, or `name` is empty |
| `ALREADY_EXISTS` | A model with the same `name` is already registered |
| `INTERNAL` | Store or serialization failure |

**Example (grpcurl):**

```bash
grpcurl -plaintext -d '{
  "name": "car-price-predictor",
  "version": "v1",
  "file_path": "/models/car_price.onnx",
  "model_type": "decision_tree",
  "model_size": 2048,
  "replicas": 2,
  "input_format": "{\"price\":\"number\",\"year\":\"number\"}"
}' localhost:50051 modelRegistryAPI.ModelRegistryAPI/RegisterModel
```

### DeRegisterModel

Removes a model from the registry by ID.

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | `string` | **Yes** | UUID of the model to remove |

**Error Codes:**

| Code | Condition |
|---|---|
| `INVALID_ARGUMENT` | `id` is empty |
| `INTERNAL` | Store failure |

### UpdateModel

Replaces the stored model metadata for an existing model.

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | `string` | **Yes** | UUID of the model to update |
| `name` | `string` | No | New name |
| `version` | `string` | No | New version |
| `file_path` | `string` | No | New file path |
| `model_type` | `string` | No | New model type |
| `model_size` | `int64` | No | New size |
| `replicas` | `int32` | No | New replica count |
| `input_format` | `string` | No | New input format |

**Error Codes:**

| Code | Condition |
|---|---|
| `INVALID_ARGUMENT` | `id` is empty |
| `INTERNAL` | Store or serialization failure |

### GetModel

Retrieves a model by its UUID.

| Field | Type | Required | Description |
|---|---|---|---|
| `id` | `string` | **Yes** | UUID of the model |

**Response:** Full `ModelInfo` message.

**Error Codes:**

| Code | Condition |
|---|---|
| `INVALID_ARGUMENT` | `id` is empty |
| `NOT_FOUND` | No model with that `id` exists |
| `INTERNAL` | Store or deserialization failure |

### ListModels

Returns all registered models. Takes an empty `None` request.

**Response:** `ListModelsResponse { repeated ModelInfo models }`

---

## Node Registration

### Proto Definition (`node.proto`)

```protobuf
service NodeRegistryAPI {
    rpc RegisterNode(NodeInfo)           returns (RegisterNodeResponse);
    rpc DeRegisterNode(NodeID)           returns (BoolResponse);
    rpc UpdateNode(UpdateNodeRequest)    returns (BoolResponse);
    rpc GetNode(NodeID)                  returns (NodeInfo);
    rpc ListNodes(None)                  returns (ListNodesResponse);
}
```

### RegisterNode

Registers a new edge node. The server generates a UUID and returns it in the response; any `node_id` in the request is ignored.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `string` | No | Human-readable node name |
| `ip` | `string` | No | IP address of the node |
| `port` | `int32` | No | Port the agent is listening on |
| `metadata` | `NodeMetadata` | No | OS type, agent version, hostname |
| `resource_capabilities` | `ResourceCapabilities` | No | Memory, storage, and compute devices |

**NodeMetadata fields:**

| Field | Type | Description |
|---|---|---|
| `os_type` | `string` | Operating system (e.g. `"linux"`, `"windows"`) |
| `agent_version` | `string` | Version of the Edgernetes agent |
| `hostname` | `string` | Machine hostname |

**ResourceCapabilities fields:**

| Field | Type | Description |
|---|---|---|
| `memory` | `MemoryInfo` | Total, free, used memory (bytes) and memory type |
| `storage` | `StorageInfo` | Total, free, used storage (bytes) |
| `compute_devices` | `ComputeDevice[]` | GPUs, TPUs, NPUs available on the node |

**ComputeDevice fields:**

| Field | Type | Description |
|---|---|---|
| `type` | `string` | Device type (`gpu`, `tpu`, `npu`) |
| `vendor` | `string` | Manufacturer (e.g. `"nvidia"`) |
| `model` | `string` | Model name (e.g. `"RTX 3080"`) |
| `memory` | `int64` | Device memory in bytes |
| `compute_units` | `int64` | Number of compute units / cores |
| `tops` | `float` | Tera-operations per second |
| `power_draw_watts` | `int64` | Power consumption in watts |

**Response:** `RegisterNodeResponse { node_id: "<generated-uuid>" }`

**Error Codes:**

| Code | Condition |
|---|---|
| `INVALID_ARGUMENT` | Request is `nil` |
| `INTERNAL` | Store or serialization failure |

**Example (grpcurl):**

```bash
grpcurl -plaintext -d '{
  "name": "edge-pi-01",
  "ip": "192.168.1.42",
  "port": 50052,
  "metadata": {
    "os_type": "linux",
    "agent_version": "0.3.0",
    "hostname": "pi-cluster-01"
  },
  "resource_capabilities": {
    "memory": { "total": 8589934592, "free": 4294967296, "used": 4294967296, "type": "RAM" },
    "storage": { "total": 64424509440, "free": 32212254720, "used": 32212254720 }
  }
}' localhost:50051 nodeRegistryAPI.NodeRegistryAPI/RegisterNode
```

### DeRegisterNode

Removes a node from the registry by ID.

| Field | Type | Required | Description |
|---|---|---|---|
| `node_id` | `string` | **Yes** | UUID of the node to remove |

**Error Codes:**

| Code | Condition |
|---|---|
| `INVALID_ARGUMENT` | `node_id` is empty |
| `INTERNAL` | Store failure |

### UpdateNode

Updates an existing node's metadata and/or resource capabilities. Fields not provided in the request are preserved from the existing record.

| Field | Type | Required | Description |
|---|---|---|---|
| `node_id` | `string` | **Yes** | UUID of the node to update |
| `metadata` | `NodeMetadata` | No | Updated metadata (replaces entirely if provided) |
| `resource_capabilities` | `ResourceCapabilities` | No | Updated capabilities (replaces entirely if provided) |

**Error Codes:**

| Code | Condition |
|---|---|
| `INVALID_ARGUMENT` | `node_id` is empty |
| `NOT_FOUND` | No node with that `node_id` exists |
| `INTERNAL` | Store or serialization failure |

### GetNode

Retrieves a node by its UUID.

| Field | Type | Required | Description |
|---|---|---|---|
| `node_id` | `string` | **Yes** | UUID of the node |

**Response:** Full `NodeInfo` message.

**Error Codes:**

| Code | Condition |
|---|---|
| `INVALID_ARGUMENT` | `node_id` is empty |
| `NOT_FOUND` | No node with that `node_id` exists |
| `INTERNAL` | Store or deserialization failure |

### ListNodes

Returns all registered nodes. Takes an empty `None` request.

**Response:** `ListNodesResponse { repeated NodeInfo nodes }`

---

## Store Key Conventions

All registry data is persisted in the shared WAL-backed key–value store using prefixed keys:

| Prefix | Entity | Example Key |
|---|---|---|
| `model:` | Model | `model:550e8400-e29b-41d4-a716-446655440000` |
| `node:` | Node | `node:6ba7b810-9dad-11d1-80b4-00c04fd430c8` |

## Timestamps (Nodes Only)

Node records track the following server-managed timestamps:

| Field | Set On |
|---|---|
| `RegisteredAt` | First registration |
| `UpdatedAt` | Every write (register, update, status change) |
| `LastHeartbeat` | Status updates |
| `LastActivity` | Registration and status updates |
