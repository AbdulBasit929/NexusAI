# NX-B2.1D phrase-literal remediation

This bundle refreezes the unchanged Qwen3 4B Q4 model, profiles, planner prompt
and schema with deterministic authority for one explicitly quoted phrase.

The authority uses the existing product parser. It applies only when exactly
one quoted source span and one matching document or transcript family hint are
present. The original characters and codepoints become the final execution
literal. The model literal, clarification, codepoints, normalization and script
mismatch remain separately auditable. Multiple phrases or contradictory family
context require clarification. Unquoted prose is not captured heuristically.

The earlier atomic plate authority remains unchanged. The fresh development
corpus contains 20 cases: eight document phrases, eight transcript phrases and
four selected-image plate controls. Each language has five cases; phrase scope
is balanced eight selected and eight workspace cases.

Run source checks:

```powershell
python .\scripts\nxb21d-phrase-remediation\validate_nxb21d_q4_phrase_remediation_corpus.py --repo . --corpus .\scripts\nxb21d-phrase-remediation\nxb21d-q4-phrase-remediation-20case-development-corpus-v1.json
.\scripts\nxb21d-phrase-remediation\test_nxb21d_q4_phrase_remediation_framework.ps1
```

After source validation, the standalone operator may execute the development
corpus once:

```powershell
.\scripts\nxb21d-phrase-remediation\run_nxb21d_q4_phrase_remediation_20case_development.ps1
```

This is development evidence only. It does not freeze a qualification holdout,
complete NX-B2.1D, authorize activation, deploy services or permit reuse of any
earlier corpus.
