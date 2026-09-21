#!/usr/bin/env python3
"""Loopback-only, gold-label-only NX-MMR human verification utility.

This tool serves original private media from an explicit read-only allowlist and
writes only to the Git-ignored NX-MMR private ground-truth directory. It does
not import or invoke ANPR, OCR, ASR, LLM, LocalAI, or product persistence code.
"""

from __future__ import annotations

import argparse
import csv
import hashlib
import json
import mimetypes
import os
import re
import secrets
import shutil
import sys
import threading
import webbrowser
from datetime import datetime, timezone
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Any
from urllib.parse import urlparse


CONTRACT_VERSION = "nexusai.nxmmr.human-verification/v1"
DEV_SEED = "NXMMR-20260829-HUMAN-DEV-V1"
REPLACEMENT_SEED = "NXMMR-20260829-INDEPENDENCE-REPLACEMENT-V1"
PRIVATE_RELATIVE = Path("local-acceptance-models/nxmmr/private-ground-truth")
IMAGE_FIELDS = [
    "sample_id", "relative_file", "size_bytes", "sha256", "width", "height",
    "split", "plate_presence", "plate_count", "plate_text", "readability",
    "difficulty", "privacy", "review_status", "reviewer", "reviewed_at",
    "label_source", "model_output_seen", "review_notes",
]
VIDEO_FIELDS = [
    "event_id", "start_seconds", "end_seconds", "event_type", "plate_count",
    "plate_text", "readability", "reviewer", "reviewed_at", "label_source",
    "model_output_seen", "privacy", "review_notes",
]
SPEECH_FIELDS = [
    "segment_id", "start_seconds", "end_seconds", "speech_present",
    "transcript_raw", "transcript_language", "unintelligible", "reviewer",
    "reviewed_at", "label_source", "model_output_seen", "privacy", "review_notes",
]


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z")


def hash_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as source:
        for block in iter(lambda: source.read(1024 * 1024), b""):
            digest.update(block)
    return digest.hexdigest()


def canonical_digest(value: Any) -> str:
    payload = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def is_within(path: Path, parent: Path) -> bool:
    try:
        path.resolve().relative_to(parent.resolve())
        return True
    except ValueError:
        return False


def atomic_json(path: Path, value: Any) -> None:
    temporary = path.with_suffix(path.suffix + ".tmp")
    temporary.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    os.replace(temporary, path)


def atomic_csv(path: Path, fields: list[str], rows: list[dict[str, Any]]) -> None:
    temporary = path.with_suffix(path.suffix + ".tmp")
    with temporary.open("w", encoding="utf-8-sig", newline="") as target:
        writer = csv.DictWriter(target, fieldnames=fields, extrasaction="ignore")
        writer.writeheader()
        writer.writerows(rows)
    os.replace(temporary, path)


def ranked(rows: list[dict[str, str]], seed: str) -> list[dict[str, str]]:
    return sorted(
        rows,
        key=lambda row: hashlib.sha256(
            f"{seed}:{row['sample_id']}:{row['sha256']}".encode("utf-8")
        ).hexdigest(),
    )


