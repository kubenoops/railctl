#!/usr/bin/env python3
"""
Docker Compose → Railway Importer

Imports a docker-compose.yml into an existing Railway project/environment,
creating and configuring services idempotently.

Usage:
    python compose_importer.py docker-compose.yml \
        --project-id <id> \
        --environment-id <id> \
        [--token <token>] \
        [--dry-run]

Environment:
    RAILWAY_TOKEN - API token (alternative to --token)
"""

import argparse
import json
import os
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Optional

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML is required. Install with: pip install pyyaml", file=sys.stderr)
    sys.exit(1)


# ─── Constants ───────────────────────────────────────────────────────────────

API_URL = "https://backboard.railway.com/graphql/v2"

# Restart policy mapping: compose → Railway
RESTART_POLICY_MAP = {
    "no": "NEVER",
    "always": "ALWAYS",
    "on-failure": "ON_FAILURE",
    "unless-stopped": "ALWAYS",  # closest equivalent
}

# Keys that indicate a variable is a secret (for log masking heuristic)
SECRET_KEY_PATTERNS = re.compile(
    r"(PASSWORD|SECRET|TOKEN|KEY|CREDENTIAL|PRIVATE)", re.IGNORECASE
)

# Variables injected by Railway — drop these during import
RAILWAY_INJECTED_PREFIX = "RAILWAY_"


# ─── Colors ──────────────────────────────────────────────────────────────────

class C:
    """ANSI color helpers."""
    RED = "\033[0;31m"
    GREEN = "\033[0;32m"
    YELLOW = "\033[1;33m"
    BLUE = "\033[0;34m"
    CYAN = "\033[0;36m"
    DIM = "\033[2m"
    BOLD = "\033[1m"
    NC = "\033[0m"


def log_info(msg: str) -> None:
    print(f"{C.BLUE}[INFO]{C.NC} {msg}")


def log_success(msg: str) -> None:
    print(f"{C.GREEN}[OK]{C.NC} {msg}")


def log_warn(msg: str) -> None:
    print(f"{C.YELLOW}[WARN]{C.NC} {msg}")


def log_error(msg: str) -> None:
    print(f"{C.RED}[ERROR]{C.NC} {msg}", file=sys.stderr)


def log_step(step: int, msg: str) -> None:
    print(f"\n{C.CYAN}[STEP {step}]{C.NC} {msg}")


def log_dry(msg: str) -> None:
    print(f"{C.DIM}  [dry-run] {msg}{C.NC}")


# ─── Data Classes ────────────────────────────────────────────────────────────

@dataclass
class ImportWarning:
    """A non-fatal issue collected during parsing/validation."""
    service: Optional[str]
    feature: str
    message: str


@dataclass
class VolumeConfig:
    """A volume to create in Railway."""
    mount_path: str
    source: str  # named volume name or host path
    is_bind_mount: bool


@dataclass
class DeployConfig:
    """Mapped deploy configuration for a Railway service."""
    start_command: Optional[str] = None
    restart_policy_type: Optional[str] = None
    restart_policy_max_retries: Optional[int] = None
    num_replicas: Optional[int] = None
    healthcheck_path: Optional[str] = None
    healthcheck_timeout: Optional[int] = None

    def to_api_input(self) -> dict:
        """Convert to ServiceInstanceUpdateInput JSON."""
        result = {}
        if self.start_command is not None:
            result["startCommand"] = self.start_command
        if self.restart_policy_type is not None:
            result["restartPolicyType"] = self.restart_policy_type
        if self.restart_policy_max_retries is not None:
            result["restartPolicyMaxRetries"] = self.restart_policy_max_retries
        if self.num_replicas is not None:
            result["numReplicas"] = self.num_replicas
        if self.healthcheck_path is not None:
            result["healthcheckPath"] = self.healthcheck_path
        if self.healthcheck_timeout is not None:
            result["healthcheckTimeout"] = self.healthcheck_timeout
        return result


@dataclass
class ServiceConfig:
    """Fully resolved configuration for a single service to push to Railway."""
    name: str
    image: str
    variables: dict[str, str] = field(default_factory=dict)
    deploy: DeployConfig = field(default_factory=DeployConfig)
    volumes: list[VolumeConfig] = field(default_factory=list)
    internal_ports: list[int] = field(default_factory=list)


@dataclass
class ImportPlan:
    """Complete import plan ready for execution."""
    services: list[ServiceConfig]
    warnings: list[ImportWarning]
    secrets_files: list[str]  # files matching *.secrets convention


# ─── Compose Parser ─────────────────────────────────────────────────────────

def parse_compose_file(path: Path) -> dict:
    """Parse a docker-compose.yml file and return the raw dict."""
    if not path.exists():
        log_error(f"Compose file not found: {path}")
        sys.exit(1)

    with open(path) as f:
        data = yaml.safe_load(f)

    if not isinstance(data, dict):
        log_error("Invalid compose file: expected a YAML mapping at root.")
        sys.exit(1)

    return data


