package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
)

// PLATE SEARCH OVER IMAGE AND VIDEO READS: exact matches and similar readings, reported separately.
//
// Measured 2026-11-10 on the Orion reference case (docs/data/plates-m5-preregistration-20261110.md, Amendment 8): "Which images show the plate SYNT9150?" was answered by the SQL engine with
// "no sighting of the plate was found" although one added image reads exactly that plate and the plate has 14 camera sightings: the engine only knows the camera-sightings table, and the older
// search-only path (plate_read_search.go) was never reached because the engine answers first. Two further facts decide the design:
//
//   - A plate reader trained on typical formats reads an unusual plate as the nearest typical one (a letter O where a digit would be is read as 0; a T is read as I). The plate typed by the
//     analyst then matches nothing exactly although the image shows it. An exact-only search reports a false absence.
//   - Folding similar readings into the same count would be worse: a different plate would be reported as "seen". So the two are separated: an exact match is a match; a similar reading (up to two
//     characters different, counting a commonly confused pair as one) is listed apart, labelled, with the instruction to check it by eye. The formats of a plate are never used to reject a reading.
//
// Policy kept (product-owner decision 2026-09-28, plate_read_search.go): plate text read from images is search-only. The answer names the plate the analyst typed, the files and the counts; it never
// prints a stored reading, and an evidence entry carries no plate text.
const (
	plateImageSearchLimit    = 20000 // reads inspected per question; more is reported as an incomplete search
	plateSimilarMaxCost      = 2
	plateSimilarMinLength    = 5
	plateImageSearchGrounded = "PLATE_IMAGE_READS"
)

// plateConfusablePairs are characters a plate reader commonly mixes up. A substitution inside a pair costs 1, any other substitution, insertion or deletion costs 2.
var plateConfusablePairs = func() map[[2]byte]bool {
	pairs := map[[2]byte]bool{}
	for _, p := range []string{"0O", "0D", "0Q", "OQ", "OD", "1I", "1L", "IL", "IT", "8B", "5S", "2Z", "6G", "MW", "MN", "UV", "HN", "EF", "PR", "CG", "AR"} {
		pairs[[2]byte{p[0], p[1]}], pairs[[2]byte{p[1], p[0]}] = true, true
	}
	return pairs
}()

// plateConfusionCost is a weighted edit distance between two normalised plate keys.
func plateConfusionCost(a, b string) int {
	la, lb := len(a), len(b)
	prev := make([]int, lb+1)
	for j := range prev {
		prev[j] = 2 * j
	}
	for i := 1; i <= la; i++ {
		cur := make([]int, lb+1)
		cur[0] = 2 * i
		for j := 1; j <= lb; j++ {
			sub := 0
			if a[i-1] != b[j-1] {
				sub = 2
				if plateConfusablePairs[[2]byte{a[i-1], b[j-1]}] {
					sub = 1
				}
			}
			cur[j] = min(prev[j]+2, cur[j-1]+2, prev[j-1]+sub)
		}
		prev = cur
	}
	return prev[lb]
}

// plateReadRow is one stored plate reading of an image or a video frame. Key is the normalised reading and is never put in a response.
type plateReadRow struct {
	EvidenceID, File, Key, Observation string
	VersionID                          string         // the evidence version the reading was made from (empty when the artifact row is missing)
	Locator                            map[string]any // where in the image the plate was found (bounding box); holds no plate text
}

type plateMatchedFile struct {
	EvidenceID, File, Observation string
	Cost                          int
	VersionID                     string
	Locator                       map[string]any
}

type plateSearchResult struct {
	Typed   string
	Exact   []plateMatchedFile
	Similar []plateMatchedFile
}

// searchPlateReads classifies the stored readings against one typed plate. Exact: the same normalised key. Similar: a different key within plateSimilarMaxCost.
func searchPlateReads(typed string, rows []plateReadRow) plateSearchResult {
	result := plateSearchResult{Typed: typed}
	seen := map[string]bool{}
	for _, row := range rows {
		if row.Key == "" || seen[row.Observation+"|"+row.EvidenceID] {
			continue
		}
		seen[row.Observation+"|"+row.EvidenceID] = true
		match := plateMatchedFile{EvidenceID: row.EvidenceID, File: row.File, Observation: row.Observation, VersionID: row.VersionID, Locator: row.Locator}
		if row.Key == typed {
			result.Exact = append(result.Exact, match)
			continue
		}
		if len(typed) < plateSimilarMinLength || len(row.Key) < plateSimilarMinLength {
			continue
		}
		if cost := plateConfusionCost(typed, row.Key); cost <= plateSimilarMaxCost {
			match.Cost = cost
			result.Similar = append(result.Similar, match)
		}
	}
	byFile := func(list []plateMatchedFile) {
		sort.SliceStable(list, func(i, j int) bool {
			if list[i].Cost != list[j].Cost {
				return list[i].Cost < list[j].Cost
			}
			return list[i].File < list[j].File
		})
	}
	byFile(result.Exact)
	byFile(result.Similar)
	return result
}

