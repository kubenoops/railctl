package api

import (
	"encoding/json"
	"testing"
)

func TestListTCPProxies(t *testing.T) {
	client := &MockClient{
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]TCPProxy, error) {
			if environmentID != "env-1" || serviceID != "svc-1" {
				t.Errorf("unexpected params: environmentID=%s, serviceID=%s", environmentID, serviceID)
			}
			return []TCPProxy{
				{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 12345, ApplicationPort: 5432},
			}, nil
		},
	}

	proxies, err := client.ListTCPProxies("env-1", "svc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proxies) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(proxies))
	}

	if proxies[0].ApplicationPort != 5432 {
		t.Errorf("unexpected application port: %d", proxies[0].ApplicationPort)
	}

	if proxies[0].Domain != "roundhouse.proxy.rlwy.net" {
		t.Errorf("unexpected domain: %s", proxies[0].Domain)
	}
}

func TestListTCPProxies_Empty(t *testing.T) {
	client := &MockClient{
		ListTCPProxiesFunc: func(environmentID, serviceID string) ([]TCPProxy, error) {
			return nil, nil
		},
	}

	proxies, err := client.ListTCPProxies("env-1", "svc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(proxies) != 0 {
		t.Errorf("expected 0 proxies, got %d", len(proxies))
	}
}

func TestCreateTCPProxy(t *testing.T) {
	client := &MockClient{
		CreateTCPProxyFunc: func(applicationPort int, environmentID, serviceID string) (TCPProxy, error) {
			if applicationPort != 5432 || environmentID != "env-1" || serviceID != "svc-1" {
				t.Errorf("unexpected params: port=%d, environmentID=%s, serviceID=%s", applicationPort, environmentID, serviceID)
			}
			return TCPProxy{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 12345, ApplicationPort: 5432}, nil
		},
	}

	proxy, err := client.CreateTCPProxy(5432, "env-1", "svc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if proxy.ID != "tcp-1" {
		t.Errorf("unexpected proxy ID: %s", proxy.ID)
	}
	if proxy.ProxyPort != 12345 {
		t.Errorf("unexpected proxy port: %d", proxy.ProxyPort)
	}
	if proxy.ApplicationPort != 5432 {
		t.Errorf("unexpected application port: %d", proxy.ApplicationPort)
	}
}

func TestTCPProxyJSON(t *testing.T) {
	original := TCPProxy{ID: "tcp-1", Domain: "roundhouse.proxy.rlwy.net", ProxyPort: 12345, ApplicationPort: 5432}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled TCPProxy
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.ID != original.ID {
		t.Errorf("ID mismatch: %s != %s", unmarshaled.ID, original.ID)
	}
	if unmarshaled.Domain != original.Domain {
		t.Errorf("Domain mismatch: %s != %s", unmarshaled.Domain, original.Domain)
	}
	if unmarshaled.ProxyPort != original.ProxyPort {
		t.Errorf("ProxyPort mismatch: %d != %d", unmarshaled.ProxyPort, original.ProxyPort)
	}
	if unmarshaled.ApplicationPort != original.ApplicationPort {
		t.Errorf("ApplicationPort mismatch: %d != %d", unmarshaled.ApplicationPort, original.ApplicationPort)
	}
}
