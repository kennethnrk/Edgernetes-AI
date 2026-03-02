# Edge AI Inference Pipeline Architecture

This document details the inference pipeline architecture for the `edgernetes-ai` agent. The architecture is designed to handle Machine Learning execution securely, reliably, and efficiently on edge devices, which often have limited RAM and CPU capabilities. 

## Architectural Goals
1. **Thread Safety**: Ensure multiple concurrent inference requests do not corrupt memory or crash the low-level ONNX C++ bindings.
2. **Prevent Out-Of-Memory (OOM) Errors**: Strict constraint of concurrency to avoid sudden spikes in RAM usage when traffic spikes occur on constrained edge nodes.
3. **High Throughput (Zero-Reload)**: Preload and cache models in memory so that the disk I/O cost of loading a model is incurred only once per deployment, not per request.
4. **Configurable Extensibility**: Allow the Control Plane to dictate exactly how many parallel workers an edge node should dedicate to a specific model.

---

## 1. The Worker Queue Pattern
To achieve these goals, the system relies on the **Actor Model / Worker Queue** concurrency pattern. It completely isolates the ONNX Runtime execution environment so that only one goroutine has access to a model's internal state at a time, whilst buffering incoming requests safely using Go Channels.

### Key Components
- **`InferenceJob`**: A struct containing the raw feature data from the incoming request, alongside bidirectional `Result` and `Err` channels so the worker can map the output prediction straight back to the original calling HTTP/gRPC thread synchronously.
- **`ModelWorker`**: Maintains the preloaded `*ort.DynamicAdvancedSession` interface alongside the buffered job queue channel.
- **`WorkerRegistry`**: A globally safe hashmap (`sync.RWMutex`) mapping `replicaID` strings to their respective isolated `ModelWorker` pools.

---

## 2. API Flow

1. **Deploy Phase (`StartModelWorkers`)** 
   - When a model is assigned to an agent, a corresponding worker queue is started. 
   - The ONNX Model is loaded from disk once into a reusable `ort.DynamicAdvancedSession`.
   - Based on the `instance_count` provided by the Control Plane, N identical goroutines are spawned, each running an infinite `select` block listening to the worker queue.
   - The memory structure is cached globally in the registry.

2. **Inference Phase (`ModelInference`)**
   - An external request arrives (via REST, sensor loop, or gRPC).
   - An `InferenceJob` struct is created, bundling the `InputData` and unbuffered Response channels.
   - The Job is submitted non-blockingly to the replica's specific bounded channel Queue.
   - The calling goroutine blocks on a `select` statement awaiting the result or a 5-second timeout.
   - A free background worker picks the job off the queue.
   - **Data Mapping**: Tensors (`ort.Value`) are uniquely mapped to the data in the job. This separation of tensors per-job run ensures thread-safety.
   - `session.Run` is executed natively.
   - The response float is pushed back into the `Job.Result` channel, cleanly terminating the blocking call in the HTTP handler.

3. **Termination Phase (`StopModelWorkers`)**
   - When a deployment is retracted, a `quit` signal forces all active pipeline readers to cleanly return.
   - The `ort.Session` is gracefully destroyed (cleaning up C++ memory allocations) and deleted from the Go Registry.

---

## 3. Protocol Modifications

To support configurable concurrency, the system's Protocol Buffer definitions were expanded:
- **`deploy.proto`**: `DeployModelRequest` now receives `int32 instance_count = 7;` securely instructing the agent on the size of the worker queue to construct.
- **`heartbeat.proto`**: `ModelReplicaDetails` emits `int32 instance_count = 11;` back to the Control Plane periodically, confirming the parallel state matches the desired deployment topology.

## 4. Why `DynamicAdvancedSession`?
The basic `ort.Session` in `onnxruntime_go` binds strict static tensors on initialization. While this works for single-file scripts, it is disastrous for highly concurrent web-servers because multiple goroutines would overwrite the internal C++ tensor memory spaces simultaneously. 

By utilizing `ort.DynamicAdvancedSession`, we can cache the computationally heavy *Model Graph* in memory, while efficiently destroying and recreating the tiny input and output tensor arrays (`ort.NewTensor`) uniquely per job request. This ensures total thread isolation while maintaining peak zero-reload execution speeds.
