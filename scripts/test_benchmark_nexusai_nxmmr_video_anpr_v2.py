import importlib.util
from pathlib import Path


MODULE_PATH = Path(__file__).with_name("benchmark_nexusai_nxmmr_video_anpr_v2.py")
SPEC = importlib.util.spec_from_file_location("nxmmr_video_v2_benchmark", MODULE_PATH)
MODULE = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(MODULE)


def test_rank_prefers_exact_then_recall_then_group_precision():
    def row(exact, recall, group, fpr, count, identity):
        return {
            "policy_id": identity,
            "group_count": count,
            "score": {
                "events": {
                    "normalized_exact_plate": {"f1": exact},
                    "event_detection_recall": recall,
                    "negative_frame_false_positive_rate": fpr,
                },
                "grouping": {"exact_group_f1": group},
            },
        }
    assert MODULE.rank(row(.3, .7, .2, .1, 5, "a")) > MODULE.rank(row(.2, 1, 1, 0, 1, "b"))
