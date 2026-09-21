package agents

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("General explanation routing", func() {
	It("does not turn a forensic noun in a concept question into case execution", func() {
		for _, q := range []string{"Explain how cell site observations differ from precise handset location.", "Define a call type breakdown.", "What does an equipment identifier represent?", "Why can a similarity score require human review?", "How does a relationship network represent connections?"} {
			Expect(deterministicForensicRouteRequested(q)).To(BeFalse(), q)
		}
	})
	It("preserves explicit tool and supplied-target case requests", func() {
		Expect(deterministicForensicRouteRequested("forensic_hybrid_query template=frequent_contacts target=923146208975")).To(BeTrue())
		Expect(deterministicForensicRouteRequested("Show call type breakdown in this case")).To(BeTrue())
	})
})
