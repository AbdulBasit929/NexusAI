"""The subscriber registrations and the device reference of a generated case, with their planted truth and an oracle (docs/data/subscriber-device-family-20261028.md section 4).

Kept OUT of the default case build on purpose: `generate.write_case(params, out, with_subscribers=True)` adds two registration exports (two sources) and a device reference list to a case, and the
default build (the register of 28 cases and every hash in it) is unchanged. Nothing here uses the random streams of the default build: the registrations come from their own stream, seeded by the
case's seed, and from the world and the rendered call records (so that the numbers, the SIMs and the handsets are those the call records show).

What is planted, so that a wrong reading shows:
  S1 SIM swap          two subjects have two registrations: the old SIM (deactivated at the swap) and the new one (from the swap).
  S2 conflicting       a second source file states a different status for two numbers in an overlapping period (both rows are kept, `conflicts` marks them), and the same registration for three
                       others (no conflict).
  S3 after the end     three subjects were deactivated in the middle of the period and the call records show events after it.
  S4 unregistered      two subjects appear in no registration file (the answer is absence).
  S5 handsets          the handsets the call records show, with a device reference that lists most of their TACs and omits one (an unknown model is unknown, not odd).
  S6 format defects    the number as 03xx, +92, 0092 or 92; two dates as dd/mm/yyyy; a blank ICCID; an exact repeated row; an unknown status word; a row with an unusable number and one with none.
  S7 identity columns  every row carries a synthetic name, a synthetic national ID number and an address, so that the dropping can be proved.
The truth is the list of unique registrations; the oracle answers from it and from the rendered call records, and shares no code with the views, the interpreter or the engine.
"""
import csv
import datetime
import os
import random
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "real_data"))
import reserved  # noqa: E402

LAYOUT_ID = "subscriber-s1"
HEADER = ["MSISDN", "Subscriber ID", "Subscriber Name", "CNIC", "Address", "IMSI", "ICCID", "Status", "Activation Date", "Deactivation Date", "Service Type", "Operator"]
REFERENCE_HEADER = ["Brand", "TAC", "SPECS"]
SOURCES = ("A", "B")
CANONICAL = {"Active": "active", "Suspended": "suspended", "Deactivated": "deactivated", "Dormant": "unknown"}


def _local(moment):
    return moment.strftime("%Y-%m-%d %H:%M:%S")


def _form(msisdn, index):
    """One number in one of its written forms (92 and ten digits is the normal one)."""
    return (msisdn, "0" + msisdn[2:], "+" + msisdn, msisdn[2:])[index % 4]


