package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const caseGovernanceContractV1 = "forensics.case-governance/v1"

// collectionManifestV1 is intentionally read-only. Counts from systems owned by
// LocalAI (KB entries, vectors, agents, and collection sources) are left with an
// explicit external status here and are augmented by the authenticated LocalAI
// API. This prevents the sidecar from manufacturing cross-system totals.
type collectionManifestV1 struct {
	ContractVersion string           `json:"contract_version"`
	GeneratedAt     time.Time        `json:"generated_at"`
	TenantID        string           `json:"tenant_id"`
	CaseID          string           `json:"case_id"`
	CollectionID    string           `json:"collection_id"`
	ReadOnly        bool             `json:"read_only"`
	DeletionCount   int              `json:"deletion_count"`
	Resources       []manifestItemV1 `json:"resources"`
	JobStatuses     []map[string]any `json:"job_statuses"`
	RecordFamilies  []map[string]any `json:"record_families"`
	Warnings        []string         `json:"warnings"`
}

type manifestItemV1 struct {
	Resource string `json:"resource"`
	Count    *int64 `json:"count"`
	Status   string `json:"status"`
	Source   string `json:"source"`
	Note     string `json:"note,omitempty"`
}

func forensicCaseManifestHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		if db == nil {
			writeError(w, http.StatusServiceUnavailable, errors.New("FORENSIC_DATABASE_URL is not configured"))
			return
		}
		caseID := strings.TrimSpace(r.PathValue("case_id"))
		if caseID == "" {
			writeError(w, http.StatusBadRequest, errors.New("case_id is required"))
			return
		}
		scope, err := bindForensicScope(r, r.URL.Query().Get("tenant_id"), caseID, caseID, "")
		if err != nil {
			writeScopeError(w, err)
			return
		}
		manifest, err := loadCollectionManifestV1(r.Context(), db, scope)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, manifest)
	}
}

func loadCollectionManifestV1(ctx context.Context, db *pgxpool.Pool, scope forensicScope) (collectionManifestV1, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return collectionManifestV1{}, fmt.Errorf("begin case manifest query: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", scope.TenantID); err != nil {
		return collectionManifestV1{}, fmt.Errorf("set tenant context: %w", err)
	}

	rows, err := rowsFromQuery(ctx, tx, `
SELECT
  (SELECT count(*) FROM forensic.kb_collection_assets WHERE tenant_id=$1 AND collection_id=$2) AS kb_assets,
  (SELECT count(*) FROM forensic.evidence_items WHERE tenant_id=$1 AND collection_id=$2) AS evidence_items,
  (SELECT count(*) FROM forensic.records WHERE tenant_id=$1 AND collection_id=$2) AS canonical_records,
  (SELECT count(*) FROM forensic.cdr_records WHERE tenant_id=$1 AND collection_id=$2) AS cdr_records,
  (SELECT count(*) FROM forensic.generic_records WHERE tenant_id=$1 AND collection_id=$2) AS generic_records,
  (SELECT count(*) FROM forensic.record_entities WHERE tenant_id=$1 AND collection_id=$2) AS record_entities,
  (SELECT count(*) FROM forensic.records_ingest_jobs WHERE tenant_id=$1 AND collection_id=$2) AS ingest_jobs,
  (SELECT count(*) FROM forensic.records_audit_log WHERE tenant_id=$1 AND collection_id=$2) AS audit_events`, scope.TenantID, scope.CollectionID)
	if err != nil {
		return collectionManifestV1{}, err
	}
	counts := firstRow(rows)
	jobs, err := rowsFromQuery(ctx, tx, `
SELECT status::text AS status, count(*) AS count,
       min(queued_at) AS first_queued_at,
       max(coalesce(completed_at, started_at, queued_at)) AS last_activity_at
FROM forensic.records_ingest_jobs
WHERE tenant_id=$1 AND collection_id=$2
GROUP BY status
ORDER BY status`, scope.TenantID, scope.CollectionID)
	if err != nil {
		return collectionManifestV1{}, err
	}
	families, err := rowsFromQuery(ctx, tx, `
SELECT record_type::text AS record_type, count(*) AS jobs,
       coalesce(sum(total_rows),0) AS total_rows,
       coalesce(sum(accepted_rows),0) AS accepted_rows,
       coalesce(sum(duplicate_rows),0) AS duplicate_rows,
       coalesce(sum(rejected_rows),0) AS rejected_rows
FROM forensic.records_ingest_jobs
WHERE tenant_id=$1 AND collection_id=$2
GROUP BY record_type
ORDER BY record_type`, scope.TenantID, scope.CollectionID)
	if err != nil {
		return collectionManifestV1{}, err
	}

	count := func(key string) *int64 {
		value := valueAsInt(counts[key])
		return &value
	}
	resources := []manifestItemV1{
		{Resource: "kb_entries", Status: "external", Source: "localai_kb", Note: "Augmented by the LocalAI case API."},
		{Resource: "vectors", Status: "external", Source: "localai_kb", Note: "Vector cardinality is not exposed by the sidecar."},
		{Resource: "kb_assets", Count: count("kb_assets"), Status: "counted", Source: "forensic.kb_collection_assets"},
		{Resource: "evidence_items", Count: count("evidence_items"), Status: "counted", Source: "forensic.evidence_items"},
		{Resource: "canonical_records", Count: count("canonical_records"), Status: "counted", Source: "forensic.records"},
		{Resource: "legacy_cdr_records", Count: count("cdr_records"), Status: "counted", Source: "forensic.cdr_records"},
		{Resource: "legacy_generic_records", Count: count("generic_records"), Status: "counted", Source: "forensic.generic_records"},
		{Resource: "record_entities", Count: count("record_entities"), Status: "counted", Source: "forensic.record_entities"},
		{Resource: "ingest_jobs", Count: count("ingest_jobs"), Status: "counted", Source: "forensic.records_ingest_jobs"},
		{Resource: "agents", Status: "external", Source: "localai_agent_pool", Note: "Augmented by the LocalAI case API."},
		{Resource: "reports", Status: "not_persisted", Source: "forensic_reports", Note: "Current reports are generated synchronously and are not stored as immutable report records."},
		{Resource: "sources", Status: "external", Source: "localai_kb", Note: "Augmented by the LocalAI case API."},
		{Resource: "audit_events", Count: count("audit_events"), Status: "counted", Source: "forensic.records_audit_log"},
	}
	return collectionManifestV1{
		ContractVersion: caseGovernanceContractV1,
		GeneratedAt:     time.Now().UTC(),
		TenantID:        scope.TenantID,
		CaseID:          scope.CaseID,
		CollectionID:    scope.CollectionID,
		ReadOnly:        true,
		DeletionCount:   0,
		Resources:       resources,
		JobStatuses:     jobs,
		RecordFamilies:  families,
		Warnings: []string{
			"This manifest performs no merge, archive, soft-delete, or permanent deletion.",
			"External resource counts remain explicitly unresolved until the authenticated LocalAI API augments them.",
		},
	}, nil
}

// forensicCaseQueryV1Handler is a bounded compatibility adapter: the accepted
// legacy hybrid handler remains byte-compatible, while new clients receive the
// typed enterprise-response/v1 contract. The URL case is authoritative and a
// conflicting body collection is rejected before any query runs.
func forensicCaseQueryV1Handler(cfg config, db *pgxpool.Pool) http.HandlerFunc {
	legacy := hybridQueryHandler(cfg, db)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
			return
		}
		caseID := strings.TrimSpace(r.PathValue("case_id"))
		if caseID == "" {
			writeError(w, http.StatusBadRequest, errors.New("case_id is required"))
			return
		}
		var body map[string]any
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("decode v1 query request: %w", err))
			return
		}
		body, earlyResponse, err := normalizeV1QueryPlan(caseID, requestIDFromHTTP(r), body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		if earlyResponse != nil {
			writeJSON(w, http.StatusOK, earlyResponse)
			return
		}
		for _, key := range []string{"case_id", "collection_id"} {
			if supplied := strings.TrimSpace(stringValueAny(body[key])); supplied != "" && supplied != caseID {
				writeError(w, http.StatusConflict, fmt.Errorf("%s %q does not match URL case %q", key, supplied, caseID))
				return
			}
		}
		body["case_id"] = caseID
		body["collection_id"] = caseID
		scope, err := bindForensicScope(
			r,
			stringValueAny(body["tenant_id"]),
			caseID,
			caseID,
			stringValueAny(body["user_id"]),
		)
		if err != nil {
			writeScopeError(w, err)
			return
		}
		body["tenant_id"] = scope.TenantID
		body["user_id"] = scope.SubjectID
		legacyPayload, err := json.Marshal(body)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		clone := r.Clone(r.Context())
		clone.Body = io.NopCloser(bytes.NewReader(legacyPayload))
		clone.ContentLength = int64(len(legacyPayload))
		recorder := httptest.NewRecorder()
		legacy.ServeHTTP(recorder, clone)
		for key, values := range recorder.Header() {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		if recorder.Code != http.StatusOK {
			w.WriteHeader(recorder.Code)
			_, _ = w.Write(recorder.Body.Bytes())
			return
		}
		var response hybridQueryResponse
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("decode legacy query result: %w", err))
			return
		}
		requestID := recorder.Header().Get("X-Request-ID")
		writeJSON(w, http.StatusOK, legacyHybridToEnterpriseV1(caseID, requestID, body, response))
	}
}

