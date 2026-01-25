package utils

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
)

// GetMemoryType detects the RAM type (DDR4, LPDDR4, etc.) of the system
func GetMemoryType() constants.MemoryType {
	switch runtime.GOOS {
	case "windows":
		return getMemoryTypeWindows()
	case "linux":
		return getMemoryTypeLinux()
	case "darwin":
		return getMemoryTypeDarwin()
	default:
		return constants.MemoryTypeUnknown
	}
}

// getMemoryTypeWindows detects RAM type on Windows using WMI
func getMemoryTypeWindows() constants.MemoryType {
	// Try to get the memory type description first (more readable)
	cmd := exec.Command("powershell", "-Command", 
		"Get-WmiObject -Class Win32_PhysicalMemory | Select-Object -First 1 -ExpandProperty MemoryType")
	
	output, err := cmd.Output()
	if err == nil {
		memTypeStr := strings.TrimSpace(string(output))
		if memTypeStr != "" && memTypeStr != "0" {
			return parseMemoryTypeFromString(memTypeStr)
		}
	}

	// Fallback: try SMBIOSMemoryType (numeric code)
	cmd = exec.Command("powershell", "-Command", 
		"Get-WmiObject -Class Win32_PhysicalMemory | Select-Object -First 1 -ExpandProperty SMBIOSMemoryType")
	
	output, err = cmd.Output()
	if err != nil {
		return constants.MemoryTypeUnknown
	}

	memTypeStr := strings.TrimSpace(string(output))
	return parseMemoryTypeFromString(memTypeStr)
}

// getMemoryTypeLinux detects RAM type on Linux using dmidecode or sysfs
func getMemoryTypeLinux() constants.MemoryType {
	// Try dmidecode first (requires root or dmidecode permissions)
	cmd := exec.Command("dmidecode", "-t", "17")
	output, err := cmd.Output()
	if err == nil {
		// Parse dmidecode output - look for Type field
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "Type:") {
				memTypeStr := strings.TrimSpace(strings.TrimPrefix(line, "Type:"))
				if memTypeStr != "Unknown" && memTypeStr != "" {
					return parseMemoryTypeFromString(memTypeStr)
				}
			}
			// Also check for Speed field which sometimes contains type info
			if strings.HasPrefix(line, "Speed:") {
				memTypeStr := strings.TrimSpace(strings.TrimPrefix(line, "Speed:"))
				// Speed field might contain type hints
				if strings.Contains(memTypeStr, "DDR") {
					return parseMemoryTypeFromString(memTypeStr)
				}
			}
		}
	}

	// Fallback: try reading from sysfs (if available)
	// Some systems expose memory info in /sys/class/dmi/id/
	// This is a best-effort approach
	return constants.MemoryTypeUnknown
}

// getMemoryTypeDarwin detects RAM type on macOS using system_profiler
func getMemoryTypeDarwin() constants.MemoryType {
	cmd := exec.Command("system_profiler", "SPMemoryDataType")
	output, err := cmd.Output()
	if err != nil {
		return constants.MemoryTypeUnknown
	}

	// Parse system_profiler output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "Type:") {
			memTypeStr := strings.TrimSpace(strings.TrimPrefix(line, "Type:"))
			return parseMemoryTypeFromString(memTypeStr)
		}
	}

	return constants.MemoryTypeUnknown
}

// parseMemoryTypeFromString converts a memory type string to MemoryType constant
func parseMemoryTypeFromString(memTypeStr string) constants.MemoryType {
	memTypeStr = strings.ToLower(strings.TrimSpace(memTypeStr))
	
	// Handle empty strings
	if memTypeStr == "" {
		return constants.MemoryTypeUnknown
	}
	
	// Handle SMBIOS memory type codes (Windows WMI) - check numeric codes first
	// SMBIOS Memory Type codes: 26=DDR4, 27=DDR5, 34=LPDDR4, 35=LPDDR5, etc.
	if len(memTypeStr) > 0 && len(memTypeStr) <= 4 && (strings.HasPrefix(memTypeStr, "0x") || 
		(memTypeStr[0] >= '0' && memTypeStr[0] <= '9')) {
		// Try to parse as numeric code
		switch memTypeStr {
		case "26", "0x1a":
			return constants.MemoryTypeDDR4
		case "27", "0x1b":
			return constants.MemoryTypeDDR5
		case "34", "0x22":
			return constants.MemoryTypeLPDDR4
		case "35", "0x23":
			return constants.MemoryTypeLPDDR5
		case "24", "0x18":
			return constants.MemoryTypeDDR3
		case "25", "0x19":
			return constants.MemoryTypeLPDDR3
		}
	}
	
	// Handle various text formats and naming conventions
	if strings.Contains(memTypeStr, "ddr5") {
		if strings.Contains(memTypeStr, "lpddr5x") || strings.Contains(memTypeStr, "lp-ddr5x") {
			return constants.MemoryTypeLPDDR5X
		}
		if strings.Contains(memTypeStr, "lpddr5") || strings.Contains(memTypeStr, "lp-ddr5") || 
		   strings.Contains(memTypeStr, "lp ddr5") {
			return constants.MemoryTypeLPDDR5
		}
		return constants.MemoryTypeDDR5
	}
	
	if strings.Contains(memTypeStr, "ddr4") {
		if strings.Contains(memTypeStr, "lpddr4x") || strings.Contains(memTypeStr, "lp-ddr4x") {
			return constants.MemoryTypeLPDDR4X
		}
		if strings.Contains(memTypeStr, "lpddr4") || strings.Contains(memTypeStr, "lp-ddr4") || 
		   strings.Contains(memTypeStr, "lp ddr4") {
			return constants.MemoryTypeLPDDR4
		}
		return constants.MemoryTypeDDR4
	}
	
	if strings.Contains(memTypeStr, "ddr3") {
		if strings.Contains(memTypeStr, "lpddr3") || strings.Contains(memTypeStr, "lp-ddr3") || 
		   strings.Contains(memTypeStr, "lp ddr3") {
			return constants.MemoryTypeLPDDR3
		}
		return constants.MemoryTypeDDR3
	}

	return constants.MemoryTypeUnknown
}
