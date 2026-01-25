package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"runtime"

	"github.com/kennethnrk/edgernetes-ai/internal/agent"
	grpcagent "github.com/kennethnrk/edgernetes-ai/internal/agent/api/grpc"
	"github.com/kennethnrk/edgernetes-ai/internal/agent/utils"
	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
)

func main() {

	controlPlaneAddress := flag.String("addr", "localhost:50051", "The address of the control plane")
	nodeName := flag.String("n", "", "The name of the node (defaults to hostname-random)")
	flag.Parse()

	log.Println("Agent started")

	//get my IP address
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

	agentInfo := &agent.Agent{
		ID:   "",
		Name: finalNodeName,
		IP:   ipAddress,
		Port: 50051,
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

	// Register with control-plane
	if err := grpcagent.RegisterWithControlPlane(*controlPlaneAddress, agentInfo); err != nil {
		log.Fatalf("Failed to register with control-plane: %v", err)
	}

	log.Printf("Agent registered successfully with node ID: %s", agentInfo.ID)
}
