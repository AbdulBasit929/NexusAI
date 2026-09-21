# Investigation Workspace final UX simplification handoff

CURRENT_BRANCH=codex/forensic-hybrid-checkpoint-20260723

CURRENT_HEAD=40717b83510c08db25dc26b9d6674bf46db363ac

UX_REVIEW_PREVIOUS=FAILED_TOO_COMPLEX

UX_SIMPLIFICATION=SOURCE_VALIDATED_ACTIVATION_PENDING_RAM

HEADER=PASS — case, one source count, Add data, and one workspace menu

EVIDENCE_SUMMARY=PASS — permanent strip removed; one compact count opens Evidence

CASE_CONTEXT_DUPLICATION=0

QUESTION_PRESENTATION=PASS_IN_SOURCE — plain conversation text; only the composer is editable

ANSWER_PRESENTATION=PASS — concise human answer, compact cardinality-aware facts, useful findings only

TECHNICAL_PROVENANCE=PASS — hidden behind More details / Evidence

EVIDENCE_DRAWER=PASS

COMPOSER=PASS — compact one-row composer with upward-arrow action

HISTORY=PASS — secondary workspace-menu drawer with retained results

ANPR_SINGLE_RESULT_ACCEPTANCE=PASS_IN_SOURCE — no technical block, duplicate fact, or one-row primary table; evidence immediate; one viewport

URDU_TRANSCRIPT_ACCEPTANCE=PASS_IN_SOURCE — natural RTL phrase visible; normalized target hidden; evidence immediate

RESPONSIVE_390=PASS

RESPONSIVE_820=PASS

RESPONSIVE_1024=PASS

RESPONSIVE_1440=PASS

PRIMARY_NAV_COMPLEXITY=MINIMAL

DUPLICATE_CASE_CONTEXT=0

PERMANENT_EVIDENCE_STRIP=REMOVED

QUESTION_DUPLICATE_INPUT_APPEARANCE=0_IN_SOURCE

PRIMARY_TECHNICAL_LABELS=0

DETERMINISTIC_FACT_LABELS_VISIBLE=0

NORMALIZED_INTERNAL_TARGET_VISIBLE=0

ONE_RESULT_GIANT_TABLE=0

PRIMARY_VISUAL_PROVENANCE_COMPLEXITY=0

NORMAL_SIMPLE_ANSWER_ONE_VIEWPORT=PASS

LIVE_CONSOLE_ERRORS=0_ON_CURRENT_SIMPLIFIED_OVERLAY; FINAL_OVERLAY_NOT_ACTIVATED

BACKEND_BEHAVIOR_PRESERVED=PASS

TESTS=PASS — 56 unit assertions passed, 1 explicit skip, focused ESLint zero errors, wrapper PASS

PLAYWRIGHT=PASS — 27/27 after the final submitted-question correction

PRODUCTION_BUILD=PASS — 687 modules

API_COMPILE=PASS — `go test -run "^$" ./core/http/...`

UI_ACTIVATION=PENDING_RAM — final attempt stopped before mutation at 4.859 GiB available; 6 GiB required

LIVE_BROWSER_ACCEPTANCE=PARTIAL_SUPERSEDED — current live image has the broad simplification and its Evidence/History drawers passed direct inspection, but the final plain-question visual correction remains sealed, not live

D_UNQUALIFIED_SOURCE_INCLUDED=NO

RETAINED_DATA_MUTATED=false

NX-B2.1D=OPEN

Q4_FULL_PLAN_ROLE=RETIRED

Q4_RESIDUAL_ROLE=INSUFFICIENT_FOR_RESIDUAL_ROLE

MODEL_REPLACEMENT=APPROVAL_REQUIRED

D_ACTIVATION=BLOCKED

FINAL_MANIFEST_SHA256=f272370da31b98163bbfd869d724298cab4bb565a2f6c77c51bb9f3e52628057

EXACT_NEXT_ACTION=Close applications manually until Windows reports at least 6 GiB available, then run `powershell -NoProfile -ExecutionPolicy Bypass -File scripts\activate_nexusai_ux_simplification_final_20260907.ps1`. The script revalidates the sealed manifest, protected services, zero active jobs, retained tuple, current base image, and RAM immediately before any mutation; it recreates only `api` and has exact-image automatic rollback.
