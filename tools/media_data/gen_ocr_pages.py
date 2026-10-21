#!/usr/bin/env python3
"""Generate English document pages with planted identifiers and exact truth, for the OCR track (T1: synthetic, exact truth).

Runs inside the evaluation image with the data folder mounted read-write:

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest python /repo/tools/media_data/gen_ocr_pages.py

Each page is A4 (150 dpi) of templated forensic-style prose plus a small table, written with an OFL font. Every identifier on the page is recorded with its type, so the evaluation can score
identifier exact match separately from character error. Variants per page: clean, noisy (grain + blur), skewed (1 degree), low-resolution JPEG (100 dpi, quality 40). Seeded: the same seed
gives the same pages byte for byte. A page belongs to dev or sealed by the last hex digit of the SHA-256 of its id (0 to b dev, c to f sealed).
"""
import hashlib
import io
import json
import os
import random
import sys

from PIL import Image, ImageDraw, ImageFilter, ImageFont

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
OUT = os.path.join(DATA, "generated", "ocr_en")
FONT = os.path.join(DATA, "fonts", "NotoSans.ttf")
SEED = 20261021
PAGES = 48

BANKS = ["HBL", "UBL", "MCB", "ALFH", "SCBL", "MEZN", "ABPA", "BAHL"]
CITIES = ["Lahore", "Karachi", "Islamabad", "Peshawar", "Quetta", "Multan", "Faisalabad", "Rawalpindi"]
PLATE_LETTERS = ["LEA", "LEB", "LRV", "ABC", "KHD", "BLX", "RIP", "ICT", "PSH", "AGY"]
VERBS = ["recorded", "confirmed", "observed", "reported", "logged", "verified"]
SENTENCES = [
    "The subscriber record shows that {phone} was activated in {city} on {date} and linked to national identity number {cnic}.",
    "A transfer of {amount} was {verb} from account {iban} on {date} at {time}.",
    "The vehicle bearing registration {plate} was {verb} near the {city} checkpoint on {date} at {time}.",
    "Handset with IMEI {imei} was {verb} on the network using number {phone}.",
    "FIR {fir} was filed at the {city} station on {date}; the complainant gave the contact number {phone}.",
    "The statement refers to a second number {phone} and to payment reference {ref} of {amount}.",
    "Records for identity number {cnic} list the address in {city} and the registration {plate}.",
    "The device with IMEI {imei} was last seen on {date} at {time}, and the call to {phone} lasted {minutes} minutes.",
]
KINDS = ("phone", "cnic", "iban", "plate", "imei", "date", "time", "amount", "fir", "ref")


def luhn_check_digit(body):
    total = 0
    for index, char in enumerate(reversed(body)):
        digit = int(char)
        if index % 2 == 0:
            digit *= 2
            if digit > 9:
                digit -= 9
        total += digit
    return str((10 - total % 10) % 10)


def iban_pk(rng):
    bank = rng.choice(BANKS).ljust(4, "A")[:4]
    account = "".join(str(rng.randrange(10)) for _ in range(16))
    rearranged = bank + account + "PK00"
    numeric = "".join(str(int(c, 36)) for c in rearranged)
    check = 98 - int(numeric) % 97
    return f"PK{check:02d}{bank}{account}"


def make_values(rng):
    return {
        "phone": rng.choice([f"+92 3{rng.randrange(0, 5)}{rng.randrange(10)} {rng.randrange(10 ** 7):07d}", f"03{rng.randrange(0, 5)}{rng.randrange(10)}-{rng.randrange(10 ** 7):07d}"]),
        "cnic": f"{rng.randrange(1, 8)}{rng.randrange(10 ** 4):04d}-{rng.randrange(10 ** 7):07d}-{rng.randrange(1, 10)}",
        "iban": iban_pk(rng),
        "plate": rng.choice([f"{rng.choice(PLATE_LETTERS)}-{rng.randrange(10, 99)}-{rng.randrange(1000, 9999)}", f"{rng.choice(PLATE_LETTERS)} {rng.randrange(100, 999)}"]),
        "imei": (lambda body: body + luhn_check_digit(body))("".join(str(rng.randrange(10)) for _ in range(14))),
        "date": f"{rng.randrange(1, 29):02d}/{rng.randrange(1, 13):02d}/{rng.randrange(2021, 2027)}",
        "time": f"{rng.randrange(0, 24):02d}:{rng.randrange(0, 60):02d}",
        "amount": f"Rs. {rng.randrange(1, 900)},{rng.randrange(1000):03d}.{rng.randrange(100):02d}",
        "fir": f"{rng.randrange(1, 999)}/{rng.randrange(2021, 2027)}",
        "ref": f"TXN{rng.randrange(10 ** 9):09d}",
        "city": rng.choice(CITIES),
        "verb": rng.choice(VERBS),
        "minutes": str(rng.randrange(1, 90)),
    }


