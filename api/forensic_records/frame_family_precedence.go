package main

import (
	"os"
	"strings"
)

// A3.2 — resolve a multi-family match through the EXISTING record-type
// precedence instead of collapsing it to cross_family. Default OFF until
// measured. See the multi-match branch of semanticFrameFamily.
const frameFamilyPrecedenceEnv = "FORENSIC_FRAME_FAMILY_PRECEDENCE"

// structuredRecordFamilies are the ingested structured-record families whose
// precedence `extractCanonicalRecordType` encodes. Deliberately an allowlist: a
// family not named here -- documents, media, faces, generic tables, anything
// added later -- keeps a multi-match question cross_family.
var structuredRecordFamilies = map[string]bool{
	"communications_cdr": true, "network_ipdr": true, "subscriber_identity": true,
	"device_sim": true, "tower_location": true, "financial_transactions": true,
	"access_security_logs": true, "anpr_vehicles": true,
}

func frameFamilyPrecedenceEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(frameFamilyPrecedenceEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// frameFamilyByRecordTypePrecedence returns the frame family that
// `extractCanonicalRecordType` would choose for the question, but ONLY when that
// family is among the ones the question actually matched. A record type that
// maps to no matched family -- or to none at all -- returns "", leaving the
// question cross_family exactly as before.
func frameFamilyByRecordTypePrecedence(question string, matched []string) string {
	if !frameFamilyPrecedenceEnabled() {
		return ""
	}
	// STRUCTURED FAMILIES ONLY. `extractCanonicalRecordType` is a structured-
	// records extractor: it does not know documents or media exist, so "plate"
	// always means ANPR sightings. The first census of this rule moved TEN
	// questions, and eight were wrong: DOC-01 and DOC-04 are document questions
	// that mention a plate (the known "no ANPR sightings involving MN1367"
	// misroute), and H1, H3, M1, M2, M5, VID-01 ask about plates a MODEL read
	// from images and video -- sending them to the ingested-sightings family is
	// the provenance conflation P1 exists to catch. So the precedence is used
	// only when every matched family is one it actually understands.
	for _, candidate := range matched {
		if !structuredRecordFamilies[candidate] {
			return ""
		}
	}
	recordType := extractCanonicalRecordType(question)
	if recordType == "" {
		return ""
	}
	family := capabilityFamilyForRecordType(recordType)
	if family == "" {
		return ""
	}
	for _, candidate := range matched {
		if candidate == family {
			return family
		}
	}
	return ""
}
