package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameskbb/casebook/internal/app"
)

// questionModeFields choose how casebook_resolve_question (and the
// checkpoint question_updates batch) changes a question: exactly one of
// resolution, defer or reopen.
type questionModeFields struct {
	Resolution string `json:"resolution,omitempty" jsonschema:"the answer; resolves the question (at most 4000 bytes)"`
	Defer      bool   `json:"defer,omitempty" jsonschema:"true parks the question for later, optionally with note"`
	Note       string `json:"note,omitempty" jsonschema:"why it is deferred (at most 2000 bytes); only with defer true"`
	Reopen     bool   `json:"reopen,omitempty" jsonschema:"true reopens a resolved or deferred question"`
}

// Question modes returned by questionModeFields.mode.
const (
	questionModeResolve = "resolve"
	questionModeDefer   = "defer"
	questionModeReopen  = "reopen"
)

// mode returns the one requested mode, or a ValidationError when none or
// several are given, or when a note comes without defer.
func (m questionModeFields) mode() (string, error) {
	var modes []string
	if strings.TrimSpace(m.Resolution) != "" {
		modes = append(modes, questionModeResolve)
	}
	if m.Defer {
		modes = append(modes, questionModeDefer)
	}
	if m.Reopen {
		modes = append(modes, questionModeReopen)
	}
	if len(modes) != 1 {
		return "", app.NewValidationError("resolution", "pass exactly one of resolution, defer: true (optional note) or reopen: true")
	}
	if !m.Defer && strings.TrimSpace(m.Note) != "" {
		return "", app.NewValidationError("note", "note is only used with defer: true")
	}
	return modes[0], nil
}

// addQuestion adds an open question.
func addQuestion(ctx context.Context, a *app.App, actor app.Actor, invRef, text string) (mutationResult[questionRecord], error) {
	q, err := a.AddQuestion(ctx, actor, invRef, app.QuestionInput{Question: text})
	if err != nil {
		return mutationResult[questionRecord]{}, err
	}
	return mutationResult[questionRecord]{
		Ref:              q.Ref,
		InvestigationRef: q.InvestigationRef,
		Action:           "created",
		Changed:          []string{},
		Record:           questionRecordOf(q, false),
	}, nil
}

var questionFieldOrder = []string{"status", "resolution", "defer_note"}

func questionFields(q app.Question) map[string]string {
	return map[string]string{
		"status":     string(q.Status),
		"resolution": q.Resolution,
		"defer_note": q.DeferNote,
	}
}

// applyQuestionUpdate resolves, defers or reopens a question. It reads the
// question first and writes by its canonical refs, then reports resolved,
// deferred, reopened or unchanged with the changed field names.
func applyQuestionUpdate(ctx context.Context, a *app.App, actor app.Actor, invRef, ref string, m questionModeFields) (mutationResult[questionRecord], error) {
	var zero mutationResult[questionRecord]
	mode, err := m.mode()
	if err != nil {
		return zero, err
	}
	before, err := a.GetQuestion(ctx, invRef, ref)
	if err != nil {
		return zero, err
	}
	var after app.Question
	action := ""
	switch mode {
	case questionModeResolve:
		after, err = a.ResolveQuestion(ctx, actor, before.InvestigationRef, before.Ref, m.Resolution)
		action = "resolved"
	case questionModeDefer:
		after, err = a.DeferQuestion(ctx, actor, before.InvestigationRef, before.Ref, m.Note)
		action = "deferred"
	default:
		after, err = a.ReopenQuestion(ctx, actor, before.InvestigationRef, before.Ref)
		action = "reopened"
	}
	if err != nil {
		return zero, err
	}
	changed := changedFields(questionFieldOrder, questionFields(before), questionFields(after))
	if len(changed) == 0 {
		action = "unchanged"
	}
	return mutationResult[questionRecord]{
		Ref:              after.Ref,
		InvestigationRef: after.InvestigationRef,
		Action:           action,
		Changed:          changed,
		Record:           questionRecordOf(after, false),
	}, nil
}

// casebook_add_question

type addQuestionInput struct {
	InvestigationRef string `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	Question         string `json:"question" jsonschema:"the open question, one sentence (at most 2000 bytes)"`
}

const addQuestionDescription = `Record an open question the case (an investigation record) still has to answer, so it survives this session and any client can pick it up: something to check, a missing fact, a decision to ask the user about. ` +
	`Open questions appear in the context pack until resolved or deferred with casebook_resolve_question. ` +
	`Returns the new Q ref (Q-001 style) and a compact record with its status; the question text is not echoed.`

// casebook_resolve_question

type resolveQuestionInput struct {
	InvestigationRef string `json:"investigation_ref,omitempty" jsonschema:"investigation ref such as INV-0001; omit to use the active investigation"`
	QuestionRef      string `json:"question_ref" jsonschema:"question ref such as Q-001 (case-insensitive), or qualified INV-0001/Q-001"`
	questionModeFields
}

const resolveQuestionDescription = `Close out or revisit an open question. Pass exactly one mode: resolution with the answer (status resolved; a new answer replaces the old one), defer: true with an optional note saying why it is parked (status deferred), or reopen: true (back to open, clearing the answer and note). ` +
	`No mode, several modes, or a note without defer is an error. ` +
	`` +
	`Returns action resolved, deferred, reopened or unchanged with the changed fields (status, resolution, defer_note) and a compact record; the text is not echoed.`

func init() {
	registerTool("casebook_add_question", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_add_question",
			Title:       "Add question",
			Description: addQuestionDescription,
			Annotations: mutatingTool(false),
		}, h.addQuestion)
	})
	registerTool("casebook_resolve_question", func(s *mcp.Server, h *handlers) {
		addTool(s, h, &mcp.Tool{
			Name:        "casebook_resolve_question",
			Title:       "Resolve, defer or reopen question",
			Description: resolveQuestionDescription,
			Annotations: mutatingTool(true),
		}, h.resolveQuestion)
	})
}

func (h *handlers) addQuestion(ctx context.Context, c call, in addQuestionInput) (mutationResult[questionRecord], string, error) {
	out, err := addQuestion(ctx, h.app, c.Actor, in.InvestigationRef, in.Question)
	if err != nil {
		return out, "", err
	}
	return out, fmt.Sprintf("Added question %s to %s.", out.Ref, out.InvestigationRef), nil
}

func (h *handlers) resolveQuestion(ctx context.Context, c call, in resolveQuestionInput) (mutationResult[questionRecord], string, error) {
	out, err := applyQuestionUpdate(ctx, h.app, c.Actor, in.InvestigationRef, in.QuestionRef, in.questionModeFields)
	if err != nil {
		return out, "", err
	}
	switch out.Action {
	case "resolved":
		return out, fmt.Sprintf("Resolved %s.", out.Ref), nil
	case "deferred":
		return out, fmt.Sprintf("Deferred %s.", out.Ref), nil
	case "reopened":
		return out, fmt.Sprintf("Reopened %s.", out.Ref), nil
	default:
		return out, fmt.Sprintf("No change to %s.", out.Ref), nil
	}
}
