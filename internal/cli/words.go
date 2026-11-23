package cli

import (
	"fmt"

	"github.com/jameskbb/casebook/internal/app"
)

// The words a person reads in a terminal are the app's: question, answer,
// finding and source, and one verb, check. JSON output keeps the stored
// values.

// checkLabel says who checked a record.
func checkLabel(v app.Verification) string {
	switch v {
	case app.VerificationAIGenerated:
		return "Not checked"
	case app.VerificationSourceVerified:
		return "AI checked the source"
	case app.VerificationHumanVerified:
		return "You checked this"
	case app.VerificationDisputed:
		return "Sources disagree"
	case app.VerificationSuperseded:
		return "Replaced"
	}
	return string(v)
}

// claimLabel is the state of a finding the answer can rest on.
func claimLabel(s app.ClaimStatus) string {
	switch s {
	case app.ClaimStatusActive:
		return "Current"
	case app.ClaimStatusDisputed:
		return "Sources disagree"
	case app.ClaimStatusSuperseded:
		return "Replaced"
	case app.ClaimStatusRejected:
		return "Ruled out"
	}
	return string(s)
}

// hypothesisLabel is the state of a finding that is being tested.
func hypothesisLabel(s app.HypothesisStatus) string {
	switch s {
	case app.HypothesisStatusOpen:
		return "Not tested yet"
	case app.HypothesisStatusSupported:
		return "Supported"
	case app.HypothesisStatusDisproven:
		return "Ruled out"
	case app.HypothesisStatusSuperseded:
		return "Replaced"
	}
	return string(s)
}

// recordLabel names a record type in search results.
func recordLabel(t app.RecordType) string {
	switch t {
	case app.RecordInvestigation:
		return "question"
	case app.RecordEvidence:
		return "source"
	case app.RecordClaim, app.RecordHypothesis:
		return "finding"
	case app.RecordQuestion:
		return "follow-up"
	}
	return string(t)
}

// sourcesChecked is the count the app shows in place of a confidence rating:
// how many of the sources a person has checked. Replaced sources are left out.
func sourcesChecked(evidence []app.Evidence) string {
	total, byYou := 0, 0
	for _, e := range evidence {
		if e.Verification == app.VerificationSuperseded {
			continue
		}
		total++
		if e.Verification == app.VerificationHumanVerified {
			byYou++
		}
	}
	if total == 0 {
		return "No sources yet"
	}
	noun := "sources"
	if total == 1 {
		noun = "source"
	}
	return fmt.Sprintf("%d of %d %s checked by you", byYou, total, noun)
}
