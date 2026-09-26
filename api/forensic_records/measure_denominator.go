package main

import (
	"fmt"
	"os"
	"strings"
)

// AN AVERAGE MUST SAY WHAT IT AVERAGED OVER.
//
// Codex raised this while curating the media layer and it is the sharpest point
// anyone has made about the answer surface:
//
//	forensics.image-ocr-observation/v1, 309 retained observations
//	    observation.confidence               263 non-null   85%
//	    observation.detection_confidence      46 non-null   15%
//	    observation.recognition_confidence    46 non-null   15%
//
// Two producers emit disjoint schemas, so the fields were curated separately --
// correctly, because their scales are not established as equivalent and
// coalescing them would invent a comparison. General phrasings now bind to the
// 263-row field, so the 46-row ones cannot be reached by "average OCR
// confidence".
//
// **But 263 of 309 is still not all of it.** "The average general OCR confidence
// is 0.87" reads as a statement about every observation in the case. It is a
// statement about 85% of them, and the analyst has no way to tell.
//
// THIS IS THE BEAM-WIDTH DEFECT WITH THE FIELD CHOICE FIXED. There, a correct
// AVG was computed over a field nobody asked about. Here, a correct AVG is
// computed over the right field and a population the analyst was never told
// about. Both produce a number that looks entirely authoritative, and no
// verifier catches either, because nothing in the plan is malformed.
//
// A curated description carries the denominator ("Present on 263 of 309 retained
// observations"), which the MODEL reads. The ANALYST reads the answer. So the
// answer states it.
//
// DEFAULT OFF. With it off, no extra column is selected and the SQL for every
// aggregate plan is byte-identical to today.
const measureDenominatorEnv = "FORENSIC_MEASURE_DENOMINATOR"

func measureDenominatorEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(measureDenominatorEnv)), "true")
}

// measureDenominatorKey is the result key carrying how many rows actually
// contributed a value to a measure.
func measureDenominatorKey(measureID string) string {
	return measureID + "__value_count"
}

// measureNeedsDenominator reports a measure whose value depends on WHICH rows
// carried a value.
//
// COUNT is excluded because a count over a sparse field IS the denominator --
// "how many observations have a confidence" is answered by the number itself.
// COUNT_DISTINCT likewise. AVG, SUM, MIN and MAX all summarise a population and
// none of them names it.
func measureNeedsDenominator(measure SourceNativeMeasureV1) bool {
	if strings.TrimSpace(measure.FieldID) == "" {
		return false
	}
	switch strings.ToUpper(measure.Op) {
	case "AVG", "SUM", "MIN", "MAX":
		return true
	}
	return false
}

// measureDenominatorSparseThreshold is how complete a field must be before the
// answer stops qualifying itself.
//
// Not 100%: a single NULL in 8,642 CDR rows does not make "the average call
// duration" misleading, and a sentence appended to every answer would be noise
// that analysts learn to skip -- which is worse than silence, because it also
// devalues the qualifier where it matters. 95% is a judgement, stated here so it
// can be argued with rather than discovered in the code.
const measureDenominatorSparseThreshold = 0.95

// measureDenominatorNote returns the sentence an answer must carry when a
// measure covered materially less than its scope, or "" when it covered enough.
//
// The sentence names the POPULATION, not the field: the analyst already knows
// which quantity they asked for, and what they were not told is how many rows
// it came from.
func measureDenominatorNote(valueCount, scopeCount int64, populationNoun string) string {
	if valueCount <= 0 || scopeCount <= 0 || valueCount >= scopeCount {
		return ""
	}
	if float64(valueCount)/float64(scopeCount) >= measureDenominatorSparseThreshold {
		return ""
	}
	noun := strings.TrimSpace(populationNoun)
	if noun == "" {
		noun = "records"
	}
	return fmt.Sprintf("Computed over the %s of %s %s that carry a value; the remaining %s do not.",
		formatAnswerNumber(float64(valueCount)), formatAnswerNumber(float64(scopeCount)),
		noun, formatAnswerNumber(float64(scopeCount-valueCount)))
}
