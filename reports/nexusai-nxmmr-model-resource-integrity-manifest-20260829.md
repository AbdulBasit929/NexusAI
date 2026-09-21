# NX-MMR model/resource integrity manifest

Verdict: `PASS_FOR_ISOLATED_EVALUATION` on 2026-08-29.

The seven-file cache contains exactly 663,933,265 bytes. All expected sizes
match. The two predeclared Tesseract hashes and the Qwen3 reranker hash match;
fresh SHA-256 values are now sealed for the three ANPR files and Paddle archive
in `configuration/nexusai_multimodal_benchmark_manifest_v1.json` and the
machine-readable acquisition receipt.

| Artifact | Integrity result |
|---|---|
| YOLOv9-t plate detector | 7,771,218 bytes; SHA-256 `888397b96d761c89db40bc9c305838e8652660f5e282c2cadebbe8d2951a77a8` |
| CCT-XS-v2 OCR | 3,344,292 bytes; SHA-256 `8031afb5fdc6b4d80462c9d542f1284ebd2cfddf5dbacd62609848d7e2855f44` |
| CCT plate config | 1,725 bytes; SHA-256 `0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6` |
| Tesseract English | 4,113,088 bytes; declared SHA-256 match |
| Tesseract Urdu | 1,398,718 bytes; declared SHA-256 match |
| Paddle Arabic PP-OCRv5 | 8,151,040 bytes; SHA-256 `a25c1f96cd0cda485b942851b5afb4a95bc4041a56c295bb4439190d53ee0a14`; publisher ETag and length match |
| Qwen3 reranker Q8 | 639,153,184 bytes; declared SHA-256 match; `GGUF` v3, 311 tensors |

Paddle archive enumeration contains only its root plus `inference.pdiparams`,
`inference.json`, and `inference.yml`; no absolute or parent-traversal member
was found. No extraction or inference occurred during this gate.

`ApprovedAcquisitionCompleted=true`

`SupplyChainVerification=PASS`
