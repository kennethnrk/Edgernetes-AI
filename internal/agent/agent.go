package agent

import (
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"runtime"
	"slices"
	"sync"
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/agent/utils"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/kennethnrk/edgernetes-ai/internal/agent/balancer"
	"github.com/kennethnrk/edgernetes-ai/internal/agent/runway"
	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	heartbeatpb "github.com/kennethnrk/edgernetes-ai/internal/common/pb/heartbeat"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

type ModelReplicaDetails struct {
	ID            string                       `json:"id"`       // This is the replica ID
	ModelID       string                       `json:"model_id"` // This is the model ID
	Name          string                       `json:"name"`
	Version       string                       `json:"version"`
	FilePath      string                       `json:"file_path"`
	ModelType     constants.ModelType          `json:"model_type"`
	ModelSize     int64                        `json:"model_size"`
	Status        constants.ModelReplicaStatus `json:"status"`
	ErrorCode     int                          `json:"error_code"`
	ErrorMessage  string                       `json:"error_message"`
	LogFile       string                       `json:"log_file"`
	InstanceCount int                          `json:"instance_count"`
}

type Agent struct {
	ID                   string                     `json:"id"`
	Name                 string                     `json:"name"`
	IP                   string                     `json:"ip"`
	Port                 int                        `json:"port"`
	Metadata             store.NodeMetadata         `json:"metadata"`
	ResourceCapabilities store.ResourceCapabilities `json:"resource_capabilities"`
	AssignedModels       []ModelReplicaDetails      `json:"assigned_models"`

	endpointCache map[string][]*heartbeatpb.EndpointDetail
	endpointMu    sync.RWMutex
	lb            balancer.LoadBalancer

	mu            sync.RWMutex
	LastHeartbeat time.Time `json:"last_heartbeat"`
}

func (a *Agent) UpdateEndpoints(endpoints []*heartbeatpb.ServiceEndpoints) {
	newCache := make(map[string][]*heartbeatpb.EndpointDetail)
	for _, se := range endpoints {
		newCache[se.ModelId] = se.Endpoints
	}
	a.endpointMu.Lock()
	a.endpointCache = newCache
	a.endpointMu.Unlock()
}

func (a *Agent) GetEndpoints(modelID string) []*heartbeatpb.EndpointDetail {
	a.endpointMu.RLock()
	defer a.endpointMu.RUnlock()
	return a.endpointCache[modelID]
}

func (a *Agent) AssignModel(model ModelReplicaDetails) error {
	if slices.ContainsFunc(a.AssignedModels, func(m ModelReplicaDetails) bool { return m.ID == model.ID }) {
		return errors.New("model already assigned")
	}
	a.AssignedModels = append(a.AssignedModels, model)
	return nil
}

func (a *Agent) UpdateLastHeartbeat() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.LastHeartbeat = time.Now()
}

func (a *Agent) IsHeartbeatStale(timeout time.Duration) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return time.Since(a.LastHeartbeat) > timeout
}

func GetAgentInfo(nodeName *string) *Agent {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatalf("failed to get IP address: %v", err)
	}
	ipAddress := conn.LocalAddr().(*net.UDPAddr).IP.String()
	conn.Close()

	//get memory info
	memfo, err := mem.VirtualMemory()
	if err != nil {
		log.Fatalf("failed to get memory info: %v", err)
	}

	//get disk info
	diskfo, err := disk.Usage("/")
	if err != nil {
		log.Fatalf("failed to get disk info: %v", err)
	}

	//get memory type
	memoryType := utils.GetMemoryType()
	if memoryType == constants.MemoryTypeUnknown {
		log.Printf("Warning: Could not detect memory type, defaulting to unknown")
	}

	//get cpu info
	cpuInfo, err := cpu.Info()
	if err != nil {
		log.Fatalf("failed to get cpu info: %v", err)
	}

	//get gpu info
	gpuInfo := utils.GetGPUInfo()
	if len(gpuInfo) == 0 {
		log.Printf("Warning: No GPUs detected")
	} else {
		log.Printf("Detected %d GPU(s)", len(gpuInfo))
	}

	//get hostname
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("failed to get hostname: %v", err)
	}

	// Determine node name
	var finalNodeName string
	if *nodeName != "" {
		finalNodeName = *nodeName
	} else {
		// Generate random number (0-9999)
		randomNum, err := rand.Int(rand.Reader, big.NewInt(10000))
		if err != nil {
			log.Fatalf("failed to generate random number: %v", err)
		}
		finalNodeName = fmt.Sprintf("%s-%04d", hostname, randomNum.Int64())
	}

	// Build compute devices list starting with CPU
	computeDevices := []store.ComputeDevice{
		{
			Type:         constants.ComputeDeviceCPU,
			Vendor:       cpuInfo[0].VendorID,
			Model:        cpuInfo[0].ModelName,
			Memory:       0,
			ComputeUnits: int(cpuInfo[0].Cores),
			TOPS:         float64(cpuInfo[0].Mhz / 1000),
			PowerDraw:    0,
			IsAvailable:  true,
		},
	}

	// Append all detected GPUs
	computeDevices = append(computeDevices, gpuInfo...)

	agent := &Agent{
		ID:   "",
		Name: finalNodeName,
		IP:   ipAddress,
		Port: 50052, // default agent heartbeat port (overridden in main from AGENT_GRPC_ADDR)
		Metadata: store.NodeMetadata{
			OSType:       runtime.GOOS,
			AgentVersion: "1.0.0",
			Hostname:     hostname,
		},
		ResourceCapabilities: store.ResourceCapabilities{
			Memory: store.MemoryInfo{
				Total: int64(memfo.Total / 1024 / 1024),
				Free:  int64(memfo.Free / 1024 / 1024),
				Used:  int64(memfo.Used / 1024 / 1024),
				Type:  memoryType,
			},
			Storage: store.StorageInfo{
				Total: int64(diskfo.Total / 1024 / 1024),
				Free:  int64(diskfo.Free / 1024 / 1024),
				Used:  int64(diskfo.Used / 1024 / 1024),
			},
			ComputeDevices: computeDevices,
		},
		endpointCache: make(map[string][]*heartbeatpb.EndpointDetail),
		lb:            balancer.NewWeightedRoundRobin(),
		LastHeartbeat: time.Now(),
	}
	return agent
}

// HandleInfer routes the inference request locally or forwards it based on the endpoint cache.
func (a *Agent) HandleInfer(modelID string, inputData []float32, isForwarded bool, scalingEnabled bool) (float32, error) {
	// First check if the current agent has the model
	if slices.ContainsFunc(a.AssignedModels, func(m ModelReplicaDetails) bool { return m.ModelID == modelID }) {
		var replicaID string
		for _, m := range a.AssignedModels {
			if m.ModelID == modelID {
				replicaID = m.ID
				break
			}
		}

		result, err := runway.ModelInference(replicaID, inputData, scalingEnabled)
		if err != nil {
			return 0, fmt.Errorf("local inference failed: %v", err)
		}
		return result, nil
	}

	// Loop detection: do not forward an already forwarded request
	if isForwarded {
		return 0, fmt.Errorf("model %s not available on this node and request was already forwarded", modelID)
	}

	// Not local, check cache and forward to a peer
	endpoints := a.GetEndpoints(modelID)
	if len(endpoints) == 0 {
		return 0, fmt.Errorf("no healthy peers known for model %s", modelID)
	}

	target, err := a.lb.Pick(endpoints)
	if err != nil {
		return 0, fmt.Errorf("failed to select peer: %v", err)
	}

	return a.forwardInfer(target, modelID, inputData, scalingEnabled)
}
