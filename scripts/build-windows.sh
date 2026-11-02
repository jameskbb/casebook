#!/usr/bin/env bash
#
# Cross-compile both Windows executables from WSL or Linux, CGO-free (Wails
# v2 on Windows needs no cgo), stamped with one scripts/version.sh version:
#
#   dist/windows-<arch>/Casebook.exe        desktop app (track A, desktop/)
#   dist/windows-<arch>/bin/casebook.exe    CLI and MCP server (track B)
#
# The CLI lives in a bin directory next to the desktop executable because
# Casebook.exe and casebook.exe are one path on case-insensitive file systems
# (docs/decisions/0012-cli-placement.md).
#
# Steps: build the frontend once (npm ci && npm run build in desktop/frontend,
# unless --skip-frontend), then per arch go build desktop/ with
# -tags desktop,production and -H windowsgui, then scripts/build-cli.sh.
# go build links any .syso resource file committed in desktop/. wails build
# creates its .syso (icon, version information, manifest) only during its
# own build and deletes it, so a WSL build usually has none of the three;
# the release workflow builds the shipped Casebook.exe with wails build.
#
# Development mode skips parts that are not present yet (exit 0); release
# mode (--release or CASEBOOK_RELEASE=1) fails at the first missing part.
#
# Exit codes: 0 built or skipped, 1 build failure or missing part in release
# mode, 2 usage error.
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() {
  cat <<'EOF'
usage: scripts/build-windows.sh [--arch amd64|arm64|all] [--skip-frontend] [--release]

  --arch ARCH       amd64 (default), arm64, or all
  --skip-frontend   reuse an existing desktop/frontend/dist instead of
                    running npm ci and npm run build
  --release         fail instead of skipping when a part is missing
                    (same as CASEBOOK_RELEASE=1)

Output: dist/windows-<arch>/Casebook.exe and dist/windows-<arch>/bin/casebook.exe
EOF
}

arch_flag="amd64"
skip_frontend=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --arch)
      [ "$#" -ge 2 ] || cb_usage_die "--arch needs a value"
      arch_flag="$2"
      shift 2
      ;;
    --skip-frontend)
      skip_frontend=1
      shift
      ;;
    --release)
      cb_set_release
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      cb_usage_die "unknown argument: $1"
      ;;
  esac
done

case "$arch_flag" in
  amd64 | arm64) archs="$arch_flag" ;;
  all) archs="amd64 arm64" ;;
  *) cb_usage_die "unsupported --arch '$arch_flag' (amd64, arm64 or all)" ;;
esac

# Validate the version inputs before anything else so a bad
# CASEBOOK_VERSION is reported even when every part is skipped.
version="$(cb_version)"
ldflags="$(cb_ldflags)"

# Which parts exist. Release mode exits 1 at the first missing one.
have_desktop=0
have_cli=0
if cb_require_part desktop "track A"; then
  have_desktop=1
fi
if cb_require_part cmd/casebook "track B"; then
  have_cli=1
fi
if [ "$have_desktop" = "0" ] && [ "$have_cli" = "0" ]; then
  cb_info "nothing to build yet"
  exit 0
fi

command -v go >/dev/null 2>&1 || cb_die "go is not on PATH (install Go, see go.mod for the version)"

# Frontend, built once for every arch.
if [ "$have_desktop" = "1" ]; then
  frontend="$CB_ROOT/desktop/frontend"
  if cb_require_part desktop/frontend/package.json "track A"; then
    if [ "$skip_frontend" = "1" ]; then
      if [ ! -d "$frontend/dist" ] || [ -z "$(ls -A "$frontend/dist")" ]; then
        cb_die "--skip-frontend needs a built desktop/frontend/dist (run npm ci && npm run build there first)"
      fi
      cb_info "reusing existing desktop/frontend/dist (--skip-frontend)"
    else
      command -v npm >/dev/null 2>&1 ||
        cb_die "npm is not on PATH: install Node 24, or pass --skip-frontend to reuse an existing desktop/frontend/dist"
      cb_info "building the frontend in desktop/frontend"
      (cd "$frontend" && npm ci) || cb_die "npm ci failed in desktop/frontend"
      (cd "$frontend" && npm run build) || cb_die "npm run build failed in desktop/frontend"
    fi
  else
    cb_info "building the desktop Go code as is, without a frontend build"
  fi
fi

# canon PATH: absolute path of an existing file with symlinks in its
# directory resolved.
canon() {
  local dir
  dir="$(cd "$(dirname "$1")" && pwd -P)" || return 1
  printf '%s/%s\n' "$dir" "$(basename "$1")"
}

# lower STRING: ASCII lower case.
lower() {
  printf '%s\n' "$1" | tr '[:upper:]' '[:lower:]'
}

dist="$(cb_dist_dir)"
built=""
for arch in $archs; do
  outdir="$dist/windows-$arch"
  gui="$outdir/$CB_GUI_NAME.exe"
  cli="$outdir/$CB_CLI_SUBDIR/$CB_CLI_NAME.exe"
  mkdir -p "$outdir" || cb_die "cannot create $outdir"
  # Remove this run's outputs from earlier runs so the summary and the
  # collision guard only see what this run built.
  rm -f "$gui" "$cli"

  if [ "$have_desktop" = "1" ]; then
    cb_info "building $CB_GUI_NAME.exe $version for windows/$arch"
    if ! (cd "$CB_ROOT/desktop" && CGO_ENABLED=0 GOOS=windows GOARCH="$arch" \
      go build -trimpath -buildvcs=false -tags desktop,production \
      -ldflags "-H windowsgui -s -w $ldflags" -o "$gui" .); then
      cb_die "go build of desktop failed for windows/$arch"
    fi
  fi

  if [ "$have_cli" = "1" ]; then
    cli_args=(--os windows --arch "$arch" --out "$cli")
    if [ "$CB_RELEASE" = "1" ]; then
      cli_args+=(--release)
    fi
    "$CB_ROOT/scripts/build-cli.sh" "${cli_args[@]}" || exit $?
  fi

  # Collision guard: the two executables must stay two files on a
  # case-insensitive file system (ADR-0012).
  if [ -f "$gui" ] && [ -f "$cli" ]; then
    if [ "$gui" -ef "$cli" ]; then
      cb_die "$CB_GUI_NAME.exe and $CB_CLI_NAME.exe are the same file in $outdir"
    fi
    if [ "$(lower "$(canon "$gui")")" = "$(lower "$(canon "$cli")")" ]; then
      cb_die "$gui and $cli differ only by case and would overwrite each other on Windows"
    fi
    if cmp -s "$gui" "$cli"; then
      cb_die "$gui and $cli have identical contents; one overwrote the other"
    fi
  fi

  for f in "$gui" "$cli"; do
    if [ -f "$f" ]; then
      built="$built$f $(wc -c <"$f" | tr -d ' ') bytes ($version)
"
    fi
  done
done

if [ -n "$built" ]; then
  cb_info "built for windows ($version):"
  printf '%s' "$built" | sed 's/^/  /' >&2
fi