def build(params, world, renders):
    """The registrations and the device reference of one case: {"files": {source: rows}, "reference": rows, "truth": the key section}."""
    rng = random.Random("subscribers:%d" % params.seed)
    start = datetime.datetime.strptime(params.start_date, "%Y-%m-%d")
    days = params.period_days
    persons = list(world.persons)
    subjects = [p for p in persons if p.role == "subject"]
    visible = {p.msisdn: renders[p.pid].visible for p in subjects}

    def events_after(person, moment):
        stamp = _local(moment)
        return sum(1 for r in visible[person.msisdn] if r["start_local"] >= stamp)

    order = list(subjects)
    rng.shuffle(order)
    s4 = order[:2]
    rest = order[2:]
    s1 = rest[:2]
    rest = rest[2:]
    s3, others = [], []
    for person in rest:
        moment = start + datetime.timedelta(days=int(days * 0.55), hours=rng.randint(0, 20), minutes=rng.randint(0, 59), seconds=rng.randint(0, 59))
        if len(s3) < 3 and events_after(person, moment) > 0:
            s3.append((person, moment))
        else:
            others.append(person)
    s3_ids = {p.pid for p, _ in s3}
    s1_ids = {p.pid for p in s1}
    s4_ids = {p.pid for p in s4}

    truth, rows = [], {"A": [], "B": []}
    serial = {"n": 0}

    def number():
        serial["n"] += 1
        return serial["n"]

    def add(source, person, imsi, iccid, word, from_dt, to_dt, tags, form_index=0, date_style="iso", blank_iccid=False, ref=None):
        n = number()
        identity = (reserved.synthetic_name(n % 9999), reserved.synthetic_cnic(n % 10 ** 7), reserved.synthetic_address(n))
        shown_from = "" if from_dt is None else (from_dt.strftime("%d/%m/%Y") if date_style == "slash" else _local(from_dt))
        shown_to = "" if to_dt is None else _local(to_dt)
        row = [_form(person.msisdn, form_index), ref or "SUB-%05d" % (person.n + 1), identity[0], identity[1], identity[2], imsi, "" if blank_iccid else iccid, word, shown_from, shown_to,
               ("Prepaid", "Postpaid")[person.n % 2], "Operator %d" % (person.operator + 1)]
        rows[source].append(row)
        local_from = None if from_dt is None else (_local(from_dt.replace(hour=0, minute=0, second=0)) if date_style == "slash" else _local(from_dt))
        truth.append({"source": source, "person": person.pid, "msisdn": person.msisdn, "imsi": imsi, "iccid": None if blank_iccid else iccid, "status": CANONICAL[word], "word": word,
                      "from": local_from, "to": None if to_dt is None else _local(to_dt), "operator": "Operator %d" % (person.operator + 1), "service": ("prepaid", "postpaid")[person.n % 2],
                      "tags": tags})

    def before_start(low, high):
        return start - datetime.timedelta(days=rng.randint(low, high), hours=rng.randint(0, 23), minutes=rng.randint(0, 59), seconds=rng.randint(0, 59))

    defect_slots = {"slash": 0, "blank_iccid": 0, "dormant": 0}
    for index, person in enumerate(persons):
        if person.pid in s4_ids:
            continue
        iccid = reserved.synthetic_iccid(person.n)
        if person.pid in s1_ids:
            swap = start + datetime.timedelta(days=int(days * (0.35 + 0.1 * (index % 3))), hours=rng.randint(0, 20), minutes=rng.randint(0, 59), seconds=rng.randint(0, 59))
            add("A", person, reserved.synthetic_imsi(800000 + person.n), reserved.synthetic_iccid(800000 + person.n), "Deactivated", before_start(300, 700), swap, ["S1"], index)
            add("A", person, person.imsi, iccid, "Active", swap, None, ["S1"], index + 1)
            continue
        if person.pid in s3_ids:
            moment = next(m for p, m in s3 if p.pid == person.pid)
            add("A", person, person.imsi, iccid, "Deactivated", before_start(60, 600), moment, ["S3"], index)
            continue
        word, style, blank = "Active", "iso", False
        if person.pid not in {p.pid for p in subjects}:
            if defect_slots["slash"] < 2:
                style = "slash"
                defect_slots["slash"] += 1
                from_dt = before_start(60, 600).replace(hour=0, minute=0, second=0)
            elif defect_slots["blank_iccid"] < 1:
                blank = True
                defect_slots["blank_iccid"] += 1
                from_dt = before_start(60, 600)
            elif defect_slots["dormant"] < 1:
                word = "Dormant"
                defect_slots["dormant"] += 1
                from_dt = before_start(60, 600)
            else:
                from_dt = before_start(60, 900)
        else:
            from_dt = before_start(60, 900)
        add("A", person, person.imsi, iccid, word, from_dt, None, ["normal"] if word == "Active" and not blank and style == "iso" else ["format"], index, style, blank)

    # S2: the second source
    a_rows = {t["msisdn"]: t for t in truth if t["source"] == "A" and t["tags"] == ["normal"]}
    candidates = [p for p in others if p.msisdn in a_rows]
    conflicting, agreeing = candidates[:2], candidates[2:4]
    plain = [p for p in persons if p.role != "subject" and p.msisdn in a_rows]
    agreeing = (agreeing + plain)[:3]
    for person in conflicting:
        base = a_rows[person.msisdn]
        moment = datetime.datetime.strptime(base["from"], "%Y-%m-%d %H:%M:%S")
        add("B", person, person.imsi, reserved.synthetic_iccid(person.n), "Suspended", moment, None, ["S2:conflict"], 0)
    for person in agreeing:
        base = a_rows[person.msisdn]
        moment = datetime.datetime.strptime(base["from"], "%Y-%m-%d %H:%M:%S")
        add("B", person, person.imsi, reserved.synthetic_iccid(person.n), "Active", moment, None, ["S2:agree"], 0)

    # rows that must be quarantined, never in the truth, and one exact repeat
    quarantined = {"msisdn_unrecognised": 1, "msisdn_missing": 1}
    junk = ["12", "SUB-JUNK", reserved.synthetic_name(0), reserved.synthetic_cnic(0), reserved.synthetic_address(0), reserved.synthetic_imsi(1), reserved.synthetic_iccid(1), "Active", "", "", "Prepaid", "Operator 1"]
    rows["A"].insert(len(rows["A"]) // 3, list(junk))
    junk_missing = list(junk)
    junk_missing[0] = ""
    rows["A"].insert(len(rows["A"]) // 2, junk_missing)
    duplicate_at = len(rows["A"]) // 4
    rows["A"].insert(duplicate_at + 1, list(rows["A"][duplicate_at]))
    # the rows of A that were a repeat counted once in the truth; the oracle's counts of rows
    handsets = {}
    for person in subjects:
        for r in visible[person.msisdn]:
            if r.get("imei"):
                entry = handsets.setdefault((person.msisdn, r["imei"], r["imsi"]), {"events": 0, "first": r["start_local"], "last": r["start_local"]})
                entry["events"] += 1
                entry["first"] = min(entry["first"], r["start_local"])
                entry["last"] = max(entry["last"], r["start_local"])
    tacs = sorted({imei[:8] for _, imei, _ in handsets})
    listed = {}
    for i, tac in enumerate(tacs):
        if i % 5 != 4:
            listed[tac] = {"make": "SYNTH", "model": "SYNTH PHONE %02d, Model SP%02d, Global, 2025" % (i + 1, i + 1)}
    reference = [[v["make"], tac, v["model"]] for tac, v in sorted(listed.items())]
    for filler in range(20):
        reference.append(["SYNTH", "00%06d" % (900000 + filler), "SYNTH FILLER %02d, Model FP%02d, 2024" % (filler, filler)])
    padded_tac = sorted(listed)[0]
    reference[0] = [reference[0][0], padded_tac[1:], reference[0][2]]       # a spreadsheet dropped a leading zero
    reference.insert(3, ["SYNTH", "000123", "SYNTH SHORT"])                  # six digits: rejected
    reference.insert(5, ["SYNTH", "", "SYNTH EMPTY"])                        # no TAC: rejected
    reference.append(list(reference[7]))                                     # an exact repeat
    reference.append([reference[8][0], reference[8][1], reference[8][2] + " (second listing)"])   # a conflicting repeat
    section = {
        "layout_id": LAYOUT_ID, "sources": list(SOURCES), "registrations": truth,
        "plants": {"s1": [p.pid for p in s1], "s3": [{"person": p.pid, "deactivated": _local(m)} for p, m in s3], "s4": [p.pid for p in s4],
                   "s2_conflict": [p.pid for p in conflicting], "s2_agree": [p.pid for p in agreeing]},
        "handsets": [{"msisdn": m, "imei": i, "imsi": s, **v} for (m, i, s), v in sorted(handsets.items())],
        "device_reference": {"listed": listed, "unlisted_tacs": [t for t in tacs if t not in listed], "rows_in_file": len(reference),
                             "planted": {"tac_padded": 1, "rejected": {"tac_length_6": 1, "tac_missing": 1}, "repeats": 1, "conflicts": 1}},
        "planted": {"duplicate_rows": 1, "quarantined": quarantined, "rows_in_file": {s: len(rows[s]) for s in SOURCES}},
    }
    return {"files": rows, "reference": reference, "truth": section}


def write(rows, path, header=HEADER):
    with open(path, "w", encoding="utf-8", newline="") as handle:
        writer = csv.writer(handle, lineterminator="\n")
        writer.writerow(header)
        writer.writerows(rows)


# ---------------------------------------------------------------- the oracle: answers from the planted truth and the rendered call records alone

def _within(row, moment):
    return (row["from"] is None or row["from"] <= moment) and (row["to"] is None or moment < row["to"])


def _overlap(a, b):
    low = lambda r: r["from"] or "0000-01-01 00:00:00"     # noqa: E731 - an open start
    high = lambda r: r["to"] or "9999-12-31 23:59:59"      # noqa: E731 - an open end
    return low(a) < high(b) and low(b) < high(a)


class Oracle:
    def __init__(self, truth, events):
        """truth: the key section; events: {msisdn: [local time strings of the subject's call-record events]}."""
        self.truth, self.events = truth, events
        self.rows = truth["registrations"]

    def registrations(self, msisdn):
        return sorted((r for r in self.rows if r["msisdn"] == msisdn), key=lambda r: (r["from"] or "", r["source"]))

    def registered_at(self, msisdn, moment):
        return [r for r in self.registrations(msisdn) if _within(r, moment)]

    def numbers_of_imsi(self, imsi):
        return sorted({r["msisdn"] for r in self.rows if r["imsi"] == imsi})

    def registered_numbers(self):
        return sorted({r["msisdn"] for r in self.rows})

    def conflicting_numbers(self):
        out = set()
        for a in self.rows:
            for b in self.rows:
                if a["msisdn"] == b["msisdn"] and a["source"] != b["source"] and _overlap(a, b) and (a["imsi"] != b["imsi"] or a["status"] != b["status"]):
                    out.add(a["msisdn"])
        return sorted(out)

    def activity(self, msisdn):
        """(events, outside every registration, at or after the last end, first, last) for a registered number; None when it has no registration."""
        regs = self.registrations(msisdn)
        if not regs:
            return None
        events = sorted(self.events.get(msisdn, []))
        outside = sum(1 for t in events if not any(_within(r, t) for r in regs))
        last_end = None if any(r["to"] is None for r in regs) else max(r["to"] for r in regs)
        after = 0 if last_end is None else sum(1 for t in events if t >= last_end)
        return len(events), outside, after, events[0] if events else None, events[-1] if events else None

    def after_the_end(self):
        """The numbers with events at or after the end of their last registration, with the count."""
        out = {}
        for msisdn in self.registered_numbers():
            events, outside, after, _, _ = self.activity(msisdn)
            if after:
                out[msisdn] = after
        return out

    def handsets(self, msisdn):
        return [h for h in self.truth["handsets"] if h["msisdn"] == msisdn]

    def model_of(self, imei):
        listed = self.truth["device_reference"]["listed"].get(imei[:8])
        return None if listed is None else listed["model"]

    def numbers_using(self, model):
        return sorted({h["msisdn"] for h in self.truth["handsets"] if self.model_of(h["imei"]) == model})

    def unregistered(self, subjects):
        return sorted(set(subjects) - set(self.registered_numbers()))
