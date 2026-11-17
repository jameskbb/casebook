#!/usr/bin/env bash
#
# Package one CLI-only release archive: the casebook executable with
# LICENSE and THIRD_PARTY_NOTICES.txt next to it, flat (no folder inside).
#
#   linux, darwin: casebook-cli-<version>-<os>-<arch>.tar.gz  (casebook)
#   windows:       casebook-cli-<version>-windows-<arch>.zip  (casebook.exe)
#
# scripts/install.sh and the Homebrew formula (packaging/homebrew) download
# these names; docs/building-releases.md lists them. The lower-case
# casebook-cli- prefix keeps them apart from the desktop packages
# (Casebook-<version>-...) on case-insensitive file systems.
#
# Exit codes: 0 written, 1 failure (missing input, missing stamp marker in
# release mode, no archiver), 2 usage error.
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() {
  cat <<'EOF'
usage: scripts/package-cli.sh --os GOOS --arch GOARCH --bin PATH [--out DIR]
                              [--notices FILE] [--release]

  --os GOOS       windows, darwin or linux
  --arch GOARCH   amd64 or arm64
  --bin PATH      the built CLI executable (scripts/build-cli.sh output)
  --out DIR       directory for the archive (default: dist/)
  --notices FILE  THIRD_PARTY_NOTICES.txt (scripts/third-party-notices.sh)
  --release       require the notices, LICENSE and the version stamp marker
                  (same as CASEBOOK_RELEASE=1)

The version in the archive name is scripts/version.sh version. LICENSE is
taken from the repository root when it exists.
EOF
}

goos=""
goarch=""
bin=""
outdir=""
notices=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --os | --arch | --bin | --out | --notices)
      if [ "$#" -lt 2 ] || [ -z "$2" ]; then
        cb_usage_die "$1 needs a value"
      fi
      case "$1" in
        --os) goos="$2" ;;
        --arch) goarch="$2" ;;
        --bin) bin="$(cb_abs_path "$2")" ;;
        --out) outdir="$(cb_abs_path "$2")" ;;
        --notices) notices="$(cb_abs_path "$2")" ;;
      esac
      shift 2
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

case "$goos" in
  windows | darwin | linux) ;;
  "") cb_usage_die "--os is required" ;;
  *) cb_usage_die "unsupported --os '$goos' (windows, darwin or linux)" ;;
esac
case "$goarch" in
  amd64 | arm64) ;;
  "") cb_usage_die "--arch is required" ;;
  *) cb_usage_die "unsupported --arch '$goarch' (amd64 or arm64)" ;;
esac
[ -n "$bin" ] || cb_usage_die "--bin is required"

version="$(cb_version)"
[ -f "$bin" ] || cb_die "$bin does not exist"
[ -s "$bin" ] || cb_die "$bin is empty"

if [ "$CB_RELEASE" = "1" ]; then
  marker="$(cb_marker)"
  if ! grep -a -F -q -- "$marker" "$bin"; then
    cb_die "$bin does not contain the stamp marker $marker"
  fi
  [ -n "$notices" ] || cb_die "--notices is required in release mode"
  [ -f "$CB_ROOT/LICENSE" ] || cb_die "LICENSE is missing at the repository root; a release ships its license"
fi
if [ -n "$notices" ] && [ ! -s "$notices" ]; then
  cb_die "$notices does not exist or is empty"
fi

if [ -z "$outdir" ]; then
  outdir="$(cb_dist_dir)"
fi
mkdir -p "$outdir" || cb_die "cannot create $outdir"

exe="$CB_CLI_NAME$(cb_exe_suffix "$goos")"
base="$CB_CLI_NAME-cli-$version-$goos-$goarch"
stage="$(mktemp -d "${TMPDIR:-/tmp}/casebook-package-cli.XXXXXX")"
trap 'rm -rf "$stage"' EXIT

cp "$bin" "$stage/$exe"
chmod 0755 "$stage/$exe"
files=("$exe")
if [ -f "$CB_ROOT/LICENSE" ]; then
  cp "$CB_ROOT/LICENSE" "$stage/LICENSE"
  files+=(LICENSE)
fi
if [ -n "$notices" ]; then
  cp "$notices" "$stage/THIRD_PARTY_NOTICES.txt"
  files+=(THIRD_PARTY_NOTICES.txt)
fi

if [ "$goos" = "windows" ]; then
  archive="$outdir/$base.zip"
  rm -f "$archive"
  if command -v 7z >/dev/null 2>&1; then
    (cd "$stage" && 7z a -tzip -bd "$archive" "${files[@]}" >/dev/null) || cb_die "7z failed for $archive"
  elif command -v zip >/dev/null 2>&1; then
    (cd "$stage" && zip -q -X "$archive" "${files[@]}") || cb_die "zip failed for $archive"
  elif command -v python3 >/dev/null 2>&1; then
    (cd "$stage" && python3 -m zipfile -c "$archive" "${files[@]}") || cb_die "python3 zipfile failed for $archive"
  else
    cb_die "no zip tool found (7z, zip or python3)"
  fi
else
  archive="$outdir/$base.tar.gz"
  rm -f "$archive"
  # COPYFILE_DISABLE keeps macOS tar from adding ._ resource fork entries.
  (cd "$stage" && COPYFILE_DISABLE=1 tar -czf "$archive" "${files[@]}") || cb_die "tar failed for $archive"
fi

cb_info "packaged: $archive (${files[*]})"
printf '%s\n' "$archive"
