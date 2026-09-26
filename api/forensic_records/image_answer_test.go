package main

import (
	"strings"
	"testing"
)

// An OCR search answered "Retrieved 1 cited evidence result from the selected
// case scope" -- a sentence that names no image, quotes no text, and could
// describe any result of any kind. The image, the matched text and the region
// were all present in the citation locator and none reached the analyst.

func ocrResponse(results []map[string]any) hybridQueryResponse {
	return hybridQueryResponse{
		Template: "image_ocr_search",
		Answer:   map[string]any{},
		Evidence: map[string]any{"result_state": "COMPLETE_RESULTS", "results": results},
	}
}

func ocrHit(text, file string) map[string]any {
	return map[string]any{
		"content": text,
		"metadata": map[string]any{
			"artifact_type": "forensics.image-ocr-observation/v1",
			"citation_locator": map[string]any{
				"source_file":      file,
				"bbox":             map[string]any{"x": 87, "y": 112, "width": 645, "height": 62},
				"coordinate_space": "original_image_pixels",
				"reading_order":    1,
			},
		},
	}
}

// Built from the real 2026-09-24 response to IMG-01.
func TestImageOCRAnswerNamesTheImageAndQuotesTheText(t *testing.T) {
	resp := ocrResponse([]map[string]any{ocrHit("Investigation Workspace", "printed-english.png")})
	req := hybridQueryRequest{Query: "Find OCR text mentioning Investigation Workspace"}

	answer := enterpriseExecutiveAnswer(req, resp, nil)
	for _, want := range []string{"printed-english.png", "Investigation Workspace"} {
		if !strings.Contains(answer, want) {
			t.Errorf("answer omits %q, which is in the citation the analyst asked about\n got: %s", want, answer)
		}
	}
	if strings.Contains(answer, "cited evidence result from the selected case scope") {
		t.Errorf("fell back to the generic non-answer: %s", answer)
	}
}

// A bounding box is something the viewer DRAWS on the image, not a sentence.
// It belongs in the locator, where the citation can open it.
func TestImageOCRAnswerDoesNotNarrateTheRegion(t *testing.T) {
	resp := ocrResponse([]map[string]any{ocrHit("Investigation Workspace", "printed-english.png")})
	answer := imageOCRExecutiveAnswer(hybridQueryRequest{}, resp)
	for _, leaked := range []string{"bbox", "87", "polygon", "original_image_pixels"} {
		if strings.Contains(answer, leaked) {
			t.Errorf("answer narrates %q, which belongs in the locator: %s", leaked, answer)
		}
	}
}

// Several images matching is a different fact from one.
func TestImageOCRAnswerReportsWhenSeveralImagesMatch(t *testing.T) {
	resp := ocrResponse([]map[string]any{
		ocrHit("REF-2026-02", "scan-a.png"),
		ocrHit("REF-2026-02 duplicate", "scan-b.png"),
	})
	answer := imageOCRExecutiveAnswer(hybridQueryRequest{Query: "Which image contains REF-2026-02?"}, resp)
	if !strings.Contains(answer, "2 images") {
		t.Errorf("two images matched and the answer must say so: %s", answer)
	}
}

// Same invariant as transcripts: cite the result that carries the claim.
func TestImageOCRAnswerCitesTheImageThatCarriesTheClaim(t *testing.T) {
	resp := ocrResponse([]map[string]any{
		ocrHit("unrelated caption text", "other.png"),
		ocrHit("Investigation Workspace", "printed-english.png"),
	})
	req := hybridQueryRequest{Query: "Find OCR text mentioning Investigation Workspace"}
	answer := imageOCRExecutiveAnswer(req, resp)
	if !strings.Contains(answer, "printed-english.png") {
		t.Errorf("cited an image that does not contain the text asked about: %s", answer)
	}
}

// Without a citable image there is no claim to make; the caller falls back
// rather than emitting a half-claim.
func TestImageOCRAnswerDeclinesWithoutACitableImage(t *testing.T) {
	resp := ocrResponse([]map[string]any{{
		"content":  "Investigation Workspace",
		"metadata": map[string]any{"citation_locator": map[string]any{"reading_order": 1}},
	}})
	if answer := imageOCRExecutiveAnswer(hybridQueryRequest{}, resp); answer != "" {
		t.Errorf("a match with no citable image must not be stated as one: %s", answer)
	}
}

// "across 1 document" names nothing an analyst can open. Found by the
// corrected oracle on DOC-02, and the same defect images and audio had.
func TestDocumentAnswerNamesTheDocument(t *testing.T) {
	result := func(text, file string) map[string]any {
		return map[string]any{
			"content":  text,
			"metadata": map[string]any{"source_file": file},
		}
	}
	resp := hybridQueryResponse{
		Template: "document_search",
		Answer:   map[string]any{},
		Evidence: map[string]any{
			"result_state": "COMPLETE_RESULTS",
			"results":      []map[string]any{result("contact number 03001234567 appears here", "nexusai-multimodal-acceptance-brief.pdf")},
		},
	}
	req := hybridQueryRequest{
		Query:     "Which document mentions contact number 03001234567?",
		ExactTerm: "03001234567",
	}
	answer := documentExecutiveAnswer(req, resp)
	if !strings.Contains(answer, "nexusai-multimodal-acceptance-brief.pdf") {
		t.Errorf("the document is known and must be named: %s", answer)
	}
}

// A sentence listing nine filenames is not a sentence.
func TestDocumentAnswerSummarisesManySources(t *testing.T) {
	sources := map[string]struct{}{}
	for _, name := range []string{"a.pdf", "b.pdf", "c.pdf", "d.pdf"} {
		sources[name] = struct{}{}
	}
	named := namedSources(sources)
	if !strings.Contains(named, "a.pdf") || !strings.Contains(named, "3 other documents") {
		t.Errorf("want the first named and the rest counted, got %q", named)
	}
	if named := namedSources(map[string]struct{}{}); named != "" {
		t.Errorf("no sources means no claim, got %q", named)
	}
}
