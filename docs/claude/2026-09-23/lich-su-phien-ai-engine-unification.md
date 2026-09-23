# Lịch sử phiên `ai-engine-unification` (22-09 20:41 → 23-09 23:57, giờ VN)

File này tóm lại cả phiên làm việc: leader giao gì, đã làm gì, quyết gì và vì sao, cái gì còn
dở. Dùng để nạp vào agent khác làm tiếp. Đọc kèm tài liệu bàn giao cùng thư mục
`ban-giao-mot-engine-ai-va-dock-nep.md`: file đó là danh sách việc, file này là câu chuyện.

Đây là bản tóm tắt, không phải transcript. Ý của leader được ghi lại bằng lời thường, trừ vài
câu quyết định được trích nguyên văn vì từng chữ trong đó có trọng lượng.

---

## 0. Kết quả cuối, xem trước

| PR | Nhánh | Nội dung |
|---|---|---|
| #643 | `claude/p0-w30-ban-giao-ai-nep` | Bàn giao và file lịch sử này |
| #644 | `claude/p0-w31-chat-tai-tranh-khoa` | Tải realtime: sửa script phân tích khoá, đo lại |
| #646 | `claude/p0-w30-nep-dock-trong-le` | Dock Nếp thiết kế lại: tờ giấy cài trong lề |
| #647 | `claude/p0-w30-mot-engine-loi-goi-ai` | Engine AI một đường (ADR-0034) |
| #648 | `claude/p0-w30-ai-luon-bat` (base #647) | AI và realtime bật mặc định ở prod |
| #649 | `claude/p0-w30-ai-chat-luong` (base #647) | Gu, ngân sách, roster như v1; bộ đo chất lượng 16 ca |

Chưa PR nào được merge. Thứ tự merge và các ràng buộc nằm ở mục 2 của tài liệu bàn giao.

---

## 1. Đầu việc được giao, theo thứ tự thời gian

1. **Hiểu hai con AI đang có.** Một con nằm trong group chat giữa người dùng và bạn bè; con kia
   là Nếp, trợ lý cá nhân nổi ngoài app. Leader muốn nắm cơ chế của cả hai trước.
2. **Giải thích vì sao AI trong nhóm có «v1» và «v2».** Leader thấy khó hiểu.
3. **Hợp nhất thành một.** Leader không muốn chia đôi. Leader muốn một cách làm đạt mức
   production, và khi AI được gọi trong nhóm thì nó trả lời tốt hơn.
4. Leader chọn trong câu hỏi: **AI được thấy mọi thứ như v1**, tức chat, tên cả nhóm, gu và lịch
   sử chuyến. Nguyên văn: «Mọi thứ như v1: chat + tên cả nhóm + gu + lịch sử chuyến tôi là
   leader làm vậy cho tôi».
5. «oke vậy chốt hướng đầu làm sao gọn gàng thông minh state of the art production nhé»: chốt
   hướng A. Client gom transcript đã giải mã và gửi kèm lời gọi; máy chủ đắp thêm thứ nó sở hữu.
6. **Luật toàn cục, lưu vào bộ nhớ.** Backend đã chuyển sang Go, nên mọi xử lý backend dùng Go
   làm mặc định. Python chỉ còn bước gọi model.
7. Leader trả lời ba câu hỏi phạm vi:
   - **Xoá v1 tận gốc, kể cả Python.**
   - **Gộp `/chia-bill`** vào cùng đường gọi.
   - **Làm luôn não chữ cho Nếp.**
8. «đi tiếp trong lúc đó chạy song song mọi thứ đi».
9. Leader hỏi «chạy tới đâu rồi … test e2e thử chưa bật app lên test thử chưa».
10. Leader hỏi các lựa chọn «tách / bật cả 2 / để nguyên» là gì, và dock Nếp là gì.
11. **Quyết định cờ.** Nguyên văn: «nó phải luôn bật và AI luôn hoạt động tốt và chạy tốt chứ
    tại sao lại real time nhưng AI dở cả 2 đều phải tốt».
