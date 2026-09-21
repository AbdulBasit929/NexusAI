package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	contributionLineageContractV1 = "forensics.contribution-lineage/v1"
	maxContributionSourceGroups   = 50
)

type contributionLineageSourceV1 struct {
	EvidenceID        string `json:"evidence_id,omitempty"`
	VersionID         string `json:"version_id,omitempty"`
	SourceFile        string `json:"source_file"`
	ContributionCount int64  `json:"contribution_count"`
	FirstRow          int64  `json:"first_row,omitempty"`
	LastRow           int64  `json:"last_row,omitempty"`
	RowGroupDigest    string `json:"row_group_digest"`
}

type contributionLineageV1 struct {
	ContractVersion   string                        `json:"contract_version"`
	OperationID       string                        `json:"operation_id"`
	ResultKey         string                        `json:"result_key,omitempty"`
	ContributionCount int64                         `json:"contribution_count"`
	SourceGroupCount  int64                         `json:"source_group_count"`
	Sources           []contributionLineageSourceV1 `json:"sources"`
	SourcesComplete   bool                          `json:"sources_complete"`
	LineageComplete   bool                          `json:"lineage_complete"`
	LineageDigest     string                        `json:"lineage_digest"`
	Parameters        map[string]string             `json:"parameters"`
}

const frequentContactContributionLineageSQL = `
WITH scoped AS (
  SELECT CASE
           WHEN regexp_replace(coalesce(r.call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g')
             THEN coalesce(nullif(r.call_org_num, ''), nullif(r.msisdn, ''))
           ELSE r.call_dialed_num
         END AS result_key,
         r.batch_id, r.source_file, r.row_number, r.row_hash
  FROM forensic.cdr_records r
  WHERE r.tenant_id = $1 AND r.collection_id = $2
    AND $3 <> ''
    AND ($4::timestamptz IS NULL OR r.call_start_ts >= $4::timestamptz)
    AND ($5::timestamptz IS NULL OR r.call_start_ts < $5::timestamptz)
    AND ($6 = '' OR r.direction = $6)
    AND (
      r.msisdn = $3 OR r.call_org_num = $3 OR r.call_dialed_num = $3
      OR (length(regexp_replace($3, '\D', '', 'g')) >= 8 AND regexp_replace(coalesce(r.msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g'))
      OR (length(regexp_replace($3, '\D', '', 'g')) >= 8 AND regexp_replace(coalesce(r.call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g'))
      OR (length(regexp_replace($3, '\D', '', 'g')) >= 8 AND regexp_replace(coalesce(r.call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g'))
    )
), selected AS (
  SELECT * FROM scoped
  WHERE result_key = ANY($7::text[])
), grouped AS (
  SELECT s.result_key,
         coalesce(j.evidence_id::text, '') AS evidence_id,
         coalesce(e.current_version_id::text, '') AS version_id,
         s.source_file,
         count(*)::bigint AS contribution_count,
         min(s.row_number)::bigint AS first_row,
         max(s.row_number)::bigint AS last_row,
         encode(digest(concat_ws('|', count(*)::text, min(s.row_hash), max(s.row_hash), min(s.row_number)::text, max(s.row_number)::text), 'sha256'), 'hex') AS row_group_digest
  FROM selected s
  LEFT JOIN forensic.records_ingest_jobs j
    ON j.tenant_id = $1 AND j.collection_id = $2 AND j.id = s.batch_id
  LEFT JOIN forensic.evidence_items e
    ON e.tenant_id = $1 AND e.collection_id = $2 AND e.evidence_id = j.evidence_id
  GROUP BY s.result_key, j.evidence_id, e.current_version_id, s.source_file
), ranked AS (
  SELECT grouped.*,
         count(*) OVER (PARTITION BY result_key)::bigint AS source_group_count,
         sum(contribution_count) OVER (PARTITION BY result_key)::bigint AS total_contribution_count,
         row_number() OVER (PARTITION BY result_key ORDER BY contribution_count DESC, source_file, evidence_id, version_id) AS source_group_position
  FROM grouped
)
SELECT result_key, evidence_id, version_id, source_file, contribution_count,
       first_row, last_row, row_group_digest, source_group_count, total_contribution_count
FROM ranked
WHERE source_group_position <= 50
ORDER BY result_key, source_group_position`

