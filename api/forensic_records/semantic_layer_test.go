package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// WI-4 — the curated semantic layer.
//
// These tests do two jobs. First they prove the shipped YAML loads and validates.
// Second, and more important, they assert the layer actually carries the things the
// 2026-09-18 baseline and the WI-0 spike proved were missing: aliases, value domains,
// derived metrics, a declared default measure and a distinct-count path. A layer that
// loads but omits those would be decoration.

func wi4Layer(t *testing.T) *SemanticLayerV1 {
	t.Helper()
	layer, err := LoadSemanticLayer(filepath.Join("..", "..", "semantic_layer"))
	if err != nil {
		t.Fatalf("the shipped semantic layer must load and validate: %v", err)
	}
	return layer
}

func TestWI4ShippedLayerLoadsAndValidates(t *testing.T) {
	layer := wi4Layer(t)
	// INGESTED SOURCE RECORDS. These bind to forensic.records and carry no
	// source block, so they are unchanged by the derived-artifact contract.
	structured := []string{"communications_cdr", "subscriber_identity", "network_ipdr",
		"anpr_vehicles", "access_security_logs", "tower_location", "financial_transactions"}
	// MODEL OBSERVATIONS. These bind to forensic.derived_artifacts. They are
	// listed separately and never merged with the structured families above:
	// a model's plate read is not a camera's sighting, and the moment the two
	// are counted together the product has fabricated a sighting.
	derived := []string{"anpr_model_observation", "video_anpr_plate_group",
		"image_ocr_observation", "audio_timestamp_segment", "audio_roman_urdu_segment",
		"face_model_observation", "image_fingerprint_observation", "image_embedding_metadata",
		// forensics.image-observation/v1 carries two KINDS of row whose fields
		// differ, so it is curated as two entities separated by
		// observation_type rather than one describing fields most of its rows
		// lack.
		"image_technical_metadata", "video_sampled_frame"}

	for _, family := range structured {
		entity, ok := layer.EntityByFamily(family)
		if !ok {
			t.Errorf("family %s is not curated", family)
			continue
		}
		if entity.Source.IsDerived() {
			t.Errorf("family %s is an ingested source family and must not bind to derived_artifacts", family)
		}
	}
	for _, family := range derived {
		entity, ok := layer.EntityByFamily(family)
		if !ok {
			t.Errorf("derived family %s is not curated", family)
			continue
		}
		if !entity.Source.IsDerived() {
			t.Errorf("family %s holds model observations and must declare source.table derived_artifacts", family)
		}
		if entity.Source.ArtifactType == "" {
			t.Errorf("family %s declares no artifact_type, so it would match every contract in the table", family)
		}
	}
	// TWO ENTITIES SHARING A CONTRACT MUST BE SEPARATED BY THEIR DISCRIMINATOR.
	// Without distinct observation_types they would match the same rows, so every
	// count over either would include the other's population -- and both answers
	// would look right.
	byContract := map[string][]SemanticLayerEntityV1{}
	for _, entity := range layer.Entities {
		if !entity.Source.IsDerived() {
			continue
		}
		byContract[entity.Source.ArtifactType] = append(byContract[entity.Source.ArtifactType], entity)
	}
	for artifactType, entities := range byContract {
		if len(entities) < 2 {
			continue
		}
		seen := map[string]string{}
		for _, entity := range entities {
			discriminator := entity.Source.ObservationType
			if discriminator == "" {
				t.Errorf("%s is one of %d entities on %s and declares no observation_type, so it matches the others' rows too",
					entity.Family, len(entities), artifactType)
				continue
			}
			if other, clash := seen[discriminator]; clash {
				t.Errorf("%s and %s share artifact_type %s AND observation_type %q, so they match the same rows",
					other, entity.Family, artifactType, discriminator)
			}
			seen[discriminator] = entity.Family
		}
	}

	if len(layer.Entities) != len(structured)+len(derived) {
		t.Fatalf("got %d entities, want %d", len(layer.Entities), len(structured)+len(derived))
	}
}

