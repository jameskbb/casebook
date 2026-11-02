#!/bin/sh
#
# Install the casebook command line tool (which also serves MCP) from the
# latest GitHub release:
#
#   curl -fsSL https://raw.githubusercontent.com/jameskbb/casebook/main/scripts/install.sh | sh
#
# It detects the OS (Linux or macOS) and CPU (amd64 or arm64), downloads
# casebook-cli-<version>-<os>-<arch>.tar.gz and SHA256SUMS.txt from the
# release, checks the archive's SHA-256, and copies casebook into
# ~/.local/bin. It never uses sudo and changes no shell profile.
#
# Environment:
#   CASEBOOK_INSTALL_DIR  install folder (default: $HOME/.local/bin)
#   CASEBOOK_VERSION      a release tag such as v1.2.3 (default: the latest
#                         published release)
#
# Pass them to sh, not to curl:
#   curl -fsSL .../install.sh | CASEBOOK_INSTALL_DIR=/opt/bin sh
#
# The desktop app is not installed; on macOS and Windows it is a separate
# download from the Releases page (docs/updating.md).
#
# POSIX sh only: this runs under dash, busybox ash and macOS /bin/sh.

set -eu

REPO="jameskbb/casebook"
GITHUB="https://github.com"

say() {
  printf 'casebook-install: %s\n' "$*"
}

die() {
  printf 'casebook-install: error: %s\n' "$*" >&2
  exit 1
}

have() {
  command -v "$1" >/dev/null 2>&1
}

# fetch URL FILE: download URL to FILE, following redirects; fail on HTTP
# errors.
fetch() {
  if have curl; then
    curl -fsSL --proto '=https' --tlsv1.2 --retry 3 -o "$2" "$1"
  elif have wget; then
    wget -q --https-only -O "$2" "$1"
  else
    die "neither curl nor wget is installed"
  fi
}

# latest_tag: print the tag of the latest published release.
latest_tag() {
  if have curl; then
    # github.com/<repo>/releases/latest redirects to .../releases/tag/<tag>;
    # following the redirect needs no API call (and no API rate limit).
    url="$(curl -fsSL --proto '=https' --tlsv1.2 --retry 3 -o /dev/null -w '%{url_effective}' \
      "$GITHUB/$REPO/releases/latest")" || return 1
    tag="${url##*/releases/tag/}"
    if [ "$tag" = "$url" ]; then
      return 1
    fi
  else
    json="$(wget -q --https-only -O - "https://api.github.com/repos/$REPO/releases/latest")" || return 1
    tag="$(printf '%s\n' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  fi
  [ -n "$tag" ] || return 1
  printf '%s\n' "$tag"
}

detect_os() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    MINGW* | MSYS* | CYGWIN* | Windows_NT)
      die "this script installs on Linux and macOS; on Windows download Casebook-<version>-windows-<arch>-portable.zip (desktop app and CLI) or casebook-cli-<version>-windows-<arch>.zip (CLI only) from $GITHUB/$REPO/releases"
      ;;
    *) die "unsupported operating system: $(uname -s) (Linux and macOS are supported)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) arch=amd64 ;;
    arm64 | aarch64) arch=arm64 ;;
    *) die "unsupported CPU architecture: $(uname -m) (amd64 and arm64 are supported)" ;;
  esac
  # A shell running under Rosetta on Apple Silicon reports x86_64; install
  # the native arm64 build instead.
  if [ "$1" = darwin ] && [ "$arch" = amd64 ] &&
    [ "$(sysctl -n hw.optional.arm64 2>/dev/null || true)" = 1 ]; then
    arch=arm64
  fi
  echo "$arch"
}

# sha256 FILE: print the SHA-256 of FILE in lower-case hex.
sha256() {
  if have sha256sum; then
    sha256sum "$1" | cut -d ' ' -f 1
  elif have shasum; then
    shasum -a 256 "$1" | cut -d ' ' -f 1
  elif have openssl; then
    openssl dgst -sha256 "$1" | sed 's/^.*= *//'
  else
    die "no SHA-256 tool found (sha256sum, shasum or openssl); cannot verify the download"
  fi
}

