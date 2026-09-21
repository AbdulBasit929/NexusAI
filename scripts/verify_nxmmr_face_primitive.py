# SPDX-License-Identifier: MIT
"""Two existing synthetic images; installed face detector only, no retention."""
import base64
import json
import sys
import time
from pathlib import Path

REPO = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(REPO))
from ingestion.forensic_records.media_pipeline import LocalAIFaceProcessor
from scripts.nxmmr_speech_breadth import private_root, save_new, sha


def main():
    root = private_root()
    if (root / 'face-primitive.json').exists():
        raise ValueError('immutable receipt already exists')
    manifest = json.loads((REPO / 'local-acceptance-inputs/NEXUSAI_MULTIMODAL_PRODUCT_ACCEPTANCE_MANIFEST.json').read_text(encoding='utf-8-sig'))
    sources = {x['source_id']: x for x in manifest['sources']}
    processor = LocalAIFaceProcessor('http://127.0.0.1:8080/v1/face', 'face-detect-yunet-sface', None)
    results = []
    for identity, expected in [('face-subject-a-frontal', 1), ('face-no-face-street', 0)]:
        source = sources[identity]
        data = (REPO / source['path']).read_bytes()
        if sha(data) != source['sha256']:
            raise ValueError('synthetic fixture hash mismatch')
        started = time.perf_counter()
        output = processor._post_json('detect', {'model': processor.model, 'img': 'data:image/png;base64,' + base64.b64encode(data).decode('ascii')})
        count = len(output['faces'])
        results.append({'fixture': identity, 'sha256': source['sha256'], 'expected_faces': expected, 'actual_faces': count,
                        'wall_seconds': time.perf_counter() - started, 'passed': count == expected})
    save_new(root / 'face-primitive.json', {'results': results, 'retained_mutation': False, 'identity_claim': False,
                                          'scope': 'direct installed detector only; worker remains disabled'})
    print(json.dumps(results))


if __name__ == '__main__':
    main()
