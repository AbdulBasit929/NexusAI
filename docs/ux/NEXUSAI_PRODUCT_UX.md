# NexusAI — Product UX Specification
**Status:** target design · **Date:** 2026-09-21 · **Owner:** Codex track (P6), informed by the Claude compiler track (P3)
**Authority:** this document is the stable UX contract. Where it conflicts with an older UI report, this wins. Where it conflicts with `docs/architecture/TARGET_ARCHITECTURE.md` on *behaviour*, the architecture wins.

---

## 0. The one-sentence mandate

> An analyst with no technical training opens a case, adds evidence, asks a question in plain English, and receives an answer they can **defend in a courtroom** — with every number traceable to a source row they can open.

Everything below serves that sentence. Anything that does not serve it is out of scope.

---

## 1. What this product is — and what it is not

| This is | This is not |
|---|---|
| A **case-centric investigation workspace** | A chat application with a sidebar |
| An **evidence system that happens to use AI** | An AI product that happens to hold evidence |
| A tool where **provenance is the primary UI object** | A tool where the *answer* is the primary UI object |
| Something an analyst trusts *because they can verify it* | Something an analyst trusts because it sounds confident |

**The information architecture is organised by CASE, then by EVIDENCE, then by QUESTION.** It is never organised by modality. There is no "Audio" section, no "Images" section, no "Documents" section in the primary navigation — those are *filters inside a case*, because an investigator thinks *"what do I know about this case"*, never *"show me my audio files"*.

---

## 2. The Local-Mind reference — honest assessment

**Source `[VERIFIED 2026-09-21]`:** `https://github.com/satiricalguru/Local-Mind` — this URL **does exist** (the earlier `.../Local-Mindui/ux` path returned 404). It is an **Electron desktop app for running local AI models offline**: Electron + React 18 + Vite + TanStack Router + Tailwind v4, SQLite via better-sqlite3, pnpm/Turborepo. Its surfaces are *playgrounds*: chat with live performance metrics, image generation, audio/video synthesis, Whisper STT, TTS, and a **Model Hub** for discovering and installing weights. Themes are **"GPU-Noir Dark Mode"** and **"Vanilla Light"**, with **glassmorphic** styling and a real-time system-monitoring sidebar showing CPU, RAM, GPU and temperature.

### 2.1 What we take from it

| Take | Why it applies to us |
|---|---|
| **The feel of a tool, not a website** — frameless, dense, purposeful, no marketing chrome | Correct instinct. Analysts use this for hours. It should feel like instrumentation. |
| **Honest system state surfaced in the interaction itself** (their live tok/s readout) | Adapt, don't copy: we surface *evidence* state — "searched 8,642 rows", "3 files still processing", "answered in 1.2 s". Truth made visible. |
| **Two named, opinionated themes driven by CSS custom properties** | Better than a generic light/dark switch. We will have two *designed* surface systems, not one design with inverted colours. |
| **A single place for model/weight management, clearly separated from use** | Their Model Hub → our `/admin`. Confirms the separation. **Analysts never see it.** |
| **TanStack Router** as a typed routing option | Worth evaluating against React Router, which the current fork already uses. Not a priority. |

### 2.2 What we explicitly reject

| Reject | Why |
|---|---|
| **Glassmorphism** | The product directive excludes it by name. It reduces contrast and legibility, and data-dense evidence tables are exactly where that hurts most. |
| **"GPU-Noir" aesthetic** | This is the neon/cyberpunk direction the directive rules out. Forensic output may be read in a courtroom. It must look like a record, not a game. |
| **Playground-per-modality IA** | The single biggest divergence. Their nav is *Chat / Image / Audio / Video / Models*. Ours is *Cases / Evidence / Investigate*. Adopting theirs would make NexusAI exactly the "generic local-chat application" the directive forbids. |
| **System telemetry sidebar (CPU/GPU/temp) for end users** | An analyst must never be asked to care about GPU temperature. That is `/admin`. |
| **Electron desktop-only shape** | NexusAI is multi-user, server-backed, with case membership and row-level security. A local single-user desktop app is the wrong container. (A desktop wrapper over the web app is possible later; it is not the architecture.) |
| **Model performance metrics in the answer surface** | Tokens/sec next to a forensic finding invites the analyst to judge the *model* instead of the *evidence*. |

