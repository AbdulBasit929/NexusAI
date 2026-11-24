#!/usr/bin/env python3
"""Serves a folder read-only on 127.0.0.1 with permissive CORS, so that a browser test can fetch a reference image and put it into the question box (the browser tool cannot open a file dialog).
Local only; stop it when the test is done.

    python tools/media_data/serve_pack.py C:/NexusAI-Showcase/orion/image-search-test-pack 8765
"""
import functools
import http.server
import sys


class Handler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header("Access-Control-Allow-Origin", "*")
        super().end_headers()

    def log_message(self, *args):
        pass


if __name__ == "__main__":
    folder, port = sys.argv[1], int(sys.argv[2])
    handler = functools.partial(Handler, directory=folder)
    http.server.ThreadingHTTPServer(("127.0.0.1", port), handler).serve_forever()
