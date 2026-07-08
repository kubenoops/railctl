//go:build e2e

package project

import (
	"testing"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestDomains exercises the imperative custom-domain lifecycle inside the
// shared fixture project under the minted project token: create (prints the
// DNS records to configure), list, delete, and the not-found error after
// deletion. No -p/-e flags anywhere: the token carries the scope; only -s
// selects the service.
//
// The domains use a placeholder apex (.railctl-example.test) — Railway
// registers them and returns DNS records without requiring ownership, and
// they are never verified, so nothing routes.
//
//	go test -tags e2e -v -run TestDomains ./tests/e2e/project/...
func TestDomains(t *testing.T) {
	env := fixtureEnv(t)
	svcName := createService(t, env)

	domainName := harness.UniqueName() + ".railctl-example.test"
	// A second custom domain stays behind after the delete subtests so the
	// delete-again error has an "available:" list to render.
	otherDomain := harness.UniqueName() + ".railctl-example.test"

	t.Run("create", func(t *testing.T) {
		r := env.RunOK(t, "create", "domain", domainName, "-s", svcName, "--port", "8080")
		harness.AssertContains(t, r.Stdout, "DNS record")
		harness.AssertContains(t, r.Stdout, domainName)
	})

	t.Run("create_second", func(t *testing.T) {
		env.RunOK(t, "create", "domain", otherDomain, "-s", svcName)
	})

	t.Run("get_lists_it", func(t *testing.T) {
		r := env.RunOK(t, "get", "domains", "-s", svcName)
		harness.AssertContains(t, r.Stdout, domainName)
		harness.AssertContains(t, r.Stdout, "custom")
	})

	t.Run("get_json", func(t *testing.T) {
		r := env.RunOK(t, "get", "domains", "-s", svcName, "-o", "json")
		harness.AssertValidJSON(t, r.Stdout)
		harness.AssertContains(t, r.Stdout, domainName)
	})

	t.Run("delete", func(t *testing.T) {
		env.RunOK(t, "delete", "domain", domainName, "-s", svcName, "--yes")
	})

	t.Run("get_no_longer_lists_it", func(t *testing.T) {
		r := env.RunOK(t, "get", "domains", "-s", svcName)
		harness.AssertNotContains(t, r.Stdout, domainName)
	})

	t.Run("delete_again_not_found", func(t *testing.T) {
		r := env.RunFail(t, "delete", "domain", domainName, "-s", svcName, "--yes")
		// Error-taxonomy class 3: not-found errors list what DOES exist.
		harness.AssertContains(t, r.Stderr+r.Stdout, "not found")
		harness.AssertContains(t, r.Stderr+r.Stdout, "available")
		harness.AssertContains(t, r.Stderr+r.Stdout, otherDomain)
	})

	t.Run("cleanup_second", func(t *testing.T) {
		env.RunOK(t, "delete", "domain", otherDomain, "-s", svcName, "--yes")
	})
}
