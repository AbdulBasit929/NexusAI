"""A minimal DOCX writer with no library: paragraphs (left to right or right to left), one table, document properties. Deterministic: the same blocks give the same bytes.

The output opens in Word and is read by `ingestion/forensic_records/document_pipeline._extract_docx`, which numbers every `w:p` element of the document in order (the paragraphs in table cells
included, empty ones too): `paragraph_numbers` tells the generator which number each paragraph it writes will have, so a mention can be located by the same locator the pipeline emits.

A block is ("p", text, options) or ("table", rows, options); `text` and each cell are plain strings; options: rtl (bool), bold (bool), size (half points).
"""
import datetime
import io
import zipfile
from xml.sax.saxutils import escape

W_NS = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
FIXED_TIME = (2026, 2, 2, 0, 0, 0)
URDU_FONT = "Jameel Noori Nastaleeq"
LATIN_FONT = "Calibri"


def _run(text, rtl=False, bold=False, size=None):
    props = []
    if rtl:
        props.append('<w:rFonts w:ascii="%s" w:hAnsi="%s" w:cs="%s"/>' % (LATIN_FONT, LATIN_FONT, URDU_FONT))
    else:
        props.append('<w:rFonts w:ascii="%s" w:hAnsi="%s" w:cs="%s"/>' % (LATIN_FONT, LATIN_FONT, LATIN_FONT))
    if bold:
        props.append("<w:b/><w:bCs/>")
    if size:
        props.append('<w:sz w:val="%d"/><w:szCs w:val="%d"/>' % (size, size))
    if rtl:
        props.append("<w:rtl/>")
    return '<w:r><w:rPr>%s</w:rPr><w:t xml:space="preserve">%s</w:t></w:r>' % ("".join(props), escape(text))


def _paragraph(text, options):
    rtl = bool(options.get("rtl"))
    ppr = "<w:pPr>%s</w:pPr>" % ("<w:bidi/>" if rtl else "")
    return "<w:p>%s%s</w:p>" % (ppr, _run(text, rtl, options.get("bold", False), options.get("size")) if text else "")


def _table(rows, options):
    out = ['<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/><w:tblBorders>']
    for side in ("top", "left", "bottom", "right", "insideH", "insideV"):
        out.append('<w:%s w:val="single" w:sz="4" w:space="0" w:color="808080"/>' % side)
    out.append("</w:tblBorders></w:tblPr>")
    for row in rows:
        out.append("<w:tr>")
        for cell in row:
            out.append("<w:tc><w:tcPr><w:tcW w:w=\"0\" w:type=\"auto\"/></w:tcPr>%s</w:tc>" % _paragraph(cell, options))
        out.append("</w:tr>")
    out.append("</w:tbl>")
    return "".join(out)


def paragraph_numbers(blocks):
    """For each block, the numbers (1-based, in document order) that `w:p` elements of the block will have: a paragraph block has one, a table one per cell; a final empty paragraph follows the body."""
    numbers, count = [], 0
    for kind, content, _ in blocks:
        if kind == "p":
            count += 1
            numbers.append([count])
        else:
            cells = []
            for row in content:
                for _cell in row:
                    count += 1
                    cells.append(count)
            numbers.append(cells)
    return numbers


def build_docx(blocks, title="", author="SYNTHETIC test document", created=None):
    created = created or datetime.datetime(2026, 2, 2, 0, 0, 0)
    body = []
    for kind, content, options in blocks:
        body.append(_paragraph(content, options) if kind == "p" else _table(content, options))
    body.append("<w:sectPr><w:pgSz w:w=\"11906\" w:h=\"16838\"/><w:pgMar w:top=\"1134\" w:right=\"1134\" w:bottom=\"1134\" w:left=\"1134\" w:header=\"708\" w:footer=\"708\" w:gutter=\"0\"/></w:sectPr>")
    document = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document xmlns:w="%s"><w:body>%s</w:body></w:document>' % (W_NS, "".join(body)))
    content_types = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">'
                     '<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/>'
                     '<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>'
                     '<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/></Types>')
    rels = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">'
            '<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>'
            '<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/></Relationships>')
    stamp = created.strftime("%Y-%m-%dT%H:%M:%SZ")
    core = ('<?xml version="1.0" encoding="UTF-8" standalone="yes"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" '
            'xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">'
            '<dc:title>%s</dc:title><dc:creator>%s</dc:creator><dcterms:created xsi:type="dcterms:W3CDTF">%s</dcterms:created></cp:coreProperties>' % (escape(title), escape(author), stamp))
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", zipfile.ZIP_DEFLATED) as archive:
        for name, data in (("[Content_Types].xml", content_types), ("_rels/.rels", rels), ("word/document.xml", document), ("docProps/core.xml", core)):
            info = zipfile.ZipInfo(name, FIXED_TIME)
            info.compress_type = zipfile.ZIP_DEFLATED
            archive.writestr(info, data.encode("utf-8"))
    return buffer.getvalue()
