"""Bank statements and wallet lists through the typed ingest (docs/data/transactions-family-20261028.md, ingest side), tested without a database.

Plain unittest:  python -m unittest ingestion.forensic_records.tests.test_transactions_family
"""

from __future__ import annotations

import sys
import tempfile
import unittest
from decimal import Decimal
from pathlib import Path

from ingestion.forensic_records import typed_ingest as ti
from ingestion.forensic_records.layout_registry import Policy, interpret_rows, load_registry
from ingestion.forensic_records.layout_registry.parties import normalize_account, parse_amount

REPO = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(REPO / "tools" / "real_data"))
import make_golden  # noqa: E402
import reserved  # noqa: E402

REGISTRY = load_registry()
BANK = REGISTRY.get("bank-stmt-t1")
WALLET = REGISTRY.get("wallet-t1")
KARACHI = Policy(source_tz="Asia/Karachi", tz_basis="declared", date_order="DMY")
EVIDENCE = "5a000000-0000-4000-8000-0000000000a1"
BATCH = "6a000000-0000-4000-8000-0000000000b1"
LINEAGE = ti.Lineage("tenant-1", "case-1", EVIDENCE, "statement.csv", BATCH)
COLUMNS = ("tenant_id", "collection_id", "evidence_id", "file_id", "batch_id", "source_row", "record_id", "record_ts", "row_hash", "content_hash", "layout_id", "layout_version", "txn_utc",
           "time_basis", "time_precision", "source_tz", "account_raw", "account_norm", "counterparty_account_raw", "counterparty_account_norm", "counterparty_msisdn", "counterparty_label",
           "direction", "amount", "currency", "balance_after", "channel", "reference", "narration", "status", "raw_payload", "quality_flags", "ingested_at")
BANK_HEADER = [c["name"] for c in BANK["header"]["columns"]]
IBAN = reserved.synthetic_iban(1)
OTHER = reserved.synthetic_iban(5)


class Sink:
    def __init__(self):
        self.rows, self.keys = [], set()

    def table_columns(self, table):
        assert table == "bank_transaction"
        return [(c, "text") for c in COLUMNS]

    def insert_batch(self, table, columns, rows):
        new = 0
        for row in rows:
            key = tuple(row[c.strip()] for c in ti.conflict_key(table).split(","))
            if key not in self.keys:
                self.keys.add(key)
                self.rows.append(row)
                new += 1
        return new


def bank_row(when, debit="", credit="", balance="", channel="IBFT", account=IBAN, other=OTHER, narration="Synthetic narration", reference="R1"):
    return [account, when, "", narration, reference, debit, credit, balance, channel, other, ""]


def read(rows, entry=BANK, header=None, policy=KARACHI):
    return list(interpret_rows(entry, header or [c["name"] for c in entry["header"]["columns"]], rows, policy=policy))


def write_csv(directory, header, rows):
    path = Path(directory) / "statement.csv"
    path.write_text(",".join(header) + "\n" + "\n".join(",".join('"%s"' % c for c in r) for r in rows) + "\n", encoding="utf-8", newline="")
    return str(path)


class NormaliserTests(unittest.TestCase):
    def test_amounts_as_a_statement_writes_them(self):
        cases = {"1,234.50": (Decimal("1234.50"), None), "(1,234.50)": (Decimal("1234.50"), "debit"), "-500": (Decimal("500"), "debit"), "500-": (Decimal("500"), "debit"),
                 "250.00 Dr": (Decimal("250.00"), "debit"), "250 CR": (Decimal("250"), "credit"), "Rs. 1,00,000.00": (Decimal("100000.00"), None), "PKR 75": (Decimal("75"), None),
                 "0": (Decimal("0"), None)}
        for text, (value, sign) in cases.items():
            got = parse_amount(text)
            self.assertEqual((got[0], got[1]), (value, sign), text)
        self.assertEqual(parse_amount("1,50"), (Decimal("1.50"), None, ["amount_decimal_comma"]))
        self.assertEqual(parse_amount("abc"), (None, None, ["amount_not_numeric"]))
        self.assertEqual(parse_amount(""), (None, None, []))
        self.assertEqual(parse_amount("1,2,3x"), (None, None, ["amount_not_numeric"]))

    def test_accounts_are_a_search_key_that_never_makes_two_accounts_equal(self):
        self.assertEqual(normalize_account("pk00 synt-0000 0000 0000 0001"), IBAN)
        self.assertNotEqual(normalize_account("SYNT-000123"), normalize_account("SYNT-000124"))
        self.assertTrue(all(reserved.is_reserved_account(reserved.synthetic_iban(n)) and reserved.is_reserved_account(reserved.synthetic_account(n)) for n in range(20)))


