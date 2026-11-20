package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

func textSearchTestRow(evidence, file, artifactType, text string) map[string]any {
	return map[string]any{
		"artifact_id": "a-" + evidence + "-" + text[:min(4, len(text))], "artifact_type": artifactType, "evidence_id": evidence, "version_id": "v-" + evidence, "source_file": file,
		"passage_text": text, "citation_locator": map[string]any{"page": 1}, "run_id": "r1", "observation_id": "o-" + evidence,
	}
}

const (
	testDocType   = "forensics.document-native-text-passage/v1"
	testOCRType   = "forensics.image-ocr-observation/v1"
	testAudioType = "forensics.audio-timestamp-segment/v1"
	testRomanType = "forensics.audio-roman-urdu-segment/v1"
)

func TestTextSearchStateGroupsEveryFileByHowItMatched(t *testing.T) {
	q := forensictext.Query{LiteralText: "042-35761234", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}
	state := newTextSearchState(q, 3)
	for _, row := range []map[string]any{
		textSearchTestRow("e1", "D01.pdf", testDocType, "Station landline 042-35761234 for walk-ins"),
		textSearchTestRow("e1", "D01.pdf", testDocType, "The same 042-35761234 again on page two"),
		textSearchTestRow("e2", "D06.txt", testDocType, "Call 04235761234 after nine"),
		textSearchTestRow("e3", "photo.jpg", testOCRType, "tel 042 35761234 open daily"),
		textSearchTestRow("e4", "D09.pdf", testDocType, "number 042-35761235 is a typo"),
		textSearchTestRow("e5", "D11.pdf", testDocType, "nothing relevant here"),
		textSearchTestRow("e6", "empty.pdf", testDocType, "   "),
	} {
		state.add(row)
	}
	if state.inspected != 7 || state.matched != 4 {
		t.Fatalf("inspected %d matched %d, want 7 and 4", state.inspected, state.matched) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	files := state.ordered()
	names := []string{}
	for _, f := range files {
		names = append(names, f.File+":"+string(f.Best))
	}
	want := "D01.pdf:exact D06.txt:normalised photo.jpg:normalised" // D09.pdf holds a number one digit off: in a typed document that is another number
	if got := strings.Join(names, " "); got != want {
		t.Fatalf("order\n got %s\nwant %s", got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if files[0].Passages != 2 {
		t.Fatalf("D01.pdf passages = %d, want 2", files[0].Passages) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchStateFindsAPlateWrittenInUrduDigitsAndKeepsRecordingsApart(t *testing.T) {
	q := forensictext.Query{LiteralText: "SYNT9150", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}
	state := newTextSearchState(q, 3)
	state.add(textSearchTestRow("e1", "witness-urdu.txt", testDocType, "اس کی گاڑی کا نمبر SYNT-۹۱۵۰ تھا"))
	state.add(textSearchTestRow("e2", "call.wav", testAudioType, "plate is SYNT9150 I think"))
	state.add(textSearchTestRow("e3", "other.jpg", testOCRType, "SYNT9151 is another vehicle"))
	files := state.ordered()
	if len(files) != 3 {
		t.Fatalf("files = %d, want 3 (one exact, one written differently, one close reading)", len(files)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if files[0].File != "call.wav" || files[0].Best != forensictext.TierExact || files[0].Kind != "recordings" {
		t.Fatalf("first = %+v", files[0]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if files[1].File != "witness-urdu.txt" || files[1].Best != forensictext.TierNormalised {
		t.Fatalf("second = %+v", files[1]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if files[2].File != "other.jpg" || files[2].Best != forensictext.TierApproximate {
		t.Fatalf("third = %+v", files[2]) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchStateOffersCloseReadingsOfNumbersOnlyWhereAModelDidTheReading(t *testing.T) {
	q := forensictext.Query{LiteralText: "0300-0001005", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}
	state := newTextSearchState(q, 3)
	state.add(textSearchTestRow("e1", "calls.txt", testDocType, "numbers 0300 0001004 and 0300 0001006 called")) // other subscribers in a typed document
	state.add(textSearchTestRow("e2", "photo.jpg", testOCRType, "tel 0300-0001006"))                             // a possible misreading of an image
	state.add(textSearchTestRow("e3", "memo.docx", testDocType, "number 0092 300 0001005 recovered"))            // the same number, written with the international prefix
	files := state.ordered()
	if len(files) != 2 || files[0].File != "memo.docx" || files[0].Best != forensictext.TierNormalised || files[1].File != "photo.jpg" || files[1].Best != forensictext.TierApproximate {
		names := []string{}
		for _, f := range files {
			names = append(names, f.File+":"+string(f.Best))
		}
		t.Fatalf("files = %v", names) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	// a word is another matter: a spelling variant of a name in a document is a close reading worth showing
	word := newTextSearchState(forensictext.Query{LiteralText: "Hassan Farooqi", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}, 3)
	word.add(textSearchTestRow("e4", "statement.txt", testDocType, "my name is Hasan Farooqi"))
	if files := word.ordered(); len(files) != 1 || files[0].Best != forensictext.TierApproximate {
		t.Fatalf("a spelling variant of a name in a document was dropped: %+v", files) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchStateHonoursTheTextRepresentation(t *testing.T) {
	q := forensictext.Query{LiteralText: "payment", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true, Representation: "raw"}
	state := newTextSearchState(q, 3)
	state.add(textSearchTestRow("e1", "a.wav", testRomanType, "payment kar do"))
	state.add(textSearchTestRow("e2", "b.wav", testAudioType, "payment kar do"))
	if files := state.ordered(); len(files) != 1 || files[0].File != "b.wav" {
		t.Fatalf("a raw-text query matched the roman derivative: %+v", files) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchStateKeepsOnlyTheBestExamplesOfAFileWithManyMatches(t *testing.T) {
	q := forensictext.Query{LiteralText: "invoice", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}
	state := newTextSearchState(q, 3)
	state.add(textSearchTestRow("e1", "big.pdf", testDocType, "the invoise number is a misreading"))
	for i := 0; i < 500; i++ {
		row := textSearchTestRow("e1", "big.pdf", testDocType, fmt.Sprintf("the invoice number %d", i))
		row["artifact_id"] = fmt.Sprintf("art-%d", i)
		state.add(row)
	}
	files := state.ordered()
	if len(files) != 1 || files[0].Passages != 501 {
		t.Fatalf("passages = %d, want 501 in one file", files[0].Passages) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if len(files[0].Examples) != 3 {
		t.Fatalf("examples = %d, want 3", len(files[0].Examples)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	for _, ex := range files[0].Examples {
		if ex.Tier != forensictext.TierExact {
			t.Fatalf("an example is %q; the best examples must be kept", ex.Tier) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextSearchAllSentenceNamesEveryFileGroupedByKindAndTier(t *testing.T) {
	files := []*textSearchFile{}
	for i := 1; i <= 9; i++ {
		files = append(files, &textSearchFile{File: fmt.Sprintf("D%02d.pdf", i), Kind: "documents", Best: forensictext.TierExact, Passages: 1})
	}
	files = append(files,
		&textSearchFile{File: "scan.jpg", Kind: "images", Best: forensictext.TierExact, Passages: 2},
		&textSearchFile{File: "witness.txt", Kind: "documents", Best: forensictext.TierNormalised, Passages: 1},
		&textSearchFile{File: "close.pdf", Kind: "documents", Best: forensictext.TierApproximate, Passages: 1},
	)
	sentence := textSearchAllSentence("042-35761234", textSearchOutcome{Files: files, Inspected: 300, Matched: 13, Complete: true})
	for _, f := range files {
		if !strings.Contains(sentence, f.File) {
			t.Fatalf("the answer leaves out %s: %s", f.File, sentence) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	for _, part := range []string{`"042-35761234" appears in 12 files (13 passages)`, "As typed in documents (9)", "images (1): scan.jpg", "Written differently", "close reading"} {
		if !strings.Contains(sentence, part) {
			t.Fatalf("the answer lacks %q: %s", part, sentence) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	if strings.Contains(sentence, "more") || strings.Contains(sentence, "stopped early") {
		t.Fatalf("a complete short list must not be shortened: %s", sentence) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchAllSentenceShortensOnlyBeyondTheNamedBudgetAndPointsToTheTable(t *testing.T) {
	files := []*textSearchFile{}
	for i := 1; i <= textSearchAllNamedInAnswer+7; i++ {
		files = append(files, &textSearchFile{File: fmt.Sprintf("F%03d.pdf", i), Kind: "documents", Best: forensictext.TierExact, Passages: 1})
	}
	sentence := textSearchAllSentence("x", textSearchOutcome{Files: files, Inspected: 10, Matched: len(files), Complete: true})
	if !strings.Contains(sentence, "and 7 more") || !strings.Contains(sentence, "The table lists every file.") {
		t.Fatalf("the long list is not marked: %s", sentence) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchAllSentenceSaysWhenNothingWasFoundAndWhenTheSearchStoppedEarly(t *testing.T) {
	none := textSearchAllSentence("zzz", textSearchOutcome{Inspected: 412, Complete: true})
	if !strings.Contains(none, "No document, image or recording text holds") || !strings.Contains(none, "412 passages were searched") {
		t.Fatalf("absence not stated: %s", none) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	early := textSearchAllSentence("zzz", textSearchOutcome{Inspected: 9, Complete: false, IncompleteReason: "it ran out of time"})
	if !strings.Contains(early, "stopped early (it ran out of time)") || !strings.Contains(early, "other files may hold it") {
		t.Fatalf("early stop not stated: %s", early) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchAllSentenceReportsTheKindTheQuestionNamedFirstAndAddsOtherKinds(t *testing.T) {
	doc := &textSearchFile{File: "D04.txt", Kind: "documents", Best: forensictext.TierExact, Passages: 1}
	photo := &textSearchFile{File: "scan.jpg", Kind: "images", Best: forensictext.TierNormalised, Passages: 1}
	// asked about recordings, found in a document: still found, and the answer says the named kind has none
	o := textSearchOutcome{Scope: "recording text", AskedKinds: []string{"recordings"}, Files: []*textSearchFile{doc}, Inspected: 40, Matched: 1, Complete: true}
	sentence := textSearchAllSentence("Hassan Farooqi", o)
	for _, part := range []string{"No recording text holds", "but it appears in 1 file of another kind", "D04.txt"} {
		if !strings.Contains(sentence, part) {
			t.Fatalf("lacks %q: %s", part, sentence) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	// asked about documents, also found in an image: the document comes first, the image is added
	o = textSearchOutcome{Scope: "document text", AskedKinds: []string{"documents"}, Files: []*textSearchFile{doc, photo}, Inspected: 40, Matched: 2, Complete: true}
	sentence = textSearchAllSentence("Hassan Farooqi", o)
	if first, second := strings.Index(sentence, "D04.txt"), strings.Index(sentence, "scan.jpg"); first < 0 || second < first || !strings.Contains(sentence, "Also in other kinds of source:") {
		t.Fatalf("order or addition wrong: %s", sentence) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	// nothing anywhere: the whole case was searched, and the answer says so
	none := textSearchAllSentence("zzz", textSearchOutcome{Scope: "recording text", AskedKinds: []string{"recordings"}, Inspected: 7, Complete: true})
	if !strings.Contains(none, "No document, image or recording text holds") {
		t.Fatalf("nothing found: %s", none) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchRecordingsAreNamedWithTheMomentOfTheFirstMatch(t *testing.T) {
	q := forensictext.Query{LiteralText: "coconut sugar", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}
	state := newTextSearchState(q, 3)
	row := textSearchTestRow("e1", "interview.wav", testAudioType, "especially Japanese coconut sugar")
	row["citation_locator"] = map[string]any{"start_seconds": 11.28, "end_seconds": 14.0}
	state.add(row)
	files := state.ordered()
	if got := textSearchFileName(files[0]); got != "interview.wav (at 0:11)" {
		t.Fatalf("name = %q", got) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchSnippetShowsTheMatchInALongPassage(t *testing.T) {
	long := strings.Repeat("filler words go here ", 40) + "the account PK00SYNT0000000000002005 was credited " + strings.Repeat("and more filler ", 40)
	snippet := textSearchSnippet(long, "PK00SYNT0000000000002005")
	if !strings.Contains(snippet, "PK00SYNT0000000000002005") || len([]rune(snippet)) > textSearchAllSnippetRunes+6 {
		t.Fatalf("snippet = %q", snippet) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	short := textSearchSnippet("short text", "text")
	if short != "short text" {
		t.Fatalf("short = %q", short) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	// a number written without its separators is still located by its beginning
	written := strings.Repeat("filler words go here ", 40) + "dial 04235761234 now " + strings.Repeat("and more filler ", 40)
	if snippet := textSearchSnippet(written, "042-35761234"); !strings.Contains(snippet, "04235761234") {
		t.Fatalf("snippet = %q", snippet) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchAllRunDeclinesWhatItDoesNotUnderstand(t *testing.T) {
	// no database, no query, a time-range query: all fall through to the path the question would have taken
	if _, ok := textSearchAllRun(context.Background(), nil, hybridQueryRequest{}, time.Now(), nil); ok {
		t.Fatal("answered without a database") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchRelaxedFindsThePartOfTheTypedTextThatIsWritten(t *testing.T) {
	q := forensictext.Query{LiteralText: "vehicle number SYNT 2019", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}
	state := newTextSearchState(q, 3)
	state.keep = true
	state.add(textSearchTestRow("e1", "D10.txt", testDocType, "registration SYNT-2019, owner national ID"))
	state.add(textSearchTestRow("e2", "D12.docx", testDocType, "گاڑی نمبر پلیٹ synt ۲۰۱۹ موقع پر موجود تھی"))
	state.add(textSearchTestRow("e3", "D01.docx", testDocType, "nothing here at all"))
	if got := state.ordered(); len(got) != 0 {
		t.Fatalf("the whole text must match nothing here, got %d files", len(got)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	parts := state.relaxed(nil)
	if len(parts) != 1 || parts[0].Phrase != "SYNT 2019" || len(parts[0].Files) != 2 {
		t.Fatalf("parts = %+v", parts) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	sentence := textSearchAllSentence(q.LiteralText, textSearchOutcome{Partial: parts, Inspected: 3, Complete: true})
	for _, want := range []string{`No document, image or recording text holds "vehicle number SYNT 2019"`, `Part of it, "SYNT 2019", is written in 2 files`, "D10.txt", "D12.docx", "only part of what you typed"} {
		if !strings.Contains(sentence, want) {
			t.Fatalf("the answer lacks %q: %s", want, sentence) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextSearchRelaxedOnlyWhereItIsAsked(t *testing.T) {
	single := newTextSearchState(forensictext.Query{LiteralText: "REF00000394", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}, 3)
	single.keep = len(strings.Fields("REF00000394")) >= 2
	single.add(textSearchTestRow("e1", "a.txt", testDocType, "nothing"))
	if parts := single.relaxed(nil); parts != nil || len(single.rows) != 0 {
		t.Fatalf("a single word has no parts and keeps no rows: %+v", parts) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	many := newTextSearchState(forensictext.Query{LiteralText: "bank account PK00SYNT0000000000002010", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}, 3)
	many.keep = true
	many.add(textSearchTestRow("e2", "D07.txt", testDocType, "accounts: PK00 SYNT 0000 0000 0000 2010 and another"))
	parts := many.relaxed(nil)
	if len(parts) != 1 || parts[0].Phrase != "PK00SYNT0000000000002010" {
		t.Fatalf("the account part was not found: %+v", parts) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	for _, levels := range textSearchSubPhrases("a bb ccc dddd") { // parts shorter than four characters are not tried
		for _, part := range levels {
			if len([]rune(part.Phrase)) < 4 {
				t.Fatalf("part %q is too short to mean anything", part.Phrase) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
		}
	}
}

func TestTextSearchWindowsMatchAPhraseSplitAcrossTranscriptSegments(t *testing.T) {
	q := forensictext.Query{LiteralText: "cover the knees", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}
	state := newTextSearchState(q, 3)
	state.keep = true
	segment := func(file, text string, start, end float64) map[string]any {
		row := textSearchTestRow("e-"+file, file, testAudioType, text)
		row["citation_locator"] = map[string]any{"start_seconds": start, "end_seconds": end}
		return row
	}
	// out of order on purpose: the windows are built from the segments in time order
	for _, row := range []map[string]any{
		segment("talk.wav", "the knees and nothing else", 4.0, 6.0),
		segment("talk.wav", "men are required to wear trousers that cover", 0.0, 4.0),
		segment("other.wav", "cover", 0.0, 1.0),
		segment("other.wav", "something unrelated entirely", 1.0, 2.0),
	} {
		state.add(row)
	}
	if got := state.ordered(); len(got) != 0 {
		t.Fatalf("no single segment holds the phrase, got %d files", len(got)) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	state.addWindows()
	files := state.ordered()
	if len(files) != 1 || files[0].File != "talk.wav" || files[0].Best != forensictext.TierExact {
		t.Fatalf("the split phrase was not found: %+v", files) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if name := textSearchFileName(files[0]); name != "talk.wav (at 0:00)" {
		t.Fatalf("the moment is the start of the first segment: %q", name) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if state.inspected != 4 {
		t.Fatalf("windows must not count as passages searched: %d", state.inspected) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchRelaxedKeepsEveryPartWithTheCodeAndListsAFileOnce(t *testing.T) {
	q := forensictext.Query{LiteralText: "vehicle number SYNT 2019", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}
	state := newTextSearchState(q, 3)
	state.keep = true
	state.add(textSearchTestRow("e1", "D04.txt", testDocType, "gari number SYNT2019 thi"))        // holds the longer part "number SYNT 2019"
	state.add(textSearchTestRow("e2", "D10.txt", testDocType, "registration SYNT-2019, owner"))   // holds only "SYNT 2019"
	state.add(textSearchTestRow("e3", "D99.txt", testDocType, "year 2019 report of the vehicle")) // a bare year is not a part
	parts := state.relaxed(nil)
	if len(parts) != 2 || parts[0].Phrase != "number SYNT 2019" || parts[0].Files[0].File != "D04.txt" || parts[1].Phrase != "SYNT 2019" || len(parts[1].Files) != 1 || parts[1].Files[0].File != "D10.txt" {
		names := []string{}
		for _, part := range parts {
			names = append(names, part.Phrase)
			for _, f := range part.Files {
				names = append(names, "  "+f.File)
			}
		}
		t.Fatalf("parts = %v", names) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	// words only: the longest level that matches, and a part that matches only as a close reading is not listed
	words := newTextSearchState(forensictext.Query{LiteralText: "IMSI values", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}, 3)
	words.keep = true
	words.add(textSearchTestRow("e4", "D08.txt", testDocType, "IMSI: 001010000001010"))
	words.add(textSearchTestRow("e5", "D05.txt", testDocType, "one value was struck out"))
	parts = words.relaxed(nil)
	if len(parts) != 1 || parts[0].Phrase != "IMSI" || len(parts[0].Files) != 1 || parts[0].Files[0].File != "D08.txt" {
		t.Fatalf("parts = %+v", parts) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}

func TestTextSearchRelaxedAddsFilesThatWriteTheNumberWithoutTheOtherWords(t *testing.T) {
	q := forensictext.Query{LiteralText: "helpline 042 35761234", MatchSemantic: forensictext.PhraseContains, CaseInsensitive: true}
	state := newTextSearchState(q, 3)
	state.keep = true
	state.add(textSearchTestRow("e1", "D09.txt", testDocType, "Complaints helpline 042-35761234."))
	state.add(textSearchTestRow("e2", "D01.docx", testDocType, "Police station landline 042-35761234, reference"))
	whole := state.ordered()
	if len(whole) != 1 || whole[0].File != "D09.txt" {
		t.Fatalf("whole = %+v", whole) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	parts := state.relaxed(map[string]bool{whole[0].EvidenceID + "|" + whole[0].File: true})
	if len(parts) != 1 || parts[0].Phrase != "042 35761234" || len(parts[0].Files) != 1 || parts[0].Files[0].File != "D01.docx" {
		t.Fatalf("parts = %+v", parts) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	sentence := textSearchAllSentence(q.LiteralText, textSearchOutcome{Files: whole, Partial: parts, Inspected: 2, Matched: 1, Complete: true})
	for _, want := range []string{`appears in 1 file`, "D09.txt", `Part of what you typed, "042 35761234", is also written in 1 more file`, "D01.docx", "Those hold only part of what you typed"} {
		if !strings.Contains(sentence, want) {
			t.Fatalf("the answer lacks %q: %s", want, sentence) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}

func TestTextSearchMayRelaxOnlyForANumberOrAShortText(t *testing.T) {
	for literal, want := range map[string]bool{
		"vehicle number SYNT 2019":            true, // carries a number
		"helpline 042 35761234":               true, // carries a number, even when the whole matched something
		"IMSI values":                         true, // two words and nothing matched the whole
		"Hassan Farooqi":                      true,
		"kis kis dastawez ya record mein hai": false, // a whole sentence with no number is not taken apart
		"Garden Town Lahore":                  false,
	} {
		if got := textSearchMayRelax(literal, 0); got != want {
			t.Errorf("textSearchMayRelax(%q, 0) = %v, want %v", literal, got, want) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
	if textSearchMayRelax("IMSI values", 3) {
		t.Error("a short text that already matched files is not taken apart") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if !textSearchMayRelax("helpline 042 35761234", 2) {
		t.Error("a text with a number is tried for parts even when the whole matched") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
}
