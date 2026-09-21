package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImageComparisonSeparatesExactNearAndSemanticSimilarity(t *testing.T) {
	faceA, faceB := 1, 2
	left := imageComparisonSignal{
		EvidenceID: "a", SHA256: strings.Repeat("a", 64), PerceptualHash: "0000000000000000",
		Dimensions: map[string]any{"width_pixels": 640, "height_pixels": 480},
		Plates:     map[string]struct{}{"MN1367": {}}, OCRTokens: map[string]struct{}{"vehicle": {}}, FaceCount: &faceA,
	}
	right := imageComparisonSignal{
		EvidenceID: "b", SHA256: strings.Repeat("b", 64), PerceptualHash: "0000000000000003",
		Dimensions: map[string]any{"width_pixels": 320, "height_pixels": 240},
		Plates:     map[string]struct{}{"MN1367": {}}, OCRTokens: map[string]struct{}{"plate": {}}, FaceCount: &faceB,
	}

	result := compareImageSignals(left, right)
	if result["exact_duplicate"] != false {
		t.Fatalf("exact duplicate = %#v", result["exact_duplicate"])
	}
	perceptual := result["perceptual"].(map[string]any)
	if got := *perceptual["hamming_distance"].(*int); got != 2 || perceptual["near_duplicate_candidate"] != true {
		t.Fatalf("perceptual result = %#v", perceptual)
	}
	if result["visual_similarity"].(map[string]any)["available"] != false {
		t.Fatal("perceptual comparison was mislabeled as semantic visual similarity")
	}
	shared := result["anpr"].(map[string]any)["shared"].([]string)
	if len(shared) != 1 || shared[0] != "MN1367" {
		t.Fatalf("ANPR overlap = %#v", shared)
	}
}

func TestImageComparisonSQLIsTenantCollectionAndEvidenceBound(t *testing.T) {
	for name, query := range map[string]string{"evidence": imageComparisonEvidenceSQL, "artifacts": imageComparisonArtifactsSQL} {
		for _, fragment := range []string{"tenant_id = $1", "collection_id = $2", "evidence_id IN ($3::uuid, $4::uuid)"} {
			if !strings.Contains(query, fragment) {
				t.Fatalf("%s query missing %q", name, fragment)
			}
		}
	}
}

func TestImageComparisonRejectsInvalidOrUnconfiguredRequests(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/evidence/compare?evidence_id_a=bad&evidence_id_b=also-bad", nil)
	rec := httptest.NewRecorder()
	imageComparisonHandler(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid UUID status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/evidence/compare?evidence_id_a=343b1e62-9f24-488c-a9a0-8d79ff91ee1c&evidence_id_b=22c74bf9-0fc1-479d-816b-311106ea707b", nil)
	rec = httptest.NewRecorder()
	imageComparisonHandler(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured database status = %d", rec.Code)
	}
}
