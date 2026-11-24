package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func intp(v int) *int           { return &v }
func floatp(v float64) *float64 { return &v }

func TestImageSearchNeedsTranslation(t *testing.T) {
	for phrase, want := range map[string]bool{
		"a red car at night":  false,
		"سرخ گاڑی رات کو":     true,
		"surkh gaari raat ko": false, // Roman Urdu is Latin script: searched as written
		"red car سرخ":         true,
		"2024 12 07 #1":       false,
		"खाली सड़क":           true,
		"café terrace":        false,
		"":                    false,
	} {
		if got := needsTranslation(phrase); got != want {
			t.Errorf("needsTranslation(%q) = %v, want %v", phrase, got, want)
		}
	}
}

func TestImageSearchCleanTranslation(t *testing.T) {
	if got := cleanTranslation("\n  \"A red car at night\"  \nNote: literal"); got != "A red car at night" {
		t.Errorf("got %q", got)
	}
	if got := cleanTranslation("   "); got != "" {
		t.Errorf("empty input should stay empty, got %q", got)
	}
	long := strings.Repeat("x", 900)
	if got := cleanTranslation(long); len([]rune(got)) != 512 {
		t.Errorf("translation not cut to 512 characters: %d", len([]rune(got)))
	}
}

func TestImageSearchCoverageStatement(t *testing.T) {
	got := coverageStatement(imageCoverage{ImagesInCase: 25, ImagesIndexed: 20, ImagesFailed: 2, ImagesIdenticalReupload: 3})
	for _, want := range []string{"Searched 20 of 25 images", "3 have not been processed yet", "2 could not be processed", "3 more are re-uploads"} {
		if !strings.Contains(got, want) {
			t.Errorf("statement %q lacks %q", got, want)
		}
	}
	plain := coverageStatement(imageCoverage{ImagesInCase: 4, ImagesIndexed: 4})
	if plain != "Searched 4 of 4 images in this case." {
		t.Errorf("complete coverage statement: %q", plain)
	}
}

func TestImageSearchBitStringAndVectorLiteral(t *testing.T) {
	bits, err := bitString(strings.Repeat("ff", 16) + strings.Repeat("00", 16))
	if err != nil || len(bits) != 256 || bits[:128] != strings.Repeat("1", 128) || bits[128:] != strings.Repeat("0", 128) {
		t.Errorf("bitString: %v %q", err, bits)
	}
	if _, err := bitString("zz"); err == nil {
		t.Error("a short or non-hex hash must be refused")
	}
	if got := vectorLiteral([]float64{0.5, -1, 0.25}); got != "[0.5,-1,0.25]" {
		t.Errorf("vectorLiteral: %s", got)
	}
}

func TestMergeCopyHitsKeepsTheStrongestTierAndCountsTheEdge(t *testing.T) {
	identical := []imageSearchHit{{EvidenceID: "a"}}
	byHash := []imageSearchHit{
		{EvidenceID: "a", HashDistance: intp(0)},  // also the identical file: stays identical
		{EvidenceID: "b", HashDistance: intp(12)}, // inside the distance
		{EvidenceID: "c", HashDistance: intp(94)}, // just outside 91
		{EvidenceID: "d", HashDistance: intp(80)}, // inside
	}
	byVector := []imageSearchHit{
		{EvidenceID: "b", CosineDistance: floatp(0.01)}, // already same_image
		{EvidenceID: "e", CosineDistance: floatp(0.15)}, // same photograph
		{EvidenceID: "f", CosineDistance: floatp(0.22)}, // just outside 0.205
		{EvidenceID: "c", CosineDistance: floatp(0.20)}, // the hash missed it but the vector catches it
	}
	matches, beyond := mergeCopyHits(identical, byHash, byVector, 91, 0.205)
	order := []string{}
	tiers := map[string]string{}
	for _, m := range matches {
		order = append(order, m.EvidenceID)
		tiers[m.EvidenceID] = m.Tier
	}
	if strings.Join(order, ",") != "a,b,d,e,c" {
		t.Errorf("order %v (identical, then same image nearest first, then same photograph nearest first)", order)
	}
	want := map[string]string{"a": "identical_file", "b": "same_image", "d": "same_image", "e": "same_photograph", "c": "same_photograph"}
	for id, tier := range want {
		if tiers[id] != tier {
			t.Errorf("%s is %s, want %s", id, tiers[id], tier)
		}
	}
	if beyond != 1 { // f only: c was rescued by the vector tier
		t.Errorf("just-outside count %d, want 1", beyond)
	}
	for i, m := range matches {
		if m.Rank != i+1 || m.TierLabel == "" {
			t.Errorf("rank or label missing on %+v", m)
		}
	}
	if matches[1].CosineDistance == nil || *matches[1].CosineDistance != 0.01 {
		t.Error("b should carry both measurements")
	}
}

