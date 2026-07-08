# railctl `token` Command Group — Implementation Plan

> **For implementers:** REQUIRED NEXT STEP — invoke one of the A0 execution skills:
> - `subagent-driven-development` (fresh subagent per task with two-stage review, recommended for substantive work)
> - `executing-plans` (batch execution with human checkpoints, for smaller plans)
> The choice is made by `decide-mode` or the user.

**Goal:** Add a `railctl token` command group (`create` / `list` / `delete`) that mints, lists, and revokes Railway project+environment-scoped access tokens.

**Architecture:** New `internal/api/tokens.go` wraps three GraphQL operations (`projectTokenCreate`, `projectTokens`, `projectTokenDelete`) behind `Client` methods added to the `APIClient` interface and `MockClient`. Four `internal/cmd` files add a noun-grouped `token` parent plus three subcommands, each using `cmdutil.ResolveContext` for project/env resolution and `cmdutil.PrintResult` for output. The minted token prints to stdout (note to stderr); `list` never exposes the raw value.

**Tech Stack:** Go 1.22, `spf13/cobra`, Railway GraphQL v2. TDD throughout (default).

**Branch:** `feat/token-minting` (already created off `origin/main`).

**Design:** `docs/designs/2026-07-08-railctl-token-minting.md`.

**Verify commands:** `make test` (or `go test ./...`) and `golangci-lint run` (the canonical lint gate).

---

## File Structure

| File | Responsibility |
|---|---|
| `internal/api/tokens.go` (create) | `ProjectToken` type + `CreateProjectToken` / `ListProjectTokens` / `DeleteProjectToken` client methods |
| `internal/api/tokens_client_test.go` (create) | httptest-backed tests for the three client methods |
| `internal/api/interface.go` (modify) | Add the three methods to `APIClient` |
| `internal/api/mock.go` (modify) | Add `…Func` fields + method stubs |
| `internal/cmd/token.go` (create) | `token` parent command, registered on `rootCmd` |
| `internal/cmd/token_create.go` (create) | `token create <name>` |
| `internal/cmd/token_list.go` (create) | `token list` + output structs/tables + `formatTokenTime` |
| `internal/cmd/token_delete.go` (create) | `token delete <id>` |
| `internal/cmd/token_test.go` (create) | command tests (create/list/delete incl. error, cancelled, not-found) |
| `README.md` (modify) | "Project Tokens" section |

---

## Task 1: API layer — client methods + client tests

**Files:**
- Create: `internal/api/tokens.go`
- Test: `internal/api/tokens_client_test.go`

- [ ] **Step 1: Write the failing client tests**

Create `internal/api/tokens_client_test.go`:

```go
package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// decodeTokenRequest reads a GraphQL request body into query + variables.
func decodeTokenRequest(t *testing.T, r *http.Request) (string, map[string]any) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("reading request body: %v", err)
	}
	var req struct {
		Query     string         `json:"query"`
		Variables map[string]any `json:"variables"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshaling request: %v", err)
	}
	return req.Query, req.Variables
}

func TestClient_CreateProjectToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, vars := decodeTokenRequest(t, r)
		input, ok := vars["input"].(map[string]any)
		if !ok {
			t.Fatalf("expected input object, got %T", vars["input"])
		}
		if input["projectId"] != "proj-1" || input["environmentId"] != "env-1" || input["name"] != "ci" {
			t.Errorf("unexpected input: %v", input)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"projectTokenCreate":"tok-secret-value"}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	tok, err := client.CreateProjectToken("proj-1", "env-1", "ci")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "tok-secret-value" {
		t.Errorf("expected token tok-secret-value, got %q", tok)
	}
}

