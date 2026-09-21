package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("D historical variant classification", func() {
	It("classifies every historical query at typed-plan level", func() {
		input := os.Getenv("NXB21_D_RECONCILIATION")
		output := os.Getenv("NXB21_D_VARIANT_RESULT")
		if input == "" || output == "" {
			Skip("requires explicit D source artifact paths")
		}
		raw, err := os.ReadFile(input)
		Expect(err).NotTo(HaveOccurred())
		var inventory struct {
			QueryLedger []struct {
				VariantID             string         `json:"variant_id"`
				Text                  string         `json:"text"`
				Language              string         `json:"language"`
				CanonicalOperation    string         `json:"canonical_operation"`
				ExpectedIntent        string         `json:"expected_intent"`
				ExpectedFamily        string         `json:"expected_family"`
				ExpectedTarget        string         `json:"expected_target"`
				ExpectedDirection     string         `json:"expected_direction"`
				ClarificationRequired bool           `json:"clarification_required"`
				FollowUpRequirements  map[string]any `json:"follow_up_requirements"`
				Status                string         `json:"status"`
			} `json:"query_ledger"`
		}
		Expect(json.Unmarshal(raw, &inventory)).To(Succeed())
		Expect(inventory.QueryLedger).To(HaveLen(214))
		results := make([]map[string]any, 0, len(inventory.QueryLedger))
		for _, historical := range inventory.QueryLedger {
			req := hybridQueryRequest{Query: historical.Text, Limit: 20, MaxKBResults: 8, TenantID: "tenant-a", UserID: "analyst-a", CollectionID: "case-a"}
			plan := planRuntimeQuery(req)
			if len(historical.FollowUpRequirements) > 0 {
				priorTarget, _ := historical.FollowUpRequirements["prior_target"].(string)
				priorOperation, _ := historical.FollowUpRequirements["prior_operation"].(string)
				priorDirection, _ := historical.FollowUpRequirements["prior_direction"].(string)
				req.ConversationContext = queryConversationContext{ContractVersion: followUpContextContractV1, TenantID: req.TenantID, UserID: req.UserID, CollectionID: req.CollectionID, Target: priorTarget, Template: queryTemplateNameByOperationID(priorOperation), Direction: priorDirection, ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)}
				req, plan = applyAuditableConversationContext(req, plan, time.Now().UTC())
			}
			actualOperation := ""
			if template, ok := queryTemplateByName(plan.Template); ok {
				actualOperation = template.OperationID
			}
			classification := "FAIL"
			switch {
			case actualOperation == historical.CanonicalOperation && string(plan.Intent) == historical.ExpectedIntent:
				classification = "EXACT_PLAN_PARITY"
			case actualOperation == historical.CanonicalOperation:
				classification = "COMPATIBLE_PLAN_PARITY"
			case historical.ClarificationRequired && actualOperation == "":
				classification = "COMPATIBLE_PLAN_PARITY"
			case historical.Status != "pass":
				classification = "AMBIGUOUS_EXPECTATION"
			}
			results = append(results, map[string]any{"variant_id": historical.VariantID, "language": historical.Language, "expected_operation": historical.CanonicalOperation, "actual_operation": actualOperation, "expected_intent": historical.ExpectedIntent, "actual_intent": plan.Intent, "classification": classification, "duplicate_or_history_preserved": true})
		}
		payload, err := json.MarshalIndent(map[string]any{"schema_version": "nexusai.nxb21.d-214-variant-regression/v1", "classification_scope": "QUESTION_TO_TYPED_PLAN_SOURCE; not result certification", "records": results}, "", "  ")
		Expect(err).NotTo(HaveOccurred())
		Expect(os.MkdirAll(filepath.Dir(output), 0o755)).To(Succeed())
		Expect(os.WriteFile(output, append(payload, '\n'), 0o600)).To(Succeed())
	})
})
