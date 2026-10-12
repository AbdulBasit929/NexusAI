"""Reading the time formats the provider layouts use, without guessing.

Rules that mirror the worker's time policy (forensics.time-policy/v1):
* a naive time is never given a time zone by this module: the zone comes from the layout (when the source or operator states
  it) or from the job's policy, and the basis is recorded with the row;
* a slash date whose two leading numbers are both 12 or less has no day-or-month order of its own: without a declared or
  confirmed order it is reported as ambiguous, never guessed;
* a dot date is always day first.
"""

from __future__ import annotations

import re
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
from decimal import ROUND_HALF_EVEN, Decimal, InvalidOperation
from typing import Iterable

try:  # the worker image carries tzdata; a bare laptop interpreter may not
    from zoneinfo import ZoneInfo, ZoneInfoNotFoundError
except ImportError:  # pragma: no cover
    ZoneInfo = None  # type: ignore[assignment]
    ZoneInfoNotFoundError = KeyError  # type: ignore[assignment,misc]

EXCEL_SERIAL = re.compile(r"^\d{4,6}(?:\.\d+)?$")
ISO_NAIVE = re.compile(r"^(\d{4})-(\d{2})-(\d{2})[ T](\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,6}))?(Z|[+-]\d{2}:?\d{2})?$")
SLASH_24H = re.compile(r"^(\d{1,2})/(\d{1,2})/(\d{4})[ T](\d{1,2}):(\d{2})(?::(\d{2}))?$")
SLASH_12H = re.compile(r"^(\d{1,2})/(\d{1,2})/(\d{4}) (\d{1,2}):(\d{2}):(\d{2}) ?([AaPp][Mm])$")
SLASH_DATE = re.compile(r"^(\d{1,2})/(\d{1,2})/(\d{4})$")
DOT_DMY = re.compile(r"^(\d{2})\.(\d{2})\.(\d{4}) (\d{1,2}):(\d{2}):(\d{2})$")


@dataclass(frozen=True)
class TimeResult:
    value: datetime | None            # naive wall-clock time as written (or rounded from an Excel serial)
    format: str | None                # which declared format read it
    issue: str | None = None          # unparsed | ambiguous_date_order | date_order_conflict
    offset: str | None = None         # an explicit offset written in the value (such as +05:00), if any
    order: str | None = None          # DMY or MDY for slash dates, and how it was settled
    order_source: str | None = None   # declared | structure_proven


def excel_serial_to_datetime(text: str) -> datetime | None:
    """1900 date system: days since 1899-12-30, rounded to the microsecond (the xlsx reader rounds the same way)."""
    try:
        serial = Decimal(text)
    except InvalidOperation:
        return None
    if serial < 1 or serial > Decimal(2958465):
        return None
    whole = int(serial)
    micros = int(((serial - whole) * Decimal(86_400_000_000)).to_integral_value(rounding=ROUND_HALF_EVEN))
    return datetime(1899, 12, 30) + timedelta(days=whole, microseconds=micros)


def _slash_order(first: int, second: int, declared: str | None) -> tuple[str | None, str | None]:
    """The day-or-month order of a slash date. Structure can prove it for one value (a part above 12 cannot be a month); a declared
    order covers the values where both orders are possible; a value whose structure contradicts the declared order is a conflict
    (the row is quarantined, never read in the other order)."""
    proven = "DMY" if first > 12 and second <= 12 else "MDY" if second > 12 and first <= 12 else None
    if declared in ("DMY", "MDY"):
        if proven and proven != declared:
            return "CONFLICT", None
        if proven:
            return proven, "structure_proven"
        if first <= 12 and second <= 12:
            return declared, "declared"
        return None, None
    if proven:
        return proven, "structure_proven"
    return None, None


def _from_parts(year: int, month: int, day: int, hour: int = 0, minute: int = 0, second: int = 0, micro: int = 0) -> datetime | None:
    try:
        return datetime(year, month, day, hour, minute, second, micro)
    except ValueError:
        return None


