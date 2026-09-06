# ADR-0023 — Xoá tài khoản là ẩn danh hoá và sổ tiền không đổi một byte; chặn là cạnh bạn bè `blocked`; báo cáo là một bảng

- **Trạng thái:** 🟡 **ĐỀ XUẤT** 2026-09-06 — chờ Lead đánh ĐÃ CHẤP NHẬN. Lát L5 (Settings, xoá tài khoản, chặn/báo cáo, phiên) của nhánh `claude/p0-w-m15-social-v1-1` không merge trước khi dòng này đổi. Lead đã duyệt kế hoạch chứa các quyết định này trong phiên 2026-09-06.
- **Quyết định bởi:** Lead (phiên 2026-09-06: xoá tài khoản và chặn/báo cáo là điều kiện lên App Store / Google Play với nội dung người dùng; ghi lại ở mục 2).
- **Hiện thực:** nhánh `claude/p0-w-m15-social-v1-1`, lát L5; kế hoạch `~/.claude/plans/mellow-waddling-lantern.md` mục 5.6.
- **Thay đổi cách một tài khoản kết thúc, cách hai người không nhìn thấy nhau, và cách tra bạn theo số điện thoại**; không đổi ba luật tiền; không xoá bất kỳ dòng nào trong các bảng tiền; không đổi cách phiên được cấp (ADR-0014, ADR-0016).

## 1. Bối cảnh

Người dùng không xoá được tài khoản, không chặn được ai, không báo cáo được nội dung; không xem được phiên đang đăng nhập ở máy khác. Apple App Store Review Guideline 5.1.1(v) và chính sách UGC của Google Play đòi cả ba. Panel «Cài đặt» hiện tại tự ghi trên màn là «không phải cài đặt production».

Hai điều đã có sẵn định hình quyết định: (a) `FriendRequest.state` đã có giá trị `blocked` và `friendship.py` đã có hành động `BLOCK` với luật `BLOCKED_IS_SILENT` (người bị chặn không phân biệt được với «không có tài khoản»); (b) tài khoản đăng nhập bằng số điện thoại có `person_id = derive_person_id(số)` **xác định** (ADR-0016 §2.2), còn tài khoản Google là `uuid4` kèm hàng `account_identities` — đo trong `verify_otp` và `login_with_google` ngày 2026-09-06.

Luật 3 (số dư tính lại được từ sổ) nghĩa là không được xoá `expenses`, `expense_versions`, `confirmed_allocations`, `collection_*`, `payment_reports`, `receipt_confirmations` — kể cả khi người có tên trong đó rời đi.

Backend do Claude làm theo uỷ quyền ADR-0016 §2.3; charter không đổi.

## 2. Quyết định

### 2.1 Xoá tài khoản là ẩn danh hoá, một transaction, theo một bản đồ đóng
1. `DELETE /people/me {confirm: true}` chạy `erase_person` trong **một** transaction theo bản đồ `app/domain/account_lifecycle.ERASURE`:
   - **xoá**: bài, bình luận, phản ứng, story, lượt xem story, ảnh `owner_person_id` của người đó (hàng và file), sở thích, địa điểm đã lưu, mọi cạnh bạn bè ở mọi trạng thái, `account_identities`, dấu đọc, thiết bị và thông báo (ADR-0024);
   - **thu hồi**: mọi `account_sessions`;
   - **rời**: mọi membership `active`/`invited` → `left` với `left_at`;
   - **ẩn danh**: hàng `people` giữ **id cũ** (khoá ngoại của sổ tiền), `display_name = 'Người dùng đã rời'`, NULL bio/thành phố/band ngân sách, `discoverable_by_phone = false`, `wall_comment_policy = 'nobody'`, `deleted_at` ghi giờ;
   - **giữ**: tin nhắn, kỷ niệm và bình luận kỷ niệm trong nhóm (hiện tên ẩn danh), check-in, bình chọn, kèo, ảnh nhóm người đó đã tải, `contexts.created_by_id`, và **mọi bảng tiền**.
2. Kết bằng một `AuditEvent(event_type='account.deleted', aggregate_type='person')` với `event_data` chỉ chứa số đếm — không tên, không bio, không số.
3. Ca Postgres so `md5` của **từng** bảng tiền trước và sau; khác một byte là đỏ.
4. `get_actor` từ chối phiên của người có `deleted_at` (lớp hai; phiên đã thu hồi là lớp một). `GET /people/{id}` của người đã xoá trả 404; `find_person_by_phone` trả **cùng câu 404** với «không tồn tại».
5. Xoá file ảnh là việc sau transaction, best-effort, ghi log số file; thiếu file không làm request thất bại.