**Summary:** Local-Mind is a well-built local-AI playground. It is a good reference for *craft and density* and a bad reference for *information architecture*. We take the former and discard the latter.

---

## 3. What investigation software actually does well `[RESEARCH 2026-09-21]`

Patterns observed across Magnet AXIOM, Cellebrite (Inseyets / Inspector / **Guardian**), Nuix Neo and the open-source CaseLinker system:

1. **Timeline as a first-class view**, not a widget. Reconstructing *when* is half of investigation.
2. **Relationship discovery as its own surface** — AXIOM calls it *Connections*; Nuix calls it relationship analysis; CaseLinker uses network visualisation over shared identifiers. NexusAI's cross-family identifier joins are the same idea and are currently the weakest family (0/5).
3. **A dedicated evidence-management and collaboration layer, separate from analysis tooling** — Cellebrite splits *extraction* from *inspection* from *Guardian* (case management + collaboration). This validates our split: Evidence ≠ Investigate ≠ Admin.
4. **Automated extraction over manual data entry** — CaseLinker's core design lesson from working with real investigators: never make an investigator retype what a document already contains.
5. Three AI patterns are converging across every vendor for 2025–2026: **artifact summarization**, **suggested next steps**, and **natural-language query**. We are building all three. Our differentiator is that ours are *grounded and abstaining*.

---

## 4. The provenance system — our actual differentiator

> **The threat `[RESEARCH]`:** industry analysis in 2026, citing a 2025 study, reports that **more than half of RAG-generated citations exhibit post-hoc rationalization** — the model decides its answer first, then scans retrieved documents for surface-level token matches to manufacture a reference. The citation looks real. It is decoration.

**NexusAI is structurally immune to this, and the UI must say so.** Our answer is computed from Fact Packet **values produced by SQL over authorized rows**, and the citation is the *row that produced the value* — not a document the model went looking for afterwards. The citation is not attached to the answer; **the citation is where the answer came from.**

This is the most valuable thing about the product. Design for it.

### 4.1 Citation patterns we adopt `[RESEARCH]`

| Pattern | Our implementation | Anti-pattern being avoided |
|---|---|---|
| **Claim-level attribution** — recommended specifically for high-stakes domains (legal, medical, finance) | Every number, name, date and identifier in the answer sentence carries its own marker. Not one marker per paragraph. | *"Citation per paragraph fudges statements, mixing sourced claims with unsourced inferences while appearing sourced."* |
| **Deep-link to the exact passage** | Store offsets at index time and honour them: document → page + character span; table → row number + row hash; audio → `t_start`/`t_end` with the player seeking there; video → frame timestamp; image → OCR/ANPR bounding region. Opening a citation **highlights the span**, it does not open the file. | *"Linking only to document homepages forces users to manually locate supporting passages."* |
| **Hover preview** | Hovering a marker shows source file, row/page, timestamp, and a ≤200-character excerpt, without leaving the answer. | Forcing a round-trip to verify one number. |
| **Source panel for multi-source answers** | Three or more distinct sources → a right rail listing them, numbered consistently with the inline markers. On mobile it collapses to an expandable "Sources" section. | Using a rail for a single-citation answer — visual imbalance. |
| **Evidence strength indicator** | Derived-artifact claims carry their calibrated confidence and their origin. A CDR source row and a 0.31-confidence OCR fragment **must not look identical**. | *"A claim backed by a single Reddit post and a claim backed by three peer-reviewed studies should not look identical."* Our version: a source row and a model guess must not look identical. |
| **Unsourced-claim disclosure** | Any sentence not anchored to a fact gets a visible "no source" treatment and is never styled like a sourced claim. In practice the deterministic answer is always sourced; this guards the **optional narration**. | *"When an AI answer includes a single unsupported claim styled identically to sourced claims, users absorb the false confidence."* |
| **Scope controls that fail loudly** | Filters (family, date range, evidence subset) sit next to the question box, persist across the session, and when they match nothing the system **says so** rather than silently widening. | Auto-widening scope on empty results. This is defect **D1** expressed as a UI principle — the same failure in a different layer. |

