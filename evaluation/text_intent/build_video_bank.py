#!/usr/bin/env python3
"""Development bank for Amendment 7 of docs/roadmap/text-intent-preregistration-20261110.md: questions ABOUT videos and their readings (not searches) and typed-plate searches that also name a file.

    python -I -X utf8 evaluation/text_intent/build_video_bank.py QUESTIONS_KEY.json OUT.json

The negatives are every question of a video question bank (tools/video_plates/video_question_bank.py: only the wording is read, never the answer key) plus hand-written variants that ask the same kinds of
thing in other words. None may be routed as a search for text or for a plate. The positives are typed-plate searches with SYNTHETIC plates, some of them naming a video or a photo by its file name:
the file name must not stop a typed plate from being found. DEVELOPMENT: the bank is used to change and to rerun; it is not a gate.
"""
import json
import random
import sys

EXTRA_NEGATIVES = [
    "How many seconds of footage does gate-cam.mp4 hold?",
    "On which frames of gate-cam.mp4 does a plate appear?",
    "How many frames in night-1.mp4 contain a number plate?",
    "What plates were read in clip-7.mp4 and at which second?",
    "List every plate read in night-1.mp4 with the time it was first read.",
    "Which of the clips has the most plates in it?",
    "Is there any plate that appears in both gate-cam.mp4 and night-1.mp4?",
    "What is the longest video we have?",
    "How many videos are there with sound?",
    "At which second of clip-7.mp4 is the clearest plate reading?",
    "Show me the readings from the second half of gate-cam.mp4.",
    "How many different vehicles' plates were picked up in the last video?",
    "What was said in the first ten seconds of interview-3.mp4?",
    "How many minutes of recordings are in the case?",
    "Which video has the most readable writing on screen?",
    "How many plates does highway-4k.mp4 show and for how long is each in view?",
    "Give the length of every video in the case.",
    "Was any plate read with low confidence in clip-7.mp4?",
    "Between which seconds is the first plate visible in night-1.mp4?",
    "How many readings were taken from night-1.mp4 altogether?",
]


def main(argv):
    key = json.load(open(argv[0], encoding="utf-8"))
    negatives = [q["q"] for q in key["questions"]] + EXTRA_NEGATIVES
    rng = random.Random(20261112)
    shapes = ["LEA {d4}", "{l3}-{d3}", "{l2}{d2}-{d4}", "{l3}{d4}", "{l2} {d3}"]
    frames = ["Which photos show {p}?", "Show me the video footage with the plate {p}.", "Is {p} visible in any of the videos?", "Which videos contain the vehicle {p}?",
              "Find the plate {p} in the recordings {f}.", "Does the plate {p} appear in {f}?", "Which pictures show {p}, apart from {f}?", "Look through the footage for {p}; start with {f}."]
    files = ["gate-cam.mp4", "night-1.mp4", "clip-7.mp4", "IMG_2031.jpg", "highway-4k.mp4", "front-gate.jpg"]
    positives = []
    for i in range(24):
        l = lambda n: "".join(rng.choice("ABCDEFGHJKLMNPRSTUVWXYZ") for _ in range(n))  # noqa: E731
        d = lambda n: "".join(rng.choice("0123456789") for _ in range(n))  # noqa: E731
        plate = rng.choice(shapes).format(l2=l(2), l3=l(3), d2=d(2), d3=d(3), d4=d(4))
        question = rng.choice(frames).format(p=plate, f=rng.choice(files))
        positives.append({"id": "TI-V-P-%03d" % (i + 1), "question": question, "plate": plate})
    bank = {"built": "2026-11-12", "seed": 20261112, "preregistration": "Amendment 7 (development bank, not a gate)",
            "positives": positives, "negatives": [{"id": "TI-V-N-%03d" % (i + 1), "question": q} for i, q in enumerate(negatives)]}
    json.dump(bank, open(argv[1], "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    print("%d positives, %d negatives" % (len(positives), len(negatives)))


if __name__ == "__main__":
    main(sys.argv[1:])
