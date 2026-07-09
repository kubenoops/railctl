// Package sshx is railctl's SSH transport foundation for `railctl exec`
// (and, in a follow-up, `railctl port-forward`).
//
// It shells out to the user's local `ssh` binary and dials Railway's global
// SSH relay (ssh.railway.com), exactly as Railway's own CLI does — the relay
// brokers the session into the target container (docker-exec-like), so the
// container needs no sshd of its own. The SSH username is a routing key: the
// service's instance UUID, not a login name.
//
// The package deliberately imports nothing from internal/api: it is pure argv
// construction and an os/exec shell-out behind a small Runner seam, so all of
// the risky ssh-shaped logic is unit-testable in isolation without any Railway
// API and without ever launching ssh in tests. railctl does not manage SSH
// keys — the user registers their key once at railway.com/account/ssh-keys and
// ssh authenticates with it (agent / ~/.ssh, or an explicit -i identity file).
package sshx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// RelayHost is Railway's global SSH relay endpoint (prod). All services and
// regions funnel through this one relay, which routes server-side by the SSH
// username (the service instance id). railctl only ever talks to the prod
// backboard, so the prod relay is the only tier we need.
const RelayHost = "ssh.railway.com"

// RelayPort is the relay's SSH port. Railway's prod/staging relay listens on
// the default 22 (only the develop relay uses 2222, which railctl never dials).
const RelayPort = 22

// Stdio bundles the three standard streams a Runner should wire to the ssh
// child process. Zero values mean "not connected"; the real Runner treats a
// nil stream as /dev/null-equivalent by leaving it unset on exec.Cmd.
type Stdio struct {
	Stdin  *os.File
	Stdout *os.File
	Stderr *os.File
}

// Runner executes an ssh invocation. It is the key testability seam: the real
// implementation (ExecRunner) shells out via os/exec; tests inject a fake that
// records the argv and returns a canned exit code without launching ssh.
//
// Run returns nil on a clean (exit 0) run. On a non-zero remote exit it returns
// an *ExitError carrying the code so callers can propagate it.
type Runner interface {
	Run(ctx context.Context, argv []string, io Stdio) error
}

// ExitError reports a non-zero exit status from the ssh child (which, for a
// command exec, is the remote command's own exit code — ssh propagates it).
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("ssh exited with code %d", e.Code)
}

// ExecRunner is the production Runner: it launches the local `ssh` binary and
// wires the provided stdio through to it.
type ExecRunner struct{}

// Run shells out to `ssh <argv...>`, inheriting the provided stdio streams.
// A non-zero exit is surfaced as *ExitError with the child's exit code so the
// caller can mirror it as railctl's own exit status.
func (ExecRunner) Run(ctx context.Context, argv []string, io Stdio) error {
	cmd := exec.CommandContext(ctx, "ssh", argv...)
	if io.Stdin != nil {
		cmd.Stdin = io.Stdin
	}
	if io.Stdout != nil {
		cmd.Stdout = io.Stdout
	}
	if io.Stderr != nil {
		cmd.Stderr = io.Stderr
	}

	err := cmd.Run()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &ExitError{Code: exitErr.ExitCode()}
	}
	return err
}

// EnsureSSHAvailable errors clearly if the local `ssh` binary cannot be found
// on PATH. railctl hard-depends on it (like Railway's own CLI); there is no
// pure-Go fallback.
func EnsureSSHAvailable() error {
	if _, err := exec.LookPath("ssh"); err != nil {
		return fmt.Errorf("the 'ssh' binary was not found on your PATH — railctl exec shells out to your local ssh; install OpenSSH and retry")
	}
	return nil
}

// ExecOpts describes one `railctl exec` invocation for ExecArgs.
type ExecOpts struct {
	// InstanceID is the service instance UUID used as the SSH username
	// (the relay's routing key).
	InstanceID string
	// IdentityFile, when non-empty, is passed to ssh via -i (a specific
	// private key). Empty lets ssh/agent present the default key.
	IdentityFile string
	// Command is the remote command and its args. Empty/nil means an
	// interactive shell.
	Command []string
	// WantTTY requests a PTY (-t); otherwise -T disables it. Callers derive
	// this from terminal state via WantTTY().
	WantTTY bool
}

