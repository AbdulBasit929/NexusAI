#!/usr/bin/env python3
"""Patch the media_pipeline.py that is INSIDE the running worker image (not the repository's newer file) so it can start the new OCR backends, and write the result for the overlay build.

    python -X utf8 tools/media_data/patch_image_media_pipeline.py <extracted image media_pipeline.py> <output path>

Why: the worker image in production is the older build plus the Ingest v2 overlay; its media_pipeline.py is 500 lines older than the repository's and a straight copy of the repository file would
bring in unrelated, unreviewed changes (video plate readers, contextual routing) next to an older worker.py. This script makes exactly two edits to the image's own file, each of which must match
once: (1) the Tesseract processor can take its languages and page mode as arguments (the router builds an English and an Urdu reader from the same pinned data); (2) the OCR branch of the factory
selects the `auto` router or the `onnx` backend from FORENSIC_OCR_BACKEND and otherwise keeps the existing Tesseract behaviour byte for byte. Line endings of the input are preserved.
"""
import sys

EDITS = [
    (
        '    @classmethod\n    def from_environment(cls) -> "TesseractImageOCRProcessor":\n        executable_value',
        '    @classmethod\n    def from_environment(cls, *, languages: str | None = None, page_segmentation_mode: int | None = None) -> "TesseractImageOCRProcessor":\n'
        '        """`languages` and `page_segmentation_mode` override the environment for one instance (the OCR router builds an English and an Urdu reader from the same pinned data)."""\n'
        '        executable_value',
    ),
    (
        '            languages=os.getenv("FORENSIC_OCR_LANGUAGES", "eng+urd"),\n',
        '            languages=languages or os.getenv("FORENSIC_OCR_LANGUAGES", "eng+urd"),\n',
    ),
    (
        '            page_segmentation_mode=int(os.getenv("FORENSIC_OCR_PSM", "11")),\n        )\n',
        '            page_segmentation_mode=int(os.getenv("FORENSIC_OCR_PSM", "11")) if page_segmentation_mode is None else page_segmentation_mode,\n        )\n',
    ),
    (
        '    if os.getenv("FORENSIC_OCR_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:\n        image_processors.append(TesseractImageOCRProcessor.from_environment())\n',
        '    if os.getenv("FORENSIC_OCR_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:\n'
        '        ocr_backend = os.getenv("FORENSIC_OCR_BACKEND", "tesseract").strip().lower()\n'
        '        if ocr_backend == "auto":\n'
        '            from ocr_router import OcrRouter\n'
        '            image_processors.append(OcrRouter.from_environment())\n'
        '        elif ocr_backend == "onnx":\n'
        '            from onnx_ocr_processor import OnnxPaddleOCRProcessor\n'
        '            image_processors.append(OnnxPaddleOCRProcessor.from_environment())\n'
        '        else:\n'
        '            image_processors.append(TesseractImageOCRProcessor.from_environment())\n',
    ),
]


def main(source, target):
    raw = open(source, "rb").read().decode("utf-8")
    crlf = "\r\n" in raw
    text = raw.replace("\r\n", "\n")
    for old, new in EDITS:
        found = text.count(old)
        if found != 1:
            sys.exit(f"expected exactly one match, found {found}: {old[:80]!r}")
        text = text.replace(old, new)
    open(target, "wb").write((text.replace("\n", "\r\n") if crlf else text).encode("utf-8"))
    print(f"patched {source} -> {target} ({'CRLF' if crlf else 'LF'}, {len(EDITS)} edits)")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit(__doc__)
    main(sys.argv[1], sys.argv[2])
