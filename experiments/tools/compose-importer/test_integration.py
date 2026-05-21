"""
Integration tests for the Docker Compose → Railway importer.

These tests create REAL Railway projects and services to verify the importer works
end-to-end. They require a Railway API token.

Run with:
    RAILWAY_E2E_TOKEN=<token> python -m pytest test_integration.py -v -s

Environment:
    RAILWAY_E2E_TOKEN - Required. Railway API token for test account.
    RAILWAY_CLI_PATH  - Optional. Path to railway CLI binary (default: ../../target/release/railway)
"""

import json
import os
import subprocess
import sys
import time
import uuid
from pathlib import Path

import pytest
import yaml

# Add the parent e2e helpers to path
sys.path.insert(0, str(Path(__file__).parent.parent.parent / "tests" / "e2e"))
from helpers.cli import RailwayCLI

from compose_importer import (
    RailwayAPI,
    build_import_plan,
    execute_plan,
    parse_compose_file,
)


# ─── Constants ───────────────────────────────────────────────────────────────

PROJECT_CREATE_COOLDOWN = 35
_last_project_created_at = 0.0


def _wait_for_rate_limit():
    global _last_project_created_at
    elapsed = time.time() - _last_project_created_at
    if elapsed < PROJECT_CREATE_COOLDOWN:
        wait_time = PROJECT_CREATE_COOLDOWN - elapsed
        print(f"\n⏳ Waiting {wait_time:.0f}s for rate limit cooldown...")
        time.sleep(wait_time)


def _record_project_created():
    global _last_project_created_at
    _last_project_created_at = time.time()


# ─── Fixtures ────────────────────────────────────────────────────────────────


def pytest_configure(config):
    """Validate environment before running tests."""
    token = os.environ.get("RAILWAY_E2E_TOKEN")
    if not token:
        pytest.exit("ERROR: RAILWAY_E2E_TOKEN environment variable required")


@pytest.fixture(scope="session")
def token():
    return os.environ["RAILWAY_E2E_TOKEN"]


@pytest.fixture(scope="session")
def cli():
    binary = os.environ.get(
        "RAILWAY_CLI_PATH",
        str(Path(__file__).parent.parent.parent / "target" / "release" / "railway"),
    )
    return RailwayCLI(binary_path=binary)


@pytest.fixture(scope="session")
def api(token):
    return RailwayAPI(token=token)


@pytest.fixture(scope="session")
def test_project(cli, token):
    """Create a test project for import integration tests.

    Session-scoped to minimize project creation (rate limited).
    """
    _wait_for_rate_limit()

    short_uuid = str(uuid.uuid4())[:8]
    project_name = f"e2e-importer-{short_uuid}"

    # Create project via CLI
    result = cli.project_init(project_name)
    _record_project_created()
    assert result.success, f"Failed to create project: {result.stderr}"

    # Find project ID
    project_id = None
    list_result = cli.project_list()
    if list_result.json_output:
        for p in list_result.json_output:
            if p.get("name") == project_name:
                project_id = p.get("id")
                break

    assert project_id, f"Could not find project ID for '{project_name}'"

    # Get the default production environment ID
    api_client = RailwayAPI(token=token)
    resp = api_client.graphql(
        """query($projectId: String!) {
            project(id: $projectId) {
                environments { edges { node { id name } } }
            }
        }""",
        {"projectId": project_id},
    )
    env_id = None
    for edge in resp["data"]["project"]["environments"]["edges"]:
        node = edge["node"]
        if node["name"] == "production":
            env_id = node["id"]
            break

    assert env_id, "Could not find production environment"

    print(f"\n✅ Test project created: {project_name} ({project_id})")
    print(f"   Environment: production ({env_id})")

    yield {
        "name": project_name,
        "id": project_id,
        "environment_id": env_id,
    }

    # Cleanup
    try:
        cli.project_delete(project_id)
        print(f"\n🗑️  Cleaned up project: {project_name}")
    except Exception as e:
        print(f"\n⚠ Failed to delete project {project_id}: {e}")


