"""Bounded native-text extraction for retained forensic documents.

This module never performs OCR and never changes source bytes.  It extracts
only text already encoded in TXT, DOCX, or PDF evidence and emits stable,
source-bound passage locators suitable for citations.
"""

from __future__ import annotations

import hashlib
import re
import subprocess
import uuid
import zipfile
from dataclasses import dataclass
from pathlib import Path
from typing import Callable
from xml.etree import ElementTree

DOCUMENT_PASSAGE_CONTRACT = "forensics.document-native-text-passage/v1"
DOCUMENT_RESULT_CONTRACT = "forensics.document-native-text-result/v1"
DOCUMENT_PROCESSOR_ID = "bounded-native-document-extractor"
DOCUMENT_PROCESSOR_REVISION = "v1"
MAX_DOCUMENT_BYTES = 64 * 1024 * 1024
MAX_DOCX_DOCUMENT_XML_BYTES = 16 * 1024 * 1024
MAX_EXTRACTED_CHARACTERS = 2_000_000
MAX_PASSAGE_CHARACTERS = 1_500


@dataclass(frozen=True)
class DocumentPassage:
    artifact_id: str
    text: str
    locator: dict[str, object]
    text_sha256: str


@dataclass(frozen=True)
class DocumentProcessResult:
    contract_version: str
    processor_id: str
    processor_revision: str
    format: str
    readiness: str
    passages: tuple[DocumentPassage, ...]
    limitations: tuple[str, ...]
    metadata: dict[str, object]


def extract_document(
    path: Path,
    *,
    source_file: str,
    evidence_id: str,
    version_id: str,
    source_sha256: str,
    pdf_runner: Callable[..., subprocess.CompletedProcess[str]] = subprocess.run,
) -> DocumentProcessResult:
    size = path.stat().st_size
    if size > MAX_DOCUMENT_BYTES:
        raise ValueError(f"document exceeds the {MAX_DOCUMENT_BYTES}-byte extraction bound")
    extension = Path(source_file).suffix.lower()
    limitations: list[str] = []
    if extension == ".txt":
        units, encoding = _extract_text(path)
        format_name = "txt"
        metadata: dict[str, object] = {"encoding": encoding}
    elif extension == ".docx":
        units = _extract_docx(path)
        format_name = "docx"
        metadata = {}
    elif extension == ".pdf":
        units = _extract_pdf(path, pdf_runner)
        format_name = "pdf"
        metadata = {"page_count_observed": len(units)}
        limitations.append("PDF extraction is native text only; scanned pages require a separately approved OCR stage.")
    else:
        raise ValueError("native document extraction supports only TXT, PDF, and DOCX")

    passages: list[DocumentPassage] = []
    extracted_characters = 0
    for unit_index, (locator_kind, locator_number, text) in enumerate(units, start=1):
        normalized = _normalize_text(text)
        if not normalized:
            continue
        for chunk_index, chunk in enumerate(_bounded_chunks(normalized), start=1):
            extracted_characters += len(chunk)
            if extracted_characters > MAX_EXTRACTED_CHARACTERS:
                limitations.append("Extracted text reached the 2,000,000-character safety bound and was truncated.")
                break
            locator = {
                "evidence_id": evidence_id,
                "version_id": version_id,
                "source_sha256": source_sha256,
                "source_file": source_file,
                locator_kind: locator_number,
                "passage": chunk_index,
            }
            stable_name = f"{evidence_id}:{version_id}:{unit_index}:{chunk_index}:{hashlib.sha256(chunk.encode()).hexdigest()}"
            passages.append(
                DocumentPassage(
                    artifact_id=str(uuid.uuid5(uuid.NAMESPACE_URL, stable_name)),
                    text=chunk,
                    locator=locator,
                    text_sha256=hashlib.sha256(chunk.encode("utf-8")).hexdigest(),
                )
            )
        if extracted_characters > MAX_EXTRACTED_CHARACTERS:
            break

    if not passages:
        limitations.append("No native text was extracted; source bytes remain retained and no OCR was attempted.")
    metadata.update(
        {
            "source_size_bytes": size,
            "passage_count": len(passages),
            "extracted_characters": min(extracted_characters, MAX_EXTRACTED_CHARACTERS),
            "native_text_only": True,
        }
    )
    return DocumentProcessResult(
        contract_version=DOCUMENT_RESULT_CONTRACT,
        processor_id=DOCUMENT_PROCESSOR_ID,
        processor_revision=DOCUMENT_PROCESSOR_REVISION,
        format=format_name,
        readiness="READY" if passages and not limitations else "READY_WITH_WARNINGS",
        passages=tuple(passages),
        limitations=tuple(limitations),
        metadata=metadata,
    )


