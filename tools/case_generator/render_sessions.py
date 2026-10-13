"""Rendering data sessions as internet-record exports (IPDR layouts I1 to I4) and application-usage exports (A1).

Like render.py this is the inverse of the registry's reference interpreter: header names, time formats, address and port columns come
from the layout entry. Text columns the canonical truth does not describe (APN, user agent, host) are filled from the entry's own
golden recipes, and the row's expectation is read back from what was written, as the goldens do.
"""
from __future__ import annotations

import datetime
import os
import random
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import world as w  # noqa: E402
import defects as defect_module  # noqa: E402
from render import RenderedFile, cell_identity, layout_hints  # noqa: E402
from ingestion.forensic_records.layout_registry.spec import column_ref  # noqa: E402

mg = w.mg
SECONDS_FIRST = ("iso_naive", "slash_24h_seconds", "dot_dmy_24h_seconds", "slash_12h_seconds", "excel_serial_1900")
PROTOCOL_NAMES = {6: "tcp", 17: "udp", 1: "icmp"}
NULL_WORDS = {"?", "-", "--", "N/A", "NA", "NULL", "NONE"}


def session_format(entry):
    """The most precise time format the entry accepts (never the minute-precision one)."""
    formats = entry["time"]["formats"]
    for candidate in SECONDS_FIRST:
        if candidate in formats:
            return candidate
    raise ValueError("%s has no second-precision time format" % entry["layout_id"])


def _preamble(entry, values):
    spec = (entry.get("golden") or {}).get("preamble")
    if not spec:
        return []
    rows = []
    for row in spec:
        cells = []
        for cell in row:
            for key, value in values.items():
                cell = cell.replace(key, value)
            cells.append(cell)
        rows.append(cells)
    return rows


def render_sessions(world, entry, items, label, preamble_values=None, damage=False) -> RenderedFile:
    """items: [(Session, Person)] in time order. Returns the export and, per row, the expectation the round trip must reproduce."""
    hints = layout_hints(entry)
    fields = entry.get("fields") or {}
    index = mg.column_index(entry)
    n_columns = len(entry["header"]["columns"])
    time_cols = (entry.get("time") or {}).get("columns", {})
    fmt = session_format(entry)
    recipes = (entry.get("golden") or {}).get("columns") or {}
    container, encoding, delimiter = entry["container"]["kinds"][0], entry["container"].get("encoding"), entry["container"].get("delimiter")
    numeric = {index[column_ref(ref)] for ref in time_cols.values()} if fmt == "excel_serial_1900" else set()
    out = RenderedFile(entry["layout_id"], label, [], 0, numeric, container, "TITLE" if container == "spreadsheetml" else "Sheet1", encoding, delimiter)
    data = []
    range_style = "public_port_from" in fields
    for session, person in items:
        r = random.Random("sess-render:%d:%s" % (world.params.seed, session.uid))
        cells = [""] * n_columns

        def put(ref, value):
            key = ref if isinstance(ref, tuple) else column_ref(ref)
            cells[index[key]] = "" if value is None else str(value)

        start, end = session.start, session.end
        put(time_cols["start"], mg.render_time(start, fmt, hints))
        if "end" in time_cols:
            put(time_cols["end"], mg.render_time(end, fmt, hints))
        if "nat_start" in time_cols:
            put(time_cols["nat_start"], mg.render_time(session.nat_start, fmt, hints))
        if "nat_end" in time_cols:
            put(time_cols["nat_end"], mg.render_time(session.nat_end, fmt, hints))
        put(fields["msisdn"], person.msisdn)
        imsi_now, imei_n = w.identity_at(person, start)
        put(fields["imsi"], imsi_now)
        imei = None
        if fields.get("imei"):
            imei = mg.imei_for(imei_n, int(hints.get("imei_digits", 15)))
            put(fields["imei"], imei)
        put(fields["ue_ip"], session.ue_ip)
        put(fields["ue_port"], session.ue_port)
        put(fields["public_ip"], session.public_ip)
        if range_style:
            put(fields["public_port_from"], session.port_from)
            put(fields["public_port_to"], session.port_to)
        else:
            put(fields["public_port"], session.port_from)
        put(fields["server_ip"], session.server_ip)
        put(fields["server_port"], session.server_port)
        protocol_expect = protocol_num_expect = None
        if "protocol" in fields and entry.get("protocol_numbers") == "iana":
            put(fields["protocol"], session.protocol_num)
            protocol_num_expect, protocol_expect = session.protocol_num, PROTOCOL_NAMES[session.protocol_num]
        category, app, feature = session.app
        for name, value in (("app_category", category), ("app", app), ("app_feature", feature)):
            if fields.get(name):
                put(fields[name], value)
        expect_ints = {}
        for name, value in (("bytes_up", session.bytes_up), ("bytes_down", session.bytes_down), ("packets_up", session.packets_up), ("packets_down", session.packets_down)):
            if fields.get(name):
                put(fields[name], value)
                expect_ints[name] = value
        if fields.get("duration_ms"):
            duration_ms = int((end - start).total_seconds() * 1000)
            put(fields["duration_ms"], duration_ms)
            expect_ints["duration_ms"] = duration_ms
        cell_expect = {"cell_key": None, "lac_tac": None, "cell_raw": None}
        cell_spec = entry.get("cell")
        if cell_spec:
            lac, ci, raw, key = cell_identity(entry, hints, person.operator, session.site)
            put(cell_spec["lac_tac"], lac)
            put(cell_spec["ci"], ci)
            cell_expect = {"cell_key": key, "lac_tac": lac, "cell_raw": ci}
        session_ref = None
        if fields.get("session_ref"):
            session_ref = "sid%06d" % int(session.uid[1:])
            put(fields["session_ref"], session_ref)
        for name in ("gateway_ip", "serving_node_ip"):
            if fields.get(name):
                put(fields[name], "10.255.%d.%d" % (r.randrange(10), r.randrange(1, 255)))
        filled = {pos for pos, cell in enumerate(cells) if cell != ""}
        for name, recipe in recipes.items():
            for (col, occ), pos in index.items():
                if col == name and pos not in filled:
                    cells[pos] = mg.recipe_value(recipe, r, len(data), {"start": start, "end": end})
        text_expect = {}
        for name in ("apn", "rat", "app_category", "app", "app_feature", "host", "url", "user_agent", "protocol", "gateway_ip", "serving_node_ip"):
            if name == "protocol" and protocol_expect is not None:
                continue
            spec = fields.get(name)
            if spec:
                value = cells[index[column_ref(spec)]]
                null_tokens = {t.upper() for t in (entry.get("null_tokens") or {}).get(name, [])} | NULL_WORDS
                text_expect[name] = None if not value or value.upper() in null_tokens else (value.casefold() if name == "protocol" else value)
        if isinstance(fields.get("cell"), dict) and fields["cell"].get("column"):
            cell_expect["cell_raw"] = cells[index[column_ref(fields["cell"]["column"])]] or None
        data.append(cells)
        out.rows.append({
            "status": "accepted", "flags": [], "uid": session.uid, "msisdn": person.msisdn, "imsi": imsi_now, "imei": imei, "ue_ip": session.ue_ip, "ue_port": session.ue_port,
            "public_ip": session.public_ip, "public_port_from": session.port_from, "public_port_to": session.port_to, "server_ip": session.server_ip,
            "server_port": session.server_port, "protocol": protocol_expect, "protocol_num": protocol_num_expect, "session_ref": session_ref,
            "start_local": start.isoformat(sep=" "), "end_local": end.isoformat(sep=" ") if "end" in time_cols else None,
            "nat_start_local": session.nat_start.isoformat(sep=" ") if "nat_start" in time_cols else None,
            "nat_end_local": session.nat_end.isoformat(sep=" ") if "nat_end" in time_cols else None,
            "plant": session.plant, **expect_ints, **cell_expect, **{k: v for k, v in text_expect.items() if k != "protocol" or protocol_expect is None}})
    if damage:
        data, out.rows, _ = defect_module.inject_sessions(entry, index, time_cols, data, out.rows, world.params.defects_per_class,
                                                          random.Random("defects:%d:%s" % (world.params.seed, label)))
        out.defects = defect_module.expectations(out.rows)
    header = [c["name"] for c in entry["header"]["columns"]]
    values = dict(preamble_values or {})
    stamps = [s.start for s, _ in items] or [datetime.datetime(2026, 1, 1)]
    values.setdefault("{ts_start}", min(stamps).strftime("%Y-%m-%d %H:%M:%S"))
    values.setdefault("{ts_end}", max(stamps).strftime("%Y-%m-%d %H:%M:%S"))
    values.setdefault("{public_ip}", "")
    preamble = _preamble(entry, values)
    out.table = preamble + [header] + data
    out.header_index = len(preamble)
    return out


