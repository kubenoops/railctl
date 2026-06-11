package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
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

// makeUnauthorizedWorkspaceServer returns a server that rejects all probes — simulates
// a completely invalid token.
func makeUnauthorizedWorkspaceServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errors":[{"message":"Not Authorized"}]}`))
	}))
}

// makeTokenDetectionServer returns a server that simulates the three Railway token-type
// probes. It dispatches by header and query body so each probe scenario can be tested.
func makeTokenDetectionServer(t *testing.T, accountWorks, workspaceWorks, projectWorks bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		body, _ := io.ReadAll(r.Body)
		bodyStr := string(body)

		// Probe 3 uses Project-Access-Token header
		if r.Header.Get("Project-Access-Token") != "" {
			if projectWorks {
				w.Write([]byte(`{"data":{"projectToken":{"projectId":"proj-123","environmentId":"env-456"}}}`))
			} else {
				w.Write([]byte(`{"errors":[{"message":"Not Authorized"}]}`))
			}
			return
		}

		// Probe 1 queries me.workspaces
		if strings.Contains(bodyStr, "workspaces") {
			if accountWorks {
				w.Write([]byte(`{"data":{"me":{"workspaces":[{"id":"ws-123","name":"test-ws"}]}}}`))
			} else {
				w.Write([]byte(`{"errors":[{"message":"Not Authorized"}]}`))
			}
			return
		}

		// Probe 2 lists projects
		if workspaceWorks {
			w.Write([]byte(`{"data":{"projects":{"edges":[]}}}`))
		} else {
			w.Write([]byte(`{"errors":[{"message":"Not Authorized"}]}`))
		}
	}))
}

func TestDetectTokenType(t *testing.T) {
	tests := []struct {
		name               string
		accountWorks       bool
		workspaceWorks     bool
		projectWorks       bool
		wantType           TokenType
		wantProjectToken   bool
		wantWorkspaceToken bool
		wantErrContains    string
	}{
		{
			name:         "account token detected",
			accountWorks: true,
			wantType:     TokenTypeAccount,
		},
		{
			name:               "workspace token detected",
			workspaceWorks:     true,
			wantType:           TokenTypeWorkspace,
			wantWorkspaceToken: true,
		},
		{
			name:             "project token detected",
			projectWorks:     true,
			wantType:         TokenTypeProject,
			wantProjectToken: true,
		},
		{
			name:            "all probes fail returns error",
			wantErrContains: "not authorized",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := makeTokenDetectionServer(t, tt.accountWorks, tt.workspaceWorks, tt.projectWorks)
			defer server.Close()

			c := NewClient("test-token")
			c.apiURL = server.URL

			got, err := c.detectTokenType()

			if tt.wantErrContains != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(strings.ToLower(err.Error()), tt.wantErrContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrContains)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantType {
				t.Errorf("detectTokenType() = %v, want %v", got, tt.wantType)
			}
			if tt.wantProjectToken && c.ProjectToken == "" {
				t.Error("expected c.ProjectToken to be set after project token detection")
			}
			if !tt.wantProjectToken && c.ProjectToken != "" {
				t.Errorf("expected c.ProjectToken to be empty, got %q", c.ProjectToken)
			}
			if tt.wantWorkspaceToken != c.WorkspaceScopedToken {
				t.Errorf("c.WorkspaceScopedToken = %v, want %v", c.WorkspaceScopedToken, tt.wantWorkspaceToken)
			}
		})
	}
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