def resolve_env_files(
    service_name: str,
    env_file_spec: Any,
    compose_dir: Path,
) -> tuple[dict[str, str], list[str], list[ImportWarning]]:
    """
    Parse env_file: references and return (variables, secrets_files, warnings).

    Files matching *.secrets are tracked for log masking.
    """
    variables: dict[str, str] = {}
    secrets_files: list[str] = []
    warnings: list[ImportWarning] = []

    if env_file_spec is None:
        return variables, secrets_files, warnings

    # Normalize to list
    if isinstance(env_file_spec, str):
        file_list = [env_file_spec]
    elif isinstance(env_file_spec, list):
        # Each item can be a string or a dict with path/required keys
        file_list = []
        for item in env_file_spec:
            if isinstance(item, str):
                file_list.append(item)
            elif isinstance(item, dict):
                file_list.append(item.get("path", ""))
            else:
                file_list.append(str(item))
    else:
        file_list = [str(env_file_spec)]

    for env_path_str in file_list:
        if not env_path_str:
            continue
        env_path = compose_dir / env_path_str
        if not env_path.exists():
            log_error(
                f"Service '{service_name}': env_file '{env_path_str}' not found. "
                f"Ensure the file exists relative to the compose file."
            )
            sys.exit(1)

        # Track secrets files
        if env_path.name.endswith(".secrets"):
            secrets_files.append(env_path_str)

        # Parse .env file
        with open(env_path) as f:
            for line_num, line in enumerate(f, 1):
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                if "=" not in line:
                    warnings.append(ImportWarning(
                        service=service_name,
                        feature="env_file",
                        message=f"Skipping malformed line {line_num} in '{env_path_str}': no '=' found.",
                    ))
                    continue
                key, _, value = line.partition("=")
                key = key.strip()
                # Strip surrounding quotes
                value = value.strip()
                if (value.startswith('"') and value.endswith('"')) or \
                   (value.startswith("'") and value.endswith("'")):
                    value = value[1:-1]
                variables[key] = value

    return variables, secrets_files, warnings


def substitute_variables(
    value: str,
    env: dict[str, str],
) -> tuple[str, list[str]]:
    """
    Resolve ${VAR} and ${VAR:-default} in a string.

    Returns (resolved_value, list_of_unresolved_var_names).
    Does NOT touch ${{...}} (Railway service references).
    """
    unresolved: list[str] = []

    def replacer(match: re.Match) -> str:
        full = match.group(0)
        # Skip Railway references ${{...}}
        if full.startswith("${{"):
            return full
        var_expr = match.group(1)
        # Handle ${VAR:-default}
        if ":-" in var_expr:
            var_name, default = var_expr.split(":-", 1)
        elif "-" in var_expr:
            var_name, default = var_expr.split("-", 1)
        else:
            var_name = var_expr
            default = None

        var_name = var_name.strip()
        resolved = env.get(var_name, os.environ.get(var_name))
        if resolved is not None:
            return resolved
        if default is not None:
            return default
        unresolved.append(var_name)
        return full

    # Match ${VAR} but not ${{VAR}} (Railway references)
    result = re.sub(r"\$\{([^{}]+)\}", replacer, value)
    return result, unresolved


def parse_environment_block(
    env_spec: Any,
) -> dict[str, str]:
    """Parse the environment: block (list or dict form) into a dict."""
    if env_spec is None:
        return {}

    if isinstance(env_spec, dict):
        return {k: str(v) if v is not None else "" for k, v in env_spec.items()}

    if isinstance(env_spec, list):
        result = {}
        for item in env_spec:
            item = str(item)
            if "=" in item:
                key, _, value = item.partition("=")
                result[key.strip()] = value.strip()
            else:
                # Key-only: inherit from host environment
                val = os.environ.get(item.strip(), "")
                result[item.strip()] = val
        return result

    return {}


def extract_internal_ports(ports_spec: Any) -> list[int]:
    """
    Extract internal (container) ports from a ports: definition.

    Examples:
      - "8080:3000" → [3000]
      - "3000" → [3000]
      - {"target": 3000, "published": 8080} → [3000]
    """
    if ports_spec is None:
        return []

    ports: list[int] = []
    if not isinstance(ports_spec, list):
        return ports

    for entry in ports_spec:
        if isinstance(entry, dict):
            target = entry.get("target")
            if target is not None:
                ports.append(int(target))
        elif isinstance(entry, (str, int)):
            entry_str = str(entry)
            # Handle "host:container" or "host:container/protocol"
            parts = entry_str.split(":")
            last_part = parts[-1].split("/")[0]  # strip /tcp, /udp
            try:
                ports.append(int(last_part))
            except ValueError:
                pass

    return sorted(set(ports))


