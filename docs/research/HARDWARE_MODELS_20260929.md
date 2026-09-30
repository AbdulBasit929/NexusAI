# Development hardware and models, 2026-09-29

## UPDATE 2026-09-29 (owner): this laptop is the only machine. It has to be made capable.

**Measured on the laptop** (read-only requests to the running model; script at scratchpad `bench_llm.py`):
- **Hardware:** Intel i7-1260P (4 performance + 8 efficiency cores, 16 threads), 15.7 GB RAM with 2.1 GB
  free. Docker is capped at 7.6 GB, and the LLM service uses 4.2 GB.
- **Model:** `qwen3-4b-instruct-2507-q4km-nxb21d-dev`, Q4_K_M, 8 threads, 4,096-token context, one
  cache slot (`parallel:1`).
- **Reading a prompt: about 29 tokens/s.** 1,339 tokens took 45.4 s.
- **Writing: about 7.1 tokens/s.** 200 tokens took 28.6 s.
- **Reuse of an already-read prompt start works:** the same 1,338-token start re-read in **0.7 s instead
  of 45 s**.
- **Why a new question takes 2–2.5 min:** each LLM call re-reads a 2,000–3,000-token prompt from cold,
  because the plan-writing, selection and narration calls share one cache slot and evict each other.

**Levers, in order (each measured before it ships):**

| # | Lever | Needs | Expected effect (to be measured) |
|---|---|---|---|
| L1 | Stable text first and the question last in every LLM prompt. One cache slot per call type (`parallel` = number of call types, context scaled to match). Warm the stable starts at startup. | Code and model-config change; memory check | Most of the prompt-reading time disappears for questions after the first |
| L2 | Fewer LLM calls per question: compiler first, skip selection when there is one candidate | Code | Removes whole calls |
| L3 | Shorter outputs: compact plan JSON, tighter `max_tokens` | Code | Fewer of the 7 tokens/s |
| L4 | Thread count tuned for this hybrid CPU (4/6/8/12 compared) | Model config | Unknown, measure |
| L5 | Newer llama.cpp backend (LocalAI reports an upgrade is available) and Q4_0 weights, which get about 40% faster AVX2 kernels through online repacking | **Owner approval** (backend install, model download) | About 1.4× on both reading and writing, if quality holds |
| L6 | A small Qwen3-1.7B for request classification and selection | **Owner approval** (download) | Those calls about 2–2.5× faster |
| L7 | Fine-tune a small model for our plan format on **Kaggle's free GPUs** (30 h/week, T4×2 or P100 16 GB), with demo and synthetic data only. Export it as GGUF and run it on the laptop. It knows the schema, so prompts get shorter. | **Owner approval** (demo data and schema uploaded to Kaggle) | More accurate and faster on the same laptop |
| L8 | Answers don't block the UI: a queue, a progress state and a "ready" notice. Cached and compiler answers stay instant. | UI and backend | 2 minutes stops feeling like a freeze |

**Ruled out:** the Iris Xe integrated GPU through Vulkan. It reads prompts at about 8–12 tokens/s for
7B models, slower than this CPU. 7–8B models on this CPU would halve the speed, and 15.7 GB of RAM
leaves no room for 20B-class models.

**Sources for the update:**
- Q4_0 online repacking, about 40% faster on AVX2: https://github.com/ggml-org/llama.cpp/pull/9921
- Vulkan on Iris Xe: https://zenvanriel.com/ai-engineer-blog/local-ai-integrated-graphics-vulkan-offload/
- Kaggle quota: https://aicreditmart.com/ai-credits-providers/kaggle-free-gpu-tpu-30-hours-week-access-guide-2026/ ·
  https://www.kaggle.com/docs/efficient-gpu-usage

The GPU options below still stand for later.

**Scope, per the owner:** an affordable GPU system for development, not a production server. Prices are
approximate US street prices from September 2026 while GDDR memory shortages continue. Check local
prices before buying.

## What matters for NexusAI

Every question carries a schema slice, family guides and examples, so a prompt runs to several thousand
tokens. The speed of **reading the prompt (prefill)** therefore matters as much as the speed of
writing the answer.

Published prefill speeds:
- For an 8B model: RTX 5090 about 7,200 tokens/s, DGX Spark about 1,200, M5 Max about 400.
- For a 40,000-token context: 6 s, 33 s and about 100 s respectively.
- NVIDIA leads on prefill. AMD's R9700 prefills 2.6–3.4× slower than NVIDIA.

## Options