12. **Dock Nếp:** «deep dive và fix sao cho nó đẹp nó stunning có câu chuyện nhưng hài hòa với
    những thứ đã có sao cho thật đẹp và tốt chứ».
13. Khi có phiên khác góp ý về dock: «bạn sẽ là người được ưu tiên hơn nhưng nghe góp ý từ nó
    để 2 bạn có cái nhìn tốt hơn».
14. **Dừng và bàn giao.** Commit, push, tạo PR, viết tài liệu bàn giao cho agent khác.
15. **Viết file lịch sử này**, rồi push lên PR bàn giao.

---

## 2. Giai đoạn 1: đọc hiểu (22-09 tối)

Trên `main` lúc đó có **ba** bề mặt AI chứ không phải hai.

- **v1 companion.** Gồm `POST /contexts/{id}/ai-turn`, cộng một nhánh tự kích hoạt trong
  `POST /messages`. Máy chủ tự đọc 40 tin gần nhất, roster, gu và ngân sách, rồi gọi model
  **đồng bộ ngay trong request**. Trả lời khá, nhưng máy chủ tự chọn đọc gì, kể cả những tin
  người gọi chưa từng mở. Cách này không sống được khi chat v2 chuyển sang E2EE.
- **v2 `chatassist`.** Gồm `POST /contexts/{id}/ai-invocations`: một hàng đợi job có lease,
  retry và idempotency. Nhưng model chỉ nhận đúng câu người gõ, `members: []`, và catalogue là
  `ORDER BY id LIMIT 40`. Toàn bộ nằm sau cờ `MOBILE_CHAT_CHANGES_CANDIDATE=1`, và cờ đó chỉ có
  trong script test. Nghĩa là production chưa từng có v2.
- **Nếp** (ADR-0033). Dock và phần vẽ ảnh đã chạy. Ô soạn chữ chỉ in «Mình chưa trả lời bằng
  chữ được».

Cây làm việc ban đầu (`codex/p0-w28-chat-go-e2ee`) đi sau `main` 65 commit, nên tôi mở worktree
mới từ `origin/main`.

## 3. Giai đoạn 2: thiết kế và kế hoạch

Thiết kế cốt lõi, về sau thành **ADR-0034**:
- **Người gọi trao ngữ cảnh.** Client (nơi có khoá) gom 40 tin đã giải mã thành `boi_canh`.
- **Máy chủ không đọc chat.** Máy chủ chỉ kiểm id tin có thuộc đúng phòng hay không, bằng MỘT
  câu SQL, và câu đó không đọc cột `body`.
- **Máy chủ đắp thêm** thứ nó sở hữu và chưa bao giờ mã hoá: roster, gu, ngân sách, catalogue
  (catalogue đi qua promptsafety).
- **Lộ ít hơn v1**, vì chỉ những gì người gọi vốn đọc được mới ra tới model.
- **Sống qua cutover E2EE**, vì không bước nào cần máy chủ giải mã.
- Hợp đồng Python `companion-reply` không đổi dòng nào.

Các agent soát kế hoạch bắt được bốn lỗi trước khi viết code:
- Migration version 2 đã có người dùng → phải lấy **version 3**.
- `/vote` còn sống, không được xoá gộp cùng `actOnMessageIntent`.
- `chia_bill` không thể là một `kind` thẻ mới, vì `GroundCard` bị ghim parity với Python.
- CLAUDE.md cấm xoá Python → phải thêm một **ngoại lệ có tên** vào CLAUDE.md.

## 4. Giai đoạn 3: xây engine (PR #647, 13 commit)

