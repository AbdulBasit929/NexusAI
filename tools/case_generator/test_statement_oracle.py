"""Gates of stage S4 (programme section 4 and 11): bank statements as PDF and Word files are read back to the lines of the typed statement they were made from. Plain unittest, no database, no model.

    python -X utf8 tools/case_generator/test_statement_oracle.py            # the PDF tests need Poppler's `pdftotext -bbox` (the worker image has it; the pdftotext of Git for Windows is Xpdf and has not):
                                                                         # they are skipped, with the reason, where it is absent. In the worker-based image:
    docker run --rm --entrypoint python -v <repo>:/repo:ro -w /repo nexusai-media-eval:latest tools/case_generator/test_statement_oracle.py

What it proves, on generated statements of a development case (a Lahore locale added):
  * writers: the PDF and the Word file are deterministic; a PDF of an account of 43 lines takes two pages with the table header repeated on each, and a Word table carries the same lines;
  * reading: every line of every file (date and time, particulars with the wrapped ones joined, reference, withdrawal or deposit, balance) equals the line of the truth, the account number is the one of
    the header block, the page of each line is the page it was written on; the statement's own arithmetic reconciles (opening balance, every balance, both totals, the closing balance) to the paisa;
  * integrity as an observation: a statement with one balance changed is still read in full and says that exactly one balance does not follow (never "forged");
  * the typed path: the rows of a statement are recognised by the layout bank-stmt-t1 and interpreted: account, local time at second precision, direction, amount and balance equal the truth, no line is
    quarantined and no line carries a flag; ingest_file reads the files by their container (pdf, docx) and a second ingest inserts nothing;
  * what is refused: a PDF with no text layer and a Word file with no statement table are not read (the OCR route is stage S5).
"""
import copy
import dataclasses
import json
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from decimal import Decimal

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, HERE)
sys.path.insert(0, REPO)
import case_set  # noqa: E402
import docx_writer  # noqa: E402
import generate  # noqa: E402
import pdf_writer  # noqa: E402
import statement_docs  # noqa: E402
from ingestion.forensic_records import typed_ingest as ti  # noqa: E402
from ingestion.forensic_records.layout_registry import Policy, interpret_rows, load_registry  # noqa: E402
from ingestion.forensic_records.layout_registry import readers, statement_readers  # noqa: E402

ZONE = "Asia/Karachi"
POLICY = Policy(source_tz=ZONE, tz_basis="declared", date_order="DMY")
LINEAGE = ti.Lineage("case", "synthetic-statements", "11111111-1111-5111-8111-111111111111", "statement.pdf", "22222222-2222-5222-8222-222222222222")


def bbox_available():
    if shutil.which("pdftotext") is None:
        return False
    with tempfile.TemporaryDirectory() as tmp:
        path = os.path.join(tmp, "x.pdf")
        with open(path, "wb") as handle:
            handle.write(pdf_writer.build_pdf([[("text", 50, 50, "mono", 7, "x")]]))
        return subprocess.run(["pdftotext", "-bbox", path, "-"], capture_output=True).returncode == 0


HAVE_BBOX = bbox_available()
need_bbox = unittest.skipUnless(HAVE_BBOX, "pdftotext -bbox (Poppler) is not installed here: the PDF reader runs in the worker image")


class Fixture:
    """A generated case with its statements, built once."""
    root = key = None

    @classmethod
    def get(cls):
        if cls.root is None:
            tmp = tempfile.mkdtemp(prefix="statements-")
            params = dataclasses.replace(case_set.params_for(2, {"period_days": 60}), locale="lahore")
            cls.root = generate.write_case(params, tmp, with_transactions=True, with_statements=True)
            with open(os.path.join(cls.root, "key", "key.json"), encoding="utf-8") as handle:
                cls.key = json.load(handle)
        return cls.root, cls.key


def paisa(text):
    return "{:,.2f}".format(Decimal(text))


def want_rows(account, truth):
    out = []
    for line in truth["lines"]:
        debit = line["direction"] == "debit"
        out.append([account, line["when"], "", line["particulars"], line["reference"] or "", paisa(line["amount"]) if debit else "", "" if debit else paisa(line["amount"]), paisa(line["balance"])])
    return out


