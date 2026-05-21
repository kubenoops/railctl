"""
Unit tests for the Docker Compose → Railway importer.

Run with:
    cd tools/compose-importer
    python -m pytest test_compose_importer.py -v
"""

import os
import textwrap
from pathlib import Path
from unittest.mock import patch

import pytest
import yaml

from compose_importer import (
    DeployConfig,
    ImportWarning,
    VolumeConfig,
    _parse_duration_seconds,
    _rewrite_service_references,
    build_import_plan,
    extract_healthcheck,
    extract_internal_ports,
    is_secret_value,
    mask_value,
    parse_compose_file,
    parse_deploy_config,
    parse_environment_block,
    parse_volumes,
    resolve_env_files,
    substitute_variables,
    transform_variables,
    validate_compose,
)


# ─── Fixtures ────────────────────────────────────────────────────────────────


@pytest.fixture
def compose_dir(tmp_path):
    """Create a temporary compose directory."""
    return tmp_path


def write_compose(compose_dir: Path, content: str) -> Path:
    """Helper to write a compose file."""
    path = compose_dir / "docker-compose.yml"
    path.write_text(textwrap.dedent(content))
    return path


def write_env_file(compose_dir: Path, filename: str, content: str) -> Path:
    """Helper to write an env file."""
    path = compose_dir / filename
    path.write_text(textwrap.dedent(content))
    return path


# ─── Parse Environment Block ────────────────────────────────────────────────


class TestParseEnvironmentBlock:
    def test_dict_form(self):
        env = {"NODE_ENV": "production", "PORT": 3000}
        result = parse_environment_block(env)
        assert result == {"NODE_ENV": "production", "PORT": "3000"}

    def test_list_form_with_equals(self):
        env = ["NODE_ENV=production", "PORT=3000"]
        result = parse_environment_block(env)
        assert result == {"NODE_ENV": "production", "PORT": "3000"}

    def test_list_form_key_only(self):
        with patch.dict(os.environ, {"MY_VAR": "hello"}):
            result = parse_environment_block(["MY_VAR"])
            assert result == {"MY_VAR": "hello"}

    def test_none(self):
        assert parse_environment_block(None) == {}

    def test_none_value_in_dict(self):
        result = parse_environment_block({"KEY": None})
        assert result == {"KEY": ""}


# ─── Variable Substitution ──────────────────────────────────────────────────


class TestSubstituteVariables:
    def test_simple_substitution(self):
        result, unresolved = substitute_variables(
            "user:${DB_PASSWORD}@host",
            {"DB_PASSWORD": "secret123"},
        )
        assert result == "user:secret123@host"
        assert unresolved == []

    def test_default_value(self):
        result, unresolved = substitute_variables(
            "${VERSION:-16}",
            {},
        )
        assert result == "16"
        assert unresolved == []

    def test_unresolved(self):
        result, unresolved = substitute_variables(
            "${MISSING_VAR}",
            {},
        )
        assert result == "${MISSING_VAR}"
        assert unresolved == ["MISSING_VAR"]

    def test_railway_reference_preserved(self):
        result, unresolved = substitute_variables(
            "${{postgres.RAILWAY_PRIVATE_DOMAIN}}",
            {},
        )
        assert result == "${{postgres.RAILWAY_PRIVATE_DOMAIN}}"
        assert unresolved == []

    def test_mixed(self):
        result, unresolved = substitute_variables(
            "postgres://${DB_USER}:${DB_PASS}@${{postgres.RAILWAY_PRIVATE_DOMAIN}}:5432",
            {"DB_USER": "admin", "DB_PASS": "pw123"},
        )
        assert result == "postgres://admin:pw123@${{postgres.RAILWAY_PRIVATE_DOMAIN}}:5432"
        assert unresolved == []

    def test_env_fallback(self):
        with patch.dict(os.environ, {"FROM_ENV": "env_value"}):
            result, unresolved = substitute_variables("${FROM_ENV}", {})
            assert result == "env_value"
            assert unresolved == []


