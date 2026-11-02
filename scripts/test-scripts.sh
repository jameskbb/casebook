#!/usr/bin/env bash
#
# Self-test harness for the Casebook build scripts. It never touches the real
# tree's dist/ or desktop/: every case builds a throwaway fixture module in a
# temporary directory (removed on exit) that holds a copy of scripts/,
# packaging/ and internal/version/version.go, plus small stand-ins for
# cmd/casebook and desktop/ when a case asks for them. Fixtures are not git
# repositories, so they also exercise the dev version fallback.
#
# Usage: scripts/test-scripts.sh [case-name...]   (no names runs every case)
#
# Adding a case (later plans do this):
#   1. Write a function named case_<name>. It runs in its own subshell with
#      `set -euo pipefail`; any failing command or helper fails the case.
#      Build fixtures with make_fixture_repo "$(new_fixture_dir)" [flags],
#      run commands with assert_rc (output lands in $OUT) and check text with
#      assert_contains.
#   2. Append <name> to the CASES array at the bottom of this file.
#
# Exit codes: 0 all cases passed, 1 at least one case failed, 2 usage error.
set -euo pipefail

TS_REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
TS_WORK="$(mktemp -d "${TMPDIR:-/tmp}/casebook-script-tests.XXXXXX")"
trap 'rm -rf "$TS_WORK"' EXIT

# Fixture builds are offline and self-contained.
export GOWORK=off GOTOOLCHAIN=local
# The caller's release mode or version overrides must not leak into cases.
unset CASEBOOK_RELEASE CASEBOOK_VERSION CASEBOOK_COMMIT \
  CASEBOOK_EXPECT_VERSION CASEBOOK_EXPECT_COMMIT

# Combined stdout and stderr of the last assert_rc.
OUT=""

# fail MESSAGE: fail the current case.
fail() {
  printf 'assertion failed: %s\n' "$*"
  exit 1
}

# new_fixture_dir: create and print a fresh, empty fixture directory, with
# symlinks resolved (macOS TMPDIR lives under /var -> /private/var) so its
# paths match the scripts' pwd -P based CB_ROOT.
new_fixture_dir() {
  local d
  d="$(mktemp -d "$TS_WORK/fixture.XXXXXX")"
  (cd "$d" && pwd -P)
}

# make_fixture_repo DIR [--with-cli] [--with-desktop]: build a fixture module
# in DIR with the real scripts and internal/version, optionally with a
# cmd/casebook stand-in (prints version.String()) and a desktop/ stand-in
# (prints version.Version; no Wails import so it cross-compiles offline).
make_fixture_repo() {
  local dir="$1"
  shift
  mkdir -p "$dir/internal/version"
  cp -R "$TS_REPO/scripts" "$dir/scripts"
  if [ -d "$TS_REPO/packaging" ]; then
    cp -R "$TS_REPO/packaging" "$dir/packaging"
  fi
  cp "$TS_REPO/internal/version/version.go" "$dir/internal/version/version.go"
  printf 'module github.com/jameskbb/casebook\n\ngo 1.27\n' >"$dir/go.mod"
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --with-cli)
        mkdir -p "$dir/cmd/casebook"
        cat >"$dir/cmd/casebook/main.go" <<'EOF'
package main

import (
	"fmt"

	"github.com/jameskbb/casebook/internal/version"
)

func main() { fmt.Println(version.String()) }
EOF
        ;;
      --with-desktop)
        mkdir -p "$dir/desktop"
        cat >"$dir/desktop/main.go" <<'EOF'
package main

import (
	"fmt"

	"github.com/jameskbb/casebook/internal/version"
)

func main() { fmt.Println("desktop " + version.Version) }
EOF
        ;;
      *) fail "make_fixture_repo: unknown flag $1" ;;
    esac
    shift
  done
}

# assert_rc EXPECTED CMD...: run CMD, keep its combined output in OUT, and
# fail with that output when the exit code is not EXPECTED.
assert_rc() {
  local want="$1" rc=0
  shift
  OUT="$("$@" 2>&1)" || rc=$?
  if [ "$rc" -ne "$want" ]; then
    printf '%s\n' "$OUT"
    fail "expected exit $want, got $rc: $*"
  fi
}

# assert_contains HAYSTACK NEEDLE: fail unless HAYSTACK contains NEEDLE.
assert_contains() {
  case "$1" in
    *"$2"*) ;;
    *)
      printf '%s\n' "$1"
      fail "output does not contain: $2"
      ;;
  esac
}

# assert_not_contains HAYSTACK NEEDLE: fail when HAYSTACK contains NEEDLE.
assert_not_contains() {
  case "$1" in
    *"$2"*)
      printf '%s\n' "$1"
      fail "output should not contain: $2"
      ;;
  esac
}

# assert_eq GOT WANT: fail unless the two strings are equal.
assert_eq() {
  if [ "$1" != "$2" ]; then
    fail "got '$1', want '$2'"
  fi
}

case_version_dev_fallback() {
  local fx
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx"
  assert_eq "$("$fx/scripts/version.sh" version)" "dev"
  # An empty commit still prints its trailing newline.
  assert_eq "$("$fx/scripts/version.sh" commit | od -An -c | tr -d ' ')" '\n'
  assert_eq "$("$fx/scripts/version.sh" numeric)" "0.0.0"
}

case_version_override_and_validation() {
  local fx bad
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx"
  assert_eq "$(CASEBOOK_VERSION=v1.2.3-rc.1 "$fx/scripts/version.sh" version)" "v1.2.3-rc.1"
  assert_eq "$(CASEBOOK_VERSION=v1.2.3-rc.1 "$fx/scripts/version.sh" numeric)" "1.2.3"
  for bad in 'v1 -X evil' 'v1"x' "v1'x" 'v1;x'; do
    assert_rc 2 env CASEBOOK_VERSION="$bad" "$fx/scripts/version.sh" version
    assert_contains "$OUT" "invalid version"
  done
  assert_rc 2 env CASEBOOK_COMMIT=xyz "$fx/scripts/version.sh" commit
  assert_rc 2 "$fx/scripts/version.sh" bogus
  assert_rc 0 env CASEBOOK_VERSION=v1.2.3 CASEBOOK_COMMIT=abc1234 "$fx/scripts/version.sh" ldflags
  assert_contains "$OUT" "internal/version.Version=v1.2.3"
  assert_contains "$OUT" "internal/version.Commit=abc1234"
  assert_contains "$OUT" "internal/version.Marker=casebook-version=v1.2.3"
  assert_eq "$(CASEBOOK_VERSION=v1.2.3 "$fx/scripts/version.sh" marker)" "casebook-version=v1.2.3"
}

case_build_cli_skip_and_release() {
  local fx
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx"
  assert_rc 0 "$fx/scripts/build-cli.sh"
  assert_contains "$OUT" "skip: cmd/casebook not present yet (track B)"
  assert_rc 1 "$fx/scripts/build-cli.sh" --release
  assert_contains "$OUT" "missing track B (cmd/casebook) in release mode"
  assert_rc 1 env CASEBOOK_RELEASE=1 "$fx/scripts/build-cli.sh"
  assert_rc 2 "$fx/scripts/build-cli.sh" --os plan9
  assert_rc 2 "$fx/scripts/build-cli.sh" --arch 386
  assert_rc 2 "$fx/scripts/build-cli.sh" --bogus
  [ ! -e "$fx/dist" ] || fail "a skipped build created dist/"
}

