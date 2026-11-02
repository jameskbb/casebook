#!/usr/bin/env bash
#
# Package the macOS release: take a Wails-built Casebook.app, put the
# universal casebook CLI inside it at Contents/MacOS/bin/casebook
# (docs/decisions/0012-cli-placement.md), sign it (the nested CLI first,
# then the bundle), and when asked to, notarize and staple the app. It then
# wraps the app in a DMG with an Applications shortcut and signs, notarizes
# and staples the DMG as well.
#
# The release workflow calls it exactly so:
#   scripts/package-macos.sh --release --app desktop/build/bin/Casebook.app --out dist/out
# and finds exactly one file in the out directory afterwards:
#   Casebook-<version>-macos-universal.dmg
#
# Environment inputs (an unset variable counts as empty):
#   CASEBOOK_VERSION, CASEBOOK_COMMIT  version stamp (scripts/lib.sh). Empty:
#                                      git describe and git rev-parse, else
#                                      dev and no commit. Both are passed on
#                                      to the CLI build so it matches the app.
#   CASEBOOK_RELEASE                   1 behaves like --release.
#   CASEBOOK_SIGN_IDENTITY             Developer ID Application identity.
#                                      Empty: ad-hoc signature only, the DMG
#                                      is not signed and notarization is
#                                      skipped (with a skip line).
#   CASEBOOK_KEYCHAIN                  keychain file holding the identity,
#                                      passed to codesign --keychain. Empty:
#                                      the default keychain search list.
#   CASEBOOK_NOTARIZE                  1 notarizes and staples the app, then
#                                      the DMG. Any other value: skipped
#                                      (with a skip line).
#   APPLE_ID, APPLE_APP_PASSWORD,      Apple ID, app-specific password and
#   APPLE_TEAM_ID                      team for notarytool. Used only when
#                                      notarizing with an identity; then an
#                                      empty value is an error (exit 1).
#
# APPLE_APP_PASSWORD is never printed: it reaches notarytool only as a
# command argument, and --dry-run shows it as ***. Never enable xtrace here.
#
# The app bundle is modified in place (the CLI is added and the bundle is
# re-signed). Without macOS the script skips with exit 0 in development and
# exits 1 in release mode; --dry-run prints the command plan on any OS
# without running anything.
#
# Exit codes: 0 packaged or skipped, 1 failure, missing part in release mode
# or incomplete notarization credentials, 2 usage error.
set -euo pipefail

# shellcheck source=scripts/lib.sh
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

usage() {
  cat <<'EOF'
usage: scripts/package-macos.sh --app PATH [--out DIR] [--release] [--dry-run]

  --app PATH   the Wails-built Casebook.app (modified in place)
  --out DIR    directory for the DMG (default: dist/)
  --release    fail instead of skipping when macOS or cmd/casebook is
               missing (same as CASEBOOK_RELEASE=1)
  --dry-run    print the command plan instead of running it (any OS)
  -h, --help   show this help

Signing and notarization read CASEBOOK_SIGN_IDENTITY, CASEBOOK_KEYCHAIN,
CASEBOOK_NOTARIZE, APPLE_ID, APPLE_APP_PASSWORD and APPLE_TEAM_ID; see the
header of this script for what each one does and what happens when it is
empty.
EOF
}

app=""
out=""
dry_run=0
while [ "$#" -gt 0 ]; do
  case "$1" in
    --app)
      if [ "$#" -lt 2 ] || [ -z "$2" ]; then
        cb_usage_die "--app needs a value"
      fi
      app="$2"
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
    --dry-run)
      dry_run=1
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

if [ -z "$app" ]; then
  usage >&2
  cb_usage_die "--app is required"
fi

# Validate the version inputs first, so a bad value is reported even when
# the rest of the run is skipped.
version="$(cb_version)"
commit="$(cb_commit)"
marker="$(cb_marker)"

