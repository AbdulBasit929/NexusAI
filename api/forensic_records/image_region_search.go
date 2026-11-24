// Region search: does a small picture (a crop, a screenshot of a detail, a logo, a sign) appear INSIDE the other pictures of the case, and where?
//
// The picture the analyst attaches is matched against the case's images with local features and a geometric check (ingestion/forensic_records/image_region.py: SIFT matches that all agree on one
// transform; Philbin et al., CVPR 2007). A picture that really contains the region gives many agreeing matches and the fitted transform says where the region sits; the answer carries that box.
//
// The service remembers the features of pictures it has seen (memory only, bounded), so the first search over a case sends every picture once and later searches send only the attached picture.
package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/httpclient"
)

const (
	imageRegionVerified      = "region_verified"
	imageRegionProbable      = "region_probable"
	imageRegionBatchKeys     = 200
	imageRegionBatchBytes    = 6 << 20
	imageRegionReadWorkers   = 4
	imageRegionMaxFileBytes  = 32 << 20
	imageRegionDefaultMax    = 400
	imageRegionServiceBudget = 150 * time.Second
)

func init() {
	imageTierLabels[imageRegionVerified] = "contains this picture or region (verified)"
	imageTierLabels[imageRegionProbable] = "probably contains this region"
	imageTierOrder[imageRegionVerified] = 2
	imageTierOrder[imageRegionProbable] = 2
}

func imageRegionEnabled() bool {
	return envBoolValue(os.Getenv("FORENSIC_IMAGE_REGION_SEARCH"), false) //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

type imageRegionSettings struct {
	VerifiedMin   int
	ProbableMin   int
	MaxCandidates int
}

func imageRegionSettingsFromEnv() imageRegionSettings {
	atoi := func(key string, fallback int) int {
		if value, err := strconv.Atoi(strings.TrimSpace(env(key, ""))); err == nil && value > 0 {
			return value
		}
		return fallback
	}
	return imageRegionSettings{
		VerifiedMin:   atoi("FORENSIC_IMAGE_REGION_VERIFIED_MIN", 12),
		ProbableMin:   atoi("FORENSIC_IMAGE_REGION_PROBABLE_MIN", 8),
		MaxCandidates: atoi("FORENSIC_IMAGE_REGION_MAX_CANDIDATES", imageRegionDefaultMax),
	}
}

type regionMatchItem struct {
	Key string `json:"key"`
	B64 string `json:"b64,omitempty"`
}

type regionMatchResult struct {
	Key      string      `json:"key"`
	Inliers  int         `json:"inliers"`
	Matches  int         `json:"matches"`
	Mirrored bool        `json:"mirrored"`
	Box      []float64   `json:"box"`
	Polygon  [][]float64 `json:"polygon"`
	Error    string      `json:"error"`
}

type regionMatchResponse struct {
	Results []regionMatchResult `json:"results"`
	Need    []string            `json:"need"`
}

// imageRegionClient is the service side: the probe against some pictures, given by content hash (and their bytes when the service has not seen them).
type imageRegionClient interface {
	RegionMatch(ctx context.Context, probe []byte, items []regionMatchItem) (regionMatchResponse, error)
}

func (c httpImageEmbedClient) RegionMatch(ctx context.Context, probe []byte, items []regionMatchItem) (regionMatchResponse, error) {
	if c.settings.ServiceURL == "" || c.settings.ServiceKey == "" {
		return regionMatchResponse{}, errImageSearchServiceDown
	}
	payload, err := json.Marshal(map[string]any{"probe_b64": base64.StdEncoding.EncodeToString(probe), "items": items})
	if err != nil {
		return regionMatchResponse{}, err
	}
	callCtx, cancel := context.WithTimeout(ctx, imageRegionServiceBudget)
	defer cancel()
	request, err := newImageServiceRequest(callCtx, c.settings, "/region-match", "application/json", payload)
	if err != nil {
		return regionMatchResponse{}, err
	}
	response, err := httpclient.NewWithTimeout(imageRegionServiceBudget).Do(request)
	if err != nil {
		return regionMatchResponse{}, fmt.Errorf("%w: %v", errImageSearchServiceDown, err)
	}
	defer func() { _ = response.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return regionMatchResponse{}, err
	}
	if response.StatusCode != http.StatusOK {
		return regionMatchResponse{}, fmt.Errorf("the image search service answered HTTP %d", response.StatusCode)
	}
	var out regionMatchResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return regionMatchResponse{}, err
	}
	return out, nil
}

