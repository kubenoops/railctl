package cmd

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/kubenoops/railctl/internal/types"
)

func TestRunGetServices_MissingProject(t *testing.T) {
	// Save and restore globals
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
	}()

	// Set up mock
	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{}
	}
	project = "" // No project set

	cmd := getServicesCmd
	err := cmd.RunE(cmd, []string{})

	if err == nil {
		t.Error("expected error for missing project")
	}
	if err != nil && err.Error() != "-p/--project is required. Use -p flag or set RAILCTL_PROJECT" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunGetServices_MissingEnvironment(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
		}
	}
	project = "my-project"
	environment = "" // No environment set

	cmd := getServicesCmd
	err := cmd.RunE(cmd, []string{})

	if err == nil {
		t.Error("expected error for missing environment")
	}
}

func TestServicesToOutput(t *testing.T) {
	servicePort := 8080
	customPort := 3000
	services := []types.ServiceDetail{
		{
			ID:         "svc-1",
			Name:       "api",
			Source:     "nginx:latest",
			SourceType: "image",
			UpdatedAt:  time.Now(),
			ServiceDomains: []types.ServiceDomain{
				{ID: "dom-1", Domain: "api.up.railway.app", TargetPort: &servicePort},
			},
			CustomDomains: []types.CustomDomain{
				{ID: "cdom-1", Domain: "api.example.com", TargetPort: &customPort},
			},
			TCPProxies: []types.TCPProxy{
				{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432},
			},
		},
		{
			ID:   "svc-2",
			Name: "worker",
		},
	}

	result := servicesToOutput(services)

	if len(result) != 2 {
		t.Fatalf("expected 2 services, got %d", len(result))
	}
	if result[0].Name != "api" {
		t.Errorf("expected name 'api', got %q", result[0].Name)
	}
	if result[0].Source != "nginx:latest" {
		t.Errorf("expected source 'nginx:latest', got %q", result[0].Source)
	}
	if len(result[0].ServiceDomains) != 1 || result[0].ServiceDomains[0].Domain != "api.up.railway.app" {
		t.Errorf("expected service domains in output, got %#v", result[0].ServiceDomains)
	}
	if len(result[0].CustomDomains) != 1 || result[0].CustomDomains[0].Domain != "api.example.com" {
		t.Errorf("expected custom domains in output, got %#v", result[0].CustomDomains)
	}
	if len(result[0].TCPProxies) != 1 || result[0].TCPProxies[0].ProxyPort != 44321 {
		t.Errorf("expected TCP proxies in output, got %#v", result[0].TCPProxies)
	}
	if result[1].UpdatedAt != "" {
		t.Errorf("expected empty updatedAt for svc-2, got %q", result[1].UpdatedAt)
	}
	if len(result[1].ServiceDomains) != 0 {
		t.Errorf("expected no service domains for svc-2, got %#v", result[1].ServiceDomains)
	}
	if len(result[1].CustomDomains) != 0 {
		t.Errorf("expected no custom domains for svc-2, got %#v", result[1].CustomDomains)
	}
	if len(result[1].TCPProxies) != 0 {
		t.Errorf("expected no TCP proxies for svc-2, got %#v", result[1].TCPProxies)
	}
}

func TestServicesToOutput_WithoutNetworkingFields(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}

	result := servicesToOutput(services)

	if len(result) != 1 {
		t.Fatalf("expected 1 service, got %d", len(result))
	}
	if len(result[0].ServiceDomains) != 0 {
		t.Errorf("expected no service domains, got %#v", result[0].ServiceDomains)
	}
	if len(result[0].CustomDomains) != 0 {
		t.Errorf("expected no custom domains, got %#v", result[0].CustomDomains)
	}
	if len(result[0].TCPProxies) != 0 {
		t.Errorf("expected no TCP proxies, got %#v", result[0].TCPProxies)
	}
}

func TestRunGetServices_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origOutputFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		outputFormat = origOutputFormat
	}()

	token = "test-token"
	outputFormat = "json"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api", Source: "nginx:latest"},
					{ID: "svc-2", Name: "web", Source: "node:20"},
				}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, nil
			},
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return nil, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := getServicesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunDescribeService_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api", Source: "nginx:latest", SourceType: "image"},
				}, nil
			},
			GetVariablesFunc: func(projectID, environmentID, serviceID string) (map[string]string, error) {
				return map[string]string{"PORT": "3000"}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, nil
			},
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return nil, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := describeServiceCmd
	err := cmd.RunE(cmd, []string{"api"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunDescribeService_MissingProject(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{}
	}
	project = "" // No project set

	cmd := describeServiceCmd
	err := cmd.RunE(cmd, []string{"api"})

	if err == nil {
		t.Error("expected error for missing project")
	}
}

func TestRunDescribeService_ServiceNotFound(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{}, nil // Empty
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := describeServiceCmd
	err := cmd.RunE(cmd, []string{"nonexistent"})

	if err == nil {
		t.Error("expected error for missing service")
	}
}

func TestServicesToTable(t *testing.T) {
	services := []types.ServiceDetail{
		{
			ID:        "svc-1",
			Name:      "api",
			Source:    "nginx:latest",
			UpdatedAt: time.Now(),
		},
	}

	table := servicesToTable(services)

	if table.RowCount() != 1 {
		t.Errorf("expected 1 row, got %d", table.RowCount())
	}
}

func TestServicesToWideTable(t *testing.T) {
	services := []types.ServiceDetail{
		{
			ID:         "svc-123456789012345",
			Name:       "api",
			Source:     "nginx:latest",
			SourceType: "image",
			UpdatedAt:  time.Now(),
			ServiceDomains: []types.ServiceDomain{
				{ID: "dom-1", Domain: "api.up.railway.app"},
			},
			TCPProxies: []types.TCPProxy{
				{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432},
			},
		},
		{
			ID:         "svc-2",
			Name:       "web",
			Source:     "github.com/org/repo",
			SourceType: "github",
			CustomDomains: []types.CustomDomain{
				{ID: "cdom-1", Domain: "web.example.com"},
			},
		},
	}

	table := servicesToWideTable(services)
	if table.RowCount() != 2 {
		t.Fatalf("expected 2 rows, got %d", table.RowCount())
	}

	rendered := table.Render()
	for _, want := range []string{"DOMAIN", "TCP", "api.up.railway.app", "roundhouse.proxy.rlwy.net:44321", "web.example.com", "svc-12345678"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("expected wide table to contain %q, got:\n%s", want, rendered)
		}
	}
}

func TestServicesToWideTable_WithoutNetworkingFields(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api", Source: "nginx:latest", SourceType: "image"}}

	rendered := servicesToWideTable(services).Render()

	if !strings.Contains(rendered, "DOMAIN") || !strings.Contains(rendered, "TCP") {
		t.Fatalf("expected wide table headers for networking columns, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "-") {
		t.Errorf("expected placeholder values for missing networking fields, got:\n%s", rendered)
	}
}

func TestServicesToTable_UnchangedWithoutNetworkingColumns(t *testing.T) {
	services := []types.ServiceDetail{{
		ID:             "svc-1",
		Name:           "api",
		Source:         "nginx:latest",
		ServiceDomains: []types.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}},
		TCPProxies:     []types.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}},
	}}

	rendered := servicesToTable(services).Render()

	if strings.Contains(rendered, "DOMAIN") || strings.Contains(rendered, "TCP") {
		t.Errorf("expected default table to remain unchanged, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "api.up.railway.app") || strings.Contains(rendered, "roundhouse.proxy.rlwy.net:44321") {
		t.Errorf("expected default table to exclude networking summaries, got:\n%s", rendered)
	}
}

func TestEnrichServicesForRichOutput_DefaultTableSkipsNetworkingLookups(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	listDomainsCalled := false
	listTCPProxiesCalled := false
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			listDomainsCalled = true
			return api.DomainList{}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			listTCPProxiesCalled = true
			return nil, nil
		},
	}

	enrichServicesForRichOutput(client, output.FormatTable, "proj-1", "env-1", services)

	if listDomainsCalled {
		t.Error("expected default table output to skip domain enrichment")
	}
	if listTCPProxiesCalled {
		t.Error("expected default table output to skip TCP proxy enrichment")
	}
}

func TestEnrichServicesForRichOutput_NonFatalLookupFailures(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}, {ID: "svc-2", Name: "worker"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			if serviceID == "svc-1" {
				return api.DomainList{}, fmt.Errorf("domains unavailable")
			}
			return api.DomainList{ServiceDomains: []api.ServiceDomain{{ID: "dom-2", Domain: "worker.up.railway.app"}}}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			if serviceID == "svc-1" {
				return nil, fmt.Errorf("tcp unavailable")
			}
			return []api.TCPProxy{{ID: "tcp-2", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 45678, ApplicationPort: 8080}}, nil
		},
	}

	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)

	if len(services[0].ServiceDomains) != 0 || len(services[0].CustomDomains) != 0 || len(services[0].TCPProxies) != 0 {
		t.Errorf("expected failed lookups to leave svc-1 networking fields empty, got %#v", services[0])
	}
	if len(services[1].ServiceDomains) != 1 || services[1].ServiceDomains[0].Domain != "worker.up.railway.app" {
		t.Errorf("expected svc-2 service domains to be enriched, got %#v", services[1].ServiceDomains)
	}
	if len(services[1].TCPProxies) != 1 || services[1].TCPProxies[0].ProxyPort != 45678 {
		t.Errorf("expected svc-2 TCP proxies to be enriched, got %#v", services[1].TCPProxies)
	}
}

func TestRunGetServices_NetworkingFetchFailureNonFatal(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origOutputFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		outputFormat = origOutputFormat
	}()

	token = "test-token"
	outputFormat = "json"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, fmt.Errorf("domains unavailable")
			},
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return nil, fmt.Errorf("tcp unavailable")
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := getServicesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Errorf("expected non-fatal networking errors, got %v", err)
	}
}

