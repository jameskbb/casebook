#!/usr/bin/env bash
#
# Documentation consistency gate. CI runs it in the scripts job; the release
# workflow's version job runs it with --release before any build
# (docs/building-releases.md).
#
# Checks:
#   1. No em dash character (U+2014) in any text file: inside the repository's
#      own git work tree, every tracked or new, non-ignored file (git grep
#      --untracked); elsewhere (the test harness fixtures), a recursive grep
#      that skips .git, dist and node_modules.
#   2. Every relative link and image in README.md, ARCHITECTURE.md,
#      CONTRIBUTING.md, SECURITY.md and docs/**/*.md resolves to an existing
#      file or folder. Fenced code blocks and inline code are ignored, as are
#      http, https and mailto links and pure #anchors; a trailing #anchor is
#      dropped. Files another track still has to deliver only warn in
#      development and fail in release mode:
#        docs/mcp-tools.md                  (track B)
#        docs/screenshots/main.png          (track A)
#        docs/screenshots/investigation.png (track A)
#   3. README.md links every guide (the documentation index).
#   4. Secret parity: the names .github/workflows/release.yml reads from the
#      secrets context equal the WINDOWS_* and APPLE_* names that
#      docs/building-releases.md documents.
#   5. FINALIZE(phase 6) markers (HTML comments) are listed as info lines in
#      development; in release mode each one is an error, because Phase 6
#      must reconcile them before a release.
#   6. Action pins: every `uses:` line in .github/workflows/*.yml (and
#      .yaml) that is not a local ./ path names a full 40-hex commit SHA
#      followed by a `# vX...` version comment, so a moved tag cannot change
#      what CI or the release runs (Dependabot keeps the pins current).
#   7. Release version parity (release mode, when CASEBOOK_VERSION is a
#      stable release version vMAJOR.MINOR.PATCH): CHANGELOG.md has a
#      `## MAJOR.MINOR.PATCH (` heading and nothing left under
#      `## Unreleased`; desktop/wails.json's productVersion and the frontend
#      package.json and package-lock.json versions equal MAJOR.MINOR.PATCH.
#      A prerelease or dry-run version skips this check, and so does an
#      unset CASEBOOK_VERSION, with a skip line saying what to set.
#   8. CLAUDE.md and AGENTS.md are identical when either file is present.
#
# Output lines start with ok:, warn:, info:, skip: or error: (stdout).
#
# Exit codes: 0 no errors, 1 at least one error, 2 usage error.
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() {
  cat <<'EOF'
usage: scripts/check-docs.sh [--release]

Checks the docs for em dashes, broken relative links, a complete README
index, secret name parity with .github/workflows/release.yml,
FINALIZE(phase 6) markers, and that every action in .github/workflows is
pinned to a commit SHA. With --release (or CASEBOOK_RELEASE=1), pending
track files and FINALIZE markers are errors instead of warnings.
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

export LC_ALL=C
cd "$CB_ROOT"

errors=0
ok() { printf 'ok: %s\n' "$*"; }
warn() { printf 'warn: %s\n' "$*"; }
info() { printf 'info: %s\n' "$*"; }
skip() { printf 'skip: %s\n' "$*"; }
err() {
  printf 'error: %s\n' "$*"
  errors=$((errors + 1))
}

# The docs set for checks 2 and 5, as repository-relative paths.
doc_files=()
for f in README.md ARCHITECTURE.md CONTRIBUTING.md SECURITY.md; do
  if [ -f "$f" ]; then
    doc_files+=("$f")
  fi
done
if [ -d docs ]; then
  while IFS= read -r f; do
    doc_files+=("${f#./}")
  done < <(find docs -type f -name '*.md' | sort)
fi

# 1. Em dash. The character is built from its UTF-8 bytes so this file
# never contains it.
emdash="$(printf '\342\200\224')"
hits=""
rc=0
if cb_own_git; then
  hits="$(git grep -I -n --untracked -F -e "$emdash" -- . \
    ':(exclude)dist' ':(exclude,glob)**/node_modules/**')" || rc=$?
else
  hits="$(grep -rIn -F --exclude-dir=.git --exclude-dir=dist \
    --exclude-dir=node_modules -e "$emdash" .)" || rc=$?
fi
case "$rc" in
  0)
    while IFS= read -r line; do
      err "em dash in ${line#./}"
    done <<<"$hits"
    ;;
  1) ok "no em dash in any text file" ;;
  *) err "the em dash search failed (exit $rc)" ;;
