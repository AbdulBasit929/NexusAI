package main

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

const semanticFrameContractV1 = "forensics.semantic-frame/v1"

const (
	semanticRetrievalNone         = "NONE"
	semanticRetrievalExactTerm    = "EXACT_TERM"
	semanticRetrievalExactPhrase  = "EXACT_PHRASE"
	semanticRetrievalFullText     = "FULL_TEXT"
	semanticRetrievalSemantic     = "SEMANTIC"
	semanticRetrievalHybrid       = "HYBRID"
	semanticRetrievalMetadata     = "METADATA_FILTERED"
	semanticRetrievalSourceScoped = "SOURCE_SCOPED"
)

type SemanticFrameFilterV1 struct {
	FieldHint string   `json:"field_hint"`
	Operator  string   `json:"operator"`
	Literal   string   `json:"literal,omitempty"`
	Literals  []string `json:"literals,omitempty"`
}

type SemanticFrameTimeIntentV1 struct {
	Kind         string   `json:"kind"`
	From         string   `json:"from,omitempty"`
	To           string   `json:"to,omitempty"`
	NamedPeriod  string   `json:"named_period,omitempty"`
	StartSeconds *float64 `json:"start_seconds,omitempty"`
	EndSeconds   *float64 `json:"end_seconds,omitempty"`
}

type SemanticFrameConfidenceV1 struct {
	Overall      float64            `json:"overall"`
	Slots        map[string]float64 `json:"slots"`
	Degradations []string           `json:"degradations"`
}

type SemanticFrameV1 struct {
	ContractVersion   string                    `json:"contract_version"`
	RequestClass      string                    `json:"request_class"`
	FamilyHint        string                    `json:"family_hint"`
	Goal              string                    `json:"goal"`
	Measure           string                    `json:"measure"`
	MeasureFieldHint  string                    `json:"measure_field_hint,omitempty"`
	GroupByHints      []string                  `json:"group_by_hints"`
	OrderIntent       string                    `json:"order_intent"`
	LimitHint         int                       `json:"limit_hint,omitempty"`
	Filters           []SemanticFrameFilterV1   `json:"filters"`
	TimeIntent        SemanticFrameTimeIntentV1 `json:"time_intent"`
	PolarityDirection string                    `json:"polarity_direction,omitempty"`
	RetrievalMode     string                    `json:"retrieval_mode"`
	Identifiers       []HybridIdentifierV1      `json:"identifiers"`
	ExplicitScope     string                    `json:"explicit_scope"`
	Confidence        SemanticFrameConfidenceV1 `json:"confidence"`
}

var semanticFrameFieldPhrase = regexp.MustCompile(`(?i)\b(?:average|avg|sum|total|minimum|min|maximum|max)\s+([\p{L}\p{N}_ -]{1,80}?)(?:\s+(?:by|per|for|among|where|over|under|above|below)|[?.!,]|$)`)

// "Which <dimension> <verb> ..." names the grouping dimension of a ranking
// question. Two-word dimensions are limited to a closed set of second words
// so a verb is never captured as part of the dimension.
var semanticFrameWhichGroup = regexp.MustCompile(`(?i)\bwhich\s+([\p{L}\p{N}_-]{2,64}(?:\s+(?:address|number|id|code|site|type))?)\s+(?:has|have|had|shows?|contains?|made|makes|did|does|was|were|is|recorded|records|handled|handles|appears?|sent|received|accessed|used|generated)\b`)
var semanticFrameByGroup = regexp.MustCompile(`(?i)\b(?:grouped?\s+)?by\s+([\p{L}\p{N}_-]{2,64})\b`)

func semanticFrameTerms(value string) map[string]bool {
	terms := map[string]bool{}
	for _, term := range semanticWords.FindAllString(strings.ToLower(forensictext.PlanningText(value)), -1) {
		switch term {
		case "src":
			term = "source"
		case "avg":
			term = "average"
		case "mins":
			term = "minutes"
		}
		terms[semanticStem(term)] = true
	}
	return terms
}

func semanticTermsContain(terms map[string]bool, values ...string) bool {
	for _, value := range values {
		if terms[value] {
			return true
		}
	}
	return false
}