case_build_cli_stamps_binary() {
  local fx exe
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli
  assert_rc 0 env CASEBOOK_VERSION=v9.8.7 CASEBOOK_COMMIT=abc1234 \
    "$fx/scripts/build-cli.sh" --release --out "$fx/out/cb"
  assert_contains "$OUT" "built: $fx/out/cb (v9.8.7)"
  assert_eq "$("$fx/out/cb")" "v9.8.7 (abc1234)"
  assert_rc 0 env CASEBOOK_VERSION=v9.8.7 CASEBOOK_COMMIT=abc1234 \
    "$fx/scripts/build-cli.sh" --os windows --arch arm64
  exe="$fx/dist/casebook-windows-arm64.exe"
  [ -f "$exe" ] || fail "missing $exe"
  grep -a -q -F casebook-version=v9.8.7 "$exe" || fail "$exe does not contain the stamp marker"
  assert_eq "$(cat "$fx/dist/.gitignore")" "*"
}

# The release checks grep executables for casebook-version=<version>. That
# text must come only from a working -X flag: with the Marker path broken,
# the binary must not contain it (with -trimpath the -ldflags string is not
# recorded in the build information either), although the bare version is
# still stamped through Version.
case_stamp_marker_needs_x_flag() {
  local fx
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli
  sed 's|/internal/version\.Marker=|/internal/version.NoSuchMarker=|' "$fx/scripts/lib.sh" >"$fx/lib.sh.new"
  mv "$fx/lib.sh.new" "$fx/scripts/lib.sh"
  grep -q 'version\.NoSuchMarker=' "$fx/scripts/lib.sh" || fail "could not break the Marker path in the fixture"
  assert_rc 0 env CASEBOOK_VERSION=v9.8.7 CASEBOOK_COMMIT=abc1234 \
    "$fx/scripts/build-cli.sh" --release --out "$fx/out/cb"
  grep -a -q -F v9.8.7 "$fx/out/cb" || fail "the fixture lost the Version stamp"
  if grep -a -q -F casebook-version=v9.8.7 "$fx/out/cb"; then
    fail "the stamp marker is present although its -X path is wrong"
  fi
}

case_stub_frontend_dist() {
  local fx before
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx"
  assert_rc 0 "$fx/scripts/stub-frontend-dist.sh"
  assert_contains "$OUT" "skip: desktop/frontend not present yet (track A)"

  # Empty frontend directory, release mode: refuse and write nothing.
  mkdir -p "$fx/desktop/frontend"
  assert_rc 1 "$fx/scripts/stub-frontend-dist.sh" --release
  assert_rc 1 env CASEBOOK_RELEASE=1 "$fx/scripts/stub-frontend-dist.sh"
  [ ! -e "$fx/desktop/frontend/dist" ] || fail "release mode created dist/"

  # Development: write the placeholder, then leave it alone on a rerun.
  assert_rc 0 "$fx/scripts/stub-frontend-dist.sh"
  [ -f "$fx/desktop/frontend/dist/index.html" ] || fail "placeholder index.html not written"
  assert_rc 0 "$fx/scripts/stub-frontend-dist.sh"
  assert_contains "$OUT" "ok: desktop/frontend/dist already populated"

  # An existing real build stays byte-identical and gains no placeholder.
  rm -rf "$fx/desktop/frontend/dist"
  mkdir -p "$fx/desktop/frontend/dist"
  printf 'real build\n' >"$fx/desktop/frontend/dist/app.js"
  cp "$fx/desktop/frontend/dist/app.js" "$fx/app.js.before"
  assert_rc 0 "$fx/scripts/stub-frontend-dist.sh"
  assert_rc 0 "$fx/scripts/stub-frontend-dist.sh" --release
  cmp -s "$fx/app.js.before" "$fx/desktop/frontend/dist/app.js" || fail "existing dist file changed"
  [ ! -e "$fx/desktop/frontend/dist/index.html" ] || fail "placeholder written over a real build"
  before="$(ls -A "$fx/desktop/frontend/dist")"
  assert_eq "$before" "app.js"
}

case_preflight_modes() {
  local fx
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx"
  assert_rc 0 "$fx/scripts/preflight.sh"
  assert_eq "$(printf '%s\n' "$OUT" | grep -c '^missing: ')" "3"
  assert_contains "$OUT" "missing: cmd/casebook (track B)"
  assert_contains "$OUT" "missing: desktop/wails.json (track A)"
  assert_contains "$OUT" "missing: desktop/frontend/package.json (track A)"
  assert_rc 1 "$fx/scripts/preflight.sh" --release
  assert_rc 1 env CASEBOOK_RELEASE=1 "$fx/scripts/preflight.sh"
  assert_rc 2 "$fx/scripts/preflight.sh" --bogus

  # Every part present: release mode passes.
  mkdir -p "$fx/cmd/casebook" "$fx/desktop/frontend"
  printf '{}\n' >"$fx/desktop/wails.json"
  printf '{}\n' >"$fx/desktop/frontend/package.json"
  assert_rc 0 "$fx/scripts/preflight.sh" --release
  assert_eq "$(printf '%s\n' "$OUT" | grep -c '^ok: ')" "3"
}

case_build_windows_skip_and_release() {
  local fx
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx"
  assert_rc 0 "$fx/scripts/build-windows.sh"
  assert_contains "$OUT" "skip: desktop not present yet (track A)"
  assert_contains "$OUT" "skip: cmd/casebook not present yet (track B)"
  [ ! -e "$fx/dist" ] || fail "a fully skipped build created dist/"
  assert_rc 1 "$fx/scripts/build-windows.sh" --release
  assert_contains "$OUT" "missing track A (desktop) in release mode"
  assert_rc 1 env CASEBOOK_RELEASE=1 "$fx/scripts/build-windows.sh"
  assert_rc 2 "$fx/scripts/build-windows.sh" --arch x86
  assert_rc 2 "$fx/scripts/build-windows.sh" --bogus
}

case_build_windows_fixture() {
  local fx arch gui cli
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli --with-desktop
  assert_rc 0 env CASEBOOK_VERSION=v9.8.7 CASEBOOK_COMMIT=abc1234 \
    "$fx/scripts/build-windows.sh" --arch all
  assert_contains "$OUT" "skip: desktop/frontend/package.json not present yet (track A)"
  for arch in amd64 arm64; do
    gui="$fx/dist/windows-$arch/Casebook.exe"
    cli="$fx/dist/windows-$arch/bin/casebook.exe"
    [ -f "$gui" ] || fail "missing $gui"
    [ -f "$cli" ] || fail "missing $cli"
    ! cmp -s "$gui" "$cli" || fail "$gui and $cli are identical"
    # The desktop stand-in never reads Marker; package initialization
    # must still keep the marker in the binary.
    grep -a -q -F casebook-version=v9.8.7 "$gui" || fail "$gui does not contain the stamp marker"
    grep -a -q -F casebook-version=v9.8.7 "$cli" || fail "$cli does not contain the stamp marker"
    assert_contains "$OUT" "$gui"
    assert_contains "$OUT" "$cli"
  done
  assert_eq "$(cat "$fx/dist/.gitignore")" "*"
  # Release mode needs the real frontend.
  assert_rc 1 "$fx/scripts/build-windows.sh" --release
  assert_contains "$OUT" "missing track A (desktop/frontend/package.json) in release mode"
}

case_build_windows_collision_guard() {
  local fx
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli --with-desktop
  # Point the CLI next to the GUI: Casebook.exe and casebook.exe are one
  # path on Windows, so the guard must fail the build.
  sed 's/^  CB_CLI_SUBDIR="bin"$/  CB_CLI_SUBDIR="."/' "$fx/scripts/lib.sh" >"$fx/lib.sh.new"
  mv "$fx/lib.sh.new" "$fx/scripts/lib.sh"
  grep -q '^  CB_CLI_SUBDIR="\."$' "$fx/scripts/lib.sh" || fail "could not rewrite CB_CLI_SUBDIR in the fixture"
  assert_rc 1 "$fx/scripts/build-windows.sh"
  assert_contains "$OUT" "differ only by case"
}

