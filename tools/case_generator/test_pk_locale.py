"""Tests for the Pakistani locale pack (plain unittest):  python -X utf8 tools/case_generator/test_pk_locale.py

Proves: an identity is the same every time it is asked for and different for different people; the national ID number always begins 00000 (no real province or district has that code), has 13
digits in the 5-7-1 form and ends odd for a man and even for a woman; names carry an Urdu spelling; a site is about 2 km from its north and its east neighbour on the Lahore grid (so the movement
plants keep their meaning) and lies in Lahore; every site has an area label; a locale that is not known is refused; a case without a locale writes the parameters as before (no new key);
narrations name no account and no real branch."""
import math
import os
import re
import sys
import unittest

HERE = os.path.dirname(os.path.abspath(__file__))
sys.path.insert(0, HERE)
import pk_locale  # noqa: E402
import world as w  # noqa: E402


def distance_km(a, b):
    p1, p2 = math.radians(a[0]), math.radians(b[0])
    h = math.sin((p2 - p1) / 2) ** 2 + math.cos(p1) * math.cos(p2) * math.sin(math.radians(b[1] - a[1]) / 2) ** 2
    return 6371.0 * 2 * math.asin(math.sqrt(h))


class IdentityTests(unittest.TestCase):
    def test_same_person_same_identity_and_people_differ(self):
        first = pk_locale.identity("lahore", 7, 2, 5)
        self.assertEqual(first, pk_locale.identity("lahore", 7, 2, 5))
        names = {pk_locale.identity("lahore", n, n % 5, n % 12)["name"] for n in range(200)}
        self.assertGreater(len(names), 60)

    def test_national_id_is_invalid_by_construction_and_follows_the_gender_rule(self):
        seen = {"M": 0, "F": 0}
        for n in range(400):
            who = pk_locale.identity("lahore", n, n % 6, n % 12)
            self.assertRegex(who["national_id"], r"^00000-\d{7}-\d$")
            last = int(who["national_id"][-1])
            self.assertEqual(last % 2, 1 if who["gender"] == "M" else 0, who["national_id"])
            seen[who["gender"]] += 1
        self.assertGreater(seen["M"], 0)
        self.assertGreater(seen["F"], 0)

    def test_names_have_an_urdu_spelling_and_the_father_carries_the_surname(self):
        for n in range(50):
            who = pk_locale.identity("lahore", n, n % 6, 0)
            self.assertTrue(re.search(r"[؀-ۿ]", who["name_urdu"]), who)
            self.assertTrue(re.search(r"[؀-ۿ]", who["father_urdu"]), who)
            self.assertEqual(who["name"].split()[-1], who["father"].split()[-1])
            self.assertIn(who["relation"], ("s/o", "d/o"))

    def test_address_places_the_person_in_the_area_of_the_home_site(self):
        who = pk_locale.identity("lahore", 3, 0, 6)
        self.assertIn("Model Town", who["address"])
        self.assertTrue(who["address"].endswith("Lahore"))


class GridTests(unittest.TestCase):
    def test_sites_are_about_two_kilometres_apart_in_both_directions_and_in_lahore(self):
        for operator in range(4):
            sites = [w.cell_geometry(operator, idx, "lahore") for idx in range(12)]
            for lat, lon in sites:
                self.assertTrue(31.4 < lat < 31.6 and 74.2 < lon < 74.4, (lat, lon))
            self.assertAlmostEqual(distance_km(sites[0], sites[4]), 2.2, delta=0.15)
            self.assertAlmostEqual(distance_km(sites[0], sites[1]), 2.2, delta=0.15)
            self.assertAlmostEqual(distance_km(sites[5], sites[6]), 2.2, delta=0.15)

    def test_the_default_grid_is_unchanged(self):
        self.assertEqual(w.cell_geometry(0, 0), (0.4, -30.6))
        self.assertEqual(w.cell_geometry(2, 5), (round(0.40 + 0.02 + 0.0014, 6), round(-30.60 + 0.02 + 0.0022, 6)))

    def test_every_site_has_a_label_and_an_unknown_locale_is_refused(self):
        labels = [pk_locale.site_label("lahore", idx) for idx in range(12)]
        self.assertEqual(len(set(labels)), 12)
        self.assertTrue(all("Site %02d" % idx in labels[idx] for idx in range(12)))
        with self.assertRaises(ValueError):
            pk_locale.grid_origin("atlantis")


class ParameterTests(unittest.TestCase):
    def test_a_case_without_a_locale_writes_the_parameters_as_before(self):
        self.assertNotIn("locale", w.params_dict(w.Params(seed=1)))
        self.assertEqual(w.params_dict(w.Params(seed=1, locale="lahore"))["locale"], "lahore")


class NarrationTests(unittest.TestCase):
    def test_a_narration_names_no_account_and_uses_the_pakistani_rails(self):
        import random
        channels = ("IBFT", "Raast", "ATM", "POS", "Cheque", "Cash", "Branch", "Online")
        for channel in channels:
            for direction in ("credit", "debit"):
                text = pk_locale.narration(random.Random(5), channel, direction, "PK00SYNT0000000000005001")
                self.assertNotRegex(text, r"PK\d\d|SYNT")
                self.assertTrue(text)
        self.assertIn("1LINK", pk_locale.narration(random.Random(1), "ATM", "debit", "x"))
        self.assertIn("IBFT", pk_locale.narration(random.Random(1), "IBFT", "debit", "x"))


if __name__ == "__main__":
    unittest.main()