- ADR-0034, và ngoại lệ trong CLAUDE.md.
- Lược đồ v3 (`schema_scope.sql`):
  - thêm các cột `scope`, `boi_canh`, `result`;
  - ràng buộc đặt tên `chat_ai_context_needs_prompt`: bối cảnh không được sống lâu hơn prompt,
    và nếu quên xoá thì câu lệnh nổ ngay chứ không lặng lẽ để lại dữ liệu;
  - index riêng `chat_ai_me_logical` cho phạm vi cá nhân: UNIQUE thường không cưỡng chế khi
    `context_id` NULL;
  - `DROP CONSTRAINT` không kèm `IF EXISTS`, để một cái tên sai sẽ nổ to thay vì bị nuốt.
- Gói bối cảnh phía client (`boi-canh.ts`, `boi-canh-chat.ts`). Commit này sửa luôn một bug
  idempotency trước đây im lặng: khoá chỉ tính theo prompt, giờ tính cả gói.
- Máy chủ nhận gói (`boicanh.go`):
  - bound tính theo rune, trường hợp xấu nhất;
  - `MaxBytesReader` 128 KiB, chỉ áp cho route này;
  - digest tính trên prompt cộng gói.
- Worker dựng `conversation` từ gói.
- **Khối «Mình đang thấy»** ngay trên nút gửi: người dùng thấy trước cái sắp đi ra.
- Vá `promptsafety` cho catalogue của worker. Lỗ này đang mở thật: một hàng `places` mang lệnh
  có thể đi thẳng tới model.
- Catalogue lọc theo điểm đến, thay vì 40 hàng đầu theo id.
- Cổng quét nguồn: package AI không được đọc hội thoại.
- Ca e2e I5; kịch bản parity cho lệnh gạch chéo.
- Sửa nhãn `aiInk` đặt trên nền `aiSoft`: tương phản 1.17:1, gần như chữ trắng trên nền trắng.
  Kèm một cổng cặp màu.

Kiểm tra: dựng stack chat-e2e và mở app bằng trình duyệt. Con số trên màn khớp con số trên
dây, 9 = 9; parity 0 khác biệt.

Lỗi gặp dọc đường và cách sửa:
- Bỏ DEFAULT của `scope` làm INSERT cũ trả 503 (đúng như dự kiến) → sửa INSERT.
- tsc báo lỗi trong khi test vẫn xanh → sửa kiểu.
- `npm test` hỏng với `node_modules` symlink → dùng `npm ci` thật.
- Fixture Postgres thiếu bảng `destinations` → thêm vào fixture.
- Có lần tôi coi `RACY` trong kết quả parity là «phải chạy lại». Sai: tổng khác biệt là 0.

## 5. Giai đoạn 4: «luôn bật» và tải realtime

Leader hỏi chọn gì giữa tách, bật cả hai, hay để nguyên. Leader chọn: **cả hai luôn bật, và cả
hai phải tốt.** Có một lần tôi chặn việc chờ leader quyết cờ trong khi `chat_e2e_stack.sh` tự
đặt cờ sẵn. Chặn như vậy là thừa; tôi đã nhận sai.