# ─── env_file Resolution ────────────────────────────────────────────────────


class TestResolveEnvFiles:
    def test_simple_env_file(self, compose_dir):
        write_env_file(compose_dir, ".env", "DB_HOST=localhost\nDB_PORT=5432\n")
        variables, secrets, warnings = resolve_env_files(
            "api", ".env", compose_dir
        )
        assert variables == {"DB_HOST": "localhost", "DB_PORT": "5432"}
        assert secrets == []

    def test_secrets_file_convention(self, compose_dir):
        write_env_file(compose_dir, ".env.secrets", "PASSWORD=abc\n")
        variables, secrets, warnings = resolve_env_files(
            "api", ".env.secrets", compose_dir
        )
        assert variables == {"PASSWORD": "abc"}
        assert secrets == [".env.secrets"]

    def test_named_secrets_file(self, compose_dir):
        write_env_file(compose_dir, ".env.db.secrets", "DB_PASS=xyz\n")
        variables, secrets, warnings = resolve_env_files(
            "api", ".env.db.secrets", compose_dir
        )
        assert secrets == [".env.db.secrets"]

    def test_multiple_env_files(self, compose_dir):
        write_env_file(compose_dir, ".env", "A=1\n")
        write_env_file(compose_dir, ".env.secrets", "B=2\n")
        variables, secrets, warnings = resolve_env_files(
            "api", [".env", ".env.secrets"], compose_dir
        )
        assert variables == {"A": "1", "B": "2"}
        assert ".env.secrets" in secrets

    def test_missing_env_file(self, compose_dir):
        with pytest.raises(SystemExit):
            resolve_env_files("api", ".env.missing", compose_dir)

    def test_comments_and_blanks(self, compose_dir):
        write_env_file(compose_dir, ".env", "# comment\n\nKEY=value\n")
        variables, _, _ = resolve_env_files("api", ".env", compose_dir)
        assert variables == {"KEY": "value"}

    def test_quoted_values(self, compose_dir):
        write_env_file(compose_dir, ".env", 'KEY="quoted value"\nKEY2=\'single\'\n')
        variables, _, _ = resolve_env_files("api", ".env", compose_dir)
        assert variables["KEY"] == "quoted value"
        assert variables["KEY2"] == "single"


# ─── Port Extraction ────────────────────────────────────────────────────────


class TestExtractInternalPorts:
    def test_short_syntax_host_container(self):
        assert extract_internal_ports(["8080:3000"]) == [3000]

    def test_short_syntax_container_only(self):
        assert extract_internal_ports(["3000"]) == [3000]

    def test_long_syntax(self):
        ports = [{"target": 3000, "published": 8080}]
        assert extract_internal_ports(ports) == [3000]

    def test_multiple_ports(self):
        assert extract_internal_ports(["8080:3000", "9090:4000"]) == [3000, 4000]

    def test_with_protocol(self):
        assert extract_internal_ports(["8080:3000/tcp"]) == [3000]

    def test_deduplication(self):
        assert extract_internal_ports(["8080:3000", "9090:3000"]) == [3000]

    def test_none(self):
        assert extract_internal_ports(None) == []

    def test_integer_port(self):
        assert extract_internal_ports([3000]) == [3000]


# ─── Healthcheck Extraction ─────────────────────────────────────────────────


class TestExtractHealthcheck:
    def test_curl_pattern(self):
        hc = {"test": ["CMD", "curl", "-f", "http://localhost:3000/health"]}
        path, timeout, warnings = extract_healthcheck(hc, "api")
        assert path == "/health"

    def test_wget_pattern(self):
        hc = {"test": "wget -q http://localhost:8080/ready"}
        path, timeout, warnings = extract_healthcheck(hc, "api")
        assert path == "/ready"

    def test_non_http(self):
        hc = {"test": ["CMD", "pg_isready"]}
        path, timeout, warnings = extract_healthcheck(hc, "db")
        assert path is None
        assert len(warnings) == 1
        assert "Non-HTTP" in warnings[0].message

    def test_timeout(self):
        hc = {"test": "curl http://localhost/health", "timeout": "30s"}
        path, timeout, warnings = extract_healthcheck(hc, "api")
        assert timeout == 30

    def test_warns_interval_retries(self):
        hc = {"test": "curl http://localhost/health", "interval": "10s", "retries": 3}
        _, _, warnings = extract_healthcheck(hc, "api")
        assert len(warnings) == 2  # interval + retries

    def test_none(self):
        path, timeout, warnings = extract_healthcheck(None, "api")
        assert path is None
        assert timeout is None
        assert warnings == []