### 2.2 Đăng ký lại cùng số điện thoại tạo một người mới
1. Vì `person_id` của tài khoản OTP suy ra từ số, `verify_otp` gặp hàng `people` có `deleted_at` sẽ **không hồi sinh** hàng đó mà mint một `people.id = uuid4()` mới kèm `account_identities(provider='phone', subject=digest)` trỏ tới người mới — cùng hình dạng với đường Google (`create_person_with_identity`).
2. **`account_identities` là nguồn sự thật cho «số này của ai»**: `find_person_by_phone` tra `get_account_identity('phone', digest)` trước, id suy ra chỉ còn là cách đặt id lần đầu cho tài khoản chưa có danh tính.
3. `PUT /people/{id}` và `POST /identity/person-id` không hồi sinh hàng đã xoá (404 cùng câu).
4. Hệ quả người dùng: đăng ký lại là bắt đầu trắng; nhóm cũ, sổ cũ, tin cũ trỏ vào «Người dùng đã rời», không dính vào người mới. Đây là nghĩa của «xoá» mà ADR này chọn.

### 2.3 Chặn dùng lại cạnh bạn bè
1. Chặn = đặt cạnh `FriendRequest` giữa hai người sang `blocked` (tạo cạnh mới nếu chưa có; `requester_id = decided_by_id = người chặn`). Vì `uq_friend_edge_live` chỉ cho một cạnh sống, chặn tự loại tình bạn `accepted`.
2. Hệ quả, viết ở `app/domain/blocking.py` và trong SQL (hai cách viết):
   - người bị chặn gửi lời mời → 409 `request_not_open`, **cùng byte** với «đã có lời mời» (`BLOCKED_IS_SILENT`);
   - bài `public`/`friends` ẩn **hai chiều** (`_readable_by` thêm `NOT EXISTS` cạnh `blocked`); bài `group` trong nhóm chung **vẫn thấy** — nhóm là của nhóm;
   - story ẩn hai chiều;
   - mở nhắn riêng → 404 cùng câu với «không thể nhắn riêng»; pair đã có → `GET /people/me/contexts` đánh dấu `unavailable: true` và `POST …/messages` trả **409 `direct_message_unavailable`, một mã và một câu cho cả hai nguyên nhân** (bị chặn, hoặc người kia đã xoá tài khoản). Đây là một oracle yếu được chấp nhận có chủ ý: người dùng messenger cần biết «không gửi được», và hai nguyên nhân cùng một câu là mức lộ ít nhất còn hữu dụng;
   - bình luận bài của người đã chặn mình → 404 (bài không đọc được);
   - avatar vẫn theo `shares_a_group_with_subject` (còn nhóm chung thì còn thấy);
   - nhóm chung giữ nguyên: chặn không đuổi ai khỏi nhóm.
3. Người chặn gỡ được: cạnh về `declined` (không tự thành bạn lại). Docstring «BLOCKED là terminal» trong `friendship.py` sửa thành «terminal với người bị chặn; người chặn được gỡ». Chặn lần hai là idempotent.

### 2.4 Báo cáo là một bảng, không có màn quản trị
`POST /reports {target_type ∈ person|post|message|comment|story, target_id, reason ∈ bộ đóng, note ≤ 500}` → 201 chỉ trả `id`, không echo `note`. Không có UI quản trị trong v1; người vận hành đọc bảng. Từ vựng `reason`/`target_type` viết hai nơi (server, client) có test đối chiếu.

### 2.5 Phiên và quyền riêng tư trong Cài đặt
`GET /sessions` liệt kê phiên còn sống của tôi (`id, issued_via, created_at, expires_at, current`) — không có nhãn thiết bị vì bảng không lưu; `DELETE /sessions/{id}` thu hồi một phiên của mình (phiên người khác → 404). `people.discoverable_by_phone` (mặc định `true`) tắt thì tra theo số trả cùng câu 404. Giao diện sáng/tối/hệ thống là tuỳ chọn **trên máy** (AsyncStorage), không lên máy chủ.

## 3. Hệ quả

