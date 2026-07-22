# LocalAI Local Functional Test Plan

This document is a practical checklist for testing LocalAI features on a laptop and presenting the results to a team lead. It focuses on basic end-to-end functionality with the smallest reasonable models first.

## Scope

Test these areas one by one:

| Area | Endpoint/UI | Recommended first model | Expected result |
|---|---|---|---|
| Server health and model list | `/v1/models`, Web UI | Any installed model | Server responds and installed models are listed |
| Chat / text generation | `/v1/chat/completions` | `llama-3.2-1b-instruct:q4_k_m` | Assistant returns a short answer |
| Streaming chat | `/v1/chat/completions` with `stream:true` | Same chat model | Server-sent event chunks arrive |
| Embeddings and comparison | `/embeddings` | `qwen3-embedding-0.6b` | Similar sentences have higher cosine similarity |
| Image understanding | `/v1/chat/completions` with `image_url` | A small LLaVA/Qwen-VL model if available | Model describes image contents |
| Image generation | `/v1/images/generations` | `flux.1-dev-ggml` or a smaller gallery SD model | Image URL/base64 is returned |
| Speech to text | `/v1/audio/transcriptions` | `whisper-1` / `whisper-base` | Audio transcript is returned |
| Text to speech | `/tts` or `/v1/audio/speech` | `pocket-tts` or `omnivoice-cpp` | WAV/MP3 audio file is generated |
| Voice activity detection | `/v1/vad` | `silero-vad` | Speech time segments are returned |
| Object detection | `/v1/detection` | `rfdetr-cpp-nano` | Bounding boxes/classes are returned |
| Face detection/recognition | `/v1/face/*` | `face-detect-yunet-sface` | Face box, embedding, or verify response is returned |
| Speaker diarization | `/v1/audio/diarization` | `pyannote-diarization` or `vibevoice-diarize` | Speaker-labelled segments are returned |

## Hardware Notes

Use this ordering for a laptop demo:

1. Light tests: health, chat with a 1B model, embeddings, VAD.
2. Medium tests: Whisper transcription, TTS, face detection, object detection.
3. Heavy tests: image generation, vision-language models, diarization, video generation.

If a heavy test fails because of RAM, VRAM, disk space, or model download size, record it as `Blocked by hardware/resource limit`, not as a LocalAI functional failure.

## Environment Record

Capture this before testing:

| Item | Value |
|---|---|
| Tester |  |
| Date |  |
| Laptop CPU |  |
| RAM |  |
| GPU |  |
| OS |  |
| LocalAI version/image |  |
| Start method | Docker / binary / source |
| API URL | `http://localhost:8080` |
| Models directory |  |
| Notes |  |

Useful Windows PowerShell commands:

```powershell
systeminfo | findstr /C:"OS Name" /C:"OS Version" /C:"Total Physical Memory"
wmic cpu get name
wmic path win32_VideoController get name
docker --version
```

## Start LocalAI

Docker CPU baseline:

```powershell
New-Item -ItemType Directory -Force localai-models
New-Item -ItemType Directory -Force localai-generated
$modelsPath = Join-Path (Get-Location) "localai-models"
$generatedPath = Join-Path (Get-Location) "localai-generated"
docker run --name local-ai --rm -it `
  -p 8080:8080 `
  -v "${modelsPath}:/models" `
  -v "${generatedPath}:/tmp/generated/content" `
  localai/localai:latest-cpu
```

NVIDIA GPU baseline:

```powershell
New-Item -ItemType Directory -Force localai-models
New-Item -ItemType Directory -Force localai-generated
$modelsPath = Join-Path (Get-Location) "localai-models"
$generatedPath = Join-Path (Get-Location) "localai-generated"
docker run --name local-ai --rm -it `
  --gpus all `
  -p 8080:8080 `
  -v "${modelsPath}:/models" `
  -v "${generatedPath}:/tmp/generated/content" `
  localai/localai:latest-gpu-nvidia-cuda-12
