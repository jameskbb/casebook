#!/usr/bin/env bash
#
# Render packaging/homebrew/casebook.rb for one release: fill the version,
# the tag, the SHA-256 of the four Linux and macOS CLI archives (from the
# release's SHA256SUMS.txt) and the SPDX license id (from LICENSE).
#
# .github/workflows/homebrew.yml runs it when a release is published; a
# maintainer can run it by hand to update the tap (docs/building-releases.md).
#
# Exit codes: 0 written, 1 missing input, 2 usage error.
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() {
  cat <<'EOF'
usage: scripts/render-homebrew-formula.sh --version TAG --sums FILE [--out FILE]

  --version TAG  the release tag, for example v1.2.3 (no +build metadata)
  --sums FILE    that release's SHA256SUMS.txt
  --out FILE     where to write the formula (default: standard output)
EOF
}

tag=""
sums=""
out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --version | --sums | --out)
      if [ "$#" -lt 2 ] || [ -z "$2" ]; then
        cb_usage_die "$1 needs a value"
      fi
      case "$1" in
        --version) tag="$2" ;;
        --sums) sums="$2" ;;
        --out) out="$2" ;;
      esac
      shift 2
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

export LC_ALL=C
[ -n "$tag" ] || cb_usage_die "--version is required"
[ -n "$sums" ] || cb_usage_die "--sums is required"
if ! [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  cb_usage_die "'$tag' is not a release tag (vMAJOR.MINOR.PATCH with an optional -prerelease suffix)"
fi
[ -f "$sums" ] || cb_die "$sums does not exist"
template="$CB_ROOT/packaging/homebrew/casebook.rb"
[ -f "$template" ] || cb_die "$template is missing"

# sum_for NAME: print the lower-case SHA-256 of NAME from the sums file.
sum_for() {
  local name="$1" s
  s="$(awk -v f="$name" '$2 == f || $2 == "*" f { print tolower($1); exit }' "$sums")"
  if ! [[ "$s" =~ ^[0-9a-f]{64}$ ]]; then
    cb_die "$sums has no SHA-256 line for $name"
  fi
  printf '%s\n' "$s"
}

darwin_arm64="$(sum_for "casebook-cli-$tag-darwin-arm64.tar.gz")"
darwin_amd64="$(sum_for "casebook-cli-$tag-darwin-amd64.tar.gz")"
linux_arm64="$(sum_for "casebook-cli-$tag-linux-arm64.tar.gz")"
linux_amd64="$(sum_for "casebook-cli-$tag-linux-amd64.tar.gz")"

# The SPDX id of the project license, read from LICENSE; the formula's
# license line is dropped when it cannot be recognized, rather than guessed.
license=""
if [ -f "$CB_ROOT/LICENSE" ]; then
  first="$(sed -n '/[^[:space:]]/{p;q;}' "$CB_ROOT/LICENSE")"
  case "$first" in
    *"MIT License"*) license="MIT" ;;
    *"Apache License"*) license="Apache-2.0" ;;
  esac
fi
if [ -z "$license" ]; then
  cb_info "warning: could not recognize the license in LICENSE; the formula has no license line"
fi

rendered="$(sed \
  -e "s|@VERSION@|${tag#v}|g" \
  -e "s|@TAG@|$tag|g" \
  -e "s|@SHA256_DARWIN_ARM64@|$darwin_arm64|g" \
  -e "s|@SHA256_DARWIN_AMD64@|$darwin_amd64|g" \
  -e "s|@SHA256_LINUX_ARM64@|$linux_arm64|g" \
  -e "s|@SHA256_LINUX_AMD64@|$linux_amd64|g" \
  "$template")"
if [ -n "$license" ]; then
  rendered="${rendered//@LICENSE@/$license}"
else
  rendered="$(sed '/@LICENSE@/d' <<<"$rendered")"
fi
# Drop the template's header comment (everything before the class line);
# the tap copy gets its own.
rendered="$(sed -n '/^class Casebook < Formula$/,$p' <<<"$rendered")"
rendered="$(printf '%s\n%s\n' \
  "# Generated from packaging/homebrew/casebook.rb in jameskbb/casebook for $tag; do not edit by hand." \
  "$rendered")"
if grep -q '@[A-Z0-9_]*@' <<<"$rendered"; then
  cb_die "the rendered formula still holds a placeholder: $(grep -o '@[A-Z0-9_]*@' <<<"$rendered" | head -n 1)"
fi

if [ -z "$out" ]; then
  printf '%s\n' "$rendered"
else
  mkdir -p "$(dirname "$out")"
  printf '%s\n' "$rendered" >"$out"
  cb_info "wrote $out for $tag"
fi
