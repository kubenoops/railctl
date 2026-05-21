package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListDeployments(t *testing.T) {
	tests := []struct {
		name          string
		projectID     string
		environmentID string
		serviceID     string
		limit         int
		response      string
		wantCount     int
		wantErr       bool
	}{
		{
			name:          "list deployments successfully",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			limit:         10,
			response: `{
				"data": {
					"deployments": {
						"edges": [
							{
								"node": {
									"id": "deploy-1",
									"status": "SUCCESS",
									"createdAt": "2024-01-01T00:00:00Z",
									"updatedAt": "2024-01-01T00:05:00Z",
									"creator": {
										"name": "Developer"
									},
									"canRedeploy": true,
									"canRollback": false,
									"meta": {
										"image": "nginx:latest"
									}
								}
							},
							{
								"node": {
									"id": "deploy-2",
									"status": "FAILED",
									"createdAt": "2024-01-02T00:00:00Z",
									"updatedAt": "2024-01-02T00:02:00Z",
									"creator": {
										"name": "CI/CD"
									},
									"canRedeploy": true,
									"canRollback": true,
									"meta": {
										"image": "nginx:broken"
									}
								}
							}
						]
					}
				}
			}`,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:          "empty deployments list",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			limit:         10,
			response: `{
				"data": {
					"deployments": {
						"edges": []
					}
				}
			}`,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:          "list with no limit",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			limit:         0,
			response: `{
				"data": {
					"deployments": {
						"edges": [
							{
								"node": {
									"id": "deploy-1",
									"status": "BUILDING",
									"createdAt": "2024-01-01T00:00:00Z",
									"updatedAt": "2024-01-01T00:01:00Z",
									"creator": {
										"name": "Developer"
									},
									"canRedeploy": false,
									"canRollback": false,
									"meta": {
										"image": ""
									}
								}
							}
						]
					}
				}
			}`,
			wantCount: 1,
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
			deployments, err := client.ListDeployments(tt.projectID, tt.environmentID, tt.serviceID, tt.limit)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListDeployments() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(deployments) != tt.wantCount {
				t.Errorf("ListDeployments() got %d deployments, want %d", len(deployments), tt.wantCount)
			}

			// Verify deployment data when present
			if !tt.wantErr && len(deployments) > 0 {
				if deployments[0].ID == "" {
					t.Error("ListDeployments() deployment missing ID")
				}
				if deployments[0].Status == "" {
					t.Error("ListDeployments() deployment missing Status")
				}
				if deployments[0].CreatedAt.IsZero() {
					t.Error("ListDeployments() deployment missing CreatedAt")
				}
			}
		})
	}
}

func TestRemoveDeployment(t *testing.T) {
	tests := []struct {
		name         string
		deploymentID string
		response     string
		wantErr      bool
	}{
		{
			name:         "remove deployment successfully",
			deploymentID: "deploy-123",
			response: `{
				"data": {
					"deploymentRemove": true
				}
			}`,
			wantErr: false,
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
			err := client.RemoveDeployment(tt.deploymentID)

			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveDeployment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDeploymentsAPIError(t *testing.T) {
	tests := []struct {
		name     string
		response string
		method   func(*Client) error
	}{
		{
			name: "list deployments with API error",
			response: `{
				"errors": [
					{"message": "Invalid project ID"}
				]
			}`,
			method: func(c *Client) error {
				_, err := c.ListDeployments("invalid", "env", "svc", 10)
				return err
			},
		},
		{
			name: "remove deployment with API error",
			response: `{
				"errors": [
					{"message": "Deployment not found"}
				]
			}`,
			method: func(c *Client) error {
				return c.RemoveDeployment("invalid")
			},
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
			err := tt.method(client)

			if err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

func TestGetDeploymentLogs(t *testing.T) {
	tests := []struct {
		name         string
		deploymentID string
		limit        int
		response     string
		wantCount    int
		wantErr      bool
	}{
		{
			name:         "get deployment logs successfully",
			deploymentID: "deploy-123",
			limit:        100,
			response: `{
"data": {
"deploymentLogs": [
{
"timestamp": "2024-01-01T12:00:00Z",
"message": "Starting application"
},
{
"timestamp": "2024-01-01T12:00:01Z",
"message": "Server listening on port 8080"
}
]
}
}`,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:         "empty logs list",
			deploymentID: "deploy-456",
			limit:        50,
			response: `{
"data": {
"deploymentLogs": []
}
}`,
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:         "logs with no limit specified",
			deploymentID: "deploy-789",
			limit:        0,
			response: `{
"data": {
"deploymentLogs": [
{
"timestamp": "2024-01-01T13:00:00Z",
"message": "Log entry 1"
}
]
}
}`,
			wantCount: 1,
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
			logs, err := client.GetDeploymentLogs(tt.deploymentID, tt.limit)

			if (err != nil) != tt.wantErr {
				t.Errorf("GetDeploymentLogs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(logs) != tt.wantCount {
				t.Errorf("GetDeploymentLogs() got %d logs, want %d", len(logs), tt.wantCount)
			}

			// Verify log data when present
			if !tt.wantErr && len(logs) > 0 {
				if logs[0].Timestamp.IsZero() {
					t.Error("GetDeploymentLogs() log missing Timestamp")
				}
				if logs[0].Message == "" {
					t.Error("GetDeploymentLogs() log missing Message")
				}
			}
		})
	}
}

func TestGetDeploymentLogsAPIError(t *testing.T) {
	response := `{
"errors": [
{"message": "Deployment not found"}
]
}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL
	_, err := client.GetDeploymentLogs("invalid-deployment", 100)

	if err == nil {
		t.Error("Expected error but got none")
	}
}
