"""Plate-format registry: normalisation, format checks, constrained choice between candidate reads, and the recipe the synthetic generator draws from.

The registry (plate_format_registry_v1.json) is DATA, versioned by date. It constrains decoding and says how to render training plates; it never fills in a character. A read that fits no
format is returned as read with the flag `format_unknown`. Pure standard library, so the worker, the training tools and the tests can all import it.
"""
from __future__ import annotations

import json
import random
import re
from functools import lru_cache
from pathlib import Path

REGISTRY_PATH = Path(__file__).with_name("plate_format_registry_v1.json")
ALNUM = re.compile(r"[^A-Z0-9]")


@lru_cache(maxsize=1)
def load_registry() -> dict:
    registry = json.loads(REGISTRY_PATH.read_text(encoding="utf-8"))
    for fmt in registry["formats"]:
        fmt["_re"] = re.compile(fmt["pattern"])
    return registry


def _digit_table() -> dict[int, str]:
    maps = load_registry()["digit_maps"]
    table: dict[int, str] = {}
    for name in ("arabic_indic", "extended_arabic_indic"):
        for index, char in enumerate(maps[name]):
            table[ord(char)] = str(index)
    return table


def normalise(text: str | None) -> str:
    """Upper case, Arabic-Indic digits to Latin, everything that is not a letter or digit removed. Used for search keys and for scoring; the stored reading keeps what was read."""
    if not text:
        return ""
    return ALNUM.sub("", text.translate(_digit_table()).upper())


def matching_formats(norm: str) -> list[str]:
    return [fmt["id"] for fmt in load_registry()["formats"] if fmt["_re"].match(norm)]


def choose(candidates: list[tuple[str, float]]) -> tuple[str, str | None, list[str]]:
    """Pick the best candidate that fits a registered format.

    `candidates` are (text, log-probability) pairs, best first as the decoder produced them. Returns (normalised text, format id or None, flags). The first candidate that fits a format wins even when
    it is not the decoder's first choice, but only if its log-probability is within `MAX_GAP` of the first choice; otherwise the decoder's first choice is returned unchanged and flagged
    `format_unknown`. Nothing is ever edited character by character.
    """
    if not candidates:
        return "", None, ["empty_read"]
    best_text, best_score = candidates[0]
    best_norm = normalise(best_text)
    found = matching_formats(best_norm)
    if found:
        return best_norm, found[0], []
    for text, score in candidates[1:]:
        norm = normalise(text)
        found = matching_formats(norm)
        if found and best_score - score <= MAX_GAP:
            return norm, found[0], ["format_chosen_over_first_read"]
    return best_norm, None, ["format_unknown"]


MAX_GAP = 4.0  # natural-log units between the decoder's first choice and a candidate that fits a format


def confusion_ranked(norm_a: str, norm_b: str) -> int:
    """Edit cost of turning a into b where a substitution inside a confusable pair costs 1 and any other substitution 2, an insertion or deletion 2. Used to rank near matches, never to merge them."""
    pairs = {frozenset(p) for p in load_registry()["confusable_pairs"]}
    la, lb = len(norm_a), len(norm_b)
    row = [j * 2 for j in range(lb + 1)]
    for i in range(1, la + 1):
        previous, row[0] = row[0], i * 2
        for j in range(1, lb + 1):
            substitution = 0 if norm_a[i - 1] == norm_b[j - 1] else (1 if frozenset((norm_a[i - 1], norm_b[j - 1])) in pairs else 2)
            previous, row[j] = row[j], min(row[j] + 2, row[j - 1] + 2, previous + substitution)
    return row[lb]


def random_plate(rng: random.Random) -> dict:
    """One synthetic plate: region, format, layout, display text, normalised text. Letters come from a listed series 60% of the time and are random otherwise, so a reader cannot memorise the series list."""
    registry = load_registry()
    region = rng.choice(list(registry["series"]))
    fmt = rng.choices(registry["formats"], weights=[f["weight"] for f in registry["formats"]])[0]
    layout = "two_line" if "two_line" in fmt["layouts"] and rng.random() < 0.15 else "one_line"  # the real sample is all one line; two-line (motorcycle) plates stay as a minority
    letters_count = 3 if fmt["id"].startswith("L3") else 2
    if rng.random() < 0.6:
        pool = [s for s in registry["series"][region] if len(s) == letters_count] or ["".join(rng.choices("ABCDEFGHJKLMNPRSTUVWXYZ", k=letters_count))]
        letters = rng.choice(pool)
    else:
        letters = "".join(rng.choices("ABCDEFGHIJKLMNOPQRSTUVWXYZ", k=letters_count))
    parts = {f"L{letters_count}": letters, "Y2": f"{rng.randrange(10, 26):02d}", "S4": f"{rng.randrange(0, 10000):04d}", "S3": f"{rng.randrange(0, 1000):03d}", "S2": f"{rng.randrange(0, 100):02d}"}
    display = fmt["display"].format(**parts)
    return {"region": region, "format": fmt["id"], "layout": layout, "display": display, "norm": normalise(display)}
