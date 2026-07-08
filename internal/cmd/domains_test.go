package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// saveDomainGlobals snapshots the package globals the domain commands touch
// and restores them after the test.
func saveDomainGlobals(t *testing.T) {
	t.Helper()
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origService := service
	origToken := token
	origFormat := outputFormat
	origCreatePort := createDomainPort
	origDeleteYes := deleteDomainYes
	t.Cleanup(func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		service = origService
		token = origToken
		outputFormat = origFormat
		createDomainPort = origCreatePort
		deleteDomainYes = origDeleteYes
	})
}

// domainsMock returns a workspace-token-shaped mock with one railway and one
// custom domain on service "api".
func domainsMock() *api.MockClient {
	port8080 := 8080
	port3000 := 3000
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
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{
				ServiceDomains: []api.ServiceDomain{
					{ID: "dom-1", Domain: "myapp.up.railway.app", TargetPort: &port8080},
				},
				CustomDomains: []api.CustomDomain{
					{ID: "cd-1", Domain: "app.example.com", TargetPort: &port3000,
						Status: &api.CustomDomainStatus{Verified: false}},
				},
			}, nil
		},
	}
}

// captureStdout runs fn while capturing everything written to os.Stdout
// (the table/JSON printers write there directly).
func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	runErr := fn()
	w.Close()
	os.Stdout = oldStdout
	out, _ := io.ReadAll(r)
	return string(out), runErr
}

