package utils

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
	"github.com/kennethnrk/edgernetes-ai/internal/control-plane/store"
)

// GPUSpec represents GPU specifications from the lookup table
type GPUSpec struct {
	ComputeUnits int     `json:"compute_units"`
	TOPS         float64 `json:"tops"`
	MemoryMB     int64   `json:"memory_mb"`
}

var gpuSpecsCache map[string]map[string]GPUSpec

// loadGPUSpecs loads GPU specifications from the JSON file
func loadGPUSpecs() (map[string]map[string]GPUSpec, error) {
	if gpuSpecsCache != nil {
		return gpuSpecsCache, nil
	}

	// Get the directory where this file is located
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	specsPath := filepath.Join(dir, "gpu_specs.json")

	data, err := os.ReadFile(specsPath)
	if err != nil {
		return nil, err
	}

	var specs map[string]map[string]GPUSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		return nil, err
	}

	gpuSpecsCache = specs
	return specs, nil
}

// normalizeModelName normalizes GPU model names for better matching
func normalizeModelName(model string) string {
	// Remove common prefixes/suffixes and normalize
	normalized := strings.ToLower(strings.TrimSpace(model))
	// Remove (TM), (R), etc.
	normalized = strings.ReplaceAll(normalized, "(tm)", "")
	normalized = strings.ReplaceAll(normalized, "(r)", "")
	normalized = strings.ReplaceAll(normalized, "®", "")
	normalized = strings.ReplaceAll(normalized, "™", "")
	// Remove extra spaces
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

// lookupGPUSpec looks up GPU specifications by vendor and model
func lookupGPUSpec(vendor, model string) *GPUSpec {
	specs, err := loadGPUSpecs()
	if err != nil {
		return nil
	}

	vendorSpecs, ok := specs[vendor]
	if !ok {
		return nil
	}

	// Try exact match first
	if spec, ok := vendorSpecs[model]; ok {
		return &spec
	}

	// Normalize the input model
	modelNormalized := normalizeModelName(model)

	// Try normalized matching
	for key, spec := range vendorSpecs {
		keyNormalized := normalizeModelName(key)
		
		// Exact normalized match
		if modelNormalized == keyNormalized {
			return &spec
		}
		
		// Check if model contains key or key contains model (for partial matches)
		if strings.Contains(modelNormalized, keyNormalized) || strings.Contains(keyNormalized, modelNormalized) {
			// Prefer longer matches (more specific)
			if len(keyNormalized) > 5 { // Only use meaningful matches
				return &spec
			}
		}
	}

	// Try partial matching for common patterns
	// For AMD Radeon Graphics, try matching "AMD Radeon" patterns
	if vendor == "amd" && strings.Contains(modelNormalized, "radeon") {
		// Check for integrated graphics patterns
		if strings.Contains(modelNormalized, "graphics") && !strings.Contains(modelNormalized, "rx") {
			// Try AMD Radeon Graphics entries
			if spec, ok := vendorSpecs["AMD Radeon(TM) Graphics"]; ok {
				return &spec
			}
			if spec, ok := vendorSpecs["AMD Radeon Graphics"]; ok {
				return &spec
			}
		}
	}

	return nil
}

// enhanceGPUWithSpecs enhances a GPU device with specifications from lookup table
func enhanceGPUWithSpecs(gpu *store.ComputeDevice) {
	spec := lookupGPUSpec(gpu.Vendor, gpu.Model)
	if spec == nil {
		return
	}

	// Only fill in missing values (don't overwrite if already set)
	if gpu.ComputeUnits == 0 && spec.ComputeUnits > 0 {
		gpu.ComputeUnits = spec.ComputeUnits
	}
	if gpu.TOPS == 0 && spec.TOPS > 0 {
		gpu.TOPS = spec.TOPS
	}
	// Only use lookup memory if detected memory is 0 or very small
	if (gpu.Memory == 0 || gpu.Memory < 100) && spec.MemoryMB > 0 {
		gpu.Memory = spec.MemoryMB
	}
}

// GetGPUInfo detects all GPUs/accelerators available on the system
func GetGPUInfo() []store.ComputeDevice {
	switch runtime.GOOS {
	case "windows":
		return getGPUInfoWindows()
	case "linux":
		return getGPUInfoLinux()
	case "darwin":
		return getGPUInfoDarwin()
	default:
		return []store.ComputeDevice{}
	}
}

// getGPUInfoWindows detects GPUs on Windows using nvidia-smi, rocm-smi, and PowerShell
func getGPUInfoWindows() []store.ComputeDevice {
	var gpus []store.ComputeDevice

	// First, try to detect NVIDIA GPUs using nvidia-smi
	nvidiaGPUs := detectNVIDIAWindows()
	gpus = append(gpus, nvidiaGPUs...)

	// Try to detect AMD GPUs using rocm-smi (if available)
	amdGPUs := detectAMDWindows()
	gpus = append(gpus, amdGPUs...)

	// Then, try to detect other GPUs using PowerShell
	otherGPUs := detectOtherGPUsWindows()
	gpus = append(gpus, otherGPUs...)

	return gpus
}

// detectAMDWindows detects AMD GPUs using rocm-smi (if available)
func detectAMDWindows() []store.ComputeDevice {
	var gpus []store.ComputeDevice

	// Try rocm-smi to get AMD GPU information
	cmd := exec.Command("rocm-smi", "--showproductname", "--showmeminfo", "vram", "--showid")
	output, err := cmd.Output()
	if err != nil {
		// rocm-smi not available
		return gpus
	}

	// Parse rocm-smi output (format varies, this is a basic parser)
	lines := strings.Split(string(output), "\n")
	var currentGPU *store.ComputeDevice

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// rocm-smi output parsing (simplified - may need adjustment based on actual output)
		if strings.Contains(line, "Card series:") || strings.Contains(line, "Card model:") {
			if currentGPU != nil {
				enhanceGPUWithSpecs(currentGPU)
				gpus = append(gpus, *currentGPU)
			}
			currentGPU = &store.ComputeDevice{
				Type:        constants.ComputeDeviceGPU,
				Vendor:      "amd",
				IsAvailable: true,
			}
			// Extract model name
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				currentGPU.Model = strings.TrimSpace(parts[1])
			}
		} else if currentGPU != nil && strings.Contains(line, "vram") {
			// Try to extract memory info
			// This is a simplified parser - rocm-smi output format may vary
		}
	}

	if currentGPU != nil {
		enhanceGPUWithSpecs(currentGPU)
		gpus = append(gpus, *currentGPU)
	}

	return gpus
}

