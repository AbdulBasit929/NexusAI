"""Generate the deterministic text-bearing PDF used by NexusAI demo acceptance."""

from pathlib import Path

from reportlab import rl_config
from reportlab.lib import colors
from reportlab.lib.enums import TA_LEFT
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle, getSampleStyleSheet
from reportlab.lib.units import mm
from reportlab.platypus import Paragraph, SimpleDocTemplate, Spacer, Table, TableStyle


OUTPUT = Path("output/pdf/nexusai-multimodal-acceptance-brief.pdf")

# Keep the fixture byte-stable so its governed manifest hash is reproducible.
rl_config.invariant = 1


def build_pdf() -> None:
    OUTPUT.parent.mkdir(parents=True, exist_ok=True)
    styles = getSampleStyleSheet()
    title = ParagraphStyle(
        "AcceptanceTitle",
        parent=styles["Title"],
        fontName="Helvetica-Bold",
        fontSize=22,
        leading=27,
        textColor=colors.HexColor("#10213F"),
        alignment=TA_LEFT,
        spaceAfter=8 * mm,
    )
    heading = ParagraphStyle(
        "AcceptanceHeading",
        parent=styles["Heading2"],
        fontName="Helvetica-Bold",
        fontSize=13,
        leading=17,
        textColor=colors.HexColor("#1D4ED8"),
        spaceBefore=3 * mm,
        spaceAfter=2 * mm,
    )
    body = ParagraphStyle(
        "AcceptanceBody",
        parent=styles["BodyText"],
        fontName="Helvetica",
        fontSize=10.5,
        leading=15,
        textColor=colors.HexColor("#24324A"),
        spaceAfter=3 * mm,
    )

    doc = SimpleDocTemplate(
        str(OUTPUT),
        pagesize=A4,
        leftMargin=20 * mm,
        rightMargin=20 * mm,
        topMargin=18 * mm,
        bottomMargin=18 * mm,
        title="NexusAI Multimodal Acceptance Brief",
        author="NexusAI deterministic acceptance fixture",
        subject="Synthetic text-bearing PDF for bounded product acceptance",
    )

    story = [
        Paragraph("NexusAI Multimodal Acceptance Brief", title),
        Paragraph(
            "Synthetic acceptance fixture - not natural evidence. This document exists "
            "only to verify deterministic PDF extraction, retrieval, citations, and "
            "authorized source preview behavior.",
            body,
        ),
        Paragraph("Reference facts", heading),
    ]

    facts = [
        ["Field", "Ground-truth value"],
        ["Case reference", "NEXUS-DEMO-2026"],
        ["Plate candidate", "MN1367"],
        ["Contact number", "03001234567"],
        ["Observed time", "10:35 PKT"],
        ["Location", "Islamabad, Pakistan"],
        ["Review state", "Synthetic and review-required"],
    ]
    table = Table(facts, colWidths=[45 * mm, 115 * mm], repeatRows=1)
    table.setStyle(
        TableStyle(
            [
                ("BACKGROUND", (0, 0), (-1, 0), colors.HexColor("#10213F")),
                ("TEXTCOLOR", (0, 0), (-1, 0), colors.white),
                ("FONTNAME", (0, 0), (-1, 0), "Helvetica-Bold"),
                ("FONTNAME", (0, 1), (0, -1), "Helvetica-Bold"),
                ("FONTSIZE", (0, 0), (-1, -1), 9.5),
                ("LEADING", (0, 0), (-1, -1), 13),
                ("GRID", (0, 0), (-1, -1), 0.5, colors.HexColor("#CBD5E1")),
                ("ROWBACKGROUNDS", (0, 1), (-1, -1), [colors.white, colors.HexColor("#F8FAFC")]),
                ("VALIGN", (0, 0), (-1, -1), "MIDDLE"),
                ("LEFTPADDING", (0, 0), (-1, -1), 7),
                ("RIGHTPADDING", (0, 0), (-1, -1), 7),
                ("TOPPADDING", (0, 0), (-1, -1), 6),
                ("BOTTOMPADDING", (0, 0), (-1, -1), 6),
            ]
        )
    )
    story.extend(
        [
            table,
            Spacer(1, 5 * mm),
            Paragraph("Expected analyst question", heading),
            Paragraph(
                "Which source mentions plate MN1367, contact 03001234567, and time "
                "10:35, and what limitation applies?",
                body,
            ),
            Paragraph("Expected grounded answer", heading),
            Paragraph(
                "This PDF mentions all three identifiers. The source is a synthetic "
                "acceptance fixture and must not be represented as natural evidence. "
                "Any answer must cite the retained document passage and source version.",
                body,
            ),
            Paragraph("Acceptance boundaries", heading),
            Paragraph(
                "Passing behavior requires source-bound extraction, deterministic "
                "identifier preservation, case-scoped retrieval, visible citations, "
                "History persistence, and an authorized PDF source action. Scanned-only "
                "PDF OCR is outside this fixture's claim.",
                body,
            ),
        ]
    )
    doc.build(story)


if __name__ == "__main__":
    build_pdf()
