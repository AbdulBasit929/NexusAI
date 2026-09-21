# NexusAI conversational investigation UI — final 2026-09-08

Status: `LIVE_VERIFIED`

The canonical `/analyst` workspace now presents a restrained, conversation-first
investigation experience while preserving the governed product sequence:
`Add data -> Ask -> Answer -> Evidence`. No backend contract, forensic datum,
citation target, Activity meaning, model-selection source, or D gate was changed.

## Final source overlay

- `core/http/react-ui/src/analyst/AnalystPortal.css` — premium light/dark visual
  tokens, 1120 px working lane, 880 px reading measure, question bubbles, answer
  cards, composer, header, drawers, intake dialog, focus, reduced-motion, RTL, and
  responsive behavior. SHA-256:
  `2194d81bb5b46ae1e36a8de99a6990a4ba3bff497f1a49233890285b037b0f09`.
- `core/http/react-ui/src/analyst/AnalystPortalLayout.jsx` — explicit source and
  attention grouping in the header. SHA-256:
  `937ea3d835f28bfa5c2acd74d7311d4d3c15373d0ab98d672e4f27c93845ad37`.
- `core/http/react-ui/src/pages/AgentChat.jsx` — human answer/clarification
  eyebrow and visible safe row/page/time/source locators in compact evidence.
  SHA-256: `7b3a15da1bc834ad097a15f2ff7531277191daf5018a9ad88e0cf0bc1b00ada8`.
- `core/http/react-ui/e2e/analyst-portal.spec.js` — question/answer visual
  contract, evidence locator visibility, 1600 px coverage, and drawer overflow
  regression. SHA-256:
  `eca9365330e5a69d9b24e0af16283d2853499909287ae3d88e0b7e23f79c7b11`.

The sealed source digest is
`17b49764f4e8d4225a18ab95b4577595d0ea84daa0e764c83e901cb1d2b89344`.
The 216-file UI-only deployment manifest SHA-256 is
`6f142a28622e430a26e1451ba191ce2735132a15d8ba186fbd6940a198707f25`.
`D_UNQUALIFIED_SOURCE_INCLUDED=NO`.

## Verification

- Focused ESLint: zero errors; existing warnings only.
- Node analyst tests: 50 passed, 1 intentional skip, 0 failed.
- Playwright analyst portal: 29 passed, 0 failed against the final live route.
- Responsive acceptance: 390, 820, 1024, 1440, and 1600 px.
- Production build: PASS, 687 modules.
- Unified UI wrapper test: PASS.
- `go test ./core/http/... -run '^$' -count=1`: PASS.
- Real browser: PASS for header, conversation, composer, exact evidence locators,
  Evidence drawer, History drawer, Add data dialog, Urdu/RTL content, light/dark
  themes, and absence of horizontal drawer overflow.

The production index SHA-256 is
`a0f1e4288c1b9ee46b4fa3c3c94870fef46b12d7c1718c36eee2e905d14ae7de`.

## Scope truth

`NX-B2.1D=OPEN`. The consumed Qwen3-8B selection12 resource failure is unchanged.
No final D candidate, holdout, qualification, or D activation was created.
`D_ACTIVATION=BLOCKED`.
