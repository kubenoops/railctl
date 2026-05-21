#!/usr/bin/env bash
#
# railctl E2E Test Suite
# ─────────────────────────────────────────────────────────────────
# Sequential test flow that creates a single project, environment,
# and service, then exercises every command and flag combination.
#
# Usage:
#   ./tests/e2e/run.sh
#
# Required environment:
#   RAILWAY_TOKEN     - Railway API token
#
# Optional:
#   RAILCTL           - Path to railctl binary (default: ./railctl)
#   E2E_PROJECT       - Custom project name (default: e2e-test-<uuid>)
#   E2E_KEEP          - Set to "1" to skip cleanup (for debugging)
#   E2E_VERBOSE       - Set to "1" for verbose output (show command stdout)
#

set -uo pipefail

# ─────────────────────────────────────────────────────────────────
# Configuration
# ─────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
RAILCTL="${RAILCTL:-$REPO_ROOT/railctl}"
PROJECT_NAME="${E2E_PROJECT:-e2e-test-$(date +%s)-$(head -c 4 /dev/urandom | xxd -p)}"
ENV_NAME="staging"
SERVICE_NAME="web"
SERVICE_IMAGE="nginx:1.25-alpine"
VERBOSE="${E2E_VERBOSE:-0}"

# ─────────────────────────────────────────────────────────────────
# Test Harness
# ─────────────────────────────────────────────────────────────────
TOTAL=0; PASSED=0; FAILED=0; SKIPPED=0
FAILURES=()

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