- Migration `9a5e1c7b3f86`: `people += discoverable_by_phone, deleted_at, notify_prefs`; bảng `reports`.
- `_readable_by` thêm `NOT EXISTS` hai chiều; index `(requester_id, addressee_id, state)` nếu chưa có. Hiệu năng query plan không được chứng minh trong tier hiện tại (ghi ở §4).
- Màn `/settings` thay stub trong `Profile.tsx`, **nhưng panel «Tài khoản» + «Đăng xuất» giữ nguyên** vì subflow `_dang-xuat-neu-co.yaml` của mọi lượt `--otp` đi qua đó.
- Flow chặn và xoá tài khoản dùng một người **E** riêng trong harness để không phá dữ liệu C–D mà các khối kiểm sau bảng đọc.
- Điều khoản sử dụng và chính sách riêng tư trong app là **bản nháp** tiếng Việt có con dấu «BẢN NHÁP», chờ Lead duyệt; ADR này không thay cho tư vấn pháp lý.

## 4. Cái này KHÔNG chứng minh

- Người thật hiểu «xoá tài khoản» ≠ «xoá sổ nợ của nhóm». Câu trên màn nói điều đó; chưa có bằng chứng hành vi.
- Báo cáo được xử lý: không có quy trình vận hành nào sau khi hàng được ghi.
- Tuân thủ pháp luật Việt Nam / GDPR về xoá dữ liệu: cần tư vấn pháp lý; ADR chỉ ghi cơ chế.
- Query plan của `_readable_by` sau khi thêm `NOT EXISTS`: chỉ đo bằng `EXPLAIN` tuỳ chọn trên 1 000 hàng.

## 5. Phương án đã bác

| Phương án | Vì sao không |
|---|---|
| Xoá cứng cascade toàn bộ | Phá luật 3: số dư nhóm không còn tính lại được; khoá ngoại của sổ trỏ vào hàng biến mất |
| Hồi sinh hàng cũ khi đăng ký lại cùng số | Tin, kỷ niệm, sổ cũ đã ẩn danh bỗng mang tên mới — liên kết lại danh tính đã xoá |
| Giữ số điện thoại (hoặc digest) để chặn đăng ký lại | ADR-0016 §2.2 cấm lưu số; digest giữ lại là một cột về người đã xoá |
| Bảng `blocks` riêng | Cạnh bạn bè đã có trạng thái `blocked`, unique một cạnh sống, và `BLOCKED_IS_SILENT` |
| 403 `direct_message_blocked` riêng | Oracle trực tiếp «bạn bị chặn»; gộp với «đã xoá» thành một mã |
| Màn quản trị báo cáo trong v1 | Chưa có vai trò quản trị hệ thống; một màn quản trị là một bề mặt quyền mới cần ADR riêng |
| Xoá file ảnh trong transaction | I/O đĩa trong transaction DB làm transaction dài và không rollback được file; best-effort sau commit + script dọn |

## 6. Cách kiểm chứng

- Postgres `test_delete_account_postgres.py`: dựng đời sống tiền (`_persist_lifecycle`), bài, story, bạn, phiên → `DELETE /people/me` → `md5(string_agg)` của 20 bảng tiền **bằng nhau từng bảng**; `audit_events` +1 và `event_data` không chứa tên/bio/số; các bảng «xoá» = 0 và file avatar không còn trong `PhotoStorage(root=tmp)`; phiên cũ → 401 cùng byte với token bịa; chèn phiên sống tay cho người `deleted_at` → vẫn 401; `balances` nhóm trước/sau bằng; đăng nhập lại cùng số → `person_id` mới, hồ sơ trống, không thấy nhóm cũ, bạn tra số thấy người mới.
- Postgres `test_blocking_visibility_postgres.py`: mọi hệ quả ở 2.3, mỗi cái một ca; «cùng byte» so thân phản hồi.
- API: `GET /sessions` chỉ phiên của tôi; `DELETE /sessions/{id}` phiên người khác 404; `find_person_by_phone` cho người ẩn/đã xoá cùng câu; báo cáo không echo note; `PATCH /people/me` với khoá lạ 422.
- Emulator: flow 44 (Cài đặt: giao diện, quyền riêng tư, phiên; đo pixel ảnh «Tối»), 45 (chặn/báo cáo với người E; canary curl hai chiều), 46 (xoá tài khoản E: về màn chào; token cũ 401; `balances` không đổi; roster hiện «Người dùng đã rời»); canary UI `_canary-dm-bi-chan.yaml` phải đỏ ở bước cuối.
- Đột biến phải đỏ: bỏ thu hồi phiên → ca 401 đỏ; xoá thêm một hàng `expense_versions` → ca md5 đỏ; ghi tên cũ vào `event_data` → đỏ; bỏ `NOT EXISTS` chặn trong `_readable_by` → ca bài public đỏ; đổi mã pair thành hai mã khác nhau → ca «một mã» đỏ.
