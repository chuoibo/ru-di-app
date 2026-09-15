# GET /g/{token}/khong-phai-toi

guests · core · trạng thái trong bộ nhớ: không có

## Mục đích

Trang "Tôi không phải `<tên>`" (spec mục 8.6). Người đọc nói link này không phải của mình. Trang cho thấy **ít hơn** trang chính: tên người được ghi và người đã ghi, không có số tiền, không hỏi người đọc là ai (`services/api/app/web/objection_view.py:81-98`). Nút gửi là một form `POST` cùng path.

## Xác thực và quyền

Không có actor; token là capability. Thứ tự (đọc từ mã, kịch bản đo):

1. Router: đuôi `/` → 307 tuyệt đối (`not_me_trailing_slash`); HEAD → 405 `allow: GET` (`head_not_me`). `POST` là route khác (xem card `POST-g-token-khong-phai-toi`).
2. Validation token → 422 JSON (`validation-422.yaml`, `not_me_page_token_*`).
3. `not_me_view` (`services/api/app/api/service.py:6734-6738`) → `_objection_envelope` (`:6725-6732`) → `get_guest_envelope` (khoá và ghi như `GET /g/{token}`). Không có hàng → trang `guest_link_broken.html` 404 (`unknown_token_not_me`).
4. `_require_permission("view_guest_envelope", …)` luôn qua.
5. `build_not_me_view` → `_guard` (`objection_view.py:72-78`): `link_state != "active"` → `ObjectionError("LINK_NOT_ACTIVE")` → **409 JSON** `{"code":"LINK_NOT_ACTIVE","detail":"Objection page is not renderable"}` (`service.py:6736-6738`; `guest_b_not_me_page_after`).

## Đầu vào

- Path `token`. Query bị bỏ qua (`not_me_query_ignored`). Không đọc header.

## Đầu ra

**200** `text/html; charset=utf-8`, mẫu `guest_not_me.html` với context `view`, `preview`, `token` (`services/api/app/api/routes/guests.py:97-119`). `view` gồm đúng bốn khoá (`objection_view.py:88-94`):

- `claimed_person_display_name`, `recorded_by_display_name` (như trang chính: nhiều người ghi thì `Người tạo đợt`);
- `already_reported` = `bool(envelope.get("not_me_reported"))`: repository không bao giờ đặt khoá này, nên trên GET **luôn False**;
- `can_object` = `objections_used < objections_allowed`, với `objections_used` là số audit `guest_objection.not_me` **và** `guest_objection.wrong_amount` trên **cả link** (`services/api/app/api/repository.py:6688-6697`, `objection_view.py:93`).

Mẫu (`services/api/app/web/templates/guest_not_me.html`):

- nhánh `already_reported` (`:24-38`) chỉ tới được từ POST;
- nhánh còn lại: eyebrow "Link này ghi tên X", tiêu đề "Nếu bạn không phải X, bấm bên dưới."; form `POST /g/<token>/khong-phai-toi` với nút "Tôi không phải X" khi `can_object` (`:51-57`), ngược lại "Bạn đã báo nhiều lần rồi. Nhắn trực tiếp cho `<người ghi>` sẽ nhanh hơn." (`guest_a_not_me_page_quota_used`);
- footer luôn có `<a href="/g/<token>">Quay lại</a>`.

Escape, byte đầu `<!doctype html>`, không newline cuối và ba header riêng tư giống `GET /g/{token}`. Cả hai tên người gửi trong kịch bản đều chứa markup, nháy, `&`, `+`, `=`, backtick, RTL mark và emoji.

## Tác dụng phụ

Như `GET /g/{token}`: khoá `FOR UPDATE` bốn bảng; `guest_links.first_opened_at = now` nếu NULL; `status='expired'` khi đã quá hạn. Chỉ commit ở 200. Ở 409 `LINK_NOT_ACTIVE` mọi thứ rollback, **kể cả chuyển `expired`**: trang chính lưu chuyển trạng thái đó, trang này thì không.