// detectNVIDIAWindows detects NVIDIA GPUs using nvidia-smi
func detectNVIDIAWindows() []store.ComputeDevice {
	var gpus []store.ComputeDevice

	// Try nvidia-smi --query-gpu=name,memory.total,compute_cap --format=csv,noheader
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.total,compute_cap", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		// nvidia-smi not available or no NVIDIA GPUs
		return gpus
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse CSV format: "GPU Name, 8192 MiB, 8.9"
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		gpu := store.ComputeDevice{
			Type:         constants.ComputeDeviceGPU,
			Vendor:       "nvidia",
			Model:        strings.TrimSpace(parts[0]),
			IsAvailable:  true,
			ComputeUnits: 0, // nvidia-smi doesn't provide this directly
			TOPS:         0,  // Would need model-specific lookup
		}

		// Parse memory (format: " 8192 MiB")
		if len(parts) >= 2 {
			memStr := strings.TrimSpace(parts[1])
			memStr = strings.TrimSuffix(memStr, " MiB")
			memStr = strings.TrimSpace(memStr)
			if memMB, err := strconv.ParseInt(memStr, 10, 64); err == nil {
				gpu.Memory = memMB
			}
		}

		gpus = append(gpus, gpu)
	}

	return gpus
}

// detectOtherGPUsWindows detects non-NVIDIA GPUs using PowerShell (more reliable than wmic)
func detectOtherGPUsWindows() []store.ComputeDevice {
	var gpus []store.ComputeDevice

	// Use PowerShell to get video controller information (more reliable parsing)
	psScript := `Get-WmiObject -Class Win32_VideoController | Where-Object { $_.Name -ne $null } | ForEach-Object { "$($_.Name)|$($_.AdapterRAM)|$($_.Status)" }`
	cmd := exec.Command("powershell", "-Command", psScript)
	output, err := cmd.Output()
	if err != nil {
		return gpus
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Format: "GPU Name|AdapterRAM|Status"
		parts := strings.Split(line, "|")
		if len(parts) < 2 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		if name == "" {
			continue
		}

		// Validate that name is not a number (should be a GPU model name)
		if _, err := strconv.ParseInt(name, 10, 64); err == nil {
			// This is a number, not a name - skip it
			continue
		}

		// Skip if already detected as NVIDIA (avoid duplicates)
		if strings.Contains(strings.ToLower(name), "nvidia") {
			continue
		}

		gpu := store.ComputeDevice{
			Type:        constants.ComputeDeviceGPU,
			Vendor:      detectVendorFromName(name),
			Model:       name,
			IsAvailable: true,
		}

		// Parse memory (in bytes, convert to MB)
		if len(parts) >= 2 {
			memStr := strings.TrimSpace(parts[1])
			// Check if it's a valid number (not empty and not "0")
			if memStr != "" && memStr != "0" {
				if memBytes, err := strconv.ParseInt(memStr, 10, 64); err == nil && memBytes > 0 {
					gpu.Memory = memBytes / 1024 / 1024 // Convert bytes to MB
				}
			}
		}

		// Check if it's an integrated GPU
		if isIntegratedGPU(name) {
			gpu.Type = constants.ComputeDeviceIntegratedGPU
		}

		// Enhance with lookup table specs
		enhanceGPUWithSpecs(&gpu)

		gpus = append(gpus, gpu)
	}

	return gpus
}

