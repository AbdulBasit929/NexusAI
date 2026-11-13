#!/usr/bin/env python3
"""Builds the VERIFIED QUESTION BANK of the Orion Lahore reference case: many questions for every family and every usage, each with the answer computed by plain SQL on the stored rows of the live collection
(read only, the superuser's view of the typed tables, never through the Ask engine's views), so that the answer is independent of the engine that is being tested.

    python -X utf8 tools/showcase/build_verified_bank.py --out docs/qa/verified-bank-20261102.json

The questions are templates filled with the case's own entities (its subscribers, accounts, plates, cameras, documents), chosen from the data (the busiest first), and written so that one reading is possible:
"voice calls" and "text messages" are never mixed, a direction is said, a time is on the case clock (Asia/Karachi), a question whose top answer is tied is not asked. Nothing here writes to the case.

Expectation kinds: scalar (a number, with its unit), text (a time or a day that must occur in the answer), set (every value must occur), top (the first row of the answer must be the value, and it must come
before its rivals), video (a question about the video clips of the case, judged by tools/video_plates/video_question_bank.py: number, set, top1, none, or named). tools/showcase/judge_verified_bank.py reads
the engine's answers and scores them.

The five video clips of the case are REAL footage of the owner's, so the plates read from them never enter the repository: a question whose answer is made of plates keeps only the count of plates in this
bank and its values go to the PRIVATE bank (--private-out, default C:/NexusAI-Showcase/orion/key/video-private-bank.json, refused inside the repository), which also holds the typed plate searches (they contain
plates). The judge reads the private bank to score them and writes the sheet with every plate masked.
"""
import argparse
import datetime
import json
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REPO = os.path.abspath(os.path.join(HERE, "..", ".."))
sys.path.insert(0, os.path.join(HERE, "..", "video_plates"))
import video_question_bank as vqb  # noqa: E402  (its key is computed from the stored readings by queries that do not use the analyst views)

COLLECTION = "orion-lahore-reference-20261030"
PLATE_KEY = os.environ.get("ORION_PLATE_KEY", "C:/NexusAI-Showcase/orion/key/plate-images-key.json")  # the construction key of the photos added to the case (synthetic plates only)
PRIVATE_BANK = os.environ.get("ORION_VIDEO_PRIVATE_BANK", "C:/NexusAI-Showcase/orion/key/video-private-bank.json")  # plates read from the owner's real footage: this machine only
SPEECH_KEY = os.environ.get("ORION_SPEECH_KEY", "C:/NexusAI-Showcase/orion/key/speech-pack-key.json")  # the speech pack: language and length of each recording and the text it was recorded from (public speech)
VIDEO_SUFFIXES = (".mp4", ".mov", ".avi", ".mkv")
TZ = "Asia/Karachi"
SUBS_VOICE_ONLY = ["923000001005"]
COMM = "forensic.comm_event c WHERE c.collection_id = '%s' AND c.dataset_kind = 'target_cdr' AND NOT ('duplicate_row' = ANY (c.quality_flags))" % COLLECTION
SUB = "coalesce(c.a_norm, c.a_raw)"
BANK = "forensic.bank_transaction b WHERE b.collection_id = '%s' AND NOT ('duplicate_row' = ANY (b.quality_flags))" % COLLECTION
SESS = "forensic.data_session s WHERE s.collection_id = '%s' AND NOT ('duplicate_row' = ANY (s.quality_flags))" % COLLECTION
USAGE = "forensic.app_usage u WHERE u.collection_id = '%s' AND NOT ('duplicate_row' = ANY (u.quality_flags))" % COLLECTION
PLATE = "forensic.plate_sighting p WHERE p.collection_id = '%s' AND NOT ('duplicate_row' = ANY (p.quality_flags))" % COLLECTION


def psql(sql):
    out = subprocess.run(["docker", "exec", "-i", "nexusai-forensic-postgres-1", "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-F", "\t", "-v", "ON_ERROR_STOP=1"], input=sql, capture_output=True, text=True, encoding="utf-8")
    if out.returncode != 0:
        raise SystemExit("psql failed: " + out.stderr[-500:] + "\n" + sql[:300])
    return [line.split("\t") for line in out.stdout.strip().splitlines() if line.strip()]


def one(sql):
    rows = psql(sql)
    return rows[0][0] if rows and rows[0] else None


def unique_top(rows):
    """The first row of (value, count) rows when its count is alone at the top, else None."""
    if not rows or (len(rows) > 1 and rows[0][1] == rows[1][1]):
        return None
    return rows


