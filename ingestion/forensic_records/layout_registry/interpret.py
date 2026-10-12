"""Reference interpreter: apply a layout entry to source records and produce canonical typed fields.

This is the executable meaning of the rules in an entry, written to be read and tested; Ingest v2 builds on it. It never
reads a file (a container reader supplies header cells and rows), never touches a database, and never guesses:
an unmapped label, an unconfirmed code, an ambiguous date, an unknown zone or a malformed number is flagged and kept.
"""

from __future__ import annotations

import ipaddress
from dataclasses import dataclass, field
from typing import Any, Iterable, Iterator, Mapping, Sequence

from .matching import DUPLICATE_SUFFIX, normalize_header
from .parties import (
    classify_party, clean, device_identifier, make_cell_key, parse_coordinate, split_combined_coordinates,
)
from .spec import column_ref
from .timeparse import parse_time, to_utc

DEFAULT_NULL_TOKENS = frozenset({"?", "-", "--", "N/A", "NA", "NULL", "NONE"})
IANA_PROTOCOLS = {1: "icmp", 6: "tcp", 17: "udp"}


@dataclass
class Policy:
    """What the job (not the file) settles: the source zone and how it is known, and the day-or-month order."""
    source_tz: str | None = None
    tz_basis: str | None = None        # assumed_by_profile | confirmed_by_analyst | declared
    date_order: str | None = None      # DMY | MDY | YMD


@dataclass
class RowResult:
    status: str                         # accepted | quarantined | skipped
    reason: str | None
    record: dict[str, Any] | None
    flags: list[str] = field(default_factory=list)
    row_number: int | None = None


def rows_to_records(header_cells: Sequence[Any], rows: Iterable[Sequence[Any]]) -> Iterator[dict[str, str]]:
    """Positional rows to records keyed as the xlsx reader keys them: a repeated header becomes `name__duplicate_N`."""
    keys: list[str | None] = []
    counts: dict[str, int] = {}
    for raw in header_cells:
        text = ("" if raw is None else str(raw)).strip()
        if not text:
            keys.append(None)
            continue
        counts[text] = counts.get(text, 0) + 1
        keys.append(text if counts[text] == 1 else "%s__duplicate_%d" % (text, counts[text]))
    for row in rows:
        yield {key: ("" if i >= len(row) or row[i] is None else str(row[i])) for i, key in enumerate(keys) if key}


class RecordView:
    """Look up a source value by the entry's column reference (name, occurrence, declared aliases)."""

    def __init__(self, entry: Mapping[str, Any], record: Mapping[str, Any]):
        self.values: dict[tuple[str, int], str] = {}
        for key, value in record.items():
            if not isinstance(key, str) or key.startswith("_xlsx_") or "__xlsx_" in key:
                continue
            suffix = DUPLICATE_SUFFIX.search(key)
            base = key[: suffix.start()] if suffix else key
            self.values.setdefault((normalize_header(base), int(suffix.group(1)) if suffix else 1), clean(value))
        self.aliases = {column_ref(name): alts for name, alts in (entry["header"].get("aliases") or {}).items()}

    def get(self, spec: Any) -> str:
        ref = column_ref(spec)
        if ref is None:
            return ""
        for candidate in [ref[0]] + list(self.aliases.get(ref, [])):
            value = self.values.get((normalize_header(candidate), ref[1]), "")
            if value:
                return value
        return ""

    def blank(self) -> bool:
        return not any(self.values.values())


def _null_tokens(entry: Mapping[str, Any], name: str) -> set[str]:
    return {t.upper() for t in DEFAULT_NULL_TOKENS} | {t.upper() for t in (entry.get("null_tokens") or {}).get(name, [])}


def _text(entry: Mapping[str, Any], view: RecordView, name: str, spec: Any) -> str | None:
    if spec is None:
        return None
    if isinstance(spec, Mapping) and "column" in spec:
        spec = spec["column"]
    value = view.get(spec)
    return None if not value or value.upper() in _null_tokens(entry, name) else value


