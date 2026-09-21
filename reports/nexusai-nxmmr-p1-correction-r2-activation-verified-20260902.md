# NX-MMR P1 correction R2 activation verified — 2026-09-02

## Result

`P1_CORRECTION_ACTIVATION=PASS` and independent verification both pass for
receipt `20260902T154901947Z`. Do not rerun this activation.

## Activated images

- API container `f26a25df227db883fbd65d9a0518dca0a102dbc6b164f8b747d2a8e05d112119`
  uses `sha256:91616844039217196b9f30e2478dc2e5e91525e0e8a4fc74d6a7887a7e3eb85a`.
- Forensic API container
  `3b42baf71842d1930713f7247b9e4b409de54f4240c4189b1fac06c533510498`
  uses `sha256:254649dcb4d032669d9f9dc46f21fddb91ccddba86a00631ea70c245b1f3e9d3`.
- Both match the sealed receipt, are running and have restart count zero.
- API Docker health is healthy; forensic API readiness passed the bundle's
  health verifier.

## Protected and retained state

- Worker container `40b6e628d586...`, PostgreSQL `f66e05a3b179...`, and NATS
  `5c49e70d132a...` retained exact container/image identities.
- Required `faster-whisper-small-ur` and `face-detect-yunet-sface` models exist.
- Active analytical jobs: zero.
- Retained tuple before and after: `63|63|76|22507|827|61`.
- No migration, model download, evidence rewrite, bulk reprocessing, prune,
  orphan removal or volume deletion was performed.

The Compose orphan message was informational. The operator did not pass
`--remove-orphans`; protected services were not removed or recreated.

## Receipt integrity

- State: `VERIFIED`.
- Activation verification: `PASS`.
- Live failed-cells acceptance: `PENDING`.
- Manifest SHA-256:
  `6db9eb32d29b9b2b53fed8b4f679b48ed5d934771d1079a7ba82b66bbc1bf7cf`.
- Source/operator seal SHA-256:
  `0823f0db215806af8fdba689c891efefead4ea531a8727e6b93f11bc000ffec9`.

## Next gate

Activation is closed. Run only the previously failed live acceptance cells:
transcript timing/anaphora and seek; explicit plate intent under selected audio;
selected-video exact ANPR/no-match/status; selected-image exact OCR/no-match;
SigLIP and face Ask/Activity presentation; cross-family citation fairness.
Retain exact case/evidence scope and fail any Urdu result that substitutes
Devanagari or Latin-only text for required Urdu script. Product certification
and NX-B2 promotion remain prohibited until that matrix passes.