class Bank:
    def __init__(self):
        self.items = []
        self.seq = {}
        self.private = {}   # bank id -> the plates a video question expects (never written to the public bank)
        self.typed = []     # the typed plate searches (their wording holds a plate): private bank only

    def add(self, area, kind, question, expect, note=""):
        self.seq[area] = self.seq.get(area, 0) + 1
        self.items.append({"id": "VB-%s-%03d" % (area.upper()[:4], self.seq[area]), "area": area, "type": kind, "question": question, "expect": expect, "bank": "extra", "note": note})


def holds_plates(question):
    """True when the expected answer of a video question is made of plates read from the footage (those stay out of the repository); file names are public."""
    if question["kind"] not in ("set", "top1"):
        return False
    return any(not str(x).lower().endswith(VIDEO_SUFFIXES) for x in question["expected"])


def add_video(bank, question, type_name=None):
    """One question of tools/video_plates/video_question_bank.py as an item of the bank; the plates it expects, if any, go to bank.private under the item's id."""
    expect = {"kind": "video", "vkind": question["kind"], "expect_view": question.get("expect_view", "")}
    for key in ("tolerance", "what"):
        if key in question:
            expect[key] = question[key]
    plates = holds_plates(question)
    if plates:
        expect["private"] = True
        expect["n"] = len(question["expected"])
    else:
        expect["value"] = question["expected"]
    bank.add("videos", type_name or question["id"], question["q"], expect,
             "the key is computed from the stored readings, not through the analyst views; a reading is a model's candidate, to be checked by eye" + ("; the plates it expects are in the private bank" if plates else ""))
    if plates:
        bank.private[bank.items[-1]["id"]] = question["expected"]


def video_questions(collection):
    """The questions about the videos of the case: the wording of the live run of 2026-11-12 (35 correct, one clarifying question) and the two other wordings that were answered correctly."""
    questions = [dict(q) for q in vqb.build(collection, "dev")]
    for which, wanted, suffix in (("sealed2", "total-minutes", "-alt"), ("sealed3", "total-seconds", "")):
        for q in vqb.build(collection, which):
            if q["id"] == wanted:
                q = dict(q)
                q["id"] = wanted + suffix
                questions.append(q)
    return questions


def typed_plate_searches(collection):
    """For each clip, the plate that stays in view longest and every clip that holds a reading of exactly that plate: [(plate, [clip, ...])]."""
    groups = vqb.psql_rows(collection, vqb.GROUPS)
    searches = []
    for clip in sorted({g["f"] for g in groups}):
        mine = [g for g in groups if g["f"] == clip]
        plate = max(mine, key=lambda g: (float(g["last"]) - float(g["first"]), g["plate"]))["plate"]
        searches.append((plate, sorted({g["f"] for g in groups if g["plate"] == plate})))
    return searches


SPEECH_ROWS = ("select e.original_filename, coalesce((d.citation_locator ->> 'start_seconds')::numeric, (d.metadata -> 'observation' ->> 'start_seconds')::numeric), "
               "coalesce((d.citation_locator ->> 'end_seconds')::numeric, (d.metadata -> 'observation' ->> 'end_seconds')::numeric), d.metadata -> 'observation' ->> 'text' "
               "from forensic.derived_artifacts d join forensic.evidence_items e on e.tenant_id = d.tenant_id and e.collection_id = d.collection_id and e.evidence_id = d.evidence_id "
               "where d.collection_id = '%s' and e.modality = 'audio' and d.artifact_type = 'forensics.audio-timestamp-segment/v1' and d.processing_status = 'completed' "
               "order by 1, 2")
LANGUAGE_WORDS = {"ps": (["Pashto"], ["Urdu", "English", "Persian", "Punjabi"]), "ur": (["Urdu"], ["Pashto", "English", "Hindi", "Punjabi"]), "en": (["English"], ["Pashto", "Urdu", "Hindi", "Punjabi"])}
LANGUAGE_NAMES = {"ps": "Pashto", "ur": "Urdu", "en": "English"}


def tokens(text):
    return re.findall(r"[^\W\d_]+", text or "", re.U)