# run CMD...: execute CMD, or in dry-run mode print it as one line prefixed
# with "dry-run: ". The Apple app-specific password is replaced by *** in
# that line wherever it appears.
run() {
  if [ "$dry_run" = "1" ]; then
    local arg line="dry-run:" pw="${APPLE_APP_PASSWORD:-}"
    local safe='^[A-Za-z0-9_./:=+@,%*-]+$'
    for arg in "$@"; do
      if [ -n "$pw" ]; then
        arg="${arg//"$pw"/***}"
      fi
      if [[ "$arg" =~ $safe ]]; then
        line="$line $arg"
      else
        line="$line $(printf '%q' "$arg")"
      fi
    done
    printf '%s\n' "$line"
    return 0
  fi
  "$@"
}

# plan_check DESCRIPTION: in dry-run mode, print a check that a real run makes.
plan_check() {
  printf 'dry-run: check: %s\n' "$*"
}

# 1. The real run needs the macOS tools.
if [ "$(uname -s)" != "Darwin" ] && [ "$dry_run" != "1" ]; then
  if [ "$CB_RELEASE" = "1" ]; then
    cb_die "macOS packaging needs macOS (lipo, codesign, hdiutil)"
  fi
  cb_skip "macOS packaging needs macOS (lipo, codesign, hdiutil)"
  exit 0
fi

# 2. The CLI is what makes the bundle useful; without it there is nothing to
# package yet.
if ! cb_require_part cmd/casebook "track B"; then
  exit 0
fi

# 3. The app bundle must exist and hold the desktop executable.
app="$(cb_abs_path "$app")"
app="${app%/}"
gui="$app/Contents/MacOS/$CB_GUI_NAME"
if [ ! -d "$app" ]; then
  cb_die "app bundle not found: $app"
fi
app="$(cd "$app" && pwd -P)" || cb_die "cannot resolve $app"
gui="$app/Contents/MacOS/$CB_GUI_NAME"
if [ ! -f "$gui" ]; then
  cb_die "desktop executable not found: $gui"
fi

identity="${CASEBOOK_SIGN_IDENTITY:-}"
keychain="${CASEBOOK_KEYCHAIN:-}"
notarize=0
if [ "${CASEBOOK_NOTARIZE:-}" = "1" ]; then
  notarize=1
fi
# A requested notarization with an identity but incomplete Apple
# credentials is a configuration error: fail before building anything.
if [ "$notarize" = "1" ] && [ -n "$identity" ]; then
  for var in APPLE_ID APPLE_APP_PASSWORD APPLE_TEAM_ID; do
    if [ -z "${!var:-}" ]; then
      cb_die "CASEBOOK_NOTARIZE=1 with a signing identity needs $var (APPLE_ID, APPLE_APP_PASSWORD and APPLE_TEAM_ID)"
    fi
  done
fi
# Every signing call names the keychain when one is given.
keychain_args=()
if [ -n "$keychain" ]; then
  keychain_args=(--keychain "$keychain")
fi

if [ "$dry_run" != "1" ]; then
  tools="lipo codesign hdiutil ditto"
  if [ "$notarize" = "1" ] && [ -n "$identity" ]; then
    tools="$tools xcrun plutil spctl"
  fi
  for tool in $tools; do
    command -v "$tool" >/dev/null 2>&1 || cb_die "$tool is not on PATH (install the Xcode command line tools)"
  done
fi

if [ -z "$out" ]; then
  out="$CB_ROOT/dist"
else
  out="$(cb_abs_path "$out")"
  out="${out%/}"
fi
dmg="$out/$CB_GUI_NAME-$version-macos-universal.dmg"
cli_dir="$app/Contents/MacOS/$CB_CLI_SUBDIR"
cli="$cli_dir/$CB_CLI_NAME"

# 4. Build the two CLI slices with the same stamp as the app.
work=""
trap '[ -z "$work" ] || rm -rf "$work"' EXIT
work="$(mktemp -d "${TMPDIR:-/tmp}/casebook-macos.XXXXXX")"

export CASEBOOK_VERSION="$version" CASEBOOK_COMMIT="$commit"
build_args=()
if [ "$CB_RELEASE" = "1" ]; then
  build_args+=(--release)
