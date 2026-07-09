package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/sshx"
	"github.com/spf13/cobra"
)

var (
	pfIdentityFile string
	pfInstanceID   string
	pfAddress      string

	// pfRunner is the SSH runner used by `railctl port-forward`. Overridable in
	// tests to assert the argv without ever launching ssh.
	pfRunner sshx.Runner = sshx.ExecRunner{}
	// pfDiscoverPublicKey is the key-discovery seam, overridable in tests.
	pfDiscoverPublicKey = defaultDiscoverPublicKey
)

// portForwardCmd represents the `railctl port-forward` command.
var portForwardCmd = &cobra.Command{
	Use:   "port-forward <service> [LOCAL:]REMOTE [[LOCAL:]HOST:REMOTE ...]",
	Short: "Forward local ports to a service over SSH (kubectl-style)",
	Long: `Forward one or more local ports to a Railway service container over
Railway's SSH relay, kubectl-port-forward style. Multiple ports ride ONE ssh
connection. Runs in the foreground and streams until you press Ctrl-C.

railctl shells out to your local 'ssh' binary and dials Railway's global relay
(ssh.railway.com). The relay brokers the forward into the container — the
container needs NO sshd of its own. You DO need a local 'ssh' binary and an SSH
key you've registered with Railway (railctl registers your existing public key
automatically the first time).

Token scope: port-forward needs an ACCOUNT or WORKSPACE token. SSH keys attach
to a user or workspace, never a project, so a project-scoped token cannot
register a key and port-forward fails fast under one.

Port-spec grammar (three forms):

  REMOTE              e.g. 8080        → localhost:8080  -> 127.0.0.1:8080 in the service
  LOCAL:REMOTE        e.g. 6543:5432   → localhost:6543  -> 127.0.0.1:5432 in the service
  LOCAL:HOST:REMOTE   the JUMP form    → localhost:LOCAL -> HOST:REMOTE (a DIFFERENT private host)

The one- and two-field forms always pin the remote host to the literal
127.0.0.1 (the service's own loopback) — a bare number can never smuggle
"localhost", which the relay resolves to an unreachable mesh address.

The three-field JUMP form is the headline: reach a DIFFERENT private,
unexposed host (a *.railway.internal DNS name) THROUGH a chosen jump service
whose container sits on the environment's private network. The relay resolves
the internal name server-side from inside the jump container's netns.

  IMPORTANT: *.railway.internal names resolve to an IPv6 address. A jump
  forward only reaches internal targets that bind their internal/IPv6
  interface ([::]:port). An IPv4-only service (e.g. default nginx on
  0.0.0.0:80) gives "empty reply" — a target-binding property, not a railctl
  bug.

Examples:
  # Forward localhost:5432 -> the db service's own 127.0.0.1:5432
  railctl port-forward db 5432 -p my-project -e production

  # Map a different local port
  railctl port-forward db 6543:5432 -p my-project -e production

  # Multiple ports over one connection
  railctl port-forward db 5432 6379 -p my-project -e production

  # JUMP form: reach a private apiserver THROUGH a jump service
  railctl port-forward jump-svc 6443:kube-apiserver.railway.internal:6443 -p my-project -e production

  # Share the forward on the LAN (opt-in) and use a specific key
  railctl port-forward db 5432 --address 0.0.0.0 -i ~/.ssh/id_ed25519
`,
	Args: cobra.MinimumNArgs(2),
	RunE: runPortForward,
}

func init() {
	rootCmd.AddCommand(portForwardCmd)
	// railctl's own flags parse anywhere; every bare positional after the
	// service is a port spec (there is no `--`-delimited remote command).
	portForwardCmd.Flags().SetInterspersed(true)
	portForwardCmd.Flags().StringVarP(&pfIdentityFile, "identity-file", "i", "",
		"SSH private key to use (default: your ~/.ssh default key or ssh-agent)")
	portForwardCmd.Flags().StringVar(&pfInstanceID, "deployment-instance", "",
		"Service instance id to target (advanced; skips the instance lookup)")
	portForwardCmd.Flags().StringVar(&pfAddress, "address", "127.0.0.1",
		"Local bind address for the forwarded ports (use 0.0.0.0 to share on the LAN)")
}

