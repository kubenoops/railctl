package diff

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/config"
)

// All values in this file are FAKE fixtures. Never paste real secrets here.

const fakePEM = "-----BEGIN RSA PRIVATE KEY-----\nFAKEFAKEFAKEFAKEFAKEFAKEFAKEFAKE\n-----END RSA PRIVATE KEY-----"

// fakeSecrets maps the leaked name shapes (any case) to fake values that must
// never appear in rendered diff output.
var fakeSecrets = map[string]string{
	"CLERK_SECRET_KEY":            "fake-snake-0001",
	"Clerk__SecretKey":            "fake-dotnet-0002",
	"githubapp__privatekeypem":    fakePEM,
	"SURREALDB__PASS":             "fake-surreal-0003",
	"hAnGfIrE__dAsHbOaRdPaSsWoRd": "fake-hangfire-0004",
	"NORMAL_VAR":                  "fake-neutral-0005", // not safe-listed → masked too
}

func assertNoLeak(t *testing.T, out string) {
	t.Helper()
	for k, v := range fakeSecrets {
		probe := v
		if v == fakePEM {
			probe = "FAKEFAKE"
		}
		if strings.Contains(out, probe) {
			t.Errorf("diff output leaked %s value:\n%s", k, out)
		}
	}
	if strings.Contains(out, "BEGIN RSA") {
		t.Errorf("diff output leaked PEM content:\n%s", out)
	}
}

func render(cs *ChangeSet) string {
	var buf bytes.Buffer
	Render(cs, &buf, false)
	return buf.String()
}

// The leak that triggered this fix: live variables omitted from the manifest
// render as removal lines carrying the CURRENT value.
func TestMasking_RemovalLines(t *testing.T) {
	live := map[string]string{"PORT": "3000"}
	for k, v := range fakeSecrets {
		live[k] = v
	}
	cs := Compute(
		[]config.ServiceConfig{{Name: "api", Image: "img:1"}},
		[]LiveService{{Name: "api", Image: "img:1", Variables: live}},
		false,
	)
	out := render(cs)
	assertNoLeak(t, out)
	if !strings.Contains(out, "variables.PORT: 3000") {
		t.Errorf("safe-listed PORT should render in clear:\n%s", out)
	}
}

func TestMasking_AddAndChangeLines(t *testing.T) {
	desired := map[string]string{}
	live := map[string]string{}
	i := 0
	for k, v := range fakeSecrets {
		if i%2 == 0 {
			desired[k] = v // added
		} else {
			desired[k] = v + "-new" // changed
			live[k] = v
		}
		i++
	}
	cs := Compute(
		[]config.ServiceConfig{{Name: "api", Image: "img:1", Variables: desired}},
		[]LiveService{{Name: "api", Image: "img:1", Variables: live}},
		false,
	)
	assertNoLeak(t, render(cs))
}

func TestMasking_CreateAndDelete(t *testing.T) {
	// Create: every desired variable is printed as an addition.
	cs := Compute([]config.ServiceConfig{{Name: "new", Image: "img:1", Variables: fakeSecrets}}, nil, false)
	assertNoLeak(t, render(cs))

	// Delete (prune): every live variable is printed as a removal.
	cs = Compute(nil, []LiveService{{Name: "old", Image: "img:1", Variables: fakeSecrets}}, true)
	assertNoLeak(t, render(cs))
}

// A multi-line value is masked even under a safe-listed key, and a change from
// one PEM to another is still reported (as a masked change line).
func TestMasking_PEMUnderSafeKey(t *testing.T) {
	cs := Compute(
		[]config.ServiceConfig{{Name: "api", Image: "img:1", Variables: map[string]string{"HOSTNAME": fakePEM + "\n"}}},
		[]LiveService{{Name: "api", Image: "img:1", Variables: map[string]string{"HOSTNAME": fakePEM}}},
		false,
	)
	out := render(cs)
	assertNoLeak(t, out)
	if !strings.Contains(out, "~ variables.HOSTNAME:") {
		t.Errorf("expected a masked change line for HOSTNAME:\n%s", out)
	}
}
