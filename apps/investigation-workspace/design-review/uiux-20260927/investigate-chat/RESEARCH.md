# Investigate conversational redesign — research ruling

Date: 2026-09-28  
Scope: Investigate layout and interaction only  
Authority: `docs/ux/NEXUSAI_PRODUCT_UX.md`

## Measured problem

The existing surface showed seven competing frames before the analyst could
complete the core loop: scope, question, answer state, finding, result, citation
rail, browser history and composer. Browser-local history was also duplicated in
the persistent case rail and in the page. The result was functionally complete
but visually read as a dashboard containing a form rather than a coherent
conversation.

## Primary-source research

- OpenAI describes ChatGPT as a multi-turn natural-language interface that keeps
  context across turns. Its search experience keeps citations inline, lets a
  user hover for a preview and opens the source for verification. NexusAI adopts
  the quiet turn-taking, persistent composer and immediate source inspection—not
  the generic assistant persona or unconstrained response shape.
  - https://help.openai.com/en/articles/9260256-chatgpt-capabilities-overview
  - https://help.openai.com/en/articles/9237897-searching-the-web-with-chatgpt
- WAI-ARIA defines `role="log"` for meaningful sequential additions such as a
  chat history, with polite announcement by default. The transcript is therefore
  one ordered log; individual results do not compete as separate live regions.
  - https://www.w3.org/WAI/WCAG22/Techniques/aria/ARIA23.html
  - https://www.w3.org/TR/wai-aria-1.2/#log
- IBM's conversational design guidance says a successful interface must have a
  focused purpose, preserve topic context and repair ambiguity or failure
  gracefully. Its AI explainability guidance requires a reviewable decision
  process, particularly for sensitive data. NexusAI therefore exposes curated
  service suggestions, clarification, failure recovery, method and limitations,
  without inventing questions or concealing provenance.
  - https://www.ibm.com/design/ai/conversation/
  - https://www.ibm.com/design/ai/conversation/planning/
  - https://www.ibm.com/design/ai/ethics/explainability/

## Product ruling

1. Use one centered conversation column. Remove page-local question-history
   duplication; the shell rail and mobile navigation drawer already provide it.
2. Keep each user question compact and visually distinct, but render the analyst
   answer as open document content rather than a stack of nested cards.
3. Keep the composer at the working edge, place the evidence-family scope inside
   it, clear submitted text, auto-grow the field, and support Enter to submit and
   Shift+Enter for a new line.
4. Populate starters only from `/query/capabilities` families that are currently
   available and have curated `suggested_queries`. If that endpoint is absent or
   fails, show no suggestions.
5. Preserve the binding answer order: answer, result, citations, derivation,
   limitations, follow-ups. Preserve claim-level citations, source preview,
   three-source rail threshold, evidence-strength text, honest zero/partial/
   unsupported/failed states, exact submitted scope and stop/retry behavior.
6. Use no attachment, voice, regeneration, streaming or tool controls because
   the corresponding analyst-safe backend capabilities do not exist.

## Measured outcome

- The composer is one compact prompt surface: a one-line auto-growing textarea,
  a 44px send action, a 44px Scope disclosure and secondary conversation actions
  in its footer. The 12-family selector appears only on request.
- Submitting with Enter clears the draft; Shift+Enter retains multiline entry;
  Alt+A and the existing keyboard source-preview path remain available.
- The page-local history rail was removed because the shell rail and mobile
  navigation drawer already expose the same browser-local questions.
- Starter questions are rendered only from available service families with
  `suggested_queries`; two unit contracts prove exact-scope filtering and the
  absence of a fabricated fallback.
- 94 unit and 46 ported tests pass. Twenty-two distinct responsive Investigate
  browser contracts pass across 1440, 1024, 820 and 390, including all seven
  result states, clarification branching, exact scope, stop, retry, keyboard
  citation preview and theme selection. The slice also measures zero horizontal
  overflow at 375px and a mobile send control no wider than 52px.
- Ten light/dark review captures cover empty, answered and clarification states
  at 1440 and 390. The final production build transforms 169 modules cleanly.

