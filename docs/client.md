# The Edgernetes-AI Client (`edgectl`)

The client is a CLI tool that allows operators and developers to interact with the Edgernetes-AI control plane. It supports registering and managing models (individually or in bulk via YAML), inspecting cluster state, running inference, and uploading model files — all from a single binary.

> [!NOTE]
> The client communicates exclusively over **gRPC** with the control plane. No REST endpoints are used.

---

## High-Level Architecture

```
┌───────────────────────────────────────────────────────────────────────┐
│                          edgectl CLI                                  │
│                                                                       │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────────┐    │
│  │  Command      │  │  YAML        │  │  Config Manager           │    │
│  │  Parser       │  │  Parser      │  │  (~/.edgectl/config.yaml) │    │
│  │  (cobra)      │  │  (gopkg.in/  │  │                           │    │
│  │              │  │   yaml.v3)   │  │  • control plane address  │    │
│  └──────┬───────┘  └──────┬───────┘  │  • default namespace      │    │
│         │                 │          │  • output format           │    │
│         ▼                 ▼          └────────────┬───────────────┘    │
│  ┌─────────────────────────────────────────────────┐                  │
│  │              gRPC Client Wrapper                 │                  │
│  │                                                  │                  │
│  │  ┌────────────────┐  ┌────────────────────────┐ │                  │
│  │  │ ModelRegistry   │  │ NodeRegistry           │ │                  │
│  │  │ Client          │  │ Client                 │ │                  │
│  │  ├────────────────┤  ├────────────────────────┤ │                  │
│  │  │ Deploy Client   │  │ ModelTransfer Client   │ │                  │
│  │  ├────────────────┤  ├────────────────────────┤ │                  │
│  │  │ Infer Client    │  │ Discovery Client       │ │                  │
│  │  └────────────────┘  └────────────────────────┘ │                  │
│  │                                                  │                  │
│  │  ┌────────────────────────────────────────────┐ │                  │
│  │  │ Connection Pool & Retry Logic               │ │                  │
│  │  └────────────────────────────────────────────┘ │                  │
│  └──────────────────────────────────────────────────┘                  │
│                                                                       │
│  ┌────────────────────┐  ┌─────────────────┐                          │
│  │  Output Formatter   │  │  Error Handler   │                         │
│  │  (table / json /   │  │  (gRPC → human   │                         │
│  │   yaml)            │  │   readable)      │                         │
│  └────────────────────┘  └─────────────────┘                          │
└───────────────────────────────────────────────────────────────────────┘
```

---

## 1. Command Parser

