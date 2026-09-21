# NX-B2.1D Q4 32-case development evaluator

This bundle performs development-only multilingual planner evaluation. It does
not certify D, freeze a qualification holdout, deploy a model, replace the Q8
profile, write retained evidence, or activate NX-B2.1.

All JSON, prompt, and corpus inputs are decoded with strict UTF-8 semantics;
an existing input BOM is recognized and stripped. The transport serializes
ordered request objects to UTF-8 without a BOM, sends
the resulting byte array with `HttpClient`, captures response bytes before any
decoding, applies strict UTF-8 decoding, and only then parses the OpenAI envelope
and planner object. Every case retains request/response hashes, exact Unicode
diagnostics, semantic oracle results, latency, and bounded resource snapshots.

Run source checks while Codex is open:

```powershell
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\nxb21d-development\test_nxb21d_q4_32case_framework.ps1
```

For the live run, close Chrome, ChatGPT/Codex, VS Code, and other heavy IDEs.
Leave Docker Desktop running, open standalone Windows PowerShell in the repository,
then run:

```powershell
cd C:\Users\sheik\Workspace\Office\Projects\NexusAI
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\nxb21d-development\run_nxb21d_q4_32case_development.ps1
```

The runner emits `CORPUS_FROZEN=PASS`, `CANDIDATE_FREEZE=PASS`, one
`NXB21D_CASE_RESULT` per executed case, `BYTE_SAFE_TRANSPORT=PASS`,
`RUNTIME_INTEGRITY=PASS`, and a final `NXB21D_Q4_32CASE` marker. Exit code 0
means all 32 development cases passed. Exit 22 means the complete run contained
semantic failures. Exit 10 is a RAM safety stop, 11 is a preflight or integrity
failure, 20 is HTTP/UTF-8 envelope corruption, and 21 is an unexpected failure.

Each timestamped output directory contains `operator.log`, per-case raw evidence
and receipts, `32case-development-summary-v1.json`, and its SHA-256 sidecar.

If Windows PowerShell exits after all 32 case receipts and
`RUNTIME_INTEGRITY=PASS` but before writing the aggregate, recover the summary
without inference:

```powershell
python .\scripts\nxb21d-development\recover_nxb21d_q4_32case_summary.py `
  --repo . `
  --run .\local-acceptance-models\nxb21-d\challengers\qwen3-4b-instruct-2507-q4km\localai-dev-results\32case-development\run-<timestamp>
```

The recovery command refuses incomplete evidence, a run outside the configured
output root, raw hash mismatches, request BOMs, corpus/oracle drift, missing
runtime-integrity evidence, or an existing aggregate receipt. It never performs
model inference.
