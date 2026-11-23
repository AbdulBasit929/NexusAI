#!/usr/bin/env python3
"""Baseline of the owner's rule "a search returns ALL occurrences, best and accurately" (feedback of 2026-11-19), on the synthetic Orion Lahore reference case.

Each probe is a word, phrase or identifier whose TRUE list of evidence files is computed from the database itself (documents, image OCR and audio segments of the case: forensic.derived_artifacts), and
each probe is asked in several natural ways (plain, indirect, formatted differently, Roman Urdu). The answer is scored on the files it cites: recall = true files cited / true files, precision =
true files cited / files cited. The wordings are NOT templates the engine is built around: they are test questions, and any change to the engine is judged on NEW wordings the engine has not been tuned to.

    python -X utf8 tools/showcase/ask_exhaustive_bank.py --out C:/tmp/nexusai-g4/orion_bank_baseline.jsonl [--only 1,2,3] [--bank bank-A]

Reads the API key from the main checkout's env file (never printed); writes nothing to the case.
"""
import argparse
import json
import os
import re
import subprocess
import sys
import time

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import ask_case  # noqa: E402

COLLECTION = "orion-lahore-reference-20261030"
URL = "http://127.0.0.1:8091"
DOCS_PHONE = ["D01-fir-english.docx", "D06-cdr-request.txt", "D09-utility-bill.txt", "D14-control-report.txt"]
TRUTH = {
    "phone": DOCS_PHONE,
    "reference": ["D01-fir-english.docx", "D09-utility-bill.txt", "D14-control-report.txt"],
    "synthetic": ["D01-fir-english.docx", "D04-witness-roman-urdu.txt", "D05-seizure-memo.docx", "D06-cdr-request.txt", "D07-bank-request.txt", "D08-subscriber-report.txt", "D09-utility-bill.txt",
                  "D10-vehicle-register.txt", "D11-chat-export.txt", "D13-extraction-index.txt", "D14-control-report.txt"],
    "imsi": ["D08-subscriber-report.txt", "ipdr-P-001-ipdr-i4-zong-txt.txt", "ipdr-P-005-ipdr-i4-zong-txt.txt", "ipdr-P-009-ipdr-i4-zong-txt.txt", "ipquery-P7-001-ipdr-i4-zong-txt.txt"],
    "name": ["D04-witness-roman-urdu.txt"],
    "plate": ["roadside-vehicle-02.jpg", "D03-witness-urdu.txt"],
    "account": ["D02-fir-urdu.docx", "D07-bank-request.txt", "D11-chat-export.txt"],  # D07 writes it with spaces (PK00 SYNT 0000 ...): the exact-substring truth of 2026-11-19 missed it; the tolerant search found it and a reading of the file confirms it
    "video": ["ipdr-P-001-ipdr-i4-zong-txt.txt", "ipdr-P-005-ipdr-i4-zong-txt.txt", "ipdr-P-009-ipdr-i4-zong-txt.txt", "ipquery-P7-001-ipdr-i4-zong-txt.txt", "highway-overpass-hd.mp4"],
    # bank-B probes (written 2026-11-19 AFTER the tolerant search was built; never used to tune it). The true lists come from an independent check: the exported passages of the case compared with
    # digits of every script folded to 0-9, separators removed, case folded, and for a mobile number the 0 / 92 / 0092 forms of the same national number. Same-script matches only.
    "b-mobile5": ["D02-fir-urdu.docx", "D05-seizure-memo.docx", "D07-bank-request.txt", "D08-subscriber-report.txt", "D12-complaint-urdu.docx", "D13-extraction-index.txt", "ipdr-P-005-ipdr-i4-zong-txt.txt"],
    "b-mobile7": ["D02-fir-urdu.docx", "D03-witness-urdu.txt", "D06-cdr-request.txt", "D11-chat-export.txt", "D13-extraction-index.txt"],
    "b-plate4024": ["D01-fir-english.docx", "D10-vehicle-register.txt", "D11-chat-export.txt", "roadside-vehicle-03.jpg"],
    "b-plate2019": ["D04-witness-roman-urdu.txt", "D10-vehicle-register.txt", "D12-complaint-urdu.docx"],
    "b-account2010": ["D01-fir-english.docx", "D07-bank-request.txt"],
    "b-amount": ["D07-bank-request.txt", "D09-utility-bill.txt", "D14-control-report.txt"],
    "b-meter": ["D09-utility-bill.txt"],
    "b-imei": ["D13-extraction-index.txt"],
    "b-mehwish": ["D01-fir-english.docx", "D11-chat-export.txt"],
    "b-garden": ["D04-witness-roman-urdu.txt", "D09-utility-bill.txt", "D11-chat-export.txt"],
    "b-johar": ["D01-fir-english.docx", "D08-subscriber-report.txt"],
    "b-masnooi": ["D02-fir-urdu.docx", "D03-witness-urdu.txt", "D12-complaint-urdu.docx"],
    "b-trousers": ["voice-note-03.wav", "voice-note-06.wav"],
    "b-hongkong": ["voice-note-01.wav", "voice-note-04.wav"],
    "b-section161": ["D01-fir-english.docx", "D04-witness-roman-urdu.txt", "D14-control-report.txt"],
    # bank-C probes (written 2026-11-20, after the shipped design was fixed; used once for the gate). Truth: the same independent check as bank-B (digits of every script folded, separators removed,
    # case folded, 0 / 92 / 0092 forms of a mobile number), same-script matches only.
    "c-mobile21": ["D01-fir-english.docx", "D12-complaint-urdu.docx"],
    "c-mobile10": ["D01-fir-english.docx", "D06-cdr-request.txt", "D08-subscriber-report.txt", "D11-chat-export.txt", "D13-extraction-index.txt"],
    "c-mobile04": ["D04-witness-roman-urdu.txt"],
    "c-plate2905": ["roadside-vehicle-01.jpg"],
    "c-imei17": ["D05-seizure-memo.docx"],
    "c-khalid": ["D09-utility-bill.txt"],
    "c-ali": ["D05-seizure-memo.docx"],
    "c-zainab": ["D11-chat-export.txt"],
    "c-faisal": ["D05-seizure-memo.docx"],
    "c-amount85": ["D05-seizure-memo.docx"],
    "c-amount350": ["D01-fir-english.docx"],
    "c-fir4526": ["D01-fir-english.docx", "D14-control-report.txt"],
    "c-date1803": ["D01-fir-english.docx", "ipdr-P-001-ipdr-i4-zong-txt.txt", "ipdr-P-005-ipdr-i4-zong-txt.txt", "ipdr-P-009-ipdr-i4-zong-txt.txt"],
    "c-dino": ["voice-note-07.wav"],
    "c-sharpener": ["voice-note-09.wav"],
    "c-victoria": ["voice-note-04.wav"],
    "c-handset": ["D05-seizure-memo.docx", "D13-extraction-index.txt"],
    "c-iccid": ["D08-subscriber-report.txt"],
    "c-lahore": ["D01-fir-english.docx", "D09-utility-bill.txt", "D13-extraction-index.txt"],
}
BANKS = {
    "bank-A": [  # the first set of wordings: used for the baseline and for any tuning
        ("phone", "Which files mention the number 042-35761234?"),
        ("phone", "Where does 042-35761234 appear in this case?"),
        ("phone", "find 04235761234"),
        ("phone", "Show every document that lists the helpline 042 35761234"),
        ("phone", "042-35761234 kis kis file mein likha hai?"),
        ("reference", "Which documents carry the reference REF00000394?"),
        ("reference", "search REF00000394"),
        ("reference", "Is the reference number REF00000394 written anywhere?"),
        ("reference", "REF00000394 kahan kahan aaya hai"),
        ("synthetic", "Which files say synthetic?"),
        ("synthetic", "Find every document that is marked as a synthetic test document"),
        ("synthetic", "how many and which documents contain the word SYNTHETIC"),
        ("synthetic", "show all evidence mentioning synthetic"),
        ("imsi", "Which files contain an IMSI?"),
        ("imsi", "Find all evidence that mentions IMSI"),
        ("imsi", "Where is the IMSI recorded in the case?"),
        ("imsi", "list the documents with IMSI values"),
        ("name", "Which statement was given by Hassan Farooqi?"),
        ("name", "find Hassan Farooqi"),
        ("name", "Does the name Hassan Farooqi appear in any evidence?"),
        ("plate", "Where does the plate SYNT9150 appear?"),
        ("plate", "Find every file that mentions the number plate SYNT9150"),
        ("plate", "Which images or documents show vehicle SYNT9150?"),
        ("plate", "SYNT-9150 gaari kahan kahan hai?"),
        ("account", "Which files mention the account PK00SYNT0000000000002005?"),
        ("account", "Find account PK00SYNT0000000000002005 in the evidence"),
        ("account", "where is bank account number PK00SYNT0000000000002005 used"),
        ("video", "Which evidence mentions the word video?"),
        ("video", "Find every recording or document that says video"),
        ("video", "where is video mentioned"),
    ],
    "bank-B": [  # NEW wordings and NEW identifiers, written after the build; used for the gates, never for tuning
        ("b-mobile5", "Which of the case files carry the mobile number 0300-0001005?"),
        ("b-mobile5", "list every piece of evidence where 03000001005 shows up"),
        ("b-mobile5", "+92 300 0001005 kis kis dastawez ya record mein hai?"),
        ("b-mobile7", "Which files contain the phone number 0300 0001007?"),
        ("b-mobile7", "everything that mentions 0300-0001007"),
        ("b-mobile7", "show me where the number +92-300-0001007 is written"),
        ("b-plate4024", "Which files mention the vehicle registration SYNT 4024?"),
        ("b-plate4024", "Where is SYNT-4024 written?"),
        ("b-plate4024", "SYNT4024 gaari ka zikr kahan kahan hai"),
        ("b-plate2019", "Find all evidence that names the number plate SYNT2019"),
        ("b-plate2019", "which documents mention vehicle number SYNT 2019"),
        ("b-plate2019", "SYNT-2019 appears in which files?"),
        ("b-account2010", "Which documents list the account PK00SYNT0000000000002010?"),
        ("b-account2010", "where is bank account PK00 SYNT 0000 0000 0000 2010 mentioned"),
        ("b-amount", "Which documents give the amount 1,234,567.00?"),
        ("b-amount", "find 1234567.00 in the evidence"),
        ("b-amount", "where does the figure 1,234,567.00 appear"),
        ("b-meter", "Which file has the meter number 112233445566?"),
        ("b-meter", "find meter 112233445566"),
        ("b-imei", "Where does the handset IMEI 000000000010058 appear in the case?"),
        ("b-imei", "search 000000000010058"),
        ("b-mehwish", "Which files mention Mehwish Bajwa?"),
        ("b-mehwish", "find every record of the person Mehwish Bajwa"),
        ("b-mehwish", "Mehwish Bajwa ka naam kahan kahan likha hai"),
        ("b-garden", "Which statements or documents mention Garden Town?"),
        ("b-garden", "find Garden Town in the evidence"),
        ("b-garden", "where is the area Garden Town named"),
        ("b-johar", "Which files talk about Johar Town?"),
        ("b-johar", "find every mention of Johar Town"),
        ("b-masnooi", "کن دستاویزات میں مصنوعی لکھا ہے؟"),
        ("b-masnooi", "find the Urdu word مصنوعی in the documents"),
        ("b-trousers", "Which recordings say trousers?"),
        ("b-trousers", "find every audio clip where trousers is mentioned"),
        ("b-trousers", "who talks about trousers covering the knees in the recordings"),
        ("b-hongkong", "کن ریکارڈنگز میں ہانگ کونگ کا ذکر ہے؟"),
        ("b-hongkong", "which voice notes talk about ہانگ کونگ"),
        ("b-section161", "Which documents refer to Section 161?"),
        ("b-section161", "find every mention of Section 161"),
        ("b-section161", "where is section 161 cited in the case"),
    ],
    "bank-C": [  # fresh probes and wordings, written after the shipped design was fixed; asked once for the gate
        ("c-mobile21", "Which documents contain the mobile number +92-300-0001021?"),
        ("c-mobile21", "is 03000001021 written anywhere in the evidence?"),
        ("c-mobile10", "Show every file that mentions the number 0300 000 1010"),
        ("c-mobile10", "where does +92 300 0001010 turn up in this case"),
        ("c-mobile10", "0300-0001010 kin kin files mein hai?"),
        ("c-mobile04", "Which statement mentions 0300-0001004?"),
        ("c-mobile04", "find the number +92 300 0001004"),
        ("c-plate2905", "Which image shows the plate SYNT2905?"),
        ("c-plate2905", "any file that mentions the registration SYNT-2905"),
        ("c-imei17", "Where is the IMEI 000000000010017 written?"),
        ("c-imei17", "find 000000000010017 in the documents"),
        ("c-khalid", "Which documents name Khalid Chandio?"),
        ("c-khalid", "is the name Khalid Chandio mentioned anywhere"),
        ("c-ali", "Which files mention Ali Orakzai?"),
        ("c-ali", "Ali Orakzai ka zikr kin dastawezat mein hai"),
        ("c-zainab", "Which chats mention Zainab Gill?"),
        ("c-zainab", "find Zainab Gill"),
        ("c-faisal", "Which document mentions Faisal Town?"),
        ("c-amount85", "Which documents state the amount PKR 85,000?"),
        ("c-amount85", "where is 85,000 written"),
        ("c-amount350", "Which document mentions the sum 350,000?"),
        ("c-fir4526", "Which documents refer to 45/26?"),
        ("c-fir4526", "find FIR 45/26 in the evidence"),
        ("c-date1803", "Which files carry the date 18/03/2026?"),
        ("c-date1803", "where does 18/03/2026 appear"),
        ("c-dino", "کن ریکارڈنگز میں ڈائنسور کا ذکر ہے؟"),
        ("c-dino", "which voice note talks about ڈائنسور"),
        ("c-sharpener", "Which recording mentions a sharpener?"),
        ("c-sharpener", "find the audio where someone says sharpener"),
        ("c-victoria", "کس ریکارڈنگ میں وکٹوریا ہاربر کا ذکر ہے؟"),
        ("c-handset", "Which documents talk about a handset?"),
        ("c-handset", "find every mention of handset"),
        ("c-iccid", "Which file lists an ICCID?"),
        ("c-iccid", "where is the SIM serial ICCID recorded"),
        ("c-lahore", "Which documents mention Lahore?"),
        ("c-lahore", "find Lahore in the evidence"),
    ],
}