def extract_healthcheck(
    hc_spec: dict | None,
    service_name: str,
) -> tuple[Optional[str], Optional[int], list[ImportWarning]]:
    """
    Try to extract an HTTP healthcheck path and timeout from compose healthcheck.

    Returns (path, timeout_seconds, warnings).
    """
    warnings: list[ImportWarning] = []
    if hc_spec is None:
        return None, None, warnings

    path = None
    timeout = None

    # Try to extract path from test command
    test = hc_spec.get("test")
    if test:
        test_str = " ".join(test) if isinstance(test, list) else str(test)
        # Look for curl/wget patterns: curl http://localhost:PORT/path
        url_match = re.search(r"https?://(?:localhost|127\.0\.0\.1)(?::\d+)?(\/[^\s\"']*)", test_str)
        if url_match:
            path = url_match.group(1)
        elif "curl" not in test_str.lower() and "wget" not in test_str.lower():
            warnings.append(ImportWarning(
                service=service_name,
                feature="healthcheck",
                message=f"Non-HTTP healthcheck not supported on Railway. Skipping: {test_str[:80]}",
            ))

    # Parse timeout
    timeout_val = hc_spec.get("timeout")
    if timeout_val:
        timeout = _parse_duration_seconds(timeout_val)

    # Warn about unsupported fields
    for f in ("interval", "retries", "start_period", "start_interval"):
        if hc_spec.get(f):
            warnings.append(ImportWarning(
                service=service_name,
                feature="healthcheck",
                message=f"Healthcheck '{f}' is managed by Railway automatically. Ignoring.",
            ))

    return path, timeout, warnings


def _parse_duration_seconds(val: Any) -> int:
    """Parse a compose duration (e.g. '30s', '1m', 10) into seconds."""
    if isinstance(val, (int, float)):
        return int(val)
    val_str = str(val).strip().lower()
    match = re.match(r"^(\d+)(s|m|h)?$", val_str)
    if match:
        num = int(match.group(1))
        unit = match.group(2) or "s"
        if unit == "m":
            return num * 60
        if unit == "h":
            return num * 3600
        return num
    return 30  # fallback default


def parse_deploy_config(
    service_name: str,
    compose_service: dict,
) -> tuple[DeployConfig, list[ImportWarning]]:
    """Parse compose deploy/restart/command/healthcheck into a Railway DeployConfig."""
    warnings: list[ImportWarning] = []
    config = DeployConfig()

    # command → startCommand
    command = compose_service.get("command")
    entrypoint = compose_service.get("entrypoint")
    if command is not None:
        if isinstance(command, list):
            config.start_command = " ".join(str(c) for c in command)
        else:
            config.start_command = str(command)
    elif entrypoint is not None:
        if isinstance(entrypoint, list):
            config.start_command = " ".join(str(e) for e in entrypoint)
        else:
            config.start_command = str(entrypoint)

    # restart: policy (top-level)
    restart = compose_service.get("restart")
    if restart:
        restart_str = str(restart).strip().lower()
        mapped = RESTART_POLICY_MAP.get(restart_str)
        if mapped:
            config.restart_policy_type = mapped
        else:
            warnings.append(ImportWarning(
                service=service_name,
                feature="restart",
                message=f"Unknown restart policy '{restart}'. Defaulting to ON_FAILURE.",
            ))
            config.restart_policy_type = "ON_FAILURE"

    # deploy: section
    deploy = compose_service.get("deploy", {})
    if deploy:
        # replicas
        replicas = deploy.get("replicas")
        if replicas is not None:
            config.num_replicas = int(replicas)

        # restart_policy (inside deploy:)
        rp = deploy.get("restart_policy", {})
        if rp:
            condition = rp.get("condition", "").lower()
            mapped = RESTART_POLICY_MAP.get(condition)
            if mapped:
                config.restart_policy_type = mapped
            max_attempts = rp.get("max_attempts")
            if max_attempts is not None:
                config.restart_policy_max_retries = int(max_attempts)

        # Warn about unsupported deploy features
        for feat, msg in [
            ("resources", "Resource limits managed by Railway plan tier, not per-service."),
            ("placement", "Placement constraints not supported. Use Railway region configuration."),
            ("update_config", "Rolling update config managed by Railway automatically."),
            ("rollback_config", "Rollback config managed by Railway automatically."),
        ]:
            if deploy.get(feat):
                warnings.append(ImportWarning(service=service_name, feature=f"deploy.{feat}", message=msg))

    # healthcheck
    hc_path, hc_timeout, hc_warnings = extract_healthcheck(
        compose_service.get("healthcheck"), service_name
    )
    config.healthcheck_path = hc_path
    config.healthcheck_timeout = hc_timeout
    warnings.extend(hc_warnings)

    return config, warnings


def parse_volumes(
    service_name: str,
    compose_service: dict,
    top_level_volumes: dict,
) -> tuple[list[VolumeConfig], list[ImportWarning]]:
    """Parse volume mounts from a compose service."""
    warnings: list[ImportWarning] = []
    volumes: list[VolumeConfig] = []

    vol_spec = compose_service.get("volumes")
    if not vol_spec:
        return volumes, warnings

    for entry in vol_spec:
        if isinstance(entry, dict):
            # Long syntax
            source = entry.get("source", "")
            target = entry.get("target", "")
            vol_type = entry.get("type", "volume")
        elif isinstance(entry, str):
            # Short syntax: source:target[:mode]
            parts = entry.split(":")
            if len(parts) >= 2:
                source = parts[0]
                target = parts[1]
            else:
                source = ""
                target = parts[0]
            # Detect bind mount by checking for path-like source
            vol_type = "bind" if source.startswith((".", "/", "~")) else "volume"
        else:
            continue

        if not target:
            continue

        is_bind = vol_type == "bind"
        if is_bind:
            warnings.append(ImportWarning(
                service=service_name,
                feature="volumes",
                message=(
                    f"Bind mount '{source}:{target}' — Railway volume created at "
                    f"'{target}'. Data won't be pre-populated."
                ),
            ))

        volumes.append(VolumeConfig(
            mount_path=target,
            source=source,
            is_bind_mount=is_bind,
        ))

    return volumes, warnings


