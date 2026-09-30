# Spec: shell frame and header (Stages 1 and 2, built 2026-09-29)

Files: `src/components/CaseShell.jsx`, `src/styles/shell.css` (owns the frame and header), `identity-a.css` (sidebar colours, badges), `tokens.css`.

## Research that decided it
- **Linear** redesign: header and sidebar read as one "inverted L" chrome; blue chrome was deliberately reduced for a neutral, timeless look; app header and view header were simplified and standardised. Decision: one continuous dark chrome, teal used only as the interaction accent, no cobalt.
- **Primer PageLayout**: sidebar is a full-height region beside the banner; below 768 px it becomes a full-screen overlay; header and content need labels. Decision: sidebar starts directly under the header on every page and the mobile drawer is a modal overlay.
- **WAI-ARIA landmarks**: one banner, one main, unique labels on repeated navigations, search as its own landmark. Decision: header is the banner, the search wrapper is `role="search"` named "Workspace search", sidebar navigation is "Primary". Covered by `CaseShell.vitest.jsx`.
- **Vercel Geist colour steps**: rest, hover, active as consecutive steps. Decision: rail rest transparent, hover `--analyst-rail-hover`, active `--analyst-rail-active` plus a teal marker (colour is never the only cue: the active link is also bolder).

## Frame
- Chrome is `--analyst-shell-base`. The content is a panel with a 20 px rounded top-left corner inset into it (desktop only).
- Header is 64 px, sticky. On desktop its brand cell is exactly the sidebar width, so the two read as one L. Sidebar is sticky, full height under the header.
- Case context bar is inside the content panel, sticky under the header. It no longer pushes the sidebar down.
- Below 1024 px: sidebar becomes the existing focus-trapped drawer; panel corner is square.

## Header contents
Brand (mark and name; the tagline is not shown in the header), workspace search (Ctrl K opens the scoped command palette), live processing chip (appears only when at least one case is processing; links to Cases), primary "Ask a question" (links to `/investigate`), Appearance. Below 900 px "Ask a question" becomes icon-only with its text still available to screen readers; below 640 px the chip is hidden.

## Measured contrast
Header action label on teal, chip text on both gradient stops, focus ring and marker on both stops, in both themes: `tokens.vitest.js`.

## States
The chip and sidebar badges read collection status through one shared 30 s cache. A case that fails to report is left out of the counts, never counted as zero problems or as healthy.

## Stage 3: case context bar and breadcrumb (built 2026-09-29)

Research: Primer UnderlineNav (sections of one context are links marked `aria-current`, not ARIA tabs; counters appear only after their data is ready so the bar never shifts; scrolls on narrow screens; icons drop at narrow widths), Atlassian breadcrumbs (they supplement navigation; collapse when tight), WAI-ARIA breadcrumb (labelled nav, ordered list, `aria-current="page"`).

Information architecture decision: the **sidebar answers "where in the workspace"** (Dashboard, Investigate, Cases, Activity, Settings) and the **top bar answers "where in this case"**. Case sections moved out of the sidebar into a section bar, and the sidebar's "Active case" card was removed because the case bar shows it.

| Part | Job | Source | Behaviour |
|---|---|---|---|
| Breadcrumb `Cases / <case>` | Orient, go up | route | Cases links to the directory; the case links to Overview except on Overview, where it is the current page. Long ids truncate; nothing else is hidden |
| Readiness pill | Can I rely on this case? | `summary.evidence_*` | Icon, label and counts (never colour alone). Attention state uses the failed colours plus the warning icon. Loading, unavailable and no-evidence states have their own copy |
| Switch case | Change case safely | configured cases | Only shown with more than one case; clears case-scoped session state first |
| Section bar | Move between Overview, Evidence, Ask this case, Timeline, Activity | route | Links with `aria-current="page"`; Evidence shows the failed-source count once status has loaded; sticky under the header on desktop, static and horizontally scrollable below 1024 px, icons hidden below 768 px |

Page headers on case pages no longer repeat `Workspace / Cases / <case>`. Only the deeper trail remains (Evidence detail shows `Evidence / Source`).

Tests: `CaseShell.vitest.jsx` (section links and current state, single breadcrumb, sidebar has no case links, count appears only after load, switch case). Test timeout raised to 15 s in `vite.config.js` because user-event typing timed out under load; no assertion was changed.

Not done here: a scroll-fade hint on the mobile section bar (Timeline is reachable by scrolling but not signposted). Logged for the S5 canvas pass.

## Stage 4: sidebar (built 2026-09-29, redesigned once after owner review)

The first pass had a "Cases" link and a separate "Your cases" list (redundant), and a "Recent questions" block with a caveat line, an empty-state sentence and long wrapped text. The owner rejected it as not enterprise grade. Redesigned from the research below.

Research: Linear favorites (personal shortcuts in their own section above the main list), Fluent Nav (two levels at most, categories that fold, secondary actions always in the DOM, at most one secondary action per node, tooltips for truncated labels), Primer NavList (group headings, `aria-current`, trailing counts).

