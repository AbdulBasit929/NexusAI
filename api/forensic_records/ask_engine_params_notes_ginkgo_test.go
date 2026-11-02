package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// A note about a parameter is written only when the query that was run reads a view the parameter governs (found in the interface on the real data: a question about the length of calls was
// answered with "Co-location window used: 10 minutes").
var _ = Describe("Ask engine parameter notes follow the query that was run", func() {
	var cat *askCatalogue

	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})

	uses := func(views ...string) []askParamUse {
		f := askFrame{Params: map[string]askParamRead{"colocation_minutes": {Found: true, Value: 10}}}
		got, issue := askResolveParams(f, views)
		Expect(issue).To(BeNil())
		return got
	}
	validated := func(sql string) *askValidated {
		v, rej := askValidate(sql, cat, nil)
		Expect(rej).To(BeNil(), sql)
		return v
	}

	It("drops the co-location window when the query reads only the call events", func() {
		all := uses("v_comm_events", "v_colocation_windows")
		Expect(all).NotTo(BeEmpty(), "the shortlist offered the co-location view and read the minutes as its window")
		kept := askParamUsesRead(all, validated("SELECT count(*) AS n FROM v_comm_events WHERE event_type = 'voice' AND duration_s > 600"))
		Expect(askParamNotes(kept)).To(BeEmpty())
	})

	It("keeps it when the query reads the co-location view", func() {
		all := uses("v_comm_events", "v_colocation_windows")
		kept := askParamUsesRead(all, validated("SELECT count(*) AS n FROM v_colocation_windows WHERE a = '923000000101' AND b = '923000000102'"))
		Expect(askParamNotes(kept)).To(ContainElement(ContainSubstring("Co-location window used: 10 minutes (read from your question)")))
	})

	It("leaves the uses alone when there is no query to compare with", func() {
		all := uses("v_colocation_windows")
		Expect(askParamUsesRead(all, nil)).To(Equal(all))
	})
})
