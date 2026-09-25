"""The AI flows wait only for words the app still draws.

Slice 7 (ADR-0039, proposed) deleted the AI tray's sentence «Chỉ lời nhờ trong
ô này được gửi cho AI. Lịch sử chat không được chia sẻ.», and flow 30's AI=0
branch went on waiting 60 s for it: a core chat flow turned red, and nothing
short of an emulator run could say so. This reads the AI flows and the app's
source and asks, for every word a flow waits for or taps, whether the source
still contains it. Seconds, no device.

How a pattern is read: the flow's own typed text (every ``inputText`` in the
Maestro tree) is the person's words, not the app's, and is stripped; regex
pieces (``[0-9]+``, ``.*`` and ``(a|b)`` groups, each option checked on its
own) split a pattern into static fragments; every fragment must be a substring
of apps/mobile/src. The caller's name «Bạn» is a word the app draws through
`tenNguoi`, so it splits fragments too and is checked by itself.

What this does NOT prove: that the words are on the screen the flow is on, or
that a Maestro regex fully matches the element. Substring presence is a
liveness check, not a render; flow 30 and flow 49 on a device are.
Negative assertions are not checked: waiting for a word NOT to show is
satisfied by a word that no longer exists, which is weak but never red.
"""

from __future__ import annotations

import re
import unittest
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[1]
MOBILE = REPO_ROOT / "apps" / "mobile"
FLOWS = MOBILE / ".maestro"
AI_FLOWS = ("_30-ai-khong-khoa.yaml", "_30-ai-co-khoa.yaml", "40-ai-plan.yaml")
#: Words the app draws from a function rather than a literal in one place.
NAMES = ("Bạn",)

STEP = re.compile(
    r'^\s*-?\s*(visible|assertVisible|tapOn):\s*"((?:[^"\\]|\\.)*)"\s*$', re.MULTILINE
)
TYPED = re.compile(r'^\s*-?\s*inputText:\s*"((?:[^"\\]|\\.)*)"\s*$', re.MULTILINE)


def source() -> str:
    parts = []
    for pattern in ("*.ts", "*.tsx"):
        for path in sorted((MOBILE / "src").rglob(pattern)):
            parts.append(path.read_text(encoding="utf-8"))
    return "\n".join(parts)


def typed() -> set[str]:
    out: set[str] = set()
    for path in FLOWS.glob("*.yaml"):
        out.update(TYPED.findall(path.read_text(encoding="utf-8")))
    return out


def alternatives(pattern: str) -> list[str]:
    """Split on top-level `|`, leaving groups whole."""
    out, depth, cur = [], 0, ""
    for ch in pattern:
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
        if ch == "|" and depth == 0:
            out.append(cur)
            cur = ""
        else:
            cur += ch
    out.append(cur)
    return out


def fragments(alt: str, typed_words: set[str]) -> list[str]:
    if alt in typed_words:
        return []
    for words in sorted(typed_words, key=len, reverse=True):
        if words and alt.endswith(words):
            alt = alt[: -len(words)]
            break
    out: list[str] = []
    for group in re.findall(r"\(([^()]*)\)", alt):
        for option in group.split("|"):
            out += fragments(option, typed_words)
    alt = (
        re.sub(r"\([^()]*\)", "\x00", alt)
        .replace("[0-9]+", "\x00")
        .replace(".*", "\x00")
    )
    for name in NAMES:
        alt = alt.replace(name, "\x00")
    return out + [f for f in alt.split("\x00") if f.strip()]


def dead_words(flow_text: str, src: str, typed_words: set[str]) -> list[str]:
    dead = []
    for key, pattern in STEP.findall(flow_text):
        for alt in alternatives(pattern):
            for fragment in fragments(alt, typed_words):
                if fragment not in src:
                    dead.append(f"{key}: {pattern!r} -> {fragment!r}")
    return dead


class AiFlowsNameLiveWords(unittest.TestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.src = source()
        cls.typed = typed()

    def test_every_ai_flow_waits_only_for_live_words(self):
        for name in AI_FLOWS:
            with self.subTest(name):
                text = (FLOWS / name).read_text(encoding="utf-8")
                self.assertTrue(
                    STEP.search(text), f"{name}: no step read; the pattern drifted"
                )
                self.assertEqual(dead_words(text, self.src, self.typed), [])

    def test_the_names_split_out_are_drawn_by_the_app(self):
        for name in NAMES:
            self.assertIn(f'"{name}"', self.src)

    def test_the_deleted_tray_sentence_is_caught(self):
        """The exact step that went dead in slice 7 must read as dead."""
        old = '- extendedWaitUntil:\n    visible: "Chỉ lời nhờ trong ô này được gửi cho AI. Lịch sử chat không được chia sẻ."\n'
        self.assertEqual(len(dead_words(old, self.src, self.typed)), 1)

    def test_typed_words_and_regex_pieces_are_not_mistaken_for_app_words(self):
        step = '- extendedWaitUntil:\n    visible: "Trả lời Bạn: @Ru Di goi y quan an|Kèm [0-9]+ tin gần đây"\n'
        self.assertEqual(dead_words(step, self.src, self.typed), [])


if __name__ == "__main__":
    unittest.main()
