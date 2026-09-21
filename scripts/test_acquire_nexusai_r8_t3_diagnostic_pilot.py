import hashlib
import unittest

from scripts.acquire_nexusai_r8_t3_diagnostic_pilot import git_blob_sha1


class DiagnosticPilotAcquisitionTests(unittest.TestCase):
    def test_git_blob_identity_includes_header(self):
        payload = b"test\n"
        expected = hashlib.sha1(b"blob 5\0test\n").hexdigest()
        self.assertEqual(git_blob_sha1(payload), expected)


if __name__ == "__main__":
    unittest.main()
