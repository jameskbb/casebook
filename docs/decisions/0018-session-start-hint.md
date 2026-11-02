# 0018: The session-start hook hints when no investigation is active

- Status: Accepted
- Date: 2026-10-07

## Context

With `casebook setup claude --hooks`, the SessionStart hook loads the active investigation's compact pack into every new Claude Code session ([Claude Code guide](../claude-code.md#what-the-hooks-do)). The first build printed nothing when no investigation was active, following the audit requirement that the hooks ask nothing of unrelated sessions; a hint existed only behind the opt-in flag `--hint-when-inactive`.

The agent eval (`scripts/agent-eval`, October 2026) measured what that silence costs. In the `config-precedence` scenario the agent finds the cause in under a minute and the person says the work will continue later. With the MCP server instructions and the hooks but no hint, Claude offered a Casebook case in 0 of 5 runs; with the one-line hint, in 4 of 5. In the `pagination` control (a quick fix that needs no case), no run offered a case, with or without the hint, over four runs.

## Decision

- The hint is on by default. When no investigation applies, `casebook hook session-start` prints one line, about 80 tokens: if this session turns into an investigation (a bug or failure still being root-caused, work likely to continue later), offer to open a Casebook case; to continue an earlier one, find it with the search tools. It is conditional, names the two kinds of work it means, and asks nothing of a session that stays unrelated.
- `casebook hook session-start --quiet-when-inactive` prints nothing instead, and `casebook setup claude --hooks --quiet-when-inactive` writes that flag into the hook command. Setup recognizes the flag (and the old `--hint-when-inactive`) on an existing command, so a rerun replaces the handler instead of adding a second one next to it; a rerun without the flag restores the hint. `--hint-when-inactive` stays accepted and hidden so hook commands written for the old default keep working.
- The marked scope (`--scope marked`, `CASEBOOK_HOOK_SCOPE=marked`) still silences both hooks outside marked projects, hint included.

## Why it stays on for the launch

The quiet default was reconsidered before the first public release, weighing the annoyance of an agent that offers cases in sessions that are not investigations and the token cost per session against the evidence.

- The hooks are opt-in. Someone who runs `setup claude --hooks` wants the agent to use Casebook without being told; without the hint, that promise failed in the eval for exactly the sessions it is meant for, the ones that end before anything was recorded.
- The cost is one line, once per session start, resume, clear or compaction, only while no case is active. When a case is active the hook injects up to 6000 bytes instead, so the hint is the cheap branch.
- The risk of over-offering is real but unmeasured beyond four control runs that showed none. The hint text is deliberately narrow, and the opt-out is a setup flag rather than a hand edit, so turning it off is one command that survives reruns.

## Consequences

- Sessions with the hooks installed and no active case carry one extra line of context.
- The numbers come from a handful of runs and are a pattern, not a benchmark. Rerun `config-precedence` and `pagination` after changing the hint text or the MCP instructions.
- If people report unwanted offers, the first move is to narrow the hint text, the second to flip the default back; both are one-line changes with the opt-out already in place.
