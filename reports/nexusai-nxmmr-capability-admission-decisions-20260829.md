# NX-MMR capability admission decisions

| Capability | Decision | Evidence boundary |
|---|---|---|
| printed English OCR | limited fixture leader: Paddle | 5/6 exact versus Tesseract 3/6; real-world data required |
| printed Urdu OCR | keep limited Tesseract floor | one sample only; neither exact; Tesseract lower CER/WER |
| handwritten English/Urdu OCR | defer | no independent data |
| Qwen3 embedding baseline | keep current baseline | 0.8182 Recall@1, 1.0 Recall@5; no-answer failure remains |
| Qwen3 reranker challenger | reject admission | 0.7273 Recall@1, 397.4 s, 3 GiB cap, one timeout retry |
| Pakistan image ANPR | retain incumbent as limited evidence; no promotion | holdout detection 20/22, exact 13/22, CER 0.201439; no negative images/boxes |
| sample.mp4 ANPR | reject current video policy for promotion | event detection 3/19, exact 3/41, grouping 4/29 |
| English/Urdu ASR | not admissible on this pack | all six human speech windows negative; no transcript; no model call |
| Roman Urdu ASR representation | defer; secondary only | raw transcript must remain authoritative |
| face candidate | retain current candidate-only limitation | no identity inference or new model benchmark |
| image semantic / open-vocabulary vision / VLM | defer | no admitted model/data evidence |
| TTS / diarization | defer | not benchmarked or required for baseline |
| evidence route plan | fixture-certified shadow only | 16/16 matrix; activation not approved |
| query/answer operation promotion | none | no new independent real-world integrated oracle |

No model was installed live. No capability/profile/backend catalogue was
mutated. Rejected means the specific reranker challenger is not justified by
this fixture; it does not ban future challengers under a new bounded approval.
