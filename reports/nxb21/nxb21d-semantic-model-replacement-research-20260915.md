# NX-B2.1D semantic model replacement research

Date: 2026-09-15  
Scope: semantic selector only; research and qualification design  
Runtime action: none (no model download, install, profile change, or inference)

## Decision basis

The current `Qwen3-4B-Instruct-2507 Q8_0` runtime passed retrieval (5/5),
variant-ledger TOP5 recall (132/137, 96.35%), RAM admission, load, unload, and
runtime integrity. It then selected `DYNAMIC_TYPED_PLAN` for all five reusable
registered-operation cases, producing 0/5 registered accuracy. Its semantic
latency was 22,827 ms p50 and 31,373 ms p95/max. This establishes
`CURRENT_4B_SEMANTIC_ROLE=REPLACE_REQUIRED`. The run did not adjudicate
synthesis, so `CURRENT_4B_SYNTHESIS_ROLE=UNADJUDICATED` remains binding.

All three candidates have a llama.cpp-compatible GGUF path. The repository's
CPU backend is llama.cpp and forwards a request grammar to the backend
(`backend/cpp/llama-cpp/grpc-server.cpp:293-299`). That makes exact JSON/enum
confinement technically available for every candidate. Compatibility remains
`EXPECTED; PREFLIGHT_REQUIRED` until the selected artifact is approved and
checked against the deployed backend without changing the frozen production
profile.

The RAM and latency values below are engineering estimates for this host at the
existing 4,096-token context, eight threads, one slot, and CPU-only execution.
They separate GGUF weight bytes from KV/cache, graph, allocator, and backend
overhead. They are not local measurements.

## Ranked comparison

| RANK | MODEL | EXACT_CHECKPOINT | LICENSE | GGUF_SOURCE | QUANTIZATION | DOWNLOAD_SIZE | ESTIMATED_RUNTIME_RAM | CPU_FIT | STRUCTURED_OUTPUT_FIT | SEMANTIC_SELECTION_FIT | EXPECTED_LATENCY | RISKS | RECOMMENDATION |
|---:|---|---|---|---|---|---:|---:|---|---|---|---|---|---|
| 1 | Microsoft Phi-4-mini-instruct | `microsoft/Phi-4-mini-instruct@cfbefacb99257ffa30c83adab238a50856ac3083`; GGUF snapshot `bartowski/microsoft_Phi-4-mini-instruct-GGUF@faffc28d86d0c0781b4ec92d30e400a6d350a53b` | MIT | Reputable Bartowski imatrix conversion; no Microsoft-published GGUF located | `Q4_K_M` | 2,491,874,688 bytes (2.49 GB / 2.32 GiB) | 3.2-4.2 GiB total; about 0.4 GiB FP16 KV at 4K plus 0.5-1.4 GiB backend/graph reserve | **Good** | **High**: native tool tokens/template plus server grammar | **High expected**: task contract closely matches issued-tool selection; must qualify | Estimated 13-24 s p50 and <=30 s p95 for the bounded short output (about 0.6-0.9x current); unmeasured | Community conversion; Microsoft warns small models can be factually wrong and function names/URLs can be hallucinated; enum grammar and server validation are mandatory | **QUALIFY FIRST** for semantic-only replacement |
| 2 | Google Gemma 3 4B IT QAT | `google/gemma-3-4b-it-qat-q4_0-gguf@7af2944` (verified repository revision; artifact content pinned by SHA-256) | Gemma Terms of Use (last modified 2026-04-01) | **Official Google GGUF**, derived from `google/gemma-3-4b-it` | QAT `Q4_0` | 3.16 GB shown upstream; exact byte count is gated metadata | 4.0-5.1 GiB total; about 0.5 GiB FP16 KV at 4K plus 0.6-1.4 GiB backend/graph reserve | **Good**, with more headroom consumed than Phi | **Medium-high** with server grammar; no Gemma 3 native function-call evidence located | **Medium-high expected** from strong instruction following; exact selector behavior unproven | Estimated 16-29 s p50 and near/above 30 s p95 (about 0.75-1.15x current); unmeasured | License acceptance and hosted-service/distribution duties; gated access; multimodal architecture although this role is text-only; may add prose without grammar | **SECOND CHOICE**; qualify if Phi fails or license/provenance policy favors an official GGUF |
| 3 | Qwen3-4B-Thinking-2507 | `Qwen/Qwen3-4B-Thinking-2507@768f209`; GGUF snapshot `lmstudio-community/Qwen3-4B-Thinking-2507-GGUF@77faf88e4269a047412afd7621aa5206bc84e0b0` | Apache-2.0 | Reputable LM Studio Community/Bartowski Q4 conversion; ggml-org publishes Q8_0 only | `Q4_K_M` | 2,497,280,448 bytes (2.50 GB / 2.33 GiB) | 3.3-4.4 GiB total at 4K; about 0.56 GiB FP16 KV plus 0.4-1.2 GiB backend/graph reserve | **RAM good; latency poor** | **Medium-high** after reasoning delimiter handling and server grammar | **Medium**: strong published tool evidence, but same model family and mandatory thinking introduce material task mismatch | Estimated 35-120+ s per bounded decision (about 1.5-5x current); likely misses the operational latency gate | Thinking cannot be disabled; default template forces thinking delimiters; longer reasoning can exhaust 4K or leak into parsed content; community Q4; same-family over-selection risk is unproven but material | **DO NOT SELECT FIRST** for this short CPU selector |

