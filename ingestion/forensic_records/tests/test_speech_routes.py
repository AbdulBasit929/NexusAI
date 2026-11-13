"""Tests of ingestion/forensic_records/speech_routes.py: the route table, the cutter, the routed transcriber (with fake readers; no model, no audio file).

    python -I -X utf8 ingestion/forensic_records/tests/test_speech_routes.py
"""
import json
import math
import sys
import tempfile
import unittest
from array import array
from pathlib import Path

ROOT = Path(__file__).resolve().parents[3]
sys.path.insert(0, str(ROOT))

from ingestion.forensic_records import speech_routes as sr  # noqa: E402
from ingestion.forensic_records.media_pipeline import MediaObservation  # noqa: E402


def table(**routes):
    return {"version": "t", "routes": routes}


def write_table(directory, data):
    path = Path(directory) / "routes.json"
    path.write_text(json.dumps(data), encoding="utf-8")
    return path


GOOD = {"backend": "omni_ctc", "model_dir": "/models/omni", "status": "SEARCH_LEAD", "measured": {"recall": 0.72}, "evidence": "docs/data/speech-lab-results-20261112.md"}


class RouteTable(unittest.TestCase):
    def test_a_good_table_loads_and_is_looked_up_by_language_code(self):
        with tempfile.TemporaryDirectory() as tmp:
            routes = sr.load_routes(write_table(tmp, table(ps=GOOD, pnb=dict(GOOD, status="LOW_RESOURCE_LANGUAGE"))))
        self.assertEqual(sr.route_for("PS", routes)["status"], "SEARCH_LEAD")
        self.assertIsNone(sr.route_for("en", routes))
        self.assertIsNone(sr.route_for(None, routes))

    def test_a_table_that_could_not_work_is_refused(self):
        for bad in (table(ps=dict(GOOD, status="GREAT")), table(ps=dict(GOOD, backend="magic")), table(ps={k: v for k, v in GOOD.items() if k != "model_dir"}), table(**{"Pashto": GOOD}), {"routes": []}):
            with tempfile.TemporaryDirectory() as tmp, self.assertRaises(sr.RouteError):
                sr.load_routes(write_table(tmp, bad))
        with self.assertRaises(sr.RouteError):
            sr.load_routes("/no/such/file.json")

    def test_the_shipped_table_is_valid(self):
        routes = sr.load_routes()
        self.assertTrue(all(r["status"] in sr.STATUSES for r in routes.values()))


def tone(seconds, rate=16000):
    return array("f", (0.5 * math.sin(2 * math.pi * 220 * i / rate) for i in range(int(seconds * rate))))


class Cutter(unittest.TestCase):
    def test_a_short_recording_is_one_piece(self):
        samples = tone(30)
        self.assertEqual(sr.pieces(samples), [(0, len(samples))])

    def test_a_long_recording_is_cut_in_the_quiet_gap_and_covers_everything_once(self):
        gap = array("f", [0.0] * int(0.4 * 16000))
        samples = tone(30) + gap + tone(40)
        parts = sr.pieces(samples)
        self.assertEqual(parts[0][0], 0)
        self.assertEqual(parts[-1][1], len(samples))
        self.assertTrue(all(a2 == b1 for (_, b1), (a2, _) in zip(parts, parts[1:])))
        self.assertTrue(all((b - a) / 16000 <= 38.0 for a, b in parts))
        self.assertTrue(30.0 <= parts[0][1] / 16000 <= 30.5)

    def test_short_pieces_for_a_model_trained_on_short_clips(self):
        parts = sr.pieces(tone(40), limit=12.0, low=6.0, high=10.8)
        self.assertGreaterEqual(len(parts), 4)
        self.assertTrue(all((b - a) / 16000 <= 12.0 for a, b in parts))
        self.assertTrue(all((b - a) / 16000 >= 6.0 for a, b in parts[:-1]))


class FakeReader:
    def __init__(self, texts):
        self.texts, self.calls = list(texts), 0

    def read(self, samples):
        self.calls += 1
        return (self.texts.pop(0) if self.texts else ""), 0.9


class FakeFallback:
    def __init__(self, language=None):
        self.calls = []
        self.language = language
        self.model = "whisper"

    def transcribe(self, path, *, evidence_id, version_id, source_file, language=None):
        self.calls.append(language)
        return (MediaObservation("x", "c", "audio_transcript_segment", None, {"source_file": source_file}, {"text": "hello", "language": self.language}),)


