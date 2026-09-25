"""The flow-40 server check reads the group AI's answer in the shape it has now.

Since slice 7 (ADR-0039, proposed) a `/plan` asked in the thread is answered by
an ai_card of kind `tra_loi`, a reply whose parts sit in ``payload.phan``. The
check that ran after Maestro flow 40 only counted top-level
``text|places|itinerary`` cards, so against that answer it reported "no AI card
at all", and it never read a single place id inside ``phan``: the grounding
check was blind. That needs a device to run, so this file holds the parsing to
synthetic messages instead -- no emulator, no server, no model.

What this does NOT prove: that the live API returns this shape (the Go
Postgres tier TestTraLoiTrichDungTinTag does), or that flow 40 ran at all.
"""

from __future__ import annotations

import importlib.util
import io
import json
import re
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock

REPO_ROOT = Path(__file__).resolve().parents[1]
CHECK = REPO_ROOT / "scripts" / "kiem_the_ai_sau_40.py"
HARNESS = REPO_ROOT / "scripts" / "mobile_native.sh"

_spec = importlib.util.spec_from_file_location("kiem_the_ai_sau_40", CHECK)
assert _spec is not None and _spec.loader is not None
kiem = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(kiem)

CATALOGUE = {"p-lau", "p-cafe"}
PLACE = {"id": "p-lau", "name": "Synthetic hotpot"}


def tra_loi(*phan: dict) -> dict:
    """An answer as the worker publishes it: author NULL, a reply, signed."""
    return {
        "id": "m-ai",
        "kind": "ai_card",
        "author_id": None,
        "reply_to": {"id": "m-plan", "kind": "text", "preview": "/plan toi nay"},
        "card": {
            "kind": "tra_loi",
            "payload": {
                "ban": 1,
                "tac_gia": "rudi-ai",
                "invocation_id": "synthetic",
                "lenh": "plan",
                "doc": {"so_tin": 0, "chi_loi_nho": True},
                "phan": list(phan),
            },
        },
    }


def text(t: str = "Synthetic answer") -> dict:
    return {"kind": "text", "payload": {"text": t}}


def places(*items: dict) -> dict:
    return {"kind": "places", "payload": {"intro": "Synthetic", "places": list(items)}}


def itinerary(*stop_places: dict) -> dict:
    stops = [{"time_text": "19:00", "note": "", "place": p} for p in stop_places]
    return {"kind": "itinerary", "payload": {"title": "Synthetic plan", "stops": stops}}


class DemTheAi(unittest.TestCase):
    def test_a_tra_loi_answer_is_counted_and_read_inside_phan(self):
        msg = tra_loi(
            text(), places(PLACE), itinerary({"id": "p-cafe", "name": "Synthetic cafe"})
        )
        self.assertEqual(
            kiem.dem_the_ai([msg], CATALOGUE),
            (1, 0, 0, "tra_loi[itinerary+places+text]"),
        )

    def test_a_place_outside_the_catalogue_inside_phan_is_caught(self):
        # In a stop and in a places list: both are where a model can invent.
        for part in (
            itinerary({"id": "p-invented", "name": "Nowhere"}),
            places({"id": "p-invented"}),
        ):
            with self.subTest(part["kind"]):
                answers, unknown, blind, _ = kiem.dem_the_ai(
                    [tra_loi(text(), part)], CATALOGUE
                )
                self.assertEqual((answers, unknown, blind), (1, 1, 0))

    def test_a_part_with_no_readable_id_is_blind_not_clean(self):
        drifted = {"kind": "places", "payload": {"items": [{"id": "p-lau"}]}}
        self.assertEqual(kiem.dem_the_ai([tra_loi(drifted)], CATALOGUE)[:3], (1, 0, 1))

    def test_a_tra_loi_with_no_part_it_can_read_is_blind(self):
        for phan in ([], [{"kind": "poll", "payload": {}}], None):
            with self.subTest(phan):
                msg = tra_loi()
                msg["card"]["payload"]["phan"] = phan
                self.assertEqual(kiem.dem_the_ai([msg], CATALOGUE)[:3], (1, 0, 1))

    def test_an_unwrapped_card_from_an_older_app_still_counts(self):
        old = {"kind": "ai_card", "author_id": None, "card": itinerary(PLACE)}
        self.assertEqual(kiem.dem_the_ai([old], CATALOGUE), (1, 0, 0, "itinerary"))

    def test_what_is_not_an_answer_is_not_counted(self):
        others = [
            {"kind": "text", "author_id": "u", "body": "/plan toi nay"},
            {
                "kind": "ai_card",
                "author_id": None,
                "card": {"kind": "poll", "payload": {"vote_id": "v"}},
            },
            # A person cannot post a tra_loi (GroundCard refuses it); if one
            # ever carried an author, it is not the AI's answer.
            {**tra_loi(text()), "author_id": "u"},
        ]
        self.assertEqual(kiem.dem_the_ai(others, CATALOGUE), (0, 0, 0, ""))

    def test_the_cli_prints_the_line_the_shell_reads(self):
        pages = {
            "/contexts/ctx/messages?limit=50": {
                "messages": [tra_loi(text(), places(PLACE))]
            },
            "/places": {"places": [{"id": "p-lau"}, {"id": "p-cafe"}]},
        }

        def urlopen(req, timeout):
            self.assertEqual(req.headers.get("Authorization"), "Bearer tok")
            return io.BytesIO(
                json.dumps(pages[req.full_url.removeprefix("http://api")]).encode()
            )

        out = io.StringIO()
        with (
            mock.patch.object(kiem.urllib.request, "urlopen", urlopen),
            redirect_stdout(out),
        ):
            self.assertEqual(kiem.main(["x", "http://api", "tok", "ctx"]), 0)
        self.assertEqual(out.getvalue(), "1|0|0|tra_loi[places+text]\n")


class HarnessUsesTheCheck(unittest.TestCase):
    """The shell must call this module, not an inline copy that drifts."""

    def body(self) -> str:
        text = HARNESS.read_text(encoding="utf-8")
        match = re.search(
            r"^kiem_may_chu_sau_40\(\) \{\n(.*?)^\}", text, re.MULTILINE | re.DOTALL
        )
        self.assertIsNotNone(match, "kiem_may_chu_sau_40 is gone from the harness")
        assert match is not None
        return match.group(1)

    def test_the_flow_40_hook_calls_the_module(self):
        body = self.body()
        self.assertIn(
            'python3 "$REPO/scripts/kiem_the_ai_sau_40.py" "$goc" "$tok" "$ctx"', body
        )
        self.assertIn("IFS='|' read -r so_ai so_la so_mu loai", body)

    def test_no_inline_top_level_filter_is_left(self):
        self.assertNotIn('("text", "places", "itinerary")', self.body())


if __name__ == "__main__":
    unittest.main()
