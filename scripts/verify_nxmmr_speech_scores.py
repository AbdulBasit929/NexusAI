# SPDX-License-Identifier: MIT
"""Independent offline recomputation; emits aggregate receipts, never transcripts."""
import json
import unicodedata
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1] / 'local-acceptance-models/nxmmr/private-benchmarks/speech-breadth-v1/fleurs'


def normalized(text):
    folded = unicodedata.normalize('NFKC', text).casefold()
    return ' '.join(''.join(' ' if unicodedata.category(c)[0] in 'PS' else c for c in folded).split())


def distance(a, b):
    # Full table independently recomputes the rolling-row production scorer.
    table = [[0] * (len(b) + 1) for _ in range(len(a) + 1)]
    for i in range(len(a) + 1):
        table[i][0] = i
    for j in range(len(b) + 1):
        table[0][j] = j
    for i, left in enumerate(a, 1):
        for j, right in enumerate(b, 1):
            table[i][j] = min(table[i-1][j] + 1, table[i][j-1] + 1, table[i-1][j-1] + (left != right))
    return table[-1][-1]


def main():
    evaluation = json.loads((ROOT / 'evaluation.json').read_text(encoding='utf8'))
    registration = json.loads((ROOT / 'evaluation-preregistration.json').read_text(encoding='utf8'))
    summaries = {}
    for result, source in zip(evaluation['results'], registration['clips'], strict=True):
        if result['reference'] != source['publisher_transcript'] or result['audio_sha256'] != source['audio_sha256']:
            raise ValueError('frozen source/oracle mismatch')
        ref, hyp = normalized(result['reference']), normalized(result['hypothesis'])
        counts = {'word_edits': distance(ref.split(), hyp.split()), 'reference_words': len(ref.split()),
                  'character_edits': distance(ref.replace(' ', ''), hyp.replace(' ', '')),
                  'reference_characters': len(ref.replace(' ', ''))}
        if any(result['score'][k] != v for k, v in counts.items()):
            raise ValueError('independent score mismatch')
        summary = summaries.setdefault(result['language'], {'clips': 0, 'cpu_seconds': 0, 'peak_rss_kib': 0,
                                                          **{k: 0 for k in counts}})
        summary['clips'] += 1
        for key, value in counts.items():
            summary[key] += value
        summary['cpu_seconds'] += result['resources']['cpu_seconds']
        summary['peak_rss_kib'] = max(summary['peak_rss_kib'], result['resources']['sampled_peak_rss_kib'])
    for lang, summary in summaries.items():
        summary['wer'] = summary['word_edits'] / summary['reference_words']
        summary['cer'] = summary['character_edits'] / summary['reference_characters']
        for key in ['wer', 'cer', 'clips']:
            if summary[key] != evaluation['summary'][lang][key]:
                raise ValueError('aggregate mismatch')
    live = json.loads((ROOT / 'live-api.json').read_text(encoding='utf-8-sig'))
    artifacts = live['details']['urdu']['derived_artifacts']
    raw = {x['metadata']['observation_id']: x for x in artifacts if x['artifact_type'] == 'forensics.audio-timestamp-segment/v1'}
    derivatives = [x for x in artifacts if x['artifact_type'] == 'forensics.audio-roman-urdu-segment/v1']
    for derivative in derivatives:
        obs = derivative['metadata']['observation']
        parent = raw[obs['parent_observation_id']]
        if obs['raw_urdu_text'] != parent['metadata']['observation']['text']:
            raise ValueError('derivative raw source mismatch')
        if any(derivative['citation_locator'][k] != parent['citation_locator'][k] for k in ['start_seconds', 'end_seconds']):
            raise ValueError('parent source time mismatch')
    print(json.dumps({'independent_recomputation': 'PASS', 'summary': summaries,
                      'retained_urdu_parents_verified': len(derivatives)}))


if __name__ == '__main__':
    main()
