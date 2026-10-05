package main

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Result-driven answer text. Before this, the analyst headline and every Fact
// Packet fact described the machinery ("Executed bounded source-native typed
// algebra over 8642 authorized source rows and returned 1 deterministic
// results") instead of the answer ("There are 8,642 CDR records in this
// case"). The LLM narrator could only paraphrase that plumbing, and its output
// was correctly rejected by validateNarrative. Every sentence built here is
// computed from returned result values only — nothing is inferred.

// resultAnswer is the analyst-facing headline plus supporting findings.
type resultAnswer struct {
	Headline string
	Findings []string
	// Value is the answer itself whenever the question has a single numeric
	// answer: a count, a total, a minimum, a maximum, or the top group's
	// measure. It lets the Fact Packet carry the answer AS A VALUE and not
	// only as a sentence, so the narrator and the presentation layer can each
	// cite it without re-parsing prose.
	Value any
}

var familyNouns = map[string][2]string{
	"cdr":            {"CDR record", "CDR records"},
	"ipdr":           {"IPDR session", "IPDR sessions"},
	"anpr":           {"ANPR sighting", "ANPR sightings"},
	"access_log":     {"access log entry", "access log entries"},
	"subscriber":     {"subscriber record", "subscriber records"},
	"tower_location": {"tower record", "tower records"},
	"transaction":    {"transaction", "transactions"},
	"email":          {"email record", "email records"},
	"generic":        {"record", "records"},
	"":               {"record", "records"},
}

func familyNoun(family string, n float64) string {
	nouns, ok := familyNouns[family]
	if !ok {
		nouns = [2]string{family + " record", family + " records"}
	}
	if n == 1 {
		return nouns[0]
	}
	return nouns[1]
}

// formatAnswerNumber renders integers with thousands separators and trims
// decimals to at most two places.
func formatAnswerNumber(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "unknown"
	}
	negative := value < 0
	value = math.Abs(value)
	whole := math.Floor(value)
	frac := value - whole
	digits := strconv.FormatFloat(whole, 'f', 0, 64)
	var out strings.Builder
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(r)
	}
	text := out.String()
	if frac > 0.0000001 {
		text += strings.TrimRight(strings.TrimRight(strconv.FormatFloat(frac, 'f', 2, 64)[1:], "0"), ".")
	}
	if negative {
		text = "-" + text
	}
	return text
}

func answerNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case string:
		parsed, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(v), ",", ""), 64)
		return parsed, err == nil
	}
	return 0, false
}

func humanFieldName(name string) string {
	name = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(name, "_", " "), "-", " "))
	return strings.ToLower(name)
}

