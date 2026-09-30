# WI-UI-8 — the verification loop, the working loop, and forensic-scale data

You are the UI/UX track (P6) on NexusAI. WI-UI-7 is accepted: identity, the three-band
shell, claim-level citations, governed palette, scope chips, route states and the
same-origin proxy all stand. **Do not redo any of it.**

This item is about making the product *fast to work in*. The brief from the product owner is
"more modern, more interactive, more productive, enterprise grade" — and in THIS product that
means a specific thing, stated below before any of the work.

---

## 0. WHAT "BETTER" MEANS HERE — read this before designing anything

`docs/ux/NEXUSAI_PRODUCT_UX.md` §8.3 forbids, by name: glassmorphism · neon/cyberpunk ·
purple AI gradients · marketing hero blocks · card soup · decorative motion · skeleton
shimmer longer than the request · emoji as status · progress bars that do not reflect real
progress. §8.1 states **"density is a feature"** and §1 states the primary UI object is
PROVENANCE, not the answer.

So "modern and attractive" is not delivered by ornament. It is delivered by:

    keyboard-first operation       an analyst works this for hours, not minutes
    direct manipulation            click the number, land on the row that produced it
    progressive disclosure         dense by default, depth one keystroke away
    information density            more evidence per screen, not more whitespace
    sub-100ms interaction          local state never waits on the network

Anything that looks impressive in a screenshot and slows an analyst down is a regression.
Judge every choice by: *does this get the analyst to a verified fact faster?*

## 1. READ FIRST

    docs/ux/NEXUSAI_PRODUCT_UX.md          AUTHORITATIVE. §4 provenance · §6.5 viewers
                                           · §6.6 answer contract · §11 responsive · §12 components
    docs/design/nexusai-app-shell-specification.md
    docs/design/nexusai-analyst-design-system-v2.md   §105 density and disclosure

## 2. CROSS-TRACK NOTICE

You own `apps/investigation-workspace/**`, `semantic_layer/**`, `db/**`, `ingestion/**`.
**Do not touch `api/**`** — the backend track is mid-flight and P3's ladder deletion is next.

The backend is at 47 CORRECT · 14 CLARIFIED · 0 WRONG on the 62-question suite, p95 5.2 s.
**The 14 clarifications are correct behaviour, not failures** — five measured attempts to
convert the last two produced seven confident-wrong answers and were all reverted. Design the
clarification surface as a first-class success state, not an error.

## 3. WORKSTREAM A — the verification loop (highest value)

**The product's whole thesis is that an analyst can verify any number in one click.** §13's
definition of done requires "open a citation to its exact source". Today
`EvidenceDetailPage.jsx` is **144 lines covering all five modalities** — it honours the
locator (page, char_span, t_start, bbox) and is a working deep-link target, which is real
progress. It is not yet the viewer §6.5 specifies.

Build them to contract. Each row is a stated requirement, not a suggestion:

| Family | Must support |
|---|---|
| Document | paginated reader · page navigation · text selection · deep-link to page + char span · **highlight on arrival** |
| Structured | **virtualised** table · column sort · filter · row hash on demand · **jump to row number** |
| Image | OCR/ANPR regions as **toggleable overlays with their confidence** · original always viewable without overlays |
| Audio | transcript **synchronised to playback** · click a line to seek · deep-link to `t_start` · speaker turns when diarization exists |
| Video | derived events (ANPR, OCR, transcript) **pinned on the scrubber** · frame extraction · deep-link to timestamp |
| Any | **lineage panel**: source file → version → derived artifacts → the model/backend and version that produced each, with confidence. Always available, never buried. |

**The citation highlight must pulse once on arrival and then stop** (§8.4 — motion ≤200 ms,
purposeful, respects `prefers-reduced-motion`, nothing loops).

Where a locator cannot be produced, the citation must not render as one (§4.2). That rule is
already honoured — keep it.

## 4. WORKSTREAM B — the analyst's working loop

This is the "productivity" the brief asks for. Every item below is a real analyst behaviour,
not a feature for its own sake.

**B1 · Command palette (⌘K / Ctrl-K).** Jump to a case, a route, an evidence item; re-run a
past question; switch scope. One keystroke from anywhere. This is the single highest-leverage
productivity addition and it is how every serious analyst tool works.

**B2 · Keyboard path through the core loop, with discoverable shortcuts.** §10 already
requires a full keyboard path; make it *fast*, not merely possible. `?` opens a shortcut
sheet. Focus never gets lost after an ask.

**B3 · Question history and pinning, per case.** `QuestionTrail` exists — make it a working
surface: re-run, edit-and-re-run, pin a question to the case. An analyst asks the same five
questions across twenty cases.

**B4 · Answer → evidence cross-navigation.** Clicking a value in THE RESULT opens the rows
that produced it. Clicking a citation opens the viewer at the locator. Both directions.

