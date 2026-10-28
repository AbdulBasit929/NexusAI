"""Measure the mention extractor against the document truth of generated cases (reference cases programme, stage S3a, gates G-M1 to G-M5 of section 10).

    python -X utf8 tools/case_generator/test_doc_oracle.py --dev [--only 1,2]        # development cases 1 to 8 of the register (a Lahore locale added), may be used to tune
    python -X utf8 tools/case_generator/test_doc_oracle.py --judge                   # orion-lahore (seed 5001) and the fresh case (seed 5003): judged ONCE, after the extractor is frozen
    python -X utf8 tools/case_generator/test_doc_oracle.py -v                        # the unit tests (generator and writer facts, no database, a few seconds)

A case is generated, its documents are read by the PRODUCTION document pipeline (`document_pipeline.extract_document`: TXT in three encodings, DOCX with a table), the extractor runs on the
passages it returns, and the (document, kind, normalised value) pairs found are compared with the pairs the generator put in. Counting is per pair: a number written five times in a document is one pair.
The truth is read off the parts the generator wrote; it shares no code with the extractor.
"""
import collections
import json
import os
import shutil
import sys
import tempfile
import time
import unittest
from pathlib import Path

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, HERE)
sys.path.insert(0, REPO)
import case_set  # noqa: E402
import doc_case  # noqa: E402
import docx_writer  # noqa: E402
import generate  # noqa: E402
import pk_locale  # noqa: E402
import world as w  # noqa: E402
from ingestion.forensic_records import mention_extractor  # noqa: E402
from ingestion.forensic_records.document_pipeline import extract_document  # noqa: E402

STRONG = ("msisdn", "national_id", "account", "iccid", "imei")
GATES = {"strong": (0.98, 0.98), "plate": (0.90, 0.95), "place": (0.90, 0.95)}
ORION = dict(seed=5001, people=100, subjects=12, associates=24, period_days=75, communities=5, p2_pairs=5, p3_pairs=2, p7_plants=3)
FRESH = dict(ORION, seed=5003)


def params_of(overrides, **extra):
    return w.Params(**dict(overrides, locale="lahore", **extra))


def read_document(root, entry):
    path = Path(root) / Path(*entry["file"].split("/"))
    return extract_document(path, source_file=entry["file"], evidence_id="11111111-1111-1111-1111-111111111111", version_id="22222222-2222-2222-2222-222222222222", source_sha256="a" * 64)


def measure(root):
    """Per document: truth pairs, found pairs, decoy hits. Returns (rows, summary)."""
    with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
        key = json.load(handle)
    truth, gazetteer = key["document_truth"]["documents"], key["document_truth"]["gazetteer"]
    rows = []
    for entry in key["documents"]:
        result = read_document(root, entry)
        found = mention_extractor.extract_from_passages(result.passages, gazetteer)
        doc = truth[entry["name"]]
        want = {(m["kind"], m["value"]) for m in doc["mentions"]}
        got = {(m["kind"], m["value"]) for m in found}
        decoy_hits = []
        for decoy in doc["decoys"]:
            for m in found:
                if m["surface"] and (m["surface"] in decoy["surface"] or decoy["surface"] in m["surface"]):
                    decoy_hits.append((decoy["surface"], m["kind"], m["surface"]))
        rows.append({"name": entry["name"], "language": entry["language"], "format": entry["format"], "encoding": entry["encoding"], "want": want, "got": got, "decoy_hits": decoy_hits,
                     "readiness": result.readiness, "passages": len(result.passages), "found": found})
    return rows


def tally(rows):
    out = collections.defaultdict(lambda: {"tp": 0, "fn": 0, "fp": 0})
    for row in rows:
        for kind, value in row["want"] & row["got"]:
            out[kind]["tp"] += 1
        for kind, value in row["want"] - row["got"]:
            out[kind]["fn"] += 1
        for kind, value in row["got"] - row["want"]:
            out[kind]["fp"] += 1
    return out


