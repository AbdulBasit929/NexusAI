package agents

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Governed transcript query routing", func() {
	const (
		evidenceID = "11111111-1111-1111-8111-111111111111"
		versionID  = "22222222-2222-2222-8222-222222222222"
	)

	selectedAudio := func() ForensicRecordsToolConfig {
		return ForensicRecordsToolConfig{QueryScope: &ForensicQueryScope{
			Kind: "selected_evidence", EvidenceID: evidenceID, EvidenceVersionID: versionID, SourceFamily: "audio",
			AvailableResultFamilies: []string{"forensics.audio-timestamp-segment/v1", "forensics.audio-roman-urdu-segment/v1"},
		}}
	}

	It("keeps an ordinary question on the selected audio current version", func() {
		args := buildForensicHybridQueryArgs("What is in this recording?", selectedAudio())
		Expect(args.Template).To(Equal("audio_transcript_search"))
		Expect(args.TranscriptMode).To(Equal("source"))
		Expect(args.EvidenceID).To(Equal(evidenceID))
		Expect(args.EvidenceVersionID).To(Equal(versionID))
		Expect(args.QueryScope.Kind).To(Equal("selected_evidence"))
	})

	It("extracts exact terms in English, Urdu, and Roman Urdu", func() {
		tests := []struct{ query, term, language string }{
			{`Did the speaker mention project lantern?`, "project lantern", "en"},
			{`کیا پاکستان کا ذکر ہوا؟`, "پاکستان", "ur"},
			{`meeting kal ka zikr hua?`, "meeting kal", "roman_urdu"},
		}
		for _, test := range tests {
			args := buildForensicHybridQueryArgs(test.query, selectedAudio())
			Expect(args.TranscriptMode).To(Equal("exact"), test.query)
			Expect(args.ExactTerm).To(Equal(test.term), test.query)
			Expect(args.QueryLanguage).To(Equal(test.language), test.query)
		}
	})

	It("extracts source-time bounds without treating them as an exact term", func() {
		args := buildForensicHybridQueryArgs("What was said between 10 and 30 seconds?", selectedAudio())
		Expect(args.TranscriptMode).To(Equal("time_range"))
		Expect(args.ExactTerm).To(BeEmpty())
		Expect(args.StartSeconds).NotTo(BeNil())
		Expect(args.EndSeconds).NotTo(BeNil())
		Expect(*args.StartSeconds).To(Equal(10.0))
		Expect(*args.EndSeconds).To(Equal(30.0))
	})

	It("keeps anaphoric timing questions out of exact phrase mode", func() {
		for _, query := range []string{"When did they say that?", "انہوں نے یہ کب کہا؟"} {
			args := buildForensicHybridQueryArgs(query, selectedAudio())
			Expect(args.Template).To(Equal("audio_transcript_search"), query)
			Expect(args.TranscriptMode).To(Equal("source_time"), query)
			Expect(args.ExactTerm).To(BeEmpty(), query)
		}
	})

	It("does not let selected audio swallow an explicit plate query", func() {
		args := buildForensicHybridQueryArgs("Find exact plate ZZ99ZZZ. Return no exact match if absent.", selectedAudio())
		Expect(args.Template).To(Equal("anpr_sightings"))
		Expect(args.EvidenceID).To(Equal(evidenceID))
		Expect(args.Target).To(Equal("ZZ99ZZZ"))
	})

	It("routes an exact plate query on selected video to retained video observations", func() {
		config := ForensicRecordsToolConfig{QueryScope: &ForensicQueryScope{
			Kind: "selected_evidence", EvidenceID: evidenceID, EvidenceVersionID: versionID, SourceFamily: "video",
			AvailableResultFamilies: []string{"forensics.video-anpr-plate-group/v1"},
		}}
		args := buildForensicHybridQueryArgs("Find exact plate AB12XYZ in this selected evidence.", config)
		Expect(args.Template).To(Equal("video_anpr_grouped_timeline"))
		Expect(args.Target).To(Equal(evidenceID))
		Expect(args.Plate).To(Equal("AB12XYZ"))
	})

	It("keeps selected-image similarity and face wording on governed submodes", func() {
		config := ForensicRecordsToolConfig{QueryScope: &ForensicQueryScope{
			Kind: "selected_evidence", EvidenceID: evidenceID, EvidenceVersionID: versionID, SourceFamily: "image",
			AvailableResultFamilies: []string{"forensics.image-embedding/v1", "forensics.face-observation/v1"},
		}}
		image := buildForensicHybridQueryArgs("Find images similar to this.", config)
		Expect(image.Template).To(Equal("image_metadata"))
		Expect(image.EvidenceID).To(Equal(evidenceID))
		face := buildForensicHybridQueryArgs("Find candidate faces visually similar to this face.", config)
		Expect(face.Template).To(Equal("face_candidate_observations"))
		Expect(face.EvidenceID).To(Equal(evidenceID))
	})

	It("routes the Data-generated exact recognized-text prompt to selected-image OCR", func() {
		config := ForensicRecordsToolConfig{QueryScope: &ForensicQueryScope{
			Kind: "selected_evidence", EvidenceID: evidenceID, EvidenceVersionID: versionID, SourceFamily: "image",
			AvailableResultFamilies: []string{"forensics.image-ocr-observation/v1"},
		}}
		args := buildForensicHybridQueryArgs(`Find the exact recognized text "Investigation Workspace" in retained derived observations, focused on printed-english.png, and cite the matching artifact and source.`, config)
		Expect(args.Template).To(Equal("image_ocr_search"))
		Expect(args.ExactTerm).To(Equal("Investigation Workspace"))
		Expect(args.EvidenceID).To(Equal(evidenceID))
	})

	It("broadens only when the analyst explicitly asks for investigation scope", func() {
		selected := buildForensicHybridQueryArgs(`Find transcript phrase: "meeting"`, selectedAudio())
		Expect(selected.QueryScope.Kind).To(Equal("selected_evidence"))

		workspace := buildForensicHybridQueryArgs(`Search the whole investigation for transcript phrase: "meeting"`, selectedAudio())
		Expect(workspace.QueryScope.Kind).To(Equal("current_workspace"))
		Expect(workspace.QueryScope.EvidenceID).To(BeEmpty())
		Expect(workspace.EvidenceID).To(BeEmpty())

		romanUrdu := buildForensicHybridQueryArgs(`tamam transcript files mein "pichla rasta band rakho" search karo`, selectedAudio())
		Expect(romanUrdu.QueryScope.Kind).To(Equal("current_workspace"))
		Expect(romanUrdu.QueryScope.EvidenceID).To(BeEmpty())
		Expect(romanUrdu.EvidenceID).To(BeEmpty())
	})

	It("validates scope identifiers before transport", func() {
		Expect((ForensicQueryScope{Kind: "selected_evidence", EvidenceID: evidenceID, EvidenceVersionID: versionID}).Validate()).To(Succeed())
		Expect((ForensicQueryScope{Kind: "selected_evidence", EvidenceID: "bad"}).Validate()).To(MatchError(ContainSubstring("valid evidence UUID")))
		Expect((ForensicQueryScope{Kind: "current_workspace", EvidenceID: evidenceID}).Validate()).To(MatchError(ContainSubstring("cannot name evidence")))
	})
})
