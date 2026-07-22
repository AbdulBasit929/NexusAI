# LocalAI Local Functional Test Report

## 1. Executive Summary

| Item | Value |
|---|---|
| Project | LocalAI |
| Tester |  |
| Test date |  |
| Test location | Local laptop |
| LocalAI version/image |  |
| API URL | `http://localhost:8080` |
| Overall result | Pass / Fail / Partial Pass |

Summary:

```text
LocalAI was tested locally across core text, embedding, audio, image/vision, detection, and optional advanced features. The purpose was to validate that each module can be installed, invoked through the API/Web UI, and produce expected output on laptop hardware.
```

## 2. Environment

| Item | Value |
|---|---|
| OS |  |
| CPU |  |
| RAM |  |
| GPU |  |
| Docker version |  |
| Start method | Docker CPU / Docker GPU / Binary / Source |
| Models path |  |
| Generated content path |  |
| Notes |  |

Evidence:

- `localai-test-evidence/phase-00-environment/systeminfo.txt`
- `localai-test-evidence/phase-00-environment/cpu.txt`
- `localai-test-evidence/phase-00-environment/gpu.txt`
- `localai-test-evidence/phase-00-environment/docker-version.txt`

## 3. Test Scope

| Module | Included | Notes |
|---|---|---|
| Server startup and Web UI | Yes |  |
| Model installation/listing | Yes |  |
| Chat/text generation | Yes |  |
| Streaming chat | Yes |  |
| Completions | Yes |  |
| Embeddings/comparison | Yes |  |
| Speech to text | Yes |  |
| Text to speech | Yes |  |
| Voice activity detection | Yes |  |
| Image understanding | Yes / Optional | Depends on installed vision model |
| Image generation | Yes / Optional | May be hardware-heavy |
| Object detection | Yes |  |
| Face detection/embedding/verify | Yes |  |
| Speaker diarization | Optional | May be hardware/model-heavy |
| Sound/music generation | Optional | Requires compatible model |
| Video generation | Optional | Usually hardware-heavy |

## 4. Installed Models

| Purpose | Model name | Installed | Notes |
|---|---|---|---|
| Chat | `llama-3.2-1b-instruct:q4_k_m` | Yes / No |  |
| Embeddings | `qwen3-embedding-0.6b` | Yes / No |  |
| Speech to text | `whisper-1` / `whisper-base` | Yes / No |  |
| Text to speech | `pocket-tts` | Yes / No |  |
| VAD | `silero-vad` | Yes / No |  |
| Object detection | `rfdetr-cpp-nano` | Yes / No |  |
| Face recognition | `face-detect-yunet-sface` | Yes / No |  |
| Vision |  | Yes / No |  |
| Image generation |  | Yes / No |  |
| Diarization |  | Yes / No |  |

Evidence:

- `localai-test-evidence/phase-01-server/models-after-install.json`
- Web UI Models screenshot.

## 5. Results Matrix

