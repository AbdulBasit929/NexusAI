package agents

import (
	"encoding/json"
	"time"

	"github.com/mudler/LocalAI/pkg/forensictext"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("B1 literal bridge", func() {
	It("keeps operation words inside the literal from steering the route", func() {
		args := buildForensicHybridQueryArgs(`Find this transcript phrase "show CDR frequent contacts"`, ForensicRecordsToolConfig{})
		Expect(args.Template).To(Equal("audio_transcript_search"))
		Expect(args.TextQuery.LiteralText).To(Equal("show CDR frequent contacts"))
	})
	DescribeTable("routes quoted spans before template extraction", func(input, family, wanted, literal string) {
		cfg := ForensicRecordsToolConfig{}
		if family != "" {
			cfg.QueryScope = &ForensicQueryScope{Kind: "selected_evidence", SourceFamily: family, EvidenceID: "11111111-1111-4111-8111-111111111111", EvidenceVersionID: "22222222-2222-4222-8222-222222222222"}
		}
		start := time.Now()
		args := buildForensicHybridQueryArgs(input, cfg)
		Expect(args.Template).To(Equal(wanted))
		Expect(args.TextQuery).NotTo(BeNil())
		Expect(args.TextQuery.LiteralText).To(Equal(literal))
		Expect(args.TextQuery.MatchSemantic).To(Equal(forensictext.PhraseContains))
		Expect(time.Since(start)).To(BeNumerically("<", time.Second))
		payload, err := json.Marshal(args)
		Expect(err).NotTo(HaveOccurred())
		var roundTrip ForensicHybridQueryArgs
		Expect(json.Unmarshal(payload, &roundTrip)).To(Succeed())
		Expect(roundTrip.TextQuery).To(Equal(args.TextQuery))
	}, Entry("historical-style Urdu", `find this transcript phrase " نئی رپورٹ"`, "", "audio_transcript_search", " نئی رپورٹ"), Entry("English", `Does this recording contain "clear phrase"?`, "", "audio_transcript_search", "clear phrase"), Entry("selected audio", `Search for "clear phrase"`, "audio", "audio_transcript_search", "clear phrase"), Entry("selected image", `Search for "clear phrase"`, "image", "image_ocr_search", "clear phrase"), Entry("selected document", `Search for "clear phrase"`, "document", "document_search", "clear phrase"), Entry("explicit document", `Find document phrase "clear phrase"`, "", "document_search", "clear phrase"))
	It("preserves unresolved literal scope without manufacturing a family", func() {
		args := buildForensicHybridQueryArgs(`Search for "clear phrase"`, ForensicRecordsToolConfig{})
		Expect(args.Template).To(BeEmpty())
		Expect(args.TextQuery.LiteralText).To(Equal("clear phrase"))
	})
	It("keeps source-time anaphora on the accepted mode", func() {
		args := buildForensicHybridQueryArgs("When did they say that?", ForensicRecordsToolConfig{QueryScope: &ForensicQueryScope{Kind: "selected_evidence", SourceFamily: "audio"}})
		Expect(args.TextQuery).To(BeNil())
		Expect(args.TranscriptMode).To(Equal("source_time"))
	})
})