// Every field must describe itself. The absence of descriptions in the inferred
// catalogue is the specific defect this layer exists to fix.
func TestWI4EveryFieldIsDescribedAndBindable(t *testing.T) {
	layer := wi4Layer(t)
	fields := 0
	for _, entity := range layer.Entities {
		for _, field := range entity.Fields {
			fields++
			if strings.TrimSpace(field.Description) == "" {
				t.Errorf("%s has no description", field.ID)
			}
			if len(field.SourceNames) == 0 {
				t.Errorf("%s declares no source_names and can never bind to a row", field.ID)
			}
			if !strings.HasPrefix(field.ID, strings.SplitN(field.ID, ".", 2)[0]+".") {
				t.Errorf("%s is not a readable <entity>.<field> id", field.ID)
			}
		}
	}
	if fields < 50 {
		t.Fatalf("only %d curated fields; the demo case needs materially more", fields)
	}
}

// SUB-02: the correct answer (6 active subscribers, not 11) is only reachable when
// `status` and `account_status` are understood as one concept.
func TestWI4SubscriberStatusUnionsBothSourceHeaders(t *testing.T) {
	layer := wi4Layer(t)
	field, ok := layer.FieldByID("subscriber.status")
	if !ok {
		t.Fatal("subscriber.status is not curated")
	}
	for _, want := range []string{"status", "account_status"} {
		if !containsString(field.SourceNames, want) {
			t.Fatalf("subscriber.status must read %q; source_names=%v", want, field.SourceNames)
		}
	}
}

// The SUB-02 failure class: "active" must be a recognisable literal, or extraction
// never makes it an obligation and the filter is silently dropped.
func TestWI4EnumeratedValueLiteralsAreResolvable(t *testing.T) {
	layer := wi4Layer(t)
	cases := []struct{ family, literal, wantField string }{
		{"subscriber_identity", "active", "subscriber.status"},
		{"subscriber_identity", "suspended", "subscriber.status"},
		{"access_security_logs", "500", "access_log.status"},
		{"access_security_logs", "server error", "access_log.status"},
		{"communications_cdr", "sms", "cdr.call_type"},
		{"communications_cdr", "incoming", "cdr.direction"},
		{"financial_transactions", "reversed", "transaction.status"},
	}
	for _, c := range cases {
		got := layer.ValueLiterals(c.family)[c.literal]
		if got != c.wantField {
			t.Errorf("%s: literal %q resolved to %q, want %s", c.family, c.literal, got, c.wantField)
		}
	}
	if len(layer.ValueLiterals("communications_cdr")) == 0 {
		t.Fatal("cdr declares no value literals at all")
	}
}

// CDR-15: there is no duration column in the source data, so "the longest call" is
// unanswerable without a derived metric.
func TestWI4DerivedDurationMetricExists(t *testing.T) {
	layer := wi4Layer(t)
	metric, ok := layer.MetricByID("cdr.call_duration_seconds")
	if !ok {
		t.Fatal("cdr.call_duration_seconds is not curated; the longest call stays unanswerable")
	}
	if metric.Expression == "" {
		t.Fatal("the duration metric must be derived; no duration column exists in the source")
	}
	if !semanticLayerExpressionPattern.MatchString(metric.Expression) {
		t.Fatalf("expression %q is outside the allowlist", metric.Expression)
	}
	for _, ref := range []string{"cdr.call_start", "cdr.call_end"} {
		if !strings.Contains(metric.Expression, ref) {
			t.Errorf("expression %q does not reference %s", metric.Expression, ref)
		}
	}
}

