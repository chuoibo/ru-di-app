# POST /contexts/{context_id}/votes

votes · core · trạng thái trong bộ nhớ: không có

## Mục đích

F17: mở một cuộc bỏ phiếu trong một nhóm: một câu hỏi, 2 đến 20 lựa chọn, tuỳ chọn gắn một chuyến đi của chính nhóm đó. Người tạo là actor. Không có hạn chót, không có trường nào đặt kết quả hay người bỏ phiếu; phiếu chỉ đóng bằng `POST /votes/{vote_id}/close`.

## Xác thực và quyền

Thứ tự từ ngoài vào (đo trên stack tham chiếu):

1. `IdempotencyMiddleware`, chỉ khi có `Idempotency-Key`: rỗng hoặc dài hơn 255 → 422 `invalid_idempotency_key` **trước** routing và xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`); key đã dùng cho request khác → 422; đã có kết quả → replay (`services/api/app/api/idempotency.py:404-502`).
2. FastAPI giải mã JSON body trước dependency: JSON hỏng → 422 `json_invalid` kể cả khi không có danh tính (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`):
   - `dev`: thiếu `X-Actor-ID` → 401, kể cả khi path không phải UUID (`anonymous_non_uuid_context`); id không phải UUID → 422 `invalid_actor_id` (`owner_actor_id_not_uuid`); role lạ → 422 `invalid_actor_roles` (`owner_roles_unknown`).
   - `prod`: `Authorization: Bearer` (`deps.py:131-140`, `bearer_token` `:93-107`), bỏ qua `X-Actor-*`.
4. Path `context_id` **và** body được validate cùng lúc, lỗi gom vào một mảng 422, lỗi path đứng trước (`stranger_non_uuid_and_bad_body`). Bước này chạy trước handler, nên người lạ gửi body sai nhận 422 chứ không 403 (`stranger_unknown_context_bad_body`).
5. `_require_permission("create_vote", actor, {"is_group_member": is_member(...)})` (`services/api/app/api/service.py:2758-2762`; bảng quyền `services/api/app/domain/permissions.py:266-269`; đánh giá `:654-678`):
   - không có role `group_admin` hoặc `member` → 403 `role_not_permitted` (`owner_roles_empty`, `owner_roles_guest`);
   - không có membership ACTIVE với `left_at` null (`services/api/app/api/repository.py:2769-2782`) → 403 `is_group_member`. Nhóm không tồn tại cũng là 403 `is_group_member`, không 404 (`stranger_unknown_context`). Người được mời chưa nhận, người đã rời, người đã xoá tài khoản đều rơi vào nhánh này.
   - Không có `extra_roles`: `group_admin` ở đây chỉ là claim trong header, không đọc từ membership. Ở `dev`, `X-Actor-Roles: group_admin` một mình vẫn qua (`owner_roles_group_admin_only` → 201).
6. `outing_id` khác null (`service.py:2763-2772`): không có chuyến đi → 404 `outing_not_found`; chuyến đi thuộc nhóm khác → 422 `outing_not_in_context`. Chạy **sau** kiểm quyền: người lạ gửi `outing_id` lạ nhận 403 (`stranger_unknown_context_unknown_outing`).

## Đầu vào

- Path `context_id`: UUID lax của pydantic (chữ hoa, không gạch, `{...}`, `urn:uuid:` đều parse được; `stranger_uppercase_unknown_context` → 403). Sai dạng → 422 `uuid_parsing`.
- Header: `Idempotency-Key` (1..255) tuỳ chọn; `Content-Type`. Không có `Content-Type` thì vẫn parse JSON (`no_content_type_is_parsed_as_json` → 201); `text/plain` → 422 `model_attributes_type`.
- Body `VoteCreateRequest` (`services/api/app/api/schemas.py:660-671`), `extra="forbid"` (`schemas.py:66-67`), thứ tự trường:
  1. `question: StrictStr`, `min_length=1`, `max_length=300`, rồi `strip()`; rỗng sau strip → `value_error` "Value error, question must not be blank". Độ dài kiểm trên **chuỗi thô, trước strip** (`question_padded_past_limit`: một khoảng trắng + 300 ký tự → 422).
  2. `options: list[VoteOptionInput]`, `min_length=2`, `max_length=20`.
  3. `outing_id: UUID | None = None`.
