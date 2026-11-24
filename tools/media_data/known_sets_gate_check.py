#!/usr/bin/env python3
"""Gates K1 and K2 of docs/data/image-known-sets-preregistration-20261124.md on the stored hashes of the Orion Lahore Reference case.

The rule applied is exactly the one of forensic.image_known_matches (migration 039): a picture is flagged when its SHA-256 equals a hash of the set (tier identical) or its 256-bit hash is within
56 bits of a hash of the set (tier same_image). The set is made of the six pictures that have saved and cropped copies in the case (hash-only: their SHA-256 and 256-bit hash).
Ground truth is the file name: `<member>-resaved.jpg` must be flagged, `<member>-cropped.jpg` may or may not be (a crop beyond 56 bits is a stated limit), every other picture must not be.

Input: a text file of `filename|sha256|status|pdq256-bits` rows, one per image of the case (a read-only query of the case database):
    docker exec nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -tA -F '|' -c "BEGIN READ ONLY; SELECT e.original_filename, e.sha256, e.processing_status, coalesce(h.pdq256::text, '')
      FROM forensic.evidence_items e LEFT JOIN forensic.image_hash h ON h.tenant_id = e.tenant_id AND h.collection_id = e.collection_id AND h.evidence_id = e.evidence_id AND h.version_id = e.current_version_id
      WHERE e.tenant_id = 'default' AND e.collection_id = 'orion-lahore-reference-20261030' AND e.modality = 'image' AND e.current_version_id IS NOT NULL ORDER BY e.original_filename; ROLLBACK;" > hashes.txt
    python tools/media_data/known_sets_gate_check.py hashes.txt

File names of pictures that are not part of the test pack (real footage) are never printed; only their counts.
"""
import sys

MEMBERS = ["motorcycle-540806", "truck-558137", "bus-124952", "market-106909", "laptop-502852", "backpack-312878"]
MAX_DISTANCE = 56


def distance(a: str, b: str) -> int:
    return bin(int(a, 2) ^ int(b, 2)).count("1")


def main(path: str) -> int:
    rows = []
    for line in open(path, encoding="utf-8"):
        parts = line.rstrip("\n").split("|")
        if len(parts) != 4 or not parts[1]:
            continue
        name, sha, status, bits = parts
        rows.append((name, sha, status, bits))
    by_name = {r[0]: r for r in rows}
    member_rows = []
    for stem in MEMBERS:
        name = f"imgsearch-{stem}.jpg"
        if name not in by_name or not by_name[name][3]:
            print(f"MISSING member {name}")
            return 2
        member_rows.append(by_name[name])
    members_sha = {r[1] for r in member_rows}
    members_bits = [r[3] for r in member_rows]

    expected_yes = {f"imgsearch-{s}.jpg" for s in MEMBERS} | {f"imgsearch-{s}-resaved.jpg" for s in MEMBERS}
    optional = {f"imgsearch-{s}-cropped.jpg" for s in MEMBERS}
    flagged, nearest_other = {}, []
    for name, sha, status, bits in rows:
        best = None
        if sha in members_sha:
            best = (0, "identical")
        if bits:
            d = min(distance(bits, m) for m in members_bits)
            if d <= MAX_DISTANCE and (best is None or d < best[0]):
                best = (d, "same_image")
            if name not in expected_yes and name not in optional:
                nearest_other.append(d)
        if best:
            flagged[name] = best

    pictures = len(rows)
    with_hash = sum(1 for r in rows if r[3])
    missed = sorted(expected_yes - set(flagged))
    k1_hits = len(expected_yes & set(flagged))
    others = [r[0] for r in rows if r[0] not in expected_yes and r[0] not in optional]
    false_flags = sorted(n for n in others if n in flagged)
    cropped_flagged = sorted(n for n in optional if n in flagged)

    print(f"pictures in the case: {pictures} ({with_hash} with a hash); set: {len(MEMBERS)} members, hashes only")
    print(f"K1 recall: {k1_hits} of {len(expected_yes)} members and re-saved copies flagged" + (f"; MISSED {missed}" if missed else ""))
    resaved_d = [flagged[n][0] for n in sorted(expected_yes) if n in flagged and n.endswith("-resaved.jpg")]
    print(f"   re-saved copies: distances {sorted(resaved_d)} (limit {MAX_DISTANCE})")
    print(f"K2 false flags: {len(false_flags)} of {len(others)} other pictures flagged" + (f": {[n for n in false_flags if n.startswith('imgsearch-')]} (+ {sum(1 for n in false_flags if not n.startswith('imgsearch-'))} not in the test pack)" if false_flags else ""))
    print(f"   nearest unrelated picture to any member: {min(nearest_other) if nearest_other else 'n/a'} bits (limit {MAX_DISTANCE})")
    print(f"cropped copies flagged (reported only, either way is fine): {len(cropped_flagged)} of {len(optional)}")
    ok = not missed and not false_flags
    print("K1:", "MET" if not missed else "NOT MET", "| K2:", "MET" if not false_flags else "NOT MET")
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
