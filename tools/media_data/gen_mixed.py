#!/usr/bin/env python3
"""Generate MIXED Urdu and English OCR test material with exact truth (T1: synthetic, exact truth): the case that no public benchmark covers.

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro nexusai-media-eval:latest python /repo/tools/media_data/gen_mixed.py

Five kinds, each a right-to-left Urdu layout that holds Latin words, digits and identifiers, the way Pakistani forms, statements, receipts and chat screenshots do:
  mixline   one Urdu sentence with English words, brand names and numbers inside it (the hardest reading-order case)
  form      a registration form: Urdu labels on the right, values on the left in English or Urdu (name, CNIC, phone, city, date of birth, vehicle plate)
  statement rows of a bank statement: date, English or Urdu narration, amount, balance
  receipt   an Urdu shop name, English item lines with prices, an Urdu total line
  chat      alternating message bubbles, Urdu and English and mixed, with times and a phone number
Layout: the logical string of a line is cut into runs by script (Arabic-script letters, Latin letters, digits); Urdu runs are shaped by Pillow's raqm layout with Noto Naskh Arabic or Noto
Nastaliq Urdu, Latin runs with Noto Sans; the runs are placed from the right edge leftwards in logical order (the rule of a right-to-left paragraph), so the image shows what a reader sees and the
truth is the logical text a reader should return. Identifiers (cnic, phone, amount, date, plate, account) are recorded with their type so that exactness is scored apart from character error.
Variants: clean, noisy (grain + blur), skewed (1 degree), lowres_jpeg (100 dpi, quality 40). Seeded; dev or sealed by the SHA-256 of the id (last hex digit 0 to b dev). All sentences come from the
text column of the Urdu sets already on disk (used as text only), numbers and names are invented.
"""
import hashlib
import io
import json
import os
import random
import re
from pathlib import Path

import pyarrow.parquet as pq
from PIL import Image, ImageDraw, ImageFilter, ImageFont

DATA = os.environ.get("NEXUSAI_MEDIA_DATA", "/data")
OUT = Path(DATA) / "generated" / ("ocr_mixed" + os.environ.get("OCR_GEN_SUFFIX", ""))
FONT_DIR = Path(DATA) / "fonts"
SEED = int(os.environ.get("OCR_GEN_SEED", "20261117"))  # a different seed makes a fresh set (acceptance runs); the default reproduces the registered set
GAP = 12          # pixels between two runs of a line: the swash of an Urdu letter reaches past its advance and touched the next Latin word
PER_KIND = {"mixline": 40, "form": 24, "statement": 24, "receipt": 24, "chat": 24}
ARABIC = re.compile(r"[\u0600-\u06ff\u0750-\u077f\ufb50-\ufdff\ufe70-\ufeff]")

EN_NAMES = ["Ahmed Raza", "Bilal Khan", "Sana Malik", "Hamza Ghauri", "Ayesha Siddiqui", "Usman Tariq", "Fatima Noor", "Imran Qureshi", "Zainab Ali", "Kamran Sheikh"]
UR_NAMES = ["احمد رضا", "بلال خان", "ثنا ملک", "حمزہ غوری", "عائشہ صدیقی", "عثمان طارق", "فاطمہ نور", "عمران قریشی", "زینب علی", "کامران شیخ"]
CITIES_EN = ["Lahore", "Karachi", "Islamabad", "Peshawar", "Quetta", "Multan", "Faisalabad", "Rawalpindi"]
CITIES_UR = ["لاہور", "کراچی", "اسلام آباد", "پشاور", "کوئٹہ", "ملتان", "فیصل آباد", "راولپنڈی"]
ITEMS = ["Milk 1L", "Bread", "Rice 5kg", "Sugar 2kg", "Tea Lipton", "Cooking Oil", "Eggs 12", "Soap Lux", "Biscuits", "Mobile Load"]
BRANDS = ["WhatsApp", "Facebook", "Google", "Daraz", "Easypaisa", "JazzCash", "Careem", "Foodpanda", "Samsung", "iPhone", "Windows", "Gmail"]
SHOPS = ["الفتح سٹور", "کراچی بیکرز", "المدینہ مارٹ", "نیو لاہور کریانہ", "بسم اللہ جنرل سٹور"]
NARRATION_EN = ["ATM Withdrawal", "Salary Credit", "Online Transfer", "Utility Bill", "POS Purchase", "Cash Deposit", "Raast Transfer"]
NARRATION_UR = ["رقم کی منتقلی", "تنخواہ جمع", "بجلی کا بل", "نقد وصولی", "خریداری"]
CHAT_UR = ["آپ کہاں ہیں؟", "میں دفتر پہنچ گیا ہوں", "کل ملاقات ہو سکتی ہے؟", "ٹھیک ہے، شکریہ", "کیا آپ نے رقم بھیج دی؟", "ابھی بھیجتا ہوں"]
CHAT_EN = ["ok done", "call me when free", "sent the money", "see you tomorrow", "where are you", "running late"]


