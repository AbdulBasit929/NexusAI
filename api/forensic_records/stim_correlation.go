package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	structuredSourceSetContractV1 = "forensics.structured-source-set/v1"
	multiCDRComparisonContractV1  = "forensics.multi-cdr-comparison/v1"
	maxStructuredSources          = 8
	maxMultiSourceCDRRows         = 5000
)

type StructuredSourceRefV1 struct {
	SourceID   string `json:"source_id"`
	EvidenceID string `json:"evidence_id,omitempty"`
	VersionID  string `json:"version_id,omitempty"`
	SourceFile string `json:"source_file"`
}

type StructuredSourceSetV1 struct {
	ContractVersion string                  `json:"contract_version"`
	Sources         []StructuredSourceRefV1 `json:"sources"`
}

const structuredSourceMembershipErrorCode = "invalid_source_membership"

type structuredSourceMembershipError struct {
	reason string
}

func (e *structuredSourceMembershipError) Error() string {
	return "selected source set is invalid for the authorized collection"
}

func (e *structuredSourceMembershipError) Unwrap() error {
	return errors.New(e.reason)
}

func validateStructuredSourceSet(sourceSet StructuredSourceSetV1) error {
	if sourceSet.ContractVersion != structuredSourceSetContractV1 {
		return fmt.Errorf("unsupported source-set contract %q", sourceSet.ContractVersion)
	}
	if len(sourceSet.Sources) < 2 || len(sourceSet.Sources) > maxStructuredSources {
		return fmt.Errorf("source set requires between 2 and %d exact sources", maxStructuredSources)
	}
	seenIDs := map[string]struct{}{}
	seenFiles := map[string]struct{}{}
	for _, source := range sourceSet.Sources {
		id := strings.TrimSpace(source.SourceID)
		file := strings.TrimSpace(source.SourceFile)
		if id == "" || file == "" {
			return errors.New("every source-set member requires source_id and source_file")
		}
		if (source.EvidenceID == "") != (source.VersionID == "") {
			return fmt.Errorf("source %q must provide evidence_id and version_id together", id)
		}
		if _, exists := seenIDs[id]; exists {
			return fmt.Errorf("duplicate source_id %q", id)
		}
		if _, exists := seenFiles[file]; exists {
			return fmt.Errorf("duplicate source_file %q", file)
		}
		seenIDs[id] = struct{}{}
		seenFiles[file] = struct{}{}
	}
	return nil
}

func normalizeStructuredSourceSet(sourceSet *StructuredSourceSetV1) {
	if sourceSet == nil {
		return
	}
	sourceSet.ContractVersion = strings.TrimSpace(sourceSet.ContractVersion)
	for index := range sourceSet.Sources {
		sourceSet.Sources[index].SourceID = strings.TrimSpace(sourceSet.Sources[index].SourceID)
		sourceSet.Sources[index].EvidenceID = strings.TrimSpace(sourceSet.Sources[index].EvidenceID)
		sourceSet.Sources[index].VersionID = strings.TrimSpace(sourceSet.Sources[index].VersionID)
		sourceSet.Sources[index].SourceFile = strings.TrimSpace(sourceSet.Sources[index].SourceFile)
	}
}

func structuredSourceFiles(sourceSet *StructuredSourceSetV1) []string {
	if sourceSet == nil {
		return []string{}
	}
	files := make([]string, 0, len(sourceSet.Sources))
	for _, source := range sourceSet.Sources {
		files = append(files, source.SourceFile)
	}
	return files
}

func structuredSourceIDs(sourceSet *StructuredSourceSetV1) []string {
	if sourceSet == nil {
		return []string{}
	}
	ids := make([]string, 0, len(sourceSet.Sources))
	for _, source := range sourceSet.Sources {
		ids = append(ids, source.SourceID)
	}
	return ids
}

