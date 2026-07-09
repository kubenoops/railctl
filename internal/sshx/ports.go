package sshx

import (
	"fmt"
	"strconv"
	"strings"
)

// loopback is the literal remote host railctl pins the -L remote side to for
// the one- and two-field port specs. Railway's relay does its own DNS
// resolution and does NOT honor the target container's /etc/hosts, so a name
// like "localhost" resolves to an unreachable mesh address. The literal
// 127.0.0.1 is the only host that reliably reaches the port the target itself
// listens on (mirrors native.rs:390-396).
const loopback = "127.0.0.1"

// PortForward is one resolved -L forward: bind localhost:LocalPort locally and
// relay it to RemoteHost:RemotePort as seen from inside the target container's
// network namespace.
//
//   - For the bare-number and LOCAL:REMOTE forms, RemoteHost is always the
//     127.0.0.1 literal (the service's own loopback).
//   - For the LOCAL:HOST:REMOTE jump form, RemoteHost is a *.railway.internal
//     name or IP passed through verbatim — the relay resolves it server-side
//     from inside the jump container's netns, so you reach a DIFFERENT private
//     host through the jump service.
type PortForward struct {
	LocalPort  int
	RemoteHost string
	RemotePort int
}

// ParsePortSpec parses one port-forward spec into a PortForward, per the
// three-form grammar (mirrors kubectl port-forward + Railway's -L pinning):
//
//	REMOTE               → 127.0.0.1:REMOTE:127.0.0.1:REMOTE   (local == remote)
//	LOCAL:REMOTE         → 127.0.0.1:LOCAL :127.0.0.1:REMOTE   (host forced to loopback)
//	LOCAL:HOST:REMOTE    → 127.0.0.1:LOCAL :HOST     :REMOTE   (jump form; HOST verbatim)
//
// The remote host of the one- and two-field forms is ALWAYS forced to the
// 127.0.0.1 literal — a bare number can never smuggle "localhost" (a foot-gun
// that resolves to the unreachable mesh address). Only the explicit three-field
// form may name a remote host, which is passed through unchanged so a
// *.railway.internal target is reachable through the jump service.
//
// Ports must be in 1..65535; a malformed spec (empty, non-numeric port,
// out-of-range port, or more than three colon-separated fields) yields a clear
// error.
func ParsePortSpec(spec string) (PortForward, error) {
	if strings.TrimSpace(spec) == "" {
		return PortForward{}, fmt.Errorf("empty port spec")
	}
	parts := strings.Split(spec, ":")

	switch len(parts) {
	case 1:
		// REMOTE — local == remote, remote host pinned to loopback.
		p, err := parsePort(parts[0], "port")
		if err != nil {
			return PortForward{}, specErr(spec, err)
		}
		return PortForward{LocalPort: p, RemoteHost: loopback, RemotePort: p}, nil

	case 2:
		// LOCAL:REMOTE — remote host FORCED to loopback (foot-gun guard).
		local, err := parsePort(parts[0], "local port")
		if err != nil {
			return PortForward{}, specErr(spec, err)
		}
		remote, err := parsePort(parts[1], "remote port")
		if err != nil {
			return PortForward{}, specErr(spec, err)
		}
		return PortForward{LocalPort: local, RemoteHost: loopback, RemotePort: remote}, nil

	default:
		// A LOCAL:HOST:REMOTE "jump" form was considered, but Railway's SSH
		// relay only forwards to the target container's OWN loopback — it does
		// not honor -L forwards to other hosts (verified live). So a spec is
		// always [LOCAL:]REMOTE and forwards into the service you name. To
		// reach a private service, port-forward directly into IT.
		return PortForward{}, specErr(spec, fmt.Errorf("expected REMOTE or LOCAL:REMOTE (port-forward reaches the target service's own loopback; forward directly into the service you want to reach)"))
	}
}

// parsePort parses a single port token and validates the 1..65535 range.
func parsePort(s, label string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty %s", label)
	}
	p, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q: not a number", label, s)
	}
	if p < 1 || p > 65535 {
		return 0, fmt.Errorf("%s %d out of range (must be 1-65535)", label, p)
	}
	return p, nil
}

func specErr(spec string, err error) error {
	return fmt.Errorf("invalid port spec %q: %w", spec, err)
}

// ForwardOpts describes one `railctl port-forward` invocation for ForwardArgs.
type ForwardOpts struct {
	// InstanceID is the service instance UUID used as the SSH username (the
	// relay's routing key) — for the jump form this is the JUMP service's
	// instance, whose netns the *.railway.internal names resolve inside.
	InstanceID string
	// IdentityFile, when non-empty, is passed to ssh via -i.
	IdentityFile string
	// Forwards is the set of -L forwards to establish over the one connection.
	Forwards []PortForward
	// Address is the local bind address (the LOCAL side). Empty defaults to
	// 127.0.0.1; pass 0.0.0.0 to share the forward on the LAN.
	Address string
}

// ForwardArgs builds the exact `ssh` argv for a forward-only session,
// byte-for-byte mirroring Railway's Rust CLI (native.rs:366-398). Flag order:
//
//	[-p <port>]  (only when the relay port is non-default)
//	[-i <identityFile>]
//	-o StrictHostKeyChecking=accept-new
//	-N
//	-o ExitOnForwardFailure=yes
//	-o ServerAliveInterval=30
//	-o ServerAliveCountMax=3
//	(-L <addr>:<local>:<host>:<remote>)...
//	<instanceID>@ssh.railway.com
//
// baseArgs already emits [-p], [-i] and StrictHostKeyChecking=accept-new (the
// -N-with-closed-stdin TOFU option); the forward-specific -N plus the
// ExitOnForwardFailure/ServerAlive* options and the -L specs are appended here.
func ForwardArgs(opts ForwardOpts) []string {
	args := baseArgs(opts.IdentityFile)

	args = append(args,
		"-N",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=30",
		"-o", "ServerAliveCountMax=3",
	)

	addr := opts.Address
	if addr == "" {
		addr = loopback
	}

	for _, f := range opts.Forwards {
		host := f.RemoteHost
		if host == "" {
			host = loopback
		}
		args = append(args, "-L",
			fmt.Sprintf("%s:%d:%s:%d", addr, f.LocalPort, host, f.RemotePort))
	}

	return append(args, target(opts.InstanceID))
}
