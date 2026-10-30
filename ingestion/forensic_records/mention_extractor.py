"""Rule-based extraction of identifier mentions from document text (reference cases programme, stage S3a).

Reads the passages that `document_pipeline.extract_document` produces and returns the strong identifiers and the known places they contain, each with the value in its normalised form,
the text as written, its position and a confidence tier. There is no model and no network: a mention is a pattern that was matched, so an analyst can be shown the rule and the text.

What a mention is, and is not:
* A mention is evidence that a document *says* an identifier, never that the person or the record it points to is involved. Linking a mention to a record is done by the normalised key
  in the typed layer; a name is never a key here (names stay dark until the masking design, and need a model).
* Strong kinds: `msisdn` (a Pakistani mobile number in national, international or bracketed form, Latin or Urdu digits), `national_id` (13 digits as 5-7-1, with a cue when run together),
  `account` (an IBAN, spaces allowed), `iccid`, `imei` (Luhn-valid 15 digits; a cue word makes it high), `imsi` (15 digits with a cue), `plate` (letters, an optional two-digit year,
  1 to 4 digits, accepted only beside a cue word and never for a legal or office abbreviation), `place` (a name from the gazetteer the caller gives: the areas the case's cell sites carry).
* Digits written in Arabic-Indic or Extended Arabic-Indic form are folded to Latin digits one for one, so positions refer to the text as written.
* A span is claimed by the most specific kind first (ICCID, account, national ID, IMEI and IMSI, mobile number, plate, place) and is not reused.
"""
from __future__ import annotations

import re
from dataclasses import dataclass
from typing import Mapping, Sequence

try:
    from .layout_registry.parties import _luhn_ok, normalize_iccid, normalize_plate
except ImportError:  # the worker's flat layout (/app): the package is a sibling of this module
    from layout_registry.parties import _luhn_ok, normalize_iccid, normalize_plate

EXTRACTOR_ID = "rule-mention-extractor"
EXTRACTOR_REVISION = "v2"

_ARABIC_INDIC = "٠١٢٣٤٥٦٧٨٩"
_EXTENDED = "۰۱۲۳۴۵۶۷۸۹"
_FOLD = {ord(c): str(i) for i, c in enumerate(_ARABIC_INDIC)}
_FOLD.update({ord(c): str(i) for i, c in enumerate(_EXTENDED)})
# letters of the Arabic script as Urdu uses it: the comma, the full stop and the digits that share the block are not letters
_URDU_LETTER = "ء-غف-يٮ-ۓەۺ-ۿ"

_SEP = r"[\s\-.‐-―]?"
_NOT_BEFORE = r"(?<![0-9A-Za-z\-+])"
_NOT_AFTER = r"(?![0-9A-Za-z])(?!-?\d)"

_ICCID = re.compile(r"(?<!\d)89\d{17,18}(?!\d)")
# v2: a serial printed in groups of four (8900 0000 0000 0100 02), with spaces or hyphens
_ICCID_GROUPED = re.compile(r"(?<!\d)89\d{2}(?:[ \-]\d{4}){3}[ \-]\d{2,4}(?!\d)")
# v2: a hyphen or a space between the groups of an IBAN
_IBAN = re.compile(r"(?<![A-Za-z0-9])PK[\s\-]?\d{2}[\s\-]?[A-Za-z]{4}(?:[\s\-]?\d{4}){4}(?![A-Za-z0-9])", re.I)
# v2: a non-breaking space between the groups of a national ID number
_GROUP_SEP = r"[ \t \-‐-―]"
_NID_DASHED = re.compile(r"(?<!\d)\d{5}" + _GROUP_SEP + r"\d{7}" + _GROUP_SEP + r"\d(?!\d)")
_NID_RUN = re.compile(r"(?<!\d)\d{13}(?!\d)")
_FIFTEEN = re.compile(r"(?<!\d)\d{15}(?!\d)")
# v2: an IMEI printed 2-6-6-1
_IMEI_GROUPED = re.compile(r"(?<!\d)\d{2}" + _GROUP_SEP + r"\d{6}" + _GROUP_SEP + r"\d{6}" + _GROUP_SEP + r"\d(?!\d)")
# v2: a mobile number in groups 4-7, 4-3-4 or 2-2-7 (national), with the operator code in brackets, or after +92 with or without brackets; every written form is read as its digits
_BODY = r"(?:\d{7}|\d{3}" + _SEP + r"\d{4})"
_MOBILE_NATIONAL = re.compile(_NOT_BEFORE + r"(?:\(0(3[0-4]\d)\)" + _SEP + _BODY + r"|0(3[0-4]\d)" + _SEP + _BODY + r"|03" + _SEP + r"[0-4]\d" + _SEP + r"\d{7})" + _NOT_AFTER)
_MOBILE_INTERNATIONAL = re.compile(r"(?<![0-9A-Za-z\-])(?:(?:\+|00)\s?92|(?<!\d)92)" + r"[\s\-.]?\(?0?\)?[\s\-.]?\(?(3[0-4]\d)\)?" + _SEP + _BODY + _NOT_AFTER)
_MOBILE_BARE = re.compile(_NOT_BEFORE + r"(3[0-4]\d)" + _SEP + _BODY + _NOT_AFTER)
_PLATE = re.compile(r"(?<![A-Za-z0-9])([A-Za-z]{2,4})[\s\-‐-―]?(?:(\d{2})[\s\-‐-―])?(\d{3,4})(?![A-Za-z0-9])")

