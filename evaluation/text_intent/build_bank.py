#!/usr/bin/env python3
"""Build a text-search intent bank (docs/roadmap/text-intent-preregistration-20261110.md, Amendment 1).

    python -I -X utf8 evaluation/text_intent/build_bank.py --out evaluation/text_intent/bank-dev2-20261110.json                   # development bank 2 (run 1 used the first bank)
    python -I -X utf8 evaluation/text_intent/build_bank.py --seed holdout --frames holdout \
        --exclude evaluation/text_intent/bank-20261110.json evaluation/text_intent/bank-dev2-20261110.json \
        --out evaluation/text_intent/bank-holdout-20261110.json                                                                    # the holdout the gates are judged on

Positives are drawn from the passages that exist in the synthetic / public-data cases: for each family, literals of one to three words that occur in at least one and at most thirty passages
(and that neither begin nor end with one of the family's forty most frequent words) are put into sentences of different shapes: the source named or not, the literal capitalised differently
from the passage, Urdu literals in Urdu sentences that name the right source. The oracle is computed here, from the stored passage text, in Python and independently of the product's matching
code: Unicode NFC, whitespace collapsed, case folded, substring; for audio both the segment-level lower bound and the whole-recording upper bound (the product also matches across up to four
adjacent segments). Negatives are taken from the verified bank plus conversational and hard negatives. Nothing here is a product rule: the sentences are test inputs.

Reads the live database through `docker exec` (read only). Real-data cases are never named here.
"""
import argparse
import collections
import hashlib
import json
import re
import subprocess
import unicodedata

PG = ["docker", "exec", "-i", "nexusai-forensic-postgres-1", "psql", "-U", "localrecall", "-d", "localrecall", "-At"]
# Cases that are synthetic or built from public data. The real private demo case is not in this list on purpose.
CASES = {
    "nexusai-multimodal-product-acceptance": ("document", "audio", "image"),
    "nexusai-media-wave1-20261021": ("audio", "image"),
    "nexusai-wave1-dev3-20261021": ("document",),
    "orion-lahore-20261030": ("document",),
}
TYPES = {
    "document": ["forensics.document-native-text-passage/v1"],
    "audio": ["forensics.audio-timestamp-segment/v1", "forensics.audio-roman-urdu-segment/v1"],
    "image": ["forensics.image-ocr-observation/v1"],
}
SQL = """
SELECT json_build_object('artifact_id', a.artifact_id::text, 'evidence_id', a.evidence_id::text, 'file', i.original_filename, 'type', a.artifact_type,
  'start', (a.citation_locator->>'start_seconds'),
  'text', CASE WHEN a.artifact_type='forensics.image-ocr-observation/v1'
       THEN coalesce(nullif(a.metadata#>>'{observation,raw_text}', ''), nullif(a.metadata#>>'{observation,normalized_text}', ''))
       ELSE coalesce(a.metadata->>'text', a.metadata#>>'{observation,roman_urdu_text}', a.metadata#>>'{observation,text}', a.metadata#>>'{observation,raw_urdu_text}') END)
FROM forensic.derived_artifacts a
JOIN forensic.evidence_items i ON i.tenant_id=a.tenant_id AND i.collection_id=a.collection_id AND i.evidence_id=a.evidence_id AND i.current_version_id=a.version_id
WHERE a.tenant_id='default' AND a.collection_id='{case}' AND a.processing_status='completed' AND a.artifact_type IN ({types})
ORDER BY a.artifact_id"""