func normalizeV1QueryPlan(caseID, requestID string, body map[string]any) (map[string]any, *EnterpriseResponseV1, error) {
	plan := body
	if nested, ok := body["query_plan"].(map[string]any); ok {
		plan = nested
	}
	if stringValueAny(plan["contract_version"]) != "forensics.query-plan/v1" {
		return body, nil, nil
	}
	if field := prohibitedExecutionField(plan); field != "" {
		return nil, nil, fmt.Errorf("query plan field %q is prohibited; plans may reference only registered operation IDs and typed parameters", field)
	}
	for _, key := range []string{"case_id", "collection_id"} {
		if supplied := strings.TrimSpace(stringValueAny(plan[key])); supplied != "" && supplied != caseID {
			return nil, nil, fmt.Errorf("query plan %s %q does not match URL case %q", key, supplied, caseID)
		}
	}
	operationID := strings.TrimSpace(stringValueAny(plan["intent"]))
	template := queryTemplateNameByOperationID(operationID)
	if template == "" {
		return body, &EnterpriseResponseV1{
			ContractVersion:       "forensics.enterprise-response/v1",
			RequestID:             requestID,
			CaseID:                caseID,
			CollectionID:          caseID,
			Status:                "unsupported",
			ResultState:           EnterpriseResultStateUnavailable,
			OperationID:           operationID,
			ExecutiveAnswer:       "The requested typed operation is not available through the bounded query endpoint.",
			DeterministicFindings: []FindingV1{}, SemanticEvidence: []CitationV1{}, InferredRelationships: []InferredRelationshipV1{},
			UnsupportedOperations: []UnsupportedOperationV1{{OperationID: operationID, Reason: "Operation is not a registered query operation.", RequiredCapability: "registered deterministic query operation"}},
			Limitations:           []string{"Detection and normalization operations run through evidence ingestion, not the case query endpoint."},
			NextActions:           []NextActionV1{}, Tables: []TableResultV1{}, Visualizations: []VisualizationV1{},
			ExecutionTrace: ExecutionTraceV1{Route: []string{"capability_guard"}, ToolIDs: []string{}, AdapterIDs: []string{}, ModelRoles: []string{}, Parameters: map[string]any{"intent": operationID}, LatencyMS: map[string]int64{}},
		}, nil
	}
	if err := validateTypedQueryPlanV1(operationID, template, plan); err != nil {
		return nil, nil, err
	}
	if clarifications := mapsFromAny(plan["clarifications"]); len(clarifications) > 0 {
		questions := make([]string, 0, len(clarifications))
		for _, clarification := range clarifications {
			if question := strings.TrimSpace(stringValueAny(clarification["question"])); question != "" {
				questions = append(questions, question)
			}
		}
		return body, &EnterpriseResponseV1{
			ContractVersion:       "forensics.enterprise-response/v1",
			RequestID:             requestID,
			CaseID:                caseID,
			CollectionID:          caseID,
			Status:                "needs_input",
			ResultState:           EnterpriseResultStateInvalidRequest,
			OperationID:           operationID,
			ExecutiveAnswer:       "The typed query plan requires clarification before execution.",
			DeterministicFindings: []FindingV1{}, SemanticEvidence: []CitationV1{}, InferredRelationships: []InferredRelationshipV1{}, UnsupportedOperations: []UnsupportedOperationV1{},
			Limitations: questions, NextActions: []NextActionV1{}, Tables: []TableResultV1{}, Visualizations: []VisualizationV1{},
			ExecutionTrace: ExecutionTraceV1{Route: []string{"clarification"}, ToolIDs: []string{}, AdapterIDs: []string{}, ModelRoles: []string{}, Parameters: map[string]any{"intent": operationID}, LatencyMS: map[string]int64{}},
		}, nil
	}
	if operationID == "cdr.device_identity_changes" && len(mapsFromAny(plan["entities"])) == 0 {
		return body, &EnterpriseResponseV1{
			ContractVersion: "forensics.enterprise-response/v1", RequestID: requestID,
			CaseID: caseID, CollectionID: caseID, Status: "needs_input", ResultState: EnterpriseResultStateInvalidRequest, OperationID: operationID,
			ExecutiveAnswer:       "The device-identity change operation requires an originating subscriber or device target.",
			DeterministicFindings: []FindingV1{}, SemanticEvidence: []CitationV1{}, InferredRelationships: []InferredRelationshipV1{}, UnsupportedOperations: []UnsupportedOperationV1{},
			Limitations: []string{"Provide an MSISDN, originating number, IMEI, or IMSI. Dialed counterparties are not treated as the owner of the row's device identifiers."},
			NextActions: []NextActionV1{}, Tables: []TableResultV1{}, Visualizations: []VisualizationV1{},
			ExecutionTrace: ExecutionTraceV1{Route: []string{"clarification"}, ToolIDs: []string{}, AdapterIDs: []string{}, ModelRoles: []string{}, Parameters: map[string]any{"intent": operationID, "required_entity_types": []string{"msisdn", "originating_number", "imei", "imsi"}}, LatencyMS: map[string]int64{}},
		}, nil
	}
	if (operationID == "ipdr.subscriber_sessions" || operationID == "ipdr.concurrent_sessions" || operationID == "ipdr.timeline") && len(mapsFromAny(plan["entities"])) == 0 {
		return body, &EnterpriseResponseV1{
			ContractVersion: "forensics.enterprise-response/v1", RequestID: requestID,
			CaseID: caseID, CollectionID: caseID, Status: "needs_input", ResultState: EnterpriseResultStateInvalidRequest, OperationID: operationID,
			ExecutiveAnswer:       "The requested IPDR operation requires an explicit subscriber or network endpoint target.",
			DeterministicFindings: []FindingV1{}, SemanticEvidence: []CitationV1{}, InferredRelationships: []InferredRelationshipV1{}, UnsupportedOperations: []UnsupportedOperationV1{},
			Limitations: []string{"Provide an explicit subscriber identifier, source IP, destination IP, NAT IP, or normalized domain. NexusAI does not infer subscriber-to-IP ownership or DNS resolutions."},
			NextActions: []NextActionV1{}, Tables: []TableResultV1{}, Visualizations: []VisualizationV1{},
			ExecutionTrace: ExecutionTraceV1{Route: []string{"clarification"}, ToolIDs: []string{}, AdapterIDs: []string{}, ModelRoles: []string{}, Parameters: map[string]any{"intent": operationID, "required_entity_types": []string{"subscriber_identifier", "ip", "nat_ip", "domain"}}, LatencyMS: map[string]int64{}},
		}, nil
	}
	if (operationID == "anpr.camera_sequence" || operationID == "anpr.co_travel" || operationID == "anpr.route_timing" || operationID == "anpr.plate_variants" || operationID == "anpr.timeline") && len(mapsFromAny(plan["entities"])) == 0 {
		return body, &EnterpriseResponseV1{
			ContractVersion: "forensics.enterprise-response/v1", RequestID: requestID,
			CaseID: caseID, CollectionID: caseID, Status: "needs_input", ResultState: EnterpriseResultStateInvalidRequest, OperationID: operationID,
			ExecutiveAnswer:       "The requested ANPR operation requires an explicit plate target.",
			DeterministicFindings: []FindingV1{}, SemanticEvidence: []CitationV1{}, InferredRelationships: []InferredRelationshipV1{}, UnsupportedOperations: []UnsupportedOperationV1{},
			Limitations: []string{"Provide an exact raw plate value or normalized plate search key. NexusAI does not infer vehicle ownership, occupants, association, or a road route."},
			NextActions: []NextActionV1{}, Tables: []TableResultV1{}, Visualizations: []VisualizationV1{},
			ExecutionTrace: ExecutionTraceV1{Route: []string{"clarification"}, ToolIDs: []string{}, AdapterIDs: []string{}, ModelRoles: []string{}, Parameters: map[string]any{"intent": operationID, "required_entity_types": []string{"plate", "plate_search_key"}}, LatencyMS: map[string]int64{}},
		}, nil
	}
	if strings.HasPrefix(operationID, "subscriber.") && !subscriberTypedEntitiesAllowed(mapsFromAny(plan["entities"])) {
		return body, &EnterpriseResponseV1{
			ContractVersion: "forensics.enterprise-response/v1", RequestID: requestID,
			CaseID: caseID, CollectionID: caseID, Status: "needs_input", ResultState: EnterpriseResultStateInvalidRequest, OperationID: operationID,
			ExecutiveAnswer:       "The supplied subscriber target type is not allowed for this operation.",
			DeterministicFindings: []FindingV1{}, SemanticEvidence: []CitationV1{}, InferredRelationships: []InferredRelationshipV1{}, UnsupportedOperations: []UnsupportedOperationV1{},
			Limitations: []string{"Use an exact MSISDN, subscriber/service reference, IMSI, ICCID, or IMEI. Full CNIC and subscriber names are not accepted as default query targets or copied into response/export fields."},
			NextActions: []NextActionV1{}, Tables: []TableResultV1{}, Visualizations: []VisualizationV1{},
			ExecutionTrace: ExecutionTraceV1{Route: []string{"clarification"}, ToolIDs: []string{}, AdapterIDs: []string{}, ModelRoles: []string{}, Parameters: map[string]any{"intent": operationID, "required_entity_types": []string{"msisdn", "subscriber_reference", "service_reference", "imsi", "iccid", "imei"}}, LatencyMS: map[string]int64{}},
		}, nil
	}
	if (operationID == "subscriber.identity_lookup" || operationID == "subscriber.validity_timeline" || operationID == "subscriber.device_links") && len(mapsFromAny(plan["entities"])) == 0 {
		return body, &EnterpriseResponseV1{
			ContractVersion: "forensics.enterprise-response/v1", RequestID: requestID,
			CaseID: caseID, CollectionID: caseID, Status: "needs_input", ResultState: EnterpriseResultStateInvalidRequest, OperationID: operationID,
			ExecutiveAnswer:       "The requested subscriber operation requires an exact subscriber identifier.",
			DeterministicFindings: []FindingV1{}, SemanticEvidence: []CitationV1{}, InferredRelationships: []InferredRelationshipV1{}, UnsupportedOperations: []UnsupportedOperationV1{},
			Limitations: []string{"Provide an exact MSISDN, subscriber/service reference, IMSI, ICCID, or IMEI. Full CNIC and subscriber names are not accepted as default query targets, and an identifier match does not prove identity or ownership."},
			NextActions: []NextActionV1{}, Tables: []TableResultV1{}, Visualizations: []VisualizationV1{},
			ExecutionTrace: ExecutionTraceV1{Route: []string{"clarification"}, ToolIDs: []string{}, AdapterIDs: []string{}, ModelRoles: []string{}, Parameters: map[string]any{"intent": operationID, "required_entity_types": []string{"msisdn", "subscriber_reference", "service_reference", "imsi", "iccid", "imei"}}, LatencyMS: map[string]int64{}},
		}, nil
	}
	if (operationID == "tower.site_lookup" || operationID == "tower.reference_timeline" || operationID == "tower.cdr_join") && len(mapsFromAny(plan["entities"])) == 0 {
		return body, &EnterpriseResponseV1{
			ContractVersion: "forensics.enterprise-response/v1", RequestID: requestID,
			CaseID: caseID, CollectionID: caseID, Status: "needs_input", ResultState: EnterpriseResultStateInvalidRequest, OperationID: operationID,
			ExecutiveAnswer:       "The requested tower/site operation requires an exact reference identifier.",
			DeterministicFindings: []FindingV1{}, SemanticEvidence: []CitationV1{}, InferredRelationships: []InferredRelationshipV1{}, UnsupportedOperations: []UnsupportedOperationV1{},
			Limitations: []string{"Provide an exact site, sector, LAC, TAC, CGI, ECGI, eNodeB, or gNodeB identifier. A reference match supplies location context only and does not prove RF coverage, handset position, or subscriber presence."},
			NextActions: []NextActionV1{}, Tables: []TableResultV1{}, Visualizations: []VisualizationV1{},
			ExecutionTrace: ExecutionTraceV1{Route: []string{"clarification"}, ToolIDs: []string{}, AdapterIDs: []string{}, ModelRoles: []string{}, Parameters: map[string]any{"intent": operationID, "required_entity_types": []string{"site", "sector", "lac", "tac", "cgi", "ecgi", "enodeb", "gnodeb"}}, LatencyMS: map[string]int64{}},
		}, nil
	}
	legacy := map[string]any{
		"tenant_id":     plan["tenant_id"],
		"case_id":       caseID,
		"collection_id": caseID,
		// Dots and underscores are valid plate separators. Remove them from the
		// generated prose so an operation ID cannot become an unintended target.
		"query":    "execute typed forensic operation " + strings.NewReplacer(".", " ", "_", " ").Replace(operationID),
		"template": template,
		"limit":    plan["limit"],
	}
	if families, ok := plan["families"].([]any); ok && len(families) > 0 {
		family := stringValueAny(families[0])
		if family == "communications_cdr" {
			legacy["record_type"] = "cdr"
		} else if family == "network_ipdr" {
			legacy["record_type"] = "ipdr"
		} else if family == "anpr_vehicles" || family == "geospatial" {
			legacy["record_type"] = "anpr"
		} else if family == "subscriber_identity" {
			legacy["record_type"] = "subscriber"
		} else if family == "tower_location" {
			legacy["record_type"] = "tower_location"
		} else if family == "generic_tabular" {
			legacy["record_type"] = "generic"
		} else if family == "financial_records" {
			legacy["record_type"] = "transaction"
		} else if family == "access_logs" {
			legacy["record_type"] = "access_log"
		} else if family == "documents_text" {
			legacy["record_type"] = "document"
		} else if family == "image_media" {
			legacy["record_type"] = "image"
		} else if family == "face_intelligence" {
			legacy["record_type"] = "image"
		} else if family == "audio_media" {
			legacy["record_type"] = "audio"
		} else if family == "video_media" {
			legacy["record_type"] = "video"
		}
	}
	entities := mapsFromAny(plan["entities"])
	targets := make([]string, 0, len(entities))
	for _, entity := range entities {
		if value := strings.TrimSpace(stringValueAny(entity["value"])); value != "" {
			if operationID == "video.anpr_grouped_timeline" || operationID == "video.timeline" || operationID == "face.candidate_observations" {
				switch strings.ToLower(strings.TrimSpace(stringValueAny(entity["type"]))) {
				case "evidence", "evidence_id", "video_evidence":
					legacy["evidence_id"] = value
				case "plate", "plate_search_key":
					if operationID == "video.anpr_grouped_timeline" {
						legacy["plate"] = value
					}
				}
			}
			targets = append(targets, value)
		}
	}
	if operationID == "video.anpr_grouped_timeline" || operationID == "video.timeline" || operationID == "face.candidate_observations" {
		if evidenceID := strings.TrimSpace(stringValueAny(legacy["evidence_id"])); evidenceID != "" {
			legacy["target"] = evidenceID
			targets = []string{evidenceID}
		}
		for _, filter := range mapsFromAny(plan["filters"]) {
			switch strings.ToLower(strings.TrimSpace(stringValueAny(filter["field"]))) {
			case "start_seconds", "source_start_seconds":
				legacy["start_seconds"] = filter["value"]
			case "end_seconds", "source_end_seconds":
				legacy["end_seconds"] = filter["value"]
			case "plate", "plate_search_key":
				if operationID == "video.anpr_grouped_timeline" {
					legacy["plate"] = filter["value"]
				}
			}
		}
	}
	if len(targets) > 0 {
		legacy["target"] = targets[0]
		legacy["targets"] = targets
	}
	if timeRange, ok := plan["time_range"].(map[string]any); ok {
		legacy["date_from"] = timeRange["from"]
		legacy["date_to"] = timeRange["to"]
	}
	if filters := mapsFromAny(plan["filters"]); len(filters) > 0 {
		switch template {
		case "canonical_records":
			payloadFilters := []map[string]any{}
			for _, filter := range filters {
				field := normalize(strings.TrimSpace(stringValueAny(filter["field"])))
				if field == "record_type" || field == "source_file" {
					legacy[field] = filter["value"]
				} else {
					payloadFilters = append(payloadFilters, filter)
				}
			}
			legacy["raw_payload_filters"] = payloadFilters
		case "generic_filter_records":
			legacy["raw_payload_filters"] = filters
		case "frequent_contacts":
			legacy["direction"] = canonicalEventDirection(stringValueAny(filters[0]["value"]))
		}
	}
	if sorts := mapsFromAny(plan["sort"]); len(sorts) > 0 {
		legacy["sort_by"] = sorts[0]["field"]
		legacy["sort_direction"] = sorts[0]["direction"]
	}
	if fields, supplied := plan["projection"]; supplied {
		legacy["projection"] = stringSliceAny(fields)
	}
	if group, supplied := plan["group"]; supplied {
		legacy["group"] = group
	}
	if compare, supplied := plan["compare"]; supplied {
		legacy["compare"] = compare
	}
	if sourceNative, supplied := plan["source_native"]; supplied {
		legacy["source_native"] = sourceNative
	}
	return legacy, nil, nil
}

