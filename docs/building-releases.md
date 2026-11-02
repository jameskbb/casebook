# Building releases

Casebook releases are built by GitHub Actions, never on a maintainer's machine. Pushing a tag named like `v1.2.3` runs `.github/workflows/release.yml` in release mode. The workflow builds the Windows and macOS desktop packages and the CLI-only archives for Linux, macOS and Windows from one stamped version, writes the third-party license notices and a checksum file, and attaches everything to a **draft** GitHub release. A maintainer reviews the draft and publishes it by hand. Publishing it updates the Homebrew tap (see [Homebrew tap](#homebrew-tap)).

This guide is for maintainers. To build from source for development, see [Development](development.md). How users install and update is in [Updating Casebook](updating.md).

## Before the first public release

Do these once, in this order. None of them is automated.

0. **Tags.** Only released versions carry tags. `git describe` takes the newest tag, so an older tag never affects `casebook --version`.
1. **Add the license.** Commit a `LICENSE` file at the repository root ([ADR 0011](decisions/0011-license.md) recommends MIT). A release refuses to build without it: the `notices` job fails, and `scripts/package-cli.sh --release` fails, because every archive ships the license.
2. **Create the Homebrew tap.** Create the public repository `jameskbb/homebrew-tap` with a `Formula/` folder (an empty `Formula/.gitkeep` is enough) and a short README. `brew install jameskbb/tap/casebook` reads `Formula/casebook.rb` from it.
3. **Create the tap token.** In GitHub, Settings, Developer settings, Fine-grained personal access tokens, create a token with access to the `jameskbb/homebrew-tap` repository only and the permission Contents: Read and write. Save it as the `jameskbb/casebook` repository secret `HOMEBREW_TAP_TOKEN`. Give it an expiry you will notice and renew it before then. Without it, the tap update is skipped with a notice and you update the formula by hand.
4. **Optionally add the signing secrets** (see [Signing and notarization](#signing-and-notarization)). Releases build and publish unsigned without them.
5. **Tag the first release** as described below, review the draft and publish it. Then check the three one-line installs from a clean machine or container:

   ```sh
   curl -fsSL https://raw.githubusercontent.com/jameskbb/casebook/main/scripts/install.sh | sh
   brew install jameskbb/tap/casebook
   go install github.com/jameskbb/casebook/cmd/casebook@latest
   ```

   `install.sh` and Homebrew need a **published, non-prerelease** release: GitHub's "latest release" skips drafts and prereleases. `go install ...@latest` needs a pushed `v*` tag on a public repository (the Go module proxy fetches it on first request) and works even before any release is published.

## Making a release

1. **Choose the version.** Releases follow [Semantic Versioning](https://semver.org/). While the version is 0.x, a change in what a person sees or does (a renamed action, a removed menu item, a new command, a schema migration) is a minor release (`0.4.0`), and a release with only fixes is a patch release (`0.3.1`).
2. **Prepare the release commit** on `main`. Nothing does this for you, and the docs check in step 3 fails the tag when a file still carries the old version:
   - `CHANGELOG.md`: rename `## Unreleased` to `## MAJOR.MINOR.PATCH (YYYY-MM-DD)` with today's date, read the entries as a person who downloads the app would, and leave an empty `## Unreleased` heading above it for the next release if you like (the check accepts an empty one).
   - `desktop/wails.json`: set `info.productVersion` to `MAJOR.MINOR.PATCH`. The workflow rewrites it on the runner anyway, but the committed value is what a source build and `git blame` show.
   - The frontend package and its lock file: `npm --prefix desktop/frontend version MAJOR.MINOR.PATCH --no-git-tag-version`.
   - `docs/acceptance.md`, when this release re-verified something (the 0.3.0 release added a no-network rerun); a release that only changes copy adds nothing there.
   - Screenshots and the README text that describes them are not a release step: every UI change re-shoots them in its own change (`CLAUDE.md`).
   Commit, push, and make sure CI is green on `main` for that commit. The workflow enforces this too: its first job, `tests`, reruns the whole CI suite on the tagged commit, and nothing is built unless it passes.
3. Run the release documentation check from the repository root, with the tag you are about to create:

   ```sh
   CASEBOOK_VERSION=v1.2.3 scripts/check-docs.sh --release
   ```

   It must pass. In release mode it also fails on any `FINALIZE(phase 6)` marker left in the docs and on any linked file that the desktop or CLI work has not delivered yet, so no release ships with placeholder documentation; with a stable `CASEBOOK_VERSION` it also fails when `CHANGELOG.md` has no section for that version or still lists entries under `## Unreleased`, or when `desktop/wails.json` or the frontend package carries another version. The release workflow runs the same check in its `version` job, with the tag's version, and stops before building anything when it fails; running it first saves a failed tag push. Optionally run `scripts/preflight.sh --release` too; it fails when `cmd/casebook`, `desktop/wails.json` or `desktop/frontend/package.json` is missing.
4. Create and push an annotated tag:

   ```sh
   git tag -a v1.2.3 -m "Casebook v1.2.3"
   git push origin v1.2.3
   ```

   The tag must match `vMAJOR.MINOR.PATCH`, optionally followed by a `-prerelease` suffix (for example `v1.2.3-rc.1`). SemVer build metadata (`+build.5`) is not allowed: the version is part of every asset name, and GitHub renames uploaded assets whose names contain special characters, which would break `SHA256SUMS.txt`. Any other tag that starts with `v` fails the `version` job, and so does a dry-run version that contains `+`.
5. Watch the run under the repository's Actions tab, workflow Release.
6. When it succeeds, open the draft release under Releases. Check that it has the thirteen files (see [Assets](#assets)), read the release notes and edit them, then publish. The notes open with a "Which file do I need?" table, already written by the workflow (see [The download table](#the-download-table)), followed by the version's entries from the tagged changelog and GitHub's generated list of changes. To publish from the command line instead, run the Publish release workflow: `gh workflow run publish-release.yml -f tag=v1.2.3` (Actions tab, "Publish release"). It checks that the release is still a draft, that it holds the thirteen files and that `SHA256SUMS.txt` matches them, then publishes it (as a pre-release when the tag has a suffix, otherwise as the latest release). After a stable release it also marks every earlier published release as superseded (one notice line at the top of its notes, naming the new release; nothing is deleted and no file is removed) and starts the Homebrew tap workflow itself, since a workflow publish does not trigger the tap on its own (GitHub never starts a workflow from an ordinary event created with the Actions token). A pre-release supersedes nothing and leaves the tap alone. When you publish by hand from the Releases page, the tap runs on its own; write the notices afterwards with `gh workflow run publish-release.yml -f tag=v1.2.3 -f supersede_only=true`.
7. Publishing runs `.github/workflows/homebrew.yml`, which updates the Homebrew formula. Check its run under Actions, workflow Homebrew tap.

A tag whose version contains a hyphen (`v1.2.3-rc.1`) becomes a prerelease. The release is named `Casebook <version>`. A prerelease never becomes GitHub's "latest release", so `install.sh` and Homebrew keep installing the previous stable release; `CASEBOOK_VERSION=v1.2.3-rc.1` makes `install.sh` install a prerelease on purpose.

## Dry runs

A dry run builds every package without creating a release:

1. Open the repository's Actions tab, choose the Release workflow, and press Run workflow.
2. Optionally enter a version such as `v0.0.0-dryrun.1`. Leave it empty to use `git describe` for the selected branch.
3. When the run finishes, download the `release-bundle` artifact from the run's summary page. It holds the thirteen files, exactly as a tag release would attach them. GitHub deletes it after 7 days, so download it within the week or run the workflow again.

The `workflow_dispatch` trigger runs every job except `publish`, so nothing is published and no tag is needed. The per-job artifacts (`casebook-third-party-notices`, `casebook-cli-linux`, `casebook-windows-amd64`, `casebook-windows-arm64`, `casebook-macos-universal`) are kept for 3 days, long enough to re-run failed jobs; each run uploads about 300 MB, so short retention keeps the account's Actions storage from filling up. A dry run also needs `LICENSE` on the selected branch.

## What the workflow does

Every third-party action in `.github/workflows/` is pinned to a full commit SHA, with its version in a trailing comment (`uses: actions/checkout@<40 hex digits> # v7.0.1`), so a moved or compromised tag cannot change what runs next to the signing secrets. `scripts/check-docs.sh` fails on any `uses:` line that is not pinned this way, and Dependabot (`.github/dependabot.yml`) opens a weekly pull request when a newer version of a pinned action is released.

The workflow sets `CASEBOOK_RELEASE=1` for every job, so every script runs in release mode: a missing part (`cmd/casebook` or `desktop/`) fails the run instead of producing a partial package. It has a read-only token everywhere except the `publish` job.

| Job | Runner | What it does |
|-----|--------|--------------|
| `tests` | the CI matrix | Calls `.github/workflows/ci.yml` (Go on Linux, Windows and macOS, the race detector, frontend, scripts and docs). Every other job waits for it, so a commit with a failing test never becomes a release. |
| `version` | ubuntu-latest, after `tests` | Runs `scripts/preflight.sh --release` and `scripts/check-docs.sh --release`. Validates the tag (or takes the dry-run input, or `git describe`), then outputs the version, the numeric `MAJOR.MINOR.PATCH` version and the short commit from `scripts/version.sh`. |
| `notices` | ubuntu-latest | Installs the locked frontend dependencies (`npm ci --ignore-scripts`), downloads the Go modules, and runs `scripts/third-party-notices.sh --release`, which writes `THIRD_PARTY_NOTICES.txt`: the license and notice files of the Go standard library, of every Go module linked into the CLI or the desktop app on any release platform, and of every production npm package bundled into the frontend. A package without a license file, or a missing `LICENSE` at the repository root, fails the job. Uploads the file as `casebook-third-party-notices`. |
| `cli-linux` | ubuntu-latest | Builds the CGO-free Linux CLI for `amd64` and `arm64` with `scripts/build-cli.sh --release`, packages each with `scripts/package-cli.sh --release` (which checks the stamp marker), checks that the amd64 CLI prints the version and that each archive holds exactly `casebook`, `LICENSE` and `THIRD_PARTY_NOTICES.txt`. |
| `windows` | windows-latest, matrix `amd64` and `arm64` | Installs the Wails CLI v2.14.0 and, when the runner image lacks it (windows-2025 does), NSIS 3.11: two attempts at the pinned Chocolatey package, then the official `nsis-3.11.zip` from SourceForge checked against a pinned SHA-256. Builds the CLI with `scripts/build-cli.sh --release`, copies `packaging/windows/` into `desktop/build/windows/installer`, and runs `wails build -nsis` for `windows/<arch>`. Checks the outputs, optionally signs, then packages the portable zip, `Casebook-<version>-windows-<arch>-portable.zip` (with `packaging/windows/README.txt`, its `@VERSION@` placeholder replaced by the version and its line endings converted to CRLF, `LICENSE`, `THIRD_PARTY_NOTICES.txt`, and a check that the zip holds exactly those five files), the installer, and the CLI-only zip from the same (signed, with the secrets) `bin/casebook.exe`. |
| `macos` | macos-latest | Runs `wails build -platform darwin/universal`, optionally imports the Developer ID certificate into a temporary keychain, then runs `scripts/package-macos.sh --release --app desktop/build/bin/Casebook.app --out dist/out`, which adds the CLI, signs, optionally notarizes and staples the app, builds the DMG, then signs and optionally notarizes and staples the DMG. `LICENSE` and `THIRD_PARTY_NOTICES.txt` go into `Casebook.app/Contents/Resources` before signing. Afterwards it cuts the signed universal CLI inside the app into its `arm64` and `x86_64` slices with `lipo -thin` (each slice keeps its own signature, which `codesign --verify` checks), and packages the two darwin CLI archives. |
| `checksums` | ubuntu-latest | Downloads every `casebook-*` artifact, requires exactly the twelve files that the workflow's `RELEASE_ASSETS` list names (all non-empty), writes `SHA256SUMS.txt`, checks it with `sha256sum -c`, and uploads the `release-bundle` artifact. |
| `publish` | ubuntu-latest, tag pushes only | Downloads `release-bundle`, checks the sums again and that it holds thirteen files, writes the download table from `RELEASE_ASSETS` (failing when a file it names is not in the bundle), and creates the draft release with those files using the runner's GitHub CLI (`gh release create --draft --verify-tag --generate-notes --notes <the table>`) and the job's own token. No third-party action runs in this job, the only one with write access. It fails when a release for the tag already exists; delete that release before re-running the job. |

## Version stamping

Every release has one version, taken from the tag (`v1.2.3`) or the dry-run input. The `version` job passes it to the build jobs, which set `CASEBOOK_VERSION` and `CASEBOOK_COMMIT`. Both executables are built with the same `scripts/version.sh ldflags` string, which sets `version.Version`, `version.Commit` and `version.Marker` in `internal/version`. `version.Marker` is the text `casebook-version=<version>`, which only that `-X` flag can put into an executable: the bare version string can also come from Go's own build information (the tag's VCS version or a dependency version), so a check for it would pass even with a broken stamp. Release builds use `-trimpath` and `-buildvcs=false` (the CLI build script passes the flag; the Wails steps set `GOFLAGS=-buildvcs=false`), so the build information carries neither the linker flags nor the tag.

The numeric form from `scripts/version.sh numeric` (`1.2.3` for `v1.2.3-rc.1`) goes where only numbers are allowed: the Windows file version properties, the installer's version information and the macOS bundle version. The workflow writes it into `desktop/wails.json` on the runner only; that change is never committed. The same runner-only edit sets the Wails `frontend:install` command to `npm ci --ignore-scripts`, so `wails build` installs exactly the locked dependencies the CI `frontend` job tested (which installs the same way) without running any dependency's install scripts on the runner that later signs the binaries, and each build job fails when the build changed `go.mod`, `go.sum` or `desktop/frontend/package-lock.json`.

The jobs check the stamp before packaging anything:

- **Windows:** `Casebook.exe` and `bin\casebook.exe` must be two different files (not the same path, not identical contents), and both must contain the stamp marker `casebook-version=<version>` (`scripts/version.sh marker`). On amd64, `casebook.exe --version` (or `casebook.exe version`) must print the version.
- **macOS:** `scripts/package-macos.sh` checks that both executables contain the stamp marker, that the CLI holds both arm64 and x86_64 code, and that the signed CLI prints the version.

The CLI answers both `casebook --version` and `casebook version` with `casebook <version>` (`casebook version --json` prints `{"version":"<version>"}`). The desktop app shows the same stamped value as the last item of its **Data** menu, **Version**; choosing it copies `Casebook <version>` for bug reports.

A build that was not stamped, such as `go install github.com/jameskbb/casebook/cmd/casebook@v1.2.3`, reports the module version Go records in its build information instead (`v1.2.3`, or a pseudo-version such as `v0.1.2-0.20261006183454-ecd10f849e61` for an untagged commit; a `go build` in a checkout with uncommitted changes adds `+dirty`). Only builds with no module version at all (`go run`, `-buildvcs=false` without the stamp) report `dev`. The fallback never sets the stamp marker, so `casebook doctor`'s `bundle_version` check and the release checks still see such a build as unstamped. The CI `go` job builds `cmd/casebook` exactly as `go install` does (CGO-free, no linker flags) and runs `casebook --version`.

## Assets

Every release has exactly these thirteen files. `<version>` is the full version, for example `v1.2.3`. The desktop packages start with `Casebook-`; the CLI-only archives start with `casebook-cli-`, so the two never collide on a case-insensitive file system. The Windows desktop files say what they are: `-portable.zip` runs where it is unzipped, `-installer.exe` installs. (Releases before 0.3.0 named the portable zip `Casebook-<version>-windows-<arch>.zip`.)

| File | Contents and layout |
|------|---------------------|
| `Casebook-<version>-windows-amd64-portable.zip` | The primary Windows download, for 64-bit Intel and AMD PCs: a portable folder that runs where it is unzipped, with no installer. `Casebook.exe` at the root, `bin\casebook.exe` next to it, `README.txt` (from `packaging/windows/README.txt`, stamped with the version), which explains where the data lives, how to update the folder in place and where backups go, plus `LICENSE` and `THIRD_PARTY_NOTICES.txt`. |
| `Casebook-<version>-windows-arm64-portable.zip` | The same portable zip for Windows on Arm. |
| `Casebook-<version>-windows-amd64-installer.exe` | The alternative for people who want Start menu entries and an uninstaller: an NSIS installer that installs `Casebook.exe` and `bin\casebook.exe` into the install folder, adds Start menu and desktop shortcuts, and offers an unchecked option to add `<install folder>\bin` to PATH. It does not install `LICENSE` and `THIRD_PARTY_NOTICES.txt` yet; they ship next to it in the release. |
| `Casebook-<version>-windows-arm64-installer.exe` | The same installer for Windows on Arm. |
| `Casebook-<version>-macos-universal.dmg` | `Casebook.app` (Apple Silicon and Intel) and an Applications shortcut. The CLI is inside the app at `Casebook.app/Contents/MacOS/bin/casebook`, and the desktop executable at `Casebook.app/Contents/MacOS/Casebook`. `LICENSE` and `THIRD_PARTY_NOTICES.txt` are in `Casebook.app/Contents/Resources`. |
| `casebook-cli-<version>-linux-amd64.tar.gz` | The CLI alone (it also serves MCP) for 64-bit Intel and AMD Linux: `casebook`, `LICENSE` and `THIRD_PARTY_NOTICES.txt`, flat, with no folder inside. `scripts/install.sh` and the Homebrew formula install it. |
| `casebook-cli-<version>-linux-arm64.tar.gz` | The same for 64-bit Arm Linux. |
| `casebook-cli-<version>-darwin-arm64.tar.gz` | The same for Apple Silicon Macs: the `arm64` slice of the signed (and, with the secrets, notarized) CLI from `Casebook.app`. |
| `casebook-cli-<version>-darwin-amd64.tar.gz` | The same for Intel Macs: the `x86_64` slice. |
| `casebook-cli-<version>-windows-amd64.zip` | The CLI alone for Windows, with no desktop app: `casebook.exe` (the same signed file as `bin\casebook.exe` in the portable zip), `LICENSE` and `THIRD_PARTY_NOTICES.txt`. |
| `casebook-cli-<version>-windows-arm64.zip` | The same for Windows on Arm. |
| `THIRD_PARTY_NOTICES.txt` | The license texts of everything both executables bundle (from the `notices` job). The same file is inside every zip, every CLI archive and `Casebook.app`. |
| `SHA256SUMS.txt` | SHA-256 checksums of the twelve files above, in `sha256sum` format. |

### The download table

The release body opens with a "Which file do I need?" section, so a first-time downloader on the Releases page knows which file to take. The `publish` job writes it from `RELEASE_ASSETS` in `.github/workflows/release.yml`, the same `group|file|what it is` lines the `checksums` job turns into the expected file set, so a renamed asset changes both or fails the run. It is two tables (Desktop app; Command line tool only), one row per file in list order, then a line on what `-portable.zip` and `-installer.exe` mean, a line pointing to [Check your download](install.md#check-your-download) at the release's tag, and a line on `THIRD_PARTY_NOTICES.txt`. [Installing Casebook](install.md#which-file-do-i-need) carries the same table with `<version>`; change both when you change an asset or its wording. The job passes the section to `gh release create` with `--notes` next to `--generate-notes`, which places it before GitHub's generated notes (`gh release create --help`: "Additional release notes can be prepended to automatically generated notes by using the `--notes` flag"). A dry run never writes it, because it has no `publish` job.

Users update by replacing the whole folder (or app) with a newer release; [Updating Casebook](updating.md) is the user-facing guide, and `packaging/windows/README.txt` is its short form inside the zip. Keep the two in step when the update steps change. Casebook checks for updates only when a person asks ([Checking for a newer version](updating.md#checking-for-a-newer-version)) and never downloads one; `scripts/install.sh`, Homebrew and `go install` download only when a person runs them.

Linux has no desktop package (desktop packaging is deferred, [ADR 0010](decisions/0010-build-and-ci-environment.md)). On Linux the CLI with its MCP server is the whole product: AI clients record and read cases through MCP, and people read them with `casebook context`, `casebook search` and `casebook case`, and back them up with `casebook backup`. The steps that only the desktop app offers today, such as marking a record `human_verified` ([ADR 0014](decisions/0014-only-people-verify.md)), need a Mac or Windows machine; the data format is the same on every platform, so `casebook export` or a backup carries a case there.

The `bin` folder next to the desktop executable is required by [ADR 0012](decisions/0012-cli-placement.md): `Casebook.exe` and `casebook.exe` would otherwise be one file on the case-insensitive file systems of Windows and macOS. Treat this layout as stable, because AI client configurations store the absolute path of the CLI.

The installer's uninstaller removes the PATH entry it added, the files it installed (`Casebook.exe`, `bin\casebook.exe` and `uninstall.exe`) and the shortcuts. It removes the `bin` folder and the install folder only when they are then empty, so it never deletes unrelated files from a folder the user chose. It keeps the user's data in `%LocalAppData%\Casebook`; [Privacy and local data](privacy.md) explains how to delete it.

## One-line installs

There are three one-line installs of the CLI. None needs administrator rights, and only `go install` needs a toolchain (Go):

| Command | What it does |
|---------|--------------|
| `curl -fsSL https://raw.githubusercontent.com/jameskbb/casebook/main/scripts/install.sh \| sh` | `scripts/install.sh` (POSIX `sh`, Linux and macOS) finds the latest published release through the `github.com/jameskbb/casebook/releases/latest` redirect (or the GitHub API when only `wget` is installed), downloads `casebook-cli-<version>-<os>-<arch>.tar.gz` and `SHA256SUMS.txt`, refuses to install on a checksum mismatch, and copies `casebook` into `~/.local/bin` (or `$CASEBOOK_INSTALL_DIR`) with an atomic rename. It never uses `sudo` and never edits shell profiles; it prints the `PATH` line to add when the folder is not on `PATH`, then the next step, `casebook setup claude`. `CASEBOOK_VERSION=v1.2.3` pins a release. On Apple Silicon it installs the arm64 build even from a shell running under Rosetta. |
| `brew install jameskbb/tap/casebook` | The formula in `jameskbb/homebrew-tap` (rendered from `packaging/homebrew/casebook.rb`, see [Homebrew tap](#homebrew-tap)) installs the same archives on macOS and Linux. `casebook setup` records Homebrew's stable `<prefix>/opt/casebook/bin/casebook` link rather than the versioned `Cellar` path, so AI client setup survives `brew upgrade`. |
| `go install github.com/jameskbb/casebook/cmd/casebook@latest` | Builds the CLI from the module proxy with the user's Go toolchain (go.mod's `go` line; an older Go downloads that toolchain automatically). The CLI is pure Go (`modernc.org/sqlite`), so no C compiler is needed, and it reports the module version (see [Version stamping](#version-stamping)). It lands in `$(go env GOPATH)/bin`. |

`scripts/test-scripts.sh` covers `install.sh` with stubbed `curl`, `wget` and `uname` commands and a fake release (success, pinned versions, every OS and CPU branch, checksum mismatch, a missing archive, `wget` only), run under `dash` with a `PATH` that holds only the tools the script may use.

None of the three installs the desktop app: on macOS and Windows that stays a download from the Releases page ([Updating Casebook](updating.md)). A CLI installed this way and the desktop app share the same per-user data folder.

## Homebrew tap

`.github/workflows/homebrew.yml` runs when a release is **published** (not when the release workflow drafts it, because a draft's assets get public download URLs only when it is published), and on demand from the Actions tab with a tag. It skips drafts and prereleases. With the `HOMEBREW_TAP_TOKEN` secret it downloads the release's `SHA256SUMS.txt`, renders the formula with `scripts/render-homebrew-formula.sh`, and commits it as `Formula/casebook.rb` to `jameskbb/homebrew-tap` through the GitHub contents API (no clone, so the token never lands in a git config). Without the secret it prints a notice and succeeds.

| Secret | What it is |
|--------|------------|
| `HOMEBREW_TAP_TOKEN` | A fine-grained personal access token with Contents: Read and write on `jameskbb/homebrew-tap` only (see [Before the first public release](#before-the-first-public-release)). It is used only by the one step that writes the formula. |

To update the tap by hand (no token, or a failed run), from a checkout of this repository:

```sh
gh release download v1.2.3 --repo jameskbb/casebook --pattern SHA256SUMS.txt --dir /tmp/casebook-sums
scripts/render-homebrew-formula.sh --version v1.2.3 --sums /tmp/casebook-sums/SHA256SUMS.txt --out ../homebrew-tap/Formula/casebook.rb
```

then commit and push `Formula/casebook.rb` in the tap. Check it with `brew audit --strict jameskbb/tap/casebook` and `brew test jameskbb/tap/casebook`. The formula's `license` line is taken from `LICENSE` (MIT or Apache-2.0 are recognized); with an unrecognized license the line is left out rather than guessed. Change the formula only in `packaging/homebrew/casebook.rb`; the tap copy is overwritten on every release.

## Signing and notarization

Signing is optional. The workflow signs only when the secrets for a platform are set, and it never fails because they are missing.

### What to obtain

- **Windows: an Authenticode code signing certificate** from a certificate authority (for example DigiCert, Sectigo or SSL.com), issued to you or your organization, that you can export with its private key as a `.pfx` file. Since June 2023 the CAs issue new code signing keys only on hardware (a USB token or a cloud HSM), and such a key cannot be exported to a `.pfx`; the workflow's `.pfx` path then does not apply as is. In that case either use a CA that offers a cloud signing service with a `signtool`-compatible client and replace the `/f $pfx /p ...` arguments in the "Sign Windows executables and installer" step with that service's documented ones, or use Microsoft's Azure Trusted Signing service (an Azure subscription and identity validation; its `signtool` plugin fits the same step). SmartScreen builds reputation for a signer over time, so the first signed releases may still show a warning.
- **macOS: membership in the Apple Developer Program** (individual or organization, paid yearly), then:
  - a **Developer ID Application** certificate (not "Apple Development", not "Mac App Distribution"), created under Certificates, Identifiers and Profiles or in Xcode, Settings, Accounts, Manage Certificates, and exported from Keychain Access with its private key as a `.p12` file;
  - an **app-specific password** for the Apple ID that is a member of the team, from account.apple.com, Sign-In and Security, App-Specific Passwords;
  - the team's **Team ID**, from the Membership details page of the developer account.
- **Linux:** nothing. Linux archives are not signed; `SHA256SUMS.txt`, served from the release, is their integrity check, and `scripts/install.sh` enforces it.

What each group covers:

| Group | Signs |
|-------|-------|
| Windows signing | `Casebook.exe`, `bin\casebook.exe` and the installer, so also the portable zips and the `casebook-cli-<version>-windows-<arch>.zip` archives, which hold those same signed files. |
| macOS Developer ID signing | `Casebook.app`, the CLI inside it, and the DMG. The `casebook-cli-<version>-darwin-<arch>.tar.gz` archives hold slices of that signed CLI, so they are signed too. |
| macOS notarization | The app (with the nested CLI) and the DMG. The CLI slices are covered by the app's notarization, because Apple records every nested executable's code directory hash; a bare executable cannot carry a stapled ticket, so Gatekeeper confirms it online when it checks it. Files downloaded with `curl` or Homebrew are not quarantined, so Gatekeeper does not block the CLI archives either way. |

### The eight secrets

| Secret | What it is |
|--------|------------|
| `WINDOWS_CERT_PFX_BASE64` | The Windows code signing certificate with its private key, exported as a `.pfx` file, then base64 encoded. |
| `WINDOWS_CERT_PASSWORD` | The password of that `.pfx` file. |
| `APPLE_CERT_P12_BASE64` | The Developer ID Application certificate with its private key, exported from Keychain Access as a `.p12` file, then base64 encoded. |
| `APPLE_CERT_PASSWORD` | The password you set when exporting that `.p12` file. |
| `APPLE_SIGNING_IDENTITY` | The identity name of that certificate, in the form `Developer ID Application: <name> (<team id>)`. `security find-identity -v -p codesigning` on the Mac that holds the certificate lists it. |
| `APPLE_ID` | The Apple ID of the Apple Developer account used for notarization. |
| `APPLE_APP_PASSWORD` | An app-specific password for that Apple ID, created in the Apple ID account settings. Never use the account's real password. |
| `APPLE_TEAM_ID` | The ten-character Team ID of the Apple Developer account, shown in the account's membership details. |

They form three groups, and each group is used only when all of its secrets are set:

- **Windows signing:** `WINDOWS_CERT_PFX_BASE64` and `WINDOWS_CERT_PASSWORD`.
- **macOS Developer ID signing:** `APPLE_CERT_P12_BASE64`, `APPLE_CERT_PASSWORD` and `APPLE_SIGNING_IDENTITY`.
- **macOS notarization:** `APPLE_ID`, `APPLE_APP_PASSWORD` and `APPLE_TEAM_ID`. Notarization also needs the Developer ID group.

### Creating the base64 values

Encode the certificate file as one line of base64:

- Linux or WSL: `base64 -w0 cert.pfx > cert.pfx.b64`
- macOS: `base64 -i cert.p12 -o cert.p12.b64`
- Windows PowerShell: `[Convert]::ToBase64String([IO.File]::ReadAllBytes("C:\path\to\cert.pfx")) | Set-Content cert.pfx.b64`

Paste the contents of the `.b64` file as the secret value, then delete the `.b64` file. Keep the certificate files themselves out of the repository and out of any shared folder.

### Adding the secrets

Open the repository's Settings, then Secrets and variables, then Actions, and choose New repository secret for each one. With the GitHub CLI you can skip the intermediate file, for example `base64 -w0 cert.pfx | gh secret set WINDOWS_CERT_PFX_BASE64`; `gh secret set APPLE_APP_PASSWORD` with no input prompts for the value.

Adding certificates later needs only these secrets; the workflow does not change ([ADR 0010](decisions/0010-build-and-ci-environment.md)).

### What the workflow does with them

Each secret value appears only in the `env` block of the one step that needs it. The job environment carries only true or false flags (`HAS_WINDOWS_CERT`, `HAS_APPLE_CERT`, `HAS_NOTARY`), because the build steps run npm scripts and other third-party code that should never see a secret. GitHub masks secret values in logs. On the short-lived runner, the `.pfx` password reaches `signtool`, the `.p12` password reaches `security import`, and the Apple app-specific password reaches `notarytool`, all as command arguments.

**Windows**, in the "Sign Windows executables and installer" step:

1. Decodes the `.pfx` into the runner's temporary folder and finds the newest x64 `signtool.exe` under the Windows 10 SDK.
2. Signs `Casebook.exe` and `bin\casebook.exe` with SHA-256 and an RFC 3161 timestamp from `http://timestamp.digicert.com`.
3. Rebuilds the installer with `makensis` from the signed executables, so the installer contains the signed files, then signs the installer.
4. Runs `signtool verify /pa` on all three files.
5. Deletes the `.pfx` file, also when a step fails.

The uninstaller inside the installer is not signed.

**macOS:**

1. The "Import Developer ID certificate" step creates a temporary keychain in the runner's temporary folder, imports the `.p12`, and checks that the certificate provides the identity named in `APPLE_SIGNING_IDENTITY`.
2. `scripts/package-macos.sh` signs the nested CLI (`Contents/MacOS/bin/casebook`) first, with the Developer ID identity, the hardened runtime (`--options runtime`) and a secure timestamp. It then signs the app bundle the same way, adding the entitlements in `packaging/macos/entitlements.plist` (an empty set), so the bundle seal covers the already signed CLI. It verifies the bundle with `codesign --verify --deep --strict`.
3. With the notarization secrets, it zips the signed app (`ditto -c -k --keepParent`), submits the zip with `xcrun notarytool submit` using the Apple ID, the app-specific password and the team ID, and waits up to 30 minutes. Any result other than Accepted prints the notary log and fails the job. It then staples the ticket to `Casebook.app` itself (`xcrun stapler staple`), validates it, and checks the app with `spctl --assess --type execute`. The ticket travels with the app after it is dragged to Applications, so the first launch also works on a Mac that is offline or behind a firewall.
4. It builds the DMG from the stapled app and signs the DMG with the same identity.
5. With the notarization secrets, it notarizes the DMG the same way, staples the ticket to the DMG, validates it, and checks the DMG with `spctl --assess --type open`.
6. The "Remove keychain" step deletes the temporary keychain, also when a step fails.

### Without secrets

The release still succeeds. Each skipped group prints a notice on the run's summary page:

- `Windows signing skipped (WINDOWS_CERT_PFX_BASE64 and WINDOWS_CERT_PASSWORD are not both set); the executables and installer are unsigned`
- `Developer ID signing skipped (APPLE_CERT_P12_BASE64, APPLE_CERT_PASSWORD and APPLE_SIGNING_IDENTITY are not all set); the app gets an ad-hoc signature only`
- `Notarization skipped (needs the Developer ID certificate secrets plus APPLE_ID, APPLE_APP_PASSWORD and APPLE_TEAM_ID)`

Unsigned Windows files and an ad-hoc signed macOS app work, but the operating systems warn users before the first launch (see [Unsigned builds](#unsigned-builds)).

## Verifying downloads

Download `SHA256SUMS.txt` together with the files, into the same folder, and check them before running anything.

Linux or WSL (`--ignore-missing` skips the files you did not download):

```sh
sha256sum -c --ignore-missing SHA256SUMS.txt
```

macOS (lines for files you did not download report that they cannot be opened; only the files you have must show OK):

```sh
shasum -a 256 -c SHA256SUMS.txt
```

Windows PowerShell: compute the hash and compare it with the line for that file in `SHA256SUMS.txt` (PowerShell prints upper case and the file uses lower case; the comparison below ignores case):

```powershell
$file = "Casebook-v1.2.3-windows-amd64-installer.exe"
$hash = (Get-FileHash ".\$file" -Algorithm SHA256).Hash
Select-String -Path .\SHA256SUMS.txt -Pattern $file | ForEach-Object { $_.Line.Split(' ')[0] -eq $hash }
```

It prints `True` when the file matches.

To check signatures on a signed release:

- **Windows:** right-click `Casebook.exe` or the installer, choose Properties, and open the Digital Signatures tab. In PowerShell, `Get-AuthenticodeSignature .\Casebook.exe` must show the status `Valid`.
- **macOS:**

  ```sh
  codesign --verify --deep --strict --verbose=2 /Applications/Casebook.app
  spctl --assess --verbose /Applications/Casebook.app
  ```

  The first command checks the signature of the app and the CLI inside it. The second should report `accepted` with `source=Notarized Developer ID` for a notarized release.

## Unsigned builds

When a release was built without signing secrets:

- **Windows:** SmartScreen may show "Windows protected your PC" when the installer or `Casebook.exe` starts. Choose More info, then Run anyway.
- **macOS:** Gatekeeper blocks the first open of the app because it is not notarized. Open System Settings, go to Privacy and Security, and choose Open Anyway for Casebook.

Without a signature, the checksum is the only way to tell that a download is the file this workflow built. Check `SHA256SUMS.txt` (above) before choosing Run anyway or Open Anyway, and get it from the official Releases page at https://github.com/jameskbb/casebook/releases.

## Building release-like packages locally

The scripts accept `--release`, which makes a missing part fail exactly as it does in CI:

- Windows executables from WSL or Linux: `scripts/build-windows.sh --release --arch all` writes `dist/windows-amd64` and `dist/windows-arm64`, each with `Casebook.exe` and `bin/casebook.exe`. An installer needs a Windows machine with NSIS (`makensis`); [Development](development.md#windows-natively) lists the steps.
- macOS: on a Mac only, build the universal app with Wails, then run `scripts/package-macos.sh --release --app desktop/build/bin/Casebook.app`. To sign locally, set `CASEBOOK_SIGN_IDENTITY` (and `CASEBOOK_KEYCHAIN` when the identity is not in your default keychain); to notarize too, also set `CASEBOOK_NOTARIZE=1` and the `APPLE_ID`, `APPLE_APP_PASSWORD` and `APPLE_TEAM_ID` environment variables. `--dry-run` prints the full plan, with the password masked, on any OS.
- The CLI alone: `scripts/build-cli.sh --release --os <os> --arch <arch>`.
- A CLI archive from that build: `scripts/package-cli.sh --os <os> --arch <arch> --bin <path> --notices dist/THIRD_PARTY_NOTICES.txt` (add `--release` to require `LICENSE` and the stamp marker as CI does).
- The notices: `npm ci --ignore-scripts` in `desktop/frontend`, then `scripts/third-party-notices.sh` (writes `dist/THIRD_PARTY_NOTICES.txt`; `--release` turns a missing license file into an error).

Set `CASEBOOK_VERSION=v1.2.3` to stamp a specific version. Local packages are for testing; only the workflow's output is published.

## Troubleshooting

- **`LICENSE is missing at the repository root`** (the `notices` job or `scripts/package-cli.sh`): commit the license file; every release archive carries it.
- **`no license file found for ...`** (the `notices` job): a Go module or npm package that the executables bundle has no `LICENSE`, `COPYING` or `NOTICE` file in its root. Check the package's license, then add its text by hand to the notices logic in `scripts/third-party-notices.sh` (or replace the dependency); never ship without it.
- **`codesign --verify` fails in "Package the macOS CLI archives"**: a slice cut from the universal CLI lost its signature. Check that `scripts/package-macos.sh` still signs `Contents/MacOS/bin/casebook` before this step runs.
- **The Homebrew tap run says `HOMEBREW_TAP_TOKEN is not set`, or the commit step fails with 401 or 403**: the token is missing, expired or lacks Contents: Read and write on `jameskbb/homebrew-tap`. Renew it, then run the workflow again from the Actions tab with the tag.

- **The "Release docs check" step fails:** `scripts/check-docs.sh --release` found a `FINALIZE(phase 6)` marker or a missing track file; its `error:` lines name each one. Fix the docs on `main`, then delete and re-create the tag.
- **`release preflight failed`:** `scripts/preflight.sh --release` found a missing part. Its `missing: <path> (<track>)` lines name it. The release cannot be built until that part is on the tagged commit.
- **`tag ... is not a release tag`:** the tag does not match `vMAJOR.MINOR.PATCH` with an optional `-prerelease` suffix. Delete it (`git push origin :refs/tags/<tag>` and `git tag -d <tag>`) and push a correct one.
- **The "Check makensis" step fails:** NSIS did not install. The "Ensure NSIS" step installs it when the image lacks it, from Chocolatey or, when that feed is down (it answered 503 and 504 for the first v0.4.0 run), from the official zip with a pinned SHA-256; check that step's log. A release tagged before a workflow fix keeps running the old workflow: re-run the failed jobs once the feed is back, or tag the next patch version. Without `makensis`, Wails would only warn and produce no installer, so the workflow stops here instead.
- **`expected exactly one desktop/build/bin/*-installer.exe`:** the Wails NSIS step produced no installer; look for makensis errors in the build step, such as `CASEBOOK_CLI_BINARY does not name an existing casebook.exe`.
- **`does not contain the stamp marker`:** an executable was built without the stamp, or a `-X` path in `scripts/lib.sh` is wrong (the Go linker ignores a wrong path silently). The desktop app and the CLI must both import `internal/version` and be built with the `scripts/version.sh ldflags` string.
- **`hdiutil: create failed - Resource busy`:** `scripts/package-macos.sh` retries three times, five seconds apart. If all attempts fail, re-run the failed job.
- **`notarization of <file> was not Accepted`** (the app zip or the DMG): the job prints the notarization status and the full `notarytool log` just before this line; the log names each rejected file and the reason. Common causes are a wrong `APPLE_TEAM_ID`, an expired app-specific password, or a certificate that is not a Developer ID Application certificate.
- **`no x64 signtool.exe under ...`:** the runner image has no Windows 10 SDK signing tool where the step looks (`Windows Kits\10\bin`). Check the runner image's software list and adjust the search in the signing step.
- **`the imported certificate does not provide the identity named in APPLE_SIGNING_IDENTITY`:** the identity text does not match the certificate. Copy it exactly from `security find-identity -v -p codesigning`.
