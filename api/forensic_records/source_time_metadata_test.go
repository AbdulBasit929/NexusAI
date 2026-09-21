package main

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSourceTimeMetadataDoesNotAssumeTimezoneOrDateOrder(t *testing.T) {
	req := httptest.NewRequest("POST", "/records/upload", nil)
	metadata, err := sourceTimeMetadata(req)
	if err != nil {
		t.Fatal(err)
	}
	if metadata["source_timezone"] != "" || metadata["source_timezone_state"] != "unknown" {
		t.Fatalf("timezone metadata = %#v, want explicit unknown state", metadata)
	}
	if metadata["source_date_order"] != "" || metadata["source_date_order_state"] != "unresolved" {
		t.Fatalf("date order metadata = %#v, want explicit unresolved state", metadata)
	}
}

func TestSourceTimeMetadataCarriesAnalystConfirmedPolicy(t *testing.T) {
	form := url.Values{
		"source_timezone":   {"Asia/Karachi"},
		"source_date_order": {"DMY"},
	}
	req := httptest.NewRequest("POST", "/records/upload", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	metadata, err := sourceTimeMetadata(req)
	if err != nil {
		t.Fatal(err)
	}
	if metadata["source_timezone_state"] != "analyst_confirmed" || metadata["source_date_order_state"] != "analyst_confirmed" {
		t.Fatalf("metadata = %#v, want analyst-confirmed states", metadata)
	}
	if metadata["allow_profile_timezone_default"] != "false" {
		t.Fatalf("metadata = %#v, analyst confirmation must not become a profile default", metadata)
	}
}

func TestSourceTimeMetadataRejectsInvalidPolicy(t *testing.T) {
	form := url.Values{"source_date_order": {"DMY-or-MDY"}}
	req := httptest.NewRequest("POST", "/records/upload", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if _, err := sourceTimeMetadata(req); err == nil {
		t.Fatal("expected invalid date order to be rejected")
	}
}