const multiSourceCDRRowsSQL = `
SELECT c.source_file, c.file_id, c.batch_id::text AS batch_id, c.row_number, c.row_hash,
       c.msisdn, c.call_org_num, c.call_dialed_num, c.direction, c.call_type,
       c.imei, c.imsi, coalesce(nullif(c.cell_site_id, ''), nullif(c.site_id, '')) AS tower,
       c.call_start_ts, c.duration_seconds,
       r.record_id, r.evidence_id,
       coalesce(e.current_version_id::text, nullif(e.metadata #>> '{version_id}', '')) AS version_id
FROM forensic.cdr_records c
LEFT JOIN forensic.records r
  ON r.tenant_id = c.tenant_id
 AND r.collection_id = c.collection_id
 AND r.file_id = c.file_id
 AND r.batch_id = c.batch_id
 AND r.row_hash = c.row_hash
 AND r.record_type = 'cdr'
LEFT JOIN forensic.evidence_items e
  ON e.tenant_id = r.tenant_id
 AND e.collection_id = r.collection_id
 AND e.evidence_id = r.evidence_id
WHERE c.tenant_id = $1 AND c.collection_id = $2
  AND EXISTS (
    SELECT 1
    FROM unnest($3::text[], $4::text[], $5::text[], $6::text[])
         AS selected(source_id, source_file, evidence_id, version_id)
    WHERE c.file_id::text = selected.source_id AND c.source_file = selected.source_file
      AND (
        selected.evidence_id = ''
        OR (
          r.evidence_id::text = selected.evidence_id
          AND coalesce(e.current_version_id::text, nullif(e.metadata #>> '{version_id}', '')) = selected.version_id
        )
      )
  )
  AND ($8::timestamptz IS NULL OR c.call_start_ts >= $8::timestamptz)
  AND ($9::timestamptz IS NULL OR c.call_start_ts < $9::timestamptz)
  AND (
    c.msisdn = $7 OR c.call_org_num = $7 OR c.call_dialed_num = $7
    OR (
      length(regexp_replace($7, '\D', '', 'g')) >= 8
      AND (
        CASE
          WHEN regexp_replace(coalesce(c.msisdn, ''), '\D', '', 'g') LIKE '0092%'
            THEN substring(regexp_replace(coalesce(c.msisdn, ''), '\D', '', 'g') FROM 3)
          WHEN regexp_replace(coalesce(c.msisdn, ''), '\D', '', 'g') ~ '^03[0-9]{9}$'
            THEN '92' || substring(regexp_replace(coalesce(c.msisdn, ''), '\D', '', 'g') FROM 2)
          ELSE regexp_replace(coalesce(c.msisdn, ''), '\D', '', 'g')
        END
      ) = (
        CASE
          WHEN regexp_replace($7, '\D', '', 'g') LIKE '0092%'
            THEN substring(regexp_replace($7, '\D', '', 'g') FROM 3)
          WHEN regexp_replace($7, '\D', '', 'g') ~ '^03[0-9]{9}$'
            THEN '92' || substring(regexp_replace($7, '\D', '', 'g') FROM 2)
          ELSE regexp_replace($7, '\D', '', 'g')
        END
      )
    )
    OR (
      length(regexp_replace($7, '\D', '', 'g')) >= 8
      AND (
        CASE
          WHEN regexp_replace(coalesce(c.call_org_num, ''), '\D', '', 'g') LIKE '0092%'
            THEN substring(regexp_replace(coalesce(c.call_org_num, ''), '\D', '', 'g') FROM 3)
          WHEN regexp_replace(coalesce(c.call_org_num, ''), '\D', '', 'g') ~ '^03[0-9]{9}$'
            THEN '92' || substring(regexp_replace(coalesce(c.call_org_num, ''), '\D', '', 'g') FROM 2)
          ELSE regexp_replace(coalesce(c.call_org_num, ''), '\D', '', 'g')
        END
      ) = (
        CASE
          WHEN regexp_replace($7, '\D', '', 'g') LIKE '0092%'
            THEN substring(regexp_replace($7, '\D', '', 'g') FROM 3)
          WHEN regexp_replace($7, '\D', '', 'g') ~ '^03[0-9]{9}$'
            THEN '92' || substring(regexp_replace($7, '\D', '', 'g') FROM 2)
          ELSE regexp_replace($7, '\D', '', 'g')
        END
      )
    )
    OR (
      length(regexp_replace($7, '\D', '', 'g')) >= 8
      AND (
        CASE
          WHEN regexp_replace(coalesce(c.call_dialed_num, ''), '\D', '', 'g') LIKE '0092%'
            THEN substring(regexp_replace(coalesce(c.call_dialed_num, ''), '\D', '', 'g') FROM 3)
          WHEN regexp_replace(coalesce(c.call_dialed_num, ''), '\D', '', 'g') ~ '^03[0-9]{9}$'
            THEN '92' || substring(regexp_replace(coalesce(c.call_dialed_num, ''), '\D', '', 'g') FROM 2)
          ELSE regexp_replace(coalesce(c.call_dialed_num, ''), '\D', '', 'g')
        END
      ) = (
        CASE
          WHEN regexp_replace($7, '\D', '', 'g') LIKE '0092%'
            THEN substring(regexp_replace($7, '\D', '', 'g') FROM 3)
          WHEN regexp_replace($7, '\D', '', 'g') ~ '^03[0-9]{9}$'
            THEN '92' || substring(regexp_replace($7, '\D', '', 'g') FROM 2)
          ELSE regexp_replace($7, '\D', '', 'g')
        END
      )
    )
  )
ORDER BY c.call_start_ts, c.source_file, c.row_number
LIMIT $10`

const structuredSourceMembershipSQL = `
WITH selected AS (
  SELECT source_id, source_file, evidence_id, version_id, ordinality
  FROM unnest($3::text[], $4::text[], $5::text[], $6::text[])
       WITH ORDINALITY AS requested(source_id, source_file, evidence_id, version_id, ordinality)
)
SELECT selected.source_id, selected.source_file, selected.evidence_id, selected.version_id,
       (
         SELECT count(*)
         FROM forensic.cdr_records c
         WHERE c.tenant_id = $1 AND c.collection_id = $2
           AND c.file_id::text = selected.source_id AND c.source_file = selected.source_file
       ) AS cdr_record_count,
       (
         SELECT count(*)
         FROM forensic.records r
         WHERE r.tenant_id = $1 AND r.collection_id = $2
            AND r.file_id::text = selected.source_id AND r.source_file = selected.source_file
           AND r.record_type <> 'cdr'
       ) AS unsupported_record_count,
       (
         SELECT count(*)
         FROM forensic.records r
         JOIN forensic.evidence_items e
           ON e.tenant_id = r.tenant_id
          AND e.collection_id = r.collection_id
          AND e.evidence_id = r.evidence_id
         WHERE r.tenant_id = $1 AND r.collection_id = $2
            AND r.file_id::text = selected.source_id AND r.source_file = selected.source_file
           AND r.record_type = 'cdr'
            AND r.evidence_id::text = selected.evidence_id
           AND coalesce(e.current_version_id::text, nullif(e.metadata #>> '{version_id}', '')) = selected.version_id
       ) AS evidence_version_record_count
FROM selected
ORDER BY selected.ordinality`

type structuredQueryRowsFunc func(context.Context, string, string, ...any) ([]map[string]any, error)

func structuredSourceEvidenceIDs(sourceSet *StructuredSourceSetV1) []string {
	values := make([]string, 0, len(sourceSet.Sources))
	for _, source := range sourceSet.Sources {
		values = append(values, source.EvidenceID)
	}
	return values
}

