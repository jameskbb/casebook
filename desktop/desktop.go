package main

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/jameskbb/casebook/internal/app"
	"github.com/jameskbb/casebook/internal/update"
	"github.com/jameskbb/casebook/internal/version"
)

// Desktop is the Go side of the Wails bindings (D-05). Every exported method
// is callable from the page: each one is a thin call into internal/app with
// the desktop actor (D-04) and errors mapped to the D-05 JSON shape. No
// method accepts an actor, and no method accepts a file path from
// JavaScript except RestoreBackup, which only accepts the path pinned by
// PickBackupToRestore (D-07).
type Desktop struct {
	ctx     context.Context
	app     *app.App
	openErr error
	dbPath  string
	native  Native
	actor   app.Actor
	now     func() time.Time
	// explicitDB is true when the database is not the per-user default
	// (--data-dir, CASEBOOK_DB or CASEBOOK_HOME), so the Connect AI clients
	// snippets pass --db to point clients at the same file.
	explicitDB bool
	// clients replaces the environment lookups of ClientConnections in
	// tests; nil means the process environment.
	clients *clientSeams
	// checkUpdate is the update check (update.Check); tests replace it so
	// no test reaches the network.
	checkUpdate func(ctx context.Context, o update.Options) (update.Result, error)

	mu             sync.Mutex
	pendingRestore string
	reviewedShares []app.SharePreview
	// lastRelease is the release page pinned by the last
	// successful CheckForUpdates, for OpenReleasePage.
	lastRelease pinnedRelease
}

// newDesktop builds the bound struct. a is nil when the database failed to
// open; openErr then says why and every data method returns an internal
// error instead of panicking. n is the native seam (nil means the Wails
// runtime).
func newDesktop(a *app.App, openErr error, dbPath string, n Native) *Desktop {
	if n == nil {
		n = &wailsNative{}
	}
	return &Desktop{
		ctx:     context.Background(),
		app:     a,
		openErr: openErr,
		dbPath:  dbPath,
		native:  n,
		actor:   app.ActorDesktop(),
		now:     time.Now,

		checkUpdate: update.Check,
	}
}

// startup is the Wails OnStartup hook. It keeps the runtime ctx for engine
// calls and for the native runtime functions.
func (d *Desktop) startup(ctx context.Context) {
	d.ctx = ctx
	if w, ok := d.native.(*wailsNative); ok {
		w.ctx = ctx
	}
}

// shutdown is the Wails OnShutdown hook: it closes the engine.
func (d *Desktop) shutdown(context.Context) {
	if d.app != nil {
		_ = d.app.Close()
	}
}

// ready reports whether the engine is open; every data binding calls it
// first.
func (d *Desktop) ready() error {
	d.mu.Lock()
	open := d.app != nil
	d.mu.Unlock()
	if !open {
		return errNotOpen
	}
	return nil
}

// StartupState tells the UI whether the database opened, and why not.
func (d *Desktop) StartupState() StartupInfo {
	d.mu.Lock()
	err := d.openErr
	d.mu.Unlock()
	return startupInfo(err, d.dbPath)
}

// DataLocation is the absolute path of the database file (sidebar footer).
func (d *Desktop) DataLocation() string {
	return d.dbPath
}

// AppVersion is the build version, the same value `casebook version`
// prints (both executables are stamped through internal/version).
func (d *Desktop) AppVersion() string {
	return version.String()
}

// Quit closes the app (the Close Casebook button on the startup error
// screen).
func (d *Desktop) Quit() {
	d.native.Quit()
}

// ChangeToken returns the live refresh token (D-10). Tokens compare only
// within this process; equal means nothing changed.
func (d *Desktop) ChangeToken() (string, error) {
	if err := d.ready(); err != nil {
		return "", err
	}
	tok, err := d.app.ChangeToken(d.ctx)
	return tok, mapErr(err)
}

// ListInvestigations lists investigation summaries, newest updated first.
func (d *Desktop) ListInvestigations(o app.ListOptions) ([]app.InvestigationSummary, error) {
	if err := d.ready(); err != nil {
		return []app.InvestigationSummary{}, err
	}
	list, err := d.app.ListInvestigations(d.ctx, o)
	if err != nil {
		return []app.InvestigationSummary{}, mapErr(err)
	}
	return list, nil
}

// GetInvestigationDetail loads everything the detail view shows.
func (d *Desktop) GetInvestigationDetail(invRef string, o app.DetailOptions) (app.InvestigationDetail, error) {
	if err := d.ready(); err != nil {
		return app.InvestigationDetail{}, err
	}
	det, err := d.app.GetInvestigationDetail(d.ctx, invRef, o)
	return det, mapErr(err)
}

