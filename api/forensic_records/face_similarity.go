package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	faceSimilarityContract       = "forensics.face-candidate-similarity/v1"
	faceObservationContract      = "forensics.face-observation/v1"
	faceSimilarityMaxEvidenceSet = 200
	faceSimilarityMaxTopK        = 50
)

const faceObservationByIDSQL = `
SELECT artifacts.artifact_id::text, artifacts.evidence_id::text,
       artifacts.version_id::text, artifacts.metadata, artifacts.citation_locator,
       artifacts.created_at,
       'nexusai://evidence/' || artifacts.evidence_id::text || '/artifacts/' || artifacts.artifact_id::text AS citation_ref
FROM forensic.derived_artifacts artifacts
JOIN forensic.evidence_items evidence
  ON evidence.tenant_id = artifacts.tenant_id
 AND evidence.collection_id = artifacts.collection_id
 AND evidence.evidence_id = artifacts.evidence_id
 AND evidence.current_version_id = artifacts.version_id
WHERE artifacts.tenant_id = $1
  AND artifacts.collection_id = $2
  AND artifacts.artifact_type = $3
  AND artifacts.processing_status = 'completed'
  AND artifacts.metadata->>'observation_id' = $4
  AND (NULLIF($5, '') IS NULL OR evidence.case_id = $5)
LIMIT 1`

const faceObservationCandidatesSQL = `
SELECT artifacts.artifact_id::text, artifacts.evidence_id::text,
       artifacts.version_id::text, artifacts.metadata, artifacts.citation_locator,
       artifacts.created_at,
       'nexusai://evidence/' || artifacts.evidence_id::text || '/artifacts/' || artifacts.artifact_id::text AS citation_ref
FROM forensic.derived_artifacts artifacts
JOIN forensic.evidence_items evidence
  ON evidence.tenant_id = artifacts.tenant_id
 AND evidence.collection_id = artifacts.collection_id
 AND evidence.evidence_id = artifacts.evidence_id
 AND evidence.current_version_id = artifacts.version_id
WHERE artifacts.tenant_id = $1
  AND artifacts.collection_id = $2
  AND artifacts.artifact_type = $3
  AND artifacts.processing_status = 'completed'
  AND (cardinality($4::uuid[]) = 0 OR artifacts.evidence_id = ANY($4::uuid[]))
  AND (NULLIF($5, '') IS NULL OR evidence.case_id = $5)
ORDER BY artifacts.created_at, artifacts.artifact_id
LIMIT 201`

type faceSimilarityObservation struct {
	ObservationID string
	EvidenceID    string
	VersionID     string
	CitationRef   string
	Locator       map[string]any
	Embedding     []float64
	Model         string
	ModelSHA256   string
	BackendDigest string
	ModelVersion  string
	Quality       map[string]any
	ReviewState   string
	FrameTime     any
	CreatedAt     any
}

type faceSimilarityCandidate struct {
	Observation faceSimilarityObservation
	Score       float64
}

func faceSimilarityHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if r.URL.Query().Get("authorization") != "explicit_case_evidence_scope" {
			writeError(w, http.StatusForbidden, errors.New("explicit_case_evidence_scope authorization is required"))
			return
		}
		queryID := strings.TrimSpace(r.URL.Query().Get("query_face_observation_id"))
		if _, err := uuid.Parse(queryID); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("query_face_observation_id must be a UUID"))
			return
		}
		candidateIDs := r.URL.Query()["candidate_evidence_id"]
		if len(candidateIDs) > faceSimilarityMaxEvidenceSet {
			writeError(w, http.StatusBadRequest, fmt.Errorf("at most %d candidate_evidence_id values are accepted", faceSimilarityMaxEvidenceSet))
			return
		}
		for _, value := range candidateIDs {
			if _, err := uuid.Parse(value); err != nil {
				writeError(w, http.StatusBadRequest, errors.New("candidate_evidence_id values must be UUIDs"))
				return
			}
		}
		topK := 10
		if raw := strings.TrimSpace(r.URL.Query().Get("top_k")); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > faceSimilarityMaxTopK {
				writeError(w, http.StatusBadRequest, fmt.Errorf("top_k must be between 1 and %d", faceSimilarityMaxTopK))
				return
			}
			topK = value
		}
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		scope, err := bindForensicScope(r, r.URL.Query().Get("tenant_id"), r.URL.Query().Get("collection_id"), r.URL.Query().Get("case_id"), "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		if scope.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}
		result, err := loadFaceSimilarity(r.Context(), db, scope.TenantID, scope.CollectionID, scope.CaseID, queryID, candidateIDs, topK)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, errors.New("query face observation was not found in the authorized current-version scope"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func loadFaceSimilarity(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID, caseID, queryID string, candidateEvidenceIDs []string, topK int) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin face similarity: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, fmt.Errorf("set tenant context: %w", err)
	}
	queryRows, err := rowsFromQuery(ctx, tx, faceObservationByIDSQL, tenantID, collectionID, faceObservationContract, queryID, caseID)
	if err != nil {
		return nil, err
	}
	if len(queryRows) != 1 {
		return nil, pgx.ErrNoRows
	}
	candidateUUIDs := make([]uuid.UUID, len(candidateEvidenceIDs))
	for index, value := range candidateEvidenceIDs {
		candidateUUIDs[index] = uuid.MustParse(value)
	}
	candidateRows, err := rowsFromQuery(ctx, tx, faceObservationCandidatesSQL, tenantID, collectionID, faceObservationContract, candidateUUIDs, caseID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit face similarity read: %w", err)
	}

	populationTruncated := len(candidateRows) > faceSimilarityMaxEvidenceSet
	if populationTruncated {
		candidateRows = candidateRows[:faceSimilarityMaxEvidenceSet]
	}
	query, err := faceObservationFromRow(queryRows[0])
	if err != nil {
		return nil, fmt.Errorf("query face observation: %w", err)
	}
	candidates := make([]faceSimilarityCandidate, 0, len(candidateRows))
	for _, row := range candidateRows {
		candidate, err := faceObservationFromRow(row)
		if err != nil || candidate.ObservationID == query.ObservationID {
			continue
		}
		if candidate.Model != query.Model || candidate.ModelVersion != query.ModelVersion || candidate.ModelSHA256 != query.ModelSHA256 || candidate.BackendDigest != query.BackendDigest || len(candidate.Embedding) != len(query.Embedding) {
			continue
		}
		score, ok := cosineSimilarity(query.Embedding, candidate.Embedding)
		if !ok {
			continue
		}
		candidates = append(candidates, faceSimilarityCandidate{Observation: candidate, Score: score})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Observation.ObservationID < candidates[j].Observation.ObservationID
		}
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}
	results := make([]map[string]any, len(candidates))
	for index, candidate := range candidates {
		results[index] = map[string]any{
			"rank": index + 1, "score": candidate.Score, "similarity_score": candidate.Score,
			"distance_metric":               "cosine_similarity",
			"candidate_face_observation_id": candidate.Observation.ObservationID,
			"candidate_evidence_id":         candidate.Observation.EvidenceID,
			"source_evidence_id":            candidate.Observation.EvidenceID,
			"candidate_version_id":          candidate.Observation.VersionID,
			"citation_ref":                  candidate.Observation.CitationRef,
			"citation_locator":              candidate.Observation.Locator,
			"query_quality":                 query.Quality,
			"candidate_quality":             candidate.Observation.Quality,
			"source_frame_time":             optionalValue(candidate.Observation.FrameTime),
			"review_state":                  candidate.Observation.ReviewState,
			"created_at":                    optionalValue(candidate.Observation.CreatedAt),
			"limitation":                    "candidate face similarity; not identity",
		}
	}
	return map[string]any{
		"contract_version": faceSimilarityContract,
		"semantics":        "candidate visual similarity; not identity",
		"scope": map[string]any{
			"tenant_id": tenantID, "collection_id": collectionID, "case_id": optionalString(caseID),
			"candidate_evidence_ids": candidateEvidenceIDs, "candidate_population": map[string]any{"source": "backend_authorized_current_versions", "bounded_count": len(candidateRows), "truncated": populationTruncated},
		},
		"query": map[string]any{
			"query_face_observation_id": query.ObservationID, "evidence_id": query.EvidenceID,
			"version_id": query.VersionID, "citation_ref": query.CitationRef, "citation_locator": query.Locator,
		},
		"model": map[string]any{
			"id": query.Model, "sha256": optionalString(query.ModelSHA256),
			"version": optionalString(query.ModelVersion), "backend_digest": optionalString(query.BackendDigest), "embedding_dimension": len(query.Embedding),
		},
		"results": results,
		"limitations": []string{
			"When candidate evidence IDs are omitted, candidates are populated by the backend from authorized current-version evidence, independent of UI pagination.",
			"A similarity score is a review candidate and never an identity conclusion.",
			"Candidate population is bounded to 200 observations; truncation is disclosed in scope.candidate_population.",
		},
	}, nil
}

