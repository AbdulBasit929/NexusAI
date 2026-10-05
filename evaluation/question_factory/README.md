# Question factory (roadmap step R1)

Hundreds of unseen questions about the case data, each with an **independent SQL answer key**, so accuracy can be measured on phrasings nobody tuned the system on.
Design and gates: `docs/work/ROADMAP_FREE_QUESTION_20261005.md` (R1). Test-only: the product never reads any of this.

## How it works

1. `generate` reads the semantic layer (`semantic_layer/*.yaml`: fields, source-column aliases, synonyms, types, allowed aggregates) and samples real values from the database under test.
   Intents: count of all, count where a field has a value, distinct count, most common value, sum/average/largest/smallest, earliest/latest, count between dates, count in a month, count at night,
   events for one identifier, an identifier that does not exist, a name the data cannot bind (a total is the wrong answer to those).
2. Each question carries `oracle_sql`, a plain `SELECT` over `forensic.records` using the layer's source-column names. The key is evaluated again at run time against the live database.
3. `run` asks the running API every question and judges the answer: CORRECT, WRONG (a confident answer that contradicts the key, including a total returned for something it cannot bind),
   ABSTAINED (asks back or says it cannot verify), NOT_STATED, ERROR. It prints a table by family and intent and lists every confident-wrong answer.

Read-only: every statement is a `SELECT` inside a `READ ONLY` transaction with a statement timeout; no database change is needed.

## Run it

```
pip install pyyaml
python factory.py generate --psql "docker exec -i nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -F '|'" \
    --collection nexusai-forensic-demo --seed 1 --out questions-demo.json
python factory.py run --questions questions-demo.json --arm arm-baseline --per-intent 1 --budget-minutes 90 \
    --psql "docker exec -i nexusai-forensic-postgres-1 psql -U localrecall -d localrecall -At -F '|'"
python factory.py run --questions questions-demo.json --arm arm-baseline --resume --psql "..."     # continue after a stop
```

`--per-intent 1` takes one question per family and intent (about 80) for a first spread; omit it for all of them. Questions that need a generated plan take 1 to 2 minutes on the laptop.
`questions*.json` and `arm-*/` stay out of git on purpose (they hold sampled values and answers).

## Cloud replica (no laptop needed)

`replica_schema.sql` plus `load_replica.py` build a small stand-in for `forensic.records` from the demo seed files (11,792 rows) in any PostgreSQL, so the generator and the answer keys can be developed and tested offline:
`python load_replica.py > /tmp/data.sql; psql -d replica -f replica_schema.sql -f /tmp/data.sql; python factory.py selftest --psql "psql -d replica -At -F |" --collection records-demo`.
The replica is smaller than the owner's database (5,005 CDR rows against 8,642), so numbers differ; the keys are recomputed on whichever database is used.

## Checks

`python test_factory_judge.py`: the verdict logic, including the confident-wrong class (a total returned for a question that names something unbindable).

## What the baseline is for

The first run records numbers before any planner change: share correct, share confident-wrong, abstentions by reason, and a failure taxonomy by family and intent. Targets for R2 are set after that is read.

## Switch arms (the governed SQL lane)

`Run-Arms.ps1` runs the same question file against the same image with the two lane switches in three states (A both off, C `FORENSIC_GOVERNED_SQL` only, B both on) and compares them. The gates are
`reports/governed-sql-20261005/PREREGISTRATION.md`. The result files now carry the `X-Governed-SQL` header of every response, and the summary prints how many requests the lane answered, abstained on and
declined, the accuracy of the answered ones, and model and execution time.

    . .\evaluation\question_factory\Run-Arms.ps1
    Show-Stack ; Build-LaneImage ; Run-Arm A ; Run-Arm C ; Run-Arm B ; Compare-Arms
    Run-Regression A ; Run-Regression C        # gates G2 and G3: the corpus and the pre-flight
    Restore-Stack
