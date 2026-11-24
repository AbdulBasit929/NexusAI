package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeRegionService behaves like the service's cache protocol: a picture is answered from memory when its hash was seen, otherwise it is named in "need" until its bytes arrive.
type fakeRegionService struct {
	mu       sync.Mutex
	seen     map[string]bool
	inliers  map[string]int
	calls    int
	withData int
}

func (f *fakeRegionService) RegionMatch(_ context.Context, _ []byte, items []regionMatchItem) (regionMatchResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	var out regionMatchResponse
	for _, item := range items {
		if item.B64 != "" {
			f.withData++
			f.seen[item.Key] = true
		}
		if !f.seen[item.Key] {
			out.Need = append(out.Need, item.Key)
			continue
		}
		out.Results = append(out.Results, regionMatchResult{Key: item.Key, Inliers: f.inliers[item.Key], Matches: f.inliers[item.Key] + 5, Box: []float64{0.1, 0.1, 0.5, 0.5}})
	}
	return out, nil
}

func regionCandidates() []regionCandidate {
	var out []regionCandidate
	for _, k := range []string{"k1", "k2", "k3", "k4", "k5"} {
		out = append(out, regionCandidate{EvidenceID: "e-" + k, Name: k + ".jpg", SHA256: k, Size: 10})
	}
	return out
}

func TestRegionPassesAskForBytesOnlyOnceAndKeepTheStrongMatches(t *testing.T) {
	service := &fakeRegionService{seen: map[string]bool{}, inliers: map[string]int{"k1": 40, "k2": 10, "k3": 3, "k5": 15}}
	reads := 0
	read := func(c regionCandidate) ([]byte, error) {
		reads++
		if c.SHA256 == "k4" {
			return nil, errors.New("file missing")
		}
		return []byte(c.SHA256), nil
	}
	cands := regionCandidates()
	results, unreadable, err := runRegionPasses(context.Background(), service, []byte("probe"), cands, read)
	if err != nil {
		t.Fatal(err)
	}
	if reads != 5 || unreadable != 1 {
		t.Errorf("first search: reads %d unreadable %d", reads, unreadable)
	}
	hits := imageRegionHits(cands, results, imageRegionSettings{VerifiedMin: 12, ProbableMin: 8})
	var order []string
	for _, h := range hits {
		order = append(order, h.OriginalFilename+":"+h.Tier)
	}
	want := []string{"k1.jpg:region_verified", "k5.jpg:region_verified", "k2.jpg:region_probable"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("hits %v, want %v", order, want)
	}
	if hits[0].Rank != 1 || *hits[0].Inliers != 40 || len(hits[0].Box) != 4 || hits[0].TierLabel == "" {
		t.Errorf("first hit %+v", hits[0])
	}
	// a second search over the same case sends no picture again
	reads, service.withData = 0, 0
	if _, _, err := runRegionPasses(context.Background(), service, []byte("probe"), cands, read); err != nil {
		t.Fatal(err)
	}
	if service.withData != 0 || reads != 1 { // only the unreadable picture is asked for again
		t.Errorf("second search sent %d pictures and read %d", service.withData, reads)
	}
}

func TestRegionPassesSplitLargeBatchesBySize(t *testing.T) {
	service := &fakeRegionService{seen: map[string]bool{}, inliers: map[string]int{}}
	big := make([]byte, imageRegionBatchBytes/2+1)
	read := func(regionCandidate) ([]byte, error) { return big, nil }
	_, _, err := runRegionPasses(context.Background(), service, []byte("probe"), regionCandidates(), read)
	if err != nil {
		t.Fatal(err)
	}
	if service.withData != 5 || service.calls < 1+3 { // one hash-only call, then at least three byte batches for five 3 MB pictures
		t.Errorf("sent %d pictures in %d calls", service.withData, service.calls)
	}
}

func TestRegionPassesFailWhenTheServiceIsDown(t *testing.T) {
	_, _, err := runRegionPasses(context.Background(), downRegionService{}, []byte("probe"), regionCandidates(), func(regionCandidate) ([]byte, error) { return []byte("x"), nil })
	if err == nil {
		t.Error("a service that is down must be an error, not an empty result")
	}
}

type downRegionService struct{}

func (downRegionService) RegionMatch(context.Context, []byte, []regionMatchItem) (regionMatchResponse, error) {
	return regionMatchResponse{}, errImageSearchServiceDown
}

func TestRegionStatementSaysWhatWasChecked(t *testing.T) {
	text := imageRegionStatement(imageRegionReport{ImagesInCase: 97, ImagesChecked: 95, Unreadable: 2, NotChecked: 0})
	if !strings.Contains(text, "Checked 95 of 97 images") || !strings.Contains(text, "2 could not be read") {
		t.Errorf("statement %q", text)
	}
	if !strings.Contains(imageRegionStatement(imageRegionReport{ImagesInCase: 500, ImagesChecked: 400, NotChecked: 100}), "100 were not checked") {
		t.Error("a case over the limit must say how many were not checked")
	}
}

func TestRegionAnswerListsContainingPicturesWithTheirBox(t *testing.T) {
	inliers := 31
	hits := []imageSearchHit{{EvidenceID: "e1", OriginalFilename: "a.jpg", Tier: imageRegionVerified, TierLabel: imageTierLabels[imageRegionVerified], Inliers: &inliers, Box: []float64{0.2, 0.3, 0.6, 0.7}}}
	result := map[string]any{"results": hits, "coverage": imageCoverage{ImagesInCase: 3, ImagesIndexed: 3, Statement: "Searched 3 of 3 images in this case."}, "region": imageRegionReport{}}
	response := imageAskResponse(imageAskTestRequest("x"), time.Now(), imageAskPlan{Kind: imageAskKindCopies, ImageName: "crop.png"}, "crop.png", result)
	text := response.Answer["answer"].(string)
	if !strings.Contains(text, "crop.png appears inside 1 other picture") {
		t.Errorf("answer %q", text)
	}
	rows := response.Evidence["results"].([]map[string]any)
	meta := rows[0]["metadata"].(map[string]any)
	if meta["match_kind"] != imageRegionVerified || meta["region_inliers"] != 31 || len(meta["region_box"].([]float64)) != 4 {
		t.Errorf("metadata %v", meta)
	}
}
