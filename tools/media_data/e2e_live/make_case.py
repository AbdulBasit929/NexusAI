import json, os, shutil
from pathlib import Path

data = Path("C:/tmp/nexusai-media-data")
case = Path("C:/tmp/nexusai-g4/ocr-e2e-case")
files = case / "files"
if case.exists():
    shutil.rmtree(case)
files.mkdir(parents=True)
chosen = []


def rows(folder):
    return [json.loads(line) for line in open(data / "generated" / folder / "truth.jsonl", encoding="utf-8")]


ur = [r for r in rows("ocr_ur") if r["partition"] == "dev" and r["kind"] == "page" and r["variant"] == "clean"]
for style in ("naskh", "nastaliq"):
    chosen.append(("ocr_ur", next(r for r in ur if r["style"] == style)))
mixed = [r for r in rows("ocr_mixed") if r["partition"] == "dev" and r["variant"] == "clean"]
for kind in ("form", "statement", "receipt", "chat", "mixline"):
    chosen.append(("ocr_mixed", next(r for r in mixed if r["kind"] == kind)))
en = [r for r in rows("ocr_en") if r["partition"] == "dev" and r["variant"] == "clean"]
chosen.append(("ocr_en", en[0]))
manifest = []
for folder, row in chosen:
    shutil.copy(data / "generated" / folder / row["file"], files / row["file"])
    manifest.append({"file": row["file"], "kind": row.get("kind") or "english page", "style": row.get("style"), "text": row["text"], "identifiers": row["identifiers"]})
json.dump(manifest, open(case / "truth.json", "w", encoding="utf-8"), ensure_ascii=False, indent=1)
print(len(manifest), "files in", files)
for m in manifest:
    print(" ", m["file"], "|", m["kind"], "|", len(m["identifiers"]), "identifiers")
