package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	maxPDFInventoryBytes = 64 << 20
	maxPDFTailBytes      = 128 << 10
	maxPDFTokens         = 500_000
)

func extractPDFMetadata(path string) (map[string]any, []string, error) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, nil, fmt.Errorf("open PDF: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("stat PDF: %w", err)
	}
	if info.Size() < 8 {
		return nil, nil, fmt.Errorf("file is too short for a PDF header")
	}

	prefixSize := info.Size()
	if prefixSize > maxPDFInventoryBytes {
		prefixSize = maxPDFInventoryBytes
	}
	prefix := make([]byte, prefixSize)
	if _, err := io.ReadFull(file, prefix); err != nil {
		return nil, nil, fmt.Errorf("read bounded PDF inventory: %w", err)
	}
	headerWindow := prefix
	if len(headerWindow) > 1024 {
		headerWindow = headerWindow[:1024]
	}
	headerOffset := bytes.Index(headerWindow, []byte("%PDF-"))
	if headerOffset < 0 || headerOffset+8 > len(headerWindow) {
		return nil, nil, fmt.Errorf("PDF header was not found in the first 1024 bytes")
	}
	version := string(headerWindow[headerOffset+5 : headerOffset+8])
	if version[0] < '0' || version[0] > '9' || version[1] != '.' || version[2] < '0' || version[2] > '9' {
		return nil, nil, fmt.Errorf("invalid PDF version %q", version)
	}

	tailSize := info.Size()
	if tailSize > maxPDFTailBytes {
		tailSize = maxPDFTailBytes
	}
	tail := make([]byte, tailSize)
	if _, err := file.ReadAt(tail, info.Size()-tailSize); err != nil && err != io.EOF {
		return nil, nil, fmt.Errorf("read PDF tail: %w", err)
	}

	prefixTokens, prefixWarnings := scanPDFStructuralTokens(prefix, maxPDFTokens)
	tailTokens := prefixTokens
	var tailWarnings []string
	if info.Size() > prefixSize {
		tailTokens, tailWarnings = scanPDFStructuralTokens(tail, maxPDFTokens/8)
	}
	allTokens := prefixTokens
	if info.Size() > prefixSize {
		allTokens = append(append(make([]string, 0, len(prefixTokens)+len(tailTokens)), prefixTokens...), tailTokens...)
	}

	declaredPageCount, observedPageObjects := pdfPageCounts(prefixTokens)
	pageCount := observedPageObjects
	pageCountSource := "observed_page_objects"
	if declaredPageCount >= 0 {
		pageCount = declaredPageCount
		pageCountSource = "page_tree_count"
	}
	hasEOF := bytes.LastIndex(tail, []byte("%%EOF")) >= 0
	truncatedScan := info.Size() > prefixSize
	hasObjectStreams := containsPDFTokenPair(allTokens, "/Type", "/ObjStm") || containsPDFToken(allTokens, "/ObjStm")
	embeddedFileObjectCount := countPDFTokenPairs(allTokens, "/Type", "/EmbeddedFile")
	javascriptIndicatorCount := countPDFToken(allTokens, "/JavaScript") + countPDFToken(allTokens, "/JS")
	pageCountReconciled := declaredPageCount >= 0 && declaredPageCount == observedPageObjects && !truncatedScan && !hasObjectStreams && hasEOF && len(prefixWarnings) == 0

	metadata := mediaMetadataBase("pdf")
	metadata["pdf_version"] = version
	metadata["file_size_bytes"] = info.Size()
	metadata["header_offset_bytes"] = headerOffset
	metadata["has_eof_marker"] = hasEOF
	metadata["has_startxref"] = containsPDFToken(allTokens, "startxref")
	metadata["linearized"] = containsPDFToken(allTokens, "/Linearized")
	metadata["encrypted"] = containsPDFToken(allTokens, "/Encrypt")
	metadata["has_embedded_files"] = containsPDFToken(allTokens, "/EmbeddedFiles") || embeddedFileObjectCount > 0
	metadata["embedded_file_object_count"] = embeddedFileObjectCount
	metadata["has_javascript"] = javascriptIndicatorCount > 0
	metadata["javascript_indicator_count"] = javascriptIndicatorCount
	metadata["has_open_action"] = containsPDFToken(allTokens, "/OpenAction")
	metadata["has_additional_actions"] = containsPDFToken(allTokens, "/AA")
	metadata["has_launch_action"] = containsPDFToken(allTokens, "/Launch")
	metadata["has_acroform"] = containsPDFToken(allTokens, "/AcroForm")
	metadata["has_xfa"] = containsPDFToken(allTokens, "/XFA")
	metadata["has_digital_signature"] = containsPDFTokenPair(allTokens, "/Type", "/Sig") || containsPDFToken(allTokens, "/ByteRange")
	metadata["has_object_streams"] = hasObjectStreams
	metadata["declared_page_count"] = declaredPageCount
	metadata["observed_page_objects"] = observedPageObjects
	metadata["page_count"] = pageCount
	metadata["page_count_source"] = pageCountSource
	metadata["page_count_reliable"] = pageCountReconciled
	metadata["scan_truncated"] = truncatedScan
	metadata["inventory_complete"] = !truncatedScan && hasEOF && len(prefixWarnings) == 0 && len(tailWarnings) == 0
	metadata["content_extracted"] = false
	metadata["content_validation"] = "bounded_lexical_structure_only"

	warnings := appendUniqueWarnings(nil, prefixWarnings...)
	warnings = appendUniqueWarnings(warnings, tailWarnings...)
	if headerOffset != 0 {
		warnings = appendUniqueWarnings(warnings, "PDF header is not at byte zero")
	}
	if !hasEOF {
		warnings = appendUniqueWarnings(warnings, "PDF EOF marker was not found in the final 128 KiB")
	}
	if truncatedScan {
		warnings = appendUniqueWarnings(warnings, "PDF structural inventory was limited to the first 64 MiB plus the final 128 KiB")
	}
	if declaredPageCount < 0 {
		warnings = appendUniqueWarnings(warnings, "PDF page-tree Count was not observable; page count uses visible Page objects")
	}
	if hasObjectStreams && declaredPageCount < 0 {
		warnings = appendUniqueWarnings(warnings, "PDF object streams can hide page dictionaries from the bounded lexical inventory")
	}
	if metadata["has_javascript"] == true || metadata["has_open_action"] == true || metadata["has_additional_actions"] == true || metadata["has_launch_action"] == true {
		warnings = appendUniqueWarnings(warnings, "PDF contains active-content indicators and requires isolated review")
	}
	if metadata["has_embedded_files"] == true {
		warnings = appendUniqueWarnings(warnings, "PDF declares embedded-file content and requires attachment inventory before extraction")
	}
	return metadata, warnings, nil
}