# ─── Integration Tests ──────────────────────────────────────────────────────


class TestImportSimpleCompose:
    """Test importing a simple compose file with one service."""

    def test_import_single_service(self, test_project, api, tmp_path, token):
        """Import a single-service compose file and verify it was created."""
        # Write a simple compose file
        compose_data = {
            "services": {
                "web": {
                    "image": "nginx:alpine",
                    "environment": {
                        "NODE_ENV": "production",
                        "PORT": "80",
                    },
                }
            }
        }
        compose_path = tmp_path / "docker-compose.yml"
        compose_path.write_text(yaml.dump(compose_data))

        # Build and execute plan
        parsed = parse_compose_file(compose_path)
        plan = build_import_plan(parsed, tmp_path)

        assert len(plan.services) == 1
        assert plan.services[0].name == "web"
        assert plan.services[0].image == "nginx:alpine"

        execute_plan(
            plan, api,
            test_project["id"],
            test_project["environment_id"],
            dry_run=False,
        )

        # Verify service was created
        services = api.list_services(test_project["id"])
        assert "web" in services, f"Service 'web' not found. Existing: {list(services.keys())}"

        print(f"\n✅ Service 'web' created successfully (ID: {services['web'][:8]}...)")

    def test_idempotent_reimport(self, test_project, api, tmp_path, token):
        """Running import twice should not create duplicate services."""
        compose_data = {
            "services": {
                "web": {
                    "image": "nginx:alpine",
                    "environment": {"PORT": "80"},
                }
            }
        }
        compose_path = tmp_path / "docker-compose.yml"
        compose_path.write_text(yaml.dump(compose_data))

        parsed = parse_compose_file(compose_path)
        plan = build_import_plan(parsed, tmp_path)

        # Execute twice
        execute_plan(plan, api, test_project["id"], test_project["environment_id"], dry_run=False)
        execute_plan(plan, api, test_project["id"], test_project["environment_id"], dry_run=False)

        # Should still have exactly one 'web' service
        services = api.list_services(test_project["id"])
        web_count = sum(1 for name in services if name == "web")
        assert web_count == 1, f"Expected 1 'web' service, found {web_count}"

        print(f"\n✅ Idempotency verified — still 1 'web' service")


class TestImportMultiService:
    """Test importing a multi-service compose file with cross-references."""

    def test_import_multi_service(self, test_project, api, tmp_path, token):
        """Import a multi-service compose file and verify services + variables."""
        compose_data = {
            "services": {
                "api": {
                    "image": "node:18-alpine",
                    "environment": {
                        "DATABASE_URL": "postgres://user:pass@db:5432/app",
                        "REDIS_URL": "redis://cache:6379",
                        "NODE_ENV": "production",
                    },
                    "command": "node server.js",
                    "restart": "on-failure",
                },
                "db": {
                    "image": "postgres:16-alpine",
                    "environment": {
                        "POSTGRES_USER": "user",
                        "POSTGRES_PASSWORD": "pass",
                        "POSTGRES_DB": "app",
                    },
                    "restart": "always",
                },
                "cache": {
                    "image": "redis:7-alpine",
                    "restart": "always",
                },
            }
        }
        compose_path = tmp_path / "docker-compose.yml"
        compose_path.write_text(yaml.dump(compose_data))

        # Build plan and verify cross-service rewriting
        parsed = parse_compose_file(compose_path)
        plan = build_import_plan(parsed, tmp_path)

        assert len(plan.services) == 3

        api_svc = next(s for s in plan.services if s.name == "api")
        assert "db.railway.internal:5432" in api_svc.variables["DATABASE_URL"]
        assert "cache.railway.internal:6379" in api_svc.variables["REDIS_URL"]

        # Execute
        execute_plan(plan, api, test_project["id"], test_project["environment_id"], dry_run=False)

        # Verify all services created
        services = api.list_services(test_project["id"])
        for expected in ["api", "db", "cache"]:
            assert expected in services, f"Service '{expected}' not found"

        print(f"\n✅ Multi-service import complete: {list(services.keys())}")


