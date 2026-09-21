package agents

import (
	"testing"
	"time"

	"github.com/mudler/LocalAI/core/services/testutil"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestCanonicalAnalysisStatusBoundsTransportErrors(t *testing.T) {
	if got := canonicalAnalysisStatus("error: records API failed with a very long diagnostic"); got != "error" {
		t.Fatalf("canonical status = %q, want error", got)
	}
	if got := canonicalAnalysisStatus(" completed "); got != "completed" {
		t.Fatalf("canonical status = %q, want completed", got)
	}
}

var _ = Describe("Analysis history contract", func() {
	It("round-trips an opaque stable cursor", func() {
		record := AgentAnalysisRecord{ID: "analysis-1", CreatedAt: time.Date(2026, 8, 10, 8, 0, 0, 123, time.UTC)}
		encoded := encodeAnalysisCursor(record)
		Expect(encoded).NotTo(ContainSubstring("analysis-1"))
		decoded, err := decodeAnalysisCursor(encoded)
		Expect(err).NotTo(HaveOccurred())
		Expect(decoded.ID).To(Equal(record.ID))
		Expect(decoded.CreatedAt).To(Equal(record.CreatedAt))
	})

	It("rejects malformed cursors", func() {
		_, err := decodeAnalysisCursor("not-a-valid-cursor")
		Expect(err).To(MatchError("invalid history cursor"))
	})

	It("derives deterministic browser import identities inside case scope", func() {
		first := BrowserImportMessageID("analyst", "agent", "case-a", "conversation-1", 2, "same query")
		Expect(BrowserImportMessageID("analyst", "agent", "case-a", "conversation-1", 2, "same query")).To(Equal(first))
		Expect(BrowserImportMessageID("analyst", "agent", "case-b", "conversation-1", 2, "same query")).NotTo(Equal(first))
		Expect(first).To(HavePrefix("browser-"))
	})

	It("preserves public row count and result state through retained History reopen", func() {
		store, err := NewAgentStore(testutil.SetupTestDB())
		Expect(err).NotTo(HaveOccurred())
		record := &AgentAnalysisRecord{
			TenantID: "tenant-test", UserID: "analyst-test", AgentName: "forensic-agent",
			CaseID: "case-test", CollectionID: "case-test", MessageID: "message-row-count",
			QueryText: "synthetic complete-zero query",
		}
		created, err := store.CreateAnalysis(record)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeTrue())
		metadata := map[string]any{"presentation": map[string]any{
			"contract_version": forensicPresentationContract,
			"result_state":     "complete_zero_results",
			"processing_state": "completed",
			"row_count":        0,
		}}
		Expect(store.CompleteAnalysis("analyst-test", "forensic-agent", "message-row-count", "Complete with zero results.", metadata)).To(Succeed())
		scope := AnalysisHistoryScope{TenantID: "tenant-test", UserID: "analyst-test", AgentName: "forensic-agent", CaseID: "case-test", CollectionID: "case-test"}
		reopened, err := store.GetAnalysis(scope, record.ID)
		Expect(err).NotTo(HaveOccurred())
		presentation := reopened.Metadata["presentation"].(map[string]any)
		Expect(presentation).To(HaveKeyWithValue("result_state", "complete_zero_results"))
		Expect(presentation).To(HaveKeyWithValue("row_count", float64(0)))
	})

	DescribeTable("terminal status classification",
		func(status string, terminal bool) { Expect(IsAnalysisTerminalStatus(status)).To(Equal(terminal)) },
		Entry("completed", "completed", true),
		Entry("timeout", "timed_out", true),
		Entry("cancelled", "cancelled", true),
		Entry("detailed error", "error: unavailable", true),
		Entry("processing", "processing", false),
	)
})