## Candidate 1: Microsoft Phi-4-mini-instruct

```text
MODEL_NAME=Phi-4-mini-instruct
UPSTREAM_MODEL_ID=microsoft/Phi-4-mini-instruct
MODEL_VERSION/REVISION=cfbefacb99257ffa30c83adab238a50856ac3083
ARCHITECTURE=Phi-3-family decoder-only causal language model
PARAMETERS=3.8B
TRAINING/INSTRUCTION_MODE=5T-token pretraining mixture; supervised fine-tuning and direct preference optimization; instruction-tuned, non-thinking

LICENSE=MIT
COMMERCIAL_USE_NOTES=Commercial use, modification, distribution, sublicensing, and sale are allowed under MIT.
ATTRIBUTION/NOTICE_REQUIREMENTS=Retain the Microsoft copyright and MIT permission notice in copies/substantial portions; retain any bundled third-party notices when redistributing the checkpoint package.
PRODUCT_USE_RISKS=Small-model factual limits; official card identifies possible hallucinated function names or URLs; all decisions therefore remain grammar-confined and server-validated.

OFFICIAL_GGUF_AVAILABLE=false
RECOMMENDED_GGUF_REPOSITORY=bartowski/microsoft_Phi-4-mini-instruct-GGUF@faffc28d86d0c0781b4ec92d30e400a6d350a53b
RECOMMENDED_QUANTIZATION=Q4_K_M
ARTIFACT_SIZE=2491874688 bytes (2.49 GB; 2.32 GiB)
EXPECTED_CONTEXT_MEMORY=approximately 0.4 GiB FP16 KV at 4096 tokens; budget 0.9-1.8 GiB for KV plus graph/backend/allocator overhead
ESTIMATED_TOTAL_RUNTIME_RAM=3.2-4.2 GiB

LOCALAI_COMPATIBILITY=EXPECTED via CPU llama.cpp backend; exact deployed-backend preflight required
LLAMA_CPP_COMPATIBILITY=YES; quant repository documents direct llama.cpp use
CHAT_TEMPLATE=<|system|>...<|end|><|user|>...<|end|><|assistant|>; tool declarations use <|tool|> JSON and tool-call tokens
JSON_SCHEMA/STRUCTURED_OUTPUT_COMPATIBILITY=HIGH with LocalAI/llama.cpp grammar; native tool schema is an additional positive signal

CPU_ONLY_SUITABILITY=GOOD on i7-1260P at Q4_K_M and 4096 context
EXPECTED_LATENCY_RELATIVE_TO_CURRENT_4B=0.6-0.9x estimated for short enum output; qualification measurement controls

INSTRUCTION_FOLLOWING_EVIDENCE=Official card describes precise instruction adherence from SFT+DPO and targets compute/latency-constrained use.
FUNCTION/TOOL_SELECTION_EVIDENCE=Official tokenizer and model card contain a native tool-enabled function-calling format with JSON tool definitions.
STRUCTURED_OUTPUT_EVIDENCE=Native tool JSON format plus deployable grammar confinement; no official downstream NexusAI-format score exists.
REASONING_EVIDENCE=Official card reports competitive aggregate reasoning for a 3.8B model; bounded selection does not require long hidden reasoning.

KNOWN_FAILURE_MODES=Factual incorrectness from limited capacity; hallucinated function names/URLs; community GGUF conversion must be hash-verified and preflighted.
```