def rate(a, b):
    return 1.0 if a + b == 0 else a / float(a + b)


def report(label, rows):
    stats = tally(rows)
    print("== %s: %d documents, %d pairs in the truth" % (label, len(rows), sum(len(r["want"]) for r in rows)))
    print("   %-12s %5s %5s %5s %8s %9s" % ("kind", "truth", "found", "extra", "recall", "precision"))
    verdict = {}
    for kind in sorted(set(stats) | set(STRONG) | {"plate", "place", "imsi"}):
        s = stats.get(kind, {"tp": 0, "fn": 0, "fp": 0})
        if s["tp"] + s["fn"] + s["fp"] == 0:
            continue
        recall, precision = rate(s["tp"], s["fn"]), rate(s["tp"], s["fp"])
        verdict[kind] = (recall, precision, s)
        print("   %-12s %5d %5d %5d %7.1f%% %8.1f%%" % (kind, s["tp"] + s["fn"], s["tp"], s["fp"], 100 * recall, 100 * precision))
    for row in rows:
        for pair in sorted(row["want"] - row["got"]):
            print("   MISSED  %-26s %s %s" % (row["name"], pair[0], pair[1]))
        for pair in sorted(row["got"] - row["want"]):
            print("   EXTRA   %-26s %s %s" % (row["name"], pair[0], pair[1]))
        for hit in row["decoy_hits"]:
            print("   DECOY   %-26s %s" % (row["name"], hit))
    return verdict


def gates(verdict, rows):
    results = []
    for kind in STRONG:
        if kind in verdict:
            recall, precision, _ = verdict[kind]
            results.append(("G-M1 %s" % kind, recall >= GATES["strong"][0] and precision >= GATES["strong"][1], "recall %.1f%%, precision %.1f%%" % (100 * recall, 100 * precision)))
    for kind, name in (("plate", "G-M2 plate"), ("place", "G-M3 place")):
        if kind in verdict:
            recall, precision, _ = verdict[kind]
            results.append((name, recall >= GATES[kind][0] and precision >= GATES[kind][1], "recall %.1f%%, precision %.1f%%" % (100 * recall, 100 * precision)))
    decoys = sum(len(r["decoy_hits"]) for r in rows)
    results.append(("G-M1 zero decoy extractions", decoys == 0, "%d decoys extracted" % decoys))
    control = [r for r in rows if r["name"].startswith("D14")]
    results.append(("G-M5 control document", all(not r["got"] for r in control), "%d mentions" % sum(len(r["got"]) for r in control)))
    slices = {"Urdu digits and script": lambda r: r["language"] == "ur", "UTF-16 file": lambda r: r["encoding"] == "utf-16", "DOCX": lambda r: r["format"] == "docx"}
    for label, pick in slices.items():
        chosen = [r for r in rows if pick(r)]
        s = tally(chosen)
        tp, fn, fp = (sum(v[k] for v in s.values()) for k in ("tp", "fn", "fp"))
        recall, precision = rate(tp, fn), rate(tp, fp)
        results.append(("G-M4 %s" % label, recall >= GATES["strong"][0] and precision >= GATES["strong"][1], "recall %.1f%%, precision %.1f%% over %d pairs" % (100 * recall, 100 * precision, tp + fn)))
    results.append(("pipeline", all(r["readiness"] in ("READY", "READY_WITH_WARNINGS") and r["passages"] > 0 for r in rows), "%d documents read" % len(rows)))
    return results


def build(params, tmp):
    started = time.time()
    root = generate.write_case(params, tmp, with_anpr=True, with_documents=True)
    return root, time.time() - started