# make_fake_app DIR: create and print DIR/Casebook.app holding a stand-in
# desktop executable, enough for package-macos.sh's input checks.
make_fake_app() {
  local app="$1/Casebook.app"
  mkdir -p "$app/Contents/MacOS"
  printf 'fake desktop executable\n' >"$app/Contents/MacOS/Casebook"
  chmod 755 "$app/Contents/MacOS/Casebook"
  printf '%s\n' "$app"
}

case_package_macos_skip_and_release() {
  local fx app
  if [ "$(uname -s)" = "Darwin" ]; then
    echo "Darwin host: the real macOS packaging run is covered by the release workflow"
    return 0
  fi
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli
  app="$(make_fake_app "$fx/app")"
  assert_rc 0 "$fx/scripts/package-macos.sh" --app "$app"
  assert_contains "$OUT" "skip: macOS packaging needs macOS (lipo, codesign, hdiutil)"
  assert_rc 1 "$fx/scripts/package-macos.sh" --release --app "$app"
  assert_contains "$OUT" "error: macOS packaging needs macOS (lipo, codesign, hdiutil)"
  assert_rc 1 env CASEBOOK_RELEASE=1 "$fx/scripts/package-macos.sh" --app "$app"
  [ ! -e "$fx/dist" ] || fail "a skipped run created dist/"
  [ ! -e "$app/Contents/MacOS/bin" ] || fail "a skipped run changed the app bundle"
}

case_package_macos_requires_cli() {
  local fx app
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx"
  app="$(make_fake_app "$fx/app")"
  assert_rc 0 "$fx/scripts/package-macos.sh" --dry-run --app "$app"
  assert_contains "$OUT" "skip: cmd/casebook not present yet (track B)"
  case "$OUT" in
    *"lipo -create"*) fail "a skipped run printed a command plan" ;;
  esac
  assert_rc 1 "$fx/scripts/package-macos.sh" --dry-run --release --app "$app"
  assert_contains "$OUT" "missing track B (cmd/casebook) in release mode"
}

case_package_macos_dry_run_adhoc() {
  local fx app want
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli
  app="$(make_fake_app "$fx/app")"
  assert_rc 0 env CASEBOOK_VERSION=v9.8.7 CASEBOOK_COMMIT=abc1234 \
    "$fx/scripts/package-macos.sh" --dry-run --app "$app" --out "$fx/out"
  for want in \
    "build-cli.sh --os darwin --arch arm64" \
    "build-cli.sh --os darwin --arch amd64" \
    "lipo -create -output $app/Contents/MacOS/bin/casebook" \
    "Contents/MacOS/bin/casebook" \
    "codesign --force --sign - $app/Contents/MacOS/bin/casebook" \
    "codesign --force --sign - $app" \
    "codesign --verify --deep --strict --verbose=2 $app" \
    "skip: Developer ID signing (CASEBOOK_SIGN_IDENTITY not set); ad-hoc signature only" \
    "ln -s /Applications" \
    "hdiutil create -volname Casebook" \
    "$fx/out/Casebook-v9.8.7-macos-universal.dmg"; do
    assert_contains "$OUT" "$want"
  done
  case "$OUT" in
    *notarytool*) fail "the ad-hoc plan mentions notarytool" ;;
  esac
  # The CLI is signed before the bundle that seals it.
  printf '%s\n' "$OUT" | grep -n 'codesign --force --sign -' >"$fx/order"
  assert_eq "$(sed -n 1p "$fx/order" | grep -c 'bin/casebook$')" "1"
  # A dry run writes nothing.
  [ ! -e "$fx/out" ] || fail "a dry run created the out directory"
  [ ! -e "$fx/dist" ] || fail "a dry run created dist/"
  [ ! -e "$app/Contents/MacOS/bin" ] || fail "a dry run changed the app bundle"
  # Release mode passes --release on to the CLI build.
  assert_rc 0 env CASEBOOK_VERSION=v9.8.7 \
    "$fx/scripts/package-macos.sh" --dry-run --release --app "$app" --out "$fx/out"
  assert_contains "$OUT" "build-cli.sh --os darwin --arch arm64 --out"
  assert_contains "$OUT" "casebook-arm64 --release"
}

case_package_macos_bad_args() {
  local fx app
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli
  app="$(make_fake_app "$fx/app")"
  assert_rc 2 "$fx/scripts/package-macos.sh"
  assert_contains "$OUT" "--app is required"
  assert_rc 2 "$fx/scripts/package-macos.sh" --dry-run
  assert_rc 2 "$fx/scripts/package-macos.sh" --app
  assert_rc 2 "$fx/scripts/package-macos.sh" --app "$app" --bogus
  assert_rc 2 env CASEBOOK_VERSION='v1 -X evil' "$fx/scripts/package-macos.sh" --dry-run --app "$app"
  assert_rc 1 "$fx/scripts/package-macos.sh" --dry-run --app "$fx/nonexistent.app"
  assert_contains "$OUT" "app bundle not found: $fx/nonexistent.app"
  rm "$app/Contents/MacOS/Casebook"
  assert_rc 1 "$fx/scripts/package-macos.sh" --dry-run --app "$app"
  assert_contains "$OUT" "desktop executable not found"
}

case_package_macos_dry_run_signed() {
  local fx app want secret="TestOnly-pw-5Q7Zx-not-real" ident="Developer ID Application: Test Only (TEAM000000)"
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli
  app="$(make_fake_app "$fx/app")"
  assert_rc 0 env CASEBOOK_VERSION=v9.8.7 CASEBOOK_COMMIT=abc1234 \
    CASEBOOK_SIGN_IDENTITY="$ident" CASEBOOK_KEYCHAIN="$fx/signing.keychain-db" \
    CASEBOOK_NOTARIZE=1 APPLE_ID=notary@example.com APPLE_APP_PASSWORD="$secret" \
    APPLE_TEAM_ID=TEAM000000 \
    "$fx/scripts/package-macos.sh" --dry-run --app "$app" --out "$fx/out"
  for want in \
    "--options runtime --timestamp" \
    "--entitlements $fx/packaging/macos/entitlements.plist" \
    "--keychain $fx/signing.keychain-db" \
    "codesign --verify --deep --strict --verbose=2 $app" \
    "notarytool submit $fx/out/Casebook-v9.8.7-macos-universal.dmg" \
    "--apple-id notary@example.com --password *** --team-id TEAM000000" \
    "--wait --timeout 30m --output-format json" \
    "stapler staple $fx/out/Casebook-v9.8.7-macos-universal.dmg" \
    "stapler validate $fx/out/Casebook-v9.8.7-macos-universal.dmg" \
    "stapler staple $app" \
    "stapler validate $app" \
    "spctl --assess --type open --context context:primary-signature"; do
    assert_contains "$OUT" "$want"
  done
  case "$OUT" in
    *"$secret"*) fail "the Apple app-specific password leaked into the dry-run output" ;;
    *"--sign -"*) fail "an identity run still ad-hoc signs" ;;
    *"skip: notarization"*) fail "a fully configured run skipped notarization" ;;
  esac
  # Order: CLI, then the app with entitlements, then notarize and staple the
  # app, then build the DMG from it, then sign, notarize and staple the DMG.
  printf '%s\n' "$OUT" | grep -E 'codesign --force|notarytool submit|stapler staple|hdiutil create' >"$fx/order"
  assert_eq "$(wc -l <"$fx/order" | tr -d ' ')" "8"
  sed -n 1p "$fx/order" | grep -q 'bin/casebook$' || fail "the CLI is not signed first"
  sed -n 2p "$fx/order" | grep -q -- '--entitlements .*Casebook\.app$' || fail "the app is not signed second with entitlements"
  sed -n 3p "$fx/order" | grep -q 'notarytool submit .*Casebook-app\.zip' || fail "the app is not notarized third"
  sed -n 4p "$fx/order" | grep -q "stapler staple $app\$" || fail "the app is not stapled before the DMG is built"
  sed -n 5p "$fx/order" | grep -q 'hdiutil create' || fail "the DMG is not built from the stapled app"
  sed -n 6p "$fx/order" | grep -q 'codesign --force .*macos-universal\.dmg$' || fail "the DMG is not signed after it is built"
  sed -n 7p "$fx/order" | grep -q 'notarytool submit .*macos-universal\.dmg' || fail "the DMG is not notarized after signing"
  sed -n 8p "$fx/order" | grep -q 'stapler staple .*macos-universal\.dmg$' || fail "the DMG is not stapled last"
  assert_contains "$OUT" "spctl --assess --type execute --verbose $app"
  # Without CASEBOOK_NOTARIZE, the signed run skips notarization.
  assert_rc 0 env CASEBOOK_VERSION=v9.8.7 CASEBOOK_SIGN_IDENTITY="$ident" \
    "$fx/scripts/package-macos.sh" --dry-run --app "$app" --out "$fx/out"
  assert_contains "$OUT" "skip: notarization not requested (CASEBOOK_NOTARIZE is not 1)"
  case "$OUT" in
    *notarytool* | *"--keychain"*) fail "notarytool or --keychain without being asked for" ;;
  esac
}

