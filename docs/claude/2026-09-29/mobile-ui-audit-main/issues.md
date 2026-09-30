# Issues mới trên main: audit UI/UX app mobile RuDi, phần sau pipeline

- Cây đo: main `461eabf`, bản web export production trên stack cục bộ thứ hai (Postgres 16, API Python, cửa
  trước Go, cổng khác stack của audit gốc), dữ liệu seed tổng hợp. Chromium 141 headless, giả lập di động như
  audit gốc (xem `report.md` §A).
- File này chỉ ghi issue **mới**, đánh số tiếp audit gốc (UI-001…UI-122 ở
  `docs/claude/2026-09-27/mobile-ui-audit/issues.md`). Kết quả đo lại các issue cũ ở `retest.md`.
- Checkpoint retest 2 đo lại 79 issue P3 còn lại và không thêm issue mới.
- Checkpoint N26 audit feature mới đầu tiên của main, hai lớp chat hai người «đám bạn» / «cặp đôi» (PR #660, task #26), và
  thêm UI-124…UI-131. Mục «N26» dưới đây nói phần nào đạt; mục «Mở rộng» ghi ba issue của audit gốc gặp lại ở màn mới.
- MODE = AUDIT_ONLY: không issue nào được sửa. «Trạng thái sửa» của mọi issue là *chưa sửa*; «Retest» là
  *không áp dụng*.
- Phân loại, mức và phương pháp như audit gốc: BUG · UX ISSUE · VISUAL POLISH; P0–P3; RUNTIME-WEB, STATIC,
  HYPOTHESIS.

## Tóm tắt theo mức

| Mức | Issue |
|---|---|
| P2 | UI-123, UI-124, UI-130 |
| P3 | UI-125, UI-126, UI-127, UI-128, UI-129, UI-131 |

---

## F00 Vỏ toàn cục

### UI-123 · Web: chuyển tab rồi bấm Back thì rời khỏi app, vì tab đầu nay là Cộng đồng mà app vẫn mở ở Khám phá

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (điều hướng, web) · **P2** |
| Feature / Screen / Layer | F00 · mọi màn tab · L05 thanh tab (`app/(tabs)/_layout.tsx`, `ui/RudiTabBar.tsx`) |
| Nền tảng, cấu hình | web, C1, có phiên và không phiên. Cơ chế không phụ thuộc bề rộng (thanh tab và rail cùng một navigator), nhưng chỉ C1 được đo. Native: Android với `backBehavior` «firstRoute» thì Back hệ thống đưa về tab đầu (Cộng đồng) thay vì tab trước, không thoát app; iOS không có Back (STATIC, chưa đo) |
| Điều kiện | App mở ở Khám phá: đó vẫn là đích sau đăng nhập trên main (`TC-F00-DT-dalat-0`: tới `/explore`). Hoặc mở thẳng bất kỳ tab nào không phải Cộng đồng |
| Tái hiện | 1. Mở app, tới Khám phá. 2. Chạm tab «Lên plan». 3. Bấm Back của trình duyệt (trên Android Chrome là cử chỉ back hệ thống) |
| Expected | Back về tab vừa rời (Khám phá), như ở bản `7ea1a7c`; ít nhất là không rời app |
| Actual | Chuyển tab không thêm mục lịch sử nào: có phiên `/explore#3` → Lên plan `/plan#3`; không phiên `/explore#2` → `/plan#2` (số sau «#» là `history.length`). Back rời app, tới trang trình duyệt mở trước đó (trong harness là trang gắn phiên `/favicon.ico`, hoặc `about:blank`). Chuỗi Khám phá → Lên plan → Tin nhắn cũng không thêm mục nào. Chỉ khi bắt đầu ở tab đầu thì có một mục: Cộng đồng `#3` → Khám phá `#4` → Lên plan `#4`, và Back về Cộng đồng chứ không về Khám phá. Đối chứng bản `7ea1a7c` bằng cùng phép đo: `/explore#3` → Lên plan `#4` → Tin nhắn `#4`, Back về `/explore` |
| Evidence | Không có ảnh: lỗi nằm ở lịch sử trình duyệt, không nằm trên màn hình. Hàng `TC-M-UI-123` (sổ của main); phép đo `kich-ban/tham-do-lich-su-tab.mjs` chạy trên cả hai bản (`report.md` §E) |
| Source | Commit `0ec19fa` thêm `Tabs.Screen name="community"` đứng đầu `app/(tabs)/_layout.tsx`. Bộ định tuyến (bản fork `useLinking` trong `expo-router/build/fork`) chỉ `history.push` khi lịch sử của navigator đang focus dài thêm (`historyDelta > 0`); bằng nhau thì `history.replace`. Tabs mặc định `backBehavior: "firstRoute"`, nên lịch sử tab chỉ giữ [tab đầu, tab hiện tại]: đi giữa hai tab không phải tab đầu thì độ dài không đổi, nên là replace. Ở `7ea1a7c`, tab đầu là Khám phá, trùng với đích sau đăng nhập, nên lần chuyển tab đầu tiên luôn là push |
| Hậu quả | Trên web, và Android Chrome, sau một lần chuyển tab, Back đóng app thay vì về tab trước. Thứ đang soạn dở trên màn đó mất, người dùng phải mở lại app. Đường tái hiện của UI-117 cũng vì vậy mà đổi: Back rời app trước khi tới màn bị khoá (`retest.md`, `TC-R-UI-117-A`/`-B`) |
| Đề xuất sửa | Chọn theo ý đồ sản phẩm: `backBehavior: "history"` cho Tabs, để mỗi lần chuyển tab là một mục lịch sử; hoặc đưa đích sau đăng nhập về tab đầu; hoặc để Khám phá đứng đầu như trước. Sau khi sửa, kiểm lại các flow Maestro có dùng Back |
| Tiêu chí gỡ | Mở app ở Khám phá, chạm «Lên plan», Back: về Khám phá (ít nhất là một màn của app). `tham-do-lich-su-tab.mjs` trên bản sửa cho `history.length` tăng khi chuyển tab |
| Trạng thái sửa · Retest | chưa sửa · không áp dụng |

---

## N26 Hai lớp chat hai người («đám bạn» / «cặp đôi», #660)

Đo trên bản web export của main với năm cặp của stack thứ hai: chat-0/chat-1 (đám bạn, chưa sổ), chat-4/chat-5 (đám bạn, sổ
đã mở), chat-12/chat-13 (lời đề nghị lập sổ chờ), chat-10/chat-11 (thành cặp đôi ngay trong lượt đo, qua UI), chat-8/chat-9
(cặp đôi). Trang lab `/dev/hai-lop-chat` đo trên server dev có cờ fixture. AI không có khoá: cặp nào cũng báo
`provider_unavailable`, nên câu trả lời thật, chân thẻ «· dùng gu của …» và tấm «Xem» ở màn sống là BLOCKED (tấm «Xem» đo
trên trang lab).

Đạt trong phạm vi đã đo (`coverage-matrix.md`, hàng `TC-N26-*`):
- Hợp đồng `chat-capabilities`: `cap_doi` chỉ đúng ở cặp đôi, `gu_chat` là `null` ngoài cặp đôi (ADR-0046 §8.4, ADR-0048 §3.2).
- Khay của đám bạn bốn công cụ, của cặp đôi năm công cụ, một hàng không nhãn nào bị cắt từ 320 tới 1024 (C1–C9); chữ khay
  viết cho hai người. Sticker «Cho hai người» chỉ ở cặp đôi. Gợi ý lệnh và chip «AI chưa sẵn sàng» viết cho hai bạn.
- Hàng mời lập sổ: đúng người thấy, người đề nghị không thấy, chạm tới «Xem lời đề nghị». Đề nghị và đồng ý «Một đôi» qua UI
  ghi đúng lên máy chủ; phía người đồng ý có hàng ghim và năm công cụ khi về chat.
- «Gu của hai bạn» nói Rủ Đi AI dùng gu trong chat; bật và «Bật lại cho chat» chạy đúng thứ tự thu hồi rồi đề nghị.
- Lỗi đọc `chat-capabilities` thì phòng hiện như đám bạn (đóng an toàn) và hồi lại khi vào lại.
- Trang lab: ba lỗi đã sửa ở `762d5c5` vẫn đạt (tấm «Xem» phủ cả cửa sổ, năm ô một hàng ở 360 và 320, chữ hai người);
  «Reduce Motion» chỉ hiện câu trọn. Bản export production chuyển `/dev/hai-lop-chat` về `/welcome`.

### UI-124 · Chat hai người chưa có tin: ở cửa sổ thấp, trạng thái rỗng đẩy ô soạn và khay công cụ ra ngoài đáy

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (bố cục, nội dung bị che) · **P2** |
| Feature / Screen / Layer | N26 · `/groups/[id]/chat` của chat hai người · trạng thái rỗng, L16 khay công cụ, ô soạn |
| Nền tảng, cấu hình | web, đo ở C1, C2, C4, C8 cho cả hai lớp. C8 (390×460) là proxy cửa sổ thấp, không phải phép đo bàn phím. Native: cùng cột bố cục, và màn không cuộn cả trang như web (STATIC, chưa đo) |
| Điều kiện | Chat hai người chưa có tin nào. Mọi chat hai người mới bắt đầu như vậy |
| Tái hiện | 1. Mở chat đôi chưa có tin ở 320×640, 375×667 hoặc 390×460. 2. Chạm «+» (Thêm vào cuộc trò chuyện) |
| Expected | Ô soạn và nút gửi luôn nằm trong cửa sổ, khay đóng hay mở; nhãn công cụ không rơi ra ngoài đáy |
| Actual | Cặp đôi, C8, khay đóng: ô soạn ở y 473–537 trong cửa sổ cao 460. Cả ô soạn và nút «+» nằm ngoài; trên web chỉ thấy khi cuộn cả trang (trang cao 544). Cặp đôi, C4, mở khay: ô soạn 671–735 (cửa sổ 667). Cặp đôi, C2, mở khay: nhãn cả năm công cụ ngoài đáy, ô soạn 702–766 (640). Đám bạn, C2, mở khay: ô soạn 611–675. Đám bạn, C8, mở khay: nhãn bốn công cụ ngoài đáy, ô soạn 593–657; khay đóng thì nút gửi vừa chạm đáy (459). C1 đạt ở cả hai lớp; đám bạn ở C4 đạt |
| Evidence | ![Cặp đôi C8, C4, C2 và đám bạn C8: ô soạn và công cụ ngoài đáy](evidence/EV-N26-SOAN-ghep.jpg) (hàng `TC-N26-SOAN-CHAT-RONG-BAN` C2, C8; `TC-N26-SOAN-CHAT-RONG-DOI` C4, C2, C8) |
| Source | `GroupChatLive.tsx:807–819`: trạng thái rỗng vẽ ngoài danh sách lật, là một khối trong cột (`rong: { paddingVertical: 24 }`, `:1184`) gồm Nếp 96, tiêu đề, dòng phụ và nút, không co và không cuộn. `FlatList` (`:820`) co về 0 trước, rồi khối rỗng, khay (`SoHen.tsx`, trần `tranKhay`) và ô soạn tràn khỏi cột. Cặp đôi thêm hàng ghim Tờ giấy 78dp dưới tiêu đề. Chat nhóm dùng cùng khối rỗng (STATIC, chưa đo nhóm rỗng) |
| Hậu quả | Ở màn thấp, người mở chat đôi lần đầu không thấy chỗ gõ tin đầu tiên (cặp đôi, C8), hoặc mở khay thì mất ô soạn và nhãn công cụ. Trên web còn cuộn được cả trang, đầu màn trôi theo; trên native nhiều khả năng không cuộn được (HYPOTHESIS) |
| Đề xuất sửa | Cho khối rỗng vào vùng co giãn (ví dụ `ListEmptyComponent`, hoặc cho khối co và cuộn), hoặc thu nó lại khi khay mở hay cửa sổ thấp. Ô soạn luôn là khối cuối, không bị đẩy |
| Tiêu chí gỡ | `n26-hai-lop-chat.mjs --chi soan`: ở C2, C4, C8, cả hai lớp, khay đóng và mở, nút gửi nằm trong cửa sổ, không nhãn công cụ nào ngoài đáy, trang không cuộn |
| Trạng thái sửa · Retest | chưa sửa · không áp dụng |

### UI-125 · Lớp phòng chỉ đọc lúc màn chat được focus: về chat thì hàng ghim Tờ giấy tắt rồi bật lại, còn phòng đang mở không đổi khi người kia đồng ý «Một đôi»

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (trạng thái cũ, nội dung nhảy) · **P3** |
| Feature / Screen / Layer | N26 · `/groups/[id]/chat` của cặp đôi · hàng ghim Tờ giấy, L16 khay |
| Nền tảng, cấu hình | web, C1. Native: cùng mã (`useFocusEffect`, `AppState`) (STATIC) |
| Điều kiện | (a) Cặp đôi chat-8/chat-9. (b) Cặp đám bạn chat-10/chat-11: chat-10 đề nghị «Một đôi» rồi để chat mở trong lúc chat-11 đồng ý |
| Tái hiện | (a) Trong chat cặp đôi, chạm hàng ghim Tờ giấy, rồi «Quay lại» từ không gian giấy. (b) chat-11 đồng ý trên máy của mình; chat-10 ở yên trong chat |
| Expected | (a) Về chat, hàng ghim đứng yên, nội dung không nhảy. (b) Phòng của chat-10 thành phòng cặp đôi (hàng ghim, «Tờ giấy») mà không phải rời chat, hoặc chat có một dấu báo |
| Actual | (a) Mạng nhanh: 2 khung (95–107 ms) không có hàng, dòng «Chưa mã hoá đầu cuối» ở y 64, rồi hàng hiện và dòng nhảy xuống y 142 (78dp). `chat-capabilities` trễ 800 ms: 49 khung (99–929 ms) phòng hiện như đám bạn, khay còn bốn công cụ, rồi nhảy 78dp. (b) 8 s sau khi chat-11 đồng ý, phòng của chat-10 vẫn không hàng ghim, khay bốn công cụ. Chỉ khi app về nền rồi trở lại (`visibilitychange`) mới có hàng ghim và năm công cụ |
| Evidence | ![Về chat, capabilities trễ 800 ms: khung 141 ms không có hàng ghim, khung 959 ms có](evidence/EV-N26-NHAY-tre-800-khung-C1.jpg) ![Từ đám bạn thành cặp đôi; khung cuối: chat-10 sau 8 s](evidence/EV-N26-CHUYEN-ghep.jpg) (hàng `TC-N26-NHAY-VE-CHAT`, `TC-N26-NHAY-VE-CHAT-TRE`, `TC-N26-CHUYEN-BEN-KIA`) |
| Source | `useChatAi.ts:43–47`: mỗi lần màn focus, `useFocusEffect` đặt `capabilities` về `null` rồi đọc lại; `AppState` «active» cũng đọc lại. `GroupChatLive.tsx:217`: `capDoi = nhanRieng && laCapDoi(ai.capabilities)`, nên chưa đọc xong là đám bạn và `hangGhimChat` (`so-doi-map.ts:30–47`) không trả hàng Tờ giấy. `HangToGiaySong.tsx:23–26, :40` (`nhip: 0`) ghi rõ: cặp thành cặp đôi thì không đọc lại. Không tin hay sự kiện nào của phòng báo «Một đôi» vừa bật |
| Hậu quả | Mỗi lần quay về chat cặp đôi (từ tờ giấy, hồ sơ, ảnh), nội dung nhảy 78dp sau một nhịp mạng, và một cú chạm vào đầu danh sách lúc đó có thể rơi chỗ khác. Người đề nghị «Một đôi» không biết người kia đã đồng ý cho tới khi rời chat |
| Đề xuất sửa | Giữ `capabilities` cũ trong lúc đọc lại (không đặt `null` khi focus), hoặc giữ chỗ hàng ghim khi đã biết là cặp đôi. Đọc lại sổ và capabilities khi change feed của phòng báo đổi, hoặc thêm một dòng hệ thống khi «Một đôi» bật |
| Tiêu chí gỡ | `--chi nhay`: `TC-N26-NHAY-VE-CHAT` và `-TRE` có 0 khung không hàng, dòng bảo mật ở một vị trí. `--chi chuyen:doi` trên một cặp đám bạn mới: phía người đề nghị có hàng ghim và năm công cụ trong 8 s mà không rời chat |
| Trạng thái sửa · Retest | chưa sửa · không áp dụng |

### UI-126 · Hàng mời «Một đôi» dẫn tới màn không nhắc lời đề nghị; lối trả lời nằm sau nút «Mở sổ cặp đôi»

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (lối đi, chữ hứa một việc màn không làm) · **P3** |
| Feature / Screen / Layer | N26 · chat đôi → `/groups/[id]/to-giay` (sổ của đám bạn) · hàng mời, L23 sheet «Loại sổ» |
| Nền tảng, cấu hình | web, C1. Native: cùng mã (STATIC) |
| Điều kiện | Cặp đám bạn đã lập sổ; người kia đề nghị «Một đôi», mình chưa trả lời (chat-10 đề nghị qua UI, chat-11 nhận) |
| Tái hiện | 1. chat-11 mở chat đôi: hàng «Chat Test 11 đề nghị hai bạn là «Một đôi». Mở để xem và trả lời.» 2. Chạm hàng |
| Expected | Như hàng mời lập sổ: màn tới nói có lời đề nghị và có ngay lối trả lời. Lập sổ có con dấu «Xem lời đề nghị» (`TC-N26-MOI-CHAM`, đạt) |
| Actual | Màn tới là thân sổ của đám bạn: «Hai người cũng thành một hội · Hẹn nhau như mọi hội bạn. Những tờ giấy cũ vẫn nằm ở đây.», «Rủ hội mình đi chơi», «Mở sổ cặp đôi». Không chữ nào nói có lời đề nghị. Lối trả lời là «Mở sổ cặp đôi» → sheet «Loại sổ» («Chat Test 11 đề nghị hai bạn là «Một đôi». Bạn đồng ý thì bật cho cả hai.», «Đồng ý là một đôi»). Maestro 47 đi đúng đường này. Thêm: câu «Những tờ giấy cũ vẫn nằm ở đây.» hiện cả khi cặp chưa có tờ nào |
| Evidence | ![Hàng mời, màn tới, sheet «Loại sổ», sau khi đồng ý, phía người đề nghị](evidence/EV-N26-CHUYEN-ghep.jpg) (hàng `TC-N26-MOI-BAT-DOI-DEN`; đạt: `TC-N26-MOI-BAT-DOI`, `TC-N26-CHUYEN-DE-NGHI`, `TC-N26-CHUYEN-DONG-Y`) |
| Source | `HangToGiay.tsx:120–123` (`CAU_DE_NGHI`); `KhongGianGiay.tsx:203` (lập sổ: con dấu «Xem lời đề nghị» khi có đề nghị); `:206–212` (thân đám bạn không đọc `deNghiBatDoi`); `LoaiSo.tsx` (câu đề nghị và nút đồng ý chỉ có trong sheet) |
| Hậu quả | Người được mời phải đoán rằng «Mở sổ cặp đôi», một nhãn đọc như tự mở một việc mới, là chỗ trả lời. Không đoán được thì quay về chat, hàng mời vẫn nằm đó |
| Đề xuất sửa | Khi có đề nghị `bat_doi` của người kia, thân sổ hiện câu và nút trả lời (như «Xem lời đề nghị» của lập sổ), hoặc chạm hàng mời mở thẳng sheet «Loại sổ». Câu về tờ giấy cũ chỉ hiện khi có tờ |
| Tiêu chí gỡ | `--chi chuyen:doi` trên một cặp đám bạn có sổ, chưa «Một đôi»: `TC-N26-MOI-BAT-DOI-DEN` thấy câu về lời đề nghị hoặc «Đồng ý là một đôi» ngay khi tới |
| Trạng thái sửa · Retest | chưa sửa · không áp dụng |

### UI-127 · Khoảnh khắc M6 «sổ hai người mở» không còn diễn ở bước nào của thang đồng ý

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (phản hồi đã thiết kế bị mất) · **P3** |
| Feature / Screen / Layer | N26 · `/groups/[id]/to-giay` · MO, khoảnh khắc M6 và thân sổ |
| Nền tảng, cấu hình | web. Lập sổ đo ở C1 và C9 (checkpoint retest 1); «Một đôi» đo ở C1. Native: cùng mã (STATIC) |
| Điều kiện | Người được đề nghị đồng ý ngay trên màn: lập sổ (chat-6/chat-7, chat-10/chat-11 ở checkpoint retest 1), rồi «Một đôi» (chat-10/chat-11 ở N26) |
| Tái hiện | Chạm «Đồng ý» của lập sổ, hoặc «Đồng ý là một đôi», khi đang ở không gian giấy; lấy mẫu mỗi khung rAF |
| Expected | M6 (ADR-0037, bảng khoảnh khắc: «sổ hai người mở», `keo-tab` rồi `nhay`; bìa sổ mở và con dấu «Sổ đã mở») diễn đúng một lần ở một bước của thang: lúc sổ mở, hoặc lúc thành cặp đôi. C9 thì cắt thẳng |
| Actual | Lập sổ: 0 khung có bìa M6 ở C1 và C9; thân sổ sang «Hai người cũng thành một hội». «Một đôi»: 0 khung M6 trong 3,5 s; thân sổ đi thẳng từ «hội» (10 khung) sang «Chưa có tờ nào tuần này» (191 khung) |
| Evidence | ![Khung thứ tư: vừa thành «Một đôi», không bìa M6](evidence/EV-N26-CHUYEN-ghep.jpg) (hàng `TC-N26-M6-BAT-DOI`; lập sổ: `TC-R-UI-084-B` C1, C9 ở `retest.md`) |
| Source | `KhongGianGiay.tsx:142–152`: `vuaMoSo` chỉ bật khi `lapSo` đổi false → true lúc màn đang mở. `:272–279`: bìa M6 chỉ vẽ trong nhánh `giay-trong`, nhánh này chỉ tới được khi `batDoi`. `:206–212`: sổ vừa lập của cặp chưa «Một đôi» rơi vào nhánh đám bạn. «Một đôi» bật sau đó không đổi `lapSo`, nên `vuaMoSo` không bật |
| Hậu quả | Khoảnh khắc thiết kế cho lúc sổ hai người mở không diễn trong luồng thật. Cả hai bước đồng ý kết thúc mà không có phản hồi nào ngoài việc thân sổ đổi |
| Đề xuất sửa | Chọn một bước (sổ mở, hay thành «Một đôi») cho M6. Nếu chọn «Một đôi», bật `vuaMoSo` khi `batDoi` đổi false → true |
| Tiêu chí gỡ | Lấy mẫu rAF thấy khung có «Sổ đã mở» đúng một lần ở bước đã chọn (C1), và ở C9 hiện ngay tư thế cuối |
| Trạng thái sửa · Retest | chưa sửa · không áp dụng |

### UI-128 · Chat hai người còn chữ của nhóm: cài đặt, nhãn trợ năng, trạng thái rỗng của cặp đôi, form kèo mới

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (chữ) · **P3** |
| Feature / Screen / Layer | N26 · chat hai người, L17 sheet cài đặt chat, trạng thái rỗng; F03 · `/outings/new` mở từ sổ của đám bạn |
| Nền tảng, cấu hình | web, C1. Nhãn trợ năng đọc từ DOM. Native: cùng chữ (STATIC) |
| Điều kiện | Chat hai người (đám bạn hoặc cặp đôi) |
| Tái hiện | (a) Chạm «…» ở đầu chat đôi. (b) Mở chat cặp đôi chưa có tin. (c) Ở sổ của đám bạn chat-4/chat-5, chạm «Rủ hội mình đi chơi» |
| Expected | Chữ và nhãn trợ năng của phòng hai người viết cho hai người. Đó là luật chữ mà khay đã theo (`762d5c5`: test cấm «hội», «nhóm» trong chữ khay của cặp), và lời nhắc cặp đôi cũng cấm «hội», «cả nhóm» (ADR-0046 §8.4) |
| Actual | (a) Sheet đã đổi tiêu đề thành «Cuộc trò chuyện», nhưng dòng dưới màu bong bóng là «Mặc định. Cả nhóm thấy cùng một màu.»; nhãn trợ năng của nút mở và của sheet đều là «Cài đặt nhóm». Nhãn nút được giữ có chủ ý vì hướng dẫn và Maestro 47 trích nó (`docs/claude/2026-09-28/chay-may-that.md` §6). (b) Nút duy nhất của phòng cặp đôi chưa có tin là «Rủ hội một buổi». (c) Form «Kèo mới»: «Hội mình đi đâu?», và «Chat Test 06 hiện có 2 người; bớt đi nếu chỉ một phần đi.». Tên phòng hai người là tên người kia, nên câu đọc thành «[người kia] hiện có 2 người» |
| Evidence | ![Cài đặt của chat đôi và trạng thái rỗng của cặp đôi](evidence/EV-N26-CHU-NHOM-ghep.jpg) ![Khung cuối: form «Kèo mới» của cặp đám bạn](evidence/EV-N26-RU-ghep.jpg) (hàng `TC-N26-BAN-CAI-DAT`, `TC-N26-DOI-TRONG`, `TC-N26-RU-FORM-CHU`) |
| Source | `CaiDatNhom.tsx:113` (sheet «Cài đặt nhóm»), `:177`; `GroupChatLive.tsx:782` (nút), `:816` («Rủ hội một buổi», hiện ở mọi phòng từ `c021702`); `CreateOutingLive.tsx:186`, `:226` |
| Hậu quả | Phòng hai người nói về «nhóm», «hội». Với cặp đôi, nó trái chính luật chữ khay đã theo. Người dùng trình đọc màn hình nghe «Cài đặt nhóm» trong chat một-một. Câu số người ở form đọc sai nghĩa |
| Đề xuất sửa | Nhánh chữ hai người cho ba chỗ: màu bong bóng («Hai bạn thấy cùng một màu.»), trạng thái rỗng của cặp đôi, và form kèo mới của chat hai người (tiêu đề, bỏ câu số người). Đổi nhãn trợ năng cùng lúc với Maestro 47 |
| Tiêu chí gỡ | `--chi ban:C1` và `doi:C1`: sheet cài đặt và trạng thái rỗng của chat hai người không còn «nhóm», «hội» ở chữ hay nhãn trợ năng; `--chi ru`: form «Kèo mới» của chat hai người không có câu «… hiện có N người» |
| Trạng thái sửa · Retest | chưa sửa · không áp dụng |

### UI-129 · Sheet «Gu của hai bạn» nói sai hoặc giấu trạng thái công tắc

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (chữ minh bạch về dữ liệu, câu lỗi ngoài chỗ nhìn) · **P3** |
| Feature / Screen / Layer | N26 · `/groups/[id]/to-giay` → «Cài đặt sổ» → sheet «Gu của hai bạn» (`GuHaiBan.tsx`) · L23 |
| Nền tảng, cấu hình | web, C1. Trạng thái đồng ý cũ dựng bằng `page.route` (sửa `gu_chat.cua_toi` của phản hồi thành `can_bat_lai`), vì mốc 29/09 của ADR-0048 đã qua nên không tạo được đồng ý cũ thật. Hai lệnh bật và tắt là lệnh thật; lỗi 503 chèn ở trình duyệt. Native: cùng mã (STATIC) |
| Điều kiện | Cặp đôi chat-8/chat-9, phía chat-9 |
| Tái hiện | (a) Người có đồng ý chia gu từ trước mốc mở sheet. (b) Chạm «Bật lại cho chat» khi lệnh thứ hai (đề nghị `chia_gu`) hỏng; hoặc chạm «Cho Chat Test 09 thấy gu của mình» khi lệnh hỏng |
| Expected | (a) Sheet nói một điều: gu này chưa được Rủ Đi AI dùng trong chat cho tới khi bật lại (ADR-0048 §3.1, §3.5). (b) Công tắc tắt (đóng an toàn, §3.2), và câu lỗi hiện trong sheet đang mở, nói công tắc đang tắt |
| Actual | (a) Hai dòng liền nhau nói ngược nhau: «Chat Test 09 thấy gu của bạn; Nếp dùng nó khi phác tờ, và Rủ Đi AI dùng nó trong chat của hai bạn.» rồi «Bạn bật từ trước, khi lời hứa chỉ là Nếp dùng gu khi phác tờ. Muốn Rủ Đi AI dùng gu của bạn trong chat thì bật lại.». (b) Bật lại: DELETE rồi POST (503); máy chủ báo `cua_toi = tat`; sheet sang «Gu của bạn đang để riêng.» và nút bật thường. Câu lỗi «Rủ Đi đang gặp sự cố nên chưa làm được việc này. Chưa có gì bị ghi sai, thử lại sau một chút.» nằm ở y 69, ngoài sheet, dưới lớp phủ (mờ, không chạm được). Công tắc vừa bị tắt, mà câu nói chưa có gì bị ghi. Bật thường mà hỏng: cùng câu, cùng chỗ |
| Evidence | ![Lời hứa, «Bật lại cho chat», bật lại hỏng, bật hỏng](evidence/EV-N26-GU-ghep.jpg) (hàng `TC-N26-GU-BAT-LAI-CAU`, `TC-N26-GU-BAT-LAI-LOI`, `TC-N26-GU-BAT-LOI`; đạt: `TC-N26-GU-LOI`, `TC-N26-GU-BAT`, `TC-N26-GU-BAT-LAI-HIEN`, `TC-N26-GU-BAT-LAI`) |
| Source | `gu-doi.ts:52` (`cuaToi` chỉ đọc `mine_shared`, không đọc `gu_chat`), `:86` (`CAU_BAT_LAI_CHO_CHAT`); `GuHaiBan.tsx:61–70`; `SoDoiSong.tsx:88` (`batLaiChoChat`: thu hồi rồi đề nghị); `useToGiay.ts:223` (lỗi vào `loiLenh`); `KhongGianGiay.tsx:433–437` (câu lỗi vẽ ở thân màn, dưới sheet); `src/api.ts:328` (câu 5xx dùng chung) |
| Hậu quả | Với đồng ý cũ, sheet vừa nói gu đang được AI dùng trong chat vừa nói chưa: lời minh bạch mà ADR-0048 §3.5 đòi thành mâu thuẫn. Khi bật lại hỏng nửa chừng, người bấm thấy công tắc của mình tắt, còn câu giải thích nằm sau lớp phủ và nói chưa có gì bị ghi |
| Đề xuất sửa | `cauGu` đọc thêm `gu_chat`: khi `can_bat_lai`, dòng của tôi chỉ nói phần sổ. Câu lỗi của lệnh gu vẽ trong sheet. Bật lại hỏng nửa chừng có câu riêng nói công tắc đang tắt và mời bật lại |
| Tiêu chí gỡ | `--chi gu:C1`: khi `can_bat_lai`, sheet không có câu «Rủ Đi AI dùng nó trong chat»; `TC-N26-GU-BAT-LAI-LOI` và `TC-N26-GU-BAT-LOI` có câu lỗi trong sheet, thấy được, nói công tắc đang tắt |
| Trạng thái sửa · Retest | chưa sửa · không áp dụng |

### UI-130 · «Rủ … tới đây» hiện cho mọi chat hai người, nhưng với cặp đám bạn chỗ vừa chọn không đi tới đâu

| Trường | Nội dung |
|---|---|
| Category / Severity | UX ISSUE (luồng, nút hứa một việc không làm) · **P2** |
| Feature / Screen / Layer | N26 × F02 · `/places/[id]` → `/groups/[id]/to-giay?ru=1&cho=…` → `/outings/new` |
| Nền tảng, cấu hình | web, C1. Native: cùng mã (STATIC) |
| Điều kiện | Người xem có chat đôi đang mở với một người chưa cùng bật «Một đôi» |
| Tái hiện | Ở chi tiết Tiệm Nướng Xóm Lào, chạm «Rủ Chat Test 02 tới đây» (chat-0, cặp chưa sổ) hoặc «Rủ Chat Test 06 tới đây» (chat-4, cặp đã lập sổ) |
| Expected | Chỗ vừa chọn nằm trên một lời rủ với người đó (tờ giấy, hay kèo của hai người), hoặc màn nói vì sao không và chỗ đó đi đâu. Nút chỉ hiện ở nơi nó làm được việc của nó |
| Actual | Chưa sổ: tới không gian giấy «Một chỗ cho chuyện hai mình. Mở sổ để gửi lời hẹn cho người thương…» với «Đề nghị lập sổ»; không chữ nào nhắc Tiệm Nướng Xóm Lào. Có sổ: câu «Hai bạn đang hẹn như một hội bạn. Bấm «Rủ hội mình đi chơi» rồi thêm chỗ này làm một chặng.»; «Rủ hội mình đi chơi» mở form «Kèo mới» không có quán, người dùng phải tìm lại. Nút hiện cho ba chat đôi đang mở đầu tiên, không xét «Một đôi» |
| Evidence | ![Trang quán, chưa sổ, có sổ, form kèo mới](evidence/EV-N26-RU-ghep.jpg) (hàng `TC-N26-RU-BAN-CHUA-SO`, `TC-N26-RU-BAN-CO-SO`) |
| Source | `PlaceDetailLive.tsx:210` (`doi` = ba chat đôi `active` đầu tiên, không xét `cap_doi`), `:328–335`; `KhongGianGiay.tsx:96–116` (`goiYCho`: có sổ mà chưa «Một đôi» thì nói câu hướng dẫn rồi bỏ chỗ; chưa sổ thì không làm gì), `:78` (`?ru=1` chỉ phác khi `batDoi`); `/outings/new` không nhận chỗ |
| Hậu quả | Nút hứa rủ người đó tới quán này. Với mọi cặp chưa «Một đôi», tức mọi chat đôi mới, lời hứa không thành: chỗ bị bỏ trong im lặng, hoặc người dùng phải làm lại từ đầu. Cùng họ với UI-085 (tuần đã có tờ chốt) |
| Đề xuất sửa | Chỉ hiện «Rủ … tới đây» cho cặp đôi; hoặc với đám bạn thì mở «Kèo mới» của chat đôi đó với quán đã điền làm chặng đầu |
| Tiêu chí gỡ | `--chi ru`: với cả hai cặp đám bạn, màn tới nhắc tên quán, và form kèo mới (nếu có) có quán; hoặc trang quán không còn nút «Rủ … tới đây» cho họ |
| Trạng thái sửa · Retest | chưa sửa · không áp dụng |

### UI-131 · Máy chủ vẫn phác tờ giấy cho cặp chưa «Một đôi»: luật «tờ giấy chỉ cho cặp đôi» chỉ do app giữ

| Trường | Nội dung |
|---|---|
| Category / Severity | BUG (luật nghiệp vụ, phía máy chủ) · **P3** |
| Feature / Screen / Layer | N26 · `POST /contexts/{id}/papers/draft` (LIVE-GO, Python `live`) → `/groups/[id]/to-giay` |
| Nền tảng, cấu hình | API trên stack thứ hai (RUNTIME); màn web C1. Tác động tới người dùng qua bản app cũ hơn #660 là HYPOTHESIS, không đo |
| Điều kiện | Cặp đám bạn đã lập sổ, chưa «Một đôi» (chat-6/chat-7, `cap_doi = false`) |
| Tái hiện | chat-6 gọi `POST /contexts/{id}/papers/draft`, lệnh app gọi ở «Rủ đi chơi» |
| Expected | Máy chủ từ chối: từ #660, tờ giấy, «Rủ đi chơi» và công cụ «Tờ giấy» chỉ dành cho cặp đôi (`c021702`, «tờ giấy và sticker đôi chỉ cho cặp đôi»; ADR-0046 §8.4 định nghĩa cặp đôi) |
| Actual | 201, tờ ở trạng thái `nhap`; số tờ của cặp 0 → 1. Không gian giấy của chat-6 vẽ tờ đó với «Gửi cho người ấy» và «Sửa trước khi gửi», dưới đầu «Hội bạn · Chat Test 08»; chat đôi không hàng nào. Không lối UI nào trên main tới được lệnh này cho cặp đám bạn (đã rà `?ru=1`, «Rủ đi chơi», khay, hàng ghim, «Rủ … tới đây»). Bản phác đã bỏ qua UI («Bỏ bản phác này»; trên dây là nghỉ tuần, tờ sang `nghi_tuan`) |
| Evidence | ![Không gian giấy «Hội bạn» với tờ phác, và chat đôi không hàng nào](evidence/EV-N26-Q2-ghep.jpg) (hàng `TC-N26-Q2-PHAC-API`; `TC-N26-Q2-UI` đạt: màn giữ tờ đang mở, đúng chú thích mã) |
| Source | `services/core/internal/domain/pairsteps/papers.go:131–146`: `DraftPaper` chỉ kiểm `is_group_member` và `cycle_active_or_temporary`, không kiểm `CanBatDoi`, trong khi `guChoNep` cùng file (`:787`) có kiểm. App gác ở `KhongGianGiay.tsx:78`, `:206` và `GroupChatLive.tsx:1054`. Route LIVE-GO với oracle Python `live`, nên sửa phải đi cả hai nửa, qua cổng parity |
| Hậu quả | Trong thời gian cuốn chiếu, bản app cũ trước #660 vẫn phác và gửi tờ cho cặp đám bạn. Người nhận ở bản mới thấy tờ trong không gian giấy, nhưng chat không có hàng ghim nào báo. Luật của hai lớp phòng không có chỗ cưỡng chế phía máy chủ |
| Đề xuất sửa | Thêm điều kiện «Một đôi» vào phác và gửi tờ, ở Go và oracle Python cùng lúc; hoặc ghi vào ADR rằng luật này chỉ ở app, và vì sao |
| Tiêu chí gỡ | `--chi q2` trên một cặp đám bạn có sổ: `POST …/papers/draft` trả 4xx, số tờ không đổi |
| Trạng thái sửa · Retest | chưa sửa · không áp dụng |

---

## Mở rộng issue của audit gốc (đo ở N26)

Ba issue của `docs/claude/2026-09-27/mobile-ui-audit/issues.md` gặp lại trên màn mới của #660. Không đánh số mới.

| Issue gốc | Gặp lại ở N26 | Hàng · ảnh |
|---|---|---|
| UI-001 (vùng bấm dưới 48dp) | Hàng mời lập sổ hay «Một đôi» ở chat đôi (`HangLoiDeNghi`, `paddingVertical: 6`, `HangToGiay.tsx:128`): 49dp khi chữ xuống hai dòng (C1–C3), 31dp khi một dòng (C6, rộng 736) | `TC-N26-MOI-VUNG-BAM` C6 · [EV-N26-MOI-ghep](evidence/EV-N26-MOI-ghep.jpg) |
| UI-083 (đọc sổ lỗi thì vẽ như sổ trống) | Hàng ghim Tờ giấy ở chat cặp đôi: GET notebook trả 503 thì hàng đọc «Chưa có tờ nào tuần này. Đi đâu không?», trong khi chat-8 đang có bản phác | `TC-N26-LOI-SO-503` · [EV-N26-MOI-ghep](evidence/EV-N26-MOI-ghep.jpg) |
| UI-093 (tablet: tờ và sheet trải hết bề ngang) | Sheet «Gu của hai bạn» rộng 768 ở C6 | `TC-N26-GU-LOI` C6 · [EV-N26-MOI-ghep](evidence/EV-N26-MOI-ghep.jpg) |

---

## Quan sát chưa thành issue (chờ audit feature mới)

Q1 và Q2 đã được đo ở checkpoint N26 và thành issue (cột cuối). Q3–Q5 còn chờ feature của chúng.

Những điều thấy trong lúc retest, thuộc phần main mới đổi. Chưa đủ căn cứ để gọi là lỗi, vì cần đối chiếu ý đồ
thiết kế của đúng feature đó. Mỗi điều được giao cho task audit tương ứng, không tính vào số issue.

| # | Quan sát | Căn cứ | Giao cho |
|---|---|---|---|
| Q1 | Cặp bạn bè (chưa «Một đôi») vừa lập sổ: thân màn sang «Hai người cũng thành một hội», khoảnh khắc bìa sổ M6 không còn diễn | `KhongGianGiay.tsx`: M6 (`vuaMoSo`) chỉ nằm trong nhánh `giay-trong`, nhánh này chỉ tới được khi `batDoi`; runtime: `TC-R-UI-084-B` C1/C9 không thấy khung M6 nào | **Đã đo ở N26, thành UI-127**: lúc thành «Một đôi» cũng không có khung M6 nào |
| Q2 | Máy chủ vẫn cho phác và gửi tờ giấy ở cặp chưa «Một đôi»; chỉ UI ẩn nút | `pairsteps/papers.go` `DraftPaper` không kiểm `CanBatDoi` (đọc mã, chưa gọi API trên cặp như vậy) | **Đã đo ở N26, thành UI-131**: phác trả 201 ở cặp chat-6/chat-7 |
| Q3 | Màn bài `/posts/[id]` không còn lối xoá bình luận nào, cho cả người viết lẫn tác giả bài; nhấn giữ bình luận cũng không mở gì. API `DELETE /posts/{id}/comments/{id}` vẫn còn | `TC-R-UI-096` (đổi); `BaiChiTietScreen.tsx` hàng bình luận chỉ có «Thích», «Trả lời» | #21 tường v2 |
| Q4 | Album kèo ghi «đã chia» bằng tổng phân bổ của mọi khoản chi trong nhóm có ngày rơi vào khoảng ngày của kèo, không theo kèo: «Kèo album retest» vừa tạo ghi «đã chia 13.705.678đ» của khoản chi lượt F04. Hai kèo trùng ngày sẽ cùng ghi một khoản | `repo/recap.go`: nối `expenses` theo `on_date BETWEEN outings.starts_on AND outings.ends_on`; bảng `expenses` không có cột kèo. Đọc mã Go, chưa đối chiếu oracle Python | #15 nhật ký chuyến |
| Q5 | Tab Cộng đồng khi chưa đăng nhập có nút «Đăng nhập», trong khi bốn tab demo kia và hai route demo không có (tính vào `TC-R-UI-082-F11`) | `TC-R-UI-082-F11` | #14 Cộng đồng |
