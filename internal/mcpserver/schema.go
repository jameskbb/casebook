package mcpserver

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/jameskbb/casebook/internal/app"
)

// contextFormat is the casebook_get_context output format.
type contextFormat string

const (
	contextFormatMarkdown contextFormat = "markdown"
	contextFormatJSON     contextFormat = "json"
)

// reportMode is the casebook_get_report output: the complete report or the
// leadership brief (the two formats of the desktop Share summary).
type reportMode string

const (
	reportModeReport reportMode = "report"
	reportModeBrief  reportMode = "brief"
)

// investigationSection names a list casebook_get_investigation can return.
type investigationSection string

const (
	sectionClaims     investigationSection = "claims"
	sectionHypotheses investigationSection = "hypotheses"
	sectionQuestions  investigationSection = "questions"
	sectionEvidence   investigationSection = "evidence"
	sectionActivity   investigationSection = "activity"
)

func allInvestigationSections() []investigationSection {
	return []investigationSection{sectionClaims, sectionHypotheses, sectionQuestions, sectionEvidence, sectionActivity}
}

// statementVerification is the verification a tool may write on a claim or
// hypothesis: superseded is left out because only superseding
// (superseded_by) sets it (ADR-0013), and human_verified because only a
// person can set it (ADR-0014; the engine refuses it from an AI actor).
type statementVerification string

func statementVerifications() []statementVerification {
	var out []statementVerification
	for _, v := range app.AllVerifications() {
		if v != app.VerificationSuperseded && v != app.VerificationHumanVerified {
			out = append(out, statementVerification(v))
		}
	}
	return out
}

// evidenceVerification is the verification a tool may write on evidence:
// the whole enum except human_verified, which only a person can set
// (ADR-0014). Evidence has no status, so superseded stays a free choice.
type evidenceVerification string

func evidenceVerifications() []evidenceVerification {
	var out []evidenceVerification
	for _, v := range app.AllVerifications() {
		if v != app.VerificationHumanVerified {
			out = append(out, evidenceVerification(v))
		}
	}
	return out
}

// verificationPtr converts an optional statement verification.
func verificationPtr(v *statementVerification) *app.Verification {
	if v == nil {
		return nil
	}
	out := app.Verification(*v)
	return &out
}

// enumValues maps every enum type that can appear in a tool input to its
// allowed values, in picker order.
var enumValues = map[reflect.Type][]any{
	reflect.TypeFor[app.InvestigationStatus](): anyValues(app.AllInvestigationStatuses()),
	reflect.TypeFor[app.Confidence]():          anyValues(app.AllConfidences()),
	reflect.TypeFor[app.Verification]():        anyValues(app.AllVerifications()),
	reflect.TypeFor[app.ClaimStatus]():         anyValues(app.AllClaimStatuses()),
	reflect.TypeFor[app.HypothesisStatus]():    anyValues(app.AllHypothesisStatuses()),
	reflect.TypeFor[app.QuestionStatus]():      anyValues(app.AllQuestionStatuses()),
	reflect.TypeFor[app.SourceType]():          anyValues(app.AllSourceTypes()),
	reflect.TypeFor[app.Relation]():            anyValues(app.AllRelations()),
	reflect.TypeFor[app.RecordType]():          anyValues(app.AllRecordTypes()),
	reflect.TypeFor[statementVerification]():   anyValues(statementVerifications()),
	reflect.TypeFor[evidenceVerification]():    anyValues(evidenceVerifications()),
	reflect.TypeFor[contextFormat]():           {string(contextFormatMarkdown), string(contextFormatJSON)},
	reflect.TypeFor[investigationSection]():    anyValues(allInvestigationSections()),
	reflect.TypeFor[reportMode]():              {string(reportModeReport), string(reportModeBrief)},
}

func anyValues[T ~string](vs []T) []any {
	out := make([]any, len(vs))
	for i, v := range vs {
		out[i] = string(v)
	}
	return out
}

// inputSchema infers the JSON schema of a tool input struct. Required
// properties are exactly the fields without omitempty; every enum-typed
// property (also inside arrays and pointer fields) lists its values. A
// nullable enum (a pointer field) also accepts null, which means "unchanged"
// in patch inputs. Inference failure is a programmer error and panics at
// startup.
func inputSchema[T any]() *jsonschema.Schema {
	ts := make(map[reflect.Type]*jsonschema.Schema, len(enumValues))
	for t, vals := range enumValues {
		ts[t] = &jsonschema.Schema{Type: "string", Enum: slices.Clone(vals)}
	}
	s, err := jsonschema.For[T](&jsonschema.ForOptions{TypeSchemas: ts})
	if err != nil {
		panic(fmt.Sprintf("mcpserver: input schema for %v: %v", reflect.TypeFor[T](), err))
	}
	allowNullEnums(s)
	return s
}

// allowNullEnums walks the schema tree and appends null to the Enum of every
// schema whose types include "null", so JSON null still validates.
func allowNullEnums(s *jsonschema.Schema) {
	if s == nil {
		return
	}
	if len(s.Enum) > 0 && slices.Contains(s.Types, "null") && !slices.Contains(s.Enum, nil) {
		s.Enum = append(s.Enum, nil)
	}
	for _, p := range s.Properties {
		allowNullEnums(p)
	}
	allowNullEnums(s.Items)
	allowNullEnums(s.AdditionalProperties)
	for _, list := range [][]*jsonschema.Schema{s.PrefixItems, s.AnyOf, s.OneOf, s.AllOf} {
		for _, sub := range list {
			allowNullEnums(sub)
		}
	}
}
