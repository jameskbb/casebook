# Development

This guide takes a contributor from a fresh clone to green checks and working builds. End users never need any of it: the release packages contain everything Casebook needs, with no Go, Node or terminal required.

For how the code fits together, read [the architecture overview](architecture.md) and the Layout list in [CLAUDE.md](../CLAUDE.md). For how releases are built, signed and published, read [Building releases](building-releases.md).

## Prerequisites

| Tool | Version | Needed for |
|------|---------|------------|
| Go | the `go` directive in `go.mod` (currently 1.27) | everything |
| Node with npm | 24 | the desktop frontend only (`desktop/frontend`) |
| Wails CLI | v2.14.0 | desktop development and native desktop builds |
| shellcheck | any recent release | linting `scripts/*.sh` |
| actionlint | v1.7.12 (the version CI installs) | linting `.github/workflows/*.yml` |

Install the Wails CLI and actionlint with Go:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.14.0
go install github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
```

Both land in `$(go env GOPATH)/bin`, which must be on your PATH. Run `wails doctor` to see what else Wails wants on your system.

Per operating system:

- **Windows:** the Microsoft WebView2 runtime (built into Windows 11). Native desktop builds also need NSIS (`makensis`) for the installer.
- **macOS:** the Xcode Command Line Tools (`xcode-select --install`). Wails needs cgo on macOS, and the packaging script uses `lipo`, `codesign` and `hdiutil` from them.
- **Linux:** desktop development needs the GTK 3 and WebKitGTK development packages and a C compiler. Linux is not a packaging target ([ADR 0010](decisions/0010-build-and-ci-environment.md)): only the CLI is built for Linux.
- **WSL:** builds and tests all Go code and cross-compiles both Windows executables without cgo, gcc or WebKit. Run the results on the Windows host.

## Quick checks

Run these from the repository root before you open a pull request. Each one must pass:

```sh
CGO_ENABLED=0 go build ./...
go vet ./...
CGO_ENABLED=0 go test ./...
scripts/test-scripts.sh
shellcheck scripts/*.sh
actionlint
scripts/check-docs.sh
```

- `scripts/test-scripts.sh` is the self-test harness for the build scripts. Every case runs in a throwaway fixture copy, so it never touches your `dist/` or `desktop/`. Pass case names to run only those cases.
- `actionlint` with no arguments checks every workflow under `.github/workflows/`.
- `scripts/check-docs.sh` is the documentation gate: no em dash characters in any text file, working relative links, every guide listed in the README, and the signing secret names in `.github/workflows/release.yml` matching [Building releases](building-releases.md).

More checks:

- **Race detector:** `go test -race ./...` needs cgo and a C compiler (`CGO_ENABLED=1`). CI runs it on Linux for every push and every pull request. Run it locally before you push changes that touch concurrency, when your machine has gcc or clang.
- **Frontend:** `npm --prefix desktop/frontend ci`, then `npm --prefix desktop/frontend test`. CI also runs the `lint` and `build` scripts when `desktop/frontend/package.json` defines them.
- **Compiling Go before the frontend is built:** the desktop package embeds `desktop/frontend/dist`, so `go build ./...` fails until that folder has files. `scripts/stub-frontend-dist.sh` writes a placeholder `desktop/frontend/dist/index.html` for compile-only checks. It never overwrites an existing build, it skips while `desktop/frontend` does not exist, and it refuses to run in release mode.

## Running in development

Desktop app, with live reload of the frontend:

```sh
cd desktop
wails dev
```

`wails dev` installs and starts the Vite dev server (`npm run dev`, from `desktop/wails.json`) and rebuilds the Go side when it changes. It needs the native WebView toolchain for your OS (WebView2 on Windows, the Xcode tools on macOS, GTK and WebKitGTK on Linux), so it does not run inside WSL; on WSL, cross-compile with `scripts/build-windows.sh` and run the result on the Windows host. The desktop app takes two flags, which also work after `wails dev --appargs`:

- `--data-dir <dir>` opens `<dir>/casebook.db` instead of the per-user database. Use it for every manual test.
- `--seed-demo` fills an empty, non-default database with the demo cases used by the screenshots (`internal/demo`; the featured case is INV-0002 "Northeast margin down 6 points in Q3"). `--seed-demo-stage before-thursday` stops before Claude Code's Thursday checkpoint, which `desktop/tools/aiwrite --checkpoint` then writes live.

The page talks to Go only through the committed bindings in `desktop/frontend/wailsjs/`. After you add or change an exported method on the `Desktop` struct (`desktop/*.go`), regenerate them with `desktop/scripts/gen-bindings.sh`, add the wrapper to `desktop/frontend/src/api/api.ts` and the stub to `src/test/mockApi.ts`, and commit all three. Frontend tests run with `npm --prefix desktop/frontend test -- --run`.

On a WSL machine with a Windows host, `desktop/scripts/` drives the real app: `win-run.sh build` cross-compiles `desktop/build/bin/Casebook.exe`, `win-smoke.sh` runs the end-to-end smoke (fresh launch, live refresh from a separate writer process, clipboard, restart persistence, brief steps 2 to 11), and `win-run.sh ui` runs the window automation in `win-ui.ps1` (wait for the window, find, invoke, capture). They keep all data under `%LOCALAPPDATA%\Temp\casebook-dev` and never touch the real per-user database.

Known issue: with Wails 2.14, if something gives the main window focus while a native open or save dialog is showing, WebView2 refuses the focus change and Wails exits the app. Normal clicks and Alt+Tab activate the dialog instead, so people rarely hit it; automation scripts must finish a file dialog before they touch the main window. The fix belongs upstream in Wails.

The CLI and MCP server:

```sh
go run ./cmd/casebook <command>
go run ./cmd/casebook mcp      # the MCP server on stdin and stdout
```

`go run ./cmd/casebook --version` prints `casebook dev`: a plain `go run` or `go build` is unstamped. The build scripts stamp the version (see [Version stamping](#version-stamping)). `go run ./cmd/casebook` with no arguments prints the status of the database it would use, then the help.

Use a throwaway database so development never touches your real data. `CASEBOOK_HOME` points Casebook at a folder (it uses `casebook.db` inside it) and `CASEBOOK_DB` at an exact database file; `CASEBOOK_DB` wins when both are set. [Privacy and local data](privacy.md) lists the default locations.

```sh
export CASEBOOK_HOME="$(mktemp -d)"
go run ./cmd/casebook status
```

In a fresh folder `status` reports that the database is not created yet (and exits 0) rather than creating it; the first command that records data creates it.

## Version stamping

Both executables report one version, set at build time through the `internal/version` package (`version.Version`, default `dev`, and `version.Commit`, default empty). A stamped build also sets `version.Marker` to `casebook-version=<version>`; release checks look for that text in each executable, because only the `-X` flag can produce it. Every build gets its values from `scripts/version.sh`:

| Subcommand | Prints |
|------------|--------|
| `scripts/version.sh version` | `CASEBOOK_VERSION` when set, else `git describe --tags --always --dirty`, else `dev` |
| `scripts/version.sh commit` | `CASEBOOK_COMMIT` when set, else `git rev-parse --short HEAD`, else an empty line |
| `scripts/version.sh numeric` | `MAJOR.MINOR.PATCH` from the start of the version (a leading `v` is dropped), else `0.0.0`; used for Windows file properties, the installer and the macOS bundle version |
| `scripts/version.sh marker` | `casebook-version=<version>`, the stamp marker release checks search for |
| `scripts/version.sh ldflags` | the `-X` flags that stamp `github.com/jameskbb/casebook/internal/version` (`Version`, `Commit` and `Marker`) |

Overrides are validated because they flow into linker flags and shell commands. `CASEBOOK_VERSION` must match `^[0-9A-Za-z][0-9A-Za-z._+-]*$` and be at most 128 bytes; `CASEBOOK_COMMIT` must be 4 to 40 lower-case hex digits. Anything else exits with code 2. The git fallbacks apply only when the repository root is itself the top of a git work tree.

The CLI build, the Windows cross-compile, the macOS packaging script and the release workflow all pass the same `scripts/version.sh ldflags` string to both the desktop app and the CLI, so both executables always carry the same value:

```sh
CASEBOOK_VERSION=v0.0.0-local scripts/build-cli.sh
```

## Development and release mode

Casebook is built in parallel tracks, so the scripts cope with parts that do not exist yet:

- **Development mode** (the default): a missing part is skipped with a `skip:` line on stderr and the script exits 0. The exact lines are `skip: cmd/casebook not present yet (track B)`, `skip: desktop not present yet (track A)`, `skip: desktop/frontend not present yet (track A)` and `skip: desktop/frontend/package.json not present yet (track A)`.
- **Release mode** (`--release` on any script, or `CASEBOOK_RELEASE=1` in the environment): a missing part fails with `error: missing <track> (<path>) in release mode` and exit code 1. The release workflow always runs in release mode, so it can never ship a partial product.

`scripts/preflight.sh` reports which release parts are present. It prints `ok: <path>` or `missing: <path> (<track>)` for `cmd/casebook`, `desktop/wails.json` and `desktop/frontend/package.json`; with `--release` it exits 1 when anything is missing.

Every script uses the same exit codes: 0 for success or a development skip, 1 for a build failure or a missing part in release mode, 2 for a usage error or an invalid version value. Each one prints its usage with `-h` or `--help`.

## Building per target

All build output goes to `dist/` at the repository root. The scripts create `dist/.gitignore` containing `*`, so the folder ignores itself and nothing in it is ever committed.

Both release layouts follow [ADR 0012](decisions/0012-cli-placement.md): the CLI lives in a `bin` folder next to the desktop executable, because `Casebook.exe` and `casebook.exe` are the same path on the case-insensitive file systems of Windows and macOS.

### The CLI, for any OS

`scripts/build-cli.sh` builds the CLI without cgo for any supported target:

```sh
scripts/build-cli.sh                                # this machine's OS and architecture
scripts/build-cli.sh --os windows --arch arm64      # dist/casebook-windows-arm64.exe
scripts/build-cli.sh --os darwin --arch amd64 --out /tmp/casebook
```

Flags: `--os windows|darwin|linux` and `--arch amd64|arm64` (both default to `go env`), `--out PATH` (default `dist/casebook-<os>-<arch>`, plus `.exe` for Windows) and `--release`. It builds with `-trimpath` and prints `built: <path> (<version>)`.

### Windows, from WSL or Linux

`scripts/build-windows.sh` cross-compiles both Windows executables without cgo:

```sh
scripts/build-windows.sh                    # amd64
scripts/build-windows.sh --arch arm64
scripts/build-windows.sh --arch all         # amd64 and arm64
scripts/build-windows.sh --skip-frontend    # reuse an existing desktop/frontend/dist
```

It builds the frontend once (`npm ci` and `npm run build` in `desktop/frontend`, unless you pass `--skip-frontend`), then for each architecture writes:

```text
dist/windows-<arch>/Casebook.exe
dist/windows-<arch>/bin/casebook.exe
```

Copy the whole `dist/windows-<arch>` folder to the Windows host (for example to a folder under `C:\Users\<you>`) and run `Casebook.exe` there; running it from a `\\wsl$` path works less reliably. The desktop executable is built with `-tags desktop,production` and `-H windowsgui`. `go build` links any `.syso` resource file committed in `desktop/`, but `wails build` generates its `.syso` (icon, version information and application manifest) only for the duration of its own build and deletes it afterwards, so usually none is committed. A WSL build of `Casebook.exe` therefore normally has no icon, no version information and no application manifest. Use it for testing; the release zips and installers come from `wails build`, which includes all three. The script fails when the two executables would end up as one file. This build has no installer; installers are built on Windows (below) or by the release workflow.

### Windows, natively

On Windows, use Git Bash with Go, Node 24, the Wails CLI and NSIS installed. The repository's `.gitattributes` keeps LF line endings in every checkout, so the bash scripts work even with Git for Windows' default `core.autocrlf=true`. A clone made before `.gitattributes` existed may still hold CRLF files; `git rm -r --cached -q . && git reset --hard` rewrites them (it discards uncommitted changes). These steps mirror the `windows` job in `.github/workflows/release.yml`:

```sh
scripts/build-cli.sh --os windows --arch amd64 --out dist/stage/bin/casebook.exe
export CASEBOOK_CLI_BINARY="$(cygpath -w "$PWD/dist/stage/bin/casebook.exe")"
mkdir -p desktop/build/windows/installer
cp -f packaging/windows/* desktop/build/windows/installer/
cd desktop
ldflags="$(../scripts/version.sh ldflags)"
wails build -clean -m -nosyncgomod -trimpath -nsis -platform windows/amd64 -ldflags "$ldflags"
```

The desktop app lands in `desktop/build/bin/Casebook.exe` and the installer in `desktop/build/bin/` with a name ending in `-installer.exe`. The NSIS project in `packaging/windows/project.nsi` reads `CASEBOOK_CLI_BINARY` to install `bin\casebook.exe` next to `Casebook.exe`, and stops with `CASEBOOK_CLI_BINARY does not name an existing casebook.exe` when the variable is unset or does not point at a file. `desktop/build/windows/installer` is a build-time copy; edit the files in `packaging/windows/` instead.

### macOS, on a Mac

macOS builds need a Mac (Wails uses cgo there). Build the universal app, then package it:

```sh
cd desktop
ldflags="$(../scripts/version.sh ldflags)"
wails build -clean -m -nosyncgomod -trimpath -platform darwin/universal -ldflags "$ldflags"
cd ..
scripts/package-macos.sh --app desktop/build/bin/Casebook.app
```

`scripts/package-macos.sh` builds the arm64 and amd64 CLI, merges them with `lipo` into `Casebook.app/Contents/MacOS/bin/casebook`, checks that both executables carry the stamp marker, signs the CLI and then the app (ad-hoc unless `CASEBOOK_SIGN_IDENTITY` is set), notarizes and staples the app when `CASEBOOK_NOTARIZE=1`, and writes `dist/Casebook-<version>-macos-universal.dmg` from that app (signed, notarized and stapled in turn when asked to). It modifies the app bundle in place. Flags: `--app PATH` (required), `--out DIR` (default `dist/`), `--release` and `--dry-run`.

`--dry-run` prints the whole command plan on any OS without building, signing or writing anything, which is useful for reviewing changes from WSL or Linux:

```sh
mkdir -p /tmp/Casebook.app/Contents/MacOS
touch /tmp/Casebook.app/Contents/MacOS/Casebook
scripts/package-macos.sh --dry-run --app /tmp/Casebook.app
```

Even a dry run checks that the app bundle and its `Contents/MacOS/Casebook` exist, so the example creates an empty stand-in. Like the other scripts, it skips while `cmd/casebook` does not exist. Signing and notarization inputs are described in [Building releases](building-releases.md).

### Linux

Only the CLI: `scripts/build-cli.sh --os linux --arch amd64` (or `arm64`). There is no Linux desktop package.

## The README example

The compact pack in the README is real engine output. `scripts/readme-example` builds the Northeast margin case through `internal/app` (Claude Code, Codex and a person in the desktop app, on a fixed clock) in a temporary database and prints its pack:

```sh
go run ./scripts/readme-example                 # as casebook context --format compact
go run ./scripts/readme-example -format md      # or json; -db <new file> keeps the database
```

When the pack format changes, rerun it and paste the output (shortened the same way) into the README.

## Continuous integration

`.github/workflows/ci.yml` runs on every push (any branch or tag), on every pull request, and on demand (Run workflow), with a read-only token and no secrets. A pull request from a branch in this repository therefore runs each check twice, which is accepted so that no pushed commit goes untested. A failing check, including any failing test, fails the build.

| Job | Runs on | Checks |
|-----|---------|--------|
| `go` (Go (ubuntu-latest), Go (windows-latest), Go (macos-latest)) | Linux, Windows, macOS, `CGO_ENABLED=0` | stubs the frontend dist, then `go build ./...`, `go vet ./...` and `go test -count=1 ./...`; on Linux also the version stamping test (`TestStamped` with the `scripts/version.sh ldflags` flags) and `scripts/build-cli.sh` |
| `race` (Go race (ubuntu-latest)) | Linux, `CGO_ENABLED=1` | `go test -race -count=1 -timeout 20m ./...` |
| `frontend` (Frontend) | Linux, Node 24 | `npm ci`, then the `lint`, `test` and `build` scripts that `desktop/frontend/package.json` defines; skips while `desktop/frontend` does not exist |
| `scripts` (Scripts and workflows) | Linux | shellcheck on `scripts/*.sh`, `packaging/**/*.sh` and `.githooks/pre-push`, actionlint, `scripts/test-scripts.sh`, `scripts/check-docs.sh`, and a check that no two tracked paths differ only by case |

The release workflow calls this same workflow (`workflow_call`) as its first job, so a tag is built only when every check passes on it. Releases come from `.github/workflows/release.yml`, described in [Building releases](building-releases.md).

### The pre-push gate

CI reports after a push; it cannot stop one, and the repository does not rely on GitHub branch protection. `.githooks/pre-push` closes that gap locally: before any push that updates `main` it runs `go vet`, the Go tests, the frontend tests, the script self-tests and the docs check, and refuses the push when one fails. Enable it once per clone:

```sh
git config core.hooksPath .githooks
```

Pushes to other branches and tag pushes skip it. `CASEBOOK_SKIP_PREPUSH=1 git push ...` bypasses it for an emergency; CI still runs afterwards.

## Troubleshooting

- **`pattern all:frontend/dist: no matching files found`:** the desktop package embeds the built frontend. Run `scripts/stub-frontend-dist.sh` for a compile-only placeholder, or build the real frontend with `npm --prefix desktop/frontend ci` and `npm --prefix desktop/frontend run build`.
- **`npm is not on PATH` from `scripts/build-windows.sh`:** install Node 24, or pass `--skip-frontend` to reuse an existing `desktop/frontend/dist`.
- **Wails warns that `makensis` was not found:** Wails still exits 0 but writes no installer. Install NSIS and make sure `makensis /VERSION` works in the same shell.
- **`hdiutil: create failed - Resource busy`:** `scripts/package-macos.sh` already retries three times, five seconds apart. Close any Finder window showing the DMG or an earlier Casebook volume, eject it, and run the script again.
- **`invalid version` with exit code 2:** `CASEBOOK_VERSION` or `CASEBOOK_COMMIT` holds characters outside the allowed set; see [Version stamping](#version-stamping).
