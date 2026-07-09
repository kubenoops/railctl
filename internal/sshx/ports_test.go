package sshx

import (
	"reflect"
	"strings"
	"testing"
)

func TestParsePortSpec(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    PortForward
		wantErr bool
	}{
		// REMOTE (one field) — local == remote, host pinned to loopback.
		{
			name: "bare number: local==remote, host pinned to 127.0.0.1",
			spec: "8080",
			want: PortForward{LocalPort: 8080, RemoteHost: "127.0.0.1", RemotePort: 8080},
		},
		// LOCAL:REMOTE (two fields) — host FORCED to loopback (foot-gun guard).
		{
			name: "LOCAL:REMOTE: distinct ports, host forced to 127.0.0.1",
			spec: "6543:5432",
			want: PortForward{LocalPort: 6543, RemoteHost: "127.0.0.1", RemotePort: 5432},
		},
		{
			name: "LOCAL:REMOTE: equal ports still pin loopback",
			spec: "5432:5432",
			want: PortForward{LocalPort: 5432, RemoteHost: "127.0.0.1", RemotePort: 5432},
		},
		// Invalid inputs.
		{name: "empty", spec: "", wantErr: true},
		{name: "port zero", spec: "0", wantErr: true},
		{name: "port too large", spec: "65536", wantErr: true},
		{name: "non-numeric single", spec: "http", wantErr: true},
		{name: "non-numeric local", spec: "abc:5432", wantErr: true},
		{name: "non-numeric remote", spec: "5432:abc", wantErr: true},
		{name: "out-of-range remote in two-field", spec: "5432:99999", wantErr: true},
		// The three-field "jump" form is rejected: Railway's relay only forwards
		// to the target container's own loopback (verified live), so a spec is
		// always [LOCAL:]REMOTE.
		{name: "three-field jump form rejected", spec: "6443:kube-apiserver.railway.internal:6443", wantErr: true},
		{name: "too many colons", spec: "1:2:3:4", wantErr: true},
		{name: "empty local", spec: ":5432", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePortSpec(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParsePortSpec(%q) = %+v, want error", tt.spec, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParsePortSpec(%q) unexpected error: %v", tt.spec, err)
			}
			if got != tt.want {
				t.Errorf("ParsePortSpec(%q) = %+v, want %+v", tt.spec, got, tt.want)
			}
		})
	}
}

func TestForwardArgs(t *testing.T) {
	tests := []struct {
		name string
		opts ForwardOpts
		want []string
	}{
		{
			name: "single forward, loopback remote",
			opts: ForwardOpts{
				InstanceID: "inst-123",
				Forwards:   []PortForward{{LocalPort: 8080, RemoteHost: "127.0.0.1", RemotePort: 8080}},
			},
			want: []string{
				"-o", "StrictHostKeyChecking=accept-new",
				"-N",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=30",
				"-o", "ServerAliveCountMax=3",
				"-L", "127.0.0.1:8080:127.0.0.1:8080",
				"inst-123@ssh.railway.com",
			},
		},
		{
			name: "multiple forwards over one connection",
			opts: ForwardOpts{
				InstanceID: "inst-123",
				Forwards: []PortForward{
					{LocalPort: 5432, RemoteHost: "127.0.0.1", RemotePort: 5432},
					{LocalPort: 6379, RemoteHost: "127.0.0.1", RemotePort: 6379},
				},
			},
			want: []string{
				"-o", "StrictHostKeyChecking=accept-new",
				"-N",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=30",
				"-o", "ServerAliveCountMax=3",
				"-L", "127.0.0.1:5432:127.0.0.1:5432",
				"-L", "127.0.0.1:6379:127.0.0.1:6379",
				"inst-123@ssh.railway.com",
			},
		},
		{
			name: "identity file placed before the -o/-N block",
			opts: ForwardOpts{
				InstanceID:   "inst-9",
				IdentityFile: "/home/u/.ssh/id_ed25519",
				Forwards:     []PortForward{{LocalPort: 5432, RemoteHost: "127.0.0.1", RemotePort: 5432}},
			},
			want: []string{
				"-i", "/home/u/.ssh/id_ed25519",
				"-o", "StrictHostKeyChecking=accept-new",
				"-N",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=30",
				"-o", "ServerAliveCountMax=3",
				"-L", "127.0.0.1:5432:127.0.0.1:5432",
				"inst-9@ssh.railway.com",
			},
		},
		{
			name: "non-default bind address (0.0.0.0 sharing)",
			opts: ForwardOpts{
				InstanceID: "inst-123",
				Address:    "0.0.0.0",
				Forwards:   []PortForward{{LocalPort: 8080, RemoteHost: "127.0.0.1", RemotePort: 8080}},
			},
			want: []string{
				"-o", "StrictHostKeyChecking=accept-new",
				"-N",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=30",
				"-o", "ServerAliveCountMax=3",
				"-L", "0.0.0.0:8080:127.0.0.1:8080",
				"inst-123@ssh.railway.com",
			},
		},
		{
			name: "empty RemoteHost defaults to loopback",
			opts: ForwardOpts{
				InstanceID: "inst-123",
				Forwards:   []PortForward{{LocalPort: 8080, RemotePort: 8080}},
			},
			want: []string{
				"-o", "StrictHostKeyChecking=accept-new",
				"-N",
				"-o", "ExitOnForwardFailure=yes",
				"-o", "ServerAliveInterval=30",
				"-o", "ServerAliveCountMax=3",
				"-L", "127.0.0.1:8080:127.0.0.1:8080",
				"inst-123@ssh.railway.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ForwardArgs(tt.opts)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ForwardArgs() =\n  %#v\nwant\n  %#v", got, tt.want)
			}
		})
	}
}

// TestForwardArgs_RemoteNeverLocalhost is a regression guard: the two-field
// grammar must never let a bare "localhost" slip into the -L remote host — it
// resolves to the unreachable mesh address. ParsePortSpec pins 127.0.0.1.
func TestForwardArgs_RemoteNeverLocalhost(t *testing.T) {
	pf, err := ParsePortSpec("5432:5432")
	if err != nil {
		t.Fatal(err)
	}
	argv := ForwardArgs(ForwardOpts{InstanceID: "i", Forwards: []PortForward{pf}})
	joined := strings.Join(argv, " ")
	if strings.Contains(joined, "localhost") {
		t.Errorf("argv must not contain 'localhost' as a forward host: %v", argv)
	}
	if !strings.Contains(joined, "127.0.0.1:5432:127.0.0.1:5432") {
		t.Errorf("expected loopback-pinned forward, got: %v", argv)
	}
}