The command parser is built with **[cobra](https://github.com/spf13/cobra)**, the de-facto standard for Go CLI tools. It provides a `kubectl`-style verb-noun command structure.

### 1.1 Environment Configuration

Before any other command, the user sets the control plane address:

```bash
# Set the control plane endpoint (persisted to ~/.edgectl/config.yaml)
edgectl config set-endpoint 192.168.1.10:50051

# View current config
edgectl config view

# Set default namespace (optional, defaults to "default")
edgectl config set-namespace production

# Set default output format
edgectl config set-output json
```

The config file is automatically created on first use:

```yaml
# ~/.edgectl/config.yaml
endpoint: "192.168.1.10:50051"
default_namespace: "default"
output_format: "table"          # table | json | yaml
timeout_seconds: 10
```

### 1.2 Full Command Tree

```
edgectl
│
├── config                              # Environment configuration
│   ├── set-endpoint <host:port>        # Set control plane address
│   ├── set-namespace <namespace>       # Set default namespace
│   ├── set-output <table|json|yaml>    # Set output format
│   └── view                            # Print current configuration
│
├── apply -f <file.yaml>                # Declarative bulk apply from YAML
│       --namespace <ns>                # Override namespace in YAML
│       --dry-run                       # Validate only, don't submit
│
├── model                               # Model management
│   ├── register                        # Register a single model
│   │       --name <name>               # (required)
│   │       --namespace <ns>            # (default: config default)
│   │       --version <ver>
│   │       --file-path <path|url>
│   │       --model-type <type>
│   │       --model-size <bytes>
│   │       --replicas <n>
│   │       --input-format <json>
│   │
│   ├── deregister <model-id>           # Remove model by ID
│   │       --namespace <ns>
│   │
│   ├── update <model-id>               # Update model fields
│   │       --name <name>
│   │       --namespace <ns>
│   │       --version <ver>
│   │       --file-path <path|url>
│   │       --model-type <type>
│   │       --model-size <bytes>
│   │       --replicas <n>
│   │       --input-format <json>
│   │
│   ├── get <model-id>                  # Get model by ID
│   │       -o <table|json|yaml>
│   │
│   ├── list                            # List all models
│   │       --namespace <ns>            # Filter by namespace
│   │       -o <table|json|yaml>
│   │
│   ├── status <model-name>             # Get deployment status & replicas
│   │       --namespace <ns>
│   │       -o <table|json|yaml>
│   │
│   ├── nodes <model-name>              # List nodes serving this model
│   │       --namespace <ns>
│   │       -o <table|json|yaml>
│   │
│   └── upload <file-path>              # Upload a model file to the CP
│           --filename <name.onnx>      # Override uploaded filename
│
├── node                                 # Node inspection
│   ├── get <node-id>                   # Get node details
│   │       -o <table|json|yaml>
│   ├── list                            # List all nodes
│   │       -o <table|json|yaml>
│   └── endpoints                       # List all node endpoints (discovery)
│           -o <table|json|yaml>
│
├── deploy <model-id>                    # Deploy model to cluster
│       --namespace <ns>
│       --instances <n>                 # Worker pool size per node
│       --file-path <path|url>          # Override model file path
│
├── infer                                # Run inference
│       --model-id <id>
│       --input <float,float,...>        # Comma-separated input data
│       --target <host:port>             # Send directly to specific agent
│       --scaling                        # Enable auto-scaling
│
└── version                              # Print client version
```

### 1.3 Global Flags

Every command accepts these global flags:

| Flag | Short | Default | Description |
|---|---|---|---|
| `--endpoint` | `-e` | Config file | Override control plane address |
| `--namespace` | `-n` | Config default | Override namespace |
| `--output` | `-o` | `table` | Output format (`table`, `json`, `yaml`) |
| `--timeout` | `-t` | `10s` | gRPC call timeout |
| `--verbose` | `-v` | `false` | Enable debug logging |

Global flags override the config file for that invocation only.

---

## 2. YAML Parser

The YAML parser enables **declarative, batch operations**. Users define multiple models and their full specifications in a single YAML file, then apply them with `edgectl apply -f`.

### 2.1 YAML Schema

```yaml
# models.yaml
apiVersion: edgernetes.ai/v1
kind: ModelManifest

# Default namespace for all models in this file (can be overridden per model)
namespace: production

models:
  - name: car-price-predictor
    # namespace: staging          # Per-model override
    version: "v1.2"
    file_path: "/models/car_price.onnx"
    model_type: decision_tree
    model_size: 2048
    replicas: 3
    input_format: '{"price": "number", "year": "number", "mileage": "number"}'

  - name: image-classifier
    namespace: ml-vision           # Different namespace from file default
    version: "v2.0"
    file_path: "https://s3.amazonaws.com/models/resnet50.onnx"
    model_type: cnn
    model_size: 102400000
    replicas: 2
    input_format: '{"image": "base64"}'

  - name: fraud-detector
    version: "v1.0"
    file_path: "/models/fraud.onnx"
    model_type: linear
    model_size: 512
    replicas: 1
    input_format: '{"amount": "number", "merchant": "string"}'

  - name: chatbot-llm
    namespace: nlp
    version: "v3.1"
    file_path: "gs://ai-models/chatbot-v3.onnx"
    model_type: llm
    model_size: 5368709120
    replicas: 1
    input_format: '{"prompt": "string", "max_tokens": "number"}'
```

### 2.2 Schema Details

| Field | Type | Required | Description |
|---|---|---|---|
| `apiVersion` | `string` | **Yes** | Must be `edgernetes.ai/v1` |
| `kind` | `string` | **Yes** | Must be `ModelManifest` |
| `namespace` | `string` | No | Default namespace for all models (default: `"default"`) |
| `models` | `[]ModelSpec` | **Yes** | List of model definitions |

**ModelSpec fields:**

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | `string` | **Yes** | Model name. Must be unique within its namespace. |
| `namespace` | `string` | No | Overrides the file-level default namespace |
| `version` | `string` | No | Semantic version string (e.g. `"v1.2"`) |
| `file_path` | `string` | No | Local path or network blob URL for the model file |
| `model_type` | `string` | No | One of: `cnn`, `linear`, `decision_tree`, `llm` |
| `model_size` | `int64` | No | Size of the model file in bytes |
| `replicas` | `int32` | No | Desired number of replicas to deploy |
| `input_format` | `string` | No | JSON schema describing the expected inference input |

### 2.3 Namespace Resolution Order

Namespace is resolved with the following precedence (highest first):

1. **CLI flag** `--namespace` (if provided with `edgectl apply -f`)
2. **Per-model** `namespace` field in the YAML
3. **File-level** `namespace` field in the YAML
4. **Config file** `default_namespace` from `~/.edgectl/config.yaml`
5. **Fallback** `"default"`

### 2.4 Apply Behavior

```bash
# Register all models from the file
edgectl apply -f models.yaml

# Override namespace for all models in the file
edgectl apply -f models.yaml --namespace staging

# Validate the YAML without submitting
edgectl apply -f models.yaml --dry-run
```

The apply command:

1. Parses and validates the YAML against the schema.
2. Resolves the namespace for each model.
3. Validates each model spec (e.g., `name` is non-empty, `model_type` is valid if provided).
4. Iterates through models and calls `RegisterModel` for each.
5. Reports per-model success or failure (does not abort on first error).

**Example output:**

```
NAMESPACE    MODEL                   STATUS
production   car-price-predictor     ✓ registered
ml-vision    image-classifier        ✓ registered
production   fraud-detector          ✗ ALREADY_EXISTS
nlp          chatbot-llm             ✓ registered

Applied 4 models: 3 succeeded, 1 failed
```

---

## 3. gRPC Client Wrapper

The gRPC client wrapper provides a clean Go interface over the raw generated protobuf stubs. It is the single point of contact between the CLI commands and the control plane.

### 3.1 Package Structure

```
internal/client/
├── client.go               # Top-level Client struct, connection lifecycle
├── config.go               # Config file read/write (~/.edgectl/config.yaml)
├── model.go                # ModelRegistryAPI calls
├── node.go                 # NodeRegistryAPI calls
├── deploy.go               # DeployAPI calls
├── transfer.go             # ModelTransferService calls (upload/download)
├── infer.go                # InferAPI calls
├── discovery.go            # DiscoveryAPI calls
└── formatter.go            # Output formatting (table, json, yaml)
```

### 3.2 Client Struct

```go
// internal/client/client.go

package client

import (
    "context"
    "time"

    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"

    deploypb   "github.com/kennethnrk/edgernetes-ai/internal/common/pb/deploy"
    discoverypb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/discovery"
    inferpb    "github.com/kennethnrk/edgernetes-ai/internal/common/pb/infer"
    modelpb    "github.com/kennethnrk/edgernetes-ai/internal/common/pb/model"
    nodepb     "github.com/kennethnrk/edgernetes-ai/internal/common/pb/node"
)

type Client struct {
    conn      *grpc.ClientConn
    timeout   time.Duration

    Models    modelpb.ModelRegistryAPIClient
    Nodes     nodepb.NodeRegistryAPIClient
    Deploy    deploypb.DeployAPIClient
    Transfer  deploypb.ModelTransferServiceClient
    Infer     inferpb.InferAPIClient
    Discovery discoverypb.DiscoveryAPIClient
}

// New creates a new Client connected to the given control plane address.
func New(endpoint string, timeout time.Duration) (*Client, error) {
    conn, err := grpc.NewClient(endpoint,
        grpc.WithTransportCredentials(insecure.NewCredentials()),
    )
    if err != nil {
        return nil, fmt.Errorf("failed to connect to %s: %w", endpoint, err)
    }

    return &Client{
        conn:      conn,
        timeout:   timeout,
        Models:    modelpb.NewModelRegistryAPIClient(conn),
        Nodes:     nodepb.NewNodeRegistryAPIClient(conn),
        Deploy:    deploypb.NewDeployAPIClient(conn),
        Transfer:  deploypb.NewModelTransferServiceClient(conn),
        Infer:     inferpb.NewInferAPIClient(conn),
        Discovery: discoverypb.NewDiscoveryAPIClient(conn),
    }, nil
}

// Close tears down the underlying gRPC connection.
func (c *Client) Close() error {
    return c.conn.Close()
}

// Context returns a context with the configured timeout.
func (c *Client) Context() (context.Context, context.CancelFunc) {
    return context.WithTimeout(context.Background(), c.timeout)
}
```

### 3.3 API Method Mapping

Each CLI command maps to one or more gRPC calls through the wrapper:

| CLI Command | gRPC Service | RPC Method | Notes |
|---|---|---|---|
| `model register` | `ModelRegistryAPI` | `RegisterModel` | Builds `ModelInfo` from flags |
| `model deregister` | `ModelRegistryAPI` | `DeRegisterModel` | |
| `model update` | `ModelRegistryAPI` | `UpdateModel` | |
| `model get` | `ModelRegistryAPI` | `GetModel` | |
| `model list` | `ModelRegistryAPI` | `ListModels` | Client-side namespace filter |
| `model status` | `ModelRegistryAPI` | `GetModelStatus` | Uses `ModelName` with namespace |
| `model nodes` | `ModelRegistryAPI` | `GetNodesByModelName` | Uses `ModelName` with namespace |
| `model upload` | `ModelTransferService` | `UploadModel` | Streaming; sends metadata + chunks |
| `node get` | `NodeRegistryAPI` | `GetNode` | |
| `node list` | `NodeRegistryAPI` | `ListNodes` | |
| `node endpoints` | `DiscoveryAPI` | `GetNodes` | |
| `deploy` | `DeployAPI` | `DeployModel` | |
| `infer` | `InferAPI` | `Infer` | Can target agent directly |
| `apply -f` | `ModelRegistryAPI` | `RegisterModel` × N | One call per model in YAML |

### 3.4 Model Upload (Streaming)

The `model upload` command streams a local `.onnx` file to the control plane:

```go
// internal/client/transfer.go

func (c *Client) UploadModel(filePath string, overrideFilename string) (*deploypb.ModelUploadResponse, error) {
    // 1. Open and stat the file
    // 2. Compute SHA256 hash
    // 3. Open the UploadModel stream
    // 4. Send ModelUploadMetadata as the first message:
    //      { filename, total_size, sha256_hash }
    // 5. Stream file in 2MB chunks with chunk_offset
    // 6. Close send and receive the ModelUploadResponse
    // 7. Progress bar on stderr
}
```

### 3.5 Inference Client

The `infer` command supports sending requests either to the control plane or directly to an agent:

```go
// internal/client/infer.go

func (c *Client) RunInference(modelID string, input []float32, scalingEnabled bool) (*inferpb.InferResponse, error) {
    ctx, cancel := c.Context()
    defer cancel()

    return c.Infer.Infer(ctx, &inferpb.InferRequest{
        ModelId:        modelID,
        InputData:      input,
        ScalingEnabled: scalingEnabled,
        IsForwarded:    false,
    })
}

// RunInferenceOnAgent connects directly to a specific agent for inference,
// bypassing the control plane. Used with --target flag.
func (c *Client) RunInferenceOnAgent(target string, modelID string, input []float32) (*inferpb.InferResponse, error) {
    // 1. Dial the agent directly at target (host:port)
    // 2. Send InferRequest with IsForwarded = false
    // 3. The agent may serve locally or forward once
}
```

---

## 4. Config Manager

The config manager persists client settings to `~/.edgectl/config.yaml` and loads them on each invocation.

### 4.1 Config Struct

```go
// internal/client/config.go

type Config struct {
    Endpoint         string `yaml:"endpoint"`
    DefaultNamespace string `yaml:"default_namespace"`
    OutputFormat     string `yaml:"output_format"`
    TimeoutSeconds   int    `yaml:"timeout_seconds"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() *Config {
    return &Config{
        Endpoint:         "localhost:50051",
        DefaultNamespace: "default",
        OutputFormat:     "table",
        TimeoutSeconds:   10,
    }
}
```

### 4.2 Config File Location

| OS | Path |
|---|---|
| Linux / macOS | `~/.edgectl/config.yaml` |
| Windows | `%USERPROFILE%\.edgectl\config.yaml` |

The config directory is created automatically on the first `edgectl config set-*` command.

---

## 5. Output Formatter

All commands that display data support three output formats, controlled by the `-o` flag or the `output_format` config.

### 5.1 Table Format (default)

Human-readable ASCII tables, using `text/tabwriter`:

```
ID                                    NAME                   NAMESPACE    VERSION  TYPE           REPLICAS
550e8400-e29b-41d4-a716-446655440000  car-price-predictor    production   v1.2     decision_tree  3
6ba7b810-9dad-11d1-80b4-00c04fd430c8  image-classifier       ml-vision    v2.0     cnn            2
```

### 5.2 JSON Format

Machine-readable JSON, piped to `jq` or consumed by scripts:

```json
{
  "models": [
    {
      "id": "550e8400-e29b-41d4-a716-446655440000",
      "name": "car-price-predictor",
      "namespace": "production",
      "version": "v1.2",
      "model_type": "decision_tree",
      "replicas": 3
    }
  ]
}
```

### 5.3 YAML Format

```yaml
models:
  - id: 550e8400-e29b-41d4-a716-446655440000
    name: car-price-predictor
    namespace: production
    version: "v1.2"
    model_type: decision_tree
    replicas: 3
```

---

## 6. Error Handler

The error handler translates gRPC status codes into human-readable CLI output:

```go
// internal/client/errors.go

func FormatError(err error) string {
    st, ok := status.FromError(err)
    if !ok {
        return fmt.Sprintf("error: %v", err)
    }

    switch st.Code() {
    case codes.Unavailable:
        return "error: cannot reach control plane — is it running?"
    case codes.NotFound:
        return fmt.Sprintf("error: not found — %s", st.Message())
    case codes.AlreadyExists:
        return fmt.Sprintf("error: already exists — %s", st.Message())
    case codes.InvalidArgument:
        return fmt.Sprintf("error: invalid input — %s", st.Message())
    case codes.DeadlineExceeded:
        return "error: request timed out"
    default:
        return fmt.Sprintf("error [%s]: %s", st.Code(), st.Message())
    }
}
```

---

## 7. Entry Point & Dependency Wiring

### 7.1 `cmd/client/main.go`

```go
package main

import (
    "os"
    "github.com/kennethnrk/edgernetes-ai/internal/client/cmd"
)

func main() {
    if err := cmd.Execute(); err != nil {
        os.Exit(1)
    }
}
```

### 7.2 Root Command Setup

```go
// internal/client/cmd/root.go

package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
    Use:   "edgectl",
    Short: "Edgernetes-AI command-line client",
    Long:  "edgectl manages models, nodes, and deployments on the Edgernetes-AI control plane.",
}

