#!/usr/bin/env python3
"""Report transparent indicative WER/CER for a ground-truth sidecar."""

from __future__ import annotations

import argparse
import json
import sys
import unicodedata
from pathlib import Path


def normalize(value: str) -> str:
    value = unicodedata.normalize("NFKC", value).casefold()
    value = "".join(
        " " if unicodedata.category(character).startswith(("P", "S")) else character
        for character in value
    )
    return " ".join(value.split())


def edit_distance(reference: list[str], hypothesis: list[str]) -> int:
    previous = list(range(len(hypothesis) + 1))
    for row, expected in enumerate(reference, start=1):
        current = [row]
        for column, actual in enumerate(hypothesis, start=1):
            current.append(
                min(
                    current[-1] + 1,
                    previous[column] + 1,
                    previous[column - 1] + (expected != actual),
                )
            )
        previous = current
    return previous[-1]


def main() -> None:
    if hasattr(sys.stdout, "reconfigure"):
        sys.stdout.reconfigure(encoding="utf-8")
    parser = argparse.ArgumentParser()
    parser.add_argument("ground_truth", type=Path)
    parser.add_argument("hypothesis")
    args = parser.parse_args()

    reference = normalize(args.ground_truth.read_text(encoding="utf-8-sig"))
    hypothesis = normalize(args.hypothesis)
    reference_words = reference.split()
    hypothesis_words = hypothesis.split()
    reference_characters = list(reference.replace(" ", ""))
    hypothesis_characters = list(hypothesis.replace(" ", ""))
    word_edits = edit_distance(reference_words, hypothesis_words)
    character_edits = edit_distance(reference_characters, hypothesis_characters)
    print(
        json.dumps(
            {
                "normalization": "Unicode NFKC + casefold + punctuation/symbol to space + whitespace collapse; CER excludes spaces",
                "reference": reference,
                "hypothesis": hypothesis,
                "reference_word_count": len(reference_words),
                "word_edits": word_edits,
                "wer": round(word_edits / len(reference_words), 4),
                "reference_character_count": len(reference_characters),
                "character_edits": character_edits,
                "cer": round(character_edits / len(reference_characters), 4),
            },
            ensure_ascii=False,
            indent=2,
        )
    )


if __name__ == "__main__":
    main()