func scanPDFStructuralTokens(data []byte, limit int) ([]string, []string) {
	tokens := make([]string, 0, min(len(data)/12, limit))
	var warnings []string
	for offset := 0; offset < len(data); {
		if len(tokens) >= limit {
			warnings = appendUniqueWarnings(warnings, fmt.Sprintf("PDF structural token limit of %d was reached", limit))
			break
		}
		value := data[offset]
		switch {
		case isPDFWhitespace(value):
			offset++
		case value == '%':
			for offset < len(data) && data[offset] != '\n' && data[offset] != '\r' {
				offset++
			}
		case value == '(':
			next, complete := skipPDFLiteralString(data, offset)
			offset = next
			if !complete {
				warnings = appendUniqueWarnings(warnings, "PDF contains an unterminated literal string in the bounded inventory")
			}
		case value == '<' && offset+1 < len(data) && data[offset+1] == '<':
			tokens = append(tokens, "<<")
			offset += 2
		case value == '>' && offset+1 < len(data) && data[offset+1] == '>':
			tokens = append(tokens, ">>")
			offset += 2
		case value == '<':
			end := bytes.IndexByte(data[offset+1:], '>')
			if end < 0 {
				warnings = appendUniqueWarnings(warnings, "PDF contains an unterminated hexadecimal string in the bounded inventory")
				offset = len(data)
			} else {
				offset += end + 2
			}
		case value == '/':
			end := offset + 1
			for end < len(data) && !isPDFDelimiter(data[end]) {
				end++
			}
			tokens = append(tokens, decodePDFName(data[offset:end]))
			offset = end
		case isPDFDelimiter(value):
			tokens = append(tokens, string(value))
			offset++
		default:
			end := offset
			for end < len(data) && !isPDFDelimiter(data[end]) {
				end++
			}
			token := string(data[offset:end])
			tokens = append(tokens, token)
			offset = end
			if token == "stream" {
				endStream := bytes.Index(data[offset:], []byte("endstream"))
				if endStream < 0 {
					warnings = appendUniqueWarnings(warnings, "PDF stream did not terminate inside the bounded inventory")
					offset = len(data)
				} else {
					offset += endStream + len("endstream")
				}
			}
		}
	}
	return tokens, warnings
}

