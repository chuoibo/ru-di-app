# Trả lời tái audit 10/09 (Codex) — «đã có bản sắc, chưa stunning xuyên suốt»

Trả lời cho `docs/codex/2026-09-10/reaudit-native-my-thuat.md` (+ `assessment-a-my-thuat.md`, `assessment-b-ky-thuat.md`)
trên `main 0c1a4169`. Phán quyết của họ: **REQUEST_CHANGES cho cổng mở rộng mỹ thuật và đóng §5 motion**. Tài liệu này
ghi từng finding: đã đo gì, chưa đo gì, quyết định nào để lại cho team. Bằng chứng của Codex được commit **một phần**
(ba bản `.md`, manifest, kết quả ma trận/motion, và đúng những PNG được trích dẫn ở đây — pin sha256); video `.mp4` của họ
không vào Git, chỉ dẫn đường dẫn trong checkout.

Thứ tự làm theo mức của họ: R1/R2 (P1) → R3 (P2, ưu tiên cho «stunning») → F45 còn mở + R4 (P2) → R5 (P3).

## PR 1 — R1 · R2: Reduce Motion tới stack, cổng đo fail-closed

### R1 (P1) — đóng trên Android dev client, có đối chứng hai chiều

Codex đúng, và đúng sâu hơn dòng họ chỉ: `_layout.tsx:155` là literal, **và** react-native-screens Android chuyển
cảnh bằng Fragment `Animation` không đi qua ba `*_animation_scale` (không có mã Reduce Motion nào trong thư viện), nên
scale 0 không bao giờ tắt được cú trượt — app phải tự xin cắt. Thêm một chỗ họ chưa thấy: spring của kit `Sheet` dùng
`ReduceMotion.System` của Reanimated, cờ đọc một lần lúc khởi động → đổi setting giữa phiên, sheet «Tạo mới» vẫn spring
(đo được 7 khung ở scale 0 sau khi chỉ sửa `_layout`).

Sửa: `stackAnimation()` trong `motion.ts` (thuần, test) → `_layout.tsx` đọc `useMotion().reduced` cho `screenOptions`
và bảy `Stack.Screen`; `useMotion` truyền `Always/Never` theo bit sống thay cho `System`.

Đo bằng khung hình, không đọc mã (`docs/claude/2026-09-11/motion-v2/README.md`): screenrecord quanh một flow chỉ có hai
lần đổi màn, `so-khung.py` đếm số khung mỗi lần đổi. Cổng tự kiểm trên **chính video của Codex** trước: `visual-reduce-
confirm.mp4` cho max 8 khung (còn trượt — đúng như họ thấy). Sau sửa, cùng một phiên app, đổi scale khi app foreground:

| lượt | flow | scale | khung mỗi lần đổi màn | đọc |
|---|---|---|---|---|
| a | chi tiết «Tiệm Nướng Xóm Lèo» + Back | 1 | [4, 1, 9] | trượt (đối chứng dương) |
| b | cùng flow, scale về 0 **giữa phiên** | 0 | [1, 1] | cắt thẳng |
| c | cùng flow, scale về 1 | 1 | [7, 1, 9] | trượt trở lại |
| d / e | sheet «Tạo mới» (transparentModal + Sheet) + Back | 1 / 0 | [7, 5] / [1, 1] | trượt / cắt |
| f / g | `rudi://check-ins/new` (`presentation: modal`) + Back | 1 / 0 | [6, 3, 7] / [1, 1] | trượt / cắt |

Khung trước/giữa/sau ở `motion-v2/r1/*.png`; trạng thái cuối khi cắt là màn trọn (sheet đủ bốn hành động, check-in đủ
tiêu đề + nút) — chỉ mất chuyển động. **Chưa đo:** iOS, máy thật, 120 Hz, TalkBack.

### R2 (P1 cho cổng) — sửa phương pháp, đo lại; canary đỏ phải đỏ

Đúng cả năm ý của B1–B3 (xem README motion). `do-motion.sh` v2: warm-up **một lần** ngoài mọi cửa sổ; mỗi chuỗi
`pid` trước → `gfxinfo reset` khi tiến trình sống → flow **chỉ thao tác** (m1–m4 bỏ `runFlow _vao-*`, mở đầu
`assertVisible "Khám phá"`) → dump → `pid` sau; hàng hợp lệ khi `rc=0 && pid không đổi && khung>0`, **exit 1** nếu có
hàng không hợp lệ; ba scale được đọc, lưu và **trả đúng giá trị gốc bằng trap**; cột «khung>150ms» = tổng bucket ≥150
(150 là **một bucket**, histogram tới 4950 ms — README cũ nói «trần bucket» là sai, đã sửa); `_vao-live.yaml` rẽ cả
hai điểm vào («Rủ Đi thôi!» và «Chào bạn») qua `_nhap-otp.yaml`.

`do-motion-canary.sh` (adb/maestro giả, không chạm máy): maestro exit 42 → runner exit 1 ✓; maestro exit 0 + dump
thật → exit 0 và hàng có số ✓ (khung>150ms = 12 = 7+4+1 đúng histogram m1 cũ). Bài học đắt một lần: canary bản đầu
để `~/.maestro/bin` đứng trước PATH giả → chạy Maestro **thật** lên máy ảo; runner giờ chỉ **nối thêm** đường tool khi
PATH chưa có.

Bảng v2 (dev client, sau sửa R1; `docs/claude/2026-09-10/motion/dev-client-v2/`, pid không đổi, exit 0 cả hai lượt):

| chuỗi | thường: khung / janky / p99 / khung>150ms | Reduce Motion: khung / janky / p99 / khung>150ms |
|---|---|---|
| m1 đổi 4 tab ×5 | 271 / 13,28% / 85ms / 0 | 45 / 80,00% / 97ms / 0 |
| m2 cuộn ×4 | 907 / 2,54% / 31ms / 0 | 704 / 1,99% / 27ms / 0 |
| m3 sheet ×5 | 406 / 8,62% / 34ms / 0 | 63 / 33,33% / 31ms / 0 |
| m4 chi tiết + Back ×5 | 273 / 10,62% / 65ms / 0 | 60 / 31,67% / 40ms / 0 |

Số khung lượt thường (271 / 907 / 406 / 273) gần với cửa sổ Codex tự đo (267 / 848 / 317 / 280): hai phép đo độc lập
khoanh được cùng một thao tác. Không so % với bảng v1 (khác cửa sổ), không so % thường–reduce (khác mẫu số: cắt cảnh làm
mất gần hết khung chuyển cảnh — đổi tab 20 lần còn 45 khung). p99 là nhãn bucket; không khung nào ≥150 ms ở cả tám hàng.
Đây là **baseline một lượt trên máy ảo**, không phải ngưỡng release.

## Cố ý không làm trong lượt này

- Đo release qua HTTPS (Codex chọn HTTPS): việc dựng stack sau TLS là một lượt riêng; tầng 1 v2 đủ để đóng R2 về
  **phương pháp**, không tuyên «§5 đạt».
- Ảnh demo Album nhất quán + ghi công (nợ §4 cũ), Sở thích phân nhóm, glyph gamepad Puppy Farm, «bản tối mất chất
  giấy» (ghi câu hỏi mở), iOS/máy thật/TalkBack/chat live, animation mới.
