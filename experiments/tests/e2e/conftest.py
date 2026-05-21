"""Pytest fixtures for Railway CLI E2E tests."""
import os
import time
import uuid
import pytest
from pathlib import Path

from helpers.cli import RailwayCLI


# Rate limit: Railway allows 1 project creation per 30s
PROJECT_CREATE_COOLDOWN = 35

# Track last project creation time globally
_last_project_created_at = 0.0


def _wait_for_rate_limit():
    """Wait if needed to respect Railway's project creation rate limit."""
    global _last_project_created_at
    elapsed = time.time() - _last_project_created_at
    if elapsed < PROJECT_CREATE_COOLDOWN:
        wait_time = PROJECT_CREATE_COOLDOWN - elapsed
        print(f"\n⏳ Waiting {wait_time:.0f}s for rate limit cooldown...")
        time.sleep(wait_time)


def _record_project_created():
    """Record that a project was just created."""
    global _last_project_created_at
    _last_project_created_at = time.time()


def pytest_configure(config):
    """Validate E2E environment before running tests."""
    token = os.environ.get("RAILWAY_E2E_TOKEN")
    if not token:
        pytest.exit("ERROR: RAILWAY_E2E_TOKEN environment variable required")


@pytest.fixture(scope="session")
def cli() -> RailwayCLI:
    """Shared CLI instance for all tests."""
    binary = os.environ.get(
        "RAILWAY_CLI_PATH",
        str(Path(__file__).parent.parent.parent / "target" / "release" / "railway"),
    )
    return RailwayCLI(binary_path=binary)


@pytest.fixture
def unique_name():
    """Factory to generate unique resource names."""
    def _make_name(prefix: str) -> str:
        short_uuid = str(uuid.uuid4())[:8]
        return f"e2e-{prefix}-{short_uuid}"
    return _make_name


@pytest.fixture
def temp_project(cli: RailwayCLI, unique_name, tmp_path):
    """Create a temporary project, cleanup on teardown.

    Yields a dict with:
        - name: project name
        - id: project ID (if found)
        - path: temp directory for linking
    """
    _wait_for_rate_limit()

    project_name = unique_name("proj")
    project_id = None

    result = cli.project_init(project_name)
    _record_project_created()
    assert result.success, f"Failed to create project: {result.stderr}"

    # Find project ID from list
    list_result = cli.project_list()
    if list_result.json_output:
        for p in list_result.json_output:
            if p.get("name") == project_name:
                project_id = p.get("id")
                break

    yield {
        "name": project_name,
        "id": project_id,
        "path": str(tmp_path),
    }

    # Cleanup via GraphQL API
    if project_id:
        try:
            cli.project_delete(project_id)
        except Exception as e:
            print(f"⚠ Failed to delete project {project_id}: {e}")


@pytest.fixture(scope="session")
def shared_project(cli: RailwayCLI, tmp_path_factory):
    """Session-scoped project shared across all environment tests.

    This avoids hitting the rate limit by creating only one project
    for all environment CRUD tests.
    """
    _wait_for_rate_limit()

    short_uuid = str(uuid.uuid4())[:8]
    project_name = f"e2e-shared-{short_uuid}"
    project_id = None
    work_dir = str(tmp_path_factory.mktemp("shared"))

    result = cli.project_init(project_name)
    _record_project_created()
    assert result.success, f"Failed to create shared project: {result.stderr}"

    # Find project ID
    list_result = cli.project_list()
    if list_result.json_output:
        for p in list_result.json_output:
            if p.get("name") == project_name:
                project_id = p.get("id")
                break

    # Link project to work directory
    link_result = cli.project_link(project_id or project_name, cwd=work_dir)
    assert link_result.success, f"Failed to link shared project: {link_result.stderr}"

    yield {
        "name": project_name,
        "id": project_id,
        "path": work_dir,
    }

    # Cleanup
    if project_id:
        try:
            cli.project_delete(project_id)
        except Exception as e:
            print(f"⚠ Failed to delete shared project {project_id}: {e}")


@pytest.fixture
def linked_project(cli: RailwayCLI, temp_project):
    """Create a project and link it to a temp directory."""
    result = cli.project_link(
        temp_project["id"] or temp_project["name"],
        cwd=temp_project["path"],
    )
    assert result.success, f"Failed to link project: {result.stderr}"

    yield temp_project
