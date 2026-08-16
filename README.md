# asobi-cli

Command-line tool for building and deploying multiplayer games on [Asobi](https://github.com/widgrensit/asobi).

## Install

**Linux / macOS:**

```bash
curl -fsSL https://raw.githubusercontent.com/widgrensit/asobi-cli/main/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/widgrensit/asobi-cli/main/install.ps1 | iex
```

Or with a package manager:

```powershell
winget install widgrensit.asobi
# or
scoop bucket add asobi https://github.com/widgrensit/scoop-bucket
scoop install asobi
```

The scripts download the latest release binary for your OS and architecture
(amd64 and arm64), verify its checksum, and install it to `~/.local/bin` on
Linux/macOS or `%LOCALAPPDATA%\asobi\bin` on Windows (override with
`ASOBI_INSTALL_DIR`). Pin a version with `ASOBI_VERSION=v0.1.0`.

Verify the install:

```bash
asobi version
```

**From source (requires Go 1.26+):**

```bash
git clone https://github.com/widgrensit/asobi-cli
cd asobi-cli
go build -o bin/asobi ./cmd/asobi
ln -s $(pwd)/bin/asobi ~/bin/asobi
```

## Quick start

### Hosted (asobi.dev)

```bash
# Authenticate via browser - opens the dashboard, you pick tenant/game/env
asobi login

# Create an environment and deploy Lua scripts to it
asobi create prod
asobi deploy prod game/

# Check engine health
asobi health

# See current session info
asobi whoami
```

### Self-hosted

If you run your own engine without the hosted dashboard, skip `asobi login`
and point the CLI at your engine directly:

```bash
asobi config set url https://your-engine.example.com
asobi config set api_key ak_your_key_here
asobi deploy prod game/
```

## Commands

| Command | Description |
|---|---|
| `asobi login` | Authenticate via browser (ECDH device-code flow) |
| `asobi logout` | Clear stored credentials |
| `asobi whoami` | Show current session info (including the active game) |
| `asobi init [dir]` | Scaffold a minimal working starter Lua game |
| `asobi init [dir] --template <name>` | Scaffold a genre starter (`arena`, `chat`, `turn-based`, `world`) or fetch a full demo (`defold`, `godot`, `unity`, `backend`) |
| `asobi games` | List your tenant's games (marks the active one) |
| `asobi use <slug>` | Set the active game (persisted in `~/.asobi`) |
| `asobi create <name> [--size xs\|s\|m\|l] [--game <slug>]` | Create an environment |
| `asobi deploy <env-name> [dir] [--game <slug>]` | Deploy Lua scripts to an environment (`dir` defaults to `.`) |
| `asobi stop <name> [--game <slug>]` | Stop a running environment |
| `asobi start <name> [--game <slug>]` | Start a stopped environment |
| `asobi resize <name> --size <xs\|s\|m\|l> [--game <slug>]` | Resize an environment |
| `asobi retention <name> --after <never\|7\|30\|90\|365> [--game <slug>]` | How long the environment keeps unclaimed guest accounts. Owner or admin only |
| `asobi delete <name> [--game <slug>]` | Delete an environment |
| `asobi destroy <env_id>` | Delete by env_id and revoke its keys (idempotent; used by CI cleanup) |
| `asobi envs [--game <slug>]` | List your environments |
| `asobi env list [--ephemeral] [--json]` | Structured environment list for scripting |
| `asobi health` | Check engine health |
| `asobi config set <k> <v>` | Set manual config (`url`, `api_key`) |
| `asobi config show` | Show current config |

## Selecting a game

A CLI token is tenant-scoped and every environment belongs to a game, so env
operations need a game. The effective game resolves in this order:

1. an explicit `--game <slug>` flag
2. the active game set with `asobi use <slug>` (stored as `active_game` in `~/.asobi/credentials.json`)
3. if neither is set: with a single game it is auto-selected; with several games and an interactive terminal the CLI prompts; otherwise it errors.

```bash
asobi games            # list games, active one marked with *
asobi use arena        # set the active game
asobi create prod      # uses the active game
asobi deploy prod lua --game ctf   # override for one command
```

## Starting a new game

```bash
asobi init mygame      # scaffold lua/match.lua + README.md
cd mygame
asobi dev              # run it locally on :8084 (Docker, no login)
asobi login
asobi use <game>
asobi deploy prod lua
```

Pick a genre starter instead of the plain scaffold - each writes a runnable
`lua/match.lua` shaped for that style of game:

```bash
asobi init mygame --template arena       # real-time movement + combat
asobi init mygame --template chat        # a real-time chat room
asobi init mygame --template turn-based  # server-enforced turn order
asobi init mygame --template world       # a persistent shared world
```

To see a full client + backend project instead, use a demo template
(`defold`, `godot`, `unity`, or `backend`) - these fetch a pinned repo:

```bash
asobi init mygame --template defold
```

## Throwaway envs (CI)

For CI integration tests, create a uniquely-named env, deploy to it, and destroy
it in a trap so cleanup runs even on failure. `asobi env list --json` gives you
the env id to destroy:

```bash
NAME="ci-$GITHUB_RUN_ID"
asobi create "$NAME"
asobi deploy "$NAME" lua

ENV_ID=$(asobi env list --json | jq -r ".[] | select(.name==\"$NAME\") | .id")
trap "asobi destroy $ENV_ID" EXIT

# ... run tests against the env ...
```

`asobi destroy <env_id>` is idempotent and revokes the env's keys, so a repeated
or racing cleanup is safe.

### Login options

```
asobi login [--saas-url <url>] [--token-name <name>]
```

- `--saas-url` - Dashboard URL (default: `https://console.asobi.dev`). Self-hosters point this at their own dashboard.
- `--token-name` - Name for this CLI session (default: hostname). Appears in the dashboard for identification.

## How login works

`asobi login` uses an ECDH-encrypted device-code flow:

1. The CLI generates an ephemeral P-256 keypair and sends the public key to the dashboard.
2. A browser opens to the dashboard's approval page, where you pick a tenant, game, and environment.
3. On approval, the dashboard encrypts your CLI credentials (access token + refresh token) with AES-256-GCM using a shared secret derived from the ECDH key exchange.
4. The CLI polls until approval, decrypts the payload locally, and stores the credentials.

This design means the credentials never travel in plaintext over the polling channel - even a passive observer on the network between the CLI and the dashboard cannot read them.

### Credential storage

Credentials are stored in `~/.asobi/credentials.json` with `0600` permissions (owner read/write only).

The `ASOBI_ACCESS_TOKEN` environment variable overrides the stored access token, which is useful for CI pipelines.

## How deploy works

When credentials are present (from `asobi login`):

1. The CLI zips the `.lua` files and posts the bundle to the control plane, authenticated with the stored access token (`Authorization: Bearer <access_token>`).
2. If the access token has expired, the CLI auto-refreshes it from the refresh token (bound to the device fingerprint from login) and retries the deploy.

This means a compromised credential file has limited blast radius - the access token is short-lived, and the refresh token is bound to the device it was issued on.

When no credentials are present, the CLI falls back to the manual `api_key` from `asobi config set` - backwards compatible for self-hosted setups.

## Configuration

| File | Purpose |
|---|---|
| `~/.asobi/config.json` | Manual config (engine URL, API key) - self-hosted fallback |
| `~/.asobi/credentials.json` | Login credentials (access/refresh tokens, tenant context) |

Credentials take precedence over manual config when both exist.

## Security

- ECDH P-256 key agreement + HKDF-SHA256 + AES-256-GCM for token transport
- Credentials file at `0600` permissions
- Access tokens: 24-hour lifetime
- Refresh tokens: 30-day lifetime, bound to device fingerprint
- Ephemeral engine keys: 1-hour lifetime, tagged with `source=cli_deploy`
- Default deploy scope: `[deploy]` only (least privilege)
- No `--insecure` flag - TLS is always required for non-localhost URLs

## License

[Apache-2.0](LICENSE)
