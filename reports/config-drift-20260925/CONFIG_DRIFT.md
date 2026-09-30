# SILENT CONFIGURATION DRIFT — found 2026-09-25 22:50

The running container had **five settings** at values the project documents as
their opposite, including the gate behind the Phase 1 zero-confident-wrong
property. The golden 62 was measured in that state and reported as clean before
anyone noticed.

| switch | documented | container | effect |
|---|---|---|---|
| `FORENSIC_VERIFIED_ONLY` | true | **false** | the S6/S9 gate was not withholding unverified answers |
| `FORENSIC_IR_ARBITRATION` | true | **false** | a refused plan was not re-earned |
| `FORENSIC_IR_FALLBACK` | true | **false** | no fallback when no plan was earned |
| `FORENSIC_PLAN_CACHE` | true | **false** | every completion paid full model cost |
| `FORENSIC_PLAN_CACHE_DIR` | `/data/forensic/spool/plan-cache` | **empty** | nowhere to persist it |

`FORENSIC_IR_SHADOW`, `FORENSIC_IR_CROSSCHECK`, `FORENSIC_DROP_INVENTED_FILTERS`
and `FORENSIC_LADDER_ROUTING` were correct.

## Cause

`docker-compose.forensic-records.yaml` declares each switch as
`${NAME:-false}`. `.env.forensic-runtime.local` defined **none of them** — only
credentials, tenant, timezone and image tags. So any deploy sourcing that file
alone silently received compose's defaults.

**An omitted switch is indistinguishable from one set to false**, and nothing
logs the gate state at startup: the only startup line is

    INFO forensic records webhook listening addr=:8091 auth_enabled=true

## When

    plan-cache directory   196 entries, last written   17:32
    baseline golden run    finished                    17:33   <- cache present
    deploy (curation+H4)                               21:40   <- drift begins
    golden run reported clean                          22:29   <- no cache, no gate

The plan cache is the tell: it stopped being written at the 21:40 deploy. Runs
at 15:14, 15:39, 16:07 and 17:33 all wrote to it, so they held the documented
configuration.

## What it invalidates, and what it does not

**Latency comparisons across 21:40 are void.** Median 2.2s -> 4.0s, with
140-180s outliers and CDR-14 crossing the 240s client ceiling and scoring as an
ERROR. That was read at the time as a cold cache; it was NO cache.

**Verdict comparisons survive the cache half.** The plan cache memoizes a
completion requested at temperature 0, so it is a deterministic function of the
payload: it changes speed, not answers.

**Verdict comparisons do NOT survive the gate half unassisted.** `VERIFIED_ONLY`,
`IR_ARBITRATION` and `IR_FALLBACK` all change which answers are given. A control
pass on the restored configuration is the only sound baseline, and that is why
the breakdown-goal measurement runs one.

**One reassuring observation, not a conclusion:** baseline (gate ON) scored 47
CORRECT / 14 CLARIFIED, and the drifted run (gate OFF) scored 46 + 1 timeout /
14 CLARIFIED. Identical clarification counts suggest the gate currently withholds
almost nothing on the 62 -- the compiler now answers what the ladder used to
answer unverified. The control pass measures this rather than assuming it.

## Fix applied

All five written explicitly into `.env.forensic-runtime.local` with the reason
for each, and asserted from the container afterwards.

## Standing protocol

**Assert configuration from the CONTAINER before scoring any run, and record it
in the report.** Takes seconds:

    docker inspect nexusai-forensic-records-api-1 --format '{{range .Config.Env}}{{println .}}{{end}}'

**Still open:** nothing logs the switch state at startup. A single startup line
naming every gate would have made this visible in the first second instead of
five hours and one misread measurement later. It is a source change needing its
own build, deliberately not bundled into the breakdown-goal measurement.
