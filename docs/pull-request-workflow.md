# Pull Request Workflow

This document describes the full lifecycle of a pull request (PR) in the `railctl` repository — what runs automatically, what requires manual intervention, and what gates must pass before merging.

---

## Overview

```
Developer pushes branch
        │
        ▼
┌──────────────────────┐
│   PR Opened           │
│                       │
│  ✅ Unit tests (auto) │
│  ⏳ E2E tests (wait)  │
│  ⏳ Review (wait)     │
└──────────┬────────────┘
           │
     Admin reviews code
           │
     ┌─────┴──────┐
     │ Comments    │
     │ /run-e2e   │
     └─────┬──────┘
           │
           ▼
┌──────────────────────┐
│  E2E Tests Run        │
│  (admin team verified)│
└──────────┬────────────┘
           │
     Admin approves PR
           │
           ▼
┌──────────────────────┐
│  All Gates Pass?      │
│  ✅ Unit tests        │
│  ✅ E2E tests         │
│  ✅ 1 approval        │
│  ✅ Conversations     │
│     resolved          │
└──────────┬────────────┘
           │
     Rebase & merge
           │
           ▼
┌──────────────────────┐
│  Merged to main       │
│  E2E runs again (auto)│
└──────────────────────┘
```

---

## What Runs Automatically

### On Every Push to a PR Branch

| Workflow | File | What it does | Duration |
|----------|------|-------------|----------|
| **PR Tests** | `.github/workflows/pr.yml` | Builds the binary, runs all unit + integration tests (`go test ./...`), posts results as a PR comment | ~2 min |

This requires **no human action** — it triggers on every push to any PR targeting `main`.

### On Merge to `main`

| Workflow | File | What it does | Duration |
|----------|------|-------------|----------|
| **E2E Tests** | `.github/workflows/e2e.yml` | Full E2E suite against the live Railway API | ~10 min |

This is an automatic safety net. If E2E tests fail after merge, the team is notified immediately.

---

## What Requires Admin Action

### Triggering E2E Tests on a PR

E2E tests do **not** run automatically on PRs. This is a deliberate security decision — E2E tests require Railway API tokens, and those secrets must not be exposed to untrusted code.

**To trigger E2E tests, an admin must:**

1. Review the PR code (especially changes to `.github/`, `tests/e2e/`, and `Makefile`)
2. Comment `/run-e2e` on the PR

**What happens:**

1. The workflow verifies the commenter is a member of `@kubenoops/maintainers`
2. If verified, it reacts with 🚀 to the comment
3. It checks out the **PR's merge commit** (not the branch directly)
4. Decrypts Railway tokens from the encrypted vault using the CI GPG key
5. Runs the full E2E suite
6. Posts results as a PR comment

**If a non-admin comments `/run-e2e`:**
The workflow starts but immediately **aborts** at the team membership check. No secrets are exposed.

**Security model:**
- The `issue_comment` event always runs the workflow file from `main`, not from the PR branch
- This means a PR author **cannot modify the security check** — the workflow code is always trusted
- Secrets are stored in the `e2e` GitHub environment and the encrypted vault

---

## Merge Gates

All of the following must pass before the "Merge" button becomes available:

### 1. Required Status Checks

| Check | Source | Required |
|-------|--------|----------|
| **`test`** | `pr.yml` — unit + integration tests | ✅ Must pass |
| **`e2e`** | `e2e.yml` — E2E tests (admin-triggered) | ✅ Must pass |

Both checks must be **up-to-date** with `main` (`strict: true`). If `main` advances after the checks ran, they must re-run. This prevents merging stale branches.

### 2. Required Reviews

| Rule | Setting |
|------|---------|
| Minimum approvals | **1** |
| Dismiss stale reviews | ✅ Yes — new pushes invalidate prior approvals |

### 3. Conversation Resolution

All PR review comments and threads must be **resolved** before merging.

### 4. Linear History (Rebase Only)

Merge commits are **not allowed**. PRs must be merged via **rebase** or **squash** to maintain a clean, linear commit history.

---

## CODEOWNERS

Certain paths require approval from `@kubenoops/maintainers`:

| Path | Owner | Why |
|------|-------|-----|
| `.github/` | `@kubenoops/maintainers` | CI/CD pipeline changes |
| `Makefile` | `@kubenoops/maintainers` | Build system changes |
| `tests/e2e/` | `@kubenoops/maintainers` | E2E test infrastructure |

If a PR modifies these paths, an admin review is automatically requested.

---

## Step-by-Step Example

Here's a complete example of the PR lifecycle:

```
1. Developer creates a branch and pushes:
   $ git checkout -b feat/add-widget
   $ git push -u origin feat/add-widget

2. Opens a PR → Unit tests run automatically
   ✅ PR Tests (test) — passed

3. Admin reviews the code
   - Checks logic, test coverage, and security-sensitive files
   - Leaves comments if changes are needed

4. Admin triggers E2E tests:
   💬 /run-e2e

5. E2E tests run:
   🚀 (reaction on the comment)
   ✅ E2E Tests (e2e) — passed

6. Admin approves the PR:
   ✅ 1/1 approvals

7. All conversations resolved:
   ✅ No unresolved threads

8. Merge button becomes available:
   🟢 "Rebase and merge"

9. After merge, E2E runs again on main (automatic safety net)
```

---

## Running Tests Locally

Before pushing, developers should run tests locally:

```bash
# Unit + integration tests (always run before pushing)
make test

# Smoke E2E (~1 min, requires Railway token)
make test-smoke

# Full E2E suite (~10 min, requires Railway token)
make test-e2e
```

---

## Quick Reference

| Question | Answer |
|----------|--------|
| Can I merge without E2E? | No — `e2e` is a required status check |
| Can I skip unit tests? | No — `test` is a required status check |
| Who can trigger E2E? | Members of `@kubenoops/maintainers` |
| How do I trigger E2E? | Comment `/run-e2e` on the PR |
| Can I force push to main? | No — force pushes are disabled |
| What merge strategy? | Rebase only (linear history enforced) |
| Do stale approvals count? | No — new pushes dismiss prior approvals |