### 4.2 The citation contract
One shape, everywhere:
```
(evidence_id, version_id, source_file, record_id, row_number, row_hash,
 page, char_span, t_start, t_end, frame_ts, bbox, derived_from, confidence)
```
A citation that cannot produce an openable locator is **not a citation** and must not be rendered as one.

---

## 5. Information architecture

```
GLOBAL
  Dashboard            what needs my attention across all my cases
  Cases                list · filter · search · create
  Activity             what I and my team did
  Settings             profile, preferences
  Admin  (authorized)  users, org, system health, models/backends  ← analysts never see this

CASE WORKSPACE  /cases/:id/…
  Overview      what is in this case, what is still processing, what is questionable
  Evidence      inventory · add data · per-item detail and viewers
  Investigate   the ask surface — answers, findings, citations, follow-ups
  Timeline      chronology across every family
  Activity      case history: who added what, who asked what, what was exported
```

**Evidence families are filters, never navigation.** Inside Evidence and Investigate, families appear as chips: `CDR · IPDR · ANPR · Subscriber · Tower · Financial · Access log · Documents · Images · Audio · Video`. Adding a nineteenth family adds a chip, not a menu item. This is the property that makes the IA scale.

---

## 6. Screen contracts

Every screen declares: purpose · primary user · data · actions · empty · loading · failure · responsive.

### 6.1 `/` Dashboard
- **Purpose:** answer "what should I do next?" in under five seconds.
- **Data:** my cases with status; evidence still processing; evidence that **failed** processing; recent questions; recent activity.
- **Empty:** a single primary action — *Create your first case* — with one line explaining what a case is. No illustration, no marketing.
- **Failure:** if the API is unreachable, say which part is unavailable and what still works. Never a blank page.

### 6.2 `/cases` · `/cases/new`
- **Create:** name, reference number, description, initial members. Four fields. Everything else is editable later.
- **List:** status, evidence count, last activity, members. Sort and filter. Search by name or reference.

### 6.3 `/cases/:id/overview`
- Evidence counts **by family**, with processing and failed counts shown, never hidden.
- **Data-quality notes as a first-class panel.** Concrete examples already found in this data: *"425 CDR rows list 923001110001 as its own counterparty"*, *"1 structured evidence item failed processing and is excluded from counts"*, *"1 file appears to be a command transcript, not evidence"*. An investigator must learn this from the product, not from a surprise in court.
- Members and their roles.

### 6.4 `/cases/:id/evidence`
- Inventory table: name, family, size, added by, added when, **status** (`queued / processing / ready / failed / excluded`), derived artifacts produced.
- **Add data:** drag-and-drop anywhere on the panel; multi-file; per-file progress; the system classifies the family and **shows what it decided** with a one-click correction. Never ask the analyst to pick a parser.
- **Failed items are never silently dropped from counts.** A failure is a visible row with a reason and a *Retry* action.
- **Empty:** a drop target and one sentence on what can be added.

### 6.5 `/cases/:id/evidence/:eid` — viewers
| Family | Viewer | Must support |
|---|---|---|
| Document | paginated reader | page navigation, text selection, **deep-link to page + char span**, highlight on arrival |
| Structured | virtualised table | column sort, filter, row hash visible on demand, jump to row number |
| Image | image with overlays | OCR regions and ANPR plates as toggleable overlays **with their confidence**, original always viewable without overlays |
| Audio | player + transcript | transcript synchronised to playback, click a line to seek, **deep-link to `t_start`**, speaker turns when diarization is available |
| Video | player + event timeline | derived events (ANPR, OCR, transcript) pinned on the scrubber, frame extraction, deep-link to timestamp |
| Any | lineage panel | source file → version → derived artifacts → the model/backend and version that produced each, with confidence. Always available, never buried. |