// getGPUInfoLinux detects GPUs on Linux using nvidia-smi and lspci
func getGPUInfoLinux() []store.ComputeDevice {
	var gpus []store.ComputeDevice

	// First, try to detect NVIDIA GPUs using nvidia-smi
	nvidiaGPUs := detectNVIDIALinux()
	gpus = append(gpus, nvidiaGPUs...)

	// Then, try to detect other GPUs using lspci
	otherGPUs := detectOtherGPUsLinux()
	gpus = append(gpus, otherGPUs...)

	return gpus
}

// detectNVIDIALinux detects NVIDIA GPUs using nvidia-smi
func detectNVIDIALinux() []store.ComputeDevice {
	var gpus []store.ComputeDevice

	// Try nvidia-smi --query-gpu=name,memory.total,compute_cap --format=csv,noheader
	cmd := exec.Command("nvidia-smi", "--query-gpu=name,memory.total,compute_cap", "--format=csv,noheader")
	output, err := cmd.Output()
	if err != nil {
		// nvidia-smi not available or no NVIDIA GPUs
		return gpus
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse CSV format: "GPU Name, 8192 MiB, 8.9"
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}

		gpu := store.ComputeDevice{
			Type:         constants.ComputeDeviceGPU,
			Vendor:       "nvidia",
			Model:        strings.TrimSpace(parts[0]),
			IsAvailable:  true,
			ComputeUnits: 0, // nvidia-smi doesn't provide this directly
			TOPS:         0,  // Would need model-specific lookup
		}

		// Parse memory (format: " 8192 MiB")
		if len(parts) >= 2 {
			memStr := strings.TrimSpace(parts[1])
			memStr = strings.TrimSuffix(memStr, " MiB")
			memStr = strings.TrimSpace(memStr)
			if memMB, err := strconv.ParseInt(memStr, 10, 64); err == nil {
				gpu.Memory = memMB
			}
		}

		// Enhance with lookup table specs
		enhanceGPUWithSpecs(&gpu)

		gpus = append(gpus, gpu)
	}

	return gpus
}

