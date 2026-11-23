package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	"github.com/mudler/LocalAI/pkg/forensictext"
)

// ANY-SOURCE TEXT SEARCH: every file whose text holds what the analyst typed, however it is written.
//
// Measured 2026-11-19 (docs/data/ask-exhaustive-search-baseline-20261119.md): on the Orion reference case 30 natural questions that ask where a number, a name, a plate, a reference or a word
// appears were fully answered in 6 of 30 (mean recall 0.29). The causes this module answers:
//
//   - A question that names no kind of source ("where does X appear?") got the clarification "Choose a source or name its evidence family" (family any has no template).
//   - Text was matched as typed, so a number written 04235761234 was not found for 042-35761234, and a plate written SYNT-۹۱۵۰ in Urdu digits was not found for SYNT9150.
//   - The answer shortened its list ("D01 and 9 other documents").
//   - A search that stopped on a budget said "text search incomplete" and listed nothing.
//
// This module reads every passage of the case's documents, image text and transcripts (within a stated budget), matches each with forensictext.MatchTier (exact / written differently /
// a close reading), groups the matches by file, and answers with ALL the files, grouped by how they matched. It says when it stopped early and why. It decides nothing about the meaning of the
// question: the text the analyst typed was already extracted by the intent step (ask_text_intent.go), and everything here is a comparison of that text with stored text.
const (
	textSearchAllEnv           = "FORENSIC_TEXT_SEARCH_ALL"
	textSearchAllPageSize      = 500
	textSearchAllMaxPages      = 400       // 200,000 passages
	textSearchAllMaxBytes      = 128 << 20 // text bytes inspected
	textSearchAllTimeout       = 25 * time.Second
	textSearchAllMaxResults    = 600 // evidence entries in the response; the table of files is never cut
	textSearchAllNamedInAnswer = 40
	textSearchAllExamples      = 3
	textSearchAllSnippetRunes  = 240
	textSearchAllGrounded      = "DERIVED_TEXT_SEARCH_ALL"
)

func textSearchAllEnabled() bool {
	return envBoolValue(os.Getenv(textSearchAllEnv), false) //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// textSearchPassage is one stored passage that matched.
type textSearchPassage struct {
	ArtifactID, ArtifactType, RunID, ObservationID, ParentID, TruthState string
	Text, Snippet                                                        string
	Locator                                                              any
	Tier                                                                 forensictext.Tier
}

// textSearchFile is everything found in one evidence file.
type textSearchFile struct {
	EvidenceID, VersionID, File string
	Kind                        string // documents, images, recordings
	Best                        forensictext.Tier
	Passages                    int
	Examples                    []textSearchPassage // the best few, best first
}

// textSearchState consumes rows (maps as read from derivedTextEvidenceSQL) and keeps the matches, grouped by file.
type textSearchState struct {
	query     forensictext.Query
	perFile   int
	files     map[string]*textSearchFile
	order     []string
	inspected int
	matched   int
	// fallback counts image-text rows that stored no raw text and were searched through their normalised text.
	fallback int
	// keep holds the rows themselves (a typed text of several words only), so that when nothing matches the whole text the parts of it can be tried without reading the case again.
	keep     bool
	rows     []map[string]any
	keptSize int
	overflow bool
}

// textSearchKeepBytes bounds the text kept for the second look at a text that matched nothing.
const textSearchKeepBytes = 64 << 20

func newTextSearchState(q forensictext.Query, perFile int) *textSearchState {
	if perFile < 1 {
		perFile = 1
	}
	return &textSearchState{query: q, perFile: perFile, files: map[string]*textSearchFile{}}
}

// addWindows matches a phrase that a recording's transcript splits across two to four consecutive segments (the existing search matched such windows, so the new one must). It runs once after the
// rows are read, on the kept rows of a typed text of several words. A window whose segments already match alone is not added again; a window that matches only together is added as one passage that
// starts at the first segment and ends at the last.
func (s *textSearchState) addWindows() {
	if !s.keep || s.overflow {
		return
	}
	groups := map[string][]map[string]any{}
	order := []string{}
	for _, row := range s.rows {
		if stringValueAny(row["artifact_type"]) != "forensics.audio-timestamp-segment/v1" {
			continue
		}
		key := stringValueAny(row["evidence_id"]) + "|" + stringValueAny(row["run_id"])
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], row)
	}
	for _, key := range order {
		segments := groups[key]
		if len(segments) < 2 {
			continue
		}
		sort.SliceStable(segments, func(i, j int) bool {
			left, _, lok := segmentTimes(segments[i])
			right, _, rok := segmentTimes(segments[j])
			return lok && rok && left < right
		})
		alone := make([]bool, len(segments))
		for i, segment := range segments {
			alone[i] = forensictext.MatchTier(stringValueAny(segment["passage_text"]), s.query) != forensictext.TierNone
		}
		for size := 2; size <= 4; size++ {
			for i := 0; i+size <= len(segments); i++ {
				texts, anyAlone := []string{}, false
				for k := i; k < i+size; k++ {
					texts = append(texts, strings.TrimSpace(stringValueAny(segments[k]["passage_text"])))
					anyAlone = anyAlone || alone[k]
				}
				joined := strings.Join(texts, " ")
				if anyAlone || forensictext.MatchTier(joined, s.query) == forensictext.TierNone {
					continue
				}
				start, _, _ := segmentTimes(segments[i])
				_, end, _ := segmentTimes(segments[i+size-1])
				window := map[string]any{}
				for field, value := range segments[i] {
					window[field] = value
				}
				window["passage_text"] = joined
				window["citation_locator"] = map[string]any{"start_seconds": start, "end_seconds": end}
				before := s.inspected
				s.add(window)
				s.inspected = before
			}
		}
	}
}

