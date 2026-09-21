# Investigation Workspace Unified UI Capability Matrix

Date: 2026-09-07  
Canonical ordinary-user route: `/analyst`  
Product sequence: **Add evidence → Ask → Answer → Verify evidence**

This matrix describes what the ordinary analyst can rely on. It does not turn
UI affordances into analytical authority: readiness, authorization, query
routing, result completeness, and citations continue to come from the current
backend contracts.

| User-facing capability | State | What the analyst sees | Governing boundary |
|---|---|---|---|
| Universal file intake | AVAILABLE | One **Add data** action and drag/drop, with per-file outcomes | Existing governed upload endpoint; server classification and routing remain authoritative |
| Structured records (CDR, IPDR, subscriber, tower, ANPR tables and other admitted families) | AVAILABLE | Ready record counts, family-aware suggestions, typed findings and evidence links | Only current certified operations and authorized investigation scope |
| Document search | PROCESSING_DEPENDENT | Search suggestions and cited passages/pages only after document evidence is ready | No passage or page precision is invented when the backend does not return it |
| OCR | PROCESSING_DEPENDENT | Visible-text findings when a completed OCR artifact exists | A preserved source is not called ready merely because file metadata exists |
| Image / structured ANPR | LIMITED | Current admitted plate/text observations and source navigation | Detector confidence is not called accuracy; similarity is not identity probability |
| Video | LIMITED | Current admitted observations with frame/time locators where supplied | No unsupported playback segment or row-level precision is invented |
| Audio / transcript | PROCESSING_DEPENDENT | Transcript search and time references after transcript readiness | Technical audio metadata alone does not prove speech/transcript availability |
| Evidence citations | AVAILABLE | Human filename plus returned row/page/time/frame context; click opens evidence | `proof_role`, completeness, representative lineage and supported locators are preserved |
| Governed follow-up | AVAILABLE | Follow-up questions remain in the same investigation/source context | Context is supplied through the existing backend conversation/query contracts, not inferred analytically in React |
| Cross-source questions | LIMITED | Offered only when the ready source families and current capability metadata support the request | Unsupported compositions return clarification/limitation rather than fabricated success |
| Processing-state transparency | AVAILABLE | Ready, Processing, Needs attention and Failed remain distinct | Ready evidence can be queried while other evidence continues processing |
| English and exact multilingual identifiers | AVAILABLE | Direction-aware composer and evidence-first answers | Deterministic/certified paths remain authoritative |
| Urdu, Roman Urdu and mixed-language residual interpretation | UNAVAILABLE | No suggestion or success claim that depends on the retired residual path | `Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE`; clarification or a truthful limitation is required |
| Unqualified Q4 planner/model path | UNAVAILABLE | Not exposed as a certified analyst capability | `NX-B2.1D=OPEN`; `D_ACTIVATION=BLOCKED` |

## State semantics

- **AVAILABLE** — supported now when the user is authorized and the referenced
  evidence is ready.
- **LIMITED** — supported only for the current admitted operation/result
  contracts; the UI must not generalize beyond them.
- **PROCESSING_DEPENDENT** — supported only after the required derived artifact
  is complete.
- **UNAVAILABLE** — deliberately not presented as working; clarification or a
  limitation is the correct result.

