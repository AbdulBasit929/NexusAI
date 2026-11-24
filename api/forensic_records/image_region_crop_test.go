package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestImageBoxIsFourFractionsInsideThePicture(t *testing.T) {
	box, err := parseImageBox(" 0.1, 0.2,0.5 ,0.9 ")
	if err != nil || box != [4]float64{0.1, 0.2, 0.5, 0.9} {
		t.Fatalf("box %v err %v", box, err)
	}
	for _, bad := range []string{"", "0.1,0.2,0.5", "0.1,0.2,0.5,0.9,1", "a,b,c,d", "-0.1,0,0.5,0.5", "0,0,1.2,0.5", "0.5,0.1,0.4,0.9", "0.1,0.5,0.9,0.5"} {
		if _, err := parseImageBox(bad); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}

func TestTheServiceClientSendsThePictureAndTheBoxAndReturnsThePart(t *testing.T) {
	var seen map[string]any
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/crop" || r.Header.Get("X-Image-Search-Key") != "k" {
			http.Error(w, "bad", http.StatusUnauthorized)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&seen)
		_ = json.NewEncoder(w).Encode(map[string]any{"png_b64": base64.StdEncoding.EncodeToString([]byte("PNGDATA")), "width": 10, "height": 10})
	}))
	defer service.Close()
	client := httpImageEmbedClient{settings: imageSearchSettings{ServiceURL: service.URL, ServiceKey: "k", Timeout: 5 * time.Second}}
	part, err := client.Crop(context.Background(), []byte("picture"), [4]float64{0.1, 0.2, 0.5, 0.9})
	if err != nil || string(part) != "PNGDATA" {
		t.Fatalf("part %q err %v", part, err)
	}
	if seen["b64"] != base64.StdEncoding.EncodeToString([]byte("picture")) || len(seen["box"].([]any)) != 4 {
		t.Errorf("request %v", seen)
	}
}

func TestThePictureTheBoxWasDrawnOnIsLeftOutOfItsOwnResults(t *testing.T) {
	hits := []imageSearchHit{{EvidenceID: "src"}, {EvidenceID: "other"}, {EvidenceID: "src"}}
	got := withoutPicture(hits, "src")
	if len(got) != 1 || got[0].EvidenceID != "other" {
		t.Errorf("got %+v", got)
	}
	if same := withoutPicture(hits, ""); len(same) != 3 {
		t.Errorf("no exclusion keeps everything: %+v", same)
	}
}

func TestTheAnswerNamesTheSelectedPartAndCapitalisesIt(t *testing.T) {
	cov := imageCoverage{ImagesInCase: 9, ImagesIndexed: 9, Statement: "S."}
	text := imageAskSentence(imageAskPlan{Kind: imageAskKindCopies}, "the selected part", imageAnswerCounts{Region: 2, RegionRan: true}, cov, 0)
	if !strings.HasPrefix(text, "The selected part appears inside 2 other pictures.") {
		t.Errorf("sentence %q", text)
	}
}