func TestRunGetServices_DefaultTableDoesNotRequireNetworkingCalls(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origOutputFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		outputFormat = origOutputFormat
	}()

	listDomainsCalled := false
	listTCPProxiesCalled := false
	token = "test-token"
	outputFormat = "table"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "api", Source: "nginx:latest"}}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				listDomainsCalled = true
				return api.DomainList{}, nil
			},
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				listTCPProxiesCalled = true
				return nil, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := getServicesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listDomainsCalled {
		t.Error("expected default table path not to call ListDomains")
	}
	if listTCPProxiesCalled {
		t.Error("expected default table path not to call ListTCPProxies")
	}
}

func TestSummarizeServiceDomain(t *testing.T) {
	t.Run("prefers service domain", func(t *testing.T) {
		svc := types.ServiceDetail{
			ServiceDomains: []types.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}},
			CustomDomains:  []types.CustomDomain{{ID: "cdom-1", Domain: "api.example.com"}},
		}
		if got := summarizeServiceDomain(svc); got != "api.up.railway.app" {
			t.Errorf("expected service domain summary, got %q", got)
		}
	})

	t.Run("falls back to custom domain", func(t *testing.T) {
		svc := types.ServiceDetail{CustomDomains: []types.CustomDomain{{ID: "cdom-1", Domain: "api.example.com"}}}
		if got := summarizeServiceDomain(svc); got != "api.example.com" {
			t.Errorf("expected custom domain summary, got %q", got)
		}
	})

	t.Run("returns placeholder when empty", func(t *testing.T) {
		if got := summarizeServiceDomain(types.ServiceDetail{}); got != "-" {
			t.Errorf("expected placeholder, got %q", got)
		}
	})
}

func TestSummarizeTCPProxy(t *testing.T) {
	t.Run("returns host and port", func(t *testing.T) {
		svc := types.ServiceDetail{TCPProxies: []types.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321}}}
		if got := summarizeTCPProxy(svc); got != "roundhouse.proxy.rlwy.net:44321" {
			t.Errorf("expected tcp summary, got %q", got)
		}
	})

	t.Run("returns placeholder when empty", func(t *testing.T) {
		if got := summarizeTCPProxy(types.ServiceDetail{}); got != "-" {
			t.Errorf("expected placeholder, got %q", got)
		}
	})
}

func TestMapServiceDomains(t *testing.T) {
	port := 8080
	mapped := mapServiceDomains([]api.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app", TargetPort: &port}})
	if len(mapped) != 1 || mapped[0].Domain != "api.up.railway.app" || mapped[0].TargetPort == nil || *mapped[0].TargetPort != 8080 {
		t.Errorf("unexpected mapped service domains: %#v", mapped)
	}
}

func TestMapCustomDomains(t *testing.T) {
	port := 3000
	mapped := mapCustomDomains([]api.CustomDomain{{ID: "cdom-1", Domain: "api.example.com", TargetPort: &port}})
	if len(mapped) != 1 || mapped[0].Domain != "api.example.com" || mapped[0].TargetPort == nil || *mapped[0].TargetPort != 3000 {
		t.Errorf("unexpected mapped custom domains: %#v", mapped)
	}
}

func TestMapTCPProxies(t *testing.T) {
	mapped := mapTCPProxies([]api.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}})
	if len(mapped) != 1 || mapped[0].Domain != "roundhouse.proxy.rlwy.net" || mapped[0].ProxyPort != 44321 || mapped[0].ApplicationPort != 5432 {
		t.Errorf("unexpected mapped TCP proxies: %#v", mapped)
	}
}

func TestMapNetworkingHelpers_EmptyInputs(t *testing.T) {
	if mapped := mapServiceDomains(nil); mapped != nil {
		t.Errorf("expected nil service domains, got %#v", mapped)
	}
	if mapped := mapCustomDomains(nil); mapped != nil {
		t.Errorf("expected nil custom domains, got %#v", mapped)
	}
	if mapped := mapTCPProxies(nil); mapped != nil {
		t.Errorf("expected nil tcp proxies, got %#v", mapped)
	}
}

func TestRunGetServices_WideOutputEnrichesNetworkingData(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origOutputFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		outputFormat = origOutputFormat
	}()

	listDomainsCalls := 0
	listTCPProxiesCalls := 0
	token = "test-token"
	outputFormat = "wide"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "api"}, {ID: "svc-2", Name: "worker"}}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				listDomainsCalls++
				return api.DomainList{}, nil
			},
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				listTCPProxiesCalls++
				return nil, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := getServicesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listDomainsCalls != 2 {
		t.Errorf("expected ListDomains to be called once per service, got %d", listDomainsCalls)
	}
	if listTCPProxiesCalls != 2 {
		t.Errorf("expected ListTCPProxies to be called once per service, got %d", listTCPProxiesCalls)
	}
}

func TestRunGetServices_JSONOutputEnrichesNetworkingData(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origOutputFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		outputFormat = origOutputFormat
	}()

	listDomainsCalls := 0
	listTCPProxiesCalls := 0
	token = "test-token"
	outputFormat = "json"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				listDomainsCalls++
				return api.DomainList{}, nil
			},
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				listTCPProxiesCalls++
				return nil, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := getServicesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listDomainsCalls != 1 {
		t.Errorf("expected ListDomains to be called once per service, got %d", listDomainsCalls)
	}
	if listTCPProxiesCalls != 1 {
		t.Errorf("expected ListTCPProxies to be called once per service, got %d", listTCPProxiesCalls)
	}
}

func TestRunGetServices_YAMLOutputEnrichesNetworkingData(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origOutputFormat := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		outputFormat = origOutputFormat
	}()

	listDomainsCalls := 0
	listTCPProxiesCalls := 0
	token = "test-token"
	outputFormat = "yaml"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				listDomainsCalls++
				return api.DomainList{}, nil
			},
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				listTCPProxiesCalls++
				return nil, nil
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := getServicesCmd
	err := cmd.RunE(cmd, []string{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if listDomainsCalls != 1 {
		t.Errorf("expected ListDomains to be called once per service, got %d", listDomainsCalls)
	}
	if listTCPProxiesCalls != 1 {
		t.Errorf("expected ListTCPProxies to be called once per service, got %d", listTCPProxiesCalls)
	}
}

func TestGetServicesOutput_FieldPresenceOnRichPaths(t *testing.T) {
	servicePort := 8080
	services := []types.ServiceDetail{{
		ID:             "svc-1",
		Name:           "api",
		ServiceDomains: []types.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app", TargetPort: &servicePort}},
		CustomDomains:  []types.CustomDomain{{ID: "cdom-1", Domain: "api.example.com", TargetPort: &servicePort}},
		TCPProxies:     []types.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}},
	}}

	out := servicesToOutput(services)
	if len(out) != 1 {
		t.Fatalf("expected one output service, got %d", len(out))
	}
	if len(out[0].ServiceDomains) != 1 || len(out[0].CustomDomains) != 1 || len(out[0].TCPProxies) != 1 {
		t.Errorf("expected networking fields in structured output, got %#v", out[0])
	}
}

func TestGetServicesWideTable_UsesCustomDomainWhenServiceDomainAbsent(t *testing.T) {
	services := []types.ServiceDetail{{
		ID:            "svc-1",
		Name:          "web",
		CustomDomains: []types.CustomDomain{{ID: "cdom-1", Domain: "web.example.com"}},
	}}

	rendered := servicesToWideTable(services).Render()
	if !strings.Contains(rendered, "web.example.com") {
		t.Errorf("expected custom domain summary in wide table, got:\n%s", rendered)
	}
}

func TestGetServicesWideTable_UsesPlaceholderWhenTCPSummaryUnavailable(t *testing.T) {
	services := []types.ServiceDetail{{
		ID:         "svc-1",
		Name:       "db",
		TCPProxies: []types.TCPProxy{{ID: "tcp-1"}},
	}}

	if got := summarizeTCPProxy(services[0]); got != "-" {
		t.Errorf("expected placeholder for incomplete TCP proxy, got %q", got)
	}
}

func TestGetServicesDefaultTable_IgnoresNetworkingDataInRender(t *testing.T) {
	services := []types.ServiceDetail{{
		ID:             "svc-1",
		Name:           "api",
		Source:         "nginx:latest",
		Status:         "SUCCESS",
		ServiceDomains: []types.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}},
		CustomDomains:  []types.CustomDomain{{ID: "cdom-1", Domain: "api.example.com"}},
		TCPProxies:     []types.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}},
	}}

	rendered := servicesToTable(services).Render()
	if !strings.Contains(rendered, "NAME") || !strings.Contains(rendered, "SOURCE") || !strings.Contains(rendered, "STATUS") || !strings.Contains(rendered, "UPDATED") {
		t.Fatalf("expected default table headers, got:\n%s", rendered)
	}
	if strings.Contains(rendered, "api.up.railway.app") || strings.Contains(rendered, "api.example.com") || strings.Contains(rendered, "roundhouse.proxy.rlwy.net") {
		t.Errorf("expected default table render to ignore networking data, got:\n%s", rendered)
	}
}

func TestGetServicesWideTable_RendersPlaceholderWhenNoDomainAvailable(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	rendered := servicesToWideTable(services).Render()
	if !strings.Contains(rendered, "-") {
		t.Errorf("expected placeholder for missing domain/tcp summaries, got:\n%s", rendered)
	}
}

func TestGetServicesOutput_UpdatedAtStillOptional(t *testing.T) {
	out := servicesToOutput([]types.ServiceDetail{{ID: "svc-1", Name: "api"}})
	if out[0].UpdatedAt != "" {
		t.Errorf("expected empty updatedAt, got %q", out[0].UpdatedAt)
	}
}

