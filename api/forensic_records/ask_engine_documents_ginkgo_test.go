package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Documents (reference cases programme, stage S3b): the mention index says which documents NAME a key and nothing more; a key of the question is an identifier the query must carry; a national ID
// number is identity data that is counted and withheld, so a question that names one is not run; the generic words of ordinary questions never bring the document views in (the coverage guard
// declines a whole question when a shortlisted view is uncovered); and the coverage guard tells a case with no documents from one whose documents were not indexed.
var _ = Describe("Ask engine documents", func() {
	const (
		number  = "923000000101"
		account = "PK00SYNT0000000000001001"
	)
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})
	check := func(question, sql string, views ...string) []askObligation {
		v, rej := askValidate(sql, cat, views)
		Expect(rej).To(BeNil(), sql)
		return askCheckFrame(askExtractFrame(question), v)
	}
	kinds := func(list []askObligation) []string {
		out := []string{}
		for _, o := range list {
			out = append(out, o.Kind)
		}
		return out
	}

	DescribeTable("reads a national ID number from the question and keeps it out of the identifiers",
		func(question string, want ...string) {
			f := askExtractFrame(question)
			if len(want) == 0 {
				Expect(f.NationalIDs).To(BeEmpty(), "%s", question)
				return
			}
			Expect(f.NationalIDs).To(Equal(want), "%s", question)
			for _, id := range want {
				Expect(f.Identifiers()).NotTo(ContainElement(id), "%s", question)
				Expect(f.Numbers).NotTo(ContainElement(strings.ReplaceAll(id, "-", "")), "%s", question)
			}
		},
		Entry("dashed", "Which documents mention the national ID number 00000-1234567-1?", "00000-1234567-1"),
		Entry("run together beside the words CNIC", "Which documents mention CNIC 0000012345671?", "00000-1234567-1"),
		Entry("beside the words national id", "Is the national id 0000012345671 in any document?", "00000-1234567-1"),
		Entry("a thirteen-digit number with no cue is not read as one", "How many calls were there on 0000012345671?"),
		Entry("a mobile number is not one", "Which documents mention the number 923000000101?"))

	It("has a plain answer for a question that names a national ID number", func() {
		Expect(askNationalIDAnswer).To(ContainSubstring("withheld"))
		Expect(askNationalIDAnswer).To(ContainSubstring("masking design"))
		Expect(askNationalIDAnswer).NotTo(MatchRegexp(`\d{5}-\d{7}-\d`))
	})

	It("requires the key of the question in the query, in the form the views compare", func() {
		q := "Which documents mention the number " + number + "?"
		good := "SELECT DISTINCT document FROM v_document_values WHERE kind = 'msisdn' AND value = '" + number + "' ORDER BY document"
		Expect(check(q, good, "v_document_values")).To(BeEmpty())
		unnamed := "SELECT DISTINCT document FROM v_document_values WHERE kind = 'msisdn' ORDER BY document"
		Expect(kinds(check(q, unnamed, "v_document_values"))).To(ContainElement("IDENTIFIER"))
		other := "SELECT DISTINCT document FROM v_document_values WHERE kind = 'msisdn' AND value = '923000000999' ORDER BY document"
		Expect(kinds(check(q, other, "v_document_values"))).To(ContainElement("FOREIGN_IDENTIFIER"))
		aq := "Which documents mention the account " + account + "?"
		Expect(check(aq, "SELECT DISTINCT document FROM v_document_values WHERE kind = 'account' AND value = '"+account+"' ORDER BY document", "v_document_values")).To(BeEmpty())
	})

	It("does not ask for a direction on a question about documents", func() {
		q := "Which documents mention the number " + number + "?"
		Expect(kinds(check(q, "SELECT DISTINCT document FROM v_document_values WHERE kind = 'msisdn' AND value = '"+number+"' ORDER BY document", "v_document_values"))).NotTo(ContainElement("DIRECTION"))
	})

	DescribeTable("brings the document views in for a question about what documents mention, and not for the ordinary words of other questions",
		func(question string, want ...string) {
			shortlist := cat.Shortlist(question, 0).Views
			if len(want) == 0 {
				for _, view := range askDocumentViews {
					Expect(shortlist).NotTo(ContainElement(view), "%s", question)
				}
				return
			}
			for _, view := range want {
				Expect(shortlist).To(ContainElement(view), "%s", question)
			}
		},
		Entry("which documents mention a number", "Which documents mention the number "+number+"?", "v_document_values"),
		Entry("which documents mention a plate", "Which documents mention the plate SYNT-4024?", "v_document_values"),
		Entry("numbers written in the documents that appear in the call records", "Which numbers written in the documents also appear in the call records?", "v_document_links"),
		Entry("a place named in documents", "How many documents mention Model Town?", "v_document_values"),
		Entry("how many documents are in the case stays with the inventory", "How many documents are in this case?"),
		Entry("a call question", "How many calls did "+number+" make in March?"),
		Entry("a statement question", "What was the largest debit of the account "+account+"?"),
		Entry("a registration question", "Which SIM was "+number+" registered on?"),
		Entry("a plate question about cameras", "At which cameras was the plate SYNT-4024 seen?"))

	DescribeTable("shortlists the document views for a question about documents that names a key, in the phrasings the first run was answered wrongly for",
		func(question string) {
			shortlist := cat.Shortlist(question, 0).Views
			Expect(shortlist).To(ContainElement("v_document_values"), "%s", question)
			Expect(shortlist).NotTo(ContainElement("v_evidence_inventory"), "%s", question)
		},
		Entry("in which documents does an account appear", "In which documents does PK00SYNT0000000000002000 appear?"),
		Entry("list the documents that name an account written in groups", "List the documents that name PK00 SYNT 0000 0000 0000 2001."),
		Entry("list the documents that name a plate in lower case", "List the documents that name synt 8041."),
		Entry("in how many documents does a number appear", "In how many documents does +92 300 0001006 appear?"),
		Entry("in which documents does a number written 03... appear", "In which documents does 03000001056 appear?"),
		Entry("documents that name a number and also a plate", "List the documents that name +92 300 0001000 and also SYNT-8041."),
		Entry("an account and a plate together", "In which documents do PK00SYNT0000000000002001 and synt 3083 appear together?"),
		Entry("the numbers in the documents whose own call records are in the case", "List the numbers in the documents whose own call records are in the case."),
		Entry("an IMEI named in documents", "List the documents that name 000000000010090."))

	It("compares a value with the kind its form says: a mobile number with msisdn, an account with account, a plate with plate, a SIM serial with iccid", func() {
		q := "Which documents mention the number " + number + "?"
		Expect(kinds(check(q, "SELECT DISTINCT document FROM v_document_values WHERE kind = 'plate' AND value = '"+number+"'", "v_document_values"))).To(ContainElement("KIND_VALUE"))
		Expect(kinds(check(q, "SELECT DISTINCT document FROM v_document_values WHERE value = '"+number+"' AND kind = 'account'", "v_document_values"))).To(ContainElement("KIND_VALUE"))
		Expect(kinds(check(q, "SELECT DISTINCT document FROM v_document_values WHERE kind = 'msisdn' AND value = '"+number+"'", "v_document_values"))).NotTo(ContainElement("KIND_VALUE"))
		pq := "Which documents mention the plate SYNT-4024?"
		Expect(kinds(check(pq, "SELECT DISTINCT document FROM v_document_values WHERE kind = 'plate' AND value = 'SYNT4024'", "v_document_values"))).NotTo(ContainElement("KIND_VALUE"))
	})

	It("keeps the inventory for a question about the files themselves, and refuses to look for an identifier among file names", func() {
		Expect(cat.Shortlist("How many documents are in this case?", 0).Views).To(ContainElement("v_evidence_inventory"))
		Expect(cat.Shortlist("How many documents are in this case?", 0).Views).NotTo(ContainElement("v_document_values"))
		Expect(cat.Shortlist("How many PDF files were uploaded?", 0).Views).NotTo(ContainElement("v_document_values"))
		q := "In which documents does " + account + " appear?"
		bad := "SELECT original_filename FROM v_evidence_inventory WHERE original_filename LIKE '%" + account + "%'"
		Expect(kinds(check(q, bad, "v_evidence_inventory"))).To(ContainElement("INVENTORY_IDENTIFIER"))
		Expect(kinds(check("When was the last document uploaded?", "SELECT max(created_at) AS latest FROM v_evidence_inventory WHERE modality = 'document'", "v_evidence_inventory"))).NotTo(ContainElement("INVENTORY_IDENTIFIER"))
	})

	It("tells a case with no documents from a case whose documents were not indexed, and lets an indexed case through", func() {
		for _, view := range askDocumentViews {
			Expect(askCoverageGap(askCoverage{AnyComm: true}, []string{view})).To(ContainSubstring("holds no documents"), view)
			Expect(askCoverageGap(askCoverage{AnyComm: true, Documents: 3}, []string{view})).To(ContainSubstring("have not been indexed"), view)
			Expect(askCoverageGap(askCoverage{AnyDocument: true, Documents: 3}, []string{view})).To(Equal(""), view)
		}
		// the document views do not need the typed call records
		Expect(askCoverageGap(askCoverage{AnyDocument: true, Files: 4, Typed: 1}, []string{"v_document_values"})).To(Equal(""))
	})

	It("carries a caveat on every document view that says what a mention is and is not", func() {
		for _, view := range askDocumentViews {
			text := askViewCaveatText[view]
			Expect(text).To(ContainSubstring("a document names the key"), view)
			Expect(text).To(ContainSubstring("does not say who holds the key"), view)
			Expect(text).To(ContainSubstring("masking design"), view)
			Expect(strings.ToLower(text)).NotTo(ContainSubstring("owner"), view)
		}
		Expect(askViewCaveatText["v_document_links"]).To(ContainSubstring("a lead, not a finding"))
	})

	It("says what was searched when a document view returns no row, and corrects a question that treats a mention as proof", func() {
		v, rej := askValidate("SELECT DISTINCT document FROM v_document_values WHERE kind = 'msisdn' AND value = '"+number+"' ORDER BY document", cat, []string{"v_document_values"})
		Expect(rej).To(BeNil())
		f := askExtractFrame("Which documents mention the number " + number + "?")
		Expect(askDocumentAbsence(v, f)).To(Equal("no document of this case names " + number))
		two := askExtractFrame("Which documents mention both " + number + " and " + account + "?")
		Expect(askDocumentAbsence(v, two)).To(ContainSubstring(" and "))
		links, rej := askValidate("SELECT DISTINCT value FROM v_document_links WHERE kind = 'msisdn' AND family = 'calls'", cat, []string{"v_document_links"})
		Expect(rej).To(BeNil())
		Expect(askDocumentAbsence(links, askExtractFrame("Which numbers written in the documents also appear in the call records?"))).To(ContainSubstring("found in the records of the case"))
		Expect(askDocumentLead(askExtractFrame("The document names "+number+": does that prove its owner is the accused?"), false)).To(ContainSubstring("does not say who holds the number"))
		Expect(askDocumentLead(askExtractFrame("Which documents mention "+number+"?"), false)).To(Equal(""))
		Expect(askReadsDocumentView(v)).To(BeTrue())
		Expect(askFamilyViews(v)).To(Equal(askDocumentViews))
	})
})