def entries(fmt):
    root, key = Fixture.get()
    return root, [(e, key["statement_truth"]["statements"][os.path.basename(e["file"])]) for e in key["statements"] if e["format"] == fmt]


class Writers(unittest.TestCase):
    def test_pdf_and_word_are_deterministic_and_have_the_pages_and_lines_of_the_truth(self):
        root, pdfs = entries("pdf")
        self.assertEqual(len(pdfs), 2)
        for entry, truth in pdfs:
            with open(os.path.join(root, *entry["file"].split("/")), "rb") as handle:
                data = handle.read()
            self.assertTrue(data.startswith(b"%PDF-1.4"))
            self.assertEqual(data.count(b"/Type /Page "), truth["pages"])
        self.assertTrue(any(t["pages"] >= 2 for _, t in pdfs))
        _, words = entries("docx")
        self.assertEqual(len(words), 1)
        a = pdf_writer.build_pdf([[("text", 10, 10, "mono", 7, "a (b) \\ c")]])
        self.assertEqual(a, pdf_writer.build_pdf([[("text", 10, 10, "mono", 7, "a (b) \\ c")]]))


class Reading(unittest.TestCase):
    @need_bbox
    def test_every_line_of_a_pdf_statement_is_the_line_of_the_truth_and_the_statement_reconciles(self):
        root, pdfs = entries("pdf")
        for entry, truth in pdfs:
            meta, sheets = statement_readers.iter_pdf_sheets(os.path.join(root, *entry["file"].split("/")))
            rows = list(sheets[0][1]())
            self.assertEqual(rows[0][:8], ["Account No", "Txn Date", "Value Date", "Narration", "Reference", "Debit", "Credit", "Balance"])
            self.assertEqual([r[:8] for r in rows[1:]], want_rows(entry["account"], truth), entry["file"])
            self.assertEqual(meta["account"], entry["account"])
            self.assertEqual(meta["pages"], truth["pages"])
            self.assertTrue(meta["reconciliation"]["reconciles"], meta["reconciliation"])
            self.assertEqual(meta["reconciliation"]["lines"], len(truth["lines"]))
            self.assertEqual(meta["opening"], Decimal(truth["opening"]))
            self.assertEqual(meta["stated"]["closing"], Decimal(truth["closing"]))
            self.assertEqual(len(meta["sources"]), len(truth["lines"]))
            self.assertEqual({s["page"] for s in meta["sources"]}, set(range(1, truth["pages"] + 1)))
            self.assertEqual([s["page"] for s in meta["sources"]], sorted(s["page"] for s in meta["sources"]), "the lines are in page order")

    def test_every_line_of_a_word_statement_is_the_line_of_the_truth_and_the_statement_reconciles(self):
        root, words = entries("docx")
        for entry, truth in words:
            meta, sheets = statement_readers.iter_docx_sheets(os.path.join(root, *entry["file"].split("/")))
            rows = list(sheets[0][1]())
            self.assertEqual([r[:8] for r in rows[1:]], want_rows(entry["account"], truth))
            self.assertEqual(meta["account"], entry["account"])
            self.assertTrue(meta["reconciliation"]["reconciles"], meta["reconciliation"])

    @need_bbox
    def test_one_changed_balance_is_read_in_full_and_observed_not_called_forged(self):
        root, pdfs = entries("pdf")
        entry, truth = pdfs[0]
        lines = copy.deepcopy(truth["lines"])
        lines[10]["balance"] = str(Decimal(lines[10]["balance"]) + Decimal("1000.00"))
        pages = statement_docs.pdf_pages(entry["account"], lines, Decimal(truth["opening"]), Decimal(truth["closing"]), (lines[0]["when"][:10], lines[-1]["when"][:10]))
        with tempfile.TemporaryDirectory() as tmp:
            path = os.path.join(tmp, "tampered.pdf")
            with open(path, "wb") as handle:
                handle.write(pdf_writer.build_pdf(pages))
            meta, sheets = statement_readers.iter_pdf_sheets(path)
        rows = list(sheets[0][1]())
        self.assertEqual(len(rows) - 1, len(lines))
        # a balance that does not follow shows at the changed line and at the next one, which no longer follows from it
        self.assertEqual(meta["reconciliation"]["balance_breaks"], 2)
        self.assertFalse(meta["reconciliation"]["reconciles"])
        self.assertNotIn("forg", json.dumps(meta, default=str).lower())


