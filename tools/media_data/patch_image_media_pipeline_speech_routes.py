#!/usr/bin/env python3
"""Patch a media_pipeline.py (the repository's, or the RUNNING worker image's own) so that the ASR client is wrapped by the speech routes when FORENSIC_ASR_ROUTES_ENABLED is on.

    python -X utf8 patch_image_media_pipeline_speech_routes.py <media_pipeline.py> <output path>

One edit: after the line that builds the ASR client, `audio_transcriber = LocalAIASRProcessor.from_environment()`, four lines wrap it in speech_routes.RoutedSpeechTranscriber when the switch is on
(default off, so the file behaves as before unless the compose file turns it on). The anchor must occur exactly once or nothing is written; the line endings of the file are kept. The image's own
file is an older build than the repository's (it has the long-recording and Hindi rules patched in by patch_image_media_pipeline.py); both carry this one line.
"""
import re
import sys

ANCHOR = re.compile(r"(?P<indent>[ ]+)audio_transcriber = LocalAIASRProcessor\.from_environment\(\)(?P<eol>\r?\n)")


def patch(text):
    if "RoutedSpeechTranscriber" in text:
        raise SystemExit("already patched; nothing written")
    matches = list(ANCHOR.finditer(text))
    if len(matches) != 1:
        raise SystemExit("anchor occurs %d times (expected 1); nothing written" % len(matches))
    m = matches[0]
    i, eol = m.group("indent"), m.group("eol")
    lines = [
        m.group(0).rstrip("\r\n"),
        i + 'if os.getenv("FORENSIC_ASR_ROUTES_ENABLED", "false").strip().lower() in {"1", "true", "yes", "on"}:',
        i + "    try:",
        i + "        from ingestion.forensic_records.speech_routes import RoutedSpeechTranscriber",
        i + "    except ModuleNotFoundError:  # pragma: no cover - supports direct worker image execution",
        i + "        from speech_routes import RoutedSpeechTranscriber",
        i + "    audio_transcriber = RoutedSpeechTranscriber(audio_transcriber)",
    ]
    return text[: m.start()] + eol.join(lines) + eol + text[m.end():]


def main(argv):
    if len(argv) != 2:
        raise SystemExit(__doc__)
    text = open(argv[0], encoding="utf-8", newline="").read()
    patched = patch(text)
    open(argv[1], "w", encoding="utf-8", newline="").write(patched)
    print("patched %s -> %s (7 lines for 1)" % (argv[0], argv[1]))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