func subscriberTypedEntitiesAllowed(entities []map[string]any) bool {
	if len(entities) == 0 {
		return true
	}
	allowed := map[string]struct{}{
		"msisdn": {}, "phone": {}, "phone_number": {},
		"subscriber_reference": {}, "subscriber_ref": {}, "subscriber_id": {},
		"imsi": {}, "iccid": {}, "imei": {},
		"service_reference": {}, "service_identifier": {},
	}
	for _, entity := range entities {
		kind := normalize(strings.ReplaceAll(stringValueAny(entity["type"]), "-", "_"))
		if _, ok := allowed[kind]; !ok || strings.TrimSpace(stringValueAny(entity["value"])) == "" {
			return false
		}
	}
	return true
}

func legacyHybridToEnterpriseV1(caseID, requestID string, request map[string]any, legacy hybridQueryResponse) EnterpriseResponseV1 {
	enterprise := legacy.Enterprise
	rows := flattenEnterpriseRows(legacy.Records, 100)
	if legacy.Template == "video_anpr_grouped_timeline" {
		rows = videoResultRows(legacy.Records["video_anpr_grouped_timeline"])
	}
	operation := mapFromAny(enterprise["operation"])
	resultState := stringValueAny(enterprise["result_state"])
	if resultState == "" {
		resultState = enterpriseResultState(legacy, rows)
	}
	response := EnterpriseResponseV1{
		ContractVersion:       "forensics.enterprise-response/v1",
		RequestID:             requestID,
		CaseID:                caseID,
		CollectionID:          legacy.CollectionID,
		Status:                stringValueAny(enterprise["status"]),
		ResultState:           resultState,
		ProcessingState:       stringValueAny(enterprise["processing_state"]),
		RowCount:              enterprisePublicRowCount(legacy, rows, resultState),
		OperationID:           stringValueAny(operation["operation_id"]),
		ExecutiveAnswer:       stringValueAny(enterprise["summary"]),
		DeterministicFindings: []FindingV1{},
		SemanticEvidence:      []CitationV1{},
		InferredRelationships: []InferredRelationshipV1{},
		ObservationPackets:    []ObservationPacketV1{},
		ToolResults:           append([]ToolResultV1(nil), legacy.ToolResults...),
		PlanValidation:        legacy.PlanValidation,
		Clarification:         legacy.Clarification,
		UnsupportedOperations: []UnsupportedOperationV1{},
		Limitations:           stringSliceAny(enterprise["limitations"]),
		NextActions:           []NextActionV1{},
		Tables:                []TableResultV1{},
		Visualizations:        []VisualizationV1{},
		ExecutionTrace: ExecutionTraceV1{
			Route: legacy.Route, ToolIDs: []string{}, AdapterIDs: []string{}, ModelRoles: []string{},
			Parameters: map[string]any{"template": legacy.Template, "limit": request["limit"]},
			LatencyMS: map[string]int64{
				"db": legacy.Telemetry.DBLatencyMS, "kb": legacy.Telemetry.KBLatencyMS, "model": legacy.Telemetry.LLMLatencyMS,
			},
		},
	}
	if response.ProcessingState == "" {
		response.ProcessingState = enterpriseProcessingState(legacy)
	}
	if legacy.Template == "video_anpr_grouped_timeline" {
		response.ExecutiveAnswer = stringValueAny(enterprise["executive_answer"])
	}
	if response.OperationID != "" {
		response.ExecutionTrace.ToolIDs = appendUniqueString(response.ExecutionTrace.ToolIDs, response.OperationID)
	}
	if response.RequestID == "" {
		response.RequestID = legacy.Telemetry.RequestID
	}
	if response.Status == "" {
		response.Status = enterpriseOutcomeStatus(legacy, rows)
	}
	if response.ExecutiveAnswer == "" {
		response.ExecutiveAnswer = stringValueAny(legacy.Answer["records_summary"])
	}

	if synthesis, ok := enterprise["synthesis"].(map[string]any); ok {
		for index, claim := range mapsFromAny(synthesis["claims"]) {
			if !strings.HasPrefix(stringValueAny(claim["source"]), "Deterministic Fact") {
				continue
			}
			measures := map[string]any{}
			if value, exists := claim["value"]; exists {
				measures["value"] = value
			}
			response.DeterministicFindings = append(response.DeterministicFindings, FindingV1{
				ID: fmt.Sprintf("finding-%d", index+1), Statement: stringValueAny(claim["claim"]), Measures: measures, CitationIDs: []string{},
			})
		}
	}
	for index, item := range mapsFromAny(enterprise["provenance"]) {
		citationID := fmt.Sprintf("citation-%d", index+1)
		locatorParts := []string{}
		for _, key := range []string{"source_entry", "row_number", "row_hash", "timestamp"} {
			if value := strings.TrimSpace(stringValueAny(item[key])); value != "" {
				locatorParts = append(locatorParts, key+":"+value)
			}
		}
		if len(locatorParts) == 0 {
			if locator := firstPresent(item, "source_locator", "citation_locator"); locator != nil {
				encoded := strings.TrimSpace(stringValueAny(locator))
				if _, ok := locator.(string); !ok {
					encoded = marshalJSONString(locator)
				}
				if encoded != "" && encoded != "null" && encoded != `""` {
					locatorParts = append(locatorParts, encoded)
				}
			}
		}
		response.SemanticEvidence = append(response.SemanticEvidence, CitationV1{
			ID: citationID, TenantID: stringValueAny(request["tenant_id"]), CollectionID: legacy.CollectionID,
			EvidenceID: stringValueAny(item["evidence_id"]), VersionID: stringValueAny(item["version_id"]),
			Locator: strings.Join(locatorParts, ","), SourceName: stringValueAny(item["source_file"]),
		})
		if legacy.Template == "video_anpr_grouped_timeline" {
			response.SemanticEvidence[len(response.SemanticEvidence)-1].Locator = marshalJSONString(item["source_locator"])
		}
	}
	if grid, ok := enterprise["data_grid"].(map[string]any); ok {
		rows := mapsFromAny(grid["rows"])
		columns := []TableColumnV1{}
		for _, column := range mapsFromAny(grid["columns"]) {
			label := stringValueAny(column["label"])
			if label == "" {
				label = stringValueAny(column["header"])
			}
			columns = append(columns, TableColumnV1{Key: stringValueAny(column["key"]), Label: label, Type: stringValueAny(column["type"])})
		}
		if len(rows) > 0 || len(columns) > 0 {
			response.Tables = append(response.Tables, TableResultV1{ID: "primary-results", Title: "Deterministic results", Columns: columns, Rows: rows})
		}
	}
	for _, action := range mapsFromAny(enterprise["recommended_actions"]) {
		response.NextActions = append(response.NextActions, NextActionV1{Label: stringValueAny(action["label"]), Query: stringValueAny(action["query"])})
	}
	if legacy.Capability.Status == "unavailable" {
		for _, capability := range legacy.Capability.MissingCapabilities {
			response.UnsupportedOperations = append(response.UnsupportedOperations, UnsupportedOperationV1{
				OperationID: legacy.Template, Reason: legacy.Capability.Explanation, RequiredCapability: capability,
			})
		}
	}
	if summary := strings.TrimSpace(stringValueAny(legacy.Answer["llm_summary"])); summary != "" && strings.TrimSpace(stringValueAny(legacy.Answer["llm_fallback_reason"])) == "" {
		response.ModelInterpretation = &ModelInterpretationV1{
			Text: summary, ModelRoleID: "nexusai.model-role.explanation", ModelID: stringValueAny(request["synthesis_model"]), CitationIDs: []string{}, Fallback: false,
		}
		response.ExecutionTrace.ModelRoles = []string{"nexusai.model-role.explanation"}
	}
	if len(response.SemanticEvidence) > 0 && len(response.DeterministicFindings) > 0 {
		citationIDs := make([]string, 0, len(response.SemanticEvidence))
		for _, citation := range response.SemanticEvidence {
			citationIDs = append(citationIDs, citation.ID)
		}
		for index := range response.DeterministicFindings {
			response.DeterministicFindings[index].CitationIDs = append([]string(nil), citationIDs...)
		}
	}
	if legacy.Composition != nil {
		return enrichCompositionEnterpriseResponse(response, request, legacy)
	}
	// Media plate observations must not acquire structured ANPR query citations.
	if legacy.Template == "video_anpr_grouped_timeline" {
		return response
	}
	if legacy.Template == "document_search" {
		return enrichDocumentEnterpriseResponse(response, legacy)
	}
	return enrichTowerEnterpriseResponse(enrichSubscriberEnterpriseResponse(enrichANPREnterpriseResponse(enrichIPDREnterpriseResponse(enrichCDREnterpriseResponse(response, request, legacy), request, legacy), request, legacy), request, legacy), request, legacy)
}

