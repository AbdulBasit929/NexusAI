package agents

import (
	"github.com/mudler/LocalAI/core/services/testutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Four P1 terminal persistence", func() {
	It("persists failure, zero-result and clarification outcomes without late completion overwrites", func() {
		store, err := NewAgentStore(testutil.SetupTestDB())
		Expect(err).NotTo(HaveOccurred())
		scope := AnalysisHistoryScope{TenantID: "test", UserID: "user", AgentName: "agent", CaseID: "case", CollectionID: "case"}
		for _, tc := range []struct{ id, answer, state, status string }{
			{"failed", "", "failed", "error"}, {"zero", "No exact match.", "no_match_for_filter", "completed"},
			{"clarify", "Which source?", "invalid_request", "needs_input"}, {"empty", "", "", "error"},
		} {
			record := &AgentAnalysisRecord{TenantID: "test", UserID: "user", AgentName: "agent", CaseID: "case", CollectionID: "case", MessageID: tc.id, QueryText: "Synthetic regression"}
			_, err := store.CreateAnalysis(record)
			Expect(err).NotTo(HaveOccurred())
			metadata := map[string]any{"presentation": map[string]any{"result_state": tc.state}}
			Expect(store.CompleteAnalysis("user", "agent", tc.id, tc.answer, metadata)).To(Succeed())
			Expect(store.UpdateAnalysisStatus("user", "agent", "case", tc.id, "completed")).To(Succeed())
			Expect(store.UpdateAnalysisStatus("user", "agent", "case", tc.id, "processing")).To(Succeed())
			reopened, err := store.GetAnalysis(scope, record.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(reopened.Status).To(Equal(tc.status), tc.id)
			Expect(reopened.AnswerText).To(Equal(tc.answer))
			scope.CaseID = "other"
			_, err = store.GetAnalysis(scope, record.ID)
			Expect(err).To(HaveOccurred())
			scope.CaseID = "case"
		}
		record := &AgentAnalysisRecord{TenantID: "test", UserID: "user", AgentName: "agent", CaseID: "case", CollectionID: "case", MessageID: "sql-error", QueryText: "Synthetic error"}
		_, err = store.CreateAnalysis(record)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.UpdateAnalysisStatus("user", "agent", "case", "sql-error", "error: internal SQL diagnostic")).To(Succeed())
		Expect(store.UpdateAnalysisStatus("user", "agent", "case", "sql-error", "processing")).To(Succeed())
		Expect(store.UpdateAnalysisStatus("user", "agent", "case", "sql-error", "completed")).To(Succeed())
		Expect(store.CompleteAnalysis("user", "agent", "sql-error", "late answer", nil)).To(Succeed())
		reopened, err := store.GetAnalysis(scope, record.ID)
		Expect(err).NotTo(HaveOccurred())
		Expect(reopened.Status).To(Equal("error"))
		Expect(reopened.AnswerText).To(BeEmpty())
	})
	It("keeps terminal registry failures canonical and sticky", func() {
		registry := &RequestStatusRegistry{}
		for _, status := range []string{"processing", "error: private diagnostic", "completed", "running"} {
			registry.Record("agent", "user", "case", "request", status)
		}
		status, ok := registry.Get("agent", "user", "case", "request")
		Expect(ok).To(BeTrue())
		Expect(status.Status).To(Equal("error"))
	})
	It("shows similarity rank score and source names without identifier columns", func() {
		for _, mode := range []string{"image_similarity", "face_similarity"} {
			row := map[string]any{"rank": 1, "similarity_score": 0.875, "source_file": "candidate.jpg", "query_source_file": "query.jpg", "candidate_evidence_id": "opaque-id", "citation_locator": map[string]any{"x": 1}}
			table := presentationTable(nil, map[string]any{"operation": map[string]any{"submode": mode}, "data_grid": map[string]any{"columns": []any{"candidate_evidence_id"}, "rows": []any{row}}})
			Expect(table["columns"]).To(Equal([]string{"rank", "similarity_score", "source_file", "query_source_file"}))
			Expect(table["priority_columns"]).To(Equal(table["columns"]))
			if mode == "face_similarity" {
				Expect(table["title"]).To(ContainSubstring("not identity"))
			}
		}
	})
})
