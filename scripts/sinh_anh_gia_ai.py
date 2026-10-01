#!/usr/bin/env python3
"""Render synthetic, non-personal images for real-provider probes of the Go AI
paths (`go run ./cmd/vnlocal-thu anh|tinh-nang`).

Every image is invented: an imaginary shop, invented dishes and amounts, no
person's name, no account number. The output directory must be outside any
Git worktree (repo guard forbids bill images in Git, and so does CLAUDE.md).

    python3 scripts/sinh_anh_gia_ai.py ~/.cache/rudi-bang-chung/agy/anh
"""

from __future__ import annotations

import os
import random
import subprocess
import sys
from pathlib import Path

from PIL import Image, ImageDraw, ImageFont

FONT = "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf"
FONT_BOLD = "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf"

# The expected readings are part of the fixture: a probe compares the model's
# grounded answer with them.
RECEIPT_LINES = [
    ("Cơm gà xối mỡ", "2", "45.000", "90.000"),
    ("Trà đá", "3", "5.000", "15.000"),
    ("Canh chua cá", "1", "60.000", "60.000"),
]
RECEIPT_TOTAL = "165.000"


def _font(size: int, bold: bool = False) -> ImageFont.FreeTypeFont:
    return ImageFont.truetype(FONT_BOLD if bold else FONT, size)


def receipt(path: Path) -> None:
    img = Image.new("RGB", (720, 980), "white")
    d = ImageDraw.Draw(img)
    y = 40
    d.text(
        (360, y), "QUÁN CƠM GIẢ LẬP", font=_font(40, True), fill="black", anchor="mt"
    )
    y += 60
    d.text(
        (360, y),
        "12 Đường Thử Nghiệm, Phường Mẫu",
        font=_font(24),
        fill="black",
        anchor="mt",
    )
    y += 50
    d.text(
        (360, y), "HOÁ ĐƠN THANH TOÁN", font=_font(30, True), fill="black", anchor="mt"
    )
    y += 50
    d.text((40, y), "Bàn: 05     Ngày: 15/09/2026 19:42", font=_font(24), fill="black")
    y += 50
    d.line((40, y, 680, y), fill="black", width=2)
    y += 15
    for x, head in ((40, "Món"), (360, "SL"), (430, "Đơn giá"), (570, "T.Tiền")):
        d.text((x, y), head, font=_font(24, True), fill="black")
    y += 45
    for name, qty, unit, total in RECEIPT_LINES:
        d.text((40, y), name, font=_font(24), fill="black")
        d.text((370, y), qty, font=_font(24), fill="black")
        d.text((430, y), unit, font=_font(24), fill="black")
        d.text((570, y), total, font=_font(24), fill="black")
        y += 45
    d.line((40, y, 680, y), fill="black", width=2)
    y += 25
    d.text((40, y), "Tổng cộng:", font=_font(30, True), fill="black")
    d.text(
        (680, y), RECEIPT_TOTAL + " đ", font=_font(30, True), fill="black", anchor="ra"
    )
    y += 70
    d.text((360, y), "Cảm ơn quý khách!", font=_font(24), fill="black", anchor="mt")
    img.save(path, "PNG")


def price_list(path: Path) -> None:
    img = Image.new("RGB", (720, 760), (250, 245, 230))
    d = ImageDraw.Draw(img)
    d.text((360, 40), "THỰC ĐƠN", font=_font(44, True), fill="black", anchor="mt")
    d.text((360, 100), "Quán Cà Phê Giả Lập", font=_font(26), fill="black", anchor="mt")
    y = 180
    for name, price in (
        ("Cà phê sữa đá", "29.000"),
        ("Bạc xỉu", "32.000"),
        ("Trà đào cam sả", "45.000"),
        ("Nước ép cam", "40.000"),
        ("Bánh mì trứng", "25.000"),
    ):
        d.text((60, y), name, font=_font(28), fill="black")
        d.text((660, y), price, font=_font(28), fill="black", anchor="ra")
        y += 70
    img.save(path, "PNG")


def transfer(path: Path) -> None:
    img = Image.new("RGB", (720, 1280), (240, 244, 248))
    d = ImageDraw.Draw(img)
    d.rectangle((0, 0, 720, 140), fill=(0, 102, 76))
    d.text(
        (360, 70),
        "Ngân Hàng Thử Nghiệm",
        font=_font(34, True),
        fill="white",
        anchor="mm",
    )
    d.ellipse((300, 200, 420, 320), fill=(0, 160, 90))
    d.text((360, 260), "✓", font=_font(70, True), fill="white", anchor="mm")
    d.text(
        (360, 360),
        "Thanh toán thành công",
        font=_font(34, True),
        fill="black",
        anchor="mt",
    )
    d.text(
        (360, 430), "250.000 VND", font=_font(52, True), fill=(0, 102, 76), anchor="mt"
    )
    y = 560
    for label, value in (
        ("Đến", "CỬA HÀNG BÁNH GIẢ LẬP"),
        ("Nội dung", "Thanh toan don banh"),
        ("Thời gian", "15/09/2026 10:21"),
        ("Mã giao dịch", "FT-THU-NGHIEM"),
    ):
        d.text((50, y), label, font=_font(26), fill=(90, 90, 90))
        d.text((670, y), value, font=_font(26, True), fill="black", anchor="ra")
        y += 70
    img.save(path, "PNG")


def scene(path: Path, sky: tuple[int, int, int], caption: str, seed: int) -> None:
    rnd = random.Random(seed)
    img = Image.new("RGB", (960, 640), sky)
    d = ImageDraw.Draw(img)
    d.rectangle((0, 420, 960, 640), fill=(60, 140, 90))
    for _ in range(6):
        x = rnd.randint(40, 900)
        d.polygon([(x, 420), (x + 50, 250), (x + 100, 420)], fill=(30, 90, 50))
    d.ellipse((760, 60, 860, 160), fill=(255, 220, 90))
    d.rectangle((0, 580, 960, 640), fill=(0, 0, 0))
    d.text((480, 610), caption, font=_font(28, True), fill="white", anchor="mm")
    img.save(path, "JPEG", quality=90)


def noise(path: Path, megabytes: int) -> None:
    side = int((megabytes * 1024 * 1024 / 3) ** 0.5)
    Image.frombytes("RGB", (side, side), os.urandom(side * side * 3)).save(
        path, "PNG", compress_level=0
    )


def main() -> int:
    if len(sys.argv) != 2:
        print(__doc__)
        return 2
    out = Path(sys.argv[1]).expanduser().resolve()
    top = subprocess.run(
        [
            "git",
            "-C",
            str(out.parent if not out.exists() else out),
            "rev-parse",
            "--show-toplevel",
        ],
        capture_output=True,
        text=True,
    )
    if top.returncode == 0:
        print(f"từ chối: {out} nằm trong worktree {top.stdout.strip()}")
        return 2
    out.mkdir(parents=True, exist_ok=True)
    receipt(out / "hoa_don.png")
    price_list(out / "thuc_don.png")
    transfer(out / "chuyen_khoan.png")
    scene(out / "canh_1.jpg", (140, 190, 240), "Đồi thông, buổi sáng", 1)
    scene(out / "canh_2.jpg", (250, 170, 120), "Hoàng hôn bên hồ", 2)
    scene(out / "canh_3.jpg", (180, 200, 220), "Chợ phiên cuối tuần", 3)
    noise(out / "nhieu_15mb.png", 15)
    for p in sorted(out.iterdir()):
        print(f"{p.name}\t{p.stat().st_size} byte")
    return 0


if __name__ == "__main__":
    sys.exit(main())
