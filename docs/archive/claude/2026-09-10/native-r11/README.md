# Bằng chứng native r11 — F41, hàng sticker hỏng ở ba cỡ chữ × hai theme

Máy: `emulator-5554`, Android 15, 1080×2400 @420dpi, dev client 1.0.0, bundle của cây
`claude/p0-w-ui5-chat-phan-hoi-muon` (dấu vân `92d0f241-…` của lượt, in trong log bảng; bảng
`.maestro-bs-r11` XANH: flow 00 + 76 qua, NEO 2b cắn, canary đỏ đúng thiết kế). Dữ liệu demo.

| Ảnh | Cấu hình | Bounds (`kiem-bounds.mjs`) |
|---|---|---|
| `bang-r11-76-fs1.0-sang.png` | 1.0 · sáng, chụp bởi bảng | (bảng không dump hierarchy) |
| `fs1.0-sang.png` · `.xml` · `.bounds.txt` | 1.0 · sáng | XANH — «Thử lại» 194px, «Bỏ» **126px = 48dp** (sàn), chữ lề 39/40px |
| `fs1.0-toi.png` | 1.0 · tối | XANH |
| `fs1.3-sang.png` | 1.3 · sáng | XANH — 234px, lề 39/40 |
| `fs1.3-toi.png` | 1.3 · tối | XANH |
| `fs2.0-sang.png` | 2.0 · sáng | XANH — 293px, lề 39/40 |
| `fs2.0-toi.png` | 2.0 · tối | XANH |

`vong-do.log` là kết quả đầy đủ của sáu lượt — **lượt chụp lại** sau vòng review context mới: bản đầu
của fix để «Bỏ» theo nội dung còn 124px = 47,2dp (dưới sàn 48dp), nay `RudiButton` có `minWidth: 48`
và cổng bounds đòi mỗi nút ≥ 48dp hai chiều (bounds cũ đỏ với sàn ấy). Ở 1.3 và 2.0 chỉ còn hai nút trong dump vì hàng
«hỏng vĩnh viễn» rơi xuống dưới khung màn — đúng, màn chứa ít hơn; hàng có hai nút là hàng đo.

## Vì sao đo bounds chứ không `assertVisible`

XML 16 của Codex có node «Thử lại» trong khi nút vẽ ngoài màn: `uiautomator` **kẹp** bounds về mép
(`[0,1209]→[195,1335]`) và **bỏ** node chữ đã rơi hẳn khỏi màn. `kiem-bounds.mjs` đòi mỗi nút có node
chữ con cùng chuỗi nằm trong nút với lề ≥ 30px hai bên, nút trong màn, hai nút không chồng. Chạy trên
chính XML 16: **ĐỎ** («KHÔNG có node chữ trong nút»). Bản đầu của script chỉ so `x ≥ 0` và xanh trên
XML 16 — cổng mù, đã bỏ.

## Chạy lại

```bash
ANDROID_ADB_SERVER_PORT=5038 ANDROID_SERIAL=emulator-5554 \
  scripts/mobile_native.sh --flows .maestro-bs-r11 --port 8096 --keep     # từ gốc repo
docs/archive/claude/2026-09-10/native-r11/chup-va-do.sh 8096 <thư-mục-ra>          # sáu cấu hình
```

Chưa đo: chat live thật (cần stack `--otp`); iOS; máy thật.
