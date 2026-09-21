"""Bounded, read-only XLSX parsing for forensic record ingestion.

The parser reads OOXML parts directly and never executes formulas, macros,
external relationships, hyperlinks, or embedded objects. Every returned row
keeps sheet/row identity plus per-cell raw and formula metadata so normalized
values never replace the source representation.
"""

from __future__ import annotations

import io
import posixpath
import re
import stat
import zipfile
from dataclasses import dataclass
from datetime import datetime, timedelta
from decimal import Decimal, InvalidOperation, ROUND_FLOOR, ROUND_HALF_EVEN
from pathlib import Path, PurePosixPath
from typing import Any, Iterable, Iterator
from xml.etree import ElementTree as ET

SPREADSHEET_NS = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
REL_NS = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
PACKAGE_REL_NS = "http://schemas.openxmlformats.org/package/2006/relationships"

CELL_REFERENCE_RE = re.compile(r"^([A-Za-z]+)([1-9][0-9]*)$")
DTD_RE = re.compile(br"<!\s*(?:DOCTYPE|ENTITY)\b", re.IGNORECASE)
BUILTIN_DATE_FORMAT_IDS = set(range(14, 23)) | set(range(27, 37)) | set(range(45, 48)) | {50, 51, 52, 53, 54, 55, 56, 57, 58}
FORMULA_METADATA_SUFFIXES = ("__xlsx_raw_value", "__xlsx_formula", "__xlsx_cell_type")


class XLSXFormatError(ValueError):
    """Raised when an OOXML workbook violates the bounded parser contract."""


@dataclass(frozen=True)
class XLSXLimits:
    max_entries: int = 10_000
    max_total_uncompressed_bytes: int = 256 * 1024 * 1024
    max_part_bytes: int = 64 * 1024 * 1024
    max_shared_strings: int = 1_000_000
    max_sheets: int = 256
    max_columns: int = 16_384
    max_rows_per_sheet: int = 1_048_576
    max_compression_ratio: int = 1_000


DEFAULT_XLSX_LIMITS = XLSXLimits()


@dataclass
class WorkbookContext:
    date_system: str
    sheets: list[dict[str, Any]]
    shared_strings: list[str]
    styles: list[dict[str, Any]]
    external_relationships: list[dict[str, str]]
    defined_name_count: int
    workbook_protected: bool
    calculation_mode: str
    warnings: list[str]


def profile_xlsx(
    path: Path,
    sample_rows_per_sheet: int = 100,
    max_sample_rows: int = 500,
    limits: XLSXLimits = DEFAULT_XLSX_LIMITS,
) -> dict[str, Any]:
    """Return bounded workbook/schema metadata and representative source rows."""

    samples: list[dict[str, Any]] = []
    headers: list[str] = []
    seen_headers: set[str] = set()
    try:
        with zipfile.ZipFile(path, "r") as archive:
            context = _load_context(archive, limits)
            for sheet_index, sheet in enumerate(context.sheets, start=1):
                remaining_samples = max(0, max_sample_rows - len(samples))
                for record in _iter_sheet_records(
                    archive,
                    context,
                    sheet,
                    sheet_index,
                    limits,
                    row_limit=min(max(0, sample_rows_per_sheet), remaining_samples),
                ):
                    samples.append(record)
                    for header in _data_headers(record):
                        if header not in seen_headers:
                            seen_headers.add(header)
                            headers.append(header)
                for header in sheet.get("headers", []):
                    if header not in seen_headers:
                        seen_headers.add(header)
                        headers.append(header)

            formula_cells = sum(int(sheet.get("formula_cell_count", 0)) for sheet in context.sheets)
            error_cells = sum(int(sheet.get("error_cell_count", 0)) for sheet in context.sheets)
            hidden_rows = sum(int(sheet.get("hidden_row_count", 0)) for sheet in context.sheets)
            merged_ranges = sum(len(sheet.get("merged_ranges", [])) for sheet in context.sheets)
            warnings = list(context.warnings)
            if formula_cells:
                warnings.append("formulas were preserved as text and were not calculated")
            if context.external_relationships:
                warnings.append("external workbook relationships were inventoried and not followed")
            if hidden_rows or any(sheet.get("state") != "visible" for sheet in context.sheets):
                warnings.append("hidden rows or sheets are retained and explicitly marked")
            if merged_ranges:
                warnings.append("merged ranges are inventoried; values are not propagated into blank merged cells")

            return {
                "format": "xlsx",
                "headers": headers,
                "sample_rows": len(samples),
                "schema": {header: "source_text" for header in headers},
                "sample_records": samples,
                "workbook": {
                    "date_system": context.date_system,
                    "sheet_count": len(context.sheets),
                    "sheets": context.sheets,
                    "defined_name_count": context.defined_name_count,
                    "workbook_protected": context.workbook_protected,
                    "calculation_mode": context.calculation_mode,
                    "external_relationships": context.external_relationships,
                    "formula_cell_count": formula_cells,
                    "error_cell_count": error_cells,
                    "hidden_row_count": hidden_rows,
                    "merged_range_count": merged_ranges,
                    "formulas_executed": False,
                    "macros_executed": False,
                    "external_links_followed": False,
                    "inventory_complete": True,
                },
                "warnings": warnings,
            }
    except (zipfile.BadZipFile, zipfile.LargeZipFile) as exc:
        raise XLSXFormatError(f"invalid XLSX ZIP package: {exc}") from exc


