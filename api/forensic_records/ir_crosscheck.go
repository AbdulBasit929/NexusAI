package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// CROSS-VERIFICATION.
//
// Two independent derivations of the same answer: the deterministic route
// (keyword ladder or a registered template) and the enum-constrained generated
// plan, which has passed S6, S9 SHAPE and CONSTRAINT_APPLIED. When they agree,
// the answer has corroboration no single path can give it. When they DISAGREE,
// one of them is wrong and nothing here can tell which -- so the honest move is
// to say so rather than pick.
//
// Measured on the 62 golden questions, 2026-09-22:
//
//	disagreements where the delivered answer was WRONG   : 12  -> abstain
//	disagreements where the delivered answer was CORRECT :  2  -> abstain
//
// 12 confident-wrong answers removed for 2 correct ones. That is the trade this
// product's rule asks for -- "abstention is a success and a confident wrong
// answer never is" -- and the Phase 1 gate says the same thing: Phase 1 fixes
// SAFETY, the compiler delivers COVERAGE.
//
// Two alternatives were measured FIRST and rejected:
//
//	override the deterministic answer on a shape mismatch : 3 wrong / 2 CORRECT
//	withhold every unverified structured answer           : 12 wrong / 9 CORRECT
//
// Both are worse than 12/2, and the first creates new confident-wrong answers.
//
// It compares EXECUTED VALUES, never plan shapes: a cheap plan-level proxy was
// measured at 5 caught / 3 correct lost, because a template can group its rows
// and still narrate the right total (CDR-02, SUB-01, TWR-02 all do).
const semanticIRCrossCheckEnv = "FORENSIC_IR_CROSSCHECK"

func semanticIRCrossCheckEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(semanticIRCrossCheckEnv)), "true")
}

// crossCheckOutcome is an AUDIT RECORD. "Why did the system decline to answer"
// has to be reconstructable months later, and "the two derivations disagreed"
// is a different event from "the compiler could not resolve the question".
type crossCheckOutcome struct {
	Ran        bool    `json:"ran"`
	Skipped    string  `json:"skipped,omitempty"`
	Agreed     bool    `json:"agreed,omitempty"`
	Determinis any     `json:"deterministic_value,omitempty"`
	Generated  any     `json:"generated_value,omitempty"`
	Delta      float64 `json:"delta,omitempty"`
}

// crossCheckValuesAgree compares two answers the way an analyst would: the same
// number is the same answer. Numeric comparison uses a relative tolerance so a
// float formatting difference is not mistaken for a disagreement; anything else
// is compared as trimmed, case-folded text.
func crossCheckValuesAgree(deterministic, generated any) bool {
	if deterministic == nil || generated == nil {
		return false
	}
	dNum, dOK := answerNumber(deterministic)
	gNum, gOK := answerNumber(generated)
	if dOK && gOK {
		if dNum == gNum {
			return true
		}
		scale := math.Max(math.Abs(dNum), math.Abs(gNum))
		if scale == 0 {
			return true
		}
		return math.Abs(dNum-gNum)/scale < 1e-9
	}
	if dOK != gOK {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(fmt.Sprint(deterministic)),
		strings.TrimSpace(fmt.Sprint(generated)))
}

// crossCheckAgainstGeneratedPlan re-derives the answer from the generated plan
// and reports whether the two agree.
//
// It runs only where a second opinion is both possible and meaningful:
//   - the delivered answer is NOT already plan-backed (a source-native answer
//     has been verified; comparing it with itself proves nothing),
//   - the question resolves to a structured family the algebra serves. Document,
//     image and audio questions have their own governed executors and no typed
//     plan exists to compare against -- withholding those would suppress a
//     competence this path does not have.
func crossCheckAgainstGeneratedPlan(ctx context.Context, cfg config, db *pgxpool.Pool,
	req hybridQueryRequest, resp hybridQueryResponse) crossCheckOutcome {
	if !semanticIRCrossCheckEnabled() {
		return crossCheckOutcome{Skipped: "disabled"}
	}
	// LOGICAL reasons before infrastructure ones: "this question has no second
	// opinion to give" is a permanent property worth recording, while a missing
	// database is a transient condition. Recording the transient one first would
	// make the audit say the wrong thing about why a check did not run.
	if req.SourceNative != nil {
		return crossCheckOutcome{Skipped: "already_plan_backed"}
	}
	family := semanticQuestionFamily(req.Query)
	if family == "" || !semanticDynamicFamilyAllowed(req, family) {
		return crossCheckOutcome{Skipped: "no_structured_family"}
	}
	if db == nil || cfg.LocalAIURL == "" {
		return crossCheckOutcome{Skipped: "unavailable"}
	}
	deterministic, ok := buildResultAnswer(req, resp, enterpriseDisplayRows(req, resp))
	if !ok || deterministic.Value == nil {
		// No single comparable value: a row listing or a narrative has nothing
		// to corroborate, and inventing a comparison would be worse than none.
		return crossCheckOutcome{Skipped: "no_comparable_value"}
	}
	generated, state := resolveSemanticIRFallback(ctx, cfg, req, "CROSSCHECK")
	if state != "semantic_ir_fallback" || generated.SourceNative == nil {
		return crossCheckOutcome{Skipped: "generator_" + state}
	}
	records, err := executeSourceNativePlanSQL(ctx, db, generated, generated.SourceNative, generated.SourceNativeCatalog)
	if err != nil {
		return crossCheckOutcome{Skipped: "second_execution_failed"}
	}
	second := hybridQueryResponse{Records: records, Answer: map[string]any{}}
	secondAnswer, ok := buildResultAnswer(generated, second, nil)
	if !ok || secondAnswer.Value == nil {
		return crossCheckOutcome{Skipped: "second_value_unavailable"}
	}
	out := crossCheckOutcome{Ran: true, Determinis: deterministic.Value, Generated: secondAnswer.Value,
		Agreed: crossCheckValuesAgree(deterministic.Value, secondAnswer.Value)}
	if d, okd := answerNumber(deterministic.Value); okd {
		if g, okg := answerNumber(secondAnswer.Value); okg {
			out.Delta = g - d
		}
	}
	return out
}

