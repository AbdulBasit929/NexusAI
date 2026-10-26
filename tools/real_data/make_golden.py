#!/usr/bin/env python3
"""Synthetic golden files for the layout registry: a file with the SAME SHAPE as a provider layout and none of its data.

    python -I -X utf8 tools/real_data/make_golden.py --layout call-c4-telenor-style --out DIR [--rows 48] [--seed 1]
    python -I -X utf8 tools/real_data/make_golden.py --all --out DIR

For each layout entry it writes a file in the same container (xlsx, SpreadsheetML, CSV or UTF-16 tab-separated text) with the
same header names and position, the same preamble, time formats, enumerations, number classes and quirks (typos, duplicate
headers), and values drawn only from the reserved ranges of tools/real_data/reserved.py. It also returns the TRUTH it
generated: what each row must be read as. The tests read the file back through the registry and compare.

What a golden proves: the registry machinery and the entry are consistent with each other. It does not prove the entry is
right about the real file; that is what profile_structure.py plus confirm_layout.py (and the reconciliation) are for.
"""
import argparse
import datetime
import io
import os
import random
import sys
import zipfile
from dataclasses import dataclass, field
from decimal import Decimal, ROUND_HALF_EVEN
from xml.sax.saxutils import escape

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.abspath(os.path.join(HERE, "..", "..")))
import reserved  # noqa: E402
from ingestion.forensic_records.layout_registry import load_registry  # noqa: E402
from ingestion.forensic_records.layout_registry.matching import normalize_header  # noqa: E402
from ingestion.forensic_records.layout_registry.parties import make_cell_key  # noqa: E402
from ingestion.forensic_records.layout_registry.spec import column_ref  # noqa: E402

BASE_DATE = datetime.datetime(2026, 1, 5, 6, 0, 0)
EPOCH_1900 = datetime.datetime(1899, 12, 30)
SERVICE_LABELS = ["INTERNET", "UNKNOWN", "GPRS"]
SENDERS = ["BankA", "ShopB", "4a123b5c", "TelcoInfo"]
SHORT_CODES = ["786", "8585", "667788", "5000"]
NOISE_TEXT = ["Example Virtual Adapter", "Example Wi-Fi Direct Adapter", "Example Ethernet Connection"]
NOISE_MAC = ["00:00:5e:00:53:01", "00:00:5e:00:53:02", "00:00:5e:00:53:03"]


@dataclass
class Golden:
    layout_id: str
    container: str
    sheet_name: str
    encoding: str | None
    delimiter: str | None
    header_index: int
    header: list
    table: list                      # preamble rows, header, data rows: every cell as written
    truth: list                      # one dict per data row
    numeric_columns: set = field(default_factory=set)
    date_order: str | None = None    # the day-or-month order the generated slash dates follow

    @property
    def rows(self):
        return self.table[self.header_index + 1:]

    def write(self, directory, stem=None):
        os.makedirs(directory, exist_ok=True)
        ext = {"xlsx": ".xlsx", "spreadsheetml": ".xls", "csv": ".csv", "tsv": ".txt"}[self.container]
        path = os.path.join(directory, (stem or self.layout_id + ".synthetic") + ext)
        if self.container == "xlsx":
            write_xlsx(path, self.sheet_name, self.table, self.numeric_columns, self.header_index)
        elif self.container == "spreadsheetml":
            write_spreadsheetml(path, self.sheet_name, self.table)
        else:
            write_delimited(path, self.table, self.encoding or "utf-8", self.delimiter or ",")
        return path


# ----------------------------------------------------------------------------------------------------------------------------
# Container writers
# ----------------------------------------------------------------------------------------------------------------------------
def _col_letter(index):
    letters = ""
    index += 1
    while index:
        index, rem = divmod(index - 1, 26)
        letters = chr(65 + rem) + letters
    return letters


FIXED_ZIP_TIME = (2026, 1, 1, 0, 0, 0)


def _put_part(archive, name, data):
    """A zip member with a fixed timestamp, so the same table always gives the same bytes."""
    info = zipfile.ZipInfo(name, date_time=FIXED_ZIP_TIME)
    info.compress_type = zipfile.ZIP_DEFLATED
    archive.writestr(info, data)


def write_xlsx(path, sheet_name, table, numeric_columns, header_index):
    ns = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
    rel_ns = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
    body = []
    for r, row in enumerate(table, start=1):
        cells = []
        for c, value in enumerate(row):
            if value == "" or value is None:
                continue
            ref = "%s%d" % (_col_letter(c), r)
            if c in numeric_columns and r - 1 > header_index:
                cells.append('<c r="%s"><v>%s</v></c>' % (ref, value))
            else:
                cells.append('<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>' % (ref, escape(str(value))))
        body.append('<row r="%d">%s</row>' % (r, "".join(cells)))
    with zipfile.ZipFile(path, "w", zipfile.ZIP_DEFLATED) as z:
        _put_part(z, "[Content_Types].xml",
                   '<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
                   '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>'
                   '<Default Extension="xml" ContentType="application/xml"/>'
                   '<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>'
                   '<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>'
                   '</Types>')
        _put_part(z, "_rels/.rels",
                   '<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
                   '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>')
        _put_part(z, "xl/workbook.xml",
                   '<?xml version="1.0" encoding="UTF-8"?><workbook xmlns="%s" xmlns:r="%s"><sheets><sheet name="%s" sheetId="1" r:id="rId1"/></sheets></workbook>'
                   % (ns, rel_ns, escape(sheet_name, {'"': "&quot;"})))
        _put_part(z, "xl/_rels/workbook.xml.rels",
                   '<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
                   '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>')
        _put_part(z, "xl/worksheets/sheet1.xml",
                   '<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="%s"><sheetData>%s</sheetData></worksheet>' % (ns, "".join(body)))


