# WI-UI-7 — conform the analyst surface to the UX contract

You are the UI/UX track (P6) on NexusAI. WI-UI-6 is accepted: the typography,
the two designed themes, the contrast harness and the review discipline all
stand. This item makes the surface conform to the contract it was built without.

**Authority for everything below:** `docs/ux/NEXUSAI_PRODUCT_UX.md` — "this
document is the stable UX contract. Where it conflicts with an older UI report,
this wins." Every defect in §4 is a stated requirement of that document, the
app-shell specification, or the brand system. None of it is taste.

---

## 1. READ FIRST

    docs/ux/NEXUSAI_PRODUCT_UX.md                     AUTHORITATIVE. Read in full.
    docs/design/nexusai-app-shell-specification.md    shell contract, context bands,
                                                      route states, responsive table
    docs/design/nexusai-brand-system.md               identity, mark, lockup,
                                                      branding contract, governance

The last two have never been given to you. The app-shell specification is where
the chrome comes from; the brand system is where the product's name comes from.

## 2. THREE CONFLICTS — RESOLVED HERE, DO NOT RE-LITIGATE

**Typography: Noto stays. The UX doc is superseded on this one point.**
UX §8.1 says "Geist is already installed in the fork." Geist has no Arabic
coverage and the corpus contains mixed evidence (`کال 03001234567 at 1035`).
Your Noto Sans / Noto Sans Arabic / Noto Sans Mono decision is correct and is
now the standard. Record the reason next to the token so nobody reverts it.

**Palette: the UX doc governs, and it is blue.**
UX §8.2 specifies a *restrained professional blue* accent; the brand system
specifies primary `#2563EB`, accent teal `#14B8A6`, and a dark navy/charcoal
canvas. These agree with each other. WI-UI-6's petrol `#0e5e62` / cyan `#68c5c1`
agrees with neither, and its provenance is undocumented. Move to the governed
direction. **The contrast matrix must be RE-MEASURED, never assumed to survive
the change** — you may have to adjust lightness to keep every pair passing, and
that is expected work, not a failure.

**Token names: keep `--analyst-*`, but the UX doc's SEMANTIC SET is required.**
The names are yours; the vocabulary is not. UX §8.2 mandates roles your tokens
do not have at all — this is a missing feature, not a rename:

    --status-ready / --status-processing / --status-failed / --status-excluded
    --evidence-strong / --evidence-medium / --evidence-weak      ← MISSING ENTIRELY
    --surface-0..3 (4 deliberate elevation levels) · --surface-ask · --focus-ring

## 3. CROSS-TRACK NOTICE — build on these, do not clobber

The backend track built these while you were rate limited. They are yours now:

    src/pages/NewCasePage.jsx            src/components/AddDataDropzone.jsx
    src/components/ProcessingWait.jsx    src/lib/apiClient.js (uploadEvidence)
    vite.config.js (/api proxy)          public/runtime-config.js

The same-origin proxy is the only safe deployment shape: the browser holds no
credential. Do not reintroduce an `apiToken` in the served config.

You own `apps/investigation-workspace/**`, `semantic_layer/**`, `db/**`,
`ingestion/**`. **Do not touch `api/**`** — the backend track is mid-flight.

## 4. DEFECTS

### Tier 1 — the provenance system, which is the product's differentiator

**D1 · Citations are paragraph-level. The contract requires claim-level.**
UX §4.1: *"Every number, name, date and identifier in the answer sentence
carries its own marker. Not one marker per paragraph."* The shipped answer —
"8,642 CDR records across 5 call type values. Largest: Data session: 5,863; SMS:
1,108; Call: 930." — carries `[1] [2]` at the end of the whole sentence. That is
the named anti-pattern: *"citation per paragraph fudges statements, mixing
sourced claims with unsourced inferences while appearing sourced."* Every one of
those five numbers is a separate claim from a separate row set.

**D2 · Evidence strength is not expressed at all.** UX §4.1 and §8.2:
`--evidence-strong / medium / weak`, and *"a source row and a 0.31-confidence
OCR fragment must not look identical."* Today they do. Add the bands, the
`EvidenceStrengthBadge`, and convey them with **text or icon as well as colour**
(§10).