func TestGetServicesOutput_PreservesStatusAndSourceFields(t *testing.T) {
	out := servicesToOutput([]types.ServiceDetail{{ID: "svc-1", Name: "api", Source: "nginx:latest", SourceType: "image", Status: "SUCCESS"}})
	if out[0].Source != "nginx:latest" || out[0].SourceType != "image" || out[0].Status != "SUCCESS" {
		t.Errorf("expected source/sourceType/status to be preserved, got %#v", out[0])
	}
}

func TestGetServicesWideTable_StillRendersRowsWithoutUpdatedAt(t *testing.T) {
	table := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "api"}})
	if table.RowCount() != 1 {
		t.Errorf("expected 1 row, got %d", table.RowCount())
	}
}

func TestGetServicesDefaultTable_StillRendersRowsWithoutUpdatedAt(t *testing.T) {
	table := servicesToTable([]types.ServiceDetail{{ID: "svc-1", Name: "api"}})
	if table.RowCount() != 1 {
		t.Errorf("expected 1 row, got %d", table.RowCount())
	}
}

func TestGetServicesRichOutput_EnrichmentDoesNotMutateServiceIdentityFields(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api", Source: "nginx:latest", SourceType: "image", Status: "SUCCESS"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{ServiceDomains: []api.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}}}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return []api.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}}, nil
		},
	}

	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)

	if services[0].ID != "svc-1" || services[0].Name != "api" || services[0].Source != "nginx:latest" || services[0].SourceType != "image" || services[0].Status != "SUCCESS" {
		t.Errorf("expected identity fields to remain unchanged, got %#v", services[0])
	}
}

func TestGetServicesRichOutput_EnrichmentFormatsBothDomainKinds(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{
				ServiceDomains: []api.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}},
				CustomDomains:  []api.CustomDomain{{ID: "cdom-1", Domain: "api.example.com"}},
			}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return nil, nil
		},
	}

	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)

	if len(services[0].ServiceDomains) != 1 || len(services[0].CustomDomains) != 1 {
		t.Errorf("expected both domain kinds to be mapped, got %#v", services[0])
	}
}

func TestGetServicesRichOutput_EnrichmentAllowsDomainFailureAndTCPFailureIndependently(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{}, fmt.Errorf("domains unavailable")
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return []api.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}}, nil
		},
	}

	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)

	if len(services[0].ServiceDomains) != 0 || len(services[0].CustomDomains) != 0 {
		t.Errorf("expected failed domain lookup to leave domains empty, got %#v", services[0])
	}
	if len(services[0].TCPProxies) != 1 {
		t.Errorf("expected tcp proxies to still be populated, got %#v", services[0].TCPProxies)
	}
}

func TestGetServicesRichOutput_EnrichmentAllowsTCPFailureAndDomainSuccessIndependently(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{ServiceDomains: []api.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}}}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return nil, fmt.Errorf("tcp unavailable")
		},
	}

	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)

	if len(services[0].ServiceDomains) != 1 {
		t.Errorf("expected service domains to still be populated, got %#v", services[0].ServiceDomains)
	}
	if len(services[0].TCPProxies) != 0 {
		t.Errorf("expected failed tcp lookup to leave proxies empty, got %#v", services[0].TCPProxies)
	}
}

func TestGetServicesWideTable_UsesFirstTCPProxyOnly(t *testing.T) {
	svc := types.ServiceDetail{TCPProxies: []types.TCPProxy{{ID: "tcp-1", Domain: "first.proxy.rlwy.net", ProxyPort: 1111}, {ID: "tcp-2", Domain: "second.proxy.rlwy.net", ProxyPort: 2222}}}
	if got := summarizeTCPProxy(svc); got != "first.proxy.rlwy.net:1111" {
		t.Errorf("expected first tcp proxy summary, got %q", got)
	}
}

func TestGetServicesWideTable_UsesFirstDomainOnly(t *testing.T) {
	svc := types.ServiceDetail{ServiceDomains: []types.ServiceDomain{{ID: "dom-1", Domain: "first.up.railway.app"}, {ID: "dom-2", Domain: "second.up.railway.app"}}}
	if got := summarizeServiceDomain(svc); got != "first.up.railway.app" {
		t.Errorf("expected first domain summary, got %q", got)
	}
}

func TestGetServicesWideTable_PrefersServiceDomainOverCustomDomain(t *testing.T) {
	svc := types.ServiceDetail{ServiceDomains: []types.ServiceDomain{{ID: "dom-1", Domain: "svc.up.railway.app"}}, CustomDomains: []types.CustomDomain{{ID: "cdom-1", Domain: "custom.example.com"}}}
	if got := summarizeServiceDomain(svc); got != "svc.up.railway.app" {
		t.Errorf("expected service domain to win, got %q", got)
	}
}

func TestGetServicesWideTable_TruncatesSourceStill(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api", Source: "abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz", SourceType: "github"}}
	rendered := servicesToWideTable(services).Render()
	if !strings.Contains(rendered, "abcdefghijklmnopqrstuvwxyzabcdefghijk...") {
		t.Errorf("expected source truncation to remain, got:\n%s", rendered)
	}
}

func TestGetServicesWideTable_TruncatesIDStill(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-123456789012345", Name: "api"}}
	rendered := servicesToWideTable(services).Render()
	if !strings.Contains(rendered, "svc-12345678") {
		t.Errorf("expected id truncation to remain, got:\n%s", rendered)
	}
}

func TestGetServicesRichOutput_SkipsEnrichmentForUnknownFormatBehaviorEquivalentToTable(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	called := false
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			called = true
			return api.DomainList{}, nil
		},
	}
	enrichServicesForRichOutput(client, output.Format("table"), "proj-1", "env-1", services)
	if called {
		t.Error("expected non-rich format to skip enrichment")
	}
}

func TestGetServicesWideTable_HeadersRemainOrdered(t *testing.T) {
	rendered := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "api"}}).Render()
	if !strings.Contains(rendered, "NAME") || !strings.Contains(rendered, "ID") || !strings.Contains(rendered, "SOURCE") || !strings.Contains(rendered, "TYPE") || !strings.Contains(rendered, "STATUS") || !strings.Contains(rendered, "DOMAIN") || !strings.Contains(rendered, "TCP") || !strings.Contains(rendered, "UPDATED") {
		t.Errorf("expected all wide headers, got:\n%s", rendered)
	}
}

func TestGetServicesDefaultTable_HeadersRemainOrdered(t *testing.T) {
	rendered := servicesToTable([]types.ServiceDetail{{ID: "svc-1", Name: "api"}}).Render()
	if !strings.Contains(rendered, "NAME") || !strings.Contains(rendered, "SOURCE") || !strings.Contains(rendered, "STATUS") || !strings.Contains(rendered, "UPDATED") {
		t.Errorf("expected default headers, got:\n%s", rendered)
	}
}

func TestGetServicesOutput_StructuredFieldsAreOmittedWhenEmptyByZeroValue(t *testing.T) {
	out := servicesToOutput([]types.ServiceDetail{{ID: "svc-1", Name: "api"}})
	if out[0].ServiceDomains != nil && len(out[0].ServiceDomains) != 0 {
		t.Errorf("expected empty serviceDomains, got %#v", out[0].ServiceDomains)
	}
	if out[0].CustomDomains != nil && len(out[0].CustomDomains) != 0 {
		t.Errorf("expected empty customDomains, got %#v", out[0].CustomDomains)
	}
	if out[0].TCPProxies != nil && len(out[0].TCPProxies) != 0 {
		t.Errorf("expected empty tcpProxies, got %#v", out[0].TCPProxies)
	}
}

func TestGetServicesOutput_StructuredFieldsPopulateThroughServiceDetail(t *testing.T) {
	port := 8080
	out := servicesToOutput([]types.ServiceDetail{{ID: "svc-1", Name: "api", ServiceDomains: []types.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app", TargetPort: &port}}}})
	if len(out[0].ServiceDomains) != 1 || out[0].ServiceDomains[0].Domain != "api.up.railway.app" {
		t.Errorf("expected populated serviceDomains, got %#v", out[0].ServiceDomains)
	}
}

func TestGetServicesOutput_CustomDomainsPopulateThroughServiceDetail(t *testing.T) {
	port := 3000
	out := servicesToOutput([]types.ServiceDetail{{ID: "svc-1", Name: "api", CustomDomains: []types.CustomDomain{{ID: "cdom-1", Domain: "api.example.com", TargetPort: &port}}}})
	if len(out[0].CustomDomains) != 1 || out[0].CustomDomains[0].Domain != "api.example.com" {
		t.Errorf("expected populated customDomains, got %#v", out[0].CustomDomains)
	}
}

func TestGetServicesOutput_TCPProxiesPopulateThroughServiceDetail(t *testing.T) {
	out := servicesToOutput([]types.ServiceDetail{{ID: "svc-1", Name: "api", TCPProxies: []types.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}}}})
	if len(out[0].TCPProxies) != 1 || out[0].TCPProxies[0].ProxyPort != 44321 {
		t.Errorf("expected populated tcpProxies, got %#v", out[0].TCPProxies)
	}
}

func TestGetServicesWideTable_UsesPlaceholderWhenDomainOnlyCustomMissingAndNoServiceDomain(t *testing.T) {
	if got := summarizeServiceDomain(types.ServiceDetail{}); got != "-" {
		t.Errorf("expected placeholder, got %q", got)
	}
}

func TestGetServicesWideTable_UsesPlaceholderWhenTCPProxyMissingFields(t *testing.T) {
	if got := summarizeTCPProxy(types.ServiceDetail{TCPProxies: []types.TCPProxy{{ID: "tcp-1", Domain: "host-only"}}}); got != "-" {
		t.Errorf("expected placeholder for missing proxy port, got %q", got)
	}
}

func TestGetServicesWideTable_UsesPlaceholderWhenTCPProxyMissingDomain(t *testing.T) {
	if got := summarizeTCPProxy(types.ServiceDetail{TCPProxies: []types.TCPProxy{{ID: "tcp-1", ProxyPort: 1234}}}); got != "-" {
		t.Errorf("expected placeholder for missing proxy domain, got %q", got)
	}
}

func TestGetServicesRichOutput_EnrichmentOnWideAlsoMapsCustomDomains(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "web"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{CustomDomains: []api.CustomDomain{{ID: "cdom-1", Domain: "web.example.com"}}}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return nil, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatWide, "proj-1", "env-1", services)
	if len(services[0].CustomDomains) != 1 || services[0].CustomDomains[0].Domain != "web.example.com" {
		t.Errorf("expected custom domains to be mapped on wide enrichment, got %#v", services[0].CustomDomains)
	}
}