func semanticFrameFamily(req hybridQueryRequest, facts HybridFactsV1, terms map[string]bool) string {
	if family := capabilityFamilyForRecordType(normalize(req.QueryScope.SourceFamily)); family != "" {
		return family
	}
	switch normalize(facts.Family) {
	case "document", "text":
		return "document_intelligence"
	case "audio":
		return "audio_intelligence"
	case "image":
		return "image_intelligence"
	}
	if semanticTermsContain(terms, "across") && semanticTermsContain(terms, "evidence", "record", "dataset") && semanticTermsContain(terms, "family") {
		return "cross_family"
	}
	if semanticTermsContain(terms, "ready", "readiness") && semanticTermsContain(terms, "case", "investigation", "evidence", "analysis") {
		return "cross_family"
	}
	type familyTerms struct {
		family string
		terms  []string
	}
	sets := []familyTerms{
		{"network_ipdr", []string{"ipdr", "dns", "nat", "protocol", "session"}},
		{"communications_cdr", []string{"cdr", "call", "calls", "callee", "caller", "contact", "contacts", "counterparty", "communication"}},
		{"subscriber_identity", []string{"subscriber", "subscription", "iccid", "cnic"}},
		{"device_sim", []string{"imei", "imsi", "sim", "device"}},
		{"tower_location", []string{"tower", "site", "cell", "lac", "tac", "cgi", "ecgi"}},
		{"financial_transactions", []string{"financial", "transaction", "payment", "invoice", "amount", "bank"}},
		{"access_security_logs", []string{"access", "authentication", "login", "http", "security"}},
		{"generic_tabular", []string{"generic", "tabular", "spreadsheet"}},
		{"document_intelligence", []string{"document", "documents", "pdf", "docx", "passage", "passages"}},
		{"knowledge_evidence", []string{"knowledge", "rag"}},
		{"anpr_vehicles", []string{"anpr", "plate", "vehicle", "camera"}},
		{"audio_intelligence", []string{"audio", "recording", "recordings", "transcript", "speech"}},
		{"video_intelligence", []string{"video", "frame", "frames"}},
		{"image_intelligence", []string{"image", "images", "ocr"}},
		{"face_intelligence", []string{"face", "facial"}},
	}
	matches := []string{}
	for _, set := range sets {
		if semanticTermsContain(terms, set.terms...) {
			matches = append(matches, set.family)
		}
	}
	if len(matches) == 1 {
		return matches[0]
	}
	if len(matches) > 1 {
		return "cross_family"
	}
	return "none"
}

