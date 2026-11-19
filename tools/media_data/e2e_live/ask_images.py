import json, re, sys
sys.path.insert(0, "C:/Users/sheik/Workspace/Office/Projects/NexusAI-localwork/tools/showcase")
import ask_case

COLLECTION = "ocr-easyocr-e2e-test-20261119"
URL = "http://127.0.0.1:8091"
QUESTIONS = [
    ("name in a mixed form (Latin letters)", "Which images mention Ahmed Raza?", "mixed-form-001-clean.png"),
    ("phone number on an Urdu page", "Which images contain the phone number 0308-5882033?", "urpage-naskh-004-clean.png"),
    ("CNIC on a Nastaliq page (never routed to text search by design)", "Which images contain the CNIC 24017-2163227-3?", "urpage-nastaliq-003-clean.png"),
    ("amount with the Rs. prefix (the re-read fix)", "Which images mention Rs. 144,665?", "urpage-naskh-004-clean.png"),
    ("vehicle number in a mixed form", "Which images show the vehicle number LEB-19-2672?", "mixed-form-001-clean.png"),
    ("phone number on a short mixed line", "Which images contain the phone number 0313-6783098?", "mixed-mixline-000-clean.png"),
    ("phone number in a mixed chat", "Which images contain the number 0344-5990591?", "mixed-chat-002-clean.png"),
    ("Urdu word on a mixed receipt", "Which images contain the word \u06a9\u0631\u06cc\u0627\u0646\u06c1?", "mixed-receipt-000-clean.png"),
    ("control: IMEI on an English page", "Which images mention IMEI 214711939616631?", "en-20261021-000-clean.png"),
]
key = ask_case.api_key()
results = []
for label, question, expected in QUESTIONS:
    reply = ask_case.ask(URL, key, COLLECTION, question, 240)
    data = reply.get("data") or {}
    enterprise = data.get("enterprise") or {}
    header = reply.get("headers", {}).get("X-Ask-Engine-V2", "") if reply.get("http") else ""
    state = re.search(r"state=(\w+)", header)
    answer = enterprise.get("executive_answer") or data.get("answer") or ""
    blob = json.dumps(data, ensure_ascii=False)
    found = expected in blob
    results.append({"label": label, "question": question, "expected": expected, "http": reply.get("http"), "state": state.group(1) if state else "none", "found_expected_file": found,
                    "seconds": reply.get("seconds"), "answer": answer[:400]})
    print(f"{'OK  ' if found else 'MISS'} {label[:52]:52} {reply.get('seconds')} s | {answer[:150]!r}", flush=True)
json.dump(results, open("C:/tmp/nexusai-g4/e2e_ask2_results.json", "w", encoding="utf-8"), ensure_ascii=False, indent=1)
print(f"\nfound the expected file in {sum(r['found_expected_file'] for r in results)} of {len(results)} answers")
