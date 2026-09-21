# NexusAI UI migration plan

Rebaselined: 2026-08-06

Status: R2 source accepted; R3 active

## Preserve

- Current Case Workspace, governed v1 case discovery, Agent Chat, deterministic
  query routing, professional answer presenter, responsive shell, and admin
  engine pages.
- Compatibility route `/app/records` as a redirect.

## Incremental slices

1. **R2:** approve product, brand, shell, navigation, role, state and white-label
   contracts. No feature implementation.
2. **R3:** implement the NexusAI app shell, assets, metadata, responsive
   navigation, role/capability framing and technical-administration boundary.
3. **R4:** implement the modular case workspace and one active-case contract.
   Preserve `R4-PRE-01` as a separately deployable source-accepted fix.
4. **R5:** build truthful Evidence and ingestion status, lineage, accounting,
   custody and processing controls.
5. **R6:** build Ask NexusAI, then take typed Agent Chat lifecycle reliability as
   a bounded later work item such as R6.2. Do not start that lifecycle slice
   during R1/R2.
6. **R7+:** add timeline, graph, map, document, image, audio and video components
   only as their evidence-family APIs pass their R-phase gates.
7. **R16/R17:** complete enterprise administration, scale, release and continuous
   regression.

## Non-goals

No big-bang React directory rewrite, no blind technical identifier rename, no
placeholder analytical charts, no fake progress, and no exposure of pending
modalities as operational.

## Exit criteria per slice

One case scope, API/source parity, keyboard access, 390/820/1024/1440 acceptance,
no console errors/broken assets/overflow, exact citations, focused tests, source
and runtime status separated, rollback documented.
