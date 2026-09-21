# NX-MMR dataset admission plan

Research completed 2026-08-29. Admission is purpose-specific: permission to
evaluate does not grant permission to train, redistribute, display in a demo,
identify a person, or promote a production model.

## Existing local evidence

| Candidate | Exact extent | Admission decision |
|---|---:|---|
| `sample.mp4` | 184,407,144 bytes; SHA-256 `d470773444dd23bc4b8d446e3fb1e057923e9902b2a2a93e9fafc83762d137ee` | `AUTHORIZED_BENCHMARK_COMPLETE_NONRETAINED`; locked human events used once; no redistribution or product promotion |
| Pakistani License Number Plates Data | 108 JPG, 50,377,210 bytes | `AUTHORIZED_BENCHMARK_COMPLETE_NONRETAINED`; 10 development plus 22 sealed images independently labeled; raw data remains private |
| retained NexusAI workspace | `24|24|9275|426|22`, Activity 20 | read-only replay allowed; fresh processing/ingest remains separately gated |
| repository fixtures | existing controlled packs | admitted for source tests; never sufficient for real-world/product certification |

The local image pack should be labeled in place through a separate hash-only
manifest. Do not copy it into Git and do not publish plate strings, faces or
contextual vehicle imagery in reports.

## Fresh public-dataset research

| Dataset | Verified scope | License/privacy | Decision |
|---|---|---|---|
| [P-LPCD v1.0.0](https://zenodo.org/records/17182320) | 1,235,177,299-byte ZIP; MD5 `293f7c2497d13f53ab282b630a362603`; 40,000 synthetic + 650 real crops | Zenodo says CC BY 4.0 while associated GitHub says CC BY-NC 4.0; real plates; metadata enumerates 37 labels while saying 36 | `DEFER`; do not download until publisher/license and schema conflict are resolved |
| [Common Voice 26.0 Urdu](https://commonvoice.mozilla.org/data) | full MP3 release 5.79 GB | CC0-1.0 plus Mozilla terms; no speaker-identification attempt | `DEFER_FULL`; design a bounded, exact-byte clip subset first |
| [Urdu Handwritten Text Dataset](https://data.mendeley.com/datasets/bg2sctsysf/1) | v1, DOI `10.17632/bg2sctsysf.1`, 273 files reported by the publisher listing | CC BY 4.0; includes demographic data | admissible in principle, but `DEFER` until exact archive bytes/checksum and privacy-minimized file selection are resolved |
| [UTRNet / UTRSet](https://github.com/abdur75648/UTRNet-High-Resolution-Urdu-Text-Recognition) | real and synthetic printed-Urdu research resources | research-only / CC BY-NC-SA; UrduDoc needs agreement | research reference only; not product-promotion evidence |

The associated P-LPCD paper reports strong results, but paper accuracy is not
NexusAI accuracy. Local independent evaluation is mandatory.

## Admission record required for every dataset

- immutable dataset/version and publisher URL;
- exact selected file names, byte sizes and publisher/local hashes;
- license text and any terms or access agreement;
- allowed uses: evaluation, training, display, redistribution and commercial;
- personal/sensitive data classification and minimization;
- independent ground-truth schema and reviewer ownership;
- development/validation/sealed-holdout split;
- storage, encryption, retention and deletion plan;
- report redaction rules;
- explicit status: proposed, acquired, verified, evaluated, promoted, rejected,
  or deleted.

## First approval boundary

The initial approval request does not include a public dataset download. It
asks only for non-retained processing of the two exact local sources after the
operator attests lawful use and privacy suitability. This avoids a 1.15 GiB
P-LPCD transfer and a 5.79 GB Common Voice transfer before smaller evidence has
shown a real gap.

Public acquisition can return as a later exact manifest. An approximate size,
landing page, paper citation, or dynamic “download all” link is not enough.
