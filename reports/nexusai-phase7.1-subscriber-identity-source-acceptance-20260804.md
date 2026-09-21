# NexusAI Phase 7.1 Subscriber Identity Source Acceptance

Date: 2026-08-04  
Scope: first bounded Phase 7A vertical slice from the approved evidence-family
architecture, including its later runtime hardening activation.

## Executive verdict

The subscriber identity slice is source-accepted. It now has a dedicated
versioned adapter, eight registered adapter operations, six deterministic
analyst workflows, a privacy-aware specialist profile, typed enterprise
responses, direct Agent Chat routing, and an analyst-visible command surface.
It is runtime-accepted for discovery, deterministic routing, professional
clarification/no-data responses, Agent Chat, and the case UI. It is not yet
populated-data-accepted because the governed demo case has no subscriber rows.

Safe source preparation later added a hashed populated synthetic fixture and
all-six-operation oracle, and closed CNIC/name target echo plus canonical
API/export privacy gaps. These later changes are source-verified but not
deployed. Detailed evidence:
`reports/nexusai-phase7.1-populated-subscriber-source-readiness-20260804.md`.

This slice deliberately does not claim subscriber identity, ownership, current
control, SIM swap, fraud, association, or continuous device use. Exact facts
come from canonical source rows. The local chat model may explain a bounded fact
packet only when the analyst explicitly asks for narrative interpretation.

## Delivered capability

### Adapter and normalization

- Adapter: `nexusai.adapter.subscriber_identity` version `1.1.0`.
- Family: `subscriber_identity`; accepted record type: `subscriber`.
- Unicode NFKC names are retained as evidence with a normalized search key and
  detected script; Urdu/Arabic-script values are not silently transliterated.
- CNIC receives a masked representation for default output.
- Activation, deactivation, `valid_from`, and `valid_to` are parsed with the
  supplied source timezone.
- Inverted validity windows and deactivation-before-activation are explicit
  review flags rather than silently repaired data.
- The existing worker entry point delegates to the adapter so legacy ingest
  behavior remains compatible.

Registered adapter operations are schema detection, normalization, identity
lookup, validity timeline, device links, status summary, conflict audit, and
reuse-candidate review.

### Deterministic analyst operations

| Operation | Analyst outcome | Required target |
| --- | --- | --- |
| `subscriber.identity_lookup` | privacy-safe exact observation lookup | MSISDN, subscriber reference, IMSI, or IMEI |
| `subscriber.validity_timeline` | explicit activation/deactivation/validity observations | exact identifier |
| `subscriber.device_links` | explicit MSISDN/reference/IMSI/IMEI co-observations | exact identifier |
| `subscriber.status_summary` | row counts by supplied status and review state | none |
| `subscriber.conflict_audit` | stable identifiers with conflicting supplied attributes | none |
| `subscriber.reuse_candidates` | identifiers observed against multiple MSISDNs | none |

All SQL is hardcoded and parameterized. Target operations fail closed when the
identifier is missing. Aggregate results do not disclose full CNIC or names.
Row-bearing results preserve source file and row locators.

### Agent and model roles

`Subscriber_Identity_Analyst` is promoted to operational family slice version
`1.1.0`. Its prompt requires exact deterministic tools first, preserves Unicode
names, masks CNIC by default, distinguishes an observation from ownership, and
abstains from unsupported conclusions. The existing Qwen 4B chat model is the
optional bounded explanation role; Qwen embedding remains retrieval-only.

Exact subscriber questions take the direct forensic route with no synthesis
model. A model is selected only for explicit explain/interpret/summarize/brief
language, and deterministic response fields remain authoritative.

### Analyst UI

The normal governed case workspace now shows a Subscriber Identity Intelligence
command surface when subscriber rows and subscriber operations are both
available. It provides prepared exact lookup, status summary, conflict audit,
privacy guidance, and a direct specialist handoff. Typed status results render
an exact-value chart, result table, citations, limitations, and audit trace.

The browser test found and corrected an integration defect where these controls
were hidden in the embedded case route even though the subscriber specialist
link was available.

## Verification evidence

- Python adapter/contract suites: PASS, 27 tests.
- Forensic API Ginkgo suite: PASS, 206 registered specs; 204 passed and two
  intentionally skipped.
- Direct Agent Chat routing and governed empty-evidence formatter: PASS.
- Full agent package: 67 passed before 13 unrelated Windows testcontainer
  failures; those suites require unsupported rootless Docker on Windows. No
  Phase 7.1 direct-route failure remained after the focused test.
- React production build: PASS, 658 modules transformed.
- Focused Playwright Phase 7.1 workflow: PASS, one Chromium scenario using the
  installed local Chrome against the production bundle.
- Targeted ESLint: zero errors; five pre-existing unused-symbol warnings in the
  large Records page.
- JSON contract/profile parsing, Go formatting, Python compilation, and bounded
  diff checks: PASS.

## Deployment and data boundary

The later guarded rebuild activated this source and passed
`Phase6Activation=PASS`: API/worker/LocalAI builds were 19.8/4.1/375.4 seconds,
the root context was 128.95 MB, the LocalAI build succeeded on its first
attempt, 6.77 GiB was free at the gate, and rollback images/named volumes were
preserved. Five specialists are active and the collection catalog stayed at
28. No subscriber file was uploaded, evidence reprocessed, database row
changed, model downloaded, collection deleted, volume deleted, or source
staged/committed.

The first live acceptance needs an authorized or synthetic subscriber dataset.
Do not use a real CNIC in a screenshot, demo prompt, test fixture, or default
result. If a production role later needs full names or CNIC, implement a
separate field-level authorization and audited reveal workflow; do not weaken
the default response.

## Guarded activation — completed

The following retained runbook was used successfully. Do not repeat it merely
to reconfirm this slice; use it only after related source changes require a new
deployment:

```powershell
Set-Location 'C:\Users\sheik\Workspace\Office\Projects\NexusAI'

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File '.\scripts\build_deploy_forensic_phase6_gate.ps1'

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File '.\scripts\bootstrap_forensic_specialists.ps1' `
  -CollectionId 'records-demo-verified' `
  -TenantId 'default' `
  -Model 'qwen_qwen3-4b-instruct-2507'
```

The historical script name is retained for compatibility; the source it builds
includes Phase 7.1.

## Focused live acceptance

1. Verify LocalAI, forensic API, worker metrics, and NATS health.
2. Confirm adapter discovery reports five adapters and operation discovery
   reports 42 operations.
3. Confirm `Subscriber_Identity_Analyst` version `1.1.0`, Qwen chat model,
   empty multimodal model, case collection, and deterministic forensic tool.
4. In a collection with accepted subscriber rows, run all six operations.
5. Confirm a missing target clarifies rather than selecting a different query.
6. Confirm CNIC is masked, names are absent by default, and source locators are
   present on row results.
7. Run one explicit model explanation; compare every number and locator with
   the deterministic response and accept a bounded fallback as valid.
8. Exercise the case Analyze surface and Subscriber Specialist handoff at 390,
   820, 1024, and 1440 px; require no horizontal page overflow, broken assets,
   console errors, or failed first-party requests.
9. Verify keyboard focus for query, target, commands, audit details, and export.
10. Record row accounting and peak RAM before declaring the slice live.

## Next approved slice

Only after this focused live gate passes, implement Phase 7.2 tower/site
reference intelligence: time-aware site lookup, cell/site alias mapping,
coordinate validation, explicit source-bound joins, RF/coverage limitations,
a separate specialist profile, deterministic maps/tables, and provider-drift
goldens. Financial, access/security, and generic mapping follow in that order.
