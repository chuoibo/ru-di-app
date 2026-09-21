# Cổng motion hữu hạn — v4 sau review lượt 4

## Hiện tại: v4 (12/09)

Giữ các sửa B11/B12 của v3, bổ sung ba ràng buộc từ review lượt 4:

- Mỗi bucket phải đúng dạng `<số nguyên>ms=<số đếm nguyên không âm>`, nhãn tăng
  dần và không trùng; chỉ một dòng histogram. Token sai không được AWK ép về 0
  rồi công bố thành “không có khung chậm”. Tổng vẫn phải bằng frames.
- Mọi lần đọc scale phải **rc 0 và số hợp lệ**: gốc hỏng → exit3 trước mọi ghi;
  đọc lại chế độ hỏng → exit4; đọc xác nhận phục hồi hỏng → exit5. Có stdout
  trông như số nhưng lệnh trả lỗi cũng không được tin. PID đọc lỗi cũng vô hiệu.
- Kiểm scale ngay trước và sau mỗi cửa sổ, lưu `m*.scale-truoc/sau.txt`.
  Scale thay đổi trong warm-up hoặc còn lệch sau gesture làm hàng invalid;
  không chạy gesture kế tiếp dưới nhãn cũ. `thuong` vẫn đo cấu hình gốc, không
  ép scale1; phải đọc cấu hình được ghi khi so với Reduce Motion.

`do-motion-canary.sh` hiện có **31 nhánh**. Có đối chứng histogram chứa đúng
ba khung ≥150ms để kiểm parser không chỉ biết trả0. Khi chạy canary mới trên
v3, **14/31 nhánh sai**; chạy trên v4 tất cả đúng. Test
`tests/test_motion_measurement_gate.py` chạy canary trong CI bằng fake ADB và
fake Maestro, không cần máy ảo. `MOTION_RUNNER` chỉ dùng để đưa script cũ vào
canary đối chứng, không thay runner khi đo máy thật.

Giới hạn: đọc trước/sau không bắt được tác nhân đổi scale **rồi trả lại** bên
trong cùng một flow; vẫn cần độc quyền máy ảo trong lúc đo. Không hứa kiểm
chứng toàn bộ trạng thái Android bằng hai mẫu đọc. Các số v1/v2/v3 bên dưới là
lịch sử, không phải benchmark của v4 hoặc bằng chứng release mới.

Hồ sơ sửa, ảnh native và lượt chạy thực v4 nằm tại
`docs/archive/codex/2026-09-12/khep-audit-luot-4/README.md`.

## Lịch sử v3 sau review 11/09 (B11/B12)

Audit 09/09 §5 hỏi một cổng đo. v1 (10/09) có số nhưng phương pháp sai ở ba chỗ (tái audit 10/09). v2 (11/09) sửa
phương pháp và đo lại, nhưng review 11/09 của Codex chạy canary tổng hợp trên chính runner và chỉ ra **lời hứa
fail-closed còn hai lỗ** (B11 · B12). v3 đóng hai lỗ ấy, nói rõ chín điều kiện của một hàng hợp lệ, và chứng minh bằng
canary 16 nhánh — canary ấy **đỏ 11 nhánh trên v2** trước khi được tin. Không thêm animation.

## Cái v2 làm sai (Codex đúng, 11/09)

| | v2 | v3 |
|---|---|---|
| Reset/dump (B11) | `gfxinfo reset` và dump chạy với `>/dev/null 2>&1`, **không đọc rc**: reset hỏng thì khung warm-up/chuỗi trước nằm trong cửa sổ của chuỗi sau mà hàng vẫn hợp lệ | rc của reset kiểm trước khi chạy Maestro: reset hỏng → **không chạy chuỗi**, hàng «KHÔNG HỢP LỆ (reset gfxinfo thất bại)»; rc của dump là một điều kiện hợp lệ |
| Metric thiếu (B11) | hợp lệ chỉ cần `rc=0`, pid không rỗng và bằng nhau, `khung>0`; percentile rỗng vẫn in hàng hợp lệ với `?`; vắng `HISTOGRAM:` thì `khung>150ms` in **0** («không đo được» thành «không có khung chậm») | parser trả **mọi** trường hoặc rỗng; hàng hợp lệ đòi đủ janky %, p50/p90/p95/p99, ba bộ đếm, `HISTOGRAM` có mặt và **cộng đúng bằng frames** (tính chất đúng trên cả 16 dump đã lưu); hàng hợp lệ không bao giờ có `?` |
| pid (B11) | chỉ kiểm không rỗng và bằng nhau → chuỗi `pid-not-known` ổn định qua | pid trước phải là số; pid sau **và pid trong header dump** (`** Graphics info for pid N [...] **`) phải bằng nó |
| Gốc scale (B12) | gốc không đọc được → `that_bai=1` nhưng **vẫn** `put 0`, warm-up và bốn chuỗi; cuối lượt exit 1 và scale ấy còn 0 | gốc nào không phải số → **exit 3 trước mọi `put` và Maestro** |
| Ghi 0 (B12) | không đọc rc, không đọc lại; `put` bị từ chối vẫn đo dưới nhãn `reduce` | mỗi `put 0` phải rc 0 **và** đọc lại bằng 0; sai → trả gốc, **exit 4**, không Maestro; `thuong` không ghi và kiểm scale không đổi giữa lúc đọc và lúc đo |
| Trả gốc (B12) | `put … \|\| true`, không đối chiếu, trap không đổi exit → restore hỏng vẫn exit 0 | trap ghi lại, đọc lại, so bằng gốc; lệch → «KHÔNG TRẢ ĐƯỢC k: muốn X, đọc Y», **exit 5** (INT: 130 chỉ khi trả xong); `scale-sau.txt` có cột `muon/doc/khop` |

