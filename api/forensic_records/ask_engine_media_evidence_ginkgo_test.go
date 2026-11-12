package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ask engine places behind an answer about a video", func() {
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})

	validated := func(sql string) *askValidated {
		v, rej := askValidate(sql, cat, nil)
		Expect(rej).To(BeNil(), sql)
		return v
	}

	It("plans the readings of a query that reads one media view, with its own conditions and alias", func() {
		plan := askMediaEvidencePlanOf(validated("SELECT count(DISTINCT plate) AS n FROM v_plate_reads WHERE source = 'video' AND original_filename = 'gate.mp4'"))
		Expect(plan).NotTo(BeNil())
		Expect(plan.View).To(Equal("v_plate_reads"))
		Expect(plan.Where).To(ContainSubstring("original_filename = 'gate.mp4'"))
		aliased := askMediaEvidencePlanOf(validated("SELECT p.plate FROM v_plate_reads p WHERE p.first_seen_seconds <= 20 AND p.last_seen_seconds >= 10"))
		Expect(aliased).NotTo(BeNil())
		Expect(aliased.Alias).To(Equal("p"))
	})

	It("does not plan a query that joins views, reads another family or builds intermediate steps", func() {
		Expect(askMediaEvidencePlanOf(validated("SELECT v.original_filename FROM v_ocr_regions v JOIN v_video_files f ON v.evidence_id = f.evidence_id"))).To(BeNil())
		Expect(askMediaEvidencePlanOf(validated("SELECT count(*) AS n FROM v_comm_events"))).To(BeNil())
		Expect(askMediaEvidencePlanOf(validated("WITH a AS (SELECT plate FROM v_plate_reads) SELECT count(*) AS n FROM a"))).To(BeNil())
		Expect(askMediaEvidencePlanOf(nil)).To(BeNil())
	})

	It("reads the same conditions again for the readings, earliest first, and the query passes the engine's own checks", func() {
		plan := askMediaEvidencePlanOf(validated("SELECT DISTINCT plate FROM v_plate_reads WHERE source = 'video' AND original_filename = 'gate.mp4' AND first_seen_seconds <= 20 AND last_seen_seconds >= 10"))
		Expect(plan).NotTo(BeNil())
		sql, ok := plan.rowsSQL(nil, 25)
		Expect(ok).To(BeTrue())
		Expect(sql).To(ContainSubstring("SELECT evidence_id::text AS evidence_id, original_filename, first_seen_seconds, last_seen_seconds FROM v_plate_reads WHERE ("))
		Expect(sql).To(ContainSubstring("ORDER BY first_seen_seconds NULLS LAST"))
		Expect(sql).To(HaveSuffix("LIMIT 25"))
		_, rej := askValidate(sql, cat, []string{"v_plate_reads"})
		Expect(rej).To(BeNil(), sql)
		frames := askMediaEvidencePlanOf(validated("SELECT count(DISTINCT frame_number) AS n FROM v_video_plate_frames WHERE original_filename = 'gate.mp4'"))
		framesSQL, ok := frames.rowsSQL(nil, 25)
		Expect(ok).To(BeTrue())
		Expect(framesSQL).To(ContainSubstring("seen_at_seconds, frame_number, x, y, width, height"))
		_, rej = askValidate(framesSQL, cat, []string{"v_video_plate_frames"})
		Expect(rej).To(BeNil(), framesSQL)
	})

	It("lists nothing for an answer that stands for every reading and names no file, and restricts a ranking to the files it names", func() {
		plan := askMediaEvidencePlanOf(validated("SELECT original_filename AS label, count(DISTINCT plate) AS value FROM v_plate_reads GROUP BY original_filename ORDER BY value DESC LIMIT 1"))
		Expect(plan).NotTo(BeNil())
		_, ok := plan.rowsSQL(nil, 25)
		Expect(ok).To(BeFalse())
		result := &govSQLResult{Columns: []string{"label", "value"}, Rows: [][]any{{"night-1.mp4", 5}, {"gate.mp4", 3}, {"a plain value", 1}}}
		files := askMediaFilesNamed(result)
		Expect(files).To(Equal([]string{"gate.mp4", "night-1.mp4"}))
		sql, ok := plan.rowsSQL(files, 25)
		Expect(ok).To(BeTrue())
		Expect(sql).To(ContainSubstring("original_filename IN ('gate.mp4', 'night-1.mp4')"))
	})

	It("turns the rows of the evidence query into places, with the files they lie in, and into openable sources without any text that was read", func() {
		plan := &askMediaEvidencePlan{View: "v_video_plate_frames"}
		columns := []string{"evidence_id", "original_filename", "seen_at_seconds", "frame_number", "x", "y", "width", "height"}
		rows := [][]any{
			{"ev-1", "gate.mp4", 6.96, int64(174), 492.0, 295.0, 272.0, 106.0},
			{"ev-1", "gate.mp4", 9.96, int64(249), 509.0, 304.0, 271.0, 94.0},
			{"ev-2", "night-1.mp4", 1.5, int64(45), 10.0, 20.0, 30.0, 40.0},
		}
		ev := askMediaRowsToEvidence(plan, columns, rows, map[string]string{"ev-1": "ver-1", "ev-2": "ver-2"}, false)
		Expect(ev.State).To(Equal("itemised"))
		Expect(ev.Total).To(Equal(3))
		Expect(ev.Files).To(HaveLen(2))
		Expect(ev.Files[0].Rows).To(Equal(2))
		Expect(ev.Rows[0].Media.Seconds).NotTo(BeNil())
		Expect(*ev.Rows[0].Media.Seconds).To(BeNumerically("==", 6.96))
		Expect(ev.Rows[0].Media.Box).To(HaveKeyWithValue("width", 272.0))
		Expect(*ev.Rows[0].Media.FrameNumber).To(Equal(int64(174)))
		Expect(ev.Rows[0].Preview).To(Equal("gate.mp4 6.96 s"))

		resp := &hybridQueryResponse{Enterprise: map[string]any{}}
		askAttachEvidence(resp, ev)
		provenance, _ := resp.Enterprise["provenance"].([]map[string]any)
		Expect(len(provenance)).To(BeNumerically(">=", 3))
		first := provenance[0]
		Expect(first["evidence_id"]).To(Equal("ev-1"))
		Expect(first["version_id"]).To(Equal("ver-1"))
		Expect(first["source_truth_state"]).To(Equal("derived_model_observation"))
		locator, _ := first["locator"].(map[string]any)
		Expect(locator).To(HaveKeyWithValue("timestamp_seconds", 6.96))
		Expect(locator).To(HaveKey("bbox"))
		Expect(locator).NotTo(HaveKey("plate"))
	})

	It("lists a reading only when its version is known, so that every source it offers can be opened", func() {
		plan := &askMediaEvidencePlan{View: "v_speech_segments"}
		ev := askMediaRowsToEvidence(plan, []string{"evidence_id", "original_filename", "start_seconds", "end_seconds"}, [][]any{{"ev-9", "call.wav", 3.0, 7.5}}, map[string]string{}, false)
		resp := &hybridQueryResponse{Enterprise: map[string]any{}}
		askAttachEvidence(resp, ev)
		provenance, _ := resp.Enterprise["provenance"].([]map[string]any)
		for _, p := range provenance {
			Expect(p["kind"]).NotTo(Equal("ask_engine_v2_media_evidence"), "no version, no openable source")
		}
		Expect(*ev.Rows[0].Media.EndSeconds).To(BeNumerically("==", 7.5))
	})
})
