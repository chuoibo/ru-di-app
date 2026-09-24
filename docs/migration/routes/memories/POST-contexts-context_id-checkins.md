# POST /contexts/{context_id}/checkins

memories · core · trạng thái trong bộ nhớ: không có

## Mục đích

F46: đánh dấu nhóm đã có mặt ở một địa điểm của catalogue, thành một dòng trên tường kỷ niệm. Body chỉ nêu `place_id`; tên và toạ độ đọc từ bảng `places` và được chụp lại vào dòng, nên người gọi không dời được địa điểm hay bịa toạ độ (`services/api/app/api/service.py:4767-4806`).

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` (`services/api/app/api/idempotency.py:404-553`): key rỗng / quá dài → 422 trước xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`).
2. JSON hỏng → 422 trước xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_context`).
4. Path + body `CheckinCreateRequest` → 422; thắng 403 (`stranger_unknown_context_bad_body`).
5. `_require_permission("post_group_memory", {"is_group_member": …})` (`service.py:4786-4790`; `services/api/app/domain/permissions.py:423-426`): 403 cho nhóm không tồn tại (`stranger_unknown_context`), nhóm thật (`stranger_real_context`), người được mời (`invitee_checks_in`); role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 201 (`owner_roles_group_admin_only`). Quyền trước địa điểm: người lạ với `place_id` không có vẫn 403 (`stranger_unknown_context_unknown_place`).
6. `place_row(place_id)` (`service.py:1241-1247`) không thấy → 422 `place_not_found` (`service.py:4791-4796`): id không có (`mate_unknown_place`), sai hoa thường (`mate_place_wrong_case`), có khoảng trắng đầu (`mate_place_leading_space`). So khớp chính xác, không chuẩn hoá.

## Đầu vào

- Path `context_id` (UUID lax).
- Header: `Idempotency-Key` tuỳ chọn.
- Body `CheckinCreateRequest` (`services/api/app/api/schemas.py:1420-1437`), `extra="forbid"`:
  - `place_id`: `StrictStr` 1..200, bắt buộc (`place_missing`, `place_empty`, `place_number` → 422).
  - `caption`: `str` (không strict) tối đa 2000 code point, nullable (`caption_null` → 201; `caption_number` → `string_type`).
  - `lat`, `lng`, `image_url` → `extra_forbidden` (`extra_field_lat_lng` cho hai lỗi, `extra_field_image_url`).

## Đầu ra

- **201** `MemoryResponse` (`schemas.py:1449-1479`), cùng thứ tự khoá với card `POST /contexts/{id}/memories`.
  - `kind: "checkin"`, `image_url: null`, `place_id` như gửi, `place_name`, `lat`, `lng` từ catalogue. `lat`/`lng` là float JSON theo `repr` của Python (đo: `"lat":11.9512,"lng":108.4451` cho `p-lung-chung-cafe`).
  - `created_at`: `_now()` Python (`service.py:4805`); `cursor`: base64url của `created_at.isoformat()|id` (`services/api/app/api/cursors.py:15-19`).
  - `reaction_count: 0`, `comment_count: 0`, `viewer_has_reacted: false`.
- Cùng địa điểm lần hai là dòng mới (`mate_checks_in_same_place_again`).
- Replay: 201 + body đã lưu + `idempotency-replayed: true`.
- Framework: 307 cho `/` cuối; `GET` → 405 `allow: POST`.

## Tác dụng phụ

- INSERT `memories (kind = checkin, image_url = NULL, place_id, place_name, lat, lng, caption, created_at)` (`services/api/app/api/repository.py:5085-5120`); CHECK `payload_matches_kind`, `lat_range`, `lng_range` (`services/api/app/db/models.py:1810-1830`).
- SELECT `memberships`, `places`.
- Commit trước response.
- Idempotency: chỉ lưu 2xx; 422 `place_not_found` nhả key (`idem_refusal_not_stored` + `idem_same_key_after_refusal`).
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:4786-4790`, `:504` |
| 422 | `place_not_found` | `No place in the catalogue has that id` | `service.py:4792-4796` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `POST /contexts` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

