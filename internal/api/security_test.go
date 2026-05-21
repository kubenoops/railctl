package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestClient_RedactSensitiveVariables(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return success for VariableCollectionUpsert
		resp := graphQLResponse{
			Data: json.RawMessage(`{"variableCollectionUpsert": true}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Create client with debug enabled
	client := NewClient("test-token")
	client.apiURL = server.URL
	client.Debug = true

	// Sensitive data
	sensitiveKey := "MY_SECRET_KEY"
	sensitiveValue := "super-secret-password-123"

	// Call SetVariables which triggers VariableCollectionUpsert mutation
	err := client.SetVariables("proj-id", "env-id", "svc-id", map[string]string{
		sensitiveKey: sensitiveValue,
	}, false)
	if err != nil {
		t.Fatalf("SetVariables failed: %v", err)
	}

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check if sensitive data is leaked in debug output
	// Before fix, this should be TRUE (leaked)
	// After fix, this should be FALSE (redacted)
	if strings.Contains(output, sensitiveValue) {
		t.Errorf("Security Vulnerability: Sensitive data %q was leaked in debug output!\nFull output:\n%s", sensitiveValue, output)
	}
}

func TestClient_RedactRegistryCredentials(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return success for CreateService
		resp := graphQLResponse{
			Data: json.RawMessage(`{"serviceCreate": {"id": "svc-1", "name": "web"}}`),
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Create client with debug enabled
	client := NewClient("test-token")
	client.apiURL = server.URL
	client.Debug = true

	// Sensitive data
	registryPass := "reg-secret-token"

	// Call CreateService with credentials
	_, err := client.CreateService("proj-id", "env-id", "web", "nginx", &RegistryCredentials{
		Username: "user",
		Password: registryPass,
	})
	if err != nil {
		t.Fatalf("CreateService failed: %v", err)
	}

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Check if sensitive data is leaked in debug output
	if strings.Contains(output, registryPass) {
		t.Errorf("Security Vulnerability: Registry password %q was leaked in debug output!\nFull output:\n%s", registryPass, output)
	}
}

func TestIsSensitiveKey(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		// Should match (sensitive keys)
		{"API_KEY", true},
		{"DATABASE_PASSWORD", true},
		{"AWS_SECRET_ACCESS_KEY", true},
		{"AUTH_TOKEN", true},
		{"PRIVATE_KEY", true},
		{"MY_CREDENTIAL", true},
		{"MY_CREDENTIALS", true},
		{"APIKEY", true},
		{"api_key", true},
		{"Secret", true},
		{"JWT_TOKEN", true},
		{"OAUTH_SECRET", true},

		// Should NOT match (false positive prevention via word boundaries)
		{"PORT", false},
		{"NODE_ENV", false},
		{"NORMAL_VAR", false},
		{"PATH", false},           // should NOT match KEY (word boundary)
		{"AUTHOR", false},         // should NOT match AUTH (word boundary)
		{"MY_PATH_CONFIG", false}, // should NOT match KEY
		{"DATABASE_URL", false},
		{"HOSTNAME", false},
		{"LOG_LEVEL", false},
	}

	for _, tc := range tests {
		result := IsSensitiveKey(tc.key)
		if result != tc.expected {
			t.Errorf("IsSensitiveKey(%q) = %v, expected %v", tc.key, result, tc.expected)
		}
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"a", "**************"},
		{"ab", "**************"},
		{"abc", "ab************"},
		{"abcde", "ab************"},
		{"mysecretpassword", "my************"},
		{"super-long-secret-value-that-is-very-long", "su************"},
	}

	for _, tc := range tests {
		result := MaskValue(tc.input)
		if result != tc.expected {
			t.Errorf("MaskValue(%q) = %q, expected %q", tc.input, result, tc.expected)
		}
	}
}

func TestMaskValue_FixedLength(t *testing.T) {
	// All non-empty values should produce the same output length (14 chars)
	veryShort := MaskValue("a")
	short := MaskValue("abc")
	long := MaskValue("this-is-a-very-long-secret-value-12345")

	if len(veryShort) != len(short) || len(short) != len(long) {
		t.Errorf("MaskValue should produce fixed-length output: len(%q)=%d, len(%q)=%d, len(%q)=%d",
			veryShort, len(veryShort), short, len(short), long, len(long))
	}
}

func TestMaskValue_UTF8Safety(t *testing.T) {
	// Multi-byte characters should not produce invalid UTF-8
	result := MaskValue("émoji🎉test")
	if result != "ém************" {
		t.Errorf("MaskValue with multi-byte chars = %q, expected %q", result, "ém************")
	}
}