// detectOtherGPUsLinux detects non-NVIDIA GPUs using lspci and rocm-smi
func detectOtherGPUsLinux() []store.ComputeDevice {
	var gpus []store.ComputeDevice

	// First, try rocm-smi for AMD GPUs (more detailed info)
	amdGPUs := detectAMDLinux()
	gpus = append(gpus, amdGPUs...)

	// Use lspci to find other VGA and 3D controllers
	cmd := exec.Command("lspci")
	output, err := cmd.Output()
	if err != nil {
		return gpus
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for VGA or 3D controller entries
		if !strings.Contains(strings.ToLower(line), "vga") &&
			!strings.Contains(strings.ToLower(line), "3d") &&
			!strings.Contains(strings.ToLower(line), "display") {
			continue
		}

		// Skip if already detected as NVIDIA (avoid duplicates)
		if strings.Contains(strings.ToLower(line), "nvidia") {
			continue
		}

		// Skip if already detected via rocm-smi (AMD GPUs)
		if strings.Contains(strings.ToLower(line), "amd") || strings.Contains(strings.ToLower(line), "radeon") {
			// Check if we already have this GPU from rocm-smi
			alreadyFound := false
			for _, existing := range gpus {
				if existing.Vendor == "amd" {
					alreadyFound = true
					break
				}
			}
			if alreadyFound {
				continue
			}
		}

		// Extract vendor and model from lspci output
		// Format: "01:00.0 VGA compatible controller: Vendor Model Name (rev XX)"
		parts := strings.Split(line, ":")
		if len(parts) < 2 {
			continue
		}

		description := strings.TrimSpace(parts[len(parts)-1])
		vendor := detectVendorFromName(description)
		model := description

		// Try to extract just the model name (remove revision info)
		if idx := strings.Index(model, "(rev"); idx > 0 {
			model = strings.TrimSpace(model[:idx])
		}

		gpu := store.ComputeDevice{
			Type:        constants.ComputeDeviceGPU,
			Vendor:      vendor,
			Model:       model,
			IsAvailable: true,
		}

		// Check if it's an integrated GPU
		if isIntegratedGPU(description) {
			gpu.Type = constants.ComputeDeviceIntegratedGPU
		}

		// Enhance with lookup table specs
		enhanceGPUWithSpecs(&gpu)

		gpus = append(gpus, gpu)
	}

	return gpus
}

// detectAMDLinux detects AMD GPUs using rocm-smi (if available)
func detectAMDLinux() []store.ComputeDevice {
	var gpus []store.ComputeDevice

	// Try rocm-smi to get AMD GPU information
	cmd := exec.Command("rocm-smi", "--showproductname", "--showmeminfo", "vram", "--showid")
	output, err := cmd.Output()
	if err != nil {
		// rocm-smi not available
		return gpus
	}

	// Parse rocm-smi output (format varies, this is a basic parser)
	lines := strings.Split(string(output), "\n")
	var currentGPU *store.ComputeDevice

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// rocm-smi output parsing (simplified - may need adjustment based on actual output)
		if strings.Contains(line, "Card series:") || strings.Contains(line, "Card model:") {
			if currentGPU != nil {
				enhanceGPUWithSpecs(currentGPU)
				gpus = append(gpus, *currentGPU)
			}
			currentGPU = &store.ComputeDevice{
				Type:        constants.ComputeDeviceGPU,
				Vendor:      "amd",
				IsAvailable: true,
			}
			// Extract model name
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				currentGPU.Model = strings.TrimSpace(parts[1])
			}
		} else if currentGPU != nil && strings.Contains(line, "vram") {
			// Try to extract memory info
			// This is a simplified parser - rocm-smi output format may vary
		}
	}

	if currentGPU != nil {
		enhanceGPUWithSpecs(currentGPU)
		gpus = append(gpus, *currentGPU)
	}

	return gpus
}

