# Thiết kế 03 — Rủ Đi AI trả lời trong luồng nhóm, kiểu Meta AI

- Ngày: 2026-09-25.
- Commit gốc: `f251db7`.
- protocol_version: không áp dụng (không phải lượt thí nghiệm).
- Trạng thái: **thiết kế đã được người dùng duyệt, chờ Lead ký ADR**. ADR đi kèm:
  `docs/decisions/proposals/ADR-0039-rudi-ai-tra-loi-trong-luong.md`.
- Phạm vi: bot nhóm «Rủ Đi AI» (`scope='group'`, lệnh `plan | chia_bill | hoi`).
  - Tin `@Rủ Đi` là tin thường. Chip xem trước nằm trên nút gửi.
  - Thẻ `tra_loi` trả lời vào tin tag. Lane cũ: cả phòng thấy chữ chạy. E2EE v2: chỉ người gọi thấy.
  - Engine ở thiết kế 01 (ADR-0037). Hàng đợi, Redis và frame WS ở thiết kế 02 (ADR-0038). Ở đây chỉ
    nói chỗ bot nhóm chạm vào hai phần đó.
- Thứ tự ưu tiên khi lệch: bảng «Hợp đồng chung» (`docs/architecture/03-ai-engine-hop-dong.md`) đè
  lên bản thiết kế gốc.
- Quy ước đánh dấu:
  - «(sửa theo phản biện)» là chỗ khác bản gốc vì hai bản phản biện đối kháng.
  - `[P1-n]` là phát hiện số n của phản biện «ràng buộc, riêng tư, bảo mật, đúng đắn».
  - `[P2-n]` là phát hiện số n của phản biện «khả thi, thứ tự, đầy đủ».
  - «(theo hợp đồng chung)» là chỗ đổi cho khớp bảng hợp đồng.

## 0. Sự thật đã kiểm tại `f251db7`

| Sự thật | Chỗ |
|---|---|
| Gõ `/plan`, `/chia-bill`, `@Rủ Đi` thì tin không được gửi: `goiMoHinh` chặn và mở khay | `apps/mobile/src/rudi/screens/chat/GroupChatLive.tsx:85-87`, `:358-366` |
| `@Rủ Đi` được đọc thành lệnh `plan`; client hỏi lại mỗi 2 s | `apps/mobile/src/rudi/chat/ai-invocations.ts:36-40`; `chat/useChatAi.ts:39-41` |
| Worker đăng `ai_card` không tác giả và **không** đặt `ReplyToID`, dù trường có sẵn | `services/core/internal/chatassist/worker.go:261`; `repo/messages.go:107` |
| Tin AI trong DB có tác giả NULL (`human_kinds_have_author`); client coi NULL là AI | `services/api/app/db/models.py:1987-1990`; `chat/boi-canh-chat.ts:30`; `screens/groups/Conversations.tsx:52` |
| Chỉ trả lời được `text|image|sticker`; bản port có oracle Python | `domain/messageedit/messageedit.go:10`, `:42-53`; ADR-0021 §2.2.2 |
| `POST /messages` gọi `CheckReplyTarget`, kiểm thẻ bằng `GroundCard` (trần chữ 600); Python còn sống làm oracle | `routes/messages_wai.go:95`, `:114-126`; `domain/companion/companion.go:15`, `:193`; `ownership/routes.json` (`LIVE-GO`) |
| Tin `@Rủ Đi` là chữ thường; máy chủ không khởi động AI vì chữ | `routes/messages_wai.go:536-540` |
| FK trả lời là composite `(reply_to_id, context_id)`, có `uq_messages_id_context` | `models.py:1997-2004` |
| Máy chủ từ chối phòng v2 bằng 409 `encrypted_invocation_required` | `chatassist/handler.go:163-176` |
| Body đọc với `DisallowUnknownFields`; digest = `command\0prompt\0gói`; hạn 8/phút/người đếm lời gọi; timeout 8 s | `handler.go:126`, `:384`, `:414-418`, `:83` |
| Gói bối cảnh: `ban` phải là 1; ≤40 lượt, ≤300 rune mỗi lượt; `thuocPhong` chỉ đọc `id` | `chatassist/boicanh.go:39-48`, `:59-84`, `:97-119` |
| Cổng nguồn cấm `SELECT … body … FROM messages` | `chatassist/khong_doc_chat_test.go:28-56` |
| Migration `chatassist` ở version 3, checksum ghim; `chat_ai_command_scope` chỉ `plan`, `chia_bill` ở nhóm | `chatassist/migrate.go:14-28`, `:31-112`; `schema_scope.sql:36-38` |
| Rời nhóm huỷ job và xoá chữ qua trigger; trigger Go trên `messages` đã có tiền lệ | `chatassist/schema.sql:38-49`, `schema_scope.sql:60-72`; `chatlegacychange/schema.sql:52` |
| `chia_bill` tự `publish`; nháp ở cột `result`; tiêu đề và số tiền là đầu ra model | `chatassist/chiabill.go:21-35`, `:201-226`, `:310-330` |
| Nâng tờ hẹn đọc thẻ `itinerary`, đóng dấu `payload.outing_id` bằng `jsonb_set` | `chatassist/promotion.go:121`, `:142`, `:217` |
| WS phòng: 1000 slot, 5 mỗi người, mỗi trang chờ ack; `query()` từ chối tham số lạ | `chatlegacychange/handler.go:45`, `:294`, `:309-324`, `:86-112` |
| Xem trước hôm nay: «Mình đang thấy», «Chỉ gửi lời nhờ» | `screens/chat/SoHen.tsx:224`, `:248` |
| Cổng «mọi mã từ chối có câu tiếng Việt» đọc mã từ handler Go | `apps/mobile/tests/cau-chu-goi-ai.test.mjs:16-20`; `LOI_GOI_AI` ở `ai-invocations.ts:71` |
| Xoá tài khoản giữ `messages`, rời membership, ẩn danh `people`; `expense_versions` có `recorded_by_id` | `domain/accountlifecycle/accountlifecycle.go:76-89`; `repo/erasure.go:361`; `models.py` lớp `ExpenseVersion` |
| `parity/` không nhắc `chat_ai_invocations` | grep tại `f251db7` |

