FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /app

RUN apt-get update && \
    apt-get install -y --no-install-recommends ffmpeg poppler-utils tesseract-ocr && \
    rm -rf /var/lib/apt/lists/*

COPY ingestion/forensic_records/requirements.txt ./requirements.txt
# Reuse package downloads after a stalled transfer, including across failed builds.
# pip 24 does not reliably retry mid-body read failures, so bound whole attempts too.
RUN --mount=type=cache,target=/root/.cache/pip \
    attempt=1; while :; do \
      echo "Worker package installation attempt ${attempt}/3 (socket timeout 120s)"; \
      python -m pip install --timeout 120 --retries 5 --progress-bar off --disable-pip-version-check -r requirements.txt && break; \
      status=$?; if [ "$attempt" -ge 3 ]; then exit "$status"; fi; \
      attempt=$((attempt + 1)); echo "Package attempt failed; retrying in 5 seconds"; sleep 5; \
    done

COPY ingestion/forensic_records/worker.py ./worker.py
COPY ingestion/forensic_records/media_pipeline.py ./media_pipeline.py
COPY ingestion/forensic_records/multilingual_ocr.py ./multilingual_ocr.py
COPY ingestion/forensic_records/processor_readiness.py ./processor_readiness.py
COPY ingestion/forensic_records/video_anpr_product_v2.py ./video_anpr_product_v2.py
COPY ingestion/forensic_records/video_anpr_onnx_v3.py ./video_anpr_onnx_v3.py
COPY ingestion/forensic_records/video_anpr_parity_adapter.py ./video_anpr_parity_adapter.py
COPY ingestion/forensic_records/image_intelligence.py ./image_intelligence.py
COPY ingestion/forensic_records/roman_urdu.py ./roman_urdu.py
COPY ingestion/forensic_records/document_pipeline.py ./document_pipeline.py
COPY ingestion/forensic_records/xlsx_reader.py ./xlsx_reader.py
COPY ingestion/forensic_records/forensic_contracts.py ./forensic_contracts.py
COPY ingestion/forensic_records/structured_maturity.py ./structured_maturity.py
COPY ingestion/forensic_records/adapters ./adapters
COPY api/forensic_records/contracts/forensic-platform-v1.json ./forensic-platform-v1.json

USER nobody

EXPOSE 9109
ENTRYPOINT ["python", "/app/worker.py"]
