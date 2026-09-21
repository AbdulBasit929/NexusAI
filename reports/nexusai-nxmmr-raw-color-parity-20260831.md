# Outcome

The authorized two-arm development-only raw-color preprocessing slice is
complete. Both arms pass their synthetic smoke, empirical development run,
numeric utility gates, resource recording, deterministic score recomputation
and upstream-parity audit. The pre-registered decision is
**`RAW_RGB_SELECTED`** because its scores equal Raw-BGR, it satisfies the
recognizer's declared RGB contract, and Raw-BGR shows no material advantage.

The selected implementation is frozen as the **V2.2 development candidate**
with digest
`b0caae6d646e9d188ec5bc387585ac43aa7f1e22504d5b2d741699bab8d9f394`.
This is not `PRODUCT_CERTIFIED`, not live, and not reserved-validated. Reserved
evaluation count remains zero. No model output was used as an oracle.

# Scope and Preservation

Exactly two modes were evaluated:

- `RAW_BGR_PARITY`: the unchanged OpenCV HWC uint8 BGR source crop, supplied
  directly to a recognizer whose declared input color convention is RGB. This
  is recorded as `DECLARED_COLOR_DEVIATION`.
- `RAW_RGB`: the same crop with only a BGR-to-RGB channel permutation. There is
  no spatial, intensity, threshold, enhancement or denoising transform. This is
  recorded as `OCRInputContract=PASS`.

Everything else remained fixed: the exact runtime and model files, 4 FPS actual
frame scheduling, 640 detector input, detector confidence 0.25, required vehicle
containment, one-second association, support >=3, best OCR confidence >=0.5,
weighted support >=1.5, length 4-10, clustering, neutral normalization and
actual-frame source times.

The original V2 digest
`212c00970d7bed1c40612102c11f1a0dd24c43435c406df83d80bd8f40117b20`
remains `FUNCTIONAL_OCR_INPUT_CONTRACT_FAIL`. V2.1 remains
`FUNCTIONAL_PASS_DEVELOPMENT_UTILITY_FAIL` and unfrozen. Neither source nor its
receipts were rewritten. The image ANPR and Paddle/OCR states are unchanged.

There was no third arm, repeat run, reserved inference, new model or dataset,
package installation, service deployment/recreation, retained-evidence access,
Activity mutation, database/volume mutation, or NX-B2.1 work.

# Pre-registration and Test Gates

Registration was signed before model inference. Registration SHA-256 is
`c475133ed13312f5f43e706a43ea779f3c0982e45f2fae11721ed1c380680ce2`.
It binds the two modes, source files, prior receipts, independent development
oracle projection, split, exact runtime, thresholds, ranking and conservative
Raw-BGR materiality rule.

The materiality rule required all of: exact-F1 gain >=0.05, at least two extra
exact TPs, exact-TP advantage distributed across at least two events, upstream
parity and deterministic rescoring. If Raw-BGR did not meet that rule, an
eligible Raw-RGB arm was preferred. This convention is a bounded selection rule,
not a statistical-equivalence test.

Unit/contract results were **68 passed**: 20 raw-color/V2.1 offline tests and 48
prior host contract tests. Compilation, whitespace and anti-hardcode checks
passed. Both synthetic smokes passed before either development run. Each smoke
loaded both detectors and the pinned recognizer, performed a real zero-frame
detector call, then used synthetic boxes only to exercise the actual crop and
OCR adapter. The same synthetic BGR crop SHA was
`b40ec3d0517fdecdefde7ffe1294d5fae47b1a4b186eaac992895a96d84e1b6e`;
the RGB-supplied permutation SHA was
`9295e312e6d9778c933407f7af6732678c6d44426f3c31f050cb8aef13980a32`.
No synthetic OCR text was scored or treated as truth.

# Exact Runtime

