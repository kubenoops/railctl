package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"

	"github.com/kubenoops/railctl/internal/contexts"
)

// executeCommandWithConfig runs the CLI like executeCommand but with
// RAILCTL_CONFIG pinned to the given path, isolating tests from any real
// config file (FR-004 parity for the rest of the suite comes from the
// sentinel used by executeCommand itself).
func executeCommandWithConfig(configPath string, args ...string) (string, string, error) {
	resetConfigCmdFlags()
	os.Setenv("RAILCTL_CONFIG", configPath)
	defer os.Unsetenv("RAILCTL_CONFIG")
	return executeCommand(args...)
}

// resetConfigCmdFlags clears the persistent flag state of the config
// subcommands: cobra flag singletons retain values and their Changed marker
// across Execute calls within one test process.
func resetConfigCmdFlags() {
	setContextWorkspace = ""
	setContextProject = ""
	setContextEnvironment = ""
	setContextCredentials = ""
	setContextCurrent = false
	configSetContextCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })

	setCredToken = ""
	setCredTokenIn = false
	configSetCredentialsCmd.Flags().VisitAll(func(f *pflag.Flag) { f.Changed = false })
}

func newTestConfigPath(t *testing.T) string {
	return filepath.Join(t.TempDir(), "config.yaml")
}

func TestConfigSetContext_Create(t *testing.T) {
	path := newTestConfigPath(t)

	stdout, _, err := executeCommandWithConfig(path,
		"config", "set-context", "shop-prod", "-w", "my-team", "-p", "shop", "-e", "production")

	if err != nil {
		t.Fatalf("unexpected error: %v (stderr captured in err)", err)
	}
	if !strings.Contains(stdout, "shop-prod") {
		t.Fatalf("output must confirm the context name, got: %q", stdout)
	}

	f, err := contexts.Load(path)
	if err != nil {
		t.Fatalf("load after set: %v", err)
	}
	want := contexts.Context{Name: "shop-prod", Workspace: "my-team", Project: "shop", Environment: "production"}
	if len(f.Contexts) != 1 || f.Contexts[0] != want {
		t.Fatalf("persisted context mismatch:\n got %+v\nwant %+v", f.Contexts, want)
	}
}

func TestConfigSetContext_CreateWithoutProjectFails(t *testing.T) {
	path := newTestConfigPath(t)

	_, _, err := executeCommandWithConfig(path, "config", "set-context", "shop")

	if err == nil || !strings.Contains(err.Error(), "project is required") {
		t.Fatalf("expected project-required error, got: %v", err)
	}
	if f, _ := contexts.Load(path); f != nil {
		t.Fatalf("failed create must not write a config file, got %+v", f)
	}
}

func TestConfigSetContext_UnknownCredentialFails(t *testing.T) {
	path := newTestConfigPath(t)
	f := &contexts.File{Credentials: []contexts.Credential{{Name: "work", Token: "t", TokenType: "account"}}}
	if err := contexts.Save(path, f); err != nil {
		t.Fatalf("setup save: %v", err)
	}

	_, _, err := executeCommandWithConfig(path,
		"config", "set-context", "shop", "-p", "shop", "--credentials", "ghost")

	if err == nil || !strings.Contains(err.Error(), "work") {
		t.Fatalf("expected error naming existing credentials, got: %v", err)
	}
}

func TestConfigSetContext_UpsertOnlyFlaggedFields(t *testing.T) {
	path := newTestConfigPath(t)
	if _, _, err := executeCommandWithConfig(path,
		"config", "set-context", "shop", "-p", "shop", "-e", "production", "-w", "team"); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, _, err := executeCommandWithConfig(path,
		"config", "set-context", "shop", "-e", "staging"); err != nil {
		t.Fatalf("update: %v", err)
	}

	f, err := contexts.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := contexts.Context{Name: "shop", Workspace: "team", Project: "shop", Environment: "staging"}
	if len(f.Contexts) != 1 || f.Contexts[0] != want {
		t.Fatalf("only -e must change:\n got %+v\nwant %+v", f.Contexts, want)
	}
}

func TestConfigSetContext_CurrentVariants(t *testing.T) {
	path := newTestConfigPath(t)

	_, _, err := executeCommandWithConfig(path, "config", "set-context", "--current", "-e", "staging")
	if err == nil || !strings.Contains(err.Error(), "no context is active") {
		t.Fatalf("expected no-active error, got: %v", err)
	}

	if _, _, err := executeCommandWithConfig(path,
		"config", "set-context", "shop", "-p", "shop"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, _, err := executeCommandWithConfig(path, "config", "use-context", "shop"); err != nil {
		t.Fatalf("activate: %v", err)
	}

	if _, _, err := executeCommandWithConfig(path,
		"config", "set-context", "--current", "-e", "staging"); err != nil {
		t.Fatalf("--current update: %v", err)
	}
	f, err := contexts.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if f.Contexts[0].Environment != "staging" || f.Contexts[0].Project != "shop" {
		t.Fatalf("--current must update active context in place: %+v", f.Contexts[0])
	}
}
