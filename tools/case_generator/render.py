"""Rendering: the canonical events of one subscription, or of a whole cell population, as a provider export in one of the registered
call-record layouts (target exports C1 to C4, tower dumps C5 to C8).

The renderer is the inverse of the layout registry's reference interpreter. Labels, header names, time formats, number forms and
cell identities all come from the layout entry; nothing here knows a real file. Each row carries the expectation the round trip must
reproduce (see test_case_generator.py). What a layout cannot say, it does not say: C6's numeric direction codes read as `unknown`,
C8's single activity code reads as `other`, C7 has no call-type column; the row's expectation holds what the interpreter will read,
and `true_direction` / `true_event_type` hold what was planted.
"""
from __future__ import annotations

import datetime
import os
import random
import sys
from dataclasses import dataclass, field

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import world as w  # noqa: E402  (sets up sys.path for the repository and the tools)
from ingestion.forensic_records.layout_registry.parties import make_cell_key  # noqa: E402
from ingestion.forensic_records.layout_registry.spec import column_ref  # noqa: E402

mg = w.mg
PERSON_FORMS = ("e164_12", "local_11", "local_10", "plus_e164")
SERVICE_KIND = {label: kind for kind, label in w.SERVICES}
CANNOT_SAY = {"event_type": "other", "direction": "unknown"}   # what the interpreter reads where the layout has no way to say it


@dataclass
class RenderedFile:
    layout_id: str
    subject: str
    table: list
    header_index: int
    numeric_columns: set
    container: str
    sheet_name: str
    encoding: str | None
    delimiter: str | None
    rows: list = field(default_factory=list)          # one expectation per data row, in order
    assumptions: list = field(default_factory=list)   # layout items the rendering relied on that are still unconfirmed
    skipped: list = field(default_factory=list)       # event uids not representable in this layout, with the reason

    def write(self, directory, stem):
        golden = mg.Golden(self.layout_id, self.container, self.sheet_name, self.encoding, self.delimiter, self.header_index,
                           self.table[self.header_index], self.table, [], self.numeric_columns)
        return golden.write(directory, stem=stem)


def label_choice(entry, event_type, direction):
    """The labels that make the layout read (event_type, direction).

    Returns ([(column ref, label, confirmed)], what the file will say). Phase 1 picks labels that mean exactly what was planted (confirmed
    rules first). Phase 2 covers a key the layout can only express as "unknown" or "other" (C6's codes, C8's activity code), preferring
    the label whose proposed meaning is the planted one. Phase 3 covers a key the layout has no rule for at all (C7 has no call type).
    A key the layout can express only wrongly (C2 has no message label) raises ValueError: the event is left out of that file.
    """
    desired = {"event_type": event_type, "direction": direction}
    by_column = {}
    for rule in entry.get("enums") or []:
        by_column.setdefault(column_ref(rule["column"]), []).append(rule)
    picks, covered = [], {}
    for ref, rules in sorted(by_column.items()):
        best = None
        for rule in rules:
            for label in sorted(rule["map"]):
                meaning = rule["map"][label]
                if meaning and all(desired.get(k) == v for k, v in meaning.items()):
                    score = (len(meaning), bool(rule["confirmed"]))
                    if best is None or score > best[0]:
                        best = (score, label, bool(rule["confirmed"]), meaning)
        if best:
            picks.append((ref, best[1], best[2]))
            covered.update(best[3])
    for key in sorted(set(desired) - set(covered)):
        found = None
        for ref, rules in sorted(by_column.items()):
            if any(ref == p[0] for p in picks):
                continue
            for rule in rules:
                for label in sorted(rule["map"]):
                    meaning = rule["map"][label]
                    if meaning.get(key) == CANNOT_SAY[key] and all(desired.get(k) == v for k, v in meaning.items() if k != key):
                        score = ((rule.get("proposed") or {}).get(label) == desired[key], bool(rule["confirmed"]))
                        if found is None or score > found[0]:
                            found = (score, ref, label, bool(rule["confirmed"]), meaning)
        if found:
            picks.append((found[1], found[2], found[3]))
            covered.update(found[4])
    keys_with_rules = {k for rules in by_column.values() for rule in rules for meaning in rule["map"].values() for k in meaning}
    for key in sorted(set(desired) - set(covered)):
        if key not in keys_with_rules:
            covered[key] = CANNOT_SAY[key]
    if set(desired) - set(covered):
        raise ValueError("layout %s cannot express %s" % (entry["layout_id"], desired))
    return picks, covered


