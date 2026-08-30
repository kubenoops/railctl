package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

func saveRegionGlobals(t *testing.T) {
	t.Helper()
	origClient := newAPIClient
	origToken := token
	origFormat := outputFormat
	t.Cleanup(func() {
		newAPIClient = origClient
		token = origToken
		outputFormat = origFormat
	})
}

func setRegionEnv(client api.APIClient, format string) {
	token = "test-token"
	outputFormat = format
	newAPIClient = func(tkn string) api.APIClient { return client }
}

func regionsMock(regions []types.Region) *api.MockClient {
	return &api.MockClient{
		ListRegionsFunc: func() ([]types.Region, error) { return regions, nil },
	}
}

var sampleRegions = []types.Region{
	{Name: "us-west2", Country: "US", Location: "California, USA"},
	{Name: "europe-west4-drams3a", Country: "NL", Location: "Amsterdam, Netherlands"},
}

// REQ-CMD-007: table shows NAME/LOCATION.
func TestRunGetRegions_Table(t *testing.T) {
	saveRegionGlobals(t)
	setRegionEnv(regionsMock(sampleRegions), "table")

	out, err := captureStdout(t, func() error {
		return getRegionsCmd.RunE(getRegionsCmd, []string{})
	})
	if err != nil {
		t.Fatalf("runGetRegions error: %v", err)
	}
	for _, want := range []string{"NAME", "LOCATION", "us-west2", "California, USA", "europe-west4-drams3a"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "COUNTRY") {
		t.Errorf("plain table should not have COUNTRY column:\n%s", out)
	}
}

// REQ-CMD-007: wide adds COUNTRY.
func TestRunGetRegions_Wide(t *testing.T) {
	saveRegionGlobals(t)
	setRegionEnv(regionsMock(sampleRegions), "wide")

	out, err := captureStdout(t, func() error {
		return getRegionsCmd.RunE(getRegionsCmd, []string{})
	})
	if err != nil {
		t.Fatalf("runGetRegions error: %v", err)
	}
	for _, want := range []string{"NAME", "LOCATION", "COUNTRY", "US", "NL"} {
		if !strings.Contains(out, want) {
			t.Errorf("wide table missing %q in:\n%s", want, out)
		}
	}
}

// REQ-CMD-007: json is valid and contains the regions.
func TestRunGetRegions_JSON(t *testing.T) {
	saveRegionGlobals(t)
	setRegionEnv(regionsMock(sampleRegions), "json")

	out, err := captureStdout(t, func() error {
		return getRegionsCmd.RunE(getRegionsCmd, []string{})
	})
	if err != nil {
		t.Fatalf("runGetRegions error: %v", err)
	}
	var got []types.Region
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	if len(got) != 2 || got[0].Name != "us-west2" {
		t.Errorf("unexpected JSON regions: %+v", got)
	}
}

func TestRunGetRegions_Empty(t *testing.T) {
	saveRegionGlobals(t)
	setRegionEnv(regionsMock(nil), "table")

	out, err := captureStdout(t, func() error {
		return getRegionsCmd.RunE(getRegionsCmd, []string{})
	})
	if err != nil {
		t.Fatalf("runGetRegions error: %v", err)
	}
	if !strings.Contains(out, "No regions found") {
		t.Errorf("expected empty message, got: %q", out)
	}
}