func structuredSourceVersionIDs(sourceSet *StructuredSourceSetV1) []string {
	values := make([]string, 0, len(sourceSet.Sources))
	for _, source := range sourceSet.Sources {
		values = append(values, source.VersionID)
	}
	return values
}

func validateStructuredSourceMembership(sourceSet StructuredSourceSetV1, rows []map[string]any) error {
	if len(rows) != len(sourceSet.Sources) {
		return &structuredSourceMembershipError{reason: "membership lookup did not resolve every selected source"}
	}
	for index, source := range sourceSet.Sources {
		row := rows[index]
		if stringValueAny(row["source_id"]) != source.SourceID || stringValueAny(row["source_file"]) != source.SourceFile {
			return &structuredSourceMembershipError{reason: "membership lookup returned a different source identity"}
		}
		if valueAsInt(row["cdr_record_count"]) == 0 {
			return &structuredSourceMembershipError{reason: "selected source has no retained CDR records"}
		}
		if valueAsInt(row["unsupported_record_count"]) != 0 {
			return &structuredSourceMembershipError{reason: "selected source contains an unsupported evidence family"}
		}
		if source.EvidenceID != "" && valueAsInt(row["evidence_version_record_count"]) == 0 {
			return &structuredSourceMembershipError{reason: "selected evidence and version do not bind to the selected source"}
		}
	}
	return nil
}

func multiSourceCDRComparison(ctx context.Context, db *pgxpool.Pool, req hybridQueryRequest) (map[string]any, error) {
	return multiSourceCDRComparisonWithQuery(ctx, req, func(queryCtx context.Context, tenantID, query string, args ...any) ([]map[string]any, error) {
		return queryRows(queryCtx, db, tenantID, query, args...)
	})
}

func multiSourceCDRComparisonWithQuery(ctx context.Context, req hybridQueryRequest, queryFn structuredQueryRowsFunc) (map[string]any, error) {
	if req.SourceSet == nil {
		return nil, errors.New("multi-CDR comparison requires an explicit source_set")
	}
	if err := validateStructuredSourceSet(*req.SourceSet); err != nil {
		return nil, err
	}
	membership, err := queryFn(ctx, req.TenantID, structuredSourceMembershipSQL,
		req.TenantID, req.CollectionID, structuredSourceIDs(req.SourceSet), structuredSourceFiles(req.SourceSet),
		structuredSourceEvidenceIDs(req.SourceSet), structuredSourceVersionIDs(req.SourceSet))
	if err != nil {
		return nil, fmt.Errorf("validate structured source membership: %w", err)
	}
	if err := validateStructuredSourceMembership(*req.SourceSet, membership); err != nil {
		return nil, err
	}
	rows, err := queryFn(ctx, req.TenantID, multiSourceCDRRowsSQL,
		req.TenantID, req.CollectionID, structuredSourceIDs(req.SourceSet), structuredSourceFiles(req.SourceSet),
		structuredSourceEvidenceIDs(req.SourceSet), structuredSourceVersionIDs(req.SourceSet), req.Target,
		dateBoundArg(req.DateFrom), dateBoundArg(req.DateTo), maxMultiSourceCDRRows+1)
	if err != nil {
		return nil, err
	}
	if len(rows) > maxMultiSourceCDRRows {
		return nil, fmt.Errorf("multi-CDR comparison exceeds the %d-row materialization limit", maxMultiSourceCDRRows)
	}
	sourceByFile := map[string]StructuredSourceRefV1{}
	for _, source := range req.SourceSet.Sources {
		sourceByFile[source.SourceFile] = source
	}
	observations := make([]multiCDRObservation, 0, len(rows))
	for _, row := range rows {
		sourceFile := stringValueAny(row["source_file"])
		source, selected := sourceByFile[sourceFile]
		if !selected {
			continue
		}
		evidenceID := stringValueAny(row["evidence_id"])
		versionID := stringValueAny(row["version_id"])
		if source.EvidenceID != "" && (source.EvidenceID != evidenceID || source.VersionID != versionID) {
			return nil, &structuredSourceMembershipError{reason: "analytical row violated the validated evidence/version source binding"}
		}
		startedAt, ok := forensicTimeValue(row["call_start_ts"])
		if !ok {
			continue
		}
		var duration *int64
		if row["duration_seconds"] != nil {
			value := int64FromAny(row["duration_seconds"])
			duration = &value
		}
		observationID := defaultString(stringValueAny(row["record_id"]), source.SourceID+":"+stringValueAny(row["row_hash"]))
		observations = append(observations, multiCDRObservation{
			ObservationID: observationID, SourceID: source.SourceID, SourceFile: sourceFile,
			EvidenceID: evidenceID, VersionID: versionID, RowNumber: int64FromAny(row["row_number"]), RowHash: stringValueAny(row["row_hash"]),
			MSISDN: stringValueAny(row["msisdn"]), Caller: stringValueAny(row["call_org_num"]), Callee: stringValueAny(row["call_dialed_num"]),
			Direction: stringValueAny(row["direction"]), EventType: stringValueAny(row["call_type"]),
			IMEI: stringValueAny(row["imei"]), IMSI: stringValueAny(row["imsi"]), Tower: stringValueAny(row["tower"]), StartedAt: startedAt, Duration: duration,
		})
	}
	return buildMultiCDRComparison(*req.SourceSet, req.Target, observations, req.Limit)
}

type multiCDRObservation struct {
	ObservationID string
	SourceID      string
	SourceFile    string
	EvidenceID    string
	VersionID     string
	RowNumber     int64
	RowHash       string
	MSISDN        string
	Caller        string
	Callee        string
	Direction     string
	EventType     string
	IMEI          string
	IMSI          string
	Tower         string
	StartedAt     time.Time
	Duration      *int64
}

