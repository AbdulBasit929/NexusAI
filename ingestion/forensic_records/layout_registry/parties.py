"""Party numbers, device identifiers and cell keys: classification and normalisation.

Number classes (roadmap finding F4): a number that identifies a person's line is `person_like`; `short_code`, `sender_id`,
`international`, `service_label` and `unknown` never enter a person-to-person graph without a decision. The module never
infers an operator from a number prefix (the country profile forbids it).
"""

from __future__ import annotations

import re
import unicodedata

SERVICE_LABELS = frozenset({
    "INTERNET", "UNKNOWN", "GPRS", "DATA", "PACKET", "SHORTCODE", "SHORT_CODE", "INTERNATIONAL", "NEW_NETWORK", "NULL", "N/A", "NA",
})
SENTINELS = frozenset({"?", "-1", "-", "--", "NULL", "N/A", "NA", "NONE", "0"})
USSD = re.compile(r"^\*[\d*#]*#$")
NUMERICISH = re.compile(r"^\+?[\d\s\-().]+$")


def clean(value: object) -> str:
    return "" if value is None else str(value).strip()


def classify_party(raw: object) -> tuple[str | None, str | None, list[str]]:
    """Return (kind, normalised, flags). `normalised` is set only when it is unambiguous:
    92 + ten digits for a Pakistani mobile number, and E.164 digits without the 00 prefix for an international number."""
    text = clean(raw)
    if text.endswith(".0") and text[:-2].isdigit():
        text = text[:-2]
    if not text:
        return None, None, ["party_missing"]
    if text.upper() in SENTINELS:
        return None, None, ["party_sentinel"]
    upper = text.upper().replace(" ", "_")
    if upper in SERVICE_LABELS:
        return "service_label", None, []
    if USSD.match(text):
        return "short_code", None, ["ussd_code"]
    if not NUMERICISH.match(text):
        return "sender_id", None, []
    plus = text.startswith("+")
    digits = re.sub(r"\D", "", text)
    if digits.startswith("0092") and len(digits) == 14 and digits[4] == "3":
        return "person_like", "92" + digits[4:], []
    if digits.startswith("92") and len(digits) == 12 and digits[2] == "3":
        return "person_like", digits, []
    if digits.startswith("03") and len(digits) == 11:
        return "person_like", "92" + digits[1:], []
    if digits.startswith("3") and len(digits) == 10 and not plus:
        return "person_like", "92" + digits, []
    if (plus or digits.startswith("00")) and 11 <= len(digits) <= 16:
        return "international", digits[2:] if digits.startswith("00") else digits, []
    if 1 <= len(digits) <= 6:
        return "short_code", None, []
    return "unknown", None, ["party_format_unrecognized"]


def device_identifier(raw: object, kind: str) -> tuple[str | None, str | None, list[str]]:
    """IMEI or IMSI: digits only. Returns (digits, tac, flags). TAC (the first eight digits) only where at least 14 digits exist."""
    text = clean(raw)
    if text.endswith(".0") and text[:-2].isdigit():
        text = text[:-2]
    if not text:
        return None, None, []
    digits = re.sub(r"\D", "", text)
    if not digits or digits != re.sub(r"[\s\-]", "", text):
        return None, None, ["%s_not_numeric" % kind]
    flags: list[str] = []
    tac = None
    if kind == "imei":
        if len(digits) not in (14, 15, 16):
            flags.append("imei_length_%d" % len(digits))
        if len(digits) >= 14:
            tac = digits[:8]
    elif kind == "imsi" and len(digits) != 15:
        flags.append("imsi_length_%d" % len(digits))
    return digits, tac, flags


def norm_cell_part(part: object) -> str | None:
    """Same rule as forensic.norm_cell_part (migration 013): numeric parts lose leading zeros, others are lower-cased."""
    text = clean(part)
    if not text:
        return None
    if text.endswith(".0") and text[:-2].isdigit():
        text = text[:-2]
    if text.isdigit():
        return text.lstrip("0") or "0"
    return text.lower()


def make_cell_key(plmn: object, lac_tac: object, ci: object) -> str | None:
    """Same rule as forensic.make_cell_key (migration 013): plmn:lac_tac:ci, PLMN zeros kept, a missing part is '?'."""
    ci_part = norm_cell_part(ci)
    if ci_part is None:
        return None
    plmn_part = clean(plmn).lower() or "?"
    return "%s:%s:%s" % (plmn_part, norm_cell_part(lac_tac) or "?", ci_part)


def parse_coordinate(raw: object, kind: str) -> tuple[float | None, list[str]]:
    text = clean(raw)
    if not text:
        return None, []
    try:
        value = float(text)
    except ValueError:
        return None, ["coordinate_not_numeric"]
    limit = 90.0 if kind == "lat" else 180.0
    if abs(value) > limit:
        return None, ["coordinate_out_of_range"]
    return value, []


