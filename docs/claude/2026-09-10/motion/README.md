# Cổng motion hữu hạn — v2 sau tái audit 10/09 (R2/B1–B3)

Audit 09/09 §5 hỏi một cổng đo. Bản v1 (10/09) có số nhưng tái audit 10/09 của Codex chỉ ra **phương pháp sai ở
ba chỗ và một câu diễn giải sai**; v2 sửa cả bốn rồi đo lại. Không thêm animation (bước 3 của audit vẫn là đề xuất).

## Cái v1 làm sai (Codex đúng)

| | v1 | v2 |
|---|---|---|
| Cửa sổ đo (B1) | `force-stop` → `reset` → flow **mở đầu bằng helper 85 dòng** (launch, dev menu, vào fixture, bỏ sở thích, đăng xuất, vào lại) → khởi động lạnh + setup nằm trong histogram | warm-up **một lần** ngoài cửa sổ; mỗi chuỗi: `pid` trước → `reset` khi tiến trình sống → flow **chỉ thao tác** → dump → `pid` sau |
| Exit code (B1) | `rc` chỉ in vào bảng, kết bằng `cat` → **exit 0 khi bốn flow đỏ** (đã xảy ra: `release-thu/bang-khong-hop-le-rc1.md`) | hàng hợp lệ khi `rc=0 && pid không đổi && khung>0`; có hàng không hợp lệ → **exit 1**; `do-motion-canary.sh` chứng minh đỏ phải đỏ, xanh phải xanh |
| Cấu hình (B3) | đặt 0 rồi **literal 1**, không lưu gốc, không trap | đọc và lưu **cả ba** scale, trả đúng giá trị gốc bằng `trap EXIT INT TERM`; `thuong` không ghi scale |
| Diễn giải p99 (B2) | «150ms là trần bucket» | **sai**: histogram tới 4950 ms; 150 là một bucket (m1 v1: 150ms=7, 200ms=4, 250ms=1). v2 có cột `khung>150ms` = tổng bucket ≥150; p50…p99 là **nhãn bucket** chứa percentile |
| «release ít nhất tốt bằng dev» | khẳng định | **giả thuyết**: dev overhead có thể ảnh hưởng; chưa có cặp so sánh tương đương (fixture-dev vs live-release khác workload) |
| Điểm vào live (B3) | `_vao-live.yaml` chỉ rẽ ở «Rủ Đi thôi!» | rẽ cả «Chào bạn» qua `_nhap-otp.yaml` |

## Cái gì được đo, bằng gì

- **Máy**: `emulator-5554`, Android 15, 1080×2400 @420dpi, host WSL2. **Máy ảo không phải điện thoại**: số là đường
  cơ sở hồi quy của rig này, không phải phán quyết «mượt».
- **Build**: dev client + bundle dev từ Metro (fixture, `EXPO_PUBLIC_RUDI_FIXTURE=1`). Bản release chưa đo (xem dưới).
- **Chuỗi** (`apps/mobile/.maestro-motion/m*.yaml`, mỗi file mở bằng `assertVisible "Khám phá"` và kết về đúng màn ấy):
  `m1` đổi bốn tab ×5 · `m2` cuộn Khám phá xuống/lên ×4 · `m3` mở/đóng sheet «Tạo mới» ×5 · `m4` vào chi tiết địa điểm
  rồi Back ×5. Không `launchApp`/`stopApp` bên trong.
- **Reduce Motion**: cùng bốn chuỗi với ba scale = 0 (đúng thứ «Remove animations» đặt và `isReduceMotionEnabled` đọc).
- **Runner**: `do-motion.sh <ra> [thuong|reduce]`; canary `do-motion-canary.sh`. Số liệu lượt này: `dev-client-v2/`.

## Kết quả v2 — dev client, sau sửa R1 (stack cắt cảnh khi Reduce Motion)

Bundle dấu vân `claude-r1-0c1a4169-093727`; CPU host rảnh (không Gradle, không bundling); Metro chỉ phục vụ bundle đã
dựng. Mỗi chuỗi một lượt — **baseline hữu hạn, không phải benchmark nhiều lần**.

Lượt thường (scale 1/1/1, pid 22205 không đổi qua bốn chuỗi, exit 0):