| Option | Cost | Published performance | Verdict |
|---|---|---|---|
| HP EliteDesk + RTX 2080, 8 GB (already owned; `HP_EliteDesk_LocalAI_Remote_Setup_Guide.docx`) | $0 | Not measured yet. Turing, compute 7.5, supported by llama.cpp CUDA. The guide says to start with 1–4B models and try 7B Q4. | **Use now** as the LLM server. Measure with `llama-bench`. |
| RTX 5060 Ti, 16 GB, new | $430–550 | Qwen3-14B Q4: 32.9 tok/s at 16k context, prefill 942.6 tok/s. Qwen3-8B prefill about 3,000 tok/s at 4k. Qwen3.5-35B-A3B about 44 tok/s. 180 W. | **Recommended** development card |
| RTX 3090, 24 GB, used | $700–1,050 | Qwen3.6-35B-A3B Q3 in about 23 GB at about 120 tok/s. 8B about 105 tok/s. 350 W. | If the bake-off picks a 30B-class model |
| RTX 3060, 12 GB, used | $270–300 | Qwen3-14B Q4 at 9–12 tok/s | Only on a very tight budget |
| Intel Arc B580, 12 GB | $249–289 | Suited to 7B models | Not recommended (our stack is CUDA) |
| Radeon AI PRO R9700, 32 GB | MSRP $1,299, street about $1,880 | MoE models at 127–156 tok/s, weak prefill, ROCm on Linux | Not for us |
| DGX Spark, 128 GB | list $4,699, street $5,000–8,200, short supply | Slow prefill | Production class |
| RTX 5090, 32 GB | about $4,000 for the GPU | 32B at about 62 tok/s | Production class |
| RTX PRO 6000 Blackwell, 96 GB | $14,000–16,000 | 70B at FP8 | Production class |

**Before upgrading the EliteDesk:** check the exact model, the power-supply wattage, whether its PCIe
power connectors are proprietary, and the card length and slot width it takes. A 180 W card usually fits
where a 350 W card doesn't. Plan for 32–64 GB of system RAM, a 1 TB NVMe drive, and Docker with the
NVIDIA container toolkit.

## Renting a GPU before buying

- RunPod Community: RTX 4090 $0.34/h, A100 PCIe $1.19/h.
- RunPod Secure: RTX 4090 $0.69/h.
- Vast.ai: RTX 4090 from $0.34/h, A100 80 GB from $0.50/h.

**Only demo and synthetic data goes to a rented machine, never case evidence.**

## Model shortlist for the Phase 4 bake-off (all Apache-2.0)

| Model | Why it's on the list |
|---|---|
| Arctic-Text2SQL-R1-7B / 14B | Best open SQL specialists at their size on BIRD (68.47% and 70.04%) |
| OmniSQL-7B / 14B | SQL specialists trained on 2.5M synthetic examples |
| Qwen3-8B / 14B | General models, the base for many fine-tunes |
| Qwen3.6-35B-A3B | Mixture of experts: 35B parameters with 3B active, so it's fast on 16–24 GB. Agentic and tool use. |
| gpt-oss-20b | Runs in 16 GB. Reported 85% vs Qwen3's 75% on one practitioner's text-to-SQL test (not a standard benchmark). |
| Current: qwen3-4b-instruct-2507 Q4_K_M | Baseline, about 43% of plans right from the model alone, on CPU |

**Decision rule, written before the run:** most correct answers on investigator questions with 0
confidently wrong, then the lowest p95 latency on the target development GPU.

## Sources

- RTX 5060 Ti: https://www.hardware-corner.net/gpu-llm-benchmarks/rtx-5060-ti-16gb/
- RTX 5070 Ti vs 5060 Ti: https://computingforgeeks.com/rtx-5070-ti-vs-5060-ti-local-ai/
- Used RTX 3090 prices: https://gpudojo.com/rtx-3090
- Qwen3.6-35B-A3B on 24 GB: https://aminrj.com/posts/llamacpp-qwen36-35b/
- Radeon R9700: https://runaihome.com/blog/amd-radeon-ai-pro-r9700-local-ai-hardware-guide-2026/
- Prefill comparison: https://presenc.ai/research/dgx-spark-vs-m5-max-vs-rtx-5090-throughput-2026
- RTX PRO 6000 pricing: https://www.thundercompute.com/blog/nvidia-rtx-pro-6000-pricing
- DGX Spark pricing: https://pi3g.com/nvidia-dgx-spark-price/
- RTX 5090 workstation cost: https://petronellatech.com/blog/how-to-build-custom-ai-workstation-2026/
- Budget GPUs: https://www.popularai.org/p/best-budget-gpus-local-llms-2026
- RunPod pricing: https://www.runpod.io/pricing
- Vast.ai pricing: https://vast.ai/article/how-much-does-it-cost-to-rent-a-gpu-in-the-cloud-live-pricing-guide
- Model licences: https://huggingface.co/Snowflake/Arctic-Text2SQL-R1-7B · https://huggingface.co/seeklhy/OmniSQL-7B · https://huggingface.co/Qwen/Qwen3.6-35B-A3B
