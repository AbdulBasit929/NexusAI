# PHASE 1 RESULT — real-world questions, 2026-09-25

Full detail. The state file
[`NEXUSAI_CONTINUATION.md`](../../NEXUSAI_CONTINUATION.md) carries the summary and
the standing rules; everything below is the evidence behind them.

## PHASE 1 — real-world questions: 7 WRONG -> 1, 2026-09-25

Held-out set (SQL-verified, phrasings nobody wrote for this system). Golden is a REGRESSION
check only. Reconciliation: [`RECONCILIATION_20260925.md`](docs/architecture/RECONCILIATION_20260925.md).

    start             5 CORRECT · 0 CLARIFIED · 7 WRONG
    after Phase 1    10 CORRECT · 2 CLARIFIED · 1 WRONG

**Backend fixes, each measured on both suites:**

    H2  the executor applied ANY extracted target as a SUBJECT predicate
        (`primary_target='VOLTE'`) although classifyTargetType already said "call_type".
        A verified plan over 739 rows returned 0 -- a FALSE NEGATIVE.
    H1  "different handsets" counted rows, not distinct IMEIs. "different" implies distinct
        ONLY without a superlative; unguarded it captures "the MOST different people".
    H7  multi-word declared synonyms never bound ("server errorS" vs "server error") --
        this matcher compared raw words while everything else stems. Then an S9 RESTRICTION
        OBLIGATION refused the unfiltered plan and arbitration EARNED one that filters.
    H4  "What is the largest network volume?" was classified GENERAL_DOMAIN_KNOWLEDGE -- a
        DEFINITION request -- and never reached the compiler. A definition request never
        asks for a maximum.

**Two S9 obligations, each with a control MEASUREMENT supplied and intuition would have
missed:** an `avg`/`sum` question cannot be answered by COUNT (restricting to avg/sum is what
saves the FOUR correct rankings where `max` comes from "most"); a value named through a
MULTI-WORD curated synonym that the plan never filters on is a dropped restriction
(single-word matches would have refused SEVEN correct answers, because `CALL` is declared by
any question containing "call").

**Curation (Codex WI-LAYER-1), verified independently:** demo coverage 81/94 -> **93/94
(99%)**, cdr 11/16 -> 16/16. Only `TAC` excluded, its meaning not recoverable from the export.

**THREE ORACLE DEFECTS IN OUR OWN HELD-OUT SET, all found and corrected today:**

    H8   expected 5 active subscribers; 6 is CORRECT. `subscriber.status` lives under TWO
         headers (`status`, `account_status`) and the layer declares both, so the executor
         COALESCEs to 6. My SQL read one header.
    H12  written as an honesty probe when Beam Width was uncurated. It is curated now and
         holds 65/45/60/90, so the honest answer is 65 -- the system clarifies, which is
         SAFE but no longer sufficient.
    H11  same: `cdr.location` is curated now, so a LOCATION is the honest answer.

**STANDING RULE: when curation lands, re-derive every expectation it touches.** Otherwise the
set congratulates the system for abstaining from questions it can now answer. Derive an
expectation from the SAME expression the executor uses, not the first column that looks right.

**GOLDEN REGRESSION, combined curation + H4 deploy: CLEAN.** 47 CORRECT / 14 CLARIFIED /
1 MANUAL, identical to the pre-deploy baseline. One verdict appeared to move — CDR-14
CORRECT -> ERROR — and it was a **240s client TIMEOUT, not a wrong answer**. The curation
changed every payload hash, so the plan cache was empty and the whole run paid cold-completion
cost (NEG-01 176s, CASE-04 148s, CDR-04 145s, all still CORRECT). Re-measured alone: 181s cold
CORRECT, then 7.8s cached CORRECT against a 5.4s baseline.

**PROTOCOL, paid for twice now (IPDR-05/ANPR-06, then CDR-14): after any deploy that changes
payloads, run a WARM-UP pass before the SCORING pass.** A cold cache can cross the 240s ceiling
and manufacture an ERROR that looks like a regression. Never revert on a first post-deploy run
without re-measuring the mover in steady state.

**The one remaining WRONG — and curation was never its problem.** H11 asks WHERE and the plan
returns a COUNT (447, itself the wrong-field count: `originating_number`, not `msisdn`). That
is the S9 `rows`-shape gap, deliberately NOT closed: a `rows` rule would refuse the 8 questions
answering correctly under the mis-assigned `lookup` goal. **Breakdown taxonomy first.**

