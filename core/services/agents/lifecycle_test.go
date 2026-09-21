package agents

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("request lifecycle", func() {
	It("carries the dispatch deadline into the execution context", func() {
		before := time.Now().Add(DefaultAgentChatTimeout).UnixMilli()
		deadlineUnixMilli := AgentChatDeadlineUnixMilli()
		after := time.Now().Add(DefaultAgentChatTimeout).UnixMilli()
		Expect(deadlineUnixMilli).To(BeNumerically(">=", before))
		Expect(deadlineUnixMilli).To(BeNumerically("<=", after))

		ctx, cancel := WithAgentChatDeadline(context.Background(), deadlineUnixMilli)
		defer cancel()
		deadline, ok := ctx.Deadline()
		Expect(ok).To(BeTrue())
		Expect(deadline.UnixMilli()).To(Equal(deadlineUnixMilli))
	})

	It("isolates recent request status by user, agent, case, and request", func() {
		var registry RequestStatusRegistry
		registry.Record("agent", "user-a", "case-alpha", "request-1", "processing")
		registry.Record("agent", "user-a", "case-alpha", "request-1", "completed")

		status, ok := registry.Get("agent", "user-a", "case-alpha", "request-1")
		Expect(ok).To(BeTrue())
		Expect(status.Status).To(Equal("completed"))
		Expect(status.UpdatedAt).ToNot(BeZero())
		_, ok = registry.Get("agent", "user-b", "case-alpha", "request-1")
		Expect(ok).To(BeFalse())
		_, ok = registry.Get("agent", "user-a", "case-beta", "request-1")
		Expect(ok).To(BeFalse())
		_, ok = registry.Get("other-agent", "user-a", "case-alpha", "request-1")
		Expect(ok).To(BeFalse())
	})

	DescribeTable("classifies terminal execution errors",
		func(err error, expected string) {
			Expect(ExecutionTerminalStatus(err)).To(Equal(expected))
		},
		Entry("deadline", context.DeadlineExceeded, "timed_out"),
		Entry("wrapped deadline", fmt.Errorf("forensic request: %w", context.DeadlineExceeded), "timed_out"),
		Entry("analyst cancellation", context.Canceled, "cancelled"),
		Entry("ordinary failure", errors.New("backend unavailable"), "error"),
	)

	It("leaves context terminal publication to the owning execution path", func() {
		statuses := []string{}
		callbacks := Callbacks{OnStatus: func(status string) { statuses = append(statuses, status) }}

		_, err := finishDeterministicForensicRoute(nil, forensicDirectResult{}, context.DeadlineExceeded, callbacks, nil)
		Expect(err).To(MatchError(ContainSubstring("context deadline exceeded")))
		_, err = finishDeterministicForensicRoute(nil, forensicDirectResult{}, context.Canceled, callbacks, nil)
		Expect(err).To(MatchError(ContainSubstring("context canceled")))
		Expect(statuses).To(BeEmpty())

		_, err = finishDeterministicForensicRoute(nil, forensicDirectResult{}, errors.New("backend unavailable"), callbacks, nil)
		Expect(err).To(MatchError(ContainSubstring("backend unavailable")))
		Expect(statuses).To(Equal([]string{"error"}))
	})
})
