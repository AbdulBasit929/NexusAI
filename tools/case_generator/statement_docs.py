"""Bank statements as PDF and Word files for a case that has transactions (reference cases programme, stage S4; the formats and their reading routes are in section 4 of the programme document).

    built = build(params, key)        # needs key["transaction_truth"]; returns {"files": {rel: bytes}, "statements": [entries], "truth": {...}}

Two accounts of the case's second-precision statement are written as multi-page PDF statements (Courier in columns, the table header repeated on every page, a page footer, an opening and a closing
balance, totals) and one as a Word table; each carries the same lines as the typed statement of that account, in the columns a Pakistani e-statement shows (date and time, particulars,
reference, withdrawals, deposits, balance; the account number is in the header block). A statement file does not carry the counterparty account or the channel: the truth of these files is the
subset of fields the format states. The issuing bank is fictional ("Specimen Bank Limited") and every file says SYNTHETIC. Own random stream; the default build is unchanged.
"""
import datetime
import os
import random
import sys
import textwrap
from decimal import Decimal

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "real_data"))
import docx_writer  # noqa: E402
import pdf_writer  # noqa: E402
import pk_locale  # noqa: E402

LAYOUT = "bank-stmt-p1"
HEADER = ["Date", "Particulars", "Ref No", "Withdrawals", "Deposits", "Balance"]
CHANNEL_WORD = {"ibft": "IBFT", "raast": "Raast", "atm": "ATM", "pos": "POS", "cheque": "Cheque", "cash": "Cash", "branch": "Branch", "online": "Online", "other": "Mystery", "wallet": "Online"}

# the page grid, in characters of the monospaced table (the advance of Courier is 0.6 of its size)
SIZE = 7.0
ADVANCE = SIZE * pdf_writer.MONO_ADVANCE
X0 = 40.0
COLS = {"date": (0, 19, "l"), "part": (21, 61, "l"), "ref": (63, 75, "l"), "wd": (77, 91, "r"), "dep": (93, 107, "r"), "bal": (109, 124, "r")}   # two blank characters between columns: the reader finds columns from the blank channels
LEAD = 10.0
FIRST_TOP, NEXT_TOP, LAST_TOP = 196.0, 70.0, 770.0


def money(value):
    return "{:,.2f}".format(Decimal(value))


def stamp(local):
    day = datetime.datetime.strptime(local, "%Y-%m-%d %H:%M:%S")
    return day.strftime("%d/%m/%Y %H:%M:%S")


def narration_of(params, row):
    if not params.locale:
        return "Synthetic narration"
    import random as _random
    return pk_locale.narration(_random.Random("narration:%s:%s" % (params.seed, row["key"])), CHANNEL_WORD.get(row["channel"], "IBFT"), row["direction"], row["counterparty"] or "")


def lines_of(params, rows):
    """The statement lines of an account, in file order, as the format shows them."""
    out = []
    for row in rows:
        out.append({"seq": row["seq"], "local": row["local"], "when": stamp(row["local"]), "particulars": narration_of(params, row), "reference": row["reference"] or "",
                    "direction": row["direction"], "amount": str(row["amount"]), "balance": str(row["balance_after"])})
    return out


def opening_of(lines):
    first = lines[0]
    amount, balance = Decimal(first["amount"]), Decimal(first["balance"])
    return balance + amount if first["direction"] == "debit" else balance - amount


def cell(text, col):
    start, end, align = COLS[col]
    width = end - start
    text = text[:width]
    return (X0 + (start + (width - len(text) if align == "r" else 0)) * ADVANCE, text)


def pdf_pages(account, lines, opening, closing, period, bank="SPECIMEN BANK LIMITED"):
    pages, items, top = [], [], FIRST_TOP
    total_wd = sum((Decimal(l["amount"]) for l in lines if l["direction"] == "debit"), Decimal("0"))
    total_dep = sum((Decimal(l["amount"]) for l in lines if l["direction"] == "credit"), Decimal("0"))

    def table_header(at):
        items.append(("rule", X0, at - 9, X0 + 124 * ADVANCE, at - 9, 0.5))
        for col, name in zip(COLS, HEADER):
            start, end, align = COLS[col]
            x = X0 + (start if align == "l" else end - len(name)) * ADVANCE
            items.append(("text", x, at, "mono-bold", SIZE, name))
        items.append(("rule", X0, at + 3, X0 + 124 * ADVANCE, at + 3, 0.5))
    items += [("text", X0, 52, "sans-bold", 14, bank), ("text", X0, 68, "sans", 8, "SYNTHETIC TEST STATEMENT - NOT A RECORD OF ANY REAL ACCOUNT"),
              ("text", X0, 96, "sans-bold", 11, "Statement of Account"),
              ("text", X0, 118, "sans-bold", 8, "Account No:"), ("text", X0 + 62, 118, "mono", 8, account),
              ("text", X0, 130, "sans-bold", 8, "Statement period:"), ("text", X0 + 62, 130, "sans", 8, "%s to %s" % period),
              ("text", X0, 142, "sans-bold", 8, "Currency:"), ("text", X0 + 62, 142, "sans", 8, "PKR"),
              ("text", X0, 154, "sans-bold", 8, "Opening Balance:"), ("text", X0 + 62, 154, "mono", 8, money(opening))]
    table_header(FIRST_TOP - 10)
    top = FIRST_TOP + 6

    def new_page():
        nonlocal items, top
        pages.append(items)
        items, top = [], NEXT_TOP
        table_header(top - 10)
        top += 6
    for line in lines:
        wrapped = textwrap.wrap(line["particulars"], COLS["part"][1] - COLS["part"][0]) or [""]
        if top + LEAD * len(wrapped) > LAST_TOP:
            new_page()
        wd, dep = (money(line["amount"]), "") if line["direction"] == "debit" else ("", money(line["amount"]))
        for pos, key, text in ((0, "date", line["when"]), (0, "part", wrapped[0]), (0, "ref", line["reference"]), (0, "wd", wd), (0, "dep", dep), (0, "bal", money(line["balance"]))):
            if text:
                x, shown = cell(text, key)
                items.append(("text", x, top, "mono", SIZE, shown))
        for extra in wrapped[1:]:
            top += LEAD
            x, shown = cell(extra, "part")
            items.append(("text", x, top, "mono", SIZE, shown))
        top += LEAD
    if top + 4 * LEAD > LAST_TOP:
        new_page()
    items.append(("rule", X0, top - 6, X0 + 124 * ADVANCE, top - 6, 0.5))
    top += 6
    for label, value in (("Total Withdrawals", money(total_wd)), ("Total Deposits", money(total_dep)), ("Closing Balance", money(closing))):
        x, shown = cell(label, "part")
        items.append(("text", x, top, "mono-bold", SIZE, shown))
        x, shown = cell(value, "bal")
        items.append(("text", x, top, "mono-bold", SIZE, shown))
        top += LEAD
    pages.append(items)
    count = len(pages)
    for number, page in enumerate(pages, start=1):
        page.append(("text", X0, 800, "sans", 7, "Page %d of %d" % (number, count)))
        page.append(("text", X0 + 300, 800, "sans", 7, "SYNTHETIC - issued by a bank that does not exist"))
    return pages