func TestGetServicesRichOutput_EnrichmentOnWideAlsoMapsTCPProxies(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "db"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return []api.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}}, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatWide, "proj-1", "env-1", services)
	if len(services[0].TCPProxies) != 1 || services[0].TCPProxies[0].ProxyPort != 44321 {
		t.Errorf("expected tcp proxies to be mapped on wide enrichment, got %#v", services[0].TCPProxies)
	}
}

func TestGetServicesWideTable_SummaryUsesMappedNetworkingData(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api", ServiceDomains: []types.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}}, TCPProxies: []types.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321}}}}
	rendered := servicesToWideTable(services).Render()
	if !strings.Contains(rendered, "api.up.railway.app") || !strings.Contains(rendered, "roundhouse.proxy.rlwy.net:44321") {
		t.Errorf("expected rendered summaries from mapped data, got:\n%s", rendered)
	}
}

func TestGetServicesOutput_RoundTripsNilSlicesAsEmptyLengthInGo(t *testing.T) {
	out := servicesToOutput([]types.ServiceDetail{{ID: "svc-1", Name: "api"}})
	if len(out[0].ServiceDomains) != 0 || len(out[0].CustomDomains) != 0 || len(out[0].TCPProxies) != 0 {
		t.Errorf("expected zero-length networking slices, got %#v", out[0])
	}
}

func TestGetServicesRichOutput_EnrichmentNoopForEmptyServiceList(t *testing.T) {
	called := false
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			called = true
			return api.DomainList{}, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", nil)
	if called {
		t.Error("expected no lookups for empty service list")
	}
}

func TestGetServicesWideTable_NoRows(t *testing.T) {
	if got := servicesToWideTable(nil).RowCount(); got != 0 {
		t.Errorf("expected 0 rows, got %d", got)
	}
}

func TestGetServicesDefaultTable_NoRows(t *testing.T) {
	if got := servicesToTable(nil).RowCount(); got != 0 {
		t.Errorf("expected 0 rows, got %d", got)
	}
}

func TestGetServicesOutput_NoRows(t *testing.T) {
	if got := len(servicesToOutput(nil)); got != 0 {
		t.Errorf("expected 0 outputs, got %d", got)
	}
}

func TestGetServicesWideTable_WithOnlyCustomDomainAndIncompleteTCPStillRenders(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "web", CustomDomains: []types.CustomDomain{{ID: "cdom-1", Domain: "web.example.com"}}, TCPProxies: []types.TCPProxy{{ID: "tcp-1", Domain: "host-only"}}}}
	rendered := servicesToWideTable(services).Render()
	if !strings.Contains(rendered, "web.example.com") {
		t.Errorf("expected custom domain summary, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "-") {
		t.Errorf("expected placeholder for incomplete tcp summary, got:\n%s", rendered)
	}
}

func TestGetServicesRichOutput_EnrichmentKeepsOtherServicesIndependent(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}, {ID: "svc-2", Name: "db"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			if serviceID == "svc-1" {
				return api.DomainList{ServiceDomains: []api.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}}}, nil
			}
			return api.DomainList{}, fmt.Errorf("domains unavailable")
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			if serviceID == "svc-2" {
				return []api.TCPProxy{{ID: "tcp-2", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 45678, ApplicationPort: 5432}}, nil
			}
			return nil, fmt.Errorf("tcp unavailable")
		},
	}
	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)
	if len(services[0].ServiceDomains) != 1 || len(services[0].TCPProxies) != 0 {
		t.Errorf("unexpected svc-1 enrichment result: %#v", services[0])
	}
	if len(services[1].ServiceDomains) != 0 || len(services[1].TCPProxies) != 1 {
		t.Errorf("unexpected svc-2 enrichment result: %#v", services[1])
	}
}

func TestGetServicesWideTable_UsesFirstCustomDomainIfNoServiceDomain(t *testing.T) {
	svc := types.ServiceDetail{CustomDomains: []types.CustomDomain{{ID: "cdom-1", Domain: "first.example.com"}, {ID: "cdom-2", Domain: "second.example.com"}}}
	if got := summarizeServiceDomain(svc); got != "first.example.com" {
		t.Errorf("expected first custom domain summary, got %q", got)
	}
}

func TestGetServicesRichOutput_EnrichmentOnYAMLBehavesLikeJSON(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{ServiceDomains: []api.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}}}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return []api.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}}, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatYAML, "proj-1", "env-1", services)
	if len(services[0].ServiceDomains) != 1 || len(services[0].TCPProxies) != 1 {
		t.Errorf("expected yaml enrichment to populate networking fields, got %#v", services[0])
	}
}

func TestGetServicesRichOutput_EnrichmentOnWideBehavesLikeJSON(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{ServiceDomains: []api.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}}}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return []api.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}}, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatWide, "proj-1", "env-1", services)
	if len(services[0].ServiceDomains) != 1 || len(services[0].TCPProxies) != 1 {
		t.Errorf("expected wide enrichment to populate networking fields, got %#v", services[0])
	}
}

func TestGetServicesRichOutput_EnrichmentSkipsTableOnly(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{ServiceDomains: []api.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}}}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return []api.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}}, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatTable, "proj-1", "env-1", services)
	if len(services[0].ServiceDomains) != 0 || len(services[0].TCPProxies) != 0 {
		t.Errorf("expected table format to skip enrichment, got %#v", services[0])
	}
}

func TestGetServicesWideTable_RendersBothNewColumnsEvenWhenEmpty(t *testing.T) {
	rendered := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "api"}}).Render()
	if !strings.Contains(rendered, "DOMAIN") || !strings.Contains(rendered, "TCP") {
		t.Errorf("expected new wide columns, got:\n%s", rendered)
	}
}

func TestGetServicesRichOutput_EnrichmentUsesServiceIDPerRow(t *testing.T) {
	seen := []string{}
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}, {ID: "svc-2", Name: "worker"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			seen = append(seen, serviceID)
			return api.DomainList{}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return nil, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)
	if len(seen) != 2 || seen[0] != "svc-1" || seen[1] != "svc-2" {
		t.Errorf("expected per-service domain lookups, got %#v", seen)
	}
}

func TestGetServicesRichOutput_EnrichmentUsesEnvironmentIDPerRow(t *testing.T) {
	seen := []string{}
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}, {ID: "svc-2", Name: "worker"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			seen = append(seen, environmentID+":"+serviceID)
			return nil, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)
	if len(seen) != 2 || seen[0] != "env-1:svc-1" || seen[1] != "env-1:svc-2" {
		t.Errorf("expected per-service tcp lookups with env id, got %#v", seen)
	}
}

func TestGetServicesRichOutput_EnrichmentUsesProjectIDForDomainLookups(t *testing.T) {
	seen := []string{}
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			seen = append(seen, projectID+":"+environmentID+":"+serviceID)
			return api.DomainList{}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return nil, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)
	if len(seen) != 1 || seen[0] != "proj-1:env-1:svc-1" {
		t.Errorf("expected project/env/service ids in domain lookup, got %#v", seen)
	}
}

func TestGetServicesWideTable_CustomDomainSummaryVisibleWhenPresent(t *testing.T) {
	rendered := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "web", CustomDomains: []types.CustomDomain{{ID: "cdom-1", Domain: "web.example.com"}}}}).Render()
	if !strings.Contains(rendered, "web.example.com") {
		t.Errorf("expected custom domain summary, got:\n%s", rendered)
	}
}

func TestGetServicesWideTable_TCPSummaryVisibleWhenPresent(t *testing.T) {
	rendered := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "db", TCPProxies: []types.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321}}}}).Render()
	if !strings.Contains(rendered, "roundhouse.proxy.rlwy.net:44321") {
		t.Errorf("expected tcp summary, got:\n%s", rendered)
	}
}

func TestGetServicesRichOutput_EnrichmentLeavesServicesSliceLengthUnchanged(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}, {ID: "svc-2", Name: "db"}}
	client := &api.MockClient{}
	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)
	if len(services) != 2 {
		t.Errorf("expected slice length unchanged, got %d", len(services))
	}
}

func TestGetServicesOutput_PreservesIDAndName(t *testing.T) {
	out := servicesToOutput([]types.ServiceDetail{{ID: "svc-1", Name: "api"}})
	if out[0].ID != "svc-1" || out[0].Name != "api" {
		t.Errorf("expected id and name preserved, got %#v", out[0])
	}
}

func TestGetServicesWideTable_PlaceholderForMissingBothDomainAndTCP(t *testing.T) {
	rendered := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "api"}}).Render()
	if !strings.Contains(rendered, "-") {
		t.Errorf("expected placeholder values, got:\n%s", rendered)
	}
}

func TestGetServicesRichOutput_EnrichmentSkipsWithoutRichFormats(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	called := false
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			called = true
			return api.DomainList{}, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatTable, "proj-1", "env-1", services)
	if called {
		t.Error("expected no domain lookups for default table")
	}
}

