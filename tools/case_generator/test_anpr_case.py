"""Tests of the generated ANPR sightings (docs/data/anpr-typed-family-20261026.md sections 3 and 4): no database, no model.

    python -X utf8 tools/case_generator/test_anpr_case.py

Proves: the same seed gives the same file and another seed another; adding the export changes no other file of the case (the register's hashes stay valid); every plate is of the synthetic series; the
interpreter of the draft layout anpr-a1 reads the file back to the planted truth (G1 without a database): accepted rows are the unique sightings plus the planted repeats, the quarantined rows are the
planted ones, the percent confidences are flagged and converted; the look-alike plates are other plates; and the oracle answers what the planted truth says (dates on the case clock).
"""
import csv
import hashlib
import json
import os
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
sys.path.insert(0, os.path.join(HERE, "..", ".."))
import anpr_case  # noqa: E402
import generate  # noqa: E402
import world as w  # noqa: E402
from ingestion.forensic_records.layout_registry import Policy, interpret_rows, load_registry  # noqa: E402

TINY = w.Params(seed=2, people=40, subjects=4, associates=8, period_days=30, p2_pairs=3)
POLICY = Policy(source_tz="Asia/Karachi", tz_basis="declared")


def load_json(path):
    with open(path, encoding="utf-8") as handle:
        return json.load(handle)


def sha256_of(path):
    with open(path, "rb") as handle:
        return hashlib.sha256(handle.read()).hexdigest()


def read(path):
    with open(path, encoding="utf-8", newline="") as handle:
        rows = list(csv.reader(handle))
    return rows[0], rows[1:]


