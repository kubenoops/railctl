package diff

import (
	"bytes"
	"strings"
	"testing"
)

func TestRender_Create(t *testing.T) {
	cs := &ChangeSet{
		Changes: []ResourceChange{
			{
				Type:        ChangeCreate,
				ServiceName: "worker",
				Fields: []FieldDiff{
					{Path: "image", Desired: "node:20-alpine"},
					{Path: "deploy.startCommand", Desired: "npm run worker"},
					{Path: "deploy.restartPolicy", Desired: "ON_FAILURE"},
					{Path: "variables.PORT", Desired: "3000"},
					{Path: "volume.mountPath", Desired: "/data"},
				},
			},
		},
	}

	var buf bytes.Buffer
	Render(cs, &buf, false)
	out := buf.String()

	if !strings.Contains(out, "Service: worker (create)") {
		t.Errorf("expected 'Service: worker (create)' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "+ image: node:20-alpine") {
		t.Errorf("expected '+ image: node:20-alpine' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "+ deploy.startCommand: npm run worker") {
		t.Errorf("expected '+ deploy.startCommand: npm run worker' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "+ variables.PORT: 3000") {
		t.Errorf("expected '+ variables.PORT: 3000' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "+ volume.mountPath: /data") {
		t.Errorf("expected '+ volume.mountPath: /data' in output, got:\n%s", out)
	}
}

func TestRender_Update(t *testing.T) {
	cs := &ChangeSet{
		Changes: []ResourceChange{
			{
				Type:        ChangeUpdate,
				ServiceName: "api",
				Fields: []FieldDiff{
					{Path: "image", Current: "node:18-alpine", Desired: "node:20-alpine"},
					{Path: "deploy.replicas", Current: "1", Desired: "2"},
					{Path: "variables.NEW_VAR", Current: "", Desired: "value"},
					{Path: "variables.OLD_VAR", Current: "removed", Desired: ""},
				},
			},
		},
	}

	var buf bytes.Buffer
	Render(cs, &buf, false)
	out := buf.String()

	if !strings.Contains(out, "Service: api (update)") {
		t.Errorf("expected 'Service: api (update)' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "~ image: node:18-alpine → node:20-alpine") {
		t.Errorf("expected '~ image: node:18-alpine → node:20-alpine' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "~ deploy.replicas: 1 → 2") {
		t.Errorf("expected '~ deploy.replicas: 1 → 2' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "+ variables.NEW_VAR: value") {
		t.Errorf("expected '+ variables.NEW_VAR: value' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "- variables.OLD_VAR: removed") {
		t.Errorf("expected '- variables.OLD_VAR: removed' in output, got:\n%s", out)
	}
}

func TestRender_Delete(t *testing.T) {
	cs := &ChangeSet{
		Changes: []ResourceChange{
			{
				Type:        ChangeDelete,
				ServiceName: "old-service",
				Fields: []FieldDiff{
					{Path: "image", Current: "nginx:1.25"},
					{Path: "variables.PORT", Current: "8080"},
				},
			},
		},
	}

	var buf bytes.Buffer
	Render(cs, &buf, false)
	out := buf.String()

	if !strings.Contains(out, "Service: old-service (delete)") {
		t.Errorf("expected 'Service: old-service (delete)' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "- image: nginx:1.25") {
		t.Errorf("expected '- image: nginx:1.25' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "- variables.PORT: 8080") {
		t.Errorf("expected '- variables.PORT: 8080' in output, got:\n%s", out)
	}
}

func TestRender_NoChanges(t *testing.T) {
	cs := &ChangeSet{}

	var buf bytes.Buffer
	Render(cs, &buf, false)
	out := buf.String()

	if !strings.Contains(out, "No changes. Railway state matches the config.") {
		t.Errorf("expected 'No changes' message, got:\n%s", out)
	}
}

func TestRender_NoColor(t *testing.T) {
	cs := &ChangeSet{
		Changes: []ResourceChange{
			{
				Type:        ChangeCreate,
				ServiceName: "web",
				Fields: []FieldDiff{
					{Path: "image", Desired: "node:20"},
				},
			},
		},
	}

	var buf bytes.Buffer
	Render(cs, &buf, false)
	out := buf.String()

	if strings.Contains(out, "\033[") {
		t.Errorf("expected no ANSI escape codes with useColor=false, got:\n%s", out)
	}
}

func TestRender_WithColor(t *testing.T) {
	cs := &ChangeSet{
		Changes: []ResourceChange{
			{
				Type:        ChangeCreate,
				ServiceName: "web",
				Fields: []FieldDiff{
					{Path: "image", Desired: "node:20"},
				},
			},
			{
				Type:        ChangeUpdate,
				ServiceName: "api",
				Fields: []FieldDiff{
					{Path: "image", Current: "old", Desired: "new"},
				},
			},
			{
				Type:        ChangeDelete,
				ServiceName: "old",
				Fields: []FieldDiff{
					{Path: "image", Current: "nginx"},
				},
			},
		},
	}

	var buf bytes.Buffer
	Render(cs, &buf, true)
	out := buf.String()

	// Check for ANSI escape codes.
	if !strings.Contains(out, "\033[") {
		t.Errorf("expected ANSI escape codes with useColor=true, got:\n%s", out)
	}

	// Bold for service headers.
	if !strings.Contains(out, colorBold) {
		t.Errorf("expected bold ANSI code in output")
	}

	// Green for additions.
	if !strings.Contains(out, colorGreen) {
		t.Errorf("expected green ANSI code in output")
	}

	// Yellow for changes.
	if !strings.Contains(out, colorYellow) {
		t.Errorf("expected yellow ANSI code in output")
	}

	// Red for deletions.
	if !strings.Contains(out, colorRed) {
		t.Errorf("expected red ANSI code in output")
	}

	// Reset codes.
	if !strings.Contains(out, colorReset) {
		t.Errorf("expected reset ANSI code in output")
	}
}

func TestIsColorSupported(t *testing.T) {
	// A bytes.Buffer is not a terminal, so auto-detection returns false.
	if IsColorSupported(&bytes.Buffer{}) {
		t.Error("expected false for a non-terminal writer")
	}

	// NO_COLOR disables color even when set to an empty value.
	t.Setenv("NO_COLOR", "")
	if IsColorSupported(&bytes.Buffer{}) {
		t.Error("NO_COLOR should disable color")
	}
}
