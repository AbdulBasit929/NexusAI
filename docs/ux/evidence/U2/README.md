# U2 evidence

`metrics.json` records the U2 acceptance matrix exercised against the local
production preview at `http://localhost:4182` on 2026-10-28 and the owner-
required navbar polish rerun on 2026-10-29.

The route smoke covers the global shell and every case section at desktop and
390 × 844. The polish matrix covers 1440, 1280, 1023 and 390 px in light, dark
and high contrast (12 combinations), verifying the 256/64 px rail, 56/48 px
header/case band and zero page-level horizontal overflow. The interaction
checks exercise real rendered controls: both rail
states, the mobile drawer, complete case-name fit, case-section select, case switcher, activity and
appearance disclosures, all appearance preferences, the command combobox,
shortcut sheet, route sequences, focus restoration, not-found and offline
states. Data shown by the shell remains sourced from the existing application;
gallery comparison values are synthetic and stay under `/design/components`.
No screenshot leaves this machine while O-8 remains open; this evidence is
DOM geometry, accessibility state, interaction output and automated tests.

The results are local Chromium laboratory evidence, not field telemetry. The
full implementation and test interpretation are documented in
`docs/ux/U2_SHELL_NAVIGATION_STAGE_NOTE_20261028.md`.
