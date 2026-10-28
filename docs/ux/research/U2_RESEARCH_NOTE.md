# U2 shell and navigation research note

**Stage:** U2 Shell and navigation · **Date:** 2026-10-28 · **Status:** implemented; owner selected B · Case atlas · **Scope:** global header, primary rail, case band, command surface, overlays, landmarks, focus, responsive navigation, preferences and global route states

## Question and starting point

U2 has to make NexusAI feel like one case-centric forensic instrument without hiding the global workspace. The shell must answer three questions before the analyst reads a page: where am I, which case constrains this work, and how do I move or act without losing that scope?

The current application already has valuable behavior that must be preserved: a persisted collapsible rail, a case switcher, real evidence-readiness summaries, case-section links, visible failure counts, a command surface, an appearance menu, an application skip link and focus traps for the existing overlays. The gap is structural rather than cosmetic. Workspace and case navigation are not exposed as distinct labelled landmarks; case navigation is split between a crowded context bar and page-local assumptions; the header does not yet contain the complete U2 utility set; route changes do not own title, main focus and a polite load announcement; and the responsive shell is not expressed as one documented system.

The stage began with a bounded synthetic gallery. After the owner selected B · Case atlas, that direction was applied to the live shell without introducing synthetic production values.

## Primary-source findings and U2 application

| Area | Evidence read | Finding | U2 application |
|---|---|---|---|
| Global versus contextual navigation | Atlassian's navigation design account and current navigation layout docs (R61–R62, **V**) | Universal actions belong in a stable top bar; vertical space makes a side rail effective for dense product navigation. The current system supports an expandable, collapsible and responsive rail with a separate contextual panel. | Keep brand, case switcher, command trigger, activity, appearance and analyst identity in the 56 px header. Put Workspace and Current case into separately labelled rail groups. Do not put page titles in the header. |
| Rail defaults | Atlassian navigation layout (R62, **V**) | A side rail can preserve a user-set width/state, collapse at narrower widths and become an overlay on small screens. Its collapse affordance must remain discoverable. | Compare expanded and collapsed desktop defaults, but keep the same remembered state and the directive's 1024/768 responsive boundaries in every option. Drawer focus is modal and returns to the trigger. |
| Command palette | WAI-ARIA APG combobox and modal dialog patterns (R63–R64, **V**) | For a listbox popup, DOM focus stays on the combobox and `aria-activedescendant` identifies the active option. A modal contains its tab sequence, closes on Escape and restores focus. | Rebuild `WorkspaceCommands` as one modal command surface with Go to, Do and Ask groups, at most eight results per group, arrow-key movement, Enter activation and Escape restoration. Opening the palette never calls a model. |
| Landmarks | APG landmark regions (R65, **V**) | A page should expose one top-level banner and main; repeated navigation landmarks need distinct purpose labels; a related side panel is a named complementary landmark. | The shell contract is `banner`, `nav "Workspace"`, `nav "Case"`, `main#workspace-main[tabindex=-1]`, and optional `aside "Details"`. Exactly one `h1` belongs to the page working surface. |
| Focus and overlays | WCAG 2.2 focus order and focus-not-obscured guidance (R66, **V**) | Sequential focus must preserve meaning. Persistent overlays must not conceal focused content; reflow, modal focus containment, Escape dismissal and focus restoration are valid patterns. | Keep DOM order aligned with visual order. Inline panels reflow at desktop; drawers and bottom sheets contain focus at smaller widths and restore it on close. Route change focus moves to main only after navigation. |
| Targets and status | WCAG 2.2 Target Size and Status Messages (R67, **V**) | WCAG's minimum target is 24 CSS px subject to exceptions; new status text that does not change context must be exposed without taking focus. | NexusAI keeps the stronger 44 px touch target rule. Route-loaded, activity and completion messages use a polite status region; blocking failures use persistent inline or alert treatment, never an ephemeral toast. |
| Native overlay semantics | WHATWG HTML interactive-elements standard (R68, **V**) | A modal dialog enters the top layer, makes the rest of the document inert, records the previously focused element and runs defined focusing steps. | Prefer native `dialog.showModal()` where integration permits; otherwise the React fallback must reproduce inertness, containment, Escape and focus restoration and must be tested as behavior, not inferred from ARIA alone. |
| Route motion | React Router's current View Transitions guide, checked against installed React 18.3.1 and React Router 7.18.4 (R69, **V**) | The current recommended React `<ViewTransition>` route boundary requires React 19.3+. React Router's older APIs still work but are deprecated. | Do not add a deprecated router transition API or upgrade React inside U2. Use a small CSS opacity transition on the persistent working surface with progressive enhancement and fully disable it for reduced motion. |
| Appearance before paint | web.dev theme-switch component (R70, **V**) | Applying a stored theme only after the application mounts creates a wrong-theme flash; a small pre-paint assignment can avoid it. | Read the guarded local preference before the main stylesheet paints, while preserving System as the default. Storage failure leaves a usable System theme. U2 also adds high contrast, density, reduce-motion override and answer-detail preferences. |
| Utility menus and shortcuts | APG menu button, menu/disclosure navigation and keyboard-interface guidance (R71, **V**) | Menu buttons expose expanded state, move focus predictably and restore it on Escape. Custom ARIA widgets need explicit, conventional keyboard behavior. | Activity and appearance use a disclosure/menu pattern appropriate to their content, not menu roles for arbitrary form controls. One shortcut table drives the palette, shortcut sheet and key handler, and shortcuts are ignored in editable fields. |