// REWRITTEN 2026-09-24, because its premise expired.
//
// It asserted that `cdr.distinct_subscribers` and `anpr.distinct_plates` must
// be DECLARED, on the stated ground that "CDR-07, CDR-08, ANPR-05 are
// inexpressible in the IR today" — a placeholder marking a known gap. The IR
// has had COUNT_DISTINCT since WI-9, and those two declarations carried no
// `expression:`, so `CatalogFields` skipped them and they reached nothing. The
// UI track removed them as dead weight and wrote the two gold plans instead;
// `bucket_a_conformance_test.go` then confirmed that a COUNT_DISTINCT plan for
// CDR-08 and ANPR-05 validates against the issued enum **without** either
// metric. The capability never came from the declaration.
//
// So the invariant is not "these metrics exist". It is the one that still
// carries weight: a COUNT_DISTINCT metric must count a field the layer marks
// distinct-capable, and the fields those questions need must stay so marked —
// that flag is what puts COUNT_DISTINCT in the issued enum.
func TestWI4DistinctCountsRestOnDistinctCapableFields(t *testing.T) {
	layer := wi4Layer(t)
	for _, entity := range layer.Entities {
		for _, metric := range entity.Metrics {
			if metric.Aggregate != "COUNT_DISTINCT" {
				continue
			}
			field, ok := layer.FieldByID(metric.FieldID)
			if !ok || !field.DistinctCapable {
				t.Errorf("%s counts distinct values of a field not marked distinct_capable", metric.ID)
			}
		}
	}
	// CDR-08 and ANPR-05 are answered by a direct COUNT_DISTINCT over these
	// two columns. Unmark either and both questions silently stop being
	// answerable, with nothing else in the suite noticing.
	for _, id := range []string{"cdr.msisdn", "anpr.plate_number"} {
		field, ok := layer.FieldByID(id)
		if !ok || !field.DistinctCapable {
			t.Errorf("%s must stay distinct_capable; a distinct-count question depends on it", id)
		}
	}
}

// TXN-02: with a declared default measure, "the largest transaction" resolves to
// MAX(amount) instead of abstaining.
func TestWI4EveryEntityDeclaresADefaultMeasure(t *testing.T) {
	layer := wi4Layer(t)
	for _, entity := range layer.Entities {
		if entity.DefaultMeasure == "" {
			t.Errorf("%s declares no default_measure", entity.Family)
			continue
		}
		if _, ok := layer.MetricByID(entity.DefaultMeasure); !ok {
			if _, isField := layer.FieldByID(entity.DefaultMeasure); !isField {
				t.Errorf("%s default_measure %s resolves to nothing", entity.Family, entity.DefaultMeasure)
			}
		}
	}
	metric, ok := layer.MetricByID("transaction.largest_amount")
	if !ok || metric.Aggregate != "MAX" || metric.FieldID != "transaction.amount" {
		t.Fatalf("the largest transaction must resolve to MAX(transaction.amount); got %+v", metric)
	}
}

// PII must declare how it is redacted. SUB-03 returns a masked CNIC because the
// masking is a control; the golden oracle expecting an unmasked value is the bug.
func TestWI4PIIFieldsDeclareRedaction(t *testing.T) {
	layer := wi4Layer(t)
	cnic, ok := layer.FieldByID("subscriber.cnic")
	if !ok {
		t.Fatal("subscriber.cnic is not curated")
	}
	if cnic.Sensitivity != "PII" || cnic.Redaction != "MASKED" {
		t.Fatalf("cnic sensitivity=%s redaction=%s; want PII/MASKED", cnic.Sensitivity, cnic.Redaction)
	}
	for _, entity := range layer.Entities {
		for _, field := range entity.Fields {
			if field.Sensitivity == "PII" && field.Redaction == "" {
				t.Errorf("%s is PII with no declared redaction", field.ID)
			}
		}
	}
}

// The join graph is what makes a cross-family question expressible at all.
func TestWI4JoinGraphIsDeclaredAndTypeConsistent(t *testing.T) {
	layer := wi4Layer(t)
	joins := 0
	for _, entity := range layer.Entities {
		for _, join := range entity.Joins {
			joins++
			left, leftOK := layer.FieldByID(join.LeftFieldID)
			right, rightOK := layer.FieldByID(join.RightFieldID)
			if !leftOK || !rightOK {
				t.Errorf("join %s references an unknown field", join.ID)
				continue
			}
			if left.Type != right.Type {
				t.Errorf("join %s crosses types %s/%s", join.ID, left.Type, right.Type)
			}
		}
	}
	if joins == 0 {
		t.Fatal("no joins declared; cross-family questions stay inexpressible")
	}
}

// --- validator: each rule must actually reject ------------------------------

