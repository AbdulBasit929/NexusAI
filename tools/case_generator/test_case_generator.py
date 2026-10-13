"""Tests for the case generator (plain unittest, standard library only):  python -X utf8 tools/case_generator/test_case_generator.py

Proves: the same seed gives the same bytes; every rendered export is selected by the registry, read without a rejected row and equals the
canonical truth; the independent audit passes; nothing in a case looks like real data; and the audit fails when a plant is removed, a
decoy stops being a decoy, or a file is the wrong layout. No real value appears here: values that look real are assembled at run time.
"""
import copy
import datetime
import hashlib
import io
import json
import os
import sys
import tempfile
import unittest
from contextlib import redirect_stdout

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import audit  # noqa: E402
import generate  # noqa: E402
import oracle  # noqa: E402
import render  # noqa: E402
import world as w  # noqa: E402
import scan_for_real_data  # noqa: E402  (tools/real_data is on the path through world.py)
from ingestion.forensic_records.layout_registry import Policy, interpret_rows, load_registry  # noqa: E402

REGISTRY = load_registry()
SMALL = dict(people=40, subjects=4, associates=8, period_days=14, p2_pairs=3)
POLICY = Policy(source_tz="Asia/Karachi", tz_basis="assumed_by_profile")
ALLOW = {"msisdn": set(), "ips": set(), "paths": [], "ignore_paths": []}


def small(seed=1, **extra):
    return w.Params(seed=seed, **dict(SMALL, **extra))


def tree_hashes(root):
    out = {}
    for base, _, names in os.walk(root):
        for name in names:
            full = os.path.join(base, name)
            with open(full, "rb") as handle:
                out[os.path.relpath(full, root)] = hashlib.sha256(handle.read()).hexdigest()
    return out


class CaseTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.root = generate.write_case(small(), cls.tmp.name)
        cls.world, cls.renders, cls.key = generate.build_case(small())

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def test_the_same_seed_gives_the_same_bytes(self):
        with tempfile.TemporaryDirectory() as other:
            again = generate.write_case(small(), other)
            self.assertEqual(tree_hashes(self.root), tree_hashes(again))
            different = generate.write_case(small(seed=2), other)
            with open(os.path.join(different, "manifest.json"), encoding="utf-8") as a, open(os.path.join(self.root, "manifest.json"), encoding="utf-8") as b:
                self.assertNotEqual(json.load(a)["case_id"], json.load(b)["case_id"])

    def test_zip_members_carry_a_fixed_timestamp(self):
        import zipfile
        for name in sorted(os.listdir(os.path.join(self.root, "files"))):
            with zipfile.ZipFile(os.path.join(self.root, "files", name)) as archive:
                self.assertEqual({info.date_time for info in archive.infolist()}, {(2026, 1, 1, 0, 0, 0)}, name)

    def test_every_number_is_inside_the_documented_project_block(self):
        # the literal limits of the specification (section 8), not the module the scanner and the audit also read
        for person in self.world.persons:
            self.assertTrue(923000000000 <= int(person.msisdn) <= 923000099999, person.pid)
            self.assertTrue(person.imsi.startswith("00101") and len(person.imsi) == 15)
            self.assertTrue(person.imei.startswith("00") and len(person.imei) == 15)

    def test_every_target_export_layout_is_covered(self):
        self.assertEqual({s["layout_id"] for s in self.key["subjects"]}, set(w.OPERATOR_LAYOUTS))

    def test_each_export_is_selected_read_and_equal_to_the_truth(self):
        for pid, rendered in self.renders.items():
            with self.subTest(layout=rendered.layout_id):
                entry = REGISTRY.get(rendered.layout_id)
                report = REGISTRY.find_header(rendered.table)
                self.assertEqual(report.matched_entry_id, rendered.layout_id, report.decision)
                self.assertEqual(report.header_row, rendered.header_index)
                results = list(interpret_rows(entry, rendered.table[rendered.header_index], rendered.table[rendered.header_index + 1:], policy=POLICY))
                self.assertEqual(len(results), len(rendered.rows))
                self.assertGreater(len(results), 20)
                for result, expected in zip(results, rendered.rows):
                    self.assertEqual(result.status, "accepted", result.reason)
                    for field in ("a_norm", "b_norm", "b_kind", "direction", "event_type", "start_local", "duration_s", "cell_key", "imsi", "imei"):
                        self.assertEqual(result.record.get(field), expected[field], "%s %s" % (expected["uid"], field))

    def test_the_audit_passes(self):
        self.assertEqual(audit.audit(self.root), [])

    def test_the_default_sized_case_passes_too(self):
        with tempfile.TemporaryDirectory() as d:
            self.assertEqual(audit.audit(generate.write_case(w.Params(seed=7), d)), [])

    def test_nothing_in_a_case_looks_like_real_data(self):
        for rendered in self.renders.values():
            text = "\n".join(",".join(str(c) for c in row) for row in rendered.table)
            self.assertEqual([f.format() for f in scan_for_real_data.scan_text(rendered.layout_id + ".csv", text, ALLOW)], [])
        for name in ("key.json", "world.json"):
            with open(os.path.join(self.root, "key", name), encoding="utf-8") as handle:
                self.assertEqual([f.format() for f in scan_for_real_data.scan_text(name, handle.read(), ALLOW)], [])
        # the check itself works: a number outside the reserved block is found
        looks_real = "92" + "346" + "8888888"
        self.assertTrue(scan_for_real_data.scan_text("x.csv", "a,%s,b" % looks_real, ALLOW))

    def test_plants_and_decoys_are_what_the_key_says(self):
        threshold = self.key["oracle"]["parameters"]["hub_threshold"]
        self.assertEqual(len(self.key["plants"]), 3)
        for plant in self.key["plants"]:
            self.assertGreaterEqual(len(plant["planted_shared"]), 3)
            service, hub = plant["decoys"]
            self.assertTrue(service["must_be_excluded"] and hub["must_be_excluded"])
            self.assertGreater(hub["subjects_in_contact"], threshold)
        answers = {frozenset((a["x"], a["y"])): a for a in self.key["oracle"]["A2"]}
        for plant in self.key["plants"]:
            answer = answers[frozenset((plant["x"]["msisdn"], plant["y"]["msisdn"]))]
            self.assertEqual(answer["plant"], plant["id"])
            shared = {s["number"] for s in answer["shared"]}
            for s in plant["planted_shared"]:
                self.assertIn(s["number"], shared)
            reasons = {e["number"]: e["reason"] for e in answer["excluded"]}
            hub = plant["decoys"][1]
            if hub["visible_in_both_files"]:
                self.assertEqual(reasons[hub["number"]], "hub")
            else:
                self.assertNotIn(hub["number"], shared)
        # the first two plants are pairs whose files can show both decoys, so both decoys are actually tested
        both = [p for p in self.key["plants"] if all(d["visible_in_both_files"] for d in p["decoys"])]
        self.assertGreaterEqual(len(both), 2)
        # the service decoy is excluded wherever both files show it; layouts without a short-code form cannot show it at all
        c2 = next(s for s in self.key["subjects"] if s["layout_id"] == "call-c2-strt-tm")
        self.assertTrue(c2["events_not_representable"] > 0)

    def test_unconfirmed_meanings_are_declared(self):
        items = {(a["layout_id"], a["item"]) for a in self.key["layout_assumptions"]}
        self.assertIn(("call-c1-company", "enum:Call Type=Outgoing"), items)

    def test_the_c2_export_never_carries_a_message(self):
        # C2's labels express calls only: its events of other kinds are left out, and the key counts them
        rendered = next(r for r in self.renders.values() if r.layout_id == "call-c2-strt-tm")
        self.assertEqual({row["event_type"] for row in rendered.rows}, {"voice"})
        self.assertTrue(all(reason.startswith("layout has no") for _, reason in rendered.skipped))

    # ---- deliberate breakages: the audit must notice ----------------------------------------------------------------------------------
    def fresh_case(self, tmp):
        return generate.write_case(small(), tmp)

    def test_removing_a_planted_contact_from_a_file_is_noticed(self):
        with tempfile.TemporaryDirectory() as d:
            root = self.fresh_case(d)
            world = generate.build_case(small())[0]
            with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
                key = json.load(handle)
            plant = key["plants"][0]
            target = next(s for s in key["subjects"] if s["msisdn"] == plant["x"]["msisdn"])
            planted = plant["planted_shared"][0]["person"]
            keep = {e.uid for e in world.events if not (e.plant == plant["id"] and planted in (e.orig, e.recip) and plant["x"]["person"] in (e.orig, e.recip))}
            subject = world.person(target["person"])
            damaged = render.render_subject(world, REGISTRY.get(target["layout_id"]), subject, only_events=keep)
            path = damaged.write(os.path.join(root, "files"), os.path.basename(target["file"]).rsplit(".", 1)[0])
            with open(os.path.join(root, "manifest.json"), encoding="utf-8") as handle:
                manifest = json.load(handle)
            with open(path, "rb") as handle:
                manifest["files"][target["file"]]["sha256"] = hashlib.sha256(handle.read()).hexdigest()
            with open(os.path.join(root, "manifest.json"), "w", encoding="utf-8") as handle:
                json.dump(manifest, handle)
            problems = audit.audit(root)
            self.assertTrue(any("planted shared contact" in p for p in problems), problems)

    def test_a_hub_that_is_not_above_the_threshold_is_noticed(self):
        with tempfile.TemporaryDirectory() as d:
            root = self.fresh_case(d)
            key_path = os.path.join(root, "key", "key.json")
            with open(key_path, encoding="utf-8") as handle:
                key = json.load(handle)
            key["oracle"]["parameters"]["hub_threshold"] = 99
            with open(key_path, "w", encoding="utf-8") as handle:
                json.dump(key, handle)
            self.assertTrue(any("hub" in p or "A2" in p for p in audit.audit(root)))

    def test_a_file_of_the_wrong_layout_is_noticed(self):
        with tempfile.TemporaryDirectory() as d:
            root = self.fresh_case(d)
            files = sorted(os.listdir(os.path.join(root, "files")))
            a, b = os.path.join(root, "files", files[0]), os.path.join(root, "files", files[1])
            with open(a, "rb") as x, open(b, "rb") as y:
                xa, yb = x.read(), y.read()
            with open(a, "wb") as x, open(b, "wb") as y:
                x.write(yb)
                y.write(xa)
            problems = audit.audit(root)
            self.assertTrue(any("registry selects" in p or "manifest hash" in p for p in problems), problems)

    def test_a_tampered_file_fails_its_manifest_hash(self):
        with tempfile.TemporaryDirectory() as d:
            root = self.fresh_case(d)
            path = os.path.join(root, "files", sorted(os.listdir(os.path.join(root, "files")))[0])
            with open(path, "ab") as handle:
                handle.write(b"x")
            self.assertTrue(any("manifest hash" in p for p in audit.audit(root)))


