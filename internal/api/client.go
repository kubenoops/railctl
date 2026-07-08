// Package api provides a client for the Railway GraphQL API.
package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/kubenoops/railctl/internal/resolver"
)

const (
	// DefaultAPIURL is the Railway GraphQL API endpoint.
	DefaultAPIURL = "https://backboard.railway.com/graphql/v2"
	// DefaultTimeout is the HTTP client timeout.
	DefaultTimeout = 60 * time.Second

	// Retry configuration for rate limiting
	MaxRetries        = 5
	InitialBackoff    = 1 * time.Second
	MaxBackoff        = 32 * time.Second
	BackoffMultiplier = 2.0
)

// errMsgNotAuthorized is the Railway API error message for invalid or insufficient token scope.
const errMsgNotAuthorized = "Not Authorized"

// TokenType represents the detected Railway API token scope.
type TokenType int

const (
	TokenTypeUnknown   TokenType = iota
	TokenTypeAccount             // personal token — can access me.workspaces
	TokenTypeWorkspace           // workspace-scoped token — can list projects but not workspaces
	TokenTypeProject             // project-scoped token — uses Project-Access-Token header
)

// Client provides methods for interacting with the Railway API.
type Client struct {
	token                string
	apiURL               string
	httpClient           *http.Client
	workspaceID          string          // cached resolved workspace ID
	workspaceResolved    bool            // true after first GetWorkspaceID() call
	Workspace            string          // workspace name provided by caller (-w flag / RAILCTL_WORKSPACE)
	ProjectToken         string          // set after detection when token is project-scoped
	WorkspaceScopedToken bool            // set after detection when token is workspace-scoped
	tokenType            TokenType       // result of detectTokenType()
	tokenTypeResolved    bool            // true after detectTokenType() completes (success or auth failure)
	tokenTypeErr         error           // non-nil when all detection probes failed
	cachedWorkspaceData  json.RawMessage // workspace response cached by detectTokenType() probe 1
	cachedProjectID      string          // project ID cached by detectTokenType() probe 3
	cachedEnvironmentID  string          // environment ID cached by detectTokenType() probe 3
	wsIdentityID         string          // own-workspace ID introspected by currentWorkspace()
	wsIdentityName       string          // own-workspace name introspected by currentWorkspace()
	wsIdentityFetched    bool            // true after currentWorkspace() completed
	wsHintChecked        bool            // true after the -w value was validated against the token scope
	Debug                bool            // enable debug logging
	WarnFn               func(string)    // called with warning messages; set by cmd layer to write to stderr
}

// NewClient creates a new Railway API client with the given token.
func NewClient(token string) *Client {
	return &Client{
		token:  token,
		apiURL: DefaultAPIURL,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// graphQLRequest represents a GraphQL request payload.
type graphQLRequest struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables,omitempty"`
}

// graphQLResponse represents a GraphQL response.
type graphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []graphQLError  `json:"errors,omitempty"`
}

// graphQLError represents a GraphQL error.
type graphQLError struct {
	Message string `json:"message"`
}

// isRateLimitError checks if an error message indicates rate limiting.
func isRateLimitError(errMsg string) bool {
	rateLimitPhrases := []string{
		"too quickly",
		"rate limit",
		"try again",
		"slow down",
	}

	errLower := strings.ToLower(errMsg)
	for _, phrase := range rateLimitPhrases {
		if strings.Contains(errLower, phrase) {
			return true
		}
	}
	return false
}

// calculateBackoff returns the backoff duration for a given attempt with jitter.
func calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: initialBackoff * (multiplier ^ attempt)
	backoff := float64(InitialBackoff)
	for i := 0; i < attempt; i++ {
		backoff *= BackoffMultiplier
	}

	// Cap at max backoff
	if backoff > float64(MaxBackoff) {
		backoff = float64(MaxBackoff)
	}

	// Add jitter (0-10% of backoff) to prevent thundering herd
	jitter := backoff * 0.1 * (float64(time.Now().UnixNano()%100) / 100.0)

	return time.Duration(backoff + jitter)
}