func enrichDocumentEnterpriseResponse(response EnterpriseResponseV1, legacy hybridQueryResponse) EnterpriseResponseV1 {
	response.ExecutionTrace.AdapterIDs = appendUniqueString(response.ExecutionTrace.AdapterIDs, "nexusai.adapter.native_document")
	response.ExecutionTrace.Parameters["family_id"] = "document_intelligence"
	if strategy := mapFromAny(legacy.Planner["retrieval_strategy"]); len(strategy) > 0 {
		response.ExecutionTrace.Parameters["retrieval_mode"] = strategy["retrieval_mode"]
		response.ExecutionTrace.Parameters["retrieval_contract"] = strategy["contract_version"]
	}
	response.Limitations = appendUniqueString(response.Limitations, "Document findings are bounded current-version passages. A locator is shown only when the extractor supplied it; scanned or unsupported pages are not inferred.")
	response.DeterministicFindings = []FindingV1{}
	for index, result := range evidenceResults(legacy.Evidence) {
		if index == 8 {
			break
		}
		passage := strings.TrimSpace(stringValueAny(firstPresent(result, "preview", "content")))
		if passage == "" {
			continue
		}
		metadata, _ := result["metadata"].(map[string]any)
		citationIDs := []string{}
		for _, citation := range response.SemanticEvidence {
			if evidenceID := strings.TrimSpace(stringValueAny(metadata["evidence_id"])); evidenceID != "" && !strings.EqualFold(citation.EvidenceID, evidenceID) {
				continue
			}
			if versionID := strings.TrimSpace(stringValueAny(metadata["version_id"])); versionID != "" && !strings.EqualFold(citation.VersionID, versionID) {
				continue
			}
			citationIDs = append(citationIDs, citation.ID)
			break
		}
		if len(citationIDs) == 0 {
			response.Limitations = appendUniqueString(response.Limitations, "One returned document passage could not be promoted to a finding because its source citation was unavailable.")
			continue
		}
		response.DeterministicFindings = append(response.DeterministicFindings, FindingV1{
			ID: fmt.Sprintf("finding-document-%d", index+1), Statement: passage,
			Measures:    map[string]any{"source_file": firstPresent(metadata, "source_file", "file_name", "source"), "source_location": documentLocatorLabel(firstPresent(metadata, "citation_locator", "source_locator"))},
			CitationIDs: citationIDs,
		})
	}
	return response
}