def docx_blocks(account, lines, opening, closing, period, bank="SPECIMEN BANK LIMITED"):
    rows = [HEADER] + [[l["when"], l["particulars"], l["reference"], money(l["amount"]) if l["direction"] == "debit" else "", money(l["amount"]) if l["direction"] == "credit" else "", money(l["balance"])] for l in lines]
    return [("p", bank, {"bold": True, "size": 28}), ("p", "SYNTHETIC TEST STATEMENT - NOT A RECORD OF ANY REAL ACCOUNT", {"size": 16}), ("p", "Statement of Account", {"bold": True, "size": 22}),
            ("p", "Account No: " + account, {}), ("p", "Statement period: %s to %s" % period, {}), ("p", "Currency: PKR", {}), ("p", "Opening Balance: " + money(opening), {}),
            ("table", rows, {}), ("p", "Closing Balance: " + money(closing), {"bold": True})]


def build(params, key):
    from txn_case import Oracle
    truth = key["transaction_truth"]
    oracle = Oracle(truth)
    rng = random.Random("statements:%d" % params.seed)
    # an account whose statement has a planted break (a row left out of the CSV) would not reconcile in a file that states every line
    dated = set(truth["plants"]["dated_accounts"]) | {truth["plants"].get("victim_account")}
    timed = [a for a in oracle.accounts() if a not in dated and a.startswith("PK00SYNT") and len([r for r in oracle.of(a) if r["file"] == "A"]) >= 20]
    timed.sort(key=lambda a: (-len(oracle.of(a)), a))
    chosen = timed[:3]   # the two longest accounts are the PDFs (multi-page), the third is the Word table
    files, entries, statements = {}, [], {}
    for slot, account in enumerate(chosen):
        rows = [r for r in oracle.of(account) if r["file"] == "A"]
        lines = lines_of(params, rows)
        opening, closing = opening_of(lines), Decimal(lines[-1]["balance"])
        period = (lines[0]["when"][:10], lines[-1]["when"][:10])
        pdf = slot < 2
        name = "statement-%s.%s" % (account[-4:], "pdf" if pdf else "docx")
        rel = "files/statements/" + name
        if pdf:
            pages = pdf_pages(account, lines, opening, closing, period)
            files[rel] = pdf_writer.build_pdf(pages, title="Statement of Account %s" % account)
            page_count = len(pages)
        else:
            files[rel] = docx_writer.build_docx(docx_blocks(account, lines, opening, closing, period), title="Statement of Account %s" % account)
            page_count = None
        entries.append({"name": name[:-4 if pdf else -5], "file": rel, "layout_id": LAYOUT, "format": "pdf" if pdf else "docx", "account": account})
        statements[name] = {"account": account, "format": "pdf" if pdf else "docx", "pages": page_count, "opening": str(opening), "closing": str(closing),
                            "total_withdrawals": str(sum((Decimal(l["amount"]) for l in lines if l["direction"] == "debit"), Decimal("0"))),
                            "total_deposits": str(sum((Decimal(l["amount"]) for l in lines if l["direction"] == "credit"), Decimal("0"))),
                            "lines": [{"seq": l["seq"], "when": l["when"], "local": l["local"], "particulars": l["particulars"], "reference": l["reference"] or None, "direction": l["direction"],
                                       "amount": l["amount"], "balance": l["balance"]} for l in lines]}
    return {"files": files, "statements": entries, "truth": {"layout": LAYOUT, "statements": statements}}


def write(built, root):
    for rel, data in built["files"].items():
        path = os.path.join(root, *rel.split("/"))
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "wb") as handle:
            handle.write(data)
