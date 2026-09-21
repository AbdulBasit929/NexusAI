# NexusAI design system

Status: R2 source accepted; token implementation belongs to R3, 2026-08-06

## Semantic tokens

Use centralized `--nx-*` aliases for canvas/surface/elevated/overlay backgrounds,
subtle/strong borders, primary/secondary/muted text, primary/accent, success,
warning, danger, information, evidence, citation, model-assisted, and
deterministic states. Existing generic theme tokens remain compatibility inputs
until migrated; new forensic components must not scatter raw brand colors.

## Core patterns

- Case header: one authoritative case, classification, status, processing count.
- Capability badge: operational, processing required, no data, manual review,
  disabled by policy, administrator only, unavailable.
- Result: direct answer, optional validated interpretation, exact totals, typed
  visualization, context/limitations, collapsed sources/trace.
- Evidence citation: stable source, version, locator, and open action.
- Loading: neutral verification language and unknown-value dashes; never a fake
  zero, no-data conclusion, or fabricated percentage.
- Error/partial: preserved completed artifacts, actionable retry eligibility,
  scope, request ID, and safe limitation.

## Accessibility and responsive rules

- Keyboard-operable controls and visible focus.
- Programmatic labels, landmark/navigation names, status/live regions.
- Touch targets and readable density at 390, 820, 1024, and 1440 pixels.
- No page-level horizontal overflow; data tables may scroll inside an explicitly
  bounded container.
- Preserve Urdu and bidirectional text without inventing transliteration.
- Long evidence identifiers wrap or truncate with an accessible full-value path.

## Acceptance

Lint, production build, focused keyboard/browser tests, console/network checks,
broken assets, and all four responsive widths are required for each changed
analyst surface.

The R2 token contract is source accepted. R3 owns incremental light/dark token
implementation plus focus/disabled/hover/selected verification for shell
surfaces. Evidence, deterministic, model-assisted, candidate, warning and
unavailable semantics must remain distinguishable without color; later phases
own domain visualization tokens as real capabilities arrive.
