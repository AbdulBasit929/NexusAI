"""Specification of a provider layout entry (schema nexusai.layout/v1): vocabularies and a validator.

A layout entry is DATA, never code: it names a provider's columns and says how to read them. It contains header names, maps and
rules, and no value from any real file. See docs/data/layout-registry.md.

The vocabularies below mirror CHECK constraints of migration 012 (forensic.layout_versions and the typed tables); the test
suite parses the SQL and fails if the two drift apart.
"""

from __future__ import annotations

import re
from typing import Any, Mapping

SCHEMA = "nexusai.layout/v1"

# forensic.layout_versions CHECK constraints (migration 012)
FAMILIES = ("communications", "internet", "app_usage", "subscriber", "cell_reference", "plate", "transaction", "access_log", "generic", "other")
DATASET_KINDS = ("target_cdr", "tower_dump", "ipdr", "app_usage", "subscriber", "cell_reference", "plate_sighting", "generic")
STATUSES = ("draft", "active", "retired")

# forensic.comm_event CHECK constraints
EVENT_TYPES = ("voice", "sms", "mms", "data", "ussd", "other")
DIRECTIONS = ("incoming", "outgoing", "none", "unknown")
NUMBER_KINDS = ("person_like", "short_code", "sender_id", "international", "service_label", "unknown")
TIME_BASES = ("declared", "assumed_by_profile", "confirmed_by_analyst", "explicit_offset")

CONTAINER_KINDS = ("xlsx", "spreadsheetml", "csv", "tsv")
HEADER_MODES = ("scan", "fixed")
DATE_ORDERS = ("DMY", "MDY", "YMD")

# Time formats a layout may declare, in the order they are tried.
TIME_FORMATS = (
    "excel_serial_1900",       # a number of days since 1899-12-30, as Excel stores it
    "iso_naive",              # YYYY-MM-DD HH:MM:SS with optional fraction, T or space; also what the xlsx reader emits for date cells
    "slash_24h_seconds",      # dd/mm/yyyy or mm/dd/yyyy (date_order) HH:MM:SS
    "slash_24h_minutes",      # same, HH:MM
    "slash_12h_seconds",      # m/d/yyyy h:mm:ss AM/PM (date_order governs day or month first)
    "dot_dmy_24h_seconds",    # dd.mm.yyyy HH:MM:SS (a dot always means day first)
    "slash_date",             # dd/mm/yyyy only (date_order)
)
SLASH_FORMATS = ("slash_24h_seconds", "slash_24h_minutes", "slash_12h_seconds", "slash_date")

DURATION_MODES = ("column_seconds", "minutes_seconds", "column_milliseconds", "from_times", "none")

# Canonical fields a layout can fill, per dataset family.
COMM_FIELDS = ("a", "b", "imsi", "imei", "lat", "lon", "location_text", "peer_operator", "volume_bytes")
SESSION_FIELDS = (
    "msisdn", "imsi", "imei", "ue_ip", "ue_port", "public_ip", "public_port", "public_port_from", "public_port_to",
    "server_ip", "server_port", "protocol", "apn", "rat", "cell", "gateway_ip", "serving_node_ip",
    "app_category", "app", "app_feature", "host", "url", "user_agent", "bytes_up", "bytes_down", "packets_up",
    "packets_down", "duration_ms", "session_ref",
)
USAGE_FIELDS = ("msisdn", "imsi", "imei", "terminal", "apn", "app_category", "app", "app_feature", "province", "city")
FIELDS_BY_FAMILY = {
    "target_cdr": COMM_FIELDS,
    "tower_dump": COMM_FIELDS,
    "ipdr": SESSION_FIELDS,
    "app_usage": USAGE_FIELDS,
}
TIME_FIELDS_BY_KIND = {
    "target_cdr": ("start", "end"),
    "tower_dump": ("start", "end"),
    "ipdr": ("start", "end", "nat_start", "nat_end"),
    "app_usage": ("start", "end"),
}
ENUM_TARGETS = {"direction": DIRECTIONS, "event_type": EVENT_TYPES}