def split_combined_coordinates(raw: object, order: str) -> tuple[float | None, float | None, list[str]]:
    """A single cell that holds two numbers (for example 'lon lat'). Returns (lat, lon, flags)."""
    text = clean(raw)
    if not text or text == "?":
        return None, None, []
    parts = re.findall(r"-?\d+(?:\.\d+)?", text)
    if len(parts) != 2:
        return None, None, ["coordinate_not_numeric"]
    first, second = float(parts[0]), float(parts[1])
    lon, lat = (first, second) if order == "lon_lat" else (second, first)
    if abs(lat) > 90 or abs(lon) > 180:
        return None, None, ["coordinate_out_of_range"]
    return lat, lon, []


# ---- plates (migration 012: forensic.normalize_plate; the two must agree on every token, a test runs both on one fixture)
_PLATE_DIGIT_SOURCES = "".join(chr(c) for c in range(0x0660, 0x066A)) + "".join(chr(c) for c in range(0x06F0, 0x06FA))
# U+00B7 middle dot, U+2022 bullet, U+2010 to U+2015 hyphens and dashes, U+2212 minus: deleted, as `translate` deletes the characters that have no target
_PLATE_DELETED = "\u00b7\u2022\u2010\u2011\u2012\u2013\u2014\u2015\u2212"
_PLATE_TABLE = {ord(c): str(i % 10) for i, c in enumerate(_PLATE_DIGIT_SOURCES)}
_PLATE_TABLE.update({ord(c): None for c in _PLATE_DELETED})
_PLATE_SEPARATORS = re.compile(r"[\s\-_./]+", re.ASCII)


def normalize_plate(raw: object) -> str:
    """The search key of a plate: Unicode NFKC, Arabic-Indic and Persian digits to ASCII, presentation separators removed, upper case.
    The raw token stays in plate_raw and is the evidence; this is only for matching, and it never makes two different plates equal
    (O is not 0, I is not 1)."""
    text = unicodedata.normalize("NFKC", clean(raw))
    return _PLATE_SEPARATORS.sub("", text.translate(_PLATE_TABLE)).upper()


def plate_script(raw: object) -> str | None:
    """latin, arabic, mixed (both), other (letters of another script), None (no letters)."""
    scripts = set()
    for ch in unicodedata.normalize("NFKC", clean(raw)):
        if ch.isalpha():
            name = unicodedata.name(ch, "")
            scripts.add("latin" if name.startswith("LATIN") else "arabic" if name.startswith("ARABIC") else "other")
    if not scripts:
        return None
    if scripts == {"latin"} or scripts == {"arabic"}:
        return next(iter(scripts))
    if scripts == {"latin", "arabic"}:
        return "mixed"
    return "other"


def _luhn_ok(digits: str) -> bool:
    total = 0
    for i, ch in enumerate(reversed(digits)):
        d = int(ch)
        if i % 2 == 1:
            d = d * 2 - 9 if d * 2 > 9 else d * 2
        total += d
    return total % 10 == 0


def normalize_iccid(raw: object) -> tuple[str | None, list[str]]:
    """The SIM serial (ICCID): digits only, 18 to 20 of them (a trailing filler F and spaces or dashes are dropped). Returns (digits, flags). A check-digit failure or an odd length is
    flagged and kept (the raw value is the evidence); text that is not a number (a spreadsheet's 8.99E+18) is not an ICCID: (None, flag)."""
    text = clean(raw)
    if text.endswith(".0") and text[:-2].isdigit():
        text = text[:-2]
    if not text:
        return None, []
    text = re.sub(r"[\s\-]", "", text)
    if text[-1:] in ("F", "f"):
        text = text[:-1]
    if not text.isdigit():
        return None, ["iccid_not_numeric"]
    if not 18 <= len(text) <= 20:
        return text, ["iccid_length_%d" % len(text)]
    return text, ([] if _luhn_ok(text) else ["iccid_luhn_fail"])


def normalize_tac(raw: object) -> tuple[str | None, list[str]]:
    """A Type Allocation Code from a reference list: eight digits. A seven-digit code lost its leading zero in a spreadsheet: it is left-padded and flagged tac_padded. Anything else (fewer
    digits, not numeric, empty) is not a TAC: (None, flag). Returns (tac, flags)."""
    text = clean(raw)
    if text.endswith(".0") and text[:-2].isdigit():
        text = text[:-2]
    if not text:
        return None, ["tac_missing"]
    if not text.isdigit():
        return None, ["tac_not_numeric"]
    if len(text) == 8:
        return text, []
    if len(text) == 7:
        return "0" + text, ["tac_padded"]
    return None, ["tac_length_%d" % len(text)]
