# NexusAI APF-3 Query-Truth Certification — 2026-08-18

> Superseded for source-closure status by
> `reports/nexusai-apf3-final-source-closure-20260818.md`. This report remains
> the historical APF-3.4–3.6 live-acceptance record; its former strict
> APF-3.7 entry gate is replaced by the subsequently authorized risk-tier gate.

## Executive verdict

- `APF-3.7Ready: NO`
- `DeploymentNeeded: NO`
- `P0Open: 0`
- The exact APF-3.4 follow-up chain, deterministic calculation oracles and KB
  retrieval are live-accepted.
- APF-3.4, APF-3.5 and APF-3.6 are live accepted.
- APF-3.7 remains blocked because 63 operations lack independent full
  certification; registry parity is not an analytical oracle.

## Runtime baseline

The accepted live baseline returned HTTP 200 from LocalAI `/readyz`, forensic
records `/healthz` and the Analyst portal route. The latest guarded activation
marker was:

```text
R8UIAPIActivation=PASS APIBuildSeconds=19.5 LocalAIBuildSeconds=49.5 WorkerRebuilt=false ModelsChanged=false ProfilesChanged=false VolumesPreserved=true RollbackImages=preserved
```

The final source corrections are deployed. No worker rebuild, model/profile
change or retained-data mutation occurred.

## Live acceptance matrix

| Gate | Live observation | Result |
|---|---|---|
| APF-3.4 turn 1 | `Who did 923001110001 contact most?` returned 8 ranked contacts | PASS |
| APF-3.4 turn 2 | `Only outgoing.` retained operation/target and returned counts 68, 67, 61, 61, 52, 51, 49, 38 | PASS |
| APF-3.4 turn 3 | `For 923001234567 instead.` retained OUTGOING, replaced the target and executed without clarification | PASS |
| Temporal oracle | Exact UTC-day query returned 4 matched, 4 nocturnal, 2 non-zero, min 10, max 45 and average 27.50 seconds | PASS |
| Deterministic latency | Fresh records queries completed in about 0.1–0.5 seconds with no language assistance | PASS |
| Assisted language | Misspelled CDR activity invoked the installed schema-bound model; 43.7 seconds, validated proposal, exact target/day, no invented direction, 4 rows | PASS |
| Multilingual equivalence | English/Roman Urdu/Urdu named-month forms completed in 300/115/104 ms with identical semantics | PASS |
| KB retrieval | `evidence` used `kb_rag` and returned 3 cited matches | PASS |
| Hybrid summary | `evidence_package_summary` returned `records_sql + kb_rag`, 17 deterministic rows and 3 KB results | PASS |
| Hybrid brief | `executive_case_brief` returned the same governed hybrid composition | PASS |
| Aggregate lineage | Frequent-contact and temporal results display complete bounded contribution lineage | PASS |
| Temporal presentation | All 12 fields render, including 10/45-second extremes and source row locators | PASS |
| Ask/History navigation | Four completed analyses retained; Continue prefills without submitting; Shift+Enter creates a newline; browser Back returns to scoped Ask | PASS |

## Independent truth oracles

`cdr.frequent_contacts` was independently recalculated with read-only grouped
SQL over deployed `forensic.cdr_records`; production query helpers were not
called. For target `923001110001`, direction OUTGOING, the ranked counts are
68, 67, 61, 61, 52, 51, 49 and 38, with exact top counterparties and first/last
timestamps.

`cdr.temporal_activity` was independently recalculated for target
`923001234567` over `[2026-07-10T00:00:00Z, 2026-07-11T00:00:00Z)`: 4 matched
events, 4 nocturnal events, 2 non-zero durations, minimum 10, maximum 45 and
average 27.50 seconds.

The checked-in read-only lineage oracle is
`reports/apf3-contribution-lineage-live-check.sql`. Its live result proves:

| Operation | Evidence/version | Source | Contribution |
|---|---|---|---|
| frequent contacts top group | `ddd78d1b…` / `60756c7e…` | `seed_cdr_large.csv` | 68 rows, bounded row range, 64-char digest |
| temporal activity | `4320772b…` / `ad4b97c9…` | `pakistan_cdr_messy_synthetic.csv` | 4 rows, rows 2–5, 64-char digest |