def select_development(rows: list[dict[str, str]], count: int = 10) -> list[str]:
    """Select a deterministic metadata-diverse set without candidate outputs."""
    development = [row for row in rows if row["split"] == "development"]
    ordered_area = sorted(development, key=lambda row: int(row["width"]) * int(row["height"]))
    area_quartile = {
        row["sample_id"]: min(3, index * 4 // max(1, len(ordered_area)))
        for index, row in enumerate(ordered_area)
    }
    buckets: dict[tuple[str, int], list[dict[str, str]]] = {}
    for row in development:
        parent = Path(row["relative_file"]).parent.as_posix().casefold()
        buckets.setdefault((parent, area_quartile[row["sample_id"]]), []).append(row)
    selected: list[str] = []
    bucket_keys = sorted(buckets)
    bucket_rows = {key: ranked(buckets[key], DEV_SEED) for key in bucket_keys}
    while len(selected) < min(count, len(development)):
        changed = False
        for key in bucket_keys:
            if bucket_rows[key] and len(selected) < count:
                selected.append(bucket_rows[key].pop(0)["sample_id"])
                changed = True
        if not changed:
            break
    return selected


class ReviewStore:
    def __init__(
        self,
        *,
        repo_root: Path,
        image_root: Path,
        video: Path,
        private_root: Path,
        verify_hashes: bool = True,
    ) -> None:
        self.repo_root = repo_root.resolve()
        expected_private = (self.repo_root / PRIVATE_RELATIVE).resolve()
        self.private_root = private_root.resolve()
        if self.private_root != expected_private:
            raise ValueError(f"write root must be exactly {expected_private}")
        self.image_root = image_root.resolve()
        self.video = video.resolve()
        if not self.image_root.is_dir() or not self.video.is_file():
            raise ValueError("approved image root and video must exist")
        self.images_dir = self.private_root / "images"
        self.video_dir = self.private_root / "video"
        self.output_dir = self.private_root / "human-verification"
        self.state_path = self.output_dir / "review-state.json"
        self.manifest_path = self.images_dir / "image-ground-truth-template.csv"
        self.image_protocol_path = self.images_dir / "image-ground-truth-protocol.json"
        self.video_protocol_path = self.video_dir / "video-ground-truth-protocol.json"
        self.audio = self.video_dir / "review-audio.m4a"
        for required in (self.manifest_path, self.image_protocol_path, self.video_protocol_path, self.audio):
            if not required.is_file():
                raise ValueError(f"required prepared artifact is missing: {required}")
        self.output_dir.mkdir(parents=True, exist_ok=True)
        with self.manifest_path.open("r", encoding="utf-8-sig", newline="") as source:
            self.inventory = list(csv.DictReader(source))
        self.by_id = {row["sample_id"]: row for row in self.inventory}
        self._verify_inventory_paths()
        if verify_hashes:
            self._verify_source_integrity()
        self._lock = threading.RLock()
        self.state = self._load_or_initialize()
        self._write_outputs()

    def _verify_inventory_paths(self) -> None:
        for row in self.inventory:
            candidate = (self.image_root / Path(row["relative_file"])).resolve()
            if not is_within(candidate, self.image_root) or not candidate.is_file():
                raise ValueError(f"image allowlist entry is missing or escapes root: {row['sample_id']}")

    def _verify_source_integrity(self) -> None:
        mismatches = []
        for row in self.inventory:
            candidate = self.image_path(row["sample_id"])
            if candidate.stat().st_size != int(row["size_bytes"]) or hash_file(candidate) != row["sha256"]:
                mismatches.append(row["sample_id"])
        video_protocol = json.loads(self.video_protocol_path.read_text(encoding="utf-8"))
        expected_video = video_protocol["source"]
        if self.video.stat().st_size != int(expected_video["size_bytes"]) or hash_file(self.video) != expected_video["sha256"]:
            mismatches.append("sample.mp4")
        if mismatches:
            raise ValueError("source integrity mismatch: " + ", ".join(mismatches))

    def _load_or_initialize(self) -> dict[str, Any]:
        if self.state_path.is_file():
            state = json.loads(self.state_path.read_text(encoding="utf-8"))
            if state.get("contract_version") != CONTRACT_VERSION:
                raise ValueError("existing review state uses a different contract version")
            expected = state.get("source_binding", {})
            if expected.get("image_inventory_digest") != self.inventory_digest():
                raise ValueError("existing review state is bound to a different image inventory")
            receipt = state.get("vad_receipt") or {}
            segments = state.get("speech_segments") or []
            if receipt.get("fallback") in {"digital_silence_audit", "no_active_window_audit"} and segments and all(
                segment.get("review_status") == "pending" for segment in segments
            ):
                duration = float(receipt.get("audio_duration_seconds", 60.010))
                width = duration / 6
                state["speech_segments"] = [
                    {
                        **segments[0], "segment_id": f"SP-{index + 1:03d}",
                        "start_seconds": round(index * width, 3),
                        "end_seconds": round(duration if index == 5 else (index + 1) * width, 3),
                    }
                    for index in range(6)
                ]
                atomic_json(self.state_path, state)
            return state
        dev_ids = select_development(self.inventory, 10)
        holdout_ids = [row["sample_id"] for row in self.inventory if row["split"] == "sealed_holdout"]
        state = {
            "contract_version": CONTRACT_VERSION,
            "mode": "GOLD_LABEL_ONLY",
            "created_at": utc_now(),
            "updated_at": utc_now(),
            "locked": False,
            "reviewer": "",
            "development_frozen": False,
            "source_binding": {
                "image_inventory_digest": self.inventory_digest(),
                "original_split_digest": self.original_split_digest(),
                "video_size_bytes": self.video.stat().st_size,
                "video_sha256": json.loads(self.video_protocol_path.read_text(encoding="utf-8"))["source"]["sha256"],
            },
            "active": {"development": dev_ids, "sealed_holdout": holdout_ids},
            "independence": {sample_id: "unknown" for sample_id in dev_ids + holdout_ids},
            "excluded_exposed": [],
            "image_labels": {},
            "video_events": [],
            "video_review": {"watched_full_source": False, "review_notes": ""},
            "speech_segments": [],
            "vad_receipt": None,
            "validation": {"status": "PENDING", "errors": []},
        }
        atomic_json(self.state_path, state)
        return state

    def inventory_digest(self) -> str:
        rows = [
            {key: row[key] for key in ("sample_id", "relative_file", "size_bytes", "sha256", "width", "height", "split")}
            for row in self.inventory
        ]
        return canonical_digest(rows)

    def original_split_digest(self) -> str:
        rows = [
            {key: row[key] for key in ("sample_id", "relative_file", "sha256", "split")}
            for row in self.inventory
        ]
        payload = "\n".join("\t".join(str(row[field]) for field in row) for row in rows)
        return hashlib.sha256(payload.encode("utf-8")).hexdigest()

    def image_path(self, sample_id: str) -> Path:
        row = self.by_id.get(sample_id)
        if not row:
            raise KeyError(sample_id)
        candidate = (self.image_root / Path(row["relative_file"])).resolve()
        if not is_within(candidate, self.image_root):
            raise PermissionError(sample_id)
        return candidate

    def _public_image(self, sample_id: str, split: str) -> dict[str, Any]:
        row = self.by_id[sample_id]
        return {
            "sample_id": sample_id,
            "relative_file": row["relative_file"],
            "size_bytes": int(row["size_bytes"]),
            "sha256": row["sha256"],
            "width": int(row["width"]),
            "height": int(row["height"]),
            "split": split,
            "independence": self.state["independence"].get(sample_id, "unknown"),
            "label": self.state["image_labels"].get(sample_id),
            "media_url": f"/media/image/{sample_id}",
        }

    def public_state(self) -> dict[str, Any]:
        with self._lock:
            validation = self.validate()
            development = [self._public_image(sample_id, "development") for sample_id in self.state["active"]["development"]]
            holdout = [self._public_image(sample_id, "sealed_holdout") for sample_id in self.state["active"]["sealed_holdout"]]
            return {
                "contract_version": CONTRACT_VERSION,
                "mode": self.state["mode"],
                "locked": self.state["locked"],
                "reviewer": self.state["reviewer"],
                "development_frozen": self.state["development_frozen"],
                "images": {"development": development, "sealed_holdout": holdout},
                "excluded_exposed": self.state["excluded_exposed"],
                "video": {
                    "url": "/media/video", "duration_seconds": 60.010,
                    "events": self.state["video_events"], "review": self.state["video_review"],
                },
                "speech": {
                    "audio_url": "/media/audio", "segments": self.state["speech_segments"],
                    "vad_receipt": self.state["vad_receipt"],
                },
                "progress": {
                    "development": sum(item["label"] is not None for item in development),
                    "development_total": len(development),
                    "holdout": sum(item["label"] is not None for item in holdout),
                    "holdout_total": len(holdout),
                    "speech": sum(segment.get("review_status") == "complete" for segment in self.state["speech_segments"]),
                    "speech_total": len(self.state["speech_segments"]),
                },
                "validation": validation,
                "privacy": "local_non_retained_do_not_commit_or_publish",
                "prohibitions": ["model inference", "product database", "Activity", "retained services", "external upload"],
            }

    def _assert_mutable(self) -> None:
        if self.state["locked"]:
            raise ValueError("gold labels are locked; comparison mode is a separate later action")

    def _save(self) -> None:
        self.state["updated_at"] = utc_now()
        self.state["validation"] = self.validate()
        atomic_json(self.state_path, self.state)
        self._write_outputs()

    def _replacement(self, split: str) -> str:
        active = set(self.state["active"]["development"] + self.state["active"]["sealed_holdout"])
        excluded = {item["sample_id"] for item in self.state["excluded_exposed"]}
        candidates = [
            row for row in self.inventory
            if row["split"] == "development" and row["sample_id"] not in active | excluded
        ]
        ordered = ranked(candidates, f"{REPLACEMENT_SEED}:{split}")
        if not ordered:
            raise ValueError("no deterministic unreviewed replacement remains")
        return ordered[0]["sample_id"]

    def action(self, payload: dict[str, Any]) -> None:
        with self._lock:
            action = str(payload.get("action", ""))
            if action == "set_reviewer":
                self._assert_mutable()
                reviewer = str(payload.get("reviewer", "")).strip()
                if len(reviewer) < 2 or len(reviewer) > 100:
                    raise ValueError("reviewer must be 2-100 characters")
                self.state["reviewer"] = reviewer
            elif action == "mark_independence":
                self._mark_independence(payload)
            elif action == "save_image":
                self._save_image(payload)
            elif action == "freeze_development":
                self._freeze_development()
            elif action == "save_video_event":
                self._save_video_event(payload)
            elif action == "delete_video_event":
                self._assert_mutable()
                event_id = str(payload.get("event_id", ""))
                self.state["video_events"] = [event for event in self.state["video_events"] if event["event_id"] != event_id]
            elif action == "set_video_review":
                self._assert_mutable()
                self.state["video_review"] = {
                    "watched_full_source": bool(payload.get("watched_full_source")),
                    "review_notes": str(payload.get("review_notes", "")).strip()[:1000],
                }
            elif action == "set_speech_segments":
                self._set_speech_segments(payload)
            elif action == "save_speech":
                self._save_speech(payload)
            elif action == "finish":
                self._finish()
            else:
                raise ValueError("unknown action")
            self._save()

    def _active_split(self, sample_id: str) -> str:
        for split in ("development", "sealed_holdout"):
            if sample_id in self.state["active"][split]:
                return split
        raise ValueError("sample is not in the active review set")

    def _mark_independence(self, payload: dict[str, Any]) -> None:
        self._assert_mutable()
        sample_id = str(payload.get("sample_id", ""))
        split = self._active_split(sample_id)
        seen = payload.get("model_output_seen")
        if not isinstance(seen, bool):
            raise ValueError("model_output_seen must be explicitly true or false")
        if seen:
            replacement = self._replacement(split)
            position = self.state["active"][split].index(sample_id)
            self.state["active"][split][position] = replacement
            self.state["independence"][sample_id] = "seen"
            self.state["independence"][replacement] = "unknown"
            self.state["excluded_exposed"].append({
                "sample_id": sample_id, "original_split": split,
                "reason": "candidate_model_output_previously_seen",
                "replacement_sample_id": replacement, "recorded_at": utc_now(),
                "reviewer": self.state["reviewer"],
            })
            self.state["image_labels"].pop(sample_id, None)
        else:
            self.state["independence"][sample_id] = "unseen"

    def _save_image(self, payload: dict[str, Any]) -> None:
        self._assert_mutable()
        if not self.state["reviewer"]:
            raise ValueError("set reviewer before saving labels")
        sample_id = str(payload.get("sample_id", ""))
        split = self._active_split(sample_id)
        if split == "sealed_holdout" and not self.state["development_frozen"]:
            raise ValueError("freeze the development labels before opening holdout labeling")
        if self.state["independence"].get(sample_id) != "unseen":
            raise ValueError("independence must be confirmed before labeling")
        presence = str(payload.get("plate_presence", "")).strip().lower()
        if presence not in {"yes", "no"}:
            raise ValueError("plate presence must be yes or no")
        count = int(payload.get("plate_count", 0))
        text = str(payload.get("plate_text", "")).strip()
        readability = str(payload.get("readability", "")).strip().lower()
        difficulty = str(payload.get("difficulty", "")).strip().lower()
        if difficulty not in {"", "easy", "medium", "hard"}:
            raise ValueError("difficulty must be blank, easy, medium, or hard")
        if presence == "no":
            if count != 0 or text or readability != "not_applicable":
                raise ValueError("no-plate labels require count 0, blank text, and not_applicable readability")
        else:
            if count < 1 or readability not in {"readable", "partial", "unreadable"}:
                raise ValueError("plate labels require a positive count and readability")
            lines = [line.strip() for line in text.splitlines() if line.strip()]
            if readability == "readable" and len(lines) != count:
                raise ValueError("enter one exact plate text line per readable plate")
            if readability == "unreadable" and text:
                raise ValueError("unreadable plate text must be blank")
        self.state["image_labels"][sample_id] = {
            "plate_presence": presence, "plate_count": count, "plate_text": text,
            "readability": readability, "difficulty": difficulty,
            "review_notes": str(payload.get("review_notes", "")).strip()[:1000],
            "review_status": "complete", "reviewer": self.state["reviewer"],
            "reviewed_at": utc_now(), "label_source": "independent_human_annotation",
            "model_output_seen": False,
            "privacy": "local_non_retained_do_not_commit_or_publish",
        }

    def _freeze_development(self) -> None:
        self._assert_mutable()
        missing = [sample_id for sample_id in self.state["active"]["development"] if sample_id not in self.state["image_labels"]]
        if missing:
            raise ValueError(f"development labels are incomplete ({len(missing)} remaining)")
        self.state["development_frozen"] = True
        self.state["development_frozen_at"] = utc_now()
        self.state["development_digest"] = canonical_digest([
            {"sample_id": sample_id, **self.state["image_labels"][sample_id]}
            for sample_id in self.state["active"]["development"]
        ])

    def _save_video_event(self, payload: dict[str, Any]) -> None:
        self._assert_mutable()
        if not self.state["reviewer"]:
            raise ValueError("set reviewer before saving events")
        start = round(float(payload.get("start_seconds")), 3)
        end = round(float(payload.get("end_seconds")), 3)
        if start < 0 or end <= start or end > 60.010:
            raise ValueError("video event bounds must be ordered within 0-60.010 seconds")
        event_type = str(payload.get("event_type", ""))
        if event_type not in {"plate", "negative"}:
            raise ValueError("event type must be plate or negative")
        count = int(payload.get("plate_count", 0))
        text = str(payload.get("plate_text", "")).strip()
        readability = str(payload.get("readability", "")).strip()
        if event_type == "negative":
            count, text, readability = 0, "", "not_applicable"
        elif count < 1 or readability not in {"readable", "partial", "unreadable"}:
            raise ValueError("plate events require count and readability")
        event_id = str(payload.get("event_id", "")) or f"VE-{secrets.token_hex(4)}"
        event = {
            "event_id": event_id, "start_seconds": start, "end_seconds": end,
            "event_type": event_type, "plate_count": count, "plate_text": text,
            "readability": readability, "reviewer": self.state["reviewer"],
            "reviewed_at": utc_now(), "label_source": "independent_human_annotation",
            "model_output_seen": False, "privacy": "local_non_retained_do_not_commit_or_publish",
            "review_notes": str(payload.get("review_notes", "")).strip()[:1000],
        }
        self.state["video_events"] = [item for item in self.state["video_events"] if item["event_id"] != event_id]
        self.state["video_events"].append(event)
        self.state["video_events"].sort(key=lambda item: (item["start_seconds"], item["end_seconds"]))

    def _set_speech_segments(self, payload: dict[str, Any]) -> None:
        self._assert_mutable()
        if self.state["speech_segments"]:
            raise ValueError("speech candidates already exist; labels are not silently replaced")
        raw_segments = payload.get("segments")
        receipt = payload.get("receipt")
        if not isinstance(raw_segments, list) or not raw_segments or len(raw_segments) > 30:
            raise ValueError("audio-energy segmentation must produce 1-30 bounded candidates")
        segments = []
        prior_end = -1.0
        for index, raw in enumerate(raw_segments, start=1):
            start = round(float(raw["start_seconds"]), 3)
            end = round(float(raw["end_seconds"]), 3)
            if start < 0 or end <= start or end > 60.010 or start < prior_end:
                raise ValueError("speech candidates must be ordered, non-overlapping, and in bounds")
            if end - start > 15.0:
                raise ValueError("speech candidates must be no longer than 15 seconds")
            prior_end = end
            segments.append({
                "segment_id": f"SP-{index:03d}", "start_seconds": start, "end_seconds": end,
                "speech_present": "", "transcript_raw": "", "transcript_language": "",
                "unintelligible": False, "review_status": "pending", "reviewer": "",
                "reviewed_at": "", "label_source": "independent_human_annotation",
                "model_output_seen": False, "privacy": "local_non_retained_do_not_commit_or_publish",
                "review_notes": "",
            })
        if not isinstance(receipt, dict) or receipt.get("method") != "browser_audio_energy_v1":
            raise ValueError("only the non-semantic browser audio-energy proposal is accepted")
        self.state["speech_segments"] = segments
        self.state["vad_receipt"] = {**receipt, "created_at": utc_now(), "semantic_model_used": False}

    def _save_speech(self, payload: dict[str, Any]) -> None:
        self._assert_mutable()
        if not self.state["reviewer"]:
            raise ValueError("set reviewer before saving speech labels")
        segment_id = str(payload.get("segment_id", ""))
        segment = next((item for item in self.state["speech_segments"] if item["segment_id"] == segment_id), None)
        if not segment:
            raise ValueError("unknown speech segment")
        presence = str(payload.get("speech_present", "")).strip().lower()
        transcript = str(payload.get("transcript_raw", "")).strip()
        language = str(payload.get("transcript_language", "")).strip().lower()
        unintelligible = bool(payload.get("unintelligible"))
        if presence not in {"yes", "no"}:
            raise ValueError("speech presence must be yes or no")
        if presence == "no":
            transcript, language, unintelligible = "", "not_applicable", False
        elif not unintelligible and (not transcript or language not in {"urdu", "english", "mixed", "other"}):
            raise ValueError("intelligible speech requires raw verbatim text and language")
        elif unintelligible and language not in {"urdu", "english", "mixed", "other", "unknown"}:
            raise ValueError("unintelligible speech requires a language or unknown")
        segment.update({
            "speech_present": presence, "transcript_raw": transcript,
            "transcript_language": language, "unintelligible": unintelligible,
            "review_status": "complete", "reviewer": self.state["reviewer"],
            "reviewed_at": utc_now(), "review_notes": str(payload.get("review_notes", "")).strip()[:1000],
        })

    def validate(self) -> dict[str, Any]:
        errors: list[str] = []
        if not self.state["reviewer"]:
            errors.append("reviewer is not set")
        for split in ("development", "sealed_holdout"):
            expected = 10 if split == "development" else 22
            ids = self.state["active"][split]
            if len(ids) != expected:
                errors.append(f"{split} must contain exactly {expected} active images")
            for sample_id in ids:
                if self.state["independence"].get(sample_id) != "unseen":
                    errors.append(f"{sample_id}: independence is not confirmed")
                if sample_id not in self.state["image_labels"]:
                    errors.append(f"{sample_id}: image label is incomplete")
        if not self.state["development_frozen"]:
            errors.append("development labels are not frozen")
        if not self.state["video_review"].get("watched_full_source"):
            errors.append("full 60.010-second video review is not attested")
        if not self.state["video_events"]:
            errors.append("video has no reviewed plate or negative interval")
        if not self.state["speech_segments"]:
            errors.append("audio-energy speech candidates have not been generated")
        for segment in self.state["speech_segments"]:
            if segment.get("review_status") != "complete":
                errors.append(f"{segment['segment_id']}: speech review is incomplete")
        status = "PASS" if not errors and self.state.get("locked") else "READY_TO_LOCK" if not errors else "PENDING_OR_INVALID"
        return {
            "contract_version": "nexusai.nxmmr.ground-truth-validation/v2",
            "status": status, "GroundTruthValidation": "PASS" if status == "PASS" else "PENDING",
            "errors": errors, "active_images": 32,
            "development_images": len(self.state["active"]["development"]),
            "holdout_images": len(self.state["active"]["sealed_holdout"]),
            "excluded_exposed": len(self.state["excluded_exposed"]),
            "video_events": len(self.state["video_events"]),
            "speech_segments": len(self.state["speech_segments"]),
            "label_source": "independent_human_annotation",
            "model_output_used_to_create_truth": False,
        }

    def _finish(self) -> None:
        self._assert_mutable()
        validation = self.validate()
        if validation["errors"]:
            raise ValueError("cannot finish: " + "; ".join(validation["errors"][:5]))
        self.state["locked"] = True
        self.state["locked_at"] = utc_now()
        self.state["gold_digest"] = canonical_digest({
            "active": self.state["active"], "labels": self.state["image_labels"],
            "video": self.state["video_events"], "speech": self.state["speech_segments"],
            "source_binding": self.state["source_binding"],
        })

    def _write_outputs(self) -> None:
        image_rows = []
        for split in ("development", "sealed_holdout"):
            for sample_id in self.state["active"][split]:
                source = self.by_id[sample_id]
                label = self.state["image_labels"].get(sample_id, {})
                image_rows.append({
                    **{key: source.get(key, "") for key in ("sample_id", "relative_file", "size_bytes", "sha256", "width", "height")},
                    "split": split, **label,
                })
        atomic_csv(self.output_dir / "image-labels.csv", IMAGE_FIELDS, image_rows)
        atomic_csv(self.output_dir / "video-events.csv", VIDEO_FIELDS, self.state["video_events"])
        atomic_csv(self.output_dir / "speech-labels.csv", SPEECH_FIELDS, self.state["speech_segments"])
        excluded_fields = ["sample_id", "original_split", "reason", "replacement_sample_id", "recorded_at", "reviewer"]
        atomic_csv(self.output_dir / "excluded-exposed-images.csv", excluded_fields, self.state["excluded_exposed"])
        validation = self.validate()
        if self.state.get("locked") and not validation["errors"]:
            validation["status"] = "PASS"
            validation["GroundTruthValidation"] = "PASS"
            validation["gold_digest"] = self.state["gold_digest"]
            validation["locked_at"] = self.state["locked_at"]
        atomic_json(self.output_dir / "ground-truth-validation.json", validation)


class VerificationHandler(BaseHTTPRequestHandler):
    server_version = "NXMMRHumanVerification/1"

    @property
    def app(self) -> "VerificationServer":
        return self.server  # type: ignore[return-value]

    def log_message(self, fmt: str, *args: Any) -> None:
        sys.stderr.write("[nxmmr-review] " + fmt % args + "\n")

    def _headers(self, status: int, content_type: str, length: int | None = None) -> None:
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        self.send_header("X-Frame-Options", "DENY")
        self.send_header("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; media-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'")
        if length is not None:
            self.send_header("Content-Length", str(length))
        self.end_headers()

    def _json(self, value: Any, status: int = HTTPStatus.OK) -> None:
        body = json.dumps(value, ensure_ascii=False).encode("utf-8")
        self._headers(status, "application/json; charset=utf-8", len(body))
        self.wfile.write(body)

    def do_GET(self) -> None:  # noqa: N802
        path = urlparse(self.path).path
        if path == "/api/state":
            self._json(self.app.store.public_state())
            return
        if path.startswith("/media/image/"):
            sample_id = path.rsplit("/", 1)[-1]
            if not re.fullmatch(r"PKPLATE-\d{4}", sample_id):
                self.send_error(HTTPStatus.NOT_FOUND)
                return
            try:
                self._serve_file(self.app.store.image_path(sample_id), allow_range=False)
            except (KeyError, PermissionError, FileNotFoundError):
                self.send_error(HTTPStatus.NOT_FOUND)
            return
        if path == "/media/video":
            self._serve_file(self.app.store.video, allow_range=True)
            return
        if path == "/media/audio":
            self._serve_file(self.app.store.audio, allow_range=True)
            return
        assets = {"/": "index.html", "/index.html": "index.html", "/app.js": "app.js", "/styles.css": "styles.css"}
        if path in assets:
            self._serve_file(self.app.assets / assets[path], allow_range=False)
            return
        self.send_error(HTTPStatus.NOT_FOUND)

    def _serve_file(self, path: Path, *, allow_range: bool) -> None:
        size = path.stat().st_size
        content_type = mimetypes.guess_type(path.name)[0] or "application/octet-stream"
        start, end = 0, size - 1
        range_header = self.headers.get("Range") if allow_range else None
        status = HTTPStatus.OK
        if range_header:
            match = re.fullmatch(r"bytes=(\d*)-(\d*)", range_header.strip())
            if not match:
                self.send_error(HTTPStatus.REQUESTED_RANGE_NOT_SATISFIABLE)
                return
            if match.group(1):
                start = int(match.group(1))
                end = min(int(match.group(2)) if match.group(2) else size - 1, size - 1)
            elif match.group(2):
                length = min(int(match.group(2)), size)
                start, end = size - length, size - 1
            if start > end or start >= size:
                self.send_error(HTTPStatus.REQUESTED_RANGE_NOT_SATISFIABLE)
                return
            status = HTTPStatus.PARTIAL_CONTENT
        length = end - start + 1
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(length))
        self.send_header("Cache-Control", "no-store")
        self.send_header("X-Content-Type-Options", "nosniff")
        if allow_range:
            self.send_header("Accept-Ranges", "bytes")
        if status == HTTPStatus.PARTIAL_CONTENT:
            self.send_header("Content-Range", f"bytes {start}-{end}/{size}")
        self.end_headers()
        with path.open("rb") as source:
            source.seek(start)
            remaining = length
            while remaining:
                block = source.read(min(1024 * 1024, remaining))
                if not block:
                    break
                try:
                    self.wfile.write(block)
                except (BrokenPipeError, ConnectionAbortedError, ConnectionResetError):
                    break
                remaining -= len(block)

    def do_POST(self) -> None:  # noqa: N802
        if urlparse(self.path).path != "/api/action":
            self.send_error(HTTPStatus.NOT_FOUND)
            return
        if self.headers.get("X-NXMMR-Token") != self.app.token:
            self._json({"error": "invalid local session token"}, HTTPStatus.FORBIDDEN)
            return
        try:
            length = int(self.headers.get("Content-Length", "0"))
            if length < 2 or length > 1024 * 1024:
                raise ValueError("request size is invalid")
            payload = json.loads(self.rfile.read(length))
            self.app.store.action(payload)
            self._json(self.app.store.public_state())
        except (ValueError, KeyError, TypeError, json.JSONDecodeError) as error:
            self._json({"error": str(error)}, HTTPStatus.BAD_REQUEST)