const temporalContributionLineageSQL = `
WITH grouped AS (
  SELECT ''::text AS result_key,
         coalesce(j.evidence_id::text, '') AS evidence_id,
         coalesce(e.current_version_id::text, '') AS version_id,
         r.source_file,
         count(*)::bigint AS contribution_count,
         min(r.row_number)::bigint AS first_row,
         max(r.row_number)::bigint AS last_row,
         encode(digest(concat_ws('|', count(*)::text, min(r.row_hash), max(r.row_hash), min(r.row_number)::text, max(r.row_number)::text), 'sha256'), 'hex') AS row_group_digest
  FROM forensic.cdr_records r
  LEFT JOIN forensic.records_ingest_jobs j
    ON j.tenant_id = $1 AND j.collection_id = $2 AND j.id = r.batch_id
  LEFT JOIN forensic.evidence_items e
    ON e.tenant_id = $1 AND e.collection_id = $2 AND e.evidence_id = j.evidence_id
  WHERE r.tenant_id = $1 AND r.collection_id = $2
    AND (
      $3 = '' OR r.msisdn = $3 OR r.call_org_num = $3 OR r.call_dialed_num = $3
      OR (length(regexp_replace($3, '\D', '', 'g')) >= 8 AND regexp_replace(coalesce(r.msisdn, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g'))
      OR (length(regexp_replace($3, '\D', '', 'g')) >= 8 AND regexp_replace(coalesce(r.call_org_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g'))
      OR (length(regexp_replace($3, '\D', '', 'g')) >= 8 AND regexp_replace(coalesce(r.call_dialed_num, ''), '\D', '', 'g') = regexp_replace($3, '\D', '', 'g'))
    )
    AND ($4::timestamptz IS NULL OR r.call_start_ts >= $4::timestamptz)
    AND ($5::timestamptz IS NULL OR r.call_start_ts < $5::timestamptz)
  GROUP BY j.evidence_id, e.current_version_id, r.source_file
), ranked AS (
  SELECT grouped.*,
         count(*) OVER ()::bigint AS source_group_count,
         sum(contribution_count) OVER ()::bigint AS total_contribution_count,
         row_number() OVER (ORDER BY contribution_count DESC, source_file, evidence_id, version_id) AS source_group_position
  FROM grouped
)
SELECT result_key, evidence_id, version_id, source_file, contribution_count,
       first_row, last_row, row_group_digest, source_group_count, total_contribution_count
FROM ranked
WHERE source_group_position <= 50
ORDER BY source_group_position`

func loadFrequentContactContributionLineage(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest, contacts []map[string]any) ([]contributionLineageV1, error) {
	resultKeys := make([]string, 0, len(contacts))
	for _, contact := range contacts {
		if key := stringValueAny(contact["counterparty"]); key != "" {
			resultKeys = append(resultKeys, key)
		}
	}
	if len(resultKeys) == 0 {
		return []contributionLineageV1{}, nil
	}
	rows, err := queryRows(ctx, db, req.TenantID, frequentContactContributionLineageSQL,
		req.TenantID, req.CollectionID, req.Target, dateBoundArg(req.DateFrom), dateBoundArg(req.DateTo), req.Direction, resultKeys)
	if err != nil {
		return nil, err
	}
	return buildContributionLineages("cdr.frequent_contacts", contributionParameters(req), rows), nil
}

func loadTemporalContributionLineage(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) ([]contributionLineageV1, error) {
	rows, err := queryRows(ctx, db, req.TenantID, temporalContributionLineageSQL,
		req.TenantID, req.CollectionID, req.Target, dateBoundArg(req.DateFrom), dateBoundArg(req.DateTo))
	if err != nil {
		return nil, err
	}
	return buildContributionLineages("cdr.temporal_activity", contributionParameters(req), rows), nil
}

func contributionParameters(req hybridQueryRequest) map[string]string {
	return map[string]string{
		"collection_id": req.CollectionID,
		"target":        req.Target,
		"date_from":     req.DateFrom,
		"date_to":       req.DateTo,
		"direction":     req.Direction,
	}
}

