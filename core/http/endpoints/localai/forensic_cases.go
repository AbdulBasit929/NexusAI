package localai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mudler/LocalAI/core/application"
)

const forensicCaseGovernanceContractV1 = "forensics.case-governance/v1"

func forensicPlatformDiscoveryV1Endpoint(section string) echo.HandlerFunc {
	return func(c echo.Context) error {
		return proxyForensicRecords(c, http.MethodGet, "/api/v1/forensics/"+section, nil, nil, "", "")
	}
}

// ListForensicAdaptersV1Endpoint returns the immutable forensic adapter catalog.
// @Summary List forensic evidence-family adapters
// @Tags records
// @Produce json
// @Success 200 {object} map[string]any "forensic adapter catalog"
// @Router /api/v1/forensics/adapters [get]
func ListForensicAdaptersV1Endpoint() echo.HandlerFunc {
	return forensicPlatformDiscoveryV1Endpoint("adapters")
}

// ListForensicOperationsV1Endpoint returns deterministic operation contracts.
// @Summary List forensic deterministic operations
// @Tags records
// @Produce json
// @Success 200 {object} map[string]any "forensic operation catalog"
// @Router /api/v1/forensics/operations [get]
func ListForensicOperationsV1Endpoint() echo.HandlerFunc {
	return forensicPlatformDiscoveryV1Endpoint("operations")
}

// ListForensicSpecialistsV1Endpoint returns specialist-agent manifests.
// @Summary List forensic specialist agents
// @Tags records
// @Produce json
// @Success 200 {object} map[string]any "forensic specialist catalog"
// @Router /api/v1/forensics/agents [get]
func ListForensicSpecialistsV1Endpoint() echo.HandlerFunc {
	return forensicPlatformDiscoveryV1Endpoint("agents")
}

// ListForensicContractsV1Endpoint returns query and response contracts.
// @Summary List forensic API contracts
// @Tags records
// @Produce json
// @Success 200 {object} map[string]any "forensic contract catalog"
// @Router /api/v1/forensics/contracts [get]
func ListForensicContractsV1Endpoint() echo.HandlerFunc {
	return forensicPlatformDiscoveryV1Endpoint("contracts")
}

type forensicCaseV1 struct {
	ContractVersion        string   `json:"contract_version"`
	CaseID                 string   `json:"case_id"`
	CollectionID           string   `json:"collection_id"`
	DisplayName            string   `json:"display_name"`
	Purpose                string   `json:"purpose"`
	Environment            string   `json:"environment"`
	Owner                  string   `json:"owner"`
	CaseStatus             string   `json:"case_status"`
	Visibility             string   `json:"visibility"`
	SecurityClassification string   `json:"security_classification"`
	RetentionClass         string   `json:"retention_class"`
	Aliases                []string `json:"aliases"`
	CleanupDisposition     string   `json:"cleanup_disposition"`
	Selectable             bool     `json:"selectable"`
	Warnings               []string `json:"warnings"`
}

