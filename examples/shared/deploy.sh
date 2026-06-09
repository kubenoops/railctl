#!/usr/bin/env bash
#
# Railway Stack Deployment Script (Generic)
# Declarative deployment using config YAML files as single source of truth.
#
# Usage:
#   ./deploy.sh                                        # Deploy all configs/ in order
#   ./deploy.sh --config n8n-postgres.yaml              # Deploy one specific config
#   ./deploy.sh --config n8n-postgres.yaml --config n8n-redis.yaml
#   ./deploy.sh --skip-wait                             # Don't wait for deployments
#
# Required environment variables:
#   RAILWAY_TOKEN          - Railway API token
#   RAILCTL_PROJECT        - Target project name (created if missing)
#   RAILCTL_ENVIRONMENT    - Target environment name (created if missing)
#
# Config files use $env(VAR) for secrets and ${{service.VAR}} for Railway
# service references. See the example configs for details.
#

set -euo pipefail

# ── Colors ───────────────────────────────────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[✓]${NC} $1"; }
log_error()   { echo -e "${RED}[✗]${NC} $1"; }
log_step()    { echo ""; echo -e "${YELLOW}[→]${NC} $1"; echo ""; }

# ── Resolve paths ────────────────────────────────────────────────────
# CALLER_DIR is where the example's deploy.sh lives (the symlink target's dir
# won't help — we need the directory the user invoked from).
CALLER_DIR="$(cd "$(dirname "${BASH_SOURCE[1]:-${BASH_SOURCE[0]}}")" && pwd)"
CONFIGS_DIR="${CALLER_DIR}/configs"
RAILCTL="${RAILCTL:-$(command -v railctl 2>/dev/null || echo "${CALLER_DIR}/../../railctl")}"

# Source .envrc if present
if [ -f "$CALLER_DIR/.envrc" ]; then
    set -a
    source "$CALLER_DIR/.envrc"
    set +a
fi

# ── Parse arguments ──────────────────────────────────────────────────
CONFIG_FILTERS=()
SKIP_WAIT=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --config)
            [[ -z "${2:-}" ]] && { echo "Error: --config requires a value"; exit 1; }
            CONFIG_FILTERS+=("$2")
            shift 2
            ;;
        --skip-wait)
            SKIP_WAIT=true
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [--config <file.yaml>]... [--skip-wait]"
            echo ""
            echo "Options:"
            echo "  --config <file>   Deploy only the specified config file(s)"
            echo "  --skip-wait       Don't wait for deployments to complete"
            echo "  -h, --help        Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--config <file.yaml>]... [--skip-wait]"
            exit 1
            ;;
    esac
done

