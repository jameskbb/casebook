#!/usr/bin/env bash
# Windows host end-to-end smoke for the Casebook desktop app (D-27), run
# from WSL after desktop/scripts/win-run.sh build.
#
#   win-smoke.sh            run every step (A to E required, F best effort)
#   win-smoke.sh A B C D    run only the named steps (C and D need B first)
#
# Steps:
#   A  fresh launch with no configuration creates the database and shows
#      "Start your first case" (UI-01, brief step 1)
#   B  seeded demo renders the detail and drills down claim, then evidence,
#      with Back (UI-15)
#   C  a separate process (aiwrite.exe as ai claude-code) writes while the
#      app is open; the app shows it within 5 seconds (UI-22, brief 18, 19)
#   D  Copy context puts the markdown pack on the real clipboard (UI-19)
#   E  close and relaunch on the same data dir keeps everything (brief 12, 13)
#   F  best effort: brief steps 2 to 11 driven through the UI
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

if want A; then
  echo "== A: fresh launch (UI-01)"
  "$W" reset fresh >>"$LOG" 2>&1
  check required "A launch on a fresh data dir" "$W" start fresh
  check required "A shows Start your first case" find_text "Start your first case"
  check required "A created casebook.db" test -s "$("$W" datadir fresh)/casebook.db"
  "$W" stop >>"$LOG" 2>&1
fi

if want B; then
  echo "== B: render and drill-down (UI-15)"
  "$W" reset demo >>"$LOG" 2>&1
  check required "B launch the seeded demo" "$W" start demo --seed-demo
  check required "B shows What we believe" find_text "What we believe"
  check required "B shows Current conclusion" find_text "Current conclusion"
  check required "B open claim C-003" ui invoke -Text "Open claim C-003: The July 6 playbook update made white-glove onboarding free"
  check required "B claim drawer shows Supporting evidence" find_text "Supporting evidence"
  # The drawer's evidence block comes after the reading column's evidence
  # row of the same name, so take the last match.
  check required "B open evidence E-008 from the claim drawer" ui invoke -Text "Open evidence E-008" -Last
  check required "B evidence drawer shows Back to C-003" find_text "Back to C-003"
  check required "B Esc closes the drawers" ui keys -Text "{ESC}"
fi

if want C; then
  echo "== C: live refresh from another process (UI-22)"
  out=$("$W" aiwrite demo 2>&1)
  echo "$out" >>"$LOG"
  echo "     aiwrite: $out"
  check required "C new evidence title within 5 s" find_text "Four Q4 deals above \$250k are already signed under the July playbook" 5
  check required "C History says Claude Code (AI) added evidence" find_text "Claude Code (AI) added evidence E-011" 5
fi

if want D; then
  echo "== D: Copy context to the real clipboard (UI-19)"
  check required "D invoke Copy for your AI" ui invoke -Text "Copy for your AI"
  clip=$("$W" ui clipboard 2>>"$LOG")
  first=$(printf '%s\n' "$clip" | head -n 1)
  echo "     clipboard starts: ${first:0:60}..."
  # The pack is framed for an AI reader: a line saying it is stored data,
  # then BEGIN CASEBOOK DATA, the pack, END CASEBOOK DATA.
  check required "D clipboard starts with the data framing line" test "${first#The block between the BEGIN and END CASEBOOK DATA }" != "$first"
  check required "D clipboard holds the INV-0002 pack" grep -qx "Investigation: INV-0002" <<<"$clip"
fi

if want E; then
  echo "== E: restart persistence (brief steps 12 and 13)"
  "$W" stop >>"$LOG" 2>&1
  check required "E relaunch without the seed flag" "$W" start demo
  check required "E the other process's evidence is still there" find_text "Four Q4 deals above \$250k are already signed under the July playbook"
  check required "E the case is still there" find_text "Northeast margin down 6 points in Q3"
  "$W" stop >>"$LOG" 2>&1
fi