def wrap(draw, text, font, width):
    lines, current = [], ""
    for word in text.split(" "):
        trial = (current + " " + word).strip()
        if draw.textlength(trial, font=font) <= width:
            current = trial
        else:
            lines.append(current)
            current = word
    if current:
        lines.append(current)
    return lines


def make_page(rng, n):
    width, height = 1240, 1754  # A4 at 150 dpi
    image = Image.new("L", (width, height), 255)
    draw = ImageDraw.Draw(image)
    title_font = ImageFont.truetype(FONT, 40)
    body_font = ImageFont.truetype(FONT, 25 if n % 2 == 0 else 28)
    small_font = ImageFont.truetype(FONT, 23)
    identifiers, lines_out = [], []
    margin, y = 110, 110
    title = rng.choice(["Case Summary Note", "Subscriber Verification Record", "Transaction and Movement Log", "Evidence Register Extract"])
    draw.text((margin, y), title, font=title_font, fill=0)
    lines_out.append(title)
    y += 90
    for _ in range(rng.randrange(4, 6)):
        values = make_values(rng)
        template = rng.choice(SENTENCES)
        sentence = template.format(**values)
        for kind in KINDS:
            if "{" + kind + "}" in template:
                identifiers.append({"type": kind, "value": values[kind]})
        for line in wrap(draw, sentence, body_font, width - 2 * margin):
            draw.text((margin, y), line, font=body_font, fill=0)
            lines_out.append(line)
            y += 44
        y += 22
    # a small table: date, reference, amount
    headers = ["Date", "Reference", "Amount"]
    columns = [margin, margin + 280, margin + 720]
    for x, label in zip(columns, headers):
        draw.text((x, y), label, font=small_font, fill=0)
    lines_out.append(" ".join(headers))
    draw.line((margin, y + 34, width - margin, y + 34), fill=0, width=2)
    y += 48
    for _ in range(6):
        values = make_values(rng)
        row = [values["date"], values["ref"], values["amount"]]
        for x, cell in zip(columns, row):
            draw.text((x, y), cell, font=small_font, fill=0)
        for kind in ("date", "ref", "amount"):
            identifiers.append({"type": kind, "value": values[kind]})
        lines_out.append(" ".join(row))
        y += 38
    return image, lines_out, identifiers


def variants(image, rng):
    out = {"clean": image}
    noisy = image.filter(ImageFilter.GaussianBlur(0.9))
    pixels = noisy.load()
    for _ in range(int(noisy.width * noisy.height * 0.03)):
        x, y = rng.randrange(noisy.width), rng.randrange(noisy.height)
        pixels[x, y] = max(0, min(255, pixels[x, y] + rng.randrange(-70, 70)))
    out["noisy"] = noisy
    out["skewed"] = image.rotate(1.0, resample=Image.BICUBIC, fillcolor=255)
    small = image.resize((int(image.width * 100 / 150), int(image.height * 100 / 150)), Image.LANCZOS)
    buffer = io.BytesIO()
    small.save(buffer, format="JPEG", quality=40)
    out["lowres_jpeg"] = Image.open(io.BytesIO(buffer.getvalue())).convert("L")
    return out


def partition(page_id):
    return "dev" if hashlib.sha256(page_id.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def main():
    if not os.path.exists(FONT):
        sys.exit(f"missing {FONT}: run tools/media_data/fetch.py ocr first")
    os.makedirs(OUT, exist_ok=True)
    rng = random.Random(SEED)
    truth = []
    for n in range(PAGES):
        page_id = f"en-{SEED}-{n:03d}"
        page, lines, identifiers = make_page(rng, n)
        for name, image in variants(page, rng).items():
            ext = "jpg" if name == "lowres_jpeg" else "png"
            rel = f"{page_id}-{name}.{ext}"
            if ext == "jpg":
                image.save(os.path.join(OUT, rel), format="JPEG", quality=40)
            else:
                image.save(os.path.join(OUT, rel))
            truth.append({"id": f"{page_id}-{name}", "page": page_id, "variant": name, "file": rel, "partition": partition(page_id),
                          "text": "\n".join(lines), "identifiers": identifiers, "tier": "T1"})
    with open(os.path.join(OUT, "truth.jsonl"), "w", encoding="utf-8") as handle:
        for item in truth:
            handle.write(json.dumps(item, ensure_ascii=False) + "\n")
    dev = sum(1 for t in truth if t["partition"] == "dev")
    print(f"{PAGES} pages x 4 variants = {len(truth)} images, dev {dev}, sealed {len(truth) - dev}, identifiers {sum(len(t['identifiers']) for t in truth)}")


if __name__ == "__main__":
    main()
