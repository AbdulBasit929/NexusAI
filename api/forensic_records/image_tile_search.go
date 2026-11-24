// Window ("tile") search: the vectors of 34 windows of every picture (halves, thirds and quarters, slid over it; ingestion/forensic_records/image_search.py, TILE_SETS) let a search find a DETAIL of a
// photograph: a small object, a region, something in a corner that the vector of the whole picture does not say. Two uses:
//   - words: a phrase is compared with the SigLIP 2 vector of the whole picture AND of every window; a picture scores as its best window, and the window's position is returned so the interface can show where;
//   - an attached picture: the DINOv2 vectors of the windows say which other pictures have a PART that looks like it ("a part of it looks like this picture", not verified; the local-feature check in
//     image_region_search.go verifies the same picture or the same fixed object).
//
// The window vectors are stored under the model name with a "#tile" suffix, so no query of the whole-picture vectors can return a window.
package main

import (
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	imageTileModelSuffix    = "#tile"
	imageTileLimit          = 5000
	imageTierSimilarRegion  = "similar_region"
	imageSimilarRegionLabel = "a part of it looks like this picture (not verified)"
	imageSimilarRegionMax   = 8
)

func init() {
	imageTierLabels[imageTierSimilarRegion] = imageSimilarRegionLabel
	imageTierOrder[imageTierSimilarRegion] = 3
}

// imageTileSets mirrors TILE_SETS in image_search.py: the window size and the positions of its top-left corner, as fractions of the picture.
var imageTileSets = map[byte]struct {
	Size      float64
	Positions []float64
}{
	'a': {0.5, []float64{0, 0.25, 0.5}},
	'b': {1.0 / 3, []float64{0, 1.0 / 3, 2.0 / 3}},
	'c': {0.25, []float64{0, 0.25, 0.5, 0.75}},
}

// imageTileBox turns a chunk_ref such as "v:<version>:tile:b12" into the window's box (x0, y0, x1, y1 as fractions of the picture); nil when it is not a window.
func imageTileBox(chunkRef string) []float64 {
	index := strings.LastIndex(chunkRef, ":tile:")
	if index < 0 {
		return nil
	}
	id := chunkRef[index+len(":tile:"):]
	if len(id) != 3 {
		return nil
	}
	set, ok := imageTileSets[id[0]]
	if !ok {
		return nil
	}
	row, errRow := strconv.Atoi(id[1:2])
	col, errCol := strconv.Atoi(id[2:3])
	if errRow != nil || errCol != nil || row >= len(set.Positions) || col >= len(set.Positions) {
		return nil
	}
	x, y := set.Positions[col], set.Positions[row]
	round := func(v float64) float64 { return float64(int(v*10000+0.5)) / 10000 }
	return []float64{round(x), round(y), round(x + set.Size), round(y + set.Size)}
}

// the nearest window of each picture: the function returns the nearest rows first, so DISTINCT ON picks each picture's best window among them.
const imageSearchTileSQL = `
SELECT DISTINCT ON (evidence_id) evidence_id::text, version_id::text, original_filename, content_type, case_id, distance, citation_ref, chunk_ref
FROM forensic.image_search_vector($1, $2, $3::vector, $4, NULLIF($5, ''), $6::uuid, $7)
ORDER BY evidence_id, distance`

func scanTileHits(rows pgx.Rows, tier string) ([]imageSearchHit, error) {
	defer rows.Close()
	var hits []imageSearchHit
	for rows.Next() {
		var h imageSearchHit
		var distance float64
		var chunkRef string
		if err := rows.Scan(&h.EvidenceID, &h.VersionID, &h.OriginalFilename, &h.ContentType, &h.CaseID, &distance, &h.CitationRef, &chunkRef); err != nil {
			return nil, err
		}
		h.CosineDistance = &distance
		h.Box = imageTileBox(chunkRef)
		h.Tier = tier
		h.ContentPath = "/evidence/" + h.EvidenceID + "/content"
		hits = append(hits, h)
	}
	sort.SliceStable(hits, func(i, j int) bool { return *hits[i].CosineDistance < *hits[j].CosineDistance })
	return hits, rows.Err()
}

