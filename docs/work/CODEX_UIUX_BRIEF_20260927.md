# CODEX — analyst interface: research, then redesign, surface by surface

Written 2026-09-27. **Your scope is `apps/investigation-workspace/**` only.** Claude owns
`api/**` and `semantic_layer/**` and is working there in parallel — do not edit them, and do not
wait on them.

The workspace is wired to the live forensic API and verified end to end. **It is a real product
with 75 passing unit tests, Playwright e2e, and a tested design-token system. Improve it surface by
surface. Do not restart it**, and do not rebuild what §2 says already exists.

---

## 1. PHASE 0 — RESEARCH BEFORE YOU CHANGE ANYTHING · ~1 day · deliverable required

Do not open a component until this is written. Produce
`docs/work/UIUX_RESEARCH_20260927.md` and **stop for review**.

**Study, and cite what you actually looked at:**

1. **Evidence and case-management interfaces** — how a tool that is read under time pressure, and
   sometimes in court, presents provenance, uncertainty and partial knowledge. Look at how
   professional analysis tools handle a result that is *incomplete* versus one that is *empty*.
2. **Enterprise data-dense application shells** — navigation rails, case/workspace switching,
   breadcrumb and header hierarchy, and how density is achieved without crowding.
3. **Conversational analysis interfaces** — how a thread keeps evidence affordances (tables,
   citations, exports) instead of reducing every answer to a chat bubble.
4. **Accessible data tables at scale** — windowing while keeping real table semantics, sortable
   headers, numeric alignment.

**The deliverable is not a mood board.** For each of the seven surfaces in §4, write: what the
surface must let a person *do*, what it must never let them *believe*, the layout you propose, and
what you are deliberately not doing. Cite sources. **Where a pattern you found conflicts with §3,
say so and follow §3.**

---

## 2. WHAT ALREADY EXISTS — BUILD ON IT, DO NOT REBUILD IT

**The design system is done and is tested.** `src/styles/tokens.css` with `tokens.vitest.js`
enforcing a **7-step type scale**, **4 weights**, spacing `8/12/16/20/24/32/48`, radii `8/12/16`,
control height `42px`, content widths `1280px`/`1060px`, two themes held identical between media
and forced dark, **WCAG AA contrast asserted by test**, and Latin/Urdu/mono font roles because
evidence contains mixed strings such as `کال 03001234567 at 1035`.

> **Extend these tokens. Never weaken a contrast ratio to achieve a look — `tokens.vitest.js` will
> fail, and it is right to.** Add new tokens rather than hard-coding values in components.

Built and working as of 2026-09-27, by Claude:

| Piece | What it does |
|---|---|
| `components/PageHeader.jsx` | breadcrumbs, eyebrow, title, description, meta row, actions. **Use it on every page.** |
| `components/CaseCard.jsx` | loads its own `/collections/status`, shows readiness, families, actions |
| `lib/useCaseOverview.js` | `useCaseOverview` + `summariseCase` — **one** definition of ready / processing / families |
| `pages/InvestigatePage.jsx` | threaded conversation; turns persist, composer at the foot |
| `pages/DashboardPage.jsx` | resume list + real case cards |
| `pages/SettingsPage.jsx` | appearance, browser-storage footprint, connection facts |
| `components/VirtualizedTable.jsx` | windows 10,000 rows **while keeping table semantics** |

Already fixed — **do not re-diagnose**: evidence upload returned **403**. The workspace sent
`case_id` in the multipart form but no `X-Forensic-Case-ID` header, so `bindForensicScope` compared
a non-empty supplied case against an empty authorised one and refused. The server was right;
the client now declares the header on that one request. Pinned by
`api/forensic_records/upload_case_scope_test.go`.

---

## 3. THE RULES THAT OUTRANK ANY PATTERN YOU FIND

1. **Never simulate a backend contract.** If the service does not expose it, the UI says so. The
   Dashboard already says *"Attention data is not connected"* rather than inventing a number. That
   refusal is the product's character, not a placeholder.
2. **Never state completeness over a subset.** See the live defect in §5.1. If you render a sample,
   say it is a sample and give the true total.
3. **An abstention is a success, not an error.** Render it as a legitimate outcome with its own
   composed treatment — calm, not alarming — with the reason and a concrete "try asking…".
