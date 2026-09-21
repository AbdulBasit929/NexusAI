"""Adjudicate immutable v2 observations and seal the single final admission correction."""
import argparse
import hashlib
import json
import math
import statistics
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
RUN = ROOT / 'local-acceptance-models/nxb21-current4b-runtime-characterization-v2/run-20260910T050248809Z'
CONFIG = HERE / 'current4b-final-characterization-freeze.json'


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def write(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n', encoding='utf-8')


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--prepare', action='store_true')
    args = parser.parse_args()
    receipt_path = RUN / 'resource-lifecycle-receipt-v1.json'
    assert sha(receipt_path) == Path(str(receipt_path) + '.sha256').read_text().strip()
    receipt = json.loads(receipt_path.read_text(encoding='utf-8-sig'))
    rows = [json.loads(x) for x in (RUN / 'resource-lifecycle-observations.ndjson').read_text().splitlines()]
    samples = [x for x in rows if 'committed_bytes' in x]
    calls = receipt['completed_calls']
    assert len(calls) == receipt['request_count'] == 5
    assert receipt['runtime_before'] == receipt['runtime_after']
    assert receipt['runtime_integrity'] == receipt['unload_state'] == 'PASS'
    assert receipt['error'] == 'RAM_STABLE_WINDOW_FAILED:pre_reload required_gib=8.627'
    start = samples[0]
    first_after = next(x for x in samples if x['stage'] == 'model_load_after')
    steady = samples[samples.index(first_after):]
    last = steady[-1]
    peak_delta = round(start['available_ram_gib'] - min(x['available_ram_gib'] for x in samples), 3)
    noise = max(x['available_ram_gib'] for x in steady) - min(x['available_ram_gib'] for x in steady)
    margin = math.ceil(3 * noise * 4) / 4
    # An engineering reserve, not an asserted measured failure boundary:
    # keep 2 GiB above the unchanged 1.5 GiB emergency abort.
    reserve = 1.5 + 2.0
    threshold = math.ceil((peak_delta + reserve + margin) * 10) / 10
    commit_delta = (max(x['committed_bytes'] for x in samples) - start['committed_bytes']) / 2**30
    commit_admission = math.ceil((commit_delta + 2.0 + margin) * 10) / 10
    assert (peak_delta, margin, threshold, commit_admission) == (4.677, 0.25, 8.5, 6.9)
    envelope = {
        'contract_version': 'nexusai.current4b-model-resource-envelope/v1',
        'state': 'CANDIDATE_FOR_SINGLE_FINAL_CHARACTERIZATION_NOT_YET_RUNTIME_QUALIFIED',
        'measured_peak_delta_gib': peak_delta,
        'measured_load_to_after_delta_gib': round(start['available_ram_gib']-first_after['available_ram_gib'], 3),
        'measured_steady_noise_gib': round(noise, 3),
        'selected_reserve_gib': reserve,
        'reserve_basis': 'Explicit engineering cushion 2 GiB above unchanged 1.5 GiB emergency guard; not a measured minimum',
        'load_margin_gib': margin,
        'margin_basis': 'Three times observed post-load physical range, rounded upward to next quarter GiB',
        'reload_admission_gib': threshold,
        'reload_commit_reserve_gib': commit_admission,
        'commit_basis': 'Measured peak commit delta plus existing 2 GiB abort reserve plus load margin, rounded upward to 0.1 GiB',
        'quiet_paging_pages_per_second': max(x['pages_input_per_second']+x['pages_output_per_second'] for x in samples),
        'unloaded_backend_rss_mib': start['backend_rss_total_mib'],
        'required_samples': 5,
        'max_first_last_decline_gib': margin,
        'maximum_wait_seconds': 120,
        'cold_admission_gib': 8.627,
        'build_recreate_floor_gib': 6,
        'operating_envelope_adopted': False,
    }
    metrics = {}
    for key in ['available_ram_gib','committed_bytes','commit_limit_bytes','commit_reserve_gib','pages_input_per_second','pages_output_per_second','wsl_swap_used_kib','vmmemwsl_working_set_mib','api_container_memory_mib','backend_rss_total_mib','compression_working_set_mib']:
        values = [s[key] for s in samples if s.get(key) is not None]
        metrics[key] = {'first': values[0], 'last': values[-1], 'min': min(values), 'max': max(values)} if values else None
    services = {}
    for name in receipt['runtime_before']['containers']:
        stats = [stat for s in samples for stat in s['service_stats'] if stat['Name'] == name]
        services[name] = {'first_memory': stats[0]['MemUsage'], 'last_memory': stats[-1]['MemUsage'], 'peak_cpu_percent': max(float(s['CPUPerc'].rstrip('%')) for s in stats)}
    summary = {
        'source_receipt': receipt_path.relative_to(ROOT).as_posix(), 'source_receipt_sha256': sha(receipt_path),
        'source_ndjson_sha256': sha(RUN/'resource-lifecycle-observations.ndjson'),
        'classification': 'INCOMPLETE_PRE_RELOAD_ADMISSION', 'first_interval': 'SAFE_AND_STABLE_FOR_TESTED_TINY_WORKLOAD',
        'model_decision': 'PENDING_SINGLE_FINAL_CHARACTERIZATION', 'semantic_quality_evidence': 'NONE',
        'metrics': metrics, 'service_stats': services,
        'tiny_latency_p50_ms': statistics.median(c['latency_ms'] for c in calls if c['stage']=='tiny'),
        'tiny_latency_max_ms': max(c['latency_ms'] for c in calls if c['stage']=='tiny'),
        'latency_caveat': 'Runner-observed latency includes telemetry polling overhead; not isolated HTTP completion latency',
        'postload_to_last_ram_drift_gib': round(last['available_ram_gib']-first_after['available_ram_gib'],3),
        'four_call_batch_ram_drift_gib': receipt['batch_results'][0]['drift_gib'],
        'postload_backend_rss_drift_mib': round(last['backend_rss_total_mib']-first_after['backend_rss_total_mib'],3),
        'postload_commit_reserve_drift_gib': round(last['commit_reserve_gib']-first_after['commit_reserve_gib'],3),
        'pagefile_used_mib': [sum(p['used_mib'] for p in s['pagefile']) for s in samples],
        'unload_final_available_gib': [x['available_ram_gib'] for x in rows if x['stage']=='pre_reload'][-1],
        'unload_final_recovery_gib': round([x['available_ram_gib'] for x in rows if x['stage']=='pre_reload'][-1]-last['available_ram_gib'],3),
        'unload_caveat': 'Physical-only recovery samples; v2 did not collect commit/swap/pagefile after unload; do not impute them',
        'calls': [{'stage':c['stage'],'latency_ms':c['latency_ms'],'tokens':c['token_usage']} for c in calls],
        'envelope': envelope,
    }
    if args.prepare:
        if CONFIG.exists():
            raise SystemExit('Final freeze already exists; never overwrite or repeat the final attempt')
        config = json.loads((HERE/'current4b-runtime-characterization-freeze-v2.json').read_text())
        config['soak_id'] = 'nxb21-current4b-final-characterization-20260910'
        config['contract_version'] = 'nexusai.current4b-final-characterization-freeze/v1'
        config['runner'] = 'scripts/nxb21-english-functional-qualification/run_current4b_final_characterization.ps1'
        config['runner_sha256'] = sha(ROOT/config['runner'])
        config['reload_envelope'] = envelope
        for path in list(RUN.iterdir()) + [HERE/'run_current4b_runtime_characterization_v2.ps1', HERE/'current4b-runtime-characterization-freeze-v2.json', HERE/'test_final_characterization.ps1', Path(__file__).resolve()]:
            config['files'][path.relative_to(ROOT).as_posix()] = sha(path)
        write(CONFIG, config)
        Path(str(CONFIG)+'.sha256').write_text(sha(CONFIG)+'\n')
        write(ROOT/'reports/nxb21/current4b-v2-runtime-adjudication.json', summary)
    config = json.loads(CONFIG.read_text())
    assert sha(CONFIG) == Path(str(CONFIG)+'.sha256').read_text().strip()
    assert config['reload_envelope'] == envelope
    assert sha(ROOT/config['runner']) == config['runner_sha256']
    for path, expected in config['files'].items():
        assert sha(ROOT/path) == expected, path
    old = (HERE/'run_current4b_runtime_characterization_v2.ps1').read_text()
    new = (ROOT/config['runner']).read_text()
    for name in ['New-GenericBody','Invoke-ResourceCall','Assert-ExperimentalSafety','Invoke-UnloadOwnedModel','Assert-ModelIdentity']:
        def function(text):
            return text.split('function '+name,1)[1].split('\nfunction ',1)[0]
        assert function(old) == function(new), name
    print(json.dumps({'state':'PASS','reload_gib':threshold,'runner_sha256':config['runner_sha256'],'freeze_sha256':sha(CONFIG),'live_inference':False}))


if __name__ == '__main__':
    main()