def iter_xlsx_records(path: Path, limits: XLSXLimits = DEFAULT_XLSX_LIMITS) -> Iterator[dict[str, Any]]:
    """Yield all non-header workbook rows while preserving sheet provenance."""

    try:
        with zipfile.ZipFile(path, "r") as archive:
            context = _load_context(archive, limits)
            for sheet_index, sheet in enumerate(context.sheets, start=1):
                yield from _iter_sheet_records(archive, context, sheet, sheet_index, limits)
    except (zipfile.BadZipFile, zipfile.LargeZipFile) as exc:
        raise XLSXFormatError(f"invalid XLSX ZIP package: {exc}") from exc


def _load_context(archive: zipfile.ZipFile, limits: XLSXLimits) -> WorkbookContext:
    names = _validate_archive(archive, limits)
    if "xl/vbaProject.bin" in names or any(name.lower().endswith("vbaproject.bin") for name in names):
        raise XLSXFormatError("macro payload is not permitted in the read-only XLSX adapter")
    workbook_root = _parse_xml_part(archive, "xl/workbook.xml", limits)
    relationships = _load_relationships(archive, "xl/_rels/workbook.xml.rels", limits)
    shared_strings = _load_shared_strings(archive, names, limits)
    styles = _load_styles(archive, names, limits)

    workbook_properties = workbook_root.find(_q(SPREADSHEET_NS, "workbookPr"))
    date1904 = workbook_properties is not None and _truthy(workbook_properties.attrib.get("date1904"))
    date_system = "1904" if date1904 else "1900"
    sheets_node = workbook_root.find(_q(SPREADSHEET_NS, "sheets"))
    if sheets_node is None:
        raise XLSXFormatError("workbook has no sheets collection")
    sheet_elements = list(sheets_node)
    if not sheet_elements or len(sheet_elements) > limits.max_sheets:
        raise XLSXFormatError(f"workbook sheet count must be between 1 and {limits.max_sheets}")

    external_relationships: list[dict[str, str]] = []
    sheets: list[dict[str, Any]] = []
    for relationship in relationships.values():
        if relationship["target_mode"] == "External":
            external_relationships.append({
                "id": relationship["id"],
                "type": relationship["type"],
                "target": relationship["target"],
            })
    for element in sheet_elements:
        relation_id = element.attrib.get(_q(REL_NS, "id"), "")
        relationship = relationships.get(relation_id)
        if relationship is None or relationship["target_mode"] == "External":
            raise XLSXFormatError(f"worksheet relationship {relation_id!r} is missing or external")
        part_name = _relationship_part("xl/workbook.xml", relationship["target"])
        if part_name not in names or not part_name.startswith("xl/worksheets/"):
            raise XLSXFormatError(f"worksheet part {part_name!r} is missing or outside xl/worksheets")
        sheets.append({
            "name": element.attrib.get("name", ""),
            "sheet_id": element.attrib.get("sheetId", ""),
            "relationship_id": relation_id,
            "state": element.attrib.get("state", "visible"),
            "part": part_name,
            "declared_dimension": "",
            "header_row": None,
            "data_rows_profiled": 0,
            "data_row_count": 0,
            "rows_truncated": False,
            "hidden_row_count": 0,
            "hidden_column_ranges": [],
            "merged_ranges": [],
            "formula_cell_count": 0,
            "error_cell_count": 0,
        })

    defined_names = workbook_root.find(_q(SPREADSHEET_NS, "definedNames"))
    protection = workbook_root.find(_q(SPREADSHEET_NS, "workbookProtection"))
    calculation = workbook_root.find(_q(SPREADSHEET_NS, "calcPr"))
    return WorkbookContext(
        date_system=date_system,
        sheets=sheets,
        shared_strings=shared_strings,
        styles=styles,
        external_relationships=external_relationships,
        defined_name_count=len(list(defined_names)) if defined_names is not None else 0,
        workbook_protected=protection is not None,
        calculation_mode=calculation.attrib.get("calcMode", "") if calculation is not None else "",
        warnings=[],
    )


