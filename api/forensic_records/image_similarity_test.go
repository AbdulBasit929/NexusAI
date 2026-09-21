package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestImageObservationParserPreservesPinnedSemanticProvenance(t *testing.T) {
	row := imageSimilarityRow(
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
		[]float64{0.1, 0.2, 0.3},
	)
	observation, err := imageObservationFromRow(row)
	if err != nil {
		t.Fatal(err)
	}
	if observation.Model != "google/siglip-base-patch16-224" || observation.ModelRevision != "7fd15f0" || len(observation.Embedding) != 3 {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestImageSimilarityUsesTenantCaseCurrentVersionAndBackendCandidateScope(t *testing.T) {
	for name, query := range map[string]string{"query": faceObservationByIDSQL, "candidates": faceObservationCandidatesSQL} {
		for _, fragment := range []string{
			"artifacts.tenant_id = $1", "artifacts.collection_id = $2",
			"evidence.current_version_id = artifacts.version_id", "evidence.case_id",
		} {
			if !strings.Contains(query, fragment) {
				t.Fatalf("%s query missing %q", name, fragment)
			}
		}
	}
	if !strings.Contains(faceObservationCandidatesSQL, "cardinality($4::uuid[]) = 0 OR artifacts.evidence_id = ANY($4::uuid[])") || !strings.Contains(faceObservationCandidatesSQL, "LIMIT 201") {
		t.Fatal("semantic image candidate query does not support bounded backend-authorized population")
	}
}

func TestImageSimilarityRequiresExplicitAuthorizationBeforeBackendPopulation(t *testing.T) {
	query := "11111111-1111-1111-1111-111111111111"
	req := httptest.NewRequest(http.MethodGet, "/images/similar?query_image_observation_id="+query, nil)
	rec := httptest.NewRecorder()
	imageSimilarityHandler(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing authorization status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/images/similar?authorization=explicit_case_evidence_scope&query_image_observation_id="+query, nil)
	rec = httptest.NewRecorder()
	imageSimilarityHandler(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("authorized backend-populated request status = %d", rec.Code)
	}
}

func TestImageComparisonKeepsSemanticAndPerceptualScoresSeparate(t *testing.T) {
	left := imageComparisonSignal{
		SHA256: "a", PerceptualHash: "0000000000000000",
		Embedding: []float64{1, 0}, EmbeddingModel: "siglip", EmbeddingSHA: "sha", EmbeddingRev: "rev",
		Dimensions: map[string]any{}, OCRTokens: map[string]struct{}{}, Plates: map[string]struct{}{},
	}
	right := imageComparisonSignal{
		SHA256: "b", PerceptualHash: "ffffffffffffffff",
		Embedding: []float64{0.9, 0.1}, EmbeddingModel: "siglip", EmbeddingSHA: "sha", EmbeddingRev: "rev",
		Dimensions: map[string]any{}, OCRTokens: map[string]struct{}{}, Plates: map[string]struct{}{},
	}
	result := compareImageSignals(left, right)
	perceptual := result["perceptual"].(map[string]any)
	visual := result["visual_similarity"].(map[string]any)
	distance := perceptual["hamming_distance"].(*int)
	if *distance != 64 || visual["available"] != true {
		t.Fatalf("perceptual=%#v visual=%#v", perceptual, visual)
	}
	if visual["score"].(float64) < 0.99 {
		t.Fatalf("semantic score = %v", visual["score"])
	}
}

func TestSharedAskImageSimilarityIntentUsesAcceptedBoundedSubmode(t *testing.T) {
	if chooseTemplate("Which images look most similar to the selected image?", "") != "image_metadata" || !isImageSimilarityQuery("Which images look most similar to the selected image?") {
		t.Fatal("image similarity intent did not route through the accepted image operation")
	}
	if isImageSimilarityQuery("Open this exact image") {
		t.Fatal("exact image open must not invoke similarity")
	}
	for _, fragment := range []string{"evidence.current_version_id=artifacts.version_id", "artifacts.evidence_id=$3::uuid", "artifacts.version_id=$4::uuid", "artifacts.artifact_type=$5"} {
		if !strings.Contains(selectedSimilarityObservationSQL, fragment) {
			t.Fatalf("selected similarity SQL missing %q", fragment)
		}
	}
}

func TestSharedAskSimilarityPresentationRanksCandidatesAndPreservesCitations(t *testing.T) {
	req := hybridQueryRequest{
		TenantID: "default", CollectionID: "case-1", Query: "Find images similar to this.",
		EvidenceID: "11111111-1111-1111-8111-111111111111", Limit: 10,
	}
	resp := hybridQueryResponse{
		Template: "image_metadata",
		Records: map[string]any{
			"status": "results_present", "row_count": 1,
			"model": map[string]any{"id": "siglip", "embedding_dimension": 768},
			"similarity_results": []map[string]any{{
				"rank": 1, "similarity_score": 0.91,
				"candidate_evidence_id":          "33333333-3333-3333-8333-333333333333",
				"candidate_version_id":           "44444444-4444-4444-8444-444444444444",
				"candidate_image_observation_id": "55555555-5555-5555-8555-555555555555",
				"citation_ref":                   "nexusai://evidence/33333333-3333-3333-8333-333333333333/artifacts/55555555-5555-5555-8555-555555555555",
				"citation_locator":               map[string]any{"scope": "full_image"},
			}},
		},
		Answer: map[string]any{"records_row_count": 1},
	}
	payload := buildEnterprisePayload(req, resp)
	if !strings.Contains(stringValueAny(payload["executive_answer"]), "Ranked 1 authorized current-version image candidate") {
		t.Fatalf("executive answer = %q", payload["executive_answer"])
	}
	operation := payload["operation"].(map[string]any)
	if operation["submode"] != "image_similarity" || operation["operation_id"] != "image.semantic_similarity" {
		t.Fatalf("operation = %#v", operation)
	}
	rows := payload["data_grid"].(map[string]any)["rows"].([]map[string]any)
	if len(rows) != 1 || rows[0]["similarity_score"] != 0.91 || rows[0]["embedding_dimension"] != nil {
		t.Fatalf("display rows = %#v", rows)
	}
	provenance := payload["provenance"].([]map[string]any)
	if len(provenance) != 1 || provenance[0]["evidence_id"] != "33333333-3333-3333-8333-333333333333" {
		t.Fatalf("provenance = %#v", provenance)
	}
}

func TestSharedAskFaceSimilarityAcceptsNaturalVisualWording(t *testing.T) {
	query := "Find candidate faces visually similar to this face."
	if chooseTemplate(query, "") != "face_candidate_observations" || !isFaceSimilarityQuery(query) {
		t.Fatal("face similarity wording did not route through the governed face-candidate submode")
	}
}

func TestDataGeneratedRecognizedTextPromptUsesOCRTemplate(t *testing.T) {
	query := `Find the exact recognized text "Investigation Workspace" in retained derived observations, focused on printed-english.png, and cite the matching artifact and source.`
	if chooseTemplate(query, "") != "image_ocr_search" {
		t.Fatalf("template = %q", chooseTemplate(query, ""))
	}
}

func imageSimilarityRow(observationID, evidenceID string, embedding []float64) map[string]any {
	values := make([]any, len(embedding))
	for index, value := range embedding {
		values[index] = value
	}
	metadata, _ := json.Marshal(map[string]any{
		"observation_id": observationID,
		"observation": map[string]any{
			"embedding":          values,
			"embedding_model":    "google/siglip-base-patch16-224",
			"embedding_revision": "7fd15f0",
			"model_sha256":       "model-sha",
			"review_state":       "model_candidate",
		},
	})
	return map[string]any{
		"evidence_id": evidenceID, "version_id": uuid.NewString(),
		"citation_ref":     "nexusai://evidence/" + evidenceID,
		"citation_locator": []byte(`{"scope":"full_image"}`), "metadata": metadata,
	}
}
