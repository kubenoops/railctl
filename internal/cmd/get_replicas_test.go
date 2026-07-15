package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// replicasMock builds a MockClient that resolves my-project/production/api and
// returns the given ReplicaList from ListReplicas.
func replicasMock(list api.ReplicaList, listErr error) *api.MockClient {
	return &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
		ListServicesFunc: func(projectID, environmentID string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
		},
		ListReplicasFunc: func(environmentID, serviceID string) (api.ReplicaList, error) {
			return list, listErr
		},
	}
}

// saveReplicaGlobals snapshots the global flags get_replicas reads and restores
// them after the test.
func saveReplicaGlobals(t *testing.T) {
	t.Helper()
	origClient := newAPIClient
	origToken := token
	origProject := project
	origEnv := environment
	origService := service
	origFormat := outputFormat
	t.Cleanup(func() {
		newAPIClient = origClient
		token = origToken
		project = origProject
		environment = origEnv
		service = origService
		outputFormat = origFormat
	})
}

func setReplicaEnv(client api.APIClient, format string) {
	token = "test-token"
	project = "my-project"
	environment = "production"
	service = "api"
	outputFormat = format
	newAPIClient = func(tkn string) api.APIClient { return client }
}

func TestRunGetReplicas_Table_Many(t *testing.T) {
	saveReplicaGlobals(t)
	list := api.ReplicaList{
		DeploymentID:     "dep-1",
		DeploymentStatus: "SUCCESS",
		Replicas: []api.Replica{
			{ID: "inst-a", Status: "RUNNING"},
			{ID: "inst-b", Status: "RUNNING"},
			{ID: "inst-c", Status: "CRASHED"},
		},
	}
	setReplicaEnv(replicasMock(list, nil), "table")

	out, err := captureStdout(t, func() error {
		return getReplicasCmd.RunE(getReplicasCmd, []string{})
	})
	if err != nil {
		t.Fatalf("runGetReplicas error: %v", err)
	}
	// Header: deployment context + status summary (alphabetical: CRASHED then RUNNING).
	if !strings.Contains(out, "Deployment dep-1 (SUCCESS)") {
		t.Errorf("missing deployment header: %q", out)
	}
	if !strings.Contains(out, "3 replicas: 1 CRASHED, 2 RUNNING") {
		t.Errorf("missing/incorrect summary: %q", out)
	}
	// Every instance id and the column header appear.
	for _, want := range []string{"INSTANCE ID", "STATUS", "inst-a", "inst-b", "inst-c"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q in:\n%s", want, out)
		}
	}
}

func TestRunGetReplicas_Empty(t *testing.T) {
	saveReplicaGlobals(t)
	setReplicaEnv(replicasMock(api.ReplicaList{Replicas: []api.Replica{}}, nil), "table")

	out, err := captureStdout(t, func() error {
		return getReplicasCmd.RunE(getReplicasCmd, []string{})
	})
	if err != nil {
		t.Fatalf("runGetReplicas error: %v", err)
	}
	if !strings.Contains(out, "No replicas running") {
		t.Errorf("expected empty message, got: %q", out)
	}
}

func TestRunGetReplicas_Wide_HasDeploymentColumn(t *testing.T) {
	saveReplicaGlobals(t)
	list := api.ReplicaList{
		DeploymentID:     "dep-1",
		DeploymentStatus: "SUCCESS",
		Replicas:         []api.Replica{{ID: "inst-a", Status: "RUNNING"}},
	}
	setReplicaEnv(replicasMock(list, nil), "wide")

	out, err := captureStdout(t, func() error {
		return getReplicasCmd.RunE(getReplicasCmd, []string{})
	})
	if err != nil {
		t.Fatalf("runGetReplicas error: %v", err)
	}
	if !strings.Contains(out, "DEPLOYMENT ID") || !strings.Contains(out, "dep-1") {
		t.Errorf("wide table must include the deployment id column: %q", out)
	}
}