func wi4WriteLayer(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "e.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

const wi4MinimalEntity = `contract_version: forensics.semantic-layer/v1
family: test_family
record_type: test
display_name: Test
description: A test entity.
synonyms: [test]
default_measure: test.record_count
fields:
  - id: test.name
    display_name: Name
    description: A name.
    source_names: [name]
    synonyms: [name]
    type: STRING
    sensitivity: NONE
    allowed_filters: [EQ]
    allowed_aggregates: [COUNT]
    projectable: true
    groupable: true
    sortable: true
metrics:
  - id: test.record_count
    display_name: Rows
    description: Row count.
    synonyms: [count]
    aggregate: COUNT
    type: NUMBER
`

func TestWI4ValidatorAcceptsAMinimalEntity(t *testing.T) {
	if _, err := LoadSemanticLayer(wi4WriteLayer(t, wi4MinimalEntity)); err != nil {
		t.Fatalf("a minimal valid entity must load: %v", err)
	}
}

func TestWI4ValidatorRejectsEachViolation(t *testing.T) {
	cases := map[string]struct{ old, new, wantErr string }{
		"missing description":  {"    description: A name.\n", "    description: \"\"\n", "no description"},
		"no source names":      {"    source_names: [name]\n", "    source_names: []\n", "source_names"},
		"unknown type":         {"    type: STRING\n", "    type: MYSTERY\n", "unknown type"},
		"sum over text":        {"    allowed_aggregates: [COUNT]\n", "    allowed_aggregates: [COUNT, SUM]\n", "cannot allow SUM"},
		"unknown filter":       {"    allowed_filters: [EQ]\n", "    allowed_filters: [EQ, SOUNDS_LIKE]\n", "unknown filter"},
		"dangling default":     {"default_measure: test.record_count\n", "default_measure: test.nope\n", "resolves to no metric or field"},
		"ambiguous synonym":    {"    synonyms: [name]\n", "    synonyms: [name, rows]\n", "ambiguous synonym"},
		"distinct not capable": {"    aggregate: COUNT\n", "    aggregate: COUNT_DISTINCT\n    field_id: test.name\n", "distinct_capable"},
	}
	for label, c := range cases {
		t.Run(label, func(t *testing.T) {
			body := wi4MinimalEntity
			if label == "ambiguous synonym" {
				// "rows" is already the metric display name; collide two FIELDS instead.
				body = strings.Replace(body, "    synonyms: [name]\n",
					"    synonyms: [name, label]\n", 1) + `  - id: test.other
    display_name: Other
    description: Another field.
    source_names: [other]
    synonyms: [label]
    type: STRING
    sensitivity: NONE
    allowed_filters: [EQ]
    allowed_aggregates: [COUNT]
    projectable: true
    groupable: true
    sortable: true
`
				// the second field block must sit under fields:, before metrics:
				body = wi4MinimalEntity
				body = strings.Replace(body, "metrics:\n", `  - id: test.other
    display_name: Other
    description: Another field.
    source_names: [other]
    synonyms: [name]
    type: STRING
    sensitivity: NONE
    allowed_filters: [EQ]
    allowed_aggregates: [COUNT]
    projectable: true
    groupable: true
    sortable: true
metrics:
`, 1)
			} else {
				if !strings.Contains(body, c.old) {
					t.Fatalf("fixture does not contain %q", c.old)
				}
				body = strings.Replace(body, c.old, c.new, 1)
			}
			_, err := LoadSemanticLayer(wi4WriteLayer(t, body))
			if err == nil {
				t.Fatalf("%s must be rejected", label)
			}
			if !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("error %q does not mention %q", err.Error(), c.wantErr)
			}
		})
	}
}

// An unknown YAML key is a typo, and a silently ignored typo is a field that quietly
// does not exist.
func TestWI4ValidatorRejectsUnknownKeys(t *testing.T) {
	body := strings.Replace(wi4MinimalEntity, "    sortable: true\n", "    sortable: true\n    sortible: true\n", 1)
	if _, err := LoadSemanticLayer(wi4WriteLayer(t, body)); err == nil {
		t.Fatal("an unknown key must be rejected, not ignored")
	}
}

// A metric may never carry raw SQL.
func TestWI4ValidatorRejectsExpressionsOutsideTheAllowlist(t *testing.T) {
	body := strings.Replace(wi4MinimalEntity,
		"    aggregate: COUNT\n    type: NUMBER\n",
		"    aggregate: MAX\n    expression: \"(SELECT 1)\"\n    type: NUMBER\n", 1)
	_, err := LoadSemanticLayer(wi4WriteLayer(t, body))
	if err == nil || !strings.Contains(err.Error(), "allowlist") {
		t.Fatalf("raw SQL in an expression must be rejected; got %v", err)
	}
}

