# Railway Service Creation Behavior & Workaround

## The Problem

When creating a service in Railway using the GraphQL API's `serviceCreate` mutation, **Railway automatically creates service instances in ALL non-fork environments**, even when you explicitly specify a target `environmentId`.

This behavior is unexpected and differs from what most users would expect: creating a service in a specific environment should only create it in that environment.

## Evidence & Sources

### GraphQL Schema Documentation

From Railway's GraphQL schema (accessible at `https://backboard.railway.com/graphql/v2`):

```graphql
serviceCreate(
  input: ServiceCreateInput!
  plugins: [String!]
  source: ServiceSourceInput
  variables: EnvironmentVariables
  environmentId: String
): Service!
```

The `environmentId` parameter documentation states:

> **environmentId**: String
> 
> Environment ID. If the specified environment is a fork, the service will only be created in it. **Otherwise it will be created in all environments that are not forks of other environments**

**Key insight**: The parameter only restricts creation to a single environment if that environment is a fork. For non-fork environments, Railway creates the service everywhere.

### API Response Evidence

When calling `serviceCreate` with `environmentId` for a non-fork environment, the response includes a `Service` object. Inspecting the service through the API shows:

```graphql
query {
  service(id: "service-id") {
    id
    name
    project {
      id
      environments {
        edges {
          node {
            id
            name
            serviceInstances {
              edges {
                node {
                  serviceId
                  # This shows instances in ALL non-fork environments
                }
              }
            }
          }
        }
      }
    }
  }
}
```

**Result**: Service instances appear in all non-fork environments, not just the target environment.

## Our Investigation

### Initial Attempt: Pass environmentId

**Hypothesis**: Not passing `environmentId` causes Railway to create services everywhere.

**Implementation**: Added `environmentId` parameter to `CreateService` API method.

**Result**: ❌ Services still created in all non-fork environments.

**Files Modified**:
- [`internal/api/services.go`](../internal/api/services.go) - Updated mutation call
- [`internal/api/interface.go`](../internal/api/interface.go) - Updated interface
- [`internal/cmd/create_service.go`](../internal/cmd/create_service.go) - Pass environment ID

### Discovery: serviceDelete with environmentId

While investigating deletion, we discovered the `serviceDelete` mutation accepts an optional `environmentId`:

```graphql
mutation {
  serviceDelete(
    id: String!
    environmentId: String  # Optional!
  ): Boolean!
}
```

**Behavior**:
- **Without `environmentId`**: Deletes the entire service from all environments
- **With `environmentId`**: Deletes only the service instance in that specific environment

This gave us the key to our workaround.

## Our Solution: Automatic Cleanup

Since we cannot prevent Railway from creating service instances in all environments, we implemented an automatic cleanup process that runs immediately after service creation.

### Implementation

**1. New API Method**: `DeleteServiceInstance`

```go
const deleteServiceInstanceMutation = `
mutation($id: String!, $environmentId: String!) {
    serviceDelete(id: $id, environmentId: $environmentId)
}
`

func (c *Client) DeleteServiceInstance(serviceID, environmentID string) error {
    _, err := c.execute(deleteServiceInstanceMutation, map[string]any{
        "id":            serviceID,
        "environmentId": environmentID,
    })
    return err
}
```

**Files**: [`internal/api/services.go`](../internal/api/services.go)

**2. Cleanup Logic in create service Command**

After creating a service:
1. Wait 500ms for service to be fully created
2. List all environments in the project
3. For each environment that is NOT the target:
   - Attempt to delete the service instance
   - Retry up to 3 times with exponential backoff (1s, 2s, 4s)
   - Log warnings on failure but don't fail the command

**Files**: [`internal/cmd/create_service.go`](../internal/cmd/create_service.go)