func Execute() error {
    return rootCmd.Execute()
}

func init() {
    // Global persistent flags
    rootCmd.PersistentFlags().StringP("endpoint",  "e", "", "Control plane address (host:port)")
    rootCmd.PersistentFlags().StringP("namespace", "n", "", "Override namespace")
    rootCmd.PersistentFlags().StringP("output",    "o", "", "Output format (table|json|yaml)")
    rootCmd.PersistentFlags().StringP("timeout",   "t", "", "Request timeout (e.g. 10s)")
    rootCmd.PersistentFlags().BoolP("verbose",     "v", false, "Enable debug logging")

    // Register subcommand groups
    rootCmd.AddCommand(configCmd)
    rootCmd.AddCommand(modelCmd)
    rootCmd.AddCommand(nodeCmd)
    rootCmd.AddCommand(deployCmd)
    rootCmd.AddCommand(inferCmd)
    rootCmd.AddCommand(applyCmd)
    rootCmd.AddCommand(versionCmd)
}
```

---

## 8. File Layout Summary

```
cmd/client/
└── main.go                         # Entry point

internal/client/
├── client.go                       # Client struct, gRPC connection, lifecycle
├── config.go                       # Config read/write (~/.edgectl/config.yaml)
├── errors.go                       # gRPC error → CLI message translation
├── formatter.go                    # Table / JSON / YAML output rendering
├── model.go                        # Model-related gRPC helper methods
├── node.go                         # Node-related gRPC helper methods
├── deploy.go                       # Deploy gRPC helper methods
├── transfer.go                     # Model upload/download streaming logic
├── infer.go                        # Inference gRPC helper methods
├── discovery.go                    # Discovery gRPC helper methods
├── yaml.go                         # YAML manifest parsing & validation
│
└── cmd/                            # Cobra command definitions
    ├── root.go                     # Root command, global flags
    ├── config.go                   # edgectl config set-endpoint|set-namespace|view
    ├── model.go                    # edgectl model register|deregister|update|get|list|status|nodes|upload
    ├── node.go                     # edgectl node get|list|endpoints
    ├── deploy.go                   # edgectl deploy
    ├── infer.go                    # edgectl infer
    ├── apply.go                    # edgectl apply -f
    └── version.go                  # edgectl version
