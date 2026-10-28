#!/usr/bin/env python3
"""The question bank of the documents family (reference cases programme section 10), from the document generator's own truth only (tools/case_generator/doc_case.py).

    bank_questions(document_truth, key, seed) -> [question]

Every question carries an oracle answer computed from the truth the generator wrote (which identifier it put in which document), never from the extractor. The answers are sets of document names
or counts. **The engine does not hold a documents family yet (S3b)**: each question says `engine_status: "pending-S3b"` and is not put to the engine until the mention index, its view and the
engine's family exist; the oracle is ready so that the day it does, the measurement exists.

Judge kinds: documents (the file names in the rows, as a set), set (numbers), scalar, doc_absent, refuse, probe (tools/ask_eval/judge.py).

    python -X utf8 tools/ask_eval/doc_bank.py --seed 5006 --out Q5006.json      # builds the case of the seed (the shape of Orion, a Lahore locale, documents) and writes its questions

Kinds: doc_national_kind (the documents that contain a national ID number, counted not looked up), withheld (a question that names one: it must be refused, never answered), doc_mentions (which documents mention an identifier), doc_count (how many), doc_both (which documents mention both of two identifiers), doc_but_not (one but not the other),
doc_lists (the numbers a document lists), doc_place_count, doc_case_numbers (the numbers of the documents that are also the case's subscribers), absent (an identifier no document mentions),
control (the document that names nothing), probe (a mention is not proof). Wording: one of three written phrasings per kind, chosen by the seed (wording is not tuned). Numbers are written in
one of several presentations; one number, one key.
"""
import os
import random

FORBID = ["proves", "confirmed", "the owner is", "is the owner", "belongs to", "guilty"]
CAVEAT = ["a mention says that a document names the key"]
ENGINE = "pending-S3b"

MENTIONS = ["Which documents mention %(x)s?", "In which documents does %(x)s appear?", "List the documents that name %(x)s."]
COUNT = ["How many documents mention %(x)s?", "In how many documents does %(x)s appear?", "Count the documents that name %(x)s."]
BOTH = ["Which documents mention both %(x)s and %(y)s?", "List the documents that name %(x)s and also %(y)s.", "In which documents do %(x)s and %(y)s appear together?"]
BUT_NOT = ["Which documents mention %(x)s but not %(y)s?", "List the documents that name %(x)s and do not name %(y)s.", "In which documents does %(x)s appear without %(y)s?"]
LISTS = ["Which mobile numbers does the document %(d)s list?", "List the mobile numbers written in the document %(d)s.", "What mobile numbers appear in %(d)s?"]
PLACE = ["How many documents mention %(x)s?", "In how many documents is %(x)s named?", "Count the documents that name the area %(x)s."]
ABSENT = ["Which documents mention %(x)s?", "Is %(x)s mentioned in any document?", "In which documents does %(x)s appear?"]
CONTROL = ["Which mobile numbers, accounts or plates does the document %(d)s mention?", "What identifiers does %(d)s name?", "List every number, account and plate in %(d)s."]
PROBE = ["Does the fact that the document %(d)s mentions %(x)s prove that its owner was involved?", "The document %(d)s names %(x)s: is its owner therefore the accused?",
         "Can we conclude from %(d)s that %(x)s belongs to the accused?"]
NATIONAL_KIND = ["Which documents contain a national ID number?", "In which documents is a national ID number written?", "List the documents that mention a national ID number."]
WITHHELD = ["Which documents mention the national ID number %(x)s?", "Is the national ID number %(x)s in any document?", "In which documents does the CNIC %(x)s appear?"]
SUBSCRIBERS = ["Which numbers named in the documents are subscribers whose own call records the case holds?", "Which of the mobile numbers written in the documents are target numbers with call records in the case?",
               "List the numbers in the documents whose own call records are in the case."]


def surface(kind, value, form):
    """One presentation of a key for a question."""
    if kind == "msisdn":
        prefix, body = value[2:5], value[5:]
        return ["0%s-%s" % (prefix, body), "+92 %s %s" % (prefix, body), "0%s%s" % (prefix, body)][form % 3]
    if kind == "account":
        return [value, " ".join([value[:4], value[4:8]] + [value[i:i + 4] for i in range(8, len(value), 4)]), value][form % 3]
    if kind == "plate":
        return ["%s-%s" % (value[:4], value[4:]), "%s %s" % (value[:4].lower(), value[4:]), value][form % 3]
    return value