class OracleTests(unittest.TestCase):
    """The reference answers on a case small enough to check by hand."""

    def rows(self, items):
        return [{"b_norm": n, "b_kind": k, "b_raw_label": None if n else n_label, "direction": d, "start_local": t, "duration_s": s}
                for n, n_label, k, d, t, s in items]

    def test_common_contacts_by_hand(self):
        x, y, z = "923000000001", "923000000002", "923000000003"
        a, b, c, hub = "923000009001", "923000009002", "923000009003", "923000009999"
        visible = {
            x: self.rows([(a, None, "person_like", "outgoing", "2026-01-01 10:00:00", 30), (a, None, "person_like", "incoming", "2026-01-02 10:00:00", 5),
                          (b, None, "person_like", "outgoing", "2026-01-03 10:00:00", 9), (hub, None, "person_like", "outgoing", "2026-01-03 11:00:00", 9),
                          (None, "786", "short_code", "incoming", "2026-01-04 10:00:00", 0), (y, None, "person_like", "outgoing", "2026-01-05 10:00:00", 9)]),
            y: self.rows([(a, None, "person_like", "outgoing", "2026-01-06 10:00:00", 12), (c, None, "person_like", "outgoing", "2026-01-07 10:00:00", 12),
                          (hub, None, "person_like", "incoming", "2026-01-08 10:00:00", 1), (None, "786", "short_code", "incoming", "2026-01-09 10:00:00", 0),
                          (x, None, "person_like", "incoming", "2026-01-05 10:00:00", 9)]),
            z: self.rows([(hub, None, "person_like", "outgoing", "2026-01-08 10:00:00", 1)]),
        }
        answer = oracle.a2_common(visible, x, y, threshold=2)
        self.assertEqual([s["number"] for s in answer["shared"]], [a])
        self.assertEqual((answer["shared"][0]["x_count"], answer["shared"][0]["y_count"]), (2, 1))
        self.assertEqual(answer["shared"][0]["x_first"], "2026-01-01 10:00:00")
        self.assertEqual(answer["shared"][0]["x_last"], "2026-01-02 10:00:00")
        reasons = {e["number"]: e["reason"] for e in answer["excluded"]}
        self.assertEqual(reasons, {hub: "hub", "786": "service"})
        # a is reached by two subjects: with a threshold of 1 it becomes a hub too, and the answer is empty
        self.assertEqual(oracle.a2_common(visible, x, y, threshold=1)["shared"], [])
        profile = oracle.a1_profile(visible)[x]
        self.assertEqual((profile[0]["number"], profile[0]["outgoing"], profile[0]["incoming"], profile[0]["voice_seconds"]), (a, 1, 1, 35))