// getGPUInfoDarwin detects GPUs on macOS using system_profiler
func getGPUInfoDarwin() []store.ComputeDevice {
	var gpus []store.ComputeDevice

	// Use system_profiler to get display information
	cmd := exec.Command("system_profiler", "SPDisplaysDataType")
	output, err := cmd.Output()
	if err != nil {
		return gpus
	}

	// Parse system_profiler output
	lines := strings.Split(string(output), "\n")
	var currentGPU *store.ComputeDevice
	var inDisplaySection bool

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Detect start of a new display section
		if strings.HasPrefix(line, "Chipset Model:") || strings.HasPrefix(line, "Device ID:") {
			if currentGPU != nil {
				gpus = append(gpus, *currentGPU)
			}
			currentGPU = &store.ComputeDevice{
				Type:        constants.ComputeDeviceGPU,
				IsAvailable: true,
			}
			inDisplaySection = true

			if strings.HasPrefix(line, "Chipset Model:") {
				model := strings.TrimSpace(strings.TrimPrefix(line, "Chipset Model:"))
				currentGPU.Model = model
				currentGPU.Vendor = detectVendorFromName(model)
			}
		} else if inDisplaySection && currentGPU != nil {
			// Parse other GPU properties
			if strings.HasPrefix(line, "VRAM (Dynamic, Max):") {
				// Format: "VRAM (Dynamic, Max): 8192 MB"
				memStr := strings.TrimSpace(strings.TrimPrefix(line, "VRAM (Dynamic, Max):"))
				memStr = strings.TrimSuffix(memStr, " MB")
				if memMB, err := strconv.ParseInt(memStr, 10, 64); err == nil {
					currentGPU.Memory = memMB
				}
			} else if strings.HasPrefix(line, "Metal Family:") {
				// Apple Silicon GPUs
				metalFamily := strings.TrimSpace(strings.TrimPrefix(line, "Metal Family:"))
				if strings.Contains(strings.ToLower(metalFamily), "apple") {
					currentGPU.Vendor = "apple"
					if currentGPU.Model == "" {
						currentGPU.Model = "Apple " + metalFamily
					}
				}
			} else if strings.HasPrefix(line, "Vendor:") {
				vendor := strings.TrimSpace(strings.TrimPrefix(line, "Vendor:"))
				if currentGPU.Vendor == "" {
					currentGPU.Vendor = strings.ToLower(vendor)
				}
			}
		}

		// Detect end of display section (empty line or new major section)
		if line == "" && inDisplaySection {
			inDisplaySection = false
		}
	}

	// Add the last GPU if exists
	if currentGPU != nil && currentGPU.Model != "" {
		enhanceGPUWithSpecs(currentGPU)
		gpus = append(gpus, *currentGPU)
	}

	// If no GPUs found, check for Apple Silicon integrated GPU
	if len(gpus) == 0 {
		// Check if running on Apple Silicon
		if runtime.GOARCH == "arm64" {
			// Try to get more info about Apple Silicon GPU
			cmd := exec.Command("sysctl", "-n", "machdep.cpu.brand_string")
			output, err := cmd.Output()
			if err == nil {
				cpuBrand := strings.TrimSpace(string(output))
				if strings.Contains(strings.ToLower(cpuBrand), "apple") {
					gpu := store.ComputeDevice{
						Type:        constants.ComputeDeviceIntegratedGPU,
						Vendor:      "apple",
						Model:       "Apple Integrated GPU",
						IsAvailable: true,
					}
					enhanceGPUWithSpecs(&gpu)
					gpus = append(gpus, gpu)
				}
			}
		}
	}

	return gpus
}

// detectVendorFromName extracts vendor name from GPU model name
func detectVendorFromName(name string) string {
	nameLower := strings.ToLower(name)
	if strings.Contains(nameLower, "nvidia") || strings.Contains(nameLower, "geforce") ||
		strings.Contains(nameLower, "quadro") || strings.Contains(nameLower, "tesla") ||
		strings.Contains(nameLower, "rtx") || strings.Contains(nameLower, "gtx") {
		return "nvidia"
	}
	if strings.Contains(nameLower, "amd") || strings.Contains(nameLower, "radeon") ||
		strings.Contains(nameLower, "firepro") || strings.Contains(nameLower, "rx") {
		return "amd"
	}
	if strings.Contains(nameLower, "intel") || strings.Contains(nameLower, "iris") ||
		strings.Contains(nameLower, "uhd") || strings.Contains(nameLower, "hd graphics") {
		return "intel"
	}
	if strings.Contains(nameLower, "apple") {
		return "apple"
	}
	return "unknown"
}

// isIntegratedGPU checks if a GPU is integrated based on its name/description
func isIntegratedGPU(name string) bool {
	nameLower := strings.ToLower(name)
	integratedKeywords := []string{
		"intel", "iris", "uhd", "hd graphics",
		"amd radeon graphics", "radeon graphics", "vega", "integrated",
		"apple", "m1", "m2", "m3", "m4",
		"ryzen", // AMD Ryzen APUs have integrated graphics
	}
	for _, keyword := range integratedKeywords {
		if strings.Contains(nameLower, keyword) {
			return true
		}
	}
	return false
}