| ID | Phase | Feature | Endpoint/UI | Model | Status | Response time | Evidence | Notes |
|---|---|---|---|---|---|---|---|---|
| TC-01.1 | 01 | Model list | `/v1/models` | N/A | Not run |  | `phase-01-server/models.json` |  |
| TC-03.1 | 03 | Chat completion | `/v1/chat/completions` |  | Not run |  | `phase-02-chat/chat-completion.json` |  |
| TC-03.2 | 03 | Streaming chat | `/v1/chat/completions` |  | Not run |  | `phase-02-chat/chat-stream.txt` |  |
| TC-03.3 | 03 | Completion | `/v1/completions` |  | Not run |  | `phase-02-chat/completion.json` |  |
| TC-04.1 | 04 | Embeddings | `/embeddings` |  | Not run |  | `phase-03-embeddings/embeddings.json` |  |
| TC-04.2 | 04 | Similarity comparison | Local Python validation |  | Not run |  | `phase-03-embeddings/similarity-result.txt` |  |
| TC-05.1 | 05 | Speech to text | `/v1/audio/transcriptions` |  | Not run |  | `phase-04-audio/transcription.json` |  |
| TC-05.2 | 05 | Text to speech | `/tts` |  | Not run |  | `phase-04-audio/tts-output.wav` |  |
| TC-05.3 | 05 | Voice activity detection | `/v1/vad` |  | Not run |  | `phase-04-audio/vad-response.json` |  |
| TC-06.1 | 06 | Image understanding | `/v1/chat/completions` |  | Not run |  | `phase-05-vision-images/vision-response.json` |  |
| TC-06.2 | 06 | Image generation | `/v1/images/generations` |  | Not run |  | `phase-05-vision-images/image-generation.json` |  |
| TC-07.1 | 07 | Object detection | `/v1/detection` |  | Not run |  | `phase-06-detection/object-detection.json` |  |
| TC-07.2 | 07 | Face detection | `/v1/face/detect` |  | Not run |  | `phase-06-detection/face-detect.json` |  |
| TC-07.3 | 07 | Face embedding | `/v1/face/embed` |  | Not run |  | `phase-06-detection/face-embedding.json` |  |
| TC-07.4 | 07 | Face verify | `/v1/face/verify` |  | Not run |  | `phase-06-detection/face-verify.json` |  |
| TC-08.1 | 08 | Speaker diarization | `/v1/audio/diarization` |  | Not run |  | `phase-07-advanced/diarization.json` |  |
| TC-08.2 | 08 | Sound generation | `/v1/sound-generation` |  | Not run |  | `phase-07-advanced/sound-output.wav` |  |
| TC-08.3 | 08 | Video generation | `/video` |  | Not run |  | `phase-07-advanced/video-generation.json` |  |

Status key:

- `Pass`: feature worked and output matched expected behavior.
- `Fail`: feature ran but output was incorrect or API failed unexpectedly.
- `Blocked`: could not run due to missing model, hardware limits, disk, network, or dependency issue.
- `Not run`: not attempted yet.

## 6. Detailed Observations

### Server and Web UI

Result:

Notes:

Evidence:

### Chat/Text

Result:

Notes:

Evidence:

### Embeddings and Comparison

Result:

Similarity result:

```text
cat_vs_kitten =
cat_vs_stock =
```

Notes:

Evidence:

### Audio

Speech to text result:

Text to speech result:

VAD result:

Notes:

Evidence:

### Vision and Image Generation

Vision result:

Image generation result:

Notes:

Evidence:

### Detection and Face Recognition

Object detection result:

Face detection result:

Face embedding result:

Face verify result:

Notes:

Evidence:

### Advanced Features

Diarization result:

Sound generation result:

Video generation result:

Notes:

Evidence:

## 7. Defect Log

| Bug ID | Feature | Severity | Status | Issue | Steps to reproduce | Expected | Actual | Evidence |
|---|---|---|---|---|---|---|---|---|
| BUG-001 |  | Low / Medium / High / Critical | Open |  |  |  |  |  |

## 8. Blockers and Risks

| Blocker ID | Area | Reason | Impact | Recommendation |
|---|---|---|---|---|
| BLK-001 |  | Missing model / Hardware limit / Network / Disk / Dependency |  |  |

## 9. Final Recommendation

Choose one:

- LocalAI is ready for local demo/testing for the validated modules.
- LocalAI is partially ready; blocked modules need additional model setup or stronger hardware.
- LocalAI is not ready yet because critical modules failed.

Recommendation text:

```text
Based on local testing, LocalAI successfully validated <list modules>. The following modules were blocked or failed: <list>. Recommended next step is <install models / use GPU machine / fix issue / rerun phase>.
```

## 10. Sign-off

| Role | Name | Date | Comments |
|---|---|---|---|
| Tester |  |  |  |
| Reviewer / Team Lead |  |  |  |
