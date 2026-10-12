"""Party numbers, device identifiers and cell keys: classification and normalisation.

Number classes (roadmap finding F4): a number that identifies a person's line is `person_like`; `short_code`, `sender_id`,
`international`, `service_label` and `unknown` never enter a person-to-person graph without a decision. The module never
infers an operator from a number prefix (the country profile forbids it).
"""

from __future__ import annotations

import re

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