Both sequential development runs used the same immutable, network-disabled
Linux x86_64 CPython 3.11.15 runtime. The base image digest is
`sha256:be13dbec4bd9bb62c824140df3777b898d7ad2801a0e68cd512c00dafaf4f9ef`;
interpreter SHA-256 is
`92d7e40ec50be176cb1b790c7568b7e08cd862137b5aa69f1413ba1967886b79`;
the 64-entry top-level lock SHA-256 is
`3880bdce39914f53537d4e231ef83348297acaeef635f889eb02104ff0c7c1b8`.
All 77 effective distributions matched the admitted import fingerprint.

Core versions were Torch 2.5.1+cpu, Torchvision 0.20.1+cpu, Ultralytics
8.0.114, NumPy 2.3.5, FastALPR 0.4.0, FastPlateOCR 1.1.0 and
open-image-models 0.6.0. No package or runtime changed. The admitted `pip check`
PASS remains applicable because there were no dependency changes.

Model/config SHA-256 values remained:

| Asset | SHA-256 |
|---|---|
| Plate detector | `8ec3b254a6c87610f037a90957462cafa11a9c03224e33a28c6a1d1ac2ac51b0` |
| Vehicle detector | `31e20dde3def09e2cf938c7be6fe23d9150bbbe503982af13345706515f2ef95` |
| OCR model | `8031afb5fdc6b4d80462c9d542f1284ebd2cfddf5dbacd62609848d7e2855f44` |
| OCR config | `0335c74a305173bb6f393efed0fde03cadeaa0b649ed8e19f431016d8232d0a6` |

# Development Evidence Boundary

Both arms used the same independently validated development projection: 11
human events, 24 truth occurrences and 17 unique normalized truth strings. The
projection SHA-256 is
`d8fa82c295b8325c020092d024d58332b42efeae4f8afca80f5a46b2338a6c06`;
the split digest is
`a759572ba657d3385267abe06f3ae5f756708001759903b040b2d7741ad79a1b`.
Each arm analyzed 133/133 scheduled actual frames from the 60-second, 60-FPS,
3,600-frame source, made 222 OCR calls, had zero failed frames and therefore
covered 3.6944% of source frames. No selected frame intersected a reserved
exclusion. Reserved labels were not read.

# Upstream Parity

The saved-output audit established:

| Frozen upstream input | Audit |
|---|---|
| Source frames, ordering and source-frame hashes | PASS |
| Plate/vehicle boxes, confidence values and classes | PASS |
| Vehicle-containment decisions | PASS |
| Crop bounds, shapes and source BGR crop hashes | PASS |
| Primary scorer recomputation | PASS |
| Stage-FPR recomputation | PASS |

The common upstream trace digest is
`89dd74abc4bc70af2bd7232ddbb76babf49506ebbe944e5358b616c39117f6f8`.
Thus the controlled difference begins only at the OCR array color convention.

# Complete Development Metrics

The primary metrics are identical between arms:

| Metric | Raw-BGR parity | Raw-RGB |
|---|---:|---:|
| Event TP / FN | 11 / 0 | 11 / 0 |
| Event recall | 1.000000 | 1.000000 |
| Event recall Wilson 95% | 0.741167-1.000000 | 0.741167-1.000000 |
| Count-proxy TP / FP / FN | 20 / 0 / 4 | 20 / 0 / 4 |
| Count-proxy precision / recall / F1 | 1.000000 / 0.833333 / 0.909091 | 1.000000 / 0.833333 / 0.909091 |
| Exact TP / FP / FN | 19 / 2 / 5 | 19 / 2 / 5 |
| Exact precision / recall / F1 | 0.904762 / 0.791667 / 0.844445 | 0.904762 / 0.791667 / 0.844445 |
| Truth-occurrence exact accuracy | 0.791667 | 0.791667 |
| Exact-accuracy Wilson 95% | 0.595295-0.907552 | 0.595295-0.907552 |
| Normalized CER | 0.178571 | 0.178571 |
| Group TP / FP / FN | 15 / 1 / 2 | 15 / 1 / 2 |
| Group precision / recall / F1 | 0.937500 / 0.882353 / 0.909091 | 0.937500 / 0.882353 / 0.909091 |
| Predicted unique strings / temporal packets | 16 / 16 | 16 / 16 |
| Final false groups | 1 | 1 |

