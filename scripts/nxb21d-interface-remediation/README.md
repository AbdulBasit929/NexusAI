# NX-B2.1D production-interface remediation development gate

This bundle runs 24 fresh development-only cases through the exact production `resolveWithLanguageAssistance` interface. It uses the full production schema, current capability filtering, strict JSON-schema response format, deterministic authorities, byte-preserving request/response capture, and the frozen 512-token completion budget.

The corpus is not a qualification holdout. A passing result does not classify Q4 suitability, complete D, authorize activation, or authorize creation of a replacement holdout.

Run from Windows PowerShell at the repository root:

```powershell
Set-ExecutionPolicy -Scope Process Bypass -Force
.\scripts\nxb21d-interface-remediation\run_nxb21d_q4_interface_remediation_24case_development.ps1
```

The runner validates all frozen hashes and runtime identities before inference. It requires 6 GiB available RAM when Q4 is unloaded and 4 GiB when Q4 is already loaded. It never rebuilds, recreates, restarts, prunes, migrates, downloads, or activates anything.

Each model-call directory contains the exact request bytes, raw response bytes, strict UTF-8 envelope, inner proposal, and case result. The aggregate receipt records structural, semantic, token-budget, latency, resource, deterministic-authority, runtime-integrity, retained-tuple, and Activity evidence.

Success ends with `NXB21D_Q4_INTERFACE_REMEDIATION_24CASE=PASS`. Preserve the run directory and return its aggregate receipt to Codex. Do not generate a replacement qualification holdout afterward without a separate review and refreeze.
