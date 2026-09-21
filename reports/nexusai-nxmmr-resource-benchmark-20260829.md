# NX-MMR resource benchmark

## Policy applied

One heavy model at a time. Fresh free-RAM floors were LIGHT 3.0 GiB, MEDIUM
3.5 GiB and HEAVY 4.5 GiB. After measurement, admission used the larger of the
class floor and observed incremental peak plus 1.5 GiB. Deployment/build keeps
its separate 6 GiB gate.

## Measurements

| Capability/run | Class | Fresh free RAM | Peak | Wall | Result |
|---|---|---:|---:|---:|---|
| Tesseract full 8 | LIGHT | 4,983,746,560 B | 19.746 MiB process | 2.066 s | pass |
| Paddle OCR batch 5 | HEAVY | 4,966,068,224 B | 445.945 MiB process | 8.423 s | pass |
| Paddle OCR full 8 | HEAVY | 5,041,774,592 B | 446.605 MiB process | 6.759 s | pass |
| embedding batch 5 | HEAVY | 5,154,701,312 B | 3,072 MiB container cap | 77.900 s | pass |
| embedding full 13 | HEAVY | 5,386,768,384 B | 3,072 MiB container cap | 136.836 s | pass |
| reranker batch 5 | HEAVY | 5,293,621,248 B | 3,072 MiB container cap | 240.444 s | pass |
| reranker full attempt 1 | HEAVY | 5,211,910,144 B | 3,072 MiB container cap | request >120 s | bounded failure |
| reranker full retry 2 | HEAVY | 5,247,168,512 B | 3,072 MiB container cap | 397.425 s | pass |

No emergency stop near 1 GiB, paging observation, evaluator kill or Docker
instability occurred. The reranker recorded one runtime timeout, then completed
the single bounded retry. Retrieval-class runs require at least 4.5 GiB free
under the measured rule and remain unsuitable for concurrent heavy execution.

The 3,072 MiB values are sampled cgroup/container peaks and reached the cap;
they are conservative lower bounds on unconstrained demand, not proof that
more memory would improve accuracy.

