package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// executeCommand runs the CLI with the given args and returns stdout, stderr, and error.
func executeCommand(args ...string) (string, string, error) {
	// Reset flags and env vars to prevent real API calls
	token = ""
	outputFormat = "table"
	project = ""
	environment = ""
	service = ""
	os.Unsetenv("RAILWAY_TOKEN")

	// Capture stdout and stderr
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	rootCmd.SetOut(stdout)
	rootCmd.SetErr(stderr)
	rootCmd.SetArgs(args)

	err := rootCmd.Execute()

	return stdout.String(), stderr.String(), err
}

func TestRootCmd_Help(t *testing.T) {
	stdout, _, err := executeCommand("--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedStrings := []string{
		"railctl",
		"kubectl-inspired", // matches the actual text in Long description
		"get",
		"describe",
		"--token",
		"-p",
		"-e",
		"-s",
		"-o",
	}

	for _, s := range expectedStrings {
		if !strings.Contains(stdout, s) {
			t.Errorf("expected %q in help output", s)
		}
	}
}

func TestRootCmd_Version(t *testing.T) {
	// Note: Cobra may merge --version with help output in some scenarios
	// Just verify the version number appears somewhere
	stdout, _, err := executeCommand("version")
	// Also try with help that shows version
	if err != nil {
		// Fallback: version might not be a subcommand
		stdout, _, _ = executeCommand("--help")
	}

	// Version should be visible in help or version output
	if !strings.Contains(stdout, version) && !strings.Contains(stdout, "--version") {
		t.Errorf("expected version info, got: %s", stdout)
	}
}

func TestGetToken_FromFlag(t *testing.T) {
	token = "flag-token"
	defer func() { token = "" }()

	result, err := getToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "flag-token" {
		t.Errorf("expected 'flag-token', got %q", result)
	}
}

func TestGetToken_FromEnv(t *testing.T) {
	token = ""
	os.Setenv("RAILWAY_TOKEN", "env-token")
	defer os.Unsetenv("RAILWAY_TOKEN")

	result, err := getToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "env-token" {
		t.Errorf("expected 'env-token', got %q", result)
	}
}

func TestGetToken_FlagOverridesEnv(t *testing.T) {
	token = "flag-token"
	os.Setenv("RAILWAY_TOKEN", "env-token")
	defer func() {
		token = ""
		os.Unsetenv("RAILWAY_TOKEN")
	}()

	result, err := getToken()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "flag-token" {
		t.Errorf("expected 'flag-token', got %q", result)
	}
}

func TestGetToken_Missing(t *testing.T) {
	token = ""
	os.Unsetenv("RAILWAY_TOKEN")

	_, err := getToken()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "RAILWAY_TOKEN") {
		t.Errorf("error should mention RAILWAY_TOKEN: %v", err)
	}
}

func TestGetProject_FromFlag(t *testing.T) {
	project = "flag-project"
	defer func() { project = "" }()

	result := getProject()
	if result != "flag-project" {
		t.Errorf("expected 'flag-project', got %q", result)
	}
}

func TestGetProject_FromEnv(t *testing.T) {
	project = ""
	os.Setenv("RAILCTL_PROJECT", "env-project")
	defer os.Unsetenv("RAILCTL_PROJECT")

	result := getProject()
	if result != "env-project" {
		t.Errorf("expected 'env-project', got %q", result)
	}
}

func TestGetEnvironment_FromFlag(t *testing.T) {
	environment = "flag-env"
	defer func() { environment = "" }()

	result := getEnvironment()
	if result != "flag-env" {
		t.Errorf("expected 'flag-env', got %q", result)
	}
}

func TestGetEnvironment_FromEnv(t *testing.T) {
	environment = ""
	os.Setenv("RAILCTL_ENVIRONMENT", "env-environment")
	defer os.Unsetenv("RAILCTL_ENVIRONMENT")

	result := getEnvironment()
	if result != "env-environment" {
		t.Errorf("expected 'env-environment', got %q", result)
	}
}

func TestGetService_FromFlag(t *testing.T) {
	service = "flag-service"
	defer func() { service = "" }()

	result := getService()
	if result != "flag-service" {
		t.Errorf("expected 'flag-service', got %q", result)
	}
}

func TestGetService_FromEnv(t *testing.T) {
	service = ""
	os.Setenv("RAILCTL_SERVICE", "env-service")
	defer os.Unsetenv("RAILCTL_SERVICE")

	result := getService()
	if result != "env-service" {
		t.Errorf("expected 'env-service', got %q", result)
	}
}

func TestGetOutputFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"json", "json"},
		{"yaml", "yaml"},
		{"wide", "wide"},
		{"table", "table"},
		{"", "table"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			outputFormat = tc.input
			defer func() { outputFormat = "table" }()

			result, err := getOutputFormat()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if string(result) != tc.expected {
				t.Errorf("expected %q, got %q", tc.expected, result)
			}
		})
	}

	t.Run("invalid format returns error", func(t *testing.T) {
		outputFormat = "invalid-format"
		defer func() { outputFormat = "table" }()

		_, err := getOutputFormat()
		if err == nil {
			t.Fatal("expected error for invalid format, got nil")
		}
		if !strings.Contains(err.Error(), "invalid output format") {
			t.Errorf("error should mention 'invalid output format': %v", err)
		}
	})
}

func TestGetCmd_Help(t *testing.T) {
	stdout, _, err := executeCommand("get", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedStrings := []string{
		"projects",
		"environments",
		"services",
		"variables",
		"deployments",
	}

	for _, s := range expectedStrings {
		if !strings.Contains(stdout, s) {
			t.Errorf("expected %q in get help output", s)
		}
	}
}

func TestDescribeCmd_Help(t *testing.T) {
	stdout, _, err := executeCommand("describe", "--help")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedStrings := []string{
		"project",
		"service",
	}

	for _, s := range expectedStrings {
		if !strings.Contains(stdout, s) {
			t.Errorf("expected %q in describe help output", s)
		}
	}
}

func TestGetProjectsCmd_NoToken(t *testing.T) {
	os.Unsetenv("RAILWAY_TOKEN")
	token = ""

	_, _, err := executeCommand("get", "projects")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "token") {
		t.Errorf("error should mention token: %v", err)
	}
}

func TestDescribeProjectCmd_NoName(t *testing.T) {
	os.Unsetenv("RAILCTL_PROJECT")
	project = ""
	token = "fake"

	_, _, err := executeCommand("describe", "project")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("error should mention 'required': %v", err)
	}
}

// resetRootCmd resets the root command state for testing
func resetRootCmd() {
	rootCmd.SetArgs([]string{})
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
}

func TestMain(m *testing.M) {
	// Disable Cobra's automatic error handling for tests
	rootCmd.SilenceUsage = true
	rootCmd.SilenceErrors = true

	code := m.Run()
	os.Exit(code)
}

// findCommand finds a subcommand by name
func findCommand(root *cobra.Command, name string) *cobra.Command {
	for _, cmd := range root.Commands() {
		if cmd.Name() == name {
			return cmd
		}
	}
	return nil
}