def _validate_archive(archive: zipfile.ZipFile, limits: XLSXLimits) -> set[str]:
    entries = archive.infolist()
    if len(entries) > limits.max_entries:
        raise XLSXFormatError(f"XLSX contains more than {limits.max_entries} ZIP entries")
    total_size = 0
    names: set[str] = set()
    for info in entries:
        name = info.filename
        path = PurePosixPath(name)
        if not name or "\\" in name or path.is_absolute() or ".." in path.parts:
            raise XLSXFormatError(f"unsafe XLSX member path {name!r}")
        if name in names:
            raise XLSXFormatError(f"duplicate XLSX member name {name!r}")
        names.add(name)
        if info.flag_bits & 0x1:
            raise XLSXFormatError(f"encrypted XLSX member {name!r} is not supported")
        unix_mode = (info.external_attr >> 16) & 0xFFFF
        if unix_mode and stat.S_ISLNK(unix_mode):
            raise XLSXFormatError(f"symbolic-link XLSX member {name!r} is not supported")
        if info.file_size > limits.max_part_bytes:
            raise XLSXFormatError(f"XLSX member {name!r} exceeds the {limits.max_part_bytes}-byte part limit")
        if info.file_size and info.compress_size == 0:
            raise XLSXFormatError(f"XLSX member {name!r} has an invalid zero compressed size")
        if info.compress_size and info.file_size / info.compress_size > limits.max_compression_ratio:
            raise XLSXFormatError(f"XLSX member {name!r} exceeds the compression-ratio limit")
        total_size += info.file_size
        if total_size > limits.max_total_uncompressed_bytes:
            raise XLSXFormatError("XLSX exceeds the total uncompressed-byte limit")
    for required in ("[Content_Types].xml", "xl/workbook.xml", "xl/_rels/workbook.xml.rels"):
        if required not in names:
            raise XLSXFormatError(f"required XLSX part {required!r} is missing")
    return names


def _read_part(archive: zipfile.ZipFile, name: str, limits: XLSXLimits) -> bytes:
    try:
        info = archive.getinfo(name)
    except KeyError as exc:
        raise XLSXFormatError(f"required XLSX part {name!r} is missing") from exc
    if info.file_size > limits.max_part_bytes:
        raise XLSXFormatError(f"XLSX part {name!r} exceeds the bounded parser limit")
    data = archive.read(info)
    if len(data) != info.file_size:
        raise XLSXFormatError(f"XLSX part {name!r} size changed while reading")
    if DTD_RE.search(data[:65536]):
        raise XLSXFormatError(f"DTD or entity declarations are not permitted in XLSX XML part {name!r}")
    return data