```

Open the Web UI:

```text
http://localhost:8080
```

Install models from the Web UI Models page, or use the `local-ai` CLI if you installed the binary locally:

```powershell
local-ai models install llama-3.2-1b-instruct:q4_k_m
local-ai models install qwen3-embedding-0.6b
local-ai models install whisper-base
local-ai models install silero-vad
local-ai models install rfdetr-cpp-nano
local-ai models install face-detect-yunet-sface
local-ai models install pocket-tts
```

Use the exact model name shown in `/v1/models` if a gallery alias differs.

## Test Cases

### TC-01: Health and Model List

```powershell
curl.exe http://localhost:8080/v1/models
```

Pass criteria:

- HTTP response is `200`.
- Response contains installed model IDs.

Evidence to save:

- Screenshot of Web UI Models page.
- Copy of `/v1/models` response.

### TC-02: Chat Completion

```powershell
curl.exe http://localhost:8080/v1/chat/completions `
  -H "Content-Type: application/json" `
  -d '{ "model": "llama-3.2-1b-instruct:q4_k_m", "messages": [{"role": "user", "content": "Reply with one short sentence: LocalAI is working."}], "temperature": 0.1, "max_tokens": 60 }'
```

Pass criteria:

- HTTP response is `200`.
- Response has `choices[0].message.content`.
- Content is relevant and not empty.

### TC-03: Streaming Chat

```powershell
curl.exe -N http://localhost:8080/v1/chat/completions `
  -H "Content-Type: application/json" `
  -d '{ "model": "llama-3.2-1b-instruct:q4_k_m", "stream": true, "messages": [{"role": "user", "content": "Count from 1 to 5."}], "max_tokens": 50 }'
```

Pass criteria:

- Response arrives as multiple `data:` chunks.
- Stream ends cleanly.

### TC-04: Embeddings and Similarity Comparison

Request embeddings:

```powershell
curl.exe http://localhost:8080/embeddings `
  -H "Content-Type: application/json" `
  -d '{ "model": "qwen3-embedding-0.6b", "input": ["A cat is sleeping on the sofa.", "A kitten is resting on a couch.", "The stock market opened lower today."] }'
```

Pass criteria:

- Response contains three embedding vectors.
- Vector dimensions are consistent.
- Similarity for sentence 1 vs sentence 2 should be higher than sentence 1 vs sentence 3.

Optional comparison script after saving response as `embeddings.json`:

```powershell
@'
import json, math
data=json.load(open("embeddings.json", encoding="utf-8"))
vecs=[x["embedding"] for x in data["data"]]
def cos(a,b):
    return sum(x*y for x,y in zip(a,b))/(math.sqrt(sum(x*x for x in a))*math.sqrt(sum(y*y for y in b)))
print("cat/kitten similarity:", cos(vecs[0], vecs[1]))
print("cat/stock similarity:", cos(vecs[0], vecs[2]))
'@ | python -
```

### TC-05: Image Understanding

Install a vision model first, then replace `VISION_MODEL_NAME`.

```powershell
curl.exe http://localhost:8080/v1/chat/completions `
  -H "Content-Type: application/json" `
  -d '{ "model": "VISION_MODEL_NAME", "messages": [{"role": "user", "content": [{"type": "text", "text": "What is visible in this image? Answer briefly."}, {"type": "image_url", "image_url": {"url": "https://upload.wikimedia.org/wikipedia/commons/thumb/d/dd/Gfp-wisconsin-madison-the-nature-boardwalk.jpg/640px-Gfp-wisconsin-madison-the-nature-boardwalk.jpg"}}]}], "max_tokens": 120 }'
```

Pass criteria:

- Model mentions visible image content such as grass, path, sky, or landscape.

### TC-06: Image Generation

Install an image generation model first. This is usually heavier than chat and may require GPU or patience on CPU.

```powershell
curl.exe http://localhost:8080/v1/images/generations `
  -H "Content-Type: application/json" `
  -d '{ "model": "IMAGE_MODEL_NAME", "prompt": "A small red robot on a desk, simple style", "size": "256x256", "step": 8 }'
```

Pass criteria:

- Response returns generated image data or an image URL/path.
- Generated file can be opened from the generated content directory or browser.

Evidence:

- Save the generated image.
- Record generation time and hardware used.

### TC-07: Speech to Text

Download a small sample audio:

```powershell
curl.exe -L "https://upload.wikimedia.org/wikipedia/commons/1/1f/George_W_Bush_Columbia_FINAL.ogg" -o sample.ogg
```

Transcribe:

```powershell
curl.exe http://localhost:8080/v1/audio/transcriptions `
  -H "Content-Type: multipart/form-data" `
  -F "file=@sample.ogg" `
  -F "model=whisper-1" `
  -F "response_format=json"
```

Pass criteria:

- Response contains `text`.
- Text is recognizably related to the audio.

