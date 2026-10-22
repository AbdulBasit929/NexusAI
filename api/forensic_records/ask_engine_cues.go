package main

import (
	"regexp"
	"strings"
)

// ASK ENGINE V2: THE WORDS THAT POINT AT THE ANALYSIS VIEWS (link analysis stages 3b and 3c: successor numbers, movement, speed alerts, change of rhythm).
//
// The curated lexicon scores single words; these analyses are named by phrases ("took over", "moved house", "faster than 150 km/h"), so they get their own cues on the normalised
// question. A cue adds points to its view, and a question that carries one is answered from that view alone: the views already join and exclude, and a second view would only tempt the
// model to write the rule again. Numbers in a question ("silent for 10 days", "a score of at least 0.5", "faster than 150 km/h") are NOT read here: ask_engine_params.go reads them.

var (
	askSuccessorRE = regexp.MustCompile(`\b(successors?|replac(?:e|ed|es|ing|ement|ements)|took over|taken over|take over|takes over|taking over|new number|switch(?:ed|es)? (?:to|numbers?)|same person (?:as|with)|changed (?:their |his |her )?(?:phone )?numbers?|got a new (?:sim|number|phone number))\b`)
	askSilentRE    = regexp.MustCompile(`\b(silent|silence|dormant|inactive|went quiet|gone quiet|went dark|gone dark|stopped (?:being )?(?:used|active)|no activity)\b`)
	askMovedRE     = regexp.MustCompile(`\b(moved|move house|moves house|relocat\w*|changed (?:their |his |her )?(?:home|residence|address|anchor|place)|new home|new residence)\b`)
	askTripRE      = regexp.MustCompile(`\b(trips?|travell?ed away|short stay|temporary|deviation|deviations|away from (?:home|their usual|his usual|her usual))\b`)
	askAnchorRE    = regexp.MustCompile(`\b(anchors?)\b`)
	askWeeklyRE    = regexp.MustCompile(`\bweeks? (?:starting|beginning|commencing|of)\b|\bweek \d{1,2}\b|\b\d{4}-\d{2}-\d{2}\b`)
	askWeekFirst   = regexp.MustCompile(`\b(weeks?|weekly)\b.*\b(sleep|sleeps|slept|sleeping|reside|resides)\b`)
	askAnchorWeek  = regexp.MustCompile(`\b(sleep|sleeps|slept|sleeping|live|lives|living|reside|resides|home|residence|workplace)\b.*\b(weeks?|weekly|hold|holds|held|stretch|stretches|since when|for how long)\b`)
	askSpeedRE     = regexp.MustCompile(`\b(speeds?|impossibl\w+|implausibl\w+|teleport\w*|too fast|faster than|km/h|kmh|kph|kilomet\w+ per hour|physically)\b`)
	askRhythmRE    = regexp.MustCompile(`\b(rhythm|pattern of use|change point|(?:activity|usage|use|calls?|events|texting) (?:changed|change|increased|decreased|dropped|rose|jumped|doubled|tripled|spiked)|(?:activity|usage|call volume|calls?|calling|texting|rate of (?:calls|events|use))\b[^.?!]{0,60}?\b(?:chang(?:e|ed|es)|(?:in|de)creas(?:e|ed|es)|drop(?:s|ped)?|rose|ris(?:e|es)|jump(?:s|ed)?|doubl(?:e|ed|es)|halv(?:e|ed|es)|tripl(?:e|ed|es)|spik(?:e|ed|es)|shift(?:s|ed)?|went up|went down|gone up|gone down|risen|fallen|fell)|(?:change|shift|jump|drop|spike|surge|increase|decrease) in (?:the )?(?:daily |weekly )?(?:activity|usage|use|rhythm|calls|events)|(?:more|less) active|busier|quieter|become (?:more|less) active|became (?:more|less) active)\b`)
)

// askAnalysisCues is which of the analyses a question speaks of.
type askAnalysisCues struct {
	Successor, Silent, Moved, Trip, Anchor, Weekly, Speed, Rhythm bool
}

// askCuesOf reads the cues of a normalised question.
func askCuesOf(normalised string) askAnalysisCues {
	q := strings.ToLower(normalised)
	return askAnalysisCues{
		Successor: askSuccessorRE.MatchString(q),
		Silent:    askSilentRE.MatchString(q),
		Moved:     askMovedRE.MatchString(q),
		Trip:      askTripRE.MatchString(q),
		Anchor:    askAnchorRE.MatchString(q) || askAnchorWeek.MatchString(q) || askWeekFirst.MatchString(q),
		Weekly:    askWeeklyRE.MatchString(q),
		Speed:     askSpeedRE.MatchString(q),
		Rhythm:    askRhythmRE.MatchString(q),
	}
}

// askAnalysisView is the single view a question with these cues is answered from ("" when it has none). The order is the order of precedence: a question about who took over from a number
// is not also about silence, a move is not also an anchor, and so on.
func (c askAnalysisCues) view() string {
	switch {
	case c.Successor:
		return "v_successor_candidates"
	case c.Silent:
		return "v_silent_numbers"
	case c.Moved:
		return "v_anchor_changes"
	case c.Trip:
		return "v_anchor_deviations"
	case c.Speed:
		return "v_speed_alerts"
	case c.Rhythm:
		return "v_rhythm_changes"
	case c.Anchor && c.Weekly:
		return "v_weekly_anchors" // a particular week or period: the anchor of each week
	case c.Anchor:
		return "v_movement_anchors" // the stretches of weeks with the same anchor
	}
	return ""
}

// askAnchorViews are the views whose rows carry the part of the day as a column (slot: night or day).
var askAnchorViews = []string{"v_weekly_anchors", "v_anchor_runs", "v_movement_anchors", "v_anchor_changes", "v_anchor_deviations"}

// askUsesAnchorView is whether the query reads one of them.
func askUsesAnchorView(v *askValidated) bool {
	for _, view := range askAnchorViews {
		if askUsesView(v, view) {
			return true
		}
	}
	return false
}

// askAnalysisViews are the views of the link-analysis stages whose empty result is an answer ("no pair met", "nobody went silent", "no move"), not a query that looked in the wrong place.
var askAnalysisViews = []string{
	"v_connection_edges", "v_colocation_windows", "v_meeting_candidates", "v_silent_numbers", "v_successor_candidates", "v_weekly_anchors", "v_anchor_runs", "v_movement_anchors",
	"v_anchor_changes", "v_anchor_deviations", "v_speed_alerts", "v_rhythm_changes",
}

// askUsesAnalysisView is whether the query reads one of them.
func askUsesAnalysisView(v *askValidated) bool {
	for _, view := range askAnalysisViews {
		if askUsesView(v, view) {
			return true
		}
	}
	return false
}
