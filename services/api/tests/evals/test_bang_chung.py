"""The evidence store refuses Git and prose; the trailer has one shape."""

from __future__ import annotations

from pathlib import Path

import pytest

from tests.evals import bang_chung as bc

REPO = Path(__file__).resolve().parents[4]


def test_thu_muc_trong_repo_bi_tu_choi() -> None:
    with pytest.raises(bc.NgoaiKho):
        bc.thu_muc_ra(REPO / "services" / "api", "run-1")


def test_thu_muc_ngoai_repo_duoc_nhan(tmp_path: Path) -> None:
    assert bc.thu_muc_ra(tmp_path, "run-1") == tmp_path / "run-1"


def test_kho_mac_dinh_nam_ngoai_repo() -> None:
    assert not bc.trong_worktree(bc.KHO_MAC_DINH)


def test_manifest_tu_choi_khoa_la_va_van_xuoi() -> None:
    ok = bc.manifest(run_id="demo-run-abcdef12", model="gemini-3.5-flash-lite", lap=5)
    assert ok["lap"] == 5
    with pytest.raises(ValueError):
        bc.manifest(the_tho={"text": "x"})
    with pytest.raises(ValueError):
        bc.manifest(model="Quán này ngon lắm, đi thôi")
    with pytest.raises(ValueError):
        bc.manifest(chi_so={"nhom_plan": {"ghi_chu": "câu trả lời của model"}})


def test_trailer_dung_hinh() -> None:
    lines = bc.trailer_nhom_plan(
        run_id="r1",
        lap=5,
        goi_da_dung=80,
        goi_duyet=80,
        model="m",
        so_ca=16,
        ca_vung=10,
        it_nhat=4,
        pass_at_1=0.6312,
        ci=(0.5, 0.75),
    )
    assert lines == [
        "Eval-Run: r1 lap=5 goi=80/80 model=m",
        "Eval-Nhom-Plan: vung 10/16 (>=4/5) pass@1 0.63 [0.50,0.75]",
    ]
