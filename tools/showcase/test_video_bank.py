"""Tests of the video part of the verified bank: what goes into the public bank and what stays private, and how the judge scores the video questions.

    python -I -X utf8 -m unittest tools/showcase/test_video_bank.py

The plates here are made up. No database or engine is needed.
"""
import json
import os
import sys
import tempfile
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)

import build_verified_bank as builder  # noqa: E402
import judge_verified_bank as judge  # noqa: E402

PLATE_A, PLATE_B, SHORT = "LEA1234", "ABC567", "AB12"


def question(qid, kind, expected, **extra):
    return dict(id=qid, q="Question %s?" % qid, kind=kind, expected=expected, expect_view=extra.pop("expect_view", "v_plate_reads"), **extra)


class PublicAndPrivateSplit(unittest.TestCase):
    def test_a_set_of_plates_is_private_and_a_set_of_files_is_not(self):
        self.assertTrue(builder.holds_plates(question("a", "set", [PLATE_A, PLATE_B])))
        self.assertTrue(builder.holds_plates(question("a", "top1", [PLATE_A])))
        self.assertFalse(builder.holds_plates(question("a", "set", ["one.mp4", "two.MOV"])))
        self.assertFalse(builder.holds_plates(question("a", "number", 3)))
        self.assertFalse(builder.holds_plates(question("a", "none", [])))

    def test_the_public_item_of_a_plate_question_keeps_only_the_count(self):
        bank = builder.Bank()
        builder.add_video(bank, question("window-x", "set", [PLATE_A, PLATE_B], what="plates"))
        item = bank.items[0]
        self.assertEqual(item["area"], "videos")
        self.assertEqual(item["expect"]["n"], 2)
        self.assertTrue(item["expect"]["private"])
        self.assertNotIn("value", item["expect"])
        self.assertEqual(bank.private[item["id"]], [PLATE_A, PLATE_B])
        text = json.dumps(bank.items)
        self.assertNotIn(PLATE_A, text)
        self.assertNotIn(PLATE_B, text)

    def test_a_number_or_a_file_list_is_public_with_its_tolerance(self):
        bank = builder.Bank()
        builder.add_video(bank, question("length-x", "number", 31.5, tolerance=0.06, expect_view="v_video_files"))
        builder.add_video(bank, question("longest-video", "top1", ["long.mp4"], expect_view="v_video_files"))
        self.assertEqual(bank.items[0]["expect"]["value"], 31.5)
        self.assertEqual(bank.items[0]["expect"]["tolerance"], 0.06)
        self.assertEqual(bank.items[1]["expect"]["value"], ["long.mp4"])
        self.assertEqual(bank.private, {})

    def test_ids_are_stable_and_numbered_in_order(self):
        bank = builder.Bank()
        for n in range(3):
            builder.add_video(bank, question("q%d" % n, "number", n))
        self.assertEqual([i["id"] for i in bank.items], ["VB-VIDE-001", "VB-VIDE-002", "VB-VIDE-003"])

    def test_typed_searches_name_every_clip_that_holds_the_plate(self):
        groups = [dict(f="a.mp4", plate=PLATE_A, first=1, last=9), dict(f="a.mp4", plate=PLATE_B, first=2, last=4), dict(f="b.mp4", plate=PLATE_A, first=3, last=5), dict(f="b.mp4", plate="XYZ999", first=3, last=4)]
        original = builder.vqb.psql_rows
        builder.vqb.psql_rows = lambda collection, sql: groups
        try:
            searches = builder.typed_plate_searches("c")
        finally:
            builder.vqb.psql_rows = original
        self.assertEqual(searches, [(PLATE_A, ["a.mp4", "b.mp4"]), (PLATE_A, ["a.mp4", "b.mp4"])])

    def test_the_private_plates_are_collected_for_the_leak_check(self):
        bank = builder.Bank()
        builder.add_video(bank, question("w", "set", [PLATE_A], what="plates"))
        typed = builder.Bank()
        typed.add("vsearch", "typed_plate_search", "Which photos or videos show the plate %s?" % PLATE_B, {"kind": "video", "vkind": "named", "value": ["a.mp4"], "what": "files"})
        bank.typed = typed.items
        self.assertEqual(builder.private_plates(bank), {PLATE_A, PLATE_B})

    def test_the_private_bank_is_refused_inside_the_repository(self):
        inside = os.path.join(builder.REPO, "docs", "qa", "private.json")
        with self.assertRaises(SystemExit):
            builder.main(["--out", os.path.join(tempfile.gettempdir(), "x.json"), "--private-out", inside])


