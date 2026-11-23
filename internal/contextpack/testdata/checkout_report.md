# TestCheckoutTotals flakes in CI

Asked on October 5, 2026. Last changed on October 7, 2026.

## Background

Why does TestCheckoutTotals fail about once in 20 CI runs and never locally?

## Answer

Two parallel tests share a package-level cart cache; the slower 4-core runners widen the race window.

0 of 6 sources checked by a person. 6 not checked.

Nobody has checked this answer yet.

How we know: The race detector reproduces it (E-006), the code shows the shared map (E-005), and every failure was on the 4-core pool (E-004).

## Findings

- TestCheckoutTotals fails when TestCheckoutDiscounts runs concurrently and overwrites the package-level cartCache map. (0 of 3 sources checked by a person)
  - Why we think so: Reproduced 50 of 50 times with -race; the map is written under t.Parallel().
  - Backs it: All 23 failures ran on the 4-core runner pool (Not checked)
  - Backs it: cartCache is a package-level map written under t.Parallel() (Not checked)
  - Backs it: go test -race reproduces a data race on cartCache, 50 of 50 runs (Not checked)
- The two checkout tests share cartCache when the package runs in parallel on slow runners. (0 of 1 source checked by a person)
  - Backs it: All 23 failures ran on the 4-core runner pool (Not checked)

## Found out

- Do the failure timestamps cluster around 00:00 UTC?
  - Answer: No: failures are spread evenly across the day (E-003).

## Still to find out

- Do other packages keep test state in package-level maps? (not answered yet)

## Ruled out

- The failure is a midnight boundary: the fixture's date flips between seeding and assertion.
  - Why we thought so: No failure is near midnight (E-003).
  - Backs it: Fixture seeds order dates from time.Now() truncated to the day (Not checked)
  - Goes against it: Failure times are spread across the day; none within 30 min of 00:00 UTC (Not checked)

## Sources

- 23 of the last 400 CI runs fail on the totals assertion (Not checked)
  - CI failure log, dated October 5, 2026
  - Where to find it: ci/logs/checkout-2026-10.txt
  - All 23 failures are the same assertion: expected 3 line items, got 2.
- Fixture seeds order dates from time.Now() truncated to the day (Not checked)
  - Something your AI read, dated October 5, 2026
  - Where to find it: internal/checkout/totals_test.go:41
  - The fixture computes the order date once and the assertion recomputes it.
- Failure times are spread across the day; none within 30 min of 00:00 UTC (Not checked)
  - CI runs query, dated October 5, 2026
  - Failures spread evenly over 24 hours.
- All 23 failures ran on the 4-core runner pool (Not checked)
  - CI runs query, dated October 6, 2026
  - 0 of 23 on the 8-core pool, which ran 61% of the jobs.
- cartCache is a package-level map written under t.Parallel() (Not checked)
  - Something your AI read, dated October 6, 2026
  - Where to find it: internal/checkout/cache.go:18
  - Both tests write the same map without a lock.
- go test -race reproduces a data race on cartCache, 50 of 50 runs (Not checked)
  - Local race run, dated October 6, 2026
  - The race detector reports concurrent map writes in every run.

## How we got here

### October 5, 2026

- Claude Code created this question
- Claude Code added the source "23 of the last 400 CI runs fail on the totals assertion"
- Claude Code added the source "Fixture seeds order dates from time.Now() truncated to the day"
- Claude Code added the finding "The failure is a midnight boundary: the fixture's date flips between seeding and assertion." with 1 source
- Claude Code added something to find out: "Do the failure timestamps cluster around 00:00 UTC?"
- Codex added the source "Failure times are spread across the day; none within 30 min of 00:00 UTC"

### October 6, 2026

- Codex added the source "All 23 failures ran on the 4-core runner pool"
- Codex added the source "Failure times are spread across the day; none within 30 min of 00:00 UTC" to the finding "The failure is a midnight boundary: the fixture's date flips between seeding and assertion.", which it goes against
- Codex ruled out the finding "The failure is a midnight boundary: the fixture's date flips between seeding and assertion."
- Codex answered "Do the failure timestamps cluster around 00:00 UTC?": No: failures are spread evenly across the day (E-003).
- Codex added the finding "The two checkout tests share cartCache when the package runs in parallel on slow runners." with 1 source
- Claude Code added the source "cartCache is a package-level map written under t.Parallel()"
- Claude Code added the source "go test -race reproduces a data race on cartCache, 50 of 50 runs"
- Claude Code added the finding "TestCheckoutTotals fails when TestCheckoutDiscounts runs concurrently and overwrites the package-level cartCache map." with 3 sources
- Claude Code said the finding "The two checkout tests share cartCache when the package runs in parallel on slow runners." turned out right
- Claude Code added something to find out: "Do other packages keep test state in package-level maps?"
- Claude Code wrote the answer

### October 7, 2026

- A person decided the finding "TestCheckoutTotals fails when TestCheckoutDiscounts runs concurrently and overwrites the package-level cartCache map." holds
- A person updated the answer

_Made with Casebook. These are the saved findings and sources, word for word._
