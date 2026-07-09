package sshx

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

// Fingerprint computes the OpenSSH SHA256 fingerprint of a public key line, in
// the canonical `SHA256:<base64-no-padding>` form that `ssh-keygen -lf` and
// Railway's API both report. This lets railctl match a locally discovered key
// against the fingerprints returned by ListSSHKeys for idempotent registration.
//
// A public-key line is `<type> <base64-blob> [comment]`; the fingerprint is the
// SHA256 of the raw (base64-decoded) key blob.
func Fingerprint(pubKeyLine string) (string, error) {
	fields := strings.Fields(strings.TrimSpace(pubKeyLine))
	if len(fields) < 2 {
		return "", fmt.Errorf("malformed public key: expected '<type> <base64> [comment]'")
	}
	blob, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return "", fmt.Errorf("malformed public key base64: %w", err)
	}
	sum := sha256.Sum256(blob)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:]), nil
}

// ReadPublicKey reads and trims a public-key file's contents.
func ReadPublicKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read public key %q: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}
