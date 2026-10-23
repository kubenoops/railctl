package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/types"
)

// All values in this file are FAKE fixtures. Never paste real secrets here.

const fakePEMCmd = "-----BEGIN RSA PRIVATE KEY-----\nFAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKE\n-----END RSA PRIVATE KEY-----"

var fakeCmdVars = map[string]string{
	"PORT":                        "3000",
	"CLERK_SECRET_KEY":            "fake-snake-0001",
	"clerk__secretkey":            "fake-dotnet-0002",
	"GitHubApp__PrivateKeyPem":    fakePEMCmd,
	"surrealDB__PASS":             "fake-surreal-0003",
	"Hangfire__DashboardPassword": "fake-hangfire-0004",
	"SurrealDb__User":             "fake-neutral-0005",
}

func assertCmdNoLeak(t *testing.T, out string) {
	t.Helper()
	for _, probe := range []string{"fake-snake-0001", "fake-dotnet-0002", "FAKEFAKE", "BEGIN RSA", "fake-surreal-0003", "fake-hangfire-0004", "fake-neutral-0005"} {
		if strings.Contains(out, probe) {
			t.Errorf("output leaked %q:\n%s", probe, out)
		}
	}
	if !strings.Contains(out, "3000") {
		t.Errorf("safe-listed PORT should be shown in clear:\n%s", out)
	}
}

func TestMasking_GetVariablesJSON(t *testing.T) {
	b, _ := json.Marshal(buildVarsWithSealed(fakeCmdVars, map[string]bool{}, false))
	assertCmdNoLeak(t, string(b))
}

func TestMasking_GetVariablesTable(t *testing.T) {
	out, _ := captureStdout(t, func() error {
		printVariablesTable(fakeCmdVars, map[string]bool{}, "api", "production", false)
		return nil
	})
	assertCmdNoLeak(t, out)
}

func TestMasking_DescribeService(t *testing.T) {
	svc := types.ServiceDetail{ID: "svc-1", Name: "api"}
	b, _ := json.Marshal(serviceDetailToOutput(svc, "proj", "production", fakeCmdVars, map[string]bool{}, false))
	assertCmdNoLeak(t, string(b))

	out, _ := captureStdout(t, func() error {
		return printServiceDetail(svc, "proj", "production", fakeCmdVars, map[string]bool{}, false)
	})
	assertCmdNoLeak(t, out)
}

// --show-values is the explicit opt-out and still reveals values.
func TestMasking_ShowValuesOptOut(t *testing.T) {
	got := buildVarsWithSealed(map[string]string{"Clerk__SecretKey": "fake-dotnet-0002"}, map[string]bool{}, true)
	if got["Clerk__SecretKey"].Value != "fake-dotnet-0002" {
		t.Errorf("--show-values should reveal, got %q", got["Clerk__SecretKey"].Value)
	}
}
