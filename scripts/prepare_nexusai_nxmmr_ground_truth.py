#!/usr/bin/env python3
"""Prepare and validate private NX-MMR human ground-truth packs without inference."""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import math
import subprocess
from pathlib import Path
from typing import Any, Iterable

from PIL import Image, ImageDraw, ImageFont, ImageOps


IMAGE_LABEL_FIELDS = [
    "plate_presence", "plate_count", "plate_text", "readability", "province",
    "angle", "blur", "occlusion", "lighting", "multiple_vehicles",
    "hard_negative", "privacy", "review_status", "reviewer", "reviewed_at",
]
VIDEO_LABEL_FIELDS = [
    "plate_visible", "plate_count", "plate_text", "readability",
    "multiple_vehicles", "hard_negative", "negative_interval", "speech_present",
    "transcript_raw", "transcript_language", "review_status", "reviewer",
    "reviewed_at",
]


def hash_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def canonical_digest(rows: Iterable[dict[str, Any]], fields: list[str]) -> str:
    payload = "\n".join(
        "\t".join(str(row.get(field, "")) for field in fields) for row in rows
    )
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def font(size: int) -> ImageFont.ImageFont:
    candidates = [
        Path("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"),
        Path("C:/Windows/Fonts/arial.ttf"),
    ]
    for candidate in candidates:
        if candidate.exists():
            return ImageFont.truetype(str(candidate), size)
    return ImageFont.load_default()


