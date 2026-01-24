package store

import (
	"time"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
)

type ResourceCapabilities struct {
	Memory         MemoryInfo      `json:"memory"`
	Storage        StorageInfo     `json:"storage"`
	ComputeDevices []ComputeDevice `json:"compute_devices"` // ALL compute: CPU, GPU, NPU, etc.
}

type MemoryInfo struct {
	Total int64                `json:"total"`
	Free  int64                `json:"free"`
	Used  int64                `json:"used"`
	Type  constants.MemoryType `json:"type"`
}

type StorageInfo struct {
	Total      int64 `json:"total"`
	Free       int64 `json:"free"`
	Used       int64 `json:"used"`
	ReadSpeed  int64 `json:"read_speed"`
	WriteSpeed int64 `json:"write_speed"`
}

type ComputeDevice struct {
	Type         constants.ComputeDeviceType `json:"type"`
	Vendor       string                      `json:"vendor"`        // "nvidia", "apple", "qualcomm", "intel"
	Model        string                      `json:"model"`         // "H100", "Neural Engine", "Hexagon NPU"
	Memory       int64                       `json:"memory_mb"`     // Available memory
	ComputeUnits int                         `json:"compute_units"` // Cores/SMs/etc
	TOPS         float64                     `json:"tops"`          // Performance metric
	PowerDraw    int                         `json:"power_draw_watts,omitempty"`
	IsAvailable  bool                        `json:"is_available"`
}

type NodeMetadata struct {
	OSType       string `json:"os_type"`
	AgentVersion string `json:"agent_version"`
	Hostname     string `json:"hostname"`
}

type NodeInfo struct {
	ID                   string               `json:"id"`
	Name                 string               `json:"name"`
	IP                   string               `json:"ip"`
	Port                 int                  `json:"port"`
	Metadata             NodeMetadata         `json:"metadata"`
	ResourceCapabilities ResourceCapabilities `json:"resource_capabilities"`
	Status               constants.Status     `json:"status"`
	AssignedModels       []string             `json:"assigned_models"`
	RegisteredAt         time.Time            `json:"registered_at"`
	UpdatedAt            time.Time            `json:"updated_at"`
	LastHeartbeat        time.Time            `json:"last_heartbeat"`
	LastActivity         time.Time            `json:"last_activity"`
}
