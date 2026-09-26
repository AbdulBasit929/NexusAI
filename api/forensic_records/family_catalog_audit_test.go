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

// Offline audit: for EVERY golden question, against REAL data, report what the
// generator would actually be offered — before spending an hour of model time
// discovering it one question at a time. No model calls, no redeploy.
//
// Run: FAMILY_AUDIT_DB=postgresql://... go test -run TestFamilyCatalogAudit -v
func TestFamilyCatalogAudit(t *testing.T) {
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

	raw, err := os.ReadFile("../../evaluation/golden_questions_v1.json")
	if err != nil {
		t.Fatalf("golden: %v", err)
	}
	var doc struct {
		Questions []map[string]any `json:"questions"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse: %v", err)
	}

	collection := os.Getenv("FAMILY_AUDIT_COLLECTION")
	if collection == "" {
		collection = "nexusai-forensic-demo"
	}
	tenant := os.Getenv("FAMILY_AUDIT_TENANT")
	if tenant == "" {
		tenant = "default"
	}

	// Gold plans say which field IDs a correct plan must reference.
	goldRaw, err := os.ReadFile("../../scripts/ir_spike/gold_plans.json")
	if err != nil {
		t.Fatalf("gold: %v", err)
	}
	var goldDoc struct {
		Items []struct {
			ID   string `json:"id"`
			Gold any    `json:"gold"`
		} `json:"items"`
	}
	if err := json.Unmarshal(goldRaw, &goldDoc); err != nil {
		t.Fatalf("gold parse: %v", err)
	}
	goldFields := map[string][]string{}
	for _, item := range goldDoc.Items {
		plan, ok := item.Gold.(map[string]any)
		if !ok {
			continue
		}
		seen := map[string]bool{}
		collect := func(v any) {
			if s, ok := v.(string); ok && s != "" && !seen[s] {
				seen[s] = true
				goldFields[item.ID] = append(goldFields[item.ID], s)
			}
		}
		for _, key := range []string{"project", "group_fields"} {
			if list, ok := plan[key].([]any); ok {
				for _, v := range list {
					collect(v)
				}
			}
		}
		for _, key := range []string{"filters", "measures"} {
			if list, ok := plan[key].([]any); ok {
				for _, v := range list {
					if m, ok := v.(map[string]any); ok {
						collect(m["field_id"])
					}
				}
			}
		}
	}

	type row struct{ id, fam, verdict string }
	rows := []row{}
	unanswerable := []string{}
	byFamily := map[string][2]int{} // curated, total
	noFamily, starved := []string{}, []string{}

	for _, q := range doc.Questions {
		id, _ := q["id"].(string)
		question, _ := q["q"].(string)
		if id == "" || question == "" {
			continue
		}
		req := hybridQueryRequest{Query: question, TenantID: tenant, CollectionID: collection,
			QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
		family := semanticQuestionFamily(question)
		if family == "" {
			noFamily = append(noFamily, id)
			rows = append(rows, row{id, "(none)", "NO_FAMILY — generator never gets an enum"})
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
			rows = append(rows, row{id, family, "CATALOG_ERROR: " + err.Error()})
			continue
		}
		catalog := buildSourceNativeFieldCatalog(req, sampled)
		catalog, _ = semanticLayerCatalogForRequest(req, question, catalog)
		retrieved, _ := retrieveSourceNativeFields(question, catalog)

		curated := 0
		for _, f := range retrieved {
			if f.Curated {
				curated++
			}
		}
		agg := byFamily[family]
		byFamily[family] = [2]int{agg[0] + curated, agg[1] + len(retrieved)}
		verdict := fmt.Sprintf("enum=%d curated=%d hashed=%d", len(retrieved), curated, len(retrieved)-curated)
		if curated == 0 {
			verdict += "  <-- STARVED"
			starved = append(starved, id)
		}
		// The decisive check: can this question POSSIBLY be answered? If a field
		// the gold plan needs is not in the enum, the generator is structurally
		// unable to produce the right plan and scoring it measures our
		// retrieval, not the model.
		if want := goldFields[id]; len(want) > 0 {
			offered := map[string]bool{}
			for _, f := range retrieved {
				offered[f.FieldID] = true
			}
			missing := []string{}
			for _, w := range want {
				if !offered[w] {
					missing = append(missing, w)
				}
			}
			if len(missing) > 0 {
				verdict += "  !! UNANSWERABLE, gold needs " + strings.Join(missing, ",")
				unanswerable = append(unanswerable, id)
			}
		}
		// The empty-enum grammar killer must be impossible for every question.
		if bad := emptyEnums(semanticSourceNativeSchema(retrieved), "schema"); len(bad) > 0 {
			verdict += fmt.Sprintf("  !! EMPTY ENUM %v", bad)
		}
		rows = append(rows, row{id, family, verdict})
	}

	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	t.Log("=== per-question ===")
	for _, r := range rows {
		t.Logf("  %-9s %-22s %s", r.id, r.fam, r.verdict)
	}
	t.Log("=== per-family curated share of the issued enum ===")
	fams := make([]string, 0, len(byFamily))
	for f := range byFamily {
		fams = append(fams, f)
	}
	sort.Strings(fams)
	for _, f := range fams {
		a := byFamily[f]
		pct := 0.0
		if a[1] > 0 {
			pct = 100 * float64(a[0]) / float64(a[1])
		}
		t.Logf("  %-24s %d/%d curated (%.0f%%)", f, a[0], a[1], pct)
	}
	t.Logf("=== no_family (%d): %s", len(noFamily), strings.Join(noFamily, " "))
	t.Logf("=== starved  (%d): %s", len(starved), strings.Join(starved, " "))
	t.Logf("=== UNANSWERABLE, gold field not offered (%d): %s", len(unanswerable), strings.Join(unanswerable, " "))
}
