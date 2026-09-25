"""Where an eval run's evidence lives, and what may be said about it in Git.

Model output is not evidence that belongs in the repository: a checked-in answer
starts being read as the expected one, and CLAUDE.md keeps raw transcripts out
of Git. So a run writes its cards and its score table to an evidence store
outside every worktree, and what reaches Git is a content-free manifest's
numbers, copied into the commit message as trailer lines.
"""

from __future__ import annotations

import hashlib
import json
import re
import subprocess
from pathlib import Path

#: The default evidence store (docs/claude/2026-09-25/thiet-ke-ai/06-bo-do-chat-luong.md §7).
KHO_MAC_DINH = Path.home() / ".cache" / "rudi-bang-chung" / "eval"


class NgoaiKho(ValueError):
    """An output directory that would put model output inside a git worktree."""


def trong_worktree(path: Path) -> bool:
    """True when path, or the nearest existing parent, is inside a git worktree."""
    here = path.resolve()
    while True:
        if (here / ".git").exists():
            return True
        if here.parent == here:
            return False
        here = here.parent


def thu_muc_ra(out: Path | None, run_id: str) -> Path:
    """The run's evidence directory; refuses anything inside a worktree."""
    base = out if out is not None else KHO_MAC_DINH
    target = base / run_id
    if trong_worktree(target):
        raise NgoaiKho(
            f"{target} nằm trong một worktree git: đầu ra model không vào Git. "
            f"Bỏ --out để dùng kho {KHO_MAC_DINH}."
        )
    return target


def git_sha(repo: Path) -> str:
    """HEAD of the repository, with -dirty when the tree has uncommitted changes."""
    sha = subprocess.run(
        ["git", "-C", str(repo), "rev-parse", "HEAD"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    dirty = subprocess.run(
        ["git", "-C", str(repo), "status", "--porcelain", "--untracked-files=no"],
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    return sha + ("-dirty" if dirty else "")


def sha256_file(path: Path) -> str:
    return hashlib.sha256(path.read_bytes()).hexdigest()


#: Every key a manifest may hold. A key outside this list is refused, so a
#: field carrying model text cannot be added without editing the list.
KHOA_MANIFEST = frozenset(
    {
        "run_id",
        "git_sha",
        "corpus",
        "corpus_sha256",
        "model",
        "lap",
        "loi_goi",
        "chi_so",
        "chua_xong",
        "loi_ha_tang",
    }
)
_CHUOI_AN_TOAN = re.compile(r"^[A-Za-z0-9_.:@/+\-]{0,120}$")


def kiem_khong_noi_dung(value: object, *, where: str = "manifest") -> None:
    """Refuse any string that could be prose: only ids, hashes, names, numbers."""
    if isinstance(value, dict):
        for key, item in value.items():
            if not isinstance(key, str) or not _CHUOI_AN_TOAN.match(key):
                raise ValueError(f"{where}: khoá lạ {key!r}")
            kiem_khong_noi_dung(item, where=f"{where}.{key}")
    elif isinstance(value, list):
        for i, item in enumerate(value):
            kiem_khong_noi_dung(item, where=f"{where}[{i}]")
    elif isinstance(value, str):
        if not _CHUOI_AN_TOAN.match(value):
            raise ValueError(f"{where}: chuỗi không phải id/tên ({value[:40]!r})")
    elif value is not None and not isinstance(value, bool | int | float):
        raise ValueError(f"{where}: kiểu {type(value).__name__} không được phép")


def manifest(**fields: object) -> dict:
    extra = set(fields) - KHOA_MANIFEST
    if extra:
        raise ValueError(f"manifest: khoá ngoài danh sách {sorted(extra)}")
    kiem_khong_noi_dung(fields)
    return dict(fields)


def ghi_manifest(directory: Path, data: dict) -> Path:
    kiem_khong_noi_dung(data)
    path = directory / "manifest.json"
    path.write_text(
        json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
    )
    return path


def trailer_nhom_plan(
    *,
    run_id: str,
    lap: int,
    goi_da_dung: int,
    goi_duyet: int,
    model: str,
    so_ca: int,
    ca_vung: int,
    it_nhat: int,
    pass_at_1: float,
    ci: tuple[float, float],
) -> list[str]:
    """Commit-message trailer lines for one group-plan run."""
    return [
        f"Eval-Run: {run_id} lap={lap} goi={goi_da_dung}/{goi_duyet} model={model}",
        (
            f"Eval-Nhom-Plan: vung {ca_vung}/{so_ca} (>={it_nhat}/{lap})"
            f" pass@1 {pass_at_1:.2f} [{ci[0]:.2f},{ci[1]:.2f}]"
        ),
    ]