```go
// Wait for service to be fully created
time.Sleep(500 * time.Millisecond)

allEnvs, err := client.ListEnvironments(project.ID)
if err != nil {
    fmt.Fprintf(os.Stderr, "Warning: Could not list environments for cleanup: %v\n", err)
} else {
    cleanupErrors := []string{}
    for _, otherEnv := range allEnvs {
        if otherEnv.ID != env.ID {
            // Retry with exponential backoff
            maxRetries := 3
            var lastErr error
            for attempt := 0; attempt < maxRetries; attempt++ {
                if attempt > 0 {
                    backoff := time.Duration(1<<uint(attempt-1)) * time.Second
                    time.Sleep(backoff)
                }
                
                lastErr = client.DeleteServiceInstance(svc.ID, otherEnv.ID)
                if lastErr == nil {
                    break
                }
            }
            
            if lastErr != nil {
                errMsg := fmt.Sprintf("environment '%s': %v (after %d retries)", 
                    otherEnv.Name, lastErr, maxRetries)
                cleanupErrors = append(cleanupErrors, errMsg)
            }
        }
    }
    
    if len(cleanupErrors) > 0 {
        // Print detailed warning with manual cleanup instructions
        fmt.Fprintf(os.Stderr, "\n⚠️  Warning: Could not remove service from other environments:\n")
        for _, errMsg := range cleanupErrors {
            fmt.Fprintf(os.Stderr, "  - %s\n", errMsg)
        }
        fmt.Fprintf(os.Stderr, "\nThe service was created successfully in '%s', but may also exist in other environments.\n", env.Name)
        fmt.Fprintf(os.Stderr, "You can manually delete unwanted instances using: railctl delete service %s -e <environment>\n\n", svc.Name)
    }
}
```

## Design Decisions

### 1. **Automatic vs. Manual Cleanup**

**Decision**: Automatic cleanup by default

**Rationale**: 
- Users expect environment-specific creation
- Manual cleanup is error-prone and tedious
- Better UX: "just works" for the common case

**Alternative Considered**: Add `--all-environments` flag for users who want the Railway default behavior

### 2. **Retry Logic**

**Decision**: 3 retries with exponential backoff (1s, 2s, 4s)

**Rationale**:
- Service instances might not be immediately deletable
- Exponential backoff handles transient API issues
- Similar to our rate limit retry logic

### 3. **Error Handling**

**Decision**: Log warnings but don't fail the command

**Rationale**:
- Primary operation (service creation) succeeded
- Cleanup is best-effort
- Users can manually clean up if needed
- Provides clear instructions in error message

### 4. **Initial Delay**

**Decision**: 500ms delay before cleanup

**Rationale**:
- Ensures service is fully created before deletion attempts
- Reduces likelihood of "service not found" errors
- Minimal impact on user experience

## User Experience

### Before Fix

```bash
$ railctl create service my-app -e test --image nginx:latest
Service 'my-app' created with image 'nginx:latest' (ID: abc123)

# ❌ Service appears in: test, production, staging
```

### After Fix

```bash
$ railctl create service my-app -e test --image nginx:latest
Service 'my-app' created with image 'nginx:latest' (ID: abc123)

# ✅ Service appears in: test ONLY
# Cleanup happens silently in background
```

### On Cleanup Failure

```bash
$ railctl create service my-app -e test --image nginx:latest
Service 'my-app' created with image 'nginx:latest' (ID: abc123)

⚠️  Warning: Could not remove service from other environments:
  - environment 'production': API error (after 3 retries)
  - environment 'staging': API error (after 3 retries)

The service was created successfully in 'test', but may also exist in other environments.
You can manually delete unwanted instances using: railctl delete service my-app -e <environment>
```

## Testing

All existing tests pass with the new cleanup logic:
- `TestRunCreateService_Success`
- `TestRunCreateService_WithRegistryCredentials`
- `TestRunCreateService_WithDeployConfig`
- `TestRunCreateService_WithHealthcheck`

The 500ms delay is included in test execution time but doesn't significantly impact test performance.

## Future Considerations

### Potential Railway API Changes

If Railway changes the `serviceCreate` behavior to respect `environmentId` for non-fork environments, our cleanup logic will:
- Attempt to delete non-existent instances
- Receive "not found" errors
- Log warnings but not fail

This is acceptable degradation and won't break functionality.

### Alternative: --all-environments Flag

We could add an optional flag to preserve Railway's default behavior:

```bash
railctl create service my-app --image nginx:latest --all-environments
```

This would skip the cleanup logic and create the service in all non-fork environments.

## Related Files

- **API Layer**: [`internal/api/services.go`](../internal/api/services.go)
- **Interface**: [`internal/api/interface.go`](../internal/api/interface.go)
- **Command**: [`internal/cmd/create_service.go`](../internal/cmd/create_service.go)
- **Mock Client**: [`internal/api/mock.go`](../internal/api/mock.go)
- **Tests**: [`internal/cmd/create_service_test.go`](../internal/cmd/create_service_test.go)

## References

1. **Railway GraphQL API**: `https://backboard.railway.com/graphql/v2`
2. **GraphQL Schema Explorer**: Available in Railway's GraphQL playground
3. **Service Type Documentation**: Accessible via GraphQL introspection query
4. **Implementation Discussion**: See [walkthrough.md](../brain/a2ed1742-564c-4b4e-a97b-8c1debdb534d/walkthrough.md) for detailed implementation notes
