package diff

import (
	"testing"

	"github.com/kubenoops/railctl/internal/config"
)

func TestCompute_CreateNew(t *testing.T) {
	desired := []config.ServiceConfig{
		{
			Name:  "web",
			Image: "node:20-alpine",
			Deploy: config.DeployConfig{
				StartCommand: "npm start",
			},
			Variables: map[string]string{"PORT": "3000"},
		},
		{
			Name:  "worker",
			Image: "python:3.12",
		},
	}

	cs := Compute(desired, nil, false)

	if len(cs.Changes) != 2 {
		t.Fatalf("expected 2 changes, got %d", len(cs.Changes))
	}

	for _, rc := range cs.Changes {
		if rc.Type != ChangeCreate {
			t.Errorf("expected ChangeCreate for service %q, got %d", rc.ServiceName, rc.Type)
		}
	}

	// Check first service has expected fields.
	web := cs.Changes[0]
	if web.ServiceName != "web" {
		t.Errorf("expected service name 'web', got %q", web.ServiceName)
	}
	fieldPaths := make(map[string]string)
	for _, f := range web.Fields {
		fieldPaths[f.Path] = f.Desired
	}
	if fieldPaths["image"] != "node:20-alpine" {
		t.Errorf("expected image 'node:20-alpine', got %q", fieldPaths["image"])
	}
	if fieldPaths["deploy.startCommand"] != "npm start" {
		t.Errorf("expected startCommand 'npm start', got %q", fieldPaths["deploy.startCommand"])
	}
	if fieldPaths["variables.PORT"] != "3000" {
		t.Errorf("expected variables.PORT '3000', got %q", fieldPaths["variables.PORT"])
	}
}

func TestCompute_NoChanges(t *testing.T) {
	desired := []config.ServiceConfig{
		{
			Name:  "web",
			Image: "node:20-alpine",
			Deploy: config.DeployConfig{
				Replicas: 2,
			},
			Variables: map[string]string{"PORT": "3000"},
		},
	}

	live := []LiveService{
		{
			Name:  "web",
			Image: "node:20-alpine",
			Deploy: LiveDeployConfig{
				Replicas: 2,
			},
			Variables: map[string]string{"PORT": "3000"},
		},
	}

	cs := Compute(desired, live, false)

	if cs.HasChanges() {
		t.Errorf("expected no changes, got %d changes", len(cs.Changes))
	}
}

func TestCompute_UpdateImage(t *testing.T) {
	desired := []config.ServiceConfig{
		{Name: "web", Image: "node:20-alpine"},
	}
	live := []LiveService{
		{Name: "web", Image: "node:18-alpine"},
	}

	cs := Compute(desired, live, false)

	if len(cs.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(cs.Changes))
	}

	rc := cs.Changes[0]
	if rc.Type != ChangeUpdate {
		t.Errorf("expected ChangeUpdate, got %d", rc.Type)
	}

	found := false
	for _, f := range rc.Fields {
		if f.Path == "image" {
			found = true
			if f.Current != "node:18-alpine" {
				t.Errorf("expected current 'node:18-alpine', got %q", f.Current)
			}
			if f.Desired != "node:20-alpine" {
				t.Errorf("expected desired 'node:20-alpine', got %q", f.Desired)
			}
		}
	}
	if !found {
		t.Error("expected image field diff not found")
	}
}

func TestCompute_UpdateDeployConfig(t *testing.T) {
	desired := []config.ServiceConfig{
		{
			Name:  "web",
			Image: "node:20",
			Deploy: config.DeployConfig{
				Replicas:      3,
				RestartPolicy: "ALWAYS",
				MaxRetries:    5,
			},
		},
	}
	live := []LiveService{
		{
			Name:  "web",
			Image: "node:20",
			Deploy: LiveDeployConfig{
				Replicas:      1,
				RestartPolicy: "ON_FAILURE",
				MaxRetries:    3,
			},
		},
	}

	cs := Compute(desired, live, false)

	if len(cs.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(cs.Changes))
	}

	rc := cs.Changes[0]
	if rc.Type != ChangeUpdate {
		t.Errorf("expected ChangeUpdate, got %d", rc.Type)
	}

	fieldMap := make(map[string]FieldDiff)
	for _, f := range rc.Fields {
		fieldMap[f.Path] = f
	}

	if f, ok := fieldMap["deploy.replicas"]; ok {
		if f.Current != "1" || f.Desired != "3" {
			t.Errorf("deploy.replicas: expected 1 → 3, got %s → %s", f.Current, f.Desired)
		}
	} else {
		t.Error("expected deploy.replicas field diff not found")
	}

	if f, ok := fieldMap["deploy.restartPolicy"]; ok {
		if f.Current != "ON_FAILURE" || f.Desired != "ALWAYS" {
			t.Errorf("deploy.restartPolicy: expected ON_FAILURE → ALWAYS, got %s → %s", f.Current, f.Desired)
		}
	} else {
		t.Error("expected deploy.restartPolicy field diff not found")
	}

	if f, ok := fieldMap["deploy.maxRetries"]; ok {
		if f.Current != "3" || f.Desired != "5" {
			t.Errorf("deploy.maxRetries: expected 3 → 5, got %s → %s", f.Current, f.Desired)
		}
	} else {
		t.Error("expected deploy.maxRetries field diff not found")
	}
}

