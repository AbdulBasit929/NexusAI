#!/usr/bin/env python3
"""Builds the Pakistani Punjabi (Shahmukhi) and Saraiki recording kit: read-aloud prompts taken from the two Wikipedias in their own languages (pnb.wikipedia.org and skr.wikipedia.org), so that
native speakers on the team can record them and the speech reading can be measured on a KNOWN text (speech lab, WP-M1 follow-up). Public text only; each prompt names its article and revision.

    python -I -X utf8 tools/media_data/build_pa_skr_kit.py --out-md docs/qa/punjabi-saraiki-recording-script-20261116.md --out-key docs/qa/punjabi-saraiki-recording-key-20261116.json

Wikipedia text is licensed CC BY-SA 4.0 (the licence of every Wikimedia project): the prompts below are copied with their source and the licence is stated in the script. A prompt is one sentence of
the introduction of an article, 8 to 18 words, written only in Perso-Arabic letters with the plain punctuation of the language (no Latin letters, digits, brackets or quotation marks), at most two per article.
The sentences are what the Wikipedia editors wrote: they are NOT checked by a native speaker (reviewer names, decision D10, are owed), so the speaker is asked to mark any sentence that is not natural or
that they cannot read, and the lab scores only the sentences the speaker kept.
"""
import argparse
import datetime
import json
import re
import sys
import time
import urllib.parse
import urllib.request

SITES = (("pnb", "Pakistani Punjabi (Shahmukhi)", "pa"), ("skr", "Saraiki", "skr"))
HEADERS = {"User-Agent": "nexusai-speech-kit/1 (public text only)"}
LETTERS = re.compile(r"[؀-ۿݐ-ݿﭐ-﷿ﹰ-﻿]")
BAD = re.compile(r"[A-Za-z0-9٠-٩۰-۹()\[\]{}<>\"'«»“”:;/\\|=*#@_]")


def api(site, **params):
    url = "https://%s.wikipedia.org/w/api.php?%s" % (site, urllib.parse.urlencode(dict(params, format="json")))
    with urllib.request.urlopen(urllib.request.Request(url, headers=HEADERS), timeout=60) as response:
        return json.loads(response.read().decode("utf-8"))


def sentences(text):
    for piece in re.split(r"(?<=[۔.!?؟])\s+|\n+", text or ""):
        piece = re.sub(r"\s+", " ", piece).strip()
        if piece:
            yield piece


def good(sentence):
    words = sentence.split()
    if not 8 <= len(words) <= 18 or not 40 <= len(sentence) <= 150 or BAD.search(sentence):
        return False
    letters = len(LETTERS.findall(sentence))
    return letters >= 0.8 * len(re.sub(r"[\s۔.,،!?؟]", "", sentence))


def collect(site, want):
    taken, per_article, seen = [], {}, set()
    rounds = 0
    while len(taken) < want and rounds < 80:
        rounds += 1
        data = api(site, action="query", generator="random", grnnamespace=0, grnlimit=50, prop="extracts|revisions", exintro=1, explaintext=1, exlimit="max", rvprop="ids")
        for page in (data.get("query", {}).get("pages") or {}).values():
            title, extract = page.get("title", ""), page.get("extract", "")
            revision = (page.get("revisions") or [{}])[0].get("revid")
            if not revision or per_article.get(title, 0) >= 2:
                continue
            for s in sentences(extract):
                if good(s) and s not in seen and per_article.get(title, 0) < 2 and len(taken) < want:
                    seen.add(s)
                    per_article[title] = per_article.get(title, 0) + 1
                    taken.append({"text": s, "article": title, "revision": revision, "url": "https://%s.wikipedia.org/w/index.php?oldid=%d" % (site, revision)})
        time.sleep(0.3)
    return taken


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out-md", required=True)
    ap.add_argument("--out-key", required=True)
    ap.add_argument("--per-language", type=int, default=60)
    args = ap.parse_args(argv)
    key, lines = [], []
    for site, name, code in SITES:
        picked = collect(site, args.per_language)
        print("%s: %d sentences from %d articles (%d rounds of the random page list)" % (site, len(picked), len({p["article"] for p in picked}), 0), flush=True)
        lines.append("\n## %s: %d sentences (`%s-phrase-01` ...)\n\n| # | Sentence | Source article (revision) | Keep? |\n|---|---|---|---|" % (name, len(picked), site))
        for n, p in enumerate(picked, 1):
            lines.append("| %d | %s | [%s](%s) | |" % (n, p["text"], p["article"].replace("|", "/"), p["url"]))
            key.append({"id": "%s-phrase-%02d" % (site, n), "language": site, "text": p["text"], "article": p["article"], "revision": p["revision"], "url": p["url"], "telephone_set": n <= 20})
    header = """# Pakistani Punjabi and Saraiki: recording script for speech tests, %s

**What this is.** %d short sentences in each language, to be read aloud and recorded by **native speakers**, so that the speech reading can be tested on a text we know. Punjabi here means the Punjabi written in Perso-Arabic script (Shahmukhi), as spoken in Pakistan; Saraiki is Saraiki.

**Where the sentences come from.** The introductions of articles of the Western Punjabi Wikipedia (pnb.wikipedia.org) and the Saraiki Wikipedia (skr.wikipedia.org), picked at random, one sentence of 8 to 18 words, at most two per article. Wikipedia text is licensed **CC BY-SA 4.0** (https://creativecommons.org/licenses/by-sa/4.0/); every sentence names its article and revision, and the recordings of it are a derivative that keeps the same licence. The sentences are what the Wikipedia editors wrote. **They have not been checked by a native speaker**, so some may read unnaturally.

**How to record** (the same as the Orion kit, `orion-speech-recording-script-20261113.md`).

* A phone voice recorder is enough. A quiet room, the phone about 20 cm from the mouth, one speaker at a time, normal speed, a short pause between sentences. **One sentence per file**: `pnb-phrase-01.m4a` ... `skr-phrase-60.m4a`. Any of wav, m4a, mp3, ogg works. Do not edit the audio.
* **Please mark "Keep?" with "no"** for any sentence you cannot read naturally, or that is not in your language (some Wikipedia sentences are mixed or odd). Only the sentences you keep are scored.
* **Telephone set.** For sentences **1 to 20 of each language, record them a second time over a real phone call**: call a second phone and read the sentence, and save the recording made on the receiving end. Name them `pnb-phrase-01-tel.m4a`. This is the quality investigators actually hear.
* Use your own natural voice. Men and women, different towns, are all useful. In a note beside the files write who recorded (a first name or initials is enough), the district you grew up in, and the phone.
* **Enough to start: 2 or 3 speakers per language, 20 sentences each.** More is better.

**What happens next.** The recordings are put through the normal upload (no language chosen), read by the speech reading, and compared with the sentence. Nothing is sent outside the machine. The recordings stay with the team and are not published.

*Built %s with `tools/media_data/build_pa_skr_kit.py`. The key to the sentences is `punjabi-saraiki-recording-key-20261116.json`.*
""" % (datetime.date.today().isoformat(), args.per_language, datetime.date.today().isoformat())
    open(args.out_md, "w", encoding="utf-8", newline="\n").write(header + "\n".join(lines) + "\n")
    open(args.out_key, "w", encoding="utf-8", newline="\n").write(json.dumps({"built": datetime.date.today().isoformat(), "licence": "CC BY-SA 4.0 (Wikipedia)", "sentences": key}, ensure_ascii=False, indent=1) + "\n")
    print("wrote", args.out_md, "and", args.out_key)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