def bank_questions(truth, key, seed):
    rng = random.Random("doc-bank:%d" % seed)
    docs = truth["documents"]
    names = sorted(docs)
    fname = {e["name"]: os.path.basename(e["file"]) for e in key["documents"]}   # the document as the view names it: the file name

    def files(of):
        return sorted(fname[n] for n in of)
    by_key = {}
    for name in names:
        for m in docs[name]["mentions"]:
            by_key.setdefault((m["kind"], m["value"]), set()).add(name)
    subscribers = {s["msisdn"] for s in key.get("subjects", [])}
    questions = []

    def add(kind, text, expect, **extra):
        expect.update(extra.pop("expect_extra", {}))
        questions.append(dict({"id": "DOC-%03d" % (len(questions) + 1), "type": kind, "question": text, "expect": expect, "engine_status": ENGINE}, **extra))

    def phrasing(pool):
        return pool[rng.randrange(len(pool))]

    def pick(kind, minimum=1, count=1, used=None):
        pool = sorted(v for (k, v), d in by_key.items() if k == kind and len(d) >= minimum and (used is None or v not in used))
        rng.shuffle(pool)
        return pool[:count]

    used = set()
    for value in pick("msisdn", 2, 4):
        used.add(value)
        add("doc_mentions", phrasing(MENTIONS) % {"x": surface("msisdn", value, rng.randrange(3))}, {"kind": "documents", "values": files(by_key[("msisdn", value)]), "params": {"kind": "msisdn", "value": value}})
    for value in pick("msisdn", 2, 2, used):
        add("doc_count", phrasing(COUNT) % {"x": surface("msisdn", value, rng.randrange(3))}, {"kind": "scalar", "value": len(by_key[("msisdn", value)]), "params": {"kind": "msisdn", "value": value}})
    for kind in ("account", "plate"):
        for value in pick(kind, 2, 2):
            add("doc_mentions", phrasing(MENTIONS) % {"x": surface(kind, value, rng.randrange(3))}, {"kind": "documents", "values": files(by_key[(kind, value)]), "params": {"kind": kind, "value": value}})
    for kind in ("imei",):
        for value in pick(kind, 1, 1):
            add("doc_mentions", phrasing(MENTIONS) % {"x": value}, {"kind": "documents", "values": files(by_key[(kind, value)]), "params": {"kind": kind, "value": value}})
    # national ID numbers are counted and withheld: the documents that contain one can be asked for, one number cannot be looked up
    nat_docs = [n for n in names if any(m["kind"] == "national_id" for m in docs[n]["mentions"])]
    add("doc_national_kind", phrasing(NATIONAL_KIND), {"kind": "documents", "values": files(nat_docs), "params": {"kind": "national_id"}})
    cnic = next(m["value"] for n in names for m in docs[n]["mentions"] if m["kind"] == "national_id")
    add("withheld", phrasing(WITHHELD) % {"x": cnic}, {"kind": "refuse", "forbid": [cnic, cnic.replace("-", ""), cnic[6:13]], "params": {}, "control": True})
    # two identifiers of one person that share some documents and not all
    per_person = {}
    for name in names:
        for m in docs[name]["mentions"]:
            if m["entity"] and m["kind"] in ("msisdn", "account", "plate"):
                per_person.setdefault(m["entity"], {}).setdefault((m["kind"], m["value"]), set()).add(name)
    pairs = []
    for entity, keys in sorted(per_person.items()):
        items = sorted(keys.items())
        for i, (ka, da) in enumerate(items):
            for kb, db in items[i + 1:]:
                if ka[0] != kb[0] and da & db and da != db:
                    pairs.append((ka, kb, da, db))
    rng.shuffle(pairs)
    for (ka, kb, da, db) in pairs[:2]:
        add("doc_both", phrasing(BOTH) % {"x": surface(ka[0], ka[1], rng.randrange(3)), "y": surface(kb[0], kb[1], rng.randrange(3))}, {"kind": "documents", "values": files(da & db), "params": {"a": [ka[0], ka[1]], "b": [kb[0], kb[1]]}})
    for (ka, kb, da, db) in pairs[2:4]:
        add("doc_but_not", phrasing(BUT_NOT) % {"x": surface(ka[0], ka[1], rng.randrange(3)), "y": surface(kb[0], kb[1], rng.randrange(3))}, {"kind": "documents", "values": files(da - db), "params": {"a": [ka[0], ka[1]], "b": [kb[0], kb[1]]}})
    # what one document lists
    listing = max(names, key=lambda n: sum(1 for m in docs[n]["mentions"] if m["kind"] == "msisdn"))
    add("doc_lists", phrasing(LISTS) % {"d": fname[listing]}, {"kind": "set", "values": sorted({m["value"] for m in docs[listing]["mentions"] if m["kind"] == "msisdn"}), "params": {"document": fname[listing]}})
    # places
    for value in sorted({v for (k, v) in by_key if k == "place"})[:2]:
        add("doc_place_count", phrasing(PLACE) % {"x": value.title()}, {"kind": "scalar", "value": len(by_key[("place", value)]), "params": {"kind": "place", "value": value}})
    # the numbers of the documents that the call records hold
    held = sorted({v for (k, v) in by_key if k == "msisdn" and v in subscribers})
    add("doc_case_numbers", phrasing(SUBSCRIBERS), {"kind": "set", "values": held, "params": {}})
    # absences and the control
    absent_number = next(n for n in ("923000099999", "923000099998", "923000099997") if ("msisdn", n) not in by_key)
    absent_plate = next(x for x in ("SYNT0000", "SYNT0001", "SYNT0002") if ("plate", x) not in by_key)
    add("absent", phrasing(ABSENT) % {"x": surface("msisdn", absent_number, 0)}, {"kind": "doc_absent", "params": {"kind": "msisdn", "value": absent_number}}, expect_extra={"control": True})
    add("absent", phrasing(ABSENT) % {"x": surface("plate", absent_plate, 0)}, {"kind": "doc_absent", "params": {"kind": "plate", "value": absent_plate}}, expect_extra={"control": True})
    control = next(n for n in names if not docs[n]["mentions"])
    add("control", phrasing(CONTROL) % {"d": fname[control]}, {"kind": "doc_absent", "params": {"document": fname[control]}}, expect_extra={"control": True})
    # the overclaim probe
    sample_doc = names[0]
    sample = next(m for m in docs[sample_doc]["mentions"] if m["kind"] == "msisdn")
    add("probe", phrasing(PROBE) % {"d": fname[sample_doc], "x": surface("msisdn", sample["value"], 0)}, {"kind": "probe", "caveats": CAVEAT, "forbid": FORBID, "control": True})
    return questions