Event recall here means at least one selected prediction in the human event; it
is not boxed detector IoU. Count-proxy and group metrics do not establish
physical vehicle identity. Raw-format exact accuracy is 0 for both arms because
the scorer's authoritative exact metric uses neutral normalization.

# False Positives and Misses

| Stage | Raw-BGR parity | Raw-RGB |
|---|---:|---:|
| Raw detector false negative-window frames | 61/72 (0.847222) | 61/72 (0.847222) |
| OCR-candidate false negative-window frames | 59/72 (0.819444) | 59/72 (0.819444) |
| Group-selected false negative-window frames | 47/72 (0.652778) | 47/72 (0.652778) |
| False packets by string and actual support time | 1 | 1 |
| False packet strings independent of time | 1 | 1 |

Raw detector and OCR-candidate rates are frame-presence rates, not per-box
precision. The packet-time match requires the selected string in a human event
and at least one actual supporting observation in that event; it does not infer
vehicle identity.

At the event level, raw exact OCR appeared in both arms for all 11 events:
both=11, BGR-only=0, RGB-only=0, neither=0. Across 24 truth occurrences:
both=20, BGR-only=0, RGB-only=0, neither=4.

For each arm, 19 truth occurrences were exactly selected, four never generated
an exact raw OCR result, and one generated an exact raw result that aggregation
did not retain. Neither arm had an event with no raw OCR candidate or no selected
packet. Raw-BGR produced 216 nonempty raw observations and Raw-RGB 218; the two
extra RGB observations did not change exact, group or FPR utility. The dominant
remaining error is therefore recognizer generation on four truth occurrences,
with a smaller one-occurrence aggregation-retention gap.

# Source-Time Measurements

Both arms produced the same selected groups and therefore the same 15 matched
group boundaries:

| Source-time metric | Both arms |
|---|---:|
| Matched groups | 15 |
| Mean absolute boundary error | 0.786367 s |
| Median absolute boundary error | 0.512000 s |
| p95 absolute boundary error | 2.877050 s |
| Mean signed first-seen error | +0.161133 s |
| Mean signed last-seen error | +0.006933 s |

These metrics exclude missed groups and describe sampled source-time boundaries,
not streaming wall-clock latency. Against the historical raw-BGR development
reference (MAE 0.778033 s, median 0.512 s, p95 2.764550 s), the changes are
+0.008334 s MAE, 0 median and +0.112500 s p95. There is no catastrophic clock,
frame-index or source-time regression, so the qualitative timing gate passes.

The event-result latency summaries were Raw-BGR mean/p50/p95/max
1.065006/0.689349/2.305197/3.147397 s and Raw-RGB
1.136823/0.808702/2.738005/3.194920 s. They are benchmark result latencies, not
certified live throughput.

# Resource Measurements

| Measurement | Raw-BGR parity | Raw-RGB |
|---|---:|---:|
| Pre-run host available RAM | 4.147266 GiB | 3.424732 GiB |
| Minimum host available RAM | 3.695625 GiB | 2.834660 GiB |
| Final host available RAM | 3.846279 GiB | 2.904530 GiB |
| Processing/scoring wall | 375.605049 s | 364.998796 s |
| Finalized process wall | 375.942957 s | 365.304456 s |
| Wall including container startup | 380.855043 s | 368.207864 s |
| Processing/scoring CPU | 1049.092239 s | 1031.631567 s |
| OS process peak RSS | 801.507813 MiB | 803.886719 MiB |
| Sampled process peak RSS | 803.453125 MiB | 805.648438 MiB |
| Incremental process-tree peak | 775.207031 MiB | 777.378906 MiB |
| Incremental peak + 1.5-GiB headroom | 2.257038 GiB | 2.259159 GiB |
| Safety stop / container exit / failed frames | No / 0 / 0 | No / 0 / 0 |

