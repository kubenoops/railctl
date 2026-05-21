"""E2E tests for Railway CLI environment operations.

All tests use the session-scoped `shared_project` fixture to avoid
hitting Railway's 30s rate limit on project creation.
"""
import time
import pytest

from helpers.cli import RailwayCLI

# Environment creation also has a 30s rate limit
ENV_CREATE_COOLDOWN = 35

# Track last environment creation time at module level
_last_env_created_at = 0.0


def _wait_for_env_rate_limit():
    """Wait if needed to respect environment creation rate limit."""
    global _last_env_created_at
    elapsed = time.time() - _last_env_created_at
    if elapsed < ENV_CREATE_COOLDOWN:
        wait_time = ENV_CREATE_COOLDOWN - elapsed
        print(f"\n⏳ Waiting {wait_time:.0f}s for env rate limit cooldown...")
        time.sleep(wait_time)


def _record_env_created():
    """Record that an environment was just created."""
    global _last_env_created_at
    _last_env_created_at = time.time()


class TestEnvironmentCreate:
    """Tests for environment creation."""

    def test_create_environment(self, cli: RailwayCLI, shared_project, unique_name):
        """Test creating a new environment."""
        _wait_for_env_rate_limit()
        env_name = unique_name("env")

        result = cli.env_new(env_name, cwd=shared_project["path"])
        _record_env_created()
        assert result.success, f"Failed: {result.stderr}"

        # Cleanup
        cli.env_delete(env_name, cwd=shared_project["path"])

    def test_create_environment_with_duplicate(
        self, cli: RailwayCLI, shared_project, unique_name
    ):
        """Test creating an environment by duplicating an existing one."""
        _wait_for_env_rate_limit()
        env_name = unique_name("staging")

        result = cli.env_new(
            env_name,
            duplicate_from="production",
            cwd=shared_project["path"],
        )
        _record_env_created()
        assert result.success, f"Failed: {result.stderr}"

        # Cleanup
        cli.env_delete(env_name, cwd=shared_project["path"])

    def test_create_multiple_environments(
        self, cli: RailwayCLI, shared_project, unique_name
    ):
        """Test creating multiple environments in sequence."""
        env_names = [unique_name(f"multi-{i}") for i in range(2)]
        created = []

        try:
            for name in env_names:
                _wait_for_env_rate_limit()
                result = cli.env_new(name, cwd=shared_project["path"])
                _record_env_created()
                assert result.success, f"Failed to create {name}: {result.stderr}"
                created.append(name)
        finally:
            for name in created:
                cli.env_delete(name, cwd=shared_project["path"])


class TestEnvironmentDelete:
    """Tests for environment deletion."""

    def test_delete_environment(self, cli: RailwayCLI, shared_project, unique_name):
        """Test creating and then deleting an environment."""
        _wait_for_env_rate_limit()
        env_name = unique_name("del")

        # Create first
        create_result = cli.env_new(env_name, cwd=shared_project["path"])
        _record_env_created()
        assert create_result.success, f"Failed to create: {create_result.stderr}"

        # Delete
        delete_result = cli.env_delete(env_name, cwd=shared_project["path"])
        assert delete_result.success, f"Failed to delete: {delete_result.stderr}"

    def test_delete_nonexistent_environment(self, cli: RailwayCLI, shared_project):
        """Test error handling when deleting non-existent environment."""
        result = cli.env_delete(
            "nonexistent-env-xyz-12345",
            cwd=shared_project["path"],
        )

        assert not result.success


class TestEnvironmentSwitch:
    """Tests for switching between environments."""

    def test_switch_environment(self, cli: RailwayCLI, shared_project, unique_name):
        """Test switching to a different environment and back."""
        _wait_for_env_rate_limit()
        env_name = unique_name("switch")

        create_result = cli.env_new(env_name, cwd=shared_project["path"])
        _record_env_created()
        assert create_result.success, f"Failed to create: {create_result.stderr}"

        try:
            # Switch to new environment
            result = cli.env_link(env_name, cwd=shared_project["path"])
            assert result.success, f"Failed to switch: {result.stderr}"

            # Switch back to production
            back_result = cli.env_link("production", cwd=shared_project["path"])
            assert back_result.success, f"Failed to switch back: {back_result.stderr}"
        finally:
            # Ensure we're back on production before deleting
            cli.env_link("production", cwd=shared_project["path"])
            cli.env_delete(env_name, cwd=shared_project["path"])

    def test_switch_to_nonexistent_environment(self, cli: RailwayCLI, shared_project):
        """Test error handling when switching to non-existent environment."""
        result = cli.env_link(
            "nonexistent-env-abc",
            cwd=shared_project["path"],
        )

        assert not result.success
