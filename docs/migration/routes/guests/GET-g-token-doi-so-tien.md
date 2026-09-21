# GET /g/{token}/doi-so-tien

guests · core · trạng thái trong bộ nhớ: không có

## Mục đích

Trang "Số tiền không đúng" cho **một** nghĩa vụ trên link (spec mục 8.2 và 10.5). Người đọc đồng ý mình là người được ghi tên nhưng thắc mắc con số: chọn lý do trong danh sách đóng, gửi, và/hoặc xin xem phần tính của mình. Trang nói rõ thắc mắc chỉ dừng thu khoản này và thiếu bằng chứng không có nghĩa là người đọc sai.

## Xác thực và quyền

Không có actor; token là capability. Thứ tự (đọc từ mã, kịch bản đo):

1. Router: đuôi `/` → 307 tuyệt đối **giữ query** (`wrong_amount_trailing_slash`); HEAD → 405 `allow: GET` (`head_wrong_amount`).
2. Validation token → 422. Query `obligation_id: str | None` không có kiểm nào.
3. **Không có `obligation_id`** (`services/api/app/api/routes/guests.py:155-162`):
   - `guest_view(token)` (`services/api/app/api/service.py:6711-6723`): token lạ → trang link hỏng 404 (`unknown_token_wrong_amount`);
   - `view["blocks"]` rỗng (link thu hồi, hết hạn) → 409 `no_open_obligation` `Nothing to dispute on this link` (`guest_b_wrong_amount_default_revoked`);
   - lấy `blocks[0]["obligation_id"]`, tức nghĩa vụ có `recipient_id` nhỏ nhất (`guest_a_wrong_amount_default` bằng từng byte `guest_a_wrong_amount_first`).
4. `wrong_amount_view(token, obligation_id)` (`service.py:6740-6746`) nạp phong bì thêm một lần: token lạ → 404 trang (`unknown_token_wrong_amount_with_id`); `_guard` → 409 `LINK_NOT_ACTIVE` (`services/api/app/web/objection_view.py:72-78`; `guest_b_wrong_amount_revoked`).
5. `build_wrong_amount_view` (`objection_view.py:101-135`) so **chuỗi**: `o["obligation_id"] == obligation_id`. Không khớp → 409 `UNKNOWN_OBLIGATION` `Objection page is not renderable`. Nhánh này bắt nghĩa vụ của link khác, chuỗi không phải UUID, chuỗi rỗng (`?obligation_id=` là `""`, không phải None), cách viết `urn:uuid:…` và chữ hoa (`guest_a_wrong_amount_other_link`, `_not_uuid`, `_empty`, `_urn`).

## Đầu vào

- Path `token`.
- Query `obligation_id` tuỳ chọn, chuỗi bất kỳ. Lặp → **giá trị cuối** (`guest_a_wrong_amount_repeated`). Tham số khác bị bỏ qua (`guest_a_wrong_amount_other_param` ra trang mặc định).

## Đầu ra

**200** `text/html; charset=utf-8`, `guest_wrong_amount.html` (`routes/guests.py:163-168`). `view` có đúng chín khoá (`objection_view.py:108-131`):

- `claimed_person_display_name`, `recorded_by_display_name`, `occasion_label`;
- `amount_display` = `format_vnd(amount_vnd)`, nhóm nghìn bằng dấu chấm;
- `obligation_id` = chuỗi query (đã khớp nên luôn là dạng chuẩn);
- `can_object` = số audit `guest_objection.wrong_amount` của **nghĩa vụ này** `< 3` (`not_me` không có nghĩa vụ nên không tính);
- `can_request_evidence` = không có audit `guest_objection.evidence_request` cho nghĩa vụ; `evidence_requested` là phủ định của nó;
- `reasons` = năm cặp đóng (`amount_too_high` "Số tiền cao hơn phần của tôi", `did_not_join` "Tôi không tham gia khoản này", `already_paid` "Tôi đã chuyển rồi", `split_wrong` "Chia sai người", `other` "Lý do khác"), `objection_view.py:57-63`.

Mẫu (`services/api/app/web/templates/guest_wrong_amount.html`):

- tiêu đề "Phần của X trong Y là `<số>`đ";
- `can_object`: form `POST /g/<token>/doi-so-tien` với `obligation_id` ẩn và năm radio, radio đầu `checked` (`:35-80`); ngược lại "Bạn đã gửi thắc mắc nhiều lần rồi…" (`guest_a_wrong_amount_first_locked`);
- thẻ thứ hai: `evidence_requested` → "Yêu cầu của bạn đã được lưu lại…" (`:84-88`; `guest_a_wrong_amount_second_asked`); `can_request_evidence` → form `POST /g/<token>/xin-cach-tinh` (`:89-94`); đoạn 10.5 luôn có;
- footer "Quay lại" về `/g/<token>`.

Evidence và hạn mức tính theo nghĩa vụ: xin cách tính cho nghĩa vụ thứ hai không đổi trang của nghĩa vụ thứ nhất, và ngược lại (`guest_a_wrong_amount_first_not_asked`, `guest_a_wrong_amount_second_open`).

Escape, byte đầu, không newline cuối và ba header riêng tư như `GET /g/{token}`.

## Tác dụng phụ