def cell_identity(entry, hints, operator, site):
    """(lac, ci, raw, cell_key) of a site on an operator, as the layout writes it."""
    spec = entry.get("cell")
    if not spec:
        return None, None, None, None
    pool = operator * w.CELLS_PER_OPERATOR + site
    ci = mg.cell_value(hints.get("cell_style", "digits5"), None, pool)
    lac = str(100 + pool * 11) if hints.get("lac_digits", 3) == 3 else str(10000 + pool * 13)
    raw = str(3000000000 + pool * 104729) if spec.get("raw") else None
    lac_used = lac if spec.get("lac_tac") else None
    return lac_used, ci, raw, make_cell_key(spec.get("plmn"), lac_used, ci)


def layout_hints(entry):
    hints = dict((entry.get("golden") or {}).get("hints") or {})
    declared = (entry.get("time") or {}).get("date_order")
    if declared:
        hints["date_order"] = declared
    return hints


def site_cell_key(entry, operator, site):
    return cell_identity(entry, layout_hints(entry), operator, site)[3]


def subject_views(world, subject, only_events=None):
    return [(ev, subject) for ev in world.events if subject.pid in (ev.orig, ev.recip) and (only_events is None or ev.uid in only_events)]


def dump_views(world, operator, sites, start, end):
    """A tower dump: every event in the window in which a party of this operator was at one of the cells, as that party's view."""
    by_pid = {p.pid: p for p in world.persons}
    views = []
    for ev in world.events:
        if not (start <= ev.start <= end):
            continue
        for pid in (ev.orig, ev.recip):
            person = by_pid.get(pid)
            if person is not None and person.operator == operator and w.cell_for(world, person, ev) in sites:
                views.append((ev, person))
    return views


def render_subject(world, entry, subject, only_events=None) -> RenderedFile:
    """The export of one subject: every event the subject takes part in that this layout can express."""
    return render_views(world, entry, subject_views(world, subject, only_events), label=subject.pid, preamble_person=subject)