# ── Resolve config filters ───────────────────────────────────────────
SELECTED_CONFIGS=()
if [ ${#CONFIG_FILTERS[@]} -gt 0 ]; then
    for filter in "${CONFIG_FILTERS[@]}"; do
        target="$CONFIGS_DIR/$filter"
        if [ -f "$target" ]; then
            SELECTED_CONFIGS+=("$target")
        elif [ -f "$CONFIGS_DIR/${filter}.yaml" ]; then
            SELECTED_CONFIGS+=("$CONFIGS_DIR/${filter}.yaml")
        else
            log_error "Config not found: $target"
            exit 1
        fi
    done
fi

should_deploy() {
    local config_file="$1"
    if [ ${#SELECTED_CONFIGS[@]} -eq 0 ]; then return 0; fi
    for selected in "${SELECTED_CONFIGS[@]}"; do
        [[ "$config_file" = "$selected" ]] && return 0
    done
    return 1
}

# ── Dependency checks ────────────────────────────────────────────────
check_dependencies() {
    local missing=()

    if ! command -v yq &>/dev/null; then
        missing+=("yq")
        log_error "yq is not installed"
        echo "  Install: https://github.com/mikefarah/yq#install"
    fi

    if [ ! -f "$RAILCTL" ] && ! command -v railctl &>/dev/null; then
        missing+=("railctl")
        log_error "railctl not found"
        echo "  Build: cd <repo-root> && go build -o railctl ./cmd/railctl"
    fi

    for var in RAILWAY_TOKEN RAILCTL_PROJECT RAILCTL_ENVIRONMENT; do
        if [ -z "${!var:-}" ]; then
            missing+=("$var")
            log_error "$var is not set"
        fi
    done

    if [ ${#missing[@]} -gt 0 ]; then
        echo ""
        log_error "Missing: ${missing[*]}"
        echo ""
        echo "Tip: cp .envrc.example .envrc && edit .envrc && source .envrc"
        exit 1
    fi
}

check_dependencies

# ── Common flags ─────────────────────────────────────────────────────
RAILCTL_FLAGS=(-p "$RAILCTL_PROJECT" -e "$RAILCTL_ENVIRONMENT")
declare -a DEPLOYED_SERVICES=()

# ── Helpers ──────────────────────────────────────────────────────────
get_config() {
    local config_file="$1" path="$2"
    [ -f "$config_file" ] || { log_error "Config not found: $config_file"; exit 1; }
    yq eval "$path" "$config_file"
}

service_exists() {
    $RAILCTL describe service "$1" "${RAILCTL_FLAGS[@]}" &>/dev/null
}

volume_exists_for_service() {
    $RAILCTL get volumes -s "$1" "${RAILCTL_FLAGS[@]}" 2>/dev/null | grep -q "$1"
}

# Expand $env(VAR) references to actual environment variable values.
# Railway service references (${{svc.VAR}}) are preserved as-is.
expand_env_refs() {
    local value="$1"
    local env_pattern='\$env\(([^)]+)\)'
    while [[ "$value" =~ $env_pattern ]]; do
        local env_var="${BASH_REMATCH[1]}"
        local env_val="${!env_var:-}"
        if [ -z "$env_val" ]; then
            log_error "Environment variable '$env_var' referenced in config but not set"
            exit 1
        fi
        value="${value/\$env($env_var)/$env_val}"
    done
    echo "$value"
}

# ── Core: ensure a service matches its config ────────────────────────
ensure_service_from_config() {
    local config_file="$1"
    local service_name
    service_name=$(get_config "$config_file" '.service.name')

    # Read service configuration
    local image start_command restart_policy max_retries replicas
    image=$(get_config "$config_file" '.service.image')
    start_command=$(get_config "$config_file" '.deploy.startCommand // ""')
    restart_policy=$(get_config "$config_file" '.deploy.restartPolicyType // "ON_FAILURE"')
    max_retries=$(get_config "$config_file" '.deploy.restartPolicyMaxRetries // 10')
    replicas=$(get_config "$config_file" '.deploy.numReplicas // 1')

    # Read networking
    local domain_port tcp_proxy_port
    domain_port=$(get_config "$config_file" '.domain.port // ""')
    tcp_proxy_port=$(get_config "$config_file" '.networking.tcpProxyPort // ""')

    # Read registry credentials
    local registry_username registry_password
    registry_username=$(get_config "$config_file" '.registry.username // ""')
    registry_password=$(get_config "$config_file" '.registry.password // ""')

    # Expand $env() in all fields
    image=$(expand_env_refs "$image")
    start_command=$(expand_env_refs "$start_command")
    [ -n "$domain_port" ]      && domain_port=$(expand_env_refs "$domain_port")
    [ -n "$tcp_proxy_port" ]   && tcp_proxy_port=$(expand_env_refs "$tcp_proxy_port")
    [ -n "$registry_username" ] && registry_username=$(expand_env_refs "$registry_username")
    [ -n "$registry_password" ] && registry_password=$(expand_env_refs "$registry_password")

    # Build the railctl command
    local action="create"
    if service_exists "$service_name"; then
        action="update"
        log_info "Service '$service_name' exists — updating..."
    else
        log_info "Creating '$service_name'..."
    fi

    local cmd="$RAILCTL $action service $service_name --image $image"
    [ "$action" = "update" ] && cmd="$cmd -y"

    if [ -n "$registry_username" ] && [ -n "$registry_password" ]; then
        cmd="$cmd --registry-username \"$registry_username\" --registry-password \"$registry_password\""
    fi
    [ -n "$start_command" ] && cmd="$cmd --start-command $(printf '%q' "$start_command")"
    cmd="$cmd --restart-policy $restart_policy --max-retries $max_retries --replicas $replicas"
    [ -n "$domain_port" ]    && cmd="$cmd --generate-domain $domain_port"
    [ -n "$tcp_proxy_port" ] && cmd="$cmd --generate-tcp $tcp_proxy_port"
    cmd="$cmd ${RAILCTL_FLAGS[*]}"

    if eval "$cmd"; then
        log_success "$service_name ${action}d"
        DEPLOYED_SERVICES+=("$service_name")
    else
        log_error "$service_name $action failed"
    fi

    # ── Volume ───────────────────────────────────────────────────────
    local mount_path
    mount_path=$(get_config "$config_file" '.volume.mountPath // ""')
    if [ -n "$mount_path" ]; then
        if volume_exists_for_service "$service_name"; then
            log_success "Volume for '$service_name' already exists"
        else
            log_info "Creating volume for '$service_name'..."
            $RAILCTL create volume --mount-path "$mount_path" -s "$service_name" "${RAILCTL_FLAGS[@]}"
            log_success "Volume created"
        fi
    fi

    # ── Variables ────────────────────────────────────────────────────
    local vars
    vars=$(yq eval '.variables | to_entries | .[] | .key + "=" + (.value | @json)' "$config_file" 2>/dev/null || echo "")

    if [ -n "$vars" ]; then
        # Validate all $env() references before setting
        local env_refs missing_vars=()
        env_refs=$(grep -oE '\$env\([^)]+\)' "$config_file" | sed 's/\$env(//;s/)//' | sort -u || true)
        if [ -n "$env_refs" ]; then
            while IFS= read -r env_var; do
                [ -z "${!env_var:-}" ] && missing_vars+=("$env_var")
            done <<< "$env_refs"
        fi
        if [ ${#missing_vars[@]} -gt 0 ]; then
            log_error "Missing env vars for '$service_name':"
            printf '  - %s\n' "${missing_vars[@]}"
            exit 1
        fi

        local var_args=()
        while IFS= read -r line; do
            [ -z "$line" ] && continue
            local key="${line%%=*}"
            local value="${line#*=}"
            value=$(echo "$value" | sed 's/^"//;s/"$//')

            # Expand $env(VAR), preserve ${{service.VAR}}
            local env_pattern='\$env\(([^)]+)\)'
            while [[ "$value" =~ $env_pattern ]]; do
                local env_var="${BASH_REMATCH[1]}"
                local env_val="${!env_var}"
                value="${value/\$env($env_var)/$env_val}"
            done

            var_args+=("$key=$value")
        done <<< "$vars"

        if [ ${#var_args[@]} -gt 0 ]; then
            log_info "Setting ${#var_args[@]} variable(s) for '$service_name'..."
            $RAILCTL set variable "${var_args[@]}" -s "$service_name" "${RAILCTL_FLAGS[@]}"
            log_success "Variables set"
        fi
    fi
}

# ── Header ───────────────────────────────────────────────────────────
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  railctl — Declarative Deployment"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Project:     $RAILCTL_PROJECT"
echo "  Environment: $RAILCTL_ENVIRONMENT"
echo "  Configs:     $CONFIGS_DIR"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# ── Phase 0: Ensure project & environment exist ──────────────────────
log_step "Phase 0: Project & Environment"

if $RAILCTL describe project "$RAILCTL_PROJECT" &>/dev/null; then
    log_success "Project '$RAILCTL_PROJECT' exists"
else
    log_info "Creating project '$RAILCTL_PROJECT'..."
    $RAILCTL create project "$RAILCTL_PROJECT"
    log_success "Project created"
fi

if $RAILCTL get environments -p "$RAILCTL_PROJECT" 2>/dev/null | grep -q "$RAILCTL_ENVIRONMENT"; then
    log_success "Environment '$RAILCTL_ENVIRONMENT' exists"
else
    log_info "Creating environment '$RAILCTL_ENVIRONMENT'..."
    $RAILCTL create environment "$RAILCTL_ENVIRONMENT" -p "$RAILCTL_PROJECT"
    log_success "Environment created"
fi

# ── Phase 1: Deploy configs ──────────────────────────────────────────
log_step "Phase 1: Deploy Services"

config_count=0
for config_file in "$CONFIGS_DIR"/*.yaml; do
    [ -f "$config_file" ] || continue
    if should_deploy "$config_file"; then
        ensure_service_from_config "$config_file"
        echo ""
        ((config_count++)) || true
    fi
done

if [ "$config_count" -eq 0 ]; then
    log_error "No config files found in $CONFIGS_DIR"
    exit 1
fi

# ── Phase 2: Await deployments ───────────────────────────────────────
log_step "Phase 2: Awaiting Deployments"

declare -a deploy_status=()
declare -a deploy_fail_count=()

if [ "$SKIP_WAIT" = true ]; then
    log_info "--skip-wait specified, skipping deployment status check"
elif [ ${#DEPLOYED_SERVICES[@]} -eq 0 ]; then
    log_info "No new deployments to await"
else
    log_info "Waiting for ${#DEPLOYED_SERVICES[@]} service(s)..."
    echo ""

    for i in "${!DEPLOYED_SERVICES[@]}"; do
        deploy_status[$i]="PENDING"
        deploy_fail_count[$i]=0
    done

    poll_interval=10
    max_fail_retries=4
    pending_count=${#DEPLOYED_SERVICES[@]}

    while [ "$pending_count" -gt 0 ]; do
        pending_count=0
        for i in "${!DEPLOYED_SERVICES[@]}"; do
            svc="${DEPLOYED_SERVICES[$i]}"
            [ "${deploy_status[$i]}" != "PENDING" ] && continue

            status=$($RAILCTL get deployments -s "$svc" -o json --limit 1 "${RAILCTL_FLAGS[@]}" 2>/dev/null \
                | python3 -c "import sys,json; d=json.load(sys.stdin); print(d[0]['status'] if d else 'UNKNOWN')" 2>/dev/null || echo "UNKNOWN")

            case "$status" in
                SUCCESS)
                    log_success "$svc: $status"
                    deploy_status[$i]="SUCCESS"
                    ;;
                FAILED|CRASHED|REMOVED|SKIPPED)
                    deploy_fail_count[$i]=$(( deploy_fail_count[$i] + 1 ))
                    if [ "${deploy_fail_count[$i]}" -ge "$max_fail_retries" ]; then
                        log_error "$svc: $status (confirmed)"
                        deploy_status[$i]="FAILED"
                    else
                        log_info "$svc: $status (may be superseded — recheck ${deploy_fail_count[$i]}/$max_fail_retries)"
                        (( pending_count++ )) || true
                    fi
                    ;;
                *)
                    deploy_fail_count[$i]=0
                    log_info "$svc: ${status:-UNKNOWN} (waiting...)"
                    (( pending_count++ )) || true
                    ;;
            esac
        done

        [ "$pending_count" -gt 0 ] && sleep $poll_interval
    done
fi

# ── Summary ──────────────────────────────────────────────────────────
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Deployment Results"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

succeeded=0
failed=0
for i in "${!DEPLOYED_SERVICES[@]}"; do
    svc="${DEPLOYED_SERVICES[$i]}"
    result="${deploy_status[$i]:-SKIPPED}"
    if [ "$result" = "SUCCESS" ] || [ "$result" = "SKIPPED" ]; then
        log_success "$svc"
        ((succeeded++)) || true
    else
        log_error "$svc ($result)"
        ((failed++)) || true
    fi
done

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  $succeeded succeeded, $failed failed"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
log_info "Inspect: railctl get services -p \"$RAILCTL_PROJECT\" -e \"$RAILCTL_ENVIRONMENT\""
echo ""

[ "$failed" -gt 0 ] && exit 1 || exit 0
