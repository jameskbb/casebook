# shellcheck shell=bash
#
# Shared helpers for the Casebook build scripts. Source this file; do not
# execute it. Callers run with `set -euo pipefail`.
#
# Exit codes used by every script:
#   0  success, or a development-mode skip of a part that is not present yet
#   1  build failure, or a missing part in release mode
#   2  usage error or invalid input (bad flag, bad version or commit value)
#
# Release mode (CASEBOOK_RELEASE=1 in the environment, or a script's
# --release flag through cb_set_release) turns a missing part into a hard
# failure. The release workflow always runs in release mode.
#
# Keep this file compatible with bash 3.2 (the macOS system bash): no
# associative arrays, no ${var,,}, no mapfile.

# Repository root, computed from the location of this file so scripts work
# from any working directory and inside the test harness's fixture copies.
CB_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"

# Layout constants. Callers use them; lib.sh alone cannot see that use.
# shellcheck disable=SC2034
{
  CB_MODULE="github.com/jameskbb/casebook"
  # Executable names and the CLI's directory next to the desktop executable:
  # see docs/decisions/0012-cli-placement.md. Windows: <dir>/Casebook.exe and
  # <dir>/bin/casebook.exe; macOS: Casebook.app/Contents/MacOS/Casebook and
  # Casebook.app/Contents/MacOS/bin/casebook.
  CB_GUI_NAME="Casebook"
  CB_CLI_NAME="casebook"
  CB_CLI_SUBDIR="bin"
}

CB_RELEASE=0
if [ "${CASEBOOK_RELEASE:-}" = "1" ]; then
  CB_RELEASE=1
fi

# cb_info MESSAGE: progress line on stderr.
cb_info() {
  printf '%s\n' "$*" >&2
}

# cb_skip MESSAGE: development-mode skip line on stderr.
cb_skip() {
  printf 'skip: %s\n' "$*" >&2
}

# cb_die MESSAGE: error line on stderr, exit 1.
cb_die() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

# cb_usage_die MESSAGE: usage or input error on stderr, exit 2.
cb_usage_die() {
  printf 'error: %s\n' "$*" >&2
  exit 2
}

# cb_set_release: switch the current script into release mode.
cb_set_release() {
  CB_RELEASE=1
}

# cb_require_part PATH LABEL: return 0 when PATH (relative to the repository
# root) exists. When it is absent, release mode exits 1 and development mode
# prints a skip line and returns 1 so the caller can skip that part.
cb_require_part() {
  local part="$1" label="$2"
  if [ -e "$CB_ROOT/$part" ]; then
    return 0
  fi
  if [ "$CB_RELEASE" = "1" ]; then
    cb_die "missing $label ($part) in release mode"
  fi
  cb_skip "$part not present yet ($label)"
  return 1
}

# cb_own_git: succeed when CB_ROOT is the top level of a git work tree, so a
# fixture copy inside some other repository never picks up that repository's
# tags or commits.
cb_own_git() {
  local top
  command -v git >/dev/null 2>&1 || return 1
  top="$(git -C "$CB_ROOT" rev-parse --show-toplevel 2>/dev/null)" || return 1
  [ -n "$top" ] || return 1
  top="$(cd "$top" && pwd -P)" || return 1
  [ "$top" = "$CB_ROOT" ]
}

# cb_version: print the build version. CASEBOOK_VERSION wins when set;
# otherwise `git describe --tags --always --dirty`; otherwise dev. The value
# flows into -ldflags and shell commands, so anything outside
# ^[0-9A-Za-z][0-9A-Za-z._+-]*$ or longer than 128 bytes is rejected (exit 2).
cb_version() {
  local LC_ALL=C
  local v="" re='^[0-9A-Za-z][0-9A-Za-z._+-]*$'
  if [ -n "${CASEBOOK_VERSION:-}" ]; then
    v="$CASEBOOK_VERSION"
  elif cb_own_git && v="$(git -C "$CB_ROOT" describe --tags --always --dirty 2>/dev/null)" && [ -n "$v" ]; then
    :
  else
    v="dev"
  fi
  if [ "${#v}" -gt 128 ] || ! [[ "$v" =~ $re ]]; then
    cb_usage_die "invalid version '$v': must match $re and be at most 128 bytes"
  fi
  printf '%s\n' "$v"
}

# cb_commit: print the short commit. CASEBOOK_COMMIT wins when set;
# otherwise `git rev-parse --short HEAD`; otherwise empty. A non-empty value
# must match ^[0-9a-f]{4,40}$ (exit 2 otherwise).
cb_commit() {
  local LC_ALL=C
  local c="" re='^[0-9a-f]{4,40}$'
  if [ -n "${CASEBOOK_COMMIT:-}" ]; then
    c="$CASEBOOK_COMMIT"
  elif cb_own_git && c="$(git -C "$CB_ROOT" rev-parse --short HEAD 2>/dev/null)"; then
    :
  else
    c=""
  fi
  if [ -n "$c" ] && ! [[ "$c" =~ $re ]]; then
    cb_usage_die "invalid commit '$c': must match $re"
  fi
  printf '%s\n' "$c"
}

# cb_numeric_version: print MAJOR.MINOR.PATCH from the start of cb_version
# (after an optional leading v), else 0.0.0. Used for the Windows and macOS
# product version fields, which accept numbers only.
cb_numeric_version() {
  local LC_ALL=C
  local v re='^v?([0-9]+)\.([0-9]+)\.([0-9]+)'
  v="$(cb_version)" || exit $?
  if [[ "$v" =~ $re ]]; then
    printf '%s.%s.%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}" "${BASH_REMATCH[3]}"
  else
    printf '0.0.0\n'
  fi
}

# cb_marker: print the stamp marker, casebook-version=<version>. The -X
# flags set internal/version.Marker to it; release checks search each
# executable for this exact text, which Go's automatic build information
# never contains. Builds use -trimpath, which also keeps the -ldflags string
# itself out of that build information.
cb_marker() {
  local v
  v="$(cb_version)" || exit $?
  printf 'casebook-version=%s\n' "$v"
}

# cb_ldflags: print the -X flags that stamp internal/version. Every Casebook
# build (both executables, every platform) uses exactly this string.
cb_ldflags() {
  local v c m
  v="$(cb_version)" || exit $?
  c="$(cb_commit)" || exit $?
  m="$(cb_marker)" || exit $?
  printf -- '-X %s/internal/version.Version=%s -X %s/internal/version.Commit=%s -X %s/internal/version.Marker=%s\n' \
    "$CB_MODULE" "$v" "$CB_MODULE" "$c" "$CB_MODULE" "$m"
}

# cb_exe_suffix GOOS: print .exe for windows, nothing otherwise.
cb_exe_suffix() {
  if [ "$1" = "windows" ]; then
    printf '.exe'
  fi
}

# cb_dist_dir: create $CB_ROOT/dist (ignoring itself through dist/.gitignore
# containing a single `*` line) and print its path.
cb_dist_dir() {
  local dist="$CB_ROOT/dist"
  mkdir -p "$dist" || cb_die "cannot create $dist"
  if [ ! -e "$dist/.gitignore" ]; then
    printf '*\n' >"$dist/.gitignore" || cb_die "cannot write $dist/.gitignore"
  fi
  printf '%s\n' "$dist"
}

# cb_abs_path PATH: print PATH made absolute against the current directory
# (the file need not exist).
cb_abs_path() {
  case "$1" in
    /*) printf '%s\n' "$1" ;;
    *) printf '%s/%s\n' "$(pwd -P)" "$1" ;;
  esac
}
