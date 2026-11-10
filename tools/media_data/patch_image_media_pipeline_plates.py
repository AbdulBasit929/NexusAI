#!/usr/bin/env python3
"""Patch the RUNNING worker image's own media_pipeline.py for the plate engine switch (docs/data/plates-m5-preregistration-20261110.md, Amendment 5).

    python -X utf8 patch_image_media_pipeline_plates.py <image's media_pipeline.py> <output path>

The image's file is older than the repository's (it carries the OCR overlay), so the repository's file cannot be copied over it. This applies the same three edits the repository's media_pipeline.py has,
to the image's text, and refuses (writes nothing, exit 1) unless every anchor occurs exactly once: a processor `detector_version` attribute, its use in the candidate record, and the
FORENSIC_ANPR_ENGINE branch in configured_media_processor(). The default engine stays `fastalpr`; nothing changes until the switch is set.
"""
import sys

EDITS = [
    ('    detector_id = "yolo-v9-t-384-license-plate-end2end"\n    ocr_id = "cct-xs-v2-global-model"\n',
     '    detector_id = "yolo-v9-t-384-license-plate-end2end"\n    detector_version = "fast-alpr-0.4.0/open-image-models-0.6.0"\n    ocr_id = "cct-xs-v2-global-model"\n'),
    ('                "detector_version": "fast-alpr-0.4.0/open-image-models-0.6.0",\n', '                "detector_version": self.detector_version,\n'),
    ('    if os.getenv("FORENSIC_ANPR_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:\n        image_processors.append(FastALPRImageProcessor.from_environment())\n',
     '''    if os.getenv("FORENSIC_ANPR_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:
        if os.getenv("FORENSIC_ANPR_ENGINE", "fastalpr").strip().lower() == "nexus":
            try:
                from ingestion.forensic_records.plate_processor_nexus import NexusPlateImageProcessor
            except ModuleNotFoundError:  # pragma: no cover - direct worker image execution
                from plate_processor_nexus import NexusPlateImageProcessor
            image_processors.append(NexusPlateImageProcessor.from_environment())
        else:
            image_processors.append(FastALPRImageProcessor.from_environment())
'''),
]


def main():
    source, target = sys.argv[1], sys.argv[2]
    text = open(source, encoding="utf-8", newline="").read()
    if "\r\n" in text:
        sys.exit("the image file has CRLF line endings; this patcher expects LF")
    for old, _ in EDITS:
        if text.count(old) != 1:
            sys.exit(f"anchor not found exactly once ({text.count(old)}): {old[:70]!r}; nothing written")
    if "FORENSIC_ANPR_ENGINE" in text:
        sys.exit("the image file already has the plate engine switch; nothing written")
    for old, new in EDITS:
        text = text.replace(old, new)
    open(target, "w", encoding="utf-8", newline="").write(text)
    print(f"patched: {len(EDITS)} edits, {target}")


if __name__ == "__main__":
    main()