# ─── Duration Parsing ────────────────────────────────────────────────────────


class TestParseDuration:
    def test_seconds_string(self):
        assert _parse_duration_seconds("30s") == 30

    def test_minutes_string(self):
        assert _parse_duration_seconds("2m") == 120

    def test_hours_string(self):
        assert _parse_duration_seconds("1h") == 3600

    def test_integer(self):
        assert _parse_duration_seconds(30) == 30

    def test_plain_number_string(self):
        assert _parse_duration_seconds("45") == 45


# ─── Deploy Config Parsing ──────────────────────────────────────────────────


class TestParseDeployConfig:
    def test_command_string(self):
        svc = {"command": "npm start"}
        config, warnings = parse_deploy_config("api", svc)
        assert config.start_command == "npm start"

    def test_command_list(self):
        svc = {"command": ["npm", "run", "start"]}
        config, warnings = parse_deploy_config("api", svc)
        assert config.start_command == "npm run start"

    def test_entrypoint_fallback(self):
        svc = {"entrypoint": "/docker-entrypoint.sh"}
        config, warnings = parse_deploy_config("api", svc)
        assert config.start_command == "/docker-entrypoint.sh"

    def test_command_takes_priority_over_entrypoint(self):
        svc = {"command": "npm start", "entrypoint": "/entry.sh"}
        config, warnings = parse_deploy_config("api", svc)
        assert config.start_command == "npm start"

    def test_restart_always(self):
        svc = {"restart": "always"}
        config, _ = parse_deploy_config("api", svc)
        assert config.restart_policy_type == "ALWAYS"

    def test_restart_on_failure(self):
        svc = {"restart": "on-failure"}
        config, _ = parse_deploy_config("api", svc)
        assert config.restart_policy_type == "ON_FAILURE"

    def test_restart_no(self):
        svc = {"restart": "no"}
        config, _ = parse_deploy_config("api", svc)
        assert config.restart_policy_type == "NEVER"

    def test_restart_unless_stopped(self):
        svc = {"restart": "unless-stopped"}
        config, _ = parse_deploy_config("api", svc)
        assert config.restart_policy_type == "ALWAYS"

    def test_deploy_replicas(self):
        svc = {"deploy": {"replicas": 3}}
        config, _ = parse_deploy_config("api", svc)
        assert config.num_replicas == 3

    def test_deploy_restart_policy(self):
        svc = {"deploy": {"restart_policy": {"condition": "on-failure", "max_attempts": 5}}}
        config, _ = parse_deploy_config("api", svc)
        assert config.restart_policy_type == "ON_FAILURE"
        assert config.restart_policy_max_retries == 5

    def test_deploy_warns_resources(self):
        svc = {"deploy": {"resources": {"limits": {"memory": "512M"}}}}
        _, warnings = parse_deploy_config("api", svc)
        assert any("Resource limits" in w.message for w in warnings)

    def test_deploy_warns_placement(self):
        svc = {"deploy": {"placement": {"constraints": ["node.role==manager"]}}}
        _, warnings = parse_deploy_config("api", svc)
        assert any("Placement" in w.message for w in warnings)


# ─── Deploy Config to API ───────────────────────────────────────────────────


