# Agent eval rubric

`grade.py` scores each session from what landed in the run's Casebook database (the exports saved after the session) and the tool calls in the session log. Every check is one point; nothing is judged by a model. A session's score is the number of checks that pass.

## Investigation scenarios, first session (10 points)

The first session of a scenario, or any later session that starts with no case in the database.

| Check | Passes when |
|---|---|
| `used` | The agent called at least one `casebook_*` tool. |
| `oriented` | Before creating a case it looked for one (`casebook_get_active_investigation`, `casebook_list_investigations`, `casebook_search_investigations`, `casebook_get_context` or `casebook_get_investigation`), or the SessionStart hook loaded an active case. |
| `case` | The session wrote to an investigation. |
| `consent` | The case was created only after the agent offered one and the person agreed (the runner's `--confirm` reply). Creating one unasked fails, and so does never offering. |
| `evidence` | At least two evidence records, and at least 80% of them carry a source a reader can follow: an existing repo path (`path:line` is fine) or, for commands and queries, a non-empty `source_ref` or `source_label`. |
| `linked` | At least one evidence link to a claim or hypothesis. |
| `hypotheses` | At least one hypothesis. |
| `ruled_out` | Something was ruled out on the record: a disproven hypothesis or a `contradicts` link. |
| `questions` | At least one question added, or one resolved or deferred. |
| `checkpoint` | A `casebook_checkpoint` call landed. |

`root-cause-recorded` (not scored) says whether a claim, open or supported hypothesis, or the conclusion matches the scenario's `root_cause` pattern.

## Investigation scenarios, continuation sessions (8 points)

A later session of the same scenario when the previous session left a case. The agent starts with no chat history; the prompt only alludes to earlier work ("from this morning").

| Check | Passes when |
|---|---|
| `used` | At least one `casebook_*` call. |
| `resumed` | The case reached the agent before it explored the code: the SessionStart hook loaded an active case, or a Casebook read came before the first `Bash`, `Read`, `Grep` or `Glob`. |
| `same_case` | It wrote to the existing case and created no new one. |
| `no_redo` | It did not re-test what the case already settled: no new hypothesis matching the scenario's `ruled_out` pattern when one was already disproven, and no `Bash`, `Read`, `Grep` or `Glob` call matching the scenario's `redo_patterns` (for flaky-checkout, hour-of-day analysis of the CI runs) once that hypothesis is disproven. The grader lists the matching calls as `redo_calls` in `score.json`. |
| `advanced` | It moved the case forward: a hypothesis, claim or conclusion update, or a resolved question. |
| `evidence`, `linked`, `checkpoint` | As above, for this session's records. |

## Control scenarios (2 points)

A quick fix that needs no case (`pagination`).

| Check | Passes when |
|---|---|
| `restraint` | No case was created unasked. Offering one is fine. |
| `fixed` | The bug is fixed in the work tree (the scenario's `fixed_check`). |

## Reading a scorecard

```
flaky-checkout / claude / sonnet / after: 17 of 18
  session 1: 10/10  +used +oriented +case +consent ...  [casebook calls 6, offered True, confirmed True, $0.31, 74s] root-cause-recorded=yes
  session 2: 7/8  +used +resumed +same_case -no_redo ...
```

`offered` means the agent offered a case somewhere in its text (a pattern match on phrases such as "open a Casebook case" or "start an investigation"); `confirmed` means the runner replied with `--confirm` because no case existed yet. Costs come from Claude Code's result events; Codex reports tokens, not dollars, so its cost shows as $0.00.

The checks are deliberately mechanical, so they can be wrong at the edges: an evidence `source_ref` naming two files separated by commas counts by its first path, and `no_redo` cannot tell a careful re-check of a settled point from a blind repeat (nor a pattern word used for something else). Read the exports in the run directory before drawing conclusions from a single point.
