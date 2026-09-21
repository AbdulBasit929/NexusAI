# NX-MMR OCR benchmark

Verdict: `FIXTURE_MEASURED`; no real-world OCR promotion is permitted.

Both candidates used the same eight deterministic pre-labeled truth crops: six
printed Latin/English identifiers under clear, blur, glare, night, skew and
occlusion conditions; one clear printed Urdu line; and one negative.

| Candidate | English exact | English CER | English mean latency | Urdu exact | Urdu CER | Urdu mean latency | Peak process RAM |
|---|---:|---:|---:|---:|---:|---:|---:|
| Tesseract `eng`/`urd+eng` | 50.0% | 0.143 | 120.1 ms | 0% | 0.214 | 172.2 ms | 19.75 MiB |
| Paddle Arabic PP-OCRv5 | 83.3% | 0.071 | 105.1 ms | 0% | 0.357 | 87.0 ms | 446.61 MiB |

Both negative controls passed. Paddle was exact on clear, blur, glare, night
and skew English identifiers, but truncated the occluded identifier. Tesseract
confused zero with `O` on clear/skew and degraded more severely on occlusion.
On the sole Urdu fixture, Tesseract made three character edits and Paddle five;
neither preserved the full line exactly. That single synthetic Urdu row is far
too small for a general quality claim.

The first Tesseract attempt is retained as `INVALID`: pointing
`TESSDATA_PREFIX` at a language-only directory hid the evaluator's native TSV
config. A one-crop diagnostic proved the cause; the accepted rerun used the
evaluator's byte-identical verified tessdata and native configuration.

## Capability states

- Printed English OCR: `FIXTURE_CERTIFIED`; Paddle is the fixture leader, but
  real printed/document/scene evidence is still required.
- Printed Urdu OCR: `LIMITED / BENCHMARK_DATA_REQUIRED`; retain Tesseract as the
  lower-memory floor until representative independent Urdu labels exist.
- Handwritten English OCR: `BENCHMARK_DATA_REQUIRED`.
- Handwritten Urdu OCR: `BENCHMARK_DATA_REQUIRED`.

No synthetic-only result promotes Paddle or Tesseract into the live runtime.
