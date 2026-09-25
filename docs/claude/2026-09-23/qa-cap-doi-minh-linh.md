# QA trải nghiệm cặp đôi — Minh (nam) & Linh (nữ), hai máy Android thật

> Ghi **trong lúc dùng**, theo giọng người dùng. Mỗi mục: chuyện gì xảy ra · vì sao
> tệ với một cặp đôi · gợi ý. Mức: 🔴 chặn đường · 🟠 tệ rõ · 🟡 gợn · 🔵 lỗi hình.

## Bối cảnh lượt chạy

| | |
|---|---|
| Cây | worktree sạch detached `01f58872` (`~/projects/.rudi-qa-couple`); cây chính đang có phiên khác sửa dở nên không dùng |
| Stack | `scripts/chat_e2e_stack.sh` — Postgres 16 + Python API + Go core, `MOBILE_AUTH_MODE=prod`, OTP debug `000000`, `MOBILE_CORE_CANDIDATE_ROUTES=ported` (19 route sổ đôi do Go phục vụ) |
| App | development build debug `com.lakiet.rudi` (prebuild + `assembleDebug` x86_64) trên **hai** AVD Android 15: Minh `emulator-5554`, Linh `emulator-5556`; Metro 8095, dấu vân `qa-couple-01f58872` hiện ở màn chào |
| AI | **Không có `GEMINI_API_KEY` trên máy → 0 lời gọi API trả phí.** Phần AI chỉ kiểm được trạng thái «chưa cấu hình». Nếp phác tờ giấy là template tất định, không cần khoá |
| Gõ phím | ADBKeyBoard (để gõ tiếng Việt có dấu) → bàn phím ảo KHÔNG hiện; lỗi «bàn phím che nội dung» không đo được ở lượt này |
| Persona | Minh (số tổng hợp, đuôi 101): Ăn uống · Nightlife · Game · Món local · 250–500K. Linh (số tổng hợp, đuôi 102): Cafe · Outdoor · Shopping · Món local · 100–250K. Trùng đúng một gu (Món local) — cố ý, để xem app có khai thác chỗ lệch không |

Ảnh chụp: `~/.cache/rudi-qa-couple/shots/` (ngoài repo, không commit).

---

## Nhật ký theo bước

### 1. Mở app, đăng ký

- 🟡 **Màn chào chỉ nói với hội bạn.** «Hẹn hội bạn. Rủ Đi lo phần còn lại.» Một cặp
  đôi mở app không thấy mình trong câu nào. Spec §14 nói không dán nhãn mode — đồng ý,
  nhưng ít nhất một trong bốn slide nên cho thấy «rủ một người» là chuyện app làm được.
- 🟠 **Không hỏi tên khi đăng ký.** OTP xong vào thẳng màn gu; tên mặc định «Thành viên
  mới». Hậu quả thấy ngay ở bước 3: người kia tìm mình theo số thì ra «Thành viên mới»
  — không biết có đúng người yêu mình không. Với một app xây trên *rủ một người cụ thể*,
  tên là thứ đầu tiên phải có. Gợi ý: một ô tên trước màn gu, bỏ qua được.
- 🟡 **Gu thiên hẳn về đi hội.** 8 ô: Ăn uống, Cafe, Nightlife, Món local, Outdoor,
  Shopping, Karaoke, Game. Không có gì kiểu «chỗ yên tĩnh», «xem phim», «triển lãm /
  workshop», «lãng mạn / view đẹp», «ở nhà nấu ăn». Ngân sách trần 250–500K, không có
  «trên 500K» — một buổi hẹn tối ở Đà Lạt hai người dễ vượt. Gu là nguyên liệu duy nhất
  để Nếp «giữ một điều» cho đôi, nên nghèo gu thì insight sau này cũng nghèo.
- 🔵 Nút Nếp nổi (tròn tím, mép phải ~y=1350) **đè lên nội dung** ở màn gu (cạnh ô Món
  local / dòng ngân sách) và **đè lên nút «Tạo nhóm»** ở màn chưa có nhóm.

### 2. Người mới, chưa có nhóm → ngõ cụt

- 🔴 **Người chưa có nhóm bị nhốt ở «Chưa có nhóm nào».** Chỉ có «Tạo nhóm», «Tôi có
  lời mời» (mã mời vào NHÓM), «Đăng xuất». Không tab bar, không Cá nhân, không Bạn bè,
  không «Rủ một người đi chơi». Câu trên màn: «Rủ Đi sống trong nhóm bạn bè». Với một
  cặp đôi, muốn dùng sổ hai người thì trước hết **phải lập một “nhóm” giả** — đúng
  thứ ADR-0021 nói là sai («muốn nói chuyện hai người phải tạo nhóm hai người và nhóm đó
  lẫn vào danh sách nhóm tiền»).
- 🔴 **Lời mời kết bạn không tới được người chưa có nhóm.** Minh (đã có nhóm) tìm Linh
  theo số → «Gửi lời mời» → «Đã gửi lời mời tới Thành viên mới». Máy Linh: vẫn «Chưa có
  nhóm nào», mở lại app vẫn thế, không có chỗ nào hiện lời mời kết bạn. «Tôi có lời
  mời» chỉ nhận mã nhóm. Tức là: **đường kết bạn → nhắn riêng → sổ hai người không mở
  được cho một người mới** trừ khi người kia mời họ vào một nhóm trước.
- 🟠 **Bảng Nếp trên màn chưa có nhóm:** nhãn «Chế độ trải nghiệm» dù đây là bản thật
  đăng nhập OTP; nói «Mình chưa rõ bạn đang ở đâu trong app» (đang ở màn chính);
  🔵 **ô «Hỏi Nếp một câu» bị ép còn ~0 px** (tâm x=52), nút «Vẽ» chiếm cả hàng và tràn
  mép phải. Người dùng không gõ được câu hỏi.
- 🟡 «Xem bản trải nghiệm» hiện ở màn «Lời mời» và hàng «Tài khoản — … đăng xuất bản
  trải nghiệm» ở Cá nhân. Có thể chỉ do bản debug (`__DEV__`) — cần xác nhận bản release
  không có; nếu có thì chữ «bản trải nghiệm» làm người thật tưởng đang ở bản demo.

### 3. Minh tạo nhóm, đặt tên, tìm Linh

- 🟡 Tạo nhóm xong vào thẳng Khám phá, xếp «theo 4 sở thích bạn đã chọn» — tốt. Nhưng
  nhóm hai người vẫn gọi là «hội» («Đặt tên cho hội», ví dụ «Hội cafe cuối tuần»).
- 🟠 **«Đã lưu — 1 địa điểm trên máy»** ngay sau khi cài mới, Minh chưa bấm Lưu lần
  nào. Hoặc là dữ liệu mồi, hoặc là trạng thái rò từ đâu đó — người dùng sẽ thấy lạ.
  (cần kiểm nguồn)
- 🟠 Tìm theo số ra **«Thành viên mới»** + avatar chữ «M» — không phải tên Linh, không
  ảnh. Không có cách nào xác nhận đúng người trước khi gửi (hệ quả của mục «không hỏi
  tên»).

### 4. Cửa «Rủ một người đi chơi» (Tạo mới) — trỏ vào cặp GIẢ

