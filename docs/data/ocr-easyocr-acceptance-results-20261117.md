# Routed OCR role with EasyOCR: product-level acceptance, results (2026-11-17)

Plan and gates: `ocr-easyocr-acceptance-preregistration-20261117.md` (amendment A1 before any fresh image was read). Settings: router threshold 0.10, 4 threads, long-side cap 1400 px, worker overlay image `nexusai-forensic-records-worker:easyocr-20261117`, the router built by `OcrRouter.from_environment()` with the live worker's OCR environment. Two arms on the same 298 fresh images (new seeds 20261201 to 20261203; 30 newspaper rows nobody had read with EasyOCR): `today` = the router as deployed, `easyocr` = the same with the role on. Scorer: `tools/media_data/accept_router_score.py`; per-image output in `results/ocr/accept-{today,easyocr}.jsonl` (outside the repository). Synthetic and public data only (tiers T1, T3).

## 1. Results side by side

| Set | Arm | n | Character error (mean per image) | Identifiers | Empty or error | Seconds median (max) | Sent to EasyOCR |
|---|---|---|---|---|---|---|---|
| English pages | today | 48 | 0.0% | 1504/1508 = 99.7% | 0% | 3.4 (6.5) | 0% |
| English pages | easyocr | 48 | 0.0% | 1504/1508 = 99.7% | 0% | 3.1 (4.5) | 0% |
| Urdu pages | today | 40 | 43.9% | 550/600 = 91.7% | 0% | 2.4 (2.9) | 0% |
| Urdu pages | easyocr | 40 | **8.7%** | 509/600 = 84.8% | 0% | 18.8 (26.7) | 100% |
| Urdu lines | today | 60 | 35.4% | - | 15.0% | 0.6 (1.0) | 0% |
| Urdu lines | easyocr | 60 | **18.4%** | - | 0% | 1.5 (3.4) | 86.7% |
| Mixed | today | 120 | 35.2% | 733/769 = 95.3% | 0% | 0.9 (2.0) | 0% |
| Mixed | easyocr | 120 | **18.3%** | 733/769 = 95.3% | 0% | 4.3 (14.3) | 75.8% |
| Real newspaper print | today | 30 | 68.8% (pooled 70.3%) | - | 0% | 1.7 (3.6) | 0% |
| Real newspaper print | easyocr | 30 | **22.1%** (pooled 21.9% [20.4, 23.8]) | - | 0% | 8.2 (25.2) | 100% |

## 2. Gates

| Gate | Requirement | Result | Verdict |
|---|---|---|---|
| R1 real Nastaliq print | pooled error at most 25% and 10 points below `today` | 21.9% against 70.3% | **met** |
| R2 Urdu | Naskh lines at most 15%, Nastaliq lines at most 25%, Urdu pages at most 15%, each at most `today` | Naskh lines 10.2% (today 16.1%); **Nastaliq lines 26.6% [21.9, 31.3] (today 54.6%)**; Urdu pages 8.7% (today 43.9%) | **not met: Nastaliq lines miss by 1.6 points** |
| R3 mixed | identifiers at least 90% and not below `today`; error at least 10 points below `today` | identifiers 95.3% = today; error 18.3% against 35.2% | **met** |
| R4 English | identifiers within 2 points; at most 2% sent to EasyOCR | identical (99.7%); 0% sent | **met** |
| R5 routing | Urdu pages at least 98% to EasyOCR; English at most 2% | Urdu pages 100%; English 0% | **met** |
| R6 time | median seconds per Urdu page at most 30 s | 18.8 s (memory: about 2.3 GB while a page is read, 0.86 GB at rest; the 1.5 GB bound is not met while reading, see the bake-off results section 6) | **time met**; memory above the bound |
| R7 clean failure | errors or empty readings on at most 5% per set | 0% on every set | **met** |

Six of seven gates are met. **R2 is not met, by one sub-gate and 1.6 points**: Nastaliq lines at 26.6% against 25%; the 95% interval (21.9% to 31.3%) contains the bound, the sealed bake-off measured 21.2% on the same kind of lines, and the router halves today's 54.6%. It is reported as missed.

## 3. What the numbers also show (not gated, but they matter)

* **Identifiers are not uniformly better.** On Urdu pages EasyOCR reads CNIC numbers perfectly (200/200 against 186/200) and phones (199/200 against 186/200), but loses **amounts** (110/200 against 178/200). The digits are right: EasyOCR reads the prefix "Rs." as "R5." ("R5. 145,622"), so a verbatim match fails and an analyst sees the right figure with a wrong currency mark. On mixed images the same pattern: phones 101/115 (today 89/115), **plates 15/28 (today 27/28)**, other types equal. Totals tie at 733/769. Plates and the "Rs" prefix are Latin-letter text read by a recogniser trained mainly for Arabic script.
* **The fix is already in the plan**: read Latin and digit regions with the PP-OCRv5 Latin recogniser on the same boxes ("per-line script split"). It is the next engineering item; nothing here was tuned to it.
* **Mixed images by kind (error today to easyocr):** forms 49.2% to 16.5%, mixed lines 34.2% to 14.2%, receipts 29.0% to 20.2%, chats 43.0% to 26.4% (13 of 22 sent to EasyOCR), **bank statements 14.9% to 15.9% (3 of 20 sent; they stay on PP-OCRv5)**.
* **The 8 Urdu lines that fell back to Tesseract** (EasyOCR returned fewer than 12 characters) read exactly as badly as today's route (40.2% mean error in both arms): the fallback neither helped nor hurt.
* Newspaper rows keep the bake-off picture: 21.9% pooled against 21.5% on the sealed 60 rows.

## 4. Limits

Synthetic and public print only: the Urdu and mixed sets are generated (T1), the newspaper rows are public print from one publisher family (T3). No real case document has been read. The role is a model observation that needs analyst review; Urdu line order and low-quality Nastaliq are not certified. Weights licence unconfirmed (ledger MOD-047 to MOD-049); by the owner's direction of 2026-11-17 it is not treated as a blocker for development.
