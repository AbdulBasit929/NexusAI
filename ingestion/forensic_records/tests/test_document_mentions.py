import json
import unittest
from types import SimpleNamespace

from ingestion.forensic_records import document_mentions
from ingestion.forensic_records.mention_extractor import EXTRACTOR_ID, EXTRACTOR_REVISION

PLACES = {"model town": ["Model Town"]}


def passage(text, number, artifact="11111111-1111-5111-8111-11111111111%d"):
    return SimpleNamespace(text=text, artifact_id=artifact % number, locator={"evidence_id": "e", "paragraph": number, "passage": 1}, text_sha256="x" * 64)


class Rows(unittest.TestCase):
    def test_a_national_id_number_is_counted_with_no_key_and_no_text(self):
        rows = document_mentions.mention_rows([passage("Accused CNIC 00000-1234567-1, mobile 0300-0001000, seen in Model Town.", 1)], PLACES)
        by_kind = {r["kind"]: r for r in rows}
        self.assertEqual(sorted(by_kind), ["msisdn", "national_id", "place"])
        self.assertIsNone(by_kind["national_id"]["value"])
        self.assertIsNone(by_kind["national_id"]["surface"])
        self.assertEqual(by_kind["msisdn"]["value"], "923000001000")
        self.assertEqual(by_kind["msisdn"]["surface"], "0300-0001000")
        self.assertEqual(by_kind["place"]["value"], "model town")
        self.assertNotIn("1234567", json.dumps(rows))

    def test_rows_carry_the_passage_locator_the_rule_the_tier_and_the_extractor(self):
        (row,) = document_mentions.mention_rows([passage("Mobile 0300-0001000", 3)])
        self.assertEqual(row["locator"]["paragraph"], 3)
        self.assertEqual((row["tier"], row["rule"]), ("high", "mobile-national"))
        self.assertEqual((row["extractor"], row["extractor_revision"]), (EXTRACTOR_ID, EXTRACTOR_REVISION))
        self.assertEqual(row["artifact_id"], "11111111-1111-5111-8111-111111111113")
        self.assertEqual((row["char_start"], row["char_end"]), (7, 19))

    def test_the_neighbouring_cell_is_the_context_of_a_cell_that_holds_only_digits(self):
        rows = document_mentions.mention_rows([passage("Mobile handset", 5), passage("000000000010009", 6)])
        self.assertEqual([(r["kind"], r["value"]) for r in rows], [("imei", "000000000010009")])

    def test_counts_by_kind_never_name_a_key(self):
        rows = document_mentions.mention_rows([passage("0300-0001000 and 0300-0001001, CNIC 00000-1234567-1", 1)])
        self.assertEqual(document_mentions.summarise(rows), {"msisdn": 2, "national_id": 1})


class Sink(unittest.TestCase):
    class Cursor:
        def __init__(self):
            self.calls = []

        def execute(self, sql, params):
            self.calls.append((sql, params))

    def test_writes_every_row_with_one_idempotent_statement(self):
        rows = document_mentions.mention_rows([passage("0300-0001000 and 0300-0001001", 1)])
        cursor = self.Cursor()
        written = document_mentions.CursorSink(cursor).write("case", "collection-1", "22222222-2222-5222-8222-222222222222", rows)
        self.assertEqual(written, 2)
        for sql, params in cursor.calls:
            self.assertIs(sql, document_mentions.INSERT_SQL)
            self.assertIn("ON CONFLICT (tenant_id, collection_id, artifact_id, kind, char_start, char_end, extractor_revision) DO NOTHING", sql)
            self.assertEqual(sql.count("%s"), len(params))
            self.assertEqual(params[:3], ("case", "collection-1", "22222222-2222-5222-8222-222222222222"))
            self.assertEqual(json.loads(params[11])["paragraph"], 1)

    def test_nothing_is_written_for_a_document_that_names_nothing(self):
        cursor = self.Cursor()
        self.assertEqual(document_mentions.CursorSink(cursor).write("case", "c", "e", document_mentions.mention_rows([passage("Routine patrol, Section 161 PPC 420, FIR 45/26.", 1)])), 0)
        self.assertEqual(cursor.calls, [])


if __name__ == "__main__":
    unittest.main()
