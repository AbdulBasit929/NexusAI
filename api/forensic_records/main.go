package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/csv"
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
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/httpclient"
	"github.com/nats-io/nats.go"
)

const ingestSubject = "forensic.records.ingest.requested"

type config struct {
	ListenAddr                string
	SpoolDir                  string
	NATSURL                   string
	KBMirrorEnabled           bool
	LocalAIURL                string
	LocalAIAPIKey             string
	DatabaseURL               string
	SynthesisModel            string
	SynthesisTimeout          time.Duration
	SemanticPlannerTimeout    time.Duration
	SemanticEmbeddingModel    string
	SemanticEmbeddingTimeout  time.Duration
	SemanticEmbeddingsEnabled bool
	SemanticSlotAssistEnabled bool
	SemanticSlotAssistModel   string
	SemanticSlotAssistTimeout time.Duration
	QueryDB                   *pgxpool.Pool
	InboundAPIKey             string
	AuthRequired              bool
	TrustedTenantID           string
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

func decodeIngestMetadata(payload []byte) (map[string]string, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	metadata := make(map[string]string, len(raw))
	for key, value := range raw {
		if bytes.Equal(value, []byte("null")) {
			continue
		}
		var text string
		if err := json.Unmarshal(value, &text); err == nil {
			metadata[key] = text
			continue
		}
		metadata[key] = string(value)
	}
	return metadata, nil
}

type evidenceClassification struct {
	Modality         string
	DetectedType     string
	ProcessingRoute  string
	Confidence       float64
	Warnings         []string
	ProcessingStatus string
	StorageMode      string
	QueueRecords     bool
}

type evidenceRegistration struct {
	EvidenceID       string
	VersionID        string
	Created          bool
	Warnings         []string
	MediaMetadata    map[string]any
	ProcessingRoute  string
	ProcessingStatus string
	RoutePlan        EvidenceRoutePlan
}

func main() {
	cfg := config{
		ListenAddr:                env("FORENSIC_API_ADDR", ":8091"),
		SpoolDir:                  env("FORENSIC_SPOOL_DIR", "/data/forensic/spool"),
		NATSURL:                   env("NATS_URL", nats.DefaultURL),
		KBMirrorEnabled:           envBool("FORENSIC_KB_MIRROR_ENABLED", false),
		LocalAIURL:                strings.TrimRight(env("LOCALAI_KB_URL", ""), "/"),
		LocalAIAPIKey:             env("LOCALAI_API_KEY", ""),
		DatabaseURL:               env("FORENSIC_DATABASE_URL", ""),
		SynthesisModel:            env("FORENSIC_SYNTHESIS_MODEL", ""),
		SynthesisTimeout:          envDuration("FORENSIC_SYNTHESIS_TIMEOUT", 120*time.Second),
		SemanticPlannerTimeout:    envDuration("FORENSIC_SEMANTIC_PLANNER_TIMEOUT", 180*time.Second),
		SemanticEmbeddingModel:    env("FORENSIC_SEMANTIC_EMBEDDING_MODEL", semanticEmbeddingModelDefault),
		SemanticEmbeddingTimeout:  envDuration("FORENSIC_SEMANTIC_EMBEDDING_TIMEOUT", semanticEmbeddingTimeoutMax),
		SemanticEmbeddingsEnabled: envBool("FORENSIC_SEMANTIC_EMBEDDINGS_ENABLED", true),
		SemanticSlotAssistEnabled: envBool("FORENSIC_SEMANTIC_SLOT_ASSIST_ENABLED", false),
		SemanticSlotAssistModel:   env("FORENSIC_SEMANTIC_SLOT_ASSIST_MODEL", ""),
		SemanticSlotAssistTimeout: envDuration("FORENSIC_SEMANTIC_SLOT_ASSIST_TIMEOUT", semanticSlotAssistTimeoutMax),
		InboundAPIKey:             env("FORENSIC_API_KEY", env("FORENSIC_RECORDS_API_KEY", "")),
		AuthRequired:              envBool("FORENSIC_API_AUTH_REQUIRED", false),
		TrustedTenantID:           env("FORENSIC_TRUSTED_TENANT_ID", "default"),
	}
	authConfig := forensicAuthConfig{
		APIKey: cfg.InboundAPIKey, Required: cfg.AuthRequired,
		TrustedTenantID: cfg.TrustedTenantID,
	}
	if err := validateForensicAuthConfig(authConfig); err != nil {
		slog.Error("invalid forensic API authentication configuration", "error", err)
		os.Exit(1)
	}
	if cfg.InboundAPIKey == "" {
		slog.Warn("forensic API service authentication is disabled; configure FORENSIC_API_KEY and enable FORENSIC_API_AUTH_REQUIRED before protected deployment")
	}
	if err := os.MkdirAll(cfg.SpoolDir, 0o750); err != nil {
		slog.Error("create spool dir", "error", err)
		os.Exit(1)
	}
	// Repopulate the plan cache before serving. A cache that silently fails to
	// load is indistinguishable from one that is working, and its whole purpose
	// is a latency property someone will check -- so it says what it restored.
	// Off unless FORENSIC_PLAN_CACHE and FORENSIC_PLAN_CACHE_DIR are both set.
	if dir := semanticPlanCacheDir(); dir != "" {
		loaded, skipped := loadSemanticPlanCache(dir)
		slog.Info("restored semantic plan cache", "dir", dir, "entries", loaded,
			"skipped_unusable", skipped, "bound", semanticPlanCacheMax)
	}
	evidenceStore, err := newContentAddressedStore(cfg.SpoolDir)
	if err != nil {
		slog.Error("initialize content-addressed evidence store", "error", err)
		os.Exit(1)
	}
	nc, err := nats.Connect(cfg.NATSURL, nats.Name("forensic-records-webhook"))
	if err != nil {
		slog.Error("connect nats", "url", cfg.NATSURL, "error", err)
		os.Exit(1)
	}
	defer nc.Close()
	publisher, err := newJetStreamIngestPublisher(nc)
	if err != nil {
		slog.Error("initialize durable forensic queue", "error", err)
		os.Exit(1)
	}

	var db *pgxpool.Pool
	if cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		db, err = pgxpool.New(ctx, cfg.DatabaseURL)
		if err == nil {
			err = validateQueueSchema(ctx, db)
		}
		cancel()
		if err != nil {
			if db != nil {
				db.Close()
			}
			slog.Error("initialize postgres queue control plane", "error", err)
			os.Exit(1)
		}
		defer db.Close()
	}
	queueCtx, stopQueueOutbox := context.WithCancel(context.Background())
	defer stopQueueOutbox()
	if db != nil {
		go runQueueOutbox(queueCtx, db, publisher, cfg.TrustedTenantID)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /webhooks/records/upload", uploadHandler(cfg, evidenceStore, publisher, db))
	mux.HandleFunc("GET /collections/status", collectionStatusHandler(db))
	mux.HandleFunc("POST /collections/repair-assets", collectionRepairHandler(db))
	mux.HandleFunc("GET /evidence", evidenceListHandler(db))
	mux.HandleFunc("GET /evidence/compare", imageComparisonHandler(db))
	mux.HandleFunc("GET /faces/similar", faceSimilarityHandler(db))
	mux.HandleFunc("GET /images/similar", imageSimilarityHandler(db))
	mux.HandleFunc("GET /evidence/{evidence_id}", evidenceDetailHandler(db))
	mux.HandleFunc("GET /evidence/{evidence_id}/content", evidenceContentHandler(db, evidenceStore))
	mux.HandleFunc("GET /evidence/{evidence_id}/reprocess-plan", evidenceReprocessPlanHandler(db))
	mux.HandleFunc("POST /evidence/{evidence_id}/reprocess", evidenceReprocessHandler(db, publisher))
	mux.HandleFunc("GET /query/templates", queryTemplatesHandler())
	mux.HandleFunc("GET /query/capabilities", forensicCapabilitiesHandler(db))
	mux.HandleFunc("POST /query/hybrid", hybridQueryHandler(cfg, db))
	mux.HandleFunc("GET /api/v1/forensics/adapters", forensicPlatformDiscoveryHandler("adapters"))
	mux.HandleFunc("GET /api/v1/forensics/operations", forensicPlatformDiscoveryHandler("operations"))
	mux.HandleFunc("GET /api/v1/forensics/agents", forensicPlatformDiscoveryHandler("agents"))
	mux.HandleFunc("GET /api/v1/forensics/contracts", forensicPlatformDiscoveryHandler("contracts"))
	mux.HandleFunc("GET /api/v1/forensics/cases/{case_id}/manifest", forensicCaseManifestHandler(db))
	mux.HandleFunc("POST /api/v1/forensics/cases/{case_id}/query", forensicCaseQueryV1Handler(cfg, db))
	mux.HandleFunc("POST /reports/generate", reportGenerateHandler(cfg, db))

	handler := forensicAuthMiddleware(authConfig, mux)
	slog.Info("forensic records webhook listening", "addr", cfg.ListenAddr, "auth_enabled", cfg.InboundAPIKey != "")
	if err := http.ListenAndServe(cfg.ListenAddr, handler); err != nil {
		slog.Error("http server failed", "error", err)
		os.Exit(1)
	}
}

func uploadHandler(cfg config, evidenceStore contentAddressedStore, publisher ingestPublisher, db *pgxpool.Pool) http.HandlerFunc {
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
		if imageUploadExceedsLimit(header.Filename, header.Size) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Errorf("image exceeds the %d-byte intake limit", maxImageEvidenceBytes))
			return
		}

		tenantID := strings.TrimSpace(r.FormValue("tenant_id"))
		requestID := requestIDFromHTTP(r)
		collectionID := strings.TrimSpace(r.FormValue("collection_id"))
		sourceEntry := strings.TrimSpace(r.FormValue("source_entry"))
		skipKBMirror := envBoolValue(r.FormValue("skip_kb_mirror"), false)
		caseID := strings.TrimSpace(r.FormValue("case_id"))
		scope, err := bindForensicScope(r, tenantID, collectionID, caseID, r.FormValue("user_id"))
		if err != nil {
			writeScopeError(w, err)
			return
		}
		tenantID, collectionID, caseID = scope.TenantID, scope.CollectionID, scope.CaseID
		if collectionID == "" {
			writeError(w, http.StatusBadRequest, errors.New("collection_id is required"))
			return
		}
		timeMetadata, err := sourceTimeMetadata(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		asrLanguage, err := requestedASRLanguage(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		jobID := uuid.NewString()
		fileID := defaultString(r.FormValue("file_id"), defaultString(sourceEntry, uuid.NewString()))
		sourceFile := filepath.Base(header.Filename)
		storedObject, err := evidenceStore.Retain(file, tenantID, collectionID)
		if err != nil {
			var retentionErr *evidenceRetentionError
			if errors.As(err, &retentionErr) {
				slog.Error("evidence retention integrity conflict", "request_id", requestID, "tenant_id", tenantID, "collection_id", collectionID, "quarantine_path", retentionErr.quarantinePath, "error", retentionErr)
			}
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		spoolPath := storedObject.Path
		hash := storedObject.SHA256
		headers, headerErr := readEvidenceHeaders(spoolPath, sourceFile)
		headerWarning := ""
		if headerErr != nil {
			headerWarning = "header detection deferred: " + headerErr.Error()
		}
		declaredContentType := strings.TrimSpace(header.Header.Get("Content-Type"))
		detectedContentType, sniffErr := sniffEvidenceContentType(spoolPath)
		contentType := declaredContentType
		if contentType == "" || contentType == "application/octet-stream" {
			contentType = detectedContentType
		}
		force := envBoolValue(r.FormValue("force"), false)
		if !force && db != nil {
			existing, err := findExistingEvidence(r.Context(), db, tenantID, collectionID, hash)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			if existing != nil && sourceEntry == "" {
				writeJSON(w, http.StatusOK, map[string]any{
					"status":        "duplicate",
					"message":       "same evidence hash already exists in this tenant and collection; pass force=true to create another evidence item",
					"tenant_id":     tenantID,
					"collection_id": collectionID,
					"sha256":        hash,
					"storage_uri":   storedObject.StorageURI,
					"existing":      existing,
				})
				return
			}
			if existing == nil {
				existing, err = findExistingIngest(r.Context(), db, tenantID, collectionID, hash)
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
						"storage_uri":   storedObject.StorageURI,
						"existing":      existing,
					})
					return
				}
			}
		}
		requestedTypeInput := strings.ToLower(strings.TrimSpace(r.FormValue("record_type")))
		requestedType, recordTypeMode := resolveRequestedRecordType(requestedTypeInput, headers)
		detectedType := detectRecordType(headers)
		job := ingestJob{
			JobID:              jobID,
			RequestID:          requestID,
			TenantID:           tenantID,
			UserID:             scope.SubjectID,
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
				"record_type_mode":               recordTypeMode,
				"requested_record_type_input":    requestedTypeInput,
				"content_type":                   contentType,
				"declared_content_type":          declaredContentType,
				"detected_content_type":          detectedContentType,
				"size_bytes":                     fmt.Sprintf("%d", storedObject.SizeBytes),
				"declared_modality":              strings.TrimSpace(r.FormValue("declared_modality")),
				"evidence_role":                  defaultString(r.FormValue("evidence_role"), "source"),
				"parent_evidence_id":             strings.TrimSpace(r.FormValue("parent_evidence_id")),
				"jurisdiction":                   defaultString(r.FormValue("jurisdiction"), defaultString(os.Getenv("FORENSIC_DEFAULT_JURISDICTION"), "PK")),
				"source_timezone":                timeMetadata["source_timezone"],
				"source_timezone_state":          timeMetadata["source_timezone_state"],
				"source_date_order":              timeMetadata["source_date_order"],
				"source_date_order_state":        timeMetadata["source_date_order_state"],
				"allow_profile_timezone_default": timeMetadata["allow_profile_timezone_default"],
				"asr_language":                   asrLanguage,
				"authenticated_actor_id":         scope.ActorID,
				"authenticated_actor_role":       scope.ActorRole,
				"request_id":                     requestID,
				"storage_uri":                    storedObject.StorageURI,
				"storage_receipt_uri":            storedObject.ReceiptURI,
				"storage_layout_version":         storedObject.LayoutVersion,
				"storage_scope_key":              storedObject.ScopeKey,
				"storage_integrity_state":        "verified",
				"storage_verified_at":            storedObject.VerifiedAt.Format(time.RFC3339Nano),
				"storage_retained_at":            storedObject.RetainedAt.Format(time.RFC3339Nano),
				"storage_verification_method":    storedObject.VerificationMethod,
				"storage_write_once":             fmt.Sprintf("%t", storedObject.WriteOnce),
				"storage_object_reused":          fmt.Sprintf("%t", storedObject.Reused),
			},
		}
		if headerWarning != "" {
			job.Metadata["header_detection_warning"] = headerWarning
		}
		if sniffErr != nil {
			job.Metadata["content_detection_warning"] = sniffErr.Error()
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
		classificationRecordType := effectiveClassificationRecordType(job.RecordType, job.DetectedRecordType, recordTypeMode)
		classification := finalizeEvidenceClassification(classifyEvidenceItem(
			job.SourceFile,
			job.Metadata["content_type"],
			classificationRecordType,
			job.Headers,
			job.Metadata,
		), job.Metadata["kb_source_entry"])
		retainASRLanguageForModality(job.Metadata, classification.Modality)
		requestedRoles := strings.FieldsFunc(r.FormValue("analysis_roles"), func(character rune) bool {
			return character == ',' || character == ';' || character == ' '
		})
		routePlan, routePlanErr := buildEvidenceRoutePlanFromPath(spoolPath, evidenceRouteInput{
			EvidenceID:      job.FileID,
			VersionID:       job.SHA256,
			SourceFilename:  job.SourceFile,
			DeclaredMIME:    declaredContentType,
			DetectedMIME:    detectedContentType,
			ScopeAuthorized: true,
			RequestedRoles:  requestedRoles,
			Classification:  classification,
		})
		if routePlanErr != nil {
			classification.Warnings = append(classification.Warnings, "route plan deferred: "+routePlanErr.Error())
		} else {
			recordEvidenceRoutePlanMetadata(job.Metadata, routePlan)
		}
		for _, warningKey := range []string{"header_detection_warning", "content_detection_warning"} {
			if warning := strings.TrimSpace(job.Metadata[warningKey]); warning != "" {
				classification.Warnings = append(classification.Warnings, warning)
			}
		}
		job.Metadata["evidence_modality"] = classification.Modality
		job.Metadata["evidence_detected_type"] = classification.DetectedType
		job.Metadata["evidence_processing_route"] = classification.ProcessingRoute
		job.Metadata["evidence_storage_mode"] = classification.StorageMode

		registration := evidenceRegistration{Created: true}
		if db != nil {
			registration, err = registerEvidenceItem(r.Context(), db, job, header.Filename, storedObject.SizeBytes, classification, force)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			classification = applyEvidenceRegistrationWarnings(classification, registration)
			job.EvidenceID = registration.EvidenceID
			if !registration.Created {
				sourceEntryLinked := false
				if sourceEntry != "" {
					if err := materializeRegisteredKBAsset(r.Context(), db, job, registration, classification, storedObject.SizeBytes); err != nil {
						writeError(w, http.StatusInternalServerError, err)
						return
					}
					sourceEntryLinked = true
				}
				writeJSON(w, http.StatusOK, map[string]any{
					"status":                "duplicate",
					"message":               duplicateEvidenceMessage(sourceEntryLinked),
					"tenant_id":             tenantID,
					"collection_id":         collectionID,
					"sha256":                hash,
					"storage_uri":           storedObject.StorageURI,
					"storage_object_reused": storedObject.Reused,
					"evidence_id":           registration.EvidenceID,
					"evidence_version_id":   registration.VersionID,
					"source_entry":          sourceEntry,
					"source_entry_linked":   sourceEntryLinked,
				})
				return
			}
		}
		if !classification.QueueRecords {
			if db != nil {
				if err := materializeRegisteredKBAsset(r.Context(), db, job, registration, classification, storedObject.SizeBytes); err != nil {
					markEvidenceFailed(r.Context(), db, job.TenantID, job.EvidenceID, "register KB evidence asset: "+err.Error())
					writeError(w, http.StatusInternalServerError, err)
					return
				}
			}
			slog.Info("registered non-record forensic evidence", "evidence_id", job.EvidenceID, "collection_id", job.CollectionID, "file", job.SourceFile, "modality", classification.Modality, "route", classification.ProcessingRoute)
			writeJSON(w, http.StatusOK, map[string]any{
				"status":                "registered",
				"tenant_id":             job.TenantID,
				"collection_id":         job.CollectionID,
				"evidence_id":           job.EvidenceID,
				"evidence_version_id":   registration.VersionID,
				"sha256":                job.SHA256,
				"storage_uri":           storedObject.StorageURI,
				"storage_object_reused": storedObject.Reused,
				"source_file":           job.SourceFile,
				"source_entry":          job.Metadata["kb_source_entry"],
				"modality":              classification.Modality,
				"detected_type":         classification.DetectedType,
				"processing_route":      classification.ProcessingRoute,
				"processing_status":     classification.ProcessingStatus,
				"media_metadata":        registration.MediaMetadata,
				"route_plan":            firstEvidenceRoutePlan(registration.RoutePlan, routePlan),
				"warnings":              classification.Warnings,
			})
			return
		}
		if db != nil {
			published, err := publishQueuedJob(r.Context(), db, publisher, job.TenantID, job.JobID)
			if err != nil {
				slog.Warn("ingest job retained for outbox retry", "job_id", job.JobID, "error", err)
				writeJSON(w, http.StatusAccepted, map[string]any{
					"status":  "publication_pending",
					"job":     job,
					"message": "job is durably registered and will be republished by the queue outbox",
				})
				return
			}
			if !published {
				writeJSON(w, http.StatusAccepted, map[string]any{"status": "publication_pending", "job": job})
				return
			}
		} else if err := publisher.Publish(r.Context(), job); err != nil {
			writeError(w, http.StatusServiceUnavailable, fmt.Errorf("publish ingest job: %w", err))
			return
		}
		slog.Info("queued forensic records ingest", "job_id", job.JobID, "collection_id", job.CollectionID, "file", job.SourceFile, "record_type", job.RecordType)
		writeJSON(w, http.StatusAccepted, job)
	}
}

