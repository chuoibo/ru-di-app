# Audit UI/UX app mobile RuDi, phần sau pipeline trên main: báo cáo

- Ngày: 29–30/09/2026. Cây đo: main `461eabf`. Audit gốc: `7ea1a7c`, tài liệu ở `docs/claude/2026-09-27/mobile-ui-audit/`.
  Nhánh ghi: `claude/busy-cray-vfmt4r`.
- MODE = **AUDIT_ONLY**: không sửa mã app. Chỉ thêm tài liệu, ảnh bằng chứng và harness đo.
- protocol_version: không áp dụng. Verdict: không có (chưa có reviewer thật; đây là báo cáo phát hiện).
- Trạng thái: **checkpoint retest 2**. Cả 122 issue của audit gốc đã được đo lại trên main (42 issue P1 và P2 ở
  checkpoint 1; 80 issue P3, gồm UI-071 đo từ checkpoint 1). Có 1 issue mới (UI-123, từ checkpoint 1).
  **Chưa đo:** năm feature mới của main (#14, #15, #21, #22, #26), nằm trong ma trận dưới dạng NOT_TESTED. Mục
  «Checkpoint» ở cuối là nguồn sự thật về phần đã và chưa đo.

Tài liệu đi kèm:
- `retest.md`: từng issue của audit gốc, trên main còn, hết hay đổi (sinh từ sổ, không sửa tay).
- `issues.md`: issue mới (từ UI-123), và các quan sát chưa thành issue.
- `coverage-matrix.md`: mọi hàng đo trên main, có đếm (sinh từ sổ).
- `evidence-manifest.md`: ảnh đã commit của phần này.
- Harness: `tests/qa/mobile-ui-audit/` (README ở đó; các script `retest-*`).

## A. Phạm vi và môi trường

### Đã chạy thật

- **Stack thứ hai**, dựng riêng cho main, không đụng stack của audit gốc:
  - Postgres 16 ở cổng 55433;
  - API Python ở 58198;
  - cửa trước Go ở 58199, build bằng Go 1.26.8, route ứng viên `ported`.
  - Cờ `MOBILE_COMMUNITY_ENABLED` bật. AI chạy engine mặc định, không có khoá, như audit gốc.
  - Dữ liệu:
    - Alembic, rồi các migration Go mới (`migrate-chat`, `-diaries`, `-community`, `-profile`);
    - danh mục 15 điểm đến, `seed:rudi` (Team Đà Lạt), seed chat 22 người.
  - Mọi dữ liệu là tổng hợp, không có dữ liệu người thật.
- **Bản web export production của main** (E1), trỏ vào cửa trước 58199. Màn chào in dấu cây `461eabf`.
  - Bảng dev `/dev/ui-lab` và `/dev/san-khau` chỉ có trên server dev của main (E2, fixture bật, cổng 8091). Dùng cho
    UI-113 ở checkpoint 1, và UI-114, UI-115, phần bảng dev của UI-027 ở checkpoint 2. Server dev tắt ngay sau mỗi lượt.
- **Trình duyệt và cấu hình** như audit gốc:
  - Chromium 141 headless, giả lập di động, chữ thân Roboto;
  - cấu hình C1–C9 ở `report.md` §A của audit gốc.
  - Mỗi issue đo lại ở đúng cấu hình issue đó nêu, trừ khi ghi chú của hàng nói khác. Còn lại là C1.
- **Để đối chứng UI-123**, stack của audit gốc (`7ea1a7c`, cổng 55432, 58098, 58099) được bật lại. Chỉ đọc, không ghi
  gì lên đó.
- **Gián đoạn.** Máy khởi động lại lần thứ sáu giữa lượt P3 F00. Lượt đó dừng ở bước gắn phiên («Failed to fetch»),
  chưa ghi hàng nào. Stack thứ hai được dựng lại bằng script của nó, rồi chạy lại lượt đó.

**Dữ liệu đã ghi lên stack thứ hai** (cục bộ, tổng hợp; phần lớn có chốt để không ghi lần hai):

| Ai | Đã ghi |
|---|---|
| Kịch bản gốc F04, F05, F06 chạy lại trên main (checkpoint 1) | Nhóm chat-test: một khoản chi 13.705.678đ, một đợt thu đã phát, tin nhắn đo ô soạn và link dài. Team Đà Lạt: 10 bill nháp của luồng chia bill (bước 2 sang 3 tạo bill nháp; không ghi sổ, không khoản chi). moi-51 lập nhóm F06 và mời moi-52, moi-53 |
| Phần retest F06 (checkpoint 1) | moi-61 được mời, vào cửa bằng OTP qua UI rồi đồng ý vào nhóm. moi-61 được nâng quản trị qua API. Vai trò của moi-51 bị bỏ bằng một chạm (đúng UI-074), rồi được trả lại qua API |
| Phần retest F07 và E5 (checkpoint 1) | Bốn cặp bạn mới, mỗi cặp có chat đôi và sổ đã lập: chat-4/5, chat-6/7, chat-10/11, chat-8/9. Riêng chat-8/9 thành cặp đôi («Một đôi» từ cả hai). chat-8 chặn chat-9 rồi bỏ chặn. Tờ của chat-9 được chat-8 đồng ý qua API, thành tờ chốt và một kèo của cặp. Thêm một bản phác của chat-8, sinh ra khi đo UI-085 |
| Phần retest F03, F08, E (checkpoint 1) | Nhóm chat-test: «Kèo retest 2 ngày chưa có chặng», «Kèo album retest» (một chặng Lưng Chừng Cafe, một check-in của chat-0), 3 ảnh tổng hợp trên tường. chat-0 có một bài «Bạn bè» với 2 bình luận của chat-1 |
| Kịch bản gốc F03 chạy lại (checkpoint 2) | Team Đà Lạt, như audit gốc: hai kèo biến thể của `seed-bien-the.mjs` («Chuyến săn mây Cầu Đất…» 12 chặng, «Kèo chưa có chặng nào»). Mỗi ca ghi chặng vào chúng thì đặt lại chặng sau đó. Bốn kèo «Kèo thử…» tạo qua form rồi xoá bằng SQL trên DB cục bộ (`donKeoThu`) |
| Kịch bản gốc F04 chạy lại (checkpoint 2) | Team Đà Lạt: một bill nháp (bước 2 sang bước 3 của luồng chia bill tạo bill nháp; luồng dừng ở gán món, không tới ghi sổ, không có khoản chi nào), một ảnh bill tổng hợp gửi đọc (máy chủ trả 503 vì không có khoá AI). «Tạo đợt thu từ sổ» bấm một lần: máy chủ từ chối, số đợt 1 → 1 |
| Kịch bản gốc F05 chạy lại (checkpoint 2) | Nhóm chat-test: hai tin của chat-1 làm chỗ nhấn giữ; một bình chọn «Kiểm thử bình chọn …» (Lẩu, Nướng), một phiếu của chat-0, đã chốt; tờ hẹn chung «Lẩu» mở từ bình chọn đó |
| Phần retest P3 F05, F06 (checkpoint 2) | Một tin của chat-1 cho sheet báo cáo; sheet mở rồi đóng, **không gửi báo cáo**. moi-51 mời lại số của moi-61 (máy chủ từ chối, 409). chat-20 gửi lời mời kết bạn qua API; chat-21 đồng ý (lần đầu bị chặn 503 ở trình duyệt), mở chat đôi; chat-20 chặn chat-21 qua UI, rồi bỏ chặn qua API sau khi đo |
| Phần retest P3 F07 (checkpoint 2) | chat-12/chat-13 thành bạn và có chat đôi qua API; chat-12 đề nghị lập sổ (lời đề nghị còn treo). Bản phác của chat-8 dời sang 08/10 rồi trả về 03/10 (đã kiểm qua API: `ngay` là 2026-10-03) |
| Phần retest P3 F08, F09 (checkpoint 2) | chat-0 có thêm một bài «Chỉ mình tôi»; chat-0 đăng một story rồi xoá nó sau khi đo. chat-2 lưu hai địa điểm. Màn «Xoá tài khoản» của chat-2 mở tới bước 2 rồi rời, **không bấm xoá** |
| Phần retest P3 E (checkpoint 2) | Nhóm F06 của moi-51: kèo «Kèo tạo từ chat retest» tạo từ khay của chat. moi-62 (tài khoản mới, đăng nhập bằng OTP qua API) được mời, đồng ý và nhắn một tin. Một lượt thăm dò ngoài harness tạo cùng tên kèo thẳng từ form, rồi kèo đó được xoá bằng SQL trên DB cục bộ trước lượt đo thật |

**Team Đà Lạt.** Report của checkpoint 1 ghi Team Đà Lạt «chỉ được đọc». Câu đó sai: kịch bản F04 gốc chạy lại ở
checkpoint 1 đã tạo 10 bill nháp trong nhóm này. Ở checkpoint 2, kịch bản F03 và F04 gốc chạy lại cũng ghi vào nó, đúng
như audit gốc từng ghi: hai kèo biến thể và kèo thử, một bill nháp, một lần bấm «Tạo đợt thu» bị từ chối.

Đếm bằng SQL chỉ đọc ngày 30/09:
- 12 bill: một của seed, 10 của checkpoint 1, một của checkpoint 2;
- vẫn đúng một khoản chi, của seed;
- không đợt thu mới, không tin nhắn mới.

### Không chạy được, và vì sao

Như audit gốc: không có Android hay iOS native (không KVM, không SDK, không macOS), không có nền bản đồ (proxy chặn tile),
không có AI thật (không khoá). Retest chỉ đo trên web. Mọi phần native của issue giữ nguyên nhãn STATIC hoặc HYPOTHESIS
như audit gốc; ma trận của phần này không thêm hàng native.

## B. Coverage thực tế (checkpoint retest 2)

Sinh bằng `tong-hop.mjs` từ sổ của main:

| Phạm vi | PASS | FAIL | BLOCKED | NOT_TESTED | N/A |
|---|---|---|---|---|---|
| Tất cả (278 hàng, web) | 77 | 192 | 2 | 5 | 2 |

| Nguồn hàng | Hàng | Kết quả |
|---|---|---|
| Retest `TC-R-UI-xxx`, đủ 122 issue (một số issue có nhiều hàng theo phần hoặc theo cấu hình) | 131 | 125 FAIL, 5 PASS, 1 BLOCKED |
| Issue mới `TC-M-UI-123` | 1 | FAIL |
| Kịch bản gốc chạy lại trên main (F00 5, F02 11, F03 50, F04 44, F05 17, F06 7, F10 7) | 141 | 72 PASS, 66 FAIL, 1 BLOCKED, 2 N/A |
| Giữ chỗ feature mới `TC-N-…` | 5 | NOT_TESTED |

- Năm hàng retest PASS:
  - hai «đổi» đạt tiêu chí của checkpoint 1 (UI-033, UI-096);
  - UI-039 ở C1: đổi, lần chạm hai rơi vào trong sheet; C6 vẫn trượt;
  - UI-092 ở C1, C8: chỉ đạt vì ngày chạy (§C);
  - phần Khám phá của UI-027 ở C9.
- Hàng BLOCKED là `TC-R-UI-117-A`.
- Hàng của kịch bản gốc là số đo trên main. Bảng retest đọc chúng qua `retest-phan-xu.mjs` (`tuHang`): hàng gốc thành hàng
  retest, status và ảnh giữ nguyên, tiêu chí là tiêu chí gỡ của issue.
- **Phân xử bằng mắt** (ảnh đã mở ra xem): khi hàng tự động không kết luận được («cần đọc ảnh ghép») hoặc kết luận sai.
  - Ở checkpoint 2:
    - `TC-F00-RESUME-CHAM` và UI-009: hàng tự động đếm nhầm nhãn năm tab thành nội dung;
    - bốn hàng MO12 của F02;
    - `TC-L10-NEN` (UI-041), `TC-F05-GHIM-TREN-NEN` (UI-070), `TC-F10-CHAY-LAI` ở C9 (UI-027);
    - UI-026, UI-028, UI-029, UI-030, UI-059;
    - ghi chú ngày của UI-092.

**Hàng đã rút** (12 test case, lý do ghi trong sổ và ở cuối `coverage-matrix.md`):
- Checkpoint 1:
  - bốn hàng của kịch bản F06 gốc chạy lại trên main: `TC-F06-DUOC-MOI-VAO-CUA`, `TC-F05.S01-DONG-Y`, `TC-F06-TU-BO-QUAN-TRI`,
    `TC-F06-MOI-LAI`;
  - `TC-R-UI-117`, thay bằng `-A` và `-B`;
  - `TC-R-UI-113`, lỗi harness.
- Checkpoint 2:
  - `TC-R-UI-007`, `TC-R-UI-013`, `TC-R-UI-102`, `TC-R-UI-105`, `TC-R-UI-118`: lỗi harness, đều đã đo lại (§E);
  - hàng giữ chỗ `TC-R-UI-027`: issue này đo theo ba phần.

## C. Issues

**1 issue sau checkpoint retest 2** (mới trên main, là UI-123; checkpoint retest 2 không thêm issue mới). Kết quả đo lại
122 issue cũ nằm ở `retest.md`.

| Mức | BUG | UX ISSUE | VISUAL POLISH |
|---|---|---|---|
| P2 | UI-123 | | |

### Retest: tóm tắt

| Mức | Còn | Hết | Chưa đo lại |
|---|---|---|---|
| P1 | 4 | 0 | 0 |
| P2 | 36 | 2 | 0 |
| P3 | 80 | 0 | 0 |

- **Cả bốn issue P1 còn trên main** (checkpoint 1):
  - **UI-120.** Người bị chặn vẫn gửi được tờ hẹn tới người đã chặn mình.
    - Trên main, chỉ cặp đôi mới phác và gửi được tờ, nên phép đo dựng một cặp đôi thật: chat-8/9, «Một đôi» từ cả hai.
    - Sau khi chat-8 chặn chat-9:
      - chat-9 vẫn chạm được «Rủ đi chơi» rồi «Gửi cho người ấy», và màn chat-9 nói «Đã gửi, chờ trả lời»;
      - máy chủ ghi tờ `da_gui` ở cả hai phía;
      - màn chat-8 có tờ tới cùng nút «Ừ, hẹn Thứ Bảy 03/10».

    ![UI-120 trên main: người bị chặn gửi, người chặn nhận](evidence/EV-R-E5-ghep.jpg)
  - **UI-082.** Không phiên, link tờ giấy của cặp thật vẫn ở lại trang và hiện sổ demo «Hội bạn · Người ấy», không nhãn,
    không lối đăng nhập.
  - **UI-049.** Web: «Gửi cho …» vẫn báo «Kiểm tra mạng», câu lỗi ở y −2855.
  - **UI-005.** Back khi khay «Tạo mới» mở vẫn để lại một vùng inert ≥ 25% màn.
- **P2: 36 còn, 2 «đổi» và đạt tiêu chí** (UI-033, UI-096). UI-006 «đổi» mà vẫn trượt. UI-117 đo theo hai đường. Chi tiết
  ở `retest.md`.
- **P3: 80 còn, 0 hết.** Mọi hàng P3 có số đo hoặc ảnh đã mở ra xem. Những chỗ đổi đáng đọc:
  - **UI-106 nặng hơn.** Màn Thành tích viết lại ở main. Ở 320px, lần đầu, thẻ «Mới mở» bẻ chữ «chân» thành «châ / n»
    trong một cột 44px.
  - **UI-092 đạt chỉ vì ngày chạy.**
    - Dải ngày của sheet sửa tờ bắt đầu từ hôm nay, nên tờ ngày 03/10 nay là lá thứ tư và thấy trọn.
    - `DeNghiSua.tsx` giống hệt bản gốc, không cuộn tới lá đang chọn.
    - Dời bản phác của chat-8 sang 08/10 (lá thứ chín) thì lá đang chọn khuất hẳn: thấy 0/56px (`TC-R-UI-092-XA`).
  - **UI-039.** Ở C1, lần chạm thứ hai rơi vào trong sheet vì sheet cao hơn (main thêm ô tìm địa điểm), nên còn một
    sheet. Ở C6, lần chạm thứ hai trúng «Đóng» ở đầu mục, sheet đóng. Công tắc `moThem` không đổi.
  - **UI-027**, ba phần:
    - Khám phá ở C9 đã hết trống: từ 118 ms sân khấu đứng trọn;
    - Nếp M5 ở C9 vẫn hai ảnh (SVG rồi Skia ở 972 ms);
    - bảng dev ở C9 vẫn trống ở 590 và 619 ms.
  - **UI-079.** Dải «Đi đâu không?» không còn ở chat đôi của cặp bạn, nhưng chỗ đó là nút «Rủ hội một buổi».
    «Đang nối lại» và «Một lời mở đầu» vẫn còn sau 20 s ở cả hai phía; phía người chặn không có câu nào nói đã chặn.
  - **UI-108.** Đường A (tab «Cá nhân» từ Khám phá) nay Back rời hẳn app (UI-123); đường B về Tin nhắn.
  - **Chữ đổi, lỗi còn:**
    - UI-029: câu lỗi mới dùng chung cho 503 lẫn mất mạng;
    - UI-061: câu giải thích mới, ở C2 vẫn 7 dòng (gốc 9);
    - UI-091: nút đổi tên thành «Lưu điều cần tránh», vẫn tắt không lý do;
    - UI-040: sheet 96% ở C8 (gốc 93%).

### Điểm cần đọc trước

1. **UI-120** (P1). Việc chặn không chặn được tờ hẹn của cặp đôi, trái ADR-0027 như audit gốc đã ghi. Trên main,
   route tờ giấy vẫn không kiểm chặn: `requirePairAlive` chỉ được gọi ở gửi tin và phát lại chat.
2. **UI-123** (mới, P2). Mỗi lần chuyển tab thay thế mục lịch sử trình duyệt, nên Back rời app. Lỗi này nay kéo theo
   UI-108 đường A.
3. **UI-005, UI-049, UI-082** (P1) còn nguyên.
4. **Không issue P3 nào đã được sửa trên main.** Hai chỗ trông như đạt (UI-092, UI-039 ở C1) là nhờ ngày chạy và bố cục
   mới, không nhờ sửa mã.
5. **Quan sát Q4** (`issues.md`). «Đã chia» của album kèo cộng mọi khoản chi của nhóm có ngày rơi vào khoảng ngày của
   kèo, không theo kèo. Ảnh `EV-R-P3-F08-ghep` (kệ album) lại cho thấy con số đó. Chưa thành issue; cần đối chiếu ý đồ ở
   audit Nhật ký chuyến.

![Retest P3 trên main, F08](evidence/EV-R-P3-F08-ghep.jpg)

## D. Thay đổi

Không file nào trong `apps/`, `services/`, `packages/`, `parity/`, `phase0/`.

- `docs/claude/2026-09-29/mobile-ui-audit-main/`: `report.md`, `retest.md`, `issues.md`, `coverage-matrix.md`,
  `evidence-manifest.md` và `.json`, `evidence/` (24 ảnh, 3,67 MiB):
  - checkpoint 1: 10 ảnh ghép retest theo feature, 5 ảnh đơn cho bốn issue P1, ảnh ghép thanh tab;
  - checkpoint 2: 8 ảnh ghép P3 `EV-R-P3-…-ghep`.
- `tests/qa/mobile-ui-audit/`:
  - `kich-ban/retest-main.mjs`: đo lại theo issue.
    - Checkpoint 1: `r-f00`, `r-f01`, `r-f02`, `r-f03`, `r-f06`, `r-f07`, `r-f08`, `r-f09`, `r-f10` (cần `AUDIT_BASE` trỏ
      server dev), `r-f11`, `r-e`, `r-moi`.
    - Checkpoint 2: `r-p3-f00`, `r-p3-f01`, `r-p3-f02`, `r-p3-f03`, `r-p3-f04`, `r-p3-f05`, `r-p3-f06`, `r-p3-f07`,
      `r-p3-f09`, `r-p3-e`, và các phần P3 của F08 nằm trong `r-f08` (`098` … `106`, dùng chung dữ liệu album).
    - Chạy riêng một phần bằng `--chi r-f08:094`.
  - `kich-ban/retest-phan-xu.mjs`:
    - phân xử bằng mắt; đưa hàng của kịch bản gốc vào khuôn retest; rút hàng lệch; thêm hàng giữ chỗ NOT_TESTED;
    - checkpoint 2 thêm hàng P3, `nhinR` và hàng theo phần.
    - Hàng giữ chỗ không còn chặn hàng đo thật (`daDo`).
  - `kich-ban/retest-ghep.mjs`: ảnh ghép, và gắn chúng vào hàng; checkpoint 2 thêm 8 ảnh ghép P3.
  - `kich-ban/f08-ky-niem.mjs`: hai id nhóm viết cứng của stack gốc thay bằng tra cứu. Trên stack gốc, tra cứu ra đúng hai
    id cũ.
  - `kich-ban/tham-do-lich-su-tab.mjs`, `retest-bang.mjs`, `kiem-tai-lieu.mjs`, `chot-anh.mjs`: như checkpoint 1.
- `.repo-guard-allowlist.json`: 8 ghim mới ở checkpoint 2, tổng 259.

## E. Verification

| Kiểm | Kết quả |
|---|---|
| `kiem-tai-lieu` thư mục này, cả `--canary` | identity xanh: 24 ảnh, 24 ghim khớp sha256, 254 link ảnh, 24/24 ảnh có tài liệu dẫn tới ngoài manifest, 1 issue (P2) khớp hai bảng. 5/5 canary đỏ đúng dự đoán (`sha`, `bang`, `muc`, `link`, `thua`) |
| `kiem-tai-lieu` thư mục audit gốc, cả `--canary` | identity xanh: 163 ảnh, 163 ghim, 521 link, 122 issue (4 P1, 38 P2, 80 P3); 5/5 canary đỏ đúng dự đoán. Thư mục gốc không đổi ở checkpoint này |
| `tu-kiem --dot-bien` | 20/20 xanh; M1–M4 đỏ đúng hàng dự đoán |
| `retest-phan-xu.mjs` chạy hai lần sau mỗi lần sửa | 411 → 452 → 452; thêm ghi chú UI-092: 452 → 453 → 453; rút giữ chỗ UI-027: 528 → 529 → 529 |
| `retest-ghep.mjs` chạy hai lần | lượt đầu gắn ảnh ghép vào 75 hàng (453 → 528); lượt hai 0. 10 ảnh ghép của checkpoint 1 dựng lại trùng từng byte với ảnh đã commit |
| Diff app từ `7ea1a7c` | 0 dòng trong `apps services packages parity phase0` |

**Phép đo UI-123 trên hai bản** (checkpoint 1; `tham-do-lich-su-tab.mjs`, dalat-0, C1; số sau «#» là `history.length`):

| Bản | Chuỗi | Back |
|---|---|---|
| main `461eabf` | `/explore#3` → Lên plan `/plan#3` → Tin nhắn `/messages#3` | rời app (trang gắn phiên của harness, rồi `about:blank`) |
| main `461eabf` | `/community#3` → Khám phá `/explore#4` → Lên plan `/plan#4` | `/community`, rồi rời app |
| `7ea1a7c` | `/explore#3` → Lên plan `/plan#4` → Tin nhắn `/messages#4` | `/explore`, rồi rời app |

### Sự cố quy trình ở phần này

Mỗi sự cố đều có dòng rút trong sổ hoặc không ghi hàng nào. Không số liệu nào ở trên dựa vào bản đo lỗi.

Checkpoint 1:
1. **UI-113.** Bộ tìm chip «Không ảnh» so tên mà không bỏ ký tự icon, nên hàng đã rút. Sửa bộ tìm, đo lại: «còn», cùng
   số.
2. **F08.** Biến kèo đặt tên `keo` đè hàm kéo `keo`. Kịch bản dừng trước khi ghi hàng nào.
3. **Tắt server dev.** `pkill -f "expo start …"` khớp luôn dòng lệnh của chính shell. Lệnh đúng viết mẫu dạng `[e]xpo`.
4. **UI-094.** Ảnh đầu chụp sau khi chụm. Thêm ảnh ngay lúc mở trình xem, rồi đo lại.
5. **F06.** Kịch bản F06 gốc chạy lại trên main bị lệch; bốn hàng đã rút.

Checkpoint 2:
6. **UI-007, UI-013.** Selector `[data-testid="create-sheet"]` trỏ vào lớp bọc toàn màn của Sheet chứ không phải panel,
   nên đo ra 100% và đỉnh 0. Rút, đo lại trên `[role="dialog"]`:
   - UI-007: 92% ở C2, 96% ở C8;
   - UI-013: khung cuối trước khi gỡ, đỉnh 710 trong 844, độ mờ 1.
7. **UI-009.** Hàng tự động xanh vì 54 ký tự đếm được là nhãn năm tab. Ảnh cho thấy nội dung trống, không chỉ báo. Phân
   xử bằng mắt thành FAIL, như audit gốc từng làm.
8. **UI-102, UI-105.** Khung ảnh đo bằng phần tử cha trực tiếp của `<img>`, nhưng expo-image bọc ảnh trong một lớp cao
   0px, nên chiều cao ra 0. Rút, đo lại trên khối đầu tiên có chiều cao: ra đúng số của audit gốc (xem trước 332×443
   `contain`, tường 340×453 `cover`; C6 702×936, C7 894×1192).
9. **UI-118, hai lần.**
   - Lần một gõ tên kèo trước khi form (một route push) sẵn sàng.
   - Lần hai: chat vẫn mount bên dưới form với các nút cùng tên, nên bộ tìm nút chạm vào «Tạo kèo» ẩn của chat. Kèo
     không được tạo.
   - Một lượt thăm dò ngoài harness chứng minh form tạo được kèo. Kèo của lượt thăm dò bị xoá bằng SQL trên DB cục bộ.
   - Đo lại bằng bộ tìm chỉ nhận nút mà tâm của nó trúng chính nó: kèo tạo được, lỗi còn.
10. **Kịch bản F08 gốc** viết cứng hai id nhóm của stack gốc. Đã thay bằng tra cứu trước khi chạy trên stack thứ hai.
11. **UI-092.** Lượt đo đầu đạt chỉ vì ngày chạy. Thêm phép đo ngày xa, dời chính bản phác của chat-8 rồi trả lại.
12. **Câu sai ở report checkpoint 1.** §A của checkpoint 1 ghi Team Đà Lạt chỉ được đọc, trong khi kịch bản F04 gốc chạy
    lại đã tạo 10 bill nháp ở đó. Không số đo nào sai theo; câu đã sửa ở §A, kèm số đếm bằng SQL chỉ đọc.

## F. Giới hạn và rủi ro còn lại

- **Chưa đo.** Năm feature mới của main: Cộng đồng (#14), Nhật ký chuyến / sổ kỷ niệm Nếp v3 (#15), hồ sơ kể chuyện và
  tường v2 (#21), Rủ Đi AI trong chat (#22), hai lớp chat (#26). Có hàng NOT_TESTED trong ma trận.
- **Chỉ web.** Native Android và iOS vẫn BLOCKED như audit gốc; phần native của từng issue không được đo lại.
- **Cấu hình.** Phần lớn issue chỉ đo lại ở cấu hình chính (thường là C1). Một số issue nêu nhiều cấu hình mà chỉ đo một:
  - UI-059 chỉ ở C1, trong khi tiêu chí nêu C1–C3;
  - UI-123 chỉ ở C1.
- **Chưa đo lại một số phần con** (checkpoint 1): UI-051 trường hợp 4; UI-048 quyết toán demo ở C2.
- **Dữ liệu chèn và phép tính tĩnh:**
  - UI-099: danh mục của stack thứ hai không có tên dài. Hai tên dài (72 ký tự, và 57 ký tự không dấu cách) được chèn vào
    bản danh mục mà trình duyệt nhận, bằng chặn mạng. Không ghi gì vào DB.
  - UI-054: đo runtime với nhóm 20 người. Ca 10 người là phép tính tĩnh của audit gốc, và hai file của nó giống hệt
    trên main.
- **Phụ thuộc ngày.** Kết luận của UI-092 dựa trên phép đo ngày xa, vì phép đo theo ngày thật đạt hay trượt tuỳ ngày chạy.
- **Đường tái hiện đã đổi.** Nhiều issue phải đi đường khác vì main đã đổi UI: khay tạo, hai lớp sổ, tab đầu, màn Thành tích
  mới. Kết luận «còn» ở những issue này là về cùng lỗi trên đường mới. Ghi chú từng hàng nói rõ đường nào.
- **Quan sát Q1–Q5** chưa thành issue, và chưa được đo như issue.
- **Dữ liệu.** Kết quả dựa trên stack cục bộ thứ hai với dữ liệu tổng hợp. Các lệnh ghi ở §A làm trạng thái stack khác
  stack của audit gốc.
- **Retest không chứng minh app đúng.** Nó chỉ nói tiêu chí gỡ của từng issue đạt hay chưa, trong phạm vi đã đo. Không
  issue nào được kết luận «hết» ngoài hai issue P2 ở checkpoint 1.

## Checkpoint

- **Đã xong.**
  - Checkpoint retest 1: 42 issue P1 và P2 cùng UI-071; issue mới UI-123; quan sát Q1–Q5; 16 ảnh.
  - Checkpoint retest 2: 79 issue P3 còn lại. Cả 122 issue của audit gốc có hàng retest; mọi ảnh ghép đã mở ra xem;
    8 ảnh mới đã ghim.
- **Bước kế:** các feature mới, theo thứ tự #26 → #14 → #15 → #21 → #22. Mỗi feature đi đủ chuỗi feature → màn → lớp →
  trạng thái như audit gốc, issue mới đánh số từ UI-124.
- **Nếu bị ngắt:** năm feature mới vẫn là NOT_TESTED trong ma trận. Không phần nào ở trên được tuyên bố là xong ngoài những
  gì liệt kê ở đây.
