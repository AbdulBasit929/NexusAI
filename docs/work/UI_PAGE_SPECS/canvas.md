# Spec: main canvas and page templates (Stage 5, built 2026-09-30)

Files: `src/styles/canvas.css` (owns these primitives), `tokens.css` (page tokens), `components/PageHeader.jsx`, `Card.jsx`, `Skeleton.jsx`, `AnalystComponents.jsx` (EmptyState, RouteState), `PageTemplatesDemo.jsx` (gallery).

## Research that decided it
- **Primer PageHeader**: breadcrumb, title with description, actions on the right, optional secondary navigation. Decision: that anatomy, plus a facts strip; the title stands alone, so the eyebrow label that repeated it is not rendered.
- **Atlassian empty state**: sentence-case header, one short description, one primary action, tone matched to context (celebratory for completion, neutral for no results, educational for first use), never negative. Decision: one state-panel system with a tone per state and a single primary action.
- **Atlassian skeleton**: match the real layout, no status colours, remove as soon as content is ready. **NN/G waits**: nothing under 1 s, skeleton or spinner about 2 to 10 s, progress with an estimate beyond 10 s. Decision: skeletons only for short waits; the 70 s question wait keeps its honest processing state, never a fake bar.
- **WCAG 2.5.8** minimum target 24 px; the product uses 44 px.

## Canvas rules
| Token | Value | Use |
|---|---|---|
| `--analyst-page-gutter` | `clamp(16px, 3vw, 32px)` | horizontal page padding, same as the case bar |
| `--analyst-page-block-start / -end` | 28 px / 64 px | vertical rhythm |
| `--analyst-content-wide` | 1280 px | dashboards, lists, tables, evidence |
| `--analyst-content-reading` | 1060 px | forms, settings, Investigate entry |
| `--analyst-card-radius` | 16 px | every card and state panel |

Every `main` in the content panel gets the same gutter and width scale (Investigate keeps its own layout until its stage). New page classes need no padding of their own.

## Primitives
| Primitive | Anatomy | Rules |
|---|---|---|
| Page header | trail (deeper only), h1, description, actions, facts | one h1; trail is a nav named "Page trail" so it never clashes with the case breadcrumb; case pages pass only the trail below the case; no eyebrow |
| Card | title (h2 by default), description, actions, body, footer | a labelled region; `tone` changes the border only, the words always say it; footer carries source and coverage notes |
| State panel | tone mark, heading, description, one primary action, optional second | empty, no match, first use, error, forbidden, unavailable, loading; loading shows decorative placeholder lines inside a busy status; error is an alert with a support reference |
| Skeleton, SkeletonCards, ShellSkeleton | placeholder lines and cards | one status message, shapes hidden from assistive technology, no numbers; the route fallback reproduces header, sidebar and page rhythm so first paint does not move |
| Mobile section bar | scroll shadows | Timeline and Activity are signposted when the bar overflows below 1024 px |

## Measured
Tone marks are at least 3:1 on their tint and text is at least 4.5:1 on the caution and critical tints, in both themes (`tokens.vitest.js`). Tests: `CanvasPrimitives.vitest.jsx`.

## Adoption
The gallery (`/design/type-proof`) shows every primitive. Pages adopt `Card` and the state panels in their own stages; the Dashboard's `chart-card` and `dash-card` become `Card` in stage D-work. Pages still carry some older section styling (for example Settings and Cases), listed in each page's audit.
