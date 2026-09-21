"""Read-only freshness and frozen contract admission; never makes inference calls."""
import argparse
import hashlib
import json
import unicodedata
from collections import Counter
from pathlib import Path


def norm(s):
    return ' '.join(unicodedata.normalize('NFC', s).casefold().split())


def strings(x):
    if isinstance(x, str):
        yield x
    elif isinstance(x, dict):
        for v in x.values():
            yield from strings(v)
    elif isinstance(x, list):
        for v in x:
            yield from strings(v)


def validate(repo, corpus):
    raw = corpus.read_bytes()
    assert not raw.startswith(b'\xef\xbb\xbf'), 'BOM'
    data = json.loads(raw.decode('utf-8'))
    cases = data['cases']
    assert data['development_only'] and not data['qualification_holdout']
    assert len(cases) == 8 and all(c['model_call'] for c in cases)
    assert Counter(c['language'] for c in cases) == dict(en=2, ur=2, roman_ur=2, mixed=2)
    assert len({c['id'] for c in cases}) == len({norm(c['query']) for c in cases}) == 8
    for language in ('en', 'ur', 'roman_ur', 'mixed'):
        subset = [c for c in cases if c['language'] == language]
        assert sum('state' in c['expected'] for c in subset) == 1
    assert Counter(c['expected']['state'] for c in cases if 'state' in c['expected']) == dict(AMBIGUOUS_INTENT=2, INSUFFICIENT_FACTS=2)
    values = [v for c in cases for v in c['values']]
    assert len(values) == len(set(map(norm, values))) == 8
    assert all(v in c['query'] for c in cases for v in c['values'])
    assert data['acceptance']['expected_model_calls'] == 8 and data['acceptance']['completion_budget'] == 512
    bindings = json.loads((corpus.parent/'schema-bindings-v1.json').read_text(encoding='utf-8'))
    refs = json.loads((repo/'api/forensic_records/contracts/query-capability-references-v1.json').read_text(encoding='utf-8'))
    refs = {x['query_capability_id']: x for x in refs['capabilities']}
    for c in cases:
        binding = bindings[c['id']]
        assert len(binding['tuples']) > 1 and binding['facts']['state'] == ''
        schema = binding['schema']
        assert schema['required'] == ['decision'] and set(schema['properties']) == {'decision'}
        choices = schema['properties']['decision']['enum']
        e = c['expected']
        expected = 'CLARIFY:'+e['state'] if 'state' in e else '/'.join(e[k] for k in ('capability','semantic','scope'))
        assert expected in choices
        if 'capability' in e:
            assert e['semantic'] in refs[e['capability']]['semantics'] and e['scope'] in refs[e['capability']]['scope_modes']
    historical = set()
    hashes = {}
    excluded = []
    for folder in ('scripts','reports/nxb21','local-acceptance-models/nxb21-d'):
        for p in (repo/folder).rglob('*'):
            if not p.is_file() or p.suffix.lower() not in ('.json','.jsonl','.txt'):
                continue
            rel = p.relative_to(repo).as_posix()
            if 'nxb21d-hybrid-decision-development' in p.parts or 'hybrid-decision-development-runs' in p.parts or p.name.startswith('d-decision8'):
                continue
            if any(part in ('go-cache','go-tmp','gocache') for part in p.parts):
                continue
            if p.stat().st_size > 16*1024*1024:
                excluded.append(rel+':size_limit')
                continue
            b = p.read_bytes()
            try:
                text = b.decode('utf-8-sig')
            except UnicodeError:
                excluded.append(rel+':not_utf8')
                continue
            hashes[rel] = hashlib.sha256(b).hexdigest()
            try:
                historical.update(norm(x) for x in strings(json.loads(text)) if x)
            except ValueError:
                historical.add(norm(text))
    for name in ('d-unseen-query-corpus-v1.json','nxb21d-q4-independent-168case-holdout-v1.json','hybrid-32case-corpus-v1.json'):
        assert any(p.endswith('/'+name) for p in hashes), 'missing required history: '+name
    assert any('/hybrid-development-runs/run-20260906T105600096Z/' in p for p in hashes)
    for c in cases:
        assert not any(norm(c['query']) in old for old in historical), 'historical question overlap: '+c['id']
        for v in c['values']:
            assert not any(norm(v) in old for old in historical), 'historical value overlap: '+v
    return dict(status='PASS',cases=8,languages=dict(en=2,ur=2,roman_ur=2,mixed=2),resolved=4,clarification=4,ambiguous=2,insufficient=2,model_calls_expected=8,question_overlap=0,value_overlap=0,corpus_sha256=hashlib.sha256(raw).hexdigest(),historical_files=len(hashes),historical_hashes=hashes,excluded_files=excluded,scan_scope='All JSON/JSONL/text under scripts, reports/nxb21 and local-acceptance-models/nxb21-d, including retired hybrid and micro-probe artifacts; excludes current gate, caches and reported oversized/non-UTF8 files')


if __name__ == '__main__':
    p=argparse.ArgumentParser();p.add_argument('--repo',required=True);p.add_argument('--corpus',required=True);p.add_argument('--output')
    a=p.parse_args();result=validate(Path(a.repo).resolve(),Path(a.corpus).resolve())
    if a.output:
        Path(a.output).write_text(json.dumps(result,indent=2,ensure_ascii=False)+'\n',encoding='utf-8')
    print(json.dumps({k:v for k,v in result.items() if k not in ('historical_hashes','excluded_files')},sort_keys=True))