class TestDeployConfigToApi:
    def test_full_config(self):
        config = DeployConfig(
            start_command="npm start",
            restart_policy_type="ON_FAILURE",
            restart_policy_max_retries=10,
            num_replicas=2,
            healthcheck_path="/health",
            healthcheck_timeout=30,
        )
        result = config.to_api_input()
        assert result == {
            "startCommand": "npm start",
            "restartPolicyType": "ON_FAILURE",
            "restartPolicyMaxRetries": 10,
            "numReplicas": 2,
            "healthcheckPath": "/health",
            "healthcheckTimeout": 30,
        }

    def test_empty_config(self):
        config = DeployConfig()
        assert config.to_api_input() == {}

    def test_partial_config(self):
        config = DeployConfig(start_command="npm start")
        result = config.to_api_input()
        assert result == {"startCommand": "npm start"}
        assert "restartPolicyType" not in result


# ─── Volume Parsing ──────────────────────────────────────────────────────────


class TestParseVolumes:
    def test_named_volume(self):
        svc = {"volumes": ["pgdata:/var/lib/postgresql/data"]}
        volumes, warnings = parse_volumes("db", svc, {"pgdata": {}})
        assert len(volumes) == 1
        assert volumes[0].mount_path == "/var/lib/postgresql/data"
        assert not volumes[0].is_bind_mount

    def test_bind_mount(self):
        svc = {"volumes": ["./data:/var/lib/data"]}
        volumes, warnings = parse_volumes("db", svc, {})
        assert len(volumes) == 1
        assert volumes[0].is_bind_mount
        assert any("Bind mount" in w.message for w in warnings)

    def test_absolute_bind_mount(self):
        svc = {"volumes": ["/host/path:/container/path"]}
        volumes, warnings = parse_volumes("db", svc, {})
        assert volumes[0].is_bind_mount

    def test_long_syntax(self):
        svc = {"volumes": [{"type": "volume", "source": "mydata", "target": "/data"}]}
        volumes, _ = parse_volumes("db", svc, {})
        assert volumes[0].mount_path == "/data"
        assert not volumes[0].is_bind_mount

    def test_no_volumes(self):
        volumes, warnings = parse_volumes("db", {}, {})
        assert volumes == []
        assert warnings == []


# ─── Validation ──────────────────────────────────────────────────────────────


class TestValidateCompose:
    def test_valid_minimal(self):
        data = {"services": {"api": {"image": "node:18"}}}
        services, warnings = validate_compose(data)
        assert "api" in services

    def test_no_services(self):
        with pytest.raises(SystemExit):
            validate_compose({})

    def test_no_image(self):
        data = {"services": {"api": {"build": "."}}}
        with pytest.raises(SystemExit):
            validate_compose(data)

    def test_build_with_image(self):
        data = {"services": {"api": {"image": "my-app:latest", "build": "."}}}
        services, warnings = validate_compose(data)
        assert "api" in services
        assert any("build" in w.feature for w in warnings)

    def test_warns_networks(self):
        data = {
            "services": {"api": {"image": "node:18"}},
            "networks": {"frontend": {}, "backend": {}},
        }
        _, warnings = validate_compose(data)
        net_warnings = [w for w in warnings if w.feature == "networks"]
        assert len(net_warnings) == 2

    def test_warns_depends_on(self):
        data = {"services": {"api": {"image": "node:18", "depends_on": ["db"]}}}
        _, warnings = validate_compose(data)
        assert any("depends_on" in w.feature for w in warnings)

    def test_warns_links(self):
        data = {"services": {"api": {"image": "node:18", "links": ["db"]}}}
        _, warnings = validate_compose(data)
        assert any("links" in w.feature for w in warnings)

    def test_warns_privileged(self):
        data = {"services": {"api": {"image": "node:18", "privileged": True}}}
        _, warnings = validate_compose(data)
        assert any("privileged" in w.feature for w in warnings)

    def test_warns_secrets(self):
        data = {
            "services": {"api": {"image": "node:18"}},
            "secrets": {"db_pass": {"file": "./secrets/db"}},
        }
        _, warnings = validate_compose(data)
        assert any("secrets" in w.feature for w in warnings)

    def test_warns_configs(self):
        data = {
            "services": {"api": {"image": "node:18"}},
            "configs": {"nginx": {"file": "./nginx.conf"}},
        }
        _, warnings = validate_compose(data)
        assert any("configs" in w.feature for w in warnings)

    def test_version_informational(self):
        data = {"version": "3.8", "services": {"api": {"image": "node:18"}}}
        _, warnings = validate_compose(data)
        assert any("version" in w.feature for w in warnings)


