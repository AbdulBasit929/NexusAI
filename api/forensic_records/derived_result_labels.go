package main

import (
	"os"
	"strings"
)

// S4. A MODEL'S PLATE READ IS NOT A CAMERA'S SIGHTING -- IN THE SENTENCE TOO.
//
// Recorded on the deployed build, 2026-09-28:
//
//	"How many plate reads were produced from the images?"  -> "There are 307 ANPR sightings in this case."
//	"How many plate groups were tracked across the video frames?" -> "There are 24 ANPR sightings in this case."
//	"How many faces were detected in the evidence?"        -> "There are 20 records in this case."
//
// The numbers are right; the nouns are not. The sentence takes its noun from
// the request's record type, and a derived-media plan carries the structured
// family's record type ("anpr") or none at all. The same case holds 1,057
// CAMERA sightings, so "307 ANPR sightings" states a different kind of evidence
// than the one counted -- the conflation the architecture forbids ("a model's
// plate read is not a camera's sighting"), made in prose after the SQL got it
// right.
//
// The response already records exactly what was counted: every derived result
// cites its `artifact_type`. The label is taken from there, and only when every
// cited row agrees on one type; a mixed or structured provenance keeps today's
// noun.
const derivedResultLabelsEnv = "FORENSIC_DERIVED_RESULT_LABELS"

func derivedResultLabelsEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(derivedResultLabelsEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

var derivedArtifactNouns = map[string][2]string{
	"forensics.anpr-observation/v1":             {"plate read", "plate reads"},
	"forensics.video-anpr-plate-group/v1":       {"video plate group", "video plate groups"},
	"forensics.image-ocr-observation/v1":        {"image text region", "image text regions"},
	"forensics.face-observation/v1":             {"detected face", "detected faces"},
	"forensics.audio-timestamp-segment/v1":      {"transcribed audio segment", "transcribed audio segments"},
	"forensics.audio-roman-urdu-segment/v1":     {"Roman-Urdu transcript segment", "Roman-Urdu transcript segments"},
	"forensics.image-fingerprint/v1":            {"image fingerprint", "image fingerprints"},
	"forensics.image-embedding-observation/v1":  {"image embedding", "image embeddings"},
	"forensics.document-native-text-passage/v1": {"document passage", "document passages"},
	"forensics.image-observation/v1":            {"image observation", "image observations"},
	"forensics.audio-observation/v1":            {"audio observation", "audio observations"},
	"forensics.video-observation/v1":            {"video observation", "video observations"},
}

// resultNoun is the noun an answer sentence uses for what it counted: the
// derived artifact type when the result cites exactly one, else the request's
// record-type noun exactly as before.
func resultNoun(req hybridQueryRequest, resp hybridQueryResponse, n float64) string {
	return resultNounForFamily(req.RecordType, resp, n)
}

// resultNounForFamily is resultNoun for a caller that resolves its own family
// (the template path falls back to the template's primary record type).
func resultNounForFamily(family string, resp hybridQueryResponse, n float64) string {
	if noun, ok := derivedResultNoun(resp, n); ok {
		return noun
	}
	return familyNoun(family, n)
}

// derivedResultNoun returns the noun for what a derived-media result actually
// counted, or ok=false when the provenance is structured, mixed or unknown.
func derivedResultNoun(resp hybridQueryResponse, n float64) (string, bool) {
	if !derivedResultLabelsEnabled() {
		return "", false
	}
	entries := mapsFromAny(resp.Records["provenance"])
	if len(entries) == 0 {
		return "", false
	}
	kind := ""
	for _, entry := range entries {
		artifactType := strings.TrimSpace(stringValueAny(entry["artifact_type"]))
		if artifactType == "" || (kind != "" && artifactType != kind) {
			return "", false
		}
		kind = artifactType
	}
	nouns, ok := derivedArtifactNouns[kind]
	if !ok {
		return "", false
	}
	if n == 1 {
		return nouns[0], true
	}
	return nouns[1], true
}
