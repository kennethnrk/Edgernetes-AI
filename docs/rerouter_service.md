# Rerouter Service: Inference Load Balancing

This document describes the architecture for distributing inference requests across edge nodes in the Edgernetes-AI cluster. The chosen design gives every agent the ability to accept any inference request and transparently route it to the correct node — without the control plane ever touching inference traffic.

---

## Problem Statement

When a model is deployed across multiple nodes (replicas), a client needs a way to send inference requests without knowing (or caring about) which specific node holds the model. The routing mechanism must:

1. **Not bottleneck on a single component** — inference throughput should scale with the number of nodes.
2. **Tolerate control plane downtime** — inference must continue even if the control plane is unreachable.
3. **Minimize network hops** — avoid unnecessary forwarding when possible.
4. **Remain simple for the client** — the client should not need a custom SDK or intimate cluster knowledge.

---

## Alternate Approaches Considered

### Approach 1: Control Plane as Reverse Proxy

The simplest approach: every inference request goes to the control plane, which selects a backend agent and forwards the request.

```
Client → Control Plane → Agent (runs inference) → Control Plane → Client
```

**Pros:**
- Dead simple for the client — one static address.
- Easy to implement — the control plane already knows all replica placements.

**Rejected because:**
- **Single bottleneck.** All inference traffic flows through one process. Throughput is limited by the control plane's CPU, memory, and network bandwidth regardless of how many agents are available.
- **Single point of failure.** If the control plane goes down, all inference stops — even though the agents and models are perfectly healthy.
- **Extra latency.** Every request takes two extra network hops (client→CP and CP→client) plus serialization overhead at the proxy layer.

This violates Kubernetes' core networking principle: the control plane manages metadata and orchestration but **never** sits in the data path.

---

### Approach 2: Dedicated Gateway Nodes

Deploy one or more lightweight "gateway" processes (separate from both the control plane and agents). Gateways maintain a local copy of the endpoint table and forward inference requests to the appropriate agent.

```
Client → Gateway → Agent (runs inference) → Gateway → Client
```

**Pros:**
- Removes the control plane from the data path.
- Gateways are stateless and horizontally scalable.
- Gives clients a small, stable set of addresses.

**Rejected because:**
- **Operational complexity.** Introduces a new component to deploy, monitor, and scale — on top of the control plane and agents that already exist.
- **Still an extra hop.** Unless the gateway happens to be co-located with the serving agent, there is always a forwarding step.
- **Underutilizes agents.** Agents already have all the information needed to route requests (they receive the endpoint table via heartbeats). Adding a separate gateway duplicates this capability in a new process.

A valid approach for external-facing production setups, but heavier than needed for Edgernetes-AI's edge-native architecture.

---

### Approach 3: Client-Side Load Balancing (Pure)

The client receives the full list of endpoints for a given model from the control plane and routes directly to the correct agent. This is how gRPC's built-in name resolution and load balancing works (e.g., using a custom resolver).

```
Client → Agent (runs inference) → Client
```

**Pros:**
- Zero extra hops — the client connects directly to the serving agent.
- No intermediate component to bottleneck or fail.

**Rejected because:**
- **Requires a smart client.** The client must implement (or import) a custom resolver, health checking, and load balancing strategy. This pushes complexity onto every consumer.
- **Stale endpoint data.** If the client caches endpoints and a node goes down, the client will keep sending requests to a dead endpoint until it re-resolves.
- **Tight coupling.** The client must understand Edgernetes-AI internals (model IDs to endpoint mappings), violating separation of concerns.

---

## Chosen Approach: Agent-Local Rerouting

Every agent in the cluster can accept an inference request for **any** model. If the agent has the model locally, it serves the request directly. If it does not, it **forwards** the request to an agent that does, using its locally cached endpoint table.

```
Client → Agent A (any agent)
          │
          ├─ Has model locally? → Run inference, return result
          │
          └─ Does not have model? → Forward to Agent B (has model)
                                      → Return result to client
```

### Why This Approach

| Requirement | How it is met |
|---|---|
| No bottleneck | Clients spread requests across all agents; no single choke point |
| Control plane independence | Agents cache endpoint tables locally; inference works without the CP |
| Minimal hops | If the client hits a node with the model, zero forwarding occurs |
| Simple client | Client only needs a list of agent addresses — no SDK, no resolver |

This mirrors how **Kubernetes Services** work internally: `kube-proxy` on every node intercepts traffic and routes it using locally programmed rules, without involving the API server. In Edgernetes-AI, each agent plays the role of `kube-proxy` — it holds the routing table and makes forwarding decisions locally.

---

## Architecture

```
                  Client
                  (knows all agent addresses)
                  picks one at random or round-robin
                   │          │           │
                   ▼          ▼           ▼
             ┌──────────┬──────────┬──────────┐
             │ Agent A  │ Agent B  │ Agent C  │
             │          │          │          │
             │ Endpoint │ Endpoint │ Endpoint │
             │ Cache:   │ Cache:   │ Cache:   │
             │ model-x→ │ model-x→ │ model-x→ │
             │  [B, C]  │  [B, C]  │  [B, C]  │
             │          │          │          │
             │ Runs:    │ Runs:    │ Runs:    │
             │ model-y  │ model-x  │ model-x  │
             └──────────┴──────────┴──────────┘

All agents have the SAME endpoint cache.
Any agent can serve any model request via forwarding.
```