// redactVariables creates a copy of the variables map with sensitive data redacted.
func redactVariables(variables map[string]any) map[string]any {
	redacted := make(map[string]any)
	for k, v := range variables {
		lower := strings.ToLower(k)
		switch {
		case lower == "variables":
			// Redact all values in environment variables map
			if vMap, ok := v.(map[string]string); ok {
				redactedVMap := make(map[string]string)
				for k2 := range vMap {
					redactedVMap[k2] = "[REDACTED]"
				}
				redacted[k] = redactedVMap
			} else if vMap, ok := v.(map[string]any); ok {
				redactedVMap := make(map[string]any)
				for k2 := range vMap {
					redactedVMap[k2] = "[REDACTED]"
				}
				redacted[k] = redactedVMap
			} else {
				redacted[k] = "[REDACTED]"
			}
		case lower == "registrycredentials":
			if vMap, ok := v.(map[string]any); ok {
				redactedVMap := make(map[string]any)
				for k2, v2 := range vMap {
					if strings.ToLower(k2) == "password" {
						redactedVMap[k2] = "[REDACTED]"
					} else {
						redactedVMap[k2] = v2
					}
				}
				redacted[k] = redactedVMap
			} else {
				redacted[k] = "[REDACTED]"
			}
		case IsSensitiveKey(k):
			redacted[k] = "[REDACTED]"
		default:
			// Recursively redact maps
			if vMap, ok := v.(map[string]any); ok {
				redacted[k] = redactVariables(vMap)
			} else {
				redacted[k] = v
			}
		}
	}
	return redacted
}

// execute sends a GraphQL query to the Railway API and returns the data field.
// It automatically retries rate-limited requests with exponential backoff.
func (c *Client) execute(query string, variables map[string]any) (json.RawMessage, error) {
	reqBody := graphQLRequest{
		Query:     query,
		Variables: variables,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Debug logging: print request
	if c.Debug {
		fmt.Fprintf(os.Stderr, "\n[DEBUG] GraphQL Request:\n")
		fmt.Fprintf(os.Stderr, "URL: %s\n", c.apiURL)
		fmt.Fprintf(os.Stderr, "Query: %s\n", query)
		if len(variables) > 0 {
			redacted := redactVariables(variables)
			varsJSON, _ := json.MarshalIndent(redacted, "", "  ")
			fmt.Fprintf(os.Stderr, "Variables: %s\n", string(varsJSON))
		}
		fmt.Fprintf(os.Stderr, "\n")
	}

	var lastErr error

	// Retry loop with exponential backoff
	for attempt := 0; attempt <= MaxRetries; attempt++ {
		// Create new request for each attempt
		req, err := http.NewRequest(http.MethodPost, c.apiURL, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		if c.ProjectToken != "" {
			req.Header.Set("Project-Access-Token", c.ProjectToken)
		} else {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("API request failed: %w", err)
			// Network errors might be transient, retry them too
			if attempt < MaxRetries {
				time.Sleep(calculateBackoff(attempt))
				continue
			}
			return nil, lastErr
		}

		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			if attempt < MaxRetries {
				time.Sleep(calculateBackoff(attempt))
				continue
			}
			return nil, lastErr
		}

		// Debug logging: print response
		if c.Debug {
			fmt.Fprintf(os.Stderr, "[DEBUG] GraphQL Response:\n")
			fmt.Fprintf(os.Stderr, "Status: %d\n", resp.StatusCode)
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, respBody, "", "  "); err == nil {
				fmt.Fprintf(os.Stderr, "Body: %s\n", prettyJSON.String())
			} else {
				fmt.Fprintf(os.Stderr, "Body: %s\n", string(respBody))
			}
			fmt.Fprintf(os.Stderr, "\n")
		}

		// Detect non-JSON responses (rate-limit pages, WAF blocks, etc.)
		// before attempting to unmarshal.
		contentType := resp.Header.Get("Content-Type")
		isJSON := strings.Contains(contentType, "application/json")
		bodyStr := strings.TrimSpace(string(respBody))

		if resp.StatusCode == 429 || (resp.StatusCode >= 500 && resp.StatusCode < 600) {
			// Rate limited or server error — always retry these
			statusLabel := "server error"
			if resp.StatusCode == 429 {
				statusLabel = "rate limited"
			}
			lastErr = fmt.Errorf("API %s (HTTP %d). %s", statusLabel, resp.StatusCode, truncateBody(bodyStr, 200))
			if attempt < MaxRetries {
				backoff := calculateBackoff(attempt)
				fmt.Fprintf(os.Stderr, "⏳ %s, retrying in %v (attempt %d/%d)...\n",
					statusLabel, backoff.Round(time.Millisecond), attempt+1, MaxRetries)
				time.Sleep(backoff)
				continue
			}
			return nil, lastErr
		}

		if !isJSON || (len(bodyStr) > 0 && bodyStr[0] != '{' && bodyStr[0] != '[') {
			// Non-JSON response (HTML error page, plain text, etc.)
			lastErr = fmt.Errorf("API returned non-JSON response (HTTP %d): %s",
				resp.StatusCode, truncateBody(bodyStr, 200))
			if attempt < MaxRetries {
				backoff := calculateBackoff(attempt)
				fmt.Fprintf(os.Stderr, "⏳ Unexpected API response, retrying in %v (attempt %d/%d)...\n",
					backoff.Round(time.Millisecond), attempt+1, MaxRetries)
				time.Sleep(backoff)
				continue
			}
			return nil, lastErr
		}

		var gqlResp graphQLResponse
		if err := json.Unmarshal(respBody, &gqlResp); err != nil {
			lastErr = fmt.Errorf("failed to parse API response (HTTP %d): %s",
				resp.StatusCode, truncateBody(bodyStr, 200))
			if attempt < MaxRetries {
				time.Sleep(calculateBackoff(attempt))
				continue
			}
			return nil, lastErr
		}

		if len(gqlResp.Errors) > 0 {
			errMsg := gqlResp.Errors[0].Message
			lastErr = fmt.Errorf("API error: %s", errMsg)

			// Check if this is a rate limit error
			if isRateLimitError(errMsg) && attempt < MaxRetries {
				backoff := calculateBackoff(attempt)
				// Optional: log retry attempt
				// fmt.Fprintf(os.Stderr, "Rate limited, retrying in %v (attempt %d/%d)...\n", backoff, attempt+1, MaxRetries)
				time.Sleep(backoff)
				continue
			}

			// Not a rate limit error, or max retries exceeded
			return nil, lastErr
		}

		// Success!
		return gqlResp.Data, nil
	}

	return nil, lastErr
}

