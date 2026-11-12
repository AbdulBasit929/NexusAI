#!/usr/bin/env python3
"""Patch a worker.py (the repository's, or the RUNNING worker image's own) so that a media reading is stored under a version-scoped id when its content id belongs to another evidence version.

    python -X utf8 patch_worker_artifact_scope.py <worker.py> <output path>

See ingestion/forensic_records/media_artifact_scope.py for the defect (the second copy of the same bytes in another case lost every reading) and the rule. Six edits in persist_media_result and
persist_anpr_media_observation; the mixed CRLF and LF line endings of the file are kept line by line, and every anchor must occur exactly once or nothing is written (the image's worker.py is an
older build than the repository's, so the anchors are the texts that both share).
"""
import re
import sys

EOL = r"(\r?\n)"

EDITS = [
    # 1. the import, next to the other sibling modules
    (
        r"(try:" + EOL + r"    from ingestion\.forensic_records\.media_pipeline import \()",
        lambda m: "try:" + m.group(2) + "    from ingestion.forensic_records.media_artifact_scope import resolve_media_artifact_id" + m.group(2)
        + "except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution" + m.group(2)
        + "    from media_artifact_scope import resolve_media_artifact_id" + m.group(2) + m.group(2)
        + "try:" + m.group(2) + "    from ingestion.forensic_records.media_pipeline import (",
    ),
    # 2. the resolved id, computed once per observation after its metadata is built
    (
        r"(                else \"derived_technical_observation\"" + EOL + r"            \)," + EOL + r"        \}" + EOL + r")(        cur\.execute\()",
        lambda m: m.group(1) + "        artifact_id = resolve_media_artifact_id(cur, version_id, observation.observation_id)" + m.group(4) + m.group(5),
    ),
    # 3. the artifact insert takes it
    (
        r"(                )observation\.observation_id,(" + EOL + r"                job\.tenant_id,)",
        lambda m: m.group(1) + "artifact_id," + m.group(2),
    ),
    # 4. the plate record gets the same id
    (
        r"persist_anpr_media_observation\(cur, job, observation, index\)",
        lambda m: "persist_anpr_media_observation(cur, job, observation, index, artifact_id)",
    ),
    # 5. its signature, its payload (so that the row hash of a scoped reading is its own) and its link
    (
        r"(    observation: Any," + EOL + r"    row_number: int,)(" + EOL + r"\) -> None:" + EOL + r"    raw_plate = )",
        lambda m: m.group(1) + m.group(2) + "    artifact_id: Any = None," + m.group(3),
    ),
    (
        r"(        \"observation_id\": )observation\.observation_id,(" + EOL + r"        \"observation_state\": \"model_candidate\",)",
        lambda m: m.group(1) + "artifact_id or observation.observation_id," + m.group(2),
    ),
    (
        r"(            json\.dumps\(normalized_fields\)," + EOL + r"            )observation\.observation_id,(" + EOL + r"        \)," + EOL + r"    \)" + EOL + EOL + EOL + r"def materialize_media_kb_asset)",
        lambda m: m.group(1) + "artifact_id or observation.observation_id," + m.group(3),
    ),
]


def patch(text):
    for pattern, replace in EDITS:
        matches = list(re.finditer(pattern, text))
        if len(matches) != 1:
            raise SystemExit("anchor %r occurs %d times (expected 1); nothing written" % (pattern[:70], len(matches)))
        text = text[: matches[0].start()] + replace(matches[0]) + text[matches[0].end():]
    return text


def main():
    source, target = sys.argv[1], sys.argv[2]
    text = open(source, encoding="utf-8", newline="").read()
    if "resolve_media_artifact_id" in text:
        raise SystemExit("already patched; nothing written")
    patched = patch(text)
    open(target, "w", encoding="utf-8", newline="").write(patched)
    print("patched: %d edits, %s" % (len(EDITS), target))


if __name__ == "__main__":
    main()
