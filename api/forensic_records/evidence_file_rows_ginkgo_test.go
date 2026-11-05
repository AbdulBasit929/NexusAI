package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The file's own rows, a page at a time (evidence_file_rows.go).
var _ = Describe("Evidence file rows under the file's own columns", func() {
	It("reads the header list of a CSV, dropping blanks and repeats, and nothing from a spreadsheet's null", func() {
		Expect(fileRowsHeaders(`["Account No", "Txn Date", " ", "Debit", "Debit"]`)).To(Equal([]string{"Account No", "Txn Date", "Debit"}))
		Expect(fileRowsHeaders("null")).To(BeEmpty())
		Expect(fileRowsHeaders("not json")).To(BeEmpty())
	})

	It("names a workbook column from its number", func() {
		Expect(fileRowsLetters(1)).To(Equal("A"))
		Expect(fileRowsLetters(8)).To(Equal("H"))
		Expect(fileRowsLetters(26)).To(Equal("Z"))
		Expect(fileRowsLetters(27)).To(Equal("AA"))
		Expect(fileRowsLetters(53)).To(Equal("BA"))
	})

	It("keeps the file's own column order for a CSV, whatever order the database kept the keys in", func() {
		stored := []fileRowStored{{RowNumber: 2, RecordType: "generic",
			Payload: map[string]any{"Debit": "67,087.78", "Credit": "", "Narration": "IBFT outward transfer", "Account No": "PK00SYNT0000000000002004"},
			Order:   []string{"Debit", "Credit", "Narration", "Account No"}}}
		columns, rows := fileRowsTable(stored, []string{"Account No", "Narration", "Debit", "Credit"})
		Expect(columns).To(Equal([]fileRowsColumn{{Name: "Account No"}, {Name: "Narration"}, {Name: "Debit"}, {Name: "Credit"}}))
		Expect(rows).To(Equal([]fileRowsRow{{RowNumber: 2, Cells: []string{"PK00SYNT0000000000002004", "IBFT outward transfer", "67,087.78", ""}}}))
	})

	It("orders a spreadsheet by its columns, gives the column letters, and hides the markers the ingest added", func() {
		payload := map[string]any{
			"IMEI": "000000000010009", "Sr #": "5", "A-Party": "923000001000",
			"IMEI__xlsx_raw_value": "", "_xlsx_row_number": float64(93), "_xlsx_sheet_name": "Sheet1",
			"_xlsx_cells": map[string]any{"IMEI": map[string]any{"reference": "H93"}, "Sr #": map[string]any{"reference": "A93"}, "A-Party": map[string]any{"reference": "B93"}},
		}
		stored := []fileRowStored{{RowNumber: 93, RecordType: "generic", Payload: payload, Order: []string{"IMEI", "Sr #", "A-Party", "IMEI__xlsx_raw_value", "_xlsx_cells", "_xlsx_row_number"}}}
		columns, rows := fileRowsTable(stored, nil)
		Expect(columns).To(Equal([]fileRowsColumn{{Name: "Sr #", Letter: "A"}, {Name: "A-Party", Letter: "B"}, {Name: "IMEI", Letter: "H"}}))
		Expect(rows).To(Equal([]fileRowsRow{{RowNumber: 93, Cells: []string{"5", "923000001000", "000000000010009"}}}))
	})

	It("marks a cell the subscriber projection removed, shows a national identity number only masked, and never returns a name", func() {
		stored := []fileRowStored{{RowNumber: 2, RecordType: "subscriber",
			Payload:  map[string]any{"Name": "A Real Name", "CNIC": "3520212345671", "MSISDN": "923001234567"},
			Metadata: map[string]any{"normalized_fields": map[string]any{"cnic_masked": "35202-*******-1"}},
			Order:    []string{"CNIC", "Name", "MSISDN"}}}
		columns, rows := fileRowsTable(stored, []string{"Name", "CNIC", "MSISDN"})
		Expect(columns).To(HaveLen(3))
		Expect(rows[0].Cells).To(Equal([]string{fileRowsWithheld, "35202-*******-1", "923001234567"}))
		for _, cell := range rows[0].Cells {
			Expect(cell).NotTo(ContainSubstring("A Real Name"))
			Expect(cell).NotTo(ContainSubstring("3520212345671"))
		}
	})

	It("keeps a column the header list does not have, after the listed ones", func() {
		stored := []fileRowStored{{RowNumber: 2, Payload: map[string]any{"A": "1", "Extra": "x"}, Order: []string{"A", "Extra"}}}
		columns, _ := fileRowsTable(stored, []string{"A"})
		Expect(columns).To(Equal([]fileRowsColumn{{Name: "A"}, {Name: "Extra"}}))
	})

	// Found by the live check of round 9d on the real Orion files (2026-11-03): the IPDR CSV's first column was stored with the byte-order mark in its name and its cells showed blank.
	It("takes the byte-order mark off the first heading, so that the cells of that column are found", func() {
		bom := string(rune(0xFEFF))
		headers := fileRowsHeaders(`["` + bom + `UFDR start time", "Msisdn"]`)
		Expect(headers).To(Equal([]string{"UFDR start time", "Msisdn"}))
		stored := []fileRowStored{{RowNumber: 2, RecordType: "generic", Payload: map[string]any{"UFDR start time": "2026-03-01 10:00:00", "Msisdn": "923000001000"}, Order: []string{"Msisdn", "UFDR start time"}}}
		columns, rows := fileRowsTable(stored, headers)
		Expect(columns).To(Equal([]fileRowsColumn{{Name: "UFDR start time"}, {Name: "Msisdn"}}))
		Expect(rows[0].Cells).To(Equal([]string{"2026-03-01 10:00:00", "923000001000"}))
		Expect(fileRowsCleanNames([]string{bom + bom + " A "})).To(Equal([]string{"A"}))
	})

	// Found by the same check: a workbook in the old XML format and a text export are laid out from the typed tables, whose cells sit in a JSON object with no order, so the columns came in the
	// order of the database's keys (app, sid, url, host, imei ...) and not the file's.
	It("lays a typed workbook or text export out under the header list the typed ingest kept, in the file's order", func() {
		typed := `[{"sheet":"Sheet1","table":"comm_event","headers":["Sr #","A-Party","B-Party","IMEI"]}]`
		Expect(fileRowsTypedHeaders(typed)).To(Equal([]string{"Sr #", "A-Party", "B-Party", "IMEI"}))
		Expect(fileRowsTypedHeaders(`[{"headers":["a","b"]},{"headers":["b","c"]}]`)).To(Equal([]string{"a", "b", "c"}))
		Expect(fileRowsTypedHeaders("null")).To(BeEmpty())
		Expect(fileRowsTypedHeaders("not json")).To(BeEmpty())
		stored := []fileRowStored{{RowNumber: 1, Payload: map[string]any{"B-Party": "923000001002", "A-Party": "923000001001", "IMEI": "000000000010001", "Sr #": "1"}, Order: []string{"B-Party", "IMEI", "Sr #", "A-Party"}}}
		columns, rows := fileRowsTable(stored, fileRowsTypedHeaders(typed))
		Expect(columns).To(Equal([]fileRowsColumn{{Name: "Sr #"}, {Name: "A-Party"}, {Name: "B-Party"}, {Name: "IMEI"}}))
		Expect(rows[0].Cells).To(Equal([]string{"1", "923000001001", "923000001002", "000000000010001"}))
		// without the list (evidence ingested before it was kept) the order is the database's, as before
		columns, _ = fileRowsTable(stored, nil)
		Expect(columns[0].Name).To(Equal("B-Party"))
	})
})
