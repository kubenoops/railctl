//go:build e2e

// Package workspace holds the L2 e2e test group: everything a
// workspace-scoped token can do (project/environment lifecycle, token
// minting, the smoke walk). The group's credential is validated once in
// TestMain and shared by every test via the package-level token var.
package workspace

import (
	"os"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// token is the workspace-scoped Railway credential for this group,
// classified and validated by the harness preflight before any test runs.
var token string

func TestMain(m *testing.M) {
	token = harness.RequireToken("RAILWAY_WORKSPACE_TOKEN", harness.TokenWorkspace)
	os.Exit(m.Run())
}