case_package_macos_notarize_without_identity() {
  local fx app
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli
  app="$(make_fake_app "$fx/app")"
  assert_rc 0 env CASEBOOK_VERSION=v9.8.7 CASEBOOK_NOTARIZE=1 \
    APPLE_ID=notary@example.com APPLE_APP_PASSWORD=unused APPLE_TEAM_ID=TEAM000000 \
    "$fx/scripts/package-macos.sh" --dry-run --app "$app" --out "$fx/out"
  assert_contains "$OUT" "skip: notarization needs a Developer ID signature"
  assert_contains "$OUT" "codesign --force --sign -"
  case "$OUT" in
    *notarytool*) fail "notarytool ran without a Developer ID signature" ;;
  esac
}

case_package_macos_notarize_missing_credentials() {
  local fx app var
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli
  app="$(make_fake_app "$fx/app")"
  for var in APPLE_ID APPLE_APP_PASSWORD APPLE_TEAM_ID; do
    assert_rc 1 env CASEBOOK_VERSION=v9.8.7 \
      CASEBOOK_SIGN_IDENTITY="Developer ID Application: Test Only (TEAM000000)" \
      CASEBOOK_NOTARIZE=1 APPLE_ID=notary@example.com APPLE_APP_PASSWORD=TestOnly-pw \
      APPLE_TEAM_ID=TEAM000000 "$var=" \
      "$fx/scripts/package-macos.sh" --dry-run --app "$app" --out "$fx/out"
    assert_contains "$OUT" "needs $var"
    case "$OUT" in
      *"lipo -create"*) fail "a misconfigured notarization still started building" ;;
    esac
  done
}

# make_docs_fixture DIR: a fixture with a minimal doc set that passes
# scripts/check-docs.sh even in release mode: a README linking the eleven
# required guides, a stub for each, and a release workflow whose single
# secret the release guide documents.
make_docs_fixture() {
  local dir="$1" t
  make_fixture_repo "$dir"
  mkdir -p "$dir/docs/decisions" "$dir/.github/workflows"
  printf '# Fixture\n\n' >"$dir/README.md"
  for t in docs/architecture.md docs/development.md docs/building-releases.md \
    docs/mcp.md docs/claude-code.md docs/codex.md docs/privacy.md docs/updating.md \
    CONTRIBUTING.md SECURITY.md docs/decisions/0011-license.md; do
    printf -- '- [%s](%s)\n' "$t" "$t" >>"$dir/README.md"
    printf '# Stub\n' >"$dir/$t"
  done
  printf '# Building releases\n\nSecret: WINDOWS_CERT_PASSWORD.\n' >"$dir/docs/building-releases.md"
  # shellcheck disable=SC2016 # literal workflow expressions
  printf '%s\n' 'on: push' 'jobs:' '  a:' '    runs-on: ubuntu-latest' \
    '    steps:' '      - run: echo "$P"' '        env:' \
    '          P: ${{ secrets.WINDOWS_CERT_PASSWORD }}' \
    >"$dir/.github/workflows/release.yml"
}

case_check_docs_instructions() {
  local fx
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  printf '# Shared instructions\n' >"$fx/CLAUDE.md"
  cp "$fx/CLAUDE.md" "$fx/AGENTS.md"
  assert_rc 0 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "ok: CLAUDE.md and AGENTS.md are identical"

  printf 'A change in one file only.\n' >>"$fx/AGENTS.md"
  assert_rc 1 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "error: CLAUDE.md and AGENTS.md differ"

  rm "$fx/AGENTS.md"
  assert_rc 1 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "error: CLAUDE.md and AGENTS.md must both exist"
}

case_check_docs_gate() {
  local fx

  # Baseline: clean in development and in release mode.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  assert_rc 0 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "ok: no em dash in any text file"
  assert_contains "$OUT" "ok: README.md links all 11 guides"
  assert_contains "$OUT" "ok: 1 secret name(s) in .github/workflows/release.yml match docs/building-releases.md"
  assert_contains "$OUT" "info: 0 FINALIZE(phase 6) markers"
  assert_contains "$OUT" "ok: docs check passed"
  assert_rc 0 "$fx/scripts/check-docs.sh" --release
  assert_rc 2 "$fx/scripts/check-docs.sh" --bogus

  # A README link to a missing file.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  printf '[gone](docs/gone.md)\n' >>"$fx/README.md"
  assert_rc 1 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "error: broken link in README.md:14: docs/gone.md"

  # Links in inline code and fenced code blocks are not links; anchors on
  # an existing file and relative paths from docs/ resolve.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  # shellcheck disable=SC2016 # literal Markdown code spans
  printf '%s\n' '`[x](nowhere.md)`' '```' '[y](nowhere.md)' '```' \
    '[ok](../README.md#fixture) [up](decisions/0011-license.md) [web](https://example.invalid/x)' \
    >>"$fx/docs/codex.md"
  assert_rc 0 "$fx/scripts/check-docs.sh"

  # A guide missing from the README index.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  grep -v -F '(SECURITY.md)' "$fx/README.md" >"$fx/README.tmp"
  mv "$fx/README.tmp" "$fx/README.md"
  assert_rc 1 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "error: README.md does not link SECURITY.md"

  # An em dash, written from its UTF-8 bytes so this file never holds one.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  printf 'one \342\200\224 two\n' >"$fx/docs/dash.md"
  assert_rc 1 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "error: em dash in docs/dash.md:1:"

  # A pending track file warns in development and fails in release mode.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  printf '[tools](mcp-tools.md)\n' >>"$fx/docs/mcp.md"
  assert_rc 0 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "warn: pending docs/mcp-tools.md (track B) linked from docs/mcp.md:2"
  assert_rc 1 "$fx/scripts/check-docs.sh" --release
  assert_contains "$OUT" "error: pending docs/mcp-tools.md (track B)"

  # A FINALIZE(phase 6) marker is info in development, an error in release.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  printf '<!-- FINALIZE(phase 6): check this -->\n' >>"$fx/docs/privacy.md"
  assert_rc 0 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "info: 1 FINALIZE(phase 6) markers"
  assert_contains "$OUT" "info:   docs/privacy.md:2"
  assert_rc 1 "$fx/scripts/check-docs.sh" --release
  assert_contains "$OUT" "error: FINALIZE(phase 6) marker at docs/privacy.md:2"
  assert_rc 1 env CASEBOOK_RELEASE=1 "$fx/scripts/check-docs.sh"

  # A second secret used in release.yml but not documented.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  # shellcheck disable=SC2016 # literal workflow expression
  printf '%s\n' '          Q: ${{ secrets.APPLE_ID }}' >>"$fx/.github/workflows/release.yml"
  assert_rc 1 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "error: secret APPLE_ID is used in .github/workflows/release.yml but not documented in docs/building-releases.md"

  # A secret documented but never used.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  printf 'Also APPLE_TEAM_ID.\n' >>"$fx/docs/building-releases.md"
  assert_rc 1 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "error: secret APPLE_TEAM_ID is documented in docs/building-releases.md but not used in .github/workflows/release.yml"

  # No release workflow: a skip line in development, an error in release.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  rm "$fx/.github/workflows/release.yml"
  assert_rc 0 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "skip: .github/workflows/release.yml not present; secret parity not checked"
  assert_rc 1 "$fx/scripts/check-docs.sh" --release

  # Action pins: a SHA with a version comment and a local reusable workflow
  # pass; a tag, a SHA without a comment and a short SHA fail.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  printf '%s\n' 'on: push' 'jobs:' '  a:' '    uses: ./.github/workflows/release.yml' '  b:' \
    '    runs-on: ubuntu-latest' '    steps:' \
    '      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1' \
    '      - name: x' '        uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e   #   v7.0.0' \
    >"$fx/.github/workflows/ci.yml"
  assert_rc 0 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "ok: 2 third-party action reference(s) in .github/workflows are pinned to commit SHAs"
  printf '%s\n' '      - uses: actions/setup-node@v7' \
    '      - uses: actions/upload-artifact@043fb46d1a93c77aae656e7c1c64a875d1fc6a0a' \
    '      - uses: actions/download-artifact@3e5f45b # v8.0.1' \
    >>"$fx/.github/workflows/ci.yml"
  assert_rc 1 "$fx/scripts/check-docs.sh"
  assert_contains "$OUT" "error: action at .github/workflows/ci.yml:11 is not pinned to a commit SHA with a version comment: actions/setup-node@v7"
  assert_contains "$OUT" "error: action at .github/workflows/ci.yml:12 is not pinned"
  assert_contains "$OUT" "error: action at .github/workflows/ci.yml:13 is not pinned"
}