func skipPDFLiteralString(data []byte, offset int) (int, bool) {
	depth := 1
	for offset++; offset < len(data); offset++ {
		switch data[offset] {
		case '\\':
			offset++
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return offset + 1, true
			}
		}
	}
	return len(data), false
}

func decodePDFName(value []byte) string {
	if !bytes.Contains(value, []byte{'#'}) {
		return string(value)
	}
	decoded := make([]byte, 0, len(value))
	for index := 0; index < len(value); index++ {
		if value[index] == '#' && index+2 < len(value) {
			parsed, err := strconv.ParseUint(string(value[index+1:index+3]), 16, 8)
			if err == nil {
				decoded = append(decoded, byte(parsed))
				index += 2
				continue
			}
		}
		decoded = append(decoded, value[index])
	}
	return string(decoded)
}

func pdfPageCounts(tokens []string) (int, int) {
	declared := -1
	observed := 0
	for index := 0; index+1 < len(tokens); index++ {
		if tokens[index] != "/Type" {
			continue
		}
		switch tokens[index+1] {
		case "/Page":
			observed++
		case "/Pages":
			end := min(len(tokens), index+66)
			for cursor := index + 2; cursor+1 < end && tokens[cursor] != ">>"; cursor++ {
				if tokens[cursor] != "/Count" {
					continue
				}
				count, err := strconv.Atoi(tokens[cursor+1])
				if err == nil && count >= 0 && count > declared {
					declared = count
				}
			}
		}
	}
	return declared, observed
}

func containsPDFToken(tokens []string, target string) bool {
	return countPDFToken(tokens, target) > 0
}

func countPDFToken(tokens []string, target string) int {
	count := 0
	for _, token := range tokens {
		if token == target {
			count++
		}
	}
	return count
}

func containsPDFTokenPair(tokens []string, first, second string) bool {
	return countPDFTokenPairs(tokens, first, second) > 0
}

func countPDFTokenPairs(tokens []string, first, second string) int {
	count := 0
	for index := 0; index+1 < len(tokens); index++ {
		if tokens[index] == first && tokens[index+1] == second {
			count++
		}
	}
	return count
}

func isPDFWhitespace(value byte) bool {
	return value == 0 || value == '\t' || value == '\n' || value == '\f' || value == '\r' || value == ' '
}

func isPDFDelimiter(value byte) bool {
	return isPDFWhitespace(value) || strings.ContainsRune("()<>[]{}/%", rune(value))
}

func appendUniqueWarnings(warnings []string, values ...string) []string {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			continue
		}
		found := false
		for _, existing := range warnings {
			if existing == value {
				found = true
				break
			}
		}
		if !found {
			warnings = append(warnings, value)
		}
	}
	return warnings
}
