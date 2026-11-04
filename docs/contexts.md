# Contexts and Credentials

railctl contexts are kubectl-style local profiles: save a targeting tuple
and a login identity once, then switch everything with a single command.

## Quick reference

| Command                                                                | Purpose                                                        |
| ---------------------------------------------------------------------- | -------------------------------------------------------------- |
| `railctl config set-credentials <name> (--token <v> \| --token-stdin)` | Store a Railway token under a name (exactly one input channel) |
| `railctl config get-credentials`                                       | List credential names + token types (values never shown)       |
| `railctl config delete-credentials <name>`                             | Delete a credential (refused while referenced)                 |
| `railctl config set-context <name> [-w -p -e --credentials]`           | Create/update a context (upsert: only passed flags change)     |
| `railctl config set-context --current -e <env>`                        | Update the active context in place                             |
| `railctl config get-contexts`                                          | List contexts, marking the active one                          |
| `railctl config current-context`                                       | Print the active context name                                  |
| `railctl config use-context <name>`                                    | Switch the active context (local write, no API calls)          |
| `railctl config delete-context <name>`                                 | Delete a context (clears the active slot if active)            |
| `railctl --context <name> <command>`                                   | Use a context for one command only                             |

## The model

- **Context** — `{workspace?, project (required), environment?,
credentials?}`. The `environment` field works like kubectl's default
  namespace: commands that need an environment use it unless `-e` overrides.
  Service is never part of a context.
- **Credential** — a named Railway token (any type: account, workspace, or
  project; detected automatically when stored). Contexts reference
  credentials **by name**, so one token can back many contexts, and the
  frequently-run `set-context` never touches a secret value.
- **Config file** — `~/.railctl/config.yaml` by default, override with the
  `RAILCTL_CONFIG` environment variable (single path). Created with `0600`
  permissions because it stores tokens; a file with looser permissions is
  refused with the exact `chmod 600` fix. Never commit it.

```yaml
currentContext: shop-prod
contexts:
  - name: shop-prod
    workspace: my-team
    project: shop
    environment: production
    credentials: work
credentials:
  - name: work
    token: <secret>
    tokenType: workspace
```

## Precedence

One uniform rule for targeting and identity alike:

```
flag  >  environment variable  >  manifest (-f commands only)  >  selected context  >  error
```

- Per-field: `-e staging` over a context replaces only the environment; the
  context still supplies the project.
- Identity: `--token > RAILWAY_TOKEN > context credentials`. When
  `RAILWAY_TOKEN` overrides the context's credentials, railctl prints a
  stderr warning naming both sources.
- For `apply`/`diff`/`delete -f`, the manifest's `project:`/`environment:`
  fields beat the context; the context fills only what the manifest omits.

## Safety properties

- **No config file = pre-context behavior**, byte for byte. CI and scripts
  are unaffected by the feature's existence.
- **Token values are never displayed** by any command in any output format;
  `get-contexts`/`get-credentials` show token _types_ only.
- **No dangling references**: `set-context` validates credential references
  at write time and `delete-credentials` refuses while referenced; a
  hand-edited dangling reference fails loudly at load, naming the context
  and the missing credential.
- **`--token-stdin` is the preferred input path** — it keeps token values
  out of shell history. `--token` exists for scripting convenience.
- **Deleting the active context** clears the active slot with a notice
  (kubectl's dangling current-context behavior is deliberately not copied);
  subsequent commands behave as context-less.
- **`use-context` makes no API calls** — a stale context surfaces its
  not-found error at the next command's normal resolution, listing the
  available candidates.

## Shell prompt integration

```bash
# zsh/bash prompt snippet
PS1="[$(railctl config current-context 2>/dev/null)] $PS1"
```
