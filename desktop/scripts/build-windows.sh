#!/usr/bin/env bash
# Builds desktop/build/bin/Casebook.exe for Windows from WSL or Linux (D-27).
# Tries the wails CLI first (it embeds the icon, manifest and version info),
# then falls back to a plain cross-compiling go build. Fails when the build
# itself changed tracked files (edits made before the build are ignored).
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
cd "$ROOT"
command -v go >/dev/null || { echo "go is not on PATH; install Go 1.27 or later" >&2; exit 1; }
gopath=$(go env GOPATH)
export PATH="$gopath/bin:$PATH"

tracked_state() {
  git status --porcelain --untracked-files=no desktop
  git diff --no-ext-diff desktop | git hash-object --stdin
}
before=$(tracked_state)

if [ ! -d desktop/frontend/node_modules ]; then
  npm --prefix desktop/frontend install
fi
npm --prefix desktop/frontend run build

method=""
if command -v wails >/dev/null 2>&1 &&
  (cd desktop && CGO_ENABLED=0 wails build -platform windows/amd64 -s -skipbindings -nocolour); then
  method="wails CLI"
else
  echo "build-windows: wails build failed or is missing; falling back to go build" >&2
  mkdir -p desktop/build/bin
  GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -tags desktop,production -ldflags "-H windowsgui" \
    -o desktop/build/bin/Casebook.exe ./desktop
  method="go build fallback"
fi

test -s desktop/build/bin/Casebook.exe
echo "build-windows: built desktop/build/bin/Casebook.exe with the $method"

after=$(tracked_state)
if [ "$before" != "$after" ]; then
  echo "build-windows: the build changed tracked files:" >&2
  git status --porcelain --untracked-files=no desktop >&2
  exit 1
fi
