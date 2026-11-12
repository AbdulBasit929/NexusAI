package main

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ASK ENGINE V2: MEDIA READINGS (docs/data/video-questions-20261112.md). What the media roles read from photos, video frames and audio is stored as derived artifacts, and migration 023 gives the
// engine views over it: v_plate_reads (a plate per photo, a plate GROUP per video: the frames in which the same plate was read, first and last second seen), v_video_plate_frames (every frame read,
// with its second), v_video_files (length, size, audio), v_speech_segments (the transcript, timed), v_ocr_regions and v_ocr_evidence_summary (text in photos and frames).
//
// Everything these views hold is a model reading that a person must check; none is an identification. A case that holds no such reading never sees these views: they are not offered to the
// shortlist, so every other question behaves exactly as it did before they existed (Shortlist keeps that meaning; ShortlistWith takes the views to leave out).

var askMediaViews = []string{"v_plate_reads", "v_video_plate_frames", "v_video_files", "v_speech_segments", "v_ocr_regions", "v_ocr_evidence_summary"}

var askMediaViewSet = func() map[string]bool {
	out := map[string]bool{}
	for _, v := range askMediaViews {
		out[v] = true
	}
	return out
}()

const askMediaPlateCaveat = "A plate here is what a model read from a photo or a video frame: a candidate that a person must check, never an identification of a vehicle or its owner. A reader can be wrong " +
	"(a letter that looks like a digit is often read as the digit), and the readings of one plate in a video differ from frame to frame. In a video a row of v_plate_reads is one plate group, the frames in which " +
	"the same plate was read, so a count of rows is a count of plates read, not of vehicles."
const askMediaSpeechCaveat = "Speech to text is a machine transcript of unverified quality (Urdu, Punjabi and Pashto are less reliable than English). It is not what was said until a person has listened."
const askMediaTextCaveat = "Text a model read from a photo or a video frame, as read: it can be wrong and needs a person's check."
const askMediaFileCaveat = "Length, size and audio are what the video container's header states; they say nothing about what the video shows."

// askReadsMediaView is whether the query reads one of the media views.
func askReadsMediaView(v *askValidated) bool {
	for _, view := range askMediaViews {
		if askReadsView(v, view) {
			return true
		}
	}
	return false
}

// askMediaCoverageQuery asks, for one case, which kinds of media reading it holds.
const askMediaCoverageQuery = `
SELECT
  EXISTS (SELECT 1 FROM forensic.derived_artifacts d WHERE d.tenant_id = $1 AND d.collection_id = $2 AND d.processing_status = 'completed'
          AND d.artifact_type IN ('forensics.anpr-observation/v1', 'forensics.video-anpr-plate-group/v1')),
  EXISTS (SELECT 1 FROM forensic.derived_artifacts d WHERE d.tenant_id = $1 AND d.collection_id = $2 AND d.processing_status = 'completed'
          AND d.artifact_type = 'forensics.audio-timestamp-segment/v1'),
  EXISTS (SELECT 1 FROM forensic.derived_artifacts d WHERE d.tenant_id = $1 AND d.collection_id = $2 AND d.processing_status = 'completed'
          AND d.artifact_type = 'forensics.image-ocr-observation/v1'),
  EXISTS (SELECT 1 FROM forensic.evidence_items e WHERE e.tenant_id = $1 AND e.collection_id = $2 AND e.modality = 'video')`

// askReadMediaCoverage fills the media fields of the coverage, with the connection's own scope.
func askReadMediaCoverage(ctx context.Context, tx pgx.Tx, tenantID, collectionID string, c *askCoverage) error {
	return tx.QueryRow(ctx, askMediaCoverageQuery, tenantID, collectionID).Scan(&c.AnyMediaPlate, &c.AnyMediaSpeech, &c.AnyMediaText, &c.AnyVideo)
}

