# ADR-0021 — Chat có sticker, trả lời, xoá tin và theme nhóm; nhắn riêng là một context hai người; Rủ Đi AI được tự lên tiếng theo nhịp

- **Trạng thái:** 🟡 **ĐỀ XUẤT** 2026-09-06 — chờ Lead đánh ĐÃ CHẤP NHẬN. Lát L1 (chat nâng cấp) và L2 (nhắn riêng) của nhánh `claude/p0-w-m15-social-v1-1` không merge trước khi dòng này đổi. Lead đã duyệt kế hoạch chứa các quyết định dưới đây trong phiên 2026-09-06 (`~/.claude/plans/mellow-waddling-lantern.md`); ADR này là chỗ ghi lại để không hợp thức hoá hậu nghiệm.
- **Quyết định bởi:** Lead (phiên 2026-09-06: chọn toàn bộ mảng chat kể cả theme nhóm — mục Claude không khuyến nghị — ghi lại ở mục 2 và 5).
- **Hiện thực:** nhánh `claude/p0-w-m15-social-v1-1`, lát L1 và L2; kế hoạch `~/.claude/plans/mellow-waddling-lantern.md` mục 5.2–5.3.
- **Thay đổi hình dạng bảng `messages` và `contexts`, thêm ba route, mở rộng ngữ pháp lệnh chat**; không đổi ba luật tiền, không đổi ADR-0015 (không đường thanh toán), không đổi cách phiên đăng nhập hoạt động (ADR-0014).

## 1. Bối cảnh