def collection_files():
    sql = f"select original_filename from forensic.evidence_items where collection_id='{COLLECTION}'"
    out = subprocess.run(["docker", "exec", "nexusai-forensic-postgres-1", "psql", "-U", "localrecall", "-d", "localrecall", "-At", "-c", sql], capture_output=True, text=True, encoding="utf-8").stdout
    return sorted({line.strip() for line in out.splitlines() if line.strip()})


def main(argv):
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--out", required=True)
    ap.add_argument("--bank", default="bank-A", choices=sorted(BANKS))
    ap.add_argument("--only", default="", help="comma-separated 1-based question numbers")
    ap.add_argument("--timeout", type=int, default=300)
    ap.add_argument("--url", default=URL, help="the API to ask (default: the live one on :8091; the staging API runs on http://127.0.0.1:8191)")
    args = ap.parse_args(argv)
    files = collection_files()
    key = ask_case.api_key()
    questions = list(enumerate(BANKS[args.bank], 1))
    if args.only:
        wanted = {int(x) for x in args.only.split(",")}
        questions = [q for q in questions if q[0] in wanted]
    done = {json.loads(line)["n"] for line in open(args.out, encoding="utf-8")} if os.path.exists(args.out) else set()
    for n, (probe, question) in questions:
        if n in done:
            continue
        reply = ask_case.ask(args.url, key, COLLECTION, question, args.timeout)
        data = reply.get("data") or {}
        enterprise = data.get("enterprise") or {}
        header = reply.get("headers", {}).get("X-Ask-Engine-V2", "") if reply.get("http") else ""
        state = re.search(r"state=(\w+)", header)
        blob = json.dumps(data, ensure_ascii=False)
        cited = sorted(f for f in files if f in blob)
        truth = TRUTH[probe]
        hit = [f for f in cited if f in truth]
        record = {"n": n, "probe": probe, "question": question, "http": reply.get("http"), "state": state.group(1) if state else "none", "seconds": reply.get("seconds"),
                  "intent": (reply.get("headers", {}) or {}).get("X-Text-Intent", ""), "truth": truth, "cited": cited, "recall": len(hit) / len(truth), "precision": (len(hit) / len(cited)) if cited else None,
                  "answer": (enterprise.get("executive_answer") or data.get("answer") or "")[:500], "limitations": (enterprise.get("limitations") or [])[:3]}
        with open(args.out, "a", encoding="utf-8", newline="\n") as handle:
            handle.write(json.dumps(record, ensure_ascii=False) + "\n")
        print(f"{n:2} {probe:9} recall {record['recall']:.2f} precision {('%.2f' % record['precision']) if record['precision'] is not None else ' n/a'} cited {len(cited):2} state {record['state']:9} {record['seconds']} s | {question[:56]}", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