// ExecArgs builds the exact `ssh` argv for an exec/interactive session. Flag
// order:
//
//	[-p <port>]  (only when the relay port is non-default)
//	[-i <identityFile>]
//	-t | -T
//	<instanceID>@ssh.railway.com
//	[-- <cmd> [args...]]   (each command token shell-quoted, see below)
//
// Remote-command argv preservation (kubectl-consistent): ssh joins every
// argument after the destination into ONE string and hands it to the remote
// login shell, which re-tokenizes it. If we appended the tokens verbatim (as
// Railway's Rust CLI does), a token carrying shell metacharacters would be
// re-split remotely — e.g. `exec svc -- sh -c 'echo a; echo b'` arrives as
// `sh -c echo a; echo b`, so the remote `sh -c echo` runs with $0=a and the
// rest leaks to the outer shell. `railctl exec` advertises kubectl-exec
// semantics, where the argv the user passed is the argv the container sees, so
// we single-quote each token: the remote shell then reconstructs the exact
// tokens. A `--` separator is still emitted so the local ssh never parses a
// remote arg as one of its own flags.
func ExecArgs(opts ExecOpts) []string {
	args := baseArgs(opts.IdentityFile)

	if opts.WantTTY {
		args = append(args, "-t")
	} else {
		args = append(args, "-T")
	}

	args = append(args, target(opts.InstanceID))

	if len(opts.Command) > 0 {
		args = append(args, "--")
		for _, tok := range opts.Command {
			args = append(args, shellQuote(tok))
		}
	}
	return args
}

// shellQuote wraps s so a POSIX remote shell reconstructs it as a single token,
// exactly as passed — the standard "close the quote, escape a literal ', reopen"
// dance for embedded single quotes. Empty stays as ” (an explicit empty arg).
func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	// Fast path: tokens made only of shell-safe characters need no quoting,
	// keeping the common case (bare command names, flags, paths) readable.
	safe := true
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' ||
			r == '.' || r == '/' || r == ':' || r == '@' || r == '%' || r == '+' || r == ',' || r == '=') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	out := make([]byte, 0, len(s)+2)
	out = append(out, '\'')
	for i := 0; i < len(s); i++ {
		if s[i] == '\'' {
			out = append(out, '\'', '\\', '\'', '\'')
			continue
		}
		out = append(out, s[i])
	}
	out = append(out, '\'')
	return string(out)
}

// baseArgs emits the relay port (only when non-default) and the identity file
// (when set), shared by every ssh mode so the setup can't drift.
func baseArgs(identityFile string) []string {
	var args []string
	if RelayPort != 22 {
		args = append(args, "-p", strconv.Itoa(RelayPort))
	}
	if identityFile != "" {
		args = append(args, "-i", identityFile)
	}
	// StrictHostKeyChecking=accept-new: trust the relay on first contact
	// (TOFU) while still rejecting a changed key. Matches Railway's posture.
	args = append(args, "-o", "StrictHostKeyChecking=accept-new")
	return args
}

// target is the `<instanceID>@<relay-host>` SSH destination.
func target(instanceID string) string {
	return instanceID + "@" + RelayHost
}

// WantTTY decides PTY allocation, mirroring native.rs:265-270 (docker/kubectl
// exec semantics):
//
//	command + both stdin&stdout TTYs → -t  (interactive tools like vim/htop work)
//	command + not both TTYs          → -T  (clean pipes for scripts)
//	no command + stdin is a TTY      → -t  (interactive shell)
//	no command + stdin not a TTY     → -T  (avoid mangling piped stdin)
func WantTTY(hasCommand, stdinTTY, stdoutTTY bool) bool {
	if hasCommand {
		return stdinTTY && stdoutTTY
	}
	return stdinTTY
}
