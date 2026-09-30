#!/bin/sh
# Resumable, checksum-verified model downloads into /models (the LocalAI models volume).
#
# Safe to run again at any time: verified files are skipped, partial files resume from their last
# byte, and only one copy runs at once. It survives Wi-Fi drops and laptop sleep: a stalled
# transfer (under 20 KB/s for 90 s) is abandoned and resumed, and every failure retries after
# 20 s, forever, until the SHA-256 matches the publisher's value.
#
#   docker exec -d nexusai-api-1 sh /models/tools/resumable_model_download.sh
#   docker exec nexusai-api-1 tail -n 20 /models/downloads.log      # progress
#
# Source: bartowski on Hugging Face. The deployed Q4_K_M model is byte-identical to that
# publisher's file (SHA-256 2fde00ce..., checked 2026-09-29).

LOG=/models/downloads.log
LOCK=/models/.download.lock

if command -v flock >/dev/null 2>&1; then
  exec 9>"$LOCK"
  flock -n 9 || { echo "$(date -u +%FT%TZ) another download is already running" >>"$LOG"; exit 0; }
fi

fetch() {
  name="$1"; url="$2"; sha="$3"; size="$4"
  dest="/models/$name"; part="$dest.part"
  if [ -f "$dest" ] && echo "$sha  $dest" | sha256sum -c - >/dev/null 2>&1; then
    echo "$(date -u +%FT%TZ) $name already verified" >>"$LOG"
    return 0
  fi
  while :; do
    have=$(stat -c %s "$part" 2>/dev/null || echo 0)
    if [ "$have" -gt "$size" ]; then
      echo "$(date -u +%FT%TZ) $name partial file is larger than expected, restarting" >>"$LOG"
      rm -f "$part"; have=0
    fi
    if [ "$have" -lt "$size" ]; then
      echo "$(date -u +%FT%TZ) $name resuming at $have of $size bytes" >>"$LOG"
      if ! curl --http1.1 -L --fail -sS -C - --connect-timeout 30 --speed-limit 20480 --speed-time 90 \
           -o "$part" "$url" >>"$LOG" 2>&1; then
        echo "$(date -u +%FT%TZ) $name interrupted, retrying in 20 s" >>"$LOG"
        sleep 20
        continue
      fi
    fi
    if echo "$sha  $part" | sha256sum -c - >/dev/null 2>&1; then
      mv "$part" "$dest"
      echo "$(date -u +%FT%TZ) $name VERIFIED ($size bytes)" >>"$LOG"
      return 0
    fi
    echo "$(date -u +%FT%TZ) $name checksum mismatch at full size, restarting from zero" >>"$LOG"
    rm -f "$part"
    sleep 5
  done
}

fetch Qwen_Qwen3-4B-Instruct-2507-Q4_0.gguf \
  "https://huggingface.co/bartowski/Qwen_Qwen3-4B-Instruct-2507-GGUF/resolve/main/Qwen_Qwen3-4B-Instruct-2507-Q4_0.gguf" \
  b2198e1e35b98e2e126a00d9e853a53bc3a4bcca5f82cfbf2a03b38a91f2662c 2375772896

fetch Qwen_Qwen3-1.7B-Q4_0.gguf \
  "https://huggingface.co/bartowski/Qwen_Qwen3-1.7B-GGUF/resolve/main/Qwen_Qwen3-1.7B-Q4_0.gguf" \
  c470091d31c4ada174ee5c2547daa020e930593cbca5ca8ca385ce8ff59a2fdf 1231813024

echo "$(date -u +%FT%TZ) ALL DOWNLOADS VERIFIED" >>"$LOG"
