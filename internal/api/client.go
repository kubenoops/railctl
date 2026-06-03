// Package api provides a client for the Railway GraphQL API.
package api

import (
	"bytes"
	"encoding/json"
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

// Client provides methods for interacting with the Railway API.
type Client struct {
	token       string
	apiURL      string
	httpClient  *http.Client
	workspaceID string // cached resolved workspace ID
	Workspace   string // workspace name provided by caller (-w flag / RAILCTL_WORKSPACE)
	Debug       bool   // enable debug logging
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
		req.Header.Set("Authorization", "Bearer "+c.token)

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

// GetWorkspaceID resolves and returns the workspace ID for the current token.
// Resolution order:
//  1. Cached value from a previous call
//  2. c.Workspace (set from -w flag or RAILCTL_WORKSPACE env var) — resolved by name
//  3. Auto-detect: use the single workspace if exactly one exists; error if multiple
func (c *Client) GetWorkspaceID() (string, error) {
	if c.workspaceID != "" {
		return c.workspaceID, nil
	}

	// Determine the workspace hint: name or ID from caller or env var
	hint := c.Workspace
	if hint == "" {
		hint = os.Getenv("RAILCTL_WORKSPACE")
	}

	data, err := c.execute(workspaceQuery, nil)
	if err != nil {
		if strings.Contains(err.Error(), "Not Authorized") {
			return "", nil
		}
		return "", err
	}

	var resp workspaceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", fmt.Errorf("failed to parse workspace response: %w", err)
	}

	workspaces := resp.Me.Workspaces

	// If a hint was given, resolve by name using the standard exact → substring pattern.
	if hint != "" {
		// Exact match first
		for _, ws := range workspaces {
			if ws.Name == hint {
				c.workspaceID = ws.ID
				return c.workspaceID, nil
			}
		}

		// Case-insensitive substring match
		hintLower := strings.ToLower(hint)
		var matches []workspaceEntry
		for _, ws := range workspaces {
			if strings.Contains(strings.ToLower(ws.Name), hintLower) {
				matches = append(matches, ws)
			}
		}

		switch len(matches) {
		case 0:
			return "", resolver.ErrNotFound{Resource: "workspace", Name: hint}
		case 1:
			c.workspaceID = matches[0].ID
			return c.workspaceID, nil
		default:
			names := make([]string, len(matches))
			for i, m := range matches {
				names[i] = m.Name
			}
			return "", resolver.ErrAmbiguous{Resource: "workspace", Name: hint, Matches: names}
		}
	}

	// No hint — auto-detect
	switch len(workspaces) {
	case 0:
		return "", nil
	case 1:
		c.workspaceID = workspaces[0].ID
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
