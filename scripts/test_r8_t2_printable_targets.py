import unittest
from html.parser import HTMLParser
from pathlib import Path


TARGET = Path(__file__).parents[1] / "tools" / "r8_t2_capture" / "printable-test-targets.html"


class CountingParser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.sheets = 0
        self.targets = 0
        self.warnings = 0

    def handle_starttag(self, tag, attrs):
        classes = dict(attrs).get("class", "").split()
        self.sheets += int(tag == "section" and "sheet" in classes)
        self.targets += int("target" in classes)
        self.warnings += int("warning" in classes)


class PrintableTargetsTest(unittest.TestCase):
    def test_two_complete_pages_without_terminal_forced_break(self):
        source = TARGET.read_text(encoding="utf-8")
        parser = CountingParser()
        parser.feed(source)
        self.assertEqual(2, parser.sheets)
        self.assertEqual(8, parser.targets)
        self.assertEqual(2, parser.warnings)
        self.assertIn("height: 190mm", source)
        self.assertIn(".sheet:last-of-type { break-after: auto; page-break-after: auto; }", source)
        self.assertNotIn('<div class="warning">CONTROLLED SYNTHETIC TEST TARGETS', source.split('<section class="sheet">', 1)[0])


if __name__ == "__main__":
    unittest.main()
