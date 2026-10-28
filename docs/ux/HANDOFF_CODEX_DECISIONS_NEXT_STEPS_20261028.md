# NexusAI hand-off pack: Codex prompt, decisions waiting, next steps

**Date:** 2026-10-28 · **Repository state:** branch `lane-arms`, head `cff4339`, pushed to `origin claude/gifted-pasteur-4kpx0r` · **Live system:** unchanged since 2026-10-26 (API image `59d033208817`, rollback tags `before-pair-direction-20261026` and `before-engine-fixes-20261026`).

This one file has three parts: **A**, the complete prompt to paste into Codex, with the exact steps to start it; **B**, every decision that waits for you, each with my recommendation, what your answer unlocks, and a one-line way to reply; **C**, the next steps in a safe order, who does each and what is never done without your explicit go. **D** is a quick reference.

---

# Part A. Codex: the interface programme

## A.1 Before you paste (about 10 minutes, four steps)

1. **Answer O-1 and O-2** (Part B, UI decisions). Codex's first stage does not start building until they are answered; until then it writes only its research note.
2. **Create Codex's own worktree** (recommended answer to O-2), from a PowerShell opened at the repository root `C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork`:

```powershell
git worktree add ..\NexusAI-ui -b ui/u1-foundations lane-arms
```

3. **Give the new worktree the API key file** without printing it: copy `.env.forensic-runtime.local` from the repository root into `..\NexusAI-ui\` (it is ignored by git; Codex must never read, print or commit it), then install the interface's dependencies there and start its server on port 4182:

```powershell
Copy-Item .\.env.forensic-runtime.local ..\NexusAI-ui\.env.forensic-runtime.local
npm --prefix ..\NexusAI-ui\apps\investigation-workspace install
npm --prefix ..\NexusAI-ui\apps\investigation-workspace run dev -- --port 4182 --strictPort
```

   You then look at the new interface at `http://localhost:4182` and the current one at `http://localhost:4181`, side by side.
4. **Open Codex at `C:\Users\sheik\Workspace\Office\Projects\NexusAI-ui`** and paste the prompt below. (If you chose answer (a) for O-2, open Codex at `NexusAI-localwork` instead and delete the two worktree lines from the prompt.)

## A.2 The prompt (paste everything inside the box)

