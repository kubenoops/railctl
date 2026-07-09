package apply

import (
	"bytes"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/config"
	"github.com/kubenoops/railctl/internal/diff"
	"github.com/kubenoops/railctl/internal/types"
)

// TestApply_TCPProxyRemoval: omitting the tcpProxy block from a service that
// has a live proxy must delete that proxy on apply (declarative un-expose).
func TestApply_TCPProxyRemoval(t *testing.T) {
	var deletedProxyID string
	mock := &api.MockClient{
		ListServicesFunc: func(_, _ string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "s-db", Name: "db"}}, nil
		},
		ListTCPProxiesFunc: func(_, _ string) ([]api.TCPProxy, error) {
			return []api.TCPProxy{{ID: "tp-1", ApplicationPort: 5432}}, nil
		},
		DeleteTCPProxyFunc: func(id string) error { deletedProxyID = id; return nil },
		ListDomainsFunc: func(_, _, _ string) (api.DomainList, error) {
			return api.DomainList{}, nil
		},
	}
	// Desired: db with NO networking; the diff flags a tcpProxy removal.
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type:        diff.ChangeUpdate,
		ServiceName: "db",
		Fields:      []diff.FieldDiff{{Path: "networking.tcpProxy.port", Current: "5432", Desired: ""}},
	}}}
	cfgMap := map[string]config.ServiceConfig{"db": {Name: "db", Image: "postgres:16"}}

	res := Apply(mock, cs, "proj", "env", cfgMap, Opts{Output: &bytes.Buffer{}})
	if len(res.Errors) != 0 {
		t.Fatalf("apply errors: %v", res.Errors)
	}
	if deletedProxyID != "tp-1" {
		t.Errorf("expected TCP proxy tp-1 deleted, got %q", deletedProxyID)
	}
}

// TestApply_DomainRemoval: omitting networking.domain removes the service domain.
func TestApply_DomainRemoval(t *testing.T) {
	var deletedDomainID string
	tp := 8080
	mock := &api.MockClient{
		ListServicesFunc: func(_, _ string) ([]types.ServiceDetail, error) {
			return []types.ServiceDetail{{ID: "s-web", Name: "web"}}, nil
		},
		ListDomainsFunc: func(_, _, _ string) (api.DomainList, error) {
			return api.DomainList{ServiceDomains: []api.ServiceDomain{{ID: "sd-1", Domain: "web.up.railway.app", TargetPort: &tp}}}, nil
		},
		DeleteServiceDomainFunc: func(id string) error { deletedDomainID = id; return nil },
		ListTCPProxiesFunc:      func(_, _ string) ([]api.TCPProxy, error) { return nil, nil },
	}
	cs := &diff.ChangeSet{Changes: []diff.ResourceChange{{
		Type:        diff.ChangeUpdate,
		ServiceName: "web",
		Fields:      []diff.FieldDiff{{Path: "networking.domain.port", Current: "8080", Desired: ""}},
	}}}
	cfgMap := map[string]config.ServiceConfig{"web": {Name: "web", Image: "web:latest"}}

	res := Apply(mock, cs, "proj", "env", cfgMap, Opts{Output: &bytes.Buffer{}})
	if len(res.Errors) != 0 {
		t.Fatalf("apply errors: %v", res.Errors)
	}
	if deletedDomainID != "sd-1" {
		t.Errorf("expected service domain sd-1 deleted, got %q", deletedDomainID)
	}
}