func classifyForensicCase(collectionID string) forensicCaseV1 {
	name := strings.TrimSpace(collectionID)
	lower := strings.ToLower(name)
	result := forensicCaseV1{
		ContractVersion:        forensicCaseGovernanceContractV1,
		CaseID:                 name,
		CollectionID:           name,
		DisplayName:            name,
		Purpose:                "investigation",
		Environment:            "local",
		Owner:                  "local-operator",
		CaseStatus:             "active",
		Visibility:             "analyst",
		SecurityClassification: "internal",
		RetentionClass:         "operator-managed",
		Aliases:                []string{},
		CleanupDisposition:     "preserve",
		Selectable:             true,
		Warnings:               []string{},
	}
	switch {
	case name == "records-demo-verified":
		result.DisplayName = "Verified Records Demo"
		result.Purpose = "canonical_pilot"
		result.Environment = "demo"
		result.RetentionClass = "preserve_reference"
		result.Aliases = []string{"records-demo", "nexusai-structured-demo-20260730", "nexusai-structured-demo-v2-20260730"}
		result.Warnings = append(result.Warnings, "Canonical pilot selection; aliases remain separate until an operator approves reconciliation.")
	case name == "records-demo" || strings.HasPrefix(lower, "nexusai-structured-demo"):
		result.Purpose = "legacy_demo"
		result.Environment = "demo"
		result.CaseStatus = "reconciliation_required"
		result.Visibility = "system"
		result.RetentionClass = "hold_for_manifest_review"
		result.CleanupDisposition = "reconcile_then_archive_candidate"
		result.Warnings = append(result.Warnings, "Legacy demo collection is available for read-only investigation and reconciliation; no merge, archive, or deletion is authorized without an exact cleanup decision.")
	case lower == "forensic_records_analyst" ||
		lower == "communications_cdr_analyst" ||
		lower == "network_ipdr_capture_analyst" ||
		lower == "vehicle_anpr_geospatial_analyst" ||
		lower == "subscriber_identity_analyst" ||
		lower == "tower_location_reference_analyst":
		result.Purpose = "accidental_name_candidate"
		result.Environment = "local"
		result.CaseStatus = "cleanup_candidate"
		result.Visibility = "system"
		result.RetentionClass = "hold_for_reference_audit"
		result.CleanupDisposition = "verify_unreferenced_then_request_approval"
		result.Warnings = append(result.Warnings, "Agent-name-derived collection is available for capability inspection; inventory must verify contents and references before permanent deletion.")
	case strings.HasPrefix(lower, "forensic-") && (strings.Contains(lower, "acceptance") || strings.Contains(lower, "validation") || strings.Contains(lower, "audit") || strings.Contains(lower, "golden")):
		result.Purpose = "acceptance_or_system"
		result.Environment = "test"
		result.CaseStatus = "retained_system"
		result.Visibility = "system"
		result.RetentionClass = "reproducibility_hold"
		result.CleanupDisposition = "hide_and_retain"
		result.Warnings = append(result.Warnings, "System/acceptance collection is retained for reproducibility. Its available operations are determined from live evidence and capability coverage.")
	}
	return result
}

// ListForensicCasesV1Endpoint returns only collections the authenticated user
// can access. System/test collections remain hidden from the default analyst
// list, but an explicit include_system request may reveal the caller's own
// accessible collections. Collection access is still enforced by the agent pool.
// @Summary List governed forensic cases
// @Tags records
// @Produce json
// @Param include_system query bool false "Include accessible legacy/system/test collections"
// @Success 200 {object} map[string]any "governed case list"
// @Router /api/v1/forensics/cases [get]
func ListForensicCasesV1Endpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		if app == nil || app.AgentPoolService() == nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "agent pool collections are not available"})
		}
		collections, err := app.AgentPoolService().ListCollectionsForUser(effectiveUserID(c))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		includeSystem := strings.EqualFold(c.QueryParam("include_system"), "true")
		cases, hiddenCount, defaultCaseID := governedForensicCases(collections, includeSystem)
		return c.JSON(http.StatusOK, map[string]any{
			"contract_version": forensicCaseGovernanceContractV1,
			"cases":            cases,
			"count":            len(cases),
			"hidden_count":     hiddenCount,
			"default_case_id":  defaultCaseID,
			"generated_at":     time.Now().UTC(),
			"deletion_count":   0,
		})
	}
}

// governedForensicCases keeps discovery and default selection separate. An
// explicit include_system request may reveal retained test or cleanup
// collections, but it must never allow one of them to become the implicit case
// used by Records Intelligence or Agent Chat.
func governedForensicCases(collections []string, includeSystem bool) ([]forensicCaseV1, int, string) {
	ordered := append([]string(nil), collections...)
	sort.Strings(ordered)
	cases := make([]forensicCaseV1, 0, len(ordered))
	hiddenCount := 0
	for _, collection := range ordered {
		item := classifyForensicCase(collection)
		if item.Visibility == "system" && !includeSystem {
			hiddenCount++
			continue
		}
		cases = append(cases, item)
	}

	for _, preferred := range []string{"nexusai-forensic-demo", "records-demo-verified"} {
		for _, item := range cases {
			if item.CaseID == preferred && item.Visibility == "analyst" && item.Selectable {
				return cases, hiddenCount, item.CaseID
			}
		}
	}
	for _, item := range cases {
		if item.Visibility == "analyst" && item.Selectable {
			return cases, hiddenCount, item.CaseID
		}
	}
	return cases, hiddenCount, ""
}