## 1. Hành vi nhìn từ người dùng

- **Người gọi.**
  1. Gõ `@Rủ Đi tối nay ăn gì gần Q1?`. Chip hiện ngay trên nút gửi: «✦ Kèm 20 tin gần đây · Xem ·
     Chỉ gửi lời nhờ».
  2. Gửi. Tin vào luồng như mọi tin khác; hàng «Rủ Đi AI đang đọc 20 tin…» hiện ngay dưới nó.
  3. Chữ chạy, trả lời vào đúng tin tag. Phần quán hay lịch trình, nếu có, hiện trước chữ.
  4. Trả lời vào tin của AI là hỏi tiếp. Chip tự bật, có nút «Không hỏi Rủ Đi».
- **Người khác, lane cũ.** Thấy tin `@Rủ Đi`, rồi hàng «đang đọc…», rồi chữ chạy. Tin thật thay hàng
  tạm khi `message_id` tới. Thất bại thì chỉ thấy «Rủ Đi AI chưa trả lời lời nhờ này.», không lý do.
- **Phòng v2** (chat-lab, lát 20). Chỉ người gọi thấy chữ chạy; cả phòng thấy «Rủ Đi AI đang trả
  lời…»; máy người gọi mã hoá rồi đăng.

## 2. Thành phần

| Nơi | Việc |
|---|---|
| `chatassist/schema_luong.sql` (mới) | cột liên kết tin tag, lane, số tin đã đọc, trigger tin tag bị xoá |
| `chatassist/handler.go` `create` | `kiemTrigger`, kiểm chuỗi, hạn phòng, suy lane, digest mới |
| `chatassist/worker.go` `publish` | dựng `tra_loi`, đặt `ReplyToID` |
| `domain/companion/tra_loi.go` `GroundReply` (mới, Go-only) | kiểm từng phần bằng `groundPlaces`/`groundItinerary`; `GroundCard` giữ nguyên |
| `routes/messages_wai.go` `laTraLoiAi` | cho trả lời vào `tra_loi` trước `CheckReplyTarget` |
| `chatassist/promotion.go` `lichTrinhCuaThe` | nâng tờ hẹn nhận cả `itinerary` lẫn phần `itinerary` trong `tra_loi` |
| `chatassist/ghi_khoan.go` (lát 14) | nhận nháp từ `result`; dấu suy từ dòng chi tiêu thật |
| `apps/mobile/src/rudi/chat/nhac-ai.ts` (thuần) | nhận diện và bỏ mention, thay `docLenhAi` |
| `screens/chat/ChipBoiCanh.tsx`, `TraLoiAi.tsx` | chip + sheet «Xem»; bong bóng trả lời |
| `chat/tin-song.ts` | `docTheAi` biết `tra_loi`; `tacGiaTin` là hàm danh tính duy nhất cho ba chỗ tác giả NULL |
| hook phòng (thiết kế 02 gọi là `useRoomAi`) | nhận frame `ai`, hàng tạm, chữ lớn dần |

## 3. Dữ liệu

### 3.1 Migration `chatassist/schema_luong.sql`

Số version **cấp theo thứ tự lên main**, không đặt trước; `serve`/`work` đòi version ≥ N (theo hợp
đồng chung) [P1-7][P2-1]. Cùng mẫu `migrateScope`: cùng transaction, cùng checksum.

```sql
ALTER TABLE chat_ai_invocations
  ADD COLUMN trigger_message_id uuid,
  ADD COLUMN lane text NOT NULL DEFAULT 'legacy' CHECK (lane IN ('legacy','v2')),
  ADD COLUMN so_tin_doc integer CHECK (so_tin_doc BETWEEN 0 AND 40),
  ADD CONSTRAINT chat_ai_trigger_room FOREIGN KEY (trigger_message_id, context_id)
      REFERENCES messages(id, context_id) ON DELETE SET NULL (trigger_message_id),
  ADD CONSTRAINT chat_ai_trigger_scope CHECK (trigger_message_id IS NULL OR scope = 'group');
ALTER TABLE chat_ai_invocations DROP CONSTRAINT chat_ai_command_scope;  -- không IF EXISTS, lý do như v3
ALTER TABLE chat_ai_invocations ADD CONSTRAINT chat_ai_command_scope CHECK (
  (scope='group' AND command IN ('plan','chia_bill','hoi')) OR (scope='me' AND command='hoi'));
CREATE UNIQUE INDEX chat_ai_one_answer_per_trigger ON chat_ai_invocations(trigger_message_id)
  WHERE trigger_message_id IS NOT NULL AND status IN ('queued','running','succeeded');
CREATE INDEX chat_ai_room_active ON chat_ai_invocations(context_id, created_at)
  WHERE status IN ('queued','running');
-- Tin tag bị thu hồi → huỷ và xoá chữ, cùng mẫu chat_ai_membership_revoked.
CREATE FUNCTION chat_ai_trigger_deleted() ... AFTER UPDATE OF kind ON messages
  WHEN (NEW.kind = 'deleted' AND OLD.kind <> 'deleted')
  → UPDATE chat_ai_invocations SET status='cancelled', code='trigger_deleted',
       prompt=NULL, boi_canh=NULL, lease_id=NULL, lease_until=NULL
     WHERE trigger_message_id = NEW.id AND status IN ('queued','running','failed');
```