type regionCandidate struct {
	EvidenceID string
	Name       string
	SHA256     string
	StorageURI string
	Size       int64
}

type imageRegionReport struct {
	ImagesInCase  int    `json:"images_in_case"`
	ImagesChecked int    `json:"images_checked"`
	Unreadable    int    `json:"images_unreadable"`
	Skipped       int    `json:"images_skipped_already_matched"`
	NotChecked    int    `json:"images_not_checked"`
	ElapsedMS     int64  `json:"elapsed_ms"`
	Statement     string `json:"statement"`
}

const imageRegionCandidatesSQL = `
SELECT e.evidence_id::text, e.original_filename, e.sha256, e.raw_storage_ref, e.size_bytes
FROM forensic.evidence_items e
WHERE e.tenant_id = current_setting('app.tenant_id', true) AND e.collection_id = $1 AND e.modality = 'image' AND e.processing_status <> 'duplicate'
  AND ($2 = '' OR e.case_id::text = $2)
ORDER BY e.evidence_id
LIMIT $3`

func listRegionCandidates(ctx context.Context, db *pgxpool.Pool, scope forensicScope, limit int) ([]regionCandidate, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", scope.TenantID); err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, imageRegionCandidatesSQL, scope.CollectionID, scope.CaseID, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []regionCandidate
	for rows.Next() {
		var c regionCandidate
		if err := rows.Scan(&c.EvidenceID, &c.Name, &c.SHA256, &c.StorageURI, &c.Size); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// runRegionPasses asks the service about every candidate: first by content hash alone (the service answers from what it remembers and names the pictures it still needs), then with the bytes of those.
func runRegionPasses(ctx context.Context, client imageRegionClient, probe []byte, cands []regionCandidate, read func(regionCandidate) ([]byte, error)) (map[string]regionMatchResult, int, error) {
	results := map[string]regionMatchResult{}
	byKey := map[string]regionCandidate{}
	for _, c := range cands {
		byKey[c.SHA256] = c
	}
	collect := func(resp regionMatchResponse) {
		for _, r := range resp.Results {
			results[r.Key] = r
		}
	}
	var need []string
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for start := 0; start < len(keys); start += imageRegionBatchKeys {
		end := start + imageRegionBatchKeys
		if end > len(keys) {
			end = len(keys)
		}
		items := make([]regionMatchItem, 0, end-start)
		for _, key := range keys[start:end] {
			items = append(items, regionMatchItem{Key: key})
		}
		resp, err := client.RegionMatch(ctx, probe, items)
		if err != nil {
			return nil, 0, err
		}
		collect(resp)
		need = append(need, resp.Need...)
	}
	unreadable := 0
	// the pictures the service has not seen: read them (a few at a time) and send them in batches bounded by size
	type loaded struct {
		key  string
		data []byte
		err  error
	}
	jobs := make(chan string)
	out := make(chan loaded)
	var wg sync.WaitGroup
	for w := 0; w < imageRegionReadWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for key := range jobs {
				data, err := read(byKey[key])
				out <- loaded{key, data, err}
			}
		}()
	}
	go func() {
		for _, key := range need {
			select {
			case jobs <- key:
			case <-ctx.Done():
				close(jobs)
				wg.Wait()
				close(out)
				return
			}
		}
		close(jobs)
		wg.Wait()
		close(out)
	}()
	var batch []regionMatchItem
	size := 0
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		resp, err := client.RegionMatch(ctx, probe, batch)
		if err != nil {
			return err
		}
		collect(resp)
		batch, size = nil, 0
		return nil
	}
	for l := range out {
		if l.err != nil || len(l.data) == 0 {
			unreadable++
			continue
		}
		if size+len(l.data) > imageRegionBatchBytes {
			if err := flush(); err != nil {
				return nil, 0, err
			}
		}
		batch = append(batch, regionMatchItem{Key: l.key, B64: base64.StdEncoding.EncodeToString(l.data)})
		size += len(l.data)
	}
	if err := flush(); err != nil {
		return nil, 0, err
	}
	for _, r := range results {
		if r.Error != "" {
			unreadable++
		}
	}
	return results, unreadable, nil
}

