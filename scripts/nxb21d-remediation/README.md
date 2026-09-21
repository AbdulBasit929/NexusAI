# NX-B2.1D atomic identifier remediation

This bundle refreezes the unchanged Qwen3 4B Q4 model, profile, planner prompt,
and schema with deterministic exact-identifier authority at the shared typed
validation boundary. It addresses the development-only `ISB-5907` split seen
in the completed 32-case receipt without changing or rerunning that candidate.

The authority applies only to `image.plate` plus `EXACT_VALUE`. It extracts an
exact source substring, requires exactly one bounded identifier, records any
model mismatch, and supplies one atomic literal/entity to the final typed plan.
No match or multiple matches requires clarification. Phrase capabilities pass
through unchanged.

The fresh 16-case corpus contains eight multilingual plate cases and eight
phrase controls. All plate cases use `selected_evidence`, because the frozen
query-capability contract does not permit workspace-wide ANPR sightings.

Run source-only checks first:

```powershell
python .\scripts\nxb21d-remediation\validate_nxb21d_q4_remediation_corpus.py --repo . --corpus .\scripts\nxb21d-remediation\nxb21d-q4-remediation-16case-development-corpus-v1.json
.\scripts\nxb21d-remediation\test_nxb21d_q4_remediation_framework.ps1
```

The operator may then execute the fresh development corpus exactly once using:

```powershell
.\scripts\nxb21d-remediation\run_nxb21d_q4_remediation_16case_development.ps1
```

This is development evidence only. It does not freeze a qualification holdout,
complete NX-B2.1D, authorize activation, deploy services, or reuse the consumed
168-case Q8 holdout.