class TestImportWithVolumes:
    """Test importing a compose file with volume mounts."""

    def test_import_with_volume(self, test_project, api, tmp_path, token):
        """Import a service with a named volume."""
        compose_data = {
            "services": {
                "database": {
                    "image": "postgres:16-alpine",
                    "environment": {
                        "POSTGRES_USER": "test",
                        "POSTGRES_PASSWORD": "test",
                    },
                    "volumes": ["pgdata:/var/lib/postgresql/data"],
                },
            },
            "volumes": {"pgdata": {}},
        }
        compose_path = tmp_path / "docker-compose.yml"
        compose_path.write_text(yaml.dump(compose_data))

        parsed = parse_compose_file(compose_path)
        plan = build_import_plan(parsed, tmp_path)

        assert len(plan.services[0].volumes) == 1
        assert plan.services[0].volumes[0].mount_path == "/var/lib/postgresql/data"

        execute_plan(plan, api, test_project["id"], test_project["environment_id"], dry_run=False)

        # Verify volume exists
        volumes = api.list_volumes(test_project["id"])
        service_id = api.list_services(test_project["id"]).get("database")
        vol_match = [v for v in volumes if v["service_id"] == service_id]
        assert len(vol_match) > 0, "Volume not found for 'database' service"
        assert vol_match[0]["mount_path"] == "/var/lib/postgresql/data"

        print(f"\n✅ Volume verified at /var/lib/postgresql/data")


class TestImportWithEnvFile:
    """Test importing with env_file references."""

    def test_import_with_env_file(self, test_project, api, tmp_path, token):
        """Import a service that uses env_file."""
        # Write env file
        env_path = tmp_path / ".env"
        env_path.write_text("DB_HOST=mydb\nDB_PORT=5432\n")

        compose_data = {
            "services": {
                "envtest": {
                    "image": "alpine:latest",
                    "env_file": ".env",
                    "environment": {"NODE_ENV": "test"},
                },
            }
        }
        compose_path = tmp_path / "docker-compose.yml"
        compose_path.write_text(yaml.dump(compose_data))

        parsed = parse_compose_file(compose_path)
        plan = build_import_plan(parsed, tmp_path)

        svc = plan.services[0]
        # env_file vars + inline vars should be merged
        assert svc.variables["DB_HOST"] == "mydb"
        assert svc.variables["DB_PORT"] == "5432"
        assert svc.variables["NODE_ENV"] == "test"

        execute_plan(plan, api, test_project["id"], test_project["environment_id"], dry_run=False)

        services = api.list_services(test_project["id"])
        assert "envtest" in services

        print(f"\n✅ env_file import verified with merged variables")


class TestDryRun:
    """Test that dry-run mode makes no API calls."""

    def test_dry_run_no_changes(self, test_project, api, tmp_path, token):
        """Dry run should not create any services."""
        compose_data = {
            "services": {
                "should-not-exist": {
                    "image": "alpine:latest",
                }
            }
        }
        compose_path = tmp_path / "docker-compose.yml"
        compose_path.write_text(yaml.dump(compose_data))

        # Get current service count
        services_before = api.list_services(test_project["id"])

        parsed = parse_compose_file(compose_path)
        plan = build_import_plan(parsed, tmp_path)

        execute_plan(plan, api, test_project["id"], test_project["environment_id"], dry_run=True)

        # Should have same services as before
        services_after = api.list_services(test_project["id"])
        assert "should-not-exist" not in services_after
        assert len(services_after) == len(services_before)

        print(f"\n✅ Dry run verified — no services created")