- 🔴 **«Rủ ai?» hiện một người không có thật.** Minh 0 bạn, chưa có sổ; màn vẫn liệt kê
  «Người ấy — Sổ hai người đang mở». Nguồn: `screens/hai-nguoi/ChonNguoi.tsx` import
  `CAP_DEMO`, `NGUOI_KIA_DEMO` từ `to-giay/fixtures-doi` và ghi cứng một hàng; comment
  trong file nói «the live build (Phase 4) lists friends and opens the direct
  conversation first» — **phần live chưa làm**. Đây là cửa vào duy nhất có tên «rủ
  một người» trong app.
- 🔴 Bấm vào → «Tờ giấy của hai mình», phụ đề **«Hai người bạn · Nhóm»** (chữ «Nhóm» lạc
  chỗ) → «Đề nghị lập sổ» → sheet «Lập sổ hai người» → bấm «Đề nghị lập sổ» **không có
  gì xảy ra**: không lỗi, không trạng thái, sheet đứng yên (bấm 3 lần). Người dùng tưởng
  app treo.
- 🟡 Trong sheet «Tạo mới», «Rủ một người đi chơi» nằm **cuối danh sách** (thứ 5, sát
  mép dưới), sau Chia hoá đơn / Đăng kỷ niệm / Đăng story. Với cặp đôi đây là việc
  chính, lại khó thấy nhất. Nút Nếp nổi đè lên mũi tên hàng «Tạo cuộc hẹn».

### 5. Mời người yêu vào nhóm — vòng luẩn quẩn cái tên

- 🔴 **Không mời được người yêu bằng tên thật.** Nhóm → Thành viên → «Mời bằng số
  điện thoại» có ô «Tên — Bạn gọi người này là gì» (ý hay cho cặp đôi). Gõ «Linh» →
  đỏ: «Người này đã được đặt tên khác trước đó, và chỉ chính họ mới đổi được. Giữ
  nguyên tên cũ, hoặc xoá "Linh" khỏi danh sách rồi thêm lại như một người mới.»
  - Không nói **tên cũ là gì** → người mời không thể «giữ nguyên tên cũ».
  - «xoá "Linh" khỏi danh sách» — không có danh sách nào có «Linh».
  - Để trống ô tên → «Đặt tên cho người bạn đang mời» (bắt buộc).
  - **Chỉ gõ đúng chuỗi «Thành viên mới» thì mới qua.** Một người dùng thật không bao
    giờ đoán ra. Nguyên nhân gốc chung với «không hỏi tên khi đăng ký»: người kia tự
    tạo tài khoản trước nên đã mang tên chữ-giữ-chỗ, và luật «chỉ chính họ đổi được»
    khoá luôn cả cái tên giữ chỗ ấy.
- 🟠 Màn xác nhận: «lời mời hiện ở **tab Tin nhắn**» — người được mời chưa có nhóm thì
  **không có tab nào** (xem mục 2).

### 6. Người được mời không thấy lời mời (đã tìm ra nguyên nhân)

- 🔴 **Lời mời vào nhóm không hiện cho người đang đăng nhập.** DB có
  `memberships.state='invited'` cho Linh; máy Linh vẫn «Chưa có nhóm nào», force-stop
  và mở lại app vẫn thế. Phải **Đăng xuất → đăng nhập lại** thì «Lời mời đang chờ —
  Minh & Linh — Đồng ý» mới hiện.
  Nguyên nhân: `screens/groups/Empty.tsx` lấy lời mời từ `phien.contexts` — bản chụp
  phiên lúc đăng nhập — và không gọi lại `GET /people/me/contexts` khi màn mở / khi
  app quay lại foreground / khi kéo xuống. Kịch bản cặp đôi điển hình (hai người cùng
  cài, một người mời người kia) **luôn** rơi vào đúng lỗi này.
- 🟠 Lời mời kết bạn (friend_requests `pending`, Minh → Linh) **hoàn toàn vô hình** với
  Linh ở màn chưa có nhóm — kể cả sau đăng nhập lại.
- 🟡 Lần đăng nhập thứ hai: **«Xin chào Thành viên mới»** — `Empty.tsx` chỉ né chữ giữ
  chỗ khi `is_new_person`, lần sau thì chào bằng đúng cái chữ mà comment nói phải tránh.
- 🟡 Hàng lời mời chỉ có «Minh & Linh · 1 thành viên» — **không nói ai mời**. Với một
  lời mời từ người yêu, «Minh rủ bạn vào…» là câu người ta muốn đọc.
- 🟠 Nhóm chat mới tạo mang nhãn **«Chưa mã hoá đầu cuối»** (ổ khoá mở) ở đầu cuộc trò
  chuyện. Cần đối chiếu với luật CLAUDE.md «Chat v2 bắt buộc E2EE, không fallback
  plaintext» — kiểm lại ở nhắn riêng (mục sau).

### 7. Nhận lời mời nhóm, kết bạn, nhắn riêng

- 🔴 **Nút Nếp nổi đè lên nút «Đồng ý» nhận lời mời** (màn chưa có nhóm). Đĩa Nếp tâm
  (975,1352) bán kính ~80px phủ nửa dưới nút «Đồng ý» (941,1284). Linh bấm «Đồng ý»
  → mở bảng Nếp. Lần hai phải bấm lệch lên mép trên của nút mới vào được nhóm. Đây là
  nút quan trọng nhất của người mới — bấm trượt thì tưởng hỏng.
- 🟠 Lời mời kết bạn của Minh chỉ thấy khi Linh tự mở Cá nhân → Bạn bè → tab «Đã
  nhận». **Không có huy hiệu/chấm** ở tab Cá nhân, ở hàng Bạn bè, ở tab «Đã nhận»;
  màn Bạn bè mở mặc định tab «Đã là bạn» (rỗng).
- 🟡 Chat nhóm «Minh & Linh» của Minh vẫn ghi **«1 thành viên»** sau khi Linh đã vào
  (không làm mới); danh sách Tin nhắn thì đã ghi «2 thành viên».
- 🟢 Nhắn riêng tốt: tiêu đề «Minh — Cuộc trò chuyện của hai mình», lời mở đầu «Một tin
  nhắn nhỏ cho Minh», hàng ghim «Tờ giấy của hai mình · Đi đâu không?», tin tới máy kia
  trong vài giây, emoji đúng, huy hiệu «1 tin chưa đọc».
- 🟡 a11y: hàng nhắn riêng trong danh sách đọc là **«Mở nhóm Linh»**.
- 🟠 **«Chưa mã hoá đầu cuối»** cả ở nhắn riêng của cặp đôi. Đúng hiện trạng đã khai
  (`docs/architecture/02-chat-go-e2ee.md`: «Chat v2 vẫn chỉ mở trong chat-lab loopback»)
  — nhãn trung thực, không phải lỗi; nhưng với tin nhắn riêng của một đôi đây là lỗ
  hổng niềm tin lớn nhất của sản phẩm, và lời hứa «Không ai ngoài hai bạn thấy sổ này»
  trong sheet lập sổ đọc cạnh ổ khoá mở thì hơi lệch.

### 8. Lập sổ hai người (trên cặp thật)

- 🟢 Minh «Đề nghị lập sổ» → sheet «Cho phép / Không kéo theo» rõ ràng, trung thực
  («Nếp không đọc tin nhắn của hai bạn», «im lặng không phải đồng ý»). Linh «Đồng ý» →
  sổ mở; máy Minh tự chuyển sang «Rủ đi chơi» không cần tải lại.