// executeWithProjectTokenHeader fires one request using c.token as a Project-Access-Token
// header instead of Authorization: Bearer. Used only during token-type detection.
func (c *Client) executeWithProjectTokenHeader(query string, variables map[string]any) (json.RawMessage, error) {
	savedProjectToken := c.ProjectToken
	c.ProjectToken = c.token
	defer func() {
		c.ProjectToken = savedProjectToken
	}()
	return c.execute(query, variables)
}

// truncateBody returns the first maxLen characters of s for use in error messages.
// It strips HTML tags and collapses whitespace for readability.
func truncateBody(s string, maxLen int) string {
	// Strip HTML tags
	var buf strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			buf.WriteByte(' ')
		case !inTag:
			buf.WriteRune(r)
		}
	}
	result := buf.String()

	// Collapse whitespace
	fields := strings.Fields(result)
	result = strings.Join(fields, " ")

	if len(result) > maxLen {
		return result[:maxLen] + "..."
	}
	return result
}

// workspaceQuery fetches all workspaces the token can access.
const workspaceQuery = `
query {
	me {
		workspaces {
			id
			name
		}
	}
}
`

// detectWorkspaceTokenQuery is the minimal probe used to check if a token can list
// projects. Workspace-scoped tokens succeed here (probe 2), having already failed the
// me.workspaces query in probe 1 which only account tokens can answer.
const detectWorkspaceTokenQuery = `
query {
	projects {
		edges {
			node { id }
		}
	}
}
`

// workspaceEntry is a single workspace from the me query.
type workspaceEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// workspaceResponse represents the response from the workspace query.
type workspaceResponse struct {
	Me struct {
		Workspaces []workspaceEntry `json:"workspaces"`
	} `json:"me"`
}

