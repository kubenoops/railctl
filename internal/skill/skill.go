// Package skill embeds the railctl usage skill into the binary at build time
// and exposes it as plain text via `railctl skill`.
//
// The source of truth is docs/railctl-skill.md — a real Markdown file so it
// renders on GitHub and is discoverable. railctl-skill.md in this directory is
// a generated, byte-identical copy that //go:embed compiles into the binary
// (go:embed cannot reach outside the package or follow a symlink). Regenerate
// it with `go generate ./internal/skill/` or `make gen`; CI (skill-sync) fails
// if the copy drifts from the source.
package skill

import (
	_ "embed"
	"strings"
)

//go:generate cp ../../docs/railctl-skill.md railctl-skill.md

//go:embed railctl-skill.md
var content string

// Content returns the embedded railctl usage skill as Markdown, with a
// trailing newline so it prints cleanly to a terminal.
func Content() string {
	return strings.TrimRight(content, "\n") + "\n"
}