```text
ROLE
You are the interface owner of NexusAI, a case-centric forensic investigation workspace used by analysts with no
technical training: they open a case, add evidence, ask in plain words, and receive an answer in which every number
opens its source row. It is an instrument used for hours and read in courtrooms: it must look like a record, never
like a toy or a marketing page.

THE OWNER'S REQUEST, IN HIS WORDS (2026-10-26)
"The best of the best, world class, clean, modern, elegant, up to date, professional, enterprise grade, accurate,
functional, correct and productive ... starting from the main theme, colours, CSS, shell, desktop rail, sidebar,
header, navbar, main dashboard, hero section ... then for every section and area deep research and the best layout,
spacing and implementation ... charts, graphs, figures, maps ... transitions, effects ... every minor and major
thing. Research online first for the best, most relevant examples and the latest guidance, and merge it all into one
main implementation."
That request is already turned into ONE directive. You execute it, one stage at a time.

READ, IN THIS ORDER, AND NOTHING ELSE FIRST
 1. AGENTS.md and .agents/CODEX.md (guardrails, scopes, commit trailers).
 2. docs/ux/MASTER_UI_UX_DIRECTIVE.md  <- THE directive (19 sections). Read sections 0 to 3, 15, 16, 18 now; then the
    sections named by the card of the stage you are given. Section 17 says what was kept and what was rejected from
    every older UI brief: do NOT read or follow those older briefs (they carry a SUPERSEDED banner).
 3. docs/ux/NEXUSAI_PRODUCT_UX.md (the UX contract; it binds, except where the directive's section 4.9 proposes an
    amendment AND the owner has accepted it: decision O-1).
 4. docs/ux/research/RESEARCH_REGISTER_20261026.md (sources with V / S / K confidence marks; you ADD rows, never delete).
 5. The newest entry of NEXUSAI_CONTINUATION.md only, and its section CODEX_UI_TRACK (the file is long; do not read it all).
 Do NOT read or act on docs/work/WI-*, CODEX_BACKLOG_*, or any engine or data document: they are not interface directives.

WHERE YOU WORK
 - Your own git worktree on branch ui/u1-foundations (repository one level up: ..\NexusAI-localwork). Commit only there.
 - Dev server for your branch: npm --prefix apps/investigation-workspace run dev -- --port 4182 --strictPort
   (the owner starts it; it forwards /api to the live service with a key it reads from a gitignored file you must
   never open, print or commit).
 - The owner compares http://localhost:4182 (yours) with http://localhost:4181 (the accepted interface).

SCOPE
 - You may edit: apps/investigation-workspace/**, docs/ux/**, and append rows to docs/work/BACKEND_REQUESTS.md.
 - You may NOT edit: api/**, db/**, ingestion/**, tools/**, evaluation/**, semantic_layer/**, docs/data/**,
   docs/roadmap/**, core/**, or anything outside the scope above. Claude owns those and edits no interface path while
   you own a stage.
 - Real case data never enters a screenshot, a fixture, a file, or a prompt to any outside tool. Use the generated
   synthetic cases only (for example the collection nexusai-wave1-dev2-20261021). Outside design tools get synthetic
   content only.
 - Never read, print, copy or commit .env.forensic-runtime.local or any secret.

WHAT EXISTS TODAY (so you do not rebuild it)
 - React 18, Vite 6, React Router 7 (code-split routes), ECharts 6 (one wrapper), Cytoscape 3, MapLibre GL 6 with an
   offline basemap, Lucide, Noto Sans fonts. 75 vitest files / 549 tests, 26 Playwright specs.
 - About 618 KB of CSS in nine files of three generations ("Identity A" navy-teal chrome, near-white cards); the owner
   dislikes that look: it is too "AI/techy" and the header does not match the page.
 - Routes: / (Dashboard), /investigate, /cases, /cases/new, /cases/:id/{overview,evidence,evidence/:eid,investigate,
   connections,timeline,activity,data}, /activity, /settings, /design/*.
 - The link-analysis workbench (layout B, chosen by the owner) is live on Connections and inside Ask answers
   (src/pages/workbench/**, src/lib/workbench/**). Its logic and tests are frozen inputs; its look is in scope at U3.
 - The data hooks and presentation logic and their tests are frozen inputs (directive rule 9): replace markup and CSS
   stage by stage, never leave a page half-migrated.

YOUR FIRST STAGE: U1 FOUNDATIONS (directive section 16, card U1)
 Method (directive section 15, binding): 1 research -> 2 two or three options built in the gallery -> 3 the owner
 decides by looking -> 4 build -> 5 prove -> 6 close -> 7 owner review.
 Deliver, and nothing else in U1:
  a) docs/ux/research/U1_RESEARCH_NOTE.md: sources for every foundation area (colour, type, spacing, depth, motion,
     density, CSS architecture, iconography), each with its V/S/K mark; add rows to the register; say what you rejected.
  b) design/tokens.json (Design Tokens Community Group format), a small generator (plain Node, no new dependency) and
     styles/tokens.generated.css; the cascade-layer architecture (@layer reset, tokens, base, layout, components,
     patterns, pages, utilities); an ALIAS LAYER mapping every old --analyst-* name to the new --nx-* names so that every
     existing page keeps working untouched.
  c) The route /design/foundations: palette ramps, surfaces, text, status, evidence-strength, data colours, type
     specimen (Latin, Urdu, tabular figures, mono), spacing, radius, depth, density modes, icons, motion samples with the
     reduced-motion switch, a GENERATED contrast table, a colour-blind simulation, and the SAME THREE SYNTHETIC SCREENS
     (a Dashboard with its hero band and metric cards; an Ask answer with the workbench inside it; a Case Overview) in
     TWO colour directions (A "Teal instrument", B "Ink and signal") x light and dark x 1440 and 390 px wide.
  d) The tests and measurements the directive's section 14 asks for: contrast of every used token pair, no raw colours /
     no !important / no duration literals in component CSS, CSS size budget, baseline bundle and INP numbers of three
     interactions.
  e) A stage note and the update of the CODEX_UI_TRACK section of NEXUSAI_CONTINUATION.md.
 Delete nothing in U1 except what the owner has told you to; the old pages keep working through the alias layer.
 If O-1 or O-2 is not answered when you start: write (a) only, make no change under apps/, and report.

DEPENDENCIES
 Only the approved list (directive rule 10). Anything else (the proposals P-1 to P-8 of section 13.2) needs the
 owner's explicit yes, asked with the trade-off written. Dev-only tools the owner has approved may be used.

RULES THAT NEVER CHANGE (short form of directive section 2)
 - Real service data only; never simulate a missing backend. A read the service does not give is a designed
   "not available yet" state plus a row in docs/work/BACKEND_REQUESTS.md (endpoint, exact fields, page, empty/error
   behaviour, privacy).
 - No fabricated trends, deltas or decorative numbers. Evidence strength (Observed / Inferred / Candidate) is always a
   word AND a shape, never colour alone.
 - No analyst-facing jargon: never model, agent, SQL, query, embedding, threshold, pipeline, ingest.
 - Design first: no stage starts by editing CSS; two or three options are built in the gallery, the owner chooses.
 - Forbidden looks: flat white or flat black canvas, purple AI gradients, neon, glass and blur-behind panels,
   decorative gradients behind data, marketing hero blocks, card soup, emoji status, looping or attention-seeking
   motion, hover lift, more than one accent colour.
 - Accessibility is a requirement: WCAG 2.2 AA in both themes, keyboard complete, screen-reader true, colour never the
   only signal, reduced motion honoured, Urdu and mixed-direction text handled with dir="auto".
 - Tests stay green and are never weakened to pass:
     npm --prefix apps/investigation-workspace run test
     npm --prefix apps/investigation-workspace run build

COMMITS
 `Assisted-by: Codex:<your model> [tools]` and the session trailer of AGENTS.md. NEVER Co-Authored-By for an AI, never
 Signed-off-by from an AI. git add <explicit paths> only (never -A or .). No force push, no history rewrite, never
 --no-verify. Plain push of your own branch only. Small coherent commits.

ASK THE OWNER FIRST BEFORE
 a new dependency, a container rebuild or deploy, any database write or migration, a model download, anything touching
 retained evidence, a long build.

STOP AND ASK WHEN
 a rule conflicts with a real constraint; a design needs a backend that does not exist (file the request, show a
 designed state, continue); a test would need weakening; real data would be needed to make something look right;
 a decision of directive section 18.2 is still open and your stage depends on it.

REPORT at the end of each work session, in ten lines: stage, what was built, what was removed, proof (tests, budgets,
accessibility), what the owner should look at (route and port), open decisions, next step.
```

