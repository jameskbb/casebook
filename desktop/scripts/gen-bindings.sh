#!/usr/bin/env bash
# Regenerates the committed Wails bindings in desktop/frontend/wailsjs (D-03)
# from the Desktop struct. `wails generate module` writes them executable
# while `wails build` writes them 0644, so the modes are normalized to 0644
# to keep git status clean either way.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
command -v go >/dev/null || { echo "go is not on PATH; install Go 1.27 or later" >&2; exit 1; }
gopath=$(go env GOPATH)
export PATH="$gopath/bin:$PATH"
cd "$ROOT/desktop"
CGO_ENABLED=0 wails generate module -nocolour >/dev/null
find frontend/wailsjs -type f -exec chmod 0644 {} +
git -C "$ROOT" status --short desktop/frontend/wailsjs
