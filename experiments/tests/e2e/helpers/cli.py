"""Railway CLI wrapper for E2E testing."""
import subprocess
import json
import os
from dataclasses import dataclass
from typing import Optional


@dataclass
class CLIResult:
    """Result of a CLI command execution."""
    exit_code: int
    stdout: str
    stderr: str
    json_output: Optional[dict | list] = None

    @property
    def success(self) -> bool:
        return self.exit_code == 0


class RailwayCLI:
    """Wrapper for Railway CLI commands."""

    GRAPHQL_URL = "https://backboard.railway.com/graphql/v2"

    def __init__(
        self,
        binary_path: str = "railway",
        token: Optional[str] = None,
    ):
        self.binary = binary_path
        self.token = token or os.environ.get("RAILWAY_E2E_TOKEN")

    def run(
        self,
        *args: str,
        parse_json: bool = False,
        cwd: Optional[str] = None,
        timeout: int = 30,
    ) -> CLIResult:
        """Execute a railway CLI command."""
        env = os.environ.copy()
        if self.token:
            env["RAILWAY_API_TOKEN"] = self.token

        cmd = [self.binary, *args]

        try:
            result = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                env=env,
                cwd=cwd,
                timeout=timeout,
            )
        except subprocess.TimeoutExpired:
            return CLIResult(
                exit_code=124,
                stdout="",
                stderr="Command timed out",
            )

        json_output = None
        if parse_json and result.returncode == 0 and result.stdout.strip():
            try:
                json_output = json.loads(result.stdout)
            except json.JSONDecodeError:
                pass

        return CLIResult(
            exit_code=result.returncode,
            stdout=result.stdout,
            stderr=result.stderr,
            json_output=json_output,
        )

    # ─────────────────────────────────────────────────────────────────
    # GraphQL API (for operations the CLI doesn't support)
    # ─────────────────────────────────────────────────────────────────

    def _graphql(self, query: str) -> dict:
        """Execute a GraphQL mutation/query via curl."""
        payload = json.dumps({"query": query})
        result = subprocess.run(
            [
                "curl", "-s", "-X", "POST",
                self.GRAPHQL_URL,
                "-H", "Content-Type: application/json",
                "-H", f"Authorization: Bearer {self.token}",
                "-d", payload,
            ],
            capture_output=True,
            text=True,
            timeout=15,
        )
        return json.loads(result.stdout)

    def project_delete(self, project_id: str) -> bool:
        """Delete a project via GraphQL API (no CLI command exists)."""
        result = self._graphql(
            f'mutation {{ projectDelete(id: "{project_id}") }}'
        )
        return result.get("data", {}).get("projectDelete", False)

    # ─────────────────────────────────────────────────────────────────
    # Project Operations
    # ─────────────────────────────────────────────────────────────────

    def project_init(self, name: str) -> CLIResult:
        """Create a new project."""
        return self.run("init", "--name", name)

    def project_list(self) -> CLIResult:
        """List all projects."""
        return self.run("list", "--json", parse_json=True)

    def project_status(self, cwd: Optional[str] = None) -> CLIResult:
        """Get current project status."""
        return self.run("status", "--json", parse_json=True, cwd=cwd)

    def project_link(
        self,
        project: str,
        environment: Optional[str] = None,
        cwd: Optional[str] = None,
    ) -> CLIResult:
        """Link to a project."""
        args = ["link", "--project", project]
        if environment:
            args.extend(["--environment", environment])
        return self.run(*args, cwd=cwd)

    def project_unlink(self, cwd: Optional[str] = None) -> CLIResult:
        """Unlink from current project.

        The CLI's `unlink` command requires interactive TTY confirmation
        that cannot be automated. Instead, we remove the project entry
        from Railway's config file directly.
        """
        config_path = os.path.expanduser("~/.railway/config.json")
        try:
            with open(config_path, "r") as f:
                config = json.load(f)

            target_path = cwd or os.getcwd()
            if target_path in config.get("projects", {}):
                del config["projects"][target_path]
                with open(config_path, "w") as f:
                    json.dump(config, f, indent=2)

            return CLIResult(exit_code=0, stdout="Unlinked\n", stderr="")
        except Exception as e:
            return CLIResult(exit_code=1, stdout="", stderr=str(e))

    # ─────────────────────────────────────────────────────────────────
    # Environment Operations
    # ─────────────────────────────────────────────────────────────────

    def env_new(
        self,
        name: str,
        duplicate_from: Optional[str] = None,
        cwd: Optional[str] = None,
    ) -> CLIResult:
        """Create a new environment."""
        args = ["environment", "new", name]
        if duplicate_from:
            args.extend(["--duplicate", duplicate_from])
        return self.run(*args, cwd=cwd)

    def env_delete(self, name: str, cwd: Optional[str] = None) -> CLIResult:
        """Delete an environment."""
        return self.run("environment", "delete", name, "--yes", cwd=cwd)

    def env_link(self, name: str, cwd: Optional[str] = None) -> CLIResult:
        """Switch to/link an environment."""
        return self.run("environment", name, cwd=cwd)
