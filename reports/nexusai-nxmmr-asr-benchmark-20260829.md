# NX-MMR ASR benchmark status

Status: `NOT_ADMISSIBLE_NO_POSITIVE_SPEECH_OR_TRANSCRIPT` (2026-08-30).

The independent human oracle is complete, locked and valid, but all six speech
audit windows are explicitly labeled `speech_present=no`. There is no positive
speech, verbatim transcript, language-bearing reference or unintelligible
speech segment. This is sufficient to establish that WER, language-specific
WER, identifier preservation and transcript timestamp error are not
measurable; it is not sufficient transcript evidence to run
`faster-whisper-small-ur` under the authorized rule.

No ASR model was loaded or called. No model output was used as an oracle. ASR
latency and peak-memory measurements are therefore `not measured`, rather than
zero. The source’s AAC stream and the private lossless review copy remain
unchanged.

| Capability | NX-MMR result |
|---|---|
| speech presence oracle | 6/6 reviewed windows negative |
| English WER | not admissible; no positive reference transcript |
| Urdu WER | not admissible; no positive reference transcript |
| Roman Urdu rendering | not admissible; no source speech or reviewed secondary rendering |
| identifier preservation | not admissible |
| ASR source-time error | not admissible |
| ASR resource measurements | not run / not measured |
| speaker identity | `REJECTED_OUT_OF_SCOPE` |
| TTS | `DEFERRED_NOT_BENCHMARKED` |

This pack adds no certification evidence for `faster-whisper-small-ur` and does
not alter its prior limited status. `BENCHMARK_DATA_REQUIRED` remains for
independently transcribed Pakistani English, native Urdu and code-switched
speech, including noise, identifiers, overlaps and explicit negative windows.
No operation or model is promoted.
