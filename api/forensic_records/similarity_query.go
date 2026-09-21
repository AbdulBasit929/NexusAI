package main

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

func isImageSimilarityQuery(query string) bool {
	return containsAny(normalizeQuestion(query), []string{"find images similar", "similar images", "visually similar images", "images look most similar", "visual similarity"})
}

func isFaceSimilarityQuery(query string) bool {
	return containsAny(normalizeQuestion(query), []string{"compare these two face candidates", "find candidate faces similar", "candidate faces visually similar", "faces visually similar", "closest face candidate", "similar face candidates", "face similarity"})
}

const selectedSimilarityObservationSQL = `
SELECT artifacts.metadata->>'observation_id' AS observation_id
FROM forensic.derived_artifacts artifacts
JOIN forensic.evidence_items evidence
  ON evidence.tenant_id=artifacts.tenant_id
 AND evidence.collection_id=artifacts.collection_id
 AND evidence.evidence_id=artifacts.evidence_id
 AND evidence.current_version_id=artifacts.version_id
WHERE artifacts.tenant_id=$1 AND artifacts.collection_id=$2
  AND artifacts.evidence_id=$3::uuid AND artifacts.version_id=$4::uuid
  AND artifacts.artifact_type=$5 AND artifacts.processing_status='completed'
  AND nullif(artifacts.metadata->>'observation_id', '') IS NOT NULL
ORDER BY artifacts.created_at, artifacts.artifact_id
LIMIT 1`

func selectedEvidenceSimilarity(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, face bool) (map[string]any, error) {
	contract := imageEmbeddingContract
	if face {
		contract = faceObservationContract
	}
	rows, err := queryRows(ctx, db, req.TenantID, selectedSimilarityObservationSQL, req.TenantID, req.CollectionID, req.EvidenceID, req.EvidenceVersionID, contract)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return map[string]any{
			"row_count": 0, "similarity_results": []map[string]any{}, "status": "complete_zero",
			"evidence_id": req.EvidenceID, "version_id": req.EvidenceVersionID,
			"limitations": []string{"The selected current evidence version has no eligible persisted similarity observation; no score was inferred."},
		}, nil
	}
	observationID := stringValueAny(rows[0]["observation_id"])
	var result map[string]any
	if face {
		result, err = loadFaceSimilarity(ctx, db, req.TenantID, req.CollectionID, "", observationID, nil, min(req.Limit, 10))
	} else {
		result, err = loadImageSimilarity(ctx, db, req.TenantID, req.CollectionID, "", observationID, nil, min(req.Limit, 10))
	}
	if err != nil {
		return nil, fmt.Errorf("load selected-evidence similarity: %w", err)
	}
	results := evidenceResults(result)
	queryMetadata, _ := result["query"].(map[string]any)
	ids := []string{stringValueAny(queryMetadata["evidence_id"])}
	for _, item := range results {
		ids = append(ids, stringValueAny(item["candidate_evidence_id"]))
	}
	// Hydrate only the bounded, already-authorized result set, independently of
	// the catalog page currently open in the browser. Ranking is unchanged.
	names, err := queryRows(ctx, db, req.TenantID, `SELECT evidence_id::text, current_version_id::text AS version_id, original_filename AS source_file FROM forensic.evidence_items WHERE tenant_id=$1 AND collection_id=$2 AND evidence_id::text=ANY($3::text[])`, req.TenantID, req.CollectionID, ids)
	if err != nil {
		return nil, fmt.Errorf("load similarity source labels: %w", err)
	}
	sources := map[string]any{}
	for _, name := range names {
		sources[stringValueAny(name["evidence_id"])+"/"+stringValueAny(name["version_id"])] = name["source_file"]
	}
	for _, item := range results {
		item["source_file"] = sources[stringValueAny(item["candidate_evidence_id"])+"/"+stringValueAny(item["candidate_version_id"])]
		item["query_source_file"] = sources[stringValueAny(queryMetadata["evidence_id"])+"/"+stringValueAny(queryMetadata["version_id"])]
		item["query_evidence_id"] = queryMetadata["evidence_id"]
		item["query_version_id"] = queryMetadata["version_id"]
		item["query_citation_ref"] = queryMetadata["citation_ref"]
		item["query_citation_locator"] = queryMetadata["citation_locator"]
	}
	delete(result, "results")
	result["similarity_results"] = results
	result["row_count"] = len(results)
	if len(results) == 0 {
		result["status"] = "complete_zero"
	} else {
		result["status"] = "results_present"
	}
	return result, nil
}
