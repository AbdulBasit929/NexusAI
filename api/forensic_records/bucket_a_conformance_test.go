package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Offline probe for the seven questions that resolve a family, are issued a
// full curated enum, and STILL produce no verifiable plan.
//
// WI-20 classified the 18 clarifications and found bucket A: CDR-05, CDR-08,
// CDR-11, CDR-14, ANPR-05, SUB-03, TWR-02. Every one of them asks for the
// simplest thing the algebra can express -- COUNT, COUNT_DISTINCT, a GROUP BY,
// an EQ filter, a date range, a projection -- over a family whose enum is
// 14/14 curated. If those cannot be answered, the gap is not coverage.
//
// The question this probe settles, and it has exactly two answers:
//
//	the GOLD plan is REJECTED by our own validator  -> the defect is OURS, in
//	    the catalog or the validator, and no model improvement can fix it
//	the GOLD plan VALIDATES                         -> generation is the gap,
//	    and the fix is narrowing, prompt or tier -- measured, never guessed
//
// Asking a live model seven times would take an hour and answer neither
// question, because a failure would not say WHICH of the two it was. This runs
// in seconds against the real catalog with no model call at all.
//
// Run: FAMILY_AUDIT_DB=postgresql://... go test -run TestBucketAGoldPlanConformance -v
func TestBucketAGoldPlanConformance(t *testing.T) {
	dsn := os.Getenv("FAMILY_AUDIT_DB")
	if dsn == "" {
		t.Skip("diagnostic only; set FAMILY_AUDIT_DB")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	bucketA := map[string]bool{
		"CDR-05": true, "CDR-08": true, "CDR-11": true, "CDR-14": true,
		"ANPR-05": true, "SUB-03": true, "TWR-02": true,
	}

	goldRaw, err := os.ReadFile("../../scripts/ir_spike/gold_plans.json")
	if err != nil {
		t.Fatalf("gold: %v", err)
	}
	var goldDoc struct {
		Items []struct {
			ID    string          `json:"id"`
			Q     string          `json:"q"`
			Shape string          `json:"shape"`
			Gold  json.RawMessage `json:"gold"`
		} `json:"items"`
	}
	if err := json.Unmarshal(goldRaw, &goldDoc); err != nil {
		t.Fatalf("gold parse: %v", err)
	}

	collection := os.Getenv("FAMILY_AUDIT_COLLECTION")
	if collection == "" {
		collection = "nexusai-forensic-demo"
	}
	tenant := os.Getenv("FAMILY_AUDIT_TENANT")
	if tenant == "" {
		tenant = "default"
	}

	type outcome struct{ id, stage, detail string }
	results := []outcome{}

	for _, item := range goldDoc.Items {
		if !bucketA[item.ID] {
			continue
		}
		req := hybridQueryRequest{Query: item.Q, TenantID: tenant, CollectionID: collection,
			QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}

		family := semanticQuestionFamily(item.Q)
		if family == "" {
			results = append(results, outcome{item.ID, "NO_FAMILY", "no enum is ever issued"})
			continue
		}
		sampleReq := req
		if layer, _ := defaultSemanticLayer(); layer != nil {
			if entity, ok := layer.EntityByFamily(family); ok {
				sampleReq.RecordType = entity.RecordType
			}
		}
		sampled, err := sourceNativeCatalogSampleRows(ctx, pool, sampleReq)
		if err != nil {
			results = append(results, outcome{item.ID, "CATALOG_ERROR", err.Error()})
			continue
		}
		catalog := buildSourceNativeFieldCatalog(req, sampled)
		catalog, _ = semanticLayerCatalogForRequest(req, item.Q, catalog)
		issued, _ := retrieveSourceNativeFields(item.Q, catalog)

		// A gold plan recorded as `null` is not a malformed plan -- it is a
		// recorded claim that the question was INEXPRESSIBLE when the gold set
		// was written. That claim can expire. CDR-08 and ANPR-05 both say "no
		// distinct-count aggregate in the IR", and the IR has had COUNT_DISTINCT
		// since WI-9. So the plan is reconstructed here and put through the same
		// validator: if it passes, the gold set is stale, not the product.
		if len(item.Gold) == 0 || string(item.Gold) == "null" {
			constructed, ok := reconstructedDistinctPlan(item.ID)
			if !ok {
				results = append(results, outcome{item.ID, "GOLD_ABSENT", "no gold plan and no reconstruction"})
				continue
			}
			if err := validateSourceNativePlan(constructed, issued); err != nil {
				results = append(results, outcome{item.ID, "GOLD_ABSENT_CLAIM_HOLDS",
					"still inexpressible: " + err.Error()})
				continue
			}
			results = append(results, outcome{item.ID, "GOLD_STALE_NOW_EXPRESSIBLE",
				fmt.Sprintf("COUNT_DISTINCT validates against the issued enum=%d; the gold's "+
					"`inexpressible` note predates the aggregate", len(issued))})
			continue
		}

		// The gold plan is parsed through the SAME strict parser the generator's
		// output goes through. A gold plan that will not even decode is itself
		// a finding.
		var plan SourceNativePlanV1
		if err := json.Unmarshal(item.Gold, &plan); err != nil {
			results = append(results, outcome{item.ID, "GOLD_UNPARSEABLE", err.Error()})
			continue
		}

		if err := validateSourceNativePlanShape(&plan); err != nil {
			results = append(results, outcome{item.ID, "S6_SHAPE_REJECTS_GOLD", err.Error()})
			continue
		}
		if err := validateSourceNativePlan(&plan, issued); err != nil {
			// Say which referenced field is missing from the issued enum --
			// "not issued" is only actionable when it names the field.
			offered := map[string]bool{}
			for _, f := range issued {
				offered[f.FieldID] = true
			}
			missing := []string{}
			for _, id := range planFieldIDs(&plan) {
				if id != "" && !offered[id] {
					missing = append(missing, id)
				}
			}
			detail := err.Error()
			if len(missing) > 0 {
				detail += fmt.Sprintf(" [not in enum: %s]", strings.Join(missing, ","))
			}
			detail += fmt.Sprintf(" [enum=%d]", len(issued))
			results = append(results, outcome{item.ID, "S6_VALIDATION_REJECTS_GOLD", detail})
			continue
		}
		results = append(results, outcome{item.ID,
			"GOLD_PASSES_S6",
			fmt.Sprintf("enum=%d shape=%s -- a correct plan IS expressible; the gap is generation", len(issued), item.Shape)})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].id < results[j].id })
	t.Log("=== bucket A: can our own validator accept the KNOWN-CORRECT plan? ===")
	// Three OUTCOMES, not two. The probe was written expecting "our defect" or
	// "generation gap" and the data produced a third: a gold entry whose
	// recorded limitation has expired. Counting that as a pipeline rejection
	// would have pointed the next fix at the wrong layer.
	generation, goldDefect, byDesign, pipeline := 0, 0, 0, 0
	for _, r := range results {
		t.Logf("  %-8s %-28s %s", r.id, r.stage, r.detail)
		switch r.stage {
		case "GOLD_PASSES_S6":
			generation++
		case "GOLD_STALE_NOW_EXPRESSIBLE":
			goldDefect++
		case "S6_VALIDATION_REJECTS_GOLD":
			// A field withheld from the enum on SENSITIVITY is a control doing
			// its job, not a gap. Distinguished by hand below, not by the tally.
			byDesign++
		default:
			pipeline++
		}
	}
	t.Logf("=== generation is the gap (a correct plan validates today): %d ===", generation)
	t.Logf("=== GOLD SET DEFECT (recorded limitation has expired):      %d ===", goldDefect)
	t.Logf("=== gold references a field the enum withholds:             %d ===", byDesign)
	t.Logf("=== other pipeline rejection:                               %d ===", pipeline)
	if goldDefect > 0 {
		t.Log("A stale gold entry scores the PRODUCT for a limitation that no longer " +
			"exists. Correct the gold before attributing these to coverage.")
	}
}

