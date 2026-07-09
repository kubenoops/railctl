package api

import (
	"encoding/json"
	"fmt"
)

// SSHKey is a public key registered with Railway. Registered keys are durable,
// like an authorized_keys entry — they persist across sessions and are never
// auto-deleted by railctl.
type SSHKey struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Fingerprint string `json:"fingerprint" yaml:"fingerprint"`
	PublicKey   string `json:"publicKey" yaml:"publicKey"`
}

// sshPublicKeyCreateMutation registers a public key. workspaceId is optional:
// null → a personal key on the token's user; set → a workspace-scoped key.
// There is no projectId — keys attach to a user or workspace, never a project,
// which is why a project-scoped token cannot register a key.
const sshPublicKeyCreateMutation = `
mutation SshPublicKeyCreate($input: SshPublicKeyCreateInput!) {
	sshPublicKeyCreate(input: $input) {
		id
		name
		fingerprint
	}
}
`

// sshPublicKeysQuery lists the SSH keys registered under the token's identity,
// used for idempotency (skip re-registering a fingerprint already present).
const sshPublicKeysQuery = `
query SshPublicKeys($workspaceId: String) {
	sshPublicKeys(workspaceId: $workspaceId) {
		edges {
			node {
				id
				name
				fingerprint
			}
		}
	}
}
`

// sshPublicKeyCreateResponse is the response for sshPublicKeyCreateMutation.
type sshPublicKeyCreateResponse struct {
	SSHPublicKeyCreate struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Fingerprint string `json:"fingerprint"`
	} `json:"sshPublicKeyCreate"`
}

// sshPublicKeysResponse is the response for sshPublicKeysQuery (a GraphQL
// connection: sshPublicKeys { edges { node { … } } }).
type sshPublicKeysResponse struct {
	SSHPublicKeys struct {
		Edges []struct {
			Node struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Fingerprint string `json:"fingerprint"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"sshPublicKeys"`
}

// RegisterSSHKey registers a public key with Railway. A non-empty workspaceID
// scopes the key to that workspace; empty registers a personal key on the
// token's user. Requires an account or workspace token — a project token has no
// user identity to attach a key to and will be denied by the API (callers
// should fail fast before reaching here).
func (c *Client) RegisterSSHKey(name, publicKey, workspaceID string) (SSHKey, error) {
	input := map[string]any{
		"name":      name,
		"publicKey": publicKey,
	}
	if workspaceID != "" {
		input["workspaceId"] = workspaceID
	}

	data, err := c.execute(sshPublicKeyCreateMutation, map[string]any{
		"input": input,
	})
	if err != nil {
		return SSHKey{}, err
	}

	var resp sshPublicKeyCreateResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return SSHKey{}, err
	}

	return SSHKey{
		ID:          resp.SSHPublicKeyCreate.ID,
		Name:        resp.SSHPublicKeyCreate.Name,
		Fingerprint: resp.SSHPublicKeyCreate.Fingerprint,
		PublicKey:   publicKey,
	}, nil
}

// ListSSHKeys returns the SSH keys already registered under the token's
// identity (personal keys, or the given workspace's keys when workspaceID is
// set). Used to skip re-registering a key whose fingerprint is already present.
func (c *Client) ListSSHKeys(workspaceID string) ([]SSHKey, error) {
	vars := map[string]any{}
	if workspaceID != "" {
		vars["workspaceId"] = workspaceID
	}

	data, err := c.execute(sshPublicKeysQuery, vars)
	if err != nil {
		return nil, fmt.Errorf("failed to list SSH keys: %w", err)
	}

	var resp sshPublicKeysResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}

	keys := make([]SSHKey, 0, len(resp.SSHPublicKeys.Edges))
	for _, e := range resp.SSHPublicKeys.Edges {
		keys = append(keys, SSHKey{
			ID:          e.Node.ID,
			Name:        e.Node.Name,
			Fingerprint: e.Node.Fingerprint,
		})
	}
	return keys, nil
}