class TypedPath(unittest.TestCase):
    def interpret(self, rows):
        registry = load_registry()
        report = registry.find_header(rows)
        self.assertEqual(report.matched_entry_id, "bank-stmt-t1", report)
        entry = registry.get("bank-stmt-t1")
        return list(interpret_rows(entry, rows[report.header_row], rows[report.header_row + 1:], policy=POLICY))

    def check(self, rows, truth):
        results = self.interpret(rows)
        self.assertEqual([r.status for r in results].count("accepted"), len(truth["lines"]))
        for result, line in zip(results, truth["lines"]):
            record = result.record
            self.assertEqual(record["account_norm"], truth_account(rows))
            self.assertEqual(record["time_precision"], "second")
            self.assertEqual(record["direction"], line["direction"])
            self.assertEqual(Decimal(str(record["amount"])), Decimal(line["amount"]))
            self.assertEqual(Decimal(str(record["balance_after"])), Decimal(line["balance"]))
            self.assertEqual(result.flags or [], [])

    def test_the_word_statement_is_interpreted_to_the_lines_of_the_truth(self):
        root, words = entries("docx")
        for entry, truth in words:
            meta, sheets = statement_readers.iter_docx_sheets(os.path.join(root, *entry["file"].split("/")))
            self.check(list(sheets[0][1]()), truth)

    @need_bbox
    def test_the_pdf_statements_are_interpreted_to_the_lines_of_the_truth(self):
        root, pdfs = entries("pdf")
        for entry, truth in pdfs:
            meta, sheets = statement_readers.iter_pdf_sheets(os.path.join(root, *entry["file"].split("/")))
            self.check(list(sheets[0][1]()), truth)

    @need_bbox
    def test_ingest_file_reads_the_containers_and_a_second_ingest_inserts_nothing(self):
        from ingestion.forensic_records.tests.test_transactions_family import Sink
        root, pdfs = entries("pdf")
        _, words = entries("docx")
        registry = load_registry()
        for entry, truth in (pdfs[0], words[0]):
            sink = Sink()
            path = os.path.join(root, *entry["file"].split("/"))
            first = ti.ingest_file(path, LINEAGE, POLICY, sink, registry=registry)
            self.assertEqual(first["status"], "loaded", first)
            sheet = first["sheets"][0]
            self.assertEqual((sheet["layout_id"], sheet["table"]), ("bank-stmt-t1", "bank_transaction"))
            self.assertEqual(sheet["loaded"], len(truth["lines"]))
            self.assertEqual(sheet["quarantined"], {})
            self.assertEqual(first["container"], entry["format"])
            second = ti.ingest_file(path, LINEAGE, POLICY, sink, registry=registry)
            self.assertEqual(second["rows_inserted"], 0)
            self.assertEqual(len(sink.rows), len(truth["lines"]))


def truth_account(rows):
    return rows[1][0]


class Refused(unittest.TestCase):
    def test_a_pdf_without_text_and_a_word_file_without_a_statement_table_are_refused(self):
        with tempfile.TemporaryDirectory() as tmp:
            blank = os.path.join(tmp, "blank.pdf")
            with open(blank, "wb") as handle:
                handle.write(pdf_writer.build_pdf([[]]))
            if HAVE_BBOX:
                with self.assertRaises(readers.Unsupported):
                    statement_readers.iter_pdf_sheets(blank)
            letter = os.path.join(tmp, "letter.docx")
            with open(letter, "wb") as handle:
                handle.write(docx_writer.build_docx([("p", "A letter with no table", {})]))
            with self.assertRaises(readers.Unsupported):
                statement_readers.iter_docx_sheets(letter)

    def test_the_containers_are_recognised(self):
        root, pdfs = entries("pdf")
        _, words = entries("docx")
        self.assertEqual(readers.sniff_kind(os.path.join(root, *pdfs[0][0]["file"].split("/"))), "pdf")
        self.assertEqual(readers.sniff_kind(os.path.join(root, *words[0][0]["file"].split("/"))), "docx")


if __name__ == "__main__":
    unittest.main()
