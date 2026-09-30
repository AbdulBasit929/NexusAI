# LAPTOP SPEED, LEVER L1a (ONE CACHE SLOT PER CALL TYPE) -- RESULT: FAILED, ROLLED BACK

Measured 2026-09-29 against THRESHOLD_PREREGISTRATION.md. Runner: scratchpad `run_speed_arms.ps1`,
with log `speed_arms.log`. The plan cache was off in both arms, so every plan called the model.

## Speed

| Arm | Config | Median per question | Total for 8 | LocalAI memory |
|---|---|---|---|---|
| A | `parallel:1`, context 4096 (as deployed) | **72.3 s** | 551.9 s | 4.13–4.15 GB |
| B | `parallel:3`, context 12288 | 93.7 s | 604.8 s | 4.88–5.40 GB |

The answer text was identical in both arms. **B is slower and uses 1.2 GB more, so the threshold failed.**
The model config was restored byte-identical from `model-config-before.yaml`, and LocalAI was restarted
and checked (context 4096, `parallel:1`). The API was redeployed with the plan cache on, and the shipped
switches were asserted afterwards.

**Why it failed.** Separate cache slots help only when prompts share a long fixed start. The plan
prompt's `issued_fields` holds the fields retrieved for *this* question, so prompts diverge within a few
hundred tokens. The earlier 45 s to 0.7 s reuse was for a prompt identical except in its last tokens,
which real questions never are. The next speed lever is **L1b**: put a long fixed part first (the full
family catalogue in a fixed order) and the question-specific retrieval, literals and question last. Then
measure again.

## New questions answered (not in the corpus), checked against the database read-only

| Id | Question | Answer | Database | Verdict |
|---|---|---|---|---|
| S1 | Total call duration of calls made by 923001110001 | refused (the plan used a disallowed IS_NOT_NULL filter) | — | refused, correctly |
| S2 | How many SMS events | refused (plan shape rejected) | — | refused, correctly |
| S3 | Most frequent cell site | 149631808 (1,114) | 149631808, 1,114 | **correct** |
| S4 | Longest single call | 1,798 | max 1,798 | **correct** |
| S5 | ANPR sightings for plate LHR-2026 | "There are 87 ANPR sightings **in this case**." | 87 for LHR-2026, 750 in the case | **Right number, misleading sentence**: the plate filter isn't stated |
| S6 | Camera with the most sightings | CAM-12 (192) | CAM-12, 192 | **correct** |
| S7 | IPDR sessions using HTTPS | "There are 644 IPDR sessions **in this case**." | HTTPS 644, 2,500 in the case | **Right number, misleading sentence**: the protocol filter isn't stated |
| S8 | Total bytes uploaded across IPDR sessions | "The total bytes uploaded across IPDR sessions is **0**." | the IPDR records have **no bytes_up field at all** (0 values) | **Confident wrong**: missing data stated as zero |

**Two defect classes found, and neither needs hardware:**
1. **A headline omits a filter the plan applied** (S5, S7). `answerScopeQualifier` names the target,
   range filters and dates, but not equality filters inside the executed plan. The number is right and
   the sentence claims more than was computed: the same class as A1b.1.
2. **A SUM over zero values is stated as 0** (S8). A total over no recorded values must say the values
   are absent, not state zero.

Both go into the next measured step, before any further speed work.