# ─── Validator ───────────────────────────────────────────────────────────────

def validate_compose(data: dict) -> tuple[dict, list[ImportWarning]]:
    """
    Validate a parsed compose file.

    Returns the services dict and collected warnings.
    Hard failures call sys.exit().
    """
    warnings: list[ImportWarning] = []

    # Check version (informational)
    version = data.get("version")
    if version:
        warnings.append(ImportWarning(
            service=None,
            feature="version",
            message=f"Compose file version '{version}' noted. The importer uses the services spec regardless.",
        ))

    # Must have services
    services = data.get("services")
    if not services or not isinstance(services, dict):
        log_error("No services found in compose file.")
        sys.exit(1)

    # Warn about top-level features we skip
    if data.get("networks"):
        for net_name in data["networks"]:
            warnings.append(ImportWarning(
                service=None,
                feature="networks",
                message=(
                    f"Custom network '{net_name}' ignored. "
                    f"Railway provides automatic private networking between all services."
                ),
            ))

    if data.get("secrets"):
        warnings.append(ImportWarning(
            service=None,
            feature="secrets",
            message="Docker secrets not supported. Consider using Railway variables instead.",
        ))

    if data.get("configs"):
        warnings.append(ImportWarning(
            service=None,
            feature="configs",
            message="Docker configs not supported. Consider embedding configuration via environment variables.",
        ))

    # Per-service validation
    for svc_name, svc_data in services.items():
        if not isinstance(svc_data, dict):
            continue

        image = svc_data.get("image")
        build = svc_data.get("build")

        # Hard fail: no image at all
        if not image:
            log_error(
                f"Service '{svc_name}' has no `image:` directive. "
                f"Railway requires a pre-built image reference (e.g., `image: postgres:16`). "
                f"If using `build:`, add `image:` with the registry tag you've pushed to."
            )
            sys.exit(1)

        # Warning: build present but we ignore it
        if build:
            warnings.append(ImportWarning(
                service=svc_name,
                feature="build",
                message=f"Service '{svc_name}': `build:` ignored. Using `image: {image}` only.",
            ))

        # Warning: depends_on
        if svc_data.get("depends_on"):
            warnings.append(ImportWarning(
                service=svc_name,
                feature="depends_on",
                message="Service dependencies ignored. Railway handles startup automatically.",
            ))

        # Warning: links
        if svc_data.get("links"):
            warnings.append(ImportWarning(
                service=svc_name,
                feature="links",
                message=(
                    "Legacy `links:` ignored. "
                    "Services communicate via `{service}.railway.internal`."
                ),
            ))

        # Warning: privileged / cap_add
        if svc_data.get("privileged"):
            warnings.append(ImportWarning(
                service=svc_name,
                feature="privileged",
                message="Privileged mode not supported on Railway.",
            ))
        if svc_data.get("cap_add"):
            warnings.append(ImportWarning(
                service=svc_name,
                feature="cap_add",
                message="Linux capabilities (cap_add) not supported on Railway.",
            ))

        # Warning: secrets/configs per service
        if svc_data.get("secrets"):
            warnings.append(ImportWarning(
                service=svc_name,
                feature="secrets",
                message="Docker secrets not supported. Use Railway variables instead.",
            ))
        if svc_data.get("configs"):
            warnings.append(ImportWarning(
                service=svc_name,
                feature="configs",
                message="Docker configs not supported. Use Railway variables instead.",
            ))

        # Warning: networks per service
        if svc_data.get("networks"):
            warnings.append(ImportWarning(
                service=svc_name,
                feature="networks",
                message="Service-level network config ignored. Railway provides automatic private networking.",
            ))

    return services, warnings


# ─── Variable Transformer ───────────────────────────────────────────────────

def transform_variables(
    variables: dict[str, str],
    service_names: set[str],
    service_ports: dict[str, list[int]],
) -> dict[str, str]:
    """
    Transform variables for Railway:
    1. Drop RAILWAY_* variables
    2. Rewrite cross-service references to .railway.internal
    """
    transformed = {}

    for key, value in variables.items():
        # Drop Railway-injected variables
        if key.startswith(RAILWAY_INJECTED_PREFIX):
            continue

        # Rewrite cross-service references
        transformed[key] = _rewrite_service_references(
            value, service_names, service_ports
        )

    return transformed


