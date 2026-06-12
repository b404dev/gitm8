#!/usr/bin/env bash
set -euo pipefail

REPO_URL="${GITM8_REPO_URL:-https://github.com/b404dev/gitm8.git}"
REF="${GITM8_REF:-main}"
INSTALL_DIR="${GITM8_INSTALL_DIR:-$HOME/.local/bin}"
MAN_DIR="${GITM8_MAN_DIR:-$HOME/.local/share/man/man1}"
BIN_NAME="${GITM8_BIN_NAME:-gitm8}"

log() {
  printf 'gitm8 install: %s\n' "$*"
}

warn() {
  printf 'gitm8 install warning: %s\n' "$*" >&2
}

die() {
  printf 'gitm8 install error: %s\n' "$*" >&2
  exit 1
}

have() {
  command -v "$1" >/dev/null 2>&1
}

sudo_cmd() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif have sudo; then
    sudo "$@"
  else
    die "sudo is required to install packages with this package manager"
  fi
}

detect_pm() {
  if have apt-get; then
    printf 'apt\n'
  elif have pacman; then
    printf 'pacman\n'
  elif have brew; then
    printf 'brew\n'
  else
    die "supported package manager not found; expected apt, pacman, or brew"
  fi
}

install_apt_gh_repo() {
  if have gh; then
    return
  fi
  sudo_cmd mkdir -p /etc/apt/keyrings
  if have curl; then
    curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo_cmd tee /etc/apt/keyrings/githubcli-archive-keyring.gpg >/dev/null
  elif have wget; then
    wget -qO- https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo_cmd tee /etc/apt/keyrings/githubcli-archive-keyring.gpg >/dev/null
  else
    warn "curl or wget is required to configure the GitHub CLI apt repo"
    return
  fi
  sudo_cmd chmod go+r /etc/apt/keyrings/githubcli-archive-keyring.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo_cmd tee /etc/apt/sources.list.d/github-cli.list >/dev/null
  sudo_cmd apt-get update
}

install_deps_apt() {
  log "installing dependencies with apt"
  sudo_cmd apt-get update
  sudo_cmd apt-get install -y ca-certificates curl git golang-go
  install_apt_gh_repo
  sudo_cmd apt-get install -y gh
  if apt-cache show yazi >/dev/null 2>&1; then
    sudo_cmd apt-get install -y yazi
  else
    warn "yazi is not available from this apt repository; skipping optional yazi install"
  fi
}

install_deps_pacman() {
  log "installing dependencies with pacman"
  sudo_cmd pacman -Sy --needed --noconfirm git go github-cli yazi
}

install_deps_brew() {
  log "installing dependencies with brew"
  brew install git go gh yazi
}

install_deps() {
  case "$(detect_pm)" in
    apt) install_deps_apt ;;
    pacman) install_deps_pacman ;;
    brew) install_deps_brew ;;
  esac
}

ensure_github_auth() {
  if ! have gh; then
    return
  fi
  if gh auth status -h github.com >/dev/null 2>&1; then
    gh auth setup-git >/dev/null 2>&1 || true
    return
  fi
  warn "GitHub CLI is not authenticated. Private repos require: gh auth login -h github.com"
}

profile_file() {
  shell_name="$(basename "${SHELL:-sh}")"
  case "$shell_name" in
    zsh) printf '%s\n' "$HOME/.zshrc" ;;
    bash) printf '%s\n' "$HOME/.bashrc" ;;
    fish) printf '%s\n' "$HOME/.config/fish/config.fish" ;;
    *) printf '%s\n' "$HOME/.profile" ;;
  esac
}

ensure_path() {
  case ":$PATH:" in
    *":$INSTALL_DIR:"*) return ;;
  esac

  if [ "${GITM8_SKIP_PATH_UPDATE:-}" = "1" ]; then
    warn "$INSTALL_DIR is not on PATH; add it with: export PATH=\"$INSTALL_DIR:\$PATH\""
    return
  fi

  rc_file="$(profile_file)"
  if ! mkdir -p "$(dirname "$rc_file")" || ! touch "$rc_file"; then
    warn "could not update $rc_file; add this manually: export PATH=\"$INSTALL_DIR:\$PATH\""
    return
  fi

  if [ "$(basename "${SHELL:-sh}")" = "fish" ]; then
    if ! grep -Fq "fish_add_path $INSTALL_DIR" "$rc_file"; then
      if ! printf '\nfish_add_path %s\n' "$INSTALL_DIR" >>"$rc_file"; then
        warn "could not update $rc_file; add $INSTALL_DIR to PATH manually"
        return
      fi
    fi
  else
    if ! grep -Fq "$INSTALL_DIR" "$rc_file"; then
      if ! printf '\nexport PATH="%s:$PATH"\n' "$INSTALL_DIR" >>"$rc_file"; then
        warn "could not update $rc_file; add this manually: export PATH=\"$INSTALL_DIR:\$PATH\""
        return
      fi
    fi
  fi

  warn "$INSTALL_DIR was added to $rc_file; restart your shell or run: export PATH=\"$INSTALL_DIR:\$PATH\""
}

build_from_source() {
  have git || die "git is required"
  have go || die "go is required"

  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT
  srcdir="$tmpdir/src"
  outbin="$tmpdir/bin/$BIN_NAME"

  log "cloning $REPO_URL"
  git clone --depth 1 --branch "$REF" "$REPO_URL" "$srcdir"

  log "building $BIN_NAME"
  mkdir -p "$(dirname "$outbin")"
  (cd "$srcdir" && go build -o "$outbin" .)

  mkdir -p "$INSTALL_DIR"
  install -m 0755 "$outbin" "$INSTALL_DIR/$BIN_NAME"
  log "installed $INSTALL_DIR/$BIN_NAME"

  if [ "${GITM8_SKIP_MAN_INSTALL:-}" = "1" ]; then
    log "skipping man page install because GITM8_SKIP_MAN_INSTALL=1"
  elif [ -f "$srcdir/docs/man/gitm8.1" ]; then
    mkdir -p "$MAN_DIR"
    install -m 0644 "$srcdir/docs/man/gitm8.1" "$MAN_DIR/$BIN_NAME.1"
    log "installed $MAN_DIR/$BIN_NAME.1"
  else
    warn "manual page not found in source checkout; skipping man page install"
  fi
}

main() {
  if [ "${GITM8_SKIP_DEPS:-}" = "1" ]; then
    log "skipping dependency install because GITM8_SKIP_DEPS=1"
  else
    install_deps
  fi
  ensure_github_auth
  build_from_source
  ensure_path
  [ -x "$INSTALL_DIR/$BIN_NAME" ] || die "$INSTALL_DIR/$BIN_NAME was not installed correctly"
  log "done"
}

main "$@"