def add_speech(bank, collection):
    """Questions about the audio recordings of the case (the speech pack: public speech of real speakers, see docs/data/speech-lab-results-20261112.md section 10). Three kinds of key, none of them the engine:
    the pack's own truth (language and length of each recording), the stored readings read straight from the artifacts table (segments, words and what a recording says: this tests the engine over the data),
    and, separately in tools/media_data/speech_pack_report.py, the reference text (this tests the reading). The questions are written the way an analyst would put them."""
    if not os.path.exists(SPEECH_KEY):
        return
    pack = {p["file"]: p for p in json.load(open(SPEECH_KEY, encoding="utf-8"))}
    stored = {}
    for name, start, end, text in psql(SPEECH_ROWS % collection):
        stored.setdefault(name, []).append({"start": float(start), "end": float(end), "text": text})
    files = sorted(pack)
    missing = [f for f in files if f not in stored]
    if missing:
        raise SystemExit("the speech pack is not fully read yet (no segment for %s)" % ", ".join(missing))
    universe = r"voice-note-\d+\.wav"
    seconds = {f: pack[f]["seconds"] for f in files}
    add = bank.add

    def audio_files(**where):
        return [f for f in files if all(pack[f][k] == v for k, v in where.items())]

    n_audio = int(one("select count(*) from forensic.evidence_items i where i.collection_id = '%s' and i.modality = 'audio'" % collection))
    if n_audio != len(files):
        raise SystemExit("the case holds %d audio files but the speech pack has %d" % (n_audio, len(files)))
    add("speech", "audio_count", "How many audio recordings are in this case?", {"kind": "scalar", "value": n_audio}, "the audio files of the case")
    add("speech", "audio_total_seconds", "How long are all the audio recordings together, in seconds?", {"kind": "scalar", "value": round(sum(seconds.values()), 1), "unit": "seconds", "tolerance": 1.5}, "the length of each recording is the pack's own; a reading ends where its last segment ends, so one second and a half is allowed over ten recordings")
    longest = max(files, key=lambda f: seconds[f])
    add("speech", "audio_longest", "Which audio recording is the longest?", {"kind": "top", "value": longest}, "the first row must be this recording")
    add("speech", "audio_longer_than", "Which audio recordings are longer than 9 seconds?", {"kind": "files", "values": sorted(f for f in files if seconds[f] > 9), "universe": universe}, "exactly these; no recording is within 0.2 s of the limit")
    add("speech", "audio_shorter_than", "Which audio recordings are shorter than 7 seconds?", {"kind": "files", "values": sorted(f for f in files if seconds[f] < 7), "universe": universe}, "exactly these; no recording is within 0.7 s of the limit")
    for f in (files[2], files[4], files[7]):
        add("speech", "audio_length", "How long is %s, in seconds?" % f, {"kind": "scalar", "value": seconds[f], "unit": "seconds", "tolerance": 0.6}, "the pack's own length; the reading ends where its last segment ends")
    # language: the truth is the pack's; an upload is made with no language chosen, so the worker has to find it
    for code in ("ps", "ur", "en"):
        mine = audio_files(language=code)
        add("speech", "audio_in_language", "Which audio recordings are in %s?" % LANGUAGE_NAMES[code], {"kind": "files", "values": mine, "universe": universe}, "no language was chosen at upload: the worker identified it; exactly these recordings")
        add("speech", "audio_language_count", "How many audio recordings are spoken in %s?" % LANGUAGE_NAMES[code], {"kind": "scalar", "value": len(mine)})
    for f, how in ((files[4], "What language is %s in?"), (files[0], "In which language is %s spoken?"), (files[8], "What language does %s use?"), (files[9], "What language is %s in?")):
        code = pack[f]["language"]
        add("speech", "audio_language_of", how % f, {"kind": "words", "any": LANGUAGE_WORDS[code][0] + [code], "forbid": LANGUAGE_WORDS[code][1]}, "the language the pack says the speaker used (%s)" % LANGUAGE_NAMES[code])
    add("speech", "audio_review", "Which audio recordings need manual review?", {"kind": "files", "values": files, "universe": universe}, "every machine transcript needs a person's check, so all of them")
    # segments: the stored readings
    for f in (files[1], files[3]):
        add("speech", "audio_segments", "How many speech segments does %s have?" % f, {"kind": "scalar", "value": len(stored[f])}, "counted in the stored readings")
    add("speech", "audio_segments_all", "How many speech segments do the audio recordings have in all?", {"kind": "scalar", "value": sum(len(v) for v in stored.values())}, "counted in the stored readings of the audio files (the soundtracks of the video clips are not counted)")
    # words: a word that the stored reading holds, in a recording that was recorded from a text that holds it
    seen = set()
    for code, phrasing in (("en", "Which audio recordings mention the word %s?"), ("ur", "Which audio recordings say %s?"), ("ps", "Which audio recordings contain the word %s?"), ("en", "Find the recordings where someone says %s"), ("ur", "Which audio recordings mention the word %s?")):
        for f in audio_files(language=code):
            reference = [r.lower() for r in tokens(pack[f]["reference"])]
            candidates = [w for w in tokens(" ".join(s["text"] for s in stored[f])) if len(w) >= 4 and w.lower() not in seen and w.lower() in reference]
            if candidates:
                word = sorted(candidates, key=lambda w: (-len(w), w))[0]
                seen.add(word.lower())
                hits = sorted(g for g in files if word.lower() in [w.lower() for w in tokens(" ".join(s["text"] for s in stored[g]))])
                said = sorted(g for g in files if word.lower() in [w.lower() for w in tokens(pack[g]["reference"])])
                add("speech", "audio_word_search", phrasing % word, {"kind": "files", "values": hits, "universe": universe},
                    "the recordings whose stored reading holds the word (%s); the recordings whose reference text holds it: %s" % (", ".join(hits), ", ".join(said) or "none"))
                break
    # what a recording says: three words of its stored reading must be in the answer
    for f, phrasing in ((files[2], "What does %s say?"), (files[0], "Give me the transcript of %s"), (files[1], "What is said in %s?"), (files[9], "Read me %s")):
        words = [w for w in tokens(" ".join(s["text"] for s in stored[f])) if len(w) >= 4][:6]
        pick = [words[0], words[2], words[4]] if len(words) >= 5 else words[:3]
        add("speech", "audio_transcript", phrasing % f, {"kind": "words", "all": pick}, "three words of the stored reading, which must be in the answer; the reading is a machine transcript (Pashto: a search lead)")


