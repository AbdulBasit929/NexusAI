"""Readers of account statements that arrive as a PDF with a text layer or as a Word table (reference cases programme, stage S4; the formats and what each can claim are in section 4 of the programme document).

Both return what the other container readers return, `(meta, sheets)` with `sheets` a list of `(name, rows_callable)`: the first row of a sheet is the header, then one row per statement line, every cell a
string. The typed ingest then reads the rows like those of a CSV: a layout of the registry recognises the header and interprets the rows.

A statement file is read as a table with these columns (names as the file prints them): Date, Particulars (or Narration), Ref No, Withdrawals, Deposits, Balance. The account number is not in the table: it is
in the header block (`Account No: ...`), so the reader puts it in the first column of every row, as `Account No`, and returns, beside the rows, `meta["sources"]`: for a PDF the page and the vertical position (points from the top) of every line, in the order of the rows, so that a line can be shown at the place it
came from. The typed table does not store that place yet (a row's `source_row` is its position in the statement): the header is exactly the 11 columns of bank-stmt-t1, because extra columns lower the
header match below the layout's threshold.

PDF route. `pdftotext -bbox` (poppler; present in the worker image) writes every word with its box on the page. A first version of this reader read the text of `pdftotext -layout`; measured on
generated statements, that text puts the amounts of a row on the line above or below it, because poppler's layout analysis orders blocks, not lines, so the coordinates are used instead. Words are grouped
into rows by the vertical position of their box; the columns are found from the page itself: the horizontal extents of all the words of the table (the header row and the rows under it) are merged
where they are closer than about one and a half characters, which keeps the words of a narration together and leaves the wider space between two columns; every header word then belongs to the column
it falls in. A row whose first column is a date starts a line; a row with text only in the Particulars column continues the narration of the line above (a long narration wraps); "Closing Balance",
"Total Withdrawals" and "Total Deposits" rows are read into the metadata and are never lines; page footers and the letterhead are left out.

What this does NOT do (stated, so that nothing is claimed that is not built): it reads text that is in the file; a scanned page has none (the OCR route, stage S5); it does not judge whether a statement is
genuine, only whether its lines add up (`reconciliation` in the metadata: the opening balance and the lines against the closing balance and the totals, to the paisa).
"""
from __future__ import annotations

import re
import shutil
import subprocess
import zipfile
from decimal import Decimal, InvalidOperation
from xml.etree import ElementTree

from .readers import Unsupported

DATE = re.compile(r"^\d{2}/\d{2}/\d{4}(?: \d{2}:\d{2}(?::\d{2})?)?$")
ACCOUNT = re.compile(r"Account\s*(?:No|Number)\.?\s*:?\s*([A-Za-z0-9][A-Za-z0-9 \-]{6,40}?)\s*(?:$|\s{2,})", re.I)
OPENING = re.compile(r"Opening\s+Balance\s*:?\s*(-?[\d,]+\.\d{2})", re.I)
FOOTER = re.compile(r"^\s*Page\s+\d+\s+of\s+\d+|^\s*SYNTHETIC\b|^\s*$", re.I)
HEADER_WORDS = {"date", "particulars", "narration", "details", "ref", "reference", "withdrawals", "withdrawal", "debit", "deposits", "deposit", "credit", "balance", "no"}
OUT_HEADER = ["Account No", "Txn Date", "Value Date", "Narration", "Reference", "Debit", "Credit", "Balance", "Channel", "Counterparty Account", "Counterparty Name"]
# where each cell of the table goes in an output row: (index in a table row of the file) -> output column
def out_row(account, date, particulars, ref, withdrawals, deposits, balance):
    return [account, date, "", particulars, ref, withdrawals, deposits, balance, "", "", ""]

TOTALS = {"total withdrawals": "total_withdrawals", "total deposits": "total_deposits", "closing balance": "closing"}
MAX_PAGES = 400


def amount(text):
    text = (text or "").strip().replace(",", "")
    if not text:
        return None
    try:
        return Decimal(text)
    except InvalidOperation:
        return None


WORD = re.compile(r'<word xMin="([\d.\-]+)" yMin="([\d.\-]+)" xMax="([\d.\-]+)" yMax="([\d.\-]+)">(.*?)</word>', re.S)
PAGE = re.compile(r"<page\b")


def _canonical(name):
    lowered = " ".join(name.lower().replace("/", " ").split())
    if lowered == "date":
        return "Date"
    if lowered in ("particulars", "narration", "details", "description"):
        return "Particulars"
    if lowered in ("ref no", "ref", "reference", "reference no", "chq no"):
        return "Ref No"
    if lowered in ("withdrawals", "withdrawal", "debit", "debits", "dr"):
        return "Withdrawals"
    if lowered in ("deposits", "deposit", "credit", "credits", "cr"):
        return "Deposits"
    if lowered in ("balance", "running balance"):
        return "Balance"
    return None


