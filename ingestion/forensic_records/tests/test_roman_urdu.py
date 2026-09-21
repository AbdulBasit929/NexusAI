from ingestion.forensic_records.roman_urdu import (
    ROMAN_URDU_SEGMENT_CONTRACT,
    contains_urdu_script,
    identifiers_in,
    roman_urdu_payload,
    romanize_urdu,
)


def test_roman_urdu_preserves_ascii_identifiers_exactly():
    raw = "اس آڈیو میں 03001234567 اور MN1367 کا ذکر 10:35 پر ہوا"
    roman = romanize_urdu(raw)

    assert "03001234567" in roman
    assert "MN1367" in roman
    assert "10:35" in roman
    assert identifiers_in(raw) == identifiers_in(roman)
    assert contains_urdu_script(raw)


def test_roman_urdu_payload_keeps_raw_urdu_authoritative():
    payload = roman_urdu_payload(
        "اس آڈیو میں کیا کہا گیا ہے؟",
        parent_observation_id="segment-1",
        evidence_id="evidence-1",
        version_id="version-1",
        source_file="clear-urdu.wav",
        start_seconds=0,
        end_seconds=4.2,
    )

    assert payload["contract_version"] == ROMAN_URDU_SEGMENT_CONTRACT
    assert payload["raw_urdu_text"] == "اس آڈیو میں کیا کہا گیا ہے؟"
    assert payload["roman_urdu_text"] != payload["raw_urdu_text"]
    assert payload["authority_state"] == "derived_analyst_representation"
    assert payload["identifier_preservation_pass"] is True
