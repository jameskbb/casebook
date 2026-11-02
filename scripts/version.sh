#!/usr/bin/env bash
#
# Print the version values every Casebook build uses. Workflows and scripts
# call `scripts/version.sh ldflags` so both executables share one string.
#
#   version  build version: CASEBOOK_VERSION, else git describe, else dev
#   commit   short commit: CASEBOOK_COMMIT, else git rev-parse --short HEAD,
#            else an empty line
#   numeric  MAJOR.MINOR.PATCH for product version fields, else 0.0.0
#   marker   casebook-version=<version>, the text release checks look for
#            in each executable (internal/version.Marker)
#   ldflags  the -X flags that stamp internal/version
#
# Exit codes: 0 success, 2 usage error or invalid version or commit value.
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() {
  cat <<'EOF'
usage: scripts/version.sh version|commit|numeric|marker|ldflags

  version  build version (CASEBOOK_VERSION, git describe --tags --always --dirty, or dev)
  commit   short commit (CASEBOOK_COMMIT, git rev-parse --short HEAD, or empty)
  numeric  MAJOR.MINOR.PATCH taken from the version, else 0.0.0
  marker   casebook-version=<version>, stamped into internal/version.Marker
  ldflags  -X flags for github.com/jameskbb/casebook/internal/version
EOF
}

if [ "$#" -ne 1 ]; then
  usage >&2
  exit 2
fi

case "$1" in
  version) cb_version ;;
  commit) cb_commit ;;
  numeric) cb_numeric_version ;;
  marker) cb_marker ;;
  ldflags) cb_ldflags ;;
  -h | --help) usage ;;
  *)
    usage >&2
    exit 2
    ;;
esac