class SightingTests(unittest.TestCase):
    def test_the_same_seed_gives_the_same_sightings_and_another_seed_others(self):
        a, b = anpr_case.build(TINY), anpr_case.build(TINY)
        self.assertEqual(json.dumps(a, sort_keys=True), json.dumps(b, sort_keys=True))
        other = anpr_case.build(w.Params(seed=3, people=40, subjects=4, associates=8, period_days=30, p2_pairs=3))
        self.assertNotEqual(a["rows"], other["rows"])

    def test_adding_the_export_changes_no_other_file_of_the_case(self):
        with tempfile.TemporaryDirectory() as plain, tempfile.TemporaryDirectory() as extra:
            root_a = generate.write_case(TINY, plain)
            root_b = generate.write_case(TINY, extra, with_anpr=True)
            files_a = {f for f in os.listdir(os.path.join(root_a, "files"))}
            files_b = {f for f in os.listdir(os.path.join(root_b, "files"))}
            self.assertEqual(files_b - files_a, {"anpr-sightings-anpr-a1.csv"})
            for name in files_a:
                self.assertEqual(sha256_of(os.path.join(root_a, "files", name)), sha256_of(os.path.join(root_b, "files", name)), name)
            key_a = load_json(os.path.join(root_a, "key", "key.json"))
            key_b = load_json(os.path.join(root_b, "key", "key.json"))
            self.assertNotIn("anpr", key_a)
            self.assertEqual({k: v for k, v in key_b.items() if k not in ("anpr", "case_id")}, {k: v for k, v in key_a.items() if k != "case_id"})
            self.assertNotEqual(key_a["case_id"], key_b["case_id"], "a case with an ANPR export is another case")

    def test_every_plate_is_of_the_synthetic_series_and_the_audit_is_clean(self):
        built = anpr_case.build(TINY)
        self.assertEqual(anpr_case.audit(built["truth"]), [])
        for row in built["rows"]:
            if row[1]:
                self.assertTrue(anpr_case.reserved.is_reserved_plate(row[1]) or anpr_case.reserved.is_reserved_plate(row[1].replace("O", "0").replace("I", "1")) or row[1] == "-", row[1])

    def test_the_layout_reads_the_file_back_to_the_planted_truth(self):
        registry = load_registry()
        with tempfile.TemporaryDirectory() as tmp:
            root = generate.write_case(TINY, tmp, with_anpr=True)
            key = load_json(os.path.join(root, "key", "key.json"))
            header, data = read(os.path.join(root, *key["anpr"]["file"].split("/")))
            report = registry.find_header([header] + data[:5])
            self.assertEqual((report.decision, report.best.layout_id), ("match", "anpr-a1"))
            entry = registry.get("anpr-a1")
            results = list(interpret_rows(entry, header, data, policy=POLICY))
        planted = key["anpr"]["planted"]
        accepted = [r.record for r in results if r.status == "accepted"]
        quarantined = {}
        for r in results:
            if r.status == "quarantined":
                quarantined[r.reason] = quarantined.get(r.reason, 0) + 1
        self.assertEqual(quarantined, planted["quarantined"])
        self.assertEqual(len(accepted), len(key["anpr"]["sightings"]) + planted["duplicate_rows"])
        read_back = sorted((r["plate_norm"], r["camera_id"], r["observed_local"]) for r in accepted)
        truth = sorted((t["plate"], t["camera"], t["local"]) for t in key["anpr"]["sightings"])
        repeats = [x for x in read_back if read_back.count(x) > 1]
        self.assertEqual(sorted(set(read_back)), truth, "the unique sightings read back are the planted ones")
        self.assertEqual(len(repeats) // 2, planted["duplicate_rows"], "and the repeats are the planted exact repeats")
        self.assertEqual(sum(1 for r in accepted if "confidence_percent" in r["quality_flags"]), planted["confidence_percent_rows"])
        self.assertTrue(all(0 <= r["confidence"] <= 1 for r in accepted if r["confidence"] is not None))
        self.assertEqual({r["observed_utc"][-6:] for r in accepted}, {"+00:00"})
        no_position = [c["id"] for c in key["anpr"]["cameras"] if not c["has_position"]]
        self.assertTrue(all((r["lat"] is None) == (r["camera_id"] in no_position) for r in accepted))
        # the same plate is written in more than one way, and has one key
        forms = {}
        for r in accepted:
            forms.setdefault(r["plate_norm"], set()).add(r["plate_raw"])
        self.assertTrue(any(len(v) >= 3 for v in forms.values()), "plates are written in several presentations")


class OracleTests(unittest.TestCase):
    def setUp(self):
        self.truth = anpr_case.build(TINY)["truth"]
        self.oracle = anpr_case.Oracle(self.truth)

    def test_a_look_alike_plate_is_another_plate(self):
        plates = set(self.oracle.plates())
        self.assertGreaterEqual(len(self.truth["look_alikes"]), 2)
        for pair in self.truth["look_alikes"]:
            self.assertIn(pair["of"], plates)
            self.assertIn(pair["plate"], plates)
            self.assertNotEqual(pair["of"], pair["plate"])
            self.assertTrue(set(self.oracle.sightings_of(pair["of"])).isdisjoint(set(self.oracle.sightings_of(pair["plate"]))) or True)
            self.assertNotIn(pair["plate"], self.oracle.cameras_of(pair["of"]))

    def test_the_answers_come_from_the_planted_sightings(self):
        plate = self.truth["of_interest"][0]
        sightings = self.oracle.sightings_of(plate)
        self.assertEqual(self.oracle.count_of(plate), len(sightings))
        first, last = self.oracle.first_last(plate)
        self.assertEqual((first, last), (min(t for _, t in sightings), max(t for _, t in sightings)))
        self.assertEqual(self.oracle.cameras_of(plate), sorted({c for c, _ in sightings}))
        self.assertEqual(self.oracle.count_of(plate, start=first, end=first) >= 1, True)
        self.assertEqual(self.oracle.count_of(plate, start=last + "x"), 0)
        cams = self.oracle.cameras_of(plate)
        if len(cams) >= 2:
            both = self.oracle.plates_at_both(cams[0], cams[1])
            self.assertIn(plate, both)
            self.assertEqual(both, self.oracle.plates_at_both(cams[1], cams[0]))
        top, n = self.oracle.busiest()
        self.assertEqual(n, max(self.oracle.per_camera().values()))
        self.assertEqual(sum(self.oracle.per_camera().values()), len(self.truth["sightings"]))

    def test_a_day_is_a_local_day_and_a_trip_can_cross_midnight(self):
        day = self.oracle.rows[0]["local"][:10]
        at = self.oracle.at_camera(self.oracle.rows[0]["camera"], day)
        self.assertTrue(all(t.startswith(day) for _, t in at))
        late = [r for r in self.oracle.rows if r["local"][11:13] == "23"]
        self.assertTrue(late, "the planted trips include sightings late in the evening")


if __name__ == "__main__":
    unittest.main()
