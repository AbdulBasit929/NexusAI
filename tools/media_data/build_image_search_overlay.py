#!/usr/bin/env python3
"""Builds the worker image with the image-search descriptors as a NEW TAG on top of the running worker image. It does not touch :latest, the running container or any switch.

Why an overlay and not a rebuild: the files inside the running worker image (worker.py, media_pipeline.py) are OLDER than the repository's current ones (the live image is a chain of overlays), so
copying the repository's files over them would change other behaviour. This script takes the two files AS THEY ARE IN THE RUNNING IMAGE, applies exactly the lines of the image-search change
(each anchor must match exactly once), and adds image_search.py and image_search_service.py. The result is smoke-tested without network; going live is a separate step (tools/dev/live-window).

    python tools/media_data/build_image_search_overlay.py --tag image-search-20261123 [--base-tag latest]
"""
import argparse
import difflib
import pathlib
import subprocess
import sys
import tempfile

REPO = pathlib.Path(__file__).resolve().parents[2]
SOURCE = REPO / "ingestion" / "forensic_records"
IMAGE = "nexusai-forensic-records-worker"


def run(args, **kw):
    return subprocess.run(args, capture_output=True, **kw)


def fetch(base, name):
    done = run(["docker", "run", "--rm", "--entrypoint", "cat", base, f"/app/{name}"])
    if done.returncode != 0:
        sys.exit(f"cannot read /app/{name} from {base}: {done.stderr.decode(errors='replace')[:300]}")
    return done.stdout


def patch(name, data, edits):
    for anchor, new in edits:
        hits = []
        for eol in ("\r\n", "\n"):
            a = anchor.replace("\n", eol).encode()
            if data.count(a) == 1:
                hits.append((a, eol))
        if len(hits) != 1:
            sys.exit(f"{name}: anchor {anchor[:60]!r} matches {len(hits)} times (needs exactly one); the running image differs from what this patch expects")
        a, eol = hits[0]
        data = data.replace(a, new.replace("\n", eol).encode() if new is not None else a)
    return data