**D3 · The source rail is used for two sources.** UX §4.1: three or more
distinct sources → a right rail; *"using a rail for a single-citation answer —
visual imbalance."* At 1440 the rail appears with 2. Make the threshold real.

**D4 · No hover preview.** UX §4.1: hovering a marker shows source file,
row/page, timestamp and a ≤200-character excerpt without leaving the answer.
`CitationPreview` is in the §12 inventory and does not exist.

**D5 · A citation that cannot produce an openable locator must not render as
one.** UX §4.2 gives the one citation shape. Audit every render path against it.

### Tier 2 — the application shell

**D6 · The product has no identity.** The header reads "Investigation
Workspace". The brand system states ordinary analyst identity is **NexusAI**,
delivered as a code-native mark and horizontal lockup plus the served
`/favicon.svg`, overridable by customer config (`instance_name`,
`instance_tagline`, `logo_url`, `logo_horizontal_url`, `favicon_url`).
Governance limits: the primary tagline belongs on identity-led surfaces, **not
repeated inside dense analytical work**; "Enterprise Forensic Intelligence" must
**not** appear in global navigation.

**D7 · One context band where the spec requires three.** Global (identity,
tenant, user, role, policy, notifications, long-running operations) · Case (case
ID/name, classification, status, permissions, evidence readiness, processing
summary — case routes only) · Task (page title, actions, filters). Today the
case ID is a monospace string in a corner; there is no user, role or
classification. Case switching must change the URL and clear incompatible
case-scoped state **before** any new request begins.

**D8 · Navigation is missing its top level and its drawer.** UX §5 GLOBAL is
Dashboard · Cases · Activity · Settings · Admin(gated). There is no Dashboard
(§6.1: *"what should I do next?"* in under five seconds) and no Settings. The
shell spec requires a **persistent collapsible rail** at 1024/1440 and a **modal
drawer with focus trap and focus return** at 390 — today 390 renders a wrapped
chip row and no drawer exists. The orphan strings at the rail's foot
("Evidence-scoped analysis", "4 evidence items") are case context and belong in
the case band.

**D9 · Evidence families are not filters anywhere.** UX §5: *"Evidence families
are filters, never navigation"* — chips inside Evidence and Investigate, with
the exact vocabulary CDR · IPDR · ANPR · Subscriber · Tower · Financial · Access
log · Documents · Images · Audio · Video. `ScopeChips` does not exist. UX §4.1
adds: scope controls **persist across the session** and **fail loudly** — when
they match nothing, say so, never silently widen.

**D10 · The theme control sits in the user's slot, and it forgets.**
`ThemeControl.jsx` writes `dataset.theme` from state and persists nothing, so
**every reload discards the analyst's choice**. Persist per viewer in
`localStorage`, wrap every read and write in try/catch, render correctly when it
throws, and keep `prefers-color-scheme` as the default when nothing is stored.
Move it into an account/preferences control (design-system v2 pairs it as
"account/theme control").

### Tier 3 — answer surface and states

**D11 · The finding is typeset as a slide title.** A whole sentence at 2rem
wrapping three lines. A forensic finding has a hierarchy: the measured quantity,
the thing measured, then the qualifying breakdown.

**D12 · Citation markers render their brackets** — `[1]` inside a pill,
baseline-misaligned. A chip is already a marker.

**D13 · "Read the analysis method" has no control affordance.** UX §6.6 item 4:
*"HOW THIS WAS DERIVED — collapsed by default"*, plain language on expand. Give
it button semantics, an expanded state and an indicator.

**D14 · Route states are incomplete, and a disabled control is shipped.** Every
route must supply `loading`, `ready`, `empty`, `partial`, `error`, `forbidden`,
`unavailable`; partial retains completed artifacts and names the failed scope;
empty is legal only after the authoritative request resolves. TimelinePage ships
`<select disabled>`, which violates UX §9 outright: *"A control that is not
implemented is not shown — not shown-and-disabled, not shown-with-a-tooltip."*

**D15 · An internal key is title-cased into a fake label.** Evidence families
renders **`Cdr`**. The backend curates display names — WI-13 fixed this exact
class on the answer side (`GPRS` → "Data session"). Render the curated label
where one exists, the raw identifier **unchanged** where none does, and never
manufacture a label by title-casing an identifier.

