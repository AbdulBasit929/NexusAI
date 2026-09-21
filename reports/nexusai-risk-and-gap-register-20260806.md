# NexusAI risk and gap register

Date: 2026-08-06  
Basis: source inspection, live APIs, container inventory, PostgreSQL schema, and browser acceptance

| ID | Risk or gap | Evidence | Impact | Priority | Safe next action | Approval gate |
| --- | --- | --- | --- | --- | --- | --- |
| R-01 | Working tree has 248 pre-existing changed/untracked paths | Git status: 56 modified, 192 untracked | accidental overwrite or incomplete handoff | critical | keep bounded diffs; never reset/stage | staging/commit requires approval |
| R-02 | Continuation file retains historical objectives below its controlling override | top block now names R2/R2-DES-01 and marks older blocks historical | future reader could ignore precedence | low | keep the controlling override and ledger synchronized | none for docs |
| R-03 | Running UI trails current source | deployed selector exposes four cleanup candidates; source shows one governed case | ordinary analysts can select non-case collections | high | guarded image refresh only after source reliability gate | full rebuild approval required |
| R-04 | Agent Chat lifecycle is not fully typed beyond the accepted historical baseline | later R6 requires cancel/retry/reconnect/concurrency envelopes | future Ask workflow could orphan or misroute requests | medium | defer bounded lifecycle slice to R6.2 after R5 | runtime activation approval required |
| R-05 | Reports are synchronous and not immutable | case manifest reports `reports: not_persisted` | weak report reproducibility | high | design report registry/version contract | migration approval later |
| R-06 | Multi-user/RBAC local profile is not live-accepted | local UI is accessible without user auth; sidecar key is enforced | enterprise isolation incomplete | critical | threat model and auth/RLS acceptance plan | retained config/migration approval |
| R-07 | Four agent-name cleanup collections remain | governed case endpoint hides four; deployed selector still shows them | scope confusion and duplicate KB risk | medium | retain and hide; execute manifest-backed cleanup only if approved | deletion approval required |
| R-08 | 64 backend families exist but only two model roles are approved | source matrix versus `/v1/models` | unsafe capability overexposure | high | role-based admin exposure and benchmarks | install/download approval required |
| R-09 | Document/audio/image/video families are mostly inventory/foundation | modality matrix and live catalog | upload may be mistaken for analytical support | high | explicit unavailable/manual-review UI states | model/backend approval later |
| R-10 | Financial/access data is populated without dedicated live specialists | case manifest plus catalog contract-only agents | weaker domain governance | medium | separate vertical slices with deterministic goldens | no runtime mutation yet |
| R-11 | `RecordsIntelligence.jsx` remains 2,905 lines | source line count | regression and ownership risk | medium | extract bounded presentation/query modules incrementally | none |
| R-12 | Deployed Case Workspace can briefly claim zero data while loading | deployed observation; R4-PRE-01 source fix and 5/5 regression pass | false capability statement until guarded refresh | high | preserve R4-PRE-01; deploy only with approval | rebuild approval required |
| R-13 | Local Playwright browser was missing | first focused run could not launch bundled Chromium | false test failure risk | low | use installed Chrome through explicit path; do not download | browser download requires approval |
| R-14 | Endpoint changes still require route-specific security/side-effect diligence | R1 now inventories all 371 route-file and 10 app registrations; global/group middleware is mapped | a future handler change could drift from feature/MCP/UI contracts | medium | enforce owning-phase route tests and parity checks | none for source tests |
| R-15 | R2 product decisions are unresolved | final assets, tagline placement, role/admin terms and branding/report configuration are open | R3 could encode inconsistent product assumptions | high | complete R2-DES-01 with product-owner decisions | product/source approval before R3 |

No retained state was changed to close any item in this register.