// GetForensicCaseV1Endpoint returns governed metadata for one accessible case.
// @Summary Get a governed forensic case
// @Tags records
// @Produce json
// @Param case_id path string true "Case ID (identical to its bound collection ID in v1)"
// @Success 200 {object} forensicCaseV1 "governed case"
// @Router /api/v1/forensics/cases/{case_id} [get]
func GetForensicCaseV1Endpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID := strings.TrimSpace(decodedParam(c, "case_id"))
		if caseID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "case_id is required"})
		}
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, classifyForensicCase(caseID))
	}
}

// GetForensicCaseManifestV1Endpoint returns a zero-deletion reconciliation
// manifest merged from the sidecar and authenticated LocalAI-owned resources.
// @Summary Get a read-only forensic case reconciliation manifest
// @Tags records
// @Produce json
// @Param case_id path string true "Case ID"
// @Success 200 {object} map[string]any "case reconciliation manifest"
// @Router /api/v1/forensics/cases/{case_id}/manifest [get]
func GetForensicCaseManifestV1Endpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID := strings.TrimSpace(decodedParam(c, "case_id"))
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		status, payload, err := fetchForensicRecordsPayload(c, http.MethodGet, "/api/v1/forensics/cases/"+url.PathEscape(caseID)+"/manifest", nil, nil, caseID, caseID)
		if err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		if status != http.StatusOK {
			return c.JSONBlob(status, payload)
		}
		var manifest map[string]any
		if err := json.Unmarshal(payload, &manifest); err != nil {
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "sidecar returned an invalid case manifest"})
		}
		userID := effectiveUserID(c)
		entries, entriesErr := app.AgentPoolService().ListCollectionEntriesForUser(userID, caseID)
		sources, sourcesErr := app.AgentPoolService().ListCollectionSourcesForUser(userID, caseID)
		batches, batchesErr := recordsService(app).ListBatches(userID)
		batchCount := int64(0)
		if batchesErr == nil {
			for _, batch := range batches {
				if batch.CollectionName == caseID {
					batchCount++
				}
			}
		}
		agentBindings := []map[string]any{}
		for agentName := range app.AgentPoolService().ListAgentsForUser(userID) {
			cfg := app.AgentPoolService().GetNativeAgentConfigForUser(userID, agentName)
			if cfg != nil && cfg.EnableForensicRecords && strings.TrimSpace(cfg.ForensicCollectionID) == caseID {
				agentBindings = append(agentBindings, map[string]any{"agent_id": agentName, "role": "forensic_records", "configured_collection_id": cfg.ForensicCollectionID})
			}
		}
		sort.Slice(agentBindings, func(i, j int) bool {
			return fmt.Sprint(agentBindings[i]["agent_id"]) < fmt.Sprint(agentBindings[j]["agent_id"])
		})
		augmentManifestResource(manifest, "kb_entries", int64(len(entries)), entriesErr, "localai_kb")
		augmentManifestResource(manifest, "sources", int64(len(sources)), sourcesErr, "localai_kb")
		augmentManifestResource(manifest, "local_record_batches", batchCount, batchesErr, "localai_records")
		augmentManifestResource(manifest, "agents", int64(len(agentBindings)), nil, "localai_agent_pool")
		manifest["agent_bindings"] = agentBindings
		manifest["case_metadata"] = classifyForensicCase(caseID)
		manifest["deletion_count"] = 0
		manifest["read_only"] = true
		return c.JSON(http.StatusOK, manifest)
	}
}

func augmentManifestResource(manifest map[string]any, resource string, count int64, sourceErr error, source string) {
	items, _ := manifest["resources"].([]any)
	updated := false
	for _, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok || item["resource"] != resource {
			continue
		}
		if sourceErr == nil {
			item["count"] = count
			item["status"] = "counted"
			item["source"] = source
			delete(item, "note")
		} else {
			item["status"] = "unavailable"
			item["note"] = sourceErr.Error()
		}
		updated = true
		break
	}
	if !updated {
		item := map[string]any{"resource": resource, "source": source}
		if sourceErr == nil {
			item["count"] = count
			item["status"] = "counted"
		} else {
			item["count"] = nil
			item["status"] = "unavailable"
			item["note"] = sourceErr.Error()
		}
		manifest["resources"] = append(items, item)
	}
}

