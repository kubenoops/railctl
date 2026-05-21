package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetVariables(t *testing.T) {
	tests := []struct {
		name          string
		projectID     string
		environmentID string
		serviceID     string
		response      string
		expectedVars  map[string]string
		expectedError bool
	}{
		{
			name:          "successful retrieval with variables",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			response: `{
				"data": {
					"variablesForServiceDeployment": {
						"DATABASE_URL": "postgres://localhost:5432/db",
						"API_KEY": "secret123",
						"DEBUG": "true"
					}
				}
			}`,
			expectedVars: map[string]string{
				"DATABASE_URL": "postgres://localhost:5432/db",
				"API_KEY":      "secret123",
				"DEBUG":        "true",
			},
			expectedError: false,
		},
		{
			name:          "empty variables",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			response: `{
				"data": {
					"variablesForServiceDeployment": null
				}
			}`,
			expectedVars:  map[string]string{},
			expectedError: false,
		},
		{
			name:          "API error",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			response: `{
				"errors": [{"message": "Service not found"}]
			}`,
			expectedVars:  nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := &Client{
				token:      "test-token",
				apiURL:     server.URL,
				httpClient: server.Client(),
			}

			vars, err := client.GetVariables(tt.projectID, tt.environmentID, tt.serviceID)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(vars) != len(tt.expectedVars) {
				t.Errorf("expected %d variables, got %d", len(tt.expectedVars), len(vars))
			}

			for key, expectedValue := range tt.expectedVars {
				if actualValue, ok := vars[key]; !ok {
					t.Errorf("expected variable %s not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("variable %s: expected %s, got %s", key, expectedValue, actualValue)
				}
			}
		})
	}
}

func TestSetVariables(t *testing.T) {
	tests := []struct {
		name          string
		projectID     string
		environmentID string
		serviceID     string
		variables     map[string]string
		skipDeploys   bool
		response      string
		expectedError bool
	}{
		{
			name:          "successful set with deployment",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			variables: map[string]string{
				"API_KEY": "new-secret",
				"DEBUG":   "false",
			},
			skipDeploys: false,
			response: `{
				"data": {
					"variableCollectionUpsert": true
				}
			}`,
			expectedError: false,
		},
		{
			name:          "successful set with skip deploys",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			variables: map[string]string{
				"FEATURE_FLAG": "enabled",
			},
			skipDeploys: true,
			response: `{
				"data": {
					"variableCollectionUpsert": true
				}
			}`,
			expectedError: false,
		},
		{
			name:          "API returns false",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			variables: map[string]string{
				"TEST": "value",
			},
			skipDeploys: false,
			response: `{
				"data": {
					"variableCollectionUpsert": false
				}
			}`,
			expectedError: true,
		},
		{
			name:          "API error",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			variables: map[string]string{
				"TEST": "value",
			},
			skipDeploys: false,
			response: `{
				"errors": [{"message": "Permission denied"}]
			}`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request body contains skipDeploys if set
				if tt.skipDeploys {
					var reqBody map[string]interface{}
					json.NewDecoder(r.Body).Decode(&reqBody)
					variables := reqBody["variables"].(map[string]interface{})
					if skipDeploys, ok := variables["skipDeploys"]; !ok || skipDeploys != true {
						t.Errorf("expected skipDeploys=true in request")
					}
				}

				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := &Client{
				token:      "test-token",
				apiURL:     server.URL,
				httpClient: server.Client(),
			}

			err := client.SetVariables(tt.projectID, tt.environmentID, tt.serviceID, tt.variables, tt.skipDeploys)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDeleteVariable(t *testing.T) {
	tests := []struct {
		name          string
		projectID     string
		environmentID string
		serviceID     string
		variableName  string
		response      string
		expectedError bool
	}{
		{
			name:          "successful deletion",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			variableName:  "OLD_API_KEY",
			response: `{
				"data": {
					"variableDelete": true
				}
			}`,
			expectedError: false,
		},
		{
			name:          "API returns false",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			variableName:  "NONEXISTENT",
			response: `{
				"data": {
					"variableDelete": false
				}
			}`,
			expectedError: true,
		},
		{
			name:          "API error",
			projectID:     "proj-123",
			environmentID: "env-456",
			serviceID:     "svc-789",
			variableName:  "TEST",
			response: `{
				"errors": [{"message": "Variable not found"}]
			}`,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := &Client{
				token:      "test-token",
				apiURL:     server.URL,
				httpClient: server.Client(),
			}

			err := client.DeleteVariable(tt.projectID, tt.environmentID, tt.serviceID, tt.variableName)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestGetSealedVariables(t *testing.T) {
	tests := []struct {
		name          string
		environmentID string
		serviceID     string
		response      string
		expectedVars  map[string]bool
		expectedError bool
	}{
		{
			name:          "successful retrieval",
			environmentID: "env-456",
			serviceID:     "svc-789",
			response: `{
				"data": {
					"environment": {
						"variables": {
							"edges": [
								{"node": {"name": "DB_URL", "isSealed": true, "serviceId": "svc-789"}},
								{"node": {"name": "DEBUG", "isSealed": false, "serviceId": "svc-789"}},
								{"node": {"name": "GLOBAL", "isSealed": false, "serviceId": ""}}
							]
						}
					}
				}
			}`,
			expectedVars: map[string]bool{
				"DB_URL": true,
				"DEBUG":  false,
				"GLOBAL": false,
			},
			expectedError: false,
		},
		{
			name:          "filters out other services",
			environmentID: "env-456",
			serviceID:     "svc-789",
			response: `{
				"data": {
					"environment": {
						"variables": {
							"edges": [
								{"node": {"name": "MY_VAR", "isSealed": false, "serviceId": "svc-789"}},
								{"node": {"name": "OTHER_VAR", "isSealed": true, "serviceId": "svc-other"}}
							]
						}
					}
				}
			}`,
			expectedVars: map[string]bool{
				"MY_VAR": false,
			},
			expectedError: false,
		},
		{
			name:          "API error",
			environmentID: "env-456",
			serviceID:     "svc-789",
			response:      `{"errors": [{"message":"Environment not found"}]}`,
			expectedVars:  nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := &Client{
				token:      "test-token",
				apiURL:     server.URL,
				httpClient: server.Client(),
			}

			vars, err := client.GetSealedVariables(tt.environmentID, tt.serviceID)

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(vars) != len(tt.expectedVars) {
				t.Errorf("expected %d variables, got %d", len(tt.expectedVars), len(vars))
			}

			for key, expectedSealed := range tt.expectedVars {
				if actual, ok := vars[key]; !ok {
					t.Errorf("expected variable %s not found", key)
				} else if actual != expectedSealed {
					t.Errorf("variable %s: expected sealed=%v, got %v", key, expectedSealed, actual)
				}
			}
		})
	}
}
