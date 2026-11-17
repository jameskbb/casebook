# Agent eval

Does an AI agent use Casebook well when nobody tells it to? This harness answers that with real headless Claude Code and Codex sessions on small buggy repositories, ordinary prompts that never mention Casebook, and a mechanical grade of what lands in the database.

Run it after any change to what the model sees: the MCP server instructions (`internal/mcpserver/server.go`), tool descriptions and schemas, the hook texts (`internal/integrations/hookio.go`), the skill (`internal/integrations/skill/SKILL.md`) or the `casebook setup instructions` section.

## Isolation

Every run lives in its own directory (`--out`, default `$TMPDIR/casebook-agent-eval/<scenario>-<client>-<time>`) and touches nothing else:

- Casebook: `CASEBOOK_DB` and `CASEBOOK_HOME` point into the run directory and the MCP entry and hooks carry `--db`. The runner checks that `casebook status` resolves the database inside the run directory before any session starts.
- Claude Code: `casebook setup claude` writes into a private `CLAUDE_CONFIG_DIR` in the run directory; the session gets that MCP entry through `--mcp-config --strict-mcp-config`, those hooks through `--settings`, and `--setting-sources project,local`, so your user settings, hooks, plugins and MCP servers are not loaded. Skills setup installed are copied into the sandbox repo's `.claude/skills`.
- Codex: a private `CODEX_HOME` in the run directory. The one link to your real setup is `~/.codex/auth.json`, symlinked in so Codex can sign in and removed when the run ends.
- `--out` is refused when it overlaps the repository, `~/.claude`, `~/.codex`, the Casebook data directory, or `CASEBOOK_HOME`, `XDG_DATA_HOME`, `CLAUDE_CONFIG_DIR` or `CODEX_HOME`.

Claude Code still writes its own session transcripts under `~/.claude/projects`: the runner resumes a session for the consent reply, which `--no-session-persistence` would rule out. Codex writes its session files into the private `CODEX_HOME`. The session logs (`session-N/log.jsonl`) stay in the run directory; nothing in it is committed.

## Requirements

- `python3` (standard library only), `go` (the scenarios are Go repositories; the runner puts `go` on the agent's PATH), `git`.
- `claude` signed in, and for `--client codex`, `codex` signed in.
- Money: a Sonnet session costs about $0.10 to $0.40, a Haiku session under $0.10, an Opus session about $0.30 to $0.60. Each Claude turn is capped by `--budget` (default $1.50) and `--max-turns` (default 40).

## Usage

```sh
python3 scripts/agent-eval/run.py flaky-checkout --model sonnet --extras=--skill
python3 scripts/agent-eval/run.py flaky-checkout --client codex --extras=--skill --snippet
python3 scripts/agent-eval/run.py config-precedence --model sonnet --extras=--skill --no-hint
python3 scripts/agent-eval/grade.py --table /tmp/casebook-agent-eval/*/
```

`run.py` builds `casebook` from this checkout (or takes `--casebook PATH`, handy for comparing two builds), copies the scenario repo into the run directory as a fresh git repository, runs `casebook setup` for the client (`--extras` adds setup flags such as `--skill`; `--no-hooks` skips the Claude hooks), then runs each session of the scenario as a new agent process with no chat history. After every session it saves the investigation list, every export and `casebook status` into `session-N/`, and at the end it prints the scorecard and writes `score.json`.

Variants:

- `--extras=--skill`: install the Casebook skill (`setup claude --skill`, `setup codex --skill`).
- `--snippet`: append `casebook setup instructions` to the sandbox's `CLAUDE.md` or `AGENTS.md`, as a person following the docs would.
- `--no-hint`: install the hooks with `--quiet-when-inactive`, so a session with no active case gets no hint about offering one (the hint is the default).

Consent: the prompts never ask for a case, and the agent should offer one rather than open it unasked. When a session ends with no case in the database and the agent offered one anywhere in its text, the runner replies once in the same session with `--confirm` ("Yes, please open a case for it."), playing the person who says yes. Codex runs approve the Casebook tools up front (`default_tools_approval_mode="approve"`), since nobody is there to click approve.

## Scenarios

| Scenario | Kind | Sessions | What it tests |
|---|---|---|---|
| `flaky-checkout` | investigation | 2 | An engineering scenario: `TestCheckoutTotals` flakes because two `t.Parallel()` tests share a package-level `cartCache` map. A midnight-boundary fixture is the decoy, and the CI logs rule it out. Session 1 ends with "we'll pick it up later"; session 2 ("back on the flaky test from this morning") should resume from the case without re-testing the disproven theory. |
| `config-precedence` | investigation | 1 | `SVC_PORT=9090` is ignored in staging: the entrypoint passes `-port ${PORT:-8080}` and the config file is applied after the environment. "Don't fix anything yet." |
| `pagination` | control | 1 | An off-by-one that drops one order per page. A quick fix: the agent should just fix it, with no case opened unasked. |

A scenario is `scenarios/<name>/scenario.json` (prompts, kind, the `root_cause`, `ruled_out` and `redo_patterns` regexes the grader uses, and for controls a `fixed_check`) plus `repo/`, a small Go module. Each repo has its own `go.mod`, so the main module's `go test ./...` skips it.

## Scoring

See [rubric.md](rubric.md). In short: a first session can earn 10 points (used Casebook, looked for an existing case, opened one with consent, sourced evidence, links, hypotheses, something ruled out, questions, checkpoint); a continuation session 8 (resumed from the case before exploring, same case, no redo of disproven work, moved the case forward, evidence, links, checkpoint); a control 2 (no unasked case, bug fixed).