### TC-08: Text to Speech

```powershell
curl.exe http://localhost:8080/tts `
  -H "Content-Type: application/json" `
  -d '{ "model": "pocket-tts", "input": "Hello, this is a LocalAI text to speech test." }' `
  --output tts-output.wav
```

Pass criteria:

- `tts-output.wav` is created.
- Audio plays and speech is understandable.

### TC-09: Voice Activity Detection

The VAD endpoint expects 16 kHz PCM float samples. For a quick smoke test, send a synthetic signal. A real speech clip is better for final evidence.

```powershell
@'
import json, math
sr=16000
audio=[0.0]*sr + [0.2*math.sin(2*math.pi*220*i/sr) for i in range(sr)] + [0.0]*sr
open("vad-request.json","w",encoding="utf-8").write(json.dumps({"model":"silero-vad","audio":audio}))
'@ | python -

curl.exe http://localhost:8080/v1/vad `
  -H "Content-Type: application/json" `
  -d "@vad-request.json"
```

Pass criteria:

- Response is valid JSON.
- For real speech input, response contains at least one speech segment.

### TC-10: Object Detection

```powershell
curl.exe -X POST http://localhost:8080/v1/detection `
  -H "Content-Type: application/json" `
  -d '{ "model": "rfdetr-cpp-nano", "image": "https://media.roboflow.com/dog.jpeg", "threshold": 0.3 }'
```

Pass criteria:

- Response contains `detections`.
- At least one detected object has `class_name`, `confidence`, and bounding box coordinates.

### TC-11: Face Detection and Face Embedding

Use `face-detect-yunet-sface` for commercial-safe testing.

```powershell
curl.exe -X POST http://localhost:8080/v1/face/detect `
  -H "Content-Type: application/json" `
  -d '{ "model": "face-detect-yunet-sface", "img": "https://upload.wikimedia.org/wikipedia/commons/8/8d/President_Barack_Obama.jpg" }'
```

```powershell
curl.exe -X POST http://localhost:8080/v1/face/embed `
  -H "Content-Type: application/json" `
  -d '{ "model": "face-detect-yunet-sface", "img": "https://upload.wikimedia.org/wikipedia/commons/8/8d/President_Barack_Obama.jpg" }'
```

Pass criteria:

- Detect returns at least one face region.
- Embed returns an embedding vector and dimension.

### TC-12: Speaker Diarization

This is an optional/heavy test. Install and configure a diarization-capable model first.

```powershell
curl.exe http://localhost:8080/v1/audio/diarization `
  -H "Content-Type: multipart/form-data" `
  -F "file=@meeting.wav" `
  -F "model=pyannote-diarization" `
  -F "num_speakers=2" `
  -F "response_format=verbose_json"
```

Pass criteria:

- Response contains `segments`.
- Segments include speaker labels such as `SPEAKER_00`.

## Results Table

| ID | Feature | Model | Status | Response time | Evidence file/screenshot | Notes |
|---|---|---|---|---|---|---|
| TC-01 | Health/model list |  | Not run |  |  |  |
| TC-02 | Chat |  | Not run |  |  |  |
| TC-03 | Streaming chat |  | Not run |  |  |  |
| TC-04 | Embeddings/comparison |  | Not run |  |  |  |
| TC-05 | Image understanding |  | Not run |  |  |  |
| TC-06 | Image generation |  | Not run |  |  |  |
| TC-07 | Speech to text |  | Not run |  |  |  |
| TC-08 | Text to speech |  | Not run |  |  |  |
| TC-09 | Voice activity detection |  | Not run |  |  |  |
| TC-10 | Object detection |  | Not run |  |  |  |
| TC-11 | Face detection/embedding |  | Not run |  |  |  |
| TC-12 | Speaker diarization |  | Not run |  |  |  |

Status values: `Pass`, `Fail`, `Blocked`, `Not run`.

## Defect Log

| ID | Feature | Issue | Steps to reproduce | Expected | Actual | Severity | Attachments |
|---|---|---|---|---|---|---|---|
| BUG-01 |  |  |  |  |  |  |  |

## Final Summary Template

```text
LocalAI local functional testing was performed on <date> using <hardware> and <LocalAI version/image>.

Passed:
- ...

Failed:
- ...

Blocked:
- ...

Key observations:
- ...

Recommendation:
- LocalAI is / is not ready for <demo/dev/local use case>.
- Follow-up required for <models/hardware/issues>.
```