def _apply_enums(entry: Mapping[str, Any], view: RecordView, flags: list[str]) -> dict[str, str]:
    """Several rules may read the same column (confirmed labels in one, unconfirmed in another); a label is unmapped only if
    no rule of that column knows it. Numeric codes arrive as '2' or '2.0' depending on the reader."""
    out: dict[str, str] = {}
    by_column: dict[tuple[str, int], list[Mapping[str, Any]]] = {}
    for rule in entry.get("enums") or []:
        by_column.setdefault(column_ref(rule["column"]), []).append(rule)
    for ref, rules in by_column.items():
        label = view.get(rules[0]["column"])
        if not label:
            continue
        if label.endswith(".0") and label[:-2].isdigit():
            label = label[:-2]
        for rule in rules:
            mapping = rule["map"]
            mapped = mapping.get(label)
            if mapped is None:
                mapped = {k.casefold(): v for k, v in mapping.items()}.get(label.casefold())
            if mapped is not None:
                out.update(mapped)
                if not rule["confirmed"]:
                    flags.append("enum_unconfirmed:" + "/".join(sorted(mapped)))
                break
        else:
            targets = sorted({key for rule in rules for value in rule["map"].values() for key in value})
            flags.append("unmapped_label:" + "/".join(targets))
    return out


def _quarantine_reason(entry: Mapping[str, Any], view: RecordView) -> str | None:
    for rule in entry.get("quarantine") or []:
        empties = rule.get("empty_all", [])
        present = rule.get("present_any", [])
        if all(not view.get(c) for c in empties) and (not present or any(view.get(c) for c in present)):
            return rule["reason"]
    return None


class _Times:
    """Parse the entry's time columns once per row and carry the zone decision."""

    def __init__(self, entry: Mapping[str, Any], view: RecordView, policy: Policy):
        spec = entry.get("time") or {}
        self.formats = spec.get("formats", [])
        self.order = spec.get("date_order") or policy.date_order
        self.zone = spec.get("source_tz") or policy.source_tz
        self.basis = (spec.get("tz_basis") if spec.get("source_tz") else None) or (policy.tz_basis if policy.source_tz else None)
        if self.zone and not self.basis:
            self.basis = "assumed_by_profile"
        self.results: dict[str, Any] = {}
        self.issue: str | None = None
        for name, column in (spec.get("columns") or {}).items():
            raw = view.get(column)
            if not raw:
                continue
            result = parse_time(raw, self.formats, self.order)
            self.results[name] = result
            if result.value is None and name == "start":
                self.issue = result.issue

    def local(self, name: str) -> str | None:
        result = self.results.get(name)
        return None if result is None or result.value is None else result.value.isoformat(sep=" ")

    def utc(self, name: str) -> str | None:
        result = self.results.get(name)
        if result is None or result.value is None or not (self.zone or result.offset):
            return None
        return to_utc(result.value, self.zone or "UTC", result.offset).isoformat()

    def basis_for_row(self) -> str | None:
        start = self.results.get("start")
        if start is not None and start.offset:
            return "explicit_offset"
        return self.basis


def _duration(entry: Mapping[str, Any], view: RecordView, times: _Times, flags: list[str]) -> int | None:
    spec = entry.get("duration") or {"mode": "none"}
    mode = spec["mode"]
    value: float | None = None
    if mode == "column_seconds":
        value = _number(view.get(spec["column"]), flags, "duration")
    elif mode == "column_milliseconds":
        raw = _number(view.get(spec["column"]), flags, "duration")
        value = None if raw is None else raw / 1000.0
    elif mode == "minutes_seconds":
        minutes, seconds = _number(view.get(spec["minutes"]), flags, "duration"), _number(view.get(spec["seconds"]), flags, "duration")
        value = None if minutes is None and seconds is None else (minutes or 0) * 60 + (seconds or 0)
    elif mode == "from_times":
        start, end = times.results.get("start"), times.results.get("end")
        if start and end and start.value and end.value:
            value = (end.value - start.value).total_seconds()
    if value is None:
        return None
    if value < 0:
        flags.append("negative_duration")
        return None
    return int(round(value))