def write_spreadsheetml(path, sheet_name, table):
    rows = []
    for row in table:
        cells = "".join('<Cell><Data ss:Type="String">%s</Data></Cell>' % escape(str(v)) if v != "" else "<Cell/>" for v in row)
        rows.append("<Row>%s</Row>" % cells)
    xml = ('<?xml version="1.0"?><Workbook xmlns="urn:schemas-microsoft-com:office:spreadsheet" xmlns:ss="urn:schemas-microsoft-com:office:spreadsheet">'
           '<Worksheet ss:Name="%s"><Table>%s</Table></Worksheet></Workbook>' % (escape(sheet_name, {'"': "&quot;"}), "".join(rows)))
    with open(path, "w", encoding="utf-8", newline="") as handle:
        handle.write(xml)


def write_delimited(path, table, encoding, delimiter):
    def quote(v):
        v = "" if v is None else str(v)
        return '"%s"' % v.replace('"', '""') if any(ch in v for ch in (delimiter, '"', "\n", "\r")) else v
    text = "\r\n".join(delimiter.join(quote(v) for v in row) for row in table) + "\r\n"
    if encoding == "utf-8-sig":
        data = b"\xef\xbb\xbf" + text.encode("utf-8")
    elif encoding == "utf-16":
        data = b"\xff\xfe" + text.encode("utf-16-le")
    else:
        data = text.encode(encoding)
    with open(path, "wb") as handle:
        handle.write(data)