def render_app_usage(world, entry, person, sessions) -> RenderedFile:
    """The application-usage export of one subscription: one row per session, with the session's application."""
    hints = layout_hints(entry)
    fields = entry["fields"]
    index = mg.column_index(entry)
    time_cols = entry["time"]["columns"]
    fmt = session_format(entry)
    recipes = (entry.get("golden") or {}).get("columns") or {}
    out = RenderedFile(entry["layout_id"], "usage-%s" % person.pid, [], 0, set(), entry["container"]["kinds"][0], "Sheet1", entry["container"].get("encoding"), entry["container"].get("delimiter"))
    data = []
    for session in sessions:
        r = random.Random("usage-render:%d:%s" % (world.params.seed, session.uid))
        cells = [""] * len(entry["header"]["columns"])

        def put(ref, value):
            cells[index[ref if isinstance(ref, tuple) else column_ref(ref)]] = "" if value is None else str(value)
        put(time_cols["start"], mg.render_time(session.start, fmt, hints))
        put(time_cols["end"], mg.render_time(session.end, fmt, hints))
        imsi_now, imei_n = w.identity_at(person, session.start)
        imei = mg.imei_for(imei_n, int(hints.get("imei_digits", 15)))
        put(fields["msisdn"], person.msisdn)
        put(fields["imsi"], imsi_now)
        put(fields["imei"], imei)
        category, app, feature = session.app
        put(fields["app_category"], category)
        put(fields["app"], app)
        put(fields["app_feature"], feature)
        filled = {pos for pos, cell in enumerate(cells) if cell != ""}
        for name, recipe in recipes.items():
            for (col, occ), pos in index.items():
                if col == name and pos not in filled:
                    cells[pos] = mg.recipe_value(recipe, r, len(data), {"start": session.start, "end": session.end})
        text = {name: (cells[index[column_ref(fields[name])]] or None) for name in ("terminal", "apn", "app_category", "app", "app_feature", "province", "city")}
        data.append(cells)
        out.rows.append({"uid": session.uid, "msisdn": person.msisdn, "imsi": imsi_now, "imei": imei, "tac": imei[:8], "start_local": session.start.isoformat(sep=" "),
                         "end_local": session.end.isoformat(sep=" "), **text})
    out.table = [[c["name"] for c in entry["header"]["columns"]]] + data
    out.header_index = 0
    return out