4. **Provenance markers are primary content.** Three states, each a glyph **and** a text label:
   `●` Source record · `◐` Candidate observation · `○` Low-confidence observation.
   **Never colour as the sole carrier of meaning.**
5. **Keep the evidence affordances.** Result grid, citations with openable exact locators,
   limitations block, export. A chat interface that hides them is a downgrade wearing a nicer coat.
6. **Real `<table>` for results.** Do not replace `VirtualizedTable` with divs.
7. **Keyboard and screen reader.** Every control reachable with a visible focus ring; `aria-live`
   on answers; the skip-link and landmark structure already pass — do not regress them.
8. **Every number on screen comes from the API.** Never compute a total the service did not give
   you, and never fill a gap with a plausible value.

### The trap that already cost a wrong answer

`requestContext()` in `lib/apiClient.js` resolves `config.collectionId || collectionIdForCase(caseId)`.
**Setting `collectionId` in runtime config PINS EVERY CASE to that collection** and the open case is
silently ignored — the page still shows the right case name. Measured: a CDR count inside
`nexusai-forensic-demo` answered with the other collection's 5,000 instead of 8,642. There is a
comment in `public/runtime-config.js` saying so. **Do not remove it.**

### The API surface — build against nothing else

    POST /query/hybrid            GET /collections/status     GET /evidence
    GET  /evidence/{id}           GET /evidence/{id}/content  GET /evidence/compare
    GET  /query/capabilities      GET /query/templates        POST /webhooks/records/upload
    GET  /images/similar          GET /faces/similar          POST /reports/generate

**There is no auth endpoint, no session endpoint, no case entity and no history endpoint.** A case
*is* a collection. Question history is `localStorage` and must stay labelled "this browser only"
until a sessions contract exists.

---

## 4. THE ORDER OF WORK — one surface at a time, each independently reviewable

Do them in this order. **Finish and verify one before starting the next.** Each ships with its
tests green and a short note on what changed and why.

    4.1  Navigation rail (sidebar)      the shell everything else sits in
    4.2  Header and case context band   identity, switching, global actions
    4.3  Dashboard                      the landing surface
    4.4  Case overview                  readiness and evidence composition
    4.5  Evidence catalog + filters     has real defects, see section 5
    4.6  Investigate thread             refine what exists
    4.7  Activity, Timeline, New case, Settings, Admin

### 4.1 Navigation rail

Today: collapsible, icons, Workspace and Current-case groups, a drawer under 768px with a working
focus trap, and a "Recent questions" list capped at 4.

Make it a **work surface**: case switching without leaving the page, thread history with pin and
rename, clear visual separation of global versus case-scoped navigation, and a collapsed state that
stays usable (tooltips, not mystery icons). Persist the collapsed preference as it already does.
**History is per-browser — label it.**

### 4.2 Header and case context band

Today: brand lockup, Quick find (Ctrl K), Display menu, and a case band with "Active case" and
"Switch case". Give it a clear hierarchy against the rail, make the active case unmistakable, and
make Quick find genuinely useful (jump to case, evidence item, or recent question).

### 4.3 Dashboard · **high priority**

Build on what exists. Add: evidence composition at a glance, processing/failed states that are
actionable, and a genuine "what can I do next". **Cross-case aggregation does not exist** — do not
imply a portfolio view.

### 4.4 Case overview · **high priority**

Today it is correct but plain: four metrics, data-quality notes, family list. Make it the page an
analyst opens to understand a case in ten seconds — composition by modality and family, readiness
including the failed item, ingest quality, and a route into asking. All of it is in
`/collections/status` already.

### 4.5 Evidence catalog and filters · **has real bugs — see §5**

### 4.6 Investigate thread

Refine the answer-card anatomy: state, headline with the number typographically dominant, result
grid, citations with provenance, limitations, follow-ups. Make the **abstention** treatment the
best-designed thing in the product.

### 4.7 The rest

`ActivityPage`, `TimelinePage` (correctly reports a missing contract — keep that honesty),
`NewCasePage`, `SettingsPage`, `AdminPage`.

---

## 5. KNOWN DEFECTS — measured 2026-09-27 on the live system

### 5.1 A completeness claim over a sample · **most serious**

