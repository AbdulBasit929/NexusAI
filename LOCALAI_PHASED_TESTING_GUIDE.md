# LocalAI Phased Local Testing Guide

This guide helps you test LocalAI functionality on your own laptop in phases. Each phase contains setup, commands, validation checks, evidence to capture, and how to record the result in the report.

Use this with [LOCALAI_TEST_REPORT_TEMPLATE.md](D:/Projects/LocalAI/LOCALAI_TEST_REPORT_TEMPLATE.md).

## Testing Rules

- Test one phase at a time.
- Keep screenshots and command outputs as evidence.
- Use `Pass`, `Fail`, `Blocked`, or `Not run`.
- If a feature needs a model that is too large for your laptop, mark it `Blocked by hardware/resources`.
- If a request fails because the model is not installed, mark it `Blocked by missing model`, install the model, then rerun.
- Use exact installed model names from `/v1/models`.

## Folder Setup

Run this once from `D:\Projects\LocalAI`:

```powershell
New-Item -ItemType Directory -Force .\localai-test-evidence
New-Item -ItemType Directory -Force .\localai-test-evidence\phase-00-environment
New-Item -ItemType Directory -Force .\localai-test-evidence\phase-01-server
New-Item -ItemType Directory -Force .\localai-test-evidence\phase-02-chat
New-Item -ItemType Directory -Force .\localai-test-evidence\phase-03-embeddings
New-Item -ItemType Directory -Force .\localai-test-evidence\phase-04-audio
New-Item -ItemType Directory -Force .\localai-test-evidence\phase-05-vision-images
New-Item -ItemType Directory -Force .\localai-test-evidence\phase-06-detection
New-Item -ItemType Directory -Force .\localai-test-evidence\phase-07-advanced
```

## Phase 00: Environment and Hardware Baseline

Purpose: prove what machine and runtime were used.

### Commands

```powershell
systeminfo | findstr /C:"OS Name" /C:"OS Version" /C:"Total Physical Memory" | Tee-Object .\localai-test-evidence\phase-00-environment\systeminfo.txt
wmic cpu get name | Tee-Object .\localai-test-evidence\phase-00-environment\cpu.txt
wmic path win32_VideoController get name | Tee-Object .\localai-test-evidence\phase-00-environment\gpu.txt
docker --version | Tee-Object .\localai-test-evidence\phase-00-environment\docker-version.txt
```

If you have NVIDIA GPU:

```powershell
nvidia-smi | Tee-Object .\localai-test-evidence\phase-00-environment\nvidia-smi.txt
```

### Validation

- OS, CPU, RAM, and GPU details are captured.
- Docker version is captured if Docker is used.

### Report Entry

Fill the `Environment` section in the report.

## Phase 01: Start LocalAI and Verify Server

Purpose: prove LocalAI starts and exposes the API and Web UI.

### Option A: CPU Docker

```powershell
New-Item -ItemType Directory -Force .\localai-models
New-Item -ItemType Directory -Force .\localai-generated
$modelsPath = Join-Path (Get-Location) "localai-models"
$generatedPath = Join-Path (Get-Location) "localai-generated"
docker run --name local-ai --rm -it `
  -p 8080:8080 `
  -v "${modelsPath}:/models" `
  -v "${generatedPath}:/tmp/generated/content" `
  localai/localai:latest-cpu
```

### Option B: NVIDIA Docker

```powershell
New-Item -ItemType Directory -Force .\localai-models
New-Item -ItemType Directory -Force .\localai-generated
$modelsPath = Join-Path (Get-Location) "localai-models"
$generatedPath = Join-Path (Get-Location) "localai-generated"
docker run --name local-ai --rm -it `
  --gpus all `
  -p 8080:8080 `
  -v "${modelsPath}:/models" `
  -v "${generatedPath}:/tmp/generated/content" `
  localai/localai:latest-gpu-nvidia-cuda-12
```

Keep this terminal running. In a second PowerShell terminal, run:

```powershell
curl.exe http://localhost:8080/v1/models `
  -o .\localai-test-evidence\phase-01-server\models.json