func semanticFrameGoal(terms map[string]bool, question string) string {
	switch {
	case semanticTermsContain(terms, "ready", "readiness") && semanticTermsContain(terms, "case", "investigation", "evidence", "analysis"):
		return "readiness"
	case semanticTermsContain(terms, "compare", "comparison", "versus", "difference", "overlap", "overlapping", "concurrent"):
		return "compare"
	case semanticTermsContain(terms, "timeline", "chronological", "sequence"):
		return "timeline"
	case semanticTermsContain(terms, "summarize", "summary", "overview", "status", "statuses"):
		return "summarize"
	// "distinct" and "unique" are unambiguous. "DIFFERENT" IS NOT: it modifies a
	// ranking just as often as it asks for a distinct count. Measured across the
	// 75-question corpus, exactly two questions carry it:
	//
	//	H1     "how many DIFFERENT handsets show up"        -> a distinct count
	//	CDR-07 "who talked to the MOST DIFFERENT people?"   -> a RANKING
	//
	// The `distinct` case sits ABOVE `rank` in this switch, so an unguarded
	// "different" would capture CDR-07 and hand a ranking question to S4's
	// distinct narrowing, which withdraws the row-count variant and offers no
	// grouping. WI-31 is the precedent: adding "breakdown" to the aggregate case
	// looked equally safe and produced a confident wrong answer.
	//
	// So "different" only implies a distinct count when no superlative is
	// present. The superlative list is the SAME one the rank case below uses --
	// two lists that could disagree about one sentence is the defect pattern
	// this project keeps paying for.
	case semanticTermsContain(terms, "distinct", "unique"),
		semanticTermsContain(terms, "different") &&
			!semanticTermsContain(terms, semanticRankMarkers...):
		return "distinct"
	case semanticTermsContain(terms, "exist", "exists", "whether"):
		return "existence"
	case semanticTermsContain(terms, "source") && semanticTermsContain(terms, "row", "rows", "record", "records"):
		return "source_rows"
	// REVERTED WITH ITS PAIR 2026-09-25, and never measured on its own.
	// "longest"/"shortest" were added here alongside "breakdown" in the
	// aggregate case below. The run produced one confident-wrong answer, which
	// was attributable to "breakdown" — CDR-15, "what was the longest call?",
	// stayed CORRECT throughout — but the declared threshold reverts the change,
	// not the half of it that looks guilty.
	//
	// So this remains a SEPARABLE, UNMEASURED candidate: `semanticFrameMeasure`
	// already maps "longest" to MAX, so the measure is right while the goal
	// falls to the `lookup` default and skips shape verification. Retrying it
	// requires its own run against the same threshold.
	case semanticTermsContain(terms, semanticRankMarkers...):
		return "rank"
	// A date range is TWO values, MIN and MAX of a time field — one number
	// cannot express it. Without this the question fell through to "lookup" and
	// returned a page of rows: measured 2026-09-22, "what date range do the CDR
	// records cover" projected every field over 20 rows. The curated layer has
	// always declared the capability — cdr.event_time's description says MIN/MAX
	// of it answers exactly this — so honouring it is reading the layer, not
	// special-casing a question. Both a range noun AND a time word are required,
	// so "which period had the most calls" stays a ranking.
	case semanticTermsContain(terms, "range", "period", "span") &&
		semanticTermsContain(terms, "cover", "covers", "covered", "date", "dates", "time", "times"):
		return "range"
	// MEASURED AND REVERTED 2026-09-25. "breakdown" was added here and ACC-02,
	// "show the breakdown of HTTP status codes in the access logs", went from
	// CORRECT to CONFIDENT-WRONG: it answered "There are 1,000 access log
	// entries in this case" — a total where a breakdown was asked for.
	//
	// The reason is a GAP IN THIS TAXONOMY, not a bad term choice. There is no
	// `breakdown` goal. `aggregate` with no group-by hints resolves to shape
	// `scalar` in `s9QuestionShape`, so classifying an explicit breakdown as
	// `aggregate` actively asserts it is a single number. Leaving it in the
	// `lookup` default is wrong too — that means "unrecognised" and skips shape
	// verification entirely — but it is wrong in the safe direction.
	//
	// A real fix introduces a breakdown goal that carries a grouping
	// obligation, which is a taxonomy change, not a vocabulary one.
	// See reports/tier-decision-20260924/GOAL_CLASSIFICATION_RULE.md.
	// SLICE 1 OF THE TAXONOMY FIX, behind FORENSIC_BREAKDOWN_GOAL (default off).
	// It sits HERE, above `aggregate`, because the reverted WI-31 attempt put
	// the same vocabulary INSIDE the aggregate case and that is precisely what
	// made ACC-02 answer a total: `aggregate` with no group hints resolves to
	// shape `scalar`. `breakdown` resolves to shape `breakdown`, which refuses
	// an ungrouped plan instead of asserting it is one number. See
	// breakdown_goal.go.
	case breakdownGoal(question) != "":
		return "breakdown"
	case semanticTermsContain(terms, "average", "sum", "count", "total", "minimum", "maximum", "many"):
		return "aggregate"
	case semanticTermsContain(terms, "find", "search", "mention", "mentions", "phrase", "occurrence"):
		return "search"
	case semanticTermsContain(terms, "extract"):
		return "extract"
	case strings.TrimSpace(question) == "":
		return "none"
	default:
		return "lookup"
	}
}

func semanticFrameMeasure(terms map[string]bool) string {
	switch {
	case semanticTermsContain(terms, "average"):
		return "avg"
	case semanticTermsContain(terms, "sum"):
		return "sum"
	case semanticTermsContain(terms, "minimum", "shortest", "least", "lowest"):
		return "min"
	case semanticTermsContain(terms, "maximum", "longest", "most", "highest"):
		return "max"
	case semanticTermsContain(terms, "distinct", "unique"):
		return "distinct_count"
	case semanticTermsContain(terms, "duration"):
		return "duration"
	case semanticTermsContain(terms, "bytes", "traffic"):
		return "bytes"
	case semanticTermsContain(terms, "amount", "invoice", "payment"):
		return "amount"
	case semanticTermsContain(terms, "count", "many", "often", "frequent", "activity"):
		return "record_count"
	default:
		return "none"
	}
}

func semanticFrameMeasureFieldHint(question string) string {
	match := semanticFrameFieldPhrase.FindStringSubmatch(forensictext.PlanningText(question))
	if len(match) < 2 {
		return ""
	}
	return strings.Trim(strings.Join(strings.Fields(match[1]), " "), " -_")
}