- `VoteOptionInput` (`schemas.py:640-657`), `extra="forbid"`:
  - `label: StrictStr`, 1..200, strip (cả tab: `owner_strips_whitespace`), rỗng sau strip → `value_error` "Value error, label must not be blank".
  - `place_name: StrictStr (≤200) | None = None`, strip, rỗng sau strip thành `null`.
- Không kiểm trùng nhãn (`owner_duplicate_labels` → 201). Độ dài đếm theo code point.

## Đầu ra

- **201** `VoteResponse` (`schemas.py:686-700`), thứ tự khoá `id`, `context_id`, `outing_id`, `created_by_id`, `question`, `created_at`, `closed_at`, `is_closed`, `options`, `total_ballots`, `leading_option_ids`, `is_tie`, `decided_option_id`, `my_option_id` (dựng ở `_wire_vote`, `service.py:925-963`).
- `options[]` (`VoteOptionResultResponse`, `schemas.py:678-683`): `id`, `position`, `label`, `place_name`, `ballot_count`. `position` là chỉ số theo thứ tự gửi, bắt đầu từ 0 (`repository.py:3010-3020`); đọc lại `ORDER BY position` (`repository.py:2367-2372`).
- Phiếu vừa tạo: mọi `ballot_count` 0, `total_ballots` 0, `leading_option_ids` `[]`, `is_tie` false, `decided_option_id` null, `my_option_id` null, `closed_at` null, `is_closed` false.
- `question`, `label`, `place_name` là giá trị **đã strip**.
- `created_at`: `_now()` của Python (`service.py:2783`, `:406-407`) truyền tay vào cột (`repository.py:3006`; cột có `server_default now()` ở `services/api/app/db/models.py:1684-1686` nhưng không dùng). Wire: `YYYY-MM-DDTHH:MM:SS.ffffffZ` (pydantic, UTC, 6 chữ số; microsecond đúng bằng 0 thì pydantic bỏ phần lẻ).
- Không có float.
- Replay idempotency: cùng 201 và cùng body, thêm `idempotency-replayed: true`, `content-length`, `content-type` (`idempotency.py:585-599`).
- Framework: 307 cho `/` cuối (`location: http://<Host>/contexts/<id>/votes`); PUT và DELETE → 405 `{"detail":"Method Not Allowed"}` với **`allow: POST`**, dù GET cùng path có tồn tại (Starlette chỉ nêu route đầu tiên khớp path).

## Tác dụng phụ

- SELECT `memberships` (quyền), `outings` khi có `outing_id` (`repository.py:2868-2870`).
- INSERT `votes` (`created_at = now`, `closed_at`/`closed_by_id` null) rồi flush, INSERT một dòng `vote_options` cho mỗi lựa chọn (`repository.py:2991-3022`). Ràng buộc: CHECK `question <> ''`, CHECK `(closed_at IS NULL) = (closed_by_id IS NULL)` (`models.py:1651-1663`); UNIQUE `(vote_id, position)`, CHECK `position >= 0`, CHECK `label <> ''` (`models.py:1700-1705`).
- Commit trước khi gửi response (`services/api/app/api/unit_of_work.py:41-72`).
- Idempotency: POST có key → đặt chỗ trong `idempotency_keys`, chỉ lưu câu trả lời 2xx; 403/404/422 nhả key (`idempotency.py:504-546`). Scope là digest bearer, hoặc chuỗi `X-Actor-ID` thô, hoặc `anonymous` (`:447-449`). Fingerprint gồm method, path, query và body JSON đã chuẩn hoá (`idem_replay_reordered_keys` → replay).
- Không có limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session` / `Session is not valid` (prod) | `deps.py:143`, `:102-106`; `service.py:4465`, `:4468` |
| 422 | `invalid_actor_id` | `X-Actor-ID must be a UUID` | `deps.py:147` |
| 422 | `invalid_actor_roles` | `X-Actor-Roles contains an unknown role` | `deps.py:152-154` |
| 403 | `permission_denied` | `role_not_permitted` hoặc `is_group_member` | `service.py:502-504` |
| 404 | `outing_not_found` | `Outing does not exist` | `service.py:2764-2766` |
| 422 | `outing_not_in_context` | `Outing does not belong to this context` | `service.py:2767-2772` |
| 422 | `invalid_idempotency_key` | `Idempotency-Key must be 1..255 characters` | `idempotency.py:432-439` |
| 422 | `idempotency_key_reuse` | `Idempotency-Key was already used for a different request` | `idempotency.py:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

