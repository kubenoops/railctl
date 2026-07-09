package cmdutil

import (
	"fmt"
	"io"
	"os"
)

// OutputIsText reports whether railctl is rendering human-facing text (the
// default table output) rather than machine output (json/yaml). The root
// command sets it once from the -o flag before any RunE runs. Advisory hints
// are suppressed unless this is true, so they never interleave with or corrupt
// piped/structured output.
var OutputIsText bool

// NoHintsEnv silences every advisory hint when set to any non-empty value.
const NoHintsEnv = "RAILCTL_NO_HINTS"

// hintWriter is where advisory hints are written. It is a package var (not a
// hard-coded os.Stderr) purely so tests can capture the hint.
var hintWriter io.Writer = os.Stderr

// maybeLeastPrivilegeHint prints a one-line nudge to stderr when a
// project-scoped operation is run with a broad account/workspace token instead
// of a least-privilege project token. It fires only in text output mode and is
// silenced by RAILCTL_NO_HINTS. isProjectToken is the token classification
// ResolveContext already computed, so this costs no extra API call.
//
// The nudge is advisory only: broad tokens work fine, but a project token is
// leaf-bound to one project+environment and cannot touch anything else, so it
// is the safer default for day-to-day project work.
func maybeLeastPrivilegeHint(isProjectToken bool) {
	if isProjectToken || !OutputIsText || os.Getenv(NoHintsEnv) != "" {
		return
	}
	fmt.Fprintln(hintWriter,
		"hint: this project-scoped operation is using a broad account/workspace token. "+
			"A project token (see 'railctl token create') grants least privilege — it is "+
			"leaf-bound to one project+environment. Silence with "+NoHintsEnv+"=1.")
}
