//go:build e2e

package harness

import (
	"encoding/json"
	"strings"
	"testing"
)

// AssertContains errors the test if haystack does not contain needle.
func AssertContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if !strings.Contains(haystack, needle) {
		t.Errorf("expected output to contain %q, got:\n%s", needle, truncate(haystack, 500))
	}
}

// AssertNotContains errors the test if haystack contains needle.
func AssertNotContains(t *testing.T, haystack, needle string) {
	t.Helper()
	if strings.Contains(haystack, needle) {
		t.Errorf("expected output NOT to contain %q, got:\n%s", needle, truncate(haystack, 500))
	}
}

// AssertValidJSON errors the test if s is not valid JSON.
func AssertValidJSON(t *testing.T, s string) {
	t.Helper()
	if !json.Valid([]byte(s)) {
		t.Errorf("expected valid JSON, got:\n%s", truncate(s, 300))
	}
}

// AssertValidYAML errors the test if s is empty (lightweight YAML sanity check).
func AssertValidYAML(t *testing.T, s string) {
	t.Helper()
	s = strings.TrimSpace(s)
	if s == "" {
		t.Error("expected non-empty YAML output")
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "... (truncated)"
	}
	return s
}