func enrichCompositionEnterpriseResponse(response EnterpriseResponseV1, request map[string]any, legacy hybridQueryResponse) EnterpriseResponseV1 {
	if legacy.Composition == nil {
		return response
	}
	composition := legacy.Composition
	response.ExecutionTrace.Parameters["plan_mode"] = "bounded_composition"
	response.ExecutionTrace.Parameters["plan_id"] = composition.PlanID
	response.ExecutionTrace.Parameters["semantic_state"] = "candidate_correlation"
	response.Limitations = appendUniqueString(response.Limitations, "Cross-family co-observation is a candidate correlation, not proof of identity, ownership, association, causation, or intent.")
	rows := []map[string]any{}
	citationByKey := map[string]string{}
	for _, step := range composition.Steps {
		response.ExecutionTrace.ToolIDs = appendUniqueString(response.ExecutionTrace.ToolIDs, step.CapabilityID)
		row := map[string]any{"step_id": step.StepID, "capability_id": step.CapabilityID, "status": step.Status, "failure_code": step.FailureCode}
		for name, field := range step.Fields {
			row[name] = field.Value
		}
		rows = append(rows, row)
		citationIDs := []string{}
		for _, citation := range step.Citations {
			key := fmt.Sprintf("%s|%s|%s|%d|%s", citation.EvidenceID, citation.VersionID, citation.SourceFile, citation.SourceRow, citation.SourceLocator)
			citationID := citationByKey[key]
			if citationID == "" {
				citationID = fmt.Sprintf("composition-citation-%d", len(citationByKey)+1)
				citationByKey[key] = citationID
				locator := citation.SourceLocator
				if locator == "" && citation.SourceRow > 0 {
					locator = fmt.Sprintf("row:%d", citation.SourceRow)
				}
				response.SemanticEvidence = append(response.SemanticEvidence, CitationV1{ID: citationID, TenantID: stringValueAny(request["tenant_id"]), CollectionID: legacy.CollectionID, EvidenceID: citation.EvidenceID, VersionID: citation.VersionID, Locator: locator, SourceName: citation.SourceFile})
			}
			citationIDs = append(citationIDs, citationID)
		}
		if step.Status == "completed" || step.Status == "no_results" {
			response.DeterministicFindings = append(response.DeterministicFindings, FindingV1{ID: "composition-" + step.StepID, Statement: fmt.Sprintf("%s completed with status %s.", step.CapabilityID, step.Status), Measures: row, CitationIDs: citationIDs})
		}
	}
	response.Tables = []TableResultV1{{ID: "composition-steps", Title: "Bounded capability composition", Columns: []TableColumnV1{{Key: "step_id", Label: "Step", Type: "string"}, {Key: "capability_id", Label: "Capability", Type: "string"}, {Key: "status", Label: "Status", Type: "string"}, {Key: "row_count", Label: "Exact Rows", Type: "integer"}, {Key: "failure_code", Label: "Failure", Type: "string"}}, Rows: rows}}
	observations := []ObservationV1{}
	for _, claim := range composition.Claims {
		if claim.SemanticState != string(AuthorityCandidateCorrelation) {
			continue
		}
		claimCitationIDs := []string{}
		for _, citation := range claim.Citations {
			key := fmt.Sprintf("%s|%s|%s|%d|%s", citation.EvidenceID, citation.VersionID, citation.SourceFile, citation.SourceRow, citation.SourceLocator)
			if citationID := citationByKey[key]; citationID != "" {
				claimCitationIDs = appendUniqueString(claimCitationIDs, citationID)
			}
		}
		if len(claimCitationIDs) == 0 {
			continue
		}
		observations = append(observations, ObservationV1{
			ID: claim.ClaimID, Kind: "cross_family_candidate", Authority: AuthorityCandidateCorrelation,
			Value: map[string]any{"step_ids": claim.StepIDs, "capability_ids": claim.CapabilityIDs}, CitationIDs: claimCitationIDs,
			Limitations: []string{"Candidate correlation only; this does not prove identity, ownership, association, causation, or intent."},
		})
	}
	if len(observations) > 0 {
		response.ObservationPackets = append(response.ObservationPackets, ObservationPacketV1{
			ContractVersion: observationPacketContractV1, PacketID: composition.PlanID + ":observations", RequestID: response.RequestID,
			CaseID: response.CaseID, CollectionID: response.CollectionID, OperationID: "bounded_composition",
			Observations: observations, Citations: append([]CitationV1(nil), response.SemanticEvidence...),
			Limitations: []string{"Cross-family claims remain candidate correlations unless independently verified."},
		})
	}
	switch composition.Status {
	case "completed":
		response.Status = "answered_with_limitations"
		response.ExecutiveAnswer = fmt.Sprintf("Completed a validated %d-step bounded analysis. The combined result is a candidate correlation only; review each cited capability result independently.", len(composition.Steps))
	case "partial_analysis":
		response.Status = "partial_analysis"
		response.ExecutiveAnswer = "The bounded analysis completed only partially. Independently valid step findings are shown, but NexusAI did not claim a complete correlation."
	default:
		response.Status = "execution_failed"
		response.ExecutiveAnswer = "The validated composition could not complete, so NexusAI did not present a cross-family conclusion."
	}
	return response
}

func enrichCDREnterpriseResponse(response EnterpriseResponseV1, request map[string]any, legacy hybridQueryResponse) EnterpriseResponseV1 {
	operationByTemplate := map[string]string{
		"frequent_contacts": "cdr.frequent_contacts", "call_type_breakdown": "cdr.call_type_breakdown",
		"temporal_activity": "cdr.temporal_activity", "duration_extremes": "cdr.duration_extremes",
		"shortest_call": "cdr.duration_extremes", "longest_call": "cdr.duration_extremes",
		"entity_timeline": "cdr.timeline", "geospatial_movement": "cdr.geospatial_movement",
		"device_identity_changes": "cdr.device_identity_changes", "service_usage": "cdr.service_usage",
		"tower_activity": "cdr.tower_activity",
	}
	operationID := operationByTemplate[legacy.Template]
	if operationID == "" {
		return response
	}
	response.ExecutionTrace.ToolIDs = appendUniqueString(response.ExecutionTrace.ToolIDs, operationID)
	response.ExecutionTrace.AdapterIDs = appendUniqueString(response.ExecutionTrace.AdapterIDs, "nexusai.adapter.cdr")
	response.ExecutionTrace.Parameters["family_id"] = "communications_cdr"
	response.ExecutionTrace.Parameters["adapter_version"] = "1.2.0"

	citationIDs := make([]string, 0, len(response.SemanticEvidence)+1)
	for _, citation := range response.SemanticEvidence {
		citationIDs = append(citationIDs, citation.ID)
	}
	if len(citationIDs) == 0 {
		citationID := "citation-cdr-query-1"
		response.SemanticEvidence = append(response.SemanticEvidence, CitationV1{
			ID: citationID, TenantID: stringValueAny(request["tenant_id"]), CollectionID: response.CollectionID,
			Locator:    "table:forensic.cdr_records;template:" + legacy.Template,
			SourceName: "forensic.cdr_records",
		})
		citationIDs = append(citationIDs, citationID)
		response.Limitations = appendUniqueString(response.Limitations, "Aggregate CDR results carry query-level table provenance; use source-record or device-change operations for row-level locators.")
	}

	rowCount := 0
	if len(response.Tables) > 0 {
		rowCount = len(response.Tables[0].Rows)
	}
	if len(response.DeterministicFindings) == 0 {
		response.DeterministicFindings = append(response.DeterministicFindings, FindingV1{
			ID:        "finding-cdr-1",
			Statement: fmt.Sprintf("The %s operation returned %d deterministic result rows.", operationID, rowCount),
			Measures:  map[string]any{"operation_id": operationID, "row_count": rowCount}, CitationIDs: append([]string(nil), citationIDs...),
		})
	} else {
		for index := range response.DeterministicFindings {
			if len(response.DeterministicFindings[index].CitationIDs) == 0 {
				response.DeterministicFindings[index].CitationIDs = append([]string(nil), citationIDs...)
			}
		}
	}

	rows := []map[string]any{}
	if len(response.Tables) > 0 {
		rows = response.Tables[0].Rows
	}
	visualization := cdrVisualizationForTemplate(legacy.Template, rows, citationIDs)
	if visualization.ID != "" {
		response.Visualizations = append(response.Visualizations, visualization)
	}
	return response
}