def parse_time(value: object, formats: Iterable[str], date_order: str | None = None) -> TimeResult:
    """Read one time cell with the first declared format that fits. Never guesses a day-or-month order."""
    text = ("" if value is None else str(value)).strip()
    if not text:
        return TimeResult(None, None, "unparsed")
    formats = list(formats)
    ambiguous = False
    conflict = False
    for fmt in formats:
        if fmt == "excel_serial_1900" and EXCEL_SERIAL.match(text):
            parsed = excel_serial_to_datetime(text)
            if parsed is not None:
                return TimeResult(parsed, fmt)
        elif fmt == "iso_naive":
            m = ISO_NAIVE.match(text)
            if m:
                y, mo, d, h, mi, s, frac, offset = m.groups()
                micro = int((frac or "0").ljust(6, "0")[:6])
                parsed = _from_parts(int(y), int(mo), int(d), int(h), int(mi), int(s), micro)
                if parsed is not None:
                    return TimeResult(parsed, fmt, offset=offset)
        elif fmt in ("slash_24h_seconds", "slash_24h_minutes"):
            m = SLASH_24H.match(text)
            if m and (fmt == "slash_24h_seconds") == (m.group(6) is not None):
                first, second, year = int(m.group(1)), int(m.group(2)), int(m.group(3))
                order, source = _slash_order(first, second, date_order)
                if order == "CONFLICT":
                    conflict = True
                    continue
                if order is None:
                    ambiguous = True
                    continue
                day, month = (first, second) if order == "DMY" else (second, first)
                parsed = _from_parts(year, month, day, int(m.group(4)), int(m.group(5)), int(m.group(6) or 0))
                if parsed is not None:
                    return TimeResult(parsed, fmt, order=order, order_source=source)
        elif fmt == "slash_12h_seconds":
            m = SLASH_12H.match(text)
            if m:
                first, second, year = int(m.group(1)), int(m.group(2)), int(m.group(3))
                order, source = _slash_order(first, second, date_order)
                if order == "CONFLICT":
                    conflict = True
                    continue
                if order is None:
                    ambiguous = True
                    continue
                day, month = (first, second) if order == "DMY" else (second, first)
                hour = int(m.group(4))
                if not 1 <= hour <= 12:
                    continue
                hour = hour % 12 + (12 if m.group(7).lower() == "pm" else 0)
                parsed = _from_parts(year, month, day, hour, int(m.group(5)), int(m.group(6)))
                if parsed is not None:
                    return TimeResult(parsed, fmt, order=order, order_source=source)
        elif fmt == "dot_dmy_24h_seconds":
            m = DOT_DMY.match(text)
            if m:
                parsed = _from_parts(int(m.group(3)), int(m.group(2)), int(m.group(1)), int(m.group(4)), int(m.group(5)), int(m.group(6)))
                if parsed is not None:
                    return TimeResult(parsed, fmt, order="DMY", order_source="structure_proven")
        elif fmt == "slash_date":
            m = SLASH_DATE.match(text)
            if m:
                first, second, year = int(m.group(1)), int(m.group(2)), int(m.group(3))
                order, source = _slash_order(first, second, date_order)
                if order == "CONFLICT":
                    conflict = True
                    continue
                if order is None:
                    ambiguous = True
                    continue
                day, month = (first, second) if order == "DMY" else (second, first)
                parsed = _from_parts(year, month, day)
                if parsed is not None:
                    return TimeResult(parsed, fmt, order=order, order_source=source)
    return TimeResult(None, None, "date_order_conflict" if conflict else "ambiguous_date_order" if ambiguous else "unparsed")


def to_utc(local: datetime, zone_name: str, offset: str | None = None) -> datetime:
    """Local wall-clock time to UTC. An explicit offset in the source wins; otherwise the named IANA zone is used.

    Pakistan has been UTC+05:00 without daylight saving since 2009; the fixed-offset fallback keeps a bare interpreter
    deterministic, and ZoneInfo (tzdata in the worker image) stays authoritative where it exists."""
    if offset:
        if offset == "Z":
            return local.replace(tzinfo=timezone.utc)
        sign = -1 if offset[0] == "-" else 1
        digits = offset[1:].replace(":", "")
        delta = timedelta(hours=int(digits[:2]), minutes=int(digits[2:4]))
        return (local - sign * delta).replace(tzinfo=timezone.utc)
    if zone_name in ("UTC", "Etc/UTC"):
        return local.replace(tzinfo=timezone.utc)
    zone = None
    if ZoneInfo is not None:
        try:
            zone = ZoneInfo(zone_name)
        except (ZoneInfoNotFoundError, ValueError):
            zone = None
    if zone is None:
        if zone_name == "Asia/Karachi":
            zone = timezone(timedelta(hours=5), name="Asia/Karachi")
        else:
            raise ValueError("unknown source time zone %r" % zone_name)
    return local.replace(tzinfo=zone).astimezone(timezone.utc)
