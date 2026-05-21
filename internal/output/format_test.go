package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseFormat(t *testing.T) {
	t.Run("valid formats", func(t *testing.T) {
		tests := []struct {
			input    string
			expected Format
		}{
			{"table", FormatTable},
			{"json", FormatJSON},
			{"yaml", FormatYAML},
			{"wide", FormatWide},
			{"", FormatTable}, // empty defaults to table
		}

		for _, tc := range tests {
			t.Run(tc.input, func(t *testing.T) {
				result, err := ParseFormat(tc.input)
				if err != nil {
					t.Fatalf("ParseFormat(%q) unexpected error: %v", tc.input, err)
				}
				if result != tc.expected {
					t.Errorf("ParseFormat(%q) = %v, expected %v", tc.input, result, tc.expected)
				}
			})
		}
	})

	t.Run("invalid formats", func(t *testing.T) {
		invalids := []string{"invalid", "JSON", "TABLE", "xml", "csv"}
		for _, input := range invalids {
			t.Run(input, func(t *testing.T) {
				_, err := ParseFormat(input)
				if err == nil {
					t.Errorf("ParseFormat(%q) expected error, got nil", input)
				}
			})
		}
	})
}

func TestValidFormats(t *testing.T) {
	formats := ValidFormats()
	if len(formats) != 4 {
		t.Errorf("expected 4 formats, got %d", len(formats))
	}

	expected := []string{"table", "wide", "json", "yaml"}
	for i, f := range expected {
		if formats[i] != f {
			t.Errorf("format[%d] = %q, expected %q", i, formats[i], f)
		}
	}
}

func TestPrinter_Format(t *testing.T) {
	p := NewPrinter(FormatJSON)
	if p.Format() != FormatJSON {
		t.Errorf("Format() = %v, expected FormatJSON", p.Format())
	}
}

func TestPrinter_IsStructured(t *testing.T) {
	tests := []struct {
		format   Format
		expected bool
	}{
		{FormatJSON, true},
		{FormatYAML, true},
		{FormatTable, false},
		{FormatWide, false},
	}

	for _, tc := range tests {
		t.Run(string(tc.format), func(t *testing.T) {
			p := NewPrinter(tc.format)
			if p.IsStructured() != tc.expected {
				t.Errorf("IsStructured() = %v, expected %v", p.IsStructured(), tc.expected)
			}
		})
	}
}

func TestPrinter_PrintJSON(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{format: FormatJSON, out: &buf}

	data := map[string]string{"name": "test", "value": "123"}
	err := p.PrintJSON(data)
	if err != nil {
		t.Fatalf("PrintJSON() error: %v", err)
	}

	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if result["name"] != "test" {
		t.Errorf("expected name='test', got %q", result["name"])
	}
}

func TestPrinter_PrintYAML(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{format: FormatYAML, out: &buf}

	data := map[string]string{"name": "test"}
	err := p.PrintYAML(data)
	if err != nil {
		t.Fatalf("PrintYAML() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "name: test") {
		t.Errorf("expected YAML with 'name: test', got: %s", output)
	}
}

func TestPrinter_PrintTable(t *testing.T) {
	var buf bytes.Buffer
	p := &Printer{format: FormatTable, out: &buf}

	table := NewTable("Name", "Value")
	table.AddRow("foo", "bar")

	err := p.PrintTable(table)
	if err != nil {
		t.Fatalf("PrintTable() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "NAME") {
		t.Errorf("expected header 'NAME', got: %s", output)
	}
	if !strings.Contains(output, "foo") {
		t.Errorf("expected 'foo' in output, got: %s", output)
	}
}
