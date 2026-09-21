package localai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/mudler/LocalAI/core/application"
	"github.com/mudler/LocalAI/core/services/records"
	"github.com/mudler/LocalAI/pkg/forensicpolicy"
	"github.com/mudler/LocalAI/pkg/httpclient"
)

func recordsService(app *application.Application) *records.Service {
	cfg := app.ApplicationConfig()
	return records.NewService(records.DefaultBaseDir(cfg.DataPath, cfg.AgentPool.StateDir, cfg.DynamicConfigsDir))
}

// IngestRecordsEndpoint ingests a structured records file.
// @Summary Ingest structured records
// @Description Upload CSV, TSV, JSON array, JSONL/NDJSON, or text/log records into the deterministic records store. If collection_name is supplied, the raw file is also uploaded to the matching Knowledge Base collection.
// @Tags records
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "Structured records file"
// @Param collection_name formData string false "Knowledge Base collection to store the raw evidence file in"
// @Param record_type formData string false "Record type override: cdr, anpr, ipdr, subscriber, tower_location, transaction, access_log, generic, or auto"
// @Success 200 {object} records.IngestResult "Ingest result"
// @Router /api/records/ingest [post]
func IngestRecordsEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		file, err := c.FormFile("file")
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "file required"})
		}
		src, err := file.Open()
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		defer src.Close()
		data, err := io.ReadAll(src)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}

		userID := effectiveUserID(c)
		collectionName := strings.TrimSpace(c.FormValue("collection_name"))
		resultWarnings := []string{}
		sourceEntry := ""
		if collectionName != "" {
			entry, err := uploadRecordEvidenceToCollection(app, userID, collectionName, file.Filename, data)
			if err != nil {
				resultWarnings = append(resultWarnings, "raw evidence upload to collection failed: "+err.Error())
			} else {
				sourceEntry = entry
			}
		}

		result, err := recordsService(app).Ingest(data, records.IngestOptions{
			UserID:         userID,
			CollectionName: collectionName,
			RecordType:     strings.TrimSpace(c.FormValue("record_type")),
			SourceFile:     file.Filename,
			SourceEntry:    sourceEntry,
		})
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		result.Warnings = append(result.Warnings, resultWarnings...)
		return c.JSON(http.StatusOK, result)
	}
}

func uploadRecordEvidenceToCollection(app *application.Application, userID, collectionName, filename string, data []byte) (string, error) {
	svc := app.AgentPoolService()
	if svc == nil {
		return "", echo.NewHTTPError(http.StatusServiceUnavailable, "agent pool collections are not available")
	}
	collections, err := svc.ListCollectionsForUser(userID)
	if err != nil {
		return "", err
	}
	if !slices.Contains(collections, collectionName) {
		if err := svc.CreateCollectionForUser(userID, collectionName); err != nil {
			return "", err
		}
	}
	return svc.UploadToCollectionForUser(userID, collectionName, filename, bytes.NewReader(data))
}

// ListRecordBatchesEndpoint lists ingested record batches.
// @Summary List record batches
// @Tags records
// @Produce json
// @Success 200 {object} map[string]any "batch list"
// @Router /api/records/batches [get]
func ListRecordBatchesEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID := effectiveUserID(c)
		batches, err := recordsService(app).ListBatches(userID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		collection := strings.TrimSpace(c.QueryParam("collection_name"))
		recordType := strings.TrimSpace(c.QueryParam("record_type"))
		if collection != "" || recordType != "" {
			filtered := batches[:0]
			for _, batch := range batches {
				if collection != "" && batch.CollectionName != collection {
					continue
				}
				if recordType != "" && batch.RecordType != recordType {
					continue
				}
				filtered = append(filtered, batch)
			}
			batches = filtered
		}
		return c.JSON(http.StatusOK, map[string]any{"batches": batches, "count": len(batches)})
	}
}

// GetRecordBatchEndpoint returns one record batch.
// @Summary Get a record batch
// @Tags records
// @Produce json
// @Param id path string true "Batch ID"
// @Success 200 {object} records.Batch "batch"
// @Router /api/records/batches/{id} [get]
func GetRecordBatchEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		batch, err := recordsService(app).GetBatch(effectiveUserID(c), c.Param("id"))
		if err != nil {
			return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, batch)
	}
}