func resolveRequestedRecordType(input string, headers []string) (string, string) {
	requested := strings.ToLower(strings.TrimSpace(input))
	if requested == "" || requested == "auto" {
		return detectRecordType(headers), "auto"
	}
	return requested, "explicit"
}

func effectiveClassificationRecordType(requested, detected, mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "explicit") {
		return normalize(requested)
	}
	return normalize(detected)
}

func imageUploadExceedsLimit(filename string, sizeBytes int64) bool {
	profile, _ := evidenceProfileForExtension(strings.ToLower(filepath.Ext(filename)))
	return profile.Modality == "image" && sizeBytes > maxImageEvidenceBytes
}

func sourceTimeMetadata(r *http.Request) (map[string]string, error) {
	timezoneName := strings.TrimSpace(r.FormValue("source_timezone"))
	timezoneState := normalize(r.FormValue("source_timezone_state"))
	if timezoneName == "" {
		timezoneState = "unknown"
	} else if timezoneState == "" {
		timezoneState = "analyst_confirmed"
	}
	if timezoneState != "unknown" && timezoneState != "source_declared" && timezoneState != "analyst_confirmed" && timezoneState != "profile_defaulted" {
		return nil, errors.New("source_timezone_state must be source_declared, analyst_confirmed, profile_defaulted, or unknown")
	}
	dateOrder := strings.ToUpper(strings.TrimSpace(r.FormValue("source_date_order")))
	dateOrderState := normalize(r.FormValue("source_date_order_state"))
	if dateOrder == "" {
		dateOrderState = "unresolved"
	} else if dateOrderState == "" {
		dateOrderState = "analyst_confirmed"
	}
	if dateOrder != "" && dateOrder != "DMY" && dateOrder != "MDY" && dateOrder != "YMD" {
		return nil, errors.New("source_date_order must be DMY, MDY, YMD, or empty")
	}
	if dateOrderState != "unresolved" && dateOrderState != "source_declared" && dateOrderState != "analyst_confirmed" && dateOrderState != "profile_defaulted" {
		return nil, errors.New("source_date_order_state must be source_declared, analyst_confirmed, profile_defaulted, or unresolved")
	}
	allowProfileDefault := "false"
	if timezoneState == "profile_defaulted" {
		allowProfileDefault = "true"
	}
	return map[string]string{
		"source_timezone": timezoneName, "source_timezone_state": timezoneState,
		"source_date_order": dateOrder, "source_date_order_state": dateOrderState,
		"allow_profile_timezone_default": allowProfileDefault,
	}, nil
}

