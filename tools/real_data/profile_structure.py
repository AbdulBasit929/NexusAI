#!/usr/bin/env python3
"""Structure-only profile of a tabular file: layout, formats, patterns, number classes and vocabularies, never the data.

    python -I -X utf8 tools/real_data/profile_structure.py FILE --out REPORT.json [--mode shareable|local] [--sheet NAME]
                                                                [--all-sheets] [--max-rows N] [--allow-unsafe-location]

Reads CSV, TSV and other delimited text (UTF-8, UTF-8 with BOM, UTF-16), XLSX, and SpreadsheetML 2003 (often saved as .xls).
Legacy binary .xls, archives and databases are refused with an instruction (convert inside the vault).

What the report contains (see docs/data/real-data-handling-policy.md, section 4):

* shareable mode (default): container facts, header names that look like identifiers, per-column fill rate, value *patterns*
  (digits become 9, letters a or A, runs counted), length range, number classes, time formats, magnitude classes for counts, and
  the vocabulary of columns on a short allow-list of enumerations (direction, type, protocol ...). No value of any other column,
  no exact time range, no exact counts. Safe to read in a chat or paste into a design note after you have looked at it.
* local mode: adds exact row and distinct counts, time minimum and maximum (as written in the file, no time zone), numeric ranges
  of measure columns and the file's SHA-256. Keep it in the vault; it is what reconcile_aggregates.py compares against.

The report is the only thing a model-assisted workflow may read. The file itself is never pasted, printed or uploaded.
"""
import argparse
import collections
import csv
import datetime
import hashlib
import json
import os
import re
import sys
import zipfile
import xml.etree.ElementTree as ET

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))  # python -I does not add the script folder itself
from guard import location_problems, require_safe_location  # noqa: E402

TOOL_VERSION = "1"
MAX_PATTERNS = 300
MAX_UNCOMPRESSED = 4 * 1024 ** 3
MAX_RATIO = 400

SS_MAIN = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
SS_REL = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
SS_2003 = "urn:schemas-microsoft-com:office:spreadsheet"

# Columns whose values are a small closed vocabulary rather than data. Anything not matched here never has a value reported.
CATEGORICAL_ALLOW = re.compile(
    r"(^|[^a-z])(call.?type|event.?type|record.?type|direction|dir|inbound.?outbound|in.?out|protocol|proto|rat|network.?type|"
    r"service.?type|bearer|status|category|sub.?category|app.?type|apn|termination|release.?cause)([^a-z]|$)", re.I)
MEASURE_HINT = re.compile(r"(duration|dur|bytes|volume|packets|size|count|port|octets|seconds|secs)", re.I)
TIME_HINT = re.compile(r"(time|date|_dt|_tm|stamp|start|end|begin|first|last|accessed|created|logged)", re.I)
NUMBER_HINT = re.compile(r"(msisdn|a.?party|b.?party|a.?number|b.?number|call.?org|call.?dialed|dialed|originat|calling|called|"
                         r"imei|imsi|cell|lac|tac|sender|receiver|number)", re.I)
SAFE_VOCAB_VALUE = re.compile(r"^[A-Za-z][A-Za-z0-9 _/.\-]{0,30}$")

_ASCII_MASK = {}
for _i in range(128):
    _c = chr(_i)
    _ASCII_MASK[_i] = "9" if _c.isdigit() else ("A" if _c.isupper() else "a") if _c.isalpha() else " " if _c.isspace() else _c
_RUNS = re.compile(r"(9{5,}|a{5,}|A{5,}|u{5,})")


# ----------------------------------------------------------------------------------------------------------------------------
# Masking and classification (pure functions, covered by the tests)
# ----------------------------------------------------------------------------------------------------------------------------
def mask(text, limit=48):
    """Shape of a value: digits -> 9, ASCII letters -> a or A, other letters -> u, runs of five or more counted."""
    text = str(text)
    if text.isascii():
        shape = text.translate(_ASCII_MASK)
    else:
        shape = "".join(
            "9" if ch.isdigit() else ("A" if ch.isupper() else "a") if (ch.isalpha() and ch.isascii()) else "u" if ch.isalpha()
            else " " if ch.isspace() else ch for ch in text)
    shape = _RUNS.sub(lambda m: "%s{%d}" % (m.group(0)[0], len(m.group(0))), shape)
    return shape[:limit]