def read_pdf_words(path):
    """The pages of a PDF as lists of words (x0, y0, x1, y1, text), from `pdftotext -bbox`."""
    import html
    if shutil.which("pdftotext") is None:
        raise Unsupported("pdftotext is not installed: the statement reader needs poppler")
    try:
        done = subprocess.run(["pdftotext", "-bbox", "-enc", "UTF-8", str(path), "-"], check=False, capture_output=True, timeout=120)
    except subprocess.TimeoutExpired:
        raise Unsupported("pdftotext took more than 120 seconds")
    if done.returncode != 0:
        raise Unsupported("pdftotext could not read the file: " + done.stderr.decode("utf-8", "replace")[:120])
    text = done.stdout.decode("utf-8", "replace")
    chunks = PAGE.split(text)[1:]
    if not chunks or len(chunks) > MAX_PAGES:
        raise Unsupported("a PDF with no text layer or more than %d pages" % MAX_PAGES)
    pages = [[(float(a), float(b), float(c), float(d), html.unescape(w)) for a, b, c, d, w in WORD.findall(chunk)] for chunk in chunks]
    if not any(pages):
        raise Unsupported("a PDF with no text layer (a scan: the OCR route)")
    return pages


def _rows_of(words):
    """Words grouped into rows by the vertical position of their box (a row is words whose middles are within a third of the box height of each other), each row sorted left to right."""
    rows = []
    for word in sorted(words, key=lambda w: ((w[1] + w[3]) / 2, w[0])):
        middle, height = (word[1] + word[3]) / 2, max(word[3] - word[1], 1.0)
        if rows and abs(middle - rows[-1]["middle"]) <= height / 3:
            rows[-1]["words"].append(word)
        else:
            rows.append({"middle": middle, "words": [word]})
    for row in rows:
        row["words"].sort(key=lambda w: w[0])
    return rows


def _text_of(row, charw):
    out, last = "", None
    for w in row["words"]:
        out += ("  " if last is not None and w[0] - last > 1.5 * charw else " " if last is not None else "") + w[4]
        last = w[2]
    return out


def _columns(rows, charw):
    """Column intervals: the horizontal extents of the words of the table merged where the gap between them is under one and a half characters."""
    extents = sorted((w[0], w[2]) for row in rows for w in row["words"])
    blocks = []
    for a, b in extents:
        if blocks and a - blocks[-1][1] < 1.5 * charw:
            blocks[-1][1] = max(blocks[-1][1], b)
        else:
            blocks.append([a, b])
    return blocks


def _cells(row, blocks):
    cells = [[] for _ in blocks]
    for w in row["words"]:
        middle = (w[0] + w[2]) / 2
        for i, (a, b) in enumerate(blocks):
            if a - 0.5 <= middle <= b + 0.5:
                cells[i].append(w[4])
                break
    return [" ".join(c) for c in cells]