func TestCompute_UpdateVariables(t *testing.T) {
	desired := []config.ServiceConfig{
		{
			Name:  "web",
			Image: "node:20",
			Variables: map[string]string{
				"PORT":    "8080",  // changed
				"NEW_VAR": "hello", // added
				// OLD_VAR is removed (present in live, not in desired)
			},
		},
	}
	live := []LiveService{
		{
			Name:  "web",
			Image: "node:20",
			Variables: map[string]string{
				"PORT":    "3000",
				"OLD_VAR": "goodbye",
			},
		},
	}

	cs := Compute(desired, live, false)

	if len(cs.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(cs.Changes))
	}

	fieldMap := make(map[string]FieldDiff)
	for _, f := range cs.Changes[0].Fields {
		fieldMap[f.Path] = f
	}

	// Changed.
	if f, ok := fieldMap["variables.PORT"]; ok {
		if f.Current != "3000" || f.Desired != "8080" {
			t.Errorf("variables.PORT: expected 3000 → 8080, got %s → %s", f.Current, f.Desired)
		}
	} else {
		t.Error("expected variables.PORT field diff not found")
	}

	// Added.
	if f, ok := fieldMap["variables.NEW_VAR"]; ok {
		if f.Current != "" || f.Desired != "hello" {
			t.Errorf("variables.NEW_VAR: expected '' → hello, got %q → %q", f.Current, f.Desired)
		}
	} else {
		t.Error("expected variables.NEW_VAR field diff not found")
	}

	// Removed.
	if f, ok := fieldMap["variables.OLD_VAR"]; ok {
		if f.Current != "goodbye" || f.Desired != "" {
			t.Errorf("variables.OLD_VAR: expected goodbye → '', got %q → %q", f.Current, f.Desired)
		}
	} else {
		t.Error("expected variables.OLD_VAR field diff not found")
	}
}

func TestCompute_VariablesSkipRailway(t *testing.T) {
	desired := []config.ServiceConfig{
		{
			Name:      "web",
			Image:     "node:20",
			Variables: map[string]string{"PORT": "3000"},
		},
	}
	live := []LiveService{
		{
			Name:  "web",
			Image: "node:20",
			Variables: map[string]string{
				"PORT":                "3000",
				"RAILWAY_ENVIRONMENT": "production",
				"RAILWAY_SERVICE_ID":  "svc-123",
			},
		},
	}

	cs := Compute(desired, live, false)

	// No changes expected — RAILWAY_ vars should be ignored.
	if cs.HasChanges() {
		for _, rc := range cs.Changes {
			for _, f := range rc.Fields {
				t.Errorf("unexpected field diff: %s (current=%q, desired=%q)", f.Path, f.Current, f.Desired)
			}
		}
	}
}

func TestCompute_UpdateVolume(t *testing.T) {
	desired := []config.ServiceConfig{
		{
			Name:   "web",
			Image:  "node:20",
			Volume: config.VolumeConfig{MountPath: "/data/new"},
		},
	}
	live := []LiveService{
		{
			Name:    "web",
			Image:   "node:20",
			Volumes: []LiveVolume{{MountPath: "/data/old"}},
		},
	}

	cs := Compute(desired, live, false)

	if len(cs.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(cs.Changes))
	}

	found := false
	for _, f := range cs.Changes[0].Fields {
		if f.Path == "volume.mountPath" {
			found = true
			if f.Current != "/data/old" || f.Desired != "/data/new" {
				t.Errorf("volume.mountPath: expected /data/old → /data/new, got %s → %s", f.Current, f.Desired)
			}
		}
	}
	if !found {
		t.Error("expected volume.mountPath field diff not found")
	}
}