# Sentence shapes. A named-source shape says which kind of source (no shape that says only "files"); the "any" shapes name none.
FRAME_SETS = {
    "dev": {
        "document": ["Which documents mention {L}?", "search the documents for {L}", "is there any PDF or Word file that talks about {L}", "find {L} in the documents",
                     "do any of the statements contain {L}", "where in the paperwork does {L} come up?", "documents with {L} please"],
        "audio": ["Did anyone say {L} in the recordings?", "find the audio where {L} is mentioned", "search the call transcripts for {L}",
                  "which voice notes talk about {L}", "is {L} spoken anywhere in the audio?"],
        "image": ["Which images have the text {L} on them?", "search the screenshots for {L}", "find photos showing {L}",
                  "any scanned page or picture with {L} written?", "OCR search for {L}"],
        "any": ["Is {L} mentioned anywhere in the case?", "find {L}", "where does {L} appear?", "anything about {L} in the evidence?", "check whether {L} shows up in any file"],
    },
    "holdout": {
        "document": ["list the documents where {L} is written", "I need every PDF that has {L} in it", "look through the letters and statements for {L}", "which emails refer to {L}?",
                     "does the paperwork say {L} anywhere", "show me documents containing {L}"],
        "audio": ["who says {L} in the calls?", "check the voice recordings for {L}", "was {L} ever spoken in the intercepted audio", "which recordings include the words {L}?",
                  "look in the transcripts of the calls for {L}"],
        "image": ["which pictures contain the writing {L}?", "check the photographs for the text {L}", "look at the scans for {L}", "any screenshot that reads {L}?",
                  "find {L} in the images"],
        "any": ["is there any trace of {L} in this case?", "where do we see {L}?", "can you find {L} for me", "has {L} come up at all?", "look for {L} across everything we hold"],
    },
}
URDU_FRAME_SETS = {
    "dev": {
        "audio": ["کیا ریکارڈنگ میں {L} کا ذکر ہے؟", "آڈیو میں {L} تلاش کریں"],
        "image": ["تصویر میں {L} لکھا ہے؟", "اسکرین شاٹ میں {L} تلاش کریں"],
        "document": ["کیا دستاویز میں {L} کا ذکر ہے؟", "دستاویزات میں {L} تلاش کریں"],
        "any": ["{L} کہاں آیا ہے؟", "کیا کیس میں {L} کا ذکر ہے؟"],
    },
    "holdout": {
        "audio": ["کالز میں {L} کس نے کہا؟", "ریکارڈ شدہ آواز میں {L} دیکھیں"],
        "image": ["کون سی تصویر پر {L} درج ہے؟", "سکین شدہ صفحات میں {L} ڈھونڈیں"],
        "document": ["کن دستاویزات میں {L} موجود ہے؟", "خطوط اور بیانات میں {L} دیکھیں"],
        "any": ["کیا {L} کہیں بھی ملتا ہے؟", "{L} کا کوئی سراغ؟"],
    },
}
PER_FAMILY = {"document": 24, "audio": 12, "image": 12}
ANY_PER_FAMILY = 3  # source-free sentences per family (expected family: any)

NEGATIVES = {
    "dev": {
        "conversation": ["hello, how are you?", "what is an IMEI?", "what can you do?", "what does the audio transcript say?", "summarise the documents", "how many documents are in this case?",
                         "list the files that were ingested", "thanks, that helps", "explain what a CDR is", "which evidence types do you support?"],
        "hard": ["Which text messages contain the word otp?", "show messages whose body mentions urgent", "which bank transactions have the remark salary",
                 "find calls to a number containing 0300", "which vehicles have a plate containing LEA", "list subscribers whose name contains Khan",
                 "does any transaction description say rent", "show the SMS that say meet me"],
    },
    "holdout": {
        "conversation": ["good morning", "what does ANPR stand for?", "can you read Urdu?", "give me an overview of the case", "how many recordings do we have?",
                         "which file types can I upload?", "what is the difference between a CDR and an IPDR?", "you were very quick, well done", "read the first document to me", "who are you?"],
        "hard": ["which messages mention the word password", "find transactions whose remark contains invoice", "which calls lasted longer than ten minutes",
                 "list vehicles with plates starting with LHR", "which subscribers are registered under the name Ahmed", "show the cell sites near the airport",
                 "does any bank remark say loan", "which SMS were sent after midnight"],
    },
}


