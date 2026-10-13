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


COMPARED = ("a_norm", "b_norm", "b_kind", "direction", "event_type", "start_local", "duration_s", "cell_key", "imsi", "imei")


def read_back(test, entry, rendered):
    """Interpret a rendered call-record export and compare every row with what the generator says it must become."""
    results = list(interpret_rows(entry, rendered.table[rendered.header_index], rendered.table[rendered.header_index + 1:], policy=POLICY))
    test.assertEqual(len(results), len(rendered.rows))
    for n, (result, expected) in enumerate(zip(results, rendered.rows)):
        status = expected.get("status", "accepted")
        test.assertEqual(result.status, status, "%s row %d (%s)" % (rendered.layout_id, n, expected.get("defect")))
        if status != "accepted":
            test.assertEqual(result.reason, expected["reason"])
            continue
        for flag in expected.get("flags", []):
            test.assertIn(flag, result.flags)
        for field in COMPARED:
            test.assertEqual(result.record.get(field), expected[field], "%s row %d %s" % (rendered.layout_id, n, field))
    return results


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
        for name in sorted(n for n in os.listdir(os.path.join(self.root, "files")) if n.endswith(".xlsx")):
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
            if pid.startswith(("ipdr-", "usage-", "ipquery-")):
                continue  # internet-record and usage exports have their own tests
            with self.subTest(layout=rendered.layout_id):
                entry = REGISTRY.get(rendered.layout_id)
                report = REGISTRY.find_header(rendered.table)
                self.assertEqual(report.matched_entry_id, rendered.layout_id, report.decision)
                self.assertEqual(report.header_row, rendered.header_index)
                self.assertGreater(len(rendered.rows), 20)
                read_back(self, entry, rendered)

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
        common = [p for p in self.key["plants"] if p["type"] == "common_contacts"]
        self.assertEqual(len(common), 3)
        for plant in common:
            self.assertGreaterEqual(len(plant["planted_shared"]), 3)
            service, hub = plant["decoys"]
            self.assertTrue(service["must_be_excluded"] and hub["must_be_excluded"])
            self.assertGreater(hub["subjects_in_contact"], threshold)
        answers = {frozenset((a["x"], a["y"])): a for a in self.key["oracle"]["A2"]}
        for plant in common:
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
        both = [p for p in common if all(d["visible_in_both_files"] for d in p["decoys"])]
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
        self.assertEqual({row["event_type"] for row in rendered.visible}, {"voice"})
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


class DeviceTests(unittest.TestCase):
    """Plant P5 and the A8 oracle."""
    PARAMS = dict(people=60, subjects=6, associates=12, period_days=30, p2_pairs=1, p3_pairs=2, p7_plants=0)

    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.params = w.Params(seed=71, **cls.PARAMS)
        cls.world, cls.renders, cls.key = generate.build_case(cls.params)
        cls.root = generate.write_case(cls.params, cls.tmp.name)
        with open(os.path.join(cls.root, "key", "key.json"), encoding="utf-8") as handle:
            cls.written = json.load(handle)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def test_identifiers_are_only_changed_where_the_layout_shows_them(self):
        for op in range(4):
            fields = REGISTRY.get(w.OPERATOR_LAYOUTS[op])["fields"]
            self.assertTrue(fields.get("imei"))
            self.assertEqual(bool(fields.get("imsi")), op in w.IDENTIFIER_OPERATORS, w.OPERATOR_LAYOUTS[op])

    def test_three_changes_and_a_dual_sim_phone(self):
        changes = [p for p in self.key["plants"] if p["type"] == "device_change"]
        self.assertEqual(sorted(p["kind"] for p in changes), ["both_change", "handset_change", "sim_change"])
        dual = [p for p in self.key["plants"] if p["type"] == "dual_sim"]
        self.assertEqual(len(dual), 1)
        operators = {s["msisdn"]: s["operator"] for s in self.key["subjects"]}
        self.assertEqual(len({operators[n] for n in dual[0]["numbers"]}), 1)

    def test_every_planted_change_is_found_once_between_the_last_old_and_first_new(self):
        numbers = self.key["oracle"]["A8"]["numbers"]
        for plant in [p for p in self.key["plants"] if p["type"] == "device_change"]:
            changes = numbers[plant["number"]]["changes"]
            self.assertEqual(len(changes), 1, plant["id"])
            change = changes[0]
            self.assertEqual(change["kind"], {"handset_change": "imei", "sim_change": "imsi", "both_change": "both"}[plant["kind"]])
            self.assertLess(change["last_seen_old"], plant["from"])
            self.assertLessEqual(plant["from"], change["first_seen_new"])
            self.assertEqual((change["old_imei"], change["new_imei"]), (plant["old_imei"], plant["new_imei"]))
        planted = {p["number"] for p in self.key["plants"] if p["type"] == "device_change"}
        self.assertEqual({n for n, v in numbers.items() if v["changes"]}, planted, "no number changes identity unless a plant says so")

    def test_the_dual_sim_phone_is_one_device_under_two_numbers_at_once(self):
        dual = next(p for p in self.key["plants"] if p["type"] == "dual_sim")
        shared = {s["imei"]: s for s in self.key["oracle"]["A8"]["shared_devices"]}
        self.assertEqual(set(shared), {dual["imei"]})
        self.assertEqual(shared[dual["imei"]]["relation"], "concurrent")
        self.assertEqual([n["number"] for n in shared[dual["imei"]]["numbers"]], dual["numbers"])

    def test_identity_follows_the_segments(self):
        person = next(p for p in self.world.persons if p.segments and p.pid in {r.get("pid") for r in self.world.p5})
        segment = person.segments[0]
        before = datetime.datetime.fromisoformat(segment["from"]) - datetime.timedelta(seconds=1)
        after = datetime.datetime.fromisoformat(segment["from"])
        self.assertEqual(w.identity_at(person, before), (person.imsi, person.n))
        self.assertEqual(w.identity_at(person, after), (segment["imsi"], segment["imei_n"]))

    def test_the_audit_passes_with_device_changes(self):
        self.assertEqual(audit.audit(self.root), [])

    def test_a_change_that_is_missing_from_the_files_is_noticed(self):
        plant = next(p for p in self.written["plants"] if p["type"] == "device_change" and p["kind"] == "handset_change")
        person = next(p for p in self.world.persons if p.msisdn == plant["number"])
        subject = next(s for s in self.written["subjects"] if s["msisdn"] == plant["number"])
        saved = list(person.segments)
        person.segments.clear()
        try:
            damaged = render.render_subject(self.world, REGISTRY.get(subject["layout_id"]), person)
        finally:
            person.segments[:] = saved
        with tempfile.TemporaryDirectory() as d:
            root = generate.write_case(self.params, d)
            path = damaged.write(os.path.join(root, "files"), os.path.basename(subject["file"]).rsplit(".", 1)[0])
            with open(os.path.join(root, "manifest.json"), encoding="utf-8") as handle:
                manifest = json.load(handle)
            with open(path, "rb") as handle:
                manifest["files"][subject["file"]]["sha256"] = hashlib.sha256(handle.read()).hexdigest()
            with open(os.path.join(root, "manifest.json"), "w", encoding="utf-8") as handle:
                json.dump(manifest, handle)
            self.assertTrue(any("is not in the files exactly once" in p or "A8 for" in p for p in audit.audit(root)), audit.audit(root))
        # the same damage by editing the key: pretend the change happened a month later than the files show
        with tempfile.TemporaryDirectory() as d:
            root = generate.write_case(self.params, d)
            key_path = os.path.join(root, "key", "key.json")
            with open(key_path, encoding="utf-8") as handle:
                key = json.load(handle)
            for p in key["plants"]:
                if p["type"] == "device_change" and p["kind"] == "handset_change":
                    p["from"] = "2026-12-31 00:00:00"
            with open(key_path, "w", encoding="utf-8") as handle:
                json.dump(key, handle)
            self.assertTrue(any("planted date is not between" in p for p in audit.audit(root)), audit.audit(root))

    def test_device_history_by_hand(self):
        def rows(*items):
            return [{"start_local": when, "imsi": imsi, "imei": imei} for when, imsi, imei in items]
        a, b, c = "A" * 15, "B" * 15, "C" * 15
        d1, d2 = "000000000000010", "000000000000020"
        visible = {
            "N1": rows(("2026-01-01 10:00:00", a, d1), ("2026-01-02 10:00:00", a, d1), ("2026-01-03 10:00:00", a, d2), ("2026-01-04 10:00:00", b, d2)),   # handset, then SIM
            "N2": rows(("2026-01-01 11:00:00", c, d1[:14] + "7"), ("2026-01-05 11:00:00", c, d1[:14] + "7")),                                           # d1 again, another check digit
            "N3": rows(("2026-01-10 09:00:00", "S", d2), ("2026-01-11 09:00:00", "S", d2)),                                                           # takes d2 after N1 left it? no: N1 still on d2 on 4th
            "N4": rows(("2026-01-20 09:00:00", None, d1[:14] + "9")),
        }
        answer = oracle.a8_device_history(visible)
        self.assertEqual([c["kind"] for c in answer["numbers"]["N1"]["changes"]], ["imei", "imsi"])
        self.assertEqual(answer["numbers"]["N1"]["changes"][0]["last_seen_old"], "2026-01-02 10:00:00")
        self.assertEqual(answer["numbers"]["N1"]["changes"][0]["first_seen_new"], "2026-01-03 10:00:00")
        self.assertEqual(answer["numbers"]["N2"]["changes"], [])
        shared = {s["imei"]: s["relation"] for s in answer["shared_devices"]}
        self.assertEqual(shared[d1[:14]], "concurrent")      # N1 used it until the 2nd, N2 on the 1st and the 5th: their periods overlap
        self.assertEqual(shared[d2[:14]], "sequential")      # N1 until the 4th, N3 from the 10th
        both = oracle.a8_device_history({"X": rows(("2026-01-01 10:00:00", a, d1), ("2026-01-02 10:00:00", b, d2))})
        self.assertEqual(both["numbers"]["X"]["changes"][0]["kind"], "both")


