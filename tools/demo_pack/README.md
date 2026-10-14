# tools/demo_pack

Builds what a demonstration needs without any real value. Both scripts write OUTSIDE the repository (default `C:\NexusAI-DemoPack`).

| Script | Output |
|---|---|
| `build_demo_pack.py [--seed 1]` | A generated case in the exact layouts of the registered provider formats (call records in four layouts, one file per subscriber like `MSISDN-<number>.xlsx`, four tower dumps, internet records in four layouts, application usage), ready to upload (`upload/`), its answer key (`key/`, never upload), `MANIFEST.md` (file to layout), `DEMO_QUESTIONS.md` and `questions.json` (questions with the answers the key gives, in three tiers: what the product answers today, link analysis, limits to say out loud) |
| `build_question_bank.py` | `ASK_QUESTION_BANK.md`: the 103 questions of the corpora with independent answer keys (`evaluation/*.json`), grouped by case and family in the order a demonstration should use, with cautions, and templates for a real-data demonstration that hold no real value |

The synthetic files carry only values of the project's reserved ranges (`tools/real_data/reserved.py`), so they can be shown to anyone. The real files supplied by the owner stay in the vault
(`docs/data/real-data-handling-policy.md`); their use for a demonstration is a use the authority register must record first.
