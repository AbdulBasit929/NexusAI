# Phase 1 · H2 — decision rule, WRITTEN BEFORE THE FIX

Date: 2026-09-25 · Owner: claude
State: golden 62 → 47 CORRECT · 14 CLARIFIED · 0 WRONG · held-out 13 → 7 WRONG.

## The defect, root-caused

H2 "How many VoLTE calls are there?" answered **"There are no CDR records
involving VOLTE"**. SQL says 739.

The plan is **correct**: `cdr.call_type EQ "VOLTE"`, a value the layer declares.
The executor then adds a SECOND predicate from `req.Target`
(`sourceNativeRequestConstraintsSQL`, `source_native_sql_executor.go:627`):

    (r.primary_target = 'VOLTE' OR r.secondary_target = 'VOLTE')

`primary_target`/`secondary_target` hold the record's SUBJECT — an MSISDN, a
plate, an account. A call type is never a subject, so the predicate matches zero
rows and the answer asserts absence.

**The system already knows this.** `classifyTargetType` (`query.go:8725`)
returns `"call_type"` for `GPRS`/`SMS`/`VOLTE`. The extraction is right; the
executor applies the target as a subject identifier regardless of its type.

**Why this is the worst defect class:** a verified plan producing a confident
false negative. An analyst told "there are no VoLTE calls" stops looking.

## The fix

Apply the subject predicate only when the target IS a subject identifier. A
target classified `call_type` is a FIELD VALUE — the plan already filters on it,
correctly, on the right column.

Narrow deliberately: an allowlist of known non-subject types (today only
`call_type`), so no question whose target is a phone, IP, email, identifier or
entity can change behaviour.

## Blast radius, before the run

Only questions whose extracted target is `GPRS`, `SMS` or `VOLTE`. Everything
else takes the identical code path.

## PRE-DECLARED THRESHOLDS — binding

- **Any question moves to WRONG on either suite → REVERT.** Eighth time this
  clause is written; it has fired twice and been honoured twice.
- **Any currently-CORRECT question is lost → REVERT.**
- **H2 converts to CORRECT (739), 0 lost, 0 new WRONG → KEEP.**
- **H2 does not convert but nothing regresses → KEEP ONLY IF** the executed SQL
  is shown to no longer carry the subject predicate; otherwise revert. A fix
  that does not change the SQL it was written to change is not a fix.

## What this does NOT fix

H1, H7, H12 are separate defects with separate causes and get their own slices.
This one change is measured alone.
