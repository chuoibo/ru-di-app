# R1 — Reduce Motion tới điều hướng stack: đo bằng khung hình, không đọc mã

Tái audit 10/09 của Codex (R1, P1): đặt cả ba `*_animation_scale` = 0, mở chi tiết quán từ Khám phá **vẫn trượt**
ngang toàn màn (`reaudit-evidence/visual-reduce-confirm.mp4`, khung `reduce-confirm-06.png`). Đúng, và lý do nằm ở
hai chỗ mã cũ không nhìn thấy:

1. `app/_layout.tsx:155` khai `animation: "slide_from_right"` **cố định**. react-native-screens trên Android chuyển
   cảnh bằng Fragment `Animation` (`android.view.animation`), thứ **không** đi qua `animator_duration_scale` hay hai
   scale còn lại; thư viện cũng không có mã nào đọc Reduce Motion (đã grep `android/src/main/java/com/swmansion/rnscreens`).
   Nên scale 0 không tắt được cú trượt — **app phải tự xin cắt cảnh**.
2. `ui/useMotion.ts` truyền `reduceMotion: ReduceMotion.System` cho spring/timing của Reanimated; cờ System được
   Reanimated đọc **một lần lúc khởi động**, nên đổi setting giữa phiên thì sheet «Tạo mới» vẫn spring (đo được:
   7 khung ở scale 0 trước khi sửa, xem lịch sử ở cuối).

## Sửa

- `src/rudi/motion.ts` (thuần, có test): `stackAnimation(wanted, reduceMotion)` → `"none"` khi giảm chuyển động.
  `tests/motion.test.mjs` pin ba giá trị → `none` và giữ nguyên khi không giảm.
- `app/_layout.tsx` `RootInner`: `const motion = useMotion()`; `screenOptions.animation` và bảy `Stack.Screen`
  đều qua `stackAnimation(...)`. `useMotion` đã subscribe `reduceMotionChanged` (RN Android có ContentObserver trên
  `TRANSITION_ANIMATION_SCALE`) nên Stack re-render ngay khi setting đổi.
- `src/rudi/ui/useMotion.ts`: spring/timing dùng `reduced ? ReduceMotion.Always : ReduceMotion.Never` — bit sống
  quyết định, Reanimated chỉ được báo kết quả.

## Cách đo: `quay-chuyen-canh.sh` + `so-khung.py`

`adb shell screenrecord` quanh **một** flow Maestro chỉ có đúng hai lần đổi màn (vào rồi Back, có chờ hai đầu);
`ffmpeg fps=30` tách khung; `so-khung.py` so mỗi khung với khung **trước nó** (ảnh xám 54×120, ngưỡng 0.02): một
chuỗi khung liên tiếp «đang đổi» là **một lần đổi màn**, độ dài chuỗi là số khung của lần đổi ấy. Trượt 300 ms ở
30 fps ≈ 7–9 khung; cắt thẳng = 1 khung.

**Cổng có đối chứng hai chiều:** scale 1 phải cho `max ≥ 3` (cổng nhìn thấy cú trượt); scale 0 sau sửa phải cho
`max ≤ 2` ở mọi lần đổi. Cổng chạy trên chính ba video của Codex trước khi tin nó:

| video của Codex | runs | max | đọc |
|---|---|---|---|
| `visual-reduce-confirm.mp4` (scale 0, **trước** sửa) | [8, 4, 7, 1, 2, 7, 1, 1, 1, 1] | 8 | còn trượt — đúng như họ thấy |
| `visual-normal.mp4` (scale 1) | [2, 1, 1, 5, 1, 7, 2, 6, 1, 1, 1, 1] | 7 | trượt |
| `visual-reduce.mp4` (scale 0, trước sửa) | [6, 1, 4, 2, 6, 10, 1, 1, 1, 1] | 10 | còn trượt |

(`codex-video/*.khung.json`.) Các run 1–2 khung lẻ là chấm tải nội dung / đồng hồ status bar, không phải chuyển cảnh.

## Kết quả trên dev client sau sửa (bundle dấu vân `claude-r1-0c1a4169-093727`, Metro 8096, fixture)

Chạy **liên tiếp trong một phiên app**, không mở lại app giữa các lượt — a→b→c chính là phép thử «đổi setting
trong phiên» mà Codex yêu cầu:

| lượt | flow | scale | runs (khung mỗi lần đổi màn) | max | đọc |
|---|---|---|---|---|---|
| a | `r1-chi-tiet` vào chi tiết «Tiệm Nướng Xóm Lèo» + Back | 1 | [4, 1, 9] | 9 | trượt (đối chứng dương) |
| b | cùng flow, **scale đặt về 0 khi app đang foreground** | 0 | [1, 1] | 1 | **cắt thẳng** |
| c | cùng flow, scale trả về 1 | 1 | [7, 1, 9] | 9 | trượt trở lại |
| d | `r1-tao-moi` sheet «Tạo mới» (transparentModal + Sheet spring) + Back | 1 | [7, 5] | 7 | trượt |
| e | cùng flow | 0 | [1, 1] | 1 | **cắt thẳng** |
| f | `r1-modal` `rudi://check-ins/new` (`presentation: modal`, slide_from_bottom) + Back | 1 | [6, 3, 7] | 7 | trượt |
| g | cùng flow | 0 | [1, 1] | 1 | **cắt thẳng** |

Khung trước/giữa/sau của lần đổi dài nhất mỗi lượt ở `r1/*.png` (mở ra nhìn: `a-…-giua-289` là hai màn cạnh nhau;
`b-…-truoc-213` là Khám phá, `b-…-sau-215` đã là chi tiết trọn, không có khung nào ở giữa). `r1/*.khung.json` là
chuỗi đầy đủ.

**Trạng thái/affordance không mất** khi cắt: khung `sau` của e và g là sheet mở trọn với đủ hành động và màn check-in
đủ tiêu đề + nút — chỉ mất chuyển động, không mất gì khác.

## Chưa chứng minh

- **Bản release**: mọi khung ở đây là dev client + bundle Metro; `stackAnimation` chạy cùng mã trên release nhưng chưa
  có clip release (cần stack sau HTTPS local — quyết định của Codex 10/09).
- **iOS**: `stackAnimation` đã nối cho cả hai nền, nhưng không có máy iOS nên **không có clip**; native-stack iOS còn
  `animationDuration` riêng chưa đụng.
- **TalkBack / các setting trợ năng khác** chạy cùng lúc với Reduce Motion: chưa thử tương tác.
- Máy thật; 120 Hz; cảm giác chạm.
- Chuyển tab (`(tabs)` → `fade` → `none`): không có clip riêng vì tab bar đổi màn tức thì ở cả hai chế độ; số khung
  ở `m1-doi-tab` của bảng đo v2 là phép đo gián tiếp.
- Cảm giác chạm; cắt thẳng là đúng theo «Remove animations» của Android (crossfade hoặc cắt), không phải phán quyết «đẹp».

## Lịch sử một dòng

Lượt quay đầu trên bundle chỉ sửa `_layout.tsx`: chi tiết và modal đã cắt (max 1) nhưng `r1-tao-moi` ở scale 0 vẫn
`[7, 1, 1]` — spring của `Sheet` theo `ReduceMotion.System` cũ. Sửa `useMotion` rồi quay lại **cả bảy** trên một bundle
để bảng trên là một phép đo nhất quán.
