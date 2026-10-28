package main

// ASK ENGINE V2: THE CAVEATS OF AN ANALYSIS (link analysis stage 3, docs/data/link-analysis-stage3-preregistration-20261022.md section 6).
//
// The engine, not the model, adds these lines to the response's limitations, keyed by the view the query reads, so the model can neither omit nor reword them. The texts are
// fixed in the pre-registration: a gate checks that every answer that returns a result of the analysis carries them, and an edit here is an amendment there.
// askAnchorCaveat is the caveat of the anchor views: weeks are blocks of 7 days from the start of the case, the night is 22:00 to 07:00 and the day 09:00 to 17:00 on weekdays, on the case clock.
const askAnchorCaveat = "Cell sites, not positions of a person: an anchor is the cell site with most events of a slot in a week, a coverage area, and a move or a trip is a change of that site. " +
	"Weeks are blocks of 7 days from the start of the case; the night is 22:00 to 07:00 and the day 09:00 to 17:00 on weekdays, on the case clock."

// askPlateCaveat is the caveat of the camera views: a sighting is a camera's reading, not a finding about a driver, and a look-alike plate is another plate.
const askPlateCaveat = "A camera sighting says that a camera read this plate at this time, not who drove; a camera can misread a plate. A plate is matched exactly as its search key (capitals, no separators): a plate that looks alike, an O for a 0, is another plate and is not counted. The position is the camera's, not the vehicle's."

// askRegistrationCaveat is the caveat of the registration views: a registration is what one source states, not who holds or uses the number.
const askRegistrationCaveat = "A registration is what one source file states: this number was registered, per that file, as this SIM with this status in this period. It does not say who holds or uses the number, " +
	"and two sources can differ (both are shown; a row marked as conflicting says so). Dates are the source's, on the case clock; a registration without a date is open on that side."

// askHandsetCaveat is the caveat of the handset view: a TAC is listed as a make and model per source, not a fact about the individual handset.
const askHandsetCaveat = "A make and model are what the device reference lists for the first eight digits of the handset's IMEI (its TAC), per source: a listing, not a fact about the individual handset. " +
	"A TAC the list does not hold is unknown, not unusual."

var askViewCaveatText = map[string]string{
	// A5: co-location within the stated window
	"v_colocation_windows": "Proximity is not association: two numbers at the same cell site within the stated window of each other were in the same coverage area, which is not evidence that they met or know each other. " +
		"No chance baseline is computed here: how often two unrelated numbers would be at one site by chance is not shown.",
	// A9: successor numbers (stage 3b)
	"v_successor_candidates": "Behavioural similarity, not identity: a new number that starts when another stops and shares its contacts, places or handset may have taken over from it, or may not. " +
		"The score is a documented formula (0.6 contact similarity, 0.2 shared places, 0.2 shared handset), not a probability.",
	"v_silent_numbers": "Silence is a gap in the records of this case, not a finding about the person: the number may have been used elsewhere, or the export may end early.",
	// A10: movement (stage 3c)
	"v_weekly_anchors":    askAnchorCaveat,
	"v_anchor_runs":       askAnchorCaveat,
	"v_movement_anchors":  askAnchorCaveat,
	"v_anchor_changes":    askAnchorCaveat,
	"v_anchor_deviations": askAnchorCaveat,
	"v_speed_alerts":      "Cell sites, not positions of a person: two events at distant cell sites too close in time may be a handover between towers, a fault or a record error, and are not proof that anyone travelled that fast.",
	// A11: change of rhythm (stage 3c)
	"v_rhythm_changes": "A change in the pattern of use, not an explanation: the daily count of events changed, and the records do not say why. A short series can show a change by chance; a change is listed only when |t| of Welch's test is at least 5.",
	// where a number was: a cell visit is a coverage area (the probe "Where was X on that Tuesday at nine?" of stage 3c is answered from it)
	"v_cell_visits": "Cell sites, not positions of a person: a cell visit says which coverage area a phone used at that time, not where its holder was.",
	// camera sightings of plates (ANPR)
	"v_plate_sightings": askPlateCaveat,
	"v_plate_visits":    askPlateCaveat,
	// registrations and handsets (subscriber and device families)
	"v_subscriber_registrations":     askRegistrationCaveat,
	"v_number_registration_activity": askRegistrationCaveat + " An event outside a registration period is a pattern in the records, not a finding: a registration can be missing, late or wrong.",
	"v_handsets":                     askHandsetCaveat,
	// A7: possible meetings
	"v_meeting_candidates": "A candidate, not a finding: proximity and contact are not proof of a meeting. " +
		"The score is a documented formula (half from the number of co-located pairs, half from how short the gap to the last contact is), not a probability.",
}

// askViewCaveats returns the caveat lines of the analysis views a validated query reads, in the order of its sorted views.
func askViewCaveats(v *askValidated) []string {
	if v == nil {
		return nil
	}
	var out []string
	for _, view := range v.Views {
		if text, ok := askViewCaveatText[view]; ok {
			out = append(out, text)
		}
	}
	return out
}
