#!/usr/bin/env bash
# Windows host end-to-end smoke for the Casebook desktop app, run
# from WSL after desktop/scripts/win-run.sh build.
#
#   win-smoke.sh            run every step (A to E required, F best effort)
#   win-smoke.sh A B C D    run only the named steps (C and D need B first)
#
# Steps:
#   A  fresh launch with no configuration creates the database and shows
#      "Check what your AI tells you"
#   B  the seeded demo opens on Home; the featured question opens from
#      there and drills down a finding, then its source, with Back; best
#      effort: Check the next source opens the one-source-at-a-time stepper
#   C  a separate process (aiwrite.exe as ai claude-code) writes while the
#      app is open; the app shows it within 5 seconds
#   D  Copy to paste into your AI puts the pack on the real clipboard
#   E  close and relaunch on the same data dir keeps everything
#   F  best effort: the core workflow driven through the UI
#
# Prints PASS or FAIL per check and exits non-zero when a required check
# fails. Every data dir lives under %LOCALAPPDATA%\Temp\casebook-dev.
set -uo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
W="$ROOT/desktop/scripts/win-run.sh"
LOG="$ROOT/desktop/build/bin/win-smoke.log"
mkdir -p "$ROOT/desktop/build/bin"
: >"$LOG"

failed=0
best_failed=0

# check <required|best> <label> <command...>: run a command quietly and
# print PASS or FAIL for it.
check() {
  local kind=$1 label=$2
  shift 2
  if "$@" >>"$LOG" 2>&1; then
    echo "PASS $label"
  else
    echo "FAIL $label"
    if [ "$kind" = required ]; then failed=1; else best_failed=1; fi
  fi
}

ui() { "$W" ui "$@"; }
find_text() { ui find -Text "$1" -TimeoutSeconds "${2:-20}"; }