def _rewrite_service_references(
    value: str,
    service_names: set[str],
    service_ports: dict[str, list[int]],
) -> str:
    """
    Scan a variable value for references to other compose service names
    and rewrite them to Railway's private networking format.

    Examples:
      "postgres:5432" → "postgres.railway.internal:5432"
      "http://redis:6379" → "http://redis.railway.internal:6379"
      "api" (in URL-like context) → "api.railway.internal"
    """
    # Already a Railway reference — leave it alone
    if "${{" in value:
        return value

    result = value
    for svc_name in sorted(service_names, key=len, reverse=True):
        # Pattern: service_name:port (most specific)
        pattern = re.compile(
            r"(?<![.\w])" + re.escape(svc_name) + r":(\d+)",
        )
        result = pattern.sub(
            f"{svc_name}.railway.internal:\\1",
            result,
        )

        # Pattern: service_name in URL context (://service_name/ or ://service_name")
        # But NOT if already rewritten (.railway.internal)
        pattern_url = re.compile(
            r"(://)" + re.escape(svc_name) + r"(?=[/\"'\s]|$)(?!\.railway\.internal)",
        )
        result = pattern_url.sub(
            f"\\1{svc_name}.railway.internal",
            result,
        )

    return result


def is_secret_value(key: str, from_secrets_file: bool) -> bool:
    """Check if a variable should be masked in output."""
    return from_secrets_file or bool(SECRET_KEY_PATTERNS.search(key))


def mask_value(value: str) -> str:
    """Mask a secret value for display."""
    if len(value) <= 4:
        return "********"
    return value[:2] + "***" + value[-2:]


# ─── Railway API Client ─────────────────────────────────────────────────────

class RailwayAPI:
    """GraphQL client for Railway API using curl (proven pattern from POC)."""

    def __init__(self, token: str, dry_run: bool = False):
        self.token = token
        self.dry_run = dry_run

    def graphql(self, query: str, variables: dict | None = None) -> dict:
        """Execute a GraphQL operation. Returns the response data."""
        payload = {"query": query, "variables": variables or {}}

        result = subprocess.run(
            [
                "curl", "-s", "-X", "POST",
                API_URL,
                "-H", "Content-Type: application/json",
                "-H", f"Authorization: Bearer {self.token}",
                "-d", json.dumps(payload),
            ],
            capture_output=True,
            text=True,
            timeout=30,
        )

        if result.returncode != 0:
            log_error(f"API request failed: {result.stderr}")
            sys.exit(1)

        try:
            response = json.loads(result.stdout)
        except json.JSONDecodeError:
            log_error(f"Invalid API response: {result.stdout[:200]}")
            sys.exit(1)

        if "errors" in response:
            error_msg = response["errors"][0].get("message", "Unknown error")
            return {"errors": response["errors"], "data": response.get("data")}

        return response

    def verify_token(self) -> bool:
        """Verify the API token is valid."""
        resp = self.graphql("query { projects { edges { node { id } } } }")
        return "errors" not in resp

    def list_services(self, project_id: str) -> dict[str, str]:
        """List services in a project. Returns {name: id}."""
        query = """
        query($projectId: String!) {
            project(id: $projectId) {
                services { edges { node { id name } } }
            }
        }
        """
        resp = self.graphql(query, {"projectId": project_id})
        if "errors" in resp:
            log_error(f"Failed to list services: {resp['errors'][0]['message']}")
            sys.exit(1)

        services = {}
        for edge in resp["data"]["project"]["services"]["edges"]:
            node = edge["node"]
            services[node["name"]] = node["id"]
        return services

    def create_service(
        self, project_id: str, name: str, image: str,
    ) -> str:
        """Create a service. Returns the service ID."""
        query = """
        mutation($projectId: String!, $name: String!, $source: ServiceSourceInput!) {
            serviceCreate(input: {
                projectId: $projectId, name: $name, source: $source
            }) { id name }
        }
        """
        variables = {
            "projectId": project_id,
            "name": name,
            "source": {"image": image},
        }
        resp = self.graphql(query, variables)
        if "errors" in resp:
            log_error(f"Failed to create service '{name}': {resp['errors'][0]['message']}")
            sys.exit(1)

        service_id = resp["data"]["serviceCreate"]["id"]
        return service_id

    def update_service_instance(
        self,
        service_id: str,
        environment_id: str,
        input_data: dict,
    ) -> bool:
        """Update service deploy configuration."""
        if not input_data:
            return True

        query = """
        mutation($serviceId: String!, $environmentId: String!, $input: ServiceInstanceUpdateInput!) {
            serviceInstanceUpdate(
                serviceId: $serviceId, environmentId: $environmentId, input: $input
            )
        }
        """
        variables = {
            "serviceId": service_id,
            "environmentId": environment_id,
            "input": input_data,
        }
        resp = self.graphql(query, variables)
        if "errors" in resp:
            log_warn(f"Deploy config update issue: {resp['errors'][0]['message']}")
            return False
        return True

    def upsert_variables(
        self,
        project_id: str,
        service_id: str,
        environment_id: str,
        variables: dict[str, str],
    ) -> bool:
        """Upsert environment variables for a service."""
        if not variables:
            return True

        query = """
        mutation($projectId: String!, $serviceId: String!, $environmentId: String!, $variables: EnvironmentVariables!) {
            variableCollectionUpsert(input: {
                projectId: $projectId, serviceId: $serviceId,
                environmentId: $environmentId, variables: $variables
            })
        }
        """
        vars_payload = {
            "projectId": project_id,
            "serviceId": service_id,
            "environmentId": environment_id,
            "variables": variables,
        }
        resp = self.graphql(query, vars_payload)
        if "errors" in resp:
            log_warn(f"Variable upsert issue: {resp['errors'][0]['message']}")
            return False
        return True

    def list_volumes(self, project_id: str) -> list[dict]:
        """List volumes and their mount points in a project."""
        query = """
        query($projectId: String!) {
            project(id: $projectId) {
                volumes {
                    edges {
                        node {
                            id name
                            volumeInstances {
                                edges { node { serviceId mountPath } }
                            }
                        }
                    }
                }
            }
        }
        """
        resp = self.graphql(query, {"projectId": project_id})
        if "errors" in resp:
            return []

        volumes = []
        for edge in resp["data"]["project"]["volumes"]["edges"]:
            node = edge["node"]
            for inst_edge in node["volumeInstances"]["edges"]:
                inst = inst_edge["node"]
                volumes.append({
                    "id": node["id"],
                    "name": node["name"],
                    "service_id": inst["serviceId"],
                    "mount_path": inst["mountPath"],
                })
        return volumes

    def create_volume(
        self,
        project_id: str,
        environment_id: str,
        service_id: str,
        mount_path: str,
    ) -> Optional[str]:
        """Create and attach a volume. Returns volume ID."""
        query = """
        mutation($projectId: String!, $environmentId: String!, $serviceId: String!, $mountPath: String!) {
            volumeCreate(input: {
                projectId: $projectId, environmentId: $environmentId,
                serviceId: $serviceId, mountPath: $mountPath
            }) { id name }
        }
        """
        variables = {
            "projectId": project_id,
            "environmentId": environment_id,
            "serviceId": service_id,
            "mountPath": mount_path,
        }
        resp = self.graphql(query, variables)
        if "errors" in resp:
            log_warn(f"Volume creation issue: {resp['errors'][0]['message']}")
            return None
        return resp["data"]["volumeCreate"]["id"]


