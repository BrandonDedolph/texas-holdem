#!/bin/sh
# Texas Hold'em installer — downloads the latest prebuilt binary for your OS/arch.
#
#   curl -fsSL https://raw.githubusercontent.com/BrandonDedolph/texas-holdem/main/install.sh | sh
#
# Env overrides:
#   HOLDEM_INSTALL_DIR   target dir for the binary (default: /usr/local/bin, or
#                        ~/.local/bin when /usr/local/bin is not writable)
#   HOLDEM_VERSION       version tag to install (default: latest release)
set -eu

REPO="BrandonDedolph/texas-holdem"
BINARY="holdem"

info() { printf '\033[0;34m==>\033[0m %s\n' "$1"; }
err()  { printf '\033[0;31merror:\033[0m %s\n' "$1" >&2; exit 1; }

# --- detect OS/arch (mapped to GoReleaser's naming) -------------------------
os=$(uname -s)
case "$os" in
  Linux)  os=linux ;;
  Darwin) os=darwin ;;
  *) err "unsupported OS: $os (download a binary from https://github.com/$REPO/releases/latest)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64)  arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) err "unsupported architecture: $arch" ;;
esac

# --- pick a downloader ------------------------------------------------------
if command -v curl >/dev/null 2>&1; then
  dl() { curl -fsSL "$1"; }
  dlo() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  dl() { wget -qO- "$1"; }
  dlo() { wget -qO "$2" "$1"; }
else
  err "need curl or wget to download"
fi

# --- resolve version --------------------------------------------------------
version="${HOLDEM_VERSION:-}"
if [ -z "$version" ]; then
  info "Resolving latest release..."
  # Buffer the API response before grepping it: piping the downloader straight
  # into `grep -m1` makes grep close the pipe on the first match, which the
  # downloader reports as a (harmless) write failure.
  api=$(dl "https://api.github.com/repos/$REPO/releases/latest")
  version=$(printf '%s\n' "$api" | grep '"tag_name"' | head -n1 | cut -d'"' -f4)
  [ -n "$version" ] || err "could not determine latest version (no releases yet?)"
fi
# GoReleaser archive names drop the leading 'v'.
ver_nov=${version#v}

# --- download & extract -----------------------------------------------------
archive="${BINARY}_${ver_nov}_${os}_${arch}.tar.gz"
url="https://github.com/$REPO/releases/download/${version}/${archive}"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

info "Downloading $archive ($version)..."
dlo "$url" "$tmp/$archive" || err "download failed: $url"

info "Extracting..."
tar -xzf "$tmp/$archive" -C "$tmp" || err "extraction failed"
[ -f "$tmp/$BINARY" ] || err "binary not found in archive"
chmod +x "$tmp/$BINARY"

# --- choose install dir & place binary --------------------------------------
dir="${HOLDEM_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  if [ -w /usr/local/bin ] 2>/dev/null; then
    dir=/usr/local/bin
  else
    dir="$HOME/.local/bin"
  fi
fi
mkdir -p "$dir"

if mv "$tmp/$BINARY" "$dir/$BINARY" 2>/dev/null; then
  :
elif command -v sudo >/dev/null 2>&1; then
  info "Need elevated permissions to write to $dir"
  sudo mv "$tmp/$BINARY" "$dir/$BINARY"
else
  err "cannot write to $dir (set HOLDEM_INSTALL_DIR to a writable path)"
fi

info "Installed $BINARY to $dir/$BINARY"
case ":$PATH:" in
  *":$dir:"*) info "Run: $BINARY" ;;
  *) info "Add $dir to your PATH, then run: $BINARY" ;;
esac
