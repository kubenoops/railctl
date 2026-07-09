package skill

import (
	"strings"
	"testing"
)

func TestContent_EmbeddedAndNonEmpty(t *testing.T) {
	c := Content()
	if strings.TrimSpace(c) == "" {
		t.Fatal("embedded skill content is empty — //go:embed did not include railctl-skill.md")
	}
	if !strings.HasSuffix(c, "\n") {
		t.Error("Content() should end with a trailing newline")
	}
}

func TestContent_CoversTokenModel(t *testing.T) {
	c := Content()
	// The token model is the core of the guide; assert its key anchors are present
	// so a gutted or wrong file fails the build.
	for _, want := range []string{
		"name: railctl-usage",  // frontmatter
		"# railctl Usage",      // heading
		"Project-Access-Token", // project-token detection detail
		"project token cannot", // limitation
		"Capability matrix",    // per-type capabilities
		"whoami",               // first-contact classification
		"least-privilege",      // the doctrine, woven through the guide
		"compute provider",     // the opinionated stance
		"delete -f",            // declarative teardown verb
		"DELETE_PROTECTION",    // deletion tripwire
		"Zero → Hero",          // the canonical operating path
		"stack.yaml",           // single-manifest doctrine
		"Drift discipline",     // imperative changes must be reconciled
		"vibe coder",           // user-posture section: translate, do not teach
	} {
		if !strings.Contains(c, want) {
			t.Errorf("embedded skill is missing expected content: %q", want)
		}
	}
}
