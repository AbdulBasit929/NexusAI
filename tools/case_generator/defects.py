"""Injected defects in the call-record exports (specification section 6).

Real exports carry rows a loader must not silently swallow. This module damages a few background rows of a rendered export on purpose,
never a planted one, and writes down what each damaged row must become under the layout registry's documented rules:

  blank_row            an empty row                                  skipped (blank_row)
  time_unparsed        the start time is not a time                  quarantined (time_unparsed)
  time_missing         the start time is empty                       quarantined (time_missing)
  date_order_conflict  a slash date in the other order, where the    quarantined (date_order_conflict)
                       day or month cannot exist in the settled order
  unknown_label        a direction label the layout does not map     accepted, direction unknown, flag unmapped_label:direction
  party_sentinel       the counterparty is -1                        accepted, no counterparty, flag b_party_sentinel
  self_counterparty    the counterparty is the subscriber (C6)       accepted, no counterparty, flag counterparty_equals_subject
  duplicate_row        a row repeated verbatim                       accepted twice; merged into one by the loader

Internet-record exports get blank_row, time_unparsed, time_missing, duplicate_row and two of their own:
  invalid_server_ip    the server address is 999.1.1.1              accepted, server address none, flag ip_invalid:server_ip
  server_port_range    the server port is 70000                      accepted, server port none, flag server_port_out_of_range

The expectations come from these documented rules, not from running the interpreter; the audit checks the interpreter against them.
A row that cannot become a visible event is left out of `visible`, so the reference answers are computed from what a loader can use.
"""
from __future__ import annotations

import datetime
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import world as w  # noqa: E402
from ingestion.forensic_records.layout_registry.spec import column_ref  # noqa: E402

mg = w.mg
KINDS = ("time_unparsed", "time_missing", "date_order_conflict", "unknown_label", "party_sentinel", "self_counterparty", "duplicate_row", "blank_row")
SLASH = ("slash_24h_seconds", "slash_12h_seconds", "slash_24h_minutes", "slash_date")


def direction_column(entry):
    """The column that carries only a direction label (not an event type), if the layout has one."""
    for rule in entry.get("enums") or []:
        meanings = [m for m in rule["map"].values() if m]
        if meanings and all(set(m) == {"direction"} for m in meanings) and any(v["direction"] in ("incoming", "outgoing") for v in meanings):
            return column_ref(rule["column"])
    return None


def counterparty_cell(entry, index, row):
    fields = entry.get("fields") or {}
    spec = fields.get("b")
    if isinstance(spec, dict) and "by_direction" in spec:
        return index[column_ref(spec["by_direction"][row["true_direction"]])]
    return index[column_ref(spec["column"] if isinstance(spec, dict) else spec)]


def inject(entry, hints, index, time_columns, fmt, data, rows, count, rng):
    """Damage `count` background rows per kind. Returns (data, rows, defects); rows stay aligned with data."""
    if not count:
        return data, rows, []
    fields = entry.get("fields") or {}
    direction_ref = direction_column(entry)
    b_spec = fields.get("b")
    drops_self = isinstance(b_spec, dict) and bool(b_spec.get("drop_if_equal_to_a"))
    declared = (entry.get("time") or {}).get("date_order")
    start_pos = index[column_ref(time_columns["start"])]
    free = [i for i, r in enumerate(rows) if not r["plant"]]
    taken = set()
    chosen = {}

    def pick(kind, eligible):
        pool = sorted(set(eligible) - taken)
        picks = sorted(rng.sample(pool, min(count, len(pool)))) if pool else []
        taken.update(picks)
        chosen[kind] = picks
    for kind in KINDS:
        if kind == "date_order_conflict":
            pick(kind, [i for i in free if declared and fmt in SLASH and datetime.datetime.fromisoformat(rows[i]["start_local"]).day > 12])
        elif kind == "unknown_label":
            pick(kind, free if direction_ref else [])
        elif kind == "self_counterparty":
            pick(kind, free if drops_self else [])
        elif kind in ("duplicate_row", "blank_row"):
            pick(kind, free)
        else:
            pick(kind, free)
    defects = []
    inserts = []   # (after original index, kind, cells, row)
    for kind, picks in chosen.items():
        for i in picks:
            row, cells = rows[i], data[i]
            truth = {k: row[k] for k in ("uid", "start_local", "b_norm", "direction", "event_type")}
            if kind == "time_unparsed":
                cells[start_pos] = "not-a-time"
                row.update({"status": "quarantined", "reason": "time_unparsed"})
            elif kind == "time_missing":
                cells[start_pos] = ""
                row.update({"status": "quarantined", "reason": "time_missing"})
            elif kind == "date_order_conflict":
                other = dict(hints, date_order="DMY" if declared == "MDY" else "MDY")
                cells[start_pos] = mg.render_time(datetime.datetime.fromisoformat(row["start_local"]), fmt, other)
                row.update({"status": "quarantined", "reason": "date_order_conflict"})
            elif kind == "unknown_label":
                cells[index[direction_ref]] = "Sideways"
                row.update({"direction": "unknown", "flags": ["unmapped_label:direction"]})
                if isinstance(b_spec, dict) and "by_direction" in b_spec:
                    # the counterparty sits in the column that depends on the direction: with no direction it cannot be told
                    row.update({"b_norm": None, "b_kind": None, "b_raw_label": None, "flags": ["unmapped_label:direction", "counterparty_direction_unresolved"]})
            elif kind == "party_sentinel":
                cells[counterparty_cell(entry, index, row)] = "-1"
                row.update({"b_norm": None, "b_kind": None, "b_raw_label": None, "flags": ["b_party_sentinel"]})
            elif kind == "self_counterparty":
                cells[counterparty_cell(entry, index, row)] = cells[index[column_ref(fields["a"])]]
                row.update({"b_norm": None, "b_kind": None, "b_raw_label": None, "flags": ["counterparty_equals_subject"]})
            elif kind == "duplicate_row":
                copy = dict(row, duplicate_of=row["uid"], defect="duplicate_row")
                inserts.append((i, kind, list(cells), copy))
                defects.append({"kind": kind, "uid": row["uid"], "truth": truth})
                continue
            elif kind == "blank_row":
                blank = {"uid": None, "status": "skipped", "reason": "blank_row", "defect": "blank_row", "plant": None}
                inserts.append((i, kind, [""] * len(cells), blank))
                defects.append({"kind": kind, "uid": None, "truth": None})
                continue
            row["defect"] = kind
            defects.append({"kind": kind, "uid": row["uid"], "truth": truth})
    for after, kind, cells, row in sorted(inserts, key=lambda t: -t[0]):
        data.insert(after + 1, cells)
        rows.insert(after + 1, row)
    return data, rows, defects