func TestGetServicesWideTable_SummaryFallsBackGracefully(t *testing.T) {
	svc := types.ServiceDetail{CustomDomains: []types.CustomDomain{{ID: "cdom-1", Domain: "custom.example.com"}}, TCPProxies: []types.TCPProxy{{ID: "tcp-1", Domain: "proxy.example.com", ProxyPort: 1234}}}
	if got := summarizeServiceDomain(svc); got != "custom.example.com" {
		t.Errorf("expected custom domain fallback, got %q", got)
	}
	if got := summarizeTCPProxy(svc); got != "proxy.example.com:1234" {
		t.Errorf("expected tcp proxy summary, got %q", got)
	}
}

func TestGetServicesWideTable_DoesNotNeedStartCommand(t *testing.T) {
	table := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "api"}})
	if table.RowCount() != 1 {
		t.Errorf("expected 1 row, got %d", table.RowCount())
	}
}

func TestGetServicesDefaultTable_DoesNotNeedStartCommand(t *testing.T) {
	table := servicesToTable([]types.ServiceDetail{{ID: "svc-1", Name: "api"}})
	if table.RowCount() != 1 {
		t.Errorf("expected 1 row, got %d", table.RowCount())
	}
}

func TestGetServicesWideTable_StillUsesSourceTypeColumn(t *testing.T) {
	rendered := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "api", SourceType: "github"}}).Render()
	if !strings.Contains(rendered, "github") {
		t.Errorf("expected source type to remain in wide table, got:\n%s", rendered)
	}
}

func TestGetServicesDefaultTable_StillUsesStatusColumn(t *testing.T) {
	rendered := servicesToTable([]types.ServiceDetail{{ID: "svc-1", Name: "api", Status: "SUCCESS"}}).Render()
	if !strings.Contains(rendered, "SUCCESS") {
		t.Errorf("expected status to remain in default table, got:\n%s", rendered)
	}
}

func TestGetServicesWideTable_StillUsesStatusColumn(t *testing.T) {
	rendered := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "api", Status: "SUCCESS"}}).Render()
	if !strings.Contains(rendered, "SUCCESS") {
		t.Errorf("expected status to remain in wide table, got:\n%s", rendered)
	}
}

func TestGetServicesWideTable_StillUsesSourceColumn(t *testing.T) {
	rendered := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "api", Source: "nginx:latest"}}).Render()
	if !strings.Contains(rendered, "nginx:latest") {
		t.Errorf("expected source to remain in wide table, got:\n%s", rendered)
	}
}

func TestGetServicesDefaultTable_StillUsesSourceColumn(t *testing.T) {
	rendered := servicesToTable([]types.ServiceDetail{{ID: "svc-1", Name: "api", Source: "nginx:latest"}}).Render()
	if !strings.Contains(rendered, "nginx:latest") {
		t.Errorf("expected source to remain in default table, got:\n%s", rendered)
	}
}

func TestGetServicesRichOutput_EnrichmentNoopForNilClientFuncs(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	client := &api.MockClient{}
	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)
	if len(services[0].ServiceDomains) != 0 || len(services[0].CustomDomains) != 0 || len(services[0].TCPProxies) != 0 {
		t.Errorf("expected empty networking fields with nil client funcs, got %#v", services[0])
	}
}

func TestGetServicesRichOutput_EnrichmentKeepsZeroValuesOnFailure(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			return api.DomainList{}, fmt.Errorf("domains unavailable")
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			return nil, fmt.Errorf("tcp unavailable")
		},
	}
	enrichServicesForRichOutput(client, output.FormatWide, "proj-1", "env-1", services)
	if services[0].ServiceDomains != nil || services[0].CustomDomains != nil || services[0].TCPProxies != nil {
		t.Errorf("expected zero values to remain nil on failure, got %#v", services[0])
	}
}

func TestGetServicesWideTable_RenderIncludesNewColumnsBeforeUpdated(t *testing.T) {
	rendered := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "api"}}).Render()
	if !strings.Contains(rendered, "DOMAIN") || !strings.Contains(rendered, "TCP") || !strings.Contains(rendered, "UPDATED") {
		t.Errorf("expected render to include new columns and updated, got:\n%s", rendered)
	}
}

func TestGetServicesStructuredOutput_NetworkingFieldsComeFromServiceDetail(t *testing.T) {
	port := 8080
	out := servicesToOutput([]types.ServiceDetail{{ID: "svc-1", Name: "api", ServiceDomains: []types.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app", TargetPort: &port}}, CustomDomains: []types.CustomDomain{{ID: "cdom-1", Domain: "api.example.com", TargetPort: &port}}, TCPProxies: []types.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}}}})
	if len(out[0].ServiceDomains) != 1 || len(out[0].CustomDomains) != 1 || len(out[0].TCPProxies) != 1 {
		t.Errorf("expected all networking fields from service detail, got %#v", out[0])
	}
}

func TestGetServicesWideTable_SummaryFunctionsDoNotPanicOnEmptySlices(t *testing.T) {
	_ = summarizeServiceDomain(types.ServiceDetail{ServiceDomains: []types.ServiceDomain{}, CustomDomains: []types.CustomDomain{}})
	_ = summarizeTCPProxy(types.ServiceDetail{TCPProxies: []types.TCPProxy{}})
}

func TestGetServicesRichOutput_EnrichmentUsesFormatGateExactly(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}}
	called := false
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			called = true
			return api.DomainList{}, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatWide, "proj-1", "env-1", services)
	if !called {
		t.Error("expected wide format to trigger enrichment")
	}
}

func TestGetServicesWideTable_RenderShowsHeadersWithNoRowsViaTableObject(t *testing.T) {
	table := servicesToWideTable(nil)
	if table == nil {
		t.Fatal("expected table object")
	}
}

func TestGetServicesDefaultTable_RenderShowsHeadersWithNoRowsViaTableObject(t *testing.T) {
	table := servicesToTable(nil)
	if table == nil {
		t.Fatal("expected table object")
	}
}

func TestGetServicesStructuredOutput_EmptySliceInputReturnsEmptySlice(t *testing.T) {
	out := servicesToOutput([]types.ServiceDetail{})
	if len(out) != 0 {
		t.Errorf("expected empty slice output, got %d", len(out))
	}
}

func TestGetServicesStructuredOutput_NilInputReturnsEmptySlice(t *testing.T) {
	out := servicesToOutput(nil)
	if len(out) != 0 {
		t.Errorf("expected empty slice output, got %d", len(out))
	}
}

func TestGetServicesWideTable_StillHandlesMultipleRows(t *testing.T) {
	table := servicesToWideTable([]types.ServiceDetail{{ID: "svc-1", Name: "api"}, {ID: "svc-2", Name: "web"}})
	if table.RowCount() != 2 {
		t.Errorf("expected 2 rows, got %d", table.RowCount())
	}
}

func TestGetServicesDefaultTable_StillHandlesMultipleRows(t *testing.T) {
	table := servicesToTable([]types.ServiceDetail{{ID: "svc-1", Name: "api"}, {ID: "svc-2", Name: "web"}})
	if table.RowCount() != 2 {
		t.Errorf("expected 2 rows, got %d", table.RowCount())
	}
}

func TestGetServicesRichOutput_EnrichmentPerServiceErrorsDoNotShortCircuitLoop(t *testing.T) {
	services := []types.ServiceDetail{{ID: "svc-1", Name: "api"}, {ID: "svc-2", Name: "web"}}
	seen := []string{}
	client := &api.MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
			seen = append(seen, "domains:"+serviceID)
			if serviceID == "svc-1" {
				return api.DomainList{}, fmt.Errorf("fail")
			}
			return api.DomainList{}, nil
		},
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
			seen = append(seen, "tcp:"+serviceID)
			if serviceID == "svc-1" {
				return nil, fmt.Errorf("fail")
			}
			return nil, nil
		},
	}
	enrichServicesForRichOutput(client, output.FormatJSON, "proj-1", "env-1", services)
	if len(seen) != 4 {
		t.Errorf("expected all lookups to run, got %#v", seen)
	}
}

func TestGetServicesWideTable_DomainSummaryDoesNotIncludePort(t *testing.T) {
	port := 8080
	svc := types.ServiceDetail{ServiceDomains: []types.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app", TargetPort: &port}}}
	if got := summarizeServiceDomain(svc); got != "api.up.railway.app" {
		t.Errorf("expected raw domain only, got %q", got)
	}
}

func TestGetServicesWideTable_TCPSummaryIncludesProxyPort(t *testing.T) {
	svc := types.ServiceDetail{TCPProxies: []types.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321}}}
	if got := summarizeTCPProxy(svc); got != "roundhouse.proxy.rlwy.net:44321" {
		t.Errorf("expected host:port summary, got %q", got)
	}
}

func TestGetServicesRichOutput_EnrichmentWithRichFormatAndNoServicesIsSafe(t *testing.T) {
	enrichServicesForRichOutput(&api.MockClient{}, output.FormatJSON, "proj-1", "env-1", []types.ServiceDetail{})
}

