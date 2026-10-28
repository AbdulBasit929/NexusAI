# U2 evidence

`metrics.json` records the U2 acceptance matrix exercised against the local
production preview at `http://localhost:4182` on 2026-10-28.

The route smoke covers the global shell and every case section at desktop and
390 × 844. Additional tablet inspection verifies the forced 56 px rail at
1023 px. The interaction checks exercise real rendered controls: both rail
states, the mobile drawer, case-section select, case switcher, activity and
appearance disclosures, all appearance preferences, the command combobox,
shortcut sheet, route sequences, focus restoration, not-found and offline
states. Data shown by the shell remains sourced from the existing application;
gallery comparison values are synthetic and stay under `/design/components`.

The results are local Chromium laboratory evidence, not field telemetry. The
full implementation and test interpretation are documented in
`docs/ux/U2_SHELL_NAVIGATION_STAGE_NOTE_20261028.md`.