On `/cases/nexusai-multimodal-product-acceptance/evidence`, `ProcessingWait` renders:

    "All 20 evidence items have finished processing"    Completed 19    Failed 1

**The case has 43 evidence items: 42 completed, 1 failed.** `ProcessingWait` reads
`data.recent_evidence` from `/collections/status`, which is a **recent-N sample**, and reports it as
the whole case. The true totals are in `summary` **in the same payload** — `evidence_total`,
`completed`, `failed` — and `CaseOverviewPage` already uses them correctly.

It also asserts *"All … finished processing"* while reporting a failure in the next line. **Failed
is not finished.** Fix the counts, and keep the four states in that component's header comment
distinct: `queued/running`, `completed`, `failed`, `not-processed`.

### 5.2 Family filter counts are unexplained and contradictory

The family chips read `All 43 · CDR 1 · IPDR 1 · ANPR 1 · Subscriber 1 · Tower 0 · Financial 1 ·
Access log 1 · Documents 2 · Images 23 · Audio 7 · Video 2`. These are **file** counts, while the
overview shows **row** counts for the same families (CDR 5,000). `Tower 0` appears although the case
has tower rows. Decide what the number means, label it, and make the zero case say why.

### 5.3 Evidence items render twice

The processing panel and the catalog list both enumerate items, with nothing explaining the
difference. Decide which is authoritative.

### 5.4 Sweep for more

Walk every route at 1440, 1280, 1024 and 768 with the console open. Check no horizontal overflow,
no duplicate DOM ids, exactly one `h1` per page, and every interactive element reachable by
keyboard. Report what you find **before** fixing it, so the list is reviewable.

---

## 6. HOW TO WORK

- **One surface per change.** A pull-sized change that touches the rail, the dashboard and the
  evidence page cannot be reviewed or reverted cleanly.
- **Tests stay green**: `npm --prefix apps/investigation-workspace run test` — 75 passing today.
  Add tests for new behaviour, especially anything that renders a count or a state.
- **Do not weaken a test to make a change pass.** If an assertion becomes ambiguous, make the
  target precise. Example: a breadcrumb `<ol>` made `getByRole('list')` ambiguous in
  `ActivityTimeline.vitest.jsx`; the fix was to give the activity list an accessible name and
  query by it — not to loosen the assertion.
- **Verify in a browser against the live API**, not against fixtures alone. Start it with
  `npm --prefix apps/investigation-workspace run dev` and open `http://127.0.0.1:4181`.
  Two real cases are configured: `nexusai-forensic-demo` (12,912 structured rows) and
  `nexusai-multimodal-product-acceptance` (43 evidence items, 10,168 rows).
- **Known-good numbers for verification**: CDR count in `nexusai-forensic-demo` is **8,642** across
  2 sources; ANPR in `nexusai-multimodal-product-acceptance` is **1,057**. If a screen shows
  something else, the scoping trap in §3 is the first thing to check.

### Environment hazards

**Bash heredocs corrupt content** — they wrote a literal `0x08` into a regex once and two NUL bytes
into a source file on 2026-09-27. Write files with an editor/tool, not a heredoc. Set env from
**PowerShell, not Git Bash**.

### Stop and report — do not proceed — if

- A change would require inventing data the API does not return.
- A change would require weakening `tokens.vitest.js`, a contrast ratio, or table semantics.
- You would need an endpoint that does not exist (sessions, auth, cross-case aggregation).
  **That is a finding, not a failure** — write it down and move to the next surface.
- Tests fail for a reason you do not understand.

### Never

Edit `api/**` or `semantic_layer/**`. `git reset --hard` · `clean` · `checkout --` · `restore` ·
`stash` · `merge` · `rebase` · `cherry-pick` · `pull` · `push` · any force operation. Bypass hooks
with `--no-verify`. Delete an untracked file you did not create.

---

## 7. DONE MEANS

An analyst can open a case, understand its composition and readiness in ten seconds, ask in a
thread, read an answer with its citations and provenance markers, open a cited source at its exact
locator, export the result, see an abstention rendered as a legitimate outcome, and browse prior
threads — **at 1440, 1280, 1024 and 768, by keyboard alone, in both themes**, with every count on
screen traceable to the API that produced it.

**And nowhere in the product does a number appear that the service did not state.**
