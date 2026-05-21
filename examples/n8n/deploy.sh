#!/usr/bin/env bash
#
# n8n Stack Deployment Script
# Declarative deployment using config.yaml files as single source of truth
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

log_step() {
    echo ""
    echo -e "${YELLOW}[→]${NC} $1"
    echo ""
}

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RAILCTL="${RAILCTL:-$SCRIPT_DIR/../../railctl}"


# Check for required tools
check_dependencies() {
    local missing_deps=()
    
    # Check for yq
    if ! command -v yq &> /dev/null; then
        missing_deps+=("yq")
        log_error "yq is not installed"
        echo "  Install with: sudo snap install yq"
        echo "  Or visit: https://github.com/mikefarah/yq"
    fi
    
    # Check for railctl
    if [ ! -f "$RAILCTL" ]; then
        missing_deps+=("railctl")
        log_error "railctl not found at $RAILCTL"
        echo "  Build it with: cd $SCRIPT_DIR/../.. && go build -o railctl ./cmd/railctl"
    fi
    
    # Check for required environment variables
    if [ -z "${RAILWAY_TOKEN:-}" ]; then
        missing_deps+=("RAILWAY_TOKEN")
        log_error "RAILWAY_TOKEN environment variable is not set"
        echo "  Set it with: export RAILWAY_TOKEN=your-token"
    fi
    
    if [ -z "${RAILCTL_PROJECT:-}" ]; then
        missing_deps+=("RAILCTL_PROJECT")
        log_error "RAILCTL_PROJECT environment variable is not set"
        echo "  Set it with: export RAILCTL_PROJECT=your-project"
    fi
    
    if [ -z "${RAILCTL_ENVIRONMENT:-}" ]; then
        missing_deps+=("RAILCTL_ENVIRONMENT")
        log_error "RAILCTL_ENVIRONMENT environment variable is not set"
        echo "  Set it with: export RAILCTL_ENVIRONMENT=your-environment"
    fi
    
    # If any dependencies are missing, exit
    if [ ${#missing_deps[@]} -gt 0 ]; then
        echo ""
        log_error "Missing dependencies: ${missing_deps[*]}"
        echo ""
        echo "Tip: Source .envrc to load environment variables:"
        echo "  direnv allow  # if using direnv"
        echo "  source .envrc # or manually source"
        exit 1
    fi
}

# Read environment variables from .envrc if it exists
if [ -f "$SCRIPT_DIR/.envrc" ]; then
    source "$SCRIPT_DIR/.envrc"
fi

# Check all dependencies before proceeding
check_dependencies

# Common flags
FLAGS="-p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT"

# Helper function to read config values
get_config() {
    local service_dir="$1"
    local path="$2"
    local config_file="$service_dir/config.yaml"
    
    if [ ! -f "$config_file" ]; then
        log_error "Config file not found: $config_file"
        exit 1
    fi
    
    yq eval "$path" "$config_file"
}

# Helper function to check if a service exists
service_exists() {
    local service_name="$1"
    $RAILCTL describe service "$service_name" $FLAGS &>/dev/null
}

# Helper function to check if a volume exists for a service
volume_exists_for_service() {
    local service_name="$1"
    $RAILCTL get volumes -s "$service_name" $FLAGS 2>/dev/null | grep -q "$service_name"
}

# Helper function to ensure service exists (create or update)
ensure_service_from_config() {
    local service_dir="$1"
    local service_name=$(get_config "$service_dir" '.service.name')
    
    # Read service configuration
    local image=$(get_config "$service_dir" '.service.image')
    local start_command=$(get_config "$service_dir" '.deploy.startCommand // ""')
    local restart_policy=$(get_config "$service_dir" '.deploy.restartPolicyType // "ON_FAILURE"')
    local max_retries=$(get_config "$service_dir" '.deploy.restartPolicyMaxRetries // 10')
    local replicas=$(get_config "$service_dir" '.deploy.numReplicas // 1')
    
    # Read registry credentials
    local registry_username=$(get_config "$service_dir" '.registry.username // ""')
    local registry_password=$(get_config "$service_dir" '.registry.password // ""')
    
    if [ -n "$registry_username" ] && [ -n "$registry_password" ]; then
        # Expand environment variables in credentials
        registry_username=$(eval echo "$registry_username")
        registry_password=$(eval echo "$registry_password")
    fi
    
    # Determine if service already exists
    if service_exists "$service_name"; then
        log_info "Service '$service_name' already exists, updating..."
        
        # Build update service command
        local cmd="$RAILCTL update service $service_name --image $image -y"
        
        if [ -n "$registry_username" ] && [ -n "$registry_password" ]; then
            cmd="$cmd --registry-username \"$registry_username\" --registry-password \"$registry_password\""
        fi
        
        if [ -n "$start_command" ]; then
            cmd="$cmd --start-command $(printf '%q' "$start_command")"
        fi
        
        cmd="$cmd --restart-policy $restart_policy --max-retries $max_retries --replicas $replicas"
        cmd="$cmd $FLAGS"
        
        eval "$cmd"
        log_success "$service_name updated"
    else
        log_info "Creating $service_name..."
        
        # Build create service command
        local cmd="$RAILCTL create service $service_name --image $image"
        
        if [ -n "$registry_username" ] && [ -n "$registry_password" ]; then
            cmd="$cmd --registry-username \"$registry_username\" --registry-password \"$registry_password\""
        fi
        
        if [ -n "$start_command" ]; then
            cmd="$cmd --start-command $(printf '%q' "$start_command")"
        fi
        
        cmd="$cmd --restart-policy $restart_policy --max-retries $max_retries --replicas $replicas"
        cmd="$cmd $FLAGS"
        
        eval "$cmd"
        log_success "$service_name created"
    fi
    
    # Ensure volume exists if specified
    local mount_path=$(get_config "$service_dir" '.volume.mountPath // ""')
    if [ -n "$mount_path" ]; then
        if volume_exists_for_service "$service_name"; then
            log_success "Volume for '$service_name' already exists, skipping"
        else
            log_info "Creating volume for '$service_name'..."
            $RAILCTL create volume --mount-path "$mount_path" -s "$service_name" $FLAGS
            log_success "Volume for '$service_name' created"
        fi
    fi
    
    # Set variables (set variable is already idempotent - upserts)
    local vars=$(yq eval '.variables | to_entries | .[] | .key + "=" + (.value | @json)' "$service_dir/config.yaml" 2>/dev/null || echo "")
    
    if [ -n "$vars" ]; then
        # Build variable arguments
        local var_args=()
        while IFS= read -r line; do
            if [ -n "$line" ]; then
                # Extract key=value
                local key="${line%%=*}"
                local value="${line#*=}"
                # Remove quotes from JSON value
                value=$(echo "$value" | sed 's/^"//;s/"$//')
                
                # Only expand ${VAR} if it's NOT a Railway service reference (${{service.VAR}})
                # Railway service references should be preserved as-is
                if [[ "$value" == *'${{'* ]]; then
                    # This is a Railway service reference, keep it as-is
                    var_args+=("$key=$value")
                elif [[ "$value" == *'${'* ]]; then
                    # This is an environment variable reference, expand it
                    # Use eval with nounset disabled temporarily to handle missing vars
                    expanded_value=$(set +u; eval echo "\"$value\"")
                    var_args+=("$key=$expanded_value")
                else
                    # No variable reference, use as-is
                    var_args+=("$key=$value")
                fi
            fi
        done <<< "$vars"
        
        if [ ${#var_args[@]} -gt 0 ]; then
            log_info "Setting variables for '$service_name'..."
            $RAILCTL set variable "${var_args[@]}" -s "$service_name" $FLAGS
            log_success "Variables for '$service_name' set"
        fi
    fi
}

# Display header
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  n8n Stack Deployment"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Project:     $RAILCTL_PROJECT"
echo "  Environment: $RAILCTL_ENVIRONMENT"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

#
# PHASE 0: Ensure Project Exists
#
log_step "Phase 0: Ensure Project Exists"

if $RAILCTL describe project "$RAILCTL_PROJECT" &>/dev/null; then
    log_success "Project '$RAILCTL_PROJECT' already exists"
else
    log_info "Creating project '$RAILCTL_PROJECT'..."
    $RAILCTL create project "$RAILCTL_PROJECT"
    log_success "Project '$RAILCTL_PROJECT' created"
fi

# Ensure environment exists
if $RAILCTL get environments -p "$RAILCTL_PROJECT" 2>/dev/null | grep -q "$RAILCTL_ENVIRONMENT"; then
    log_success "Environment '$RAILCTL_ENVIRONMENT' already exists"
else
    log_info "Creating environment '$RAILCTL_ENVIRONMENT'..."
    $RAILCTL create environment "$RAILCTL_ENVIRONMENT" -p "$RAILCTL_PROJECT"
    log_success "Environment '$RAILCTL_ENVIRONMENT' created"
fi
echo ""

#
# PHASE 1: Infrastructure Services
#
log_step "Phase 1: Infrastructure Services"

# 1. PostgreSQL
ensure_service_from_config "$SCRIPT_DIR/n8n-postgres"
echo ""

# 2. Redis
ensure_service_from_config "$SCRIPT_DIR/n8n-redis"
echo ""

#
# PHASE 2: Application Services
#
log_step "Phase 2: Application Services"

# 3. n8n Primary
ensure_service_from_config "$SCRIPT_DIR/n8n-primary"
echo ""

# 4. n8n Worker
ensure_service_from_config "$SCRIPT_DIR/n8n-worker"
echo ""

# Display completion
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Deployment Complete"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
log_success "Postgres - Database with volume"
log_success "Redis - Cache with volume"
log_success "Primary - Main application"
log_success "Worker - Worker nodes"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
log_info "Check services: railctl get services -p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT"
echo ""
