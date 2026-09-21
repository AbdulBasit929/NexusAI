# NX-MMR sample.mp4 video ANPR benchmark

Status: `COMPLETE_PRIVATE_HUMAN_GOLD_NOT_ADMISSIBLE_FOR_PROMOTION` (2026-08-30).

## Verdict

The incumbent fixed one-second sampled-frame policy is not adequate for this
video. Against 19 independent-human plate events containing 41 plate
occurrences, it detected activity in 3/19 events and recognized 3/41 plate
occurrences exactly. The result blocks video-ANPR promotion.

The 60.010-second, 3840×2160/60-fps source was hash-bound to
`d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee`.
The locked oracle digest was
`88c6827c23fbed467f097843404ed09c947bf7f27432e5cb5fa8427be9f48567`.
The reviewer attested to the full source; all unmarked temporal gaps are the
negative frame-presence oracle. No model output was used as truth.

## Frozen execution policy

- 60 frames at source seconds 0 through 59;
- fixed pre-oracle `1.0`-second full-duration ANPR-only sampling;
- the exact incumbent model/configuration and `0.75` threshold used for the
  image benchmark;
- isolated, network-disabled, read-only-input execution with four CPU and
  2 GiB limits; no tracking, tuning or rescore against the human oracle.

## Event and recognition metrics

| Metric | Result |
|---|---:|
| human plate events / truth plate occurrences | 19 / 41 |
| events with any detection / missed events | 3 / 16 |
| event presence recall (95% Wilson) | 0.157895 (0.055205–0.375655) |
| count-proxy detection TP / FP / FN | 3 / 0 / 38 |
| count-proxy precision / recall / F1 | 1.000000 / 0.073171 / 0.136364 |
| normalized exact TP / FP / FN | 3 / 0 / 38 |
| normalized exact precision / recall / F1 | 1.000000 / 0.073171 / 0.136364 |
| exact accuracy per truth plate (95% Wilson) | 0.073171 (0.025198–0.194274) |
| raw exact accuracy | 0.000000; formatting differed |
| normalized CER | 0.926829 |

The event count proxy is not localization precision: the oracle has event
counts/text but no per-frame boxes.

## Sampled-frame false positives and misses

| Metric | Result |
|---|---:|
| sampled positive / negative frames | 34 / 26 |
| frame-presence TP / FP / FN / TN | 3 / 1 / 31 / 25 |
| frame-presence precision / recall | 0.750000 / 0.088235 |
| positive-frame recall, 95% Wilson | 0.030466–0.229605 |
| negative-frame false-positive rate | 0.038462 (1/26) |
| negative-frame FPR, 95% Wilson | 0.006822–0.188928 |
| specificity | 0.961538 (25/26) |

The one temporal false positive was not a novel identifier hallucination: its
normalized value matched a human truth group elsewhere in the video but fell
outside every annotated positive interval. Oracle-wise it remains a false
positive and demonstrates boundary/cadence error.

## Exact-value grouping and source time

| Metric | Result |
|---|---:|
| human / predicted exact-value groups | 29 / 4 |
| exact group TP / FP / FN | 4 / 0 / 25 |
| exact group precision / recall / F1 | 1.000000 / 0.137931 / 0.242424 |
| group-recall 95% Wilson | 0.054974–0.305590 |
| matched groups with boundary measurements | 4 |
| mean / median / p95 absolute boundary error | 1.178250 / 1.011000 / 2.745000 s |
| mean signed first-seen error | +1.648500 s (late) |
| mean signed last-seen error | −0.670500 s (early) |

Grouping is by exact normalized plate value because the oracle contains no
separate physical-track IDs. One exact group came only from the temporally
negative sample described above; identity grouping therefore must not be read
as temporal correctness.

## Redacted error and resource analysis

- Two short events containing four plates had no frame at the fixed cadence.
- Fourteen more events containing 30 plates had one or more sampled frames but
  still had no detection. The three detected events contained seven truth
  plates; only three were detected/recognized, leaving four within-event plate
  misses. Sampling alone therefore does not explain the failure.
- Both independently marked partial events were missed; readable events were
  detected in only 3/17 cases.
- All four frame detections were close to the fixed detector threshold
  (0.750533–0.780030), while their OCR confidences were 0.993588–0.999902.
  The principal observed video bottleneck is plate detection/cadence, not OCR
  after a crop is found.
- Direct model-inference time across 60 frames was 3.930030 seconds: mean
  0.065500, minimum 0.055026 and maximum 0.098327 seconds per frame.
- End-to-end wall time was 63.878912 seconds, process CPU 13.380812 seconds,
  and process peak RSS 273.211 MiB. End-to-end time includes bounded FFmpeg
  decoding of the 184,407,144-byte 4K source.

## Certification consequence

The current video policy/model combination is rejected for any new real-world
or product certification claim. The exact operation ledger and ordinary
suggestions remain unchanged. A new development video, denser policy and/or
tracking-capable detector may be evaluated only against development truth; a
new untouched sealed video is required for an independent future decision.