---

# Part B. Decisions waiting for you

**How to reply in one line.** Copy the line at the end of each group and edit it, for example:
`UI: O-1 yes, O-2 b, O-4 evolve, O-5 yes, O-7 yes as recommended, O-8 I will check, O-9 yes at acceptance, O-10 PKT. ENGINE: W-1 go, P-1 hold, P-2 hold, P-3 later. REVIEWS: E-3 names: ...`
A decision you do not mention stays open and nothing depending on it starts. **"Yes to all recommendations" covers only the items marked ✔ below; the items marked ⚠ change the live system and are always asked on their own.**

## B.1 Interface decisions (needed before or during stage U1)

| # | Decision | Options | My recommendation | What your answer unlocks |
|---|---|---|---|---|
| **O-1** ✔ | Accept the proposed amendments to the UX contract §8 (visual system) and §11 (responsive): tokens renamed `--nx-*`, three-state themes kept, a high-contrast theme, density modes, new breakpoints, motion ladder | yes / no / partly (name the parts) | **Yes.** They are the redesign you asked for; the contract's forbidden looks stay | U1 may build anything that differs from the contract. Without it Codex writes its research note only |
| **O-2** ✔ | How Codex works | (a) in the lane tree with path-scoped commits; (b) **in its own git worktree and branch, served on port 4182**, merged into `lane-arms` when you accept a stage | **(b).** Two agents committing in one tree contend for the git index and the commit hook; a worktree is still one repository; you compare :4182 with :4181 | Where Codex is opened (Part A.1) |
| **O-3** | Colour direction: A "Teal instrument" (continuity with D23, refined teal) or B "Ink and signal" (near-neutral ink, deeper blue-teal accent) | A / B / mix | **Decide by looking** at `/design/foundations` when U1 is ready | Fixes the palette for every later stage |
| **O-4** ✔ | Brand mark | keep the code-native mark / evolve it lightly to the chosen palette | Evolve lightly | The header and rail mark |
| **O-5** ✔ | Density: compact tables by default (your 2026-10-12 choice) and default spacing for pages | confirm / change | Confirm | Table and page defaults |
| **O-6** | Urdu: Nastaliq for evidence text with Sans Arabic for labels, or one Naskh/Sans face; needs a native reader | the pair / one face | Nastaliq for evidence text, Sans Arabic for labels, **checked by a native reader** (needs names, E-3) | The font loading in U6 |
| **O-7** ✔ | Dependencies: P-1 React Aria Components for six complex widgets, P-3 `@axe-core/playwright` (dev), P-4 `culori` (dev), P-5 `stylelint` (dev); P-2 TanStack Table, P-6 a Nastaliq webfont, P-7 sigma.js, P-8 Storybook / UI kits | yes/no each | **Yes to P-1, P-3, P-4, P-5. No to P-2, P-6, P-7, P-8 for now** | Codex may add exactly those |
| **O-8** | The offline map tiles: source, licence and the attribution it requires (unverified) | you tell me the source / I trace it | I will trace it from the tile files and report; confirm the licence before any screenshot leaves the machine | The attribution line on maps |
| **O-9** ✔ | Delete `/connections/classic` and the old `ConnectionsPage` once you accept the workbench | yes at acceptance / keep | Yes, at acceptance | A smaller code base at U3 |
| **O-10** ✔ | Case time zone shown with every time | PKT default now, per-case setting later / per-case now | PKT now, per-case later | U4 |
| **O-11** | Urdu interface (all labels translated) | now / later | Later | Nothing yet |
| **O-12** | Real analysts for the unaided usability test (the last stage, U10) | names | Name 3 to 5 when you can | The acceptance test of the whole interface |