- 422 framework đã đo (`{"detail":[{"type","loc","msg"[,"ctx"]}]}`, không có `input`, `services/api/app/api/main.py:319-351`): `json_invalid`, `uuid_parsing` (path và `outing_id`), `too_short`/`too_long` (`options`), `list_type`, `model_attributes_type` (phần tử `options`, body mảng, `text/plain`), `missing` (`question`, `options`, `label`, cả body), `string_too_short`, `string_too_long`, `string_type`, `value_error` (có `ctx: {"error": {}}`), `extra_forbidden` (body và trong phần tử `options`).
- Lỗi của middleware ghi bằng `json.dumps` mặc định nên có khoảng trắng sau `:` và `,` (`idempotency.py:602-618`); lỗi route thì gọn.

## Mã Python

- Route: `services/api/app/api/routes/votes.py:31-43`
- Service: `services/api/app/api/service.py:2752-2785` (`create_vote`), `:925-963` (`_wire_vote`), `:475-504` (`_require_permission`)
- Repository: `services/api/app/api/repository.py:2991-3022` (`create_vote`), `:2367-2389` (`_vote_record`), `:2769-2782` (`is_member`), `:2868-2870` (`get_outing`)
- Domain: `services/api/app/domain/vote.py:20-63` (`tally`), `services/api/app/domain/permissions.py:266-269`
- Schema: `services/api/app/api/schemas.py:640-700`
- Model: `services/api/app/db/models.py:1642-1756`

## Test đang phủ

- `services/api/tests/postgres/test_votes_postgres.py`: `test_a_member_creates_three_options_and_reads_them_back_in_sent_order` (285), `test_a_vote_attached_to_an_outing_reads_back_its_outing_id` (328), `test_an_outing_from_another_group_is_rejected_without_cross_group_link` (342), `test_a_missing_outing_is_reported_as_not_found` (372), `test_a_stranger_can_neither_create_read_nor_ballot` (663), `test_vote_creation_rejects_invalid_option_counts_and_blank_questions` (843), `test_a_vote_lifecycle_never_changes_any_money_table` (873), `test_vote_tables_have_no_foreign_key_into_money_tables` (902)
- `services/api/tests/domain/test_vote_tally.py` (37-193): tally của phiếu vừa tạo
- Không có ca nào trong `tests/api/` gọi route này (fake repository).

## Kịch bản parity