class ColocationOracleTests(unittest.TestCase):
    """A5 on cases small enough to check by hand."""

    @staticmethod
    def rows(items):
        return [{"start_local": when, "cell_key": cell} for when, cell in items]

    def test_windows_pairs_and_the_boundary_by_hand(self):
        x = self.rows([("2026-02-03 10:00:00", "A"), ("2026-02-03 10:06:00", "A"), ("2026-02-03 12:00:00", "B"), ("2026-02-03 15:00:00", "A")])
        y = self.rows([("2026-02-03 10:03:00", "A"), ("2026-02-03 10:20:00", "A"), ("2026-02-03 12:20:00", "B"), ("2026-02-03 15:00:00", "C")])
        answer = oracle.a5_colocation(x, y, 15)
        # (10:00,10:03) (10:06,10:03) (10:06,10:20) are on cell A; (10:00,10:20) and (12:00,12:20) are over 15 minutes apart; 15:00 is two cells
        self.assertEqual((answer["pairs_observed"], answer["pairs_within_window"]), (3, 4))
        self.assertEqual(answer["windows"], [{"cell_key": "A", "start_local": "2026-02-03 10:00:00", "end_local": "2026-02-03 10:20:00", "pairs": 3}])
        self.assertEqual(answer["caveat"], "proximity is not association")
        exact = oracle.a5_colocation(self.rows([("2026-02-03 10:00:00", "A")]), self.rows([("2026-02-03 10:15:00", "A")]), 15)
        self.assertEqual(exact["pairs_observed"], 1, "a difference of exactly W is co-located")
        over = oracle.a5_colocation(self.rows([("2026-02-03 10:00:00", "A")]), self.rows([("2026-02-03 10:15:01", "A")]), 15)
        self.assertEqual(over["pairs_observed"], 0)
        self.assertEqual(oracle.a5_colocation(self.rows([("2026-02-03 10:00:00", "A")]), self.rows([("2026-02-03 10:01:00", "B")]), 15)["pairs_observed"], 0)

    def test_the_chance_baseline_by_hand(self):
        x = self.rows([("2026-02-03 10:0%d:00" % i, cell) for i, cell in enumerate("AAAB")])
        y = self.rows([("2026-02-03 10:04:00", "A"), ("2026-02-03 10:05:00", "B")])
        answer = oracle.a5_colocation(x, y, 15)
        # eight pairs inside the window; X is at A three times in four, Y at A half the time: same cell with probability 0.5 each
        self.assertEqual(answer["pairs_within_window"], 8)
        self.assertAlmostEqual(answer["pairs_expected"], 4.0)
        self.assertEqual(answer["pairs_observed"], 4)
        self.assertEqual(answer["lift"], {"value": 1.0, "note": None})
        # one co-located pair, but both subjects roam over four cells at that hour: only 0.0625 pairs are expected, too few for a ratio
        x = self.rows([("2026-02-03 10:00:00", "A"), ("2026-02-04 10:00:00", "B"), ("2026-02-05 10:00:00", "C"), ("2026-02-06 10:00:00", "D")])
        y = self.rows([("2026-02-03 10:01:00", "A"), ("2026-02-04 10:40:00", "E"), ("2026-02-05 10:40:00", "F"), ("2026-02-06 10:40:00", "G")])
        few = oracle.a5_colocation(x, y, 15)
        self.assertEqual((few["pairs_observed"], few["pairs_within_window"], few["pairs_expected"]), (1, 1, 0.0625))
        self.assertEqual(few["lift"], {"value": None, "note": "too few events"})

    def test_a_week_later_or_other_cell_names_change_nothing_but_the_labels(self):
        x = self.rows([("2026-02-03 10:00:00", "A"), ("2026-02-03 10:06:00", "A"), ("2026-02-04 09:00:00", "B")] * 1)
        y = self.rows([("2026-02-03 10:03:00", "A"), ("2026-02-03 10:20:00", "A"), ("2026-02-04 09:10:00", "B")])
        base = oracle.a5_colocation(x, y, 15)
        week = datetime.timedelta(days=7)

        def shifted(rows, delta, rename):
            return [{"start_local": (datetime.datetime.fromisoformat(r["start_local"]) + delta).isoformat(sep=" "), "cell_key": rename.get(r["cell_key"], r["cell_key"])} for r in rows]
        later = oracle.a5_colocation(shifted(x, week, {"A": "cell-1", "B": "cell-2"}), shifted(y, week, {"A": "cell-1", "B": "cell-2"}), 15)
        for field in ("pairs_observed", "pairs_within_window", "pairs_expected", "lift"):
            self.assertEqual(later[field], base[field], field)
        self.assertEqual([w["pairs"] for w in later["windows"]], [w["pairs"] for w in base["windows"]])


