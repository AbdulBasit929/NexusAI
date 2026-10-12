# tools/real_data

Tooling for decision D1 (real-data handling). The policy is `docs/data/real-data-handling-policy.md`; read it first. Python 3.10 standard library only (no pytest, no third-party packages), PowerShell 5.1 for the vault scripts.

| File | What it does |
|---|---|
| `reserved.py` | Reserved ranges (documentation IPs, test network, test TAC, invalid CNIC prefix, project mobile-number block), Luhn check, `canonical_msisdn`, and the generators `synthetic_msisdn/imsi/imei/ipv4/ipv6/cnic` |
| `guard.py` | Location checks: refuses a git work tree, a cloud-synchronised folder, a scratch or Downloads folder |
| `profile_structure.py` | Structure-only profile of CSV/TSV/UTF-16 text, XLSX and SpreadsheetML: shareable or local report, never the data |
| `reconcile_aggregates.py` | `emit-sql` prints the read-only aggregate query for loaded rows; `compare` checks a local profile against it, aggregates only |
| `scan_for_real_data.py` | Fails a commit or push that adds real-looking numbers, identifiers, public IPs, media or secrets; masked output |
| `Initialize-Vault.ps1` | Creates the vault folder with restricted access and reports encryption; `-WhatIf` supported |
| `Register-Sample.ps1` | Registers one real file in the vault together with its authority; refuses without it; `-WhatIf` supported |
| `test_real_data_tools.py` | 31 tests on synthetic files (`python -I -X utf8 tools/real_data/test_real_data_tools.py`) |
| `Test-VaultTools.ps1` | 13 checks of the two vault scripts on a temporary folder |

Run from the repository root. Use `python -I -X utf8` for anything that reads a downloaded or real file (`-I` ignores the environment and the current directory, `-X utf8` avoids code-page errors on Urdu or Chinese text).

## Optional allowlist

`tools/real_data/allowlist.json` is read by the scanner if it exists (it is not created by default; `allowlist.local.json` is ignored by git for private additions):

```json
{
  "msisdn": [],
  "ips": [],
  "paths": ["tests/fixtures/media/"],
  "ignore_paths": ["reports/some-generated-folder/"]
}
```

`msisdn` and `ips` list synthetic values that are not in a reserved range; `paths` are folders where newly added media or archives are expected (the `file.media_or_archive` review is skipped); `ignore_paths` are skipped entirely. Prefer the reserved ranges and the `real-data-ok` line marker over this file.

## What is deliberately not here

No code that reads a real file into a model, a cloud service or a repository. No real value, in tests or documents: values that look real are assembled at run time in the tests so the scanner does not flag its own test file.