# ─── Plan Builder ────────────────────────────────────────────────────────────

def build_import_plan(
    compose_data: dict,
    compose_dir: Path,
) -> ImportPlan:
    """
    Build a complete import plan from parsed compose data.

    Resolves all variables, validates, and transforms.
    """
    all_warnings: list[ImportWarning] = []
    all_secrets_files: list[str] = []

    # Validate
    services_dict, validation_warnings = validate_compose(compose_data)
    all_warnings.extend(validation_warnings)

    # Collect service names and ports for cross-reference rewriting
    service_names = set(services_dict.keys())
    service_ports: dict[str, list[int]] = {}
    for svc_name, svc_data in services_dict.items():
        service_ports[svc_name] = extract_internal_ports(svc_data.get("ports"))

    top_level_volumes = compose_data.get("volumes", {}) or {}

    service_configs: list[ServiceConfig] = []

    for svc_name, svc_data in services_dict.items():
        if not isinstance(svc_data, dict):
            continue

        image = svc_data["image"]

        # ── Variables ──
        # 1. Start with env_file vars
        env_vars, secrets_files, env_warnings = resolve_env_files(
            svc_name, svc_data.get("env_file"), compose_dir
        )
        all_warnings.extend(env_warnings)
        all_secrets_files.extend(secrets_files)

        # 2. Merge inline environment (overrides env_file)
        inline_vars = parse_environment_block(svc_data.get("environment"))
        env_vars.update(inline_vars)

        # 3. Substitute ${VAR} references
        resolved_vars: dict[str, str] = {}
        for key, value in env_vars.items():
            resolved, unresolved = substitute_variables(value, env_vars)
            if unresolved:
                log_error(
                    f"Service '{svc_name}': Variable(s) {unresolved} could not be resolved. "
                    f"Set them in your environment or .env file."
                )
                sys.exit(1)
            resolved_vars[key] = resolved

        # 4. Transform: drop RAILWAY_*, rewrite service references
        transformed_vars = transform_variables(
            resolved_vars, service_names, service_ports
        )

        # ── Deploy config ──
        deploy_config, deploy_warnings = parse_deploy_config(svc_name, svc_data)
        all_warnings.extend(deploy_warnings)

        # ── Volumes ──
        vol_configs, vol_warnings = parse_volumes(svc_name, svc_data, top_level_volumes)
        all_warnings.extend(vol_warnings)

        # ── Ports ──
        internal_ports = extract_internal_ports(svc_data.get("ports"))

        service_configs.append(ServiceConfig(
            name=svc_name,
            image=image,
            variables=transformed_vars,
            deploy=deploy_config,
            volumes=vol_configs,
            internal_ports=internal_ports,
        ))

    return ImportPlan(
        services=service_configs,
        warnings=all_warnings,
        secrets_files=all_secrets_files,
    )


