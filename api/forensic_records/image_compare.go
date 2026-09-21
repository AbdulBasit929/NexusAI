package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/bits"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	imageComparisonContract       = "forensics.image-comparison/v1"
	imagePerceptualHashAlgorithm  = "dhash-64-ffmpeg-area-v1"
	imageNearDuplicateMaxDistance = 10
)

const imageComparisonEvidenceSQL = `
SELECT evidence_id::text, current_version_id::text, original_filename, sha256,
       media_metadata, processing_status,
       'nexusai://evidence/' || evidence_id::text AS citation_ref
FROM forensic.evidence_items
WHERE tenant_id = $1
  AND collection_id = $2
  AND evidence_id IN ($3::uuid, $4::uuid)
  AND (NULLIF($5, '') IS NULL OR case_id = $5)
  AND modality = 'image'
ORDER BY evidence_id`

const imageComparisonArtifactsSQL = `
SELECT artifact_id::text, evidence_id::text, version_id::text, artifact_type,
       metadata, citation_locator,
       'nexusai://evidence/' || evidence_id::text || '/artifacts/' || artifact_id::text AS citation_ref
FROM forensic.derived_artifacts
WHERE tenant_id = $1
  AND collection_id = $2
  AND evidence_id IN ($3::uuid, $4::uuid)
  AND processing_status = 'completed'
ORDER BY created_at, artifact_id`

type imageComparisonSignal struct {
	EvidenceID     string
	VersionID      string
	SourceFile     string
	SHA256         string
	CitationRef    string
	Dimensions     map[string]any
	PerceptualHash string
	OCRTokens      map[string]struct{}
	Plates         map[string]struct{}
	FaceCount      *int
	Embedding      []float64
	EmbeddingModel string
	EmbeddingSHA   string
	EmbeddingRev   string
	ArtifactRefs   []string
}

func imageComparisonHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		leftID := strings.TrimSpace(r.URL.Query().Get("evidence_id_a"))
		rightID := strings.TrimSpace(r.URL.Query().Get("evidence_id_b"))
		if _, err := uuid.Parse(leftID); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("evidence_id_a must be a UUID"))
			return
		}
		if _, err := uuid.Parse(rightID); err != nil {
			writeError(w, http.StatusBadRequest, errors.New("evidence_id_b must be a UUID"))
			return
		}
		if leftID == rightID {
			writeError(w, http.StatusBadRequest, errors.New("two distinct evidence IDs are required"))
			return
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
		result, err := loadImageComparison(r.Context(), db, scope.TenantID, scope.CollectionID, scope.CaseID, leftID, rightID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				writeError(w, http.StatusNotFound, errors.New("both scoped image evidence items are required"))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func loadImageComparison(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID, caseID, leftID, rightID string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin image comparison: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, fmt.Errorf("set tenant context: %w", err)
	}
	evidenceRows, err := rowsFromQuery(ctx, tx, imageComparisonEvidenceSQL, tenantID, collectionID, leftID, rightID, caseID)
	if err != nil {
		return nil, err
	}
	if len(evidenceRows) != 2 {
		return nil, pgx.ErrNoRows
	}
	artifactRows, err := rowsFromQuery(ctx, tx, imageComparisonArtifactsSQL, tenantID, collectionID, leftID, rightID)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit image comparison: %w", err)
	}
	signals := map[string]imageComparisonSignal{}
	for _, row := range evidenceRows {
		signal := imageSignalFromEvidenceRow(row)
		signals[signal.EvidenceID] = signal
	}
	for _, row := range artifactRows {
		evidenceID := imageStringValue(row["evidence_id"])
		signal, ok := signals[evidenceID]
		if !ok {
			continue
		}
		applyImageArtifact(&signal, row)
		signals[evidenceID] = signal
	}
	left, leftOK := signals[leftID]
	right, rightOK := signals[rightID]
	if !leftOK || !rightOK {
		return nil, pgx.ErrNoRows
	}
	return compareImageSignals(left, right), nil
}