## B.2 Engine and data decisions (Claude's side; nothing here is applied yet)

| # | Decision | My recommendation | What your answer does |
|---|---|---|---|
| **W-1** ⚠ | **Deploy the answer-wording changes** (Amendments 5 and 6: a no-row answer says what was searched; a score is "a candidate, not proof"; a stated conclusion is corrected; one alert, move and cell visit are sentences) **together with the plate code**, which is inert on the live database (checked 2026-10-28, read-only: the plate table exists, is empty, and the plate views do not exist; plates are read from a question only where the catalogue has the plate views, commit `cff4339`) | **Go.** Measured twice (20 of 20 correct on fresh cases 113 and 114, 0 wrong); numbers do not change, only the first sentence; reversible by one rollback tag | I tag the current image `before-wording-20261028`, cross-build, build, recreate the API with the current switches (ASK_V2 and typed families as now), run the live smoke (5 questions) and the F4 questions, and report. Rollback is one command |
| **P-1** ⚠ | Apply migration 027 (the two plate views) to the live database | **Hold.** It is additive and rehearsed, but nothing in the live system holds a camera export, so it would answer nothing yet. Apply it together with P-2 | Nothing changes today |
| **P-2** ⚠ | Register the draft layout `anpr-a1` in the live layout table and deploy the worker so a camera CSV can be read, then upload a **synthetic** camera export to a retained synthetic case to demonstrate plate questions in the live interface | **After W-1 and your look at the workbench (E-2).** A demonstration on synthetic data only | I would ask again with the exact steps (backup, migration 027, layout, worker overlay, synthetic upload, smoke) |
| **P-3** | A **real** camera export (any ANPR CSV or XLSX you can share) read in the vault, counts and booleans only, to confirm or replace the draft layout | **Yes, when you have one.** Without it the layout stays "designed, not observed" | I run the vault tools (`profile_structure`, `confirm_layout`) and report which columns, time format and confidence scale it uses |
| **P-4** | Next typed families after plates (design-first, same gates as plates): subscriber / SIM / device reference, then cell / tower reference, then access logs and transactions; message exports first among the new families (D29) | Continue in that order, one family at a time, each with its own pre-registered gates and a fresh judgement case | I start the next family's design document |
| **P-5** | Speed of pair questions (70 to 117 s because of two model calls): put a worked example of the pair shape in the prompt | Optional; do after W-1 | A measured change on a fresh case |

## B.3 Reviews and information only you can give