// --- WI-5 item 0: the layer is wired into the compiler ----------------------

func wi5CuratedCatalog(t *testing.T, family string) []FieldDescriptorV1 {
	t.Helper()
	entity, ok := wi4Layer(t).EntityByFamily(family)
	if !ok {
		t.Fatalf("family %s is not curated", family)
	}
	return entity.CatalogFields(hybridQueryRequest{TenantID: "t", CollectionID: "c"})
}

// The curated catalogue must carry meaning the inferred one cannot.
func TestWI5CuratedCatalogCarriesDescriptions(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "communications_cdr")
	if len(catalog) == 0 {
		t.Fatal("no curated CDR fields")
	}
	for _, field := range catalog {
		if field.Description == "" || field.DisplayName == "" {
			t.Errorf("%s reached the catalogue without a description or display name", field.FieldID)
		}
		if !field.Curated {
			t.Errorf("%s is not marked curated", field.FieldID)
		}
	}
	byID := sourceNativeFieldMap(catalog)
	if byID["cdr.call_type"].EffectiveType != "STRING" {
		t.Fatalf("cdr.call_type type = %q", byID["cdr.call_type"].EffectiveType)
	}
}

// SUB-02. One concept stored under two headers must become ONE catalogue field
// whose SourceNames carry both, because sourceNativeValue walks that list.
func TestWI5CuratedCatalogUnionsSourceAliases(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "subscriber_identity")
	field, ok := sourceNativeFieldMap(catalog)["subscriber.status"]
	if !ok {
		t.Fatal("subscriber.status is missing from the curated catalogue")
	}
	for _, want := range []string{"status", "account_status"} {
		if !containsString(field.SourceNames, want) {
			t.Fatalf("subscriber.status SourceNames=%v, must include %q", field.SourceNames, want)
		}
	}
}

// PII must not reach the dynamic catalogue. The inferred catalogue drops it by
// name; the curated path must not quietly widen exposure.
func TestWI5CuratedCatalogExcludesPII(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "subscriber_identity")
	for _, field := range catalog {
		if field.FieldID == "subscriber.cnic" || field.FieldID == "subscriber.full_name" {
			t.Fatalf("PII field %s reached the dynamic catalogue", field.FieldID)
		}
	}
}

// The invariant is "never advertise an aggregate the plan cannot execute".
// COUNT_DISTINCT used to fail it because the typed plan had no distinct
// aggregate; it has one now, so the rule inverts: a distinct_capable field must
// OFFER it, and a field that is not distinct_capable must not.
func TestWI5DistinctAggregateMatchesWhatThePlanCanExecute(t *testing.T) {
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable")
	}
	offered := 0
	for _, family := range []string{"communications_cdr", "anpr_vehicles", "subscriber_identity"} {
		entity, ok := layer.EntityByFamily(family)
		if !ok {
			continue
		}
		capable := map[string]bool{}
		for _, field := range entity.Fields {
			capable[field.ID] = field.DistinctCapable
		}
		for _, field := range wi5CuratedCatalog(t, family) {
			advertised := containsString(field.AllowedAggregates, "COUNT_DISTINCT")
			if advertised && !capable[field.FieldID] {
				t.Errorf("%s offers COUNT_DISTINCT but the layer does not mark it distinct_capable", field.FieldID)
			}
			if !advertised && capable[field.FieldID] {
				t.Errorf("%s is distinct_capable but COUNT_DISTINCT is withheld", field.FieldID)
			}
			if advertised {
				offered++
			}
		}
	}
	if offered == 0 {
		t.Fatal("no field offered COUNT_DISTINCT; the guard proves nothing")
	}
}

// The merge adds meaning without removing reach.
func TestWI5MergeKeepsInferredFieldsTheLayerDoesNotDescribe(t *testing.T) {
	curated := wi5CuratedCatalog(t, "communications_cdr")
	inferred := []FieldDescriptorV1{
		{FieldID: "fld_known", NormalizedName: "call_type", SourceNames: []string{"CALL_TYPE"}},
		{FieldID: "fld_novel", NormalizedName: "operator_note", SourceNames: []string{"operator_note"}},
	}
	merged := mergeSemanticLayerCatalog(curated, inferred)
	byID := sourceNativeFieldMap(merged)
	if _, ok := byID["fld_novel"]; !ok {
		t.Fatal("an inferred field the layer does not describe was dropped")
	}
	if _, ok := byID["fld_known"]; ok {
		t.Fatal("an inferred duplicate of a curated field survived the merge")
	}
	if _, ok := byID["cdr.call_type"]; !ok {
		t.Fatal("the curated field is missing after the merge")
	}
}

