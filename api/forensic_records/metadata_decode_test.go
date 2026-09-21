package main

import "testing"

func TestDecodeIngestMetadataPreservesMixedJSONScalars(t *testing.T) {
	metadata, err := decodeIngestMetadata([]byte(`{"text":"value","attempt":5,"enabled":true,"nested":{"key":"value"},"empty":null}`))
	if err != nil {
		t.Fatalf("decode mixed metadata: %v", err)
	}
	for key, want := range map[string]string{
		"text": "value", "attempt": "5", "enabled": "true", "nested": `{"key":"value"}`,
	} {
		if got := metadata[key]; got != want {
			t.Fatalf("metadata[%q] = %q, want %q", key, got, want)
		}
	}
	if _, exists := metadata["empty"]; exists {
		t.Fatal("null metadata should be omitted")
	}
}