func TestImageSearchHandlerValidation(t *testing.T) {
	handler := imageCaseSearchHandlerWith(imageSearchDeps{settings: imageSearchSettings{}})
	cases := []struct {
		name, method, target string
		want                 int
	}{
		{"method", http.MethodPut, "/images/search?kind=by_words&collection_id=c", http.StatusMethodNotAllowed},
		{"bad top_k", http.MethodGet, "/images/search?kind=by_words&q=x&collection_id=c&top_k=0", http.StatusBadRequest},
		{"top_k too large", http.MethodGet, "/images/search?kind=by_words&q=x&collection_id=c&top_k=101", http.StatusBadRequest},
		{"missing collection", http.MethodGet, "/images/search?kind=by_words&q=x", http.StatusBadRequest},
		{"no database", http.MethodGet, "/images/search?kind=by_words&q=x&collection_id=c", http.StatusServiceUnavailable},
		{"post of another kind", http.MethodPost, "/images/search?kind=by_words&collection_id=c", http.StatusBadRequest},
	}
	for _, c := range cases {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(c.method, c.target, nil))
		if recorder.Code != c.want {
			t.Errorf("%s: status %d, want %d (%s)", c.name, recorder.Code, c.want, recorder.Body.String())
		}
	}
}

func TestImageSearchHandlerRefusesAForgedCollection(t *testing.T) {
	handler := imageCaseSearchHandlerWith(imageSearchDeps{})
	request := httptest.NewRequest(http.MethodGet, "/images/search?kind=by_words&q=x&collection_id=someone-elses", nil)
	request = request.WithContext(context.WithValue(request.Context(), forensicPrincipalContextKey{}, forensicPrincipal{
		Authenticated: true, TenantID: "t1", ActorID: "analyst", SubjectID: "s", ActorRole: "user", CollectionID: "mine",
	}))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Errorf("a collection other than the authorised one must be refused, got %d", recorder.Code)
	}
}

func TestImageSearchActorForTheLog(t *testing.T) {
	if got := imageSearchActor(forensicScope{ActorID: "analyst-7"}); got != "analyst-7" {
		t.Error(got)
	}
	if got := imageSearchActor(forensicScope{SubjectID: "s"}); got != "s" {
		t.Error(got)
	}
	if got := imageSearchActor(forensicScope{}); got != "unauthenticated" {
		t.Error(got)
	}
}

func TestAppendSimilarKeepsCopiesFirstAndNeverListsAnImageTwice(t *testing.T) {
	copies := []imageSearchHit{{EvidenceID: "a", Tier: "same_image", Rank: 1}, {EvidenceID: "b", Tier: "same_photograph", Rank: 2}}
	d := func(v float64) *float64 { return &v }
	similar := []imageSearchHit{
		{EvidenceID: "a", CosineDistance: d(0.0)}, // already a copy
		{EvidenceID: "c", CosineDistance: d(0.2)},
		{EvidenceID: "d", CosineDistance: d(0.4)},
		{EvidenceID: "e", CosineDistance: d(0.6)},
		{EvidenceID: "c", CosineDistance: d(0.2)}, // repeated by a second chunk
	}
	got := appendSimilar(copies, similar, 2, 0.5)
	order := []string{}
	for _, h := range got {
		order = append(order, h.EvidenceID+":"+h.Tier)
	}
	if strings.Join(order, ",") != "a:same_image,b:same_photograph,c:similar,d:similar" {
		t.Errorf("order %v", order)
	}
	if got[2].Rank != 3 || got[3].Rank != 4 || got[2].Similarity == nil || *got[2].Similarity != 0.8 || got[2].TierLabel == "" {
		t.Errorf("ranks or similarity wrong: %+v %+v", got[2], got[3])
	}
}

func TestAppendSimilarDropsWhatIsNotSimilarEnough(t *testing.T) {
	d := func(v float64) *float64 { return &v }
	similar := []imageSearchHit{{EvidenceID: "x", CosineDistance: d(0.20)}, {EvidenceID: "y", CosineDistance: d(0.46)}, {EvidenceID: "z", CosineDistance: d(0.30)}}
	got := appendSimilar(nil, similar, 10, 0.70)
	if len(got) != 2 || got[0].EvidenceID != "x" || got[1].EvidenceID != "z" {
		t.Errorf("only images at least 0.70 similar may be listed, got %+v", got)
	}
	if none := appendSimilar(nil, []imageSearchHit{{EvidenceID: "q", CosineDistance: d(0.45)}}, 10, 0.70); len(none) != 0 {
		t.Error("an unrelated picture must not get a similar list")
	}
}