# ─── Variable Transformation ────────────────────────────────────────────────


class TestTransformVariables:
    def test_drops_railway_vars(self):
        variables = {
            "NODE_ENV": "production",
            "RAILWAY_STATIC_URL": "...",
            "RAILWAY_GIT_COMMIT_SHA": "abc",
        }
        result = transform_variables(variables, set(), {})
        assert "NODE_ENV" in result
        assert "RAILWAY_STATIC_URL" not in result
        assert "RAILWAY_GIT_COMMIT_SHA" not in result

    def test_plain_vars_pass_through(self):
        variables = {"NODE_ENV": "production", "PORT": "3000"}
        result = transform_variables(variables, {"other"}, {})
        assert result == variables

    def test_preserves_railway_references(self):
        variables = {"DB_HOST": "${{postgres.RAILWAY_PRIVATE_DOMAIN}}"}
        result = transform_variables(variables, {"postgres"}, {})
        assert result["DB_HOST"] == "${{postgres.RAILWAY_PRIVATE_DOMAIN}}"


class TestRewriteServiceReferences:
    def test_service_with_port(self):
        result = _rewrite_service_references(
            "postgres:5432", {"postgres"}, {}
        )
        assert result == "postgres.railway.internal:5432"

    def test_service_in_url(self):
        result = _rewrite_service_references(
            "http://api:3000/path", {"api"}, {}
        )
        assert result == "http://api.railway.internal:3000/path"

    def test_connection_string(self):
        result = _rewrite_service_references(
            "postgres://user:pass@db:5432/mydb",
            {"db"},
            {},
        )
        assert result == "postgres://user:pass@db.railway.internal:5432/mydb"

    def test_no_false_positives(self):
        """Should not rewrite words that happen to match service names in plain text."""
        result = _rewrite_service_references(
            "some random text about api usage",
            {"api"},
            {},
        )
        # 'api' here is not in a URL context or with a port
        assert "railway.internal" not in result

    def test_multiple_services(self):
        result = _rewrite_service_references(
            "redis://redis:6379 postgres://postgres:5432/db",
            {"redis", "postgres"},
            {},
        )
        assert "redis.railway.internal:6379" in result
        assert "postgres.railway.internal:5432" in result

    def test_already_railway_reference(self):
        result = _rewrite_service_references(
            "${{postgres.RAILWAY_PRIVATE_DOMAIN}}",
            {"postgres"},
            {},
        )
        assert result == "${{postgres.RAILWAY_PRIVATE_DOMAIN}}"

    def test_does_not_double_rewrite(self):
        """Already rewritten values should not be touched."""
        result = _rewrite_service_references(
            "http://api.railway.internal:3000",
            {"api"},
            {},
        )
        assert result.count("railway.internal") == 1


# ─── Secret Masking ─────────────────────────────────────────────────────────


class TestSecretMasking:
    def test_password_detected(self):
        assert is_secret_value("DB_PASSWORD", False)

    def test_secret_detected(self):
        assert is_secret_value("API_SECRET_KEY", False)

    def test_token_detected(self):
        assert is_secret_value("AUTH_TOKEN", False)

    def test_plain_not_detected(self):
        assert not is_secret_value("NODE_ENV", False)
        assert not is_secret_value("PORT", False)

    def test_from_secrets_file(self):
        assert is_secret_value("ANYTHING", True)

    def test_mask_short_value(self):
        assert mask_value("ab") == "********"

    def test_mask_long_value(self):
        masked = mask_value("my-secret-password")
        assert masked.startswith("my")
        assert masked.endswith("rd")
        assert "***" in masked


