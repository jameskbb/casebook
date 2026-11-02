package domain

import (
	"strings"
	"testing"
)

type enumLike interface {
	~string
	Valid() bool
	Label() string
	Validate(field string) error
}

type enumCase struct {
	name     string
	got      []string
	want     []string
	valid    func(string) bool
	label    func(string) string
	validate func(string, string) error
	parse    func(string) (string, error)
}

func mkEnum[T enumLike](name string, all []T, parse func(string) (T, error), want ...string) enumCase {
	got := make([]string, len(all))
	for i, v := range all {
		got[i] = string(v)
	}
	return enumCase{
		name:     name,
		got:      got,
		want:     want,
		valid:    func(s string) bool { return T(s).Valid() },
		label:    func(s string) string { return T(s).Label() },
		validate: func(field, s string) error { return T(s).Validate(field) },
		parse: func(s string) (string, error) {
			v, err := parse(s)
			return string(v), err
		},
	}
}

func allEnums() []enumCase {
	return []enumCase{
		mkEnum("InvestigationStatus", AllInvestigationStatuses(), ParseInvestigationStatus,
			"new", "investigating", "blocked", "resolved", "archived"),
		mkEnum("Confidence", AllConfidences(), ParseConfidence, "unknown", "low", "medium", "high"),
		mkEnum("Verification", AllVerifications(), ParseVerification,
			"ai_generated", "human_verified", "source_verified", "disputed", "superseded"),
		mkEnum("ClaimStatus", AllClaimStatuses(), ParseClaimStatus, "active", "disputed", "superseded", "rejected"),
		mkEnum("HypothesisStatus", AllHypothesisStatuses(), ParseHypothesisStatus,
			"open", "supported", "disproven", "superseded"),
		mkEnum("QuestionStatus", AllQuestionStatuses(), ParseQuestionStatus, "open", "resolved", "deferred"),
		mkEnum("SourceType", AllSourceTypes(), ParseSourceType,
			"sql_query", "code", "file", "jira", "documentation", "api_response", "user_statement",
			"ai_tool_output", "manual_observation", "external_url", "casebook_artifact", "evernote_note"),
		mkEnum("Relation", AllRelations(), ParseRelation, "supports", "contradicts"),
		mkEnum("ActorKind", AllActorKinds(), ParseActorKind, "human", "ai", "import", "system"),
		mkEnum("RecordType", AllRecordTypes(), ParseRecordType,
			"investigation", "evidence", "claim", "hypothesis", "question"),
	}
}

func TestEnumsAllValues(t *testing.T) {
	for _, e := range allEnums() {
		t.Run(e.name, func(t *testing.T) {
			if strings.Join(e.got, ",") != strings.Join(e.want, ",") {
				t.Fatalf("All = %v, want %v", e.got, e.want)
			}
			for _, v := range e.want {
				if !e.valid(v) {
					t.Errorf("%q.Valid() = false", v)
				}
				if err := e.validate("field", v); err != nil {
					t.Errorf("%q.Validate() = %v", v, err)
				}
				if e.label(v) == "" {
					t.Errorf("%q.Label() is empty", v)
				}
			}
			for _, bad := range []string{"", "bogus", strings.ToUpper(e.want[0])} {
				if e.valid(bad) {
					t.Errorf("%q.Valid() = true", bad)
				}
				err := e.validate("my_field", bad)
				ve, ok := IsValidation(err)
				if !ok || ve.Field != "my_field" {
					t.Errorf("%q.Validate(my_field) = %v, want ValidationError on my_field", bad, err)
				}
			}
		})
	}
}

func TestEnumsAllReturnsFreshSlice(t *testing.T) {
	a := AllConfidences()
	a[0] = "mutated"
	if AllConfidences()[0] != ConfidenceUnknown {
		t.Fatal("AllConfidences returned a shared slice")
	}
}

func TestEnumsParse(t *testing.T) {
	for _, e := range allEnums() {
		t.Run(e.name, func(t *testing.T) {
			for _, v := range e.want {
				got, err := e.parse("  " + strings.ToUpper(v) + " ")
				if err != nil || got != v {
					t.Errorf("Parse(%q) = %q, %v; want %q", strings.ToUpper(v), got, err, v)
				}
			}
			_, err := e.parse("bogus")
			ve, ok := IsValidation(err)
			if !ok {
				t.Fatalf("Parse(bogus) error = %v, want ValidationError", err)
			}
			for _, v := range e.want {
				if !strings.Contains(ve.Message, v) {
					t.Errorf("Parse(bogus) message %q does not list %q", ve.Message, v)
				}
			}
		})
	}
	if got, err := ParseConfidence(" High "); err != nil || got != ConfidenceHigh {
		t.Fatalf("ParseConfidence(\" High \") = %q, %v", got, err)
	}
}

func TestEnumsLabels(t *testing.T) {
	cases := map[string]string{
		ConfidenceHigh.Label():                   "High",
		ConfidenceUnknown.Label():                "Unknown",
		VerificationAIGenerated.Label():          "AI generated",
		VerificationHumanVerified.Label():        "Human verified",
		VerificationSourceVerified.Label():       "Source verified",
		InvestigationStatusInvestigating.Label(): "Investigating",
		SourceTypeSQLQuery.Label():               "SQL query",
		RelationContradicts.Label():              "Contradicts",
		ActorAI.Label():                          "AI",
		ClaimStatusActive.Label():                "Current",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("Label = %q, want %q", got, want)
		}
	}
}

func TestEnumsDefaultVerification(t *testing.T) {
	cases := map[ActorKind]Verification{
		ActorAI:     VerificationAIGenerated,
		ActorHuman:  VerificationHumanVerified,
		ActorImport: VerificationAIGenerated,
		ActorSystem: VerificationAIGenerated,
	}
	for kind, want := range cases {
		if got := DefaultVerification(kind); got != want {
			t.Errorf("DefaultVerification(%s) = %q, want %q", kind, got, want)
		}
	}
}

func TestEnumsRecordTypePrefix(t *testing.T) {
	cases := map[RecordType]string{
		RecordInvestigation: "INV",
		RecordEvidence:      "E",
		RecordClaim:         "C",
		RecordHypothesis:    "H",
		RecordQuestion:      "Q",
	}
	for rt, want := range cases {
		if got := rt.Prefix(); got != want {
			t.Errorf("%s.Prefix() = %q, want %q", rt, got, want)
		}
	}
}