def _number(text: str, flags: list[str], tag: str) -> float | None:
    if not text:
        return None
    try:
        return float(text)
    except ValueError:
        flags.append("%s_not_numeric" % tag)
        return None


def _integer(text: str | None, flags: list[str], tag: str) -> int | None:
    if not text:
        return None
    try:
        value = int(float(text))
    except ValueError:
        flags.append("%s_not_numeric" % tag)
        return None
    if value < 0:
        flags.append("%s_negative" % tag)
        return None
    return value


def _ip(text: str | None, flags: list[str], tag: str) -> str | None:
    if not text:
        return None
    try:
        return str(ipaddress.ip_address(text))
    except ValueError:
        flags.append("ip_invalid:%s" % tag)
        return None


def _port(text: str | None, flags: list[str], tag: str) -> int | None:
    value = _integer(text, flags, tag)
    if value is not None and value > 65535:
        flags.append("%s_out_of_range" % tag)
        return None
    return value


def _cell(entry: Mapping[str, Any], view: RecordView, flags: list[str]) -> dict[str, Any]:
    spec = entry.get("cell")
    out = {"cell_key": None, "lac_tac": None, "cell_raw": None}
    if not spec:
        return out
    lac = view.get(spec["lac_tac"]) if spec.get("lac_tac") else ""
    ci = view.get(spec["ci"]) if spec.get("ci") else ""
    raw = view.get(spec["raw"]) if spec.get("raw") else ""
    out["lac_tac"] = lac or None
    out["cell_raw"] = raw or ci or None
    if ci:
        out["cell_key"] = make_cell_key(spec.get("plmn"), lac, ci)
        if spec.get("plmn") is None or not lac:
            flags.append("cell_identity_partial")
    elif spec.get("ci"):
        flags.append("cell_missing")
    return out


def _coordinates(entry: Mapping[str, Any], view: RecordView, flags: list[str]) -> tuple[float | None, float | None]:
    fields = entry.get("fields") or {}
    lat_spec, lon_spec = fields.get("lat"), fields.get("lon")
    if isinstance(lat_spec, Mapping) and "combined" in lat_spec:
        lat, lon, extra = split_combined_coordinates(view.get(lat_spec["combined"]), lat_spec["order"])
        flags.extend(extra)
        return lat, lon
    lat, lon = None, None
    if lat_spec is not None:
        lat, extra = parse_coordinate(view.get(lat_spec["column"] if isinstance(lat_spec, Mapping) else lat_spec), "lat")
        flags.extend(extra)
    if lon_spec is not None:
        lon, extra = parse_coordinate(view.get(lon_spec["column"] if isinstance(lon_spec, Mapping) else lon_spec), "lon")
        flags.extend(extra)
    if (lat is None) != (lon is None):
        flags.append("coordinate_incomplete")
        return None, None
    if lat == 0.0 and lon == 0.0:
        flags.append("coordinate_zero")
        return None, None
    return lat, lon


def _raw_payload(record: Mapping[str, Any]) -> dict[str, str]:
    return {k: clean(v) for k, v in record.items() if isinstance(k, str) and not k.startswith("_xlsx_") and "__xlsx_" not in k}


