package localai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/mudler/LocalAI/core/application"
	"github.com/mudler/LocalAI/pkg/httpclient"
)

const (
	defaultCollectionSearchMaxResults      = 10
	maxCollectionSearchMaxResults          = 100
	defaultCollectionSourceIntervalMinutes = 60
)

func ListCollectionsEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := getUserID(c)
		cols, err := svc.ListCollectionsForUser(userID)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}

		resp := map[string]any{
			"collections": cols,
			"count":       len(cols),
		}

		// Admin cross-user aggregation
		if wantsAllUsers(c) {
			usm := svc.UserServicesManager()
			if usm != nil {
				userIDs, _ := usm.ListAllUserIDs()
				userGroups := map[string]any{}
				for _, uid := range userIDs {
					if uid == userID {
						continue
					}
					userCols, err := svc.ListCollectionsForUser(uid)
					if err != nil || len(userCols) == 0 {
						continue
					}
					userGroups[uid] = map[string]any{"collections": userCols}
				}
				if len(userGroups) > 0 {
					resp["user_groups"] = userGroups
				}
			}
		}

		return c.JSON(http.StatusOK, resp)
	}
}

func CreateCollectionEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := getUserID(c)
		var payload struct {
			Name string `json:"name"`
		}
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := svc.CreateCollectionForUser(userID, payload.Name); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusCreated, map[string]string{"status": "ok", "name": payload.Name})
	}
}

func UploadToCollectionEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		actorID, subjectID, actorRole, identityErr := forensicForwardIdentity(c)
		name := decodedParam(c, "name")
		file, err := c.FormFile("file")
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "file required"})
		}
		src, err := file.Open()
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		defer src.Close()
		key, err := svc.UploadToCollectionForUser(userID, name, file.Filename, src)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			if isEmbeddingDimensionMismatch(err) {
				return c.JSON(http.StatusConflict, map[string]string{
					"error": "embedding model dimensionality changed and the collection could not be migrated automatically; restart LocalAI to trigger re-embedding, or reset the collection",
				})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		resp := map[string]any{"status": "ok", "filename": file.Filename, "key": key}
		if forward := forensicRecordsKBUploadConfigFromEnv(); forward.Enabled && shouldForwardKBUploadRequestToForensicRecords(file.Filename, c.FormValue("skip_forensic_records")) {
			if identityErr != nil {
				resp["records_status"] = "failed"
				resp["evidence_status"] = "failed"
				resp["records_warning"] = identityErr.Error()
			} else if _, err := src.Seek(0, io.SeekStart); err != nil {
				resp["records_status"] = "skipped"
				resp["evidence_status"] = "skipped"
				resp["records_warning"] = "file could not be rewound for forensic evidence registration: " + err.Error()
			} else {
				result, err := forwardKBUploadToForensicRecords(c.Request().Context(), forward, name, subjectID, actorID, actorRole, file.Filename, file.Header.Get("Content-Type"), key, src, map[string]string{
					"record_type":             c.FormValue("record_type"),
					"declared_modality":       c.FormValue("declared_modality"),
					"evidence_role":           c.FormValue("evidence_role"),
					"parent_evidence_id":      c.FormValue("parent_evidence_id"),
					"case_id":                 c.FormValue("case_id"),
					"jurisdiction":            c.FormValue("jurisdiction"),
					"source_timezone":         c.FormValue("source_timezone"),
					"source_timezone_state":   c.FormValue("source_timezone_state"),
					"source_date_order":       c.FormValue("source_date_order"),
					"source_date_order_state": c.FormValue("source_date_order_state"),
					"asr_language":            c.FormValue("asr_language"),
				})
				if err != nil {
					resp["records_status"] = "failed"
					resp["evidence_status"] = "failed"
					resp["records_warning"] = err.Error()
				} else {
					status := forensicRecordsResultStatus(result)
					resp["records_status"] = status
					resp["evidence_status"] = status
					resp["records_ingest"] = result
					resp["evidence_registration"] = result
				}
			}
		}
		return c.JSON(http.StatusOK, resp)
	}
}

func forensicRecordsResultStatus(result map[string]any) string {
	if status, ok := result["status"].(string); ok && strings.TrimSpace(status) != "" {
		return strings.TrimSpace(status)
	}
	return "queued"
}