// textSearchMayRelax says whether the parts of a typed text are tried. They are when the text carries a number or code (the descriptive words around it are the likely reason it matched too little,
// and the number is distinctive on its own), and when nothing matched the whole of a text of at most two words ("IMSI values"). A longer text without a number is not taken apart: measured
// 2026-11-20, a whole Roman Urdu sentence that the model took for the search text was split into the common words "record" and "mein", which listed four unrelated files.
func textSearchMayRelax(literal string, wholeFiles int) bool {
	if textSearchHasDigit(literal) {
		return true
	}
	return wholeFiles == 0 && len(strings.Fields(literal)) <= 2
}

// textSearchPartial is a part of the typed text that some file holds although nothing holds the whole of it.
type textSearchPartial struct {
	Phrase  string
	Files   []*textSearchFile
	Matched int
}

// textSearchSubPhrase is a contiguous part of a typed text: words Start to Start+Length-1.
type textSearchSubPhrase struct {
	Phrase        string
	Start, Length int
}

// textSearchSubPhrases are the contiguous parts of a typed text, longest first, each level a list: what is left when words are taken off either end. A part must be long enough to mean something
// (four characters or more).
func textSearchSubPhrases(literal string) [][]textSearchSubPhrase {
	words := strings.Fields(literal)
	if len(words) < 2 || len(words) > 12 {
		return nil
	}
	levels := [][]textSearchSubPhrase{}
	for length := len(words) - 1; length >= 1; length-- {
		level := []textSearchSubPhrase{}
		for start := 0; start+length <= len(words); start++ {
			phrase := strings.Join(words[start:start+length], " ")
			if len([]rune(phrase)) >= 4 {
				level = append(level, textSearchSubPhrase{Phrase: phrase, Start: start, Length: length})
			}
		}
		if len(level) > 0 {
			levels = append(levels, level)
		}
	}
	return levels
}

// relaxed looks for parts of the typed text in the rows kept by the first pass, used only when nothing holds the whole of it.
//   - When the text carries a number or code (a word with a digit), the descriptive words around it ("vehicle number", "bank account") are the likely reason nothing matched, so every part that still
//     holds ALL the digit words is tried, longest first, and a file is listed once, under the longest part it holds. A part must be six characters or more, so a bare year is never a part.
//   - When it carries none, only the longest level that matches anything is reported.
//
// A file that holds a part only as a close reading is not listed: a part is a weaker claim than the whole, and a reading error on top of it is too weak to show.
func (s *textSearchState) relaxed(already map[string]bool) []textSearchPartial {
	if !s.keep || s.overflow || len(s.rows) == 0 {
		return nil
	}
	words := strings.Fields(s.query.LiteralText)
	first, last := -1, -1
	for index, word := range words {
		if textSearchHasDigit(word) {
			if first < 0 {
				first = index
			}
			last = index
		}
	}
	listed := map[string]bool{}
	for key := range already { // files that hold the whole text are not listed again as holding a part
		listed[key] = true
	}
	found := []textSearchPartial{}
	for _, level := range textSearchSubPhrases(s.query.LiteralText) {
		for _, part := range level {
			if first >= 0 && (part.Start > first || part.Start+part.Length-1 < last || len([]rune(part.Phrase)) < 6) {
				continue
			}
			query := s.query
			query.LiteralText = part.Phrase
			sub := newTextSearchState(query, s.perFile)
			for _, row := range s.rows {
				sub.add(row)
			}
			files := []*textSearchFile{}
			for _, file := range sub.ordered() {
				key := file.EvidenceID + "|" + file.File
				if listed[key] || file.Best == forensictext.TierApproximate {
					continue
				}
				listed[key] = true
				files = append(files, file)
			}
			if len(files) > 0 {
				found = append(found, textSearchPartial{Phrase: part.Phrase, Files: files, Matched: sub.matched})
			}
		}
		if first < 0 && len(found) > 0 {
			return found
		}
	}
	return found
}

// textSearchKind is the plain-language kind of the file a passage came from.
func textSearchKind(artifactType string) (kind string, rank int) {
	switch artifactType {
	case "forensics.document-native-text-passage/v1":
		return "documents", 0
	case "forensics.image-ocr-observation/v1":
		return "images", 1
	case "forensics.audio-timestamp-segment/v1", "forensics.audio-roman-urdu-segment/v1":
		return "recordings", 2
	}
	return "other files", 3
}

func textSearchKindRank(kind string) int {
	switch kind {
	case "documents":
		return 0
	case "images":
		return 1
	case "recordings":
		return 2
	}
	return 3
}

