package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kubenoops/railctl/internal/types"
)

func TestListServices(t *testing.T) {
	tests := []struct {
		name          string
		projectID     string
		environmentID string
		response      string
		wantCount     int
		wantErr       bool
	}{
		{
			name:          "list services successfully",
			projectID:     "proj-123",
			environmentID: "env-456",
			response: `{
				"data": {
					"project": {
						"services": {
							"edges": [
								{
									"node": {
										"id": "service-1",
										"name": "web",
										"icon": "🌐",
										"updatedAt": "2024-01-01T00:00:00Z",
										"serviceInstances": {
											"edges": [
												{
													"node": {
														"id": "instance-1",
														"environmentId": "env-456",
														"startCommand": "npm start",
														"source": {
															"image": "nginx:latest",
															"repo": ""
														},
														"latestDeployment": {
															"id": "deploy-1",
															"status": "SUCCESS",
															"createdAt": "2024-01-01T00:00:00Z",
															"meta": {},
															"deploymentStopped": false
														},
														"activeDeployments": [
															{"id": "deploy-1", "status": "SUCCESS"}
														]
													}
												}
											]
										}
									}
								}
							]
						}
					}
				}
			}`,
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:          "empty services list",
			projectID:     "proj-123",
			environmentID: "env-456",
			response: `{
				"data": {
					"project": {
						"services": {
							"edges": []
						}
					}
				}
			}`,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:          "filter by environment",
			projectID:     "proj-123",
			environmentID: "env-456",
			response: `{
				"data": {
					"project": {
						"services": {
							"edges": [
								{
									"node": {
										"id": "service-1",
										"name": "web",
										"icon": "🌐",
										"updatedAt": "2024-01-01T00:00:00Z",
										"serviceInstances": {
											"edges": [
												{
													"node": {
														"id": "instance-1",
														"environmentId": "env-different",
														"startCommand": "npm start",
														"source": {"image": "nginx:latest", "repo": ""},
														"latestDeployment": null,
														"activeDeployments": []
													}
												}
											]
										}
									}
								}
							]
						}
					}
				}
			}`,
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := NewClient("test-token")
			client.apiURL = server.URL
			services, err := client.ListServices(tt.projectID, tt.environmentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListServices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(services) != tt.wantCount {
				t.Errorf("ListServices() got %d services, want %d", len(services), tt.wantCount)
			}
		})
	}
}

func TestGetService(t *testing.T) {
	tests := []struct {
		name      string
		serviceID string
		response  string
		wantName  string
		wantErr   bool
	}{
		{
			name:      "get service successfully",
			serviceID: "service-123",
			response: `{
				"data": {
					"service": {
						"id": "service-123",
						"name": "web-service",
						"icon": "🚀",
						"updatedAt": "2024-01-01T00:00:00Z",
						"serviceInstances": {
							"edges": [
								{
									"node": {
										"id": "instance-1",
										"environmentId": "env-456",
										"startCommand": "npm start",
										"source": {
											"image": "node:18",
											"repo": ""
										},
										"latestDeployment": {
											"id": "deploy-1",
											"status": "SUCCESS",
											"createdAt": "2024-01-01T00:00:00Z",
											"meta": {},
											"deploymentStopped": false
										},
										"activeDeployments": []
									}
								}
							]
						}
					}
				}
			}`,
			wantName: "web-service",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := NewClient("test-token")
			client.apiURL = server.URL
			service, err := client.GetService(tt.serviceID)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && service.Name != tt.wantName {
				t.Errorf("GetService() name = %v, want %v", service.Name, tt.wantName)
			}
		})
	}
}

func TestCreateService(t *testing.T) {
	tests := []struct {
		name      string
		projectID string
		svcName   string
		image     string
		creds     *RegistryCredentials
		response  string
		wantName  string
		wantErr   bool
	}{
		{
			name:      "create service with image",
			projectID: "proj-123",
			svcName:   "web",
			image:     "nginx:latest",
			creds:     nil,
			response: `{
				"data": {
					"serviceCreate": {
						"id": "service-new",
						"name": "web"
					}
				}
			}`,
			wantName: "web",
			wantErr:  false,
		},
		{
			name:      "create service with registry credentials",
			projectID: "proj-123",
			svcName:   "private-app",
			image:     "registry.com/app:latest",
			creds: &RegistryCredentials{
				Username: "user",
				Password: "pass",
			},
			response: `{
				"data": {
					"serviceCreate": {
						"id": "service-private",
						"name": "private-app"
					}
				}
			}`,
			wantName: "private-app",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				// Verify credentials are included when provided
				if tt.creds != nil {
					var req struct {
						Variables map[string]interface{} `json:"variables"`
					}
					json.NewDecoder(r.Body).Decode(&req)
					input := req.Variables["input"].(map[string]interface{})
					if _, ok := input["registryCredentials"]; !ok {
						t.Error("Expected registryCredentials in input")
					}
				}

				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := NewClient("test-token")
			client.apiURL = server.URL
			service, err := client.CreateService(tt.projectID, "test-env-id", tt.svcName, tt.image, tt.creds)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateService() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && service.Name != tt.wantName {
				t.Errorf("CreateService() name = %v, want %v", service.Name, tt.wantName)
			}
		})
	}
}