def parse_pdf_pages(pages):
    """(rows, meta) from the pages of `pdftotext -bbox` words. `rows` is the table with its header; `meta` holds the account, the opening balance and the totals the file states."""
    meta = {"account": None, "opening": None, "pages": len(pages), "stated": {}, "format": "pdf", "sources": []}
    rows, columns, previous = [OUT_HEADER], None, None
    for page_number, words in enumerate(pages, start=1):
        widths = [(w[2] - w[0]) / max(len(w[4]), 1) for w in words if len(w[4]) >= 3]
        charw = sorted(widths)[len(widths) // 2] if widths else 4.0
        lines = _rows_of(words)
        texts = [_text_of(line, charw) for line in lines]
        header_at = next((i for i, text in enumerate(texts) if sum(1 for w in re.split(r"\s+", text.strip().lower()) if w in HEADER_WORDS) >= 4 and "balance" in text.lower()
                          and not text.lower().lstrip().startswith(("opening", "closing"))), None)
        for text in texts if header_at is None else texts[:header_at]:
            m = ACCOUNT.search(text + "  ")
            if m and not meta["account"]:
                meta["account"] = re.sub(r"\s+", "", m.group(1)).upper()
            m = OPENING.search(text)
            if m and meta["opening"] is None:
                meta["opening"] = amount(m.group(1))
        if header_at is None:
            continue
        table = [(line, text) for line, text in list(zip(lines, texts))[header_at:] if not FOOTER.search(text)]
        blocks = _columns([line for line, _ in table], charw)
        names = [_canonical(" ".join(c.split())) for c in _cells(table[0][0], blocks)]
        if columns is None:
            columns = names
            if sorted(n for n in names if n) != sorted(["Date", "Particulars", "Ref No", "Withdrawals", "Deposits", "Balance"]) or None in names:
                raise Unsupported("the table header of the statement is not Date, Particulars, Ref No, Withdrawals, Deposits, Balance (found %d columns: %s)" % (len(blocks), _cells(table[0][0], blocks)))
        elif names != columns:
            raise Unsupported("the table of page %d has other columns than the first page" % page_number)
        index = {name: i for i, name in enumerate(columns)}
        for line, text in table[1:]:
            fields = _cells(line, blocks)
            first = fields[index["Date"]]
            label = " ".join(fields[index["Particulars"]].lower().split())
            if label in TOTALS and not first:
                meta["stated"][TOTALS[label]] = amount(fields[index["Balance"]] or fields[index["Withdrawals"]] or fields[index["Deposits"]])
                previous = None
                continue
            at = str(int(round(line["middle"])))
            if DATE.match(first):
                row = out_row(meta["account"] or "", first, fields[index["Particulars"]], fields[index["Ref No"]], fields[index["Withdrawals"]], fields[index["Deposits"]], fields[index["Balance"]])
                rows.append(row)
                meta["sources"].append({"page": page_number, "y": int(at)})
                previous = row
            elif previous is not None and not first and fields[index["Particulars"]] and not any(fields[index[k]] for k in ("Ref No", "Withdrawals", "Deposits", "Balance")):
                previous[3] = (previous[3] + " " + fields[index["Particulars"]]).strip()
    return rows, meta


def reconcile(rows, meta):
    """The statement's own arithmetic, to the paisa: opening balance, then each line's balance from the one before, the totals and the closing balance the file states."""
    notes = {"lines": len(rows) - 1, "balance_breaks": 0, "opening_stated": meta.get("opening") is not None}
    balance = meta.get("opening")
    total_wd = total_dep = Decimal("0")
    for row in rows[1:]:
        wd, dep, bal = amount(row[5]), amount(row[6]), amount(row[7])
        total_wd += wd or Decimal("0")
        total_dep += dep or Decimal("0")
        if balance is not None and bal is not None and balance - (wd or Decimal("0")) + (dep or Decimal("0")) != bal:
            notes["balance_breaks"] += 1
        balance = bal if bal is not None else balance
    stated = meta.get("stated", {})
    notes["totals_agree"] = (("total_withdrawals" not in stated or stated["total_withdrawals"] == total_wd) and ("total_deposits" not in stated or stated["total_deposits"] == total_dep))
    notes["closing_agrees"] = "closing" not in stated or balance == stated["closing"]
    notes["reconciles"] = notes["balance_breaks"] == 0 and notes["totals_agree"] and notes["closing_agrees"] and notes["opening_stated"]
    return notes


def iter_pdf_sheets(path):
    pages = read_pdf_words(path)
    rows, meta = parse_pdf_pages(pages)
    if len(rows) < 2:
        raise Unsupported("no statement table was found in the text of the PDF")
    if not meta["account"]:
        raise Unsupported("the statement names no account (an Account No line is expected in the header block)")
    for row in rows[1:]:
        row[0] = meta["account"]
    meta["reconciliation"] = reconcile(rows, meta)
    return meta, [("statement", lambda: iter([list(r) for r in rows]))]


W = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"


def _paragraph_text(p):
    return "".join(t.text or "" for t in p.iter(W + "t")).strip()


def iter_docx_sheets(path):
    """The tables of a Word file that are statements (their first row is Date, Particulars, Ref No, Withdrawals, Deposits, Balance), one sheet each; the account is read from the paragraph that says it."""
    try:
        with zipfile.ZipFile(path) as z:
            info = z.getinfo("word/document.xml")
            if info.file_size > 16 * 1024 * 1024:
                raise Unsupported("word/document.xml is larger than the 16 MB bound")
            root = ElementTree.fromstring(z.read(info))
    except (KeyError, zipfile.BadZipFile, ElementTree.ParseError) as exc:
        raise Unsupported("not a readable Word file: %s" % exc)
    body = root.find(W + "body")
    account, opening, stated, sheets, meta = None, None, {}, [], {"account": None, "format": "docx", "stated": {}}
    for child in list(body):
        if child.tag == W + "p":
            text = _paragraph_text(child)
            m = ACCOUNT.search(text + "  ")
            if m and account is None:
                account = re.sub(r"\s+", "", m.group(1)).upper()
            m = OPENING.search(text)
            if m and opening is None:
                opening = amount(m.group(1))
            low = " ".join(text.lower().split())
            for label, key in TOTALS.items():
                if low.startswith(label):
                    value = amount(re.sub(r"^[^\d\-]*", "", text))
                    if value is not None:
                        stated[key] = value
        elif child.tag == W + "tbl":
            table = [[_paragraph_text(cell) for cell in row.findall(W + "tc")] for row in child.findall(W + "tr")]
            if not table or [_canonical(c) for c in table[0]] != ["Date", "Particulars", "Ref No", "Withdrawals", "Deposits", "Balance"]:
                continue
            rows = [OUT_HEADER] + [out_row(account or "", r[0], r[1], r[2], r[3], r[4], r[5]) for r in table[1:] if len(r) == 6 and DATE.match(r[0])]
            sheets.append(rows)
    if not sheets:
        raise Unsupported("no statement table was found in the Word file")
    if not account:
        raise Unsupported("the statement names no account (an Account No line is expected before the table)")
    meta.update({"account": account, "opening": opening, "stated": stated})
    meta["reconciliation"] = reconcile(sheets[0], meta)
    return meta, [("statement-%d" % (i + 1), (lambda rows=rows: iter([list(r) for r in rows]))) for i, rows in enumerate(sheets)]
