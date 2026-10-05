# PROMPT-CACHE PROBE for the front door: does the fixed start of its prompt get reused between messages?
# Sends six short greetings back to back and prints the front-door time of each (from the X-Front-Door header).
# If calls 2 to 6 take a few seconds and call 1 takes much longer, the start of the prompt is being reused. If every call is slow, it is not.
#
#     python cache_probe.py
import re
import sys
import os

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import conversation_baseline as cb  # noqa: E402

MESSAGES = ["Hello", "Hi there", "Good afternoon", "Thanks a lot", "Hey", "Goodbye"]
times = []
for message in MESSAGES:
    status, blob, seconds, header = cb.ask(message)
    m = re.search(r"ms=(\d+)", header or "")
    ms = int(m.group(1)) / 1000.0 if m else None
    times.append(ms)
    print("%-16s http=%s wall=%5.1fs front-door=%s  %s" % (message, status, seconds, ("%.1fs" % ms) if ms else "-", header or ""))
warm = [t for t in times[1:] if t]
if warm and times[0]:
    print("first %.1fs, then median %.1fs (%s)" % (times[0], sorted(warm)[len(warm) // 2], "prefix reuse is working" if sorted(warm)[len(warm) // 2] < times[0] * 0.6 else "no clear sign of prefix reuse"))