func semanticFrameGroupHints(question string, terms map[string]bool) []string {
	hints := []string{}
	for _, pattern := range []*regexp.Regexp{semanticFrameWhichGroup, semanticFrameByGroup} {
		if match := pattern.FindStringSubmatch(forensictext.PlanningText(question)); len(match) > 1 {
			hints = append(hints, strings.TrimSpace(match[1]))
		}
	}
	for _, item := range []struct {
		hint  string
		terms []string
	}{
		{"hour", []string{"hour", "hourly"}}, {"day", []string{"day", "daily"}},
		{"week", []string{"week", "weekly"}}, {"month", []string{"month", "monthly"}},
		{"counterparty", []string{"contact", "contacts", "caller", "callee", "counterparty", "talked", "talk", "spoke", "speak", "communicated"}},
		{"status", []string{"status", "statuses", "active", "inactive", "suspended"}},
		{"source", []string{"source", "file"}}, {"camera", []string{"camera", "capture"}},
	} {
		if semanticTermsContain(terms, item.terms...) {
			hints = append(hints, item.hint)
		}
	}
	if regexp.MustCompile(`(?i)\bwho\b[^?.!]{0,96}\bcall(?:ed|s)?\b`).MatchString(forensictext.PlanningText(question)) {
		hints = append(hints, "counterparty")
	}
	// "phone number(s)"/"msisdn" are analyst synonyms for the counterparty
	// grouping dimension. Without this, a case-wide rank-by-number question
	// (no target given) left GroupByHints empty and lost the structural
	// grouping-compatibility signal that should prefer frequent_contacts
	// (which asks for a target when one is missing) over an unrelated
	// CDR-family rank operation like top_locations that happened to score
	// higher on raw lexical overlap alone — a live-reproduced wrong-answer.
	if (semanticTermsContain(terms, "phone") && semanticTermsContain(terms, "number")) || semanticTermsContain(terms, "msisdn") {
		hints = append(hints, "counterparty")
	}
	return capabilityUniqueSortedStrings(hints)
}

// semanticCuratedGroupHints are the controlled-vocabulary grouping dimensions
// semanticFrameGroupHints can produce — as opposed to the raw free-text it
// also captures from "by X"/"which X" phrasing (e.g. "rank partners by event
// frequency" captures the literal string "event frequency", which describes
// a rank/sort criterion, not a real grouping dimension). Only the curated
// set is reliable enough to use as a hard rejection signal; see
// curatedGroupHints and its caller in open_ended_semantic_planner.go.
var semanticCuratedGroupHints = map[string]bool{
	"hour": true, "day": true, "week": true, "month": true,
	"counterparty": true, "status": true, "source": true, "camera": true,
}

func curatedGroupHints(hints []string) []string {
	out := make([]string, 0, len(hints))
	for _, hint := range hints {
		if semanticCuratedGroupHints[hint] {
			out = append(out, hint)
		}
	}
	return out
}

func semanticFrameOrder(terms map[string]bool) string {
	switch {
	case semanticTermsContain(terms, "most", "top", "highest", "largest", "longest"):
		return "descending"
	case semanticTermsContain(terms, "least", "lowest", "smallest", "shortest"):
		return "ascending"
	case semanticTermsContain(terms, "chronological", "timeline", "sequence"):
		return "chronological"
	default:
		return "none"
	}
}

func semanticFrameRetrievalMode(req hybridQueryRequest, facts HybridFactsV1, terms map[string]bool) string {
	if (forensictext.ExplicitScope(req.Query) == "selected_evidence" || req.QueryScope.Kind == string(EvidenceScopeSelected) || req.EvidenceID != "") && facts.TextQuery != nil {
		return semanticRetrievalSourceScoped
	}
	if facts.TextQuery != nil {
		switch facts.TextQuery.MatchSemantic {
		case forensictext.ExactPhrase:
			return semanticRetrievalExactPhrase
		case forensictext.ExactValue:
			return semanticRetrievalExactTerm
		case forensictext.PhraseContains, forensictext.TokenSearch:
			if semanticTermsContain(terms, "every", "all") {
				return semanticRetrievalFullText
			}
			return semanticRetrievalExactTerm
		}
	}
	if semanticTermsContain(terms, "semantic", "conceptual", "similar") {
		return semanticRetrievalSemantic
	}
	if semanticTermsContain(terms, "hybrid", "related", "relevant") {
		return semanticRetrievalHybrid
	}
	if semanticTermsContain(terms, "metadata") {
		return semanticRetrievalMetadata
	}
	return semanticRetrievalNone
}

func semanticFrameFilters(facts HybridFactsV1) []SemanticFrameFilterV1 {
	filters := make([]SemanticFrameFilterV1, 0, len(facts.Filters)+1)
	for _, filter := range facts.Filters {
		values := make([]string, 0, len(filter.Values))
		for _, value := range filter.Values {
			values = append(values, strings.TrimSpace(toString(value)))
		}
		filters = append(filters, SemanticFrameFilterV1{
			FieldHint: filter.Field, Operator: strings.ToUpper(filter.Op),
			Literal: strings.TrimSpace(toString(filter.Value)), Literals: values,
		})
	}
	if facts.TextQuery != nil {
		filters = append(filters, SemanticFrameFilterV1{FieldHint: "derived_text", Operator: string(facts.TextQuery.MatchSemantic), Literal: facts.TextQuery.LiteralText})
	}
	return filters
}

