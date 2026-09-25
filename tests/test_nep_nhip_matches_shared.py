"""ADR-0034 §2.5: the weekly ceiling is one number in three places -- the
shared file the client reads, the Python domain, and the Go domain -- and this
refuses the day the first two disagree (the Go side has its own test)."""

from __future__ import annotations

import json
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "services" / "api"))

from app.domain import pair_paper  # noqa: E402


def test_tran_to_moi_tuan_khop_file_dung_chung():
    shared = json.loads((ROOT / "packages" / "shared" / "nep-nhip.json").read_text(encoding="utf-8"))
    assert shared["to_moi_nguoi_moi_tuan"] == pair_paper.TO_MOI_NGUOI_MOI_TUAN
