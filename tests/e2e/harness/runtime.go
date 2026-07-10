//go:build e2e

package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// RunEnv is Run with extra environment variables layered onto the process
// environment (e.g. RAILCTL_NO_HINTS=1). It otherwise behaves exactly like Run:
// injects --token, never fails the test, 3-minute timeout.
func (e *Env) RunEnv(extraEnv []string, args ...string) Result {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	fullArgs := append([]string{"--token", e.Token}, args...)
	cmd := exec.CommandContext(ctx, Railctl, fullArgs...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	code := 0
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			code = -1
			stderr.WriteString("\n[TIMEOUT] command exceeded 3-minute deadline")
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), ExitCode: code}
}

// BgProc is a railctl process started in the background (e.g. `port-forward`,
// which blocks in the foreground until interrupted). Stop() sends an interrupt
// and waits, mirroring a user's Ctrl-C.
type BgProc struct {
	cmd    *exec.Cmd
	stderr *bytes.Buffer
	done   chan error
}

// StartBackground launches railctl with the suite token and returns a handle
// without waiting for it to exit. Stdout is discarded; stderr is captured for
// diagnostics (read it only after Stop()). Use for foreground-blocking
// commands like port-forward.
func (e *Env) StartBackground(args ...string) *BgProc {
	fullArgs := append([]string{"--token", e.Token}, args...)
	cmd := exec.Command(Railctl, fullArgs...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	b := &BgProc{cmd: cmd, stderr: &stderr, done: make(chan error, 1)}
	if err := cmd.Start(); err != nil {
		b.done <- err
		return b
	}
	go func() { b.done <- cmd.Wait() }()
	return b
}

// Stop sends an interrupt (Ctrl-C equivalent) and waits up to 10s for the
// process to exit, killing it if it overstays. Returns the captured stderr for
// diagnostics.
func (b *BgProc) Stop() string {
	if b.cmd.Process != nil {
		_ = b.cmd.Process.Signal(os.Interrupt)
	}
	select {
	case <-b.done:
	case <-time.After(10 * time.Second):
		_ = b.cmd.Process.Kill()
		<-b.done
	}
	return b.stderr.String()
}

// WaitForDeploymentSuccess polls `get deployments` for the given service until
// the latest deployment reports SUCCESS, or fails on a terminal status /
// timeout. extraFlags carries scope flags (-p/-e) for tokens that need them; a
// project token runs flag-free. exec/port-forward need a live container, so
// tests call this before dialing the relay.
func WaitForDeploymentSuccess(e *Env, service string, extraFlags ...string) error {
	const attempts = 25
	for i := 0; i < attempts; i++ {
		args := append([]string{"get", "deployments", "-s", service, "-o", "json", "--limit", "1"}, extraFlags...)
		r := e.Run(args...)
		if r.ExitCode == 0 {
			var deps []struct {
				Status string `json:"status"`
			}
			if json.Unmarshal([]byte(r.Stdout), &deps) == nil && len(deps) > 0 {
				switch deps[0].Status {
				case "SUCCESS":
					e.T.Logf("Deployment for %s reached SUCCESS after %d poll(s)", service, i+1)
					return nil
				case "FAILED", "CRASHED", "REMOVED", "SKIPPED":
					return fmt.Errorf("deployment for %s reached terminal status %s", service, deps[0].Status)
				}
			}
		}
		time.Sleep(12 * time.Second)
	}
	return fmt.Errorf("deployment for %s did not reach SUCCESS after %d attempts", service, attempts)
}

// AssertContainsAny fails unless haystack contains at least one needle. Useful
// when the exact wording may vary but any of several stable tokens proves it.
func AssertContainsAny(t *testing.T, haystack string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return
		}
	}
	t.Errorf("expected output to contain one of %v, got:\n%s", needles, haystack)
}
