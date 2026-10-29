# U2 navbar polish design note

**Date:** 2026-10-29 · **Stage:** U2 acceptance polish · **Status:** rebuilt and proved on `ui/u2-shell-navigation`; awaiting owner review · **Scope:** header, global rail, case band, collapsed rail, keyboard and mobile navigation

## Problem

The owner accepted B · Case atlas as the base but found the navigation too busy. The same case context was represented in three places: the header switcher, an inline Cases list and Current case section in the rail, and the case band. Evidence-readiness and numeric badges then competed with the actual navigation. The result exposed more choices without improving orientation.

The owner set a hard constraint: add no option to the current-case area or the rail's Cases item; remove clutter, and place secondary actions in the command palette or owning page.

## Research that informs the pass

| Question | Primary-source finding | Application |
|---|---|---|
| Which layer owns which navigation? | Atlassian's current navigation separates global top-navigation actions from product-specific side navigation; its layout guidance gives the rail a header/body/footer structure and adaptive collapse below wide layouts (R72). | Each destination has one navigation owner. The rail owns global destinations; the header owns case switching; the band owns case sections. |
| How should the rail adapt? | Microsoft NavigationView documents expanded, compact/icon and minimal modes selected by available width (R73). | Use an aligned 256 px expanded rail/brand track, a 64 px icon track below 1100 px and the modal drawer below 768 px. |
| What belongs in a shell? | Carbon's UI shell separates header, side navigation and global actions instead of nesting page content into navigation chrome (R74). | Move readiness and case-status detail out of the shell; pages and Activity own those facts. |
| Are navigation links a menu widget? | APG's disclosure-navigation example uses native links inside `nav`, explicitly avoiding `menu` semantics for ordinary site navigation; keyboard guidance says shortcuts augment, never replace, the normal Tab path (R75). | Preserve native links and linear Tab order. Arrow/Home/End supplement the case-section links; no roving focus is imposed on the global rail. |
| What are the interaction floors? | WCAG 2.2 Focus Appearance describes a 2 CSS-pixel perimeter as a straightforward sufficient indicator with 3:1 change/adjacent contrast; Target Size (Enhanced) uses 44×44 CSS px (R76). | Immediate 2 px focus ring; 40 px compact desktop rows, but 44 px controls in the drawer and coarse-pointer environments. |
| How should icons and movement behave? | Lucide recommends consistent size/stroke and accessible naming for meaningful icon-only controls (R58). Carbon distinguishes short productive motion from expressive motion, while reduced-motion preferences remove non-essential movement (R54–R55). | One 18 px / 1.75 stroke icon grammar; 150–240 ms tonal/rail transitions; no hover lift; reduced motion is effectively instant. |

## Options considered

### A — Keep dual case context, only tighten it

Retain the inline case list and Current case group, but reduce spacing and badges. This preserves maximum visible reach but leaves three competing representations of the same scope and cannot meet the owner's clutter finding.

### B — One owner per navigation layer — selected, then refined below

The rail contains only Dashboard, Ask across cases, Cases and Activity, with Settings in its footer. The header remains the only case switcher. The first pass retained case identity in the band; the owner review correctly found that still duplicated scope, so the final band contains sections only. Readiness, source failures, case lists and new-case actions stay on their owning pages or existing command surface.

### C — Rail-only context

Remove the case band and keep case pages in the rail. This saves 48 px vertically but weakens case orientation, duplicates the mobile problem, and makes section navigation disappear when the rail collapses.

## First-pass decision — superseded by the owner correction below

Build B. Specifically:

1. Remove the inline Cases disclosure/list/count/new-case controls and the entire Current case rail group.
2. Remove rail and case-tab badges. Keep the header Activity badge because it describes the control it opens and is the single home for live processing status.
3. Reduce the case band to identity and section links; remove readiness from the band. This was superseded below: the final band is section-only and the header owns the full case name.
4. Use one 18 px, 1.75 stroke icon grammar. Desktop rail rows are 40 px; drawer/coarse-pointer targets are at least 44 px.
5. Hover changes tone only. Active uses a quiet surface plus a 3 px leading marker. Focus is a 2 px ring with no delayed animation.
6. Collapsed navigation uses labelled hover/focus tooltips, not hidden meaning. The rail change uses productive motion and becomes instant under reduced motion.
7. Keyboard order remains native and predictable. Case sections additionally support Arrow Left/Right, Home and End. Existing command and switch-case shortcuts remain supplementary.
8. At mobile width the drawer contains the same four global links and Settings; the header keeps the case name and the band uses the native section select. No case list is reintroduced.

## Pre-declared proof

- Widths: 1440, 1280, 1023 and 390 CSS px.
- Themes: light, dark and high contrast at every width (12 combinations).
- States: hover, active, focus-visible, expanded/collapsed, drawer open/closed, long case name, empty/unavailable live status and reduced motion.
- Keyboard: skip link, global links, collapse/expand, case switcher, case-section arrows, command palette, Activity, Appearance and mobile drawer Escape/focus return.
- Geometry: no horizontal page overflow; no clipped focused control; tooltips stay legible; 44 px mobile targets.
- Honesty: no synthetic count or trend enters the live shell; readiness and attention are not duplicated into navigation.

Primary URLs are recorded as R72–R76 in the shared research register. No screenshot may leave this machine until O-8 is resolved.

## Owner correction after the first implementation review

The owner rejected the first implementation as still visually flawed, naming the top header/rail composition, the collapsed control and case-name fit. This invalidates the earlier "polish complete" claim and reopens U2.

### Revised options

1. **Patch the existing four-column header.** Lowest code change, but it preserves unrelated column widths, brittle child selectors and the compact-viewport state mismatch that made the collapse control misleading.
2. **Aligned instrument shell — selected.** Recompose the header, rail and case strip as one grid: a 256 px rail/brand column (64 px compact), a bounded two-line case scope that owns the complete case name, a flexible command field, and a stable utility cluster. The case strip becomes section-only, eliminating the last duplicate case name. At forced compact widths the misleading collapse control is absent; at user-collapsed desktop it is a visible centred expand control with the same labelled tooltip behavior as destinations.
3. **Top-navigation-only shell.** Maximises canvas width, but loses the fast, stable global destination model that the owner already accepted in Case atlas.

### Revised decision

Build option 2. The header and rail share 256/64 px alignment, not independent offsets. The case scope has a semantic eyebrow and one ellipsized name line whose full text remains accessible; when only one case exists it is static text rather than a useless switch button. At phone width the header keeps the menu, case name, command and live Activity; Appearance remains available from Settings. The 48 px case strip contains sections only. The collapse affordance is rendered only when the rail can actually change width, so its icon, label and `aria-expanded` state can never contradict the visual rail.

### Final proof

The 4-width × 3-theme matrix passed 12/12: 1440 and 1280 px use the 256 px rail, 1023 px uses the exact 64 px compact rail, and 390 px uses the drawer. The header remains 56 px, the section strip 48 px, the live case identifier fits at every width, and no combination has page-level horizontal overflow. Keyboard focus exposes the collapsed tooltip and 2 px ring; the centred expand control restores the 256 px rail; mobile Escape restores the menu trigger; Arrow Left/Right/Home/End traverse case sections; reduced motion produces 0 s shell transitions. Browser diagnostics reported no warnings or errors. Focused tests passed 49/49, the style contract passed, and the production build transformed 2,733 modules. No screenshot was exported while O-8 remains open.
