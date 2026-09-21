import subprocess
import tempfile
import unittest
import zipfile
from pathlib import Path
from unittest.mock import patch

from ingestion.forensic_records.document_pipeline import (
    DOCUMENT_PASSAGE_CONTRACT,
    extract_document,
)


class DocumentPipelineTests(unittest.TestCase):
    def identity(self, source_file: str) -> dict[str, str]:
        return {
            "source_file": source_file,
            "evidence_id": "11111111-1111-1111-1111-111111111111",
            "version_id": "22222222-2222-2222-2222-222222222222",
            "source_sha256": "a" * 64,
        }

    def test_extracts_utf8_text_with_stable_source_locator(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "case.txt"
            path.write_text("First line\nSecond line", encoding="utf-8")
            first = extract_document(path, **self.identity("case.txt"))
            second = extract_document(path, **self.identity("case.txt"))

        self.assertEqual(first.readiness, "READY")
        self.assertEqual(first.passages[0].text, "First line\nSecond line")
        self.assertEqual(first.passages[0].locator["section"], 1)
        self.assertEqual(first.passages[0].artifact_id, second.passages[0].artifact_id)

    def test_extracts_docx_paragraphs_without_rewriting_source(self):
        document_xml = (
            '<?xml version="1.0" encoding="UTF-8"?>'
            '<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">'
            '<w:body><w:p><w:r><w:t>Alpha</w:t></w:r></w:p>'
            '<w:p><w:r><w:t>Urdu: پاکستان</w:t></w:r></w:p></w:body></w:document>'
        )
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "brief.docx"
            with zipfile.ZipFile(path, "w") as archive:
                archive.writestr("word/document.xml", document_xml)
            before = path.read_bytes()
            result = extract_document(path, **self.identity("brief.docx"))
            after = path.read_bytes()

        self.assertEqual(before, after)
        self.assertEqual([item.text for item in result.passages], ["Alpha", "Urdu: پاکستان"])
        self.assertEqual(result.passages[1].locator["paragraph"], 2)

    def test_rejects_oversized_docx_document_xml_before_expansion(self):
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "oversized.docx"
            with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_DEFLATED) as archive:
                archive.writestr("word/document.xml", "<w:document>bounded</w:document>")
            with patch(
                "ingestion.forensic_records.document_pipeline.MAX_DOCX_DOCUMENT_XML_BYTES",
                8,
            ):
                with self.assertRaisesRegex(ValueError, "exceeds the 8-byte extraction bound"):
                    extract_document(path, **self.identity("oversized.docx"))

    def test_extracts_pdf_pages_through_bounded_native_text_runner(self):
        calls = []

        def runner(args, **kwargs):
            calls.append((args, kwargs))
            return subprocess.CompletedProcess(args, 0, stdout="Page one\fPage two\f", stderr="")

        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "policy.pdf"
            path.write_bytes(b"%PDF-1.4\n")
            result = extract_document(path, pdf_runner=runner, **self.identity("policy.pdf"))

        self.assertEqual([item.locator["page"] for item in result.passages], [1, 2])
        self.assertEqual(result.passages[0].text, "Page one")
        self.assertEqual(result.readiness, "READY_WITH_WARNINGS")
        self.assertEqual(calls[0][0][0], "pdftotext")
        self.assertEqual(calls[0][1]["timeout"], 60)

    def test_scanned_pdf_is_truthfully_unavailable_without_ocr(self):
        def runner(args, **kwargs):
            return subprocess.CompletedProcess(args, 0, stdout="\f", stderr="")

        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "scan.pdf"
            path.write_bytes(b"%PDF-1.4\n")
            result = extract_document(path, pdf_runner=runner, **self.identity("scan.pdf"))

        self.assertEqual(result.passages, ())
        self.assertTrue(any("no OCR was attempted" in item for item in result.limitations))

    def test_contract_is_versioned(self):
        self.assertEqual(DOCUMENT_PASSAGE_CONTRACT, "forensics.document-native-text-passage/v1")


if __name__ == "__main__":
    unittest.main()
