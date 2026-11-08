#!/bin/sh
# Installs the stacklit binary from a GitHub release, verifying its sha256.
#
#   curl -fsSL https://raw.githubusercontent.com/glincker/stacklit/master/scripts/install.sh | sh
#
# Options (pass after `sh -s --` when piping):
#   --version X.Y.Z   install that version (default: latest release)
#   --prefix DIR      install into DIR/bin (default: $HOME/.local, no sudo needed)
#   --dry-run         resolve and verify, print what would happen, install nothing
#   --help
set -eu

REPO="glincker/stacklit"
VERSION=""
PREFIX="${STACKLIT_PREFIX:-$HOME/.local}"
DRY=0

say() { printf '%s\n' "$*"; }
die() { printf 'install.sh: %s\n' "$*" >&2; exit 1; }

while [ $# -gt 0 ]; do
  case "$1" in
    --version) [ $# -ge 2 ] || die "--version needs a value"; VERSION="${2#v}"; shift 2 ;;
    --prefix) [ $# -ge 2 ] || die "--prefix needs a value"; PREFIX="$2"; shift 2 ;;
    --dry-run) DRY=1; shift ;;
    -h|--help) sed -n '2,12p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported OS $(uname -s). On Windows use: npm install -g stacklit, or the zip from https://github.com/$REPO/releases" ;;
esac
case "$(uname -m)" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) die "unsupported architecture $(uname -m)" ;;
esac

if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL "$1" -o "$2"; }
  final_url() { curl -fsSL -o /dev/null -w '%{url_effective}' "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -q "$1" -O "$2"; }
  final_url() { wget -q --max-redirect=5 -S --spider "$1" 2>&1 | sed -n 's/^ *Location: //p' | tail -n 1; }
else
  die "need curl or wget"
fi

if command -v sha256sum >/dev/null 2>&1; then
  sha() { sha256sum "$1" | cut -d ' ' -f 1; }
elif command -v shasum >/dev/null 2>&1; then
  sha() { shasum -a 256 "$1" | cut -d ' ' -f 1; }
else
  die "need sha256sum or shasum to verify the download"
fi

if [ -z "$VERSION" ]; then
  url=$(final_url "https://github.com/$REPO/releases/latest") || die "could not resolve the latest release"
  VERSION="${url##*/v}"
  [ -n "$VERSION" ] && [ "$VERSION" != "$url" ] || die "could not parse the latest version from $url"
fi

name="stacklit_${VERSION}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/v${VERSION}"
bindir="$PREFIX/bin"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

say "stacklit $VERSION for $os/$arch"
fetch "$base/$name" "$tmp/$name" || die "download failed: $base/$name"
fetch "$base/checksums.txt" "$tmp/checksums.txt" || die "download failed: $base/checksums.txt"

want=$(awk -v f="$name" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || die "$name is not listed in checksums.txt"
got=$(sha "$tmp/$name")
[ "$want" = "$got" ] || die "checksum mismatch for $name (expected $want, got $got)"
say "sha256 ok: $got"

if [ "$DRY" -eq 1 ]; then
  say "dry run: would install $bindir/stacklit"
  exit 0
fi

tar -xzf "$tmp/$name" -C "$tmp" stacklit || die "archive does not contain a stacklit binary"
mkdir -p "$bindir"
mv "$tmp/stacklit" "$bindir/stacklit"
chmod 755 "$bindir/stacklit"
say "installed $bindir/stacklit"

case ":$PATH:" in
  *":$bindir:"*) ;;
  *) say "note: $bindir is not on your PATH. Add: export PATH=\"$bindir:\$PATH\"" ;;
esac

say ""
say "Next:"
say "  cd your-repo"
say "  stacklit init        # writes stacklit.json and wires your agents"
say "  stacklit setup claude --session-hook --refresh-hook   # optional Claude Code hooks"
