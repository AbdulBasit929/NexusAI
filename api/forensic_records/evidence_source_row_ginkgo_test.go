package main

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The cited row under the file's own column names (evidence_source_row.go).
var _ = Describe("Evidence source row under the file's own headings", func() {
	It("keeps the order the cells were stored in, and drops empty cells and internal markers", func() {
		payload := `{"MSISDN": "923000001001", "STRT_TM": "02/02/2026 05:23:53", "BNUMBER": "03000001077", "MINS": 0, "SECS": "32", "EMPTY": "", "_xlsx_row_number": 5, "FLAG": true}`
		Expect(sourceRowOrder(payload)).To(Equal([]string{"MSISDN", "STRT_TM", "BNUMBER", "MINS", "SECS", "EMPTY", "_xlsx_row_number", "FLAG"}))
		cells, err := sourceRowCells(payload)
		Expect(err).NotTo(HaveOccurred())
		Expect(cells).To(Equal([]sourceRowCell{
			{Name: "MSISDN", Value: "923000001001"}, {Name: "STRT_TM", Value: "02/02/2026 05:23:53"}, {Name: "BNUMBER", Value: "03000001077"},
			{Name: "MINS", Value: "0"}, {Name: "SECS", Value: "32"}, {Name: "FLAG", Value: "true"},
		}))
	})

	It("refuses text that is not an object, never guessing a row from it", func() {
		_, err := sourceRowCells("not json")
		Expect(err).To(HaveOccurred())
		Expect(sourceRowOrder("[1,2]")).To(BeEmpty())
	})

	It("reads the workbook address of each cell, and the column number from it", func() {
		addresses := sourceRowAddresses(`{"IMEI": {"value": "1", "reference": "H93"}, "Duration": {"reference": "AA93"}, "Odd": {"reference": "93"}, "Bad": "text"}`)
		Expect(addresses).To(Equal(map[string]string{"IMEI": "H93", "Duration": "AA93"}))
		Expect(sourceRowColumn("A1")).To(Equal(1))
		Expect(sourceRowColumn("H93")).To(Equal(8))
		Expect(sourceRowColumn("AA7")).To(Equal(27))
		for _, notAnAddress := range []string{"", "93", "H", "h93", "H9X"} {
			Expect(sourceRowColumn(notAnAddress)).To(BeZero(), notAnAddress)
		}
		Expect(sourceRowAddresses("not json")).To(BeNil())
	})

	It("orders the cells by the columns of the workbook when every cell has an address", func() {
		cells := []sourceRowCell{{Name: "IMEI", Value: "1"}, {Name: "Sr #", Value: "5"}, {Name: "A-Party", Value: "923000001000"}}
		addresses := map[string]string{"IMEI": "H93", "Sr #": "A93", "A-Party": "B93"}
		ordered := sourceRowInFileOrder(context.Background(), nil, "t", "e", 93, "{}", cells, addresses)
		Expect(ordered).To(Equal([]sourceRowCell{{Name: "Sr #", Value: "5", Cell: "A93"}, {Name: "A-Party", Value: "923000001000", Cell: "B93"}, {Name: "IMEI", Value: "1", Cell: "H93"}}))
	})

	It("reads a number without a trailing zero and a nested value as text", func() {
		cells, err := sourceRowCells(`{"LAT": 31.4807, "LNG": 74.30455, "EXTRA": {"a": 1}}`)
		Expect(err).NotTo(HaveOccurred())
		Expect(cells).To(Equal([]sourceRowCell{{Name: "LAT", Value: "31.4807"}, {Name: "LNG", Value: "74.30455"}, {Name: "EXTRA", Value: `{"a":1}`}}))
	})
})
