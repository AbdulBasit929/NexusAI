#!/usr/bin/env python3
"""Direct smoke check for source-timestamp video sampling and cleanup."""

from pathlib import Path

from media_pipeline import _sample_video_frames, cleanup_sampled_frames


frames = _sample_video_frames(Path("/tmp/mmv2-sample.mp4"), interval=5, max_frames=6)
print([(round(timestamp, 6), path.stat().st_size) for path, timestamp in frames])
assert [round(timestamp) for _, timestamp in frames] == [0, 5, 10, 15, 20, 25]
cleanup_sampled_frames(frames)
assert all(not path.exists() for path, _ in frames)
print("patched-sampler-smoke: PASS cleanup: PASS")