func TestServiceDetailToOutput(t *testing.T) {
	servicePort := 8080
	customPort := 3000
	svc := types.ServiceDetail{
		ID:           "svc-1",
		Name:         "api",
		Source:       "nginx:latest",
		SourceType:   "image",
		StartCommand: "nginx -g 'daemon off;'",
		UpdatedAt:    time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		ServiceDomains: []types.ServiceDomain{
			{ID: "dom-1", Domain: "api.up.railway.app", TargetPort: &servicePort},
		},
		CustomDomains: []types.CustomDomain{
			{ID: "cdom-1", Domain: "api.example.com", TargetPort: &customPort},
		},
		TCPProxies: []types.TCPProxy{
			{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432},
		},
	}

	result := serviceDetailToOutput(svc, "my-project", "production", nil, nil, false)

	if result.Name != "api" {
		t.Errorf("expected name 'api', got %q", result.Name)
	}
	if result.Project != "my-project" {
		t.Errorf("expected project 'my-project', got %q", result.Project)
	}
	if result.Environment != "production" {
		t.Errorf("expected environment 'production', got %q", result.Environment)
	}
	if result.StartCommand != "nginx -g 'daemon off;'" {
		t.Errorf("expected startCommand, got %q", result.StartCommand)
	}
	if len(result.ServiceDomains) != 1 || result.ServiceDomains[0].Domain != "api.up.railway.app" {
		t.Errorf("expected service domain in output, got %#v", result.ServiceDomains)
	}
	if len(result.CustomDomains) != 1 || result.CustomDomains[0].Domain != "api.example.com" {
		t.Errorf("expected custom domain in output, got %#v", result.CustomDomains)
	}
	if len(result.TCPProxies) != 1 || result.TCPProxies[0].ApplicationPort != 5432 {
		t.Errorf("expected TCP proxy in output, got %#v", result.TCPProxies)
	}
}

func TestServiceDetailToOutput_WithoutNetworkingFields(t *testing.T) {
	svc := types.ServiceDetail{
		ID:   "svc-1",
		Name: "api",
	}

	result := serviceDetailToOutput(svc, "my-project", "production", nil, nil, false)

	if len(result.ServiceDomains) != 0 {
		t.Errorf("expected no service domains, got %#v", result.ServiceDomains)
	}
	if len(result.CustomDomains) != 0 {
		t.Errorf("expected no custom domains, got %#v", result.CustomDomains)
	}
	if len(result.TCPProxies) != 0 {
		t.Errorf("expected no TCP proxies, got %#v", result.TCPProxies)
	}
}

func TestRunDescribeService_NetworkingFetchFailureNonFatal(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
			},
			GetVariablesFunc: func(projectID, environmentID, serviceID string) (map[string]string, error) {
				return map[string]string{"PORT": "3000"}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, fmt.Errorf("domains unavailable")
			},
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return nil, fmt.Errorf("tcp unavailable")
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := describeServiceCmd
	err := cmd.RunE(cmd, []string{"api"})

	if err != nil {
		t.Errorf("expected non-fatal networking errors, got %v", err)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"this is a long string", 10, "this is..."},
		{"exactly10.", 10, "exactly10."},
	}

	for _, tc := range tests {
		result := truncate(tc.input, tc.maxLen)
		if result != tc.expected {
			t.Errorf("truncate(%q, %d) = %q, expected %q", tc.input, tc.maxLen, result, tc.expected)
		}
	}
}

func TestRunUpdateService_MissingProject(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{}
	}
	project = "" // No project set

	cmd := updateServiceCmd
	err := cmd.RunE(cmd, []string{"myservice"})

	if err == nil {
		t.Error("expected error for missing project")
	}
	if err != nil && err.Error() != "-p/--project is required. Use -p flag or set RAILCTL_PROJECT" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunUpdateService_MissingEnvironment(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
		}
	}
	project = "my-project"
	environment = "" // No environment set

	cmd := updateServiceCmd
	err := cmd.RunE(cmd, []string{"myservice"})

	if err == nil {
		t.Error("expected error for missing environment")
	}
}