def run(cases, label):
    all_rows, ok = [], True
    for name, params in cases:
        tmp = tempfile.mkdtemp(prefix="docs-")
        try:
            root, seconds = build(params, tmp)
            rows = measure(root)
            report("%s %s (built in %.0f s)" % (label, name, seconds), rows)
            all_rows.extend(rows)
        finally:
            shutil.rmtree(tmp, ignore_errors=True)
    verdict = tally(all_rows)
    summary = {kind: (rate(s["tp"], s["fn"]), rate(s["tp"], s["fp"]), s) for kind, s in verdict.items()}
    print("\n== %s, all cases together" % label)
    for name, passed, detail in gates(summary, all_rows):
        print("   %-34s %s  %s" % (name, "PASS" if passed else "FAIL", detail))
        ok = ok and passed
    print("DOCORACLE=%s" % ("PASS" if ok else "FAIL"))
    return ok


# ------------------------------------------------------------------------------------------------ unit tests
class GeneratorFacts(unittest.TestCase):
    def test_docx_numbering_matches_what_the_pipeline_counts(self):
        blocks = [("p", "first", {}), ("table", [["a", "b"], ["c", "d"]], {}), ("p", "", {}), ("p", "last", {})]
        self.assertEqual(docx_writer.paragraph_numbers(blocks), [[1], [2, 3, 4, 5], [6], [7]])
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "x.docx"
            path.write_bytes(docx_writer.build_docx(blocks))
            result = extract_document(path, source_file="x.docx", evidence_id="e", version_id="v", source_sha256="a" * 64)
        self.assertEqual([passage.locator["paragraph"] for passage in result.passages], [1, 2, 3, 4, 5, 7])

    def test_docx_is_deterministic_and_keeps_urdu(self):
        blocks = [("p", "ابتدائی اطلاعی رپورٹ", {"rtl": True})]
        self.assertEqual(docx_writer.build_docx(blocks), docx_writer.build_docx(blocks))
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "u.docx"
            path.write_bytes(docx_writer.build_docx(blocks))
            result = extract_document(path, source_file="u.docx", evidence_id="e", version_id="v", source_sha256="a" * 64)
        self.assertEqual(result.passages[0].text, "ابتدائی اطلاعی رپورٹ")

    def test_surfaces_fold_back_to_the_value(self):
        for form in range(8):
            for urdu in (False, True):
                text = doc_case.msisdn_surface("923000001000", form, urdu)
                found = mention_extractor.extract_mentions("mobile %s" % text)
                self.assertEqual([m.value for m in found if m.kind == "msisdn"], ["923000001000"], text)
        for form in range(3):
            text = doc_case.nid_surface("00000-1234567-1", form)
            found = mention_extractor.extract_mentions("CNIC %s" % text)
            self.assertEqual([m.value for m in found if m.kind == "national_id"], ["00000-1234567-1"], text)
        for form in range(4):
            text = doc_case.iban_surface("PK00SYNT0000000000002004", form)
            found = mention_extractor.extract_mentions("account %s" % text)
            self.assertEqual([m.value for m in found if m.kind == "account"], ["PK00SYNT0000000000002004"], text)
        for form in range(4):
            text = doc_case.plate_surface("SYNT4024", form)
            found = mention_extractor.extract_mentions("vehicle %s" % text)
            self.assertEqual([m.value for m in found if m.kind == "plate"], ["SYNT4024"], text)

    def test_a_case_without_a_locale_is_refused(self):
        with self.assertRaises(ValueError):
            doc_case.build(w.Params(seed=1), None, {})


def main(argv):
    if "--dev" in argv:
        only = None
        if "--only" in argv:
            only = {int(x) for x in argv[argv.index("--only") + 1].split(",")}
        cases = [("dev-%d" % seed, params_of(dict(overrides, seed=seed))) for kind, seed, overrides in case_set.CASES if kind == "development" and (only is None or seed in only)]
        return 0 if run(cases, "development") else 1
    if "--judge" in argv:
        return 0 if run([("orion-lahore", params_of(ORION)), ("fresh-5003", params_of(FRESH))], "judgement") else 1
    unittest.main(argv=[argv[0]] + [a for a in argv[1:] if a.startswith("-v")])


if __name__ == "__main__":
    sys.exit(main(sys.argv))