## Lỗi

| Status | Thân | Nguồn |
|---|---|---|
| 404 | trang `guest_link_broken.html` | `service.py:6726-6728`; `main.py:313-314` |
| 409 | `{"code":"LINK_NOT_ACTIVE","detail":"Objection page is not renderable"}` | `objection_view.py:77-78`; `service.py:6736-6738` |
| 409 | `FORBIDDEN_FIELD_IN_INPUT` / `FIELD_NOT_ALLOWED` (không tới được) | `objection_view.py:75-76`, `:96-97` |
| 422 | validation token | `routes/guests.py:28-31`; `main.py:319-351` |
| 405 | `{"detail":"Method Not Allowed"}` | Starlette |
| 307 | `location` tuyệt đối | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/guests.py:97-119`
- Service: `services/api/app/api/service.py:6725-6738`
- View model: `services/api/app/web/objection_view.py:72-98`
- Repository: `services/api/app/api/repository.py:6516-6722`
- Mẫu: `services/api/app/web/templates/guest_not_me.html`

## Test đang phủ

- `services/api/tests/api/test_guest_objections.py`, lớp `TestNotMe`: `test_the_page_loads_instead_of_404` (38), `test_it_shows_less_than_the_main_page_never_more` (43), `test_it_never_asks_the_reader_who_they_are` (51); `TestNoPageClaimsSomeoneWasTold` (171)
- `services/api/tests/api/test_guest_link_broken.py` (56), `test_guest_privacy_headers.py` (110)

Repository giả; không chứng minh khoá hay quy tắc rollback.

## Kịch bản parity

`parity/scenarios/w5/guests/GET-g-token-khong-phai-toi.yaml`, id `w5/guests/get-g-token-khong-phai-toi` (27 bước, `dev`):

- `guest_a_opens_not_me` (ghi `first_opened_at`), `guest_a_opens_not_me_again`, `not_me_query_ignored`;
- ba lần phản đối số tiền (`guest_a_objects_amount_1..3`) → `guest_a_not_me_page_quota_used` (không còn nút) → `guest_a_not_me_after_quota` (POST 429);
- `guest_b_opens_not_me` → `guest_b_says_not_me` → `guest_b_not_me_page_after` (409) → `guest_b_home_revoked` (200 trạng thái thu hồi);
- `head_not_me`, `not_me_trailing_slash`, `unknown_token_not_me`.

Corpus 422 sinh tự động: hoãn, `carries ['pattern']`.

## Chưa phủ / lưu ý cho bản Go

- Link hết hạn không phủ (không có bước đồng hồ). Bản Go phải rollback chuyển `expired` khi trả 409 ở trang này.
- `can_object` ở đây là hạn mức **cả link** (not_me + wrong_amount), khác với `can_object` theo từng nghĩa vụ trên trang chính và trang sai số tiền.
- Nhánh `already_reported` của mẫu chết trên GET; đừng "sửa" bằng cách đọc audit `not_me`, vì link đã thu hồi thì GET đã là 409.
- 409 là JSON với mã viết hoa, không phải trang HTML.

## Lỗi Python (chỉ báo, không sửa)

- `build_not_me_view` đọc khoá `not_me_reported` mà repository không bao giờ đưa; `already_reported` trên GET luôn False.
- Hạn mức cả link: ba lần thắc mắc số tiền của **một** nghĩa vụ làm mất nút "Tôi không phải X", và trang nói "Bạn đã báo nhiều lần rồi" với người chưa từng bấm nút đó. Điều này ngược với lý do tách hạn mức theo nghĩa vụ ghi ở `service.py:6878-6906`.
- Người đọc mở lại trang sau khi đã báo thì nhận JSON 409 tiếng Anh viết hoa thay vì một câu cho người đọc.
