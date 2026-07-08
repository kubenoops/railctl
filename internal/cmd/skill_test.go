package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestSkillCmd_PrintsEmbeddedGuide(t *testing.T) {
	var out bytes.Buffer
	skillCmd.SetOut(&out)
	t.Cleanup(func() { skillCmd.SetOut(nil) })

	if err := skillCmd.RunE(skillCmd, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := out.String()
	if strings.TrimSpace(got) == "" {
		t.Fatal("skill command produced no output")
	}
	// Spot-check a couple of anchors so a broken embed surfaces here too.
	for _, want := range []string{"# railctl Usage", "token model"} {
		if !strings.Contains(got, want) {
			t.Errorf("skill output missing %q", want)
		}
	}
}

func TestSkillCmd_TakesNoArgs(t *testing.T) {
	// Args validator should reject positional arguments.
	if err := skillCmd.Args(skillCmd, []string{"unexpected"}); err == nil {
		t.Error("expected 'skill' to reject positional arguments")
	}
}
