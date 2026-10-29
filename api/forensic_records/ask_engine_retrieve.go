package main

import (
	"math"
	"regexp"
	"sort"
	"strings"
)

// ASK ENGINE V2: THE SHORTLIST AND EXAMPLE RETRIEVAL (roadmap 6.8.3).
//
// The prompt must stay short, because on the CPU laptop every token is latency, so the model is shown only the views the question needs and the one or two
// verified examples closest to it. Both choices are made here by rules over the question's words, the kinds of identifier it contains and the catalogue's
// own descriptions, with no embedding and no model call. The code records that an embedding shortlist once caused two live defects in this project, so
// the rule is written first and measured against a labelled set (ask_engine_retrieval_cases.json); an embedding ranker is adopted only if it measurably
// beats it, family by family.
//
// What this is not: a router that decides to answer or not. A weak signal returns the three best views and Confident=false, and the caller may ask the
// user to clarify. The model can still abstain; the validator and checks still run on whatever it writes.

// askStopwords are dropped from a question before matching. Words that carry intent ("most", "between", "both") are NOT stopwords.
var askStopwords = map[string]bool{
	"a": true, "an": true, "the": true, "of": true, "in": true, "on": true, "at": true, "to": true, "for": true, "by": true, "with": true, "and": true,
	"or": true, "is": true, "are": true, "was": true, "were": true, "be": true, "been": true, "did": true, "do": true, "does": true, "has": true, "have": true,
	"had": true, "it": true, "its": true, "this": true, "that": true, "these": true, "those": true, "from": true, "as": true, "me": true, "my": true,
	"show": true, "list": true, "give": true, "tell": true, "what": true, "which": true, "who": true, "whom": true, "whose": true, "how": true,
	"many": true, "much": true, "any": true, "ever": true, "please": true, "can": true, "you": true, "i": true, "they": true, "them": true, "their": true,
	"there": true, "than": true, "january": true, "february": true, "march": true, "april": true, "may": true, "june": true, "july": true, "august": true,
	"september": true, "october": true, "november": true, "december": true, "local": true, "time": true, "past": true, "since": true, "during": true, "then": true, "also": true, "all": true, "each": true, "per": true, "into": true, "out": true, "up": true, "about": true,
}

// askStem is a deliberately crude stemmer: enough to match call/calls/called/calling and site/sites/websites, not a language model. Lexicon words and
// question words go through the same function, so what matters is that it is consistent.
func askStem(w string) string {
	for i := 0; i < 2; i++ { // twice, so that pairings and pairing meet
		next := askStemOnce(w)
		if next == w {
			break
		}
		w = next
	}
	return w
}

func askStemOnce(w string) string {
	rest := func(suffix string) bool { return strings.HasSuffix(w, suffix) && len(w)-len(suffix) >= 3 }
	switch {
	case rest("ies"):
		return strings.TrimSuffix(w, "ies") + "y"
	case rest("sses"), rest("xes"), rest("zes"), rest("ches"), rest("shes"):
		return strings.TrimSuffix(w, "es")
	case rest("ing"):
		return strings.TrimSuffix(w, "ing")
	case rest("ed"):
		return strings.TrimSuffix(w, "ed")
	case rest("s") && !strings.HasSuffix(w, "ss") && !strings.HasSuffix(w, "us") && !strings.HasSuffix(w, "is"):
		return strings.TrimSuffix(w, "s")
	}
	return w
}