---

## Component Details

### 1. Endpoint Cache

Each agent maintains an in-memory map of model IDs to endpoints, synchronized from the control plane via heartbeat responses.

```go
type Agent struct {
    // ... existing fields ...

    endpointCache map[string][]Endpoint  // model_id → []Endpoint
    endpointMu    sync.RWMutex
}

type Endpoint struct {
    NodeID    string
    ReplicaID string
    IP        string
    Port      int
    Healthy   bool
    Weight    float64  // Derived from node TOPS / memory
}
```

The cache is **read-heavy** (read on every inference request, written only on heartbeat updates), so it is protected by an `RWMutex`.

### 2. Heartbeat Sync

The `RequestHeartbeatResponse` will be extended to include the cluster-wide endpoint table:

```protobuf
message RequestHeartbeatResponse {
    string nodeID = 1;
    repeated ModelReplicaDetails ModelReplicas = 2;
    bool success = 3;
    repeated ServiceEndpoints service_endpoints = 4;  // NEW
}

message ServiceEndpoints {
    string model_id = 1;
    repeated EndpointDetail endpoints = 2;
}

message EndpointDetail {
    string node_id = 1;
    string replica_id = 2;
    string ip = 3;
    int32 port = 4;
    bool healthy = 5;
    double weight = 6;
}
```

On each heartbeat response, the agent replaces its local `endpointCache` with the fresh data from the control plane. If the control plane becomes unreachable, the agent continues routing with its last known cache.

### 3. Inference Handler (Rerouting Logic)

Every agent exposes the same inference endpoint. The handler logic:

```go
func (a *Agent) HandleInfer(modelID string, input []byte, isForwarded bool) ([]byte, error) {
    // Step 1: Serve locally if possible (zero hops)
    if replica, ok := a.getLocalReplica(modelID); ok {
        return replica.Infer(input)
    }

    // Step 2: Prevent forwarding loops
    if isForwarded {
        return nil, fmt.Errorf("model %s not available on this node", modelID)
    }

    // Step 3: Forward to a node that has the model
    a.endpointMu.RLock()
    endpoints := a.endpointCache[modelID]
    a.endpointMu.RUnlock()

    if len(endpoints) == 0 {
        return nil, fmt.Errorf("no healthy endpoints for model %s", modelID)
    }

    target := a.lb.Pick(filterHealthy(endpoints))
    return a.forwardInfer(target.IP, target.Port, modelID, input, true)
}
```

Key behaviors:
- **Local-first**: if the agent has the model, it runs inference directly with no network hop.
- **Forward-once**: the `isForwarded` flag prevents infinite loops. A request is forwarded at most once.
- **Transparent**: from the client's perspective, every agent behaves identically regardless of which models it hosts.

### 4. Load Balancing Strategies

The load balancer is an interface, allowing pluggable strategies:

```go
type LoadBalancer interface {
    Pick(endpoints []Endpoint) (*Endpoint, error)
}
```

| Strategy | Description | Best for |
|---|---|---|
| Round Robin | Cycles through endpoints sequentially | Uniform nodes |
| Weighted Round Robin | Weights by node capability (`TOPS`, memory) | Heterogeneous edge devices |
| Least Connections | Picks endpoint with fewest in-flight requests | Variable request latencies |
| Random | Random selection from healthy pool | Simplicity |

For heterogeneous edge clusters, **Weighted Round Robin** is the default. Weights are derived from the node's `ComputeDevice.TOPS` and available memory, which are already reported during node registration.

### 5. Client Endpoint Discovery

When a client first connects, it retrieves the list of all agent addresses from the control plane:

```
GET /api/v1/nodes → [agentA:8080, agentB:8080, agentC:8080]
```

The client caches this list and distributes requests across nodes using simple round-robin or random selection. The client does not need to know which node has which model — any node will accept and route any request.

---

## Failure Scenarios

| Scenario | Behavior |
|---|---|
| **Client's chosen agent is down** | Client retries on the next agent in its list |
| **Target agent goes down after forwarding starts** | Entry agent returns error; client retries on another agent |
| **Control plane is down** | Agents continue routing with last-known endpoint cache; new deployments are blocked but inference continues |
| **Model replica becomes unhealthy** | Next heartbeat marks the endpoint as unhealthy; agents stop routing to it |
| **Stale endpoint cache** | At worst, one heartbeat interval of stale data (~15-30s); forwarded request fails and client retries on another agent |

---

## Future Enhancements

- **Sticky sessions**: route requests from the same client to the same agent for cache-warm benefits (useful for LLM context windows).
- **Latency-aware routing**: prefer the lowest-latency endpoint rather than pure round-robin, using agent-to-agent ping measurements.
- **Connection pooling**: maintain persistent gRPC connections between agents to avoid per-request connection overhead.
- **Request queuing**: if all endpoints for a model are saturated, queue the request briefly rather than immediately failing.
