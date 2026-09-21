package main

import (
	"context"
	"errors"
	"fmt"
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
	imageSimilarityContract       = "forensics.semantic-image-candidate-similarity/v1"
	imageEmbeddingContract        = "forensics.image-embedding-observation/v1"
	imageSimilarityMaxEvidenceSet = 200
	imageSimilarityMaxTopK        = 50
)

type imageSimilarityObservation struct {
	ObservationID string
	EvidenceID    string
	VersionID     string
	CitationRef   string
	Locator       map[string]any
	Embedding     []float64
	Model         string
	ModelSHA256   string
	ModelRevision string
	ReviewState   string
	FrameTime     any
	CreatedAt     any
}

type imageSimilarityCandidate struct {
	Observation imageSimilarityObservation
	Score       float64
}

func imageSimilarityHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if r.URL.Query().Get("authorization") != "explicit_case_evidence_scope" {
			writeError(w, http.StatusForbidden, errors.New("explicit_case_evidence_scope authorization is required"))
			return
		}
		queryID := strings.TrimSpace(r.URL.Query().Get("query_image_observation_id"))
		if _, err := uuid.Parse(queryID); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("query_image_observation_id must be a UUID"))
			return
		}
		candidateIDs := r.URL.Query()["candidate_evidence_id"]
		if len(candidateIDs) > imageSimilarityMaxEvidenceSet {
			writeError(w, http.StatusBadRequest, fmt.Errorf("at most %d candidate_evidence_id values are accepted", imageSimilarityMaxEvidenceSet))
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
			if err != nil || value < 1 || value > imageSimilarityMaxTopK {
				writeError(w, http.StatusBadRequest, fmt.Errorf("top_k must be between 1 and %d", imageSimilarityMaxTopK))
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
		result, err := loadImageSimilarity(r.Context(), db, scope.TenantID, scope.CollectionID, scope.CaseID, queryID, candidateIDs, topK)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, errors.New("query image embedding was not found in the authorized current-version scope"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func loadImageSimilarity(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID, caseID, queryID string, candidateEvidenceIDs []string, topK int) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin image similarity: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, fmt.Errorf("set tenant context: %w", err)
	}
	queryRows, err := rowsFromQuery(ctx, tx, faceObservationByIDSQL, tenantID, collectionID, imageEmbeddingContract, queryID, caseID)
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
	candidateRows, err := rowsFromQuery(ctx, tx, faceObservationCandidatesSQL, tenantID, collectionID, imageEmbeddingContract, candidateUUIDs, caseID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit image similarity read: %w", err)
	}

	populationTruncated := len(candidateRows) > imageSimilarityMaxEvidenceSet
	if populationTruncated {
		candidateRows = candidateRows[:imageSimilarityMaxEvidenceSet]
	}
	query, err := imageObservationFromRow(queryRows[0])
	if err != nil {
		return nil, fmt.Errorf("query image embedding: %w", err)
	}
	candidates := make([]imageSimilarityCandidate, 0, len(candidateRows))
	for _, row := range candidateRows {
		candidate, err := imageObservationFromRow(row)
		if err != nil || candidate.ObservationID == query.ObservationID {
			continue
		}
		if candidate.Model != query.Model || candidate.ModelRevision != query.ModelRevision ||
			candidate.ModelSHA256 != query.ModelSHA256 || len(candidate.Embedding) != len(query.Embedding) {
			continue
		}
		score, ok := cosineSimilarity(query.Embedding, candidate.Embedding)
		if ok {
			candidates = append(candidates, imageSimilarityCandidate{Observation: candidate, Score: score})
		}
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
			"distance_metric":                "cosine_similarity",
			"candidate_image_observation_id": candidate.Observation.ObservationID,
			"candidate_evidence_id":          candidate.Observation.EvidenceID,
			"candidate_version_id":           candidate.Observation.VersionID,
			"citation_ref":                   candidate.Observation.CitationRef,
			"citation_locator":               candidate.Observation.Locator,
			"source_frame_time":              optionalValue(candidate.Observation.FrameTime),
			"review_state":                   candidate.Observation.ReviewState,
			"created_at":                     optionalValue(candidate.Observation.CreatedAt),
			"limitation":                     "candidate semantic visual similarity; not evidence identity or fact",
		}
	}
	return map[string]any{
		"contract_version": imageSimilarityContract,
		"semantics":        "candidate semantic visual similarity; not evidence identity or fact",
		"scope": map[string]any{
			"tenant_id": tenantID, "collection_id": collectionID, "case_id": optionalString(caseID),
			"candidate_evidence_ids": candidateEvidenceIDs, "candidate_population": map[string]any{"source": "backend_authorized_current_versions", "bounded_count": len(candidateRows), "truncated": populationTruncated},
		},
		"query": map[string]any{
			"query_image_observation_id": query.ObservationID, "evidence_id": query.EvidenceID,
			"version_id": query.VersionID, "citation_ref": query.CitationRef, "citation_locator": query.Locator,
		},
		"model": map[string]any{
			"id": query.Model, "sha256": query.ModelSHA256, "revision": query.ModelRevision,
			"embedding_dimension": len(query.Embedding),
		},
		"results": results,
		"limitations": []string{
			"When candidate evidence IDs are omitted, candidates are populated by the backend from authorized current-version evidence, independent of UI pagination.",
			"Scores are review candidates and do not establish duplicate identity, object identity, event identity, or truth.",
			"Text-to-image query execution is not exposed by this API slice.",
			"Candidate population is bounded to 200 observations; truncation is disclosed in scope.candidate_population.",
		},
	}, nil
}

func imageObservationFromRow(row map[string]any) (imageSimilarityObservation, error) {
	metadata := objectValue(row["metadata"])
	observation := objectValue(metadata["observation"])
	embedding, err := finiteEmbedding(observation["embedding"])
	if err != nil {
		return imageSimilarityObservation{}, err
	}
	return imageSimilarityObservation{
		ObservationID: imageStringValue(metadata["observation_id"]),
		EvidenceID:    imageStringValue(row["evidence_id"]), VersionID: imageStringValue(row["version_id"]),
		CitationRef: imageStringValue(row["citation_ref"]), Locator: objectValue(row["citation_locator"]),
		Embedding: embedding, Model: imageStringValue(observation["embedding_model"]),
		ModelSHA256:   strings.ToLower(imageStringValue(observation["model_sha256"])),
		ModelRevision: imageStringValue(observation["embedding_revision"]),
		ReviewState:   imageStringValue(observation["review_state"]),
		FrameTime:     observation["frame_timestamp_seconds"], CreatedAt: row["created_at"],
	}, nil
}