def looks_like_header_cell(cell):
    cell = str(cell).strip()
    if not cell or len(cell) > 60:
        return False
    if not any(ch.isalpha() for ch in cell):
        return False
    if re.search(r"\d{4,}", cell):
        return False
    return True


def header_name(cell):
    """Header text verbatim if it is identifier-like, otherwise a mask (headers can carry values in preamble-style files)."""
    cell = str(cell).strip()
    if looks_like_header_cell(cell) and not re.search(r"[:=]\s*\S", cell):
        return cell
    return "<masked:%s>" % mask(cell, 40)


def value_class(value):
    """Class of a number-like value, for the histogram of identifier formats. Never returns the value."""
    v = str(value).strip()
    if not v:
        return "empty"
    if re.fullmatch(r"\d+\.\d+", v):
        return "excel_float"
    if re.fullmatch(r"\d{1,3}(\.\d{1,3}){3}", v):
        return "ipv4_text"
    if re.search(r"[A-Za-z\u0080-￿]", v):
        return "alnum_sender_id" if re.search(r"\d", v) else "alpha_label"
    digits = re.sub(r"\D", "", v)
    if v.startswith("+"):
        return "plus_%d_digits" % len(digits)
    if digits.startswith("0092"):
        return "0092_prefix_%d_digits" % len(digits)
    if digits.startswith("92") and len(digits) == 12:
        return "92_prefix_12_digits"
    if digits.startswith("03") and len(digits) == 11:
        return "03_prefix_11_digits"
    if digits.startswith("3") and len(digits) == 10:
        return "3_prefix_10_digits"
    if len(digits) <= 6:
        return "short_code_up_to_6"
    return "other_%d_digits" % len(digits)


def magnitude_class(n):
    """Order of magnitude of a count, so a shareable report does not carry exact numbers."""
    if n == 0:
        return "0"
    if n < 10:
        return "1-9"
    exp = len(str(int(n))) - 1
    return "10^%d..10^%d" % (exp, exp + 1)


TEXT_TIME_FORMATS = ("iso_with_offset", "iso_naive", "iso_date", "dmy_or_mdy_12h", "dmy_or_mdy_24h", "dmy_or_mdy_date", "day_monthname_year")


def time_format_of(value, numeric_ok):
    """Name of the time format a value is written in, or None. Epoch and Excel serials are numbers: only trusted when the
    header says the column is a time (numeric_ok), because a ten-digit phone number is also ten digits."""
    v = str(value).strip()
    if not v:
        return None
    if numeric_ok:
        if re.fullmatch(r"\d{5}(\.\d+)?", v):
            return "excel_serial"
        if re.fullmatch(r"\d{10}", v):
            return "epoch_seconds"
        if re.fullmatch(r"\d{13}", v):
            return "epoch_millis"
    if re.fullmatch(r"\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:?\d{2})?", v):
        return "iso_with_offset" if re.search(r"(Z|[+-]\d{2}:?\d{2})$", v) else "iso_naive"
    if re.fullmatch(r"\d{4}-\d{2}-\d{2}", v):
        return "iso_date"
    if re.fullmatch(r"\d{1,2}[/.\-]\d{1,2}[/.\-]\d{4}( \d{1,2}:\d{2}(:\d{2})?( ?[AaPp][Mm])?)?", v):
        return "dmy_or_mdy_" + ("12h" if re.search(r"[AaPp][Mm]$", v) else "24h" if ":" in v else "date")
    if re.fullmatch(r"\d{1,2}[\- ][A-Za-z]{3}[\- ]\d{2,4}( \d{1,2}:\d{2}(:\d{2})?)?", v):
        return "day_monthname_year"
    return None