_NOT_A_PLATE = frozenset({
    "FIR", "PPC", "CRPC", "CNIC", "NIC", "IMEI", "IMSI", "ICCID", "REF", "SMS", "MMS", "PKR", "RS", "ATM", "POS", "PTA", "NADRA", "IBAN", "SIM", "NTN", "FBR", "CPLC", "SEC", "SECT", "NO",
    "ID", "TEL", "PH", "FAX", "ROOM", "FLAT", "PLOT", "BLOCK", "STREET", "ST", "HOUSE", "HNO", "ACT", "ART", "CASE", "PAGE", "PG", "VOL", "DATE", "AMT", "INV", "BILL", "TXN", "UTR",
})

_CUE_BEFORE = {
    "imei": re.compile(r"(?i)(\bimei\s?[12]?(?![A-Za-z0-9])|\bhandset\b|\bdevice\b|آئی\s*ایم\s*ایم|آئی\s*ایم\s*ایم\s*آئی|ہینڈ\s*سیٹ)[^\d]{0,24}$"),
    "imsi": re.compile(r"(?i)\bimsi\b[^\d]{0,24}$"),
    "national_id": re.compile(r"(?i)(\bcnic\b|\bnic\b|\bidentity\s*card\b|\bid\s*card\b|\bnational\s*id\b|شناختی|کارڈ)[^\d]{0,30}$"),
    "mobile": re.compile(r"(?i)(\bmobile\b|\bphone\b|\bcell\b|\bcontact\b|\bnumber\b|\bno\.?|\bsim\b|\bwhatsapp\b|\bcalled?\b|\btel\b|\bnamb[ae]r\b|نمبر|موبائل|فون|کال|رابطہ)[^\d]{0,24}$"),
}
_PLATE_CUE = re.compile(
    r"(?i)(\bnumber\s*plate\b|\bplate\b|\bregistration\b|\bregistered\b|\breg\.?\s*no\b|\breg\b|\bveh\b|\bvehicle\b|\bcar\b|\bmotor\s*cycle\b|\bmotorcycle\b|\bmotor\s?bike\b|\bbike\b|\bjeep\b|\bvan\b|\btruck\b|\brickshaw\b|\bgaa?r[iy]\b|\bgaa?d[iy]\b|\bgaddi\b|"
    r"گاڑی|نمبر\s*پلیٹ|رجسٹریشن|موٹر\s*سائیکل|موٹرسائیکل|کار)")


@dataclass(frozen=True)
class Mention:
    kind: str
    value: str          # normalised: the key that links to records
    surface: str        # the text as written
    start: int          # offsets in the text given
    end: int
    tier: str           # "high" or "low"
    rule: str


def fold_digits(text: str) -> str:
    return text.translate(_FOLD)


def _luhn(digits: str) -> bool:
    return _luhn_ok(digits)


def _msisdn_digits(written: str) -> str | None:
    """The key of a mobile number as written in any of its forms: 92 and ten digits, the operator code between 300 and 349; None when the digits are not such a number."""
    digits = re.sub(r"\D", "", written)
    if digits.startswith("0092"):
        digits = digits[2:]
    if digits.startswith("92") and len(digits) == 13 and digits[2] == "0":
        digits = digits[:2] + digits[3:]
    elif digits.startswith("0") and len(digits) == 11:
        digits = "92" + digits[1:]
    elif len(digits) == 10 and digits.startswith("3"):
        digits = "92" + digits
    if len(digits) == 12 and digits.startswith("923") and "300" <= digits[2:5] <= "349":
        return digits
    return None


def _free(claimed: list[tuple[int, int]], start: int, end: int) -> bool:
    return all(end <= a or start >= b for a, b in claimed)


def _cue(kind: str, cue_text: str, position: int) -> bool:
    return _CUE_BEFORE[kind].search(cue_text[max(0, position - 40):position]) is not None


def _place_patterns(places: Mapping[str, Sequence[str]] | None):
    rows = []
    for key, spellings in (places or {}).items():
        for spelling in spellings:
            rows.append((key, spelling))
    rows.sort(key=lambda row: -len(row[1]))
    out = []
    for key, spelling in rows:
        escaped = re.escape(spelling).replace(r"\ ", r"\s+")
        if re.search(r"[A-Za-z]", spelling):
            out.append((key, re.compile(r"(?<![A-Za-z])" + escaped + r"(?![A-Za-z])", re.I)))
        else:
            out.append((key, re.compile(r"(?<![" + _URDU_LETTER + r"])" + escaped + r"(?![" + _URDU_LETTER + r"])")))
    return out


