#!/usr/bin/env bash
#
# Build the casebook CLI (which also serves `casebook mcp`) for any supported
# GOOS/GOARCH, CGO-free and stamped with scripts/version.sh values.
#
# Development mode skips with exit 0 while cmd/casebook is not present yet
# (track B); release mode (--release or CASEBOOK_RELEASE=1) fails instead.
#
# Exit codes: 0 built or skipped, 1 build failure or missing part in release
# mode, 2 usage error.
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() {
  cat <<'EOF'
usage: scripts/build-cli.sh [--os GOOS] [--arch GOARCH] [--out PATH] [--release]

  --os GOOS      windows, darwin or linux (default: go env GOOS)
  --arch GOARCH  amd64 or arm64 (default: go env GOARCH)
  --out PATH     output file (default: dist/casebook-<os>-<arch>[.exe])
  --release      fail instead of skipping when cmd/casebook is missing
                 (same as CASEBOOK_RELEASE=1)
EOF
}

goos=""
goarch=""
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --os)
      [ "$#" -ge 2 ] || cb_usage_die "--os needs a value"
      goos="$2"
      shift 2
      ;;
    --arch)
      [ "$#" -ge 2 ] || cb_usage_die "--arch needs a value"
      goarch="$2"
      shift 2
      ;;
    --out)
      if [ "$#" -lt 2 ] || [ -z "$2" ]; then
        cb_usage_die "--out needs a value"
      fi
      out="$2"
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
  "" | windows | darwin | linux) ;;
  *) cb_usage_die "unsupported --os '$goos' (windows, darwin or linux)" ;;
esac
case "$goarch" in
  "" | amd64 | arm64) ;;
  *) cb_usage_die "unsupported --arch '$goarch' (amd64 or arm64)" ;;
esac

# Validate the version inputs before deciding to skip, so a bad
# CASEBOOK_VERSION is reported even in development.
version="$(cb_version)"
ldflags="$(cb_ldflags)"

if ! cb_require_part cmd/casebook "track B"; then
  exit 0
fi

command -v go >/dev/null 2>&1 || cb_die "go is not on PATH (install Go, see go.mod for the version)"

if [ -z "$goos" ]; then
  goos="$(go env GOOS)"
fi
if [ -z "$goarch" ]; then
  goarch="$(go env GOARCH)"
fi
case "$goos" in
  windows | darwin | linux) ;;
  *) cb_usage_die "unsupported host GOOS '$goos'; pass --os" ;;
esac
case "$goarch" in
  amd64 | arm64) ;;
  *) cb_usage_die "unsupported host GOARCH '$goarch'; pass --arch" ;;
esac

if [ -z "$out" ]; then
  dist="$(cb_dist_dir)"
  out="$dist/$CB_CLI_NAME-$goos-$goarch$(cb_exe_suffix "$goos")"
else
  out="$(cb_abs_path "$out")"
fi
mkdir -p "$(dirname "$out")" || cb_die "cannot create $(dirname "$out")"

cb_info "building $CB_CLI_NAME $version for $goos/$goarch"
if ! (cd "$CB_ROOT" && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
  go build -trimpath -buildvcs=false -ldflags "-s -w $ldflags" -o "$out" ./cmd/casebook); then
  cb_die "go build of cmd/casebook failed for $goos/$goarch"
fi

cb_info "built: $out ($version)"
