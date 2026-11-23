package cli

import (
	"fmt"

	"github.com/jameskbb/casebook/internal/app"
)

// The words a person reads in a terminal are the app's: question, answer,
// finding and source, and one verb, check. The labels come from the core
// (PlainLabel, PlainWord). JSON output keeps the stored values.

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
