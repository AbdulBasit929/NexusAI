package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/httpclient"
	"github.com/nats-io/nats.go"
)

const ingestSubject = "forensic.records.ingest.requested"

type config struct {
	ListenAddr      string
	SpoolDir        string
	NATSURL         string
	KBMirrorEnabled bool
	LocalAIURL      string
	LocalAIAPIKey   string
	DatabaseURL     string
	SynthesisModel  string
}

type ingestJob struct {
	JobID              string            `json:"job_id"`
	EvidenceID         string            `json:"evidence_id,omitempty"`
	RequestID          string            `json:"request_id,omitempty"`
	TenantID           string            `json:"tenant_id"`
	UserID             string            `json:"user_id"`
	CollectionID       string            `json:"collection_id"`
	FileID             string            `json:"file_id"`
	SourceFile         string            `json:"source_file"`
	SpoolPath          string            `json:"spool_path"`
	SHA256             string            `json:"sha256"`
	RecordType         string            `json:"record_type"`
	DetectedRecordType string            `json:"detected_record_type"`
	Headers            []string          `json:"headers"`
	QueuedAt           time.Time         `json:"queued_at"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

type evidenceClassification struct {
	Modality         string
	DetectedType     string
	ProcessingRoute  string
	Confidence       float64
	Warnings         []string
	ProcessingStatus string
}

func main() {
	cfg := config{
		ListenAddr:      env("FORENSIC_API_ADDR", ":8091"),
		SpoolDir:        env("FORENSIC_SPOOL_DIR", "/data/forensic/spool"),
		NATSURL:         env("NATS_URL", nats.DefaultURL),
		KBMirrorEnabled: envBool("FORENSIC_KB_MIRROR_ENABLED", false),
		LocalAIURL:      strings.TrimRight(env("LOCALAI_KB_URL", ""), "/"),
		LocalAIAPIKey:   env("LOCALAI_API_KEY", ""),
		DatabaseURL:     env("FORENSIC_DATABASE_URL", ""),
		SynthesisModel:  env("FORENSIC_SYNTHESIS_MODEL", ""),
	}
	if err := os.MkdirAll(cfg.SpoolDir, 0o750); err != nil {
		slog.Error("create spool dir", "error", err)
		os.Exit(1)
	}
	nc, err := nats.Connect(cfg.NATSURL, nats.Name("forensic-records-webhook"))
	if err != nil {
		slog.Error("connect nats", "url", cfg.NATSURL, "error", err)
		os.Exit(1)
	}
	defer nc.Close()

	var db *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		db, err = pgxpool.New(ctx, cfg.DatabaseURL)
		cancel()
		if err != nil {
			slog.Error("connect postgres", "error", err)
			os.Exit(1)
		}
		defer db.Close()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /webhooks/records/upload", uploadHandler(cfg, nc, db))
	mux.HandleFunc("GET /collections/status", collectionStatusHandler(db))
	mux.HandleFunc("POST /collections/repair-assets", collectionRepairHandler(db))
	mux.HandleFunc("GET /evidence", evidenceListHandler(db))
	mux.HandleFunc("GET /evidence/{evidence_id}", evidenceDetailHandler(db))
	mux.HandleFunc("GET /query/templates", queryTemplatesHandler())
	mux.HandleFunc("POST /query/hybrid", hybridQueryHandler(cfg, db))
	mux.HandleFunc("POST /reports/generate", reportGenerateHandler(cfg, db))

	slog.Info("forensic records webhook listening", "addr", cfg.ListenAddr)
	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		slog.Error("http server failed", "error", err)
		os.Exit(1)
	}
}

func uploadHandler(cfg config, nc *nats.Conn, db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("parse multipart form: %w", err))
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			writeError(w, http.StatusBadRequest, errors.New("multipart field file is required"))
			return
		}
		defer file.Close()

		tenantID := defaultString(r.FormValue("tenant_id"), "default")
		requestID := requestIDFromHTTP(r)
		collectionID := strings.TrimSpace(r.FormValue("collection_id"))
		if collectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}
		sourceEntry := strings.TrimSpace(r.FormValue("source_entry"))
		skipKBMirror := envBoolValue(r.FormValue("skip_kb_mirror"), false)
		caseID := strings.TrimSpace(r.FormValue("case_id"))
		jobID := uuid.NewString()
		fileID := defaultString(r.FormValue("file_id"), uuid.NewString())
		sourceFile := filepath.Base(header.Filename)
		spoolPath := filepath.Join(cfg.SpoolDir, tenantID, collectionID, jobID+"-"+sourceFile)

		hash, headers, err := spoolUpload(file, spoolPath)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		force := envBoolValue(r.FormValue("force"), false)
		if !force && db != nil {
			existing, err := findExistingIngest(r.Context(), db, tenantID, collectionID, hash)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			if existing != nil {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":        "duplicate",
					"message":       "same file hash already exists in this tenant and collection; pass force=true to ingest another batch",
					"tenant_id":     tenantID,
					"collection_id": collectionID,
					"sha256":        hash,
					"existing":      existing,
				})
				return
			}
		}
		requestedType := strings.ToLower(strings.TrimSpace(r.FormValue("record_type")))
		if requestedType == "" || requestedType == "auto" {
			requestedType = detectRecordType(headers)
		}
		detectedType := detectRecordType(headers)
		job := ingestJob{
			JobID:              jobID,
			RequestID:          requestID,
			TenantID:           tenantID,
			UserID:             r.FormValue("user_id"),
			CollectionID:       collectionID,
			FileID:             fileID,
			SourceFile:         sourceFile,
			SpoolPath:          spoolPath,
			SHA256:             hash,
			RecordType:         requestedType,
			DetectedRecordType: detectedType,
			Headers:            headers,
			QueuedAt:           time.Now().UTC(),
			Metadata: map[string]string{
				"content_type": header.Header.Get("Content-Type"),
				"size_bytes":   fmt.Sprintf("%d", header.Size),
			},
		}
		if sourceEntry != "" {
			job.Metadata["kb_source_entry"] = sourceEntry
		}
		if caseID != "" {
			job.Metadata["case_id"] = caseID
		}
		if skipKBMirror {
			if sourceEntry != "" {
				job.Metadata["kb_mirror_status"] = "mirrored"
				job.Metadata["kb_mirror_mode"] = "existing_kb_entry"
			} else {
				job.Metadata["kb_mirror_status"] = "disabled"
				job.Metadata["kb_mirror_mode"] = "skipped_by_request"
			}
		} else if cfg.KBMirrorEnabled {
			sourceEntry, mirrorErr := mirrorToKnowledgeBase(r.Context(), cfg, collectionID, sourceFile, spoolPath, job.UserID)
			if mirrorErr != nil {
				job.Metadata["kb_mirror_status"] = "failed"
				job.Metadata["kb_mirror_error"] = mirrorErr.Error()
				slog.Warn("kb mirror failed", "job_id", job.JobID, "collection_id", collectionID, "error", mirrorErr)
			} else {
				job.Metadata["kb_mirror_status"] = "mirrored"
				job.Metadata["kb_source_entry"] = sourceEntry
			}
		} else {
			job.Metadata["kb_mirror_status"] = "disabled"
		}
		if db != nil {
			evidenceID, err := registerEvidenceItem(r.Context(), db, job, header.Filename, header.Size)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			job.EvidenceID = evidenceID
		}
		payload, err := json.Marshal(job)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		msg := nats.NewMsg(ingestSubject)
		msg.Data = payload
		msg.Header.Set("X-Request-ID", requestID)
		if err := nc.PublishMsg(msg); err != nil {
			markEvidenceFailed(r.Context(), db, job.EvidenceID, "publish ingest job: "+err.Error())
			writeError(w, http.StatusServiceUnavailable, fmt.Errorf("publish ingest job: %w", err))
			return
		}
		if err := nc.FlushTimeout(5 * time.Second); err != nil {
			markEvidenceFailed(r.Context(), db, job.EvidenceID, "flush ingest job: "+err.Error())
			writeError(w, http.StatusServiceUnavailable, fmt.Errorf("flush ingest job: %w", err))
			return
		}
		slog.Info("queued forensic records ingest", "job_id", job.JobID, "collection_id", job.CollectionID, "file", job.SourceFile, "record_type", job.RecordType)
		writeJSON(w, http.StatusAccepted, job)
	}
}

func classifyEvidenceItem(sourceFile, contentType, detectedRecordType string, headers []string) evidenceClassification {
	ext := strings.ToLower(filepath.Ext(sourceFile))
	recordType := normalize(detectedRecordType)
	classification := evidenceClassification{
		Modality:         "unknown",
		DetectedType:     "unknown",
		ProcessingRoute:  "manual_review",
		Confidence:       0.15,
		ProcessingStatus: "queued",
	}
	if contentType == "" {
		classification.Warnings = append(classification.Warnings, "content type was not supplied by the upload client")
	}
	switch ext {
	case ".csv", ".tsv", ".json", ".jsonl", ".ndjson", ".parquet":
		classification.Modality = "tabular"
		classification.DetectedType = "generic_tabular"
		classification.ProcessingRoute = "forensic_records_worker"
		classification.Confidence = 0.65
	case ".log":
		classification.Modality = "text"
		classification.DetectedType = "access_log"
		classification.ProcessingRoute = "forensic_records_worker"
		classification.Confidence = 0.6
	case ".txt", ".md":
		classification.Modality = "text"
		classification.DetectedType = "text"
		classification.ProcessingRoute = "kb_text_and_forensic_records_worker"
		classification.Confidence = 0.55
	case ".pdf":
		classification.Modality = "document"
		classification.DetectedType = "pdf"
		classification.ProcessingRoute = "kb_document_pipeline"
		classification.Confidence = 0.55
	case ".doc", ".docx", ".rtf", ".odt":
		classification.Modality = "document"
		classification.DetectedType = "document"
		classification.ProcessingRoute = "kb_document_pipeline"
		classification.Confidence = 0.5
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".tif", ".tiff":
		classification.Modality = "image"
		classification.DetectedType = "image"
		classification.ProcessingRoute = "image_metadata_pipeline"
		classification.Confidence = 0.5
	case ".wav", ".mp3", ".m4a", ".flac", ".ogg", ".aac":
		classification.Modality = "audio"
		classification.DetectedType = "audio"
		classification.ProcessingRoute = "audio_metadata_pipeline"
		classification.Confidence = 0.5
	}
	switch recordType {
	case "cdr", "ipdr", "anpr", "subscriber", "tower_location", "transaction", "access_log":
		classification.Modality = "structured_records"
		classification.DetectedType = recordType
		classification.ProcessingRoute = "forensic_records_worker"
		classification.Confidence = 0.9
	case "generic":
		if len(headers) > 0 && classification.DetectedType == "unknown" {
			classification.Modality = "structured_records"
			classification.DetectedType = "generic"
			classification.ProcessingRoute = "forensic_records_worker"
			classification.Confidence = 0.45
			classification.Warnings = append(classification.Warnings, "record schema did not match a specialized forensic adapter")
		}
	}
	return classification
}

func registerEvidenceItem(ctx context.Context, db *pgxpool.Pool, job ingestJob, originalFilename string, sizeBytes int64) (string, error) {
	if db == nil {
		return "", nil
	}
	evidenceID := uuid.NewString()
	classification := classifyEvidenceItem(job.SourceFile, job.Metadata["content_type"], job.DetectedRecordType, job.Headers)
	metadata := map[string]any{
		"request_id":            job.RequestID,
		"file_id":               job.FileID,
		"job_id":                job.JobID,
		"headers":               job.Headers,
		"requested_record_type": job.RecordType,
		"detected_record_type":  job.DetectedRecordType,
		"kb_mirror_status":      job.Metadata["kb_mirror_status"],
		"kb_mirror_mode":        job.Metadata["kb_mirror_mode"],
	}
	if errText := job.Metadata["kb_mirror_error"]; errText != "" {
		classification.Warnings = append(classification.Warnings, "kb mirror failed: "+errText)
		metadata["kb_mirror_error"] = errText
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("marshal evidence metadata: %w", err)
	}
	warningsJSON, err := json.Marshal(classification.Warnings)
	if err != nil {
		return "", fmt.Errorf("marshal evidence warnings: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err = db.Exec(ctx, `
INSERT INTO forensic.evidence_items (
  evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file,
  content_type, extension, size_bytes, sha256, modality, detected_type,
  classifier_confidence, processing_route, processing_status, raw_storage_ref,
  kb_entry_ref, metadata, warnings
)
VALUES (
  $1::uuid, $2, $3, NULLIF($4, ''), $5, $6, $7,
  NULLIF($8, ''), NULLIF($9, ''), $10, $11, $12, $13,
  $14, $15, $16, $17,
  NULLIF($18, ''), $19::jsonb, $20::jsonb
)`,
		evidenceID,
		job.TenantID,
		job.CollectionID,
		job.Metadata["case_id"],
		job.UserID,
		originalFilename,
		job.SourceFile,
		job.Metadata["content_type"],
		strings.TrimPrefix(strings.ToLower(filepath.Ext(job.SourceFile)), "."),
		sizeBytes,
		job.SHA256,
		classification.Modality,
		classification.DetectedType,
		classification.Confidence,
		classification.ProcessingRoute,
		classification.ProcessingStatus,
		job.SpoolPath,
		job.Metadata["kb_source_entry"],
		string(metadataJSON),
		string(warningsJSON),
	)
	if err != nil {
		return "", fmt.Errorf("register evidence item: %w", err)
	}
	return evidenceID, nil
}

func markEvidenceFailed(ctx context.Context, db *pgxpool.Pool, evidenceID, message string) {
	if db == nil || strings.TrimSpace(evidenceID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, _ = db.Exec(ctx, `
UPDATE forensic.evidence_items
SET processing_status='failed',
    errors = errors || jsonb_build_array(jsonb_build_object('message', $2, 'recorded_at', now()))
WHERE evidence_id=$1::uuid`, evidenceID, truncate(message, 4000))
}

func requestIDFromHTTP(r *http.Request) string {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return requestID
}

func findExistingIngest(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID, sha256 string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var (
		jobID      string
		evidenceID string
		sourceFile string
		status     string
		recordType string
		queuedAt   time.Time
	)
	err := db.QueryRow(ctx, `
SELECT id::text, coalesce(evidence_id::text, ''), source_file, status::text, record_type::text, queued_at
FROM forensic.records_ingest_jobs
WHERE tenant_id = $1 AND collection_id = $2 AND sha256 = $3
  AND status IN ('queued', 'running', 'completed')
ORDER BY queued_at DESC
LIMIT 1`, tenantID, collectionID, sha256).Scan(&jobID, &evidenceID, &sourceFile, &status, &recordType, &queuedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("check duplicate ingest: %w", err)
	}
	return map[string]any{
		"job_id":      jobID,
		"evidence_id": evidenceID,
		"source_file": sourceFile,
		"status":      status,
		"record_type": recordType,
		"queued_at":   queuedAt.UTC().Format(time.RFC3339),
	}, nil
}

func mirrorToKnowledgeBase(ctx context.Context, cfg config, collectionID, sourceFile, spoolPath, userID string) (string, error) {
	if cfg.LocalAIURL == "" {
		return "", errors.New("LOCALAI_KB_URL is required when FORENSIC_KB_MIRROR_ENABLED=true")
	}
	if err := ensureKBCollection(ctx, cfg, collectionID); err != nil {
		return "", err
	}

	file, err := os.Open(spoolPath)
	if err != nil {
		return "", fmt.Errorf("open spool for kb mirror: %w", err)
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", sourceFile)
	if err != nil {
		return "", fmt.Errorf("create kb multipart field: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return "", fmt.Errorf("copy kb multipart file: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close kb multipart body: %w", err)
	}

	uploadURL := cfg.LocalAIURL + "/api/agents/collections/" + url.PathEscape(collectionID) + "/upload"
	if userID != "" {
		values := url.Values{}
		values.Set("user_id", userID)
		uploadURL += "?" + values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, &body)
	if err != nil {
		return "", fmt.Errorf("create kb upload request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if cfg.LocalAIAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.LocalAIAPIKey)
	}
	resp, err := httpclient.NewWithTimeout(30 * time.Second).Do(req)
	if err != nil {
		return "", fmt.Errorf("kb upload request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return "", fmt.Errorf("kb upload failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}
	var result struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode kb upload response: %w", err)
	}
	return result.Key, nil
}

func ensureKBCollection(ctx context.Context, cfg config, collectionID string) error {
	body, _ := json.Marshal(map[string]string{"name": collectionID})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.LocalAIURL+"/api/agents/collections", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create kb collection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.LocalAIAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.LocalAIAPIKey)
	}
	resp, err := httpclient.NewWithTimeout(15 * time.Second).Do(req)
	if err != nil {
		return fmt.Errorf("create kb collection: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusConflict {
		return nil
	}
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("create kb collection failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(payload)))
}

func spoolUpload(file multipart.File, dst string) (string, []string, error) {
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return "", nil, fmt.Errorf("create spool parent: %w", err)
	}
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return "", nil, fmt.Errorf("create spool file: %w", err)
	}
	hasher := sha256.New()
	tee := io.TeeReader(file, hasher)
	if _, err := io.Copy(out, tee); err != nil {
		_ = out.Close()
		return "", nil, fmt.Errorf("write spool file: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", nil, fmt.Errorf("close spool file: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		return "", nil, fmt.Errorf("finalize spool file: %w", err)
	}
	headers, err := readCSVHeaders(dst)
	if err != nil {
		return "", nil, err
	}
	return hex.EncodeToString(hasher.Sum(nil)), headers, nil
}

func readCSVHeaders(path string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json", ".jsonl", ".ndjson":
		if headers, err := readJSONHeaders(path); err == nil && len(headers) > 0 {
			return headers, nil
		}
	case ".parquet":
		return []string{"parquet_payload"}, nil
	case ".txt", ".log":
		if headers, err := readTextHeaders(path); err == nil && len(headers) > 0 {
			return headers, nil
		}
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open spooled file for header detection: %w", err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	headers, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv headers: %w", err)
	}
	for i := range headers {
		headers[i] = strings.TrimSpace(headers[i])
	}
	return headers, nil
}

func readJSONHeaders(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open spooled json for header detection: %w", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var value any
		if err := json.Unmarshal([]byte(line), &value); err != nil {
			return nil, fmt.Errorf("read json headers: %w", err)
		}
		return objectHeaders(value), nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan json headers: %w", err)
	}
	return nil, errors.New("read json headers: empty file")
}

func objectHeaders(value any) []string {
	switch typed := value.(type) {
	case map[string]any:
		headers := make([]string, 0, len(typed))
		for key := range typed {
			headers = append(headers, key)
		}
		return headers
	case []any:
		if len(typed) > 0 {
			return objectHeaders(typed[0])
		}
	}
	return nil
}

func readTextHeaders(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open spooled text for header detection: %w", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		headers := make([]string, 0, len(parts))
		for _, part := range parts {
			key, _, ok := strings.Cut(part, "=")
			if !ok {
				key, _, ok = strings.Cut(part, ":")
			}
			if ok && strings.TrimSpace(key) != "" {
				headers = append(headers, strings.TrimSpace(key))
			}
		}
		if len(headers) > 0 {
			return headers, nil
		}
		return []string{"line"}, nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan text headers: %w", err)
	}
	return nil, errors.New("read text headers: empty file")
}

func detectRecordType(headers []string) string {
	seen := map[string]bool{}
	for _, header := range headers {
		seen[normalize(header)] = true
	}
	has := func(keys ...string) bool {
		for _, key := range keys {
			if seen[key] {
				return true
			}
		}
		return false
	}
	if has("source_number", "src_number", "from_number", "calling_number", "caller") &&
		has("target_number", "dst_number", "to_number", "called_number", "callee") &&
		has("timestamp", "call_time", "call_start_ts", "call_start_dt_tm", "call_start_time", "duration_seconds", "call_type") {
		return "cdr"
	}
	if has("msisdn") && has("call_start_dt_tm", "call_start_time") && has("call_dialed_num", "dialed_number") {
		return "cdr"
	}
	if has("plate", "plate_number", "license_plate", "registration_number") {
		return "anpr"
	}
	if has("source_ip", "src_ip", "ip_address") && has("destination_ip", "dst_ip", "dest_ip") {
		return "ipdr"
	}
	if has("msisdn", "phone_number", "subscriber_number") && has("cnic", "customer_id", "subscriber_id", "name") {
		return "subscriber"
	}
	if has("cell_id", "cell_site_id", "site_id") && has("lat", "latitude") && has("longitude", "lon", "lng") {
		return "tower_location"
	}
	if has("amount", "transaction_amount") && has("account", "account_number", "from_account", "sender") {
		return "transaction"
	}
	if has("ip", "source_ip", "src_ip") && has("user", "username", "user_id", "path", "url") {
		return "access_log"
	}
	return "generic"
}

func normalize(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	return envBoolValue(os.Getenv(key), fallback)
}

func envBoolValue(value string, fallback bool) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "yes" || value == "on"
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
