# SPDX-License-Identifier: MIT
"""Read-only, sampled resource receipt for the already-running ASR process."""
import json
import subprocess


# Runs inside the existing Linux container; never installs or writes anything.
MONITOR = r'''
import json, os, pathlib, select, sys, time
matches = []
for path in pathlib.Path('/proc').glob('[0-9]*/cmdline'):
    try:
        if b'/backends/cpu-faster-whisper/backend.py' in path.read_bytes().split(b'\0'):
            matches.append(path.parent)
    except (FileNotFoundError, PermissionError):
        pass
if len(matches) != 1:
    raise RuntimeError('expected exactly one existing ASR backend')
proc = matches[0]
def snapshot():
    fields = (proc / 'stat').read_text().rsplit(')', 1)[1].split()
    status = dict(line.split(':', 1) for line in (proc / 'status').read_text().splitlines())
    return ((int(fields[11]) + int(fields[12])) / os.sysconf('SC_CLK_TCK'),
            int(status['VmRSS'].split()[0]), int(status['VmHWM'].split()[0]))
cpu0, rss0, hwm0 = snapshot()
started = time.monotonic()
peak, samples = rss0, 1
print(json.dumps({'ready': True, 'pid': int(proc.name)}), flush=True)
while not select.select([sys.stdin], [], [], 0.05)[0]:
    cpu, rss, hwm = snapshot()
    peak, samples = max(peak, rss), samples + 1
sys.stdin.readline()
cpu, rss, hwm = snapshot()
elapsed = time.monotonic() - started
print(json.dumps({'backend_pid': int(proc.name), 'cpu_seconds': cpu - cpu0,
    'elapsed_seconds': elapsed, 'cpu_percent_one_core': (cpu - cpu0) / elapsed * 100,
    'sampled_peak_rss_kib': max(peak, rss), 'rss_before_kib': rss0, 'rss_after_kib': rss,
    'lifetime_hwm_before_kib': hwm0, 'lifetime_hwm_after_kib': hwm,
    'samples': samples + 1, 'sample_interval_seconds': 0.05,
    'scope': 'existing ASR process only; concurrent requests could contribute'}), flush=True)
'''


class ASRResources:
    def __enter__(self):
        self.process = subprocess.Popen([
            'docker', 'exec', '-i', 'nexusai-api-1',
            '/backends/cpu-faster-whisper/venv/bin/python', '-c', MONITOR,
        ], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        line = self.process.stdout.readline()
        if not line or not json.loads(line).get('ready'):
            self.process.communicate(timeout=10)
            raise RuntimeError('ASR resource monitor unavailable; inference not started')
        return self

    def __exit__(self, *exception):
        stdout, _ = self.process.communicate('\n', timeout=15)
        if self.process.returncode != 0:
            raise RuntimeError('ASR resource monitor failed')
        self.receipt = json.loads(stdout)