- **`lane` giữ `DEFAULT 'legacy'` (sửa theo phản biện) [P2-16].** Bản gốc `DROP DEFAULT` làm hỏng
  INSERT của replica cũ khi deploy cuốn chiếu (`handler.go:422` không có cột này).
- **`lane` do máy chủ tự suy, client không gửi (sửa theo phản biện) [P1-5].**
  - Nguồn là đúng câu `chat_v2_conversations` mà `authority` đã chạy (`handler.go:165-171`).
  - Request không có trường `lane`; gửi lên là 400 `invalid_body` nhờ `DisallowUnknownFields`.
  - Tới lát 20, `authority` còn trả 409 cho phòng v2, nên mọi hàng đều là `legacy`. Cột có sẵn để
    writer (`aistream`, thiết kế 02 §5.1) chọn khoá chỉ từ hàng DB.
- Cột v2 (`trigger_v2_sequence`, `result_digest`, `delivered_sequence`) vào migration của lát 20.
- `so_tin_doc` ≤40, vì trần gói giữ `maxLuot = 40` (`boicanh.go:40`); chuỗi hỏi tiếp tính trong 40 đó.
- `'hoi'` có ngay từ migration này. Handler vẫn từ chối `hoi` (`lenhNhom`, `handler.go:321`) và
  capability báo `ai.hoi.available=false` tới lát 9. Bảng rộng hơn handler thì an toàn; ngược lại
  mới ra 500.
- Trigger `chat_ai_trigger_deleted` là writer Go trên bảng Python-owned, cùng tiền lệ
  `chatlegacychange/schema.sql:52`. Nó chỉ ghi `chat_ai_invocations`, bảng parity không so.

### 3.2 Thẻ `tra_loi` (theo hợp đồng chung) [P1-7]

```json
{"kind":"tra_loi","payload":{"ban":1,"tac_gia":"rudi-ai","invocation_id":"…","lenh":"hoi|plan|chia_bill",
 "doc":{"so_tin":20,"chi_loi_nho":false},
 "phan":[{"kind":"text","text":"≤ trần chữ nhóm"},
         {"kind":"places","intro":"…","places":[…bản sao catalogue…],"omitted_place_count":0},
         {"kind":"itinerary","title":"…","stops":[…]},
         {"kind":"expense_draft","so_khoan":3,"da_ghi":[]}],
 "outing_id":"(chỉ promotion đóng dấu)"}}
```

- Tác giả DB là NULL; danh tính nằm trong thẻ (`tac_gia` là enum một giá trị). Chỉ cách này sống qua
  E2EE: ở v2, người gửi MLS bắt buộc là người gọi.
- `doc.so_tin` lấy từ cột `so_tin_doc` do `thuocPhong` đã kiểm, không lấy lời client khai.
- Tối đa 3 phần, mỗi loại tối đa một. Phần rớt kiểm thì bỏ; không còn phần nào → `invalid_ai_result`.
  Mỗi phần có thể mang provenance `{nguon, doc_tin, cong_cu}` đúng hình thiết kế 01 §3.5; `doc_tin`
  bằng `doc.so_tin`.