func TestDeleteService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"serviceDelete": true}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL
	err := client.DeleteService("service-123")

	if err != nil {
		t.Errorf("DeleteService() error = %v", err)
	}
}

func TestUpdateServiceInstance(t *testing.T) {
	tests := []struct {
		name          string
		serviceID     string
		environmentID string
		image         string
		creds         *RegistryCredentials
		wantErr       bool
	}{
		{
			name:          "update image only",
			serviceID:     "service-123",
			environmentID: "env-456",
			image:         "nginx:alpine",
			creds:         nil,
			wantErr:       false,
		},
		{
			name:          "update with credentials",
			serviceID:     "service-123",
			environmentID: "env-456",
			image:         "private.registry/app:v2",
			creds: &RegistryCredentials{
				Username: "user",
				Password: "pass",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"data": {"serviceInstanceUpdate": true}}`))
			}))
			defer server.Close()

			client := NewClient("test-token")
			client.apiURL = server.URL
			err := client.UpdateServiceInstance(tt.serviceID, tt.environmentID, tt.image, tt.creds)

			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateServiceInstance() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// regCredsFromInput extracts input.registryCredentials from a captured GraphQL
// request's variables.input, or nil if absent.
func regCredsFromInput(input map[string]any) map[string]any {
	rc, ok := input["registryCredentials"].(map[string]any)
	if !ok {
		return nil
	}
	return rc
}

// captureInputServer returns a test server that records the GraphQL request's
// variables.input into *got and replies with resp.
func captureInputServer(got *map[string]any, resp string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				Input map[string]any `json:"input"`
			} `json:"variables"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		*got = req.Variables.Input
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(resp))
	}))
}

func TestCreateService_SendsRegistryCredentials(t *testing.T) {
	var input map[string]any
	server := captureInputServer(&input, `{"data":{"serviceCreate":{"id":"svc-1","name":"api"}}}`)
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL
	if _, err := client.CreateService("proj-1", "env-1", "api", "ghcr.io/acme/api:v1",
		&RegistryCredentials{Username: "acme-bot", Password: "ghp_token"}); err != nil {
		t.Fatalf("CreateService: %v", err)
	}

	rc := regCredsFromInput(input)
	if rc == nil {
		t.Fatal("expected registryCredentials in serviceCreate input")
	}
	if rc["username"] != "acme-bot" || rc["password"] != "ghp_token" {
		t.Errorf("registryCredentials mismatch: got %v", rc)
	}
}

func TestCreateService_OmitsRegistryCredentialsWhenNil(t *testing.T) {
	var input map[string]any
	server := captureInputServer(&input, `{"data":{"serviceCreate":{"id":"svc-1","name":"pg"}}}`)
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL
	if _, err := client.CreateService("proj-1", "env-1", "pg", "postgres:16", nil); err != nil {
		t.Fatalf("CreateService: %v", err)
	}
	if rc := regCredsFromInput(input); rc != nil {
		t.Errorf("did not expect registryCredentials for nil creds, got %v", rc)
	}
}

func TestUpdateServiceInstance_SendsRegistryCredentials(t *testing.T) {
	var input map[string]any
	server := captureInputServer(&input, `{"data":{"serviceInstanceUpdate":true}}`)
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL
	if err := client.UpdateServiceInstance("svc-1", "env-1", "ghcr.io/acme/api:v2",
		&RegistryCredentials{Username: "acme-bot", Password: "ghp_token"}); err != nil {
		t.Fatalf("UpdateServiceInstance: %v", err)
	}

	rc := regCredsFromInput(input)
	if rc == nil {
		t.Fatal("expected registryCredentials in serviceInstanceUpdate input")
	}
	if rc["username"] != "acme-bot" || rc["password"] != "ghp_token" {
		t.Errorf("registryCredentials mismatch: got %v", rc)
	}
}