def interpret_record(entry: Mapping[str, Any], record: Mapping[str, Any], *, row_number: int | None = None,
                     policy: Policy | None = None) -> RowResult:
    policy = policy or Policy()
    view = RecordView(entry, record)
    if view.blank():
        return RowResult("skipped", "blank_row", None, [], row_number)
    reason = _quarantine_reason(entry, view)
    if reason:
        return RowResult("quarantined", reason, None, [], row_number)
    flags: list[str] = []
    if any(view.get(c) for c in entry.get("noise_columns") or []):
        flags.append("noise_columns_populated")
    times = _Times(entry, view, policy)
    if "start" in (entry.get("time") or {}).get("columns", {}) and "start" not in times.results:
        return RowResult("quarantined", "time_missing", None, [], row_number)
    if times.issue:
        return RowResult("quarantined", times.issue if times.issue in ("ambiguous_date_order", "date_order_conflict") else "time_unparsed", None, [], row_number)
    kind = entry["dataset_kind"]
    if kind in ("target_cdr", "tower_dump"):
        built = _comm_event(entry, view, times, flags)
    elif kind == "ipdr":
        built = _data_session(entry, view, times, flags)
    else:
        built = _app_usage(entry, view, times, flags)
    if not times.zone and not any(r.offset for r in times.results.values()):
        flags.append("source_timezone_unknown")
    start, end = times.results.get("start"), times.results.get("end")
    if start and end and start.value and end.value and end.value < start.value:
        flags.append("end_before_start")
    record_out: dict[str, Any] = {
        "layout_id": entry["layout_id"], "layout_version": entry["layout_version"], "dataset_kind": kind, "source_row": row_number,
        "source_tz": times.zone, "time_basis": times.basis_for_row(),
    }
    record_out.update(built)
    record_out["quality_flags"] = sorted(set(flags))
    record_out["raw_payload"] = _raw_payload(record)
    return RowResult("accepted", None, record_out, record_out["quality_flags"], row_number)


def _comm_event(entry: Mapping[str, Any], view: RecordView, times: _Times, flags: list[str]) -> dict[str, Any]:
    fields = entry.get("fields") or {}
    enumerated = _apply_enums(entry, view, flags)
    direction = enumerated.get("direction", "unknown")
    event_type = enumerated.get("event_type")
    if event_type is None:
        event_type = "other"
        flags.append("event_type_not_in_layout")
    a_raw = _text(entry, view, "a", fields.get("a"))
    b_spec = fields.get("b")
    b_raw: str | None
    if isinstance(b_spec, Mapping) and "by_direction" in b_spec:
        column = b_spec["by_direction"].get(direction) or b_spec.get("fallback")
        if column is None:
            flags.append("counterparty_direction_unresolved")
        b_raw = _text(entry, view, "b", column)
    else:
        b_raw = _text(entry, view, "b", b_spec)
    a_kind, a_norm, a_flags = classify_party(a_raw)
    b_kind, b_norm, b_flags = classify_party(b_raw)
    flags.extend(["a_" + f for f in a_flags if f != "party_missing"])
    flags.extend(["b_" + f for f in b_flags if f != "party_missing"])
    if a_raw is None:
        flags.append("subject_missing")
    if isinstance(b_spec, Mapping) and b_spec.get("drop_if_equal_to_a") and b_norm is not None and b_norm == a_norm:
        b_raw, b_kind, b_norm = None, None, None
        flags.append("counterparty_equals_subject")
    imsi, _, imsi_flags = device_identifier(_text(entry, view, "imsi", fields.get("imsi")), "imsi")
    imei, tac, imei_flags = device_identifier(_text(entry, view, "imei", fields.get("imei")), "imei")
    flags.extend(imsi_flags + imei_flags)
    cell = _cell(entry, view, flags)
    lat, lon = _coordinates(entry, view, flags)
    loc_method = "source_supplied" if lat is not None else "none"
    peer = _peer_operator(entry, view, fields.get("peer_operator"))
    volume = _number(view.get(fields["volume_bytes"]), flags, "volume") if fields.get("volume_bytes") else None
    return {
        "event_type": event_type, "direction": direction,
        "a_raw": a_raw, "a_norm": a_norm, "a_kind": a_kind, "b_raw": b_raw, "b_norm": b_norm, "b_kind": b_kind,
        "start_local": times.local("start"), "start_utc": times.utc("start"), "end_local": times.local("end"), "end_utc": times.utc("end"),
        "duration_s": _duration(entry, view, times, flags),
        "imsi": imsi, "imei": imei, "tac": tac, **cell,
        "lat": lat, "lon": lon, "loc_method": loc_method, "location_text": _text(entry, view, "location_text", fields.get("location_text")),
        "peer_operator": peer, "volume_bytes": volume,
    }


