# OPEN ITEMS — known gaps and defect status

Extracted from `NEXUSAI_CONTINUATION.md` at its 300-line cap. Current as of
2026-09-25. The state file keeps the headline; this keeps the detail.

## Known gaps, characterised but not closed

- **The goal taxonomy has no `breakdown`** — built and measured NEUTRAL, switch off (§9).
  `aggregate` + no group hints resolves to shape `scalar`, so calling an explicit breakdown
  `aggregate` asserts it is a single number (ACC-02 went CORRECT → WRONG proving it). The
  `lookup` default means *"unrecognised"* and skips shape verification — **16 of 62 (26%)
  unclassified; 8 answer CORRECT with no S9 shape check.** A fix adds a breakdown goal with a
  grouping obligation, touching every consumer of `frame.Goal`.
- **NOTHING LOGS THE SWITCH STATE AT STARTUP**, and an env file that OMITS a switch is
  indistinguishable from one setting it false. Five settings — including `VERIFIED_ONLY` —
  ran at compose's `:-false` defaults for five hours and a golden suite was scored that way
  before it was noticed. Fixed in the env file; a startup line naming every gate is still
  owed. [`CONFIG_DRIFT.md`](reports/config-drift-20260925/CONFIG_DRIFT.md).
- **Grammar failure is silent** in the same way. If generation fails server-side, `chat.go`
  logs `Failed generating grammar` and **proceeds with NO grammar at all** — every constraint
  lost, 200 response. Same startup assertion would cover it.
- **Without the plan cache a completion can cross the 240s client ceiling** and score as an
  ERROR that reads like a regression. A timeout is never a verdict (§1).
- **Distinct answers do not name what was counted** ("the count across CDR records is 10" when
  there are 8,642) · **`ir_shadow_*` is shadow-mode only**, so `withhold_detail`'s
  `issued_fields=0` means "not recorded", not "empty enum".

---

## Defect status

**FIXED AND CONFIRMED:** D3 HTTP 500s (0/62) · D2 spurious GROUP BY · Fact Packets carried
plumbing. **FIXED AT THE COMPILER, STILL LIVE ON THE LADDER** (27/62 bypass them): **D1**
unbound literals dropped · **D4** cross-family misrouting.

**OPEN:** D8 payload filters discarded unless the template is `canonical_records`
(`query.go:7728`) · D7 "Explain …" treated as a dictionary definition · 3 `no_family`
structured gaps (CDR-07, CDR-10, ANPR-03).

**Oracle defects found (eight):** filename checks scoring a coverage listing (DOC-01,
AUD-01/03) · CDR-13 expecting raw values the UX contract forbids · CDR-08/ANPR-05 `gold: null`
notes that expired when COUNT_DISTINCT shipped · held-out H8/H11/H12 (§1). **Read every
expectation against the analyst-facing text.** SUB-03's *scoring* gold is correct (masked);
only `scripts/ir_spike/gold_plans.json` holds the raw CNIC.

---

