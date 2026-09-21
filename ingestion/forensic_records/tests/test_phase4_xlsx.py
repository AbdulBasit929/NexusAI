import json
import tempfile
import unittest
import zipfile
from dataclasses import replace
from pathlib import Path

from ingestion.forensic_records.worker import (
    ADAPTERS,
    IngestJob,
    classify_job_error,
    iter_records,
    normalize_generic_row,
    prepare_xlsx_sheet_mappings,
    profile_records,
    resolve_adapter,
    xlsx_row_adapter,
)
from ingestion.forensic_records.xlsx_reader import XLSXFormatError, profile_xlsx


WORKBOOK_NS = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
REL_NS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
PACKAGE_REL_NS = "http://schemas.openxmlformats.org/package/2006/relationships"
GOLDEN_PATH = Path(__file__).parent / "fixtures" / "pakistan_xlsx_goldens_v1.json"
MIXED_GOLDEN_PATH = Path(__file__).parent / "fixtures" / "pakistan_xlsx_sheet_mapping_goldens_v1.json"


def forensic_xlsx_parts() -> dict[str, bytes]:
    shared = [
        "txn_time",
        "debit_account",
        "debit_amount",
        "currency_code",
        "counterparty",
        "computed_total",
        "error_state",
        "PKR",
        "case_id",
    ]
    shared_xml = "".join(f"<si><t>{value}</t></si>" for value in shared)
    return {
        "[Content_Types].xml": b"""<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>""",
        "_rels/.rels": f"""<?xml version="1.0"?><Relationships xmlns="{PACKAGE_REL_NS}"><Relationship Id="rId1" Type="officeDocument" Target="xl/workbook.xml"/></Relationships>""".encode(),
        "xl/workbook.xml": f"""<?xml version="1.0"?><workbook xmlns="{WORKBOOK_NS}" xmlns:r="{REL_NS}"><workbookPr date1904="1"/><workbookProtection lockStructure="1"/><sheets><sheet name="Transactions" sheetId="1" r:id="rId1"/><sheet name="Review" sheetId="2" state="hidden" r:id="rId2"/></sheets><definedNames><definedName name="CaseRange">Transactions!$A$2:$G$3</definedName></definedNames><calcPr calcMode="manual"/></workbook>""".encode(),
        "xl/_rels/workbook.xml.rels": f"""<?xml version="1.0"?><Relationships xmlns="{PACKAGE_REL_NS}"><Relationship Id="rId1" Type="worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="worksheet" Target="worksheets/sheet2.xml"/><Relationship Id="rId9" Type="externalLink" Target="https://invalid.example/evidence.xlsx" TargetMode="External"/></Relationships>""".encode(),
        "xl/sharedStrings.xml": f"""<?xml version="1.0"?><sst xmlns="{WORKBOOK_NS}" count="9" uniqueCount="9">{shared_xml}</sst>""".encode(),
        "xl/styles.xml": f"""<?xml version="1.0"?><styleSheet xmlns="{WORKBOOK_NS}"><cellXfs count="2"><xf numFmtId="0"/><xf numFmtId="14"/></cellXfs></styleSheet>""".encode(),
        "xl/worksheets/sheet1.xml": f"""<?xml version="1.0"?><worksheet xmlns="{WORKBOOK_NS}"><dimension ref="A1:G4"/><cols><col min="7" max="7" hidden="1"/></cols><sheetData>
<row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="C1" t="s"><v>2</v></c><c r="D1" t="s"><v>3</v></c><c r="E1" t="s"><v>4</v></c><c r="F1" t="s"><v>5</v></c><c r="G1" t="s"><v>6</v></c></row>
<row r="2"><c r="A2" s="1"><v>1</v></c><c r="B2" t="inlineStr"><is><t>00012345</t></is></c><c r="C2"><v>001250.5000</v></c><c r="D2" t="s"><v>7</v></c><c r="E2" t="inlineStr"><is><t>کراچی مرچنٹ</t></is></c><c r="F2"><f>SUM(C2:C2)</f><v>1250.5</v></c><c r="G2" t="e"><v>#DIV/0!</v></c></row>
<row r="3" hidden="1"><c r="A3" s="1"><v>2.5</v></c><c r="B3" t="inlineStr"><is><t>00067890</t></is></c><c r="C3"><v>-50.00</v></c><c r="D3" t="s"><v>7</v></c><c r="E3" t="inlineStr"><is><t>review</t></is></c><c r="F3" t="str"><f>WEBSERVICE(&quot;https://invalid.example/&quot;)</f><v>NOT_EXECUTED</v></c></row>
<row r="4"><c r="E4" t="inlineStr"><is><t>merged note</t></is></c></row>
</sheetData><mergeCells count="1"><mergeCell ref="E4:F4"/></mergeCells></worksheet>""".encode("utf-8"),
        "xl/worksheets/sheet2.xml": f"""<?xml version="1.0"?><worksheet xmlns="{WORKBOOK_NS}"><dimension ref="A1:C2"/><sheetData><row r="1"><c r="A1" t="s"><v>8</v></c><c r="B1" t="inlineStr"><is><t></t></is></c><c r="C1" t="s"><v>8</v></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>PK-CASE-1</t></is></c><c r="B2" t="inlineStr"><is><t>hidden review value</t></is></c><c r="C2" t="inlineStr"><is><t>duplicate header retained</t></is></c></row></sheetData></worksheet>""".encode(),
    }