def _peer_operator(entry: Mapping[str, Any], view: RecordView, spec: Any) -> str | None:
    if spec is None:
        return None
    value = view.get(spec["column"] if isinstance(spec, Mapping) else spec)
    if not value:
        return None
    operators = {o.casefold() for o in (spec.get("operators") or [])} if isinstance(spec, Mapping) else set()
    return value.casefold() if not operators or value.casefold() in operators else None


def _data_session(entry: Mapping[str, Any], view: RecordView, times: _Times, flags: list[str]) -> dict[str, Any]:
    fields = entry.get("fields") or {}

    def text(name: str) -> str | None:
        return _text(entry, view, name, fields.get(name))

    msisdn_kind, msisdn_norm, msisdn_flags = classify_party(text("msisdn"))
    flags.extend("msisdn_" + f for f in msisdn_flags if f != "party_missing")
    imsi, _, imsi_flags = device_identifier(text("imsi"), "imsi")
    imei, tac, imei_flags = device_identifier(text("imei"), "imei")
    flags.extend(imsi_flags + imei_flags)
    public_port = text("public_port")
    port_from = _port(text("public_port_from") or public_port, flags, "public_port")
    port_to = _port(text("public_port_to") or public_port, flags, "public_port")
    if port_from is not None and port_to is not None and port_to < port_from:
        flags.append("public_port_range_reversed")
        port_from = port_to = None
    protocol_text = text("protocol")
    protocol_num, protocol = None, None
    if protocol_text:
        if protocol_text.isdigit() and entry.get("protocol_numbers") == "iana":
            protocol_num = int(protocol_text)
            protocol = IANA_PROTOCOLS.get(protocol_num)
            if protocol is None:
                flags.append("protocol_number_unknown")
        else:
            protocol = protocol_text.casefold()
    cell = _cell(entry, view, flags)
    if not cell["cell_raw"]:
        cell_spec = fields.get("cell")
        if isinstance(cell_spec, Mapping) and cell_spec.get("column"):
            cell["cell_raw"] = view.get(cell_spec["column"]) or None
    return {
        "start_local": times.local("start"), "start_utc": times.utc("start"), "end_local": times.local("end"), "end_utc": times.utc("end"),
        "nat_start_local": times.local("nat_start"), "nat_start_utc": times.utc("nat_start"),
        "nat_end_local": times.local("nat_end"), "nat_end_utc": times.utc("nat_end"),
        "msisdn": msisdn_norm or text("msisdn"), "msisdn_kind": msisdn_kind, "imsi": imsi, "imei": imei, "tac": tac,
        "ue_ip": _ip(text("ue_ip"), flags, "ue_ip"), "ue_port": _port(text("ue_port"), flags, "ue_port"),
        "public_ip": _ip(text("public_ip"), flags, "public_ip"), "public_port_from": port_from, "public_port_to": port_to,
        "server_ip": _ip(text("server_ip"), flags, "server_ip"), "server_port": _port(text("server_port"), flags, "server_port"),
        "protocol": protocol, "protocol_num": protocol_num, "apn": text("apn"), "rat": text("rat"),
        **cell,
        "gateway_ip": _ip(text("gateway_ip"), flags, "gateway_ip"), "serving_node_ip": _ip(text("serving_node_ip"), flags, "serving_node_ip"),
        "app_category": text("app_category"), "app": text("app"), "app_feature": text("app_feature"), "host": text("host"),
        "url": text("url"), "user_agent": text("user_agent"),
        "bytes_up": _integer(text("bytes_up"), flags, "bytes_up"), "bytes_down": _integer(text("bytes_down"), flags, "bytes_down"),
        "packets_up": _integer(text("packets_up"), flags, "packets_up"), "packets_down": _integer(text("packets_down"), flags, "packets_down"),
        "duration_ms": _integer(text("duration_ms"), flags, "duration_ms"), "session_ref": text("session_ref"),
    }