Như `GET /g/{token}`: khoá `FOR UPDATE` bốn bảng, `first_opened_at` nếu NULL, chuyển `expired` nếu quá hạn. Nhánh không có id nạp phong bì hai lần trong cùng transaction. Chỉ commit ở 200; mọi 409/404 rollback, kể cả `first_opened_at` và chuyển `expired`.

## Lỗi

| Status | Thân | Nguồn |
|---|---|---|
| 404 | trang `guest_link_broken.html` | `service.py:6712-6714`, `:6726-6728`; `main.py:313-314` |
| 409 | `{"code":"no_open_obligation","detail":"Nothing to dispute on this link"}` | `routes/guests.py:157-161` |
| 409 | `{"code":"LINK_NOT_ACTIVE","detail":"Objection page is not renderable"}` | `objection_view.py:77-78`; `service.py:6744-6746` |
| 409 | `{"code":"UNKNOWN_OBLIGATION","detail":"Objection page is not renderable"}` | `objection_view.py:105-106` |
| 409 | `FORBIDDEN_FIELD_IN_INPUT`, `FIELD_NOT_ALLOWED` (không tới được) | `objection_view.py:75-76`, `:133-134` |
| 422 | validation token | `main.py:319-351` |
| 405 / 307 | framework | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/guests.py:148-168`
- Service: `services/api/app/api/service.py:6711-6746`
- View model: `services/api/app/web/objection_view.py:57-135`; `services/api/app/web/guest_view.py:69-80`
- Repository: `services/api/app/api/repository.py:6516-6722`
- Mẫu: `services/api/app/web/templates/guest_wrong_amount.html`

## Test đang phủ

- `services/api/tests/api/test_guest_objections.py`: `TestWrongAmount` (76, 81, 93, 99, 126), `TestObjectingOnALinkWithTwoDebts.test_every_objection_link_names_its_own_obligation` (590), `TestEvidenceRequest` (195), `TestQuota` (244, 265)
- `services/api/tests/api/test_guest_link_broken.py` (56)

Repository giả.

## Kịch bản parity

`parity/scenarios/w5/guests/GET-g-token-doi-so-tien.yaml`, id `w5/guests/get-g-token-doi-so-tien` (44 bước, `dev`). Bữa tối chủ đợt trả (chia ba) và taxi người thứ tư trả (chia ba) làm mỗi link mang hai nghĩa vụ tới hai người, do hai người ghi. Tên chủ đợt, người thứ tư và hai nhãn chứa markup, nháy, `+`, `=`, backtick, RTL mark và emoji.

- `guest_a_home`, `guest_a_wrong_amount_default`, `guest_a_wrong_amount_first`, `guest_a_wrong_amount_second`;
- so chuỗi: `guest_a_wrong_amount_other_link`, `_not_uuid`, `_empty`, `_urn`, `_repeated`, `_other_param`;
- theo nghĩa vụ: `guest_a_asks_evidence_second`, `guest_a_wrong_amount_second_asked`, `guest_a_wrong_amount_first_not_asked`, `guest_a_objects_first_1..3`, `guest_a_wrong_amount_first_locked`, `guest_a_wrong_amount_second_open`, `guest_a_home_disputed`, `guest_b_home`;
- thu hồi: `guest_b_says_not_me`, `guest_b_wrong_amount_default_revoked`, `guest_b_wrong_amount_revoked`;
- `unknown_token_wrong_amount`, `unknown_token_wrong_amount_with_id`, `head_wrong_amount`, `wrong_amount_trailing_slash`.

Cũng mở trang này: `POST-g-token-xin-cach-tinh.yaml` (`guest_a_follows_redirect`, `guest_a_follows_urn_redirect`), `POST-g-token-khong-phai-toi.yaml` (`guest_a_wrong_amount_*_after_revoke`), `validation-422.yaml` (`wrong_amount_page_token_*`).

Corpus 422 sinh tự động: hoãn, `carries ['pattern']`.

## Chưa phủ / lưu ý cho bản Go

- Link hết hạn không phủ (không có bước đồng hồ).
- `obligation_id` là chuỗi so từng byte với dạng chuẩn thường của uuid; đừng parse UUID ở route này, vì parse sẽ biến 409 thành 200 cho `urn:uuid:` và chữ hoa.
- Hai kiểu viết mã trên cùng route: `no_open_obligation` chữ thường từ route, `LINK_NOT_ACTIVE` và `UNKNOWN_OBLIGATION` chữ hoa từ view model.
- Không có id: phải đi qua `guest_view` trước (404 trang, rồi 409 khi không có block) rồi nạp lại.
- Rollback cả `first_opened_at` lẫn `expired` ở mọi 409.

## Lỗi Python (chỉ báo, không sửa)

- Trạng thái lỗi của một trang trình duyệt là JSON 409 tiếng Anh, với hai kiểu viết hoa khác nhau.
- So chuỗi: các route POST nhận `urn:uuid:` và chữ hoa (qua `uuid.UUID`), trang này thì không. `xin-cach-tinh` redirect với giá trị thô nên có thể dẫn tới 409 (xem card đó).
- `?obligation_id=` rỗng là 409 thay vì trang mặc định.
- Link thu hồi: không có id là `no_open_obligation`, có id là `LINK_NOT_ACTIVE`, dù cùng một sự thật.