// CreateInvestigation creates an investigation as the desktop user.
func (d *Desktop) CreateInvestigation(in app.CreateInvestigationInput) (app.Investigation, error) {
	if err := d.ready(); err != nil {
		return app.Investigation{}, err
	}
	inv, err := d.app.CreateInvestigation(d.ctx, d.actor, in)
	return inv, mapErr(err)
}

// copyContextMaxBytes is the Copy context budget: the same 24,000 bytes an
// MCP client gets from casebook_get_context by default (AR-14), so a pasted
// pack fits a chat the way the AI's own pack does. The pack says what it
// trimmed.
const copyContextMaxBytes = 24000

// CopyContext renders the Markdown context pack of an investigation, framed
// by app.FrameContext, and writes it to the clipboard (UI-19). People paste
// it into AI chats, so it carries the same data-not-instructions line and
// BEGIN and END CASEBOOK DATA markers as casebook_get_context: the bytes
// match that tool's context for the same pack. A clipboard failure is kind
// "other" on field "clipboard".
func (d *Desktop) CopyContext(invRef string) error {
	return d.copyContext(invRef, true)
}

// CopyContextPlain copies the same pack without the frame, for pasting into
// notes, tickets or documents rather than an AI chat (the More menu's Copy
// as plain Markdown).
func (d *Desktop) CopyContextPlain(invRef string) error {
	return d.copyContext(invRef, false)
}

// copyContext renders the pack within copyContextMaxBytes (the frame comes on
// top, as in casebook_get_context) and copies it, framed or not.
func (d *Desktop) copyContext(invRef string, framed bool) error {
	if err := d.ready(); err != nil {
		return err
	}
	text, err := d.app.RenderContext(d.ctx, invRef, "markdown", app.ContextOptions{MaxBytes: copyContextMaxBytes})
	if err != nil {
		return mapErr(err)
	}
	if framed {
		text = app.FrameContext(text)
	}
	return nativeErr("clipboard", d.native.SetClipboard(text))
}

// CopyReport renders the Markdown report of an investigation (app.Report,
// the same text casebook report prints) and writes it to the clipboard
// unframed: it is meant for pull requests, incident documents and
// postmortems, not AI chats. A clipboard failure is kind "other" on field
// "clipboard".
func (d *Desktop) CopyReport(invRef string) error {
	if err := d.ready(); err != nil {
		return err
	}
	text, err := d.app.Report(d.ctx, invRef, app.ReportOptions{})
	if err != nil {
		return mapErr(err)
	}
	return nativeErr("clipboard", d.native.SetClipboard(text))
}

// ReportPreview is the Share summary dialog's content: the report as
// Markdown (what Copy puts on the clipboard) and as HTML (what the sheet
// shows). Neither is framed; it is for a person, not an AI chat.
type ReportPreview struct {
	Markdown string `json:"markdown"`
	HTML     string `json:"html"`
}

// ReportPreview renders the report of an investigation (app.Report) as
// Markdown and as HTML from one app snapshot for the Share summary dialog.
func (d *Desktop) ReportPreview(invRef string) (ReportPreview, error) {
	if err := d.ready(); err != nil {
		return ReportPreview{}, err
	}
	p, err := d.app.SharePreview(d.ctx, invRef, app.ShareOptions{Mode: "report"})
	if err != nil {
		return ReportPreview{}, mapErr(err)
	}
	return ReportPreview{Markdown: p.Markdown, HTML: p.HTML}, nil
}

// UpdateInvestigation patches the title, problem statement or status.
func (d *Desktop) UpdateInvestigation(invRef string, p app.InvestigationPatch) (app.Investigation, error) {
	if err := d.ready(); err != nil {
		return app.Investigation{}, err
	}
	inv, err := d.app.UpdateInvestigation(d.ctx, d.actor, invRef, p)
	return inv, mapErr(err)
}

// ArchiveInvestigation archives an investigation (nothing is deleted).
func (d *Desktop) ArchiveInvestigation(invRef string) (app.Investigation, error) {
	if err := d.ready(); err != nil {
		return app.Investigation{}, err
	}
	inv, err := d.app.ArchiveInvestigation(d.ctx, d.actor, invRef)
	return inv, mapErr(err)
}

// UnarchiveInvestigation restores an archived investigation's prior status.
func (d *Desktop) UnarchiveInvestigation(invRef string) (app.Investigation, error) {
	if err := d.ready(); err != nil {
		return app.Investigation{}, err
	}
	inv, err := d.app.UnarchiveInvestigation(d.ctx, d.actor, invRef)
	return inv, mapErr(err)
}

