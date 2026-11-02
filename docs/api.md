# internal/app API reference

The app and the guides call an investigation a case ([ADR 0021](decisions/0021-case-is-the-word.md)); the API keeps the name investigation.

This is the contract every Casebook surface codes against: the desktop app (phase 2), the CLI and MCP server (phase 3), hooks and diagnostics (phase 4). It was written from `go doc -all ./internal/app` and `go doc -all ./internal/app/apptest` at the end of phase 1. When this file and the code disagree, the code wins and this file is the bug. The Go doc comments on each method carry the full detail; this page is the map.

Contents:

1. Purpose and rules
2. Opening the engine
3. Actors per surface
4. Refs
5. Active investigation defaulting
6. Errors and surface mapping
7. Method reference
8. DTOs and JSON fields
9. Enums
10. Activity actions
11. Byte limits
12. Context pack
13. Change token (live refresh)
14. Checkpoint
15. Export format v2 and legacy import
16. Backup and restore
17. Test fixture (apptest)
18. Concurrency model

## 1. Purpose and rules

- `internal/app` is the only API. Surfaces import `github.com/jameskbb/casebook/internal/app` and, only to show or locate data files, `internal/paths`.
- Surfaces never import `internal/store`, `internal/domain` or `internal/contextpack`. Every DTO, enum, constant, error and helper a surface needs is re-exported from `internal/app` as an alias (`app.Evidence`, `app.ConfidenceHigh`, `app.ErrNotFound`, `app.Pack`, ...).
- `internal/app/boundary_test.go` (`TestPackageBoundaries`) enforces this for every `.go` file in the module, including files added by later phases: only `internal/app` (not `internal/app/apptest`) and `internal/store` may import `internal/store`; only `internal/app`, `internal/store` and `internal/contextpack` may import `internal/domain`; only `internal/app` may import `internal/contextpack`; nothing imports `internal/deps`.
- Business logic lives in `internal/app`. A surface validates nothing the engine already validates; it passes user input through and maps the returned error (section 6).
- `internal/app/contract_test.go` pins every method name and signature (`engineContract`), the JSON tag rules of every DTO and the presence of every method in this file.

## 2. Opening the engine

```go
func Open(dbPath string, opts ...Option) (*App, error)
func OpenDefault(opts ...Option) (*App, error)
func (a *App) Close() error
func (a *App) DBPath() string

func WithLogger(l *slog.Logger) Option
func WithClock(now func() time.Time) Option
func WithBusyTimeout(d time.Duration) Option
func WithRetryBudget(d time.Duration) Option
func WithWorkDir(dir string) Option
```

