# Edgernetes AI

> **AI orchestration for heterogeneous edge devices**

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

## 🎯 Overview

**Edgernetes** is a distributed AI orchestration platform designed specifically for edge computing environments. Unlike traditional orchestration systems that assume homogeneous infrastructure, Edgernetes enables flexible deployment of AI models across diverse edge devices with varying compute capabilities, memory constraints, and network conditions.

### The Problem

Traditional AI orchestration platforms (like Kubernetes) are built for large, homogeneous data centers with abundant resources. They assume:
- Consistent hardware across nodes
- High-bandwidth, low-latency networks
- Unlimited compute and memory resources
- Centralized control and management

**Edge AI scenarios break all these assumptions.** Edge devices are:
- **Heterogeneous**: Different CPUs, GPUs, NPUs, and accelerators
- **Resource-constrained**: Limited memory, storage, and compute
- **Network-variable**: Unreliable, metered, or high-latency connections
- **Distributed**: Geographically dispersed with varying capabilities

Edgernetes bridges this gap by providing intelligent orchestration that understands and adapts to the unique characteristics of each edge device.

## ✨ Key Features

- **🔧 Heterogeneous Device Support**: Automatically detects and manages devices with different compute capabilities (CPUs, GPUs, NPUs, edge accelerators)
- **📊 Intelligent Model Scheduling**: Deploys heavier models to high-compute nodes and lighter models to resource-constrained devices
- **⚡ Low-Latency Communication**: Built on gRPC and Protocol Buffers for efficient control-plane-to-agent communication
- **🔄 Multi-Model Orchestration**: Similar to NVIDIA Triton, enables concurrent serving of multiple AI models across the cluster
- **💾 Persistent State Management**: Embedded, disk-backed key-value store for durable cluster state without external dependencies
- **🌐 Edge-Optimized**: Designed from the ground up for edge computing constraints and requirements

## 🏗️ Architecture

Edgernetes follows a **control-plane/agent architecture**:

```
┌──────────────────────────────────┐
│         Control Plane            │
│  ┌──────────┐  ┌──────────┐      │
│  │ Registry │  │ Scheduler│      │
│  └──────────┘  └──────────┘      │
│  ┌──────────────────────────┐    │
│  │   Persistent Store (WAL) │    │
│  └──────────────────────────┘    │
└──────────────────────────────────┘
           │ gRPC │
           │      │
    ┌──────┴──────┴──────┐
    │                    │
┌───▼───┐          ┌─────▼─────┐
│ Agent │          │   Agent   │
│ Node 1│          │  Node 2   │
│       │          │           │
│ ONNX  │          │  ONNX     │
│Runtime│          │  Runtime  │
└───────┘          └───────────┘
```

### Components

- **Control Plane**: Central orchestrator that manages cluster state, schedules model deployments, and coordinates agents
- **Agent Nodes**: Edge devices that register their capabilities and execute AI model inference
- **Persistent Store**: Embedded WAL-based storage for cluster state (no external database required)
- **gRPC API**: Low-latency communication protocol for control-plane/agent interactions

For detailed architecture documentation, see [docs/architecture.md](docs/architecture.md).

## 🚀 Getting Started

### Prerequisites

- **Go 1.25+**: [Install Go](https://go.dev/doc/install)
- **Protocol Buffers Compiler**: [Install protoc](https://grpc.io/docs/protoc-installation/)

### Building from Source

1. **Clone the repository**:
   ```bash
   git clone https://github.com/kennethnrk/Edgernetes-AI.git
   cd Edgernetes-AI
   ```

2. **Generate Protocol Buffer code**:
   ```bash
   # Generate Go code from .proto files
   ```bash
   protoc --go_out=. --go-grpc_out=. api/proto/*.proto
   ```

3. **Build the control plane**:
   ```bash
   go build -o bin/control-plane ./cmd/control-plane
   ```

4. **Build an agent**:
   ```bash
   go build -o bin/agent ./cmd/agent
   ```

### Running Edgernetes

1. **Start the control plane**:
   ```bash
   ./bin/control-plane
   # Or with custom configuration:
   CONTROL_PLANE_GRPC_ADDR=:50051 STORE_DATA_DIR=./data ./bin/control-plane
   ```

2. **Start agent nodes** (on edge devices):
   ```bash
   ./bin/agent
   ```

## 📁 Project Structure

```
edgernetes-ai/
├── api/                          # API definitions
│   ├── proto/                   # Protocol Buffer definitions
│   │   ├── model.proto          # Model registry API
│   │   └── node.proto           # Node registry API
│   └── openapi/                 # OpenAPI specifications
│       └── api.yaml
│
├── cmd/                         # Application entry points
│   ├── control-plane/           # Control plane server
│   ├── agent/                   # Edge agent
│   └── client/                  # CLI client
│
├── internal/                    # Private application code
│   ├── common/                  # Shared code
│   │   └── pb/                  # Generated protobuf code
│   └── control-plane/           # Control plane implementation
│       ├── api/grpc/            # gRPC handlers
│       ├── controller/          # Business logic
│       └── store/               # Persistent storage
│
├── docs/                        # Documentation
│   ├── architecture.md          # Architecture details
│   ├── blueprint.md             # Project blueprint
│   └── overview.md              # Project overview
│
└── tests/                       # Test files
```

## 🛠️ Technologies

- **Go**: Core implementation language
- **gRPC**: High-performance RPC framework for control-plane/agent communication
- **Protocol Buffers**: Efficient serialization for API contracts
- **ONNX Runtime**: Model inference engine (planned integration)
- **Embedded WAL Store**: Custom persistent storage for cluster state

## 📚 Documentation

- [Architecture Documentation](docs/architecture.md) - Detailed system architecture
- [Project Blueprint](docs/blueprint.md) - Project structure and design
- [Overview](docs/overview.md) - High-level project overview

## 🤝 Contributing

Contributions are welcome! This is an active project with many areas for improvement:

- Model scheduling algorithms
- Additional runtime support (TensorFlow Lite, PyTorch Mobile, etc.)
- Network-aware deployment strategies
- Resource optimization and auto-scaling
- Monitoring and observability

Please feel free to open issues or submit pull requests.

## 📝 License

This project is licensed under the Apache License 2.0 - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by the **Qualcomm Edge AI Hackathon** experience
- Built with insights from Kubernetes, NVIDIA Triton, and modern edge computing practices
- Designed for the future of distributed AI at the edge

---

**Built with ❤️ for the edge AI community**
