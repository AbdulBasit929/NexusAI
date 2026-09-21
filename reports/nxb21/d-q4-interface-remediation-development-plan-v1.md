# NX-B2.1D Q4 interface-remediation development gate

The failed 168-case corpus is permanently consumed and must never be used by this gate, including from a copied or renamed file.

Before a new candidate is frozen, run a fresh development-only diagnostic corpus with new prompts and identifiers that have never appeared in a qualification holdout. Cover English, Urdu script, Roman Urdu, and mixed language. In each language include ordinary and critical examples across exact identifiers, quoted phrases, time ranges, direction, top-k, selected evidence, and authorized workspace scope.

Capture the exact UTF-8 request and response bytes, HTTP status, `finish_reason`, prompt and completion token counts, strict envelope parse, strict planner parse, schema validation, deterministic-authority result, latency, and memory before and after every inference. Exercise the production `resolveWithLanguageAssistance` request shape with strict `forensic_dynamic_capability_plan` response format and the 512-token completion budget. Require every response to finish without `length`, every planner object to parse and validate, all critical cases to pass, and every language to meet the existing 95% natural-language gate.

Only after that diagnostic passes may the implementation, model artifact/profile, prompt, schema, capability references, evaluator, and fresh development receipt be combined into a new candidate identity. Then create and independently freeze a new qualification holdout whose content and hash differ from `aaa8517983ea4523e235c4b3de31e5562bfaeada21ef24a7b23065a567d31d13`. Activation remains blocked until a complete passing receipt and matching source seal exist for that new candidate and new holdout.