// UpdateConclusion replaces the conclusion, its confidence and rationale.
func (d *Desktop) UpdateConclusion(invRef string, in app.ConclusionInput) (app.Investigation, error) {
	if err := d.ready(); err != nil {
		return app.Investigation{}, err
	}
	inv, err := d.app.UpdateConclusion(d.ctx, d.actor, invRef, in)
	return inv, mapErr(err)
}

// SetActiveInvestigation makes an investigation the shared active one.
func (d *Desktop) SetActiveInvestigation(invRef string) (app.Investigation, error) {
	if err := d.ready(); err != nil {
		return app.Investigation{}, err
	}
	inv, err := d.app.SetActiveInvestigation(d.ctx, d.actor, invRef)
	return inv, mapErr(err)
}

// ClearActiveInvestigation unsets the shared active investigation, but only
// while it is still invRef: a stale "Active" button must not clear a choice
// an AI client or the CLI made since the page last loaded. It reports
// whether it cleared anything. The engine checks and clears in one write
// transaction (ClearActiveInvestigationIf).
func (d *Desktop) ClearActiveInvestigation(invRef string) (bool, error) {
	if err := d.ready(); err != nil {
		return false, err
	}
	if strings.TrimSpace(invRef) == "" {
		return false, nil
	}
	cleared, err := d.app.ClearActiveInvestigationIf(d.ctx, d.actor, invRef)
	return cleared, mapErr(err)
}

// Search runs a full-text search; each hit carries its snippet as
// segments with the matches flagged (the page highlights those).
func (d *Desktop) Search(query string, o app.SearchOptions) ([]app.SearchHit, error) {
	if err := d.ready(); err != nil {
		return []app.SearchHit{}, err
	}
	hits, err := d.app.Search(d.ctx, query, o)
	if err != nil {
		return []app.SearchHit{}, mapErr(err)
	}
	return hits, nil
}

// AddEvidence adds evidence to an investigation.
func (d *Desktop) AddEvidence(invRef string, in app.EvidenceInput) (app.Evidence, error) {
	if err := d.ready(); err != nil {
		return app.Evidence{}, err
	}
	ev, err := d.app.AddEvidence(d.ctx, d.actor, invRef, in)
	return ev, mapErr(err)
}

// GetEvidence loads one evidence, with raw detail when asked.
func (d *Desktop) GetEvidence(invRef, evidenceRef string, o app.GetEvidenceOptions) (app.Evidence, error) {
	if err := d.ready(); err != nil {
		return app.Evidence{}, err
	}
	ev, err := d.app.GetEvidence(d.ctx, invRef, evidenceRef, o)
	return ev, mapErr(err)
}

// UpdateEvidence patches an evidence.
func (d *Desktop) UpdateEvidence(invRef, evidenceRef string, p app.EvidencePatch) (app.Evidence, error) {
	if err := d.ready(); err != nil {
		return app.Evidence{}, err
	}
	ev, err := d.app.UpdateEvidence(d.ctx, d.actor, invRef, evidenceRef, p)
	return ev, mapErr(err)
}

// SetEvidenceVerification sets an evidence's verification.
func (d *Desktop) SetEvidenceVerification(invRef, evidenceRef string, v app.Verification) (app.Evidence, error) {
	if err := d.ready(); err != nil {
		return app.Evidence{}, err
	}
	ev, err := d.app.SetEvidenceVerification(d.ctx, d.actor, invRef, evidenceRef, v)
	return ev, mapErr(err)
}

// AddClaim adds a claim with its evidence links.
func (d *Desktop) AddClaim(invRef string, in app.ClaimInput) (app.Claim, error) {
	if err := d.ready(); err != nil {
		return app.Claim{}, err
	}
	c, err := d.app.AddClaim(d.ctx, d.actor, invRef, in)
	return c, mapErr(err)
}

// GetClaim loads one claim with its links.
func (d *Desktop) GetClaim(invRef, claimRef string) (app.Claim, error) {
	if err := d.ready(); err != nil {
		return app.Claim{}, err
	}
	c, err := d.app.GetClaim(d.ctx, invRef, claimRef)
	return c, mapErr(err)
}

// UpdateClaim patches a claim.
func (d *Desktop) UpdateClaim(invRef, claimRef string, p app.ClaimPatch) (app.Claim, error) {
	if err := d.ready(); err != nil {
		return app.Claim{}, err
	}
	c, err := d.app.UpdateClaim(d.ctx, d.actor, invRef, claimRef, p)
	return c, mapErr(err)
}

