"""E2E tests for Railway CLI project operations.

These tests create/delete real projects via the Railway API.
Each test that creates a project will wait for the rate limit cooldown (30s).
"""
import pytest

from helpers.cli import RailwayCLI


class TestProjectCreate:
    """Tests for project creation."""

    def test_create_project_appears_in_list(self, cli: RailwayCLI, temp_project):
        """Test that a created project appears in the project list."""
        result = cli.project_list()

        assert result.success
        assert result.json_output is not None

        project_names = [p.get("name") for p in result.json_output]
        assert temp_project["name"] in project_names
        assert temp_project["id"] is not None


class TestProjectLink:
    """Tests for project linking/unlinking."""

    def test_link_project_by_id(self, cli: RailwayCLI, temp_project):
        """Test linking to a project by ID."""
        assert temp_project["id"] is not None, "Project ID not available"

        result = cli.project_link(
            temp_project["id"],
            cwd=temp_project["path"],
        )

        assert result.success, f"Failed: {result.stderr}"

        # Cleanup
        cli.project_unlink(cwd=temp_project["path"])

    def test_link_and_unlink(self, cli: RailwayCLI, temp_project):
        """Test linking and then unlinking from a project."""
        # Link
        link_result = cli.project_link(
            temp_project["id"] or temp_project["name"],
            cwd=temp_project["path"],
        )
        assert link_result.success

        # Unlink
        unlink_result = cli.project_unlink(cwd=temp_project["path"])
        assert unlink_result.success


class TestProjectStatus:
    """Tests for project status."""

    def test_status_when_linked(self, cli: RailwayCLI, linked_project):
        """Test getting status when linked to a project."""
        result = cli.project_status(cwd=linked_project["path"])

        assert result.success
        assert result.json_output is not None

    def test_status_when_not_linked(self, cli: RailwayCLI, tmp_path):
        """Test getting status when not linked to any project."""
        result = cli.project_status(cwd=str(tmp_path))

        # Should fail when not linked
        assert not result.success