def build():
    bank = Bank()
    add = bank.add
    subs = [r[0] for r in psql("select %s s from %s group by 1 order by count(*) desc" % (SUB, COMM))]
    sms_subs = [r[0] for r in psql("select %s from %s and c.event_type = 'sms' group by 1 order by count(*) desc" % (SUB, COMM))]

    # ------------------------------------------------------------------ calls and messages of one subscriber
    for s in subs:
        v = "%s and %s = '%s' and c.event_type = 'voice'" % (COMM, SUB, s)
        add("calls", "voice_count", "How many voice calls does %s have in this case?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s" % v))}, "calls made and received, from the number's own records")
        add("calls", "voice_out", "How many outgoing voice calls did %s make?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and c.direction = 'outgoing'" % v))})
        add("calls", "voice_in", "How many incoming voice calls did %s receive?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and c.direction = 'incoming'" % v))})
        add("calls", "talk_time", "What is the total talk time of %s across all its voice calls, in seconds?" % s, {"kind": "scalar", "value": int(one("select coalesce(sum(c.duration_s), 0) from %s" % v)), "unit": "seconds"})
        add("calls", "longest", "What is the duration of the longest voice call of %s, in seconds?" % s, {"kind": "scalar", "value": int(one("select max(c.duration_s) from %s" % v)), "unit": "seconds"})
        add("calls", "average", "What is the average duration of the voice calls of %s, in seconds?" % s, {"kind": "scalar", "value": round(float(one("select avg(c.duration_s) from %s" % v)), 2), "unit": "seconds"}, "103.40 and 103.4 are the same answer")
        add("calls", "over10", "How many voice calls of %s lasted more than 10 minutes?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and c.duration_s > 600" % v))})
        add("calls", "night", "How many voice calls of %s started between 00:00 and 04:59 Karachi time?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and extract(hour from c.start_utc at time zone '%s') < 5" % (v, TZ)))}, "case clock; the hour 05:00 to 05:59 is not included")
        both = "%s and %s = '%s'" % (COMM, SUB, s)
        add("calls", "first_event", "When was the first call or message of %s?" % s, {"kind": "text", "value": one("select to_char(min(c.start_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, both))}, "on the case clock")
        add("calls", "last_event", "When was the last call or message of %s?" % s, {"kind": "text", "value": one("select to_char(max(c.start_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, both))}, "on the case clock")
        top = unique_top(psql("select c.dst_key, count(*) from (select coalesce(c.b_norm, c.b_raw) dst_key from %s and c.direction = 'outgoing' and c.b_norm is not null) c group by 1 order by 2 desc, 1 limit 4" % v.replace("forensic.comm_event c WHERE", "forensic.comm_event c WHERE") ))
        if top:
            add("calls", "top_called", "Which number did %s call the most with outgoing voice calls?" % s, {"kind": "top", "value": top[0][0], "rivals": [r[0] for r in top[1:]], "count": int(top[0][1])}, "%s calls; the next number has %s" % (top[0][1], top[1][1] if len(top) > 1 else "none"))
        day = unique_top(psql("select to_char(c.start_utc at time zone '%s', 'YYYY-MM-DD') d, count(*) from %s group by 1 order by 2 desc, 1 limit 3" % (TZ, v)))
        if day:
            add("calls", "busiest_day", "Which day had the most voice calls for %s?" % s, {"kind": "top", "value": day[0][0], "rivals": [r[0] for r in day[1:]], "count": int(day[0][1])}, "%s voice calls that day" % day[0][1])
        distinct = int(one("select count(distinct c.b_norm) from %s and c.b_kind in ('person_like', 'international')" % both))
        add("calls", "distinct_contacts", "How many different numbers did %s contact, leaving out services?" % s, {"kind": "scalar", "value": distinct}, "person-like and international numbers; short codes and sender names are services")

    for s in sms_subs[:6]:
        m = "%s and %s = '%s' and c.event_type = 'sms'" % (COMM, SUB, s)
        add("messages", "sms_count", "How many text messages does %s have in this case?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s" % m))}, "sent and received, from the number's own records")
        add("messages", "sms_out", "How many text messages did %s send?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and c.direction = 'outgoing'" % m))})
        add("messages", "sms_in", "How many text messages did %s receive?" % s, {"kind": "scalar", "value": int(one("select count(*) from %s and c.direction = 'incoming'" % m))})
        top = unique_top(psql("select coalesce(c.b_norm, c.b_raw), count(*) from %s and c.direction = 'outgoing' and coalesce(c.b_norm, c.b_raw) is not null group by 1 order by 2 desc, 1 limit 4" % m))
        if top:
            add("messages", "sms_top", "Which number did %s send the most text messages to?" % s, {"kind": "top", "value": top[0][0], "rivals": [r[0] for r in top[1:]], "count": int(top[0][1])})
        add("messages", "sms_first", "When was the first text message of %s?" % s, {"kind": "text", "value": one("select to_char(min(c.start_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, m))}, "on the case clock")

    # ------------------------------------------------------------------ pairs: one subscriber and one other number
    for s in subs[:8]:
        pairs = unique_top(psql("select c.b_norm, count(*) from %s and %s = '%s' and c.b_norm is not null and c.b_kind in ('person_like', 'international') group by 1 order by 2 desc, 1 limit 3" % (COMM, SUB, s)))
        if not pairs:
            continue
        other = pairs[0][0]
        p = "%s and %s = '%s' and c.b_norm = '%s'" % (COMM, SUB, s, other)
        add("pairs", "pair_events", "How many calls and messages did %s have with %s?" % (s, other), {"kind": "scalar", "value": int(one("select count(*) from %s" % p))}, "in the records of the first number, both directions")
        add("pairs", "pair_voice_out", "How many outgoing voice calls did %s make to %s?" % (s, other), {"kind": "scalar", "value": int(one("select count(*) from %s and c.event_type = 'voice' and c.direction = 'outgoing'" % p))})
        add("pairs", "pair_talk", "What is the total duration of the voice calls between %s and %s, in seconds?" % (s, other), {"kind": "scalar", "value": int(one("select coalesce(sum(c.duration_s), 0) from %s and c.event_type = 'voice'" % p)), "unit": "seconds"})
        add("pairs", "pair_last", "When was the last contact between %s and %s?" % (s, other), {"kind": "text", "value": one("select to_char(max(c.start_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, p))}, "on the case clock")

    # ------------------------------------------------------------------ internet sessions and app usage
    for s, n in [(r[0], r[1]) for r in psql("select s.msisdn, count(*) from %s and s.msisdn is not null group by 1 order by 2 desc limit 6" % SESS)]:
        q = "%s and s.msisdn = '%s'" % (SESS, s)
        add("internet", "sessions", "How many internet sessions does %s have?" % s, {"kind": "scalar", "value": int(n)})
        add("internet", "bytes_down", "How many bytes did %s download in total?" % s, {"kind": "scalar", "value": int(one("select coalesce(sum(s.bytes_down), 0) from %s" % q)), "unit": "bytes"})
        add("internet", "bytes_up", "How many bytes did %s upload in total?" % s, {"kind": "scalar", "value": int(one("select coalesce(sum(s.bytes_up), 0) from %s" % q)), "unit": "bytes"})
        add("internet", "apps_distinct", "How many different apps did %s use in its internet sessions?" % s, {"kind": "scalar", "value": int(one("select count(distinct s.app) from %s and s.app is not null" % q))})
        top = unique_top(psql("select s.app, count(*) from %s and s.app is not null group by 1 order by 2 desc, 1 limit 3" % q))
        if top:
            add("internet", "top_app", "Which app did %s use the most in its internet sessions?" % s, {"kind": "top", "value": top[0][0], "rivals": [r[0] for r in top[1:]], "count": int(top[0][1])}, "by number of sessions")
        host = unique_top(psql("select s.host, count(*) from %s and s.host is not null group by 1 order by 2 desc, 1 limit 3" % q))
        if host:
            add("internet", "top_host", "Which host did %s connect to most often?" % s, {"kind": "top", "value": host[0][0], "rivals": [r[0] for r in host[1:]], "count": int(host[0][1])})
    add("internet", "most_sessions", "Which number has the most internet sessions?", {"kind": "top", "value": psql("select s.msisdn from %s and s.msisdn is not null group by 1 order by count(*) desc, 1 limit 1" % SESS)[0][0], "rivals": [], "count": 0})
    for s, n in [(r[0], r[1]) for r in psql("select u.msisdn, count(*) from %s and u.msisdn is not null group by 1 order by 2 desc limit 3" % USAGE)]:
        q = "%s and u.msisdn = '%s'" % (USAGE, s)
        add("apps", "usage_count", "How many app usage records does %s have?" % s, {"kind": "scalar", "value": int(n)})
        add("apps", "usage_distinct", "How many different apps did %s use?" % s, {"kind": "scalar", "value": int(one("select count(distinct u.app) from %s and u.app is not null" % q))})
        top = unique_top(psql("select u.app, count(*) from %s and u.app is not null group by 1 order by 2 desc, 1 limit 3" % q))
        if top:
            add("apps", "usage_top", "Which app did %s use the most?" % s, {"kind": "top", "value": top[0][0], "rivals": [r[0] for r in top[1:]], "count": int(top[0][1])})

    # ------------------------------------------------------------------ bank and wallet
    accounts = [r[0] for r in psql("select b.account_norm from %s group by 1 order by count(*) desc limit 12" % BANK)]
    for a in accounts:
        q = "%s and b.account_norm = '%s'" % (BANK, a)
        add("bank", "txn_count", "How many transactions does the account %s have?" % a, {"kind": "scalar", "value": int(one("select count(*) from %s" % q))})
        add("bank", "credits_total", "What is the total amount credited to the account %s?" % a, {"kind": "scalar", "value": round(float(one("select coalesce(sum(b.amount), 0) from %s and b.direction = 'credit'" % q)), 2), "unit": "rupees"})
        add("bank", "debits_total", "What is the total amount debited from the account %s?" % a, {"kind": "scalar", "value": round(float(one("select coalesce(sum(b.amount), 0) from %s and b.direction = 'debit'" % q)), 2), "unit": "rupees"})
        add("bank", "largest_debit", "What is the largest single debit of the account %s?" % a, {"kind": "scalar", "value": round(float(one("select max(b.amount) from %s and b.direction = 'debit'" % q)), 2), "unit": "rupees"})
        last = psql("select b.balance_after from %s and b.balance_after is not null order by b.txn_utc desc, b.txn_id desc limit 1" % q)
        if last:
            add("bank", "last_balance", "What was the balance after the last transaction of the account %s?" % a, {"kind": "scalar", "value": round(float(last[0][0]), 2), "unit": "rupees"})
        add("bank", "first_day", "On which day was the first transaction of the account %s?" % a, {"kind": "text", "value": one("select to_char(min(b.txn_utc at time zone '%s'), 'YYYY-MM-DD') from %s" % (TZ, q))}, "on the case clock")
        add("bank", "last_day", "On which day was the last transaction of the account %s?" % a, {"kind": "text", "value": one("select to_char(max(b.txn_utc at time zone '%s'), 'YYYY-MM-DD') from %s" % (TZ, q))}, "on the case clock")
        pay = unique_top(psql("select coalesce(b.counterparty_account_norm, b.counterparty_msisdn), sum(b.amount) from %s and b.direction = 'debit' and coalesce(b.counterparty_account_norm, b.counterparty_msisdn) is not null group by 1 order by 2 desc, 1 limit 3" % q))
        if pay:
            add("bank", "top_payee", "Who did the account %s pay the most money to?" % a, {"kind": "top", "value": pay[0][0], "rivals": [r[0] for r in pay[1:]], "count": float(pay[0][1])}, "by total amount debited")
        add("bank", "payees_distinct", "How many different accounts or numbers did the account %s pay?" % a, {"kind": "scalar", "value": int(one("select count(distinct coalesce(b.counterparty_account_norm, b.counterparty_msisdn)) from %s and b.direction = 'debit' and coalesce(b.counterparty_account_norm, b.counterparty_msisdn) is not null" % q))})
        senders = [r[0] for r in psql("select distinct coalesce(b.counterparty_account_norm, b.counterparty_msisdn) from %s and b.direction = 'credit' and coalesce(b.counterparty_account_norm, b.counterparty_msisdn) is not null order by 1" % q)]
        if 1 <= len(senders) <= 8:
            add("bank", "senders", "Which accounts or numbers sent money to the account %s?" % a, {"kind": "set", "values": senders})
        biggest = psql("select b.amount from %s and b.direction = 'debit' order by b.amount desc limit 1" % q)
        if biggest:
            limit = int(float(biggest[0][0]) // 2)
            add("bank", "debits_over", "How many debits of the account %s were above %s rupees?" % (a, limit), {"kind": "scalar", "value": int(one("select count(*) from %s and b.direction = 'debit' and b.amount > %d" % (q, limit)))}, "strictly above")

    # ------------------------------------------------------------------ vehicle plates
    plates = psql("select p.plate_norm, count(*), count(distinct p.camera_id) from %s group by 1 order by 2 desc limit 8" % PLATE)
    for plate, n, cams in plates:
        q = "%s and p.plate_norm = '%s'" % (PLATE, plate)
        add("plates", "plate_count", "How many times was the plate %s seen?" % plate, {"kind": "scalar", "value": int(n)})
        add("plates", "plate_cameras", "At which cameras was the plate %s seen?" % plate, {"kind": "set", "values": [r[0] for r in psql("select distinct p.camera_id from %s order by 1" % q)]})
        add("plates", "plate_first", "When was the plate %s first seen?" % plate, {"kind": "text", "value": one("select to_char(min(p.observed_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, q))}, "on the case clock")
        add("plates", "plate_last", "When was the plate %s last seen?" % plate, {"kind": "text", "value": one("select to_char(max(p.observed_utc at time zone '%s'), 'YYYY-MM-DD HH24:MI:SS') from %s" % (TZ, q))}, "on the case clock")
    for cam, n, distinct in psql("select p.camera_id, count(*), count(distinct p.plate_norm) from %s group by 1 order by 1" % PLATE):
        add("plates", "camera_count", "How many plate sightings did the camera %s record?" % cam, {"kind": "scalar", "value": int(n)})
        add("plates", "camera_plates", "How many different plates did the camera %s see?" % cam, {"kind": "scalar", "value": int(distinct)})
    tied = [r[0] for r in plates[1:] if r[1] == plates[0][1]]
    add("plates", "most_seen", "Which plate was seen the most?", {"kind": "top", "value": plates[0][0], "ties": tied, "rivals": [r[0] for r in plates[1:3] if r[1] != plates[0][1]], "count": int(plates[0][1])},
        "plates that share the top count are all in the answer (the judge reads ties as a set)" if tied else "")

    # ------------------------------------------------------------------ handsets
    for s in subs:
        imeis = [r[0] for r in psql("select distinct c.imei from %s and %s = '%s' and c.imei is not null order by 1" % (COMM, SUB, s))]
        if imeis:
            add("handsets", "imei_of", "Which handset (IMEI) was %s seen with?" % s, {"kind": "set", "values": imeis})
            add("handsets", "imei_count", "How many different handsets was %s seen with?" % s, {"kind": "scalar", "value": len(imeis)})

    # ------------------------------------------------------------------ cell sites
    for s in subs[:8]:
        cells = unique_top(psql("select c.cell_key, count(*) from %s and %s = '%s' and c.cell_key is not null group by 1 order by 2 desc, 1 limit 3" % (COMM, SUB, s)))
        if cells:
            add("cells", "top_cell", "Which cell site did %s use the most?" % s, {"kind": "top", "value": cells[0][0], "rivals": [r[0] for r in cells[1:]], "count": int(cells[0][1])}, "by number of events")
        add("cells", "cells_distinct", "How many different cell sites did %s use?" % s, {"kind": "scalar", "value": int(one("select count(distinct c.cell_key) from %s and %s = '%s' and c.cell_key is not null" % (COMM, SUB, s)))})

    # ------------------------------------------------------------------ registrations
    regs = psql("select r.msisdn, count(*), min(r.imsi), min(r.status) from forensic.subscriber_registration r where r.collection_id = '%s' and r.msisdn is not null and r.imsi is not null group by 1 having count(*) = 1 order by 1 limit 8" % COLLECTION)
    for n, _, imsi, status in regs:
        add("registrations", "reg_imsi", "Which SIM identity (IMSI) is registered for the number %s?" % n, {"kind": "set", "values": [imsi]}, "a registration is what a source file states, not who holds the number")
        if status:
            add("registrations", "reg_status", "What is the status of the registration of the number %s?" % n, {"kind": "set", "values": [status]})

    # ------------------------------------------------------------------ the evidence files
    for modality, n in psql("select i.modality, count(*) from forensic.evidence_items i where i.collection_id = '%s' group by 1 order by 1" % COLLECTION):
        add("inventory", "files_by_modality", "How many %s files are in this case?" % {"structured_records": "structured record", "text": "text", "document": "document", "tabular": "tabular"}.get(modality, modality), {"kind": "scalar", "value": int(n)}, "the evidence files of the case, by their type")
    add("inventory", "files_total", "How many evidence files are in this case?", {"kind": "scalar", "value": int(one("select count(*) from forensic.evidence_items i where i.collection_id = '%s'" % COLLECTION))})

    # ------------------------------------------------------------------ the photos added to the case (plate search over image readings)
    # The oracle is the construction key, not the engine: each constructed photo is a real car photo with an Orion plate written on it, so the plate it shows is known. The engine reads the plate with a
    # model, so the photo may be listed as an exact match or as a "similar reading" (one or two characters different); either way the file must be named. Real labelled photos are not in this bank:
    # their labels are the owner's and stay local.
    key_path = PLATE_KEY if os.path.exists(PLATE_KEY) else None
    if key_path:
        for image in json.load(open(key_path, encoding="utf-8"))["images"]:
            if image.get("kind") == "constructed" and image.get("plate_written"):
                bank.add("imgplate", "plate_in_image", "Which photos show the plate %s?" % image["plate_written"], {"kind": "set", "values": [image["file"]]},
                         "a constructed photo: a real car photo with this plate written on it; listed as an exact match or as a similar reading to check by eye")
        bank.add("imgplate", "plate_in_image_none", "Which photos show the plate QZX7731?", {"kind": "text", "value": "No image or video reading matches the plate QZX7731 exactly"}, "a plate that is in no photo: no exact match is reported")

    # ------------------------------------------------------------------ the video clips added to the case (real footage: the plates stay in the private bank)
    for question in video_questions(COLLECTION):
        add_video(bank, question)
    add_speech(bank, COLLECTION)
    typed = Bank()
    for plate, clips in typed_plate_searches(COLLECTION):
        typed.add("vsearch", "typed_plate_search", "Which photos or videos show the plate %s?" % plate, {"kind": "video", "vkind": "named", "value": clips, "what": "files"},
                  "a plate typed by the analyst: every clip that holds a reading of exactly that plate must be named; a reading that differs by one or two characters may be listed as 'check by eye'")
    bank.typed = typed.items

    return bank


def private_plates(bank):
    """Every plate string of the private side, to prove that none of them reached the public bank."""
    plates = set()
    for values in bank.private.values():
        plates.update(str(v) for v in values)
    for item in bank.typed:
        plates.add(item["question"].rsplit(" ", 1)[1].rstrip("?"))
    return plates


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", required=True)
    ap.add_argument("--private-out", default=PRIVATE_BANK, help="where the plates read from the real footage go; refused inside the repository")
    args = ap.parse_args(argv)
    private_path = os.path.abspath(args.private_out)
    try:
        inside = os.path.commonpath([os.path.normcase(private_path), os.path.normcase(REPO)]) == os.path.normcase(REPO)
    except ValueError:      # another drive: certainly outside
        inside = False
    if inside:
        raise SystemExit("the private bank holds plates read from real footage and must not be inside the repository: " + private_path)
    bank = build()
    items = bank.items
    areas = {}
    for i in items:
        areas[i["area"]] = areas.get(i["area"], 0) + 1
    public_text = json.dumps({"collection": COLLECTION, "built": datetime.date.today().isoformat(), "questions": items}, indent=1, ensure_ascii=False) + "\n"
    leaked = sorted(p for p in private_plates(bank) if p in public_text)
    if leaked:
        raise SystemExit("%d plate(s) of the private bank occur in the public bank; nothing was written" % len(leaked))
    with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
        handle.write(public_text)
    print("%d questions -> %s" % (len(items), args.out))
    for area, n in sorted(areas.items()):
        print("  %-14s %d" % (area, n))
    if bank.private or bank.typed:
        os.makedirs(os.path.dirname(private_path), exist_ok=True)
        with open(private_path, "w", encoding="utf-8", newline="\n") as handle:
            json.dump({"collection": COLLECTION, "built": datetime.date.today().isoformat(), "values": bank.private, "questions": bank.typed}, handle, indent=1, ensure_ascii=False)
            handle.write("\n")
        print("private bank (plates read from the footage; this machine only): %d expected plate lists, %d typed plate searches -> %s" % (len(bank.private), len(bank.typed), private_path))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
