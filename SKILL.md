---
name: Railway CLI Development
description: Guidelines for developing the Railway CLI (railctl) - a Go-based CLI tool for Railway.app infrastructure management
---

# Railway CLI Development Skill

This skill provides guidelines for developing features in the `railctl` project, a Go-based CLI tool for managing Railway.app infrastructure.

## Project Structure

```
railctl/
├── cmd/railctl/             # Go CLI main entry point
├── internal/                # Go implementation (primary focus)
│   ├── api/                 # Railway GraphQL API client (+ mock client)
│   ├── apply/               # Declarative config apply engine
│   ├── cmd/                 # Cobra command implementations
│   ├── cmdutil/             # Shared command scaffolding (ResolveContext, PrintResult)
│   ├── config/              # Declarative config parsing
│   ├── diff/                # Drift diff engine
│   ├── output/              # Output formatting (table, JSON, YAML, wide)
│   ├── resolver/            # Name/ID resolution logic (exact → substring → ambiguous)
│   ├── skill/               # Embedded copy of docs/railctl-skill.md
│   ├── sshx/                # SSH exec/port-forward helpers
│   └── types/               # Data structures
├── tests/e2e/               # Live-Railway E2E tests (account/workspace/project groups, build tag: e2e)
├── examples/                # Deployment examples (n8n, temporal, shared scripts)
├── experiments/             # Legacy Python prototypes — not part of the build or CI, reference only
├── docs/                    # railctl-skill.md, declarative-config.md, testing-architecture.md, etc.
├── go.mod                   # Go module definition
├── go.sum                   # Go dependencies
├── SKILL.md                 # This file - development guidelines
└── README.md                # Project documentation
```

## Development Methodology

### 1. Research Phase
**Always start by understanding the existing implementation:**

1. **Review existing Go code**:
   - Check `internal/api/` for similar API patterns
   - Look at `internal/cmd/` for command structure examples
   - Review `internal/types/` for data structures

2. **Check the docs**:
   - `docs/token-capability-matrix.md` for what each Railway token scope can do
   - `docs/testing-architecture.md` and `tests/e2e/README.md` for existing E2E coverage
   - `docs/designs/` for prior design decisions on related features

3. **Study field types as needed** by querying the live Railway GraphQL API directly
   (see Debugging Workflow below) — there is no local copy of the schema.

### 2. Implementation Pattern

#### API Layer (`internal/api/`)

**GraphQL Queries/Mutations:**
```go
// Always define as constants with clear documentation
const queryName = `
query($param: Type!) {
    field(param: $param) {
        id
        name
        # Include all fields needed
    }
}
`
```

**Response Types:**
```go
// Mirror the GraphQL response structure exactly
type responseType struct {
    Field struct {
        ID   string `json:"id"`
        Name string `json:"name"`
    } `json:"field"`
}
```

**Client Methods:**
```go
// Follow this pattern for all API methods
func (c *Client) MethodName(params) (types.Result, error) {
    data, err := c.execute(queryName, map[string]any{
        "param": value,
    })
    if err != nil {
        return nil, err
    }

    var resp responseType
    if err := json.Unmarshal(data, &resp); err != nil {
        return nil, err
    }

    // Transform to types.Result
    return transformFunction(resp), nil
}
```

#### Command Layer (`internal/cmd/`)

**Command Structure:**
```go
var commandCmd = &cobra.Command{
    Use:     "command <arg> [flags]",
    Aliases: []string{"alias"},  // Always provide short aliases
    Short:   "Brief description",
    Long:    `Detailed description with examples.
    
For features requiring env vars, document them here.`,
    Args:    cobra.ExactArgs(1),
    Example: `  railctl command example -p project
  railctl command example --flag value`,
    RunE:    runCommand,
}
```

**Flag Patterns:**
```go
func init() {
    // Document env var fallbacks in flag descriptions
    cmd.Flags().StringVar(&flag, "flag", "", "Description (env: ENV_VAR_NAME)")
    
    // Only mark truly required flags
    cmd.MarkFlagRequired("required-flag")
}
```

**Command Implementation:**
```go
func runCommand(cmd *cobra.Command, args []string) error {
    // 1. Get token
    token, err := getToken()
    if err != nil {
        return err
    }

    // 2. Validate required flags/env vars
    if requiredFlag == "" {
        return fmt.Errorf("flag is required")
    }

    // 3. Create API client
    client := newAPIClient(token)

    // 4. Resolve names to IDs using resolver package
    project, err := resolver.ResolveProject(projects, projectFlag)
    if err != nil {
        return err
    }

    // 5. Execute API calls
    result, err := client.Method(params)
    if err != nil {
        return fmt.Errorf("failed to ...: %w", err)
    }

    // 6. Format output
    format := getOutputFormat()
    switch format {
    case output.FormatJSON:
        // JSON output
    case output.FormatYAML:
        // YAML output
    default:
        // Human-readable output
    }

    return nil
}
```