func TestCompute_UpdateDomain(t *testing.T) {
	desired := []config.ServiceConfig{
		{
			Name:  "web",
			Image: "node:20",
			Networking: config.NetworkingConfig{
				Domain: config.DomainConfig{Port: 8080},
			},
		},
	}
	live := []LiveService{
		{
			Name:    "web",
			Image:   "node:20",
			Domains: []LiveDomain{{Domain: "web.up.railway.app", Port: 3000}},
		},
	}

	cs := Compute(desired, live, false)

	if len(cs.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(cs.Changes))
	}

	found := false
	for _, f := range cs.Changes[0].Fields {
		if f.Path == "networking.domain.port" {
			found = true
			if f.Current != "3000" || f.Desired != "8080" {
				t.Errorf("networking.domain.port: expected 3000 → 8080, got %s → %s", f.Current, f.Desired)
			}
		}
	}
	if !found {
		t.Error("expected networking.domain.port field diff not found")
	}
}

func TestCompute_UpdateTCPProxy(t *testing.T) {
	desired := []config.ServiceConfig{
		{
			Name:  "db",
			Image: "postgres:16",
			Networking: config.NetworkingConfig{
				TCPProxy: config.TCPProxyConfig{Port: 5432},
			},
		},
	}
	live := []LiveService{
		{
			Name:       "db",
			Image:      "postgres:16",
			TCPProxies: []LiveTCPProxy{{ApplicationPort: 3306, ProxyPort: 12345, Domain: "db.proxy.rlwy.net"}},
		},
	}

	cs := Compute(desired, live, false)

	if len(cs.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(cs.Changes))
	}

	found := false
	for _, f := range cs.Changes[0].Fields {
		if f.Path == "networking.tcpProxy.port" {
			found = true
			if f.Current != "3306" || f.Desired != "5432" {
				t.Errorf("networking.tcpProxy.port: expected 3306 → 5432, got %s → %s", f.Current, f.Desired)
			}
		}
	}
	if !found {
		t.Error("expected networking.tcpProxy.port field diff not found")
	}
}

func TestCompute_DeletePrune(t *testing.T) {
	desired := []config.ServiceConfig{
		{Name: "web", Image: "node:20"},
	}
	live := []LiveService{
		{Name: "web", Image: "node:20"},
		{Name: "old-service", Image: "nginx:1.25", Variables: map[string]string{"PORT": "8080"}},
	}

	cs := Compute(desired, live, true)

	if len(cs.Changes) != 1 {
		t.Fatalf("expected 1 change (delete), got %d", len(cs.Changes))
	}

	rc := cs.Changes[0]
	if rc.Type != ChangeDelete {
		t.Errorf("expected ChangeDelete, got %d", rc.Type)
	}
	if rc.ServiceName != "old-service" {
		t.Errorf("expected service name 'old-service', got %q", rc.ServiceName)
	}
}

func TestCompute_NoPrune(t *testing.T) {
	desired := []config.ServiceConfig{
		{Name: "web", Image: "node:20"},
	}
	live := []LiveService{
		{Name: "web", Image: "node:20"},
		{Name: "old-service", Image: "nginx:1.25"},
	}

	cs := Compute(desired, live, false)

	if cs.HasChanges() {
		t.Errorf("expected no changes with prune=false, got %d changes", len(cs.Changes))
	}
}

func TestCompute_Mixed(t *testing.T) {
	desired := []config.ServiceConfig{
		{Name: "new-service", Image: "redis:7"}, // create
		{Name: "web", Image: "node:20-alpine"},  // update (image change)
		// old-service not in desired — should be deleted with prune=true
	}
	live := []LiveService{
		{Name: "web", Image: "node:18-alpine"},
		{Name: "old-service", Image: "nginx:1.25"},
	}

	cs := Compute(desired, live, true)

	if len(cs.Changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(cs.Changes))
	}

	typeMap := make(map[string]ChangeType)
	for _, rc := range cs.Changes {
		typeMap[rc.ServiceName] = rc.Type
	}

	if typeMap["new-service"] != ChangeCreate {
		t.Errorf("expected ChangeCreate for new-service, got %d", typeMap["new-service"])
	}
	if typeMap["web"] != ChangeUpdate {
		t.Errorf("expected ChangeUpdate for web, got %d", typeMap["web"])
	}
	if typeMap["old-service"] != ChangeDelete {
		t.Errorf("expected ChangeDelete for old-service, got %d", typeMap["old-service"])
	}
}