The runtime contract is `forensics.contribution-lineage/v1`, caps groups at 50,
uses evidence/version/source identity, contribution counts, bounded row ranges
and digests, and marks incomplete identity fail-closed.

## Query-variant ledger

The generated artifact is
`api/forensic_records/contracts/query-variant-ledger-v1.json`.

| Measure | Count |
|---|---:|
| accepted occurrences | 155 |
| unique variants | 143 |
| duplicates | 12 |
| canonical occurrences | 65 |
| additional unique variants | 78 |
| follow-ups | 8 |
| clarifications | 4 |
| English / mixed / Roman Urdu / Urdu occurrences | 133 / 2 / 12 / 8 |
| routing / parameter / semantic passes | 155 / 155 / 155 |
| failures | 0 |

This corrects the earlier historical 141/11 count and retains the three
live-accepted APF-3.5 language-equivalence phrases. A drift test regenerates
the ledger from accepted source corpora and fails if the checked-in artifact
drifts.

## Operation-certification ledger

| Dimension | Result |
|---|---:|
| executable operations | 65 |
| registry/routing contracts | 65/65 |
| parameter registry | 65/65 |
| independent calculation passes | 2/65 |
| presentation passes in source | 2/65 |
| live citation passes | 2/65 |
| `certified` | 2 |
| `blocked_missing_fixture` | 63 |
| fully certified | 2 |

The two independently proven CDR operations are fully certified. The other 63
require genuinely independent family fixtures/oracles; registry and routing
parity do not imply analytical certification.

## Source corrections completed

- Deterministic records no longer invoke bounded model synthesis; hybrid intent
  remains eligible for synthesis.
- Governed records execution explicitly dispatches deterministic and hybrid
  capabilities instead of sending hybrid operations to the deterministic-only
  executor.
- Frequent-contact and temporal aggregates now include capped contribution
  lineage and incomplete-lineage limitations/provenance.
- Temporal presentation supports all 12 calculation columns instead of the
  generic seven-column cap.
- Explicit temporal CDR wording routes to `cdr.temporal_activity`.
- Target-replacement follow-ups recognize the accepted replacement phrases.
- Assisted-language family ownership is server-derived; query-relevant schemas
  restrict operation choices and prevent invented target/direction filters.
- English and Urdu named months are parsed deterministically with exclusive UTC
  day bounds.

## Issue disposition

| Severity | Finding | Disposition |
|---|---|---|
| P0 | None found | closed |
| P1 | Deterministic queries invoke slow synthesis | closed live |
| P1 | Hybrid templates enter deterministic executor | closed live |
| P1 | Aggregate contribution lineage absent | closed live and independently proven |
| P1 | Temporal calculation columns hidden | closed live |
| P1 | Assisted-language model timeout/invalid filters | closed with query-bounded schema and deterministic multilingual parsing |
| P2 | 63 operations lack independent goldens | controlled certification debt |
| P2 | Some deterministic explanatory prose remains generic | backlog after truth gates |
| P3 | UI lint/build emit existing warnings and large-chunk notices | non-blocking |

## Verification

- `go test ./api/forensic_records -count=1` — PASS.
- Focused forensic presentation Ginkgo contract — PASS.
- Scoped `AgentChat.jsx` ESLint — 0 errors, 18 existing warnings.
- React production build — PASS, 677 modules.
- Full agent package — 99/112 specs passed; 13 failures require rootless
  Docker/testcontainers support unavailable on this Windows host. The focused
  changed presentation contract passes.

## Final gate

Guarded activation and bounded live re-acceptance succeeded. The Analyst portal
is open at `http://localhost:8080/analyst/ask?case=nexusai-forensic-demo`.
`DeploymentNeeded=NO`.

`APF-3.7Ready=NO` remains mandatory because 63 operations are still
`blocked_missing_fixture`. The next work is independent operation
certification, not another rebuild or a waiver of unknown correctness.
