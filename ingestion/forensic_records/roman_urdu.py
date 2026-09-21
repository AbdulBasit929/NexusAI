"""Deterministic, identifier-safe Urdu-script to Roman Urdu representation."""

from __future__ import annotations

import re
import uuid
from typing import Any


ROMAN_URDU_SEGMENT_CONTRACT = "forensics.audio-roman-urdu-segment/v1"
ROMAN_URDU_PROCESSOR = "nexusai-deterministic-urdu-transliteration"
ROMAN_URDU_PROCESSOR_REVISION = "roman-urdu-m1-v1"

_URDU_CHARACTER = re.compile(r"[\u0600-\u06ff]")
_DEVANAGARI_CHARACTER = re.compile(r"[\u0900-\u097f]")
_IDENTIFIER = re.compile(
    r"(?<!\w)(?:\+?\d[\d().:/-]{2,}\d|[A-Za-z]{1,5}-?\d{2,}|\d{1,2}:\d{2})(?!\w)"
)

_TRANSLITERATION = {
    "ا": "a", "آ": "aa", "أ": "a", "إ": "i", "ب": "b", "پ": "p", "ت": "t", "ٹ": "t",
    "ث": "s", "ج": "j", "چ": "ch", "ح": "h", "خ": "kh", "د": "d", "ڈ": "d", "ذ": "z",
    "ر": "r", "ڑ": "r", "ز": "z", "ژ": "zh", "س": "s", "ش": "sh", "ص": "s", "ض": "z",
    "ط": "t", "ظ": "z", "ع": "", "غ": "gh", "ف": "f", "ق": "q", "ک": "k", "ك": "k",
    "گ": "g", "ل": "l", "م": "m", "ن": "n", "ں": "n", "و": "o", "ؤ": "o", "ہ": "h",
    "ھ": "h", "ۃ": "h", "ء": "", "ئ": "y", "ی": "i", "ي": "i", "ے": "e", "ۓ": "e",
    "َ": "a", "ِ": "i", "ُ": "u", "ْ": "", "ّ": "", "ٰ": "a", "ـ": "",
}


def contains_urdu_script(text: str) -> bool:
    return bool(_URDU_CHARACTER.search(text or ""))


def contains_devanagari_script(text: str) -> bool:
    return bool(_DEVANAGARI_CHARACTER.search(text or ""))


def romanize_urdu(text: str) -> str:
    """Create a reviewable Roman Urdu alias while preserving identifiers exactly."""

    if not text:
        return ""
    protected: dict[str, str] = {}

    def protect(match: re.Match[str]) -> str:
        key = f"\ufff0{len(protected)}\ufff1"
        protected[key] = match.group(0)
        return key

    guarded = _IDENTIFIER.sub(protect, text)
    roman = "".join(_TRANSLITERATION.get(character, character) for character in guarded)
    roman = re.sub(r"[ \t]+", " ", roman).strip()
    for key, identifier in protected.items():
        roman = roman.replace(key, identifier)
    return roman


def identifiers_in(text: str) -> list[str]:
    return [match.group(0) for match in _IDENTIFIER.finditer(text or "")]


def roman_urdu_payload(
    raw_text: str,
    *,
    parent_observation_id: str,
    evidence_id: str,
    version_id: str,
    source_file: str,
    start_seconds: float | None,
    end_seconds: float | None,
) -> dict[str, Any]:
    roman = romanize_urdu(raw_text)
    raw_identifiers = identifiers_in(raw_text)
    roman_identifiers = identifiers_in(roman)
    return {
        "observation_id": str(uuid.uuid5(uuid.NAMESPACE_URL, f"{parent_observation_id}:roman-urdu:v1")),
        "contract_version": ROMAN_URDU_SEGMENT_CONTRACT,
        "raw_urdu_text": raw_text,
        "roman_urdu_text": roman,
        "source_language": "ur",
        "representation_language": "ur-Latn",
        "authority_state": "derived_analyst_representation",
        "parent_observation_id": parent_observation_id,
        "parent_evidence_id": evidence_id,
        "parent_version_id": version_id,
        "source_file": source_file,
        "start_seconds": start_seconds,
        "end_seconds": end_seconds,
        "processor": ROMAN_URDU_PROCESSOR,
        "processor_revision": ROMAN_URDU_PROCESSOR_REVISION,
        "identifiers": raw_identifiers,
        "identifier_preservation_pass": raw_identifiers == roman_identifiers,
        "manual_review_required": True,
    }
