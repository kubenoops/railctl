#!/usr/bin/env python3
"""
Clean up leftover test projects from E2E / integration test runs.

SAFETY: Only deletes projects whose names start with 'e2e-'.
This matches the naming conventions used across all test suites:
  - e2e-proj-{uuid}       (from tests/e2e conftest.py)
  - e2e-shared-{uuid}     (from tests/e2e conftest.py)
  - e2e-importer-{uuid}   (from tools/compose-importer integration tests)

Usage:
    # Preview what would be deleted (dry run by default)
    python cleanup.py

    # Actually delete
    python cleanup.py --confirm

    # Use a specific token
    RAILWAY_E2E_TOKEN=<token> python cleanup.py --confirm
"""

import json
import os
import subprocess
import sys

GRAPHQL_URL = "https://backboard.railway.com/graphql/v2"
E2E_PREFIX = "e2e-"


def graphql(token: str, query: str) -> dict:
    """Execute a GraphQL query against Railway's API."""
    result = subprocess.run(
        [
            "curl", "-s", "-X", "POST",
            GRAPHQL_URL,
            "-H", "Content-Type: application/json",
            "-H", f"Authorization: Bearer {token}",
            "-d", json.dumps({"query": query}),
        ],
        capture_output=True,
        text=True,
        timeout=15,
    )
    data = json.loads(result.stdout)
    if "errors" in data:
        print(f"❌ GraphQL error: {data['errors'][0]['message']}")
        sys.exit(1)
    return data


def list_projects(token: str) -> list[dict]:
    """List all projects and filter to e2e- prefix only."""
    data = graphql(
        token,
        "query { projects { edges { node { id name updatedAt } } } }",
    )
    all_projects = [
        edge["node"]
        for edge in data["data"]["projects"]["edges"]
    ]
    return [p for p in all_projects if p["name"].startswith(E2E_PREFIX)]


def delete_project(token: str, project_id: str) -> bool:
    """Delete a project by ID."""
    data = graphql(
        token,
        f'mutation {{ projectDelete(id: "{project_id}") }}',
    )
    return data.get("data", {}).get("projectDelete", False)


def main():
    token = os.environ.get("RAILWAY_E2E_TOKEN")
    if not token:
        print("❌ RAILWAY_E2E_TOKEN environment variable required")
        sys.exit(1)

    confirm = "--confirm" in sys.argv

    print(f"🔍 Searching for leftover test projects (prefix: '{E2E_PREFIX}')...\n")

    projects = list_projects(token)

    if not projects:
        print("✅ No leftover test projects found. All clean!")
        return

    print(f"Found {len(projects)} test project(s):\n")
    for p in projects:
        print(f"  • {p['name']}  ({p['id'][:12]}...)  updated: {p['updatedAt']}")

    if not confirm:
        print(f"\n⚠️  Dry run — nothing deleted.")
        print(f"   Run with --confirm to delete these {len(projects)} project(s).")
        return

    print(f"\n🗑️  Deleting {len(projects)} project(s)...\n")
    deleted = 0
    for p in projects:
        success = delete_project(token, p["id"])
        if success:
            print(f"  ✅ Deleted: {p['name']}")
            deleted += 1
        else:
            print(f"  ❌ Failed:  {p['name']}")

    print(f"\nDone. {deleted}/{len(projects)} projects deleted.")


if __name__ == "__main__":
    main()
