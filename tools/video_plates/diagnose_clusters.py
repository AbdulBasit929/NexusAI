#!/usr/bin/env python3
"""Why does the grouping lose plates in a long or noisy clip? Reads the clip with the frozen V4 configuration, then groups the observations twice: with the policy as it is and with the cap on active
groups lifted, and reports how many clusters each makes and how many survive the precision gates. Counts only; plate text is not printed.

    docker run --rm --network none --entrypoint python -v <repo>:/repo:ro -v <repo>/models/media:/models/media:ro -v <video dir>:/v:ro -v C:/tmp/video-plates:/clips \
        nexusai-media-eval:latest /repo/tools/video_plates/diagnose_clusters.py --video /v/clip.mp4 [--max-seconds N]
"""
import argparse
import dataclasses
import os
import sys
import uuid
from pathlib import Path

os.environ.setdefault("FORENSIC_ANPR_ONNX_PROVIDERS", "CPUExecutionProvider")
os.environ.setdefault("FORENSIC_ANPR_NEXUS_DETECTOR_MODEL_PATH", "/models/media/nexus-plates/rfdetr_alpr.onnx")
os.environ.setdefault("FORENSIC_ANPR_NEXUS_OCR_MODEL_PATH", "/models/media/nexus-plates/cct_s_v2_global.onnx")
os.environ.setdefault("FORENSIC_ANPR_NEXUS_OCR_CONFIG_PATH", "/models/media/nexus-plates/cct_s_v2_global_plate_config.yaml")
os.environ.setdefault("FORENSIC_ANPR_DETECTOR_MODEL_PATH", "/models/media/open-image-models/yolo-v9-t-384-license-plate-end2end/yolo-v9-t-384-license-plates-end2end.onnx")
os.environ.setdefault("FORENSIC_ANPR_OCR_MODEL_PATH", "/models/media/fast-plate-ocr/cct-xs-v2-global-model/cct_xs_v2_global.onnx")
os.environ.setdefault("FORENSIC_ANPR_OCR_CONFIG_PATH", "/models/media/fast-plate-ocr/cct-xs-v2-global-model/cct_xs_v2_global_plate_config.yaml")
sys.path.insert(0, "/repo")
sys.path.insert(0, "/repo/ingestion/forensic_records")

import cv2  # noqa: E402


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--video", required=True)
    parser.add_argument("--max-seconds", type=float, default=0)
    args = parser.parse_args()
    from plate_processor_nexus import NexusPlateImageProcessor
    from video_anpr_nexus_v4 import DEFAULT_POLICY, VideoANPRNexusV4Processor, default_engine_processor
    from video_anpr_parity_adapter import AggregationPolicy, TimeInterval, aggregate_observations
    from video_anpr_product_v2 import aggregate_product_observations
    processor = VideoANPRNexusV4Processor(NexusPlateImageProcessor.from_environment(minimum_confidence=0.5), cv2, extra_processors=(default_engine_processor(),), engine_every=(2, 1))
    allowed = [TimeInterval(0.0, args.max_seconds)] if args.max_seconds else None
    run = processor.run_video(Path(args.video), context={"source_sha256": "0" * 64, "source_file": "clip.mp4", "evidence_id": str(uuid.uuid4()), "version_id": str(uuid.uuid4())}, allowed_intervals=allowed)
    obs = run.observations
    texts = [o.normalized_ocr for o in obs]
    print(f"observations={len(obs)} distinct_strings={len(set(texts))} reads_of_4plus_chars={sum(len(t) >= 4 for t in texts)} mean_weight={sum(o.support_weight() for o in obs) / max(1, len(obs)):.2f}")
    for label, cap in (("as is (cap 128)", DEFAULT_POLICY.maximum_active_groups), ("cap lifted (4096)", 4096)):
        policy = dataclasses.replace(DEFAULT_POLICY, maximum_active_groups=cap)
        clusters = aggregate_observations(obs, AggregationPolicy(strategy="confidence_weighted_support", minimum_selected_support=1, maximum_active_groups=cap))
        groups = aggregate_product_observations(obs, policy)
        print(f"{label:18} clusters={len(clusters):4} support>=3 clusters={sum(g.support_count >= 3 for g in clusters):3} groups after the precision gates={len(groups)}")
    weak = dataclasses.replace(DEFAULT_POLICY, maximum_active_groups=4096, minimum_weighted_support=0.5)
    print(f"cap lifted and weighted-support floor 0.5: groups={len(aggregate_product_observations(obs, weak))}")


if __name__ == "__main__":
    main()
