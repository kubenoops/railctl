package cmd

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/contexts"
)

// withStdin replaces os.Stdin with the given input for the duration of fn.
func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()
	old := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	if _, err := w.WriteString(input); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = old }()
	fn()
}

// errBoom is a sentinel detection failure.
var errBoom = errors.New("detection failed")

// overrideClient swaps the API client factory for the duration of the test
// (same injection point whoami_test.go uses).
func overrideClient(t *testing.T, mock *api.MockClient) {
	t.Helper()
	orig := newAPIClient
	t.Cleanup(func() { newAPIClient = orig })
	newAPIClient = func(tkn string) api.APIClient { return mock }
}

// mockTypeClient returns a MockClient whose detection probes classify the
// token as the given type.
func mockTypeClient(t *testing.T, tokenType string) *api.MockClient {
	t.Helper()
	mock := &api.MockClient{}
	mock.IsProjectTokenFunc = func() (bool, error) { return tokenType == "project", nil }
	mock.IsWorkspaceTokenFunc = func() (bool, error) { return tokenType == "workspace", nil }
	return mock
}

// set-credentials stores the token with its live-detected type; --token-stdin
// is one of exactly two input channels; both or neither is an error
// (FR-012). The token value never appears in output (FR-020).
func TestConfigSetCredentials_Channels(t *testing.T) {
	t.Run("stdin trims newline and persists detected type", func(t *testing.T) {
		path := newTestConfigPath(t)
		overrideClient(t, mockTypeClient(t, "workspace"))

		var stdout string
		var err error
		withStdin(t, "tok-secret-value\n", func() {
			stdout, _, err = executeCommandWithConfig(path,
				"config", "set-credentials", "work", "--token-stdin")
		})
		if err != nil {
			t.Fatalf("set-credentials: %v", err)
		}
		if strings.Contains(stdout, "tok-secret-value") {
			t.Fatal("token value must never be printed (FR-020)")
		}
		f, _ := contexts.Load(path)
		cred := f.Credential("work")
		if cred == nil || cred.Token != "tok-secret-value" || cred.TokenType != "workspace" {
			t.Fatalf("credential not stored with detected type: %+v", cred)
		}
	})

	t.Run("--token flag works", func(t *testing.T) {
		path := newTestConfigPath(t)
		overrideClient(t, mockTypeClient(t, "project"))
		if _, _, err := executeCommandWithConfig(path,
			"config", "set-credentials", "work", "--token", "tok-2"); err != nil {
			t.Fatalf("set-credentials: %v", err)
		}
		f, _ := contexts.Load(path)
		if cred := f.Credential("work"); cred == nil || cred.TokenType != "project" {
			t.Fatalf("token/type not persisted: %+v", cred)
		}
	})

	t.Run("both channels fail", func(t *testing.T) {
		path := newTestConfigPath(t)
		var err error
		withStdin(t, "tok\n", func() {
			_, _, err = executeCommandWithConfig(path,
				"config", "set-credentials", "work", "--token", "x", "--token-stdin")
		})
		if err == nil || !strings.Contains(err.Error(), "both") {
			t.Fatalf("expected both-channels error, got: %v", err)
		}
	})

	t.Run("neither channel fails", func(t *testing.T) {
		path := newTestConfigPath(t)
		_, _, err := executeCommandWithConfig(path, "config", "set-credentials", "work")
		if err == nil || !strings.Contains(err.Error(), "token") {
			t.Fatalf("expected missing-token error, got: %v", err)
		}
	})
}

// A failed detection probe must fail the write — a typo'd or revoked token
// is caught at entry (research D2).
func TestConfigSetCredentials_DetectionFailureFailsWrite(t *testing.T) {
	path := newTestConfigPath(t)
	mock := &api.MockClient{}
	mock.IsProjectTokenFunc = func() (bool, error) { return false, errBoom }
	overrideClient(t, mock)

	var err error
	withStdin(t, "tok\n", func() {
		_, _, err = executeCommandWithConfig(path,
			"config", "set-credentials", "work", "--token-stdin")
	})
	if err == nil {
		t.Fatal("expected detection failure to fail the write")
	}
	if f, _ := contexts.Load(path); f != nil {
		t.Fatalf("failed set must not persist: %+v", f)
	}
}

// get-credentials lists names and types only, in every format (FR-014,
// FR-020); delete-credentials refuses while referenced (FR-015).
func TestConfigGetDeleteCredentials(t *testing.T) {
	path := newTestConfigPath(t)
	seedConfig(t, path, &contexts.File{
		Contexts: []contexts.Context{{Name: "shop-prod", Project: "shop", Credentials: "work"}},
		Credentials: []contexts.Credential{
			{Name: "work", Token: "super-secret-value", TokenType: "workspace"},
			{Name: "personal", Token: "another-secret", TokenType: "account"},
		},
	})

	table, err := captureStdout(t, func() error {
		_, _, err := executeCommandWithConfig(path, "config", "get-credentials")
		return err
	})
	if err != nil {
		t.Fatalf("get-credentials: %v", err)
	}
	for _, want := range []string{"work", "personal", "workspace", "account"} {
		if !strings.Contains(table, want) {
			t.Fatalf("table must contain %q, got:\n%s", want, table)
		}
	}
	if strings.Contains(table, "super-secret-value") || strings.Contains(table, "another-secret") {
		t.Fatal("token values must never be rendered (FR-020)")
	}

	json, err := captureStdout(t, func() error {
		_, _, err := executeCommandWithConfig(path, "config", "get-credentials", "-o", "json")
		return err
	})
	if err != nil {
		t.Fatalf("json: %v", err)
	}
	if strings.Contains(json, "super-secret-value") {
		t.Fatal("token values must never be rendered in json (FR-020)")
	}
	if !strings.Contains(json, `"tokenType": "workspace"`) {
		t.Fatalf("json must carry tokenType, got: %s", json)
	}

	_, _, err = executeCommandWithConfig(path, "config", "delete-credentials", "work")
	if err == nil || !strings.Contains(err.Error(), "shop-prod") {
		t.Fatalf("expected refusal naming referencing context, got: %v", err)
	}

	if _, _, err := executeCommandWithConfig(path, "config", "delete-credentials", "personal"); err != nil {
		t.Fatalf("unreferenced delete: %v", err)
	}
	f, _ := contexts.Load(path)
	if f.Credential("personal") != nil || f.Credential("work") == nil {
		t.Fatalf("delete state wrong: %+v", f.Credentials)
	}
}
