package main

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCosineSimilarityRanksSameDirectionAboveDifferentDirection(t *testing.T) {
	same, ok := cosineSimilarity([]float64{1, 0, 1}, []float64{0.9, 0.1, 0.9})
	if !ok {
		t.Fatal("same-direction cosine unavailable")
	}
	different, ok := cosineSimilarity([]float64{1, 0, 1}, []float64{-1, 0, 0})
	if !ok || same <= different || math.Abs(same-0.9969) > 0.01 {
		t.Fatalf("same=%f different=%f", same, different)
	}
}

func TestFaceObservationParserRequiresFiniteEmbeddingAndProvenance(t *testing.T) {
	row := faceSimilarityRow("11111111-1111-1111-1111-111111111111", "22222222-2222-2222-2222-222222222222", []float64{1, 2, 3})
	observation, err := faceObservationFromRow(row)
	if err != nil {
		t.Fatal(err)
	}
	if observation.ObservationID == "" || observation.Model != "face-detect-yunet-sface" || len(observation.Embedding) != 3 {
		t.Fatalf("observation = %#v", observation)
	}
}

func TestFaceSimilaritySQLIsTenantCollectionCaseCurrentVersionAndBackendBounded(t *testing.T) {
	for name, query := range map[string]string{"query": faceObservationByIDSQL, "candidates": faceObservationCandidatesSQL} {
		for _, fragment := range []string{"artifacts.tenant_id = $1", "artifacts.collection_id = $2", "evidence.current_version_id = artifacts.version_id", "evidence.case_id"} {
			if !strings.Contains(query, fragment) {
				t.Fatalf("%s query missing %q", name, fragment)
			}
		}
	}
	if !strings.Contains(faceObservationCandidatesSQL, "cardinality($4::uuid[]) = 0 OR artifacts.evidence_id = ANY($4::uuid[])") || !strings.Contains(faceObservationCandidatesSQL, "LIMIT 201") {
		t.Fatal("candidate query does not support bounded backend-authorized population")
	}
}

func TestFaceSimilarityRequiresExplicitAuthorizationBeforeBackendPopulation(t *testing.T) {
	query := "11111111-1111-1111-1111-111111111111"
	req := httptest.NewRequest(http.MethodGet, "/faces/similar?query_face_observation_id="+query, nil)
	rec := httptest.NewRecorder()
	faceSimilarityHandler(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("missing authorization status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/faces/similar?authorization=explicit_case_evidence_scope&query_face_observation_id="+query, nil)
	rec = httptest.NewRecorder()
	faceSimilarityHandler(nil).ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("authorized backend-populated request status = %d", rec.Code)
	}
}

func TestSharedAskFaceSimilarityIntentUsesAcceptedSafeSubmode(t *testing.T) {
	query := "Which authorized image has the closest face candidate?"
	if chooseTemplate(query, "") != "face_candidate_observations" || !isFaceSimilarityQuery(query) {
		t.Fatal("face similarity intent did not route through the accepted face-candidate operation")
	}
	if isFaceSimilarityQuery("Who is this person?") {
		t.Fatal("identity question must not be treated as candidate similarity")
	}
}

func faceSimilarityRow(observationID, evidenceID string, embedding []float64) map[string]any {
	values := make([]any, len(embedding))
	for index, value := range embedding {
		values[index] = value
	}
	metadata, _ := json.Marshal(map[string]any{
		"observation_id": observationID,
		"observation": map[string]any{
			"embedding": values, "embedding_model": "face-detect-yunet-sface", "embedding_version": "nexusai-post-bf-a-v1", "model_sha256": "model-sha", "quality": map[string]any{"minimum_dimension_pixels": 64}, "review_state": "model_candidate",
		},
	})
	return map[string]any{
		"evidence_id": evidenceID, "version_id": uuid.NewString(), "citation_ref": "nexusai://evidence/" + evidenceID,
		"citation_locator": []byte(`{"bbox":{"x":1,"y":2,"width":3,"height":4}}`), "metadata": metadata,
	}
}
