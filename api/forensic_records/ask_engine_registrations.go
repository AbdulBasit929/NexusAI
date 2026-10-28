package main

import (
	"fmt"
	"regexp"
	"strings"
)

// ASK ENGINE V2: REGISTRATIONS AND HANDSETS (docs/data/subscriber-device-family-20261028.md, piece g). What is said when a question about a registration or a handset finds no row, and the
// correction put in front of a question that asks who owns or holds a number. A registration is what one source file records; the case holds no name, and the engine says nothing of who a person is.

// askOwnershipCue is a question that asks who holds a number, which no registration states.
var askOwnershipCue = regexp.MustCompile(`\b(?:owns?|owner|owned|ownership|belongs? to|holder|whose|registered to whom)\b|\bwho (?:is|was) (?:\S+ ){0,6}registered (?:to|with)\b`)

// askRegistrationAbsence says what was searched when a registration view returns no row: the number, the SIM serial or the SIM identity of the question, or the pattern asked for.
func askRegistrationAbsence(v *askValidated, f askFrame) string {
	lowerSQL := strings.ToLower(v.SQL)
	if askReadsView(v, "v_number_registration_activity") {
		what := "an event outside its registration periods"
		if strings.Contains(lowerSQL, "events_after_last_end") {
			what = "an event after the end of its last registration"
		}
		if len(f.Numbers) == 1 {
			return fmt.Sprintf("%s has no registration with %s in the call records of this case", f.Numbers[0], what)
		}
		return "no number with " + what + " was found in the call records of this case"
	}
	switch {
	case strings.Contains(lowerSQL, "conflicts"):
		return "no registration that another source file contradicts was found"
	case len(f.Serials) == 1:
		return "no registration on the SIM serial " + f.Serials[0] + " was found in the files of this case"
	case len(f.IMEIs) == 1:
		return "no registration on the SIM identity " + f.IMEIs[0] + " was found in the files of this case"
	case len(f.Numbers) == 1:
		return "no registration of " + f.Numbers[0] + askPeriodPhrase(f) + " was found in the files of this case"
	case len(f.Numbers) > 1:
		return "no registration of " + strings.Join(f.Numbers, ", ") + askPeriodPhrase(f) + " was found in the files of this case"
	}
	return "no registration matching your question was found in the files of this case"
}

// askHandsetAbsence says what was searched when the handset view returns no row.
func askHandsetAbsence(v *askValidated, f askFrame) string {
	switch {
	case len(f.Serials) == 1:
		return "no handset with the TAC " + f.Serials[0] + " is recorded in the call records of this case"
	case len(f.IMEIs) == 1:
		return "no handset with the IMEI " + f.IMEIs[0] + " is recorded in the call records of this case"
	case len(f.Numbers) == 1:
		return "no handset of " + f.Numbers[0] + askPeriodPhrase(f) + " is recorded in the call records of this case"
	}
	return "no handset matching your question is recorded in the call records of this case"
}

// askRegistrationLead is the correction put in front of a question that asks who owns or holds a number, or states a conclusion the registrations cannot support. "" when it asks none.
func askRegistrationLead(v *askValidated, f askFrame, stated bool) string {
	lower := strings.ToLower(f.Question)
	switch {
	case askReadsView(v, "v_subscriber_registrations"), askReadsView(v, "v_number_registration_activity"):
		if stated || askOwnershipCue.MatchString(lower) {
			return "A registration records what one source file states, not who holds or uses the number."
		}
	case askReadsView(v, "v_handsets"):
		if stated {
			return "A listed make and model is what a reference list says for the handset's TAC, not a fact about the individual handset."
		}
	}
	return ""
}
