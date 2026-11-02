Casebook @VERSION@ for Windows (portable)
==========================================

Casebook gives AI investigations durable memory. This folder runs as is:
nothing needs installing. Keep the whole folder together.

What is in this folder
----------------------

  Casebook.exe          the desktop app (double-click to start)
  bin\casebook.exe      the command line tool; AI clients also start it
                        as their MCP server ("casebook mcp")
  README.txt            this file

Your data is not in this folder
-------------------------------

Casebook stores its data per Windows user, in

  %LOCALAPPDATA%\Casebook\casebook.db

(usually C:\Users\<you>\AppData\Local\Casebook). Replacing or deleting
this folder never touches it. The CASEBOOK_HOME and CASEBOOK_DB
environment variables move it elsewhere, if you set them.

Connect an AI client
--------------------

Open a terminal in this folder and run

  bin\casebook.exe setup claude
  bin\casebook.exe setup codex

The clients remember the full path of bin\casebook.exe, so keep this
folder in a stable place (for example C:\Tools\Casebook), not in
Downloads.

Check your version
------------------

  Desktop app:  the Data menu (bottom of the sidebar) shows the version.
  Terminal:     bin\casebook.exe --version

Update to a new release
-----------------------

Casebook never checks for updates on its own. Choose "Check for updates..."
in the Data menu, or run bin\casebook.exe update, to ask GitHub whether a
newer release exists; nothing is downloaded. To update:

  1. Download the new Casebook-<version>-windows-<arch>-portable.zip and
     check it against SHA256SUMS.txt from the same release:
       Get-FileHash .\Casebook-<version>-windows-amd64-portable.zip -Algorithm SHA256
  2. Close Casebook and every AI client that uses it (Claude Code, Codex
     and so on). They run bin\casebook.exe, and Windows will not replace
     a file that is running.
  3. Replace the contents of this folder with the contents of the new zip.
     Replace the whole folder, never only one of the two programs.
  4. Run  bin\casebook.exe doctor  and check that every line reads ok.
     The bundle_version line says whether the desktop app and the
     command line tool are the same version.

If you unzip a new release into a different folder instead, run the setup
commands above again from the new folder; doctor flags clients that still
point at the old one.

Backups
-------

When a new release changes the data format, the first time it opens your
data it copies the data, before upgrading it, to

  %LOCALAPPDATA%\Casebook\backups\casebook-pre-upgrade-schema<N>-<time>.db

Restores also leave a casebook-pre-restore copy there. To go back to an
older release after an upgrade, start the older Casebook: it says the
data needs a newer Casebook and offers Restore from backup. Choose the
casebook-pre-upgrade copy; the newer data file is moved into the backups
folder, not deleted. Changes made since the upgrade stay only in that
newer file.

In the desktop app, Data > Back up all data... writes a backup wherever
you choose; do that before any update you are unsure about.