// imageRegionHits turns the service's matches into hits: pictures that contain the attached picture or region, strongest first, each with the box where it sits.
func imageRegionHits(cands []regionCandidate, results map[string]regionMatchResult, settings imageRegionSettings) []imageSearchHit {
	var hits []imageSearchHit
	for _, c := range cands {
		r, ok := results[c.SHA256]
		if !ok || r.Error != "" || r.Inliers < settings.ProbableMin {
			continue
		}
		tier := imageRegionProbable
		if r.Inliers >= settings.VerifiedMin {
			tier = imageRegionVerified
		}
		inliers, matches := r.Inliers, r.Matches
		hits = append(hits, imageSearchHit{
			EvidenceID: c.EvidenceID, OriginalFilename: c.Name, Tier: tier, TierLabel: imageTierLabels[tier], ContentPath: "/evidence/" + c.EvidenceID + "/content",
			CitationRef: "nexusai://evidence/" + c.EvidenceID, Inliers: &inliers, Matches: &matches, Box: r.Box, Mirrored: r.Mirrored,
		})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if *hits[i].Inliers != *hits[j].Inliers {
			return *hits[i].Inliers > *hits[j].Inliers
		}
		return hits[i].EvidenceID < hits[j].EvidenceID
	})
	for i := range hits {
		hits[i].Rank = i + 1
	}
	return hits
}

func imageRegionStatement(report imageRegionReport) string {
	text := fmt.Sprintf("Checked %d of %d images for the attached picture inside them.", report.ImagesChecked, report.ImagesInCase)
	if report.NotChecked > 0 {
		text += fmt.Sprintf(" %d were not checked (the case holds more than the limit for one search).", report.NotChecked)
	}
	if report.Unreadable > 0 {
		text += fmt.Sprintf(" %d could not be read.", report.Unreadable)
	}
	return text
}

// imageRegionSearch runs the region pass over the case. Pictures already found as copies are left out (they are the whole picture, not a region of it).
func imageRegionSearch(ctx context.Context, deps imageSearchDeps, scope forensicScope, probe []byte, skip map[string]bool) ([]imageSearchHit, imageRegionReport, error) {
	started := time.Now()
	report := imageRegionReport{}
	if deps.region == nil || deps.db == nil || deps.store.root == "" {
		return nil, report, errors.New("region search is not available here")
	}
	settings := imageRegionSettingsFromEnv()
	all, err := listRegionCandidates(ctx, deps.db, scope, settings.MaxCandidates)
	if err != nil {
		return nil, report, err
	}
	report.ImagesInCase = len(all)
	if len(all) > settings.MaxCandidates {
		report.NotChecked = len(all) - settings.MaxCandidates
		all = all[:settings.MaxCandidates]
	}
	var cands []regionCandidate
	for _, c := range all {
		if skip[c.EvidenceID] {
			report.Skipped++
			continue
		}
		cands = append(cands, c)
	}
	read := func(c regionCandidate) ([]byte, error) {
		if c.Size <= 0 || c.Size > imageRegionMaxFileBytes {
			return nil, errors.New("not read")
		}
		verified, err := deps.store.Verify(c.StorageURI, c.Size)
		if err != nil {
			return nil, err
		}
		return os.ReadFile(verified.Path)
	}
	results, unreadable, err := runRegionPasses(ctx, deps.region, probe, cands, read)
	if err != nil {
		return nil, report, err
	}
	report.Unreadable = unreadable
	report.ImagesChecked = len(all) - unreadable
	report.ElapsedMS = time.Since(started).Milliseconds()
	report.Statement = imageRegionStatement(report)
	return imageRegionHits(cands, results, settings), report, nil
}

// copyTier says whether a tier is a whole-picture copy.
func copyTier(tier string) bool {
	return tier == "identical_file" || tier == "same_image" || tier == "same_photograph"
}

func newImageServiceRequest(ctx context.Context, settings imageSearchSettings, path, contentType string, body []byte) (*http.Request, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, settings.ServiceURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", contentType)
	request.Header.Set("X-Image-Search-Key", settings.ServiceKey)
	return request, nil
}
