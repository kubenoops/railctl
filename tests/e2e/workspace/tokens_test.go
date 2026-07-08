//go:build e2e

package workspace

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestProjectTokens exercises the project-token lifecycle with a workspace
// token: mint (create), list (masked), list -o json, delete not-found, and
// delete. Uses the fixture project's default `production` environment — no
// custom environment needed.
//
//	go test -tags e2e -v -run TestProjectTokens ./tests/e2e/workspace/...
func TestProjectTokens(t *testing.T) {
	env := harness.SetupProject(t, token)

	tokenName := harness.UniqueName()
	var minted string // raw token value captured from create's stdout

	t.Run("create", func(t *testing.T) {
		r := env.RunOK(t, "token", "create", tokenName, "-p", env.ProjectName, "-e", "production")
		minted = strings.TrimSpace(r.Stdout)
		// stdout must be exactly the raw token value (uuid-shaped, 36 chars,
		// no extra output) so it is capturable via $(railctl token create ...).
		if len(minted) != 36 {
			t.Errorf("expected stdout to be exactly a 36-char token value, got %d chars:\nstdout: %q", len(minted), r.Stdout)
		}
		harness.AssertContains(t, r.Stderr, "Store it now")
	})

	t.Run("list", func(t *testing.T) {
		r := env.RunOK(t, "token", "list", "-p", env.ProjectName)
		harness.AssertContains(t, r.Stdout, tokenName)

		// Even in wide output the raw minted value must never appear — token
		// values are masked after creation.
		r = env.RunOK(t, "token", "list", "-p", env.ProjectName, "-o", "wide")
		if minted == "" {
			t.Skip("create subtest did not capture a token value")
		}
		harness.AssertNotContains(t, r.Stdout, minted)
	})

	t.Run("list_json", func(t *testing.T) {
		r := env.RunOK(t, "token", "list", "-p", env.ProjectName, "-o", "json")
		harness.AssertValidJSON(t, r.Stdout)
		harness.AssertContains(t, r.Stdout, tokenName)
	})

	t.Run("delete_not_found", func(t *testing.T) {
		r := env.RunFail(t, "token", "delete", "nonexistent-id-xyz", "-p", env.ProjectName, "--yes")
		harness.AssertContains(t, r.Stderr+r.Stdout, "not found")
		// The token minted by the create subtest is enumerable, so the
		// not-found error must list the existing token names.
		harness.AssertContains(t, r.Stderr+r.Stdout, "available:")
	})

	t.Run("delete", func(t *testing.T) {
		// Resolve the created token's id from the JSON listing.
		r := env.RunOK(t, "token", "list", "-p", env.ProjectName, "-o", "json")
		var listed []struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		}
		if err := json.Unmarshal([]byte(r.Stdout), &listed); err != nil {
			t.Fatalf("failed to unmarshal token list JSON: %v\nstdout: %s", err, r.Stdout)
		}
		var id string
		for _, tk := range listed {
			if tk.Name == tokenName {
				id = tk.ID
				break
			}
		}
		if id == "" {
			t.Fatalf("minted token %q not found in token list JSON:\n%s", tokenName, r.Stdout)
		}

		env.RunOK(t, "token", "delete", id, "-p", env.ProjectName, "--yes")

		r = env.RunOK(t, "token", "list", "-p", env.ProjectName)
		harness.AssertNotContains(t, r.Stdout, tokenName)
	})
}
