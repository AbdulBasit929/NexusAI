package main

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
	"time"
)

// A3. A TEMPLATE ANSWER STATES ITS RESULT, NOT THE SIZE OF ITS TABLE.
//
// Measured 2026-09-29 on the deployed build (reports/column-labels-20260929/on):
// two questions scored NOT_STATED. The result was computed and on screen in the
// table, but the direct answer did not say it.
//
//	CDR-10   "Who did 923001110001 contact most frequently?"
//	         -> "8 phone contacts ranked for 923001110001."
//	ANPR-03  "When was LHR-2026 first and last seen?"
//	         -> "Computed exact normalized target matches, record-family
//	            coverage, and cited co-observed entities ..."
//
// Both values are already computed by the template's own SQL; nothing new is
// queried. The rules that keep each sentence true:
//
//   - A leader is named only together with every row tied with it. CDR-10's top
//     two contacts are tied at 121 CDR records; naming one of them would be a
//     choice the data does not make. frequentContactsSQL orders by the ranked
//     count, so a tie can only continue past the page when every returned row
//     ties and the page is full; then nothing is claimed.
//   - The count is stated as what the SQL counted: CDR records involving both
//     numbers. A record's direction field is relative to that record's owner,
//     so it is not restated as who called whom.
//   - First and last seen come from the family coverage, which
//     compileCrossFamilyCandidateRows computes over every exact match (the
//     function fails rather than truncate), never from the capped page of
//     displayed matches.
//   - A filter the question applied is named, so a bounded result does not read
//     as all-time.
//
// DEFAULT OFF, like every switch gating new behaviour. Off reproduces today's
// sentences exactly.
const templateStatesResultEnv = "FORENSIC_TEMPLATE_STATES_RESULT"

func templateStatesResultEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(templateStatesResultEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// maxStatedLeaders bounds how many tied values one sentence names.
const maxStatedLeaders = 5

// frequentContactLeaders names the contact, or every contact tied, with the
// highest ranked count, or returns "" when that cannot be established.
func frequentContactLeaders(req hybridQueryRequest, resp hybridQueryResponse) string {
	if !templateStatesResultEnabled() || resp.Template != "frequent_contacts" {
		return ""
	}
	rows := mapsFromAny(resp.Records["frequent_contacts"])
	if len(rows) == 0 {
		return ""
	}
	var top int64 = -1
	for _, row := range rows {
		total, ok := statedWholeCount(row["total_interactions"])
		if !ok || strings.TrimSpace(stringValueAny(row["counterparty"])) == "" {
			return ""
		}
		if total > top {
			top = total
		}
	}
	if top <= 0 {
		return ""
	}
	leaders := make([]string, 0, len(rows))
	for _, row := range rows {
		if total, _ := statedWholeCount(row["total_interactions"]); total == top {
			leaders = append(leaders, strings.TrimSpace(stringValueAny(row["counterparty"])))
		}
	}
	if len(leaders) == len(rows) && req.Limit > 0 && len(rows) >= req.Limit {
		return ""
	}
	target := defaultString(strings.TrimSpace(req.Target), "the selected participant")
	scope := statedFilterScope(req)
	records := fmt.Sprintf("%s CDR record%s", formatAnswerNumber(float64(top)), pluralSuffix(int(top)))
	if len(leaders) == 1 {
		return fmt.Sprintf("The most frequent contact of %s%s is %s, with %s between them.", target, scope, leaders[0], records)
	}
	return fmt.Sprintf("%s are tied as the most frequent contacts of %s%s, with %s each.", statedList(leaders), target, scope, records)
}

// firstLastSeenQuestion recognises a question about when something was first
// or last observed. countedFirstLast excludes "the last 5 calls", which asks
// for rows, not a time.
var (
	firstLastSeenQuestion = regexp.MustCompile(`(?i)\b(?:first|last|earliest|latest|most\s+recent(?:ly)?)\b[^.?!]{0,40}?\b(?:seen|sighted|sightings?|observed|observations?|appear(?:s|ed|ance|ances)?|recorded|detected|spotted|captured)\b`)
	countedFirstLast      = regexp.MustCompile(`(?i)\b(?:first|last|latest)\s+\d`)
)

// targetFirstLastSeen states when each requested target was first and last
// observed, from the complete family coverage, or returns "".
func targetFirstLastSeen(req hybridQueryRequest, resp hybridQueryResponse) string {
	if !templateStatesResultEnabled() || resp.Template != "cross_family_correlation" {
		return ""
	}
	if !firstLastSeenQuestion.MatchString(req.Query) || countedFirstLast.MatchString(req.Query) {
		return ""
	}
	coverage := mapsFromAny(resp.Records["family_coverage"])
	if len(coverage) == 0 {
		return ""
	}
	type span struct {
		first, last             time.Time
		firstFamily, lastFamily string
		count                   int64
		families                int
	}
	order := []string{}
	spans := map[string]*span{}
	for _, row := range coverage {
		target := strings.TrimSpace(stringValueAny(row["requested_target"]))
		first, firstOK := forensicTimeValue(row["first_seen"])
		last, lastOK := forensicTimeValue(row["last_seen"])
		count, countOK := statedWholeCount(row["observation_count"])
		if target == "" || !firstOK || !lastOK || !countOK || count <= 0 || last.Before(first) {
			return ""
		}
		family := strings.TrimSpace(stringValueAny(row["record_type"]))
		current := spans[target]
		if current == nil {
			current = &span{first: first, last: last, firstFamily: family, lastFamily: family}
			spans[target] = current
			order = append(order, target)
		} else {
			if first.Before(current.first) {
				current.first, current.firstFamily = first, family
			}
			if last.After(current.last) {
				current.last, current.lastFamily = last, family
			}
		}
		current.count += count
		current.families++
	}
	if len(order) > maxCrossFamilyTargets {
		return ""
	}
	scope := statedFilterScope(req)
	sentences := make([]string, 0, len(order))
	for _, target := range order {
		s := spans[target]
		first, last := statedTimestamp(s.first), statedTimestamp(s.last)
		switch {
		case s.families > 1:
			sentences = append(sentences, fmt.Sprintf("%s was first seen at %s (%s) and last seen at %s (%s), across %s matching records in %d record families%s.",
				target, first, familyNoun(s.firstFamily, 1), last, familyNoun(s.lastFamily, 1),
				formatAnswerNumber(float64(s.count)), s.families, scope))
		case s.first.Equal(s.last) && s.count == 1:
			sentences = append(sentences, fmt.Sprintf("%s was seen once, at %s (1 %s)%s.", target, first, familyNoun(s.firstFamily, 1), scope))
		case s.first.Equal(s.last):
			sentences = append(sentences, fmt.Sprintf("All %s %s of %s%s share one time, %s.",
				formatAnswerNumber(float64(s.count)), familyNoun(s.firstFamily, float64(s.count)), target, scope, first))
		default:
			sentences = append(sentences, fmt.Sprintf("%s was first seen at %s and last seen at %s, across %s %s%s.",
				target, first, last, formatAnswerNumber(float64(s.count)), familyNoun(s.firstFamily, float64(s.count)), scope))
		}
	}
	return strings.Join(sentences, " ")
}

// statedFilterScope names the filters a template applied, without restating
// their bounds (the date upper bound is exclusive, which "between" would
// misstate).
func statedFilterScope(req hybridQueryRequest) string {
	parts := []string{}
	if strings.TrimSpace(req.DateFrom) != "" || strings.TrimSpace(req.DateTo) != "" {
		parts = append(parts, "in the requested date range")
	}
	if direction := strings.ToLower(strings.TrimSpace(req.Direction)); direction != "" {
		parts = append(parts, "counting only "+direction+" records")
	}
	if strings.TrimSpace(req.RecordType) != "" || strings.TrimSpace(req.SourceFile) != "" || strings.TrimSpace(req.BatchID) != "" {
		parts = append(parts, "within the requested source filters")
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, ", ")
}

// statedList joins values as "a, b and c", naming at most maxStatedLeaders.
func statedList(values []string) string {
	if len(values) > maxStatedLeaders {
		rest := len(values) - maxStatedLeaders
		return strings.Join(values[:maxStatedLeaders], ", ") + fmt.Sprintf(" and %d other%s", rest, pluralSuffix(rest))
	}
	if len(values) <= 1 {
		return strings.Join(values, "")
	}
	return strings.Join(values[:len(values)-1], ", ") + " and " + values[len(values)-1]
}

// statedTimestamp renders an instant exactly, to the second, in UTC.
func statedTimestamp(value time.Time) string {
	return value.UTC().Format("2006-01-02 15:04:05") + " UTC"
}

// statedWholeCount reads a non-negative whole count, or ok=false. A count is
// never rounded into existence.
func statedWholeCount(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), typed >= 0
	case int32:
		return int64(typed), typed >= 0
	case int64:
		return typed, typed >= 0
	case float64:
		if typed >= 0 && typed == math.Trunc(typed) && !math.IsInf(typed, 0) {
			return int64(typed), true
		}
	}
	return 0, false
}
