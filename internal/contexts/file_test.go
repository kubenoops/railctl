package contexts

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Load must treat a missing config file as "no config, no error" so that
// machines without a config behave exactly like pre-context railctl (FR-004).
func TestLoad_MissingFileReturnsNil(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	f, err := Load(path)

	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}
	if f != nil {
		t.Fatalf("expected nil file for missing file, got %+v", f)
	}
}

// Save must persist the whole aggregate and create the file with 0600
// permissions; Load must read it back identically (FR-001, FR-002).
func TestSaveLoad_Roundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	in := &File{
		CurrentContext: "shop-prod",
		Contexts: []Context{{
			Name:        "shop-prod",
			Workspace:   "my-team",
			Project:     "shop",
			Environment: "production",
			Credentials: "work",
		}},
		Credentials: []Credential{{Name: "work", Token: "secret-token", TokenType: "workspace"}},
	}

	if err := Save(path, in); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after save: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected file mode 0600, got %o", perm)
	}

	out, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("roundtrip mismatch:\n got %+v\nwant %+v", out, in)
	}
}

// Saving must bootstrap the config directory when it does not exist yet —
// on a fresh machine ~/.railctl is absent and the first write (the
// feature's bootstrap) must create it, not fail (PR #96 review).
func TestSave_CreatesMissingParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", ".railctl", "config.yaml")

	if err := Save(path, &File{CurrentContext: "a", Contexts: []Context{{Name: "a", Project: "p"}}}); err != nil {
		t.Fatalf("Save to a path with missing parent directories failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after save: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected file mode 0600, got %o", perm)
	}
	if _, err := Load(path); err != nil {
		t.Fatalf("Load after bootstrap save: %v", err)
	}
}

// A config file with group or other permission bits must be refused with an
// error naming the path and the chmod remediation (FR-002).
func TestLoad_RefusesLoosePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("currentContext: x\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Load(path)

	if err == nil {
		t.Fatal("expected permission refusal error, got nil")
	}
	if want := "chmod 600 " + path; !strings.Contains(err.Error(), want) {
		t.Fatalf("error must contain %q, got: %v", want, err)
	}
}

// A corrupt config file must fail with an error naming the file path —
// never a silent fallback to context-less behavior (Edge Case 1).
func TestLoad_CorruptYAMLNamesPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("contexts: [unclosed"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Load(path)

	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Fatalf("error must name the file path %q, got: %v", path, err)
	}
}

// A context referencing a credential name that does not exist must fail at
// load, naming both the context and the missing credential (FR-021).
func TestLoad_DanglingCredentialReferenceFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	yaml := `currentContext: shop-prod
contexts:
  - name: shop-prod
    project: shop
    credentials: work
credentials:
  - name: other
    token: t
    tokenType: account
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	_, err := Load(path)

	if err == nil {
		t.Fatal("expected dangling-reference error, got nil")
	}
	for _, want := range []string{"shop-prod", "work"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must name %q, got: %v", want, err)
		}
	}
}

// Path resolution: RAILCTL_CONFIG overrides the default location; without it
// the path is ~/.railctl/config.yaml (FR-001).
func TestConfigPath_EnvOverrideAndDefault(t *testing.T) {
	t.Setenv("RAILCTL_CONFIG", "/tmp/custom/railctl.yaml")
	if got := ConfigPath(); got != "/tmp/custom/railctl.yaml" {
		t.Fatalf("expected RAILCTL_CONFIG value, got %q", got)
	}

	t.Setenv("RAILCTL_CONFIG", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("UserHomeDir: %v", err)
	}
	want := filepath.Join(home, ".railctl", "config.yaml")
	if got := ConfigPath(); got != want {
		t.Fatalf("expected default %q, got %q", want, got)
	}
}
