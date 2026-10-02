# Models, architecture, hardware and ownership: findings and roadmap (2026-10-02)

Three passes, in the order the product owner asked for. Pass 1 is read from this repository only (no database or Docker is
reachable from this session, so nothing here was measured live). Pass 2 is external research; its figures are other people's
machines and are leads to test, not predictions. Pass 3 is a design and a roadmap that follows the repository's own rules:
deterministic parsers and SQL for exact facts, models only for synthesis, abstain rather than guess, thresholds written
before measuring.

## Pass 1. What the code and schema already say

### Does the database record who created each case and each piece of evidence?

**Evidence: yes, as a free-text identity string. Cases: no.**

| Thing | Where | What it records | Caveat |
|---|---|---|---|
| Evidence item | `forensic.evidence_items.user_id text NOT NULL DEFAULT ''` (`db/forensic_records/007_evidence_items.sql`) | The subject that registered the file | Empty string when none was supplied |
| Evidence version | `forensic.evidence_versions.created_by text NOT NULL DEFAULT ''` (`008_phase3_evidence_control_plane.sql`, filled from `NEW.user_id` at registration, around line 690) | Who created each version (a re-upload makes version 2 and so on) | Same string, copied |
| Processing run | `processing_runs.requested_by` | Who asked for the run | |
| Custody and processing events | `actor_type`, `actor_id` on the custody and event tables (008 lines ~262, ~290); chain tail in 011 | Who did what, as an append-only chain | Defaults to `system` / empty |
| Reprocess | `job.Metadata["reprocess_requested_by"] = scope.ActorID` (`api/forensic_records/reprocess.go`) | Who asked for a retry | In job metadata, not a column |
| Case | **no table** | Nothing. A case is a `collection_id` / `case_id` string that exists because evidence rows carry it | No owner, no creator, no created time, no name, no status, no members |

How the identity string gets there (`api/forensic_records/auth_scope.go`, `main.go` ~257, `cases.go` ~208):

- With API auth required, the gateway sets `X-Forensic-Actor-ID`, `X-Forensic-Subject-ID` and `X-Forensic-Actor-Role`
  (`user`, `admin` or `agent-worker`) and the API trusts them. The evidence `user_id` is the subject header.
- With auth off, the API takes `user_id` from the request form or body, so anyone can claim any user. That is acceptable for
  local development and wrong for any "my cases" feature.