type multiCDRContactAggregate struct {
	Contact        string
	InteractionCnt int64
	IncomingCnt    int64
	OutgoingCnt    int64
	DurationSum    int64
	DurationCnt    int64
	FirstSeen      time.Time
	LastSeen       time.Time
	Citations      []map[string]any
}

type multiCDRIdentifierAggregate struct {
	Identifier string
	Sources    map[string]struct{}
	Citations  []map[string]any
}

type acceptedMultiCDREvent struct {
	observation  multiCDRObservation
	sourceID     string
	counterparty string
	direction    string
	baseKey      string
	exactKey     string
}

func buildMultiCDRComparison(sourceSet StructuredSourceSetV1, target string, observations []multiCDRObservation, resultLimit int) (map[string]any, error) {
	if err := validateStructuredSourceSet(sourceSet); err != nil {
		return nil, err
	}
	targetKey := canonicalPhoneKey(target)
	if len(targetKey) < 8 {
		return nil, errors.New("multi-CDR comparison requires one exact phone/MSISDN target")
	}
	if len(observations) > maxMultiSourceCDRRows {
		return nil, fmt.Errorf("multi-CDR comparison exceeds the %d-row materialization limit", maxMultiSourceCDRRows)
	}
	if resultLimit < 1 {
		resultLimit = defaultHybridLimit
	}
	resultLimit = minInt(resultLimit, maxHybridLimit)

	sourceByFile := map[string]string{}
	sourceOrder := make([]string, 0, len(sourceSet.Sources))
	for _, source := range sourceSet.Sources {
		sourceByFile[source.SourceFile] = source.SourceID
		sourceOrder = append(sourceOrder, source.SourceID)
	}

	contactsBySource := map[string]map[string]*multiCDRContactAggregate{}
	identifierGroups := map[string]map[string]*multiCDRIdentifierAggregate{
		"imei": {}, "imsi": {}, "tower": {},
	}
	accepted := make([]acceptedMultiCDREvent, 0, len(observations))
	for _, observation := range observations {
		sourceID, selected := sourceByFile[observation.SourceFile]
		if !selected {
			continue
		}
		if observation.SourceID != "" && observation.SourceID != sourceID {
			return nil, fmt.Errorf("observation %q source identity conflicts with the source set", observation.ObservationID)
		}
		counterparty, direction, ok := cdrCounterpartyForTarget(observation, targetKey)
		if !ok {
			continue
		}
		citation := multiCDRCitation(observation, sourceID)
		byContact := contactsBySource[sourceID]
		if byContact == nil {
			byContact = map[string]*multiCDRContactAggregate{}
			contactsBySource[sourceID] = byContact
		}
		aggregate := byContact[counterparty]
		if aggregate == nil {
			aggregate = &multiCDRContactAggregate{Contact: counterparty, FirstSeen: observation.StartedAt, LastSeen: observation.StartedAt}
			byContact[counterparty] = aggregate
		}
		aggregate.InteractionCnt++
		if direction == "INCOMING" {
			aggregate.IncomingCnt++
		} else {
			aggregate.OutgoingCnt++
		}
		if observation.Duration != nil && *observation.Duration >= 0 {
			aggregate.DurationSum += *observation.Duration
			aggregate.DurationCnt++
		}
		if observation.StartedAt.Before(aggregate.FirstSeen) {
			aggregate.FirstSeen = observation.StartedAt
		}
		if observation.StartedAt.After(aggregate.LastSeen) {
			aggregate.LastSeen = observation.StartedAt
		}
		aggregate.Citations = append(aggregate.Citations, citation)

		for kind, value := range map[string]string{"imei": observation.IMEI, "imsi": observation.IMSI, "tower": observation.Tower} {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			group := identifierGroups[kind][value]
			if group == nil {
				group = &multiCDRIdentifierAggregate{Identifier: value, Sources: map[string]struct{}{}}
				identifierGroups[kind][value] = group
			}
			group.Sources[sourceID] = struct{}{}
			group.Citations = append(group.Citations, citation)
		}

		baseKey := strings.Join([]string{canonicalPhoneKey(observation.Caller), canonicalPhoneKey(observation.Callee), observation.StartedAt.UTC().Format(time.RFC3339Nano), canonicalCDRServiceClass(observation.EventType)}, "|")
		durationKey := "missing"
		if observation.Duration != nil {
			durationKey = fmt.Sprintf("%d", *observation.Duration)
		}
		exactKey := strings.Join([]string{baseKey, durationKey, strings.TrimSpace(observation.IMEI), strings.TrimSpace(observation.IMSI), strings.TrimSpace(observation.Tower), direction}, "|")
		accepted = append(accepted, acceptedMultiCDREvent{observation: observation, sourceID: sourceID, counterparty: counterparty, direction: direction, baseKey: baseKey, exactKey: exactKey})
	}

	common, unique := multiCDRContactSets(sourceOrder, contactsBySource)
	contactComparison := multiCDRContactRows(sourceOrder, contactsBySource, resultLimit)
	shared := map[string]any{}
	for _, kind := range []string{"imei", "imsi", "tower"} {
		shared[kind] = multiCDRSharedIdentifierRows(kind, identifierGroups[kind], resultLimit)
	}
	temporal := multiCDRExactTemporalOverlaps(accepted, resultLimit)
	duplicates, conflicts := multiCDRDuplicateAndConflictRows(accepted, resultLimit)
	relationships := multiCDRRelationshipRows(target, common, unique, shared, temporal, conflicts, resultLimit)
	graph := multiCDRGraph(target, relationships, resultLimit)

	limitations := []string{
		"Results compare only the exact selected source files and target; absent links are no evidence, not evidence of absence.",
		"Shared contacts, devices, SIMs, towers, and timestamps are observations and do not prove identity, ownership, presence, association, causation, or intent.",
		"Temporal overlap means the exact same normalized event instant in two selected sources; it does not establish a meeting or shared activity.",
		"Cross-source duplicate candidates remain separate observations; NexusAI does not collapse or reconcile them at query time.",
		"The enterprise provenance view is bounded to 200 distinct observations; citations attached to displayed rows are direct support, but the bounded view is not complete aggregate contribution lineage when more than 200 observations match.",
	}
	return map[string]any{
		"contract_version":     multiCDRComparisonContractV1,
		"source_set":           sourceSet,
		"target":               target,
		"source_count":         len(sourceOrder),
		"matched_event_count":  len(accepted),
		"common_contacts":      common,
		"unique_contacts":      unique,
		"contact_comparison":   contactComparison,
		"shared_imei":          shared["imei"],
		"shared_imsi":          shared["imsi"],
		"shared_tower":         shared["tower"],
		"temporal_overlaps":    temporal,
		"duplicate_candidates": duplicates,
		"conflicts":            conflicts,
		"relationships":        relationships,
		"graph":                graph,
		"timeline":             temporal,
		"limitations":          limitations,
		"row_count":            len(contactComparison),
	}, nil
}