```

---

## 9. Dependencies

| Dependency | Purpose | Version Strategy |
|---|---|---|
| `github.com/spf13/cobra` | CLI framework (command tree, flags, help text) | Latest stable |
| `gopkg.in/yaml.v3` | YAML parsing for model manifests and config | Latest stable |
| `google.golang.org/grpc` | gRPC client transport | Already in `go.mod` |
| `google.golang.org/protobuf` | Protobuf serialization | Already in `go.mod` |

> [!TIP]
> `cobra` and `yaml.v3` are the only new dependencies. Both are widely used, well-maintained, and have no transitive dependency conflicts.

---

## 10. Example Workflows

### 10.1 First-Time Setup

```bash
# 1. Configure the control plane address
edgectl config set-endpoint 192.168.1.10:50051

# 2. Set the default namespace
edgectl config set-namespace production

# 3. Verify
edgectl config view
```

### 10.2 Register a Single Model

```bash
edgectl model register \
  --name car-price-predictor \
  --version v1.2 \
  --file-path /models/car_price.onnx \
  --model-type decision_tree \
  --model-size 2048 \
  --replicas 3 \
  --input-format '{"price":"number","year":"number"}'
```

### 10.3 Bulk Apply from YAML

```bash
# Apply all models in the manifest
edgectl apply -f models.yaml

