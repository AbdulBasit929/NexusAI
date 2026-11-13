#!/bin/bash
# Speech lab dev baselines, in the order of section 7 of docs/data/speech-lab-preregistration-20261112.md. ONE evaluation container at a time (--cpus 8, -m 5g, no network), and nothing else
# may run on the machine while it runs (the real-time factor is a result). Re-running skips nothing: a result file is written per run; delete a run's file to repeat it on purpose.
#
#   bash tools/media_data/run_speech_baseline.sh convert        # the Pashto model to CTranslate2 int8 (once)
#   bash tools/media_data/run_speech_baseline.sh smoke          # 3 clips, to prove the mounts and the result file
#   bash tools/media_data/run_speech_baseline.sh dev            # the whole dev baseline (about 4 hours)
#   bash tools/media_data/run_speech_baseline.sh sealed-ps      # sealed, Pashto and nothing else: only after the dev decision, once
set -u
DATA=${NEXUSAI_MEDIA_DATA_HOST:-C:/tmp/nexusai-media-data}
REPO=$(cd "$(dirname "$0")/../.." && pwd -W)
IMAGE=nexusai-media-eval-asr:latest
TURBO=faster-whisper-large-v3-turbo-ct2
PS=ps-small-cv24-ct2-int8

docker_run() { MSYS_NO_PATHCONV=1 docker run --rm --network none --cpus 8 -m 5g -v "$DATA:/data" -v "$REPO:/repo:ro" --entrypoint python "$IMAGE" "$@"; }
lab() { echo "== $(date '+%H:%M:%S') speech_lab $*"; docker_run /repo/tools/media_data/speech_lab.py "$@" || echo "FAILED: $*"; }

case "${1:-}" in
  convert)
    if [ -f "$DATA/models/asr/$PS/model.bin" ]; then echo "already converted"; exit 0; fi
    docker_run -m ctranslate2.converters.transformers --model /data/models/speech/exp_009_small_cv24_vanilla --output_dir /data/models/asr/$PS --quantization int8 --copy_files tokenizer.json preprocessor_config.json
    ;;
  smoke)
    lab --set ps_af --partition dev --limit 3 --model $PS --told ps --label smoke
    ;;
  dev)
    lab --set ps_af --partition dev --limit 40 --model $TURBO --told ps
    lab --set ps_af --partition dev --limit 40 --model $PS --told ps
    for s in pst_Arab pbt_Arab pbu_Arab; do
      lab --set $s --partition dev --limit 40 --model $PS --told ps
      lab --set $s --partition dev --limit 40 --model $TURBO --told ps
    done
    lab --set pnb_Arab --partition dev --limit 40 --model $TURBO --told ur
    lab --set pnb_Arab --partition dev --limit 40 --model $TURBO --told pa
    lab --set ur_pk --partition dev --limit 60 --model $TURBO --told ur
    lab --set en_us --partition dev --limit 30 --model $TURBO --told en
    echo "== $(date '+%H:%M:%S') DEV-DONE"
    ;;
  sealed-ps)
    for s in ps_af pst_Arab pbt_Arab pbu_Arab; do lab --set $s --partition sealed --limit 40 --model "${SEALED_MODEL:?set SEALED_MODEL to the model chosen on dev}" --told ps; done
    ;;
  *) echo "usage: $0 convert|smoke|dev|sealed-ps"; exit 2 ;;
esac
