//go:build e2e

package harness

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// backboardGraphQL is Railway's production GraphQL endpoint. railctl itself
// only ever talks to prod backboard (see internal/api), so the e2e harness
// registers its throwaway SSH keys against the same tier. SSH-key management is
// deliberately NOT in railctl (users self-register at railway.com/account/
// ssh-keys), so the harness performs the register/revoke directly — mirroring
// what a user does once by hand — to exercise exec/port-forward end-to-end.
const backboardGraphQL = "https://backboard.railway.com/graphql/v2"

// SSHKey is a throwaway SSH keypair generated for one e2e run: the private key
// lives in a temp file, and the public key is registered with Railway so that
// exec/port-forward can authenticate. Revoke() removes the public key.
type SSHKey struct {
	PrivateKeyPath string // pass to railctl via -i
	keyID          string // Railway SshPublicKey id, for revocation
	registerToken  string // the token used to register (also used to revoke)
}

// SSHToolingAvailable reports whether both `ssh` and `ssh-keygen` are on PATH.
// exec/port-forward e2e tests skip cleanly when they are not.
func SSHToolingAvailable() bool {
	if _, err := exec.LookPath("ssh"); err != nil {
		return false
	}
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		return false
	}
	return true
}

// RegisterEphemeralSSHKey generates an ed25519 keypair and registers its public
// key with Railway using registerToken (a workspace or account token — project
// tokens cannot register keys). It returns a handle whose Revoke() the caller
// MUST defer. It fatals on any failure. The private key is written under
// t.TempDir(), so it is cleaned up with the test automatically; only the
// remote public key needs explicit revocation.
func RegisterEphemeralSSHKey(t *testing.T, registerToken string) *SSHKey {
	t.Helper()
	if !SSHToolingAvailable() {
		t.Skip("ssh/ssh-keygen not available — skipping SSH-based e2e")
	}

	dir := t.TempDir()
	priv := filepath.Join(dir, "id_ed25519")

	// -N "" (no passphrase), -q (quiet); ssh-keygen writes priv + priv.pub.
	kg := exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-C", "railctl-e2e", "-f", priv, "-q")
	if out, err := kg.CombinedOutput(); err != nil {
		t.Fatalf("ssh-keygen failed: %v\n%s", err, out)
	}
	pubBytes, err := os.ReadFile(priv + ".pub")
	if err != nil {
		t.Fatalf("reading generated public key: %v", err)
	}
	pub := string(bytes.TrimSpace(pubBytes))

	name := "railctl-e2e-" + UniqueName()
	data, err := postGraphQL(registerToken,
		`mutation($i: SshPublicKeyCreateInput!) { sshPublicKeyCreate(input: $i) { id } }`,
		map[string]any{"i": map[string]any{"name": name, "publicKey": pub}})
	if err != nil {
		t.Fatalf("registering SSH public key: %v", err)
	}
	var reg struct {
		SshPublicKeyCreate struct{ ID string } `json:"sshPublicKeyCreate"`
	}
	if err := json.Unmarshal(data, &reg); err != nil || reg.SshPublicKeyCreate.ID == "" {
		t.Fatalf("unexpected sshPublicKeyCreate response: %s (err %v)", data, err)
	}

	t.Logf("Registered ephemeral SSH key %s (%s)", name, reg.SshPublicKeyCreate.ID)
	return &SSHKey{PrivateKeyPath: priv, keyID: reg.SshPublicKeyCreate.ID, registerToken: registerToken}
}

// Revoke removes the registered public key from Railway. Best-effort: it logs
// but does not fatal, so a revoke hiccup never masks the test's own result.
func (k *SSHKey) Revoke(t *testing.T) {
	t.Helper()
	if k == nil || k.keyID == "" {
		return
	}
	if _, err := postGraphQL(k.registerToken,
		`mutation($id: String!) { sshPublicKeyDelete(id: $id) }`,
		map[string]any{"id": k.keyID}); err != nil {
		t.Logf("warning: failed to revoke ephemeral SSH key %s: %v", k.keyID, err)
		return
	}
	t.Logf("Revoked ephemeral SSH key %s", k.keyID)
}

// postGraphQL issues one authenticated GraphQL request to prod backboard and
// returns the raw `data` object. A GraphQL-level error is surfaced as a Go
// error. Used only for SSH-key CRUD, which railctl intentionally does not do.
func postGraphQL(token, query string, variables map[string]any) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]any{"query": query, "variables": variables})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, backboardGraphQL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var out struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding GraphQL response (HTTP %d): %w", resp.StatusCode, err)
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL error: %s", out.Errors[0].Message)
	}
	return out.Data, nil
}
