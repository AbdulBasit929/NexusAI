package main

import (
	"strings"
	"time"
)

// The CONSTRAINT_APPLIED allowlist decides whether a filter value came from the
// analyst. It is a SAFETY control -- it is what stops a generated plan from
// filtering on a value nobody asked for -- and every widening of it has to be
// justified by a measured false rejection, never by a hunch that it is strict.
//
// Two shapes were measured as false rejections on 2026-09-24, both read from
// the CACHED MODEL COMPLETION rather than inferred: an all-alphabetic
// identifier (SUB-03) and a date bound derived from a month name (CDR-14). In
// both the generator produced the exactly-correct plan and the allowlist
// refused it.
//
// Deliberately NOT widened: a value the generator invented outright. CDR-11
// appended `cdr.direction EQ "outbound"` and TWR-02 appended
// `tower.site_location CONTAINS "where"` to otherwise-correct plans. The
// allowlist is RIGHT to refuse those, and they are handled as a separate
// concern -- an invented filter is not a missing literal.

// structuredIdentifierSeparators are the characters real case identifiers use.
func structuredIdentifierSeparators(r rune) bool {
	return r == '-' || r == '_' || r == '/' || r == ':' || r == '.'
}

// structuredIdentifierToken reports a token that is an identifier by SHAPE
// rather than by carrying a digit: at least two non-empty separated segments.
//
// "PK-SUB-SYN-ALPHA" qualifies. An ordinary word does not, because it has no
// separator at all -- which is what keeps this from admitting the question's
// own vocabulary as filterable values.
func structuredIdentifierToken(token string) bool {
	if !strings.ContainsFunc(token, structuredIdentifierSeparators) {
		return false
	}
	segments := 0
	for _, part := range strings.FieldsFunc(token, structuredIdentifierSeparators) {
		if strings.TrimSpace(part) != "" {
			segments++
		}
	}
	return segments >= 2
}

// sourceNativeTemporalLayouts are the renderings a generated plan uses for an
// instant. The list is closed: an unparseable value is NOT analyst-supplied.
var sourceNativeTemporalLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02 15:04:05",
	"2006-01-02",
	"2006-01",
	"2006",
}

func parseSourceNativeInstant(value string) (time.Time, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, false
	}
	for _, layout := range sourceNativeTemporalLayouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			return parsed.UTC(), true
		}
	}
	return time.Time{}, false
}

// temporalFilterWithinRequestedRange reports that a date bound on a TEMPORAL
// field falls inside the range the analyst asked for.
//
// Three conditions, all required, so this cannot become a general escape from
// the allowlist:
//
//  1. the analyst supplied a date range -- `req.DateFrom`/`DateTo` come from
//     deterministic extraction (S2), not from the model;
//  2. the filtered field is a TIMESTAMP or DATE in the issued catalog, so a
//     string column can never be validated this way;
//  3. the value parses as an instant and lies within the requested range,
//     inclusive of both ends.
//
// The end is widened by one day because a month is legitimately rendered as
// either an inclusive last instant or an exclusive next-month boundary, and
// both were observed from the same question on different runs.
func temporalFilterWithinRequestedRange(req hybridQueryRequest, fieldID, value string, catalog []FieldDescriptorV1) bool {
	from, hasFrom := parseSourceNativeInstant(req.DateFrom)
	to, hasTo := parseSourceNativeInstant(req.DateTo)
	if !hasFrom && !hasTo {
		return false
	}
	temporal := false
	for _, field := range catalog {
		if field.FieldID != fieldID {
			continue
		}
		temporal = field.EffectiveType == fieldTypeTimestamp || field.EffectiveType == fieldTypeDate
		break
	}
	if !temporal {
		return false
	}
	instant, ok := parseSourceNativeInstant(value)
	if !ok {
		return false
	}
	if hasFrom && instant.Before(from) {
		return false
	}
	if hasTo && instant.After(to.AddDate(0, 0, 1)) {
		return false
	}
	return true
}
