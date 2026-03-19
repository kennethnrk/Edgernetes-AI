package client

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// Format specifies the output format.
type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// ParseFormat converts a string to a Format, defaulting to table.
func ParseFormat(s string) Format {
	switch s {
	case "json":
		return FormatJSON
	case "yaml":
		return FormatYAML
	default:
		return FormatTable
	}
}

// Formatter renders output in the requested format.
type Formatter struct {
	format Format
	writer io.Writer
}

// NewFormatter creates a Formatter that writes to stdout.
func NewFormatter(format Format) *Formatter {
	return &Formatter{format: format, writer: os.Stdout}
}

// PrintTable writes rows as an aligned ASCII table.
// The first row is treated as the header.
func (f *Formatter) PrintTable(headers []string, rows [][]string) {
	tw := tabwriter.NewWriter(f.writer, 0, 0, 2, ' ', 0)
	for _, h := range headers {
		fmt.Fprintf(tw, "%s\t", h)
	}
	fmt.Fprintln(tw)
	for _, row := range rows {
		for _, col := range row {
			fmt.Fprintf(tw, "%s\t", col)
		}
		fmt.Fprintln(tw)
	}
	tw.Flush()
}

// PrintJSON marshals v as indented JSON and writes it.
func (f *Formatter) PrintJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("json marshal: %w", err)
	}
	fmt.Fprintln(f.writer, string(data))
	return nil
}

// PrintYAML marshals v as YAML and writes it.
func (f *Formatter) PrintYAML(v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("yaml marshal: %w", err)
	}
	fmt.Fprint(f.writer, string(data))
	return nil
}

// Print renders v according to the configured format.
// tableFunc is called for table format; v is used for json/yaml.
func (f *Formatter) Print(v any, tableFunc func()) error {
	switch f.format {
	case FormatJSON:
		return f.PrintJSON(v)
	case FormatYAML:
		return f.PrintYAML(v)
	default:
		tableFunc()
		return nil
	}
}