// VERIFIED-ONLY MODE.
//
// A structured analytical question is answered from a typed plan that passed S6,
// S9 SHAPE and CONSTRAINT_APPLIED — or it is not answered at all. The keyword
// ladder and the registered templates answer confidently and are checked by
// nothing, and that is where every confident-wrong answer comes from.
//
// Measured on the 62, 2026-09-22: withholds 12 wrong answers and 9 correct ones.
// A worse ratio than cross-verification's projected 12/2, but cross-verification
// turned out to be unable to judge template answers at all (they expose no
// single comparable value), so this is the mechanism that actually works.
//
// The 9 correct answers are the price of the Phase 1 gate, and the master
// prompt is explicit about that trade: "Phase 1 fixes SAFETY; the compiler
// delivers COVERAGE." They return as the IR path improves — each point of plan
// accuracy converts a clarification back into an answer, with no safety cost.
//
// Scoped to STRUCTURED families. Document, image and audio questions have their
// own governed executors and no typed plan exists for them; withholding those
// would suppress a competence this path does not have and does not judge.
const semanticVerifiedOnlyEnv = "FORENSIC_VERIFIED_ONLY"

func semanticVerifiedOnlyEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(semanticVerifiedOnlyEnv)), "true")
}

// verifiedOnlyWithholds reports that this answer would be stated on no
// authority. It is deliberately blind to whether the answer is right: that is
// the whole point — nothing at runtime knows, which is why an unverified route
// must not assert.
func verifiedOnlyWithholds(req hybridQueryRequest) bool {
	if !semanticVerifiedOnlyEnabled() || req.SourceNative != nil {
		return false
	}
	family := semanticQuestionFamily(req.Query)
	return family != "" && semanticDynamicFamilyAllowed(req, family)
}

// mediaArtifactNouns are the words that make a question about a DOCUMENT, an
// IMAGE or a RECORDING rather than about rows. Deliberately concrete nouns: a
// question is about a document when it says so.
var mediaArtifactNouns = []string{
	"document", "documents", "image", "images", "photo", "photos", "picture",
	"recording", "recordings", "audio", "transcript", "transcripts", "video",
	"guide", "notes", "pdf", "ocr",
}