def expectations(rows):
    """Per data row of the final export, what the registry must make of it: [(row number, status, reason, flags)]."""
    out = []
    for n, row in enumerate(rows):
        if row.get("defect"):
            out.append({"data_row": n, "kind": row["defect"], "uid": row.get("uid"), "status": row.get("status", "accepted"), "reason": row.get("reason"),
                        "flags": row.get("flags", []), "duplicate_of": row.get("duplicate_of")})
    return out


SESSION_KINDS = ("time_unparsed", "time_missing", "invalid_server_ip", "server_port_range", "duplicate_row", "blank_row")


def inject_sessions(entry, index, time_columns, data, rows, count, rng):
    """The same for an internet-record export; the unit of damage is a session row. Returns (data, rows, defects)."""
    if not count:
        return data, rows, []
    fields = entry["fields"]
    start_pos = index[column_ref(time_columns["start"])]
    free = [i for i, r in enumerate(rows) if not r["plant"]]
    taken = set()
    chosen = {}
    for kind in SESSION_KINDS:
        pool = sorted(set(free) - taken)
        picks = sorted(rng.sample(pool, min(count, len(pool)))) if pool else []
        taken.update(picks)
        chosen[kind] = picks
    defects, inserts = [], []
    for kind, picks in chosen.items():
        for i in picks:
            row, cells = rows[i], data[i]
            truth = {"uid": row["uid"], "start_local": row["start_local"]}
            if kind == "time_unparsed":
                cells[start_pos] = "not-a-time"
                row.update({"status": "quarantined", "reason": "time_unparsed"})
            elif kind == "time_missing":
                cells[start_pos] = ""
                row.update({"status": "quarantined", "reason": "time_missing"})
            elif kind == "invalid_server_ip":
                cells[index[column_ref(fields["server_ip"])]] = "999.1.1.1"
                row.update({"server_ip": None, "flags": ["ip_invalid:server_ip"]})
            elif kind == "server_port_range":
                cells[index[column_ref(fields["server_port"])]] = "70000"
                row.update({"server_port": None, "flags": ["server_port_out_of_range"]})
            elif kind == "duplicate_row":
                inserts.append((i, list(cells), dict(row, duplicate_of=row["uid"], defect="duplicate_row")))
                defects.append({"kind": kind, "uid": row["uid"], "truth": truth})
                continue
            elif kind == "blank_row":
                inserts.append((i, [""] * len(cells), {"uid": None, "status": "skipped", "reason": "blank_row", "defect": "blank_row", "plant": None}))
                defects.append({"kind": kind, "uid": None, "truth": None})
                continue
            row["defect"] = kind
            defects.append({"kind": kind, "uid": row["uid"], "truth": truth})
    for after, cells, row in sorted(inserts, key=lambda t_: -t_[0]):
        data.insert(after + 1, cells)
        rows.insert(after + 1, row)
    return data, rows, defects