// add compares one stored passage with the query and records it when it matches.
func (s *textSearchState) add(row map[string]any) {
	s.inspected++
	if s.keep && !s.overflow {
		s.keptSize += len(stringValueAny(row["passage_text"]))
		if s.keptSize > textSearchKeepBytes {
			s.overflow, s.rows = true, nil
		} else {
			s.rows = append(s.rows, row)
		}
	}
	artifactType := stringValueAny(row["artifact_type"])
	roman := artifactType == "forensics.audio-roman-urdu-segment/v1"
	if s.query.Representation == "raw" && roman || s.query.Representation == "roman_derivative" && !roman {
		return
	}
	text := strings.TrimSpace(stringValueAny(row["passage_text"]))
	if text == "" {
		return
	}
	tier := forensictext.MatchTier(text, s.query)
	if tier == forensictext.TierNone {
		return
	}
	// Text typed into a document has no reading errors: a number one digit off there is another number (a call list holds hundreds of them), not a misreading, so it is not offered as a close
	// reading. Text a model read (image text, transcripts) can be misread, and a close reading of it stays.
	if tier == forensictext.TierApproximate && artifactType == "forensics.document-native-text-passage/v1" && textSearchHasDigit(s.query.LiteralText) {
		return
	}
	s.matched++
	if row["raw_text_unavailable"] == true {
		s.fallback++
	}
	evidenceID := strings.TrimSpace(stringValueAny(row["evidence_id"]))
	file := stringValueAny(row["source_file"])
	key := evidenceID + "|" + file
	entry := s.files[key]
	if entry == nil {
		kind, _ := textSearchKind(artifactType)
		entry = &textSearchFile{EvidenceID: evidenceID, VersionID: stringValueAny(row["version_id"]), File: file, Kind: kind}
		s.files[key] = entry
		s.order = append(s.order, key)
	}
	entry.Passages++
	if tier.Rank() > entry.Best.Rank() {
		entry.Best = tier
	}
	entry.Examples = append(entry.Examples, textSearchPassage{
		ArtifactID: stringValueAny(row["artifact_id"]), ArtifactType: artifactType, RunID: stringValueAny(row["run_id"]), ObservationID: stringValueAny(row["observation_id"]),
		ParentID: stringValueAny(row["parent_artifact_id"]), TruthState: stringValueAny(row["source_truth_state"]),
		Text: text, Snippet: textSearchSnippet(text, s.query.LiteralText), Locator: row["citation_locator"], Tier: tier,
	})
	// Keep the best few: more than twice the allowance is trimmed as it grows, so a file with thousands of matching passages costs a handful of entries.
	if len(entry.Examples) > 2*s.perFile+2 {
		trimTextSearchExamples(entry, s.perFile)
	}
}

func textSearchHasDigit(value string) bool {
	for _, r := range forensictext.Fold(value) {
		if unicode.IsDigit(r) {
			return true
		}
	}
	return false
}

func trimTextSearchExamples(entry *textSearchFile, keep int) {
	sort.SliceStable(entry.Examples, func(i, j int) bool { return entry.Examples[i].Tier.Rank() > entry.Examples[j].Tier.Rank() })
	if len(entry.Examples) > keep {
		entry.Examples = entry.Examples[:keep]
	}
}

// ordered returns the matched files: best match first, then documents, images, recordings, then by name.
func (s *textSearchState) ordered() []*textSearchFile {
	out := make([]*textSearchFile, 0, len(s.order))
	for _, key := range s.order {
		entry := s.files[key]
		trimTextSearchExamples(entry, s.perFile)
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Best.Rank() != out[j].Best.Rank() {
			return out[i].Best.Rank() > out[j].Best.Rank()
		}
		if ki, kj := textSearchKindRank(out[i].Kind), textSearchKindRank(out[j].Kind); ki != kj {
			return ki < kj
		}
		return out[i].File < out[j].File
	})
	return out
}

// textSearchSnippet is the part of a passage around the first place the literal (or its nearest written form) appears, never more than textSearchAllSnippetRunes.
func textSearchSnippet(text, literal string) string {
	runes := []rune(strings.Join(strings.Fields(text), " "))
	if len(runes) <= textSearchAllSnippetRunes {
		return string(runes)
	}
	at := textSearchLocate(runes, literal)
	start := max(0, at-textSearchAllSnippetRunes/3)
	end := min(len(runes), start+textSearchAllSnippetRunes)
	start = max(0, end-textSearchAllSnippetRunes)
	snippet := string(runes[start:end])
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(runes) {
		snippet += "..."
	}
	return snippet
}