LAYOUT_ID = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
TZ_NAME = re.compile(r"^[A-Za-z]+(?:/[A-Za-z0-9_+\-]+)+$|^UTC$")

# Keys that describe the layout's life, not its content: not part of the definition hash.
MUTABLE_KEYS = ("status", "confirmation")


def column_ref(spec: Any) -> tuple[str, int] | None:
    """Normalise a column reference: "Name" or {"name": "Name", "occurrence": 2} to (name, occurrence)."""
    if isinstance(spec, str):
        return spec, 1
    if isinstance(spec, Mapping) and isinstance(spec.get("name"), str):
        occurrence = spec.get("occurrence", 1)
        if isinstance(occurrence, int) and occurrence >= 1:
            return spec["name"], occurrence
    return None


def _known_columns(entry: Mapping[str, Any]) -> set[tuple[str, int]]:
    seen: dict[str, int] = {}
    known: set[tuple[str, int]] = set()
    for column in entry.get("header", {}).get("columns", []):
        name = column.get("name") if isinstance(column, Mapping) else None
        if not isinstance(name, str):
            continue
        seen[name] = seen.get(name, 0) + 1
        known.add((name, seen[name]))
    return known


def _check_column(problems: list[str], known: set[tuple[str, int]], where: str, spec: Any) -> None:
    ref = column_ref(spec)
    if ref is None:
        problems.append("%s: not a column reference" % where)
    elif ref not in known:
        problems.append("%s: column %r (occurrence %d) is not in header.columns" % (where, ref[0], ref[1]))