func runPortForward(cmd *cobra.Command, args []string) error {
	serviceName := args[0]
	specArgs := args[1:]

	// Parse every port spec up front so a malformed spec fails before any API
	// or ssh work.
	forwards := make([]sshx.PortForward, 0, len(specArgs))
	for _, s := range specArgs {
		pf, err := sshx.ParsePortSpec(s)
		if err != nil {
			return err
		}
		forwards = append(forwards, pf)
	}

	// Fail fast if the local ssh binary is missing before doing any API work.
	if err := sshx.EnsureSSHAvailable(); err != nil {
		return err
	}

	tkn, err := getToken()
	if err != nil {
		return err
	}
	client := newAPIClient(tkn)

	// --- Token gate (fail fast, BEFORE resolving anything) ---
	isProject, err := client.IsProjectToken()
	if err != nil {
		return err
	}
	if isProject {
		return errors.New("railctl port-forward requires an account or workspace token — SSH keys cannot be registered with a project-scoped token (Railway ties keys to a user/workspace, not a project). Re-run with a workspace or account token.")
	}

	workspaceID, err := client.GetWorkspaceID()
	if err != nil {
		return err
	}

	// --- Resolve project → environment → service ---
	rctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		ServiceName:     serviceName,
		NeedEnvironment: true,
		NeedService:     true,
	})
	if err != nil {
		return err
	}

	// --- Resolve the connectable instance id (the SSH username) ---
	instanceID := pfInstanceID
	if instanceID == "" {
		instanceID, err = client.GetServiceInstanceID(rctx.Environment.ID, rctx.Service.ID)
		if err != nil {
			return fmt.Errorf("failed to resolve the service instance: %w", err)
		}
	}

	// --- Ensure the local public key is registered (idempotent) ---
	pubKeyPath, err := pfDiscoverPublicKey(pfIdentityFile)
	if err != nil {
		return err
	}
	if err := ensureKeyRegistered(client, pubKeyPath, workspaceID); err != nil {
		return err
	}

	// --- Build the ssh argv ---
	argv := sshx.ForwardArgs(sshx.ForwardOpts{
		InstanceID:   instanceID,
		IdentityFile: pfIdentityFile,
		Forwards:     forwards,
		Address:      pfAddress,
	})

	// --- Friendly banner (stderr, keep stdout clean) ---
	printForwardBanner(os.Stderr, serviceName, forwards)

	// --- Signal handling: SIGINT/SIGTERM cancels the context, which kills the
	// ssh child (it runs -N in the foreground inheriting stdio). ---
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Fprintln(os.Stderr, "\nStopping port-forward...")
		cancel()
	}()

	runErr := pfRunner.Run(ctx, argv, sshx.Stdio{
		// -N runs no command: no stdin, no stdout; keep stderr for relay/auth
		// and per-forward failures (ExitOnForwardFailure).
		Stderr: os.Stderr,
	})
	if runErr != nil {
		// A Ctrl-C tear-down is the normal way to stop a forward: swallow the
		// resulting non-zero exit when we initiated the cancel.
		if ctx.Err() != nil {
			return nil
		}
		var exitErr *sshx.ExitError
		if errors.As(runErr, &exitErr) {
			os.Exit(exitErr.Code)
		}
		return runErr
	}
	return nil
}

// printForwardBanner writes one human-readable line per forward before the
// command blocks, so the user sees exactly what is being tunneled.
func printForwardBanner(w *os.File, serviceName string, forwards []sshx.PortForward) {
	bind := pfAddress
	if bind == "" {
		bind = "127.0.0.1"
	}
	host := "localhost"
	if bind != "127.0.0.1" {
		host = bind
	}
	for _, f := range forwards {
		if f.RemoteHost == "" || f.RemoteHost == "127.0.0.1" {
			// Direct forward into the service's own loopback.
			fmt.Fprintf(w, "Forwarding: %s:%d → %s:%d … (Ctrl-C to stop)\n",
				host, f.LocalPort, serviceName, f.RemotePort)
		} else {
			// Jump form: reaching a different private host via the service.
			fmt.Fprintf(w, "Forwarding: %s:%d → %s:%d (via %s) … (Ctrl-C to stop)\n",
				host, f.LocalPort, f.RemoteHost, f.RemotePort, serviceName)
		}
	}
}