def _parse_xml_part(archive: zipfile.ZipFile, name: str, limits: XLSXLimits) -> ET.Element:
    data = _read_part(archive, name, limits)
    try:
        return ET.fromstring(data)
    except ET.ParseError as exc:
        raise XLSXFormatError(f"invalid XML in XLSX part {name!r}: {exc}") from exc


def _load_relationships(archive: zipfile.ZipFile, name: str, limits: XLSXLimits) -> dict[str, dict[str, str]]:
    root = _parse_xml_part(archive, name, limits)
    relationships: dict[str, dict[str, str]] = {}
    for element in root.findall(_q(PACKAGE_REL_NS, "Relationship")):
        relation_id = element.attrib.get("Id", "")
        if not relation_id or relation_id in relationships:
            raise XLSXFormatError("workbook relationship IDs must be present and unique")
        relationships[relation_id] = {
            "id": relation_id,
            "type": element.attrib.get("Type", ""),
            "target": element.attrib.get("Target", ""),
            "target_mode": element.attrib.get("TargetMode", "Internal"),
        }
    return relationships


def _relationship_part(source_part: str, target: str) -> str:
    if not target or "\\" in target:
        raise XLSXFormatError(f"unsafe empty or backslash relationship target {target!r}")
    if target.startswith("/"):
        normalized = posixpath.normpath(target.lstrip("/"))
    else:
        normalized = posixpath.normpath(posixpath.join(posixpath.dirname(source_part), target))
    if normalized.startswith("../") or normalized == "..":
        raise XLSXFormatError(f"relationship target escapes the OOXML package: {target!r}")
    return normalized


def _load_shared_strings(archive: zipfile.ZipFile, names: set[str], limits: XLSXLimits) -> list[str]:
    if "xl/sharedStrings.xml" not in names:
        return []
    root = _parse_xml_part(archive, "xl/sharedStrings.xml", limits)
    strings: list[str] = []
    for item in root.findall(_q(SPREADSHEET_NS, "si")):
        strings.append("".join(node.text or "" for node in item.iter(_q(SPREADSHEET_NS, "t"))))
        if len(strings) > limits.max_shared_strings:
            raise XLSXFormatError("shared string count exceeds the bounded parser limit")
    return strings


def _load_styles(archive: zipfile.ZipFile, names: set[str], limits: XLSXLimits) -> list[dict[str, Any]]:
    if "xl/styles.xml" not in names:
        return []
    root = _parse_xml_part(archive, "xl/styles.xml", limits)
    custom_formats: dict[int, str] = {}
    formats = root.find(_q(SPREADSHEET_NS, "numFmts"))
    if formats is not None:
        for item in formats:
            try:
                format_id = int(item.attrib.get("numFmtId", ""))
            except ValueError:
                continue
            custom_formats[format_id] = item.attrib.get("formatCode", "")
    styles: list[dict[str, Any]] = []
    cell_formats = root.find(_q(SPREADSHEET_NS, "cellXfs"))
    if cell_formats is None:
        return styles
    for item in cell_formats:
        try:
            format_id = int(item.attrib.get("numFmtId", "0"))
        except ValueError:
            format_id = 0
        format_code = custom_formats.get(format_id, "")
        styles.append({
            "num_fmt_id": format_id,
            "format_code": format_code,
            "is_date": format_id in BUILTIN_DATE_FORMAT_IDS or _looks_like_date_format(format_code),
            "has_time": format_id in {18, 19, 20, 21, 22, 45, 46, 47} or _format_has_time(format_code),
        })
    return styles


def _looks_like_date_format(format_code: str) -> bool:
    cleaned = _clean_format_code(format_code)
    return bool(re.search(r"[ydhs]", cleaned)) or ("m" in cleaned and any(separator in cleaned for separator in ("/", "-", ":")))


