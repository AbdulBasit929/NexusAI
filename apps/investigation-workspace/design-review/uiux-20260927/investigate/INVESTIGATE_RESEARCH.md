# Investigate — research and implementation ruling

Date: 2026-09-27  
Slice: 4.6 only  
Authority: `docs/ux/NEXUSAI_PRODUCT_UX.md` and `docs/work/UIUX_RESEARCH_20260927.md`

## The problem measured in the current surface

The answer renderer is evidentially mature, but the working surface still reads
like a long generated document. The evidence-family scope sits above the thread,
the question that actually ran does not retain a visible submitted scope, local
history occupies the main reading path, and a long answer pushes the next
question far down the page. The screen contains the right objects but does not
make the analyst's loop obvious: set scope, ask, assess state, inspect result,
verify sources, continue.

This is not a generic chat problem. A forensic answer is a reviewable result
artifact. Chat bubbles, avatars, typing theatre and an assistant persona would
make the evidence hierarchy less clear and are rejected.

## Backend capability audit

The active request is `POST /query/hybrid`. The UI can truthfully send:

- `tenant_id`, the current collection as `collection_id`, the exact `query`, and
  an optional curated `record_type` selected by the evidence-family control;
- one abortable request at a time, with the HTTP request/correlation identifier
  preserved only for a genuine failed state;
- no percentage and no streaming state—the endpoint is synchronous.

The returned contract can supply distinct answered, zero, partial, processing,
clarification, unsupported and failed presentations; a curated data grid;
claim-level, openable locators; contribution groups; evidence strength;
limitations; bounded derivation language; and up to four server-provided
follow-ups. The UI must not manufacture any of those.

Browser-local drafts, visible turns, saved question history, labels, pins and
comparisons are conveniences, not server audit history. They will be labelled
accordingly. Changing family scope clears incompatible visible turns before a
new request can begin and never silently widens to all evidence.

`POST /reports/generate` exists, but its current Markdown names PostgreSQL,
templates, implementation routes and other internal mechanics forbidden by UX
§9. This slice will not expose or silently rewrite that response. A governed
analyst-report projection or corrected endpoint response is required before a
case-report action can be honest. The result table's existing client-side CSV
export remains available and accurately names its bounded scope.

## Online research translated into product decisions

### 1. Evidence and action stay in the investigation context

Microsoft Defender's incident workflow keeps the attack story, evidence,
entities and supported actions inside the incident so an analyst can review and
pivot without losing context. Its evidence items carry explicit verdict and
state, rather than relying on visual treatment alone. NexusAI applies that
principle as one continuous case-scoped workspace: the submitted question and
scope, state, finding, result, sources and next action remain one turn.

Source: https://learn.microsoft.com/en-us/defender-xdr/investigate-incidents

The Security Analyst Agent workflow also asks for a data source, asks a
clarifying question when the request is broad, then presents findings with
supporting evidence and next steps. NexusAI adopts the sequence but not the
agent framing: scope is explicit before submission, clarification is a careful
non-error state, and only returned evidence/follow-ups render.

Source: https://learn.microsoft.com/th-th/defender-xdr/security-analyst-agent

### 2. Provenance and limits are part of the result, not fine print

NIST AI RMF distinguishes transparency (what happened), explainability (how),
and interpretation in the user's context. It calls for documented knowledge
limits, human oversight and provenance sufficient to support review. Applied
here: state and coverage precede the prose finding; locators stay claim-level;
evidence strength is written as well as coloured; method is available on
demand; limitations are never hidden behind success styling.

Sources:

- https://airc.nist.gov/airmf-resources/airmf/3-sec-characteristics/
- https://airc.nist.gov/airmf-resources/airmf/5-sec-core/

Carbon's AI-label guidance reinforces consistent placement and layered
explainability. NexusAI adopts the layered principle (finding → sources →
method), but does **not** add an AI badge or model popover: product identity is
already clear, and UX §9 forbids model/backend details on analyst surfaces.

Sources:

- https://carbondesignsystem.com/guidelines/carbon-for-ai/
- https://carbondesignsystem.com/components/ai-label/usage/

### 3. Progressive disclosure must retain native semantics

WAI-ARIA's disclosure pattern requires a button, `aria-expanded`, predictable
Enter/Space operation, and an optional relationship to the controlled region.
The existing native `details` implementation for “How this was derived” meets
that interaction model and remains collapsed by default. Browser-local question
history also remains a native disclosure instead of becoming an always-open
secondary feed.

Source: https://www.w3.org/WAI/ARIA/apg/patterns/disclosure/

### 4. Asynchronous status is polite and focus is predictable

W3C's status-message technique specifies `role=status`/polite announcement for
non-interruptive updates. NexusAI keeps one polite live region around the thread,
announces the genuine checking state, and moves focus only to the returned turn
anchor requested by the analyst. A visible Stop control aborts the in-flight
browser request without inventing a server-side cancellation state.

Sources:

- https://www.w3.org/WAI/WCAG21/Techniques/aria/ARIA22
- https://www.w3.org/WAI/ARIA/apg/practices/keyboard-interface/

USWDS advises short, contextual status titles and distinguishes status alerts
from validation or destructive confirmation. Clarification and unsupported
states therefore remain calm and specific; only genuine request failure uses
the critical treatment.

Source: https://designsystem.digital.gov/components/alert/

## Implementation ruling

1. Build a desktop analysis workspace with a dominant thread and a narrow,
   sticky browser-history rail; collapse naturally to one column on smaller
   screens.
2. Keep the family scope in a bounded control bar and state that it persists in
   this browser and applies to the next question.
3. Record the exact submitted family label on every turn. Display turn number,
   submitted scope and an “Ask a variation” action in a compact question header.
4. Keep the composer in the main working column, sticky without obscuring
   content. While busy, expose Stop; describe clearing as clearing the visible
   thread, not deleting audit history.
5. Make the empty state teach the loop in three terse steps without inventing
   example findings or queries.
6. Retain the governed answer order. Tighten spacing and borders so state,
   finding, result, sources, method, limitations and supported follow-ups read
   as one analytical artifact rather than stacked cards.
7. Add a compact source-meaning key derived from strength labels actually
   present in the response. Colour is redundant; symbols and text carry the
   meaning.
8. Do not add a report action until `/reports/generate` has an analyst-safe
   projection. Do not add avatars, AI gradients, fake suggestions, streaming,
   percentage progress, or unsupported case actions.

## Verification for this slice

- answered, clarification, complete-zero, partial, unsupported and failed
  fixtures remain semantically distinct;
- exact `record_type` is sent and the submitted scope is visible on the turn;
- changing scope clears incompatible visible turns before another request;
- Stop aborts the active browser request and produces an explicit stopped turn;
- local history is labelled `This browser only`;
- claim markers, keyboard previews, source navigation, RTL/mixed-direction text,
  result export, comparison and server-provided follow-ups continue to work;
- both themes at 390 and 1440, plus 375px no page overflow;
- unit, ported, focused Playwright, full route sweep and production build pass.

## Measured outcome

The slice exposed and fixed two pre-existing functional defects: the absent
`scope` query parameter was continuously resetting every family selection to
`all`, so no selected `record_type` survived to submission; and clarification
branches never populated the visible decision trail. The focused contract now
proves exact `record_type: cdr`, frozen `Scope: CDR` turn context, clear-before-
switch behavior, browser-request cancellation, the exact three-line
clarification trail, and zero horizontal page overflow at 390px. The established
five-path Investigate suite and the two new slice tests pass, as do 77 unit, 46
ported, the production build, and 48 route/width observations.