| # | Item | Why it matters | How |
|---|---|---|---|
| **E-1** | Your marks on the 15-answer review page (https://claude.ai/artifact/2scXefWBMKu2HvFuHk7HZA). Any answer you mark as overclaiming reopens that analysis | I checked all 15 against the records myself (all agree; 9 thin in wording). The wording changes of W-1 address those 9 | Open the page and mark |
| **E-2** | Look at the workbench on `http://localhost:4181` (Connections and inside Ask): I checked the dark theme only, not the light theme or a phone width | It is the visual starting point of U3 | Tell me what you dislike, in a sentence |
| **E-3** | Names: investigators who will write the 50 test questions (E1, D26) and language reviewers (D10: Urdu, Punjabi, Pashto) | The honest accuracy figure and the Urdu rules cannot be finished without real readers | Names or roles |

---

# Part C. Next steps, in a safe order

## C.1 Today, nothing waits on a decision

| # | Step | Who | Safety |
|---|---|---|---|
| 1 | Read this file; send me the reply line of Part B | You | none |
| 2 | Create Codex's worktree and start it (Part A.1) once O-1 and O-2 are answered | You | the worktree is a separate branch; the live system is not touched |
| 3 | Keep every live step on hold | Me | nothing is applied until you say go |

## C.2 After your reply

| Trigger | Step | Who | Gate / proof | Rollback |
|---|---|---|---|---|
| **W-1 go** | Deploy the wording and plate code (recipe: tag the running image, cross-build the Go binary, rebuild the image `FROM` the rollback tag with the new binary and `semantic_layer`, recreate the API only through the Run-Arms helper, never start `nexusai-api-1`) | Me | live smoke 5 of 5; the four pair questions answer 4, 4, 2, 7; the first sentences read as designed; then a short note in `NEXUSAI_CONTINUATION.md` | `docker tag ...:before-wording-20261028 ...:latest` and recreate |
| **O-1, O-2 answered** | Codex runs stage U1 (research note, tokens, layers, gallery with both colour directions) | Codex | directive section 14 (tests green, budgets, contrast table, a11y checks, screenshots on synthetic data) | the branch is not merged until you accept |
| **U1 ready** | You look at `/design/foundations` on :4182 and choose O-3 | You | by looking | none |
| **O-3 chosen** | Codex stage U2 (shell, rail, header, command palette); then U3 (workbench restyle), U4 (Ask), U5 (Dashboard with the hero, Cases, Overview), U6 (Evidence and viewers), U7 to U9 as backends land, U10 (polish and usability test) | Codex, you accept each on :4182 | one stage at a time, never two writers | revert the stage's commits |
| **P-3** | Real camera sample read in the vault | Me, with you providing the file | counts and booleans only | none |
| **P-2 go** (later) | Live demonstration of plates on a synthetic case | Me | backup, rehearsal already done, smoke | migration 027 rollback script, worker rollback tag |

## C.3 Parallel work Claude can do without live changes (in this order)

1. **Next typed family** (P-4): design document and gates first, then layout, writer, view, engine and bank on a throwaway database, as for plates.
2. **Speed** (P-5) and the **multi-move sentence** (a list of moves counted, dates in the table) if you want the dates in the sentence.
3. **Remaining unopened judgement cases: 117 to 120**, kept for the next fixes and families; sealed cases 901 to 904 stay sealed.
4. **Backend requests** that Codex files (BR-1 case summary and attention feed, BR-2 timeline events and density, BR-3 annotations and notes): each a fixed read over a reviewed view, with a spec equal to the oracle.

## C.4 Never done without your explicit go

Any deploy or container rebuild; any database write, migration or backfill (migration 027 included); registering a layout in the live table; a model download; changing worker roles or the typed-families switch; opening the sealed cases; putting real case values in a screenshot, a repository file or any cloud tool; `Co-Authored-By` for an AI; force-push or `--no-verify`.

---

# Part D. Quick reference

| Topic | Where |
|---|---|
| The interface directive | `docs/ux/MASTER_UI_UX_DIRECTIVE.md` (stages in section 16, merge ledger 17, decisions 18) |
| Research register | `docs/ux/research/RESEARCH_REGISTER_20261026.md` |
| The UX contract | `docs/ux/NEXUSAI_PRODUCT_UX.md` |
| Codex start prompt (earlier, shorter copy) | `docs/ux/CODEX_UI_START_PROMPT.md` (this file's Part A is the current one) |
| Answer wording: design, gates, runs | `docs/data/link-analysis-stage3-preregistration-20261022.md` (Amendments 5 and 6, run log) |
| Plates (ANPR): design, gates G1 to G6, results | `docs/data/anpr-typed-family-20261026.md` (status in sections 6 to 9) |
| Current state of the project | `NEXUSAI_CONTINUATION.md` (sections 9 and CODEX_UI_TRACK) |
| Plan of record | `docs/roadmap/master-plan-20261021.md` |

**Results to remember.** Wording: judgement cases 113 and 114, 20 of 20 correct each, wording gate 0 failures after reading past the correction. Plates: development cases 1 to 8, views equal the generator's answer key (1,994 rows loaded, 1,933 unique sightings, 366 camera pairs, 480 camera-days, 24 look-alike pairs, zero differences); judgement case 115 13 correct, 0 wrong, 1 abstained (a year-like plate, fixed); case 116 15 of 15 correct, 0 wrong; 316 Ask-engine tests pass. Live: nothing from these changes is deployed.

**Checks you can run yourself** (PowerShell at the repository root):

```powershell
python -X utf8 tools/case_generator/test_plate_oracle.py --cases 2
python -X utf8 tools/ask_eval/test_wording_gate.py
npm --prefix apps/investigation-workspace run test
```
