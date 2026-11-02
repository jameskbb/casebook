# Installing Casebook

Install the desktop app to read, review and edit your cases in a window. It includes the command line tool your AI client uses to work with Casebook.

For a terminal-only setup, install just the command line tool. Claude Code, Codex and other MCP clients can use it without the desktop app. Both programs open the same cases on your computer.

## Command line tool

Pick one:

```sh
# macOS or Linux, install script
curl -fsSL https://raw.githubusercontent.com/jameskbb/casebook/main/scripts/install.sh | sh

# Any platform with Go 1.27 or later
go install github.com/jameskbb/casebook/cmd/casebook@latest
```

The install script downloads a release binary; `go install` builds the latest tagged source. To build a local checkout instead, see [Development](development.md#the-cli-for-any-os).

Then check it:

```sh
casebook --version
casebook doctor
```

`casebook doctor` prints where the database lives, whether it opens, and whether Claude Code and Codex are connected. The command line tool builds and runs on Windows, macOS and Linux.

The install script puts `casebook` in `~/.local/bin` after checking the download against the release's `SHA256SUMS.txt`; it never uses `sudo`. On Windows, download `casebook-cli-<version>-windows-amd64.zip` (or `-arm64`) from the Releases page, or install the desktop app, which includes the command line tool. [Updating Casebook](updating.md#command-line-only-linux-macos-windows) has the details for each method.

On Linux the command line tool is the whole product: AI clients work through `casebook mcp`, and you read cases with `casebook context`, `casebook report` and `casebook search`. Individual evidence, claim and hypothesis review, and choosing which findings go into the leadership brief, need the desktop app today ([ADR 0014](decisions/0014-only-people-verify.md)); export the case or a backup to a Mac or Windows machine for those. The CLI supports explicit conclusion review.

## Desktop app

Download the file for your system from the [Releases page](https://github.com/jameskbb/casebook/releases), together with `SHA256SUMS.txt`, and [check the download](#check-your-download) before you run it. Each release lists its files under [Which file do I need?](#which-file-do-i-need) at the top of its notes.

There is no Linux desktop package yet; use the command line tool.

### Which file do I need?

`<version>` is the release, such as `v0.3.0`. Files ending in `-portable.zip` run where you unzip them; files ending in `-installer.exe` install with a Start menu entry and an uninstaller; the `.dmg` is the Mac app; `casebook-cli-*` archives are the command line tool alone.

Desktop app:

| File | What it is |
|---|---|
| `Casebook-<version>-windows-amd64-portable.zip` | Windows, most PCs (64-bit Intel or AMD). Portable (recommended). Not sure? Take this one. Unzip and run `Casebook.exe`; nothing to install, and your data lives in `%LocalAppData%\Casebook` either way. Includes the command line tool at `bin\casebook.exe`. |
| `Casebook-<version>-windows-arm64-portable.zip` | The same portable zip for Windows on Arm. |
| `Casebook-<version>-windows-amd64-installer.exe` | Windows, most PCs (64-bit Intel or AMD). Installer: Start menu entry, uninstaller, optional PATH entry. The same app and command line tool. |
| `Casebook-<version>-windows-arm64-installer.exe` | The same installer for Windows on Arm. |
| `Casebook-<version>-macos-universal.dmg` | macOS 11 or later, Apple Silicon and Intel. Drag to Applications. The command line tool is inside the app. |

Command line tool only (also the MCP server):

| File | What it is |
|---|---|
| `casebook-cli-<version>-linux-amd64.tar.gz` | Linux, 64-bit Intel or AMD. `scripts/install.sh` downloads this for you. |
| `casebook-cli-<version>-linux-arm64.tar.gz` | The same for Linux on 64-bit Arm. |
| `casebook-cli-<version>-darwin-arm64.tar.gz` | macOS, Apple Silicon. `scripts/install.sh` downloads this for you. |
| `casebook-cli-<version>-darwin-amd64.tar.gz` | The same for Intel Macs. |
| `casebook-cli-<version>-windows-amd64.zip` | Windows, 64-bit Intel or AMD: `casebook.exe` alone, no desktop app. |
| `casebook-cli-<version>-windows-arm64.zip` | Windows on Arm: `casebook.exe` alone, no desktop app. |

`SHA256SUMS.txt` holds the checksums ([check your download](#check-your-download)), and `THIRD_PARTY_NOTICES.txt` the license texts of everything Casebook bundles; the same file is inside every download. Releases before 0.3.0 named the portable zip without `-portable`.

### Windows: portable folder or installer

- **Portable (recommended):** `Casebook-<version>-windows-amd64-portable.zip` (or `-arm64-portable.zip`). Unzip it into a folder you keep, such as `C:\Tools\Casebook`, and run `Casebook.exe`. The folder also holds the command line tool (`bin\casebook.exe`) and a `README.txt`. Updating means replacing the folder's contents with a newer zip.
- **Installer:** `Casebook-<version>-windows-amd64-installer.exe` (or `-arm64-installer.exe`) adds a Start menu entry and an uninstaller, installs the same app and command line tool under `C:\Program Files\Casebook`, and offers an option, unchecked by default, to add the `bin` folder to your PATH.
- Either way, your data is stored per user in `%LocalAppData%\Casebook`, not in the program folder, so updating or uninstalling never touches it.
- Casebook needs the Microsoft WebView2 runtime. It is built into Windows 11. When it is missing, the installer can download it from Microsoft.
- **SmartScreen:** release builds are not code signed yet, so Windows may show "Windows protected your PC". Choose More info, then Run anyway, only after you have checked the download.

### macOS

- Open the DMG and drag Casebook to Applications. The command line tool is inside the app at `/Applications/Casebook.app/Contents/MacOS/bin/casebook`; link it onto your PATH if you want to type `casebook`, for example `ln -s /Applications/Casebook.app/Contents/MacOS/bin/casebook /usr/local/bin/casebook`.
- **Gatekeeper:** when a build is not signed and notarized, macOS blocks the first open. Open System Settings, go to Privacy and Security, and choose Open Anyway for Casebook. You only need to do this once.

## Which version am I running?

The last item in the desktop app's **Data** menu shows the version; `casebook --version` prints it; and `casebook doctor` warns when the desktop app and the command line tool in the same folder are different versions. Casebook never checks for updates on its own: it asks GitHub for the latest release only when you choose **Check for updates...** in the **Data** menu or run `casebook update`, and it never downloads one. [Updating Casebook](updating.md) covers updates and going back to a pre-upgrade backup.

## Check your download

Each release includes `SHA256SUMS.txt` with a checksum for every file. Compare it with the checksum of your download before you run anything:

- Windows PowerShell: `Get-FileHash .\<file> -Algorithm SHA256`, then compare with the file's line in `SHA256SUMS.txt` (the comparison ignores case).
- Linux or WSL: `sha256sum --ignore-missing -c SHA256SUMS.txt` in the folder that holds both files.
- macOS: `shasum -a 256 <file>`, then compare with the file's line in `SHA256SUMS.txt`.

[Building releases](building-releases.md#verifying-downloads) explains how releases are built and checked.

## Next

Connect your AI clients: [Claude Code](claude-code.md), [Codex](codex.md), or [any MCP client](mcp.md).