- 🟠 **Người nhận không được báo.** Sau khi Minh đề nghị, hàng ghim trên máy Linh vẫn
  «Chưa có tờ nào tuần này. Đi đâu không?» (theo dõi 20 s), không tin hệ thống trong
  chat, không chấm. Linh chỉ biết nếu tự bấm vào hàng ghim → «Xem lời đề nghị».
- 🟡 «**Người ấy** đã đề nghị» — app biết là Minh, sao không nói «Minh đề nghị lập sổ
  cho hai bạn»?
- 🟡 Lệch spec §14.1 («nghi thức lập sổ CHÍNH LÀ lời rủ đầu tiên»): bản live bắt qua
  một bước consent riêng trước khi rủ được gì — thành ra lời mở đầu với người yêu là
  một bảng điều khoản.
- 🟠 **Cả hai cùng được bảo «Tuần này bạn mở lời».** Cơ chế «gậy» (spec §4.2, lượt thuộc
  về người ÍT mở lời) không có trên bản live — `SoDoiSong.tsx` ghi thẳng
  `luotCuaToi: true`. Hệ quả người dùng: hoặc cả hai chờ nhau, hoặc cả hai cùng rủ.

### 9. Hai ô ràng buộc — ô «Đừng» KHÔNG BAO GIỜ lưu được

- 🔴 **Mất dữ liệu âm thầm.** Linh điền «Không ăn được: Hải sản, đồ cay nhiều» + «Đừng:
  Quán bar ồn ào, chỗ phải đi xa hơn 30 phút» → «Lưu hai ô của tôi» → sheet đóng, không
  lỗi. DB (`pair_shared_constraints`) chỉ có `khong_an_duoc`. Làm lại, đọc giá trị ô
  ngay trước khi lưu (có đủ chữ) → vẫn mất. Minh («Rau mùi» + «Leo dốc đi bộ nhiều»):
  cũng chỉ lưu «Rau mùi». Máy người kia hiện «Đừng: chưa ghi».
  **Nguyên nhân:** `to-giay/SoDoiSong.tsx` `datRangBuoc` bắn hai lệnh `void song.datRangBuocNay(...)`
  liên tiếp không `await`; `useToGiay.lam` giữ khoá một-lệnh (`dangLamRef`) và lệnh thứ
  hai `return false` im lặng. Vì ô «Không ăn được» luôn được gửi trước (kể cả khi không
  đổi — hoặc DELETE khi trống), ô «Đừng» **không có đường nào lưu được qua UI**.
  Sửa: chạy tuần tự (`await`), chỉ gửi ô đã đổi, và báo lỗi khi `lam` trả `false`.
- 🟡 Lưu xong không có xác nhận («Đã lưu») — sheet chỉ đóng lại.
- 🟢 Thiết kế đúng: mỗi người thấy hai ô của người kia ngay dưới ô của mình («Linh đã
  ghi · Không ăn được: …»).

### 10. Ô nhập chữ — chữ không bám góc trên-trái (user phản hồi lúc theo dõi)

- 🟠 **Ô nhiều dòng canh chữ/placeholder giữa theo chiều dọc** thay vì bám góc trên-trái
  như mọi ô soạn thảo (Material 3, iOS Notes, Messenger…). Thấy rõ ở: «Đừng» (sheet
  Hai ô ràng buộc, ảnh 42), «Vì sao đổi» (Đề nghị sửa, ảnh 45), «Giới thiệu» (Chỉnh hồ
  sơ), «Lời nhờ lập kế hoạch» (khay Tờ hẹn). Ô cao 108dp mà placeholder trôi ở nửa
  dưới → nhìn như ô bị vỡ layout, và người dùng không biết gõ vào đâu / chữ sẽ mọc từ
  đâu.
  Nguyên nhân (đọc mã, chờ hai đánh giá độc lập xác nhận): `src/rudi/ui/Field.tsx` —
  khung `fieldMultiline` có `alignItems: "flex-start", paddingTop: 13` nhưng lớp bọc
  `fieldInputWrap` vẫn `justifyContent: "center"`, và `TextInput` **không có
  `textAlignVertical: "top"`** — Android mặc định gravity `center_vertical` cho
  EditText multiline. Có `paddingTop` mà không có `paddingBottom` tương ứng.
  Sửa: với `multiline` → `textAlignVertical: "top"` trên TextInput, `justifyContent:
  "flex-start"` + `alignSelf: "stretch"` cho lớp bọc, padding dọc đối xứng.
- 🔵 **Đĩa Nếp nổi vẽ ĐÈ LÊN bottom sheet**: che mép phải ô «Đừng» (ảnh 42) và ô «Đi
  tiếp» (ảnh 45), che chữ bong bóng tin nhắn trong chat. Một vật nổi toàn cục không
  được nằm trên sheet đang mở.
  - **Đợt 4 (24/09) — đã sửa:** sheet mở thì Nếp không vẽ (đếm theo từng sheet, sheet đóng
    hết mới hiện lại, không hé câu nào lúc có sheet); welcome/đăng nhập/OTP/gu lần đầu: Nếp
    vắng (danh sách riêng `MAN_NEP_VANG`, không nới «Luật Nếp Đứng Xa Tiền»). Kiểm trên máy:
    sheet «Đề nghị sửa» 0 node Nếp, đóng sheet thì Nếp về; welcome/login/OTP 0 node.
    **Còn mở:** đĩa Nếp vẫn đè nội dung tĩnh ở mép phải trên màn không có sheet (dòng hướng
    dẫn ở màn kèo, bong bóng tin của mình) — giới hạn của thiết kế ray dọc (ADR-0033).
- 🟡 Ngày nhập là chữ ISO thô «2026-09-26», giờ là chữ tự do «18:30» — không có bộ chọn
  ngày/giờ; hai ô cùng tên «Giờ» (a11y đọc trùng).

### 11. Tờ giấy → kèo: vòng «cùng đi» bị đứt

- 🟢 **Đây là phần làm tốt nhất của app.** Bản phác → sửa → gửi → «Linh chưa xem» →
  Linh thấy tờ trong ~3 s với «MINH GỬI», nút «Ừ, hẹn Thứ Bảy 26/09» / «Đề nghị sửa» →
  Linh đổi giờ, máy Minh hiện **diff** «Giờ chỗ chính: 18:30 → 19:00 · Vì sao sửa: Em
  tan làm 18h…» → Minh «Ừ» → «ĐÃ CHỐT», máy chủ sinh đúng một kèo `headcount=2`.
  Vật liệu tờ giấy (góc gấp coral, con dấu) có bản sắc thật.
- 🔴 **Bản phác của Nếp rỗng nghĩa:** «18:30 · Ăn tối · Thứ Bảy 26/09». Không chỗ,
  không lý do, không dùng gu chung (Món local), không nhắc Linh không ăn hải sản, không
  dùng «Thích cafe yên tĩnh» trong hồ sơ Linh. Lời hứa lõi «Nếp giữ một điều» chưa có
  gì để giữ — đây là câu trả lời thẳng cho câu hỏi «insight cho cặp đôi có sâu không»:
  **hiện là 0**, người dùng tự viết hết.
  - **Đợt 4 (24/09) — bước đầu:** bản phác đọc tờ đã chốt của chu kỳ đang mở (giờ quen, chỗ
    đã chọn) và danh mục cùng thành phố/cùng loại, tránh chỗ trùng chữ hai ô ràng buộc, nói
    rõ chưa kiểm món. Trên máy: «19:15 · Ăn tối · Tiệm Nướng Xóm Lào — Lần trước hai bạn hẹn
    19:15 ở Lẩu Gà Lá É Tao Ngộ; Nếp giữ giờ đó…». Tờ giấy giờ hiện tên quán dưới chặng.
    **Còn mở:** gu hai người chỉ vào sau ADR + consent `chia_gu` (Đợt 7); tờ lời rủ trước khi
    lập sổ không thành lịch sử (đúng ADR-0027 §4) nên tuần đầu vẫn là bản mẫu.