main() {
  os="$(detect_os)"
  arch="$(detect_arch "$os")"
  install_dir="${CASEBOOK_INSTALL_DIR:-}"
  if [ -z "$install_dir" ]; then
    [ -n "${HOME:-}" ] || die "HOME is not set; set CASEBOOK_INSTALL_DIR"
    install_dir="$HOME/.local/bin"
  fi

  version="${CASEBOOK_VERSION:-}"
  if [ -z "$version" ]; then
    say "looking up the latest release of $REPO"
    version="$(latest_tag)" ||
      die "could not find the latest release at $GITHUB/$REPO/releases; set CASEBOOK_VERSION=v1.2.3 to pick one"
  fi
  case "$version" in
    v[0-9]*) ;;
    *) die "'$version' is not a release tag (expected something like v1.2.3)" ;;
  esac
  case "$version" in
    *[!0-9A-Za-z._-]*) die "'$version' is not a release tag (expected something like v1.2.3)" ;;
  esac

  archive="casebook-cli-$version-$os-$arch.tar.gz"
  base="$GITHUB/$REPO/releases/download/$version"

  tmp="$(mktemp -d 2>/dev/null || mktemp -d -t casebook-install)"
  # Remove the download folder whatever happens.
  trap 'rm -rf "$tmp"' EXIT
  trap 'rm -rf "$tmp"; exit 130' INT TERM

  say "downloading casebook $version for $os/$arch"
  fetch "$base/$archive" "$tmp/$archive" ||
    die "download failed: $base/$archive (does release $version have a CLI archive for $os/$arch?)"
  fetch "$base/SHA256SUMS.txt" "$tmp/SHA256SUMS.txt" ||
    die "download failed: $base/SHA256SUMS.txt"

  want="$(awk -v f="$archive" '$2 == f || $2 == "*" f { print $1; exit }' "$tmp/SHA256SUMS.txt")"
  [ -n "$want" ] || die "SHA256SUMS.txt has no line for $archive"
  got="$(sha256 "$tmp/$archive")"
  want="$(printf '%s' "$want" | tr 'A-F' 'a-f')"
  if [ "$got" != "$want" ]; then
    die "checksum mismatch for $archive (expected $want, got $got); nothing was installed"
  fi
  say "checksum ok ($got)"

  mkdir "$tmp/x"
  tar -xzf "$tmp/$archive" -C "$tmp/x" casebook ||
    die "could not extract casebook from $archive"
  [ -f "$tmp/x/casebook" ] || die "$archive does not contain casebook"

  mkdir -p "$install_dir" || die "cannot create $install_dir (set CASEBOOK_INSTALL_DIR to a folder you can write)"
  [ -w "$install_dir" ] || die "$install_dir is not writable (set CASEBOOK_INSTALL_DIR to a folder you can write; this script never uses sudo)"
  # Copy next to the target, then rename, so a running casebook (an AI
  # client's MCP server) keeps its old file and the swap is atomic.
  cp "$tmp/x/casebook" "$install_dir/.casebook.new.$$"
  chmod 0755 "$install_dir/.casebook.new.$$"
  mv -f "$install_dir/.casebook.new.$$" "$install_dir/casebook" ||
    die "could not write $install_dir/casebook"

  installed="$("$install_dir/casebook" --version 2>/dev/null || true)"
  say "installed $install_dir/casebook${installed:+ ($installed)}"

  case ":${PATH:-}:" in
    *":$install_dir:"*)
      found="$(command -v casebook 2>/dev/null || true)"
      if [ -n "$found" ] && [ "$found" != "$install_dir/casebook" ]; then
        say "note: 'casebook' on your PATH is $found, which comes before $install_dir; remove it or reorder PATH"
      fi
      cmd=casebook
      ;;
    *)
      say "$install_dir is not on your PATH. Add it, for example:"
      # shellcheck disable=SC2016
      printf '\n    export PATH="%s:$PATH"\n\n' "$install_dir"
      say "(put that line in ~/.profile, ~/.bashrc or ~/.zshrc to keep it)"
      cmd="$install_dir/casebook"
      ;;
  esac

  say "next step: connect an AI client, for example"
  printf '\n    %s setup claude\n\n' "$cmd"
  say "(setup codex for Codex, setup mcp for any other MCP client; doctor checks the install)"
}

main "$@"
