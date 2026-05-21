# Railway CLI E2E Tests

End-to-end integration tests for the Railway CLI using Python and pytest.
These tests invoke the real compiled `railway` binary against the live Railway API.

## Prerequisites

1. **Python 3.10+**
2. **Built Railway CLI binary** (`cargo build --release`)
3. **Railway account with API token** (Account Settings → Tokens → Create Token)

## Setup

```bash
cd tests/e2e

# Create virtual environment and install dependencies
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt

# Set environment variables
export RAILWAY_E2E_TOKEN="your-api-token"

# Optional: path to binary (defaults to ../../target/release/railway)
export RAILWAY_CLI_PATH="/path/to/railway"
```

## Running Tests

```bash
# Run all tests (~5 min due to rate limit cooldowns)
pytest -v -s

# Run only project tests
pytest test_projects.py -v -s

# Run only environment tests
pytest test_environments.py -v -s

# Run a single test
pytest test_projects.py::TestProjectCreate::test_create_project_appears_in_list -v -s
```

## Test List

### Project Tests (`test_projects.py`)

| Test | Description |
|------|-------------|
| `TestProjectCreate::test_create_project_appears_in_list` | Creates a new project via `railway init --name`, then runs `railway list --json` and verifies the newly created project appears in the JSON output with a valid ID. |
| `TestProjectLink::test_link_project_by_id` | Creates a project, then links it to a temporary directory using `railway link --project <id>`. Verifies the link succeeds and then unlinks. |
| `TestProjectLink::test_link_and_unlink` | Creates a project, links it via `railway link`, then unlinks it by removing the entry from `~/.railway/config.json` (the CLI's `unlink` command requires interactive TTY confirmation). Verifies both operations succeed. |
| `TestProjectStatus::test_status_when_linked` | Links to a project and runs `railway status --json`. Verifies the command succeeds and returns valid JSON output containing project information. |
| `TestProjectStatus::test_status_when_not_linked` | Runs `railway status --json` from a directory that is not linked to any project. Verifies the command fails gracefully. |

### Environment Tests (`test_environments.py`)

| Test | Description |
|------|-------------|
| `TestEnvironmentCreate::test_create_environment` | Creates a new environment via `railway environment new <name>` inside a linked project. Verifies creation succeeds, then cleans up by deleting the environment. |
| `TestEnvironmentCreate::test_create_environment_with_duplicate` | Creates a new environment by duplicating from production using `railway environment new <name> --duplicate production`. Verifies the duplicated environment is created successfully. |
| `TestEnvironmentCreate::test_create_multiple_environments` | Creates two environments in sequence (with rate limit waits between them). Verifies each creation succeeds, confirming a project can hold multiple custom environments. |
| `TestEnvironmentDelete::test_delete_environment` | Creates an environment and then deletes it via `railway environment delete <name> --yes`. Verifies both the creation and deletion succeed. |
| `TestEnvironmentDelete::test_delete_nonexistent_environment` | Attempts to delete an environment name that doesn't exist. Verifies the CLI returns a non-zero exit code (graceful error). |
| `TestEnvironmentSwitch::test_switch_environment` | Creates a new environment, switches to it via `railway environment <name>`, then switches back to production. Verifies both switches succeed, confirming environment context switching works. |
| `TestEnvironmentSwitch::test_switch_to_nonexistent_environment` | Attempts to switch to an environment that doesn't exist. Verifies the CLI returns a non-zero exit code. |

## Architecture

```
tests/e2e/
├── helpers/
│   ├── __init__.py
│   └── cli.py           # RailwayCLI wrapper class
├── conftest.py          # Pytest fixtures and rate limit handling
├── test_projects.py     # Project CRUD tests (5 tests)
├── test_environments.py # Environment CRUD tests (7 tests)
├── requirements.txt
├── pytest.ini
└── README.md
```

### Key Design Decisions

- **`RAILWAY_API_TOKEN`** is used (not `RAILWAY_TOKEN`). `RAILWAY_TOKEN` is for project-scoped tokens only; `RAILWAY_API_TOKEN` provides user-level bearer auth needed for creating/listing projects.
- **Rate limit cooldowns**: Railway enforces a 30-second cooldown between project and environment creation. Tests automatically wait between resource-creating operations.
- **Session-scoped shared project**: All environment tests share a single project (created once) to minimize API calls and avoid hitting the free-plan project limit.
- **GraphQL cleanup**: The CLI has no `delete` command for projects. Cleanup uses the Railway GraphQL API (`projectDelete` mutation) via `curl`.
- **Config-based unlink**: The CLI's `unlink` command requires interactive TTY confirmation that can't be automated. The test framework removes the link entry from `~/.railway/config.json` directly.
- **Unique resource names**: All resources are created with `e2e-{prefix}-{uuid}` names for easy identification and cleanup.

## Cleanup

If tests fail mid-run and leave orphaned projects, clean them up:

```bash
export RAILWAY_API_TOKEN="your-token"

# List all projects
railway list --json

# Delete via GraphQL API
curl -s -X POST https://backboard.railway.com/graphql/v2 \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $RAILWAY_API_TOKEN" \
  -d '{"query":"mutation { projectDelete(id: \"PROJECT_ID\") }"}'
```
