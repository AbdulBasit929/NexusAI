package main

import (
	"reflect"
	"strings"
	"testing"
)

func dupImage(id string, size int64) duplicateImage {
	return duplicateImage{ID: id, Name: id + ".jpg", SHA: "sha-" + id, Size: size}
}

func TestDuplicateGroupsStackByLinkAndPickTheLargestAsTheLead(t *testing.T) {
	images := []duplicateImage{dupImage("a", 100), dupImage("b", 300), dupImage("c", 50), dupImage("d", 999), dupImage("e", 10), dupImage("f", 20), dupImage("g", 30)}
	bits, distance := 10, 0.05
	edges := []duplicateEdge{
		{A: "a", B: "b", Tier: "same_image", Hash: &bits},
		{A: "b", B: "c", Tier: "same_photograph", Distance: &distance},
		{A: "e", B: "f", Tier: "same_image", Hash: &bits},
		{A: "f", B: "g", Tier: "same_photograph", Distance: &distance},
	}
	hits, summary := groupDuplicates(images, edges)
	if summary.Groups != 2 || summary.Pictures != 6 || summary.CouldSetAside != 4 || summary.Considered != 7 {
		t.Fatalf("summary %+v", summary)
	}
	var order []string
	for _, h := range hits {
		order = append(order, h.OriginalFilename+":"+h.Tier)
	}
	want := []string{
		"b.jpg:duplicate_lead", "a.jpg:same_image", "c.jpg:same_photograph", // group 1: b is the largest file; a and c link to it directly
		"g.jpg:duplicate_lead", "e.jpg:linked_copy", "f.jpg:same_photograph", // group 2: e is linked only through f
	}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order\n got %v\nwant %v", order, want)
	}
	if hits[0].Group != 1 || hits[0].GroupSize != 3 || hits[3].Group != 2 {
		t.Errorf("group numbers %+v %+v", hits[0], hits[3])
	}
	if hits[1].HashDistance == nil || *hits[1].HashDistance != 10 {
		t.Errorf("a member keeps the distance of its link to the lead: %+v", hits[1])
	}
	for _, h := range hits {
		if h.OriginalFilename == "d.jpg" {
			t.Error("a picture with no copy is not in any group")
		}
	}
}

func TestDuplicateGroupsKeepTheStrongestLinkBetweenTwoPictures(t *testing.T) {
	images := []duplicateImage{dupImage("a", 2), dupImage("b", 1)}
	bits, distance := 30, 0.09
	hits, _ := groupDuplicates(images, []duplicateEdge{
		{A: "a", B: "b", Tier: "same_photograph", Distance: &distance},
		{A: "b", B: "a", Tier: "same_image", Hash: &bits},
		{A: "a", B: "b", Tier: "identical_file"},
	})
	if len(hits) != 2 || hits[1].Tier != "identical_file" {
		t.Errorf("hits %+v", hits)
	}
}

func TestDuplicateGroupsIgnoreLinksToPicturesOutsideTheCaseAndCapTheGroupsShown(t *testing.T) {
	var images []duplicateImage
	var edges []duplicateEdge
	for i := 0; i < imageDuplicateMaxGroups+5; i++ {
		a, b := dupImage(string(rune('A'+i/26))+string(rune('a'+i%26))+"x", 2), dupImage(string(rune('A'+i/26))+string(rune('a'+i%26))+"y", 1)
		images = append(images, a, b)
		edges = append(edges, duplicateEdge{A: a.ID, B: b.ID, Tier: "identical_file"})
	}
	edges = append(edges, duplicateEdge{A: "ghost", B: images[0].ID, Tier: "identical_file"})
	hits, summary := groupDuplicates(images, edges)
	if summary.Groups != imageDuplicateMaxGroups+5 || summary.GroupsShown != imageDuplicateMaxGroups || len(hits) != 2*imageDuplicateMaxGroups {
		t.Errorf("summary %+v with %d hits", summary, len(hits))
	}
}

func TestDuplicateStatementsSayWhatWasFound(t *testing.T) {
	none := duplicateStatement(imageDuplicateSummary{Considered: 97})
	if !strings.Contains(none, "No duplicate or near-duplicate pictures among the 97") {
		t.Errorf("none: %q", none)
	}
	some := duplicateStatement(imageDuplicateSummary{Groups: 3, Pictures: 8, CouldSetAside: 5, GroupsShown: 3, Considered: 97})
	if !strings.Contains(some, "3 groups of duplicate or near-duplicate pictures, 8 pictures in all; 5 could be set aside") {
		t.Errorf("some: %q", some)
	}
	limited := duplicateStatement(imageDuplicateSummary{Groups: 1, Pictures: 2, CouldSetAside: 1, GroupsShown: 1, Limited: true})
	if !strings.Contains(limited, "Only identical files were compared") {
		t.Errorf("limited: %q", limited)
	}
}