### 3. Interface & Mock Updates

**Always update in this order:**

1. **Interface** (`internal/api/interface.go`):
   ```go
   type APIClient interface {
       NewMethod(params) (result, error)
   }
   ```

2. **Implementation** (`internal/api/*.go`):
   ```go
   func (c *Client) NewMethod(params) (result, error) {
       // Implementation
   }
   ```

3. **Mock** (`internal/api/mock.go`):
   ```go
   type MockClient struct {
       NewMethodFunc func(params) (result, error)
   }

   func (m *MockClient) NewMethod(params) (result, error) {
       if m.NewMethodFunc != nil {
           return m.NewMethodFunc(params)
       }
       return defaultValue, nil
   }
   ```

### 4. Testing Requirements

**Always run tests before committing:**
```bash
go build -o railctl ./cmd/railctl
go test ./...
```

**Test coverage expectations:**
- Unit tests for complex logic
- Integration tests for API interactions (using mocks)
- Manual testing with real Railway API
- **E2E tests** for command additions or flag changes (see below)

#### E2E Test Suite

**Location:** `tests/e2e/{account,workspace,project}` — three Go test groups
keyed to Railway token scope (shared harness in `tests/e2e/harness/`); see
`tests/e2e/README.md`. Pick the group by what the token type can do: workspace
enumeration → `account/`, project/env lifecycle + minting → `workspace/`,
everything in-scope + boundary fail-fasts → `project/` (the bulk).

**When to update E2E tests:**
- Adding a new command (e.g., `railctl get foo`)
- Adding new flags to existing commands (e.g., `--new-flag`)
- Changing flag behavior or output format
- Modifying error messages or validation logic
- Adding new output formats

**Running E2E tests:**
```bash
# Build the binary
go build -o railctl ./cmd/railctl

# Set up tokens (each group runs under exactly its token type)
export RAILWAY_ACCOUNT_TOKEN="..."
export RAILWAY_WORKSPACE_TOKEN="..."

# Run the suite (or one group)
make test-e2e
make test-e2e-project

# One test directly
RAILCTL=$(pwd)/railctl RAILWAY_WORKSPACE_TOKEN=... \
  go test -tags e2e -v -run TestBoundaries ./tests/e2e/project/...

# Keep resources on failure for manual inspection
E2E_KEEP=1 make test-e2e-project
```

**What to add when creating a new command:**
1. Add test cases for the command in the appropriate phase function
2. Test all output formats: table, wide, json, yaml
3. Test success cases with various flag combinations
4. Test error cases (missing flags, invalid inputs, nonexistent resources)
5. If the command affects deployments, update `test_deployment_lifecycle()`
6. Update the test count in `tests/e2e/README.md`

**Example pattern for adding tests:**
```bash
# In the appropriate test_* function:

# Success case
_test_header "new command with --flag"
rcpe new command --flag value
assert_success "new command"

# Output formats
_test_header "new command -o json"
rcpe new command -o json
assert_valid_json "new command -o json"

# Error case
_test_header "new command without required flag (expect error)"
rcpe new command
assert_failure "new command missing flag"
```

See `tests/e2e/README.md` for full documentation on the E2E test suite structure and coverage.

### 5. Output Formatting

**Support three output formats:**

1. **Table** (default) - Human-readable, aligned columns
2. **JSON** (`-o json`) - Machine-readable
3. **YAML** (`-o yaml`) - Configuration-friendly

**Pattern:**
```go
type outputStruct struct {
    Field string `json:"field" yaml:"field"`
}

func toOutput(input types.Data) outputStruct {
    return outputStruct{
        Field: input.Field,
    }
}
```

### 6. Environment Variable Support

**Pattern for all credentials/config:**
```go
// 1. Check flag first
value := flagValue

// 2. Fall back to env var
if value == "" {
    value = os.Getenv("RAILCTL_VAR_NAME")
}

// 3. Validate if required
if value == "" {
    return fmt.Errorf("value required via --flag or RAILCTL_VAR_NAME")
}
```

**Document in:**
- Flag description: `"Description (env: RAILCTL_VAR_NAME)"`
- Command Long description
- README.md

### 7. Error Handling

**Patterns:**
```go
// Wrap errors with context
return fmt.Errorf("failed to fetch service: %w", err)

// User-friendly messages
return fmt.Errorf("service '%s' not found in environment", name)

// Validation errors
if image == "" && creds == nil {
    return fmt.Errorf("at least one of --image or registry credentials is required")
}
```

