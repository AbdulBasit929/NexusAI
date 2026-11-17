package main

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ASK ENGINE V2: MEDIA READINGS (docs/data/video-questions-20261112.md). What the media roles read from photos, video frames and audio is stored as derived artifacts, and migration 023 gives the
// engine views over it: v_plate_reads (a plate per photo, a plate GROUP per video: the frames in which the same plate was read, first and last second seen), v_video_plate_frames (every frame read,
// with its second), v_video_files (length, size, audio), v_audio_files (migration 037: an audio recording, its length, language and reading), v_speech_segments (the transcript, timed, with its
// language and whether it is a recording or the soundtrack of a video), v_ocr_regions and v_ocr_evidence_summary (text in photos and frames).
//
// Everything these views hold is a model reading that a person must check; none is an identification. A case that holds no such reading never sees these views: they are not offered to the
// shortlist, so every other question behaves exactly as it did before they existed (Shortlist keeps that meaning; ShortlistWith takes the views to leave out).

var askMediaViews = []string{"v_plate_reads", "v_video_plate_frames", "v_video_files", "v_audio_files", "v_speech_segments", "v_ocr_regions", "v_ocr_evidence_summary"}

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
const askMediaAudioFileCaveat = "Length, sample rate and channels are what the audio file's own header states. The language and the reading are a machine's: a person must listen before relying on them (a Pashto reading is a search lead, not a transcript)."

// askReadsOnlyAudioViews is whether the query reads recordings and no other media view.
func askReadsOnlyAudioViews(v *askValidated) bool {
	if !askReadsView(v, "v_audio_files") && !askReadsView(v, "v_speech_segments") {
		return false
	}
	for _, view := range askMediaViews {
		if view != "v_audio_files" && view != "v_speech_segments" && askReadsView(v, view) {
			return false
		}
	}
	return true
}

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
  EXISTS (SELECT 1 FROM forensic.evidence_items e WHERE e.tenant_id = $1 AND e.collection_id = $2 AND e.modality = 'video'),
  EXISTS (SELECT 1 FROM forensic.evidence_items e WHERE e.tenant_id = $1 AND e.collection_id = $2 AND e.modality = 'audio')`

// askReadMediaCoverage fills the media fields of the coverage, with the connection's own scope.
func askReadMediaCoverage(ctx context.Context, tx pgx.Tx, tenantID, collectionID string, c *askCoverage) error {
	return tx.QueryRow(ctx, askMediaCoverageQuery, tenantID, collectionID).Scan(&c.AnyMediaPlate, &c.AnyMediaSpeech, &c.AnyMediaText, &c.AnyVideo, &c.AnyAudio)
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
	case "v_audio_files":
		if !c.AnyAudio {
			return "the case holds no audio recordings"
		}
	}
	return ""
}

// askAnyMedia is whether the case holds any media reading at all.
func askAnyMedia(c askCoverage) bool {
	return c.AnyMediaPlate || c.AnyMediaSpeech || c.AnyMediaText || c.AnyVideo || c.AnyAudio
}

// askHowManyUnit is a "how many" that is followed by a unit of time: it asks for a measure (a position in a video, a length of time), which a MIN, MAX, AVG or SUM states, and not for the number of
// rows. The count obligation of the frame accepts such a query (measured 2026-11-12: "how many seconds into the video does the first plate show up" was abstained three times for want of a COUNT).
// A plain read of a length column also states such a measure (measured 2026-11-16: "How many seconds is the file voice-note-01.wav?" was abstained three times, the right query being SELECT duration_seconds).
var askHowManyUnit = regexp.MustCompile(`\bhow (?:many|much)\s+(?:seconds?|minutes?|hours?|days?|weeks?|months?|years?)\b`)

// askMediaFileNameRE is the file name of a video, a recording or a picture inside a question.
var askMediaFileNameRE = regexp.MustCompile(`(?i)\S+\.(?:mp4|mov|avi|mkv|webm|m4v|wmv|flv|3gp|mp3|wav|m4a|aac|ogg|flac|amr|opus|jpe?g|png|gif|bmp|tiff?|heic|webp)\b`)

// askWithoutMediaFileNames is the question with the file names of media taken out: a file name is an identifier, not words ("night" in street-night-hd.mp4 is not a time of day, and "owner" in
// owner-sample.mp4 is not a question about an owner: measured 2026-11-12 on the Orion case, where the engine added an hour filter and answered "0 plates"). The names stay in the question the model reads.
func askWithoutMediaFileNames(question string) string {
	return askMediaFileNameRE.ReplaceAllString(question, " ")
}

// askMediaFileWords are the words that a media file name says about the kind of source: a question that names clip.mp4 is about a video, one that names note.m4a about a recording, one that names
// scene.jpg about a photo. ONE word is added to the question for the shortlist, so that "how long is clip.mp4" finds the view of videos (the file name is one token and says nothing else). One and not
// several: two strong words of the video view (video, clip) outweighed "plates" and took the plate views out of the shortlist of "how many plates were read in clip.mp4" (caught by the media specs).
func askMediaFileWords(question string) string {
	kinds := map[string]string{}
	for _, field := range strings.Fields(strings.ToLower(question)) {
		token := strings.TrimRight(strings.Trim(field, "?!,;:()[]{}\"'"), ".")
		dot := strings.LastIndex(token, ".")
		if dot < 0 || !tiFileNameToken.MatchString(token) {
			continue
		}
		switch ext := token[dot+1:]; ext {
		case "mp4", "mov", "avi", "mkv", "webm", "m4v", "wmv", "flv", "3gp":
			kinds["video"] = "video"
		case "mp3", "wav", "m4a", "aac", "ogg", "flac", "amr", "opus":
			kinds["audio"] = "audio"
		case "jpg", "jpeg", "png", "gif", "bmp", "tif", "tiff", "heic", "webp":
			kinds["photo"] = "photo"
		}
	}
	var words []string
	for _, k := range kinds {
		words = append(words, k)
	}
	sort.Strings(words)
	return strings.Join(words, " ")
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
var askMediaWord = regexp.MustCompile(`\b(?:videos?|clips?|footage|photos?|photographs?|images?|pictures?|frames?|mp4|mov|avi|mkv|jpe?g|png|recordings?|audio|speech|transcripts?|screenshots?|visible|on screen|on-screen|wav|mp3|m4a|aac|ogg|flac|amr|opus|voice notes?)\b`)

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
	for _, files := range []string{"v_video_files", "v_audio_files"} {
		if askMediaGap(c, files) == "" {
			// v_video_files lists every video with its length, size and audio (and v_audio_files every recording with its length, language and reading), so v_evidence_inventory beside it only gives a
			// model two views to mix (measured 2026-11-12: a column of one asked of the other, twice)
			kept := []string{}
			for _, v := range chosen {
				if !(v == "v_evidence_inventory" && askHasMediaView(chosen) && containsString(chosen, files)) {
					kept = append(kept, v)
				}
			}
			chosen = kept
		}
	}
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
	case askReadsView(v, "v_audio_files"):
		return "no audio recording of this case matches your question"
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
