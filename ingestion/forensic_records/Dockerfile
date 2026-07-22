FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

WORKDIR /app

COPY ingestion/forensic_records/requirements.txt ./requirements.txt
RUN pip install --no-cache-dir -r requirements.txt

COPY ingestion/forensic_records/worker.py ./worker.py

USER nobody

EXPOSE 9109
ENTRYPOINT ["python", "/app/worker.py"]
