import math
import sys
from pathlib import Path

import numpy as np

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

import plate_reader as pr  # noqa: E402


class FakeCV2:
    """The two OpenCV functions prepare_crop uses, without OpenCV, so the pure functions test anywhere."""
    COLOR_BGR2GRAY = 6
    INTER_AREA = 3

    @staticmethod
    def cvtColor(image, _code):
        return image.mean(axis=2).astype(np.uint8)

    @staticmethod
    def resize(image, size, interpolation=None):
        width, height = size
        rows = np.linspace(0, image.shape[0] - 1, height).astype(int)
        cols = np.linspace(0, image.shape[1] - 1, width).astype(int)
        return image[np.ix_(rows, cols)]


def one_hot_path(text, blanks=1):
    """Log-probabilities (T, C) that spell `text` with `blanks` blank frames between characters; every frame is certain."""
    classes = len(pr.CHARSET) + 1
    frames = []
    for index, char in enumerate(text):
        frames.append(pr.CHARSET.index(char) + 1)
        if index < len(text) - 1:
            frames.extend([0] * blanks)
    matrix = np.full((len(frames), classes), math.log(1e-6), np.float32)
    for t, cls in enumerate(frames):
        matrix[t, cls] = math.log(1 - 1e-3)
    return matrix


def test_prepare_crop_shape_range_and_two_line_layout():
    one_line = np.full((50, 240, 3), 255, np.uint8)
    out = pr.prepare_crop(one_line, FakeCV2)
    assert out.shape == (1, pr.HEIGHT, pr.WIDTH) and out.dtype == np.float32
    assert float(out.max()) <= 1.0 and float(out.min()) >= -1.0
    two_line = np.zeros((100, 180, 3), np.uint8)  # aspect 1.8: two lines
    two_line[:50] = 255  # upper half white, lower half black
    out2 = pr.prepare_crop(two_line, FakeCV2)
    assert out2.shape == (1, pr.HEIGHT, pr.WIDTH)
    left, right = out2[0, :, : pr.WIDTH // 2], out2[0, :, pr.WIDTH // 2:]
    assert left.mean() > 0.9 and right.mean() < -0.9  # upper line left, lower line right
    assert pr.prepare_crop(np.zeros((1, 1), np.uint8), FakeCV2).sum() == 0  # a degenerate crop does not crash


def test_beam_reads_a_clear_path_and_collapses_repeats_only_across_blanks():
    best = pr.ctc_beam(one_hot_path("LEA1234"))[0][0]
    assert best == "LEA1234"
    doubled = pr.ctc_beam(one_hot_path("AA11", blanks=1))[0][0]  # a repeat separated by a blank is two characters
    assert doubled == "AA11"
    no_blank = pr.ctc_beam(one_hot_path("AB", blanks=0))[0][0]
    assert no_blank == "AB"
    classes = len(pr.CHARSET) + 1
    run = np.full((4, classes), math.log(1e-6), np.float32)
    run[:, pr.CHARSET.index("7") + 1] = math.log(1 - 1e-3)  # the same character in four frames in a row is one character
    assert pr.ctc_beam(run)[0][0] == "7"


def test_beam_returns_ranked_alternatives_and_confidence_is_a_share_of_the_beam():
    classes = len(pr.CHARSET) + 1
    matrix = one_hot_path("B8")
    matrix = matrix.copy()
    # second frame set (the 8) is uncertain between 8 and B
    t = matrix.shape[0] - 1
    matrix[t] = math.log(1e-6)
    matrix[t, pr.CHARSET.index("8") + 1] = math.log(0.6)
    matrix[t, pr.CHARSET.index("B") + 1] = math.log(0.39)
    ranked = pr.ctc_beam(matrix)
    assert [text for text, _ in ranked][:2] == ["B8", "BB"]
    confidence = pr.OnnxPlateReader.confidence(ranked)
    assert 0.55 < confidence < 0.65
    assert pr.OnnxPlateReader.confidence([]) == 0.0
    assert classes == 37