## Chín điều kiện của một hàng hợp lệ, và mã thoát

Một hàng chỉ có số khi **tất cả** đúng: (1) Maestro rc 0 · (2) dump rc 0 · (3) pid trước là số · (4) pid sau bằng nó ·
(5) pid trong header dump bằng nó · (6) `Total frames rendered` > 0 · (7) janky %, p50/p90/p95/p99, slow UI, slow draw,
missed vsync đều có và là số · (8) có dòng `HISTOGRAM:` · (9) tổng các bucket của histogram = frames. Điều kiện nào
hỏng được in ngay trong ô «KHÔNG HỢP LỆ (…)». Mã thoát: **0** mọi hàng hợp lệ · **1** có hàng không hợp lệ (bảng vẫn in) ·
**2** tham số sai · **3** gốc scale không đọc được (chưa đụng máy) · **4** không đặt/đọc lại được chế độ đo (đã trả gốc) ·
**5** không trả được gốc (ghi đè mọi mã khác) · **130** ngắt tay và đã trả gốc.

## Canary — 16 nhánh, đỏ phải đỏ ở bước cuối, xanh phải xanh

`do-motion-canary.sh` dựng `adb`/`maestro` giả trên PATH (không chạm máy); adb giả giữ ba scale trong file trạng thái
nên canary đếm được lệnh `put` và đọc được cái để lại; pidof giả trả **pid trong header dump mẫu** nên đối chứng xanh
chứng minh điều kiện (5) qua được trên dữ liệu thật. Gốc giả là `0.5 / 1.5 / 2` (không phải 1) để thấy trả gốc thật.

| nhánh | exit | hàng hợp lệ | maestro | ghi chú |
|---|---|---|---|---|
| xanh (dump v2 thật) · xanh-thuong | 0 | 4 | 5 | thuong: 0 lệnh `put`; không `?` trong hàng hợp lệ |
| maestro 42 · pid rỗng · pid `pid-not-known` · reset hỏng | 1 | 0 | 1 | chỉ warm-up chạy; chuỗi không được chạy dưới nhãn hợp lệ |
| pid đổi · frames 0 · dump rỗng · dump chỉ một dòng · dump của pid khác · histogram ≠ frames | 1 | 0 | 5 | |
| gốc animator rỗng | 3 | 0 | 0 | **0 lệnh put**, scale không đổi |
| put 0 bị từ chối | 4 | 0 | 0 | trả về gốc |
| trả gốc bị từ chối | 5 | 4 | 5 | bảng in; scale còn 0 và được nói ra |
| INT giữa chuỗi 2 | 130 | 0 | 2 | trả gốc |

Cùng canary chạy trên **v2** (bản sao trong cây gương): 11/16 nhánh SAI — reset hỏng exit 0 với 4 hàng, dump một dòng
in `?`, `pid-not-known` qua, gốc rỗng exit 1 nhưng để `0.5/1.5/0`, put hỏng exit 0, restore hỏng exit 0 — đúng bảng
Codex. Canary Python độc lập của Codex (`docs/archive/codex/2026-09-11/technical-evidence/independent-canary.py`, chạy bản sao
trong scratchpad) trên v3: `original-empty` → 3, `put-failed` → 4, `restore-failed` → 5, `reset-failed`/`dump-malformed`/
`pid-malformed` → 1; ca `normal` của họ **đỏ vì pid giả 4242 ≠ pid 4143 trong header dump mẫu** — đó là điều kiện (5)
cắn; muốn ca ấy xanh, pidof giả phải trả pid của header (canary của tôi làm vậy).

Canary không chạm đường hạnh phúc trên máy, nên **mỗi lần đổi runner phải có một lượt thật** — mục dưới.

## Kết quả v3 — dev client, lượt thật sau khi thêm năm điều kiện

