# LAPTOP SPEED, LEVER L5a: THE SAME 4B MODEL IN Q4_0 -- THRESHOLDS

Written 2026-09-29, before any Q4_0 measurement. Owner approval: downloads, 2026-09-29.

## Why

llama.cpp repacks Q4_0 weights at load time into interleaved AVX2 kernels. Published tests on an
AVX2 Intel CPU show Q4_0 and Q8_0 running about 40% faster than without the optimised kernels
(ggml-org/llama.cpp PR #9921). Today's model is Q4_K_M, which doesn't get that path. Q4_0 is a
slightly coarser quantisation, so answer quality has to be measured, not assumed.

## Files (SHA-256 verified against the publisher, bartowski, the source of today's model)

    Qwen_Qwen3-4B-Instruct-2507-Q4_0.gguf   2,375,772,896 bytes  b2198e1e...
    config  model-config-q40.yaml: identical to today's config except `name` and `model`

## Measurements

1. **Speed**, with `bench_nonce.py` on the same backend and the other model unloaded. Every prompt
   starts with a random nonce, so no reuse of an already-read prompt start is possible. Measured:
   prompt-reading tokens/s on a 1,300-token prompt, and writing tokens/s over 200 tokens, 3 runs each,
   median, for Q4_K_M and Q4_0.
2. **Accuracy**: the full corpus of 103, the 38-question pre-flight, 14 everyday, 16 relational, the
   plate probe and the 8 new questions, run with `FORENSIC_SYNTHESIS_MODEL` set to the Q4_0 name. The
   plan cache is keyed on the payload, which includes the model name, so every plan is generated fresh
   by Q4_0. The reference is the latest shipped-posture run: the headline-honesty round 2 run matching
   the configuration that ships.

## Pass criteria

    speed      Q4_0 median prompt-reading AND writing speed higher than Q4_K_M
    accuracy   0 corpus questions lose CORRECT; 0 new WRONG; pre-flight 38/38; plate 8/8 with no leaks
    new qs     none of the 8 gets worse (correct stays correct, refusal stays refusal or improves)
    memory     no OOM; peak LocalAI memory recorded

Pass: switch `FORENSIC_SYNTHESIS_MODEL` to the Q4_0 name in `.env`, keeping Q4_K_M for rollback.
Fail: keep Q4_K_M and record why.
