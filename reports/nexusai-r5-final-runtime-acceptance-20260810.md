# NexusAI R5 final runtime acceptance

Date: 2026-08-10  
Status: complete; live runtime accepted

## Activation evidence

- Guarded Phase 6 activation: PASS.
- API/worker/LocalAI build times: 11.6/3.6/58.1 seconds.
- LocalAI attempts: one; free RAM before build: 6.42 GiB.
- Active images: LocalAI `f330662ea578`, forensic API `d32b62687b6f`, worker
  `d3604b389b0a`.
- Rollback images and named volumes: preserved.
- LocalAI, worker, PostgreSQL and NATS containers: healthy; forensic API HTTP
  health: 200.
- LocalAI `/readyz`, API `/healthz`, worker `/metrics` and NATS `/healthz`: 200.
- R5 acceptance marker: `live_accepted`; mutating requests: zero.

## Retained-state and custody reconciliation

- One authorized case remains selectable; deletion count is zero.
- Governed case totals: 10 evidence items and 9,274 canonical rows.
- Accepted synthetic source: 309 bytes, one immutable run, two total rows, two
  accepted, zero duplicate and zero rejected.
- Custody: three same-second append-only events, one root, predecessor-linked
  registered -> processing -> completed topology, zero broken links and
  `chain_valid: true`.
- Migration 011 marker remains verified. It changed only the custody trigger
  function, rewrote zero retained rows and preserved a validated 5,707,090-byte
  rollback archive with SHA-256
  `030208fbeed598d82ec91dca73a7522c81821a6396a49387222ef66040c2998b`.
- The deployed trigger function contains the unreferenced-tail selector; the
  timestamp/random-UUID tail assumption is no longer active.
- Six specialist agents remain registered and operational. The two approved
  model IDs remain `qwen3-embedding-0.6b` and
  `qwen_qwen3-4b-instruct-2507`.

## Product acceptance

The live Evidence Operations desk reports 10 registered sources, nine ready,
one truthful failed source, zero in flight, 9,274 accepted rows and one failed
job. The accepted source shows exact row accounting, immutable lineage, one KB
asset, three processing events, `Hash chain verified`, three custody events,
zero broken links and the read-only reprocess approval boundary.

Responsive checks at 390, 820, 1024 and 1440 pixels reported no page-level
horizontal overflow. The evidence, accounting, custody and approval content
remained present at every width and the browser console contained no warnings
or errors.

## Closure decision

R5 satisfies its scoped source, deployment, runtime, data, UI, provenance,
failure and governance gates. R5 is complete. Reprocess execution, further
retained ingestion, cleanup, schema changes and deployment remain independently
approval-gated and are not implied by this acceptance.

