package apply

import (
	"testing"

	"github.com/kubenoops/railctl/internal/api"
)

// committedConfig echoes an environment config where svc-1 runs nginx:1.25,
// is placed in us-west2, and has vol-1 mounted at /data.
func committedConfig() string {
	return `{"services":{"svc-1":{"source":{"image":"nginx:1.25"},"deploy":{"multiRegionConfig":{"us-west2":{"numReplicas":1}}},"volumeMounts":{"vol-1":{"mountPath":"/data"}}}}}`
}

func committedVars() map[string]string {
	return map[string]string{"POSTGRES_PASSWORD": "secret", "PGDATA": "/var/lib/postgresql/data/pgdata"}
}

func committedMock(t *testing.T) *api.MockClient {
	t.Helper()
	return &api.MockClient{
		GetEnvironmentConfigFunc: func(string) (string, error) {
			return committedConfig(), nil
		},
		GetVariablesFunc: func(_, _, _ string) (map[string]string, error) {
			return committedVars(), nil
		},
	}
}

func TestAwaitConfigCommitted(t *testing.T) {
	base := ConfigExpectations{
		ServiceID: "svc-1",
		RegionID:  "us-west2",
		VolumeID:  "vol-1",
		MountPath: "/data",
		Variables: committedVars(),
	}

	if !AwaitConfigCommitted(committedMock(t), "proj-1", "env-1", base) {
		t.Fatal("fully-staged expectations must confirm immediately")
	}

	cases := []struct {
		name string
		mut  func(*ConfigExpectations)
	}{
		{"region key absent", func(e *ConfigExpectations) { e.RegionID = "europe-west4-drams3a" }},
		{"source image mismatch", func(e *ConfigExpectations) { e.SourceImage = "redis:7-alpine" }},
		{"volume id absent", func(e *ConfigExpectations) { e.VolumeID = "vol-other" }},
		{"mount path mismatch", func(e *ConfigExpectations) { e.MountPath = "/elsewhere" }},
		{"variable value mismatch", func(e *ConfigExpectations) { e.Variables["PGDATA"] = "/other" }},
		{"variable missing", func(e *ConfigExpectations) { e.Variables["NEW_KEY"] = "v" }},
		{"removed variable still present", func(e *ConfigExpectations) { e.RemovedVariables = []string{"PGDATA"} }},
	}
	for _, c := range cases {
		exp := base
		c.mut(&exp)
		if AwaitConfigCommitted(committedMock(t), "proj-1", "env-1", exp) {
			t.Errorf("%s: must not confirm", c.name)
		}
	}

	// Read errors are not confirmation either.
	failing := &api.MockClient{
		GetEnvironmentConfigFunc: func(string) (string, error) { return "", errBoom },
	}
	if AwaitConfigCommitted(failing, "proj-1", "env-1", base) {
		t.Error("config read error must not confirm")
	}

	// Empty expectations need no reads at all — nothing staged, nothing to
	// wait for.
	if !AwaitConfigCommitted(&api.MockClient{}, "proj-1", "env-1", ConfigExpectations{ServiceID: "svc-1"}) {
		t.Error("unmanaged expectations must confirm without reads")
	}
}

// errBoom is a stub error for read-failure cases.
var errBoom = &stubError{}

type stubError struct{}

func (e *stubError) Error() string { return "boom" }