// answerScopeQualifier describes the constraints that shaped the result so
// the headline never overstates its scope.
func answerScopeQualifier(req hybridQueryRequest) string {
	parts := []string{}
	if target := strings.TrimSpace(req.Target); target != "" {
		parts = append(parts, "involving "+target)
	}
	// A1b.1. A RANGE FILTER SHAPED THE RESULT EXACTLY AS A TARGET DOES, and the
	// headline must say so for the same reason.
	//
	// Measured 2026-09-27: with A1b binding the filter, "How many transactions
	// have an amount above 50000?" answered "There is 1 transaction in this
	// case." The count is right and the case holds FOUR transactions, so the
	// sentence read alone -- exported, quoted, pasted into a report -- states
	// something false. A true number inside a sentence that claims more than
	// was computed is the defect this function exists to prevent; it simply had
	// no case for this filter shape.
	parts = append(parts, rangeFilterQualifiers(req)...)
	from, to := strings.TrimSpace(req.DateFrom), strings.TrimSpace(req.DateTo)
	switch {
	case from != "" && to != "":
		parts = append(parts, fmt.Sprintf("from %s up to (not including) %s", shortDate(from), shortDate(to)))
	case from != "":
		parts = append(parts, "from "+shortDate(from))
	case to != "":
		parts = append(parts, "before "+shortDate(to))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

// rangeComparisonWording renders one bound in the analyst's terms. Phrasing
// only -- the operator itself is the plan's, never re-interpreted here.
func rangeComparisonWording(op string) string {
	switch strings.ToUpper(strings.TrimSpace(op)) {
	case "GT":
		return "above"
	case "GTE":
		return "of at least"
	case "LT":
		return "below"
	case "LTE":
		return "of at most"
	}
	return ""
}

// rangeFilterQualifiers describes every range bound the EXECUTED PLAN carries.
//
// Read from the plan, never from the question: the plan is what ran, and the
// question is only what was asked. That distinction is the whole reason
// `answerNamesUnfilteredTarget` exists.
//
// Gated on the same switch as A1b so the two are one measurable unit. With the
// switch off nothing in this build emits a range filter, so the qualifier is
// moot and today's wording is untouched.
func rangeFilterQualifiers(req hybridQueryRequest) []string {
	if !rangeFiltersEnabled() || req.SourceNative == nil {
		return nil
	}
	out := []string{}
	for _, filter := range req.SourceNative.Filters {
		wording := rangeComparisonWording(filter.Op)
		if wording == "" {
			continue
		}
		value := strings.TrimSpace(filter.Value)
		if value == "" {
			continue
		}
		field := filter.FieldID
		if _, tail, found := strings.Cut(field, "."); found {
			field = tail
		}
		out = append(out, "with "+humanFieldName(field)+" "+wording+" "+value)
	}
	return out
}

func shortDate(value string) string {
	if len(value) >= 10 {
		return value[:10]
	}
	return value
}

// buildResultAnswer returns an answer computed from the returned rows, or
// ok=false when the result shape is not one it can describe faithfully (the
// caller then keeps its existing text).
func buildResultAnswer(req hybridQueryRequest, resp hybridQueryResponse, rows []map[string]any) (resultAnswer, bool) {
	if plan, ok := resp.Records["plan"].(*SourceNativePlanV1); ok && plan != nil {
		return sourceNativeResultAnswer(req, resp, plan)
	}
	if planMap, ok := resp.Records["plan"].(map[string]any); ok && planMap != nil {
		var plan SourceNativePlanV1
		if decodeViaJSON(planMap, &plan) == nil && plan.ContractVersion != "" {
			return sourceNativeResultAnswer(req, resp, &plan)
		}
	}
	return tabularResultAnswer(req, resp, rows)
}

func sourceNativeResultAnswer(req hybridQueryRequest, resp hybridQueryResponse, plan *SourceNativePlanV1) (resultAnswer, bool) {
	results, _ := resp.Records["source_native_results"].([]map[string]any)
	if results == nil {
		results = mapsFromAny(resp.Records["source_native_results"])
	}
	if len(plan.Project) > 0 {
		return resultAnswer{}, false
	}
	if len(plan.Measures) != 1 {
		if low, high, ok := sourceNativeRangeMeasures(plan); ok && answerStatesValuesEnabled() && len(results) > 0 {
			label := humanFieldName(sourceNativeFieldNameFor(resp, low.FieldID))
			if headline := answerRangeHeadline(label,
				resultNoun(req, resp, 2), answerScopeQualifier(req),
				stringValueAny(results[0][low.MeasureID]),
				stringValueAny(results[0][high.MeasureID])); headline != "" {
				return resultAnswer{Headline: headline}, true
			}
		}
		// S1 Part A: an ungrouped multi-measure plan still counted its records.
		if answer, ok := multiMeasureRecordCountHeadline(req, resp, plan, results); ok {
			return answer, true
		}
		return resultAnswer{}, false
	}
	names := map[string]string{}
	// A curated display name is what the analyst should read. Falling back to the
	// raw source name produces "5 inbound outbound ind values" where the layer
	// says "Call direction" — the same class of defect as a column headed "M1".
	display := map[string]string{}
	for _, field := range catalogFromAny(resp.Records["field_catalog"]) {
		names[field.FieldID] = field.NormalizedName
		if strings.TrimSpace(field.DisplayName) != "" {
			display[field.FieldID] = strings.ToLower(field.DisplayName)
		}
	}
	measure := plan.Measures[0]
	// Filters the plan applied are part of the scope. See headline_honesty.go.
	scope := answerScopeQualifier(req) + planFilterQualifiers(req, resp, plan)
	measureLabel := func() string {
		field := humanFieldName(names[measure.FieldID])
		if curated, ok := display[measure.FieldID]; ok {
			field = curated
		}
		switch measure.Op {
		case "SUM":
			return "total " + field
		case "AVG":
			return "average " + field
		case "MIN":
			return "minimum " + field
		case "MAX":
			return "maximum " + field
		}
		return "count"
	}

	if len(plan.GroupFields) == 0 && plan.TimeBucket == nil {
		value := 0.0
		if len(results) > 0 {
			value, _ = answerNumber(results[0][measure.MeasureID])
		}
		if measure.Op == "COUNT" && !countsFieldValues(plan, measure) {
			if value == 0 {
				return resultAnswer{Headline: withPlateReadNote(fmt.Sprintf("There are no %s%s in this case.", plateReadNounOr(plan, resultNoun(req, resp, 0), 0), scope), resp, plan, 0), Value: 0.0}, true
			}
			verb := "are"
			if value == 1 {
				verb = "is"
			}
			return resultAnswer{Headline: withPlateReadNote(fmt.Sprintf("There %s %s %s%s in this case.", verb, formatAnswerNumber(value), plateReadNounOr(plan, resultNoun(req, resp, value), value), scope), resp, plan, value), Value: value}, true
		}
		if len(results) == 0 {
			return resultAnswer{Headline: fmt.Sprintf("No %s%s were available to compute the %s.", resultNoun(req, resp, 0), scope, measureLabel())}, true
		}
		// A total over no recorded values is not zero. See headline_honesty.go.
		if stated, ok := emptyAggregateHeadline(req, resp, measure, results[0], resultNoun(req, resp, 0), scope); ok {
			return resultAnswer{Headline: stated}, true
		}
		headline := fmt.Sprintf("The %s across %s%s is %s.", measureLabel(), resultNoun(req, resp, 0), scope, formatAnswerNumber(value))
		// A4: a count of a field names the field. See count_names_field.go.
		if stated, ok := countedFieldHeadline(req, resp, plan, measure, value, scope); ok {
			headline = stated
		}
		// AN AVERAGE MUST SAY WHAT IT AVERAGED OVER. A field present on 263 of 309
		// rows produces a number that reads as a statement about all 309.
		if valueCount, ok := answerNumber(results[0][measureDenominatorKey(measure.MeasureID)]); ok {
			contributing := 0.0
			if metadata, ok := results[0]["metadata"].(map[string]any); ok {
				contributing, _ = answerNumber(metadata["contributing_row_count"])
			}
			if note := measureDenominatorNote(int64(valueCount), int64(contributing), resultNoun(req, resp, contributing)); note != "" {
				headline += " " + note
			}
		}
		return resultAnswer{Headline: headline, Value: value}, true
	}
	if len(plan.GroupFields) != 1 || plan.TimeBucket != nil {
		return resultAnswer{}, false
	}
	groupName := names[plan.GroupFields[0]]
	if groupName == "" || len(results) == 0 {
		return resultAnswer{}, false
	}
	type entry struct {
		key   string
		value float64
	}
	entries := []entry{}
	total := 0.0
	for _, row := range results {
		value, ok := answerNumber(row[measure.MeasureID])
		if !ok {
			continue
		}
		key := strings.TrimSpace(stringValueAny(row[groupName]))
		if key == "" || key == "<nil>" {
			key = "(no value)"
		}
		entries = append(entries, entry{key, value})
		total += value
	}
	if len(entries) == 0 {
		return resultAnswer{}, false
	}
	dimension := humanFieldName(groupName)
	groupFieldID := ""
	if len(plan.GroupFields) > 0 {
		groupFieldID = plan.GroupFields[0]
	}
	if curated, ok := display[groupFieldID]; ok {
		dimension = curated
	}
	ranked := len(plan.Sort) > 0 && plan.Sort[0].Target == measure.MeasureID
	if ranked && (plan.Limit == 1 || len(entries) == 1) {
		top := entries[0]
		// A ranking whose winner has no value is not an answer. "The counterparty
		// with the highest count is (no value)" is a confident non-answer, and
		// worse than declining: the data does not support the ranking that was
		// asked for. Returning false lets the caller clarify instead.
		if top.key == "(no value)" {
			return resultAnswer{}, false
		}
		if plan.Sort[0].Direction == "ASC" {
			return resultAnswer{Headline: fmt.Sprintf("The %s with the lowest %s%s is %s (%s).", dimension, measureLabel(), scope, curatedValueLabel(groupFieldID, top.key), formatAnswerNumber(top.value)), Value: top.value}, true
		}
		return resultAnswer{Headline: fmt.Sprintf("The %s with the highest %s%s is %s (%s).", dimension, measureLabel(), scope, curatedValueLabel(groupFieldID, top.key), formatAnswerNumber(top.value)), Value: top.value}, true
	}
	if !ranked {
		sort.SliceStable(entries, func(i, j int) bool { return entries[i].value > entries[j].value })
	}
	findings := make([]string, 0, len(entries))
	for _, e := range entries {
		findings = append(findings, fmt.Sprintf("%s %s: %s", dimension, curatedValueLabel(groupFieldID, e.key), formatAnswerNumber(e.value)))
	}
	complete, _ := resp.Records["complete"].(bool)
	headline := fmt.Sprintf("By %s%s: %s.", dimension, scope, joinTopEntries(findings, dimension, 5))
	if measure.Op == "COUNT" && complete && len(entries) < sourceNativeGroupLimit {
		stated := 3
		lead := "Largest"
		if answerStatesValuesEnabled() && len(entries) <= answerBreakdownFullLimit {
			stated = len(entries)
			lead = "In full"
		}
		headline = fmt.Sprintf("%s %s%s across %d %s value%s. %s: %s.", formatAnswerNumber(total), resultNoun(req, resp, total), scope, len(entries), dimension, pluralSuffix(len(entries)), lead, joinTopEntries(findings, dimension, stated))
	} else if answerStatesValuesEnabled() && complete && len(entries) <= answerBreakdownFullLimit {
		headline = fmt.Sprintf("By %s%s: %s.", dimension, scope, joinTopEntries(findings, dimension, len(entries)))
	}
	return resultAnswer{Headline: headline, Findings: findings, Value: total}, true
}

// joinTopEntries renders the first n "<dimension> <key>: <value>" findings as
// "<key>: <value>" for a headline, where the dimension is already named.
// curatedValueLabel renders an enumerated VALUE the way the analyst reads it.
//
// The dimension was already curated ("Call type"), but its values were not, so
// one screen showed the answer saying "GPRS: 5,863 ; SMS: 1,108 ; CALL: 930"
// above a table saying "Data session", "SMS", "Call" — the same values in two
// vocabularies, which invites the analyst to wonder whether they are the same
// data. Observed 2026-09-23 in the UI review screenshots.
//
// An uncurated value returns unchanged: a raw identifier is a truthful
// fallback, and inventing a label for a value the layer does not define would
// be worse than showing the value itself.
func curatedValueLabel(fieldID, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || strings.TrimSpace(fieldID) == "" {
		return value
	}
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		return value
	}
	field, ok := layer.FieldByID(strings.TrimSpace(fieldID))
	if !ok {
		return value
	}
	for _, candidate := range field.Values {
		if !strings.EqualFold(strings.TrimSpace(candidate.Value), trimmed) {
			continue
		}
		if name := strings.TrimSpace(candidate.DisplayName); name != "" {
			return name
		}
	}
	return value
}

func joinTopEntries(items []string, dimension string, n int) string {
	parts := []string{}
	for i, item := range items {
		if i >= n {
			break
		}
		parts = append(parts, strings.TrimPrefix(item, dimension+" "))
	}
	return strings.Join(parts, "; ")
}

// Measure columns produced by the registered templates, in preference order.
var tabularCountColumns = []string{"event_count", "session_count", "transaction_count", "row_count", "observation_count", "sighting_count", "record_count", "count"}
var tabularAmountColumns = []string{"total_amount_decimal", "total_amount", "total_bytes"}

// tabularResultAnswer summarises a registered template's grouped table: the
// grand total of its count column, per-category totals for the first
// categorical column, and totals for amount columns. It declines rather than
// guess when the table has no recognised measure column or is truncated.
func tabularResultAnswer(req hybridQueryRequest, resp hybridQueryResponse, rows []map[string]any) (resultAnswer, bool) {
	if len(rows) == 0 || len(rows) >= maxFactPacketRows*10 {
		return resultAnswer{}, false
	}
	if total := numericFloat(resp.Answer["records_row_count"]); total > float64(len(rows)) {
		return resultAnswer{}, false // bounded page: totals would understate
	}
	countColumn := ""
	for _, column := range tabularCountColumns {
		if _, ok := answerNumber(rows[0][column]); ok {
			countColumn = column
			break
		}
	}
	if countColumn == "" {
		return resultAnswer{}, false
	}
	dimension := tabularDimension(rows, countColumn)
	total := 0.0
	perDimension := map[string]float64{}
	order := []string{}
	for _, row := range rows {
		value, ok := answerNumber(row[countColumn])
		if !ok {
			return resultAnswer{}, false
		}
		total += value
		if dimension != "" {
			key := defaultString(strings.TrimSpace(stringValueAny(row[dimension])), "(no value)")
			if _, seen := perDimension[key]; !seen {
				order = append(order, key)
			}
			perDimension[key] += value
		}
	}
	family := req.RecordType
	if family == "" {
		family = templatePrimaryRecordType(resp.Template)
	}
	scope := answerScopeQualifier(req)
	findings := []string{}
	headline := fmt.Sprintf("%s %s%s in total.", formatAnswerNumber(total), resultNounForFamily(family, resp, total), scope)
	if dimension != "" && len(order) > 1 {
		sort.SliceStable(order, func(i, j int) bool { return perDimension[order[i]] > perDimension[order[j]] })
		for _, key := range order {
			findings = append(findings, fmt.Sprintf("%s %s: %s", humanFieldName(dimension), key, formatAnswerNumber(perDimension[key])))
		}
		headline = fmt.Sprintf("%s %s%s across %d %s value%s. Largest: %s.", formatAnswerNumber(total), resultNounForFamily(family, resp, total), scope, len(order), humanFieldName(dimension), pluralSuffix(len(order)), joinTopEntries(findings, humanFieldName(dimension), 3))
	}
	for _, column := range tabularAmountColumns {
		if _, ok := answerNumber(rows[0][column]); !ok {
			continue
		}
		sum := 0.0
		for _, row := range rows {
			value, _ := answerNumber(row[column])
			sum += value
		}
		currency := ""
		if c := strings.TrimSpace(stringValueAny(rows[0]["currency"])); c != "" && c != "<nil>" {
			currency = c + " "
			for _, row := range rows {
				if strings.TrimSpace(stringValueAny(row["currency"])) != strings.TrimSpace(c) {
					currency = "" // mixed currencies: never sum across them silently
					sum = math.NaN()
					break
				}
			}
		}
		if !math.IsNaN(sum) {
			findings = append([]string{fmt.Sprintf("Combined %s: %s%s", humanFieldName(column), currency, formatAnswerNumber(sum))}, findings...)
		}
	}
	return resultAnswer{Headline: headline, Findings: findings}, true
}

// tabularDimension picks the categorical column a grouped table is "by".
// Columns whose names say they are categories win; timestamps, identifiers of
// provenance, review flags and numeric columns are never chosen.
func tabularDimension(rows []map[string]any, countColumn string) string {
	excluded := map[string]bool{countColumn: true, "section": true, "manual_review_required": true, "amount_role": true, "evidence_id": true, "row_hash": true, "record_id": true, "version_id": true}
	preferred := []string{"type", "status", "protocol", "domain", "direction", "camera", "location", "counterparty", "account", "plate", "site", "entity_value", "source_file", "currency"}
	candidates := []string{}
	for _, key := range sortedRowKeys(rows[0]) {
		if excluded[key] || strings.HasSuffix(key, "_seen") || strings.HasSuffix(key, "_at") || strings.Contains(key, "time") || strings.Contains(key, "date") || strings.Contains(key, "validity") {
			continue
		}
		value, isString := rows[0][key].(string)
		if !isString {
			continue
		}
		if _, numeric := answerNumber(value); numeric {
			continue
		}
		if _, err := parseAnswerTimestamp(value); err == nil {
			continue
		}
		candidates = append(candidates, key)
	}
	for _, hint := range preferred {
		for _, key := range candidates {
			if strings.Contains(key, hint) {
				return key
			}
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func parseAnswerTimestamp(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("not a timestamp")
}

func templatePrimaryRecordType(template string) string {
	for _, entry := range supportedQueryTemplates() {
		if entry.Name == template && len(entry.RecordTypes) == 1 && entry.RecordTypes[0] != "all" {
			return entry.RecordTypes[0]
		}
	}
	return ""
}

func sortedRowKeys(row map[string]any) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// decodeViaJSON converts a loosely typed value (e.g. a plan stored as a map)
// into a typed struct by round-tripping through JSON.
func decodeViaJSON(value any, out any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

func catalogFromAny(value any) []FieldDescriptorV1 {
	switch v := value.(type) {
	case []FieldDescriptorV1:
		return v
	default:
		var out []FieldDescriptorV1
		if decodeViaJSON(value, &out) == nil {
			return out
		}
	}
	return nil
}

// factPacketPlumbingMetric lists execution metrics that describe how a result
// was produced, not what the evidence says.
var factPacketPlumbingMetric = map[string]bool{
	"Template": true, "Route": true, "Planner Confidence": true, "Display Rows": true,
	"Records Row Count": true, "Provenance Items": true, "Evidence Count": true,
}

// factPacketPlumbingText recognises sentences that describe the machinery
// rather than the evidence. They are real engineering detail and they belong
// in the trace, but presented as a "fact" they are worse than useless: the
// narrator can only paraphrase them, and validateNarrative then correctly
// rejects the paraphrase, so the analyst is left with nothing.
func factPacketPlumbingText(text string) bool {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return true
	}
	if lower == "structured records row count computed." ||
		strings.HasPrefix(lower, "executed bounded source-native typed algebra") ||
		// Names the table and the query mechanics; says nothing about evidence.
		strings.HasPrefix(lower, "queried forensic.records") ||
		strings.HasPrefix(lower, "computed deterministic records result") ||
		strings.HasPrefix(lower, "retrieved registered") {
		return true
	}
	// "The analysis returned 1 exact result row." states the size of the
	// result set, never the answer that was asked for.
	if strings.HasPrefix(lower, "the analysis returned ") && strings.Contains(lower, "result row") {
		return true
	}
	for _, label := range []string{"template:", "route:", "planner confidence", "display rows", "records row count"} {
		if strings.HasPrefix(lower, label) {
			return true
		}
	}
	return false
}
