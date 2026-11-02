#!/usr/bin/env bash
# Windows host helper for Casebook (D-27), run from WSL.
#
#   win-run.sh build                 build desktop/build/bin/Casebook.exe
#   win-run.sh reset <name>          delete the test data dir <name>
#   win-run.sh start <name> [args]   launch Casebook.exe --data-dir <name> [args]
#                                    (through WMI, so it cannot take the
#                                    foreground) and wait for its window,
#                                    which win-ui.ps1 parks off-screen
#   win-run.sh stop                  stop every Casebook.exe started from the
#                                    dev folder (ok when none runs)
#   win-run.sh ui <action> [args]    run win-ui.ps1 (waitwindow, find, capture)
#   win-run.sh datadir <name>        print the WSL path of the data dir <name>
#   win-run.sh aiwrite <name> [args] run desktop/tools/aiwrite (built as
#                                    aiwrite.exe) with --data-dir <name>: it
#                                    writes as an AI client from a separate
#                                    process, or takes a backup (--backup-to)
#   win-run.sh cli [args]            run the casebook CLI (built for Windows as
#                                    bin\casebook.exe in the dev folder, where
#                                    the app finds it as its bundled CLI)
#   win-run.sh devdir                print the Windows path of the dev folder
#
# With CASEBOOK_WIN_USERPROFILE set to a folder under the dev folder, start
# and cli run with that folder as the user profile, so client settings
# (.claude.json, .codex) are read and written there instead of the real
# ones (the Connect AI clients screenshot).
#
# With CASEBOOK_WIN_DATA_ROOT set to a folder at a drive root (for example
# C:\Casebook-demo), reset, start, datadir and aiwrite put the data dir
# <name> there instead of in the dev folder, so a capture that shows the
# database path names no Windows user (the Data menu screenshots).
#
# Everything else lives under %LOCALAPPDATA%\Temp\casebook-dev. Casebook is
# never launched without --data-dir, so the real per-user database is never
# touched.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
SCRIPTS="$ROOT/desktop/scripts"
PS=/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe

die() {
  echo "win-run: $*" >&2
  exit 1
}

[ -x "$PS" ] || die "powershell.exe not found at $PS"

# wincall <windows program> [args]: run a Windows program from a Windows
# directory (so it never starts in a UNC path), with CRs dropped. WSL
# interop sometimes fails to start the program at all ("UtilAcceptVsock:
# accept4 failed"); nothing ran then, so that one failure is retried.
wincall() {
  local err rc attempt
  err=$(mktemp)
  for attempt in $(seq 30); do
    rc=0
    (cd /mnt/c && "$@") 2>"$err" | tr -d '\r' || rc=$?
    if [ "$rc" -eq 0 ] || ! grep -q 'UtilAcceptVsock' "$err"; then
      cat "$err" >&2
      rm -f "$err"
      return "$rc"
    fi
    sleep 2
  done
  cat "$err" >&2
  rm -f "$err"
  return "$rc"
}

winps() {
  wincall "$PS" -NoProfile -NonInteractive "$@"
}

LOCALAPPDATA_WIN=$(winps -Command '$env:LOCALAPPDATA')
[ -n "$LOCALAPPDATA_WIN" ] || die "could not read %LOCALAPPDATA%"
DEV_WIN="$LOCALAPPDATA_WIN\\Temp\\casebook-dev"
DEV=$(wslpath -u "$DEV_WIN")
DATA_WIN=$DEV_WIN
if [ -n "${CASEBOOK_WIN_DATA_ROOT:-}" ]; then
  # One folder directly under a drive root, so reset can only remove a
  # data dir inside it.
  [[ "$CASEBOOK_WIN_DATA_ROOT" =~ ^[A-Za-z]:\\[A-Za-z0-9._-]+$ ]] ||
    die "CASEBOOK_WIN_DATA_ROOT must be one folder at a drive root, like C:\\Casebook-demo"
  DATA_WIN=$CASEBOOK_WIN_DATA_ROOT
fi
DATA=$(wslpath -u "$DATA_WIN")

check_name() {
  [[ "${1:-}" =~ ^[A-Za-z0-9._-]+$ ]] && [[ "$1" != "." ]] && [[ "$1" != ".." ]] ||
    die "bad data dir name '${1:-}' (letters, digits, dot, dash, underscore)"
}

# check_profile <windows path>: a user profile override must live in the dev
# folder.
check_profile() {
  case "$1" in
  "$DEV_WIN"\\*) ;;
  *) die "CASEBOOK_WIN_USERPROFILE must be under $DEV_WIN" ;;
  esac
}

# Quote one argument as a PowerShell single-quoted string.
psquote() {
  printf "'%s'" "${1//\'/\'\'}"
}

stage_ui() {
  mkdir -p "$DEV"
  cp "$SCRIPTS/win-ui.ps1" "$DEV/win-ui.ps1"
}

run_ui() {
  stage_ui
  winps -ExecutionPolicy Bypass -File "$DEV_WIN\\win-ui.ps1" "$@"
}

cmd=${1:-}
[ -n "$cmd" ] || die "usage: win-run.sh build|reset|start|stop|ui|datadir|devdir|aiwrite|cli ..."
shift

case "$cmd" in
build)
  bash "$SCRIPTS/build-windows.sh"
  ;;