def _format_has_time(format_code: str) -> bool:
    cleaned = _clean_format_code(format_code)
    return bool(re.search(r"[hs]", cleaned)) or ":" in cleaned


def _clean_format_code(format_code: str) -> str:
    cleaned = re.sub(r'"[^"]*"', "", format_code.lower())
    cleaned = re.sub(r"\\.", "", cleaned)
    cleaned = re.sub(r"\[(?!h+\]|m+\]|s+\])[^\]]*\]", "", cleaned)
    return cleaned


def _iter_sheet_records(
    archive: zipfile.ZipFile,
    context: WorkbookContext,
    sheet: dict[str, Any],
    sheet_index: int,
    limits: XLSXLimits,
    row_limit: int | None = None,
) -> Iterator[dict[str, Any]]:
    data = _read_part(archive, sheet["part"], limits)
    try:
        events = ET.iterparse(io.BytesIO(data), events=("end",))
        headers: list[str] | None = None
        header_row_number: int | None = None
        rows_yielded = 0
        data_rows_seen = 0
        hidden_columns: list[tuple[int, int]] = []
        formula_cells = 0
        error_cells = 0
        hidden_rows = 0
        merged_ranges: list[str] = []
        declared_dimension = ""
        for _, element in events:
            tag = _local_name(element.tag)
            if tag == "dimension":
                declared_dimension = element.attrib.get("ref", "")
                element.clear()
                continue
            if tag == "col":
                if _truthy(element.attrib.get("hidden")):
                    minimum = _bounded_int(element.attrib.get("min"), 1, limits.max_columns, "hidden column min")
                    maximum = _bounded_int(element.attrib.get("max"), minimum, limits.max_columns, "hidden column max")
                    hidden_columns.append((minimum - 1, maximum - 1))
                element.clear()
                continue
            if tag == "mergeCell":
                reference = element.attrib.get("ref", "")
                if reference:
                    merged_ranges.append(reference)
                element.clear()
                continue
            if tag != "row":
                continue

            row_number = _bounded_int(element.attrib.get("r"), 1, limits.max_rows_per_sheet, "worksheet row")
            hidden_row = _truthy(element.attrib.get("hidden"))
            if hidden_row:
                hidden_rows += 1
            cells = _row_cells(element, context, hidden_columns, limits)
            formula_cells += sum(1 for cell in cells.values() if cell.get("formula") is not None)
            error_cells += sum(1 for cell in cells.values() if cell.get("cell_type") == "error")
            nonempty = [cell for cell in cells.values() if cell.get("value") not in (None, "") or cell.get("formula")]
            if not nonempty:
                element.clear()
                continue
            if headers is None:
                max_column = max(cells) if cells else -1
                source_headers = [str(cells.get(index, {}).get("value") or "") for index in range(max_column + 1)]
                headers = _disambiguate_headers(source_headers)
                header_row_number = row_number
                sheet["source_headers"] = source_headers
                sheet["headers"] = headers
                sheet["header_row"] = row_number
                element.clear()
                continue

            data_rows_seen += 1
            if row_limit is None or rows_yielded < row_limit:
                record = _record_from_cells(cells, headers, sheet, sheet_index, row_number, hidden_row)
                rows_yielded += 1
                yield record
            else:
                sheet["rows_truncated"] = True
            element.clear()

        sheet["declared_dimension"] = declared_dimension
        sheet["header_row"] = header_row_number
        sheet["data_rows_profiled"] = rows_yielded
        sheet["data_row_count"] = data_rows_seen
        sheet["hidden_row_count"] = hidden_rows
        sheet["hidden_column_ranges"] = [f"{_column_letters(start)}:{_column_letters(end)}" for start, end in hidden_columns]
        sheet["merged_ranges"] = merged_ranges
        sheet["formula_cell_count"] = formula_cells
        sheet["error_cell_count"] = error_cells
    except ET.ParseError as exc:
        raise XLSXFormatError(f"invalid worksheet XML in {sheet['part']!r}: {exc}") from exc


