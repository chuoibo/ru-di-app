# Bằng chứng native r12 — F43 câu lỗi, F44 placeholder ô tìm ở ba cỡ chữ × hai theme

Máy: `emulator-5554`, Android 15, 1080×2400 @420dpi, dev client 1.0.0, bundle của nhánh
`claude/p0-w-ui5-loi-noi-voi-nguoi` (dấu vân `ad9c8532-…`, in trong log bảng). Bảng `.maestro-bs-r12`
XANH: flow 00 · 77 · 78 · 79 qua, NEO 2b cắn, canary đỏ đúng thiết kế. Dữ liệu demo, không có stack API
(đúng cách tạo lỗi kết nối như audit đã làm ở ảnh 22).

| Flow | Đo gì | Ảnh |
|---|---|---|
| 77 `loi-noi-voi-nguoi` | `DiemDenScreen` không có máy chủ → câu status 0 mới; `assertNotVisible "http.*"`; `kiem-placeholder.mjs` từ chối mọi node chữ mang URL | `anh/r12-77-loi-mang-*.png` |
| 78 `o-tim-ban-thu` | ba ô tìm trên ui-lab: câu dài 30 ký tự, câu «Đi đâu?», ô đã gõ | `anh/r12-78-o-tim-*.png` |
| 79 `o-tim-kham-pha` | hàng tìm Khám phá (fixture) — ở chữ lớn ô chiếm cả hàng, nút xuống dòng | `anh/r12-79-kham-pha-*.png` |

## Số đo (`kiem-placeholder.mjs`, px ở 420dpi)

| Cấu hình | 78 bàn thử — câu 30 ký tự / câu «Đi đâu?» | 79 Khám phá — «Tìm quán, món...» | 77 lỗi |
|---|---|---|---|
| 1.0 sáng · tối | cao 63 · rộng 838 (một dòng) | cao 63 · rộng **544** (cùng hàng với hai nút) | XANH, không URL (sáng: bảng; tối: vòng đo) |
| 1.3 sáng · tối | cao 70 · rộng 838 | cao 70 · rộng **838** (ô chiếm cả hàng, nút xuống dòng) | — |
| 2.0 sáng · tối | cao 95 · rộng 838, câu dài cắt «…» | cao 95 · rộng 838 | XANH, không URL (sáng) |

Trần một dòng là `max(24dp, 17sp × cỡ chữ) × 1,35` = 85/85/120px. Bản đầu dùng 1,5 × 24dp × cỡ chữ
(95/123/189px) giả định `lineHeight` nhân tuyến tính — nhưng đo được 63/70/95px: trên Android RN không nhân
`lineHeight` 24dp của kit theo cỡ chữ, hộp dòng = max(lineHeight, glyph 17sp); với trần cũ một hint hai dòng
ở 2.0 (≈190px) chỉ vượt trần 189px **1px** — reviewer context mới chỉ ra, trần đổi theo số đo thật.
14 lượt đo, 14 XANH; ảnh của bảng (1.0 sáng) là `bang-r12-7x-fs1.0-sang.png`.

## Vì sao đo chiều cao node chữ chứ không `assertVisible`

`assertVisible` chỉ biết node có mặt. Hint native của Android **không** là node chữ (uiautomator không lộ
hint), nên ảnh 20 của Codex không đo được bằng cây view — đó cũng là lý do placeholder phải là chữ của
nhà vẽ: một `TextView` thật, `numberOfLines={1}`, cao đúng một dòng. `kiem-placeholder.mjs` đòi mọi node
bắt đầu bằng «Tìm » cao ≤ `max(24dp, 17sp × cỡ chữ) × 1,35` và nằm trong màn; hint xuống dòng là hai dòng, không lọt.
Tự kiểm: XML 20 của Codex → ĐỎ («không có node placeholder»), XML 22 → ĐỎ (node chữ mang URL).

## Chạy lại

```bash
ANDROID_ADB_SERVER_PORT=5038 ANDROID_SERIAL=emulator-5554 \
  scripts/mobile_native.sh --flows .maestro-bs-r12 --port 8096 --keep
docs/archive/claude/2026-09-10/native-r12/chup.sh 8096 <ra> apps/mobile/.maestro-bs-r12/78-o-tim-ban-thu.yaml r12-78-o-tim
docs/archive/claude/2026-09-10/native-r12/chup.sh 8096 <ra> apps/mobile/.maestro-bs-r12/79-o-tim-kham-pha.yaml r12-79-kham-pha
```

Chưa đo: 404/5xx trên máy (câu ấy gác bằng `trang-thai.test.mjs`; máy chỉ tạo được ca mất kết nối không
cần stack); Khám phá **live** (cần stack `--otp`; hàng thích ứng cùng mã với fixture); iOS; máy thật.
