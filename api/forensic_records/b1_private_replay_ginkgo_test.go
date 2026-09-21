package main

import (
	"encoding/json"
	"os"
	"time"

	"github.com/mudler/LocalAI/pkg/forensictext"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("B1 private frozen artifact replay", func() {
	It("replays original contracts and explicit phrase semantics against frozen independent artifact IDs", func() {
		path := os.Getenv("NXB21_PRIVATE_FIXTURE")
		if path == "" {
			Skip("requires ignored frozen A oracle; no retained requests are sent")
		}
		var cases []struct {
			ID       string             `json:"id"`
			Request  hybridQueryRequest `json:"request"`
			Rows     []map[string]any   `json:"rows"`
			IDs      []string           `json:"oracle_artifact_ids"`
			Positive bool               `json:"expected_positive"`
			Hash     string             `json:"literal_sha256"`
		}
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(data, &cases)).To(Succeed())
		Expect(cases).To(HaveLen(14))
		report := []map[string]any{}
		for _, c := range cases {
			original := governedDerivedTextEvidence(c.Rows, c.Request)
			req := c.Request
			req.TextQuery = &forensictext.Query{LiteralText: req.ExactTerm, MatchSemantic: forensictext.PhraseContains}
			start := time.Now()
			result := governedDerivedTextEvidence(c.Rows, req)
			latency := time.Since(start).Microseconds()
			ids := map[string]bool{}
			for _, r := range evidenceResults(result) {
				ids[stringValueAny(r["id"])] = true
			}
			passed := len(ids) == 0
			if c.Positive {
				passed = true
				for _, id := range c.IDs {
					passed = passed && ids[id]
				}
			}
			report = append(report, map[string]any{"id": c.ID, "query_sha256": c.Hash, "original_state": original["result_state"], "original_rows": len(evidenceResults(original)), "explicit_state": result["result_state"], "explicit_rows": len(ids), "contract_pass": passed, "latency_us": latency, "synthesis_requested": false, "retained_requests": false})
			Expect(passed).To(BeTrue(), "case label: "+c.ID)
		}
		if output := os.Getenv("NXB21_PRIVATE_RESULT"); output != "" {
			data, err = json.MarshalIndent(report, "", "  ")
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(output, data, 0600)).To(Succeed())
		}
	})
})