# ─── Display ─────────────────────────────────────────────────────────────────

def print_plan_summary(plan: ImportPlan) -> None:
    """Print a human-readable summary of the import plan."""
    print(f"\n{C.BOLD}═══ Import Plan ═══{C.NC}\n")

    # Warnings
    if plan.warnings:
        print(f"{C.YELLOW}Warnings ({len(plan.warnings)}):{C.NC}")
        for w in plan.warnings:
            prefix = f"  [{w.service}]" if w.service else "  [global]"
            print(f"  {C.YELLOW}⚠{C.NC} {prefix} {w.message}")
        print()

    # Secret files
    if plan.secrets_files:
        print(f"{C.DIM}Secret files detected: {', '.join(plan.secrets_files)}")
        print(f"  Values from these files will be masked in output.{C.NC}\n")

    # Services
    print(f"{C.BOLD}Services ({len(plan.services)}):{C.NC}")
    for svc in plan.services:
        print(f"\n  {C.CYAN}● {svc.name}{C.NC}")
        print(f"    Image: {svc.image}")

        if svc.internal_ports:
            print(f"    Ports: {', '.join(str(p) for p in svc.internal_ports)}")

        if svc.deploy.start_command:
            cmd_display = svc.deploy.start_command
            if len(cmd_display) > 60:
                cmd_display = cmd_display[:57] + "..."
            print(f"    Command: {cmd_display}")

        if svc.deploy.restart_policy_type:
            restart_display = svc.deploy.restart_policy_type
            if svc.deploy.restart_policy_max_retries:
                restart_display += f" (max {svc.deploy.restart_policy_max_retries} retries)"
            print(f"    Restart: {restart_display}")

        if svc.deploy.num_replicas:
            print(f"    Replicas: {svc.deploy.num_replicas}")

        if svc.deploy.healthcheck_path:
            print(f"    Healthcheck: {svc.deploy.healthcheck_path}")

        if svc.volumes:
            for vol in svc.volumes:
                bind_tag = " (bind)" if vol.is_bind_mount else ""
                print(f"    Volume: {vol.mount_path}{bind_tag}")

        if svc.variables:
            secret_keys = {
                k for k in svc.variables
                if is_secret_value(k, any(
                    sf in plan.secrets_files for sf in plan.secrets_files
                ))
            }
            var_count = len(svc.variables)
            secret_count = len(secret_keys)
            plain_count = var_count - secret_count
            print(f"    Variables: {var_count} total ({plain_count} plain, {secret_count} masked)")

            for key, value in sorted(svc.variables.items()):
                if is_secret_value(key, key in secret_keys):
                    display_val = mask_value(value)
                else:
                    display_val = value
                    if len(display_val) > 60:
                        display_val = display_val[:57] + "..."
                print(f"      {C.DIM}{key}={display_val}{C.NC}")

    print()

    # Summary line
    total_vars = sum(len(s.variables) for s in plan.services)
    total_vols = sum(len(s.volumes) for s in plan.services)
    parts = [f"{len(plan.services)} services", f"{total_vars} variables"]
    if total_vols:
        parts.append(f"{total_vols} volumes")
    print(f"{C.BOLD}Total: {', '.join(parts)}{C.NC}")

    # API note
    print(f"\n{C.DIM}ℹ️  Railway does not support marking variables as secrets via the API.")
    print(f"   All imported variables are visible in the Railway dashboard.")
    print(f"   To seal sensitive values, use the Railway dashboard after import.{C.NC}\n")


# ─── Executor ────────────────────────────────────────────────────────────────