def norm(text):
    return unicodedata.normalize("NFC", " ".join((text or "").split())).casefold()


def fetch(case, family):
    types = ",".join("'%s'" % t for t in TYPES[family])
    out = subprocess.run(PG + ["-c", SQL.replace("{case}", case).replace("{types}", types)], capture_output=True, text=True, encoding="utf-8", check=True).stdout
    rows = [json.loads(line) for line in out.splitlines() if line.strip()]
    return [r for r in rows if (r.get("text") or "").strip()]


WORD = re.compile(r"[^\W\d_]{3,}(?:[-'][^\W\d_]+)*|\d{4,}", flags=re.UNICODE)


def grams(text, family):
    """Words, and runs of two or three words that touch with plain white space between them, so every gram is a substring of the normalised passage."""
    flat = " ".join((text or "").split())
    spans = [(m.start(), m.end()) for m in WORD.finditer(flat)]
    out = set()
    for size in (1, 2, 3):
        if family == "audio" and size == 3:
            continue
        for i in range(len(spans) - size + 1):
            window = spans[i:i + size]
            if all(flat[window[k][1]:window[k + 1][0]] == " " for k in range(size - 1)):
                out.add(flat[window[0][0]:window[-1][1]])
    return out


def oracle(rows, family, literal):
    """Evidence ids (lower bound: one passage holds it; upper bound for audio: the recording's joined text holds it) and the passage count."""
    needle = norm(literal)
    lower, passages, by_evidence = set(), 0, {}
    for row in rows:
        by_evidence.setdefault(row["evidence_id"], []).append(row)
        if needle in norm(row["text"]):
            lower.add(row["evidence_id"])
            passages += 1
    upper = set(lower)
    if family == "audio":
        for evidence_id, items in by_evidence.items():
            items.sort(key=lambda r: float(r["start"] or 0))
            if needle in norm(" ".join(r["text"] for r in items)):
                upper.add(evidence_id)
    return sorted(lower), sorted(upper), passages


def pick(candidates, seed, count):
    return sorted(candidates, key=lambda c: hashlib.sha256((seed + c).encode("utf-8")).hexdigest())[:count]


def vary(literal, index):
    return [literal, literal.lower(), literal.upper()][index % 3] if re.search(r"[A-Za-z]", literal) else literal


