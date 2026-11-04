package cmd

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/contexts"
)

// Declarative -f commands: flag > env > YAML file > active context (FR-016).
// The context only fills gaps the YAML leaves (US4).
func TestApplyDiff_ContextFillsGapsOnly(t *testing.T) {
	configPath := newTestConfigPath(t)
	seedConfig(t, configPath, &contexts.File{
		CurrentContext: "ctx",
		// Deliberately targets a project the mock does NOT know, so if the
		// context wrongly beats the YAML file, resolution fails loudly.
		Contexts: []contexts.Context{{Name: "ctx", Project: "other-project", Environment: "other-env"}},
	})

	t.Run("yaml targeting wins over context", func(t *testing.T) {
		overrideClient(t, newMockForApply(nil))
		file := writeTestConfig(t, `
project: test-project
environment: test-env
services:
  - name: api
    image: nginx:latest
`)
		_, _, err := executeCommandWithConfig(configPath, "diff", "--token", "test-token", "-f", file)
		if err != nil {
			t.Fatalf("file targeting must win over the context, got: %v", err)
		}
	})

	t.Run("context fills when yaml omits targeting", func(t *testing.T) {
		// Re-seed: context targets what the mock knows, so gap-filling
		// resolves successfully.
		seedConfig(t, configPath, &contexts.File{
			CurrentContext: "ctx",
			Contexts:       []contexts.Context{{Name: "ctx", Project: "test-project", Environment: "test-env"}},
		})
		overrideClient(t, newMockForApply(nil))
		file := writeTestConfig(t, `
services:
  - name: api
    image: nginx:latest
`)
		_, _, err := executeCommandWithConfig(configPath, "diff", "--token", "test-token", "-f", file)
		if err != nil {
			t.Fatalf("context must fill omitted targeting, got: %v", err)
		}
	})

	t.Run("no context and no targeting keeps the required error", func(t *testing.T) {
		seedConfig(t, configPath, &contexts.File{})
		overrideClient(t, newMockForApply(nil))
		file := writeTestConfig(t, `
services:
  - name: api
    image: nginx:latest
`)
		_, _, err := executeCommandWithConfig(configPath, "diff", "--token", "test-token", "-f", file)
		if err == nil || !strings.Contains(err.Error(), "required") {
			t.Fatalf("expected unchanged required error, got: %v", err)
		}
	})
}