### 8. Commit Message Format

**Use conventional commits, scoped to the affected package or area:**
```
feat(cmd): add feature description
fix(api): fix issue description
docs(skill): update documentation
test(e2e): add tests for feature
```

A scope is optional for changes that span multiple areas (`feat: add deployment status to services`).

**Multi-line commits for complex changes:**
```
feat(api): add deployment status to services

- Add GetBuildLogs API method
- Update ServiceDetail struct with status fields
- Show deployment errors in describe command
- Add tests for new functionality
```

## Common Patterns

### Name/ID Resolution
```go
// Use resolver package for flexible name/ID matching
import "github.com/kubenoops/railctl/internal/resolver"

project, err := resolver.ResolveProject(projects, userInput)
env, err := resolver.ResolveEnvironment(environments, userInput)
```

### Relative Time Display
```go
import "github.com/kubenoops/railctl/internal/types"

// Use RelativeTime for human-friendly timestamps
fmt.Printf("Updated: %s\n", types.RelativeTime(service.UpdatedAt))
```

### Credential Handling
```go
// Always support both flags and env vars
type RegistryCredentials struct {
    Username string
    Password string
}

func getCredentials(flagUser, flagPass string) *RegistryCredentials {
    user := flagUser
    if user == "" {
        user = os.Getenv("RAILCTL_REGISTRY_USERNAME")
    }
    
    pass := flagPass
    if pass == "" {
        pass = os.Getenv("RAILCTL_REGISTRY_PASSWORD")
    }
    
    if user != "" && pass != "" {
        return &RegistryCredentials{
            Username: user,
            Password: pass,
        }
    }
    return nil
}
```

## Debugging Workflow

### 1. API Issues
```bash
# Test GraphQL directly with curl
curl -X POST https://backboard.railway.app/graphql/v2 \
  -H "Authorization: Bearer $RAILWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "query { ... }"}'
```

### 2. Schema Introspection
```bash
# Query the live schema directly — there is no local schema.json
curl -X POST https://backboard.railway.app/graphql/v2 \
  -H "Authorization: Bearer $RAILWAY_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"query": "query { __type(name: \"TypeName\") { fields { name type { name kind } } } }"}'
```

### 3. Compare with Existing Go Code
```bash
# Check how a similar query/mutation is already handled
rg "mutationName" internal/api/
rg "queryName" internal/api/
```

## Key Learnings from This Session

### Railway API Specifics

