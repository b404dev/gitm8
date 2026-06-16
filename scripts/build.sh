#!/usr/bin/env bash
set -euo pipefail

# Build gitm8 from the local working tree and install it into the user bin dir.
# Unlike install.sh (which clones a remote ref), this compiles whatever is
# currently checked out, so it is the script to use while developing.

INSTALL_DIR="${GITM8_INSTALL_DIR:-$HOME/.local/bin}"
MAN_DIR="${GITM8_MAN_DIR:-$HOME/.local/share/man/man1}"
BIN_NAME="${GITM8_BIN_NAME:-gitm8}"

# Resolve the repo root from this script's location so it works from any cwd.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

log() {
  printf 'gitm8 build: %s\n' "$*"
}

warn() {
  printf 'gitm8 build warning: %s\n' "$*" >&2
}

die() {
  printf 'gitm8 build error: %s\n' "$*" >&2
  exit 1
}

have() {
  command -v "$1" >/dev/null 2>&1
}

have go || die "go is required"

cd "$REPO_ROOT"

if [ "${GITM8_SKIP_CHECKS:-}" = "1" ]; then
  log "skipping vet and tests because GITM8_SKIP_CHECKS=1"
else
  log "running go vet"
  go vet ./...
  log "running go test"
  go test ./...
fi

# Build the binary into the repo root (gitignored) for quick local runs.
log "building $BIN_NAME"
go build -o "$REPO_ROOT/$BIN_NAME" .

# Install it into the user bin dir.
mkdir -p "$INSTALL_DIR"
install -m 0755 "$REPO_ROOT/$BIN_NAME" "$INSTALL_DIR/$BIN_NAME"
log "installed $INSTALL_DIR/$BIN_NAME"

if [ "${GITM8_SKIP_MAN_INSTALL:-}" = "1" ]; then
  log "skipping man page install because GITM8_SKIP_MAN_INSTALL=1"
elif [ -f "$REPO_ROOT/docs/man/gitm8.1" ]; then
  mkdir -p "$MAN_DIR"
  install -m 0644 "$REPO_ROOT/docs/man/gitm8.1" "$MAN_DIR/$BIN_NAME.1"
  log "installed $MAN_DIR/$BIN_NAME.1"
else
  warn "manual page not found at docs/man/gitm8.1; skipping man page install"
fi

case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) warn "$INSTALL_DIR is not on PATH; add it with: export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac

log "done"
