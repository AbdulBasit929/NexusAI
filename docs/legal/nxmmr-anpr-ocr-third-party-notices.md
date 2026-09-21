# NX-MMR ANPR/OCR proposed third-party notices

Status: **PROPOSED_NOT_AUTHORIZED**. This file is an engineering packaging
input, not legal advice and not evidence that activation or redistribution has
been approved. Entries marked conditional must be closed before copying their
assets into `models/media`.

## ankandrew inference libraries

FastALPR, FastPlateOCR, and open-image-models are distributed under the MIT
License.

> Copyright (c) 2024 ankandrew
>
> Permission is hereby granted, free of charge, to any person obtaining a copy
> of this software and associated documentation files (the "Software"), to deal
> in the Software without restriction, subject to inclusion of the copyright
> notice and permission notice in all copies or substantial portions.

Package the complete upstream MIT text from:

- <https://github.com/ankandrew/fast-alpr/blob/master/LICENSE>
- <https://github.com/ankandrew/fast-plate-ocr/blob/master/LICENSE>
- <https://github.com/ankandrew/open-image-models/blob/master/LICENSE>

Conditional checkpoint notice: the exact
`yolo-v9-t-384-license-plates-end2end.onnx` and
`cct-xs-v2-global-model` ONNX/config rights must be confirmed as covered by
those MIT grants before packaging. The repository license alone is not treated
as checkpoint-specific proof.

## Muhammad Zeerak Khan Video plate detector source

The surrounding `Automatic-License-Plate-Recognition-using-YOLOv8` repository
contains an MIT license with copyright (c) 2023 Muhammad Zeerak Khan. Package
the complete upstream text from:

- <https://github.com/Muhammad-Zeerak-Khan/Automatic-License-Plate-Recognition-using-YOLOv8/blob/main/LICENSE>

Conditional checkpoint notice: this does not independently settle the exact
`license_plate_detector.pt` rights. The local snapshot lacks Git metadata; the
checkpoint was trained with Ultralytics YOLOv8 and the repository points to the
Roboflow `license-plate-recognition-rxg4e` v4 dataset.

If the checkpoint is lawfully admitted, also include:

> License Plate Recognition Dataset, Roboflow Universe Projects,
> <https://universe.roboflow.com/roboflow-universe-projects/license-plate-recognition-rxg4e/dataset/4>,
> licensed CC BY 4.0.

## Ultralytics YOLO

Ultralytics 8.0.114 metadata identifies AGPL-3.0. Ultralytics' current official
licensing page says its trained models are AGPL-3.0 by default and that private,
internal, SaaS, proprietary, or commercial use without the AGPL disclosure path
requires an applicable Enterprise/commercial license.

- <https://www.ultralytics.com/license>
- <https://www.ultralytics.com/legal/terms-of-service>

Before packaging `ultralytics` or either YOLOv8-derived `.pt` file, record one
approved path:

1. an AGPL-3.0 compliance decision with complete corresponding-source and
   network-use obligations implemented; or
2. the applicable executed Ultralytics commercial agreement and its scope.

No approved path is currently recorded.

## PaddleOCR and PaddlePaddle

PaddleOCR and PaddlePaddle are distributed under Apache License 2.0. Include a
complete copy of Apache-2.0, retain applicable copyright/patent/trademark and
attribution notices, mark modified files, and propagate any upstream NOTICE
content that accompanies the exact distribution.

> Copyright (c) 2016 PaddlePaddle Authors. All Rights Reserved.
> Licensed under the Apache License, Version 2.0.

- <https://github.com/PaddlePaddle/PaddleOCR/blob/main/LICENSE>
- <https://github.com/PaddlePaddle/Paddle/blob/develop/LICENSE>

Conditional checkpoint notice: the existing cache directories for
`PP-OCRv5_mobile_det`, `en_PP-OCRv5_mobile_rec`, and
`arabic_PP-OCRv5_mobile_rec` contain `inference.yml` but no LICENSE or NOTICE.
Capture the official source URL/version receipt and confirmation that the exact
checkpoint payloads are covered by Apache-2.0 before copying them into the live
model tree.

## Tesseract fallback

Tesseract and all `tessdata_fast` data are Apache-2.0; Tesseract documents its
Leptonica dependency as BSD-2-Clause. Package the complete Apache-2.0 and
Leptonica BSD notice texts.

- <https://github.com/tesseract-ocr/tesseract/blob/main/LICENSE>
- <https://github.com/tesseract-ocr/tessdata_fast/blob/main/LICENSE>

The pinned fallback files are `eng.traineddata` at revision
`65727574dfcd264acbb0c3e07860e4e9e9b22185` and `urd.traineddata` at the same
revision. Their local hashes remain in the activation-readiness manifest.