class VideoVerdicts(unittest.TestCase):
    def item(self, expect, qid="VB-VIDE-001"):
        return {"id": qid, "area": "videos", "expect": dict({"kind": "video", "expect_view": "v_plate_reads"}, **expect)}

    def result(self, answer="", rows=None, sql="select 1 from analyst.v_plate_reads"):
        return {"state": "answered", "answer": answer, "rows": rows, "sql": sql}

    def test_a_number_within_its_tolerance_is_correct_and_outside_is_wrong(self):
        item = self.item({"vkind": "number", "value": 31.5, "tolerance": 0.06, "expect_view": "v_video_files"})
        self.assertEqual(judge.verdict(item, self.result(rows=[{"d": 31.53}], sql="v_video_files")), "correct")
        self.assertEqual(judge.verdict(item, self.result(rows=[{"d": 33.0}], sql="v_video_files")), "wrong")

    def test_a_right_number_through_another_view_is_lucky(self):
        item = self.item({"vkind": "number", "value": 4, "expect_view": "v_video_plate_frames"})
        self.assertEqual(judge.verdict(item, self.result(rows=[{"n": 4}], sql="select count(*) from analyst.v_plate_reads")), "lucky")

    def test_a_set_must_be_exact_and_an_extra_plate_is_wrong(self):
        item = self.item({"vkind": "set", "private": True, "n": 2, "what": "plates"})
        private = {"VB-VIDE-001": [PLATE_A, PLATE_B]}
        self.assertEqual(judge.verdict(item, self.result(rows=[[PLATE_A], [PLATE_B]]), private), "correct")
        self.assertEqual(judge.verdict(item, self.result(rows=[[PLATE_A], [PLATE_B], ["QRS789"]]), private), "wrong")
        self.assertEqual(judge.verdict(item, self.result(rows=[[PLATE_A]]), private), "wrong")

    def test_without_the_private_bank_a_plate_question_is_not_run(self):
        item = self.item({"vkind": "set", "private": True, "n": 1, "what": "plates"})
        self.assertEqual(judge.verdict(item, self.result(rows=[[PLATE_A]]), {}), "not run")

    def test_a_typed_search_is_correct_when_every_expected_clip_is_named(self):
        item = self.item({"vkind": "named", "value": ["a.mp4", "b.mp4"], "what": "files"}, "VB-VSEA-001")
        self.assertEqual(judge.verdict(item, self.result(answer="Seen in a.mp4 at 0:12 and in b.mp4 at 0:03.")), "correct")
        self.assertEqual(judge.verdict(item, self.result(answer="Seen in a.mp4 at 0:12.")), "wrong")

    def test_a_typed_plate_search_is_scored_on_the_plate_route_answer_which_has_no_engine_header(self):
        item = dict(self.item({"vkind": "named", "value": ["a.mp4"], "what": "files"}, "VB-VSEA-001"), area="vsearch")
        route = {"state": "declined", "header": "", "older_path_answer": "The plate was read in a.mp4 at 0:12."}
        self.assertEqual(judge.verdict(item, route), "correct")
        self.assertEqual(judge.verdict(item, dict(route, older_path_answer="No reading matches.")), "wrong")
        # an engine that really declined (it sent its header) is still a decline
        self.assertEqual(judge.verdict(item, dict(route, header="state=declined")), "declined")
        # and a question of another family is never rescued this way
        self.assertEqual(judge.verdict(dict(item, area="calls", expect={"kind": "scalar", "value": 3}), route), "declined")

    def test_a_word_search_is_scored_on_the_text_search_answer_unless_it_says_it_is_incomplete(self):
        item = {"id": "VB-SPEE-023", "area": "speech", "type": "audio_word_search", "expect": {"kind": "files", "values": ["voice-note-03.wav", "voice-note-06.wav"], "universe": r"voice-note-\d+\.wav"}}
        route = {"state": "declined", "header": "", "older_path_answer": "voice-note-06.wav mentions it at 0 seconds. 2 recordings carry a matching segment: voice-note-06.wav, voice-note-03.wav."}
        self.assertEqual(judge.verdict(item, route), "correct")
        # a recording missing or one too many is wrong
        self.assertEqual(judge.verdict(item, dict(route, older_path_answer="voice-note-06.wav mentions it at 0 seconds.")), "wrong")
        self.assertEqual(judge.verdict(item, dict(route, older_path_answer=route["older_path_answer"] + " Also voice-note-01.wav.")), "wrong")
        # an answer that says its search is incomplete makes no claim to be the full list: it stays a decline
        incomplete = route["older_path_answer"] + " This search is incomplete; absence of further matches has not been established."
        self.assertEqual(judge.verdict(item, dict(route, older_path_answer=incomplete)), "declined")
        # an engine that really declined (it sent its header) is still a decline, and another question type is never rescued this way
        self.assertEqual(judge.verdict(item, dict(route, header="state=declined")), "declined")
        self.assertEqual(judge.verdict(dict(item, type="audio_length", expect={"kind": "scalar", "value": 3}), route), "declined")

    def test_a_declined_answer_is_not_a_verdict_on_the_content(self):
        item = self.item({"vkind": "number", "value": 3})
        self.assertEqual(judge.verdict(item, {"state": "declined"}), "declined")
        self.assertEqual(judge.verdict(item, {"state": "answered", "timed_out": True}), "timed out")

    def test_the_text_of_a_video_answer_hides_every_known_plate_even_a_short_one(self):
        text = "Plates %s, %s and %s; also %s" % (PLATE_A, PLATE_B, SHORT, "QRS789")
        out = judge.masked(text, {PLATE_A, PLATE_B, SHORT})
        for plate in (PLATE_A, PLATE_B, SHORT, "QRS789"):
            self.assertNotIn(plate, out)


