# Latency, measured properly — 2026-09-24

Harness: `scripts/nexusai_latency_bench.py`. Discards a warm-up, repeats N times, reports
min/median/max with the spread ratio, samples container load around every run, and prints the
UNACCOUNTED remainder. An unaccounted majority means the instrument is incomplete and no
tuning conclusion may be drawn from the run.

## The instrument is now complete

The planner audit is seeded at request entry in `query.go` before any planning, so cost
recording no longer depends on which path happens to create one.

    unaccounted, before:  74 - 154 s   (three of four probes reported ir_gen = 0)
    unaccounted, after:   0.1 - 0.2 s  on generating questions

99.8% of every slow request is explained, and it is **all IR generation**.

## Generation time varies up to 2.89x on identical input

    CDR-16    31.9s / 92.0s   spread 2.89x
    ANPR-04   66.3s / 73.7s   spread 1.11x

This is why single samples misled twice in one session — once toward "20-25% improvement",
once toward "most questions answer in 3-4 s". Neither survived repetition.

## The distribution is bimodal, and the two modes are 40x apart

20-question sample, one timed run each:

| | Questions | Median wall | IR generation |
|---|---:|---:|---|
| **Generate a plan** | 9 of 20 (**45%**) | **138.4 s** | essentially 100% of the time |
| **Deterministic only** | 11 of 20 (55%) | **3.4 s** | none |

    TXN-01 182.7s · IPDR-04 173.7s · SUB-01 165.5s · ACC-03 158.6s · ANPR-04 138.4s
    CDR-05 121.9s · CDR-16 104.3s · TWR-01 103.1s · CDR-02 95.4s
    ---
    CDR-01 4.7s · CDR-13 4.2s · ACC-01 4.2s · IPDR-01 4.1s · ANPR-01 3.9s · SUB-02 3.4s
    AUD-02 2.1s · X-01 1.7s · CASE-01 1.7s · DOC-02 1.2s · IMG-04 0.8s

    p95 across the sample: 173.7 s      P3 gate: p95 < 15 s

## What this means — it is a POLICY question, not a tuning one

The nine generating questions are not failures. **Seven of them are answers the ladder
already produces CORRECTLY** (CDR-02, CDR-16, SUB-01, TWR-01, IPDR-04, ACC-03, ANPR-04).
They generate because verified-only mode triggers arbitration to EARN a verified plan for an
answer that would otherwise be stated on no authority.

So the measured cost of the verification guarantee is:

    ~45% of questions pay ~138 s to convert an already-correct UNVERIFIED answer
    into a VERIFIED one.

That guarantee is what took confident-wrong from 23 to 0 and closed the Phase 1 gate.
Trading it away for latency is the exact trade this product refused three times already, and
it is the product owner's decision, not an engineering one.

## Options, with what each costs

1. **Plan cache keyed on question shape.** The same question deterministically produces the
   same plan; a normalized-question -> verified-plan cache makes a repeat question instant
   and preserves the guarantee completely. Highest value, no safety cost. Bounded by how
   much real analyst traffic repeats.
2. **Generate asynchronously.** Return the deterministic answer immediately, marked
   explicitly as not yet verified, and upgrade it when the plan lands. Preserves both
   latency and the guarantee, but introduces an answer that CHANGES after it is shown —
   which needs a UX contract decision, and the UX contract currently says the narrative may
   never block and may never be required.
3. **Reduce generation cost itself.** The prompt carries the issued field enum; a smaller
   enum is a shorter prompt. Bounded by retrieval quality — WI-7 showed that starving the
   enum makes plans worse.
4. **Narrow when arbitration fires.** Only arbitrate where the deterministic answer is
   actually at risk, rather than on every unverified structured answer. Requires a signal
   for "at risk" — and shape-mismatch was measured and REJECTED as that signal (3 wrong /
   2 CORRECT).

Not recommended: accepting unverified answers to save time. That is precisely the state the
Phase 1 work removed.

## Method note

Everything above is median-of-repeats with the spread stated. Container load was sampled
around every run; LocalAI reached 791-1539% CPU with memory healthy (4.0 of 7.6 GiB) and the
host at 23% of 16 logical cores, so this is inference cost, not resource starvation.
