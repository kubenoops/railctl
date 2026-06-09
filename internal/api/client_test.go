package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kubenoops/railctl/internal/resolver"
)

func TestClient_Debug(t *testing.T) {
	tests := []struct {
		name          string
		debug         bool
		wantDebugLogs bool
	}{
		{
			name:          "debug enabled shows logs",
			debug:         true,
			wantDebugLogs: true,
		},
		{
			name:          "debug disabled hides logs",
			debug:         false,
			wantDebugLogs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Return a simple successful response
				w.Header().Set("Content-Type", "application/json")
				resp := graphQLResponse{
					Data: json.RawMessage(`{"test": "data"}`),
				}
				json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			// Capture stderr
			oldStderr := os.Stderr
			r, w, _ := os.Pipe()
			os.Stderr = w

			// Create client with debug flag
			client := NewClient("test-token")
			client.apiURL = server.URL
			client.Debug = tt.debug

			// Execute a query
			_, err := client.execute("query { test }", map[string]any{"var": "value"})
			if err != nil {
				t.Fatalf("execute failed: %v", err)
			}

			// Restore stderr and read captured output
			w.Close()
			os.Stderr = oldStderr
			var buf bytes.Buffer
			io.Copy(&buf, r)
			output := buf.String()

			// Check if debug logs are present
			hasDebugLogs := strings.Contains(output, "[DEBUG] GraphQL Request") &&
				strings.Contains(output, "[DEBUG] GraphQL Response")

			if tt.wantDebugLogs && !hasDebugLogs {
				t.Errorf("expected debug logs but got none. Output: %s", output)
			}
			if !tt.wantDebugLogs && hasDebugLogs {
				t.Errorf("expected no debug logs but got some. Output: %s", output)
			}

			// Verify debug output contains expected information
			if tt.wantDebugLogs {
				if !strings.Contains(output, "URL:") {
					t.Error("debug output missing URL")
				}
				if !strings.Contains(output, "Query:") {
					t.Error("debug output missing Query")
				}
				if !strings.Contains(output, "Variables:") {
					t.Error("debug output missing Variables")
				}
				if !strings.Contains(output, "Status:") {
					t.Error("debug output missing Status")
				}
				if !strings.Contains(output, "Body:") {
					t.Error("debug output missing Body")
				}
			}
		})
	}
}

func TestClient_DebugWithError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := graphQLResponse{
			Errors: []graphQLError{
				{Message: "test error"},
			},
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

	// Execute a query (should fail)
	_, err := client.execute("query { test }", nil)

	// Restore stderr and read captured output
	w.Close()
	os.Stderr = oldStderr
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Should have error
	if err == nil {
		t.Fatal("expected error but got none")
	}

	// Should still have debug logs
	if !strings.Contains(output, "[DEBUG] GraphQL Request") {
		t.Error("expected debug request log")
	}
	if !strings.Contains(output, "[DEBUG] GraphQL Response") {
		t.Error("expected debug response log")
	}

	// Should show the error in response
	if !strings.Contains(output, "test error") {
		t.Error("expected error message in debug output")
	}
}

func TestCalculateBackoff(t *testing.T) {
	t.Run("increases with attempts", func(t *testing.T) {
		b0 := calculateBackoff(0)
		b1 := calculateBackoff(1)
		b2 := calculateBackoff(2)

		// Each attempt should be roughly larger than the previous (allowing for jitter)
		if b0 > b1 {
			t.Errorf("backoff did not increase: attempt 0=%v, attempt 1=%v", b0, b1)
		}
		if b1 > b2 {
			t.Errorf("backoff did not increase: attempt 1=%v, attempt 2=%v", b1, b2)
		}
	})

	t.Run("capped at MaxBackoff", func(t *testing.T) {
		b := calculateBackoff(100)
		// With jitter, can be up to 110% of MaxBackoff
		maxWithJitter := time.Duration(float64(MaxBackoff) * 1.11)
		if b > maxWithJitter {
			t.Errorf("backoff %v exceeded cap %v", b, maxWithJitter)
		}
	})
}

