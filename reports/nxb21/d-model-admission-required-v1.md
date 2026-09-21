# NX-B2.1D model admission required

Status: **MODEL_ADMISSION_REQUIRED**. Activation is blocked.

The 2026-09-04 one-shot run consumed 81 of 168 frozen cases before Ginkgo's
independent one-hour suite timeout. All 81 cases called the installed
`qwen_qwen3-4b-instruct-2507` profile. Eighty returned `malformed_proposal`, one
timed out, and none produced schema-valid output. Host available RAM fell to
1.412 GiB and the API container peaked at 5.812 GiB. These observations make the
deployed Q8 profile insufficient for this bounded planner role.

The run was also invalid as an intrinsic base-model evaluation. The frozen
manifest specified an 8192-token context, while LocalAI reported the live
profile's effective context as 4096. The live YAML's default temperature was
0.6, but the planner request set temperature zero, so that default is not an
observed decoding mismatch. The YAML also had `function.grammar.disable: true`,
but source inspection shows that LocalAI handles `response_format` JSON-schema
grammar separately from generated function-call grammar. No grammar-generation
error appeared in the LocalAI log. Because the original evaluator discarded
malformed raw completion content, the exact cause remains unresolved among
backend schema enforcement, chat-template or stopword behavior, prompt behavior,
and other response-content failures. llama.cpp documents that JSON Schema
constrains output but is not injected into the prompt, so the planner prompt must
explicitly describe the expected fields. It also documents schema-conversion
limitations that must be checked before another evaluation. See the
[llama.cpp grammar guide](https://github.com/ggml-org/llama.cpp/blob/master/grammars/README.md).

The preferred corrective candidate is the same Qwen base model at Q4_K_M:
`bartowski/Qwen_Qwen3-4B-Instruct-2507-GGUF`, revision
`ae44f08e1392f39c0e474af10c3ff8355c8b6688`, exact file
`Qwen_Qwen3-4B-Instruct-2507-Q4_K_M.gguf`, 2,497,280,736 bytes, LFS SHA-256
`2fde00ce69dd4899c70d020845e2638353015bba0fdf161b3eb965f2bca4464e`.
The quantization publisher labels Q4_K_M a recommended default at 2.50 GB, while
the current Q8 file is 4.28 GB. The Qwen publisher describes the 4B Instruct
model as non-thinking, Apache-2.0, improved for tool use and multilingual
performance; the Qwen3 family documentation lists 119 languages and dialects,
including Urdu. [Qwen model card](https://huggingface.co/Qwen/Qwen3-4B-Instruct-2507),
[Q4_K_M artifact](https://huggingface.co/bartowski/Qwen_Qwen3-4B-Instruct-2507-GGUF),
[Qwen3 language list](https://qwenlm.github.io/blog/qwen3/).

This choice preserves the model family most likely to support the required Urdu
coverage while reducing artifact memory by 41.66%. Based on the observed Q8
container peak and the artifact reduction, the estimated Q4 API-container peak
is 3.9–4.5 GiB at a 4096-token development context. That is an engineering
estimate and must be measured locally. Structured JSON reliability, Roman Urdu,
mixed-language accuracy, and latency remain unproven.

The serious alternative is Mistral AI's official
`mistralai/Ministral-3-3B-Instruct-2512-GGUF`, revision
`eb599d408350ea2bb60452cb86be7c7b2fc28227`, Q4_K_M file
`Ministral-3-3B-Instruct-2512-Q4_K_M.gguf`, 2,147,023,008 bytes, LFS SHA-256
`9ed150d4367e68df0ac8e1540f6ddc65b42d0ee26378329d1ecbca60f93fc5f8`.
Mistral documents Apache-2.0 licensing, llama.cpp use, edge deployment, native
function calling and JSON output, and a 2.15 GB Q4_K_M. Its published language
list does not explicitly include Urdu, so it cannot be preferred until a
development corpus proves Urdu and Roman Urdu behavior.
[Official Ministral GGUF card](https://huggingface.co/mistralai/Ministral-3-3B-Instruct-2512-GGUF).

Google's official Gemma 3 4B QAT Q4_0 is a fallback: revision
`15f73f5eee9c28f53afef5723e29680c2fc78a`, exact file
`gemma-3-4b-it-q4_0.gguf`, 3,155,051,328 bytes. Google states that QAT preserves
quality close to bfloat16 with lower memory and that Gemma 3 supports more than
140 languages. The repository is manually gated under Gemma terms, it is larger
than both leading candidates, and strict planner JSON behavior remains
unmeasured. [Official Gemma GGUF card](https://huggingface.co/google/gemma-3-4b-it-qat-q4_0-gguf).

The next authorized phase must use development data only: acquire exactly one
pinned candidate, verify its size and SHA-256, create an isolated profile with a
single context value, validate LocalAI's `response_format` schema path, preserve
malformed raw completions in private development evidence, explicitly describe
every JSON field in the prompt, and test English, Urdu, Roman Urdu,
mixed language, literal and identifier preservation, time, follow-ups,
composition, clarification, and adversarial rejection. Only after this passes
with safe memory and latency may a new independent 168-case holdout be frozen.
The consumed corpus cannot be tuned against or rescored.

No candidate was downloaded, installed, loaded, or activated while preparing
this admission report. The exact requested approval is recorded in the paired
machine-readable JSON.
