# CODEX — Raise every workspace surface to enterprise grade

**Date:** 2026-09-28 · **From:** the backend track (Claude) at the product owner's request ·
**Scope:** `apps/investigation-workspace/**` only

The product owner wants every page (dashboard, cases, case overview, evidence, investigate,
timeline, activity, settings) to be **modern, clean, elegant, professional, interactive and
responsive at enterprise grade**. You own the UI. This brief adds the quality bar, the data each page
can truthfully show (verified against the live API today), and the answer behaviours shipped on the
backend today that the UI must render correctly.

**Still authoritative, and this brief does not override them:** `docs/ux/NEXUSAI_PRODUCT_UX.md`
(the UX contract) and `docs/work/UIUX_RESEARCH_20260927.md` (your research). Where the owner's words
seem to pull against the contract, read them as the contract's own terms:

| Owner asks for | Means here | Never means |
|---|---|---|
| "decorative" | considered iconography, real data visualisation, precise depth and spacing, crafted empty states | gradients, glass, neon, illustrations, looping or attention-seeking motion (contract §8.3) |
| "interactive" | every row, chip, count and chart opens or filters something real; keyboard shortcuts; command palette | controls that do nothing, fake demos, disabled-with-tooltip placeholders (contract §9) |
| "modern" | a tight type scale, a 4 px grid, calm neutrals, one restrained accent, crisp focus states | trend styling that reduces legibility in a courtroom printout |

---

## 1. The quality bar — what reviewers will check

Treat these products as references for **craft only** (density, typography, table and interaction
quality), never for IA or aesthetics: Linear, Vercel dashboard, Stripe dashboard, Datadog, IBM Carbon
data tables, Microsoft Fluent 2. For the domain: Magnet AXIOM *Connections* and Cellebrite
*Guardian* (case management). For plain language: GOV.UK.

A surface is enterprise-grade here when:

1. **The first screenful answers its question in 5 seconds.** The summary comes before the detail,
   and the most important state is visible without scrolling at 1280×800.
2. **Every number is live and traceable.** It comes from an API field named in §3, has a text label,
   and clicking it filters or opens the records behind it.
3. **State is encoded in form as well as colour:** a pill, an icon or a stripe, with text
   (contract §10).
4. **Nothing is decorative in the forbidden sense.** No gradients, no glass, no card soup. Borders,
   fills and shadows separate things by role only.
5. **It works at 1440, 1280, 1024, 768 and 375, in both themes, and by keyboard alone,** with no
   horizontal page scroll at 375.
6. **Every state is designed:** loading, empty, partial, processing, failed, unavailable, zero
   results, clarify.

---

## 2. First slice — design-system upgrade (do this before any page)

Everything after this slice reuses it. Put it in your existing tokens and style files. Keep
`tokens.vitest.js` green and extend it, never weaken it.

- **Type scale:** 12 / 13 / 14 (body) / 16 / 20 / 24 / 32, with line heights set per step. Use
  tabular numerals wherever digits align. Keep the installed Noto Sans and Noto Sans Mono variable
  fonts. No new font dependency.
- **Spacing:** a 4 px grid (4, 8, 12, 16, 24, 32, 48). Layout gaps use `gap`, not per-element
  margins.
- **Elevation:** four surface levels (contract §8.2), with shadows tuned separately per theme. Dark
  surfaces differ by lightness, not by glow.
- **Status and confidence tokens:** ready, processing, failed, excluded, and a *source record*
  vs *model observation* pair (§4.3). Each has a text or icon equivalent.
- **Icons:** an in-repo set of about 30 SVG React components (16 and 20 px, 1.5 px stroke,
  `currentColor`, `aria-hidden` unless standalone with a label). **No new npm dependency without the
  product owner's approval.** Neither `lucide-react` nor a chart library is installed today.
- **Data-visualisation primitives:** hand-built SVG components, drawn to one scale from real values,
  each with a text equivalent: `ProportionBar` (ready / processing / failed), `StackedBar` (rows by
  record type), `Sparkline` (only for series the API actually returns), `MeterBar` (accepted vs
  duplicate vs rejected).
- **Interaction states:** hover, focus-visible, active, selected, disabled and loading, defined once.
  Motion 120–200 ms, respecting `prefers-reduced-motion`. Nothing loops.
- **Density toggle:** comfortable or compact for tables, stored per browser. That is a legitimate
  browser-local preference.

**Acceptance:** a token and primitive gallery page (the existing `TypeProofPage` is the natural
place) showing every token, icon and primitive in both themes. Screenshots at 1440 and 375.

---

## 3. The data each page can truthfully show (verified against the live API, 2026-09-28)