# Dry-run to validate first
edgectl apply -f models.yaml --dry-run

# Override namespace for all models
edgectl apply -f models.yaml -n staging
```

### 10.4 Inspect Cluster State

```bash
# List all models in a namespace
edgectl model list -n production

# Get detailed model status (replicas breakdown)
edgectl model status car-price-predictor -n production

# List nodes serving a model
edgectl model nodes car-price-predictor -n production

# List all registered nodes
edgectl node list

# Get node details
edgectl node get 6ba7b810-9dad-11d1-80b4-00c04fd430c8 -o json
```

### 10.5 Deploy and Infer

```bash
# Deploy a model with 4 worker instances per node
edgectl deploy 550e8400-e29b-41d4-a716-446655440000 --instances 4

# Run inference via the control plane
edgectl infer --model-id 550e8400-e29b-41d4-a716-446655440000 --input 25000,2019,50000

# Run inference directly on a specific agent
edgectl infer --model-id 550e8400-e29b-41d4-a716-446655440000 \
  --input 25000,2019,50000 \
  --target 192.168.1.42:50052
```

### 10.6 Upload a Model File

```bash
# Upload a local .onnx file to the control plane
edgectl model upload ./my_model.onnx

# Upload with a custom filename
edgectl model upload ./my_model.onnx --filename production_model.onnx
```