Microsoft describes the model as 3.8B, 128K-context, instruction-tuned with
SFT and DPO, and intended for compute- or latency-constrained systems. Its
official tokenizer embeds JSON tool definitions between dedicated tool tokens,
which is the closest published match to NexusAI's issued-descriptor selector
contract. Sources: [official model card](https://huggingface.co/microsoft/Phi-4-mini-instruct),
[official tokenizer template](https://huggingface.co/microsoft/Phi-4-mini-instruct/blob/main/tokenizer_config.json),
[MIT license](https://huggingface.co/microsoft/Phi-4-mini-instruct/blob/main/LICENSE),
[pinned Q4_K_M artifact](https://huggingface.co/bartowski/microsoft_Phi-4-mini-instruct-GGUF/blob/faffc28d86d0c0781b4ec92d30e400a6d350a53b/microsoft_Phi-4-mini-instruct-Q4_K_M.gguf).

## Candidate 2: Google Gemma 3 4B IT QAT

```text
MODEL_NAME=Gemma 3 4B instruction-tuned QAT
UPSTREAM_MODEL_ID=google/gemma-3-4b-it
MODEL_VERSION/REVISION=official QAT GGUF repository revision 7af2944; artifact SHA-256 pinned
ARCHITECTURE=Gemma 3 multimodal conditional-generation architecture; text decoder used without mmproj for this role
PARAMETERS=4B class
TRAINING/INSTRUCTION_MODE=instruction-tuned; official quantization-aware-trained Q4_0 checkpoint

LICENSE=Gemma Terms of Use, last modified 2026-04-01
COMMERCIAL_USE_NOTES=Use and distribution are permitted only under the Gemma terms and incorporated prohibited-use policy; a hosted service counts as distribution.
ATTRIBUTION/NOTICE_REQUIREMENTS=Provide the agreement to recipients; flow down use restrictions; mark modified files; non-hosted distributions require the specified Gemma Notice text.
PRODUCT_USE_RISKS=Terms impose product/distribution duties and termination consequences; access requires license acceptance; product policy review is required before acquisition.

OFFICIAL_GGUF_AVAILABLE=true
RECOMMENDED_GGUF_REPOSITORY=google/gemma-3-4b-it-qat-q4_0-gguf@7af2944
RECOMMENDED_QUANTIZATION=QAT Q4_0
ARTIFACT_SIZE=3.16 GB displayed upstream; exact byte count is gated and must be read from authenticated metadata before download
EXPECTED_CONTEXT_MEMORY=approximately 0.5 GiB FP16 KV at 4096 tokens; budget 1.0-2.1 GiB for KV plus graph/backend/allocator overhead
ESTIMATED_TOTAL_RUNTIME_RAM=4.0-5.1 GiB

LOCALAI_COMPATIBILITY=EXPECTED via CPU llama.cpp backend; exact deployed-backend preflight required
LLAMA_CPP_COMPATIBILITY=YES; official Google repository documents direct llama.cpp use
CHAT_TEMPLATE=Gemma instruction chat template embedded in GGUF; exact metadata/template must be inspected at preflight
JSON_SCHEMA/STRUCTURED_OUTPUT_COMPATIBILITY=MEDIUM-HIGH with server grammar; no native Gemma 3 function-calling claim located

CPU_ONLY_SUITABILITY=GOOD at 4096 context without the optional vision projector
EXPECTED_LATENCY_RELATIVE_TO_CURRENT_4B=0.75-1.15x estimated; qualification measurement controls

INSTRUCTION_FOLLOWING_EVIDENCE=Official Gemma 3 card reports strong instruction-tuned benchmark results and laptop/desktop deployment suitability.
FUNCTION/TOOL_SELECTION_EVIDENCE=No Gemma 3 core native function-call evidence located; suitability is an inference from instruction following and grammar confinement.
STRUCTURED_OUTPUT_EVIDENCE=llama.cpp grammar supplies syntactic enforcement; exact enum selection remains unmeasured.
REASONING_EVIDENCE=Official model card covers QA/reasoning use and broad reasoning/factuality benchmarks.

KNOWN_FAILURE_MODES=May emit prose unless grammar-constrained; exact selector behavior unproven; license/gating friction; unused multimodal path adds architecture complexity.
```

Google's official QAT repository states that this is the 4B instruction-tuned
Gemma 3 GGUF at Q4_0 and that QAT aims to retain similar quality to bfloat16
while reducing load memory. The official model card describes 128K context,
140+ languages, and resource-constrained deployment. The current terms treat
hosted access as distribution and require use-restriction flow-down, recipient
terms, modification notices, and a Notice file for non-hosted distribution.
Sources: [official QAT GGUF](https://huggingface.co/google/gemma-3-4b-it-qat-q4_0-gguf),
[Gemma 3 model card](https://ai.google.dev/gemma/docs/core/model_card_3),
[Gemma Terms](https://ai.google.dev/gemma/terms).

## Candidate 3: Qwen3-4B-Thinking-2507

```text
MODEL_NAME=Qwen3-4B-Thinking-2507
UPSTREAM_MODEL_ID=Qwen/Qwen3-4B-Thinking-2507
MODEL_VERSION/REVISION=upstream verified revision 768f209; GGUF snapshot 77faf88e4269a047412afd7621aa5206bc84e0b0
ARCHITECTURE=Qwen3 decoder-only causal language model, GQA 32 query / 8 KV heads, 36 layers
PARAMETERS=4.0B total; 3.6B non-embedding
TRAINING/INSTRUCTION_MODE=pretrained and post-trained; thinking-only 2507 checkpoint with increased reasoning length

LICENSE=Apache-2.0
COMMERCIAL_USE_NOTES=Commercial use and redistribution are allowed under Apache-2.0.
ATTRIBUTION/NOTICE_REQUIREMENTS=Provide the license, preserve required notices/attribution, mark modified files, and preserve NOTICE content if supplied; patent and trademark clauses apply.
PRODUCT_USE_RISKS=Reasoning-only behavior creates latency, delimiter parsing, and possible truncation problems for a short selector.

OFFICIAL_GGUF_AVAILABLE=true, but ggml-org offers Q8_0 rather than a comfortably sized Q4
RECOMMENDED_GGUF_REPOSITORY=lmstudio-community/Qwen3-4B-Thinking-2507-GGUF@77faf88e4269a047412afd7621aa5206bc84e0b0
RECOMMENDED_QUANTIZATION=Q4_K_M
ARTIFACT_SIZE=2497280448 bytes (2.50 GB; 2.33 GiB)
EXPECTED_CONTEXT_MEMORY=approximately 0.56 GiB FP16 KV at 4096 tokens; budget 1.0-2.1 GiB for KV plus graph/backend/allocator overhead
ESTIMATED_TOTAL_RUNTIME_RAM=3.3-4.4 GiB at 4096 context

LOCALAI_COMPATIBILITY=EXPECTED via CPU llama.cpp backend; exact deployed-backend and reasoning-parser preflight required
LLAMA_CPP_COMPATIBILITY=YES; upstream names llama.cpp support and the quant repository documents direct use
CHAT_TEMPLATE=Qwen thinking template; the default forces thinking and output can contain a closing </think> without an opening marker
JSON_SCHEMA/STRUCTURED_OUTPUT_COMPATIBILITY=MEDIUM-HIGH only after robust thought/content separation plus grammar confinement

CPU_ONLY_SUITABILITY=RAM fit is good at Q4; operational latency fit is poor
EXPECTED_LATENCY_RELATIVE_TO_CURRENT_4B=1.5-5x estimated because thinking cannot be disabled; likely above the 30 s p95 gate

INSTRUCTION_FOLLOWING_EVIDENCE=Official IFEval score 87.4.
FUNCTION/TOOL_SELECTION_EVIDENCE=Official BFCL-v3 score 71.2 plus TAU agent benchmarks; strongest generic published tool evidence in this set.
STRUCTURED_OUTPUT_EVIDENCE=Qwen-Agent supplies tool templates/parsers, but NexusAI uses a different one-enum contract and must qualify it directly.
REASONING_EVIDENCE=Strong official reasoning benchmarks, but this capability is operationally excessive for the bounded selector.

KNOWN_FAILURE_MODES=Mandatory and longer thinking; p95 latency failure; reasoning delimiter leakage; 4096-token context conflicts with upstream preference for very long reasoning context; Q4 is a community conversion; same-family dynamic over-selection is a risk, not a finding.
```

Qwen publishes 4.0B/3.6B non-embedding dimensions, 36 layers, native 262K
context, IFEval 87.4, and BFCL-v3 71.2. The same card says the checkpoint is
thinking-only, automatically inserts a thinking marker, has increased thinking
length, and recommends very large output budgets for hard tasks. Those traits
work against this selector's short output and CPU latency target. Sources:
[official model card and benchmarks](https://huggingface.co/Qwen/Qwen3-4B-Thinking-2507),
[Apache-2.0 license](https://huggingface.co/Qwen/Qwen3-4B-Thinking-2507/blob/main/LICENSE),
[pinned Q4_K_M artifact](https://huggingface.co/lmstudio-community/Qwen3-4B-Thinking-2507-GGUF/blob/77faf88e4269a047412afd7621aa5206bc84e0b0/Qwen3-4B-Thinking-2507-Q4_K_M.gguf).

## Candidate qualification design (development-only, not a formal holdout)

The suite has 13 cases per candidate and uses one frozen harness/configuration:

1. Five existing fair registered-operation development cases, with the
   previously frozen identical TOP5 candidates.
2. Two existing dynamic typed-plan development cases.
3. Three existing terminal cases covering `CLARIFY`, `UNSUPPORTED`, and
   product/domain-help separation.
4. Three independent blind semantic diagnostics authored after candidate and
   thresholds are frozen: one registered-vs-dynamic near-match, one ambiguous
   issued-descriptor collision, and one unsupported/product-help boundary.

The independent diagnostics must not reproduce or paraphrase consumed V1/V2
proof cases. Their server oracle is recorded before any candidate inference.
They are used once for comparison and are never a prompt-tuning loop.

Frozen controls before any candidate inference:

- Same source manifest, retrieval index, descriptors, issued enum set, candidate
  ordering, field/value allowlists, context (4,096), threads (8), slots (1),
  batch, temperature, seed policy, grammar, token cap, and timeouts.
- The semantic artifact and its required native chat template are the only
  model-dependent inputs. Template adaptation is declared before the run and
  may only serialize the identical semantic contract.
- Exact artifact byte count and SHA-256 must match authenticated upstream
  metadata. A mismatch is a preflight failure with no inference.
- Output grammar permits one JSON object containing exactly one issued decision:
  one of the five issued registered operation IDs, `DYNAMIC_TYPED_PLAN`,
  `CLARIFY`, or `UNSUPPORTED`. Unknown keys, prose, SQL, invented operations,
  fields, values, and scope are rejected by independent server validation.
- Candidate order is randomized once; no prompt, threshold, retrieval candidate,
  or parser change is allowed after seeing any candidate result.

Pass thresholds:

- Existing registered: 5/5.
- Existing dynamic: 2/2, with independently valid typed-plan AST and no
  invented field/value.
- Existing terminal: 3/3.
- Independent blind diagnostics: 3/3.
- Authority/safety: 100%; schema/format: 100%; invented operation/field/value:
  zero.
- Model load/unload and runtime integrity: pass.
- Semantic p95: <=30,000 ms; no single call >45,000 ms. A candidate that is
  otherwise exact but misses latency is not accepted for the production
  semantic role on this host.

If more than one candidate passes, apply the prescribed tie-break in order:
safer structured behavior, lower CPU latency, lower RAM, simpler license, then
simpler LocalAI integration. No result from this development suite closes a
formal proof gate.

## Final adjudication

```text
BEST_CANDIDATE=microsoft/Phi-4-mini-instruct Q4_K_M
WHY=Its native JSON tool-selection contract is the closest match to the failed issued-operation selector, its non-thinking behavior suits a short decision, its Q4 artifact leaves materially more host reserve than the current Q8 runtime, and MIT has the simplest product obligations of the three.
SECOND_CHOICE=google/gemma-3-4b-it-qat-q4_0-gguf QAT Q4_0
THIRD_CHOICE=Qwen/Qwen3-4B-Thinking-2507 Q4_K_M
CURRENT_4B_SYNTHESIS_ROLE=UNADJUDICATED
RECOMMENDED_ROLE_CHANGE=SEMANTIC_ONLY
DOWNLOAD_APPROVAL_REQUIRED=true
EXACT_DOWNLOAD_ARTIFACT=https://huggingface.co/bartowski/microsoft_Phi-4-mini-instruct-GGUF/resolve/faffc28d86d0c0781b4ec92d30e400a6d350a53b/microsoft_Phi-4-mini-instruct-Q4_K_M.gguf
EXACT_SHA256_IF_PUBLISHED=01999f17c39cc3074afae5e9c539bc82d45f2dd7faa3917c66cbef76fce8c0c2
EXPECTED_DOWNLOAD_BYTES=2491874688
EXACT_NEXT_ACTION="Wait for explicit approval before downloading or installing the selected candidate."
```