**D16 · Overview sets a slug as a display title** (`nexusai-forensic-demo`,
monospace, display size) and gives all four metric cards the same accent rule,
so none reads as primary. UX §8.3 also forbids **card soup**. UX §6.3 requires
**data-quality notes as a first-class panel** — concrete sentences like *"425 CDR
rows list 923001110001 as its own counterparty"*, not only four counters.

**D17 · Bare native `<select>`** in ThemeControl, ActivityPage and TimelinePage.

## 5. CONSTRAINTS THAT DO NOT MOVE

- Never invent evidence, chronology, a percentage for processing, or a label for
  an uncurated value. Zero / unavailable / unprocessed stay distinct.
- Every state uses icon + label + description. Colour never carries meaning alone.
- CNIC masking is enforced **server-side at projection**. The UI never unmasks
  and never adds masking of its own.
- No model, backend, agent, operation, template, SQL, embedding, confidence
  threshold or token count on any analyst surface (UX §9).
- Forbidden (UX §8.3): flat white canvas · flat black canvas · purple AI
  gradients · neon/cyberpunk · glassmorphism · marketing hero blocks · card soup
  · decorative motion · emoji as status · progress bars that do not reflect real
  progress. Motion ≤200 ms, respects `prefers-reduced-motion`.
- Logical properties throughout; WI-UI-3's RTL mirroring must still hold, and
  mixed LTR/RTL evidence keeps `dir="auto"` / `<bdi>` isolation.
- Port, do not rewrite, `analyst*Presentation.js` and their tests.

## 6. ACCEPTANCE — measured, not asserted

1. Re-measured contrast matrix for the governed palette, both themes, enforced
   by `tokens.vitest.js`. Text ≥4.5:1; components and focus ≥3:1.
2. Screenshots at 390 / 820 / 1024 / 1440 × both themes for overview, evidence,
   investigate-answered, clarification, zero, failed, activity, type-proof,
   **plus drawer-open, rail-collapsed, and a claim-level-citation close-up**.
3. **No horizontal page scroll at 375px** (UX §13 — you tested 390, the contract
   says 375). Touch targets ≥44px. Tables pin their first column at 768–1279.
4. A recorded keyboard-only pass: skip link → open drawer → focus trapped →
   Escape → focus returned to the trigger; then the full core loop of UX §13.
5. Theme choice survives reload, and the app renders correctly when storage throws.
6. Every route's seven states rendered from fixtures and shown in the review.
7. Ported, unit and Playwright suites green — Playwright covering clarify,
   zero-result and unsupported, not only the happy path. Production build clean.

## 7. A MEASURED DEFECT IN YOUR OWN TRACK — `semantic_layer/**`

Found offline by the backend track today, handed over rather than edited,
because that directory is yours:

**`scripts/ir_spike/gold_plans.json` is stale for CDR-08 and ANPR-05.** Both
carry `gold: null` with `inexpressible: "No distinct-count aggregate in the
IR."` **COUNT_DISTINCT has existed since WI-9**, and the reconstructed
one-measure plan validates against the issued enum for both today
(`bucket_a_conformance_test.go`). Write the two gold plans:

    CDR-08   COUNT_DISTINCT over cdr.msisdn
    ANPR-05  COUNT_DISTINCT over anpr.plate_number

Also inert: `cdr.distinct_subscribers` and `anpr.distinct_plates` declare
`aggregate: COUNT_DISTINCT` + `field_id:` but no `expression:`, and
`CatalogFields` skips any metric without one — so those declared metrics never
become catalogue fields. Either give them an executable form or delete them; a
declaration that silently does nothing is worse than its absence.

**Do not "fix" SUB-03.** Its gold expects the literal CNIC `00000-1000001-1`,
and CNIC is masked server-side by product rule. Satisfying that gold means
leaking a CNIC. The gold is wrong.

## 8. REPORT BACK

What you changed, what you measured, and **anything in the three documents you
could not honour because the endpoint does not exist.** There is no case entity
and no create-case endpoint; a collection comes into existence when its first
file is accepted. If classification, role, tenant or Dashboard data have no
source, **say which endpoint is missing and leave the field absent.** Do not
render a plausible value — an invented role badge or a fabricated "needs your
attention" count is a forensic defect, not a placeholder.
