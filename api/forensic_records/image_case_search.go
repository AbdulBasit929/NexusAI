package main

// Case-wide image search (module M1; plan docs/roadmap/media-modules-reconciled-plan-20261120.md, gates docs/data/image-search-m1-preregistration-20261120.md).
//
// One governed endpoint instead of "name your evidence ids": inside ONE collection (and optionally one case) it finds
//   - the identical file (same SHA-256) stored under another evidence id,
//   - "the same image, re-saved" (256-bit perceptual hash within a calibrated Hamming distance),
//   - "the same photograph, edited or cropped" (DINOv2 vector within a calibrated cosine distance),
//   - photos by words (SigLIP 2 text against SigLIP 2 image vectors, ranked; a phrase in another script is translated to English by the language model first),
//   - more like this image (SigLIP 2 neighbours, ranked).
// The search is an exact scan of the case's current evidence versions, so an answer can say how much of the case it covered (the coverage line). Every search leaves a row in forensic.image_search_log.
// Nothing here identifies a person or an event: a match is a candidate for an analyst to look at. No raw vector is ever returned.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/httpclient"
)

const (
	imageCaseSearchContract  = "forensics.image-case-search/v1"
	imageSearchDinoModelID   = "facebook/dinov2-base"
	imageSearchSiglipModelID = "google/siglip2-base-patch16-224"
	imageSearchMaxUpload     = 32 << 20
	imageSearchMaxTopK       = 100
	imageSearchDefaultTopK   = 20
	imageSearchListLimit     = 1000
)

// imageSearchSettings carries the calibrated distances. The defaults are the figures measured on the development part of the M1 acceptance set (docs/data/image-search-m1-results-*.md); they can be
// overridden in the environment without a rebuild so that a recalibration on real casework needs no code change.
type imageSearchSettings struct {
	ServiceURL         string
	ServiceKey         string
	HashMaxDistance    int
	HashBand           int
	DinoMaxDistance    float64
	DinoBand           float64
	WindowOnlyRatio    float64 // a picture outside the ranked tiers is listed as "in a part of the picture" when its best window scores at least this fraction of the best window of the case
	WindowPenalty      float64 // subtracted from a WINDOW's similarity before it competes with the whole picture's (the best of many noisy windows is biased upward); calibrated offline from whole_similarity and window_similarity
	WordsBestRatio     float64 // a picture is a "best match" for a phrase when its similarity is at least this fraction of the best one (0.88: precision 0.97, recall 0.71 on 16 queries against COCO captions, docs/roadmap/image-search-real-world-plan-20261123.md)
	WordsPossibleRatio float64 // and "also possible" from this fraction (0.75: precision 0.78, recall 0.79); weaker pictures are not listed
	TileDinoMax        float64 // the largest DINOv2 cosine distance between the attached picture and a WINDOW of another picture for "a part of it looks like this" (image_tile_search.go); provisional until calibrated
	SimilarMin         float64 // the lowest similarity (1 minus the cosine distance) at which an image is listed as visually similar; provisional, chosen from the live Lahore case (similar vehicles 0.80 to 0.84, an unrelated picture 0.53 to 0.57)
	Timeout            time.Duration
}

func imageSearchSettingsFromEnv() imageSearchSettings {
	atoi := func(key string, fallback int) int {
		if value, err := strconv.Atoi(strings.TrimSpace(env(key, ""))); err == nil && value >= 0 {
			return value
		}
		return fallback
	}
	atof := func(key string, fallback float64) float64 {
		if value, err := strconv.ParseFloat(strings.TrimSpace(env(key, "")), 64); err == nil && value >= 0 {
			return value
		}
		return fallback
	}
	return imageSearchSettings{
		ServiceURL:         strings.TrimRight(env("FORENSIC_IMAGE_SEARCH_URL", ""), "/"),
		ServiceKey:         env("FORENSIC_IMAGE_SEARCH_SERVICE_KEY", ""),
		HashMaxDistance:    atoi("FORENSIC_IMAGE_SEARCH_HASH_MAX", 56),
		HashBand:           atoi("FORENSIC_IMAGE_SEARCH_HASH_BAND", 20),
		DinoMaxDistance:    atof("FORENSIC_IMAGE_SEARCH_DINO_MAX", 0.108),
		DinoBand:           atof("FORENSIC_IMAGE_SEARCH_DINO_BAND", 0.03),
		WindowPenalty:      atof("FORENSIC_IMAGE_SEARCH_WINDOW_PENALTY", 0.06), // measured: the ranking is best when windows never win (tools/media_data/image_words_penalty_calibration.py)
		WindowOnlyRatio:    atof("FORENSIC_IMAGE_SEARCH_WINDOW_ONLY_RATIO", 0.92),
		WordsBestRatio:     atof("FORENSIC_IMAGE_SEARCH_WORDS_BEST_RATIO", 0.88),
		WordsPossibleRatio: atof("FORENSIC_IMAGE_SEARCH_WORDS_POSSIBLE_RATIO", 0.75),
		TileDinoMax:        atof("FORENSIC_IMAGE_SEARCH_TILE_DINO_MAX", 0.30),
		SimilarMin:         atof("FORENSIC_IMAGE_SEARCH_SIMILAR_MIN", 0.70),
		Timeout:            60 * time.Second,
	}
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------- collaborators

type imageProbeVectors struct {
	PDQHex  string
	Vectors map[string][]float64
}

type imageEmbedClient interface {
	EmbedText(ctx context.Context, text string) ([]float64, error)
	EmbedImage(ctx context.Context, data []byte) (imageProbeVectors, error)
}

type imageQueryTranslator interface {
	ToEnglish(ctx context.Context, text string) (string, error)
}

type httpImageEmbedClient struct{ settings imageSearchSettings }

var errImageSearchServiceDown = errors.New("the image search service is not configured or not reachable")

func (c httpImageEmbedClient) post(ctx context.Context, path, contentType string, body []byte, out any) error {
	if c.settings.ServiceURL == "" || c.settings.ServiceKey == "" {
		return errImageSearchServiceDown
	}
	callCtx, cancel := context.WithTimeout(ctx, c.settings.Timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, c.settings.ServiceURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("X-Image-Search-Key", c.settings.ServiceKey)
	response, err := httpclient.NewWithTimeout(c.settings.Timeout).Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", errImageSearchServiceDown, err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return err
	}
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("the image search service answered HTTP %d", response.StatusCode)
	}
	return json.Unmarshal(raw, out)
}

