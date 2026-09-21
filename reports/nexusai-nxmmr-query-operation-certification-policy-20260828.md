# NX-MMR query-operation certification policy

## Product rule

An ordinary visible suggestion is a product promise. It may appear only when
its exact operation is `PRODUCT_CERTIFIED`. Queryable, registered, implemented,
fixture-tested, retained-artifact-present, or historically demonstrated are not
synonyms for product-certified.

## Certification levels

| Level | Meaning | Ordinary suggestion eligible? |
|---|---|---:|
| `REGISTERED` | operation contract exists and validates | no |
| `SOURCE_VALIDATED` | executor/source contract and bounded tests pass | no |
| `FIXTURE_CERTIFIED` | independent controlled oracle passes | no |
| `REAL_WORLD_CERTIFIED` | representative lawful real-world oracle passes | no |
| `PRODUCT_CERTIFIED` | real-world plus live integrated API/agent/UI/Activity/security/performance acceptance | yes |

`PRODUCT_CERTIFIED` also requires the existing overall certification verdict
to be `certified` or `certified_with_limitation`. A known correctness defect,
failed security gate, unbounded scope, self-declared oracle, no-data family, or
missing processor makes suggestion eligibility false.

## Current source ledger

The generated `forensics.operation-certification/v1` ledger has 79 entries:

- 62 `REGISTERED`;
- 12 `SOURCE_VALIDATED` (the NX-B1 family operations);
- one `FIXTURE_CERTIFIED` (`cdr.multi_source_comparison`);
- four `PRODUCT_CERTIFIED`.

The four currently suggestion-eligible operations are:

- `cdr.frequent_contacts`;
- `cdr.temporal_activity`;
- `forensics.evidence`;
- `forensics.evidence_package_summary`.

Capability-family suggestion materialization is even more conservative: only
certified prompts with an explicit prompt-to-operation mapping are emitted.
At present that is the two CDR prompts. Evidence/package operations remain
ledger-eligible but are not silently mapped to a family prompt.

## Independent oracle rule

The implementation team may register and source-validate an operation, but it
cannot certify its own output. Certification evidence must state the oracle
owner, fixture/dataset version, expected calculation, duplicate/null/sentinel/
time/tie rules, query variants, negative controls and review record. Model
outputs cannot label the same samples used to assess that model.

## Promotion path

1. Contract registration and plan validation.
2. Deterministic executor or explicitly typed observation/retrieval authority.
3. Security, scope, budget and provenance tests.
4. Independent fixture oracle.
5. Representative lawful real-world oracle.
6. Live API and agent routing equivalence.
7. Ask presentation, citations, source navigation and Activity reopen.
8. Positive, complete-zero, no-match, unavailable, processing and failure.
9. Four-viewport, Urdu/mixed-LTR, performance and rollback acceptance.
10. Ledger regeneration, review, then suggestion publication.

Demotion is immediate when a correctness/security defect or processor/model
unavailability invalidates the certified behavior. The operation may remain
explicitly callable with truthful limitations only if its criticality policy
allows it; its suggestion disappears.

## Source change and verification

The bounded NX-MMR source slice adds the five levels to
`operation_certification.go`, validates them, makes suggestion eligibility
require `PRODUCT_CERTIFIED`, regenerates the 79-entry JSON contract, and
filters capability suggestions through the certification ledger. Focused tests
for registration, independent oracle, risk/suggestion gate and capability
materialization pass (`go test`, 10.916 seconds). No deployment occurred, so
the running UI continues to reflect its older live capability payload until a
separately approved activation.

