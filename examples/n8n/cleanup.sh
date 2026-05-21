#!/usr/bin/env bash
set -euo pipefail

# n8n Stack Cleanup Script
# Deletes all services created by deploy.sh

# Color output helpers
log_info() { echo -e "\033[0;36m[INFO]\033[0m $1"; }
log_success() { echo -e "\033[0;32m[✓]\033[0m $1"; }
log_error() { echo -e "\033[0;31m[✗]\033[0m $1"; }
log_step() { echo -e "\n\033[1;35m[→]\033[0m $1\n"; }

# Read environment variables from .envrc if it exists
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/.envrc" ]; then
    source "$SCRIPT_DIR/.envrc"
fi

# Check required environment variables
REQUIRED_VARS=(
    "RAILWAY_TOKEN"
    "RAILCTL_PROJECT"
    "RAILCTL_ENVIRONMENT"
)

MISSING_VARS=()
for var in "${REQUIRED_VARS[@]}"; do
    if [ -z "${!var:-}" ]; then
        MISSING_VARS+=("$var")
    fi
done

if [ ${#MISSING_VARS[@]} -ne 0 ]; then
    log_error "Missing required environment variables:"
    for var in "${MISSING_VARS[@]}"; do
        echo "  - $var"
    done
    exit 1
fi

# Check railctl is available (use relative path to repo root)
RAILCTL="${RAILCTL:-$SCRIPT_DIR/../../railctl}"

if [ ! -x "$RAILCTL" ]; then
    log_error "railctl binary not found at $RAILCTL"
    log_error "Please build it first: cd ../../ && go build -o railctl ./cmd/railctl"
    exit 1
fi

# Common flags
FLAGS="-p $RAILCTL_PROJECT -e $RAILCTL_ENVIRONMENT"

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  n8n Stack Cleanup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Project:     $RAILCTL_PROJECT"
echo "  Environment: $RAILCTL_ENVIRONMENT"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Confirm deletion
read -p "⚠️  This will DELETE all n8n services and volumes. Continue? [y/N] " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log_info "Cleanup cancelled"
    exit 0
fi

echo ""

# Services to delete (in reverse order of creation)
SERVICES=(
    "Worker"
    "Primary"
    "Redis"
    "Postgres"
)

log_step "Deleting Services"

for service in "${SERVICES[@]}"; do
    log_info "Deleting $service..."
    if $RAILCTL delete service "$service" $FLAGS --yes; then
        log_success "$service deleted"
    else
        log_error "Failed to delete $service (may not exist)"
    fi
done

echo ""
log_step "Deleting Orphaned Volumes"

# Get all volumes and delete them
# Use a simple approach: list all volumes and delete each one
VOLUME_LIST=$($RAILCTL get volumes $FLAGS 2>/dev/null || echo "")

if [ -n "$VOLUME_LIST" ]; then
    # Extract volume names from table output (skip header)
    VOLUME_NAMES=$(echo "$VOLUME_LIST" | tail -n +2 | awk '{print $1}' || true)
    
    if [ -n "$VOLUME_NAMES" ]; then
        while IFS= read -r volume_name; do
            if [ -n "$volume_name" ] && [ "$volume_name" != "NAME" ]; then
                log_info "Deleting volume: $volume_name"
                if $RAILCTL delete volume "$volume_name" $FLAGS --yes 2>/dev/null; then
                    log_success "Volume $volume_name deleted"
                else
                    log_error "Failed to delete volume $volume_name"
                fi
            fi
        done <<< "$VOLUME_NAMES"
    else
        log_info "No volumes found"
    fi
else
    log_info "No volumes found"
fi

echo ""
log_step "Deleting Environment"

read -p "⚠️  Delete environment '$RAILCTL_ENVIRONMENT' from project '$RAILCTL_PROJECT'? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    log_info "Deleting environment '$RAILCTL_ENVIRONMENT'..."
    if $RAILCTL delete environment "$RAILCTL_ENVIRONMENT" -p "$RAILCTL_PROJECT" --yes 2>/dev/null; then
        log_success "Environment '$RAILCTL_ENVIRONMENT' deleted"
    else
        log_error "Failed to delete environment '$RAILCTL_ENVIRONMENT'"
    fi
else
    log_info "Environment kept"
fi

echo ""
log_step "Deleting Project"

read -p "⚠️  Delete project '$RAILCTL_PROJECT' as well? [y/N] " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    log_info "Deleting project '$RAILCTL_PROJECT'..."
    if $RAILCTL delete project "$RAILCTL_PROJECT" --yes 2>/dev/null; then
        log_success "Project '$RAILCTL_PROJECT' deleted"
    else
        log_error "Failed to delete project '$RAILCTL_PROJECT'"
    fi
else
    log_info "Project kept"
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Cleanup Complete"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
log_info "All services and volumes have been deleted from $RAILCTL_ENVIRONMENT"
echo ""
