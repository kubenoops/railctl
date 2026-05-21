#!/usr/bin/env python3
"""
railctl — Stateless Railway Management CLI

A kubectl-inspired CLI that manages Railway resources via GraphQL API.
No linking, no config files. Token = workspace context, names everywhere.

Usage:
    railctl get projects
    railctl get projects -o wide
    railctl get projects -o json
    railctl get projects -o yaml

Environment:
    RAILWAY_TOKEN         API token (workspace context)
    RAILCTL_PROJECT       Default project name
    RAILCTL_ENVIRONMENT   Default environment name
"""

import argparse
import json
import os
import subprocess
import sys
from datetime import datetime, timezone
from typing import Any, Optional

try:
    import yaml

    HAS_YAML = True
except ImportError:
    HAS_YAML = False


# ─── Constants ───────────────────────────────────────────────────────────────

VERSION = "0.1.0"
API_URL = "https://backboard.railway.com/graphql/v2"


# ─── Colors ──────────────────────────────────────────────────────────────────


class C:
    """ANSI color helpers. Disabled when stdout is not a TTY."""

    _enabled = sys.stdout.isatty()

    RED = "\033[0;31m" if _enabled else ""
    GREEN = "\033[0;32m" if _enabled else ""
    YELLOW = "\033[1;33m" if _enabled else ""
    BLUE = "\033[0;34m" if _enabled else ""
    CYAN = "\033[0;36m" if _enabled else ""
    DIM = "\033[2m" if _enabled else ""
    BOLD = "\033[1m" if _enabled else ""
    NC = "\033[0m" if _enabled else ""


def _err(msg: str) -> None:
    print(f"{C.RED}Error:{C.NC} {msg}", file=sys.stderr)


def _warn(msg: str) -> None:
    print(f"{C.YELLOW}Warning:{C.NC} {msg}", file=sys.stderr)


# ─── Output Formatters ──────────────────────────────────────────────────────


def format_table(rows: list[dict], columns: list[tuple[str, str]]) -> str:
    """
    Format rows as an aligned table.

    columns: list of (key, header_label) tuples.
    """
    if not rows:
        return "No resources found."

    # Calculate column widths
    widths = {}
    for key, label in columns:
        widths[key] = len(label)
        for row in rows:
            val = str(row.get(key, ""))
            widths[key] = max(widths[key], len(val))

    # Header
    header = "  ".join(label.upper().ljust(widths[key]) for key, label in columns)
    lines = [header]

    # Rows
    for row in rows:
        line = "  ".join(
            str(row.get(key, "")).ljust(widths[key]) for key, label in columns
        )
        lines.append(line)

    return "\n".join(lines)


def format_output(data: Any, output_format: str, table_columns: list[tuple[str, str]]) -> str:
    """Format data according to the requested output format."""
    if output_format == "json":
        return json.dumps(data, indent=2, default=str)
    elif output_format == "yaml":
        if not HAS_YAML:
            _err("PyYAML is required for YAML output. Install with: pip install pyyaml")
            sys.exit(1)
        return yaml.dump(data, default_flow_style=False, sort_keys=False).rstrip()
    elif output_format == "wide":
        if isinstance(data, list):
            return format_table(data, table_columns)
        return json.dumps(data, indent=2, default=str)
    else:
        # Default: names-only table
        if isinstance(data, list):
            return format_table(data, table_columns)
        return str(data)


def relative_time(iso_str: str) -> str:
    """Convert an ISO timestamp to a human-readable relative time."""
    try:
        dt = datetime.fromisoformat(iso_str.replace("Z", "+00:00"))
        now = datetime.now(timezone.utc)
        diff = now - dt
        seconds = int(diff.total_seconds())
        if seconds < 60:
            return f"{seconds}s ago"
        minutes = seconds // 60
        if minutes < 60:
            return f"{minutes}m ago"
        hours = minutes // 60
        if hours < 24:
            return f"{hours}h ago"
        days = hours // 24
        if days < 30:
            return f"{days}d ago"
        return dt.strftime("%Y-%m-%d")
    except (ValueError, TypeError):
        return str(iso_str)


# ─── Railway API Client ─────────────────────────────────────────────────────


