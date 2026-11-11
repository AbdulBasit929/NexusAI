#!/usr/bin/env python3
"""Build the plate-routing bank of docs/roadmap/text-intent-preregistration-20261110.md, Amendment 3.

    python -I -X utf8 evaluation/text_intent/build_plate_bank.py [--out evaluation/text_intent/bank-plate-20261110.json]

Every plate string is generated here (seeded); no owner-labelled plate is used. Positives ask which photos/images/videos show a typed plate; negatives use the same kind of plates in questions
about camera sightings, counts, owners, timelines and "plates containing", which are answered from other tables.
"""
import argparse
import json
import random
import string

SEED = 20261110
SEED_B = 20261111
L, D = string.ascii_uppercase, string.digits

# plate shapes: name, generator
def shapes(rng):
    pick = lambda alphabet, n: "".join(rng.choice(alphabet) for _ in range(n))
    return [
        ("L3D4", lambda: f"{pick(L, 3)}-{pick(D, 4)}"),
        ("L3D3", lambda: f"{pick(L, 3)} {pick(D, 3)}"),
        ("L2D4", lambda: f"{pick(L, 2)}-{pick(D, 4)}"),
        ("L3D4_plain", lambda: f"{pick(L, 3)}{pick(D, 4)}"),
        ("D4_L3", lambda: f"{pick(D, 4)}-{pick(L, 3)}"),
        ("L2D2L2D4", lambda: f"{pick(L, 2)}{pick(D, 2)}{pick(L, 2)}{pick(D, 4)}"),
        ("L1D5", lambda: f"{pick(L, 1)} {pick(D, 5)}"),
        ("synthetic_O", lambda: f"SYN{pick(L, 1)}{rng.choice('O0')}{pick(D, 2)}"),
        ("free_mixed", lambda: f"{pick(L, 2)}{pick(D, 2)}{pick(L, 1)}{pick(D, 1)}"),
    ]


POSITIVE_FRAMES = [
    "Which photos show {p}?",
    "Is the plate {p} visible in any of the images?",
    "Find the pictures where {p} appears",
    "does {p} show up in any video",
    "Any image with the number plate {p}?",
    "Show me every photo of the vehicle with plate {p}",
    "کون سی تصاویر میں {p} نمبر پلیٹ نظر آتی ہے؟",
    "kin tasveeron mein {p} number plate nazar aati hai?",
    "Check the footage for {p}",
    "list the images containing {p}",
    "was a car with registration {p} photographed?",
    "Look through the videos for plate {p}",
]

NEGATIVE_FRAMES = [
    "When was the plate {p} last seen?",
    "How many times was the plate {p} seen by the cameras?",
    "Which cameras recorded {p}?",
    "List all ANPR sightings of {p}",
    "Who owns the vehicle {p}?",
    "Show the sighting timeline for {p}",
    "Which vehicles have a plate containing {q}?",
    "Is {p} registered to anyone in the case?",
]


# Bank B (Amendment 4): different seed, different wording, written before the instruction was changed; it is run once for the gates.
POSITIVE_FRAMES_B = [
    "Do any photos in the case contain the number plate {p}?",
    "I need every picture where {p} can be seen.",
    "Search the images for {p}.",
    "has a vehicle with plate {p} been captured in the videos?",
    "pull up the photographs showing {p}",
    "Is there footage of {p}?",
    "{p} - which of the uploaded photos show this plate?",
    "کیا کسی ویڈیو میں گاڑی {p} موجود ہے؟",
    "kisi tasveer mein gari {p} dikhai deti hai?",
    "Look in the pictures for a vehicle numbered {p}.",
    "Does the image evidence show {p} anywhere?",
    "Locate all stills and clips that show plate {p}.",
]
NEGATIVE_FRAMES_B = [
    "What time did {p} pass the checkpoint camera?",
    "Count the ANPR hits for {p}.",
    "Give me the sighting history of {p}.",
    "Which camera locations saw {p} on 12 March?",
    "Is the vehicle {p} linked to any phone number in the case?",
    "How often does {p} appear in the camera sightings table?",
    "Find vehicles whose plate starts with {q}.",
    "Where was {p} last recorded by ANPR?",
    "Does {p} show up in the vehicle records?",
    "Which owner is registered for {p}?",
]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--out", default="evaluation/text_intent/bank-plate-20261110.json")
    parser.add_argument("--set", choices=["A", "B"], default="A")
    args = parser.parse_args()
    global POSITIVE_FRAMES, NEGATIVE_FRAMES, SEED
    if args.set == "B":
        POSITIVE_FRAMES, NEGATIVE_FRAMES, SEED = POSITIVE_FRAMES_B, NEGATIVE_FRAMES_B, SEED_B
    rng = random.Random(SEED)
    gens = shapes(rng)
    positives, negatives = [], []
    for i in range(36):
        name, make = gens[i % len(gens)]
        plate = make()
        frame = POSITIVE_FRAMES[i % len(POSITIVE_FRAMES)]
        positives.append({"id": f"TI-P{args.set}-{i + 1:03d}", "question": frame.format(p=plate), "plate": plate, "shape": name, "frame": i % len(POSITIVE_FRAMES)})
    for i in range(24):
        name, make = gens[(i * 2 + 1) % len(gens)]
        plate = make()
        frame = NEGATIVE_FRAMES[i % len(NEGATIVE_FRAMES)]
        stem = "".join(ch for ch in plate if ch.isalnum())[:3]
        negatives.append({"id": f"TI-PN{args.set}-{i + 1:03d}", "question": frame.format(p=plate, q=stem), "plate": plate, "shape": name, "frame": i % len(NEGATIVE_FRAMES)})
    bank = {"built": "2026-11-10", "seed": SEED, "preregistration": "docs/roadmap/text-intent-preregistration-20261110.md Amendment 3",
            "frames": {"positive": POSITIVE_FRAMES, "negative": NEGATIVE_FRAMES}, "positives": positives, "negatives": negatives}
    with open(args.out, "w", encoding="utf-8") as handle:
        json.dump(bank, handle, ensure_ascii=False, indent=1)
    print(f"{len(positives)} positives, {len(negatives)} negatives -> {args.out}")


if __name__ == "__main__":
    main()
