# Audit UI/UX app mobile RuDi, phần sau pipeline trên main: báo cáo

- Ngày: 29/09/2026. Cây đo: main `461eabf`. Audit gốc: `7ea1a7c`, tài liệu ở `docs/claude/2026-09-27/mobile-ui-audit/`.
  Nhánh ghi: `claude/busy-cray-vfmt4r`.
- MODE = **AUDIT_ONLY**: không sửa mã app. Chỉ thêm tài liệu, ảnh bằng chứng và harness đo.
- protocol_version: không áp dụng. Verdict: không có (chưa có reviewer thật; đây là báo cáo phát hiện).
- Trạng thái: **checkpoint retest 1**. Đã đo lại đủ 42 issue P1 và P2 của audit gốc trên main, thêm UI-071 (P3).
  Có 1 issue mới (UI-123). **Chưa đo:** 79 issue P3 còn lại, và năm feature mới của main (#14, #15, #21, #22, #26).
  Tất cả nằm trong ma trận dưới dạng NOT_TESTED. Mục «Checkpoint» ở cuối là nguồn sự thật về phần đã và chưa đo.

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
  - UI-113 cần bảng dev `/dev/ui-lab`, nên đo trên server dev của main (E2, fixture bật, cổng 8091). Server dev tắt
    ngay sau lượt đo.
- **Trình duyệt và cấu hình** như audit gốc:
  - Chromium 141 headless, giả lập di động, chữ thân Roboto;
  - cấu hình C1–C9 ở `report.md` §A của audit gốc.
  - Mỗi issue đo lại ở đúng cấu hình issue đó nêu. Còn lại là C1.
- **Để đối chứng UI-123**, stack của audit gốc (`7ea1a7c`, cổng 55432, 58098, 58099) được bật lại. Chỉ đọc, không ghi
  gì lên đó.

**Dữ liệu đã ghi lên stack thứ hai** (cục bộ; phần lớn có chốt để không ghi lần hai):

| Ai | Đã ghi |
|---|---|
| Kịch bản gốc F04, F05, F06 chạy lại trên main | Nhóm chat-test: một khoản chi 13.705.678đ, một đợt thu đã phát, tin nhắn đo ô soạn và link dài. moi-51 lập nhóm F06 và mời moi-52, moi-53 |
| Phần retest F06 | moi-61 được mời, vào cửa bằng OTP qua UI rồi đồng ý vào nhóm. moi-61 được nâng quản trị qua API. Vai trò của moi-51 bị bỏ bằng một chạm (đúng UI-074), rồi được trả lại qua API |
| Phần retest F07 và E5 | Bốn cặp bạn mới, mỗi cặp có chat đôi và sổ đã lập: chat-4/5, chat-6/7, chat-10/11, chat-8/9. Riêng chat-8/9 thành cặp đôi («Một đôi» từ cả hai). chat-8 chặn chat-9 rồi bỏ chặn. Tờ của chat-9 được chat-8 đồng ý qua API, thành tờ chốt và một kèo của cặp. Thêm một bản phác của chat-8, sinh ra khi đo UI-085 |
| Phần retest F03, F08, E | Nhóm chat-test: «Kèo retest 2 ngày chưa có chặng», «Kèo album retest» (một chặng Lưng Chừng Cafe, một check-in của chat-0), 3 ảnh tổng hợp trên tường. chat-0 có một bài «Bạn bè» với 2 bình luận của chat-1 |

Team Đà Lạt chỉ được đọc. Lệnh ghi duy nhất có thể chạm nó là phím mũi tên trên tay nắm thứ tự chặng (UI-036), và phím đó
không đổi gì.

### Không chạy được, và vì sao

Như audit gốc: không có Android hay iOS native (không KVM, không SDK, không macOS), không có nền bản đồ (proxy chặn tile),
không có AI thật (không khoá). Retest chỉ đo trên web. Mọi phần native của issue giữ nguyên nhãn STATIC hoặc HYPOTHESIS
như audit gốc; ma trận của phần này không thêm hàng native.

## B. Coverage thực tế (checkpoint retest 1)

Sinh bằng `tong-hop.mjs` từ sổ của main:

| Phạm vi | PASS | FAIL | BLOCKED | NOT_TESTED | N/A |
|---|---|---|---|---|---|
| Tất cả (173 hàng, web) | 20 | 68 | 1 | 84 | 0 |

| Nguồn hàng | Hàng | Kết quả |
|---|---|---|
| Retest `TC-R-UI-xxx`, đã đo (43 issue) | 47 | 44 FAIL, 2 PASS, 1 BLOCKED |
| Issue mới `TC-M-UI-123` | 1 | FAIL |
| Kịch bản gốc F00, F04, F05, F06 chạy lại trên main | 41 | 18 PASS, 23 FAIL |
| Giữ chỗ P3 chưa đo | 79 | NOT_TESTED |
| Giữ chỗ feature mới `TC-N-…` | 5 | NOT_TESTED |

Hàng của kịch bản gốc là số đo trên main. Bảng retest đọc chúng qua `retest-phan-xu.mjs`: tám issue lấy luôn hàng gốc
làm hàng retest (bảy của F04, F05 và UI-071 của F06). Một hàng F00 được phân xử bằng mắt: `TC-F00-TAB-DOI`, tab đang chọn nhìn thấy được trên
`EV-F00-TAB-ghep-a`.

**Hàng đã rút** (6 test case, lý do ghi trong sổ và ở cuối `coverage-matrix.md`):
- Bốn hàng của kịch bản F06 gốc chạy lại trên main.
  - Bước mời lại không gửi lời mời nào cho moi-53. Vì vậy ba hàng sau đó đo trên một số chưa được mời:
    `TC-F06-DUOC-MOI-VAO-CUA`, `TC-F05.S01-DONG-Y`, `TC-F06-TU-BO-QUAN-TRI`.
  - Riêng `TC-F06-MOI-LAI` không phân định được là lỗi app hay kịch bản cũ lệch với màn Mời mới.
  - UI-073 và UI-074 được đo lại đúng cách ở `r-f06`.
- `TC-R-UI-117`: thay bằng hai hàng theo hai đường, `-A` và `-B`.
- `TC-R-UI-113`: lỗi harness, đo lại (xem §E).

## C. Issues

**1 issue sau checkpoint retest 1** (mới trên main). Kết quả đo lại 122 issue cũ nằm ở `retest.md`.

| Mức | BUG | UX ISSUE | VISUAL POLISH |
|---|---|---|---|
| P2 | UI-123 | | |

### Retest: tóm tắt

| Mức | Còn | Hết | Chưa đo lại |
|---|---|---|---|
| P1 | 4 | 0 | 0 |
| P2 | 36 | 2 | 0 |
| P3 | 1 | 0 | 79 |

- **Cả bốn issue P1 còn trên main.**
  - **UI-120.** Người bị chặn vẫn gửi được tờ hẹn tới người đã chặn mình.
    - Trên main, chỉ cặp đôi mới phác và gửi được tờ. Vì vậy phép đo dựng một cặp đôi thật: chat-8/9, «Một đôi» từ
      cả hai.
    - Sau khi chat-8 chặn chat-9:
      - chat-9 vẫn chạm được «Rủ đi chơi» rồi «Gửi cho người ấy»;
      - màn chat-9 nói «Đã gửi, chờ trả lời»;
      - máy chủ ghi tờ `da_gui` ở cả hai phía;
      - màn chat-8 có tờ tới cùng nút «Ừ, hẹn Thứ Bảy 03/10».

    ![UI-120 trên main: người bị chặn gửi, người chặn nhận](evidence/EV-R-E5-ghep.jpg)
  - **UI-082.** Không phiên, link tờ giấy của cặp thật vẫn ở lại trang và hiện sổ demo «Hội bạn · Người ấy», không nhãn,
    không lối đăng nhập.
    - Phần demo đã đổi: sổ demo nay là sổ «hội bạn», không còn «Rủ đi chơi», nên không còn tới được «Đã gửi» giả.
    - Tab Tin nhắn demo vẫn không nhãn. Không màn demo nào có lối «Đăng nhập», trừ tab Cộng đồng (`TC-R-UI-082-F11`).
  - **UI-049.** Web: «Gửi cho …» vẫn báo «Kiểm tra mạng», câu lỗi ở y −2855.
  - **UI-005.** Khay «Tạo mới» nay chỉ mở được từ nút trong Lên plan: thanh tab năm cột bỏ cột «+». Back khi khay mở
    vẫn để lại một vùng inert ≥ 25% màn.
- **P2: 36 còn, 2 «đổi» và đạt tiêu chí.**
  - UI-033: «Về Lịch trình» nay ghim và thấy trọn ở C1–C4 và C8. Dòng thứ hai của lời giải thích vẫn bị nút che.
  - UI-096: màn bài bỏ hẳn nút xoá bình luận, nên không còn xoá một chạm. Nhưng cũng không còn lối xoá nào (quan sát Q3).
  - UI-006 «đổi» mà vẫn trượt: chạm «Tạo mới» hai lần nhanh thì lần hai rơi vào nền «Đóng» và khay đóng. Tiêu chí đòi
    đúng một hộp thoại.
- **UI-117 đo theo hai đường** (`TC-R-UI-117-A`, `-B`).
  - Đường cũ, từ Khám phá: Back rời hẳn app (UI-123), nên không tới được màn bị khoá. Hàng này BLOCKED.
  - Đường còn lại: Cộng đồng → Khám phá → Lên plan, mở «Tùy chọn», Back. Về Cộng đồng, trang trông bình thường nhưng có
    8 vùng inert, và chạm tab «Khám phá» không phản hồi. Lỗi gốc còn.
- **UI-084:** vẫn quay về trạng thái mời.
  - (a) Sheet chờ của người đề nghị không tự đóng, và đổi về «Đề nghị lập sổ» ngay khi thân màn đã sang sổ mở.
  - (b) Người đồng ý thấy 9 khung rAF trạng thái mời ở C1, 2 khung ở C9.
- **Đổi ở main ảnh hưởng tới cách tái hiện**, và đã ghi vào ghi chú từng hàng:
  - khay tạo mở từ Lên plan;
  - cặp chưa «Một đôi» có thân màn «Hai người cũng thành một hội»;
  - Cộng đồng thành tab đầu.

### Điểm cần đọc trước

1. **UI-120** (P1). Việc chặn không chặn được tờ hẹn của cặp đôi, trái ADR-0027 như audit gốc đã ghi. Trên main,
   route tờ giấy vẫn không kiểm chặn: `requirePairAlive` chỉ được gọi ở gửi tin và phát lại chat.
2. **UI-123** (mới, P2). Mỗi lần chuyển tab thay thế mục lịch sử trình duyệt, nên Back rời app. Nguyên nhân là
   Cộng đồng thành tab đầu, trong khi app vẫn mở ở Khám phá. Đo được trên cả hai bản, cùng một phép đo.
3. **UI-005, UI-049, UI-082** (P1) còn nguyên.
4. **Quan sát Q4** (`issues.md`). «Đã chia» của album kèo cộng mọi khoản chi của nhóm có ngày rơi vào khoảng ngày của
   kèo, không theo kèo. Chưa thành issue; cần đối chiếu ý đồ ở audit Nhật ký chuyến.

## D. Thay đổi

Không file nào trong `apps/`, `services/`, `packages/`, `parity/`, `phase0/`.

- `docs/claude/2026-09-29/mobile-ui-audit-main/`: `report.md`, `retest.md`, `issues.md`, `coverage-matrix.md`,
  `evidence-manifest.md` và `.json`, `evidence/` (16 ảnh, 2,14 MiB):
  - 10 ảnh ghép retest theo feature;
  - 5 ảnh đơn cho bốn issue P1;
  - ảnh ghép thanh tab dùng cho một hàng phân xử bằng mắt.
- `tests/qa/mobile-ui-audit/`:
  - `kich-ban/retest-main.mjs`: đo lại theo issue.
    - Các phần: `r-f00`, `r-f01`, `r-f02`, `r-f03`, `r-f06`, `r-f07`, `r-f08`, `r-f09`, `r-f10` (cần `AUDIT_BASE`
      trỏ server dev), `r-f11`, `r-e`, `r-moi`.
    - Chạy riêng một phần bằng `--chi r-f08:094`.
  - `kich-ban/retest-phan-xu.mjs`: phân xử bằng mắt; đưa hàng của kịch bản gốc vào khuôn retest; rút hàng lệch; thêm
    hàng giữ chỗ NOT_TESTED.
  - `kich-ban/retest-ghep.mjs`: ảnh ghép, và gắn chúng vào hàng.
  - `kich-ban/tham-do-lich-su-tab.mjs`: phép đo lịch sử tab, chỉ đọc.
  - `retest-bang.mjs`: sinh `retest.md`.
  - `kiem-tai-lieu.mjs`:
    - số issue liền nhau tính từ số nhỏ nhất (thư mục này bắt đầu ở UI-123);
    - canary `bang` và `muc` chạy được cả khi ô chỉ có một issue.
  - `chot-anh.mjs`: lý do ghim theo thư mục; lý do của ảnh audit gốc giữ nguyên từng chữ.
- `.repo-guard-allowlist.json`: 16 ghim mới, tổng 251.

## E. Verification

| Kiểm | Kết quả |
|---|---|
| `kiem-tai-lieu` thư mục này, cả `--canary` | identity xanh: 16 ảnh, 16 ghim khớp sha256, 119 link ảnh, 16/16 ảnh có tài liệu dẫn tới ngoài manifest, 1 issue (P2) khớp hai bảng. 5/5 canary đỏ đúng dự đoán (`sha`, `bang`, `muc`, `link`, `thua`) |
| `tu-kiem --dot-bien` | 20/20 xanh; M1–M4 đỏ đúng hàng dự đoán |
| `kiem-tai-lieu` thư mục audit gốc, sau khi sửa công cụ | identity xanh; 5/5 canary đỏ đúng dự đoán. Canary `bang` và `muc` nay rút UI-005 ở hàng P1, trước là một issue P2 |
| `retest-phan-xu.mjs` chạy hai lần | lượt đầu 95 → 112 dòng; thêm hàng giữ chỗ 161 → 245. Lượt hai: 0 dòng mới cả hai lần |
| `retest-ghep.mjs` chạy hai lần | lượt đầu gắn ảnh ghép vào 49 hàng; lượt hai 0 |
| Diff app từ `7ea1a7c` | 0 dòng trong `apps services packages parity phase0` |

**Phép đo UI-123 trên hai bản** (`tham-do-lich-su-tab.mjs`, dalat-0, C1; số sau «#» là `history.length`):

| Bản | Chuỗi | Back |
|---|---|---|
| main `461eabf` | `/explore#3` → Lên plan `/plan#3` → Tin nhắn `/messages#3` | rời app (trang gắn phiên của harness, rồi `about:blank`) |
| main `461eabf` | `/community#3` → Khám phá `/explore#4` → Lên plan `/plan#4` | `/community`, rồi rời app |
| `7ea1a7c` | `/explore#3` → Lên plan `/plan#4` → Tin nhắn `/messages#4` | `/explore`, rồi rời app |

Nguyên nhân đọc từ mã bộ định tuyến (bản fork `useLinking` của `expo-router`): chỉ push khi lịch sử của navigator dài
thêm. Chi tiết ở `issues.md`.

### Sự cố quy trình ở phần này

Mỗi sự cố đều có dòng rút trong sổ hoặc không ghi hàng nào. Không số liệu nào ở trên dựa vào bản đo lỗi.
1. **UI-113.** Bộ tìm chip «Không ảnh» so tên mà không bỏ ký tự icon, đúng bài học F06. Chip không được chạm, và mỗi
   cấu hình chờ hết 240 s.
   - Số đo C2 vẫn trùng issue gốc, vì bảng mặc định cũng không ảnh. Nhưng hàng ghi sai cách đo, nên đã rút.
   - Sửa bộ tìm, đo lại: «còn», cùng số: tim «Still Cafe» ở 294–342 trong cửa sổ 320, chỉ thấy 26px.
2. **F08.** Biến kèo đặt tên `keo` đè hàm kéo `keo`. Kịch bản dừng ở UI-094, chưa ghi hàng nào. Dữ liệu vừa đăng có chốt
   nên lượt sau không đăng lần hai.
3. **Tắt server dev.** `pkill -f "expo start …"` khớp luôn dòng lệnh của chính shell và giết nó, đúng bài học F11. Không
   ảnh hưởng số liệu; lệnh đúng phải viết mẫu dạng `[e]xpo`.
4. **UI-094.** Ảnh đầu chụp sau khi chụm, nên chỉ thấy một vùng nền phóng to. Thêm ảnh ngay lúc mở trình xem
   (`EV-R-UI-094-MO-C1`) và đo lại.
5. **F06.** Kịch bản F06 gốc chạy lại trên main bị lệch. Bốn hàng đã rút (§B).

## F. Giới hạn và rủi ro còn lại

- **Chưa đo.**
  - 79 issue P3 (danh sách ở cuối `retest.md`).
  - Năm feature mới của main: Cộng đồng (#14), Nhật ký chuyến / sổ kỷ niệm Nếp v3 (#15), hồ sơ kể chuyện và tường v2
    (#21), Rủ Đi AI trong chat (#22), hai lớp chat (#26).
  - Tất cả có hàng NOT_TESTED trong ma trận.
- **Chỉ web.** Native Android và iOS vẫn BLOCKED như audit gốc; phần native của từng issue không được đo lại.
- **Cấu hình.** Phần lớn issue chỉ đo lại ở cấu hình chính (thường là C1). UI-123 chỉ đo ở C1.
- **Chưa đo lại một số phần con:**
  - UI-051: trường hợp 4, ảnh bill máy chủ không đọc được;
  - UI-048: quyết toán demo ở C2;
  - UI-049: ca (b) và (c) đo bằng kịch bản F04 gốc (giả lập `navigator.share`), như audit gốc.
- **Đường tái hiện đã đổi.** Nhiều issue phải đi đường khác vì main đã đổi UI (khay tạo, hai lớp sổ, tab đầu). Kết
  luận «còn» ở những issue này là về cùng lỗi trên đường mới. Ghi chú từng hàng nói rõ đường nào.
- **Quan sát Q1–Q5** chưa thành issue, và chưa được đo như issue.
- **Dữ liệu.** Kết quả dựa trên stack cục bộ thứ hai với dữ liệu tổng hợp. Các lệnh ghi ở §A làm trạng thái stack khác
  stack của audit gốc.
- **Retest không chứng minh app đúng.** Nó chỉ nói tiêu chí gỡ của từng issue đạt hay chưa, trong phạm vi đã đo.

## Checkpoint

- **Đã xong (checkpoint retest 1).**
  - 42 issue P1 và P2 cùng UI-071 đã có hàng retest, và đều đã mở ảnh xem.
  - Issue mới UI-123.
  - Quan sát Q1–Q5.
  - 16 ảnh đã ghim.
- **Bước kế** (task #28): 79 issue P3.
  - Cùng khuôn `TC-R-UI-xxx`: thêm phần vào `retest-main.mjs`, hoặc chạy lại phần tương ứng của kịch bản gốc rồi đưa
    qua `retest-phan-xu.mjs`.
  - Sau đó là checkpoint retest 2.
- **Rồi các feature mới**, theo thứ tự #26 → #14 → #15 → #21 → #22.
- **Nếu bị ngắt:** phần chưa đo vẫn là NOT_TESTED trong ma trận. Không phần nào ở trên được tuyên bố là xong ngoài những
  gì liệt kê ở đây.