class SuccessorTests(unittest.TestCase):
    """Plant P6 and the A9 oracle."""
    PARAMS = dict(people=80, subjects=10, associates=16, period_days=60, p2_pairs=2, p3_pairs=2, p7_plants=0)

    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.params = w.Params(seed=81, **cls.PARAMS)
        cls.world, cls.renders, cls.key = generate.build_case(cls.params)
        cls.root = generate.write_case(cls.params, cls.tmp.name)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def plants(self, kind):
        return [p for p in self.key["plants"] if p["type"] == kind]

    def test_the_parts_are_allocated_without_overlap(self):
        roles = self.world.roles
        taken = [pid for pair in roles["p3_pairs"] + roles["p6_pairs"] for pid in pair] + [roles["p6_silent"], roles["p6_late"]]
        self.assertEqual(len(taken), len(set(taken)), "no subject plays two of these parts")
        self.assertEqual(len(roles["p3_pairs"]), 2)
        self.assertEqual(len(roles["p6_pairs"]), 2)
        operator = {p.pid: p.operator for p in self.world.persons}
        for a, b in roles["p3_pairs"] + roles["p6_pairs"]:
            self.assertEqual(operator[a], operator[b], "a pair shares an operator, so cell identities can be compared")

    def test_two_successors_a_silent_number_and_a_late_starter(self):
        self.assertEqual(len(self.plants("successor")), 2)
        self.assertEqual(len(self.plants("silent_without_successor")), 1)
        self.assertEqual(len(self.plants("late_without_predecessor")), 1)
        self.assertEqual(sorted(p["with_device"] for p in self.plants("successor")), [False, True])

    def test_each_successor_is_the_first_candidate_and_the_decoys_have_none(self):
        silent = {s["number"]: s["candidates"] for s in self.key["oracle"]["A9"]["silent"]}
        threshold = self.key["oracle"]["A9"]["parameters"]["threshold"]
        for plant in self.plants("successor"):
            candidates = silent[plant["old"]]
            self.assertEqual(candidates[0]["number"], plant["new"])
            self.assertGreaterEqual(candidates[0]["score"], threshold)
            self.assertEqual(candidates[0]["shared_device"], plant["with_device"])
            self.assertGreater(candidates[0]["contact_cosine"], 0.5)
            self.assertGreater(candidates[0]["cell_jaccard"], 0.5)
        decoy = self.plants("silent_without_successor")[0]["number"]
        self.assertEqual(silent[decoy], [])
        late = self.plants("late_without_predecessor")[0]["number"]
        self.assertTrue(all(c["number"] != late for cands in silent.values() for c in cands))

    def test_the_numbers_are_in_use_only_on_their_planted_days(self):
        start = datetime.date.fromisoformat(self.params.start_date)
        for pid, (first_day, last_day) in self.world.active.items():
            days = [(e.start.date() - start).days for e in self.world.events if pid in (e.orig, e.recip)]
            self.assertTrue(days)
            self.assertGreaterEqual(min(days), first_day, pid)
            self.assertLessEqual(max(days), last_day, pid)
            sessions = [(s.start.date() - start).days for s in self.world.sessions if s.pid == pid]
            self.assertTrue(all(first_day <= d <= last_day for d in sessions), pid)

    def test_the_successor_that_keeps_the_handset_shows_as_a_sequential_shared_device(self):
        with_device = next(p for p in self.plants("successor") if p["with_device"])
        shared = [s for s in self.key["oracle"]["A8"]["shared_devices"] if {n["number"] for n in s["numbers"]} == {with_device["old"], with_device["new"]}]
        self.assertEqual([s["relation"] for s in shared], ["sequential"])

    def test_the_audit_passes_with_successors(self):
        self.assertEqual(audit.audit(self.root), [])

    def test_a_wrongly_keyed_handset_is_noticed(self):
        with tempfile.TemporaryDirectory() as d:
            root = generate.write_case(self.params, d)
            key_path = os.path.join(root, "key", "key.json")
            with open(key_path, encoding="utf-8") as handle:
                key = json.load(handle)
            for p in key["plants"]:
                if p["type"] == "successor":
                    p["with_device"] = not p["with_device"]
            with open(key_path, "w", encoding="utf-8") as handle:
                json.dump(key, handle)
            self.assertTrue(any("shared handset differs" in p for p in audit.audit(root)))

    def test_successor_candidates_by_hand(self):
        base = datetime.datetime(2026, 2, 1)

        def row(day, contact, cell, imei=None):
            return {"start_local": (base + datetime.timedelta(days=day, hours=10)).isoformat(sep=" "), "b_kind": "person_like", "b_norm": contact, "cell_key": cell, "imei": imei}
        contacts = ["C1", "C2", "C3"]
        old = [row(d, contacts[d % 3], "K1" if d % 2 else "K2", "000000000000010") for d in range(0, 30)]          # active days 0 to 29
        new = [row(d, contacts[d % 3], "K1" if d % 2 else "K2", "000000000000019") for d in range(33, 60)]          # starts on day 33, same contacts and cells, same device (14 digits)
        stranger = [row(d, "S%d" % (d % 3), "Z1" if d % 2 else "Z2") for d in range(33, 60)]                          # starts at the same time, shares nothing
        early = [row(d, contacts[d % 3], "K1") for d in range(0, 60)]                                               # active all along: never a successor
        visible = {"OLD": old, "NEW": new, "STR": stranger, "ALL": early}
        end = base + datetime.timedelta(days=60)
        answer = oracle.a9_successors(visible, base, end)
        silent = {s["number"]: s["candidates"] for s in answer["silent"]}
        self.assertEqual(set(silent), {"OLD"})
        self.assertEqual([c["number"] for c in silent["OLD"]], ["NEW"])                # the stranger scores 0, the always-active number was never new
        c = silent["OLD"][0]
        self.assertAlmostEqual(c["contact_cosine"], 1.0, places=2)
        self.assertEqual((c["cell_jaccard"], c["shared_device"], c["score"]), (1.0, True, 1.0))
        # the window: a successor that starts 15 days after the last event is outside the 14-day window
        late = {"OLD": old, "NEW": [row(d, contacts[d % 3], "K1") for d in range(45, 60)]}
        self.assertEqual(oracle.a9_successors(late, base, end)["silent"][0]["candidates"], [])
        # silence: a number whose last event is 6 days before the end is not silent yet
        recent = {"OLD": [row(d, "C1", "K1") for d in range(0, 54)], "NEW": [row(d, "C1", "K1") for d in range(20, 60)]}
        self.assertEqual(oracle.a9_successors(recent, base, end)["silent"], [])


