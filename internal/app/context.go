package app

import (
	"context"
	"database/sql"
	"strings"

	"github.com/jameskbb/casebook/internal/contextpack"
	"github.com/jameskbb/casebook/internal/domain"
	"github.com/jameskbb/casebook/internal/store"
)

// contextActivityRows is how many of the newest activity rows a context pack
// snapshot reads; recent changes are chosen from these.
const contextActivityRows = 200

// ContextPack builds the deterministic context pack of an investigation
// (empty invRef means the active one) from one read snapshot: investigation,
// evidence (never raw detail), claims and hypotheses with links, questions
// and the newest 200 activity rows. See contextpack.Build for what is
// included. MaxBytes and RecentChanges must be >= 0 (ValidationError on
// max_bytes or recent_changes); Compact with MaxBytes 0 applies
// DefaultCompactMaxBytes. The budget measures the Markdown rendering only.
// RecentChanges 0 means DefaultRecentChanges; there is no value for "none"
// (callers that want none ignore Pack.RecentChanges).
func (a *App) ContextPack(ctx context.Context, invRef string, o ContextOptions) (Pack, error) {
	const op = "context.pack"
	if o.MaxBytes < 0 {
		return Pack{}, a.fail(op, domain.NewValidationError("max_bytes", "must be 0 or more"))
	}
	if o.RecentChanges < 0 {
		return Pack{}, a.fail(op, domain.NewValidationError("recent_changes", "must be 0 or more"))
	}
	var snap contextpack.Snapshot
	err := a.db.WithReadTx(ctx, func(tx *sql.Tx) error {
		s, err := a.readSnapshot(ctx, tx, invRef, contextActivityRows)
		snap = s
		return err
	})
	if err != nil {
		return Pack{}, a.fail(op, err)
	}
	p := contextpack.Build(snap, o)
	a.log.Debug(op, "investigation_ref", p.Investigation.Ref, "truncated", p.Truncated)
	return p, nil
}

// RenderContext renders ContextPack as text. format is case-insensitive and
// trimmed: "markdown" or "md" gives Markdown, "json" gives indented JSON
// with "format": "casebook.context" and "version": 1. Any other format is a
// ValidationError on format. MaxBytes bounds the Markdown size: JSON renders
// the same trimmed pack and can be larger than MaxBytes.
func (a *App) RenderContext(ctx context.Context, invRef, format string, o ContextOptions) (string, error) {
	const op = "context.render"
	f := strings.ToLower(strings.TrimSpace(format))
	switch f {
	case "markdown", "md", "json":
	default:
		return "", a.fail(op, domain.NewValidationError("format", `must be "markdown" or "json"`))
	}
	p, err := a.ContextPack(ctx, invRef, o)
	if err != nil {
		return "", err
	}
	if f == "json" {
		b, err := contextpack.RenderJSON(p)
		if err != nil {
			return "", a.fail(op, err)
		}
		return string(b), nil
	}
	return contextpack.RenderMarkdown(p), nil
}

// readSnapshot reads what a context pack or report needs about one
// investigation (empty invRef means the active one) inside tx: the
// investigation, evidence (never raw detail), claims and hypotheses with
// links, questions and the newest activityRows activity rows.
func (a *App) readSnapshot(ctx context.Context, tx *sql.Tx, invRef string, activityRows int) (contextpack.Snapshot, error) {
	inv, err := a.resolveInvestigation(ctx, tx, invRef)
	if err != nil {
		return contextpack.Snapshot{}, err
	}
	s := contextpack.Snapshot{Investigation: inv}
	if s.Evidence, err = store.ListEvidence(ctx, tx, inv.ID); err != nil {
		return s, err
	}
	claims, err := store.ListStatements(ctx, tx, store.LinkClaim, inv.ID)
	if err != nil {
		return s, err
	}
	s.Claims = store.Claims(claims)
	hyps, err := store.ListStatements(ctx, tx, store.LinkHypothesis, inv.ID)
	if err != nil {
		return s, err
	}
	s.Hypotheses = store.Hypotheses(hyps)
	if s.Questions, err = store.ListQuestions(ctx, tx, inv.ID); err != nil {
		return s, err
	}
	s.Activity, err = store.ListActivity(ctx, tx,
		store.ActivityQuery{InvestigationID: inv.ID, Limit: activityRows})
	return s, err
}

// FrameContext wraps rendered context text for an AI client: a line saying
// the block is stored data written by people and AI clients, not
// instructions, then the text between BEGIN and END CASEBOOK DATA marker
// lines labelled with an id derived from the text (see contextpack.Frame).
// Every surface that hands a pack to a model frames it with this.
func FrameContext(text string) string { return contextpack.Frame(text) }

// FrameContextOverhead is the number of bytes FrameContext adds around its
// text (a trailing newline of the text is dropped inside the frame).
func FrameContextOverhead() int { return contextpack.FrameOverhead() }