def extract_mentions(text: str, places: Mapping[str, Sequence[str]] | None = None, *, include_low: bool = False, context_before: str = "") -> list[Mention]:
    """The mentions in one passage. `places` maps a place key to the spellings it is written in (Latin and Urdu); `include_low` keeps the low-confidence tier; `context_before` is the text of the
    passage just before this one in the document (in a table, the neighbouring cell), which may hold the cue word of an identifier whose own cell holds only the digits."""
    folded = fold_digits(text)
    lead = fold_digits(context_before)[-60:] + "\n"
    cue_text, shift = lead + folded, len(lead)
    claimed: list[tuple[int, int]] = []
    found: list[Mention] = []

    def take(kind, value, start, end, tier, rule):
        if tier == "low" and not include_low:
            return
        if not _free(claimed, start, end):
            return
        claimed.append((start, end))
        found.append(Mention(kind, value, text[start:end], start, end, tier, rule))

    for pattern in (_ICCID, _ICCID_GROUPED):
        for m in pattern.finditer(folded):
            value, flags = normalize_iccid(m.group(0))
            if value:
                # a serial that fails its check digit is kept as evidence of what the document says, in the low tier
                take("iccid", value, m.start(), m.end(), "low" if flags else "high", "iccid-check-failed" if flags else "iccid-luhn")
    for m in _IBAN.finditer(folded):
        take("account", re.sub(r"[\s\-]+", "", m.group(0)).upper(), m.start(), m.end(), "high", "iban-form")
    for m in _NID_DASHED.finditer(folded):
        digits = re.sub(r"\D", "", m.group(0))
        take("national_id", "%s-%s-%s" % (digits[:5], digits[5:12], digits[12]), m.start(), m.end(), "high", "national-id-5-7-1")
    for m in _NID_RUN.finditer(folded):
        digits = m.group(0)
        if _cue("national_id", cue_text, m.start() + shift):
            take("national_id", "%s-%s-%s" % (digits[:5], digits[5:12], digits[12]), m.start(), m.end(), "high", "national-id-run-with-cue")
    for pattern in (_FIFTEEN, _IMEI_GROUPED):
        for m in pattern.finditer(folded):
            digits = re.sub(r"\D", "", m.group(0))
            if _cue("imsi", cue_text, m.start() + shift):
                take("imsi", digits, m.start(), m.end(), "high", "imsi-cue")
            elif _luhn(digits):
                cued = _cue("imei", cue_text, m.start() + shift)
                take("imei", digits, m.start(), m.end(), "high" if cued else "low", "imei-luhn-cue" if cued else "imei-luhn")
    for pattern, rule in ((_MOBILE_NATIONAL, "mobile-national"), (_MOBILE_INTERNATIONAL, "mobile-international")):
        for m in pattern.finditer(folded):
            value = _msisdn_digits(m.group(0))
            if value:
                take("msisdn", value, m.start(), m.end(), "high", rule)
    for m in _MOBILE_BARE.finditer(folded):
        if _cue("mobile", cue_text, m.start() + shift):
            value = _msisdn_digits(m.group(0))
            if value:
                take("msisdn", value, m.start(), m.end(), "high", "mobile-bare-with-cue")
    for m in _PLATE.finditer(folded):
        letters = m.group(1).upper()
        if letters in _NOT_A_PLATE:
            continue
        window = cue_text[max(0, m.start() + shift - 60):m.start() + shift] + " " + folded[m.end():m.end() + 24]
        if _PLATE_CUE.search(window):
            take("plate", normalize_plate(m.group(0)), m.start(), m.end(), "high", "plate-with-cue")
    for key, pattern in _place_patterns(places):
        for m in pattern.finditer(text):
            take("place", key, m.start(), m.end(), "high", "gazetteer")
    return sorted(found, key=lambda item: (item.start, item.kind))


def extract_from_passages(passages: Sequence[object], places: Mapping[str, Sequence[str]] | None = None, *, include_low: bool = False) -> list[dict]:
    """Mentions of the passages of one document (objects with `.text` and `.locator`), each with its locator, ready to store as a derived record."""
    out = []
    previous = ""
    for passage in passages:
        before, previous = previous, passage.text
        for mention in extract_mentions(passage.text, places, include_low=include_low, context_before=before):
            out.append({"kind": mention.kind, "value": mention.value, "surface": mention.surface, "start": mention.start, "end": mention.end, "tier": mention.tier,
                        "rule": mention.rule, "locator": dict(passage.locator), "passage_sha256": getattr(passage, "text_sha256", None)})
    return out
