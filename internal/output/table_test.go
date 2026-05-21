package output

import (
	"strings"
	"testing"
)

func TestNewTable_Headers(t *testing.T) {
	table := NewTable("Name", "Status", "Age")
	if len(table.columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(table.columns))
	}
	if table.columns[0].Header != "NAME" {
		t.Errorf("expected header 'NAME', got '%s'", table.columns[0].Header)
	}
}

func TestTable_AddRow(t *testing.T) {
	table := NewTable("Name", "Count")
	table.AddRow("my-app", "5")
	table.AddRow("other", "10")

	if len(table.rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(table.rows))
	}
}

func TestTable_Render_Empty(t *testing.T) {
	table := NewTable("Name", "Status")
	result := table.Render()

	if !strings.Contains(result, "No resources found") {
		t.Errorf("expected 'No resources found' message, got: %s", result)
	}
}

func TestTable_Render_WithData(t *testing.T) {
	table := NewTable("Name", "Count")
	table.AddRow("my-app", "5")
	table.AddRow("other-service", "10")

	result := table.Render()

	// Should have header row
	if !strings.Contains(result, "NAME") {
		t.Errorf("expected header 'NAME', got: %s", result)
	}
	if !strings.Contains(result, "COUNT") {
		t.Errorf("expected header 'COUNT', got: %s", result)
	}

	// Should have data rows
	if !strings.Contains(result, "my-app") {
		t.Errorf("expected 'my-app' in output, got: %s", result)
	}
	if !strings.Contains(result, "other-service") {
		t.Errorf("expected 'other-service' in output, got: %s", result)
	}
}

func TestTable_Render_Alignment(t *testing.T) {
	table := NewTable("Name", "X")
	table.AddRow("short", "1")
	table.AddRow("very-long-name", "2")

	result := table.Render()
	lines := strings.Split(strings.TrimSpace(result), "\n")

	// Header and data lines should have consistent spacing
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}
}

func TestTable_RowCount(t *testing.T) {
	table := NewTable("Name")
	if table.RowCount() != 0 {
		t.Errorf("expected 0 rows, got %d", table.RowCount())
	}

	table.AddRow("a")
	table.AddRow("b")
	if table.RowCount() != 2 {
		t.Errorf("expected 2 rows, got %d", table.RowCount())
	}
}

func TestPadRight(t *testing.T) {
	tests := []struct {
		input    string
		width    int
		expected string
	}{
		{"hello", 10, "hello     "},
		{"hello", 5, "hello"},
		{"hello", 3, "hello"},
		{"", 5, "     "},
	}

	for _, tc := range tests {
		result := padRight(tc.input, tc.width)
		if result != tc.expected {
			t.Errorf("padRight(%q, %d) = %q, expected %q",
				tc.input, tc.width, result, tc.expected)
		}
	}
}