func TestUpdateServiceInstance_OmitsRegistryCredentialsWhenNil(t *testing.T) {
	var input map[string]any
	server := captureInputServer(&input, `{"data":{"serviceInstanceUpdate":true}}`)
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL
	if err := client.UpdateServiceInstance("svc-1", "env-1", "nginx:alpine", nil); err != nil {
		t.Fatalf("UpdateServiceInstance: %v", err)
	}
	if rc := regCredsFromInput(input); rc != nil {
		t.Errorf("did not expect registryCredentials for nil creds, got %v", rc)
	}
}

func TestDeployServiceInstance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"serviceInstanceDeployV2": "deployment-new-123"}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL
	deploymentID, err := client.DeployServiceInstance("service-123", "env-456")

	if err != nil {
		t.Errorf("DeployServiceInstance() error = %v", err)
		return
	}

	if deploymentID != "deployment-new-123" {
		t.Errorf("DeployServiceInstance() got %v, want %v", deploymentID, "deployment-new-123")
	}
}

func TestRedeployDeployment(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"deploymentRedeploy": {"id": "deployment-123"}}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL
	err := client.RedeployDeployment("deployment-123")

	if err != nil {
		t.Errorf("RedeployDeployment() error = %v", err)
	}
}

func TestGetBuildLogs(t *testing.T) {
	tests := []struct {
		name         string
		deploymentID string
		limit        int
		response     string
		wantLogCount int
		wantErr      bool
	}{
		{
			name:         "get build logs successfully",
			deploymentID: "deploy-123",
			limit:        100,
			response: `{
				"data": {
					"buildLogs": [
						{"message": "Building..."},
						{"message": "Build complete"},
						{"message": ""}
					]
				}
			}`,
			wantLogCount: 2,
			wantErr:      false,
		},
		{
			name:         "empty logs",
			deploymentID: "deploy-456",
			limit:        100,
			response: `{
				"data": {
					"buildLogs": []
				}
			}`,
			wantLogCount: 0,
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := NewClient("test-token")
			client.apiURL = server.URL
			logs, err := client.GetBuildLogs(tt.deploymentID, tt.limit)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetBuildLogs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(logs) != tt.wantLogCount {
				t.Errorf("GetBuildLogs() got %d logs, want %d", len(logs), tt.wantLogCount)
			}
		})
	}
}

func TestExtractErrorFromLogs(t *testing.T) {
	tests := []struct {
		name    string
		logs    []string
		wantErr string
	}{
		{
			name: "extract error message",
			logs: []string{
				"=========================",
				"Container failed to start",
				"Error: Port 3000 already in use",
			},
			wantErr: "Error: Port 3000 already in use",
		},
		{
			name: "skip decorative lines",
			logs: []string{
				"",
				"=========================",
				"Build failed",
				"npm ERR! Missing dependency",
			},
			wantErr: "npm ERR! Missing dependency",
		},
		{
			name:    "no error found",
			logs:    []string{"=========================", "Container failed to start"},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractErrorFromLogs(tt.logs)
			if got != tt.wantErr {
				t.Errorf("ExtractErrorFromLogs() = %v, want %v", got, tt.wantErr)
			}
		})
	}
}