// IsProjectToken reports whether the client is using a project-scoped token.
// Triggers lazy token-type detection on first call; detection errors are returned.
func (c *Client) IsProjectToken() (bool, error) {
	if !c.tokenTypeResolved {
		if _, err := c.detectTokenType(); err != nil {
			return false, err
		}
	}
	if c.tokenTypeErr != nil {
		return false, c.tokenTypeErr
	}
	return c.ProjectToken != "", nil
}

// IsWorkspaceToken reports whether the client is using a workspace-scoped token.
// Triggers lazy token-type detection on first call; detection errors are returned.
func (c *Client) IsWorkspaceToken() (bool, error) {
	if !c.tokenTypeResolved {
		if _, err := c.detectTokenType(); err != nil {
			return false, err
		}
	}
	if c.tokenTypeErr != nil {
		return false, c.tokenTypeErr
	}
	return c.WorkspaceScopedToken, nil
}

// GetWorkspaceID resolves and returns the workspace ID for the current token.
// Triggers lazy token-type detection on first call. Resolution order:
//  1. Cached value from a previous call
//  2. c.Workspace (set from -w flag or RAILCTL_WORKSPACE env var) — resolved by name
//  3. Auto-detect: use the single workspace if exactly one exists; error if multiple
func (c *Client) GetWorkspaceID() (string, error) {
	if c.workspaceResolved {
		return c.workspaceID, nil
	}

	if !c.tokenTypeResolved {
		if _, err := c.detectTokenType(); err != nil {
			return "", err
		}
	}
	if c.tokenTypeErr != nil {
		return "", c.tokenTypeErr
	}

	// Project tokens have no resolvable workspace ID (their -w contradiction
	// check lives in GetProjectContext, the path every project-token command
	// resolves through).
	if c.ProjectToken != "" {
		c.workspaceResolved = true
		return "", nil
	}

	// Workspace-scoped tokens are pinned to their own workspace, so there is
	// no ID to resolve — but a -w value naming a DIFFERENT workspace is a
	// contradiction and must fail fast rather than be silently ignored.
	if c.WorkspaceScopedToken {
		if err := c.checkWorkspaceHint(); err != nil {
			return "", err
		}
		c.workspaceResolved = true
		return "", nil
	}

	// Use workspace data cached by detectTokenType() — avoids a second roundtrip
	var resp workspaceResponse
	if err := json.Unmarshal(c.cachedWorkspaceData, &resp); err != nil {
		return "", fmt.Errorf("failed to parse workspace response: %w", err)
	}

	workspaces := resp.Me.Workspaces
	hint := c.Workspace

	if hint != "" {
		resources := make([]resolver.Resource, len(workspaces))
		for i, ws := range workspaces {
			resources[i] = resolver.Resource{ID: ws.ID, Name: ws.Name}
		}
		id, _, err := resolver.ResolveWithName(hint, resources)
		if err != nil {
			var nf resolver.ErrNotFound
			if errors.As(err, &nf) {
				// nf.Available already holds the workspace names (input order).
				return "", resolver.ErrNotFound{Resource: "workspace", Name: nf.Name, Available: nf.Available}
			}
			var amb resolver.ErrAmbiguous
			if errors.As(err, &amb) {
				return "", resolver.ErrAmbiguous{Resource: "workspace", Name: amb.Name, Matches: amb.Matches}
			}
			return "", err
		}
		c.workspaceID = id
		c.workspaceResolved = true
		return c.workspaceID, nil
	}

	switch len(workspaces) {
	case 0:
		c.workspaceResolved = true
		return "", nil
	case 1:
		c.workspaceID = workspaces[0].ID
		c.workspaceResolved = true
		return c.workspaceID, nil
	default:
		return "", fmt.Errorf("multiple workspaces found (%s): use -w <name> or set RAILCTL_WORKSPACE=<name>",
			joinWorkspaceNames(workspaces))
	}
}

func joinWorkspaceNames(workspaces []workspaceEntry) string {
	names := make([]string, len(workspaces))
	for i, ws := range workspaces {
		names[i] = ws.Name
	}
	return strings.Join(names, ", ")
}

