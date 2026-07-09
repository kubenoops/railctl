package sshx

import (
	"reflect"
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
		{
			// kubectl-consistent argv preservation: a token carrying shell
			// metacharacters is single-quoted so the remote shell reconstructs
			// it as ONE token instead of re-splitting it.
			name: "sh -c pipeline is quoted so remote argv survives",
			opts: ExecOpts{InstanceID: "inst-9", Command: []string{"sh", "-c", "echo a; echo b"}, WantTTY: false},
			want: []string{
				"-o", "StrictHostKeyChecking=accept-new",
				"-T",
				"inst-9@ssh.railway.com",
				"--", "sh", "-c", "'echo a; echo b'",
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

func TestShellQuote(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", "''"},
		{"whoami", "whoami"},               // fast path: no quoting
		{"/etc/hostname", "/etc/hostname"}, // paths stay bare
		{"echo a; echo b", "'echo a; echo b'"},
		{"a b", "'a b'"},
		{`it's`, `'it'\''s'`}, // embedded single quote: close, escape, reopen
	}
	for _, tt := range tests {
		if got := shellQuote(tt.in); got != tt.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tt.in, got, tt.want)
		}
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
