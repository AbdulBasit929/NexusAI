# Ask engine evaluation (roadmap 6.8.10)

Measures the Ask engine (`api/forensic_records/ask_engine_*.go`) and the governed SQL lane on the same questions, with the real local model, against answers that come from
the case generator's independent oracle. Throwaway database only; nothing here reads or needs real data.

```text
python -X utf8 tools/ask_eval/build_questions.py --seed 73 --out Q73.json
python -X utf8 tools/case_generator/check_ask_engine.py --seed 73 --questions Q73.json --out R73.jsonl --arms engine,lane
python -X utf8 tools/ask_eval/judge.py Q73.json R73.jsonl --show-wrong
```

* `build_questions.py` generates the case of `--seed` (the same parameters the harness uses) and writes about forty questions, two phrasings of each kind: pair counts, incoming
  counts, top contacts, common contacts, distinct contacts, devices, who held an address at an instant, connections within three hops, a number the case does not hold, and
  questions the data cannot answer. Every number in a question is a synthetic number of the reserved block (923000000000 to 923000099999), and every expected answer is the
  oracle's. `questions-seed61.json` is the set the engine was calibrated on; **measure on a seed that was not used to tune** (73 was the first fresh one).
* `check_ask_engine.py` (without `--questions`) runs the 13 executor and audit specs instead. The model is reached at `--model-url` (default `http://127.0.0.1:8080`, the
  LocalAI container that serves the CPU model) as `--model`.
* `judge.py` scores each arm: **correct**, **wrong** (answered and not the expected answer: the defect the budget is measured on), **abstained** (said it could not answer,
  with a reason) and **declined** (gave no answer and handed the question on). For a question the data cannot answer an abstention or a decline is the right outcome.
  It also prints p50, p90 and the longest time per arm, and repairs.
* The gates of a measured run are written in `docs/data/ask-engine-v2.md` before the run, and the run is read against them; a change made after seeing a result is measured
  again on another fresh seed.

On the CPU laptop one question takes about 45 s with the engine (nearly all of it the model writing about 200 tokens at about 5 a second), so forty questions and two arms take
about two hours. Run it when the machine is otherwise idle: the timings are part of the result.