func buildContributionLineages(operationID string, parameters map[string]string, rows []map[string]any) []contributionLineageV1 {
	grouped := map[string][]map[string]any{}
	for _, row := range rows {
		key := stringValueAny(row["result_key"])
		grouped[key] = append(grouped[key], row)
	}
	keys := make([]string, 0, len(grouped))
	for key := range grouped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lineages := make([]contributionLineageV1, 0, len(keys))
	for _, key := range keys {
		groupRows := grouped[key]
		lineage := contributionLineageV1{
			ContractVersion: contributionLineageContractV1,
			OperationID:     operationID,
			ResultKey:       key,
			Parameters:      copyContributionParameters(parameters),
			Sources:         make([]contributionLineageSourceV1, 0, len(groupRows)),
		}
		if len(groupRows) > 0 {
			lineage.ContributionCount = valueAsInt(groupRows[0]["total_contribution_count"])
			lineage.SourceGroupCount = valueAsInt(groupRows[0]["source_group_count"])
		}
		var displayedContributionCount int64
		identitiesComplete := true
		for _, row := range groupRows {
			source := contributionLineageSourceV1{
				EvidenceID:        stringValueAny(row["evidence_id"]),
				VersionID:         stringValueAny(row["version_id"]),
				SourceFile:        stringValueAny(row["source_file"]),
				ContributionCount: valueAsInt(row["contribution_count"]),
				FirstRow:          valueAsInt(row["first_row"]),
				LastRow:           valueAsInt(row["last_row"]),
				RowGroupDigest:    stringValueAny(row["row_group_digest"]),
			}
			displayedContributionCount += source.ContributionCount
			if source.EvidenceID == "" || source.VersionID == "" || source.SourceFile == "" || source.RowGroupDigest == "" {
				identitiesComplete = false
			}
			lineage.Sources = append(lineage.Sources, source)
		}
		lineage.SourcesComplete = lineage.SourceGroupCount > 0 && lineage.SourceGroupCount <= maxContributionSourceGroups && int64(len(lineage.Sources)) == lineage.SourceGroupCount
		lineage.LineageComplete = lineage.SourcesComplete && identitiesComplete && displayedContributionCount == lineage.ContributionCount && lineage.ContributionCount > 0
		lineage.LineageDigest = contributionLineageDigest(lineage)
		lineages = append(lineages, lineage)
	}
	return lineages
}

func contributionLineageDigest(lineage contributionLineageV1) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s|%s|%s|%d|%d|", lineage.ContractVersion, lineage.OperationID, lineage.ResultKey, lineage.ContributionCount, lineage.SourceGroupCount)
	parameterKeys := make([]string, 0, len(lineage.Parameters))
	for key := range lineage.Parameters {
		parameterKeys = append(parameterKeys, key)
	}
	sort.Strings(parameterKeys)
	for _, key := range parameterKeys {
		fmt.Fprintf(&b, "%s=%s|", key, lineage.Parameters[key])
	}
	for _, source := range lineage.Sources {
		fmt.Fprintf(&b, "%s|%s|%s|%d|%d|%d|%s|", source.EvidenceID, source.VersionID, source.SourceFile, source.ContributionCount, source.FirstRow, source.LastRow, source.RowGroupDigest)
	}
	digest := sha256.Sum256([]byte(b.String()))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func copyContributionParameters(parameters map[string]string) map[string]string {
	out := make(map[string]string, len(parameters))
	for key, value := range parameters {
		out[key] = value
	}
	return out
}

func validateContributionLineage(lineage contributionLineageV1) error {
	if lineage.ContractVersion != contributionLineageContractV1 {
		return fmt.Errorf("unsupported contribution-lineage contract %q", lineage.ContractVersion)
	}
	if strings.TrimSpace(lineage.OperationID) == "" || lineage.ContributionCount <= 0 || lineage.SourceGroupCount <= 0 {
		return fmt.Errorf("operation, contribution count, and source-group count are required")
	}
	if len(lineage.Sources) == 0 || len(lineage.Sources) > maxContributionSourceGroups || int64(len(lineage.Sources)) > lineage.SourceGroupCount {
		return fmt.Errorf("contribution sources must contain between 1 and %d bounded groups", maxContributionSourceGroups)
	}
	if lineage.SourcesComplete != (int64(len(lineage.Sources)) == lineage.SourceGroupCount) {
		return fmt.Errorf("sources_complete does not match the bounded source groups")
	}
	if !strings.HasPrefix(lineage.LineageDigest, "sha256:") || len(lineage.LineageDigest) != len("sha256:")+64 {
		return fmt.Errorf("lineage digest must be a SHA-256 locator")
	}
	return nil
}

func contributionLineageProvenance(value any) []map[string]any {
	lineages, ok := value.([]contributionLineageV1)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(lineages))
	for _, lineage := range lineages {
		item := map[string]any{
			"source":               "records_aggregate",
			"contract_version":     lineage.ContractVersion,
			"operation_id":         lineage.OperationID,
			"result_key":           lineage.ResultKey,
			"contribution_count":   lineage.ContributionCount,
			"source_group_count":   lineage.SourceGroupCount,
			"sources_complete":     lineage.SourcesComplete,
			"lineage_complete":     lineage.LineageComplete,
			"lineage_digest":       lineage.LineageDigest,
			"contribution_sources": lineage.Sources,
		}
		if len(lineage.Sources) == 1 {
			item["source_file"] = lineage.Sources[0].SourceFile
			item["evidence_id"] = lineage.Sources[0].EvidenceID
			item["version_id"] = lineage.Sources[0].VersionID
		}
		out = append(out, item)
	}
	return out
}

func contributionLineageIncomplete(value any) bool {
	lineages, ok := value.([]contributionLineageV1)
	if !ok || len(lineages) == 0 {
		return true
	}
	for _, lineage := range lineages {
		if !lineage.LineageComplete {
			return true
		}
	}
	return false
}
