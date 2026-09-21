# NexusAI BF-A retained evidence checksum manifest

Generated: 2026-08-23  
Tenant: `default`  
Collection/case: `nexusai-breadth-acceptance-20260820`  
Retention: review hold; cleanup is not authorized

This manifest records the SHA-256 values and current immutable version IDs
stored by the evidence control plane. It is an accounting export; it does not
copy, alter or delete retained evidence.

| Source | Evidence ID | Current version ID | Bytes | SHA-256 | Current/latest job |
| --- | --- | --- | ---: | --- | --- |
| `test_plate.jpg` | `343b1e62-9f24-488c-a9a0-8d79ff91ee1c` | `c5398b10-639b-4eed-bdc3-aca6a4b55a3a` | 235572 | `a15a7ec947fdb2c5a08a70243a62ed66e4dddd89defaba9d135f6c5ac67559f6` | `88df3883-6c4e-44a2-ac54-704e10d3d26f`, generation 1, completed |
| `wikimedia-road-cars-islamabad.jpg` | `22c74bf9-0fc1-479d-816b-311106ea707b` | `6eb10eed-a8a5-439e-9b6f-96180f684ce8` | 1430057 | `ba1aa5a7e28e18bdd78b2cba132a6fe1cd87612d930be91fa6db22194cae0038` | `831c9b62-4872-4302-a8e5-c2cf7f27042b`, generation 1, completed |
| `clear-urdu.wav` | `fb025351-f86b-405f-bbf0-1b5287d7bf8b` | `d8a5e346-4f1a-42a8-b2e3-a881822a4225` | 199758 | `c499e06e26b8cbf37386b88bba26f37045439a775e964bdad8634376435a056e` | `b74b8502-1741-4e66-bde9-d0d5a520da0e`, generation 0, completed |
| `conversational-urdu.wav` | `8c49950b-a91d-49cb-813b-3ff639aa9462` | `691e33c0-f03a-401a-85df-7c02a670b579` | 336078 | `f016076b153c2ed6be21fa5f7a4bbc22b3e391d40080b9fa3de3b8a2dd517550` | `a93943d4-5f42-4530-b20e-510cb9e22e75`, generation 0, completed |
| `urdu-english-identifier.wav` | `88a9962d-24a0-4e3a-86be-c2a6db6e244f` | `e827d3e4-2d18-4782-b963-249356fa35af` | 248992 | `606ffc7e281940b1b175fff98169a83a6a6ab7744ffd49f05849ac94318fd314` | `c7f148c4-47ae-42ba-8f28-a87e22f0422f`, generation 0, completed |
| `pakistan-short-video.mp4` | `0dcb49a7-bd47-4b63-b4a2-7d67e2caffb0` | `5608c3ca-48f8-4739-80de-600c882d25dd` | 477174 | `94be8b97d62f28eb0349b1eaea02e8073a669d60ffbd3da6511f7f7d27004a26` | `cb1bb9a2-7abc-46a1-9ee3-748632caafb2`, generation 0, completed |
| `HP_EliteDesk_LocalAI_Remote_Setup_Guide.docx` | `2291d02e-e383-41d1-99dd-87d698a38210` | `72c2440f-59ec-495e-b32a-f45395851a01` | 34352 | `85b1ab53150a556f84391a222b7fb4dbf69aa80d1c4d7d7a7b54d5f283ea4865` | no media job; registered, document extraction pending |

Total source bytes: `2961983`  
Evidence/version pairs: `7/7`  
Latest media generations completed: `6/6`  
Preserved historical dead-letter jobs: `2`  
Derived artifacts: `15`  
KB mirrors: `7`  
Canonical structured records in scope: `0`

Worker image provenance:

- active: `nexusai/forensic-records-worker:phase3-runtime`,
  `sha256:a4a4c228ccc6942bd37b415a27245410377a6aed78ecc19e3a9a6e16c2094307`
- rollback: `nexusai/forensic-records-worker:rollback-before-bfa-media-completion-20260821`,
  `sha256:99f2858a0a78dec531eadd0373d57c67c302373fad6e156331b4be9527882808`