# make_release_files DIR VERSION: the files check 7 of scripts/check-docs.sh
# reads, all carrying VERSION (MAJOR.MINOR.PATCH), with an empty Unreleased
# section in the changelog.
make_release_files() {
  local dir="$1" v="$2"
  mkdir -p "$dir/desktop/frontend"
  printf '# Changelog\n\n## Unreleased\n\n## %s (2026-01-02)\n\n### Added\n\n- A thing.\n' "$v" >"$dir/CHANGELOG.md"
  printf '{\n  "name": "casebook",\n  "info": {\n    "productVersion": "%s"\n  }\n}\n' "$v" >"$dir/desktop/wails.json"
  printf '{\n  "name": "casebook-frontend",\n  "version": "%s"\n}\n' "$v" >"$dir/desktop/frontend/package.json"
  printf '{\n  "name": "casebook-frontend",\n  "version": "%s",\n  "packages": {\n    "": {\n      "version": "%s"\n    }\n  }\n}\n' "$v" "$v" >"$dir/desktop/frontend/package-lock.json"
}

# Check 7: in release mode with a stable CASEBOOK_VERSION, the changelog,
# wails.json and the frontend package must carry that version.
case_check_docs_release_version() {
  local fx

  # Development mode never checks parity; release mode skips without a
  # version, and for a prerelease or dry-run version.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  assert_rc 0 env CASEBOOK_VERSION=v1.2.3 "$fx/scripts/check-docs.sh"
  assert_not_contains "$OUT" "version parity"
  assert_rc 0 "$fx/scripts/check-docs.sh" --release
  assert_contains "$OUT" "skip: CASEBOOK_VERSION is not set; version parity not checked"
  assert_rc 0 env CASEBOOK_VERSION=v1.2.3-rc.1 "$fx/scripts/check-docs.sh" --release
  assert_contains "$OUT" "skip: version v1.2.3-rc.1 is not a stable release version"
  assert_rc 0 env CASEBOOK_VERSION=v0.0.0-dryrun.1 "$fx/scripts/check-docs.sh" --release
  assert_contains "$OUT" "skip: version v0.0.0-dryrun.1 is not a stable release version"

  # A stable version with every file in agreement passes.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  make_release_files "$fx" 1.2.3
  assert_rc 0 env CASEBOOK_VERSION=v1.2.3 "$fx/scripts/check-docs.sh" --release
  assert_contains "$OUT" "ok: CHANGELOG.md, desktop/wails.json and the frontend package carry version 1.2.3"

  # The files are missing.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  assert_rc 1 env CASEBOOK_VERSION=v1.2.3 "$fx/scripts/check-docs.sh" --release
  assert_contains "$OUT" "error: CHANGELOG.md is missing; a release needs a ## 1.2.3 section"
  assert_contains "$OUT" "error: desktop/wails.json is missing"
  assert_contains "$OUT" "error: desktop/frontend/package.json is missing"
  assert_contains "$OUT" "error: desktop/frontend/package-lock.json is missing"

  # The tag moved on but the files still say the previous version, and the
  # changelog still has entries under Unreleased.
  fx="$(new_fixture_dir)"
  make_docs_fixture "$fx"
  make_release_files "$fx" 1.2.3
  printf '# Changelog\n\n## Unreleased\n\n### Changed\n\n- One.\n- Two.\n\n## 1.2.3 (2026-01-02)\n\n- Old.\n' >"$fx/CHANGELOG.md"
  assert_rc 1 env CASEBOOK_VERSION=v1.3.0 "$fx/scripts/check-docs.sh" --release
  assert_contains "$OUT" 'error: CHANGELOG.md has no "## 1.3.0 (YYYY-MM-DD)" section; rename ## Unreleased before tagging v1.3.0'
  assert_contains "$OUT" "error: CHANGELOG.md still lists 3 entries under ## Unreleased; move them into ## 1.3.0"
  assert_contains "$OUT" "error: desktop/wails.json productVersion is not 1.3.0"
  assert_contains "$OUT" "error: desktop/frontend/package.json version is 1.2.3, not 1.3.0 (run: npm --prefix desktop/frontend version 1.3.0 --no-git-tag-version)"
  assert_contains "$OUT" "error: desktop/frontend/package-lock.json version is 1.2.3, not 1.3.0"
  assert_not_contains "$OUT" "ok: CHANGELOG.md, desktop/wails.json"
}

