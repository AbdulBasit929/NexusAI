import unittest
from html.parser import HTMLParser
from pathlib import Path


TARGET = Path(__file__).parents[1] / "tools" / "r8_t2_capture" / "capture-assistant.html"


class ElementParser(HTMLParser):
    def __init__(self):
        super().__init__()
        self.ids = set()

    def handle_starttag(self, _tag, attrs):
        identity = dict(attrs).get("id")
        if identity:
            self.ids.add(identity)


class CaptureAssistantTest(unittest.TestCase):
    def test_local_assistant_has_required_fail_closed_controls(self):
        source = TARGET.read_text(encoding="utf-8")
        parser = ElementParser(); parser.feed(source)
        self.assertTrue({"planFile", "folderButton", "cameraButton", "captureButton", "camera", "cameraDiagnostic", "arrangement", "targetReference"} <= parser.ids)
        self.assertIn("navigator.mediaDevices.getUserMedia", source)
        self.assertIn("showDirectoryPicker", source)
        self.assertIn("payload.items.length !== 32", source)
        self.assertIn("image/jpeg", source)
        self.assertIn("does not create ground truth", source)
        self.assertIn("How to arrange this slot", source)
        self.assertIn("about 25–45% of image width", source)
        self.assertIn("Cover about 20–30%", source)
        self.assertIn("sealed holdout", source)
        self.assertIn("Live camera frame verified", source)
        self.assertIn("No camera frame arrived within 10 seconds", source)
        self.assertNotIn("fetch(", source)


if __name__ == "__main__":
    unittest.main()
