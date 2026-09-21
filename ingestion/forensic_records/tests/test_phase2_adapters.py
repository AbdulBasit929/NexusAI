import csv
import json
import unittest
from datetime import timezone
from pathlib import Path

from ingestion.forensic_records.worker import (
    IngestJob,
    ADAPTERS,
    normalize_generic_row,
    normalize_cdr_row,
    parse_money_amount,
    parse_ts_any,
    resolve_adapter,
    validate_pk_iban,
    validate_cdr_headers,
)


REPO_ROOT = Path(__file__).resolve().parents[3]
PAKISTAN_CDR_FIXTURE = Path(__file__).parent / "fixtures" / "pakistan_cdr_messy_synthetic.csv"
PAKISTAN_ANPR_FIXTURE = Path(__file__).parent / "fixtures" / "pakistan_anpr_messy_synthetic.csv"
PAKISTAN_FINANCIAL_FIXTURE = Path(__file__).parent / "fixtures" / "pakistan_financial_messy_synthetic.jsonl"
PAKISTAN_STRUCTURED_GOLDENS = Path(__file__).parent / "fixtures" / "pakistan_structured_goldens_v1.json"
PAKISTAN_SUBSCRIBER_FIXTURE = Path(__file__).parent / "fixtures" / "pakistan_subscriber_messy_synthetic.jsonl"
PAKISTAN_TOWER_FIXTURE = Path(__file__).parent / "fixtures" / "pakistan_tower_sector_messy_synthetic.csv"
PAKISTAN_ACCESS_FIXTURE = Path(__file__).parent / "fixtures" / "pakistan_access_security_messy_synthetic.jsonl"
PAKISTAN_STRUCTURED_GOLDENS_V2 = Path(__file__).parent / "fixtures" / "pakistan_structured_goldens_v2.json"
PAKISTAN_PROFILE = REPO_ROOT / "configuration" / "forensic_country_profiles" / "pakistan.json"