Both runs stayed inside the fixed 3-GiB, four-CPU, one-heavy-evaluator policy.
The small resource differences do not alter selection or reduce the admission
policy. The separate live/build 6-GiB constraint remains unchanged.

# Selection and Freeze

Both arms pass every registered numeric gate:

| Gate | Required | Both arms |
|---|---:|---:|
| Event recall | >=0.75; strong >=0.85 | 1.000000 PASS/strong |
| Group precision | >=0.80 | 0.937500 PASS |
| Group F1 | >=0.65 | 0.909091 PASS |
| Exact F1 | >=0.60 | 0.844445 PASS |
| Catastrophic source-time regression | None | None PASS |

Raw-BGR exact-F1 delta, extra exact TP and distributed event advantage checks
are all false. It therefore has no material advantage. Because Raw-RGB is
eligible, metric-identical and contract-conforming, the only admissible
pre-registered selection is **`RAW_RGB_SELECTED`**. Installed FastPlate source
inspection was not triggered because that inspection was required only before
admitting a material Raw-BGR win.

The configuration is
`configuration/nxmmr_video_anpr_product_baseline_v22.json`, SHA-256
`e379d3561b307de4f8e44ea2833cdade9b87b067f5332c427ad268cdda2e2e9a`.
The source freeze is
`video-v22-product-source-freeze.json`, file SHA-256
`84350341c05a5bf5d01a2ed8d44295b260d6cf39754099b91e9fca20ff71ddd5`.
Its canonical candidate digest is
`b0caae6d646e9d188ec5bc387585ac43aa7f1e22504d5b2d741699bab8d9f394`.
The comparison receipt SHA-256 is
`39475cab20269c7f285a288d57f64ecc550c8134a198f23435dc689f27c182b6`.
Selected Raw-RGB development receipt SHA-256 is
`76f90ebc6cf56a338cf594597ac571b854a694cc7aed50805045844054a16ef3`;
nonselected Raw-BGR receipt SHA-256 is
`224a93dd27ceea193277e8540079031bd4ce49ff1e7af16e943aeaed63c9f1e9`.

# Certification Consequences and Remaining Gaps

V2.2 is now **development-frozen and reserved-unevaluated**. The result repairs
the V2 functional OCR input contract and recovers the strong development utility
seen in the raw-color baseline without relying on a declared-color deviation.
It does not certify generalization, production behavior or licensing.

Remaining evidence/model gaps are:

- the sealed reserved video split has not been evaluated for this new digest;
- the small single-video development comparison cannot establish statistical
  equivalence or broad Pakistani plate/domain generalization;
- four of 24 truth occurrences never received an exact raw recognition and one
  exact raw candidate was not retained, so recognizer and aggregation gaps remain;
- raw/OCR/group negative-window frame rates remain high and need reserved
  confirmation plus manual packet review;
- no boxed human detector oracle exists here, so localization IoU/AP is not
  certified;
- resource results are a bounded offline run, not concurrent streaming capacity;
- model/config licensing remains an independent admission gate;
- live API, Ask, Data, citation, Activity, UI and manual product-acceptance gates
  remain open.

No direct promotion to `PRODUCT_CERTIFIED` occurred.

# Exact Next Action

**Stop now and obtain explicit authorization for one one-time sealed-reserved
evaluation of exactly the frozen V2.2 digest
`b0caae6d646e9d188ec5bc387585ac43aa7f1e22504d5b2d741699bab8d9f394`.**
That authorization must forbid tuning, repeats, challenger substitution and live
activation, and must require complete reserved event/exact/CER/group/FPR/time and
resource reporting. Until that authorization is granted, do not read reserved
labels or run any additional model inference.