- 🟠 Sửa **bản nháp của chính mình** lại mở sheet «Đề nghị sửa» với câu «Sửa gì thì
  thành phiên bản 2. Người ấy sẽ thấy đúng chỗ đổi» và nút **«Gửi phiên bản 2»** —
  nhưng bấm xong KHÔNG gửi gì (DB vẫn `nhap`, version 1). Nhãn nói sai việc nút làm.
- 🟡 «Chỗ chính» là chữ tự do — không chọn được quán từ Khám phá (dù có sẵn «Lẩu Gà Lá
  É Tao Ngộ» với địa chỉ, giờ mở). Không kiểm giờ: chặng hai 20:00 sau lẩu 19:00.
- 🔴 **Kèo sinh ra từ tờ giấy vô hình.** DB: outing «**Tờ lời rủ 26/09**» trong context
  `pair`, `itinerary_days=[]`, 0 `outing_stops`, budget 0 — **hai chặng của tờ không
  được chép sang kèo**, tên kèo là chữ kỹ thuật. Tab «Lên plan» chỉ hiện «Kèo của Minh &
  Linh» (nhóm giả): «Chưa có kèo nào». «Thêm vào kèo» từ trang quán: «Nhóm chưa có kèo
  nào». Hồ sơ đếm «1 kèo» nhưng bấm vào không đi đâu. Màn tờ đã chốt chỉ còn «Huỷ buổi
  này», không có đường sang kèo. → Chặng «CÙNG ĐI» và «GIỮ MỘT ĐIỀU» của vòng trải
  nghiệm (spec §2) không làm được.
  - **Đợt 3 (24/09) — đã sửa phần máy chủ + đường vào:** chặng tờ nhận id danh mục (slug);
    chốt ghi chặng đã đồng ý thành `outing_stops` cùng transaction, tên kèo theo quán/việc
    («Lẩu gà lá é · 26/09»); tờ đã chốt có nút «Xem kèo» (`/outings/{id}?ctx={pair}`); tab
    «Lên plan» thêm mục «Hẹn của hai bạn»; kèo có «Chia bill buổi này» chia trong đúng sổ
    của kèo; `/settlements/{id}` đọc đúng sổ. **Còn mở:** chọn «Chỗ chính» từ Khám phá và
    kiểm giờ chặng (làm cùng bộ chọn ngày/giờ của `DeNghiSua`, Đợt 5); «Thêm vào kèo» chọn
    kèo của pair; mốc «đã hẹn» trong DM (dựng ở client, E2EE).
- 🟡 Chốt xong không có dấu mốc nào trong chat (một dòng «Minh & Linh hẹn Thứ Bảy
  26/09 · Lẩu Gà Lá É» sẽ là khoảnh khắc đáng nhớ nhất của cả luồng).
- 🟠 Khay «+» của nhắn riêng có «Tờ hẹn» (AI lập kế hoạch) song song với «Tờ giấy của
  hai mình» (ghim) — hai khái niệm tên gần giống cho cùng một việc; trong DM nó còn
  hỏi «Bạn muốn rủ **hội** đi đâu?». Không khoá AI: «AI chưa sẵn sàng. Bạn vẫn có thể
  tự tạo kèo.» (trung thực).
- 🟡 Sticker toàn bộ chung (Đi thôi!, Kẹt xe, Trả tiền nè…) — không có gì cho một đôi.
- 🟡 Khám phá cá nhân hoá theo từng người (Linh: cafe lên đầu; Minh: nướng lên đầu) —
  tốt; nhưng không có góc «hợp cả hai» và không có nút «Rủ Minh tới đây» ở trang quán
  để đưa quán vào tờ giấy, dù tag «Nhẹ nhàng · View đẹp» khớp đúng điều Linh ghi.

### 12. Chia hoá đơn — trông như form CRUD, không như tờ hoá đơn (user phản hồi)

Bối cảnh: Tạo mới → Chia hoá đơn chạy trong **nhóm «Minh & Linh»**, không phải sổ hai
người và không gắn với kèo vừa chốt (vì kèo nằm trong `pair`).

- 🟠 **Đẹp/câu chuyện:** màn «Xem lại hoá đơn» là một chồng form: mỗi món bung ba ô với
  nhãn «Món / Số lượng / Thành tiền» lặp lại, thùng rác «Bỏ món này» lặp lại. Không có
  gì gợi một **tờ hoá đơn** (dòng món, chấm dẫn, tổng ở đáy) dù cả app dựng trên vật
  liệu giấy — tờ lời rủ có góc gấp, con dấu; tờ hoá đơn thì không. Không nhắc buổi đi
  («Lẩu Gà Lá É · Thứ Bảy»), không mặt hai người. Trông như «một AI làm», đúng như user
  nói. Gợi ý: danh sách món dạng dòng hoá đơn, chạm để sửa tại chỗ; header là buổi đi.
- 🔴 **Nghĩa của con số mâu thuẫn:** tiêu đề món ghi «12 × 70.000đ» (đọc như đơn giá ×
  số lượng = 840.000đ) nhưng tổng «2 món · 420.000đ» coi 70.000 là **thành tiền**. Người
  dùng không biết ô «Thành tiền» là giá một cái hay cả dòng. (Số 12 do driver QA gõ
  lỡ — nhưng chính cái lỡ đó cho thấy app không hỏi lại một ca cao nóng 12 ly cho hai
  người, và dòng tóm tắt hiểu sai.)
- 🟡 Ô tiền hiện số thô «350000» ngay dưới dòng «350.000đ»; không tự chèn dấu chấm,
  không hậu tố «đ».
- 🟡 Câu «Bạn nhập tay 2 món. **Máy chủ** chưa đọc ảnh nào.» — chữ kỹ thuật.

(bổ sung mục 12)
- 🔴 **Ô số lượng không xoá được để gõ lại.** «12» → Backspace còn «1» → Backspace nữa
  vẫn «1» (rỗng bị từ chối) → gõ «2» thành «12». Người dùng thật không có đường sửa
  số lượng trừ khi đặt con trỏ lên đầu và xoá tới. `chia-bill/ChiaBillLive.tsx:413-417`:
  `value={String(line.quantity)}` + `setQuantity` từ chối chuỗi rỗng → ô controlled
  không bao giờ rỗng. Sửa: giữ chuỗi nháp cục bộ, kiểm khi blur/next.
- 🟠 Bước 3 «Ai dùng món nào?» với **2 người** vẫn có ba nút «Cả nhóm / Bỏ hết / Như món
  trên» — nhiều hơn chính hai chip người; chip người không có avatar; «Cả nhóm» sai
  ngữ cảnh đôi. Câu «Máy chủ giữ bản gán này».
