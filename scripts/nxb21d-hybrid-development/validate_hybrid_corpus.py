"""Read-only structural/oracle/freshness validation; never dispatch inference."""
import argparse
import hashlib
import json
import unicodedata
from collections import Counter
from pathlib import Path


def texts(value):
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for v in value.values():
            yield from texts(v)
    elif isinstance(value, list):
        for v in value:
            yield from texts(v)


def norm(s):
    return ' '.join(unicodedata.normalize('NFC', s).casefold().split())


def validate(repo, path):
    raw = path.read_bytes()
    assert not raw.startswith(b'\xef\xbb\xbf'), 'BOM'
    c = json.loads(raw.decode('utf-8'))
    assert c['development_only'] and not c['qualification_holdout']
    cases = c['cases']
    assert len(cases) == 32
    assert Counter(x['language'] for x in cases) == Counter(dict(en=8, ur=8, roman_ur=8, mixed=8))
    assert len(set(x['id'] for x in cases)) == 32
    assert len(set(norm(x['query']) for x in cases)) == 32
    assert sum(x['model_call'] for x in cases) == c['acceptance']['expected_model_calls'] == 6
    assert c['acceptance']['completion_budget'] == 512
    refs = json.loads((repo/'api/forensic_records/contracts/query-capability-references-v1.json').read_text(encoding='utf-8'))
    refs = {r['query_capability_id']: r for r in refs['capabilities']}
    for x in cases:
        e = x['expected']
        if 'capability' in e:
            r = refs[e['capability']]
            assert e['semantic'] in r['semantics'], x['id']
            assert e['scope'] in r['scope_modes'], x['id']
        assert 'no_stale_family' not in e
        for field in ('target', 'literal'):
            if e.get(field):
                assert e[field] in x['query'], (x['id'], field)
    historical = set()
    hashes = {}
    for directory in ('scripts', 'reports/nxb21', 'local-acceptance-models/nxb21-d'):
        for p in (repo/directory).rglob('*.json'):
            if p.resolve() == path.resolve() or 'nxb21d-hybrid-development' in p.parts or 'hybrid-development-runs' in p.parts:
                continue
            if not any(s in p.name.lower() for s in ('corpus', 'holdout', 'request', 'case-result', '12case')):
                continue
            if p.stat().st_size > 8*1024*1024:
                continue
            b = p.read_bytes()
            try:
                data = json.loads(b.decode('utf-8-sig'))
            except (ValueError, UnicodeDecodeError):
                continue
            hashes[str(p.relative_to(repo)).replace('\\', '/')] = hashlib.sha256(b).hexdigest()
            historical.update(norm(t) for t in texts(data) if t)
    # Known consumed/retired corpora must actually participate in the proof.
    for name in ('d-unseen-query-corpus-v1.json', 'nxb21d-q4-independent-168case-holdout-v1.json', 'nxb21d-q4-production-interface-24case-development-corpus-v1.json', 'nxb21d-q4-production-interface-24case-development-corpus-v2.json'):
        assert any(p.endswith('/'+name) for p in hashes), ('missing history', name)
    values = []
    for x in cases:
        assert norm(x['query']) not in historical, ('question overlap', x['id'])
        case_values = set()
        for key in ('target', 'literal'):
            if x['expected'].get(key):
                case_values.add(x['expected'][key])
        if x['input'].get('prior_target'):
            case_values.add(x['input']['prior_target'])
        values.extend(case_values)
    assert len(values) == len(set(values)), 'duplicate exact values'
    for value in values:
        assert not any(norm(value) in old for old in historical), ('historical value overlap', value)
    return dict(status='PASS',cases=32,languages=dict(Counter(x['language'] for x in cases)),model_cases=6,deterministic_cases=26,corpus_sha256=hashlib.sha256(raw).hexdigest(),historical_files=len(hashes),historical_hashes=hashes,question_overlap=0,value_overlap=0,development_only=True)


if __name__ == '__main__':
    p=argparse.ArgumentParser()
    p.add_argument('--repo',required=True)
    p.add_argument('--corpus',required=True)
    p.add_argument('--output')
    a=p.parse_args()
    result=validate(Path(a.repo).resolve(),Path(a.corpus).resolve())
    if a.output:
        Path(a.output).write_text(json.dumps(result,indent=2,ensure_ascii=False)+'\n',encoding='utf-8')
    print(json.dumps({k:v for k,v in result.items() if k!='historical_hashes'},sort_keys=True))