// mergeWordHits scores each picture by the better of its whole-picture similarity and its best window's similarity MINUS a penalty, keeps the window's box when the window won, and returns the best topK.
// The penalty exists because the best of 34 noisy window scores is biased upward: without it, unrelated pictures climb toward the top (measured 2026-11-23: precision 0.97 -> 0.69 at the same cut-off).
// Both similarities are kept on the hit (whole_similarity, window_similarity) so that the penalty can be calibrated offline.
func mergeWordHits(whole, windows []imageSearchHit, topK int, penalty float64) []imageSearchHit {
	best := map[string]*imageSearchHit{}
	similarity := func(h imageSearchHit) float64 { return 1 - *h.CosineDistance }
	for i := range whole {
		h := whole[i]
		h.Box = nil
		s := similarity(h)
		h.WholeSimilarity = &s
		best[h.EvidenceID] = &h
	}
	for i := range windows {
		w := windows[i]
		ws := similarity(w)
		effective := ws - penalty
		current, ok := best[w.EvidenceID]
		if !ok {
			w.WindowSimilarity = &ws
			d := 1 - effective
			w.CosineDistance = &d
			best[w.EvidenceID] = &w
			continue
		}
		current.WindowSimilarity = &ws
		if effective > similarity(*current) {
			current.Box = w.Box
			d := 1 - effective
			current.CosineDistance = &d
		}
	}
	out := make([]imageSearchHit, 0, len(best))
	for _, h := range best {
		out = append(out, *h)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if similarity(out[i]) != similarity(out[j]) {
			return similarity(out[i]) > similarity(out[j])
		}
		return out[i].EvidenceID < out[j].EvidenceID
	})
	if topK > 0 && len(out) > topK {
		out = out[:topK]
	}
	return out
}

const (
	imageTierPossible     = "possible"
	imageTierInPart       = "in_part"
	imageWordsPossibleMax = 12
	imageWordsInPartMax   = 6
)

func init() {
	imageTierLabels[imageTierPossible] = "also possible, a weaker match"
	imageTierLabels[imageTierInPart] = "in a part of the picture (not verified)"
	imageTierOrder[imageTierInPart] = 3
	imageTierOrder[imageTierPossible] = 3
}

// tierWordHits cuts a words search by what each picture scores against the best one: "best matches" at bestRatio of the best similarity, then "also possible" down to possibleRatio; weaker pictures
// are not listed. The ratios were chosen on 16 queries with the captions of the test pictures as ground truth (docs/roadmap/image-search-real-world-plan-20261123.md). The input is sorted best first.
func tierWordHits(hits []imageSearchHit, bestRatio, possibleRatio float64, bestCap int) []imageSearchHit {
	if len(hits) == 0 || hits[0].CosineDistance == nil {
		return hits
	}
	top := 1 - *hits[0].CosineDistance
	strong, possible := []imageSearchHit{}, []imageSearchHit{}
	for _, h := range hits {
		sim := 1 - *h.CosineDistance
		switch {
		case sim >= bestRatio*top:
			h.Tier = "ranked"
			strong = append(strong, h)
		case sim >= possibleRatio*top:
			h.Tier = imageTierPossible
			possible = append(possible, h)
		}
	}
	if bestCap > 0 && len(strong) > bestCap {
		strong = strong[:bestCap]
	}
	if len(possible) > imageWordsPossibleMax {
		possible = possible[:imageWordsPossibleMax]
	}
	return append(strong, possible...)
}

// windowOnlyHits lists pictures that are NOT in the ranked tiers but have a window that matches the phrase well: "in a part of the picture", with the window's box. The main ranking uses the whole
// picture only (the window penalty makes windows effectively never win there), because with caption ground truth the best score of many noisy windows lowers precision (0.97 to 0.69); a window that
// finds something small is shown apart, as a candidate, never mixed into the ranking. windows are sorted best first; ratio is a fraction of the best window similarity of the whole case.
func windowOnlyHits(windows []imageSearchHit, listed map[string]bool, ratio float64) []imageSearchHit {
	if len(windows) == 0 || windows[0].CosineDistance == nil {
		return nil
	}
	top := 1 - *windows[0].CosineDistance
	var out []imageSearchHit
	for _, w := range windows {
		if len(out) >= imageWordsInPartMax {
			break
		}
		if listed[w.EvidenceID] || len(w.Box) != 4 || 1-*w.CosineDistance < ratio*top {
			continue
		}
		w.Tier = imageTierInPart
		out = append(out, w)
	}
	return out
}

// dropAlreadyListed removes from similar-region hits the pictures that are already listed as copies or as containing the region.
func dropAlreadyListed(similarRegion []imageSearchHit, listed []imageSearchHit) []imageSearchHit {
	seen := map[string]bool{}
	for _, h := range listed {
		seen[h.EvidenceID] = true
	}
	kept := similarRegion[:0:0]
	for _, h := range similarRegion {
		if !seen[h.EvidenceID] {
			kept = append(kept, h)
		}
	}
	return kept
}