fi
cb_info "building the universal $CB_CLI_NAME CLI ($version)"
for arch in arm64 amd64; do
  run "$CB_ROOT/scripts/build-cli.sh" --os darwin --arch "$arch" \
    --out "$work/$CB_CLI_NAME-$arch" ${build_args[@]+"${build_args[@]}"}
done

# 5. One universal binary inside the bundle (ADR-0012).
run mkdir -p "$cli_dir"
run lipo -create -output "$cli" "$work/$CB_CLI_NAME-arm64" "$work/$CB_CLI_NAME-amd64"
run chmod 755 "$cli"

# 6. The two executables must stay two files, carry both architectures and
# carry the same version stamp. The check looks for the stamp marker
# (casebook-version=<version>), which only the -X flags produce; the bare
# version string could also come from Go's build information.
lower() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}
if [ "$(lower "$gui")" = "$(lower "$cli")" ]; then
  cb_die "$gui and $cli differ only by case and would overwrite each other on APFS (see ADR-0012)"
fi
if [ "$dry_run" = "1" ]; then
  plan_check "lipo -archs $cli lists arm64 and x86_64"
  plan_check "cmp -s $gui $cli reports different files (ADR-0012)"
  plan_check "$gui and $cli both contain the stamp marker $marker"
else
  archs="$(lipo -archs "$cli")" || cb_die "lipo -archs failed for $cli"
  case " $archs " in
    *" arm64 "*) ;;
    *) cb_die "$cli lacks the arm64 slice (lipo -archs: $archs)" ;;
  esac
  case " $archs " in
    *" x86_64 "*) ;;
    *) cb_die "$cli lacks the x86_64 slice (lipo -archs: $archs)" ;;
  esac
  if [ "$gui" -ef "$cli" ] || cmp -s "$gui" "$cli"; then
    cb_die "$gui and $cli are the same file; the CLI must not replace the desktop app (see ADR-0012)"
  fi
  for exe in "$gui" "$cli"; do
    grep -a -q -F -- "$marker" "$exe" ||
      cb_die "$exe does not contain the stamp marker $marker (is internal/version linked and stamped?)"
  done
fi

# 7. Sign: inner code first, so the bundle seal covers it. Adding the CLI
# after Wails built the bundle invalidates its seal, and a downloaded app
# with a broken seal is reported as damaged, so re-signing is mandatory.
entitlements="$CB_ROOT/packaging/macos/entitlements.plist"
if [ -n "$identity" ]; then
  [ -f "$entitlements" ] || cb_die "entitlements file not found: $entitlements"
  cb_info "signing with the Developer ID identity (hardened runtime)"
  run codesign --force --options runtime --timestamp ${keychain_args[@]+"${keychain_args[@]}"} \
    --sign "$identity" "$cli"
  run codesign --force --options runtime --timestamp ${keychain_args[@]+"${keychain_args[@]}"} \
    --entitlements "$entitlements" --sign "$identity" "$app"
else
  cb_skip "Developer ID signing (CASEBOOK_SIGN_IDENTITY not set); ad-hoc signature only"
  run codesign --force --sign - "$cli"
  run codesign --force --sign - "$app"
fi
run codesign --verify --deep --strict --verbose=2 "$app"

# The CLI must run and report the version. This runs after signing so the
# arm64 slice carries a valid signature on Apple silicon.
if [ "$dry_run" = "1" ]; then
  plan_check "$cli --version (or $cli version) prints $version"
else
  cli_out="$("$cli" --version 2>&1)" || cli_out=""
  case "$cli_out" in
    *"$version"*) ;;
    *)
      cli_out="$("$cli" version 2>&1)" || cli_out=""
      case "$cli_out" in
        *"$version"*) ;;
        *) cb_die "$cli --version and $cli version do not print $version" ;;
      esac
      ;;
  esac
fi

# notarize_file PATH LABEL: submit PATH (a zip of the app, or the DMG) to
# the Apple notary service and fail unless the result is Accepted.
# notarytool's exit code alone does not say Accepted; the status is read
# from its JSON output.
apple_args=()
if [ "$notarize" = "1" ] && [ -n "$identity" ]; then
  apple_args=(--apple-id "$APPLE_ID" --password "$APPLE_APP_PASSWORD" --team-id "$APPLE_TEAM_ID")