func imageSignalFromEvidenceRow(row map[string]any) imageComparisonSignal {
	metadata := objectValue(row["media_metadata"])
	dimensions := map[string]any{
		"width_pixels":  metadata["width_pixels"],
		"height_pixels": metadata["height_pixels"],
	}
	return imageComparisonSignal{
		EvidenceID: imageStringValue(row["evidence_id"]), VersionID: imageStringValue(row["current_version_id"]),
		SourceFile: imageStringValue(row["original_filename"]), SHA256: strings.ToLower(imageStringValue(row["sha256"])),
		CitationRef: imageStringValue(row["citation_ref"]), Dimensions: dimensions,
		OCRTokens: map[string]struct{}{}, Plates: map[string]struct{}{}, ArtifactRefs: []string{},
	}
}

func applyImageArtifact(signal *imageComparisonSignal, row map[string]any) {
	metadata := objectValue(row["metadata"])
	observation := objectValue(metadata["observation"])
	switch imageStringValue(metadata["observation_type"]) {
	case "image_fingerprint_observation":
		signal.PerceptualHash = strings.ToLower(imageStringValue(observation["perceptual_hash"]))
	case "anpr_ocr_observation":
		if plate := normalizeImagePlate(imageStringValue(observation["normalized_plate_text"])); plate != "" {
			signal.Plates[plate] = struct{}{}
		}
	case "image_ocr_observation", "general_image_ocr_observation":
		text := imageStringValue(observation["raw_text"])
		if text == "" {
			text = imageStringValue(observation["text"])
		}
		for _, token := range imageTextTokens(text) {
			signal.OCRTokens[token] = struct{}{}
		}
	case "semantic_image_embedding_observation":
		values, err := finiteEmbedding(observation["embedding"])
		if err == nil {
			signal.Embedding = values
			signal.EmbeddingModel = imageStringValue(observation["embedding_model"])
			signal.EmbeddingSHA = strings.ToLower(imageStringValue(observation["model_sha256"]))
			signal.EmbeddingRev = imageStringValue(observation["embedding_revision"])
		}
	case "face_detection_observation":
		count := 1
		if signal.FaceCount == nil {
			signal.FaceCount = &count
		} else {
			count = *signal.FaceCount + 1
			signal.FaceCount = &count
		}
	}
	if ref := imageStringValue(row["citation_ref"]); ref != "" {
		signal.ArtifactRefs = append(signal.ArtifactRefs, ref)
	}
}

func compareImageSignals(left, right imageComparisonSignal) map[string]any {
	shaMatch := left.SHA256 != "" && left.SHA256 == right.SHA256
	var distance *int
	var similarity *float64
	nearDuplicate := false
	if validDHash(left.PerceptualHash) && validDHash(right.PerceptualHash) {
		value := bits.OnesCount64(parseDHash(left.PerceptualHash) ^ parseDHash(right.PerceptualHash))
		distance = &value
		score := 1 - float64(value)/64
		similarity = &score
		nearDuplicate = !shaMatch && value <= imageNearDuplicateMaxDistance
	}
	visualSimilarity := map[string]any{
		"available": false, "score": nil,
		"limitation": "compatible current-version semantic image embeddings are unavailable",
	}
	if left.EmbeddingModel != "" && left.EmbeddingModel == right.EmbeddingModel &&
		left.EmbeddingSHA != "" && left.EmbeddingSHA == right.EmbeddingSHA &&
		left.EmbeddingRev != "" && left.EmbeddingRev == right.EmbeddingRev &&
		len(left.Embedding) == len(right.Embedding) {
		if score, ok := cosineSimilarity(left.Embedding, right.Embedding); ok {
			visualSimilarity = map[string]any{
				"available": true, "score": score, "distance_metric": "cosine_similarity",
				"model": left.EmbeddingModel, "model_revision": left.EmbeddingRev,
				"model_sha256": left.EmbeddingSHA, "embedding_dimension": len(left.Embedding),
				"semantics": "candidate semantic visual similarity; not evidence identity or fact",
			}
		}
	}
	return map[string]any{
		"contract_version": imageComparisonContract,
		"source_a":         imageSourceView(left), "source_b": imageSourceView(right),
		"sha256_match": shaMatch, "exact_duplicate": shaMatch,
		"dimensions": map[string]any{"source_a": left.Dimensions, "source_b": right.Dimensions, "match": dimensionsAvailable(left.Dimensions) && dimensionsAvailable(right.Dimensions) && equalJSON(left.Dimensions, right.Dimensions)},
		"perceptual": map[string]any{
			"algorithm": imagePerceptualHashAlgorithm, "hash_a": optionalString(left.PerceptualHash), "hash_b": optionalString(right.PerceptualHash),
			"hamming_distance": distance, "similarity": similarity, "near_duplicate_max_distance": imageNearDuplicateMaxDistance,
			"near_duplicate_candidate": nearDuplicate, "semantics": "pixel-layout resemblance; not semantic visual similarity",
		},
		"ocr": overlapView(left.OCRTokens, right.OCRTokens), "anpr": overlapView(left.Plates, right.Plates),
		"faces":             map[string]any{"source_a_count": left.FaceCount, "source_b_count": right.FaceCount, "similarity": nil},
		"visual_similarity": visualSimilarity,
		"limitations": []string{
			"Perceptual similarity is a near-duplicate signal, not scene understanding or identity.",
			"Missing OCR, ANPR, or face observations are reported as unavailable rather than inferred.",
		},
	}
}