// DeleteRecordBatchEndpoint deletes one record batch.
// @Summary Delete a record batch
// @Tags records
// @Produce json
// @Param id path string true "Batch ID"
// @Success 200 {object} map[string]string "status"
// @Router /api/records/batches/{id} [delete]
func DeleteRecordBatchEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		if err := recordsService(app).DeleteBatch(effectiveUserID(c), c.Param("id")); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

// QueryRecordsEndpoint queries exact structured records.
// @Summary Query structured records
// @Tags records
// @Accept json
// @Produce json
// @Param request body records.QueryRequest true "query"
// @Success 200 {object} records.QueryResponse "query result"
// @Router /api/records/query [post]
func QueryRecordsEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req records.QueryRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		resp, err := recordsService(app).Query(effectiveUserID(c), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	}
}

// AggregateRecordsEndpoint aggregates exact structured records.
// @Summary Aggregate structured records
// @Tags records
// @Accept json
// @Produce json
// @Param request body records.AggregateRequest true "aggregate query"
// @Success 200 {object} records.AggregateResponse "aggregate result"
// @Router /api/records/aggregate [post]
func AggregateRecordsEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req records.AggregateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		resp, err := recordsService(app).Aggregate(effectiveUserID(c), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	}
}

// CorrelateRecordsEndpoint correlates records by shared entities and time window.
// @Summary Correlate structured records
// @Tags records
// @Accept json
// @Produce json
// @Param request body records.CorrelateRequest true "correlation query"
// @Success 200 {object} records.CorrelateResponse "correlation result"
// @Router /api/records/correlate [post]
func CorrelateRecordsEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req records.CorrelateRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		resp, err := recordsService(app).Correlate(effectiveUserID(c), req)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	}
}

// GetRecordSchemaEndpoint returns aliases and observed schema for a record type.
// @Summary Get structured record schema
// @Tags records
// @Produce json
// @Param record_type path string true "Record type"
// @Success 200 {object} records.SchemaResponse "schema"
// @Router /api/records/schema/{record_type} [get]
func GetRecordSchemaEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		resp, err := recordsService(app).Schema(effectiveUserID(c), c.Param("record_type"))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, resp)
	}
}

type forensicRecordsSidecarConfig struct {
	Enabled  bool
	APIURL   string
	APIKey   string
	TenantID string
}

func forensicRecordsSidecarConfigFromEnv() forensicRecordsSidecarConfig {
	apiURL := strings.TrimRight(strings.TrimSpace(os.Getenv("FORENSIC_RECORDS_API_URL")), "/")
	if apiURL == "" {
		apiURL = strings.TrimRight(strings.TrimSpace(os.Getenv("LOCALAI_FORENSIC_RECORDS_API_URL")), "/")
	}
	return forensicRecordsSidecarConfig{
		Enabled: apiURL != "",
		APIURL:  apiURL,
		APIKey:  strings.TrimSpace(os.Getenv("FORENSIC_RECORDS_API_KEY")),
		TenantID: defaultString(
			strings.TrimSpace(os.Getenv("FORENSIC_RECORDS_TENANT_ID")), "default",
		),
	}
}

// GetForensicRecordsStatusEndpoint proxies collection ingest/asset status from
// the forensic records sidecar for the Records Intelligence UI.
// @Summary Get forensic records collection status
// @Tags records
// @Produce json
// @Param tenant_id query string false "Tenant ID, defaults to default"
// @Param collection_id query string true "Knowledge Base / records collection ID"
// @Param limit query int false "Maximum recent jobs to include"
// @Success 200 {object} map[string]any "sidecar collection status"
// @Router /api/records/forensic/status [get]
func GetForensicRecordsStatusEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		collectionID := strings.TrimSpace(c.QueryParam("collection_id"))
		if collectionID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "collection_id is required"})
		}
		if err := requireForensicCollectionAccess(c, app, collectionID); err != nil {
			return err
		}
		values := url.Values{}
		if tenantID := strings.TrimSpace(c.QueryParam("tenant_id")); tenantID != "" {
			values.Set("tenant_id", tenantID)
		}
		values.Set("collection_id", collectionID)
		if limit := strings.TrimSpace(c.QueryParam("limit")); limit != "" {
			values.Set("limit", limit)
		}
		return proxyForensicRecords(c, http.MethodGet, "/collections/status", values, nil, collectionID, "")
	}
}

