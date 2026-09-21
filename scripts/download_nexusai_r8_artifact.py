#!/usr/bin/env python3
"""Resumable HTTPS downloader for pinned NexusAI R8 evaluation artifacts."""

from __future__ import annotations

import argparse
import json
import sys
import time
import urllib.error
import urllib.request
from pathlib import Path


def download(
    url: str,
    destination: Path,
    expected_size: int,
    attempts: int = 12,
    read_timeout_seconds: int = 60,
    max_retry_delay_seconds: int = 15,
    progress_interval_mib: int = 64,
) -> dict:
    destination.parent.mkdir(parents=True, exist_ok=True)
    last_error: Exception | None = None
    for attempt in range(1, attempts + 1):
        current = destination.stat().st_size if destination.exists() else 0
        if current == expected_size:
            return {"status": "pass", "bytes": current, "attempts": attempt - 1}
        if current > expected_size:
            raise RuntimeError(f"partial file exceeds expected size: {current}>{expected_size}")
        request = urllib.request.Request(url, headers={"User-Agent": "NexusAI-R8-Evaluator/1.0"})
        if current:
            request.add_header("Range", f"bytes={current}-")
        try:
            with urllib.request.urlopen(request, timeout=read_timeout_seconds) as response:
                status = getattr(response, "status", response.getcode())
                if current and status != 206:
                    raise RuntimeError(f"server ignored resume range at byte {current}; HTTP {status}")
                if current:
                    content_range = response.headers.get("Content-Range", "")
                    expected_prefix = f"bytes {current}-"
                    if not content_range.startswith(expected_prefix) or not content_range.endswith(f"/{expected_size}"):
                        raise RuntimeError(f"server returned unsafe resume range: {content_range!r}")
                mode = "ab" if current else "wb"
                next_progress = current + progress_interval_mib * 1024 * 1024
                with destination.open(mode) as stream:
                    while True:
                        chunk = response.read(1024 * 1024)
                        if not chunk:
                            break
                        stream.write(chunk)
                        current += len(chunk)
                        if current >= next_progress:
                            print(json.dumps({
                                "status": "progress", "attempt": attempt, "bytes": current,
                                "expected_size": expected_size,
                                "percent": round(100.0 * current / expected_size, 3),
                            }, sort_keys=True), flush=True)
                            next_progress = current + progress_interval_mib * 1024 * 1024
            final_size = destination.stat().st_size
            if final_size == expected_size:
                return {"status": "pass", "bytes": final_size, "attempts": attempt}
            last_error = RuntimeError(f"incomplete response: {final_size}/{expected_size}")
        except (OSError, TimeoutError, urllib.error.URLError, RuntimeError) as exc:
            last_error = exc
        if attempt < attempts:
            delay = min(3 * attempt, max_retry_delay_seconds)
            print(json.dumps({
                "status": "retry", "attempt": attempt, "resume_byte": destination.stat().st_size if destination.exists() else 0,
                "delay_seconds": delay, "reason": str(last_error),
            }, sort_keys=True), file=sys.stderr, flush=True)
            time.sleep(delay)
    raise RuntimeError(f"download did not complete after {attempts} attempts: {last_error}")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", required=True)
    parser.add_argument("--destination", type=Path, required=True)
    parser.add_argument("--expected-size", type=int, required=True)
    parser.add_argument("--attempts", type=int, default=12)
    parser.add_argument("--read-timeout-seconds", type=int, default=60)
    parser.add_argument("--max-retry-delay-seconds", type=int, default=15)
    parser.add_argument("--progress-interval-mib", type=int, default=64)
    args = parser.parse_args()
    if args.attempts < 1 or args.read_timeout_seconds < 1 or args.max_retry_delay_seconds < 1 or args.progress_interval_mib < 1:
        parser.error("attempt, timeout, delay and progress values must be positive")
    print(json.dumps(download(
        args.url, args.destination, args.expected_size, attempts=args.attempts,
        read_timeout_seconds=args.read_timeout_seconds,
        max_retry_delay_seconds=args.max_retry_delay_seconds,
        progress_interval_mib=args.progress_interval_mib,
    ), sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