// askMediaGap says why the media views cannot speak for this case when the question reads the given view; "" when they can (or the view is not a media view).
func askMediaGap(c askCoverage, view string) string {
	switch view {
	case "v_plate_reads", "v_video_plate_frames":
		if !c.AnyMediaPlate {
			return "the case holds no plate readings from photos or video"
		}
	case "v_speech_segments":
		if !c.AnyMediaSpeech {
			return "the case holds no speech transcripts"
		}
	case "v_ocr_regions", "v_ocr_evidence_summary":
		if !c.AnyMediaText {
			return "the case holds no text read from photos or video frames"
		}
	case "v_video_files":
		if !c.AnyVideo {
			return "the case holds no video files"
		}
	}
	return ""
}

// askAnyMedia is whether the case holds any media reading at all.
func askAnyMedia(c askCoverage) bool {
	return c.AnyMediaPlate || c.AnyMediaSpeech || c.AnyMediaText || c.AnyVideo
}

// askHasMediaView is whether any of the views is a media view.
func askHasMediaView(names []string) bool {
	for _, name := range names {
		if askMediaViewSet[name] {
			return true
		}
	}
	return false
}

// askExampleCount is how many verified examples a question is shown: two, and three when it reads a media view (the media views have many kinds of question and a model that is shown two
// missed the one that matters, measured 2026-11-12: a video grouped by its kind of evidence and not by its file).
func askExampleCount(names []string) int {
	if askHasMediaView(names) {
		return 3
	}
	return 2
}

// askClockTime is a clock time written in a question: 0:10, 1:05 or 1:02:03.
var askClockTime = regexp.MustCompile(`\b(\d{1,2}):(\d{2})(?::(\d{2}))?\b`)

// askMediaClockTimes reads, by code and never by the model, the clock times of a question about a video into seconds from the start: a time written 0:10 is minutes and seconds (10 seconds),
// and one written 1:02:03 is hours, minutes and seconds. A model left to read "from 0:10 to 0:20" took it for hours and minutes and asked for seconds 600 to 1200 (measured 2026-11-12).
func askMediaClockTimes(question string) []string {
	var out []string
	for _, m := range askClockTime.FindAllStringSubmatch(question, -1) {
		a, _ := strconv.Atoi(m[1])
		b, _ := strconv.Atoi(m[2])
		if b > 59 {
			continue
		}
		seconds := a*60 + b
		if m[3] != "" {
			c, _ := strconv.Atoi(m[3])
			if c > 59 {
				continue
			}
			seconds = a*3600 + b*60 + c
		}
		out = append(out, fmt.Sprintf("%s = %d seconds", m[0], seconds))
	}
	return askUnique(out)
}

// askOnlyMediaViews is whether the query reads media views and nothing else.
func askOnlyMediaViews(v *askValidated) bool {
	if v == nil || len(v.Views) == 0 {
		return false
	}
	for _, name := range v.Views {
		if !askMediaViewSet[name] {
			return false
		}
	}
	return true
}

// askMediaWord is a word that says the question is about footage, photos or recordings. A case that holds typed records as well as media readings is offered the media views only for a question
// that says so, so that its questions about calls, plates of a camera export or accounts are put to the model exactly as before.
var askMediaWord = regexp.MustCompile(`\b(?:videos?|clips?|footage|photos?|photographs?|images?|pictures?|frames?|mp4|mov|avi|mkv|jpe?g|png|recordings?|audio|speech|transcripts?|screenshots?|visible|on screen|on-screen)\b`)

// askHasTypedData is whether the case holds typed records of any family the analyst views read (call, internet, usage, plate sightings of a camera export, registrations, transactions, documents).
func askHasTypedData(c askCoverage) bool {
	return c.AnyComm || c.AnySession || c.AnyUsage || c.AnyPlate || c.AnyRegistration || c.AnyTransaction || c.AnyDocument || c.Files > 0
}

