# LAPTOP SPEED, LEVER L1a: ONE CACHE SLOT PER KIND OF CALL -- THRESHOLDS

Written 2026-09-29, before the arms ran.

## Why

Measured on this laptop (i7-1260P, CPU only):
- Qwen3-4B Q4_K_M reads prompts at about 29 tok/s and writes at about 7.1 tok/s.
- Re-reading an already-seen 1,338-token prompt start takes **0.7 s instead of 45 s**.

LocalAI runs the model with `parallel:1`, a single cache slot. The selection, plan-writing, arbitration
and narration calls have different prompt starts, so each one evicts the previous one's cached start,
and nearly every call reads its prompt cold.

## Change (config only, no code)

`/models/qwen3-4b-instruct-2507-q4km-nxb21d-dev.yaml` (Docker volume `nexusai_models`):
- `parallel:1` becomes `parallel:3`
- `context_size: 4096` becomes `12288`, so each slot keeps 4,096 tokens

The original is backed up to `reports/laptop-speed-20260929/model-config-before.yaml` for rollback.
The extra KV memory is about 1.2 GB in f16, checked against the 7.6 GB Docker limit.

## Arms

    A  parallel:1 (as deployed)      API with FORENSIC_PLAN_CACHE=false, so every plan calls the model
    B  parallel:3, context 12288     same API
    Probe: 8 questions not in the corpus, grouped by type (4 CDR, 2 ANPR, 2 IPDR), run in order,
    speed_probe.py. The same questions in each arm; the model is warmed before each arm.

## Pass criteria

    answers   the text of all 8 answers is identical between A and B (output must not change)
    speed     B median wall time per question lower than A; each question's time reported
    memory    no container restart or OOM; peak LocalAI memory recorded
    then      before shipping B: the full 103 unchanged and pre-flight 38/38 with the plan cache back on

Fail any line: roll back to the backed-up config.
