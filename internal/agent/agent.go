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

	"github.com/kennethnrk/edgernetes-ai/internal/agent/utils"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

type ModelReplicaDetails struct {
	ID           string                       `json:"id"`       // This is the replica ID
	ModelID      string                       `json:"model_id"` // This is the model ID
	Name         string                       `json:"name"`
	Version      string                       `json:"version"`
	FilePath     string                       `json:"file_path"`
	ModelType    constants.ModelType          `json:"model_type"`
	ModelSize    int64                        `json:"model_size"`
	Status       constants.ModelReplicaStatus `json:"status"`
	ErrorCode    int                          `json:"error_code"`
	ErrorMessage string                       `json:"error_message"`
	LogFile      string                       `json:"log_file"`
}

type Agent struct {
	ID                   string                     `json:"id"`
	Name                 string                     `json:"name"`
	IP                   string                     `json:"ip"`
	Port                 int                        `json:"port"`
	Metadata             store.NodeMetadata         `json:"metadata"`
	ResourceCapabilities store.ResourceCapabilities `json:"resource_capabilities"`
	AssignedModels       []ModelReplicaDetails      `json:"assigned_models"`
}

func (a *Agent) AssignModel(model ModelReplicaDetails) error {
	if slices.ContainsFunc(a.AssignedModels, func(m ModelReplicaDetails) bool { return m.ID == model.ID }) {
		return errors.New("model already assigned")
	}
	a.AssignedModels = append(a.AssignedModels, model)
	return nil
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
	}
	return agent
}