class RailwayAPI:
    """GraphQL client for Railway API. Stateless — token is the only context."""

    def __init__(self, token: str):
        self.token = token
        # Caches for name→ID resolution
        self._projects_cache: Optional[list[dict]] = None
        self._workspace_id: Optional[str] = None

    def graphql(self, query: str, variables: dict | None = None) -> dict:
        """Execute a GraphQL operation. Returns the 'data' field."""
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
            _err(f"API request failed: {result.stderr}")
            sys.exit(1)

        try:
            response = json.loads(result.stdout)
        except json.JSONDecodeError:
            _err(f"Invalid API response: {result.stdout[:200]}")
            sys.exit(1)

        if "errors" in response:
            msg = response["errors"][0].get("message", "Unknown error")
            _err(f"API error: {msg}")
            sys.exit(1)

        return response.get("data", {})

    # ── Queries ──────────────────────────────────────────────────────────

    def list_projects(self) -> list[dict]:
        """List all projects in the workspace."""
        workspace_id = self.get_workspace_id()
        data = self.graphql("""
            query($workspaceId: String) {
                projects(workspaceId: $workspaceId) {
                    edges {
                        node {
                            id
                            name
                            updatedAt
                            environments {
                                edges {
                                    node {
                                        id
                                        name
                                    }
                                }
                            }
                            services {
                                edges {
                                    node {
                                        id
                                        name
                                    }
                                }
                            }
                        }
                    }
                }
            }
        """, {"workspaceId": workspace_id})

        projects = []
        for edge in data.get("projects", {}).get("edges", []):
            node = edge["node"]
            envs = [e["node"] for e in node.get("environments", {}).get("edges", [])]
            svcs = [s["node"] for s in node.get("services", {}).get("edges", [])]
            projects.append({
                "name": node["name"],
                "id": node["id"],
                "updatedAt": node.get("updatedAt", ""),
                "updated": relative_time(node.get("updatedAt", "")),
                "environments": envs,
                "environment_count": len(envs),
                "environment_names": ", ".join(e["name"] for e in envs),
                "services": svcs,
                "service_count": len(svcs),
                "service_names": ", ".join(s["name"] for s in svcs),
            })

        self._projects_cache = projects
        return projects

    def get_project(self, project_id: str) -> dict:
        """Get detailed project information by ID."""
        data = self.graphql("""
            query($id: String!) {
                project(id: $id) {
                    id
                    name
                    updatedAt
                    environments {
                        edges {
                            node {
                                id
                                name
                                serviceInstances {
                                    edges {
                                        node {
                                            serviceId
                                            serviceName
                                        }
                                    }
                                }
                            }
                        }
                    }
                    services {
                        edges {
                            node {
                                id
                                name
                            }
                        }
                    }
                }
            }
        """, {"id": project_id})
        return data.get("project", {})

    # ── Mutations ────────────────────────────────────────────────────────

    def get_workspace_id(self) -> str:
        """Get the workspace ID from the current token. Cached after first call."""
        if self._workspace_id:
            return self._workspace_id

        data = self.graphql("""
            query {
                me {
                    workspaces {
                        id
                    }
                }
            }
        """)
        workspaces = data.get("me", {}).get("workspaces", [])
        if not workspaces:
            _err("Could not determine workspace ID from token.")
            sys.exit(1)
        self._workspace_id = workspaces[0]["id"]
        return self._workspace_id

    def create_project(self, name: str) -> dict:
        """Create a new project. Returns {id, name, environments}."""
        workspace_id = self.get_workspace_id()
        data = self.graphql("""
            mutation($name: String, $workspaceId: String) {
                projectCreate(input: { name: $name, workspaceId: $workspaceId }) {
                    id
                    name
                    environments {
                        edges {
                            node {
                                id
                                name
                            }
                        }
                    }
                }
            }
        """, {"name": name, "workspaceId": workspace_id})
        return data.get("projectCreate", {})

    def delete_project(self, project_id: str) -> bool:
        """Delete a project by ID."""
        data = self.graphql("""
            mutation($id: String!) {
                projectDelete(id: $id)
            }
        """, {"id": project_id})
        return data.get("projectDelete", False)

    def create_environment(self, project_id: str, name: str) -> dict:
        """Create a new environment in a project. Returns {id, name}."""
        data = self.graphql("""
            mutation($projectId: String!, $name: String!) {
                environmentCreate(
                    input: { projectId: $projectId, name: $name }
                ) {
                    name
                    id
                }
            }
        """, {"projectId": project_id, "name": name})
        return data.get("environmentCreate", {})

    def delete_environment(self, environment_id: str) -> bool:
        """Delete an environment by ID."""
        data = self.graphql("""
            mutation($id: String!) {
                environmentDelete(id: $id)
            }
        """, {"id": environment_id})
        return data.get("environmentDelete", False)

    def create_service(self, project_id: str, environment_id: str, name: str,
                       image: str | None = None) -> dict:
        """Create a new service in a project+environment. Returns {id, name}."""
        variables: dict = {
            "name": name,
            "projectId": project_id,
            "environmentId": environment_id,
        }
        if image:
            variables["source"] = {"image": image}

        data = self.graphql("""
            mutation($name: String, $projectId: String!, $environmentId: String!, $source: ServiceSourceInput) {
                serviceCreate(
                    input: { name: $name, projectId: $projectId, environmentId: $environmentId, source: $source }
                ) {
                    id
                    name
                }
            }
        """, variables)
        return data.get("serviceCreate", {})

    def delete_service(self, service_id: str, environment_id: str) -> bool:
        """Delete a service by ID (scoped to an environment)."""
        data = self.graphql("""
            mutation($serviceId: String!, $environmentId: String!) {
                serviceDelete(environmentId: $environmentId, id: $serviceId)
            }
        """, {"serviceId": service_id, "environmentId": environment_id})
        return data.get("serviceDelete", False)

    def get_deployments(self, service_id: str, environment_id: str, limit: int = 5) -> list[dict]:
        """Get recent deployments for a service in an environment."""
        data = self.graphql("""
            query($input: DeploymentListInput!, $first: Int) {
                deployments(input: $input, first: $first) {
                    edges {
                        node {
                            id
                            createdAt
                            status
                        }
                    }
                }
            }
        """, {
            "input": {"serviceId": service_id, "environmentId": environment_id},
            "first": limit,
        })
        return [e["node"] for e in data.get("deployments", {}).get("edges", [])]

    # ── Variables ─────────────────────────────────────────────────────────

    def get_variables(self, project_id: str, environment_id: str, service_id: str) -> dict:
        """Get variables for a service deployment. Returns dict of key→value."""
        data = self.graphql("""
            query($projectId: String!, $environmentId: String!, $serviceId: String!) {
                variablesForServiceDeployment(
                    projectId: $projectId
                    environmentId: $environmentId
                    serviceId: $serviceId
                )
            }
        """, {
            "projectId": project_id,
            "environmentId": environment_id,
            "serviceId": service_id,
        })
        return data.get("variablesForServiceDeployment", {})

    def upsert_variables(self, project_id: str, environment_id: str,
                         service_id: str, variables: dict) -> bool:
        """Upsert variables for a service. Returns True on success."""
        data = self.graphql("""
            mutation($projectId: String!, $serviceId: String!, $environmentId: String!, $variables: EnvironmentVariables!) {
                variableCollectionUpsert(
                    input: { projectId: $projectId, environmentId: $environmentId, serviceId: $serviceId, variables: $variables }
                )
            }
        """, {
            "projectId": project_id,
            "environmentId": environment_id,
            "serviceId": service_id,
            "variables": variables,
        })
        return data.get("variableCollectionUpsert", False)

    def delete_variable(self, project_id: str, environment_id: str,
                        service_id: str, name: str) -> bool:
        """Delete a variable by name. Returns True on success."""
        data = self.graphql("""
            mutation($projectId: String!, $environmentId: String!, $name: String!, $serviceId: String) {
                variableDelete(
                    input: { projectId: $projectId, environmentId: $environmentId, name: $name, serviceId: $serviceId }
                )
            }
        """, {
            "projectId": project_id,
            "environmentId": environment_id,
            "serviceId": service_id,
            "name": name,
        })
        return data.get("variableDelete", False)

    # ── Volumes ───────────────────────────────────────────────────────────

    def create_volume(self, project_id: str, environment_id: str,
                      service_id: str, mount_path: str) -> dict:
        """Create a volume attached to a service. Returns {id, name}."""
        data = self.graphql("""
            mutation($projectId: String!, $environmentId: String!, $serviceId: String!, $mountPath: String!) {
                volumeCreate(
                    input: { projectId: $projectId, environmentId: $environmentId, serviceId: $serviceId, mountPath: $mountPath }
                ) {
                    id
                    name
                }
            }
        """, {
            "projectId": project_id,
            "environmentId": environment_id,
            "serviceId": service_id,
            "mountPath": mount_path,
        })
        return data.get("volumeCreate", {})

    def delete_volume(self, volume_id: str) -> bool:
        """Delete a volume by ID."""
        data = self.graphql("""
            mutation($id: String!) {
                volumeDelete(volumeId: $id)
            }
        """, {"id": volume_id})
        return data.get("volumeDelete", False)

    # ── Deployments & Rollout ────────────────────────────────────────────

    def deploy_service(self, service_id: str, environment_id: str) -> bool:
        """Trigger a new deploy for a service instance."""
        data = self.graphql("""
            mutation($environmentId: String!, $serviceId: String!) {
                serviceInstanceDeployV2(environmentId: $environmentId, serviceId: $serviceId)
            }
        """, {"environmentId": environment_id, "serviceId": service_id})
        return data.get("serviceInstanceDeployV2", False)

    def restart_deployment(self, deployment_id: str) -> bool:
        """Restart a deployment by ID."""
        data = self.graphql("""
            mutation($id: String!) {
                deploymentRestart(id: $id)
            }
        """, {"id": deployment_id})
        return data.get("deploymentRestart", False)

    def redeploy_deployment(self, deployment_id: str) -> dict:
        """Redeploy a deployment by ID. Returns {id}."""
        data = self.graphql("""
            mutation($id: String!) {
                deploymentRedeploy(id: $id) {
                    id
                }
            }
        """, {"id": deployment_id})
        return data.get("deploymentRedeploy", {})

    def get_latest_deployment(self, service_id: str, environment_id: str) -> dict | None:
        """Get the latest deployment for a service+environment. Returns {id, status} or None."""
        data = self.graphql("""
            query($serviceId: String!, $environmentId: String!) {
                serviceInstance(environmentId: $environmentId, serviceId: $serviceId) {
                    latestDeployment {
                        id
                        status
                    }
                }
            }
        """, {"serviceId": service_id, "environmentId": environment_id})
        si = data.get("serviceInstance")
        if si:
            return si.get("latestDeployment")
        return None

    def get_deployment_logs(self, deployment_id: str, limit: int = 100) -> list[dict]:
        """Get deployment logs. Returns list of {timestamp, message}."""
        data = self.graphql("""
            query($deploymentId: String!, $limit: Int) {
                deploymentLogs(deploymentId: $deploymentId, limit: $limit) {
                    timestamp
                    message
                }
            }
        """, {"deploymentId": deployment_id, "limit": limit})
        return data.get("deploymentLogs", [])

    def get_build_logs(self, deployment_id: str, limit: int = 100) -> list[dict]:
        """Get build logs. Returns list of {timestamp, message}."""
        data = self.graphql("""
            query($deploymentId: String!, $limit: Int) {
                buildLogs(deploymentId: $deploymentId, limit: $limit) {
                    timestamp
                    message
                }
            }
        """, {"deploymentId": deployment_id, "limit": limit})
        return data.get("buildLogs", [])

    # ── Name Resolution ──────────────────────────────────────────────────

    def resolve_project(self, name: str) -> dict:
        """
        Resolve a project name to its full record.

        Returns the project dict with id, name, environments, services, etc.
        Exits with error if not found or ambiguous.
        """
        projects = self._projects_cache or self.list_projects()

        # Exact match first
        exact = [p for p in projects if p["name"] == name]
        if len(exact) == 1:
            return exact[0]

        # Substring match
        matches = [p for p in projects if name.lower() in p["name"].lower()]
        if len(matches) == 0:
            _err(f"Project '{name}' not found.")
            sys.exit(1)
        elif len(matches) == 1:
            return matches[0]
        else:
            _err(f"Ambiguous project name '{name}'. Matches:")
            for m in matches:
                print(f"  - {m['name']}", file=sys.stderr)
            sys.exit(1)

    def resolve_environment(self, project: dict, name: str) -> dict:
        """
        Resolve an environment name within a project.

        Returns the environment dict with id and name.
        Exits with error if not found or ambiguous.
        """
        envs = project.get("environments", [])

        # Exact match first
        exact = [e for e in envs if e["name"] == name]
        if len(exact) == 1:
            return exact[0]

        # Substring match
        matches = [e for e in envs if name.lower() in e["name"].lower()]
        if len(matches) == 0:
            _err(f"Environment '{name}' not found in project '{project['name']}'.")
            sys.exit(1)
        elif len(matches) == 1:
            return matches[0]
        else:
            _err(f"Ambiguous environment name '{name}'. Matches:")
            for m in matches:
                print(f"  - {m['name']}", file=sys.stderr)
            sys.exit(1)

    def resolve_service(self, project: dict, name: str) -> dict:
        """
        Resolve a service name within a project.

        Returns the service dict with id and name.
        Exits with error if not found or ambiguous.
        """
        svcs = project.get("services", [])

        # Exact match first
        exact = [s for s in svcs if s["name"] == name]
        if len(exact) == 1:
            return exact[0]

        # Substring match
        matches = [s for s in svcs if name.lower() in s["name"].lower()]
        if len(matches) == 0:
            _err(f"Service '{name}' not found in project '{project['name']}'.")
            sys.exit(1)
        elif len(matches) == 1:
            return matches[0]
        else:
            _err(f"Ambiguous service name '{name}'. Matches:")
            for m in matches:
                print(f"  - {m['name']}", file=sys.stderr)
            sys.exit(1)


