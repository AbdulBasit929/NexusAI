import unittest
from types import SimpleNamespace

from ingestion.forensic_records.mention_extractor import extract_from_passages, extract_mentions, fold_digits

PLACES = {"model town": ["Model Town", "ماڈل ٹاؤن"], "gulberg": ["Gulberg", "گلبرگ"]}


def pairs(text, **kw):
    return [(m.kind, m.value) for m in extract_mentions(text, PLACES, **kw)]


class MobileNumbers(unittest.TestCase):
    def test_every_written_form_gives_one_key(self):
        for text in ("0300-1110001", "0300 1110001", "03001110001", "+92 300 1110001", "+92-300-1110001", "0092 300 1110001", "92 300 1110001", "(0300) 1110001", "+92 (0) 300 1110001"):
            self.assertEqual(pairs("mobile " + text), [("msisdn", "923001110001")], text)

    def test_urdu_and_arabic_digits_are_folded_and_positions_stay_in_the_text_as_written(self):
        text = "نمبر ۰۳۰۰۱۱۱۰۰۰۱ ہے"
        (mention,) = extract_mentions(text)
        self.assertEqual((mention.value, mention.surface), ("923001110001", "۰۳۰۰۱۱۱۰۰۰۱"))
        self.assertEqual(text[mention.start:mention.end], mention.surface)
        self.assertEqual(fold_digits("٠١٢"), "012")

    def test_what_is_not_a_mobile_number_is_left_alone(self):
        for text in ("landline 042-35761234", "REF00000394", "amount 1,234,567.00", "date 20260305", "run 112233445566", "inside 9903001110001555", "prefix 0399-1110001", "ends 03001110001234"):
            self.assertEqual([k for k, _ in pairs(text) if k == "msisdn"], [], text)

    def test_a_bare_ten_digit_number_needs_a_cue(self):
        self.assertEqual(pairs("mobile no 3001110001"), [("msisdn", "923001110001")])
        self.assertEqual(pairs("ref 3001110001"), [])

    def test_a_list_of_numbers_gives_each_number(self):
        found = pairs("0300-1110001 0301-2223334, +92 302 3334445")
        self.assertEqual(found, [("msisdn", "923001110001"), ("msisdn", "923012223334"), ("msisdn", "923023334445")])


class OtherIdentifiers(unittest.TestCase):
    def test_national_id_dashed_spaced_and_run_with_a_cue(self):
        self.assertEqual(pairs("00000-1234567-1"), [("national_id", "00000-1234567-1")])
        self.assertEqual(pairs("00000 1234567 1"), [("national_id", "00000-1234567-1")])
        self.assertEqual(pairs("CNIC 0000012345671"), [("national_id", "00000-1234567-1")])
        self.assertEqual(pairs("serial 0000012345671"), [])

    def test_account_iban_with_and_without_spaces_and_case(self):
        for text in ("PK00 SYNT 0000 0000 0000 2004", "PK00SYNT0000000000002004", "pk00synt0000000000002004"):
            self.assertEqual(pairs("account " + text), [("account", "PK00SYNT0000000000002004")], text)

    def test_iccid_needs_the_luhn_check(self):
        self.assertEqual(pairs("SIM 8900000000000010002"), [("iccid", "8900000000000010002")])
        self.assertEqual(pairs("SIM 8900000000000010003"), [])
        self.assertEqual(pairs("SIM 8900000000000010003", include_low=True), [("iccid", "8900000000000010003")])

    def test_imei_needs_luhn_and_a_cue_for_the_high_tier_and_imsi_needs_its_cue(self):
        self.assertEqual(pairs("IMEI 000000000010009"), [("imei", "000000000010009")])
        self.assertEqual(pairs("number 000000000010009"), [])
        self.assertEqual(pairs("number 000000000010009", include_low=True), [("imei", "000000000010009")])
        self.assertEqual(pairs("IMEI 123456789012345"), [])
        self.assertEqual(pairs("IMSI 001010000001000"), [("imsi", "001010000001000")])

    def test_the_cue_may_sit_in_the_neighbouring_cell(self):
        passages = [SimpleNamespace(text="Mobile handset", locator={"paragraph": 5}), SimpleNamespace(text="000000000010009", locator={"paragraph": 6})]
        found = extract_from_passages(passages)
        self.assertEqual([(m["kind"], m["value"], m["locator"]["paragraph"]) for m in found], [("imei", "000000000010009", 6)])


class PlatesAndPlaces(unittest.TestCase):
    def test_a_plate_beside_a_cue_in_each_writing(self):
        for text in ("vehicle SYNT-4024", "registration synt 4024", "gari SYNT4024 thi", "گاڑی SYNT-۴۰۲۴", "plate LEA-21-1234"):
            kinds = [k for k, _ in pairs(text)]
            self.assertEqual(kinds, ["plate"], text)
        self.assertEqual(pairs("vehicle LEA-21-1234"), [("plate", "LEA211234")])

    def test_legal_and_office_tokens_are_not_plates(self):
        for text in ("vehicle FIR 45/26", "vehicle PPC 420", "vehicle Section 161", "vehicle SYNT4O24", "SYNT-4024 without any cue"):
            self.assertEqual([k for k, _ in pairs(text) if k == "plate"], [], text)

    def test_places_in_latin_and_urdu_with_urdu_punctuation_around(self):
        self.assertEqual(pairs("seen in Model Town today"), [("place", "model town")])
        self.assertEqual(pairs("تھانہ ماڈل ٹاؤن، ضلع لاہور"), [("place", "model town")])
        self.assertEqual(pairs("علاقہ گلبرگ۔"), [("place", "gulberg")])
        self.assertEqual(pairs("Gulbergs"), [])


class Claims(unittest.TestCase):
    def test_a_span_is_claimed_once_by_the_most_specific_kind(self):
        found = extract_mentions("CNIC 00000-1234567-1 and mobile 0300-1110001")
        self.assertEqual([m.kind for m in found], ["national_id", "msisdn"])

    def test_positions_point_at_the_surface(self):
        text = "Call +92 300 1110001 now"
        (m,) = extract_mentions(text)
        self.assertEqual(text[m.start:m.end], m.surface)


if __name__ == "__main__":
    unittest.main()