```

Open:

```text
http://localhost:8080
```

### Validation

- `/v1/models` returns JSON.
- Web UI opens.
- No server crash in Docker logs.

### Evidence

- `phase-01-server\models.json`
- Screenshot of Web UI home page.

## Phase 02: Model Installation Baseline

Purpose: install the smallest practical models needed for local validation.

Install through Web UI `Models` page, or with local CLI if available:

```powershell
local-ai models install llama-3.2-1b-instruct:q4_k_m
local-ai models install qwen3-embedding-0.6b
local-ai models install whisper-base
local-ai models install silero-vad
local-ai models install rfdetr-cpp-nano
local-ai models install face-detect-yunet-sface
local-ai models install pocket-tts
```

After installing, capture model list:

```powershell
curl.exe http://localhost:8080/v1/models `
  -o .\localai-test-evidence\phase-01-server\models-after-install.json
```

### Validation

- Installed models appear in `/v1/models`.
- Model names match the names you will use in later phases.

### Evidence

- `models-after-install.json`
- Screenshot of installed models in Web UI.

## Phase 03: Chat and Text Generation

Purpose: validate core LLM functionality.

Set your model name:

```powershell
$ChatModel = "llama-3.2-1b-instruct:q4_k_m"
```

### TC-03.1: Chat Completion

```powershell
curl.exe http://localhost:8080/v1/chat/completions `
  -H "Content-Type: application/json" `
  -d "{ `"model`": `"$ChatModel`", `"messages`": [{`"role`": `"user`", `"content`": `"Reply with one short sentence: LocalAI chat works.`"}], `"temperature`": 0.1, `"max_tokens`": 80 }" `
  -o .\localai-test-evidence\phase-02-chat\chat-completion.json
