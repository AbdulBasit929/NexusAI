package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"os"
	"strings"
	"unicode"
)

// PII MASKING AT PROJECTION — the plate alias.
//
// WI-LAYER-7's curation ruling, accepted: a plate candidate is projected as
// "one opaque, case-scoped alias shared across both families", and **revealing
// a prefix, suffix or length is not approved**. That last clause rules out the
// `••••-A7C2` shape an earlier handoff proposed: a trailing fragment of a plate
// is not a safe remainder.
//
// WHY A SECRET IS REQUIRED, AND WHY THIS FAILS CLOSED WITHOUT ONE.
//
// Plate strings are LOW ENTROPY — that is the stated reason last-four was
// rejected, and it applies with equal force to an unkeyed hash. A plausible
// Pakistani registration space is a few million strings, so `sha256(plate)` or
// `sha256(collection||plate)` is reversible by enumeration in seconds by anyone
// holding an alias and this source file. An unkeyed digest is not an alias; it
// is the plate in a different encoding.
//
// So the alias is an HMAC under a deployment secret, and when that secret is
// absent masking REFUSES rather than degrading to an unkeyed digest. A masking
// scheme that silently weakens when misconfigured is worse than none, because
// the label still says MASKED. This is the same fail-closed posture as
// FORENSIC_API_AUTH_REQUIRED, and for the same reason: on 2026-09-17 a whole
// session served the forensic API unauthenticated because a missing value was
// treated as permission to proceed.
//
// CASE-SCOPED. The collection id is bound into the MAC, so the same plate in two
// cases yields two unrelated aliases and an analyst cannot correlate subjects
// across cases they were not authorized for. Within one case the alias IS stable
// and IS shared between `anpr_model_observation` and `video_anpr_plate_group` —
// both curate `observation.normalized_plate_text` — which is precisely the
// analytically useful property: the same vehicle reads as the same alias whether
// it came from a still or a video group.
//
// FIXED WIDTH. Every alias is the same length regardless of the value behind it,
// so the rendering discloses nothing about the plate's length.
const (
	piiAliasSecretEnv = "FORENSIC_PII_ALIAS_SECRET"
	// 10 base32 characters = 50 bits, over a Crockford-style alphabet with I, L,
	// O and U removed so an alias is unambiguous when read aloud or transcribed.
	// Across the 307 retained plate candidates
	// a collision is ~1 in 10^11, and the width is constant.
	piiAliasChars = 10
)

var piiAliasEncoding = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

// FORENSIC_PII_MASKED_PROJECTION gates admitting MASKED fields to the issued
// catalogue at all. Default OFF, because this is a behavioural change that makes
// a previously unanswerable class of question answerable, and it must be
// measured against a control like every other one.
//
// Unlike the derived provenance label — which shipped without a switch because
// default-off would have kept a lying citation as the default — nothing unsafe
// happens while this is off: the fields simply stay withheld, which is exactly
// the state that has been shipping.
const piiMaskedProjectionEnv = "FORENSIC_PII_MASKED_PROJECTION"

func piiMaskedProjectionEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(piiMaskedProjectionEnv)), "true") //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// piiAliasSecret returns the deployment masking secret, or "" when unset.
func piiAliasSecret() string {
	return strings.TrimSpace(os.Getenv(piiAliasSecretEnv)) //nolint:forbidigo // forensic records product API reads its feature switches from the environment by design (docker compose); not LocalAI server config
}

// piiMaskingAvailable reports whether a value can be masked at all. When this is
// false the caller must WITHHOLD, never emit the raw value and never fall back
// to an unkeyed digest.
func piiMaskingAvailable() bool {
	return piiAliasSecret() != ""
}

// normalizePlateForAlias folds a plate candidate to the form the alias is
// computed over, so the same vehicle maps to the same alias across both
// families whatever separators or case the producer emitted.
//
// It is deliberately aggressive: a plate that differs only by spacing, hyphens
// or case is the SAME plate, and giving it two aliases would split one vehicle
// into two apparent subjects — a fabricated distinction, which is the same class
// of error as merging two into one.
func normalizePlateForAlias(value string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// piiPlateAlias returns the analyst-facing alias for one plate candidate.
//
// The second result is false when masking is unavailable or the value is empty;
// the caller must then withhold. It NEVER returns the raw value.
func piiPlateAlias(collectionID, rawValue string) (string, bool) {
	normalized := normalizePlateForAlias(rawValue)
	if normalized == "" {
		return "", false
	}
	secret := piiAliasSecret()
	if secret == "" {
		return "", false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	// The domain tag keeps this MAC from colliding with any other use of the
	// same secret, and the collection id is length-prefixed so that
	// ("ab","c") and ("a","bc") cannot produce the same input.
	mac.Write([]byte("nexusai/pii-alias/plate/v1\x00"))
	mac.Write([]byte(collectionID))
	mac.Write([]byte{0})
	mac.Write([]byte(normalized))
	encoded := piiAliasEncoding.EncodeToString(mac.Sum(nil))
	return "Plate candidate " + encoded[:piiAliasChars], true
}

// piiMaskScheme names the masking scheme implemented for a field, or "" when
// none is.
//
// THIS IS A WHITELIST AND IT EXISTS BECAUSE THE FIRST VERSION WAS NOT.
// Admitting every `PII`/`MASKED` field let `subscriber.cnic` into the dynamic
// catalogue -- a national identity number -- alongside `subscriber.full_name`
// and `subscriber.cnic_last4`. Those three are `MASKED` in the curation, but
// WI-LAYER-7 ruled on the PLATE fields only and no scheme exists for a CNIC or a
// person name. They would have been admitted and then run through the PLATE
// aliaser, labelling a national ID "Plate candidate ...". Caught by
// TestWI4NoPIIFieldReachesTheDynamicCatalogue, which was already there.
//
// So the question is not "is this field MASKED?" but "can this field actually be
// masked?", and a field whose scheme is unimplemented stays withheld. Adding a
// scheme is a deliberate act with its own ruling, not a consequence of curation
// setting a redaction value.
//
// Keyed on the curated SOURCE NAME, which is what makes the plate alias shared:
// both plate families declare `observation.normalized_plate_text`, so the same
// vehicle yields the same alias in each.
func piiMaskScheme(field FieldDescriptorV1) string {
	for _, name := range append([]string{field.SourceName}, field.SourceNames...) {
		if strings.EqualFold(strings.TrimSpace(name), "observation.normalized_plate_text") {
			return "plate_alias_v1"
		}
	}
	return ""
}

// maskCuratedValue applies the curated redaction to one value before it can
// reach an analyst.
//
// It is keyed on the field's own declared `redaction`, so the curated layer
// decides and this code does not carry a second list that could disagree with
// it. An unrecognised redaction WITHHOLDS: a new redaction value must be
// implemented deliberately, not default to disclosure.
func maskCuratedValue(field FieldDescriptorV1, collectionID string, raw any) (any, bool) {
	if field.RedactionState != "MASKED" {
		return nil, false
	}
	// A MASKED field with no implemented scheme yields NOTHING, never the raw
	// value. It should not have been admitted at all; if one reaches here the
	// safe outcome is an empty cell, not a disclosure.
	if piiMaskScheme(field) != "plate_alias_v1" {
		return nil, true
	}
	text := strings.TrimSpace(stringValueAny(raw))
	if text == "" {
		return nil, true
	}
	alias, ok := piiPlateAlias(collectionID, text)
	if !ok {
		return nil, true
	}
	return alias, true
}