func TestCompute_SensitiveVarMasked(t *testing.T) {
	desired := []config.ServiceConfig{
		{
			Name:      "web",
			Image:     "node:20",
			Variables: map[string]string{"API_KEY": "new-secret-key-value"},
		},
	}
	live := []LiveService{
		{
			Name:      "web",
			Image:     "node:20",
			Variables: map[string]string{"API_KEY": "old-secret-key-value"},
		},
	}

	cs := Compute(desired, live, false)

	if len(cs.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(cs.Changes))
	}

	for _, f := range cs.Changes[0].Fields {
		if f.Path == "variables.API_KEY" {
			// Values should be masked — not contain the raw secret.
			if f.Current == "old-secret-key-value" {
				t.Error("current value should be masked for sensitive key API_KEY")
			}
			if f.Desired == "new-secret-key-value" {
				t.Error("desired value should be masked for sensitive key API_KEY")
			}
			// Should contain asterisks.
			if len(f.Current) == 0 || len(f.Desired) == 0 {
				t.Error("masked values should not be empty")
			}
			return
		}
	}
	t.Error("expected variables.API_KEY field diff not found")
}

func TestChangeSet_HasChanges(t *testing.T) {
	tests := []struct {
		name     string
		cs       ChangeSet
		expected bool
	}{
		{
			name:     "empty changeset",
			cs:       ChangeSet{},
			expected: false,
		},
		{
			name: "only ChangeNone",
			cs: ChangeSet{
				Changes: []ResourceChange{{Type: ChangeNone, ServiceName: "web"}},
			},
			expected: false,
		},
		{
			name: "has create",
			cs: ChangeSet{
				Changes: []ResourceChange{{Type: ChangeCreate, ServiceName: "web"}},
			},
			expected: true,
		},
		{
			name: "has update",
			cs: ChangeSet{
				Changes: []ResourceChange{{Type: ChangeUpdate, ServiceName: "web"}},
			},
			expected: true,
		},
		{
			name: "has delete",
			cs: ChangeSet{
				Changes: []ResourceChange{{Type: ChangeDelete, ServiceName: "web"}},
			},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.cs.HasChanges() != tc.expected {
				t.Errorf("HasChanges() = %v, expected %v", tc.cs.HasChanges(), tc.expected)
			}
		})
	}
}

func TestChangeSet_Summary(t *testing.T) {
	cs := ChangeSet{
		Changes: []ResourceChange{
			{Type: ChangeCreate, ServiceName: "new1"},
			{Type: ChangeCreate, ServiceName: "new2"},
			{Type: ChangeUpdate, ServiceName: "updated"},
			{Type: ChangeDelete, ServiceName: "deleted"},
		},
	}

	expected := "2 to create, 1 to update, 1 to delete"
	if cs.Summary() != expected {
		t.Errorf("Summary() = %q, expected %q", cs.Summary(), expected)
	}

	// Empty changeset.
	empty := ChangeSet{}
	expectedEmpty := "0 to create, 0 to update, 0 to delete"
	if empty.Summary() != expectedEmpty {
		t.Errorf("Summary() = %q, expected %q", empty.Summary(), expectedEmpty)
	}
}

func TestCompute_CreateShowsRegistryMaskedPassword(t *testing.T) {
	desired := []config.ServiceConfig{
		{
			Name:  "api",
			Image: "ghcr.io/acme/api:v1",
			Registry: config.RegistryConfig{
				Username: "acme-bot",
				Password: "ghp_supersecrettoken",
			},
		},
	}

	cs := Compute(desired, nil, false)
	if len(cs.Changes) != 1 || cs.Changes[0].Type != ChangeCreate {
		t.Fatalf("expected 1 create change, got %+v", cs.Changes)
	}

	var sawUser, sawPass bool
	for _, f := range cs.Changes[0].Fields {
		switch f.Path {
		case "registry.username":
			sawUser = true
			if f.Desired != "acme-bot" {
				t.Errorf("registry.username should be shown in clear, got %q", f.Desired)
			}
		case "registry.password":
			sawPass = true
			if f.Desired == "ghp_supersecrettoken" {
				t.Error("registry.password must be masked, not shown in clear")
			}
			if f.Desired == "" {
				t.Error("registry.password should be present (masked), not empty")
			}
		}
	}
	if !sawUser {
		t.Error("expected registry.username field in create diff")
	}
	if !sawPass {
		t.Error("expected registry.password field in create diff")
	}
}

func TestCompute_NoRegistryNoRegistryFields(t *testing.T) {
	desired := []config.ServiceConfig{
		{Name: "pg", Image: "postgres:16"}, // public image, no registry block
	}
	cs := Compute(desired, nil, false)
	if len(cs.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(cs.Changes))
	}
	for _, f := range cs.Changes[0].Fields {
		if f.Path == "registry.username" || f.Path == "registry.password" {
			t.Errorf("did not expect registry field %q for a service without a registry block", f.Path)
		}
	}
}