- `Open(dbPath)` creates the parent directory (0700) and the file (0600) when missing, applies the pragmas (WAL, foreign keys, busy timeout, synchronous NORMAL) and the embedded forward-only migrations. Before it migrates an existing database (schema version above 0) that has pending migrations, it writes a `VACUUM INTO` copy to `<directory of the database>/backups/casebook-pre-upgrade-schema<old version>-<UTC 20060102T150405Z>.db` (`-2`, `-3`, ... on collision; the prefix is `app.PreUpgradePrefix`). A fresh or up-to-date database takes no copy. When the copy cannot be written, `Open` fails with an error that starts `cannot back up the database before upgrading it` (wrapping the filesystem reason, or `ErrInternal`, `ErrBusy` or `ErrConflict`) and the file keeps its old schema. A database written by a newer Casebook is refused with `ErrSchemaTooNew` and left untouched. Concurrent first opens from several processes are safe; each may write its own pre-upgrade copy.
- `OpenDefault()` opens `paths.DBPath()`. Resolution order: `CASEBOOK_DB` (a file path) wins; else `CASEBOOK_HOME/casebook.db`; else the per-OS data directory plus `casebook.db`: Windows `%LocalAppData%\Casebook`, macOS `~/Library/Application Support/Casebook`, Linux and others `$XDG_DATA_HOME/casebook` or `~/.local/share/casebook`. Relative env values are made absolute.
- Permissions: directories Casebook creates are 0700, the database, its `-wal` and `-shm` files and every backup are 0600 (where the OS supports modes; skipped on Windows).
- `Close()` releases everything, including the change token connection; calling it twice is safe. Open one `App` per process and share it; it is safe for concurrent use.
- `DBPath()` returns the absolute database path (show it in status and diagnostics).
- `WithLogger`: the default logger discards everything and Casebook never writes to stdout. The MCP server must keep stdout for protocol traffic, so it passes a stderr logger, for example `app.WithLogger(slog.New(slog.NewTextHandler(os.Stderr, nil)))`. Logs carry operation names and refs only, never evidence text, raw detail or metadata; raw storage errors appear only at Warn level.
- `WithClock` replaces the clock for every timestamp (tests and golden files). Production code never passes it.
- `WithWorkDir` sets the directory the surface works in (the CLI's current directory, a hook payload's `cwd`, the MCP server's working directory). Empty or a value that cannot be made absolute means none. With a working directory, "the active investigation" (an empty `invRef`, `GetActiveInvestigation`, `is_active`) is the directory binding of that directory or its nearest bound ancestor, else the global setting ([ADR-0015](decisions/0015-per-directory-active-investigation.md)). The desktop app passes none and sees only the global setting.
- `WithBusyTimeout` sets SQLite's per-connection `busy_timeout` (default 5s). `WithRetryBudget` sets how long a write keeps retrying on lock contention before returning `ErrBusy` (default 10s). The budget is checked between attempts and each attempt can itself wait inside SQLite for up to the busy timeout, so the worst-case wait is about the retry budget plus one busy timeout (15s with the defaults). Surfaces normally keep both defaults.

## 3. Actors per surface

Every mutating method takes `actor Actor` as its second argument. `Actor` is `{Kind ActorKind, Client string}` (JSON `{"kind","client"}`); the client is one line of at most 64 bytes.

| Surface | Constructor | Value |
|---|---|---|
| Desktop app | `app.ActorDesktop()` | `{human, casebook-ui}` |
| CLI | `app.ActorCLI()` | `{human, casebook-cli}` |
| Hooks (run through the CLI) | `app.ActorCLI()` | `{human, casebook-cli}` |
| MCP server | `app.ActorMCP(clientName)` | `{ai, <client name>}` where the name comes from the MCP `initialize` handshake (`clientInfo.name`, for example `claude-code` or `codex`) |

- `ActorMCP` normalizes the untrusted client name so every MCP write validates: invalid UTF-8, control characters and invisible format characters are dropped as for free text, newlines and tabs become a space, surrounding whitespace is trimmed, the name is cut to 64 bytes at a character boundary, and an empty result becomes `app.DefaultMCPClient` (`mcp-client`). Pass `clientInfo.name` as received.
- Kinds `import` and `system` are written by the engine itself (import provenance); surfaces do not construct them.
- Verification defaults follow the actor kind (`app.DefaultVerification(kind)`): a new evidence, claim or hypothesis from a `human` actor is `human_verified`, from any other kind `ai_generated`, unless the input sets a verification explicitly. Patches that leave a superseded (or, for claims, disputed) state reset verification to the actor's default unless they set one.
- Only a person sets `human_verified` ([ADR-0014](decisions/0014-only-people-verify.md)): any write by an actor whose kind is not `human` (`ai`, `import`, `system`) that names verification `human_verified` on evidence, a claim or a hypothesis is a `ValidationError` on `verification` (`only a person can mark a record human_verified; use source_verified when you checked it against the source, or leave it for a person to verify`). This covers `AddEvidence`, `UpdateEvidence`, `SetEvidenceVerification`, `AddClaim`, `UpdateClaim`, `SetClaimVerification`, `AddHypothesis`, `UpdateHypothesis`, `SetHypothesisVerification` and every `CheckpointBatch` item (the field then names the item path, for example `claim_updates[0].patch.verification`), and applies even when the record already is `human_verified`. Such an actor may still set `ai_generated`, `source_verified` or `disputed`, including on a record a person verified, so contradicting evidence can always dispute it.
- A non-human actor's edit lowers a vouched verification ([ADR-0014](decisions/0014-only-people-verify.md)): when an `UpdateClaim`, `UpdateHypothesis` or `UpdateEvidence` patch (or a `CheckpointBatch` claim or hypothesis update) from an `ai`, `import` or `system` actor changes a claim's or hypothesis's statement, or any evidence field other than verification, on a record that is `human_verified` or `source_verified`, and names no verification, the same write sets verification `ai_generated`. The activity summary lists `verification` and ends `; verification reset to ai_generated after AI edit` (`after import edit`, `after system edit`). Confidence, rationale, status and link changes keep the verification; a person's edits keep it; a verification named in the same patch (for example `source_verified` after rechecking) wins. `UpdateClaimOutcome`, `UpdateHypothesisOutcome` and `UpdateEvidenceOutcome` are the same writes returning an `UpdateOutcome` as well: `VerificationReset` (`verification_reset`) is true when the write reset the verification this way, and `PreviousVerification` (`previous_verification`) is the label it replaced (empty otherwise), so a surface can tell the person their check was removed. A `CheckpointBatch` reports the same in `verification_resets`.
- The actor is recorded on every activity row and as `created_by` on records and links.

## 4. Refs

- Investigations: `INV-0001` (at least four digits; grows naturally to `INV-10000`). Records are numbered per investigation: evidence `E-001`, claims `C-001`, hypotheses `H-001`, questions `Q-001` (at least three digits).
- Parsing is lenient and case-insensitive: `inv-1`, ` INV-0001 `, `e-1` and `E-001` all work. Returned refs are always canonical (`INV-0001`, `E-001`).
- Qualified record refs: `INV-0002/E-001`. With an empty `invRef` the qualified investigation is used; with an `invRef` naming another investigation it is a `ValidationError` on `ref`.
- A malformed investigation ref is a `ValidationError` on `investigation_ref`. A record ref with the wrong prefix (for example `C-001` passed to `GetEvidence`) or malformed text is a `ValidationError` on `ref`.
- A well-formed ref that does not exist is `ErrNotFound` naming it: `not found: investigation INV-0009`, `not found: evidence E-009 in INV-0001`.
- Refs are never reused: per-kind counters only grow. Import keeps record refs and keeps the investigation ref when it is free, otherwise allocates the next one (section 15).
- Ref numbers run from 1 to `app.MaxRefNum` (999999999999). Parsing and allocation share this one bound, so every returned ref parses back; an allocation past it is `ErrConflict` (`... refs are exhausted ...`) and writes nothing. Import accepts investigation ref numbers up to `app.MaxImportInvestigationRefNum` (999999) and record ref numbers up to `app.MaxImportRecordRefNum` (99999), so an imported file can never push a counter near the bound.
- Links connect evidence to a claim or hypothesis of the same investigation. A missing evidence ref is `ErrNotFound`; a qualified evidence ref in another investigation is `ErrConflict`.

## 5. Active investigation defaulting

The active investigation is one row in the database's settings table (ADR-0008), so setting it in any surface or process is immediately visible to every other one (proven by `TestActiveAcrossProcesses`).

- Every method that takes `invRef` treats an empty (or whitespace-only) string as "the active investigation". When none is set (or it points at an investigation that no longer exists) the method returns `ErrNoActiveInvestigation`, which is distinct from `ErrNotFound`.
- Exceptions:
  - `ListActivity`: an empty (or whitespace-only) `ActivityFilter.InvestigationRef` means all investigations, including global rows.
  - `Search`: an empty (or whitespace-only) `SearchOptions.InvestigationRef` searches every investigation.
  - `SetActiveInvestigation` requires a ref (empty or whitespace-only is a `ValidationError` on `investigation_ref`).
  - `GetActiveInvestigation` returns `nil, nil` when none is set instead of an error.
  - `ImportInvestigation` never changes the active investigation.
- Archiving the active investigation leaves it active. `ClearActiveInvestigation` is the only way to unset it.
- `Investigation.IsActive` and `InvestigationSummary.IsActive` report the flag from the same read snapshot; `StatusInfo.ActiveInvestigationRef` is `""` when none is set.

## 6. Errors and surface mapping

All errors are safe to show: raw SQLite text never leaves `internal/app` (the `a.fail` mapping logs it and returns a sentinel). Test with `errors.Is` for sentinels and `app.IsValidation(err)` for validation errors; check validation first.

| Error | How to test | Meaning | Desktop | CLI | MCP tool result |
|---|---|---|---|---|---|
| `*ValidationError` | `ve, ok := app.IsValidation(err)` | Bad input for one field (400-like). `ve.Field` is a JSON-style path (`title`, `links[1].relation`, `evidence[2].summary`); `ve.Message` says what is wrong and the limit. | Show `ve.Message` next to the field named by `ve.Field` | Print `invalid <field>: <message>`, exit non-zero | Tool error `invalid <field>: <message>` |
| `ErrNotFound` | `errors.Is(err, app.ErrNotFound)` | The ref does not exist (404-like). Text names it: `not found: claim C-009 in INV-0001`. | Show the message; refresh the view | Print the message, exit non-zero | Tool error with the text (starts `not found: ...`) |
| `ErrConflict` | `errors.Is(err, app.ErrConflict)` | The request contradicts existing data (409-like): cross-investigation link, superseding by a superseded record, deleting a record another record depends on or an investigation that is not archived, backup destination exists, invalid backup file (`conflict: not a valid Casebook backup (<reason>)`). | Show the message | Print the message, exit non-zero | Tool error with the text |
| `ErrNoActiveInvestigation` | `errors.Is(err, app.ErrNoActiveInvestigation)` | Empty `invRef` and no active investigation. Text: `no active investigation is set; pass an investigation ref or set one active`. | Ask the user to pick or activate an investigation | Suggest `--inv INV-0001` or activating one | Tool error with the text so the AI passes a ref or asks the user |
| `ErrBusy` | `errors.Is(err, app.ErrBusy)` | The database stayed locked past the retry budget. Text: `Casebook database is busy; try again`. Nothing was written. | Show the text with a Retry action | Print the text, exit non-zero | Tool error with the text; the AI may retry |
| `ErrSchemaTooNew` | `errors.Is(err, app.ErrSchemaTooNew)` | Returned by `Open`: the file comes from a newer Casebook. Text: `this database was saved by a newer version of Casebook; update Casebook to open it (database schema N, this Casebook supports up to M); to keep using this version instead, restore the casebook-pre-upgrade backup from the backups folder next to the database (nothing has been changed)`. | Tell the user to update Casebook; do not offer writes. The startup screen also offers Restore from backup, for going back to this version with the pre-upgrade copy | Print the text, exit non-zero | Same; the server cannot start |
| `*IntegrityError` | `ie, ok := app.IsIntegrity(err)` (also `errors.Is(err, app.ErrInternal)`) | Returned by `CheckIntegrity` only: the database is damaged. `ie.Problems` lists up to 10 sanitized lines naming the check, tables, indexes and row ids (`foreign key violation in claim_evidence (rowid 3) referencing claims`, `integrity_check: problem in index claims_superseded_by`, `integrity_check: damaged database pages`), never raw SQLite text or values; `ie.Total` counts every problem. Text: `casebook: database integrity check failed (N problems): ...`. | Show the text and suggest restoring a backup | Print the problems, exit non-zero | Tool error with the text |
| `ErrInternal` | `errors.Is(err, app.ErrInternal)` | Unexpected storage failure. Details are only in the logs. | Generic failure message | Generic failure, exit non-zero | Tool error `casebook: internal storage error` |
| `context.Canceled`, `context.DeadlineExceeded` | `errors.Is` | The caller's context ended. | Usually silent | Exit non-zero | Report cancellation |
| other (filesystem) | none of the above | `Open`, `Backup` and `Restore` return filesystem problems (permission denied, missing directory) wrapped with the path. | Show the message | Print the message, exit non-zero | Tool error with the text |

`ValidationError` itself marshals as `{"field": "...", "message": "..."}` and its `Error()` is `field: message`. `app.NewValidationError` exists for surfaces that validate their own flags, not for re-validating engine input.

## 7. Method reference

All methods take `ctx context.Context` first; mutators take `actor Actor` second. Read methods use one read snapshot. Every mutation runs in one write transaction that also writes its activity row, so a change and its activity row land together or not at all. "No-op" means the call succeeds, returns the current state and writes nothing (no activity row, change token unchanged). Every limit option (`DetailOptions.ActivityLimit`, `ListOptions.Limit`, `SearchOptions.Limit`, `ActivityFilter.Limit`) works the same way: 0 means the default, and a negative value or one above the maximum is a `ValidationError` on that field (`activity_limit` or `limit`); nothing is silently clamped.

### Investigations

```go
func (a *App) CreateInvestigation(ctx context.Context, actor Actor, in CreateInvestigationInput) (Investigation, error)
func (a *App) GetInvestigation(ctx context.Context, invRef string) (Investigation, error)
func (a *App) GetInvestigationDetail(ctx context.Context, invRef string, o DetailOptions) (InvestigationDetail, error)
func (a *App) ListInvestigations(ctx context.Context, o ListOptions) ([]InvestigationSummary, error)
func (a *App) UpdateInvestigation(ctx context.Context, actor Actor, invRef string, p InvestigationPatch) (Investigation, error)
func (a *App) ArchiveInvestigation(ctx context.Context, actor Actor, invRef string) (Investigation, error)
func (a *App) UnarchiveInvestigation(ctx context.Context, actor Actor, invRef string) (Investigation, error)
func (a *App) UpdateConclusion(ctx context.Context, actor Actor, invRef string, in ConclusionInput) (Investigation, error)
func (a *App) DeleteInvestigation(ctx context.Context, actor Actor, invRef string) error
```

- `CreateInvestigation`: next INV ref; status defaults to `new` and must not be `archived`; `MakeActive` also sets it active (extra `active.set` row).
- `GetInvestigation`: one investigation; empty ref means the active one.
- `GetInvestigationDetail`: investigation plus evidence, claims and hypotheses (with links), questions and the newest `ActivityLimit` activity rows (0 means 20, maximum 200) from one snapshot; raw detail only with `IncludeRawDetail`. The desktop detail view needs only this call.
- `ListInvestigations`: summaries newest updated first; archived hidden unless `IncludeArchived` or `Status` is `archived`; `Status` `""` means any; `Limit` 0 means 100, maximum 1000.
- `UpdateInvestigation`: patch (nil fields unchanged). Status `archived` archives; a non-archived status on an archived investigation unarchives to it. No-op when nothing changes; otherwise one row naming the changed fields, never their values.
- `ArchiveInvestigation`: status `archived`, `archived_at` set, prior status remembered; already archived is a no-op. The investigation stays active if it was.
- `UnarchiveInvestigation`: restores the prior status (`investigating` when none recorded); not archived is a no-op.
- `DeleteInvestigation`: permanently deletes an archived investigation with every record, link and activity row in it (not archived is `ErrConflict`; `invRef` is required, blank is a `ValidationError` on `investigation_ref`, so it never falls back to the active investigation). Clears it as the active investigation when it was, and records one global `investigation.delete` row naming the ref, title and record counts. Surfaces confirm first.
- `UpdateConclusion`: replaces conclusion, confidence (empty means `unknown`) and rationale together; an empty conclusion clears it, and then confidence must be empty or `unknown` and rationale empty. Optional `Verification` follows actor authority in ADR 0014. `SupportingClaims` and `ContradictingClaims` select at most 100 refs per side in the same investigation; nil preserves a selection, `[]` clears it, duplicate refs collapse and a ref on both sides is refused. A selected claim cannot be deleted until removed from the conclusion.
- A conclusion exposes `ConclusionVerification`, `ConclusionSupportingClaims`, `ConclusionContradictingClaims`, `ConclusionReviewedAt`, `ConclusionReviewBasis`, `ConclusionNeedsReview` and the opaque `ConclusionReviewToken`. Explicit human-only `ConclusionInput.Review` requires `ReviewBasis` (at most 2000 bytes) and `ExpectedReviewToken` from the displayed investigation. The same write transaction checks the token and refuses stale material with `ErrConflict`; save changed selections before reviewing. Statement, confidence, rationale or selection edits clear the review; a substantive non-human statement edit also resets checked verification unless a permitted verification is explicitly supplied. Selected claim or directly linked evidence changes flag review needed without erasing historical human statement verification or its recorded review. Comparison is synchronous, with no background work. See [ADR 0023](decisions/0023-conclusion-accountability.md).

### Evidence

```go
func (a *App) AddEvidence(ctx context.Context, actor Actor, invRef string, in EvidenceInput) (Evidence, error)
func (a *App) GetEvidence(ctx context.Context, invRef, evidenceRef string, o GetEvidenceOptions) (Evidence, error)
func (a *App) UpdateEvidence(ctx context.Context, actor Actor, invRef, evidenceRef string, p EvidencePatch) (Evidence, error)
func (a *App) UpdateEvidenceOutcome(ctx context.Context, actor Actor, invRef, evidenceRef string, p EvidencePatch) (Evidence, UpdateOutcome, error)
func (a *App) ListEvidence(ctx context.Context, invRef string) ([]Evidence, error)
func (a *App) SetEvidenceVerification(ctx context.Context, actor Actor, invRef, evidenceRef string, v Verification) (Evidence, error)
func (a *App) DeleteEvidence(ctx context.Context, actor Actor, invRef, evidenceRef string) error
```

- `AddEvidence`: next E ref; title, summary and source type required; `captured_at` defaults to now; verification by actor kind; metadata must be a JSON object. The result omits raw detail (`has_raw_detail` says whether any is stored).
- `GetEvidence`: one evidence with its claim and hypothesis links; raw detail only with `IncludeRawDetail`.
- `UpdateEvidence`: patch; provided fields follow the add rules (empty `CapturedAt` resets to the creation time, empty `RawDetail` or `Metadata` clears); no-op when nothing changes.
- `ListEvidence`: ordered by ref, without raw detail.
- `SetEvidenceVerification`: sets verification; same value is a no-op; `human_verified` from a non-human actor is a `ValidationError` (section 3).
- `DeleteEvidence`: permanently deletes the evidence (`evidence.delete`); `ErrConflict` while any claim or hypothesis links it (unlink first). Surfaces confirm first.

### Claims

```go
func (a *App) AddClaim(ctx context.Context, actor Actor, invRef string, in ClaimInput) (Claim, error)
func (a *App) GetClaim(ctx context.Context, invRef, claimRef string) (Claim, error)
func (a *App) ListClaims(ctx context.Context, invRef string) ([]Claim, error)
func (a *App) UpdateClaim(ctx context.Context, actor Actor, invRef, claimRef string, p ClaimPatch) (Claim, error)
func (a *App) UpdateClaimOutcome(ctx context.Context, actor Actor, invRef, claimRef string, p ClaimPatch) (Claim, UpdateOutcome, error)
func (a *App) SetClaimVerification(ctx context.Context, actor Actor, invRef, claimRef string, v Verification) (Claim, error)
func (a *App) SupersedeClaim(ctx context.Context, actor Actor, invRef, claimRef, replacementRef string) (Claim, error)
func (a *App) DeleteClaim(ctx context.Context, actor Actor, invRef, claimRef string) error
```

- Status and verification of claims and hypotheses form one state (ADR-0013): verification is `superseded` exactly when the status is `superseded` (only superseding writes it), and a claim's verification is `disputed` exactly when its status is `disputed`. Either half alone sets both; another verification (`ai_generated`, `human_verified`, `source_verified`) on a disputed claim makes it `active`; a contradicting pair is a `ValidationError` on `verification`; a verification change alone on a superseded record is `ErrConflict` (reinstate it with a status first). Hypotheses have no disputed status, so their verification may be `disputed` in any status but `superseded`. Migration 0002 repaired older rows and its triggers refuse a disagreeing pair.
- `AddClaim`: next C ref with its evidence links in the same transaction (one `claim.add` row); confidence defaults to `unknown`, status to `active`; status or verification `superseded` is rejected (add, then supersede); status `disputed` and verification `disputed` imply each other. Duplicate links with the same relation collapse; with different relations they are a `ValidationError` on `links[i].relation`.
- `GetClaim`: one claim with links ordered by evidence ref.
- `ListClaims`: every claim in every status, ordered by ref.
- `UpdateClaim`: patch. `SupersededBy` supersedes like `SupersedeClaim` (another verification in the same patch is a `ValidationError`); status `superseded` without a replacement is a `ValidationError` on `superseded_by` (unless one exists); leaving `superseded` clears the replacement. Status and verification follow the pairing rule above. No-op patches write nothing.
- `SetClaimVerification`: sets verification and, by the pairing rule, the status (`disputed` disputes the claim, a verifying value on a disputed claim makes it active; the summary then ends `; status <new status>`); `superseded` is a `ValidationError`; any change on a superseded claim is `ErrConflict`; same value is a no-op.
- `SupersedeClaim`: status and verification `superseded`, `superseded_by` the replacement. The replacement must be another, non-superseded claim of the same investigation (`ErrConflict`; missing is `ErrNotFound`). Same replacement again is a no-op.
- `DeleteClaim`: permanently deletes the claim with its own evidence links (`claim.delete`); `ErrConflict` while another claim names it in `superseded_by` (reinstate or delete that one first). Surfaces confirm first.

### Links

```go
func (a *App) LinkEvidenceToClaim(ctx context.Context, actor Actor, invRef, claimRef, evidenceRef string, rel Relation) (Claim, error)
func (a *App) UnlinkEvidenceFromClaim(ctx context.Context, actor Actor, invRef, claimRef, evidenceRef string) (Claim, error)
func (a *App) LinkEvidenceToHypothesis(ctx context.Context, actor Actor, invRef, hypRef, evidenceRef string, rel Relation) (Hypothesis, error)
func (a *App) UnlinkEvidenceFromHypothesis(ctx context.Context, actor Actor, invRef, hypRef, evidenceRef string) (Hypothesis, error)
```

- `LinkEvidenceToClaim`: new link `link.add`; relinking with another relation updates it (`link.update`); relinking with the same relation is idempotent (no-op). Returns the claim.
- `UnlinkEvidenceFromClaim`: removes the link (`link.remove`); a missing link is `ErrNotFound` (`not found: no link between C-001 and E-002`).
- `LinkEvidenceToHypothesis`, `UnlinkEvidenceFromHypothesis`: same rules for hypotheses; return the hypothesis.

### Hypotheses

```go
func (a *App) AddHypothesis(ctx context.Context, actor Actor, invRef string, in HypothesisInput) (Hypothesis, error)
func (a *App) GetHypothesis(ctx context.Context, invRef, hypRef string) (Hypothesis, error)
func (a *App) ListHypotheses(ctx context.Context, invRef string) ([]Hypothesis, error)
func (a *App) UpdateHypothesis(ctx context.Context, actor Actor, invRef, hypRef string, p HypothesisPatch) (Hypothesis, error)
func (a *App) UpdateHypothesisOutcome(ctx context.Context, actor Actor, invRef, hypRef string, p HypothesisPatch) (Hypothesis, UpdateOutcome, error)
func (a *App) SetHypothesisVerification(ctx context.Context, actor Actor, invRef, hypRef string, v Verification) (Hypothesis, error)
func (a *App) SupersedeHypothesis(ctx context.Context, actor Actor, invRef, hypRef, replacementRef string) (Hypothesis, error)
func (a *App) DeleteHypothesis(ctx context.Context, actor Actor, invRef, hypRef string) error
```

- `AddHypothesis`: next H ref with links; confidence `unknown`, status `open` by default; status or verification `superseded` rejected; link rules as `AddClaim`.
- `GetHypothesis`, `ListHypotheses`: with links; every status, ordered by ref.
- `UpdateHypothesis`: patch with the `UpdateClaim` rules.
- `SetHypothesisVerification`: `superseded` is a `ValidationError`; any change on a superseded hypothesis is `ErrConflict`; same value is a no-op.
- `SupersedeHypothesis`: same rules as `SupersedeClaim`.
- `DeleteHypothesis`: same rules as `DeleteClaim` (`hypothesis.delete`).

### Questions

```go
func (a *App) AddQuestion(ctx context.Context, actor Actor, invRef string, in QuestionInput) (Question, error)
func (a *App) GetQuestion(ctx context.Context, invRef, qRef string) (Question, error)
func (a *App) ListQuestions(ctx context.Context, invRef string) ([]Question, error)
func (a *App) UpdateQuestion(ctx context.Context, actor Actor, invRef, qRef, text string) (Question, error)
func (a *App) ResolveQuestion(ctx context.Context, actor Actor, invRef, qRef, resolution string) (Question, error)
func (a *App) DeferQuestion(ctx context.Context, actor Actor, invRef, qRef, note string) (Question, error)
func (a *App) ReopenQuestion(ctx context.Context, actor Actor, invRef, qRef string) (Question, error)
func (a *App) DeleteQuestion(ctx context.Context, actor Actor, invRef, qRef string) error
```

- `AddQuestion`: next Q ref, status `open`.
- `GetQuestion`, `ListQuestions`: every status, ordered by ref.
- `UpdateQuestion`: replaces the text, status unchanged; same text is a no-op.
- `ResolveQuestion`: resolution required; status `resolved`, `resolved_at` now, defer note cleared; new text replaces it, same text is a no-op.
- `DeferQuestion`: optional note; status `deferred`, resolution and `resolved_at` cleared; same note again is a no-op.
- `ReopenQuestion`: back to `open`, clearing resolution, defer note and `resolved_at`; already open is a no-op.

Deleting (AR-10) is for mistakes; archiving stays the normal way to put an investigation away. Every `Delete*` method is permanent, writes one activity row and touches the investigation (`DeleteInvestigation` writes a global row instead). Refs are never reused: counters only grow, so a deleted `E-003` stays unassigned. The engine enforces only the guards above; asking the user to confirm is each surface's job.

### Activity

```go
func (a *App) ListActivity(ctx context.Context, f ActivityFilter) ([]Activity, error)
```

- `ListActivity`: empty `InvestigationRef` means all investigations plus global rows (not the active one). `Limit` 0 means 50, capped at 500. The order depends on `AfterID`:
  - `AfterID` 0: the newest `Limit` rows, newest first (recent activity views).
  - `AfterID > 0`: the rows with a larger id in ascending id order (oldest first), at most `Limit` of them. This is a gap-free forward cursor for incremental feeds: set `AfterID` to the largest id received and call again until fewer than `Limit` rows come back.
  - A restore replaces the activity table with the backup's rows, so ids can go down or be reused. A feed that sees the change token move and the newest id fall below its cursor must start over with `AfterID` 0.

### Active investigation

```go
func (a *App) GetActiveInvestigation(ctx context.Context) (*Investigation, error)
func (a *App) SetActiveInvestigation(ctx context.Context, actor Actor, invRef string) (Investigation, error)
func (a *App) ClearActiveInvestigation(ctx context.Context, actor Actor) error
func (a *App) ClearActiveInvestigationIf(ctx context.Context, actor Actor, invRef string) (bool, error)
func (a *App) SetGlobalActiveInvestigation(ctx context.Context, actor Actor, invRef string) (Investigation, error)
func (a *App) ActiveInvestigationInfo(ctx context.Context) (ActiveInfo, error)
func (a *App) BindDirectory(ctx context.Context, actor Actor, dir, invRef string) (DirectoryBinding, error)
func (a *App) UnbindDirectory(ctx context.Context, actor Actor, dir string) (string, error)
func (a *App) ListDirectoryBindings(ctx context.Context) ([]DirectoryBinding, error)
func (a *App) WorkDir() string
func ProjectRoot(dir string) (string, error)
```

"Active" depends on the App's working directory (`WithWorkDir`, [ADR-0015](decisions/0015-per-directory-active-investigation.md)): a directory binding of that directory or its nearest bound ancestor wins over the global setting.


- `GetActiveInvestigation`: the active investigation, or `nil, nil` when none.
- `SetActiveInvestigation`: ref required; where a directory binding applies it re-points that binding (records `active.bind`), otherwise it sets the global setting (records `active.set`); already active is a no-op; result has `is_active` true.
- `SetGlobalActiveInvestigation`: always sets the global setting (records `active.set`), even where a binding applies; the binding still wins there, so `is_active` can be false.
- `ActiveInvestigationInfo`: the effective active investigation (`nil` when none), its `scope` (`directory`, `global`, or `""`), the bound `dir` and the global `global_ref`.
- `BindDirectory`: binds `dir` (made absolute and cleaned) to `invRef`; an empty `dir` binds the project root of the working directory (`ProjectRoot`: the nearest ancestor holding `.git`, else the directory itself), a ValidationError on `dir` without one. Records `active.bind` unless already bound to it.
- `UnbindDirectory`: removes the binding of `dir`, or with an empty `dir` the binding that applies to the working directory; returns the directory it removed, `""` when none (no row). Records a global `active.unbind`.
- `ListDirectoryBindings`: every binding, ordered by directory; never nil.
- `ClearActiveInvestigation`: records a global `active.clear` row; silent no-op when none is set.
- `ClearActiveInvestigationIf`: compare-and-clear in one write transaction: clears (and records the same `active.clear` row) only while `invRef` is still the active investigation, and reports whether it cleared. Another active investigation, or none, is `false, nil` with no row. `invRef` must exist (`ErrNotFound`); blank is a ValidationError. The desktop's Active toggle uses it so a stale view never clears a choice an AI client made since.

### Search

```go
func (a *App) Search(ctx context.Context, query string, o SearchOptions) ([]SearchHit, error)
```

- `Search`: full text over investigation titles, problem statements and conclusions, evidence titles and summaries (never raw detail), claim and hypothesis statements, questions and resolutions; ranked by bm25 (lower `rank` is better, title matches weigh more). The query is user text, not FTS5 syntax: each term is quoted, the last term matches as a prefix, terms are ANDed. Empty query is a `ValidationError` on `query`; punctuation only returns `[]`. Empty `InvestigationRef` searches all; archived investigations are skipped unless `IncludeArchived` or named explicitly. `Limit` 0 means 20, maximum 100. Snippets mark matches with `**` and elide with `...`.

### Context pack

```go
func (a *App) ContextPack(ctx context.Context, invRef string, o ContextOptions) (Pack, error)
func (a *App) RenderContext(ctx context.Context, invRef, format string, o ContextOptions) (string, error)
```

- `ContextPack`: the deterministic pack (section 12) as a struct. `MaxBytes` and `RecentChanges` must be at least 0; `RecentChanges` 0 means the default 10 (there is no value for "none"; ignore `recent_changes` instead).
- `RenderContext`: the pack as text; `format` is `markdown` (or `md`) or `json`, case-insensitive; anything else is a `ValidationError` on `format`. `MaxBytes` bounds the Markdown size; `json` renders the same trimmed pack and can be larger than `MaxBytes`.

### Report

```go
func (a *App) Report(ctx context.Context, invRef string, o ReportOptions) (string, error)
func (a *App) ReportHTML(ctx context.Context, invRef string, o ReportOptions) (string, error)
```

- `Report`: a deterministic Markdown write-up of an investigation (empty `invRef` means the active one) for a pull request description, an incident document or a postmortem (section 12a). `ReportOptions.TimelineLimit` (`timeline_limit`) > 0 keeps only the newest N timeline entries; 0 keeps every change it reads; a negative value is a `ValidationError` on `timeline_limit`. The CLI `casebook report`, the MCP tool `casebook_get_report` and the desktop app's **Copy report** all print this text.
- Brief access: the leadership brief is `SharePreview` in `brief` mode (see [Reviewed sharing snapshots](#reviewed-sharing-snapshots)). `casebook report --brief` and the MCP tool `casebook_get_report` with `mode` `brief` return its Markdown with Casebook's default selection; `mode` `report` (the default) returns the complete report. Neither writes anything, and neither knows or records where the text goes next ([ADR 0024](decisions/0024-portable-output-instead-of-evernote.md)).
- `ReportHTML`: the same report as HTML (`contextpack.ReportHTML` over the `Report` text) for the desktop app's Share summary preview; same errors as `Report`. It turns only the shapes the report emits into tags (`#`, `##` and `###` headings, `-` bullets with one nested level, a leading `**bold**` run on a bullet, the `_..._` footer as emphasis, paragraphs, with the backslash before an escaped leading block marker removed) and escapes every other character as text. The golden file is `internal/contextpack/testdata/checkout_report.html`.

### Checkpoint

```go
func (a *App) Checkpoint(ctx context.Context, actor Actor, invRef, note string) (CheckpointResult, error)
func (a *App) CheckpointBatch(ctx context.Context, actor Actor, invRef string, in CheckpointInput) (CheckpointResult, error)
```

- `CheckpointBatch`: applies an optional batch of new and changed records and records a `checkpoint` row, all in one write transaction (all or nothing), and returns the refs created and changed, what changed since the previous checkpoint and open items (section 14).
- `Checkpoint`: `CheckpointBatch` with only a note.

### Portability

```go
func (a *App) ExportInvestigation(ctx context.Context, invRef string) ([]byte, error)
func (a *App) ImportInvestigation(ctx context.Context, actor Actor, data []byte) (ImportResult, error)
func (a *App) ImportInvestigationFrom(ctx context.Context, actor Actor, r io.Reader) (ImportResult, error)
```

- `ExportInvestigation`: one investigation as indented `casebook.investigation` version 2 JSON ending in a newline; writes no activity.
- `ImportInvestigation`: untrusted JSON in, everything validated before one transaction; one `import` row (section 15).
- `ImportInvestigationFrom`: the same, reading at most `MaxImportBytes` plus one byte from `r`, so an oversized or endless input is refused (ValidationError on `data`) without being held in memory. Surfaces importing from a file use it.

### Backup and restore

```go
func (a *App) Backup(ctx context.Context, destPath string) (BackupInfo, error)
func (a *App) ValidateBackup(ctx context.Context, path string) (BackupInfo, error)
func (a *App) Restore(ctx context.Context, actor Actor, srcPath string) (RestoreResult, error)
```

- `Backup`: consistent snapshot to a new 0600 file; takes no actor and writes no activity row (it changes no Casebook data).
- `ValidateBackup`: read-only check that a file is a genuine, untampered Casebook backup.
- `Restore`: validated, safety-copied, atomic in-place replacement of the whole database (section 16).

### Live refresh

```go
func (a *App) ChangeToken(ctx context.Context) (string, error)
```

- `ChangeToken`: cheap token that changes whenever the database changes, from any process (section 13).

### Status and diagnostics

```go
func (a *App) Status(ctx context.Context) (StatusInfo, error)
func (a *App) CheckIntegrity(ctx context.Context) error
```

- `Status`: database path, data directory, schema version, active ref and row counts from one snapshot (CLI `status`, doctor).
- `CheckIntegrity`: `PRAGMA integrity_check` plus `foreign_key_check`; nil when clean, else an `*IntegrityError` (wraps `ErrInternal`) listing up to 10 sanitized problems by check, table or index and row id, with the total count (section 6).

## 8. DTOs and JSON fields

Rules for every DTO (enforced by `TestDTOJSONTags` and `TestNoJSONNulls`):

- JSON tags are snake_case; the Go field name is the CamelCase form (`problem_statement` is `ProblemStatement`).
- Timestamps are strings in the fixed UTC layout `2006-01-02T15:04:05.000Z` (`app.TimeLayout`, valid RFC3339, sorts as text). An unset timestamp is `""`, never null. Inputs accept any RFC3339 timestamp and normalize it; one whose UTC year falls outside 0001 to 9999 is a `ValidationError`.
- Slices in returned DTOs are never nil: they encode as `[]`. No DTO uses maps or `time.Time`.
- Enums are strings (section 9). `id` fields are internal row ids, useful as stable keys in a UI list; address records by ref.
- `Actor` is `{"kind": ActorKind, "client": string}` everywhere it appears (`actor`, `created_by`).

### Investigation (returned by investigation methods)

| JSON | Type | Notes |
|---|---|---|
| `id` | int64 | internal id |
| `ref` | string | `INV-0001` |
| `title` | string | required, one line |
| `problem_statement` | string | the primary question |
| `status` | InvestigationStatus | |
| `conclusion` | string | `""` until set |
| `conclusion_confidence` | Confidence | `unknown` until set |
| `conclusion_rationale` | string | |
| `created_at`, `updated_at` | string | timestamps |
| `archived_at` | string | `""` unless archived |
| `archived` | bool | |
| `is_active` | bool | is the shared active investigation |
| `created_by` | Actor | |

### InvestigationSummary and RecordCounts (ListInvestigations)

| JSON | Type | Notes |
|---|---|---|
| `id`, `ref`, `title`, `status`, `conclusion_confidence` | | as Investigation |
| `created_at`, `updated_at`, `archived_at` | string | |
| `archived`, `is_active` | bool | |
| `counts` | RecordCounts | `evidence`, `claims`, `hypotheses`, `questions`, `open_questions` (ints) |

### InvestigationDetail (GetInvestigationDetail)

| JSON | Type | Notes |
|---|---|---|
| `investigation` | Investigation | |
| `evidence` | []Evidence | by ref; `raw_detail` only with `IncludeRawDetail` |
| `claims` | []Claim | by ref, with links |
| `hypotheses` | []Hypothesis | by ref, with links |
| `questions` | []Question | by ref |
| `recent_activity` | []Activity | newest first |

### Evidence

| JSON | Type | Notes |
|---|---|---|
| `id`, `ref`, `investigation_ref` | | `E-001` in `INV-0001` |
| `title`, `summary` | string | required |
| `raw_detail` | string | omitted (`omitempty`) unless requested and present |
| `has_raw_detail` | bool | whether raw detail is stored |
| `source_type` | SourceType | |
| `source_ref` | string | path, URL, ticket key, query name |
| `source_label` | string | human label for the source |
| `captured_at`, `created_at`, `updated_at` | string | |
| `created_by` | Actor | |
| `verification` | Verification | |
| `metadata` | string | compact JSON object text, or `""` |
| `imported` | bool | the row came from `ImportInvestigation` (section 15) |
| `claim_links`, `hypothesis_links` | []RecordLink | `{"ref","relation"}` of linked claims and hypotheses |

### Claim and Hypothesis

| JSON | Type | Notes |
|---|---|---|
| `id`, `ref`, `investigation_ref` | | |
| `statement` | string | required |
| `confidence` | Confidence | |
| `confidence_rationale` (Claim) / `rationale` (Hypothesis) | string | |
| `status` | ClaimStatus / HypothesisStatus | |
| `verification` | Verification | |
| `superseded_by` | string | ref of the replacement, `""` when none |
| `created_at`, `updated_at` | string | |
| `created_by` | Actor | |
| `imported` | bool | the row came from `ImportInvestigation` (section 15) |
| `links` | []EvidenceLink | by evidence ref |

EvidenceLink: `evidence_ref`, `relation`, `evidence_title` (so a surface needs no second lookup), `created_at`, `created_by`.

### Question

| JSON | Type | Notes |
|---|---|---|
| `id`, `ref`, `investigation_ref` | | |
| `question` | string | required |
| `status` | QuestionStatus | |
| `resolution` | string | set when resolved |
| `defer_note` | string | set when deferred |
| `resolved_at` | string | `""` unless resolved |
| `created_at`, `updated_at` | string | |
| `created_by` | Actor | |
| `imported` | bool | the row came from `ImportInvestigation` (section 15) |

### Activity

| JSON | Type | Notes |
|---|---|---|
| `id` | int64 | increasing; use with `ActivityFilter.AfterID` (a restore can reset it) |
| `investigation_ref` | string | `""` for global rows |
| `actor` | Actor | |
| `action` | string | section 10 |
| `target_ref` | string | record or investigation ref, may be `""` |
| `summary` | string | one line, at most 300 bytes |
| `note` | string | checkpoint note, else `""` |
| `created_at` | string | |
| `imported` | bool | a historical row copied from an imported file: its actor and summary are what the file claimed; show it as imported (section 15) |

### SearchHit

| JSON | Type | Notes |
|---|---|---|
| `investigation_ref`, `investigation_title` | string | |
| `record_type` | RecordType | |
| `record_ref` | string | the INV ref for an investigation hit |
| `snippet` | string | matches wrapped in `**`, elisions as `...` (for text output) |
| `segments` | []SnippetSegment | the same snippet as `{text, match}` pieces, never null; surfaces that highlight use these instead of parsing `**`, which stored text may contain |
| `rank` | float64 | bm25, lower is better |

### CheckpointResult and CheckpointItem

| JSON | Type | Notes |
|---|---|---|
| `activity_id` | int64 | the checkpoint row |
| `investigation_ref`, `note` | string | |
| `created` | []string | refs `CheckpointBatch` created, in the order applied; `[]` for a note-only checkpoint |
| `updated` | []string | refs it actually changed (claims, hypotheses, questions, or the INV ref for the conclusion), in order, without duplicates; no-op items are left out |
| `verification_resets` | []VerificationReset | omitted when empty: each claim or hypothesis update whose edit reset a vouched verification to `ai_generated`, as `{type, ref, previous_verification}`, in the order applied |
| `previous_checkpoint_at` | string | `""` on the first checkpoint |
| `changes` | []Activity | since the previous checkpoint, newest first, at most 50 |
| `changes_truncated` | bool | more than 50 changes exist |
| `open_questions`, `open_hypotheses`, `claims_to_review` | []CheckpointItem | by ref |

CheckpointItem: `type` (RecordType), `ref`, `text` (one line, at most 200 bytes), `status`, `confidence` and `verification` (both `""` for questions).

### StatusInfo, Counts, BackupInfo, RestoreResult, ImportResult

| DTO | JSON fields |
|---|---|
| StatusInfo | `db_path`, `data_dir`, `schema_version`, `active_investigation_ref` (`""` when none), `counts` |
| Counts | `investigations`, `evidence`, `claims`, `hypotheses`, `questions`, `claim_links`, `hypothesis_links`, `activity` |
| BackupInfo | `path` (absolute), `size_bytes`, `schema_version`, `created_at` (Backup: App clock time; ValidateBackup: file modification time), `counts` (rows inside the backup) |
| RestoreResult | `source_path`, `safety_copy_path`, `source_schema_version` (before migration), `counts` (after the restore, so `activity` includes the restore row) |
| ImportResult | `investigation_ref` (the ref it received), `title` (the imported title), `original_ref`, `ref_changed`, `counts` (inserted rows; `investigations` is 1, `activity` includes the import row) |
| ValidationError | `field`, `message` |
| IntegrityError | `problems` (sanitized lines, at most 10), `total` |

### Inputs and options

| Type | JSON fields |
|---|---|
| CreateInvestigationInput | `title` (required), `problem_statement`, `status` (optional, not `archived`), `make_active` |
| InvestigationPatch | `title`, `problem_statement`, `status` (pointers, `omitempty`; nil means unchanged) |
| ConclusionInput | `conclusion` (empty clears; then `confidence` must be empty or `unknown` and `rationale` empty), `confidence` (empty means `unknown`), `rationale` |
| EvidenceInput | `title`, `summary`, `source_type` (required), `raw_detail`, `source_ref`, `source_label`, `captured_at` (RFC3339), `verification`, `metadata` (JSON object text) |
| EvidencePatch | the EvidenceInput fields as pointers |
| GetEvidenceOptions | `include_raw_detail` |
| ClaimInput | `statement` (required), `confidence`, `confidence_rationale`, `status`, `verification`, `links` ([]EvidenceLinkInput) |
| ClaimPatch | `statement`, `confidence_rationale`, `confidence`, `status`, `verification`, `superseded_by` (pointers) |
| HypothesisInput | `statement` (required), `confidence`, `rationale`, `status`, `verification`, `links` |
| HypothesisPatch | `statement`, `rationale`, `confidence`, `status`, `verification`, `superseded_by` (pointers) |
| EvidenceLinkInput | `evidence_ref` (`E-001` or `INV-0001/E-001`), `relation` |
| QuestionInput | `question` (required) |
| DetailOptions | `activity_limit` (0 means 20, max 200), `include_raw_detail` |
| ListOptions | `include_archived`, `status` (plain string, `""` any), `limit` (0 means 100, max 1000) |
| SearchOptions | `investigation_ref` (`""` all), `include_archived`, `limit` (0 means 20, max 100) |
| ActivityFilter | `investigation_ref` (`""` all), `after_id` (0: newest first; above 0: rows after it, oldest first), `limit` (0 means 50, max 500) |
| ContextOptions | `compact`, `max_bytes`, `recent_changes` (0 means 10) |
| ReportOptions | `timeline_limit` (0 means all) |
| CheckpointInput | `note` (required), `evidence` ([]CheckpointEvidence), `claims` ([]ClaimInput), `claim_updates` ([]ClaimUpdate), `hypotheses` ([]HypothesisInput), `hypothesis_updates` ([]HypothesisUpdate), `questions` ([]QuestionInput), `question_updates` ([]QuestionUpdate), `conclusion` (*ConclusionInput, `omitempty`) |
| CheckpointEvidence | `key` (optional, 1 to 32 of `A-Z a-z 0-9 _ -`, unique in the batch), `evidence` (EvidenceInput) |
| ClaimUpdate, HypothesisUpdate | `ref`, `patch` (ClaimPatch or HypothesisPatch; at least one field) |
| QuestionUpdate | `ref`, `question` (optional new text), then at most one of `resolution` (resolve), `defer` (bool, with optional `note`) or `reopen` (bool); at least one change |

All free text is sanitized the same way (D-15): invalid UTF-8 is a `ValidationError`, CRLF becomes LF, control characters other than tab and newline (C0, DEL and C1) are dropped, invisible format characters are dropped (bidi embeddings, overrides and isolates, zero-width space, word joiner, BOM, soft hyphen, tag characters outside an emoji subdivision flag) while the joiners ZWJ and ZWNJ, the direction marks LRM, RLM and ALM, and visible script format marks are kept, noncharacters such as U+FFFE are dropped, surrounding whitespace is trimmed, required fields must be non-empty after trimming, and limits are in bytes (section 11). Single-line fields (titles, labels, refs, client names) collapse newlines and tabs into one space.

Migration 0004 applies these rules once to rows stored before the invisible-character rules existed. It runs automatically when a Casebook with schema version 4 first opens an older database (after the usual pre-upgrade backup, section 2) or restores an older backup, in the same transaction that records the version, so it runs exactly once. Every free-text column of investigations, evidence (including `raw_detail`, which stays out of the search index), claims, hypotheses, questions, link rows and activity (client names, target ref, summary and note) is passed through the same `SanitizeText` or `SanitizeLine` its writes use, without the byte limits (sanitizing only removes characters). Only values that change are rewritten. `updated_at`, status, verification and `imported` keep their values and no activity row is written: the record says what it said before, minus characters nobody could see. The search index follows through its update triggers. A required field (a title, summary, statement or question) that held nothing but dropped characters becomes `(invisible text removed)`, and such a client name becomes `unknown`. Metadata is JSON that is validated, not stripped, on write, so it is left as stored.

## 9. Enums

Every enum is a named string type with constants, `app.All<Enum>s()` (ordered list for pickers), `Valid()`, `Label()` for display and `app.Parse<Enum>(s)` (trimmed, case-insensitive; `ValidationError` on failure). There are no numeric scores anywhere.

| Enum | Values (label) |
|---|---|
| InvestigationStatus | `new` (New), `investigating` (Investigating), `blocked` (Blocked), `resolved` (Resolved), `archived` (Archived) |
| Confidence | `unknown` (Unknown), `low` (Low), `medium` (Medium), `high` (High) |
| Verification | `ai_generated` (AI generated), `human_verified` (Human verified), `source_verified` (Source verified), `disputed` (Disputed), `superseded` (Superseded). On claims and hypotheses `superseded` (and on claims `disputed`) always matches the status (section 7, ADR-0013). |
| ClaimStatus | `active` (Current), `disputed` (Disputed), `superseded` (Superseded), `rejected` (Rejected) |
| HypothesisStatus | `open` (Open), `supported` (Supported), `disproven` (Disproven), `superseded` (Superseded) |
| QuestionStatus | `open` (Open), `resolved` (Resolved), `deferred` (Deferred) |
| SourceType | `sql_query` (SQL query), `code` (Code), `file` (File), `jira` (Jira), `documentation` (Documentation), `api_response` (API response), `user_statement` (User statement), `ai_tool_output` (AI tool output), `manual_observation` (Manual observation), `external_url` (External URL), `casebook_artifact` (Casebook artifact), `evernote_note` (Evernote note). `evernote_note` stays a valid value so existing evidence keeps its type and label; nothing suggests it for new evidence since ADR 0024. |
| Relation | `supports` (Supports), `contradicts` (Contradicts) |
| ActorKind | `human` (Human), `ai` (AI), `import` (Import), `system` (System) |
| RecordType | `investigation` (Investigation), `evidence` (Evidence), `claim` (Claim), `hypothesis` (Hypothesis), `question` (Question) |

Constants follow the pattern `app.InvestigationStatusInvestigating`, `app.ConfidenceHigh`, `app.VerificationHumanVerified`, `app.ClaimStatusDisputed`, `app.HypothesisStatusDisproven`, `app.QuestionStatusDeferred`, `app.SourceTypeSQLQuery`, `app.RelationContradicts`, `app.ActorAI`, `app.RecordClaim`.

## 10. Activity actions

Activity is a simple log (ADR-0007): every mutation writes exactly one row (two for `CreateInvestigation` with `MakeActive`) in the same transaction. No-ops write none.

Summaries are one line of at most 300 bytes. They name refs and, for updates, the changed field names. A changed enum field also names its new value (`Updated hypothesis H-001: rationale, status disproven`; `Updated claim C-001: confidence high`; `Updated evidence E-002: source_type log, verification source_verified`): the status, confidence and verification of claims and hypotheses and the source type and verification of evidence. Free-text fields are named only. A verification reset by an AI or import edit is named `verification` with the reset note saying the new value. Rows written before this rule list only field names and are not rewritten. A few copy short user text, collapsed to one line:

- `investigation.create`: the investigation title.
- `evidence.add`: the evidence title.
- `question.add`, `question.update`: the question text (first 200 bytes).
- `question.resolve`, `question.defer`: the resolution or defer note (first 200 bytes).
- `checkpoint`: the note (the full note is in `note`).
- `restore`: the file names of the backup and the safety copy.
- `import`: the original and new investigation refs and record counts, plus `; N verifications reset to ai_generated` when the file claimed human or source verification (section 15).

Summaries never contain evidence summaries, raw detail or metadata, claim or hypothesis statements or rationales, conclusions or problem statements. A summary is written once: editing or clearing a record later does not rewrite earlier summaries, which stay visible in `ListActivity`, exports and context pack recent changes.

| Action | Written by | Scope |
|---|---|---|
| `investigation.create` | CreateInvestigation | investigation |
| `investigation.update` | UpdateInvestigation (including unarchiving to a status) | investigation |
| `investigation.archive` | ArchiveInvestigation, UpdateInvestigation with status archived | investigation |
| `investigation.unarchive` | UnarchiveInvestigation | investigation |
| `conclusion.update` | UpdateConclusion | investigation |
| `evidence.add` | AddEvidence | investigation |
| `evidence.update` | UpdateEvidence, SetEvidenceVerification | investigation |
| `claim.add` | AddClaim (its links are part of this row) | investigation |
| `claim.update` | UpdateClaim, SetClaimVerification | investigation |
| `claim.supersede` | SupersedeClaim, UpdateClaim with SupersededBy | investigation |
| `hypothesis.add` | AddHypothesis | investigation |
| `hypothesis.update` | UpdateHypothesis, SetHypothesisVerification | investigation |
| `hypothesis.supersede` | SupersedeHypothesis, UpdateHypothesis with SupersededBy | investigation |
| `link.add`, `link.update`, `link.remove` | LinkEvidenceTo*, UnlinkEvidenceFrom* | investigation |
| `question.add`, `question.update`, `question.resolve`, `question.defer`, `question.reopen` | the matching question methods | investigation |
| `evidence.delete`, `claim.delete`, `hypothesis.delete`, `question.delete` | DeleteEvidence, DeleteClaim, DeleteHypothesis, DeleteQuestion (`Deleted claim C-003`; evidence adds its title) | investigation |
| `investigation.delete` | DeleteInvestigation (`target_ref` names the deleted ref; summary its title and record counts) | global |
| `active.set` | SetActiveInvestigation, CreateInvestigation with MakeActive | investigation |
| `active.clear` | ClearActiveInvestigation (`target_ref` names the previous one) | global |
| `active.bind` | BindDirectory, SetActiveInvestigation or CreateInvestigation with MakeActive where a binding applies | investigation |
| `active.unbind` | UnbindDirectory (`target_ref` names the investigation it pointed at) | global |
| `checkpoint` | Checkpoint (full note in `note`) | investigation |
| `evernote.post` | Legacy: written by `ConfirmNotePosted` before ADR 0024 (`Posted the case block to Evernote note '<title>'`); no longer written, existing rows still render | investigation |
| `evernote.unlink` | Legacy: written by `ClearNoteLink` before ADR 0024; no longer written | investigation |
| `import` | ImportInvestigation (actor `{import, <importing client>}`) | investigation |
| `restore` | Restore | global |

Backup, export, reads and ChangeToken write nothing. `active.set`, `active.clear`, `active.bind`, `active.unbind`, `checkpoint` and the legacy `evernote.post` and `evernote.unlink` never appear in a context pack's recent changes or a report's timeline.

## 11. Byte limits

Limits are in bytes after sanitizing; exceeding one is a `ValidationError` naming the field and the limit. Constants are exported (`app.MaxTitleBytes`, ...).

| Field | Constant | Bytes |
|---|---|---|
| Investigation and evidence title | MaxTitleBytes | 200 |
| Problem statement | MaxProblemStatementBytes | 4000 |
| Conclusion | MaxConclusionBytes | 4000 |
| Conclusion rationale | MaxConclusionRationaleBytes | 2000 |
| Evidence summary | MaxEvidenceSummaryBytes | 4000 |
| Evidence raw detail | MaxRawDetailBytes | 65536 |
| Source ref | MaxSourceRefBytes | 2000 |
| Source label | MaxSourceLabelBytes | 200 |
| Metadata (compact JSON) | MaxMetadataBytes | 16384 |
| Claim and hypothesis statement | MaxStatementBytes | 2000 |
| Confidence rationale, hypothesis rationale | MaxRationaleBytes | 2000 |
| Question | MaxQuestionBytes | 2000 |
| Resolution | MaxResolutionBytes | 4000 |
| Defer note | MaxDeferNoteBytes | 2000 |
| Activity summary (engine generated) | MaxActivitySummaryBytes | 300 |
| Checkpoint note | MaxCheckpointNoteBytes | 1000 |
| Checkpoint batch items (not bytes: list elements plus one for the conclusion) | MaxCheckpointItems | 50 |
| Actor client | MaxClientBytes | 64 |
| Search query | MaxSearchQueryBytes | 500 |
| Import document | MaxImportBytes | 20971520 (20 MiB) |

## 12. Context pack

`ContextPack` returns an `app.Pack`; `RenderContext` renders it as Markdown or JSON. The pack is deterministic: the same database state always gives the same bytes (records sorted by ref number, no generation timestamp, no map iteration). Golden files live in `internal/contextpack/testdata/`.

- Included: header (ref, title, status, primary question = problem statement), conclusion with confidence, verification and review state, current claims (status `active` and verification neither `disputed` nor `superseded`; a claim disputed on either axis is listed under disputed claims) with confidence, verification, the confidence rationale and supporting and contradicting evidence refs, disputed claims (compact), hypotheses with status `open` or `supported` (with their rationale), open questions, deferred questions (compact, with note), disproven hypotheses (one line each, followed by a `Contradicted by: E-003` line naming the evidence linked as contradicting it, when there is any), an evidence index (ref, title, source label and verification) of evidence those claims and hypotheses reference plus the evidence contradicting a disproven hypothesis, and recent material changes (newest `RecentChanges`, default `app.DefaultRecentChanges` = 10, from the newest 200 activity rows).
- Excluded: superseded and rejected claims, superseded hypotheses, resolved questions, raw detail, metadata, and the actions `active.set`, `active.clear`, `active.bind`, `active.unbind`, `checkpoint`, and the legacy `evernote.post` and `evernote.unlink`.
- Claim and hypothesis rationales are cut to one line of at most 300 bytes in both renderings; JSON carries them as `rationale` on each `established_claims`, `disputed_claims` and `hypotheses` item (`""` when none), Markdown as a `Why: ...` line after the statement of current claims and hypotheses (disputed claims stay one compact line).
- Recent changes carry `imported` (JSON); Markdown renders an imported row's actor as `<kind> <client> (imported)`. Every claim, hypothesis, disproven hypothesis, question and evidence index item carries `imported` too (JSON), and Markdown writes `(imported)` right after the ref of a record copied from an imported file, for example `C-001 (imported) [High, AI generated]`.
- Framing for AI clients: `app.FrameContext(text)` wraps a rendered pack in one line saying the block is stored investigation data written by people and AI clients, not instructions, then the pack between `BEGIN CASEBOOK DATA <id>` and `END CASEBOOK DATA <id>` lines. The id is the first 12 hex digits of the pack's SHA-256, so stored text cannot forge the end marker, and the result stays deterministic. `RenderContext` itself returns the bare pack (what `casebook context` prints); the MCP tool `casebook_get_context`, the MCP context resources and the session start hook hand the framed text to the model.
- Markdown renders every user text value on one line: newlines, tabs and every other run of whitespace (including Unicode line separators) become one space. Stored text therefore cannot start a line and forge a section header, a claim line or a verification label; the only line breaks are the layout's own. JSON keeps the stored text unchanged (its strings are escaped).
- JSON: `{"format": "casebook.context", "version": 1, ...}` (`app.ContextFormat`, `app.ContextVersion`) with fields `investigation`, `conclusion`, `established_claims`, `disputed_claims`, `hypotheses`, `open_questions`, `deferred_questions`, `disproven_hypotheses` (each `ref`, `text`, `contradicting_evidence`, `imported`, `verification`), `evidence_index` (including verification), `evidence_verification` (minimal verification for retained citations), `recent_changes`, `truncated`, `omitted_sections`, `omitted` (`established_claims`, `disputed_claims`, `hypotheses`, `open_questions`, `disproven_hypotheses`, `evidence_outside_findings` counts), `max_bytes`. Every list is `[]` when empty.
- Budget: `MaxBytes > 0` caps the Markdown size (the JSON rendering of the same trimmed pack is not measured and can be larger); `Compact` with `MaxBytes` 0 applies `app.DefaultCompactMaxBytes` (6000, sized for session hook injection). Trimming stops at the first step that fits: (1) drop `recent_changes`, the claim and hypothesis rationales (`statement_rationales`), `evidence_index`, `deferred_questions`, then the conclusion `rationale`; (2) shorten claim and hypothesis statements and open questions to 240, then 120, then 60 bytes ending in `...` (disproven hypothesis one-liners, at most 200 bytes, are cut to 120 and 60 at the same steps; their `Contradicted by` refs are kept); (3) drop `disputed_claims`; (4) remove disproven hypotheses from the end, counting them in `omitted.disproven_hypotheses` (the section key is added to `omitted_sections` once none is left), so a large investigation keeps saying what not to retry for as long as possible; (5) remove open questions from the end, then hypotheses, then current claims, counting them in `omitted`. `truncated` reports any trimming and `omitted_sections` lists dropped sections in drop order. A list cut short ends with `(N more ... omitted)`. The header, conclusion accountability and mandatory omission notices are never cut. A budget smaller than this minimum yields a larger pack. Evidence verification stays beside each retained citation even if the evidence index is omitted; omitted disputes and evidence outside the included findings retain an explicit count.

Excerpt of the golden Markdown for the fixture:

```text
Investigation: INV-0001
Title: September cloud bill up 38%
Status: Investigating

Primary question:
Why did the cloud bill rise 38% in September when traffic was flat?

Current conclusion:
Debug logging requested for payments only reached every service and drove most of the increase.

Conclusion confidence:
High

Conclusion rationale:
The change, the pricing page and the billing export agree; the remaining 29% is still open.

Current claims:
C-001 [High, Human verified]
A shared platform default moved every service to debug logging on August 28; only payments asked for it.
Why: The chart diff and the head of platform agree.
Evidence: E-001 [AI generated], E-004 [Human verified]
C-002 [High, AI generated]
Log ingestion, billed per GB, is $96,400 of the $135,800 increase; compute and network are flat.
Why: The pricing model and the billing export agree.
Evidence: E-002 [AI generated], E-003 [AI generated]

Hypotheses:
H-001 [Medium, Open, AI generated]
The new tracing agent may also be shipping some services' logs twice.

Open questions:
Q-001
Can every service except payments go back to info level today?

Known disproven hypotheses:
H-002 Data transfer from the region move doubled the network line.
Contradicted by: E-003 [AI generated]
```

## 12a. Report

`Report` renders `contextpack.RenderReport` over one read snapshot: the investigation, its evidence (never raw detail), claims and hypotheses with links, questions and the newest 2000 activity rows. The same database state always gives the same bytes (records sorted by ref number, activity by id, no generation timestamp). The golden file is `internal/contextpack/testdata/checkout_report.md`.

Layout, in order (optional sections are left out when empty):

- `# INV-0002: <title>`, then a list with the status, the opened date and the last updated date.
- `## Question`: the problem statement (`Not recorded.` when empty).
- `## Conclusion`: the conclusion, then `Confidence: <label>. <rationale>` (`No conclusion recorded yet.` when unset).
- `## Current claims`: active claims (as in the context pack), each `- **C-001** <statement> (<confidence> confidence, <verification label>)` followed by `Why:` (the confidence rationale) and one `Supported by E-004: <title> (<source>; <verification>)` or `Contradicted by ...` line per linked evidence item; `None yet.` when empty. `<source>` is the source label (or the source type label) followed by `: <source ref>` when set.
- `## Disputed claims` (optional), in the same form.
- `## Ruled out` (optional): disproven hypotheses, then rejected claims, with their rationale and evidence lines, so the evidence that contradicted them is named.
- `## Hypotheses still open or supported` (optional).
- `## Answered questions` (optional): each resolved question with `Answer: <resolution> (<resolved date>)`.
- `## Open questions`: open questions, then deferred ones marked `(deferred: <note>)`; `None.` when empty.
- `## Evidence` (optional): every evidence item with its source, captured date, verification label and summary.
- `## Timeline (UTC)` (optional): material activity rows (not `active.set`, `active.clear` or `checkpoint`), oldest first, under one `### YYYY-MM-DD` heading per day, each `- HH:MM <actor kind> <client>: <change>` (` (imported)` after the client for an imported row). The change is written in plain words from the action and its target ref, the same phrasing as the desktop History view (`Created this case`, `Marked C-001 Human verified`, `Ruled out H-001`), with stored values as their labels; an action Casebook does not know shows the stored summary. `Earlier changes are not shown.` opens the section when `TimelineLimit` or the 2000-row read left changes out.
- A closing `_Generated by Casebook from INV-0002._` line.

Superseded claims and hypotheses are left out (their replacements carry the story; the timeline still records the change). Every user text value is rendered on one line, and a paragraph that would start with a Markdown block marker (`#`, `>`, `-`, `1.` and similar) is escaped with a backslash, so stored text cannot add headings or list items. A record copied from an imported file shows `(imported)` after its ref.

The MCP tool frames the report with `FrameContext` like the context pack. When it would not fit the 64 KiB response it retries with timeline limits 200, 100, 50, 20 and 5, then cuts the report at a line break, ends it with a line saying so and sets `truncated`.

## 13. Change token (live refresh)

`ChangeToken(ctx)` returns `"<data_version>.<max activity id>"`. `PRAGMA data_version` is read on one dedicated long-lived connection, so it moves whenever any other connection commits: another `App`, another process (MCP server, CLI, hook) or this App's own write pool. `max(activity.id)` also catches this App's writes, because every mutation writes an activity row. It runs two tiny queries and never opens a transaction.

How the desktop app uses it: call it on a timer (for example every 2 seconds) on the same `App`, keep the last value, and reload the visible data (for example `ListInvestigations` and `GetInvestigationDetail`) only when the token differs. Tokens are only comparable within one `App` instance: never persist them or compare them across processes or restarts. Equal tokens mean nothing changed; no-op calls do not move it. `TestChangeTokenCrossProcess` proves it moves on another process's write and stays equal otherwise.

## 14. Checkpoint

`CheckpointBatch(ctx, actor, invRef, in)` is the engine side of the `casebook_checkpoint` MCP tool; `Checkpoint(ctx, actor, invRef, note)` is the note-only form the CLI and hooks use.

- Input: `note` is required, at most 1000 bytes (`ValidationError` on `note`); `invRef` empty means the active investigation, resolved once for the whole batch.
- Batch: at most `MaxCheckpointItems` (50) items (`ValidationError` on `items`), applied in the order evidence, claims, claim updates, hypotheses, hypothesis updates, questions, question updates (text first, then the resolve, defer or reopen), conclusion. Each item follows the rules of its single-record method (`AddEvidence`, `AddClaim`, `UpdateClaim`, `AddHypothesis`, `UpdateHypothesis`, `AddQuestion`, `UpdateQuestion`, `ResolveQuestion`, `DeferQuestion`, `ReopenQuestion`, `UpdateConclusion`) and writes that method's one activity row; no-op items write nothing. Claims and hypotheses link evidence of the same batch with `evidence_ref` `#<key>`.
- Atomic: every input is validated before the write transaction, and the items plus the checkpoint row are written in one transaction, so a failure anywhere writes nothing (no records, no consumed refs, no activity, change token unchanged). A `ValidationError` field names the item path (`evidence[2].summary`, `claims[0].links[1].evidence_ref`, `claim_updates[0].patch.status`); `ErrNotFound` and `ErrConflict` raised while applying are prefixed with the item path (`question_updates[0]: not found: question Q-099 in INV-0001`) and still match with `errors.Is`. Verification `human_verified` in any item from a non-human actor fails the whole batch this way (section 3, ADR-0014).
- It records a `checkpoint` activity row last: the full note in `note`, the note cut to one line as `summary`. The checkpoint row does not touch the investigation's `updated_at` (batch items do) and never appears in context pack recent changes.
- It returns `created` and `updated` refs and the activity recorded since the previous checkpoint of that investigation, including this batch's rows (newest first, at most 50, `changes_truncated` when more exist; `previous_checkpoint_at` is `""` the first time), open questions, open hypotheses, and claims to review (verification `ai_generated` or status `disputed`; superseded and rejected claims skipped). Item texts are one line of at most 200 bytes.

## 15. Export format v2 and legacy import

`ExportInvestigation` writes one investigation as indented JSON (two spaces, trailing newline, no HTML escaping). Abridged shape, with `...` standing for values and repeated objects:

```json
{
  "format": "casebook.investigation",
  "version": 2,
  "exported_at": "2026-01-15T09:00:00.000Z",
  "investigation": {"ref": "INV-0001", "title": "...", "problem_statement": "...", "status": "investigating",
    "pre_archive_status": "", "conclusion": "...", "conclusion_confidence": "high", "conclusion_rationale": "...",
    "created_at": "...", "updated_at": "...", "archived_at": "", "created_by": {"kind": "human", "client": "casebook-ui"}},
  "evidence": [{"ref": "E-001", "title": "...", "summary": "...", "raw_detail": "...", "source_type": "code",
    "source_ref": "...", "source_label": "...", "captured_at": "...", "verification": "ai_generated",
    "metadata": {"affected_records": 22}, "created_at": "...", "updated_at": "...", "created_by": {...}}],
  "claims": [{"ref": "C-001", "statement": "...", "confidence": "high", "confidence_rationale": "...", "status": "active",
    "verification": "human_verified", "superseded_by": "", "created_at": "...", "updated_at": "...", "created_by": {...}}],
  "hypotheses": [{"ref": "H-001", "statement": "...", "confidence": "medium", "rationale": "", "status": "open",
    "verification": "ai_generated", "superseded_by": "", "created_at": "...", "updated_at": "...", "created_by": {...}}],
  "questions": [{"ref": "Q-001", "question": "...", "status": "open", "resolution": "", "defer_note": "",
    "resolved_at": "", "created_at": "...", "updated_at": "...", "created_by": {...}}],
  "claim_links": [{"claim_ref": "C-001", "evidence_ref": "E-001", "relation": "supports", "created_at": "...", "created_by": {...}}],
  "hypothesis_links": [{"hypothesis_ref": "H-002", "evidence_ref": "E-003", "relation": "contradicts", "created_at": "...", "created_by": {...}}],
  "activity": [{"actor": {...}, "action": "investigation.create", "target_ref": "INV-0001", "summary": "...", "note": "", "created_at": "..."}]
}
```

- Records are addressed by refs, never internal ids, ordered by ref number; links by record ref then evidence ref; activity in the order written. Every array is present.
- The output is stable ([ADR-0016](decisions/0016-stable-export-for-repo-sharing.md)): exporting an unchanged investigation gives identical bytes. `exported_at` is the newest timestamp in the document, not the wall clock, and activity rows whose action starts with `active.` (machine-local: `active.set`, `active.bind`) are left out. Raw detail is included. `metadata` is embedded as a JSON object and omitted when the evidence has none. Types: `app.ExportDocument` and the `app.Export*` structs; constants `app.ExportFormat` and `app.ExportVersion`.
- Export version 2 adds conclusion verification, selected supporting and contradicting claim refs, review timestamp, review basis and review-needed state. Import accepts legacy version 1. A file never confers a trusted review: import lowers checked conclusion verification and clears its timestamp, review basis and private comparison fingerprint. Local database backup and restore preserves these fields.
- Import treats the input as untrusted: more than 20 MiB is a `ValidationError` on `data` before decoding; malformed JSON, unknown fields and trailing data are a `ValidationError` on `data` naming only a byte offset; a different `format` or any `version` other than 1 or 2 is a `ValidationError` on `format` or `version`; every field is sanitized and validated with the app rules and a failure names an indexed path (`evidence[2].summary`, `claim_links[3].evidence_ref`); refs must have the right prefix, be unqualified and unique per kind, with numbers of at most 999999 for the investigation and 99999 for records (`ValidationError` on the ref field otherwise); links and `superseded_by` must name refs in the document; consistency rules mirror what the app methods can produce and are enforced: archived fields only when archived, `resolved_at` and `resolution` exactly when resolved, `defer_note` only when deferred, `superseded_by` exactly when the status is `superseded`, no supersede cycles. A claim or hypothesis whose status and verification disagree (files from before ADR-0013) is repaired the way migration 0002 repairs stored rows rather than refused.
- Provenance (AR-09): every inserted evidence, claim, hypothesis, question and historical activity row has `imported` true (schema version 3 column; the export format is unchanged and does not carry it). A file's `human_verified` or `source_verified` is a claim by whoever wrote the file, so it is not trusted: those records start as `ai_generated` here (other verification states are kept) until someone verifies them in this database, which does not clear `imported`. Historical activity rows keep the actor and summary the file gives, flagged `imported`, so surfaces and context packs attribute them to the file rather than to a local author. Links keep their file authors without a flag.
- Everything is inserted in one transaction or nothing is. Record refs are kept. The investigation keeps its ref when free; on collision it gets the next INV ref and `ref_changed` is true. Original authors and timestamps are preserved, so re-exporting gives the same document plus the import row (and an `exported_at` that is that row's time), except that verification the import did not trust reads `ai_generated`. One `import` activity row is added with actor `{import, <importing actor's client>}` and a summary naming the original ref (`Imported INV-0001 as INV-0002 (...)`). Imported historical activity rows keep their original `created_at` but receive new ids above every existing one, so in the global `ListActivity` feed (ordered by id) they appear as the newest rows and after any `AfterID` cursor; use `created_at` to tell their age. The active investigation is unchanged. Search finds imported text immediately.
- Same case: `same_case_refs` (omitted when empty) lists the investigations already in the database with the file's investigation `created_at` and `created_by` (the original or earlier imports of it), in ref order. The import still adds a separate copy; it never merges into or replaces them (ADR-0016).

## 16. Backup and restore

Backup (`Backup(ctx, destPath)`):

- Uses SQLite `VACUUM INTO`, a consistent snapshot of the whole database even while other connections and processes write (`TestBackupWhileOtherProcessWrites`).
- The destination is created owner-only and never overwritten: an existing file or the live database path is `ErrConflict`; an empty path or missing parent directory is a `ValidationError` on `path`.
- The new file is validated before returning. No actor, no activity row.

Validation (`ValidateBackup(ctx, path)`):

- Opens the file read-only and immutable (no `-wal` or `-shm` appear, the file is untouched) with trusted schema off.
- Requires, in this order: a schema version from 1 to this build's version; a schema identical to what the embedded migrations produce at that version (extra tables, indexes, triggers or views are refused, whatever their name; only SQLite's own automatic indexes and `sqlite_sequence`, `sqlite_stat1` and `sqlite_stat4` are ignored); `integrity_check` ok; no foreign key violations; data within Casebook limits; and the cross-row rules the engine relies on: ref numbers and counters within `app.MaxRefNum`, links and `superseded_by` within one investigation (and never a record superseding itself), `superseded_by` set exactly when the status is `superseded`, status and verification in agreement (ADR-0013; checked for schema version 2 and later, older files are repaired by the migration), `archived_at` set exactly when the investigation is archived, and question resolution, `resolved_at` and defer note only in the status that allows them. The schema is compared before `integrity_check`, so a crafted file's expressions are never evaluated.
- A missing file is `ErrNotFound`; anything else wrong, including a file that is not a database, is `ErrConflict` `not a valid Casebook backup (<short reason>)` naming tables or columns only.

Restore (`Restore(ctx, actor, srcPath)`):

1. Validate the source; any failure returns before anything is written.
2. Copy it to a private 0600 temp file in `<directory of the database>/backups`, validate the copy again, and migrate it forward if it is older.
3. Write a safety copy of the current database with `VACUUM INTO` to `<directory of the database>/backups/casebook-pre-restore-<UTC 20060102T150405Z>.db` (`-2`, `-3`, ... on collision). Its path is `RestoreResult.SafetyCopyPath`; show it to the user.
4. Inside one IMMEDIATE transaction on the live database, replace every row with the copy's rows, repair the ref counters, rebuild the search index and record a global `restore` activity row. Counter repair raises the global INV counter and every investigation's E, C, H and Q counters to at least the highest ref in use (counters only go up), so a backup whose counters lag behind its refs restores fine and later writes never collide with an existing ref.

Backups and restores carry every table, including the dormant Evernote links table (`evernote_links`, migrations 0006 and 0007) that databases from earlier versions may hold; nothing reads or writes it since ADR 0024. An investigation export (section 15) has never included it.

The file is never swapped, so other processes that hold the database open see either the old or the new contents, never a mix, and read the restored state as soon as the transaction commits (`TestRestoreSeenByOtherProcess`); their change tokens move. The temp copy is always removed. Restoring the live database onto itself is `ErrConflict`. `RestoreResult.Counts` are read after the restore, so `activity` is the backup's count plus one.

## 17. Test fixture (apptest)

`github.com/jameskbb/casebook/internal/app/apptest` is importable by tests in every phase (it imports only `internal/app`).

```go
var FixedTime = time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)
var Human = app.ActorDesktop()          // {human, casebook-ui}
var AI = app.ActorMCP("claude-code")    // {ai, claude-code}

func Open(t testing.TB, opts ...app.Option) *app.App
func Reopen(t testing.TB, path string, opts ...app.Option) *app.App
func FixedClock() func() time.Time
func StepClock(start time.Time, step time.Duration) func() time.Time
func AssertNoJSONNulls(t testing.TB, v any)
func SeedCloudSpend(t testing.TB, a *app.App) Fixture
```

- `Open(t)`: an App on a fresh `casebook.db` in `t.TempDir()` with `FixedClock` (caller options applied after, so they win), closed when the test ends. `Reopen(t, path)`: another App on an existing path (close and reopen, two Apps on one file).
- `FixedClock()` always returns `FixedTime`; `StepClock(start, step)` returns start, start+step, ... and is safe for concurrent use.
- `AssertNoJSONNulls(t, v)` fails when `v` marshals to JSON containing a null.
- `SeedCloudSpend(t, a)` builds the demo case through public methods on a fresh App and returns a `Fixture`:

| Fixture field | Value |
|---|---|
| `InvestigationRef` | `INV-0001` "September cloud bill up 38%", status investigating, active, conclusion "Debug logging requested for payments only reached every service and drove most of the increase." (high) |
| `Evidence` | `E-001` code (AI, has raw detail), `E-002` documentation (AI), `E-003` sql_query with metadata `{"affected_records":214}` (AI), `E-004` user_statement (human) |
| `Claims` | `C-001` high, supported by E-001 and E-004 (human); `C-002` high, supported by E-002 and E-003 (AI); `C-003` low, contradicted by E-003 |
| `SupersededClaimRef` | `C-003`, superseded by C-002 |
| `Hypotheses` | `H-001` medium, open; `H-002` low, contradicted by E-003 |
| `DisprovenHypothesisRef` | `H-002`, disproven |
| `Questions` | `Q-001` open: "Can every service except payments go back to info level today?" |

With `apptest.Open`'s fixed clock every timestamp is `2026-01-15T09:00:00.000Z`, so the fixture's context pack matches the golden files byte for byte.

## 18. Concurrency model

- SQLite in WAL mode (ADR-0004). Each `App` has a writer pool (one connection, `BEGIN IMMEDIATE`) and a reader pool (several connections). Reads run in snapshot transactions and never wait for writers.
- Every write goes through one helper that retries lock contention (SQLITE_BUSY and SQLITE_LOCKED, by error code) with jittered backoff up to the retry budget (default 10s), then returns `ErrBusy`. Write transactions are short and do no network or file I/O, so `ErrBusy` should be rare; when it happens nothing was written and the call can be retried.
- Any number of Apps and processes may share one file: the desktop app, several MCP servers, the CLI and hooks at once. Ref allocation happens inside the write transaction, so refs stay unique and contiguous under contention.
- Proven in tests: two Apps with 8 writer goroutines in one process (`TestTwoAppContention`), and 3 child processes plus the parent writing linked records to one investigation while the parent reads, with exact counts, contiguous refs and no raw lock text (`TestMultiProcessContention`); change token, active investigation, backup during writes and restore visibility across processes (section 13, 5 and 16).
- `go test -race` needs cgo and runs in CI (phase 5); local runs use `CGO_ENABLED=0 go test ./...`.

## Focused human review

```go
func (a *App) GetReviewFindings(ctx context.Context, invRef string) ([]ReviewFinding, error)
func (a *App) ReviewStatement(ctx context.Context, actor Actor, invRef string, in ReviewStatementInput) (ReviewFinding, error)
```

`GetReviewFindings` returns current claims and hypotheses in ref order, claims first. Superseded, rejected and disproven records remain in the case but are excluded. Each finding includes `kind`, `ref`, `statement`, `confidence`, `rationale`, `status`, `verification`, `evidence` (relationship plus the full evidence, including raw detail), `filters`, `warnings` and `basis`. Filters are `unchecked`, `source_checked`, `disputed` and `support_attention`. Support needs attention when no supporting evidence is linked or supporting evidence is disputed or superseded. These are current conditions, not a claim that a person's earlier verification changed.

`ReviewStatementInput` names `kind`, `ref`, the displayed `basis`, `verification` (`human_verified` or `disputed`), and `settle_dispute`. Only a human actor may use it. The write transaction rechecks the exact statement and linked source snapshot before using the shared statement update rule. Missing or stale basis is `ErrConflict` and writes nothing. Verifying a disputed finding requires `settle_dispute: true`. No operation checks all findings or changes skipped records. Archived cases keep their existing editing behavior.

`InvestigationDetail.review_bases` is a slice of `{ref, basis}` pairs from the same snapshot as its records. It allows an individual drawer to verify its displayed content without blessing a concurrent edit. Hashing includes full source content even when raw detail is omitted from the detail response. Review bases are ephemeral read guards, not stored review history.

## Reviewed sharing snapshots

```go
func (a *App) SharePreview(ctx context.Context, invRef string, o ShareOptions) (SharePreview, error)
```

`ShareOptions.mode` is `brief` (the default) or `report`. The leadership brief selects existing records with `claim_refs` (current, disputed and superseded claims; group `claims`), `hypothesis_refs` (open and supported hypotheses; group `hypotheses`), `question_refs` (group `questions`) and `ruled_out_refs` (rejected claims and disproven hypotheses; group `ruled_out`). Omitted or null selections choose an initial set of up to three active claims, two open or supported hypotheses, two unresolved questions and two ruled-out findings, each in ref order. Empty arrays explicitly select none. Unknown refs, refs in the wrong group and refs from another case are validation errors. Selection and rendering happen in the application layer without an AI summarizer or inferred recommendations.

Disputed evidence, current disputed claims and hypotheses, current findings linked to disputed sources or contradicting evidence, and every claim selected as the conclusion's supporting or contradicting basis are retained automatically. Rejected claims, disproven hypotheses and superseded findings remain optional unless they are part of that claim basis. This keeps retired explanations from expanding the brief into a full history. Claims that are not rejected print under `## Selected findings and what is still in doubt`; selected open or supported hypotheses print under `## Explanations still open or supported`; selected rejected claims and disproven hypotheses print under `## Ruled out`, before `## Selected questions`. Each section that has nothing selected says so. Each finding keeps its state and verification label, and its confidence except on a rejected claim or a disproven hypothesis; a verification label that repeats the status (Disputed) prints once. Evidence citations under a finding give the ref, title and verification label; the source prints once, under `## Evidence and source caveats`, with the stored summary. `## Scope and omissions` holds a fixed paragraph explaining the verification labels, then the omission notice, which names every omitted ref in order (claims, hypotheses, questions, evidence), for example `Not included: Q-001, Q-002 and E-002. Activity history is not included. Ask the person who shared this for the complete report.`, then `Disputed findings, contradicting evidence and the claims behind the answer are always included.` The closing line is `_Prepared by Casebook from the saved records of INV-0002. Casebook did not rewrite them or add to them._` The golden file is `internal/contextpack/testdata/margin_brief.md`.

`SharePreview` contains `markdown`, `html`, `snapshot_id`, `investigation_ref`, `mode`, `candidates` and `omitted`. Markdown and HTML derive from one read transaction. Candidates identify each existing selectable record, its group (`claims`, `hypotheses`, `questions` or `ruled_out`), its label (the state the brief prints, such as `Current; High confidence; AI generated`, `Disputed; Low confidence` or `Disproven; AI generated`; a question's status), whether it is selected, and whether it is required. `omitted` (`contextpack.BriefOmitted`) has the counts `claims`, `hypotheses`, `questions` and `evidence` and the lists `claim_refs`, `hypothesis_refs`, `question_refs` and `evidence_refs`, each in ref order and `[]` when empty; for `report` every count is 0. The deterministic snapshot ID changes when the underlying snapshot or rendered bytes change.

The desktop retains up to eight reviewed previews in memory. `CopySharePreview(snapshotID)` and `SaveSharePreview(snapshotID)` use exactly the retained Markdown and never reread the case, including when another AI client changes it while a save dialog is open. Save uses the native dialog, refuses Casebook data destinations, and writes privately through an atomic replacement. Unknown or expired preview IDs require a new preview and never regenerate content silently. The dialog identifies the reviewed snapshot and offers an explicit refresh before sharing new case state. Existing report APIs remain available.