class VerificationServer(ThreadingHTTPServer):
    def __init__(self, address: tuple[str, int], store: ReviewStore, assets: Path, token: str) -> None:
        super().__init__(address, VerificationHandler)
        self.store = store
        self.assets = assets.resolve()
        self.token = token


def parse_args() -> argparse.Namespace:
    repo_root = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image-root", type=Path, required=True)
    parser.add_argument("--video", type=Path, required=True)
    parser.add_argument("--private-root", type=Path, default=repo_root / PRIVATE_RELATIVE)
    parser.add_argument("--port", type=int, default=0)
    parser.add_argument("--no-browser", action="store_true")
    parser.add_argument("--skip-hash-check", action="store_true", help=argparse.SUPPRESS)
    return parser.parse_args()


def main() -> None:
    args = parse_args()
    repo_root = Path(__file__).resolve().parents[1]
    store = ReviewStore(
        repo_root=repo_root, image_root=args.image_root, video=args.video,
        private_root=args.private_root, verify_hashes=not args.skip_hash_check,
    )
    assets = repo_root / "tools/nxmmr-human-verification"
    if not assets.is_dir():
        raise SystemExit(f"review UI assets are missing: {assets}")
    token = secrets.token_urlsafe(32)
    server = VerificationServer(("127.0.0.1", args.port), store, assets, token)
    port = server.server_address[1]
    url = f"http://127.0.0.1:{port}/?token={token}"
    print("NX-MMR local human verification is ready.", flush=True)
    print(f"Open: {url}", flush=True)
    print(f"Writes only: {store.output_dir}", flush=True)
    print("Mode: GOLD_LABEL_ONLY (no model inference or product persistence)", flush=True)
    if not args.no_browser:
        webbrowser.open(url)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()


if __name__ == "__main__":
    main()
