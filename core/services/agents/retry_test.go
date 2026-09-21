package agents

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Chat retry idempotency", func() {
	It("replays the same reservation without creating a second dispatch", func() {
		var registry ChatRetryRegistry
		requested := ChatRetryReservation{
			AgentName: "forensic", UserID: "analyst-a", CaseID: "case-1",
			OriginalMessageID: "original-1", RetryMessageID: "retry-1",
			IdempotencyKeyHash: ChatRetryMessageHash("1234567890abcdef"), MessageHash: ChatRetryMessageHash("query"),
		}
		id := ChatRetryReservationID("forensic", "analyst-a", "case-1", "1234567890abcdef")

		first, created, err := registry.Reserve(id, requested)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeTrue())
		Expect(first.RetryMessageID).To(Equal("retry-1"))

		requested.RetryMessageID = "retry-losing-race"
		second, created, err := registry.Reserve(id, requested)
		Expect(err).NotTo(HaveOccurred())
		Expect(created).To(BeFalse())
		Expect(second).To(Equal(first))
	})

	It("rejects an idempotency key rebound to another payload", func() {
		var registry ChatRetryRegistry
		requested := ChatRetryReservation{
			AgentName: "forensic", UserID: "analyst-a", CaseID: "case-1",
			OriginalMessageID: "original-1", RetryMessageID: "retry-1",
			IdempotencyKeyHash: ChatRetryMessageHash("1234567890abcdef"), MessageHash: ChatRetryMessageHash("query"),
		}
		id := ChatRetryReservationID("forensic", "analyst-a", "case-1", "1234567890abcdef")
		_, _, err := registry.Reserve(id, requested)
		Expect(err).NotTo(HaveOccurred())
		requested.MessageHash = ChatRetryMessageHash("changed query")
		_, created, err := registry.Reserve(id, requested)
		Expect(created).To(BeFalse())
		Expect(err).To(MatchError(ErrRetryPayloadConflict))
	})

	DescribeTable("terminal eligibility",
		func(status string, eligible bool) { Expect(IsRetryEligibleStatus(status)).To(Equal(eligible)) },
		Entry("error", "error", true),
		Entry("detailed error", "error: backend unavailable", true),
		Entry("timeout", "timed_out", true),
		Entry("analyst cancellation", "cancelled", true),
		Entry("active", "processing", false),
		Entry("successful", "completed", false),
	)
})