class MovementTests(unittest.TestCase):
    """Plant P8 (a move, a week away, impossible speed and its slow decoy, a change of rhythm and one unusual day) and the A10 and A11 oracles."""
    PARAMS = dict(people=80, subjects=12, associates=16, period_days=60, p2_pairs=2, p3_pairs=2, p7_plants=0)

    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.params = w.Params(seed=91, **cls.PARAMS)
        cls.world, cls.renders, cls.key = generate.build_case(cls.params)
        cls.root = generate.write_case(cls.params, cls.tmp.name)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def plants(self, kind):
        return [p for p in self.key["plants"] if p["type"] == kind]

    def test_every_part_is_planted_on_subjects_that_play_nothing_else(self):
        for kind, count in (("relocation", 1), ("impossible_speed", 2), ("slow_decoy", 1), ("rate_change", 1), ("outlier_day", 1), ("short_trip", 1)):
            self.assertEqual(len(self.plants(kind)), count, kind)
        roles = self.world.roles
        busy = {pid for pair in roles["p3_pairs"] + roles["p6_pairs"] for pid in pair} | {roles["p6_silent"], roles["p6_late"]}
        self.assertFalse(busy & set(roles["p8"]))
        self.assertEqual(len(roles["p8"]), 2)
        self.assertEqual({p["pid"] for p in self.world.p8}, set(roles["p8"]))

    def test_the_home_moves_for_good_and_the_trip_comes_back(self):
        move = self.plants("relocation")[0]
        found = [c for c in self.key["oracle"]["A10"]["numbers"][move["number"]]["changes"] if c["slot"] == "night"]
        self.assertEqual([(c["from_cell"], c["to_cell"]) for c in found], [(move["old_cell_key"], move["new_cell_key"])])
        self.assertTrue(7 * (found[0]["first_week"] - 1) <= move["from_day"] < 7 * (found[0]["first_week"] + 1))
        self.assertGreater(found[0]["distance_km"], 4.0)
        self.assertEqual([c for c in self.key["oracle"]["A10"]["numbers"][move["number"]]["changes"] if c["slot"] == "day"], [], "only the home moved")
        trip = self.plants("short_trip")[0]
        first = (datetime.date.fromisoformat(trip["from_date"]) - datetime.date.fromisoformat(self.params.start_date)).days
        self.assertEqual((first % 7, (datetime.date.fromisoformat(trip["to_date"]) - datetime.date.fromisoformat(trip["from_date"])).days), (0, 6), "a whole week of the case")
        report = self.key["oracle"]["A10"]["numbers"][trip["number"]]
        self.assertEqual([c for c in report["changes"] if c["slot"] == "night"], [])
        self.assertIn(("night", trip["trip_cell_key"], trip["first_week"]), [(d["slot"], d["cell_key"], d["first_week"]) for d in report["short_deviations"]])

    def test_the_impossible_pairs_are_alerts_and_the_slow_pair_is_not(self):
        alerts = self.key["oracle"]["A10"]["numbers"][self.plants("impossible_speed")[0]["number"]]["speed_alerts"]
        self.assertEqual(len(alerts), 2, "the other calls of the subject are far enough away never to make a second alert")
        for plant in self.plants("impossible_speed"):
            first, second = sorted(e["start_local"] for e in plant["events"])
            hit = [a for a in alerts if (a["from_time"], a["to_time"]) == (first, second)]
            self.assertEqual(len(hit), 1)
            self.assertEqual(hit[0]["gap_s"], 90)
            self.assertGreater(hit[0]["speed_kmh"], 300)
        slow = self.plants("slow_decoy")[0]
        first, second = sorted(e["start_local"] for e in slow["events"])
        self.assertFalse([a for a in alerts if a["from_time"] >= first and a["to_time"] <= second])

    def test_only_the_planted_rhythm_is_a_change_and_the_unusual_day_is_not(self):
        changes = {n: r["change"] for n, r in self.key["oracle"]["A11"]["numbers"].items() if r["change"]}
        plant = self.plants("rate_change")[0]
        self.assertEqual(list(changes), [plant["number"]])
        change = changes[plant["number"]]
        self.assertEqual(change["direction"], "up")
        self.assertLessEqual(abs((datetime.date.fromisoformat(change["first_day"]) - datetime.date.fromisoformat(plant["from_date"])).days), 5)
        self.assertGreater(change["ratio"], 1.5)
        outlier = self.plants("outlier_day")[0]
        self.assertEqual(outlier["number"], plant["number"])
        self.assertGreater(abs((datetime.date.fromisoformat(change["first_day"]) - datetime.date.fromisoformat(outlier["day"])).days), 3)

    def test_the_planted_speed_calls_have_no_neighbours_within_half_an_hour(self):
        for plant in self.plants("impossible_speed") + self.plants("slow_decoy"):
            pid = next(p.pid for p in self.world.persons if p.msisdn == plant["number"])
            planted = [e for e in self.world.events if e.plant and e.plant.startswith("P8-") and pid in (e.orig, e.recip)]
            others = [e for e in self.world.events if pid in (e.orig, e.recip) and not (e.plant or "").startswith("P8-")]
            self.assertTrue(planted)
            for e in planted:
                for o in others:
                    self.assertGreater(abs((o.start - e.start).total_seconds()), 1800, (plant["id"], o.uid))

    def test_every_week_of_the_p8_subjects_has_enough_night_events_for_an_anchor(self):
        base = datetime.datetime.combine(datetime.date.fromisoformat(self.params.start_date), datetime.time(0, 0))
        for pid in self.world.roles["p8"]:
            rows = self.renders[pid].visible
            per_week = {}
            for r in rows:
                moment = datetime.datetime.fromisoformat(r["start_local"])
                if moment.hour >= 22 or moment.hour < 7:
                    per_week[(moment - base).days // 7] = per_week.get((moment - base).days // 7, 0) + 1
            weeks = self.params.period_days // 7
            self.assertTrue(all(per_week.get(wk, 0) >= 3 for wk in range(weeks)), (pid, sorted(per_week.items())))

    def test_the_unusual_day_has_far_more_events_than_a_usual_one(self):
        plant = self.plants("outlier_day")[0]
        pid = next(p.pid for p in self.world.persons if p.msisdn == plant["number"])
        per_day = {}
        for r in self.renders[pid].visible:
            per_day[r["start_local"][:10]] = per_day.get(r["start_local"][:10], 0) + 1
        before = sorted(n for day, n in per_day.items() if day < self.plants("rate_change")[0]["from_date"])     # the days before the rhythm changes
        usual = before[len(before) // 2]
        self.assertGreater(per_day[plant["day"]], 2 * usual)

    def test_the_audit_passes_with_movement(self):
        self.assertEqual(audit.audit(self.root), [])

    def test_a_wrongly_keyed_move_or_alert_is_noticed(self):
        with tempfile.TemporaryDirectory() as d:
            root = generate.write_case(self.params, d)
            key_path = os.path.join(root, "key", "key.json")
            with open(key_path, encoding="utf-8") as handle:
                key = json.load(handle)
            for p in key["plants"]:
                if p["type"] == "relocation":
                    p["from_day"] = 0
            number = next(p["number"] for p in key["plants"] if p["type"] == "impossible_speed")
            key["oracle"]["A10"]["numbers"][number]["speed_alerts"].pop()
            with open(key_path, "w", encoding="utf-8") as handle:
                json.dump(key, handle)
            problems = audit.audit(root)
            self.assertTrue(any("planted move" in p for p in problems))
            self.assertTrue(any("speed alerts in the key differ" in p for p in problems))

    def test_anchors_and_speeds_by_hand(self):
        base = datetime.datetime(2026, 2, 2)    # a Monday
        place = {"K1": (0.0, 0.0), "K2": (0.0, 0.09), "K3": (0.09, 0.0), "K9": (0.0, 0.18)}

        def row(when, cell):
            return {"start_local": when.isoformat(sep=" "), "uid": "u%d" % abs(hash((when, cell))), "cell_key": cell, "lat": place[cell][0], "lon": place[cell][1]}

        def nights(week, cell, count=3):
            return [row(base + datetime.timedelta(days=7 * week + k, hours=23), cell) for k in range(count)]
        moved = sum([nights(wk, "K1") for wk in range(4)], []) + sum([nights(wk, "K2") for wk in range(4, 8)], [])
        trip = sum([nights(wk, "K1") for wk in (0, 1, 2, 4, 5, 6)], []) + nights(3, "K2")
        thin = sum([nights(wk, "K1") for wk in (0, 1, 3, 4)], []) + nights(2, "K2", 2)    # the week with 2 events has no anchor
        work = [row(base + datetime.timedelta(days=7 * wk + k, hours=10), "K9") for wk in range(4) for k in range(3)]
        work += [row(base + datetime.timedelta(days=7 * wk + 5, hours=10, minutes=k), "K3") for wk in range(4) for k in range(3)]     # Saturdays are not the day anchor
        answer = oracle.a10_movement({"MOVED": moved, "TRIP": trip, "THIN": thin, "WORK": work}, base)["numbers"]
        self.assertEqual([(c["slot"], c["from_cell"], c["to_cell"], c["first_week"]) for c in answer["MOVED"]["changes"]], [("night", "K1", "K2", 4)])
        self.assertAlmostEqual(answer["MOVED"]["changes"][0]["distance_km"], 10.007, places=2)
        self.assertEqual(answer["MOVED"]["changes"][0]["first_week_start"], "2026-03-02")
        self.assertEqual(answer["TRIP"]["changes"], [])
        self.assertEqual([(d["slot"], d["cell_key"], d["first_week"]) for d in answer["TRIP"]["short_deviations"]], [("night", "K2", 3)])
        self.assertEqual((answer["THIN"]["changes"], answer["THIN"]["short_deviations"]), ([], []))
        self.assertEqual((answer["WORK"]["anchors"]["day"], answer["WORK"]["anchors"]["night"]), ([{"cell_key": "K9", "first_week": 0, "last_week": 3}], []))
        # speed: 10.007 km in 3 minutes is 200 km/h (an alert); in 181 seconds it is not; the same cell twice is never one
        noon = base + datetime.timedelta(hours=12)
        pairs = {"AT": [row(noon, "K1"), row(noon + datetime.timedelta(minutes=3), "K2")], "SLOW": [row(noon, "K1"), row(noon + datetime.timedelta(seconds=181), "K2")],
                 "SAME": [row(noon, "K1"), row(noon + datetime.timedelta(seconds=1), "K1")]}
        alerts = {n: r["speed_alerts"] for n, r in oracle.a10_movement(pairs, base)["numbers"].items()}
        self.assertEqual((len(alerts["AT"]), len(alerts["SLOW"]), len(alerts["SAME"])), (1, 0, 0))
        self.assertAlmostEqual(alerts["AT"][0]["speed_kmh"], 200.15, delta=0.1)

    def test_rhythm_by_hand(self):
        base = datetime.datetime(2026, 2, 2)

        def rows(counts):
            return [{"start_local": (base + datetime.timedelta(days=d, hours=9, minutes=m)).isoformat(sep=" ")} for d, n in enumerate(counts) for m in range(n)]
        steady = [4, 6] * 20
        answer = oracle.a11_rate_change({"UP": rows(steady[:20] + [x * 2 for x in steady[:20]]),
                                         "DOWN": rows([x * 2 for x in steady[:20]] + steady[:20]),
                                         "SPIKE": rows(steady[:20] + [40] + steady[21:]),
                                         "SMALL": rows([10, 11] * 10 + [12, 13] * 10),                    # 10.5 to 12.5 a day: far beyond chance, but a ratio of 1.2
                                         "NOISY": rows([0, 10] * 10 + [0, 20] * 10),                    # a ratio of 2, but the days vary too much for t to reach 4
                                         "SHORT": rows([2] * 8 + [20] * 8)})["numbers"]
        self.assertEqual((answer["UP"]["change"]["first_day"], answer["UP"]["change"]["direction"]), ("2026-02-22", "up"))
        self.assertEqual((answer["DOWN"]["change"]["first_day"], answer["DOWN"]["change"]["direction"]), ("2026-02-22", "down"))
        for name in ("SPIKE", "SMALL", "NOISY", "SHORT"):
            self.assertIsNone(answer[name]["change"], name)


class SessionTests(unittest.TestCase):
    """Internet-record exports (I1 to I4) and application usage (A1)."""
    FIELDS = ("msisdn", "imsi", "imei", "tac", "ue_ip", "ue_port", "public_ip", "public_port_from", "public_port_to", "server_ip", "server_port", "protocol",
              "session_ref", "start_local", "end_local", "nat_start_local", "nat_end_local", "bytes_up", "bytes_down", "packets_up", "packets_down",
              "duration_ms", "cell_key", "apn", "rat", "app_category", "app", "app_feature", "host", "url", "user_agent")

    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.params = w.Params(seed=31, people=60, subjects=4, associates=12, period_days=12, p2_pairs=1, p3_pairs=0)
        cls.world, cls.renders, cls.key = generate.build_case(cls.params)
        cls.root = generate.write_case(cls.params, cls.tmp.name)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def test_each_internet_layout_reads_back_equal_to_the_truth(self):
        names = [n for n in self.renders if n.startswith("ipdr-")]
        self.assertEqual(len(names), 4)
        self.assertEqual({self.renders[n].layout_id for n in names}, set(w.OPERATOR_IPDR_LAYOUTS))
        for name in names:
            rendered = self.renders[name]
            entry = REGISTRY.get(rendered.layout_id)
            with self.subTest(layout=rendered.layout_id):
                self.assertEqual(REGISTRY.find_header(rendered.table).matched_entry_id, rendered.layout_id)
                results = list(interpret_rows(entry, rendered.table[rendered.header_index], rendered.table[rendered.header_index + 1:], policy=POLICY))
                self.assertEqual(len(results), len(rendered.rows))
                self.assertGreater(len(results), 20)
                for result, expected in zip(results, rendered.rows):
                    self.assertEqual(result.status, expected.get("status", "accepted"), "%s %s" % (expected.get("uid"), expected.get("defect")))
                    if result.status != "accepted":
                        self.assertEqual(result.reason, expected["reason"])
                        continue
                    for flag in expected.get("flags", []):
                        self.assertIn(flag, result.flags)
                    for field in self.FIELDS:
                        if field in expected:
                            self.assertEqual(result.record.get(field), expected[field], "%s %s" % (expected["uid"], field))

    def test_application_usage_reads_back_equal_to_the_truth(self):
        names = [n for n in self.renders if n.startswith("usage-")]
        self.assertEqual(len(names), 2)
        for name in names:
            rendered = self.renders[name]
            entry = REGISTRY.get(rendered.layout_id)
            results = list(interpret_rows(entry, rendered.table[0], rendered.table[1:], policy=POLICY))
            self.assertEqual(len(results), len(rendered.rows))
            for result, expected in zip(results, rendered.rows):
                self.assertEqual(result.status, "accepted", result.reason)
                for field in ("msisdn", "imsi", "imei", "tac", "start_local", "end_local", "terminal", "apn", "app_category", "app", "app_feature", "province", "city"):
                    self.assertEqual(result.record.get(field), expected[field], field)

    def test_the_audit_passes_with_sessions(self):
        self.assertEqual(audit.audit(self.root), [])

    def test_no_two_background_sessions_hold_one_address_and_port_at_once(self):
        sessions = sorted((s for s in self.world.sessions if not s.plant), key=lambda s: s.start)
        self.assertGreater(len(sessions), 150)
        for i, a in enumerate(sessions):
            for b in sessions[i + 1:]:
                if b.start > a.nat_end:
                    break
                if a.public_ip == b.public_ip and a.port_from <= b.port_to and b.port_from <= a.port_to:
                    self.fail("two sessions share %s ports %s-%s" % (a.public_ip, a.port_from, a.port_to))

    def test_the_allocator_never_hands_out_a_held_block(self):
        import datetime as dt
        import random
        t0, t1 = dt.datetime(2026, 2, 2, 10, 0), dt.datetime(2026, 2, 2, 10, 30)
        held = {"192.0.2.1": [(t0, t1, 1024, 60000)]}
        for seed in range(5):
            low, high = w.allocate_port_block(held, "192.0.2.1", t0 + dt.timedelta(minutes=5), t1, 64, random.Random(seed))
            self.assertGreater(low, 60000, "a block that overlaps a held one was handed out")
            held["192.0.2.1"].pop()                                   # keep only the first hold for the next seed
        # the same ports are free once the earlier hold is over
        low, _ = w.allocate_port_block({"192.0.2.1": [(t0, t1, 1024, 65534)]}, "192.0.2.1", t1 + dt.timedelta(minutes=1), t1 + dt.timedelta(minutes=9), 1, random.Random(1))
        self.assertGreaterEqual(low, 1024)
        with self.assertRaises(RuntimeError):                          # nothing free while everything is held
            w.allocate_port_block({"192.0.2.1": [(t0, t1, 1024, 65534)]}, "192.0.2.1", t0, t1, 1, random.Random(1))

    def test_port_blocks_follow_the_operators_layout(self):
        operator_of = {p.pid: p.operator for p in self.world.persons}
        for s in (x for x in self.world.sessions if not x.plant):
            width = s.port_to - s.port_from + 1
            self.assertEqual(width, w.PORT_BLOCK.get(operator_of[s.pid], 1))
            self.assertIn(s.public_ip, w.PUBLIC_POOL[operator_of[s.pid]])

    def test_sessions_exist_for_subjects_only(self):
        subjects = {p.pid for p in self.world.persons if p.role == "subject"}
        self.assertEqual({s.pid for s in self.world.sessions if not s.plant}, subjects)

    def test_a_session_file_with_a_forged_address_is_noticed(self):
        with tempfile.TemporaryDirectory() as d:
            root = generate.write_case(self.params, d)
            with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
                item = json.load(handle)["sessions"][0]
            rendered = self.renders[item["name"]]
            table = [list(row) for row in rendered.table]
            columns = table[rendered.header_index]
            table[rendered.header_index + 1][columns.index(next(c for c in columns if c.lower().replace(" ", "_") in ("public_ip", "public_source_ip")))] = "8.8.4.4"
            damaged = render.RenderedFile(rendered.layout_id, rendered.subject, table, rendered.header_index, rendered.numeric_columns, rendered.container,
                                          rendered.sheet_name, rendered.encoding, rendered.delimiter, rendered.rows)
            path = damaged.write(os.path.join(root, "files"), os.path.basename(item["file"]).rsplit(".", 1)[0])
            with open(os.path.join(root, "manifest.json"), encoding="utf-8") as handle:
                manifest = json.load(handle)
            with open(path, "rb") as handle:
                manifest["files"][item["file"]]["sha256"] = hashlib.sha256(handle.read()).hexdigest()
            with open(os.path.join(root, "manifest.json"), "w", encoding="utf-8") as handle:
                json.dump(manifest, handle)
            self.assertTrue(any("documentation blocks" in p for p in audit.audit(root)), audit.audit(root))


class DefectTests(unittest.TestCase):
    """Damaged rows and what each must become."""
    PARAMS = dict(people=60, subjects=6, associates=12, period_days=30, p2_pairs=1, p3_pairs=2, p7_plants=0)

    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.params = w.Params(seed=51, **cls.PARAMS)
        cls.world, cls.renders, cls.key = generate.build_case(cls.params)
        cls.root = generate.write_case(cls.params, cls.tmp.name)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def kinds_in(self, layout_id):
        return {d["kind"] for name, items in self.key["defects"].items() if self.renders[name].layout_id == layout_id for d in items}

    def test_every_kind_appears_somewhere_and_at_most_three_times_per_export(self):
        import defects as defect_module
        seen = {d["kind"] for items in self.key["defects"].values() for d in items}
        self.assertEqual(seen, set(defect_module.KINDS) | set(defect_module.SESSION_KINDS))
        for name, items in self.key["defects"].items():
            for kind in set(defect_module.KINDS) | set(defect_module.SESSION_KINDS):
                self.assertLessEqual(sum(1 for d in items if d["kind"] == kind), 3, (name, kind))

    def test_only_layouts_that_can_show_a_defect_get_it(self):
        self.assertIn("date_order_conflict", self.kinds_in("call-c2-strt-tm"))              # settled slash order
        self.assertNotIn("date_order_conflict", self.kinds_in("call-c1-company"))           # Excel serials have no order
        self.assertIn("self_counterparty", self.kinds_in("geofence-c6-telenor"))            # the only layout that drops it
        self.assertNotIn("self_counterparty", self.kinds_in("call-c4-telenor-style"))
        self.assertNotIn("unknown_label", self.kinds_in("call-c1-company"))                 # its label also carries the event type
        self.assertIn("unknown_label", self.kinds_in("call-c3-ufone-style"))

    def test_no_planted_row_is_damaged(self):
        for name, rendered in self.renders.items():
            for row in rendered.rows:
                if row.get("defect") and row.get("uid"):
                    self.assertFalse(row["plant"], (name, row["uid"]))

    def test_damaged_rows_are_not_visible_events_and_repeats_count_once(self):
        for name, rendered in self.renders.items():
            if not rendered.defects:
                continue
            uids = [(r["uid"], r.get("a_norm", r.get("msisdn"))) for r in rendered.visible]     # a dump shows one event once for each party
            self.assertEqual(len(uids), len(set(uids)), name)
            quarantined = {(r["uid"], r.get("a_norm", r.get("msisdn"))) for r in rendered.rows if r.get("status") == "quarantined"}
            self.assertFalse(quarantined & set(uids), name)
            self.assertEqual(len(rendered.visible), len(rendered.rows) - sum(1 for r in rendered.rows if r.get("status") != "accepted" or r.get("duplicate_of")))

    def test_the_internet_record_exports_are_damaged_too(self):
        import defects as defect_module
        kinds = {d["kind"] for name, items in self.key["defects"].items() if name.startswith("ipdr-") for d in items}
        self.assertEqual(kinds, set(defect_module.SESSION_KINDS))
        flags = {f for name, rendered in self.renders.items() if name.startswith("ipdr-") for row in rendered.rows for f in row.get("flags", [])}
        self.assertEqual(flags, {"ip_invalid:server_ip", "server_port_out_of_range"})
        for name, rendered in self.renders.items():
            if name.startswith("ipquery-"):
                self.assertEqual(rendered.defects, [])      # the probes' evidence is never damaged

    def test_every_damaged_call_record_export_reads_back_to_its_expectation(self):
        for name, rendered in self.renders.items():
            if name.startswith(("ipdr-", "usage-", "ipquery-")):
                continue
            read_back(self, REGISTRY.get(rendered.layout_id), rendered)
        counted = sum(1 for r in self.renders.values() for row in r.rows if row.get("flags") == ["b_party_sentinel"])
        self.assertGreater(counted, 0)

    def test_the_key_lists_what_the_loader_must_quarantine_and_merge(self):
        quarantine_reasons = {q["reason"] for q in self.key["expected_quarantine"]}
        self.assertEqual(quarantine_reasons, {"time_unparsed", "time_missing", "date_order_conflict"})
        self.assertTrue(self.key["expected_merge"])
        self.assertTrue(all(m["duplicate_of"] for m in self.key["expected_merge"]))

    def test_the_audit_passes_with_defects(self):
        self.assertEqual(audit.audit(self.root), [])

    def test_the_oracles_see_only_the_visible_rows(self):
        total = sum(len(r.rows) for r in self.renders.values() if r.defects)
        visible = sum(len(r.visible) for r in self.renders.values() if r.defects)
        self.assertLess(visible, total)

    def test_no_defects_when_switched_off(self):
        params = w.Params(seed=51, defects_per_class=0, **self.PARAMS)
        _, renders, key = generate.build_case(params)
        self.assertEqual(key["defects"], {})
        self.assertEqual(key["expected_quarantine"], [])
        for name, rendered in renders.items():
            self.assertEqual(len(rendered.visible), len(rendered.rows), name)

    def test_a_damaged_row_the_registry_accepts_is_noticed(self):
        with tempfile.TemporaryDirectory() as d:
            root = generate.write_case(self.params, d)
            with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
                written = json.load(handle)
            subject = next(s for s in written["subjects"] if any(e["kind"] == "time_unparsed" for e in written["defects"].get(s["person"], [])))
            rendered = self.renders[subject["person"]]
            entry = REGISTRY.get(rendered.layout_id)
            row_number = next(e["data_row"] for e in written["defects"][subject["person"]] if e["kind"] == "time_unparsed")
            table = [list(r) for r in rendered.table]
            cells = table[rendered.header_index + 1 + row_number]
            cells[[i for i, c in enumerate(cells) if c == "not-a-time"][0]] = "2026-02-03 10:00:00" if "iso_naive" in entry["time"]["formats"] else cells[0]
            fixed = render.RenderedFile(rendered.layout_id, rendered.subject, table, rendered.header_index, rendered.numeric_columns, rendered.container,
                                        rendered.sheet_name, rendered.encoding, rendered.delimiter, rendered.rows)
            path = fixed.write(os.path.join(root, "files"), os.path.basename(subject["file"]).rsplit(".", 1)[0])
            with open(os.path.join(root, "manifest.json"), encoding="utf-8") as handle:
                manifest = json.load(handle)
            with open(path, "rb") as handle:
                manifest["files"][subject["file"]]["sha256"] = hashlib.sha256(handle.read()).hexdigest()
            with open(os.path.join(root, "manifest.json"), "w", encoding="utf-8") as handle:
                json.dump(manifest, handle)
            self.assertTrue(any("should be quarantined" in p for p in audit.audit(root)), audit.audit(root))


class MeetingTests(unittest.TestCase):
    """Plant P4 and the A7 oracle."""
    PARAMS = dict(people=60, subjects=6, associates=12, period_days=30, p2_pairs=1, p3_pairs=2, p7_plants=0)

    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.params = w.Params(seed=11, **cls.PARAMS)
        cls.world, cls.renders, cls.key = generate.build_case(cls.params)
        cls.root = generate.write_case(cls.params, cls.tmp.name)
        with open(os.path.join(cls.root, "key", "key.json"), encoding="utf-8") as handle:
            cls.written = json.load(handle)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def plants(self):
        return [p for p in self.key["plants"] if p["type"] == "possible_meeting"]

    def test_one_meeting_plant_per_co_location_plant(self):
        self.assertEqual(len(self.plants()), 2)
        for plant in self.plants():
            self.assertEqual(len(plant["planted_meetings"]), 2)
            self.assertEqual({n["kind"] for n in plant["negatives"]}, {"contact_too_early", "co_location_without_contact", "contact_without_co_location"})

    def test_the_planted_meetings_are_the_candidates_and_the_negatives_are_not(self):
        answers = {frozenset((a["x"], a["y"])): a for a in self.key["oracle"]["A7"]}
        for plant in self.plants():
            answer = answers[frozenset((plant["x"]["msisdn"], plant["y"]["msisdn"]))]
            starts = {c["start_local"] for c in answer["candidates"]}
            self.assertEqual(starts, {m["window_start"] for m in plant["planted_meetings"]})
            for negative in plant["negatives"]:
                if negative["window_start"]:
                    self.assertNotIn(negative["window_start"], starts, negative["kind"])
            for candidate in answer["candidates"]:
                self.assertLessEqual(candidate["gap_minutes"], 60)
                self.assertTrue(0 < candidate["score"] <= 1)

    def test_the_pairs_have_no_background_contact(self):
        for plant in self.world.p3:
            both = {plant["x"], plant["y"]}
            direct = [e for e in self.world.events if {e.orig, e.recip} == both]
            self.assertTrue(direct)
            self.assertTrue(all(e.plant for e in direct))

    def test_the_pairs_have_no_background_contact_even_when_they_share_a_community(self):
        removed = 0
        for seed in range(1, 21):
            world = w.build_world(w.Params(seed=seed, people=60, subjects=8, associates=12, communities=4, period_days=20, p3_pairs=4))
            removed += world.removed_contacts
            for plant in world.p3:
                both = {plant["x"], plant["y"]}
                self.assertTrue(all(e.plant for e in world.events if {e.orig, e.recip} == both), (seed, plant["id"]))
        self.assertGreater(removed, 0, "in twenty cases some pair was linked in the background, and its contacts were removed")

    def test_a_contact_is_never_itself_a_co_location(self):
        for plant in self.world.p3:
            contacts = [e for e in self.world.events if {e.orig, e.recip} == {plant["x"], plant["y"]}]
            for e in contacts:
                self.assertNotEqual(e.cells[plant["x"]], e.cells[plant["y"]])

    def test_the_audit_passes_with_meetings(self):
        self.assertEqual(audit.audit(self.root), [])

    def test_a_missing_planted_contact_is_noticed(self):
        with tempfile.TemporaryDirectory() as d:
            root = generate.write_case(self.params, d)
            plant = self.plants()[0]
            tag = "%s:m1" % plant["pair"]
            keep = {e.uid for e in self.world.events if e.plant != tag}
            with open(os.path.join(root, "manifest.json"), encoding="utf-8") as handle:
                manifest = json.load(handle)
            for side in ("x", "y"):
                subject = next(s for s in self.written["subjects"] if s["msisdn"] == plant[side]["msisdn"])
                damaged = render.render_subject(self.world, REGISTRY.get(subject["layout_id"]), self.world.person(subject["person"]), only_events=keep)
                path = damaged.write(os.path.join(root, "files"), os.path.basename(subject["file"]).rsplit(".", 1)[0])
                with open(path, "rb") as handle:
                    manifest["files"][subject["file"]]["sha256"] = hashlib.sha256(handle.read()).hexdigest()
            with open(os.path.join(root, "manifest.json"), "w", encoding="utf-8") as handle:
                json.dump(manifest, handle)
            problems = audit.audit(root)
            self.assertTrue(any("A7" in p or "planted meeting" in p for p in problems), problems)

    def test_the_meeting_oracle_by_hand(self):
        window = {"cell_key": "A", "start_local": "2026-02-03 10:00:00", "end_local": "2026-02-03 10:12:00", "pairs": 1}
        three = dict(window, pairs=3)

        def contacts(*times, y="Y", x="X"):
            return ([{"start_local": t, "b_norm": y} for t in times], [])
        rx, ry = contacts("2026-02-03 09:30:00")
        answer = oracle.a7_meetings(rx, ry, "X", "Y", [window], 60)
        self.assertEqual([(c["gap_minutes"], c["score"]) for c in answer["candidates"]], [(30.0, 0.4167)])
        rx, ry = contacts("2026-02-03 08:59:59")
        self.assertEqual(oracle.a7_meetings(rx, ry, "X", "Y", [window], 60)["candidates"], [])          # one second over the gap
        rx, ry = contacts("2026-02-03 09:00:00")
        self.assertEqual(len(oracle.a7_meetings(rx, ry, "X", "Y", [window], 60)["candidates"]), 1)      # exactly the gap
        rx, ry = contacts("2026-02-03 10:00:00")
        self.assertEqual(oracle.a7_meetings(rx, ry, "X", "Y", [three], 60)["candidates"][0]["score"], 1.0)  # contact at the start, three pairs
        rx, ry = contacts("2026-02-03 10:00:01")
        self.assertEqual(oracle.a7_meetings(rx, ry, "X", "Y", [window], 60)["candidates"], [])          # after the window started
        own = [{"start_local": "2026-02-03 09:45:00", "b_norm": "Z"}]                                   # a contact with someone else is no contact
        self.assertEqual(oracle.a7_meetings(own, [], "X", "Y", [window], 60)["candidates"], [])
        from_y = [{"start_local": "2026-02-03 09:50:00", "b_norm": "X"}]                                 # the other side's file shows it too
        self.assertEqual(len(oracle.a7_meetings([], from_y, "X", "Y", [window], 60)["candidates"]), 1)


class AddressTranslationTests(unittest.TestCase):
    """Plant P7 and the A13 attribution."""

    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.params = w.Params(seed=41, people=60, subjects=4, associates=12, period_days=12, p2_pairs=1, p3_pairs=0)
        cls.world, cls.renders, cls.key = generate.build_case(cls.params)
        cls.root = generate.write_case(cls.params, cls.tmp.name)
        with open(os.path.join(cls.root, "key", "key.json"), encoding="utf-8") as handle:
            cls.written = json.load(handle)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def plants(self):
        return [p for p in self.key["plants"] if p["type"] == "address_translation"]

    def test_three_plants_on_operators_whose_layout_gives_a_session_end(self):
        self.assertEqual(len(self.plants()), 3)
        self.assertEqual({p["operator"] for p in self.plants()}, {"zong", "mobilink", "telenor"})
        self.assertEqual(len(self.key["ip_queries"]), 3)

    def test_every_probe_shows_what_was_planted(self):
        for plant in self.plants():
            self.assertEqual(len(plant["probes"]), 7)
            for probe in plant["probes"]:
                self.assertEqual(probe["result"], probe["intent"], probe["label"])
                self.assertEqual(probe["holders"], probe["intended_holders"], probe["label"])
            self.assertEqual([p["result"] for p in plant["probes"]].count("ambiguous"), 1)
            self.assertEqual([p["result"] for p in plant["probes"]].count("none"), 1)
        self.assertEqual(len(self.key["oracle"]["A13"]), 21)

    def test_the_query_exports_read_back_and_the_audit_passes(self):
        self.assertEqual(audit.audit(self.root), [])
        for query in self.key["ip_queries"]:
            rendered = self.renders[query["name"]]
            entry = REGISTRY.get(rendered.layout_id)
            results = list(interpret_rows(entry, rendered.table[rendered.header_index], rendered.table[rendered.header_index + 1:], policy=POLICY))
            self.assertEqual(len(results), 6)
            self.assertTrue(all(r.status == "accepted" and r.record["public_ip"] == query["public_ip"] for r in results))

    def test_the_zong_query_carries_its_criteria_in_the_preamble(self):
        zong = next(q for q in self.key["ip_queries"] if q["operator"] == "zong")
        table = self.renders[zong["name"]].table
        text = " | ".join(",".join(row) for row in table[:self.renders[zong["name"]].header_index])
        self.assertIn(zong["public_ip"], text)
        self.assertIn(zong["window_start"], text)

    def test_the_two_plants_that_hold_a_port_back_to_back_are_unique_either_side_of_the_second(self):
        for plant in self.plants():
            labels = {p["label"]: p for p in plant["probes"]}
            first = labels["back to back: last second of the first"]
            second = labels["back to back: first second of the second"]
            self.assertEqual((first["result"], second["result"]), ("unique", "unique"))
            self.assertNotEqual(first["holders"], second["holders"])

    def test_attribution_by_hand_with_ranges_and_boundaries(self):
        sessions = [
            {"public_ip": "X", "port_from": 1000, "port_to": 1063, "begin": "2026-02-02 10:00:00", "end": "2026-02-02 10:20:00", "msisdn": "A"},
            {"public_ip": "X", "port_from": 1032, "port_to": 1095, "begin": "2026-02-02 10:10:00", "end": "2026-02-02 10:30:00", "msisdn": "B"},
            {"public_ip": "X", "port_from": 1032, "port_to": 1095, "begin": "2026-02-02 10:12:00", "end": "2026-02-02 10:13:00", "msisdn": "B"},
        ]
        who = lambda port, when, ip="X": oracle.a13_holders(sessions, ip, port, when)
        self.assertEqual(who(1040, "2026-02-02 10:15:00"), {"result": "ambiguous", "holders": ["A", "B"]})
        self.assertEqual(who(1010, "2026-02-02 10:15:00"), {"result": "unique", "holders": ["A"]})
        self.assertEqual(who(1070, "2026-02-02 10:25:00"), {"result": "unique", "holders": ["B"]})
        self.assertEqual(who(1040, "2026-02-02 10:20:00")["result"], "ambiguous")   # the last second counts
        self.assertEqual(who(1040, "2026-02-02 10:20:01"), {"result": "unique", "holders": ["B"]})
        self.assertEqual(who(1040, "2026-02-02 09:59:59")["result"], "none")
        self.assertEqual(who(1040, "2026-02-02 10:15:00", ip="Y")["result"], "none")
        self.assertEqual(who(1032, "2026-02-02 10:12:30"), {"result": "ambiguous", "holders": ["A", "B"]})
        self.assertEqual(who(1100, "2026-02-02 10:15:00")["result"], "none")           # one past the block
        self.assertEqual(who(1095, "2026-02-02 10:25:00"), {"result": "unique", "holders": ["B"]})  # the last port of the block

    def test_removing_the_colliding_holder_from_the_export_is_noticed(self):
        with tempfile.TemporaryDirectory() as d:
            root = generate.write_case(self.params, d)
            query = self.written["ip_queries"][0]
            plant = next(p for p in self.world.p7 if p["id"] == query["plant"])
            rendered = self.renders[query["name"]]
            keep = [i for i, row in enumerate(rendered.rows) if not (row["plant"] or "").endswith("collision-b")]
            self.assertEqual(len(keep), len(rendered.rows) - 1)
            table = rendered.table[:rendered.header_index + 1] + [rendered.table[rendered.header_index + 1 + i] for i in keep]
            damaged = render.RenderedFile(rendered.layout_id, rendered.subject, table, rendered.header_index, rendered.numeric_columns, rendered.container,
                                          rendered.sheet_name, rendered.encoding, rendered.delimiter, [rendered.rows[i] for i in keep])
            path = damaged.write(os.path.join(root, "files"), os.path.basename(query["file"]).rsplit(".", 1)[0])
            with open(os.path.join(root, "manifest.json"), encoding="utf-8") as handle:
                manifest = json.load(handle)
            with open(path, "rb") as handle:
                manifest["files"][query["file"]]["sha256"] = hashlib.sha256(handle.read()).hexdigest()
            with open(os.path.join(root, "manifest.json"), "w", encoding="utf-8") as handle:
                json.dump(manifest, handle)
            problems = audit.audit(root)
            self.assertTrue(any("probe 'collision'" in p for p in problems), problems)


class DumpTests(unittest.TestCase):
    """Tower dumps (C5 to C8) and the A6 population."""

    @classmethod
    def setUpClass(cls):
        cls.tmp = tempfile.TemporaryDirectory()
        cls.params = w.Params(seed=21, people=60, subjects=4, associates=12, period_days=20, p2_pairs=2, p3_pairs=0)
        cls.world, cls.renders, cls.key = generate.build_case(cls.params)
        cls.root = generate.write_case(cls.params, cls.tmp.name)

    @classmethod
    def tearDownClass(cls):
        cls.tmp.cleanup()

    def test_one_dump_per_operator_in_its_own_layout(self):
        self.assertEqual([d["layout_id"] for d in self.key["dumps"]], ["geofence-c7-ufone", "geofence-c8-zong", "geofence-c5-mobilink", "geofence-c6-telenor"])
        for dump in self.key["dumps"]:
            self.assertGreater(dump["rows"], 20)
        self.assertEqual(audit.audit(self.root), [])

    def test_each_dump_reads_back_equal_to_the_truth(self):
        for dump in self.key["dumps"]:
            rendered = self.renders[dump["name"]]
            self.assertEqual(REGISTRY.find_header(rendered.table).matched_entry_id, dump["layout_id"])
            read_back(self, REGISTRY.get(dump["layout_id"]), rendered)

    def test_a_layout_that_cannot_say_a_thing_says_unknown_and_the_key_says_so(self):
        telenor = self.renders["dump-telenor"]       # C6: numeric direction codes with no confirmed meaning
        self.assertEqual({r["direction"] for r in telenor.visible}, {"unknown"})
        self.assertEqual({r["true_direction"] for r in telenor.visible}, {"incoming", "outgoing"})
        zong = self.renders["dump-zong"]             # C8: one activity code
        self.assertEqual({r["event_type"] for r in zong.visible}, {"other"})
        ufone = self.renders["dump-ufone"]           # C7: no call-type column at all
        self.assertEqual({r["event_type"] for r in ufone.visible}, {"other"})
        items = {(a["layout_id"], a["item"]) for a in self.key["layout_assumptions"]}
        self.assertTrue(any(layout == "geofence-c6-telenor" and item.startswith("enum:INBOUND_OUTBOUND_IND=") for layout, item in items))

    def test_the_population_includes_people_who_are_not_subjects(self):
        subjects = {s["msisdn"] for s in self.key["subjects"]}
        for answer in self.key["oracle"]["A6"]:
            numbers = {p["number"] for p in answer["population"]}
            self.assertTrue(numbers - subjects, answer["dump"])

    def test_the_c5_dump_has_no_cells_and_says_so(self):
        c5 = next(a for a in self.key["oracle"]["A6"] if a["dump"] == "dump-mobilink")
        self.assertIsNone(c5["at_two_or_more_cells"])
        self.assertIn("no cell column", c5["note"])
        self.assertTrue(all(not p["cells"] for p in c5["population"]))

    def test_rows_stay_inside_the_cells_and_the_window(self):
        for dump in self.key["dumps"]:
            rows = self.renders[dump["name"]].visible
            self.assertTrue(all(dump["window_start"] <= r["start_local"] <= dump["window_end"] for r in rows), dump["name"])
            wanted = [k for k in dump["cell_keys"] if k]
            if wanted:
                self.assertTrue(all(r["cell_key"] in wanted for r in rows), dump["name"])

    def test_a_dump_with_a_row_missing_is_noticed(self):
        with tempfile.TemporaryDirectory() as d:
            root = generate.write_case(self.params, d)
            with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
                dump = json.load(handle)["dumps"][1]
            rendered = self.renders[dump["name"]]
            rendered = render.RenderedFile(rendered.layout_id, rendered.subject, rendered.table[:-1], rendered.header_index, rendered.numeric_columns,
                                           rendered.container, rendered.sheet_name, rendered.encoding, rendered.delimiter, rendered.rows[:-1])
            path = rendered.write(os.path.join(root, "files"), os.path.basename(dump["file"]).rsplit(".", 1)[0])
            with open(os.path.join(root, "manifest.json"), encoding="utf-8") as handle:
                manifest = json.load(handle)
            with open(path, "rb") as handle:
                manifest["files"][dump["file"]]["sha256"] = hashlib.sha256(handle.read()).hexdigest()
            with open(os.path.join(root, "manifest.json"), "w", encoding="utf-8") as handle:
                json.dump(manifest, handle)
            self.assertTrue(any("rows read" in p or "population" in p for p in audit.audit(root)), audit.audit(root))

    def test_the_population_oracle_by_hand(self):
        rows = [{"a_norm": "n1", "cell_key": "A"}, {"a_norm": "n1", "cell_key": "B"}, {"a_norm": "n1", "cell_key": "A"},
                {"a_norm": "n2", "cell_key": "A"}, {"a_norm": "n3", "cell_key": "Z"}]
        answer = oracle.a6_population(rows, ["A", "B"])
        self.assertEqual(answer["at_two_or_more_cells"], ["n1"])
        self.assertEqual({p["number"]: p["cells"] for p in answer["population"]}, {"n1": {"A": 2, "B": 1}, "n2": {"A": 1}, "n3": {}})
        blind = oracle.a6_population(rows, [None])
        self.assertIsNone(blind["at_two_or_more_cells"])


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
