package store

import "time"

type ResourceCapabilities struct {
	Memory         MemoryInfo      `json:"memory"`
	Storage        StorageInfo     `json:"storage"`
	ComputeDevices []ComputeDevice `json:"compute_devices"` // ALL compute: CPU, GPU, NPU, etc.
}

type MemoryType string

const (
	MemoryTypeUnknown MemoryType = "unknown"
	MemoryTypeLPDDR4  MemoryType = "lpddr4"
	MemoryTypeLPDDR4X MemoryType = "lpddr4x"
	MemoryTypeLPDDR5  MemoryType = "lpddr5"
	MemoryTypeLPDDR5X MemoryType = "lpddr5x"
)

type MemoryInfo struct {
	Total int64      `json:"total"`
	Free  int64      `json:"free"`
	Used  int64      `json:"used"`
	Type  MemoryType `json:"type"`
}

type StorageInfo struct {
	Total      int64 `json:"total"`
	Free       int64 `json:"free"`
	Used       int64 `json:"used"`
	ReadSpeed  int64 `json:"read_speed"`
	WriteSpeed int64 `json:"write_speed"`
}

type ComputeDeviceType string

const (
	ComputeDeviceCPU           ComputeDeviceType = "cpu"
	ComputeDeviceGPU           ComputeDeviceType = "gpu"
	ComputeDeviceNPU           ComputeDeviceType = "npu"
	ComputeDeviceTPU           ComputeDeviceType = "tpu"
	ComputeDeviceDSP           ComputeDeviceType = "dsp"
	ComputeDeviceVPU           ComputeDeviceType = "vpu"
	ComputeDeviceFPGA          ComputeDeviceType = "fpga"
	ComputeDeviceIntegratedGPU ComputeDeviceType = "integrated_gpu"
)

type ComputeDevice struct {
	Type         ComputeDeviceType `json:"type"`
	Vendor       string            `json:"vendor"`        // "nvidia", "apple", "qualcomm", "intel"
	Model        string            `json:"model"`         // "H100", "Neural Engine", "Hexagon NPU"
	Memory       int64             `json:"memory_mb"`     // Available memory
	ComputeUnits int               `json:"compute_units"` // Cores/SMs/etc
	TOPS         float64           `json:"tops"`          // Performance metric
	PowerDraw    int               `json:"power_draw_watts,omitempty"`
	IsAvailable  bool              `json:"is_available"`
}

type Status string

const (
	StatusUnknown Status = "unknown"
	StatusOnline  Status = "online"
	StatusOffline Status = "offline"
	StatusError   Status = "error"
)

type NodeMetadata struct {
	OSType       string `json:"os_type"`
	OSVersion    string `json:"os_version"`
	AgentVersion string `json:"agent_version"`
	Hostname     string `json:"hostname"`
}

type NetworkInfo struct {
	Type      string `json:"type"`
	Bandwidth int64  `json:"bandwidth_mbps"`
	IsMetered bool   `json:"is_metered"`
	Latency   int    `json:"latency_ms"`
}

type NodeInfo struct {
	ID                   string               `json:"id"`
	Name                 string               `json:"name"`
	IP                   string               `json:"ip"`
	Port                 int                  `json:"port"`
	Metadata             NodeMetadata         `json:"metadata"`
	NetworkInfo          NetworkInfo          `json:"network_info"`
	ResourceCapabilities ResourceCapabilities `json:"resource_capabilities"`
	Status               Status               `json:"status"`
	RegisteredAt         time.Time            `json:"registered_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
	LastHeartbeat        time.Time            `json:"last_heartbeat"`
	LastActivity         time.Time            `json:"last_activity"`
}
