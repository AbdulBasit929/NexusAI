"""Tests for check_ledger.py. Plain unittest, standard library only:  python -X utf8 tools/licence_ledger/test_check_ledger.py

Each rule is also broken on purpose in a temporary copy of the real ledger, and the checker must fail on it."""
import csv
import os
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import check_ledger as cl  # noqa: E402


def write(path, text):
    with open(path, "w", encoding="utf-8") as handle:
        handle.write(text)


def read(path):
    with open(path, encoding="utf-8") as handle:
        return handle.read()


class Workspace:
    """A temporary copy of the ledger, the requirements file and the compose file, editable row by row."""

    def __init__(self):
        self.dir = tempfile.TemporaryDirectory()
        _, self.rows = cl.read_rows(cl.LEDGER)
        self.requirements = read(cl.REQUIREMENTS)
        self.compose = read(cl.COMPOSE)

    def run(self):
        ledger = os.path.join(self.dir.name, "ledger.csv")
        with open(ledger, "w", encoding="utf-8", newline="") as handle:
            writer = csv.DictWriter(handle, fieldnames=cl.COLUMNS, lineterminator="\n")
            writer.writeheader()
            writer.writerows(self.rows)
        req = os.path.join(self.dir.name, "requirements.txt")
        compose = os.path.join(self.dir.name, "compose.yaml")
        write(req, self.requirements)
        write(compose, self.compose)
        return cl.check(ledger, req, compose)

    def row(self, ref):
        return next(r for r in self.rows if ref in r["config_ref"].split(";"))

    def close(self):
        self.dir.cleanup()


class LedgerTests(unittest.TestCase):
    def setUp(self):
        self.ws = Workspace()
        self.addCleanup(self.ws.close)

    def assertFails(self, text):
        problems = self.ws.run()
        self.assertTrue(any(text in p for p in problems), "expected a problem containing %r, got %s" % (text, problems))

    def test_the_real_ledger_passes(self):
        self.assertEqual(self.ws.run(), [])

    def test_a_pinned_package_without_a_row_fails(self):
        self.ws.requirements += "\nsome-new-package==1.0.0\n"
        self.assertFails("some-new-package is pinned")

    def test_a_removed_row_fails_coverage(self):
        self.ws.rows.remove(self.ws.row("pkg:polars"))
        self.assertFails("polars is pinned")

    def test_a_new_model_setting_without_a_row_fails(self):
        self.ws.compose += "\n      FORENSIC_NEW_ENGINE_MODEL_PATH: ${X:-/models/x}\n"
        self.assertFails("FORENSIC_NEW_ENGINE_MODEL_PATH")

    def test_every_model_setting_of_the_compose_file_is_found(self):
        found = set(cl.MODEL_SETTING.findall(self.ws.compose))
        for name in ("FORENSIC_SYNTHESIS_MODEL", "FORENSIC_FACE_MODEL", "FORENSIC_ANPR_DETECTOR_MODEL_PATH",
                     "FORENSIC_VIDEO_ANPR_VEHICLE_DETECTOR_PATH", "FORENSIC_OCR_TESSERACT_PATH", "FORENSIC_PADDLE_ARABIC_MODEL_DIR"):
            self.assertIn(name, found)

    def test_requirement_lines_are_parsed(self):
        path = os.path.join(self.ws.dir.name, "r.txt")
        write(path, "# comment\n--extra-index-url https://example.invalid/x\nFoo_Bar[extra]==1.0  # note\nbaz>=2\ntorch==2.5.1+cpu\n\n")
        self.assertEqual(cl.package_names(path), ["foo-bar", "baz", "torch"])

    def test_approved_needs_tier_a(self):
        self.ws.row("pkg:polars")["evidence_tier"] = "B"
        self.assertFails("approved needs evidence tier A")

    def test_approved_needs_a_permissive_licence(self):
        self.ws.row("pkg:polars")["licence"] = "AGPL-3.0"
        self.assertFails("not plainly permissive")
        self.ws.row("pkg:polars")["licence"] = "MIT with non-commercial clause"
        self.assertFails("not plainly permissive")
        self.ws.row("pkg:polars")["licence"] = "LGPL-3.0-only"
        self.assertFails("not plainly permissive")

    def test_conditional_needs_a_condition(self):
        self.ws.row("pkg:psycopg")["condition"] = ""
        self.assertFails("needs a condition")

    def test_blocked_and_unverified_are_ship_blocking(self):
        self.ws.row("FORENSIC_VIDEO_ANPR_VEHICLE_DETECTOR_PATH")["ship_blocking"] = "no"
        self.assertFails("ship_blocking must be yes")
        self.ws.row("FORENSIC_VIDEO_ANPR_VEHICLE_DETECTOR_PATH")["ship_blocking"] = "yes"
        self.ws.row("pkg:polars")["ship_blocking"] = "yes"
        self.assertFails("ship_blocking must be yes")

    def test_schema_rules(self):
        self.ws.rows.append(dict(self.ws.rows[0]))
        self.assertFails("duplicate id")
        self.ws.rows.pop()
        self.ws.rows[0]["checked_on"] = "yesterday"
        self.assertFails("is not a date")
        self.ws.rows[0]["checked_on"] = "2026-10-12"
        self.ws.rows[0]["status"] = "fine"
        self.assertFails("status 'fine'")
        self.ws.rows[0]["status"] = "approved"
        self.ws.rows[0]["licence"] = " "
        self.assertFails("licence is empty")

    def test_wrong_columns_fail(self):
        path = os.path.join(self.ws.dir.name, "bad.csv")
        write(path, "id,component\nX,y\n")
        self.assertTrue(cl.check(path)[0].startswith("columns are"))

    def test_the_known_open_issues_are_recorded(self):
        by_ref = {r["config_ref"]: r for r in self.ws.rows if r["config_ref"]}
        self.assertEqual(by_ref["FORENSIC_VIDEO_ANPR_VEHICLE_DETECTOR_PATH"]["status"], "blocked")
        self.assertEqual(by_ref["pkg:ultralytics"]["status"], "blocked")
        self.assertEqual(by_ref["FORENSIC_ANPR_OCR_MODEL_PATH"]["status"], "unverified")


if __name__ == "__main__":
    unittest.main(verbosity=1)
