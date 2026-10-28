"""A minimal PDF writer with no library: pages of positioned text (the standard Courier and Helvetica fonts, no embedding) and rules. Deterministic: the same pages give the same bytes (no date, no id).

`pdftotext -layout` reconstructs the columns of a monospaced table from the positions of the characters, which is how the statement reader (ingestion/forensic_records/layout_registry/statement_readers.py)
reads these files; Courier has a fixed advance (0.6 of the font size), so a column of characters is a column on the page.

A page is a list of items: ("text", x, top, font, size, text) with x and top in points from the top-left corner of an A4 page, font one of "mono", "mono-bold", "sans", "sans-bold";
("rule", x1, top1, x2, top2, width). Text is Latin-1 (WinAnsi): this writer is for the English statements of the showcase, not for Urdu.
"""
PAGE_W, PAGE_H = 595, 842
FONTS = {"mono": ("F1", "Courier"), "mono-bold": ("F2", "Courier-Bold"), "sans": ("F3", "Helvetica"), "sans-bold": ("F4", "Helvetica-Bold")}
MONO_ADVANCE = 0.6


def _escape(text):
    return text.encode("latin-1", "replace").replace(b"\\", b"\\\\").replace(b"(", b"\\(").replace(b")", b"\\)")


def _content(items):
    out = []
    for item in items:
        if item[0] == "text":
            _, x, top, font, size, text = item
            out.append(b"BT /%s %s Tf %.2f %.2f Td (" % (FONTS[font][0].encode(), (("%g" % size).encode()), x, PAGE_H - top) + _escape(text) + b") Tj ET")
        elif item[0] == "rule":
            _, x1, t1, x2, t2, width = item
            out.append(b"%g w %.2f %.2f m %.2f %.2f l S" % (width, x1, PAGE_H - t1, x2, PAGE_H - t2))
    return b"\n".join(out)


def build_pdf(pages, title=""):
    objects = []   # bytes of each object body, numbered from 1

    def add(body):
        objects.append(body)
        return len(objects)
    catalog, pages_root = add(b""), add(b"")
    font_ids = {key: add(b"<< /Type /Font /Subtype /Type1 /BaseFont /%s /Encoding /WinAnsiEncoding >>" % name.encode()) for key, (_, name) in FONTS.items()}
    font_dict = b"<< " + b" ".join(b"/%s %d 0 R" % (FONTS[key][0].encode(), font_ids[key]) for key in FONTS) + b" >>"
    info = add(b"<< /Producer (NexusAI test statement writer) /Title (" + _escape(title) + b") >>")
    kids = []
    for items in pages:
        stream = _content(items)
        content_id = add(b"<< /Length %d >>\nstream\n" % len(stream) + stream + b"\nendstream")
        kids.append(add(b"<< /Type /Page /Parent %d 0 R /MediaBox [0 0 %d %d] /Resources << /Font %s >> /Contents %d 0 R >>" % (pages_root, PAGE_W, PAGE_H, font_dict, content_id)))
    objects[catalog - 1] = b"<< /Type /Catalog /Pages %d 0 R >>" % pages_root
    objects[pages_root - 1] = b"<< /Type /Pages /Kids [" + b" ".join(b"%d 0 R" % k for k in kids) + b"] /Count %d >>" % len(kids)
    out = bytearray(b"%PDF-1.4\n%\xe2\xe3\xcf\xd3\n")
    offsets = []
    for number, body in enumerate(objects, start=1):
        offsets.append(len(out))
        out += b"%d 0 obj\n" % number + body + b"\nendobj\n"
    xref = len(out)
    out += b"xref\n0 %d\n0000000000 65535 f \n" % (len(objects) + 1)
    for offset in offsets:
        out += b"%010d 00000 n \n" % offset
    out += b"trailer\n<< /Size %d /Root %d 0 R /Info %d 0 R >>\nstartxref\n%d\n%%%%EOF\n" % (len(objects) + 1, catalog, info, xref)
    return bytes(out)
