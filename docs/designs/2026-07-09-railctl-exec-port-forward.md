> ## ⚠️ CORRECTION (verified live 2026-07-09, post-design)
> The **three-field `LOCAL:HOST:REMOTE` "jump" form was removed** — it does not
> work. Railway's SSH relay forwards **only to the target container's own
> loopback**; an `-L` to any other host (even a resolvable `*.railway.internal`
> name) yields an empty reply, while the SAME name is reachable with `curl` from
> *inside* that container (proving internal networking is fine — the relay's `-L`
> handler simply doesn't honor non-loopback targets). This is why Railway's own
> CLI always pins `127.0.0.1`. **Private services are reached by forwarding
> directly INTO them** (kubectl's model), which is proven and shipped. Also
> verified: the forward target must bind IPv4 loopback/`0.0.0.0` (IPv6-only
> `[::]` binds are unreachable). The rest of this design (transport, auth, token
> scope, exec) is accurate and implemented.

> ## ⚠️ CORRECTION (operator decision, 2026-07-09) — in-tool SSH key management removed
> railctl **no longer registers, discovers, lists, or manages SSH keys** in any
> way. The user registers their SSH key **once** at
> [railway.com/account/ssh-keys](https://railway.com/account/ssh-keys); `ssh`
> then authenticates with it (agent / `~/.ssh` defaults, or an explicit `-i`
> identity file). The following are **deleted**: the key discovery/registration
> flow, the `ListSSHKeys`/`RegisterSSHKey` API methods and the `SSHKey` type, and
> the `sshx` key-discovery/fingerprint helpers.
>
> **The token-scope constraint dissolves as a result.** The old
> account/workspace-only gate existed *only* because key registration is
> user/workspace-scoped. With registration gone, exec/port-forward authenticate
> by the pre-registered key and use the token **only** to resolve the service
> instance — and `serviceInstance(environmentId, serviceId)` resolves under **any
> token, including a project token** (verified live). So the `IsProjectToken()`
> fail-fast gate is **removed**: exec and port-forward now work with **any**
> token. On an ssh auth failure (publickey/permission), the commands print a
> one-line hint pointing at railway.com/account/ssh-keys. Sections below that
> describe key registration or the project-token gate are superseded by this note.

# Design: `railctl exec` + `railctl port-forward` — SSH transport to Railway service instances

**Date:** 2026-07-09
**Status:** Draft (design) — pending review, then implementation
**Scope:** Two new commands over one shared SSH-transport layer, split across ~2 PRs
**Evidence base:** Reverse-engineered from Railway's open-source Rust CLI
(`railway-cli`, read in full: `src/commands/ssh/native.rs`, `common.rs`, `mod.rs`,
`config.rs`; `src/controllers/ssh/{authentication,keys}.rs`; `src/commands/{connect,sandbox}.rs`;
`src/gql/mutations/strings/{SshPublicKeyCreate,SshPublicKeyDelete,GenerateShellToken}.graphql`;
`src/gql/queries/strings/ServiceInstance.graphql`; `src/client.rs`). File:line citations
below are into that Rust tree.

---

## 1. Executive summary — how Railway actually does it

Railway's `ssh`/`connect`/`sandbox forward` commands **shell out to the user's local
`ssh` binary** (`Command::new("ssh")`, native.rs:41) and dial a **single global relay
host** — `ssh.railway.com` (prod/staging) or `ssh.railway-develop.com:2222` (dev)
(config.rs:335-339). There is **no per-region hostname**; all regions funnel through the
one relay, which routes server-side.

The **SSH username is a routing key**, not a login name. For a service it is the
`serviceInstance.id` (a UUID resolved from `(environmentId, serviceId)`); the relay parses
it to attach the session to the right container. The connection is authenticated purely by
an **SSH public key** the user has pre-registered with Railway (`sshPublicKeyCreate`) —
their existing `~/.ssh` key or ssh-agent key. **The CLI never generates an ephemeral
keypair, and never deletes keys on session end** — registered keys are durable, like an
`authorized_keys` entry (keys.rs:220-258; no Drop/defer cleanup anywhere).

Port-forwarding is plain OpenSSH `-L`, with the **remote side pinned to the literal
`127.0.0.1`** (native.rs:390-396). Crucially, `connect.rs` SSHes **directly into the target
service's own container** (username = that service's instance id) and forwards to *its own*
loopback — it is **not** a jump host reaching `*.railway.internal`. That is the whole trick
for reaching a private, unexposed service: you become the service's container, and
`localhost:<port>` inside it is the service. (Full reasoning in §6.)

**Two mechanisms exist in the Rust CLI that we should NOT conflate.** Only the SSH-binary
path above is in scope. `railway sandbox exec` uses a completely separate raw-WebSocket +
`GenerateShellToken` JWT protocol (`wss://<relay>:2226/ws/exec`, sandbox_exec.rs:82-115) —
we deliberately do **not** adopt that; railctl targets *services*, not Railway sandboxes,
and the SSH path covers both exec and forward.

### What this buys railctl

railctl's own skill doc already flags the exact gap this closes: reaching a datastore that
has **no `tcpProxy`/`domain` block** — "you need to reach a database from your laptop"
(`docs/railctl-skill.md:686`). Today that forces the user to publicly expose the service.
`railctl port-forward` lets them reach it privately, and `railctl exec` gives a
kubectl-style shell into any running service instance.

---

## 2. Answers to the six research questions (evidence-backed)

**Q1 — Transport.** Native `ssh` shell-out, confirmed `Command::new("ssh")` at
native.rs:41 (the *only* such site). Argv shapes:

- Interactive shell: `ssh [-p 2222] [-i <key>] <instance-id>@ssh.railway.com`
  (PTY auto: `-t` if both stdin+stdout are TTYs, `-T` otherwise; native.rs:265-322).
- Exec a command: same, then `ssh <target> <arg0> <arg1> …` — each arg appended
  individually, no local shell-join (native.rs:310-314). ssh's own remote tokenization applies.
- Port-forward: `ssh [-p 2222] [-i <key>] -N -o StrictHostKeyChecking=accept-new -o
  ExitOnForwardFailure=yes -o ServerAliveInterval=30 -o ServerAliveCountMax=3 -L
  127.0.0.1:<local>:127.0.0.1:<remote> [-L …] <target>@ssh.railway.com`
  (native.rs:366-398). No `ProxyCommand`/`ProxyJump`/`IdentitiesOnly`/`UserKnownHostsFile`
  anywhere. Relay host is **environment-tier-scoped, not region-scoped** (config.rs:335-339).

**Q2 — Auth.** No keygen in the CLI at all. It *discovers* existing `~/.ssh/*.pub` and
ssh-agent keys (keys.rs:94-160); if none, it tells the user to run `ssh-keygen -t ed25519`
(native.rs:78-83). Registration is `sshPublicKeyCreate(input:{name, publicKey, workspaceId?})`
(SshPublicKeyCreate.graphql; call site keys.rs:220-239). `workspaceId=null` → personal key;
set → workspace key (needs workspace ADMIN). **No project-level key concept.** There is a
separate short-lived-JWT mutation `generateShellToken` — but it is used **only** by
`sandbox exec`'s WebSocket path (sandbox.rs:1617-1639), passed as a **WebSocket subprotocol
value**, never to `ssh`. Key deletion `sshPublicKeyDelete(id)` is a **manual user command
only** (`railway ssh keys rm`), never automatic — keys persist across sessions.

**Q3 — Target resolution.** Terminal identifier = `serviceInstance.id`, from
`serviceInstance(environmentId, serviceId){ id }` (ServiceInstance.graphql; call site
native.rs:50-65). Takes **no replica index / deployment id** — the server returns one
canonical "first active instance." The only override is a `--deployment-instance <id>` flag
that bypasses the query and is used directly as the ssh username (mod.rs:42-45, 96-98).
No way to pick "replica 2 of 3." **railctl already surfaces `InstanceID` and `DeploymentID`
on `types.ServiceDetail`** (`internal/types/project.go:59,68`) — but that's the *service
instance config* id, which may or may not equal the connectable instance id; treat the
dedicated `serviceInstance` query as the source of truth (see §9 risk R3).

**Q4 — Private-service reach.** `connect.rs` SSHes into the **target service's own
instance** (username = that service's `serviceInstance.id`) and forwards to `127.0.0.1:<port>`
inside it. Not a bastion; not `*.railway.internal`. `local_tunnel_url` rewrites the
service's private `DATABASE_URL` host to `127.0.0.1:<localPort>` while keeping the real
in-container port as the `-L` remote (connect.rs:347-375, test 493-510). This is THE
mechanism for a private apiserver: forward into the apiserver's own container.

**Q5 — Token compatibility.** `railway ssh keys` **hard-refuses project tokens**
(keys.rs:99-105): *"SSH key management is not supported with project tokens (RAILWAY_TOKEN).
Use a workspace API token (RAILWAY_API_TOKEN) or run railway login."* Structural reason:
`SshPublicKeyCreateInput` has `workspaceId?` but **no `projectId`** — keys attach to a
user or workspace, never a project; a project token has no user identity to attach a
personal key to. This maps cleanly onto railctl's existing three-tier token model
(`internal/api/client.go`: `TokenTypeAccount/Workspace/Project`): **key registration needs
Account or Workspace token; a Project token cannot register a key.** (Connecting with an
already-registered key would work under any token that can run the `serviceInstance`
lookup — but auto-registration will fail, so we fail fast; see §7.)

**Q6 — Prerequisites.** Needs the local `ssh` binary (native.rs:41). The target container
does **not** need its own sshd — the relay is Railway's own multiplexing SSH gateway that
brokers the session into arbitrary customer containers (evidence: single global relay for
all services/regions, config.rs:335-339; the CLI runs `apt-get install tmux` *over* the
session implying it lands in a real app-container shell, native.rs:179-207; authors' own
comment frames it as "like docker exec / kubectl exec", native.rs:262-263; the SFTP path
even accepts any host key with a "no idea if Railway pins keys" comment, sftp.rs:167-173).
Host-key verification is TOFU-only (`accept-new`).

---

## 3. Command surface (the UX contract)

Two new top-level commands, mirroring kubectl and Railway's `sandbox` ergonomics.

### `railctl exec`

```
railctl exec <service> [flags] [-- <cmd> [args...]]
```

| Form | Behaviour |
|---|---|
| `railctl exec api -p app -e prod` | Interactive shell (`-it` semantics) into the first active instance of `api`. |
| `railctl exec api -p app -e prod -- ls -la /data` | Run a one-off command, stream stdout/stderr, propagate exit code. |
| `railctl exec api … --deployment-instance <id> -- …` | Target a specific instance id, skipping the `serviceInstance` lookup. |

Flags (beyond the global `-p/-e/-w/--token`): `--deployment-instance <id>` (advanced
override), `-i/--identity-file <path>` (use a specific private key instead of agent/default).
The service is the **positional arg**, so `-s` is intentionally *not* used (consistent with
`logs <service>` at `internal/cmd/logs_service.go:25`). `cobra.MinimumNArgs(1)` +
`SetInterspersed(false)` so everything after `--` is the remote command verbatim.

PTY decision mirrors native.rs:265-270: allocate `-t` when a command is given and both
stdin+stdout are TTYs, or when no command is given and stdin is a TTY; else `-T`. Detect via
`term.IsTerminal(int(os.Stdin.Fd()))` / `os.Stdout.Fd()` (`golang.org/x/term`).

### `railctl port-forward`

```
railctl port-forward <service> [LOCAL:]REMOTE [[LOCAL:]REMOTE ...] [flags]
```

| Form | Behaviour |
|---|---|
| `railctl port-forward db -p app -e prod 5432` | `localhost:5432` → `127.0.0.1:5432` in `db`'s container. |
| `railctl port-forward db … 6543:5432` | `localhost:6543` → `127.0.0.1:5432` in `db`. |
| `railctl port-forward db … 5432 6379` | Multiple ports over **one** SSH connection. |

**Headline capability — private internal target through a jump service.** Because the base
mechanism pins the `-L` remote to the target container's own `127.0.0.1`, reaching a
*different* private host (`kube-apiserver.railway.internal`) requires the extended
`LOCAL:REMOTEHOST:REMOTEPORT` triple form, which we forward through a chosen **jump
service** whose container sits on the environment's private network:

```
railctl port-forward <jump-service> 6443:kube-apiserver.railway.internal:6443 -p app -e prod
```

This emits `-L 127.0.0.1:6443:kube-apiserver.railway.internal:6443` and SSHes into
`<jump-service>`'s instance — the relay resolves the internal DNS name **server-side from
inside that container's netns** (the private mesh is reachable there). This is the exact
private-apiserver answer.

> ⚠️ **Server-side DNS caveat (from native.rs:361-365).** Railway's relay does its own
> resolution and does *not* honor the target container's `/etc/hosts`; `localhost`-style
> names resolve to unreachable mesh addresses. So for the *service's own* port, always emit
> the literal `127.0.0.1` as the remote host (the two-field `LOCAL:REMOTE` form). The
> three-field form is only for genuine `*.railway.internal` DNS names, which the relay
> *does* resolve. We enforce this: a bare `LOCAL:REMOTE` always becomes
> `127.0.0.1:<local>:127.0.0.1:<remote>`; a triple passes the middle host through verbatim.

Flags: `-i/--identity-file`, `--deployment-instance <id>`, `--strict` (fail if a local port
is busy instead of auto-picking a nearby free one — mirrors sandbox.rs `--strict`),
`--address <addr>` (bind address, default `127.0.0.1`). Positional port specs are
`required=true`, repeated.

Port-spec grammar (`parsePortSpec`):
- `REMOTE` → `{local:REMOTE, host:"127.0.0.1", remote:REMOTE}`
- `LOCAL:REMOTE` → `{local:LOCAL, host:"127.0.0.1", remote:REMOTE}`
- `LOCAL:HOST:REMOTE` → `{local:LOCAL, host:HOST, remote:REMOTE}` (HOST used verbatim)

---

## 4. Package layout (mirrors existing railctl conventions)

| File | Responsibility |
|---|---|
| `internal/api/ssh_keys.go` (new) | `SSHPublicKey` type; `RegisterSSHKey`, `ListSSHKeys`, `DeleteSSHKey` client methods + GraphQL const strings. |
| `internal/api/instances.go` (new) | `GetServiceInstanceID(environmentID, serviceID) (string, error)` + `serviceInstance` query. |
| `internal/api/interface.go` (modify) | Add the four methods to `APIClient`. |
| `internal/api/mock.go` (modify) | Add `…Func` fields + stubs. |
| `internal/sshx/sshx.go` (new) | Transport layer: relay resolution, argv construction, key discovery, `ssh` shell-out, signal handling, `ForwardGuard`. **No railctl-API imports** — pure `os/exec` + local key logic, unit-testable in isolation. |
| `internal/sshx/ports.go` (new) | `parsePortSpec`, free-port selection, `PortForward` struct. |
| `internal/cmd/exec.go` (new) | `railctl exec` cobra command. |
| `internal/cmd/port_forward.go` (new) | `railctl port-forward` cobra command. |
| `internal/cmd/exec_test.go`, `port_forward_test.go`, `internal/sshx/*_test.go`, `internal/api/*_test.go` (new) | tests per §8. |
| `README.md` (modify) | new "Exec & Port-forward" section. |

Rationale for a dedicated `internal/sshx` package: the transport is the risky, ssh-shaped
part; isolating it keeps argv/key/port logic unit-testable without any Railway API and
keeps the two cobra commands thin (resolve context → build spec → hand to `sshx`).

---

## 5. API layer — method signatures & GraphQL

Follows the exact pattern of `internal/api/deployments.go` (const query string → `c.execute`
→ unmarshal into a private response struct).

```go
// internal/api/instances.go
const serviceInstanceQuery = `
query ServiceInstance($environmentId: String!, $serviceId: String!) {
    serviceInstance(environmentId: $environmentId, serviceId: $serviceId) { id }
}`

// GetServiceInstanceID resolves (environment, service) → the connectable instance id
// used as the SSH username. Returns a friendly error if no active instance exists.
func (c *Client) GetServiceInstanceID(environmentID, serviceID string) (string, error)
```

```go
// internal/api/ssh_keys.go
type SSHPublicKey struct {
    ID          string
    Name        string
    Fingerprint string
    WorkspaceID string // "" for a personal key
}

const sshPublicKeyCreateMutation = `
mutation SshPublicKeyCreate($input: SshPublicKeyCreateInput!) {
    sshPublicKeyCreate(input: $input) { id name fingerprint workspaceId }
}`

const sshPublicKeyDeleteMutation = `
mutation SshPublicKeyDelete($id: String!) { sshPublicKeyDelete(id: $id) }`

// RegisterSSHKey registers a public key (workspaceID "" → personal key).
// Fails at the API layer for project tokens (no user identity); callers should
// fail fast before reaching here (see §7).
func (c *Client) RegisterSSHKey(name, publicKey, workspaceID string) (SSHPublicKey, error)

// ListSSHKeys returns keys already registered for the token's identity, so an
// already-registered key can be reused without a duplicate create.
func (c *Client) ListSSHKeys() ([]SSHPublicKey, error) // uses an SshPublicKeys-style query

// DeleteSSHKey removes a key by id (used only by an explicit cleanup path, not per-session).
func (c *Client) DeleteSSHKey(id string) error
```

> Verify-live TODO before coding: confirm the read query name/shape for listing keys
> (the Rust CLI's `get_registered_ssh_keys` — find its `.graphql` and mirror it). The three
> mutation bodies above are quoted verbatim from the Rust tree and are trusted.

---

## 6. The transport layer (`internal/sshx`)

### 6.1 Relay resolution

```go
// Mirrors Configs::get_ssh_relay (config.rs:335-339). Environment-tier scoped, NOT region.
func relayHostPort() (host string, port int) {
    // railctl targets prod backboard (api.client.DefaultAPIURL = backboard.railway.com),
    // so the prod relay is correct. Keep an override hook for parity/testing:
    if h := os.Getenv("RAILWAY_SSH_HOST"); h != "" { /* parse host[:port] */ }
    return "ssh.railway.com", 22
}
```

railctl only ever talks to `backboard.railway.com` (`internal/api/client.go:20`), so the
prod relay (`ssh.railway.com:22`) is the only tier we need; the dev relay is a Railway-internal
concern. Expose `RAILWAY_SSH_HOST` purely as an escape hatch (and for tests).

### 6.2 Argv construction (the core — mirror native.rs exactly)

```go
type PortForward struct{ LocalPort int; RemoteHost string; RemotePort int } // RemoteHost defaults "127.0.0.1"

func baseArgs(instanceID string, identityFile string) (args []string, target string) {
    host, port := relayHostPort()
    if port != 22 { args = append(args, "-p", strconv.Itoa(port)) }
    if identityFile != "" { args = append(args, "-i", identityFile) }
    return args, instanceID + "@" + host
}

// Interactive / exec — native.rs:271-322
func execArgs(instanceID, identityFile string, cmd []string, wantTTY bool) []string {
    args, target := baseArgs(instanceID, identityFile)
    if wantTTY { args = append(args, "-t") } else { args = append(args, "-T") }
    args = append(args, target)
    return append(args, cmd...) // cmd appended verbatim, no local join (native.rs:310-314)
}

// Forward — native.rs:366-398
func forwardArgs(instanceID, identityFile string, fwds []PortForward, addr string) []string {
    args, target := baseArgs(instanceID, identityFile)
    args = append(args,
        "-N",
        "-o", "StrictHostKeyChecking=accept-new",
        "-o", "ExitOnForwardFailure=yes",
        "-o", "ServerAliveInterval=30",
        "-o", "ServerAliveCountMax=3",
    )
    for _, f := range fwds {
        host := f.RemoteHost; if host == "" { host = "127.0.0.1" }
        args = append(args, "-L",
            fmt.Sprintf("%s:%d:%s:%d", addr, f.LocalPort, host, f.RemotePort))
    }
    return append(args, target)
}
```

These byte-for-byte reproduce the Rust argv (options, order, and the `127.0.0.1` remote-host
default). Keep the exact `-o` set — each has a documented reason (native.rs:369-374):
`accept-new` because `-N` closes stdin so no interactive host-key prompt is possible;
`ExitOnForwardFailure=yes` to fail loudly on a bound-port clash; the two `ServerAlive*` to
detect a dead relay in ~90s.

### 6.3 Key discovery & registration (mirror keys.rs, no keygen)

`ensureIdentity()`:
1. If `-i/--identity-file` given → use it; derive its `.pub`; ensure that pubkey is
   registered (list → register-if-absent). Return the private key path for `-i`.
2. Else discover candidates: ssh-agent keys, then `~/.ssh/{id_ed25519,id_ecdsa,id_rsa}.pub`
   in the same preference order as Rust `SUPPORTED_KEY_TYPES` (keys.rs:56-63). Ensure at
   least one is registered; prefer letting the agent present it (no `-i`) when available.
3. If **no local key exists at all** → error with the actionable message
   (`No SSH key found. Generate one with: ssh-keygen -t ed25519, then retry.`),
   mirroring native.rs:78-83. **We do NOT generate a key** — matches Railway's own model and
   avoids surprising the user with keys they didn't create.

Registration idempotency: fingerprint the local pubkey (SHA256) and skip `RegisterSSHKey`
if a matching fingerprint is already in `ListSSHKeys()`. Key name: `railctl@<hostname>` so
the user can recognize/remove it later.

**No automatic key deletion.** Matches Railway (keys.rs has no Drop/defer cleanup). A future
`railctl ssh-keys` management group can expose `DeleteSSHKey` explicitly; out of scope here.

### 6.4 Process exec & signal handling

- Interactive/exec: `cmd := exec.Command("ssh", args...)`; wire `Stdin/Stdout/Stderr =
  os.Stdin/os.Stdout/os.Stderr` (inherit, like native.rs:316-318). Do **not** intercept
  SIGINT — with a PTY (`-t`), Ctrl+C must reach the remote process, so let `ssh` own the
  terminal; just `cmd.Run()` and propagate `exec.ExitError.ExitCode()` (mirrors
  `run_native_ssh` returning `status.code()`).
- Blocking forward (foreground `railctl port-forward`): `Stdin=null, Stdout=null,
  Stderr=inherit` (native.rs:350-356). Install a SIGINT/SIGTERM handler (pattern already in
  `internal/cmd/logs_service.go:164-170`) that kills the ssh child and returns cleanly — a
  `ForwardGuard`-style `defer cmd.Process.Kill()` (native.rs:404-413).
- Print a human-readable banner to **stderr** before blocking (`Forwarding 127.0.0.1:6443 ->
  kube-apiserver.railway.internal:6443 via <service> (Ctrl+C to stop)`), keeping stdout
  clean — consistent with railctl's stdout/stderr discipline (see token-minting design).

### 6.5 Port parsing & free-port fallback (`ports.go`)

`parsePortSpec(s string) (PortForward, error)` per the grammar in §3. When the local port is
busy and `--strict` is off, probe upward for a free port (`net.Listen("tcp", addr:0)` or
sequential try) and warn on stderr which local port was actually chosen (mirrors
sandbox.rs:1779-1801 behaviour). With `--strict`, error out.

---

## 7. Token-scope handling (fail-fast)

railctl already classifies tokens (`client.IsProjectToken()`, `IsWorkspaceToken()`,
`internal/api/client.go`). Gate key *registration*, not connection:

```go
isProj, err := client.IsProjectToken()
if err != nil { return err }
if isProj {
    // Only blocks if we'd need to REGISTER a key. If a matching key is already
    // registered under the user's account, connection still works. But a project
    // token can't even list personal keys, so treat it as unsupported and fail fast:
    return fmt.Errorf(
        "railctl exec/port-forward needs to register an SSH key, which a project token cannot do " +
        "(SSH keys attach to a user or workspace, never a project). " +
        "Use an account or workspace token (RAILWAY_TOKEN=<account/workspace token>), or run the " +
        "command once with such a token to register your key.")
}
```

This mirrors the Rust guard verbatim in intent (keys.rs:99-105) and matches railctl's
established "contradiction/out-of-scope → fail fast, never warn-and-proceed" doctrine
(`docs/token-capability-matrix.md` §"railctl flag semantics"). For a workspace-scoped key
(shared CI use), pass `workspaceID` from `client.GetWorkspaceID()`; for an account token,
register a personal key (`workspaceID=""`).

Add a row to `docs/token-capability-matrix.md`:
`| exec / port-forward (SSH) | ✅ (personal key) | ✅ (workspace key) | ❌ cannot register key | — |`.

---

## 8. Test strategy

The interactive transport can't be exercised in CI, so split by testability (railctl's Tier
1/2/3 model, `docs/testing-architecture.md`):

**Tier 1 — unit (the bulk of the value, fully deterministic):**
- `sshx.execArgs` / `forwardArgs`: table-driven, assert exact argv slices for interactive,
  exec, single/multi-forward, and the `LOCAL:HOST:REMOTE` triple. This is the highest-value
  test — it locks the argv contract against native.rs.
- `parsePortSpec`: `REMOTE`, `LOCAL:REMOTE`, `LOCAL:HOST:REMOTE`, bad inputs; assert the
  `127.0.0.1` default is injected for the two-field form and the host passes through for the
  triple.
- `relayHostPort` incl. `RAILWAY_SSH_HOST` override parsing.
- Key selection/fingerprint + idempotency logic (feed fake `ListSSHKeys` results, assert
  register-or-skip decision) — inject the key-discovery filesystem via an interface or a temp
  `$HOME`.
- PTY decision function (`wantTTY(cmd, stdinTTY, stdoutTTY)`) against the native.rs:265-270
  truth table.

**Tier 2 — integration (cobra + mock `APIClient`):**
- `exec`/`port-forward` command wiring: resolve context via `MockClient`, assert the correct
  `GetServiceInstanceID` call and that a project token yields the fail-fast error before any
  ssh spawn. Inject a fake "ssh runner" (an interface `sshx.Runner` with a real impl and a
  recording test impl) so the command's happy path can be asserted (args passed, exit code
  propagated) **without launching ssh**.
- Token-scope gating: project token → §7 error; account/workspace token → proceeds to runner.

**Tier 3 — e2e / manual (documented, not automated in the standard gate):**
- Real `railctl exec <svc> -- echo ok` and `railctl port-forward <db> 5432` against a live
  Railway project with a registered key. Add to the `project`-tier e2e harness as an
  **opt-in** test (needs a live service + a real local ssh key + relay reachability — gate
  behind an env flag like the existing scope-gated groups). The private-internal triple form
  (`6443:kube-apiserver.railway.internal:6443` via a jump service) is manual-verify only.

Design the `sshx.Runner` seam explicitly so Tier 2 never shells out — this is the single most
important testability decision.

---

## 9. Scope cut & PR plan

**PR 1 — `feat/ssh-transport` (foundation + exec):**
`internal/sshx` (argv, key discovery, runner, signals), `internal/api/{instances,ssh_keys}.go`,
interface/mock, `railctl exec` (interactive + command), token-scope gating, Tier 1/2 tests,
README. Headline: kubectl-style shell into a service.

**PR 2 — `feat/port-forward`:**
`internal/sshx/ports.go`, `ForwardGuard`, `railctl port-forward` (multi-port, free-port
fallback, `--strict`), and **the headline `LOCAL:HOST:REMOTE` private-internal-target form**
(the private-apiserver capability). Tier 1/2 tests + documented manual e2e. Update the
token-capability matrix.

Splitting keeps PR 1 free of the trickier port grammar/reconnect logic while still delivering
a usable command; PR 2 is where the private-network headline lands.

---

## 10. Honest hard parts & risks

- **R1 — Instance-id semantics.** `serviceInstance(environmentId, serviceId).id` returns one
  "first active" instance with no replica selection (native.rs, mod.rs:42-45). For a
  multi-replica service, exec/forward hits an arbitrary replica. Acceptable for v1
  (kubectl's default is similar); document it. `--deployment-instance <id>` is the escape
  hatch. **Verify live** what that id maps to before trusting `types.ServiceDetail.InstanceID`
  as a substitute for the dedicated query.
- **R2 — Key registration is a side effect with lasting state.** Unlike an ephemeral-cert
  model, we leave a durable key in the user's Railway account (matches Railway's own CLI).
  Mitigate: idempotent register (skip if fingerprint present), a recognizable key name
  (`railctl@host`), and a documented `railctl ssh-keys rm` follow-up (future). Call this out
  in `--help` and README so it isn't a surprise.
- **R3 — Project tokens are common in railctl's world.** railctl heavily uses project tokens
  (the whole token-minting feature, the e2e project tier). exec/port-forward simply **can't**
  work under them for key registration. This is a real ergonomic wall — the fail-fast message
  must be excellent and point at the exact fix. Flagged as the single biggest adoption caveat.
- **R4 — Relay host-key TOFU.** We inherit Railway's `accept-new`/no-pinning posture
  (native.rs:369-374, sftp.rs:167-173). A MITM on first connect to `ssh.railway.com` is
  theoretically possible; we trust DNS+the relay exactly as Railway's own CLI does. Note it,
  don't try to out-engineer upstream.
- **R5 — Server-side DNS foot-gun.** If a user writes `port-forward db 5432:localhost:5432`
  expecting the service's loopback, the relay resolves `localhost` to an unreachable mesh
  address (native.rs:361-365). Our parser must inject the literal `127.0.0.1` for the
  two-field form and reserve the middle field strictly for real `*.railway.internal` names;
  document that `localhost` in the triple form is wrong.
- **R6 — Windows / no-ssh-binary environments.** We hard-depend on a local `ssh`. Detect its
  absence (`exec.LookPath("ssh")`) and error clearly. Pure-Go SSH (e.g. `golang.org/x/crypto/ssh`)
  is a *possible* future to remove the dependency, but it would mean reimplementing agent
  handshake, `-L` channel multiplexing, PTY, and known_hosts — a large surface. **Out of
  scope; shell-out is the pragmatic v1**, exactly as Rust does.
- **R7 — `-o SetEnv` durable-session parity.** Railway's tmux/durable-resume features
  (native.rs:282-293, 215-251) are explicitly **out of scope** — nice-to-have, not needed for
  the exec/forward core. Don't build them in v1.

---

## 11. Explicitly NOT doing (v1)

`generateShellToken`/WebSocket `sandbox exec` transport (different protocol, sandbox-only);
Railway sandboxes at all; durable/tmux sessions; automatic key cleanup; replica selection
beyond `--deployment-instance`; the dev relay tier; pure-Go SSH client.
