package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	evidenceRoutePlanVersion = "nexusai-evidence-route-plan/v1"
	autoRouteThreshold       = 0.98
	maxRouteProbeBytes       = 8192
	maxImageDimension        = 65535
)

type evidenceRouteState string

const (
	routeStateReady                evidenceRouteState = "ROUTABLE_READY"
	routeStateDegraded             evidenceRouteState = "ROUTABLE_DEGRADED"
	routeStateModelRequired        evidenceRouteState = "MODEL_REQUIRED"
	routeStateProcessorUnavailable evidenceRouteState = "PROCESSOR_UNAVAILABLE"
	routeStateManualReview         evidenceRouteState = "MANUAL_REVIEW"
	routeStateRejectedSecurity     evidenceRouteState = "REJECTED_SECURITY"
)

type EvidenceRoutePlan struct {
	PlanID                string                   `json:"plan_id"`
	PlanVersion           string                   `json:"plan_version"`
	ExecutionAdmission    string                   `json:"execution_admission"`
	EvidenceID            string                   `json:"evidence_id,omitempty"`
	VersionID             string                   `json:"version_id,omitempty"`
	DetectedFormat        string                   `json:"detected_format"`
	DetectedMIME          string                   `json:"detected_mime"`
	ClassificationSignals []evidenceRouteSignal    `json:"classification_signals"`
	PrimaryFamily         string                   `json:"primary_family"`
	ComposedRoles         []string                 `json:"composed_roles"`
	RequiredCapabilities  []string                 `json:"required_capabilities"`
	ResolvedProcessors    []evidenceRouteProcessor `json:"resolved_processors"`
	Resource              evidenceRouteResource    `json:"resource"`
	Security              evidenceRouteSecurity    `json:"security"`
	Confidence            float64                  `json:"routing_confidence"`
	State                 evidenceRouteState       `json:"state"`
	Fallback              string                   `json:"fallback"`
	HumanReviewReason     string                   `json:"human_review_reason,omitempty"`
	Limitations           []string                 `json:"limitations"`
}