### `GET /collections/status?collection_id=…` (per case)

    summary              evidence_total, evidence_completed, evidence_failed, evidence_in_flight,
                         accepted_rows, duplicate_rows, rejected_rows, total_rows,
                         jobs_total, completed_jobs, failed_jobs, kb_assets_total,
                         completed_jobs_missing_kb_asset
    record_families[]    record_type, accepted_rows, duplicate_rows, rejected_rows, total_rows, jobs
    recent_jobs[]        status, record_type, source_file, total/accepted/duplicate/rejected rows,
                         queued_at, started_at, completed_at, attempt_count, max_attempts,
                         error_message, last_error_class, dead_lettered_at
    recent_evidence[]    source_file, modality, detected_type, processing_status, size_bytes,
                         created_at, updated_at, errors, warnings
    missing_kb_assets[]

Known-good values: `nexusai-forensic-demo` has 12 evidence items (11 completed, 1 failed), 12,912
accepted rows, 301 duplicates and 7 rejected. CDR alone is 8,642 accepted and 299 duplicates.

### `GET /query/capabilities?collection_id=…`

`families[]` (id, label, formats, support_level), `summary` (queryable, manual_review, no_data,
unavailable), `coverage.valid_target_examples`, and `query_corpus.entries / scenarios`.
**The corpus entries are the only legitimate source of suggested questions.** Never invent a
suggested question.

### Also available

`GET /evidence` and `GET /evidence/{id}` (and `/content`), `POST /query/hybrid`, `GET /images/similar`
and `GET /faces/similar`, and browser-local question history (labelled *this browser only*).

### Does not exist — do not simulate

Users, assignment, severity, priority, investigative status, a case entity separate from a
collection, cross-case aggregation, sessions or auth, a timeline or source-event endpoint, and server
question history. The dashboard aggregates **only the configured cases**, client-side, and says so.
If a page needs one of these, record it as a backend request in `CODEX_UI_TRACK` and move on.

---

## 4. Answer behaviours shipped today — Investigate must render these correctly

All of these went live on 2026-09-28, each measured before shipping.

### 4.1 Withheld answers are a designed state, one variant per reason

The response carries `clarification.reason_code`. Render a distinct, calm card per code: a title in
plain words, the system's sentence verbatim, and the clarification `options` as one-click re-runs.

| `reason_code` | Card title | Primary action |
|---|---|---|
| `condition_not_applied` | A condition wasn't applied | Re-run with a named field (from options) |
| `relationship_not_computed` | That relationship wasn't computed | Name the shared field, or search one identifier across evidence |
| `uncomputed_quantity` | No count was computed | Pick what to count (options) |
| `single_family_negative` | Only one kind of evidence was searched | Search across all evidence |
| `media_question_structured_template` | Different evidence answered | Search the documents, images or video named in the question |
| `target_not_filtered` | Not restricted to the item you named | Restrict to it |
| `missing_required_parameter` | One detail needed | Options |
| `no_verified_plan` | Couldn't verify a query | Options, or narrow the question |
| `ambiguous_operation` | Couldn't map the question | Options, plus the real suggested questions for this case |
| `cross_check_disagreement` | Two checks disagreed | Narrow the question (options) |

Any code not in this table falls back to a neutral "One detail needed" card that shows the sentence
verbatim. Never invent a title from the code string.

A withheld answer is **never** styled as an error and never shows a number.

### 4.2 Sentences whose meaning the layout must support

- *"Showing 20 of 87 ANPR sightings that matched this question."* The result table shows a real
  total and page controls, never an implied complete list.
- *"There are 218 ANPR sightings involving ABC-123 in this case. The other values computed for this
  question are shown with this answer."* **The result table must sit directly under the answer**,
  visible without expanding anything.
- *"The results shown come from: Rule-based anomaly summary…"* The sections appear as a titled
  result, with no fabricated headline figure.

### 4.3 Source records vs model observations

Media answers now name what they counted: *"307 **plate reads**"*, *"20 **detected faces**"*,
*"309 **image text regions**"*. Camera records stay *"1,057 ANPR sightings"*. When a citation carries
`source_truth_state: derived_model_observation`, show a **Model observation** badge with its
confidence. A source row gets **Source record**. They must never look identical (contract §4.1).

### 4.4 Latency is real, so show it honestly

A question the system has seen before answers in about 1 s. **A new wording can take 2–2.5 minutes**
on this CPU-only server while the model writes the query. Show an honest in-progress state
("Writing the query for a new question — this can take a couple of minutes on this server"), keep it
cancellable, and never show a fake progress percentage. Show the elapsed time.

---

## 5. Surface-by-surface

For each surface: build, verify against the live API in the browser, screenshot both themes at 1440
and 375, test it, then record it in `CODEX_UI_TRACK`.

