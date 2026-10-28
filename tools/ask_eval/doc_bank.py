#!/usr/bin/env python3
"""The question bank of the documents family (reference cases programme section 10), from the document generator's own truth only (tools/case_generator/doc_case.py).

    bank_questions(document_truth, key, seed) -> [question]

Every question carries an oracle answer computed from the truth the generator wrote (which identifier it put in which document), never from the extractor. The answers are sets of document names
or counts. **The engine does not hold a documents family yet (S3b)**: each question says `engine_status: "pending-S3b"` and is not put to the engine until the mention index, its view and the
engine's family exist; the oracle is ready so that the day it does, the measurement exists.

Kinds: doc_mentions (which documents mention an identifier), doc_count (how many), doc_both (which documents mention both of two identifiers), doc_but_not (one but not the other),
doc_lists (the numbers a document lists), doc_place_count, doc_case_numbers (the numbers of the documents that are also the case's subscribers), absent (an identifier no document mentions),
control (the document that names nothing), probe (a mention is not proof). Wording: one of three written phrasings per kind, chosen by the seed (wording is not tuned). Numbers are written in
one of several presentations; one number, one key.
"""
import random

FORBID = ["proves", "confirmed", "the owner is", "is the owner", "belongs to", "guilty"]
CAVEAT = ["a mention in a document says that the document names it"]
ENGINE = "pending-S3b"

MENTIONS = ["Which documents mention %(x)s?", "In which documents does %(x)s appear?", "List the documents that name %(x)s."]
COUNT = ["How many documents mention %(x)s?", "In how many documents does %(x)s appear?", "Count the documents that name %(x)s."]
BOTH = ["Which documents mention both %(x)s and %(y)s?", "List the documents that name %(x)s and also %(y)s.", "In which documents do %(x)s and %(y)s appear together?"]
BUT_NOT = ["Which documents mention %(x)s but not %(y)s?", "List the documents that name %(x)s and do not name %(y)s.", "Where does %(x)s appear without %(y)s?"]
LISTS = ["Which mobile numbers does the document %(d)s list?", "List the mobile numbers written in the document %(d)s.", "What mobile numbers appear in %(d)s?"]
PLACE = ["How many documents mention %(x)s?", "In how many documents is %(x)s named?", "Count the documents that name the area %(x)s."]
ABSENT = ["Which documents mention %(x)s?", "Is %(x)s mentioned in any document?", "In which documents does %(x)s appear?"]
CONTROL = ["Which mobile numbers, accounts or plates does the document %(d)s mention?", "What identifiers does %(d)s name?", "List every number, account and plate in %(d)s."]
PROBE = ["Does the fact that the document %(d)s mentions %(x)s prove that its owner was involved?", "The document %(d)s names %(x)s: is its owner therefore the accused?",
         "Can we conclude from %(d)s that %(x)s belongs to the accused?"]
SUBSCRIBERS = ["Which of the mobile numbers written in the documents are numbers that call records of the case hold?", "Which numbers named in the documents also appear in the call records?",
               "List the numbers in the documents that are in the case's call records."]


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
        add("doc_mentions", phrasing(MENTIONS) % {"x": surface("msisdn", value, rng.randrange(3))}, {"kind": "set", "values": sorted(by_key[("msisdn", value)])})
    for value in pick("msisdn", 2, 2, used):
        add("doc_count", phrasing(COUNT) % {"x": surface("msisdn", value, rng.randrange(3))}, {"kind": "scalar", "value": len(by_key[("msisdn", value)])})
    for kind in ("account", "plate"):
        for value in pick(kind, 2, 2):
            add("doc_mentions", phrasing(MENTIONS) % {"x": surface(kind, value, rng.randrange(3))}, {"kind": "set", "values": sorted(by_key[(kind, value)])})
    for kind in ("national_id", "imei"):
        for value in pick(kind, 1, 1):
            add("doc_mentions", phrasing(MENTIONS) % {"x": value}, {"kind": "set", "values": sorted(by_key[(kind, value)])})
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
        add("doc_both", phrasing(BOTH) % {"x": surface(ka[0], ka[1], rng.randrange(3)), "y": surface(kb[0], kb[1], rng.randrange(3))}, {"kind": "set", "values": sorted(da & db)})
    for (ka, kb, da, db) in pairs[2:4]:
        add("doc_but_not", phrasing(BUT_NOT) % {"x": surface(ka[0], ka[1], rng.randrange(3)), "y": surface(kb[0], kb[1], rng.randrange(3))}, {"kind": "set", "values": sorted(da - db)})
    # what one document lists
    listing = max(names, key=lambda n: sum(1 for m in docs[n]["mentions"] if m["kind"] == "msisdn"))
    add("doc_lists", phrasing(LISTS) % {"d": listing}, {"kind": "set", "values": sorted({m["value"] for m in docs[listing]["mentions"] if m["kind"] == "msisdn"})})
    # places
    for value in sorted({v for (k, v) in by_key if k == "place"})[:2]:
        add("doc_place_count", phrasing(PLACE) % {"x": value.title()}, {"kind": "scalar", "value": len(by_key[("place", value)])})
    # the numbers of the documents that the call records hold
    held = sorted({v for (k, v) in by_key if k == "msisdn" and v in subscribers})
    add("doc_case_numbers", phrasing(SUBSCRIBERS), {"kind": "set", "values": held})
    # absences and the control
    absent_number = next(n for n in ("923000099999", "923000099998", "923000099997") if ("msisdn", n) not in by_key)
    absent_plate = next(x for x in ("SYNT0000", "SYNT0001", "SYNT0002") if ("plate", x) not in by_key)
    add("absent", phrasing(ABSENT) % {"x": surface("msisdn", absent_number, 0)}, {"kind": "absent"}, expect_extra={"control": True})
    add("absent", phrasing(ABSENT) % {"x": surface("plate", absent_plate, 0)}, {"kind": "absent"}, expect_extra={"control": True})
    control = next(n for n in names if not docs[n]["mentions"])
    add("control", phrasing(CONTROL) % {"d": control}, {"kind": "absent"}, expect_extra={"control": True})
    # the overclaim probe
    sample_doc = names[0]
    sample = next(m for m in docs[sample_doc]["mentions"] if m["kind"] == "msisdn")
    add("probe", phrasing(PROBE) % {"d": sample_doc, "x": surface("msisdn", sample["value"], 0)}, {"kind": "probe", "caveats": CAVEAT, "forbid": FORBID, "control": True})
    return questions
