#!/usr/bin/env bash
#
# Railway Stack Cleanup Script (Generic)
# Removes all services and volumes deployed by deploy.sh.
#
# Usage:
#   ./cleanup.sh              # Interactive cleanup of all services
#   ./cleanup.sh --yes        # Skip confirmation prompts
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
AUTO_YES=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --yes|-y) AUTO_YES=true; shift ;;
        -h|--help)
            echo "Usage: $0 [--yes]"
            echo "  --yes, -y   Skip confirmation prompts"
            exit 0
            ;;
        *) echo "Unknown option: $1"; exit 1 ;;
    esac
done

# ── Dependency checks ────────────────────────────────────────────────
# Only the token is unconditionally required; with a project token the
# project/environment are derived from the token via `railctl whoami`.
if [ -z "${RAILWAY_TOKEN:-}" ]; then
    log_error "RAILWAY_TOKEN is not set"
    exit 1
fi

if [ ! -f "$RAILCTL" ] && ! command -v railctl &>/dev/null; then
    log_error "railctl not found"
    exit 1
fi

# ── Token preflight ──────────────────────────────────────────────────
# Service/volume deletion works with any token type (in-scope for a project
# token, no -p/-e needed). Environment/project deletion is workspace-scope:
# those prompts are skipped for project tokens.
WHOAMI_JSON=$($RAILCTL whoami -o json 2>/dev/null) || {
    log_error "Token check failed — is RAILWAY_TOKEN valid? (railctl whoami)"
    exit 1
}
TOKEN_TYPE=$(echo "$WHOAMI_JSON" | python3 -c 'import sys,json;print(json.load(sys.stdin)["type"])')

if [ "$TOKEN_TYPE" = "project" ]; then
    RAILCTL_PROJECT="${RAILCTL_PROJECT:-$(echo "$WHOAMI_JSON" | python3 -c 'import sys,json;print(json.load(sys.stdin)["project"]["name"])')}"
    RAILCTL_ENVIRONMENT="${RAILCTL_ENVIRONMENT:-$(echo "$WHOAMI_JSON" | python3 -c 'import sys,json;print(json.load(sys.stdin)["environment"]["name"])')}"
    RAILCTL_FLAGS=()
else
    for var in RAILCTL_PROJECT RAILCTL_ENVIRONMENT; do
        if [ -z "${!var:-}" ]; then
            log_error "$var is required with a $TOKEN_TYPE token"
            exit 1
        fi
    done
    RAILCTL_FLAGS=(-p "$RAILCTL_PROJECT" -e "$RAILCTL_ENVIRONMENT")
fi

# ── Header ───────────────────────────────────────────────────────────
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  railctl — Stack Cleanup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Project:     $RAILCTL_PROJECT"
echo "  Environment: $RAILCTL_ENVIRONMENT"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# ── Confirm ──────────────────────────────────────────────────────────
if [ "$AUTO_YES" != true ]; then
    read -p "⚠️  This will DELETE all services and volumes. Continue? [y/N] " -n 1 -r
    echo
    [[ ! $REPLY =~ ^[Yy]$ ]] && { log_info "Cancelled"; exit 0; }
fi

# ── Discover services from configs (reverse order) ───────────────────
log_step "Deleting Services"

SERVICE_NAMES=()
if [ -d "$CONFIGS_DIR" ]; then
    for config_file in "$CONFIGS_DIR"/*.yaml; do
        [ -f "$config_file" ] || continue
        name=$(yq eval '.service.name' "$config_file" 2>/dev/null || true)
        [ -n "$name" ] && SERVICE_NAMES+=("$name")
    done
fi

# Reverse the array (delete in reverse dependency order)
REVERSED=()
for ((i=${#SERVICE_NAMES[@]}-1; i>=0; i--)); do
    REVERSED+=("${SERVICE_NAMES[$i]}")
done

for service in "${REVERSED[@]}"; do
    log_info "Deleting $service..."
    if $RAILCTL delete service "$service" "${RAILCTL_FLAGS[@]}" --yes 2>/dev/null; then
        log_success "$service deleted"
    else
        log_error "Failed to delete $service (may not exist)"
    fi
done

# ── Orphaned volumes ─────────────────────────────────────────────────
log_step "Deleting Orphaned Volumes"

VOLUME_LIST=$($RAILCTL get volumes "${RAILCTL_FLAGS[@]}" 2>/dev/null || echo "")
if [ -n "$VOLUME_LIST" ]; then
    VOLUME_NAMES=$(echo "$VOLUME_LIST" | tail -n +2 | awk '{print $1}' || true)
    if [ -n "$VOLUME_NAMES" ]; then
        while IFS= read -r vol; do
            [ -n "$vol" ] && [ "$vol" != "NAME" ] || continue
            log_info "Deleting volume: $vol"
            $RAILCTL delete volume "$vol" "${RAILCTL_FLAGS[@]}" --yes 2>/dev/null && \
                log_success "Volume $vol deleted" || \
                log_error "Failed to delete volume $vol"
        done <<< "$VOLUME_NAMES"
    else
        log_info "No volumes found"
    fi
else
    log_info "No volumes found"
fi

# ── Optional: delete environment & project ───────────────────────────
if [ "$TOKEN_TYPE" = "project" ]; then
    log_info "Project token: environment/project deletion requires a workspace or account token — skipping"
elif [ "$AUTO_YES" != true ]; then
    echo ""
    read -p "Delete environment '$RAILCTL_ENVIRONMENT'? [y/N] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        $RAILCTL delete environment "$RAILCTL_ENVIRONMENT" -p "$RAILCTL_PROJECT" --yes 2>/dev/null && \
            log_success "Environment deleted" || log_error "Failed to delete environment"
    fi

    read -p "Delete project '$RAILCTL_PROJECT'? [y/N] " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        $RAILCTL delete project "$RAILCTL_PROJECT" --yes 2>/dev/null && \
            log_success "Project deleted" || log_error "Failed to delete project"
    fi
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "  Cleanup Complete"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