`parity/scenarios/w2/votes/POST-contexts-context_id-votes.yaml`, id `w2/votes/post-contexts-context_id-votes` (91 bước):

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context` (401), `anonymous_malformed_json` (422 trước 401), `anonymous_empty_idempotency_key` (422 middleware trước 401), `stranger_non_uuid_context`, `stranger_uppercase_unknown_context`, `stranger_unknown_context` (403, không 404), `stranger_unknown_context_bad_body` (422 trước 403), `stranger_non_uuid_and_bad_body` (hai lỗi gom một), `stranger_unknown_context_unknown_outing` (403 trước 404).
- Ma trận thành viên: `stranger_real_group`, `stranger_real_group_own_outing`, `invitee_not_yet_active`, `leaver_after_leaving`, `ghost_after_deletion` (403); `mate_creates` (201).
- Đường vui: `owner_two_options`, `owner_strips_whitespace`, `owner_twenty_options`, `owner_duplicate_labels`, `owner_with_outing`, `owner_outing_null`, `question_at_limit`, `no_content_type_is_parsed_as_json`, `owner_lists_what_was_stored` (đọc lại).
- Chuyến đi: `owner_unknown_outing` (404), `owner_outing_of_other_group` (422).
- Role và danh tính: `owner_roles_group_admin_only`, `owner_roles_member_only` (201), `owner_roles_empty`, `owner_roles_guest` (403), `owner_roles_unknown`, `owner_actor_id_not_uuid` (422).
- Validate body: `one_option`, `zero_options`, `twenty_one_options`, `options_not_a_list`, `options_items_not_objects`, `options_missing`, `question_missing`, `question_empty`, `question_blank`, `question_too_long`, `question_padded_past_limit`, `question_number`, `label_blank`, `label_empty`, `label_too_long`, `place_name_too_long`, `label_number_and_missing`, `extra_field_closes_at`, `option_extra_field_id`, `outing_id_not_uuid`, `text_plain_content_type`, `empty_body`, `top_level_array`.
- Framework: `trailing_slash_redirects` (307), `put_not_allowed`, `delete_not_allowed` (405, `allow: POST`).
- Idempotency: `idem_first`, `idem_replay`, `idem_replay_reordered_keys`, `idem_reuse_different_body`, `idem_second_distinct_key`, `idem_refusal_not_stored` + `idem_same_key_after_refusal` (403 nhả key), `idem_validation_refusal_not_stored` + `idem_same_key_after_validation_refusal` (422 pydantic nhả key), `idem_outing_refusal_not_stored` + `idem_same_key_after_outing_refusal` (404 không lưu, chạy lại ra 404), `idem_key_too_long`, `mate_same_key_as_owner` (scope theo actor).
- `prod`: `parity/scenarios/w2/votes/prod-auth.yaml` (`anonymous_create`, `owner_creates_vote_with_empty_roles_header`).

## Chưa phủ / lưu ý cho bản Go

- `prod` được viết trong `w2/votes/prod-auth` và đã so trên cặp stack `prod` mới dựng (3 lần `parity run` lane DB bật, `differences=0`; canary bắt đủ); ở `prod` roster luôn cấp `member` (`repository.py:3629`) nên `role_not_permitted` không tới được, và `X-Actor-Roles: ""` không đổi gì.
- 409 `idempotency_request_in_flight` cần hai request đồng thời cùng key; harness chạy tuần tự.
- Strip: `str.strip()` của Python bỏ mọi khoảng trắng Unicode theo `str.isspace` (kể cả `\x1c`-`\x1f`, `\x85`, NBSP). `strings.TrimSpace` của Go không coi `\x1c`-`\x1f` là khoảng trắng. Kịch bản chỉ phủ dấu cách và tab.
- Độ dài: `min_length`/`max_length` đếm code point trên chuỗi **chưa strip**; Go phải dùng `utf8.RuneCountInString` và kiểm trước khi strip, rồi mới kiểm "blank".
- Thứ tự lỗi pydantic: theo thứ tự khai báo trường (`question`, `options`, `outing_id`; trong phần tử: `label`, `place_name`), mọi lỗi của một request gom một mảng. `value_error` mang tiền tố `Value error, ` và `ctx: {"error": {}}`.
- 405 trên path này nói `allow: POST` dù có GET; bản Go tái tạo đúng header đó hoặc mở ADR.
- `outing_not_in_context` cho biết một id chuyến đi có tồn tại ở nhóm khác (khác 404 `outing_not_found`); là hành vi đo được, không phải lựa chọn của bản Go.
- `created_at` là đồng hồ Python; phiếu tạo trong cùng một microsecond sẽ xếp theo `id` ngẫu nhiên ở route list. Harness không tạo được hai request cùng microsecond.
- Microsecond bằng 0: pydantic ghi `...:SSZ` không phần lẻ; normaliser coi đó là shape `f0`, bản Go phải làm giống (xác suất gặp trong kịch bản xấp xỉ 10^-6 mỗi mốc, không cố ý phủ).