def validate_entry(entry: Mapping[str, Any]) -> list[str]:
    """Return every problem found in a layout entry (an empty list means the entry is valid)."""
    problems: list[str] = []
    if entry.get("schema") != SCHEMA:
        problems.append("schema must be %r" % SCHEMA)
    layout_id = entry.get("layout_id")
    if not isinstance(layout_id, str) or not LAYOUT_ID.match(layout_id):
        problems.append("layout_id must be lower-case words joined by hyphens")
    version = entry.get("layout_version")
    if not isinstance(version, int) or isinstance(version, bool) or version < 1:
        problems.append("layout_version must be a positive integer")
    if entry.get("status") not in STATUSES:
        problems.append("status must be one of %s" % ", ".join(STATUSES))
    if entry.get("family") not in FAMILIES:
        problems.append("family must be one of %s" % ", ".join(FAMILIES))
    kind = entry.get("dataset_kind")
    if kind not in DATASET_KINDS:
        problems.append("dataset_kind must be one of %s" % ", ".join(DATASET_KINDS))
    if not isinstance(entry.get("label"), str) or not entry.get("label"):
        problems.append("label is required")
    operator = entry.get("operator_label")
    if operator is not None and not isinstance(operator, str):
        problems.append("operator_label must be a string or null (a label from the file name, never inferred from a number prefix)")

    container = entry.get("container")
    if not isinstance(container, Mapping):
        problems.append("container is required")
        container = {}
    kinds = container.get("kinds")
    if not isinstance(kinds, list) or not kinds or any(k not in CONTAINER_KINDS for k in kinds):
        problems.append("container.kinds must be a non-empty list drawn from %s" % ", ".join(CONTAINER_KINDS))
    header_row = container.get("header_row", {})
    if not isinstance(header_row, Mapping) or header_row.get("mode") not in HEADER_MODES:
        problems.append("container.header_row.mode must be one of %s" % ", ".join(HEADER_MODES))
    elif header_row.get("mode") == "fixed" and not isinstance(header_row.get("index"), int):
        problems.append("container.header_row.index is required when mode is fixed")

    header = entry.get("header")
    if not isinstance(header, Mapping) or not isinstance(header.get("columns"), list) or not header.get("columns"):
        problems.append("header.columns must be a non-empty list")
        return problems
    names_seen: dict[str, int] = {}
    for index, column in enumerate(header["columns"]):
        if not isinstance(column, Mapping) or not isinstance(column.get("name"), str) or not column["name"].strip():
            problems.append("header.columns[%d] needs a name" % index)
            continue
        names_seen[column["name"]] = names_seen.get(column["name"], 0) + 1
        declared = column.get("occurrence")
        if declared is not None and declared != names_seen[column["name"]]:
            problems.append("header.columns[%d] occurrence %r does not match its position among equal names" % (index, declared))
    known = _known_columns(entry)
    for name in header.get("required", []):
        _check_column(problems, known, "header.required", name)
    for name, alternatives in (header.get("aliases") or {}).items():
        _check_column(problems, known, "header.aliases", name)
        if not isinstance(alternatives, list) or not all(isinstance(a, str) for a in alternatives):
            problems.append("header.aliases[%r] must be a list of names" % name)
    min_score = header.get("min_score", 0.85)
    if not isinstance(min_score, (int, float)) or not 0 < min_score <= 1:
        problems.append("header.min_score must be in (0, 1]")

    allowed_fields = FIELDS_BY_FAMILY.get(kind, ())
    for field, spec in (entry.get("fields") or {}).items():
        if field not in allowed_fields:
            problems.append("fields.%s is not a field of dataset kind %s" % (field, kind))
            continue
        _check_field(problems, known, "fields.%s" % field, spec)

    time_spec = entry.get("time") or {}
    allowed_time = TIME_FIELDS_BY_KIND.get(kind, ())
    for name, spec in (time_spec.get("columns") or {}).items():
        if name not in allowed_time:
            problems.append("time.columns.%s is not a time field of dataset kind %s" % (name, kind))
            continue
        _check_column(problems, known, "time.columns.%s" % name, spec)
    formats = time_spec.get("formats", [])
    if time_spec.get("columns") and (not isinstance(formats, list) or not formats or any(f not in TIME_FORMATS for f in formats)):
        problems.append("time.formats must be a non-empty list drawn from %s" % ", ".join(TIME_FORMATS))
    if time_spec.get("date_order") not in (None,) + DATE_ORDERS:
        problems.append("time.date_order must be null or one of %s" % ", ".join(DATE_ORDERS))
    if any(f in SLASH_FORMATS for f in formats) and "date_order" not in time_spec:
        problems.append("time.date_order must be stated (null when unconfirmed) for slash formats")
    zone = time_spec.get("source_tz")
    if zone is not None and (not isinstance(zone, str) or not TZ_NAME.match(zone)):
        problems.append("time.source_tz must be null or an IANA zone name")
    if time_spec.get("tz_basis") not in (None, "unknown") + TIME_BASES:
        problems.append("time.tz_basis must be unknown or one of %s" % ", ".join(TIME_BASES))

    duration = entry.get("duration") or {"mode": "none"}
    if duration.get("mode") not in DURATION_MODES:
        problems.append("duration.mode must be one of %s" % ", ".join(DURATION_MODES))
    elif duration["mode"] in ("column_seconds", "column_milliseconds"):
        _check_column(problems, known, "duration.column", duration.get("column"))
    elif duration["mode"] == "minutes_seconds":
        _check_column(problems, known, "duration.minutes", duration.get("minutes"))
        _check_column(problems, known, "duration.seconds", duration.get("seconds"))
    elif duration["mode"] == "from_times" and not (time_spec.get("columns") or {}).get("end"):
        problems.append("duration.mode from_times needs time.columns.end")

    for index, rule in enumerate(entry.get("enums") or []):
        _check_enum(problems, known, "enums[%d]" % index, rule)

    cell = entry.get("cell")
    if cell is not None:
        if not isinstance(cell, Mapping):
            problems.append("cell must be an object")
        else:
            for key in ("lac_tac", "ci", "raw", "plmn_column", "site"):
                if cell.get(key) is not None:
                    _check_column(problems, known, "cell.%s" % key, cell[key])
            plmn = cell.get("plmn")
            if plmn is not None and not (isinstance(plmn, str) and re.fullmatch(r"\d{5,6}", plmn)):
                problems.append("cell.plmn must be null or five or six digits")

    for name in entry.get("noise_columns") or []:
        _check_column(problems, known, "noise_columns", name)
    if entry.get("protocol_numbers") not in (None, "iana"):
        problems.append("protocol_numbers must be absent or 'iana'")

    for index, rule in enumerate(entry.get("quarantine") or []):
        if not isinstance(rule, Mapping) or not isinstance(rule.get("reason"), str):
            problems.append("quarantine[%d] needs a reason" % index)
            continue
        for key in ("empty_all", "present_any"):
            for name in rule.get(key, []):
                _check_column(problems, known, "quarantine[%d].%s" % (index, key), name)

    confirmation = entry.get("confirmation")
    if not isinstance(confirmation, Mapping) or not isinstance(confirmation.get("headers"), str) or not isinstance(confirmation.get("unconfirmed"), list):
        problems.append("confirmation needs headers (how the header list was established) and an unconfirmed list")
    elif entry.get("status") == "active" and confirmation.get("unconfirmed"):
        problems.append("an active layout cannot have unconfirmed items; resolve them or keep the layout in draft")

    for index, column in enumerate(((entry.get("golden") or {}).get("columns") or {})):
        if column not in {c.get("name") for c in header["columns"] if isinstance(c, Mapping)}:
            problems.append("golden.columns names %r, which is not a header column" % column)
    return problems


