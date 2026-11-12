import sys
import unittest
import uuid
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE.parent))
sys.path.insert(0, str(HERE.parents[2] / "tools" / "media_data"))

from media_artifact_scope import resolve_media_artifact_id, scoped_artifact_id  # noqa: E402
import patch_worker_artifact_scope as patcher  # noqa: E402


class FakeCursor:
    """Answers the one query of resolve_media_artifact_id from a dictionary of artifact id to the version that holds it."""

    def __init__(self, held, as_dict=False):
        self.held, self.as_dict, self.asked = held, as_dict, []
        self._row = None

    def execute(self, sql, params):
        assert "forensic.derived_artifacts" in sql and "artifact_id" in sql
        self.asked.append(params[0])
        version = self.held.get(params[0])
        self._row = None if version is None else ({"version_id": version} if self.as_dict else (version,))

    def fetchone(self):
        return self._row


CONTENT_ID = str(uuid.uuid5(uuid.NAMESPACE_URL, "sha:frame:3"))
V1, V2, V3 = "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222", "33333333-3333-4333-8333-333333333333"


class ResolveTests(unittest.TestCase):
    def test_a_content_id_nobody_holds_is_kept(self):
        self.assertEqual(resolve_media_artifact_id(FakeCursor({}), V1, CONTENT_ID), CONTENT_ID)

    def test_the_first_copy_and_every_replay_of_it_keep_the_content_id(self):
        self.assertEqual(resolve_media_artifact_id(FakeCursor({CONTENT_ID: V1}), V1, CONTENT_ID), CONTENT_ID)

    def test_a_second_copy_in_another_version_gets_an_id_of_its_own(self):
        resolved = resolve_media_artifact_id(FakeCursor({CONTENT_ID: V1}), V2, CONTENT_ID)
        self.assertNotEqual(resolved, CONTENT_ID)
        self.assertEqual(resolved, scoped_artifact_id(V2, CONTENT_ID))
        uuid.UUID(resolved)  # a valid uuid: the column is a uuid

    def test_the_scoped_id_is_the_same_every_time_so_a_replay_stores_nothing_twice(self):
        held = {CONTENT_ID: V1}
        first = resolve_media_artifact_id(FakeCursor(held), V2, CONTENT_ID)
        again = resolve_media_artifact_id(FakeCursor(held), V2, CONTENT_ID)
        self.assertEqual(first, again)

    def test_three_copies_of_one_file_have_three_ids(self):
        held = {CONTENT_ID: V1}
        ids = {resolve_media_artifact_id(FakeCursor(held), v, CONTENT_ID) for v in (V1, V2, V3)}
        self.assertEqual(len(ids), 3)

    def test_rows_as_dictionaries_are_read_too(self):
        self.assertEqual(resolve_media_artifact_id(FakeCursor({CONTENT_ID: V1}, as_dict=True), V2, CONTENT_ID), scoped_artifact_id(V2, CONTENT_ID))
        self.assertEqual(resolve_media_artifact_id(FakeCursor({CONTENT_ID: V1}, as_dict=True), V1, CONTENT_ID), CONTENT_ID)

    def test_it_asks_for_the_content_id_it_was_given(self):
        cur = FakeCursor({})
        resolve_media_artifact_id(cur, V1, CONTENT_ID)
        self.assertEqual(cur.asked, [CONTENT_ID])


class WorkerPatchTests(unittest.TestCase):
    def test_the_repository_worker_stores_a_reading_under_the_resolved_id(self):
        text = (HERE.parent / "worker.py").read_text(encoding="utf-8")
        self.assertIn("from ingestion.forensic_records.media_artifact_scope import resolve_media_artifact_id", text)
        self.assertIn("artifact_id = resolve_media_artifact_id(cur, version_id, observation.observation_id)", text)
        self.assertIn("persist_anpr_media_observation(cur, job, observation, index, artifact_id)", text)
        self.assertEqual(text.count("artifact_id or observation.observation_id"), 2, "the plate record's payload and its link carry the resolved id")
        self.assertNotIn("                observation.observation_id,\n                job.tenant_id,", text.replace("\r\n", "\n"), "the artifact insert no longer takes the content id directly")

    def test_the_patch_refuses_a_text_that_lacks_an_anchor_and_a_text_that_has_it_twice(self):
        with self.assertRaises(SystemExit):
            patcher.patch("def persist_media_result():\n    pass\n")
        anchor = "try:\n    from ingestion.forensic_records.media_pipeline import (\n"
        with self.assertRaises(SystemExit):
            patcher.patch(anchor + anchor)

    def test_the_patch_keeps_mixed_line_endings_line_by_line(self):
        sample = (
            "try:\r\n    from ingestion.forensic_records.media_pipeline import (\r\n        A,\r\n    )\r\nexcept X:\r\n    pass\r\n"
        )
        # only the first edit's anchor is present: the rest must refuse, never half-apply
        with self.assertRaises(SystemExit):
            patcher.patch(sample)


if __name__ == "__main__":
    unittest.main()
