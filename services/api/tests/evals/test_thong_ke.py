"""Known-answer tests for the eval statistics: each must fail on a wrong answer."""

from __future__ import annotations

import pytest

from tests.evals import thong_ke as tk


def test_gop_theo_ca_giu_thu_tu_va_dem_dung() -> None:
    cases = tk.gop_theo_ca([("a", True), ("b", True), ("a", False), ("a", True)])
    assert [(c.case_id, c.dat, c.tong) for c in cases] == [("a", 2, 3), ("b", 1, 1)]


def test_pass_at_1_la_trung_binh_theo_ca_khong_theo_luot() -> None:
    cases = [tk.KetQuaCa("a", 1, 2), tk.KetQuaCa("b", 5, 5)]
    # By case: (0.5 + 1.0) / 2. Pooled runs would give 6/7 = 0.857.
    assert tk.pass_at_1(cases) == pytest.approx(0.75)


def test_phan_vi_noi_suy_tuyen_tinh() -> None:
    values = [1.0, 2.0, 3.0, 4.0, 5.0]
    assert tk._quantile(values, 0.0) == 1.0
    assert tk._quantile(values, 0.25) == 2.0
    assert tk._quantile(values, 0.5) == 3.0
    assert tk._quantile(values, 0.9) == pytest.approx(4.6)
    assert tk._quantile(values, 1.0) == 5.0


def test_du_lieu_hang_so_cho_khoang_rong_bang_khong() -> None:
    assert tk.bootstrap_ci([0.6] * 16) == (pytest.approx(0.6), pytest.approx(0.6))


def test_khoang_tin_cay_tat_dinh_theo_hat_giong() -> None:
    values = [0.0, 0.2, 0.4, 1.0, 1.0, 0.6, 0.8, 0.0]
    assert tk.bootstrap_ci(values) == tk.bootstrap_ci(values)
    lo, hi = tk.bootstrap_ci(values)
    mean = sum(values) / len(values)
    assert lo < mean < hi


def test_lay_mau_lai_theo_ca_nen_khoang_rong_hon_gop_luot() -> None:
    # Ten cases, each all-or-nothing over five runs, half of them passing.
    # Resampling cases sees 10 draws of 0 or 1; pooling 50 runs would see 50
    # and print an interval about half as wide. The width is the claim.
    per_case = [1.0] * 5 + [0.0] * 5
    lo, hi = tk.bootstrap_ci(per_case)
    assert hi - lo >= 0.5, (lo, hi)


def test_ca_vung_va_pass_mu_k() -> None:
    cases = [tk.KetQuaCa("a", 5, 5), tk.KetQuaCa("b", 4, 5), tk.KetQuaCa("c", 3, 5)]
    assert tk.ca_vung(cases, it_nhat=4) == 2
    assert tk.pass_mu_k(cases) == 1


def test_can_tren_ba_chia_n() -> None:
    assert tk.can_tren_khi_khong_loi(80) == pytest.approx(0.0375)
    assert tk.can_tren_khi_khong_loi(2) == 1.0
    with pytest.raises(ValueError):
        tk.can_tren_khi_khong_loi(0)


def test_so_ghep_cap_chi_tren_ca_chung() -> None:
    delta, (lo, hi), common = tk.delta_ghep_cap(
        {"a": 0.2, "b": 0.4}, {"a": 0.6, "b": 0.4, "c": 1.0}
    )
    assert common == ["a", "b"]
    assert delta == pytest.approx(0.2)
    assert lo <= delta <= hi
    with pytest.raises(ValueError):
        tk.delta_ghep_cap({"a": 1.0}, {"b": 1.0})
