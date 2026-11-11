#!/usr/bin/env python3
"""Patch of the production worker image's own media_pipeline.py for the video plate processor V4 (docs/data/video-plates-research-and-preregistration-20261111.md).

    python patch_image_media_pipeline_video_v4.py <image media_pipeline.py> <output>

The image's file is an older build than the repository's (its video function samples a few frames and runs the image path on them; there is no video plate grouping), so the repository's file is never
copied over it. This patch makes four edits, each anchored on text that must occur EXACTLY ONCE, else nothing is written and the exit code is 1:
 1. DeterministicMediaProcessor.__init__ takes a video plate processor and a processor for the sampled frames (text recognition and the like, without the plate reader);
 2. _process_video runs the video plate processor when there is one and puts its observations into the result;
 3. the sampled-frame loop uses the frame processor (the plate reader reads video through V4 only, so no frame is read twice);
 4. configured_media_processor builds V4 when FORENSIC_VIDEO_ANPR_V4_ENABLED is true (default false).
With the switch off the behaviour of the image is unchanged.
"""
import sys

path_in, path_out = sys.argv[1], sys.argv[2]
text = open(path_in, "rb").read().decode("utf-8")
if "\r\n" in text:
    raise SystemExit("the image file has CRLF line endings; this patch expects LF")


def replace(old: str, new: str) -> None:
    global text
    if text.count(old) != 1:
        raise SystemExit(f"anchor found {text.count(old)} times, expected once: {old[:80]!r}")
    text = text.replace(old, new)


replace('''    def __init__(self, image_processor: ImageProcessor | None = None, audio_transcriber: AudioTranscriber | None = None) -> None:
        self.image_processor = image_processor
        self.audio_transcriber = audio_transcriber
''', '''    def __init__(self, image_processor: ImageProcessor | None = None, audio_transcriber: AudioTranscriber | None = None, video_anpr_processor: Any | None = None,
                 video_frame_processor: ImageProcessor | None = None) -> None:
        self.image_processor = image_processor
        self.audio_transcriber = audio_transcriber
        self.video_anpr_processor = video_anpr_processor      # the video plate processor (V4); None keeps the earlier behaviour
        self.video_frame_processor = video_frame_processor    # what reads the sampled frames when V4 reads the plates: text recognition and the like, never the plate reader
''')

replace('''        frames = _sample_video_frames(path)
        if not frames:
            limitations.append("no frames were sampled; ffmpeg may be unavailable or the video may have no decodable video stream")
        try:
            for frame_path, timestamp in frames:''', '''        video_anpr_result = None
        if self.video_anpr_processor is not None:
            video_anpr_result = self.video_anpr_processor.process_video(path, **context)
            observations.extend(video_anpr_result.observations)
            limitations.extend(video_anpr_result.limitations)
        frame_processor = self.video_frame_processor if self.video_anpr_processor is not None else self.image_processor
        frames = _sample_video_frames(path)
        if not frames:
            limitations.append("no frames were sampled; ffmpeg may be unavailable or the video may have no decodable video stream")
        try:
            for frame_path, timestamp in frames:''')

replace('''                if self.image_processor is not None:
                    frame_result = self.image_processor.process_image(
                        frame_path,
                        evidence_id=context["evidence_id"],
                        version_id=context["version_id"],
                        source_sha256=context["source_sha256"],
                        source_file=source_file,
                        locator_prefix={"timestamp_seconds": timestamp},
                    )
                    observations.extend(frame_result.observations)
                    limitations.extend(frame_result.limitations)
                else:
                    limitations.append("ANPR analysis is unavailable without a configured image processor")''', '''                if frame_processor is not None:
                    frame_result = frame_processor.process_image(
                        frame_path,
                        evidence_id=context["evidence_id"],
                        version_id=context["version_id"],
                        source_sha256=context["source_sha256"],
                        source_file=source_file,
                        locator_prefix={"timestamp_seconds": timestamp},
                    )
                    observations.extend(frame_result.observations)
                    limitations.extend(frame_result.limitations)
                elif video_anpr_result is None:
                    limitations.append("ANPR analysis is unavailable without a configured image processor")''')

replace('''            metadata={"sampled_frame_count": len(frames)},
        )
''', '''            metadata={"sampled_frame_count": len(frames), **({"result_states": video_anpr_result.metadata.get("result_states", {}), "video_anpr_v4": video_anpr_result.metadata} if video_anpr_result is not None else {})},
        )
''')

replace('''    return DeterministicMediaProcessor(image_processor, audio_transcriber)
''', '''    video_anpr_processor = None
    video_frame_processor = None
    if os.getenv("FORENSIC_VIDEO_ANPR_V4_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:
        from video_anpr_nexus_v4 import VideoANPRNexusV4Processor
        video_anpr_processor = VideoANPRNexusV4Processor.from_environment()
        frame_readers = [item for item in image_processors if not isinstance(item, FastALPRImageProcessor)]   # the plate reader reads video through V4 only
        video_frame_processor = (frame_readers[0] if len(frame_readers) == 1 else CompositeImageProcessor(tuple(frame_readers))) if frame_readers else None
    return DeterministicMediaProcessor(image_processor, audio_transcriber, video_anpr_processor, video_frame_processor)
''')

open(path_out, "wb").write(text.encode("utf-8"))
print("patched", path_out)