WORKER_EDITS = [
    ("    from media_artifact_scope import resolve_media_artifact_id\n",
     "    from media_artifact_scope import resolve_media_artifact_id\n"
     "\ntry:\n    from ingestion.forensic_records.image_search import IMAGE_SEARCH_CONTRACTS, IMAGE_SEARCH_VECTOR_CONTRACT, persist_image_search_rows\n"
     "except ModuleNotFoundError:  # pragma: no cover - supports direct worker.py execution\n"
     "    from image_search import IMAGE_SEARCH_CONTRACTS, IMAGE_SEARCH_VECTOR_CONTRACT, persist_image_search_rows\n"),
    ("                    IMAGE_EMBEDDING_CONTRACT,\n",
     "                    IMAGE_EMBEDDING_CONTRACT,\n                    IMAGE_SEARCH_VECTOR_CONTRACT,\n"),
    ("            persist_anpr_media_observation(cur, job, observation, index, artifact_id)\n",
     "            persist_anpr_media_observation(cur, job, observation, index, artifact_id)\n"
     "        if observation.contract_version in IMAGE_SEARCH_CONTRACTS:\n            persist_image_search_rows(cur, job, version_id, observation, artifact_id)\n"),
]
PIPELINE_EDITS = [
    ("def configured_media_processor() -> DeterministicMediaProcessor:\n", "def configured_media_processor() -> Any:\n"),
    ("    return DeterministicMediaProcessor(image_processor, audio_transcriber, video_anpr_processor, video_frame_processor)\n",
     "    processor = DeterministicMediaProcessor(image_processor, audio_transcriber, video_anpr_processor, video_frame_processor)\n"
     "    if os.getenv(\"FORENSIC_IMAGE_SEARCH_ENABLED\", \"false\").strip().lower() in {\"1\", \"true\", \"yes\", \"on\"}:\n"
     "        try:\n"
     "            from ingestion.forensic_records.image_search import ImageSearchAugmentedProcessor, search_processor_from_environment\n"
     "        except ModuleNotFoundError:  # pragma: no cover - direct worker image execution\n"
     "            from image_search import ImageSearchAugmentedProcessor, search_processor_from_environment\n"
     "        return ImageSearchAugmentedProcessor(processor, search_processor_from_environment())\n"
     "    return processor\n"),
]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--tag", required=True)
    parser.add_argument("--base-tag", default="latest")
    args = parser.parse_args()
    base, output = f"{IMAGE}:{args.base_tag}", f"{IMAGE}:{args.tag}"
    if not run(["docker", "images", "-q", base]).stdout.strip():
        sys.exit(f"base image {base} does not exist")
    if run(["docker", "images", "-q", output]).stdout.strip():
        sys.exit(f"{output} already exists; pick a new tag")
    base_id = run(["docker", "image", "inspect", "-f", "{{.Id}}", base]).stdout.decode().strip()
    print(f"base {base} = {base_id[7:19]}")
    worker_live, pipeline_live = fetch(base, "worker.py"), fetch(base, "media_pipeline.py")
    worker_new, pipeline_new = patch("worker.py", worker_live, WORKER_EDITS), patch("media_pipeline.py", pipeline_live, PIPELINE_EDITS)
    for name, live, new in (("worker.py", worker_live, worker_new), ("media_pipeline.py", pipeline_live, pipeline_new)):
        diff = [l for l in difflib.unified_diff(live.decode().splitlines(), new.decode().splitlines(), lineterm="", n=0) if l[:1] in "+-" and l[:3] not in ("+++", "---")]
        print(f"{name}: {sum(1 for l in diff if l[0] == '+')} lines added, {sum(1 for l in diff if l[0] == '-')} removed relative to the running image")
    with tempfile.TemporaryDirectory() as context:
        ctx = pathlib.Path(context)
        (ctx / "worker.py").write_bytes(worker_new)
        (ctx / "media_pipeline.py").write_bytes(pipeline_new)
        for name in ("image_search.py", "image_search_service.py"):
            (ctx / name).write_bytes((SOURCE / name).read_bytes())
        (ctx / "Dockerfile").write_text(f"FROM {base}\nCOPY worker.py /app/worker.py\nCOPY media_pipeline.py /app/media_pipeline.py\nCOPY image_search.py /app/image_search.py\nCOPY image_search_service.py /app/image_search_service.py\n", encoding="ascii")
        done = subprocess.run(["docker", "build", "-t", output, str(ctx)])
        if done.returncode != 0:
            sys.exit("docker build failed")
    print(f"built {output}; :latest and the running worker are unchanged")
    smoke = [
        ("files parse", ["python", "-B", "-c", "import ast;[ast.parse(open(p).read()) for p in ('/app/worker.py','/app/media_pipeline.py','/app/image_search.py','/app/image_search_service.py')];print('ok')"]),
        ("modules import", ["python", "-B", "-c", "import media_pipeline, image_search, image_search_service;print('ok')"]),
        ("flag off: the media processor is the one it was", ["python", "-B", "-c", "import media_pipeline as m;print(type(m.configured_media_processor()).__name__)"]),
        ("flag on + service address: wrapped, nothing is called yet", ["python", "-B", "-c", "import os;os.environ['FORENSIC_IMAGE_SEARCH_ENABLED']='true';os.environ['FORENSIC_IMAGE_SEARCH_URL']='http://x:1';os.environ['FORENSIC_IMAGE_SEARCH_SERVICE_KEY']='k';import media_pipeline as m;p=m.configured_media_processor();print(type(p).__name__,type(p.search).__name__)"]),
        ("worker module imports", ["python", "-B", "-c", "import worker;print(hasattr(worker,'persist_media_result'))"]),
    ]
    failed = False
    for label, command in smoke:
        done = run(["docker", "run", "--rm", "--network", "none", "--entrypoint", command[0], output, *command[1:]])
        text = (done.stdout + done.stderr).decode(errors="replace").strip().splitlines()
        print(f"smoke: {label}: {'OK' if done.returncode == 0 else 'FAILED'} {text[-1] if text else ''}")
        failed = failed or done.returncode != 0
    if failed:
        sys.exit("a smoke test failed; do not use this image")


if __name__ == "__main__":
    main()