if want F; then
  echo "== F: brief steps 2 to 11 through the UI (best effort)"
  "$W" reset typed >>"$LOG" 2>&1
  check best "F launch a fresh data dir" "$W" start typed
  # 2. Create a case.
  check best "F2 open New case" ui invoke -Text "New case"
  check best "F2 type the title" ui type -Text "Title" -Value "Smoke typed case"
  # Leave it inactive so step 11 can set it active.
  check best "F2 untick Make this your AI's current case" ui invoke -Text "Make this your AI's current case"
  check best "F2 Create case" ui invoke -Text "Create case"
  check best "F2 the new case opens" find_text "INV-0001" 5
  # 3. Edit title and question.
  check best "F3 open Write the question (the empty question block)" ui invoke -Text "Write the question"
  check best "F3 type the question" ui type -Text "Primary question (optional)" -Value "Why did Northeast margin fall in Q3?"
  check best "F3 Save changes" ui invoke -Text "Save changes"
  check best "F3 the question shows" find_text "Why did Northeast margin fall in Q3?" 5
  # 4. Add evidence.
  check best "F4 open Add evidence" ui invoke -Text "Add evidence"
  check best "F4 type the title" ui type -Text "Title" -Value "Deal desk shows seven deals with no onboarding fee"
  check best "F4 type the summary" ui type -Text "Summary" -Value "The deal desk approvals export lists seven Northeast deals above \$250k with an onboarding fee of \$0."
  check best "F4 submit with Ctrl+Enter" ui keys -Text "^{ENTER}"
  check best "F4 the evidence shows" find_text "Deal desk shows seven deals with no onboarding fee" 5
  # 5 and 6. Add a claim supported by the evidence at Medium confidence.
  check best "F5 open Add claim" ui invoke -Text "Add claim"
  check best "F5 type the statement" ui type -Text "Statement" -Value "The onboarding fee was waived by the July playbook."
  # Statement, then the confidence group (Not assessed), Right twice is
  # Medium; Tab to the rationale, Tab to the E-001 choice (None), Right is
  # Supports.
  check best "F6 choose Medium and Supports with the keyboard" ui keys -Text "{TAB}{RIGHT}{RIGHT}{TAB}{TAB}{RIGHT}"
  check best "F6 submit with Ctrl+Enter" ui keys -Text "^{ENTER}"
  check best "F6 the claim shows" find_text "The onboarding fee was waived by the July playbook." 5
  check best "F6 the claim is Medium confidence" find_text "Confidence: Medium" 5
  check best "F6 E-001 supports the claim" find_text "E-001 supports this" 5
  # 7. Add a hypothesis.
  check best "F7 open Add hypothesis" ui invoke -Text "Add hypothesis"
  check best "F7 type the statement" ui type -Text "Statement" -Value "Travel on the white-glove accounts may also be up."
  check best "F7 submit with Ctrl+Enter" ui keys -Text "^{ENTER}"
  check best "F7 the hypothesis shows" find_text "Travel on the white-glove accounts may also be up." 5
  # 8. Add a question.
  check best "F8 open Add question" ui invoke -Text "Add question"
  check best "F8 type the question" ui type -Text "Question" -Value "Who approves a change to the sales playbook?"
  check best "F8 submit with Ctrl+Enter" ui keys -Text "^{ENTER}"
  check best "F8 the question shows" find_text "Who approves a change to the sales playbook?" 5
  # 9 and 10. Write the conclusion.
  check best "F9 open Write a conclusion" ui invoke -Text "Write a conclusion"
  check best "F9 type the conclusion" ui type -Text "Conclusion" -Value "Free white-glove onboarding drove the margin drop."
  check best "F9 Save conclusion" ui invoke -Text "Save conclusion"
  check best "F10 the conclusion shows" find_text "Free white-glove onboarding drove the margin drop." 5
  # 11. Use with my AI.
  check best "F11 Use with my AI" ui invoke -Text "Use with my AI"
  check best "F11 the toast confirms it" find_text "is now your AI's current case" 5
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
