+++
title = "NexusAI media upload correction"
toc = true
description = "Prepared media intake size policy and safe upload lifecycle"
categories = ["Features"]
+++

## Active local correction

The correction was deployed and runtime-verified on the local reference
instance on 2026-08-31. `LOCALAI_UPLOAD_LIMIT` and its legacy alias are both
320 MiB; the embedded UI enforces 300 MiB per general/video file and 64 MiB per
image. Other installations must still use a reviewed configuration and should
not treat this local receipt as authority for their deployment.

## Policy

The NX-MMR correction permits general/video files up to **300 MiB
(314,572,800 bytes)** in Add data. Images keep their existing **64 MiB**
limit. File signatures, decoding guards, classification and other format-specific
limits remain authoritative; a size allowance does not certify a file or model.

`docker-compose.nxmmr-upload-300m.yaml` prepares `LOCALAI_UPLOAD_LIMIT=320`
for the LocalAI `api` service. This is a **320 MiB whole-request** limit, leaving
room for multipart metadata around a 300 MiB file. The UI's 300 MiB per-file
limit is not a new backend per-file limit: direct API requests remain bounded
by the whole-request cap and existing backend format checks.

The fragment is not a standalone deployment recipe. It must be merged into a
reviewed snapshot of the current service configuration, with rollback and
explicit deployment approval. Rebuilding the worker alone does not change the
LocalAI upload cap or its embedded UI. Until deployed together, the old live
limit and old UI continue to apply. Reverse proxies, if present, must also
permit the intended request size. No global default was raised.

## When to wait or close Add data

1. Select files. **Waiting** before submission means only selected locally.
   Click **Add selected data** to start. Closing now discards the selection;
   it does not upload or delete the local files.
2. During an active batch, Waiting means queued behind the two registration
   slots. Keep the browser page open during **Uploading/Registering**.
   The dialog's Close, Escape and backdrop dismissal are blocked then;
   do not refresh or close the browser tab.
3. Once registration is accepted and **Processing** appears, server-side
   processing continues after the dialog is closed. Check Data for the result.
4. A confirmed **HTTP 413** means a size refusal. A connection failure can
   instead leave registration uncertain: inspect Data before retrying to avoid
   treating duplicate/reused evidence as fresh acceptance.

## OCR stored-image input

The Paddle detector now receives the already decoded BGR frame, not the
extensionless content-addressed storage filename. This preserves original
bytes and removes suffix-dependent input failures. Decoder errors remain
errors; a genuine blank image can complete with zero observations. Processor
revision: `nxmmr-anpr-ocr-vertical-v1-inputfix1`. Models, recognition thresholds,
ANPR routing and benchmark oracles are unchanged.

Source tests and synthetic smoke checks do not establish product certification.
Live API, Data, Ask, citations, Activity, UI and manual acceptance still need
separately authorized verification. Existing failed retained sources are not
automatically retried by this correction.