func TestWordHitsAreCutIntoBestAndPossibleByTheirShareOfTheBestSimilarity(t *testing.T) {
	// similarities 0.120, 0.115, 0.100, 0.080, 0.050 -> best >= 0.1056, possible >= 0.09
	hits := []imageSearchHit{hitWithDistance("a", 0.88, nil), hitWithDistance("b", 0.885, nil), hitWithDistance("c", 0.90, nil), hitWithDistance("d", 0.92, nil), hitWithDistance("e", 0.95, nil)}
	got := tierWordHits(hits, 0.88, 0.75, 20)
	var order []string
	for _, h := range got {
		order = append(order, h.EvidenceID+":"+h.Tier)
	}
	if want := []string{"a:ranked", "b:ranked", "c:possible"}; !reflect.DeepEqual(order, want) {
		t.Errorf("got %v, want %v", order, want)
	}
	if capped := tierWordHits(hits, 0.88, 0.75, 1); len(capped) != 2 || capped[0].Tier != "ranked" || capped[1].Tier != "possible" {
		t.Errorf("the best list is capped on its own: %+v", capped)
	}
	if empty := tierWordHits(nil, 0.88, 0.75, 20); len(empty) != 0 {
		t.Errorf("empty in, empty out: %v", empty)
	}
}

func TestSentencesForDuplicatesAndTieredWords(t *testing.T) {
	cov := imageCoverage{ImagesInCase: 9, ImagesIndexed: 9, Statement: "Searched 9 of 9 images in this case."}
	words := imageAskSentence(imageAskPlan{Kind: imageAskKindPhotos}, "a laptop", imageAnswerCounts{Ranked: 3, Possible: 2}, cov, 0)
	if !strings.Contains(words, "Closest pictures to \"a laptop\": 3 best matches, and 2 more also possible.") {
		t.Errorf("words: %q", words)
	}
	one := imageAskSentence(imageAskPlan{Kind: imageAskKindPhotos}, "a clock", imageAnswerCounts{Ranked: 1}, cov, 0)
	if !strings.Contains(one, "1 best match.") {
		t.Errorf("one: %q", one)
	}
	dup := imageAskSentence(imageAskPlan{Kind: imageAskKindDuplicates}, "this case", imageAnswerCounts{DupStatement: "2 groups of duplicate or near-duplicate pictures."}, cov, 0)
	if !strings.HasPrefix(dup, "2 groups of duplicate") {
		t.Errorf("duplicates: %q", dup)
	}
	attached := imageAskSentence(imageAskPlan{Kind: imageAskKindCopies}, "your picture", imageAnswerCounts{Region: 2, RegionRan: true}, cov, 0)
	if !strings.HasPrefix(attached, "Your picture appears inside 2 other pictures.") {
		t.Errorf("attached: %q", attached)
	}
}

func TestTheGateAcceptsDuplicateQuestionsAndTheIntentSchemaKnowsTheKind(t *testing.T) {
	for _, query := range []string{"show me the duplicate pictures", "find duplicates in this case", "are there near-duplicate images"} {
		if !imageAskMediaWords.MatchString(query) {
			t.Errorf("the gate must let %q reach the model", query)
		}
	}
	plan, why := imageIntentValidate("find duplicates", imageIntentWire{Kind: imageAskKindDuplicates})
	if why != "" || plan.Kind != imageAskKindDuplicates {
		t.Errorf("plan %+v reason %q", plan, why)
	}
	enum := imageIntentSchema()["properties"].(map[string]any)["a_kind"].(map[string]any)["enum"].([]string)
	found := false
	for _, kind := range enum {
		found = found || kind == imageAskKindDuplicates
	}
	if !found || !strings.Contains(imageIntentSystemPrompt(), "duplicate_groups") {
		t.Error("the schema and the prompt must both name duplicate_groups")
	}
}

func TestWindowOnlyHitsAreSeparateBoxedAndCapped(t *testing.T) {
	box := []float64{0, 0, 0.5, 0.5}
	windows := []imageSearchHit{
		hitWithDistance("a", 0.86, box), hitWithDistance("b", 0.87, box), hitWithDistance("c", 0.88, nil), hitWithDistance("d", 0.89, box), hitWithDistance("e", 0.95, box),
	}
	for i := 0; i < 8; i++ {
		windows = append(windows, hitWithDistance(string(rune('m'+i)), 0.87, box))
	}
	// the best window similarity is 0.14; the ratio 0.92 keeps those at 0.1288 or more: a (0.14), b (0.13), c (no box: skipped), d (0.11 is below), the eight m-t at 0.13
	listed := map[string]bool{"a": true}
	got := windowOnlyHits(windows, listed, 0.92)
	if len(got) != imageWordsInPartMax {
		t.Fatalf("capped at %d, got %d: %+v", imageWordsInPartMax, len(got), got)
	}
	for _, h := range got {
		if h.EvidenceID == "a" || h.EvidenceID == "c" || h.EvidenceID == "d" || h.EvidenceID == "e" {
			t.Errorf("%s must not be listed (already ranked, no box, or too weak)", h.EvidenceID)
		}
		if h.Tier != imageTierInPart || len(h.Box) != 4 {
			t.Errorf("a window-only hit carries its tier and its box: %+v", h)
		}
	}
	if none := windowOnlyHits(nil, nil, 0.92); none != nil {
		t.Errorf("no windows, no hits: %v", none)
	}
}

func TestInPartCountsAppearInTheWordsSentence(t *testing.T) {
	cov := imageCoverage{ImagesInCase: 9, ImagesIndexed: 9, Statement: "S."}
	text := imageAskSentence(imageAskPlan{Kind: imageAskKindPhotos}, "a phone", imageAnswerCounts{Ranked: 2, Possible: 1, InPart: 4}, cov, 0)
	if !strings.Contains(text, "2 best matches, and 1 more also possible. 4 more may show it in a part of the picture (not verified).") {
		t.Errorf("sentence %q", text)
	}
}
