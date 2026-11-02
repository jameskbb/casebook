# Updating Casebook

To update Casebook, replace the program files with a newer release or rerun the installer you used. Your data stays in its separate data folder. Casebook can [check for a newer version](#checking-for-a-newer-version) when you ask; it does not download or install updates.

## Versions

Every release has one version, such as `v1.2.3`, and both programs in a release (the desktop app and the `casebook` command line tool) carry the same one. A version with a hyphen, such as `v1.2.3-rc.1`, is a prerelease.

To see which version you have:

- **Desktop app:** open the **Data** menu at the bottom of the sidebar. Its last item, **Version**, shows the version; choosing it copies `Casebook <version>` for a bug report.
- **Command line:** `casebook --version` (or `casebook version`) prints `casebook <version>`. On Windows, from the Casebook folder, run `bin\casebook.exe --version`.
- **Doctor:** `casebook doctor` prints the version in its first line. Its `bundle_version` line checks that the desktop app next to the CLI is the same version: `ok` when they match, a warning such as `the desktop app is v1.2.0, this CLI is v1.3.0: replace the whole folder when you update` when only part of a folder was replaced, and `skip` for builds from source or a CLI with no desktop app next to it.

## Checking for a newer version

Casebook can ask GitHub whether a newer release exists. It does this only when you ask, never at startup or on a timer, and it never downloads or installs anything. The check is one request to GitHub's public releases API; it sends no version, no identifier and nothing from your cases, and GitHub sees your IP address as for any web page.

- **Desktop app:** open the **Data** menu at the bottom of the sidebar and choose **Check for updates...**. A small dialog explains what will be sent; nothing is sent until you choose **Check now**. The result shows the latest version and whether it is newer than yours, and offers **Open release page** and **Copy download link**.
- **Command line:** run `casebook update`. When a newer release exists it prints, for example:

  ```text
  A newer Casebook is available: v1.3.0 (you have v1.2.3).
  Release page: https://github.com/jameskbb/casebook/releases/tag/v1.3.0
  Download:     https://github.com/jameskbb/casebook/releases/download/v1.3.0/casebook-cli-v1.3.0-linux-amd64.tar.gz
  Updating Casebook: https://github.com/jameskbb/casebook/blob/main/docs/updating.md
  ```

  The `Download:` line names the command line tool for your system and is left out when the release has none. Otherwise it prints `Casebook v1.2.3 is the latest release.`, or says that your copy is newer than the latest release (a prerelease or a build from source) or is not a release version at all. `casebook update --json` prints the same result as JSON: `current`, `latest`, `state` (`update_available`, `up_to_date`, `ahead` or `unknown`), `release_url`, `published_at`, `asset` (the download's `name`, `url` and `size_bytes`, when there is one) and `checked_at`.

If the check fails, Casebook says why in one sentence and changes nothing; `casebook update` exits 1. `could not reach GitHub` means there is no network connection or GitHub did not answer in time. `GitHub lists no release for jameskbb/casebook` means GitHub answered but showed no published release: the repository may be private or have no release yet. You can still look at the [Releases page](https://github.com/jameskbb/casebook/releases) yourself.

The check only tells you a release exists. Downloading it, checking it and replacing your copy are the steps in [Getting a release](#getting-a-release) and the sections after it.

## Getting a release

Releases are published on the [Releases page](https://github.com/jameskbb/casebook/releases) of `jameskbb/casebook`. No account is needed:

- **In a browser:** open the Releases page and download the file for your system from the release's Assets list, together with `SHA256SUMS.txt`.
- **With the GitHub CLI:**

  ```sh
  gh release download v1.2.3 --repo jameskbb/casebook --pattern '*windows-amd64-portable.zip'
  gh release download v1.2.3 --repo jameskbb/casebook --pattern SHA256SUMS.txt
  ```

  Use `--pattern '*windows-arm64-portable.zip'` for Windows on Arm, `--pattern '*macos-universal.dmg'` for a Mac, or `--pattern '*windows-amd64-installer.exe'` for the Windows installer.

Check the download against `SHA256SUMS.txt` before you run anything:

- Windows PowerShell: `Get-FileHash .\Casebook-v1.2.3-windows-amd64-portable.zip -Algorithm SHA256`, then compare the hash with the file's line in `SHA256SUMS.txt` (PowerShell prints it in upper case; the comparison ignores case).
- Linux or WSL: `sha256sum --ignore-missing -c SHA256SUMS.txt` in the folder that holds both files.
- macOS: `shasum -a 256 Casebook-v1.2.3-macos-universal.dmg`, then compare with the file's line in `SHA256SUMS.txt`.

| Your system | Download |
|---|---|
| Windows (most PCs, 64-bit Intel or AMD) | `Casebook-<version>-windows-amd64-portable.zip` (portable, recommended) or `Casebook-<version>-windows-amd64-installer.exe` |
| Windows on Arm | `Casebook-<version>-windows-arm64-portable.zip` or `Casebook-<version>-windows-arm64-installer.exe` |
| macOS (Apple Silicon and Intel) | `Casebook-<version>-macos-universal.dmg` |
| Linux | the command line tool only; see [Command line only](#command-line-only-linux-macos-windows) |

[Which file do I need?](install.md#which-file-do-i-need) describes every file in a release. Releases before 0.3.0 named the portable zip without `-portable`. Each desktop package includes the command line tool. If you only want the command line tool (for example on a server, or because you only use Casebook through AI clients), see [Command line only](#command-line-only-linux-macos-windows).

## Command line only (Linux, macOS, Windows)

The `casebook` command line tool is a single file. It holds everything an AI client needs (it serves MCP with `casebook mcp`) and uses the same data folder as the desktop app. On Linux it is the whole product: there is no Linux desktop app, so AI clients work through MCP and you read cases with `casebook context`, `casebook search` and `casebook case`. Marking a record as verified by a person happens in the desktop app today; `casebook export` or a backup carries a case to a Mac or Windows machine for that.

Install it one of three ways. None needs administrator rights.

- **Install script (Linux and macOS):**

  ```sh
  curl -fsSL https://raw.githubusercontent.com/jameskbb/casebook/main/scripts/install.sh | sh
  ```

  It downloads the latest release's `casebook-cli-<version>-<os>-<arch>.tar.gz`, checks it against `SHA256SUMS.txt` (and installs nothing on a mismatch), and puts `casebook` in `~/.local/bin`. It never uses `sudo` and never edits your shell profile; if `~/.local/bin` is not on your `PATH`, it prints the line to add. To choose the folder or the version, set the variables for `sh`, not `curl`: `curl -fsSL <url> | CASEBOOK_INSTALL_DIR=~/bin CASEBOOK_VERSION=v1.2.3 sh`. To read the script first, download it, read it, then run `sh install.sh`.
- **Homebrew (macOS and Linux):**

  ```sh
  brew install jameskbb/tap/casebook
  ```

- **Go (any platform with Go installed):**

  ```sh
  go install github.com/jameskbb/casebook/cmd/casebook@latest
  ```

  This builds from source into `$(go env GOPATH)/bin` (usually `~/go/bin`). No C compiler is needed.
- **Windows, by hand:** download `casebook-cli-<version>-windows-amd64.zip` (or `-arm64`), check it, and extract `casebook.exe` into a folder you keep, such as `%LocalAppData%\Programs\casebook`. Add that folder to your user `PATH` if you want to type `casebook` anywhere.

Then connect your AI client:

```sh
casebook setup claude      # Claude Code
casebook setup codex       # Codex
casebook doctor            # checks the install, the data and the client setup
```

### Updating the command line tool

Quitting AI clients first is not required on Linux and macOS: the new file replaces the old one, and running sessions keep the old copy until they restart. Restart them to use the new version. On Windows, quit every AI client that uses Casebook first, as for the portable folder.

| Installed with | Update with |
|---|---|
| The install script | Run the same command again. It replaces `~/.local/bin/casebook` (or your `CASEBOOK_INSTALL_DIR`) in place. |
| Homebrew | `brew upgrade casebook` |
| Go | `go install github.com/jameskbb/casebook/cmd/casebook@latest` again |
| The Windows zip | Replace `casebook.exe` with the one from the new zip. |

Each of these keeps the path that AI clients run, so client setup keeps working. (With Homebrew, `casebook setup` records Homebrew's stable `opt/casebook/bin/casebook` link, which `brew upgrade` repoints to the new version.) Run `casebook doctor` afterwards.

To uninstall, delete the file (`rm ~/.local/bin/casebook`, `brew uninstall casebook`, or delete `casebook` from `$(go env GOPATH)/bin`). Your data stays in the data folder until you delete it ([Privacy and local data](privacy.md)).

## Windows portable folder (the zip)

The zip is the main way to run Casebook on Windows. Unzip it to a folder you keep, such as `C:\Tools\Casebook`, rather than your Downloads folder: AI clients you connect with `casebook setup claude` or `casebook setup codex` remember the full path of `bin\casebook.exe` in that folder. The folder holds:

- `Casebook.exe`: the desktop app.
- `bin\casebook.exe`: the command line tool, which AI clients also start as their MCP server (`casebook mcp`).
- `README.txt`: a short version of this guide, stamped with the release version.

Nothing in the folder is your data, so you can replace the whole folder safely.

### Updating the folder in place

1. Download the new zip and check it (see [Getting a release](#getting-a-release)).
2. Quit the Casebook desktop app, and quit every AI client that uses Casebook (Claude Code, Codex and so on). Each client session runs `bin\casebook.exe mcp` while it is open, and Windows refuses to replace a program file that is running ("the action can't be completed because the file is open"). If a file still cannot be replaced, Task Manager shows which `casebook.exe` is still running.
3. Replace the folder's contents with the contents of the new zip: delete the old `Casebook.exe`, `bin` and `README.txt`, then extract the new zip into the same folder. Always replace both programs together.
4. Run `bin\casebook.exe doctor` from the folder. Every line should read `ok` (or `skip` for clients you do not use), and `bundle_version` should say both programs are the new version.

Because the path of `bin\casebook.exe` did not change, your AI client setup keeps working with no further steps.

### Moving to a new folder instead

If you extract the new release into a different folder, run setup again from the new folder so the clients point at it:

```text
C:\Tools\Casebook-new> bin\casebook.exe setup claude
C:\Tools\Casebook-new> bin\casebook.exe setup codex
```

Until you do, `casebook doctor` from the new folder reports `claude_config` and `codex_config` as `casebook runs <old path>, not this CLI` (or as a failure once the old folder is deleted).

## Windows installer

The installer remains available for people who prefer a Start menu entry and an uninstaller. To update an installed copy, quit Casebook and every AI client that uses it, run the new installer and keep the same install folder (by default under Program Files). It replaces `Casebook.exe` and `bin\casebook.exe` in place, so client setup keeps pointing at the same `bin\casebook.exe`. Run `casebook doctor` afterwards. Uninstalling never removes your data.

## macOS

Quit Casebook and every AI client that uses it, open the new DMG and drag Casebook to Applications, replacing the old copy. The CLI lives inside the app at `Casebook.app/Contents/MacOS/bin/casebook`, so replacing the app replaces both programs together and client setup keeps working. Run `/Applications/Casebook.app/Contents/MacOS/bin/casebook doctor` afterwards.

## What happens to your data

Your data stays in the per-user data folder (Windows `%LocalAppData%\Casebook`, macOS `~/Library/Application Support/Casebook`, Linux `~/.local/share/casebook`; [Privacy and local data](privacy.md#where-it-lives) has the details), never in the program folder. Updating the programs does not touch it until a new version opens it.

- **Automatic pre-upgrade backup.** When a new release changes the database format (its schema version), the first Casebook program that opens your data (the desktop app, the CLI, an AI client's MCP server or `casebook doctor`) first copies the whole database to `<data folder>/backups/casebook-pre-upgrade-schema<old version>-<UTC timestamp>.db`, then upgrades it. A release that does not change the format makes no copy. If the copy cannot be written (a full disk, a read-only folder), Casebook refuses to upgrade, says so, and leaves your data exactly as it was.
- **Forward only.** Upgrades only go forward. Once a newer Casebook has upgraded your data, an older Casebook cannot open it: it says the data was saved by a newer version of Casebook, names the two schema versions, and changes nothing. Restores also leave a `casebook-pre-restore-<UTC timestamp>.db` copy in the same `backups` folder.
- **Your own backups.** Before an update you are unsure about, make a backup with **Back up all data...** in the desktop app's **Data** menu, or `casebook backup <file>`.

### Going back to an older version

Changes you made after the upgrade exist only in the upgraded data, so going back loses them unless you redo them.

1. Find the copy to restore. Open the `backups` folder in the data folder and look for `casebook-pre-upgrade-schema<N>-<timestamp>.db` files. The older Casebook can restore a copy whose `<N>` is at most the schema version it supports, which its error message names (`this Casebook supports up to N`). Pick the newest such file.
2. With the older Casebook:
   - **Desktop app:** start it. The startup screen says the data needs a newer Casebook and offers **Restore from backup...**. Choose the pre-upgrade copy. Casebook moves the newer data file into the `backups` folder as `casebook-unopened-<timestamp>.db` (nothing is deleted) and opens the restored data in its place.
   - **Command line only:** quit every Casebook program, move `casebook.db` and any `casebook.db-wal` and `casebook.db-shm` files out of the data folder together (into a folder of their own), then run `casebook restore <path to the pre-upgrade copy> --yes`. The restore starts from an empty database and fills it from the copy.
3. Run `casebook doctor` to confirm.

To return to the newer version later, update again: it upgrades the restored data (taking a new pre-upgrade copy first).

## Updating checklist

1. Back up (optional but cheap): **Data**, **Back up all data...**.
2. Download the release and check it against `SHA256SUMS.txt`.
3. Quit Casebook and every AI client that uses it.
4. Replace the whole program folder (or run the installer, or drag the app).
5. Run `casebook doctor`.
