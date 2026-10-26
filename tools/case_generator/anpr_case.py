"""The ANPR sightings of a generated case, with their planted truth and an oracle (docs/data/anpr-typed-family-20261026.md section 3).

Kept OUT of the default case build on purpose: `generate.write_case(params, out, with_anpr=True)` adds one camera export to a case, and the default build (the register of 28 cases and every hash in it) is
unchanged. Nothing here uses the random streams of the default build: the sightings come from their own stream, seeded by the case's seed, so the same seed gives the same sightings.

What is invented is vehicles and cameras only. Every plate is of the reserved synthetic series SYNT (tools/real_data/reserved.py: four letters, which no registration series has), so a plate of this file
cannot be a registration. What is planted, so that a wrong reading shows:
  * vehicles of interest, seen again and again along routes of two to five cameras, trips that cross midnight among them;
  * the same plate written in five presentations (case, separators, a typographic dash, Arabic-Indic digits): one plate, one search key;
  * look-alike plates (an O for a 0, an I for a 1) seen a few times at other cameras: another plate, which must never be returned for the planted one;
  * exact repeats of a row (the same second, the same camera, the same token): the views count them once;
  * a camera without a position, a confidence written as a percent in some rows, rows without a plate and a row without a readable time (quarantined, never in the truth).
The truth is the list of unique sightings; the oracle answers from it and shares no code with the views, the interpreter or the engine.
"""
import csv
import datetime
import json
import os
import random
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(HERE, "..", "real_data"))
import reserved  # noqa: E402

LAYOUT_ID = "anpr-a1"
HEADER = ["Date Time", "Plate Number", "Camera ID", "Camera Name", "Lane", "Latitude", "Longitude", "Direction", "Confidence"]
FORMS = ("SYNT-%s", "synt %s", "SYNT%s", "SYNT–%s", "SYNT-%s")   # the last is written with Arabic-Indic digits
ARABIC_INDIC = "٠١٢٣٤٥٦٧٨٩"


def key_of(token_number):
    return "SYNT" + token_number


def _local(moment):
    return moment.strftime("%Y-%m-%d %H:%M:%S")