422 framework: `missing`, `string_too_short`, `string_too_long`, `string_type`, `extra_forbidden`, `model_attributes_type`, `json_invalid`, `uuid_parsing`.

## Mã Python

- Route: `services/api/app/api/routes/memories.py:61-80`
- Service: `services/api/app/api/service.py:4767-4806` (`post_context_checkin`), `:1241-1247` (`place_row`), `:832-848`
- Domain: `services/api/app/domain/permissions.py:423-426`
- Repository: `services/api/app/api/repository.py:5085-5120`, `:2769-2782`
- Model: `services/api/app/db/models.py:1759-1869`

## Test đang phủ

- `services/api/tests/postgres/test_group_checkins_postgres.py`: `test_a_member_checks_in_and_the_place_comes_from_the_catalogue` (138), `test_the_request_body_cannot_move_the_place` (185), `test_a_place_the_catalogue_never_heard_of_is_refused` (212), `test_a_stranger_can_neither_read_nor_write_a_groups_checkins` (242), `test_a_checkin_from_another_group_never_appears` (297), `test_photos_and_checkins_share_one_wall_and_can_be_narrowed` (336), `test_the_database_refuses_a_row_that_is_both_kinds_at_once` (399)

## Kịch bản parity

`parity/scenarios/w3/memories/POST-contexts-context_id-checkins.yaml`, id `w3/memories/post-contexts-context_id-checkins` (42 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context`, `anonymous_malformed_json`, `anonymous_empty_idempotency_key`, `stranger_unknown_context`, `stranger_unknown_context_unknown_place` (403 trước 422), `stranger_unknown_context_bad_body` (422 trước 403).
- 403: `stranger_real_context`, `invitee_checks_in`, `mate_roles_empty`.
- Đường vui: `mate_checks_in`, `mate_checks_in_with_caption`, `mate_checks_in_same_place_again`, `owner_roles_group_admin_only`, `caption_null`, `owner_lists_checkins`.
- 422 handler: `mate_unknown_place`, `mate_place_wrong_case`, `mate_place_leading_space`.
- Validate: `place_missing`, `place_empty`, `place_number`, `caption_number`, `extra_field_lat_lng`, `extra_field_image_url`, `top_level_array`.
- Idempotency: `idem_first`, `idem_replay`, `idem_reuse_different_body`, `idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_key_too_long`.
- Framework: `trailing_slash_redirects`, `get_not_allowed`.

`prod` (`w3/memories/prod-auth`): `actor_headers_ignored_checkin` (401), `owner_checks_in` (201).

Chạy với làn DB bật; `cursor` (câu trả lời và `idempotency_keys.response_body`) được bind thành `<b64u:<ts#r|dạng>|<uuid#n>>` như ở card `POST /contexts/{id}/memories` (ADR-0029 §2.4). Corpus 422 sinh tự động: `parity/scenarios/generated/w3-422/post-contexts-context_id-checkins.yaml`, id `generated/w3-422/post-contexts-context_id-checkins` (72 bước; `MAX_STEPS` của bộ sinh đã nâng lên 80).

## Chưa phủ / lưu ý cho bản Go

- `cursor` được bind theo cấu trúc (card `POST /contexts/{id}/memories`): padding, bảng chữ và cách viết thời điểm vẫn được so.
- Float `lat`/`lng`: Python in theo `repr` ngắn nhất (`11.9512`), đọc từ cột `double precision`. Mode `float-lost-point` của canary không áp được vì các toạ độ trong catalogue không có dạng `x.0`; một toạ độ nguyên sẽ là chỗ Go dễ in `11` thay vì `11.0`.
- Độ dài `caption` 2000 tính theo code point; corpus sinh tự động dò các biên độ dài bằng chữ 3 byte, emoji 4 byte và surrogate escape (các bước `caption_len_*`).
- 409 in-flight chưa phủ.

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này nằm trong vùng chạm của `check_go_owned_python_touch.py` do đồ thị gọi so theo tên hàm trần (`ApiService.pair_notebook` trùng tên module `pair_notebook`); mã của route không đọc đồng ý của sổ đôi. Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /contexts/{context_id}/checkins`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
