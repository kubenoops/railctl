package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
		CreateServiceDomainFunc: func(serviceID, environmentID string, targetPort int) (ServiceDomain, error) {
			if serviceID != "svc-1" || environmentID != "env-1" {
				t.Errorf("unexpected params: serviceID=%s, environmentID=%s", serviceID, environmentID)
			}
			return ServiceDomain{ID: "dom-1", Domain: "myapp-production.up.railway.app"}, nil
		},
	}

	domain, err := client.CreateServiceDomain("svc-1", "env-1", 8080)
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

// TestUpdateServiceDomainPort_MutationInput verifies all four NON_NULL fields
// (plus targetPort) are sent; omitting any makes Railway reject the request.
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

	err := client.UpdateServiceDomainPort("dom-abc-123", "myapp.up.railway.app", "env-1", "svc-1", 8080)
	if err != nil {
		t.Fatalf("UpdateServiceDomainPort returned error: %v", err)
	}

	// All four NON_NULL fields must be present with the right values.
	wantStrings := map[string]string{
		"serviceDomainId": "dom-abc-123",
		"domain":          "myapp.up.railway.app",
		"environmentId":   "env-1",
		"serviceId":       "svc-1",
	}
	for key, want := range wantStrings {
		if got, ok := capturedInput[key]; !ok {
			t.Errorf("input missing %q", key)
		} else if got != want {
			t.Errorf("%s = %v, want %q", key, got, want)
		}
	}

	if port, ok := capturedInput["targetPort"]; !ok {
		t.Error("input missing 'targetPort'")
	} else if port != float64(8080) { // JSON numbers decode as float64
		t.Errorf("targetPort = %v, want %v", port, 8080)
	}
}

// TestCreateServiceDomain_SendsTargetPort verifies targetPort is sent on create
// (when > 0) and omitted otherwise.
func TestCreateServiceDomain_SendsTargetPort(t *testing.T) {
	var capturedInput map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		capturedInput, _ = req.Variables["input"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"serviceDomainCreate": map[string]any{"id": "dom-1", "domain": "x.up.railway.app"}},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	if _, err := client.CreateServiceDomain("svc-1", "env-1", 8080); err != nil {
		t.Fatalf("CreateServiceDomain returned error: %v", err)
	}
	if port, ok := capturedInput["targetPort"]; !ok {
		t.Error("input missing 'targetPort'")
	} else if port != float64(8080) {
		t.Errorf("targetPort = %v, want %v", port, 8080)
	}

	// A zero port must be omitted so Railway auto-detects the port.
	if _, err := client.CreateServiceDomain("svc-1", "env-1", 0); err != nil {
		t.Fatalf("CreateServiceDomain returned error: %v", err)
	}
	if _, ok := capturedInput["targetPort"]; ok {
		t.Error("targetPort must be omitted when port is 0")
	}
}

// TestCreateCustomDomain verifies the input carries projectId and the returned
// DNS records are parsed for printing.
func TestCreateCustomDomain(t *testing.T) {
	var capturedInput map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		var req struct {
			Variables map[string]any `json:"variables"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		capturedInput, _ = req.Variables["input"].(map[string]any)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"customDomainCreate": map[string]any{
				"id": "cd-1", "domain": "app.example.com", "targetPort": 8080,
				"status": map[string]any{
					"verified":            false,
					"verificationDnsHost": "_railway-verify.app",
					"verificationToken":   "railway-verify=token123",
					"dnsRecords": []any{
						map[string]any{
							"recordType": "DNS_RECORD_TYPE_CNAME", "purpose": "DNS_RECORD_PURPOSE_TRAFFIC_ROUTE",
							"hostlabel": "app", "fqdn": "app.example.com",
							"requiredValue": "abc.up.railway.app", "currentValue": "", "status": "PROPAGATING",
						},
					},
				},
			}},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	cd, err := client.CreateCustomDomain("proj-1", "env-1", "svc-1", "app.example.com", 8080)
	if err != nil {
		t.Fatalf("CreateCustomDomain returned error: %v", err)
	}

	for _, k := range []string{"domain", "environmentId", "projectId", "serviceId"} {
		if _, ok := capturedInput[k]; !ok {
			t.Errorf("input missing %q", k)
		}
	}
	if cd.Status == nil || len(cd.Status.DNSRecords) != 1 {
		t.Fatalf("expected 1 routing DNS record, got %+v", cd.Status)
	}
	if r := cd.Status.DNSRecords[0]; r.RecordType != "DNS_RECORD_TYPE_CNAME" || r.RequiredValue != "abc.up.railway.app" {
		t.Errorf("record[0] = %+v, want CNAME → abc.up.railway.app", r)
	}
	// Verification TXT comes from separate status fields, not dnsRecords.
	if cd.Status.VerificationDNSHost != "_railway-verify.app" || cd.Status.VerificationToken != "railway-verify=token123" {
		t.Errorf("verification fields = %q / %q, want _railway-verify.app / railway-verify=token123",
			cd.Status.VerificationDNSHost, cd.Status.VerificationToken)
	}
}

// TestDeleteCustomDomain_MutationInput verifies the customDomainDelete
// mutation is sent with the domain ID (the schema takes only `id`).
func TestDeleteCustomDomain_MutationInput(t *testing.T) {
	var capturedQuery string
	var capturedVars map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		var req struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		capturedQuery = req.Query
		capturedVars = req.Variables
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"customDomainDelete": true},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	if err := client.DeleteCustomDomain("cd-abc-123"); err != nil {
		t.Fatalf("DeleteCustomDomain returned error: %v", err)
	}

	if !strings.Contains(capturedQuery, "customDomainDelete") {
		t.Errorf("query does not call customDomainDelete:\n%s", capturedQuery)
	}
	if got := capturedVars["id"]; got != "cd-abc-123" {
		t.Errorf("id = %v, want %q", got, "cd-abc-123")
	}
}

// TestListDomains_ParsesCustomDomainStatus verifies the domains query selects
// the custom-domain verification status and that it round-trips into
// CustomDomain.Status (used by `get domains` for the STATUS column).
func TestListDomains_ParsesCustomDomainStatus(t *testing.T) {
	var capturedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()
		var req struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("failed to unmarshal request: %v", err)
		}
		capturedQuery = req.Query
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": map[string]any{"domains": map[string]any{
				"serviceDomains": []any{
					map[string]any{"id": "dom-1", "domain": "x.up.railway.app", "targetPort": 8080},
				},
				"customDomains": []any{
					map[string]any{
						"id": "cd-1", "domain": "app.example.com", "targetPort": 3000,
						"status": map[string]any{"verified": true},
					},
				},
			}},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	domains, err := client.ListDomains("proj-1", "env-1", "svc-1")
	if err != nil {
		t.Fatalf("ListDomains returned error: %v", err)
	}

	if !strings.Contains(capturedQuery, "verified") {
		t.Errorf("query does not select custom-domain verification status:\n%s", capturedQuery)
	}
	if len(domains.CustomDomains) != 1 {
		t.Fatalf("expected 1 custom domain, got %d", len(domains.CustomDomains))
	}
	cd := domains.CustomDomains[0]
	if cd.Status == nil || !cd.Status.Verified {
		t.Errorf("expected verified status parsed, got %+v", cd.Status)
	}
}
