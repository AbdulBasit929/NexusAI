#!/usr/bin/env python3
"""Builds the FRESH-WORDING AUDIO BANK of the Orion Lahore reference case: 34 questions about the ten audio recordings, written in wordings that neither the 31 speech questions of the verified bank
(VB-SPEE-001 to 031) nor the examples and descriptions written for the audio views (migration 037, ask_engine_examples.json) use, so that the run measures the engine and not the examples.

    python -X utf8 tools/showcase/build_fresh_audio_bank.py --out docs/qa/audio-fresh-wording-20261116.json

Keys are the same three kinds as in the verified bank and none is the engine: the speech pack's own truth (language and length of each recording; the file header gives the length), the stored readings
read straight from the artifacts table (segments, words, what a recording says), and the case's own file names. Thresholds sit at least 0.2 s from every recording; a question whose top answer is tied is
not asked. Nothing here writes to the case. The gates, fixed before the run, are in docs/data/speech-lab-results-20261112.md (section 11).
"""
import argparse
import datetime
import json
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import build_verified_bank as vb  # noqa: E402

UNIVERSE = r"voice-note-\d+\.wav"


def build(collection):
    pack = {p["file"]: p for p in json.load(open(vb.SPEECH_KEY, encoding="utf-8"))}
    stored = {}
    for name, start, end, text in vb.psql(vb.SPEECH_ROWS % collection):
        stored.setdefault(name, []).append({"start": float(start), "end": float(end), "text": text})
    files = sorted(pack)
    missing = [f for f in files if f not in stored]
    if missing:
        raise SystemExit("the speech pack is not fully read yet (no segment for %s)" % ", ".join(missing))
    seconds = {f: pack[f]["seconds"] for f in files}
    lang = {f: pack[f]["language"] for f in files}
    names = vb.LANGUAGE_NAMES
    items = []

    def add(kind, question, expect, note=""):
        items.append({"id": "FA-%03d" % (len(items) + 1), "area": "speech", "type": kind, "question": question, "expect": expect, "bank": "fresh", "note": note})

    def of(code):
        return [f for f in files if lang[f] == code]

    def margin(limit, wanted):
        near = [f for f in files if abs(seconds[f] - limit) < 0.2]
        if near:
            raise SystemExit("a recording is within 0.2 s of the limit %s: %s" % (limit, near))
        return wanted

    # ---- length
    for f in ("voice-note-07.wav", "voice-note-09.wav", "voice-note-01.wav"):
        add("audio_length", ("What is the duration of %s?" if f.endswith("07.wav") else "Tell me how many seconds %s lasts." if f.endswith("09.wav") else "How many seconds is the file %s?") % f,
            {"kind": "scalar", "value": seconds[f], "unit": "seconds", "tolerance": 0.6}, "the pack's own length (the file header)")
    shortest = sorted(files, key=lambda f: seconds[f])
    if seconds[shortest[1]] - seconds[shortest[0]] < 0.2:
        raise SystemExit("the shortest recording is tied")
    add("audio_shortest", "Which recording has the shortest duration?", {"kind": "top", "value": shortest[0]}, "the first row must be this recording")
    add("audio_total_minutes", "What is the combined running time of every audio file in the case, in minutes?", {"kind": "scalar", "value": round(sum(seconds.values()) / 60, 2), "unit": "minutes", "tolerance": 0.03}, "the pack's own lengths, in minutes")
    add("audio_count_longer", "How many recordings run longer than 9.5 seconds?", {"kind": "scalar", "value": margin(9.5, len([f for f in files if seconds[f] > 9.5]))}, "no recording within 0.2 s of the limit")
    add("audio_under", "List the audio files that are under 8 seconds.", {"kind": "files", "values": margin(8.0, sorted(f for f in files if seconds[f] < 8.0)), "universe": UNIVERSE}, "exactly these; no recording within 0.2 s of the limit")
    add("audio_average", "What is the average length of the audio recordings, in seconds?", {"kind": "scalar", "value": round(sum(seconds.values()) / len(files), 2), "unit": "seconds", "tolerance": 0.3}, "the pack's own lengths")

    # ---- language
    add("audio_in_language", "Which voice notes are spoken in English?", {"kind": "files", "values": of("en"), "universe": UNIVERSE}, "exactly these recordings")
    add("audio_in_language", "List the recordings where Urdu is spoken.", {"kind": "files", "values": of("ur"), "universe": UNIVERSE}, "exactly these recordings")
    add("audio_language_count", "Count the recordings whose language is Pashto.", {"kind": "scalar", "value": len(of("ps"))})
    f7 = "voice-note-07.wav"
    add("audio_language_of", "Which language is spoken in %s?" % f7, {"kind": "words", "any": vb.LANGUAGE_WORDS[lang[f7]][0] + [lang[f7]], "forbid": vb.LANGUAGE_WORDS[lang[f7]][1]}, "the language the pack says the speaker used (%s)" % names[lang[f7]])
    f4 = "voice-note-04.wav"
    add("audio_language_of", "Is %s in Pashto, Urdu or English?" % f4, {"kind": "words", "any": vb.LANGUAGE_WORDS[lang[f4]][0] + [lang[f4]]}, "the options are in the question, so no word is forbidden")
    add("audio_language_distinct", "How many different languages are spoken across the recordings?", {"kind": "scalar", "value": len(set(lang.values()))})
    counts = sorted(((len(of(c)), c) for c in set(lang.values())), reverse=True)
    if counts[0][0] == counts[1][0]:
        raise SystemExit("the language with the most recordings is tied")
    add("audio_language_top", "Which language has the most recordings?", {"kind": "top", "value": names[counts[0][1]]}, "%d recordings; the next has %d" % (counts[0][0], counts[1][0]))

    # ---- two conditions
    add("audio_two_conditions", "Which recordings are in Urdu and longer than 9.2 seconds?", {"kind": "files", "values": margin(9.2, sorted(f for f in of("ur") if seconds[f] > 9.2)), "universe": UNIVERSE}, "exactly these")
    english = sorted(of("en"), key=lambda f: -seconds[f])
    if seconds[english[0]] - seconds[english[1]] < 0.2:
        raise SystemExit("the longest English recording is tied")
    add("audio_longest_in_language", "What is the longest English recording?", {"kind": "top", "value": english[0]}, "the first row must be this recording")
    pashto = sorted(of("ps"), key=lambda f: seconds[f])
    add("audio_length_of_shortest_in_language", "How long is the shortest Pashto recording, in seconds?", {"kind": "scalar", "value": seconds[pashto[0]], "unit": "seconds", "tolerance": 0.6}, "the pack's own length (%s)" % pashto[0])
    add("audio_segments_in_language", "How many speech segments do the Urdu recordings have in total?", {"kind": "scalar", "value": sum(len(stored[f]) for f in of("ur"))}, "counted in the stored readings")

    # ---- segments
    add("audio_segments", "How many segments is %s split into?" % f4, {"kind": "scalar", "value": len(stored[f4])}, "counted in the stored readings")
    add("audio_segments", "Into how many speech segments was %s divided?" % "voice-note-09.wav", {"kind": "scalar", "value": len(stored["voice-note-09.wav"])}, "counted in the stored readings")
    add("audio_segments_all", "What is the total number of speech segments across all the audio files?", {"kind": "scalar", "value": sum(len(v) for v in stored.values())}, "counted in the stored readings")
    by_segments = sorted(files, key=lambda f: -len(stored[f]))
    if len(stored[by_segments[0]]) == len(stored[by_segments[1]]):
        raise SystemExit("the recording with the most segments is tied")
    add("audio_most_segments", "Which recording has the most speech segments?", {"kind": "top", "value": by_segments[0]}, "%d segments; the next has %d" % (len(stored[by_segments[0]]), len(stored[by_segments[1]])))
    add("audio_single_segment", "How many recordings consist of a single segment?", {"kind": "scalar", "value": len([f for f in files if len(stored[f]) == 1])}, "counted in the stored readings")

    # ---- review
    add("audio_review", "Which audio files still need a person to check them?", {"kind": "files", "values": files, "universe": UNIVERSE}, "every machine transcript needs a person's check, so all of them")
    add("audio_review_count", "How many recordings need manual review?", {"kind": "scalar", "value": len(files)}, "every machine transcript needs a person's check, so all of them")

    # ---- what a recording says: three words of its stored reading
    for f, phrasing in (("voice-note-06.wav", "Show the text of %s"), ("voice-note-07.wav", "Transcribe %s"), ("voice-note-08.wav", "What does the speaker in %s say?")):
        words = [w for w in vb.tokens(" ".join(s["text"] for s in stored[f])) if len(w) >= 4][:6]
        pick = [words[0], words[2], words[4]] if len(words) >= 5 else words[:3]
        add("audio_transcript", phrasing % f, {"kind": "words", "all": pick}, "three words of the stored reading, which must be in the answer; the reading is a machine transcript (Pashto: a search lead)")

    # ---- words: not the words of the verified bank
    used = set()
    for code, _ in (("en", 0), ("ur", 0), ("ps", 0), ("en", 0), ("ur", 0)):
        for f in of(code):
            reference = [r.lower() for r in vb.tokens(pack[f]["reference"])]
            candidates = [w for w in vb.tokens(" ".join(s["text"] for s in stored[f])) if len(w) >= 4 and w.lower() not in used and w.lower() in reference]
            if candidates:
                used.add(sorted(candidates, key=lambda w: (-len(w), w))[0].lower())
                break
    for source, phrasing in (("voice-note-09.wav", "Which of the voice notes talk about %s?"), ("voice-note-07.wav", "Search the recordings for %s"), ("voice-note-10.wav", "Where is %s mentioned in the audio?"), ("voice-note-03.wav", "Which recording contains %s?")):
        reference = [r.lower() for r in vb.tokens(pack[source]["reference"])]
        candidates = sorted({w for w in vb.tokens(" ".join(s["text"] for s in stored[source])) if len(w) >= 4 and w.lower() in reference and w.lower() not in used}, key=lambda w: (-len(w), w))
        if not candidates:
            raise SystemExit("no fresh word in " + source)
        word = candidates[0]
        used.add(word.lower())
        hits = sorted(g for g in files if word.lower() in [w.lower() for w in vb.tokens(" ".join(s["text"] for s in stored[g]))])
        add("audio_word_search", phrasing % word, {"kind": "files", "values": hits, "universe": UNIVERSE}, "the recordings whose stored reading holds the word (%s)" % ", ".join(hits))
    absent = "zebra"
    if any(absent in " ".join(s["text"] for s in stored[g]).lower() for g in files):
        raise SystemExit("the absent word occurs in a reading")
    add("audio_word_absent", "Which audio recordings mention the word %s?" % absent, {"kind": "files", "values": [], "universe": UNIVERSE}, "no stored reading holds it: the answer must name no recording")
    return items


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", required=True)
    args = ap.parse_args(argv)
    items = build(vb.COLLECTION)
    with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(json.dumps({"collection": vb.COLLECTION, "built": datetime.date.today().isoformat(), "questions": items}, indent=1, ensure_ascii=False) + "\n")
    kinds = {}
    for i in items:
        kinds[i["type"]] = kinds.get(i["type"], 0) + 1
    print("%d questions -> %s" % (len(items), args.out))
    for kind, n in sorted(kinds.items()):
        print("  %-38s %d" % (kind, n))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
