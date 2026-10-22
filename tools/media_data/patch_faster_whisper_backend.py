#!/usr/bin/env python3
"""Make the faster-whisper backend's beam size and thread count configurable per model (media activation, track A).

    python -X utf8 tools/media_data/patch_faster_whisper_backend.py backend/python/faster-whisper/backend.py

Why: the backend hard-codes beam_size=5 and never passes cpu_threads, so a model installed in LocalAI always decodes with beam 5 on CTranslate2's default thread count. The speech bake-off
(docs/data/media-activation-results-20261021.md) measured large-v3-turbo on Urdu at word error 20.8% / real-time factor 1.74 with beam 5 and 21.2% / 1.49 with beam 1 on 8 threads; the pre-registered
ceiling is 1.5, so the choice needs beam 1 and 8 threads. A model's YAML now sets them with `options: [beam_size:1, cpu_threads:8]` (LocalAI passes `options` as ModelOptions.Options), or the
environment (FASTER_WHISPER_BEAM_SIZE, FASTER_WHISPER_CPU_THREADS); the YAML's `threads:` is deliberately not used (LocalAI fills it with the CPU count, and 12 threads were slower than 8). Without any of them nothing changes: beam 5 and
CTranslate2's default threads, exactly as before. Out-of-range or unreadable values fall back to the default instead of failing the model load.

The file has mixed line endings (the upstream part is CRLF, the project's additions are LF); each edit below touches LF lines only, matches exactly once, and the rest of the file is kept byte for byte.
"""
import sys

EDITS = [
    (
        b'    return local_model if os.path.isdir(local_model) else model\n',
        b'    return local_model if os.path.isdir(local_model) else model\n'
        b'\n'
        b'\n'
        b'def parse_model_options(options):\n'
        b'    """The `options:` of a LocalAI model YAML arrive as \'key:value\' strings."""\n'
        b'\n'
        b'    parsed = {}\n'
        b'    for item in options or []:\n'
        b'        key, _, value = str(item).partition(":")\n'
        b'        if key.strip():\n'
        b'            parsed[key.strip().lower()] = value.strip()\n'
        b'    return parsed\n'
        b'\n'
        b'\n'
        b'def bounded_int(value, default, low, high):\n'
        b'    """An integer inside [low, high], or the default when the value is missing, unreadable or out of range."""\n'
        b'\n'
        b'    try:\n'
        b'        number = int(str(value).strip())\n'
        b'    except (TypeError, ValueError):\n'
        b'        return default\n'
        b'    return number if low <= number <= high else default\n',
    ),
    (
        b'        try:\n'
        b'            print("Preparing models, please wait", file=sys.stderr)\n'
        b'            self.model = WhisperModel(\n'
        b'                resolve_local_model_reference(request.Model),\n'
        b'                device=device,\n'
        b'                compute_type=compute_type,\n'
        b'            )\n',
        b'        try:\n'
        b'            options = parse_model_options(getattr(request, "Options", None))\n'
        b'            self.beam_size = bounded_int(options.get("beam_size", os.environ.get("FASTER_WHISPER_BEAM_SIZE")), 5, 1, 10)\n'
        b'            cpu_threads = bounded_int(options.get("cpu_threads", os.environ.get("FASTER_WHISPER_CPU_THREADS")), 0, 0, 32)\n'
        b'            print(f"Preparing models, please wait (beam_size={self.beam_size}, cpu_threads={cpu_threads})", file=sys.stderr)\n'
        b'            self.model = WhisperModel(\n'
        b'                resolve_local_model_reference(request.Model),\n'
        b'                device=device,\n'
        b'                compute_type=compute_type,\n'
        b'                cpu_threads=cpu_threads,\n'
        b'            )\n',
    ),
    (
        b'                beam_size=5,\n',
        b'                beam_size=getattr(self, "beam_size", 5),\n',
    ),
]


def main(path):
    data = open(path, "rb").read()
    for old, new in EDITS:
        found = data.count(old)
        if found != 1:
            sys.exit(f"expected exactly one match, found {found}: {old[:70]!r}")
        data = data.replace(old, new)
    open(path, "wb").write(data)
    compile(data.replace(b"\r\n", b"\n"), path, "exec")
    print(f"patched {path} ({len(EDITS)} edits)")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        sys.exit(__doc__)
    main(sys.argv[1])
