package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Tower activity query contract", func() {
	It("queries both CDR and tower-sector rows from the canonical store", func() {
		normalized := strings.Join(strings.Fields(towerActivitySQL), " ")

		Expect(normalized).To(ContainSubstring("FROM forensic.records"))
		Expect(normalized).To(ContainSubstring("record_type IN ('cdr', 'tower_location')"))
		Expect(normalized).To(ContainSubstring("normalized_fields,site_identifier"))
		Expect(normalized).To(ContainSubstring("normalized_fields,cell_site_id"))
		Expect(normalized).To(ContainSubstring("normalized_fields,sector_identifier"))
		Expect(normalized).To(ContainSubstring("source_file"))
		Expect(normalized).To(ContainSubstring("row_number"))
	})
})