func TestClient_ListProjectTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, vars := decodeTokenRequest(t, r)
		if vars["projectId"] != "proj-1" {
			t.Errorf("expected projectId proj-1, got %v", vars["projectId"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"projectTokens":{"edges":[
			{"node":{"id":"t1","name":"ci","environmentId":"env-1","createdAt":"2026-07-01T00:00:00Z","displayToken":"tok-****"}},
			{"node":{"id":"t2","name":"staging","environmentId":"env-2","createdAt":"2026-07-02T00:00:00Z","displayToken":"sta-****"}}
		]}}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	tokens, err := client.ListProjectTokens("proj-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 2 {
		t.Fatalf("expected 2 tokens, got %d", len(tokens))
	}
	if tokens[0].ID != "t1" || tokens[0].Name != "ci" || tokens[0].EnvironmentID != "env-1" {
		t.Errorf("unexpected first token: %+v", tokens[0])
	}
}

func TestClient_DeleteProjectToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, vars := decodeTokenRequest(t, r)
		if vars["id"] != "t1" {
			t.Errorf("expected id t1, got %v", vars["id"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data":{"projectTokenDelete":true}}`))
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.apiURL = server.URL

	if err := client.DeleteProjectToken("t1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/api/ -run TestClient_.*ProjectToken -v`
Expected: FAIL — `client.CreateProjectToken` / `ListProjectTokens` / `DeleteProjectToken` undefined.

- [ ] **Step 3: Write the client implementation**

Create `internal/api/tokens.go`:

```go
package api

import (
	"encoding/json"
	"fmt"
)

// ProjectToken is a project + environment-scoped access token. DisplayToken is
// masked by Railway; the raw value is returned only once, at creation.
type ProjectToken struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	EnvironmentID string `json:"environmentId"`
	CreatedAt     string `json:"createdAt"`
	DisplayToken  string `json:"displayToken"`
}

// CreateProjectToken mints a token scoped to one project + environment and
// returns the raw token value. The value is shown only once by Railway and
// cannot be retrieved later.
func (c *Client) CreateProjectToken(projectID, environmentID, name string) (string, error) {
	mutation := `
		mutation ProjectTokenCreate($input: ProjectTokenCreateInput!) {
			projectTokenCreate(input: $input)
		}
	`
	variables := map[string]any{
		"input": map[string]any{
			"projectId":     projectID,
			"environmentId": environmentID,
			"name":          name,
		},
	}

	data, err := c.execute(mutation, variables)
	if err != nil {
		return "", err
	}

	var result struct {
		Token string `json:"projectTokenCreate"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("failed to unmarshal response: %w", err)
	}
	return result.Token, nil
}

// ListProjectTokens returns the project tokens for a project, across all
// environments. Values are masked (DisplayToken); the raw value is never listed.
func (c *Client) ListProjectTokens(projectID string) ([]ProjectToken, error) {
	query := `
		query ProjectTokens($projectId: String!) {
			projectTokens(projectId: $projectId) {
				edges {
					node {
						id
						name
						environmentId
						createdAt
						displayToken
					}
				}
			}
		}
	`
	variables := map[string]any{"projectId": projectID}

	data, err := c.execute(query, variables)
	if err != nil {
		return nil, err
	}

	var result struct {
		ProjectTokens struct {
			Edges []struct {
				Node ProjectToken `json:"node"`
			} `json:"edges"`
		} `json:"projectTokens"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	tokens := make([]ProjectToken, 0, len(result.ProjectTokens.Edges))
	for _, e := range result.ProjectTokens.Edges {
		tokens = append(tokens, e.Node)
	}
	return tokens, nil
}

// DeleteProjectToken revokes a project token by ID.
func (c *Client) DeleteProjectToken(tokenID string) error {
	mutation := `
		mutation ProjectTokenDelete($id: String!) {
			projectTokenDelete(id: $id)
		}
	`
	variables := map[string]any{"id": tokenID}

	_, err := c.execute(mutation, variables)
	return err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/api/ -run TestClient_.*ProjectToken -v`
Expected: PASS (3 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/api/tokens.go internal/api/tokens_client_test.go
git commit -m "feat(api): add project token client methods (create/list/delete)"
```

---

## Task 2: Wire the interface + mock

**Files:**
- Modify: `internal/api/interface.go`
- Modify: `internal/api/mock.go`

TDD note: this task is pure interface plumbing — no behavior of its own. It's verified by compilation and by Task 4–6 tests. No standalone test.

- [ ] **Step 1: Add methods to the `APIClient` interface**

In `internal/api/interface.go`, add inside the `APIClient` interface (after the volume methods block, before `// Workspace`):

```go
	// Project tokens
	CreateProjectToken(projectID, environmentID, name string) (string, error)
	ListProjectTokens(projectID string) ([]ProjectToken, error)
	DeleteProjectToken(tokenID string) error
```

- [ ] **Step 2: Add `…Func` fields to `MockClient`**

In `internal/api/mock.go`, add to the `MockClient` struct (near the other volume `…Func` fields):

```go
	// Project tokens
	CreateProjectTokenFunc func(projectID, environmentID, name string) (string, error)
	ListProjectTokensFunc  func(projectID string) ([]ProjectToken, error)
	DeleteProjectTokenFunc func(tokenID string) error
```

- [ ] **Step 3: Add mock method implementations**

Append to `internal/api/mock.go`:

```go
func (m *MockClient) CreateProjectToken(projectID, environmentID, name string) (string, error) {
	if m.CreateProjectTokenFunc != nil {
		return m.CreateProjectTokenFunc(projectID, environmentID, name)
	}
	return "", nil
}

func (m *MockClient) ListProjectTokens(projectID string) ([]ProjectToken, error) {
	if m.ListProjectTokensFunc != nil {
		return m.ListProjectTokensFunc(projectID)
	}
	return nil, nil
}

func (m *MockClient) DeleteProjectToken(tokenID string) error {
	if m.DeleteProjectTokenFunc != nil {
		return m.DeleteProjectTokenFunc(tokenID)
	}
	return nil
}
```

- [ ] **Step 4: Verify it compiles**

Run: `go build ./...`
Expected: builds clean (MockClient still satisfies APIClient).

- [ ] **Step 5: Commit**

```bash
git add internal/api/interface.go internal/api/mock.go
git commit -m "feat(api): expose project token methods on APIClient + mock"
```

---

## Task 3: `token` parent command group

**Files:**
- Create: `internal/cmd/token.go`

TDD note: a parent Cobra group with no `RunE` has no behavior to unit-test; it's exercised by the subcommand tests. Verified by build + `--help`.

- [ ] **Step 1: Create the parent command**

Create `internal/cmd/token.go`:

```go
package cmd

import (
	"github.com/spf13/cobra"
)

// tokenCmd is the parent for project token management.
var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Manage project/environment access tokens",
	Long: `Create, list, and delete Railway project tokens.

A project token is scoped to a single project and environment — a much smaller
blast radius than an account or workspace token. Minting a token requires an
account or workspace token; a project-scoped token cannot create tokens.`,
	Example: `  railctl token create ci --project my-app --environment production
  railctl token list --project my-app
  railctl token delete <id> --project my-app`,
}

func init() {
	rootCmd.AddCommand(tokenCmd)
}
```

- [ ] **Step 2: Verify it registers**

Run: `go run ./cmd/railctl token --help`
Expected: prints the token group help with `Available Commands:` (empty for now) and the examples.

- [ ] **Step 3: Commit**

```bash
git add internal/cmd/token.go
git commit -m "feat(cmd): add token command group"
```

---

## Task 4: `token create`

**Files:**
- Create: `internal/cmd/token_create.go`
- Test: `internal/cmd/token_test.go` (created here; extended in Tasks 5–6)

- [ ] **Step 1: Write the failing tests**

Create `internal/cmd/token_test.go`:

```go
package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/types"
)

// tokenTestMock returns a MockClient wired with one project ("my-project")
// and one environment ("production"). IsProjectToken defaults to false.
func tokenTestMock() *api.MockClient {
	return &api.MockClient{
		ListProjectsFunc: func() ([]types.Project, error) {
			return []types.Project{{ID: "proj-1", Name: "my-project"}}, nil
		},
		ListEnvironmentsFunc: func(projectID string) ([]types.Environment, error) {
			return []types.Environment{{ID: "env-1", Name: "production"}}, nil
		},
	}
}

func TestRunTokenCreate_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origOutput := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		outputFormat = origOutput
	}()

	var capProject, capEnv, capName string
	mock := tokenTestMock()
	mock.CreateProjectTokenFunc = func(projectID, environmentID, name string) (string, error) {
		capProject, capEnv, capName = projectID, environmentID, name
		return "tok-secret-value", nil
	}

	token = "test-token"
	project = "my-project"
	environment = "production"
	outputFormat = "table"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	var stdout, stderr bytes.Buffer
	tokenCreateCmd.SetOut(&stdout)
	tokenCreateCmd.SetErr(&stderr)
	defer func() { tokenCreateCmd.SetOut(nil); tokenCreateCmd.SetErr(nil) }()

	if err := tokenCreateCmd.RunE(tokenCreateCmd, []string{"ci"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capProject != "proj-1" || capEnv != "env-1" || capName != "ci" {
		t.Errorf("unexpected mint args: project=%q env=%q name=%q", capProject, capEnv, capName)
	}
	// Token goes to stdout, ONLY the token (trailing newline allowed).
	if strings.TrimSpace(stdout.String()) != "tok-secret-value" {
		t.Errorf("stdout = %q, want just the token", stdout.String())
	}
	// The human note goes to stderr and must NOT contain the raw token.
	if !strings.Contains(stderr.String(), "will not be shown again") {
		t.Errorf("stderr missing the store-now note: %q", stderr.String())
	}
	if strings.Contains(stderr.String(), "tok-secret-value") {
		t.Errorf("stderr leaked the token value: %q", stderr.String())
	}
}

func TestRunTokenCreate_ProjectTokenRejected(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
	}()

	minted := false
	mock := tokenTestMock()
	mock.IsProjectTokenFunc = func() (bool, error) { return true, nil }
	mock.CreateProjectTokenFunc = func(projectID, environmentID, name string) (string, error) {
		minted = true
		return "should-not-happen", nil
	}

	token = "test-token"
	project = "my-project"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	err := tokenCreateCmd.RunE(tokenCreateCmd, []string{"ci"})
	if err == nil {
		t.Fatal("expected an error when run with a project-scoped token")
	}
	if !strings.Contains(err.Error(), "account or workspace token") {
		t.Errorf("error = %q, want it to mention 'account or workspace token'", err.Error())
	}
	if minted {
		t.Error("CreateProjectToken must not be called when using a project token")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cmd/ -run TestRunTokenCreate -v`
Expected: FAIL — `tokenCreateCmd` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/cmd/token_create.go`:

```go
package cmd

import (
	"fmt"

	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
)

var tokenCreateCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a project token scoped to a project + environment",
	Long: `Create a project token for a project and environment.

The raw token is printed to stdout and shown only once — store it immediately.
Minting requires an account or workspace token; a project-scoped token cannot
create tokens.`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl token create ci --project my-app --environment production
  TOKEN=$(railctl token create ci -p my-app -e production)`,
	RunE: runTokenCreate,
}

func init() {
	tokenCmd.AddCommand(tokenCreateCmd)
}

// tokenCreateOutput is the structured (-o json/yaml) form of a minted token.
type tokenCreateOutput struct {
	Name          string `json:"name" yaml:"name"`
	ProjectID     string `json:"projectId" yaml:"projectId"`
	EnvironmentID string `json:"environmentId" yaml:"environmentId"`
	Token         string `json:"token" yaml:"token"`
}

func runTokenCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	tkn, err := getToken()
	if err != nil {
		return err
	}
	client := newAPIClient(tkn)

	// Fast, actionable failure: a project-scoped token cannot mint tokens.
	if isProject, err := client.IsProjectToken(); err == nil && isProject {
		return fmt.Errorf("creating project tokens requires an account or workspace token; a project-scoped token cannot mint tokens")
	}

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		NeedEnvironment: true,
	})
	if err != nil {
		return err
	}

	value, err := client.CreateProjectToken(ctx.Project.ID, ctx.Environment.ID, name)
	if err != nil {
		return fmt.Errorf("failed to create project token: %w", err)
	}

	switch format {
	case output.FormatJSON, output.FormatYAML:
		return cmdutil.PrintResult(format, tokenCreateOutput{
			Name:          name,
			ProjectID:     ctx.Project.ID,
			EnvironmentID: ctx.Environment.ID,
			Token:         value,
		}, nil, nil, "")
	default:
		fmt.Fprintf(cmd.ErrOrStderr(),
			"Created project token '%s' (project %s / %s). Store it now — it will not be shown again.\n",
			name, ctx.Project.Name, ctx.Environment.Name)
		fmt.Fprintln(cmd.OutOrStdout(), value)
		return nil
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cmd/ -run TestRunTokenCreate -v`
Expected: PASS (2 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/token_create.go internal/cmd/token_test.go
git commit -m "feat(cmd): add token create with stdout token / stderr note"
```

---

## Task 5: `token list`

**Files:**
- Create: `internal/cmd/token_list.go`
- Test: `internal/cmd/token_test.go` (extend)

- [ ] **Step 1: Add the failing test**

Append to `internal/cmd/token_test.go`:

```go
func TestRunTokenList_JSON(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origEnvironment := environment
	origToken := token
	origOutput := outputFormat
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		environment = origEnvironment
		token = origToken
		outputFormat = origOutput
	}()

	called := false
	mock := tokenTestMock()
	mock.ListProjectTokensFunc = func(projectID string) ([]api.ProjectToken, error) {
		called = true
		if projectID != "proj-1" {
			t.Errorf("expected projectId proj-1, got %q", projectID)
		}
		return []api.ProjectToken{
			{ID: "t1", Name: "ci", EnvironmentID: "env-1", CreatedAt: "2026-07-01T00:00:00Z", DisplayToken: "tok-****"},
		}, nil
	}

	token = "test-token"
	project = "my-project"
	environment = ""
	outputFormat = "json"
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := tokenListCmd.RunE(tokenListCmd, []string{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected ListProjectTokens to be called")
	}
}

func TestFormatTokenTime(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty", "", "-"},
		{"valid RFC3339", "2026-07-01T13:45:00Z", "2026-07-01 13:45"},
		{"invalid falls back to raw", "not-a-time", "not-a-time"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatTokenTime(tt.in); got != tt.want {
				t.Errorf("formatTokenTime(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cmd/ -run 'TestRunTokenList|TestFormatTokenTime' -v`
Expected: FAIL — `tokenListCmd` / `formatTokenTime` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/cmd/token_list.go`:

```go
package cmd

import (
	"fmt"
	"time"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/kubenoops/railctl/internal/output"
	"github.com/spf13/cobra"
)

var tokenListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List project tokens",
	Long: `List the project tokens for a project. Token values are masked; use
'railctl token create' to mint a new one. Pass --environment to filter by
environment.`,
	Args: cobra.NoArgs,
	Example: `  railctl token list --project my-app
  railctl token list --project my-app --environment production
  railctl token list -p my-app -o json`,
	RunE: runTokenList,
}

func init() {
	tokenCmd.AddCommand(tokenListCmd)
}

func runTokenList(cmd *cobra.Command, args []string) error {
	format, err := getOutputFormat()
	if err != nil {
		return err
	}

	tkn, err := getToken()
	if err != nil {
		return err
	}
	client := newAPIClient(tkn)

	needEnv := getEnvironment() != ""
	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		NeedEnvironment: needEnv,
	})
	if err != nil {
		return err
	}

	tokens, err := client.ListProjectTokens(ctx.Project.ID)
	if err != nil {
		return err
	}

	if needEnv {
		var filtered []api.ProjectToken
		for _, tk := range tokens {
			if tk.EnvironmentID == ctx.Environment.ID {
				filtered = append(filtered, tk)
			}
		}
		tokens = filtered
	}

	// Best-effort environment id → name map for friendly output.
	envNames := map[string]string{}
	if envs, err := client.ListEnvironments(ctx.Project.ID); err == nil {
		for _, e := range envs {
			envNames[e.ID] = e.Name
		}
	}

	return cmdutil.PrintResult(
		format,
		tokensToOutput(tokens, envNames),
		tokensToTable(tokens, envNames),
		tokensToWideTable(tokens, envNames),
		fmt.Sprintf("No project tokens found for project '%s'.", ctx.Project.Name),
	)
}

// tokenOutput is the structured (-o json/yaml) form of a listed token.
type tokenOutput struct {
	Name        string `json:"name" yaml:"name"`
	Environment string `json:"environment" yaml:"environment"`
	ID          string `json:"id" yaml:"id"`
	CreatedAt   string `json:"createdAt" yaml:"createdAt"`
}

func envName(m map[string]string, id string) string {
	if n, ok := m[id]; ok && n != "" {
		return n
	}
	return id
}

func tokensToOutput(tokens []api.ProjectToken, envs map[string]string) []tokenOutput {
	result := make([]tokenOutput, len(tokens))
	for i, tk := range tokens {
		result[i] = tokenOutput{
			Name:        tk.Name,
			Environment: envName(envs, tk.EnvironmentID),
			ID:          tk.ID,
			CreatedAt:   tk.CreatedAt,
		}
	}
	return result
}

func tokensToTable(tokens []api.ProjectToken, envs map[string]string) *output.Table {
	table := output.NewTable("NAME", "ENVIRONMENT", "ID", "CREATED")
	for _, tk := range tokens {
		table.AddRow(tk.Name, envName(envs, tk.EnvironmentID), tk.ID, formatTokenTime(tk.CreatedAt))
	}
	return table
}

func tokensToWideTable(tokens []api.ProjectToken, envs map[string]string) *output.Table {
	table := output.NewTable("NAME", "ENVIRONMENT", "ID", "CREATED", "TOKEN")
	for _, tk := range tokens {
		table.AddRow(tk.Name, envName(envs, tk.EnvironmentID), tk.ID, formatTokenTime(tk.CreatedAt), tk.DisplayToken)
	}
	return table
}

// formatTokenTime renders an RFC3339 timestamp as "2006-01-02 15:04".
func formatTokenTime(ts string) string {
	if ts == "" {
		return "-"
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return t.Format("2006-01-02 15:04")
	}
	return ts
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/cmd/ -run 'TestRunTokenList|TestFormatTokenTime' -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/token_list.go internal/cmd/token_test.go
git commit -m "feat(cmd): add token list with masked values and env filter"
```

---

## Task 6: `token delete`

**Files:**
- Create: `internal/cmd/token_delete.go`
- Test: `internal/cmd/token_test.go` (extend)

- [ ] **Step 1: Add the failing tests**

Append to `internal/cmd/token_test.go`:

```go
func TestRunTokenDelete_Success(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origYes := tokenDeleteYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		tokenDeleteYes = origYes
	}()

	var capturedID string
	mock := tokenTestMock()
	mock.ListProjectTokensFunc = func(projectID string) ([]api.ProjectToken, error) {
		return []api.ProjectToken{{ID: "t1", Name: "ci", EnvironmentID: "env-1"}}, nil
	}
	mock.DeleteProjectTokenFunc = func(tokenID string) error {
		capturedID = tokenID
		return nil
	}

	token = "test-token"
	project = "my-project"
	tokenDeleteYes = true
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := tokenDeleteCmd.RunE(tokenDeleteCmd, []string{"t1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedID != "t1" {
		t.Errorf("expected delete of t1, got %q", capturedID)
	}
}

func TestRunTokenDelete_Cancelled(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origYes := tokenDeleteYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		tokenDeleteYes = origYes
		tokenDeleteCmd.SetIn(nil)
	}()

	deleteCalled := false
	mock := tokenTestMock()
	mock.ListProjectTokensFunc = func(projectID string) ([]api.ProjectToken, error) {
		return []api.ProjectToken{{ID: "t1", Name: "ci", EnvironmentID: "env-1"}}, nil
	}
	mock.DeleteProjectTokenFunc = func(tokenID string) error {
		deleteCalled = true
		return nil
	}

	token = "test-token"
	project = "my-project"
	tokenDeleteYes = false
	tokenDeleteCmd.SetIn(strings.NewReader("n\n"))
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := tokenDeleteCmd.RunE(tokenDeleteCmd, []string{"t1"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleteCalled {
		t.Error("expected delete to be cancelled, but DeleteProjectToken was called")
	}
}

func TestRunTokenDelete_NotFound(t *testing.T) {
	origAPIClient := newAPIClient
	origProject := project
	origToken := token
	origYes := tokenDeleteYes
	defer func() {
		newAPIClient = origAPIClient
		project = origProject
		token = origToken
		tokenDeleteYes = origYes
	}()

	mock := tokenTestMock()
	mock.ListProjectTokensFunc = func(projectID string) ([]api.ProjectToken, error) {
		return []api.ProjectToken{{ID: "t1", Name: "ci", EnvironmentID: "env-1"}}, nil
	}

	token = "test-token"
	project = "my-project"
	tokenDeleteYes = true
	newAPIClient = func(tkn string) api.APIClient { return mock }

	if err := tokenDeleteCmd.RunE(tokenDeleteCmd, []string{"nonexistent"}); err == nil {
		t.Error("expected error for unknown token id")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/cmd/ -run TestRunTokenDelete -v`
Expected: FAIL — `tokenDeleteCmd` / `tokenDeleteYes` undefined.

- [ ] **Step 3: Write the implementation**

Create `internal/cmd/token_delete.go`:

```go
package cmd

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/kubenoops/railctl/internal/api"
	"github.com/kubenoops/railctl/internal/cmdutil"
	"github.com/spf13/cobra"
)

var tokenDeleteYes bool

var tokenDeleteCmd = &cobra.Command{
	Use:   "delete <id>",
	Short: "Delete (revoke) a project token",
	Long: `Delete a project token by ID from a project.

This immediately revokes the token. This operation is irreversible.`,
	Args: cobra.ExactArgs(1),
	Example: `  railctl token delete <id> --project my-app
  railctl token delete <id> --project my-app --yes`,
	RunE: runTokenDelete,
}

func init() {
	tokenDeleteCmd.Flags().BoolVarP(&tokenDeleteYes, "yes", "y", false, "Skip confirmation prompt")
	tokenCmd.AddCommand(tokenDeleteCmd)
}

func runTokenDelete(cmd *cobra.Command, args []string) error {
	tokenID := args[0]

	tkn, err := getToken()
	if err != nil {
		return err
	}
	client := newAPIClient(tkn)

	ctx, err := cmdutil.ResolveContext(client, cmdutil.ResolveOpts{
		ProjectName:     getProject(),
		EnvironmentName: getEnvironment(),
		NeedEnvironment: false,
	})
	if err != nil {
		return err
	}

	// Resolve the token within the project for a friendly prompt + not-found error.
	tokens, err := client.ListProjectTokens(ctx.Project.ID)
	if err != nil {
		return err
	}
	var found *api.ProjectToken
	for i := range tokens {
		if tokens[i].ID == tokenID {
			found = &tokens[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("project token '%s' not found in project '%s'", tokenID, ctx.Project.Name)
	}

	if !tokenDeleteYes {
		fmt.Fprintf(cmd.OutOrStdout(), "Delete project token '%s' (%s) from project '%s'? [y/N]: ", found.Name, tokenID, ctx.Project.Name)
		reader := bufio.NewReader(cmd.InOrStdin())
		response, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}
		response = strings.ToLower(strings.TrimSpace(response))
		if response != "y" && response != "yes" {
			fmt.Fprintln(cmd.OutOrStdout(), "Deletion cancelled.")
			return nil
		}
	}

	if err := client.DeleteProjectToken(tokenID); err != nil {
		return fmt.Errorf("failed to delete project token: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Project token '%s' deleted.\n", found.Name)
	return nil
}
```

- [ ] **Step 4: Run tests + full package to verify they pass**

Run: `go test ./internal/cmd/ -run TestRunTokenDelete -v && go test ./...`
Expected: PASS (all packages green).

- [ ] **Step 5: Commit**

```bash
git add internal/cmd/token_delete.go internal/cmd/token_test.go
git commit -m "feat(cmd): add token delete with confirmation and not-found check"
```

---

## Task 7: README documentation

**Files:**
- Modify: `README.md`

TDD note: doc-only change. Verified by spot-check + `docs-guard` (no `RAILCTL_*` var added, so the env-var table is untouched).

- [ ] **Step 1: Add the "Project Tokens" section**

In `README.md`, add a new section (place it near the other resource sections, e.g. after "Environment Variables"):

```markdown
### Project Tokens

Project tokens are scoped to a single project **and** environment — a much
smaller blast radius than an account or workspace token. Minting requires an
account or workspace token (a project-scoped token cannot create tokens).

```bash
# Create a token (printed once, to stdout — store it immediately)
railctl token create ci -p my-app -e production
TOKEN=$(railctl token create ci -p my-app -e production)   # capture for scripts

# List a project's tokens (values are masked)
railctl token list -p my-app
railctl token list -p my-app -e production   # filter by environment
railctl token list -p my-app -o wide         # include the masked token column

# Delete (revoke) a token by ID
railctl token delete <id> -p my-app
railctl token delete <id> -p my-app --yes    # skip confirmation
```

> The raw token value is shown **only once**, at creation. `token list` never
> displays it — store it securely when you create it.
```

- [ ] **Step 2: Verify**

Run: `golangci-lint run && go test ./...`
Then spot-check: `go run ./cmd/railctl token --help` and `go run ./cmd/railctl token create --help` show complete help.
Expected: lint clean, tests green, help text complete.

- [ ] **Step 3: Commit**

```bash
git add README.md
git commit -m "docs: document the token command group"
```

---

## Self-Review

**Design coverage:**
- `token create` (stdout token / stderr note, `-o json/yaml`, project-token fast-fail) → Task 4 ✅
- `token list` (`projectTokens`, table/wide/json/yaml, `-e` filter, masked values) → Task 5 ✅
- `token delete` (`-p` required, name lookup, `[y/N]`, `--yes`, not-found) → Task 6 ✅
- API layer `CreateProjectToken`/`ListProjectTokens`/`DeleteProjectToken` + interface + mock → Tasks 1–2 ✅
- Auth actionable error → Task 4 ✅
- Secret handling (stdout-only token, stderr note, list never raw) → Task 4 (asserted in tests) ✅
- Tests: client (Task 1), create success + project-token error (Task 4), list json + formatTokenTime (Task 5), delete success/cancelled/not-found (Task 6) ✅
- README → Task 7 ✅
- Every API method wired to a command (no dead code) ✅

**Placeholder scan:** none — every code/test step contains complete code.

**Type consistency:** `ProjectToken{ID,Name,EnvironmentID,CreatedAt,DisplayToken}`, `CreateProjectToken(projectID,environmentID,name)→(string,error)`, `ListProjectTokens(projectID)→([]ProjectToken,error)`, `DeleteProjectToken(tokenID)→error`, `tokenCmd`/`tokenCreateCmd`/`tokenListCmd`/`tokenDeleteCmd`, `tokenDeleteYes`, `formatTokenTime`, `tokenTestMock` — names identical across all tasks. `cmdutil.ResolveContext`/`ResolveOpts`/`PrintResult`, `output.NewTable`/`FormatJSON`/`FormatYAML`, globals `token`/`project`/`environment`/`outputFormat`/`newAPIClient` and helpers `getToken`/`getProject`/`getEnvironment`/`getOutputFormat` all verified against the current codebase.

**Scope check:** single cohesive feature (one command group), one PR — appropriately tactical.