// GetForensicRecordsTemplatesEndpoint proxies deterministic query templates
// from the forensic records sidecar.
// @Summary List forensic records query templates
// @Tags records
// @Produce json
// @Success 200 {object} map[string]any "sidecar query templates"
// @Router /api/records/forensic/templates [get]
func GetForensicRecordsTemplatesEndpoint() echo.HandlerFunc {
	return func(c echo.Context) error {
		return proxyForensicRecords(c, http.MethodGet, "/query/templates", nil, nil, "", "")
	}
}

// GetForensicRecordsCapabilitiesEndpoint returns collection-aware support for
// every registered forensic data family and operation.
// @Summary Get forensic records capability coverage
// @Tags records
// @Produce json
// @Param tenant_id query string false "Tenant ID, defaults to the trusted forensic tenant"
// @Param collection_id query string true "Knowledge Base / records collection ID"
// @Param case_id query string false "Authorized case filter"
// @Param evidence_id query string false "Selected evidence UUID; omitted for workspace readiness"
// @Param evidence_version_id query string false "Selected current version UUID"
// @Success 200 {object} map[string]any "collection-aware capability catalog"
// @Router /api/records/forensic/capabilities [get]
func GetForensicRecordsCapabilitiesEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		collectionID := strings.TrimSpace(c.QueryParam("collection_id"))
		if collectionID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "collection_id is required"})
		}
		if err := requireForensicCollectionAccess(c, app, collectionID); err != nil {
			return err
		}
		values := url.Values{"collection_id": []string{collectionID}}
		if tenantID := strings.TrimSpace(c.QueryParam("tenant_id")); tenantID != "" {
			values.Set("tenant_id", tenantID)
		}
		for _, key := range []string{"case_id", "evidence_id", "evidence_version_id"} {
			if value := strings.TrimSpace(c.QueryParam(key)); value != "" {
				values.Set(key, value)
			}
		}
		return proxyForensicRecords(c, http.MethodGet, "/query/capabilities", values, nil, collectionID, values.Get("case_id"))
	}
}

// ListForensicEvidenceEndpoint proxies the unified evidence registry from the
// forensic records sidecar.
// @Summary List forensic evidence
// @Tags records
// @Produce json
// @Param tenant_id query string false "Tenant ID, defaults to default"
// @Param collection_id query string true "Knowledge Base / records collection ID"
// @Param case_id query string false "Case ID"
// @Param modality query string false "Evidence modality"
// @Param detected_type query string false "Detected evidence type"
// @Param processing_status query string false "Processing status"
// @Param q query string false "Filename, hash, KB entry, or metadata search"
// @Param limit query int false "Maximum evidence items to include"
// @Param offset query int false "Pagination offset"
// @Success 200 {object} map[string]any "sidecar evidence catalog"
// @Router /api/records/forensic/evidence [get]
func ListForensicEvidenceEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		collectionID := strings.TrimSpace(c.QueryParam("collection_id"))
		if collectionID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "collection_id is required"})
		}
		if err := requireForensicCollectionAccess(c, app, collectionID); err != nil {
			return err
		}
		values := url.Values{}
		for _, key := range []string{"tenant_id", "collection_id", "case_id", "modality", "detected_type", "processing_status", "q", "limit", "offset"} {
			if value := strings.TrimSpace(c.QueryParam(key)); value != "" {
				values.Set(key, value)
			}
		}
		return proxyForensicRecords(c, http.MethodGet, "/evidence", values, nil, collectionID, values.Get("case_id"))
	}
}