```

Validation:

- Response contains `choices`.
- Response text is non-empty and relevant.

### TC-03.2: Streaming Chat

```powershell
curl.exe -N http://localhost:8080/v1/chat/completions `
  -H "Content-Type: application/json" `
  -d "{ `"model`": `"$ChatModel`", `"stream`": true, `"messages`": [{`"role`": `"user`", `"content`": `"Count from 1 to 5.`"}], `"max_tokens`": 80 }" `
  -o .\localai-test-evidence\phase-02-chat\chat-stream.txt
```

Validation:

- Output contains `data:` chunks.
- Stream finishes cleanly.

### TC-03.3: Completion Endpoint

```powershell
curl.exe http://localhost:8080/v1/completions `
  -H "Content-Type: application/json" `
  -d "{ `"model`": `"$ChatModel`", `"prompt`": `"Complete this sentence: LocalAI is`", `"max_tokens`": 40, `"temperature`": 0.1 }" `
  -o .\localai-test-evidence\phase-02-chat\completion.json
```

Validation:

- Response contains completion text.

## Phase 04: Embeddings and Text Comparison

Purpose: validate vector generation and similarity comparison.

Set model:

```powershell
$EmbeddingModel = "qwen3-embedding-0.6b"
```

Request embeddings:

```powershell
curl.exe http://localhost:8080/embeddings `
  -H "Content-Type: application/json" `
  -d "{ `"model`": `"$EmbeddingModel`", `"input`": [`"A cat is sleeping on the sofa.`", `"A kitten is resting on a couch.`", `"The stock market opened lower today.`"] }" `
  -o .\localai-test-evidence\phase-03-embeddings\embeddings.json
```

Run comparison:

```powershell
@'
import json, math
path = r".\localai-test-evidence\phase-03-embeddings\embeddings.json"
data = json.load(open(path, encoding="utf-8"))
vecs = [x["embedding"] for x in data["data"]]
def cos(a, b):
    return sum(x*y for x, y in zip(a, b)) / (math.sqrt(sum(x*x for x in a)) * math.sqrt(sum(y*y for y in b)))
same_topic = cos(vecs[0], vecs[1])
different_topic = cos(vecs[0], vecs[2])
print(f"cat_vs_kitten={same_topic}")
print(f"cat_vs_stock={different_topic}")
print("PASS" if same_topic > different_topic else "FAIL")
'@ | python - | Tee-Object .\localai-test-evidence\phase-03-embeddings\similarity-result.txt
```

Validation:

- Three vectors are returned.
- `cat_vs_kitten` is greater than `cat_vs_stock`.

## Phase 05: Audio Features

Purpose: validate speech-to-text, text-to-speech, and voice activity detection.

### TC-05.1: Speech to Text

Download sample:

```powershell
curl.exe -L "https://upload.wikimedia.org/wikipedia/commons/1/1f/George_W_Bush_Columbia_FINAL.ogg" `
  -o .\localai-test-evidence\phase-04-audio\sample.ogg
```

Transcribe:

```powershell
curl.exe http://localhost:8080/v1/audio/transcriptions `
  -H "Content-Type: multipart/form-data" `
  -F "file=@.\localai-test-evidence\phase-04-audio\sample.ogg" `
  -F "model=whisper-1" `
  -F "response_format=json" `
  -o .\localai-test-evidence\phase-04-audio\transcription.json
```

If your installed model name is `whisper-base`, replace `whisper-1`.

Validation:

- Response contains `text`.
- Text matches the speech topic.

### TC-05.2: Text to Speech

```powershell
curl.exe http://localhost:8080/tts `
  -H "Content-Type: application/json" `
  -d '{ "model": "pocket-tts", "input": "Hello, this is a LocalAI text to speech validation test." }' `
  --output .\localai-test-evidence\phase-04-audio\tts-output.wav
```

Validation:

- `tts-output.wav` exists.
- You can play it and hear understandable speech.

### TC-05.3: Voice Activity Detection

Create request:

```powershell
@'
import json, math
sr = 16000
audio = [0.0] * sr + [0.2 * math.sin(2 * math.pi * 220 * i / sr) for i in range(sr)] + [0.0] * sr
open(r".\localai-test-evidence\phase-04-audio\vad-request.json", "w", encoding="utf-8").write(json.dumps({"model": "silero-vad", "audio": audio}))
'@ | python -
```

Run VAD:

```powershell
curl.exe http://localhost:8080/v1/vad `
  -H "Content-Type: application/json" `
  -d "@.\localai-test-evidence\phase-04-audio\vad-request.json" `
  -o .\localai-test-evidence\phase-04-audio\vad-response.json
```

Validation:

- Response is JSON.
- For real speech input, segments should appear.
- Synthetic tone may be useful as endpoint smoke test only; final VAD evidence should use real speech if possible.

## Phase 06: Vision and Image Generation

Purpose: validate image understanding and image generation. These tests are more resource-heavy.

### TC-06.1: Image Understanding

Install a vision model from the gallery first. Then set:

```powershell
$VisionModel = "REPLACE_WITH_INSTALLED_VISION_MODEL"
```

Run:

```powershell
curl.exe http://localhost:8080/v1/chat/completions `
  -H "Content-Type: application/json" `
  -d "{ `"model`": `"$VisionModel`", `"messages`": [{`"role`": `"user`", `"content`": [{`"type`": `"text`", `"text`": `"What is visible in this image? Answer briefly.`"}, {`"type`": `"image_url`", `"image_url`": {`"url`": `"https://upload.wikimedia.org/wikipedia/commons/thumb/d/dd/Gfp-wisconsin-madison-the-nature-boardwalk.jpg/640px-Gfp-wisconsin-madison-the-nature-boardwalk.jpg`"}}]}], `"max_tokens`": 120 }" `
  -o .\localai-test-evidence\phase-05-vision-images\vision-response.json
```

Validation:

- Response describes the image.
- Expected words may include path, grass, field, sky, or landscape.

### TC-06.2: Image Generation

Install an image generation model from the gallery. Then set:

```powershell
$ImageModel = "REPLACE_WITH_INSTALLED_IMAGE_MODEL"
```

Run:

```powershell
curl.exe http://localhost:8080/v1/images/generations `
  -H "Content-Type: application/json" `
  -d "{ `"model`": `"$ImageModel`", `"prompt`": `"A small red robot on a desk, simple style`", `"size`": `"256x256`", `"step`": 8 }" `
  -o .\localai-test-evidence\phase-05-vision-images\image-generation.json
```

Validation:

- Response contains image output or a generated content URL/path.
- Generated image opens successfully.

Record generation time:

- Start time.
- End time.
- CPU/GPU usage if visible.

## Phase 07: Detection and Recognition

Purpose: validate object detection and face features.

### TC-07.1: Object Detection

```powershell
curl.exe -X POST http://localhost:8080/v1/detection `
  -H "Content-Type: application/json" `
  -d '{ "model": "rfdetr-cpp-nano", "image": "https://media.roboflow.com/dog.jpeg", "threshold": 0.3 }' `
  -o .\localai-test-evidence\phase-06-detection\object-detection.json
```

Validation:

- Response contains `detections`.
- Detection has class name, confidence, and bounding box fields.

### TC-07.2: Face Detection

```powershell
curl.exe -X POST http://localhost:8080/v1/face/detect `
  -H "Content-Type: application/json" `
  -d '{ "model": "face-detect-yunet-sface", "img": "https://upload.wikimedia.org/wikipedia/commons/8/8d/President_Barack_Obama.jpg" }' `
  -o .\localai-test-evidence\phase-06-detection\face-detect.json
```

Validation:

- Response contains at least one face region.

### TC-07.3: Face Embedding

```powershell
curl.exe -X POST http://localhost:8080/v1/face/embed `
  -H "Content-Type: application/json" `
  -d '{ "model": "face-detect-yunet-sface", "img": "https://upload.wikimedia.org/wikipedia/commons/8/8d/President_Barack_Obama.jpg" }' `
  -o .\localai-test-evidence\phase-06-detection\face-embedding.json
```

Validation:

- Response contains embedding vector and dimension.

### TC-07.4: Face Verify

Use two images of the same person if you have local files:

```powershell
$Img1 = [Convert]::ToBase64String([IO.File]::ReadAllBytes("C:\path\to\person1.jpg"))
$Img2 = [Convert]::ToBase64String([IO.File]::ReadAllBytes("C:\path\to\person2.jpg"))
$Body = @{
  model = "face-detect-yunet-sface"
  img1 = "data:image/jpeg;base64,$Img1"
  img2 = "data:image/jpeg;base64,$Img2"
} | ConvertTo-Json
$Body | Out-File .\localai-test-evidence\phase-06-detection\face-verify-request.json -Encoding utf8
curl.exe -X POST http://localhost:8080/v1/face/verify `
  -H "Content-Type: application/json" `
  -d "@.\localai-test-evidence\phase-06-detection\face-verify-request.json" `
  -o .\localai-test-evidence\phase-06-detection\face-verify.json
```

Validation:

- Response contains `verified`, `distance`, and `threshold`.
- Same-person images should usually verify true; different-person images should usually verify false.

## Phase 08: Advanced/Optional Features

Run these only after core tests pass.

### TC-08.1: Speaker Diarization

Requires a diarization-capable model.

```powershell
curl.exe http://localhost:8080/v1/audio/diarization `
  -H "Content-Type: multipart/form-data" `
  -F "file=@.\localai-test-evidence\phase-07-advanced\meeting.wav" `
  -F "model=pyannote-diarization" `
  -F "num_speakers=2" `
  -F "response_format=verbose_json" `
  -o .\localai-test-evidence\phase-07-advanced\diarization.json
```

Validation:

- Response contains speaker-labelled segments.

### TC-08.2: Sound or Music Generation

Requires a compatible model such as ACE-Step.

```powershell
curl.exe http://localhost:8080/v1/sound-generation `
  -H "Content-Type: application/json" `
  -d '{ "model": "SOUND_MODEL_NAME", "text": "A short calm piano sound effect", "duration_seconds": 5 }' `
  --output .\localai-test-evidence\phase-07-advanced\sound-output.wav
```

Validation:

- Output audio file exists and plays.

### TC-08.3: Video Generation

Requires a compatible video generation model and is usually heavy.

```powershell
curl.exe http://localhost:8080/video `
  -H "Content-Type: application/json" `
  -d '{ "model": "VIDEO_MODEL_NAME", "prompt": "A red ball rolling on a table", "size": "256x256" }' `
  -o .\localai-test-evidence\phase-07-advanced\video-generation.json
```

Validation:

- Response includes generated video output or path.

## Phase 09: Final Review

Before presenting:

1. Confirm all evidence files exist.
2. Fill every row in the report result matrix.
3. Add screenshots for Web UI and generated image/audio/video outputs.
4. Add blocked reasons for heavy tests that could not run locally.
5. Summarize what works, what failed, what was blocked, and what needs better hardware.

## Suggested Presentation Order

1. Environment and LocalAI version.
2. Startup and Web UI.
3. Installed models.
4. Core API results: chat, streaming, embeddings.
5. Audio results: STT, TTS, VAD.
6. Image/vision results.
7. Detection/recognition results.
8. Optional advanced results.
9. Defects and blockers.
10. Recommendation.