# ----------------------------------------------------------------------------------------------------------------------------
# Value rendering
# ----------------------------------------------------------------------------------------------------------------------------
def render_time(dt, fmt, hints):
    if fmt == "excel_serial_1900":
        seconds = Decimal(int((dt - EPOCH_1900).total_seconds()))
        places = int(hints.get("serial_decimals", 12))
        return str((seconds / Decimal(86400)).quantize(Decimal(1).scaleb(-places), rounding=ROUND_HALF_EVEN))
    if fmt == "iso_naive":
        text = dt.strftime("%Y-%m-%d %H:%M:%S")
        return text + ".%03d" % (dt.microsecond // 1000) if hints.get("iso_millis") else text
    order = hints.get("date_order", "DMY")
    first, second = (dt.day, dt.month) if order == "DMY" else (dt.month, dt.day)
    if fmt == "slash_24h_seconds":
        return "%02d/%02d/%d %02d:%02d:%02d" % (first, second, dt.year, dt.hour, dt.minute, dt.second)
    if fmt == "slash_24h_minutes":
        return "%02d/%02d/%d %02d:%02d" % (first, second, dt.year, dt.hour, dt.minute)
    if fmt == "slash_12h_seconds":
        hour, ampm = dt.hour % 12 or 12, "AM" if dt.hour < 12 else "PM"
        return "%d/%02d/%d %d:%02d:%02d %s" % (first, second, dt.year, hour, dt.minute, dt.second, ampm)
    if fmt == "dot_dmy_24h_seconds":
        return "%02d.%02d.%d %02d:%02d:%02d" % (dt.day, dt.month, dt.year, dt.hour, dt.minute, dt.second)
    if fmt == "slash_date":
        return "%02d/%02d/%d" % (first, second, dt.year)
    raise ValueError("cannot render time format %s" % fmt)


def party_forms(n):
    e164 = reserved.synthetic_msisdn(n)
    return {"e164_12": e164, "local_11": "0" + e164[2:], "local_10": e164[2:], "plus_e164": "+" + e164}


def make_party(form, rng, n):
    """Return (raw, kind, norm) for one counterparty form."""
    if form in ("e164_12", "local_11", "local_10", "plus_e164"):
        forms = party_forms(n)
        return forms[form], "person_like", forms["e164_12"]
    if form == "short_code":
        return rng.choice(SHORT_CODES), "short_code", None
    if form == "sender_id":
        return rng.choice(SENDERS), "sender_id", None
    if form == "service_label":
        return rng.choice(SERVICE_LABELS), "service_label", None
    if form == "international_00":
        digits = "999" + "%09d" % (n * 7919 % 10 ** 9)
        return "00" + digits, "international", digits
    raise ValueError("unknown party form %s" % form)


def imei_for(n, digits, defect=False):
    base = reserved.synthetic_imei(n)                      # 15 digits, test TAC prefix 00
    if defect:
        return base[2:]                                    # 13 digits: a known defect of one layout
    if digits == 14:
        return base[:14]
    if digits == 16:
        return base[:14] + "01"
    return base


def cell_value(style, rng, pool_index):
    if style == "digits5":
        return str(10000 + pool_index * 37 % 89999)
    if style == "digits8":
        return str(10000000 + pool_index * 7919 % 89999999)
    if style == "digits10":
        return str(3000000000 + pool_index * 104729 % 999999999)
    if style == "alnum7":
        return "%03d%s%02d%s" % (100 + pool_index, chr(65 + pool_index % 26), 10 + pool_index, chr(66 + pool_index % 25))
    raise ValueError(style)


def recipe_value(recipe, rng, i, ctx):
    if recipe == "row_number":
        return str(i + 1)
    if recipe == "empty":
        return ""
    if recipe == "noise_text":
        return rng.choice(NOISE_TEXT) if rng.random() < 0.06 else ""
    if recipe == "noise_mac":
        return rng.choice(NOISE_MAC) if rng.random() < 0.03 else ""
    if recipe.startswith("choice:"):
        return rng.choice(recipe[7:].split("|"))
    if recipe.startswith("int:"):
        lo, hi = recipe[4:].split("-")
        return str(rng.randint(int(lo), int(hi)))
    if recipe == "digits15_or_empty":
        return reserved.synthetic_imei(rng.randint(1, 999)) if rng.random() < 0.5 else ""
    if recipe == "date_of_start":
        return ctx["start"].strftime("%d/%m/%Y")
    if recipe == "ts_ms_of_end":
        return render_time(ctx["end"], "iso_naive", {"iso_millis": True})
    if recipe == "mostly_empty_host":
        return "host-%d.example.test" % rng.randint(1, 9) if rng.random() < 0.05 else ""
    if recipe == "mostly_empty_path":
        return "/path/%d" % rng.randint(1, 99) if rng.random() < 0.05 else ""
    if recipe == "mostly_empty_agent":
        return "ExampleAgent/1.%d" % rng.randint(0, 9) if rng.random() < 0.05 else ""
    raise ValueError("unknown recipe %s" % recipe)


# ----------------------------------------------------------------------------------------------------------------------------
# Generation
# ----------------------------------------------------------------------------------------------------------------------------
def column_index(entry):
    index, seen = {}, {}
    for i, column in enumerate(entry["header"]["columns"]):
        seen[column["name"]] = seen.get(column["name"], 0) + 1
        index[(column["name"], seen[column["name"]])] = i
    return index


def sample_labels(entry, rng):
    """For each enumerated column pick one label (every label of every rule is reachable) and what it means."""
    by_column = {}
    for rule in entry.get("enums") or []:
        by_column.setdefault(column_ref(rule["column"]), []).append(rule)
    chosen = {}
    for ref, rules in by_column.items():
        options = [(label, mapped, rule["confirmed"]) for rule in rules for label, mapped in rule["map"].items()]
        chosen[ref] = options[rng.randrange(len(options))]
    return chosen, by_column


def generate(entry, rows=None, seed=1, date_order=None, defects=True):
    kind = entry["dataset_kind"]
    hints = dict((entry.get("golden") or {}).get("hints") or {})
    declared = (entry.get("time") or {}).get("date_order")
    if declared:
        hints["date_order"] = declared  # an entry that has settled its order is written in that order
    if date_order:
        hints["date_order"] = date_order
    recipes = (entry.get("golden") or {}).get("columns") or {}
    n_rows = rows or (entry.get("golden") or {}).get("rows") or 48
    rng = random.Random("%s:%d" % (entry["layout_id"], seed))
    index = column_index(entry)
    width = len(entry["header"]["columns"])
    formats = (entry.get("time") or {}).get("formats", [])
    time_columns = (entry.get("time") or {}).get("columns", {})
    fmt = formats[0] if formats else None
    builder = {"target_cdr": _comm_row, "tower_dump": _comm_row, "ipdr": _session_row, "app_usage": _usage_row, "plate_sighting": _plate_row}[kind]
    data, truth = [], []
    ctx = {"entry": entry, "hints": hints, "recipes": recipes, "rng": rng, "index": index, "width": width, "fmt": fmt,
           "time_columns": time_columns, "kind": kind}
    defect_at = {int(n_rows * 0.20): "blank", int(n_rows * 0.35): "time_unparsed", int(n_rows * 0.50): "time_missing",
                 int(n_rows * 0.65): "unknown_label", int(n_rows * 0.80): "party_sentinel"} if defects else {}
    for i in range(n_rows):
        defect = defect_at.get(i)
        cells, expect = builder(i, ctx, defect)
        data.append(cells)
        truth.append(expect)
    container, encoding, delimiter = _container(entry)
    preamble = _preamble(entry, ctx)
    header_cells = [c["name"] for c in entry["header"]["columns"]]
    numeric = {index[column_ref(ref)] for ref in time_columns.values()} if fmt == "excel_serial_1900" else set()
    # Real call-record templates carry empty, unnamed header cells between the data columns and the last two (Description, MACAddress);
    # a blank cell at the very end would be trimmed by the readers, so the padding goes before the last two named columns.
    padding = int((entry.get("golden") or {}).get("blank_columns") or 0)
    if padding:
        at = max(0, len(header_cells) - 2)
        header_cells = header_cells[:at] + [""] * padding + header_cells[at:]
        data = [cells[:at] + [""] * padding + cells[at:] for cells in data]
        numeric = {i + padding if i >= at else i for i in numeric}
    table = preamble + [header_cells] + data
    return Golden(entry["layout_id"], container, "Sheet1" if container != "spreadsheetml" else "TITLE", encoding, delimiter,
                  len(preamble), header_cells, table, truth, numeric, hints.get("date_order") if fmt and fmt.startswith(("slash", "dot")) else None)


def _container(entry):
    c = entry["container"]
    kinds = c["kinds"]
    kind = kinds[0]
    return kind, c.get("encoding"), c.get("delimiter")


def _preamble(entry, ctx):
    spec = (entry.get("golden") or {}).get("preamble")
    if not spec:
        return []
    rng = ctx["rng"]
    subject = party_forms(0)["local_10"]
    values = {"{subject10}": subject, "{ref13}": "00000-%07d-%d" % (rng.randint(0, 9999999), 0),
              "{ts_start}": BASE_DATE.strftime("%Y-%m-%d %H:%M:%S"), "{ts_end}": (BASE_DATE + datetime.timedelta(days=10)).strftime("%Y-%m-%d %H:%M:%S"),
              "{public_ip}": reserved.synthetic_ipv4(0)}
    rows = []
    for row in spec:
        out = []
        for cell in row:
            for key, value in values.items():
                cell = cell.replace(key, value)
            out.append(cell)
        rows.append(out)
    return rows


def _put(cells, index, ref, value):
    key = ref if isinstance(ref, tuple) else column_ref(ref)
    cells[index[key]] = "" if value is None else str(value)


def _fill_other_columns(cells, ctx, i, mapped_positions, context):
    rng, recipes = ctx["rng"], ctx["recipes"]
    for name, recipe in recipes.items():
        for (col, occ), pos in ctx["index"].items():
            if col == name and pos not in mapped_positions:
                cells[pos] = recipe_value(recipe, rng, i, context)


def _trunc(dt, ctx):
    """A layout with minute precision cannot carry seconds: the truth must not either."""
    return dt.replace(second=0, microsecond=0) if ctx["fmt"] == "slash_24h_minutes" else dt


def _times(i, ctx):
    rng = ctx["rng"]
    start = BASE_DATE + datetime.timedelta(days=rng.randint(0, 9), minutes=rng.randint(0, 600), seconds=rng.randint(0, 59))
    return _trunc(start, ctx)


def _comm_row(i, ctx, defect):
    entry, hints, rng, index = ctx["entry"], ctx["hints"], ctx["rng"], ctx["index"]
    fields, kind = entry.get("fields") or {}, ctx["kind"]
    width = ctx["width"]
    cells = [""] * width
    if defect == "blank":
        return cells, {"status": "skipped", "reason": "blank_row", "tag": "blank"}
    subject_n = (i % 5) + 1 if kind == "tower_dump" else 1
    forms = party_forms(subject_n)
    subject_form = hints.get("subject_form", "e164_12")
    a_raw = forms[subject_form]
    labels, by_column = sample_labels(entry, rng)
    mapped, flags, enum_flags = {}, set(), set()
    for ref, (label, meaning, confirmed) in labels.items():
        if defect == "unknown_label" and any(meaning_key == "direction" for meaning_key in meaning):
            label = "Sideways"
            meaning = {}
            targets = sorted({key for rule in by_column[ref] for value in rule["map"].values() for key in value})
            enum_flags.add("unmapped_label:" + "/".join(targets))
        else:
            if not confirmed:
                enum_flags.add("enum_unconfirmed:" + "/".join(sorted(meaning)))
        mapped.update(meaning)
        _put(cells, index, ref, label)
    direction = mapped.get("direction", "unknown")
    event_type = mapped.get("event_type")
    if event_type is None:
        event_type = "other"
        flags.add("event_type_not_in_layout")
    # counterparty
    b_forms = hints.get("party_b_forms") or ["e164_12"]
    form = rng.choice(b_forms)
    b_spec = fields.get("b")
    if form == "self":
        b_raw, b_kind, b_norm = a_raw, "person_like", forms["e164_12"]
    else:
        b_raw, b_kind, b_norm = make_party(form, rng, 100 + i)
    b_expect = {"b_raw": b_raw, "b_kind": b_kind, "b_norm": b_norm}
    if defect == "party_sentinel":
        b_raw, b_kind, b_norm = "-1", None, None
        b_expect = {"b_raw": "-1", "b_kind": None, "b_norm": None}
        flags.add("b_party_sentinel")
    elif form == "self" and isinstance(b_spec, dict) and b_spec.get("drop_if_equal_to_a"):
        b_expect = {"b_raw": None, "b_kind": None, "b_norm": None}
        flags.add("counterparty_equals_subject")
    if isinstance(b_spec, dict) and "by_direction" in b_spec:
        target = b_spec["by_direction"].get(direction)
        other = b_spec["by_direction"].get("incoming" if direction == "outgoing" else "outgoing")
        if target is None:
            b_expect = {"b_raw": None, "b_kind": None, "b_norm": None}
            flags.add("counterparty_direction_unresolved")
        else:
            _put(cells, index, target, b_raw)
        if other is not None and direction in ("incoming", "outgoing"):
            _put(cells, index, other, a_raw)
    elif b_spec is not None:
        _put(cells, index, b_spec["column"] if isinstance(b_spec, dict) else b_spec, b_raw)
    a_spec = fields["a"]
    a_unresolved = isinstance(a_spec, dict) and "by_direction" in a_spec and direction not in ("incoming", "outgoing")
    if a_unresolved:
        flags.add("subject_direction_unresolved")
        flags.add("subject_missing")
    if isinstance(a_spec, dict) and "by_direction" in a_spec:
        # a caller and a callee and no subscriber column: the b by_direction block above has written the subject into the column the direction picks
        if not (isinstance(b_spec, dict) and "by_direction" in b_spec):
            raise ValueError("a layout whose subject depends on the direction must say the same for the other party")
    else:
        _put(cells, index, a_spec, a_raw)
    # identifiers
    imei = None
    imei_flags = set()
    if fields.get("imei"):
        defect_imei = hints.get("imei_defect_every") and i % int(hints["imei_defect_every"]) == int(hints["imei_defect_every"]) - 1
        imei = imei_for(subject_n, int(hints.get("imei_digits", 15)), bool(defect_imei))
        if defect_imei:
            imei_flags.add("imei_length_13")
        _put(cells, index, fields["imei"], imei)
    imsi = reserved.synthetic_imsi(subject_n) if fields.get("imsi") else None
    if imsi:
        _put(cells, index, fields["imsi"], imsi)
    # time and duration
    start = _times(i, ctx)
    duration = 0 if event_type == "sms" else rng.randint(1, 900)
    end = start + datetime.timedelta(seconds=duration)
    time_cols, fmt = ctx["time_columns"], ctx["fmt"]
    context = {"start": start, "end": end}
    if defect == "time_unparsed":
        _put(cells, index, time_cols["start"], "not-a-time")
    elif defect == "time_missing":
        pass
    else:
        _put(cells, index, time_cols["start"], render_time(start, fmt, hints))
    if "end" in time_cols:
        _put(cells, index, time_cols["end"], render_time(end, fmt, hints))
    dur_spec = entry.get("duration") or {"mode": "none"}
    duration_expect = None
    if dur_spec["mode"] == "column_seconds":
        _put(cells, index, dur_spec["column"], duration)
        duration_expect = duration
    elif dur_spec["mode"] == "minutes_seconds":
        _put(cells, index, dur_spec["minutes"], duration // 60)
        _put(cells, index, dur_spec["seconds"], duration % 60)
        duration_expect = duration
    elif dur_spec["mode"] == "from_times":
        duration_expect = duration
    # cell
    cell_expect = {"cell_key": None, "lac_tac": None, "cell_raw": None}
    cell_spec = entry.get("cell")
    if cell_spec:
        pool = (i * 3) % 6
        ci = cell_value(hints.get("cell_style", "digits5"), rng, pool)
        lac = str(100 + pool * 11) if hints.get("lac_digits", 3) == 3 else str(10000 + pool * 13)
        if cell_spec.get("lac_tac"):
            _put(cells, index, cell_spec["lac_tac"], lac)
        _put(cells, index, cell_spec["ci"], ci)
        global_id = None
        if cell_spec.get("raw"):
            global_id = str(3000000000 + pool * 104729)
            _put(cells, index, cell_spec["raw"], global_id)
        cell_expect = {"cell_key": make_cell_key(cell_spec.get("plmn"), lac if cell_spec.get("lac_tac") else None, ci),
                       "lac_tac": lac if cell_spec.get("lac_tac") else None, "cell_raw": global_id or ci}
        flags.add("cell_identity_partial")
    # location
    lat = lon = None
    loc_text = None
    if fields.get("lat") and not (isinstance(fields["lat"], dict) and "combined" in fields["lat"]):
        if rng.random() < 0.9:
            places = int(hints.get("coordinate_decimals", 5))
            lat_text, lon_text = ("%.*f" % (places, -60.0 + rng.random())), ("%.*f" % (places, -30.0 + rng.random()))
            _put(cells, index, fields["lat"], lat_text)
            _put(cells, index, fields["lon"], lon_text)
            lat, lon = float(lat_text), float(lon_text)
    if fields.get("location_text"):
        loc_text = rng.choice(["Site A", "Site B", "?"])
        _put(cells, index, fields["location_text"], loc_text)
        loc_text = None if loc_text == "?" else loc_text
    peer = None
    peer_spec = fields.get("peer_operator")
    if peer_spec:
        value = rng.choice(["ufone", "UFONE", "zong", "telenor", "INTERNET", "SHORTCODE"])
        _put(cells, index, peer_spec["column"], value)
        peer = value.casefold() if value.casefold() in set(peer_spec.get("operators") or []) else None
    mapped_positions = {pos for pos, cell in enumerate(cells) if cell != ""}
    _fill_other_columns(cells, ctx, i, mapped_positions, context)
    if any(cells[index[column_ref(c)]] for c in entry.get("noise_columns") or []):
        flags.add("noise_columns_populated")
    flags |= enum_flags | imei_flags
    expect = {
        "status": "accepted", "tag": defect or "normal", "flags": sorted(flags),
        "record": {
            "event_type": event_type, "direction": direction, "a_raw": None if a_unresolved else a_raw, "a_kind": None if a_unresolved else "person_like", "a_norm": None if a_unresolved else forms["e164_12"],
            **b_expect, "start_local": start.isoformat(sep=" "), "end_local": end.isoformat(sep=" ") if "end" in time_cols else None,
            "duration_s": duration_expect, "imsi": imsi, "imei": imei, "tac": imei[:8] if imei and len(imei) >= 14 else None,
            **cell_expect, "lat": lat, "lon": lon, "location_text": loc_text, "peer_operator": peer,
        },
    }
    if defect == "time_unparsed":
        expect = {"status": "quarantined", "reason": "time_unparsed", "tag": defect}
    elif defect == "time_missing":
        expect = {"status": "quarantined", "reason": "time_missing", "tag": defect}
    return cells, expect


def _session_row(i, ctx, defect):
    entry, hints, rng, index = ctx["entry"], ctx["hints"], ctx["rng"], ctx["index"]
    fields = entry.get("fields") or {}
    cells = [""] * ctx["width"]
    if defect == "blank":
        return cells, {"status": "skipped", "reason": "blank_row", "tag": "blank"}
    subject_n = (i % 4) + 1
    forms = party_forms(subject_n)
    msisdn = forms["e164_12"]
    if defect == "party_sentinel" and not fields.get("server_port"):
        defect = None  # the defect is an out-of-range server port, and this layout states none
    start = _times(i, ctx)
    dur_ms = rng.randint(50, 600000)
    end = _trunc(start + datetime.timedelta(milliseconds=dur_ms), ctx)
    nat_start = start
    nat_end = end if rng.random() < 0.5 else None
    time_cols, fmt = ctx["time_columns"], ctx["fmt"]
    context = {"start": start, "end": end}
    if defect == "time_unparsed":
        _put(cells, index, time_cols["start"], "not-a-time")
    elif defect == "time_missing":
        pass
    else:
        _put(cells, index, time_cols["start"], render_time(start, fmt, hints))
    if "end" in time_cols:
        _put(cells, index, time_cols["end"], render_time(end, fmt, hints))
    if "nat_start" in time_cols:
        _put(cells, index, time_cols["nat_start"], render_time(nat_start, fmt, hints))
    if "nat_end" in time_cols and (nat_end is not None or hints.get("nat_end_always")):
        _put(cells, index, time_cols["nat_end"], render_time(nat_end or end, fmt, hints))
    flags = set()
    _put(cells, index, fields["msisdn"], msisdn)
    # a layout need not state every field (the 8-column seed export has no IMSI, ports or public address): what it does not state is None in the truth
    imsi = reserved.synthetic_imsi(subject_n) if fields.get("imsi") else None
    if imsi:
        _put(cells, index, fields["imsi"], imsi)
    imei = None
    if fields.get("imei"):
        imei = imei_for(subject_n, int(hints.get("imei_digits", 15)))
        _put(cells, index, fields["imei"], imei)
    ue_ip = "10.%d.%d.%d" % (rng.randint(0, 255), rng.randint(0, 255), rng.randint(1, 254))
    public_ip = reserved.synthetic_ipv4(i)
    server_ip = reserved.synthetic_ipv4(300 + i) if i % 7 else reserved.synthetic_ipv6(i)
    ue_port, server_port = rng.randint(1024, 65535), rng.choice([53, 80, 443, 8080])
    style = hints.get("port_style", "public_single")
    port_from = rng.randint(1024, 60000)
    port_to = port_from if style == "public_single" else port_from + rng.randint(0, 999)
    if defect == "unknown_label":
        server_ip = "999.1.1.1"
        flags.add("ip_invalid:server_ip")
    if defect == "party_sentinel":
        server_port = 70000
        flags.add("server_port_out_of_range")
    _put(cells, index, fields["ue_ip"], ue_ip)
    if fields.get("ue_port"):
        _put(cells, index, fields["ue_port"], ue_port)
    else:
        ue_port = None
    if fields.get("public_ip"):
        _put(cells, index, fields["public_ip"], public_ip)
    else:
        public_ip = None
    if "public_port" in fields:
        _put(cells, index, fields["public_port"], port_from)
    elif fields.get("public_port_from"):
        _put(cells, index, fields["public_port_from"], port_from)
        _put(cells, index, fields["public_port_to"], port_to)
    else:
        port_from = port_to = None
    _put(cells, index, fields["server_ip"], server_ip)
    if fields.get("server_port"):
        _put(cells, index, fields["server_port"], server_port)
    else:
        server_port = None
    protocol_expect, protocol_num_expect = None, None
    if "protocol" in fields:
        if entry.get("protocol_numbers") == "iana":
            number = rng.choice([6, 17, 1])
            _put(cells, index, fields["protocol"], number)
            protocol_num_expect, protocol_expect = number, {6: "tcp", 17: "udp", 1: "icmp"}[number]
        elif "protocol" in ctx["recipes"]:
            pass
    bytes_up, bytes_down = rng.randint(0, 99999), rng.randint(0, 999999)
    values = {"bytes_up": bytes_up, "bytes_down": bytes_down, "packets_up": rng.randint(1, 99), "packets_down": rng.randint(1, 99)}
    expect_ints = {}
    for name, value in values.items():
        if fields.get(name):
            _put(cells, index, fields[name], value)
            expect_ints[name] = value
    if fields.get("duration_ms"):
        _put(cells, index, fields["duration_ms"], dur_ms)
        expect_ints["duration_ms"] = dur_ms
    cell_expect = {"cell_key": None, "lac_tac": None, "cell_raw": None}
    cell_spec = entry.get("cell")
    if cell_spec:
        pool = (i * 3) % 6
        ci = cell_value(hints.get("cell_style", "digits5"), rng, pool)
        lac = str(10000 + pool * 13)
        _put(cells, index, cell_spec["lac_tac"], lac)
        _put(cells, index, cell_spec["ci"], ci)
        cell_expect = {"cell_key": make_cell_key(cell_spec.get("plmn"), lac, ci), "lac_tac": lac, "cell_raw": ci}
        flags.add("cell_identity_partial")
    session_ref = None
    if fields.get("session_ref"):
        session_ref = "sid%06d" % i
        _put(cells, index, fields["session_ref"], session_ref)
    mapped_positions = {pos for pos, cell in enumerate(cells) if cell != ""}
    _fill_other_columns(cells, ctx, i, mapped_positions, context)
    # what the recipes wrote for mapped text fields
    text_expect = {}
    for name in ("apn", "rat", "app_category", "app", "app_feature", "host", "url", "user_agent", "protocol"):
        if name == "protocol" and protocol_expect is not None:
            continue
        spec = fields.get(name)
        if spec:
            value = cells[index[column_ref(spec)]]
            null_tokens = {t.upper() for t in (entry.get("null_tokens") or {}).get(name, [])} | {"?", "-", "--", "N/A", "NA", "NULL", "NONE"}
            text_expect[name] = None if not value or value.upper() in null_tokens else (value.casefold() if name == "protocol" else value)
    for name, spec in (("gateway_ip", fields.get("gateway_ip")), ("serving_node_ip", fields.get("serving_node_ip"))):
        if spec:
            value = "10.255.%d.%d" % (rng.randint(0, 9), rng.randint(1, 254))
            _put(cells, index, spec, value)
            text_expect[name] = value
    if "cell" in fields and isinstance(fields["cell"], dict) and fields["cell"].get("column"):
        raw_cell = cells[index[column_ref(fields["cell"]["column"])]]
        cell_expect["cell_raw"] = raw_cell or None
    flag_list = sorted(flags)
    record = {
        "msisdn": msisdn, "imsi": imsi, "imei": imei, "tac": imei[:8] if imei and len(imei) >= 14 else None,
        "ue_ip": ue_ip, "ue_port": ue_port, "public_ip": public_ip, "public_port_from": port_from, "public_port_to": port_to,
        "server_ip": None if defect == "unknown_label" else server_ip, "server_port": None if defect == "party_sentinel" else server_port,
        "protocol": protocol_expect, "protocol_num": protocol_num_expect, "session_ref": session_ref,
        "start_local": start.replace(microsecond=0).isoformat(sep=" ") if not hints.get("iso_millis") else start.isoformat(sep=" "),
        "end_local": (end.replace(microsecond=(end.microsecond // 1000) * 1000)).isoformat(sep=" ") if "end" in time_cols else None,
        "nat_start_local": start.isoformat(sep=" ") if "nat_start" in time_cols else None,
        "nat_end_local": ((nat_end or end).isoformat(sep=" ") if hints.get("nat_end_always") else (nat_end.isoformat(sep=" ") if nat_end else None))
        if "nat_end" in time_cols else None,
        **expect_ints, **cell_expect, **{k: v for k, v in text_expect.items() if k != "protocol" or protocol_expect is None},
    }
    # the generator writes whole seconds except where a layout carries milliseconds
    if not hints.get("iso_millis"):
        record["end_local"] = end.replace(microsecond=0).isoformat(sep=" ") if "end" in time_cols else None
        if "nat_end" in time_cols:
            record["nat_end_local"] = ((nat_end or end).replace(microsecond=0).isoformat(sep=" ") if hints.get("nat_end_always") else
                                      (nat_end.replace(microsecond=0).isoformat(sep=" ") if nat_end else None))
    expect = {"status": "accepted", "tag": defect or "normal", "flags": flag_list, "record": record}
    if defect == "time_unparsed":
        expect = {"status": "quarantined", "reason": "time_unparsed", "tag": defect}
    elif defect == "time_missing":
        expect = {"status": "quarantined", "reason": "time_missing", "tag": defect}
    return cells, expect


def _usage_row(i, ctx, defect):
    entry, hints, rng, index = ctx["entry"], ctx["hints"], ctx["rng"], ctx["index"]
    fields = entry.get("fields") or {}
    cells = [""] * ctx["width"]
    if defect == "blank":
        return cells, {"status": "skipped", "reason": "blank_row", "tag": "blank"}
    subject_n = (i % 4) + 1
    msisdn = party_forms(subject_n)["e164_12"]
    start = _times(i, ctx)
    end = start + datetime.timedelta(seconds=rng.randint(1, 3600))
    time_cols, fmt = ctx["time_columns"], ctx["fmt"]
    if defect == "time_unparsed":
        _put(cells, index, time_cols["start"], "not-a-time")
    elif defect == "time_missing":
        pass
    else:
        _put(cells, index, time_cols["start"], render_time(start, fmt, hints))
    _put(cells, index, time_cols["end"], render_time(end, fmt, hints))
    _put(cells, index, fields["msisdn"], msisdn)
    imsi = reserved.synthetic_imsi(subject_n)
    imei = imei_for(subject_n, int(hints.get("imei_digits", 15)))
    _put(cells, index, fields["imsi"], imsi)
    _put(cells, index, fields["imei"], imei)
    mapped_positions = {pos for pos, cell in enumerate(cells) if cell != ""}
    _fill_other_columns(cells, ctx, i, mapped_positions, {"start": start, "end": end})
    text = {}
    for name in ("terminal", "apn", "app_category", "app", "app_feature", "province", "city"):
        value = cells[index[column_ref(fields[name])]]
        text[name] = value or None
    record = {"msisdn": msisdn, "imsi": imsi, "imei": imei, "tac": imei[:8], "start_local": start.isoformat(sep=" "),
              "end_local": end.isoformat(sep=" "), **text}
    expect = {"status": "accepted", "tag": defect or "normal", "flags": [], "record": record}
    if defect == "time_unparsed":
        expect = {"status": "quarantined", "reason": "time_unparsed", "tag": defect}
    elif defect == "time_missing":
        expect = {"status": "quarantined", "reason": "time_missing", "tag": defect}
    return cells, expect


PLATE_FORMS = ("SYNT-%04d", "synt %04d", "SYNT%04d", "SYNT\u2013%04d", "SYNT-%s")
ARABIC_INDIC = "\u0660\u0661\u0662\u0663\u0664\u0665\u0666\u0667\u0668\u0669"


def _plate_row(i, ctx, defect):
    """One sighting. The plate is written in one of five presentations (case, separators, a typographic dash, Arabic-Indic digits) and the truth keeps what the file wrote and the search key."""
    entry, hints, rng, index = ctx["entry"], ctx["hints"], ctx["rng"], ctx["index"]
    fields = entry.get("fields") or {}
    cells = [""] * ctx["width"]
    if defect == "blank":
        return cells, {"status": "skipped", "reason": "blank_row", "tag": "blank"}
    n = i % 12 + 1
    form = PLATE_FORMS[i % len(PLATE_FORMS)]
    raw = form % ("".join(ARABIC_INDIC[int(d)] for d in "%04d" % n)) if form.endswith("%s") else form % n
    start = _times(i, ctx)
    time_cols, fmt = ctx["time_columns"], ctx["fmt"]
    if defect == "time_unparsed":
        _put(cells, index, time_cols["start"], "not-a-time")
    elif defect == "time_missing":
        pass
    else:
        _put(cells, index, time_cols["start"], render_time(start, fmt, hints))
    camera = i % 5 + 1
    lat, lon = round(31.50 + 0.012 * camera, 5), round(74.30 + 0.017 * camera, 5)
    confidence = round(0.80 + 0.01 * (i % 20), 2)
    lane = "Lane %d" % (i % 3 + 1)
    _put(cells, index, fields["plate"], raw)
    _put(cells, index, fields["camera_id"], "CAM-%02d" % camera)
    _put(cells, index, fields["camera_label"], "Camera %d" % camera)
    _put(cells, index, fields["lane"], lane)
    _put(cells, index, fields["lat"], lat)
    _put(cells, index, fields["lon"], lon)
    _put(cells, index, fields["confidence"], confidence)
    mapped_positions = {pos for pos, cell in enumerate(cells) if cell != ""}
    _fill_other_columns(cells, ctx, i, mapped_positions, {"start": start})
    plate_norm = "SYNT%04d" % n
    record = {"plate_raw": raw, "plate_norm": plate_norm, "plate_script": "latin", "camera_id": "CAM-%02d" % camera, "camera_label": "Camera %d" % camera, "lane": lane,
              "lat": lat, "lon": lon, "confidence": confidence, "source_kind": "csv", "loc_method": "source_supplied", "observed_local": start.isoformat(sep=" ")}
    expect = {"status": "accepted", "tag": defect or "normal", "flags": [], "record": record}
    if defect == "time_unparsed":
        expect = {"status": "quarantined", "reason": "time_unparsed", "tag": defect}
    elif defect == "time_missing":
        expect = {"status": "quarantined", "reason": "time_missing", "tag": defect}
    return cells, expect


def main(argv=None):
    ap = argparse.ArgumentParser(description="Write synthetic golden files for the layout registry.")
    ap.add_argument("--layout", help="a layout id (see --list)")
    ap.add_argument("--all", action="store_true")
    ap.add_argument("--list", action="store_true")
    ap.add_argument("--out", help="folder for the golden files")
    ap.add_argument("--rows", type=int)
    ap.add_argument("--seed", type=int, default=1)
    args = ap.parse_args(argv)
    registry = load_registry()
    if args.list:
        for layout_id in registry.ids():
            print(layout_id)
        return 0
    if not args.out or not (args.layout or args.all):
        ap.error("--out and either --layout or --all are required")
    targets = registry.ids() if args.all else [args.layout]
    for layout_id in targets:
        golden = generate(registry.get(layout_id), args.rows, args.seed)
        print("wrote", golden.write(args.out), "(%d rows)" % len(golden.rows))
    return 0


if __name__ == "__main__":
    sys.exit(main())