- 🟠 Bước 4 tiêu đề màn là **«Máy chủ chia»**, «Máy chủ chia 2 món…», «máy chủ tự kiểm
  tổng khớp…». Tiền chia ĐÚNG (210.000đ + 210.000đ = 420.000đ ✓ luật 2).
- 🔴 **Câu chuyện sai với một đôi:** Quyết toán hiện «**Linh → Minh 210.000đ** · Đề xuất,
  chưa phải nghĩa vụ», «Đợt thu», «Máy chủ chứng minh đây là danh sách chuyển ngắn
  nhất». Sau buổi hẹn, app biến bạn gái thành con nợ bằng ngôn ngữ kế toán. `ban-tinh.ts`
  đã khai `tienHien: "chi-tieu-chung"` cho sổ đôi — **không màn nào đọc trường đó**.
- 🔵 «**Chưa có chuyến**» (câu trạng thái rỗng) in bằng font display teal cỡ lớn ở vị trí
  của một con số — trông như giá trị, không như trạng thái trống. «Nhóm chưa có kèo
  nào…» dù hai người VỪA chốt một kèo (nằm trong `pair`).

### 13. «Một đôi» — lời tỏ bày thân mật nhất, UX mỏng nhất, và TRẠNG THÁI LỆCH

- 🟠 Linh: Cài đặt sổ → Loại sổ → chạm «Một đôi» trên segmented control = **gửi đề nghị
  ngay**, không sheet giải thích (sheet `BatMotDoi` «Cho phép / Không kéo theo» có trong
  `hai-nguoi/DongYBac.tsx` nhưng không được mở ở đây). Lập sổ có nghi thức; bước thân
  mật hơn nhiều lại là một cú chạm dễ trượt.
- 🔴 **Người nhận không biết.** Máy Minh: hàng ghim, màn tờ, chat, Tin nhắn — không dấu
  hiệu nào. Chỉ thấy nếu tự vào Cài đặt sổ → Loại sổ.
- 🔴 **Sai góc nhìn:** ở đó Minh (người NHẬN) đọc «Đã đề nghị «Một đôi». Chờ người ấy đồng
  ý trên máy của người ấy.» + con dấu «ĐÃ ĐỀ NGHỊ» — như thể chính mình đề nghị; không
  có nút «Đồng ý». `LoaiSo.tsx` không đọc `cuaToi` như sheet lập sổ.
- 🔴 **Consent lệch giữa UI và máy chủ.** Minh bấm «Một đôi» (đường duy nhất) → UI hai máy
  đều «Đang là một đôi. Nếp nói chuyện với hai bạn như với một đôi.», tiêu đề «Một đôi ·
  Linh». DB: **hai** `pair_consent_proposals` `bat_doi` riêng (Linh đề nghị, Minh đề
  nghị), mỗi cái chỉ có consent của người đề nghị, `completed_at` NULL,
  `active_couple_members` = **0**. Tức máy chủ chưa bao giờ bật đôi.
  Nguyên nhân: client `LoaiSo` → `deNghiBatDoi` → `xinBac("bat_doi")` = **tạo đề nghị
  mới** thay vì `dongYDeNghi(<id của Linh>)`; `to-giay/so-doi-map.ts:33`
  `caHaiDongY` gộp consent **theo purpose** chứ không theo cùng một đề nghị; máy chủ cho
  hai đề nghị cùng purpose mở song song. Loại blocker: **consent** (charter) — UI tuyên
  bố một bậc đồng ý mà máy chủ không có.
- 🟡 Bật xong, thứ đổi duy nhất nhìn thấy là phụ đề «Một đôi · Linh». Không vai «Người lo /
  Người chấm» (sheet hứa «Mở đường cho vai…»), Nếp không làm gì khác — đúng như
  `ban-tinh.ts` khai `coVai`, `nhip`, `nepDuocLam` nhưng không màn nào đọc.

### 14. Nếp trong sổ đôi, responsive, kỷ niệm, đóng sổ, hồ sơ

- 🔴 **Nếp mù ngữ cảnh ngay trong sổ đôi.** Mở Nếp trong nhắn riêng của một đôi vừa chốt
  hẹn: «Mình chưa rõ bạn đang ở đâu trong app.»
  **Đợt 4 (24/09) — đã sửa:** Tin nhắn, nhắn riêng/chat nhóm, tờ giấy, Lên plan, Cá nhân khai
  phiếu; phiếu khai theo focus và gắn đường dẫn (trước đây quay lại màn dưới stack thì mất
  phiếu); câu «Mình đang thấy» đọc như câu («sổ một đôi · còn 2 ngày nữa · 1 chặng», không còn
  «soNguoi: 2»); nhãn «Mình đang thấy» trước đây trắng trên nền tím nhạt (dùng nhầm `aiInk`). Ô «Hỏi Nếp một câu» vẫn **0 px** —
  nguyên nhân (đánh giá A): `nep/NepBang.tsx:152-172` hai `RudiButton compact` không
  truyền `full={false}`, mà `RudiButton` mặc định `full=true` → `width:"100%"` +
  `flexShrink:0` (`ui.tsx:421, 1065-1066`) bóp `TextInput` `flex:1` về 0. Hỏi Nếp
  **không dùng được ở bất kỳ đâu**.
- 🟡 Đổi dark mode / cỡ chữ → app **mất chỗ đang đứng** (đang ở nhắn riêng → nhảy về Khám
  phá): Activity dựng lại khi `fontScale` đổi.
- 🟢 Dark + chữ 1.3: tờ giấy, chat reflow tốt, không chữ vỡ. 🟡 Khám phá ở 1.3: nút AI rơi
  xuống hàng riêng dưới ô tìm; giá bị cắt «mỗi n…».
- 🔵 Ở dark, **chất giấy biến mất** — tờ lời rủ chỉ còn là một thẻ navy; dòng «Vì:» thụt
  lệch lề thẻ. Sau khi chốt, nửa dưới màn tờ **trống trơn** chỉ còn «Huỷ buổi này» đỏ —
  đỉnh cảm xúc của cả luồng (hẹn được nhau) không có gì: không đếm ngày, không bản đồ hai
  chặng, không «thêm một điều muốn nhớ».
- 🔴 **Đăng kỷ niệm hỏng trên cả hai máy**: «Không nối được máy chủ. Kiểm tra mạng rồi
  thử lại.» (chat cùng lúc vẫn chạy). Request không tới Python; route `POST
  /contexts/{id}/photos` là Go-owned và trả 401 khi gọi không phiên từ host. **Chưa loại
  trừ được nguyên nhân do stack test** (không kiểm được bằng phiên thật từ host) — cần một
  lượt `scripts/check_media_persists.sh` / flow Maestro 32 để phân xử.
- 🔴 **Thử lại chắc chắn hỏng:** lần hai báo «Không mở được tấm ảnh này», logcat
  `cache/ImagePicker/…jpeg: ENOENT`. `ky-niem/chon-anh.ts:40-47` — `finally` của
  `nenVaDung` **xoá ảnh gốc đã chọn kể cả khi upload hỏng**, trong khi màn vẫn hiện ảnh
  xem trước (URI đã chết). Sửa: chỉ dọn file tạm khi thành công hoặc khi rời màn.