func TestCompute_CreatePartialRegistryOmitted(t *testing.T) {
	// Only a username (no password) — apply would send no creds, so the diff
	// must not show a partial, misleading credential.
	desired := []config.ServiceConfig{
		{
			Name:     "api",
			Image:    "ghcr.io/acme/api:v1",
			Registry: config.RegistryConfig{Username: "acme-bot"},
		},
	}
	cs := Compute(desired, nil, false)
	if len(cs.Changes) != 1 {
		t.Fatalf("expected 1 change, got %d", len(cs.Changes))
	}
	for _, f := range cs.Changes[0].Fields {
		if f.Path == "registry.username" || f.Path == "registry.password" {
			t.Errorf("did not expect registry field %q when creds are incomplete", f.Path)
		}
	}
}

func TestCompute_UpdateShowsRegistryWhenOtherChanges(t *testing.T) {
	desired := []config.ServiceConfig{
		{
			Name:     "api",
			Image:    "ghcr.io/acme/api:v2", // changed → triggers an update
			Registry: config.RegistryConfig{Username: "acme-bot", Password: "ghp_token"},
		},
	}
	live := []LiveService{{Name: "api", Image: "ghcr.io/acme/api:v1"}}

	cs := Compute(desired, live, false)
	if len(cs.Changes) != 1 || cs.Changes[0].Type != ChangeUpdate {
		t.Fatalf("expected 1 update change, got %+v", cs.Changes)
	}

	var sawUser, sawPass bool
	for _, f := range cs.Changes[0].Fields {
		switch f.Path {
		case "registry.username":
			sawUser = true
		case "registry.password":
			sawPass = true
			if f.Desired == "ghp_token" {
				t.Error("registry.password must be masked in the update diff")
			}
		}
	}
	if !sawUser || !sawPass {
		t.Error("expected registry.username and registry.password in the update diff")
	}
}

func TestCompute_UpdateNoOtherChangesOmitsRegistry(t *testing.T) {
	// Fully converged except (unknowable) creds — no other field differs, so the
	// service must NOT show as an update just for registry (avoids spurious redeploy).
	svc := config.ServiceConfig{
		Name:     "api",
		Image:    "ghcr.io/acme/api:v1",
		Registry: config.RegistryConfig{Username: "acme-bot", Password: "ghp_token"},
	}
	live := []LiveService{{Name: "api", Image: "ghcr.io/acme/api:v1"}}

	cs := Compute([]config.ServiceConfig{svc}, live, false)
	if len(cs.Changes) != 0 {
		t.Errorf("expected no changes for a converged service, got %+v", cs.Changes)
	}
}

func TestCompute_DeployConfigConvergesWhenLiveMatches(t *testing.T) {
	// Live now carries deploy config (restartPolicy/maxRetries/etc.), so a service
	// whose live deploy config already matches desired shows no deploy diff.
	desired := []config.ServiceConfig{
		{
			Name:  "api",
			Image: "ghcr.io/acme/api:v1",
			Deploy: config.DeployConfig{
				RestartPolicy: "ON_FAILURE",
				MaxRetries:    10,
			},
		},
	}
	live := []LiveService{
		{
			Name:  "api",
			Image: "ghcr.io/acme/api:v1",
			Deploy: LiveDeployConfig{
				RestartPolicy: "ON_FAILURE",
				MaxRetries:    10,
			},
		},
	}

	cs := Compute(desired, live, false)
	if len(cs.Changes) != 0 {
		t.Errorf("expected no changes when live deploy config matches, got %+v", cs.Changes)
	}
}

func TestCompute_DeployConfigUndeclaredFieldsUnmanaged(t *testing.T) {
	// A config with no deploy fields must not diff against Railway's defaults —
	// otherwise it perma-diffs and apply overwrites them with zeros (e.g. 0 replicas).
	desired := []config.ServiceConfig{
		{Name: "api", Image: "ghcr.io/acme/api:v1"},
	}
	live := []LiveService{
		{
			Name:  "api",
			Image: "ghcr.io/acme/api:v1",
			Deploy: LiveDeployConfig{
				Replicas:      1,
				RestartPolicy: "ON_FAILURE",
				MaxRetries:    10,
			},
		},
	}

	cs := Compute(desired, live, false)
	if len(cs.Changes) != 0 {
		t.Errorf("expected no changes for a config with no deploy block, got %+v", cs.Changes)
	}
}