func TestDetectTokenType_CachedAfterFirstCall(t *testing.T) {
	var callCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"errors":[{"message":"Not Authorized"}]}`))
	}))
	defer server.Close()

	c := NewClient("test-token")
	c.apiURL = server.URL

	// Three separate callers that each trigger detection internally.
	_, _ = c.IsProjectToken()
	_, _ = c.IsWorkspaceToken()
	_, _ = c.GetWorkspaceID()

	// Exactly 3 API calls: one probe sequence (account + workspace + project), never repeated.
	if got := callCount.Load(); got != 3 {
		t.Errorf("expected exactly 3 API calls (one probe sequence), got %d", got)
	}
}

func TestGetProjectContext(t *testing.T) {
	t.Run("returns cached IDs without extra API call", func(t *testing.T) {
		var callCount atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			callCount.Add(1)
			w.Header().Set("Content-Type", "application/json")
			if r.Header.Get("Project-Access-Token") != "" {
				w.Write([]byte(`{"data":{"projectToken":{"projectId":"proj-123","environmentId":"env-456"}}}`))
			} else {
				w.Write([]byte(`{"errors":[{"message":"Not Authorized"}]}`))
			}
		}))
		defer server.Close()

		c := NewClient("test-token")
		c.apiURL = server.URL

		projectID, environmentID, err := c.GetProjectContext()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if projectID != "proj-123" {
			t.Errorf("projectID = %q, want %q", projectID, "proj-123")
		}
		if environmentID != "env-456" {
			t.Errorf("environmentID = %q, want %q", environmentID, "env-456")
		}
		// 3 probes during detection, 0 extra calls for GetProjectContext (uses cache)
		if got := callCount.Load(); got != 3 {
			t.Errorf("expected 3 API calls (detection only), got %d", got)
		}
	})

	t.Run("returns error for non-project token", func(t *testing.T) {
		server := makeTokenDetectionServer(t, false, true, false) // workspace token
		defer server.Close()

		c := NewClient("test-token")
		c.apiURL = server.URL

		_, _, err := c.GetProjectContext()
		if err == nil {
			t.Fatal("expected error for workspace token, got nil")
		}
	})
}

func TestGetWorkspaceID_ProjectToken(t *testing.T) {
	// Project token: GetWorkspaceID should return "" with no error.
	server := makeTokenDetectionServer(t, false, false, true)
	defer server.Close()

	c := NewClient("test-token")
	c.apiURL = server.URL

	id, err := c.GetWorkspaceID()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "" {
		t.Errorf("expected empty workspace ID for project token, got %q", id)
	}
}

func TestGetWorkspaceID_Warnings(t *testing.T) {
	tests := []struct {
		name            string
		accountWorks    bool
		workspaceWorks  bool
		projectWorks    bool
		workspaceHint   string
		wantWarnContain string
	}{
		{
			name:            "workspace token + -w flag prints warning",
			workspaceWorks:  true,
			workspaceHint:   "my-team",
			wantWarnContain: "workspace token is already scoped",
		},
		{
			name:            "project token + -w flag prints warning",
			projectWorks:    true,
			workspaceHint:   "my-team",
			wantWarnContain: "project token is already scoped",
		},
		{
			name:           "workspace token without -w flag prints no warning",
			workspaceWorks: true,
			workspaceHint:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := makeTokenDetectionServer(t, tt.accountWorks, tt.workspaceWorks, tt.projectWorks)
			defer server.Close()

			c := NewClient("test-token")
			c.apiURL = server.URL
			c.Workspace = tt.workspaceHint

			var buf bytes.Buffer
			c.WarnFn = func(msg string) { fmt.Fprintln(&buf, msg) }

			_, _ = c.GetWorkspaceID()

			stderr := buf.String()

			if tt.wantWarnContain != "" {
				if !strings.Contains(stderr, tt.wantWarnContain) {
					t.Errorf("expected warning to contain %q, got: %q", tt.wantWarnContain, stderr)
				}
			} else {
				if strings.Contains(stderr, "Warning:") {
					t.Errorf("expected no warning, got: %q", stderr)
				}
			}
		})
	}
}

func TestGetWorkspaceID_Unauthorized(t *testing.T) {
	// All three detection probes fail — simulates a completely invalid token.
	server := makeUnauthorizedWorkspaceServer(t)
	defer server.Close()

	for _, name := range []string{"no workspace hint", "with workspace hint"} {
		t.Run(name, func(t *testing.T) {
			c := NewClient("test-token")
			c.apiURL = server.URL
			if name == "with workspace hint" {
				c.Workspace = "my-team"
			}

			_, err := c.GetWorkspaceID()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(strings.ToLower(err.Error()), "not authorized") {
				t.Errorf("error %q should mention authorization", err.Error())
			}
		})
	}
}