- 🟡 Kỷ niệm đi lên **tường nhóm «Minh & Linh»**, không gắn vào tờ đã chốt (spec §2 «giữ
  một điều» = thêm một dòng vào CHÍNH tờ giấy). Ảnh dọc nằm trong khung ngang có hai dải
  xám — trông như thẻ đang tải, không như một bản in. Chọn ảnh xong, nút «Chia sẻ» bị
  đẩy xuống dưới mép màn, phải cuộn mới thấy.
- 🟢 Xem trước đóng sổ trung thực và rõ hậu quả. Nhưng nó **tự xác nhận lỗi mục 13**:
  «· 2 lời đề nghị đang chờ sẽ huỷ» ngay dưới tiêu đề «Một đôi · Linh». 🟡 «Đóng là
  đóng. Tờ chưa mở thì thôi; không gì sống lại.» — câu quá lạnh cho một khoảnh khắc
  có thể là chia tay.
- 🟡 Hồ sơ Minh dưới mắt Linh: nhãn «**Bạn bè**» dù đang «Một đôi»; không một dấu vết
  chung (buổi hẹn đã chốt, gu trùng «Món local», kỷ niệm).

---

## Tổng kết — hai người dùng chấm

**Minh (người hay phải lo):** «Tờ giấy qua lại thì đã. Nhưng để tới được đó tôi phải lập
một cái nhóm giả, đoán tên người yêu là "Thành viên mới", và Nếp — cái đáng lẽ nghĩ hộ
tôi — chỉ viết "Ăn tối". Hẹn xong thì kèo biến mất, bill lại bảo bạn gái nợ tôi.»

**Linh:** «Tôi không thấy lời mời của anh ấy cho tới khi đăng xuất vào lại. Tôi ghi
"đừng rủ chỗ bar ồn" thì app không lưu. Tôi bấm "Một đôi" thì anh ấy không hề biết.»

| Mặt | Điểm /10 | Một câu |
|---|---|---|
| Chức năng lõi tờ giấy (gửi / sửa / diff / chốt) | **8** | Phần tốt nhất, realtime ~3 s, phiên bản rõ |
| Vào được sổ hai người (onboarding → kết bạn → sổ) | **2** | 4 blocker liên tiếp; không người thật nào tự qua được |
| Độ ổn định | **4** | Mất dữ liệu âm thầm («Đừng»), consent lệch, upload hỏng, retry chết |
| Insight / khai thác dữ liệu cặp đôi | **1** | Nếp phác «18:30 · Ăn tối»; gu, ràng buộc, hồ sơ không được dùng; vai / nhịp / chi tiêu chung chỉ khai trong `ban-tinh.ts` |
| Câu chuyện & cảm xúc | **3** | Vật liệu giấy có bản sắc ở tờ lời rủ; mọi thứ sau «Đã chốt» là form + kế toán |
| Thẩm mỹ UI | **6** | Hệ màu, chữ, con dấu tốt; ô nhiều dòng lệch, Nếp đè sheet, hoá đơn như form CRUD, dark mất chất giấy |
| Responsive (dark, chữ 1.3) | **7** | Reflow ổn; mất vị trí khi đổi cấu hình; vài dòng cắt |

### Ưu tiên sửa (theo thứ tự chặn người dùng)

1. **Đường vào cho người mới** — tab/Bạn bè/lời mời phải thấy được khi chưa có nhóm;
   `Empty.tsx` đọc lại `/people/me/contexts`; hỏi tên lúc đăng ký; bỏ vòng luẩn quẩn
   tên-giữ-chỗ khi mời. (mục 2, 5, 6)
2. **`ChonNguoi.tsx` trỏ vào cặp demo** — liệt kê bạn thật / mở DM. (mục 4)
3. **Ô «Đừng» không bao giờ lưu** — tuần tự hoá hai lệnh ghi. (mục 9)
4. **Consent «Một đôi»** — người nhận phải được báo, màn phải là «Đồng ý» đề nghị của
   người kia (`dongYDeNghi`), `caHaiDongY` phải theo cùng đề nghị, máy chủ không cho hai
   đề nghị cùng purpose mở song song. (mục 13)
5. **Kèo từ tờ giấy** — chép chặng sang kèo, đặt tên theo chỗ, cho thấy kèo `pair` ở Lên
   plan / từ màn đã chốt. (mục 11)
6. **Nếp**: ô hỏi 0 px (`NepBang.tsx`), đĩa nổi đè sheet/nút «Đồng ý», ngữ cảnh sổ đôi,
   và bản phác có dùng gu chung + ràng buộc. (mục 7, 10, 14)
7. **`Field` multiline** — `textAlignVertical: "top"` (12 ô, 0 chỗ nào trong cây có thuộc
   tính này). (mục 10)
8. Tiền trong sổ đôi đọc `tienHien: "chi-tieu-chung"`; bỏ «Máy chủ …» khỏi chữ người đọc;
   ô số lượng xoá được. (mục 12)
9. Kỷ niệm: không xoá ảnh khi upload hỏng; phân xử lỗi upload bằng cổng media. (mục 14)

### Chưa đo ở lượt này (đừng đọc im lặng thành xanh)

- AI thật (không có `GEMINI_API_KEY` → **0 lời gọi trả phí**): `@Rủ Đi`, Tờ hẹn AI, tìm
  bằng câu, đọc ảnh bill. `place-reasons` được gọi 2 lần nhưng đi đường không khoá.
- Bàn phím ảo (ADBKeyBoard thay thế) → chưa đo bàn phím che nội dung.
- Thông báo đẩy; hẹn mở / túi riêng / thư năm sau / bản đồ hai người (lát 2–3, chưa có
  trên `main`); vai Người lo / Người chấm; đóng sổ thật; story; iOS.
- Bản release (các chữ «bản trải nghiệm» có thể chỉ do `__DEV__`).

### Critique impeccable — ô nhập chữ

Chạy theo `reference/critique.md`: Assessment A (review thiết kế) và B (detector +
bằng chứng) là hai sub-agent độc lập. Detector exit 0 trên `ui/`, `hai-nguoi/`, `chat/`
(canary HTML đỏ đúng → detector sống; nhưng luật của nó là CSS/HTML nên «sạch» là bằng
chứng yếu cho RN). Điểm Nielsen phần nhập chữ: status 1 · real-world 2 · control 3 ·
consistency 1 · error-prevention 1 · recognition 2 · aesthetic 3 · recovery 1.
Ưu tiên: P0 ô Nếp 0 px · P1 multiline canh giữa · P1 Nếp đè sheet · P2 ngày/giờ gõ thô
· P2 hai nhãn «Giờ» + placeholder trông như giá trị · P3 không có trạng thái focus / chỗ
lỗi dưới ô. Ô một dòng làm tốt (52dp, viền 3:1, placeholder một dòng có «…»); OTP đúng
chuẩn (một input thật, `sms-otp`, focus màu).

## Đợt 5 (24/09) — bộ ô nhập và UI chung: đã đóng / còn mở

Đo trên dev build Android (APK dựng lại với plugin mới), stack cô lập, tài khoản Linh.

- **Ô nhiều dòng canh giữa → đã sửa** (`ui/Field.tsx`, hàm thuần `kieuO`): chữ và gợi ý ở góc
  trên-trái, padding trên dưới bằng nhau, cao theo `numberOfLines`, quá 8 dòng thì cuộn; viền
  accent 2dp khi đang nhập (chữ không xê dịch), chỗ cho `error`/`helper`. Ảnh: sheet «Đề nghị
  sửa» (sáng) và «Hai ô ràng buộc» (tối, đang nhập).
