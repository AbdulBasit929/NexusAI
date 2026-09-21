# R8 isolated evaluator third-party notices

This notice describes the isolated evaluation image only. It does not assign a
production NexusAI model role.

| Component | Evaluated version/artifact | License evidence | Disposition |
| --- | --- | --- | --- |
| PaddleOCR | Python package 3.7.0; `PP-OCRv5_mobile_det`, `en_PP-OCRv5_mobile_rec`, `arabic_PP-OCRv5_mobile_rec` | Package metadata and official project repository identify Apache License 2.0 | permitted for isolated evaluation; retain Apache notice before redistribution/production packaging |
| PaddlePaddle | Python package 3.3.1 | Installed package metadata identifies Apache Software License | permitted for isolated evaluation; dependency notices remain required |
| Tesseract | Debian Tesseract 5.5.0 | Installed Debian copyright and official repository identify Apache-2.0 | permitted for isolated evaluation |
| Tesseract `eng`/`urd` data | Debian language data derived from `tessdata_fast` | Installed Debian copyright and official `tessdata_fast` repository identify Apache-2.0 | permitted for isolated evaluation |
| Pillow | Python package 12.1.1 | Pillow license applies; retain the package's bundled license in any redistributed evaluator | permitted for isolated fixture generation |

Primary license references:

- <https://github.com/PaddlePaddle/PaddleOCR/blob/main/LICENSE>
- <https://github.com/PaddlePaddle/Paddle/blob/develop/LICENSE>
- <https://github.com/tesseract-ocr/tesseract/blob/main/LICENSE>
- <https://github.com/tesseract-ocr/tessdata_fast/blob/main/LICENSE>
- <https://github.com/python-pillow/Pillow/blob/main/LICENSE>

The evaluator image and cache are not a redistribution artifact. Before any
production packaging, export the complete installed dependency license inventory
and retain required copyright, patent and attribution notices.