def render_views(world, entry, views, label, preamble_person=None) -> RenderedFile:
    hints = layout_hints(entry)
    fields = entry.get("fields") or {}
    index = mg.column_index(entry)
    n_columns = len(entry["header"]["columns"])
    time_columns = (entry.get("time") or {}).get("columns", {})
    fmt = entry["time"]["formats"][0]
    dur_spec = entry.get("duration") or {"mode": "none"}
    cell_spec = entry.get("cell")
    recipes = (entry.get("golden") or {}).get("columns") or {}
    forms = [f for f in hints.get("party_b_forms") or [] if f in PERSON_FORMS]
    service_kinds = {f for f in hints.get("party_b_forms") or [] if f in ("short_code", "sender_id", "service_label")}
    subject_form = hints.get("subject_form", "e164_12")
    container, encoding, delimiter = entry["container"]["kinds"][0], entry["container"].get("encoding"), entry["container"].get("delimiter")
    numeric = {index[column_ref(ref)] for ref in time_columns.values()} if fmt == "excel_serial_1900" else set()
    out = RenderedFile(entry["layout_id"], label, [], 0, numeric, container, "Sheet1", encoding, delimiter)
    by_pid = {p.pid: p for p in world.persons}
    data = []
    used_unconfirmed = set()
    for ev, subject in views:
        outgoing = ev.orig == subject.pid
        other_id = ev.recip if outgoing else ev.orig
        direction = "outgoing" if outgoing else "incoming"
        event_type = "voice" if ev.kind == "call" else "sms"
        r = random.Random("render:%d:%s:%s" % (world.params.seed, subject.pid, ev.uid))
        if other_id in by_pid:
            other = by_pid[other_id]
            if not forms:
                out.skipped.append([ev.uid, "layout has no person-number form"])
                continue
            b_raw = mg.party_forms(other.n)[r.choice(forms)]
            b_kind, b_norm, peer = "person_like", other.msisdn, w.OPERATOR_LABELS[other.operator]
        else:
            kind = SERVICE_KIND[other_id]
            if kind not in service_kinds:
                out.skipped.append([ev.uid, "layout has no %s form" % kind])
                continue
            b_raw, b_kind, b_norm, peer = other_id, kind, None, "SHORTCODE"
        if event_type == "sms" and ev.duration_s:
            raise ValueError("an SMS has no duration")
        try:
            labels, visible = label_choice(entry, event_type, direction)
        except ValueError:
            out.skipped.append([ev.uid, "layout has no label for %s %s" % (direction, event_type)])
            continue
        cells = [""] * n_columns

        def put(ref, value):
            key = ref if isinstance(ref, tuple) else column_ref(ref)
            cells[index[key]] = "" if value is None else str(value)

        for ref, text, confirmed in labels:
            put(ref, text)
            if not confirmed:
                used_unconfirmed.add("enum:%s=%s" % (ref[0], text))
        a_raw = mg.party_forms(subject.n)[subject_form]
        b_spec = fields.get("b")
        if isinstance(b_spec, dict) and "by_direction" in b_spec:
            put(b_spec["by_direction"][direction], b_raw)
            put(b_spec["by_direction"]["incoming" if outgoing else "outgoing"], a_raw)
        else:
            put(b_spec["column"] if isinstance(b_spec, dict) else b_spec, b_raw)
        put(fields["a"], a_raw)
        imei = imsi = None
        if fields.get("imei"):
            imei = mg.imei_for(subject.n, int(hints.get("imei_digits", 15)))
            put(fields["imei"], imei)
        if fields.get("imsi"):
            imsi = subject.imsi
            put(fields["imsi"], imsi)
        start = ev.start
        end = start + datetime.timedelta(seconds=ev.duration_s)
        if fmt == "slash_24h_minutes":
            raise ValueError("minute-precision layouts are not used by the call-record exports")
        put(time_columns["start"], mg.render_time(start, fmt, hints))
        if "end" in time_columns:
            put(time_columns["end"], mg.render_time(end, fmt, hints))
        duration_expect = None
        if dur_spec["mode"] == "column_seconds":
            put(dur_spec["column"], ev.duration_s)
            duration_expect = ev.duration_s
        elif dur_spec["mode"] == "minutes_seconds":
            put(dur_spec["minutes"], ev.duration_s // 60)
            put(dur_spec["seconds"], ev.duration_s % 60)
            duration_expect = ev.duration_s
        elif dur_spec["mode"] == "from_times":
            duration_expect = ev.duration_s
        site = w.cell_for(world, subject, ev)
        cell_key = None
        if cell_spec:
            lac, ci, raw_id, cell_key = cell_identity(entry, hints, subject.operator, site)
            if cell_spec.get("lac_tac"):
                put(cell_spec["lac_tac"], lac)
            put(cell_spec["ci"], ci)
            if cell_spec.get("raw"):
                put(cell_spec["raw"], raw_id)
        lat, lon = w.cell_geometry(subject.operator, site)
        places = int(hints.get("coordinate_decimals", 5))
        lat_spec = fields.get("lat")
        if isinstance(lat_spec, dict) and "combined" in lat_spec:
            first, second = (lon, lat) if lat_spec["order"] == "lon_lat" else (lat, lon)
            put(lat_spec["combined"], "%.*f %.*f" % (places, first, places, second))
        elif lat_spec:
            put(fields["lat"], "%.*f" % (places, lat))
            put(fields["lon"], "%.*f" % (places, lon))
        if fields.get("location_text"):
            put(fields["location_text"], "Site %02d" % site)
        peer_spec = fields.get("peer_operator")
        if peer_spec:
            put(peer_spec["column"], peer)
        filled = {pos for pos, cell in enumerate(cells) if cell != ""}
        for name, recipe in recipes.items():
            if recipe in ("noise_text", "noise_mac"):
                continue
            for (col, occ), pos in index.items():
                if col == name and pos not in filled:
                    cells[pos] = mg.recipe_value(recipe, r, len(data), {"start": start, "end": end})
        data.append(cells)
        out.rows.append({
            "uid": ev.uid, "a_norm": subject.msisdn, "b_norm": b_norm, "b_kind": b_kind, "b_raw_label": None if b_norm else b_raw,
            "direction": visible["direction"], "event_type": visible["event_type"], "true_direction": direction, "true_event_type": event_type,
            "start_local": start.isoformat(sep=" "), "duration_s": duration_expect, "cell_key": cell_key, "site": site, "operator": subject.operator,
            "imsi": imsi, "imei": imei, "plant": ev.plant})
    header = [c["name"] for c in entry["header"]["columns"]]
    padding = int((entry.get("golden") or {}).get("blank_columns") or 0)
    if padding:
        at = max(0, len(header) - 2)
        header = header[:at] + [""] * padding + header[at:]
        data = [cells[:at] + [""] * padding + cells[at:] for cells in data]
        out.numeric_columns = {i + padding if i >= at else i for i in out.numeric_columns}
    preamble = _preamble(entry, preamble_person, random.Random("preamble:%d:%s" % (world.params.seed, label)))
    out.table = preamble + [header] + data
    out.header_index = len(preamble)
    out.assumptions = [{"layout_id": entry["layout_id"], "item": item, "used": "unconfirmed rule of the entry"} for item in sorted(used_unconfirmed)]
    return out


def _preamble(entry, subject, rng):
    spec = (entry.get("golden") or {}).get("preamble")
    if not spec or subject is None:
        return []
    values = {"{subject10}": mg.party_forms(subject.n)["local_10"], "{ref13}": "00000-%07d-0" % rng.randint(0, 9999999)}
    rows = []
    for row in spec:
        cells = []
        for cell in row:
            for key, value in values.items():
                cell = cell.replace(key, value)
            cells.append(cell)
        rows.append(cells)
    return rows
