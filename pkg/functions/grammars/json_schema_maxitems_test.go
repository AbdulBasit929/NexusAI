package grammars_test

import (
	"strings"

	"github.com/mudler/LocalAI/pkg/functions/grammars"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// maxItems was parsed and then ignored, so every array compiled to unbounded
// repetition. A grammar-constrained model fills what the grammar allows: a
// bounded analytical plan ran to 1791 bytes where a complete one is ~292, and
// never terminated. The token budget was blamed; the grammar was the cause.

func arrayRule(schema string) string {
	got, err := grammars.NewJSONSchemaConverter("").GrammarFromBytes([]byte(schema))
	Expect(err).ToNot(HaveOccurred())
	rule := ""
	for _, line := range strings.Split(got, "\n") {
		if strings.HasPrefix(line, "root-xs ::=") {
			rule = line
		}
	}
	Expect(rule).ToNot(BeEmpty(), "no rule generated for the array:\n%s", got)
	return rule
}

var _ = Describe("JSON schema maxItems", func() {
	It("bounds the grammar of an array with maxItems", func() {
		// Scoped to the ARRAY's own rule: the generic string rule legitimately uses
		// unbounded repetition for its characters.
		rule := arrayRule(`{"type":"object","properties":{"xs":{"type":"array","items":{"type":"string"},"maxItems":2}},"required":["xs"]}`)
		GinkgoWriter.Printf("bounded: %s\n", rule)
		Expect(rule).ToNot(ContainSubstring(`)*`), "array still compiles to unbounded repetition")
		Expect(strings.Count(rule, "?")).To(BeNumerically(">=", 2), "expected nested optionals bounding the array at 2 items")
	})

	It("keeps unbounded repetition for arrays without maxItems", func() {
		rule := arrayRule(`{"type":"object","properties":{"xs":{"type":"array","items":{"type":"string"}}},"required":["xs"]}`)
		Expect(rule).To(ContainSubstring(`)*`), "an unconstrained array must keep unbounded repetition")
	})

	It("admits no item when maxItems is 0", func() {
		rule := arrayRule(`{"type":"object","properties":{"xs":{"type":"array","items":{"type":"string"},"maxItems":0}},"required":["xs"]}`)
		Expect(rule).ToNot(ContainSubstring("xs-item"), "maxItems 0 must not admit an item")
	})
})
