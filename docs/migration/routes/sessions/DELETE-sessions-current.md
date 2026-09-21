# DELETE /sessions/current

sessions · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Đăng xuất. Nằm ở server chứ không ở client vì lý do thường tình: một phiên mà chỉ cái điện thoại quên đi thì vẫn là một credential sống trên server, và mất điện thoại đúng là lúc điều đó quan trọng (`routes/sessions.py:16-18`).

Route trả lời **giống hệt nhau dù token còn sống hay không** (`service.py:4475-4480`). Người gọi đang giữ một token thật không học được gì, mà người gọi đang đoán cũng không học được gì.

## Xác thực và quyền

**Không có `get_actor`.** Đây là điểm khác biệt đáng nhớ nhất của route: nó không hỏi ai đang gọi, nó chỉ tiêu cái token đi kèm. Thứ tự (`routes/sessions.py:78-92`):

1. Middleware idempotency: `DELETE` nằm trong `WRITE_METHODS` (`idempotency.py:76`), phạm vi `anonymous` vì không có actor.
2. Router: `/sessions/current` được khai báo **trước** `/sessions/{session_id}` nên chữ `current` không bao giờ bị đọc thành một id (`routes/sessions.py:105-107`).
3. `bearer_token(authorization)` (`deps.py:93-107`) → 401 `authentication_required`, detail `Missing bearer session`. Thiếu header, header dị dạng, scheme không phải `bearer`, và token rỗng **trả lời như nhau**. `Bearer` khớp không phân biệt hoa thường vì RFC 7235 nói scheme là như vậy, và một client viết hoa khác đi không phải kẻ tấn công.
4. `revoke_session_token` tra digest; không thấy thì **return im lặng**, thấy thì thu hồi.

Ở chế độ `dev` không có bearer nào cả, nên route này là 401 với mọi thứ gửi lên. Đó là lý do file kịch bản `prod` tồn tại.

## Đầu vào

Không path, không query, không thân. Một header bắt buộc: `Authorization: Bearer <token>`.

## Đầu ra

**204**, không thân. Khai báo `-> Response` chứ không `-> None` là cố ý: FastAPI từ chối dựng router khi một 204 hứa có thân, và hai phiên bản thư viện repo này chạy bất đồng về việc annotation nào tính là lời hứa (`routes/sessions.py:87-90`, gác bởi `tests/api/test_bodyless_status_declarations.py`).

## Tác dụng phụ

Đặt `revoked_at` trên đúng một hàng `account_sessions` (`repository.py:3574`), hoặc không ghi gì khi digest không tra ra hàng nào. Không đụng phiên nào khác của cùng người.

## Idempotency

`DELETE` nằm trong `WRITE_METHODS`. Bản thân hành vi đã luỹ đẳng: đăng xuất hai lần trên cùng token cho cùng một 204.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing bearer session` | `deps.py:102`, `:106` |
| 307 / 405 | — | | Starlette |

Không có 403 và không có 404: cả hai đều sẽ là câu trả lời khác nhau cho token thật và token đoán.

## Mã Python

- Route: `services/api/app/api/routes/sessions.py:78-92`
- Service: `services/api/app/api/service.py:4475-4486`
- Repository: `repository.py:3562` (`get_account_session_by_digest`), `:3574` (`revoke_account_session`)
- Bearer: `services/api/app/api/deps.py:93-107`

## Test đang phủ

`services/api/tests/postgres/test_session_bootstrap_postgres.py`: `test_signing_out_kills_the_session` (527), `test_get_actor_is_the_dependency_every_route_shares` (558).

## Kịch bản parity

`parity/scenarios/w9/sessions/prod-sessions.yaml`, id `w9/sessions/prod-sessions` (18 bước, `prod`) — route này chỉ nói được ở prod:

- Mọi hình dạng header bị từ chối, tất cả một câu trả lời: `anonymous_revokes_current`, `junk_bearer_lists`, `empty_bearer_lists`, `scheme_only_lists`, `basic_scheme_lists`, `lowercase_scheme_lists` (chấp nhận), `actor_headers_without_bearer`.
- Đăng xuất tiêu đúng hàng của request này: `other_revokes_current`, `other_lists_after_logout`, `owner_still_signed_in`.
- Token đã tiêu và token rác cho cùng một câu: `other_revokes_current_again`, `junk_bearer_revokes_current`.
- Framework: `trailing_slash_current`, `patch_current_not_allowed`.

Ở `dev`, bước `word_current_as_session_id` trong `DELETE-sessions-session_id.yaml` cho thấy thứ tự khai báo: chữ `current` tới tay handler này, không tới route id.

Corpus sinh: **không có**, và lý do nằm trong wave `w9` dưới dạng `excluded`. Ở `dev` route này trả 401 cho **mọi** bước, mà `runner.PersonasRefused` (`parity/internal/runner/runner.go:507-519`) đọc một kịch bản mà mọi bước persona đều 401 là một stack không nhận phiên, rồi dừng cả lượt bằng `INFRA`. 401 ở đây là câu trả lời trung thực của route, không phải hạ tầng hỏng. Đã thử: corpus sinh ra lần đầu chạy đúng thành `INFRA`, nên route được chuyển sang `excluded`.

## Chưa phủ / lưu ý cho bản Go

- Bản Go **không được** gọi `get_actor` ở route này. Thêm một actor vào sẽ đổi 401 của một header dị dạng thành một câu trả lời khác và làm lộ thông tin ở nơi Python cố tình im.
- Token không tra ra hàng nào vẫn phải là **204**, không phải 404.
- Thu hồi phải chạm đúng một hàng; `revoke_all_account_sessions` (`repository.py:4625`) là một hàm khác, dùng cho xoá tài khoản.