// textSearchLocate is the rune offset of the first word of the text that is the literal's first word once both are folded (see forensictext.Fold), or within one edit of it. It is used only to
// choose which part of a long passage to show; the match itself was decided by forensictext.MatchTier.
func textSearchLocate(runes []rune, literal string) int {
	words := strings.FieldsFunc(forensictext.Fold(literal), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	if len(words) == 0 {
		return 0
	}
	first := []rune(words[0])
	type textWord struct {
		at     int
		folded []rune
	}
	found := []textWord{}
	start := -1
	for i := 0; i <= len(runes); i++ {
		inWord := i < len(runes) && (unicode.IsLetter(runes[i]) || unicode.IsNumber(runes[i]) || unicode.IsMark(runes[i]))
		switch {
		case inWord && start < 0:
			start = i
		case !inWord && start >= 0:
			found = append(found, textWord{at: start, folded: []rune(forensictext.Fold(string(runes[start:i])))})
			start = -1
		}
	}
	for _, w := range found { // the word as the literal has it
		if string(w.folded) == string(first) {
			return w.at
		}
	}
	for _, w := range found { // the nearest written form: a long common beginning (a number written without its hyphens, a word with a reading error further on)
		common := 0
		for common < len(w.folded) && common < len(first) && w.folded[common] == first[common] {
			common++
		}
		if common >= 3 && common*2 >= len(first) {
			return w.at
		}
	}
	return 0
}

// textSearchTierLabel says how a file matched, in words an analyst reads.
func textSearchTierLabel(t forensictext.Tier) string {
	switch t {
	case forensictext.TierExact:
		return "as typed"
	case forensictext.TierNormalised:
		return "written differently"
	case forensictext.TierApproximate:
		return "close reading"
	}
	return ""
}

// textSearchFileName is a file with the moment of its first match in a recording ("talk.wav (at 0:12)").
func textSearchFileName(f *textSearchFile) string {
	if f.Kind == "recordings" && len(f.Examples) > 0 {
		if locator := decodedCitationLocator(f.Examples[0].Locator); locator != nil {
			if seconds, ok := transcriptSeconds(locator["start_seconds"]); ok {
				return fmt.Sprintf("%s (at %s)", f.File, plateClock(seconds))
			}
		}
	}
	return f.File
}

// textSearchListByKind names files grouped by kind. It names at most *budget files in all and says how many were left to the table.
func textSearchListByKind(files []*textSearchFile, budget *int) string {
	kinds := []string{}
	byKind := map[string][]*textSearchFile{}
	for _, f := range files {
		if _, ok := byKind[f.Kind]; !ok {
			kinds = append(kinds, f.Kind)
		}
		byKind[f.Kind] = append(byKind[f.Kind], f)
	}
	sort.SliceStable(kinds, func(i, j int) bool { return textSearchKindRank(kinds[i]) < textSearchKindRank(kinds[j]) })
	parts := []string{}
	for _, kind := range kinds {
		list := byKind[kind]
		names := []string{}
		for _, f := range list {
			if *budget <= 0 {
				break
			}
			names = append(names, textSearchFileName(f))
			*budget--
		}
		part := fmt.Sprintf("%s (%d): %s", kind, len(list), strings.Join(names, ", "))
		if len(names) < len(list) {
			part += fmt.Sprintf(" and %d more", len(list)-len(names))
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "; ")
}

// textSearchScope names the kinds of source a search covered, from the contracts it read: "document, image or recording text", or just "document text" when the question named documents.
func textSearchScope(contracts []string) string {
	kinds := []string{}
	add := func(kind string) {
		if !containsString(kinds, kind) {
			kinds = append(kinds, kind)
		}
	}
	for _, contract := range contracts {
		switch contract {
		case "forensics.document-native-text-passage/v1":
			add("document")
		case "forensics.image-ocr-observation/v1":
			add("image")
		case "forensics.audio-timestamp-segment/v1", "forensics.audio-roman-urdu-segment/v1":
			add("recording")
		}
	}
	order := map[string]int{"document": 0, "image": 1, "recording": 2}
	sort.SliceStable(kinds, func(i, j int) bool { return order[kinds[i]] < order[kinds[j]] })
	switch len(kinds) {
	case 0:
		return "text"
	case 1:
		return kinds[0] + " text"
	}
	return strings.Join(kinds[:len(kinds)-1], ", ") + " or " + kinds[len(kinds)-1] + " text"
}

// textSearchKindsOf is the kinds of source (documents, images, recordings) the contracts stand for.
func textSearchKindsOf(contracts []string) []string {
	kinds := []string{}
	for _, contract := range contracts {
		if kind, _ := textSearchKind(contract); kind != "other files" && !containsString(kinds, kind) {
			kinds = append(kinds, kind)
		}
	}
	return kinds
}

// textSearchOutcome is what a search found and how far it got.
type textSearchOutcome struct {
	Scope            string   // what the question named: "document text", or every kind
	AskedKinds       []string // the kinds of source the question named (documents, images, recordings); all three when it named none
	Files            []*textSearchFile
	Partial          []textSearchPartial // parts of the typed text, only when nothing holds the whole of it
	Inspected        int
	Matched          int
	Fallback         int
	Complete         bool
	IncompleteReason string
}

// textSearchTierLists names ALL the files of a group, by how they matched (as typed, written differently, close reading) and, inside each, by kind of source. It names at most *budget files in all.
func textSearchTierLists(files []*textSearchFile, budget *int) string {
	var b strings.Builder
	for _, tier := range []forensictext.Tier{forensictext.TierExact, forensictext.TierNormalised, forensictext.TierApproximate} {
		group := []*textSearchFile{}
		for _, f := range files {
			if f.Best == tier {
				group = append(group, f)
			}
		}
		if len(group) == 0 {
			continue
		}
		lead := map[forensictext.Tier]string{
			forensictext.TierExact:       " As typed in ",
			forensictext.TierNormalised:  " Written differently (other digits, separators or capitals) in ",
			forensictext.TierApproximate: " A close reading, one or two characters different and possibly a reading error, so check it by eye, in ",
		}[tier]
		b.WriteString(lead + textSearchListByKind(group, budget) + ".")
	}
	return b.String()
}

// textSearchAllSentence is the answer: the typed text, how many files hold it, and ALL the files grouped by how they matched. Every kind of source is searched whatever the question named; the kind it
// named comes first and anything found in another kind is added, so a question that names the wrong kind (a name asked about "in recordings" that is written in a document) still finds it.
func textSearchAllSentence(literal string, o textSearchOutcome) string {
	var b strings.Builder
	allScope := textSearchScope(searchableDerivedTextContracts)
	asked, other := []*textSearchFile{}, []*textSearchFile{}
	for _, f := range o.Files {
		if len(o.AskedKinds) == 0 || len(o.AskedKinds) >= 3 || containsString(o.AskedKinds, f.Kind) {
			asked = append(asked, f)
		} else {
			other = append(other, f)
		}
	}
	budget := textSearchAllNamedInAnswer
	switch {
	case len(o.Files) == 0 && len(o.Partial) > 0:
		fmt.Fprintf(&b, "No %s holds %q as typed, written with other digits, separators, letter forms or capitals, or one or two characters off.", allScope, literal)
		for _, part := range o.Partial {
			fmt.Fprintf(&b, " Part of it, %q, is written in %d %s:%s", part.Phrase, len(part.Files), plural(len(part.Files), "file", "files"), textSearchTierLists(part.Files, &budget))
		}
		b.WriteString(" These hold only part of what you typed, so check that they are what you meant.")
	case len(o.Files) == 0:
		fmt.Fprintf(&b, "No %s holds %q as typed, written with other digits, separators, letter forms or capitals, or one or two characters off. %d passages were searched.", allScope, literal, o.Inspected)
	case len(asked) == 0:
		fmt.Fprintf(&b, "No %s holds %q as typed or written differently, but it appears in %d %s of another kind (%d %s).", o.Scope, literal, len(other), plural(len(other), "file", "files"), o.Matched, plural(o.Matched, "passage", "passages"))
		b.WriteString(textSearchTierLists(other, &budget))
	default:
		fmt.Fprintf(&b, "%q appears in %d %s (%d %s).", literal, len(o.Files), plural(len(o.Files), "file", "files"), o.Matched, plural(o.Matched, "passage", "passages"))
		b.WriteString(textSearchTierLists(asked, &budget))
		if len(other) > 0 {
			b.WriteString(" Also in other kinds of source:" + textSearchTierLists(other, &budget))
		}
	}
	if len(o.Files) > 0 && len(o.Partial) > 0 {
		for _, part := range o.Partial {
			fmt.Fprintf(&b, " Part of what you typed, %q, is also written in %d more %s:%s", part.Phrase, len(part.Files), plural(len(part.Files), "file", "files"), textSearchTierLists(part.Files, &budget))
		}
		b.WriteString(" Those hold only part of what you typed.")
	}
	if (len(o.Files) > 0 || len(o.Partial) > 0) && budget <= 0 {
		b.WriteString(" The table lists every file.")
	}
	if !o.Complete {
		fmt.Fprintf(&b, " The search stopped early (%s), so other files may hold it.", o.IncompleteReason)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// textSearchAllFetch reads every searchable passage of the case under the tenant's row-level security, page by page, and feeds the rows to the state. It stops at a budget and says which.
func textSearchAllFetch(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, contracts []string, state *textSearchState) (complete bool, reason string, err error) {
	ctx, cancel := context.WithTimeout(ctx, textSearchAllTimeout)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return false, "", err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id',$1,true)", req.TenantID); err != nil {
		return false, "", err
	}
	bytes := 0
	for page := 0; page < textSearchAllMaxPages; page++ {
		cursor, err := tx.Query(ctx, derivedTextEvidenceSQL, req.TenantID, req.CollectionID, contracts, "", "", page*textSearchAllPageSize, []string{}, []string{}, []string{})
		if err != nil {
			if ctx.Err() != nil {
				return false, "it ran out of time", nil
			}
			return false, "", err
		}
		rows, err := rowsToMaps(cursor)
		cursor.Close()
		if err != nil {
			if ctx.Err() != nil {
				return false, "it ran out of time", nil
			}
			return false, "", err
		}
		more := len(rows) > textSearchAllPageSize
		if more {
			rows = rows[:textSearchAllPageSize]
		}
		for _, row := range rows {
			bytes += len(stringValueAny(row["passage_text"]))
			if bytes > textSearchAllMaxBytes {
				return false, fmt.Sprintf("it reached the %d MB limit on text read", textSearchAllMaxBytes>>20), nil
			}
			state.add(row)
		}
		if !more {
			return true, "", nil
		}
	}
	return false, fmt.Sprintf("it reached the limit of %d passages", textSearchAllMaxPages*textSearchAllPageSize), nil
}

// textSearchAllEvidence builds the response's evidence block in the shape the governed text search uses, so citations, the evidence panel and the source viewers work unchanged.
func textSearchAllEvidence(req hybridQueryRequest, o textSearchOutcome) map[string]any {
	perFile := textSearchAllExamples
	if len(o.Files) > 100 {
		perFile = 1
	}
	results := []map[string]any{}
	type evidenceFile struct {
		file   *textSearchFile
		phrase string // set for a file that holds only part of the typed text
	}
	listed := []evidenceFile{}
	for _, f := range o.Files {
		listed = append(listed, evidenceFile{file: f})
	}
	for _, part := range o.Partial {
		for _, f := range part.Files {
			listed = append(listed, evidenceFile{file: f, phrase: part.Phrase})
		}
	}
	for _, item := range listed {
		f := item.file
		for index, ex := range f.Examples {
			if index >= perFile || len(results) >= textSearchAllMaxResults {
				break
			}
			locatorText := marshalJSONString(ex.Locator)
			metadata := map[string]any{
				"tenant_id": req.TenantID, "collection_id": req.CollectionID, "evidence_id": f.EvidenceID, "version_id": f.VersionID, "source_file": f.File,
				"source_locator": locatorText, "citation_locator": ex.Locator, "source_family": derivedTextSourceFamily(ex.ArtifactType), "artifact_id": ex.ArtifactID,
				"artifact_type": ex.ArtifactType, "parent_artifact_id": ex.ParentID, "raw_text": derivedTextPreview(ex.Text, 2000), "observation_id": ex.ObservationID,
				"run_id": ex.RunID, "match_semantic": string(req.TextQuery.MatchSemantic), "text_representation": "raw", "match_tier": string(ex.Tier), "match_snippet": ex.Snippet,
				"search_normalization": forensictext.NormalizationV1 + "+tolerant-v1",
			}
			if ex.ArtifactType == "forensics.audio-roman-urdu-segment/v1" {
				metadata["text_representation"] = "roman_derivative"
			}
			if item.phrase != "" {
				metadata["matched_part_of_typed_text"] = item.phrase
			}
			if state := citationTruthState(ex.TruthState); state != "" {
				metadata["source_truth_state"] = state
			}
			if locator := decodedCitationLocator(ex.Locator); locator != nil {
				metadata["start_seconds"], metadata["end_seconds"] = locator["start_seconds"], locator["end_seconds"]
			}
			results = append(results, map[string]any{
				"id": ex.ArtifactID, "content": derivedTextPreview(ex.Text, 2000), "preview": derivedTextPreview(ex.Text, 2000), "score": float64(ex.Tier.Rank()) / 3,
				"citation": fmt.Sprintf("nexusai://evidence/%s/artifacts/%s", f.EvidenceID, ex.ArtifactID), "metadata": metadata,
			})
		}
	}
	state, completeness := "COMPLETE_RESULTS", "EXHAUSTIVE"
	if len(o.Files) == 0 {
		state = "NO_EXACT_MATCH" // a part of the typed text is not a match of the typed text
	}
	if !o.Complete {
		state, completeness = "SEARCH_INCOMPLETE", "INCOMPLETE"
	}
	return map[string]any{
		"candidate_scope": "completed_current_version_observations", "search_completeness": completeness, "row_count": len(results), "mode": "governed_derived_text_all_sources",
		"query_mode": string(req.TextQuery.MatchSemantic), "exact_term": req.TextQuery.LiteralText, "result_state": state, "results": results,
		"retrieval_mode": "derived_text_tolerant_all_sources", "candidates_inspected": o.Inspected, "files_matched": len(o.Files), "passages_matched": o.Matched,
	}
}

// textSearchAllRows is one table row per matched file: every file, never shortened.
func textSearchAllRows(o textSearchOutcome) []map[string]any {
	rows := make([]map[string]any, 0, len(o.Files))
	for _, f := range o.Files {
		snippet := ""
		if len(f.Examples) > 0 {
			snippet = f.Examples[0].Snippet
		}
		rows = append(rows, map[string]any{"file": f.File, "kind": f.Kind, "match": textSearchTierLabel(f.Best), "passages": f.Passages, "first_match": snippet})
	}
	for _, part := range o.Partial {
		for _, f := range part.Files {
			snippet := ""
			if len(f.Examples) > 0 {
				snippet = f.Examples[0].Snippet
			}
			rows = append(rows, map[string]any{"file": f.File, "kind": f.Kind, "match": "part of what you typed: " + part.Phrase, "passages": f.Passages, "first_match": snippet})
		}
	}
	return rows
}

// textSearchAllRun answers a text search over the text of every kind of source at once. It returns (response, true) when it answered; on any failure it returns false and the question takes the
// path it would have taken without it.
func textSearchAllRun(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, startedAt time.Time, contracts []string) (hybridQueryResponse, bool) {
	if db == nil || req.TenantID == "" || req.CollectionID == "" || req.TextQuery == nil || req.TextQuery.Validate() != nil {
		return hybridQueryResponse{}, false
	}
	switch req.TextQuery.MatchSemantic {
	case forensictext.ExactValue, forensictext.ExactPhrase, forensictext.PhraseContains, forensictext.TokenSearch:
	default:
		return hybridQueryResponse{}, false
	}
	if req.StartSeconds != nil || req.EndSeconds != nil || req.EvidenceID != "" || req.EvidenceVersionID != "" || documentComparisonRequested(req) {
		return hybridQueryResponse{}, false
	}
	if len(contracts) == 0 {
		contracts = searchableDerivedTextContracts
	}
	// Every kind of source is searched; the kind the question named (the contracts passed) is reported first.
	state := newTextSearchState(*req.TextQuery, textSearchAllExamples)
	state.keep = len(strings.Fields(req.TextQuery.LiteralText)) >= 2
	complete, reason, err := textSearchAllFetch(ctx, db, req, searchableDerivedTextContracts, state)
	if err != nil {
		return hybridQueryResponse{}, false
	}
	state.addWindows()
	askedKinds := textSearchKindsOf(contracts)
	ordered := state.ordered()
	var partial []textSearchPartial
	if complete && textSearchMayRelax(req.TextQuery.LiteralText, len(ordered)) {
		// Nothing holds the whole text, or the text carries a number or code that other files write without all the words typed around it (a "helpline" that another file calls a "landline"): the parts
		// of it that other files hold, labelled as parts.
		already := map[string]bool{}
		for _, f := range ordered {
			already[f.EvidenceID+"|"+f.File] = true
		}
		partial = state.relaxed(already)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return containsString(askedKinds, ordered[i].Kind) && !containsString(askedKinds, ordered[j].Kind)
	})
	outcome := textSearchOutcome{Scope: textSearchScope(contracts), AskedKinds: askedKinds, Files: ordered, Partial: partial, Inspected: state.inspected, Matched: state.matched, Fallback: state.fallback, Complete: complete, IncompleteReason: reason}
	text := textSearchAllSentence(req.TextQuery.LiteralText, outcome)
	evidence := textSearchAllEvidence(req, outcome)
	limitation := "This search covers the " + textSearchScope(searchableDerivedTextContracts) + " of the case. Tables loaded as records (spreadsheets, call records) are not part of it. " +
		"Matches written differently or close readings are listed apart from exact ones and need a look by eye."
	if outcome.Fallback > 0 {
		limitation += " Some image text was searched through its normalised form because the raw reading was not stored."
	}
	if !complete {
		limitation += " The search stopped before it covered all the text of the case."
	}
	completeness := stringValueAny(evidence["search_completeness"])
	answer := map[string]any{
		"result_state": "COMPLETED", "answer": text, "grounding": textSearchAllGrounded, "case_specific_facts": []any{},
		"text_search":         map[string]any{"files_matched": len(outcome.Files), "passages_matched": outcome.Matched, "passages_searched": outcome.Inspected, "search_complete": complete},
		"search_completeness": completeness, "evidence_status": strings.ToLower(stringValueAny(evidence["result_state"])),
	}
	enterprise := terminalEnterprisePayload(req, forensicrequest.GovernedAnalysis, answer)
	enterprise["summary"], enterprise["limitations"] = limitation, []string{limitation}
	enterprise["executive_answer"] = text
	enterprise["result_state"] = evidence["result_state"]
	enterprise["data_grid"] = enterpriseDataGrid(textSearchAllRows(outcome))
	enterprise["row_count"] = len(textSearchAllRows(outcome))
	enterprise["provenance"] = enterpriseProvenance(req, nil, evidence, nil)
	enterprise["operation"] = map[string]any{"template": "text_search_all", "operation_id": "forensics.text_search_all", "family_id": "derived_text", "source_access": "derived_text"}
	enterprise["coverage"] = map[string]any{"tenant_id": req.TenantID, "collection_id": req.CollectionID, "route": []string{"text_search_all"}, "queried_record_sql": false, "queried_kb": false,
		"passages_searched": outcome.Inspected, "search_complete": complete}
	return hybridQueryResponse{
		CollectionID: req.CollectionID, RequestClass: forensicrequest.GovernedAnalysis, Intent: intentRecords, Target: req.TextQuery.LiteralText,
		Route: []string{"text_search_all"}, Policy: "derived_text_search",
		Planner:   map[string]any{"route": "text_search_all", "match_semantic": string(req.TextQuery.MatchSemantic), "contracts": len(searchableDerivedTextContracts), "asked": strings.Join(askedKinds, ",")},
		QueryPlan: QueryPlan{}, Capability: queryCapabilityAssessment{Status: "operational"},
		Evidence: evidence, Answer: answer, GeneratedAt: time.Now().UTC(), Enterprise: enterprise,
		Telemetry: QueryTelemetry{TotalLatencyMS: time.Since(startedAt).Milliseconds()},
	}, true
}

// textSearchAddMentions adds to an answer that was produced another way the files whose text holds a value. The answer, its evidence and its citations are kept as they are; this appends one sentence
// (lead, then every file grouped by how it matched), the matching passages as further evidence entries and their citations. It adds nothing when the value is found nowhere, so an answer that was right
// is never cluttered, and it returns the answer unchanged when the search cannot run.
func textSearchAddMentions(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, answered hybridQueryResponse, literal string, contracts []string, lead, tail string) hybridQueryResponse {
	if db == nil || strings.TrimSpace(literal) == "" || req.TenantID == "" || req.CollectionID == "" {
		return answered
	}
	query := forensictext.Query{LiteralText: literal, MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}
	if query.Validate() != nil {
		return answered
	}
	state := newTextSearchState(query, textSearchAllExamples)
	complete, reason, err := textSearchAllFetch(ctx, db, req, contracts, state)
	if err != nil {
		return answered
	}
	files := state.ordered()
	if len(files) == 0 && complete {
		return answered
	}
	textReq := req
	textReq.TextQuery = &query
	outcome := textSearchOutcome{Scope: textSearchScope(contracts), AskedKinds: textSearchKindsOf(contracts), Files: files, Inspected: state.inspected, Matched: state.matched, Complete: complete, IncompleteReason: reason}
	budget := textSearchAllNamedInAnswer
	extra := ""
	if len(files) > 0 {
		extra = fmt.Sprintf("%s %d %s:%s%s", lead, len(files), plural(len(files), "file", "files"), textSearchTierLists(files, &budget), tail)
	}
	if !complete {
		extra += fmt.Sprintf(" The search of the text stopped early (%s).", reason)
	}
	extra = strings.Join(strings.Fields(extra), " ")
	if extra == "" {
		return answered
	}
	evidence := textSearchAllEvidence(textReq, outcome)
	if answered.Answer != nil {
		answered.Answer["text_mentions"] = map[string]any{"files": len(files), "passages_searched": state.inspected, "search_complete": complete}
	}
	if answered.Enterprise != nil {
		answered.Enterprise["executive_answer"] = strings.TrimSpace(stringValueAny(answered.Enterprise["executive_answer"]) + " " + extra)
		provenance := enterpriseProvenance(textReq, nil, evidence, nil)
		switch existing := answered.Enterprise["provenance"].(type) {
		case []map[string]any:
			answered.Enterprise["provenance"] = append(existing, provenance...)
		case nil:
			answered.Enterprise["provenance"] = provenance
		}
		// the table of the answer is kept; an answer with no table (the plate search) gets the table of files
		if grid, ok := answered.Enterprise["data_grid"].(map[string]any); !ok || grid == nil || stringValueAny(grid["count"]) == "0" || grid["count"] == nil {
			answered.Enterprise["data_grid"] = enterpriseDataGrid(textSearchAllRows(outcome))
		}
	}
	if answered.Evidence == nil {
		answered.Evidence = map[string]any{"retrieval_mode": "derived_text_tolerant_all_sources"}
	}
	merged := append([]map[string]any{}, evidenceResults(answered.Evidence)...)
	merged = append(merged, evidenceResults(evidence)...)
	answered.Evidence["results"] = merged
	answered.Evidence["row_count"] = len(merged)
	return answered
}

// textSearchPlateMentions adds to a plate-in-images answer the documents and recordings that WRITE the plate (a statement in Urdu digits, a chat, a report). The plate readings of images and video
// were searched by plateImageSearchRun and stay as they are; this adds typed text and transcripts only, which the plate search never looked at (measured 2026-11-19: a witness statement that writes
// the plate as SYNT-<Urdu digits> was missing from the answer).
func textSearchPlateMentions(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, answered hybridQueryResponse, plate string) hybridQueryResponse {
	contracts := []string{"forensics.document-native-text-passage/v1", "forensics.audio-timestamp-segment/v1", "forensics.audio-roman-urdu-segment/v1"}
	merged := textSearchAddMentions(ctx, db, req, answered, plate, contracts, " The plate is also written in", " (typed text and transcripts, not image or video readings)")
	if answered.Answer != nil && merged.Enterprise != nil {
		merged.Answer["answer"] = merged.Enterprise["executive_answer"]
	}
	return merged
}

// textLookupAugment adds the files whose text holds the value of the message to an answer the records route gave with nothing in it (no rows, or a request for more input); an answer that has rows
// is returned unchanged. The value is the number or code the analyst typed (req.TextLookup, ask_text_lookup.go); when the message holds none, the model is asked for the value (textLookupExtract), which
// is safe here and only here: the answer was empty, so a wrong guess costs one extra sentence and cannot replace anything.
func textLookupAugment(ctx context.Context, cfg config, db *pgxpool.Pool, req hybridQueryRequest, resp hybridQueryResponse) hybridQueryResponse {
	if !textSearchAllEnabled() || !textLookupAnswerEmpty(resp) {
		return resp
	}
	literal := req.TextLookup
	if literal == "" {
		literal = textLookupExtract(ctx, cfg, req)
	}
	if literal == "" {
		return resp
	}
	return textSearchAddMentions(ctx, db, req, resp, literal, searchableDerivedTextContracts, " The text you asked about is also written in", "")
}
