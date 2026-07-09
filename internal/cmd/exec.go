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
	// execDiscoverPublicKey is the key-discovery seam, overridable in tests.
	execDiscoverPublicKey = defaultDiscoverPublicKey
)

// execCmd represents the `railctl exec` command.
var execCmd = &cobra.Command{
	Use:   "exec <service> [-- command [args...]]",
	Short: "Run a command or open a shell in a service container over SSH",
	Long: `Open an interactive shell or run a one-off command inside a running
Railway service container, kubectl-exec style, over Railway's SSH relay.

railctl shells out to your local 'ssh' binary and dials Railway's global relay
(ssh.railway.com). The relay brokers the session into the container like
'docker exec' — the container needs NO sshd of its own. You DO need a local
'ssh' binary and an SSH key you've registered with Railway (railctl registers
your existing public key automatically the first time).

Token scope: exec needs an ACCOUNT or WORKSPACE token. SSH keys attach to a
user or workspace, never a project, so a project-scoped token cannot register a
key and exec fails fast under one.

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

	// --- Token gate (fail fast, BEFORE resolving anything) ---
	isProject, err := client.IsProjectToken()
	if err != nil {
		return err
	}
	if isProject {
		return errors.New("railctl exec requires an account or workspace token — SSH keys cannot be registered with a project-scoped token (Railway ties keys to a user/workspace, not a project). Re-run with a workspace or account token.")
	}

	// Derive the workspace to register the key under. For a workspace token
	// this is its own workspace; for an account token it is the -w-selected /
	// sole workspace, or "" (a personal key) when ambiguous — mirroring
	// Railway's own null-and-let-the-resolver-default behavior.
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
	instanceID := execInstanceID
	if instanceID == "" {
		instanceID, err = client.GetServiceInstanceID(rctx.Environment.ID, rctx.Service.ID)
		if err != nil {
			return fmt.Errorf("failed to resolve the service instance: %w", err)
		}
	}

	// --- Ensure the local public key is registered (idempotent) ---
	pubKeyPath, err := execDiscoverPublicKey(execIdentityFile)
	if err != nil {
		return err
	}
	if err := ensureKeyRegistered(client, pubKeyPath, workspaceID); err != nil {
		return err
	}

	// --- Build the ssh argv and run it, propagating the exit code ---
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
			// Propagate the remote command's exit code as railctl's own.
			os.Exit(exitErr.Code)
		}
		return runErr
	}
	return nil
}

// ensureKeyRegistered registers the local public key with Railway unless a key
// with the same fingerprint is already present. It prints a one-line note to
// stderr the first time it registers a key.
func ensureKeyRegistered(client api.APIClient, pubKeyPath, workspaceID string) error {
	pubKey, err := sshx.ReadPublicKey(pubKeyPath)
	if err != nil {
		return err
	}
	fp, err := sshx.Fingerprint(pubKey)
	if err != nil {
		return err
	}

	existing, err := client.ListSSHKeys(workspaceID)
	if err != nil {
		return err
	}
	for _, k := range existing {
		if k.Fingerprint == fp {
			return nil // already registered — nothing to do
		}
	}

	name := sshKeyName()
	key, err := client.RegisterSSHKey(name, pubKey, workspaceID)
	if err != nil {
		return fmt.Errorf("failed to register SSH key with Railway: %w", err)
	}
	fingerprint := key.Fingerprint
	if fingerprint == "" {
		fingerprint = fp
	}
	fmt.Fprintf(os.Stderr, "Registered SSH key %s (%s) with your workspace\n", name, fingerprint)
	return nil
}

// sshKeyName is a recognizable name for the registered key so the user can find
// and remove it later.
func sshKeyName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "railctl"
	}
	return "railctl@" + host
}

// defaultDiscoverPublicKey finds the public key to register/connect with,
// honoring an -i/--identity-file override.
func defaultDiscoverPublicKey(identityFile string) (string, error) {
	sshDir, err := sshx.DefaultSSHDir()
	if err != nil {
		return "", err
	}
	return sshx.DiscoverPublicKey(sshDir, identityFile)
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
