# Speed of one LocalAI chat model on this laptop, with prompt-start reuse defeated by a random nonce.
#
#     python reports/laptop-speed-20260929/bench_nonce.py <model-name> [runs]
#
# Read-only: sends chat completions to LocalAI and prints prompt-reading and writing speed.
import json
import secrets
import statistics
import sys
import time
import urllib.request

URL = "http://127.0.0.1:8080/v1/chat/completions"


def call(model, prompt, max_tokens):
    body = json.dumps({"model": model, "messages": [{"role": "user", "content": prompt}],
                       "max_tokens": max_tokens, "temperature": 0}).encode()
    request = urllib.request.Request(URL, data=body, headers={"Content-Type": "application/json"})
    started = time.time()
    with urllib.request.urlopen(request, timeout=900) as response:
        usage = json.loads(response.read()).get("usage", {})
    return time.time() - started, usage


def main():
    model = sys.argv[1]
    runs = int(sys.argv[2]) if len(sys.argv) > 2 else 3
    filler = " ".join("cdr.msisdn subscriber number; cdr.call_start_ts call start time; anpr.plate_number plate;" for _ in range(60))
    call(model, "Say OK.", 4)  # load and warm the model; not timed
    reads, writes = [], []
    for _ in range(runs):
        nonce = secrets.token_hex(8)
        seconds, usage = call(model, "Run %s.\nSchema:\n%s\nQuestion: reply with the single word DONE." % (nonce, filler), 4)
        reads.append(usage.get("prompt_tokens", 0) / max(seconds, 1e-6))
        nonce = secrets.token_hex(8)
        seconds, usage = call(model, "Run %s. Write the numbers from 1 to 200 separated by commas." % nonce, 200)
        writes.append(usage.get("completion_tokens", 0) / max(seconds, 1e-6))
    print(json.dumps({"model": model, "runs": runs,
                      "prompt_read_tok_s": [round(x, 1) for x in reads], "prompt_read_median": round(statistics.median(reads), 1),
                      "write_tok_s": [round(x, 2) for x in writes], "write_median": round(statistics.median(writes), 2)}))


if __name__ == "__main__":
    main()
