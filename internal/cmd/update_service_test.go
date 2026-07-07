package cmd

import (
	"errors"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/spf13/cobra"
)

func TestIsPrivateDockerRegistry(t *testing.T) {
	tests := []struct {
		image string
		want  bool
	}{
		{"nginx:latest", false},
		{"node:20", false},
		{"myapp", false},
		{"", false},
		{"gcr.io/my-project/app", true},
		{"ghcr.io/user/repo:v1", true},
		{"registry.example.com/app", true},
		{"localhost:5000/myimage", true},
	}

	for _, tt := range tests {
		t.Run(tt.image, func(t *testing.T) {
			got := isPrivateDockerRegistry(tt.image)
			if got != tt.want {
				t.Errorf("isPrivateDockerRegistry(%q) = %v, want %v", tt.image, got, tt.want)
			}
		})
	}
}

func TestValidateDeployConfigFlags(t *testing.T) {
	t.Run("valid restart policy ON_FAILURE", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("restart-policy", "", "")
		cmd.Flags().Int("max-retries", 0, "")
		cmd.Flags().Int("replicas", 0, "")

		updateServiceRestartPolicy = "on_failure"
		cmd.Flags().Set("restart-policy", "on_failure")

		err := validateDeployConfigFlags(cmd)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if updateServiceRestartPolicy != "ON_FAILURE" {
			t.Errorf("expected uppercase ON_FAILURE, got %q", updateServiceRestartPolicy)
		}
	})

	t.Run("invalid restart policy", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("restart-policy", "", "")
		cmd.Flags().Int("max-retries", 0, "")
		cmd.Flags().Int("replicas", 0, "")

		updateServiceRestartPolicy = "invalid"
		cmd.Flags().Set("restart-policy", "invalid")

		err := validateDeployConfigFlags(cmd)
		if err == nil {
			t.Error("expected error for invalid restart policy")
		}
	})

	t.Run("max-retries without restart-policy", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("restart-policy", "", "")
		cmd.Flags().Int("max-retries", 0, "")
		cmd.Flags().Int("replicas", 0, "")

		updateServiceRestartPolicy = ""
		cmd.Flags().Set("max-retries", "3")

		err := validateDeployConfigFlags(cmd)
		if err == nil {
			t.Error("expected error for max-retries without restart-policy")
		}
	})

	t.Run("replicas less than 1", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("restart-policy", "", "")
		cmd.Flags().Int("max-retries", 0, "")
		cmd.Flags().Int("replicas", 0, "")

		updateServiceRestartPolicy = ""
		updateServiceReplicas = 0
		cmd.Flags().Set("replicas", "0")

		err := validateDeployConfigFlags(cmd)
		if err == nil {
			t.Error("expected error for replicas < 1")
		}
	})

	t.Run("no flags set", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("restart-policy", "", "")
		cmd.Flags().Int("max-retries", 0, "")
		cmd.Flags().Int("replicas", 0, "")

		updateServiceRestartPolicy = ""

		err := validateDeployConfigFlags(cmd)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestApplyUpdateDeployConfig(t *testing.T) {
	t.Run("applies start command", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("start-command", "", "")
		cmd.Flags().String("restart-policy", "", "")
		cmd.Flags().Int("max-retries", 0, "")
		cmd.Flags().Int("replicas", 0, "")
		cmd.Flags().String("healthcheck-path", "", "")
		cmd.Flags().Int("healthcheck-timeout", 0, "")

		updateServiceStartCommand = "npm start"
		cmd.Flags().Set("start-command", "npm start")

		var calledWith *string
		mock := &api.MockClient{
			UpdateServiceInstanceConfigFunc: func(serviceID, environmentID string, startCommand, restartPolicy *string, maxRetries, replicas *int, healthcheckPath *string, healthcheckTimeout *int) error {
				calledWith = startCommand
				return nil
			},
		}

		err := applyUpdateDeployConfig(cmd, mock, "svc-1", "env-1")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if calledWith == nil || *calledWith != "npm start" {
			t.Errorf("expected start command 'npm start', got %v", calledWith)
		}
	})

	t.Run("api error wraps", func(t *testing.T) {
		cmd := &cobra.Command{Use: "test"}
		cmd.Flags().String("start-command", "", "")
		cmd.Flags().String("restart-policy", "", "")
		cmd.Flags().Int("max-retries", 0, "")
		cmd.Flags().Int("replicas", 0, "")
		cmd.Flags().String("healthcheck-path", "", "")
		cmd.Flags().Int("healthcheck-timeout", 0, "")

		updateServiceStartCommand = "npm start"
		cmd.Flags().Set("start-command", "npm start")

		mock := &api.MockClient{
			UpdateServiceInstanceConfigFunc: func(serviceID, environmentID string, startCommand, restartPolicy *string, maxRetries, replicas *int, healthcheckPath *string, healthcheckTimeout *int) error {
				return errors.New("api failure")
			},
		}

		err := applyUpdateDeployConfig(cmd, mock, "svc-1", "env-1")
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestPrintUpdateServiceResult(t *testing.T) {
	// These tests just verify no panic; output goes to stdout
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("start-command", "", "")
	cmd.Flags().String("restart-policy", "", "")
	cmd.Flags().Int("max-retries", 0, "")
	cmd.Flags().Int("replicas", 0, "")
	cmd.Flags().String("healthcheck-path", "", "")
	cmd.Flags().Int("healthcheck-timeout", 0, "")

	t.Run("image with creds", func(t *testing.T) {
		creds := &api.RegistryCredentials{Username: "user", Password: "pass"}
		printUpdateServiceResult(cmd, "web", "nginx:latest", creds, false, "deploy-1")
	})

	t.Run("image only", func(t *testing.T) {
		printUpdateServiceResult(cmd, "web", "nginx:latest", nil, false, "")
	})

	t.Run("creds only", func(t *testing.T) {
		creds := &api.RegistryCredentials{Username: "user", Password: "pass"}
		printUpdateServiceResult(cmd, "web", "", creds, false, "")
	})

	t.Run("deploy config with flags", func(t *testing.T) {
		updateServiceStartCommand = "node app.js"
		cmd.Flags().Set("start-command", "node app.js")
		printUpdateServiceResult(cmd, "web", "", nil, true, "deploy-2")
	})
}

func TestValidateNetworkingMutationFlags(t *testing.T) {
	origGenerateDomain := updateServiceGenerateDomain
	origGenerateTCP := updateServiceGenerateTCP
	origRemoveDomain := updateServiceRemoveDomain
	origRemoveTCP := updateServiceRemoveTCP
	defer func() {
		updateServiceGenerateDomain = origGenerateDomain
		updateServiceGenerateTCP = origGenerateTCP
		updateServiceRemoveDomain = origRemoveDomain
		updateServiceRemoveTCP = origRemoveTCP
	}()

	t.Run("rejects generate and remove domain together", func(t *testing.T) {
		updateServiceGenerateDomain = 3000
		updateServiceRemoveDomain = true
		updateServiceGenerateTCP = 0
		updateServiceRemoveTCP = false

		err := validateNetworkingMutationFlags()
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("rejects generate and remove tcp together", func(t *testing.T) {
		updateServiceGenerateDomain = 0
		updateServiceRemoveDomain = false
		updateServiceGenerateTCP = 5432
		updateServiceRemoveTCP = true

		err := validateNetworkingMutationFlags()
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("accepts independent mutations", func(t *testing.T) {
		updateServiceGenerateDomain = 0
		updateServiceRemoveDomain = true
		updateServiceGenerateTCP = 0
		updateServiceRemoveTCP = false

		err := validateNetworkingMutationFlags()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRemoveServiceDomain(t *testing.T) {
	t.Run("removes custom domain first", func(t *testing.T) {
		var deletedCustomID string
		var deletedServiceID string
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{
					CustomDomains:  []api.CustomDomain{{ID: "cdom-1", Domain: "api.example.com"}},
					ServiceDomains: []api.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}},
				}, nil
			},
			DeleteCustomDomainFunc: func(id string) error {
				deletedCustomID = id
				return nil
			},
			DeleteServiceDomainFunc: func(id string) error {
				deletedServiceID = id
				return nil
			},
		}

		err := removeServiceDomain(mock, "proj-1", "env-1", "svc-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if deletedCustomID != "cdom-1" {
			t.Fatalf("expected custom domain delete, got %q", deletedCustomID)
		}
		if deletedServiceID != "" {
			t.Fatalf("expected service domain not to be deleted, got %q", deletedServiceID)
		}
	})

	t.Run("removes service domain when no custom domain exists", func(t *testing.T) {
		var deletedServiceID string
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{ServiceDomains: []api.ServiceDomain{{ID: "dom-1", Domain: "api.up.railway.app"}}}, nil
			},
			DeleteServiceDomainFunc: func(id string) error {
				deletedServiceID = id
				return nil
			},
		}

		err := removeServiceDomain(mock, "proj-1", "env-1", "svc-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if deletedServiceID != "dom-1" {
			t.Fatalf("expected service domain delete, got %q", deletedServiceID)
		}
	})

	t.Run("is noop when no domains exist", func(t *testing.T) {
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, nil
			},
		}

		if err := removeServiceDomain(mock, "proj-1", "env-1", "svc-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestRemoveTCPProxy(t *testing.T) {
	t.Run("removes first tcp proxy", func(t *testing.T) {
		var deletedID string
		mock := &api.MockClient{
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return []api.TCPProxy{{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 44321, ApplicationPort: 5432}}, nil
			},
			DeleteTCPProxyFunc: func(id string) error {
				deletedID = id
				return nil
			},
		}

		err := removeTCPProxy(mock, "env-1", "svc-1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if deletedID != "tcp-1" {
			t.Fatalf("expected tcp proxy delete, got %q", deletedID)
		}
	})

	t.Run("is noop when no tcp proxies exist", func(t *testing.T) {
		mock := &api.MockClient{
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return nil, nil
			},
		}

		if err := removeTCPProxy(mock, "env-1", "svc-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestGenerateServiceDomain(t *testing.T) {
	t.Run("creates domain when none exists and sets requested port", func(t *testing.T) {
		var createCalled bool
		var updateCalled bool
		var createdPort int
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, nil
			},
			CreateServiceDomainFunc: func(serviceID, environmentID string, targetPort int) (api.ServiceDomain, error) {
				createCalled = true
				createdPort = targetPort
				return api.ServiceDomain{ID: "dom-1", Domain: "myapp-production.up.railway.app"}, nil
			},
			UpdateServiceDomainPortFunc: func(serviceDomainID, domain, environmentID, serviceID string, port int) error {
				updateCalled = true
				return nil
			},
		}

		err := generateServiceDomain(mock, "proj-1", "env-1", "svc-1", 5678)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !createCalled {
			t.Error("expected CreateServiceDomain to be called")
		}
		// Port is set at creation — no separate update.
		if updateCalled {
			t.Error("expected UpdateServiceDomainPort NOT to be called (port set at creation)")
		}
		if createdPort != 5678 {
			t.Errorf("expected CreateServiceDomain to receive port 5678, got %d", createdPort)
		}
	})

	t.Run("returns error when creating domain fails", func(t *testing.T) {
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, nil
			},
			CreateServiceDomainFunc: func(serviceID, environmentID string, targetPort int) (api.ServiceDomain, error) {
				return api.ServiceDomain{}, errors.New("api failure")
			},
		}

		err := generateServiceDomain(mock, "proj-1", "env-1", "svc-1", 5678)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("creates domain when none exists without setting port when port is zero", func(t *testing.T) {
		var createCalled bool
		var updateCalled bool
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, nil
			},
			CreateServiceDomainFunc: func(serviceID, environmentID string, targetPort int) (api.ServiceDomain, error) {
				createCalled = true
				return api.ServiceDomain{ID: "dom-1", Domain: "myapp-production.up.railway.app"}, nil
			},
			UpdateServiceDomainPortFunc: func(serviceDomainID, domain, environmentID, serviceID string, port int) error {
				updateCalled = true
				return nil
			},
		}

		err := generateServiceDomain(mock, "proj-1", "env-1", "svc-1", 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !createCalled {
			t.Error("expected CreateServiceDomain to be called")
		}
		if updateCalled {
			t.Error("expected UpdateServiceDomainPort NOT to be called when port is zero")
		}
	})

	t.Run("skips creation when service domain already exists", func(t *testing.T) {
		var createCalled bool
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{
					ServiceDomains: []api.ServiceDomain{
						{ID: "dom-1", Domain: "existing.up.railway.app"},
					},
				}, nil
			},
			CreateServiceDomainFunc: func(serviceID, environmentID string, targetPort int) (api.ServiceDomain, error) {
				createCalled = true
				return api.ServiceDomain{}, nil
			},
		}

		err := generateServiceDomain(mock, "proj-1", "env-1", "svc-1", 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if createCalled {
			t.Error("expected CreateServiceDomain NOT to be called when domain already exists")
		}
	})

	t.Run("skips creation when custom domain exists", func(t *testing.T) {
		var createCalled bool
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{
					CustomDomains: []api.CustomDomain{
						{ID: "cdom-1", Domain: "app.example.com"},
					},
				}, nil
			},
			CreateServiceDomainFunc: func(serviceID, environmentID string, targetPort int) (api.ServiceDomain, error) {
				createCalled = true
				return api.ServiceDomain{}, nil
			},
		}

		err := generateServiceDomain(mock, "proj-1", "env-1", "svc-1", 0)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if createCalled {
			t.Error("expected CreateServiceDomain NOT to be called when custom domain exists")
		}
	})

	t.Run("auto-updates service domain port on mismatch", func(t *testing.T) {
		oldPort := 8080
		var updateCalled bool
		var updatedPort int
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{
					ServiceDomains: []api.ServiceDomain{
						{ID: "dom-1", Domain: "existing.up.railway.app", TargetPort: &oldPort},
					},
				}, nil
			},
			UpdateServiceDomainPortFunc: func(serviceDomainID, domain, environmentID, serviceID string, port int) error {
				updateCalled = true
				updatedPort = port
				return nil
			},
		}

		err := generateServiceDomain(mock, "proj-1", "env-1", "svc-1", 5678)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !updateCalled {
			t.Error("expected UpdateServiceDomainPort to be called")
		}
		if updatedPort != 5678 {
			t.Errorf("expected port 5678, got %d", updatedPort)
		}
	})

	t.Run("auto-updates custom domain port on mismatch", func(t *testing.T) {
		oldPort := 8080
		var updateCalled bool
		var updatedPort int
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{
					CustomDomains: []api.CustomDomain{
						{ID: "cdom-1", Domain: "app.example.com", TargetPort: &oldPort},
					},
				}, nil
			},
			UpdateCustomDomainPortFunc: func(customDomainID, environmentID string, port int) error {
				updateCalled = true
				updatedPort = port
				return nil
			},
		}

		err := generateServiceDomain(mock, "proj-1", "env-1", "svc-1", 5678)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !updateCalled {
			t.Error("expected UpdateCustomDomainPort to be called")
		}
		if updatedPort != 5678 {
			t.Errorf("expected port 5678, got %d", updatedPort)
		}
	})

	t.Run("returns error when list fails", func(t *testing.T) {
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, errors.New("api failure")
			},
		}

		err := generateServiceDomain(mock, "proj-1", "env-1", "svc-1", 0)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("returns error when create fails", func(t *testing.T) {
		mock := &api.MockClient{
			ListDomainsFunc: func(projectID, environmentID, serviceID string) (api.DomainList, error) {
				return api.DomainList{}, nil
			},
			CreateServiceDomainFunc: func(serviceID, environmentID string, targetPort int) (api.ServiceDomain, error) {
				return api.ServiceDomain{}, errors.New("api failure")
			},
		}

		err := generateServiceDomain(mock, "proj-1", "env-1", "svc-1", 0)
		if err == nil {
			t.Error("expected error")
		}
	})
}

