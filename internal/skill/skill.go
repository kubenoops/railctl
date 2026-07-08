// Package skill embeds the railctl usage skill (railctl-skill.md) into the
// binary at build time and exposes it as plain text. The markdown file is the
// single source of truth: `railctl skill` prints it, and it doubles as a
// portable agent skill that lives in the repository.
package skill

import (
	_ "embed"
	"strings"
)

//go:embed railctl-skill.md
var content string

// Content returns the embedded railctl usage skill as Markdown, with a
// trailing newline so it prints cleanly to a terminal.
func Content() string {
	return strings.TrimRight(content, "\n") + "\n"
}
