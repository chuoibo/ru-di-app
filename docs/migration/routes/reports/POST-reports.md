# POST /reports

reports · core · trạng thái trong bộ nhớ: không có

## Mục đích

Ghi một báo cáo của người gọi về một người hoặc một nội dung (ADR-0023 §2.4). Trả về id và thời điểm, không bao giờ trả lại ghi chú, và không kiểm mục tiêu còn tồn tại hay không.

## Xác thực và quyền

Thứ tự từ ngoài vào:

1. `IdempotencyMiddleware` (nếu có header `Idempotency-Key`): key rỗng hoặc dài hơn 255 → 422, trước routing và xác thực (`services/api/app/api/idempotency.py:432-439`).
2. FastAPI giải mã JSON body trước khi giải dependency: JSON hỏng → 422 `json_invalid`, kể cả khi ẩn danh (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`): `prod` đọc Bearer và bỏ qua `X-Actor-*` (`:131-140`); `dev` thiếu `X-Actor-ID` → 401, id sai → 422, role lạ → 422, contexts sai → 422 (`:142-163`).
4. Validate body (pydantic) → 422.
5. `_require_permission("file_report", actor, {})` (`services/api/app/api/service.py:4655`): role `member`, không có predicate nào (`services/api/app/domain/permissions.py:224`). Thiếu `member` → 403 `role_not_permitted`.
6. `validate_report` (`services/api/app/domain/reports.py:42-63`): trên thực tế luôn qua, vì pydantic đã thu hẹp kiểu và độ dài.
7. INSERT. Người gọi **chưa có dòng `people`** thì khoá ngoại `fk_reports_reporter` (`services/api/app/db/models.py:2491-2495`) nổ khi flush → 500 (không kiểm trước như `_require_registered_person`).

Không có 404 ở bất kỳ đâu: `target_id` không được tra (`services/api/app/api/routes/reports.py:30-40`, `models.py:2464-2467`).

## Đầu vào

- Không tham số path hay query.
- Header: `Idempotency-Key` tuỳ chọn; `Content-Type` (thiếu thì vẫn parse JSON; `text/plain` → 422 `model_attributes_type`).
- Body `ReportCreateRequest` (`services/api/app/api/schemas.py:1022-1029`), `extra="forbid"` (`schemas.py:66-67`), thứ tự trường:
  1. `target_type`: `Literal["person","post","message","comment","story"]`, phân biệt hoa thường.
  2. `target_id`: `UUID` (pydantic lax: nhận chữ hoa, 32 hex không gạch; số → `uuid_type`).
  3. `reason`: `Literal["spam","harassment","inappropriate","impersonation","other"]`.
  4. `note`: `StrictStr` tối đa 500 **code point**, hoặc `null`, mặc định `null`. Độ dài đo trên chuỗi **thô**; domain `strip()` sau đó, chuỗi rỗng thành `null` (`domain/reports.py:55-62`).

## Đầu ra

- 201 `ReportResponse` (`schemas.py:1032-1037`), thứ tự khoá `id`, `created_at`. Route khai `status_code=201` (`routes/reports.py:24-29`), không có nhánh 200.
- `id`: `uuid.uuid4()` sinh ở Python (`services/api/app/api/repository.py:4758`).
- `created_at`: đồng hồ Python `_now()` = `datetime.now(UTC)` (`service.py:406-407`, `:4672`), truyền tay vào INSERT; `server_default now()` của cột (`models.py:2500-2502`) không được dùng. Pydantic ghi `Z` và 6 chữ số phần giây, **giữ số 0 cuối** (`.070000Z`); khi micro giây đúng bằng 0 thì bỏ hẳn phần giây.
- Không có float.
- Replay idempotency: cùng 201 và cùng body (cùng `id`, cùng `created_at`), thêm `idempotency-replayed: true`.
- Framework: 307 cho `/reports/`, 405 + `allow: POST` cho GET.
- 500: `text/plain; charset=utf-8`, body `Internal Server Error` (`services/api/app/api/guest_privacy.py:43-65`, đăng ký ở `services/api/app/api/main.py:272`).

## Tác dụng phụ

- INSERT một dòng `reports` (`repository.py:4747-4768`), `note` đã strip. CHECK trên `target_type`, `reason`, `length(note) <= 500` (`models.py:2472-2483`).
- Commit trước khi gửi response (`services/api/app/api/unit_of_work.py:41-48`).
- Idempotency: có key thì đặt chỗ trong `idempotency_keys`, lưu câu trả lời 201, nhả key khi lỗi (`idempotency.py:504-546`). Scope là digest bearer, hoặc `X-Actor-ID` thô, hoặc `anonymous`.
- Không có limiter, không trigger.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` (prod) | `deps.py:143`, `:102-106` |
| 401 | `authentication_required` | `Session is not valid` (prod) | `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem `deps.py:147`, `:152-154`, `:159-163` | |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:433-438` |
| 409 | `idempotency_request_in_flight` | `An earlier request with this key has not finished. Retry with this same key; sending a different one would write it twice` | `idempotency.py:481-493` |
| 500 | (text/plain) | `Internal Server Error` | FK người báo cáo |

- Không tới được qua HTTP: `ApiProblem(422, refused.code.lower(), "Báo cáo chưa hợp lệ.")` (`service.py:4662-4665`) với `unknown_target_type`, `unknown_reason`, `note_too_long`, `note_not_text`. Pydantic đã chặn trước, và strip chỉ làm chuỗi ngắn đi.
- 422 framework, không có `input` (`main.py:318-351`), ví dụ đã đo: `literal_error` kèm `ctx.expected` `'person', 'post', 'message', 'comment' or 'story'`; `string_too_long` kèm `ctx.max_length: 500`; `uuid_parsing` kèm `ctx.error` ``invalid character: found `k` at 1``; body `{}` cho ra ba lỗi `missing` theo thứ tự trường.
- Lỗi middleware có khoảng trắng: `{"code": "...", "detail": "..."}` (`idempotency.py:602-618`).

## Mã Python

- Route: `services/api/app/api/routes/reports.py:24-41`
- Service: `services/api/app/api/service.py:4649-4674` (`create_report`)
- Repository: `services/api/app/api/repository.py:4747-4768`
- Domain: `services/api/app/domain/reports.py:27-63`, `services/api/app/domain/permissions.py:224`
- Model: `services/api/app/db/models.py:2456-2502`

## Test đang phủ

- `services/api/tests/api/test_sessions_and_blocking.py::test_a_report_keeps_the_note_off_the_answer` (154), `::test_a_report_outside_the_vocabulary_is_refused_at_the_wire` (178, tham số `target_type`, `reason`, note 501 ký tự)
- Chỉ có tầng fake repository; không có ca `tests/postgres` nào cho `POST /reports`, nên khoá ngoại người báo cáo (nhánh 500) chưa có test nào ghim.

## Kịch bản parity

`parity/scenarios/w1/reports/POST-reports.yaml`, id `w1/reports/post-reports` (46 bước):

- Xác thực và thứ tự: `anonymous_valid_body` (401), `anonymous_malformed_json` (422 trước 401), `anonymous_with_key_is_released` (401 dù có key), `ghost_reporter_has_no_person_row` (500).
- Đường vui, cả 5 `target_type` và 5 `reason`: `person_spam_no_note`, `post_harassment_note_trimmed`, `comment_impersonation` (note `null`), `message_inappropriate_whitespace_note` (note toàn khoảng trắng), `story_other_about_self`.
- Biên độ dài note: `note_500_ascii` (201), `note_501_ascii` (422), `note_500_plus_trailing_space` (422, đo trước khi strip), `note_500_emoji_code_points` (201, 2000 byte UTF-8), `note_501_emoji_code_points` (422), `note_number` (422).
- Validate: `target_type_unknown`, `target_type_wrong_case`, `reason_unknown`, `target_id_not_uuid`, `target_id_number`, `missing_target_id`, `extra_field_reporter_id`, `empty_object`, `empty_body`, `text_plain_content_type`; nhận: `target_id_uppercase`, `target_id_unhyphenated`.
- Role: `roles_guest_only`, `roles_empty` (403), `roles_unknown` (422).
- Framework: `get_not_allowed` (405), `trailing_slash_redirects` (307).
- Idempotency: `idem_first_write`, `idem_replay_same_bytes`, `idem_replay_reordered_keys` (JSON chuẩn hoá → replay), `idem_reuse_different_body`, `idem_second_distinct_key`, `idem_same_key_other_actor` (scope khác → ghi mới), `idem_raw_body_first` + `idem_raw_body_respaced_is_reuse` (không có content-type JSON → băm bytes thô → reuse 422), `idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_key_too_long`, `idem_empty_key`.

## Chưa phủ / lưu ý cho bản Go

- `prod` chưa phủ (harness chưa có persona `prod`): 401 bearer, scope `bearer:<sha256>`.
- 409 in-flight cần request đồng thời; harness tuần tự.
- Harness không so DB, nên ghi chú **đã lưu** không được so. Đây là bẫy thật: `str.strip()` của Python bỏ cả `\x1c`..`\x1f` và các khoảng trắng Unicode, còn `strings.TrimSpace` của Go thì không bỏ `\x1c`..`\x1f`. Ghi chú lưu có thể khác mà HTTP vẫn EQUAL.
- Độ dài note: đếm rune (`utf8.RuneCountInString`) trên chuỗi **chưa** strip; không đếm byte, không đếm UTF-16. PostgreSQL `length()` cũng đếm ký tự.
- Nhánh 500 là hành vi hiện tại, không phải hợp đồng mong muốn. Bản Go trả 409 `person_not_registered` sẽ lệch parity; đổi thì mở ADR, đừng vá lặng lẽ.
- `created_at`: 6 chữ số phần giây, giữ số 0 cuối, hậu tố `Z`; micro giây bằng 0 thì không có phần giây. `time.RFC3339Nano` của Go **cắt** số 0 cuối, nên sẽ lệch.
- UUID trong body: pydantic nhận chữ hoa, 32 hex, `{...}`, `urn:uuid:`. Thông điệp `uuid_parsing` là chữ của pydantic-core (``invalid character: found `k` at 1``, `invalid group length in group 4: expected 12, found 11`) và phải được chép lại nguyên văn.
