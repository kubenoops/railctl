package sshx

import (
	"context"
	"os/exec"
	"testing"
)

func TestEnsureSSHAvailable(t *testing.T) {
	// In CI/dev ssh is normally present; assert no error when it is, and the
	// error path by hiding PATH.
	if _, err := exec.LookPath("ssh"); err == nil {
		if err := EnsureSSHAvailable(); err != nil {
			t.Errorf("ssh is on PATH but EnsureSSHAvailable errored: %v", err)
		}
	}

	t.Setenv("PATH", "")
	if err := EnsureSSHAvailable(); err == nil {
		t.Error("expected an error when ssh is not on PATH")
	}
}

// recordingRunner is a fake Runner used to assert the Runner seam contract.
type recordingRunner struct {
	gotArgv []string
	ret     error
}

func (r *recordingRunner) Run(_ context.Context, argv []string, _ Stdio) error {
	r.gotArgv = argv
	return r.ret
}

func TestExitError(t *testing.T) {
	e := &ExitError{Code: 42}
	if e.Error() == "" {
		t.Error("ExitError.Error() is empty")
	}
	if e.Code != 42 {
		t.Errorf("Code = %d, want 42", e.Code)
	}
}

// Confirms a Runner is satisfiable by a test double (compile-time + behavior).
func TestRunnerSeam(t *testing.T) {
	var r Runner = &recordingRunner{ret: &ExitError{Code: 7}}
	err := r.Run(context.Background(), []string{"-T", "x@ssh.railway.com"}, Stdio{})
	var exitErr *ExitError
	if !asExit(err, &exitErr) || exitErr.Code != 7 {
		t.Errorf("expected ExitError code 7, got %v", err)
	}
}

func asExit(err error, target **ExitError) bool {
	if e, ok := err.(*ExitError); ok {
		*target = e
		return true
	}
	return false
}
