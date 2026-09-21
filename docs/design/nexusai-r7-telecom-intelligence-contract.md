# NexusAI R7 Pakistan telecom intelligence contract

Status: complete; R7.1-R7.10 accepted at governed source/data/runtime boundaries  
Date: 2026-08-12  
Scope: communications CDR, subscriber reference, tower/site reference and their
deterministic, time-valid relationships

## Purpose

R7 turns accepted structured telecom foundations into a mature Pakistan-ready
vertical. It reuses the operational CDR, subscriber and tower adapters, public
forensic query route, specialist profiles, provenance model and Ask NexusAI
presentation contract. It does not introduce a new LLM or make an LLM the
authority for identifiers, timestamps, joins, counts, coordinates or citations.

## Evidence boundaries

- Preserve raw MSISDN, IMSI, IMEI, provider, site, sector, LAC/TAC, cell,
  coordinate, datum, uncertainty and timestamp values.
- Add canonical/search keys only when transformation is deterministic and retain
  the original value beside them.
- A CDR-to-reference join provides supplied tower context at an event time. It
  does not establish RF coverage, handset position, subscriber presence, home,
  route, ownership, association or continuous movement.
- Missing validity bounds remain unknown. Recency does not establish current
  operational truth.
- Protected real CDR remains read-only and outside repository fixtures. Ingest,
  migration or retained-data validation requires explicit approval and case,
  retention and access metadata.

## Time-valid tower join contract

Input:

- authorized `tenant_id` and `collection_id`;
- one exact cell/site target;
- optional inclusive start and exclusive end timestamps;
- bounded limit and offset.

For every matching CDR observation the operation returns:

- CDR timestamp and source-row provenance;
- the exact CDR cell/site identifier;
- `reference_candidate_count`: all exact supplied references for the identifier;
- `eligible_reference_count`: references whose supplied validity window contains
  the CDR timestamp;
- one deterministic display reference: the latest eligible validity start, with
  stable source/row tie-breaking;
- tower/site fields, coordinates, datum, uncertainty and reference provenance;
- `match_status`, one of:
  - `matched`;
  - `ambiguous_overlapping_references`;
  - `unmatched_no_reference`;
  - `unmatched_outside_validity_window`.

An overlapping reference set is never silently reconciled. A deterministic row
may be displayed for inspection, but its candidate count and ambiguous status
remain visible. Unmatched CDR observations remain in the result.

## Reuse classification

| Capability | Decision | R7 disposition |
| --- | --- | --- |
| CDR adapter and legacy table | reused and extended non-breakingly | adapter 1.2 adds role-separated raw/canonical identifiers and timezone/quality provenance |
| Subscriber adapter and six operations | reused and extended non-breakingly | adapter 1.2 adds SIM/device/service roles, validity status and observation-only association semantics |
| Tower adapter and six operations | reused and extended non-breakingly | adapter 1.2 preserves provider/site/sector history, validity basis, datum/uncertainty and overlap review |
| `POST /api/records/forensic/query` | reuse unchanged | publish deeper result fields through the existing contract |
| Specialist agents | reuse profiles | refine prompts only when a deterministic semantic changes |
| Qwen explanation model | reuse unchanged | no new telecom model is required |
| Generic result grid | reuse for R7.1 | add telecom map/timeline presentation in R7.6 |
| Real CDR | blocked for retained ingest | metadata/read-only audit only until explicit approval |

## R7 internal slices

1. **R7.1 — entry reconciliation and exact join outcomes:** inventory accepted
   capabilities; expose time-valid matched, ambiguous and unmatched states.
2. **R7.2 — Pakistan CDR normalization:** raw/canonical phone roles, timezone
   provenance, provider schema aliases, sentinels and rejection semantics.
   **Source accepted:** `cdr-canonical/v2` is mirrored additively while legacy
   table columns and duplicate hashes remain compatible.
3. **R7.3 — subscriber/device/service semantics:** prevent role collapse and add
   explicit validity/conflict behavior. **Source accepted:** subscriber,
   SIM/ICCID/IMSI, device/IMEI and service/provider roles remain separate cited
   co-observations with explicit validity and no ownership inference.
4. **R7.4 — tower reference history:** provider/sector aliases, time windows,
   coordinate/datum uncertainty and overlap review. **Source accepted:** tower
   adapter 1.2 uses a provider/site/sector history key, preserves reference
   version and coordinate provenance, and exposes half-open validity overlaps.
5. **R7.5 — governed telecom operations and real-world language:** family query
   variants, missing parameters, invalid/no-result behavior and composition.
   **Source accepted:** the existing 65-operation catalog remains stable while
   realistic tower-history, ICCID/service and service/device language routes to
   deterministic operations; clarification and bounded-negative-result
   contracts are exercised.
6. **R7.6 — telecom presentation:** synchronized tables, timeline, uncertainty-
   aware map and evidence drawer using the NexusAI visual system. **Source
   accepted:** Ask NexusAI renders the typed visualization spec rather than a
   label-only placeholder; selections synchronize timeline/map/evidence detail,
   and missing coordinates produce a truthful empty state.
7. **R7.7 — specialist and Ask integration:** typed telecom results and safe
   explanation/fallback behavior. **Source accepted:** catalog/profile versions
   are reconciled at 1.2 for the CDR, subscriber and tower specialists and the
   primary analyst exposes their governed operation surface. Scope remains
   request/case bound, peer delegation remains prohibited, exact operations stay
   authoritative and explanation remains optional/fail-closed.
8. **R7.8 — protected CDR validation:** separately approved read-only audit and,
   only if explicitly authorized, retained case ingest. **Read-only accepted:**
   the supplied protected CDR passed schema, accounting, normalization,
   role/timezone provenance, locator, classification and performance checks with
   zero raw disclosure or retained mutation. Retained ingest was unnecessary.
9. **R7.9 — performance, security and provenance:** large-case bounds, tenant/case
   isolation and source-locator verification. **Source accepted:** hard caps,
   stable scoped pagination, CDR/reference locators and bounded nested
   presentation values are exercised.
10. **R7.10 — runtime/manual acceptance:** guarded deployment, product-owner test
    pack, concise team-lead demonstration and closure. **Live accepted:** full
    activation, six profiles, deterministic/model-assisted matrices and
    synchronized desktop/mobile telecom UI pass; R7 phase closure still awaits
    the independently governed R7.8 data gate.

## R7 exit gate

R7 closes only when synthetic goldens and approved protected-data validation
agree; all exact joins retain citations; unmatched and ambiguous reference states
are visible; the map/timeline preserves uncertainty; no RF-presence or invented-
route claim is possible; and the accepted source is separately runtime accepted.
