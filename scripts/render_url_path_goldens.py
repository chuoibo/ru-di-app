#!/usr/bin/env python3
"""Render Starlette's `request.url.path` as goldens for the Go front door.

`api_problem_handler` (services/api/app/api/main.py) answers the guest
broken-link page only when `is_guest_path(request.url.path)`. That string is
not `scope["path"]`: Starlette rebuilds a URL from the Host header, the scope
path and the query string, then reads its path back through
`urllib.parse.urlsplit`, which can also raise. The Go router's URLPath must
give the same path, or fail exactly where this does.

Each case builds the scope uvicorn would and asks the pinned libraries, so
nothing about URL parsing is reimplemented here. Run it inside the API image:

    docker run --rm -v "$PWD":/repo:ro -w /srv --network none --entrypoint python \\
      mobile-parity-api:7bf58e3d /repo/scripts/render_url_path_goldens.py \\
      > services/core/internal/httpapi/router/testdata/starlette_url_paths.json
"""

from __future__ import annotations

import json
import sys
import unicodedata

from starlette.requests import Request

TOKEN = "t" * 43
PATHS = [
    f"/g/{TOKEN}",
    f"/g/{TOKEN}/khong-phai-toi",
    f"/g/{TOKEN}/doi-so-tien",
    "/contexts/x",
]
QUERIES = [b"", b"obligation_id=abc", b"a=1&b=%2F", b"x=/g/y", b"x=?#"]

HOSTS = [
    "",
    "parity.test",
    "127.0.0.1:8000",
    "x/y",
    "x?y",
    "x#y",
    "[",
    "]",
    "[]",
    "][",
    "[::1]",
    "[::1]:8000",
    "[::1]x",
    "[::1]:x",
    "x[::1]",
    "a@[::1]",
    "[a@b]",
    "[::1%25eth0]",
    "[::1%eth0]",
    "[::1%]",
    "[::1%a%b]",
    "[1.2.3.4]",
    "[::1.2.3.4]",
    "[::ffff:1.2.3.4]",
    "[::01.2.3.4]",
    "[::256.2.3.4]",
    "[1:2:3:4:5:6:7:8]",
    "[1:2:3:4:5:6:7:8:9]",
    "[1:2:3:4:5:6:7::]",
    "[::2:3:4:5:6:7:8]",
    "[1::2::3]",
    "[:1:2:3:4:5:6:7]",
    "[1:2:3:4:5:6:7:]",
    "[12345::]",
    "[g::]",
    "[::]",
    "[:]",
    "[::1/64]",
    "[v1.x]",
    "[v1.]",
    "[vg.x]",
    "[V1.x]",
    "[v.x]",
    "[" + "1:" * 7 + "1" + "f" * 40 + "]",
    "[1:2:3:4:5:6:1.2.3.4]",
    "[1:2:3:4:5:6:7:1.2.3.4]",
    "[::1.2.3]",
    "[:::]",
    "[1:2:3:4:5:6:7:8:9:10:11]",
    "a]",
    "a[",
    "u@v:w",
    "\t[::1]",
    "a\tb",
    "[::\n1]",
    " x",
    "%5B",
    "!$&'()*+,;=-._~",
]


def url_path(host: str | None, path: str, query: bytes) -> dict:
    headers = [] if host is None else [(b"host", host.encode("latin-1"))]
    scope = {
        "type": "http",
        "scheme": "http",
        "server": ("127.0.0.1", 8000),
        "path": path,
        "query_string": query,
        "headers": headers,
    }
    try:
        return {"url_path": Request(scope).url.path}
    except Exception as exc:  # noqa: BLE001 - the class is the golden
        return {"raises": type(exc).__name__}


def main() -> None:
    # Every latin-1 character alone: urlsplit's NFKC netloc check never fires
    # on one, which the Go port relies on.
    for code in range(0x80, 0x100):
        n = unicodedata.normalize("NFKC", chr(code))
        if any(c in n for c in "/?#@:"):
            sys.exit(f"U+{code:04X} normalises to a netloc delimiter")

    hosts = list(HOSTS)
    hosts += [chr(code) for code in range(256)]
    hosts += ["[" + chr(code) + "]" for code in range(256)]
    hosts += ["[::" + chr(code) + "]" for code in range(256)]
    cases = []
    for host in hosts:
        for path in PATHS[:2]:
            for query in QUERIES[:2]:
                cases.append((host, path, query))
    for host in HOSTS[:12]:
        for path in PATHS:
            for query in QUERIES:
                cases.append((host, path, query))
    for path in PATHS:
        cases.append((None, path, b""))

    out = []
    for host, path, query in cases:
        row = {
            "host": host,
            "path": path,
            "query": query.decode("latin-1"),
        }
        row.update(url_path(host, path, query))
        out.append(row)
    json.dump(out, sys.stdout, ensure_ascii=False, indent=0)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