- **Ngày/giờ gõ thô, hai ô cùng tên «Giờ» → đã sửa**: ngày là chip từ hôm nay tới hết tuần sau
  («T7 26/09», nhãn trợ năng «Thứ Bảy 26/09»); giờ chuẩn hoá «1800»/«21h30» → «18:00»/«21:30»,
  kiểm `hh:mm`, «Đi tiếp phải sau 19:15» (qua nửa đêm vẫn được); tên trợ năng «Giờ chỗ chính» /
  «Giờ đi tiếp»; chặng có giờ mà không có việc thì không gửi (trước: máy chủ sẽ 422).
- **«Chỗ chính» không chọn được quán → đã sửa**: «Chọn chỗ / Đổi chỗ / Bỏ chỗ» từ danh mục
  ngay trong sheet; quán đã gắn không còn bị rơi khi sửa dòng việc; đổi quán hiện thành một dòng
  «Chỗ chính: Tiệm Nướng Xóm Lào → Lẩu Gà Lá É Tao Ngộ» ở cả hai phía. Trên máy: gửi từ app, máy
  chủ lưu `place_id: p-lau-ga-la-e`.
- **Bản phác của mình mở «Gửi phiên bản 2» → đã sửa**: «Sửa bản phác / Lưu bản phác».
- **Đổi cỡ chữ nhảy về màn đầu → đã sửa** (plugin `giu-man-khi-doi-co-chu`: `fontScale|density`
  trong `configChanges`). Trên máy: đang ở tờ giấy, `font_scale 1.3` → vẫn ở tờ giấy, chữ to lên.
- **«máy chủ» trong câu người dùng đọc → đã đổi 49 dòng** trên đường đi thường (các bước chia
  bill, hoá đơn, quyết toán, đợt thu, hồ sơ, mời, kỷ niệm) và lỗi chung. Còn 73 dòng (lỗi hiếm ở
  màn cũ/legacy, chữ chẩn đoán) chưa đổi.
- **«Chưa có chuyến» in như con số, «Nhóm chưa có kèo nào» khi vừa chốt kèo → đã sửa.**
- **Sheet lập sổ hứa «Không ai ngoài hai bạn thấy» cạnh ổ khoá mở; bật đôi hứa vai Người lo/chấm
  chưa có → đã sửa** thành câu đúng phạm vi. Câu đóng sổ bớt lạnh. «Vì:» hết thụt lệch.
- **Chat nhóm không cập nhật «n thành viên» khi có người vào → đã sửa** (đọc lại khi focus và
  khi có tin từ người lạ; đếm người `active`).
- **Còn mở:** chất giấy ở chế độ tối (`paper` #2e335c là hợp đồng màu có đo trong spec §16 /
  DESIGN.md — cần một lượt thiết kế, không sửa lén); Khám phá ở chữ 1.3 (nút AI xuống hàng, giá
  bị cắt) chưa làm.

## Đợt 6 (24/09) — tiền và kỷ niệm: đã đóng / còn mở

- **Đăng kỷ niệm hỏng («Không nối được máy chủ») → tìm ra gốc và đã sửa.** Máy chủ nhận ảnh
  bình thường (curl với phiên thật: 201). Trên máy, lỗi thật là `Unsupported FormDataPart
  implementation`: `fetch` toàn cục của app là của Expo (runtime winter), chỉ nhận phần
  multipart là chuỗi, `Blob` hoặc đối tượng có `bytes()`, còn phần `{ uri, name, type }` kiểu
  React Native bị ném lỗi **trước khi một byte rời máy**. Cùng lỗi ở ảnh nhóm/chat, ảnh đại
  diện, ảnh bài đăng, quét bill và quét ảnh chụp màn hình. Sửa: app cài `datCachDocTepAnh`
  (đọc bằng `File` của `expo-file-system`), mọi phần ảnh đi qua `phanTepAnh`. Trên máy: ảnh
  lên tường nhóm; quét bill giờ tới máy chủ (trả «chưa bật» vì máy không có khoá — 0 lời gọi).
- **Thử lại chắc chắn hỏng (ảnh gốc bị xoá) → đã sửa**: bản nén luôn dọn, ảnh đã chọn chỉ dọn
  khi gửi thành công hoặc khi rời màn. (Ảnh **bill** vẫn xoá ngay cả khi hỏng — luật riêng tư.)
- **Ảnh dọc có hai dải xám, nút «Chia sẻ» bị đẩy khỏi màn → đã sửa** (khung theo tỉ lệ ảnh
  3:4…1.91:1; nút ở footer).
- **Ô số lượng không xoá được để gõ lại, ô tiền hiện số thô → đã sửa** (chuỗi nháp khi đang gõ,
  rời ô thì «420.000»); «Thành tiền» ghi rõ «Tổng cả dòng · 210.000đ mỗi phần».
- **«Xem lại hoá đơn» như form CRUD → thành tờ hoá đơn** (đầu tờ là buổi đi, dòng chấm dẫn,
  tổng ở đáy). Bước gán món với 2 người: một chip «Cả hai» thay ba nút.
- **Quyết toán biến bạn gái thành con nợ → đã sửa cho context `pair`**: «Chi tiêu chung» — mỗi
  người «trả nhiều hơn / ít hơn phần mình» với đúng số dư ròng máy chủ tính; danh sách chuyển
  và đợt thu thu vào «Muốn cân lại? Xem cách chuyển». Nhóm thường 2 người giữ nguyên.
- **Còn mở:** tường nhóm cắt ảnh dọc vào khung ngang; kỷ niệm chưa gắn vào tờ đã chốt (spec §2
  «giữ một điều»); chưa có tổng «hai bạn đã chi bao nhiêu» vì không có route tổng chi của một
  context (không tự cộng trên máy — luật tiền).

## Đợt 7A (24/09) — lỗ rò: bản phác chưa gửi hiện cho người kia

- **Tái hiện** trên stack cô lập bằng hai phiên thật (đăng nhập OTP qua API công khai): Minh phác
  «Bí mật: quà sinh nhật (dữ liệu mẫu)» kèm lý do riêng, **không gửi**, rồi «Tuần này nghỉ» → danh
  sách của Linh có tờ `nghi_tuan` với nguyên dòng đó, `GET /papers/{id}` của Linh trả cả nội dung lẫn
  lý do. Gốc: quyền xem chỉ kiểm `state != "nhap"`.
- **Đã sửa** (Go + Python oracle, 29 tệp evidence): tờ chưa có phiên bản nào được gửi chỉ chủ bản
  phác thấy, ở mọi trạng thái (`nhap`, `nghi_tuan`, `bo`, `het_han`). Sau khi dựng lại core: danh sách
  Linh không còn tờ đó, chi tiết 404; Minh vẫn thấy tờ của mình. Tờ đã gửi thì đóng rồi vẫn đọc được
  cho cả hai như cũ.
- **Phần còn lại của Đợt 7 chờ Lead ký ADR-0034** (gu/vai/nhịp/câu hỏi tuần) — xem
  `docs/team/hang-doi.md`.

## Lượt «còn nợ» 1 (24/09) — khoảnh khắc hẹn, kỷ niệm của hai bạn, Khám phá chữ lớn

Đo trên dev build Android, stack cô lập dựng mới (cặp Minh/Linh gieo bằng API công khai).

