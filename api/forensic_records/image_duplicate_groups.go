// Duplicate and near-duplicate groups of a whole case: the pictures that are the same file, the same image saved again, or the same photograph edited are stacked, one picture stands for the stack and
// every member stays openable. This is what a reviewer wants first (Magnet Griffeye shows the count of copies on each thumbnail and where each was found): it cuts the pile without deleting or hiding
// anything. Evidence is never changed; the groups are computed from the stored descriptors with the same calibrated distances as the copy search.
package main

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
)

const (
	imageDuplicateMaxImages = 5000 // pairwise comparison is exact; above this only identical files are grouped
	imageDuplicateMaxGroups = 60
	imageTierDuplicateLead  = "duplicate_lead"
	imageTierLinkedCopy     = "linked_copy"
)

func init() {
	imageTierLabels[imageTierDuplicateLead] = "stands for the group"
	imageTierLabels[imageTierLinkedCopy] = "in the group, linked through other copies"
}

type imageDuplicateSummary struct {
	Groups        int    `json:"groups"`
	Pictures      int    `json:"pictures_in_groups"`
	CouldSetAside int    `json:"could_set_aside"`
	Considered    int    `json:"images_considered"`
	GroupsShown   int    `json:"groups_shown"`
	Limited       bool   `json:"perceptual_grouping_limited"`
	Statement     string `json:"statement"`
}

type duplicateImage struct {
	ID, Name, SHA string
	Size          int64
	ContentType   *string
	CaseID        *string
}

type duplicateEdge struct {
	A, B     string
	Tier     string
	Hash     *int
	Distance *float64
}

const imageDuplicateImagesSQL = `
SELECT e.evidence_id::text, e.original_filename, e.sha256, e.size_bytes, e.content_type, e.case_id
FROM forensic.evidence_items e
WHERE e.tenant_id = current_setting('app.tenant_id', true) AND e.collection_id = $1 AND e.modality = 'image' AND e.current_version_id IS NOT NULL
  AND ($2 = '' OR e.case_id = $2)
ORDER BY e.evidence_id
LIMIT $3`

const imageDuplicateHashSQL = `
SELECT a.evidence_id::text, b.evidence_id::text, bit_count(a.pdq256 # b.pdq256)::int
FROM forensic.image_hash a
JOIN forensic.evidence_items ea ON ea.tenant_id = a.tenant_id AND ea.collection_id = a.collection_id AND ea.evidence_id = a.evidence_id AND ea.current_version_id = a.version_id
JOIN forensic.image_hash b ON b.tenant_id = a.tenant_id AND b.collection_id = a.collection_id AND a.evidence_id < b.evidence_id
JOIN forensic.evidence_items eb ON eb.tenant_id = b.tenant_id AND eb.collection_id = b.collection_id AND eb.evidence_id = b.evidence_id AND eb.current_version_id = b.version_id
WHERE a.tenant_id = current_setting('app.tenant_id', true) AND a.collection_id = $1
  AND ($2 = '' OR (ea.case_id = $2 AND eb.case_id = $2))
  AND bit_count(a.pdq256 # b.pdq256) <= $3`

const imageDuplicateVectorSQL = `
SELECT a.evidence_id::text, b.evidence_id::text, (a.embedding <=> b.embedding)::float8
FROM forensic.content_embedding a
JOIN forensic.evidence_items ea ON ea.tenant_id = a.tenant_id AND ea.collection_id = a.collection_id AND ea.evidence_id = a.evidence_id AND a.chunk_ref = 'v:' || ea.current_version_id::text || ':full'
JOIN forensic.content_embedding b ON b.tenant_id = a.tenant_id AND b.collection_id = a.collection_id AND b.model_id = a.model_id AND b.modality = 'image' AND a.evidence_id < b.evidence_id
JOIN forensic.evidence_items eb ON eb.tenant_id = b.tenant_id AND eb.collection_id = b.collection_id AND eb.evidence_id = b.evidence_id AND b.chunk_ref = 'v:' || eb.current_version_id::text || ':full'
WHERE a.tenant_id = current_setting('app.tenant_id', true) AND a.collection_id = $1 AND a.model_id = $2 AND a.modality = 'image'
  AND ($3 = '' OR (ea.case_id = $3 AND eb.case_id = $3))
  AND (a.embedding <=> b.embedding) <= $4`

