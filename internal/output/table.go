package output

import (
	"strings"
)

// Column defines a table column with its header and data key.
type Column struct {
	Header string
	Width  int
}

// Table represents a formatted table with columns and rows.
type Table struct {
	columns []Column
	rows    [][]string
}

// NewTable creates a new table with the given column headers.
func NewTable(headers ...string) *Table {
	columns := make([]Column, len(headers))
	for i, h := range headers {
		columns[i] = Column{
			Header: strings.ToUpper(h),
			Width:  len(h),
		}
	}
	return &Table{columns: columns}
}

// AddRow adds a row of values to the table.
// The number of values should match the number of columns.
func (t *Table) AddRow(values ...string) {
	// Pad or truncate to match column count
	row := make([]string, len(t.columns))
	for i := range t.columns {
		if i < len(values) {
			row[i] = values[i]
			if len(values[i]) > t.columns[i].Width {
				t.columns[i].Width = len(values[i])
			}
		}
	}
	t.rows = append(t.rows, row)
}

// Render produces the formatted table string.
func (t *Table) Render() string {
	if len(t.rows) == 0 {
		return "No resources found.\n"
	}

	var sb strings.Builder

	// Header row
	for i, col := range t.columns {
		if i > 0 {
			sb.WriteString("  ")
		}
		sb.WriteString(padRight(col.Header, col.Width))
	}
	sb.WriteString("\n")

	// Data rows
	for _, row := range t.rows {
		for i, val := range row {
			if i > 0 {
				sb.WriteString("  ")
			}
			sb.WriteString(padRight(val, t.columns[i].Width))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// padRight pads a string to the given width with spaces.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// RowCount returns the number of data rows in the table.
func (t *Table) RowCount() int {
	return len(t.rows)
}