### 6.6 `/cases/:id/investigate` — **the core screen**

Fixed vertical order. Never inverted, never reordered by response type:

```
 1  THE ANSWER          one sentence, largest type on the screen, with inline
                        claim-level citation markers.
                        "There are 8,642 CDR records in this case."
 2  THE RESULT          the number, table, chart or timeline the answer came from.
                        Column headers are DISPLAY NAMES from the semantic layer
                        — "Call duration", never "M1".
 3  CITATIONS           numbered, matching the inline markers; hover preview;
                        click opens the source at the exact locator.
 4  HOW THIS WAS DERIVED   collapsed by default. Plain language on expand:
                        "Counted rows in Call Detail Records for this case."
                        The compiled plan and SQL live one level deeper, for
                        admins and for audit.
 5  LIMITATIONS         shown only when real: excluded failed evidence, low
                        confidence, partial coverage, capability gaps.
 6  FOLLOW-UPS          2–4 concrete next questions built from the semantic
                        layer and the current result — never generic prompts.
```

**States, all of which are first-class:**
- **Answered** — the above.
- **Clarify** — §7. A success, styled as one.
- **Zero results** — *"No calls from 923001110001 in August 2026."* State the filters that were applied. Offer to widen **explicitly**, never automatically.
- **Unsupported** — *"This deployment cannot analyse video scenes."* Say what *is* possible with this evidence. Never a generic error.
- **Partial** — answer what is answerable, name what was excluded and why.
- **Processing** — *"3 files are still processing; this answer covers the 12 that are ready."* Offer to re-run when complete.
- **Failed** — plain language, an action, and an error reference. Never a stack trace, never `expected 3 arguments, got 23`.

**Streaming:** the deterministic answer appears first, complete. Optional narration streams in *below* it and is visually distinguished as interpretation. If narration times out or fails grounding validation, **nothing visible is lost** — no spinner that resolves to an error, no answer that disappears.

### 6.7 `/cases/:id/timeline`
Chronology across families on one axis. Filter by family, identifier, date range. Each event opens its source. This is where cross-family work becomes legible.

### 6.8 `/cases/:id/activity`
Evidence added and reprocessed, custody events, **every question with its answer and its compiled plan**, exports. This is an audit record, written in plain language for analysts, with the technical trace one level down.

### 6.9 `/admin`
Users, roles, org, system health, LocalAI status, models and backends, queue depth, capability matrix. **Authorization-gated and absent from analyst navigation entirely** — not merely disabled.

---

## 7. The clarification surface

Abstention is a designed outcome of the architecture, so it must be a designed *experience*. Done well it reads as competence; done badly it reads as failure. Same information, entirely different product.

```
✗  "Query could not be planned."
✗  "MEASURE_FIELD_UNRESOLVED"
✗  "I'm not sure what you mean. Could you rephrase?"

✓  I can answer this, I just need one detail.
   By "total", do you mean:
   [ Call duration ]  [ Data volume ]  [ Transaction amount ]
   Asked: "What's the total for 923001110001 in August?"
```

