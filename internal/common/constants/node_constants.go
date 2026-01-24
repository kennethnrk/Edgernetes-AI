package constants

type MemoryType string

const (
	MemoryTypeUnknown MemoryType = "unknown"
	MemoryTypeLPDDR4  MemoryType = "lpddr4"
	MemoryTypeLPDDR4X MemoryType = "lpddr4x"
	MemoryTypeLPDDR5  MemoryType = "lpddr5"
	MemoryTypeLPDDR5X MemoryType = "lpddr5x"
)

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


type Status string

const (
	StatusUnknown Status = "unknown"
	StatusOnline  Status = "online"
	StatusOffline Status = "offline"
	StatusError   Status = "error"
)