def parse_naive(value, fmt, day_first):
    """Parse to a naive datetime for min and max. Returns None when the value does not fit the stated format."""
    v = str(value).strip()
    try:
        if fmt == "excel_serial":
            return datetime.datetime(1899, 12, 30) + datetime.timedelta(days=float(v))
        if fmt == "epoch_seconds":
            return datetime.datetime(1970, 1, 1) + datetime.timedelta(seconds=int(v))
        if fmt == "epoch_millis":
            return datetime.datetime(1970, 1, 1) + datetime.timedelta(milliseconds=int(v))
        if fmt in ("iso_naive", "iso_with_offset"):
            core = re.sub(r"(Z|[+-]\d{2}:?\d{2})$", "", v).replace("T", " ")
            return datetime.datetime.strptime(core[:26], "%Y-%m-%d %H:%M:%S.%f" if "." in core else "%Y-%m-%d %H:%M:%S")
        if fmt == "iso_date":
            return datetime.datetime.strptime(v, "%Y-%m-%d")
        if fmt.startswith("dmy_or_mdy"):
            m = re.fullmatch(r"(\d{1,2})[/.\-](\d{1,2})[/.\-](\d{4})(?: (\d{1,2}):(\d{2})(?::(\d{2}))?(?: ?([AaPp][Mm]))?)?", v)
            if not m or day_first is None:
                return None
            a, b, y = int(m.group(1)), int(m.group(2)), int(m.group(3))
            day, month = (a, b) if day_first else (b, a)
            hour, minute, second = int(m.group(4) or 0), int(m.group(5) or 0), int(m.group(6) or 0)
            if m.group(7):
                hour = hour % 12 + (12 if m.group(7).lower() == "pm" else 0)
            return datetime.datetime(y, month, day, hour, minute, second)
    except (ValueError, OverflowError):
        return None
    return None


# ----------------------------------------------------------------------------------------------------------------------------
# Readers (streaming)
# ----------------------------------------------------------------------------------------------------------------------------
class Unsupported(Exception):
    pass


def sniff_kind(path):
    with open(path, "rb") as handle:
        head = handle.read(4096)
    if head[:4] == b"\xd0\xcf\x11\xe0":
        raise Unsupported("legacy binary .xls (OLE): save it as .xlsx or .csv inside the vault and profile that copy")
    if head[:2] == b"PK":
        try:
            with zipfile.ZipFile(path) as z:
                names = z.namelist()
        except zipfile.BadZipFile:
            raise Unsupported("a zip container that cannot be opened")
        if "xl/workbook.xml" in names:
            return "xlsx"
        raise Unsupported("an archive: extract it inside the vault, then profile each member")
    if head[:7] == b"Rar!\x1a\x07\x00" or head[:6] == b"7z\xbc\xaf\x27\x1c":
        raise Unsupported("an archive: extract it inside the vault, then profile each member")
    if head.lstrip(b"\xef\xbb\xbf").lstrip()[:5] == b"<?xml":
        if SS_2003.encode() in head:
            return "spreadsheetml"
        raise Unsupported("an XML file that is not SpreadsheetML 2003")
    if head.startswith(b"SQLite format 3"):
        raise Unsupported("a SQLite database")
    return "text"


def reject_entities(raw):
    """Refuse XML with a DOCTYPE or entity declarations: spreadsheets never need them, and entity expansion is a bomb."""
    if b"<!DOCTYPE" in raw[:65536] or b"<!ENTITY" in raw[:65536]:
        raise Unsupported("XML with a DOCTYPE or entity declarations is refused")


def decode_text_stream(path):
    with open(path, "rb") as handle:
        head = handle.read(4)
    if head[:2] in (b"\xff\xfe", b"\xfe\xff"):
        return "utf-16"
    if head[:3] == b"\xef\xbb\xbf":
        return "utf-8-sig"
    return "utf-8"


def choose_delimiter(sample_lines):
    best, best_score = ",", -1.0
    lines = [ln for ln in sample_lines if ln.strip()][:25]
    if not lines:
        return best
    for delim in [",", "\t", ";", "|"]:
        counts = [ln.count(delim) for ln in lines]
        common = collections.Counter(counts).most_common(1)[0]
        score = common[0] * (common[1] / len(counts)) if common[0] else 0.0
        if score > best_score:
            best, best_score = delim, score
    return best


def iter_text_rows(path):
    encoding = decode_text_stream(path)
    try:
        with open(path, "r", encoding=encoding, errors="strict", newline="") as probe:
            sample = [probe.readline() for _ in range(40)]
    except UnicodeDecodeError:
        encoding = "latin-1"
        with open(path, "r", encoding=encoding, newline="") as probe:
            sample = [probe.readline() for _ in range(40)]
    delimiter = choose_delimiter(sample)
    csv.field_size_limit(10 * 1024 * 1024)
    meta = {"encoding": encoding, "delimiter": {",": "comma", "\t": "tab", ";": "semicolon", "|": "pipe"}[delimiter]}

    def rows():
        with open(path, "r", encoding=encoding, errors="replace", newline="") as handle:
            for row in csv.reader(handle, delimiter=delimiter):
                yield row

    return meta, [("text", rows)]


