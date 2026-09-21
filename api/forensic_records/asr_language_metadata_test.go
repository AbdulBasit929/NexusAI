package main

import (
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestRequestedASRLanguageNormalizesExplicitLanguage(t *testing.T) {
	request := httptest.NewRequest("POST", "/webhooks/records/upload", nil)
	request.Form = url.Values{"asr_language": {" UR "}}

	language, err := requestedASRLanguage(request)
	if err != nil {
		t.Fatalf("requestedASRLanguage returned an error: %v", err)
	}
	if language != "ur" {
		t.Fatalf("expected normalized ur language, got %q", language)
	}
}

func TestRequestedASRLanguagePreservesAutomaticDetection(t *testing.T) {
	request := httptest.NewRequest("POST", "/webhooks/records/upload", nil)

	language, err := requestedASRLanguage(request)
	if err != nil {
		t.Fatalf("requestedASRLanguage returned an error: %v", err)
	}
	if language != "" {
		t.Fatalf("expected empty language for automatic detection, got %q", language)
	}
}

func TestRequestedASRLanguageRejectsUnboundedValue(t *testing.T) {
	request := httptest.NewRequest("POST", "/webhooks/records/upload", nil)
	request.Form = url.Values{"asr_language": {"urdu"}}

	if _, err := requestedASRLanguage(request); err == nil {
		t.Fatal("expected invalid ASR language to be rejected")
	}
}

func TestRetainASRLanguageForAudioAndVideoOnly(t *testing.T) {
	for _, modality := range []string{"audio", "video"} {
		metadata := map[string]string{"asr_language": "ur"}
		retainASRLanguageForModality(metadata, modality)
		if metadata["asr_language"] != "ur" {
			t.Fatalf("expected Urdu hint to remain for %s", modality)
		}
	}
	metadata := map[string]string{"asr_language": "ur"}
	retainASRLanguageForModality(metadata, "document")
	if _, exists := metadata["asr_language"]; exists {
		t.Fatal("expected ASR hint to be removed from non-speech evidence")
	}
}