var (
	askWordRE   = regexp.MustCompile(`[a-z]+`)
	askIPv4RE   = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)
	askDateRE   = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}(?:[ t]\d{2}:\d{2}(?::\d{2})?)?\b`)
	askDigitsRE = regexp.MustCompile(`\+?\d{9,15}`)
	askCellRE   = regexp.MustCompile(`\b[0-9a-f]{3,6}:[0-9a-f]{1,6}:[0-9a-f]{1,10}\b`)
)

// askFeatures are the identifiers found in a question.
type askFeatures struct {
	Phones, IMEIs, IPs, Dates, Cells int
}

// askNormalise lowercases a question, replaces identifiers by placeholder words (so that examples with other numbers still match) and returns the
// features found.
func askNormalise(question string) (string, askFeatures) {
	text := strings.ToLower(question)
	var f askFeatures
	text = askIPv4RE.ReplaceAllStringFunc(text, func(string) string { f.IPs++; return " ipaddr " })
	text = askCellRE.ReplaceAllStringFunc(text, func(string) string { f.Cells++; return " cellkey " })
	text = askDateRE.ReplaceAllStringFunc(text, func(string) string { f.Dates++; return " datevalue " })
	text = askDigitsRE.ReplaceAllStringFunc(text, func(s string) string {
		digits := strings.TrimPrefix(s, "+")
		if len(digits) == 15 {
			f.IMEIs++
			return " imeinumber "
		}
		f.Phones++
		return " phonenumber "
	})
	return text, f
}

// askQuestionTokens returns the stemmed, non-stop words of a normalised question.
func askQuestionTokens(normalised string) []string {
	var out []string
	for _, w := range askWordRE.FindAllString(normalised, -1) {
		if askStopwords[w] {
			continue
		}
		out = append(out, askStem(w))
	}
	return out
}

// askLexicon maps each view to the words (stems) that point at it. It is curated, not generated, so that a native speaker can extend it per language the
// way the semantic layer's synonyms are extended (roadmap 6.4). English only today; Urdu, Roman Urdu, Punjabi and Pashto lists are a later addition.
var askLexicon = map[string][]string{
	"v_contact_pairs": {"contact", "talk", "spoke", "speak", "phone", "text", "sms", "message", "common", "both", "mutual", "shared", "share", "connect",
		"connection", "path", "link", "hop", "chain", "network", "acquainted", "associate", "frequent", "often", "service", "shortcode", "hub",
		"everyone", "never", "back", "lead", "relationship", "between"},
	"v_comm_events": {"call", "sms", "text", "message", "event", "duration", "long", "longest", "minute", "hour", "day", "daily", "night", "midnight", "week", "weekday",
		"month", "date", "when", "incoming", "outgoing", "receive", "abroad", "international", "history", "conversation", "exchange", "activity", "texting"},
	"v_connection_edges":   {"connect", "connection", "path", "link", "chain", "route", "related", "relationship", "acquainted", "lead", "separation", "linked"},
	"v_evidence_inventory": {"file", "evidence", "image", "photo", "picture", "audio", "recording", "video", "document", "pdf", "inventory", "upload", "attachment", "item", "failed"},
	"v_cell_visits": {"cell", "tower", "site", "location", "where", "place", "visit", "area", "movement", "move", "travel", "near", "located", "colocation",
		"meet", "same", "sleep", "anchor", "map", "simultaneously"},
	"v_colocation_windows":   {"colocation", "together", "same", "minute", "window", "simultaneously", "near"},
	"v_meeting_candidates":   {"meet", "meeting", "met", "candidate"},
	"v_silent_numbers":       {"silent", "silence", "dormant", "inactive"},
	"v_successor_candidates": {"successor", "replace", "replacement", "takeover"},
	"v_movement_anchors":     {"anchor", "anchors"},
	"v_anchor_changes":       {"moved", "relocated", "relocation"},
	"v_anchor_deviations":    {"trip", "deviation"},
	"v_speed_alerts":         {"speed", "impossible", "implausible"},
	"v_rhythm_changes":       {"rhythm"},
	"v_cells":                {"cell", "tower", "site", "coverage", "radius", "position", "latitude", "longitude", "operator", "operated", "cover", "km", "within"},
	"v_device_pairings":      {"handset", "imei", "imsi", "sim", "device", "swap", "swapped", "change", "new", "model", "phone", "card"},
	"v_data_sessions": {"ip", "address", "port", "internet", "session", "browse", "website", "domain", "host", "server", "download", "upload", "byte", "traffic",
		"nat", "held", "public", "data", "web", "subscriber"},
	// "seen" is not here: it is in every question about who was seen with a phone, and a shortlisted view that the case cannot speak for declines the whole question (case 118, SUB-015)
	"v_plate_sightings": {"plate", "vehicle", "car", "registration", "anpr", "camera", "sight", "sighting", "licence", "license", "lane"},
	"v_plate_visits":    {"plate", "vehicle", "car", "camera", "visited", "anpr", "registration"}, // not "both": it brought this view into "which accounts did both transact with" (case 119) and declined the question
	// only words that mean a registration: subscriber, sim, imsi, after, before, active, operator and status are everywhere in call-record questions, and a shortlisted view that the case cannot
	// speak for makes the coverage guard decline the whole question
	"v_subscriber_registrations": {"registered!", "registration!", "registrations!", "unregistered!", "iccid", "activated", "activation", "deactivated", "deactivation", "suspended", "prepaid", "postpaid",
		"conflicting", "disagree", "disagrees", "owns", "owner", "owned", "ownership", "holder", "whose"},
	"v_number_registration_activity": {"registered", "registration", "registrations", "unregistered", "deactivated", "deactivation", "expired"},
	// only what separates a handset's make and model from the pairing of a number with an IMEI: the words imei, handset, device and used belong to v_device_pairings
	"v_handsets": {"model", "models", "make", "brand", "manufacturer", "tac", "samsung", "apple", "iphone", "xiaomi", "huawei", "oppo", "nokia", "vivo", "infinix", "tecno"},
	// transactions: words of a statement and a wallet; the generic words of call-record questions stay out (a shortlisted view that the case cannot speak for makes the coverage guard decline the question)
	"v_bank_transactions":      {"transaction", "transactions", "bank", "statement", "balance", "debit", "debits", "credit", "credits", "deposit", "deposits", "withdrawal", "withdrawals", "iban", "ibft", "raast", "salary", "wallet", "payment", "payments"},
	"v_account_counterparties": {"counterparty", "counterparties", "paid", "pay", "payer", "payee", "iban", "transfers", "transact", "transacted"},
	"v_account_activity":       {"balance", "daily", "closing", "credits", "debits", "bank", "statement"},
	"v_account_flows":          {"pass-through!", "passthrough!", "passed", "forwarded", "again", "followed"},
	"v_account_limit_bands":    {"limit", "structuring", "threshold"},
	// documents: only words that mean what a document names. "document" itself belongs to v_evidence_inventory ("how many documents are in the case"), and a shortlisted view that the case
	// cannot speak for makes the coverage guard decline the whole question, so the generic words of ordinary questions stay out
	// (no lexicon words for the document views: "which numbers no registration file mentions" is not about documents and the case 121 run declined it for views the case could not speak for;
	// a question about documents is recognised by the word document with a key, a kind of key, an area or a verb of naming, in askApplyFeatures)
	"v_app_usage": {"app", "application", "whatsapp", "facebook", "tiktok", "youtube", "social", "usage", "feature", "province", "city", "networking",
		"lahore", "karachi", "peshawar", "quetta", "islamabad"},
}

// askJoinPartners lists the views that are usually wanted together: a question about a cell's name or position needs the cell list too.
var askJoinPartners = map[string][]string{}

// askShortlist is the views chosen for one question, best first, with the reasons that put each there.
type askShortlist struct {
	Views     []string
	Scores    map[string]float64
	Reasons   map[string][]string
	Confident bool
}

const (
	askShortlistMax       = 3
	askShortlistRelative  = 0.45 // a view stays if it scores at least this share of the best one
	askShortlistMinScore  = 1.5
	askShortlistConfident = 2.5
	askDescribedCap       = 0.9 // the most the words of a view's own description can add
)

// viewDocument is the words of one view's own description: its name, its description, its column names and their descriptions.
func askViewTokens(v *askView) map[string]bool {
	var b strings.Builder
	b.WriteString(strings.ReplaceAll(v.Name, "_", " "))
	b.WriteString(" ")
	b.WriteString(v.Description)
	for _, c := range v.Columns {
		b.WriteString(" ")
		b.WriteString(strings.ReplaceAll(c.Name, "_", " "))
		b.WriteString(" ")
		b.WriteString(c.Description)
	}
	out := map[string]bool{}
	for _, t := range askQuestionTokens(strings.ToLower(b.String())) {
		out[t] = true
	}
	return out
}

// Shortlist ranks the catalogue's views for a question. Three signals add up: the curated lexicon (weight 2 per distinct hit), the question's words found
// in the view's own description and columns (inverse-frequency weighted, at most 1.5 each), and the kinds of identifier in the question (an address
// points at internet sessions, a 15-digit number at devices, two phone numbers with a linking word at contact pairs).
func (c *askCatalogue) Shortlist(question string, max int) askShortlist {
	if max <= 0 {
		max = askShortlistMax
	}
	normalised, features := askNormalise(question)
	tokens := askQuestionTokens(normalised)
	tokenSet := map[string]bool{}
	for _, t := range tokens {
		tokenSet[t] = true
	}

	docs := map[string]map[string]bool{}
	frequency := map[string]int{}
	for i := range c.Views {
		doc := askViewTokens(&c.Views[i])
		docs[c.Views[i].Name] = doc
		for t := range doc {
			frequency[t]++
		}
	}

	scores := map[string]float64{}
	reasons := map[string][]string{}
	add := func(view string, points float64, why string) {
		scores[view] += points
		reasons[view] = append(reasons[view], why)
	}
	for _, v := range c.Views {
		for _, word := range askLexicon[v.Name] {
			// a word marked with ! names the family so plainly that one hit puts it on the shortlist (3.5 points, not 2: at 5 it crowded the call records out of "call events after the end of a registration", case 121): "registered" in a question about the SIM an IMSI was registered on, which
			// the words imsi and sim (v_device_pairings) had outweighed (found by the subscriber judgement run on case 118: answered "no row matches" from the call records)
			points := 2.0
			if strings.HasSuffix(word, "!") {
				word, points = strings.TrimSuffix(word, "!"), 3.5
			}
			if tokenSet[askStem(word)] {
				add(v.Name, points, "word: "+word)
			}
		}
		// The words of a view's own description are a weak signal (generic words such as number and used are in most of them), so the sum is capped
		// below the shortlist's minimum score: a description alone can order views but never select one among stronger ones.
		described, why := 0.0, []string{}
		for t := range tokenSet {
			if docs[v.Name][t] {
				idf := math.Log(1 + float64(len(c.Views))/float64(frequency[t]))
				if idf > 1.5 {
					idf = 1.5
				}
				described += idf * 0.6
				why = append(why, t)
			}
		}
		if described > askDescribedCap {
			described = askDescribedCap
		}
		if described > 0 {
			sort.Strings(why)
			add(v.Name, described, "described: "+strings.Join(why, " "))
		}
	}
	askApplyFeatures(c, features, tokenSet, normalised, add)

	ranked := make([]string, 0, len(c.Views))
	for _, v := range c.Views {
		if scores[v.Name] > 0 {
			ranked = append(ranked, v.Name)
		}
	}
	// Equal scores: a view whose own name holds a word of the question comes first (a question about "pairing" prefers v_device_pairings to a view that merely describes pairs),
	// then the alphabet.
	nameHit := func(view string) bool {
		for _, part := range strings.Split(view, "_") {
			if part != "v" && tokenSet[askStem(part)] {
				return true
			}
		}
		return false
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if scores[ranked[i]] != scores[ranked[j]] {
			return scores[ranked[i]] > scores[ranked[j]]
		}
		if hi, hj := nameHit(ranked[i]), nameHit(ranked[j]); hi != hj {
			return hi
		}
		return ranked[i] < ranked[j]
	})
	out := askShortlist{Scores: scores, Reasons: reasons}
	if len(ranked) == 0 {
		return out
	}
	best := scores[ranked[0]]
	out.Confident = best >= askShortlistConfident
	for _, name := range ranked {
		if len(out.Views) >= max {
			break
		}
		if len(out.Views) > 0 && (scores[name] < askShortlistRelative*best || scores[name] < askShortlistMinScore) {
			continue
		}
		out.Views = append(out.Views, name)
	}
	for _, name := range append([]string(nil), out.Views...) {
		for _, partner := range askJoinPartners[name] {
			if c.View(partner) != nil && !askContains(out.Views, partner) {
				out.Views = append(out.Views, partner)
			}
		}
	}
	// A question about how two numbers are connected is answered from v_connection_edges alone: the contact pairs would only tempt the model to write the walk again. The common-contact
	// questions ("both", "common", "mutual", "shared") and the single-number reach questions keep their views.
	if askContains(out.Views, "v_connection_edges") && askContains(out.Views, "v_contact_pairs") && features.Phones >= 2 && askConnectionWords(tokenSet) && !askCommonWords(tokenSet) {
		kept := out.Views[:0]
		for _, name := range out.Views {
			if name != "v_contact_pairs" {
				kept = append(kept, name)
			}
		}
		out.Views = kept
	}
	// A question about a possible meeting is answered from v_meeting_candidates, which carries the windows; a question about the same place within minutes from v_colocation_windows.
	// The hourly view and the cell list would only tempt the model to join them again (stage 3a of the link-analysis bank).
	if features.Phones >= 2 && askMeetingWords(tokenSet) && askContains(out.Views, "v_meeting_candidates") {
		out.Views = askWithoutViews(out.Views, "v_cell_visits", "v_colocation_windows", "v_cells", "v_comm_events")
	} else if features.Phones >= 2 && askMinuteColocation(tokenSet) && askContains(out.Views, "v_colocation_windows") {
		out.Views = askWithoutViews(out.Views, "v_cell_visits", "v_cells", "v_comm_events")
	} else if askHourColocation(tokenSet) {
		// "How many different hours did A and B spend at the same cell site together?" counts hours of v_cell_visits; the windows of v_colocation_windows are 15-minute stretches, and
		// counting their start times answered 3 and 4 where the hours were 5 and 5 (found by the stage-3 milestone run on case 110).
		out.Views = askWithoutViews(out.Views, "v_colocation_windows")
	}
	// A question that names one of the analyses of stages 3b and 3c is answered from that view alone.
	if view := askCuesOf(normalised).view(); view != "" && askContains(out.Views, view) {
		out.Views = []string{view}
	}
	// A weak signal still gets an answer attempt, with the best three views, and says it is not confident.
	if !out.Confident && len(out.Views) < askShortlistMax {
		for _, name := range ranked {
			if len(out.Views) >= askShortlistMax {
				break
			}
			if !askContains(out.Views, name) {
				out.Views = append(out.Views, name)
			}
		}
	}
	return out
}

// askApplyFeatures adds the points the kinds of identifier in a question give.
// askDocumentKeyRE is a key as a question writes it, in lower case: an account (pk00synt...), a plate (three or four letters and four digits), or a long run of digits (a number, a serial, a handset).
var askDocumentKeyRE = regexp.MustCompile(`\bpk\d{2}\s?[a-z]{4}|\b[a-z]{3,4}[\s-]?\d{4}\b|\d{7,}|\+\d{2}[\s-]?\d{3}`)

// askMadeByRE is "made by", "manufactured by" or "produced by": the maker of a handset is in the handsets view.
var askMadeByRE = regexp.MustCompile(`\b(?:made|manufactured|produced|built)\s+by\b`)

// askDocFileRE is the name of a document file as a question writes it, and askUnregisteredRE the phrases for a number that has no registration.
var askDocFileRE = regexp.MustCompile(`\b[\w-]+\.(?:txt|docx|pdf|eml)\b`)
var askUnregisteredRE = regexp.MustCompile(`\b(?:no registrations?|not registered|unregistered|never registered|without (?:a )?registrations?|(?:have|has|had) no registrations?)\b`)

// askAccountKeyRE is an account as a question writes it, in lower case.
var askAccountKeyRE = regexp.MustCompile(`\bpk\d{2}\s?[a-z]{4}`)

// askAnyToken is whether the question has one of the words (stemmed).
func askAnyToken(tokens map[string]bool, words ...string) bool {
	for _, w := range words {
		if tokens[askStem(w)] {
			return true
		}
	}
	return false
}

func askApplyFeatures(c *askCatalogue, f askFeatures, tokens map[string]bool, normalised string, add func(string, float64, string)) {
	has := func(name string) bool { return c.View(name) != nil }
	// The phrases that name an analysis of stages 3b and 3c (ask_engine_cues.go): successor numbers, silence, a move, a trip, anchors, impossible speed, a change of rhythm.
	if view := askCuesOf(normalised).view(); view != "" && has(view) {
		add(view, 6, "the question names the analysis of "+view)
	}
	if f.IPs > 0 && has("v_data_sessions") {
		add("v_data_sessions", 3, "an IP address is named")
	}
	// An account (pk00synt...) in a question is a key of the bank views; a credit and a debit in one question are a pair (found by the transactions judgement run on case 119: the pairs
	// were looked for with a self-join on the statement lines, and "which accounts did both transact with" was shortlisted the call records)
	if askAccountKeyRE.MatchString(normalised) {
		if has("v_bank_transactions") {
			add("v_bank_transactions", 4, "an account is named")
		}
		if has("v_account_counterparties") {
			add("v_account_counterparties", 3, "an account is named")
		}
	}
	if askMadeByRE.MatchString(normalised) && askAnyToken(tokens, "handset", "phone", "device", "mobile") && has("v_handsets") {
		add("v_handsets", 6, "a handset made by a maker")
	}
	if tokens[askStem("credit")] && tokens[askStem("debit")] && has("v_account_flows") {
		add("v_account_flows", 6, "a credit and a debit in one question")
	}
	// A question about documents that names a key (a number, an account, a plate, a handset) or says one of those kinds is about what the documents NAME: the document views, not the
	// inventory of files (found by the documents judgement run on seed 5006: nine questions of the form "in which documents does X appear" were shortlisted the inventory and answered
	// from file names, six of them with a confident "no record")
	if tokens[askStem("document")] && (f.Phones > 0 || f.IMEIs > 0 || askDocumentKeyRE.MatchString(normalised) || askAnyToken(tokens, "number", "account", "plate", "mobile", "iban", "imei", "sim", "area", "town", "place", "locality", "mention", "name", "appear", "written", "contain")) {
		for _, name := range []string{"v_document_values", "v_document_mentions", "v_document_links"} {
			if has(name) {
				add(name, 6, "a question about documents names a key")
			}
		}
		if has("v_evidence_inventory") {
			add("v_evidence_inventory", -3, "the inventory lists files, not what they name")
		}
	}
	// A document named by its file, with a word for what a document holds ("what mobile numbers appear in D06-cdr-request.txt"), is a question about what the document names (found by the
	// documents run on dev seed 5100: declined, no view fitted closely enough).
	if askDocFileRE.MatchString(normalised) && askAnyToken(tokens, "number", "account", "plate", "mobile", "iban", "imei", "sim", "area", "town", "place", "mention", "name", "appear", "written", "contain", "list", "identifier") {
		for _, name := range []string{"v_document_values", "v_document_mentions"} {
			if has(name) {
				add(name, 6, "a document file is named")
			}
		}
		if has("v_evidence_inventory") {
			add("v_evidence_inventory", -3, "the inventory lists files, not what they name")
		}
	}
	// Numbers of the call records that have no registration are the call records less the registrations, which only those two views hold (the view of registration activity lists numbers that
	// HAVE a registration; found by the subscribers run on dev case 2).
	if askUnregisteredRE.MatchString(normalised) && askAnyToken(tokens, "call", "event", "record") && has("v_comm_events") && has("v_subscriber_registrations") {
		add("v_comm_events", 6, "numbers of the call records with no registration")
		add("v_subscriber_registrations", 6, "numbers of the call records with no registration")
		if has("v_number_registration_activity") {
			add("v_number_registration_activity", -4, "the activity view lists numbers that have a registration")
		}
	}
	if f.IMEIs > 0 && has("v_device_pairings") {
		add("v_device_pairings", 3, "a 15-digit identifier is named")
	}
	if f.Cells > 0 && has("v_cells") {
		add("v_cells", 2, "a cell key is named")
	}
	linking := false
	for _, w := range []string{"between", "both", "common", "mutual", "connect", "link", "path", "chain", "acquainted", "lead"} {
		if tokens[askStem(w)] {
			linking = true
		}
	}
	spatial := false
	for _, w := range []string{"place", "cell", "tower", "location", "where", "area", "map", "site"} {
		if tokens[askStem(w)] {
			spatial = true
		}
	}
	if f.Phones >= 2 && linking && !spatial && has("v_contact_pairs") {
		add("v_contact_pairs", 2, "two numbers and a linking word")
	}
	if f.Phones >= 2 && askConnectionWords(tokens) && !spatial && has("v_connection_edges") {
		add("v_connection_edges", 6, "two numbers and a connection word")
	}
	if f.Phones >= 2 && askMeetingWords(tokens) && has("v_meeting_candidates") {
		add("v_meeting_candidates", 6, "two numbers and a meeting word")
	}
	if f.Phones >= 2 && askMinuteColocation(tokens) && has("v_colocation_windows") {
		add("v_colocation_windows", 6, "two numbers, minutes and a same-place word")
	}
	if f.Phones >= 1 {
		for _, name := range []string{"v_contact_pairs", "v_comm_events"} {
			if has(name) {
				add(name, 0.5, "a phone number is named")
			}
		}
	}
}

// askConnectionWords is whether the question speaks of a connection between two things.
func askConnectionWords(tokens map[string]bool) bool {
	for _, w := range []string{"connect", "connection", "path", "link", "chain", "route", "relate", "relationship", "acquainted", "lead", "separation"} {
		if tokens[askStem(w)] {
			return true
		}
	}
	return false
}

// askMeetingWords is whether the question speaks of a possible meeting between two numbers (v_meeting_candidates, analysis A7).
func askMeetingWords(tokens map[string]bool) bool {
	for _, w := range []string{"meet", "meeting", "met"} {
		if tokens[askStem(w)] {
			return true
		}
	}
	return false
}

// askMinuteColocation is whether the question asks for two numbers at the same place within minutes (the 15-minute co-location of v_colocation_windows, analysis A5), not within
// the hour (v_cell_visits): it names minutes and a same-place word.
func askMinuteColocation(tokens map[string]bool) bool {
	if !tokens[askStem("minute")] {
		return false
	}
	for _, w := range []string{"same", "together", "place", "cell", "tower", "site", "colocation", "near", "simultaneously", "window"} {
		if tokens[askStem(w)] {
			return true
		}
	}
	return false
}

// askHourColocation is whether the question asks about hours at the same place and names no minutes: the hourly view answers it.
func askHourColocation(tokens map[string]bool) bool {
	if !tokens[askStem("hour")] || tokens[askStem("minute")] {
		return false
	}
	for _, w := range []string{"same", "together", "place", "cell", "tower", "site", "colocation", "simultaneously"} {
		if tokens[askStem(w)] {
			return true
		}
	}
	return false
}

// askWithoutViews returns the list without the named views.
func askWithoutViews(views []string, drop ...string) []string {
	out := make([]string, 0, len(views))
	for _, v := range views {
		if !askContains(drop, v) {
			out = append(out, v)
		}
	}
	return out
}

// askCommonWords is whether the question asks for what two numbers share (common contacts) rather than how they are connected.
func askCommonWords(tokens map[string]bool) bool {
	for _, w := range []string{"both", "common", "mutual", "shared", "share"} {
		if tokens[askStem(w)] {
			return true
		}
	}
	return false
}

func askContains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// askShapeCues maps words in a question to the kind of result it asks for. The cue only orders examples; it never constrains the query.
var askShapeCues = map[string][]string{
	"graph":  {"connect", "connection", "path", "link", "hop", "chain", "network", "acquainted", "lead"},
	"map":    {"where", "map", "location", "moved", "movement", "place", "tower", "cell", "same", "meet", "sleep"},
	"series": {"daily", "weekly", "monthly", "trend", "over", "weekday", "hourly", "perday"},
	"ranked": {"most", "top", "often", "frequent", "rank", "biggest", "highest"},
}

// RetrieveExamples returns up to n verified examples for the question: those whose views are in the shortlist, ordered by how close their question is
// (identifier-free words, weighted by rarity) plus a bonus when the question's cues match the example's result shape. An example whose views are not all
// in the shortlist is never returned, because the model would be shown a view it was not offered.
func askRetrieveExamples(question string, shortlist []string, examples []askExample, n int) []askExample {
	if n <= 0 || len(examples) == 0 {
		return nil
	}
	normalised, _ := askNormalise(question)
	qTokens := map[string]bool{}
	for _, t := range askQuestionTokens(normalised) {
		qTokens[t] = true
	}
	exTokens := make([]map[string]bool, len(examples))
	// A word is rare or common among the examples the question could be matched with (those on views of the shortlist), not among every example: examples on views the question was
	// never offered cannot make a word less telling for it, so adding examples for other views does not change which example is nearest.
	frequency := map[string]int{}
	candidates := make([]bool, len(examples))
	nCandidates := 0
	for i, e := range examples {
		en, _ := askNormalise(e.Question)
		exTokens[i] = map[string]bool{}
		for _, t := range askQuestionTokens(en) {
			exTokens[i][t] = true
		}
		candidates[i] = true
		for _, v := range e.Views {
			if !askContains(shortlist, v) {
				candidates[i] = false
			}
		}
		if candidates[i] {
			nCandidates++
			for t := range exTokens[i] {
				frequency[t]++
			}
		}
	}
	type scored struct {
		example askExample
		score   float64
	}
	var all []scored
	for i, e := range examples {
		if !candidates[i] {
			continue
		}
		score := 0.0
		for t := range qTokens {
			if exTokens[i][t] {
				score += math.Log(1 + float64(nCandidates)/float64(frequency[t]))
			}
		}
		for _, cue := range askShapeCues[e.Shape] {
			if qTokens[askStem(cue)] {
				score += 1.5
				break
			}
		}
		if len(shortlist) > 0 && len(e.Views) == 1 && e.Views[0] == shortlist[0] {
			score += 0.5 // with no word in common, the example for the best view is still the closest
		}
		if score > 0 {
			all = append(all, scored{e, score})
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		return all[i].example.ID < all[j].example.ID
	})
	var out []askExample
	for _, s := range all {
		if len(out) >= n {
			break
		}
		out = append(out, s.example)
	}
	return out
}
