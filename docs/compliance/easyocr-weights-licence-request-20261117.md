# EasyOCR weights: licence and training-data request (draft, 2026-11-17)

**Status: a draft for the owner to send. Nothing has been sent.** It can go as a GitHub issue on `JaidedAI/EasyOCR` (the maintainers' usual channel) or by email to the project; the owner decides which and under whose name. It contains no case data and no personal information.

## Why this is needed

The OCR bake-off v2 (`docs/data/ocr-bakeoff-v2-preregistration-20261117.md`) found EasyOCR the strongest CPU-capable reader we have tried for printed Urdu and for English mixed into Urdu. The code is Apache-2.0 (LICENSE file and PyPI metadata). The licence of the **model weights** and the data they were trained on are not stated on the project pages we read (the model hub lists the files without a licence; a Hugging Face copy says the weights are under "Jaided AI license terms" and points back to the repository's LICENSE). Until that is answered the weights stay "evaluation only" in our licence ledger (MOD-047 to MOD-049, ship-blocking).

## The message (English, for the issue or the email)

> **Subject: Licence and training data of the pretrained EasyOCR models (CRAFT detector, Arabic-script and English recognisers)**
>
> Hello, and thank you for EasyOCR. We are evaluating it for an on-premises document-analysis tool and would like to use it in a product, so we need the terms of the pretrained model files, not only of the code.
>
> The repository and the PyPI package are Apache-2.0. For the model files we downloaded from the releases page we could not find a licence statement:
> * `craft_mlt_25k.zip` (pre-v1.1.6) and the CRAFT detector,
> * `arabic.zip` (pre-v1.1.6; serves Arabic, Persian, Uyghur and Urdu),
> * `english_g2.zip` (v1.3).
>
> Could you tell us:
> 1. Are these files released under the same Apache-2.0 licence as the code, or under other terms? If other terms, where are they written?
> 2. What data were the recognisers trained on (public datasets, synthetic text, licensed data), and is anything in it restricted for commercial use?
> 3. For the CRAFT detector file, does the licence of the upstream CRAFT repository apply, and are there restrictions from its training data (for example SynthText or the ICDAR sets)?
> 4. Is there a statement we can cite in our compliance records?
>
> We also plan to train our own recogniser with your trainer (None-VGG-BiLSTM-CTC) on synthetic Urdu and English text rendered with open fonts; if you have advice on the Arabic-script character set (Urdu letters, Arabic-Indic digits) we would welcome it.
>
> Thank you.

## What we do while we wait (the licence-clean route, no dependency on the answer)

1. **Train our own recogniser.** EasyOCR's trainer is open (the `None-VGG-BiLSTM-CTC` recogniser, trained with the deep-text-recognition-benchmark code). We render Urdu and mixed Urdu and English lines from open fonts (Noto Naskh Arabic, Noto Nastaliq Urdu, Noto Sans; SIL OFL) and public-domain or CC0 text, with the degradations of the bake-off, and train on public data only. Cloud GPU training on public and synthetic data is already approved (decision D36); nothing from a case is ever uploaded. The result is a weights file that is ours.
2. **Use a licence-clean detector.** PaddleOCR's PP-OCRv5 detector (Apache-2.0, already in the product) finds the text boxes; our recogniser reads them. No CRAFT weights are needed.
3. **Measure it on the same sealed sets** as EasyOCR's pretrained recogniser (the same gates), so the two are compared like for like. The real Nastaliq newspaper rows are kept for evaluation only.

## If the answer is favourable

The pretrained files can be used under the stated terms; the ledger rows become "approved" with the answer attached and the file hashes (recorded at fetch: `craft_mlt_25k.zip` 8dc6a1c7..., `arabic.zip` 90b08b8e..., `english_g2.zip` 1b5eaebf..., in `C:/tmp/nexusai-media-data/manifest-20261117-easyocr.jsonl`). Our own recogniser can still replace them later if it measures better.