Chat nhóm (M3, #537) có text, ảnh, sáu phản ứng, `@Rủ Đi` và ba lệnh `/plan` `/chia-bill` `/vote`. Rà soát 2026-09-06 trên `main` cho thấy những thứ người dùng một app nhắn tin sờ vào đầu tiên đều chưa có: không sticker, không trả lời một tin cụ thể, không xoá tin mình vừa gửi nhầm, không đổi tên hay màu cuộc trò chuyện từ app, và **không nhắn riêng được với một người bạn** — muốn nói chuyện hai người phải tạo nhóm hai người và nhóm đó lẫn vào danh sách nhóm tiền.

Về AI: F08 trong `product/feature_list.md` nói «User không nhất thiết phải @AI», nhưng hiện Rủ Đi AI chỉ nói khi được gọi bằng lệnh hoặc mention. Sở thích cá nhân (ADR-0019, M11) đã lưu trên máy chủ và đã vào chấm điểm Khám phá, nhưng chưa vào prompt của companion.

Backend do Claude làm theo uỷ quyền ADR-0016 §2.3; charter không đổi. Lead giao trực tiếp toàn bộ chương trình social v1.1 ngày 2026-09-06.

## 2. Quyết định

### 2.1 Sticker là một từ vựng đóng, vẽ bằng vector
1. `MessageKind` thêm `sticker`. Thân tin (`body`) là **id sticker** thuộc từ vựng đóng do máy chủ giữ ở `app/domain/stickers.py`; client có bản sao `packages/shared/stickers.json` và một component vector (`react-native-svg`) cho mỗi id. Ba nơi phải khớp, có test gốc đối chiếu (mẫu `tests/test_interest_vocabulary_matches_client.py`).
2. Id là slug ASCII `^[a-z0-9-]{1,32}$` (CHECK ở DB), không có số dài để repo guard không đọc nhầm thành số tài khoản. Không có sticker raster, không GIF bên thứ ba, không lấy sticker từ URL.
3. Client gặp id lạ vẽ một ô «?» có nhãn «Sticker», không ném và không rỗng.

### 2.2 Trả lời một tin là một khoá ngoại, không phải trích chữ
1. `messages.reply_to_id` là FK **composite** `(reply_to_id, context_id) → messages(id, context_id)` kèm `UNIQUE(id, context_id)`: cơ sở dữ liệu tự chặn trả lời chéo nhóm, service không cần tin lời khai của client.
2. Chỉ trả lời được tin `text|image|sticker`. Tin trả lời tới một tin không tồn tại hoặc ở nhóm khác nhận **cùng một mã 404** (`message_not_found`); tới tin đã xoá nhận 409.
3. Máy chủ gắn `reply_to {id, kind, author_id, preview}` khi liệt kê để client không phải gọi thêm; preview do máy chủ dựng, ≤ 80 ký tự.

### 2.3 Xoá tin là đổi `kind`, không xoá hàng
1. Người gửi xoá được tin `text|image|sticker` của **chính mình**. Không xoá `ai_card` (poll, bản nháp chi tiêu do máy chủ sinh có thứ khác trỏ vào).
2. Xoá = `kind = 'deleted'`, `body/image_url/card` về NULL, `deleted_at` ghi giờ; hàng giữ lại vì `context_read_marks.last_read_message_id` và `reply_to_id` của tin khác còn trỏ tới. CHECK `(kind = 'deleted') = (deleted_at IS NOT NULL)` và nhánh `deleted` trong CHECK payload là hai cách viết của cùng một luật; schema wire có validator thứ ba.
3. Phản ứng của tin đã xoá bị xoá cùng transaction. Cửa sổ `/chia-bill` và cửa sổ hội thoại gửi mô hình bỏ qua tin `deleted` (nhịp `plan_turn` vẫn đếm). `create_chat_expense_draft` trên tin đã xoá trả 409.
4. **File ảnh không xoá** khi xoá tin `image`: hàng `uploaded_images` là ảnh của nhóm và có thể đang được kỷ niệm hoặc bài đăng trỏ tới. Đây là điều người dùng cần biết khi bấm xoá; câu trên màn nói «Mọi người sẽ thấy “Tin nhắn đã bị xoá”», không hứa ảnh biến mất khỏi máy chủ.

### 2.4 Theme nhóm là năm bộ màu đóng, không phải màu tự do
1. `contexts.theme` với CHECK trong bộ `('mac-dinh','hoang-hon','bien-dem','rung-thong','ruc-ro')`; mặc định `mac-dinh`. Mọi thành viên `active` đổi được (`PATCH /contexts/{id}`), cùng chỗ với đổi tên.
2. Mỗi slug có bảng màu `{bubble, bubbleInk, accent}` cho cả light và dark trong `packages/shared/tokens.json` dưới khoá **`chatTheme`, nằm ngoài `color.*`** để cổng `test_shared_tokens` không đòi biến trong `guest.css`; tương phản `bubbleInk` trên `bubble` ≥ 4.5:1, có test. `mac-dinh` bằng đúng `accent/accentInk` hiện tại nên ảnh của mọi flow cũ không đổi.
3. Theme chỉ chạm bong bóng của người gửi và viền chip phản ứng của mình. Không đổi tông dẫn của màn, không đổi nút gửi, không đổi header (Luật Một Tông Dẫn của DESIGN.md).

### 2.5 Nhắn riêng là một `context` có `kind = 'pair'`
1. `contexts.kind ∈ ('group','pair')`, `pair_key` duy nhất = hai id người theo thứ tự `friendship.pair_key`, CHECK `(kind = 'pair') = (pair_key IS NOT NULL)`. Không có bảng tin nhắn riêng thứ hai: mọi thứ của chat (tin, ảnh, phản ứng, dấu đọc, Rủ Đi AI, theme) dùng lại nguyên vẹn.
2. Chỉ mở được giữa **hai người là bạn** (`FriendRequest.state = 'accepted'`); tình bạn là sự đồng thuận, không có bước «chấp nhận cuộc trò chuyện». `POST /people/{person_id}/dm` trả 201 khi tạo, 200 khi đã có, thân là `ContextSummary` như một hàng của `GET /people/me/contexts` (có `kind`, `counterpart`, `theme`).
3. **Mọi lý do từ chối mở** — không phải bạn, đã chặn, đã xoá tài khoản, không tồn tại — là một câu 404 duy nhất «Chưa thể nhắn riêng với người này.» Cửa này không được là oracle cho quan hệ giữa hai người.
4. Hai membership `active` sinh cùng savepoint với context; đua trên `pair_key` giải bằng unique index rồi đọc lại, không bằng `if exists`.
5. Trong pair **không có** mời, duyệt theo link, đổi vai trò, rời, mời vào kèo: các route đụng roster trả 409 `not_a_group`. Tiền, kèo, bình chọn, kỷ niệm **vẫn cho phép** — sổ cái đã chạy với mọi context ≥ 2 người và chia bill hai người là việc có thật. Màn tiền của app không bao giờ mặc định vào một pair (`chonNhomMacDinh` bỏ qua `kind = 'pair'`).
6. `display_name` của pair lưu rỗng; máy chủ điền tên người kia khi trả về. Pair là «nhóm chung» theo nghĩa `shares_active_context`, nên hai người trong DM thấy avatar nhau — hợp lý vì đã là bạn.

### 2.6 Rủ Đi AI tự lên tiếng khi nhóm hỏi ăn gì, đi đâu — theo nhịp cũ, tắt được theo nhóm
1. `app/domain/chat_intent.detect_ask` nhận diện một tập mẫu đóng («ăn gì», «đi đâu», «quán nào», «chỗ nào», «cafe nào», …) trên tin `text` không có lệnh, không mention. Khi `contexts.ai_auto_suggest` (mặc định bật) còn bật, máy chủ gọi companion với `requested=False`: **nhịp `plan_turn` giữ nguyên** (không nói hai lần liền, cooldown 90 s, tối đa 3 lượt mỗi cửa sổ 20 tin) và dùng chung `message_intent_limiter` với `/plan`. Bị nhịp chặn thì im lặng — không `intent_error`, không 429.
2. Companion nhận thêm `taste` = tổng hợp gu của thành viên active (`group_taste`, ADR-0019 §2.2), chỉ gồm tag có trong từ vựng và band ngân sách; không có gu thì không có khối gu. Trong pair, hai người là «nhóm» nên cùng hàm.
3. Công tắc «Rủ Đi AI tự gợi ý» nằm trong Cài đặt nhóm; tắt là `PATCH /contexts/{id} {ai_auto_suggest:false}`.

## 3. Hệ quả

- Migration `5c1a7e3d9b42` (L1) và `6d2b8f4e0c53` (L2) nối tiếp head `e1f2a3b4c5d6` của `origin/main`; id không nối tiếp dạng xoay vì lane Codex đang dùng trùng id trên cây chưa commit.
- `ck_messages_payload_matches_kind` và `ck_messages_message_kind` được drop/recreate; `ck_messages_human_kinds_have_author` giữ nguyên (tin `deleted` vẫn có tác giả).
- `ContextSummary`, `ContextResponse`, `MessageResponse`, `ContextLastMessage` mở rộng; `_message_preview` có hai bản (repository thật và fake) phải sửa cùng.
- Client: `Sheet` (đang không có ai dùng) trở thành khay sticker, menu nhấn giữ và Cài đặt nhóm; `src/screens/quan-tri/quan-tri.ts` (`roiNhom`, `datVaiTro`) được nối vào màn Thành viên thay vì viết lại.
- Nhãn mà các flow Maestro cũ đang bấm («Gửi ảnh», «Ô soạn tin», «Gửi tin nhắn», «Tin nhắn: …», sáu nhãn phản ứng, «❤️ 1», «Bạn: …») giữ nguyên văn.

## 4. Cái này KHÔNG chứng minh

- Sticker và theme có hợp ý người dùng thật không — chưa có bằng chứng hành vi (ADR-0006 vẫn đúng). Tám sticker là bộ khởi điểm.
- Realtime vẫn là poll 4 giây khi màn đang mở; ADR này không đổi điều đó (thông báo ở ADR-0024).
- Rủ Đi AI tự gợi ý có «hay» không: chỉ có eval tay với khoá thật (`scripts/eval_companion.py`), không có trong CI.
- Race thật khi hai người cùng mở một pair đúng lúc: chỉ được mô phỏng bằng unique index trong tier Postgres.

## 5. Phương án đã bác

| Phương án | Vì sao không |
|---|---|
| Sticker raster/Lottie tải lên hoặc từ bên thứ ba | Binary vào Git đụng repo guard fail-closed; URL ngoài là kênh rò rỉ địa chỉ người đọc; vector theo hướng hình ảnh v2 (ADR-0020) |
| Xoá cứng hàng tin | Vỡ `reply_to_id`, `last_read_message_id` và cursor phân trang; «Tin nhắn đã bị xoá» là hành vi người dùng messenger mong đợi |
| Trích dẫn bằng cách chép chữ vào body | Không theo được khi tin gốc bị xoá; không kiểm được chéo nhóm |
| Màu bong bóng tự do do người dùng chọn | Cổng `test_contrast_floor` và `rudi-khong-hex` không thể canh màu chưa biết; DESIGN.md v2 vừa chốt hệ token. Lead vẫn chọn theme → giải bằng bộ đóng có test tương phản |
| Bảng `direct_messages` riêng | Nhân đôi toàn bộ chat (ảnh, phản ứng, AI, dấu đọc, idempotency) cho cùng một hành vi |
| Door limiter riêng cho AI tự gợi ý | Thêm door là thêm mặt để lọt; nhịp `plan_turn` và `message_intent_limiter` hiện có đã đủ chặn vòng lặp |

## 6. Cách kiểm chứng

- Domain: `tests/domain/test_stickers.py`, `test_message_edit.py`, `test_direct.py`, `test_chat_intent.py` (bảng `detect_ask` có/không).
- Gốc: `tests/test_sticker_vocabulary_matches_client.py`, `tests/test_chat_theme_matches_tokens.py`; `services/api/tests/web/test_chat_theme_tokens.py` (slug CHECK = tokens = `theme.ts`; đủ light/dark; tương phản ≥ 4.5).
- API (fake repo): tin `deleted` không mang nội dung ở wire; sticker lạ 422; trả lời chéo nhóm 404 cùng câu với không tồn tại; xoá tin người khác 403; `/chia-bill` bỏ tin đã xoá; DM: mọi từ chối cùng một 404; từng cửa roster trên pair 409.
- Postgres: FK composite từ chối `reply_to_id` khác context; CHECK từ chối `deleted` có body; `soft_delete_message` xoá phản ứng; `uq_contexts_pair_key` từ chối hàng thứ hai; hai membership cùng savepoint.
- Emulator: flow 37 (sticker → trả lời → xoá), 39 (đổi tên, theme, rời nhóm — đọc lại từ máy chủ sau khi tắt/mở app), 41 (nhắn riêng), 47 (AI tự gợi ý, cần `--ai`), kèm khối «máy chủ xác nhận» và canary bằng curl (người khác xoá tin → 403; theme lạ → 422; mở DM với người chưa là bạn → 404 cùng câu).
- Đột biến phải đỏ: giữ `body` khi xoá → validator wire và CHECK đỏ; nới regex id sticker → test từ vựng đỏ; bỏ `_require_group_kind` ở một cửa roster → ca âm cửa đó đỏ; đổi 404 chung của DM thành 403 → ca «cùng câu» đỏ.