steps=("$@")
[ ${#steps[@]} -gt 0 ] || steps=(A B C D E F)
want() {
  local s
  for s in "${steps[@]}"; do [ "$s" = "$1" ] && return 0; done
  return 1
}

"$W" stop >>"$LOG" 2>&1
# win-ui.ps1 logs here when the cursor moved or a Casebook window took the
# foreground during an action; neither should happen.
WARN="$(wslpath -u "$("$W" devdir)")/win-ui-warnings.log"
rm -f "$WARN"

FEATURED='Why did calls go up 30% in October when sales stayed the same?'
AI_SOURCE='Refunds: 1,650 of the 1,900 customers were paid back by October 10'

if want A; then
  echo "== A: fresh launch"
  "$W" reset fresh >>"$LOG" 2>&1
  check required "A launch on a fresh data dir" "$W" start fresh
  check required "A shows Check what your AI tells you" find_text "Check what your AI tells you"
  check required "A created casebook.db" test -s "$("$W" datadir fresh)/casebook.db"
  "$W" stop >>"$LOG" 2>&1
fi

if want B; then
  echo "== B: Home, the question, a finding and its source"
  "$W" reset demo >>"$LOG" 2>&1
  check required "B launch the seeded demo" "$W" start demo --seed-demo
  # The app opens on Home, which lists the questions with sources to check.
  check required "B Home lists the featured question" find_text "$FEATURED"
  check required "B open the featured question from Home" ui invoke -Text "Check: $FEATURED"
  check required "B shows The answer" find_text "The answer"
  check required "B shows What backs this up" find_text "What backs this up"
  check required "B open finding C-003" ui invoke -Text "Open finding C-003: A billing mistake on October 3 charged some customers twice."
  check required "B finding drawer shows Sources that back it up" find_text "Sources that back it up"
  # The drawer's source block comes after the page's row of the same name,
  # so take the last match.
  check required "B open source E-007 from the finding drawer" ui invoke -Text "Open source E-007: Notice dated October 4: 1,900 customers were charged twice on October 3" -Last
  check required "B source drawer shows Back to C-003" find_text "Back to C-003"
  check required "B Esc closes the drawers" ui keys -Text "{ESC}"
  # One checking flow: a source at a time with I checked this and Skip.
  check best "B open Check the next source" ui invoke -Text "Check the next source"
  check best "B the stepper offers Skip" find_text "Skip" 5
  check best "B Esc closes the stepper" ui keys -Text "{ESC}"
fi

if want C; then
  echo "== C: live refresh from another process"
  out=$("$W" aiwrite demo 2>&1)
  echo "$out" >>"$LOG"
  echo "     aiwrite: $out"
  check required "C new source name within 5 s" find_text "$AI_SOURCE" 5
  check required "C How we got here says Claude Code added the source" find_text "Claude Code added the source" 5
fi

if want D; then
  echo "== D: Copy to paste into your AI, on the real clipboard"
  check required "D open More" ui invoke -Text "More"
  check required "D invoke Copy to paste into your AI" ui invoke -Text "Copy to paste into your AI"
  clip=$("$W" ui clipboard 2>>"$LOG")
  first=$(printf '%s\n' "$clip" | head -n 1)
  echo "     clipboard starts: ${first:0:60}..."
  # The pack is framed for an AI reader: a line saying it is stored data,
  # then BEGIN CASEBOOK DATA, the pack, END CASEBOOK DATA.
  check required "D clipboard starts with the data framing line" test "${first#The block between the BEGIN and END CASEBOOK DATA }" != "$first"
  check required "D clipboard holds the INV-0002 pack" grep -qx "Question: INV-0002" <<<"$clip"
  check required "D the pack says Answered, not an engine status" grep -qx "Status: Answered" <<<"$clip"
fi

if want E; then
  echo "== E: restart persistence"
  "$W" stop >>"$LOG" 2>&1
  check required "E relaunch without the seed flag" "$W" start demo
  check required "E the question is still there" find_text "$FEATURED"
  check required "E open the featured question from Home" ui invoke -Text "Check: $FEATURED"
  check required "E the other process's source is still there" find_text "$AI_SOURCE"
  "$W" stop >>"$LOG" 2>&1
fi

if want F; then
  echo "== F: the core workflow through the UI (best effort)"
  "$W" reset typed >>"$LOG" 2>&1
  check best "F launch a fresh data dir" "$W" start typed
  # 1. Add a question.
  check best "F1 open New question" ui invoke -Text "New question"
  check best "F1 type the question" ui type -Text "Question" -Value "Why are help desk tickets taking longer to close?"
  check best "F1 type the background" ui type -Text "Background (optional)" -Value "Tickets took 2 days to close in September and 5 days in October."
  check best "F1 Add question" ui invoke -Text "Add question"
  check best "F1 the new question opens" find_text "Why are help desk tickets taking longer to close?" 5
  check best "F1 the background shows" find_text "Tickets took 2 days to close in September and 5 days in October." 5
  # 2. Add a source.
  check best "F2 open Add source" ui invoke -Text "Add source"
  check best "F2 type the name" ui type -Text "Name" -Value "Password reset tickets: 120 in September, 610 in October"
  check best "F2 type what it shows" ui type -Text "What it shows" -Value "The help desk's ticket list by type shows 120 password reset tickets in September and 610 in October."
  check best "F2 submit with Ctrl+Enter" ui keys -Text "^{ENTER}"
  check best "F2 the source shows" find_text "Password reset tickets: 120 in September, 610 in October" 5
  # 3. Add a finding.
  check best "F3 open Add finding" ui invoke -Text "Add finding"
  check best "F3 type the finding" ui type -Text "Finding" -Value "Almost all of the extra tickets in October are password resets."
  check best "F3 submit with Ctrl+Enter" ui keys -Text "^{ENTER}"
  check best "F3 the finding shows" find_text "Almost all of the extra tickets in October are password resets." 5
  # 4. Add something to find out.
  check best "F4 open Add something to find out" ui invoke -Text "Add something to find out"
  check best "F4 type it" ui keys -Text "When did the new password rules start?"
  check best "F4 submit with Ctrl+Enter" ui keys -Text "^{ENTER}"
  check best "F4 it shows" find_text "When did the new password rules start?" 5
  # 5. Write the answer.
  check best "F5 open Write the answer" ui invoke -Text "Write the answer"
  check best "F5 type the answer" ui type -Text "Answer" -Value "Password resets went from 120 to 610, and that is most of the extra work."
  check best "F5 Save answer" ui invoke -Text "Save answer"
  check best "F5 the answer shows" find_text "Password resets went from 120 to 610, and that is most of the extra work." 5
  ui capture -Out "$ROOT/desktop/build/bin/smoke-typed.png" >>"$LOG" 2>&1
  "$W" stop >>"$LOG" 2>&1
fi

echo "== log: $LOG"
fg=$(grep -c 'took the foreground' "$WARN" 2>/dev/null || true)
if [ "${fg:-0}" -gt 0 ]; then
  echo "win-smoke: a Casebook window took the foreground during $fg action(s):"
  grep 'took the foreground' "$WARN" | sed 's/^/  /'
else
  echo "win-smoke: no action took the foreground"
fi
if [ "$failed" -ne 0 ]; then
  echo "win-smoke: a required step failed"
  exit 1
fi
if [ "$best_failed" -ne 0 ]; then
  echo "win-smoke: required steps passed; some best effort F checks failed"
else
  echo "win-smoke: every step passed"
fi
exit 0