// The SUB-02 fix end to end at the plan level: "active" becomes a filter.
func TestWI5ValueLiteralBecomesAFilter(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "subscriber_identity")
	question := "How many subscribers are active?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan == nil {
		t.Fatalf("expected a plan, got nil (%s)", reason)
	}
	if len(plan.Filters) != 1 {
		t.Fatalf("expected the ACTIVE filter, got %+v", plan.Filters)
	}
	got := plan.Filters[0]
	if got.FieldID != "subscriber.status" || got.Op != "EQ" || got.Value != "ACTIVE" {
		t.Fatalf("filter = %+v; want subscriber.status EQ ACTIVE", got)
	}
}

// A question naming no declared value must not acquire a filter from ordinary
// English. Inventing a filter is as wrong as dropping one.
func TestWI5NoSpuriousValueFilter(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "subscriber_identity")
	question := "How many subscribers are there?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan == nil {
		t.Fatalf("expected a plan, got nil (%s)", reason)
	}
	if len(plan.Filters) != 0 {
		t.Fatalf("a plain count acquired filters: %+v", plan.Filters)
	}
}

// ACC-04: "status 500" must bind, and it must bind to the status field.
func TestWI5HTTPStatusLiteralBinds(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "access_security_logs")
	question := "How many requests failed with a server error (status 500)?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan == nil {
		t.Fatalf("expected a plan, got nil (%s)", reason)
	}
	bound := false
	for _, filter := range plan.Filters {
		if filter.FieldID == "access_log.status" && filter.Value == "500" {
			bound = true
		}
	}
	if !bound {
		t.Fatalf("the 500 literal did not bind: %+v", plan.Filters)
	}
}

// A missing layer must degrade to the inferred catalogue, never fail the request.
func TestWI5MissingLayerFallsBackToInferred(t *testing.T) {
	inferred := []FieldDescriptorV1{{FieldID: "fld_a", NormalizedName: "x", SourceNames: []string{"x"}}}
	got, state := semanticLayerCatalogForRequest(hybridQueryRequest{Query: "how many emails are in this case?"}, "how many emails are in this case?", inferred)
	if len(got) != 1 || got[0].FieldID != "fld_a" {
		t.Fatalf("a family with no curated entity must return the inferred catalogue, got %+v", got)
	}
	if state == "" {
		t.Fatal("the catalogue decision must be recorded for audit")
	}
}

// CDR-13. "Incoming versus outgoing" names two values of one field, which is a
// request to break down BY that field. Filtering to one of them answers a
// narrower question than was asked — it returned the incoming count alone.
func TestWI5MultipleValuesOfOneFieldMeanGroupNotFilter(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "communications_cdr")
	question := "How many incoming versus outgoing calls are there?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan == nil {
		t.Fatalf("expected a plan, got nil (%s)", reason)
	}
	if len(plan.Filters) != 0 {
		t.Fatalf("a breakdown must not filter to one value: %+v", plan.Filters)
	}
	if len(plan.GroupFields) != 1 || plan.GroupFields[0] != "cdr.direction" {
		t.Fatalf("group_fields = %v; want [cdr.direction]", plan.GroupFields)
	}
}

// A single named value still filters — the grouping rule must not swallow it.
func TestWI5SingleValueStillFilters(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "communications_cdr")
	question := "How many SMS records are there?"
	frame := extractSemanticFrame(hybridQueryRequest{Query: question})
	plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
	if plan == nil {
		t.Fatalf("expected a plan, got nil (%s)", reason)
	}
	found := false
	for _, filter := range plan.Filters {
		if filter.FieldID == "cdr.call_type" && filter.Value == "SMS" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a single named value must still bind: %+v", plan.Filters)
	}
}

