# UI/UX pre-fix route sweep

**Date:** 2026-09-27  
**Target:** live forensic API through the same-origin Vite proxy  
**Case:** `nexusai-multimodal-product-acceptance`  
**Evidence detail sampled:** `50b7343b-6172-4a81-a5fc-1b10c0a96764`  
**Runner:** `e2e/uiux-preflight-audit.spec.js`

## Coverage

Four widths were measured: 1440, 1280, 1024, and 768 pixels.

Twelve routes were inspected at every width:

- Dashboard
- Cases
- New case
- Global Activity
- Settings
- Admin
- Case Overview
- Evidence
- Investigate
- Timeline
- Case Activity
- A live Evidence detail

Total observations: **48**.

## Results before redesign work

| Check | Result |
|---|---:|
| Exactly one `h1` after authoritative loading resolved | 48 / 48 pass |
| Duplicate DOM IDs | 0 |
| Horizontal page overflow | 0 px on every route and width |
| Browser console warnings/errors | 0 |
| Enabled visible interactive elements excluded from keyboard focus | 0 |

The audit deliberately waits for the live Evidence detail request to resolve. The skip-link target `<main tabindex="-1">` is intentionally programmatically focusable and is not treated as an interactive control.

## Rail-specific findings recorded before fixing

1. The recent-question list is browser-local but is labelled only `Recent questions`; it can be mistaken for shared or server history.
2. The rail renders a maximum of four questions but exposes no pin or rename action there, despite both being required for a productive resume surface.
3. Pin state exists in browser storage and the full Question History surface can toggle it, but the rail offers no equivalent control.
4. Rename is not implemented in browser state at all.
5. Case switching exists in the case context band but not in the rail, so the rail is not yet a complete case work surface.
6. The collapsed rail retains labelled icon links and the collapse preference, which should be preserved rather than rebuilt.
7. The mobile drawer focus trap, Escape behavior, and focus return are already tested and should be extended without changing their contract.

## Previously measured evidence findings carried forward

These were supplied as live measurements and are not reclassified by the clean route sweep:

- `ProcessingWait` makes a completeness claim over `recent_evidence`, a recent-N sample.
- Family chips show file counts without naming the unit and can conflict with row-family coverage.
- Processing and catalog panels enumerate the same evidence without identifying the authoritative list.

They belong to surface 4.5 and are not changed during the navigation-rail slice.
