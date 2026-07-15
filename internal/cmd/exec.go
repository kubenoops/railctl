package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/sshx"
	"github.com/spf13/cobra"
)

var (
	execIdentityFile string
	execInstanceID   string

	// execRunner is the SSH runner used by `railctl exec`. Overridable in
	// tests to assert the argv without ever launching ssh.
	execRunner sshx.Runner = sshx.ExecRunner{}
)

// sshKeyHint is the actionable one-liner printed to stderr when an ssh child
// exits non-zero — most commonly a publickey/permission failure because the
// user has not registered their SSH key. railctl does not manage keys.
const sshKeyHint = "If this failed with a publickey/permission error, register your SSH key at https://railway.com/account/ssh-keys, then retry."

// warnIfUnknownReplica checks a user-supplied --deployment-instance id against
// the service's current replicas and prints a stderr hint if it's not found.
//
// It deliberately fails OPEN — it warns and returns, never blocking the
// connection. Two reasons: (1) the relay also accepts the service-instance id
// (which ListReplicas does not return, since that's the parent, not a replica),
// so a "not found" id can still be a valid target; (2) if replica discovery
// itself fails, a hint must never stop a real connection. The value is catching
// the common case — a stale or mistyped replica id — before ssh drops the user
// into Railway's relay picker with a cryptic "No target found".
func warnIfUnknownReplica(client api.APIClient, envID, serviceID, instanceID string) {
	list, err := client.ListReplicas(envID, serviceID)
	if err != nil {
		return // discovery failed — don't block the connect over a hint
	}
	for _, r := range list.Replicas {
		if r.ID == instanceID {
			return // known replica — connect silently
		}
	}
	fmt.Fprintf(os.Stderr,
		"⚠️  Warning: instance %s is not among this service's current replicas.\n"+
			"   Run 'railctl get replicas' to list valid instance IDs. Attempting to connect anyway...\n",
		instanceID)
}

// execCmd represents the `railctl exec` command.
var execCmd = &cobra.Command{
	Use:   "exec <service> [-- command [args...]]",
	Short: "Run a command or open a shell in a service container over SSH",
	Long: `Open an interactive shell or run a one-off command inside a running
Railway service container, kubectl-exec style, over Railway's SSH relay.

railctl shells out to your local 'ssh' binary and dials Railway's global relay
(ssh.railway.com). The relay brokers the session into the container like
'docker exec' — the container needs NO sshd of its own. You DO need a local
'ssh' binary and an SSH key you've registered ONCE with Railway at
https://railway.com/account/ssh-keys (railctl does not manage keys).

Token scope: exec works with ANY token (account, workspace, or project) — the
token is used only to resolve the service instance. Authentication is by your
SSH key, not the token.

The service is a positional argument (like 'logs <service>'). Everything after
'--' is the remote command, passed verbatim; omit it for an interactive shell.

Examples:
  # Interactive shell into the first active instance of 'api'
  railctl exec api -p my-project -e production

  # Run a one-off command and propagate its exit code
  railctl exec api -p my-project -e production -- ls -la /data

  # Use a specific private key
  railctl exec api -p my-project -e production -i ~/.ssh/id_ed25519 -- env

  # Target a specific instance id (skip the instance lookup)
  railctl exec api -p my-project -e production --deployment-instance <id>
`,
	Args:               cobra.MinimumNArgs(1),
	DisableFlagParsing: false,
	RunE:               runExec,
}

func init() {
	rootCmd.AddCommand(execCmd)
	// railctl's own flags (-p/-e/-i/--deployment-instance) parse anywhere
	// before `--`; everything after `--` is the remote command, verbatim
	// (split via ArgsLenAtDash in runExec). Mirrors `kubectl exec pod -it -- cmd`.
	execCmd.Flags().SetInterspersed(true)
	execCmd.Flags().StringVarP(&execIdentityFile, "identity-file", "i", "",
		"SSH private key to use (default: your ~/.ssh default key or ssh-agent)")
	execCmd.Flags().StringVar(&execInstanceID, "deployment-instance", "",
		"Service instance id to target (advanced; skips the instance lookup)")
}

func runExec(cmd *cobra.Command, args []string) error {
	serviceName := args[0]
	// The remote command is only what follows `--`. Args between the service
	// name and `--` are not a command (they'd be railctl flags, already parsed).
	var remoteCmd []string
	if dash := cmd.ArgsLenAtDash(); dash >= 0 {
		remoteCmd = args[dash:]
	} else if len(args) > 1 {
		// No `--`: extra bare positionals are taken as the command (lenient,
		// works for flagless commands like `exec api ls`).
		remoteCmd = args[1:]
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
	instanceID := execInstanceID
	if instanceID == "" {
		// Default path: let the relay pick a replica. No extra API call.
		instanceID, err = client.GetServiceInstanceID(rctx.Environment.ID, rctx.Service.ID)
		if err != nil {
			return fmt.Errorf("failed to resolve the service instance: %w", err)
		}
	} else {
		// User targeted a specific replica — validate softly (warn, don't block).
		warnIfUnknownReplica(client, rctx.Environment.ID, rctx.Service.ID, instanceID)
	}

	// --- Build the ssh argv and run it, propagating the exit code ---
	// No key discovery/registration: authentication is by the user's SSH key,
	// which they register once at https://railway.com/account/ssh-keys. When
	// -i/--identity-file is unset, ssh uses its own defaults (agent, ~/.ssh).
	stdinTTY := isTerminal(os.Stdin)
	stdoutTTY := isTerminal(os.Stdout)
	argv := sshx.ExecArgs(sshx.ExecOpts{
		InstanceID:   instanceID,
		IdentityFile: execIdentityFile,
		Command:      remoteCmd,
		WantTTY:      sshx.WantTTY(len(remoteCmd) > 0, stdinTTY, stdoutTTY),
	})

	runErr := execRunner.Run(context.Background(), argv, sshx.Stdio{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if runErr != nil {
		var exitErr *sshx.ExitError
		if errors.As(runErr, &exitErr) {
			// A non-zero ssh exit is often a publickey/permission failure. Surface
			// the actionable key-registration hint after ssh's own error, then
			// propagate the exit code as railctl's own.
			fmt.Fprintln(os.Stderr, sshKeyHint)
			os.Exit(exitErr.Code)
		}
		return runErr
	}
	return nil
}

// isTerminal reports whether f is attached to a terminal (character device).
// This avoids adding a golang.org/x/term dependency: an interactive TTY is a
// character device, while pipes/files/redirects are not.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
