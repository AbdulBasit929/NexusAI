package main

import (
	"reflect"
	"strings"
	"testing"
)

func TestImageTileBoxMirrorsThePythonLayout(t *testing.T) {
	cases := map[string][]float64{
		"v:abc:tile:a00": {0, 0, 0.5, 0.5},
		"v:abc:tile:a22": {0.5, 0.5, 1, 1},
		"v:abc:tile:a01": {0.25, 0, 0.75, 0.5},
		"v:abc:tile:b12": {0.6667, 0.3333, 1, 0.6667},
		"v:abc:tile:c33": {0.75, 0.75, 1, 1},
	}
	for ref, want := range cases {
		if got := imageTileBox(ref); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: %v, want %v", ref, got, want)
		}
	}
	for _, bad := range []string{"v:abc:full", "v:abc:tile:z00", "v:abc:tile:a30", "v:abc:tile:a0", "v:abc:tile:b33", "v:abc:tile:"} {
		if got := imageTileBox(bad); got != nil {
			t.Errorf("%q gave %v, want nil", bad, got)
		}
	}
}

func hitWithDistance(id string, distance float64, box []float64) imageSearchHit {
	d := distance
	return imageSearchHit{EvidenceID: id, OriginalFilename: id + ".jpg", CosineDistance: &d, Box: box, Tier: "ranked"}
}

func TestWordHitsScoreAPictureAsItsBestWindow(t *testing.T) {
	whole := []imageSearchHit{hitWithDistance("a", 0.90, nil), hitWithDistance("b", 0.88, nil)}
	windows := []imageSearchHit{hitWithDistance("a", 0.80, []float64{0, 0, 0.5, 0.5}), hitWithDistance("b", 0.95, []float64{0.5, 0.5, 1, 1}), hitWithDistance("c", 0.85, []float64{0.25, 0, 0.75, 0.5})}
	got := mergeWordHits(whole, windows, 10, 0)
	order := []string{got[0].EvidenceID, got[1].EvidenceID, got[2].EvidenceID}
	if !reflect.DeepEqual(order, []string{"a", "c", "b"}) {
		t.Fatalf("order %v", order)
	}
	if len(got[0].Box) != 4 || len(got[1].Box) != 4 {
		t.Errorf("a window that won must keep its box: %v %v", got[0].Box, got[1].Box)
	}
	if got[2].Box != nil {
		t.Errorf("b was best as a whole picture and must carry no box, got %v", got[2].Box)
	}
	if limited := mergeWordHits(whole, windows, 2, 0); len(limited) != 2 {
		t.Errorf("topK not applied: %d", len(limited))
	}
}

func TestSimilarPartsDropPicturesAlreadyListedAbove(t *testing.T) {
	parts := []imageSearchHit{hitWithDistance("a", 0.1, nil), hitWithDistance("b", 0.2, nil), hitWithDistance("c", 0.3, nil)}
	kept := dropAlreadyListed(parts, []imageSearchHit{{EvidenceID: "b"}})
	if len(kept) != 2 || kept[0].EvidenceID != "a" || kept[1].EvidenceID != "c" {
		t.Errorf("kept %v", kept)
	}
	if len(parts) != 3 {
		t.Error("the input must not be changed")
	}
}

func TestPartCountsAppearInTheSentence(t *testing.T) {
	text := imageAskSentence(imageAskPlan{Kind: imageAskKindCopies}, "crop.png", imageAnswerCounts{Region: 1, Part: 3, RegionRan: true}, imageCoverage{ImagesInCase: 9, ImagesIndexed: 9, Statement: "S."}, 0)
	if want := "3 more have a part that looks like it (not verified)"; !strings.Contains(text, want) {
		t.Errorf("sentence %q lacks %q", text, want)
	}
}

func TestWindowPenaltyKeepsAWeakWindowFromBeatingTheWholePicture(t *testing.T) {
	whole := []imageSearchHit{hitWithDistance("a", 0.90, nil), hitWithDistance("b", 0.92, nil)}
	// a: window 0.115 vs whole 0.10 (wins at no penalty, loses at 0.02); b: window 0.13 vs whole 0.08 (wins at both)
	windows := []imageSearchHit{hitWithDistance("a", 0.885, []float64{0, 0, 0.5, 0.5}), hitWithDistance("b", 0.87, []float64{0.5, 0.5, 1, 1})}
	free := mergeWordHits(whole, windows, 10, 0)
	if free[1].EvidenceID != "a" || len(free[1].Box) != 4 {
		t.Fatalf("with no penalty a's window wins: %+v", free[1])
	}
	penalised := mergeWordHits(whole, windows, 10, 0.02)
	byID := map[string]imageSearchHit{}
	for _, h := range penalised {
		byID[h.EvidenceID] = h
	}
	if byID["a"].Box != nil || (1-*byID["a"].CosineDistance) < 0.0999 || (1-*byID["a"].CosineDistance) > 0.1001 {
		t.Errorf("with the penalty a keeps its whole-picture score and no box: %+v", byID["a"])
	}
	if len(byID["b"].Box) != 4 || (1-*byID["b"].CosineDistance) < 0.1099 || (1-*byID["b"].CosineDistance) > 0.1101 {
		t.Errorf("b's window still wins, at its penalised score: %+v", byID["b"])
	}
	if byID["a"].WholeSimilarity == nil || byID["a"].WindowSimilarity == nil || *byID["a"].WindowSimilarity < 0.1149 || *byID["a"].WindowSimilarity > 0.1151 {
		t.Errorf("both similarities are kept for calibration: %+v", byID["a"])
	}
}
