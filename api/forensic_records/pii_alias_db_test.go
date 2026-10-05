package main

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// THE REAL PROOF: run a masked GROUP BY over the 307 retained plate candidates
// and assert no raw plate survives anywhere in the payload.
//
// A unit test over a hand-written value proves the alias function. It does not
// prove the PATH -- that the executor applies the redaction at every place a
// group label, a projected cell or a citation could carry the value. The 9,580
// defect was invisible to code review and obvious to one database test, and a
// PII leak deserves at least that much.

// plateShaped matches a bare registration-like token, which is what must NOT
// appear. Deliberately loose: it is better for this to flag an alias than to
// miss a plate.
var plateShaped = regexp.MustCompile(`\b[A-Z]{2,3}[- ]?\d{2,4}\b`)

func TestMaskedPlateGroupingLeaksNoRawPlate(t *testing.T) {
	dsn := os.Getenv("DERIVED_EXEC_DB")
	if dsn == "" {
		t.Skip("needs a live database; set DERIVED_EXEC_DB") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Setenv(derivedArtifactExecutionEnv, "true")
	t.Setenv(mediaFamilyRoutingEnv, "true")
	t.Setenv(piiMaskedProjectionEnv, "true")
	t.Setenv(piiAliasSecretEnv, testAliasSecret)

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	defer pool.Close()

	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	// video_anpr_plate_group, not anpr_model_observation: the source-native
	// GROUP CARDINALITY guard caps a grouping at 100, and the 307 individual
	// plate candidates exceed it -- that guard is correct and is left alone. The
	// 24 grouped video plate candidates exercise the same masking path within it.
	entity, ok := layer.EntityByFamily("video_anpr_plate_group")
	if !ok {
		t.Skip("family missing") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	collection := os.Getenv("DERIVED_EXEC_COLLECTION")
	if collection == "" {
		collection = "nexusai-multimodal-product-acceptance"
	}
	req := hybridQueryRequest{
		Query:        "How many supporting reads are there for each video plate group?",
		TenantID:     "default",
		CollectionID: collection,
		QueryScope:   queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)},
		Limit:        100,
	}
	catalog := entity.CatalogFields(req)

	var plateField string
	for _, field := range catalog {
		if strings.Contains(field.FieldID, "plate_text") {
			plateField = field.FieldID
		}
	}
	if plateField == "" {
		t.Fatal("plate_text was not issued, so this test proves nothing about masking it") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}

	plan := &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Project:         []string{}, Filters: []SourceNativeFilterV1{},
		GroupFields: []string{plateField},
		Measures:    []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT"}},
		Having:      []SourceNativeHavingV1{}, Sort: []SourceNativeSortV1{}, Limit: 100,
	}
	records, err := executeSourceNativePlanSQL(ctx, pool, req, plan, catalog)
	if err != nil {
		t.Fatalf("the masked grouping did not execute: %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}

	results := mapsFromAny(records["source_native_results"])
	if len(results) == 0 {
		t.Fatal("the grouping returned no rows, so this proves nothing") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}

	// THE WHOLE PAYLOAD, not just the cells this test knows to look at. A leak
	// through a citation, a metadata echo or a field catalogue entry is still a
	// leak, and checking only the group label would miss it.
	blob, err := json.Marshal(records)
	if err != nil {
		t.Fatalf("marshal: %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	upper := strings.ToUpper(string(blob))

	// Independent oracle: the actual retained plate values, read straight from
	// the table. This is the set that must not appear.
	rows, err := pool.Query(ctx,
		`SELECT DISTINCT metadata->'observation'->>'normalized_plate_text'
		 FROM forensic.derived_artifacts
		 WHERE tenant_id=$1 AND collection_id=$2
		   AND artifact_type='forensics.video-anpr-plate-group/v1' AND processing_status='completed'
		   AND metadata->'observation'->>'normalized_plate_text' IS NOT NULL`,
		req.TenantID, collection)
	if err != nil {
		t.Fatalf("oracle query: %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	defer rows.Close()
	checked, leaked := 0, []string{}
	for rows.Next() {
		var plate string
		if err := rows.Scan(&plate); err != nil {
			t.Fatalf("scan: %v", err) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
		normalized := normalizePlateForAlias(plate)
		if normalized == "" {
			continue
		}
		checked++
		if strings.Contains(upper, normalized) || strings.Contains(upper, strings.ToUpper(plate)) {
			leaked = append(leaked, plate)
		}
	}
	if checked == 0 {
		t.Fatal("the oracle found no plate values, so this proves nothing") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	if len(leaked) > 0 {
		shown := leaked
		if len(shown) > 3 {
			shown = shown[:3]
		}
		t.Errorf("%d of %d retained plates appear RAW in the payload, e.g. %v", len(leaked), checked, shown) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}

	// Every group label must be an alias of the expected shape.
	aliased := 0
	for _, row := range results {
		for key, value := range row {
			if key == "metadata" || key == "m1" || strings.HasSuffix(key, "_count") {
				continue
			}
			text := strings.TrimSpace(stringValueAny(value))
			if text == "" {
				continue
			}
			if !strings.HasPrefix(text, "Plate candidate ") {
				t.Errorf("group label %q is not an alias", text) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
				continue
			}
			if plateShaped.MatchString(strings.TrimPrefix(text, "Plate candidate ")) {
				t.Errorf("alias %q still looks like a registration", text) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
			}
			aliased++
		}
	}
	if aliased == 0 {
		t.Fatal("no group label was checked, so the masking assertion is vacuous") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Logf("%d group labels, all aliased; %d distinct retained plates, none present raw", aliased, checked) //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
}

// THE NEGATIVE CONTROL. With masking unavailable the field must not be issued at
// all -- which is what makes the test above non-vacuous: it is not passing
// because the grouping quietly returned nothing.
func TestWithoutMaskingThePlateFieldIsNotEvenIssued(t *testing.T) {
	if os.Getenv("DERIVED_EXEC_DB") == "" {
		t.Skip("needs a live database; set DERIVED_EXEC_DB") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	t.Setenv(piiMaskedProjectionEnv, "true")
	t.Setenv(piiAliasSecretEnv, "")
	layer, _ := defaultSemanticLayer()
	if layer == nil {
		t.Skip("curated layer unavailable") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	entity, ok := layer.EntityByFamily("anpr_model_observation")
	if !ok {
		t.Skip("family missing") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
	}
	for _, field := range entity.CatalogFields(hybridQueryRequest{CollectionID: "nexusai-multimodal-product-acceptance"}) {
		if strings.Contains(field.FieldID, "plate_text") {
			t.Fatal("plate_text was issued with no alias secret; a plan could then project it raw") //nolint:forbidigo // test uses the standard testing package; conversion to Ginkgo is tracked in docs/work/LINT_DEBT_20261005.md
		}
	}
}