func (c httpImageEmbedClient) EmbedText(ctx context.Context, text string) ([]float64, error) {
	payload, _ := json.Marshal(map[string]string{"text": text})
	var out struct {
		Vector []float64 `json:"vector"`
	}
	if err := c.post(ctx, "/embed-text", "application/json", payload, &out); err != nil {
		return nil, err
	}
	if len(out.Vector) == 0 {
		return nil, errors.New("the image search service returned no vector")
	}
	return out.Vector, nil
}

func (c httpImageEmbedClient) EmbedImage(ctx context.Context, data []byte) (imageProbeVectors, error) {
	var out struct {
		PDQ     string               `json:"pdq256_hex"`
		Vectors map[string][]float64 `json:"vectors"`
	}
	if err := c.post(ctx, "/embed-image", "application/octet-stream", data, &out); err != nil {
		return imageProbeVectors{}, err
	}
	return imageProbeVectors{PDQHex: out.PDQ, Vectors: out.Vectors}, nil
}

// localAITranslator asks the resident language model for an English rendering of a search phrase. Temperature 0, a short answer, nothing else.
type localAITranslator struct{ cfg config }

func (t localAITranslator) ToEnglish(ctx context.Context, text string) (string, error) {
	if t.cfg.LocalAIURL == "" || t.cfg.SynthesisModel == "" {
		return "", errors.New("no language model is configured for translating the search phrase")
	}
	payload, err := json.Marshal(map[string]any{
		"model": t.cfg.SynthesisModel, "temperature": 0, "max_tokens": 80,
		"messages": []map[string]string{
			{"role": "system", "content": "You translate a short search phrase about photographs into plain English. Reply with the English phrase only: no quotes, no notes, no explanation."},
			{"role": "user", "content": text},
		},
	})
	if err != nil {
		return "", err
	}
	callCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(callCtx, http.MethodPost, strings.TrimRight(t.cfg.LocalAIURL, "/")+"/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	if t.cfg.LocalAIAPIKey != "" {
		request.Header.Set("Authorization", "Bearer "+t.cfg.LocalAIAPIKey)
	}
	response, err := httpclient.NewWithTimeout(90 * time.Second).Do(request)
	if err != nil {
		return "", err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("the language model answered HTTP %d", response.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var envelope struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || len(envelope.Choices) == 0 {
		return "", errors.New("the language model's translation could not be used")
	}
	return cleanTranslation(envelope.Choices[0].Message.Content), nil
}

// cleanTranslation keeps the first non-empty line, drops surrounding quotes and cuts it to the 512 characters the text encoder accepts.
func cleanTranslation(raw string) string {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "\"'`“”"))
		if line != "" {
			if len([]rune(line)) > 512 {
				line = string([]rune(line)[:512])
			}
			return line
		}
	}
	return ""
}

// needsTranslation is true when the phrase has a letter outside the Latin script (Arabic-script Urdu, Pashto, Saraiki, Devanagari, Chinese ...). Roman Urdu is Latin script and is NOT detected:
// it is searched as written, and the limitations line says so.
func needsTranslation(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) && !unicode.Is(unicode.Latin, r) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------- request and response

type imageSearchRequest struct {
	Kind        string // copies | copies_of_upload | by_words | more_like_this
	Scope       forensicScope
	EvidenceID  string
	Text        string
	Upload      []byte
	UploadName  string
	TopK        int
	QueryLang   string
	TranslatedQ string
	// a part drawn on a stored picture (image_region_crop.go): the words the answer uses for it, and the picture itself, which is left out of the results
	Subject         string
	ExcludeEvidence string
}

type imageSearchHit struct {
	EvidenceID       string   `json:"evidence_id"`
	VersionID        string   `json:"version_id,omitempty"`
	OriginalFilename string   `json:"original_filename"`
	ContentType      *string  `json:"content_type"`
	CaseID           *string  `json:"case_id"`
	Tier             string   `json:"tier"`
	TierLabel        string   `json:"tier_label"`
	Rank             int      `json:"rank,omitempty"`
	HashDistance     *int     `json:"hash_distance_bits,omitempty"`
	CosineDistance   *float64 `json:"cosine_distance,omitempty"`
	Similarity       *float64 `json:"similarity,omitempty"`
	CitationRef      string   `json:"citation_ref,omitempty"`
	ContentPath      string   `json:"content_path"`
	// region search (image_region_search.go): how many local-feature matches agree on one transform, where the attached picture sits in this image (x0, y0, x1, y1 as fractions of its width and height),
	// and whether it matched mirrored
	Inliers  *int      `json:"inliers,omitempty"`
	Matches  *int      `json:"matches,omitempty"`
	Box      []float64 `json:"box,omitempty"`
	Mirrored bool      `json:"mirrored,omitempty"`
	// duplicate groups (image_duplicate_groups.go): which group this picture is in and how many pictures the group has
	Group     int `json:"group,omitempty"`
	GroupSize int `json:"group_size,omitempty"`
	// photos by words: the similarity of the whole picture and of its best window, kept apart so that the window penalty can be calibrated
	WholeSimilarity  *float64 `json:"whole_similarity,omitempty"`
	WindowSimilarity *float64 `json:"window_similarity,omitempty"`
}