func cdrVisualizationForTemplate(template string, rows []map[string]any, citationIDs []string) VisualizationV1 {
	if len(rows) == 0 {
		return VisualizationV1{}
	}
	if len(rows) > 50 {
		rows = rows[:50]
	}
	base := VisualizationV1{CitationIDs: append([]string(nil), citationIDs...)}
	switch template {
	case "frequent_contacts":
		base.ID, base.Type, base.Title = "cdr-contact-frequency", "bar", "Frequent contacts"
		base.Spec = map[string]any{"category_field": "dialed_number", "value_field": "total_interactions", "series_fields": []string{"incoming_count", "outgoing_count"}, "rows": rows}
	case "call_type_breakdown", "service_usage":
		base.ID, base.Type, base.Title = "cdr-service-usage", "bar", "Communication service usage"
		categoryField := "call_type"
		if template == "service_usage" {
			categoryField = "service_class"
		}
		base.Spec = map[string]any{"category_field": categoryField, "value_field": "event_count", "series_field": "direction", "rows": rows}
	case "temporal_activity":
		base.ID, base.Type, base.Title = "cdr-hourly-activity", "bar", "Hourly communications activity"
		base.Spec = map[string]any{"category_field": "hour_of_day", "value_field": "event_count", "rows": rows}
	case "duration_extremes", "shortest_call", "longest_call":
		base.ID, base.Type, base.Title = "cdr-duration-extremes", "bar", "Call duration extremes"
		base.Spec = map[string]any{"category_field": "metric", "value_field": "duration_seconds", "rows": rows}
	case "entity_timeline", "device_identity_changes":
		base.ID, base.Type, base.Title = "cdr-communications-timeline", "timeline", "Communications timeline"
		base.Spec = map[string]any{"time_fields": []string{"event_time", "observed_at", "call_start_ts"}, "label_fields": []string{"event_label", "change_type", "call_type"}, "rows": rows}
	case "geospatial_movement", "tower_activity":
		base.ID, base.Type, base.Title = "cdr-location-sequence", "map", "CDR location sequence"
		base.Spec = map[string]any{
			"latitude_field": "latitude", "longitude_field": "longitude",
			"time_fields":     []string{"call_start_ts", "observed_at"},
			"label_fields":    []string{"location", "cell_site_id", "site_id"},
			"source_fields":   []string{"source_file", "row_number", "row_hash"},
			"route_inference": false, "rows": rows,
		}
	}
	return base
}

func enrichIPDREnterpriseResponse(response EnterpriseResponseV1, request map[string]any, legacy hybridQueryResponse) EnterpriseResponseV1 {
	operationByTemplate := map[string]string{
		"ipdr_endpoint_summary":    "ipdr.endpoint_summary",
		"ipdr_domain_summary":      "ipdr.domain_summary",
		"ipdr_protocol_breakdown":  "ipdr.protocol_breakdown",
		"ipdr_session_volume":      "ipdr.session_volume",
		"ipdr_subscriber_sessions": "ipdr.subscriber_sessions",
		"ipdr_concurrent_sessions": "ipdr.concurrent_sessions",
		"ipdr_timeline":            "ipdr.timeline",
	}
	operationID := operationByTemplate[legacy.Template]
	if operationID == "" {
		return response
	}
	response.ExecutionTrace.ToolIDs = appendUniqueString(response.ExecutionTrace.ToolIDs, operationID)
	response.ExecutionTrace.AdapterIDs = appendUniqueString(response.ExecutionTrace.AdapterIDs, "nexusai.adapter.ipdr")
	response.ExecutionTrace.Parameters["family_id"] = "network_ipdr"
	response.ExecutionTrace.Parameters["adapter_version"] = "1.1.0"
	response.Limitations = appendUniqueString(response.Limitations, "IPDR rows ingested before adapter 1.1 are read only from explicit legacy raw-payload fields when normalized fields are absent; no ownership, DNS, NAT, route, location, payload, or attribution facts are inferred.")

	citationIDs := make([]string, 0, len(response.SemanticEvidence)+1)
	for _, citation := range response.SemanticEvidence {
		citationIDs = append(citationIDs, citation.ID)
	}
	if len(citationIDs) == 0 {
		citationID := "citation-ipdr-query-1"
		response.SemanticEvidence = append(response.SemanticEvidence, CitationV1{
			ID: citationID, TenantID: stringValueAny(request["tenant_id"]), CollectionID: response.CollectionID,
			Locator: "table:forensic.records;record_type:ipdr;template:" + legacy.Template, SourceName: "forensic.records",
		})
		citationIDs = append(citationIDs, citationID)
		response.Limitations = appendUniqueString(response.Limitations, "Aggregate IPDR results carry query-level canonical-table provenance; subscriber-session, overlap, and timeline operations return row-level source locators.")
	}

	rows := []map[string]any{}
	if len(response.Tables) > 0 {
		rows = response.Tables[0].Rows
	}
	if len(response.DeterministicFindings) == 0 {
		response.DeterministicFindings = append(response.DeterministicFindings, FindingV1{
			ID: "finding-ipdr-1", Statement: fmt.Sprintf("The %s operation returned %d deterministic result rows.", operationID, len(rows)),
			Measures: map[string]any{"operation_id": operationID, "row_count": len(rows)}, CitationIDs: append([]string(nil), citationIDs...),
		})
	} else {
		for index := range response.DeterministicFindings {
			if len(response.DeterministicFindings[index].CitationIDs) == 0 {
				response.DeterministicFindings[index].CitationIDs = append([]string(nil), citationIDs...)
			}
		}
	}
	if visualization := ipdrVisualizationForTemplate(legacy.Template, rows, citationIDs); visualization.ID != "" {
		response.Visualizations = append(response.Visualizations, visualization)
	}
	return response
}

func ipdrVisualizationForTemplate(template string, rows []map[string]any, citationIDs []string) VisualizationV1 {
	if len(rows) == 0 {
		return VisualizationV1{}
	}
	if len(rows) > 50 {
		rows = rows[:50]
	}
	base := VisualizationV1{CitationIDs: append([]string(nil), citationIDs...)}
	switch template {
	case "ipdr_endpoint_summary":
		base.ID, base.Type, base.Title = "ipdr-endpoint-volume", "bar", "Network endpoint sessions"
		base.Spec = map[string]any{"category_fields": []string{"source_ip", "destination_ip"}, "value_field": "session_count", "secondary_value_field": "total_bytes", "rows": rows}
	case "ipdr_domain_summary":
		base.ID, base.Type, base.Title = "ipdr-domain-volume", "bar", "Observed IPDR domains"
		base.Spec = map[string]any{"category_field": "domain", "value_field": "session_count", "secondary_value_field": "total_bytes", "rows": rows}
	case "ipdr_protocol_breakdown":
		base.ID, base.Type, base.Title = "ipdr-protocol-volume", "bar", "IPDR protocol breakdown"
		base.Spec = map[string]any{"category_field": "protocol", "value_field": "session_count", "secondary_value_field": "total_bytes", "rows": rows}
	case "ipdr_session_volume":
		base.ID, base.Type, base.Title = "ipdr-hourly-volume", "timeline", "Hourly network-session volume"
		base.Spec = map[string]any{"time_fields": []string{"hour_start"}, "label_fields": []string{"session_count", "total_bytes"}, "rows": rows}
	case "ipdr_subscriber_sessions", "ipdr_concurrent_sessions", "ipdr_timeline":
		base.ID, base.Type, base.Title = "ipdr-session-timeline", "timeline", "Network-session timeline"
		base.Spec = map[string]any{"time_fields": []string{"session_start", "session_end", "overlap_boundary"}, "label_fields": []string{"session_identifier", "protocol", "domain", "source_ip", "destination_ip"}, "rows": rows}
	}
	return base
}