func TestRunUpdateService_ServiceNotFound(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{}, nil // No services
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := updateServiceCmd
	err := cmd.RunE(cmd, []string{"nonexistent"})

	if err == nil {
		t.Error("expected error for service not found")
	}
	if err != nil && !strings.Contains(err.Error(), "service 'nonexistent' not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunUpdateService_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origImage := updateServiceImage
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		updateServiceImage = origImage
	}()

	var updatedServiceID, updatedEnvID, updatedImage string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			UpdateServiceInstanceFunc: func(serviceID, environmentID, image string, creds *api.RegistryCredentials) error {
				updatedServiceID = serviceID
				updatedEnvID = environmentID
				updatedImage = image
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	updateServiceImage = "node:20-alpine"

	cmd := updateServiceCmd
	err := cmd.RunE(cmd, []string{"api"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if updatedServiceID != "svc-1" {
		t.Errorf("expected service ID 'svc-1', got %q", updatedServiceID)
	}
	if updatedEnvID != "env-1" {
		t.Errorf("expected environment ID 'env-1', got %q", updatedEnvID)
	}
	if updatedImage != "node:20-alpine" {
		t.Errorf("expected image 'node:20-alpine', got %q", updatedImage)
	}
}

func TestRunCreateService_MissingProject(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{}
	}
	project = "" // No project set

	cmd := createServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err == nil {
		t.Error("expected error for missing project")
	}
	if err != nil && err.Error() != "-p/--project is required. Use -p flag or set RAILCTL_PROJECT" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCreateService_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origImage := serviceImage
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		serviceImage = origImage
	}()

	var createdProjectID, createdName, createdImage string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			CreateServiceFunc: func(projectID, environmentID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
				createdProjectID = projectID
				createdName = name
				createdImage = image
				return types.Service{ID: "svc-new", Name: name}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	serviceImage = "nginx:latest"

	cmd := createServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if createdProjectID != "proj-1" {
		t.Errorf("expected project ID 'proj-1', got %q", createdProjectID)
	}
	if createdName != "my-service" {
		t.Errorf("expected name 'my-service', got %q", createdName)
	}
	if createdImage != "nginx:latest" {
		t.Errorf("expected image 'nginx:latest', got %q", createdImage)
	}
}

func TestRunCreateService_WithRegistryCredentials(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origImage := serviceImage
	origRegUser := createRegistryUsername
	origRegPass := createRegistryPassword
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		serviceImage = origImage
		createRegistryUsername = origRegUser
		createRegistryPassword = origRegPass
	}()

	var capturedCreds *api.RegistryCredentials

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			CreateServiceFunc: func(projectID, environmentID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
				capturedCreds = creds
				return types.Service{ID: "svc-new", Name: name}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	serviceImage = "registry.example.com/myapp:v1"
	createRegistryUsername = "testuser"
	createRegistryPassword = "testpass"

	cmd := createServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if capturedCreds == nil {
		t.Error("expected registry credentials to be passed, got nil")
	} else {
		if capturedCreds.Username != "testuser" {
			t.Errorf("expected username 'testuser', got %q", capturedCreds.Username)
		}
		if capturedCreds.Password != "testpass" {
			t.Errorf("expected password 'testpass', got %q", capturedCreds.Password)
		}
	}
}

// NEW TESTS FOR DEPLOYMENT CONFIGURATION FLAGS

func TestRunCreateService_WithDeployConfig(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origImage := serviceImage
	origStartCmd := createServiceStartCommand
	origRestartPolicy := createServiceRestartPolicy
	origMaxRetries := createServiceMaxRetries
	origReplicas := createServiceReplicas
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		serviceImage = origImage
		createServiceStartCommand = origStartCmd
		createServiceRestartPolicy = origRestartPolicy
		createServiceMaxRetries = origMaxRetries
		createServiceReplicas = origReplicas
		// Reset flag "Changed" state to prevent leakage to other tests
		createServiceCmd.Flags().Lookup("start-command").Changed = false
		createServiceCmd.Flags().Lookup("restart-policy").Changed = false
		createServiceCmd.Flags().Lookup("max-retries").Changed = false
		createServiceCmd.Flags().Lookup("replicas").Changed = false
	}()

	var capturedStartCmd, capturedRestartPolicy *string
	var capturedMaxRetries, capturedReplicas *int

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			CreateServiceFunc: func(projectID, environmentID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
				return types.Service{ID: "svc-new", Name: name}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			UpdateServiceInstanceConfigFunc: func(serviceID, envID string, startCmd, restartPolicy *string, maxRetries, replicas *int, healthcheckPath *string, healthcheckTimeout *int) error {
				capturedStartCmd = startCmd
				capturedRestartPolicy = restartPolicy
				capturedMaxRetries = maxRetries
				capturedReplicas = replicas
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	serviceImage = "node:20"
	createServiceStartCommand = "npm start"
	createServiceRestartPolicy = "ON_FAILURE"
	createServiceMaxRetries = 3
	createServiceReplicas = 2

	cmd := createServiceCmd
	// Simulate flags being set
	cmd.Flags().Set("start-command", "npm start")
	cmd.Flags().Set("restart-policy", "ON_FAILURE")
	cmd.Flags().Set("max-retries", "3")
	cmd.Flags().Set("replicas", "2")

	err := cmd.RunE(cmd, []string{"my-service"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if capturedStartCmd == nil || *capturedStartCmd != "npm start" {
		t.Errorf("expected start command 'npm start', got %v", capturedStartCmd)
	}
	if capturedRestartPolicy == nil || *capturedRestartPolicy != "ON_FAILURE" {
		t.Errorf("expected restart policy 'ON_FAILURE', got %v", capturedRestartPolicy)
	}
	if capturedMaxRetries == nil || *capturedMaxRetries != 3 {
		t.Errorf("expected max retries 3, got %v", capturedMaxRetries)
	}
	if capturedReplicas == nil || *capturedReplicas != 2 {
		t.Errorf("expected replicas 2, got %v", capturedReplicas)
	}
}

func TestRunCreateService_InvalidRestartPolicy(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origImage := serviceImage
	origRestartPolicy := createServiceRestartPolicy
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		serviceImage = origImage
		createServiceRestartPolicy = origRestartPolicy
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			CreateServiceFunc: func(projectID, environmentID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
				return types.Service{ID: "svc-new", Name: name}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	serviceImage = "node:20"
	createServiceRestartPolicy = "INVALID_POLICY"

	cmd := createServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err == nil {
		t.Error("expected error for invalid restart policy")
	}
	if err != nil && !contains(err.Error(), "invalid restart policy") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCreateService_MaxRetriesWithoutRestartPolicy(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origImage := serviceImage
	origMaxRetries := createServiceMaxRetries
	origReplicas := createServiceReplicas
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		serviceImage = origImage
		createServiceMaxRetries = origMaxRetries
		createServiceReplicas = origReplicas
		// Reset flags
		createServiceCmd.Flags().Lookup("max-retries").Changed = false
		createServiceCmd.Flags().Set("max-retries", "0")
		createServiceCmd.Flags().Set("replicas", "0")
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			CreateServiceFunc: func(projectID, environmentID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
				return types.Service{ID: "svc-new", Name: name}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	serviceImage = "node:20"
	createServiceMaxRetries = 3
	createServiceReplicas = 0 // Don't set replicas

	cmd := createServiceCmd
	// Only set max-retries flag, not replicas
	cmd.Flags().Lookup("max-retries").Changed = true

	err := cmd.RunE(cmd, []string{"my-service"})

	if err == nil {
		t.Error("expected error for max-retries without restart-policy")
	}
	if err != nil && err.Error() != "--max-retries requires --restart-policy" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCreateService_InvalidReplicas(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origImage := serviceImage
	origReplicas := createServiceReplicas
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		serviceImage = origImage
		createServiceReplicas = origReplicas
		// Reset flags
		createServiceCmd.Flags().Lookup("max-retries").Changed = false
		createServiceCmd.Flags().Lookup("replicas").Changed = false
		createServiceCmd.Flags().Set("replicas", "0")
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			CreateServiceFunc: func(projectID, environmentID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
				return types.Service{ID: "svc-new", Name: name}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	serviceImage = "node:20"
	createServiceReplicas = 0

	// Reset flag state from previous tests
	createServiceCmd.Flags().Lookup("max-retries").Changed = false
	createServiceCmd.Flags().Lookup("restart-policy").Changed = false

	cmd := createServiceCmd
	cmd.Flags().Set("replicas", "0")

	err := cmd.RunE(cmd, []string{"my-service"})

	if err == nil {
		t.Error("expected error for invalid replicas")
	}
	if err != nil && err.Error() != "--replicas must be >= 1" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunCreateService_WithHealthcheck(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origImage := serviceImage
	origHealthcheckPath := createServiceHealthcheckPath
	origHealthcheckTimeout := createServiceHealthcheckTimeout
	origReplicas := createServiceReplicas
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		serviceImage = origImage
		createServiceHealthcheckPath = origHealthcheckPath
		createServiceHealthcheckTimeout = origHealthcheckTimeout
		createServiceReplicas = origReplicas
		// Reset flags
		createServiceCmd.Flags().Lookup("max-retries").Changed = false
		createServiceCmd.Flags().Lookup("healthcheck-path").Changed = false
		createServiceCmd.Flags().Lookup("healthcheck-timeout").Changed = false
		createServiceCmd.Flags().Set("healthcheck-path", "")
		createServiceCmd.Flags().Set("healthcheck-timeout", "0")
		createServiceCmd.Flags().Set("replicas", "0")
	}()

	var capturedHealthcheckPath *string
	var capturedHealthcheckTimeout *int

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			CreateServiceFunc: func(projectID, environmentID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
				return types.Service{ID: "svc-new", Name: name}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			UpdateServiceInstanceConfigFunc: func(serviceID, envID string, startCmd, restartPolicy *string, maxRetries, replicas *int, healthcheckPath *string, healthcheckTimeout *int) error {
				capturedHealthcheckPath = healthcheckPath
				capturedHealthcheckTimeout = healthcheckTimeout
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	serviceImage = "node:20"
	createServiceHealthcheckPath = "/health"
	createServiceHealthcheckTimeout = 60
	createServiceReplicas = 0 // Don't set replicas

	// Reset flag state from previous tests
	createServiceCmd.Flags().Lookup("max-retries").Changed = false
	createServiceCmd.Flags().Lookup("restart-policy").Changed = false
	createServiceCmd.Flags().Lookup("replicas").Changed = false

	cmd := createServiceCmd
	// Only set healthcheck flags, not replicas
	cmd.Flags().Lookup("healthcheck-path").Changed = true
	cmd.Flags().Lookup("healthcheck-timeout").Changed = true

	err := cmd.RunE(cmd, []string{"my-service"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if capturedHealthcheckPath == nil || *capturedHealthcheckPath != "/health" {
		t.Errorf("expected healthcheck path '/health', got %v", capturedHealthcheckPath)
	}
	if capturedHealthcheckTimeout == nil || *capturedHealthcheckTimeout != 60 {
		t.Errorf("expected healthcheck timeout 60, got %v", capturedHealthcheckTimeout)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestRunUpdateService_WithRegistryCredentials(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origImage := updateServiceImage
	origRegUser := updateRegistryUsername
	origRegPass := updateRegistryPassword
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		updateServiceImage = origImage
		updateRegistryUsername = origRegUser
		updateRegistryPassword = origRegPass
	}()

	var capturedCreds *api.RegistryCredentials

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			UpdateServiceInstanceFunc: func(serviceID, environmentID, image string, creds *api.RegistryCredentials) error {
				capturedCreds = creds
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	updateServiceImage = "registry.example.com/myapp:v2"
	updateRegistryUsername = "reguser"
	updateRegistryPassword = "regpass"

	cmd := updateServiceCmd
	err := cmd.RunE(cmd, []string{"api"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if capturedCreds == nil {
		t.Error("expected registry credentials to be passed, got nil")
	} else {
		if capturedCreds.Username != "reguser" {
			t.Errorf("expected username 'reguser', got %q", capturedCreds.Username)
		}
		if capturedCreds.Password != "regpass" {
			t.Errorf("expected password 'regpass', got %q", capturedCreds.Password)
		}
	}
}

func TestRunDeleteService_MissingProject(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{}
	}
	project = "" // No project set

	cmd := deleteServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err == nil {
		t.Error("expected error for missing project")
	}
	if err != nil && err.Error() != "-p/--project is required. Use -p flag or set RAILCTL_PROJECT" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunDeleteService_MissingEnvironment(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
		}
	}
	project = "my-project"
	environment = "" // No environment set

	cmd := deleteServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err == nil {
		t.Error("expected error for missing environment")
	}
	if err != nil && err.Error() != "-e/--environment is required. Use -e flag or set RAILCTL_ENVIRONMENT" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunDeleteService_ServiceNotFound(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{}, nil // Empty list
			},
		}
	}
	project = "my-project"
	environment = "production"

	cmd := deleteServiceCmd
	err := cmd.RunE(cmd, []string{"nonexistent"})

	if err == nil {
		t.Error("expected error for missing service")
	}
	if err != nil && !strings.Contains(err.Error(), "service 'nonexistent' not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunDeleteService_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origYes := deleteServiceYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		deleteServiceYes = origYes
	}()

	var deletedServiceID string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			DeleteServiceFunc: func(id string) error {
				deletedServiceID = id
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	deleteServiceYes = true // Skip confirmation

	cmd := deleteServiceCmd
	err := cmd.RunE(cmd, []string{"api"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deletedServiceID != "svc-1" {
		t.Errorf("expected deleted service ID 'svc-1', got %q", deletedServiceID)
	}
}

func TestRunCreateService_WithGenerateDomain(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origImage := serviceImage
	origGenerateDomain := createServiceGenerateDomain
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		serviceImage = origImage
		createServiceGenerateDomain = origGenerateDomain
		// Reset flags
		createServiceCmd.Flags().Lookup("generate-domain").Changed = false
	}()

	var domainCreated bool

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			CreateServiceFunc: func(projectID, environmentID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
				return types.Service{ID: "svc-new", Name: name}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, nil // No existing domains
			},
			CreateServiceDomainFunc: func(serviceID, environmentID string, targetPort int) (api.ServiceDomain, error) {
				domainCreated = true
				return api.ServiceDomain{ID: "dom-1", Domain: "my-service-production.up.railway.app"}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	serviceImage = "nginx:latest"
	createServiceGenerateDomain = 5678

	// Reset deploy config flags from previous tests
	createServiceCmd.Flags().Lookup("max-retries").Changed = false
	createServiceCmd.Flags().Lookup("restart-policy").Changed = false
	createServiceCmd.Flags().Lookup("replicas").Changed = false
	createServiceCmd.Flags().Lookup("healthcheck-path").Changed = false
	createServiceCmd.Flags().Lookup("healthcheck-timeout").Changed = false

	cmd := createServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !domainCreated {
		t.Error("expected CreateServiceDomain to be called")
	}
}

func TestRunCreateService_WithGenerateDomain_AlreadyExists(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origImage := serviceImage
	origGenerateDomain := createServiceGenerateDomain
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		serviceImage = origImage
		createServiceGenerateDomain = origGenerateDomain
		createServiceCmd.Flags().Lookup("generate-domain").Changed = false
	}()

	var domainCreated bool

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			CreateServiceFunc: func(projectID, environmentID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
				return types.Service{ID: "svc-new", Name: name}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{
					ServiceDomains: []api.ServiceDomain{
						{ID: "dom-1", Domain: "existing.up.railway.app"},
					},
				}, nil // Domain already exists
			},
			CreateServiceDomainFunc: func(serviceID, environmentID string, targetPort int) (api.ServiceDomain, error) {
				domainCreated = true
				return api.ServiceDomain{}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	serviceImage = "nginx:latest"
	createServiceGenerateDomain = 5678

	createServiceCmd.Flags().Lookup("max-retries").Changed = false
	createServiceCmd.Flags().Lookup("restart-policy").Changed = false
	createServiceCmd.Flags().Lookup("replicas").Changed = false
	createServiceCmd.Flags().Lookup("healthcheck-path").Changed = false
	createServiceCmd.Flags().Lookup("healthcheck-timeout").Changed = false

	cmd := createServiceCmd
	err := cmd.RunE(cmd, []string{"my-service"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if domainCreated {
		t.Error("expected CreateServiceDomain NOT to be called when domain already exists")
	}
}

func TestRunUpdateService_WithGenerateDomain(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origImage := updateServiceImage
	origGenerateDomain := updateServiceGenerateDomain
	origGenerateTCP := updateServiceGenerateTCP
	origRemoveDomain := updateServiceRemoveDomain
	origRemoveTCP := updateServiceRemoveTCP
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		updateServiceImage = origImage
		updateServiceGenerateDomain = origGenerateDomain
		updateServiceGenerateTCP = origGenerateTCP
		updateServiceRemoveDomain = origRemoveDomain
		updateServiceRemoveTCP = origRemoveTCP
	}()
	var domainCreated bool

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "api"},
				}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, nil
			},
			CreateServiceDomainFunc: func(serviceID, environmentID string, targetPort int) (api.ServiceDomain, error) {
				domainCreated = true
				return api.ServiceDomain{ID: "dom-1", Domain: "api-production.up.railway.app"}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	updateServiceImage = "" // No image update -- generate-domain only
	updateServiceGenerateDomain = 5678

	cmd := updateServiceCmd
	err := cmd.RunE(cmd, []string{"api"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !domainCreated {
		t.Error("expected CreateServiceDomain to be called")
	}
}

func TestRunCreateService_WithGenerateTCP(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origImage := serviceImage
	origGenerateTCP := createServiceGenerateTCP
	origGenerateDomain := createServiceGenerateDomain
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		serviceImage = origImage
		createServiceGenerateTCP = origGenerateTCP
		createServiceGenerateDomain = origGenerateDomain
		createServiceCmd.Flags().Lookup("generate-tcp").Changed = false
	}()

	var tcpProxyCreated bool
	var capturedPort int

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			CreateServiceFunc: func(projectID, environmentID, name, image string, creds *api.RegistryCredentials) (types.Service, error) {
				return types.Service{ID: "svc-new", Name: name}, nil
			},
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return nil, nil // No existing proxies
			},
			CreateTCPProxyFunc: func(applicationPort int, environmentID, serviceID string) (api.TCPProxy, error) {
				tcpProxyCreated = true
				capturedPort = applicationPort
				return api.TCPProxy{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 12345, ApplicationPort: applicationPort}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	serviceImage = "postgres:16"
	createServiceGenerateTCP = 5432
	createServiceGenerateDomain = 0

	// Reset deploy config flags from previous tests
	createServiceCmd.Flags().Lookup("max-retries").Changed = false
	createServiceCmd.Flags().Lookup("restart-policy").Changed = false
	createServiceCmd.Flags().Lookup("replicas").Changed = false
	createServiceCmd.Flags().Lookup("healthcheck-path").Changed = false
	createServiceCmd.Flags().Lookup("healthcheck-timeout").Changed = false

	cmd := createServiceCmd
	err := cmd.RunE(cmd, []string{"my-db"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !tcpProxyCreated {
		t.Error("expected CreateTCPProxy to be called")
	}
	if capturedPort != 5432 {
		t.Errorf("expected port 5432, got %d", capturedPort)
	}
}

func TestRunUpdateService_WithGenerateTCP(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origImage := updateServiceImage
	origGenerateTCP := updateServiceGenerateTCP
	origGenerateDomain := updateServiceGenerateDomain
	origRemoveDomain := updateServiceRemoveDomain
	origRemoveTCP := updateServiceRemoveTCP
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		updateServiceImage = origImage
		updateServiceGenerateTCP = origGenerateTCP
		updateServiceGenerateDomain = origGenerateDomain
		updateServiceRemoveDomain = origRemoveDomain
		updateServiceRemoveTCP = origRemoveTCP
	}()

	var tcpProxyCreated bool

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{
					{ID: "svc-1", Name: "db"},
				}, nil
			},
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return nil, nil
			},
			CreateTCPProxyFunc: func(applicationPort int, environmentID, serviceID string) (api.TCPProxy, error) {
				tcpProxyCreated = true
				return api.TCPProxy{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 12345, ApplicationPort: applicationPort}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	updateServiceImage = "" // No image update -- generate-tcp only
	updateServiceGenerateTCP = 5432
	updateServiceGenerateDomain = 0
	updateServiceRemoveDomain = false
	updateServiceRemoveTCP = false

	cmd := updateServiceCmd
	err := cmd.RunE(cmd, []string{"db"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !tcpProxyCreated {
		t.Error("expected CreateTCPProxy to be called")
	}
}

func TestRunUpdateService_WithRemoveDomain(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origImage := updateServiceImage
	origGenerateTCP := updateServiceGenerateTCP
	origGenerateDomain := updateServiceGenerateDomain
	origRemoveDomain := updateServiceRemoveDomain
	origRemoveTCP := updateServiceRemoveTCP
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		updateServiceImage = origImage
		updateServiceGenerateTCP = origGenerateTCP
		updateServiceGenerateDomain = origGenerateDomain
		updateServiceRemoveDomain = origRemoveDomain
		updateServiceRemoveTCP = origRemoveTCP
	}()

	var deletedCustomDomainID string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{CustomDomains: []api.CustomDomain{{ID: "cdom-1", Domain: "api.example.com"}}}, nil
			},
			DeleteCustomDomainFunc: func(id string) error {
				deletedCustomDomainID = id
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	updateServiceImage = ""
	updateServiceGenerateTCP = 0
	updateServiceGenerateDomain = 0
	updateServiceRemoveDomain = true
	updateServiceRemoveTCP = false

	cmd := updateServiceCmd
	err := cmd.RunE(cmd, []string{"api"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deletedCustomDomainID != "cdom-1" {
		t.Errorf("expected custom domain to be deleted, got %q", deletedCustomDomainID)
	}
}

func TestRunUpdateService_WithRemoveTCP(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origImage := updateServiceImage
	origGenerateTCP := updateServiceGenerateTCP
	origGenerateDomain := updateServiceGenerateDomain
	origRemoveDomain := updateServiceRemoveDomain
	origRemoveTCP := updateServiceRemoveTCP
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		updateServiceImage = origImage
		updateServiceGenerateTCP = origGenerateTCP
		updateServiceGenerateDomain = origGenerateDomain
		updateServiceRemoveDomain = origRemoveDomain
		updateServiceRemoveTCP = origRemoveTCP
	}()

	var deletedTCPProxyID string

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "db"}}, nil
			},
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return []api.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 12345, ApplicationPort: 5432}}, nil
			},
			DeleteTCPProxyFunc: func(id string) error {
				deletedTCPProxyID = id
				return nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	updateServiceImage = ""
	updateServiceGenerateTCP = 0
	updateServiceGenerateDomain = 0
	updateServiceRemoveDomain = false
	updateServiceRemoveTCP = true

	cmd := updateServiceCmd
	err := cmd.RunE(cmd, []string{"db"})

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deletedTCPProxyID != "tcp-1" {
		t.Errorf("expected tcp proxy to be deleted, got %q", deletedTCPProxyID)
	}
}

func TestRunUpdateService_RejectsConflictingNetworkingFlags(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origImage := updateServiceImage
	origGenerateTCP := updateServiceGenerateTCP
	origGenerateDomain := updateServiceGenerateDomain
	origRemoveDomain := updateServiceRemoveDomain
	origRemoveTCP := updateServiceRemoveTCP
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		updateServiceImage = origImage
		updateServiceGenerateTCP = origGenerateTCP
		updateServiceGenerateDomain = origGenerateDomain
		updateServiceRemoveDomain = origRemoveDomain
		updateServiceRemoveTCP = origRemoveTCP
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	updateServiceImage = ""
	updateServiceGenerateDomain = 3000
	updateServiceRemoveDomain = true
	updateServiceGenerateTCP = 0
	updateServiceRemoveTCP = false

	cmd := updateServiceCmd
	err := cmd.RunE(cmd, []string{"api"})

	if err == nil {
		t.Fatal("expected error for conflicting domain flags")
	}
}

func TestRunUpdateService_RemoveDomainSatisfiesMutationRequirement(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origImage := updateServiceImage
	origGenerateTCP := updateServiceGenerateTCP
	origGenerateDomain := updateServiceGenerateDomain
	origRemoveDomain := updateServiceRemoveDomain
	origRemoveTCP := updateServiceRemoveTCP
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		updateServiceImage = origImage
		updateServiceGenerateTCP = origGenerateTCP
		updateServiceGenerateDomain = origGenerateDomain
		updateServiceRemoveDomain = origRemoveDomain
		updateServiceRemoveTCP = origRemoveTCP
	}()

	token = "test-token"
	newAPIClient = func(tkn string) api.APIClient {
		return &api.MockClient{
			ListProjectsFunc: func() ([]types.Project, error) {
				return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
			},
			ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
				return []types.Environment{{ID: "env-1", Name: "production"}}, nil
			},
			ListServicesFunc: func(projectID, envID string) ([]types.ServiceDetail, error) {
				return []types.ServiceDetail{{ID: "svc-1", Name: "api"}}, nil
			},
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, nil
			},
		}
	}
	project = "my-project"
	environment = "production"
	updateServiceImage = ""
	updateServiceGenerateDomain = 0
	updateServiceGenerateTCP = 0
	updateServiceRemoveDomain = true
	updateServiceRemoveTCP = false

	cmd := updateServiceCmd
	err := cmd.RunE(cmd, []string{"api"})

	if err != nil {
		t.Fatalf("expected remove-domain alone to satisfy mutation requirement, got %v", err)
	}
}