type imageCoverage struct {
	ImagesInCase            int64  `json:"images_in_case"`
	ImagesIndexed           int64  `json:"images_indexed"`
	ImagesFailed            int64  `json:"images_failed"`
	ImagesIdenticalReupload int64  `json:"images_identical_reupload"`
	Statement               string `json:"statement"`
}

func coverageStatement(c imageCoverage) string {
	text := fmt.Sprintf("Searched %d of %d images in this case.", c.ImagesIndexed, c.ImagesInCase)
	if pending := c.ImagesInCase - c.ImagesIndexed - c.ImagesFailed; pending > 0 {
		text += fmt.Sprintf(" %d have not been processed yet and are not included.", pending)
	}
	if c.ImagesFailed > 0 {
		text += fmt.Sprintf(" %d could not be processed.", c.ImagesFailed)
	}
	if c.ImagesIdenticalReupload > 0 {
		text += fmt.Sprintf(" %d more are re-uploads of an identical file.", c.ImagesIdenticalReupload)
	}
	return text
}

var imageTierLabels = map[string]string{
	"identical_file":  "identical file (same SHA-256)",
	"same_image":      "the same image, re-saved or resized",
	"same_photograph": "the same photograph, re-compressed or lightly edited",
	"similar":         "visually similar, ranked (not a copy)",
	"ranked":          "ranked by similarity",
}

var imageTierOrder = map[string]int{"identical_file": 0, "same_image": 1, "same_photograph": 2, "ranked": 3, "similar": 3}

// mergeCopyHits takes the three raw result lists of a "find copies" search and returns one list: each image under the strongest tier it reached (identical file, then same image, then same
// photograph), nearest first within a tier, plus the images that fell just outside both distances (not matches, counted so the analyst knows the edge).
func mergeCopyHits(identical, byHash, byVector []imageSearchHit, hashMax int, dinoMax float64) (matches []imageSearchHit, justBeyond int) {
	best := map[string]imageSearchHit{}
	consider := func(h imageSearchHit) {
		key := h.EvidenceID
		if existing, ok := best[key]; ok {
			if imageTierOrder[h.Tier] > imageTierOrder[existing.Tier] {
				// keep the stronger tier but remember the other measurement
				existing.HashDistance = firstInt(existing.HashDistance, h.HashDistance)
				existing.CosineDistance = firstFloat(existing.CosineDistance, h.CosineDistance)
				best[key] = existing
				return
			}
			h.HashDistance = firstInt(h.HashDistance, existing.HashDistance)
			h.CosineDistance = firstFloat(h.CosineDistance, existing.CosineDistance)
		}
		best[key] = h
	}
	for _, h := range identical {
		h.Tier = "identical_file"
		consider(h)
	}
	for _, h := range byHash {
		if h.HashDistance == nil {
			continue
		}
		if *h.HashDistance <= hashMax {
			h.Tier = "same_image"
			consider(h)
		}
	}
	for _, h := range byVector {
		if h.CosineDistance == nil {
			continue
		}
		if *h.CosineDistance <= dinoMax {
			sim := 1 - *h.CosineDistance
			h.Similarity = &sim
			h.Tier = "same_photograph"
			consider(h)
		}
	}
	seenBeyond := map[string]bool{}
	for _, h := range byHash {
		if h.HashDistance != nil && *h.HashDistance > hashMax {
			if _, matched := best[h.EvidenceID]; !matched {
				seenBeyond[h.EvidenceID] = true
			}
		}
	}
	for _, h := range byVector {
		if h.CosineDistance != nil && *h.CosineDistance > dinoMax {
			if _, matched := best[h.EvidenceID]; !matched {
				seenBeyond[h.EvidenceID] = true
			}
		}
	}
	matches = make([]imageSearchHit, 0, len(best))
	for _, h := range best {
		h.TierLabel = imageTierLabels[h.Tier]
		matches = append(matches, h)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		a, b := matches[i], matches[j]
		if a.Tier != b.Tier {
			return imageTierOrder[a.Tier] < imageTierOrder[b.Tier]
		}
		da, db := hitDistance(a), hitDistance(b)
		if da != db {
			return da < db
		}
		return a.EvidenceID < b.EvidenceID
	})
	for i := range matches {
		matches[i].Rank = i + 1
	}
	return matches, len(seenBeyond)
}

func hitDistance(h imageSearchHit) float64 {
	switch h.Tier {
	case "same_image":
		if h.HashDistance != nil {
			return float64(*h.HashDistance)
		}
	case "same_photograph":
		if h.CosineDistance != nil {
			return *h.CosineDistance
		}
	}
	return 0
}

func firstInt(a, b *int) *int {
	if a != nil {
		return a
	}
	return b
}

func firstFloat(a, b *float64) *float64 {
	if a != nil {
		return a
	}
	return b
}