func enrichANPREnterpriseResponse(response EnterpriseResponseV1, request map[string]any, legacy hybridQueryResponse) EnterpriseResponseV1 {
	operationByTemplate := map[string]string{
		"anpr_sightings":              "anpr.sightings",
		"anpr_camera_sequence":        "anpr.camera_sequence",
		"anpr_camera_activity":        "anpr.camera_activity",
		"anpr_co_travel":              "anpr.co_travel",
		"anpr_route_timing":           "anpr.route_timing",
		"anpr_plate_variants":         "anpr.plate_variants",
		"anpr_timeline":               "anpr.timeline",
		"video_anpr_grouped_timeline": "video.anpr_grouped_timeline",
	}
	operationID := operationByTemplate[legacy.Template]
	if operationID == "" {
		return response
	}
	response.ExecutionTrace.ToolIDs = appendUniqueString(response.ExecutionTrace.ToolIDs, operationID)
	response.ExecutionTrace.AdapterIDs = appendUniqueString(response.ExecutionTrace.AdapterIDs, "nexusai.adapter.anpr")
	response.ExecutionTrace.Parameters["family_id"] = "anpr_vehicles"
	response.ExecutionTrace.Parameters["adapter_version"] = "1.1.0"
	response.Limitations = appendUniqueString(response.Limitations, "ANPR results describe supplied structured observations only. They do not establish vehicle ownership, occupants, identity, association, or image-level OCR accuracy.")
	if legacy.Template == "anpr_co_travel" {
		response.Limitations = appendUniqueString(response.Limitations, "Co-travel is a bounded same-camera observation within five minutes and is not proof that vehicles travelled together or are associated.")
	}
	if legacy.Template == "anpr_route_timing" {
		response.Limitations = appendUniqueString(response.Limitations, "Route timing uses consecutive sightings and optional WGS84 straight-line distance; it does not reconstruct a road route, account for camera clock drift, or prove continuous travel.")
	}

	citationIDs := make([]string, 0, len(response.SemanticEvidence)+1)
	for _, citation := range response.SemanticEvidence {
		citationIDs = append(citationIDs, citation.ID)
	}
	if len(citationIDs) == 0 {
		citationID := "citation-anpr-query-1"
		response.SemanticEvidence = append(response.SemanticEvidence, CitationV1{
			ID: citationID, TenantID: stringValueAny(request["tenant_id"]), CollectionID: response.CollectionID,
			Locator: "table:forensic.records;record_type:anpr;template:" + legacy.Template, SourceName: "forensic.records",
		})
		citationIDs = append(citationIDs, citationID)
		response.Limitations = appendUniqueString(response.Limitations, "Aggregate ANPR results carry query-level canonical-table provenance; sighting, sequence, route-timing, and timeline rows include source-row locators.")
	}

	rows := []map[string]any{}
	if len(response.Tables) > 0 {
		rows = response.Tables[0].Rows
	}
	if len(response.DeterministicFindings) == 0 {
		response.DeterministicFindings = append(response.DeterministicFindings, FindingV1{
			ID: "finding-anpr-1", Statement: fmt.Sprintf("The %s operation returned %d deterministic result rows.", operationID, len(rows)),
			Measures: map[string]any{"operation_id": operationID, "row_count": len(rows)}, CitationIDs: append([]string(nil), citationIDs...),
		})
	} else {
		for index := range response.DeterministicFindings {
			if len(response.DeterministicFindings[index].CitationIDs) == 0 {
				response.DeterministicFindings[index].CitationIDs = append([]string(nil), citationIDs...)
			}
		}
	}
	if visualization := anprVisualizationForTemplate(legacy.Template, rows, citationIDs); visualization.ID != "" {
		response.Visualizations = append(response.Visualizations, visualization)
	}
	return response
}

func anprVisualizationForTemplate(template string, rows []map[string]any, citationIDs []string) VisualizationV1 {
	if len(rows) == 0 {
		return VisualizationV1{}
	}
	if len(rows) > 50 {
		rows = rows[:50]
	}
	base := VisualizationV1{CitationIDs: append([]string(nil), citationIDs...)}
	switch template {
	case "anpr_camera_activity":
		base.ID, base.Type, base.Title = "anpr-camera-activity", "bar", "ANPR camera activity"
		base.Spec = map[string]any{"category_fields": []string{"camera_id", "location"}, "value_field": "sighting_count", "secondary_value_field": "distinct_plate_count", "rows": rows}
	case "anpr_sightings", "anpr_camera_sequence", "anpr_timeline":
		base.ID, base.Type, base.Title = "anpr-sighting-timeline", "timeline", "ANPR sighting timeline"
		base.Spec = map[string]any{"time_fields": []string{"observed_at"}, "label_fields": []string{"plate_number", "camera_id", "location"}, "rows": rows}
	case "anpr_route_timing":
		base.ID, base.Type, base.Title = "anpr-observation-map", "map", "ANPR consecutive observations"
		base.Spec = map[string]any{"latitude_field": "latitude", "longitude_field": "longitude", "time_fields": []string{"to_observed_at"}, "label_fields": []string{"to_camera_id", "to_location"}, "rows": rows, "distance_field": "straight_line_distance_km"}
	case "anpr_co_travel":
		base.ID, base.Type, base.Title = "anpr-co-observations", "bar", "Same-camera temporal co-observations"
		base.Spec = map[string]any{"category_fields": []string{"co_observed_plate", "camera_id"}, "value_field": "co_observation_count", "secondary_value_field": "minimum_separation_seconds", "rows": rows}
	case "anpr_plate_variants":
		base.ID, base.Type, base.Title = "anpr-plate-variants", "bar", "Observed plate variants"
		base.Spec = map[string]any{"category_field": "observed_plate_variant", "value_field": "sighting_count", "secondary_value_field": "average_supplied_confidence", "rows": rows}
	}
	return base
}

func enrichSubscriberEnterpriseResponse(response EnterpriseResponseV1, request map[string]any, legacy hybridQueryResponse) EnterpriseResponseV1 {
	operationByTemplate := map[string]string{
		"subscriber_identity_lookup":   "subscriber.identity_lookup",
		"subscriber_validity_timeline": "subscriber.validity_timeline",
		"subscriber_device_links":      "subscriber.device_links",
		"subscriber_status_summary":    "subscriber.status_summary",
		"subscriber_conflict_audit":    "subscriber.conflict_audit",
		"subscriber_reuse_candidates":  "subscriber.reuse_candidates",
	}
	operationID := operationByTemplate[legacy.Template]
	if operationID == "" {
		return response
	}
	response.ExecutionTrace.ToolIDs = appendUniqueString(response.ExecutionTrace.ToolIDs, operationID)
	response.ExecutionTrace.AdapterIDs = appendUniqueString(response.ExecutionTrace.AdapterIDs, "nexusai.adapter.subscriber_identity")
	response.ExecutionTrace.Parameters["family_id"] = "subscriber_identity"
	response.ExecutionTrace.Parameters["adapter_version"] = "1.2.0"
	response.Limitations = appendUniqueString(response.Limitations, "Subscriber results expose role-separated, case-authorized subscriber, SIM, device, and service observations while masking CNIC and omitting subscriber names. Co-observation does not establish identity, ownership, physical SIM/device use, entitlement, or current control.")
	if legacy.Template == "subscriber_validity_timeline" {
		response.Limitations = appendUniqueString(response.Limitations, "Missing or open validity boundaries remain unknown; the newest row is not promoted to historical ownership or uninterrupted service.")
	}
	if legacy.Template == "subscriber_conflict_audit" || legacy.Template == "subscriber_reuse_candidates" {
		response.Limitations = appendUniqueString(response.Limitations, "Conflict and reuse candidates require human reconciliation and do not prove reassignment, SIM swap, device sharing, fraud, or ownership.")
	}

	citationIDs := make([]string, 0, len(response.SemanticEvidence)+1)
	for _, citation := range response.SemanticEvidence {
		citationIDs = append(citationIDs, citation.ID)
	}
	if len(citationIDs) == 0 {
		citationID := "citation-subscriber-query-1"
		response.SemanticEvidence = append(response.SemanticEvidence, CitationV1{
			ID: citationID, TenantID: stringValueAny(request["tenant_id"]), CollectionID: response.CollectionID,
			Locator: "table:forensic.records;record_type:subscriber;template:" + legacy.Template, SourceName: "forensic.records",
		})
		citationIDs = append(citationIDs, citationID)
		response.Limitations = appendUniqueString(response.Limitations, "Aggregate subscriber results carry query-level canonical-table provenance; identity, validity, and device-link rows include source-row locators.")
	}

	rows := []map[string]any{}
	if len(response.Tables) > 0 {
		rows = response.Tables[0].Rows
	}
	if len(response.DeterministicFindings) == 0 {
		response.DeterministicFindings = append(response.DeterministicFindings, FindingV1{
			ID: "finding-subscriber-1", Statement: fmt.Sprintf("The %s operation returned %d deterministic result rows.", operationID, len(rows)),
			Measures: map[string]any{"operation_id": operationID, "row_count": len(rows)}, CitationIDs: append([]string(nil), citationIDs...),
		})
	} else {
		for index := range response.DeterministicFindings {
			if len(response.DeterministicFindings[index].CitationIDs) == 0 {
				response.DeterministicFindings[index].CitationIDs = append([]string(nil), citationIDs...)
			}
		}
	}
	if visualization := subscriberVisualizationForTemplate(legacy.Template, rows, citationIDs); visualization.ID != "" {
		response.Visualizations = append(response.Visualizations, visualization)
	}
	return response
}