// structuredTemplateAnsweringMediaQuestion reports a MISROUTE: the question
// names a media artifact and a structured analytical template answered it.
//
// Measured 2026-09-23: "What remote access setup does the HP guide describe?"
// was answered from `access_failed_events` because the word "access" matched the
// access-log family, and "Which image contains the text REF-2026-02?" was
// answered from `cross_family_correlation`. Both stated a confident wrong answer
// about evidence they never looked at.
//
// This is narrower than withholding every unverified structured answer, which
// measured 4 wrong removed against 3 CORRECT lost. Keyed on the artifact noun it
// removes 2 wrong and loses NOTHING: a question that says "document" is not a
// question about CDR rows, whatever keyword the ladder matched.
// EXEMPTION REMOVED 2026-09-24. This guard previously spared any answer backed
// by a typed plan, on the reasoning that a verified plan has earned the right
// to speak. DOC-01 is the counter-example: "Search the case documents for
// mentions of plate MN1367" compiled a perfectly sound ANPR plan -- correctly
// filtered on the plate -- and answered "There are no ANPR sightings involving
// MN1367". Sound arithmetic over the wrong evidence.
//
// **Verification proves a plan against ITSELF, never against the question it
// answers.** That is the same lesson DOC-04 taught through a different door,
// and it is why being plan-backed cannot buy an exemption here.
//
// Removing the exemption was MEASURED AND REJECTED once, at 1 wrong removed
// against 1 CORRECT lost -- the correct one being DOC-01 itself, which passed
// only because its gold expectation was `MN1367`, a string its wrong answer
// echoed back. With the v2 oracle that false CORRECT is gone and the same
// change now measures 1 removed, 0 lost. The fix did not change; the evidence
// about it did.
func structuredTemplateAnsweringMediaQuestion(req hybridQueryRequest, template string) bool {
	// A MEDIA ENTITY ANSWERING A MEDIA QUESTION IS THE POINT, NOT THE DEFECT.
	//
	// This guard exists because "What remote access setup does the HP guide
	// describe?" was answered from `access_failed_events` -- a STRUCTURED template
	// asserting something about a document it never read. That is still wrong and
	// still withheld.
	//
	// But the curated media entities read forensic.derived_artifacts, which IS the
	// image, the video and the audio. Measured 2026-09-26: this guard withheld
	// FIVE media questions that had routed correctly -- "how many text regions
	// were read from the images?" was told "you asked about images, but this
	// question matched a different kind of evidence". The evidence was exactly
	// right; the guard could not tell.
	//
	// So it stands down when the question resolves to a DERIVED family. The
	// narrowing is precise: a structured family naming a media noun is untouched.
	if family := mediaFamilyForQuestion(req.Query); family != "" {
		return false
	}
	lowered := strings.ToLower(req.Query)
	named := false
	for _, noun := range mediaArtifactNouns {
		if strings.Contains(lowered, noun) {
			named = true
			break
		}
	}
	if !named {
		return false
	}
	entry, ok := queryTemplateByName(template)
	return ok && semanticDynamicFamilyAllowed(req, entry.FamilyID)
}

// ---------------------------------------------------------------------------
// WI-10 -- the last five confident-wrong answers, all media/cross-family.
//
// Every guard below was scored OFFLINE against the 62 saved responses of
// reports/golden-ir-20260923-final BEFORE it was written, and two earlier
// candidates were REJECTED on those numbers. They are recorded here so nobody
// spends the afternoon re-deriving them:
//
//   - REJECTED: drop the `req.SourceNative != nil` exemption from
//     structuredTemplateAnsweringMediaQuestion, so a plan-backed answer to a
//     media question is withheld too. Fires on exactly 2 -- DOC-04 (WRONG) and
//     DOC-01 (CORRECT). A 1:1 trade that loses a correct answer. The real
//     discriminator is not "is it plan-backed" but "did the plan apply the
//     constraint the sentence claims" -- see answerNamesUnfilteredTarget.
//
//   - REJECTED: withhold when a quantity question resolves to a bounded record
//     listing (`presentation_type == "bounded_records_table"`). Fires on 15:
//     13 CORRECT, 2 WRONG. `canonical_records` legitimately answers most "how
//     many" questions. Shipping this would have destroyed 13 correct answers to
//     fix 2. The usable signal is only available AFTER execution -- whether the
//     answer builder computed anything at all.
// ---------------------------------------------------------------------------

// uncomputedAnswerMarker is set by enterpriseExecutiveAnswer when it falls all
// the way through to describing its own result set. See the comment there.
const uncomputedAnswerMarker = "executive_answer_uncomputed"

// quantityQuestionPhrases ask for a COUNT or a BREAKDOWN -- a question whose
// answer is a number, not a page of rows.
var quantityQuestionPhrases = []string{
	"how many", "number of", "count of", "for each", "total number", "per record type",
}

func quantityQuestion(question string) bool {
	lowered := strings.ToLower(question)
	for _, phrase := range quantityQuestionPhrases {
		if strings.Contains(lowered, phrase) {
			return true
		}
	}
	return false
}

// uncomputedQuantityAnswer reports that the analyst asked HOW MANY and the
// system answered with the size of the page it happened to return.
//
// Reaching the terminal fallback in enterpriseExecutiveAnswer means nothing
// computed an answer: buildResultAnswer produced no headline and
// records_summary was machinery. Stating the row count there presents a bounded
// page as if it were a finding.
//
// Measured 2026-09-23 across all 62: that sentence appears on exactly 2
// responses and BOTH are confident-wrong -- CASE-01 answers "20" to a
// per-record-type breakdown whose own payload reports total_count 12,912, and
// IMG-04 answers "20" where the true count is 22. No CORRECT answer reaches it.
// 2 removed, 0 lost.
func uncomputedQuantityAnswer(req hybridQueryRequest, resp hybridQueryResponse) bool {
	if resp.Answer == nil || resp.Answer[uncomputedAnswerMarker] != true {
		return false
	}
	return quantityQuestion(req.Query)
}