// detectTokenType probes the Railway API to determine the token scope and caches the
// result. It sets c.ProjectToken or c.WorkspaceScopedToken as a side-effect so all
// subsequent calls automatically use the correct auth header.
//
// Probe sequence:
//  1. Bearer + me.workspaces    → account token
//  2. Bearer + projects listing → workspace-scoped token
//  3. Project-Access-Token + projectToken query → project-scoped token
//  4. All fail → error
func (c *Client) detectTokenType() (TokenType, error) {
	if c.tokenTypeResolved {
		return c.tokenType, c.tokenTypeErr
	}

	// Probe 1: account token
	data, err := c.execute(workspaceQuery, nil)
	if err == nil {
		c.tokenType = TokenTypeAccount
		c.cachedWorkspaceData = data
		c.tokenTypeResolved = true
		return c.tokenType, nil
	}
	if !strings.Contains(err.Error(), errMsgNotAuthorized) {
		return TokenTypeUnknown, err
	}

	// Probe 2: workspace-scoped token. A -w value is NOT validated here —
	// detection is a probe path used by everything; the contradiction check
	// happens lazily in GetWorkspaceID (via checkWorkspaceHint).
	_, err = c.execute(detectWorkspaceTokenQuery, nil)
	if err == nil {
		c.tokenType = TokenTypeWorkspace
		c.WorkspaceScopedToken = true
		c.tokenTypeResolved = true
		return c.tokenType, nil
	}
	if !strings.Contains(err.Error(), errMsgNotAuthorized) {
		return TokenTypeUnknown, err
	}

	// Probe 3: project-scoped token (different HTTP header)
	data, err = c.executeWithProjectTokenHeader(projectTokenQuery, nil)
	if err == nil {
		var resp projectTokenContext
		if jsonErr := json.Unmarshal(data, &resp); jsonErr != nil {
			return TokenTypeUnknown, fmt.Errorf("failed to parse project token response: %w", jsonErr)
		}
		// A -w value is NOT validated here — the contradiction check happens
		// lazily in GetProjectContext (via checkWorkspaceHint).
		if resp.ProjectToken.ProjectID != "" {
			c.tokenType = TokenTypeProject
			c.ProjectToken = c.token
			c.cachedProjectID = resp.ProjectToken.ProjectID
			c.cachedEnvironmentID = resp.ProjectToken.EnvironmentID
			c.tokenTypeResolved = true
			return c.tokenType, nil
		}
	} else if !strings.Contains(err.Error(), errMsgNotAuthorized) {
		return TokenTypeUnknown, err
	}

	// Cache the failure so subsequent calls don't re-probe.
	c.tokenTypeErr = fmt.Errorf("token is not authorized")
	c.tokenTypeResolved = true
	return TokenTypeUnknown, c.tokenTypeErr
}

// projectTokenQuery retrieves the project and environment IDs for a project-scoped token.
const projectTokenQuery = `
query {
	projectToken {
		projectId
		environmentId
	}
}
`

type projectTokenContext struct {
	ProjectToken struct {
		ProjectID     string `json:"projectId"`
		EnvironmentID string `json:"environmentId"`
	} `json:"projectToken"`
}

// GetProjectContext returns the project and environment IDs associated with the project token.
// Triggers lazy token-type detection if not yet resolved; IDs are cached from probe 3 so no
// additional API call is made. When a -w/RAILCTL_WORKSPACE value is set it is validated
// against the workspace the token actually belongs to (contradictions fail fast).
func (c *Client) GetProjectContext() (projectID, environmentID string, err error) {
	if !c.tokenTypeResolved {
		if _, err := c.detectTokenType(); err != nil {
			return "", "", err
		}
	}
	if c.tokenTypeErr != nil {
		return "", "", c.tokenTypeErr
	}
	if c.tokenType != TokenTypeProject {
		return "", "", fmt.Errorf("token is not a project-scoped token")
	}
	if err := c.checkWorkspaceHint(); err != nil {
		return "", "", err
	}
	return c.cachedProjectID, c.cachedEnvironmentID, nil
}

