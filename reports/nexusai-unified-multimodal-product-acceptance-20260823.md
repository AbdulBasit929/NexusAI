# NexusAI unified multimodal product acceptance — 2026-08-23

Verdict: **ACCEPTED WITH RECORDED LIMITATIONS**

Scope: `default/nexusai-multimodal-product-acceptance`, actor
`nexusai-breadth-acceptance-operator`, exact approved 21-source manifest.

## Retained reconciliation

| State | Result |
| --- | ---: |
| Evidence / versions | 21 / 21 |
| Jobs | 34 total; 33 completed; 1 historical dead-letter; 0 active |
| Canonical records | 9,274 |
| Derived artifacts | 372 |
| KB assets | 19 |
| Latest evidence states | 21 completed |

Exactly 12 normal-endpoint immutable reprocess jobs completed sequentially in
the approved order. There was no first failure, second reprocess, upload,
replacement, deletion or cleanup.

## Capability verdicts

| Capability | Live proof | Verdict |
| --- | --- | --- |
| CDR/IPDR/subscriber/tower/ANPR/other structured | 65-operation matrix: 64 answered, 1 accepted no-result, p95 904.3 ms | PASS |
| Media-derived ANPR | `MN1367`, original-pixel locator/crop/hash lineage, cited Ask/History | LIMITED PASS |
| Image OCR | 219 retained source-bound OCR observations | LIMITED PASS |
| Image comparison/dHash | Two comparisons returned exact hash, dHash and SigLIP signals; both truthful non-duplicates | PASS WITH DATA LIMIT |
| SigLIP | Explicit five-candidate ranking, 768-dimensional governed model contract | LIMITED PASS |
| Face detection/similarity | Four reviewed synthetic positives, no-face negative, bounded 3-candidate ranking | LIMITED PASS — NOT IDENTITY |
| Urdu/Roman Urdu audio | Six timestamp segments and six Roman-Urdu derivatives; raw Urdu authoritative | LIMITED PASS |
| Video | 44 retained artifacts including timestamped text and sampled-frame image intelligence | LIMITED PASS |
| TXT/PDF/DOCX | 114 native passages; KB retrieval; TXT/PDF inline and DOCX governed attachment | PASS |
| Ask/citations/History | Structured positive, grounded document result, evidence detail analyses and truthful negative persist and reopen | PASS |
| Scope controls | `403/400/403/404/404` for missing authorization/candidate, cross-tenant, wrong-case and missing evidence | PASS |

## Related P1 corrections

The forensic API now:

- serves allowlisted native `.txt/.md/.log` sources even when an immutable
  legacy modality label remains `structured_records`;
- sends retrieval-only capabilities to bounded KB/derived-text execution rather
  than the deterministic records executor;
- treats evidence-only results with citations as answered rather than zero-row
  failures.

Focused Go tests pass. The prior API image is rollback-tagged; only the forensic
API was recreated for these related corrections. PostgreSQL, NATS, LocalAI,
worker, models, volumes, evidence, versions and jobs were preserved.

## Recorded limitations

- No positive exact-duplicate or threshold-positive near-duplicate pair exists
  in the approved manifest; no result is fabricated.
- Face and SigLIP scores are review candidates, never identity or facts.
- General OCR, ANPR and ASR are model observations requiring review.
- Spoken plate formatting remains `PENDING_M2`.
- The in-app browser reports audio/video codec playback unsupported, while the
  authenticated full/range source endpoints pass. Use a codec-capable browser
  for playback.
- Audio detail presentation currently labels capability readiness unavailable
  despite showing completed Urdu/Roman-Urdu artifacts; the catalog and source
  list correctly mark it ready.
- Immutable earlier processing notes and the pre-correction failed Ask remain
  visible as audit history.

The authoritative machine-readable proof is
`reports/unified-multimodal-product-acceptance-20260823/product-acceptance-api.json`.
Evidence/file checksums are in `export-checksum-manifest.json`. Retained data
remains on review hold; cleanup requires separate explicit approval.
