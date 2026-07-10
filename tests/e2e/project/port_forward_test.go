//go:build e2e

package project

import (
	"net/http"
	"testing"
	"time"

	"github.com/kubenoops/railctl/tests/e2e/harness"
)

// TestPortForward proves `railctl port-forward` end-to-end under the group's
// PROJECT token: it tunnels a local port into the service's own loopback over
// Railway's SSH relay and serves real traffic — no public domain/proxy on the
// service. nginx (the fixture image) listens on IPv4 :80, so a GET through the
// forward returns HTTP 200. The ephemeral SSH key is registered with the
// bootstrap workspace token and revoked at the end.
func TestPortForward(t *testing.T) {
	env := fixtureEnv(t)
	key := harness.RegisterEphemeralSSHKey(t, bootstrapToken) // t.Skip if no ssh/ssh-keygen
	defer key.Revoke(t)

	svc := deployReadySSHService(t, env)

	// A high, fixed local port; tests in this package do not run in parallel.
	const localPort = "59087"
	bg := env.StartBackground("port-forward", svc, localPort+":80", "-i", key.PrivateKeyPath)
	defer func() {
		if stderr := bg.Stop(); stderr != "" {
			t.Logf("port-forward stderr:\n%s", stderr)
		}
	}()

	// Poll the forwarded port until nginx answers or we give up. The relay +
	// -L take a moment to establish after the banner prints.
	url := "http://127.0.0.1:" + localPort + "/"
	client := &http.Client{Timeout: 5 * time.Second}
	served := false
	for i := 0; i < 15; i++ {
		time.Sleep(2 * time.Second)
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			served = true
			t.Logf("port-forward served HTTP 200 on %s after %d poll(s)", url, i+1)
			break
		}
	}
	if !served {
		t.Errorf("port-forward did not serve HTTP 200 on %s within the poll window", url)
	}
}
