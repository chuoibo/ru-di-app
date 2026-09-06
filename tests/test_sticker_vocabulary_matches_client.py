"""The sticker vocabulary is one list, written down three times (ADR-0021 §2.1).

The server owns the ids (`app/domain/stickers.py`) and refuses a message whose
body is not one of them. `packages/shared/stickers.json` is the copy both apps
may read without a session. The client draws each id as a vector shape in
`src/rudi/chat/sticker.ts`, and an id with no shape is a «?» tile.

Each copy is internally consistent; only a reader of all three sees the day
they drift: a new id on the server alone is a sticker nobody can send, a new
shape on the client alone is a 422 on the write, an id in the JSON alone is a
row neither side understands. Read as text, imported nowhere (the client half
is TypeScript, the server half needs `services/api` on the path).
"""

from __future__ import annotations

import ast
import json
import re
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
SERVER = REPO_ROOT / "services" / "api" / "app" / "domain" / "stickers.py"
SHARED = REPO_ROOT / "packages" / "shared" / "stickers.json"
CLIENT = REPO_ROOT / "apps" / "mobile" / "src" / "rudi" / "chat" / "sticker.ts"

SLUG = re.compile(r"^[a-z0-9-]{1,32}$")


def _server_ids() -> list[str]:
    tree = ast.parse(SERVER.read_text(encoding="utf-8"))
    for node in tree.body:
        if not isinstance(node, ast.AnnAssign | ast.Assign):
            continue
        targets = [node.target] if isinstance(node, ast.AnnAssign) else node.targets
        names = [t.id for t in targets if isinstance(t, ast.Name)]
        if names[:1] != ["STICKERS"]:
            continue
        assert isinstance(node.value, ast.Tuple), "STICKERS phải là tuple hằng"
        ids = []
        for call in node.value.elts:
            assert isinstance(call, ast.Call) and call.args, "phần tử lạ trong STICKERS"
            first = call.args[0]
            assert isinstance(first, ast.Constant), "id phải là literal"
            ids.append(first.value)
        return ids
    raise AssertionError(f"không thấy STICKERS trong {SERVER}")


def _shared_ids() -> list[str]:
    data = json.loads(SHARED.read_text(encoding="utf-8"))
    return [row["id"] for row in data["stickers"]]


def _client_ids() -> list[str]:
    """Ids inside `export const STICKER_IDS = [ ... ] as const;`."""
    text = CLIENT.read_text(encoding="utf-8")
    start = text.index("export const STICKER_IDS")
    end = text.index("] as const", start)
    return re.findall(r'"([^"]+)"', text[start:end])


def _client_shape_keys() -> set[str]:
    """Keys of the shape table: every id the client can actually draw."""
    text = CLIENT.read_text(encoding="utf-8")
    start = text.index("const HINH")
    end = text.index("};", start)
    return set(re.findall(r'^\s*"([a-z0-9-]+)":\s*\[', text[start:end], re.M))


def test_the_three_copies_name_the_same_ids_in_the_same_order() -> None:
    server, shared, client = _server_ids(), _shared_ids(), _client_ids()
    assert server == shared == client, (
        "từ vựng sticker lệch nhau.\n"
        f"  máy chủ ({SERVER.name}): {server}\n"
        f"  chia sẻ ({SHARED.name}): {shared}\n"
        f"  máy khách ({CLIENT.name}): {client}"
    )


def test_every_id_has_a_shape_the_client_can_draw() -> None:
    missing = set(_shared_ids()) - _client_shape_keys()
    assert not missing, f"id không có hình vector trên máy khách: {sorted(missing)}"


def test_ids_are_ascii_slugs_and_never_look_like_a_number() -> None:
    for sticker_id in _shared_ids():
        assert SLUG.match(sticker_id), sticker_id
        assert not re.search(r"\d{4,}", sticker_id), (
            f"{sticker_id}: một dãy số dài là thứ repo guard đọc thành số tài khoản"
        )


def test_labels_exist_and_avoid_the_em_dash() -> None:
    data = json.loads(SHARED.read_text(encoding="utf-8"))
    for row in data["stickers"]:
        assert row["label"].strip(), row["id"]
        assert "—" not in row["label"], row["id"]
