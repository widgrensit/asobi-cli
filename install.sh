#!/bin/sh
# Asobi CLI installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/widgrensit/asobi-cli/main/install.sh | sh
#
# Environment variables:
#   ASOBI_VERSION       Install a specific tag (e.g. v0.1.0). Default: latest release.
#   ASOBI_INSTALL_DIR   Install directory. Default: $HOME/.local/bin.

set -eu

REPO="widgrensit/asobi-cli"
INSTALL_DIR="${ASOBI_INSTALL_DIR:-$HOME/.local/bin}"

log() {
	printf '%s\n' "$*"
}

err() {
	printf 'error: %s\n' "$*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || err "required command not found: $1"
}

detect_os() {
	os=$(uname -s)
	case "$os" in
		Linux) echo linux ;;
		Darwin) echo darwin ;;
		MINGW* | MSYS* | CYGWIN* | Windows_NT)
			err "Windows is not supported by this installer. Download the zip from https://github.com/$REPO/releases" ;;
		*) err "unsupported operating system: $os" ;;
	esac
}

detect_arch() {
	arch=$(uname -m)
	case "$arch" in
		x86_64 | amd64) echo amd64 ;;
		aarch64 | arm64) echo arm64 ;;
		*) err "unsupported architecture: $arch" ;;
	esac
}

latest_version() {
	need curl
	curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
		| grep '"tag_name":' \
		| head -n 1 \
		| sed -e 's/.*"tag_name":[[:space:]]*"//' -e 's/".*//'
}

verify_checksum() {
	file="$1"
	sums="$2"
	name="$3"
	expected=$(grep " $name\$" "$sums" | awk '{print $1}')
	[ -n "$expected" ] || err "no checksum found for $name in checksums.txt"
	if command -v sha256sum >/dev/null 2>&1; then
		actual=$(sha256sum "$file" | awk '{print $1}')
	elif command -v shasum >/dev/null 2>&1; then
		actual=$(shasum -a 256 "$file" | awk '{print $1}')
	else
		err "need sha256sum or shasum to verify the download"
	fi
	[ "$expected" = "$actual" ] || err "checksum mismatch for $name (expected $expected, got $actual)"
}

main() {
	need curl
	need tar

	os=$(detect_os)
	arch=$(detect_arch)

	version="${ASOBI_VERSION:-}"
	if [ -z "$version" ]; then
		version=$(latest_version)
	fi
	[ -n "$version" ] || err "could not resolve a release version"

	asset="asobi_${os}_${arch}.tar.gz"
	base="https://github.com/$REPO/releases/download/$version"

	tmp=$(mktemp -d 2>/dev/null || mktemp -d -t asobi)
	trap 'rm -rf "$tmp"' EXIT INT TERM

	log "Downloading asobi $version ($os/$arch)..."
	curl -fsSL -o "$tmp/$asset" "$base/$asset" \
		|| err "failed to download $asset - check that $version has a $os/$arch build"
	curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt" \
		|| err "failed to download checksums.txt"

	log "Verifying checksum..."
	verify_checksum "$tmp/$asset" "$tmp/checksums.txt" "$asset"

	tar -xzf "$tmp/$asset" -C "$tmp" asobi \
		|| err "failed to extract asobi from $asset"

	mkdir -p "$INSTALL_DIR"
	install_path="$INSTALL_DIR/asobi"
	mv "$tmp/asobi" "$install_path"
	chmod +x "$install_path"

	log "Installed asobi to $install_path"

	case ":$PATH:" in
		*":$INSTALL_DIR:"*)
			log "Run 'asobi version' to get started." ;;
		*)
			log ""
			log "$INSTALL_DIR is not on your PATH. Add it with:"
			log "  export PATH=\"$INSTALL_DIR:\$PATH\""
			log "Then run 'asobi version' to get started." ;;
	esac
}

main "$@"
