# SPDX-License-Identifier: MIT
"""Persist evaluator resource evidence without masking a processing exception."""
from contextlib import contextmanager
import json
import time


@contextmanager
def finalized_resources(path, monitor, *, phase, started, cpu_started):
    failure = None
    try:
        yield
    except BaseException as exc:
        failure = exc
        raise
    finally:
        try:
            value = {
                "phase": phase, "state": "FAIL" if failure else "PASS",
                "exit_state": "exception" if failure else "completed",
                "error": None if failure is None else {
                    "type": type(failure).__name__, "message": str(failure)},
                "wall_seconds": time.perf_counter() - started,
                "cpu_seconds": time.process_time() - cpu_started,
                "resources": monitor.finish(),
            }
            with path.open("x", encoding="utf-8") as stream:
                json.dump(value, stream, indent=2)
                stream.write("\n")
        except BaseException as recording_error:
            if failure is None:
                raise
            failure.add_note(f"Resource finalization also failed: {recording_error}")