def _extract_text(path: Path) -> tuple[list[tuple[str, int, str]], str]:
    payload = path.read_bytes()
    for encoding in ("utf-8-sig", "utf-16", "utf-16-le", "utf-16-be"):
        try:
            return [("section", 1, payload.decode(encoding))], encoding
        except UnicodeDecodeError:
            continue
    return [("section", 1, payload.decode("utf-8", errors="replace"))], "utf-8-replacement"


def _extract_docx(path: Path) -> list[tuple[str, int, str]]:
    with zipfile.ZipFile(path) as archive:
        try:
            document_xml = archive.getinfo("word/document.xml")
        except KeyError:
            raise ValueError("DOCX package does not contain word/document.xml")
        if document_xml.file_size > MAX_DOCX_DOCUMENT_XML_BYTES:
            raise ValueError(
                f"DOCX word/document.xml exceeds the {MAX_DOCX_DOCUMENT_XML_BYTES}-byte extraction bound"
            )
        root = ElementTree.fromstring(archive.read(document_xml))
    namespace = "{http://schemas.openxmlformats.org/wordprocessingml/2006/main}"
    units: list[tuple[str, int, str]] = []
    for index, paragraph in enumerate(root.iter(namespace + "p"), start=1):
        text = "".join(node.text or "" for node in paragraph.iter(namespace + "t"))
        if text.strip():
            units.append(("paragraph", index, text))
    return units


def _extract_pdf(
    path: Path,
    runner: Callable[..., subprocess.CompletedProcess[str]],
) -> list[tuple[str, int, str]]:
    try:
        completed = runner(
            ["pdftotext", "-layout", "-enc", "UTF-8", str(path), "-"],
            check=False,
            capture_output=True,
            text=True,
            encoding="utf-8",
            errors="replace",
            timeout=60,
        )
    except FileNotFoundError as exc:
        raise RuntimeError("pdftotext is unavailable in the document worker image") from exc
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError("PDF native-text extraction exceeded 60 seconds") from exc
    if completed.returncode != 0:
        detail = (completed.stderr or "unknown pdftotext error").strip()[:500]
        raise ValueError(f"PDF native-text extraction failed: {detail}")
    pages = completed.stdout.split("\f")
    if pages and not pages[-1].strip():
        pages.pop()
    return [("page", index, text) for index, text in enumerate(pages, start=1)]


def _normalize_text(value: str) -> str:
    value = value.replace("\x00", "")
    lines = [re.sub(r"[ \t]+", " ", line).strip() for line in value.splitlines()]
    return "\n".join(line for line in lines if line).strip()


def _bounded_chunks(text: str) -> list[str]:
    if len(text) <= MAX_PASSAGE_CHARACTERS:
        return [text]
    chunks: list[str] = []
    remaining = text
    while remaining:
        split_at = min(MAX_PASSAGE_CHARACTERS, len(remaining))
        if split_at < len(remaining):
            boundary = max(remaining.rfind("\n", 0, split_at), remaining.rfind(" ", 0, split_at))
            if boundary >= MAX_PASSAGE_CHARACTERS // 2:
                split_at = boundary
        chunks.append(remaining[:split_at].strip())
        remaining = remaining[split_at:].strip()
    return [chunk for chunk in chunks if chunk]
