package main

import (
	"mime"
	"path/filepath"
	"strings"
)

type evidenceTypeProfile struct {
	Extensions      []string
	Modality        string
	DetectedType    string
	ProcessingRoute string
	Confidence      float64
	WorkerReadable  bool
}

var evidenceTypeProfiles = []evidenceTypeProfile{
	{[]string{".csv"}, "tabular", "delimited_text", "forensic_records_worker", 0.72, true},
	{[]string{".json", ".jsonl", ".ndjson"}, "structured_data", "json_records", "forensic_records_worker", 0.72, true},
	{[]string{".parquet"}, "tabular", "parquet", "forensic_records_worker", 0.78, true},
	{[]string{".log"}, "text", "text", "text_or_log_classification_pending", 0.55, false},
	{[]string{".txt"}, "text", "text", "native_document_worker", 0.72, true},
	{[]string{".tsv"}, "tabular", "delimited_text", "forensic_records_worker", 0.72, true},
	{[]string{".xlsx"}, "tabular", "spreadsheet", "forensic_records_worker", 0.82, true},
	{[]string{".xls", ".xlsm", ".ods", ".arrow", ".feather", ".avro", ".orc", ".xml"}, "tabular", "tabular_data", "tabular_adapter_pending", 0.62, false},
	{[]string{".pdf"}, "document", "pdf", "native_document_worker", 0.82, true},
	{[]string{".docx"}, "document", "document", "native_document_worker", 0.78, true},
	{[]string{".doc", ".rtf", ".odt", ".ppt", ".pptx", ".odp", ".html", ".htm", ".epub", ".eml", ".msg"}, "document", "document", "document_extraction_pending", 0.7, false},
	{[]string{".md", ".yaml", ".yml"}, "text", "text_document", "kb_text_index", 0.72, false},
	{[]string{".srt", ".vtt", ".ass", ".ssa"}, "text", "transcript", "transcript_index_pending", 0.8, false},
	{[]string{".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".tif", ".tiff"}, "image", "image", "unified_media_worker", 0.78, true},
	{[]string{".heic", ".heif", ".dng", ".raw", ".svg"}, "image", "image", "image_decoder_pending", 0.7, false},
	{[]string{".wav", ".mp3", ".m4a", ".flac", ".ogg", ".aac", ".opus", ".wma", ".amr"}, "audio", "audio", "unified_media_worker", 0.78, true},
	{[]string{".mp4", ".mov", ".mkv", ".avi", ".webm", ".m4v", ".mpg", ".mpeg", ".mts", ".m2ts", ".3gp"}, "video", "video", "unified_media_worker", 0.78, true},
	{[]string{".pcap", ".pcapng", ".cap", ".evtx"}, "network_or_system_capture", "capture", "capture_adapter_pending", 0.82, false},
	{[]string{".sqlite", ".sqlite3", ".db", ".sql"}, "database", "database", "database_adapter_pending", 0.7, false},
	{[]string{".zip", ".7z", ".rar", ".tar", ".gz", ".tgz", ".bz2", ".xz"}, "container", "archive", "archive_inventory_pending", 0.75, false},
}

var supportedDeclaredModalities = map[string]struct{}{
	"structured_records":        {},
	"tabular":                   {},
	"structured_data":           {},
	"text":                      {},
	"document":                  {},
	"image":                     {},
	"audio":                     {},
	"video":                     {},
	"network_or_system_capture": {},
	"database":                  {},
	"container":                 {},
	"unknown":                   {},
}

func classifyEvidenceItem(sourceFile, contentType, detectedRecordType string, headers []string, hints ...map[string]string) evidenceClassification {
	ext := strings.ToLower(filepath.Ext(sourceFile))
	profile, found := evidenceProfileForExtension(ext)
	if !found {
		profile = evidenceProfileForContentType(contentType)
	}
	classification := evidenceClassification{
		Modality:         "unknown",
		DetectedType:     "unknown",
		ProcessingRoute:  "manual_review",
		Confidence:       0.15,
		ProcessingStatus: "registered",
		StorageMode:      "records_only",
	}
	if found || profile.Modality != "" {
		classification.Modality = profile.Modality
		classification.DetectedType = profile.DetectedType
		classification.ProcessingRoute = profile.ProcessingRoute
		classification.Confidence = profile.Confidence
	}
	if strings.TrimSpace(contentType) == "" {
		classification.Warnings = append(classification.Warnings, "content type was not supplied by the upload client")
	}

	recordType := normalize(detectedRecordType)
	workerReadable := profile.WorkerReadable || ext == ".log" || ext == ".txt"
	switch recordType {
	case "cdr", "ipdr", "anpr", "subscriber", "tower_location", "transaction", "access_log":
		classification.Modality = "structured_records"
		classification.DetectedType = recordType
		classification.Confidence = 0.9
		if workerReadable {
			classification.ProcessingRoute = "forensic_records_worker"
			classification.ProcessingStatus = "queued"
			classification.QueueRecords = true
		} else {
			classification.ProcessingRoute = "structured_adapter_pending"
			classification.Warnings = append(classification.Warnings, "record family was recognized but the current records worker does not parse this file format")
		}
	case "generic":
		if len(headers) > 0 && profile.WorkerReadable {
			classification.Modality = "structured_records"
			classification.DetectedType = "generic"
			classification.ProcessingRoute = "forensic_records_worker"
			classification.Confidence = 0.55
			classification.ProcessingStatus = "queued"
			classification.QueueRecords = true
			classification.Warnings = append(classification.Warnings, "record schema did not match a specialized forensic adapter")
		} else if ext == ".xlsx" && profile.WorkerReadable {
			classification.Modality = "tabular"
			classification.DetectedType = "generic"
			classification.ProcessingRoute = "forensic_records_worker"
			classification.Confidence = 0.72
			classification.ProcessingStatus = "queued"
			classification.QueueRecords = true
			classification.Warnings = append(classification.Warnings, "workbook schema detection is deferred to the bounded read-only XLSX worker")
		} else if profile.WorkerReadable {
			classification.ProcessingRoute = "structured_validation_pending"
			classification.Warnings = append(classification.Warnings, "structured format was recognized but no parseable schema was detected")
		}
	}
	if recordType == "generic" && profile.WorkerReadable && (profile.Modality == "image" || profile.Modality == "audio" || profile.Modality == "video") {
		classification.ProcessingRoute = "unified_media_worker"
		classification.ProcessingStatus = "queued"
		classification.QueueRecords = true
		classification.StorageMode = "hybrid"
	}
	if recordType == "generic" && profile.ProcessingRoute == "native_document_worker" {
		classification.ProcessingRoute = "native_document_worker"
		classification.ProcessingStatus = "queued"
		classification.QueueRecords = true
		classification.StorageMode = "hybrid"
	}

	if len(hints) > 0 {
		classification = applyEvidenceClassificationHints(classification, hints[0])
	}
	return classification
}

func evidenceProfileForExtension(ext string) (evidenceTypeProfile, bool) {
	for _, profile := range evidenceTypeProfiles {
		for _, candidate := range profile.Extensions {
			if ext == candidate {
				return profile, true
			}
		}
	}
	return evidenceTypeProfile{}, false
}

func evidenceProfileForContentType(contentType string) evidenceTypeProfile {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(contentType))
	if err != nil {
		mediaType = strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	}
	switch {
	case strings.HasPrefix(mediaType, "image/"):
		return evidenceTypeProfile{Modality: "image", DetectedType: "image", ProcessingRoute: "image_ocr_vision_pending", Confidence: 0.6}
	case strings.HasPrefix(mediaType, "audio/"):
		return evidenceTypeProfile{Modality: "audio", DetectedType: "audio", ProcessingRoute: "audio_stt_pending", Confidence: 0.6}
	case strings.HasPrefix(mediaType, "video/"):
		return evidenceTypeProfile{Modality: "video", DetectedType: "video", ProcessingRoute: "video_analysis_pending", Confidence: 0.6}
	case mediaType == "application/pdf":
		return evidenceTypeProfile{Modality: "document", DetectedType: "pdf", ProcessingRoute: "native_document_worker", Confidence: 0.65, WorkerReadable: true}
	case strings.HasPrefix(mediaType, "text/"):
		return evidenceTypeProfile{Modality: "text", DetectedType: "text", ProcessingRoute: "kb_text_index", Confidence: 0.5}
	default:
		return evidenceTypeProfile{}
	}
}

func applyEvidenceClassificationHints(classification evidenceClassification, hints map[string]string) evidenceClassification {
	declaredModality := normalize(hints["declared_modality"])
	if declaredModality != "" {
		if _, ok := supportedDeclaredModalities[declaredModality]; ok {
			classification.Modality = declaredModality
		} else {
			classification.Warnings = append(classification.Warnings, "declared modality is not supported and was ignored")
		}
	}
	switch normalize(hints["evidence_role"]) {
	case "tts_output":
		classification.Modality = "audio"
		classification.DetectedType = "tts_output"
		classification.ProcessingRoute = "tts_artifact_registry"
		classification.QueueRecords = false
		classification.ProcessingStatus = "registered"
	case "stt_transcript":
		classification.Modality = "text"
		classification.DetectedType = "stt_transcript"
		classification.ProcessingRoute = "transcript_index_pending"
		classification.QueueRecords = false
		classification.ProcessingStatus = "registered"
	case "derived_artifact":
		classification.QueueRecords = false
		classification.ProcessingStatus = "registered"
	}
	return classification
}

func finalizeEvidenceClassification(classification evidenceClassification, kbEntry string) evidenceClassification {
	if classification.QueueRecords {
		classification.ProcessingStatus = "queued"
		if strings.TrimSpace(kbEntry) != "" {
			classification.StorageMode = "hybrid"
		}
		return classification
	}
	if strings.TrimSpace(kbEntry) != "" {
		classification.StorageMode = "rag_only"
		if classification.ProcessingRoute == "kb_text_index" {
			classification.ProcessingStatus = "completed"
		}
	}
	return classification
}
