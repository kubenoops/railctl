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
)

// portForwardCmd represents the `railctl port-forward` command.
var portForwardCmd = &cobra.Command{
	Use:   "port-forward <service> [LOCAL:]REMOTE [[LOCAL:]REMOTE ...]",
	Short: "Forward local ports to a service over SSH (kubectl-style)",
	Long: `Forward one or more local ports to a Railway service container over
Railway's SSH relay, kubectl-port-forward style. Multiple ports ride ONE ssh
connection. Runs in the foreground and streams until you press Ctrl-C.

railctl shells out to your local 'ssh' binary and dials Railway's global relay
(ssh.railway.com). The relay brokers the forward into the named service's own
container — the container needs NO sshd of its own. You DO need a local 'ssh'
binary and an SSH key you've registered ONCE with Railway at
https://railway.com/account/ssh-keys (railctl does not manage keys).

Token scope: port-forward works with ANY token (account, workspace, or
project) — the token is used only to resolve the service instance.
Authentication is by your SSH key, not the token.

Reaching a PRIVATE service: forward directly INTO it — name the private
service and it works even with no public domain/proxy (kubectl's model: you
port-forward the target itself, not a bastion). Railway's relay forwards only
to the target container's OWN loopback, so there is no jump/bastion form.

Port-spec grammar:

  REMOTE          e.g. 8080       → localhost:8080 -> the service's 127.0.0.1:8080
  LOCAL:REMOTE    e.g. 6543:5432  → localhost:6543 -> the service's 127.0.0.1:5432

The remote side is always the service's own loopback (127.0.0.1). NOTE: the
target must LISTEN on IPv4 loopback/0.0.0.0. A service that binds IPv6-only
([::]) is not reachable this way (an SSH -L to loopback finds nothing).

Examples:
  # Forward localhost:5432 -> the db service's own 127.0.0.1:5432
  railctl port-forward db 5432 -p my-project -e production

  # Reach a PRIVATE apiserver directly (no public exposure needed)
  railctl port-forward kube-apiserver 6443 -p my-project -e production

  # Map a different local port
  railctl port-forward db 6543:5432 -p my-project -e production

  # Multiple ports over one connection
  railctl port-forward db 5432 6379 -p my-project -e production

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

	// --- Build the ssh argv ---
	// No key discovery/registration: authentication is by the user's SSH key,
	// which they register once at https://railway.com/account/ssh-keys. When
	// -i/--identity-file is unset, ssh uses its own defaults (agent, ~/.ssh).
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
			// A non-zero ssh exit that we did NOT initiate is often a
			// publickey/permission failure. Surface the actionable key hint
			// after ssh's own error, then propagate the exit code.
			fmt.Fprintln(os.Stderr, sshKeyHint)
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
		// The remote side is always the service's own loopback.
		fmt.Fprintf(w, "Forwarding: %s:%d → %s:%d … (Ctrl-C to stop)\n",
			host, f.LocalPort, serviceName, f.RemotePort)
	}
}
