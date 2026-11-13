#!/bin/bash
# Speech lab dev baselines, in the order of section 7 of docs/data/speech-lab-preregistration-20261112.md. ONE evaluation container at a time (--cpus 8, -m 5g, no network), and nothing else
# may run on the machine while it runs (the real-time factor is a result). Re-running skips nothing: a result file is written per run; delete a run's file to repeat it on purpose.
#
#   bash tools/media_data/run_speech_baseline.sh convert        # the Pashto model to CTranslate2 int8 (once)
#   bash tools/media_data/run_speech_baseline.sh omni-convert   # the Omnilingual CTC 300M checkpoint to a standard wav2vec2 folder (own converter)
#   bash tools/media_data/run_speech_baseline.sh smoke          # 3 clips, to prove the mounts and the result file
#   bash tools/media_data/run_speech_baseline.sh dev            # the whole dev baseline (about 4 hours)
#   bash tools/media_data/run_speech_baseline.sh dev-chunk      # the one correction round for the Pashto fine-tune (amendment A5), about 10 minutes (two sets; amendment A6)
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
    docker_run /repo/tools/media_data/convert_whisper_ct2.py /data/models/speech/exp_009_small_cv24_vanilla /data/models/asr/$PS
    ;;
  omni-convert)
    # the Omnilingual CTC 300M checkpoint to a Wav2Vec2ForCTC folder with tools/media_data/omni_ctc.py (own converter; no package installed, no network)
    docker_run /repo/tools/media_data/omni_ctc.py describe /data/models/speech/omniASR-CTC-300M/omniASR-CTC-300M.pt
    docker_run /repo/tools/media_data/omni_ctc.py convert /data/models/speech/omniASR-CTC-300M/omniASR-CTC-300M.pt /data/models/speech/omniASR-CTC-300M/omniASR_tokenizer.model /data/models/asr/omni-ctc-300m-hf
    ;;
  lab)
    shift
    lab "$@"
    ;;
  smoke)
    lab --set ps_af --partition dev --limit 3 --model $PS --told ps --label smoke --no-timestamps
    ;;
  dev)
    # amendment A2 of the pre-registration (2026-11-13): only the runs that can change a decision, about 1.5 hours
    lab --set ps_af --partition dev --limit 40 --model $PS --told ps --no-timestamps
    for s in pst_Arab pbt_Arab pbu_Arab; do
      lab --set $s --partition dev --limit 40 --model $PS --told ps --no-timestamps
    done
    lab --set ps_af --partition dev --limit 40 --model $TURBO --told ps
    lab --set pst_Arab --partition dev --limit 20 --model $TURBO --told ps
    lab --set pnb_Arab --partition dev --limit 20 --model $TURBO --told ur
    lab --set pnb_Arab --partition dev --limit 5 --model $TURBO --told pa --label script-check
    echo "== $(date '+%H:%M:%S') DEV-DONE"
    ;;
  dev-omni)
    # amendment A7 (2026-11-13): the Omnilingual CTC model on the dev sets, sample sizes reduced to what decides, about 40 minutes
    O=omni-ctc-300m-hf
    lab --backend omni --set pst_Arab --partition dev --limit 40 --model $O --told ps
    lab --backend omni --set pbt_Arab --partition dev --limit 20 --model $O --told ps
    lab --backend omni --set pbu_Arab --partition dev --limit 20 --model $O --told ps
    lab --backend omni --set pnb_Arab --partition dev --limit 20 --model $O --told pnb
    lab --backend omni --set ur_pk --partition dev --limit 40 --model $O --told ur
    lab --set ur_pk --partition dev --limit 40 --model $TURBO --told ur
    echo "== $(date '+%H:%M:%S') DEV-OMNI-DONE"
    ;;
  dev-omni-urdu)
    # amendment A8 (2026-11-13): Urdu with the output held to the Arabic script, 3 minutes
    lab --backend omni --set ur_pk --partition dev --limit 40 --model omni-ctc-300m-hf --told ur --script arabic --label arabic
    ;;
  sealed-omni)
    # amendment A9 (2026-11-13): the one sealed read for Pashto with the Omnilingual model (the dev run on the read set first), about 26 minutes
    O=omni-ctc-300m-hf
    lab --backend omni --set ps_af --partition dev --limit 40 --model $O --told ps
    for s in ps_af pst_Arab pbt_Arab pbu_Arab; do
      lab --backend omni --set $s --partition sealed --limit 40 --model $O --told ps
    done
    echo "== $(date '+%H:%M:%S') SEALED-OMNI-DONE"
    ;;
  dev-chunk)
    # amendment A5 (2026-11-13): the one correction round for the Pashto fine-tune: short pieces cut at the quiet points
    for s in ps_af pst_Arab; do
      lab --set $s --partition dev --limit 40 --model $PS --told ps --no-timestamps --chunk-seconds 12 --label chunk12
    done
    echo "== $(date '+%H:%M:%S') DEV-CHUNK-DONE"
    ;;
  sealed-ps)
    for s in ps_af pst_Arab pbt_Arab pbu_Arab; do lab --set $s --partition sealed --limit 40 --model "${SEALED_MODEL:?set SEALED_MODEL to the model chosen on dev}" --told ps ${SEALED_FLAGS:-}; done
    ;;
  *) echo "usage: $0 convert|smoke|dev|sealed-ps"; exit 2 ;;
esac
