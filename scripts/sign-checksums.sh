#!/usr/bin/env bash
# Signs a file with the release ed25519 key (raw signature, verifiable by Go's
# crypto/ed25519). Invoked by goreleaser `signs`. The key arrives as the
# ASOBI_SIGNING_KEY env var (a PEM private key) and is written to a temp file
# that is removed on exit.
set -euo pipefail

in="$1"
out="$2"

if [ -z "${ASOBI_SIGNING_KEY:-}" ]; then
	echo "sign-checksums: ASOBI_SIGNING_KEY is not set" >&2
	exit 1
fi

key="$(mktemp)"
trap 'rm -f "$key"' EXIT
printf '%s' "$ASOBI_SIGNING_KEY" >"$key"

openssl pkeyutl -sign -inkey "$key" -rawin -in "$in" -out "$out"