def _row_cells(
    row: ET.Element,
    context: WorkbookContext,
    hidden_columns: list[tuple[int, int]],
    limits: XLSXLimits,
) -> dict[int, dict[str, Any]]:
    cells: dict[int, dict[str, Any]] = {}
    next_column = 0
    for element in row.findall(_q(SPREADSHEET_NS, "c")):
        reference = element.attrib.get("r", "")
        column = _column_index(reference) if reference else next_column
        if column < 0 or column >= limits.max_columns:
            raise XLSXFormatError(f"worksheet column index {column + 1} exceeds the XLSX limit")
        if column in cells:
            raise XLSXFormatError(f"worksheet row contains duplicate cell column {column + 1}")
        next_column = column + 1
        style_id = _optional_int(element.attrib.get("s"), 0)
        style = context.styles[style_id] if 0 <= style_id < len(context.styles) else {}
        cell_type = element.attrib.get("t", "n")
        raw_node = element.find(_q(SPREADSHEET_NS, "v"))
        raw_value = raw_node.text if raw_node is not None and raw_node.text is not None else ""
        formula_node = element.find(_q(SPREADSHEET_NS, "f"))
        formula = (formula_node.text or "") if formula_node is not None else None
        value, normalized_type, review_flag = _cell_value(element, cell_type, raw_value, style, context)
        cells[column] = {
            "reference": reference or f"{_column_letters(column)}?",
            "cell_type": normalized_type,
            "source_type": cell_type,
            "style_id": style_id,
            "number_format_id": style.get("num_fmt_id"),
            "number_format": style.get("format_code", ""),
            "raw_value": raw_value,
            "value": value,
            "formula": formula,
            "formula_attributes": dict(formula_node.attrib) if formula_node is not None else {},
            "cached_formula_value": raw_value if formula_node is not None else None,
            "formula_executed": False if formula_node is not None else None,
            "hidden_column": any(start <= column <= end for start, end in hidden_columns),
            "review_flag": review_flag,
        }
    return cells


def _cell_value(
    element: ET.Element,
    cell_type: str,
    raw_value: str,
    style: dict[str, Any],
    context: WorkbookContext,
) -> tuple[str, str, str]:
    if cell_type == "inlineStr":
        inline = element.find(_q(SPREADSHEET_NS, "is"))
        value = "" if inline is None else "".join(node.text or "" for node in inline.iter(_q(SPREADSHEET_NS, "t")))
        return value, "inline_string", ""
    if cell_type == "s":
        try:
            index = int(raw_value)
        except ValueError as exc:
            raise XLSXFormatError(f"shared-string cell has invalid index {raw_value!r}") from exc
        if index < 0 or index >= len(context.shared_strings):
            raise XLSXFormatError(f"shared-string index {index} is outside the table")
        return context.shared_strings[index], "shared_string", ""
    if cell_type == "b":
        if raw_value not in {"0", "1"}:
            return raw_value, "boolean", "invalid_boolean_value"
        return "true" if raw_value == "1" else "false", "boolean", ""
    if cell_type == "e":
        return raw_value, "error", "formula_or_cell_error"
    if cell_type in {"str", "d"}:
        return raw_value, "formula_string" if cell_type == "str" else "iso_date", ""
    if style.get("is_date") and raw_value:
        converted, review_flag = _excel_serial_to_iso(raw_value, context.date_system, bool(style.get("has_time")))
        return converted if converted is not None else raw_value, "excel_date", review_flag
    return raw_value, "number" if cell_type in {"", "n"} else cell_type, ""


