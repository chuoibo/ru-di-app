# PUT /people/me/interests

preferences · core · trạng thái trong bộ nhớ: không có

## Mục đích

Thay toàn bộ câu trả lời khẩu vị và khoảng ngân sách của chính người gọi (không phải patch). `GET /people/me` đọc lại; không route nào đọc câu trả lời của người khác.

## Xác thực và quyền

Thứ tự từ ngoài vào, đo trên FastAPI 0.115.6 trong image:

1. `IdempotencyMiddleware`, chỉ khi có header `Idempotency-Key`: rỗng hoặc dài hơn 255 → 422 trước cả routing và xác thực (`services/api/app/api/idempotency.py:432-439`).
2. FastAPI đọc và giải mã JSON body **trước** khi giải dependency: JSON hỏng → 422 `json_invalid` kể cả khi không có danh tính (kịch bản `anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`):
   - `prod`: `Authorization: Bearer` (`deps.py:131-140`, `bearer_token` `:93-107`), bỏ qua `X-Actor-*`; thiếu hoặc sai dạng → 401.
   - `dev`: thiếu `X-Actor-ID` → 401 (`:142-143`); không phải UUID → 422 (`:144-147`); role lạ → 422 (`:149-154`); `X-Actor-Contexts` sai → 422 (`:156-163`).
4. Validate body (pydantic) → 422 dạng `{"detail":[...]}`.
5. `_require_permission("manage_own_interests", actor, {"is_self": True})` (`services/api/app/api/service.py:4141`): bảng quyền đòi role `member` (`services/api/app/domain/permissions.py:207`). Thiếu `member` → 403 `permission_denied`, detail `role_not_permitted` (`service.py:475-504`, `permissions.py:673-674`). `is_self` luôn đúng nên không bao giờ là lý do.
6. Chuẩn hoá từ vựng (`service.py:4142-4150`): tag trước, band sau.
7. Tìm dòng `people` của người gọi (`service.py:4151-4155`) → 404 nếu chưa đăng ký.

Hệ quả thứ tự: người chưa đăng ký gửi tag lạ nhận 422 chứ không 404 (`ghost_unknown_tag_before_person_lookup`); người lạ gửi JSON hỏng nhận 422 chứ không 401.

## Đầu vào

- Path cố định `/people/me/interests`, không có tham số. Router `preferences` được include sau `people` (`services/api/app/api/main.py:219`, `:234`); `/people/{person_id}` chỉ khớp một đoạn nên không nuốt `me/interests`.
- Header: `Idempotency-Key` (1..255 ký tự) tuỳ chọn; `Content-Type`. Không có `Content-Type` thì FastAPI vẫn parse JSON (`no_content_type_is_parsed_as_json` → 200); `text/plain` thì body là bytes → 422 `model_attributes_type`.
- Body `InterestsUpdateRequest` (`services/api/app/api/schemas.py:895-908`), `extra="forbid"` (`schemas.py:66-67`), thứ tự trường:
  1. `interests: list[StrictStr]`, bắt buộc, được rỗng, được trùng, không có giới hạn độ dài ở schema. Phần tử số hoặc `null` → 422 `string_type`; `null` hoặc chuỗi thay list → 422 `list_type`.
  2. `budget_band: StrictStr | None = None`. Vắng mặt hoặc `null` nghĩa là bỏ qua và **xoá** band đã lưu.
- Tag phải khớp chính xác (phân biệt hoa thường, không trim) một id trong `INTEREST_IDS` (`services/api/app/domain/interests.py:132-152`); band khớp `BUDGET_BAND_IDS` (`:155-164`).

## Đầu ra

- 200 (không bao giờ 201) `MyInterestsResponse` (`schemas.py:884-892`), thứ tự khoá `interests`, `budget_band`.
  - `interests`: đã bỏ trùng và xếp theo **thứ tự từ vựng**, không theo thứ tự gửi hay thứ tự chữ cái (`service.py:4161-4165`; repository trả theo `tag` ASC ở `services/api/app/api/repository.py:3998-4012`, service xếp lại).
  - `budget_band`: chuỗi hoặc `null`.
