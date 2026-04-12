# asobi-cli

Command-line tool for building and deploying multiplayer games on [Asobi](https://github.com/widgrensit/asobi).

## Install

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
# Authenticate via browser — opens the dashboard, you pick tenant/game/env
asobi login

# Deploy Lua scripts to the engine
asobi deploy game/

# Check engine health
asobi health

# See current session info
asobi whoami
```

### Self-hosted

If you run your own engine without the hosted dashboard:

```bash
asobi config set url https://your-engine.example.com
asobi config set api_key ak_your_key_here
asobi deploy game/
```

## Commands

| Command | Description |
|---|---|
| `asobi login` | Authenticate via browser (ECDH device-code flow) |
| `asobi logout` | Clear stored credentials |
| `asobi whoami` | Show current session info |
| `asobi deploy <dir>` | Deploy Lua scripts to the engine |
| `asobi health` | Check engine health |
| `asobi config set <k> <v>` | Set manual config (`url`, `api_key`) |
| `asobi config show` | Show current config |

### Login options

```
asobi login [--saas-url <url>] [--token-name <name>]
```

- `--saas-url` — Dashboard URL (default: `https://app.asobi.dev`). Self-hosters point this at their own dashboard.
- `--token-name` — Name for this CLI session (default: hostname). Appears in the dashboard for identification.

## How login works

`asobi login` uses an ECDH-encrypted device-code flow:

1. The CLI generates an ephemeral P-256 keypair and sends the public key to the dashboard.
2. A browser opens to the dashboard's approval page, where you pick a tenant, game, and environment.
3. On approval, the dashboard encrypts your CLI credentials (access token + refresh token) with AES-256-GCM using a shared secret derived from the ECDH key exchange.
4. The CLI polls until approval, decrypts the payload locally, and stores the credentials.

This design means the credentials never travel in plaintext over the polling channel — even a passive observer on the network between the CLI and the dashboard cannot read them.

### Credential storage

Credentials are stored in `~/.asobi/credentials.json` with `0600` permissions (owner read/write only).

The `ASOBI_ACCESS_TOKEN` environment variable overrides the stored access token, which is useful for CI pipelines.

## How deploy works

When credentials are present (from `asobi login`):

1. The CLI mints a **1-hour ephemeral engine API key** via the dashboard (`POST /internal/cli/mint-key`), authenticated with the stored access token.
2. If the access token has expired, the CLI auto-refreshes it from the refresh token (bound to the device fingerprint from login).
3. The ephemeral key is used for the actual deploy call to the engine.

This means a compromised credential file has limited blast radius — the access token can mint deploy keys but expires in 24 hours, and the refresh token is bound to the device it was issued on.

When no credentials are present, the CLI falls back to the manual `api_key` from `asobi config set` — backwards compatible for self-hosted setups.

## Configuration

| File | Purpose |
|---|---|
| `~/.asobi/config.json` | Manual config (engine URL, API key) — self-hosted fallback |
| `~/.asobi/credentials.json` | Login credentials (access/refresh tokens, tenant context) |

Credentials take precedence over manual config when both exist.

## Security

- ECDH P-256 key agreement + HKDF-SHA256 + AES-256-GCM for token transport
- Credentials file at `0600` permissions
- Access tokens: 24-hour lifetime
- Refresh tokens: 30-day lifetime, bound to device fingerprint
- Ephemeral engine keys: 1-hour lifetime, tagged with `source=cli_deploy`
- Default deploy scope: `[deploy]` only (least privilege)
- No `--insecure` flag — TLS is always required for non-localhost URLs

## License

[Apache-2.0](LICENSE)