case_package_cli_archives() {
  local fx bin listing
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli
  export CASEBOOK_VERSION=v1.2.3 CASEBOOK_COMMIT=abc1234
  printf 'notices\n' >"$fx/notices.txt"

  # Usage errors.
  assert_rc 2 "$fx/scripts/package-cli.sh" --arch amd64 --bin x
  assert_contains "$OUT" "--os is required"
  assert_rc 2 "$fx/scripts/package-cli.sh" --os plan9 --arch amd64 --bin x
  assert_rc 2 "$fx/scripts/package-cli.sh" --os linux --arch amd64
  assert_contains "$OUT" "--bin is required"

  assert_rc 0 "$fx/scripts/build-cli.sh" --os linux --arch amd64 --out "$fx/dist/linux/casebook"
  bin="$fx/dist/linux/casebook"

  # Development: the archive holds the CLI and the notices (no LICENSE in
  # the fixture), flat.
  assert_rc 0 "$fx/scripts/package-cli.sh" --os linux --arch amd64 --bin "$bin" \
    --notices "$fx/notices.txt" --out "$fx/out"
  assert_contains "$OUT" "casebook-cli-v1.2.3-linux-amd64.tar.gz"
  listing="$(tar -tzf "$fx/out/casebook-cli-v1.2.3-linux-amd64.tar.gz" | LC_ALL=C sort | tr '\n' ' ')"
  assert_eq "$listing" "THIRD_PARTY_NOTICES.txt casebook "

  # Release mode needs LICENSE and the notices.
  assert_rc 1 "$fx/scripts/package-cli.sh" --release --os linux --arch amd64 --bin "$bin" \
    --notices "$fx/notices.txt" --out "$fx/out"
  assert_contains "$OUT" "LICENSE is missing"
  printf 'MIT License\n\nCopyright (c) test\n' >"$fx/LICENSE"
  assert_rc 1 "$fx/scripts/package-cli.sh" --release --os linux --arch amd64 --bin "$bin" --out "$fx/out"
  assert_contains "$OUT" "--notices is required in release mode"
  assert_rc 0 "$fx/scripts/package-cli.sh" --release --os linux --arch amd64 --bin "$bin" \
    --notices "$fx/notices.txt" --out "$fx/out"
  listing="$(tar -tzf "$fx/out/casebook-cli-v1.2.3-linux-amd64.tar.gz" | LC_ALL=C sort | tr '\n' ' ')"
  assert_eq "$listing" "LICENSE THIRD_PARTY_NOTICES.txt casebook "
  mkdir "$fx/x"
  tar -xzf "$fx/out/casebook-cli-v1.2.3-linux-amd64.tar.gz" -C "$fx/x"
  [ -x "$fx/x/casebook" ] || fail "casebook is not executable after extraction"

  # Release mode refuses a binary without the stamp marker.
  printf 'not a casebook binary\n' >"$fx/fake"
  assert_rc 1 "$fx/scripts/package-cli.sh" --release --os linux --arch amd64 --bin "$fx/fake" \
    --notices "$fx/notices.txt" --out "$fx/out"
  assert_contains "$OUT" "does not contain the stamp marker casebook-version=v1.2.3"

  # Windows gets a zip with casebook.exe.
  if command -v 7z >/dev/null 2>&1 || command -v zip >/dev/null 2>&1 || command -v python3 >/dev/null 2>&1; then
    assert_rc 0 "$fx/scripts/build-cli.sh" --os windows --arch arm64 --out "$fx/dist/win/casebook.exe"
    assert_rc 0 "$fx/scripts/package-cli.sh" --release --os windows --arch arm64 \
      --bin "$fx/dist/win/casebook.exe" --notices "$fx/notices.txt" --out "$fx/out"
    [ -s "$fx/out/casebook-cli-v1.2.3-windows-arm64.zip" ] || fail "windows CLI zip missing"
    if command -v python3 >/dev/null 2>&1; then
      listing="$(python3 -c 'import sys, zipfile; print(" ".join(sorted(zipfile.ZipFile(sys.argv[1]).namelist())))' \
        "$fx/out/casebook-cli-v1.2.3-windows-arm64.zip")"
      assert_eq "$listing" "LICENSE THIRD_PARTY_NOTICES.txt casebook.exe"
    fi
  fi
}

case_third_party_notices() {
  local fx
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx" --with-cli
  assert_rc 2 "$fx/scripts/third-party-notices.sh" --bogus
  # The fixture has no dependencies: only the Go standard library.
  assert_rc 0 "$fx/scripts/third-party-notices.sh" --out "$fx/out/N.txt"
  assert_contains "$(cat "$fx/out/N.txt")" "Go standard library and runtime"
  assert_contains "$(cat "$fx/out/N.txt")" "--- LICENSE ---"

  command -v node >/dev/null 2>&1 || return 0
  # Frontend packages: production ones are listed, dev ones and absent
  # optional ones are not.
  local fe="$fx/desktop/frontend"
  mkdir -p "$fe/node_modules/left-pad" "$fe/node_modules/devtool" "$fe/node_modules/@scope/nolicense"
  cat >"$fe/package-lock.json" <<'EOF'
{
  "lockfileVersion": 3,
  "packages": {
    "": {"name": "x"},
    "node_modules/left-pad": {"version": "1.3.0"},
    "node_modules/devtool": {"version": "2.0.0", "dev": true},
    "node_modules/@esbuild/other-os": {"version": "0.1.0", "optional": true}
  }
}
EOF
  printf 'left-pad license text\n' >"$fe/node_modules/left-pad/LICENSE"
  printf 'dev license text\n' >"$fe/node_modules/devtool/LICENSE"
  assert_rc 0 "$fx/scripts/third-party-notices.sh" --release --out "$fx/out/N.txt"
  assert_contains "$OUT" "1 npm packages"
  assert_contains "$(cat "$fx/out/N.txt")" "npm package left-pad 1.3.0"
  assert_contains "$(cat "$fx/out/N.txt")" "left-pad license text"
  if grep -q 'devtool' "$fx/out/N.txt"; then
    fail "a dev dependency was listed"
  fi

  # A production package without a license file warns in development and
  # fails in release mode.
  cat >"$fe/package-lock.json" <<'EOF'
{"lockfileVersion": 3, "packages": {"node_modules/@scope/nolicense": {"version": "1.0.0"}}}
EOF
  assert_rc 0 "$fx/scripts/third-party-notices.sh" --out "$fx/out/N.txt"
  assert_contains "$OUT" "warn: no license file found for npm package @scope/nolicense 1.0.0"
  assert_rc 1 "$fx/scripts/third-party-notices.sh" --release --out "$fx/out/N.txt"
  assert_contains "$OUT" "error: no license file found for npm package @scope/nolicense 1.0.0"

  # Missing node_modules: an error in release mode.
  rm -rf "$fe/node_modules"
  assert_rc 1 "$fx/scripts/third-party-notices.sh" --release --out "$fx/out/N.txt"
  assert_contains "$OUT" "desktop/frontend/node_modules is missing"
}