Architecture: **workspace links first, then the analyst's own shortcuts.**

| Part | Job | Behaviour |
|---|---|---|
| Dashboard, Investigate, Activity, Settings | Workspace destinations | Dashboard carries the count of cases needing review |
| **Cases** (one category) | Reach any case from any page | Row opens the directory and shows the case count; a chevron folds the list (choice remembered); a plus starts a new case. Each case is one line: status icon, name (ellipsis, full name in a tooltip), failed-source flag. Status is also spoken ("Needs review, 1 failed source"). Active case first, then cases needing review, then configured order; six shown, then "N more". Every configured case is listed from first paint as "Checking status" and is never shown as healthy before its status arrives |
| **Pinned** | The analyst's own shortcuts | Only rendered if something is pinned; up to four |
| **Recent** | Resume work | Only rendered if there is history; up to four. Header carries a small "Saved in this browser only" icon instead of a caveat line. No empty-state text at all |
| Question row | One question | Two lines: the question (one line, ellipsis, full text in tooltip), then case and relative time ("12m ago"). Urdu and mixed-direction text keep `dir="auto"` |
| Row actions | Pin, rename | One "more actions" button per row, always in the DOM, visible on hover or focus and always on touch. It opens an inline row (Pin or Unpin with a pressed state, Rename) under the question, so nothing floats or gets clipped; Escape closes it |
| Collapse | Save space | Sticky at the bottom, labelled, reports `aria-expanded`; the collapsed rail keeps the workspace icons with tooltips and badges |

Drawer (below 1024 px): same content, every target 44 px, actions always visible.

Tests (`CaseShell.vitest.jsx`): case list order and cap, single Cases category with count, new-case link and spoken status, fold and remember, pinned and recent across cases with case and time, no question sections when nothing was asked, pin moves a question, rename and Escape, collapse state, relative-time formatting. Two real defects were found by the tests and fixed: the case count and list waited on status responses (now listed immediately), and test cleanup ran before unmount.

Review harness only: `design-review/ui-redesign-20260930/capture-sidebar.mjs` seeds browser-local question history so the rows can be reviewed. That history is not product data and is never shipped.

Widened the sidebar from 248 to 264 px for long generated case names.

## Revision after team-lead review (2026-09-30): question history, "Ask a question", Appearance

### Where recent questions live
Decision: **not in the sidebar.** The sidebar holds stable destinations; a changing list of long questions is noise there. Research: Linear opens its search menu on recent searches and recent items; the command-palette pattern shows a "recent" group while the query is empty, turning "type to find" into "open and pick"; the last few things a user did are the most likely next actions.

| Place | Job | Behaviour |
|---|---|---|
| **Command palette (Ctrl K)** | Fastest way to resume | On an empty query it opens with **Pinned questions** and **Recent questions from every case**, each with its case; typing filters them; empty groups are not drawn; the scope is stated ("This browser only"); a miss says the scope was not widened |
| **Investigate page, "Your questions"** | Browse, manage, resume | Pinned then Recent across every case, a filter box (question, label or case), eight recent shown and "Show all N", visible Pin and Rename buttons (no hidden actions), case and relative time on each row, a "Saved in this browser only" chip, an honest empty state |
| Sidebar and drawer | none | Removed; a test asserts no question text appears there |

The Dashboard's "Recent questions" strip is unchanged for now and is reviewed under D8. Dead sidebar question code and CSS were deleted.

### Header: "Ask a question"
The solid mint block glared on the dark header. It is now a ghost-accent button: mint icon, pale mint label, a mint wash and an 80% mint border, filling slightly on hover. Measured on both gradient stops in both themes (`tokens.vitest.js`): label at least 4.5:1, hover label at least 4.5:1, border at least 3:1. A first attempt at a 55% border measured 2.7:1 and was strengthened rather than the test loosened. A defect found on the way: an older `.workspace-header span` rule turned the label dark in light theme; the label now inherits the button colour, and the capture harness fails if the two ever differ.

### Header: Appearance
Replaced the text "Appearance" disclosure with a 44 px icon button that shows the theme in use (its accessible name states it, for example "Appearance, system, currently dark"). It opens a small panel with one native radio group of three preview tiles: System, Light, Dark (arrow keys and one tab stop come from the radios), a one-line status ("Following this device. Currently dark."), and a link to "All display settings". Escape and an outside click close it and focus returns to the button. The theme store is now one hook (`useThemePreference`) shared by this menu and the Settings page, so they cannot disagree. Tests: `AppearanceMenu.vitest.jsx`.

## Known items for the next stages
- S5: page header and canvas templates (and the scroll hint on the mobile case section bar).
- Dead CSS from earlier rounds (`.rail-case-switch`, `.desktop-rail__header`, old `.rail-question__actions`) remains in `workspace.css`; remove in a cleanup pass once every page is redesigned.