func semanticFrameTime(facts HybridFactsV1, terms map[string]bool) SemanticFrameTimeIntentV1 {
	intent := SemanticFrameTimeIntentV1{Kind: "none", From: facts.DateFrom, To: facts.DateTo, StartSeconds: facts.StartSeconds, EndSeconds: facts.EndSeconds}
	switch {
	case facts.StartSeconds != nil || facts.EndSeconds != nil:
		intent.Kind = "source_seconds_range"
	case facts.DateFrom != "" || facts.DateTo != "":
		intent.Kind = "explicit_range"
	case semanticTermsContain(terms, "today", "yesterday", "week", "month"):
		intent.Kind = "named_period"
		for _, period := range []string{"today", "yesterday", "week", "month"} {
			if terms[period] {
				intent.NamedPeriod = period
				break
			}
		}
	case semanticTermsContain(terms, "last", "previous", "recent"):
		intent.Kind = "relative_window"
	}
	return intent
}

func extractSemanticFrame(req hybridQueryRequest) SemanticFrameV1 {
	bound, facts := extractHybridFacts(req)
	terms := semanticFrameTerms(req.Query)
	goal := semanticFrameGoal(terms, req.Query)
	family := semanticFrameFamily(bound, facts, terms)
	scope := forensictext.ExplicitScope(req.Query)
	if scope == "" {
		scope = facts.Scope
	}
	limit := facts.TopK
	if limit == 0 && req.Limit > 0 {
		limit = clampLimit(req.Limit)
	}
	confidence := SemanticFrameConfidenceV1{Overall: 0.78, Slots: map[string]float64{}, Degradations: []string{}}
	confidence.Slots["request_class"] = 1
	confidence.Slots["goal"] = map[bool]float64{true: 0.95, false: 0.65}[goal != "lookup" && goal != "none"]
	confidence.Slots["family_hint"] = map[bool]float64{true: 0.95, false: 0.45}[family != "none"]
	confidence.Slots["identifiers"] = map[bool]float64{true: 1, false: 0.5}[len(facts.Identifiers) > 0]
	if facts.State != "" {
		confidence.Overall = 0
		confidence.Degradations = append(confidence.Degradations, facts.State)
	}
	return SemanticFrameV1{
		ContractVersion: semanticFrameContractV1, RequestClass: semanticRequestClass(bound),
		FamilyHint: family, Goal: goal, Measure: semanticFrameMeasure(terms),
		MeasureFieldHint: semanticFrameMeasureFieldHint(req.Query), GroupByHints: semanticFrameGroupHints(req.Query, terms),
		OrderIntent: semanticFrameOrder(terms), LimitHint: limit, Filters: semanticFrameFilters(facts),
		TimeIntent: semanticFrameTime(facts, terms), PolarityDirection: facts.Direction,
		RetrievalMode: semanticFrameRetrievalMode(bound, facts, terms), Identifiers: facts.Identifiers,
		ExplicitScope: scope, Confidence: confidence,
	}
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	if number, ok := value.(float64); ok {
		return strconv.FormatFloat(number, 'f', -1, 64)
	}
	return strings.TrimSpace(strings.Join(strings.Fields(stringValueAny(value)), " "))
}

func sortedSemanticFrameStrings(values []string) []string {
	out := append([]string(nil), values...)
	sort.Strings(out)
	return out
}

// semanticRankMarkers are the superlatives that make a question a RANKING.
//
// Named once and shared by the `rank` case and the `different` guard above.
// They were two literal lists until 2026-09-25; the whole point is that one
// sentence cannot be read as a ranking by one rule and a distinct count by
// another.
// "fewest"/"fewer" were absent until 2026-09-25 and the gap was found by this
// change's own control test: "which number had the FEWEST DIFFERENT contacts?"
// classified as a distinct count because no superlative vetoed it. Measured
// before adding: zero questions in the 75-question corpus carry either word, so
// nothing already measured moves.
//
// "longest"/"shortest" are deliberately NOT here. They were reverted with the
// WI-31 vocabulary change and remain a separable candidate that has never been
// measured on its own; adding them here would smuggle that in unmeasured.
var semanticRankMarkers = []string{
	"top", "most", "least", "fewest", "fewer", "highest", "lowest",
	"largest", "smallest", "greatest", "rank", "frequent",
}
