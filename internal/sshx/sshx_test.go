package sshx

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestExecArgs(t *testing.T) {
	tests := []struct {
		name string
		opts ExecOpts
		want []string
	}{
		{
			name: "interactive shell with TTY",
			opts: ExecOpts{InstanceID: "inst-123", WantTTY: true},
			want: []string{
				"-o", "StrictHostKeyChecking=accept-new",
				"-t",
				"inst-123@ssh.railway.com",
			},
		},
		{
			name: "interactive shell without TTY",
			opts: ExecOpts{InstanceID: "inst-123", WantTTY: false},
			want: []string{
				"-o", "StrictHostKeyChecking=accept-new",
				"-T",
				"inst-123@ssh.railway.com",
			},
		},
		{
			name: "command exec with TTY",
			opts: ExecOpts{InstanceID: "inst-123", Command: []string{"ls", "-la", "/data"}, WantTTY: true},
			want: []string{
				"-o", "StrictHostKeyChecking=accept-new",
				"-t",
				"inst-123@ssh.railway.com",
				"--", "ls", "-la", "/data",
			},
		},
		{
			name: "command exec without TTY (piped)",
			opts: ExecOpts{InstanceID: "inst-123", Command: []string{"cat", "/etc/hostname"}, WantTTY: false},
			want: []string{
				"-o", "StrictHostKeyChecking=accept-new",
				"-T",
				"inst-123@ssh.railway.com",
				"--", "cat", "/etc/hostname",
			},
		},
		{
			name: "with identity file",
			opts: ExecOpts{InstanceID: "inst-123", IdentityFile: "/home/u/.ssh/id_ed25519", WantTTY: true},
			want: []string{
				"-i", "/home/u/.ssh/id_ed25519",
				"-o", "StrictHostKeyChecking=accept-new",
				"-t",
				"inst-123@ssh.railway.com",
			},
		},
		{
			name: "identity file + command",
			opts: ExecOpts{InstanceID: "inst-9", IdentityFile: "/k", Command: []string{"whoami"}, WantTTY: false},
			want: []string{
				"-i", "/k",
				"-o", "StrictHostKeyChecking=accept-new",
				"-T",
				"inst-9@ssh.railway.com",
				"--", "whoami",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExecArgs(tt.opts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ExecArgs() =\n  %#v\nwant\n  %#v", got, tt.want)
			}
		})
	}
}

func TestWantTTY(t *testing.T) {
	tests := []struct {
		name                string
		hasCommand          bool
		stdinTTY, stdoutTTY bool
		want                bool
	}{
		{"command + both TTYs -> -t", true, true, true, true},
		{"command + stdin only -> -T", true, true, false, false},
		{"command + stdout only -> -T", true, false, true, false},
		{"command + neither -> -T", true, false, false, false},
		{"no command + stdin TTY -> -t", false, true, false, true},
		{"no command + stdin not TTY -> -T", false, false, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := WantTTY(tt.hasCommand, tt.stdinTTY, tt.stdoutTTY); got != tt.want {
				t.Errorf("WantTTY(%v,%v,%v) = %v, want %v", tt.hasCommand, tt.stdinTTY, tt.stdoutTTY, got, tt.want)
			}
		})
	}
}

func TestDiscoverPublicKey_PreferenceOrder(t *testing.T) {
	dir := t.TempDir()
	// Write rsa and ecdsa but NOT ed25519; ecdsa is preferred over rsa.
	mustWrite(t, filepath.Join(dir, "id_rsa.pub"), "ssh-rsa AAAA rsa")
	mustWrite(t, filepath.Join(dir, "id_ecdsa.pub"), "ecdsa-sha2 AAAA ecdsa")

	got, err := DiscoverPublicKey(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join(dir, "id_ecdsa.pub"); got != want {
		t.Errorf("DiscoverPublicKey picked %q, want %q (ecdsa before rsa)", got, want)
	}
}

func TestDiscoverPublicKey_Ed25519First(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "id_ed25519.pub"), "ssh-ed25519 AAAA ed")
	mustWrite(t, filepath.Join(dir, "id_rsa.pub"), "ssh-rsa AAAA rsa")

	got, err := DiscoverPublicKey(dir, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if want := filepath.Join(dir, "id_ed25519.pub"); got != want {
		t.Errorf("DiscoverPublicKey picked %q, want %q (ed25519 first)", got, want)
	}
}

func TestDiscoverPublicKey_NoneFound(t *testing.T) {
	dir := t.TempDir()
	_, err := DiscoverPublicKey(dir, "")
	if err == nil {
		t.Fatal("expected an error when no key is present")
	}
	if !strings.Contains(err.Error(), "ssh-keygen") {
		t.Errorf("error should point at ssh-keygen, got: %v", err)
	}
}

func TestDiscoverPublicKey_IdentityOverride(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "mykey")
	pub := priv + ".pub"
	mustWrite(t, pub, "ssh-ed25519 AAAA mine")
	// A default key is also present but must be ignored in favor of -i.
	mustWrite(t, filepath.Join(dir, "id_ed25519.pub"), "ssh-ed25519 AAAA default")

	got, err := DiscoverPublicKey(dir, priv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != pub {
		t.Errorf("DiscoverPublicKey honored default, got %q, want %q", got, pub)
	}
}

func TestDiscoverPublicKey_IdentityDotPubDirect(t *testing.T) {
	dir := t.TempDir()
	pub := filepath.Join(dir, "mykey.pub")
	mustWrite(t, pub, "ssh-ed25519 AAAA mine")

	got, err := DiscoverPublicKey(dir, pub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != pub {
		t.Errorf("got %q, want %q", got, pub)
	}
}

func TestDiscoverPublicKey_IdentityMissingPub(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "mykey") // no .pub sibling
	_, err := DiscoverPublicKey(dir, priv)
	if err == nil {
		t.Fatal("expected an error when the identity file has no .pub sibling")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