type evidenceRouteSignal struct {
	Name       string  `json:"name"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
	Authority  string  `json:"authority"`
}

type evidenceRouteProcessor struct {
	Role        string `json:"role"`
	ProcessorID string `json:"processor_id"`
	ModelID     string `json:"model_id,omitempty"`
	Readiness   string `json:"readiness"`
}

type evidenceRouteResource struct {
	Class               string `json:"class"`
	MinimumFreeRAMBytes int64  `json:"minimum_free_ram_bytes"`
	ParallelHeavyModels int    `json:"parallel_heavy_models"`
	StreamingRequired   bool   `json:"streaming_required"`
	EstimateBasis       string `json:"estimate_basis"`
}

type evidenceRouteSecurity struct {
	ScopeAuthorized bool     `json:"scope_authorized"`
	Result          string   `json:"result"`
	Checks          []string `json:"checks"`
	Findings        []string `json:"findings"`
}

type evidenceRouteProcessorState struct {
	ProcessorID string
	ModelID     string
	Readiness   string
}

type evidenceRouteInput struct {
	EvidenceID           string
	VersionID            string
	SourceFilename       string
	DeclaredMIME         string
	DetectedMIME         string
	Head                 []byte
	Tail                 []byte
	SizeBytes            int64
	ScopeAuthorized      bool
	DuplicateContent     bool
	PasswordProtected    bool
	ArchiveBombSuspected bool
	UnsupportedCodec     bool
	RequestedRoles       []string
	Classification       evidenceClassification
	Processors           map[string]evidenceRouteProcessorState
}

type evidenceMagicMatch struct {
	Format     string
	MIME       string
	Family     string
	Confidence float64
	Signals    []evidenceRouteSignal
	Findings   []string
	Fatal      bool
	Truncated  bool
	Width      uint32
	Height     uint32
}

func buildEvidenceRoutePlanFromPath(path string, input evidenceRouteInput) (EvidenceRoutePlan, error) {
	info, err := os.Stat(path)
	if err != nil {
		return EvidenceRoutePlan{}, fmt.Errorf("stat evidence route input: %w", err)
	}
	input.SizeBytes = info.Size()
	file, err := os.Open(path)
	if err != nil {
		return EvidenceRoutePlan{}, fmt.Errorf("open evidence route input: %w", err)
	}
	defer file.Close()

	input.Head = make([]byte, minInt64(maxRouteProbeBytes, input.SizeBytes))
	read, err := io.ReadFull(file, input.Head)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return EvidenceRoutePlan{}, fmt.Errorf("read evidence route head: %w", err)
	}
	input.Head = input.Head[:read]
	if input.SizeBytes > maxRouteProbeBytes {
		tailSize := minInt64(maxRouteProbeBytes, input.SizeBytes)
		input.Tail = make([]byte, tailSize)
		if _, err := file.ReadAt(input.Tail, input.SizeBytes-int64(tailSize)); err != nil && err != io.EOF {
			return EvidenceRoutePlan{}, fmt.Errorf("read evidence route tail: %w", err)
		}
	} else {
		input.Tail = append([]byte(nil), input.Head...)
	}
	return buildEvidenceRoutePlan(input), nil
}

func buildEvidenceRoutePlan(input evidenceRouteInput) EvidenceRoutePlan {
	match := detectEvidenceMagic(input.Head, input.Tail, input.SizeBytes)
	declaredMIME := normalizeMIME(input.DeclaredMIME)
	detectedMIME := normalizeMIME(input.DetectedMIME)
	extension := strings.ToLower(filepath.Ext(input.SourceFilename))

	plan := EvidenceRoutePlan{
		PlanVersion:        evidenceRoutePlanVersion,
		ExecutionAdmission: "SHADOW_ONLY",
		EvidenceID:         input.EvidenceID,
		VersionID:          input.VersionID,
		DetectedFormat:     match.Format,
		DetectedMIME:       match.MIME,
		PrimaryFamily:      match.Family,
		ComposedRoles:      []string{},
		Confidence:         match.Confidence,
		Fallback:           "manual_review",
		Limitations:        []string{"source contract is persisted in shadow mode; automatic execution requires separate activation approval"},
		Security: evidenceRouteSecurity{
			ScopeAuthorized: input.ScopeAuthorized,
			Result:          "PASS",
			Checks: []string{
				"scope_authorization", "empty_or_truncated", "signature_polyglot",
				"declared_mime_consistency", "password_protection", "archive_expansion", "image_dimensions",
			},
			Findings: append([]string(nil), match.Findings...),
		},
		ClassificationSignals: append([]evidenceRouteSignal(nil), match.Signals...),
	}
	if plan.DetectedFormat == "" {
		plan.DetectedFormat = "unknown"
	}
	if plan.DetectedMIME == "" {
		plan.DetectedMIME = detectedMIME
	}
	if plan.DetectedMIME == "" {
		plan.DetectedMIME = "application/octet-stream"
	}
	if plan.PrimaryFamily == "" {
		plan.PrimaryFamily = "unknown"
	}
	if detectedMIME != "" {
		plan.ClassificationSignals = append(plan.ClassificationSignals, evidenceRouteSignal{
			Name: "server_detected_mime", Value: detectedMIME, Confidence: 0.9, Authority: "bounded_content_sniff",
		})
	}
	if declaredMIME != "" {
		plan.ClassificationSignals = append(plan.ClassificationSignals, evidenceRouteSignal{
			Name: "client_declared_mime", Value: declaredMIME, Confidence: 0.25, Authority: "untrusted_hint",
		})
	}
	if extension != "" {
		plan.ClassificationSignals = append(plan.ClassificationSignals, evidenceRouteSignal{
			Name: "filename_extension", Value: extension, Confidence: 0.2, Authority: "untrusted_hint",
		})
	}

	corroborateTextClassification(&plan, input.Classification, detectedMIME, extension)
	securityRouteDecision(&plan, input, match, declaredMIME, detectedMIME, extension)
	plan.Resource = resourceEstimateForFamily(plan.PrimaryFamily)
	plan.ComposedRoles = selectEvidenceRoles(plan.PrimaryFamily, input.RequestedRoles)
	plan.RequiredCapabilities = requiredCapabilities(plan.PrimaryFamily, plan.ComposedRoles)
	plan.ResolvedProcessors = resolveEvidenceProcessors(plan.RequiredCapabilities, input.Processors)
	applyProcessorReadiness(&plan)
	if plan.State == "" {
		if plan.Confidence >= autoRouteThreshold {
			plan.State = routeStateReady
			plan.Fallback = "none"
		} else {
			plan.State = routeStateManualReview
			plan.HumanReviewReason = "routing confidence below the 0.98 automatic-selection threshold"
		}
	}
	plan.PlanID = evidenceRoutePlanID(plan)
	return plan
}

func detectEvidenceMagic(head, tail []byte, size int64) evidenceMagicMatch {
	if size == 0 || len(head) == 0 {
		return evidenceMagicMatch{Format: "empty", MIME: "application/octet-stream", Family: "unknown", Fatal: true, Findings: []string{"empty input"}}
	}
	type signature struct {
		name   string
		mime   string
		family string
		match  bool
	}
	isoFormat, isoMIME, isoFamily := "mp4", "video/mp4", "video"
	if len(head) >= 12 && bytes.Equal(head[8:12], []byte("M4A ")) {
		isoFormat, isoMIME, isoFamily = "m4a", "audio/mp4", "audio"
	}
	signatures := []signature{
		{"pdf", "application/pdf", "document", bytes.HasPrefix(head, []byte("%PDF-"))},
		{"png", "image/png", "image", bytes.HasPrefix(head, []byte("\x89PNG\r\n\x1a\n"))},
		{"jpeg", "image/jpeg", "image", len(head) >= 3 && bytes.Equal(head[:3], []byte{0xff, 0xd8, 0xff})},
		{"gif", "image/gif", "image", bytes.HasPrefix(head, []byte("GIF87a")) || bytes.HasPrefix(head, []byte("GIF89a"))},
		{"zip", "application/zip", "container", bytes.HasPrefix(head, []byte{'P', 'K', 3, 4})},
		{"gzip", "application/gzip", "container", len(head) >= 2 && head[0] == 0x1f && head[1] == 0x8b},
		{"sqlite", "application/vnd.sqlite3", "database", bytes.HasPrefix(head, []byte("SQLite format 3\x00"))},
		{"pcap", "application/vnd.tcpdump.pcap", "network_or_system_capture", hasAnyPrefix(head, [][]byte{{0xd4, 0xc3, 0xb2, 0xa1}, {0xa1, 0xb2, 0xc3, 0xd4}, {0x4d, 0x3c, 0xb2, 0xa1}, {0xa1, 0xb2, 0x3c, 0x4d}})},
		{"pcapng", "application/x-pcapng", "network_or_system_capture", bytes.HasPrefix(head, []byte{0x0a, 0x0d, 0x0d, 0x0a})},
		{isoFormat, isoMIME, isoFamily, len(head) >= 12 && bytes.Equal(head[4:8], []byte("ftyp"))},
		{"riff", "application/riff", "unknown", len(head) >= 12 && bytes.HasPrefix(head, []byte("RIFF"))},
	}
	matched := make([]signature, 0, 2)
	for _, candidate := range signatures {
		if candidate.match {
			matched = append(matched, candidate)
		}
	}
	if len(matched) == 0 && utf8.Valid(head) && textLike(head) {
		return evidenceMagicMatch{
			Format: "text", MIME: "text/plain", Family: "text", Confidence: 0.9,
			Signals: []evidenceRouteSignal{{Name: "text_probe", Value: "valid_utf8_text", Confidence: 0.9, Authority: "bounded_signature_probe"}},
		}
	}
	if len(matched) == 0 {
		return evidenceMagicMatch{Format: "unknown", MIME: "application/octet-stream", Family: "unknown", Confidence: 0.1}
	}
	selected := matched[0]
	result := evidenceMagicMatch{
		Format: selected.name, MIME: selected.mime, Family: selected.family, Confidence: 0.995,
		Signals: []evidenceRouteSignal{{Name: "magic_signature", Value: selected.name, Confidence: 0.995, Authority: "bounded_signature_probe"}},
	}
	if len(matched) > 1 {
		result.Fatal = true
		result.Findings = append(result.Findings, "multiple leading signatures indicate a polyglot")
	}
	if embeddedForeignSignature(head, selected.name) {
		result.Fatal = true
		result.Findings = append(result.Findings, "embedded foreign signature indicates a polyglot")
	}
	switch selected.name {
	case "pdf":
		if size < 8 || !bytes.Contains(tail, []byte("%%EOF")) {
			result.Truncated = true
			result.Findings = append(result.Findings, "PDF end marker is absent")
		}
	case "png":
		if len(head) < 24 || !bytes.Contains(tail, []byte("IEND")) {
			result.Truncated = true
			result.Findings = append(result.Findings, "PNG structure is truncated")
		} else {
			result.Width = binary.BigEndian.Uint32(head[16:20])
			result.Height = binary.BigEndian.Uint32(head[20:24])
		}
	case "jpeg":
		if len(tail) < 2 || !bytes.Equal(tail[len(tail)-2:], []byte{0xff, 0xd9}) {
			result.Truncated = true
			result.Findings = append(result.Findings, "JPEG end marker is absent")
		}
	case "riff":
		form := string(head[8:12])
		switch form {
		case "WAVE":
			result.Format, result.MIME, result.Family = "wav", "audio/wav", "audio"
		case "AVI ":
			result.Format, result.MIME, result.Family = "avi", "video/x-msvideo", "video"
		default:
			result.Confidence = 0.7
		}
	}
	return result
}

func securityRouteDecision(plan *EvidenceRoutePlan, input evidenceRouteInput, match evidenceMagicMatch, declaredMIME, detectedMIME, extension string) {
	if !input.ScopeAuthorized {
		rejectRoute(plan, "scope authorization did not pass")
		return
	}
	if match.Fatal || input.ArchiveBombSuspected {
		rejectRoute(plan, firstNonEmpty(strings.Join(match.Findings, "; "), "archive expansion exceeds the bounded policy"))
		return
	}
	if match.Format == "empty" {
		rejectRoute(plan, "empty evidence is not processable")
		return
	}
	if match.Width > maxImageDimension || match.Height > maxImageDimension {
		rejectRoute(plan, "image dimensions exceed the bounded decoder policy")
		return
	}
	if match.Truncated {
		manualRoute(plan, "container signature is present but bounded structural validation failed")
		return
	}
	if input.PasswordProtected {
		manualRoute(plan, "password-protected evidence requires authorized human handling")
		return
	}
	if input.UnsupportedCodec {
		manualRoute(plan, "container is recognized but its codec is unsupported")
		return
	}
	if input.DuplicateContent {
		manualRoute(plan, "duplicate content must follow the existing idempotent registration decision")
		return
	}
	if match.Format == "unknown" {
		manualRoute(plan, "no admitted deterministic signature matched")
		return
	}
	if declaredMIME != "" && declaredMIME != "application/octet-stream" && !mimeCompatible(declaredMIME, plan.DetectedMIME) {
		plan.Security.Findings = append(plan.Security.Findings, "declared MIME conflicts with the content signature")
		manualRoute(plan, "declared MIME conflicts with the content signature")
		return
	}
	if detectedMIME != "" && detectedMIME != "application/octet-stream" && !mimeCompatible(detectedMIME, plan.DetectedMIME) {
		plan.Security.Findings = append(plan.Security.Findings, "server-detected MIME conflicts with the route signature")
		manualRoute(plan, "server-detected MIME conflicts with the route signature")
		return
	}
	if expected, ok := mimeForExtension(extension); ok && !mimeCompatible(expected, plan.DetectedMIME) {
		plan.Security.Findings = append(plan.Security.Findings, "filename extension conflicts with the content signature")
		plan.Limitations = append(plan.Limitations, "extension is retained only as a conflicting hint")
	}
}

func corroborateTextClassification(plan *EvidenceRoutePlan, classification evidenceClassification, detectedMIME, extension string) {
	if plan.DetectedFormat != "text" || classification.Modality == "" || classification.Modality == "unknown" {
		return
	}
	if !strings.HasPrefix(detectedMIME, "text/") && detectedMIME != "application/json" && detectedMIME != "application/x-ndjson" {
		return
	}
	profile, found := evidenceProfileForExtension(extension)
	if !found || profile.Modality == "" {
		return
	}
	plan.DetectedFormat = classification.DetectedType
	plan.PrimaryFamily = classification.Modality
	plan.Confidence = 0.99
	plan.ClassificationSignals = append(plan.ClassificationSignals, evidenceRouteSignal{
		Name: "bounded_schema_classification", Value: classification.DetectedType, Confidence: 0.99, Authority: "deterministic_header_or_schema_parser",
	})
}

func selectEvidenceRoles(family string, requested []string) []string {
	allowed := map[string]map[string]struct{}{
		"image":    {"anpr": {}, "ocr": {}, "face_candidate": {}, "image_embedding": {}},
		"audio":    {"asr": {}},
		"video":    {"anpr": {}, "ocr": {}, "asr": {}, "bounded_frames": {}},
		"document": {"ocr": {}},
	}
	roles := make([]string, 0, len(requested))
	seen := map[string]struct{}{}
	for _, role := range requested {
		role = normalize(role)
		if _, ok := allowed[family][role]; !ok {
			continue
		}
		if _, ok := seen[role]; ok {
			continue
		}
		seen[role] = struct{}{}
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}

func requiredCapabilities(family string, roles []string) []string {
	primary := map[string]string{
		"structured_records": "structured_profile", "structured_data": "structured_profile", "tabular": "structured_profile",
		"text": "text_parse", "document": "document_parse", "image": "image_metadata", "audio": "audio_metadata",
		"video": "video_metadata", "container": "archive_inventory", "database": "database_inventory",
		"network_or_system_capture": "capture_inventory",
	}[family]
	capabilities := make([]string, 0, len(roles)+1)
	if primary != "" {
		capabilities = append(capabilities, primary)
	}
	capabilities = append(capabilities, roles...)
	return capabilities
}

func resolveEvidenceProcessors(capabilities []string, states map[string]evidenceRouteProcessorState) []evidenceRouteProcessor {
	resolved := make([]evidenceRouteProcessor, 0, len(capabilities))
	for _, capability := range capabilities {
		state, ok := states[capability]
		if !ok {
			state = defaultEvidenceProcessorState(capability)
		}
		resolved = append(resolved, evidenceRouteProcessor{
			Role: capability, ProcessorID: state.ProcessorID, ModelID: state.ModelID, Readiness: state.Readiness,
		})
	}
	return resolved
}

func defaultEvidenceProcessorState(capability string) evidenceRouteProcessorState {
	switch capability {
	case "structured_profile":
		return evidenceRouteProcessorState{ProcessorID: "forensic_records_worker", Readiness: "READY"}
	case "text_parse":
		return evidenceRouteProcessorState{ProcessorID: "kb_text_index", Readiness: "DEGRADED"}
	case "document_parse":
		return evidenceRouteProcessorState{ProcessorID: "native_document_worker", Readiness: "READY"}
	case "image_metadata", "audio_metadata", "video_metadata":
		return evidenceRouteProcessorState{ProcessorID: "unified_media_worker", Readiness: "READY"}
	case "archive_inventory", "database_inventory", "capture_inventory":
		return evidenceRouteProcessorState{ProcessorID: capability + "_pending", Readiness: "UNAVAILABLE"}
	default:
		return evidenceRouteProcessorState{ProcessorID: capability + "_processor", Readiness: "MODEL_REQUIRED"}
	}
}

func applyProcessorReadiness(plan *EvidenceRoutePlan) {
	if plan.State == routeStateRejectedSecurity || plan.State == routeStateManualReview {
		return
	}
	for _, processor := range plan.ResolvedProcessors {
		switch processor.Readiness {
		case "MODEL_REQUIRED":
			plan.State = routeStateModelRequired
			plan.Fallback = "manual_review"
			plan.HumanReviewReason = "a required processor model is absent"
			return
		case "UNAVAILABLE", "UNHEALTHY":
			plan.State = routeStateProcessorUnavailable
			plan.Fallback = "manual_review"
			plan.HumanReviewReason = "a required processor is unavailable"
			return
		case "DEGRADED":
			plan.State = routeStateDegraded
			plan.Fallback = "explicit_degraded_route_only"
		}
	}
}

func resourceEstimateForFamily(family string) evidenceRouteResource {
	resource := evidenceRouteResource{Class: "LIGHT", MinimumFreeRAMBytes: 3 * 1024 * 1024 * 1024, ParallelHeavyModels: 1, EstimateBasis: "NX-MMR workload-class floor"}
	switch family {
	case "image", "audio", "document":
		resource.Class = "MEDIUM"
		resource.MinimumFreeRAMBytes = int64(3.5 * 1024 * 1024 * 1024)
	case "video":
		resource.Class = "HEAVY"
		resource.MinimumFreeRAMBytes = int64(4.5 * 1024 * 1024 * 1024)
		resource.StreamingRequired = true
	}
	return resource
}

func evidenceRoutePlanID(plan EvidenceRoutePlan) string {
	copyPlan := plan
	copyPlan.PlanID = ""
	payload, _ := json.Marshal(copyPlan)
	digest := sha256.Sum256(payload)
	return "erp-" + hex.EncodeToString(digest[:16])
}

func recordEvidenceRoutePlanMetadata(metadata map[string]string, plan EvidenceRoutePlan) {
	payload, err := json.Marshal(plan)
	if err != nil {
		metadata["evidence_route_plan_error"] = err.Error()
		return
	}
	metadata["evidence_route_plan"] = string(payload)
	metadata["evidence_route_plan_id"] = plan.PlanID
	metadata["evidence_route_plan_version"] = plan.PlanVersion
	metadata["evidence_route_state"] = string(plan.State)
}

func evidenceRoutePlanFromMetadata(metadata map[string]string) EvidenceRoutePlan {
	var plan EvidenceRoutePlan
	if metadata == nil {
		return plan
	}
	_ = json.Unmarshal([]byte(metadata["evidence_route_plan"]), &plan)
	return plan
}

func firstEvidenceRoutePlan(plans ...EvidenceRoutePlan) EvidenceRoutePlan {
	for _, plan := range plans {
		if plan.PlanVersion != "" {
			return plan
		}
	}
	return EvidenceRoutePlan{}
}

func rejectRoute(plan *EvidenceRoutePlan, reason string) {
	plan.Security.Result = "REJECTED"
	plan.Security.Findings = appendUniqueWarnings(plan.Security.Findings, reason)
	plan.State = routeStateRejectedSecurity
	plan.Fallback = "none"
	plan.HumanReviewReason = reason
}

func manualRoute(plan *EvidenceRoutePlan, reason string) {
	plan.Security.Result = "REVIEW_REQUIRED"
	plan.State = routeStateManualReview
	plan.Fallback = "manual_review"
	plan.HumanReviewReason = reason
}

func normalizeMIME(value string) string {
	mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
	if err != nil {
		return strings.ToLower(strings.TrimSpace(strings.Split(value, ";")[0]))
	}
	return strings.ToLower(mediaType)
}

func mimeCompatible(left, right string) bool {
	left, right = normalizeMIME(left), normalizeMIME(right)
	if left == right {
		return true
	}
	if strings.HasPrefix(left, "text/") && strings.HasPrefix(right, "text/") {
		return true
	}
	aliases := map[string]string{
		"image/jpg": "image/jpeg", "application/x-zip-compressed": "application/zip",
		"audio/x-wav": "audio/wav", "application/x-sqlite3": "application/vnd.sqlite3",
	}
	if alias := aliases[left]; alias != "" {
		left = alias
	}
	if alias := aliases[right]; alias != "" {
		right = alias
	}
	return left == right
}

func mimeForExtension(extension string) (string, bool) {
	profile, ok := evidenceProfileForExtension(extension)
	if !ok {
		return "", false
	}
	switch profile.DetectedType {
	case "pdf":
		return "application/pdf", true
	case "image":
		if extension == ".jpg" || extension == ".jpeg" {
			return "image/jpeg", true
		}
		if extension == ".png" {
			return "image/png", true
		}
	case "audio":
		if extension == ".wav" {
			return "audio/wav", true
		}
	case "video":
		if extension == ".mp4" {
			return "video/mp4", true
		}
	}
	return "", false
}

func embeddedForeignSignature(head []byte, selected string) bool {
	patterns := map[string][]byte{
		"pdf": []byte("%PDF-"), "png": []byte("\x89PNG\r\n\x1a\n"), "jpeg": {0xff, 0xd8, 0xff}, "zip": {'P', 'K', 3, 4},
	}
	for name, pattern := range patterns {
		if name == selected {
			continue
		}
		if offset := bytes.Index(head, pattern); offset > 0 {
			return true
		}
	}
	return false
}

func hasAnyPrefix(value []byte, prefixes [][]byte) bool {
	for _, prefix := range prefixes {
		if bytes.HasPrefix(value, prefix) {
			return true
		}
	}
	return false
}

func textLike(value []byte) bool {
	if len(value) == 0 {
		return false
	}
	printable := 0
	for _, character := range string(value) {
		if character == '\n' || character == '\r' || character == '\t' || character >= 0x20 {
			printable++
		}
	}
	return float64(printable)/float64(utf8.RuneCount(value)) >= 0.95
}

func minInt64(left int, right int64) int {
	if right < int64(left) {
		return int(right)
	}
	return left
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return "security policy rejected the evidence"
}