// SetClaimVerification sets a claim's verification.
func (d *Desktop) SetClaimVerification(invRef, claimRef string, v app.Verification) (app.Claim, error) {
	if err := d.ready(); err != nil {
		return app.Claim{}, err
	}
	c, err := d.app.SetClaimVerification(d.ctx, d.actor, invRef, claimRef, v)
	return c, mapErr(err)
}

// SupersedeClaim marks a claim superseded by another claim.
func (d *Desktop) SupersedeClaim(invRef, claimRef, replacementRef string) (app.Claim, error) {
	if err := d.ready(); err != nil {
		return app.Claim{}, err
	}
	c, err := d.app.SupersedeClaim(d.ctx, d.actor, invRef, claimRef, replacementRef)
	return c, mapErr(err)
}

// LinkEvidenceToClaim links evidence to a claim, or changes the relation.
func (d *Desktop) LinkEvidenceToClaim(invRef, claimRef, evidenceRef string, rel app.Relation) (app.Claim, error) {
	if err := d.ready(); err != nil {
		return app.Claim{}, err
	}
	c, err := d.app.LinkEvidenceToClaim(d.ctx, d.actor, invRef, claimRef, evidenceRef, rel)
	return c, mapErr(err)
}

// UnlinkEvidenceFromClaim removes a claim's evidence link.
func (d *Desktop) UnlinkEvidenceFromClaim(invRef, claimRef, evidenceRef string) (app.Claim, error) {
	if err := d.ready(); err != nil {
		return app.Claim{}, err
	}
	c, err := d.app.UnlinkEvidenceFromClaim(d.ctx, d.actor, invRef, claimRef, evidenceRef)
	return c, mapErr(err)
}

// LinkEvidenceToHypothesis links evidence to a hypothesis, or changes the
// relation.
func (d *Desktop) LinkEvidenceToHypothesis(invRef, hypRef, evidenceRef string, rel app.Relation) (app.Hypothesis, error) {
	if err := d.ready(); err != nil {
		return app.Hypothesis{}, err
	}
	h, err := d.app.LinkEvidenceToHypothesis(d.ctx, d.actor, invRef, hypRef, evidenceRef, rel)
	return h, mapErr(err)
}

// UnlinkEvidenceFromHypothesis removes a hypothesis's evidence link.
func (d *Desktop) UnlinkEvidenceFromHypothesis(invRef, hypRef, evidenceRef string) (app.Hypothesis, error) {
	if err := d.ready(); err != nil {
		return app.Hypothesis{}, err
	}
	h, err := d.app.UnlinkEvidenceFromHypothesis(d.ctx, d.actor, invRef, hypRef, evidenceRef)
	return h, mapErr(err)
}

// AddHypothesis adds a hypothesis with its evidence links.
func (d *Desktop) AddHypothesis(invRef string, in app.HypothesisInput) (app.Hypothesis, error) {
	if err := d.ready(); err != nil {
		return app.Hypothesis{}, err
	}
	h, err := d.app.AddHypothesis(d.ctx, d.actor, invRef, in)
	return h, mapErr(err)
}

// GetHypothesis loads one hypothesis with its links.
func (d *Desktop) GetHypothesis(invRef, hypRef string) (app.Hypothesis, error) {
	if err := d.ready(); err != nil {
		return app.Hypothesis{}, err
	}
	h, err := d.app.GetHypothesis(d.ctx, invRef, hypRef)
	return h, mapErr(err)
}

// UpdateHypothesis patches a hypothesis.
func (d *Desktop) UpdateHypothesis(invRef, hypRef string, p app.HypothesisPatch) (app.Hypothesis, error) {
	if err := d.ready(); err != nil {
		return app.Hypothesis{}, err
	}
	h, err := d.app.UpdateHypothesis(d.ctx, d.actor, invRef, hypRef, p)
	return h, mapErr(err)
}

// SetHypothesisVerification sets a hypothesis's verification.
func (d *Desktop) SetHypothesisVerification(invRef, hypRef string, v app.Verification) (app.Hypothesis, error) {
	if err := d.ready(); err != nil {
		return app.Hypothesis{}, err
	}
	h, err := d.app.SetHypothesisVerification(d.ctx, d.actor, invRef, hypRef, v)
	return h, mapErr(err)
}

// SupersedeHypothesis marks a hypothesis superseded by another hypothesis.
func (d *Desktop) SupersedeHypothesis(invRef, hypRef, replacementRef string) (app.Hypothesis, error) {
	if err := d.ready(); err != nil {
		return app.Hypothesis{}, err
	}
	h, err := d.app.SupersedeHypothesis(d.ctx, d.actor, invRef, hypRef, replacementRef)
	return h, mapErr(err)
}