**5.1 Shell** — navigation rail with icons and labels (collapsible to icons at ≥1024, a drawer
below). Header with case switcher, breadcrumbs and theme. A **command palette (Ctrl/⌘+K)** covering
real routes, configured cases, recent questions (this browser) and "Ask in <case>". Every entry must
navigate somewhere real.

**5.2 Dashboard** — "what needs me now?":
- A **KPI strip** across configured cases: evidence total, ready, processing, failed, accepted rows.
  Each tile filters the case table below. Label it "configured cases only".
- A **case readiness table**: `ProportionBar` for ready/processing/failed, a `StackedBar` of accepted
  rows by record type, and the last evidence activity. Rows open the case overview.
- **Needs attention**: failed evidence and failed jobs, with `error_message` in plain words, attempt
  count and a link to the item. Then completed jobs missing a retained asset.
- **Ingestion quality**: accepted, duplicate and rejected rows per record type (`MeterBar`), with a
  one-line explanation of each.
- **Recent processing**: `recent_jobs` as a compact timeline with real durations
  (`completed_at − started_at`).
- **Continue recent questions** (this browser), and **Suggested questions** from `query_corpus`,
  each opening Investigate pre-filled.
- Refresh with a "last updated" time. When anything is `in_flight`, poll every 15 s, paused while
  the tab is hidden.

**5.3 Cases** — a sortable, filterable, searchable table: status pill, evidence count, family
composition bar and last activity. Keyboard row navigation.

**5.4 New case** — keep the four-field rule and evidence-first intake. Polish the drop zone
(per-file progress, detected family shown with one-click correction).

**5.5 Case overview** — composition by record type (`StackedBar` plus exact counts), processing
state, a **data-quality panel** (duplicates, rejected rows, failed items with reasons), suggested
questions for this case, and recent evidence.

**5.6 Evidence catalogue** — a dense table with a sticky header and pinned first column below 1280,
status pills, family chips whose counts say what they count, a detail drawer preview, and a density
toggle. Failed items stay visible with their reason.

**5.7 Evidence detail and viewers** — per contract §6.5, as far as the API supports: an always
reachable lineage panel, OCR and plate overlays with confidence on images, and a transcript beside
audio. Anything unsupported gets an honest unavailable state.

**5.8 Investigate** — the contract §6.6 order: answer, result, citations, derivation (collapsed),
limitations, follow-ups. Also a thread with a question-history sidebar (this browser), scope chips
beside the input, keyboard shortcuts (focus ask, re-run, copy answer), every §4 state, and a
citations rail at ≥1280.

**5.9 Timeline** — still no endpoint. Design the unavailable state properly: why, what to ask
instead (real suggestions), and no fake axis.

**5.10 Activity** (global and case) — group by day, search, filter by type, and plain-language
entries.

**5.11 Settings** — polish only; it's done. **5.12 Route states** — designed 404, error and
offline pages with a way back.

---

## 6. Order of work

Design system (§2) → Shell (5.1) → **Dashboard (5.2)** → **Investigate (5.8)** → Case overview (5.5) →
Evidence (5.6, 5.7) → Cases and New case (5.3, 5.4) → Activity, Timeline, route states. One surface
per change, each independently reviewable and revertible.

## 7. Verification for every slice

- `npm --prefix apps/investigation-workspace run test` stays green. Add tests for every rendered
  count and state, especially the §4.1 variants and the §4.2 layouts.
- Check in the browser against the **live API** at `http://127.0.0.1:4181`, not fixtures alone. The
  known-good numbers in §3 must appear exactly.
- Screenshots: both themes at 1440, 1280, 1024, 768 and 375. Check for zero horizontal overflow,
  zero console errors and zero duplicate IDs.
- Keyboard-only pass through the core loop: open case, add data, ask, read, open citation, follow
  up, find it in Activity.
- Contrast: text at least 4.5:1, UI and focus at least 3:1 against every adjacent surface.

## 8. Never, and stop-and-report

**Never:** edit `api/**` or `semantic_layer/**`. Invent or estimate a number, a suggestion, a user or
a status the API didn't return. Show a model, template, SQL or operation name to an analyst. Add an
npm dependency without the product owner's approval. Run git writes (`reset --hard`, `clean`,
`checkout --`, `restore`, `stash`, `merge`, `rebase`, `cherry-pick`, `pull`, `push`, force), or
bypass hooks. Delete an untracked file you didn't create. Write files with Bash heredocs (they have
corrupted files here twice).

**Stop and report if** a page needs an endpoint that doesn't exist, a test fails for a reason you
don't understand, or a design choice would weaken contrast, table semantics or a token test.

## 9. Done means

Every surface in §5 meets §1, renders every §4 state, uses only §3 data, and passes §7. It should
read as one coherent, calm, fast, professional product at every width and in both themes. **No number
appears anywhere that the service did not state.**