func imageSourceView(signal imageComparisonSignal) map[string]any {
	return map[string]any{
		"evidence_id": signal.EvidenceID, "version_id": signal.VersionID, "source_file": signal.SourceFile,
		"sha256": signal.SHA256, "citation_ref": signal.CitationRef, "artifact_refs": signal.ArtifactRefs,
	}
}

func validDHash(value string) bool {
	if len(value) != 16 {
		return false
	}
	for _, char := range value {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return true
}

func parseDHash(value string) uint64 {
	var result uint64
	for _, char := range value {
		result <<= 4
		switch {
		case char >= '0' && char <= '9':
			result |= uint64(char - '0')
		case char >= 'a' && char <= 'f':
			result |= uint64(char-'a') + 10
		case char >= 'A' && char <= 'F':
			result |= uint64(char-'A') + 10
		}
	}
	return result
}

func overlapView(left, right map[string]struct{}) map[string]any {
	shared := []string{}
	union := map[string]struct{}{}
	for value := range left {
		union[value] = struct{}{}
		if _, ok := right[value]; ok {
			shared = append(shared, value)
		}
	}
	for value := range right {
		union[value] = struct{}{}
	}
	sort.Strings(shared)
	var score *float64
	if len(union) > 0 {
		value := float64(len(shared)) / float64(len(union))
		score = &value
	}
	return map[string]any{"available": len(union) > 0, "shared": shared, "jaccard": score}
}

func dimensionsAvailable(value map[string]any) bool {
	return value["width_pixels"] != nil && value["height_pixels"] != nil
}

func imageTextTokens(value string) []string {
	return strings.Fields(strings.ToLower(strings.Map(func(r rune) rune {
		if r == '_' || r == '-' {
			return ' '
		}
		return r
	}, value)))
}

func normalizeImagePlate(value string) string {
	return strings.ToUpper(strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value))
}

func objectValue(value any) map[string]any {
	if object, ok := value.(map[string]any); ok {
		return object
	}
	var object map[string]any
	var data []byte
	switch typed := value.(type) {
	case []byte:
		data = typed
	case string:
		data = []byte(typed)
	case json.RawMessage:
		data = typed
	default:
		return map[string]any{}
	}
	if json.Unmarshal(data, &object) != nil {
		return map[string]any{}
	}
	return object
}

func imageStringValue(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	if data, ok := value.([]byte); ok {
		return string(data)
	}
	return fmt.Sprint(value)
}

func optionalString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func equalJSON(left, right any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}
