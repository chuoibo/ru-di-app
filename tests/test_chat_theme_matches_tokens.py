"""The five chat theme slugs are one list, written down three times (ADR-0021 §2.4).

`app/domain/chat_theme.py` is what the server accepts; `chatTheme` in
`packages/shared/tokens.json` is where the colours live; `THEME_CHAT` in the
client's `src/rudi/mau-chat.ts` is what the settings sheet offers. The colour
arithmetic (both schemes present, ink on bubble ≥ 4.5:1) is checked next to the
other token tests in `services/api/tests/web/test_chat_theme_tokens.py`; this
file only refuses drift between the three name lists, read as text.
"""

from __future__ import annotations

import ast
import json
import re
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
SERVER = REPO_ROOT / "services" / "api" / "app" / "domain" / "chat_theme.py"
TOKENS = REPO_ROOT / "packages" / "shared" / "tokens.json"
CLIENT = REPO_ROOT / "apps" / "mobile" / "src" / "rudi" / "mau-chat.ts"


def _server_slugs() -> list[str]:
    tree = ast.parse(SERVER.read_text(encoding="utf-8"))
    for node in tree.body:
        if not isinstance(node, ast.AnnAssign | ast.Assign):
            continue
        targets = [node.target] if isinstance(node, ast.AnnAssign) else node.targets
        names = [t.id for t in targets if isinstance(t, ast.Name)]
        if names[:1] != ["THEMES"]:
            continue
        assert isinstance(node.value, ast.Tuple)
        return [elt.value for elt in node.value.elts if isinstance(elt, ast.Constant)]
    raise AssertionError(f"không thấy THEMES trong {SERVER}")


def _token_slugs() -> list[str]:
    return list(json.loads(TOKENS.read_text(encoding="utf-8"))["chatTheme"].keys())


def _client_slugs() -> list[str]:
    text = CLIENT.read_text(encoding="utf-8")
    start = text.index("export const THEME_CHAT")
    end = text.index("] as const", start)
    return re.findall(r'"([^"]+)"', text[start:end])


def test_the_three_copies_name_the_same_slugs_in_the_same_order() -> None:
    server, tokens, client = _server_slugs(), _token_slugs(), _client_slugs()
    assert server == tokens == client, (
        "danh sách theme lệch nhau.\n"
        f"  máy chủ ({SERVER.name}): {server}\n"
        f"  tokens ({TOKENS.name}): {tokens}\n"
        f"  máy khách ({CLIENT.name}): {client}"
    )


def test_the_client_never_spells_a_theme_colour_itself() -> None:
    """Colours come from tokens.json; a hex literal in the client copy would be
    a fourth spelling nobody compares (and `rudi-khong-hex` refuses it too)."""
    text = CLIENT.read_text(encoding="utf-8")
    assert not re.search(r"#[0-9a-fA-F]{3,8}\b", text), (
        "mau-chat.ts không được viết hex"
    )