def mixed_family_xlsx_parts() -> dict[str, bytes]:
    sheets = ["Operator CDR", "ANPR Export", "Wallet Transactions", "Case Notes"]
    workbook_sheets = "".join(
        f'<sheet name="{name}" sheetId="{index}" r:id="rId{index}"/>'
        for index, name in enumerate(sheets, start=1)
    )
    relationships = "".join(
        f'<Relationship Id="rId{index}" Type="worksheet" Target="worksheets/sheet{index}.xml"/>'
        for index in range(1, len(sheets) + 1)
    )

    def worksheet(headers: list[str], values: list[str]) -> bytes:
        header_cells = "".join(
            f'<c r="{chr(65 + index)}1" t="inlineStr"><is><t>{value}</t></is></c>'
            for index, value in enumerate(headers)
        )
        value_cells = "".join(
            f'<c r="{chr(65 + index)}2" t="inlineStr"><is><t>{value}</t></is></c>'
            for index, value in enumerate(values)
        )
        end_column = chr(64 + len(headers))
        return (
            f'<?xml version="1.0"?><worksheet xmlns="{WORKBOOK_NS}">'
            f'<dimension ref="A1:{end_column}2"/><sheetData><row r="1">{header_cells}</row>'
            f'<row r="2">{value_cells}</row></sheetData></worksheet>'
        ).encode("utf-8")

    return {
        "[Content_Types].xml": b"""<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/></Types>""",
        "_rels/.rels": f'<?xml version="1.0"?><Relationships xmlns="{PACKAGE_REL_NS}"><Relationship Id="rId1" Type="officeDocument" Target="xl/workbook.xml"/></Relationships>'.encode(),
        "xl/workbook.xml": f'<?xml version="1.0"?><workbook xmlns="{WORKBOOK_NS}" xmlns:r="{REL_NS}"><sheets>{workbook_sheets}</sheets></workbook>'.encode(),
        "xl/_rels/workbook.xml.rels": f'<?xml version="1.0"?><Relationships xmlns="{PACKAGE_REL_NS}">{relationships}</Relationships>'.encode(),
        "xl/worksheets/sheet1.xml": worksheet(
            ["A Number", "B Number", "Call Time", "Duration Seconds", "IMEI"],
            ["+92 346 167 8183", "923001234567", "2026-07-29 08:30:00", "61", "356938035643809"],
        ),
        "xl/worksheets/sheet2.xml": worksheet(
            ["Registration No", "Reading Time", "Camera ID", "City"],
            ["ICT-AB-123", "2026-07-29T09:15:00+05:00", "ISB-CAM-07", "Islamabad"],
        ),
        "xl/worksheets/sheet3.xml": worksheet(
            ["Debit Account", "Transaction Amount", "Booking Date", "Currency Code", "Counterparty"],
            ["00012345", "001250.50", "2026-07-29T10:00:00+05:00", "PKR", "کراچی مرچنٹ"],
        ),
        "xl/worksheets/sheet4.xml": worksheet(
            ["Case ID", "Observation"],
            ["PK-CASE-4D", "Unstructured review note retained without typed coercion"],
        ),
    }


def write_xlsx(path: Path, parts: dict[str, bytes] | None = None) -> None:
    with zipfile.ZipFile(path, "w", compression=zipfile.ZIP_STORED) as archive:
        for name, payload in (parts or forensic_xlsx_parts()).items():
            archive.writestr(name, payload)