def execute_plan(
    plan: ImportPlan,
    api: RailwayAPI,
    project_id: str,
    environment_id: str,
    dry_run: bool,
) -> None:
    """Execute the import plan against Railway."""

    total_steps = len(plan.services) * 4  # create, config, vars, volumes per service
    current_step = 0

    # Get existing services for idempotency
    if not dry_run:
        log_step(0, "Verifying API token...")
        if not api.verify_token():
            log_error("Invalid API token. Check your RAILWAY_TOKEN.")
            sys.exit(1)
        log_success("Token verified")

    existing_services = {}
    existing_volumes = []
    if not dry_run:
        log_step(0, "Fetching existing services...")
        existing_services = api.list_services(project_id)
        existing_volumes = api.list_volumes(project_id)
        log_success(f"Found {len(existing_services)} existing services, {len(existing_volumes)} volumes")

    for i, svc in enumerate(plan.services, 1):
        print(f"\n{C.BOLD}{'─' * 50}{C.NC}")
        print(f"{C.BOLD}Service {i}/{len(plan.services)}: {svc.name}{C.NC}")
        print(f"{C.BOLD}{'─' * 50}{C.NC}")

        # ── Step 1: Create or find service ──
        log_step(1, f"Service '{svc.name}'...")
        service_id = existing_services.get(svc.name)

        if service_id:
            log_success(f"Service exists ({service_id[:8]}...)")
        elif dry_run:
            log_dry(f"Would create service '{svc.name}' with image '{svc.image}'")
            service_id = "DRY_RUN_ID"
        else:
            service_id = api.create_service(project_id, svc.name, svc.image)
            log_success(f"Service created ({service_id[:8]}...)")

        # ── Step 2: Deploy configuration ──
        deploy_input = svc.deploy.to_api_input()
        if deploy_input:
            log_step(2, "Deploy configuration...")
            if dry_run:
                for k, v in deploy_input.items():
                    display_v = str(v)
                    if len(display_v) > 60:
                        display_v = display_v[:57] + "..."
                    log_dry(f"Would set {k}: {display_v}")
            else:
                if api.update_service_instance(service_id, environment_id, deploy_input):
                    log_success("Deploy config applied")
                else:
                    log_warn("Deploy config may have partially failed")
        else:
            log_step(2, "No deploy config to apply")

        # ── Step 3: Variables ──
        if svc.variables:
            log_step(3, f"Variables ({len(svc.variables)})...")
            if dry_run:
                log_dry(f"Would upsert {len(svc.variables)} variables")
            else:
                if api.upsert_variables(project_id, service_id, environment_id, svc.variables):
                    log_success(f"{len(svc.variables)} variables configured")
                else:
                    log_warn("Variable upsert may have partially failed")
        else:
            log_step(3, "No variables to set")

        # ── Step 4: Volumes ──
        if svc.volumes:
            log_step(4, f"Volumes ({len(svc.volumes)})...")
            for vol in svc.volumes:
                # Check if volume already exists
                vol_exists = any(
                    v["service_id"] == service_id and v["mount_path"] == vol.mount_path
                    for v in existing_volumes
                )

                if vol_exists:
                    log_success(f"Volume already exists at: {vol.mount_path}")
                elif dry_run:
                    log_dry(f"Would create volume at: {vol.mount_path}")
                else:
                    vol_id = api.create_volume(
                        project_id, environment_id, service_id, vol.mount_path
                    )
                    if vol_id:
                        log_success(f"Volume created at: {vol.mount_path}")
                    else:
                        log_warn(f"Volume creation may have failed for: {vol.mount_path}")
        else:
            log_step(4, "No volumes to create")

    # Final summary
    print(f"\n{C.BOLD}{'═' * 50}{C.NC}")
    if dry_run:
        print(f"{C.YELLOW}DRY RUN complete — no changes were made{C.NC}")
    else:
        print(f"{C.GREEN}Import complete!{C.NC}")
        print(f"{C.DIM}View in dashboard: https://railway.com/project/{project_id}{C.NC}")
    print()


# ─── Main ────────────────────────────────────────────────────────────────────

def main() -> None:
    parser = argparse.ArgumentParser(
        description="Import a Docker Compose file into a Railway project.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Dry run to preview what would be imported
  python compose_importer.py docker-compose.yml \\
    --project-id abc123 --environment-id def456 --dry-run

  # Import for real
  python compose_importer.py docker-compose.yml \\
    --project-id abc123 --environment-id def456

  # Use token from command line
  python compose_importer.py docker-compose.yml \\
    --project-id abc123 --environment-id def456 --token my-token
        """,
    )

    parser.add_argument(
        "compose_file",
        type=Path,
        help="Path to docker-compose.yml file",
    )
    parser.add_argument(
        "--project-id",
        required=True,
        help="Railway project ID",
    )
    parser.add_argument(
        "--environment-id",
        required=True,
        help="Railway environment ID",
    )
    parser.add_argument(
        "--token",
        default=os.environ.get("RAILWAY_TOKEN"),
        help="Railway API token (default: RAILWAY_TOKEN env var)",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Preview import plan without making changes",
    )

    args = parser.parse_args()

    if not args.token:
        log_error(
            "No API token provided. Use --token or set RAILWAY_TOKEN environment variable.\n"
            "Get a token from: https://railway.com/account/tokens"
        )
        sys.exit(1)

    # Parse compose file
    compose_path = args.compose_file.resolve()
    compose_dir = compose_path.parent

    log_info(f"Parsing {compose_path.name}...")
    compose_data = parse_compose_file(compose_path)

    # Build import plan
    log_info("Building import plan...")
    plan = build_import_plan(compose_data, compose_dir)

    # Show the plan
    print_plan_summary(plan)

    if args.dry_run:
        print(f"{C.YELLOW}Dry run mode — showing plan only.{C.NC}")
        print(f"{C.DIM}Remove --dry-run to execute the import.{C.NC}\n")
        return

    # Confirm
    print(f"{C.BOLD}About to push {len(plan.services)} services to Railway.{C.NC}")
    print(f"Project: {args.project_id}")
    print(f"Environment: {args.environment_id}")
    response = input(f"\n{C.BOLD}Proceed? [y/N]{C.NC} ").strip().lower()
    if response not in ("y", "yes"):
        print("Aborted.")
        sys.exit(0)

    # Execute
    api = RailwayAPI(args.token)
    execute_plan(plan, api, args.project_id, args.environment_id, dry_run=False)


if __name__ == "__main__":
    main()