// duplicateTierRank orders the kinds of link: a stronger link wins when two pictures are linked in more than one way.
func duplicateTierRank(tier string) int {
	switch tier {
	case "identical_file":
		return 0
	case "same_image":
		return 1
	}
	return 2
}

// groupDuplicates stacks the pictures by union of the links. The lead of a group is its largest file (the best quality copy; the smallest id breaks a tie); every other member says how it is linked
// to the lead: directly (identical, saved again, edited) or only through other copies.
func groupDuplicates(images []duplicateImage, edges []duplicateEdge) (hits []imageSearchHit, summary imageDuplicateSummary) {
	index := map[string]int{}
	parent := make([]int, len(images))
	for i, im := range images {
		index[im.ID] = i
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[rb] = ra
		}
	}
	best := map[[2]string]duplicateEdge{}
	for _, e := range edges {
		ia, oka := index[e.A]
		ib, okb := index[e.B]
		if !oka || !okb {
			continue
		}
		union(ia, ib)
		key := [2]string{e.A, e.B}
		if e.B < e.A {
			key = [2]string{e.B, e.A}
		}
		if current, ok := best[key]; !ok || duplicateTierRank(e.Tier) < duplicateTierRank(current.Tier) {
			best[key] = e
		}
	}
	members := map[int][]int{}
	for i := range images {
		members[find(i)] = append(members[find(i)], i)
	}
	type group struct{ ids []int }
	var groups []group
	for _, ids := range members {
		if len(ids) > 1 {
			groups = append(groups, group{ids})
		}
	}
	lead := func(g group) int {
		l := g.ids[0]
		for _, i := range g.ids[1:] {
			if images[i].Size > images[l].Size || (images[i].Size == images[l].Size && images[i].ID < images[l].ID) {
				l = i
			}
		}
		return l
	}
	sort.SliceStable(groups, func(i, j int) bool {
		if len(groups[i].ids) != len(groups[j].ids) {
			return len(groups[i].ids) > len(groups[j].ids)
		}
		return images[lead(groups[i])].ID < images[lead(groups[j])].ID
	})
	summary.Considered = len(images)
	summary.Groups = len(groups)
	for _, g := range groups {
		summary.Pictures += len(g.ids)
		summary.CouldSetAside += len(g.ids) - 1
	}
	shown := groups
	if len(shown) > imageDuplicateMaxGroups {
		shown = shown[:imageDuplicateMaxGroups]
	}
	summary.GroupsShown = len(shown)
	mk := func(im duplicateImage, tier string, group, size int) imageSearchHit {
		return imageSearchHit{EvidenceID: im.ID, OriginalFilename: im.Name, ContentType: im.ContentType, CaseID: im.CaseID, Tier: tier, TierLabel: imageTierLabels[tier], ContentPath: "/evidence/" + im.ID + "/content", Group: group, GroupSize: size}
	}
	for n, g := range shown {
		l := lead(g)
		hits = append(hits, mk(images[l], imageTierDuplicateLead, n+1, len(g.ids)))
		var rest []int
		for _, i := range g.ids {
			if i != l {
				rest = append(rest, i)
			}
		}
		sort.Slice(rest, func(a, b int) bool { return images[rest[a]].ID < images[rest[b]].ID })
		for _, i := range rest {
			key := [2]string{images[l].ID, images[i].ID}
			if images[i].ID < images[l].ID {
				key = [2]string{images[i].ID, images[l].ID}
			}
			hit := mk(images[i], imageTierLinkedCopy, n+1, len(g.ids))
			if edge, ok := best[key]; ok {
				hit.Tier, hit.TierLabel, hit.HashDistance, hit.CosineDistance = edge.Tier, imageTierLabels[edge.Tier], edge.Hash, edge.Distance
			}
			hits = append(hits, hit)
		}
	}
	return hits, summary
}