- Không float, không datetime trong câu trả lời.
- 307 cho đường có `/` cuối (`location: http://<Host>/people/me/interests`); 405 + `allow: PUT` cho GET.
- Replay idempotency: cùng status và body đã lưu, thêm `idempotency-replayed: true`, `content-length`, `content-type` (`idempotency.py:585-599`).

## Tác dụng phụ

- `person_interests`: đọc các dòng hiện có, DELETE những tag không còn muốn, INSERT tag mới theo thứ tự sort, `created_at` là `_now()` của Python truyền tay (`repository.py:4014-4040`; cột có `server_default now()` ở `services/api/app/db/models.py:1049-1051` nhưng không được dùng). Dòng giữ lại giữ nguyên `created_at`. Ràng buộc: UNIQUE `(person_id, tag)`, CHECK tag không rỗng (`models.py:1034-1038`).
- `people.budget_band`: chỉ UPDATE (khoá dòng `FOR UPDATE`) khi khác giá trị đang lưu (`service.py:4157-4160`, `repository.py:3915-3926`).
- Commit trước khi gửi response (`services/api/app/api/unit_of_work.py:41-48`, `:51-72`; `deps.py:196-209`).
- Idempotency: PUT có key thì middleware đặt chỗ trong `idempotency_keys`, chỉ lưu câu trả lời 2xx, trả lời khác 2xx hoặc exception thì nhả key (`idempotency.py:504-546`). Scope là digest bearer, hoặc chuỗi `X-Actor-ID` thô, hoặc `anonymous` (`:447-449`). Fingerprint gồm method, path, query và body đã chuẩn hoá (sort key, bỏ khoảng trắng) khi `content-type` là JSON (`:249-311`).
- Không có limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` (prod) | `deps.py:143`, `:102-106` |
| 401 | `authentication_required` | `Session is not valid` (prod, phiên không tồn tại / hết hạn / bị thu hồi) | `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` | `X-Actor-ID must be a UUID` | `deps.py:147` |
| 422 | `invalid_actor_roles` | `X-Actor-Roles contains an unknown role` | `deps.py:152-154` |
| 422 | `invalid_actor_contexts` | `X-Actor-Contexts must contain comma-separated UUIDs` | `deps.py:159-163` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |
| 422 | `interest_unknown` | `Lựa chọn không nằm trong danh sách máy chủ biết.` | `service.py:4145-4150` |
| 422 | `budget_band_unknown` | cùng câu trên | như trên |
| 404 | `person_not_found` | `Chưa có hồ sơ cho tài khoản này.` | `service.py:4152-4155` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:433-438` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | `An earlier request with this key has not finished. Retry with this same key; sending a different one would write it twice` | `idempotency.py:481-493` |

- Không tới được qua HTTP: `interests_not_a_list`, `interest_not_a_string`, `budget_band_not_a_string` (pydantic chặn trước), `interests_too_many` (`domain/interests.py:150`, không thể xảy ra).
- 422 của framework: `{"detail":[{"type","loc","msg"[,"ctx"]}]}`, **không** có `input` (handler xoá nó, `main.py:318-351`). `json_invalid` có `loc: ["body", <vị trí byte>]` và `ctx.error`.
- Ba lỗi của middleware ghi body bằng `json.dumps` mặc định, nên **có khoảng trắng**: `{"code": "...", "detail": "..."}` (`idempotency.py:602-618`). Lỗi của route thì gọn: `{"code":"...","detail":"..."}` (`main.py:315-316`).

## Mã Python

- Route: `services/api/app/api/routes/preferences.py:66-81`
- Service: `services/api/app/api/service.py:4129-4165` (`set_my_interests`), `:475-504` (`_require_permission`)
- Repository: `services/api/app/api/repository.py:2525-2527` (`get_person`), `:3998-4012` (`list_person_interests`), `:4014-4040` (`set_person_interests`), `:3915-3926` (`update_person_profile`)
- Domain: `services/api/app/domain/interests.py:132-164`, `services/api/app/domain/permissions.py:207`
- Middleware: `services/api/app/api/idempotency.py:404-553`

## Test đang phủ

