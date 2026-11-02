#!/usr/bin/env bash
#
# Wails desktop code embeds the built frontend (go:embed all:frontend/dist),
# so go build, go vet and go test cannot compile desktop/ until
# desktop/frontend/dist exists. CI compile checks run this script first:
#
#   - desktop/frontend missing: skip (track A not landed yet), exit 0
#   - desktop/frontend/dist non-empty: leave it alone, exit 0
#   - otherwise, development: write a placeholder dist/index.html
#   - otherwise, release mode (--release or CASEBOOK_RELEASE=1): exit 1,
#     because a release must ship the real frontend build
#
# It never overwrites or deletes existing files.
#
# Exit codes: 0 ok or skipped, 1 release mode without a built frontend,
# 2 usage error.
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() {
  cat <<'EOF'
usage: scripts/stub-frontend-dist.sh [--release]

Writes a placeholder desktop/frontend/dist/index.html for compile-only
checks when the frontend has not been built. Refuses in release mode.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
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

if ! cb_require_part desktop/frontend "track A"; then
  exit 0
fi

dist="$CB_ROOT/desktop/frontend/dist"
if [ -d "$dist" ] && [ -n "$(ls -A "$dist")" ]; then
  cb_info "ok: desktop/frontend/dist already populated"
  exit 0
fi

if [ "$CB_RELEASE" = "1" ]; then
  cb_die "desktop/frontend/dist is empty in release mode; releases must build the real frontend (npm ci && npm run build in desktop/frontend)"
fi

index="$dist/index.html"
mkdir -p "$dist" || cb_die "cannot create $dist"
printf '%s\n' '<!doctype html><title>Casebook</title><p>Placeholder written by scripts/stub-frontend-dist.sh for compile-only checks; not a real frontend build.</p>' >"$index" ||
  cb_die "cannot write $index"
cb_info "ok: wrote placeholder desktop/frontend/dist/index.html (compile-only)"