Cùng máy, cùng bốn chuỗi, cùng bundle fixture (dấu vân `claude-r1-5061b610-170628`), CPU host rảnh; mỗi chuỗi một
lượt. Cả hai chế độ **exit 0**, 8/8 hàng hợp lệ theo chín điều kiện, `scale-sau.txt` `khop=1` cả ba khoá. Số liệu:
`dev-client-v3/{thuong,reduce}/` (bảng, scale gốc/lúc đo/sau, dump đã bỏ dòng `Uptime`).

Lượt thường (scale 1/1/1, pid 8258 không đổi qua bốn chuỗi):

| chuỗi | khung | janky | p50 | p90 | p95 | p99 | khung>150ms | slow UI | slow draw | missed vsync |
|---|---|---|---|---|---|---|---|---|---|---|
| m1-doi-tab | 268 | 13.43% | 26 | 31 | 32 | 89 | 0 | 21 | 26 | 1 |
| m2-cuon-kham-pha | 924 | 3.14% | 16 | 27 | 30 | 32 | **3** | 3 | 27 | 2 |
| m3-sheet-tao | 406 | 9.11% | 20 | 31 | 32 | 36 | 0 | 3 | 30 | 0 |
| m4-back-chi-tiet | 274 | 10.22% | 28 | 32 | 32 | 65 | 0 | 6 | 26 | 0 |

Lượt Reduce Motion (scale 0/0/0 ghi và đọc lại trước khi đo; pid 9823 không đổi; trả về 1/1/1 và đọc lại đúng):

| chuỗi | khung | janky | p50 | p90 | p95 | p99 | khung>150ms | slow UI | slow draw | missed vsync |
|---|---|---|---|---|---|---|---|---|---|---|
| m1-doi-tab | 45 | 80.00% | 28 | 46 | 81 | 97 | 0 | 22 | 25 | 2 |
| m2-cuon-kham-pha | 708 | 2.26% | 16 | 27 | 28 | 32 | **2** | 1 | 16 | 0 |
| m3-sheet-tao | 62 | 24.19% | 14 | 16 | 16 | 16 | 0 | 0 | 13 | 0 |
| m4-back-chi-tiet | 59 | 38.98% | 23 | 32 | 32 | 42 | 0 | 7 | 16 | 0 |

Đọc số:
- Số khung lặp lại được giữa v2 và v3 (thường 271/907/406/273 → 268/924/406/274; reduce 45/704/63/60 → 45/708/62/59):
  cửa sổ đo khoanh đúng thao tác ở hai lượt độc lập.
- **Khác v2:** cuộn Khám phá có 3 khung (thường) và 2 khung (reduce) rơi vào bucket ≥150 ms ở lượt này, v2 là 0. Một
  lượt mỗi chuỗi không phân biệt được nhiễu máy ảo và hồi quy; ghi số thật, không làm tròn về 0. Muốn kết luận cần
  ≥ 3 lượt cùng điều kiện — chưa làm.
- pid khác nhau giữa hai chế độ vì warm-up (`_vao-app-sach`) `pm clear` và mở lại app; trong một chế độ pid không đổi.
- % janky không so được giữa thường và Reduce Motion (mẫu số khác); đọc số tuyệt đối.

## Lịch sử v2 (11/09 sáng) — phương pháp đúng, lời hứa fail-closed chưa đủ

### Cái v1 làm sai (Codex đúng)

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
- **Runner**: `scripts/do/do-motion.sh <ra> [thuong|reduce]`; canary
  `scripts/do/do-motion-canary.sh`. Số liệu lượt này: `dev-client-v2/`.
  *(Cả hai script đã chuyển từ thư mục này sang `scripts/do/` ở đợt dọn repo
  2026-09-21: `tests/test_motion_measurement_gate.py` thi hành canary, nên nó là
  hạ tầng cổng chứ không phải bằng chứng một lần. Dump green control
  `m1-doi-tab.gfxinfo.txt` đi cùng sang `scripts/do/mau/` để cổng tự đứng được.)*

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
- Khung hình **giữa** chuyển cảnh: cổng ấy nằm ở `docs/archive/claude/2026-09-11/motion-v2/` (R1), không ở bảng này.

## Chạy lại

```bash
# dev client đang nối Metro của cây này, app đã cài; runner tự warm-up bằng _vao-app-sach
scripts/do/do-motion.sh <ra> thuong
scripts/do/do-motion.sh <ra-reduce> reduce
scripts/do/do-motion-canary.sh          # tự kiểm runner, không chạm máy
# live/release (khi có HTTPS local):
FLOWS_DIR=.maestro-motion-live OTP_PHONE=<số> OTP_CODE=000000 scripts/do/do-motion.sh <ra> thuong
```

## Lịch sử (v1, 10/09) — giữ để đối chiếu, không dùng làm số

Bảng v1 ở `dev-client/{thuong,reduce}/bang.md` và `release-thu/`; lý do không dùng ở phần đầu tài liệu này.