def _app_usage(entry: Mapping[str, Any], view: RecordView, times: _Times, flags: list[str]) -> dict[str, Any]:
    fields = entry.get("fields") or {}

    def text(name: str) -> str | None:
        return _text(entry, view, name, fields.get(name))

    msisdn_kind, msisdn_norm, msisdn_flags = classify_party(text("msisdn"))
    flags.extend("msisdn_" + f for f in msisdn_flags if f != "party_missing")
    imsi, _, imsi_flags = device_identifier(text("imsi"), "imsi")
    imei, tac, imei_flags = device_identifier(text("imei"), "imei")
    flags.extend(imsi_flags + imei_flags)
    return {
        "start_local": times.local("start"), "start_utc": times.utc("start"), "end_local": times.local("end"), "end_utc": times.utc("end"),
        "msisdn": msisdn_norm or text("msisdn"), "msisdn_kind": msisdn_kind, "imsi": imsi, "imei": imei, "tac": tac,
        "terminal": text("terminal"), "apn": text("apn"), "app_category": text("app_category"), "app": text("app"),
        "app_feature": text("app_feature"), "province": text("province"), "city": text("city"),
    }


def interpret_rows(entry: Mapping[str, Any], header_cells: Sequence[Any], rows: Iterable[Sequence[Any]], *,
                   policy: Policy | None = None, first_row_number: int = 1) -> Iterator[RowResult]:
    for number, record in enumerate(rows_to_records(header_cells, rows), start=first_row_number):
        yield interpret_record(entry, record, row_number=number, policy=policy)


def summarize(results: Iterable[RowResult]) -> dict[str, Any]:
    """Aggregates a reconciliation can compare: counts by status and reason, flag counts, class and enumeration histograms,
    first and last local time, distinct party counts."""
    status: dict[str, int] = {}
    reasons: dict[str, int] = {}
    flag_counts: dict[str, int] = {}
    histograms: dict[str, dict[str, int]] = {}
    parties: dict[str, set[str]] = {"a_raw": set(), "b_raw": set(), "msisdn": set()}
    first = last = None
    for result in results:
        status[result.status] = status.get(result.status, 0) + 1
        if result.reason:
            reasons[result.reason] = reasons.get(result.reason, 0) + 1
        for flag in result.flags:
            flag_counts[flag] = flag_counts.get(flag, 0) + 1
        record = result.record
        if not record:
            continue
        for key in ("a_kind", "b_kind", "msisdn_kind", "direction", "event_type", "protocol", "app_category"):
            if record.get(key) is not None:
                histograms.setdefault(key, {})
                histograms[key][record[key]] = histograms[key].get(record[key], 0) + 1
        for key in parties:
            if record.get(key):
                parties[key].add(record[key])
        start = record.get("start_local")
        if start:
            first = start if first is None or start < first else first
            last = start if last is None or start > last else last
    return {
        "status": dict(sorted(status.items())), "quarantine_reasons": dict(sorted(reasons.items())),
        "flags": dict(sorted(flag_counts.items())), "histograms": {k: dict(sorted(v.items())) for k, v in sorted(histograms.items())},
        "first_start_local": first, "last_start_local": last,
        "distinct": {k: len(v) for k, v in parties.items() if v},
    }
