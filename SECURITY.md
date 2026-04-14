# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability in `asobi-cli`, please report it
**privately** so we can fix it before it is publicly disclosed.

**Do not open a public GitHub issue for security issues.**

### How to report

Either of these channels work:

- **GitHub Security Advisory (preferred):**
  [Report privately](https://github.com/widgrensit/asobi-cli/security/advisories/new)
- **Email:** security@asobi.dev

### What to expect

- Acknowledgement within **48 hours**
- Initial assessment within **7 days**
- Coordinated disclosure timeline agreed with you
- Credit in the security advisory if you want it

## Supported versions

| Version | Supported |
|---------|-----------|
| latest stable | ✅ |
| older releases | ❌ — please upgrade |

## Scope

**In scope:**
- The `asobi-cli` Go binary (this repository)
- Stored credential format and the device-code login flow

**Out of scope:**
- The hosted asobi.dev SaaS — see https://asobi.dev/security
- Third-party Go dependencies — please report upstream

## Credential storage

`asobi-cli` stores credentials at `~/.asobi/credentials.json` with mode
`0600`, in a directory with mode `0700`. The CLI does not transmit
credentials over unencrypted channels and uses ECDH+AES-GCM for the
initial device-code login exchange.
