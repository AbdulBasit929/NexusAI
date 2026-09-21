FROM python:3.11-slim@sha256:90744cff8f32887f075c47d747a173ff333e9e98801667af93c357fa9f5e28ff

ENV DEBIAN_FRONTEND=noninteractive \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK=1

RUN apt-get update && apt-get install -y --no-install-recommends \
      fonts-dejavu-core \
      fonts-noto-core \
      libgomp1 \
      tesseract-ocr \
      tesseract-ocr-eng \
      tesseract-ocr-urd \
    && rm -rf /var/lib/apt/lists/*

RUN python -m pip install --no-cache-dir \
      paddlepaddle==3.3.1 \
      paddleocr==3.7.0 \
      Pillow==12.1.1

# The pinned PaddleX OCR dependency imports OpenCV at module initialization.
# Debian's libGL runtime is required even for CPU-only, headless inference.
# Keep this after the large pip layer so a runtime-library correction does not
# invalidate and repeat the PaddlePaddle wheel download.
RUN apt-get update && apt-get install -y --no-install-recommends libgl1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /work
COPY tools/r8_ocr_eval/generate_fixtures.py /opt/nexusai-r8/generate_fixtures.py
COPY tools/r8_ocr_eval/run_benchmark.py /opt/nexusai-r8/run_benchmark.py
COPY scripts/benchmark_r8_anpr_models.py /opt/nexusai-r8/score_benchmark.py
COPY configuration/forensic_r8_model_approval_gate.json /opt/nexusai-r8/model_approval_gate.json
COPY tests/fixtures/forensic_modalities/r8_image_fixture_taxonomy_v1.json /opt/nexusai-r8/image_fixture_taxonomy.json
COPY tools/r8_ocr_eval/THIRD_PARTY_NOTICES.md /opt/nexusai-r8/THIRD_PARTY_NOTICES.md

RUN python -c "import paddle, paddleocr, PIL; print('paddle='+paddle.__version__); print('paddleocr='+paddleocr.__version__); print('pillow='+PIL.__version__)" \
    && tesseract --version \
    && tesseract --list-langs | grep -E '^(eng|urd)$'

ENTRYPOINT ["python", "/opt/nexusai-r8/run_benchmark.py"]