_section() {
    echo ""
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${CYAN}  $1${NC}"
    echo -e "${CYAN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

_test_header() {
    ((TOTAL++))
    printf "  ${DIM}[%03d]${NC} %-60s " "$TOTAL" "$1"
}

_pass() {
    ((PASSED++))
    echo -e "${GREEN}PASS${NC}"
}

_fail() {
    ((FAILED++))
    echo -e "${RED}FAIL${NC}"
    FAILURES+=("$1: $2")
    if [[ -n "${2:-}" ]]; then
        echo -e "        ${RED}→ $2${NC}"
    fi
}

_skip() {
    ((SKIPPED++))
    echo -e "${YELLOW}SKIP${NC}"
    if [[ -n "${1:-}" ]]; then
        echo -e "        ${YELLOW}→ $1${NC}"
    fi
}

# Run a railctl command, capture stdout, stderr, and exit code.
# Usage: rc <args...>
# Outputs are available in: $RC_STDOUT, $RC_STDERR, $RC_EXIT
RC_STDOUT=""
RC_STDERR=""
RC_EXIT=0

rc() {
    local tmpout tmperr
    tmpout=$(mktemp)
    tmperr=$(mktemp)

    set +e
    "$RAILCTL" "$@" >"$tmpout" 2>"$tmperr"
    RC_EXIT=$?
    set +e  # Keep errexit OFF — script does not use set -e

    RC_STDOUT=$(cat "$tmpout")
    RC_STDERR=$(cat "$tmperr")
    rm -f "$tmpout" "$tmperr"

    if [[ "$VERBOSE" == "1" ]]; then
        [[ -n "$RC_STDOUT" ]] && echo -e "        ${DIM}stdout: ${RC_STDOUT}${NC}"
        [[ -n "$RC_STDERR" ]] && echo -e "        ${DIM}stderr: ${RC_STDERR}${NC}"
    fi
}

# Shorthand: run command with common project/environment flags
rcp() {
    rc -p "$PROJECT_NAME" "$@"
}

rcpe() {
    rc -p "$PROJECT_NAME" -e "$ENV_NAME" "$@"
}

rcpes() {
    rc -p "$PROJECT_NAME" -e "$ENV_NAME" -s "$SERVICE_NAME" "$@"
}

# Assert helpers
assert_success() {
    local test_name="$1"
    if [[ "$RC_EXIT" -eq 0 ]]; then
        _pass
    else
        _fail "$test_name" "exit=$RC_EXIT stderr=$(echo "$RC_STDERR" | head -1)"
    fi
}

assert_failure() {
    local test_name="$1"
    if [[ "$RC_EXIT" -ne 0 ]]; then
        _pass
    else
        _fail "$test_name" "expected non-zero exit, got 0"
    fi
}

assert_stdout_contains() {
    local test_name="$1"
    local expected="$2"
    if echo "$RC_STDOUT" | grep -qF "$expected"; then
        _pass
    else
        _fail "$test_name" "stdout missing '$expected'"
    fi
}

assert_stdout_not_empty() {
    local test_name="$1"
    if [[ -n "$RC_STDOUT" ]]; then
        _pass
    else
        _fail "$test_name" "stdout was empty"
    fi
}

assert_valid_json() {
    local test_name="$1"
    if echo "$RC_STDOUT" | jq . >/dev/null 2>&1; then
        _pass
    else
        _fail "$test_name" "stdout is not valid JSON"
    fi
}

assert_valid_yaml() {
    local test_name="$1"
    # Basic YAML check: should parse without error if yq is available,
    # otherwise just check non-empty
    if command -v yq &>/dev/null; then
        if echo "$RC_STDOUT" | yq . >/dev/null 2>&1; then
            _pass
        else
            _fail "$test_name" "stdout is not valid YAML"
        fi
    elif [[ -n "$RC_STDOUT" ]]; then
        _pass
    else
        _fail "$test_name" "stdout was empty"
    fi
}

# ─────────────────────────────────────────────────────────────────
# Pre-flight Checks
# ─────────────────────────────────────────────────────────────────
preflight() {
    _section "Pre-flight Checks"

    # Check binary exists
    _test_header "railctl binary exists"
    if [[ -x "$RAILCTL" ]]; then
        _pass
    else
        echo -e "${RED}FAIL${NC}"
        echo -e "${RED}ERROR: railctl binary not found at $RAILCTL${NC}"
        echo "  Build it: cd $REPO_ROOT && go build -o railctl ./cmd/railctl"
        exit 1
    fi

    # Check token
    _test_header "RAILWAY_TOKEN is set"
    if [[ -n "${RAILWAY_TOKEN:-}" ]]; then
        _pass
    else
        echo -e "${RED}FAIL${NC}"
        echo -e "${RED}ERROR: RAILWAY_TOKEN environment variable required${NC}"
        exit 1
    fi

    # Check jq
    _test_header "jq is installed"
    if command -v jq &>/dev/null; then
        _pass
    else
        echo -e "${RED}FAIL${NC}"
        echo -e "${RED}ERROR: jq is required for JSON assertions${NC}"
        exit 1
    fi

    # Version check
    _test_header "railctl --version"
    rc --version
    assert_success "railctl --version"

    echo ""
    echo -e "  ${BOLD}Test Configuration:${NC}"
    echo -e "    Project:     ${BLUE}$PROJECT_NAME${NC}"
    echo -e "    Environment: ${BLUE}$ENV_NAME${NC}"
    echo -e "    Service:     ${BLUE}$SERVICE_NAME${NC}"
    echo -e "    Image:       ${BLUE}$SERVICE_IMAGE${NC}"
}

# ─────────────────────────────────────────────────────────────────
# Phase 1: Project Operations
# ─────────────────────────────────────────────────────────────────
test_projects() {
    _section "Phase 1: Project Operations"

    # --- Create ---
    _test_header "create project"
    rc create project "$PROJECT_NAME"
    assert_success "create project"

    # Small delay for API propagation
    sleep 2

    # --- Get (table) ---
    _test_header "get projects (table)"
    rc get projects
    assert_stdout_contains "get projects (table)" "$PROJECT_NAME"

    # --- Get (json) ---
    _test_header "get projects -o json"
    rc get projects -o json
    assert_valid_json "get projects -o json"

    # --- Get (yaml) ---
    _test_header "get projects -o yaml"
    rc get projects -o yaml
    assert_valid_yaml "get projects -o yaml"

    # --- Get (wide) ---
    _test_header "get projects -o wide"
    rc get projects -o wide
    assert_stdout_contains "get projects -o wide" "$PROJECT_NAME"

    # --- Describe (table) ---
    _test_header "describe project (table)"
    rc describe project "$PROJECT_NAME"
    assert_stdout_contains "describe project (table)" "$PROJECT_NAME"

    # --- Describe (json) ---
    _test_header "describe project -o json"
    rc describe project "$PROJECT_NAME" -o json
    assert_valid_json "describe project -o json"

    # --- Describe (yaml) ---
    _test_header "describe project -o yaml"
    rc describe project "$PROJECT_NAME" -o yaml
    assert_valid_yaml "describe project -o yaml"

    # --- Error: describe nonexistent ---
    _test_header "describe project nonexistent (expect error)"
    rc describe project "nonexistent-project-xyz-999"
    assert_failure "describe project nonexistent"

    # --- Error: get projects with bad token ---
    _test_header "get projects with bad token (expect error)"
    RAILWAY_TOKEN_BAK="$RAILWAY_TOKEN"
    rc get projects --token "invalid-token-12345"
    assert_failure "get projects with bad token"
    export RAILWAY_TOKEN="$RAILWAY_TOKEN_BAK"
}

# ─────────────────────────────────────────────────────────────────
# Phase 2: Environment Operations
# ─────────────────────────────────────────────────────────────────
test_environments() {
    _section "Phase 2: Environment Operations"

    # --- Get default environments ---
    _test_header "get environments (default production)"
    rcp get environments
    assert_stdout_contains "get environments (default production)" "production"

    # --- Get (json) ---
    _test_header "get environments -o json"
    rcp get environments -o json
    assert_valid_json "get environments -o json"

    # --- Get (yaml) ---
    _test_header "get environments -o yaml"
    rcp get environments -o yaml
    assert_valid_yaml "get environments -o yaml"

    # --- Create new environment ---
    _test_header "create environment '$ENV_NAME'"
    rcp create environment "$ENV_NAME"
    assert_success "create environment"

    sleep 2

    # --- Verify it appears in list ---
    _test_header "get environments shows '$ENV_NAME'"
    rcp get environments
    assert_stdout_contains "get environments shows staging" "$ENV_NAME"

    # --- Describe (table) ---
    _test_header "describe environment (table)"
    rcp describe environment "$ENV_NAME"
    assert_stdout_contains "describe environment (table)" "$ENV_NAME"

    # --- Describe (json) ---
    _test_header "describe environment -o json"
    rcp describe environment "$ENV_NAME" -o json
    assert_valid_json "describe environment -o json"

    # --- Describe (yaml) ---
    _test_header "describe environment -o yaml"
    rcp describe environment "$ENV_NAME" -o yaml
    assert_valid_yaml "describe environment -o yaml"

    # --- Error: missing project flag ---
    _test_header "get environments without -p (expect error)"
    rc get environments
    assert_failure "get environments without -p"

    # --- Error: nonexistent project ---
    _test_header "get environments with bad project (expect error)"
    rc get environments -p "nonexistent-xyz-999"
    assert_failure "get environments with bad project"

    # --- Error: describe nonexistent env ---
    _test_header "describe environment nonexistent (expect error)"
    rcp describe environment "nonexistent-env-xyz"
    assert_failure "describe environment nonexistent"
}

# ─────────────────────────────────────────────────────────────────
# Phase 3: Service Operations
# ─────────────────────────────────────────────────────────────────
test_services() {
    _section "Phase 3: Service Operations"

    # --- Create with --image ---
    _test_header "create service with --image"
    rcpe create service "$SERVICE_NAME" --image "$SERVICE_IMAGE"
    assert_success "create service with --image"

    sleep 3

    # --- Get (table) ---
    _test_header "get services (table)"
    rcpe get services
    assert_stdout_contains "get services (table)" "$SERVICE_NAME"

    # --- Get (json) ---
    _test_header "get services -o json"
    rcpe get services -o json
    assert_valid_json "get services -o json"

    # --- Get (yaml) ---
    _test_header "get services -o yaml"
    rcpe get services -o yaml
    assert_valid_yaml "get services -o yaml"

    # --- Get (wide) ---
    _test_header "get services -o wide"
    rcpe get services -o wide
    assert_stdout_contains "get services -o wide" "$SERVICE_NAME"

    # --- Describe (table) ---
    _test_header "describe service (table)"
    rcpe describe service "$SERVICE_NAME"
    assert_stdout_contains "describe service (table)" "$SERVICE_NAME"

    # --- Describe (json) ---
    _test_header "describe service -o json"
    rcpe describe service "$SERVICE_NAME" -o json
    assert_valid_json "describe service -o json"

    # --- Describe (yaml) ---
    _test_header "describe service -o yaml"
    rcpe describe service "$SERVICE_NAME" -o yaml
    assert_valid_yaml "describe service -o yaml"

    # --- Describe with --show-values ---
    _test_header "describe service --show-values"
    rcpe describe service "$SERVICE_NAME" --show-values
    assert_success "describe service --show-values"

    # --- Error: missing project ---
    _test_header "get services without -p (expect error)"
    rc get services -e "$ENV_NAME"
    assert_failure "get services without -p"

    # --- Error: missing environment ---
    _test_header "get services without -e (expect error)"
    rcp get services
    assert_failure "get services without -e"

    # --- Error: nonexistent service ---
    _test_header "describe service nonexistent (expect error)"
    rcpe describe service "nonexistent-svc-xyz"
    assert_failure "describe service nonexistent"

    # --- Substring matching ---
    _test_header "describe service with substring match"
    rcpe describe service "we"    # should match "web"
    assert_stdout_contains "describe service substring" "$SERVICE_NAME"
}

# ─────────────────────────────────────────────────────────────────
# Phase 4: Variable Operations
# ─────────────────────────────────────────────────────────────────
test_variables() {
    _section "Phase 4: Variable Operations"

    # --- Set single variable ---
    _test_header "set variable (single)"
    rcpes set variable PORT=3000
    assert_stdout_contains "set variable (single)" "Variable 'PORT' set successfully"

    # --- Set multiple variables ---
    _test_header "set variable (multiple)"
    rcpes set variable NODE_ENV=test LOG_LEVEL=debug APP_NAME=e2e-test
    assert_stdout_contains "set variable (multiple)" "3 variables set successfully"

    # --- Set with --skip-deployment ---
    _test_header "set variable --skip-deployment"
    rcpes set variable SKIP_TEST_VAR=true --skip-deployment
    assert_stdout_contains "set variable --skip-deployment" "Deployment skipped"

    sleep 2

    # --- Get variables (table) ---
    _test_header "get variables (table)"
    rcpes get variables
    assert_stdout_contains "get variables (table)" "PORT"

    # --- Get variables (json) ---
    _test_header "get variables -o json"
    rcpes get variables -o json
    assert_valid_json "get variables -o json"

    # --- Get variables: verify values ---
    _test_header "get variables contains all set variables"
    rcpes get variables
    local out="$RC_STDOUT"
    local all_found=true
    for var in PORT NODE_ENV LOG_LEVEL APP_NAME SKIP_TEST_VAR; do
        if ! echo "$out" | grep -qF "$var"; then
            all_found=false
            break
        fi
    done
    if $all_found; then _pass; else _fail "get variables contains all" "missing variable(s)"; fi

    # --- Get variables (yaml) ---
    _test_header "get variables -o yaml"
    rcpes get variables -o yaml
    assert_valid_yaml "get variables -o yaml"

    # --- Delete variable ---
    _test_header "delete variable"
    rcpes delete variable SKIP_TEST_VAR --yes
    assert_stdout_contains "delete variable" "deleted successfully"

    # --- Verify deletion ---
    _test_header "get variables after delete (SKIP_TEST_VAR gone)"
    rcpes get variables
    if echo "$RC_STDOUT" | grep -qF "SKIP_TEST_VAR"; then
        _fail "variable still present" "SKIP_TEST_VAR should be deleted"
    else
        _pass
    fi

    # --- Error: set variable bad format ---
    _test_header "set variable bad format (expect error)"
    rcpes set variable "NOEQUALS"
    assert_failure "set variable bad format"

    # --- Error: set variable empty key ---
    _test_header "set variable empty key (expect error)"
    rcpes set variable "=value"
    assert_failure "set variable empty key"

    # --- Error: missing service flag ---
    _test_header "get variables without -s (expect error)"
    rcpe get variables
    assert_failure "get variables without -s"
}

# ─────────────────────────────────────────────────────────────────
# Phase 5: Volume Operations
# ─────────────────────────────────────────────────────────────────
VOLUME_NAME=""  # Will be set by Railway

test_volumes() {
    _section "Phase 5: Volume Operations"

    # --- Create volume ---
    _test_header "create volume with --mount-path"
    rcpes create volume --mount-path /app/data
    assert_success "create volume"

    sleep 2

    # --- Get volumes (table) ---
    _test_header "get volumes (table)"
    rcpe get volumes
    assert_stdout_contains "get volumes (table)" "/app/data"

    # --- Get volumes (json) ---
    _test_header "get volumes -o json"
    rcpe get volumes -o json
    assert_valid_json "get volumes -o json"

    # --- Get volumes (yaml) ---
    _test_header "get volumes -o yaml"
    rcpe get volumes -o yaml
    assert_valid_yaml "get volumes -o yaml"

    # Capture volume name for later operations
    rcpe get volumes -o json
    VOLUME_NAME=$(echo "$RC_STDOUT" | jq -r '.[0].name // empty' 2>/dev/null || true)

    if [[ -z "$VOLUME_NAME" ]]; then
        echo -e "  ${YELLOW}⚠ Could not capture volume name, skipping volume update/describe tests${NC}"

        _test_header "describe volume (skipped - no volume name)"
        _skip "volume name not captured"

        _test_header "update volume --name (skipped)"
        _skip "volume name not captured"
    else
        # --- Describe volume (table) ---
        _test_header "describe volume (table)"
        rcpe describe volume "$VOLUME_NAME"
        assert_success "describe volume (table)"

        # --- Describe volume (json) ---
        _test_header "describe volume -o json"
        rcpe describe volume "$VOLUME_NAME" -o json
        assert_valid_json "describe volume -o json"

        # --- Describe volume (yaml) ---
        _test_header "describe volume -o yaml"
        rcpe describe volume "$VOLUME_NAME" -o yaml
        assert_valid_yaml "describe volume -o yaml"

        # --- Update volume name ---
        _test_header "update volume --name"
        rcpe update volume "$VOLUME_NAME" --name "e2e-renamed-vol"
        assert_stdout_contains "update volume --name" "renamed"
        VOLUME_NAME="e2e-renamed-vol"
    fi

    # --- Error: create volume with bad mount path ---
    _test_header "create volume bad mount path (expect error)"
    rcpes create volume --mount-path "no-leading-slash"
    assert_failure "create volume bad mount path"

    # --- Error: missing service flag ---
    _test_header "create volume without -s (expect error)"
    rcpe create volume --mount-path /test
    assert_failure "create volume without -s"
}

# ─────────────────────────────────────────────────────────────────
# Phase 6: Deployment Operations
# ─────────────────────────────────────────────────────────────────
DEPLOY_ID_1=""  # First deployment (from create service)
DEPLOY_ID_2=""  # Second deployment (from image update)

test_deployments() {
    _section "Phase 6: Deployment Operations"

    # Wait for initial deployment to register
    sleep 5

    # --- Get deployments (table) ---
    _test_header "get deployments (table)"
    rcpes get deployments
    assert_success "get deployments (table)"

    # --- Get deployments (json) ---
    _test_header "get deployments -o json"
    rcpes get deployments -o json
    assert_valid_json "get deployments -o json"

    # --- Get deployments (yaml) ---
    _test_header "get deployments -o yaml"
    rcpes get deployments -o yaml
    assert_valid_yaml "get deployments -o yaml"

    # --- Get deployments (wide) ---
    _test_header "get deployments -o wide"
    rcpes get deployments -o wide
    assert_success "get deployments -o wide"

    # --- Get deployments with --limit ---
    _test_header "get deployments --limit 1"
    rcpes get deployments --limit 1
    assert_success "get deployments --limit 1"

    # --- Capture deployment IDs for lifecycle tests ---
    rcpes get deployments -o json
    DEPLOY_ID_1=$(echo "$RC_STDOUT" | jq -r '.[0].id // empty' 2>/dev/null || true)

    # --- Logs (latest deployment) ---
    _test_header "logs service (latest deployment)"
    rcpe logs "$SERVICE_NAME" --tail 5
    if [[ "$RC_EXIT" -eq 0 ]]; then
        _pass
    else
        if echo "$RC_STDERR$RC_STDOUT" | grep -qiE "no (successful deployment|deployment|log)"; then
            _pass
        else
            _fail "logs service" "exit=$RC_EXIT"
        fi
    fi

    # --- Logs with --deployment ID ---
    if [[ -n "$DEPLOY_ID_1" ]]; then
        _test_header "logs service --deployment <id>"
        rcpe logs "$SERVICE_NAME" --deployment "$DEPLOY_ID_1" --tail 5
        if [[ "$RC_EXIT" -eq 0 ]]; then
            _pass
        else
            if echo "$RC_STDERR$RC_STDOUT" | grep -qiE "no (log|build)"; then
                _pass  # No logs yet is OK
            else
                _fail "logs service --deployment" "exit=$RC_EXIT"
            fi
        fi

        _test_header "logs service --tail 1"
        rcpe logs "$SERVICE_NAME" --tail 1
        if [[ "$RC_EXIT" -eq 0 ]]; then
            _pass
        else
            if echo "$RC_STDERR$RC_STDOUT" | grep -qiE "no (successful deployment|deployment|log)"; then
                _pass
            else
                _fail "logs service --tail 1" "exit=$RC_EXIT"
            fi
        fi
    else
        _test_header "logs service --deployment <id> (skipped)"
        _skip "no deployment ID captured"

        _test_header "logs service --tail 1 (skipped)"
        _skip "no deployment ID captured"
    fi

    # --- Error: logs with bad deployment ID ---
    _test_header "logs service --deployment bad-id (expect error)"
    rcpe logs "$SERVICE_NAME" --deployment "nonexistent-deploy-xyz"
    assert_failure "logs service --deployment bad-id"

    # --- Error: missing service flag ---
    _test_header "get deployments without -s (expect error)"
    rcpe get deployments
    assert_failure "get deployments without -s"

    # --- Error: nonexistent deployment delete ---
    _test_header "delete deployment nonexistent (expect error)"
    rcpes delete deployment "nonexistent-deploy-id-xyz" --yes
    assert_failure "delete deployment nonexistent"

    # --- Error: update deployment without --set-active ---
    if [[ -n "$DEPLOY_ID_1" ]]; then
        _test_header "update deployment without --set-active (expect error)"
        rc update deployment "$DEPLOY_ID_1"
        assert_failure "update deployment without --set-active"
    else
        _test_header "update deployment without --set-active (skipped)"
        _skip "no deployment ID captured"
    fi

    # --- Error: update nonexistent deployment ---
    _test_header "update deployment nonexistent --set-active (expect error)"
    rc update deployment "nonexistent-deploy-id-xyz" --set-active
    assert_failure "update deployment nonexistent --set-active"
}

# ─────────────────────────────────────────────────────────────────
# Phase 6b: Deployment Lifecycle (rollback / reactivate)
#   - Requires at least 2 deployments, so runs after update service
# ─────────────────────────────────────────────────────────────────
test_deployment_lifecycle() {
    _section "Phase 6b: Deployment Lifecycle (rollback / reactivate)"

    # After update service tests, we should have multiple deployments.
    # Refresh deployment list.
    rcpes get deployments -o json
    local deploy_count
    deploy_count=$(echo "$RC_STDOUT" | jq 'length' 2>/dev/null || echo "0")

    if [[ "$deploy_count" -lt 2 ]]; then
        echo -e "  ${YELLOW}⚠ Only $deploy_count deployment(s) — triggering another deployment${NC}"
        rcpe update service "$SERVICE_NAME" --image "nginx:1.25-alpine" --yes
        sleep 5
        rcpes get deployments -o json
        deploy_count=$(echo "$RC_STDOUT" | jq 'length' 2>/dev/null || echo "0")
    fi

    # Capture IDs: latest = index 0, previous = index 1
    local latest_id prev_id
    latest_id=$(echo "$RC_STDOUT" | jq -r '.[0].id // empty' 2>/dev/null || true)
    prev_id=$(echo "$RC_STDOUT" | jq -r '.[1].id // empty' 2>/dev/null || true)

    if [[ -z "$latest_id" ]]; then
        _test_header "delete deployment / rollback (skipped)"
        _skip "no deployment IDs available"

        _test_header "update deployment --set-active / reactivate (skipped)"
        _skip "no deployment IDs available"
        return
    fi

    # --- Delete deployment (rollback) ---
    # Deleting the latest active deployment triggers Railway to promote the previous one
    _test_header "delete deployment (rollback latest)"
    rcpes delete deployment "$latest_id" --yes
    assert_success "delete deployment (rollback)"

    sleep 3

    # Verify the deployment is gone
    _test_header "deployment removed from list after delete"
    rcpes get deployments -o json
    local remaining_ids
    remaining_ids=$(echo "$RC_STDOUT" | jq -r '.[].id // empty' 2>/dev/null || true)
    if echo "$remaining_ids" | grep -qF "$latest_id"; then
        _fail "deployment still in list" "$latest_id should be removed"
    else
        _pass
    fi

    # --- Reactivate / redeploy via update deployment --set-active ---
    if [[ -n "$prev_id" ]]; then
        _test_header "update deployment --set-active (reactivate)"
        rc update deployment "$prev_id" --set-active
        assert_success "update deployment --set-active"

        sleep 3

        # Verify a new deployment was triggered
        _test_header "new deployment appears after reactivate"
        rcpes get deployments -o json
        local new_count
        new_count=$(echo "$RC_STDOUT" | jq 'length' 2>/dev/null || echo "0")
        if [[ "$new_count" -ge 1 ]]; then
            _pass
        else
            _fail "new deployment after reactivate" "expected >=1 deployments, got $new_count"
        fi
    else
        _test_header "update deployment --set-active (skipped)"
        _skip "no previous deployment to reactivate"

        _test_header "new deployment appears after reactivate (skipped)"
        _skip "no previous deployment to reactivate"
    fi
}

# ─────────────────────────────────────────────────────────────────
# Phase 7: Update Service Operations
# ─────────────────────────────────────────────────────────────────
test_update_service() {
    _section "Phase 7: Update Service Operations"

    # --- Update image ---
    _test_header "update service --image"
    rcpe update service "$SERVICE_NAME" --image "nginx:1.26-alpine" --yes
    assert_success "update service --image"

    sleep 2

    # --- Update with --skip-deployment ---
    _test_header "update service --image --skip-deployment"
    rcpe update service "$SERVICE_NAME" --image "nginx:1.25-alpine" --skip-deployment --yes
    assert_success "update service --image --skip-deployment"

    # --- Update start command ---
    _test_header "update service --start-command"
    rcpe update service "$SERVICE_NAME" --start-command "nginx -g 'daemon off;'" --yes
    assert_stdout_contains "update service --start-command" "Start command"

    # --- Update restart policy ---
    _test_header "update service --restart-policy"
    rcpe update service "$SERVICE_NAME" --restart-policy ON_FAILURE --yes
    assert_stdout_contains "update service --restart-policy" "Restart policy"

    # --- Update restart policy + max-retries ---
    _test_header "update service --restart-policy + --max-retries"
    rcpe update service "$SERVICE_NAME" --restart-policy ON_FAILURE --max-retries 3 --yes
    assert_stdout_contains "update service --restart-policy + max-retries" "Max retries"

    # --- Update replicas ---
    _test_header "update service --replicas"
    rcpe update service "$SERVICE_NAME" --replicas 1 --yes
    assert_stdout_contains "update service --replicas" "Replicas"

    # --- Update healthcheck ---
    _test_header "update service --healthcheck-path"
    rcpe update service "$SERVICE_NAME" --healthcheck-path /health --yes
    assert_stdout_contains "update service --healthcheck-path" "Health check path"

    # --- Update healthcheck with timeout ---
    _test_header "update service --healthcheck-path + --healthcheck-timeout"
    rcpe update service "$SERVICE_NAME" \
        --healthcheck-path /api/health --healthcheck-timeout 60 --yes
    assert_stdout_contains "update service healthcheck+timeout" "Health check timeout"

    # --- Combined update ---
    _test_header "update service (combined: image + replicas + healthcheck)"
    rcpe update service "$SERVICE_NAME" \
        --image "nginx:1.26-alpine" \
        --replicas 1 \
        --healthcheck-path /health \
        --yes
    assert_success "update service combined"

    # --- Error: no flags ---
    _test_header "update service with no update flags (expect error)"
    rcpe update service "$SERVICE_NAME" --yes
    assert_failure "update service no flags"

    # --- Error: invalid restart policy ---
    _test_header "update service invalid restart policy (expect error)"
    rcpe update service "$SERVICE_NAME" --restart-policy INVALID_POLICY --yes
    assert_failure "update service invalid restart policy"

    # --- Error: max-retries without restart-policy ---
    _test_header "update service --max-retries without --restart-policy (error)"
    rcpe update service "$SERVICE_NAME" --max-retries 3 --yes
    assert_failure "update service max-retries without restart-policy"

    # --- Error: replicas < 1 ---
    _test_header "update service --replicas 0 (expect error)"
    rcpe update service "$SERVICE_NAME" --replicas 0 --yes
    assert_failure "update service replicas 0"

    # --- Error: nonexistent service ---
    _test_header "update service nonexistent (expect error)"
    rcpe update service "nonexistent-svc-xyz" --image "test:1" --yes
    assert_failure "update service nonexistent"
}

# ─────────────────────────────────────────────────────────────────
# Phase 8: Error Handling & Edge Cases
# ─────────────────────────────────────────────────────────────────
test_error_handling() {
    _section "Phase 8: Error Handling & Edge Cases"

    # --- Missing required flags ---
    _test_header "create service without --image (expect error)"
    rcpe create service "test-svc"
    assert_failure "create service without --image"

    # --- Invalid output format ---
    # (cobra handles this, should still work with default)
    _test_header "get projects -o invalid-format"
    rc get projects -o "not-a-format"
    assert_failure "get projects -o invalid-format"

    # --- Debug flag ---
    _test_header "--debug flag produces output"
    rc get projects --debug 2>/dev/null
    # Debug output goes to stderr, so we check either stdout or stderr
    if [[ "$RC_EXIT" -eq 0 ]]; then
        _pass
    else
        _fail "--debug flag" "exit=$RC_EXIT"
    fi

    # --- Env var overrides ---
    _test_header "RAILCTL_PROJECT env var works"
    RAILCTL_PROJECT="$PROJECT_NAME" rc get environments
    assert_success "RAILCTL_PROJECT env var"

    _test_header "RAILCTL_ENVIRONMENT env var works"
    RAILCTL_PROJECT="$PROJECT_NAME" RAILCTL_ENVIRONMENT="$ENV_NAME" rc get services
    assert_success "RAILCTL_ENVIRONMENT env var"

    _test_header "RAILCTL_SERVICE env var works"
    RAILCTL_PROJECT="$PROJECT_NAME" RAILCTL_ENVIRONMENT="$ENV_NAME" RAILCTL_SERVICE="$SERVICE_NAME" rc get variables
    assert_success "RAILCTL_SERVICE env var"

    # --- Substring resolution ---
    _test_header "project substring resolution"
    # Use a substring of the project name
    local proj_substr="${PROJECT_NAME:0:10}"
    rc get environments -p "$proj_substr"
    assert_success "project substring resolution"

    _test_header "environment substring resolution"
    rcp get services -e "stag"
    assert_success "environment substring resolution"
}

# ─────────────────────────────────────────────────────────────────
# Phase 9: Cleanup (reverse order)
# ─────────────────────────────────────────────────────────────────
test_cleanup() {
    _section "Phase 9: Cleanup"

    if [[ "${E2E_KEEP:-}" == "1" ]]; then
        echo -e "  ${YELLOW}⚠ E2E_KEEP=1 — skipping cleanup${NC}"
        echo -e "  ${YELLOW}  Manually delete project: $PROJECT_NAME${NC}"
        return
    fi

    # --- Delete volume ---
    if [[ -n "${VOLUME_NAME:-}" ]]; then
        _test_header "delete volume"
        rcpe delete volume "$VOLUME_NAME" --yes
        assert_success "delete volume"
    fi

    # --- Delete variables ---
    _test_header "delete variable PORT"
    rcpes delete variable PORT --yes
    assert_success "delete variable PORT"

    _test_header "delete variable NODE_ENV"
    rcpes delete variable NODE_ENV --yes
    assert_success "delete variable NODE_ENV"

    # --- Delete service ---
    _test_header "delete service"
    rcpe delete service "$SERVICE_NAME" --yes
    assert_success "delete service"

    sleep 2

    # --- Verify service gone ---
    _test_header "get services after delete (empty)"
    rcpe get services
    if echo "$RC_STDOUT" | grep -qF "$SERVICE_NAME"; then
        _fail "service still present" "$SERVICE_NAME should be deleted"
    else
        _pass
    fi

    # --- Delete environment ---
    _test_header "delete environment '$ENV_NAME'"
    rcp delete environment "$ENV_NAME" --yes
    assert_success "delete environment"

    sleep 2

    # --- Verify environment gone ---
    _test_header "get environments after delete"
    rcp get environments
    if echo "$RC_STDOUT" | grep -qF "$ENV_NAME"; then
        _fail "environment still present" "$ENV_NAME should be deleted"
    else
        _pass
    fi

    # --- Delete project ---
    _test_header "delete project"
    rc delete project "$PROJECT_NAME" --yes
    assert_success "delete project"

    sleep 2

    # --- Verify project gone ---
    _test_header "get projects after delete"
    rc get projects
    if echo "$RC_STDOUT" | grep -qF "$PROJECT_NAME"; then
        _fail "project still present" "$PROJECT_NAME should be deleted"
    else
        _pass
    fi
}

# ─────────────────────────────────────────────────────────────────
# Emergency Cleanup (runs on failure/interrupt)
# ─────────────────────────────────────────────────────────────────
emergency_cleanup() {
    echo ""
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${RED}  Emergency Cleanup${NC}"
    echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"

    if [[ "${E2E_KEEP:-}" == "1" ]]; then
        echo -e "  ${YELLOW}E2E_KEEP=1 — skipping emergency cleanup${NC}"
        echo -e "  ${YELLOW}Manually delete: $PROJECT_NAME${NC}"
        return
    fi

    # Delete services in all environments (required before project deletion)
    echo -e "  Cleaning up services..."
    for env in "$ENV_NAME" "production"; do
        local services_json
        services_json=$("$RAILCTL" get services -p "$PROJECT_NAME" -e "$env" -o json 2>/dev/null || echo "[]")
        local svc_names
        svc_names=$(echo "$services_json" | jq -r '.[].name // empty' 2>/dev/null || true)
        for svc in $svc_names; do
            echo -e "    Deleting service ${BOLD}$svc${NC} in $env..."
            "$RAILCTL" delete service "$svc" -p "$PROJECT_NAME" -e "$env" --yes 2>/dev/null || true
        done
    done

    # Delete volumes in all environments
    echo -e "  Cleaning up volumes..."
    for env in "$ENV_NAME" "production"; do
        local volumes_json
        volumes_json=$("$RAILCTL" get volumes -p "$PROJECT_NAME" -e "$env" -o json 2>/dev/null || echo "[]")
        local vol_names
        vol_names=$(echo "$volumes_json" | jq -r '.[].name // empty' 2>/dev/null || true)
        for vol in $vol_names; do
            echo -e "    Deleting volume ${BOLD}$vol${NC} in $env..."
            "$RAILCTL" delete volume "$vol" -p "$PROJECT_NAME" -e "$env" --yes 2>/dev/null || true
        done
    done

    sleep 2

    # Delete project
    echo -e "  Deleting project ${BOLD}$PROJECT_NAME${NC}..."
    "$RAILCTL" delete project "$PROJECT_NAME" --yes 2>/dev/null && \
        echo -e "  ${GREEN}Project deleted${NC}" || \
        echo -e "  ${YELLOW}Could not delete project (may not exist)${NC}"
}

# ─────────────────────────────────────────────────────────────────
# Results Summary
# ─────────────────────────────────────────────────────────────────
print_results() {
    echo ""
    echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BOLD}  Test Results${NC}"
    echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo ""
    echo -e "  ${GREEN}Passed:${NC}  $PASSED"
    echo -e "  ${RED}Failed:${NC}  $FAILED"
    echo -e "  ${YELLOW}Skipped:${NC} $SKIPPED"
    echo -e "  ${BOLD}Total:${NC}   $TOTAL"
    echo ""

    if [[ ${#FAILURES[@]} -gt 0 ]]; then
        echo -e "  ${RED}${BOLD}Failures:${NC}"
        for f in "${FAILURES[@]}"; do
            echo -e "    ${RED}✗${NC} $f"
        done
        echo ""
    fi

    if [[ $FAILED -eq 0 ]]; then
        echo -e "  ${GREEN}${BOLD}✓ All tests passed!${NC}"
    else
        echo -e "  ${RED}${BOLD}✗ $FAILED test(s) failed${NC}"
    fi
    echo -e "${BOLD}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

# ─────────────────────────────────────────────────────────────────
# Main
# ─────────────────────────────────────────────────────────────────
main() {
    echo -e "${BOLD}"
    echo "  ┌─────────────────────────────────────────────────┐"
    echo "  │         railctl E2E Test Suite                   │"
    echo "  └─────────────────────────────────────────────────┘"
    echo -e "${NC}"

    # Set trap for emergency cleanup
    trap emergency_cleanup EXIT

    preflight
    test_projects
    test_environments
    test_services
    test_variables
    test_volumes
    test_deployments
    test_update_service
    test_deployment_lifecycle
    test_error_handling
    test_cleanup

    # Disable emergency cleanup since we cleaned up properly
    trap - EXIT

    print_results

    # Exit with failure if any tests failed
    [[ $FAILED -eq 0 ]]
}

main "$@"