class Phase2AdapterTests(unittest.TestCase):
    def job(self, record_type="auto"):
        return IngestJob(
            job_id="00000000-0000-0000-0000-000000000001",
            tenant_id="default",
            user_id="",
            collection_id="records-demo",
            file_id="file-1",
            source_file="sample.csv",
            spool_path="sample.csv",
            sha256="0" * 64,
            record_type=record_type,
            headers=[],
            metadata={},
        )

    def pk_job(self, record_type):
        base = self.job(record_type)
        return IngestJob(**{**base.__dict__, "metadata": {"source_timezone": "Asia/Karachi", "jurisdiction": "PK"}})

    def test_detects_anpr_headers(self):
        adapter = resolve_adapter(self.job(), ["plate_number", "camera_id", "capture_time", "location"])
        self.assertEqual(adapter.name, "anpr")

    def test_detects_pakistan_vendor_anpr_and_financial_aliases(self):
        anpr = resolve_adapter(
            self.job(),
            ["Registration No.", "Camera Code", "Captured At", "Camera Location"],
        )
        transaction = resolve_adapter(
            self.job(),
            ["txn_id", "txn_time", "debit_account", "debit_amount", "currency_code"],
        )
        self.assertEqual(anpr.name, "anpr")
        self.assertEqual(transaction.name, "transaction")

    def test_detects_ipdr_headers(self):
        adapter = resolve_adapter(self.job(), ["session_start", "source_ip", "destination_ip", "bytes"])
        self.assertEqual(adapter.name, "ipdr")

    def test_detects_subscriber_registry_headers(self):
        adapter = resolve_adapter(self.job("subscriber"), ["msisdn", "subscriber_name", "cnic_last4", "home_city"])
        self.assertEqual(adapter.name, "subscriber")

    def test_requested_adapter_requires_matching_headers(self):
        with self.assertRaises(ValueError):
            resolve_adapter(self.job("anpr"), ["source_ip", "destination_ip"])

    def test_access_log_adapter_accepts_ready_fixture_client_ip_alias(self):
        adapter = resolve_adapter(
            self.job("access_log"),
            ["timestamp", "client_ip", "method", "path", "status", "user"],
        )
        self.assertEqual(adapter.name, "access_log")

    def test_generic_row_preserves_raw_and_extracts_entities(self):
        raw = {
            "plate_number": "ABC-123",
            "camera_id": "CAM-7",
            "capture_time": "2026-07-14 10:11:12",
            "location": "Gate 4",
        }
        normalized = normalize_generic_row(self.pk_job("anpr"), raw, "00000000-0000-0000-0000-000000000001", 2, ADAPTERS["anpr"])
        self.assertEqual(normalized["record_type"], "anpr")
        self.assertEqual(normalized["primary_entity"], "ABC-123")
        self.assertEqual(normalized["secondary_entity"], "CAM-7")
        self.assertEqual(normalized["location"], "Gate 4")
        self.assertIsNotNone(normalized["observed_at"])
        self.assertIn("plate_number", normalized["raw_record"])

    def test_cdr_row_accepts_iso_timestamps_and_formatted_numbers(self):
        raw = {
            "MSISDN": "+92 346 167 8183",
            "call_org_num": "+92-300-111-2222",
            "CALL_DIALED_NUM": "923334445555",
            "IMSI": "410010123456789",
            "IMEI": "356789012345678",
            "CALL_START_DT_TM": "2026-07-10T21:04:12Z",
            "CALL_END_DT_TM": "2026-07-10T21:08:02Z",
            "INBOUND_OUTBOUND_IND": "OUTGOING",
            "Call_Network_Volume": "0",
            "Lac_Id": "LAC-12",
            "Site_Id": "SITE-014",
            "Cell_SITE_ID": "LHR-GUL-014",
            "lat": "31.520604",
            "longitude": "74.358747",
            "CALL_TYPE": "VOICE",
            "location": "Gulberg",
        }
        normalized = normalize_cdr_row(self.job("cdr"), raw, "00000000-0000-0000-0000-000000000001", 2)
        self.assertEqual(normalized["duration_seconds"], 230)
        self.assertEqual(normalized["direction"], "OUTGOING")
        self.assertEqual(normalized["cell_site_id"], "LHR-GUL-014")
        self.assertIsNotNone(normalized["call_start_ts"])

    def test_cdr_adapter_accepts_modern_alias_fixture(self):
        headers = [
            "call_id",
            "call_time",
            "source_number",
            "target_number",
            "duration_seconds",
            "area",
        ]
        adapter = resolve_adapter(self.job("cdr"), headers)
        self.assertEqual(adapter.name, "cdr")
        validate_cdr_headers(headers)

        normalized = normalize_cdr_row(
            self.pk_job("cdr"),
            {
                "call_id": "cdr-001",
                "call_time": "2026-07-10 08:00:00",
                "source_number": "03001234567",
                "target_number": "03111234567",
                "duration_seconds": "45",
                "area": "Gulberg",
            },
            "00000000-0000-0000-0000-000000000001",
            2,
        )
        self.assertEqual(normalized["duration_seconds"], 45)
        self.assertEqual(normalized["call_org_num"], "03001234567")
        self.assertEqual(normalized["call_dialed_num"], "03111234567")
        self.assertEqual(normalized["location"], "Gulberg")
        self.assertIsNotNone(normalized["call_start_ts"])

    def test_pakistan_source_timezone_converts_naive_timestamps_to_utc(self):
        base_job = self.job("cdr")
        job = IngestJob(**{**base_job.__dict__, "metadata": {"source_timezone": "Asia/Karachi"}})
        normalized = normalize_cdr_row(
            job,
            {
                "MSISDN": "923001234567",
                "CALL_DIALED_NUM": "923111234567",
                "CALL_START_DT_TM": "2026-07-10 08:00:00",
                "CALL_END_DT_TM": "2026-07-10 08:00:45",
            },
            "00000000-0000-0000-0000-000000000001",
            2,
        )
        self.assertEqual(normalized["call_start_ts"].tzinfo, timezone.utc)
        self.assertEqual(normalized["call_start_ts"].isoformat(), "2026-07-10T03:00:00+00:00")
        self.assertEqual(normalized["duration_seconds"], 45)

    def test_explicit_timestamp_offset_overrides_pakistan_default(self):
        parsed = parse_ts_any("2026-07-10T08:00:00+03:00", "Asia/Karachi")
        self.assertIsNotNone(parsed)
        self.assertEqual(parsed.isoformat(), "2026-07-10T05:00:00+00:00")

    def test_unknown_source_timezone_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "unknown source timezone"):
            parse_ts_any("2026-07-10 08:00:00", "Invalid/Nowhere")

    def test_synthetic_pakistan_cdr_fixture_preserves_messy_provider_tokens(self):
        with PAKISTAN_CDR_FIXTURE.open(newline="", encoding="utf-8-sig") as handle:
            rows = list(csv.DictReader(handle))
        self.assertEqual(len(rows), 5)
        self.assertEqual(rows[1]["Cell_SITE_ID"], "-1")
        self.assertEqual(rows[1]["CALL_DIALED_NUM"], "INTERNET")
        self.assertEqual(rows[2]["CALL_DIALED_NUM"], "*123#")
        self.assertEqual(rows[2]["location"], "اسلام آباد")
        self.assertEqual(rows[0], rows[4])

        normalized = normalize_cdr_row(
            self.pk_job("cdr"), rows[1], "00000000-0000-0000-0000-000000000001", 3
        )
        self.assertEqual(normalized["call_dialed_num"], "INTERNET")
        self.assertEqual(normalized["cell_site_id"], "-1")
        self.assertIsNone(normalized["lac_id"])
        self.assertIsNone(normalized["latitude"])
        self.assertEqual(len(normalized["imei"]), 14)

    def test_pakistan_profile_covers_required_local_identifiers_and_modalities(self):
        profile = json.loads(PAKISTAN_PROFILE.read_text(encoding="utf-8"))
        self.assertEqual(profile["jurisdiction"]["default_timezone"], "Asia/Karachi")
        self.assertEqual(profile["identifiers"]["imsi"]["pakistan_mcc"], "410")
        self.assertEqual(profile["identifiers"]["iban"]["total_characters"], 24)
        self.assertIn(14, profile["identifiers"]["imei"]["accepted_digits"])
        self.assertEqual(profile["identifiers"]["vehicle_registration"]["strategy"], "province_series_and_period_specific")
        self.assertEqual(
            set(profile["evidence_family_requirements"]),
            {
                "cdr_ipdr_and_logs",
                "anpr",
                "financial_transactions",
                "audio_stt_and_tts",
                "images_documents_and_video",
                "databases_archives_and_office_files",
            },
        )

    def test_pakistan_anpr_golden_has_exact_accounting_and_preserves_urdu(self):
        manifest = json.loads(PAKISTAN_STRUCTURED_GOLDENS.read_text(encoding="utf-8"))["datasets"]["anpr"]
        with PAKISTAN_ANPR_FIXTURE.open(newline="", encoding="utf-8-sig") as handle:
            rows = list(csv.DictReader(handle))
        self.assertEqual(len(rows), manifest["total_rows"])

        accepted = []
        rejected = []
        for row_number, raw in enumerate(rows, start=2):
            try:
                accepted.append(
                    normalize_generic_row(
                        self.pk_job("anpr"),
                        raw,
                        "00000000-0000-0000-0000-000000000001",
                        row_number,
                        ADAPTERS["anpr"],
                    )
                )
            except ValueError as exc:
                rejected.append(str(exc))

        self.assertEqual(len(accepted), manifest["accepted_rows"])
        self.assertEqual(len(rejected), manifest["rejected_rows"])
        self.assertEqual(len(accepted) - len({row["row_hash"] for row in accepted}), manifest["duplicate_rows"])
        for fragment in manifest["expected_rejection_fragments"]:
            self.assertTrue(any(fragment in message for message in rejected), (fragment, rejected))

        first = json.loads(accepted[0]["normalized_record"])
        urdu = json.loads(accepted[2]["normalized_record"])
        low_confidence = json.loads(accepted[3]["normalized_record"])
        self.assertEqual(accepted[0]["observed_at"].isoformat(), manifest["exact_checks"]["first_timestamp_utc"])
        self.assertEqual(accepted[1]["observed_at"].isoformat(), manifest["exact_checks"]["explicit_offset_timestamp_utc"])
        self.assertEqual(first["ocr_confidence"], "0.98")
        self.assertEqual(urdu["plate_raw"], "اسلام آباد ۱۲۳")
        self.assertEqual(urdu["plate_search_key"], manifest["exact_checks"]["urdu_plate_search_key"])
        self.assertEqual(urdu["plate_script"], "arabic_derived")
        self.assertTrue(low_confidence["manual_review_required"])
        self.assertIn("low_ocr_confidence", low_confidence["quality_flags"])
        self.assertIn("لاہور", accepted[0]["raw_record"])

    def test_pakistan_financial_golden_preserves_exact_pkr_and_visible_rejections(self):
        manifest = json.loads(PAKISTAN_STRUCTURED_GOLDENS.read_text(encoding="utf-8"))["datasets"]["financial_transactions"]
        rows = [json.loads(line) for line in PAKISTAN_FINANCIAL_FIXTURE.read_text(encoding="utf-8-sig").splitlines() if line.strip()]
        self.assertEqual(len(rows), manifest["total_rows"])

        accepted = []
        rejected = []
        for row_number, raw in enumerate(rows, start=2):
            try:
                accepted.append(
                    normalize_generic_row(
                        self.pk_job("transaction"),
                        raw,
                        "00000000-0000-0000-0000-000000000001",
                        row_number,
                        ADAPTERS["transaction"],
                    )
                )
            except ValueError as exc:
                rejected.append(str(exc))

        self.assertEqual(len(accepted), manifest["accepted_rows"])
        self.assertEqual(len(rejected), manifest["rejected_rows"])
        self.assertEqual(len(accepted) - len({row["row_hash"] for row in accepted}), manifest["duplicate_rows"])
        for fragment in manifest["expected_rejection_fragments"]:
            self.assertTrue(any(fragment in message for message in rejected), (fragment, rejected))

        formatted = json.loads(accepted[0]["normalized_record"])
        smallest = json.loads(accepted[2]["normalized_record"])
        reversal = json.loads(accepted[3]["normalized_record"])
        invalid_iban = json.loads(accepted[4]["normalized_record"])
        missing_time = json.loads(accepted[5]["normalized_record"])
        self.assertEqual(formatted["amount_decimal"], manifest["exact_checks"]["formatted_pkr_decimal"])
        self.assertEqual(formatted["amount_minor_units"], manifest["exact_checks"]["formatted_pkr_minor_units"])
        self.assertTrue(formatted["primary_pk_iban_valid"])
        self.assertTrue(formatted["secondary_pk_iban_valid"])
        self.assertEqual(smallest["amount_decimal"], manifest["exact_checks"]["smallest_minor_unit_decimal"])
        self.assertEqual(reversal["amount_decimal"], manifest["exact_checks"]["reversal_decimal"])
        self.assertEqual(reversal["amount_minor_units"], manifest["exact_checks"]["reversal_minor_units"])
        self.assertTrue(reversal["is_reversal_or_chargeback"])
        self.assertTrue(invalid_iban["manual_review_required"])
        self.assertIn("primary_pk_iban_invalid", invalid_iban["quality_flags"])
        self.assertTrue(missing_time["manual_review_required"])
        self.assertIn("timestamp_missing", missing_time["quality_flags"])
        self.assertIsNone(accepted[5]["observed_at"])
        self.assertIn("دکاندار تجرباتی", accepted[2]["raw_record"])

    def test_pkr_parser_and_pk_iban_validation_are_deterministic(self):
        amount, currency = parse_money_amount("Rs. 1,234.50", "PKR")
        self.assertEqual(str(amount), "1234.50")
        self.assertEqual(currency, "PKR")
        compact, valid = validate_pk_iban("PK65 TEST 0000 0000 0000 0000")
        self.assertEqual(compact, "PK65TEST0000000000000000")
        self.assertTrue(valid)
        _, invalid = validate_pk_iban("PK00TEST0000000000000000")
        self.assertFalse(invalid)

    def test_pakistan_subscriber_golden_normalizes_identifiers_and_flags_ambiguity(self):
        manifest = json.loads(PAKISTAN_STRUCTURED_GOLDENS_V2.read_text(encoding="utf-8"))["datasets"]["subscriber"]
        rows = [json.loads(line) for line in PAKISTAN_SUBSCRIBER_FIXTURE.read_text(encoding="utf-8-sig").splitlines() if line.strip()]
        accepted = []
        rejected = []
        for row_number, raw in enumerate(rows, start=2):
            try:
                accepted.append(normalize_generic_row(self.pk_job("subscriber"), raw, "00000000-0000-0000-0000-000000000001", row_number, ADAPTERS["subscriber"]))
            except ValueError as exc:
                rejected.append(str(exc))

        self.assertEqual(len(rows), manifest["total_rows"])
        self.assertEqual(len(accepted), manifest["accepted_rows"])
        self.assertEqual(len(rejected), manifest["rejected_rows"])
        self.assertEqual(len(accepted) - len({row["row_hash"] for row in accepted}), manifest["duplicate_rows"])
        for fragment in manifest["expected_rejection_fragments"]:
            self.assertTrue(any(fragment in message for message in rejected), (fragment, rejected))

        first = json.loads(accepted[0]["normalized_record"])
        urdu = json.loads(accepted[1]["normalized_record"])
        invalid = json.loads(accepted[2]["normalized_record"])
        missing_time = json.loads(accepted[4]["normalized_record"])
        self.assertEqual(first["msisdn_canonical"], manifest["exact_checks"]["first_msisdn_canonical"])
        self.assertEqual(accepted[0]["observed_at"].isoformat(), manifest["exact_checks"]["first_timestamp_utc"])
        self.assertTrue(first["cnic_format_valid"])
        self.assertTrue(first["imei_luhn_valid"])
        self.assertEqual(urdu["subscriber_name"], manifest["exact_checks"]["urdu_name"])
        self.assertTrue(set(manifest["exact_checks"]["invalid_identity_flags"]).issubset(invalid["quality_flags"]))
        self.assertIn(manifest["exact_checks"]["missing_timestamp_flag"], missing_time["quality_flags"])
        self.assertIn(manifest["exact_checks"]["urdu_name"], accepted[1]["raw_record"])

    def test_pakistan_tower_golden_preserves_sector_geometry_and_review_signals(self):
        manifest = json.loads(PAKISTAN_STRUCTURED_GOLDENS_V2.read_text(encoding="utf-8"))["datasets"]["tower_location"]
        with PAKISTAN_TOWER_FIXTURE.open(newline="", encoding="utf-8-sig") as handle:
            rows = list(csv.DictReader(handle))
        accepted = []
        rejected = []
        for row_number, raw in enumerate(rows, start=2):
            try:
                accepted.append(normalize_generic_row(self.pk_job("tower_location"), raw, "00000000-0000-0000-0000-000000000001", row_number, ADAPTERS["tower_location"]))
            except ValueError as exc:
                rejected.append(str(exc))

        self.assertEqual(len(rows), manifest["total_rows"])
        self.assertEqual(len(accepted), manifest["accepted_rows"])
        self.assertEqual(len(rejected), manifest["rejected_rows"])
        self.assertEqual(len(accepted) - len({row["row_hash"] for row in accepted}), manifest["duplicate_rows"])
        for fragment in manifest["expected_rejection_fragments"]:
            self.assertTrue(any(fragment in message for message in rejected), (fragment, rejected))

        first = json.loads(accepted[0]["normalized_record"])
        explicit = accepted[1]
        outside = json.loads(accepted[2]["normalized_record"])
        missing = json.loads(accepted[3]["normalized_record"])
        self.assertEqual(accepted[0]["observed_at"].isoformat(), manifest["exact_checks"]["first_timestamp_utc"])
        self.assertEqual(first["azimuth_degrees"], manifest["exact_checks"]["first_azimuth"])
        self.assertEqual(explicit["observed_at"].isoformat(), manifest["exact_checks"]["explicit_offset_timestamp_utc"])
        self.assertIn(manifest["exact_checks"]["outside_pakistan_flag"], outside["quality_flags"])
        self.assertIn(manifest["exact_checks"]["assumed_datum_flag"], missing["quality_flags"])

    def test_pakistan_access_log_golden_validates_network_and_event_fields(self):
        manifest = json.loads(PAKISTAN_STRUCTURED_GOLDENS_V2.read_text(encoding="utf-8"))["datasets"]["access_log"]
        rows = [json.loads(line) for line in PAKISTAN_ACCESS_FIXTURE.read_text(encoding="utf-8-sig").splitlines() if line.strip()]
        accepted = []
        rejected = []
        for row_number, raw in enumerate(rows, start=2):
            try:
                accepted.append(normalize_generic_row(self.pk_job("access_log"), raw, "00000000-0000-0000-0000-000000000001", row_number, ADAPTERS["access_log"]))
            except ValueError as exc:
                rejected.append(str(exc))

        self.assertEqual(len(rows), manifest["total_rows"])
        self.assertEqual(len(accepted), manifest["accepted_rows"])
        self.assertEqual(len(rejected), manifest["rejected_rows"])
        self.assertEqual(len(accepted) - len({row["row_hash"] for row in accepted}), manifest["duplicate_rows"])
        for fragment in manifest["expected_rejection_fragments"]:
            self.assertTrue(any(fragment in message for message in rejected), (fragment, rejected))

        first = json.loads(accepted[0]["normalized_record"])
        ipv6 = json.loads(accepted[1]["normalized_record"])
        missing_user = json.loads(accepted[4]["normalized_record"])
        self.assertEqual(accepted[0]["observed_at"].isoformat(), manifest["exact_checks"]["first_timestamp_utc"])
        self.assertEqual(first["http_method"], manifest["exact_checks"]["first_method"])
        self.assertEqual(ipv6["source_ip_canonical"], manifest["exact_checks"]["ipv6_canonical"])
        self.assertEqual(ipv6["user_or_principal"], manifest["exact_checks"]["urdu_principal"])
        self.assertIn(manifest["exact_checks"]["missing_user_flag"], missing_user["quality_flags"])
        self.assertIn(manifest["exact_checks"]["urdu_principal"], accepted[1]["raw_record"])


if __name__ == "__main__":
    unittest.main()
