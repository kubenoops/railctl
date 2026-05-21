// Package output provides formatting utilities for CLI output.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// Format represents an output format type.
type Format string

const (
	FormatTable Format = "table"
	FormatWide  Format = "wide"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
)

// ParseFormat converts a string to a Format, returning an error for invalid values.
func ParseFormat(s string) (Format, error) {
	switch s {
	case "table", "":
		return FormatTable, nil
	case "json":
		return FormatJSON, nil
	case "yaml":
		return FormatYAML, nil
	case "wide":
		return FormatWide, nil
	default:
		return "", fmt.Errorf("invalid output format %q (valid: table, wide, json, yaml)", s)
	}
}

// ValidFormats returns the list of valid format strings.
func ValidFormats() []string {
	return []string{"table", "wide", "json", "yaml"}
}

// Printer handles formatted output to a writer.
type Printer struct {
	format Format
	out    io.Writer
}

// NewPrinter creates a new Printer with the given format.
func NewPrinter(format Format) *Printer {
	return &Printer{
		format: format,
		out:    os.Stdout,
	}
}

// PrintJSON outputs data as formatted JSON.
func (p *Printer) PrintJSON(data any) error {
	encoder := json.NewEncoder(p.out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// PrintYAML outputs data as YAML.
func (p *Printer) PrintYAML(data any) error {
	encoder := yaml.NewEncoder(p.out)
	encoder.SetIndent(2)
	defer encoder.Close()
	return encoder.Encode(data)
}

// PrintTable outputs data as an aligned table.
func (p *Printer) PrintTable(table *Table) error {
	_, err := fmt.Fprint(p.out, table.Render())
	return err
}

// Format returns the printer's format.
func (p *Printer) Format() Format {
	return p.format
}

// IsStructured returns true if the format is JSON or YAML.
func (p *Printer) IsStructured() bool {
	return p.format == FormatJSON || p.format == FormatYAML
}
