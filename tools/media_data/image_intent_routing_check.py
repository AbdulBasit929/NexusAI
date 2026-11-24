#!/usr/bin/env python3
"""Checks the image-intent model step on its own: 29 sentences (descriptions of photographs in English, Roman Urdu and Urdu; plates, written words, records, people, counts; copies of a named file), each
sent to the language model with the REAL system prompt and schema of api/forensic_records/ask_image_search.go. Prints what the model decided against what was expected. No case data is touched.

    python tools/media_data/image_intent_routing_check.py [--model qwen3-4b-instruct-2507-q4km-nxb21d-dev] [--url http://127.0.0.1:8080]

A sentence is a pass when the model's a_kind equals the expected kind. Expected kinds: photos (photos_matching_description), other, copies (copies_of_named_image), similar (similar_to_named_image).
"""
import argparse
import json
import re
import sys
import time
import urllib.request
from pathlib import Path

GO = Path(__file__).resolve().parents[2] / "api" / "forensic_records" / "ask_image_search.go"
SECRETS = r"C:\Users\sheik\Workspace\Office\Projects\NexusAI\.env.forensic-runtime.local"
KIND = {"photos": "photos_matching_description", "other": "other", "copies": "copies_of_named_image", "similar": "similar_to_named_image"}

SENTENCES = [
    ("show me photos of an airplane", "photos"),
    ("find a baby watching something on a laptop", "photos"),
    ("any picture with a plane in it", "photos"),
    ("pictures of people eating at a market", "photos"),
    ("photos where someone is carrying a backpack", "photos"),
    ("a red bus on the street", "photos"),
    ("images of a dog", "photos"),
    ("show me images with a motorcycle and a car", "photos"),
    ("رات کو سڑک پر موٹر سائیکل کی تصاویر", "photos"),
    ("mujhe laptop wali tasveeren dikhao", "photos"),
    ("which photos show a truck", "photos"),
    ("is there a picture of a clock tower", "photos"),
    ("pics with food on a table", "photos"),
    ("photos of kids playing outside", "photos"),
    ("which photos show the plate SYNT9150", "other"),
    ("find the number plate LEC5112 in the images", "other"),
    ("which images contain the word PARKING", "other"),
    ("what is written on the sign in the photo", "other"),
    ("who called 0300 0001006 the most", "other"),
    ("how many call records are there", "other"),
    ("summarise the audio recording", "other"),
    ("who is the person in this photo", "other"),
    ("show faces in the pictures", "other"),
    ("how many images are in this case", "other"),
    ("which photo files were uploaded yesterday", "other"),
    ("find documents mentioning the invoice", "other"),
    ("read the text in imgsearch-sign-127865.jpg", "other"),
    ("find copies of imgsearch-truck-558137.jpg", "copies"),
    ("pictures that look like imgsearch-bus-124952.jpg", "similar"),
]


def system_prompt():
    body = GO.read_text(encoding="utf-8").split("func imageIntentSystemPrompt() string {")[1].split("\n}\n")[0]
    return "".join(json.loads('"' + p + '"') for p in re.findall(r'"((?:[^"\\]|\\.)*)"', body))


def main():
    sys.stdout.reconfigure(encoding="utf-8")  # the Urdu sentence cannot be printed through the Windows console code page
    parser = argparse.ArgumentParser()
    parser.add_argument("--model", default="qwen3-4b-instruct-2507-q4km-nxb21d-dev")
    parser.add_argument("--url", default="http://127.0.0.1:8080")
    args = parser.parse_args()
    found = [l.split("=", 1)[1].strip() for l in open(SECRETS, encoding="utf-8") if l.startswith("LOCALAI_API_KEY=")]
    key = found[0] if found else ""
    system = system_prompt()
    schema = {"type": "json_schema", "json_schema": {"name": "forensic_image_intent", "strict": True, "schema": {
        "type": "object", "additionalProperties": False, "required": ["a_kind", "description_en", "image_name"],
        "properties": {"a_kind": {"type": "string", "enum": list(KIND.values())}, "description_en": {"type": "string"}, "image_name": {"type": "string"}}}}}
    passed = 0
    for sentence, expected in SENTENCES:
        payload = {"model": args.model, "temperature": 0, "max_tokens": 120, "response_format": schema, "messages": [{"role": "system", "content": system}, {"role": "user", "content": sentence}]}
        started = time.time()
        request = urllib.request.Request(args.url + "/v1/chat/completions", data=json.dumps(payload).encode(), headers={"Content-Type": "application/json", "Authorization": "Bearer " + key})
        with urllib.request.urlopen(request, timeout=300) as response:
            wire = json.loads(json.loads(response.read().decode())["choices"][0]["message"]["content"])
        ok = wire["a_kind"] == KIND[expected]
        passed += ok
        print(f"{'PASS' if ok else 'FAIL'}  {time.time() - started:4.1f}s  expected={expected:<7} got={wire['a_kind'].split('_')[0]:<7} desc={wire['description_en']!r:<48} {sentence}", flush=True)
    print(f"{passed} of {len(SENTENCES)} as expected")
    return 0 if passed == len(SENTENCES) else 1


if __name__ == "__main__":
    sys.exit(main())