func canonicalPhoneKey(value string) string {
	digits := nonDigitPattern.ReplaceAllString(value, "")
	if strings.HasPrefix(digits, "0092") {
		digits = digits[2:]
	}
	if strings.HasPrefix(digits, "03") && len(digits) == 11 {
		digits = "92" + digits[1:]
	}
	return digits
}

func canonicalCDRServiceClass(value string) string {
	token := strings.ToUpper(strings.TrimSpace(value))
	switch {
	case strings.Contains(token, "USSD"):
		return "USSD"
	case strings.Contains(token, "SMS"):
		return "SMS"
	case strings.Contains(token, "GPRS"), strings.Contains(token, "DATA"), strings.Contains(token, "INTERNET"), strings.Contains(token, "PACKET"):
		return "PACKET_DATA"
	case strings.Contains(token, "VOICE"), strings.Contains(token, "VOLTE"), strings.Contains(token, "CALL"):
		return "VOICE"
	default:
		return "OTHER_OR_UNSPECIFIED"
	}
}

func cdrCounterpartyForTarget(observation multiCDRObservation, targetKey string) (string, string, bool) {
	callerKey := canonicalPhoneKey(defaultString(observation.Caller, observation.MSISDN))
	calleeKey := canonicalPhoneKey(observation.Callee)
	var counterparty, direction string
	switch {
	case callerKey == targetKey && calleeKey != targetKey:
		counterparty, direction = calleeKey, "OUTGOING"
	case calleeKey == targetKey && callerKey != targetKey:
		counterparty, direction = callerKey, "INCOMING"
	default:
		return "", "", false
	}
	if len(counterparty) < 8 || len(counterparty) > 19 || counterparty == targetKey {
		return "", "", false
	}
	return counterparty, direction, true
}

func multiCDRCitation(observation multiCDRObservation, sourceID string) map[string]any {
	return map[string]any{
		"source_id": sourceID, "source_file": observation.SourceFile,
		"evidence_id": observation.EvidenceID, "version_id": observation.VersionID,
		"row_number": observation.RowNumber, "row_hash": observation.RowHash,
		"observation_id": observation.ObservationID,
	}
}

func multiCDRContactSets(sourceOrder []string, contacts map[string]map[string]*multiCDRContactAggregate) ([]string, map[string][]string) {
	frequency := map[string]int{}
	for _, sourceID := range sourceOrder {
		for contact := range contacts[sourceID] {
			frequency[contact]++
		}
	}
	common := []string{}
	unique := map[string][]string{}
	for contact, count := range frequency {
		if count == len(sourceOrder) {
			common = append(common, contact)
		}
	}
	for _, sourceID := range sourceOrder {
		unique[sourceID] = []string{}
		for contact := range contacts[sourceID] {
			if frequency[contact] == 1 {
				unique[sourceID] = append(unique[sourceID], contact)
			}
		}
		sort.Strings(unique[sourceID])
	}
	sort.Strings(common)
	return common, unique
}

