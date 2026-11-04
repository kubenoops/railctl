package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/contexts"
)

// Identity precedence: --token > RAILWAY_TOKEN > active context's
// credentials; a conflict between env token and context credentials warns on
// stderr naming both sources (FR-016, FR-018).
func TestTokenPrecedence_EnvContextCredentials(t *testing.T) {
	cred := &contexts.Credential{Name: "work", Token: "cred-token", TokenType: "account"}
	seedCtx := &contexts.Context{Name: "shop-prod", Project: "shop", Credentials: "work"}
	file := &contexts.File{
		CurrentContext: "shop-prod",
		Contexts:       []contexts.Context{*seedCtx},
		Credentials:    []contexts.Credential{*cred},
	}

	t.Run("context credentials fill when nothing explicit", func(t *testing.T) {
		token = ""
		os.Unsetenv("RAILWAY_TOKEN")
		activeContext = seedCtx
		activeConfig = file
		t.Cleanup(func() { activeContext, activeConfig = nil, nil })

		got, err := getToken()
		if err != nil || got != "cred-token" {
			t.Fatalf("expected context credentials token, got %q, %v", got, err)
		}
	})

	t.Run("env beats context credentials with warning", func(t *testing.T) {
		token = ""
		os.Setenv("RAILWAY_TOKEN", "env-token")
		defer os.Unsetenv("RAILWAY_TOKEN")
		activeContext = seedCtx
		activeConfig = file
		t.Cleanup(func() { activeContext, activeConfig = nil, nil })

		var got string
		var err error
		stderr := captureStderr(t, func() {
			got, err = getToken()
		})
		if err != nil || got != "env-token" {
			t.Fatalf("env must win, got %q, %v", got, err)
		}
		if !strings.Contains(stderr, "RAILWAY_TOKEN") || !strings.Contains(stderr, "shop-prod") {
			t.Fatalf("warning must name RAILWAY_TOKEN and the context, got %q", stderr)
		}
	})

	t.Run("no warning without context credentials", func(t *testing.T) {
		token = ""
		os.Setenv("RAILWAY_TOKEN", "env-token")
		defer os.Unsetenv("RAILWAY_TOKEN")
		activeContext = &contexts.Context{Name: "bare", Project: "p"}
		activeConfig = &contexts.File{}
		t.Cleanup(func() { activeContext, activeConfig = nil, nil })

		var err error
		stderr := captureStderr(t, func() {
			_, err = getToken()
		})
		if err != nil {
			t.Fatalf("unexpected: %v", err)
		}
		if strings.Contains(stderr, "RAILWAY_TOKEN") {
			t.Fatalf("no warning expected without context credentials, got %q", stderr)
		}
	})
}