// askMediaExcluded are the views the shortlist leaves out for this case and question:
//   - the media views the case cannot speak for, so a case without media readings behaves as it always did;
//   - every media view, when the case also holds typed records and the question does not speak of footage, photos or recordings;
//   - the camera views of plates, when every plate of the case is a reading of a photo or a video (a question about plates is then about those readings);
//   - every view the case cannot speak for, when the case holds media readings and no typed record at all (nothing but media can answer it, and a call view that a general word of the
//     question had brought in would make the coverage guard decline the whole question).
func askMediaExcluded(cat *askCatalogue, c askCoverage, question string) map[string]bool {
	out := map[string]bool{}
	for _, v := range askMediaViews {
		if askMediaGap(c, v) != "" {
			out[v] = true
		}
	}
	switch {
	case !askAnyMedia(c):
		return out
	case askHasTypedData(c):
		if !askMediaWord.MatchString(strings.ToLower(question)) {
			for _, v := range askMediaViews {
				out[v] = true
			}
		}
	default:
		if cat != nil {
			for _, v := range cat.Views {
				if !askMediaViewSet[v.Name] && askCoverageGap(c, []string{v.Name}) != "" {
					out[v.Name] = true
				}
			}
		}
	}
	if c.AnyMediaPlate && !c.AnyPlate {
		out["v_plate_sightings"] = true
		out["v_plate_visits"] = true
	}
	return out
}

// askMediaPrune drops, from a shortlist whose best view is a media view the case can speak for, the views that score lower and that the case cannot speak for: a general word of the question
// ("when", "long", "day") had brought a call view into a question about a video, and the coverage guard would have declined the whole question for a family it never asked about.
// Without a media view as the best one, the shortlist is returned as it is.
func askMediaPrune(c askCoverage, short askShortlist) []string {
	chosen := append([]string{}, short.Views...)
	if len(chosen) == 0 || !askMediaViewSet[chosen[0]] || askMediaGap(c, chosen[0]) != "" {
		return chosen
	}
	out := []string{}
	for _, v := range chosen {
		if v != chosen[0] && short.Scores[v] < short.Scores[chosen[0]] && askCoverageGap(c, []string{v}) != "" {
			continue
		}
		out = append(out, v)
	}
	return out
}

// without is a copy of the catalogue with the named views left out; the shortlist then scores over the views that remain, so a left-out view changes nothing for the others.
func (c *askCatalogue) without(names map[string]bool) *askCatalogue {
	out := &askCatalogue{Views: make([]askView, 0, len(c.Views))}
	for _, v := range c.Views {
		if !names[v.Name] {
			out.Views = append(out.Views, v)
		}
	}
	return out
}

// askMediaAbsence says what was searched when a media view returns no row.
func askMediaAbsence(v *askValidated, f askFrame) string {
	switch {
	case askReadsView(v, "v_speech_segments"):
		return "no speech segment of this case matches your question"
	case askReadsView(v, "v_ocr_regions"), askReadsView(v, "v_ocr_evidence_summary"):
		return "no text read from a photo or a video frame of this case matches your question"
	case askReadsView(v, "v_video_files"):
		return "no video file of this case matches your question"
	}
	return "no plate reading from a photo or a video of this case matches your question. The reader may have missed it or misread it, so this does not show the plate is absent"
}

// askFileNameWord is a file name inside a question; the premise cue must not read "owner" in owner-sample.mp4.
var askFileNameWord = regexp.MustCompile(`\S+\.[a-z][a-z0-9]{1,4}\b`)

// askMediaLead is the correction put in front of a question that treats a machine reading as an identification or a fact about the person or the vehicle's owner. "" when it does not.
func askMediaLead(f askFrame, stated bool) string {
	if stated || askOwnershipCue.MatchString(askFileNameWord.ReplaceAllString(strings.ToLower(f.Question), " ")) {
		return "A reading a model made from a photo or a video frame is a candidate that needs checking. It does not say who owns the vehicle or who was in it."
	}
	return ""
}
