"""The identity a media reading is stored under, scoped to the evidence version that holds it.

A reading of a photo, a video frame or a recording (a plate, a group of plate reads, a piece of text, a technical observation) gets its id from the SHA-256 of the file and the reading's own position
(media_pipeline.py: uuid5 of "<source_sha256>:..."). That id says what the reading IS and is the same wherever the file is. It is also the primary key of forensic.derived_artifacts, and the worker
stores with ON CONFLICT (artifact_id) DO NOTHING. So the second copy of the same bytes in another case lost every reading it made: measured 2026-11-12 when the five real clips already read in one collection
were uploaded into the Orion case (the speech segments, whose id carries the evidence and the version, survived; every plate read, group and summary was dropped, and 49 plate records of the new case pointed
at artifacts of the other one).

`resolve_media_artifact_id` keeps every id that has no other holder exactly as it is (the first copy of a file, every replay of a version, all data stored before this module) and gives a reading whose
content id already belongs to ANOTHER evidence version an id of its own: uuid5 of "<version_id>:<content id>". The same version resolves to the same id every time, so a replay still stores nothing twice.
"""
import uuid


def scoped_artifact_id(version_id, observation_id):
    """The id of a reading of this evidence version whose content id is held by another version."""
    return str(uuid.uuid5(uuid.NAMESPACE_URL, "%s:%s" % (version_id, observation_id)))


def resolve_media_artifact_id(cur, version_id, observation_id):
    """The artifact id a reading is stored under: its content id, or the version-scoped id when another evidence version already holds the content id.

    `cur` is a cursor of the worker's connection (rows as tuples or as dicts); the table is read whole, not through a collection scope, because its primary key is global.
    """
    cur.execute("SELECT version_id::text FROM forensic.derived_artifacts WHERE artifact_id = %s::uuid", (observation_id,))
    row = cur.fetchone()
    if row is None:
        return observation_id
    holder = next(iter(row.values())) if isinstance(row, dict) else row[0]
    if str(holder) == str(version_id):
        return observation_id
    return scoped_artifact_id(version_id, observation_id)