case_render_homebrew_formula() {
  local fx f
  fx="$(new_fixture_dir)"
  make_fixture_repo "$fx"
  {
    printf '%064d  Casebook-v1.2.3-macos-universal.dmg\n' 0
    printf '%s  casebook-cli-v1.2.3-darwin-arm64.tar.gz\n' "$(printf 'a%.0s' $(seq 64))"
    printf '%s  casebook-cli-v1.2.3-darwin-amd64.tar.gz\n' "$(printf 'b%.0s' $(seq 64))"
    printf '%s  casebook-cli-v1.2.3-linux-arm64.tar.gz\n' "$(printf 'c%.0s' $(seq 64))"
    printf '%s  casebook-cli-v1.2.3-linux-amd64.tar.gz\n' "$(printf 'D%.0s' $(seq 64))"
  } >"$fx/SHA256SUMS.txt"

  assert_rc 2 "$fx/scripts/render-homebrew-formula.sh" --sums "$fx/SHA256SUMS.txt"
  assert_rc 2 "$fx/scripts/render-homebrew-formula.sh" --version 1.2.3 --sums "$fx/SHA256SUMS.txt"
  assert_rc 2 "$fx/scripts/render-homebrew-formula.sh" --version v1.2.3+meta --sums "$fx/SHA256SUMS.txt"

  # No LICENSE: rendered without a license line.
  assert_rc 0 "$fx/scripts/render-homebrew-formula.sh" --version v1.2.3 --sums "$fx/SHA256SUMS.txt" --out "$fx/casebook.rb"
  f="$(cat "$fx/casebook.rb")"
  assert_contains "$f" 'version "1.2.3"'
  assert_contains "$f" 'url "https://github.com/jameskbb/casebook/releases/download/v1.2.3/casebook-cli-v1.2.3-darwin-arm64.tar.gz"'
  assert_contains "$f" "sha256 \"$(printf 'a%.0s' $(seq 64))\""
  assert_contains "$f" "sha256 \"$(printf 'd%.0s' $(seq 64))\""
  assert_contains "$f" 'casebook-cli-v1.2.3-linux-amd64.tar.gz'
  if grep -q 'license\|@[A-Z0-9_]*@' "$fx/casebook.rb"; then
    fail "unexpected license line or placeholder in the rendered formula"
  fi
  assert_eq "$(head -n 2 "$fx/casebook.rb" | tail -n 1)" "class Casebook < Formula"

  # An MIT LICENSE gives license "MIT".
  printf '\nMIT License\n\nCopyright (c) test\n' >"$fx/LICENSE"
  assert_rc 0 "$fx/scripts/render-homebrew-formula.sh" --version v1.2.3 --sums "$fx/SHA256SUMS.txt" --out "$fx/casebook.rb"
  assert_contains "$(cat "$fx/casebook.rb")" 'license "MIT"'
  if command -v ruby >/dev/null 2>&1; then
    assert_rc 0 ruby -c "$fx/casebook.rb"
  fi

  # A missing archive line fails.
  grep -v linux-arm64 "$fx/SHA256SUMS.txt" >"$fx/partial.txt"
  assert_rc 1 "$fx/scripts/render-homebrew-formula.sh" --version v1.2.3 --sums "$fx/partial.txt"
  assert_contains "$OUT" "has no SHA-256 line for casebook-cli-v1.2.3-linux-arm64.tar.gz"
}

# make_install_env DIR: a fake release v9.9.9 for linux and darwin on both
# CPUs under DIR/release, stub curl, wget and uname commands under
# DIR/stub, and DIR/sys with only the system tools install.sh needs (so a
# real curl or wget never runs). Stubs read FAKE_OS, FAKE_ARCH, FAKE_TAG
# and log each URL to DIR/urls.log.
make_install_env() {
  local dir="$1" os arch tool p
  mkdir -p "$dir/release" "$dir/stub" "$dir/sys" "$dir/build"
  for os in linux darwin; do
    for arch in amd64 arm64; do
      printf '#!/bin/sh\necho "casebook v9.9.9 (%s/%s)"\n' "$os" "$arch" >"$dir/build/casebook"
      chmod 0755 "$dir/build/casebook"
      printf 'license\n' >"$dir/build/LICENSE"
      tar -czf "$dir/release/casebook-cli-v9.9.9-$os-$arch.tar.gz" -C "$dir/build" casebook LICENSE
    done
  done
  (cd "$dir/release" && sha256sum -- *.tar.gz >SHA256SUMS.txt)

  for tool in sh tar gzip awk sed cut tr mkdir cp chmod mv rm mktemp head cat sha256sum dirname basename; do
    p="$(command -v "$tool")" || fail "test needs $tool"
    ln -s "$p" "$dir/sys/$tool"
  done

  cat >"$dir/stub/uname" <<'EOF'
#!/bin/sh
case "$1" in
  -s) echo "$FAKE_OS" ;;
  -m) echo "$FAKE_ARCH" ;;
  *) echo "$FAKE_OS" ;;
esac
EOF
  cat >"$dir/stub/curl" <<EOF
#!/bin/sh
out="" url="" w=""
while [ "\$#" -gt 0 ]; do
  case "\$1" in
    -o) out="\$2"; shift 2 ;;
    -w) w="\$2"; shift 2 ;;
    --proto | --retry) shift 2 ;;
    -*) shift ;;
    *) url="\$1"; shift ;;
  esac
done
echo "curl \$url" >>"$dir/urls.log"
if [ -n "\$w" ]; then
  [ "\$url" = "https://github.com/jameskbb/casebook/releases/latest" ] || exit 22
  printf 'https://github.com/jameskbb/casebook/releases/tag/%s' "\$FAKE_TAG"
  exit 0
fi
case "\$url" in
  https://github.com/jameskbb/casebook/releases/download/v9.9.9/*) ;;
  *) exit 22 ;;
esac
f="$dir/release/\${url##*/}"
[ -f "\$f" ] || exit 22
cp "\$f" "\$out"
EOF
  cat >"$dir/stub/wget" <<EOF
#!/bin/sh
out="" url=""
while [ "\$#" -gt 0 ]; do
  case "\$1" in
    -O) out="\$2"; shift 2 ;;
    -*) shift ;;
    *) url="\$1"; shift ;;
  esac
done
echo "wget \$url" >>"$dir/urls.log"
if [ "\$url" = "https://api.github.com/repos/jameskbb/casebook/releases/latest" ]; then
  printf '{\n  "url": "x",\n  "tag_name": "%s",\n  "name": "Casebook"\n}\n' "\$FAKE_TAG"
  exit 0
fi
case "\$url" in
  https://github.com/jameskbb/casebook/releases/download/v9.9.9/*) ;;
  *) exit 8 ;;
esac
f="$dir/release/\${url##*/}"
[ -f "\$f" ] || exit 8
cp "\$f" "\$out"
EOF
  chmod 0755 "$dir/stub/uname" "$dir/stub/curl" "$dir/stub/wget"
}

# run_install DIR EXPECTED_RC [VAR=VALUE...]: run scripts/install.sh the way
# `curl ... | sh` does (script on stdin), under dash when it is installed,
# with PATH limited to the stubs and DIR/sys.
run_install() {
  local dir="$1" want="$2" shell=sh
  shift 2
  if command -v dash >/dev/null 2>&1; then
    shell="$(command -v dash)"
  fi
  assert_rc "$want" env -i HOME="$dir/home" PATH="$dir/stub:$dir/sys" \
    FAKE_OS=Linux FAKE_ARCH=x86_64 FAKE_TAG=v9.9.9 "$@" "$shell" -s <"$TS_REPO/scripts/install.sh"
}