func vectorLiteral(vector []float64) string {
	parts := make([]string, len(vector))
	for i, v := range vector {
		parts[i] = strconv.FormatFloat(v, 'g', 8, 64)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func bitString(pdqHex string) (string, error) {
	raw, err := hex.DecodeString(pdqHex)
	if err != nil || len(raw) != 32 {
		return "", errors.New("the perceptual hash must be 64 hexadecimal characters")
	}
	var b strings.Builder
	for _, octet := range raw {
		fmt.Fprintf(&b, "%08b", octet)
	}
	return b.String(), nil
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------- SQL

const imageSearchProbeSQL = `
SELECT e.sha256, e.original_filename, h.pdq256::text,
       (SELECT ce.embedding::text FROM forensic.content_embedding ce
          JOIN forensic.derived_artifacts a ON a.tenant_id = ce.tenant_id AND a.collection_id = ce.collection_id AND a.artifact_id = ce.derived_artifact_id
         WHERE ce.tenant_id = e.tenant_id AND ce.collection_id = e.collection_id AND ce.evidence_id = e.evidence_id AND ce.modality = 'image'
           AND ce.model_id = $3 AND a.version_id = e.current_version_id ORDER BY ce.chunk_ref LIMIT 1) AS vector_text
FROM forensic.evidence_items e
LEFT JOIN forensic.image_hash h ON h.tenant_id = e.tenant_id AND h.collection_id = e.collection_id AND h.evidence_id = e.evidence_id AND h.version_id = e.current_version_id
WHERE e.tenant_id = current_setting('app.tenant_id', true) AND e.collection_id = $1 AND e.evidence_id = $2::uuid AND e.modality = 'image'
  AND (NULLIF($4, '') IS NULL OR e.case_id = $4)
ORDER BY h.chunk_ref
LIMIT 1`

const imageSearchHashSQL = `
SELECT evidence_id::text, version_id::text, original_filename, content_type, case_id, distance, citation_ref
FROM forensic.image_search_hash($1, $2::bit(256), $3, NULLIF($4, ''), $5::uuid, $6)`

const imageSearchVectorSQL = `
SELECT evidence_id::text, version_id::text, original_filename, content_type, case_id, distance, citation_ref
FROM forensic.image_search_vector($1, $2, $3::vector, $4, NULLIF($5, ''), $6::uuid, $7)`

const imageSearchExactSQL = `
SELECT evidence_id::text, original_filename, content_type, case_id
FROM forensic.image_exact_matches($1, $2, NULLIF($3, ''), $4::uuid, $5)`

const imageSearchCoverageSQL = `SELECT images_in_case, images_indexed, images_failed, images_identical_reupload FROM forensic.image_index_state($1, NULLIF($2, ''))`

const imageSearchLogSQL = `
INSERT INTO forensic.image_search_log (tenant_id, collection_id, case_id, searched_by, kind, probe_evidence_id, probe_sha256, query_text, query_language, parameters,
                                       result_count, images_in_case, images_indexed, elapsed_ms)
VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6::uuid, NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), $10::jsonb, $11, $12, $13, $14)
RETURNING search_id::text`

func scanHits(rows pgx.Rows, tier string, distanceKind string) ([]imageSearchHit, error) {
	defer rows.Close()
	var hits []imageSearchHit
	for rows.Next() {
		var h imageSearchHit
		if distanceKind == "exact" {
			if err := rows.Scan(&h.EvidenceID, &h.OriginalFilename, &h.ContentType, &h.CaseID); err != nil {
				return nil, err
			}
		} else if distanceKind == "hash" {
			var distance int32
			if err := rows.Scan(&h.EvidenceID, &h.VersionID, &h.OriginalFilename, &h.ContentType, &h.CaseID, &distance, &h.CitationRef); err != nil {
				return nil, err
			}
			d := int(distance)
			h.HashDistance = &d
		} else {
			var distance float64
			if err := rows.Scan(&h.EvidenceID, &h.VersionID, &h.OriginalFilename, &h.ContentType, &h.CaseID, &distance, &h.CitationRef); err != nil {
				return nil, err
			}
			h.CosineDistance = &distance
		}
		h.Tier = tier
		h.ContentPath = "/evidence/" + h.EvidenceID + "/content"
		hits = append(hits, h)
	}
	return hits, rows.Err()
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------- the search

type imageSearchDeps struct {
	db         *pgxpool.Pool
	settings   imageSearchSettings
	embed      imageEmbedClient
	translator imageQueryTranslator
	crop       imageCropClient       // cuts a drawn box out of a stored picture
	store      contentAddressedStore // the retained evidence bytes, for region search (image_region_search.go); empty = no region search
	region     imageRegionClient
}

type imageSearchError struct {
	Status  int
	Message string
}

func (e imageSearchError) Error() string { return e.Message }

func runImageCaseSearch(ctx context.Context, deps imageSearchDeps, req imageSearchRequest) (map[string]any, error) {
	started := time.Now()
	// reference / reference_upload: "find exact and similar to this image" = the copy tiers, then a ranked list of visually similar images below them
	requestedKind, withSimilar := req.Kind, false
	switch req.Kind {
	case "reference":
		req.Kind, withSimilar = "copies", true
	case "reference_upload":
		req.Kind, withSimilar = "copies_of_upload", true
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	scope := req.Scope
	tx, err := deps.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("begin image search: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", scope.TenantID); err != nil {
		return nil, fmt.Errorf("set tenant context: %w", err)
	}
	var coverage imageCoverage
	if err := tx.QueryRow(ctx, imageSearchCoverageSQL, scope.CollectionID, scope.CaseID).Scan(&coverage.ImagesInCase, &coverage.ImagesIndexed, &coverage.ImagesFailed, &coverage.ImagesIdenticalReupload); err != nil {
		return nil, fmt.Errorf("coverage: %w", err)
	}
	coverage.Statement = coverageStatement(coverage)

	response := map[string]any{
		"contract_version": imageCaseSearchContract,
		"kind":             requestedKind,
		"scope":            map[string]any{"tenant_id": scope.TenantID, "collection_id": scope.CollectionID, "case_id": optionalString(scope.CaseID)},
		"coverage":         coverage,
	}
	var (
		hits          []imageSearchHit
		similarRegion []imageSearchHit // pictures with a part that looks like the attached one (reference_upload)
		probeSHA      string
		probeID       string
		queryText     string
		resultCount   int
		parameters    = map[string]any{}
	)
	limitations := []string{
		"A match is a candidate for an analyst to look at. It does not show that two images are the same evidence, the same event, or true.",
		"Only images that have been processed are searched; the coverage line says how many that is.",
	}

	switch req.Kind {
	case "copies", "copies_of_upload", "more_like_this":
		var probeHash, probeVector, probeSiglip string
		if req.Kind == "copies_of_upload" {
			if deps.embed == nil {
				return nil, imageSearchError{http.StatusServiceUnavailable, errImageSearchServiceDown.Error()}
			}
			sum := sha256.Sum256(req.Upload)
			probeSHA = hex.EncodeToString(sum[:])
			probe, err := deps.embed.EmbedImage(ctx, req.Upload)
			if err != nil {
				if errors.Is(err, errImageSearchServiceDown) {
					return nil, imageSearchError{http.StatusServiceUnavailable, err.Error()}
				}
				return nil, imageSearchError{http.StatusBadRequest, "the uploaded file could not be read as an image: " + err.Error()}
			}
			bits, err := bitString(probe.PDQHex)
			if err != nil {
				return nil, err
			}
			probeHash = bits
			if v, ok := probe.Vectors["dinov2"]; ok {
				probeVector = vectorLiteral(v)
			}
			if v, ok := probe.Vectors["siglip2"]; ok {
				probeSiglip = vectorLiteral(v)
			}
		} else {
			if _, err := uuid.Parse(req.EvidenceID); err != nil {
				return nil, imageSearchError{http.StatusBadRequest, "evidence_id must be a UUID"}
			}
			probeID = req.EvidenceID
			var hashText, vecText *string
			model := imageSearchDinoModelID
			if req.Kind == "more_like_this" {
				model = imageSearchSiglipModelID
			}
			if err := tx.QueryRow(ctx, imageSearchProbeSQL, scope.CollectionID, req.EvidenceID, model, scope.CaseID).Scan(&probeSHA, new(string), &hashText, &vecText); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, imageSearchError{http.StatusNotFound, "that image was not found in this case"}
				}
				return nil, fmt.Errorf("probe: %w", err)
			}
			if hashText != nil {
				probeHash = *hashText
			}
			if vecText != nil {
				probeVector = *vecText
			}
			if withSimilar {
				var siglipText *string
				if err := tx.QueryRow(ctx, imageSearchProbeSQL, scope.CollectionID, req.EvidenceID, imageSearchSiglipModelID, scope.CaseID).Scan(new(string), new(string), new(*string), &siglipText); err == nil && siglipText != nil {
					probeSiglip = *siglipText
				}
			}
		}
		if req.Kind == "more_like_this" {
			text := probeVector
			if text == "" {
				return nil, imageSearchError{http.StatusConflict, "that image has not been processed for similarity search yet"}
			}
			rows, err := tx.Query(ctx, imageSearchVectorSQL, scope.CollectionID, imageSearchSiglipModelID, text, 2.0, scope.CaseID, nullableUUID(probeID), req.TopK)
			if err != nil {
				return nil, err
			}
			if hits, err = scanHits(rows, "ranked", "vector"); err != nil {
				return nil, err
			}
			kept := hits[:0]
			for _, h := range hits {
				if 1-*h.CosineDistance >= deps.settings.SimilarMin {
					kept = append(kept, h)
				}
			}
			hits = kept
			for i := range hits {
				sim := 1 - *hits[i].CosineDistance
				hits[i].Similarity = &sim
				hits[i].TierLabel = imageTierLabels["ranked"]
				hits[i].Rank = i + 1
			}
			parameters["top_k"] = req.TopK
			limitations = append(limitations, "Ranked by visual similarity, nearest first; this is not a complete list of every image that looks alike, and its accuracy beyond near-duplicates has not been measured.")
			break
		}
		if probeHash == "" {
			return nil, imageSearchError{http.StatusConflict, "that image has not been processed for copy search yet"}
		}
		var identical, byHash, byVector []imageSearchHit
		if probeSHA != "" {
			rows, err := tx.Query(ctx, imageSearchExactSQL, scope.CollectionID, probeSHA, scope.CaseID, nullableUUID(probeID), imageSearchListLimit)
			if err != nil {
				return nil, err
			}
			if identical, err = scanHits(rows, "identical_file", "exact"); err != nil {
				return nil, err
			}
		}
		rows, err := tx.Query(ctx, imageSearchHashSQL, scope.CollectionID, probeHash, deps.settings.HashMaxDistance+deps.settings.HashBand, scope.CaseID, nullableUUID(probeID), imageSearchListLimit)
		if err != nil {
			return nil, err
		}
		if byHash, err = scanHits(rows, "same_image", "hash"); err != nil {
			return nil, err
		}
		if probeVector != "" {
			rows, err := tx.Query(ctx, imageSearchVectorSQL, scope.CollectionID, imageSearchDinoModelID, probeVector, deps.settings.DinoMaxDistance+deps.settings.DinoBand, scope.CaseID, nullableUUID(probeID), imageSearchListLimit)
			if err != nil {
				return nil, err
			}
			if byVector, err = scanHits(rows, "same_photograph", "vector"); err != nil {
				return nil, err
			}
		} else {
			limitations = append(limitations, "The edited-or-cropped search was not run because this image has no similarity vector yet.")
		}
		var beyond int
		hits, beyond = mergeCopyHits(identical, byHash, byVector, deps.settings.HashMaxDistance, deps.settings.DinoMaxDistance)
		response["just_outside_the_distance"] = map[string]any{"count": beyond, "note": "Images that came close to, but outside, the distance used for a match. They are not matches."}
		if withSimilar {
			if probeSiglip == "" {
				limitations = append(limitations, "The list of similar images was not made because this image has no similarity vector yet.")
			} else {
				simRows, err := tx.Query(ctx, imageSearchVectorSQL, scope.CollectionID, imageSearchSiglipModelID, probeSiglip, 2.0, scope.CaseID, nullableUUID(probeID), req.TopK+len(hits))
				if err != nil {
					return nil, err
				}
				similar, err := scanHits(simRows, "similar", "vector")
				if err != nil {
					return nil, err
				}
				hits = appendSimilar(hits, similar, req.TopK, deps.settings.SimilarMin)
				parameters["include_similar"], parameters["top_k"] = true, req.TopK
				limitations = append(limitations, "Similar images are ranked by visual and semantic likeness, nearest first. They are not copies, the list is not complete, and its accuracy beyond near-duplicates has not been measured.")
			}
		}
		// other pictures with a PART that looks like the attached one (DINOv2 vectors of their windows); shown apart from the verified region matches and apart from the look-alikes
		if withSimilar && probeVector != "" && deps.settings.TileDinoMax > 0 {
			tileRows, err := tx.Query(ctx, imageSearchTileSQL, scope.CollectionID, imageSearchDinoModelID+imageTileModelSuffix, probeVector, deps.settings.TileDinoMax, scope.CaseID, nullableUUID(probeID), imageTileLimit)
			if err != nil {
				return nil, err
			}
			if similarRegion, err = scanTileHits(tileRows, imageTierSimilarRegion); err != nil {
				return nil, err
			}
			parameters["tile_dino_max_cosine_distance"] = deps.settings.TileDinoMax
		}
		parameters["hash_max_bits"], parameters["dino_max_cosine_distance"] = deps.settings.HashMaxDistance, deps.settings.DinoMaxDistance
		limitations = append(limitations,
			"Measured on public photographs: copies that were re-saved, resized, re-compressed, mirrored, lightly rotated, toned or captioned were found; a heavy crop (60 percent of the picture), a picture inside a white frame, and a picture inside a full-screen phone screenshot were often not found. No match does not prove that no copy exists.",
			"Photographs of the same scene taken a moment apart can come back as matches; the analyst has to tell them apart.")

	case "by_words":
		text := strings.TrimSpace(req.Text)
		if text == "" || len([]rune(text)) > 512 {
			return nil, imageSearchError{http.StatusBadRequest, "q must contain 1 to 512 characters"}
		}
		queryText = text
		english := text
		if needsTranslation(text) {
			if deps.translator == nil {
				return nil, imageSearchError{http.StatusServiceUnavailable, "no language model is available to translate the phrase"}
			}
			translated, err := deps.translator.ToEnglish(ctx, text)
			if err != nil || translated == "" {
				return nil, imageSearchError{http.StatusServiceUnavailable, "the phrase could not be translated to English"}
			}
			english = translated
			response["translated_query"] = translated
			limitations = append(limitations, "The phrase was translated to English by the language model before searching; the accuracy of that route has not been measured on reviewed captions.")
		}
		if deps.embed == nil {
			return nil, imageSearchError{http.StatusServiceUnavailable, errImageSearchServiceDown.Error()}
		}
		vector, err := deps.embed.EmbedText(ctx, english)
		if err != nil {
			if errors.Is(err, errImageSearchServiceDown) {
				return nil, imageSearchError{http.StatusServiceUnavailable, err.Error()}
			}
			return nil, err
		}
		rows, err := tx.Query(ctx, imageSearchVectorSQL, scope.CollectionID, imageSearchSiglipModelID, vectorLiteral(vector), 2.0, scope.CaseID, nil, imageSearchListLimit)
		if err != nil {
			return nil, err
		}
		if hits, err = scanHits(rows, "ranked", "vector"); err != nil {
			return nil, err
		}
		// a picture also scores as its best WINDOW (a small object or a detail that the whole-picture vector does not say), and the window's box says where
		tileRows, err := tx.Query(ctx, imageSearchTileSQL, scope.CollectionID, imageSearchSiglipModelID+imageTileModelSuffix, vectorLiteral(vector), 2.0, scope.CaseID, nil, imageTileLimit)
		if err != nil {
			return nil, err
		}
		windows, err := scanTileHits(tileRows, "ranked")
		if err != nil {
			return nil, err
		}
		hits = tierWordHits(mergeWordHits(hits, windows, imageSearchListLimit, deps.settings.WindowPenalty), deps.settings.WordsBestRatio, deps.settings.WordsPossibleRatio, req.TopK)
		listed := map[string]bool{}
		for _, h := range hits {
			listed[h.EvidenceID] = true
		}
		hits = append(hits, windowOnlyHits(windows, listed, deps.settings.WindowOnlyRatio)...)
		for i := range hits {
			sim := 1 - *hits[i].CosineDistance
			hits[i].Similarity = &sim
			hits[i].TierLabel = imageTierLabels[hits[i].Tier]
			hits[i].Rank = i + 1
		}
		parameters["top_k"], parameters["words_best_ratio"], parameters["words_possible_ratio"] = req.TopK, deps.settings.WordsBestRatio, deps.settings.WordsPossibleRatio
		limitations = append(limitations,
			"Pictures are scored by how well they match the words and listed best first: the best matches, then weaker ones that are also possible. A description has no sharp edge, so this is not a complete list of every picture that could match.",
			"A search by words always returns the closest pictures, even when nothing in the case shows what you described: look at them to judge.",
			"Roman Urdu (Urdu written in English letters) is searched as written and is not translated.")

	case "duplicate_groups":
		dupHits, summary, err := imageDuplicateGroupsRun(ctx, tx, scope, deps.settings)
		if err != nil {
			return nil, err
		}
		hits = dupHits
		response["duplicates"] = summary
		parameters["duplicate_groups"] = true
		req.Kind = "copies" // logged under a kind the log accepts; the parameters say what was asked
		limitations = append(limitations, "Groups are made from the stored descriptors with the same distances as the copy search. A group is a set of candidates for review: nothing is hidden, merged or removed, and every member opens as its own file.")

	default:
		return nil, imageSearchError{http.StatusBadRequest, "kind must be copies, copies_of_upload, reference, reference_upload, by_words, more_like_this or duplicate_groups"}
	}

	if hits == nil {
		hits = []imageSearchHit{}
	}
	resultCount = len(hits)
	response["results"] = hits
	response["result_count"] = resultCount
	response["limitations"] = limitations
	if probeID != "" {
		response["probe"] = map[string]any{"evidence_id": probeID}
	} else if probeSHA != "" {
		response["probe"] = map[string]any{"uploaded_file_sha256": probeSHA}
	}
	actor := imageSearchActor(scope)
	parameterJSON, _ := json.Marshal(parameters)
	var searchID string
	if err := tx.QueryRow(ctx, imageSearchLogSQL, scope.TenantID, scope.CollectionID, scope.CaseID, actor, req.Kind, nullableUUID(probeID), probeSHA, queryText, req.QueryLang, string(parameterJSON),
		resultCount, coverage.ImagesInCase, coverage.ImagesIndexed, int(time.Since(started).Milliseconds())).Scan(&searchID); err != nil {
		return nil, fmt.Errorf("write the search log: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit image search: %w", err)
	}
	response["search_id"] = searchID
	// a picture the analyst attached may also be a REGION of other pictures (a crop, a detail, a logo): the local-feature check says which images contain it and where. Whole-picture copies are not
	// checked again.
	if requestedKind == "reference_upload" && len(req.Upload) > 0 {
		var copies, rest []imageSearchHit
		for _, h := range withoutPicture(hits, req.ExcludeEvidence) {
			if copyTier(h.Tier) {
				copies = append(copies, h)
			} else {
				rest = append(rest, h)
			}
		}
		var regionHits []imageSearchHit
		if deps.region != nil {
			skip := map[string]bool{}
			if req.ExcludeEvidence != "" {
				skip[req.ExcludeEvidence] = true
			}
			for _, h := range copies {
				skip[h.EvidenceID] = true
			}
			found, report, rerr := imageRegionSearch(ctx, deps, scope, req.Upload, skip)
			if rerr != nil {
				limitations = append(limitations, "The check for the attached picture inside other pictures could not run, so only whole-picture copies, look-alikes and similar parts are listed.")
			} else {
				regionHits = found
				response["region"] = report
				limitations = append(limitations,
					"A picture is reported as containing the attached picture only when many small local features of the two agree on one placement; the box shows where. This finds the same picture or the same fixed object (a sign, a logo, a vehicle, a building) in other photographs; a different object of the same kind is what the similar-part and look-alike lists are for.")
			}
		}
		parts := dropAlreadyListed(withoutPicture(similarRegion, req.ExcludeEvidence), append(append([]imageSearchHit{}, copies...), regionHits...))
		if len(parts) > imageSimilarRegionMax {
			parts = parts[:imageSimilarRegionMax]
		}
		if len(parts) > 0 {
			limitations = append(limitations, "A \"part looks like it\" match compares a window of the other picture with the attached picture by appearance only; it is a candidate to look at, not a verified match.")
		}
		merged := append(append(append(append([]imageSearchHit{}, copies...), regionHits...), parts...), rest...) // never nil: an empty answer is a list, not null
		response["results"], response["result_count"], response["limitations"] = merged, len(merged), limitations
	}
	response["elapsed_ms"] = int(time.Since(started).Milliseconds())
	return response, nil
}

func nullableUUID(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

// imageSearchActor names who searched for the log: the authenticated actor, else the subject, else a fixed word when authentication is off.
func imageSearchActor(scope forensicScope) string {
	for _, v := range []string{scope.ActorID, scope.SubjectID} {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return "unauthenticated"
}

// ---------------------------------------------------------------------------------------------------------------------------------------------------------------- handlers

func imageCaseSearchHandler(cfg config, db *pgxpool.Pool, store contentAddressedStore) http.HandlerFunc {
	settings := imageSearchSettingsFromEnv()
	deps := imageSearchDeps{db: db, settings: settings, embed: httpImageEmbedClient{settings: settings}, translator: localAITranslator{cfg: cfg}, store: store, crop: httpImageEmbedClient{settings: settings}}
	if imageRegionEnabled() { // FORENSIC_IMAGE_REGION_SEARCH
		deps.region = httpImageEmbedClient{settings: settings}
	}
	return imageCaseSearchHandlerWith(deps)
}

func imageCaseSearchHandlerWith(deps imageSearchDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		req := imageSearchRequest{Kind: strings.TrimSpace(query.Get("kind")), EvidenceID: strings.TrimSpace(query.Get("evidence_id")), Text: query.Get("q"), QueryLang: strings.TrimSpace(query.Get("lang")), TopK: imageSearchDefaultTopK}
		switch r.Method {
		case http.MethodGet:
		case http.MethodPost:
			if req.Kind == "" {
				req.Kind = "reference_upload"
			}
			if req.Kind != "copies_of_upload" && req.Kind != "reference_upload" {
				writeError(w, http.StatusBadRequest, errors.New("only copies_of_upload and reference_upload accept a POST"))
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, imageSearchMaxUpload+1<<20)
			if err := r.ParseMultipartForm(8 << 20); err != nil {
				writeError(w, http.StatusBadRequest, errors.New("send the image as a multipart field named image (at most 32 MB)"))
				return
			}
			file, header, err := r.FormFile("image")
			if err != nil {
				writeError(w, http.StatusBadRequest, errors.New("the multipart field image is required"))
				return
			}
			defer func() { _ = file.Close() }()
			data, err := io.ReadAll(io.LimitReader(file, imageSearchMaxUpload+1))
			if err != nil || len(data) == 0 || len(data) > imageSearchMaxUpload {
				writeError(w, http.StatusBadRequest, errors.New("the image must be between 1 byte and 32 MB"))
				return
			}
			req.Upload = data
			req.UploadName = strings.TrimSpace(filepath.Base(header.Filename))
		default:
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if raw := strings.TrimSpace(query.Get("top_k")); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > imageSearchMaxTopK {
				writeError(w, http.StatusBadRequest, fmt.Errorf("top_k must be between 1 and %d", imageSearchMaxTopK))
				return
			}
			req.TopK = value
		}
		scope, err := bindForensicScope(r, query.Get("tenant_id"), query.Get("collection_id"), query.Get("case_id"), "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		if scope.CollectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}
		if deps.db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		req.Scope = scope
		if req.Kind == "reference_region" { // a box drawn on a stored picture: search for that part, leaving the picture itself out
			box, err := parseImageBox(query.Get("box"))
			if err == nil {
				if _, uuidErr := uuid.Parse(req.EvidenceID); uuidErr != nil {
					err = errors.New("evidence_id must be a UUID")
				}
			}
			if err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}
			part, err := cropStoredPicture(r.Context(), deps, scope, req.EvidenceID, box)
			if err != nil {
				var searchErr imageSearchError
				if errors.As(err, &searchErr) {
					writeError(w, searchErr.Status, errors.New(searchErr.Message))
					return
				}
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			req.Kind, req.Upload, req.UploadName, req.Subject, req.ExcludeEvidence, req.EvidenceID = "reference_upload", part, "the selected part", "the selected part", req.EvidenceID, ""
		}
		result, err := runImageCaseSearch(r.Context(), deps, req)
		if err != nil {
			var searchErr imageSearchError
			if errors.As(err, &searchErr) {
				writeError(w, searchErr.Status, errors.New(searchErr.Message))
				return
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		if query.Get("format") == "ask" { // the same answer the Ask question would give (ask_image_search.go), so the workspace shows it as any other answer
			writeJSON(w, http.StatusOK, imageSearchAsAskAnswer(req, result))
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

// imageSearchAsAskAnswer turns the result of the endpoint into the answer shape of the Ask engine, for the workspace's "attach an image" and for any caller that wants a ready answer.
func imageSearchAsAskAnswer(req imageSearchRequest, result map[string]any) hybridQueryResponse {
	started := time.Now()
	scope := req.Scope
	question := hybridQueryRequest{TenantID: scope.TenantID, UserID: scope.ActorID, CollectionID: scope.CollectionID, RequestClass: "governed_analysis"}
	plan, subject := imageAskPlan{Kind: imageAskKindPhotos, Description: req.Text}, strings.TrimSpace(req.Text)
	if translated, ok := result["translated_query"].(string); ok && translated != "" {
		plan.Description, subject = translated, translated
	}
	switch req.Kind {
	case "copies_of_upload", "reference_upload":
		name := firstNonEmpty(req.UploadName, "the attached image")
		plan, subject = imageAskPlan{Kind: imageAskKindCopies, ImageName: name}, firstNonEmpty(req.Subject, "your picture")
		question.Query = "Find copies of " + name
	case "copies", "reference", "more_like_this":
		kind := imageAskKindCopies
		if req.Kind == "more_like_this" {
			kind = imageAskKindSimilar
		}
		plan, subject = imageAskPlan{Kind: kind}, "the chosen image"
		question.Query = "Find " + map[string]string{imageAskKindCopies: "copies of", imageAskKindSimilar: "images like"}[kind] + " the chosen image"
	case "duplicate_groups":
		plan, subject = imageAskPlan{Kind: imageAskKindDuplicates}, "this case"
		question.Query = "Show the duplicate pictures of this case"
	default:
		question.Query = "Photos of " + subject
	}
	return imageAskResponse(question, started, plan, subject, result)
}

// appendSimilar adds the ranked similar images below the copies: an image already shown as a copy is not listed again, and at most topK similar images follow, numbered after the copies.
func appendSimilar(copies, similar []imageSearchHit, topK int, minSimilarity float64) []imageSearchHit {
	seen := make(map[string]bool, len(copies))
	for _, h := range copies {
		seen[h.EvidenceID] = true
	}
	next, added := len(copies)+1, 0
	for _, h := range similar {
		if seen[h.EvidenceID] || h.CosineDistance == nil || added >= topK || 1-*h.CosineDistance < minSimilarity {
			continue
		}
		seen[h.EvidenceID] = true
		sim := 1 - *h.CosineDistance
		h.Similarity, h.Tier, h.TierLabel, h.Rank = &sim, "similar", imageTierLabels["similar"], next
		next++
		added++
		copies = append(copies, h)
	}
	return copies
}
