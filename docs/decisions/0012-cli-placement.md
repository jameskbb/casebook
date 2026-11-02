# 0012: CLI placement in a bin directory next to the desktop app

- Status: Accepted
- Date: 2026-10-02

Decided during Phase 5 planning (packaging, CI and docs) so that every track codes against one layout. Maintainer review is requested.

## Context

ADR-0002 and requirements PKG-02 and PKG-03 ship the `casebook` CLI next to the `Casebook` desktop executable: "side by side" on Windows and "inside Contents/MacOS" on macOS.

The default file systems on both platforms (NTFS on Windows, APFS on macOS) are case-insensitive. `Casebook.exe` and `casebook.exe` are one path on Windows, and `Casebook.app/Contents/MacOS/Casebook` and `Casebook.app/Contents/MacOS/casebook` are one path on macOS. Putting both executables in the same directory therefore fails in every delivery channel:

- Unzipping the release archive, running the installer, or copying the files overwrites one executable with the other, silently leaving a single binary.
- On Windows, a PATH entry for the install folder would make the `casebook` command start the GUI (or the desktop shortcut start the CLI), depending on which file won.

## Decision

The CLI lives in a `bin` directory next to the desktop executable.

- Windows (zip and NSIS install folder alike): `<dir>\Casebook.exe` and `<dir>\bin\casebook.exe`. The installer may add `<install dir>\bin` to PATH; PATH gets only the `bin` folder, never the folder holding `Casebook.exe`.
- macOS: `Casebook.app/Contents/MacOS/Casebook` and `Casebook.app/Contents/MacOS/bin/casebook`. The CLI stays inside `Contents/MacOS` and is signed as nested code before the app bundle.

Lookup rules for code:

- From the desktop executable, the CLI is `dir(desktop executable)/bin/casebook` (plus `.exe` on Windows).
- From the CLI, the desktop executable is `dir(dir(CLI))/Casebook` (plus `.exe` on Windows); on macOS that is the `Contents/MacOS/Casebook` of the enclosing app bundle.

## Consequences

- Track A's "Connect AI clients" panel (INT-10) shows and copies the CLI path using the first lookup rule, and track B's `casebook ui` (CLI-10) launches the desktop app using the second.
- AI client configurations store the absolute CLI path (ADR-0002), so moving the CLI after a public release breaks existing user configurations. This layout should be treated as stable.
- The names live once in `scripts/lib.sh` (`CB_GUI_NAME=Casebook`, `CB_CLI_NAME=casebook`, `CB_CLI_SUBDIR=bin`) and are mirrored in the release workflow and the NSIS script.
- `scripts/build-windows.sh` and the release jobs fail when the two executables would be one file (same path, a path differing only by case, or identical contents).
- ADR-0002 is amended by this record; its file is not edited. The requirement wording "side by side" and "inside Contents/MacOS" is met by the `bin` directory next to the desktop executable.
