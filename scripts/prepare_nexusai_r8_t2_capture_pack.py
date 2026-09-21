#!/usr/bin/env python3
"""Create a non-evidence T2 capture checklist and empty manifest workspace."""

from __future__ import annotations

import argparse
import json
from pathlib import Path


SCENES = [
    ("single-frontal-day-near", "printed_synthetic_target", "near", "frontal", "daylight", "none", "center", None),
    ("single-frontal-shadow-medium", "printed_synthetic_target", "medium", "frontal", "shadow", "none", "center", None),
    ("single-angle-day-medium", "controlled_mock_plate_board", "medium", "horizontal_moderate", "daylight", "none", "edge", None),
    ("single-vertical-glare", "controlled_mock_plate_board", "medium", "vertical_moderate", "glare", "none", "center", None),
    ("single-low-light", "controlled_mock_plate_board", "medium", "frontal", "low_light", "none", "center", None),
    ("single-slight-blur", "controlled_mock_plate_board", "medium", "frontal", "daylight", "slight", "center", None),
    ("single-motion-blur", "controlled_mock_plate_board", "medium", "horizontal_moderate", "daylight", "motion", "edge", None),
    ("single-small-far", "controlled_mock_plate_board", "far", "frontal", "daylight", "none", "small_target", None),
    ("single-large-near", "controlled_plate_crop", "near", "frontal", "daylight", "none", "large_target", None),
    ("single-occluded", "controlled_mock_plate_board", "medium", "frontal", "shadow", "none", "center", None),
    ("vehicle-plate-day", "controlled_owned_vehicle", "medium", "frontal", "daylight", "none", "center", None),
    ("vehicle-plate-angle", "controlled_owned_vehicle", "medium", "horizontal_moderate", "daylight", "none", "edge", None),
    ("vehicle-plate-shadow", "controlled_owned_vehicle", "medium", "frontal", "shadow", "none", "center", None),
    ("vehicle-plate-glare", "controlled_owned_vehicle", "medium", "frontal", "glare", "none", "center", None),
    ("multiple-plates", "controlled_mock_plate_board", "medium", "horizontal_moderate", "daylight", "none", "center", None),
    ("urdu-target-frontal", "printed_synthetic_target", "near", "frontal", "daylight", "none", "center", None),
    ("urdu-target-angle", "printed_synthetic_target", "medium", "horizontal_moderate", "shadow", "slight", "edge", None),
    ("latin-target-compressed", "printed_synthetic_target", "medium", "frontal", "daylight", "none", "center", None),
    ("negative-vehicle-body", "controlled_negative_scene", "medium", "not_applicable", "daylight", "none", "center", "vehicle_body"),
    ("negative-logo", "controlled_negative_scene", "near", "not_applicable", "daylight", "none", "center", "logo"),
    ("negative-street-sign", "controlled_negative_scene", "medium", "not_applicable", "daylight", "none", "edge", "street_sign"),
    ("negative-label", "controlled_negative_scene", "near", "not_applicable", "daylight", "none", "center", "rectangular_label"),
    ("negative-document", "controlled_negative_scene", "near", "not_applicable", "daylight", "none", "center", "document"),
    ("negative-screen", "controlled_negative_scene", "near", "not_applicable", "low_light", "none", "center", "screen"),
    ("negative-random-text", "controlled_negative_scene", "medium", "not_applicable", "daylight", "none", "edge", "random_text"),
    ("negative-vehicle-no-target", "controlled_negative_scene", "medium", "not_applicable", "daylight", "none", "center", "vehicle_without_target"),
    ("negative-structure", "controlled_negative_scene", "far", "not_applicable", "daylight", "none", "small_target", "background_structure"),
    ("negative-plain", "controlled_negative_scene", "medium", "not_applicable", "shadow", "none", "center", "plain_scene"),
    ("holdout-single-day", "printed_synthetic_target", "medium", "frontal", "daylight", "none", "center", None),
    ("holdout-single-angle", "controlled_mock_plate_board", "medium", "horizontal_moderate", "shadow", "slight", "edge", None),
    ("holdout-vehicle-low-light", "controlled_owned_vehicle", "medium", "frontal", "low_light", "none", "center", None),
    ("holdout-negative-text", "controlled_negative_scene", "medium", "not_applicable", "glare", "none", "edge", "random_text"),
]


def build_plan() -> dict:
    items = []
    for index, (name, source, distance, angle, lighting, blur, position, negative) in enumerate(SCENES, 1):
        items.append({
            "slot": index, "image_id": f"t2-{index:03d}-{name}",
            "filename": f"images/t2-{index:03d}-{name}.jpg",
            "partition": "holdout" if index >= 29 else "development",
            "capture_source_class": source, "camera_orientation": "landscape",
            "distance": distance, "perspective": angle, "lighting": lighting,
            "blur_level": blur, "target_position": position, "negative_class": negative,
            "instruction": "Use only synthetic non-road-use targets or owned/explicitly consented scenes; exclude unrelated people, plates, addresses, documents and screens.",
        })
    return {
        "contract_version": "forensics.controlled-demo-capture-plan/v1",
        "pack_id": "nexusai-r8-t2-anpr-demo", "planned_image_count": len(items),
        "development_count": 28, "holdout_count": 4,
        "holdout_policy": "Capture and annotate independently; do not inspect model output or tune against holdout slots.",
        "items": items,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--output-dir", type=Path, required=True)
    args = parser.parse_args()
    args.output_dir.mkdir(parents=True, exist_ok=True)
    (args.output_dir / "images").mkdir(exist_ok=True)
    (args.output_dir / "capture-plan.json").write_text(json.dumps(build_plan(), indent=2) + "\n", encoding="utf-8")
    manifest = {
        "contract_version": "forensics.controlled-demo-pack/v1", "pack_id": "nexusai-r8-t2-anpr-demo",
        "version": "1.0.0-draft", "tier": "T2", "status": "capture_pending",
        "purpose": "Controlled, owned and non-operational R8 ANPR demonstration and benchmark evidence",
        "ground_truth_policy": "independent_human_annotation_only",
        "storage_policy": "raw assets outside Git; manifest may enter Git only after privacy and path review",
        "capture_plan": "capture-plan.json", "required_image_count": 32,
        "partition_policy": "28 development images and 4 sealed holdout images", "images": [],
    }
    (args.output_dir / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"status": "pass", "output_dir": str(args.output_dir), "planned_images": 32, "manifest": str(args.output_dir / "manifest.json")}))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