- Nobody sends a per-person identity today. The investigation workspace's own client sends a **fixed** one:
  `X-Forensic-Actor-ID` and `X-Forensic-Subject-ID` default to `investigation-workspace`
  (`apps/investigation-workspace/src/lib/apiClient.js` lines ~34-35), with one shared API key attached by the dev proxy. The
  LocalAI proxy path is fixed too: `docker-compose.forensic-runtime.localai.yaml` sets
  `FORENSIC_RECORDS_PROXY_ACTOR_ID` to `nexusai-local-operator` and the role to `admin` ("an intentionally local no-auth UI;
  authenticated deployments ignore these values and forward the real user"). So evidence is recorded as uploaded by one
  of those two constants, not by a person. The dev-proxy comment says the same: real multi-user identity does not exist yet
  (no users, roles or membership tables for this API).
- LocalAI's own front door does have users (`core/http/auth/users.go`) and passes real `actorID` / `subjectID` when it calls
  the forensic API (`core/http/endpoints/localai/agent_collections.go` ~258). That is the natural source of real identity.

**What this means for "my own cases".** Today it can only be approximated as "cases where my subject registered any
evidence", and for anything uploaded through this workspace that set is empty or wrong, because the subject is a constant.
A true "my cases" needs three things that do not exist: a case record with an owner, membership (who else may open it),
and a real per-person identity flowing from sign-in through the proxy to the API. Backend request 18 (case name and status)
already needs a case record; ownership and members belong in the same table.

### One read-only check to run on the real database

Run it on the machine that has the database (nothing here can):

```sql
-- who is recorded as having registered evidence, per case
SELECT collection_id, user_id, count(*) AS items, min(created_at) AS first_seen
FROM forensic.evidence_items
GROUP BY collection_id, user_id
ORDER BY collection_id, items DESC;

-- who is recorded in custody events
SELECT actor_type, actor_id, count(*) FROM forensic.evidence_custody_events GROUP BY 1, 2 ORDER BY 3 DESC;
```

(If the custody table has a different name in your build, `\dt forensic.*` lists it.) Expect one dominant value,
`nexusai-local-operator` or `investigation-workspace`, plus whatever the earlier test fixtures used.

### What is and is not in this checkout about the model work

The pre-registered measurements, `headline_honesty.go`, the Q4_0 model configs and the downloaded 4B and 1.7B Q4_0 weights
described in the earlier session are not on this branch or in this container; they live in the local checkout and Docker
volumes. What the repository does show: the synthesis model is Qwen3-4B-Instruct-2507 at Q4_K_M on **CPU only, about 4 tokens
per second, about 15.7 GiB RAM** (`NEXUSAI_CONTINUATION.md` ~line 161), the model catalogue prefers deterministic engines for
exact facts and says never to download large weights without approval (`configuration/forensic_evidence_model_catalog.json`),
and a hardware-tier contract exists (`configuration/nexusai_hardware_tiers.json`).

## Pass 2. External research

All figures below come from public write-ups on other hardware. They tell us where to look, not what we will get.

1. **Generation speed on a CPU is limited by memory bandwidth, not by arithmetic.** AVX-512 speeds up reading the prompt
   (reported 2.8 to 10 times) but not producing tokens, which is memory-bound
   ([InsiderLLM](https://insiderllm.com/guides/cpu-only-llms-what-actually-works/),
   [SwiftInference](https://www.swiftinference.ai/blog/run-llm-inference-on-cpu-with-llamacpp-and-a-rest-api-2026-04-24)).
   A 3 to 4B model at about 4.5 bits is roughly 2.5 GB of weights read once per token. Web reports for modern desktop CPUs
   with dual-channel DDR5 give 25 to 45 tokens per second for 3B-class models
   ([Popular AI](https://www.popularai.org/p/best-cpu-only-local-llm-2026),
   [PromptQuorum](https://www.promptquorum.com/local-llms/best-cpu-only-llm)). **Our measured 4 tokens per second for a 4B model
   is far below that**, which suggests the limit is the machine's configuration (single memory channel, too few threads
   handed to the container, a CPU quota, or an old CPU) before it is the model. Diagnose that first; it may be the cheapest
   win available.
2. **Q4_0 against Q4_K_M.** Q4_K_M keeps more quality at a similar size; one 8B measurement had Q4_0 faster on prompt
   processing (97 against 88 tokens per second) and slower on generation (4.4 against 5.1)
   ([arXiv 2601.14277](https://arxiv.org/html/2601.14277v1), [SitePoint](https://www.sitepoint.com/quantization-q4km-vs-awq-fp16-local-llms/)).
   So Q4_0 is a prefill lever (it can be repacked for fast CPU kernels), not a generation lever. That matches why it was tried:
   our latency is dominated by long planner and synthesis prompts. Below four bits there is no speed gain on CPU.
3. **Small models.** Candidates that run on CPU: Qwen3 1.7B and 4B, Phi-4-mini (3.8B), Llama 3.2 3B, Gemma 3 and Gemma 4 small
   variants ([Hugging Face blog](https://huggingface.co/blog/daya-shankar/open-source-llm-models-to-run-locally),
   [SitePoint](https://www.sitepoint.com/best-local-llm-models-2026/)). The write-ups do not compare grammar-constrained JSON
   or tool-call reliability, which is what the planner needs, so this has to be measured on our own fixtures. Check each
   licence before use (the catalogue's rule is no downloads without approval).
4. **Speculative decoding** (a small draft model proposes tokens, the big one verifies) is reported at 1.5 to 2.5 times on
   GPUs, but llama.cpp's own documentation publishes no figures, and one benchmark found no mode faster than baseline;
   on an already small model the draft's overhead can make it slower
   ([llama.cpp docs](https://github.com/ggml-org/llama.cpp/blob/master/docs/speculative.md),
   [HackMD test](https://hackmd.io/ODXuOQNzSiyUITz7g9mtBw),
   [llama.cpp issue 21453](https://github.com/ggml-org/llama.cpp/issues/21453)). A 1.7B draft for a 4B target is worth one
   measured trial on our CPU, no more.
5. **Hardware options if CPU tuning is not enough** (prices as of late September 2026 in the cited write-ups): an AMD Ryzen AI
   Max+ 395 mini PC with 128 GB unified memory (about $3,450 to $3,650, 256 GB/s, runs 120B-class mixture-of-experts models
   at roughly 31 to 55 tokens per second, dense 70B at 4 to 6), a Mac Studio M4 Max 128 GB (about $3,700, 546 GB/s, faster per
   token), or a discrete GPU box (RTX 4090 builds about $2,200 to $2,800 but about 30B parameters of capacity)
   ([Terminal Bytes](https://terminalbytes.com/best-mini-pc-for-local-llm-2026/),
   [Compute Market](https://www.compute-market.com/blog/strix-halo-mini-pc-local-ai-2026),
   [How-To Geek](https://www.howtogeek.com/amd-ryzen-ai-halo-mini-pc-price-availability/)). The trade is capacity (Strix Halo) against
   speed on models that fit (Mac, GPU). For this product a GPU or unified-memory box would be an inference node reached over
   the network, not a replacement for the database host.

## Pass 3. Design and roadmap

### Principles (from the repository, restated)

1. Exact facts come from deterministic SQL and parsers; the model only plans, explains and summarises, and abstains when
   unsure. So the fastest model is the one that is called least and is given the shortest prompt.
2. Every change is measured against pre-registered thresholds on fixed fixtures; a change that fails a threshold is not
   shipped, and a model never silently falls back to a weaker one (hardware-tier rule).
3. Nothing is downloaded or activated without explicit approval; no live data is mutated by a measurement.

### Roadmap

**M0. Find out why 4 tokens per second (one afternoon, no downloads). Tool written: `scripts/diagnose_cpu_inference_m0.ps1`.**
Run `powershell -ExecutionPolicy Bypass -File scripts\diagnose_cpu_inference_m0.ps1 -Quick` on the Windows host with the
LocalAI container running and the laptop plugged in (add `-Model`, `-BaseUri` or `-ApiKey` if yours differ; drop `-Quick` for a
long-prompt timing too). It is read-only and writes `reports\m0-cpu-diagnostic-<time>\report.md`. The compose file names the
reference machine a 16 GiB Windows laptop, which makes four things likely suspects, in this order: a single memory module
(single-channel memory), the Windows power plan or battery throttling, the Docker VM's processors and the model's thread
count, and a container CPU limit. The script checks each and prints a finding when one applies. How to read its numbers: generation speed
times model size is the memory bandwidth actually used; if that is under about a third of the estimated peak, something other
than the model is the limit; if it is above half, only a smaller model or faster memory will help. *Not yet run, and the
script has not been executed anywhere (no PowerShell in the authoring environment), so expect to fix a typo or two.* What it
was designed to do: On the host and inside the LLM container record: CPU
model and flags (AVX2, AVX-512), memory type, number of populated channels and speed, threads given to the service against
cores available, any Docker CPU or memory limit, and a `llama-bench` run of the current model (prompt processing and
generation separately). Also split a real slow answer into prompt time and generation time. Outcome: either "the box is
mis-configured, fix it" or "it is the hardware, the ceiling is X". Everything after depends on this.

**M1. Spend fewer tokens.** Measure, over the existing question corpus, how many answers need the model at all, how long the
prompts are, and where grammar-constrained planning could be shortened. Cache identical plans. This is model-agnostic and
helps on any hardware.

**M2. Model and quantisation bake-off (already under way).** Compare Qwen3-4B Q4_K_M, Qwen3-4B Q4_0 and Qwen3-1.7B Q4_0, and at
most one outside candidate (Phi-4-mini or a Gemma small model, licence permitting), on the same fixtures, with accuracy
thresholds fixed before the run, reporting prompt time, generation time, planner validity and abstention behaviour. The
smaller model can only replace the larger one for roles where it meets the accuracy threshold (for example routing and
extraction), never for answers that state facts.

**M3. One speculative-decoding trial** (1.7B draft, 4B target) on our CPU, with a keep-or-drop rule written first: keep only
if median latency improves by a stated margin with identical outputs.

**M4. Hardware decision, only if M0 to M3 leave a gap.** Choose by measured cost per answered question within the latency
target: (a) fix the current host (memory channels, threads), (b) add a network inference node (GPU or unified-memory box)
behind the existing LocalAI remote setup, keeping evidence and database where they are, (c) a larger mixture-of-experts model
on a high-capacity box if answer quality, not speed, is the limit. Evidence handling and air-gap requirements decide whether
a remote node is allowed at all; that is a policy question for the product owner.

**O1 to O4. Ownership and "my own cases" (runs in parallel; backend work, another session).**
- O1: a `cases` table (id, display name, status, created_by, created_at) plus `case_members` (case, subject, role); request 18
  grows to cover it.
- O2: real identity end to end: sign-in produces a subject, the proxy forwards it (replacing the constant
  `investigation-workspace`), the API trusts it only behind authentication, never from a form field.
- O3: backfill rule for existing cases: owner is unknown unless the earlier SQL shows a single real subject; unknown stays
  unknown, never guessed.
- O4: UI: a "My cases" filter and an owner column, shown only when the data is real; until then the UI says ownership is not
  recorded.

### Decisions needed from the product owner

1. Is a network inference node acceptable under the evidence-handling policy (M4)?
2. Which outside model licences are acceptable for evaluation (M2)?
3. Should ownership (O1 to O4) be scheduled before or after the model work? They do not depend on each other.
4. Approval to run the L1b arms and the pre-registered Q4_0 speed test, and a thread-count test (these touch model config).
5. Does the laptop (Lenovo 21BVS0QX00) have a free second SODIMM slot? A matching DDR4-3200 module would give dual-channel memory.

## M0 results (measured 2026-10-02, four runs)

Single-channel DDR4-3200 (peak about 25.6 GB/s): decode 6.9 to 9.0 tok/s, 63% to 82% of peak, so memory-bound. Threads (8), power
plan, container limits and memory pressure are not limiting. Prefill is 38 to 48 tok/s. The prompt cache works: a 2,301-token
prefix took 60.67 s cold and 1.04 s on each later request. The planner prompt layout defeats prefix caching beyond the system
prompt; the proposed fix and its pre-registered thresholds are in `L1B_SHARED_PREAMBLE_SPEC_20261002.md` (not implemented, not measured).