## Options taken to the gallery

The three options deliberately share the same information architecture, commands, responsive thresholds, states and accessibility contract. The owner is choosing desktop emphasis, not permission to weaken a rule.

### A — Ledger rail

An expanded 248 px rail is the default. Workspace and Current case receive equal visual weight, while the 48 px case band is a compact scope/status strip. This is the most explicit direction and the easiest for a new or occasional analyst to scan. Its cost is the largest persistent horizontal footprint.

### B — Case atlas

An expanded 264 px rail is the default, but the current case is treated as a distinct working block while global destinations recede. This makes the CASE → EVIDENCE → QUESTION hierarchy most obvious and makes scope changes harder to miss. Its cost is a visually heavier rail.

### C — Signal dock

A 56 px icon rail is the desktop default, with the case band and command surface doing more orientation work. This gives the largest canvas to evidence tables, maps and connection work. Its cost is recall: infrequent analysts depend more on tooltips, the command palette or expanding the rail.

The gallery at `/design/components` retains A, B and C in light, dark and high contrast, at 1440 and 390 px, and in case and global scope. **Owner decision, 2026-10-28: B · Case atlas.** B is selected in the gallery and now defines the live shell; A and C remain as the comparison record.

## Fixed implementation contract after the owner decision

1. Preserve existing real state and behavior; replace markup and CSS without introducing synthetic production values.
2. The top bar is 56 px. The rail is 248 px expanded and 56 px collapsed unless the selected direction explicitly adjusts the expanded default within the directive. The case band is 48 px.
3. At 1024–1279 px the rail is collapsed and the context panel is a drawer; at 768–1023 px the case strip scrolls and the switcher shortens; below 768 px navigation is a modal drawer, case sections become a select-style control and details become a bottom sheet.
4. The command palette has one combobox/listbox focus model. It consumes existing `WorkspaceCommands` actions and does not start an Ask request merely by opening or typing.
5. One top-level banner, two purpose-labelled navigation landmarks, one focusable main and one optional named complementary region are the shell invariant.
6. Route change updates the document title, focuses main and announces the loaded destination without stealing focus twice.
7. Errors that require action remain inline or in Activity. Toasts are limited to completed, non-actionable confirmations and are pausable and dismissible.
8. No new component dependency is justified by this research. Native elements plus the existing React/lucide stack can meet the contract.

## Acceptance probes declared before implementation

- Keyboard-only: skip link → Workspace navigation → Case navigation → command palette → activity → appearance → main → details; no trap outside an open modal and focus returns after every close.
- Screen reader: landmarks are distinguishable; one `h1`; opening a named case and then Ask is understandable without visual position; route load is announced once.
- Responsive: 1440, 1280, 1024, 768 and 390 px; no focused element is hidden beneath a drawer, sticky band or bottom sheet.
- Themes: light, dark and high contrast at desktop and phone widths; all icon-only controls retain visible focus and accessible names.
- Global states: offline, 404/not found, access denied and unexpected failure stay distinct and provide the next valid action.
- Regression: existing case state, failure counts, case switch, command routes and stored preferences remain truthful.

The primary URLs and confidence marks are recorded as R61–R71 in the shared research register. Each **V** source was fetched and read for this stage on 2026-10-28.