type forensicRecordsKBUploadConfig struct {
	Enabled       bool
	APIURL        string
	APIKey        string
	TenantID      string
	UploadTimeout time.Duration
}

func forensicRecordsKBUploadConfigFromEnv() forensicRecordsKBUploadConfig {
	apiURL := strings.TrimRight(strings.TrimSpace(os.Getenv("FORENSIC_RECORDS_API_URL")), "/")
	if apiURL == "" {
		apiURL = strings.TrimRight(strings.TrimSpace(os.Getenv("LOCALAI_FORENSIC_RECORDS_API_URL")), "/")
	}
	enabled := apiURL != "" && envBoolValueLocal(os.Getenv("FORENSIC_RECORDS_KB_UPLOAD_ENABLED"), true)
	return forensicRecordsKBUploadConfig{
		Enabled:       enabled,
		APIURL:        apiURL,
		APIKey:        strings.TrimSpace(os.Getenv("FORENSIC_RECORDS_API_KEY")),
		TenantID:      defaultStringLocal(os.Getenv("FORENSIC_RECORDS_TENANT_ID"), "default"),
		UploadTimeout: durationFromEnvLocal("FORENSIC_RECORDS_UPLOAD_TIMEOUT", 5*time.Minute),
	}
}

func shouldForwardKBUploadToForensicRecords(filename string) bool {
	base := strings.TrimSpace(filepath.Base(filename))
	return base != "" && base != "." && base != string(filepath.Separator)
}

func shouldForwardKBUploadRequestToForensicRecords(filename, skipValue string) bool {
	return !envBoolValueLocal(skipValue, false) && shouldForwardKBUploadToForensicRecords(filename)
}

func forwardKBUploadToForensicRecords(ctx context.Context, cfg forensicRecordsKBUploadConfig, collectionID, subjectID, actorID, actorRole, filename, contentType, sourceEntry string, src io.Reader, hints ...map[string]string) (map[string]any, error) {
	reader, pipeWriter := io.Pipe()
	writer := multipart.NewWriter(pipeWriter)
	fields := map[string]string{
		"tenant_id":      cfg.TenantID,
		"collection_id":  collectionID,
		"user_id":        subjectID,
		"record_type":    "auto",
		"skip_kb_mirror": "true",
		"source_entry":   sourceEntry,
	}
	if len(hints) > 0 {
		for _, key := range []string{
			"record_type", "declared_modality", "evidence_role", "parent_evidence_id",
			"case_id", "jurisdiction", "source_timezone", "source_timezone_state",
			"source_date_order", "source_date_order_state", "asr_language",
		} {
			if value := strings.TrimSpace(hints[0][key]); value != "" {
				fields[key] = value
			}
		}
	}
	go func() {
		for key, value := range fields {
			if strings.TrimSpace(value) == "" {
				continue
			}
			if err := writer.WriteField(key, value); err != nil {
				_ = pipeWriter.CloseWithError(fmt.Errorf("write forensic field %s: %w", key, err))
				return
			}
		}
		partHeader := make(textproto.MIMEHeader)
		partHeader.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": "file", "filename": filename}))
		if contentType = strings.TrimSpace(contentType); contentType != "" {
			partHeader.Set("Content-Type", contentType)
		}
		part, err := writer.CreatePart(partHeader)
		if err != nil {
			_ = pipeWriter.CloseWithError(fmt.Errorf("create forensic file field: %w", err))
			return
		}
		if _, err := io.Copy(part, src); err != nil {
			_ = pipeWriter.CloseWithError(fmt.Errorf("copy forensic file: %w", err))
			return
		}
		if err := writer.Close(); err != nil {
			_ = pipeWriter.CloseWithError(fmt.Errorf("close forensic multipart: %w", err))
			return
		}
		_ = pipeWriter.Close()
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.APIURL+"/webhooks/records/upload", reader)
	if err != nil {
		_ = reader.Close()
		return nil, fmt.Errorf("create forensic upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}
	req.Header.Set("X-Forensic-Tenant-ID", cfg.TenantID)
	req.Header.Set("X-Forensic-Actor-ID", strings.TrimSpace(actorID))
	req.Header.Set("X-Forensic-Subject-ID", strings.TrimSpace(subjectID))
	req.Header.Set("X-Forensic-Actor-Role", strings.TrimSpace(actorRole))
	req.Header.Set("X-Forensic-Collection-ID", strings.TrimSpace(collectionID))
	if len(hints) > 0 {
		req.Header.Set("X-Forensic-Case-ID", strings.TrimSpace(hints[0]["case_id"]))
	}
	timeout := cfg.UploadTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	resp, err := httpclient.NewWithTimeout(timeout).Do(req)
	if err != nil {
		return nil, fmt.Errorf("forensic upload request: %w", err)
	}
	defer resp.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("forensic upload failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("decode forensic upload response: %w", err)
	}
	return result, nil
}

func envBoolValueLocal(value string, fallback bool) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func defaultStringLocal(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func durationFromEnvLocal(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// isEmbeddingDimensionMismatch detects the pgvector-side error that bubbles up
// from LocalRecall when the configured embedding model returns vectors of a
// different dimensionality than the collection's vector column. LocalRecall
// migrates the column on startup; this guard only fires for edge cases the
// migration path doesn't cover (e.g. a model swapped at runtime), so we
// surface a 409 with an actionable message instead of an opaque 500.
func isEmbeddingDimensionMismatch(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLSTATE 22000") &&
		strings.Contains(msg, "expected ") &&
		strings.Contains(msg, " dimensions, not ")
}

func ListCollectionEntriesEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		entries, err := svc.ListCollectionEntriesForUser(userID, decodedParam(c, "name"))
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"entries": entries,
			"count":   len(entries),
		})
	}
}

func GetCollectionEntryContentEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		entryParam := c.Param("*")
		entry, err := url.PathUnescape(entryParam)
		if err != nil {
			entry = entryParam
		}
		content, chunkCount, err := svc.GetCollectionEntryContentForUser(userID, decodedParam(c, "name"), entry)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"content":     content,
			"chunk_count": chunkCount,
		})
	}
}

func SearchCollectionEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		var payload struct {
			Query      string `json:"query"`
			MaxResults int    `json:"max_results"`
		}
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		query, maxResults, err := normalizeCollectionSearchRequest(payload.Query, payload.MaxResults)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		name := decodedParam(c, "name")
		effectiveMaxResults := maxResults
		if entries, listErr := svc.ListCollectionEntriesForUser(userID, name); listErr == nil {
			if len(entries) == 0 {
				return c.JSON(http.StatusOK, map[string]any{
					"results":               []any{},
					"count":                 0,
					"requested_max_results": maxResults,
					"effective_max_results": 0,
				})
			}
			if indexedCount := estimateCollectionSearchDocumentCount(svc, userID, name, entries, effectiveMaxResults); indexedCount > 0 && effectiveMaxResults > indexedCount {
				effectiveMaxResults = indexedCount
			}
		}
		results, err := svc.SearchCollectionForUser(userID, name, query, effectiveMaxResults)
		if err != nil && effectiveMaxResults > 1 {
			if capped := capSearchResultsFromCollectionSizeError(err, effectiveMaxResults); capped > 0 && capped < effectiveMaxResults {
				effectiveMaxResults = capped
			} else if strings.Contains(err.Error(), "nResults must be") {
				effectiveMaxResults = 1
			}
			results, err = svc.SearchCollectionForUser(userID, name, query, effectiveMaxResults)
		}
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"results":               results,
			"count":                 len(results),
			"requested_max_results": maxResults,
			"effective_max_results": effectiveMaxResults,
		})
	}
}

func estimateCollectionSearchDocumentCount(svc interface {
	GetCollectionEntryContentForUser(string, string, string) (string, int, error)
}, userID, collection string, entries []string, requested int) int {
	if requested <= 0 {
		return 0
	}
	total := 0
	for _, entry := range entries {
		_, chunks, err := svc.GetCollectionEntryContentForUser(userID, collection, entry)
		if err != nil || chunks <= 0 {
			continue
		}
		total += chunks
		if total >= requested {
			return total
		}
	}
	return total
}