# ─── Helpers ────────────────────────────────────────────────────────────────


def require_flag(value: Any, flag_name: str, env_var: str) -> str:
    """Require a flag/env var to be set, or exit with error."""
    if not value:
        _err(f"{flag_name} is required. Use {flag_name} flag or set {env_var}.")
        sys.exit(1)
    return value


def confirm_delete(resource_type: str, resource_name: str, yes: bool) -> bool:
    """Prompt for delete confirmation unless --yes is set."""
    if yes:
        return True
    try:
        answer = input(f"Delete {resource_type} '{resource_name}'? [y/N] ").strip().lower()
        return answer in ("y", "yes")
    except (EOFError, KeyboardInterrupt):
        print()
        return False


# ─── Command Handlers ───────────────────────────────────────────────────────


def cmd_get_projects(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl get projects"""
    projects = api.list_projects()

    # Define columns per output format
    if args.output == "wide":
        columns = [
            ("name", "Name"),
            ("environment_count", "Environments"),
            ("service_count", "Services"),
            ("environment_names", "Environment Names"),
            ("service_names", "Service Names"),
            ("updated", "Updated"),
        ]
    else:
        columns = [
            ("name", "Name"),
            ("service_count", "Services"),
            ("updated", "Updated"),
        ]

    if args.output in ("json", "yaml"):
        # Structured output — clean up internal fields
        clean = []
        for p in projects:
            clean.append({
                "name": p["name"],
                "id": p["id"],
                "environments": [{"name": e["name"], "id": e["id"]} for e in p["environments"]],
                "services": [{"name": s["name"], "id": s["id"]} for s in p["services"]],
                "updatedAt": p["updatedAt"],
            })
        print(format_output(clean, args.output, columns))
    else:
        print(format_output(projects, args.output, columns))


def cmd_create_project(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl create project <name>"""
    name = args.name
    if not name:
        _err("Project name is required: railctl create project <name>")
        sys.exit(1)

    result = api.create_project(name)
    project_id = result.get("id", "")
    envs = [e["node"] for e in result.get("environments", {}).get("edges", [])]
    env_names = ", ".join(e["name"] for e in envs)

    print(f"Project '{name}' created.")
    if env_names:
        print(f"Default environments: {env_names}")


def cmd_describe_project(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl describe project <name>"""
    name = args.name or args.project
    if not name:
        _err("Project name is required: railctl describe project <name>")
        sys.exit(1)

    project = api.resolve_project(name)
    detail = api.get_project(project["id"])

    envs = [e["node"] for e in detail.get("environments", {}).get("edges", [])]
    svcs = [s["node"] for s in detail.get("services", {}).get("edges", [])]

    if args.output in ("json", "yaml"):
        clean = {
            "name": detail["name"],
            "id": detail["id"],
            "updatedAt": detail.get("updatedAt", ""),
            "environments": [],
            "services": [{"name": s["name"], "id": s["id"]} for s in svcs],
        }
        for env in envs:
            svc_instances = [si["node"] for si in env.get("serviceInstances", {}).get("edges", [])]
            clean["environments"].append({
                "name": env["name"],
                "id": env["id"],
                "services": [{"name": si.get("serviceName", ""), "serviceId": si.get("serviceId", "")} for si in svc_instances],
            })
        print(format_output(clean, args.output, []))
    else:
        # Human-readable describe
        print(f"Name:         {detail['name']}")
        print(f"ID:           {detail['id']}")
        print(f"Updated:      {relative_time(detail.get('updatedAt', ''))}")
        print(f"Services:     {len(svcs)}")
        if svcs:
            for s in svcs:
                print(f"  - {s['name']}")
        print(f"Environments: {len(envs)}")
        if envs:
            for env in envs:
                svc_instances = [si["node"] for si in env.get("serviceInstances", {}).get("edges", [])]
                svc_info = f" ({len(svc_instances)} services)" if svc_instances else ""
                print(f"  - {env['name']}{svc_info}")


def cmd_delete_project(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl delete project <name>"""
    name = args.name or args.project
    if not name:
        _err("Project name is required: railctl delete project <name>")
        sys.exit(1)

    project = api.resolve_project(name)

    # Guard: fail if environments exist
    envs = project.get("environments", [])
    if envs:
        _err(
            f"Cannot delete project '{name}': {len(envs)} environment(s) exist "
            f"({', '.join(e['name'] for e in envs)}). "
            f"Delete all environments first."
        )
        sys.exit(1)

    if not confirm_delete("project", name, getattr(args, "yes", False)):
        print("Cancelled.")
        return

    api.delete_project(project["id"])
    print(f"Project '{name}' deleted.")


def cmd_get_environments(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl get environments -p <project>"""
    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    project = api.resolve_project(project_name)

    envs = project.get("environments", [])

    # Enrich with service counts from detailed project data
    detail = api.get_project(project["id"])
    detail_envs = [e["node"] for e in detail.get("environments", {}).get("edges", [])]
    env_map = {e["id"]: e for e in detail_envs}

    rows = []
    for env in envs:
        detail_env = env_map.get(env["id"], {})
        svc_instances = [si["node"] for si in detail_env.get("serviceInstances", {}).get("edges", [])]
        rows.append({
            "name": env["name"],
            "id": env["id"],
            "service_count": len(svc_instances),
            "service_names": ", ".join(si.get("serviceName", "") for si in svc_instances),
        })

    if args.output == "wide":
        columns = [
            ("name", "Name"),
            ("service_count", "Services"),
            ("service_names", "Service Names"),
            ("id", "ID"),
        ]
    else:
        columns = [
            ("name", "Name"),
            ("service_count", "Services"),
        ]

    if args.output in ("json", "yaml"):
        clean = []
        for r in rows:
            clean.append({
                "name": r["name"],
                "id": r["id"],
                "serviceCount": r["service_count"],
            })
        print(format_output(clean, args.output, columns))
    else:
        print(format_output(rows, args.output, columns))


def cmd_create_environment(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl create environment <name> -p <project>"""
    env_name = args.name
    if not env_name:
        _err("Environment name is required: railctl create environment <name> -p <project>")
        sys.exit(1)

    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    project = api.resolve_project(project_name)

    result = api.create_environment(project["id"], env_name)
    print(f"Environment '{result.get('name', env_name)}' created in project '{project_name}'.")


def cmd_delete_environment(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl delete environment <name> -p <project>"""
    env_name = args.name or args.environment
    if not env_name:
        _err("Environment name is required: railctl delete environment <name> -p <project>")
        sys.exit(1)

    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    project = api.resolve_project(project_name)

    # Refresh project data with service instances to check for services
    detail = api.get_project(project["id"])
    detail_envs = [e["node"] for e in detail.get("environments", {}).get("edges", [])]

    # Resolve environment from detail (has service instance data)
    env = None
    for de in detail_envs:
        if de["name"] == env_name:
            env = de
            break
    if not env:
        # Try substring match
        matches = [de for de in detail_envs if env_name.lower() in de["name"].lower()]
        if len(matches) == 1:
            env = matches[0]
        elif len(matches) == 0:
            _err(f"Environment '{env_name}' not found in project '{project_name}'.")
            sys.exit(1)
        else:
            _err(f"Ambiguous environment name '{env_name}'. Matches:")
            for m in matches:
                print(f"  - {m['name']}", file=sys.stderr)
            sys.exit(1)

    # Guard: fail if services exist in this environment
    svc_instances = [si["node"] for si in env.get("serviceInstances", {}).get("edges", [])]
    if svc_instances:
        svc_names = ", ".join(si.get("serviceName", "unknown") for si in svc_instances)
        _err(
            f"Cannot delete environment '{env['name']}': {len(svc_instances)} service(s) exist "
            f"({svc_names}). Delete all services first."
        )
        sys.exit(1)

    if not confirm_delete("environment", env["name"], getattr(args, "yes", False)):
        print("Cancelled.")
        return

    api.delete_environment(env["id"])
    print(f"Environment '{env['name']}' deleted from project '{project_name}'.")


def cmd_get_services(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl get services -p <project> -e <env>"""
    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")
    project = api.resolve_project(project_name)
    env = api.resolve_environment(project, env_name)

    # Get service instances from detailed project data
    detail = api.get_project(project["id"])
    detail_envs = [e["node"] for e in detail.get("environments", {}).get("edges", [])]
    target_env = None
    for de in detail_envs:
        if de["id"] == env["id"]:
            target_env = de
            break

    svc_instances = []
    if target_env:
        svc_instances = [si["node"] for si in target_env.get("serviceInstances", {}).get("edges", [])]

    # Build rows from project-level services, enriched with instance data
    svcs = project.get("services", [])
    instance_map = {si.get("serviceId"): si for si in svc_instances}

    rows = []
    for svc in svcs:
        rows.append({
            "name": svc["name"],
            "id": svc["id"],
            "has_instance": "Yes" if svc["id"] in instance_map else "No",
        })

    if args.output == "wide":
        columns = [
            ("name", "Name"),
            ("has_instance", "Deployed"),
            ("id", "ID"),
        ]
    else:
        columns = [
            ("name", "Name"),
            ("has_instance", "Deployed"),
        ]

    if args.output in ("json", "yaml"):
        clean = [{"name": r["name"], "id": r["id"]} for r in rows]
        print(format_output(clean, args.output, columns))
    else:
        print(format_output(rows, args.output, columns))


def cmd_create_service(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl create service <name> -p <project> -e <env> [--image <image>]"""
    svc_name = args.name
    if not svc_name:
        _err("Service name is required: railctl create service <name> -p <project> -e <env>")
        sys.exit(1)

    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")
    project = api.resolve_project(project_name)
    env = api.resolve_environment(project, env_name)

    image = getattr(args, "image", None)
    result = api.create_service(project["id"], env["id"], svc_name, image=image)
    msg = f"Service '{result.get('name', svc_name)}' created in project '{project_name}' environment '{env_name}'."
    if image:
        msg += f" Image: {image}"
    print(msg)


def cmd_describe_service(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl describe service <name> -p <project> -e <env>"""
    svc_name = args.name or args.service
    if not svc_name:
        _err("Service name is required: railctl describe service <name> -p <project> -e <env>")
        sys.exit(1)

    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")
    project = api.resolve_project(project_name)
    env = api.resolve_environment(project, env_name)
    svc = api.resolve_service(project, svc_name)

    # Get deployments
    deployments = api.get_deployments(svc["id"], env["id"], limit=5)

    if args.output in ("json", "yaml"):
        clean = {
            "name": svc["name"],
            "id": svc["id"],
            "project": project_name,
            "environment": env_name,
            "deployments": [
                {
                    "id": d["id"],
                    "status": d.get("status", ""),
                    "createdAt": d.get("createdAt", ""),
                }
                for d in deployments
            ],
        }
        print(format_output(clean, args.output, []))
    else:
        print(f"Name:         {svc['name']}")
        print(f"ID:           {svc['id']}")
        print(f"Project:      {project_name}")
        print(f"Environment:  {env_name}")
        print(f"Deployments:  {len(deployments)}")
        if deployments:
            for d in deployments:
                status = d.get("status", "UNKNOWN")
                created = relative_time(d.get("createdAt", ""))
                print(f"  - {status}  {created}  {d['id'][:12]}")


def cmd_delete_service(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl delete service <name> -p <project> -e <env>"""
    svc_name = args.name or args.service
    if not svc_name:
        _err("Service name is required: railctl delete service <name> -p <project> -e <env>")
        sys.exit(1)

    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")
    project = api.resolve_project(project_name)
    env = api.resolve_environment(project, env_name)
    svc = api.resolve_service(project, svc_name)

    if not confirm_delete("service", svc["name"], getattr(args, "yes", False)):
        print("Cancelled.")
        return

    api.delete_service(svc["id"], env["id"])
    print(f"Service '{svc['name']}' deleted from project '{project_name}' environment '{env_name}'.")


# ─── Variable Commands ──────────────────────────────────────────────────────


def cmd_get_variables(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl get variables -p <project> -e <env> -s <service>"""
    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")
    svc_name = require_flag(args.service, "-s/--service", "RAILCTL_SERVICE")
    project = api.resolve_project(project_name)
    env = api.resolve_environment(project, env_name)
    svc = api.resolve_service(project, svc_name)

    variables = api.get_variables(project["id"], env["id"], svc["id"])

    if args.output in ("json", "yaml"):
        print(format_output(variables, args.output, []))
    else:
        rows = [{"key": k, "value": v} for k, v in sorted(variables.items())]
        columns = [("key", "Key"), ("value", "Value")]
        print(format_output(rows, args.output, columns))


def cmd_set_variable(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl set variable KEY=VALUE -p <project> -e <env> -s <service>"""
    pairs = getattr(args, "pairs", []) or []
    if not pairs:
        _err("At least one KEY=VALUE pair is required: railctl set variable KEY=VALUE")
        sys.exit(1)

    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")
    svc_name = require_flag(args.service, "-s/--service", "RAILCTL_SERVICE")
    project = api.resolve_project(project_name)
    env = api.resolve_environment(project, env_name)
    svc = api.resolve_service(project, svc_name)

    variables = {}
    for pair in pairs:
        if "=" not in pair:
            _err(f"Invalid variable format '{pair}'. Expected KEY=VALUE.")
            sys.exit(1)
        key, value = pair.split("=", 1)
        variables[key] = value

    api.upsert_variables(project["id"], env["id"], svc["id"], variables)
    for key, value in variables.items():
        print(f"Variable '{key}' set on service '{svc_name}'.")


def cmd_delete_variable(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl delete variable KEY -p <project> -e <env> -s <service>"""
    var_name = args.name
    if not var_name:
        _err("Variable name is required: railctl delete variable KEY -p <project> -e <env> -s <service>")
        sys.exit(1)

    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")
    svc_name = require_flag(args.service, "-s/--service", "RAILCTL_SERVICE")
    project = api.resolve_project(project_name)
    env = api.resolve_environment(project, env_name)
    svc = api.resolve_service(project, svc_name)

    api.delete_variable(project["id"], env["id"], svc["id"], var_name)
    print(f"Variable '{var_name}' deleted from service '{svc_name}'.")


# ─── Volume Commands ────────────────────────────────────────────────────────


def cmd_create_volume(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl create volume <mount-path> -p <project> -e <env> -s <service>"""
    mount_path = args.name  # positional arg reused as mount_path
    if not mount_path:
        _err("Mount path is required: railctl create volume /data -p <project> -e <env> -s <service>")
        sys.exit(1)

    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")
    svc_name = require_flag(args.service, "-s/--service", "RAILCTL_SERVICE")
    project = api.resolve_project(project_name)
    env = api.resolve_environment(project, env_name)
    svc = api.resolve_service(project, svc_name)

    result = api.create_volume(project["id"], env["id"], svc["id"], mount_path)
    print(f"Volume '{result.get('name', '')}' created at {mount_path} on service '{svc_name}'.")


def cmd_delete_volume(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl delete volume <name> -p <project> -e <env> -s <service>"""
    vol_name = args.name
    if not vol_name:
        _err("Volume name or ID is required: railctl delete volume <name> --yes")
        sys.exit(1)

    if not confirm_delete("volume", vol_name, getattr(args, "yes", False)):
        print("Cancelled.")
        return

    # vol_name is treated as the volume ID here
    api.delete_volume(vol_name)
    print(f"Volume '{vol_name}' deleted.")


# ─── Rollout Commands ───────────────────────────────────────────────────────


def _resolve_pse(api: RailwayAPI, args: argparse.Namespace):
    """Resolve project, environment, service from args. Returns (project, env, svc)."""
    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")
    svc_name = require_flag(args.service, "-s/--service", "RAILCTL_SERVICE")
    project = api.resolve_project(project_name)
    env = api.resolve_environment(project, env_name)
    svc = api.resolve_service(project, svc_name)
    return project, env, svc


def cmd_deploy(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl deploy -p <project> -e <env> -s <service>"""
    project, env, svc = _resolve_pse(api, args)
    api.deploy_service(svc["id"], env["id"])
    print(f"Deploy triggered for service '{svc['name']}' in environment '{env['name']}'.")


def cmd_restart(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl restart -p <project> -e <env> -s <service>"""
    project, env, svc = _resolve_pse(api, args)
    latest = api.get_latest_deployment(svc["id"], env["id"])
    if not latest:
        _err(f"No deployment found for service '{svc['name']}' in environment '{env['name']}'.")
        sys.exit(1)
    api.restart_deployment(latest["id"])
    print(f"Restart triggered for service '{svc['name']}' (deployment {latest['id'][:12]}).")


def cmd_redeploy(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl redeploy -p <project> -e <env> -s <service>"""
    project, env, svc = _resolve_pse(api, args)
    latest = api.get_latest_deployment(svc["id"], env["id"])
    if not latest:
        _err(f"No deployment found for service '{svc['name']}' in environment '{env['name']}'.")
        sys.exit(1)
    result = api.redeploy_deployment(latest["id"])
    new_id = result.get("id", "unknown")
    print(f"Redeploy triggered for service '{svc['name']}'. New deployment: {new_id[:12]}")


def cmd_logs(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl logs -p <project> -e <env> -s <service> [--lines N] [--build]"""
    project, env, svc = _resolve_pse(api, args)
    latest = api.get_latest_deployment(svc["id"], env["id"])
    if not latest:
        _err(f"No deployment found for service '{svc['name']}' in environment '{env['name']}'.")
        sys.exit(1)

    limit = getattr(args, "lines", 100) or 100
    use_build = getattr(args, "build", False)

    if use_build:
        logs = api.get_build_logs(latest["id"], limit=limit)
    else:
        logs = api.get_deployment_logs(latest["id"], limit=limit)

    if args.output in ("json", "yaml"):
        print(format_output(logs, args.output, []))
    else:
        if not logs:
            print("No logs found.")
        for entry in logs:
            ts = entry.get("timestamp", "")
            msg = entry.get("message", "")
            print(f"{ts}  {msg}")


def cmd_get_deployments(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl get deployments -p <project> -e <env> -s <service>"""
    project, env, svc = _resolve_pse(api, args)
    deployments = api.get_deployments(svc["id"], env["id"], limit=20)

    if args.output in ("json", "yaml"):
        clean = [
            {
                "id": d["id"],
                "status": d.get("status", ""),
                "createdAt": d.get("createdAt", ""),
            }
            for d in deployments
        ]
        print(format_output(clean, args.output, []))
    else:
        rows = []
        for d in deployments:
            rows.append({
                "status": d.get("status", "UNKNOWN"),
                "created": relative_time(d.get("createdAt", "")),
                "id": d["id"][:12],
            })
        columns = [("status", "Status"), ("created", "Created"), ("id", "ID")]
        if args.output == "wide":
            rows_wide = []
            for d in deployments:
                rows_wide.append({
                    "status": d.get("status", "UNKNOWN"),
                    "created": relative_time(d.get("createdAt", "")),
                    "id": d["id"],
                })
            print(format_output(rows_wide, args.output, columns))
        else:
            print(format_output(rows, args.output, columns))


# ─── Declarative Config — Docker Compose Format (Slice 7) ────────────────────
#
# export/diff/apply operate on Docker Compose files (services: / environment:).
# Parsing reuses the compose-importer's parse_environment_block when available.

# Import compose-importer parsing helpers
_COMPOSE_IMPORTER_DIR = os.path.join(os.path.dirname(__file__), "..", "compose-importer")
if _COMPOSE_IMPORTER_DIR not in sys.path:
    sys.path.insert(0, _COMPOSE_IMPORTER_DIR)

try:
    from compose_importer import parse_environment_block as _ci_parse_env_block

    HAS_COMPOSE_IMPORTER = True
except ImportError:
    HAS_COMPOSE_IMPORTER = False


def _parse_env_block(env_spec: Any) -> dict[str, str]:
    """Parse a compose service's environment: block into a flat dict.

    Delegates to compose-importer when available; falls back to local logic.
    Supports both dict form (KEY: VAL) and list form (- KEY=VAL).
    """
    if HAS_COMPOSE_IMPORTER:
        return _ci_parse_env_block(env_spec)
    # ── Fallback ──
    if env_spec is None:
        return {}
    if isinstance(env_spec, dict):
        return {k: str(v) if v is not None else "" for k, v in env_spec.items()}
    if isinstance(env_spec, list):
        out: dict[str, str] = {}
        for item in env_spec:
            s = str(item)
            if "=" in s:
                k, _, v = s.partition("=")
                out[k.strip()] = v.strip()
            else:
                out[s.strip()] = os.environ.get(s.strip(), "")
        return out
    return {}


def _load_compose(file_path: str) -> dict:
    """Load a Docker Compose file (YAML or JSON)."""
    if not os.path.exists(file_path):
        _err(f"Compose file not found: {file_path}")
        sys.exit(1)

    with open(file_path, "r") as f:
        content = f.read()

    # Try JSON first
    try:
        return json.loads(content)
    except json.JSONDecodeError:
        pass

    # Try YAML
    if HAS_YAML:
        try:
            return yaml.safe_load(content)
        except yaml.YAMLError as e:
            _err(f"Invalid YAML in {file_path}: {e}")
            sys.exit(1)

    _err(f"Could not parse {file_path} as JSON or YAML")
    sys.exit(1)


def _export_compose(api: RailwayAPI, project: dict, env_name: str) -> dict:
    """Export a project + environment as a Docker Compose dict.

    Returns::

        services:
          api:
            environment:
              PORT: "8080"
          worker:
            environment:
              REDIS_URL: "redis://..."
    """
    env = api.resolve_environment(project, env_name)
    services: dict[str, dict] = {}
    for svc in project.get("services", []):
        entry: dict[str, Any] = {}
        try:
            variables = api.get_variables(project["id"], env["id"], svc["id"])
        except SystemExit:
            variables = {}
        if variables:
            entry["environment"] = variables
        services[svc["name"]] = entry

    return {"services": services}


def cmd_export(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl export -p <project> -e <env> [-o json|yaml]"""
    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")

    project = api.resolve_project(project_name)
    config = _export_compose(api, project, env_name)

    fmt = getattr(args, "output", None) or "yaml"
    if fmt == "json":
        print(json.dumps(config, indent=2))
    else:
        if not HAS_YAML:
            _err("PyYAML is required for YAML output.  pip install pyyaml")
            sys.exit(1)
        print(yaml.dump(config, default_flow_style=False, sort_keys=False))


def _compose_diff(
    api: RailwayAPI, project: dict, env_name: str, compose: dict,
) -> list[dict]:
    """Diff a Docker Compose file against current Railway state.

    Returns a list of change dicts::

        {"action": "create"|"update"|"delete",
         "resource": "service"|"variable",
         "path": "svc" | "svc/KEY",
         "current": ..., "desired": ...}
    """
    changes: list[dict] = []
    env = api.resolve_environment(project, env_name)
    current_svcs = {s["name"]: s for s in project.get("services", [])}

    for svc_name, svc_spec in (compose.get("services") or {}).items():
        svc_spec = svc_spec or {}

        # ── New service ──
        if svc_name not in current_svcs:
            image = svc_spec.get("image", "")
            changes.append({
                "action": "create", "resource": "service",
                "path": svc_name,
                "current": None, "desired": image or svc_name,
            })
            for key, val in _parse_env_block(svc_spec.get("environment")).items():
                changes.append({
                    "action": "create", "resource": "variable",
                    "path": f"{svc_name}/{key}",
                    "current": None, "desired": val,
                })
            continue

        # ── Existing service — diff variables ──
        cur_svc = current_svcs[svc_name]
        try:
            cur_vars = api.get_variables(project["id"], env["id"], cur_svc["id"])
        except SystemExit:
            cur_vars = {}

        des_vars = _parse_env_block(svc_spec.get("environment"))

        for key, des_val in des_vars.items():
            cur_val = cur_vars.get(key)
            if cur_val is None:
                changes.append({
                    "action": "create", "resource": "variable",
                    "path": f"{svc_name}/{key}",
                    "current": None, "desired": des_val,
                })
            elif str(cur_val) != str(des_val):
                changes.append({
                    "action": "update", "resource": "variable",
                    "path": f"{svc_name}/{key}",
                    "current": cur_val, "desired": des_val,
                })

        for key in cur_vars:
            if key not in des_vars:
                changes.append({
                    "action": "delete", "resource": "variable",
                    "path": f"{svc_name}/{key}",
                    "current": cur_vars[key], "desired": None,
                })

    return changes


def _format_diff(changes: list[dict]) -> str:
    """Format a list of changes as coloured diff output."""
    if not changes:
        return "No changes. Infrastructure is up-to-date."

    lines: list[str] = []
    for ch in changes:
        act, res, path = ch["action"], ch["resource"], ch["path"]
        if act == "create":
            line = f"{C.GREEN}+{C.NC} {res}: {path}"
            if ch["desired"]:
                line += f" = {ch['desired']}"
        elif act == "update":
            line = f"{C.YELLOW}~{C.NC} {res}: {path}"
            line += f" ({ch['current']} → {ch['desired']})"
        elif act == "delete":
            line = f"{C.RED}-{C.NC} {res}: {path}"
            if ch["current"]:
                line += f" = {ch['current']}"
        else:
            line = f"  {res}: {path}"
        lines.append(line)

    creates = sum(1 for c in changes if c["action"] == "create")
    updates = sum(1 for c in changes if c["action"] == "update")
    deletes = sum(1 for c in changes if c["action"] == "delete")
    parts: list[str] = []
    if creates:
        parts.append(f"{C.GREEN}{creates} to add{C.NC}")
    if updates:
        parts.append(f"{C.YELLOW}{updates} to change{C.NC}")
    if deletes:
        parts.append(f"{C.RED}{deletes} to destroy{C.NC}")
    lines.append(f"\n{C.BOLD}Plan:{C.NC} " + ", ".join(parts) + ".")

    return "\n".join(lines)


def cmd_diff(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl diff -p <project> -e <env> -f <compose-file>"""
    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")
    config_file = getattr(args, "file", None)
    if not config_file:
        _err("Compose file required. Use -f <file>")
        sys.exit(1)

    project = api.resolve_project(project_name)
    compose = _load_compose(config_file)
    changes = _compose_diff(api, project, env_name, compose)
    print(_format_diff(changes))


def _apply_compose(
    api: RailwayAPI, project: dict, env_name: str,
    changes: list[dict], compose: dict,
) -> None:
    """Execute the changes produced by _compose_diff."""
    desired_svcs = compose.get("services") or {}

    for ch in changes:
        act, res, path = ch["action"], ch["resource"], ch["path"]

        if res == "service" and act == "create":
            image = (desired_svcs.get(path) or {}).get("image")
            api._projects_cache = None
            project = api.resolve_project(project["name"])
            env = api.resolve_environment(project, env_name)
            api.create_service(project["id"], env["id"], path, image=image)
            img_msg = f" (image: {image})" if image else ""
            print(f"{C.GREEN}Created{C.NC} service: {path}{img_msg}")

        elif res == "variable":
            svc_name, key = path.split("/", 1)
            api._projects_cache = None
            project = api.resolve_project(project["name"])
            env = api.resolve_environment(project, env_name)
            svc = api.resolve_service(project, svc_name)

            if act in ("create", "update"):
                api.upsert_variables(
                    project["id"], env["id"], svc["id"], {key: ch["desired"]},
                )
                verb = "Created" if act == "create" else "Updated"
                color = C.GREEN if act == "create" else C.YELLOW
                print(f"{color}{verb}{C.NC} variable: {path}")
            elif act == "delete":
                api.delete_variable(project["id"], env["id"], svc["id"], key)
                print(f"{C.RED}Deleted{C.NC} variable: {path}")


def cmd_apply(api: RailwayAPI, args: argparse.Namespace) -> None:
    """Handle: railctl apply -p <project> -e <env> -f <compose-file> [--yes]"""
    project_name = require_flag(args.project, "-p/--project", "RAILCTL_PROJECT")
    env_name = require_flag(args.environment, "-e/--environment", "RAILCTL_ENVIRONMENT")
    config_file = getattr(args, "file", None)
    if not config_file:
        _err("Compose file required. Use -f <file>")
        sys.exit(1)

    project = api.resolve_project(project_name)
    compose = _load_compose(config_file)
    changes = _compose_diff(api, project, env_name, compose)

    if not changes:
        print("No changes. Infrastructure is up-to-date.")
        return

    print(_format_diff(changes))
    print()

    if not getattr(args, "yes", False):
        try:
            answer = input("Do you want to apply these changes? [y/N]: ").strip().lower()
            if answer != "y":
                print("Cancelled.")
                return
        except EOFError:
            print("Cancelled.")
            return

    _apply_compose(api, project, env_name, changes, compose)
    print(f"\n{C.GREEN}Apply complete!{C.NC}")


# ─── CLI Parser ─────────────────────────────────────────────────────────────


def build_parser() -> argparse.ArgumentParser:
    """Build the argument parser tree."""
    # Common arguments that should work at any position (kubectl-style)
    # Note: --token is NOT included here because subparser defaults override parent values
    common_args = argparse.ArgumentParser(add_help=False)
    common_args.add_argument(
        "-p", "--project",
        default=os.environ.get("RAILCTL_PROJECT"),
        help="Project name (default: RAILCTL_PROJECT env var)",
    )
    common_args.add_argument(
        "-e", "--environment",
        default=os.environ.get("RAILCTL_ENVIRONMENT"),
        help="Environment name (default: RAILCTL_ENVIRONMENT env var)",
    )
    common_args.add_argument(
        "-s", "--service",
        default=os.environ.get("RAILCTL_SERVICE"),
        help="Service name (default: RAILCTL_SERVICE env var)",
    )
    common_args.add_argument(
        "-o", "--output",
        choices=["json", "yaml", "wide"],
        default=None,
        help="Output format (default: table with names)",
    )

    parser = argparse.ArgumentParser(
        prog="railctl",
        description="Stateless Railway management CLI",
        parents=[common_args],
    )
    parser.add_argument("--version", action="version", version=f"railctl {VERSION}")
    parser.add_argument(
        "--token",
        default=os.environ.get("RAILWAY_TOKEN"),
        help="Railway API token (default: RAILWAY_TOKEN env var)",
    )

    subparsers = parser.add_subparsers(dest="command", help="Command")

    # ── get ───────────────────────────────────────────────────────────
    get_parser = subparsers.add_parser("get", help="List resources", parents=[common_args])
    get_sub = get_parser.add_subparsers(dest="resource", help="Resource type")

    get_sub.add_parser("projects", help="List all projects", parents=[common_args])
    get_sub.add_parser("environments", help="List environments (requires -p)", parents=[common_args])
    get_sub.add_parser("services", help="List services (requires -p -e)", parents=[common_args])
    get_sub.add_parser("variables", help="List variables (requires -p -e -s)", parents=[common_args])
    get_sub.add_parser("deployments", help="List deployments (requires -p -e -s)", parents=[common_args])

    # ── create ────────────────────────────────────────────────────────
    create_parser = subparsers.add_parser("create", help="Create a resource", parents=[common_args])
    create_sub = create_parser.add_subparsers(dest="resource", help="Resource type")

    create_project = create_sub.add_parser("project", help="Create a new project", parents=[common_args])
    create_project.add_argument("name", nargs="?", help="Project name")

    create_env = create_sub.add_parser("environment", help="Create a new environment", parents=[common_args])
    create_env.add_argument("name", nargs="?", help="Environment name")

    create_svc = create_sub.add_parser("service", help="Create a new service", parents=[common_args])
    create_svc.add_argument("name", nargs="?", help="Service name")
    create_svc.add_argument("--image", default=None, help="Docker image to deploy")

    create_vol = create_sub.add_parser("volume", help="Create a volume", parents=[common_args])
    create_vol.add_argument("name", nargs="?", help="Mount path (e.g. /data)")

    # ── describe ──────────────────────────────────────────────────────
    describe_parser = subparsers.add_parser("describe", help="Show detailed info", parents=[common_args])
    describe_sub = describe_parser.add_subparsers(dest="resource", help="Resource type")

    describe_project = describe_sub.add_parser("project", help="Describe a project", parents=[common_args])
    describe_project.add_argument("name", nargs="?", help="Project name")

    describe_svc = describe_sub.add_parser("service", help="Describe a service", parents=[common_args])
    describe_svc.add_argument("name", nargs="?", help="Service name")

    # ── delete ────────────────────────────────────────────────────────
    delete_parser = subparsers.add_parser("delete", help="Delete a resource", parents=[common_args])
    delete_sub = delete_parser.add_subparsers(dest="resource", help="Resource type")

    delete_project = delete_sub.add_parser("project", help="Delete a project", parents=[common_args])
    delete_project.add_argument("name", nargs="?", help="Project name")
    delete_project.add_argument("--yes", action="store_true", help="Skip confirmation")

    delete_env = delete_sub.add_parser("environment", help="Delete an environment", parents=[common_args])
    delete_env.add_argument("name", nargs="?", help="Environment name")
    delete_env.add_argument("--yes", action="store_true", help="Skip confirmation")

    delete_svc = delete_sub.add_parser("service", help="Delete a service", parents=[common_args])
    delete_svc.add_argument("name", nargs="?", help="Service name")
    delete_svc.add_argument("--yes", action="store_true", help="Skip confirmation")

    delete_var = delete_sub.add_parser("variable", help="Delete a variable", parents=[common_args])
    delete_var.add_argument("name", nargs="?", help="Variable name (KEY)")

    delete_vol = delete_sub.add_parser("volume", help="Delete a volume", parents=[common_args])
    delete_vol.add_argument("name", nargs="?", help="Volume name or ID")
    delete_vol.add_argument("--yes", action="store_true", help="Skip confirmation")

    # ── set ───────────────────────────────────────────────────────────
    set_parser = subparsers.add_parser("set", help="Set a resource property", parents=[common_args])
    set_sub = set_parser.add_subparsers(dest="resource", help="Resource type")

    set_var = set_sub.add_parser("variable", help="Set variable(s) KEY=VALUE", parents=[common_args])
    set_var.add_argument("pairs", nargs="*", help="KEY=VALUE pairs")

    # ── deploy / restart / redeploy / logs ────────────────────────────
    subparsers.add_parser("deploy", help="Deploy a service (requires -p -e -s)", parents=[common_args])
    subparsers.add_parser("restart", help="Restart latest deployment (requires -p -e -s)", parents=[common_args])
    subparsers.add_parser("redeploy", help="Redeploy latest deployment (requires -p -e -s)", parents=[common_args])

    logs_parser = subparsers.add_parser("logs", help="View deployment logs (requires -p -e -s)", parents=[common_args])
    logs_parser.add_argument("--lines", "-n", type=int, default=100, help="Number of log lines (default: 100)")
    logs_parser.add_argument("--build", action="store_true", help="Show build logs instead of deploy logs")

    # ── export / diff / apply ─────────────────────────────────────────
    subparsers.add_parser("export", help="Export project config as Docker Compose (requires -p -e)", parents=[common_args])

    diff_parser = subparsers.add_parser("diff", help="Diff compose file vs Railway state (requires -p -e -f)", parents=[common_args])
    diff_parser.add_argument("-f", "--file", required=True, help="Docker Compose file")

    apply_parser = subparsers.add_parser("apply", help="Apply compose file to Railway (requires -p -e -f)", parents=[common_args])
    apply_parser.add_argument("-f", "--file", required=True, help="Docker Compose file")
    apply_parser.add_argument("--yes", action="store_true", help="Skip confirmation")

    return parser


def main(argv: list[str] | None = None) -> int:
    """Main entry point."""
    parser = build_parser()
    args = parser.parse_args(argv)

    if not args.command:
        parser.print_help()
        return 1

    # Validate token
    if not args.token:
        _err(
            "No API token provided. Set RAILWAY_TOKEN or use --token.\n"
            "Get a token at: https://railway.com/account/tokens"
        )
        return 1

    api = RailwayAPI(args.token)

    # Route commands
    if args.command == "get":
        if args.resource == "projects":
            cmd_get_projects(api, args)
            return 0
        elif args.resource == "environments":
            cmd_get_environments(api, args)
            return 0
        elif args.resource == "services":
            cmd_get_services(api, args)
            return 0
        elif args.resource == "variables":
            cmd_get_variables(api, args)
            return 0
        elif args.resource == "deployments":
            cmd_get_deployments(api, args)
            return 0
        else:
            _err(f"Unknown resource: {args.resource}")
            return 1
    elif args.command == "create":
        if args.resource == "project":
            cmd_create_project(api, args)
            return 0
        elif args.resource == "environment":
            cmd_create_environment(api, args)
            return 0
        elif args.resource == "service":
            cmd_create_service(api, args)
            return 0
        elif args.resource == "volume":
            cmd_create_volume(api, args)
            return 0
        else:
            _err(f"Unknown resource: {args.resource}")
            return 1
    elif args.command == "describe":
        if args.resource == "project":
            cmd_describe_project(api, args)
            return 0
        elif args.resource == "service":
            cmd_describe_service(api, args)
            return 0
        else:
            _err(f"Unknown resource: {args.resource}")
            return 1
    elif args.command == "delete":
        if args.resource == "project":
            cmd_delete_project(api, args)
            return 0
        elif args.resource == "environment":
            cmd_delete_environment(api, args)
            return 0
        elif args.resource == "service":
            cmd_delete_service(api, args)
            return 0
        elif args.resource == "variable":
            cmd_delete_variable(api, args)
            return 0
        elif args.resource == "volume":
            cmd_delete_volume(api, args)
            return 0
        else:
            _err(f"Unknown resource: {args.resource}")
            return 1
    elif args.command == "set":
        if args.resource == "variable":
            cmd_set_variable(api, args)
            return 0
        else:
            _err(f"Unknown resource: {args.resource}")
            return 1
    elif args.command == "deploy":
        cmd_deploy(api, args)
        return 0
    elif args.command == "restart":
        cmd_restart(api, args)
        return 0
    elif args.command == "redeploy":
        cmd_redeploy(api, args)
        return 0
    elif args.command == "logs":
        cmd_logs(api, args)
        return 0
    elif args.command == "export":
        cmd_export(api, args)
        return 0
    elif args.command == "diff":
        cmd_diff(api, args)
        return 0
    elif args.command == "apply":
        cmd_apply(api, args)
        return 0
    else:
        _err(f"Unknown command: {args.command}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