**Tải realtime.** Một agent được giao sửa tranh khoá. Nó phát hiện bản sửa đã lên `main` từ
hôm trước (PR #633), nên chỉ đo lại, trên cùng máy:
- Burst 300/s: p99 **2.600 → 208 ms**. Toàn vẹn 9 triệu/9 triệu, 0 hụt dãy.
- Mẫu chờ khoá tuple giảm 214 → 18.

Agent sửa thêm script phân tích khoá (trước đó đọc sai kẻ giữ «idle in transaction») và sửa
dòng tiến độ đã cũ. Kết quả thành PR #644.

**«Luôn bật».** Một agent khác làm, kết quả thành PR #648.
- Luật cờ `resolveChatFeatures`:
  - không đặt + prod → bật;
  - `0` → tắt, có WARN;
  - không đặt + dev → tắt, có WARN, không từ chối khởi động.
- Vì sao dev vẫn tắt: agent dựng lại được cảnh ở dev, một header có thể đúc ra bearer của
  người khác. Ràng buộc prod là bất biến bảo mật thật.
- `core migrate-chat` không đòi cờ; compose có service `migrate-chat`.
- Số đo: tầng Postgres 60 ca, chat-e2e 42 ca, parity 604 bước với 0 khác biệt, đột biến đều đỏ
  đúng ca.
- Một phiên khác cảnh báo `make up` sẽ chết. Hoá ra brief ban đầu đã chặn đúng tổ hợp đó.

## 6. Giai đoạn 5: chất lượng trả lời AI

Phiên song song cùng lịch sử chỉ ra một lỗ thật: **mọi cổng chỉ đo đường ống, không cổng nào
đọc một câu trả lời.** Tầng e2e dùng brain stub tất định. Một agent làm phần này, kết quả thành
PR #649:
- Chuyển catalogue và gu từ `routes` sang `service`, cắt dán nguyên khối.
- Worker đắp gu các thành viên đang ở trong phòng, ngân sách, và roster.
- **Roster dùng bút danh** («Mình», «Bạn 1»…). Lý do: khối xem trước đang hứa «Tên tài khoản
  không đi kèm». Đổi sang tên thật là quyết định của leader, xem bàn giao mục 6.
- Corpus 16 ca viết tay, chấm bằng máy. Lượt thật trên gemini-2.5-flash: **9/16 ca qua**.
  Output model đã mất khi máy khởi động lại.

## 7. Giai đoạn 6: dock Nếp

**Vòng 1.** Đổi đĩa tím 57dp thành một tờ giấy (paper, hairline `lineStrong`, không bóng). Thêm
cơ chế **«Chừa một chỗ cho nhau»**: sự kiện `nhuong-cho` có đếm lồng nhau, được `ui/Sheet` và
khay Tờ hẹn gọi. Trên khay, phần tờ lộ ra giảm từ 57px xuống 7px, và 0 chữ bị che.

Finish reviewer độc lập (context mới) chấm `fix`, năm lỗi:
1. Dải tím «có việc» vẫn hiện ở màn tiền, trái ADR-0033.
2. Khi nghỉ, tờ vẫn cắt chữ trên thẻ ở Khám phá («20|0», «22:|»).
3. Thiếu ghi chú miễn concept-roll (Extension flow).
4. Chưa chụp các trạng thái `he`, `an` và `coViec`.
5. Nhìn giống một cái tab UI chứ chưa ra một tờ giấy: thiếu góc gấp.

**Đo trước khi sửa.** Tôi quét lề phải của chữ trên các tab và trong hội thoại. Giờ tin nhắn kết
thúc đúng **16dp** tính từ mép, bằng lề trang. Kết luận: thứ gì nằm nghỉ mà rộng hơn 16dp thì
chắc chắn đè chữ ở một vị trí cuộn nào đó.

**Thiết kế lại, commit `e4e98ae0`:**
- Mặc định Nếp **cài trong lề**: tờ nhô 10dp; khi có việc thêm 4dp của tờ `accentSoft`, tổng 14dp.
- Góc trên bên trái gấp theo ngữ pháp ToGiay (vẽ SVG, vì góc gấp là góc bị thiếu). Không san hô.
- Vùng chạm dừng đúng ở lề (`slopTrai`).
- `hienToSau` cấm tờ «có việc» ở màn tiền, khi có tờ khác mở, và khi bảng Nếp đang mở.
- Lưu trên máy đổi sang `rudi.nep.dock.v2` với field `ra`.
- Maestro 12 viết lại theo thứ tự mới.

**Góp ý từ phiên `chat-backend-go-migration`**, người đã đo trên máy thật:
- Đĩa cũ nuốt «Đồng ý» ở flow 25. Đo bằng uiautomator: tâm nút nằm trong chính cái đĩa.
- Màn story (vùng chạm neo `right: 0`) và bản đồ tràn mép không có lề cho Nếp.
- Vùng chạm 16dp dưới sàn 48dp của audit a11y.
- Dòng hé có thể nuốt cú chạm.
- Dải cử chỉ Back rộng 29.7dp ở mỗi mép.

**Sửa theo góp ý, commit `656c1235`:**
- Nếp đang cài không bao giờ tự hé dòng.
- Màn story và bản đồ khiến Nếp nhường chỗ.
- Audit a11y báo riêng nhóm mục tiêu **chạm** sát cạnh màn, và chỉ nhóm chạm.
- Kéo Nếp ra bằng chạm, không bằng vuốt vào trong, vì vuốt vào trong ở mép là cử chỉ Back của
  hệ thống. Tôi không đòi lại mép (không dùng `setSystemGestureExclusionRects`) để khỏi cướp
  Back của người dùng.

**Số đo trên bản web**, sửa lại phép đo để tính phần chữ thật sự hiện ra: lúc nghỉ 0/153 chữ bị
che ở tab đầu và 0/180 trong hội thoại; khi khay mở 0/373.

**Chưa xong:** reviewer vòng 2, chụp lại ảnh, dòng hé, và mọi phép đo trên máy thật.

## 8. Sự cố phối hợp và môi trường

- **Hai tiến trình của cùng một hội thoại cùng nhận một worktree là của mình.**
  - Worktree `wt-ai-engine` hoá ra thuộc tiến trình kia. Tôi đã chuyển phần dock sang worktree
    riêng (patch khớp từng byte), rồi trả cây kia về sạch.
  - Một bên thứ ba ghi vào cây `wt-chat-ui` của phiên khác. Cuối cùng xác định là một tiến trình
    thứ hai của chính phiên đó.
- **WSL khởi động lại ba lần** (11:22, 20:05, 21:41) khi nhiều việc nặng chạy song song. Mỗi
  lần mất sạch `/tmp`, scratchpad, stack và máy ảo; git còn nguyên. Tôi chuyển sang chạy tuần
  tự, commit sớm, và để script cùng bằng chứng ngoài `/tmp` (`~/.cache/rudi-bang-chung/`).
- **Metro giữ biến env cũ.** `expo export` không kèm `--clear` lấy lại `EXPO_PUBLIC_API_URL`
  của bước build:check, khiến đăng nhập bắn vào `api.build-check.invalid`. Bẫy này đã lưu vào
  bộ nhớ.

## 9. Những quyết định kỹ thuật đáng nhớ

- Máy chủ **không kiểm nguyên văn** transcript client gửi: client có thể dán thẳng vào prompt,
  nên kiểm cũng không chặn được gì. Máy chủ chỉ kiểm điều có nghĩa: id thuộc phòng, tác giả, bound.
- Giữ tên package `chatassist` để không cắt đứt vết kiểm toán của lần review độc lập ngày 21-09.
- `chia_bill` là thẻ `kind:"text"`, số liệu để ở cột `result`, không thêm kind mới.
- Không xoá v1 trước khi «luôn bật» vào `main`, vì production sẽ mất sạch AI.
- Dock: lề trang là ngân sách đo được, không phải lựa chọn thẩm mỹ. Nhường chỗ là một tính cách
  của nhân vật, không phải một z-index.

## 10. Việc tiếp theo

Xem mục 3 đến 7 của `ban-giao-mot-engine-ai-va-dock-nep.md`. Tóm tắt:
- Merge theo thứ tự #644 → #646 → #647 (sau khi merge `main` vào) → #648 và #649 (đổi base về
  `main`).
- Dock: chụp lại, reviewer vòng 2, đo trên máy thật.
- Sau #648: xoá v1, gộp `chia_bill`, làm não chữ cho Nếp.
- Leader quyết roster dùng bút danh hay tên thật.
- Sửa 7 ca AI trả lời trượt.
