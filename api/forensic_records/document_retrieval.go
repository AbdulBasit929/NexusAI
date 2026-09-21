package main

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mudler/LocalAI/pkg/forensictext"
)

const documentRetrievalStrategyContractV1 = "forensics.document-retrieval-strategy/v1"

var unquotedDocumentLiteralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*which\s+documents?\s+(?:mention|mentions|contain|contains)\s+(.+?)\s*[?.!]*$`),
	regexp.MustCompile(`(?i)^\s*(?:show|find)\s+(?:me\s+)?(?:every|all)\s+mentions?\s+of\s+(.+?)\s*[?.!]*$`),
	regexp.MustCompile(`(?i)^\s*(?:find|search\s+for)\s+(?:the\s+)?exact\s+term\s+(.+?)\s+(?:in|across)\s+(?:the\s+)?documents?\s*[?.!]*$`),
	regexp.MustCompile(`(?i)^\s*(?:search|find\s+in)\s+documents?\s+(?:for\s+)?(.+?)\s*[?.!]*$`),
	regexp.MustCompile(`(?i)^\s*documents?\s+mein\s+dhoondo\s+(.+?)\s*[?.!]*$`),
}

// bindSemanticRetrievalStrategy turns the SemanticFrame retrieval signal into
// an executor input. The mode is advisory for ranking and routing only; scope,
// authorization, and citation authority remain server-owned.
func bindSemanticRetrievalStrategy(req hybridQueryRequest) hybridQueryRequest {
	if documentComparisonRequested(req) && req.TextQuery == nil {
		req.TextQuery = &forensictext.Query{MatchSemantic: forensictext.Source}
		req.RetrievalMode = semanticRetrievalSourceScoped
	}
	if req.Template == "document_search" && req.TextQuery == nil {
		if literal, exhaustive := extractUnquotedDocumentLiteral(req.Query); literal != "" {
			req.TextQuery = &forensictext.Query{LiteralText: literal, MatchSemantic: forensictext.TokenSearch}
			if exhaustive {
				req.RetrievalMode = semanticRetrievalFullText
			}
		}
	}
	frame := extractSemanticFrame(req)
	if req.RetrievalMode == "" {
		req.RetrievalMode = frame.RetrievalMode
	}
	if req.RetrievalMode == semanticRetrievalNone {
		switch req.Template {
		case "document_search", "evidence":
			req.RetrievalMode = semanticRetrievalHybrid
		}
	}
	return req
}

func extractUnquotedDocumentLiteral(question string) (literal string, exhaustive bool) {
	if extracted, _ := forensictext.Extract(question); extracted != nil {
		return "", false
	}
	for index, pattern := range unquotedDocumentLiteralPatterns {
		match := pattern.FindStringSubmatch(strings.TrimSpace(question))
		if len(match) != 2 {
			continue
		}
		literal = strings.Trim(strings.Join(strings.Fields(match[1]), " "), " \t\r\n'\"“”‘’.,!?;:")
		if len([]rune(literal)) > 256 {
			return "", false
		}
		return literal, index <= 1
	}
	return "", false
}

func retrievalUsesKnowledgeBase(req hybridQueryRequest) bool {
	switch req.RetrievalMode {
	case semanticRetrievalSemantic, semanticRetrievalHybrid:
		return true
	default:
		return req.Template == "evidence" && req.RetrievalMode == semanticRetrievalNone
	}
}

func retrievalUsesDerivedText(req hybridQueryRequest) bool {
	if req.Template != "document_search" && req.Template != "audio_transcript_search" && req.Template != "image_ocr_search" && req.Template != "evidence" {
		return false
	}
	switch req.RetrievalMode {
	case semanticRetrievalSemantic, semanticRetrievalMetadata:
		return false
	default:
		return true
	}
}

func retrievalStrategyAudit(req hybridQueryRequest) map[string]any {
	mode := req.RetrievalMode
	if mode == "" {
		mode = semanticRetrievalNone
	}
	return map[string]any{
		"contract_version":  documentRetrievalStrategyContractV1,
		"retrieval_mode":    mode,
		"semantic_frame":    semanticFrameContractV1,
		"kb_retrieval":      retrievalUsesKnowledgeBase(req),
		"derived_text":      retrievalUsesDerivedText(req),
		"exact_is_semantic": false,
	}
}

func annotateRetrievalEvidence(req hybridQueryRequest, evidence map[string]any) map[string]any {
	if evidence == nil {
		evidence = map[string]any{"results": []map[string]any{}}
	}
	evidence["retrieval_strategy"] = retrievalStrategyAudit(req)
	evidence["retrieval_mode"] = req.RetrievalMode
	return evidence
}

func filterEvidenceToRetrievalScope(req hybridQueryRequest, evidence map[string]any) map[string]any {
	results := evidenceResults(evidence)
	if len(results) == 0 {
		return evidence
	}
	wantedEvidence := map[string]bool{}
	wantedFiles := map[string]bool{}
	if req.EvidenceID != "" {
		wantedEvidence[strings.ToLower(req.EvidenceID)] = true
	}
	if req.SourceFile != "" {
		wantedFiles[strings.ToLower(filepath.Base(req.SourceFile))] = true
	}
	if req.SourceSet != nil {
		for _, source := range req.SourceSet.Sources {
			if source.EvidenceID != "" {
				wantedEvidence[strings.ToLower(source.EvidenceID)] = true
			}
			if source.SourceFile != "" {
				wantedFiles[strings.ToLower(filepath.Base(source.SourceFile))] = true
			}
		}
	}
	documentOnly := req.Template == "document_search"
	filtered := make([]map[string]any, 0, len(results))
	for _, item := range results {
		metadata, _ := item["metadata"].(map[string]any)
		evidenceID := strings.ToLower(strings.TrimSpace(stringValueAny(metadata["evidence_id"])))
		file := strings.ToLower(filepath.Base(strings.TrimSpace(stringValueAny(firstPresent(metadata, "source_file", "file_name", "source")))))
		if len(wantedEvidence) > 0 && !wantedEvidence[evidenceID] {
			continue
		}
		if len(wantedEvidence) == 0 && len(wantedFiles) > 0 && !wantedFiles[file] {
			continue
		}
		if documentOnly && !retrievalResultIsDocument(metadata, file) {
			continue
		}
		filtered = append(filtered, item)
	}
	evidence["results"] = filtered
	evidence["row_count"] = len(filtered)
	if len(filtered) == 0 && len(results) > 0 {
		evidence["result_state"] = "NO_MATCH"
		evidence["scope_filtered_count"] = len(results)
	}
	return evidence
}

func documentComparisonRequested(req hybridQueryRequest) bool {
	return req.Template == "document_search" && containsAny(normalizeAnalystSemantics(req.Query), []string{
		"compare these two documents", "compare the two documents", "compare selected documents",
		"compare the selected documents", "document comparison",
	})
}

func retrievalResultIsDocument(metadata map[string]any, file string) bool {
	if strings.EqualFold(stringValueAny(metadata["source_family"]), "document_text") ||
		containsString([]string{"document", "text"}, normalize(stringValueAny(metadata["modality"]))) ||
		stringValueAny(metadata["artifact_type"]) == "forensics.document-native-text-passage/v1" {
		return true
	}
	switch strings.ToLower(filepath.Ext(file)) {
	case ".txt", ".md", ".yaml", ".yml", ".pdf", ".doc", ".docx", ".rtf", ".odt", ".ppt", ".pptx", ".odp", ".html", ".htm", ".epub", ".eml", ".msg":
		return true
	default:
		return false
	}
}