def col_index(ref):
    n = 0
    for ch in re.match(r"[A-Z]+", ref).group(0):
        n = n * 26 + (ord(ch) - 64)
    return n - 1


def check_zip(z):
    total = sum(i.file_size for i in z.infolist())
    packed = max(1, sum(i.compress_size for i in z.infolist()))
    if total > MAX_UNCOMPRESSED or total / packed > MAX_RATIO:
        raise Unsupported("a workbook whose uncompressed size or compression ratio is outside the safety limits")


def iter_xlsx_sheets(path):
    z = zipfile.ZipFile(path)
    check_zip(z)
    for member in z.namelist():
        if member.endswith(".xml"):
            with z.open(member) as handle:
                reject_entities(handle.read(65536))
    shared = []
    if "xl/sharedStrings.xml" in z.namelist():
        with z.open("xl/sharedStrings.xml") as handle:
            current = []
            for event, el in ET.iterparse(handle, events=("end",)):
                if el.tag == "{%s}t" % SS_MAIN:
                    current.append(el.text or "")
                elif el.tag == "{%s}si" % SS_MAIN:
                    shared.append("".join(current))
                    current = []
                    el.clear()
    wb = ET.fromstring(z.read("xl/workbook.xml"))
    rels = {}
    if "xl/_rels/workbook.xml.rels" in z.namelist():
        for rel in ET.fromstring(z.read("xl/_rels/workbook.xml.rels")):
            rels[rel.get("Id")] = rel.get("Target")
    sheets = []
    for s in wb.find("{%s}sheets" % SS_MAIN):
        target = rels.get(s.get("{%s}id" % SS_REL), "")
        member = target.lstrip("/") if target.startswith("/") else "xl/" + target
        sheets.append((s.get("name"), member))

    def make(member):
        def rows():
            with z.open(member) as handle:
                context = ET.iterparse(handle, events=("start", "end"))
                root = None
                for event, el in context:
                    if root is None:
                        root = el
                    if event != "end" or el.tag != "{%s}row" % SS_MAIN:
                        continue
                    cells = {}
                    for c in el.findall("{%s}c" % SS_MAIN):
                        t = c.get("t")
                        v = c.find("{%s}v" % SS_MAIN)
                        if t == "s" and v is not None:
                            val = shared[int(v.text)]
                        elif t == "inlineStr":
                            val = "".join(x.text or "" for x in c.iter("{%s}t" % SS_MAIN))
                        else:
                            val = v.text if v is not None and v.text is not None else ""
                        cells[col_index(c.get("r"))] = val
                    yield [cells.get(i, "") for i in range(max(cells) + 1)] if cells else []
                    root.clear()
        return rows

    return {"encoding": None, "delimiter": None}, [(name, make(member)) for name, member in sheets]


def iter_spreadsheetml_sheets(path):
    with open(path, "rb") as handle:
        reject_entities(handle.read(65536))
    names = []
    for event, el in ET.iterparse(path, events=("start",)):
        if el.tag == "{%s}Worksheet" % SS_2003:
            names.append(el.get("{%s}Name" % SS_2003) or "sheet%d" % (len(names) + 1))

    def make(index):
        def rows():
            seen = -1
            root = None
            for event, el in ET.iterparse(path, events=("start", "end")):
                if root is None:
                    root = el
                if event == "start" and el.tag == "{%s}Worksheet" % SS_2003:
                    seen += 1
                if event == "end" and el.tag == "{%s}Row" % SS_2003:
                    if seen == index:
                        vals = []
                        for c in el.findall("{%s}Cell" % SS_2003):
                            idx = c.get("{%s}Index" % SS_2003)
                            if idx:
                                while len(vals) < int(idx) - 1:
                                    vals.append("")
                            d = c.find("{%s}Data" % SS_2003)
                            vals.append("".join(d.itertext()) if d is not None else "")
                        yield vals
                    el.clear()
        return rows

    return {"encoding": "xml", "delimiter": None}, [(n, make(i)) for i, n in enumerate(names)]


