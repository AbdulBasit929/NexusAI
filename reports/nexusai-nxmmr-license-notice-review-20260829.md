# NX-MMR license and notice review

Verdict: `PASS_FOR_PRIVATE_ISOLATED_EVALUATION`; production admission remains
separately gated.

| Resource | Authoritative declaration | Evaluation disposition |
|---|---|---|
| open-image-models YOLO asset | project `LICENSE`, MIT, copyright ankandrew | admitted for isolated evaluation; preserve attribution on later distribution |
| FastPlateOCR model/config | repository `LICENSE` on `master`, MIT, copyright ankandrew | admitted for isolated evaluation; preserve attribution on later distribution |
| tessdata_fast `eng`/`urd` | repository `LICENSE`, Apache-2.0 | admitted; distribution must carry license/notice obligations |
| PaddleOCR Arabic PP-OCRv5 | PaddleOCR `LICENSE`, Apache-2.0; archive has model files only | admitted; production packaging/NOTICE compatibility still required |
| Qwen3 Reranker 0.6B GGUF | pinned conversion model card and original model card declare Apache-2.0 | admitted; conversion/original attribution and production redistribution review remain required |

The local 108-image directory is user-attested as lawfully processable for this
private non-retained benchmark. No embedded dataset license or redistribution
manifest exists in that directory, so redistribution/publication remains
prohibited and the evidence is not a reusable public test corpus.

Evaluation permission is not treated as product installation, redistribution,
or commercial release approval. Those require a new exact license, NOTICE,
supply-chain and packaging decision.