def partition(key):
    return "dev" if hashlib.sha256(key.encode()).hexdigest()[-1] in "0123456789ab" else "sealed"


def sentences():
    out = []
    for style in ("naskh", "nastaliq"):
        table = pq.read_table(Path(DATA) / "urdu_ocr" / f"{style}-val.parquet", columns=["text"])
        for text in table.column("text").to_pylist():
            t = re.sub(r"\s+", " ", text).strip()
            if 4 <= len(t.split()) <= 9 and len(t) <= 70 and not re.search(r"[A-Za-z0-9]", t):
                out.append(t)
    return out


def cnic(rng):
    return "%05d-%07d-%d" % (rng.randint(10000, 99999), rng.randint(1000000, 9999999), rng.randint(1, 9))


def phone(rng):
    return "03%02d-%07d" % (rng.randint(0, 49), rng.randint(1000000, 9999999))


def plate(rng):
    return "%s-%02d-%d" % (rng.choice(["LEA", "LEB", "ABC", "KHD", "RIP", "ICT"]), rng.randint(10, 24), rng.randint(1000, 9999))


def amount(rng):
    return "{:,}".format(rng.randint(150, 250000))


def date(rng):
    return "%02d/%02d/%d" % (rng.randint(1, 28), rng.randint(1, 12), rng.randint(2024, 2026))


def runs(text):
    """Cut a logical string into (script, run) pieces: 'ar' for Arabic-script letters (and the spaces between them), 'la' for Latin letters, digits and their joiners."""
    pieces, current, kind = [], "", None
    for ch in text:
        k = "ar" if ARABIC.match(ch) else ("la" if re.match(r"[A-Za-z0-9]", ch) or (ch in "-/,.:+@#()&%'" and kind == "la") else None)
        if k is None:
            k = kind or "ar"
        if k != kind and current:
            pieces.append((kind, current))
            current = ""
        kind = k
        current += ch
    if current:
        pieces.append((kind, current))
    return pieces


class Canvas:
    def __init__(self, width, height, style):
        self.image = Image.new("L", (width, height), 255)
        self.draw = ImageDraw.Draw(self.image)
        self.width, self.height = width, height
        self.urdu_font = FONT_DIR / ("NotoNastaliqUrdu.ttf" if style == "nastaliq" else "NotoNaskhArabic.ttf")
        self.latin_font = FONT_DIR / "NotoSans.ttf"
        self.fonts = {}

    def font(self, kind, size):
        key = (kind, size)
        if key not in self.fonts:
            self.fonts[key] = ImageFont.truetype(str(self.urdu_font if kind == "ar" else self.latin_font), size, layout_engine=ImageFont.Layout.RAQM)
        return self.fonts[key]

    def measure(self, text, size):
        return sum(self.font(k, size).getlength(r, direction="rtl" if k == "ar" else "ltr") + GAP for k, r in runs(text))

    def rtl_line(self, text, right, y, size):
        """Place the logical text as a right-to-left line whose right edge is x=right; returns the left edge."""
        x = right
        for kind, run in runs(text):
            font = self.font(kind, size)
            w = font.getlength(run, direction="rtl" if kind == "ar" else "ltr")
            x -= w
            self.draw.text((x, y), run, font=font, fill=0, direction="rtl" if kind == "ar" else "ltr")
            x -= GAP
        return x

    def ltr_line(self, text, left, y, size):
        x = left
        for kind, run in runs(text):
            font = self.font(kind, size)
            self.draw.text((x, y), run, font=font, fill=0, direction="rtl" if kind == "ar" else "ltr")
            x += font.getlength(run, direction="rtl" if kind == "ar" else "ltr") + GAP
        return x