class Phase4XLSXAdapterTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.path = Path(self.temp_dir.name) / "pakistan_financial_multisheet.xlsx"
        write_xlsx(self.path)

    def tearDown(self) -> None:
        self.temp_dir.cleanup()

    def job(self, record_type: str = "generic") -> IngestJob:
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000401",
            tenant_id="default",
            user_id="analyst",
            collection_id="pakistan-financial-evidence",
            file_id="xlsx-1",
            source_file=self.path.name,
            spool_path=str(self.path),
            sha256="0" * 64,
            record_type=record_type,
            headers=[],
            metadata={"jurisdiction": "PK", "source_timezone": "Asia/Karachi"},
        )

    def test_profiles_complete_workbook_inventory_without_executing_content(self) -> None:
        profile = profile_xlsx(self.path, sample_rows_per_sheet=1, max_sample_rows=1)
        workbook = profile["workbook"]
        transactions, review = workbook["sheets"]

        self.assertEqual(workbook["date_system"], "1904")
        self.assertEqual(workbook["sheet_count"], 2)
        self.assertEqual(workbook["formula_cell_count"], 2)
        self.assertEqual(workbook["error_cell_count"], 1)
        self.assertEqual(workbook["merged_range_count"], 1)
        self.assertTrue(workbook["workbook_protected"])
        self.assertEqual(workbook["calculation_mode"], "manual")
        self.assertFalse(workbook["formulas_executed"])
        self.assertFalse(workbook["macros_executed"])
        self.assertFalse(workbook["external_links_followed"])
        self.assertEqual(len(workbook["external_relationships"]), 1)
        self.assertEqual(transactions["data_row_count"], 3)
        self.assertEqual(transactions["data_rows_profiled"], 1)
        self.assertTrue(transactions["rows_truncated"])
        self.assertEqual(transactions["merged_ranges"], ["E4:F4"])
        self.assertEqual(transactions["hidden_column_ranges"], ["G:G"])
        self.assertEqual(transactions["hidden_row_count"], 1)
        self.assertEqual(review["state"], "hidden")
        self.assertEqual(review["data_rows_profiled"], 0)
        self.assertEqual(review["data_row_count"], 1)
        self.assertIn("case_id__duplicate_2", profile["headers"])
        self.assertIn("formulas were preserved as text and were not calculated", profile["warnings"])

    def test_matches_versioned_pakistan_xlsx_golden_contract(self) -> None:
        golden = json.loads(GOLDEN_PATH.read_text(encoding="utf-8"))
        profile = profile_xlsx(self.path)
        first = next(iter(iter_records(self.path)))

        self.assertEqual(golden["schema_version"], "1.0.0")
        for key, expected in golden["adapter_contract"].items():
            actual = profile["format"] if key == "format" else profile["workbook"][key]
            self.assertEqual(actual, expected, key)
        expected = golden["first_transaction"]
        self.assertEqual(first["_xlsx_sheet_name"], expected["sheet_name"])
        self.assertEqual(first["_xlsx_row_number"], expected["row_number"])
        self.assertEqual(first["txn_time"], expected["txn_time"])
        self.assertEqual(first["txn_time__xlsx_raw_value"], expected["txn_time_raw"])
        self.assertEqual(first["debit_account"], expected["debit_account"])
        self.assertEqual(first["debit_amount"], expected["debit_amount"])
        self.assertEqual(first["currency_code"], expected["currency_code"])
        self.assertEqual(first["counterparty"], expected["counterparty"])
        self.assertEqual(first["computed_total__xlsx_formula"], expected["formula"])
        self.assertEqual(first["_xlsx_cells"]["computed_total"]["cached_formula_value"], expected["cached_formula_value"])
        self.assertEqual(first["error_state"], expected["error_state"])

    def test_streams_all_sheets_with_exact_values_and_source_locators(self) -> None:
        rows = list(iter_records(self.path))
        first = rows[0]
        hidden = rows[1]
        review = rows[3]

        self.assertEqual(len(rows), 4)
        self.assertEqual(first["txn_time"], "1904-01-02")
        self.assertEqual(first["txn_time__xlsx_raw_value"], "1")
        self.assertEqual(first["debit_account"], "00012345")
        self.assertEqual(first["debit_amount"], "001250.5000")
        self.assertEqual(first["counterparty"], "کراچی مرچنٹ")
        self.assertEqual(first["computed_total__xlsx_formula"], "SUM(C2:C2)")
        self.assertEqual(first["_xlsx_cells"]["computed_total"]["cached_formula_value"], "1250.5")
        self.assertFalse(first["_xlsx_cells"]["computed_total"]["formula_executed"])
        self.assertEqual(first["error_state"], "#DIV/0!")
        self.assertEqual(first["error_state__xlsx_review_flag"], "formula_or_cell_error")
        self.assertTrue(first["_xlsx_cells"]["error_state"]["hidden_column"])
        self.assertEqual(first["_xlsx_sheet_name"], "Transactions")
        self.assertEqual(first["_xlsx_row_number"], 2)
        self.assertTrue(hidden["_xlsx_hidden_row"])
        self.assertEqual(hidden["computed_total"], "NOT_EXECUTED")
        self.assertIn("WEBSERVICE", hidden["computed_total__xlsx_formula"])
        self.assertEqual(review["_xlsx_sheet_state"], "hidden")
        self.assertEqual(review["_column_2"], "hidden review value")
        self.assertEqual(review["case_id__duplicate_2"], "duplicate header retained")

    def test_worker_profile_and_generic_normalization_keep_workbook_lineage(self) -> None:
        profile = profile_records(self.path)
        rows = list(iter_records(self.path))
        adapter = resolve_adapter(self.job(), profile["headers"], force_generic=True)
        normalized = normalize_generic_row(self.job(), rows[0], self.job().job_id, 2, adapter, profile)
        raw = json.loads(normalized["raw_record"])
        details = json.loads(normalized["normalized_record"])

        self.assertEqual(profile["format"], "xlsx")
        self.assertEqual(adapter.name, "generic")
        self.assertEqual(details["xlsx_locator"]["sheet_name"], "Transactions")
        self.assertEqual(details["xlsx_locator"]["row_number"], 2)
        self.assertEqual(raw["debit_amount"], "001250.5000")
        self.assertEqual(raw["_xlsx_cells"]["computed_total"]["formula"], "SUM(C2:C2)")
        explicit = resolve_adapter(self.job("transaction"), profile["headers"], force_generic=True)
        self.assertEqual(explicit.name, "transaction")

    def test_maps_mixed_workbook_sheets_to_typed_rows_without_dropping_notes(self) -> None:
        path = Path(self.temp_dir.name) / "pakistan_mixed_provider_export.xlsx"
        write_xlsx(path, mixed_family_xlsx_parts())
        golden = json.loads(MIXED_GOLDEN_PATH.read_text(encoding="utf-8"))
        job = replace(self.job(), source_file=path.name, spool_path=str(path))
        job.metadata["record_type_mode"] = "auto"

        profile = profile_records(path)
        mappings, summary_adapter = prepare_xlsx_sheet_mappings(job, profile)
        rows = list(iter_records(path))

        self.assertEqual(golden["schema_version"], "1.0.0")
        self.assertEqual(profile["workbook"]["sheet_mapping_policy"], golden["mapping_policy"])
        self.assertEqual(summary_adapter.name, "generic")
        self.assertEqual(profile["workbook"]["mapped_record_types"], ["anpr", "cdr", "generic", "transaction"])

        for expected in golden["sheet_mappings"]:
            mapping = mappings[expected["sheet_index"]]
            self.assertEqual(mapping["sheet_name"], expected["sheet_name"])
            self.assertEqual(mapping["detected_record_type"], expected["record_type"])
            self.assertEqual(mapping["decision"], expected["decision"])
            self.assertEqual(mapping["review_required"], expected["review_required"])
            self.assertEqual(len(mapping["header_fingerprint"]), 64)

        normalized_rows = []
        for raw in rows:
            row_adapter, mapping = xlsx_row_adapter(raw, mappings, summary_adapter)
            normalized = normalize_generic_row(
                job,
                raw,
                job.job_id,
                raw["_xlsx_row_number"],
                row_adapter,
                profile,
                mapping,
            )
            details = json.loads(normalized["normalized_record"])
            normalized_rows.append((normalized, details))
            self.assertEqual(normalized["row_number"], raw["_xlsx_row_number"])
            self.assertEqual(details["xlsx_locator"]["sheet_name"], raw["_xlsx_sheet_name"])
            self.assertEqual(details["xlsx_mapping"]["detected_record_type"], row_adapter.name)
            self.assertEqual(details["xlsx_mapping"]["header_fingerprint"], mapping["header_fingerprint"])

        for (normalized, details), expected in zip(normalized_rows, golden["normalized_rows"], strict=True):
            self.assertEqual(normalized["record_type"], expected["record_type"])
            self.assertEqual(normalized["row_number"], expected["row_number"])
            self.assertEqual(normalized["primary_entity"], expected["primary_entity"])
            if "secondary_entity" in expected:
                self.assertEqual(normalized["secondary_entity"], expected["secondary_entity"])
            if "plate_search_key" in expected:
                self.assertEqual(details["plate_search_key"], expected["plate_search_key"])
            if "amount_decimal" in expected:
                self.assertEqual(details["amount_decimal"], expected["amount_decimal"])
                self.assertEqual(details["currency"], expected["currency"])

    def test_explicit_sheet_type_is_validated_and_mismatches_are_preserved_for_review(self) -> None:
        path = Path(self.temp_dir.name) / "pakistan_mixed_explicit_transaction.xlsx"
        write_xlsx(path, mixed_family_xlsx_parts())
        job = replace(self.job("transaction"), source_file=path.name, spool_path=str(path))
        job.metadata["record_type_mode"] = "explicit"

        profile = profile_records(path)
        mappings, summary_adapter = prepare_xlsx_sheet_mappings(job, profile)

        self.assertEqual(summary_adapter.name, "generic")
        self.assertEqual(mappings[3]["detected_record_type"], "transaction")
        self.assertEqual(mappings[3]["decision"], "explicit_typed_schema_validated")
        for sheet_index in (1, 2, 4):
            self.assertEqual(mappings[sheet_index]["detected_record_type"], "generic")
            self.assertEqual(mappings[sheet_index]["decision"], "explicit_type_mismatch_preserved_generic")
            self.assertTrue(mappings[sheet_index]["review_required"])
        self.assertTrue(profile["workbook"]["mapping_review_required"])

    def test_rejects_macro_payload_dtd_and_unsafe_zip_path(self) -> None:
        cases: list[tuple[str, dict[str, bytes], str]] = []
        macro_parts = forensic_xlsx_parts()
        macro_parts["xl/vbaProject.bin"] = b"not executable test bytes"
        cases.append(("macro.xlsx", macro_parts, "macro payload"))

        dtd_parts = forensic_xlsx_parts()
        dtd_parts["xl/workbook.xml"] = b'<?xml version="1.0"?><!DOCTYPE workbook [<!ENTITY x "unsafe">]><workbook/>'
        cases.append(("dtd.xlsx", dtd_parts, "DTD or entity"))

        traversal_parts = forensic_xlsx_parts()
        traversal_parts["../escape.xml"] = b"unsafe"
        cases.append(("traversal.xlsx", traversal_parts, "unsafe XLSX member path"))

        for filename, parts, message in cases:
            with self.subTest(filename=filename):
                path = Path(self.temp_dir.name) / filename
                write_xlsx(path, parts)
                with self.assertRaisesRegex(XLSXFormatError, message):
                    profile_xlsx(path)

    def test_flags_excel_1900_fictitious_leap_day_without_fabricating_a_date(self) -> None:
        parts = forensic_xlsx_parts()
        parts["xl/workbook.xml"] = parts["xl/workbook.xml"].replace(b'date1904="1"', b'date1904="0"')
        parts["xl/worksheets/sheet1.xml"] = parts["xl/worksheets/sheet1.xml"].replace(
            b'<c r="A2" s="1"><v>1</v></c>',
            b'<c r="A2" s="1"><v>60</v></c>',
        )
        path = Path(self.temp_dir.name) / "excel_1900_leap_day.xlsx"
        write_xlsx(path, parts)

        first = next(iter(iter_records(path)))
        self.assertEqual(first["txn_time"], "60")
        self.assertEqual(first["txn_time__xlsx_review_flag"], "excel_1900_fictitious_leap_day")

    def test_rejects_shared_string_index_outside_the_table(self) -> None:
        parts = forensic_xlsx_parts()
        parts["xl/worksheets/sheet1.xml"] = parts["xl/worksheets/sheet1.xml"].replace(
            b'<c r="D2" t="s"><v>7</v></c>',
            b'<c r="D2" t="s"><v>999</v></c>',
        )
        path = Path(self.temp_dir.name) / "invalid_shared_string.xlsx"
        write_xlsx(path, parts)

        with self.assertRaisesRegex(XLSXFormatError, "outside the table"):
            list(iter_records(path))

    def test_corrupt_zip_is_a_permanent_xlsx_input_error(self) -> None:
        path = Path(self.temp_dir.name) / "corrupt.xlsx"
        path.write_bytes(b"PK\x03\x04truncated")

        with self.assertRaisesRegex(XLSXFormatError, "invalid XLSX ZIP package") as raised:
            profile_xlsx(path)
        self.assertEqual(classify_job_error(raised.exception), "permanent_input")


if __name__ == "__main__":
    unittest.main()