func duplicateStatement(s imageDuplicateSummary) string {
	if s.Groups == 0 {
		text := fmt.Sprintf("No duplicate or near-duplicate pictures among the %d processed pictures.", s.Considered)
		if s.Limited {
			text += " Only identical files were compared: the case is larger than the limit for comparing pictures by appearance."
		}
		return text
	}
	text := fmt.Sprintf("%d %s of duplicate or near-duplicate pictures, %d pictures in all; %d could be set aside.", s.Groups, plural(s.Groups, "group", "groups"), s.Pictures, s.CouldSetAside)
	if s.GroupsShown < s.Groups {
		text += fmt.Sprintf(" The %d largest groups are shown.", s.GroupsShown)
	}
	if s.Limited {
		text += " Only identical files were compared: the case is larger than the limit for comparing pictures by appearance."
	}
	return text
}

// imageDuplicateGroupsRun reads the pictures and their links inside the open transaction and groups them.
func imageDuplicateGroupsRun(ctx context.Context, tx pgx.Tx, scope forensicScope, settings imageSearchSettings) ([]imageSearchHit, imageDuplicateSummary, error) {
	rows, err := tx.Query(ctx, imageDuplicateImagesSQL, scope.CollectionID, scope.CaseID, imageDuplicateMaxImages+1)
	if err != nil {
		return nil, imageDuplicateSummary{}, err
	}
	var images []duplicateImage
	for rows.Next() {
		var im duplicateImage
		if err := rows.Scan(&im.ID, &im.Name, &im.SHA, &im.Size, &im.ContentType, &im.CaseID); err != nil {
			rows.Close()
			return nil, imageDuplicateSummary{}, err
		}
		images = append(images, im)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, imageDuplicateSummary{}, err
	}
	limited := len(images) > imageDuplicateMaxImages
	if limited {
		images = images[:imageDuplicateMaxImages]
	}
	var edges []duplicateEdge
	bySHA := map[string]string{}
	for _, im := range images {
		if first, ok := bySHA[im.SHA]; ok {
			edges = append(edges, duplicateEdge{A: first, B: im.ID, Tier: "identical_file"})
		} else {
			bySHA[im.SHA] = im.ID
		}
	}
	if !limited {
		hashRows, err := tx.Query(ctx, imageDuplicateHashSQL, scope.CollectionID, scope.CaseID, settings.HashMaxDistance)
		if err != nil {
			return nil, imageDuplicateSummary{}, err
		}
		for hashRows.Next() {
			var a, b string
			var bits int
			if err := hashRows.Scan(&a, &b, &bits); err != nil {
				hashRows.Close()
				return nil, imageDuplicateSummary{}, err
			}
			edges = append(edges, duplicateEdge{A: a, B: b, Tier: "same_image", Hash: &bits})
		}
		hashRows.Close()
		if err := hashRows.Err(); err != nil {
			return nil, imageDuplicateSummary{}, err
		}
		vectorRows, err := tx.Query(ctx, imageDuplicateVectorSQL, scope.CollectionID, imageSearchDinoModelID, scope.CaseID, settings.DinoMaxDistance)
		if err != nil {
			return nil, imageDuplicateSummary{}, err
		}
		for vectorRows.Next() {
			var a, b string
			var distance float64
			if err := vectorRows.Scan(&a, &b, &distance); err != nil {
				vectorRows.Close()
				return nil, imageDuplicateSummary{}, err
			}
			d := distance
			edges = append(edges, duplicateEdge{A: a, B: b, Tier: "same_photograph", Distance: &d})
		}
		vectorRows.Close()
		if err := vectorRows.Err(); err != nil {
			return nil, imageDuplicateSummary{}, err
		}
	}
	hits, summary := groupDuplicates(images, edges)
	summary.Limited = limited
	summary.Statement = duplicateStatement(summary)
	return hits, summary, nil
}