// AddQuestion adds an open question.
func (d *Desktop) AddQuestion(invRef string, in app.QuestionInput) (app.Question, error) {
	if err := d.ready(); err != nil {
		return app.Question{}, err
	}
	q, err := d.app.AddQuestion(d.ctx, d.actor, invRef, in)
	return q, mapErr(err)
}

// UpdateQuestion rewords a question.
func (d *Desktop) UpdateQuestion(invRef, qRef, text string) (app.Question, error) {
	if err := d.ready(); err != nil {
		return app.Question{}, err
	}
	q, err := d.app.UpdateQuestion(d.ctx, d.actor, invRef, qRef, text)
	return q, mapErr(err)
}

// ResolveQuestion resolves a question with its answer.
func (d *Desktop) ResolveQuestion(invRef, qRef, resolution string) (app.Question, error) {
	if err := d.ready(); err != nil {
		return app.Question{}, err
	}
	q, err := d.app.ResolveQuestion(d.ctx, d.actor, invRef, qRef, resolution)
	return q, mapErr(err)
}

// DeferQuestion defers a question with an optional note.
func (d *Desktop) DeferQuestion(invRef, qRef, note string) (app.Question, error) {
	if err := d.ready(); err != nil {
		return app.Question{}, err
	}
	q, err := d.app.DeferQuestion(d.ctx, d.actor, invRef, qRef, note)
	return q, mapErr(err)
}

// ReopenQuestion reopens a resolved or deferred question.
func (d *Desktop) ReopenQuestion(invRef, qRef string) (app.Question, error) {
	if err := d.ready(); err != nil {
		return app.Question{}, err
	}
	q, err := d.app.ReopenQuestion(d.ctx, d.actor, invRef, qRef)
	return q, mapErr(err)
}

// maxCopyTextBytes caps CopyText ("Copy path" actions); context packs go
// through CopyContext instead.
const maxCopyTextBytes = 4096

// CopyText writes a short text (a file path) to the clipboard.
func (d *Desktop) CopyText(text string) error {
	if err := d.ready(); err != nil {
		return err
	}
	if len(text) > maxCopyTextBytes {
		return mapErr(app.NewValidationError("text", "must be at most %d bytes", maxCopyTextBytes))
	}
	return nativeErr("clipboard", d.native.SetClipboard(text))
}

// SetWindowTheme matches the native window frame to the UI theme: system,
// light or dark. It touches no data, so it also works on the startup error
// screen when the database failed to open.
func (d *Desktop) SetWindowTheme(theme string) error {
	switch theme {
	case "system", "light", "dark":
		d.native.SetTheme(theme)
		return nil
	}
	return mapErr(app.NewValidationError("theme", "must be system, light or dark"))
}

// DeleteInvestigation permanently deletes an archived investigation and
// everything in it (AR-10). The page confirms first; a live investigation is
// a conflict (archive it first).
func (d *Desktop) DeleteInvestigation(invRef string) error {
	if err := d.ready(); err != nil {
		return err
	}
	return mapErr(d.app.DeleteInvestigation(d.ctx, d.actor, invRef))
}

// DeleteEvidence permanently deletes one evidence; a conflict while a claim
// or hypothesis links it.
func (d *Desktop) DeleteEvidence(invRef, evidenceRef string) error {
	if err := d.ready(); err != nil {
		return err
	}
	return mapErr(d.app.DeleteEvidence(d.ctx, d.actor, invRef, evidenceRef))
}

// DeleteClaim permanently deletes one claim and its evidence links; a
// conflict while another claim is superseded by it.
func (d *Desktop) DeleteClaim(invRef, claimRef string) error {
	if err := d.ready(); err != nil {
		return err
	}
	return mapErr(d.app.DeleteClaim(d.ctx, d.actor, invRef, claimRef))
}

// DeleteHypothesis permanently deletes one hypothesis and its evidence
// links; a conflict while another hypothesis is superseded by it.
func (d *Desktop) DeleteHypothesis(invRef, hypRef string) error {
	if err := d.ready(); err != nil {
		return err
	}
	return mapErr(d.app.DeleteHypothesis(d.ctx, d.actor, invRef, hypRef))
}

// DeleteQuestion permanently deletes one question.
func (d *Desktop) DeleteQuestion(invRef, qRef string) error {
	if err := d.ready(); err != nil {
		return err
	}
	return mapErr(d.app.DeleteQuestion(d.ctx, d.actor, invRef, qRef))
}
