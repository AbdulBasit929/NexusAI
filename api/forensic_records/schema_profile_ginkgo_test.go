package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Schema profile query contract", func() {
	It("treats missing or scalar forensic header metadata as zero headers", func() {
		normalized := strings.Join(strings.Fields(schemaProfileAssetsSQL), " ")

		Expect(normalized).To(ContainSubstring("WHEN jsonb_typeof(headers) = 'array' THEN jsonb_array_length(headers)"))
		Expect(normalized).To(ContainSubstring("ELSE 0"))
	})
})