class Transcriber(unittest.TestCase):
    SPEECH_WITH_A_PAUSE = tone(30) + array("f", [0.0] * int(0.4 * 16000)) + tone(30)       # 60.4 s, a pause at 30 s

    def make(self, texts, fallback=None, samples=None, **kwargs):
        reader = FakeReader(texts)
        routes = {"ps": dict(GOOD, table_version="t1")}
        t = sr.RoutedSpeechTranscriber(fallback or FakeFallback(), routes=routes, reader_factory=lambda route: reader, decoder=lambda path: samples if samples is not None else self.SPEECH_WITH_A_PAUSE, **kwargs)
        return t, reader

    def run_it(self, t, language="ps"):
        return t.transcribe(Path("a.wav"), evidence_id="e", version_id="v", source_file="a.wav", language=language)

    def test_a_routed_language_is_read_in_pieces_with_the_time_of_each_piece_and_the_status(self):
        t, reader = self.make(["one", "two"])
        out = self.run_it(t)
        self.assertEqual([o.payload["text"] for o in out], ["one", "two"])
        self.assertEqual(out[0].payload["start_seconds"], 0.0)
        self.assertEqual(out[0].payload["end_seconds"], out[1].payload["start_seconds"])
        self.assertAlmostEqual(out[1].payload["end_seconds"], 60.4, places=2)
        self.assertTrue(30.0 <= out[0].payload["end_seconds"] <= 30.5)
        p = out[0].payload
        self.assertEqual((p["reliability_state"], p["language"], p["timing_status"], p["manual_review_required"], p["reading_complete"]), ("SEARCH_LEAD", "ps", "reported", True, True))
        self.assertEqual(p["route"]["measured"], {"recall": 0.72})
        self.assertTrue(any("Search lead" in w for w in out[0].warnings))
        self.assertEqual(out[0].citation_locator["start_seconds"], 0.0)
        self.assertEqual(len({o.observation_id for o in out}), 2)

    def test_an_empty_piece_makes_no_segment(self):
        t, _ = self.make(["one", "  ", "three"], samples=tone(100))
        self.assertEqual([o.payload["text"] for o in self.run_it(t)], ["one", "three"])

    def test_another_language_goes_to_the_wrapped_client_untouched(self):
        fallback = FakeFallback()
        t, reader = self.make([], fallback)
        out = self.run_it(t, language="ur")
        self.assertEqual(fallback.calls, ["ur"])
        self.assertEqual(reader.calls, 0)
        self.assertEqual(out[0].payload["text"], "hello")

    def test_no_language_requested_and_the_client_hears_a_routed_language_reads_again_by_the_route(self):
        fallback = FakeFallback(language="ps")
        t, reader = self.make(["pashto text"], fallback, samples=tone(10))
        out = self.run_it(t, language=None)
        self.assertEqual(fallback.calls, [None])
        self.assertEqual(out[0].payload["text"], "pashto text")
        self.assertEqual((out[0].payload["requested_language"], out[0].payload["detected_language"]), (None, "ps"))

    def test_no_language_requested_and_an_unrouted_language_heard_keeps_the_client_result(self):
        fallback = FakeFallback(language="en")
        t, reader = self.make([], fallback)
        self.assertEqual(self.run_it(t, language=None)[0].payload["text"], "hello")
        self.assertEqual(reader.calls, 0)

    def test_a_reading_that_runs_out_of_time_says_so_and_keeps_what_it_read(self):
        ticks = iter([0.0, 5.0, 99.0, 99.0, 99.0, 99.0])       # the start, the check before piece 2 (in time), the check before piece 3 (over the budget)
        t, reader = self.make(["one", "two", "three"], samples=tone(150), budget_seconds=10.0, clock=lambda: next(ticks))
        out = self.run_it(t)
        self.assertEqual(reader.calls, 2)
        self.assertEqual(len(out), 2)
        self.assertFalse(out[0].payload["reading_complete"])
        self.assertTrue(any("time budget" in w for w in out[0].warnings))

    def test_the_wrapped_clients_attributes_stay_reachable(self):
        t, _ = self.make([])
        self.assertEqual(t.model, "whisper")


class WorkerPatch(unittest.TestCase):
    sys.path.insert(0, str(ROOT / "tools" / "media_data"))

    def test_the_repository_media_pipeline_wraps_the_asr_client_behind_the_switch(self):
        text = (ROOT / "ingestion" / "forensic_records" / "media_pipeline.py").read_text(encoding="utf-8")
        self.assertEqual(text.count("RoutedSpeechTranscriber(audio_transcriber)"), 1)
        self.assertIn('FORENSIC_ASR_ROUTES_ENABLED", "false"', text)

    def test_the_patch_adds_the_wrap_once_keeps_crlf_and_refuses_twice_or_none(self):
        import patch_image_media_pipeline_speech_routes as patcher
        crlf, lf = chr(13) + chr(10), chr(10)
        source = crlf.join(["def build():", "    if x:", "        audio_transcriber = LocalAIASRProcessor.from_environment()", "    return 1", ""])
        out = patcher.patch(source)
        self.assertIn("audio_transcriber = RoutedSpeechTranscriber(audio_transcriber)", out)
        self.assertNotIn(lf, out.replace(crlf, ""))                       # every line ending is still a CRLF
        compile(out.replace(crlf, lf), "patched", "exec")
        with self.assertRaises(SystemExit):
            patcher.patch(out)                                  # already patched
        with self.assertRaises(SystemExit):
            patcher.patch("nothing here" + lf)                  # no anchor
        with self.assertRaises(SystemExit):
            patcher.patch(source + source)                      # two anchors


if __name__ == "__main__":
    unittest.main()