class InterpreterTests(unittest.TestCase):
    def test_a_debit_and_a_credit_column_give_a_positive_amount_and_a_direction(self):
        out = read([bank_row("2026-03-05 10:11:12", debit="1,234.50", balance="98,765.50"), bank_row("2026-03-05 11:00:00", credit="500.00", balance="99,265.50")])
        first, second = out[0].record, out[1].record
        self.assertEqual((first["direction"], first["amount"], first["balance_after"]), ("debit", Decimal("1234.50"), Decimal("98765.50")))
        self.assertEqual((second["direction"], second["amount"]), ("credit", Decimal("500.00")))
        self.assertEqual((first["account_norm"], first["counterparty_account_norm"], first["channel"], first["time_precision"]), (IBAN, OTHER, "ibft", "second"))
        self.assertEqual(first["start_local"], "2026-03-05 10:11:12")

    def test_a_row_with_both_columns_or_neither_amount_is_refused_never_guessed(self):
        both = read([bank_row("2026-03-05 10:11:12", debit="10", credit="10")])[0]
        self.assertEqual((both.status, both.reason), ("quarantined", "amount_ambiguous"))
        none = read([bank_row("2026-03-05 10:11:12")])[0]
        self.assertEqual((none.status, none.reason), ("quarantined", "amount_unreadable"))
        junk = read([bank_row("2026-03-05 10:11:12", debit="lots")])[0]
        self.assertEqual((junk.status, junk.reason), ("quarantined", "amount_unreadable"))
        missing = read([bank_row("2026-03-05 10:11:12", debit="10", account="")])[0]
        self.assertEqual((missing.status, missing.reason), ("quarantined", "account_missing"))

    def test_a_date_without_a_time_is_kept_at_midnight_and_marked(self):
        record = read([bank_row("05/03/2026", debit="10")])[0].record
        self.assertEqual((record["time_precision"], record["start_local"]), ("date", "2026-03-05 00:00:00"))
        self.assertIn("date_only", record["quality_flags"])
        minute = read([bank_row("05/03/2026 10:11", debit="10")])[0].record
        self.assertEqual((minute["time_precision"], minute["start_local"]), ("minute", "2026-03-05 10:11:00"))
        self.assertEqual(read([bank_row("not-a-time", debit="10")])[0].reason, "time_unparsed")

    def test_the_channel_words_and_an_unknown_word(self):
        words = {"IBFT": "ibft", "Raast": "raast", "ATM": "atm", "POS": "pos", "Cheque": "cheque", "Cash": "cash", "Branch": "branch", "Online": "online"}
        for word, meaning in words.items():
            self.assertEqual(read([bank_row("2026-03-05 10:11:12", debit="1", channel=word)])[0].record["channel"], meaning, word)
        strange = read([bank_row("2026-03-05 10:11:12", debit="1", channel="Mystery")])[0]
        self.assertEqual(strange.record["channel"], "other")
        self.assertIn("unmapped_label:channel", strange.record["quality_flags"])

    def test_a_wallet_amount_takes_its_direction_from_the_type_and_a_type_it_does_not_know_is_refused(self):
        header = [c["name"] for c in WALLET["header"]["columns"]]

        def wallet(kind, amount="1,000.00", other=reserved.synthetic_msisdn(5)):
            return [reserved.synthetic_msisdn(1), "TXN00000001", "2026-03-05 10:11:12", kind, amount, "9,000.00", other, "Completed"]

        for kind, direction, channel in (("Sent", "debit", "wallet"), ("Received", "credit", "wallet"), ("Cash In", "credit", "cash"), ("Cash Out", "debit", "cash"), ("Bill Payment", "debit", "online")):
            record = read([wallet(kind)], WALLET, header)[0].record
            self.assertEqual((record["direction"], record["channel"], record["amount"]), (direction, channel, Decimal("1000.00")), kind)
        record = read([wallet("Sent")], WALLET, header)[0].record
        self.assertEqual((record["counterparty_msisdn"], record["reference"], record["account_norm"]), (reserved.synthetic_msisdn(5), "TXN00000001", reserved.synthetic_msisdn(1)))
        refused = read([wallet("Mystery")], WALLET, header)[0]
        self.assertEqual((refused.status, refused.reason), ("quarantined", "amount_direction_unknown"))
        self.assertEqual(read([wallet("Sent", amount="-50")], WALLET, header)[0].record["direction"], "debit", "a minus sign is a debit")


