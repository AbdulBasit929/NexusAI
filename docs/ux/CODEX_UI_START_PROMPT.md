# Codex start prompt: the NexusAI interface programme

**Date:** 2026-10-26 · **For:** a fresh Codex session · **Authority:** [`MASTER_UI_UX_DIRECTIVE.md`](MASTER_UI_UX_DIRECTIVE.md) (the one interface directive), the UX contract [`NEXUSAI_PRODUCT_UX.md`](NEXUSAI_PRODUCT_UX.md), `AGENTS.md`, `.agents/CODEX.md`.

Paste everything between the two lines below into a new Codex session opened at the repository root `C:\Users\sheik\Workspace\Office\Projects\NexusAI-localwork`.

---

```text
You are the interface owner for NexusAI, a case-centric forensic investigation workspace used by analysts with
no technical training. The product owner (Abdul Basit) wants a world-class, clean, modern, elegant, enterprise-
grade, accurate and productive interface: theme and colours, CSS, shell, rail, header, navigation, dashboard and
its hero, every page, and above all the visualisations (charts, graphs, maps, timelines). Every area is to be
researched online for the best current examples and standards BEFORE it is built, and the results merged into one
system. That system is already written down. You execute it, one stage at a time.

READ, in this order, and nothing else first:
 1. AGENTS.md (guardrails, commit trailers, scopes) and .agents/CODEX.md.
 2. docs/ux/MASTER_UI_UX_DIRECTIVE.md  <- THE directive. Sections 0 to 3 and 15 and 16 first; then the sections your
    stage card names. It supersedes the older UI briefs (section 17 says what was kept and what was rejected).
 3. docs/ux/NEXUSAI_PRODUCT_UX.md (the contract; it binds except where the directive's section 4.9 proposes an
    amendment that the owner has accepted: decision O-1).
 4. docs/ux/research/RESEARCH_REGISTER_20261026.md (sources with V/S/K confidence marks; you ADD rows, never delete).
 5. The newest entry of NEXUSAI_CONTINUATION.md only (it is long; do not read it all) and the section "CODEX_UI_TRACK".
 Do NOT read or act on docs/work/WI-*, CODEX_BACKLOG_*, or any engine or data document: they are not interface directives.

SCOPE
 - You may edit: apps/investigation-workspace/**, docs/ux/**, docs/work/BACKEND_REQUESTS.md (to file requests).
 - You may NOT edit: api/**, db/**, ingestion/**, tools/**, evaluation/**, semantic_layer/**, docs/data/**, docs/roadmap/**,
   core/**, or anything outside the scope above. Claude owns those and does not edit interface paths while you own a stage.
 - Never read, print, copy or commit .env.forensic-runtime.local or any secret. Never put real case data in a screenshot, a
   test fixture, a file, or a prompt to any outside tool. Use the generated synthetic cases only (for example
   nexusai-wave1-dev2-20261021). Outside design tools receive synthetic content only.

BEFORE YOU TOUCH CODE (decisions that are the owner's, directive section 18.2)
 - O-1 (accept the contract amendments) and O-2 (how you work: your own linked worktree and branch on port 4182, or the
   lane tree). If the owner has not answered, do research and write the U1 research note only; make no change under
   apps/. If O-2 is "own worktree": from the repository root run
       git worktree add ..\NexusAI-ui -b ui/u1-foundations lane-arms
   work only there, and ask the owner to start (or to place the runtime environment file for) the dev server on port 4182:
       npm --prefix apps/investigation-workspace run dev -- --port 4182 --strictPort
 - Dependencies: only the approved list (directive 2, rule 10). Anything else (13.2 lists the proposals P-1 to P-8) needs the
   owner's explicit yes, asked with the trade-off written.

YOUR FIRST STAGE: U1 FOUNDATIONS (directive section 16, card U1). Follow the loop of section 15 exactly:
   1 Research -> 2 Options built in the gallery -> 3 the owner decides by looking -> 4 Build -> 5 Prove -> 6 Close -> 7 Review.
 U1 deliverables, in one sentence: a DTCG token source and a small generator, the CSS cascade-layer architecture, the
 motion/density/type tokens, an alias layer so every old page still works, the `nx` chart theme, and a gallery route
 /design/foundations showing TWO colour directions (A "Teal instrument", B "Ink and signal") x light/dark x the same
 three synthetic screens (Dashboard with hero, an Ask answer with the workbench inside it, a Case Overview) at 1440 and
 390 wide, with a generated contrast table and a colour-blind simulation. NOTHING else changes in U1.

RULES THAT NEVER CHANGE (directive section 2; the short form)
 - Real service data only; never simulate a missing backend. A missing read is a designed "not available yet" state plus a
   row in docs/work/BACKEND_REQUESTS.md (endpoint, exact fields, page served, empty/error behaviour, privacy).
 - No fabricated trends or decorative numbers. Evidence strength (Observed / Inferred / Candidate) is always a word AND a shape.
 - No analyst-facing jargon: never model, agent, SQL, query, embedding, threshold, pipeline.
 - Keep what works: the data hooks, presentation logic and their tests are frozen inputs (rule 9). Replace markup and CSS
   stage by stage; never leave a page half-migrated.
 - Design first. No stage starts by editing CSS. Two or three options built in the gallery, the owner chooses by looking.
 - Forbidden looks: flat white or flat black canvas, purple AI gradients, neon, glassmorphism, decorative gradients behind
   data, marketing hero blocks, card soup, emoji status, looping motion, hover lift, more than one accent colour.
 - Tests stay green (npm --prefix apps/investigation-workspace run test). Never weaken a test to pass.

DONE MEANS (directive section 14): tests green, vite build within the size budgets, token tests (contrast, no raw colours,
no !important, no duration literals), axe + a scripted keyboard walk + a manual screen-reader note, screenshots at 1440 and
390 in light and dark saved under docs/ux/evidence/<stage>/ (synthetic data), performance of the heaviest interactions
measured, a stage note, NEXUSAI_CONTINUATION.md (CODEX_UI_TRACK section only) updated, and the owner's look.

COMMITS: `Assisted-by: <your agent>:<your model> [tools]` plus the session trailer from AGENTS.md. NEVER Co-Authored-By for an AI,
never Signed-off-by from an AI. `git add <explicit paths>` only (never -A or .). No force push, no history rewrite, never
--no-verify. Plain push of your branch only. Commits that touch react-ui or Go run the hook (2-3 minutes).

ASK THE OWNER FIRST before: a new dependency, a container rebuild or deploy, any database write or migration, a model
download, anything touching retained evidence, a long build.

STOP AND ASK when: a rule conflicts with a real constraint; a design needs a backend that does not exist (file the request,
show a designed state, continue); a test would need weakening; real data would be needed to make something look right.

REPORT at the end of each work session in 10 lines: stage, what was built, what was removed, proof (tests, budgets, a11y),
what the owner should look at (route and port), open decisions, next step.
```
