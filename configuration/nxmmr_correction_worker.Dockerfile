ARG BASE_IMAGE
FROM ${BASE_IMAGE}
# Reuse the activated dependencies and change only the tested OCR adapter.
COPY ingestion/forensic_records/multilingual_ocr.py /app/multilingual_ocr.py
