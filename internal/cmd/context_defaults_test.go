package cmd

import (
	"os"
	"testing"

	"github.com/kubenoops/railctl/internal/contexts"
)

func setActiveContext(t *testing.T, ctx *contexts.Context) {
	t.Helper()
	activeContext = ctx
	t.Cleanup(func() { activeContext = nil })
}

// Targeting precedence is uniform: flag > env var > active context > empty
// (FR-016). Per-field: an explicit flag replaces only its own field.
func TestTargetingPrecedence_FlagEnvContext(t *testing.T) {
	ctx := &contexts.Context{Name: "shop", Workspace: "ctx-team", Project: "ctx-shop", Environment: "ctx-prod"}

	t.Run("flag beats context", func(t *testing.T) {
		setActiveContext(t, ctx)
		project = "flag-shop"
		defer func() { project = "" }()

		if got := getProject(); got != "flag-shop" {
			t.Fatalf("flag must win, got %q", got)
		}
	})

	t.Run("env beats context", func(t *testing.T) {
		setActiveContext(t, ctx)
		project = ""
		os.Setenv("RAILCTL_PROJECT", "env-shop")
		defer os.Unsetenv("RAILCTL_PROJECT")

		if got := getProject(); got != "env-shop" {
			t.Fatalf("env must beat context, got %q", got)
		}
	})

	t.Run("context fills when nothing explicit", func(t *testing.T) {
		setActiveContext(t, ctx)
		project = ""
		os.Unsetenv("RAILCTL_PROJECT")

		if got := getProject(); got != "ctx-shop" {
			t.Fatalf("context must fill, got %q", got)
		}
		if got := getEnvironment(); got != "ctx-prod" {
			t.Fatalf("context env must fill, got %q", got)
		}
		if got := getWorkspace(); got != "ctx-team" {
			t.Fatalf("context workspace must fill, got %q", got)
		}
	})

	t.Run("empty context field does not shadow", func(t *testing.T) {
		// A context without environment must not inject an empty value —
		// env-requiring commands keep demanding -e (Edge Case 6).
		setActiveContext(t, &contexts.Context{Name: "noenv", Project: "p"})
		environment = ""
		os.Unsetenv("RAILCTL_ENVIRONMENT")

		if got := getEnvironment(); got != "" {
			t.Fatalf("empty context field must stay empty, got %q", got)
		}
	})
}

// Running any command must load the config file and resolve the active
// context for the process (FR-003): a config error fails the command.
func TestRootCommandLoadsActiveContext(t *testing.T) {
	path := newTestConfigPath(t)
	seedConfig(t, path, &contexts.File{
		CurrentContext: "shop",
		Contexts:       []contexts.Context{{Name: "shop", Project: "seeded-shop", Environment: "seeded-prod"}},
	})

	if _, _, err := executeCommandWithConfig(path, "config", "get-contexts"); err != nil {
		t.Fatalf("command with active context: %v", err)
	}
	if activeContext == nil || activeContext.Project != "seeded-shop" {
		t.Fatalf("active context must be loaded during command execution, got %+v", activeContext)
	}

	corrupt := newTestConfigPath(t)
	if err := os.WriteFile(corrupt, []byte("contexts: [broken"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, _, err := executeCommandWithConfig(corrupt, "config", "get-contexts"); err == nil {
		t.Fatal("corrupt config must fail every command")
	}
}