def is_urdu(text):
    return bool(re.search(r"[؀-ۿ]", text))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--out", required=True)
    parser.add_argument("--verified-bank", default="docs/qa/verified-bank-20261102.json")
    parser.add_argument("--seed", default="dev")
    parser.add_argument("--frames", default="dev", choices=sorted(FRAME_SETS))
    parser.add_argument("--exclude", nargs="*", default=[], help="banks whose literals and negatives must not be used again")
    args = parser.parse_args()
    excluded_literals, excluded_ids = set(), set()
    for path in args.exclude:
        previous = json.load(open(path, encoding="utf-8"))
        excluded_literals |= {norm(p["oracle_literal"]) for p in previous["positives"]}
        excluded_ids |= {n["id"] for n in previous["negatives"]}
    frames, urdu_frames = FRAME_SETS[args.frames], URDU_FRAME_SETS[args.frames]
    prefix = "TI-D-" if args.seed == "dev" else "TI-%s-" % args.seed[:1].upper()
    positives, counter = [], 0
    for case, families in CASES.items():
        for family in families:
            rows = fetch(case, family)
            frequency = {}
            for row in rows:
                for gram in grams(row["text"], family):
                    frequency.setdefault(norm(gram), set()).add(row["artifact_id"])
            token_df = collections.Counter()
            for row in rows:
                for token in {norm(w) for w in WORD.findall(" ".join(row["text"].split()))}:
                    token_df[token] += 1
            common = {token for token, _ in token_df.most_common(40)}  # language independent: the family's own most frequent words

            def fringe_free(gram):
                words = gram.split(" ")
                return words[0] not in common and words[-1] not in common
            pool = sorted(g for g, ids in frequency.items() if 1 <= len(ids) <= 30 and len(g) >= 5 and fringe_free(g) and g not in excluded_literals)
            urdu_pool = [g for g in pool if is_urdu(g)]
            latin_pool = [g for g in pool if not is_urdu(g)]
            quota = -(-PER_FAMILY[family] // sum(1 for f in CASES.values() if family in f))  # the family's share, split equally between the cases that hold it
            for literal in pick(latin_pool, args.seed + case + family, quota):
                counter += 1
                positives.append(dict(id="%s%03d" % (prefix, counter), case=case, family=family, literal=literal, rows=rows, urdu=False, named=True))
            for literal in pick(urdu_pool, args.seed + case + family + "ur", 3):
                counter += 1
                positives.append(dict(id="%s%03d" % (prefix, counter), case=case, family=family, literal=literal, rows=rows, urdu=True, named=True))
            for literal in pick(latin_pool, args.seed + case + family + "any", ANY_PER_FAMILY):
                counter += 1
                positives.append(dict(id="%s%03d" % (prefix, counter), case=case, family=family, literal=literal, rows=rows, urdu=False, named=False))
    bank_positive = []
    for n, item in enumerate(positives):
        family, literal = item["family"], item["literal"]
        shown = vary(literal, n)
        shape = (urdu_frames if item["urdu"] else frames)[family if item["named"] else "any"]
        sentence = shape[n % len(shape)].format(L=shown)
        lower, upper, passages = oracle(item["rows"], family, literal)
        bank_positive.append(dict(id=item["id"], case=item["case"], question=sentence, source_family=family,
                                  expected_family=family if item["named"] else "any", literal=shown, oracle_literal=literal,
                                  oracle_evidence_lower=lower, oracle_evidence_upper=upper, oracle_passages=passages))
    verified = json.load(open(args.verified_bank, encoding="utf-8"))["questions"]
    by_area = {}
    for q in verified:
        if q["id"] not in excluded_ids:
            by_area.setdefault(q["area"], []).append(q)
    negatives = []
    for area, items in sorted(by_area.items()):
        for q in pick([i["id"] for i in items], "neg" + args.seed + area, max(2, round(40 * len(items) / len(verified)))):
            text = next(i["question"] for i in items if i["id"] == q)
            negatives.append(dict(id=q, kind="plain", question=text, area=area))
    negatives = negatives[:40]
    tag = "D" if args.seed == "dev" else args.seed[:1].upper()
    for n, text in enumerate(NEGATIVES[args.frames]["conversation"]):
        negatives.append(dict(id="TI-%s-N%02d" % (tag, n + 1), kind="plain", question=text, area="conversation"))
    for n, text in enumerate(NEGATIVES[args.frames]["hard"]):
        negatives.append(dict(id="TI-%s-H%02d" % (tag, n + 1), kind="hard", question=text, area="structured-field"))
    bank = dict(built="2026-11-10", seed=args.seed, frames=args.frames, preregistration="docs/roadmap/text-intent-preregistration-20261110.md", positives=bank_positive, negatives=negatives)
    with open(args.out, "w", encoding="utf-8", newline="\n") as handle:
        json.dump(bank, handle, ensure_ascii=False, indent=1)
    families = {}
    for p in bank_positive:
        families[p["source_family"]] = families.get(p["source_family"], 0) + 1
    print("positives", len(bank_positive), families, "negatives", sum(n["kind"] == "plain" for n in negatives), "plain +", sum(n["kind"] == "hard" for n in negatives), "hard")
    print("source named:", sum(p["expected_family"] != "any" for p in bank_positive), "source free:", sum(p["expected_family"] == "any" for p in bank_positive),
          "urdu:", sum(is_urdu(p["literal"]) for p in bank_positive), "empty oracle:", sum(not p["oracle_evidence_upper"] for p in bank_positive))


if __name__ == "__main__":
    main()