func TestTruncateBody(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string", "hello", 10, "hello"},
		{"exact length", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hello..."},
		{"strips html", "<html><body>content</body></html>", 100, "content"},
		{"collapses whitespace", "a   b  \n  c", 100, "a b c"},
		{"empty string", "", 10, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateBody(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateBody(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func makeWorkspaceServer(t *testing.T, workspaces []workspaceEntry) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		type wsResp struct {
			Data struct {
				Me struct {
					Workspaces []workspaceEntry `json:"workspaces"`
				} `json:"me"`
			} `json:"data"`
		}
		var resp wsResp
		resp.Data.Me.Workspaces = workspaces
		json.NewEncoder(w).Encode(resp)
	}))
}

func makeUnauthorizedWorkspaceServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"errors":[{"message":"Not Authorized"}]}`))
	}))
}

func TestGetWorkspaceID(t *testing.T) {
	wsPersonal := workspaceEntry{ID: "id-personal", Name: "personal"}
	wsTeam := workspaceEntry{ID: "id-team", Name: "acme"}
	wsTeamSub := workspaceEntry{ID: "id-team-sub", Name: "acme-staging"}

	tests := []struct {
		name       string
		workspaces []workspaceEntry
		hint       string // set via c.Workspace
		wantID     string
		wantErrIs  error // resolver.ErrNotFound or resolver.ErrAmbiguous
		wantErrMsg string
	}{
		{
			name:       "single workspace auto-detected",
			workspaces: []workspaceEntry{wsTeam},
			wantID:     "id-team",
		},
		{
			name:       "exact match",
			workspaces: []workspaceEntry{wsPersonal, wsTeam},
			hint:       "acme",
			wantID:     "id-team",
		},
		{
			name:       "exact match preferred over substring",
			workspaces: []workspaceEntry{wsTeam, wsTeamSub},
			hint:       "acme",
			wantID:     "id-team",
		},
		{
			name:       "substring match",
			workspaces: []workspaceEntry{wsPersonal, wsTeam},
			hint:       "acm",
			wantID:     "id-team",
		},
		{
			name:       "case-insensitive substring match",
			workspaces: []workspaceEntry{wsPersonal, wsTeam},
			hint:       "ACME",
			wantID:     "id-team",
		},
		{
			name:       "not found",
			workspaces: []workspaceEntry{wsPersonal, wsTeam},
			hint:       "unknown",
			wantErrIs:  resolver.ErrNotFound{},
		},
		{
			name:       "ambiguous substring",
			workspaces: []workspaceEntry{wsTeam, wsTeamSub},
			hint:       "acm",
			wantErrIs:  resolver.ErrAmbiguous{},
		},
		{
			name:       "multiple workspaces no hint errors",
			workspaces: []workspaceEntry{wsPersonal, wsTeam},
			wantErrMsg: "multiple workspaces found",
		},
		{
			name:       "no workspaces returns empty",
			workspaces: []workspaceEntry{},
			wantID:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := makeWorkspaceServer(t, tt.workspaces)
			defer server.Close()

			c := NewClient("test-token")
			c.apiURL = server.URL
			c.Workspace = tt.hint

			got, err := c.GetWorkspaceID()

			if tt.wantErrIs != nil {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				switch tt.wantErrIs.(type) {
				case resolver.ErrNotFound:
					var target resolver.ErrNotFound
					if !errors.As(err, &target) {
						t.Errorf("expected ErrNotFound, got %T: %v", err, err)
					}
				case resolver.ErrAmbiguous:
					var target resolver.ErrAmbiguous
					if !errors.As(err, &target) {
						t.Errorf("expected ErrAmbiguous, got %T: %v", err, err)
					}
				}
				return
			}

			if tt.wantErrMsg != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrMsg)
				}
				if !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrMsg)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantID {
				t.Errorf("GetWorkspaceID() = %q, want %q", got, tt.wantID)
			}
		})
	}
}

func TestGetWorkspaceID_Unauthorized(t *testing.T) {
	server := makeUnauthorizedWorkspaceServer(t)
	defer server.Close()

	t.Run("no hint returns error", func(t *testing.T) {
		c := NewClient("test-token")
		c.apiURL = server.URL

		_, err := c.GetWorkspaceID()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not authorized") {
			t.Errorf("error %q should mention authorization", err.Error())
		}
	})

	t.Run("with hint returns error", func(t *testing.T) {
		c := NewClient("test-token")
		c.apiURL = server.URL
		c.Workspace = "my-team"

		_, err := c.GetWorkspaceID()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "not authorized") {
			t.Errorf("error %q should mention authorization", err.Error())
		}
	})
}