// GetForensicEvidenceEndpoint proxies one unified evidence registry item,
// including linked ingest jobs, KB assets, row previews, and entity rollups.
// @Summary Get forensic evidence detail
// @Tags records
// @Produce json
// @Param id path string true "Evidence ID"
// @Param tenant_id query string false "Tenant ID, defaults to the trusted forensic tenant"
// @Param collection_id query string true "Knowledge Base / records collection ID"
// @Param limit query int false "Maximum canonical row previews"
// @Success 200 {object} map[string]any "sidecar evidence detail"
// @Router /api/records/forensic/evidence/{id} [get]
func GetForensicEvidenceEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		evidenceID := strings.TrimSpace(c.Param("id"))
		if evidenceID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "evidence id is required"})
		}
		collectionID := strings.TrimSpace(c.QueryParam("collection_id"))
		if collectionID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "collection_id is required"})
		}
		if err := requireForensicCollectionAccess(c, app, collectionID); err != nil {
			return err
		}
		values := url.Values{}
		if tenantID := strings.TrimSpace(c.QueryParam("tenant_id")); tenantID != "" {
			values.Set("tenant_id", tenantID)
		}
		values.Set("collection_id", collectionID)
		if limit := strings.TrimSpace(c.QueryParam("limit")); limit != "" {
			values.Set("limit", limit)
		}
		return proxyForensicRecords(c, http.MethodGet, "/evidence/"+url.PathEscape(evidenceID), values, nil, collectionID, "")
	}
}

// ReprocessForensicEvidenceEndpoint requests a new immutable processing job for
// existing evidence. Earlier jobs and outputs remain available.
// @Summary Reprocess forensic evidence
// @Description Create an idempotent reprocessing job without resetting or deleting prior processing history.
// @Tags records
// @Accept json
// @Produce json
// @Param id path string true "Evidence ID"
// @Param Idempotency-Key header string true "Stable 8-128 character request key"
// @Param request body map[string]any true "collection_id, reason, and optional max_attempts"
// @Success 202 {object} map[string]any "reprocessing job"
// @Router /api/records/forensic/evidence/{id}/reprocess [post]
func ReprocessForensicEvidenceEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		evidenceID := strings.TrimSpace(c.Param("id"))
		if evidenceID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "evidence id is required"})
		}
		var body map[string]any
		decoder := json.NewDecoder(io.LimitReader(c.Request().Body, 64<<10))
		if err := decoder.Decode(&body); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		collectionID, _ := body["collection_id"].(string)
		collectionID = strings.TrimSpace(collectionID)
		if collectionID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "collection_id is required"})
		}
		if err := requireForensicCollectionAccess(c, app, collectionID); err != nil {
			return err
		}
		key := strings.TrimSpace(c.Request().Header.Get("Idempotency-Key"))
		if key == "" {
			key, _ = body["idempotency_key"].(string)
			key = strings.TrimSpace(key)
		}
		if key == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Idempotency-Key is required"})
		}
		c.Request().Header.Set("Idempotency-Key", key)
		payload, err := json.Marshal(body)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		caseID, _ := body["case_id"].(string)
		return proxyForensicRecords(
			c, http.MethodPost,
			"/evidence/"+url.PathEscape(evidenceID)+"/reprocess",
			nil, bytes.NewReader(payload), collectionID, strings.TrimSpace(caseID),
		)
	}
}

// QueryForensicRecordsEndpoint proxies natural-language hybrid analytics to
// the forensic records sidecar.
// @Summary Query forensic records through the sidecar
// @Tags records
// @Accept json
// @Produce json
// @Param request body map[string]any true "hybrid forensic query"
// @Success 200 {object} map[string]any "sidecar hybrid query result"
// @Router /api/records/forensic/query [post]
func QueryForensicRecordsEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		var body map[string]any
		if err := json.NewDecoder(io.LimitReader(c.Request().Body, 1<<20)).Decode(&body); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		collectionID, _ := body["collection_id"].(string)
		collectionID = strings.TrimSpace(collectionID)
		if collectionID == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "collection_id is required"})
		}
		if err := requireForensicCollectionAccess(c, app, collectionID); err != nil {
			return err
		}
		payload, err := json.Marshal(body)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return proxyForensicRecords(c, http.MethodPost, "/query/hybrid", nil, bytes.NewReader(payload), strings.TrimSpace(collectionID), "")
	}
}

func requireForensicCollectionAccess(c echo.Context, app *application.Application, collectionID string) error {
	if app == nil || app.AgentPoolService() == nil {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "agent pool collections are not available")
	}
	collections, err := app.AgentPoolService().ListCollectionsForUser(effectiveUserID(c))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not verify collection access")
	}
	if !forensicCollectionAllowed(collections, collectionID) {
		return echo.NewHTTPError(http.StatusForbidden, "forensic collection access denied")
	}
	return nil
}

func forensicCollectionAllowed(collections []string, collectionID string) bool {
	return slices.Contains(collections, strings.TrimSpace(collectionID))
}

