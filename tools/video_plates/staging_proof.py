import json
import os
import sys
import time
import uuid
from collections import Counter
from pathlib import Path

env = {
    "FORENSIC_ANPR_ENABLED": "true", "FORENSIC_ANPR_ENGINE": "nexus", "FORENSIC_ANPR_ONNX_PROVIDERS": "CPUExecutionProvider",
    "FORENSIC_ANPR_NEXUS_DETECTOR_MODEL_PATH": "/models/media/nexus-plates/rfdetr_alpr.onnx",
    "FORENSIC_ANPR_NEXUS_OCR_MODEL_PATH": "/models/media/nexus-plates/cct_s_v2_global.onnx",
    "FORENSIC_ANPR_NEXUS_OCR_CONFIG_PATH": "/models/media/nexus-plates/cct_s_v2_global_plate_config.yaml",
    "FORENSIC_ANPR_DETECTOR_MODEL_PATH": "/models/media/open-image-models/yolo-v9-t-384-license-plate-end2end/yolo-v9-t-384-license-plates-end2end.onnx",
    "FORENSIC_ANPR_OCR_MODEL_PATH": "/models/media/fast-plate-ocr/cct-xs-v2-global-model/cct_xs_v2_global.onnx",
    "FORENSIC_ANPR_OCR_CONFIG_PATH": "/models/media/fast-plate-ocr/cct-xs-v2-global-model/cct_xs_v2_global_plate_config.yaml",
    "FORENSIC_OCR_ENABLED": "false", "FORENSIC_ASR_ENABLED": "false", "FORENSIC_FACE_ENABLED": "false", "FORENSIC_IMAGE_EMBEDDING_ENABLED": "false",
    "FORENSIC_VIDEO_ANPR_V4_ENABLED": sys.argv[2] if len(sys.argv) > 2 else "true",
}
os.environ.update(env)
sys.path.insert(0, "/app")
import media_pipeline as mp  # noqa: E402

processor = mp.configured_media_processor()
print("video processor:", type(processor.video_anpr_processor).__name__ if processor.video_anpr_processor else None, "| frame processor:", type(processor.video_frame_processor).__name__ if processor.video_frame_processor else None)
started = time.time()
result = processor.process(Path(sys.argv[1]), modality="video", evidence_id=str(uuid.uuid4()), version_id=str(uuid.uuid4()), source_sha256="a" * 64, source_file=os.path.basename(sys.argv[1]), admitted_metadata={})
print("seconds", round(time.time() - started, 1), "| readiness", result.readiness)
types = Counter(o.observation_type for o in result.observations)
print("observation types", dict(types))
print("metadata result_states", result.metadata.get("result_states"), "| group_count", (result.metadata.get("video_anpr_v4") or {}).get("group_count"), "| execution_state", (result.metadata.get("video_anpr_v4") or {}).get("execution_state"))
groups = [o for o in result.observations if o.observation_type == "video_anpr_plate_group_observation"]
frames = [o for o in result.observations if o.observation_type == "anpr_ocr_observation"]
print("groups", [(g.payload["normalized_plate_text"], g.payload["sightings_count"], g.payload["first_seen_seconds"], g.payload["last_seen_seconds"]) for g in groups][:6])
if frames:
    first = frames[0]
    print("a frame observation: contract", first.contract_version, "| locator keys", sorted(first.citation_locator), "| payload engine", first.payload.get("engine_processor"), "| review required", first.payload.get("manual_review_required"))
print("limitations", list(result.limitations)[:4])
