# NexusAI R3.1 visual audit and system

Date: 2026-08-06  
Status: implementation in progress  
Scope: React UI only; no API, backend, model, evidence, database, or deployment change

## Accepted R3 baseline

The accepted R3 production bundle remains the compatibility baseline: 662
modules, 27/27 focused production-browser contracts, responsive acceptance at
390/820/1024/1440, light/dark themes, keyboard support, NexusAI metadata and a
clean browser console.

## Current visual audit

The accepted shell is structurally reliable and has no page-level horizontal
overflow at any required width. Its visual system is not yet distinctive enough
for the NexusAI product:

- the 200 px primary rail, 58 px header, compact typography, shallow surface
  separation and small controls read as an upstream administration dashboard;
- the Home hierarchy leaves large undirected canvas areas while its primary
  assistant and composer actions compete for attention;
- Knowledge, Models and Settings use dense generic cards, tables, filters and
  actions with weak grouping; Settings also exposes legacy LocalAI imagery in
  its branding preview;
- the forensic Case Workspace is information-rich but visually flat: 25 of 33
  visible controls are below the intended enterprise control/font baseline;
- light and dark modes are complete, but neither yet expresses a coherent
  NexusAI intelligence-command identity.

Measured on the accepted production preview, Home contained 15-17 undersized
controls depending on viewport. There was no horizontal overflow at 390, 820,
1024 or 1440 px.

## Visual direction

R3.1 uses a calm **NexusAI Intelligence Command System**: deep slate/navy
canvas, disciplined blue actions, restrained teal state accents, clear surface
elevation, compact-but-readable data presentation and an unmistakable
code-native NexusAI identity. It deliberately avoids neon, cyberpunk styling,
decorative imagery and generic dashboard gradients.

## Shared system contract

- Geist and Geist Mono remain the bundled type system; no font download.
- Standard controls target 40-44 px; compact table actions remain at least
  32 px with visible focus.
- The primary rail targets 248 px expanded and 72 px collapsed; the global
  context header targets 64 px.
- Page spacing follows an 8 px rhythm with 24-32 px page gutters, 20-24 px
  panel padding, 10-14 px radii and restrained two-level elevation.
- Semantic tokens remain the source of truth for canvas, surfaces, text,
  borders, focus, action and status colors in both themes.
- Existing routes, components, permissions, branding configuration and data
  contracts are reused. R3.1 adds no route or API.

## Acceptance focus

The production build, existing 27-contract R3 suite, focused R3.1 visual-system
assertions and interactive browser evidence must all pass before this slice is
accepted. Deployment remains a separate guarded approval.