reset)
  check_name "${1:-}"
  rm -rf -- "${DATA:?}/$1"
  echo "win-run: reset $DATA_WIN\\$1"
  ;;
start)
  check_name "${1:-}"
  name=$1
  shift
  exe="$ROOT/desktop/build/bin/Casebook.exe"
  [ -s "$exe" ] || die "missing $exe; run win-run.sh build first"
  mkdir -p "$DEV"
  # Windows can hold the old exe open for a moment after stop.
  for _ in 1 2 3 4 5 6 7 8 9 10; do
    cp "$exe" "$DEV/Casebook.exe" 2>/dev/null && break
    sleep 1
  done
  cmp -s "$exe" "$DEV/Casebook.exe" || die "could not copy Casebook.exe to $DEV_WIN"
  stage_ui
  mkdir -p "$DATA"
  # The command line, built in PowerShell with $q (a double quote, which
  # cannot pass through WSL interop) around each argument.
  cmdline="\$q+$(psquote "$DEV_WIN\\Casebook.exe")+\$q"
  for a in --data-dir "$DATA_WIN\\$name" "$@"; do
    cmdline="$cmdline+' '+\$q+$(psquote "$a")+\$q"
  done
  # The app's window steals the foreground when the process that starts it
  # may set the foreground, and a process started from this terminal may.
  # Start it through WMI instead, so it has no such right and the person
  # keeps the window they are using (waitwindow then parks it off-screen).
  # Win32_ProcessStartup replaces the whole environment, so pass this
  # one's (with the user profile override).
  env="\$e=@{}; Get-ChildItem env: | ForEach-Object { \$e[\$_.Name]=\$_.Value }; "
  if [ -n "${CASEBOOK_WIN_USERPROFILE:-}" ]; then
    check_profile "$CASEBOOK_WIN_USERPROFILE"
    env="$env\$e['USERPROFILE']=$(psquote "$CASEBOOK_WIN_USERPROFILE"); "
  fi
  winps -Command "\$q=[char]34; $env\$vars=[string[]](\$e.GetEnumerator() | ForEach-Object { \$_.Key + '=' + \$_.Value }); \$si=New-CimInstance -ClassName Win32_ProcessStartup -ClientOnly -Property @{ShowWindow=[uint16]4; EnvironmentVariables=\$vars}; \$r=Invoke-CimMethod -ClassName Win32_Process -MethodName Create -Arguments @{CommandLine=($cmdline); CurrentDirectory=$(psquote "$DEV_WIN"); ProcessStartupInformation=\$si}; if (\$r.ReturnValue -ne 0) { Write-Output ('win-run: Win32_Process.Create returned ' + \$r.ReturnValue); exit 1 }"
  run_ui -Action waitwindow
  ;;
stop)
  # Only processes started from the dev folder are stopped; a real Casebook
  # the user runs is never killed.
  run_ui -Action stop
  ;;
ui)
  action=${1:-}
  [ -n "$action" ] || die "usage: win-run.sh ui <action> [args]"
  shift
  pass=(-Action "$action")
  out_dest=""
  while [ $# -gt 0 ]; do
    if [ "$1" = "-Out" ] && [ $# -ge 2 ]; then
      # The capture lands in the dev dir first, then is copied to the
      # requested WSL path.
      out_dest=$2
      pass+=(-Out "$DEV_WIN\\capture.png")
      shift 2
    else
      pass+=("$1")
      shift
    fi
  done
  rc=0
  run_ui "${pass[@]}" || rc=$?
  if [ "$rc" -eq 0 ] && [ -n "$out_dest" ]; then
    mkdir -p "$(dirname "$out_dest")"
    cp "$DEV/capture.png" "$out_dest"
    echo "win-run: saved $out_dest"
  fi
  exit "$rc"
  ;;
datadir)
  check_name "${1:-}"
  echo "$DATA/$1"
  ;;
devdir)
  echo "$DEV_WIN"
  ;;
cli)
  mkdir -p "$ROOT/desktop/build/bin" "$DEV/bin"
  (cd "$ROOT" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
    go build -o desktop/build/bin/casebook.exe ./cmd/casebook)
  cp "$ROOT/desktop/build/bin/casebook.exe" "$DEV/bin/casebook.exe"
  if [ -n "${CASEBOOK_WIN_USERPROFILE:-}" ]; then
    check_profile "$CASEBOOK_WIN_USERPROFILE"
    export USERPROFILE="$CASEBOOK_WIN_USERPROFILE"
    export WSLENV="${WSLENV:+$WSLENV:}USERPROFILE"
  fi
  wincall "$DEV/bin/casebook.exe" "$@"
  ;;
aiwrite)
  check_name "${1:-}"
  name=$1
  shift
  mkdir -p "$ROOT/desktop/build/bin" "$DEV"
  (cd "$ROOT" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
    go build -o desktop/build/bin/aiwrite.exe ./desktop/tools/aiwrite)
  cp "$ROOT/desktop/build/bin/aiwrite.exe" "$DEV/aiwrite.exe"
  wincall "$DEV/aiwrite.exe" --data-dir "$DATA_WIN\\$name" "$@"
  ;;
*)
  die "unknown command '$cmd'"
  ;;
esac