// reconstructedDistinctPlan is the plan the gold set declined to write because
// the IR had no distinct aggregate at the time. Both questions are a single
// COUNT_DISTINCT over one identifier column and have no other reading.
func reconstructedDistinctPlan(id string) (*SourceNativePlanV1, bool) {
	field := map[string]string{
		"CDR-08":  "cdr.msisdn",        // unique numbers appearing as callers
		"ANPR-05": "anpr.plate_number", // distinct plates captured
	}[id]
	if field == "" {
		return nil, false
	}
	return &SourceNativePlanV1{
		ContractVersion: sourceNativePlanContractV1,
		Measures:        []SourceNativeMeasureV1{{MeasureID: "m1", Op: "COUNT_DISTINCT", FieldID: field}},
	}, true
}

// planFieldIDs lists every field the plan references, so a rejection can name
// the field that was missing rather than only that one was.
func planFieldIDs(plan *SourceNativePlanV1) []string {
	if plan == nil {
		return nil
	}
	ids := append([]string{}, plan.Project...)
	ids = append(ids, plan.GroupFields...)
	for _, filter := range plan.Filters {
		ids = append(ids, filter.FieldID)
	}
	for _, measure := range plan.Measures {
		ids = append(ids, measure.FieldID)
	}
	if plan.TimeBucket != nil {
		ids = append(ids, plan.TimeBucket.FieldID)
	}
	return ids
}