func requestedASRLanguage(r *http.Request) (string, error) {
	language := strings.ToLower(strings.TrimSpace(r.FormValue("asr_language")))
	if language == "" {
		return "", nil
	}
	if len(language) < 2 || len(language) > 3 {
		return "", errors.New("asr_language must be a two- or three-letter language code")
	}
	for _, character := range language {
		if character < 'a' || character > 'z' {
			return "", errors.New("asr_language must be a two- or three-letter language code")
		}
	}
	return language, nil
}

func retainASRLanguageForModality(metadata map[string]string, modality string) {
	if normalize(modality) != "audio" && normalize(modality) != "video" {
		delete(metadata, "asr_language")
	}
}

func registerEvidenceItem(ctx context.Context, db *pgxpool.Pool, job ingestJob, originalFilename string, sizeBytes int64, classification evidenceClassification, force bool) (evidenceRegistration, error) {
	if db == nil {
		return evidenceRegistration{Created: true, RoutePlan: evidenceRoutePlanFromMetadata(job.Metadata)}, nil
	}
	evidenceID := uuid.NewString()
	versionID := uuid.NewString()
	job.Metadata["version_id"] = versionID
	routePlan := evidenceRoutePlanFromMetadata(job.Metadata)
	if routePlan.PlanVersion != "" {
		routePlan.EvidenceID = evidenceID
		routePlan.VersionID = versionID
		routePlan.PlanID = evidenceRoutePlanID(routePlan)
		recordEvidenceRoutePlanMetadata(job.Metadata, routePlan)
	}
	mediaMetadata, mediaWarnings := extractDeterministicMediaMetadata(job.SpoolPath, job.SourceFile, classification)
	classification.Warnings = append(classification.Warnings, mediaWarnings...)
	classification = applyImageIntakeClassification(classification, mediaMetadata)
	metadata := evidenceRegistrationMetadata(job, classification, versionID)
	if errText := job.Metadata["kb_mirror_error"]; errText != "" {
		classification.Warnings = append(classification.Warnings, "kb mirror failed: "+errText)
		metadata["kb_mirror_error"] = errText
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return evidenceRegistration{}, fmt.Errorf("marshal evidence metadata: %w", err)
	}
	mediaMetadataJSON, err := json.Marshal(mediaMetadata)
	if err != nil {
		return evidenceRegistration{}, fmt.Errorf("marshal deterministic media metadata: %w", err)
	}
	warningsJSON, err := json.Marshal(classification.Warnings)
	if err != nil {
		return evidenceRegistration{}, fmt.Errorf("marshal evidence warnings: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := db.Begin(ctx)
	if err != nil {
		return evidenceRegistration{}, fmt.Errorf("begin evidence registration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", job.TenantID); err != nil {
		return evidenceRegistration{}, fmt.Errorf("set evidence registration tenant context: %w", err)
	}
	if !force {
		lockKey := evidenceAdvisoryLockKey(job.TenantID, job.CollectionID, job.SHA256)
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
			return evidenceRegistration{}, fmt.Errorf("lock evidence registration: %w", err)
		}
		var existing evidenceRegistration
		err := tx.QueryRow(ctx, `
SELECT evidence_id::text, coalesce(metadata->>'version_id', '')
FROM forensic.evidence_items
WHERE tenant_id=$1 AND collection_id=$2 AND sha256=$3
ORDER BY created_at
LIMIT 1`, job.TenantID, job.CollectionID, job.SHA256).Scan(&existing.EvidenceID, &existing.VersionID)
		if err == nil {
			existing.Created = false
			if err := tx.Commit(ctx); err != nil {
				return evidenceRegistration{}, fmt.Errorf("commit duplicate evidence registration: %w", err)
			}
			return existing, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return evidenceRegistration{}, fmt.Errorf("check evidence registration: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `
INSERT INTO forensic.evidence_items (
  evidence_id, tenant_id, collection_id, case_id, user_id, original_filename, source_file,
  content_type, extension, size_bytes, sha256, modality, detected_type,
  classifier_confidence, processing_route, processing_status, raw_storage_ref,
  kb_entry_ref, media_metadata, metadata, warnings
)
VALUES (
  $1::uuid, $2, $3, NULLIF($4, ''), $5, $6, $7,
  NULLIF($8, ''), NULLIF($9, ''), $10, $11, $12, $13,
  $14, $15, $16, $17,
  NULLIF($18, ''), $19::jsonb, $20::jsonb, $21::jsonb
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
		defaultString(job.Metadata["storage_uri"], job.SpoolPath),
		job.Metadata["kb_source_entry"],
		string(mediaMetadataJSON),
		string(metadataJSON),
		string(warningsJSON),
	)
	if err != nil {
		return evidenceRegistration{}, fmt.Errorf("register evidence item: %w", err)
	}
	if classification.QueueRecords {
		job.EvidenceID = evidenceID
		if err := persistQueuedJobTx(ctx, tx, job); err != nil {
			return evidenceRegistration{}, fmt.Errorf("register evidence queue outbox atomically: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return evidenceRegistration{}, fmt.Errorf("commit evidence registration: %w", err)
	}
	return evidenceRegistration{EvidenceID: evidenceID, VersionID: versionID, Created: true, Warnings: classification.Warnings, MediaMetadata: mediaMetadata, ProcessingRoute: classification.ProcessingRoute, ProcessingStatus: classification.ProcessingStatus, RoutePlan: routePlan}, nil
}

func applyEvidenceRegistrationWarnings(classification evidenceClassification, registration evidenceRegistration) evidenceClassification {
	classification.Warnings = appendUniqueWarnings(classification.Warnings, registration.Warnings...)
	if registration.ProcessingRoute != "" {
		classification.ProcessingRoute = registration.ProcessingRoute
	}
	if registration.ProcessingStatus != "" {
		classification.ProcessingStatus = registration.ProcessingStatus
	}
	return classification
}

func applyImageIntakeClassification(classification evidenceClassification, metadata map[string]any) evidenceClassification {
	if classification.Modality != "image" {
		return classification
	}
	if metadata["validation_state"] == "manual_review_required" {
		classification.ProcessingRoute = "image_intake_manual_review"
		classification.ProcessingStatus = "registered"
		classification.QueueRecords = false
	}
	return classification
}

func evidenceRegistrationMetadata(job ingestJob, classification evidenceClassification, versionID string) map[string]any {
	metadata := map[string]any{
		"request_id":                  job.RequestID,
		"file_id":                     job.FileID,
		"job_id":                      job.JobID,
		"headers":                     job.Headers,
		"requested_record_type":       job.RecordType,
		"detected_record_type":        job.DetectedRecordType,
		"kb_mirror_status":            job.Metadata["kb_mirror_status"],
		"kb_mirror_mode":              job.Metadata["kb_mirror_mode"],
		"version_id":                  versionID,
		"evidence_role":               defaultString(job.Metadata["evidence_role"], "source"),
		"declared_modality":           job.Metadata["declared_modality"],
		"parent_evidence_id":          job.Metadata["parent_evidence_id"],
		"jurisdiction":                job.Metadata["jurisdiction"],
		"source_timezone":             job.Metadata["source_timezone"],
		"source_timezone_state":       job.Metadata["source_timezone_state"],
		"source_date_order":           job.Metadata["source_date_order"],
		"source_date_order_state":     job.Metadata["source_date_order_state"],
		"processing_route":            classification.ProcessingRoute,
		"storage_mode":                classification.StorageMode,
		"storage_uri":                 job.Metadata["storage_uri"],
		"storage_receipt_uri":         job.Metadata["storage_receipt_uri"],
		"storage_layout_version":      job.Metadata["storage_layout_version"],
		"storage_scope_key":           job.Metadata["storage_scope_key"],
		"storage_integrity_state":     job.Metadata["storage_integrity_state"],
		"storage_verified_at":         job.Metadata["storage_verified_at"],
		"storage_retained_at":         job.Metadata["storage_retained_at"],
		"storage_verification_method": job.Metadata["storage_verification_method"],
		"storage_write_once":          job.Metadata["storage_write_once"],
		"storage_object_reused":       job.Metadata["storage_object_reused"],
	}
	if routePlan := evidenceRoutePlanFromMetadata(job.Metadata); routePlan.PlanVersion != "" {
		metadata["evidence_route_plan"] = routePlan
	}
	return metadata
}

func evidenceAdvisoryLockKey(tenantID, collectionID, sha256 string) string {
	key, _ := json.Marshal([]string{tenantID, collectionID, sha256})
	return string(key)
}

func materializeRegisteredKBAsset(ctx context.Context, db *pgxpool.Pool, job ingestJob, registration evidenceRegistration, classification evidenceClassification, sizeBytes int64) error {
	if db == nil || strings.TrimSpace(job.Metadata["kb_source_entry"]) == "" {
		return nil
	}
	headersJSON, err := json.Marshal(job.Headers)
	if err != nil {
		return fmt.Errorf("marshal KB evidence headers: %w", err)
	}
	routingJSON, err := json.Marshal(map[string]any{
		"storage_mode":            classification.StorageMode,
		"modality":                classification.Modality,
		"detected_type":           classification.DetectedType,
		"processing_route":        classification.ProcessingRoute,
		"evidence_role":           defaultString(job.Metadata["evidence_role"], "source"),
		"parent_evidence_id":      job.Metadata["parent_evidence_id"],
		"version_id":              registration.VersionID,
		"routing_reason":          "universal_kb_evidence_registration",
		"reused_evidence":         !registration.Created,
		"storage_uri":             job.Metadata["storage_uri"],
		"storage_receipt_uri":     job.Metadata["storage_receipt_uri"],
		"storage_layout_version":  job.Metadata["storage_layout_version"],
		"storage_integrity_state": job.Metadata["storage_integrity_state"],
	})
	if err != nil {
		return fmt.Errorf("marshal KB evidence routing: %w", err)
	}
	qualityJSON, err := json.Marshal(map[string]any{
		"classifier_confidence":       classification.Confidence,
		"warnings":                    classification.Warnings,
		"storage_verified_at":         job.Metadata["storage_verified_at"],
		"storage_verification_method": job.Metadata["storage_verification_method"],
	})
	if err != nil {
		return fmt.Errorf("marshal KB evidence quality: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin KB evidence asset registration: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", job.TenantID); err != nil {
		return fmt.Errorf("set KB evidence tenant context: %w", err)
	}
	_, err = tx.Exec(ctx, `
INSERT INTO forensic.kb_collection_assets (
  tenant_id, user_id, collection_id, file_id, batch_id, source_file,
  source_entry, sha256, detected_record_type, requested_record_type, storage_mode,
  rag_status, structured_status, content_type, size_bytes, evidence_id, headers,
  routing_decision, quality_report
)
VALUES (
  $1, $2, $3, $4, NULL, $5,
  $6, $7, 'generic'::forensic_record_type, 'generic'::forensic_record_type, $8,
  'mirrored', 'skipped', NULLIF($9, ''), $10, NULLIF($11, '')::uuid, $12::jsonb,
  $13::jsonb, $14::jsonb
)
ON CONFLICT (tenant_id, collection_id, file_id, sha256)
DO UPDATE SET
  source_entry=EXCLUDED.source_entry,
  storage_mode=EXCLUDED.storage_mode,
  rag_status=EXCLUDED.rag_status,
  structured_status=EXCLUDED.structured_status,
  evidence_id=coalesce(forensic.kb_collection_assets.evidence_id, EXCLUDED.evidence_id),
  routing_decision=forensic.kb_collection_assets.routing_decision || EXCLUDED.routing_decision,
  quality_report=forensic.kb_collection_assets.quality_report || EXCLUDED.quality_report,
  updated_at=now()`,
		job.TenantID,
		job.UserID,
		job.CollectionID,
		job.FileID,
		job.SourceFile,
		job.Metadata["kb_source_entry"],
		job.SHA256,
		classification.StorageMode,
		job.Metadata["content_type"],
		sizeBytes,
		registration.EvidenceID,
		string(headersJSON),
		string(routingJSON),
		string(qualityJSON),
	)
	if err != nil {
		return fmt.Errorf("register KB evidence asset: %w", err)
	}
	auditDetails, err := json.Marshal(map[string]any{
		"request_id":              job.RequestID,
		"evidence_id":             registration.EvidenceID,
		"evidence_version_id":     registration.VersionID,
		"sha256":                  job.SHA256,
		"source_file":             job.SourceFile,
		"source_entry":            job.Metadata["kb_source_entry"],
		"modality":                classification.Modality,
		"detected_type":           classification.DetectedType,
		"processing_route":        classification.ProcessingRoute,
		"reused_evidence":         !registration.Created,
		"storage_uri":             job.Metadata["storage_uri"],
		"storage_receipt_uri":     job.Metadata["storage_receipt_uri"],
		"storage_integrity_state": job.Metadata["storage_integrity_state"],
		"storage_verified_at":     job.Metadata["storage_verified_at"],
	})
	if err != nil {
		return fmt.Errorf("marshal KB evidence audit event: %w", err)
	}
	auditAction := "evidence.kb.registered"
	if !registration.Created {
		auditAction = "evidence.kb.source_linked"
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO forensic.records_audit_log (
  tenant_id, user_id, action, collection_id, file_id, batch_id, details
)
VALUES ($1, $2, $3, $4, $5, NULL, $6::jsonb)`,
		job.TenantID,
		job.UserID,
		auditAction,
		job.CollectionID,
		job.FileID,
		string(auditDetails),
	); err != nil {
		return fmt.Errorf("record KB evidence audit event: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit KB evidence asset registration: %w", err)
	}
	return nil
}

func duplicateEvidenceMessage(sourceEntryLinked bool) string {
	if sourceEntryLinked {
		return "same evidence hash already exists; the distinct Knowledge Base source entry was linked to the existing evidence"
	}
	return "evidence was registered concurrently by another request"
}

func markEvidenceFailed(ctx context.Context, db *pgxpool.Pool, tenantID, evidenceID, message string) {
	if db == nil || strings.TrimSpace(evidenceID) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := db.Begin(ctx)
	if err != nil {
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return
	}
	if _, err := tx.Exec(ctx, `
UPDATE forensic.evidence_items
SET processing_status='failed',
    errors = errors || jsonb_build_array(jsonb_build_object('message', $2, 'recorded_at', now()))
WHERE evidence_id=$1::uuid`, evidenceID, truncate(message, 4000)); err == nil {
		_ = tx.Commit(ctx)
	}
}

func requestIDFromHTTP(r *http.Request) string {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if requestID == "" {
		requestID = uuid.NewString()
	}
	return requestID
}

func findExistingEvidence(ctx context.Context, db *pgxpool.Pool, tenantID, collectionID, sha256 string) (map[string]any, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var (
		evidenceID      string
		versionID       string
		sourceFile      string
		modality        string
		detectedType    string
		processingRoute string
		status          string
		createdAt       time.Time
	)
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin duplicate evidence check: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, fmt.Errorf("set duplicate evidence tenant context: %w", err)
	}
	err = tx.QueryRow(ctx, `
SELECT evidence_id::text, coalesce(metadata->>'version_id', ''), source_file,
       modality, detected_type, processing_route, processing_status, created_at
FROM forensic.evidence_items
WHERE tenant_id=$1 AND collection_id=$2 AND sha256=$3
ORDER BY created_at
LIMIT 1`, tenantID, collectionID, sha256).Scan(
		&evidenceID,
		&versionID,
		&sourceFile,
		&modality,
		&detectedType,
		&processingRoute,
		&status,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("check duplicate evidence: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit duplicate evidence check: %w", err)
	}
	return map[string]any{
		"evidence_id":         evidenceID,
		"evidence_version_id": versionID,
		"source_file":         sourceFile,
		"modality":            modality,
		"detected_type":       detectedType,
		"processing_route":    processingRoute,
		"status":              status,
		"created_at":          createdAt.UTC().Format(time.RFC3339),
	}, nil
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
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin duplicate ingest check: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, "SELECT set_config('app.tenant_id', $1, true)", tenantID); err != nil {
		return nil, fmt.Errorf("set duplicate ingest tenant context: %w", err)
	}
	err = tx.QueryRow(ctx, `
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
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit duplicate ingest check: %w", err)
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
	if err := writer.WriteField("skip_forensic_records", "true"); err != nil {
		return "", fmt.Errorf("write forensic mirror marker: %w", err)
	}
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

func sniffEvidenceContentType(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open spooled file for content detection: %w", err)
	}
	defer file.Close()
	buffer := make([]byte, 512)
	read, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read spooled file for content detection: %w", err)
	}
	if read == 0 {
		return "application/octet-stream", nil
	}
	return http.DetectContentType(buffer[:read]), nil
}

func readCSVHeaders(path string) ([]string, error) {
	return readEvidenceHeaders(path, path)
}

func readEvidenceHeaders(path, sourceName string) ([]string, error) {
	ext := strings.ToLower(filepath.Ext(sourceName))
	switch ext {
	case ".json", ".jsonl", ".ndjson":
		headers, err := readJSONHeaders(path)
		if err != nil {
			return nil, err
		}
		return validateDetectedHeaders(headers)
	case ".parquet":
		return []string{"parquet_payload"}, nil
	case ".txt", ".log":
		headers, err := readTextHeaders(path)
		if err != nil {
			return nil, err
		}
		return validateDetectedHeaders(headers)
	case ".csv":
		// Continue below with the CSV parser.
	default:
		// Header detection is intentionally limited to formats understood by the
		// current records worker. Treating arbitrary binary bytes as CSV can place
		// NUL escapes in JSONB metadata and block otherwise safe registration.
		return nil, nil
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
	return validateDetectedHeaders(headers)
}

func validateDetectedHeaders(headers []string) ([]string, error) {
	if len(headers) > 256 {
		return nil, fmt.Errorf("header detection rejected %d columns; maximum is 256", len(headers))
	}
	for _, header := range headers {
		if !utf8.ValidString(header) || strings.ContainsRune(header, '\x00') {
			return nil, errors.New("header detection rejected invalid UTF-8 or NUL bytes")
		}
		if len(header) > 512 {
			return nil, fmt.Errorf("header detection rejected a column name longer than 512 bytes")
		}
	}
	return headers, nil
}

func readJSONHeaders(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open spooled json for header detection: %w", err)
	}
	defer f.Close()
	if strings.EqualFold(filepath.Ext(path), ".json") {
		var value any
		if err := json.NewDecoder(f).Decode(&value); err != nil {
			return nil, fmt.Errorf("read json headers: %w", err)
		}
		headers := objectHeaders(value)
		if len(headers) == 0 {
			return nil, errors.New("read json headers: top-level value contains no object fields")
		}
		return headers, nil
	}
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
	if has("plate", "plate_number", "license_plate", "registration_number", "registration_no", "reg_no", "vehicle_no", "number_plate", "plate_no", "vehicle_registration_no", "registration_mark", "vrn") {
		return "anpr"
	}
	if has("source_ip", "src_ip", "ip_address") && has("destination_ip", "dst_ip", "dest_ip") {
		return "ipdr"
	}
	if has("msisdn", "phone_number", "subscriber_number", "mobile_no", "cellular_number", "mdn", "account_msisdn") &&
		has("cnic", "cnic_no", "national_id", "nic", "cnic_last4", "customer_id", "customer_no", "subscriber_id", "name", "full_name", "subscriber_name", "customer_name") {
		return "subscriber"
	}
	if has("cell_id", "cell_site_id", "site_id", "site_code", "cgi", "ecgi", "enodeb_id", "gnodeb_id") &&
		has("lat", "latitude", "latitude_wgs84") && has("longitude", "lon", "lng", "longitude_wgs84") {
		return "tower_location"
	}
	if has("amount", "transaction_amount", "txn_amount", "amount_pkr", "debit_amount", "credit_amount", "transaction_value", "local_amount") &&
		has("account", "account_number", "iban", "from_account", "from_iban", "source_account", "debit_account", "payer_account", "remitter_account", "sender", "sender_iban") {
		return "transaction"
	}
	if has("ip", "source_ip", "src_ip", "client_ip", "remote_addr", "remote_ip", "source_address") &&
		has("user", "username", "user_id", "principal", "actor", "path", "url", "uri", "event_action") {
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

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		slog.Warn("invalid duration environment value; using fallback", "key", key, "value", value, "fallback", fallback)
		return fallback
	}
	return parsed
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
