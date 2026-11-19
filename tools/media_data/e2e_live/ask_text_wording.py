import json, re, sys
sys.path.insert(0, "C:/Users/sheik/Workspace/Office/Projects/NexusAI-localwork/tools/showcase")
import ask_case

COLLECTION = "ocr-easyocr-e2e-test-20261119"
QUESTIONS = [
    ("vehicle number written on a form, worded as text", "Which images contain the text LEB-19-2672?", "mixed-form-001-clean.png"),
    ("phone number in a chat, worded as text", "Which images contain the text 0344-5990591?", "mixed-chat-002-clean.png"),
    ("IMEI on an English page, worded as text", "Which images contain the text 214711939616631?", "en-20261021-000-clean.png"),
]
key = ask_case.api_key()
for label, question, expected in QUESTIONS:
    reply = ask_case.ask("http://127.0.0.1:8091", key, COLLECTION, question, 240)
    data = reply.get("data") or {}
    enterprise = data.get("enterprise") or {}
    answer = enterprise.get("executive_answer") or data.get("answer") or ""
    found = expected in json.dumps(data, ensure_ascii=False)
    print(f"{'OK  ' if found else 'MISS'} {label[:55]:55} {reply.get('seconds')} s | {answer[:170]!r}", flush=True)
