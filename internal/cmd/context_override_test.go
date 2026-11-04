package cmd

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/contexts"
)

// --context selects a context for the current invocation only — targeting
// and credentials — without changing the active context (FR-017).
func TestContextFlag_OverridesPerInvocation(t *testing.T) {
	path := newTestConfigPath(t)
	seedConfig(t, path, &contexts.File{
		CurrentContext: "a",
		Contexts: []contexts.Context{
			{Name: "a", Project: "proj-a"},
			{Name: "b", Project: "proj-b", Credentials: "work"},
		},
		Credentials: []contexts.Credential{{Name: "work", Token: "cred-token", TokenType: "account"}},
	})

	if _, _, err := executeCommandWithConfig(path, "--context", "b", "config", "get-contexts"); err != nil {
		t.Fatalf("command with --context: %v", err)
	}

	if activeContext == nil || activeContext.Name != "b" || activeContext.Project != "proj-b" {
		t.Fatalf("--context must select b for the invocation, got %+v", activeContext)
	}
	if got, err := getToken(); err != nil || got != "cred-token" {
		t.Fatalf("--context must select its credentials too, got %q, %v", got, err)
	}

	f, _ := contexts.Load(path)
	if f.CurrentContext != "a" {
		t.Fatalf("active context must be unchanged, got %q", f.CurrentContext)
	}
}

// An unknown --context name fails listing available contexts before any API
// call (FR-017).
func TestContextFlag_UnknownNameFails(t *testing.T) {
	path := newTestConfigPath(t)
	seedConfig(t, path, &contexts.File{
		Contexts: []contexts.Context{{Name: "a", Project: "p"}, {Name: "b", Project: "p"}},
	})

	_, _, err := executeCommandWithConfig(path, "--context", "nope", "config", "get-contexts")

	if err == nil || !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Fatalf("expected error listing contexts, got: %v", err)
	}
}