func TestRunGetReplicas_JSON(t *testing.T) {
	saveReplicaGlobals(t)
	list := api.ReplicaList{
		DeploymentID:     "dep-1",
		DeploymentStatus: "SUCCESS",
		Replicas:         []api.Replica{{ID: "inst-a", Status: "RUNNING"}},
	}
	setReplicaEnv(replicasMock(list, nil), "json")

	out, err := captureStdout(t, func() error {
		return getReplicasCmd.RunE(getReplicasCmd, []string{})
	})
	if err != nil {
		t.Fatalf("runGetReplicas error: %v", err)
	}
	// Structured output carries the full ReplicaList, not the human header.
	for _, want := range []string{`"deploymentId"`, `"dep-1"`, `"replicas"`, `"inst-a"`, `"RUNNING"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Deployment dep-1 (SUCCESS)") {
		t.Errorf("structured output must not include the human header: %q", out)
	}
}

func TestRunGetReplicas_APIError(t *testing.T) {
	saveReplicaGlobals(t)
	setReplicaEnv(replicasMock(api.ReplicaList{}, fmt.Errorf("boom")), "table")

	_, err := captureStdout(t, func() error {
		return getReplicasCmd.RunE(getReplicasCmd, []string{})
	})
	if err == nil {
		t.Fatal("expected an error when ListReplicas fails")
	}
	if !strings.Contains(err.Error(), "failed to list replicas") {
		t.Errorf("error should be wrapped: %v", err)
	}
}

func TestReplicaSummary(t *testing.T) {
	tests := []struct {
		name     string
		replicas []api.Replica
		want     string
	}{
		{
			name:     "single replica uses singular noun",
			replicas: []api.Replica{{ID: "a", Status: "RUNNING"}},
			want:     "1 replica: 1 RUNNING",
		},
		{
			name: "mixed statuses sorted alphabetically",
			replicas: []api.Replica{
				{ID: "a", Status: "RUNNING"},
				{ID: "b", Status: "CRASHED"},
				{ID: "c", Status: "RUNNING"},
			},
			want: "3 replicas: 1 CRASHED, 2 RUNNING",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replicaSummary(tt.replicas); got != tt.want {
				t.Errorf("replicaSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

// captureStderr mirrors captureStdout for the B-soft warning path, which writes
// to os.Stderr.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

func TestWarnIfUnknownReplica(t *testing.T) {
	known := api.ReplicaList{Replicas: []api.Replica{{ID: "inst-a", Status: "RUNNING"}}}

	t.Run("known replica is silent", func(t *testing.T) {
		client := &api.MockClient{ListReplicasFunc: func(e, s string) (api.ReplicaList, error) { return known, nil }}
		out := captureStderr(t, func() {
			warnIfUnknownReplica(client, "env-1", "svc-1", "inst-a")
		})
		if out != "" {
			t.Errorf("known replica must not warn, got: %q", out)
		}
	})

	t.Run("unknown replica warns", func(t *testing.T) {
		client := &api.MockClient{ListReplicasFunc: func(e, s string) (api.ReplicaList, error) { return known, nil }}
		out := captureStderr(t, func() {
			warnIfUnknownReplica(client, "env-1", "svc-1", "inst-bogus")
		})
		if !strings.Contains(out, "not among this service's current replicas") ||
			!strings.Contains(out, "get replicas") {
			t.Errorf("unknown replica should warn with a hint, got: %q", out)
		}
	})

	t.Run("discovery error fails open (silent)", func(t *testing.T) {
		client := &api.MockClient{ListReplicasFunc: func(e, s string) (api.ReplicaList, error) {
			return api.ReplicaList{}, fmt.Errorf("boom")
		}}
		out := captureStderr(t, func() {
			warnIfUnknownReplica(client, "env-1", "svc-1", "inst-a")
		})
		if out != "" {
			t.Errorf("a discovery error must not warn (fail open), got: %q", out)
		}
	})
}