func faceObservationFromRow(row map[string]any) (faceSimilarityObservation, error) {
	metadata := objectValue(row["metadata"])
	observation := objectValue(metadata["observation"])
	embedding, err := finiteEmbedding(observation["embedding"])
	if err != nil {
		return faceSimilarityObservation{}, err
	}
	return faceSimilarityObservation{
		ObservationID: imageStringValue(metadata["observation_id"]),
		EvidenceID:    imageStringValue(row["evidence_id"]), VersionID: imageStringValue(row["version_id"]),
		CitationRef: imageStringValue(row["citation_ref"]), Locator: objectValue(row["citation_locator"]),
		Embedding: embedding, Model: imageStringValue(observation["embedding_model"]),
		ModelSHA256:   strings.ToLower(imageStringValue(observation["model_sha256"])),
		BackendDigest: strings.ToLower(imageStringValue(observation["backend_digest"])),
		ModelVersion:  imageStringValue(observation["embedding_version"]),
		Quality:       objectValue(observation["quality"]),
		ReviewState:   imageStringValue(observation["review_state"]),
		FrameTime:     observation["frame_timestamp_seconds"],
		CreatedAt:     row["created_at"],
	}, nil
}

func finiteEmbedding(value any) ([]float64, error) {
	var rawEmbedding []any
	switch typed := value.(type) {
	case []any:
		rawEmbedding = typed
	case []float64:
		rawEmbedding = make([]any, len(typed))
		for index, number := range typed {
			rawEmbedding[index] = number
		}
	default:
		return nil, errors.New("embedding is unavailable")
	}
	embedding := make([]float64, 0, len(rawEmbedding))
	for _, item := range rawEmbedding {
		number, ok := item.(float64)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
			return nil, errors.New("embedding contains a non-finite value")
		}
		embedding = append(embedding, number)
	}
	if len(embedding) == 0 {
		return nil, errors.New("embedding is unavailable")
	}
	return embedding, nil
}

func optionalValue(value any) any {
	if value == nil || imageStringValue(value) == "" {
		return nil
	}
	return normalizeDBValue(value)
}

func cosineSimilarity(left, right []float64) (float64, bool) {
	if len(left) == 0 || len(left) != len(right) {
		return 0, false
	}
	var dot, leftNorm, rightNorm float64
	for index := range left {
		dot += left[index] * right[index]
		leftNorm += left[index] * left[index]
		rightNorm += right[index] * right[index]
	}
	if leftNorm == 0 || rightNorm == 0 {
		return 0, false
	}
	return dot / (math.Sqrt(leftNorm) * math.Sqrt(rightNorm)), true
}
