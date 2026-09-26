package main

import (
	"fmt"
	"testing"
)

// The IR schema becomes a GBNF grammar inside llama.cpp. An empty `enum` is not
// a narrow choice but an impossible one: it converts to an alternation with no
// alternatives, and the backend rejects the WHOLE request with
// "Failed to initialize samplers: failed to parse grammar" (HTTP 500) before
// generating a token. That surfaced as `runtime_rejected` on 3 of the first 21
// golden-IR questions and was invisible until the response body was recorded.

func schemaField(id, kind string) FieldDescriptorV1 {
	return FieldDescriptorV1{
		ContractVersion: fieldDescriptorContractV1, FieldID: id,
		SourceName: id, SourceNames: []string{id}, NormalizedName: id,
		EffectiveType: kind, AllowedFilters: []string{"EQ"}, AllowedAggregates: []string{"COUNT"},
		Projectable: true, Groupable: true, Sortable: true, Curated: true,
	}
}

// emptyEnums walks the schema and reports the path of every empty enum.
func emptyEnums(node any, path string) []string {
	found := []string{}
	switch typed := node.(type) {
	case map[string]any:
		for key, value := range typed {
			if key == "enum" {
				if list, ok := value.([]string); ok && len(list) == 0 {
					found = append(found, path+".enum")
					continue
				}
				if list, ok := value.([]any); ok && len(list) == 0 {
					found = append(found, path+".enum")
					continue
				}
			}
			found = append(found, emptyEnums(value, path+"."+key)...)
		}
	case []any:
		for i, value := range typed {
			found = append(found, emptyEnums(value, fmt.Sprintf("%s[%d]", path, i))...)
		}
	}
	return found
}

// A question that mentions no time legitimately retrieves no TIMESTAMP field.
// That must not produce a grammar the backend cannot parse.
func TestIRSchemaNeverEmitsAnEmptyEnum(t *testing.T) {
	cases := map[string][]FieldDescriptorV1{
		"no timestamp field retrieved": {
			schemaField("ipdr.bytes", "NUMBER"),
			schemaField("ipdr.subscriber_id", "STRING"),
		},
		"timestamp field retrieved": {
			schemaField("cdr.msisdn", "STRING"),
			schemaField("cdr.event_time", fieldTypeTimestamp),
		},
		"single non-time field": {schemaField("cdr.source_file", "STRING")},
	}
	for name, fields := range cases {
		if bad := emptyEnums(semanticSourceNativeSchema(fields), "schema"); len(bad) > 0 {
			t.Errorf("%s: empty enum(s) at %v — llama.cpp answers this with HTTP 500 "+
				"\"failed to parse grammar\" before inference", name, bad)
		}
	}
}

// With nothing to bucket, time_bucket must be null-only rather than an object
// whose field_id can never be satisfied.
func TestIRSchemaTimeBucketIsNullOnlyWithoutTimeFields(t *testing.T) {
	schema := semanticSourceNativeSchema([]FieldDescriptorV1{
		schemaField("ipdr.bytes", "NUMBER"), schemaField("ipdr.subscriber_id", "STRING")})
	bucket, ok := schema["properties"].(map[string]any)["time_bucket"].(map[string]any)
	if !ok {
		t.Fatal("time_bucket missing from the schema")
	}
	if bucket["type"] != "null" {
		t.Fatalf("time_bucket must be null-only when no field can be bucketed, got %v", bucket)
	}

	// And it must still be offered when there IS something to bucket.
	schema = semanticSourceNativeSchema([]FieldDescriptorV1{
		schemaField("cdr.msisdn", "STRING"), schemaField("cdr.event_time", fieldTypeTimestamp)})
	bucket = schema["properties"].(map[string]any)["time_bucket"].(map[string]any)
	alts, ok := bucket["anyOf"].([]any)
	if !ok || len(alts) != 2 {
		t.Fatalf("time_bucket must stay nullable-object when a time field exists, got %v", bucket)
	}
	inner := alts[0].(map[string]any)["properties"].(map[string]any)["field_id"].(map[string]any)
	// Enums are issued as []any because that is the ONLY form the GBNF
	// converter reads; asserting []string here passed while the grammar it
	// stands for was unconstrained.
	if list := schemaEnumValues(inner["enum"]); len(list) != 1 || list[0] != "cdr.event_time" {
		t.Fatalf("time_bucket field_id enum = %v, want [cdr.event_time]", inner["enum"])
	}
}

// schemaEnumValues reads an issued enum in either representation, so a test
// cannot pass on a form the grammar converter silently ignores.
func schemaEnumValues(node any) []string {
	switch typed := node.(type) {
	case []string:
		return typed
	case []any:
		out := make([]string, 0, len(typed))
		for _, value := range typed {
			text, ok := value.(string)
			if !ok {
				return nil
			}
			out = append(out, text)
		}
		return out
	}
	return nil
}