func multiCDRContactRows(sourceOrder []string, contacts map[string]map[string]*multiCDRContactAggregate, limit int) []map[string]any {
	rows := []map[string]any{}
	for _, sourceID := range sourceOrder {
		for _, aggregate := range contacts[sourceID] {
			directionClass := "one_way_outgoing"
			if aggregate.IncomingCnt > 0 && aggregate.OutgoingCnt > 0 {
				directionClass = "reciprocal_observation"
			} else if aggregate.IncomingCnt > 0 {
				directionClass = "one_way_incoming"
			}
			rows = append(rows, map[string]any{
				"source_id": sourceID, "counterparty": aggregate.Contact,
				"interaction_count": aggregate.InteractionCnt, "incoming_count": aggregate.IncomingCnt,
				"outgoing_count": aggregate.OutgoingCnt, "direction_class": directionClass,
				"duration_sum_seconds": aggregate.DurationSum, "duration_value_count": aggregate.DurationCnt,
				"missing_duration_count": aggregate.InteractionCnt - aggregate.DurationCnt,
				"first_seen":             aggregate.FirstSeen.UTC().Format(time.RFC3339Nano),
				"last_seen":              aggregate.LastSeen.UTC().Format(time.RFC3339Nano),
				"citations":              aggregate.Citations,
			})
		}
	}
	sort.Slice(rows, func(i, j int) bool {
		left, right := valueAsInt(rows[i]["interaction_count"]), valueAsInt(rows[j]["interaction_count"])
		if left != right {
			return left > right
		}
		if rows[i]["source_id"] != rows[j]["source_id"] {
			return stringValueAny(rows[i]["source_id"]) < stringValueAny(rows[j]["source_id"])
		}
		return stringValueAny(rows[i]["counterparty"]) < stringValueAny(rows[j]["counterparty"])
	})
	if len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func multiCDRSharedIdentifierRows(kind string, groups map[string]*multiCDRIdentifierAggregate, limit int) []map[string]any {
	rows := []map[string]any{}
	for _, group := range groups {
		if len(group.Sources) < 2 {
			continue
		}
		rows = append(rows, map[string]any{
			"entity_type": kind, "identifier": group.Identifier,
			"relationship": "observed_in_multiple_sources", "relationship_strength": "observed_association",
			"source_ids": sortedSetValues(group.Sources), "source_count": len(group.Sources),
			"citations": group.Citations,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		left, right := valueAsInt(rows[i]["source_count"]), valueAsInt(rows[j]["source_count"])
		if left != right {
			return left > right
		}
		return stringValueAny(rows[i]["identifier"]) < stringValueAny(rows[j]["identifier"])
	})
	if len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func multiCDRExactTemporalOverlaps(events []acceptedMultiCDREvent, limit int) []map[string]any {
	rows := []map[string]any{}
	for left := 0; left < len(events); left++ {
		for right := left + 1; right < len(events); right++ {
			if events[left].sourceID == events[right].sourceID || !events[left].observation.StartedAt.Equal(events[right].observation.StartedAt) {
				continue
			}
			rows = append(rows, map[string]any{
				"relationship": "temporal_overlap", "relationship_strength": "exact_normalized_instant",
				"observed_at":     events[left].observation.StartedAt.UTC().Format(time.RFC3339Nano),
				"source_ids":      []string{events[left].sourceID, events[right].sourceID},
				"observation_ids": []string{events[left].observation.ObservationID, events[right].observation.ObservationID},
				"citations":       []map[string]any{multiCDRCitation(events[left].observation, events[left].sourceID), multiCDRCitation(events[right].observation, events[right].sourceID)},
			})
			if len(rows) >= limit {
				return rows
			}
		}
	}
	return rows
}

func multiCDRDuplicateAndConflictRows(events []acceptedMultiCDREvent, limit int) ([]map[string]any, []map[string]any) {
	byBase := map[string][]int{}
	byExact := map[string][]int{}
	for index, event := range events {
		byBase[event.baseKey] = append(byBase[event.baseKey], index)
		byExact[event.exactKey] = append(byExact[event.exactKey], index)
	}
	duplicates := []map[string]any{}
	for exactKey, indexes := range byExact {
		sources := map[string]struct{}{}
		observationIDs := make([]string, 0, len(indexes))
		citations := make([]map[string]any, 0, len(indexes))
		for _, index := range indexes {
			sources[events[index].sourceID] = struct{}{}
			observationIDs = append(observationIDs, events[index].observation.ObservationID)
			citations = append(citations, multiCDRCitation(events[index].observation, events[index].sourceID))
		}
		if len(sources) < 2 {
			continue
		}
		duplicates = append(duplicates, map[string]any{
			"relationship": "cross_source_duplicate_candidate", "relationship_strength": "exact_event_signature",
			"event_signature": exactKey, "source_ids": sortedSetValues(sources), "support_count": len(indexes),
			"observation_ids": observationIDs, "citations": citations,
		})
	}
	conflicts := []map[string]any{}
	for baseKey, indexes := range byBase {
		sources := map[string]struct{}{}
		exact := map[string]struct{}{}
		observationIDs := make([]string, 0, len(indexes))
		citations := make([]map[string]any, 0, len(indexes))
		for _, index := range indexes {
			sources[events[index].sourceID] = struct{}{}
			exact[events[index].exactKey] = struct{}{}
			observationIDs = append(observationIDs, events[index].observation.ObservationID)
			citations = append(citations, multiCDRCitation(events[index].observation, events[index].sourceID))
		}
		if len(sources) < 2 || len(exact) < 2 {
			continue
		}
		conflicts = append(conflicts, map[string]any{
			"relationship": "conflict", "relationship_strength": "source_disagreement",
			"event_key": baseKey, "source_ids": sortedSetValues(sources), "variant_count": len(exact),
			"observation_ids": observationIDs, "citations": citations,
			"limitation": "NexusAI preserves every source observation and does not decide which source is correct.",
		})
	}
	sort.Slice(duplicates, func(i, j int) bool {
		return stringValueAny(duplicates[i]["event_signature"]) < stringValueAny(duplicates[j]["event_signature"])
	})
	sort.Slice(conflicts, func(i, j int) bool {
		return stringValueAny(conflicts[i]["event_key"]) < stringValueAny(conflicts[j]["event_key"])
	})
	if len(duplicates) > limit {
		duplicates = duplicates[:limit]
	}
	if len(conflicts) > limit {
		conflicts = conflicts[:limit]
	}
	return duplicates, conflicts
}

func multiCDRRelationshipRows(target string, common []string, unique map[string][]string, shared map[string]any, temporal, conflicts []map[string]any, limit int) []map[string]any {
	rows := []map[string]any{}
	for _, contact := range common {
		rows = append(rows, map[string]any{"from_type": "phone", "from_value": target, "to_type": "phone", "to_value": contact, "relationship": "shared_contact", "relationship_strength": "observed_association"})
	}
	for sourceID, contacts := range unique {
		for _, contact := range contacts {
			rows = append(rows, map[string]any{"from_type": "phone", "from_value": target, "to_type": "phone", "to_value": contact, "relationship": "observed_in_source", "relationship_strength": "single_source_observation", "source_ids": []string{sourceID}})
		}
	}
	for _, kind := range []string{"imei", "imsi", "tower"} {
		items, _ := shared[kind].([]map[string]any)
		for _, item := range items {
			rows = append(rows, map[string]any{"from_type": "phone", "from_value": target, "to_type": kind, "to_value": item["identifier"], "relationship": "shared_" + kind + "_observation", "relationship_strength": "observed_association", "source_ids": item["source_ids"]})
		}
	}
	for _, row := range temporal {
		rows = append(rows, row)
	}
	for _, row := range conflicts {
		rows = append(rows, row)
	}
	if len(rows) > limit {
		return rows[:limit]
	}
	return rows
}

func multiCDRGraph(target string, relationships []map[string]any, limit int) map[string]any {
	nodes := map[string]map[string]any{"phone\x00" + target: {"id": "phone:" + target, "entity_type": "phone", "value": target}}
	edges := []map[string]any{}
	for _, relationship := range relationships {
		fromType, fromValue := stringValueAny(relationship["from_type"]), stringValueAny(relationship["from_value"])
		toType, toValue := stringValueAny(relationship["to_type"]), stringValueAny(relationship["to_value"])
		if fromType == "" || fromValue == "" || toType == "" || toValue == "" {
			continue
		}
		nodes[fromType+"\x00"+fromValue] = map[string]any{"id": fromType + ":" + fromValue, "entity_type": fromType, "value": fromValue}
		nodes[toType+"\x00"+toValue] = map[string]any{"id": toType + ":" + toValue, "entity_type": toType, "value": toValue}
		edges = append(edges, map[string]any{"from": fromType + ":" + fromValue, "to": toType + ":" + toValue, "relationship": relationship["relationship"], "relationship_strength": relationship["relationship_strength"], "source_ids": relationship["source_ids"]})
		if len(edges) >= limit {
			break
		}
	}
	nodeRows := make([]map[string]any, 0, len(nodes))
	for _, node := range nodes {
		nodeRows = append(nodeRows, node)
	}
	sort.Slice(nodeRows, func(i, j int) bool { return stringValueAny(nodeRows[i]["id"]) < stringValueAny(nodeRows[j]["id"]) })
	return map[string]any{"nodes": nodeRows, "edges": edges, "semantics": "Only returned typed relationships are rendered; graph prominence is not analytical strength."}
}

func buildMultiCDREnterprisePayload(req hybridQueryRequest, resp hybridQueryResponse) map[string]any {
	comparisonRows := typedRows(resp.Records["contact_comparison"])
	relationshipRows := typedRows(resp.Records["relationships"])
	limitations := stringSliceAny(resp.Records["limitations"])
	provenance := multiCDRProvenance(req.CollectionID, resp.Records)
	dataGrid := enterpriseDataGrid(comparisonRows)
	dataGrid["title"] = "Per-source contact comparison"
	dataGrid["count_label"] = "source/contact groups"
	status := "answered_with_limitations"
	executiveAnswer := fmt.Sprintf("Compared %d exact CDR sources for the selected target and preserved %d matching source events without merging source identity.", valueAsInt(resp.Records["source_count"]), valueAsInt(resp.Records["matched_event_count"]))
	if len(comparisonRows) == 0 {
		status = "no_results"
		executiveAnswer = "No qualifying phone-counterparty events were found for the selected target in the exact source set; no relationship was inferred."
	}
	return map[string]any{
		"status": status, "executive_answer": executiveAnswer, "summary": executiveAnswer,
		"metrics": []map[string]any{
			{"label": "Selected Sources", "value": resp.Records["source_count"], "source": "source_set"},
			{"label": "Matching Events", "value": resp.Records["matched_event_count"], "source": "records_sql"},
			{"label": "Common Contacts", "value": len(stringSliceAny(resp.Records["common_contacts"])), "source": "deterministic_comparison"},
			{"label": "Conflicts", "value": len(typedRows(resp.Records["conflicts"])), "source": "deterministic_comparison"},
			{"label": "Provenance Items", "value": len(provenance), "source": "records_sql"},
		},
		"data_grid": dataGrid,
		"comparison": map[string]any{
			"target": req.Target, "source_set": resp.Records["source_set"],
			"common": resp.Records["common_contacts"], "unique_by_source": resp.Records["unique_contacts"],
			"frequency_and_duration": comparisonRows,
		},
		"relationship_table": relationshipRows,
		"visualizations": []map[string]any{
			{"type": "relationship_graph", "title": "Evidence-backed source relationships", "spec": resp.Records["graph"]},
			{"type": "timeline", "title": "Exact-instant cross-source overlaps", "spec": resp.Records["timeline"]},
		},
		"conflicts": resp.Records["conflicts"], "duplicate_candidates": resp.Records["duplicate_candidates"],
		"provenance": provenance, "limitations": limitations,
		"recommended_actions": []map[string]any{{"label": "Review contributing rows", "query": "show source records", "template": "source_records", "reason": "Verify duplicate candidates and conflicts against the cited source observations."}},
		"operation":           map[string]any{"template": "multi_cdr_comparison", "operation_id": "cdr.multi_source_comparison", "family_id": "communications_cdr", "source_access": "records", "output_description": "Source-aware comparison with typed relationship, graph, timeline, conflict, and provenance views."},
		"coverage":            map[string]any{"tenant_id": req.TenantID, "collection_id": req.CollectionID, "template": resp.Template, "route": resp.Route, "target": req.Target, "date_from": req.DateFrom, "date_to": req.DateTo, "source_set": resp.Records["source_set"]},
		"telemetry":           resp.Telemetry,
		"synthesis":           map[string]any{"claims": enterpriseClaims(resp), "policy": "Every displayed relationship is a deterministic typed observation; no graph edge implies identity, ownership, guilt, intent, or physical presence."},
	}
}

func multiCDRProvenance(collectionID string, records map[string]any) []map[string]any {
	out := []map[string]any{}
	seen := map[string]struct{}{}
	var visit func(any)
	visit = func(value any) {
		if len(out) >= 200 || value == nil {
			return
		}
		switch typed := value.(type) {
		case []map[string]any:
			for _, item := range typed {
				visit(item)
			}
		case []any:
			for _, item := range typed {
				visit(item)
			}
		case map[string]any:
			if evidenceID := stringValueAny(typed["evidence_id"]); evidenceID != "" {
				key := evidenceID + "\x00" + stringValueAny(typed["version_id"]) + "\x00" + stringValueAny(typed["row_hash"])
				if _, exists := seen[key]; !exists {
					seen[key] = struct{}{}
					out = append(out, map[string]any{"source": "records_sql", "collection_id": collectionID, "source_id": typed["source_id"], "source_file": typed["source_file"], "evidence_id": evidenceID, "version_id": typed["version_id"], "row_number": typed["row_number"], "row_hash": typed["row_hash"], "observation_id": typed["observation_id"]})
				}
			}
			for _, nested := range typed {
				visit(nested)
			}
		}
	}
	for _, key := range []string{"contact_comparison", "shared_imei", "shared_imsi", "shared_tower", "temporal_overlaps", "duplicate_candidates", "conflicts"} {
		visit(records[key])
	}
	return out
}

type typedStructuredObservation struct {
	ObservationID string
	Family        string
	SourceID      string
	EvidenceID    string
	VersionID     string
	EntityType    string
	EntityValue   string
	ObservedAt    time.Time
	ValidFrom     *time.Time
	ValidTo       *time.Time
}

func buildTypedCrossFamilyRelationships(observations []typedStructuredObservation) []map[string]any {
	relations := []map[string]any{}
	for left := 0; left < len(observations); left++ {
		for right := left + 1; right < len(observations); right++ {
			first, second := observations[left], observations[right]
			if first.Family == second.Family && first.SourceID == second.SourceID {
				continue
			}
			if first.EntityType != second.EntityType || typedEntityKey(first.EntityType, first.EntityValue) == "" || typedEntityKey(first.EntityType, first.EntityValue) != typedEntityKey(second.EntityType, second.EntityValue) {
				continue
			}
			relationship, strength, ok := supportedTypedRelationship(first, second)
			if !ok {
				continue
			}
			relations = append(relations, map[string]any{
				"left_observation_id": first.ObservationID, "right_observation_id": second.ObservationID,
				"entity_type": first.EntityType, "entity_value": first.EntityValue,
				"relationship": relationship, "relationship_strength": strength,
				"source_ids":   []string{first.SourceID, second.SourceID},
				"evidence_ids": []string{first.EvidenceID, second.EvidenceID},
				"version_ids":  []string{first.VersionID, second.VersionID},
				"limitations":  []string{"Identifier equality and time validity do not prove identity, ownership, physical presence, causation, or intent."},
			})
		}
	}
	sort.Slice(relations, func(i, j int) bool {
		left := stringValueAny(relations[i]["left_observation_id"]) + "|" + stringValueAny(relations[i]["right_observation_id"])
		right := stringValueAny(relations[j]["left_observation_id"]) + "|" + stringValueAny(relations[j]["right_observation_id"])
		return left < right
	})
	return relations
}

func typedEntityKey(entityType, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	switch entityType {
	case "phone":
		key := canonicalPhoneKey(value)
		if len(key) < 8 {
			return ""
		}
		return key
	case "imei", "imsi":
		key := nonDigitPattern.ReplaceAllString(value, "")
		if len(key) < 8 {
			return ""
		}
		return key
	case "plate":
		return strings.ToUpper(regexpNonAlphanumeric.ReplaceAllString(value, ""))
	case "site", "subscriber_reference":
		return strings.ToUpper(value)
	default:
		return ""
	}
}

var regexpNonAlphanumeric = regexp.MustCompile(`[^[:alnum:]]`)

func supportedTypedRelationship(first, second typedStructuredObservation) (string, string, bool) {
	families := map[string]bool{first.Family: true, second.Family: true}
	event, reference := first, second
	if first.Family == "subscriber_identity" || first.Family == "tower_location" {
		event, reference = second, first
	}
	switch {
	case families["communications_cdr"] && families["subscriber_identity"]:
		if !containsString([]string{"phone", "imsi", "imei"}, first.EntityType) || !observationWithinValidity(event.ObservedAt, reference) {
			return "", "", false
		}
		return "cdr_subscriber_identifier_match", "source_declared_relationship", true
	case families["communications_cdr"] && families["tower_location"]:
		if first.EntityType != "site" || !observationWithinValidity(event.ObservedAt, reference) {
			return "", "", false
		}
		return "cdr_tower_reference_match", "exact_match", true
	case families["network_ipdr"] && families["subscriber_identity"]:
		if !containsString([]string{"phone", "imsi", "subscriber_reference"}, first.EntityType) || !observationWithinValidity(event.ObservedAt, reference) {
			return "", "", false
		}
		return "ipdr_subscriber_identifier_match", "source_declared_relationship", true
	case families["communications_cdr"] && families["network_ipdr"]:
		if !containsString([]string{"phone", "imsi"}, first.EntityType) || first.ObservedAt.IsZero() || !first.ObservedAt.Equal(second.ObservedAt) {
			return "", "", false
		}
		return "cdr_ipdr_temporal_identifier_overlap", "temporal_overlap", true
	case first.Family == "anpr_vehicles" && second.Family == "anpr_vehicles" && first.EntityType == "plate":
		return "plate_observed_in_multiple_sources", "exact_match", true
	default:
		return "", "", false
	}
}

func observationWithinValidity(observedAt time.Time, reference typedStructuredObservation) bool {
	if observedAt.IsZero() || (reference.ValidFrom == nil && reference.ValidTo == nil) {
		return false
	}
	if reference.ValidFrom != nil && observedAt.Before(*reference.ValidFrom) {
		return false
	}
	return reference.ValidTo == nil || observedAt.Before(*reference.ValidTo)
}
