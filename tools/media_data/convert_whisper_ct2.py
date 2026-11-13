#!/usr/bin/env python3
"""Converts a Hugging Face Whisper checkpoint to CTranslate2 (int8) inside the evaluation image.

    docker run --rm --network none -v C:/tmp/nexusai-media-data:/data -v <repo>:/repo:ro --entrypoint python nexusai-media-eval-asr:latest \
        /repo/tools/media_data/convert_whisper_ct2.py /data/models/speech/exp_009_small_cv24_vanilla /data/models/asr/ps-small-cv24-ct2-int8

Why a script and not `ct2-transformers-converter`: the evaluation image carries transformers 4.49.0 (the worker's runtime) and ctranslate2 4.8.3, whose converter loads the model with the keyword
`dtype`, which transformers 4.49 does not know (it reads that keyword as a model argument and fails with "unexpected keyword argument 'dtype'"). The loader is wrapped here to pass `torch_dtype`
instead; nothing else changes and no package is installed. The tokenizer configuration of a checkpoint saved by a newer transformers lists its extra special tokens as a list (the 107 standard Whisper
tokens, all also in tokenizer.json), which transformers 4.49 cannot read: the conversion reads a staging folder (symbolic links to the source files, a copy of tokenizer_config.json without that list),
and the source folder is never changed. The output folder gets a `SOURCE.txt` with the source folder, the hash of the converted `model.bin` and the versions used.
"""
import hashlib
import json
import os
import sys
import tempfile

import transformers

_original = transformers.PreTrainedModel.from_pretrained.__func__


def _from_pretrained(cls, *args, **kwargs):
    if "dtype" in kwargs:
        kwargs["torch_dtype"] = kwargs.pop("dtype")
    return _original(cls, *args, **kwargs)


transformers.PreTrainedModel.from_pretrained = classmethod(_from_pretrained)

import ctranslate2  # noqa: E402
from ctranslate2.converters import TransformersConverter  # noqa: E402


def main(argv):
    if len(argv) != 2:
        raise SystemExit(__doc__)
    source, target = argv
    if os.path.exists(os.path.join(target, "model.bin")):
        print("already converted:", target)
        return 0
    with tempfile.TemporaryDirectory() as staging:
        for name in os.listdir(source):
            if name == "tokenizer_config.json":
                with open(os.path.join(source, name), encoding="utf-8") as handle:
                    config = json.load(handle)
                if isinstance(config.get("extra_special_tokens"), list):
                    config.pop("extra_special_tokens")
                with open(os.path.join(staging, name), "w", encoding="utf-8") as handle:
                    json.dump(config, handle)
            else:
                os.symlink(os.path.join(source, name), os.path.join(staging, name))
        TransformersConverter(staging, copy_files=["tokenizer.json", "preprocessor_config.json"]).convert(target, quantization="int8")
    digest = hashlib.sha256()
    with open(os.path.join(target, "model.bin"), "rb") as handle:
        for block in iter(lambda: handle.read(1 << 20), b""):
            digest.update(block)
    with open(os.path.join(target, "SOURCE.txt"), "w", encoding="utf-8") as handle:
        handle.write("converted from %s to CTranslate2 int8 with ctranslate2 %s and transformers %s\nmodel.bin sha256 %s\n" % (source, ctranslate2.__version__, transformers.__version__, digest.hexdigest()))
    print("converted; model.bin sha256", digest.hexdigest())
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