// apiTokenWorkspacesQuery introspects the identity of the workspace a
// WORKSPACE-scoped token is bound to. The apiToken query is the token's
// self-introspection endpoint: with a workspace token it returns exactly the
// one workspace the token is scoped to (with an account token, all of the
// account's workspaces).
const apiTokenWorkspacesQuery = `
query {
	apiToken {
		workspaces {
			id
			name
		}
	}
}
`

// apiTokenWorkspacesResponse represents the response from apiTokenWorkspacesQuery.
type apiTokenWorkspacesResponse struct {
	APIToken struct {
		Workspaces []workspaceEntry `json:"workspaces"`
	} `json:"apiToken"`
}

// projectTokenWorkspaceQuery introspects the identity of the workspace a
// PROJECT-scoped token is contained in. The direct workspace(id) query is
// denied for project tokens; the nested path through the token's own project
// is the only sanctioned route.
const projectTokenWorkspaceQuery = `
query {
	projectToken {
		project {
			workspace {
				id
				name
			}
		}
	}
}
`

// projectTokenWorkspaceResponse represents the response from projectTokenWorkspaceQuery.
type projectTokenWorkspaceResponse struct {
	ProjectToken struct {
		Project struct {
			Workspace workspaceEntry `json:"workspace"`
		} `json:"project"`
	} `json:"projectToken"`
}

// currentWorkspace resolves the identity (id, name) of the workspace the
// current workspace- or project-scoped token is bound to: workspace tokens
// self-identify via apiToken, project tokens via the nested path through
// their own project (the direct workspace(id) query is denied for them).
// Returns empty strings without error only if the API unexpectedly reports
// no workspace. The result is cached; at most one API call is made per client.
func (c *Client) currentWorkspace() (id, name string, err error) {
	if c.wsIdentityFetched {
		return c.wsIdentityID, c.wsIdentityName, nil
	}

	var ws workspaceEntry
	if c.ProjectToken != "" {
		data, err := c.execute(projectTokenWorkspaceQuery, nil)
		if err != nil {
			return "", "", fmt.Errorf("failed to introspect the token's workspace: %w", err)
		}
		var resp projectTokenWorkspaceResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return "", "", fmt.Errorf("failed to parse workspace identity response: %w", err)
		}
		ws = resp.ProjectToken.Project.Workspace
	} else {
		data, err := c.execute(apiTokenWorkspacesQuery, nil)
		if err != nil {
			return "", "", fmt.Errorf("failed to introspect the token's workspace: %w", err)
		}
		var resp apiTokenWorkspacesResponse
		if err := json.Unmarshal(data, &resp); err != nil {
			return "", "", fmt.Errorf("failed to parse workspace identity response: %w", err)
		}
		if len(resp.APIToken.Workspaces) > 0 {
			ws = resp.APIToken.Workspaces[0]
		}
	}

	c.wsIdentityID = ws.ID
	c.wsIdentityName = ws.Name
	c.wsIdentityFetched = true
	return c.wsIdentityID, c.wsIdentityName, nil
}

// checkWorkspaceHint validates the -w/RAILCTL_WORKSPACE value against the
// workspace the token is actually scoped to. A matching value (by ID, exact
// name, or unique substring) proceeds silently; a mismatch is a
// contradiction and fails fast. Purely defensively, if the API reports no
// workspace identity at all, it falls back to the historical
// warn-and-proceed with a note that verification was impossible. The check
// runs at most once per client and is a no-op without a -w value.
func (c *Client) checkWorkspaceHint() error {
	if c.Workspace == "" || c.wsHintChecked {
		return nil
	}
	id, name, err := c.currentWorkspace()
	if err != nil {
		return err
	}
	if id == "" {
		if c.WarnFn != nil {
			c.WarnFn("Warning: -w/RAILCTL_WORKSPACE ignored — the token is already scoped to a specific workspace (identity could not be verified)")
		}
		c.wsHintChecked = true
		return nil
	}
	if c.Workspace != id {
		if _, _, rErr := resolver.ResolveWithName(c.Workspace, []resolver.Resource{{ID: id, Name: name}}); rErr != nil {
			return fmt.Errorf("token is scoped to workspace '%s' but -w/--workspace '%s' was given — refusing to proceed", name, c.Workspace)
		}
	}
	c.wsHintChecked = true
	return nil
}