- `services/api/tests/api/test_interests.py`: `test_answers_round_trip_through_the_profile` (59), `test_unticking_removes_a_taste` (78), `test_choosing_nothing_is_a_supported_answer` (91), `test_a_word_the_server_does_not_know_is_refused` (100), `test_the_refusal_does_not_echo_the_word_back` (116), `test_an_unknown_budget_band_is_refused` (127), `test_a_skipped_budget_clears_a_previous_one` (138), `test_interests_are_not_part_of_anybody_elses_view` (152), `test_the_route_only_ever_writes_the_caller` (176), `test_writing_needs_a_session` (190)
- `services/api/tests/postgres/test_interests_postgres.py`: 72, 96, 114, 158, 167, 177
- `services/api/tests/domain/test_interests.py` (24-121)

## Kịch bản parity

`parity/scenarios/w1/preferences/PUT-people-me-interests.yaml`, id `w1/preferences/put-people-me-interests` (48 bước):

- Thứ tự từ chối: `anonymous_valid_body` (401), `anonymous_malformed_json` (422 trước 401), `anonymous_empty_idempotency_key` (422 middleware trước 401), `ghost_valid_body_no_person_row` (404), `ghost_unknown_tag_before_person_lookup` (422 trước 404).
- Đường vui: `me_dedupes_and_reorders`, `me_every_tag_reversed`, `me_untick_and_skip_budget`, `me_chooses_nothing`, `no_content_type_is_parsed_as_json`, cùng các lần đọc lại `me_profile_*` qua `GET /people/me`.
- Validate body: `missing_interests`, `empty_object`, `empty_body`, `interests_not_a_list`, `interests_null`, `interest_not_a_string`, `interest_null_item`, `budget_band_number`, `extra_field_person_id`, `top_level_array`, `text_plain_content_type`.
- Từ vựng: `interest_wrong_case`, `interest_leading_space`, `budget_band_unknown`, `unknown_tag_and_unknown_band` (mã của tag thắng).
- Danh tính và role: `roles_guest_only`, `roles_empty` (403), `roles_unknown`, `actor_id_not_uuid` (422).
- Framework: `trailing_slash_redirects` (307), `get_not_allowed` (405).
- Cô lập: `mate_writes_only_mate`, `me_profile_unchanged_by_mate`.
- Idempotency: `idem_first_write`, `idem_replay_same_bytes`, `idem_replay_reordered_keys_and_spaces` (chuẩn hoá JSON → replay), `idem_reuse_different_array_order` (thứ tự mảng là nghĩa → reuse 422), `idem_second_distinct_key`, `idem_same_key_other_actor` (scope theo actor), `idem_refusal_is_not_stored` + `idem_same_key_after_refusal_writes` (key được nhả), `idem_key_too_long` (256), `idem_key_at_limit` (255).

## Chưa phủ / lưu ý cho bản Go

- `prod` chưa phủ: harness chưa tạo phiên, nên 401 `Missing bearer session`, các lỗi của `actor_for_session_token` (`service.py:4452`) và scope idempotency `bearer:<sha256>` chưa được so.
- 409 in-flight cần hai request đồng thời cùng key, harness chạy tuần tự nên không tạo được.
- Tài khoản đã xoá (`people.deleted_at` khác null): `get_person` vẫn trả bản ghi nên ghi được. Chưa có kịch bản.
- Harness chỉ so HTTP, không so DB. Việc dòng giữ lại giữ `created_at` và band chỉ UPDATE khi đổi là không nhìn thấy được; `GET /people/me` chỉ đọc lại một phần.
- Hai mức JSON: lỗi middleware có khoảng trắng sau `:` và `,`, lỗi route thì không. Go phải tái tạo cả hai byte-for-byte.
- Fingerprint: phải khớp `json.dumps(sort_keys=True, separators=(",", ":"), ensure_ascii=False)`, kể cả sort key theo code point và cách Python ghi số (`1.0` vẫn là `1.0`, `1e5` thành `100000.0`). Content-type không phải JSON thì băm bytes thô. Có thêm `legacy_fingerprint` (`idempotency.py:459-464`, `:192-207`) để nhận key cũ.
- Scope `dev` là chuỗi `X-Actor-ID` **thô**, không chuẩn hoá: UUID viết hoa và viết thường là hai scope khác nhau.
- Thứ tự trả về là thứ tự từ vựng, không phải `ORDER BY tag`.
- Tag so khớp chính xác theo byte: không casefold, không trim, không chuẩn hoá Unicode.