# ─── Full Plan Build ────────────────────────────────────────────────────────


class TestBuildImportPlan:
    def test_simple_compose(self, compose_dir):
        compose_data = {
            "services": {
                "web": {
                    "image": "nginx:latest",
                    "ports": ["8080:80"],
                    "environment": {"NODE_ENV": "production"},
                },
            }
        }
        plan = build_import_plan(compose_data, compose_dir)
        assert len(plan.services) == 1
        svc = plan.services[0]
        assert svc.name == "web"
        assert svc.image == "nginx:latest"
        assert svc.internal_ports == [80]
        assert svc.variables["NODE_ENV"] == "production"

    def test_multi_service_with_cross_refs(self, compose_dir):
        compose_data = {
            "services": {
                "api": {
                    "image": "myapp:latest",
                    "environment": {
                        "DATABASE_URL": "postgres://user:pass@db:5432/app",
                        "REDIS_URL": "redis://cache:6379",
                    },
                },
                "db": {"image": "postgres:16"},
                "cache": {"image": "redis:7"},
            }
        }
        plan = build_import_plan(compose_data, compose_dir)
        assert len(plan.services) == 3

        api_svc = next(s for s in plan.services if s.name == "api")
        assert "db.railway.internal:5432" in api_svc.variables["DATABASE_URL"]
        assert "cache.railway.internal:6379" in api_svc.variables["REDIS_URL"]

    def test_with_env_file(self, compose_dir):
        write_env_file(compose_dir, ".env", "DB_PASS=secret123\n")
        compose_data = {
            "services": {
                "api": {
                    "image": "myapp:latest",
                    "env_file": ".env",
                    "environment": {"NODE_ENV": "production"},
                },
            }
        }
        plan = build_import_plan(compose_data, compose_dir)
        svc = plan.services[0]
        # env_file vars + inline vars merged
        assert svc.variables["DB_PASS"] == "secret123"
        assert svc.variables["NODE_ENV"] == "production"

    def test_with_volumes(self, compose_dir):
        compose_data = {
            "services": {
                "db": {
                    "image": "postgres:16",
                    "volumes": ["pgdata:/var/lib/postgresql/data"],
                },
            },
            "volumes": {"pgdata": {}},
        }
        plan = build_import_plan(compose_data, compose_dir)
        svc = plan.services[0]
        assert len(svc.volumes) == 1
        assert svc.volumes[0].mount_path == "/var/lib/postgresql/data"

    def test_drops_railway_vars(self, compose_dir):
        compose_data = {
            "services": {
                "api": {
                    "image": "myapp:latest",
                    "environment": {
                        "NODE_ENV": "production",
                        "RAILWAY_STATIC_URL": "should_be_dropped",
                    },
                },
            }
        }
        plan = build_import_plan(compose_data, compose_dir)
        svc = plan.services[0]
        assert "NODE_ENV" in svc.variables
        assert "RAILWAY_STATIC_URL" not in svc.variables

    def test_secrets_files_tracked(self, compose_dir):
        write_env_file(compose_dir, ".env.secrets", "SECRET=val\n")
        compose_data = {
            "services": {
                "api": {
                    "image": "myapp:latest",
                    "env_file": ".env.secrets",
                },
            }
        }
        plan = build_import_plan(compose_data, compose_dir)
        assert ".env.secrets" in plan.secrets_files

    def test_deploy_config_mapped(self, compose_dir):
        compose_data = {
            "services": {
                "api": {
                    "image": "myapp:latest",
                    "command": "npm start",
                    "restart": "on-failure",
                    "deploy": {"replicas": 3},
                },
            }
        }
        plan = build_import_plan(compose_data, compose_dir)
        svc = plan.services[0]
        assert svc.deploy.start_command == "npm start"
        assert svc.deploy.restart_policy_type == "ON_FAILURE"
        assert svc.deploy.num_replicas == 3