- **Đỉnh cảm xúc sau «Đã chốt» trống trơn → đã sửa**: tờ đã chốt có con dấu mực «CÒN 2 NGÀY» (coral
  giữ cho «Xem kèo», một điểm dẫn mỗi bề mặt) và «Lần hẹn đầu tiên / thứ N của hai bạn»; hàng ghim
  trong nhắn riêng thành mốc «Hai bạn hẹn Thứ Bảy 26/09 · Lẩu Gà Lá É Tao Ngộ · còn 2 ngày» (dựng ở
  client từ sổ, không chèn gì vào chat — E2EE).
- **Kèo «0đ / 0đ» → «Chưa đặt ngân sách cho buổi này.»**
- **Kỷ niệm chỉ đi lên tường nhóm → đã có «Kỷ niệm của hai bạn»** (ADR-0021 §2.5 cho phép): lối vào
  ở «Cài đặt sổ» và sau buổi đi («Giữ một tấm ảnh của buổi này»); màn đăng nói «Chỉ hai bạn thấy».
  Trên máy: ảnh vào đúng sổ đôi. **Lỗi phát hiện kèm:** nút «Thả khoảnh khắc» trên tường của MỘT
  nhóm mở `/moments/new` không kèm nhóm → ảnh vào nhóm *hiện tại*; nay mang `?ctx=`.
- **Tường nhóm cắt ảnh dọc thành dải 4:3 → khung theo tỉ lệ ảnh** (3:4 … 1.91:1, đọc từ ảnh lúc tải).
- **«Thêm vào kèo» không thấy kèo của sổ đôi, và chuyển hướng đi khi chưa có nhóm → đã sửa**: mục
  «Hẹn của hai bạn»; trên máy thêm «Tiệm Nướng Xóm Lào» vào kèo của đôi, mở đúng kèo.
- **Khám phá ở chữ 1.3**: ô tìm và nút AI giữ một hàng; dòng giá được hai dòng khi chữ lớn.
- **«máy chủ» còn sót → đã viết lại 66 câu** (ErrorState mặc định «Chưa đọc được từ máy chủ» →
  «Chưa tải được»; câu nêu tên biến môi trường cho người dựng hệ → câu người dùng làm được gì). Còn 5
  chuỗi chẩn đoán nội bộ và 1 ở màn dev.
- **Để mở có chủ đích:** chất giấy chế độ tối — vân giấy đo ≈ 2 mức trên nền tối (phẳng), màu tờ đêm
  là hợp đồng đã đo (spec §16, DESIGN «sổ đóng trên bàn»); cần một lượt thiết kế có đọc mù.

## Lượt «còn nợ» 2 (24/09) — «Rủ … tới đây» từ trang quán

Đóng mục 🟡 ở §11 («không có nút Rủ Minh tới đây ở trang quán»). Đo trên máy thật, tài khoản Linh
(`emulator-5554`), ảnh ở `~/.cache/rudi-qa-couple/shots/no2-*.png` (ngoài repo):

- Trang quán có một nút «Rủ <tên> tới đây» cho mỗi sổ đôi đang bật (tối đa 3). Bấm → tờ giấy của hai
  người, bản phác mở sẵn trình sửa với **quán đó làm chỗ chính** và dòng việc theo loại quán, cùng
  bảng với bản phác của máy chủ (`pair_paper._VIEC_THEO_LOAI` / `pairpaper.viecTheoLoai`): Lưng Chừng
  Cafe → «Cà phê», không còn «Ăn tối» ở quán cà phê. «Bản phác sẽ đổi:» liệt kê đúng hai dòng.
  Lưu → «BẢN PHÁC · Cà phê · Lưng Chừng Cafe»; Gửi → «ĐÃ GỬI · Minh chưa xem».
- Tờ người kia gửi (Linh là người nhận, còn đề nghị sửa được) → mở «Đề nghị sửa» với chỗ đó.
- Tờ mình đã gửi → một câu «Tờ tuần này đang chờ Minh trả lời. Chỗ bạn chọn chưa được thêm…»;
  chỗ đã có trên tờ → «Chỗ này đã ở trên tờ tuần này rồi.»; tờ tuần đã chốt → «để dành cho tuần sau».
  Trước đó chỗ bị bỏ lặng lẽ.
- Lỗi có sẵn, sửa luôn: `?ru=1` («Rủ một người đi chơi») gọi xin tờ cả khi đã có tờ mở, và màn hiện
  lỗi đỏ «Tờ giấy không ở trạng thái làm được việc này.» trên một tờ bình thường. Giờ chờ lần đọc
  đầu (`daNap`) và chỉ xin khi chưa có tờ mở (`nenXinTo`).

- Khay «+» của nhắn riêng (🟠 §11) → ô thứ tư là «Tờ giấy», mở tờ giấy của hai người; không còn
  «Tờ hẹn» AI song song. Đường AI tường minh (`@Rủ Đi …`) vẫn mở khung phác, nhưng hỏi «Hai bạn muốn đi
  đâu?»; bình chọn «Hai mình chọn gì? / Tối nay mình ăn gì?». Nhóm giữ nguyên chữ của hội.
  **Lỗi hình phát hiện kèm:** nhãn «Tờ giấy» bị cắt còn «Tờ» (Android đo chữ sát, «giấy» rớt xuống dòng
  không hiện) → nhãn giãn theo cột. Ảnh `no2-07c` (trước) / `no2-07d` (sau).
- Hàng ghim trong nhắn riêng nói «Người ấy chưa xem» trong khi tờ nói «Minh chưa xem» → giờ cùng tên.

Không đo: phía Minh bấm «Rủ Linh tới đây» trên tờ Linh gửi (chỉ có ca thuần `goiYChoLam`, máy thứ hai
không bật vì RAM); trình đọc màn hình.

## ADR-0034 lát 1 (25/09) — `chia_gu`: gu của hai bạn, mỗi người tự bật

Lead ký ADR-0034 ngày 25/09. Lát này làm §2.1–2.2; phần Nếp dùng gu, người lo, câu hỏi tuần là lát sau.

- Máy chủ (Go + Python oracle): mục đích đồng ý `chia_gu` theo người: chỉ trong «Một đôi»; tạo là
  người đề nghị đồng ý và hoàn tất luôn, không bao giờ nằm chờ người kia; người kia không «đồng ý hộ»
  được. `GET …/notebook` có `taste`: null ngoài «Một đôi»; gu người kia chỉ khi HỌ bật; gu chung chỉ khi
  CẢ HAI bật; chỉ khi đó máy chủ mới đọc `person_interests`. Migration `e3b7c1d9a4f2` mở CHECK.
- App: «Cài đặt sổ» → «Gu của hai bạn» (chỉ hiện khi là một đôi). Đo trên máy thật (Linh, stack
  cô lập dựng lại với core mới + migration): Minh đã bật → «Minh thích Ăn uống, Cafe, Outdoor và Game.»;
  Linh bấm «Cho Minh thấy gu của mình» → «Hai bạn cùng thích Cafe và Outdoor.» hiện trong sheet và dưới
  tờ giấy; «Thôi cho Minh thấy…» → dòng chung biến mất, lần đọc sau. Ảnh `gu-01…03` (ngoài repo).
- Không hứa điều chưa làm: câu «Nếp dùng gu khi phác tờ» bị bỏ khỏi màn cho tới lát (a).