class ColocationCaseTests(unittest.TestCase):
    PARAMS = dict(people=60, subjects=6, associates=12, period_days=30, p2_pairs=2, p3_pairs=2)

    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.params = w.Params(seed=11, **cls.PARAMS)
        cls.root = generate.write_case(cls.params, cls.tmp.name)
        with open(os.path.join(cls.root, "key", "key.json"), encoding="utf-8") as handle:
            cls.key = json.load(handle)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def plants(self):
        return [p for p in self.key["plants"] if p["type"] == "colocation"]

    def test_the_audit_passes_with_co_location(self):
        self.assertEqual(audit.audit(self.root), [])

    def test_two_plants_on_the_same_operator(self):
        plants = self.plants()
        self.assertEqual(len(plants), 2)
        operators = {s["msisdn"]: s["operator"] for s in self.key["subjects"]}
        for plant in plants:
            self.assertEqual(operators[plant["x"]["msisdn"]], operators[plant["y"]["msisdn"]])
            self.assertEqual(len(plant["planted_windows"]), 4)
            self.assertEqual({d["kind"] for d in plant["decoys"]}, {"same_cell_far_apart_in_time", "same_cell_just_outside_window", "same_moment_different_cells"})

    def test_planted_windows_are_in_the_answer_and_decoys_are_not(self):
        answers = {frozenset((a["x"], a["y"])): a for a in self.key["oracle"]["A5"]}
        for plant in self.plants():
            answer = answers[frozenset((plant["x"]["msisdn"], plant["y"]["msisdn"]))]
            self.assertEqual(answer["plant"], plant["id"])
            for planted in plant["planted_windows"]:
                self.assertTrue(any(w_["cell_key"] == planted["cell_key"] and w_["start_local"] <= planted["start_local"] and w_["end_local"] >= planted["end_local"]
                                    for w_ in answer["windows"]), planted["tag"])
            decoy_times = {e["start_local"] for d in plant["decoys"] for e in d["events"]}
            covered = set()
            for w_ in answer["windows"]:
                covered |= {t for t in decoy_times if w_["start_local"] <= t <= w_["end_local"] and w_["cell_key"] == plant["planted_windows"][0]["cell_key"]}
            self.assertEqual(covered, set(), "no decoy event lies inside an answer window on the meeting cell")

    def test_the_meeting_cell_is_nobody_s_background_cell(self):
        world = w.build_world(self.params)
        self.assertEqual(len(world.p3), 2)
        for plant in world.p3:
            for pid in (plant["x"], plant["y"]):
                person = world.person(pid)
                background = {person.home_cell, person.work_cell, (person.home_cell + 1) % w.CELLS_PER_OPERATOR, (person.work_cell + 1) % w.CELLS_PER_OPERATOR}
                self.assertNotIn(plant["meeting_site"], background)
                self.assertNotIn(plant["other_site"], background)
            self.assertNotEqual(plant["meeting_site"], plant["other_site"])

    def test_only_pairs_on_one_operator_have_an_answer(self):
        operators = {s["msisdn"]: s["operator"] for s in self.key["subjects"]}
        self.assertTrue(self.key["oracle"]["A5"])
        for answer in self.key["oracle"]["A5"]:
            self.assertEqual(operators[answer["x"]], operators[answer["y"]])

    def test_a_planted_window_that_vanishes_from_the_files_is_noticed(self):
        with tempfile.TemporaryDirectory() as d:
            root = generate.write_case(self.params, d)
            world = generate.build_case(self.params)[0]
            key = self.key
            plant = key["plants"][[p["type"] for p in key["plants"]].index("colocation")]
            tag = plant["planted_windows"][0]["tag"]
            subject = next(s for s in key["subjects"] if s["msisdn"] == plant["y"]["msisdn"])
            keep = {e.uid for e in world.events if e.plant != tag or plant["x"]["person"] in (e.orig, e.recip)}
            damaged = render.render_subject(world, REGISTRY.get(subject["layout_id"]), world.person(subject["person"]), only_events=keep)
            path = damaged.write(os.path.join(root, "files"), os.path.basename(subject["file"]).rsplit(".", 1)[0])
            with open(os.path.join(root, "manifest.json"), encoding="utf-8") as handle:
                manifest = json.load(handle)
            with open(path, "rb") as handle:
                manifest["files"][subject["file"]]["sha256"] = hashlib.sha256(handle.read()).hexdigest()
            with open(os.path.join(root, "manifest.json"), "w", encoding="utf-8") as handle:
                json.dump(manifest, handle)
            problems = audit.audit(root)
            self.assertTrue(any("A5" in p or "planted window" in p for p in problems), problems)

    def test_nothing_in_the_co_location_case_looks_real(self):
        for name in ("key.json", "world.json"):
            with open(os.path.join(self.root, "key", name), encoding="utf-8") as handle:
                self.assertEqual([f.format() for f in scan_for_real_data.scan_text(name, handle.read(), ALLOW)], [])

    def test_the_default_case_has_two_co_location_plants(self):
        with tempfile.TemporaryDirectory() as d:
            root = generate.write_case(w.Params(seed=5), d)
            with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
                key = json.load(handle)
            self.assertEqual(len([p for p in key["plants"] if p["type"] == "colocation"]), 2)
            self.assertEqual(audit.audit(root), [])


class CommandLineTests(unittest.TestCase):
    def test_main_writes_a_case(self):
        with tempfile.TemporaryDirectory() as d:
            out = io.StringIO()
            with redirect_stdout(out):
                code = generate.main(["--seed", "3", "--out", d, "--people", "40", "--subjects", "4", "--associates", "8", "--days", "10", "--p2-pairs", "2"])
            self.assertEqual(code, 0)
            self.assertIn("wrote", out.getvalue())
            self.assertEqual(audit.audit(os.path.join(d, "case-3")), [])

    def test_world_is_deterministic_and_pairs_are_limited_by_the_subjects(self):
        a, b = w.build_world(small(p2_pairs=50)), w.build_world(small(p2_pairs=50))
        self.assertEqual(w.world_to_json(a), w.world_to_json(b))
        self.assertEqual(len(a.p2), 6)  # four subjects make six pairs


if __name__ == "__main__":
    unittest.main(verbosity=1)