class WriterTests(unittest.TestCase):
    def test_the_table_and_its_key(self):
        self.assertEqual(ti.TABLE_FOR_FAMILY["transaction"], "bank_transaction")
        self.assertEqual(ti.conflict_key("bank_transaction"), "tenant_id, collection_id, file_id, row_hash, txn_utc")

    def test_a_running_balance_that_breaks_is_flagged_and_the_row_is_kept(self):
        rows = [bank_row("2026-03-05 09:00:00", credit="1,000.00", balance="11,000.00"),
                bank_row("2026-03-05 10:00:00", debit="200.00", balance="10,800.00"),
                bank_row("2026-03-05 11:00:00", debit="300.00", balance="10,000.00"),      # 10,800 - 300 = 10,500: a break
                bank_row("2026-03-05 12:00:00", credit="50.00", balance="10,050.00"),       # follows from the break row: fine
                bank_row("2026-03-05 13:00:00", credit="75.00", balance="500.00", account=OTHER)]   # the first row of another account is its start
        with tempfile.TemporaryDirectory() as directory:
            sink = Sink()
            out = ti.ingest_file(write_csv(directory, BANK_HEADER, rows), LINEAGE, KARACHI, sink, registry=REGISTRY)["sheets"][0]
            self.assertEqual((out["status"], out["loaded"], out["inserted"], out["table"], out["layout_id"]), ("loaded", 5, 5, "bank_transaction", "bank-stmt-t1"))
            broken = [r["source_row"] for r in sink.rows if "balance_break" in r["quality_flags"]]
            self.assertEqual(broken, [3])
            self.assertEqual(out["flags"].get("balance_break"), 1)

    def test_an_exact_repeat_is_flagged_once_and_does_not_break_the_balance(self):
        first = bank_row("2026-03-05 09:00:00", credit="1,000.00", balance="11,000.00")
        rows = [first, list(first), bank_row("2026-03-05 10:00:00", debit="200.00", balance="10,800.00")]
        with tempfile.TemporaryDirectory() as directory:
            sink = Sink()
            out = ti.ingest_file(write_csv(directory, BANK_HEADER, rows), LINEAGE, KARACHI, sink, registry=REGISTRY)["sheets"][0]
            self.assertEqual((out["loaded"], out["duplicates_in_file"], out["flags"].get("duplicate_row"), out["flags"].get("balance_break")), (3, 1, 1, None))
            again = ti.ingest_file(write_csv(directory, BANK_HEADER, rows), LINEAGE, KARACHI, sink, registry=REGISTRY)["sheets"][0]
            self.assertEqual((again["inserted"], again["already_present"]), (0, 3))

    def test_a_row_without_a_zone_is_not_written_and_none_is_guessed(self):
        with tempfile.TemporaryDirectory() as directory:
            sink = Sink()
            out = ti.ingest_file(write_csv(directory, BANK_HEADER, [bank_row("2026-03-05 09:00:00", credit="10")]), LINEAGE, Policy(), sink, registry=REGISTRY)["sheets"][0]
            self.assertEqual((out["loaded"], out["quarantined"]), (0, {"quarantined:source_timezone_unknown": 1}))

    def test_the_golden_files_are_written_once_and_every_row_follows_the_balance_before_it(self):
        for entry in (BANK, WALLET):
            with tempfile.TemporaryDirectory() as directory:
                path = make_golden.generate(entry, rows=60, seed=3, defects=False).write(directory)
                sink = Sink()
                out = ti.ingest_file(path, LINEAGE, KARACHI, sink, registry=REGISTRY)["sheets"][0]
                self.assertEqual((out["status"], out["loaded"], out["inserted"]), ("loaded", 60, 60), entry["layout_id"])
                self.assertIsNone(out["flags"].get("balance_break"), entry["layout_id"])
                self.assertTrue(all(r["time_precision"] == "second" and r["amount"] > 0 for r in sink.rows))


if __name__ == "__main__":
    unittest.main()
