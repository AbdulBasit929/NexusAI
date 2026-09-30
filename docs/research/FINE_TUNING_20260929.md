# Fine-tuning research, 2026-09-29

**Question from the team lead:** can we fine-tune a model to get better results, on the current laptop
or on a future GPU system?

## Evidence

- **Small curated data is enough.** LIMIT (arXiv 2609.24186) trained Qwen3-8B on 796 examples for BIRD
  and 863 for Spider. It reached 69.1% and 88.9% execution accuracy, beating methods trained on 20× more
  data. Curation beats volume.
- **Rewarding correct execution works.** Arctic-Text2SQL-R1 trains with the execution result as the
  reward (arXiv 2505.20315).
- **Synthetic data at scale** is how OmniSQL trained (SynSQL-2.5M, arXiv 2503.02240).
- **Memory needed with Unsloth QLoRA:**
  - Qwen3-14B fits a 16 GB T4.
  - Qwen3-30B-A3B needs about 17.5 GB.
  - An 8 GB card handles up to about 4B with short sequences.
  - Unsloth runs on Turing GPUs, since the T4 is Turing.
- **CPU-only fine-tuning** is possible for small models but takes days. It isn't practical on this
  laptop, which has 15.7 GB of RAM and often under 2 GB free.

## Feasibility by machine

| Machine | Feasible | Use |
|---|---|---|
| This laptop (CPU) | Tiny models only, over days | None |
| RTX 2080, 8 GB | Up to about 4B with short sequences | A request classifier (short inputs), not SQL writing (long schema prompts) |
| 16 GB GPU | 8B comfortably, 14B tight | The SQL writer |
| 24 GB GPU | 14B comfortably | The SQL writer, with more context |
| Rented 80 GB A100 | Any of these, about 1–2 h for about 1k examples (estimate) | Cheapest start, demo and synthetic data only |

## Plan (roadmap Phase 5; only if the Phase 4 bake-off leaves a measured gap)

1. **Training set:** 800–1,000 question-to-query pairs (plan or SQL), each verified by execution, covering
   every analyst view.
   - Sources: the approved-example library, plus synthetic questions generated over demo cases and
     filtered by execution.
2. **Never train on the evaluation sets:** the 103 corpus questions, the cross-case set and the
   investigator set.
3. **Train a QLoRA adapter** on the chosen base model. Serve it next to the base model, so it can be
   switched off instantly.
4. **Gate:** more correct answers and 0 new confidently wrong answers than the base model, under a
   decision rule written first.
5. **Retrain when the schema changes.** The schema-drift test from Phase 2 triggers it.

## Risks

- Overfitting to our phrasing.
- Contamination of the evaluation sets.
- Stale knowledge after schema changes.
- Loss of general ability. The adapter approach limits this.
- Case evidence must never leave the premises for training.

## Sources

- LIMIT: https://arxiv.org/abs/2609.24186
- Arctic-Text2SQL-R1: https://arxiv.org/abs/2505.20315
- OmniSQL: https://arxiv.org/pdf/2503.02240
- Unsloth Qwen3: https://unsloth.ai/docs/models/tutorials/qwen3-how-to-run-and-fine-tune
- Unsloth requirements: https://unsloth.ai/docs/get-started/fine-tuning-for-beginners/unsloth-requirements
- CPU LoRA: https://arxiv.org/abs/2507.01806