// QueryForensicCaseV1Endpoint makes the URL case authoritative and rejects a
// conflicting body before proxying to the sidecar compatibility adapter.
// @Summary Query a governed forensic case
// @Tags records
// @Accept json
// @Produce json
// @Param case_id path string true "Case ID"
// @Param request body map[string]any true "raw question or typed query plan"
// @Success 200 {object} map[string]any "forensics.enterprise-response/v1"
// @Router /api/v1/forensics/cases/{case_id}/query [post]
func QueryForensicCaseV1Endpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID := strings.TrimSpace(decodedParam(c, "case_id"))
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		var body map[string]any
		if err := json.NewDecoder(io.LimitReader(c.Request().Body, 1<<20)).Decode(&body); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := bindForensicCaseBodyScope(body, caseID); err != nil {
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return proxyForensicRecords(c, http.MethodPost, "/api/v1/forensics/cases/"+url.PathEscape(caseID)+"/query", nil, bytes.NewReader(payload), caseID, caseID)
	}
}

// ListForensicCaseEvidenceV1Endpoint is the URL-bound evidence catalog.
// @Summary List evidence for one governed case
// @Tags records
// @Produce json
// @Param case_id path string true "Case ID"
// @Router /api/v1/forensics/cases/{case_id}/evidence [get]
func ListForensicCaseEvidenceV1Endpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID := strings.TrimSpace(decodedParam(c, "case_id"))
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		values := url.Values{"collection_id": []string{caseID}, "case_id": []string{caseID}}
		for _, key := range []string{"tenant_id", "modality", "detected_type", "processing_status", "q", "limit", "offset"} {
			if value := strings.TrimSpace(c.QueryParam(key)); value != "" {
				values.Set(key, value)
			}
		}
		return proxyForensicRecords(c, http.MethodGet, "/evidence", values, nil, caseID, caseID)
	}
}

// GetForensicCaseEvidenceV1Endpoint returns one evidence item's immutable
// identity, processing history, accounting, linked assets, and bounded previews
// while keeping the URL case authoritative.
// @Summary Inspect evidence in one governed forensic case
// @Tags records
// @Produce json
// @Param case_id path string true "Case ID"
// @Param evidence_id path string true "Evidence ID"
// @Param limit query int false "Maximum canonical row previews"
// @Param include_records_preview query bool false "Include bounded canonical row previews"
// @Success 200 {object} map[string]any "case-scoped evidence detail"
// @Router /api/v1/forensics/cases/{case_id}/evidence/{evidence_id} [get]
func GetForensicCaseEvidenceV1Endpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID := strings.TrimSpace(decodedParam(c, "case_id"))
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		evidenceID := strings.TrimSpace(decodedParam(c, "evidence_id"))
		if evidenceID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "evidence_id is required"})
		}
		values := url.Values{"collection_id": []string{caseID}}
		for _, key := range []string{"tenant_id", "limit", "include_records_preview"} {
			if value := strings.TrimSpace(c.QueryParam(key)); value != "" {
				values.Set(key, value)
			}
		}
		return proxyForensicRecords(
			c,
			http.MethodGet,
			"/evidence/"+url.PathEscape(evidenceID),
			values,
			nil,
			caseID,
			caseID,
		)
	}
}

// GetForensicCaseFaceSimilarityV1Endpoint ranks explicitly bounded face
// candidates through the already-qualified forensic sidecar contract. The URL
// case supplies the authorization scope; the browser never receives the
// sidecar credential.
// @Summary Rank face candidates inside one governed case
// @Tags records
// @Produce json
// @Param case_id path string true "Case ID"
// @Param query_face_observation_id query string true "Query face observation UUID"
// @Param candidate_evidence_id query []string true "Explicit candidate evidence UUIDs"
// @Param top_k query int false "Maximum ranked candidates"
// @Success 200 {object} map[string]any "forensics.face-candidate-similarity/v1"
// @Router /api/v1/forensics/cases/{case_id}/faces/similar [get]
func GetForensicCaseFaceSimilarityV1Endpoint(app *application.Application) echo.HandlerFunc {
	return proxyForensicCaseMediaRead(app, "/faces/similar", []string{"tenant_id", "query_face_observation_id", "candidate_evidence_id", "top_k"}, true)
}

