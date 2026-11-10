import random
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import plate_formats as pf  # noqa: E402


def test_normalise_keeps_letters_and_digits_and_converts_urdu_digits():
    assert pf.normalise("lea-14-1234") == "LEA141234"
    assert pf.normalise(" LEA 1234 ") == "LEA1234"
    assert pf.normalise("LEA-١٢٣٤") == "LEA1234"
    assert pf.normalise("LEA-۱۲۳۴") == "LEA1234"
    assert pf.normalise(None) == "" and pf.normalise("---") == ""


def test_formats_match_the_registered_shapes_only():
    assert pf.matching_formats("LEA141234") == ["L3_Y2_S4"]
    assert "L3_S4" in pf.matching_formats("LEA1234")
    assert pf.matching_formats("LEA12") == []
    assert pf.matching_formats("1234LEA") == []
    assert pf.matching_formats("") == []


def test_choose_prefers_a_format_fit_only_within_the_gap_and_never_edits_characters():
    # the first read already fits: kept as it is
    assert pf.choose([("LEA-1234", -0.2), ("LEA-1284", -1.5)]) == ("LEA1234", "L3_S4", [])
    # the first read fits nothing; a close second fits: chosen, and said so
    text, fmt, flags = pf.choose([("LEA-12", -0.3), ("LEA-1234", -1.1)])
    assert (text, flags) == ("LEA1234", ["format_chosen_over_first_read"]) and fmt == "L3_S4"
    # the only fitting candidate is far worse than the first read: first read returned, flagged
    text, fmt, flags = pf.choose([("LEA-12", -0.2), ("LEA-1234", -9.0)])
    assert (text, fmt, flags) == ("LEA12", None, ["format_unknown"])
    # nothing fits: the read is returned unchanged and flagged
    assert pf.choose([("XX", -1.0)]) == ("XX", None, ["format_unknown"])
    assert pf.choose([]) == ("", None, ["empty_read"])


def test_confusion_cost_ranks_lookalikes_below_other_differences():
    assert pf.confusion_ranked("LEA1234", "LEA1234") == 0
    assert pf.confusion_ranked("LEA0234", "LEAO234") == 1  # 0 and O are a confusable pair
    assert pf.confusion_ranked("LEA8234", "LEAB234") == 1  # 8 and B too
    assert pf.confusion_ranked("LEA0234", "LEAX234") == 2  # any other substitution costs more
    assert pf.confusion_ranked("LEA234", "LEA1234") == 2   # so does a missing character


def test_random_plates_follow_their_own_format_and_are_deterministic():
    first = [pf.random_plate(random.Random(7)) for _ in range(3)]
    again = [pf.random_plate(random.Random(7)) for _ in range(3)]
    assert first == again
    rng = random.Random(11)
    seen_formats, seen_layouts = set(), set()
    for _ in range(500):
        plate = pf.random_plate(rng)
        assert plate["format"] in pf.matching_formats(plate["norm"]), plate
        assert plate["layout"] in ("one_line", "two_line")
        seen_formats.add(plate["format"])
        seen_layouts.add(plate["layout"])
    assert len(seen_formats) >= 4 and seen_layouts == {"one_line", "two_line"}


def test_registry_declares_its_status_and_series():
    registry = pf.load_registry()
    assert registry["status"] == "approximation_to_be_reviewed"
    assert registry["registry_version"].startswith("pk-plates-")
    assert all(len(code) == 3 for series in registry["series"].values() for code in series)
