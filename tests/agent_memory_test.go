package tests

import (
	"testing"

	"github.com/kennethnrk/edgernetes-ai/internal/agent/utils"
	"github.com/kennethnrk/edgernetes-ai/internal/common/constants"
)

func TestParseMemoryTypeFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected constants.MemoryType
	}{
		{"Empty string", "", constants.MemoryTypeUnknown},
		{"Numeric DDR4 code decimal", "26", constants.MemoryTypeDDR4},
		{"Numeric DDR4 code hex", "0x1a", constants.MemoryTypeDDR4},
		{"Numeric DDR5 code decimal", "27", constants.MemoryTypeDDR5},
		{"Numeric DDR5 code hex", "0x1b", constants.MemoryTypeDDR5},
		{"Numeric LPDDR4 code decimal", "34", constants.MemoryTypeLPDDR4},
		{"Numeric LPDDR4 code hex", "0x22", constants.MemoryTypeLPDDR4},
		{"Numeric LPDDR5 code decimal", "35", constants.MemoryTypeLPDDR5},
		{"Numeric LPDDR5 code hex", "0x23", constants.MemoryTypeLPDDR5},
		{"Numeric DDR3 code decimal", "24", constants.MemoryTypeDDR3},
		{"Numeric DDR3 code hex", "0x18", constants.MemoryTypeDDR3},
		{"Numeric LPDDR3 code decimal", "25", constants.MemoryTypeLPDDR3},
		{"Numeric LPDDR3 code hex", "0x19", constants.MemoryTypeLPDDR3},

		{"String LPDDR5X", "LPDDR5X", constants.MemoryTypeLPDDR5X},
		{"String LPDDR5X alternate", "LP-DDR5X", constants.MemoryTypeLPDDR5X},
		{"String LPDDR5", "LPDDR5", constants.MemoryTypeLPDDR5},
		{"String LPDDR5 alternate", "lp ddr5", constants.MemoryTypeLPDDR5},
		{"String DDR5", "DDR5", constants.MemoryTypeDDR5},

		{"String LPDDR4X", "LPDDR4X", constants.MemoryTypeLPDDR4X},
		{"String LPDDR4X alternate", "LP-DDR4X", constants.MemoryTypeLPDDR4X},
		{"String LPDDR4", "LPDDR4", constants.MemoryTypeLPDDR4},
		{"String LPDDR4 alternate", "lp ddr4", constants.MemoryTypeLPDDR4},
		{"String DDR4", "DDR4", constants.MemoryTypeDDR4},

		{"String LPDDR3", "LPDDR3", constants.MemoryTypeLPDDR3},
		{"String LPDDR3 alternate", "LP-DDR3", constants.MemoryTypeLPDDR3},
		{"String DDR3", "DDR3", constants.MemoryTypeDDR3},

		{"Unknown type", "RandomRAM", constants.MemoryTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := utils.ParseMemoryTypeFromString(tt.input)
			if result != tt.expected {
				t.Errorf("ParseMemoryTypeFromString(%q) expected %v, got %v", tt.input, tt.expected, result)
			}
		})
	}
}