**B5 · Never lose a draft.** An in-progress question survives navigation and reload
(per-viewer storage, wrapped in try/catch, correct when it throws).

**B6 · Comparison.** Two results side by side for the same case — the analyst's most common
manual workaround today is two browser tabs.

## 5. WORKSTREAM C — forensic-scale data

The demo case is 12,912 rows; real cases are larger. §12 specifies `ResultTable`
**(virtualised, semantic-layer headers)** and it is not virtualised today.

- Virtualise, and keep real `<table>` semantics with header associations (§10 — virtualisation
  must not break them).
- Sort, and a density toggle (comfortable / compact).
- Sticky header and sticky first column; §11 requires the pinned first column at 768–1279.
- Numerals use tabular figures and right alignment; a totals row where a total is meaningful
  and **never invented** where it is not.
- Export the current result (CSV) with its citation locators intact.

## 6. WORKSTREAM D — craft defects visible in the WI-UI-7 review set

Each is from `design-review/wi-ui-7/answered-daylight-ledger-1440.png`.

**D1 · The claim markers make the finding hard to read.** The sentence renders as
"8,642 ① CDR records across 5 ① call type values. Largest: Data session: 5,863 ①; SMS: 1,108
②; Call: 930 ①." Claim-level attribution is required (§4.1) and correct — but interleaved
numbered pills break reading flow at exactly the moment the analyst is reading the answer.
Keep the attribution; change the treatment. A subtle underline or marker on hover/focus with
the number revealed on demand satisfies §4.1's "every claim carries its own marker" without
shredding the sentence. **Measure it: the finding must be readable aloud in one pass.**

**D2 · The ask panel outweighs the finding.** A large tinted block at the top is the heaviest
element on the page; the finding is what the analyst came for. Rebalance.

**D3 · The scope chips consume two full rows** at the top of the working surface. They are a
filter, not the subject. Consider a single row with overflow, or a compact summary that
expands.

**D4 · Dead space.** At 1440 the citation rail ends and roughly a third of the viewport below
it is empty, while the result table is cramped into a narrow column. Use the canvas (§11:
"bounded readable widths plus analytical canvases").

**D5 · The navigation rail is mostly empty** below seven items. Case context, evidence
readiness or recent questions belong there — the case band already carries some of it.

## 7. CONSTRAINTS THAT DO NOT MOVE

- Never invent evidence, chronology, a percentage for processing, a label for an uncurated
  value, or a total that was not computed. Zero / unavailable / unprocessed stay distinct.
- Every state uses icon + label + description; colour never carries meaning alone.
- A citation without an openable locator is not rendered as a citation.
- A source row and a 0.31-confidence OCR fragment must never look identical (§4.1).
- CNIC masking is server-side at projection. The UI never unmasks and never adds its own.
- No model, backend, agent, operation, template, SQL, embedding or token count on any analyst
  surface (§9). A control that is not implemented is not shown — not shown-and-disabled.
- Motion ≤200 ms, purposeful, respects `prefers-reduced-motion`, nothing loops.
- Logical properties throughout; RTL mirroring and `dir="auto"` / `<bdi>` isolation hold.
- The browser holds no credential. The same-origin proxy is mandatory.
- Port, do not rewrite, `analyst*Presentation.js` and their tests.

## 8. ACCEPTANCE — measured, not asserted

1. **A recorded keyboard-only run of the full core loop** (§13): login → case → add data →
   wait → ask → read → **open a citation to its exact source** → follow up → find it in
   Activity. This is the definition of done and it must be demonstrated end to end.
2. Each of the five viewers shown opening at a real locator, with the highlight on arrival.
3. Virtualised table proven at **≥10,000 rows**: scroll smooth, header semantics intact,
   sticky first column at 768–1279, keyboard navigable.
4. Command palette reachable from every route; `?` shortcut sheet.
5. Draft question survives reload and survives storage throwing.
6. Contrast re-measured for anything whose colour changed; `tokens.vitest.js` still enforces.
7. Screenshots at 375 / 820 / 1024 / 1440 in both themes for every new surface, plus the
   before/after of D1's finding treatment.
8. No horizontal page scroll at 375px; touch targets ≥44px.
9. Ported, unit and Playwright suites green — Playwright covering the citation-to-viewer path,
   clarification, zero-result and unsupported.
10. Production build clean.

## 9. REPORT BACK

What you changed, what you measured, and **what you could not build because the endpoint does
not exist** — naming the missing endpoint rather than rendering a plausible value. Your
WI-UI-7 report did this well; keep it.

State explicitly which of §6.5's viewer requirements are met and which are not. A viewer that
cannot yet show speaker turns because diarization is not wired is an honest gap; a viewer that
implies it has them is a defect.
