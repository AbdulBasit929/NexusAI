package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("R6.6 query quality", func() {
	It("requires an explicit participant for frequent contacts", func() {
		Expect(needsClarification("who are the frequent contacts?", "frequent_contacts", "")).To(BeTrue())
		Expect(clarificationQuestion("who are the frequent contacts?", "frequent_contacts")).To(Equal("Which number or subscriber should I analyze for frequent contacts?"))
		Expect(needsClarification("who does 923001110001 contact most?", "frequent_contacts", "923001110001")).To(BeFalse())
	})

	It("publishes the frequent-contact target as required", func() {
		guidance := queryTemplateGuidanceFor("frequent_contacts")
		Expect(guidance.TargetRequired).To(BeTrue())
		Expect(guidance.OperationID).To(Equal("cdr.frequent_contacts"))
	})

	It("excludes service labels and the selected participant from contact ranking", func() {
		Expect(frequentContactsSQL).To(ContainSubstring("$3 <> ''"))
		Expect(frequentContactsSQL).To(ContainSubstring("~ '^[0-9]{8,19}$'"))
		Expect(frequentContactsSQL).To(ContainSubstring("<> regexp_replace($3"))
		Expect(strings.ToUpper(frequentContactsSQL)).ToNot(ContainSubstring("WHEN $3 = '' THEN"))
		Expect(frequentContactsSQL).To(ContainSubstring("$6 = '' OR direction = $6"))
	})

	It("reuses bounded context only for explicit follow-up language", func() {
		context := queryConversationContext{Target: "923001110001", Template: "frequent_contacts", Direction: "OUTGOING"}
		followUp := hybridQueryRequest{Query: "Only outgoing.", Direction: extractEventDirection("Only outgoing."), ConversationContext: context}
		resolved, plan := applyConversationContext(followUp, planRuntimeQuery(followUp))
		Expect(resolved.Target).To(Equal("923001110001"))
		Expect(resolved.Template).To(Equal("frequent_contacts"))
		Expect(resolved.Direction).To(Equal("OUTGOING"))
		Expect(plan.Template).To(Equal("frequent_contacts"))
		Expect(plan.QueryPlan.AppliedFilters).To(HaveKeyWithValue("conversation_context", true))

		unrelated := hybridQueryRequest{Query: "who are the frequent contacts?", ConversationContext: context}
		unresolved, _ := applyConversationContext(unrelated, planRuntimeQuery(unrelated))
		Expect(unresolved.Target).To(BeEmpty())
	})

	It("uses analyst-facing executive answers for inventory and readiness", func() {
		Expect(chooseTemplate("is this case ready for analysis?", "")).To(Equal("case_readiness"))

		inventory := enterpriseExecutiveAnswer(
			hybridQueryRequest{},
			hybridQueryResponse{Template: "source_file_audit", Answer: map[string]any{"records_row_count": 10}},
			nil,
		)
		Expect(inventory).To(Equal("10 source files are registered in this case."))

		readiness := enterpriseExecutiveAnswer(
			hybridQueryRequest{},
			hybridQueryResponse{Template: "case_readiness", Answer: map[string]any{"records_row_count": 3}},
			nil,
		)
		Expect(readiness).To(ContainSubstring("ready for analysis"))
		Expect(readiness).To(ContainSubstring("not a certification"))
	})
})