class WholeSheet(unittest.TestCase):
    def test_the_sheet_scores_masks_and_counts(self):
        with tempfile.TemporaryDirectory() as tmp:
            bank = {"collection": "c", "questions": [
                {"id": "VB-CALL-001", "area": "calls", "type": "voice_count", "question": "How many voice calls does 923000001005 have?", "expect": {"kind": "scalar", "value": 7}, "note": ""},
                {"id": "VB-VIDE-001", "area": "videos", "type": "length-x", "question": "How long is x.mp4?", "expect": {"kind": "video", "vkind": "number", "value": 31.5, "tolerance": 0.06, "expect_view": "v_video_files"}, "note": ""},
                {"id": "VB-VIDE-002", "area": "videos", "type": "window-x", "question": "Which plates were read in x.mp4?", "expect": {"kind": "video", "vkind": "set", "private": True, "n": 2, "what": "plates", "expect_view": "v_plate_reads"}, "note": ""}]}
            private = {"collection": "c", "values": {"VB-VIDE-002": [PLATE_A, SHORT]}, "questions": [
                {"id": "VB-VSEA-001", "area": "vsearch", "type": "typed_plate_search", "question": "Which photos or videos show the plate %s?" % PLATE_A, "expect": {"kind": "video", "vkind": "named", "value": ["x.mp4"], "what": "files"}, "note": ""}]}
            results = [
                {"id": "VB-CALL-001", "state": "answered", "answer": "7 calls", "rows": None, "total_ms": 1000},
                {"id": "VB-VIDE-001", "state": "answered", "answer": "31.5 seconds", "rows": [{"d": 31.5}], "sql": "select d from analyst.v_video_files", "total_ms": 2000},
                {"id": "VB-VIDE-002", "state": "answered", "answer": "%s and %s" % (PLATE_A, SHORT), "rows": [[PLATE_A], [SHORT]], "sql": "select p from analyst.v_plate_reads where p = '%s'" % PLATE_A, "total_ms": 3000},
                {"id": "VB-VSEA-001", "state": "answered", "answer": "%s is in x.mp4" % PLATE_A, "rows": None, "total_ms": 4000}]
            paths = {name: os.path.join(tmp, name) for name in ("bank.json", "private.json", "results.jsonl")}
            json.dump(bank, open(paths["bank.json"], "w", encoding="utf-8"))
            json.dump(private, open(paths["private.json"], "w", encoding="utf-8"))
            with open(paths["results.jsonl"], "w", encoding="utf-8") as handle:
                for r in results:
                    handle.write(json.dumps(r) + "\n")
            out = os.path.join(tmp, "sheet")
            self.assertEqual(judge.main([paths["bank.json"], paths["results.jsonl"], "--out", out, "--private-bank", paths["private.json"], "--run-note", "Run note."]), 0)
            written = {ext: open(out + ext, encoding="utf-8").read() for ext in (".json", ".csv", ".md", ".html")}
            for text in written.values():
                self.assertNotIn(PLATE_A, text)
                self.assertNotIn(SHORT, text)
            pack = json.loads(written[".json"])
            verdicts = {q["id"]: q["live"]["verdict"] for q in pack["questions"]}
            self.assertEqual(verdicts, {"VB-CALL-001": "correct", "VB-VIDE-001": "correct", "VB-VIDE-002": "correct", "VB-VSEA-001": "correct"})
            self.assertIn("Run note.", written[".md"])
            self.assertIn("five real video clips", written[".md"])
            self.assertIn("five real video clips", pack["scope"])
            # the same sheet without the private bank: the plate questions are not run, nothing else changes
            out2 = os.path.join(tmp, "sheet2")
            judge.main([paths["bank.json"], paths["results.jsonl"], "--out", out2, "--private-bank", os.path.join(tmp, "absent.json")])
            pack2 = json.loads(open(out2 + ".json", encoding="utf-8").read())
            self.assertEqual({q["id"]: q["live"]["verdict"] for q in pack2["questions"]}, {"VB-CALL-001": "correct", "VB-VIDE-001": "correct", "VB-VIDE-002": "not run"})

    def test_a_bank_without_videos_keeps_its_old_header(self):
        with tempfile.TemporaryDirectory() as tmp:
            bank = {"collection": "c", "questions": [{"id": "VB-CALL-001", "area": "calls", "type": "t", "question": "How many calls?", "expect": {"kind": "scalar", "value": 7}, "note": ""}]}
            json.dump(bank, open(os.path.join(tmp, "b.json"), "w", encoding="utf-8"))
            open(os.path.join(tmp, "r.jsonl"), "w").write(json.dumps({"id": "VB-CALL-001", "state": "answered", "answer": "7", "rows": None, "total_ms": 1}) + "\n")
            judge.main([os.path.join(tmp, "b.json"), os.path.join(tmp, "r.jsonl"), "--out", os.path.join(tmp, "s"), "--private-bank", os.path.join(tmp, "none.json")])
            self.assertIn("Synthetic data only.", open(os.path.join(tmp, "s.md"), encoding="utf-8").read())


if __name__ == "__main__":
    unittest.main()
