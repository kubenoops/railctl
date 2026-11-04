package cmd

import (
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/contexts"
)

func seedConfig(t *testing.T, path string, f *contexts.File) {
	t.Helper()
	if err := contexts.Save(path, f); err != nil {
		t.Fatalf("seed save: %v", err)
	}
}

// current-context prints the active name only; with no active context it
// fails with the exact error on stderr and a non-zero exit (FR-009).
func TestConfigCurrentContext(t *testing.T) {
	path := newTestConfigPath(t)
	seedConfig(t, path, &contexts.File{
		CurrentContext: "shop-prod",
		Contexts:       []contexts.Context{{Name: "shop-prod", Project: "shop"}},
	})

	stdout, _, err := executeCommandWithConfig(path, "config", "current-context")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.TrimSpace(stdout) != "shop-prod" {
		t.Fatalf("must print only the context name, got %q", stdout)
	}

	path2 := newTestConfigPath(t)
	seedConfig(t, path2, &contexts.File{Contexts: []contexts.Context{{Name: "a", Project: "p"}}})
	_, _, err = executeCommandWithConfig(path2, "config", "current-context")
	if err == nil {
		t.Fatal("expected non-zero exit for no active context")
	}
	// The real CLI routes returned errors to stderr via Execute()
	// (root.go), so asserting the error message covers the FR-009 contract.
	if !strings.Contains(err.Error(), "no active context is set") {
		t.Fatalf("error must be the exact no-active message, got %q", err.Error())
	}
}

// use-context switches the active context as a local write with no API
// calls; unknown names fail listing available contexts without mutating the
// file (FR-010).
func TestConfigUseContext(t *testing.T) {
	path := newTestConfigPath(t)
	seedConfig(t, path, &contexts.File{
		Contexts: []contexts.Context{{Name: "a", Project: "p1"}, {Name: "b", Project: "p2"}},
	})

	if _, _, err := executeCommandWithConfig(path, "config", "use-context", "b"); err != nil {
		t.Fatalf("use-context: %v", err)
	}
	f, err := contexts.Load(path)
	if err != nil || f.CurrentContext != "b" {
		t.Fatalf("active context not persisted: %+v, %v", f, err)
	}

	before, _ := contexts.Load(path)
	_, _, err = executeCommandWithConfig(path, "config", "use-context", "nope")
	if err == nil || !strings.Contains(err.Error(), "a") || !strings.Contains(err.Error(), "b") {
		t.Fatalf("expected error listing contexts, got: %v", err)
	}
	after, _ := contexts.Load(path)
	if after.CurrentContext != before.CurrentContext {
		t.Fatal("failed use-context must not mutate the file")
	}
}

// get-contexts lists contexts marking the active one and never shows token
// values (FR-008, FR-020).
func TestConfigGetContexts(t *testing.T) {
	path := newTestConfigPath(t)
	seedConfig(t, path, &contexts.File{
		CurrentContext: "shop-prod",
		Contexts: []contexts.Context{{
			Name: "shop-prod", Workspace: "my-team", Project: "shop",
			Environment: "production", Credentials: "work",
		}},
		Credentials: []contexts.Credential{{Name: "work", Token: "super-secret-value", TokenType: "workspace"}},
	})

	stdout, err := captureStdout(t, func() error {
		_, _, err := executeCommandWithConfig(path, "config", "get-contexts")
		return err
	})
	if err != nil {
		t.Fatalf("get-contexts: %v", err)
	}
	for _, want := range []string{"shop-prod", "my-team", "shop", "production", "work", "workspace", "*"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table must contain %q, got:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "super-secret-value") {
		t.Fatal("token value must never be rendered (FR-020)")
	}

	json, err := captureStdout(t, func() error {
		_, _, err := executeCommandWithConfig(path, "config", "get-contexts", "-o", "json")
		return err
	})
	if err != nil {
		t.Fatalf("json output: %v", err)
	}
	if strings.Contains(json, "super-secret-value") {
		t.Fatal("token value must never be rendered in json (FR-020)")
	}
	if !strings.Contains(json, `"tokenType": "workspace"`) {
		t.Fatalf("json must carry the persisted tokenType, got: %s", json)
	}
}

// delete-context removes contexts; deleting the active one clears the slot
// with a stderr notice naming it (FR-011).
func TestConfigDeleteContext(t *testing.T) {
	path := newTestConfigPath(t)
	seedConfig(t, path, &contexts.File{
		CurrentContext: "a",
		Contexts:       []contexts.Context{{Name: "a", Project: "p1"}, {Name: "b", Project: "p2"}},
	})

	_, stderr, err := executeCommandWithConfig(path, "config", "delete-context", "a")
	if err != nil {
		t.Fatalf("delete active: %v", err)
	}
	if !strings.Contains(stderr, "a") {
		t.Fatalf("stderr notice must name the removed context, got %q", stderr)
	}
	f, _ := contexts.Load(path)
	if f.CurrentContext != "" || len(f.Contexts) != 1 || f.Contexts[0].Name != "b" {
		t.Fatalf("active slot must be cleared and context removed: %+v", f)
	}

	_, _, err = executeCommandWithConfig(path, "config", "delete-context", "ghost")
	if err == nil {
		t.Fatal("unknown context must fail")
	}
}