func capSearchResultsFromCollectionSizeError(err error, requested int) int {
	if err == nil {
		return 0
	}
	message := err.Error()
	if !strings.Contains(message, "nResults must be") && !strings.Contains(message, "number of documents") {
		return 0
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)nResults\s+must\s+be\s+<=\s+(?:the\s+)?(?:number\s+of\s+)?documents?\s+in\s+the\s+collection\s*\(?\s*(\d+)\s*\)?`),
		regexp.MustCompile(`(?i)(?:number\s+of\s+documents|documents?\s+in\s+the\s+collection|collection\s+contains)\D+(\d+)`),
	}
	for _, pattern := range patterns {
		match := pattern.FindStringSubmatch(message)
		if len(match) < 2 {
			continue
		}
		value, parseErr := strconv.Atoi(match[1])
		if parseErr == nil && value > 0 && value < requested {
			return value
		}
	}
	return 0
}

func ResetCollectionEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		if err := svc.ResetCollectionForUser(userID, decodedParam(c, "name")); err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

func DeleteCollectionEntryEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		var payload struct {
			Entry string `json:"entry"`
		}
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		remaining, err := svc.DeleteCollectionEntryForUser(userID, decodedParam(c, "name"), payload.Entry)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"remaining_entries": remaining,
			"count":             len(remaining),
		})
	}
}

func AddCollectionSourceEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		var payload struct {
			URL            string          `json:"url"`
			UpdateInterval json.RawMessage `json:"update_interval"`
		}
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		sourceURL := strings.TrimSpace(payload.URL)
		if sourceURL == "" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "url is required"})
		}
		intervalMinutes, err := parseCollectionSourceIntervalMinutes(payload.UpdateInterval)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := svc.AddCollectionSourceForUser(userID, decodedParam(c, "name"), sourceURL, intervalMinutes); err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

func normalizeCollectionSearchRequest(query string, maxResults int) (string, int, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", 0, fmt.Errorf("query is required")
	}
	if maxResults < 1 {
		maxResults = defaultCollectionSearchMaxResults
	}
	if maxResults > maxCollectionSearchMaxResults {
		maxResults = maxCollectionSearchMaxResults
	}
	return query, maxResults, nil
}

func parseCollectionSourceIntervalMinutes(raw json.RawMessage) (int, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "null" {
		return defaultCollectionSourceIntervalMinutes, nil
	}

	var minutes float64
	if err := json.Unmarshal(raw, &minutes); err == nil {
		return normalizeCollectionSourceIntervalMinutes(minutes)
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return parseCollectionSourceIntervalString(text)
	}

	return 0, fmt.Errorf("update_interval must be minutes or a duration like 30m or 1h")
}

func parseCollectionSourceIntervalString(value string) (int, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return defaultCollectionSourceIntervalMinutes, nil
	}

	if minutes, err := strconv.ParseFloat(value, 64); err == nil {
		return normalizeCollectionSourceIntervalMinutes(minutes)
	}

	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("update_interval must be minutes or a duration like 30m or 1h")
	}
	return normalizeCollectionSourceIntervalMinutes(duration.Minutes())
}

func normalizeCollectionSourceIntervalMinutes(minutes float64) (int, error) {
	if minutes < 1 {
		return defaultCollectionSourceIntervalMinutes, nil
	}
	if math.IsInf(minutes, 0) || math.IsNaN(minutes) {
		return 0, fmt.Errorf("update_interval must be a finite number of minutes")
	}
	return int(math.Ceil(minutes)), nil
}

func RemoveCollectionSourceEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		var payload struct {
			URL string `json:"url"`
		}
		if err := c.Bind(&payload); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := svc.RemoveCollectionSourceForUser(userID, decodedParam(c, "name"), payload.URL); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	}
}

// GetCollectionEntryRawFileEndpoint serves the original uploaded binary file.
func GetCollectionEntryRawFileEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		entryParam := c.Param("*")
		entry, err := url.PathUnescape(entryParam)
		if err != nil {
			entry = entryParam
		}
		fpath, err := svc.GetCollectionEntryFilePathForUser(userID, decodedParam(c, "name"), entry)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.File(fpath)
	}
}

func ListCollectionSourcesEndpoint(app *application.Application) echo.HandlerFunc {
	return func(c echo.Context) error {
		svc := app.AgentPoolService()
		userID := effectiveUserID(c)
		sources, err := svc.ListCollectionSourcesForUser(userID, decodedParam(c, "name"))
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return c.JSON(http.StatusNotFound, map[string]string{"error": err.Error()})
			}
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{
			"sources": sources,
			"count":   len(sources),
		})
	}
}
