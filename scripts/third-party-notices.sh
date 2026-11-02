#!/usr/bin/env bash
#
# Write THIRD_PARTY_NOTICES.txt: the license texts of everything linked or
# bundled into the Casebook executables (docs/decisions/0011-license.md).
#
#   1. The Go standard library and runtime (GOROOT/LICENSE).
#   2. Every Go module that provides a package linked into ./cmd/casebook or
#      ./desktop on windows, darwin or linux, with its LICENSE, COPYING,
#      NOTICE and similar files from the module cache.
#   3. Every production (non-dev) npm package in
#      desktop/frontend/package-lock.json, with the same files from
#      desktop/frontend/node_modules. Vite bundles these into the desktop app.
#
# The Go modules must be downloadable (go mod download) and the frontend
# dependencies installed (npm ci --ignore-scripts in desktop/frontend). A
# package without a license file, or a missing node_modules folder, is a
# warning in development and an error in release mode (--release or
# CASEBOOK_RELEASE=1), so no release ships with an incomplete notice.
#
# Exit codes: 0 written, 1 missing input in release mode or a tool failure,
# 2 usage error.
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() {
  cat <<'EOF'
usage: scripts/third-party-notices.sh [--out FILE] [--release]

  --out FILE  where to write the notices (default: dist/THIRD_PARTY_NOTICES.txt)
  --release   fail on a missing license file or missing node_modules
              (same as CASEBOOK_RELEASE=1)
EOF
}

out=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --out)
      if [ "$#" -lt 2 ] || [ -z "$2" ]; then
        cb_usage_die "--out needs a value"
      fi
      out="$(cb_abs_path "$2")"
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

export LC_ALL=C
command -v go >/dev/null 2>&1 || cb_die "go is not on PATH"
if [ -z "$out" ]; then
  out="$(cb_dist_dir)/THIRD_PARTY_NOTICES.txt"
fi
mkdir -p "$(dirname "$out")" || cb_die "cannot create $(dirname "$out")"

problems=0
# problem MESSAGE: an error in release mode, a warning otherwise.
problem() {
  if [ "$CB_RELEASE" = "1" ]; then
    printf 'error: %s\n' "$*" >&2
  else
    printf 'warn: %s\n' "$*" >&2
  fi
  problems=$((problems + 1))
}

# license_files DIR: print the license and notice files directly in DIR,
# sorted, one per line.
license_files() {
  find "$1" -maxdepth 1 -type f \( -iname 'LICEN[CS]E*' -o -iname 'COPYING*' \
    -o -iname 'NOTICE*' -o -iname 'COPYRIGHT*' -o -iname 'UNLICENSE*' \) 2>/dev/null |
    LC_ALL=C sort
}

# section TITLE DIR: append a heading and every license file in DIR.
section() {
  local title="$1" dir="$2" files f
  files="$(license_files "$dir")"
  if [ -z "$files" ]; then
    problem "no license file found for $title in $dir"
    return 0
  fi
  {
    printf '%s\n' "================================================================================"
    printf '%s\n' "$title"
    printf '%s\n\n' "================================================================================"
  } >>"$tmp"
  while IFS= read -r f; do
    {
      printf -- '--- %s ---\n\n' "$(basename "$f")"
      # Strip carriage returns so the file has one line ending style.
      tr -d '\r' <"$f"
      printf '\n\n'
    } >>"$tmp"
  done <<<"$files"
}

tmp="$(mktemp "${TMPDIR:-/tmp}/casebook-notices.XXXXXX")"
trap 'rm -f "$tmp"' EXIT

cat >"$tmp" <<'EOF'
Casebook third-party notices

The Casebook desktop app and the casebook command line tool include the
software listed below. Each entry reproduces that software's license and
notice files exactly as its authors distribute them. Casebook's own license
is in the LICENSE file that accompanies this one.

EOF

# 1. Go itself.
goroot="$(go env GOROOT)"
section "Go standard library and runtime ($(go env GOVERSION))" "$goroot"

# 2. Go modules linked into either executable, on every release platform.
# -e keeps going when desktop/frontend/dist (embedded) is not built yet;
# module information does not depend on it.
modules=""
cd "$CB_ROOT"
targets=(./cmd/casebook)
if [ -d desktop ]; then
  targets+=(./desktop)
fi
fmt='{{with .Module}}{{if not .Main}}{{.Path}}{{"\t"}}{{.Version}}{{end}}{{end}}'
# The CLI builds with CGO_ENABLED=0 and the macOS desktop app with cgo, so
# list both settings; listing needs no C compiler.
for goos in windows darwin linux; do
  for cgo in 0 1; do
    list="$(GOOS="$goos" CGO_ENABLED="$cgo" go list -e -deps -f "$fmt" "${targets[@]}")" ||
      cb_die "go list failed for GOOS=$goos CGO_ENABLED=$cgo"
    modules="$(printf '%s\n%s\n' "$modules" "$list")"
  done
done
modules="$(printf '%s\n' "$modules" | sed '/^$/d' | LC_ALL=C sort -u)"
count_go=0
while IFS="$(printf '\t')" read -r path ver; do
  [ -n "$path" ] || continue
  dir="$(go mod download -json "$path@$ver" | sed -n 's/^[[:space:]]*"Dir": "\(.*\)",\{0,1\}$/\1/p' | head -n 1)" ||
    cb_die "go mod download failed for $path@$ver"
  if [ -z "$dir" ] || [ ! -d "$dir" ]; then
    problem "module $path@$ver is not in the module cache"
    continue
  fi
  section "Go module $path $ver" "$dir"
  count_go=$((count_go + 1))
done <<<"$modules"

# 3. Production npm packages bundled into the desktop frontend.
count_npm=0
frontend="$CB_ROOT/desktop/frontend"
if [ -f "$frontend/package-lock.json" ]; then
  if [ ! -d "$frontend/node_modules" ]; then
    problem "desktop/frontend/node_modules is missing; run npm ci --ignore-scripts in desktop/frontend first"
  else
    command -v node >/dev/null 2>&1 || cb_die "node is not on PATH (needed to read desktop/frontend/package-lock.json)"
    # Prints "<lock path>\t<name>\t<version>" for every package that is not
    # dev-only. The lock path is relative to desktop/frontend.
    pkgs="$(node -e '
      const lock = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"));
      const out = [];
      for (const [p, meta] of Object.entries(lock.packages || {})) {
        if (!p.startsWith("node_modules/") || meta.dev || meta.devOptional || meta.link) continue;
        const name = meta.name || p.slice(p.lastIndexOf("node_modules/") + "node_modules/".length);
        out.push(p + "\t" + name + "\t" + (meta.version || ""));
      }
      out.sort();
      process.stdout.write(out.join("\n") + (out.length ? "\n" : ""));
    ' "$frontend/package-lock.json")" || cb_die "cannot read desktop/frontend/package-lock.json"
    while IFS="$(printf '\t')" read -r rel name ver; do
      [ -n "$rel" ] || continue
      if [ ! -d "$frontend/$rel" ]; then
        # Optional platform packages (for example native binaries for
        # another OS) are absent by design and never bundled.
        continue
      fi
      section "npm package $name $ver" "$frontend/$rel"
      count_npm=$((count_npm + 1))
    done <<<"$pkgs"
  fi
fi

if [ "$problems" -gt 0 ] && [ "$CB_RELEASE" = "1" ]; then
  cb_die "$problems notice problem(s); see the error lines above"
fi

chmod 0644 "$tmp"
mv -f "$tmp" "$out"
trap - EXIT
cb_info "wrote $out ($count_go Go modules, $count_npm npm packages)"