def _excel_serial_to_iso(raw_value: str, date_system: str, has_time: bool) -> tuple[str | None, str]:
    try:
        serial = Decimal(raw_value)
    except InvalidOperation:
        return None, "invalid_excel_date_serial"
    if date_system == "1900" and serial >= 60 and serial < 61:
        return None, "excel_1900_fictitious_leap_day"
    whole_days = int(serial.to_integral_value(rounding=ROUND_FLOOR))
    fraction = serial - Decimal(whole_days)
    microseconds = int((fraction * Decimal(86_400_000_000)).to_integral_value(rounding=ROUND_HALF_EVEN))
    base = datetime(1904, 1, 1) if date_system == "1904" else datetime(1899, 12, 30)
    try:
        value = base + timedelta(days=whole_days, microseconds=microseconds)
    except OverflowError:
        return None, "excel_date_out_of_range"
    if has_time or value.time() != datetime.min.time():
        return value.isoformat(timespec="microseconds").rstrip("0").rstrip("."), ""
    return value.date().isoformat(), ""


def _record_from_cells(
    cells: dict[int, dict[str, Any]],
    headers: list[str],
    sheet: dict[str, Any],
    sheet_index: int,
    row_number: int,
    hidden_row: bool,
) -> dict[str, Any]:
    record: dict[str, Any] = {
        "_xlsx_sheet_name": sheet["name"],
        "_xlsx_sheet_index": sheet_index,
        "_xlsx_sheet_state": sheet["state"],
        "_xlsx_header_row": sheet.get("header_row"),
        "_xlsx_row_number": row_number,
        "_xlsx_hidden_row": hidden_row,
    }
    metadata: dict[str, Any] = {}
    for column, cell in sorted(cells.items()):
        header = headers[column] if column < len(headers) else f"_extra_column_{column + 1}"
        record[header] = cell["value"]
        metadata[header] = cell
        if cell["raw_value"] != cell["value"]:
            record[header + "__xlsx_raw_value"] = cell["raw_value"]
        if cell["formula"] is not None:
            record[header + "__xlsx_formula"] = cell["formula"]
        if cell["review_flag"]:
            record[header + "__xlsx_review_flag"] = cell["review_flag"]
    record["_xlsx_cells"] = metadata
    return record


def _data_headers(record: dict[str, Any]) -> Iterable[str]:
    for key in record:
        if key.startswith("_xlsx_") or key.endswith(FORMULA_METADATA_SUFFIXES) or key.endswith("__xlsx_review_flag"):
            continue
        yield key


def _disambiguate_headers(source_headers: list[str]) -> list[str]:
    headers: list[str] = []
    counts: dict[str, int] = {}
    for index, source_header in enumerate(source_headers, start=1):
        base = str(source_header).strip() or f"_column_{index}"
        counts[base] = counts.get(base, 0) + 1
        headers.append(base if counts[base] == 1 else f"{base}__duplicate_{counts[base]}")
    return headers


def _column_index(reference: str) -> int:
    match = CELL_REFERENCE_RE.match(reference)
    if match is None:
        raise XLSXFormatError(f"invalid XLSX cell reference {reference!r}")
    value = 0
    for character in match.group(1).upper():
        value = value * 26 + ord(character) - ord("A") + 1
    return value - 1


def _column_letters(index: int) -> str:
    value = index + 1
    result = ""
    while value:
        value, remainder = divmod(value - 1, 26)
        result = chr(ord("A") + remainder) + result
    return result


def _q(namespace: str, name: str) -> str:
    return f"{{{namespace}}}{name}"


def _local_name(tag: str) -> str:
    return tag.rsplit("}", 1)[-1]


def _truthy(value: str | None) -> bool:
    return str(value or "").strip().lower() in {"1", "true", "yes", "on"}


def _optional_int(value: str | None, fallback: int) -> int:
    try:
        return int(value) if value is not None else fallback
    except ValueError:
        return fallback


def _bounded_int(value: str | None, minimum: int, maximum: int, label: str) -> int:
    try:
        parsed = int(value or "")
    except ValueError as exc:
        raise XLSXFormatError(f"{label} is not an integer") from exc
    if parsed < minimum or parsed > maximum:
        raise XLSXFormatError(f"{label} must be between {minimum} and {maximum}")
    return parsed