func TestServiceNodeToServiceDetail(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name  string
		node  serviceNode
		envID string
		want  types.ServiceDetail
	}{
		{
			name: "service with image source and deploy config",
			node: serviceNode{
				ID:        "service-1",
				Name:      "web",
				Icon:      "🌐",
				UpdatedAt: now,
				ServiceInstances: serviceInstances{Edges: []serviceInstanceEdge{{Node: serviceInstanceNode{
					ID:                      "instance-1",
					EnvironmentID:           "env-1",
					StartCommand:            "npm start",
					RestartPolicyType:       "ON_FAILURE",
					RestartPolicyMaxRetries: 10,
					NumReplicas:             2,
					HealthcheckPath:         "/health",
					HealthcheckTimeout:      30,
					Source:                  serviceInstanceSource{Image: "nginx:latest"},
					LatestDeployment: &serviceInstanceDeployment{
						ID: "deploy-1", Status: "SUCCESS", CreatedAt: now, Meta: map[string]any{},
					},
					ActiveDeployments: []serviceInstanceDeployment{{ID: "deploy-1", Status: "SUCCESS"}},
				}}}},
			},
			envID: "env-1",
			want: types.ServiceDetail{
				ID:                 "service-1",
				Name:               "web",
				Icon:               "🌐",
				UpdatedAt:          now,
				InstanceID:         "instance-1",
				StartCommand:       "npm start",
				RestartPolicy:      "ON_FAILURE",
				MaxRetries:         10,
				Replicas:           2,
				HealthcheckPath:    "/health",
				HealthcheckTimeout: 30,
				Source:             "nginx:latest",
				SourceType:         "image",
				Status:             "SUCCESS",
				DeploymentID:       "deploy-1",
				DeployedAt:         now,
			},
		},
		{
			name: "service with repo source",
			node: serviceNode{
				ID:        "service-2",
				Name:      "api",
				Icon:      "⚙️",
				UpdatedAt: now,
				ServiceInstances: serviceInstances{Edges: []serviceInstanceEdge{{Node: serviceInstanceNode{
					ID:            "instance-2",
					EnvironmentID: "env-1",
					StartCommand:  "go run main.go",
					Source:        serviceInstanceSource{Repo: "github.com/user/repo"},
				}}}},
			},
			envID: "env-1",
			want: types.ServiceDetail{
				ID:           "service-2",
				Name:         "api",
				Icon:         "⚙️",
				UpdatedAt:    now,
				InstanceID:   "instance-2",
				StartCommand: "go run main.go",
				Source:       "github.com/user/repo",
				SourceType:   "repo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.node.toServiceDetail(tt.envID)
			if got.ID != tt.want.ID {
				t.Errorf("ID = %v, want %v", got.ID, tt.want.ID)
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %v, want %v", got.Name, tt.want.Name)
			}
			if got.Source != tt.want.Source {
				t.Errorf("Source = %v, want %v", got.Source, tt.want.Source)
			}
			if got.SourceType != tt.want.SourceType {
				t.Errorf("SourceType = %v, want %v", got.SourceType, tt.want.SourceType)
			}
			if got.RestartPolicy != tt.want.RestartPolicy {
				t.Errorf("RestartPolicy = %v, want %v", got.RestartPolicy, tt.want.RestartPolicy)
			}
			if got.MaxRetries != tt.want.MaxRetries {
				t.Errorf("MaxRetries = %v, want %v", got.MaxRetries, tt.want.MaxRetries)
			}
			if got.Replicas != tt.want.Replicas {
				t.Errorf("Replicas = %v, want %v", got.Replicas, tt.want.Replicas)
			}
			if got.HealthcheckPath != tt.want.HealthcheckPath {
				t.Errorf("HealthcheckPath = %v, want %v", got.HealthcheckPath, tt.want.HealthcheckPath)
			}
			if got.HealthcheckTimeout != tt.want.HealthcheckTimeout {
				t.Errorf("HealthcheckTimeout = %v, want %v", got.HealthcheckTimeout, tt.want.HealthcheckTimeout)
			}
		})
	}
}

func TestDeleteServiceInstance(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": {"serviceDelete": true}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL
	err := client.DeleteServiceInstance("service-123", "env-456")

	if err != nil {
		t.Errorf("DeleteServiceInstance() error = %v", err)
	}
}

func TestUpdateServiceInstanceConfig(t *testing.T) {
	t.Run("with all fields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": {"serviceInstanceUpdate": true}}`))
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.apiURL = server.URL

		cmd := "npm start"
		policy := "ON_FAILURE"
		retries := 3
		replicas := 2
		hcPath := "/health"
		hcTimeout := 30

		err := client.UpdateServiceInstanceConfig("svc-1", "env-1", &cmd, &policy, &retries, &replicas, &hcPath, &hcTimeout)
		if err != nil {
			t.Errorf("UpdateServiceInstanceConfig() error = %v", err)
		}
	})

	t.Run("with no fields is noop", func(t *testing.T) {
		// Should not make any API call
		err := NewClient("test-token").UpdateServiceInstanceConfig("svc-1", "env-1", nil, nil, nil, nil, nil, nil)
		if err != nil {
			t.Errorf("UpdateServiceInstanceConfig() error = %v", err)
		}
	})

	t.Run("with partial fields", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": {"serviceInstanceUpdate": true}}`))
		}))
		defer server.Close()

		client := NewClient("test-token")
		client.apiURL = server.URL

		cmd := "python app.py"
		err := client.UpdateServiceInstanceConfig("svc-1", "env-1", &cmd, nil, nil, nil, nil, nil)
		if err != nil {
			t.Errorf("UpdateServiceInstanceConfig() error = %v", err)
		}
	})
}