- **`expense_draft` chỉ là con trỏ (sửa theo phản biện) [P1-10].**
  - Bản gốc nói «nháp chỉ dựng ở máy chủ, không lấy từ model». Câu đó sai: `title` và `amount_vnd` là
    đầu ra skill `chat-expense` (`chiabill.go:21-35`).
  - Nháp ở lại cột `result` như hôm nay (`chiabill.go:201-226`, #651).
  - Phần trên thẻ chỉ mang `so_khoan` và `da_ghi` (chỉ số các khoản đã có dòng chi tiêu thật, §4.6).
    Không số tiền, không id người trả, không `shared_by`.
- Trần chữ nhóm: đề xuất 1500 rune, Lead chốt. `GroundCard` giữ trần 600 của oracle.
- Không tạo được `tra_loi` qua `POST /messages`: route đó chỉ biết `GroundCard`, trả 422
  `card_ungrounded` (`messages_wai.go:124-126`).

### 3.3 Wire

- **`POST /contexts/{c}/ai-invocations`**: thêm `trigger_message_id`, bắt buộc với `hoi`, tuỳ chọn với
  `plan`/`chia_bill` để client cũ còn chạy. `Invocation` trả thêm `trigger_message_id`, `so_tin_doc`.
- **Gói `ban: 2`**: thêm `trongLuong: bool` mỗi lượt. Máy chủ nhận `ban ∈ {1,2}` trước, client gửi 2
  sau; hôm nay `kiemBoiCanh` từ chối mọi `ban ≠ 1` (`boicanh.go:60`).
- **`chat-capabilities`**: thêm `ai.hoi{available, reason}`, `ai.mention: true`, và
  `ai.stream: "room" | "requester" | "none"` (tên do thiết kế 02 §5.2 chốt, thay hai tên của hai bản
  gốc) [P2-3].
- **Người gọi** nhận SSE `GET /contexts/{c}/ai-invocations/{id}/events` (theo hợp đồng chung). Route
  này phải rẽ trước timeout 8 s ở `handler.go:83`.
- **Người xem khác nhận frame `ai` trên WS `chatlegacychange` sẵn có (sửa theo phản biện) [P2-3].**
  - Route SSE phòng `GET /contexts/{c}/ai-live` của bản gốc bị bỏ.
  - Opt-in nằm trong khung `authenticate`, vì `query()` từ chối tham số lạ.
  - Phong bì do thiết kế 02 §5.3 chốt: `{"type":"ai","inv","tin","so_tin","id","e","d"}`.
- **Lát 14**: `POST …/ai-invocations/{id}/khoan/{k}/nhan` và `…/khoan/{k}/ghi`.
  **Lát 20**: `POST …/{id}/delivered`, `GET …/{id}/receipt`.
- Mọi đường mới vào `Matches` (`handler.go:72-78`) và vào `ownership/routes.json` loại `GO-ONLY`
  trong cùng commit (sửa theo phản biện) [P2-17].

## 4. Trình tự

### 4.1 Gửi (client, `useChatAi.guiNhoAi`)

1. Lúc bấm: tạo Attempt, **đóng băng gói**. Tin tag chưa có id nên không nằm trong gói.
2. `chat.gui(body, traLoi)` qua hàng gửi sẵn có, `Idempotency-Key = attempt.key`.
3. Thành công thì `goiAi(lenh, prompt = tachLoiNho(body), logical_id = attempt.key, trigger_message_id, gói)`.
4. Bước 3 lỗi: hàng riêng người gọi dưới tin tag, câu từ `LOI_GOI_AI`, có **Thử lại** và **Bỏ**.
   Thử lại dùng cùng key và cùng gói, nên máy chủ phát lại chứ không trả 409.
5. Cặp đang chờ chỉ nằm trong bộ nhớ. App bị tắt giữa chừng: `MenuTin` trên tin tag của chính mình
   mà chưa có lời gọi liên kết có mục «Nhờ Rủ Đi AI trả lời tin này», mở sheet chip trước.
6. Bỏ đoạn chặn `goiMoHinh` (`GroupChatLive.tsx:85-87`, `:361-366`). Khay «Tờ hẹn» chèn `/plan ` vào
   ô soạn; «Tự tạo kèo» giữ nguyên.

### 4.2 `create()` ở máy chủ, thêm vào thứ tự hiện có

1. **`kiemTrigger`**: `SELECT id, author_id, kind, created_at, deleted_at FROM messages WHERE id=$1
   AND context_id=$2` (không `body`). Cần: tác giả là người gọi; `kind='text'`; chưa xoá; ≤24 giờ.
   Sai → 422 `trigger_khong_hop_le`. Máy chủ **không** so `prompt` với chữ tin tag: nó không đọc được
   chữ đó, và ADR-0036 §2.4 đã nói vì sao không cần.
2. **Kiểm chuỗi** cho lượt `trongLuong`: CTE đệ quy trên `messages(id, reply_to_id)`, ≤6 bước, chỉ id.
   Lệch → `boi_canh_mismatch`.
3. **Suy lane** từ kết quả `authority` (§3.1) [P1-5].
4. **Digest** = `sha256(command\0prompt\0gói\0trigger)`.
5. **Hạn phòng**: ≤3 lời gọi `queued|running` → 429 `invocation_room_busy`; ≤30 mỗi phòng mỗi giờ →
   429 `invocation_room_rate_limited`. Hạn 8/phút/người giữ nguyên.
6. Vi phạm `chat_ai_one_answer_per_trigger` với `logical_id` khác → 409 `invocation_trigger_taken`.
7. Lưu `so_tin_doc` = số lượt `thuocPhong` đã xác nhận.

Mọi câu đọc mới (bước 1, 2 và câu đọc `card` ở §4.3) phải hiện trong cổng đọc xuyên gói của lát 5,
có allowlist theo gốc; `khong_doc_chat` mở ra `internal/**` (sửa theo phản biện) [P1-13][P2-5].

### 4.3 Worker

- **Lát 7 (còn đi brain).** `plan` (kể cả `@Rủ Đi`, vẫn ánh xạ sang `plan`) đi `companion-reply` và
  `GroundCard` như hôm nay; thẻ kết quả thành **một** phần của `tra_loi`. `chia_bill` giữ `chiabill.go`,
  đăng `tra_loi` với `[text(theChiaBill), expense_draft{so_khoan}]`.
- **Lát 9 (engine Go).** `hoi` và `plan` chạy `aiharness.Engine.Run` (ADR-0037).
  - Ý định đóng của nhóm: `chat_answer, plan, find_places, chia_bill_draft, app_help`; `poll_draft` tắt.
  - `lenh=plan` ép ý định `plan`; `lenh=chia_bill` ép `chia_bill_draft`; `lenh=hoi` để Understand chọn
    1–3 ý định. Đây là ý định «hỏi chuyện thường» người dùng muốn có trong nhóm.
- **Understand là một lời gọi, đầu ra toàn enum và slot (theo hợp đồng chung; sửa theo phản biện)
  [P1-8].** Đầu ra đã kiểm vào prompt kế tiếp **bên trong** `<du_lieu nguon="hieu">` và qua guard thêm
  một lần. Không câu viết lại tự do nào đi tiếp.
- **Toolset nhóm** lọc từ registry duy nhất `aiharness/tools` qua `quyen.golden.json`: `search_places`,
  `get_place`, `list_destinations`, `nearest_area`, `group_snapshot`, `list_group_outings`,
  `search_app_manual`, `propose_places`, `propose_itinerary` (`draft_poll` tắt).
- **Nhóm không bao giờ được** `recall_memory`, `remember_fact`, `forget_fact`, `set_reminder`,
  `my_upcoming_outings`, `explain_screen`, `suggest_screen` (sửa theo phản biện) [P1-1]. Tool trí nhớ
  chỉ tới được từ gốc `scope=me`. Nhóm không có trí nhớ dài hạn; luật «quên là xoá cứng» thuộc Nếp
  (ADR-0041).
- **Kiểm trước, nói sau, không trễ thêm (sửa theo phản biện) [P2-9].**
  - Phần `places`/`itinerary` sinh từ tool nháp. Tool nháp kiểm với ledger ngay lúc chạy, trước bước
    sinh chữ cuối. Vậy `phan` phát trước chữ, và chữ không hứa một thẻ về sau bị bỏ.
  - Fast path (`find_places` một ý định): phần `places` dựng từ đúng các hàng retriever trả về.
  - `GroundReply` chạy lại lúc publish để phòng thủ; phần rớt ở đó thì thẻ cuối không có nó, và `xong`
    là thứ client tin.
- **Chuỗi hỏi tiếp đọc `card`, không đọc `body` (sửa theo phản biện) [P2-15].** Với lượt `vai:"ai"`,
  máy chủ đọc `messages.card` của chính hàng đó (`kind='ai_card' AND author_id IS NULL AND
  context_id=$1 AND id = ANY($2)`). Được đủ câu trước (không cắt 300 rune) và client không bịa được.
  Phòng v2 không có `card` rõ nên dùng chữ trong gói.
- **`chia_bill` publish đúng một lần (sửa theo phản biện) [P1-11][P2-12].** Tách `chiaBillParts()`
  thuần khỏi `processChiaBill`; worker publish một lần ở cuối. Ở lát 9, tám lời gọi tuần tự trên
  `gemini-2.5-flash` thành **một** lời gọi flash-lite gộp trong Go, tính vào trần [P1-25][P2-10].
- **Trần gọi model**: một hằng `MaxModelCallsPerTurn` trong `aiharness/llm`, đếm cả retry, mục tiêu
  p95 ≤4 (theo hợp đồng chung) [P1-6]. Bot nhóm không có bộ đếm riêng.
- **Mã guard không lộ ra phòng (sửa theo phản biện) [P1-3].** Hàng DB và frame phòng chỉ thấy mã chung
  `ai_tu_choi`. Câu hỗ trợ tự hại đi riêng tới người gọi qua khoá stream của người gọi, không ghi Postgres.

### 4.4 `publish()` và trả lời vào tin AI

- `CreateMessage{Kind:"ai_card", Card: tra_loi, ReplyToID: &trigger_message_id}`.
- Tin tag bị xoá giữa chừng: trigger DB đã huỷ job, nên `SELECT … FOR UPDATE` ở `worker.go:252`
  không thấy hàng `running` và `publish` về không (nhánh có sẵn). Phòng nhận `huy`.
- `xong{message_id}` chỉ phát **sau** khi transaction commit (thiết kế 01 §2).
- `laTraLoiAi(target)` trong `routes/messages_wai.go`: đúng khi `kind='ai_card'`, `AuthorID == nil`,
  `card.kind == 'tra_loi'`. Hàm chạy **trước** `CheckReplyTarget` (dòng 95). Bình chọn, tờ hẹn, thẻ
  cũ vẫn nhận 422.
- `CheckReplyTarget` và golden Python không sửa. Đây là ngoại lệ Go-only có tên với ADR-0021 §2.2.2:
  hàng `tra_loi` chỉ sinh từ worker Go, Python không bao giờ có, nên parity không chạm được nhánh này.
- `messagePreview` (`messages_wai.go:765-802`) thêm nhánh `tra_loi`: «Rủ Đi AI: » cộng đầu phần chữ,
  tổng ≤80 rune (ADR-0021 §2.2.3). Cũng Go-only.
- `actOnMessageIntent` (`:536-548`) giữ hành vi, chỉ sửa comment để trỏ tới `trigger_message_id`.

### 4.5 Stream

- **Latency (theo hợp đồng chung) [P1-16][P2-9].** Sự kiện trạng thái đầu p95 ≤300 ms, đo từ 202 tới
  `trang_thai` đầu trên SSE người gọi. Token chữ đầu p50 ≤2.5 s, p95 ≤5 s. `plan` trọn lượt p95 ≤8 s.
  Client người gọi vẽ hàng «đang đọc» ngay khi nhận 202.
- **Cửa sổ 48 rune là nơi duy nhất sinh `delta` (theo hợp đồng chung; sửa theo phản biện) [P1-4].**
  - Bỏ «rủi ro chấp nhận» của bản gốc (chữ chạy trước guard rồi phát `failed`).
  - Không byte nào tới cả phòng trước khi guard quét nó; người xem thấy chữ chậm hơn model ~48 rune.
  - Vi phạm thì dừng luồng. Phần đã nhả giữ nguyên, **không rút lại**, cộng câu cố định
    `ai_tra_loi_bi_chan`. Tin đăng mang đúng chữ đó.
- **Enum sự kiện đóng của `aistream`** (theo hợp đồng chung): `hello`, `trang_thai{cau}`,
  `phan{kind,json}`, `delta{p,text}`, `lam_lai`, `xong`, `that_bai{code}`, `huy`, `thu_hoi`,
  `ket_noi_lai`, cộng `: ping`. Frame phòng chỉ mang `trang_thai`, `phan`, `delta`, `lam_lai`,
  `xong{message_id}`, `that_bai{mã chung}`, `huy`. Người xem bị rút quyền thì WS tự đóng.
- **Lane v2 cưỡng chế ở writer (sửa theo phản biện) [P1-5].** Hàm chọn khoá duy nhất của
  `aistream.Writer` không bao giờ `XADD` khoá phòng cho job `lane='v2'`. WS không bơm frame `ai` cho
  context lane v2. Canary M6 của bản gốc chuyển xuống writer.
- **Mất Redis**: chữ tạm là tạm. Nguồn sự thật là tin đã đăng, tới qua change feed. Người gọi rơi về
  polling 2 s như hôm nay (`useChatAi.ts:39-41`, thiết kế 02 §5.4); người xem khác không có hàng «đang
  trả lời…», chỉ thấy tin thật khi nó tới. Không mất gì đã ACK (ADR-0031 §2).

### 4.6 Chia bill từ thẻ (lát 14; sửa theo phản biện) [P1-10]

```sql
CREATE TABLE chat_ai_expense_claims (   -- migration riêng, số cấp lúc lên main
  invocation_id uuid NOT NULL REFERENCES chat_ai_invocations(id) ON DELETE CASCADE,
  khoan smallint NOT NULL CHECK (khoan BETWEEN 0 AND 7),
  context_id uuid NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
  claimed_by uuid NOT NULL REFERENCES people(id), claimed_at timestamptz NOT NULL,
  expense_id uuid UNIQUE REFERENCES expenses(id), recorded_at timestamptz,
  PRIMARY KEY (invocation_id, khoan));
```

1. **Ghi khoản này** → `…/khoan/{k}/nhan`. Máy chủ kiểm: người bấm còn trong phòng; job `succeeded`,
   `lenh='chia_bill'`; `k` < số nháp trong `result`. Trả **đúng một** nháp đó cho người nhận. Người
   thứ hai nhận 409 kèm tên hiển thị người đã nhận.
2. Client mở `ChiaBillLive` (`/smart-split/moi/review`, `app/smart-split/[id]/review.tsx`) điền sẵn một
   dòng, người trả, người tham gia. Phần còn lại đi đúng `proposeSplit` và `confirmExpense` hiện có.
   Máy chủ AI không tính phần chia nào: ba luật tiền không đổi.
3. Xác nhận xong → `…/khoan/{k}/ghi {expense_id}`. Trong một transaction, máy chủ kiểm: claim là của
   người gọi; `expenses.context_id` là phòng này; bản 1 trong `expense_versions` có `recorded_by_id` là
   người gọi; `expenses.created_at ≥ claimed_at`; `expense_id` chưa gắn khoản khác. Đúng hết thì
   `jsonb_set` thêm `k` vào `phan[j].da_ghi` (mẫu `promotion.go:217`); change feed vẽ lại thẻ.
   Dấu «Đã ghi vào sổ» **suy từ dòng chi tiêu thật**; không có nút «đánh dấu xong».
4. Xoá tài khoản: `claimed_by` đi qua **một trigger Go trên `people.deleted_at`** (ADR-0041), phủ mọi
   bảng Go có cột người, có test Postgres liệt kê cột [P1-14][P2-8]. `da_ghi` không mang id người.

### 4.7 E2EE v2 (lát 20, chỉ chat-lab)

1. `authority` cho phòng v2; máy chủ đặt `lane='v2'`. Id tin tag và id trong gói là sequence chat v2,
   kiểm bằng bản v2 của `thuocPhong` trên `chat_v2_events`.
2. Kết quả niêm ở `result` như của Nếp; `delta` chỉ tới khoá lời gọi của người gọi.
3. Phòng chỉ nhận metadata «Rủ Đi AI đang trả lời…»; kênh cho nó là thiết kế 02 §11.4.
4. Máy người gọi dựng `tra_loi`, mã hoá ở epoch hiện hành, đăng `in_reply_to` sequence tin tag, rồi gọi
   `/delivered{sequence}`. Máy chủ kiểm người gửi sequence là người gọi rồi xoá `result`.
5. Người khác gọi `/receipt`, so `result_digest` với sha256 của thẻ đã giải mã. Lệch thì thẻ vẽ như tin
   của chính người gửi, kèm «Tin này tự nhận là của Rủ Đi AI nhưng không khớp».
6. Người gọi offline quá cửa sổ 15 phút: `result` bị xoá; phòng thấy «Rủ Đi AI chưa gửi được câu trả lời.».

## 5. Client: chip, hiển thị, câu chữ

- **`nhac-ai.ts`**: `timNhacAi(text)` nhận NFC/NFD, `@rủ đi | @ru di | @rudi` ở ranh giới từ ở bất kỳ
  vị trí nào (không khớp khi `@rudi` nằm giữa một địa chỉ email), `/plan`, `/chia-bill` ở đầu tin. `tachLoiNho(body)` bỏ mention
  ở mọi vị trí; lời nhờ rỗng dùng câu mặc định.
- **Chip** một dòng ngay trên nút gửi (giữ ADR-0036 §2.5). Số tin luôn đọc từ chính đối tượng gói.
  «Xem» mở `Sheet` liệt kê đúng gói đó (testID `chat-boi-canh*` chuyển sang đây). «Chỉ gửi lời nhờ» là
  một chạm và **không gửi lượt nào, kể cả chuỗi hỏi tiếp**.
- **Gói mặc định**: 20 lượt gần nhất, cộng tối đa 6 lượt chuỗi, trong trần 40.
- **`TraLoiAi.tsx`**: trích tin tag ở trên; chữ là bong bóng căn trái; `places`/`itinerary` là tờ
  `ToGiay` anh em (`TheAi.tsx:41`), không thẻ lồng thẻ. Ký ở chân: `sparkles 15` + `caption ai`, «Rủ Đi
  AI · đọc {n} tin» hoặc «Rủ Đi AI · chỉ đọc lời nhờ». Không mặt Nếp (ADR-0036 §2.6). Mention trong
  bong bóng in đậm 600 bằng mực của bong bóng; tông `ai` sẽ trượt cổng tương phản trên theme.
- **Hàng tạm** ở đầu mới nhất của danh sách, bị thay khi `message_id` tới. Câu «Rủ Đi AI đang đọc {n}
  tin…», sau 8 s «Rủ Đi AI vẫn đang nghĩ…». Không animation từng token, không vòng xoay
  (`DESIGN.md:1292`); shimmer tắt dưới Reduce Motion.
- **UI làm bằng `/impeccable`**, ở nơi có skill (session cloud này không có) [P2-17]. Ràng buộc từ
  `.impeccable/surfaces/chat.md` và DESIGN.md: không màu, font, bo góc mới; AI ký ở chân; nhịp
  100/200/300/550; `useMotion`; chờ bằng chữ, không toast; vùng chạm 48dp.

**Câu chữ đi cùng commit với hành vi** (mẫu ADR-0036 §5):

| Chỗ | Câu mới |
|---|---|
| Chip | «Kèm {n} tin gần đây · Xem · Chỉ gửi lời nhờ» |
| Chip, có chuỗi | «Kèm luồng {k} tin và {n} tin gần đây · Xem · Chỉ gửi lời nhờ» |
| Chip sau «Chỉ gửi lời nhờ» | «Chỉ gửi lời nhờ, không kèm tin nào · Kèm lại {n} tin» |
| Chip, nhóm trống | «Nhóm chưa có tin nào, chỉ gửi lời nhờ» |
| Đầu sheet «Xem» | «Rủ Đi AI sẽ đọc đúng {n} tin dưới đây cùng lời nhờ trong tin của bạn. Lời nhờ và câu trả lời hiện cho cả nhóm.», cộng câu sẵn có về ảnh, sticker, tên |
| AI chưa sẵn sàng | «Rủ Đi AI chưa sẵn sàng · Gửi như tin thường» |
| `LENH` | `@Rủ Đi` → «Hỏi Rủ Đi AI ngay trong nhóm» |
| Người xem, thất bại | «Rủ Đi AI chưa trả lời lời nhờ này.» |

- Mã mới cần câu trong `LOI_GOI_AI` cùng commit (`cau-chu-goi-ai.test.mjs` gác): `trigger_khong_hop_le`,
  `invocation_trigger_taken`, `invocation_room_busy`, `invocation_room_rate_limited`; lát 14 thêm mã
  nhận/ghi khoản.
- Tài liệu sửa cùng commit: `apps/mobile/docs/chat-ui-contract.md:26-29`, `:150-152`;
  `.impeccable/surfaces/chat.md:43` («lời nhờ trong ô» thành «lời nhờ trong tin @Rủ Đi và đúng gói
  hiện trên chip trên nút gửi»); DESIGN.md thêm mục «Trả lời của Rủ Đi AI trong luồng».

## 6. Giới hạn

| Hạng mục | Trần | Khi chạm |
|---|---|---|
| Lời gọi đang chạy mỗi phòng | 3 | 429 `invocation_room_busy` |
| Lời gọi mỗi phòng mỗi giờ | 30 | 429 `invocation_room_rate_limited` |
| Lời gọi mỗi người | 8/phút (giữ nguyên) | 429 `invocation_rate_limited` |
| Lời gọi model mỗi lượt | `MaxModelCallsPerTurn` (thiết kế 01 §5) | `ai_het_ngan_sach` |
| Tuổi tin tag | 24 giờ | 422 `trigger_khong_hop_le` |
| Lượt trong gói | 40, chuỗi ≤6 tính trong đó | 413 `boi_canh_qua_lon` |
| Phần trong thẻ | 3, mỗi loại một | bỏ phần thừa |
| Chữ trả lời | đề xuất 1500 rune, Lead chốt | cắt ở ranh giới câu |
| Khoản `chia_bill` | 8 (`maxLuotDocChi`, `chiabill.go:41`) | «và N khoản nữa» |

## 7. Hỏng hóc

| Tình huống | Hành vi |
|---|---|
| Tin tag gửi được, `goiAi` lỗi | Hàng riêng người gọi, Thử lại cùng key |
| App tắt giữa cặp | Mục «Nhờ Rủ Đi AI trả lời tin này» trong `MenuTin` |
| Bấm hai lần, hai `logical_id` | Một 202, một 409 `invocation_trigger_taken` |
| Tin tag bị xoá trước hoặc giữa job | Trigger DB huỷ, xoá chữ; phòng nhận `huy` |
| Người gọi rời nhóm giữa chừng | `chat_ai_membership_revoked` huỷ; SSE người gọi `thu_hoi` |
| Output guard chặn giữa chừng | Giữ phần đã nhả, cộng câu cố định; không rút lại |
| Mọi phần rớt kiểm | `invalid_ai_result`; phòng thấy câu thất bại chung |
| Worker chết | Theo ADR-0038: chạy lại chỉ khi `first_token_at IS NULL`, phòng nhận `lam_lai`; đăng vẫn một lần |
| Redis mất | Không có chữ chạy; tin thật vẫn tới qua change feed |
| Phòng v2 trước lát 20 | 409 `encrypted_invocation_required` như hôm nay |
| Injection trong một lượt gói | Guard gắn `nghi_chi_thi`, bọc `<du_lieu>`; Understand không chở chữ tự do đi tiếp |

## 8. Cổng và test

- **Node (`apps/mobile`)**: mới `nhac-ai`, `chip-boi-canh` (số tin lấy từ gói), `tra-loi-ai`; mở rộng
  `ai-boi-canh` (mặc định 20; chuỗi; «Chỉ gửi lời nhờ» gửi 0 lượt) và `cau-chu-goi-ai`.
- **`go test ./...`**: bảng `GroundReply`; `laTraLoiAi`; frame phòng; `khong_doc_chat` vẫn xanh và
  `daThay` thấy các câu đọc mới.
- **`scripts/go_postgres_tier.sh`** (skip là đỏ):
  - tin tag ở phòng khác, của người khác, đã xoá, quá 24 giờ → 422;
  - hai lần tạo đồng thời trên một tin tag → một 202, một 409; `publish` trích đúng tin tag;
  - xoá tin tag → huỷ và xoá chữ; rút membership giữa stream;
  - trả lời vào `tra_loi` → 201, vào bình chọn → 422; phòng v2 không bao giờ ra `lane='legacy'`;
  - đua nhận khoản; `ghi` với chi tiêu phòng khác hoặc do người khác ghi → từ chối.
- **Parity**: thêm vào `parity/scenarios/wai/messages-flow.yaml` hai ca: «tin @Rủ Đi là chữ thường,
  không intent» và «trả lời vào thẻ bình chọn bị từ chối».
- **`python3 scripts/check_route_ownership.py`**: mọi route mới có hàng `GO-ONLY`.
- **Maestro**: sửa `40-ai-plan.yaml`, `_30-ai-co-khoa.yaml`. Mới `49-rudi-ai-trong-luong.yaml`: chip →
  Xem → gửi → hàng «đang đọc» → câu trả lời có trích → trả lời vào AI → câu trả lời thứ hai. Sau flow,
  harness mở WS như thành viên thứ hai và đếm frame `ai`. **Mở từng ảnh chụp ra nhìn**: sáng, tối,
  Reduce Motion.
- **Eval**: thêm ≥8 ca vào file riêng `services/core/internal/aieval/testdata/corpus/nhom-trong-luong.json`
  (thiết kế 06 §1; file lõi 16 ca đóng băng làm nền M0): hỏi đáp thường; chữ + quán; hỏi tiếp trong
  chuỗi; `/plan` phải ra lịch trình; chỉ lời nhờ; injection trong lượt gói; mention giữa câu; đòi
  chuyển tiền bị từ chối. Cổng lát 9: lõi ≥14/16 ổn định qua 5 lần (theo hợp đồng chung); ca mới ≥6/8.
  Lời gọi thật cần Lead duyệt số lượng.

## 9. Canary và đột biến

**Canary** (đỏ đúng chỗ dự đoán, ca identity xanh)
1. Bỏ chip → flow 49 đỏ ở khẳng định `Kèm .* tin`.
2. Job `lane='v2'` → sau cả lượt, `XRANGE` khoá phòng rỗng; đo ở writer, không ở tầng đọc [P1-5].
3. Fact trí nhớ Nếp đã seed không có trong request nào của nhóm (bộ ghi request của thiết kế 01) [P1-1].
4. Câu vi phạm guard để lại 0 byte trong khoá phòng và 0 frame `ai` [P1-4].

**Đột biến tự nghĩ** (kiểm tương đương trước; mỗi cái đỏ đúng bước dự đoán)
- M1 bỏ trigger khỏi digest → ca «hai lần tạo» nhận 200 thay 409.
- M2 `publish` không đặt `ReplyToID` → ca «trích đúng tin tag» đỏ.
- M3 đếm chip từ `tinHien.length` → `chip-boi-canh` đỏ.
- M4 `laTraLoiAi` nhận mọi `ai_card` → ca trả lời bình chọn (Postgres và parity) đỏ.
- M5 «Chỉ gửi lời nhờ» vẫn gửi chuỗi → `ai-boi-canh` đỏ.
- M6 writer ghi khoá phòng cho lane v2 → canary 2 đỏ.
- M7 `create` đặt `lane='legacy'` bất kể phòng → ca Postgres phòng v2 đỏ.
- M8 `ghi` bỏ kiểm `recorded_by_id` → ca «do người khác ghi» đỏ.

## 10. Lát, theo số của kế hoạch

| Lát | Phần của nhóm | Phụ thuộc |
|---|---|---|
| 1 | Hàng `chatassist` loại `GO-ONLY` trong manifest; `core routes --json` phát `chatassist.Routes()` | – |
| 2 | Đường nền M0 của eval nhóm: 16 ca × 5 lần | – |
| 5 | Cổng đọc xuyên gói thấy `kiemTrigger`, CTE chuỗi, câu đọc `card` | – |
| 7 | Lõi trong luồng, còn đi brain: migration, `reply_to`, `tra_loi` một phần, `laTraLoiAi`, hạn phòng, chip, gửi theo cặp, `TraLoiAi`, `tacGiaTin`, ADR-0039 và mọi câu chữ | 1 |
| 9 | `hoi` trên engine Go, nhiều phần, chuỗi (`ban:2`), Understand, `chia_bill` một lời gọi; cổng ≥14/16 | 5, 7, 8 |
| 11 | SSE người gọi; hàng «đang đọc» | 9, 10 |
| 12 | Frame `ai` cho người xem; UI hoàn thiện bằng `/impeccable` | 11 |
| 14 | Nhận khoản, `ChiaBillLive` điền sẵn, dấu suy từ chi tiêu thật | 7 |
| 18 | 👍/👎 trên `tra_loi` (ADR-0042 sở hữu bảng) | 12 |
| 19 | Gỡ `companion-reply` cùng mã Go gọi brain và hàng manifest, một commit | 18 |
| 20 | E2EE v2 ở chat-lab | 12 |

## 11. Việc còn mở

1. **Trần chữ nhóm**: đề xuất 1500 rune; Lead chốt.
2. **Phong bì frame phòng** (`tin`, `so_tin`) chưa có trong bảng hợp đồng; thiết kế 02 §11.2 nêu cùng
   điểm. Đề xuất ghi vào `03-ai-engine-hop-dong.md` khi Lead ký.
3. **Ai được nhận một khoản**: mọi thành viên (đề xuất), hay chỉ người trả trong nháp.
4. **Tuổi tin tag 24 giờ**: là đề xuất, chưa có số đo.
5. **Dấu «Đã ghi vào sổ» ở phòng v2**: chưa thiết kế, vì thẻ không còn rõ để `jsonb_set`.
6. **`draft_poll`**: tắt tới khi có kind thẻ bình chọn nháp; muốn bật thì sửa hợp đồng và ADR.