// Analyst-facing text must use the curated display name, not the raw source
// column. "5 inbound outbound ind values" is the same defect class as a column
// headed "M1".
func TestWI5AnswerTextUsesCuratedDisplayName(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "communications_cdr")
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		GroupFields: []string{"cdr.direction"},
		Measures:    []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}}}
	resp := hybridQueryResponse{Records: map[string]any{
		"plan": plan, "complete": true, "field_catalog": catalog,
		"source_native_results": []map[string]any{
			{"m1": 2906, "inbound_outbound_ind": "INCOMING"},
			{"m1": 2592, "inbound_outbound_ind": "OUTGOING"},
		}}}
	answer, ok := buildResultAnswer(hybridQueryRequest{RecordType: "cdr"}, resp, nil)
	if !ok {
		t.Fatal("expected an answer")
	}
	if strings.Contains(answer.Headline, "inbound outbound ind") {
		t.Fatalf("raw source name leaked into the analyst text: %q", answer.Headline)
	}
	if !strings.Contains(strings.ToLower(answer.Headline), "call direction") {
		t.Fatalf("headline does not use the curated display name: %q", answer.Headline)
	}
}

// --- WI-5 items 3 and 4 -----------------------------------------------------

// The analyst-facing headline must never describe the machinery. These exact
// sentences reached five analysts' screens in the 2026-09-21 run.
func TestWI5ExecutiveAnswerNeverReturnsProcessJargon(t *testing.T) {
	for _, jargon := range []string{
		"Executed bounded source-native typed algebra over 8642 authorized source rows and returned 1 deterministic results with contribution lineage.",
		"Queried forensic.records with parameterized canonical filters, paging, totals.",
		"Structured records row count computed.",
	} {
		if !factPacketPlumbingText(jargon) {
			t.Errorf("not recognised as plumbing: %q", jargon)
		}
	}
	resp := hybridQueryResponse{
		Template: "canonical_records",
		Answer: map[string]any{
			"records_summary":   "Executed bounded source-native typed algebra over 8642 authorized source rows and returned 1 deterministic results with contribution lineage.",
			"records_row_count": 8642,
		},
		Records: map[string]any{},
	}
	got := enterpriseExecutiveAnswer(hybridQueryRequest{RecordType: "cdr"}, resp, []map[string]any{{"a": 1}})
	if factPacketPlumbingText(got) || strings.Contains(strings.ToLower(got), "algebra") {
		t.Fatalf("jargon reached the analyst headline: %q", got)
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("rejecting jargon must not leave the analyst with nothing")
	}
}

// SUB-02. A registered template returns a fixed breakdown and cannot carry the
// filter the question asks for, so a bound value literal must hand the question
// to the typed plan.
func TestWI5ValueLiteralPrefersTheTypedPlan(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "subscriber_identity")
	filters := appendSourceNativeValueFilters(nil, "How many subscribers are active?", catalog)
	if len(filters) != 1 || filters[0].FieldID != "subscriber.status" {
		t.Fatalf("expected the ACTIVE value filter to bind, got %+v", filters)
	}
	// A question naming no declared value must NOT preempt a registered match.
	if plain := appendSourceNativeValueFilters(nil, "How many subscribers are there?", catalog); len(plain) != 0 {
		t.Fatalf("a plain count must not preempt the registered path: %+v", plain)
	}
}

// CDR-06. A ranking whose top group has no value is not an answer. "The
// counterparty with the highest count is (no value)" is a confident non-answer
// and is worse than declining — the data does not support the ranking asked for.
func TestWI5RankedAnswerRefusesANullWinner(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "communications_cdr")
	plan := &SourceNativePlanV1{ContractVersion: sourceNativePlanContractV1,
		GroupFields: []string{"cdr.dialed_number"},
		Measures:    []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		Sort:        []SourceNativeSortV1{{Target: "m1", Direction: "DESC"}}, Limit: 1}
	resp := hybridQueryResponse{Records: map[string]any{
		"plan": plan, "complete": true, "field_catalog": catalog,
		"source_native_results": []map[string]any{{"m1": 2592, "call_dialed_num": nil}}}}
	if answer, ok := buildResultAnswer(hybridQueryRequest{RecordType: "cdr"}, resp, nil); ok {
		t.Fatalf("a null winner must not be presented as an answer: %q", answer.Headline)
	}
	// A real winner still answers.
	resp.Records["source_native_results"] = []map[string]any{{"m1": 872, "call_dialed_num": "923009998887"}}
	answer, ok := buildResultAnswer(hybridQueryRequest{RecordType: "cdr"}, resp, nil)
	if !ok || !strings.Contains(answer.Headline, "923009998887") {
		t.Fatalf("a real winner must still answer; ok=%v headline=%q", ok, answer.Headline)
	}
}

