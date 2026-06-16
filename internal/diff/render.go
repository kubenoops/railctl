package diff

import (
	"fmt"
	"io"
	"os"
)

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
)

// Render writes a human-readable diff to the given writer.
// If useColor is true, uses ANSI colors (green for +, red for -, yellow for ~).
func Render(cs *ChangeSet, w io.Writer, useColor bool) {
	if !cs.HasChanges() {
		fmt.Fprintln(w, "No changes. Railway state matches the config.")
		return
	}

	for i, rc := range cs.Changes {
		if rc.Type == ChangeNone {
			continue
		}

		label := changeLabel(rc.Type)
		if useColor {
			fmt.Fprintf(w, "%sService: %s (%s)%s\n", colorBold, rc.ServiceName, label, colorReset)
		} else {
			fmt.Fprintf(w, "Service: %s (%s)\n", rc.ServiceName, label)
		}

		for _, f := range rc.Fields {
			renderField(w, rc.Type, f, useColor)
		}

		// Blank line between services, but not after the last one.
		if i < len(cs.Changes)-1 {
			fmt.Fprintln(w)
		}
	}
}

// IsColorSupported returns true if the given writer appears to be an
// interactive terminal that supports ANSI colors. Returns false if the
// NO_COLOR env var is set, if the writer is not an *os.File, or if the
// file descriptor does not point to a character device (e.g., pipe or file).
func IsColorSupported(w io.Writer) bool {
	if _, ok := os.LookupEnv("NO_COLOR"); ok {
		return false
	}

	// Only *os.File can be a terminal.
	f, ok := w.(*os.File)
	if !ok {
		return false
	}

	// Check if the fd is a character device (terminal).
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// changeLabel returns the human-readable label for a change type.
func changeLabel(ct ChangeType) string {
	switch ct {
	case ChangeCreate:
		return "create"
	case ChangeUpdate:
		return "update"
	case ChangeDelete:
		return "delete"
	default:
		return "none"
	}
}

// renderField writes a single field diff line.
func renderField(w io.Writer, ct ChangeType, f FieldDiff, useColor bool) {
	switch {
	case ct == ChangeCreate:
		// All fields are additions.
		printLine(w, "+", f.Path, f.Desired, "", useColor, colorGreen)

	case ct == ChangeDelete:
		// All fields are removals.
		printLine(w, "-", f.Path, f.Current, "", useColor, colorRed)

	case ct == ChangeUpdate:
		// Determine prefix based on field state.
		switch {
		case f.Current == "" && f.Desired != "":
			// Addition.
			printLine(w, "+", f.Path, f.Desired, "", useColor, colorGreen)
		case f.Current != "" && f.Desired == "":
			// Removal.
			printLine(w, "-", f.Path, f.Current, "", useColor, colorRed)
		default:
			// Change.
			printChange(w, f, useColor)
		}
	}
}

// printLine writes a line like "  + path: value" or "  - path: value".
func printLine(w io.Writer, prefix, path, value, _ string, useColor bool, color string) {
	if useColor {
		fmt.Fprintf(w, "  %s%s %s: %s%s\n", color, prefix, path, value, colorReset)
	} else {
		fmt.Fprintf(w, "  %s %s: %s\n", prefix, path, value)
	}
}

// printChange writes a change line like "  ~ path: old → new".
func printChange(w io.Writer, f FieldDiff, useColor bool) {
	if useColor {
		fmt.Fprintf(w, "  %s~ %s: %s → %s%s\n", colorYellow, f.Path, f.Current, f.Desired, colorReset)
	} else {
		fmt.Fprintf(w, "  ~ %s: %s → %s\n", f.Path, f.Current, f.Desired)
	}
}