func enrichTowerEnterpriseResponse(response EnterpriseResponseV1, request map[string]any, legacy hybridQueryResponse) EnterpriseResponseV1 {
	operationByTemplate := map[string]string{
		"tower_site_lookup":        "tower.site_lookup",
		"tower_reference_timeline": "tower.reference_timeline",
		"tower_coordinate_audit":   "tower.coordinate_audit",
		"tower_status_summary":     "tower.status_summary",
		"tower_alias_conflicts":    "tower.alias_conflicts",
		"tower_cdr_join":           "tower.cdr_join",
	}
	operationID := operationByTemplate[legacy.Template]
	if operationID == "" {
		return response
	}
	response.ExecutionTrace.ToolIDs = appendUniqueString(response.ExecutionTrace.ToolIDs, operationID)
	response.ExecutionTrace.AdapterIDs = appendUniqueString(response.ExecutionTrace.AdapterIDs, "nexusai.adapter.tower_location")
	response.ExecutionTrace.Parameters["family_id"] = "tower_location"
	response.ExecutionTrace.Parameters["adapter_version"] = "1.2.0"
	response.Limitations = appendUniqueString(response.Limitations, "Tower/site identifiers, coordinates, datums, uncertainty, status, and validity are supplied reference facts. Results do not establish RF coverage, handset position, subscriber presence, home, continuous movement, route, ownership, or association.")
	if legacy.Template == "tower_cdr_join" {
		response.Limitations = appendUniqueString(response.Limitations, "The CDR join uses exact cell/site identifiers and the latest eligible supplied reference window; unmatched or missing reference history remains explicit.")
	}

	citationIDs := make([]string, 0, len(response.SemanticEvidence)+1)
	for _, citation := range response.SemanticEvidence {
		citationIDs = append(citationIDs, citation.ID)
	}
	if len(citationIDs) == 0 {
		citationID := "citation-tower-query-1"
		response.SemanticEvidence = append(response.SemanticEvidence, CitationV1{
			ID: citationID, TenantID: stringValueAny(request["tenant_id"]), CollectionID: response.CollectionID,
			Locator: "table:forensic.records;record_type:tower_location;template:" + legacy.Template, SourceName: "forensic.records",
		})
		citationIDs = append(citationIDs, citationID)
		response.Limitations = appendUniqueString(response.Limitations, "Aggregate tower results carry query-level canonical-table provenance; reference rows include source-row locators when present.")
	}
	for index := range response.DeterministicFindings {
		if len(response.DeterministicFindings[index].CitationIDs) == 0 {
			response.DeterministicFindings[index].CitationIDs = append([]string(nil), citationIDs...)
		}
	}
	rows := []map[string]any{}
	if len(response.Tables) > 0 {
		rows = response.Tables[0].Rows
	}
	if len(response.DeterministicFindings) == 0 {
		response.DeterministicFindings = append(response.DeterministicFindings, FindingV1{
			ID: "finding-tower-1", Statement: fmt.Sprintf("The %s operation returned %d deterministic result rows.", operationID, len(rows)),
			Measures: map[string]any{"operation_id": operationID, "row_count": len(rows)}, CitationIDs: append([]string(nil), citationIDs...),
		})
	}
	response.Visualizations = append(response.Visualizations, towerVisualizationsForTemplate(legacy.Template, rows, citationIDs)...)
	return response
}

func towerVisualizationsForTemplate(template string, rows []map[string]any, citationIDs []string) []VisualizationV1 {
	if len(rows) == 0 {
		return nil
	}
	if len(rows) > 50 {
		rows = rows[:50]
	}
	baseSpec := map[string]any{
		"rows":                    rows,
		"source_fields":           []string{"source_file", "row_number", "row_hash", "evidence_id", "cdr_source_file", "cdr_row_number", "cdr_row_hash", "reference_source_file", "reference_row_number", "reference_row_hash", "reference_evidence_id"},
		"datum_field":             "coordinate_datum",
		"uncertainty_field":       "uncertainty_radius_m",
		"uncertainty_class_field": "uncertainty_class",
		"match_status_field":      "match_status",
		"route_inference":         false,
		"rf_presence_inference":   false,
	}
	copySpec := func() map[string]any {
		result := make(map[string]any, len(baseSpec)+4)
		for key, value := range baseSpec {
			result[key] = value
		}
		return result
	}
	visualizations := []VisualizationV1{}
	switch template {
	case "tower_coordinate_audit", "tower_cdr_join", "tower_site_lookup":
		spec := copySpec()
		spec["latitude_field"], spec["longitude_field"] = "latitude", "longitude"
		spec["label_fields"] = []string{"site_identifier", "cell_site_id", "sector_identifier", "site_location"}
		visualizations = append(visualizations, VisualizationV1{
			ID: "tower-reference-map", Type: "map", Title: "Supplied tower/site reference coordinates",
			CitationIDs: append([]string(nil), citationIDs...), Spec: spec,
		})
	}
	if template == "tower_reference_timeline" || template == "tower_cdr_join" {
		spec := copySpec()
		if template == "tower_cdr_join" {
			spec["time_fields"] = []string{"cdr_observed_at", "call_start_ts", "observed_at", "valid_from_at", "valid_to_at"}
			spec["label_fields"] = []string{"match_status", "cell_site_id", "site_identifier", "sector_identifier"}
		} else {
			spec["time_fields"] = []string{"valid_from_at", "valid_to_at"}
			spec["label_fields"] = []string{"site_identifier", "sector_identifier", "technology", "operational_status"}
		}
		visualizations = append(visualizations, VisualizationV1{
			ID: "tower-reference-timeline", Type: "timeline", Title: "Tower/site reference history",
			CitationIDs: append([]string(nil), citationIDs...), Spec: spec,
		})
	}
	if template == "tower_status_summary" {
		spec := copySpec()
		spec["category_field"], spec["series_field"], spec["value_field"] = "operational_status", "technology", "reference_row_count"
		visualizations = append(visualizations, VisualizationV1{
			ID: "tower-status-summary", Type: "bar", Title: "Tower/site status and technology",
			CitationIDs: append([]string(nil), citationIDs...), Spec: spec,
		})
	}
	if len(visualizations) == 0 {
		spec := copySpec()
		visualizations = append(visualizations, VisualizationV1{
			ID: "tower-reference-results", Type: "table", Title: "Tower/site reference results",
			CitationIDs: append([]string(nil), citationIDs...), Spec: spec,
		})
	}
	return visualizations
}

func subscriberVisualizationForTemplate(template string, rows []map[string]any, citationIDs []string) VisualizationV1 {
	if len(rows) == 0 {
		return VisualizationV1{}
	}
	if len(rows) > 50 {
		rows = rows[:50]
	}
	base := VisualizationV1{CitationIDs: append([]string(nil), citationIDs...)}
	switch template {
	case "subscriber_identity_lookup", "subscriber_validity_timeline", "subscriber_device_links":
		base.ID, base.Type, base.Title = "subscriber-validity-timeline", "timeline", "Subscriber identity observations"
		base.Spec = map[string]any{"time_fields": []string{"valid_from_at", "activation_at", "observed_from"}, "label_fields": []string{"msisdn", "subscriber_reference", "service_identifier", "subscriber_status", "imsi", "iccid", "imei"}, "rows": rows}
	case "subscriber_status_summary":
		base.ID, base.Type, base.Title = "subscriber-status-summary", "bar", "Subscriber status and review posture"
		base.Spec = map[string]any{"category_field": "subscriber_status", "value_field": "row_count", "series_field": "manual_review_required", "rows": rows}
	case "subscriber_conflict_audit":
		base.ID, base.Type, base.Title = "subscriber-conflict-audit", "bar", "Subscriber attribute conflicts"
		base.Spec = map[string]any{"category_field": "stable_identifier", "value_field": "observation_count", "series_fields": []string{"distinct_msisdn_count", "distinct_imsi_count", "distinct_imei_count", "distinct_status_count"}, "rows": rows}
	case "subscriber_reuse_candidates":
		base.ID, base.Type, base.Title = "subscriber-reuse-candidates", "bar", "Subscriber identifier reuse candidates"
		base.Spec = map[string]any{"category_fields": []string{"identifier_kind", "identifier_value"}, "value_field": "distinct_msisdn_count", "secondary_value_field": "observation_count", "rows": rows}
	}
	return base
}

func appendUniqueString(values []string, value string) []string {
	if strings.TrimSpace(value) == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func mapsFromAny(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return typed
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if row, ok := item.(map[string]any); ok {
				result = append(result, row)
			}
		}
		return result
	default:
		return []map[string]any{}
	}
}

func sortedManifestResources(items []manifestItemV1) []manifestItemV1 {
	result := append([]manifestItemV1(nil), items...)
	sort.Slice(result, func(i, j int) bool { return result[i].Resource < result[j].Resource })
	return result
}