1. **Deployment Redeployment:**
   - Use `deploymentRedeploy(id: $deploymentID)` mutation
   - NOT `serviceInstanceRedeploy` (doesn't exist)
   - Requires the deployment ID, not service/environment IDs

2. **Service Updates:**
   - `serviceInstanceUpdate` automatically triggers deployment
   - No need to manually call redeploy after update
   - Can update image, credentials, or both

3. **Error Messages:**
   - Deployment errors are in build logs, not in `meta` field
   - `meta` contains deployment configuration, not error messages
   - Use `buildLogs` query to fetch error details

4. **Credentials:**
   - Private registry credentials go in `registryCredentials` field
   - Format: `{username: string, password: string}`
   - Only include if both username and password are provided

### Go-Specific Patterns

1. **JSON Handling:**
   ```go
   // Use any for flexible JSON structures
   Meta any `json:"meta"`
   
   // Type assert when accessing
   if meta, ok := field.Meta.(map[string]any); ok {
       // Access nested fields
   }
   ```

2. **Optional Fields:**
   ```go
   // Use omitempty for optional JSON fields
   Field string `json:"field,omitempty"`
   
   // Use pointers for optional struct fields
   LatestDeployment *struct {
       ID string `json:"id"`
   } `json:"latestDeployment"`
   ```

3. **GraphQL Input:**
   ```go
   // Build input dynamically
   input := map[string]any{}
   if value != "" {
       input["field"] = value
   }
   ```

## Recent Development Patterns (2026)

### Debug Logging

**Pattern for API debugging:**
```go
// Add Debug field to Client struct
type Client struct {
    token      string
    apiURL     string
    httpClient *http.Client
    Debug      bool
}

// In execute method, log requests/responses when Debug is true
if c.Debug {
    fmt.Fprintf(os.Stderr, "\n[DEBUG] GraphQL Request:\n")
    fmt.Fprintf(os.Stderr, "URL: %s\n", c.apiURL)
    fmt.Fprintf(os.Stderr, "Query: %s\n", query)
    if len(variables) > 0 {
        varsJSON, _ := json.MarshalIndent(variables, "", "  ")
        fmt.Fprintf(os.Stderr, "Variables: %s\n", string(varsJSON))
    }
}
```

**Global flag pattern:**
```go
// In root.go
var debug bool

rootCmd.PersistentFlags().BoolVar(&debug, "debug", false,
    "Enable debug logging (shows GraphQL requests/responses)")

// Wire to API client
client := api.NewClient(token)
client.Debug = debug
```

### Service Instance Cleanup

**Pattern for environment-specific service creation:**

Railway creates services in ALL non-fork environments by default. To create in a specific environment only:

```go
// 1. Create service (will create in all environments)
service, err := client.CreateService(input, environmentID)

// 2. Get all environments
environments, err := client.GetEnvironments(projectID)

// 3. Delete instances from non-target environments
for _, env := range environments {
    if env.ID != targetEnvironmentID && !env.IsFork {
        // Add delay before cleanup
        time.Sleep(500 * time.Millisecond)
        
        // Retry with exponential backoff
        err := retryWithBackoff(func() error {
            return client.DeleteServiceInstance(service.ID, env.ID)
        }, 3, time.Second)
        
        if err != nil {
            // Log but don't fail - provide manual cleanup instructions
            fmt.Fprintf(os.Stderr, "Warning: failed to cleanup service instance in %s\n", env.Name)
        }
    }
}
```

**Retry pattern with exponential backoff:**
```go
func retryWithBackoff(fn func() error, maxRetries int, initialDelay time.Duration) error {
    var lastErr error
    for i := 0; i < maxRetries; i++ {
        if err := fn(); err == nil {
            return nil
        } else {
            lastErr = err
        }
        
        if i < maxRetries-1 {
            delay := initialDelay * time.Duration(1<<uint(i)) // 1s, 2s, 4s
            time.Sleep(delay)
        }
    }
    return lastErr
}
```

### Volume Management

**Pattern for listing and deleting volumes:**

```go
// Volumes are orphaned when services are deleted
// Always clean up volumes after deleting services

// 1. Delete services first
for _, serviceName := range services {
    client.DeleteService(serviceName, projectID, environmentID)
}

// 2. List all volumes in environment
volumes, err := client.GetVolumes(projectID, environmentID)

// 3. Delete each volume
for _, volume := range volumes {
    client.DeleteVolume(volume.Name, projectID, environmentID)
}
```

**Note:** Railway UI may not show orphaned volumes, but they still exist and consume quota. Always use the CLI to verify volume cleanup.

### Config-Driven Deployments

**Pattern for bash scripts using yq:**

```bash
# Helper to read config values
get_config() {
    local service_dir="$1"
    local path="$2"
    yq eval "$path" "$service_dir/config.yaml"
}

# Create service from config
create_service_from_config() {
    local service_dir="$1"
    local service_name=$(get_config "$service_dir" '.service.name')
    local image=$(get_config "$service_dir" '.service.image')
    
    # Read all config and build command
    railctl create service "$service_name" --image "$image" ...
}
```

**Variable expansion pattern:**
```bash
# Preserve Railway service references (${{service.VAR}})
# Expand environment variables (${VAR})
if [[ "$value" == *'${{'* ]]; then
    # Railway reference - keep as-is
    var_args+=("$key=$value")
elif [[ "$value" == *'${'* ]]; then
    # Environment variable - expand it
    expanded_value=$(set +u; eval echo "\"$value\"")
    var_args+=("$key=$expanded_value")
fi
```


## Checklist for New Features

- [ ] Check `internal/api/` for existing similar patterns
- [ ] Query the live GraphQL schema for field types (see Debugging Workflow)
- [ ] Define GraphQL query/mutation constant
- [ ] Create response type structs
- [ ] Implement API client method
- [ ] Update APIClient interface
- [ ] Update MockClient
- [ ] Create/update command file
- [ ] Add flags with env var documentation
- [ ] Implement command logic with proper error handling
- [ ] Support JSON/YAML/table output
- [ ] Add examples to command help
- [ ] Write/update unit tests
- [ ] **Update the E2E suite** (`tests/e2e/{account,workspace,project}`) for new commands/flags — and `docs/railctl-skill.md` (CI enforces it)
- [ ] Build and test manually
- [ ] Run E2E tests: `make test-e2e`
- [ ] Update README if needed
- [ ] Commit with conventional commit message

## Resources

- **Railway GraphQL API:** `https://backboard.railway.app/graphql/v2`
- **Token capability matrix:** `docs/token-capability-matrix.md`
- **E2E test suite:** `tests/e2e/README.md`
- **Testing architecture:** `docs/testing-architecture.md`
- **Declarative config:** `docs/declarative-config.md`
- **Embedded skill guide (source):** `docs/railctl-skill.md`
- **Design docs:** `docs/designs/`