def write_csv(path: Path, fields: list[str], rows: list[dict[str, Any]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8-sig", newline="") as target:
        writer = csv.DictWriter(target, fieldnames=fields, extrasaction="ignore")
        writer.writeheader()
        writer.writerows(rows)


def make_contact_sheets(
    images: list[tuple[str, Path]], output: Path, *, columns: int = 4,
    cell_width: int = 420, cell_height: int = 330, per_sheet: int = 20,
) -> list[str]:
    output.mkdir(parents=True, exist_ok=True)
    label_font = font(20)
    names: list[str] = []
    for offset in range(0, len(images), per_sheet):
        page = images[offset:offset + per_sheet]
        rows = math.ceil(len(page) / columns)
        sheet = Image.new("RGB", (columns * cell_width, rows * cell_height), "white")
        draw = ImageDraw.Draw(sheet)
        for index, (label, path) in enumerate(page):
            x = (index % columns) * cell_width
            y = (index // columns) * cell_height
            with Image.open(path) as source:
                image = ImageOps.exif_transpose(source).convert("RGB")
                image.thumbnail((cell_width - 20, cell_height - 55))
                left = x + (cell_width - image.width) // 2
                top = y + 35 + (cell_height - 45 - image.height) // 2
                sheet.paste(image, (left, top))
            draw.rectangle((x, y, x + cell_width - 1, y + cell_height - 1), outline="#555555")
            draw.text((x + 10, y + 7), label, fill="black", font=label_font)
        name = f"contact-sheet-{offset // per_sheet + 1:02d}.jpg"
        sheet.save(output / name, quality=88, optimize=True)
        names.append(name)
    return names


def prepare_images(args: argparse.Namespace) -> None:
    root = args.input_root.resolve()
    output = args.output_root.resolve()
    if output == root or root in output.parents:
        raise SystemExit("output must not be inside the evidence directory")
    files = sorted(
        (path for path in root.rglob("*") if path.is_file() and path.suffix.lower() in {".jpg", ".jpeg"}),
        key=lambda path: path.relative_to(root).as_posix().casefold(),
    )
    if not files:
        raise SystemExit("no JPG/JPEG files found")
    inventories: list[dict[str, Any]] = []
    for index, path in enumerate(files, start=1):
        with Image.open(path) as source:
            width, height = source.size
        inventories.append({
            "sample_id": f"PKPLATE-{index:04d}",
            "relative_file": path.relative_to(root).as_posix(),
            "size_bytes": path.stat().st_size,
            "sha256": hash_file(path),
            "width": width,
            "height": height,
        })
    ranked = sorted(
        inventories,
        key=lambda row: hashlib.sha256(
            f"{args.seed}:{row['sha256']}".encode("utf-8")
        ).hexdigest(),
    )
    holdout_count = max(1, math.ceil(len(ranked) * args.holdout_percent))
    holdout_ids = {row["sample_id"] for row in ranked[:holdout_count]}
    fields = [
        "sample_id", "relative_file", "size_bytes", "sha256", "width", "height",
        "split", *IMAGE_LABEL_FIELDS, "label_source", "model_output_seen",
        "review_notes",
    ]
    rows = []
    for item in inventories:
        rows.append({
            **item,
            "split": "sealed_holdout" if item["sample_id"] in holdout_ids else "development",
            **{field: "" for field in IMAGE_LABEL_FIELDS},
            "label_source": "independent_human_annotation",
            "model_output_seen": "false",
            "review_notes": "",
        })
    output.mkdir(parents=True, exist_ok=True)
    manifest_path = output / "image-ground-truth-template.csv"
    write_csv(manifest_path, fields, rows)
    sheet_images = [(row["sample_id"], root / row["relative_file"]) for row in rows]
    sheets = make_contact_sheets(sheet_images, output / "image-contact-sheets")
    split_fields = ["sample_id", "relative_file", "sha256", "split"]
    split_digest = canonical_digest(rows, split_fields)
    protocol = {
        "contract_version": "nexusai.nxmmr.image-ground-truth-pack/v1",
        "ground_truth_authority": "independent_human_annotation_only",
        "model_output_may_define_or_influence_labels": False,
        "source_root": str(root),
        "sample_count": len(rows),
        "total_bytes": sum(int(row["size_bytes"]) for row in rows),
        "holdout_count": holdout_count,
        "holdout_fraction": holdout_count / len(rows),
        "split_seed": args.seed,
        "split_digest": split_digest,
        "split_digest_fields": split_fields,
        "inventory_digest": canonical_digest(rows, ["relative_file", "size_bytes", "sha256"]),
        "template": manifest_path.name,
        "contact_sheets": sheets,
        "privacy": "local_non_retained_do_not_commit_or_publish",
        "status": "PENDING_INDEPENDENT_HUMAN_LABELS",
    }
    (output / "image-ground-truth-protocol.json").write_text(
        json.dumps(protocol, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    print(json.dumps(protocol, ensure_ascii=False, indent=2))


def run_json(command: list[str]) -> dict[str, Any]:
    completed = subprocess.run(command, check=True, capture_output=True, text=True, timeout=60)
    return json.loads(completed.stdout)


def prepare_video(args: argparse.Namespace) -> None:
    video = args.video.resolve()
    output = args.output_root.resolve()
    output.mkdir(parents=True, exist_ok=True)
    metadata = run_json([
        "ffprobe", "-v", "error", "-show_format", "-show_streams", "-of", "json", str(video)
    ])
    duration = float(metadata["format"]["duration"])
    audio_streams = [stream for stream in metadata.get("streams", []) if stream.get("codec_type") == "audio"]
    frames_dir = output / "video-review-frames"
    frames_dir.mkdir(parents=True, exist_ok=True)
    pattern = frames_dir / "frame-%04d.jpg"
    subprocess.run([
        "ffmpeg", "-nostdin", "-v", "error", "-i", str(video),
        "-vf", f"fps=1/{args.interval_seconds},scale={args.frame_width}:-2",
        "-q:v", "3", "-y", str(pattern),
    ], check=True, timeout=300)
    frames = sorted(frames_dir.glob("frame-*.jpg"))
    sheet_images = [
        (f"{index * args.interval_seconds:.3f}s", path)
        for index, path in enumerate(frames)
    ]
    sheets = make_contact_sheets(sheet_images, output / "video-contact-sheets")
    review_audio = None
    if audio_streams:
        review_audio = output / "review-audio.m4a"
        subprocess.run([
            "ffmpeg", "-nostdin", "-v", "error", "-i", str(video), "-vn",
            "-c:a", "copy", "-y", str(review_audio),
        ], check=True, timeout=120)
    fields = [
        "segment_id", "start_seconds", "end_seconds", "review_frame",
        *VIDEO_LABEL_FIELDS, "label_source", "model_output_seen", "review_notes",
    ]
    rows = []
    for index, path in enumerate(frames):
        start = index * args.interval_seconds
        rows.append({
            "segment_id": f"VIDEO-{index + 1:04d}",
            "start_seconds": f"{start:.3f}",
            "end_seconds": f"{min(duration, start + args.interval_seconds):.3f}",
            "review_frame": path.relative_to(output).as_posix(),
            **{field: "" for field in VIDEO_LABEL_FIELDS},
            "label_source": "independent_human_annotation",
            "model_output_seen": "false",
            "review_notes": "Refine exact first/last source times in the original video when a plate or speech is present.",
        })
    template = output / "video-ground-truth-template.csv"
    write_csv(template, fields, rows)
    protocol = {
        "contract_version": "nexusai.nxmmr.video-ground-truth-pack/v1",
        "ground_truth_authority": "independent_human_annotation_only",
        "model_output_may_define_or_influence_labels": False,
        "source": {"path": str(video), "size_bytes": video.stat().st_size, "sha256": hash_file(video)},
        "ffprobe": metadata,
        "coarse_interval_seconds": args.interval_seconds,
        "review_frame_count": len(frames),
        "template": template.name,
        "contact_sheets": sheets,
        "audio_stream_count": len(audio_streams),
        "review_audio": review_audio.name if review_audio else None,
        "labeling_rule": "Mark negative intervals explicitly; refine exact plate/speech interval boundaries against the original video and enter verbatim raw transcript only after human listening.",
        "privacy": "local_non_retained_do_not_commit_or_publish",
        "status": "PENDING_INDEPENDENT_HUMAN_LABELS",
    }
    (output / "video-ground-truth-protocol.json").write_text(
        json.dumps(protocol, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    print(json.dumps({
        "source_sha256": protocol["source"]["sha256"], "duration_seconds": duration,
        "review_frames": len(frames), "contact_sheets": len(sheets),
        "status": protocol["status"],
    }, indent=2))


def validate(args: argparse.Namespace) -> None:
    with args.manifest.open("r", encoding="utf-8-sig", newline="") as source:
        rows = list(csv.DictReader(source))
    if not rows:
        raise SystemExit("manifest contains no rows")
    required = IMAGE_LABEL_FIELDS if "relative_file" in rows[0] else VIDEO_LABEL_FIELDS
    errors: list[str] = []
    for index, row in enumerate(rows, start=2):
        identity = row.get("sample_id") or row.get("segment_id") or f"row-{index}"
        for field in required:
            if not str(row.get(field, "")).strip():
                errors.append(f"{identity}: missing {field}")
        if row.get("label_source") != "independent_human_annotation":
            errors.append(f"{identity}: label_source must be independent_human_annotation")
        if str(row.get("model_output_seen", "")).strip().lower() != "false":
            errors.append(f"{identity}: model_output_seen must remain false while truth is sealed")
    holdout = [row for row in rows if row.get("split") == "sealed_holdout"]
    if holdout and len(holdout) / len(rows) < 0.20:
        errors.append("sealed holdout is below 20 percent")
    result = {
        "contract_version": "nexusai.nxmmr.ground-truth-validation/v1",
        "manifest": str(args.manifest.resolve()),
        "rows": len(rows),
        "holdout_rows": len(holdout),
        "errors": errors,
        "status": "PASS" if not errors else "PENDING_OR_INVALID",
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))
    if errors:
        raise SystemExit(2)


def main() -> None:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)
    images = subparsers.add_parser("prepare-images")
    images.add_argument("--input-root", type=Path, required=True)
    images.add_argument("--output-root", type=Path, required=True)
    images.add_argument("--holdout-percent", type=float, default=0.20)
    images.add_argument("--seed", default="NXMMR-20260829-SEALED-SPLIT-V1")
    images.set_defaults(function=prepare_images)
    video = subparsers.add_parser("prepare-video")
    video.add_argument("--video", type=Path, required=True)
    video.add_argument("--output-root", type=Path, required=True)
    video.add_argument("--interval-seconds", type=float, default=2.0)
    video.add_argument("--frame-width", type=int, default=640)
    video.set_defaults(function=prepare_video)
    validator = subparsers.add_parser("validate")
    validator.add_argument("manifest", type=Path)
    validator.set_defaults(function=validate)
    args = parser.parse_args()
    if getattr(args, "holdout_percent", 0.20) < 0.20 or getattr(args, "holdout_percent", 0.20) >= 1:
        parser.error("holdout-percent must be at least 0.20 and below 1")
    if getattr(args, "interval_seconds", 2.0) <= 0:
        parser.error("interval-seconds must be positive")
    args.function(args)


if __name__ == "__main__":
    main()