esac

# 2. Relative links.

# normpath PATH: collapse . and .. segments of a relative path without
# touching the file system.
normpath() {
  local part out="" parts=()
  IFS=/ read -r -a parts <<<"$1"
  for part in ${parts[@]+"${parts[@]}"}; do
    case "$part" in
      "" | .) ;;
      ..)
        case "$out" in
          "" | .. | */..) out="${out:+$out/}.." ;;
          */*) out="${out%/*}" ;;
          *) out="" ;;
        esac
        ;;
      *) out="${out:+$out/}$part" ;;
    esac
  done
  printf '%s\n' "${out:-.}"
}

# pending_track PATH: print the track that still has to deliver PATH, or
# nothing.
pending_track() {
  case "$1" in
    docs/mcp-tools.md) printf 'track B' ;;
    docs/screenshots/main.png | docs/screenshots/investigation.png) printf 'track A' ;;
  esac
}

# links FILE: print "<line>\t<target>" for every Markdown link or image
# target outside fenced code blocks and inline code.
links() {
  awk '
    /^[ \t]*(```|~~~)/ { fence = !fence; next }
    fence { next }
    {
      line = $0
      gsub(/`[^`]*`/, "", line)
      while (match(line, /\]\([^) \t]+\)/)) {
        printf "%d\t%s\n", NR, substr(line, RSTART + 2, RLENGTH - 3)
        line = substr(line, RSTART + RLENGTH)
      }
    }
  ' "$1"
}

link_errors_before=$errors
pending_seen=0
for f in "${doc_files[@]}"; do
  dir="$(dirname "$f")"
  while IFS="$(printf '\t')" read -r lineno target; do
    [ -n "$target" ] || continue
    case "$target" in
      http://* | https://* | mailto:* | "#"*) continue ;;
    esac
    path="${target%%#*}"
    [ -n "$path" ] || continue
    case "$path" in
      /*) rel="$(normpath "${path#/}")" ;;
      *) rel="$(normpath "$dir/$path")" ;;
    esac
    if [ -e "$rel" ]; then
      continue
    fi
    track="$(pending_track "$rel")"
    if [ -n "$track" ]; then
      pending_seen=$((pending_seen + 1))
      if [ "$CB_RELEASE" = "1" ]; then
        err "pending $rel ($track) is linked from $f:$lineno but missing in release mode"
      else
        warn "pending $rel ($track) linked from $f:$lineno"
      fi
    else
      err "broken link in $f:$lineno: $target"
    fi
  done < <(links "$f")
done
if [ "$errors" -eq "$link_errors_before" ]; then
  if [ "$pending_seen" -gt 0 ]; then
    ok "relative links resolve in ${#doc_files[@]} docs (except $pending_seen pending track link(s))"
  else
    ok "relative links resolve in ${#doc_files[@]} docs"
  fi
fi

# 3. README index.
index_targets=(
  docs/architecture.md
  docs/development.md
  docs/building-releases.md
  docs/mcp.md
  docs/claude-code.md
  docs/codex.md
  docs/privacy.md
  docs/updating.md
  CONTRIBUTING.md
  SECURITY.md
  docs/decisions/0011-license.md
)
if [ ! -f README.md ]; then
  err "README.md is missing"
else
  index_errors_before=$errors
  for t in "${index_targets[@]}"; do
    if ! grep -q -F -e "]($t)" -e "]($t#" README.md; then
      err "README.md does not link $t"
    fi
  done
  if [ "$errors" -eq "$index_errors_before" ]; then
    ok "README.md links all ${#index_targets[@]} guides"
  fi
fi

# 4. Secret parity between release.yml and docs/building-releases.md.
workflow=.github/workflows/release.yml
guide=docs/building-releases.md
if [ ! -f "$workflow" ]; then
  if [ "$CB_RELEASE" = "1" ]; then
    err "$workflow is missing in release mode"
  else
    skip "$workflow not present; secret parity not checked"
  fi
elif [ ! -f "$guide" ]; then
  err "$guide is missing, so the secrets $workflow uses are undocumented"
else
  used="$(grep -o -E 'secrets\.[A-Z][A-Z0-9_]*' "$workflow" | sed 's/^secrets\.//' | sort -u)" || true
  documented="$(grep -o -E '(^|[^A-Za-z0-9_])(WINDOWS|APPLE)_[A-Z0-9_]*[A-Z0-9]' "$guide" |
    sed -E 's/^[^A-Z]//' | sort -u)" || true
  parity_errors_before=$errors
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    if ! grep -q -x -F -e "$name" <<<"$documented" && ! grep -q -w -F -e "$name" "$guide"; then
      err "secret $name is used in $workflow but not documented in $guide"
    fi
  done <<<"$used"
  while IFS= read -r name; do
    [ -n "$name" ] || continue
    if ! grep -q -x -F -e "$name" <<<"$used"; then
      err "secret $name is documented in $guide but not used in $workflow"
    fi
  done <<<"$documented"
  if [ "$errors" -eq "$parity_errors_before" ]; then
    count=0
    if [ -n "$used" ]; then
      count="$(printf '%s\n' "$used" | wc -l | tr -d ' ')"
    fi
    ok "$count secret name(s) in $workflow match $guide"
  fi
fi

# 5. FINALIZE(phase 6) markers.
markers=""
if [ "${#doc_files[@]}" -gt 0 ]; then
  markers="$(grep -n -E '<!--[[:space:]]*FINALIZE\(phase 6\)' "${doc_files[@]}" /dev/null)" || true
fi
marker_count=0
if [ -n "$markers" ]; then
  marker_count="$(printf '%s\n' "$markers" | wc -l | tr -d ' ')"
fi
info "$marker_count FINALIZE(phase 6) markers"
if [ -n "$markers" ]; then
  while IFS= read -r line; do
    loc="$(printf '%s\n' "$line" | cut -d: -f1-2)"
    if [ "$CB_RELEASE" = "1" ]; then
      err "FINALIZE(phase 6) marker at $loc must be reconciled before a release"
    else
      info "  $loc"
    fi
  done <<<"$markers"
fi

# 6. Third-party actions pinned to commit SHAs.
workflows=()
for f in .github/workflows/*.yml .github/workflows/*.yaml; do
  [ -f "$f" ] && workflows+=("$f")
done
if [ "${#workflows[@]}" -eq 0 ]; then
  skip "no workflows in .github/workflows; action pins not checked"
else
  pin_errors_before=$errors
  pinned=0
  uses_lines="$(grep -n -H -E '^[[:space:]]*(-[[:space:]]+)?uses:' "${workflows[@]}")" || true
  while IFS= read -r line; do
    [ -n "$line" ] || continue
    loc="$(printf '%s\n' "$line" | cut -d: -f1-2)"
    value="$(printf '%s\n' "$line" | cut -d: -f3- | sed -E 's/^[[:space:]]*(-[[:space:]]+)?uses:[[:space:]]*//')"
    case "$value" in
      ./*) continue ;;
    esac
    if printf '%s\n' "$value" | grep -q -E '^[A-Za-z0-9_.-]+/[A-Za-z0-9_./-]+@[0-9a-f]{40}[[:space:]]+#[[:space:]]*v[0-9]'; then
      pinned=$((pinned + 1))
    else
      err "action at $loc is not pinned to a commit SHA with a version comment: $value"
    fi
  done <<<"$uses_lines"
  if [ "$errors" -eq "$pin_errors_before" ]; then
    ok "$pinned third-party action reference(s) in .github/workflows are pinned to commit SHAs"
  fi
fi

# 7. Release version parity: the files that carry a version agree with the
# tag. Release mode only; see docs/building-releases.md, Making a release.
if [ "$CB_RELEASE" = "1" ]; then
  release_version="${CASEBOOK_VERSION:-}"
  stable_re='^v([0-9]+\.[0-9]+\.[0-9]+)$'
  if [ -z "$release_version" ]; then
    skip "CASEBOOK_VERSION is not set; version parity not checked (set it to the tag, for example CASEBOOK_VERSION=v1.2.3)"
  elif ! [[ "$release_version" =~ $stable_re ]]; then
    skip "version $release_version is not a stable release version; version parity not checked"
  else
    numeric="${BASH_REMATCH[1]}"
    parity_errors_before=$errors

    # CHANGELOG.md: the version has its own section, and nothing is left
    # under Unreleased (an empty Unreleased heading is fine).
    if [ ! -f CHANGELOG.md ]; then
      err "CHANGELOG.md is missing; a release needs a ## $numeric section"
    else
      if ! grep -q -E "^## $(printf '%s' "$numeric" | sed 's/\./\\./g') \(" CHANGELOG.md; then
        err "CHANGELOG.md has no \"## $numeric (YYYY-MM-DD)\" section; rename ## Unreleased before tagging v$numeric"
      fi
      unreleased_entries="$(awk '
        /^## / { in_unreleased = ($0 == "## Unreleased") ; next }
        in_unreleased && /^(- |### )/ { n++ }
        END { print n + 0 }' CHANGELOG.md)"
      if [ "$unreleased_entries" -gt 0 ]; then
        err "CHANGELOG.md still lists $unreleased_entries entries under ## Unreleased; move them into ## $numeric"
      fi
    fi

    # desktop/wails.json: productVersion is the Windows and macOS product
    # version field. The workflow rewrites it on the runner, but the
    # committed value is what a source build and git blame show.
    if [ ! -f desktop/wails.json ]; then
      err "desktop/wails.json is missing; a release needs its productVersion set to $numeric"
    elif ! grep -q -E "\"productVersion\":[[:space:]]*\"$(printf '%s' "$numeric" | sed 's/\./\\./g')\"" desktop/wails.json; then
      err "desktop/wails.json productVersion is not $numeric"
    fi

    # The frontend package and its lock file (npm version writes both).
    for f in desktop/frontend/package.json desktop/frontend/package-lock.json; do
      if [ ! -f "$f" ]; then
        err "$f is missing; a release needs its version set to $numeric"
        continue
      fi
      found="$(grep -m1 -E '^[[:space:]]*"version":' "$f" | sed -E 's/.*"version":[[:space:]]*"([^"]*)".*/\1/')"
      if [ "$found" != "$numeric" ]; then
        err "$f version is ${found:-missing}, not $numeric (run: npm --prefix desktop/frontend version $numeric --no-git-tag-version)"
      fi
    done

    if [ "$errors" -eq "$parity_errors_before" ]; then
      ok "CHANGELOG.md, desktop/wails.json and the frontend package carry version $numeric"
    fi
  fi
fi

# 8. Shared repository instructions must not drift between AI clients.
if [ -e CLAUDE.md ] || [ -e AGENTS.md ]; then
  if [ ! -f CLAUDE.md ] || [ ! -f AGENTS.md ]; then
    err "CLAUDE.md and AGENTS.md must both exist and be identical"
  elif cmp -s CLAUDE.md AGENTS.md; then
    ok "CLAUDE.md and AGENTS.md are identical"
  else
    err "CLAUDE.md and AGENTS.md differ; update both files together"
  fi
fi

if [ "$errors" -gt 0 ]; then
  printf 'error: docs check failed with %d error(s)\n' "$errors"
  exit 1
fi
if [ "$CB_RELEASE" = "1" ]; then
  ok "docs check passed (release mode)"
else
  ok "docs check passed"
fi
