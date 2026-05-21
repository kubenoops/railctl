package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListDomains(t *testing.T) {
	client := &MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (DomainList, error) {
			if projectID != "proj-1" || environmentID != "env-1" || serviceID != "svc-1" {
				t.Errorf("unexpected params: projectID=%s, environmentID=%s, serviceID=%s", projectID, environmentID, serviceID)
			}
			return DomainList{
				ServiceDomains: []ServiceDomain{
					{ID: "dom-1", Domain: "myapp-production.up.railway.app"},
				},
			}, nil
		},
	}

	domains, err := client.ListDomains("proj-1", "env-1", "svc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(domains.ServiceDomains) != 1 {
		t.Fatalf("expected 1 service domain, got %d", len(domains.ServiceDomains))
	}

	if domains.ServiceDomains[0].Domain != "myapp-production.up.railway.app" {
		t.Errorf("unexpected domain: %s", domains.ServiceDomains[0].Domain)
	}
}

func TestListDomains_Empty(t *testing.T) {
	client := &MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (DomainList, error) {
			return DomainList{}, nil
		},
	}

	domains, err := client.ListDomains("proj-1", "env-1", "svc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(domains.ServiceDomains) != 0 {
		t.Errorf("expected 0 service domains, got %d", len(domains.ServiceDomains))
	}
	if len(domains.CustomDomains) != 0 {
		t.Errorf("expected 0 custom domains, got %d", len(domains.CustomDomains))
	}
}

func TestListDomains_WithCustomDomains(t *testing.T) {
	client := &MockClient{
		ListDomainsFunc: func(projectID, environmentID, serviceID string) (DomainList, error) {
			port := 5678
			return DomainList{
				ServiceDomains: []ServiceDomain{
					{ID: "dom-1", Domain: "myapp.up.railway.app", TargetPort: &port},
				},
				CustomDomains: []CustomDomain{
					{ID: "cdom-1", Domain: "app.example.com", TargetPort: &port},
				},
			}, nil
		},
	}

	domains, err := client.ListDomains("proj-1", "env-1", "svc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(domains.ServiceDomains) != 1 {
		t.Fatalf("expected 1 service domain, got %d", len(domains.ServiceDomains))
	}
	if len(domains.CustomDomains) != 1 {
		t.Fatalf("expected 1 custom domain, got %d", len(domains.CustomDomains))
	}
	if domains.CustomDomains[0].Domain != "app.example.com" {
		t.Errorf("unexpected custom domain: %s", domains.CustomDomains[0].Domain)
	}
}

func TestCreateServiceDomain(t *testing.T) {
	client := &MockClient{
		CreateServiceDomainFunc: func(serviceID, environmentID string) (ServiceDomain, error) {
			if serviceID != "svc-1" || environmentID != "env-1" {
				t.Errorf("unexpected params: serviceID=%s, environmentID=%s", serviceID, environmentID)
			}
			return ServiceDomain{ID: "dom-1", Domain: "myapp-production.up.railway.app"}, nil
		},
	}

	domain, err := client.CreateServiceDomain("svc-1", "env-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if domain.ID != "dom-1" {
		t.Errorf("unexpected domain ID: %s", domain.ID)
	}
	if domain.Domain != "myapp-production.up.railway.app" {
		t.Errorf("unexpected domain: %s", domain.Domain)
	}
}

func TestServiceDomainJSON(t *testing.T) {
	port := 5678
	original := ServiceDomain{ID: "dom-1", Domain: "myapp.up.railway.app", TargetPort: &port}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled ServiceDomain
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.ID != original.ID {
		t.Errorf("ID mismatch: %s != %s", unmarshaled.ID, original.ID)
	}
	if unmarshaled.Domain != original.Domain {
		t.Errorf("Domain mismatch: %s != %s", unmarshaled.Domain, original.Domain)
	}
	if unmarshaled.TargetPort == nil || *unmarshaled.TargetPort != 5678 {
		t.Errorf("TargetPort mismatch: got %v", unmarshaled.TargetPort)
	}
}

// TestUpdateServiceDomainPort_MutationInput verifies that the GraphQL mutation
// sends only serviceDomainId and targetPort in the input — not serviceId or
// environmentId, which would cause a Railway API error.
func TestUpdateServiceDomainPort_MutationInput(t *testing.T) {
	var capturedInput map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read request body: %v", err)
		}
		defer r.Body.Close()

		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}

		input, ok := req.Variables["input"].(map[string]any)
		if !ok {
			t.Fatal("expected 'input' key in variables")
		}
		capturedInput = input

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"data": map[string]any{
				"serviceDomainUpdate": true,
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	err := client.UpdateServiceDomainPort("dom-abc-123", 8080)
	if err != nil {
		t.Fatalf("UpdateServiceDomainPort returned error: %v", err)
	}

	// Verify expected keys are present with correct values.
	if id, ok := capturedInput["serviceDomainId"]; !ok {
		t.Error("input missing 'serviceDomainId'")
	} else if id != "dom-abc-123" {
		t.Errorf("serviceDomainId = %v, want %q", id, "dom-abc-123")
	}

	if port, ok := capturedInput["targetPort"]; !ok {
		t.Error("input missing 'targetPort'")
	} else if port != float64(8080) { // JSON numbers decode as float64
		t.Errorf("targetPort = %v, want %v", port, 8080)
	}

	// Verify no extraneous keys that would break the mutation.
	for key := range capturedInput {
		switch key {
		case "serviceDomainId", "targetPort":
			// expected
		default:
			t.Errorf("unexpected key in input: %q", key)
		}
	}

	// Explicit check for the fields the bug originally included.
	if _, ok := capturedInput["serviceId"]; ok {
		t.Error("input must NOT contain 'serviceId'")
	}
	if _, ok := capturedInput["environmentId"]; ok {
		t.Error("input must NOT contain 'environmentId'")
	}
}