# ----------------------------------------------------------------------------------------------------------------------------
# Profiling
# ----------------------------------------------------------------------------------------------------------------------------
def find_header(head_rows):
    for i, row in enumerate(head_rows[:40]):
        cells = [str(c).strip() for c in row if str(c).strip()]
        if len(cells) < 4:
            continue
        good = [c for c in cells if looks_like_header_cell(c)]
        if len(good) < max(4, int(len(cells) * 0.8)):
            continue
        nxt = head_rows[i + 1] if i + 1 < len(head_rows) else []
        if len([c for c in nxt if str(c).strip()]) >= max(2, len(cells) // 2):
            return i
    return None


class Column:
    def __init__(self, name, max_distinct):
        self.name = name
        self.max_distinct = max_distinct
        self.rows = 0
        self.nonempty = 0
        self.patterns = collections.Counter()
        self.pattern_overflow = 0
        self.min_len = None
        self.max_len = 0
        self.distinct = set()
        self.distinct_capped = False
        self.classes = collections.Counter()
        self.time_formats = collections.Counter()
        self.first_part_max = 0
        self.second_part_max = 0
        self.tmin = {}
        self.tmax = {}
        self.tbad = collections.Counter()
        self.vocab = collections.Counter()
        self.vocab_unsafe = False
        self.num_min = None
        self.num_max = None
        self.numeric_seen = 0
        self.time_like = bool(TIME_HINT.search(name))
        self.number_like = bool(NUMBER_HINT.search(name))
        self.vocab_allowed = bool(CATEGORICAL_ALLOW.search(name))
        self.measure_like = bool(MEASURE_HINT.search(name))

    def add(self, raw):
        self.rows += 1
        value = "" if raw is None else str(raw).strip()
        if not value:
            return
        self.nonempty += 1
        shape = mask(value)
        if shape in self.patterns or len(self.patterns) < MAX_PATTERNS:
            self.patterns[shape] += 1
        else:
            self.pattern_overflow += 1
        n = len(value)
        self.min_len = n if self.min_len is None else min(self.min_len, n)
        self.max_len = max(self.max_len, n)
        if not self.distinct_capped:
            self.distinct.add(hash(value))
            if len(self.distinct) >= self.max_distinct:
                self.distinct_capped = True
        if self.number_like or self.nonempty <= 200:
            self.classes[value_class(value)] += 1
        if self.time_like or self.nonempty <= 200:
            fmt = time_format_of(value, self.time_like)
            if fmt:
                self.time_formats[fmt] += 1
                if fmt.startswith("dmy_or_mdy"):
                    m = re.match(r"(\d{1,2})[/.\-](\d{1,2})", value)
                    if m:
                        self.first_part_max = max(self.first_part_max, int(m.group(1)))
                        self.second_part_max = max(self.second_part_max, int(m.group(2)))
                    interpretations = (True, False)
                else:
                    interpretations = (None,)
                for interp in interpretations:
                    parsed = parse_naive(value, fmt, interp)
                    key = (fmt, interp)
                    if parsed is None:
                        self.tbad[key] += 1
                    else:
                        if key not in self.tmin or parsed < self.tmin[key]:
                            self.tmin[key] = parsed
                        if key not in self.tmax or parsed > self.tmax[key]:
                            self.tmax[key] = parsed
        if self.vocab_allowed and not self.vocab_unsafe:
            if SAFE_VOCAB_VALUE.match(value) and not re.search(r"\d{4,}", value):
                self.vocab[value] += 1
                if len(self.vocab) > 25:
                    self.vocab_unsafe = True
            else:
                self.vocab_unsafe = True
        if self.measure_like:
            try:
                f = float(value)
            except ValueError:
                pass
            else:
                self.numeric_seen += 1
                self.num_min = f if self.num_min is None else min(self.num_min, f)
                self.num_max = f if self.num_max is None else max(self.num_max, f)

    def day_first(self):
        if self.first_part_max > 12 and self.second_part_max <= 12:
            return True
        if self.second_part_max > 12 and self.first_part_max <= 12:
            return False
        return None

    def time_summary(self):
        """Dominant time format with min and max, or only the format when day and month order cannot be told apart."""
        if not self.time_formats:
            return None
        fmt = self.time_formats.most_common(1)[0][0]
        interp = self.day_first() if fmt.startswith("dmy_or_mdy") else None
        out = {"day_month_order": {True: "day_first", False: "month_first", None: "not_applicable"}[interp]
               if fmt.startswith("dmy_or_mdy") and interp is not None else
               ("ambiguous" if fmt.startswith("dmy_or_mdy") else "not_applicable")}
        key = (fmt, interp)
        if key in self.tmin:
            out["min"], out["max"] = self.tmin[key].isoformat(sep=" "), self.tmax[key].isoformat(sep=" ")
            out["span_days"] = (self.tmax[key] - self.tmin[key]).days
            out["unparsed"] = self.tbad[key]
        return out

    def report(self, mode):
        entry = {
            "header": header_name(self.name),
            "nonempty_share": round(self.nonempty / max(1, self.rows), 3),
            "length_range": [self.min_len or 0, self.max_len],
            "top_patterns": [[p, round(c / max(1, self.nonempty), 3)] for p, c in self.patterns.most_common(8)],
        }
        if self.pattern_overflow:
            entry["pattern_overflow_share"] = round(self.pattern_overflow / max(1, self.nonempty), 3)
        distinct = len(self.distinct)
        if mode == "local":
            entry["nonempty"] = self.nonempty
            entry["distinct"] = distinct
            entry["distinct_is_lower_bound"] = self.distinct_capped
        else:
            entry["distinct_class"] = (">=" if self.distinct_capped else "") + magnitude_class(distinct)
        sampled = min(self.nonempty, 200)
        if self.number_like and self.classes:
            entry["value_classes"] = {k: (v if mode == "local" else round(v / max(1, self.nonempty), 3))
                                      for k, v in self.classes.most_common(10)}
        time_share = sum(self.time_formats.values()) / max(1, sampled if not self.time_like else self.nonempty)
        if self.time_formats and (self.time_like or time_share >= 0.9):
            summary = self.time_summary() or {}
            entry["time"] = {"formats": {k: round(v / max(1, sampled if not self.time_like else self.nonempty), 3)
                                         for k, v in self.time_formats.most_common(4)},
                             "day_month_order": summary.get("day_month_order", "not_applicable")}
            if "span_days" in summary:
                entry["time"]["span_days"] = summary["span_days"] if mode == "local" else int(round(summary["span_days"], -1))
                entry["time"]["unparsed"] = summary["unparsed"]
            if mode == "local" and "min" in summary:
                entry["time"]["min"], entry["time"]["max"] = summary["min"], summary["max"]
        if self.vocab_allowed and self.vocab and not self.vocab_unsafe:
            entry["vocabulary"] = {k: (v if mode == "local" else round(v / max(1, self.nonempty), 3))
                                   for k, v in self.vocab.most_common(25)}
        if mode == "local" and self.measure_like and self.numeric_seen:
            entry["numeric_range"] = [self.num_min, self.num_max]
        return entry


def safe_sheet_name(name):
    pattern = r"(?i)(sheet|table|data|report|result|cdr|ipdr|basic information)\s*\d*"
    return name if re.fullmatch(pattern, str(name).strip()) else "<masked:%s>" % mask(name, 24)


def profile_rows(row_iter, mode, max_rows, max_distinct):
    head = []
    iterator = iter(row_iter)
    for row in iterator:
        head.append(row)
        if len(head) >= 60:
            break
    header_at = find_header(head)
    preamble = []
    footer = collections.deque(maxlen=4)
    columns = []
    state = {"rows": 0, "blank": 0}
    truncated = False
    width_hist = collections.Counter()

    def feed(row):
        if not any(str(c).strip() for c in row):
            state["blank"] += 1
            return
        state["rows"] += 1
        width_hist[len([c for c in row if str(c).strip()])] += 1
        for i in range(len(columns), len(row)):
            columns.append(Column("col%d" % (i + 1), max_distinct))
        for i, col in enumerate(columns):
            col.add(row[i] if i < len(row) else "")
        footer.append(row)

    if header_at is None:
        for row in head:
            feed(row)
    else:
        for row in head[:header_at]:
            cells = [c for c in row if str(c).strip()]
            if cells:
                preamble.append({"cells": len(cells), "masked": " | ".join(mask(c, 32) for c in cells)[:200]})
        columns.extend(Column(str(c).strip() or "col%d" % (i + 1), max_distinct) for i, c in enumerate(head[header_at]))
        for row in head[header_at + 1:]:
            feed(row)
    for row in iterator:
        if max_rows and state["rows"] >= max_rows:
            truncated = True
            break
        feed(row)

    rows_total = state["rows"]
    ragged = sum(n for w, n in width_hist.items() if columns and w not in (0, len(columns)))
    result = {
        "header_row_index": header_at,
        "preamble_rows": preamble[:30],
        "columns": [c.report(mode) for c in columns],
        "row_count_class": magnitude_class(rows_total),
        "truncated": truncated,
        "footer_masked": [" | ".join(mask(c, 32) for c in r if str(c).strip())[:160] for r in list(footer)[-2:]
                          if sum(1 for c in r if str(c).strip()) <= max(2, len(columns) // 3)],
    }
    if mode == "local":
        result["row_count"] = rows_total
        result["blank_rows"] = state["blank"]
        result["ragged_rows"] = ragged
    else:
        result["ragged_rows_class"] = magnitude_class(ragged)
    return result


def size_class(n):
    for limit, label in ((1024 ** 2, "<1 MB"), (10 * 1024 ** 2, "1-10 MB"), (100 * 1024 ** 2, "10-100 MB"), (1024 ** 3, "100 MB-1 GB")):
        if n < limit:
            return label
    return ">=1 GB"


def build_report(path, mode, sheet, all_sheets, max_rows, max_distinct):
    kind = sniff_kind(path)
    if kind == "xlsx":
        meta, sheets = iter_xlsx_sheets(path)
    elif kind == "spreadsheetml":
        meta, sheets = iter_spreadsheetml_sheets(path)
    else:
        meta, sheets = iter_text_rows(path)
    size = os.path.getsize(path)
    report = {
        "tool": "profile_structure", "tool_version": TOOL_VERSION, "mode": mode,
        "generated_utc": datetime.datetime.utcnow().strftime("%Y-%m-%dT%H:%M:%SZ"),
        "file": {"kind": kind, "encoding": meta["encoding"], "delimiter": meta["delimiter"],
                 "extension": os.path.splitext(path)[1].lower(), "size_class": size_class(size), "sheet_count": len(sheets)},
        "sheets": [],
    }
    if mode == "local":
        sha = hashlib.sha256()
        with open(path, "rb") as handle:
            for chunk in iter(lambda: handle.read(1 << 20), b""):
                sha.update(chunk)
        report["file"]["sha256"] = sha.hexdigest()
        report["file"]["size_bytes"] = size
    chosen = sheets if all_sheets else [s for s in sheets if sheet is None or s[0] == sheet]
    profiles = []
    for name, rows in chosen:
        p = profile_rows(rows(), mode, max_rows, max_distinct)
        p["name"] = safe_sheet_name(name)
        profiles.append(p)
    if not all_sheets and sheet is None and len(profiles) > 1:
        profiles.sort(key=lambda p: -len(p["columns"]))  # default: the sheet with the most columns
        profiles = profiles[:1]
    report["sheets"] = profiles
    return report


def main(argv=None):
    ap = argparse.ArgumentParser(description="Structure-only profile of a tabular file (no data values).")
    ap.add_argument("file")
    ap.add_argument("--out", required=True, help="report path (JSON); not inside a git work tree or a cloud-synced folder")
    ap.add_argument("--mode", choices=["shareable", "local"], default="shareable")
    ap.add_argument("--sheet")
    ap.add_argument("--all-sheets", action="store_true")
    ap.add_argument("--max-rows", type=int, default=5000000)
    ap.add_argument("--max-distinct", type=int, default=200000, help="per-column cap on exact distinct counting (lower bound beyond it)")
    ap.add_argument("--allow-unsafe-location", action="store_true",
                    help="only for synthetic test files: skip the vault location checks")
    args = ap.parse_args(argv)

    require_safe_location(args.file, args.allow_unsafe_location, "input file")
    out_problems = [p for p in location_problems(args.out) if args.mode == "local" or "git work tree" in p or "cloud" in p]
    if out_problems and not args.allow_unsafe_location:
        raise SystemExit("refusing to write the report to %r: %s" % (os.path.basename(args.out), "; ".join(out_problems)))
    try:
        report = build_report(args.file, args.mode, args.sheet, args.all_sheets, args.max_rows, args.max_distinct)
    except Unsupported as exc:
        raise SystemExit("cannot profile this file: %s" % exc)
    with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
        json.dump(report, handle, indent=2, ensure_ascii=True)
        handle.write("\n")
    sheet0 = report["sheets"][0] if report["sheets"] else {"columns": [], "row_count_class": "0"}
    print("wrote %s: kind=%s mode=%s sheets=%d columns=%d rows~%s" % (
        os.path.basename(args.out), report["file"]["kind"], args.mode, len(report["sheets"]), len(sheet0["columns"]),
        sheet0["row_count_class"]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