func distinctPlateFiles(list []plateMatchedFile) []string {
	seen, out := map[string]bool{}, []string{}
	for _, m := range list {
		if m.File != "" && !seen[m.File] {
			seen[m.File] = true
			out = append(out, m.File)
		}
	}
	return out
}

func plateFileList(files []string) string {
	if len(files) > 5 {
		return strings.Join(files[:5], ", ") + fmt.Sprintf(" and %d more", len(files)-5)
	}
	return strings.Join(files, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// plateSearchSentence is the analyst-facing text for one typed plate. It contains the typed plate, counts and file names, and no stored reading.
func plateSearchSentence(r plateSearchResult) string {
	exact, similar := distinctPlateFiles(r.Exact), distinctPlateFiles(r.Similar)
	var b strings.Builder
	switch {
	case len(exact) > 0:
		fmt.Fprintf(&b, "The plate %s was read in %d image or video %s: %s.", r.Typed, len(exact), plural(len(exact), "file", "files"), plateFileList(exact))
	default:
		fmt.Fprintf(&b, "No image or video reading matches the plate %s exactly.", r.Typed)
	}
	if len(similar) > 0 {
		fmt.Fprintf(&b, " %d other %s a similar reading (one or two characters different, often characters a reader confuses, such as O and 0 or I and T): %s. This may be the same plate or a different one: check %s by eye.",
			len(similar), plural(len(similar), "file shows", "files show"), plateFileList(similar), plural(len(similar), "it", "them"))
	}
	if len(exact) == 0 && len(similar) == 0 {
		b.WriteString(" The plate reader may have misread it, so this does not show the plate is absent from the images or video.")
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// plateReadRows reads the plate readings of the case that came from media (images, video frames), read-only, under the tenant's row-level security.
func plateReadRows(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID string) ([]plateReadRow, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(ctx, "SELECT set_config('app.tenant_id',$1,true)", tenantID); err != nil {
		return nil, false, err
	}
	rows, err := tx.Query(ctx, `
		SELECT coalesce(r.source_file, ''), coalesce(r.evidence_id::text, ''), coalesce(r.primary_target, ''), coalesce(r.metadata->>'derived_observation_id', r.row_hash),
		       coalesce(a.version_id::text, ''), coalesce(a.citation_locator::text, '')
		FROM forensic.records r
		LEFT JOIN forensic.derived_artifacts a
		  ON a.tenant_id = r.tenant_id AND a.collection_id = r.collection_id AND a.evidence_id = r.evidence_id
		 AND a.artifact_id::text = r.metadata->>'derived_observation_id' AND a.processing_status = 'completed'
		WHERE r.tenant_id = $1 AND r.collection_id = $2 AND r.record_type = 'anpr' AND r.metadata->>'derived_from_media' = 'true'
		ORDER BY r.source_file, r.row_number
		LIMIT $3`, tenantID, collectionID, plateImageSearchLimit+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	out := []plateReadRow{}
	for rows.Next() {
		var row plateReadRow
		var locator string
		if err := rows.Scan(&row.File, &row.EvidenceID, &row.Key, &row.Observation, &row.VersionID, &locator); err != nil {
			return nil, false, err
		}
		if locator != "" {
			_ = json.Unmarshal([]byte(locator), &row.Locator) // a locator that does not parse leaves the source listed but not openable
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	complete := len(out) <= plateImageSearchLimit
	if !complete {
		out = out[:plateImageSearchLimit]
	}
	return out, complete, nil
}

// plateImageSearchRun answers a question that supplies a plate and asks about images or video. `typed` is the plate(s) the model extracted from the question (the intent step, ask_text_intent.go);
// when it is empty the keyword fallback decides and extracts. It returns (response, true) when it answered; on any failure it returns false and the question takes the path it would have taken
// without it.
func plateImageSearchRun(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, startedAt time.Time, typed []string) (hybridQueryResponse, bool) {
	if db == nil || req.TenantID == "" || req.CollectionID == "" {
		return hybridQueryResponse{}, false
	}
	if len(typed) == 0 { // no model decision: the keyword fallback decides whether the question is a plate-in-images question and which plate it names
		if !plateReadSearchApplies(req.Query) {
			return hybridQueryResponse{}, false
		}
		typed = analystPlateLiterals(req.Query)
	}
	rows, complete, err := plateReadRows(ctx, db, req.TenantID, req.CollectionID)
	if err != nil {
		return hybridQueryResponse{}, false
	}
	sentences, results, provenance := []string{}, []map[string]any{}, []map[string]any{}
	exactTotal, similarTotal := 0, 0
	for _, key := range typed {
		r := searchPlateReads(key, rows)
		sentences = append(sentences, plateSearchSentence(r))
		exactTotal += len(distinctPlateFiles(r.Exact))
		similarTotal += len(distinctPlateFiles(r.Similar))
		for kind, list := range map[string][]plateMatchedFile{"exact": r.Exact, "similar": r.Similar} {
			for _, m := range list {
				results = append(results, map[string]any{
					"id": m.Observation, "preview": "", "content": "", "score": map[string]float64{"exact": 1, "similar": 0.5}[kind],
					"metadata": map[string]any{"evidence_id": m.EvidenceID, "version_id": m.VersionID, "source_file": m.File, "source_family": "image_or_video", "match_kind": kind, "review_state": "model_candidate",
						"artifact_type": "forensics.anpr-observation/v1", "plate_distance": m.Cost},
				})
				// One openable source per matched image: the evidence, its version and where in the image the plate was found. The locator holds a bounding box, never plate text.
				if m.VersionID != "" && m.Locator != nil {
					locator := map[string]any{"artifact_id": m.Observation, "source_file": m.File}
					for _, field := range []string{"bbox", "coordinate_space", "frame_ts", "t_start"} {
						if value, ok := m.Locator[field]; ok {
							locator[field] = value
						}
					}
					provenance = append(provenance, map[string]any{
						"citation_id": "plate-" + m.Observation, "evidence_id": m.EvidenceID, "version_id": m.VersionID, "source_file": m.File,
						"artifact_type": "forensics.anpr-observation/v1", "source_truth_state": "derived_model_observation",
						"proof_role": map[string]string{"exact": "exact plate reading", "similar": "similar plate reading, check by eye"}[kind], "locator": locator,
					})
				}
			}
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		ki, kj := results[i]["metadata"].(map[string]any)["match_kind"].(string), results[j]["metadata"].(map[string]any)["match_kind"].(string)
		if ki != kj {
			return ki == "exact"
		}
		return results[i]["metadata"].(map[string]any)["source_file"].(string) < results[j]["metadata"].(map[string]any)["source_file"].(string)
	})
	text := strings.Join(sentences, " ") + " These are unreviewed model readings, not camera sightings."
	state := "COMPLETE_RESULTS"
	limitation := "Plate readings are model observations and need analyst review. Plate text read from images is search-only: this answer names only the plate you typed."
	if exactTotal == 0 && similarTotal == 0 {
		state = "NO_EXACT_MATCH"
	}
	if !complete {
		text += fmt.Sprintf(" Only the first %d plate readings of the case were searched, so the search is incomplete.", plateImageSearchLimit)
		state, limitation = "SEARCH_INCOMPLETE", limitation+" The search covered only part of the case's plate readings."
	}
	answer := map[string]any{
		"result_state": "COMPLETED", "answer": text, "grounding": plateImageSearchGrounded, "case_specific_facts": []any{},
		"plate_search":        map[string]any{"plates_typed": len(typed), "exact_files": exactTotal, "similar_files": similarTotal, "readings_searched": len(rows), "search_complete": complete},
		"search_completeness": map[bool]string{true: "EXHAUSTIVE", false: "INCOMPLETE"}[complete], "evidence_status": strings.ToLower(state),
	}
	enterprise := terminalEnterprisePayload(req, forensicrequest.GovernedAnalysis, answer)
	enterprise["summary"], enterprise["limitations"] = limitation, []string{limitation}
	enterprise["executive_answer"] = text
	enterprise["provenance"] = provenance
	return hybridQueryResponse{
		CollectionID: req.CollectionID, RequestClass: forensicrequest.GovernedAnalysis, Intent: intentRecords, Target: strings.Join(typed, ","),
		Route: []string{"plate_image_search"}, Policy: "plate_search_only",
		Planner:   map[string]any{"route": "plate_image_search", "plates_typed": len(typed), "similar_max_cost": plateSimilarMaxCost},
		QueryPlan: QueryPlan{}, Capability: queryCapabilityAssessment{Status: "operational"},
		Evidence: map[string]any{"result_state": state, "search_completeness": answer["search_completeness"], "retrieval_mode": "plate_read_exact_and_similar", "row_count": len(results), "results": results},
		Answer:   answer, GeneratedAt: time.Now().UTC(), Enterprise: enterprise,
		Telemetry: QueryTelemetry{TotalLatencyMS: time.Since(startedAt).Milliseconds()},
	}, true
}
