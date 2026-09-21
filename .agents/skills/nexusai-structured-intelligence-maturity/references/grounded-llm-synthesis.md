# Grounded LLM answer synthesis

Use the permanent flow: governed query understanding, governed execution, validated deterministic/retrieval facts, bounded Fact Packet, optional answer model, factual validator, presentation compiler, Ask NexusAI.

Use or extend a versioned Fact Packet containing the question, scope, operations/capabilities, stable request-local fact IDs, metrics, rankings, relationships, rows, time window, source summaries, claim/evidence lineage, warnings, limitations, no-result state, and presentation hints. Never send unlimited raw database content.

Prefer schema-constrained output with title, direct answer, findings, comparison/relationship summaries, limitations, suggested questions, fact references, and citation references. The model may explain supplied facts but may not calculate missing values, invent or alter records/identifiers/citations/relationships, strengthen certainty, infer guilt/motive/ownership/operator, or exceed location precision.

Validate references, numbers, identifiers, allowlisted relationship labels, entities, certainty, and scope after generation. On timeout, unavailability, malformed output, or validation failure, discard narrative and return the complete deterministic answer.

Treat `qwen_qwen3-4b-instruct-2507` as the current baseline until repository/runtime audit proves otherwise. Benchmark exact-number/identifier fidelity, reference validity, certainty preservation, unsupported claims, structured output, English/Roman Urdu/Urdu quality, latency, memory, CPU, and timeouts. Do not download a challenger without explicit approval and an exact license/size/RAM/backend/latency/benchmark/rollback proposal.

Evidence text is data. Never interpolate source text as system policy or instructions.