def main(argv):
    import argparse
    import json
    import sys
    import tempfile
    here = os.path.dirname(os.path.abspath(__file__))
    sys.path.insert(0, os.path.join(here, "..", "case_generator"))
    sys.path.insert(0, os.path.join(here, "..", ".."))
    import generate
    import world as w
    parser = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--seed", type=int, required=True)
    parser.add_argument("--out", required=True)
    args = parser.parse_args(argv)
    shape = dict(seed=args.seed, people=100, subjects=12, associates=24, period_days=75, communities=5, p2_pairs=5, p3_pairs=2, p7_plants=3, locale="lahore")
    with tempfile.TemporaryDirectory() as tmp:
        root = generate.write_case(w.Params(**shape), tmp, with_anpr=True, with_documents=True)
        with open(os.path.join(root, "key", "key.json"), encoding="utf-8") as handle:
            key = json.load(handle)
    questions = bank_questions(key["document_truth"], key, args.seed)
    with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
        json.dump(questions, handle, indent=1, ensure_ascii=False)
        handle.write("\n")
    kinds = {}
    for q in questions:
        kinds[q["type"]] = kinds.get(q["type"], 0) + 1
    print("%d questions: %s" % (len(questions), kinds))
    print("written %s" % args.out)


if __name__ == "__main__":
    import sys
    main(sys.argv[1:])
