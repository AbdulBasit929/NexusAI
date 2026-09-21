#!/usr/bin/env python3
"""Freeze one fresh product proof; never dispatch inference or overwrite a freeze."""
import argparse
import hashlib
import json
import os
import re
import subprocess
import unicodedata
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
CORPUS = HERE / "small-english-product-corpus-v1.json"
FREEZE = HERE / "small-english-product-freeze-v1.json"
EVALUATOR = ROOT / "local-acceptance-models/nxb21-small-english-product-proof-v1/product-proof-evaluator.exe"


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def normalized(value):
    return " ".join(re.findall(r"\w+", unicodedata.normalize("NFKC", value).casefold()))


def write(path, value):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")


def validate():
    corpus = json.loads(CORPUS.read_text(encoding="utf-8"))
    assert {k: len(v) for k, v in corpus.items()} == {"semantic": 16, "router": 8, "synthesis": 13, "general": 8}
    rows = [c for section in corpus.values() for c in section]
    assert len({c["id"] for c in rows}) == 45
    questions = {c["id"]: normalized(c["question"]) for c in rows}
    assert len(set(questions.values())) == 45
    names = subprocess.check_output(["rg", "--files", "--hidden", "-g", "!.git", "-g", "!node_modules", "-g", "!vendor"], cwd=ROOT, text=True).splitlines()
    # Acceptance directories are gitignored: explicitly include their old corpora.
    acceptance_roots = [p for p in (ROOT / "local-acceptance-models").iterdir() if p.is_dir() and (p.name.startswith("nxb21") or p.name == "benchmark-results")]
    for acceptance_root in acceptance_roots:
        print("FRESHNESS_INVENTORY=" + acceptance_root.name, flush=True)
        for directory, directories, files in os.walk(acceptance_root):
            directories[:] = [d for d in directories if d not in {"venv", ".venv", "node_modules", "site-packages", "__pycache__", ".git"} and not any(s in d for s in ("go-cache", "go-tmp"))]
            names += [str((Path(directory) / f).relative_to(ROOT)) for f in files if f.endswith(".json")]
    collisions, scanned, hashes = [], 0, {}
    for name in sorted(set(names)):
        path = ROOT / name
        if path.resolve() == CORPUS or path == FREEZE or path.suffix not in {".json", ".go", ".js", ".jsx", ".py", ".ps1", ".md", ".txt", ".yaml", ".yml"}:
            continue
        if "small-english-product" in name or "product-proof-results" in name:
            continue
        text = path.read_text(encoding="utf-8", errors="replace")
        comparison = normalized(text)
        scanned += 1
        if scanned % 1000 == 0:
            print("FRESHNESS_FILES_SCANNED=" + str(scanned), flush=True)
        for case_id, question in questions.items():
            if question in comparison:
                collisions.append({"id": case_id, "path": name})
        if any(marker in name.lower() for marker in ("corpus", "holdout", "decision8")):
            hashes[path.relative_to(ROOT).as_posix()] = digest(path)
    if collisions:
        raise SystemExit(json.dumps({"freshness": "FAIL", "collisions": collisions}, indent=2))
    return {"state": "PASS", "cases": 45, "files_scanned": scanned, "scope": "NFKC/casefold/word-normalized exact overlap across repository text and ignored nxb21/benchmark-results acceptance JSON; installed model runtimes excluded; not a claim of semantic novelty", "prior_corpus_hashes": hashes}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--freeze", action="store_true")
    args = parser.parse_args()
    if FREEZE.exists():
        frozen = json.loads(FREEZE.read_text(encoding="utf-8"))
        assert digest(FREEZE) == Path(str(FREEZE) + ".sha256").read_text().strip()
        for path, expected in frozen["files"].items():
            assert digest(ROOT / path) == expected, path
        assert digest(HERE / "run_small_english_product_proof.ps1") == frozen["runner_sha256"]
        print("FROZEN_PRODUCT_PROOF_IDENTITY=PASS")
        return
    freshness = validate()
    print(json.dumps({k: v for k, v in freshness.items() if k != "prior_corpus_hashes"}, indent=2))
    if not args.freeze:
        return
    assert EVALUATOR.is_file(), "Precompile evaluator before freezing; standalone run must not build"
    assert not (EVALUATOR.parent / "product-proof.dispatched.lock").exists()
    frozen = json.loads((HERE / "current4b-final-characterization-freeze.json").read_text(encoding="utf-8"))
    # Reuse only the completed experiment's exact model/runtime/guard inputs.
    frozen = {key: frozen[key] for key in ("model", "runtime", "resource", "reload_envelope")}
    envelope = json.loads((ROOT / "configuration/current4b-runtime-envelope-v1.json").read_text(encoding="utf-8"))
    receipt_path = ROOT / envelope["source_receipt"]
    assert digest(receipt_path) == envelope["source_receipt_sha256"]
    frozen["accepted_runtime"] = json.loads(receipt_path.read_text(encoding="utf-8"))["runtime_after"]
    frozen.update(contract_version="nexusai.small-english-product-freeze/v1", proof_id="ep1-20260910", cases=45, corpus=CORPUS.relative_to(ROOT).as_posix(), evaluator=EVALUATOR.relative_to(ROOT).as_posix(), runner_sha256=digest(HERE / "run_small_english_product_proof.ps1"), freshness=freshness,
                  acceptance="All 45 cases captured; schema/known-decision/safety checks pass; independently adjudicate every synthesis/general answer and fallback count. No automatic strict or D closure.",
                  resource_track="CLOSED", scope="Source planner, source router classifier/policy and source narrative validator; synthetic packets; no retained query execution or end-to-end deployment proof")
    files = [CORPUS, EVALUATOR, Path(__file__), HERE / "run_current4b_final_characterization.ps1", ROOT / "configuration/current4b-runtime-envelope-v1.json", ROOT / "go.mod", ROOT / "go.sum"]
    for directory in ("api/forensic_records", "core/services/agents"):
        files += list((ROOT / directory).rglob("*.go"))
        files += list((ROOT / directory).rglob("*.json"))
    frozen["files"] = {p.relative_to(ROOT).as_posix(): digest(p) for p in sorted(set(files))}
    frozen["files"].update(freshness["prior_corpus_hashes"])
    write(FREEZE, frozen)
    Path(str(FREEZE) + ".sha256").write_text(digest(FREEZE) + "\n", encoding="utf-8")
    print("SMALL_ENGLISH_PRODUCT_PROOF_FROZEN=PASS")


if __name__ == "__main__":
    main()
