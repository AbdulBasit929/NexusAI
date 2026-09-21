# NX-MMR internal demo third-party notices

Scope: private internal development and team-lead demonstration. This notice is
an engineering packaging record, not legal advice or authorization for external
redistribution or customer delivery.

## ankandrew MIT components and project-published assets

The demo uses `fast-alpr`, `fast-plate-ocr`, `open-image-models`, the
`yolo-v9-t-384-license-plates-end2end.onnx` project release asset, and the
`cct-xs-v2-global-model` ONNX/config project release assets.

Copyright (c) 2024 ankandrew

These components are distributed by their upstream projects under the MIT
License. Retain the complete upstream MIT license text with copies or substantial
portions. The upstream model hubs publish the exact asset locations used by the
hash-pinned demo. No separate restrictive model term was located during this
review. External redistribution remains a separate review boundary.

- https://github.com/ankandrew/open-image-models/blob/main/LICENSE
- https://github.com/ankandrew/open-image-models/blob/main/open_image_models/detection/core/hub.py
- https://github.com/ankandrew/fast-plate-ocr/blob/master/LICENSE
- https://github.com/ankandrew/fast-plate-ocr/releases/tag/v1.1.0

## PaddleOCR and PaddlePaddle

The printed-text demo uses PaddleOCR/PaddlePaddle and the official
`PP-OCRv5_mobile_det`, `en_PP-OCRv5_mobile_rec`, and
`arabic_PP-OCRv5_mobile_rec` model payloads.

Copyright (c) 2016 PaddlePaddle Authors

The upstream projects use Apache License 2.0. Preserve the Apache-2.0 license,
retained attribution/notices, and changed-file notices where applicable. No
separate restrictive term was located in the official model documentation or
cached payloads. External packaging/redistribution remains a separate review.

- https://github.com/PaddlePaddle/PaddleOCR/blob/main/LICENSE
- https://github.com/PaddlePaddle/Paddle/blob/develop/LICENSE
- https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/pipeline_usage/OCR.md
- https://github.com/PaddlePaddle/PaddleOCR/blob/main/docs/version3.x/module_usage/text_detection.en.md

## Explicitly excluded

The internal demo does not enable or use the Ultralytics runtime,
`yolov8n.pt`, or the YOLOv8-derived `license_plate_detector.pt`. Those remain
blocked until an applicable commercial/R&D agreement or approved AGPL posture
is documented.
