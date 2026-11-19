End-to-end live check of the routed OCR role (docs/data/ocr-easyocr-e2e-live-check-20261119.md). Synthetic development images only; a THROWAWAY collection.

1. python make_case.py                      builds C:/tmp/nexusai-g4/ocr-e2e-case (8 images + truth.json)
2. python -X utf8 tools/showcase/upload_case.py C:/tmp/nexusai-g4/ocr-e2e-case --collection <name> [--apply]
3. wait for the jobs (forensic.records_ingest_jobs, collection_id = <name>)
4. python -X utf8 check_readings.py          stored readings against the truth (docker exec psql)
5. python -X utf8 ask_images.py / ask_text_wording.py   nine questions through the live Ask engine

The scripts hold absolute paths and the collection name used on 2026-11-19; edit the constants at the top before reuse.
