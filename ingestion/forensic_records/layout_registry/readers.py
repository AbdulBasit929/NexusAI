"""Row readers for provider exports: the rows of every sheet of an xlsx workbook, a SpreadsheetML 2003 file, or a delimited text file, as lists of cell text.

One implementation for the worker (typed ingest, `typed_ingest.py`) and the vault tools (`tools/real_data/profile_structure.py` imports from here), so that a
file is read the same way when it is profiled, when its layout is confirmed and when it is ingested. Standard library only. Streaming: a reader returns
`(meta, [(sheet name, rows)])` where `rows` is a function that yields the rows lazily. Archives, legacy .xls, DOCTYPE or entity declarations, and workbooks
outside the size and compression-ratio limits are refused with `Unsupported`.
"""
import collections
import csv
import re
import zipfile
import xml.etree.ElementTree as ET

MAX_UNCOMPRESSED = 4 * 1024 ** 3
MAX_RATIO = 400

SS_MAIN = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
SS_REL = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
SS_2003 = "urn:schemas-microsoft-com:office:spreadsheet"


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
        if "word/document.xml" in names:
            return "docx"   # a Word file: read as a statement table by statement_readers (stage S4)
        raise Unsupported("an archive: extract it inside the vault, then profile each member")
    if head[:5] == b"%PDF-":
        return "pdf"   # a PDF with a text layer: read as a statement table by statement_readers (stage S4); a scan has no text and is the OCR route
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


def reject_entities_in_file(path, chunk=1 << 20):
    """The same refusal for a whole file, read in chunks: a DOCTYPE may follow a long comment, so the first 64 KB are not enough for a file that arrives from outside."""
    keep = b""
    with open(path, "rb") as handle:
        while True:
            block = handle.read(chunk)
            if not block:
                return
            window = keep + block
            if b"<!DOCTYPE" in window or b"<!ENTITY" in window:
                raise Unsupported("XML with a DOCTYPE or entity declarations is refused")
            keep = window[-16:]


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
    reject_entities_in_file(path)
    names = []
    with open(path, "rb") as handle:
        for event, el in ET.iterparse(handle, events=("start",)):
            if el.tag == "{%s}Worksheet" % SS_2003:
                names.append(el.get("{%s}Name" % SS_2003) or "sheet%d" % (len(names) + 1))

    def make(index):
        def rows():
            seen = -1
            root = None
            with open(path, "rb") as handle:
                for event, el in ET.iterparse(handle, events=("start", "end")):
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
