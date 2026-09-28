"""Statistics for the answer-quality evals: what a number from k runs may claim.

A case is the unit. Runs of one case are not independent of each other (same
prompt, same catalogue), so every interval here resamples whole cases, never
runs. Pooling runs and quoting a Wilson interval would read narrower than the
data allows.

Pure functions, no model and no file access, so each can be checked against a
known answer offline (``test_thong_ke.py``).
"""

from __future__ import annotations

import random
from collections import defaultdict
from collections.abc import Iterable, Sequence
from dataclasses import dataclass

#: Fixed so two graders of the same saved run print the same interval.
SEED = 20260925
RESAMPLES = 10_000


@dataclass(frozen=True)
class KetQuaCa:
    """One case over its runs: how many runs passed every machine check."""

    case_id: str
    dat: int
    tong: int

    @property
    def ty_le(self) -> float:
        return self.dat / self.tong


def gop_theo_ca(rows: Iterable[tuple[str, bool]]) -> list[KetQuaCa]:
    """(case_id, passed) per run → one KetQuaCa per case, in first-seen order."""
    order: list[str] = []
    dat: dict[str, int] = defaultdict(int)
    tong: dict[str, int] = defaultdict(int)
    for case_id, passed in rows:
        if case_id not in tong:
            order.append(case_id)
        tong[case_id] += 1
        dat[case_id] += 1 if passed else 0
    return [KetQuaCa(c, dat[c], tong[c]) for c in order]


def pass_at_1(cases: Sequence[KetQuaCa]) -> float:
    """Mean over cases of the fraction of runs that passed."""
    if not cases:
        raise ValueError("không có ca nào")
    return sum(c.ty_le for c in cases) / len(cases)


def _quantile(sorted_values: Sequence[float], q: float) -> float:
    """Linear interpolation between order statistics (numpy's default)."""
    if not sorted_values:
        raise ValueError("không có giá trị")
    pos = (len(sorted_values) - 1) * q
    lo = int(pos)
    hi = min(lo + 1, len(sorted_values) - 1)
    return sorted_values[lo] + (sorted_values[hi] - sorted_values[lo]) * (pos - lo)


def bootstrap_ci(
    values: Sequence[float],
    *,
    resamples: int = RESAMPLES,
    seed: int = SEED,
    alpha: float = 0.05,
) -> tuple[float, float]:
    """Percentile interval for the mean of per-case values, resampling cases."""
    if not values:
        raise ValueError("không có ca nào")
    rng = random.Random(seed)
    n = len(values)
    means = sorted(
        sum(rng.choice(values) for _ in range(n)) / n for _ in range(resamples)
    )
    return _quantile(means, alpha / 2), _quantile(means, 1 - alpha / 2)


def ca_vung(cases: Sequence[KetQuaCa], *, it_nhat: int) -> int:
    """Cases that passed at least ``it_nhat`` of their runs ("solid" cases)."""
    return sum(1 for c in cases if c.dat >= it_nhat)


def pass_mu_k(cases: Sequence[KetQuaCa]) -> int:
    """Cases whose every run passed."""
    return sum(1 for c in cases if c.dat == c.tong)


def can_tren_khi_khong_loi(n: int) -> float:
    """95% upper bound on a failure rate after n runs with zero failures (3/n)."""
    if n <= 0:
        raise ValueError("n phải dương")
    return min(1.0, 3.0 / n)


def delta_ghep_cap(
    nen: dict[str, float],
    moi: dict[str, float],
    *,
    resamples: int = RESAMPLES,
    seed: int = SEED,
    alpha: float = 0.05,
) -> tuple[float, tuple[float, float], list[str]]:
    """Paired change new − baseline over the cases both runs share.

    Returns (Δ, CI of Δ, the case ids compared). Cases only one side has are
    left out and must be reported separately by the caller: comparing across
    different case sets is how a regression hides.
    """
    common = [c for c in nen if c in moi]
    if not common:
        raise ValueError("hai lượt không có ca chung")
    diffs = [moi[c] - nen[c] for c in common]
    delta = sum(diffs) / len(diffs)
    return (
        delta,
        bootstrap_ci(diffs, resamples=resamples, seed=seed, alpha=alpha),
        common,
    )
