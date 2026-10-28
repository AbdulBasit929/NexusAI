#!/usr/bin/env python3
"""Fetch the public, licensed reference data for the typed families P1-B (device reference, transactions) into C:/tmp/nexusai-family-data/, OUTSIDE the repository.

    python -X utf8 tools/family_data/fetch.py --list
    python -X utf8 tools/family_data/fetch.py device
    python -X utf8 tools/family_data/fetch.py transactions_docs

It is the media tool (tools/media_data/fetch.py) with other groups: only files named here, never overwritten if the size matches, every file in manifest.jsonl with its SHA-256, size, source URL,
licence and fetch time; a file whose bytes differ from the manifest is refused. Standard library only. Plan: docs/data/subscriber-device-family-20261028.md section 5 and
docs/data/transactions-family-20261028.md section 5.

What is NOT here, and why: record-level Pakistani subscriber, SIM, call or bank-transaction data. None is lawfully published (only aggregates: PTA, PBS, SBP); the leaked personal databases that
circulate are never fetched. Those families are tested on the generator's synthetic files (tools/real_data/reserved.py) and, when the owner can lawfully share real samples, in the vault
(docs/data/real-data-handling-policy.md).
"""
import os
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, os.path.join(os.path.dirname(HERE), "media_data"))
import fetch as base  # noqa: E402

base.ROOT = os.environ.get("NEXUSAI_FAMILY_DATA", "C:/tmp/nexusai-family-data")
GH = "https://raw.githubusercontent.com"

base.GROUPS = {
    "device": [
        dict(url=f"{GH}/MoazEb/tac-database/main/{name}", rel=f"tac/{name}", licence="MIT (repository LICENSE; the TAC rows are community-collected, provenance in its README)", ledger="FAM-DEV-1",
             note=note)
        for name, note in (("tac_full.csv", "Type Allocation Codes to manufacturer and model, up to January 2026 (about 255,000 rows): the device reference for test"),
                           ("LICENSE", "licence text of the repository"), ("README.md", "provenance and columns of the TAC file"))
    ],
    "transactions_docs": [
        dict(url="https://huggingface.co/datasets/flwrlabs/fed-fraud-paysim-banks/resolve/main/README.md", rel="paysim/fed-fraud-paysim-banks-README.md", licence="CC-BY-4.0 (hub tag)",
             ledger="FAM-TXN-1", note="dataset card of a synthetic mobile-money transaction set (PaySim): column meanings, types and how it was generated"),
    ],
}

if __name__ == "__main__":
    sys.argv[0] = "fetch.py"
    base.main()