// answerNamesUnfilteredTarget reports a CONSTRAINT_APPLIED violation that
// reached the analyst: the executed plan filtered on nothing, and the sentence
// claims the result is "involving <target>".
//
// DOC-04 is the case. "What does the case notes document say about plate
// ABC-123?" extracted ABC-123 as a target identifier, compiled a bare
// COUNT over the ANPR family with `filters: []`, scanned all 218 rows and
// stated "There are 218 ANPR sightings involving ABC-123 in this case."
// The count is of everything; the sentence attributes it to one plate.
//
// The template-level `applied_filters.target` is NOT evidence that a filter
// ran: it is recorded from extraction. When a typed plan executes, that plan's
// filters are the only ground truth. DOC-01 is the control -- same template,
// same family, same bare COUNT shape, but its plan DOES carry
// `anpr.plate_number CONTAINS MN1367`, so it is untouched.
//
// Measured 2026-09-23 across the 33 plan-backed answers: fires on 1, DOC-04,
// which is WRONG. 1 removed, 0 lost.
func answerNamesUnfilteredTarget(req hybridQueryRequest) bool {
	if req.SourceNative == nil {
		return false
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		return false
	}
	for _, filter := range req.SourceNative.Filters {
		if strings.EqualFold(strings.TrimSpace(filter.Value), target) {
			return false
		}
		for _, value := range filter.Values {
			if strings.EqualFold(strings.TrimSpace(value), target) {
				return false
			}
		}
	}
	return true
}

// singleFamilyNegativeClaimsCaseScope reports a FALSE NEGATIVE: a template that
// searched ONE evidence family reporting absence across the whole case.
//
// X-01 is the case. "Where does 03001234567 appear across all evidence?" routed
// to `top_locations` -- declared `scope_mode: case_wide` while bound to the
// single family `communications_cdr` -- found no CDR location rows and stated
// "No matching records were found for 03001234567 in the selected case scope."
// The gold shows that number DOES appear in this case, in a PDF and a WAV. A
// confident "not found" is the worst answer a forensic product can give,
// because the analyst stops looking.
//
// The predicate needs no new heuristic: it falls out of the capability metadata
// the catalogue already declares. A template may assert case-wide absence only
// when its search was case-wide. Measured 2026-09-23: `case_wide` scope on a
// single non-cross family holds for exactly one template in the corpus, and
// DOC-06 -- the model of a correctly scoped negative, "in the searched current
// source scope" -- is `case_or_target` and untouched. 1 removed, 0 lost.
func singleFamilyNegativeClaimsCaseScope(resp hybridQueryResponse, entry queryTemplateCatalogEntry) bool {
	if entry.ScopeMode != "case_wide" {
		return false
	}
	if entry.FamilyID == "" || entry.FamilyID == "case_cross_family" {
		return false
	}
	state := strings.ToLower(strings.TrimSpace(stringValueAny(resp.Enterprise["result_state"])))
	return state == "no_match_for_filter"
}

// withholdPostExecution replaces a confident NON-ANSWER with a clarification.
//
// Both conditions it checks are only knowable after execution -- whether the
// answer builder computed anything, and whether a single-family search came
// back empty -- so this runs at the end of the records path rather than at the
// pre-execution gate that verified-only uses.
//
// It reports whether it withheld, so the caller can rebuild the payload.
func withholdPostExecution(req hybridQueryRequest, resp *hybridQueryResponse, entry queryTemplateCatalogEntry) bool {
	if resp == nil || resp.Intent == intentClarify {
		return false
	}
	var reason withholdReason
	switch {
	case uncomputedQuantityAnswer(req, *resp):
		reason = withholdReason{
			Code: withholdUncomputedQuantity,
			Question: "I can count this, but I did not compute a count here -- I only returned a " +
				"bounded page of rows, and its size is not the answer to your question. Tell me which " +
				"field to count and how you want it broken down, and I will compute it.",
			Guardrail: "No count was stated because none was computed: the result is a bounded page " +
				"of records, and reporting its size would present a page length as a finding.",
			Detail: "template=" + entry.Name,
		}
	case singleFamilyNegativeClaimsCaseScope(*resp, entry):
		reason = withholdReason{
			Code: withholdSingleFamilyNegative,
			Question: "I found nothing, but I only searched one kind of evidence, so I cannot tell you " +
				"it is absent from this case. Name the evidence you want searched -- or ask for a " +
				"cross-evidence search -- and I will look there.",
			Guardrail: "No absence was asserted because the search covered a single evidence family " +
				"while the question asks about the case. A negative finding may only be stated for the " +
				"scope actually searched.",
			Detail: "searched_family=" + familyOrNone(entry.FamilyID) + " template=" + entry.Name,
		}
	default:
		return false
	}
	applyWithhold(req, resp, reason)
	return true
}
