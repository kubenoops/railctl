package sshx

import (
	"context"
	"os"
	"os/exec"
	"testing"
)

func TestFingerprint_MatchesSSHKeygen(t *testing.T) {
	// A real, well-known test key; compare against ssh-keygen if available,
	// otherwise assert the SHA256: shape and stability.
	pub := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIOMqqnkVzrm0SdG6UOoqKLsabgH5C9okWi0dh2l9GKJl test@example"

	fp, err := Fingerprint(pub)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fp) < len("SHA256:") || fp[:7] != "SHA256:" {
		t.Fatalf("fingerprint %q is not in SHA256: form", fp)
	}

	if path, lerr := exec.LookPath("ssh-keygen"); lerr == nil {
		f, err := os.CreateTemp(t.TempDir(), "key*.pub")
		if err != nil {
			t.Fatalf("temp: %v", err)
		}
		if _, err := f.WriteString(pub + "\n"); err != nil {
			t.Fatalf("write: %v", err)
		}
		f.Close()
		out, err := exec.Command(path, "-lf", f.Name()).Output()
		if err != nil {
			t.Skipf("ssh-keygen -lf failed, skipping cross-check: %v", err)
		}
		// Output: "256 SHA256:xxxx comment (ED25519)"
		if want := parseKeygenFP(string(out)); want != "" && want != fp {
			t.Errorf("Fingerprint()=%q, ssh-keygen=%q", fp, want)
		}
	}
}

func parseKeygenFP(line string) string {
	for _, f := range splitFields(line) {
		if len(f) > 7 && f[:7] == "SHA256:" {
			return f
		}
	}
	return ""
}

func splitFields(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func TestFingerprint_Malformed(t *testing.T) {
	if _, err := Fingerprint("garbage"); err == nil {
		t.Error("expected error for malformed key (single field)")
	}
	if _, err := Fingerprint("ssh-ed25519 !!!notbase64!!! c"); err == nil {
		t.Error("expected error for bad base64")
	}
}

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