// CDR-06. "Which phone number made the most calls" must group by the subscriber
// number. Before curated synonyms were consulted, the hint "phone number" was
// scored against the normalized name "msisdn", resolved to cdr.call_type, failed,
// and the question lost its grouping entirely.
func TestWI5CuratedSynonymResolvesTheGroupingDimension(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "communications_cdr")
	cases := map[string]string{
		"Which phone number made the most calls?": "cdr.msisdn",
		"Which cell site handled the most calls?": "cdr.cell_site_id",
		"Which call type has the most records?":   "cdr.call_type",
	}
	for question, want := range cases {
		frame := extractSemanticFrame(hybridQueryRequest{Query: question})
		plan, reason, _ := compileDeterministicSourceNativePlan(frame, question, catalog, map[string]float64{})
		if plan == nil {
			t.Errorf("%q: expected a plan (%s)", question, reason)
			continue
		}
		if len(plan.GroupFields) != 1 || plan.GroupFields[0] != want {
			t.Errorf("%q: group_fields=%v, want [%s]", question, plan.GroupFields, want)
		}
	}
}

// Exact match only: a synonym that merely resembles a field must not bind it,
// because a near-match on a structural role is the D2 defect.
func TestWI5CuratedNameMatchingIsExactOnly(t *testing.T) {
	catalog := wi5CuratedCatalog(t, "communications_cdr")
	if f, ok := sourceNativeFieldByCuratedName("phone", catalog, nil); ok {
		t.Fatalf("partial hint %q bound %s; matching must be exact", "phone", f.FieldID)
	}
	if f, ok := sourceNativeFieldByCuratedName("phone number", catalog, nil); !ok || f.FieldID != "cdr.msisdn" {
		t.Fatalf("exact synonym must bind cdr.msisdn, got %s ok=%v", f.FieldID, ok)
	}
}

// ACC-03. The keyword ladder chose a cross-family entity operation for an
// access-log question and answered with phone and location counts. A template
// whose family contradicts the question's record type must be refused.
func TestWI5LadderRefusesAForeignFamilyTemplate(t *testing.T) {
	if runtimeTemplateFamilyAllowed("Which IP address made the most requests?", "entity_activity") {
		t.Fatal("a cross-family entity template must not serve an access-log question")
	}
	if runtimeTemplateFamilyAllowed("Which domain was accessed most often?", "top_locations") {
		t.Fatal("a CDR location template must not serve an IPDR question")
	}
	// In-family templates, family-agnostic templates and the canonical fallback
	// all stay allowed: precision over recall.
	if !runtimeTemplateFamilyAllowed("Which IP address made the most requests?", "access_failed_events") {
		t.Fatal("an in-family access-log template must stay allowed")
	}
	if !runtimeTemplateFamilyAllowed("Which IP address made the most requests?", "canonical_records") {
		t.Fatal("canonical_records must always be allowed")
	}
	// A question naming no record type must not have its template filtered.
	if !runtimeTemplateFamilyAllowed("Give me an overview of this case", "entity_activity") {
		t.Fatal("a question naming no record type must not be filtered")
	}
}

// DOC-01. A document search naming a plate is still a document search. The
// family guard judges structured families only; refusing a retrieval template on
// a structured record type sent "search the case documents for mentions of plate
// MN1367" to the ANPR rows.
func TestWI5LadderGuardDoesNotJudgeRetrievalTemplates(t *testing.T) {
	for _, tpl := range []string{"document_search", "audio_transcript_search"} {
		if !runtimeTemplateFamilyAllowed("Search the case documents for mentions of plate MN1367", tpl) {
			t.Errorf("%s must stay allowed for a document question", tpl)
		}
	}
	// Structured cross-family and foreign structured templates stay refused.
	if runtimeTemplateFamilyAllowed("Which IP address made the most requests?", "entity_activity") {
		t.Fatal("a cross-family template must not serve a single-family question")
	}
}