// GetForensicCaseImageSimilarityV1Endpoint ranks explicitly bounded semantic
// image candidates using retained embeddings. Similarity is not identity.
// @Summary Rank semantic image candidates inside one governed case
// @Tags records
// @Produce json
// @Param case_id path string true "Case ID"
// @Param query_image_observation_id query string true "Query image observation UUID"
// @Param candidate_evidence_id query []string true "Explicit candidate evidence UUIDs"
// @Param top_k query int false "Maximum ranked candidates"
// @Success 200 {object} map[string]any "forensics.semantic-image-candidate-similarity/v1"
// @Router /api/v1/forensics/cases/{case_id}/images/similar [get]
func GetForensicCaseImageSimilarityV1Endpoint(app *application.Application) echo.HandlerFunc {
	return proxyForensicCaseMediaRead(app, "/images/similar", []string{"tenant_id", "query_image_observation_id", "candidate_evidence_id", "top_k"}, true)
}

// CompareForensicCaseImagesV1Endpoint compares two explicit retained image
// sources without broadening beyond the governed URL case.
// @Summary Compare two retained images inside one governed case
// @Tags records
// @Produce json
// @Param case_id path string true "Case ID"
// @Param evidence_id_a query string true "First evidence UUID"
// @Param evidence_id_b query string true "Second evidence UUID"
// @Success 200 {object} map[string]any "forensics.image-comparison/v1"
// @Router /api/v1/forensics/cases/{case_id}/evidence/compare [get]
func CompareForensicCaseImagesV1Endpoint(app *application.Application) echo.HandlerFunc {
	return proxyForensicCaseMediaRead(app, "/evidence/compare", []string{"tenant_id", "evidence_id_a", "evidence_id_b"}, false)
}

func proxyForensicCaseMediaRead(app *application.Application, targetPath string, allowedQuery []string, explicitAuthorization bool) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID := strings.TrimSpace(decodedParam(c, "case_id"))
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		values := forensicCaseMediaQuery(c.QueryParams(), caseID, allowedQuery, explicitAuthorization)
		return proxyForensicRecords(c, http.MethodGet, targetPath, values, nil, caseID, caseID)
	}
}

func forensicCaseMediaQuery(input url.Values, caseID string, allowedQuery []string, explicitAuthorization bool) url.Values {
	values := url.Values{"collection_id": []string{caseID}, "case_id": []string{caseID}}
	for _, key := range allowedQuery {
		for _, value := range input[key] {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				values.Add(key, trimmed)
			}
		}
	}
	if explicitAuthorization {
		values.Set("authorization", "explicit_case_evidence_scope")
	}
	return values
}

// GetForensicCaseEvidenceContentV1Endpoint streams verified retained media for
// an authorized case. Model artifacts never replace these source bytes.
// @Summary Preview retained image, audio, or video evidence
// @Tags records
// @Produce application/octet-stream
// @Param case_id path string true "Case ID"
// @Param evidence_id path string true "Evidence ID"
// @Success 200 {file} binary "verified retained evidence"
// @Router /api/v1/forensics/cases/{case_id}/evidence/{evidence_id}/content [get]
func GetForensicCaseEvidenceContentV1Endpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID := strings.TrimSpace(decodedParam(c, "case_id"))
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		evidenceID := strings.TrimSpace(decodedParam(c, "evidence_id"))
		if evidenceID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "evidence_id is required"})
		}
		values := url.Values{"collection_id": []string{caseID}, "case_id": []string{caseID}}
		if tenantID := strings.TrimSpace(c.QueryParam("tenant_id")); tenantID != "" {
			values.Set("tenant_id", tenantID)
		}
		return proxyForensicMedia(
			c,
			"/evidence/"+url.PathEscape(evidenceID)+"/content",
			values,
			caseID,
			caseID,
		)
	}
}

