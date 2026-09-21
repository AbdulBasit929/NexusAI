# NexusAI conversational investigation UI — activation 2026-09-08

Status: `UI_ACTIVATION=PASS`, `CONVERSATION_UI_FINAL=LIVE_VERIFIED`

- Activated at: `2026-09-08T11:24:34.8566359Z`
- Base image: `sha256:bed2eec364ca8c65151625685f79f5ab7a606a8223af88d1696504751b4a03a1`
- Live image: `sha256:91620e4c932764c46931dd4eecb47f9f72408e2387da3189f8fcd4ab4d7506be`
- Live API container: `fb3d8fd761690364d070da2f01561a85a6722832ec7112a3c3f47024c9c1c2d9`
- API restart count: `0`
- Rollback image: `nexusai/localai-forensic:rollback-before-conversation-ui-final-20260908`
- Served `/analyst` index SHA-256:
  `a0f1e4288c1b9ee46b4fa3c3c94870fef46b12d7c1718c36eee2e905d14ae7de`
- Retained tuple before/after: `67|67|80|22510|863|66`
- Activity before/after: `344`
- Active jobs before/after: `0`
- Loaded model before/after: `qwen3-embedding-0.6b`
- D-owned model loaded/executing: `false`
- Protected services: all four container IDs, image IDs, and restart counts
  preserved exactly.
- Retained data mutated: `false`
- Database migration: `false`
- D-unqualified source included: `false`

The final activation entered the mutation gate at 6.621 GiB available RAM against
the required 6 GiB floor. Only the API service was recreated with `--no-deps`.
Post-activation health, `/readyz`, `/analyst`, served-index identity, protected
service identity/restarts, retained tuple, Activity, active jobs, and D/model
safety checks passed.

Activation receipt SHA-256:
`0288b3ebdabfd84f75bb553c9355a130a48c915eec24fd8f9a4b7831c4c5f0bc`.

Durable operator script:
`scripts/activate_nexusai_conversation_ui_final_20260908.ps1`, SHA-256
`4f55f23678bcc057031b6d19a459ecf03499ddcfc26e54d5968ae460ba8062f2`.

`NX-B2.1D=OPEN`; `D_ACTIVATION=BLOCKED`.