def degrade(image, variant, rng):
    if variant == "noisy":
        image = image.filter(ImageFilter.GaussianBlur(0.8))
        px = image.load()
        for _ in range(image.width * image.height // 12):
            x, y = rng.randrange(image.width), rng.randrange(image.height)
            px[x, y] = max(0, min(255, px[x, y] + rng.randint(-45, 45)))
    elif variant == "skewed":
        image = image.rotate(1.0, resample=Image.BICUBIC, expand=True, fillcolor=255)
    elif variant == "lowres_jpeg":
        small = image.resize((int(image.width * 100 / 150), int(image.height * 100 / 150)), Image.LANCZOS)
        buf = io.BytesIO()
        small.save(buf, "JPEG", quality=40)
        image = Image.open(io.BytesIO(buf.getvalue())).convert("L")
    return image


def build_item(kind, n, rng, texts):
    style = rng.choice(["naskh", "nastaliq"])
    idents = []
    lines = []          # (logical text, align) align = 'r' right-to-left line, or ('cells', [(text, side)...])
    if kind == "mixline":
        parts = []
        for _ in range(rng.randint(1, 2)):
            t = rng.choice(texts).split()
            t.insert(rng.randint(1, len(t) - 1), rng.choice(BRANDS))
            parts.append(" ".join(t))
        sentence = " ".join(parts)
        if rng.random() < 0.7:
            ph = phone(rng)
            sentence += " " + ph
            idents.append({"type": "phone", "value": ph})
        lines.append(("r", sentence))
        canvas = Canvas(1500, 130, style)
        # Fixed 2026-11-19: the canvas used to stay 1500 px wide whatever the sentence, so a long sentence ran off the LEFT edge and took its phone number (the last run, drawn leftmost) with it:
        # 56 of 160 mixline images of the registered set were cut (docs/data, erratum of 2026-11-19). The canvas now grows to fit; OCR_GEN_LEGACY_CLIP=1 reproduces the registered (cut) images.
        if os.environ.get("OCR_GEN_LEGACY_CLIP", "") != "1":
            needed = canvas.measure(sentence, 40) + 80
            if needed > 1500:
                canvas = Canvas(int(needed) + 1, 130, style)
    else:
        canvas = None
    if kind == "form":
        i = rng.randrange(len(EN_NAMES))
        c, ph, pl, dob, ci = cnic(rng), phone(rng), plate(rng), date(rng), rng.randrange(len(CITIES_EN))
        rows = [("نام", EN_NAMES[i], None), ("اردو نام", UR_NAMES[i], None), ("شناختی کارڈ نمبر", c, "cnic"), ("موبائل نمبر", ph, "phone"), ("شہر", CITIES_EN[ci] if rng.random() < 0.5 else CITIES_UR[ci], None), ("تاریخ پیدائش", dob, "date"), ("گاڑی نمبر", pl, "plate")]
        for label, value, typ in rows:
            lines.append(("cells", label, value))
            if typ:
                idents.append({"type": typ, "value": value})
        canvas = Canvas(1240, 90 + 78 * len(rows), style)
    elif kind == "statement":
        rows = []
        for _ in range(rng.randint(6, 9)):
            d, am, bal = date(rng), amount(rng), amount(rng)
            narration = rng.choice(NARRATION_EN) if rng.random() < 0.6 else rng.choice(NARRATION_UR)
            rows.append((d, narration, am, bal))
            idents.extend([{"type": "date", "value": d}, {"type": "amount", "value": am}, {"type": "amount", "value": bal}])
        for r in rows:
            lines.append(("statement", r))
        canvas = Canvas(1400, 80 + 64 * len(rows), style)
    elif kind == "receipt":
        shop = rng.choice(SHOPS)
        lines.append(("r", shop))
        total = 0
        for _ in range(rng.randint(4, 7)):
            item, price = rng.choice(ITEMS), rng.randint(40, 2400)
            total += price
            lines.append(("receiptrow", item, "{:,}".format(price)))
            idents.append({"type": "amount", "value": "{:,}".format(price)})
        tot = "{:,}".format(total)
        lines.append(("r", "کل رقم %s روپے" % tot))
        idents.append({"type": "amount", "value": tot})
        ph = phone(rng)
        lines.append(("r", "رابطہ نمبر %s" % ph))
        idents.append({"type": "phone", "value": ph})
        canvas = Canvas(900, 110 + 62 * len(lines), style)
    elif kind == "chat":
        count = rng.randint(5, 8)
        for k in range(count):
            urdu = rng.random() < 0.55
            msg = rng.choice(CHAT_UR) if urdu else rng.choice(CHAT_EN)
            if rng.random() < 0.3:
                ph = phone(rng)
                msg += " " + ph
                idents.append({"type": "phone", "value": ph})
            lines.append(("bubble", msg, "%02d:%02d" % (rng.randint(8, 23), rng.randint(0, 59)), urdu))
        canvas = Canvas(1100, 80 + 78 * count, style)
    truth = []
    size = 40 if kind != "chat" else 36
    y = 30
    for line in lines:
        if line[0] == "r":
            canvas.rtl_line(line[1], canvas.width - 40, y, size)
            truth.append(line[1])
            y += 64 if kind != "mixline" else 0
        elif line[0] == "cells":
            _, label, value = line
            canvas.rtl_line(label, canvas.width - 40, y, size)
            if ARABIC.search(value):
                canvas.rtl_line(value, canvas.width // 2 - 20, y, size)
            else:
                canvas.ltr_line(value, 60, y, size)
            truth.append(label + " " + value)
            y += 78
        elif line[0] == "statement":
            d, narration, am, bal = line[1]
            canvas.ltr_line(d, 40, y, 34)
            if ARABIC.search(narration):
                canvas.rtl_line(narration, 760, y, 34)
            else:
                canvas.ltr_line(narration, 330, y, 34)
            canvas.ltr_line(am, 820, y, 34)
            canvas.ltr_line(bal, 1100, y, 34)
            truth.append("%s %s %s %s" % (d, narration, am, bal))
            y += 64
        elif line[0] == "receiptrow":
            _, item, price = line
            canvas.ltr_line(item, 50, y, 34)
            canvas.ltr_line(price, 640, y, 34)
            truth.append("%s %s" % (item, price))
            y += 62
        elif line[0] == "bubble":
            _, msg, hhmm, urdu = line
            w = int(canvas.measure(msg, size)) + 40
            if urdu:
                canvas.draw.rounded_rectangle([canvas.width - 40 - w, y - 8, canvas.width - 40, y + 56], radius=18, outline=0, width=2)
                canvas.rtl_line(msg, canvas.width - 60, y, size)
            else:
                canvas.draw.rounded_rectangle([40, y - 8, 40 + w, y + 56], radius=18, outline=0, width=2)
                canvas.ltr_line(msg, 60, y, size)
            canvas.ltr_line(hhmm, (canvas.width - 40 - w - 90) if urdu else (40 + w + 14), y + 14, 22)      # the time sits outside the bubble: inside it overlapped a long phone number
            truth.append(msg + " " + hhmm)
            y += 78
    return canvas, truth, idents, style


def main():
    rng = random.Random(SEED)
    texts = sentences()
    OUT.mkdir(parents=True, exist_ok=True)
    rows = []
    for kind, count in PER_KIND.items():
        for n in range(count):
            canvas, truth, idents, style = build_item(kind, n, rng, texts)
            for variant in ("clean", "noisy", "skewed", "lowres_jpeg"):
                ident = "mixed-%s-%03d-%s" % (kind, n, variant)
                img = degrade(canvas.image, variant, random.Random(f"{SEED}-{ident}"))
                ext = "jpg" if variant == "lowres_jpeg" else "png"
                path = OUT / f"{ident}.{ext}"
                img.save(path, quality=40) if ext == "jpg" else img.save(path)
                rows.append({"id": ident, "file": path.name, "text": "\n".join(truth), "kind": kind, "style": style, "variant": variant, "partition": partition(f"mixed-{kind}-{n:03d}"), "identifiers": idents, "tier": "T1"})
    with open(OUT / "truth.jsonl", "w", encoding="utf-8", newline="\n") as handle:
        for r in rows:
            handle.write(json.dumps(r, ensure_ascii=False) + "\n")
    print(len(rows), "images;", sum(1 for r in rows if r["partition"] == "dev"), "dev,", sum(1 for r in rows if r["partition"] == "sealed"), "sealed ->", OUT)


if __name__ == "__main__":
    main()
