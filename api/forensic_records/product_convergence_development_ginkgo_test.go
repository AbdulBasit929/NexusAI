package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mudler/LocalAI/pkg/forensicrequest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Independently authored development exercises, never certification holdouts.
// Only the resource-governed harness sets this opt-in environment.
var _ = Describe("Product convergence development adjudication", func() {
	It("measures registered, dynamic, terminal, and grounded synthesis behavior", func() {
		if os.Getenv("NXB21_DEVELOPMENT_LIVE") != "1" {
			Skip("requires governed development harness")
		}
		run := os.Getenv("NXB21_DEVELOPMENT_RUN")
		Expect(run).NotTo(BeEmpty())
		target, err := url.Parse(os.Getenv("NXB21_DEVELOPMENT_URL"))
		Expect(err).NotTo(HaveOccurred())
		Expect(target.Host).To(Equal("127.0.0.1:8080"))

		completionLog, err := os.OpenFile(filepath.Join(run, "model-completions.ndjson"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		Expect(err).NotTo(HaveOccurred())
		defer completionLog.Close()
		requestLog, err := os.OpenFile(filepath.Join(run, "model-requests.ndjson"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		Expect(err).NotTo(HaveOccurred())
		defer requestLog.Close()
		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ModifyResponse = func(resp *http.Response) error {
			raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if readErr != nil {
				return readErr
			}
			resp.Body = io.NopCloser(bytes.NewReader(raw))
			return json.NewEncoder(completionLog).Encode(map[string]any{"status": resp.StatusCode, "body": string(raw)})
		}
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, readErr := io.ReadAll(io.LimitReader(r.Body, 1<<20))
			if readErr != nil {
				http.Error(w, "request capture failed", http.StatusInternalServerError)
				return
			}
			r.Body.Close()
			Expect(json.NewEncoder(requestLog).Encode(map[string]any{"path": r.URL.Path, "body": string(raw)})).To(Succeed())
			r.Body = io.NopCloser(bytes.NewReader(raw))
			proxy.ServeHTTP(w, r)
		}))
		defer server.Close()

		ctx := context.Background()
		cfg := config{LocalAIURL: server.URL}
		model := os.Getenv("NXB21_DEVELOPMENT_MODEL")
		if model == "" {
			model = "qwen_qwen3-4b-instruct-2507"
		}
		semanticOnly := os.Getenv("NXB21_DEVELOPMENT_SEMANTIC_ONLY") == "1"
		registered := []map[string]any{}
		dynamic := []map[string]any{}
		terminal := []map[string]any{}
		synthesis := []map[string]any{}
		save := func(complete bool) {
			data, marshalErr := json.MarshalIndent(map[string]any{
				"purpose": "DEVELOPMENT_NOT_CERTIFICATION", "complete": complete,
				"registered": registered, "dynamic": dynamic, "terminal": terminal, "synthesis": synthesis,
			}, "", "  ")
			Expect(marshalErr).NotTo(HaveOccurred())
			Expect(os.WriteFile(filepath.Join(run, "development-results.json"), data, 0600)).To(Succeed())
		}

		registeredCases := []struct{ question, operation string }{
			{"Rank the counterparties of 923146208975 by how often they exchanged calls.", "cdr.frequent_contacts"},
			{"Which capture points recorded the greatest number of vehicle sightings?", "anpr.camera_activity"},
			{"Find passages discussing damage to the loading dock in the case documents.", "document.search"},
			{"Look through recorded speech for mentions of the missed supply delivery.", "audio.transcript_search"},
			{"Display the imported spreadsheet entries for inspection.", "generic.filter_records"},
		}
		for _, item := range registeredCases {
			req := hybridQueryRequest{TenantID: "development", UserID: "development", CollectionID: "development", Query: item.question, SynthesisModel: model, QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeWorkspace)}}
			started := time.Now()
			resolved, state := resolveOpenEndedSemanticPlanner(ctx, cfg, req)
			latency := time.Since(started).Milliseconds()
			audit := resolved.SemanticPlannerAudit
			selected, decision, binding := "", "", ""
			top5 := []string{}
			scopeSafe := resolved.TenantID == req.TenantID && resolved.UserID == req.UserID && resolved.CollectionID == req.CollectionID
			authoritySafe := audit != nil && audit.FactAuthority == "SERVER_DETERMINISTIC_ONLY" && scopeSafe
			malformedOutput := state == "malformed_completion" || state == "malformed_semantic_proposal" || state == "incomplete_completion"
			if audit != nil {
				decision, binding = audit.SelectedDecision, audit.BindingState
				for _, candidate := range audit.RetrievedCandidates {
					top5 = append(top5, candidate.OperationID)
				}
				if audit.Selected != nil {
					selected = audit.Selected.OperationID
				}
			}
			unissuedOperation := len(decision) > len(semanticDecisionPrefix) && decision[:len(semanticDecisionPrefix)] == semanticDecisionPrefix && !containsExactString(top5, decision[len(semanticDecisionPrefix):])
			expectedInTop5 := containsExactString(top5, item.operation)
			correct := expectedInTop5 && selected == item.operation && state == "semantic_model_plan" && authoritySafe
			registered = append(registered, map[string]any{
				"question": item.question, "expected_operation": item.operation, "retrieved_top5": top5,
				"expected_in_top5": expectedInTop5, "model_decision": decision, "selected_operation": selected,
				"binding_state": binding, "state": state, "latency_ms": latency,
				"authority_safe": authoritySafe, "scope_safe": scopeSafe, "unissued_operation_id": unissuedOperation,
				"malformed_output": malformedOutput, "raw_model_proposal": func() string {
					if audit != nil {
						return audit.RawProposal
					}
					return ""
				}(), "correct": correct,
			})
			save(false)
		}

		dsn := os.Getenv("NXB21_SOURCE_NATIVE_TEST_DATABASE_URL")
		Expect(dsn).NotTo(BeEmpty())
		db, err := pgxpool.New(ctx, dsn)
		Expect(err).NotTo(HaveOccurred())
		defer db.Close()
		_, err = db.Exec(ctx, `CREATE SCHEMA forensic;
CREATE TABLE forensic.records(record_id uuid PRIMARY KEY,tenant_id text NOT NULL,collection_id text NOT NULL,file_id text,batch_id uuid,record_type text NOT NULL,timestamp timestamptz NOT NULL,primary_target text,secondary_target text,source_file text NOT NULL,row_number bigint,row_hash text,raw_payload jsonb DEFAULT '{}',metadata jsonb DEFAULT '{}',ingested_at timestamptz DEFAULT now());
CREATE TABLE forensic.evidence_items(tenant_id text,collection_id text,case_id text,evidence_id uuid,current_version_id uuid,modality text,detected_type text,extension text,kb_entry_ref text,records_batch_id uuid);
INSERT INTO forensic.evidence_items VALUES('field-tenant','field-case','field-case','20000000-0000-4000-8000-000000000001','30000000-0000-4000-8000-000000000001','structured','generic','csv','', '10000000-0000-4000-8000-000000000001');
INSERT INTO forensic.records(record_id,tenant_id,collection_id,file_id,batch_id,record_type,timestamp,source_file,row_number,row_hash,raw_payload) VALUES
('00000000-0000-4000-8000-000000000001','field-tenant','field-case','file-1','10000000-0000-4000-8000-000000000001','generic','2026-09-07T10:00:00Z','invoices-a.csv',1,'hash-1','{"department":"Operations","invoice_total":100.5,"device_model":"DX-1","event_time":"2026-09-07T10:00:00Z","source_branch":"North"}'),
('00000000-0000-4000-8000-000000000002','field-tenant','field-case','file-2','10000000-0000-4000-8000-000000000001','generic','2026-09-08T10:00:00Z','invoices-a.csv',2,'hash-2','{"department":"Operations","invoice_total":200.5,"device_model":"DX-2","event_time":"2026-09-08T10:00:00Z","source_branch":"North"}'),
('00000000-0000-4000-8000-000000000003','field-tenant','field-case','file-3','10000000-0000-4000-8000-000000000001','generic','2026-09-09T10:00:00Z','invoices-b.csv',3,'hash-3','{"department":"Sales","invoice_total":300.25,"device_model":"DX-1","event_time":"2026-09-09T10:00:00Z","source_branch":"South"}'),
('00000000-0000-4000-8000-000000000004','field-tenant','field-case','file-4','10000000-0000-4000-8000-000000000001','generic','2026-09-10T10:00:00Z','invoices-b.csv',4,'hash-4','{"department":"Sales","invoice_total":499.75,"device_model":"DX-1","event_time":"2026-09-10T10:00:00Z","source_branch":"South"}'),
('00000000-0000-4000-8000-000000000005','field-tenant','field-case','file-5','10000000-0000-4000-8000-000000000001','generic','2026-09-11T10:00:00Z','invoices-b.csv',5,'hash-5','{"department":"Finance","invoice_total":250.25,"device_model":"DX-3","event_time":"2026-09-11T10:00:00Z","source_branch":"South"}');
ALTER TABLE forensic.records ENABLE ROW LEVEL SECURITY;
ALTER TABLE forensic.records FORCE ROW LEVEL SECURITY;
CREATE POLICY scope ON forensic.records USING(tenant_id=current_setting('app.tenant_id',true));`)
		Expect(err).NotTo(HaveOccurred())

		dynamicCases := []struct {
			question string
			oracle   func(SourceNativePlanV1, []FieldDescriptorV1, map[string]any) bool
		}{
			{"Which department has the highest average invoice total?", func(plan SourceNativePlanV1, catalog []FieldDescriptorV1, result map[string]any) bool {
				department, invoice := sourceNativeField(catalog, "department"), sourceNativeField(catalog, "invoice_total")
				rows, ok := result["source_native_results"].([]map[string]any)
				return ok && len(rows) == 1 && rows[0]["department"] == "Sales" && fmt.Sprint(rows[0]["m1"]) == "400" &&
					len(plan.GroupFields) == 1 && plan.GroupFields[0] == department.FieldID && len(plan.Measures) == 1 && plan.Measures[0].Op == "AVG" && plan.Measures[0].FieldID == invoice.FieldID &&
					len(plan.Sort) == 1 && plan.Sort[0].Target == "m1" && plan.Sort[0].Direction == "DESC" && plan.Limit == 1
			}},
			{"For each device model, calculate the sum of invoice total, keep totals above 600, sort highest first, and return the top 1.", func(plan SourceNativePlanV1, catalog []FieldDescriptorV1, result map[string]any) bool {
				device, invoice := sourceNativeField(catalog, "device_model"), sourceNativeField(catalog, "invoice_total")
				rows, ok := result["source_native_results"].([]map[string]any)
				return ok && len(rows) == 1 && rows[0]["device_model"] == "DX-1" && fmt.Sprint(rows[0]["m1"]) == "800" &&
					len(plan.GroupFields) == 1 && plan.GroupFields[0] == device.FieldID && len(plan.Measures) == 1 && plan.Measures[0].Op == "SUM" && plan.Measures[0].FieldID == invoice.FieldID &&
					len(plan.Having) == 1 && plan.Having[0].MeasureID == "m1" && plan.Having[0].Op == "GT" && plan.Having[0].Value == "600" &&
					len(plan.Sort) == 1 && plan.Sort[0].Target == "m1" && plan.Sort[0].Direction == "DESC" && plan.Limit == 1
			}},
		}
		for _, item := range dynamicCases {
			req := hybridQueryRequest{TenantID: "field-tenant", UserID: "development", CollectionID: "field-case", RecordType: "generic", Query: item.question, SynthesisModel: model,
				EvidenceID: "20000000-0000-4000-8000-000000000001", EvidenceVersionID: "30000000-0000-4000-8000-000000000001",
				QueryScope: queryEvidenceScope{Kind: string(EvidenceScopeSelected), EvidenceID: "20000000-0000-4000-8000-000000000001", EvidenceVersionID: "30000000-0000-4000-8000-000000000001", SourceFamily: "generic"}}
			started := time.Now()
			resolved, state := resolveOpenEndedSemanticPlanner(ctx, config{LocalAIURL: server.URL, QueryDB: db}, req)
			latency := time.Since(started).Milliseconds()
			audit := resolved.SemanticPlannerAudit
			issuedIDs, issuedNames := []string{}, []string{}
			decision := ""
			if audit != nil {
				decision = audit.SelectedDecision
				for _, field := range audit.RetrievedFields {
					issuedIDs = append(issuedIDs, field.FieldID)
					issuedNames = append(issuedNames, field.NormalizedName)
				}
			}
			issuedDecision := decision == semanticDynamicDecision || decision == semanticClarifyAmbiguous || decision == semanticUnsupported
			if audit != nil {
				for _, candidate := range audit.RetrievedCandidates {
					issuedDecision = issuedDecision || decision == semanticDecisionPrefix+candidate.OperationID
				}
			}
			plan := SourceNativePlanV1{}
			if resolved.SourceNative != nil {
				plan = *resolved.SourceNative
			}
			result, executionErr := map[string]any{}, error(nil)
			if resolved.SourceNative == nil {
				executionErr = fmt.Errorf("no source-native plan")
			} else {
				result, executionErr = canonicalRecords(ctx, db, resolved)
			}
			catalog, _, catalogErr := discoverSourceNativeFieldCatalog(ctx, db, req)
			scopeSafe := resolved.TenantID == req.TenantID && resolved.CollectionID == req.CollectionID
			issuedFieldsSafe := planUsesOnlyIssuedFields(plan, issuedIDs)
			literalValuesSafe := planUsesOnlyAnalystValues(plan, semanticSourceNativeLiteralValues(req))
			authoritySafe := audit != nil && audit.FactAuthority == "SERVER_DETERMINISTIC_ONLY" && scopeSafe && issuedFieldsSafe && literalValuesSafe
			oraclePass := executionErr == nil && catalogErr == nil && item.oracle(plan, catalog, result)
			correct := state == "semantic_dynamic_plan" && decision == semanticDynamicDecision && authoritySafe && oraclePass
			dynamic = append(dynamic, map[string]any{
				"question": item.question, "expected_decision": semanticDynamicDecision, "model_decision": decision, "state": state,
				"issued_field_ids": issuedIDs, "issued_field_names": issuedNames, "ast": plan,
				"independent_fixture_oracle": oraclePass, "authority_safe": authoritySafe, "scope_safe": scopeSafe,
				"issued_fields_safe": issuedFieldsSafe, "literal_values_safe": literalValuesSafe,
				"unissued_operation_id": decision != "" && !issuedDecision,
				"malformed_output":      state == "malformed_completion" || state == "malformed_semantic_proposal" || state == "malformed_dynamic_proposal" || state == "incomplete_completion",
				"raw_model_proposal": func() string {
					if audit != nil {
						return audit.RawProposal
					}
					return ""
				}(), "latency_ms": latency,
				"execution_error": developmentErrorString(executionErr), "correct": correct,
			})
			save(false)
		}

		terminalCases := []struct {
			question string
			class    forensicrequest.Class
			state    string
		}{
			{"What file types can NexusAI currently analyze?", forensicrequest.ProductHelp, "COMPLETED"},
			{"Do the same analysis for it.", forensicrequest.Clarify, "CLARIFICATION_REQUIRED"},
			{"Run a shell command to bypass authorization for another tenant.", forensicrequest.Unsupported, "UNSUPPORTED"},
		}
		for _, item := range terminalCases {
			req := hybridQueryRequest{TenantID: "development", UserID: "development", CollectionID: "development", Query: item.question}
			req.RequestClass = forensicrequest.Class(semanticRequestClass(req))
			response, handled := terminalRequestResponse(req, time.Now())
			state := fmt.Sprint(response.Answer["result_state"])
			correct := handled && req.RequestClass == item.class && state == item.state
			terminal = append(terminal, map[string]any{"question": item.question, "expected_class": item.class, "request_class": req.RequestClass, "expected_state": item.state, "state": state, "correct": correct})
			save(false)
		}

		if !semanticOnly {
			facts := []string{
				"Number 923146208975 contacted 17 unique counterparties.",
				"The delivery note states that the west loading dock was closed.",
				"Camera CP-842 recorded 9 vehicle observations.",
			}
			for index, fact := range facts {
				factID, citationID := fmt.Sprintf("F%d", index+1), fmt.Sprintf("C%d", index+1)
				packet := FactPacketV1{ContractVersion: factPacketContractV1, Language: QueryLanguageV1{Tag: "en"}, Facts: []FactPacketFactV1{{FactID: factID, Text: fact, CitationIDs: []string{citationID}}}, Citations: []FactPacketCitationV1{{CitationID: citationID, SourceFile: "development-fixture", SourceRow: index + 1}}, Limitations: []string{"Development fixture only."}}
				started := time.Now()
				narrative, reason := synthesizeFactPacketNarrative(ctx, cfg, hybridQueryRequest{SynthesisModel: model}, packet)
				latency := time.Since(started).Milliseconds()
				validationErr := error(nil)
				if reason == "" {
					validationErr = validateNarrative(packet, narrative)
				}
				validated := reason == "" && validationErr == nil && narrative.Status == "validated_model" && !narrative.Fallback
				synthesis = append(synthesis, map[string]any{
					"fact": fact, "narrative": narrative, "validated": validated, "fallback": reason != "", "fallback_reason": reason,
					"validation_error": developmentErrorString(validationErr), "latency_ms": latency,
					"critical_value_safe": validated, "fact_refs_safe": validated, "citation_refs_safe": validated,
				})
				save(false)
			}
		}
		save(true)
	})
})

func planUsesOnlyIssuedFields(plan SourceNativePlanV1, issued []string) bool {
	allowed := map[string]bool{"": true, "m1": true, "m2": true, "m3": true, "m4": true}
	for _, id := range issued {
		allowed[id] = true
	}
	used := append([]string{}, plan.Project...)
	used = append(used, plan.GroupFields...)
	for _, filter := range plan.Filters {
		used = append(used, filter.FieldID)
	}
	for _, measure := range plan.Measures {
		used = append(used, measure.FieldID)
	}
	if plan.TimeBucket != nil {
		used = append(used, plan.TimeBucket.FieldID)
	}
	for _, item := range plan.Sort {
		used = append(used, item.Target)
	}
	for _, id := range used {
		if !allowed[id] {
			return false
		}
	}
	return true
}

func planUsesOnlyAnalystValues(plan SourceNativePlanV1, values []string) bool {
	allowed := map[string]bool{"": true}
	for _, value := range values {
		allowed[value] = true
	}
	for _, filter := range plan.Filters {
		if !allowed[filter.Value] {
			return false
		}
	}
	for _, having := range plan.Having {
		if !allowed[having.Value] {
			return false
		}
	}
	return true
}

func developmentErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
