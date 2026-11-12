package main

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Ask engine media views (migration 023)", func() {
	var cat *askCatalogue
	BeforeEach(func() {
		var err error
		cat, err = askGoldenCatalogue()
		Expect(err).NotTo(HaveOccurred())
	})

	mediaOnly := askCoverage{AnyMediaPlate: true, AnyMediaSpeech: true, AnyMediaText: true, AnyVideo: true}
	mixed := askCoverage{AnyMediaPlate: true, AnyMediaSpeech: true, AnyMediaText: true, AnyVideo: true, AnyComm: true, AnyPlate: true, Files: 1, Typed: 1}

	It("keeps every media view out of Shortlist, so a case without media readings behaves as it did before they existed", func() {
		for _, q := range []string{"Which plates were read in the video gate.mp4?", "How long is the video gate.mp4?", "What is said in the videos?", "What text is visible in the videos?"} {
			for _, view := range cat.Shortlist(q, 0).Views {
				Expect(askMediaViewSet[view]).To(BeFalse(), q)
			}
		}
		Expect(askMediaExcluded(cat, askCoverage{}, "Which plates were read in the video gate.mp4?")).To(HaveLen(len(askMediaViews)))
	})

	It("offers the media views a case can speak for, and the camera views of plates only when the case has camera sightings", func() {
		excludedMediaOnly := askMediaExcluded(cat, mediaOnly, "Which plates were read in the video gate.mp4?")
		Expect(excludedMediaOnly).To(HaveKey("v_plate_sightings"))
		Expect(excludedMediaOnly).To(HaveKey("v_plate_visits"))
		Expect(excludedMediaOnly).To(HaveKey("v_comm_events"), "no typed record at all: a call view cannot speak for the case")
		Expect(excludedMediaOnly).NotTo(HaveKey("v_plate_reads"))
		Expect(excludedMediaOnly).NotTo(HaveKey("v_evidence_inventory"))
		Expect(askMediaExcluded(cat, mixed, "Which plates were read in the video gate.mp4?")).To(BeEmpty())
		speechOnly := askCoverage{AnyMediaSpeech: true}
		excluded := askMediaExcluded(cat, speechOnly, "What is said in the interview?")
		Expect(excluded).To(HaveKey("v_plate_reads"))
		Expect(excluded).To(HaveKey("v_video_files"))
		Expect(excluded).NotTo(HaveKey("v_speech_segments"))
		Expect(excluded).To(HaveKey("v_plate_sightings"), "a case with no typed record at all: a view it cannot speak for is not offered")
		Expect(excluded).NotTo(HaveKey("v_evidence_inventory"))
	})

	It("finds the media view for questions about videos, plates in them, speech and text, in a case that holds media readings", func() {
		for _, c := range []struct {
			q     string
			views []string
		}{
			{"How many different number plates were read in pexels-6719160-hd.mp4?", []string{"v_plate_reads"}},
			{"Which video has the most distinct plates?", []string{"v_plate_reads"}},
			{"At what time in owner-sample-4k.mp4 is the first plate seen?", []string{"v_plate_reads"}},
			{"Which plates appear between 0:10 and 0:20 in pexels-6571483-hd.mp4?", []string{"v_plate_reads"}},
			{"Is any plate seen in more than one video?", []string{"v_plate_reads"}},
			{"Which plate stays on screen the longest in pexels-6719160-hd.mp4?", []string{"v_plate_reads"}},
			{"How many frames were read in roboflow-vehicles-4k.mp4?", []string{"v_video_plate_frames"}},
			{"What is the highest confidence plate reading in pexels-5293898-hd.mp4?", []string{"v_video_plate_frames"}},
			{"How long is the video gate.mp4?", []string{"v_video_files"}},
			{"How many minutes of footage does the case hold?", []string{"v_video_files"}},
			{"What is said in the videos?", []string{"v_speech_segments"}},
			{"Which recordings have someone say the word delivery?", []string{"v_speech_segments"}},
			{"What text is visible in the videos?", []string{"v_ocr_regions"}},
			{"Which photos or videos show the text INVOICE?", []string{"v_ocr_regions"}},
			{"What text can be read in the videos?", []string{"v_ocr_regions"}},
		} {
			short := cat.ShortlistWith(c.q, 0, askMediaExcluded(cat, mediaOnly, c.q))
			Expect(short.Confident).To(BeTrue(), c.q)
			for _, v := range c.views {
				Expect(short.Views).To(ContainElement(v), c.q)
			}
			Expect(len(short.Views)).To(BeNumerically("<=", askShortlistMax), c.q)
		}
	})

	It("loses no view of the labelled set when the media views are in the catalogue of a case that holds both kinds of data", func() {
		var cases []askRetrievalCase
		Expect(json.Unmarshal(askRetrievalCasesJSON, &cases)).To(Succeed())
		var total, hit int
		var misses []string
		for _, c := range cases {
			if c.Heldout {
				continue
			}
			total++
			short := cat.ShortlistWith(c.Q, 0, askMediaExcluded(cat, mixed, c.Q))
			ok := true
			for _, v := range c.Views {
				ok = ok && askContains(short.Views, v)
			}
			if ok {
				hit++
			} else {
				misses = append(misses, c.Q)
			}
		}
		Expect(float64(hit)/float64(total)).To(BeNumerically(">=", askGateRecall), "misses: %v", misses)
	})

	It("offers a case that holds typed records as well the media views only for a question that speaks of footage, photos or recordings", func() {
		call := "Who does 923001110001 talk to most?"
		Expect(askMediaExcluded(cat, mixed, call)).To(HaveKey("v_plate_reads"))
		Expect(askMediaExcluded(cat, mixed, call)).To(HaveKey("v_video_files"))
		Expect(askMediaExcluded(cat, mixed, "How many times was the plate SYNT0001 seen?")).To(HaveKey("v_plate_reads"))
		Expect(askMediaExcluded(cat, mixed, "How many times was the plate SYNT0001 seen on video?")).NotTo(HaveKey("v_plate_reads"))
		Expect(askMediaExcluded(cat, mixed, "What is said in the recording interview.mp4?")).NotTo(HaveKey("v_speech_segments"))
		Expect(askMediaExcluded(cat, mixed, "Which photos show the text INVOICE?")).NotTo(HaveKey("v_ocr_regions"))
	})

	It("says why a media view cannot speak for a case that holds no such reading", func() {
		Expect(askCoverageGap(askCoverage{}, []string{"v_plate_reads"})).To(Equal("the case holds no plate readings from photos or video"))
		Expect(askCoverageGap(askCoverage{}, []string{"v_video_files"})).To(Equal("the case holds no video files"))
		Expect(askCoverageGap(askCoverage{}, []string{"v_speech_segments"})).To(Equal("the case holds no speech transcripts"))
		Expect(askCoverageGap(askCoverage{}, []string{"v_ocr_regions"})).To(Equal("the case holds no text read from photos or video frames"))
		Expect(askCoverageGap(mediaOnly, []string{"v_plate_reads", "v_video_files", "v_speech_segments", "v_ocr_regions", "v_video_plate_frames", "v_ocr_evidence_summary"})).To(BeEmpty())
	})

	It("drops a view the case cannot speak for from a media question whose best view the case can speak for, and only then", func() {
		short := askShortlist{Views: []string{"v_plate_reads", "v_comm_events"}, Scores: map[string]float64{"v_plate_reads": 5, "v_comm_events": 2}}
		Expect(askMediaPrune(mediaOnly, short)).To(Equal([]string{"v_plate_reads"}))
		// a case that holds call records too keeps the view it can speak for
		Expect(askMediaPrune(mixed, short)).To(Equal([]string{"v_plate_reads", "v_comm_events"}))
		// a question whose best view is not a media view is returned as it is, even when a media view follows it
		call := askShortlist{Views: []string{"v_comm_events", "v_plate_reads"}, Scores: map[string]float64{"v_comm_events": 5, "v_plate_reads": 2}}
		Expect(askMediaPrune(mediaOnly, call)).To(Equal([]string{"v_comm_events", "v_plate_reads"}))
		// a media view that the case cannot speak for is never the anchor
		noMedia := askShortlist{Views: []string{"v_plate_reads", "v_comm_events"}, Scores: map[string]float64{"v_plate_reads": 5, "v_comm_events": 2}}
		Expect(askMediaPrune(askCoverage{}, noMedia)).To(Equal([]string{"v_plate_reads", "v_comm_events"}))
	})

	It("carries a caveat for every media view, and declares each media view untraceable with its reason", func() {
		for _, view := range askMediaViews {
			Expect(askViewCaveatText[view]).NotTo(BeEmpty(), view)
			Expect(askUntraceable[view]).NotTo(BeEmpty(), view)
			Expect(cat.View(view)).NotTo(BeNil(), view)
		}
		Expect(askMediaPlateCaveat).To(ContainSubstring("never an identification"))
	})

	It("has a verified example for the questions an analyst asks about a video, and each reads only media views", func() {
		examples, err := askExamples()
		Expect(err).NotTo(HaveOccurred())
		byID := map[string]askExample{}
		for _, e := range examples {
			byID[e.ID] = e
		}
		for _, id := range []string{"video-plate-count", "video-plate-list", "video-first-plate", "video-plate-window", "video-plate-longest", "video-most-plates", "plate-in-several-videos",
			"video-frames-with-plates", "video-best-reading", "video-length", "video-footage-total", "speech-window", "speech-word", "ocr-word"} {
			e, ok := byID[id]
			Expect(ok).To(BeTrue(), id)
			Expect(e.Views).NotTo(BeEmpty(), id)
			for _, v := range e.Views {
				Expect(askMediaViewSet[v]).To(BeTrue(), id)
			}
		}
	})
})
