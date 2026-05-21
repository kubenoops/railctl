package cmdutil

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/output"
)

// OutputConfig describes how a command renders its results.
type OutputConfig struct {
	Format output.Format
}

// PrintResult handles the format switch for any command's output.
// It takes the format and three render options:
//   - data: the structured data for JSON/YAML output
//   - table: the default table
//   - wideTable: the optional wide table (if nil, falls back to table)
//   - emptyMessage: message to print when the table has zero rows
func PrintResult(format output.Format, data any, table *output.Table, wideTable *output.Table, emptyMessage string) error {
	printer := output.NewPrinter(format)

	switch format {
	case output.FormatJSON:
		return printer.PrintJSON(data)
	case output.FormatYAML:
		return printer.PrintYAML(data)
	case output.FormatWide:
		t := wideTable
		if t == nil {
			t = table
		}
		if t.RowCount() == 0 {
			fmt.Println(emptyMessage)
			return nil
		}
		return printer.PrintTable(t)
	default:
		if table.RowCount() == 0 {
			fmt.Println(emptyMessage)
			return nil
		}
		return printer.PrintTable(table)
	}
}