Rules:
1. **Name the ambiguity specifically.** Never "please rephrase".
2. **Offer 2–4 concrete options**, labelled with the semantic layer's `display_name` and, on hover, its `description`.
3. **One click re-runs** the question with the choice bound. The analyst never retypes.
4. **Show the original question** so the analyst keeps their place.
5. **Never more than one clarification round** before offering a manual scope/filter path instead.
6. When a literal could not be bound (defect D1's guard firing), say which one: *"I couldn't find a field holding `03999999999` in this case's evidence."* That sentence is more useful than any answer would have been.

---

## 8. Visual system

### 8.1 Principles
- **Layered surfaces, never flat canvases.** Light mode is not white. Dark mode is not black. Both use 4 deliberate elevation levels.
- **Colour carries meaning, not decoration.** Accent = interactive. Status colours = status. Nothing is coloured to look modern.
- **Density is a feature.** Investigators scan. Generous whitespace between rows of a 10,000-row table is hostile.
- **Typography does the hierarchy work**, not borders and shadows. Geist is already installed in the fork.

### 8.2 Tokens
Define once on `:root`, redefine under `@media (prefers-color-scheme: dark)` guarded by `:root:not([data-theme="light"])`, and again under `:root[data-theme="dark"]`. Never hard-code a colour in a component.

```
--surface-0   application backdrop        (light: cool neutral, NOT #fff)
--surface-1   primary working surface     (light: white/near-white, elevated)
--surface-2   raised panels, drawers, cards
--surface-3   popovers, hover previews, menus
--surface-ask soft blue — the question/conversation surface only
--border-subtle / --border-strong
--text-primary / --text-secondary / --text-muted
--accent            restrained professional blue — interactive only
--status-ready / --status-processing / --status-failed / --status-excluded
--evidence-strong / --evidence-medium / --evidence-weak   ← confidence bands
--focus-ring        ≥3:1 against every adjacent surface
```

**Light:** `surface-0` cool neutral (a desaturated slate-tinted grey), `surface-1` white or near-white, content sitting *on* the app rather than being the app. **Dark:** `surface-0` deep cool slate — never pure black — with three visibly distinct levels above it; data surfaces stay clearly readable; active states use a subtle blue fill rather than a glow.

### 8.3 Explicitly forbidden
Flat white canvas · flat black canvas · purple AI gradients · neon or cyberpunk styling · glassmorphism · large marketing hero blocks · card soup (every element in its own bordered card) · decorative motion · generic admin-template chrome · skeleton shimmer that runs longer than the request · emoji as status indicators · progress bars that do not reflect real progress.

### 8.4 Motion
Purposeful only: drawer open/close, a citation highlight pulsing **once** on arrival, row expansion, streaming text. Everything ≤200 ms. Everything respects `prefers-reduced-motion`. Nothing loops. Nothing draws attention to itself.

---

## 9. Non-technical user contract

An analyst must **never** be required to choose or even see: a model, an agent, an operation, a template, a processor, a backend, SQL, a vector index, an embedding, a confidence threshold, or a token count.

Language rules, with real replacements drawn from current defects `[TEST: P0-baseline-report.md §3]`:

| Never show | Show instead |
|---|---|
| *"Executed bounded source-native typed algebra over 8642 authorized source rows and returned 1 deterministic results with contribution lineage"* | *"There are 8,642 CDR records in this case."* |
| Column header `M1` | `Call duration` (the semantic layer's `display_name`) |
| *"Records Row Count: 1"* for a count of 8,642 | the count itself, as the answer |
| *"Template: financial_transaction_summary"*, *"Route: records_sql"*, *"Planner Confidence: 1"* | nothing — these belong in the audit trace |
| *"execute analytical template: expected 3 arguments, got 23"* | *"Something went wrong running this query. Reference: QRY-4821."* |

Every visible control must correspond to real, working functionality. A control that is not implemented is not shown — not shown-and-disabled, not shown-with-a-tooltip.

---

## 10. Accessibility
- WCAG 2.2 AA minimum. Text ≥4.5:1; UI components and focus rings ≥3:1 against **every** adjacent surface, in both themes.
- Full keyboard path through the core loop: open case → add data → ask → read answer → open a citation → follow up. Tested, not assumed.
- Visible focus at all times. Skip-to-content. Logical tab order in drawers, with focus trapped while open and restored on close.
- Every status conveyed by colour is **also** conveyed by text or icon. Confidence bands especially.
- Live regions for streaming answers and processing status, announced once — not on every token.
- Tables use real `<table>` semantics with header associations. Virtualisation must not break them.
- Respect `prefers-reduced-motion` and `prefers-contrast`.

---

## 11. Responsive behaviour
Desktop is the primary target; the product must remain usable on a tablet in the field.
- **≥1280px** — three zones: navigation, working surface, contextual rail (citations, lineage).
- **768–1279px** — rail collapses to a drawer; tables gain horizontal scroll with a pinned first column.
- **<768px** — single column; Evidence and History become full-screen drawers; the ask surface stays primary; 16px side gutter; **no horizontal page scroll**; touch targets ≥44px.

---

## 12. Component inventory
`AnswerBlock` · `CitationMarker` · `CitationPreview` · `SourcePanel` · `ClarificationPrompt` · `EvidenceStrengthBadge` · `ResultTable` (virtualised, semantic-layer headers) · `ResultTimeline` · `ScopeChips` · `AskInput` · `FollowUpChips` · `DerivationDisclosure` · `LimitationsNote` · `EvidenceInventory` · `EvidenceStatusPill` · `AddDataDropzone` · `LineagePanel` · `DocumentViewer` · `TableViewer` · `ImageViewer` (overlay toggles) · `AudioViewer` (transcript sync) · `VideoViewer` (event scrubber) · `CaseHeader` · `CaseSwitcher` · `ActivityFeed` · `DataQualityNotes` · `EmptyState` · `FailureState`.

Port and keep — do not rewrite — the existing presentation logic and its tests: `analystPrimaryAnswer.js`, `analystAskPresentation.js`, `analystDataPresentation.js`, `analystActivityPresentation.js`, `analystMediaPresentation.js`, `investigationWorkspace.js`.

Keep as *patterns* from the current LocalAI UI `[SOURCE: router.jsx]`: route-level code splitting with hover preloading (`preloadRoute`), and capability-gated routes (`RequireFeature`) driven by the capability matrix rather than hard-coded.

---

## 13. Definition of done for the UI phase

1. A non-technical analyst, observed and unaided, completes: **login → create case → add data → wait for processing → ask a question → read the answer → open a citation to its exact source → ask a follow-up → find it again in Activity.**
2. Every screen implements its empty, loading, failure, zero-result, clarify, partial and unsupported states.
3. No analyst-facing surface exposes a model, backend, agent, operation, template or SQL.
4. Every number in every answer is traceable to an openable source locator, in one click.
5. Keyboard-only completion of the core loop.
6. WCAG 2.2 AA verified in **both** themes.
7. Playwright E2E covers every flow **including clarify, zero-result and unsupported** — not only the happy path.
8. No horizontal page scroll at 375px.

---

## 14. References
- `https://github.com/satiricalguru/Local-Mind` — team-lead UX reference, assessed in §2 `[VERIFIED 2026-09-21]`
- [AI citation and source UI design patterns for 2026 — AYDesign](https://www.aydesign.ai/blog/ai-citation-source-ui-patterns-2026) — §4 patterns and anti-patterns
- [AI Hallucination and Grounding — ClarityArc](https://www.clarityarc.com/insights/ai-hallucination-grounding-citation) · [Provenance for Every Answer — Sphere](https://www.sphereinc.com/blogs/provenance-for-every-answer) — provenance-as-trust; post-hoc rationalization finding
- [How Source Attribution Visualization Shapes User Attention and Preference (eye-tracking, four chatbot layouts)](https://pmc.ncbi.nlm.nih.gov/articles/PMC13513764/) — *full text was CAPTCHA-blocked this session; listed as a pointer, not relied upon*
- [SpheriCity: Designing Trustworthy Conversational AI for Decision Support (arXiv 2606.13854)](https://arxiv.org/pdf/2606.13854) — experts rarely accept synthesized claims without traceable evidence
- [CaseLinker: Cross-Case Analysis of ICAC Reports (arXiv 2603.18020)](https://arxiv.org/pdf/2603.18020) — investigator-facing design lessons
- [Top 5 DFIR Tools for 2026](https://guptadeepak.com/tools/top-5-dfir-tools-2026/) · [Magnet AXIOM vs Cellebrite vs EnCase](https://www.decryptiondigest.com/blog/digital-forensics-tools-magnet-axiom-cellebrite-encase-comparison) — vendor IA patterns in §3
