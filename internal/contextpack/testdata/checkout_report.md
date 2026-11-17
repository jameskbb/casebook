# INV-0002: TestCheckoutTotals flakes in CI

- Status: Investigating
- Opened: 2026-10-05
- Last updated: 2026-10-07

## Question

Why does TestCheckoutTotals fail about once in 20 CI runs and never locally?

## Conclusion

Two parallel tests share a package-level cart cache; the slower 4-core runners widen the race window.

Conclusion verification: AI generated

Supporting claims: none selected.

Contradicting claims: none selected.

Conclusion review: not reviewed; needs review.

Confidence: High. The race detector reproduces it (E-006), the code shows the shared map (E-005), and every failure was on the 4-core pool (E-004).

## Current claims

- **C-001** TestCheckoutTotals fails when TestCheckoutDiscounts runs concurrently and overwrites the package-level cartCache map. (High confidence, Human verified)
  - Why: Reproduced 50 of 50 times with -race; the map is written under t.Parallel().
  - Supported by E-004: All 23 failures ran on the 4-core runner pool (CI runs query; AI generated)
  - Supported by E-005: cartCache is a package-level map written under t.Parallel() (Code: internal/checkout/cache.go:18; AI generated)
  - Supported by E-006: go test -race reproduces a data race on cartCache, 50 of 50 runs (Local race run; AI generated)

## Ruled out

- **H-001** The failure is a midnight boundary: the fixture's date flips between seeding and assertion. (disproven, AI generated)
  - Why: No failure is near midnight (E-003).
  - Supported by E-002: Fixture seeds order dates from time.Now() truncated to the day (Code: internal/checkout/totals_test.go:41; AI generated)
  - Contradicted by E-003: Failure times are spread across the day; none within 30 min of 00:00 UTC (CI runs query; AI generated)

## Hypotheses still open or supported

- **H-002** The two checkout tests share cartCache when the package runs in parallel on slow runners. (Supported, High confidence, AI generated)
  - Supported by E-004: All 23 failures ran on the 4-core runner pool (CI runs query; AI generated)

## Answered questions

- **Q-001** Do the failure timestamps cluster around 00:00 UTC?
  - Answer: No: failures are spread evenly across the day (E-003). (2026-10-06)

## Open questions

- **Q-002** Do other packages keep test state in package-level maps?

## Evidence

- **E-001** 23 of the last 400 CI runs fail on the totals assertion
  - CI failure log: ci/logs/checkout-2026-10.txt, captured 2026-10-05, AI generated
  - All 23 failures are the same assertion: expected 3 line items, got 2.
- **E-002** Fixture seeds order dates from time.Now() truncated to the day
  - Code: internal/checkout/totals_test.go:41, captured 2026-10-05, AI generated
  - The fixture computes the order date once and the assertion recomputes it.
- **E-003** Failure times are spread across the day; none within 30 min of 00:00 UTC
  - CI runs query, captured 2026-10-05, AI generated
  - Failures spread evenly over 24 hours.
- **E-004** All 23 failures ran on the 4-core runner pool
  - CI runs query, captured 2026-10-06, AI generated
  - 0 of 23 on the 8-core pool, which ran 61% of the jobs.
- **E-005** cartCache is a package-level map written under t.Parallel()
  - Code: internal/checkout/cache.go:18, captured 2026-10-06, AI generated
  - Both tests write the same map without a lock.
- **E-006** go test -race reproduces a data race on cartCache, 50 of 50 runs
  - Local race run, captured 2026-10-06, AI generated
  - The race detector reports concurrent map writes in every run.

## Timeline (UTC)

### 2026-10-05

- 11:13 ai claude-code: Created this case
- 13:26 ai claude-code: Added evidence E-001 "23 of the last 400 CI runs fail on the totals assertion"
- 15:39 ai claude-code: Added evidence E-002 "Fixture seeds order dates from time.Now() truncated to the day"
- 17:52 ai claude-code: Added hypothesis H-001 (Medium confidence, 1 evidence link)
- 20:05 ai claude-code: Asked Q-001: Do the failure timestamps cluster around 00:00 UTC?
- 22:18 ai codex: Added evidence E-003 "Failure times are spread across the day; none within 30 min of 00:00 UTC"

### 2026-10-06

- 00:31 ai codex: Added evidence E-004 "All 23 failures ran on the 4-core runner pool"
- 02:44 ai codex: Linked evidence E-003 to H-001, which it contradicts
- 04:57 ai codex: Ruled out H-001; edited the rationale
- 07:10 ai codex: Resolved Q-001: No: failures are spread evenly across the day (E-003).
- 09:23 ai codex: Added hypothesis H-002 (Medium confidence, 1 evidence link)
- 11:36 ai claude-code: Added evidence E-005 "cartCache is a package-level map written under t.Parallel()"
- 13:49 ai claude-code: Added evidence E-006 "go test -race reproduces a data race on cartCache, 50 of 50 runs"
- 16:02 ai claude-code: Added claim C-001 (High confidence, 3 evidence links)
- 18:15 ai claude-code: Marked H-002 Supported; confidence now High
- 20:28 ai claude-code: Asked Q-002: Do other packages keep test state in package-level maps?
- 22:41 ai claude-code: Updated the conclusion (Medium confidence)

### 2026-10-07

- 00:54 human casebook-ui: Marked C-001 Human verified
- 03:07 human casebook-ui: Updated the conclusion (High confidence)

_Generated by Casebook from INV-0002._