fi
notarize_file() {
  local file="$1" label="$2" result submit_rc status submission
  cb_info "submitting $label for notarization (waits up to 30 minutes)"
  if [ "$dry_run" = "1" ]; then
    run xcrun notarytool submit "$file" "${apple_args[@]}" --wait --timeout 30m --output-format json
    plan_check "plutil -extract status raw -o - on the notarytool JSON for $label is Accepted (else xcrun notarytool log <id> is printed and the run fails)"
    return 0
  fi
  result="$work/notarize-$label.json"
  submit_rc=0
  run xcrun notarytool submit "$file" "${apple_args[@]}" --wait --timeout 30m \
    --output-format json >"$result" || submit_rc=$?
  status="$(plutil -extract status raw -o - "$result" 2>/dev/null)" || status=""
  submission="$(plutil -extract id raw -o - "$result" 2>/dev/null)" || submission=""
  if [ "$status" != "Accepted" ]; then
    cb_info "notarization status: ${status:-unknown} (notarytool exit $submit_rc)"
    if [ -n "$submission" ]; then
      cb_info "notarization log for submission $submission:"
      run xcrun notarytool log "$submission" "${apple_args[@]}" >&2 || true
    else
      cat "$result" >&2 || true
    fi
    cb_die "notarization of $file was not Accepted"
  fi
  cb_info "notarization of $label Accepted (submission $submission)"
}

# 8. Notarize and staple the app itself, before it goes into the DMG. The
# ticket must travel with the app: after drag-to-Applications the DMG is
# gone, and an offline Mac can only check a ticket stapled to the bundle.
if [ "$notarize" != "1" ]; then
  cb_skip "notarization not requested (CASEBOOK_NOTARIZE is not 1)"
elif [ -z "$identity" ]; then
  cb_skip "notarization needs a Developer ID signature"
else
  app_zip="$work/$CB_GUI_NAME-app.zip"
  run ditto -c -k --sequesterRsrc --keepParent "$app" "$app_zip"
  notarize_file "$app_zip" app
  run xcrun stapler staple "$app"
  run xcrun stapler validate "$app"
  run spctl --assess --type execute --verbose "$app"
fi

# 9. The DMG: the (stapled) app and an Applications shortcut.
stage="$work/dmg"
run mkdir -p "$stage"
run ditto "$app" "$stage/$CB_GUI_NAME.app"
run ln -s /Applications "$stage/Applications"

# 10. The out directory; dist/ keeps ignoring itself.
case "$out/" in
  "$CB_ROOT/dist/"*)
    if [ "$dry_run" != "1" ]; then
      cb_dist_dir >/dev/null
    fi
    ;;
esac
run mkdir -p "$out"
run rm -f "$dmg"

attempt=1
while :; do
  if run hdiutil create -volname "$CB_GUI_NAME" -srcfolder "$stage" -ov -format UDZO "$dmg"; then
    break
  fi
  if [ "$attempt" -ge 3 ]; then
    cb_die "hdiutil create failed 3 times for $dmg"
  fi
  cb_info "hdiutil create failed (attempt $attempt of 3); retrying in 5 seconds"
  sleep 5
  attempt=$((attempt + 1))
done

# 11. Sign the DMG itself when there is an identity.
if [ -n "$identity" ]; then
  run codesign --force --timestamp ${keychain_args[@]+"${keychain_args[@]}"} \
    --sign "$identity" "$dmg"
fi

# 12. Notarize and staple the DMG too, so the download itself opens cleanly.
if [ "$notarize" = "1" ] && [ -n "$identity" ]; then
  notarize_file "$dmg" dmg
  run xcrun stapler staple "$dmg"
  run xcrun stapler validate "$dmg"
  run spctl --assess --type open --context context:primary-signature --verbose "$dmg"
fi

if [ "$dry_run" = "1" ]; then
  cb_info "dry-run: nothing was built, signed or written; a real run produces $dmg"
else
  cb_info "packaged: $dmg"
fi
