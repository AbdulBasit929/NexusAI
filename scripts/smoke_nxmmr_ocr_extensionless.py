#!/usr/bin/env python3
"""Network-disabled functional smoke with newly generated pixels, never evidence."""
import argparse
import dataclasses
import hashlib
import json
import os
from pathlib import Path
import resource
import sys
import tempfile
import time


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--manifest', required=True)
    parser.add_argument('--receipt', required=True)
    args = parser.parse_args()
    manifest = json.loads(Path(args.manifest).read_text())
    for name, value in manifest['worker_environment'].items():
        if name.startswith('FORENSIC_PADDLE_'):
            os.environ[name] = value
    os.environ.update(HF_HUB_OFFLINE='1', TRANSFORMERS_OFFLINE='1', PADDLE_PDX_DISABLE_MODEL_SOURCE_CHECK='True')
    sys.path.insert(0, '/app')
    started = time.monotonic()
    receipt = {'state': 'FAIL', 'retained_evidence_accessed': False,
               'benchmark_scoring': False, 'model_downloads': False, 'cases': []}
    try:
        import cv2
        import numpy as np
        from multilingual_ocr import PaddleMultilingualOCRProcessor, PROCESSOR_REVISION
        processor = PaddleMultilingualOCRProcessor.from_environment()
        receipt['processor_revision'] = PROCESSOR_REVISION
        with tempfile.TemporaryDirectory() as directory:
            for label in ('blank', 'printed-control'):
                frame = np.full((240, 640, 3), 255, dtype=np.uint8)
                if label == 'printed-control':
                    cv2.putText(frame, 'INPUT CONTRACT 123', (20, 120), cv2.FONT_HERSHEY_SIMPLEX, 1.4, (0, 0, 0), 3)
                named = Path(directory) / (label + '.png')
                assert cv2.imwrite(str(named), frame)
                content = named.read_bytes()
                digest = hashlib.sha256(content).hexdigest()
                extensionless = Path(directory) / digest
                extensionless.write_bytes(content)
                context = dict(evidence_id='synthetic', version_id='synthetic', source_sha256=digest, source_file=label + '.png')
                named_result = processor.process_image(named, **context)
                spool_result = processor.process_image(extensionless, **context)
                assert dataclasses.asdict(named_result) == dataclasses.asdict(spool_result), 'Suffix-dependent output'
                assert hashlib.sha256(extensionless.read_bytes()).hexdigest() == digest
                if label == 'blank':
                    assert not spool_result.observations, 'Blank control must complete without observations'
                else:
                    assert spool_result.metadata['detected_region_count'] > 0, 'Recognition path was not exercised'
                receipt['cases'].append({'case': label, 'suffix_parity': True, 'source_unchanged': True,
                                         'result_state': spool_result.metadata['result_state'],
                                         'regions': spool_result.metadata['detected_region_count'],
                                         'observations': len(spool_result.observations)})
        receipt['state'] = 'PASS'
    except Exception as error:
        receipt['error'] = f'{type(error).__name__}: {error}'
        raise
    finally:
        receipt['wall_seconds'] = round(time.monotonic() - started, 3)
        receipt['peak_rss_mib'] = round(resource.getrusage(resource.RUSAGE_SELF).ru_maxrss / 1024, 3)
        Path(args.receipt).write_text(json.dumps(receipt, indent=2) + '\n')
        print(json.dumps(receipt, indent=2), flush=True)


if __name__ == '__main__':
    main()