func forensicRecordsProxyTimeout(path string) time.Duration {
	cleanPath := strings.TrimSuffix(strings.TrimSpace(path), "/")
	if cleanPath == "/query/hybrid" || strings.HasSuffix(cleanPath, "/query") {
		// Share the agent's complete-interaction bound, including final fallback.
		return forensicpolicy.QueryTransportTimeout
	}
	return 30 * time.Second
}

func proxyForensicRecords(c echo.Context, method, path string, query url.Values, body io.Reader, collectionID, caseID string) error {
	cfg := forensicRecordsSidecarConfigFromEnv()
	if !cfg.Enabled {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{
			"error":   "forensic records sidecar is not configured",
			"enabled": false,
		})
	}
	target, err := url.Parse(cfg.APIURL + path)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "invalid forensic records API URL"})
	}
	if query != nil {
		target.RawQuery = query.Encode()
	}
	ctx := c.Request().Context()
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	req.Header.Set("Accept", "application/json")
	requestID := strings.TrimSpace(c.Request().Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	req.Header.Set("X-Request-ID", requestID)
	c.Response().Header().Set("X-Request-ID", requestID)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	if key := strings.TrimSpace(c.Request().Header.Get("Idempotency-Key")); key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	actorID, subjectID, actorRole, err := forensicForwardIdentity(c)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	}
	req.Header.Set("X-Forensic-Tenant-ID", cfg.TenantID)
	req.Header.Set("X-Forensic-Actor-ID", actorID)
	req.Header.Set("X-Forensic-Subject-ID", subjectID)
	req.Header.Set("X-Forensic-Actor-Role", actorRole)
	req.Header.Set("X-Forensic-Collection-ID", strings.TrimSpace(collectionID))
	req.Header.Set("X-Forensic-Case-ID", strings.TrimSpace(caseID))
	resp, err := httpclient.NewWithTimeout(forensicRecordsProxyTimeout(path)).Do(req)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("forensic records sidecar request failed: %v", err)})
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("read forensic records sidecar response: %v", err)})
	}
	if len(payload) == 0 {
		return c.NoContent(resp.StatusCode)
	}
	return c.JSONBlob(resp.StatusCode, payload)
}

func proxyForensicMedia(c echo.Context, path string, query url.Values, collectionID, caseID string) error {
	cfg := forensicRecordsSidecarConfigFromEnv()
	if !cfg.Enabled {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{"error": "forensic records sidecar is not configured", "enabled": false})
	}
	target, err := url.Parse(cfg.APIURL + path)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "invalid forensic records API URL"})
	}
	target.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(c.Request().Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	req.Header.Set("Accept", defaultString(c.Request().Header.Get("Accept"), "application/octet-stream"))
	if value := strings.TrimSpace(c.Request().Header.Get("Range")); value != "" {
		req.Header.Set("Range", value)
	}
	requestID := strings.TrimSpace(c.Request().Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	req.Header.Set("X-Request-ID", requestID)
	c.Response().Header().Set("X-Request-ID", requestID)
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	actorID, subjectID, actorRole, err := forensicForwardIdentity(c)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
	}
	req.Header.Set("X-Forensic-Tenant-ID", cfg.TenantID)
	req.Header.Set("X-Forensic-Actor-ID", actorID)
	req.Header.Set("X-Forensic-Subject-ID", subjectID)
	req.Header.Set("X-Forensic-Actor-Role", actorRole)
	req.Header.Set("X-Forensic-Collection-ID", strings.TrimSpace(collectionID))
	req.Header.Set("X-Forensic-Case-ID", strings.TrimSpace(caseID))
	resp, err := httpclient.New().Do(req)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": fmt.Sprintf("forensic media request failed: %v", err)})
	}
	defer resp.Body.Close()
	for _, name := range []string{"Accept-Ranges", "Cache-Control", "Content-Disposition", "Content-Length", "Content-Range", "Content-Type", "ETag", "X-Content-Type-Options", "X-Evidence-SHA256"} {
		if value := resp.Header.Get(name); value != "" {
			c.Response().Header().Set(name, value)
		}
	}
	c.Response().WriteHeader(resp.StatusCode)
	_, err = io.Copy(c.Response().Writer, io.LimitReader(resp.Body, (256<<20)+1))
	return err
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