def _check_field(problems: list[str], known: set[tuple[str, int]], where: str, spec: Any) -> None:
    if isinstance(spec, Mapping) and "by_direction" in spec:
        by_direction = spec["by_direction"]
        if not isinstance(by_direction, Mapping) or not by_direction:
            problems.append("%s.by_direction must map directions to columns" % where)
            return
        for direction, column in by_direction.items():
            if direction not in ("incoming", "outgoing"):
                problems.append("%s.by_direction key %r must be incoming or outgoing" % (where, direction))
            _check_column(problems, known, "%s.by_direction.%s" % (where, direction), column)
        fallback = spec.get("fallback")
        if fallback is not None:
            _check_column(problems, known, "%s.fallback" % where, fallback)
        return
    if isinstance(spec, Mapping) and "combined" in spec:
        _check_column(problems, known, "%s.combined" % where, spec["combined"])
        if spec.get("order") not in ("lon_lat", "lat_lon"):
            problems.append("%s.order must be lon_lat or lat_lon" % where)
        return
    if isinstance(spec, Mapping) and "column" in spec:
        _check_column(problems, known, where, spec["column"])
        return
    _check_column(problems, known, where, spec)


def _check_enum(problems: list[str], known: set[tuple[str, int]], where: str, rule: Any) -> None:
    if not isinstance(rule, Mapping):
        problems.append("%s must be an object" % where)
        return
    _check_column(problems, known, "%s.column" % where, rule.get("column"))
    mapping = rule.get("map")
    if not isinstance(mapping, Mapping) or not mapping:
        problems.append("%s.map must map source labels to canonical values" % where)
        return
    if not isinstance(rule.get("confirmed"), bool):
        problems.append("%s.confirmed must be true or false" % where)
    for label, target in mapping.items():
        if not isinstance(target, Mapping) or not target:
            problems.append("%s.map[%r] must be an object such as {\"direction\": \"incoming\"}" % (where, label))
            continue
        for key, value in target.items():
            if key not in ENUM_TARGETS:
                problems.append("%s.map[%r] sets %r, which is not an enumerated field" % (where, label, key))
            elif value not in ENUM_TARGETS[key]:
                problems.append("%s.map[%r].%s = %r is not one of %s" % (where, label, key, value, ", ".join(ENUM_TARGETS[key])))