// GetForensicCaseEvidenceReprocessPlanV1Endpoint returns a case-bound,
// non-executing reprocess plan. The URL case is the only accepted authority.
// @Summary Inspect governed evidence reprocess eligibility
// @Tags records
// @Produce json
// @Param case_id path string true "Case ID"
// @Param evidence_id path string true "Evidence ID"
// @Success 200 {object} map[string]any "read-only reprocess plan"
// @Router /api/v1/forensics/cases/{case_id}/evidence/{evidence_id}/reprocess-plan [get]
func GetForensicCaseEvidenceReprocessPlanV1Endpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID := strings.TrimSpace(decodedParam(c, "case_id"))
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		evidenceID := strings.TrimSpace(decodedParam(c, "evidence_id"))
		if evidenceID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "evidence_id is required"})
		}
		values := url.Values{"collection_id": []string{caseID}, "case_id": []string{caseID}}
		if tenantID := strings.TrimSpace(c.QueryParam("tenant_id")); tenantID != "" {
			values.Set("tenant_id", tenantID)
		}
		return proxyForensicRecords(c, http.MethodGet, "/evidence/"+url.PathEscape(evidenceID)+"/reprocess-plan", values, nil, caseID, caseID)
	}
}

// GenerateForensicCaseReportV1Endpoint binds generated reports to the URL case.
// @Summary Generate a bounded report for one governed case
// @Tags records
// @Accept json
// @Produce json
// @Param case_id path string true "Case ID"
// @Router /api/v1/forensics/cases/{case_id}/reports [post]
func GenerateForensicCaseReportV1Endpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		caseID := strings.TrimSpace(decodedParam(c, "case_id"))
		if err := requireForensicCollectionAccess(c, app, caseID); err != nil {
			return err
		}
		var body map[string]any
		if err := json.NewDecoder(io.LimitReader(c.Request().Body, 1<<20)).Decode(&body); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := bindForensicCaseBodyScope(body, caseID); err != nil {
			return c.JSON(http.StatusConflict, map[string]string{"error": err.Error()})
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return proxyForensicRecords(c, http.MethodPost, "/reports/generate", nil, bytes.NewReader(payload), caseID, caseID)
	}
}

// bindForensicCaseBodyScope validates the entire caller-supplied scope before
// applying the URL-authoritative compatibility mapping. Avoiding partial writes
// keeps rejected requests intact for deterministic auditing and tests.
func bindForensicCaseBodyScope(body map[string]any, caseID string) error {
	for _, key := range []string{"case_id", "collection_id"} {
		if supplied := strings.TrimSpace(fmt.Sprint(body[key])); body[key] != nil && supplied != "" && supplied != caseID {
			return fmt.Errorf("%s does not match URL case", key)
		}
	}
	body["case_id"] = caseID
	body["collection_id"] = caseID
	return nil
}

func fetchForensicRecordsPayload(c echo.Context, method, path string, query url.Values, body io.Reader, collectionID, caseID string) (int, []byte, error) {
	cfg := forensicRecordsSidecarConfigFromEnv()
	if !cfg.Enabled {
		return http.StatusServiceUnavailable, nil, fmt.Errorf("forensic records sidecar is not configured")
	}
	target, err := url.Parse(cfg.APIURL + path)
	if err != nil {
		return http.StatusServiceUnavailable, nil, fmt.Errorf("invalid forensic records API URL")
	}
	if query != nil {
		target.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(c.Request().Context(), method, target.String(), body)
	if err != nil {
		return http.StatusInternalServerError, nil, err
	}
	req.Header.Set("Accept", "application/json")
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	actorID, subjectID, actorRole, err := forensicForwardIdentity(c)
	if err != nil {
		return http.StatusServiceUnavailable, nil, err
	}
	req.Header.Set("X-Forensic-Tenant-ID", cfg.TenantID)
	req.Header.Set("X-Forensic-Actor-ID", actorID)
	req.Header.Set("X-Forensic-Subject-ID", subjectID)
	req.Header.Set("X-Forensic-Actor-Role", actorRole)
	req.Header.Set("X-Forensic-Collection-ID", strings.TrimSpace(collectionID))
	req.Header.Set("X-Forensic-Case-ID", strings.TrimSpace(caseID))
	requestID := strings.TrimSpace(c.Request().Header.Get("X-Request-ID"))
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return http.StatusBadGateway, nil, fmt.Errorf("forensic records sidecar request failed: %w", err)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return http.StatusBadGateway, nil, fmt.Errorf("read forensic records sidecar response: %w", err)
	}
	return resp.StatusCode, payload, nil
}
