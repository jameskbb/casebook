#!/usr/bin/env bash
#
# Check that the parts a release needs are present:
#
#   cmd/casebook                  CLI and MCP server
#   desktop/wails.json            desktop app
#   desktop/frontend/package.json desktop frontend
#
# Prints `ok: <path>` or `missing: <path> (<label>)` for each. Development
# mode always exits 0; release mode (--release or CASEBOOK_RELEASE=1) exits 1
# when anything is missing. The release workflow runs it first in every
# build job.
#
# Exit codes: 0 ready (or development mode), 1 missing part in release mode,
# 2 usage error.
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() {
  cat <<'EOF'
usage: scripts/preflight.sh [--release]

Reports which release parts are present. With --release (or
CASEBOOK_RELEASE=1) a missing part fails with exit 1.
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

missing=0
check() {
  local part="$1" label="$2"
  if [ -e "$CB_ROOT/$part" ]; then
    printf 'ok: %s\n' "$part"
  else
    printf 'missing: %s (%s)\n' "$part" "$label"
    missing=$((missing + 1))
  fi
}

check cmd/casebook "CLI"
check desktop/wails.json "desktop app"
check desktop/frontend/package.json "desktop app"

if [ "$missing" -gt 0 ] && [ "$CB_RELEASE" = "1" ]; then
  cb_die "release preflight failed: $missing part(s) missing"
fi
exit 0