func TestRunGetDomains_Table(t *testing.T) {
	saveDomainGlobals(t)

	token = "test-token"
	project = "my-project"
	environment = "production"
	service = "api"
	outputFormat = "table"
	newAPIClient = func(tkn string) api.APIClient { return domainsMock() }

	out, err := captureStdout(t, func() error {
		return getDomainsCmd.RunE(getDomainsCmd, []string{})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{
		"TYPE", "DOMAIN", "PORT", "STATUS",
		"railway", "myapp.up.railway.app", "8080",
		"custom", "app.example.com", "3000", "pending",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in table output, got:\n%s", want, out)
		}
	}
}

func TestRunGetDomains_JSON(t *testing.T) {
	saveDomainGlobals(t)

	token = "test-token"
	project = "my-project"
	environment = "production"
	service = "api"
	outputFormat = "json"
	newAPIClient = func(tkn string) api.APIClient { return domainsMock() }

	out, err := captureStdout(t, func() error {
		return getDomainsCmd.RunE(getDomainsCmd, []string{})
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rows []domainRow
	if jsonErr := json.Unmarshal([]byte(out), &rows); jsonErr != nil {
		t.Fatalf("-o json output is not valid JSON: %v\noutput: %s", jsonErr, out)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Type != "railway" || rows[0].Domain != "myapp.up.railway.app" {
		t.Errorf("row[0] = %+v, want railway myapp.up.railway.app", rows[0])
	}
	if rows[1].Type != "custom" || rows[1].Status != "pending" || rows[1].ID != "cd-1" {
		t.Errorf("row[1] = %+v, want custom/pending/cd-1", rows[1])
	}
}

func TestRunCreateDomain_Success(t *testing.T) {
	saveDomainGlobals(t)

	var gotProject, gotEnv, gotSvc, gotDomain string
	var gotPort int
	mock := domainsMock()
	mock.CreateCustomDomainFunc = func(projectID, environmentID, serviceID, domain string, targetPort int) (api.CustomDomain, error) {
		gotProject, gotEnv, gotSvc, gotDomain, gotPort = projectID, environmentID, serviceID, domain, targetPort
		return api.CustomDomain{
			ID: "cd-new", Domain: domain,
			Status: &api.CustomDomainStatus{
				Verified:            false,
				VerificationDNSHost: "_railway-verify.app",
				VerificationToken:   "railway-verify=token123",
				DNSRecords: []api.DNSRecord{{
					RecordType: "DNS_RECORD_TYPE_CNAME", Purpose: "DNS_RECORD_PURPOSE_TRAFFIC_ROUTE",
					Hostlabel: "app", Fqdn: domain, RequiredValue: "abc.up.railway.app",
				}},
			},
		}, nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	service = "api"
	createDomainPort = 3000
	newAPIClient = func(tkn string) api.APIClient { return mock }

	var buf bytes.Buffer
	createDomainCmd.SetOut(&buf)
	defer createDomainCmd.SetOut(nil)

	if err := createDomainCmd.RunE(createDomainCmd, []string{"app.example.com"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotProject != "proj-1" || gotEnv != "env-1" || gotSvc != "svc-1" {
		t.Errorf("CreateCustomDomain called with %s/%s/%s, want proj-1/env-1/svc-1", gotProject, gotEnv, gotSvc)
	}
	if gotDomain != "app.example.com" || gotPort != 3000 {
		t.Errorf("CreateCustomDomain called with domain=%s port=%d, want app.example.com/3000", gotDomain, gotPort)
	}

	out := buf.String()
	for _, want := range []string{
		"Custom domain 'app.example.com' created",
		"DNS record",
		"CNAME",
		"abc.up.railway.app",
		"railway-verify=token123", // verification TXT
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in DNS output, got:\n%s", want, out)
		}
	}
}

func TestRunCreateDomain_InvalidName(t *testing.T) {
	saveDomainGlobals(t)

	mock := domainsMock()
	mock.CreateCustomDomainFunc = func(projectID, environmentID, serviceID, domain string, targetPort int) (api.CustomDomain, error) {
		t.Error("CreateCustomDomain must not be called for an invalid name")
		return api.CustomDomain{}, nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	service = "api"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	err := createDomainCmd.RunE(createDomainCmd, []string{"nodot"})
	if err == nil {
		t.Fatal("expected error for domain without a dot")
	}
	if !strings.Contains(err.Error(), "fully qualified") {
		t.Errorf("error should explain the expected shape: %v", err)
	}
}

func TestRunDeleteDomain_Success(t *testing.T) {
	saveDomainGlobals(t)

	var deletedID string
	mock := domainsMock()
	mock.DeleteCustomDomainFunc = func(id string) error {
		deletedID = id
		return nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	service = "api"
	deleteDomainYes = false
	newAPIClient = func(tkn string) api.APIClient { return mock }

	var buf bytes.Buffer
	deleteDomainCmd.SetOut(&buf)
	deleteDomainCmd.SetIn(strings.NewReader("y\n"))
	defer func() {
		deleteDomainCmd.SetOut(nil)
		deleteDomainCmd.SetIn(nil)
	}()

	if err := deleteDomainCmd.RunE(deleteDomainCmd, []string{"app.example.com"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if deletedID != "cd-1" {
		t.Errorf("DeleteCustomDomain called with id=%q, want cd-1", deletedID)
	}
	out := buf.String()
	if !strings.Contains(out, "[y/N]") {
		t.Errorf("expected confirmation prompt, got:\n%s", out)
	}
	if !strings.Contains(out, "Custom domain 'app.example.com' deleted.") {
		t.Errorf("expected deletion confirmation, got:\n%s", out)
	}
}

func TestRunDeleteDomain_NotFound(t *testing.T) {
	saveDomainGlobals(t)

	mock := domainsMock()
	mock.DeleteCustomDomainFunc = func(id string) error {
		t.Error("DeleteCustomDomain must not be called when the domain is not found")
		return nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	service = "api"
	deleteDomainYes = true
	newAPIClient = func(tkn string) api.APIClient { return mock }

	err := deleteDomainCmd.RunE(deleteDomainCmd, []string{"missing.example.com"})
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error should say not found: %v", err)
	}
	if !strings.Contains(err.Error(), "available: app.example.com") {
		t.Errorf("error should list available custom domains: %v", err)
	}
}

func TestRunDeleteDomain_Cancelled(t *testing.T) {
	saveDomainGlobals(t)

	mock := domainsMock()
	mock.DeleteCustomDomainFunc = func(id string) error {
		t.Error("DeleteCustomDomain must not be called after 'n'")
		return nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	service = "api"
	deleteDomainYes = false
	newAPIClient = func(tkn string) api.APIClient { return mock }

	var buf bytes.Buffer
	deleteDomainCmd.SetOut(&buf)
	deleteDomainCmd.SetIn(strings.NewReader("n\n"))
	defer func() {
		deleteDomainCmd.SetOut(nil)
		deleteDomainCmd.SetIn(nil)
	}()

	if err := deleteDomainCmd.RunE(deleteDomainCmd, []string{"app.example.com"}); err != nil {
		t.Fatalf("expected no error on declined confirmation, got: %v", err)
	}
	if !strings.Contains(buf.String(), "Deletion cancelled.") {
		t.Errorf("expected cancellation message, got:\n%s", buf.String())
	}
}

func TestRunDeleteDomain_YesBypassesPrompt(t *testing.T) {
	saveDomainGlobals(t)

	var deletedID string
	mock := domainsMock()
	mock.DeleteCustomDomainFunc = func(id string) error {
		deletedID = id
		return nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	service = "api"
	deleteDomainYes = true
	newAPIClient = func(tkn string) api.APIClient { return mock }

	var buf bytes.Buffer
	deleteDomainCmd.SetOut(&buf)
	// No stdin wired up: reading a confirmation would fail, proving --yes
	// never prompts.
	deleteDomainCmd.SetIn(strings.NewReader(""))
	defer func() {
		deleteDomainCmd.SetOut(nil)
		deleteDomainCmd.SetIn(nil)
	}()

	if err := deleteDomainCmd.RunE(deleteDomainCmd, []string{"app.example.com"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedID != "cd-1" {
		t.Errorf("DeleteCustomDomain called with id=%q, want cd-1", deletedID)
	}
	if strings.Contains(buf.String(), "[y/N]") {
		t.Errorf("--yes must not prompt, got:\n%s", buf.String())
	}
}

// TestRunGetDomains_ProjectTokenFlagFree exercises the flag-free path: with a
// project token, -p/-e come from the token's baked scope; only -s is needed.
func TestRunGetDomains_ProjectTokenFlagFree(t *testing.T) {
	saveDomainGlobals(t)

	mock := domainsMock()
	mock.IsProjectTokenFunc = func() (bool, error) { return true, nil }
	mock.GetProjectContextFunc = func() (string, string, error) { return "proj-1", "env-1", nil }
	mock.GetProjectFunc = func(id string) (types.Project, error) {
		return types.Project{ID: id, Name: "my-project"}, nil
	}
	mock.ListProjectsFunc = nil // project tokens cannot enumerate projects

	token = "test-token"
	project = ""
	environment = ""
	service = "api"
	outputFormat = "table"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	out, err := captureStdout(t, func() error {
		return getDomainsCmd.RunE(getDomainsCmd, []string{})
	})
	if err != nil {
		t.Fatalf("expected flag-free get domains to work under a project token, got: %v", err)
	}
	if !strings.Contains(out, "app.example.com") {
		t.Errorf("expected custom domain in output, got:\n%s", out)
	}
}
