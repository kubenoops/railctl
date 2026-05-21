# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| Latest  | ✅        |
| < Latest | ❌       |

We recommend always using the latest release.

## Reporting a Vulnerability

If you discover a security vulnerability in railctl, please report it responsibly.

**Do NOT open a public GitHub issue for security vulnerabilities.**

Instead, please email: **security@kubenoops.dev**

Include:
- Description of the vulnerability
- Steps to reproduce
- Impact assessment
- Any suggested fixes (optional)

We will acknowledge receipt within 48 hours and aim to provide a fix or mitigation plan within 7 days for critical issues.

## Security Practices

railctl follows these security practices:

- **No secrets in source code** — All credentials are loaded from environment variables
- **Sensitive value masking** — The `--show-values` flag is required to display secrets; values are masked by default
- **Debug log redaction** — GraphQL variables containing sensitive data are automatically redacted in debug output
- **Minimal dependencies** — Only two direct dependencies (`cobra`, `yaml.v3`) to minimize supply chain risk
- **Dependabot enabled** — Automated dependency updates for Go modules and GitHub Actions
- **CI/CD hardening** — Workflows use least-privilege permissions and pin action versions

## Credential Handling

railctl requires a Railway API token to operate. Best practices:

- Store tokens in environment variables, never in files
- Use scoped tokens with minimal permissions
- Rotate tokens regularly
- Never commit tokens to version control