| chuỗi | khung | janky | p50 | p90 | p95 | p99 | khung>150ms | slow UI | slow draw | missed vsync |
|---|---|---|---|---|---|---|---|---|---|---|
| m1-doi-tab | 271 | 13.28% | 26ms | 31ms | 32ms | 85ms | 0 | 21 | 34 | 0 |
| m2-cuon-kham-pha | 907 | 2.54% | 16ms | 26ms | 29ms | 31ms | 0 | 0 | 20 | 0 |
| m3-sheet-tao | 406 | 8.62% | 16ms | 29ms | 31ms | 34ms | 0 | 2 | 28 | 0 |
| m4-back-chi-tiet | 273 | 10.62% | 27ms | 32ms | 36ms | 65ms | 0 | 6 | 28 | 0 |

Lượt Reduce Motion (scale 0/0/0, trả về 1/1/1 sau lượt — `scale-sau.txt`):

| chuỗi | khung | janky | p50 | p90 | p95 | p99 | khung>150ms | slow UI | slow draw | missed vsync |
|---|---|---|---|---|---|---|---|---|---|---|
| m1-doi-tab | 45 | 80.00% | 30ms | 61ms | 69ms | 97ms | 0 | 20 | 29 | 0 |
| m2-cuon-kham-pha | 704 | 1.99% | 16ms | 21ms | 24ms | 27ms | 0 | 0 | 14 | 0 |
| m3-sheet-tao | 63 | 33.33% | 16ms | 16ms | 26ms | 31ms | 0 | 0 | 19 | 0 |
| m4-back-chi-tiet | 60 | 31.67% | 23ms | 29ms | 32ms | 40ms | 0 | 5 | 18 | 0 |

(pid 23771 không đổi qua bốn chuỗi, exit 0.) Đổi tab còn **45 khung** cho 20 lần đổi tab: cắt cảnh nghĩa là gần như không có khung chuyển cảnh nào được vẽ — 80% janky là 36/45 khung mount nặng do JS, không phải «chậm gấp sáu».

Đọc số:
- Số khung v2 (271 / 907 / 406 / 273) gần với cửa sổ Codex tự đo (267 / 848 / 317 / 280) — hai lần đo độc lập cùng
  khoanh được thao tác. **Không so % với bảng v1** (804 / 1482 / 933 / 812 khung): v1 chứa cả khởi động và setup.
- p99 cột thường là **nhãn bucket**; `khung>150ms = 0` ở cả bốn chuỗi nghĩa là không khung nào rơi vào bucket ≥150.
- % janky **không so được** giữa thường và Reduce Motion: cắt cảnh làm mất khung chuyển cảnh, mẫu số khác; đọc số
  tuyệt đối (slow UI, khung>150ms) và số khung.
- Codex đúng khi nói dev overhead **có thể** ảnh hưởng; bản này không nói release tốt hơn hay kém hơn — chưa đo.

## Cái này KHÔNG chứng minh

- Cảm giác chạm trên điện thoại thật; 120 Hz; iOS; haptic; độ trễ touch.
- Release/live: cần stack sau **HTTPS local** (quyết định của Codex 10/09; bản release chặn HTTP cleartext theo thiết
  kế). `FLOWS_DIR=.maestro-motion-live OTP_PHONE=… do-motion.sh` đã sẵn; live `m4` đo `rudi://destinations`, **không**
  đo `places/[id]` vì tên địa điểm live không cố định.
- Khung hình **giữa** chuyển cảnh: cổng ấy nằm ở `docs/claude/2026-09-11/motion-v2/` (R1), không ở bảng này.

## Chạy lại

```bash
# dev client đang nối Metro của cây này, app đã cài; runner tự warm-up bằng _vao-app-sach
docs/claude/2026-09-10/motion/do-motion.sh <ra> thuong
docs/claude/2026-09-10/motion/do-motion.sh <ra-reduce> reduce
docs/claude/2026-09-10/motion/do-motion-canary.sh          # tự kiểm runner, không chạm máy
# live/release (khi có HTTPS local):
FLOWS_DIR=.maestro-motion-live OTP_PHONE=<số> OTP_CODE=000000 docs/claude/2026-09-10/motion/do-motion.sh <ra> thuong
```

## Lịch sử (v1, 10/09) — giữ để đối chiếu, không dùng làm số

Bảng v1 ở `dev-client/{thuong,reduce}/bang.md` và `release-thu/`; lý do không dùng ở phần đầu tài liệu này.
