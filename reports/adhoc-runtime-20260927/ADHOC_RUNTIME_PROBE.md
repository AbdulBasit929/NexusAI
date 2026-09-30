# AD-HOC RUNTIME PROBE — what happens when somebody types their own words

Measured 2026-09-27 against the shipped posture, through the analyst workspace proxy
(`127.0.0.1:4181/api` → forensic API `:8091`), actor role `user`.

**This is not an evaluation suite.** Fourteen questions written on the spot, deliberately not
copied from `golden_questions_v2`, `holdout_questions_v1` or `holdout_media_v1`, and phrased the
way a person asks rather than the way a template was written. Read-only: no data written, nothing
reprocessed, no suite run.

Raw responses: `adhoc_results.json`. Probe: kept with the session scratchpad.

---

## 1. THE INSTRUMENT FAILED FIRST, AND THAT IS THE HEADLINE CAVEAT

The first version of this probe read `answer.narrative.direct_answer` and reported **0 of 14
answered**. That was false. `direct_answer` is not at a fixed path in the payload, and the same
extractor reported the known-good control (`How many CDR records do we have in this case?` →
8,642) as empty too.

**Fourteenth oracle defect in this project's own instruments**, and it would have been reported as
a product failure. The probe now runs a **selftest against that control question first and refuses
to report a verdict unless it comes back ANSWERED containing 8,642**. Every number below is from
the post-selftest run.

## 2. RESULT

Fourteen questions. **Zero produced a correct, fully stated answer.**

| Outcome | n | Questions |
|---|---|---|
| Honest refusal — "could not map that request" | 5 | images-with-faces, list audio files, audio languages, `hello`, "what can this system do?" |
| Honest clarification — asked for the missing entity | 1 | "What happened on 15 June 2026?" → *"Which entity should I build the timeline for?"* |
| No general-knowledge answer | 1 | "What is a CDR?" → *"A bounded general definition is unavailable for that term."* |
| Thin but not wrong | 1 | "What can you tell me about the video evidence?" → *"Retrieved 1 cited evidence result."* |
| Partial — right rows, answer not stated | 2 | "Who did 923001110001 speak to most often?" → *"8 phone contacts ranked"* (never names who); "everything about plate ABC-123" → *"1 ANPR sighting matched"* |
| **Misleading — constraint dropped, count stated** | **4** | see §3 |

## 3. THE FOUR THAT MATTER — A DROPPED CONSTRAINT, ANSWERED RATHER THAN REFUSED

| Question | Stated answer | Plan |
|---|---|---|
| Show me all the calls that lasted longer than ten minutes | "20 CDR records matched this question." | `"filters": []` |
| Do any subscribers share the same handset? | "11 subscriber records matched this question." | `"filters": []` |
| Is there any link between the plate sightings and the call records? | "There are 750 ANPR sightings in this case." | `"filters": []`, `"project": []` |
| Summarise the suspicious activity in this case | "14 records matched this question." | `executive_answer_uncomputed` |

The duration filter, the relational condition and the correlation never reached the SQL. Each
returned a bounded row count and stated it as the answer.

**This is DEFECT 2 from `NEXUSAI_CONTINUATION.md` §7 in a place the fix does not reach.**
`FORENSIC_BOOLEAN_RESTRICTION` refuses a plan that ignores a curated **BOOLEAN** field on a
**derived** family. These are structured `canonical_records` plans carrying numeric and relational
constraints, and nothing guards them. As recorded then: *verification proves a plan against ITSELF,
never against the question* — so an empty filter list verifies perfectly.

## 4. CONTROLS — an empty filter list is not normal, and proves the reading

Two measured-CORRECT questions that DO carry a constraint were run through the same inspection.
Had they also shown `filters: []`, §3 would prove nothing.

    How many times was plate LHR-2026 seen?
      plan    filters: [{"field_id":"anpr.plate_number","op":"EQ","value":"LHR-2026"}]
      says    "There are 87 ANPR sightings involving LHR-2026 in this case."

    How many sightings involve plate QQQ-9999?
      plan    filters: [{"field_id":"anpr.plate_number","op":"CONTAINS","value":"QQQ-9999"}]
      says    "There are no ANPR sightings involving QQQ-9999 in this case."

**The tell is in the sentence.** A bound constraint is repeated back ("involving LHR-2026"). A
dropped one yields "N records matched this question", naming nothing.

## 5. WHAT THIS DOES AND DOES NOT SAY

- It does **not** contradict the 67 measured-CORRECT answers. Those are real and reproduce.
- It **does** say the curated set is the working surface, and free-form phrasing is not.
- It does **not** show fabricated evidence. No invented plate, sighting, transcript or identity
  appeared. The failure is a stated count for an unasked question, not a made-up fact.
- **Conversational roles (UX §9 A) are absent**, not merely thin: greeting, capability and concept
  questions all return a forensic error or a blank definition.

## 6. NOT DONE HERE, DELIBERATELY

No fix was attempted. Generalising the dropped-filter refusal from BOOLEAN-on-derived to any
curated field the question names is a behavioural change, and this project's rule is that such a
change is measured on all three suites against a threshold written beforehand. **An unmeasured
change the night before a demo is how 67 working answers get broken.**