case_install_script() {
  local d f
  d="$(new_fixture_dir)"
  make_install_env "$d"

  # Latest release, default folder, not on PATH: installs, verifies the
  # checksum and prints the PATH line and the next step.
  run_install "$d" 0
  assert_contains "$OUT" "checksum ok"
  assert_contains "$OUT" "installed $d/home/.local/bin/casebook (casebook v9.9.9 (linux/amd64))"
  assert_contains "$OUT" "is not on your PATH"
  assert_contains "$OUT" "export PATH=\"$d/home/.local/bin:\$PATH\""
  assert_contains "$OUT" "$d/home/.local/bin/casebook setup claude"
  assert_contains "$(cat "$d/urls.log")" "curl https://github.com/jameskbb/casebook/releases/latest"
  assert_contains "$(cat "$d/urls.log")" "releases/download/v9.9.9/casebook-cli-v9.9.9-linux-amd64.tar.gz"
  [ -x "$d/home/.local/bin/casebook" ] || fail "casebook was not installed executable"
  for f in "$d/home/.local/bin"/.casebook.new*; do
    [ ! -e "$f" ] || fail "a temporary file was left in the install folder: $f"
  done

  # A pinned version and folder on PATH: no lookup, plain next step.
  rm -f "$d/urls.log"
  run_install "$d" 0 CASEBOOK_VERSION=v9.9.9 CASEBOOK_INSTALL_DIR="$d/opt bin" PATH="$d/stub:$d/sys:$d/opt bin"
  assert_contains "$OUT" "installed $d/opt bin/casebook"
  assert_contains "$OUT" "    casebook setup claude"
  if grep -q 'releases/latest' "$d/urls.log"; then
    fail "a pinned version still looked up the latest release"
  fi

  # CPU and OS detection.
  run_install "$d" 0 FAKE_ARCH=aarch64 CASEBOOK_INSTALL_DIR="$d/a"
  assert_contains "$OUT" "(casebook v9.9.9 (linux/arm64))"
  run_install "$d" 0 FAKE_OS=Darwin FAKE_ARCH=arm64 CASEBOOK_INSTALL_DIR="$d/b"
  assert_contains "$OUT" "(casebook v9.9.9 (darwin/arm64))"
  run_install "$d" 0 FAKE_OS=Darwin FAKE_ARCH=x86_64 CASEBOOK_INSTALL_DIR="$d/c"
  assert_contains "$OUT" "(casebook v9.9.9 (darwin/amd64))"
  run_install "$d" 1 FAKE_OS=FreeBSD
  assert_contains "$OUT" "unsupported operating system: FreeBSD"
  run_install "$d" 1 FAKE_OS=MINGW64_NT-10.0
  assert_contains "$OUT" "on Windows download Casebook-<version>-windows-<arch>-portable.zip"
  run_install "$d" 1 FAKE_ARCH=i686
  assert_contains "$OUT" "unsupported CPU architecture: i686"

  # Bad and missing versions.
  run_install "$d" 1 CASEBOOK_VERSION=1.2.3
  assert_contains "$OUT" "is not a release tag"
  run_install "$d" 1 CASEBOOK_VERSION='v1;rm'
  assert_contains "$OUT" "is not a release tag"
  run_install "$d" 1 FAKE_TAG=v8.8.8 CASEBOOK_INSTALL_DIR="$d/missing"
  assert_contains "$OUT" "download failed"
  [ ! -e "$d/missing/casebook" ] || fail "installed after a failed download"

  # A checksum mismatch installs nothing.
  cp "$d/release/SHA256SUMS.txt" "$d/sums.bak"
  awk '{ printf "%064d  %s\n", 0, $2 }' "$d/sums.bak" >"$d/release/SHA256SUMS.txt"
  run_install "$d" 1 CASEBOOK_INSTALL_DIR="$d/bad"
  assert_contains "$OUT" "checksum mismatch for casebook-cli-v9.9.9-linux-amd64.tar.gz"
  assert_contains "$OUT" "nothing was installed"
  [ ! -e "$d/bad/casebook" ] || fail "installed despite a checksum mismatch"
  grep -v linux-amd64 "$d/sums.bak" >"$d/release/SHA256SUMS.txt"
  run_install "$d" 1 CASEBOOK_INSTALL_DIR="$d/bad"
  assert_contains "$OUT" "SHA256SUMS.txt has no line for casebook-cli-v9.9.9-linux-amd64.tar.gz"
  cp "$d/sums.bak" "$d/release/SHA256SUMS.txt"

  # Without curl, wget and the API.
  rm "$d/stub/curl"
  rm -f "$d/urls.log"
  run_install "$d" 0 CASEBOOK_INSTALL_DIR="$d/w"
  assert_contains "$OUT" "installed $d/w/casebook"
  assert_contains "$(cat "$d/urls.log")" "wget https://api.github.com/repos/jameskbb/casebook/releases/latest"

  # Neither tool.
  rm "$d/stub/wget"
  run_install "$d" 1 CASEBOOK_VERSION=v9.9.9 CASEBOOK_INSTALL_DIR="$d/n"
  assert_contains "$OUT" "neither curl nor wget is installed"
}

# The agent eval harness (scripts/agent-eval) refuses any run directory that
# overlaps the repository or real client config and data, before it builds
# or runs anything; its scenario repos compile and its scenario files parse.
case_agent_eval_harness() {
  if ! command -v python3 >/dev/null 2>&1; then
    printf 'skip: python3 not found\n'
    return 0
  fi
  local ev="$TS_REPO/scripts/agent-eval" d
  d="$(new_fixture_dir)"
  assert_rc 2 python3 -B "$ev/run.py" pagination --out "$TS_REPO/agent-eval-run"
  assert_contains "$OUT" "overlaps"
  assert_rc 2 env CASEBOOK_HOME="$d" python3 -B "$ev/run.py" pagination --out "$d/run"
  assert_contains "$OUT" "overlaps"
  assert_rc 2 env HOME="$d" python3 -B "$ev/run.py" pagination --out "$d/.claude/run"
  assert_contains "$OUT" "overlaps"
  assert_rc 2 python3 -B "$ev/grade.py"
  local sc
  for sc in "$ev"/scenarios/*/; do
    assert_rc 0 python3 -c 'import json, sys; d = json.load(open(sys.argv[1])); assert d["kind"] in ("investigation", "control") and d["sessions"]' "$sc/scenario.json"
    # shellcheck disable=SC2016 # $1 expands in the inner shell
    assert_rc 0 sh -c 'cd "$1" && go vet ./...' sh "$sc/repo"
  done
}

# The real README is written for a person who downloads an app: it stays
# short, and it tells the reader to download the app before any code block.
case_readme_plain_reader() {
  local readme="$TS_REPO/README.md" words first_fence first_download
  words=$(wc -w <"$readme")
  [ "$words" -lt 1800 ] || fail "README.md is $words words; keep it under 1800"
  first_fence=$(grep -n '^```' "$readme" | head -n 1 | cut -d: -f1)
  first_download=$(grep -n 'Download the app' "$readme" | head -n 1 | cut -d: -f1)
  if [ -z "$first_download" ] || { [ -n "$first_fence" ] && [ "$first_download" -ge "$first_fence" ]; }; then
    fail "README.md must tell a reader to download the app before any code block"
  fi
}

CASES=(
  version_dev_fallback
  version_override_and_validation
  build_cli_skip_and_release
  build_cli_stamps_binary
  stamp_marker_needs_x_flag
  stub_frontend_dist
  preflight_modes
  build_windows_skip_and_release
  build_windows_fixture
  build_windows_collision_guard
  package_macos_skip_and_release
  package_macos_requires_cli
  package_macos_dry_run_adhoc
  package_macos_bad_args
  package_macos_dry_run_signed
  package_macos_notarize_without_identity
  package_macos_notarize_missing_credentials
  check_docs_gate
  check_docs_instructions
  check_docs_release_version
  package_cli_archives
  third_party_notices
  render_homebrew_formula
  install_script
  agent_eval_harness
  readme_plain_reader
)

# run_case NAME: run one case in a subshell with errexit active (not inside
# an if or || list, where bash would silently ignore set -e).
run_case() {
  local name="$1" log="$TS_WORK/$1.log" rc
  set +e
  (
    set -euo pipefail
    "case_$name"
  ) >"$log" 2>&1
  rc=$?
  set -e
  if [ "$rc" -eq 0 ]; then
    printf 'ok: %s\n' "$name"
    return 0
  fi
  printf 'FAIL: %s\n' "$name"
  sed 's/^/    /' "$log"
  return 1
}

main() {
  local selected name failures=0
  if [ "$#" -gt 0 ]; then
    for name in "$@"; do
      if ! declare -F "case_$name" >/dev/null; then
        printf 'error: unknown case: %s\n' "$name" >&2
        exit 2
      fi
    done
    selected=("$@")
  else
    selected=("${CASES[@]}")
  fi
  command -v go >/dev/null 2>&1 || {
    printf 'error: go is not on PATH\n' >&2
    exit 1
  }
  for name in "${selected[@]}"; do
    run_case "$name" || failures=$((failures + 1))
  done
  if [ "$failures" -eq 0 ]; then
    printf 'all script tests passed\n'
    return 0
  fi
  printf '%s script test(s) failed\n' "$failures"
  return 1
}

main "$@"
