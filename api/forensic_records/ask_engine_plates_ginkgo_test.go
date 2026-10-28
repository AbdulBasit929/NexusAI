package main

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Plates as identifiers of the question (docs/data/anpr-typed-family-20261026.md, piece f): the Go normaliser equals the Python one and the SQL function on one fixture; a plate is found in a
// question only where one is written; the query must compare it as its search key, and must name no other.
var _ = Describe("Ask engine plates", func() {
	It("normalises a plate like forensic.normalize_plate and the Python port: the same keys on the shared fixture", func() {
		raw, err := os.ReadFile("../../ingestion/forensic_records/tests/plate_tokens.json")
		Expect(err).NotTo(HaveOccurred())
		var keyed []struct {
			Token string `json:"token"`
			Key   string `json:"key"`
		}
		Expect(json.Unmarshal(raw, &keyed)).To(Succeed())
		Expect(len(keyed)).To(BeNumerically(">=", 60))
		for _, row := range keyed {
			Expect(askNormalizePlate(row.Token)).To(Equal(row.Key), "token %q", row.Token)
		}
	})

	DescribeTable("finds a plate in a question only where one is written",
		func(question string, want ...string) {
			f := askExtractFrame(question)
			if len(want) == 0 {
				Expect(f.Plates).To(BeEmpty(), question)
				return
			}
			Expect(f.Plates).To(Equal(want), question)
		},
		Entry("capitals with a dash", "How many times was SYNT-0420 seen?", "SYNT0420"),
		Entry("after the word plate, in lower case", "How many times was the plate synt 0420 seen?", "SYNT0420"),
		Entry("lower case with no introducing word is not a plate", "synt 0420 seen where"),
		Entry("two plates", "Which cameras saw LEA-1234 and ABC 123?", "LEA1234", "ABC123"),
		Entry("a series year", "Where was the plate AB 12 3456 seen?", "AB123456"),
		Entry("a look-alike with an O among the digits", "How many times was the plate SYNT-O420 seen?", "SYNTO420"),
		Entry("a look-alike with an I among the digits", "Where was LEA-1I34 seen?", "LEA1I34"),
		Entry("the plate and its look-alike are two plates", "Was SYNT-0420 seen with SYNT-O420?", "SYNT0420", "SYNTO420"),
		Entry("capitals that are not digits are not a plate", "Show the TOP OIO events"),
		Entry("a month and a year are not a plate", "How many calls in MARCH 2026?"),
		Entry("a year after a word is not a plate", "Show the events since 2026"),
		Entry("a year after capitals is not a plate", "ABC 2024 events"),
		Entry("a plate whose number looks like a year, joined by a dash", "List the sightings of SYNT-2020 with the camera and the time of each.", "SYNT2020"),
		Entry("a plate whose number looks like a year, after four capitals", "How often was SYNT 1999 seen?", "SYNT1999"),
		Entry("a word and a year are still not a plate", "Show the events SINCE 2026 and UNTIL 2027"),
		Entry("a year after the word plate may be one", "Where was the plate ABC 2024 seen?", "ABC2024"),
		Entry("a camera is not a plate", "Which plates were seen at camera CAM-123?"),
		Entry("a two-digit camera is not a plate", "Which plates were seen at camera CAM-02 and camera CAM 01?"),
		Entry("a limit is not a plate", "Show the TOP 100 contacts"),
		Entry("a phone number is a number, not a plate", "Did 923001110001 call 923001110002?"),
		Entry("a plate beside a phone number", "Did 923001110001 drive the vehicle LEA-1234?", "LEA1234"),
		Entry("the same plate twice is one", "Was LEA-1234 seen, and was LEA 1234 seen at night?", "LEA1234"))

	It("names the plate in the canonical question and in the identifiers, so that the probe and the audit see it", func() {
		f := askExtractFrame("How many times was the plate SYNT-0420 seen at camera CAM-02?")
		Expect(f.Question).To(ContainSubstring("SYNT0420"))
		Expect(f.Identifiers()).To(ContainElement("SYNT0420"))
		Expect(f.PlateWritten["SYNT0420"]).To(Equal("SYNT-0420"))
		Expect(askUserPrompt(f)).To(ContainSubstring("compare plate_norm with SYNT0420"))
	})

	Describe("the query's duty to the plates of the question", func() {
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

		It("accepts a query that compares the search key", func() {
			Expect(check("How many times was the plate SYNT-0420 seen?", "SELECT count(*) AS n FROM v_plate_sightings WHERE plate_norm = 'SYNT0420'", "v_plate_sightings")).To(BeEmpty())
		})

		It("asks for the search key when the query compares the raw plate, or the plate written with its separator", func() {
			for _, sql := range []string{
				"SELECT count(*) AS n FROM v_plate_sightings WHERE plate_raw = 'SYNT-0420'",
				"SELECT count(*) AS n FROM v_plate_sightings WHERE plate_norm = 'SYNT-0420'",
			} {
				unmet := check("How many times was the plate SYNT-0420 seen?", sql, "v_plate_sightings")
				Expect(kinds(unmet)).To(ContainElement("IDENTIFIER"), sql)
				Expect(unmet[0].Message).To(ContainSubstring("Compare plate_norm with 'SYNT0420'"))
				Expect(unmet[0].Message).To(ContainSubstring("SYNT-0420"))
			}
		})

		It("refuses a plate the question did not name, and a look-alike for the one it did", func() {
			unmet := check("How many times was the plate SYNT-0420 seen?", "SELECT count(*) AS n FROM v_plate_sightings WHERE plate_norm IN ('SYNT0420', 'SYNTO420')", "v_plate_sightings")
			Expect(kinds(unmet)).To(ConsistOf("FOREIGN_IDENTIFIER"))
			Expect(unmet[0].Message).To(ContainSubstring("SYNTO420"))
			unmet = check("How many times was the plate SYNT-0420 seen?", "SELECT count(*) AS n FROM v_plate_sightings WHERE plate_norm = 'LEA1234'", "v_plate_sightings")
			Expect(kinds(unmet)).To(ContainElements("IDENTIFIER", "FOREIGN_IDENTIFIER"))
		})

		It("leaves a question with no plate to the cameras: no plate duty, no plate refused for being named", func() {
			Expect(check("Which plates were seen at both camera CAM-01 and camera CAM-02?",
				"SELECT plate_norm AS label, sum(sightings) AS value FROM v_plate_visits WHERE camera_id IN ('CAM-01', 'CAM-02') GROUP BY plate_norm HAVING count(DISTINCT camera_id) = 2 ORDER BY value DESC LIMIT 200", "v_plate_visits")).To(BeEmpty())
		})
	})

	It("declines the plate views for a case with no typed camera sightings, and asks nothing of the call records for them", func() {
		Expect(askCoverageGap(askCoverage{AnyPlate: false}, []string{"v_plate_sightings"})).To(ContainSubstring("no typed camera sightings"))
		Expect(askCoverageGap(askCoverage{AnyPlate: false}, []string{"v_plate_visits"})).To(ContainSubstring("no typed camera sightings"))
		Expect(askCoverageGap(askCoverage{AnyPlate: true}, []string{"v_plate_visits"})).To(Equal(""))
		Expect(askCoverageGap(askCoverage{AnyPlate: true, Files: 2, Typed: 1}, []string{"v_plate_sightings"})).To(Equal(""))
		Expect(askProbeColumn("plate_norm")).To(BeTrue())
		Expect(askProbeColumn("plate_raw")).To(BeFalse())
	})

	Describe("what the ANPR development check on case 2 found", func() {
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

		It("does not let the sightings of a plate at a camera be counted as the rows of v_plate_visits", func() {
			q := "List the cameras that saw the plate SYNT-1438 with the number of sightings at each."
			bad := check(q, "SELECT camera_id, camera_label, count(*) AS sightings FROM v_plate_visits WHERE plate_norm = 'SYNT1438' GROUP BY camera_id, camera_label ORDER BY sightings DESC", "v_plate_visits")
			Expect(kinds(bad)).To(ContainElement("AGGREGATE"))
			Expect(bad[len(bad)-1].Message).To(ContainSubstring("sightings column"))
			Expect(check(q, "SELECT camera_id, camera_label, sightings FROM v_plate_visits WHERE plate_norm = 'SYNT1438' ORDER BY sightings DESC LIMIT 200", "v_plate_visits")).To(BeEmpty())
			Expect(check(q, "SELECT camera_id, sum(sightings) AS n FROM v_plate_visits WHERE plate_norm = 'SYNT1438' GROUP BY camera_id", "v_plate_visits")).To(BeEmpty())
			// how many cameras saw it is a count of the rows, and stays one
			Expect(kinds(check("How many cameras saw the plate SYNT-1438?", "SELECT count(*) AS n FROM v_plate_visits WHERE plate_norm = 'SYNT1438'", "v_plate_visits"))).NotTo(ContainElement("AGGREGATE"))
			Expect(kinds(check("How many plates were seen at camera CAM-01?", "SELECT count(*) AS n FROM v_plate_visits WHERE camera_id = 'CAM-01'", "v_plate_visits"))).NotTo(ContainElement("AGGREGATE"))
		})

		It("asks for the start of the next day when the question includes its last day, and leaves a bare between alone", func() {
			q := "Count the sightings of the plate SYNT 5854 from 2026-02-10 through 2026-03-13 (local time)."
			stops := "SELECT count(*) AS n FROM v_plate_sightings WHERE plate_norm = 'SYNT5854' AND observed_utc >= '2026-02-10 00:00:00+05' AND observed_utc < '2026-03-13 00:00:00+05'"
			bad := check(q, stops, "v_plate_sightings")
			Expect(kinds(bad)).To(ConsistOf("DATE_END"))
			Expect(bad[0].Message).To(ContainSubstring("< '2026-03-14 00:00:00+05'"))
			covers := "SELECT count(*) AS n FROM v_plate_sightings WHERE plate_norm = 'SYNT5854' AND observed_utc >= '2026-02-10 00:00:00+05' AND observed_utc < '2026-03-14 00:00:00+05'"
			Expect(check(q, covers, "v_plate_sightings")).To(BeEmpty())
			// the last day alone, written as the next day, satisfies the date check too
			only := "SELECT count(*) AS n FROM v_plate_sightings WHERE plate_norm = 'SYNT5854' AND observed_utc >= '2026-02-10 00:00:00+05' AND observed_utc < '2026-03-14 00:00:00+05'"
			Expect(check("How many times was SYNT 5854 seen between 2026-02-10 and 2026-03-13, both days included?", only, "v_plate_sightings")).To(BeEmpty())
			Expect(kinds(check("How many times was SYNT 5854 seen between 2026-02-10 and 2026-03-13, both days included?", stops, "v_plate_sightings"))).To(ConsistOf("DATE_END"))
			// "between D1 and D2" with no word that includes the last day is ambiguous and stays as it was
			Expect(check("How many times was SYNT 5854 seen between 2026-02-10 and 2026-03-13?", stops, "v_plate_sightings")).To(BeEmpty())
			Expect(askInclusiveEnd("connected through 923001110001", nil)).To(Equal(""))
			Expect(askInclusiveEnd("from 2026-02-10 through 2026-03-13", []string{"2026-02-10", "2026-03-13"})).To(Equal("2026-03-13"))
			Expect(askInclusiveEnd("both days included", []string{"2026-02-10", "2026-03-13"})).To(Equal("2026-03-13"))
			Expect(askNextDay("2026-02-28")).To(Equal("2026-03-01"))
		})

		It("reads how often as a count, not a ranking", func() {
			f := askExtractFrame("How often was SYNT-8572 seen?")
			Expect(f.Cues["count"]).To(BeTrue())
			Expect(f.Cues["rank"]).To(BeFalse())
			Expect(askExtractFrame("Who does 923001110001 contact most often?").Cues["rank"]).To(BeTrue())
		})

		It("says a search for plates at several cameras as that, not as a camera that saw nothing", func() {
			f := askExtractFrame("Which plates appear at camera CAM-05 and at camera CAM-10?")
			both, rej := askValidate("SELECT plate_norm AS label, sum(sightings) AS value FROM v_plate_visits WHERE camera_id IN ('CAM-05', 'CAM-10') GROUP BY plate_norm HAVING count(DISTINCT camera_id) = 2 ORDER BY value DESC", cat, []string{"v_plate_visits"})
			Expect(rej).To(BeNil())
			none := &govSQLResult{Columns: []string{"label", "value"}, Texts: []bool{true, false}, Rows: [][]any{}}
			Expect(askWording(askNoRowSentence, none, both, f, "ranked", "")).To(Equal("No row matches what you asked: no plate was seen at all of the cameras CAM-05, CAM-10."))
			any, rej := askValidate("SELECT plate_norm FROM v_plate_visits WHERE camera_id IN ('CAM-05', 'CAM-10')", cat, []string{"v_plate_visits"})
			Expect(rej).To(BeNil())
			Expect(askWording(askNoRowSentence, none, any, f, "table", "")).To(Equal("No row matches what you asked: no sighting at cameras CAM-05, CAM-10 was found."))
		})
	})

	It("says a plate that occurs nowhere as a plate, and leaves the sentence for numbers alone", func() {
		f := askExtractFrame("Was the plate SYNT-0420 seen?")
		Expect(askAbsenceOf([]string{"SYNT0420"}, f)).To(Equal("No record of this case holds the plate SYNT-0420 in any camera sighting."))
		Expect(askAbsenceOf([]string{"923000000101"}, askFrame{})).To(Equal("No record of this case holds 923000000101 in any number, device, SIM, address or cell column."))
		both := askExtractFrame("Did 923000000101 drive the plate SYNT-0420?")
		Expect(askAbsenceOf([]string{"923000000101", "SYNT0420"}, both)).To(ContainSubstring("SYNT-0420 in any number, device, SIM, address, cell or plate column"))
	})

	It("says what a camera search required when it found no row, and counts sightings with their cameras", func() {
		cat, err := askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
		question := "Was the plate SYNT-0420 seen at camera CAM-02 on 2026-03-05?"
		f := askExtractFrame(question)
		v, rej := askValidate("SELECT observed_utc, camera_id FROM v_plate_sightings WHERE plate_norm = 'SYNT0420' AND camera_id = 'CAM-02' AND observed_utc >= '2026-03-05 00:00:00+05' AND observed_utc < '2026-03-06 00:00:00+05'", cat, []string{"v_plate_sightings"})
		Expect(rej).To(BeNil())
		none := &govSQLResult{Columns: []string{"observed_utc", "camera_id"}, Texts: []bool{true, true}, Rows: [][]any{}}
		Expect(askWording(askNoRowSentence, none, v, f, "table", "")).To(Equal("No row matches what you asked: no sighting of the plate SYNT-0420 at camera CAM-02 on 2026-03-05 was found."))
		rows := &govSQLResult{Columns: []string{"observed_utc", "camera_id"}, Texts: []bool{true, true}, Rows: [][]any{{"2026-03-05 09:00", "CAM-02"}, {"2026-03-05 10:00", "CAM-03"}, {"2026-03-05 11:00", "CAM-02"}}}
		Expect(askWording("3 rows.", rows, v, f, "table", "")).To(Equal("3 sightings of the plate SYNT-0420 at 2 cameras."))
		leading := askExtractFrame("Does it prove the vehicle with the plate SYNT-0420 was at CAM-02?")
		Expect(askWording("x", rows, v, leading, "table", "")).To(HavePrefix("A camera sighting does not show who was driving"))
	})
})