func TestGenerateTCPProxy(t *testing.T) {
	t.Run("creates proxy when none exists", func(t *testing.T) {
		var createCalled bool
		mock := &api.MockClient{
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return nil, nil
			},
			CreateTCPProxyFunc: func(applicationPort int, environmentID, serviceID string) (api.TCPProxy, error) {
				createCalled = true
				if applicationPort != 5432 {
					t.Errorf("expected port 5432, got %d", applicationPort)
				}
				return api.TCPProxy{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 12345, ApplicationPort: 5432}, nil
			},
		}

		err := generateTCPProxy(mock, "env-1", "svc-1", 5432)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !createCalled {
			t.Error("expected CreateTCPProxy to be called")
		}
	})

	t.Run("skips creation when proxy for same port already exists", func(t *testing.T) {
		var createCalled bool
		mock := &api.MockClient{
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return []api.TCPProxy{
					{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 12345, ApplicationPort: 5432},
				}, nil
			},
			CreateTCPProxyFunc: func(applicationPort int, environmentID, serviceID string) (api.TCPProxy, error) {
				createCalled = true
				return api.TCPProxy{}, nil
			},
		}

		err := generateTCPProxy(mock, "env-1", "svc-1", 5432)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if createCalled {
			t.Error("expected CreateTCPProxy NOT to be called when proxy already exists")
		}
	})

	t.Run("creates proxy for different port even if other proxy exists", func(t *testing.T) {
		var createCalled bool
		mock := &api.MockClient{
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return []api.TCPProxy{
					{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 12345, ApplicationPort: 5432},
				}, nil
			},
			CreateTCPProxyFunc: func(applicationPort int, environmentID, serviceID string) (api.TCPProxy, error) {
				createCalled = true
				return api.TCPProxy{ID: "tcp-2", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 12346, ApplicationPort: 6379}, nil
			},
		}

		err := generateTCPProxy(mock, "env-1", "svc-1", 6379)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !createCalled {
			t.Error("expected CreateTCPProxy to be called for a different port")
		}
	})

	t.Run("returns error when list fails", func(t *testing.T) {
		mock := &api.MockClient{
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return nil, errors.New("api failure")
			},
		}

		err := generateTCPProxy(mock, "env-1", "svc-1", 5432)
		if err == nil {
			t.Error("expected error")
		}
	})

	t.Run("returns error when create fails", func(t *testing.T) {
		mock := &api.MockClient{
			ListTCPProxiesFunc: func(environmentID, serviceID string) ([]api.TCPProxy, error) {
				return nil, nil
			},
			CreateTCPProxyFunc: func(applicationPort int, environmentID, serviceID string) (api.TCPProxy, error) {
				return api.TCPProxy{}, errors.New("api failure")
			},
		}

		err := generateTCPProxy(mock, "env-1", "svc-1", 5432)
		if err == nil {
			t.Error("expected error")
		}
	})
}
