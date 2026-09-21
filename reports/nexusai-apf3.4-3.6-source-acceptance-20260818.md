# NexusAI APF-3.4-3.6 source acceptance — 2026-08-18

## Decision

APF-3.4, APF-3.5 and APF-3.6 are source accepted. APF-3.7 is not started.
The source delta requires one guarded API/UI rebuild before live acceptance.

## Operation certification baseline

The machine-readable authority is
`api/forensic_records/contracts/operation-certification-v1.json`.

| Family | Operations | Independently certified | Correctness defects | Presentation defects | Blocked missing fixture |
| --- | ---: | ---: | ---: | ---: | ---: |
| ANPR vehicles | 7 | 0 | 0 known | 0 known | 7 |
| Case/cross-family | 21 | 0 | 0 known | 0 known | 21 |
| Communications CDR | 17 | 0 | 0 known | 0 known | 17 |
| Knowledge evidence | 1 | 0 | 0 known | 0 known | 1 |
| Network IPDR | 7 | 0 | 0 known | 0 known | 7 |
| Subscriber identity | 6 | 0 | 0 known | 0 known | 6 |
| Tower/location | 6 | 0 | 0 known | 0 known | 6 |
| **Total** | **65** | **0** | **0 known** | **0 known** | **65** |

Routing/parameter registry parity is 65/65. This is not computation
certification. Each blocked row needs an independent expected result, citation
and presentation oracle before its status can become certified.

## Issue priority and disposition

| ID | Priority | Finding | Disposition |
| --- | --- | --- | --- |
| ASK-SCROLL-01 | P1 | `scrollIntoView` moved the portal page/ancestors for long Ask history. | Fixed and browser-tested by scrolling only the transcript container. |
| ASK-PREFILL-01 | P1 | URL prompt remained durable and could re-own editor state. | Fixed: consume once with replace while preserving case parameters. |
| OPCERT-01 | P1 truth debt | 65 registered operations lacked independent per-operation result oracles. | Bounded honestly: 65-row ledger plus future registration gate; all rows remain blocked missing fixture. |
| LINT-BASELINE-01 | P2 | Full lint has six unrelated existing errors in `Chat.jsx`/coverage fixtures. | Recorded; changed files pass scoped lint. |
| GINKGO-DUAL-SUITE-01 | P2 harness | Unfiltered endpoint package invokes two Ginkgo `RunSpecs` suites. | 239 internal specs passed; affected suite rerun separately. Schedule harness consolidation outside this tranche. |

No known P0/P1 factual or authorization defect was waived.

## Accepted contracts

- `forensics.follow-up-context/v1`: field-level provenance, source turn and
  analysis, expiry, scope isolation, explicit-current-turn precedence.
- `forensics.language-assistance-proposal/v1`: strict JSON schema, allowlisted
  operations/families/output/parameters and no model-controlled scope.
- `forensics.knowledge-execution/v1`: authorized passage evidence plus
  chunk/hash/version/locator citation and truthful no-result abstention.
- `forensics.model-capability/v1`: role-based modality/output/artifact,
  runtime, maturity, hardware/resource, timeout, confidence and abstention.

The installed `qwen_qwen3-4b-instruct-2507` returned valid JSON-schema output
in a live non-retained probe. No model or data was downloaded.

## Verification

- complete `go test ./api/forensic_records -count=1`: pass;
- affected `go test ./core/services/agents -count=1`: pass;
- internal LocalAI endpoint Ginkgo: 239/239 specs passed in the combined run;
  the unfiltered package then hit the tracked dual-suite harness defect;
- focused Ask Playwright: 3/3 pass;
- changed UI source/tests scoped ESLint: pass;
- live model JSON-schema probe: pass in approximately four seconds;
- full lint: not clean because of six recorded unrelated baseline errors.

## Manual live UI deck after deployment

At 390, 820, 1024 and 1440 pixels, in light and dark themes:

1. Open `/analyst/ask?case=nexusai-forensic-demo`; verify page scroll remains
   fixed while the transcript owns scrolling.
2. Open a History item and Continue in Ask; verify the question is prefilled,
   `prompt` disappears from the URL, Back is stable, and no analysis runs.
3. Submit with Enter; verify Shift+Enter inserts a newline and the submitted
   turn becomes visible even after scrolling upward.
4. During a long answer, scroll upward; verify streaming does not force the
   transcript down and Jump to latest restores the edge.
5. Run an explicit follow-up, explicit override, standalone reset and missing
   target; verify inherited provenance/expiry and specific non-executing
   clarification.
6. Run canonical English and messy/Roman-Urdu/Urdu questions; verify canonical
   requests retain the deterministic fast path and unsafe model proposals never
   execute.
7. Verify History, citations/source reopening, workspace switching, console
   errors and horizontal overflow.

## Deployment boundary

No deployment, database migration, upload, reprocess, retained query, worker
rebuild, model/profile change, staging, commit or push occurred. Use the
existing guarded `scripts/build_deploy_nexusai_r8_ui_api_gate.ps1`; first run
its read-only preflight, then run it once without `-PreflightOnly` only after
operator approval. It preserves rollback images and named volumes and rebuilds
only the forensic API and LocalAI/UI services.
