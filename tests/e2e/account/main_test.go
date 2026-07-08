//go:build e2e

// Package account holds the L1 e2e test group: everything exclusive to an
// account-scoped token (workspace enumeration and -w disambiguation). The
// group's credential is validated once in TestMain and shared by every test
// via the package-level token var.
package account

import (
	"os"
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// token is the account-scoped Railway credential for this group,
// classified and validated by the harness preflight before any test runs.
var token string

func TestMain(m *testing.M) {
	token = harness.RequireToken("RAILWAY_ACCOUNT_TOKEN", harness.TokenAccount)
	os.Exit(m.Run())
}
