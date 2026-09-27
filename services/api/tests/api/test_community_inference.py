"""Synthetic-only checks of the private inference boundary, not model accuracy."""

import json
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest

from app.api.community_inference import infer_community


@pytest.mark.parametrize(
    "url",
    [
        "",
        "https://example.com",
        "http://8.8.8.8",
        "http://0.0.0.0",
        "http://u:p@127.0.0.1",
        "http://127.0.0.1#secret",
        "file:///tmp/model",
    ],
)
def test_community_refuses_unconfigured_or_external_inference(monkeypatch, url):
    monkeypatch.setenv("COMMUNITY_INFERENCE_URL", url)
    with pytest.raises(RuntimeError):
        infer_community("moderate", {"body": "Synthetic walk"})


def test_community_forwards_only_explicit_bundle_and_refuses_redirect(monkeypatch):
    seen = []

    class Server(BaseHTTPRequestHandler):
        def log_message(self, *_args):
            pass

        def do_POST(self):
            seen.append(
                json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            )
            if self.path == "/nep":
                self.send_response(302)
                self.send_header("Location", "https://example.com")
                self.end_headers()
                return
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b'{"safe":true,"relevant":true,"confidence_milli":950}')

    server = ThreadingHTTPServer(("127.0.0.1", 0), Server)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    monkeypatch.setenv(
        "COMMUNITY_INFERENCE_URL", f"http://127.0.0.1:{server.server_port}"
    )
    try:
        bundle = {"body": "Synthetic walk", "media": [], "comment": False}
        assert infer_community("moderate", bundle)["safe"] is True
        assert seen == [bundle]
        with pytest.raises(RuntimeError, match="inference_redirect_refused"):
            infer_community(
                "nep", {"excerpt": "Synthetic excerpt", "request": "Short note"}
            )
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)