def build(params, vehicles=6, background=36, cameras=None):
    """The sightings of one case: {"rows": the file's rows (header excluded), "truth": the key section}."""
    rng = random.Random("anpr:%d" % params.seed)
    n_cameras = cameras or 8 + params.seed % 5
    start = datetime.datetime.strptime(params.start_date, "%Y-%m-%d")
    days = params.period_days
    cams = []
    for i in range(n_cameras):
        cams.append({"id": "CAM-%02d" % (i + 1), "label": "Camera %d" % (i + 1), "lat": round(31.45 + rng.random() * 0.15, 5), "lon": round(74.25 + rng.random() * 0.20, 5),
                     "lane": "Lane %d" % (i % 3 + 1)})
    no_position = cams[rng.randrange(n_cameras)]["id"]
    by_id = {c["id"]: c for c in cams}

    numbers = rng.sample(range(1000, 9999), vehicles + background)
    # vehicles of interest: the first two plates carry a 0 and the third a 1, so that look-alikes exist
    interest = []
    for slot, number in enumerate(numbers[:vehicles]):
        text = "%04d" % number
        if slot < 2 and "0" not in text:
            text = text[0] + "0" + text[2:]
        if slot == 2 and "1" not in text:
            text = text[0] + "1" + text[2:]
        interest.append(text)
    plain = ["%04d" % n for n in numbers[vehicles:]]
    sightings = []   # (plate_text, camera_id, datetime)

    def add(text, camera, moment):
        sightings.append((text, camera, moment))

    for slot, text in enumerate(interest):
        route = rng.sample([c["id"] for c in cams], rng.randint(2, min(5, n_cameras)))
        for _ in range(rng.randint(3, 12)):
            moment = start + datetime.timedelta(days=rng.randrange(days), hours=rng.randint(6, 21), minutes=rng.randint(0, 59), seconds=rng.randint(0, 59))
            if slot == 0 and rng.random() < 0.5:
                moment = moment.replace(hour=23, minute=rng.randint(40, 59))   # trips that cross midnight
            for camera in route:
                add(text, camera, moment)
                moment += datetime.timedelta(minutes=rng.randint(3, 25), seconds=rng.randint(0, 59))
    for text in plain:
        for _ in range(rng.randint(1, 4)):
            moment = start + datetime.timedelta(days=rng.randrange(days), hours=rng.randint(0, 23), minutes=rng.randint(0, 59), seconds=rng.randint(0, 59))
            add(text, cams[rng.randrange(n_cameras)]["id"], moment)
    look_alikes = []
    for text in interest[:3]:
        twin = text.replace("0", "O", 1) if "0" in text else text.replace("1", "I", 1)
        look_alikes.append({"of": key_of(text), "plate": key_of(twin)})
        for _ in range(rng.randint(2, 4)):
            moment = start + datetime.timedelta(days=rng.randrange(days), hours=rng.randint(6, 21), minutes=rng.randint(0, 59), seconds=rng.randint(0, 59))
            add(twin, cams[rng.randrange(n_cameras)]["id"], moment)

    # unique sightings: an exact repeat (the same plate, camera and second) is one sighting
    unique = sorted({(t, c, m) for t, c, m in sightings}, key=lambda s: (s[2], s[1], s[0]))
    rows, truth, duplicates, percent_rows = [], [], 0, 0
    for index, (text, camera, moment) in enumerate(unique):
        form = FORMS[index % len(FORMS)]
        digits = "".join(ARABIC_INDIC[int(d)] if d.isdigit() else d for d in text) if index % len(FORMS) == len(FORMS) - 1 else text
        token = form % digits
        cam = by_id[camera]
        confidence = round(0.80 + rng.random() * 0.19, 2)
        shown = "%d" % round(confidence * 100) if index % 17 == 5 else repr(confidence)
        percent_rows += 1 if index % 17 == 5 else 0
        lat = "" if camera == no_position else repr(cam["lat"])
        lon = "" if camera == no_position else repr(cam["lon"])
        row = [_local(moment), token, camera, cam["label"], cam["lane"], lat, lon, rng.choice(("Towards", "Away")), shown]
        rows.append(row)
        truth.append({"plate": key_of(text), "camera": camera, "local": _local(moment), "raw": token})
        if index % 33 == 7:
            rows.append(list(row))    # the same row again (a percent confidence is written, and counted, twice)
            duplicates += 1
            percent_rows += 1 if index % 17 == 5 else 0
    # rows that must be quarantined, never in the truth
    for position in (len(rows) // 4, len(rows) // 2):
        rows.insert(position, [_local(start + datetime.timedelta(days=1, hours=3)), "", cams[0]["id"], cams[0]["label"], cams[0]["lane"], "", "", "Towards", "0.9"])
    rows.insert(len(rows) // 3, ["not-a-time", "SYNT-0001", cams[1]["id"], cams[1]["label"], cams[1]["lane"], "", "", "Away", "0.9"])
    section = {
        "layout_id": LAYOUT_ID, "cameras": [dict(c, has_position=c["id"] != no_position) for c in cams], "of_interest": [key_of(t) for t in interest], "look_alikes": look_alikes,
        "sightings": truth, "planted": {"duplicate_rows": duplicates, "confidence_percent_rows": percent_rows, "quarantined": {"plate_missing": 2, "time_unparsed": 1}, "rows_in_file": len(rows)},
    }
    return {"rows": rows, "truth": section}


def write(rows, path):
    with open(path, "w", encoding="utf-8", newline="") as handle:
        writer = csv.writer(handle, lineterminator="\n")
        writer.writerow(HEADER)
        writer.writerows(rows)


# ---------------------------------------------------------------- the oracle: answers from the planted truth alone
class Oracle:
    def __init__(self, truth):
        self.rows = truth["sightings"]
        self.cameras = {c["id"]: c for c in truth["cameras"]}

    def sightings_of(self, plate, start=None, end=None):
        """(camera, local time) of every sighting of a plate (its search key), optionally from start up to and including end (local time text)."""
        return sorted((r["camera"], r["local"]) for r in self.rows if r["plate"] == plate and (start is None or r["local"] >= start) and (end is None or r["local"] <= end))

    def count_of(self, plate, start=None, end=None):
        return len(self.sightings_of(plate, start, end))

    def cameras_of(self, plate):
        return sorted({r["camera"] for r in self.rows if r["plate"] == plate})

    def first_last(self, plate):
        times = sorted(r["local"] for r in self.rows if r["plate"] == plate)
        return (times[0], times[-1]) if times else (None, None)

    def at_camera(self, camera, date=None):
        """The (plate, local time) of the sightings at a camera, on a local date when one is given."""
        return sorted((r["plate"], r["local"]) for r in self.rows if r["camera"] == camera and (date is None or r["local"].startswith(date)))

    def plates_at_both(self, first, second):
        return sorted(set(r["plate"] for r in self.rows if r["camera"] == first) & set(r["plate"] for r in self.rows if r["camera"] == second))

    def per_camera(self):
        counts = {}
        for r in self.rows:
            counts[r["camera"]] = counts.get(r["camera"], 0) + 1
        return counts

    def busiest(self):
        counts = self.per_camera()
        top = max(counts.values())
        return sorted(c for c, n in counts.items() if n == top), top

    def plates(self):
        return sorted({r["plate"] for r in self.rows})


def audit(section):
    """Problems of a section: every plate is of the synthetic series, nothing in the truth is quarantined, the cameras are the file's."""
    problems = []
    for row in section["sightings"]:
        if not reserved.is_reserved_plate(row["raw"]) and not reserved.is_reserved_plate(row["raw"].replace("O", "0").replace("I", "1")):
            problems.append("plate %r is not of the synthetic series" % row["raw"])
    known = {c["id"] for c in section["cameras"]}
    for row in section["sightings"]:
        if row["camera"] not in known:
            problems.append("camera %r is not a camera of the case" % row["camera"])
    return problems


if __name__ == "__main__":
    import world as w
    p = w.Params(seed=int(sys.argv[1]) if len(sys.argv) > 1 else 2)
    built = build(p)
    print(json.dumps({"rows": len(built["truth"]["planted"]), "unique_sightings": len(built["truth"]["sightings"]), "planted": built["truth"]["planted"]}))
