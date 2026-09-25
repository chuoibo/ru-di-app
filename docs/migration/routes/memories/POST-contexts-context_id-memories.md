# POST /contexts/{context_id}/memories

memories · core · trạng thái trong bộ nhớ: không có

## Mục đích

Đưa một bức ảnh lên tường kỷ niệm riêng của nhóm (rd-be-07), có thể gắn một địa điểm của catalogue (M12, ADR-0017 §2.4). Tên địa điểm đọc từ bảng `places` phía server, không tin body. Ảnh chỉ được nêu bằng đường dẫn tương đối vào kho ảnh của **chính nhóm này**.

## Xác thực và quyền

Thứ tự (đo trên stack tham chiếu):

1. `IdempotencyMiddleware` (`services/api/app/api/idempotency.py:404-553`): key rỗng / quá dài → 422 trước xác thực (`anonymous_empty_idempotency_key`, `idem_key_too_long`).
2. JSON hỏng → 422 `json_invalid` trước xác thực (`anonymous_malformed_json`).
3. `get_actor` (`services/api/app/api/deps.py:110-164`) → 401, thắng lỗi path (`anonymous_non_uuid_context`) và lỗi body (`anonymous_invalid_body`).
4. Path + body `MemoryCreateRequest` → một mảng 422, path trước (`stranger_non_uuid_and_bad_url`); 422 pattern thắng 403 (`stranger_unknown_context_bad_url`).
5. `_require_permission("post_group_memory", {"is_group_member": …})` (`services/api/app/api/service.py:2088-2092`; `services/api/app/domain/permissions.py:423-426`): nhóm không tồn tại và nhóm thật cùng 403 `is_group_member` (`stranger_unknown_context`, `stranger_real_context`); người được mời (`invitee_posts`), người đã rời (`leaver_posts`) → 403; `X-Actor-Contexts` không được tin (`stranger_claims_context_header`); role rỗng → `role_not_permitted` (`mate_roles_empty`); chỉ `group_admin` → 201 (`mate_roles_group_admin_only`). Kiểm quyền **trước** kiểm nhóm của URL: người lạ gửi URL của nhóm khác vẫn nhận 403 (`stranger_unknown_context_mismatched_url`).
6. `_require_photo_url_context` (`service.py:599-634`): tách `image_url`; không đúng dạng → 422 `photo_url_invalid` (không tới được, pattern đã chặn); nhóm trong URL khác nhóm trong path → 422 `photo_context_mismatch` (`mate_posts_other_groups_url`). Chạy trước kiểm địa điểm (`mate_posts_other_url_unknown_place`).
7. `place_id` có giá trị → `place_row` (`service.py:1241-1247`, đọc bảng `places`): không thấy → 422 `place_not_found` (`mate_posts_unknown_place`).

## Đầu vào

- Path `context_id` (UUID lax).
- Header: `Idempotency-Key` tuỳ chọn.
- Body `MemoryCreateRequest` (`services/api/app/api/schemas.py:1400-1417`), `extra="forbid"`:
  - `image_url`: `RelativePhotoUrl` = `StrictStr` với pattern `\A/contexts/<uuid>/photos/<uuid>\z`, hex không phân biệt hoa thường (`schemas.py:29-39`). Không khớp → `string_pattern_mismatch`, `msg` in cả pattern (`url_person_photo`, `url_trailing_slash`, `url_trailing_newline`); số → `string_type` (`url_number`); thiếu → `missing` (`url_missing`). UUID chữ hoa khớp pattern và được lưu nguyên văn (`owner_posts_uppercase_photo_id`).
  - `caption`: `str | None`, **không strict, không giới hạn độ dài**; số → `string_type` (`caption_number`).
  - `place_id`: `StrictStr` 1..200, nullable (`place_empty`, `place_201_letters` → 422).
  - `author_id`, `lat` → `extra_forbidden` (`extra_field_author_id`, `extra_field_lat`).
- Không kiểm ảnh có tồn tại trong `uploaded_images`: một uuid ảnh bịa được chấp nhận, và cùng URL đăng hai lần là hai dòng (`owner_posts_same_url_again`).

## Đầu ra

- **201** `MemoryResponse` (`schemas.py:1449-1479`, `service.py:832-848`), thứ tự khoá `id`, `context_id`, `author_id`, `kind`, `image_url`, `caption`, `place_id`, `place_name`, `lat`, `lng`, `created_at`, `cursor`, `reaction_count`, `comment_count`, `viewer_has_reacted`.
  - `kind: "photo"`; `lat`, `lng` luôn `null`, kể cả khi gắn địa điểm (`mate_posts_at_place`: `place_name` từ catalogue).
  - `created_at`: `_now()` của Python truyền vào INSERT (`service.py:2110`); pydantic UTC `Z`.
  - `cursor`: `base64.urlsafe_b64encode(f"{created_at.isoformat()}|{id}")` bỏ `=` (`services/api/app/api/cursors.py:15-19`), tức base64url của chuỗi dạng `2026-09-15T00:21:17.320149+00:00|<uuid>`.
  - `reaction_count: 0`, `comment_count: 0`, `viewer_has_reacted: false`.
- Replay: 201 + body đã lưu + `idempotency-replayed: true`.
- Framework: 307 cho `/` cuối; `PUT` → 405 `allow: POST`.

## Tác dụng phụ

- INSERT `memories (context_id, author_id, kind = photo, image_url, caption, place_id, place_name, created_at)` (`services/api/app/api/repository.py:5052-5083`). CHECK `payload_matches_kind` (ảnh phải có `image_url`, địa điểm có đủ cặp id/tên, `lat`/`lng` null) và `lat_range`/`lng_range` (`services/api/app/db/models.py:1810-1830`).
- SELECT `memberships`, `places` (khi có `place_id`).
- Commit trước response.
- Idempotency: chỉ lưu 2xx; 422 của handler nhả key (`idem_refusal_not_stored` + `idem_same_key_after_refusal`). Body đã lưu trong `idempotency_keys.response_body` chứa `cursor`.
- Không limiter.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | xem card `POST /contexts` | `deps.py:102-106`, `:143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` / `invalid_actor_contexts` | xem card `POST /contexts` | `deps.py:144-163` |
| 403 | `permission_denied` | `role_not_permitted` / `is_group_member` | `service.py:2088-2092`, `:504` |
| 422 | `photo_context_mismatch` | `Photo URL context does not match the requested context` | `service.py:629-634` |
| 422 | `photo_url_invalid` | `Photo URL is not a path into this product's photo storage` (không tới được) | `service.py:623-628` |
| 422 | `place_not_found` | `No place in the catalogue has that id` | `service.py:2102-2105` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | xem card `POST /contexts` | `idempotency.py:432-439`, `:473-480` |
| 409 | `idempotency_request_in_flight` | xem card `PUT /people/me/interests` | `idempotency.py:481-493` |

422 framework (không `input`): `string_pattern_mismatch` (`ctx.pattern`), `string_type`, `string_too_short`, `string_too_long`, `missing`, `extra_forbidden`, `model_attributes_type`, `json_invalid`, `uuid_parsing`.

## Mã Python

- Route: `services/api/app/api/routes/memories.py:46-58`
- Service: `services/api/app/api/service.py:2082-2114` (`post_context_memory`), `:599-634`, `:1241-1247`, `:832-848` (`_wire_memory`)
- Codec: `services/api/app/api/cursors.py:15-19`
- Domain: `services/api/app/domain/permissions.py:423-426`, `:654-678`
- Repository: `services/api/app/api/repository.py:5052-5083`, `:2769-2782`
- Model: `services/api/app/db/models.py:1759-1869`

## Test đang phủ

- `services/api/tests/postgres/test_group_memories_postgres.py`: `test_a_member_posts_a_memory_and_reads_it_back` (146), `test_a_stranger_can_neither_read_nor_post_group_memories` (185), `test_a_person_who_left_the_group_stops_seeing_its_memories` (222), `test_the_caption_is_optional_but_the_photo_is_not` (294)
- `services/api/tests/api/test_photo_url_context_guard.py::PhotoUrlContextGuardTests` (39: 40, 46, 51, 59)
- `services/api/tests/postgres/test_group_photo_privacy_postgres.py`: `test_an_invited_person_cannot_put_a_photo_on_a_wall_they_have_not_joined` (205), `test_a_place_shows_my_groups_photographs_and_nobody_elses` (320)

## Kịch bản parity

`parity/scenarios/w3/memories/POST-contexts-context_id-memories.yaml`, id `w3/memories/post-contexts-context_id-memories` (57 bước), lane DB bật:

- Thứ tự từ chối: `anonymous_unknown_context`, `anonymous_non_uuid_context`, `anonymous_malformed_json`, `anonymous_empty_idempotency_key`, `anonymous_invalid_body`, `stranger_unknown_context`, `stranger_unknown_context_bad_url` (422 trước 403), `stranger_non_uuid_and_bad_url`, `stranger_unknown_context_mismatched_url` (403 trước 422 handler).
- 403: `stranger_real_context`, `stranger_claims_context_header`, `invitee_posts`, `leaver_posts`, `mate_roles_empty`.
- Đường vui: `mate_posts_photo`, `mate_posts_with_caption`, `mate_posts_null_caption_and_place`, `mate_posts_at_place`, `owner_posts_uppercase_photo_id`, `owner_posts_same_url_again`, `mate_roles_group_admin_only`, `owner_reads_wall`.
- 422 handler: `mate_posts_unknown_place`, `mate_posts_other_groups_url`, `mate_posts_other_url_unknown_place`.
- Validate: `url_person_photo`, `url_trailing_slash`, `url_trailing_newline`, `url_number`, `url_missing`, `caption_number`, `place_empty`, `place_201_letters`, `extra_field_author_id`, `extra_field_lat`, `top_level_array`.
- Idempotency: `idem_first`, `idem_replay`, `idem_reuse_different_body`, `idem_refusal_not_stored` + `idem_same_key_after_refusal`, `idem_key_too_long`.
- Framework: `trailing_slash_redirects`, `put_not_allowed`.

`prod` (`w3/memories/prod-auth`): `anonymous_post_memory` (401), `mate_posts_photo_empty_roles_header` (201).

Chạy với làn DB bật. `cursor` trong câu trả lời và trong `idempotency_keys.response_body` được harness bind thành `<b64u:<ts#r|dạng>|<uuid#n>>` (ADR-0029 §2.4, xem mục dưới). Corpus 422 sinh tự động hoãn: `carries ['pattern']`.

## Chưa phủ / lưu ý cho bản Go

- **`cursor` được bind theo cấu trúc** (ADR-0029 §2.4): chuỗi base64url không padding giải mã được thành `<thời điểm>|<uuid4>` trở thành `<b64u:<ts#r|dạng>|<uuid#n>>`, hai phần bên trong bind như văn bản thường, nên cùng một cursor ở hai chỗ phải trỏ cùng dòng và cùng thời điểm. Vẫn lộ ra nếu bản Go giữ padding, dùng bảng chữ `+/`, viết thời điểm khác `isoformat()` của Python (`+00:00` chứ không `Z`; không phần lẻ khi micro giây bằng 0, 6 chữ số khi khác 0), hay ghép sai `|`. Canary có mode `cursor-padding-kept` cho padding.
- `caption` không giới hạn độ dài và không strict ở route này (check-in giới hạn 2000).
- Không kiểm ảnh có thật: parity giữ nguyên, xem báo cáo.
- 409 in-flight và hai lần đăng đồng thời chưa phủ.

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này nằm trong vùng chạm của `check_go_owned_python_touch.py` do đồ thị gọi so theo tên hàm trần (`ApiService.pair_notebook` trùng tên module `pair_notebook`); mã của route không đọc đồng ý của sổ đôi. Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `POST /contexts/{context_id}/memories`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `POST /contexts/{context_id}/memories`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — `chia_gu`: gu trong sổ đôi, mỗi người tự bật (ADR-0034 §2.1–2.2)

Diff này thêm mục đích đồng ý `chia_gu` (CONSENT_PURPOSES, PER_PERSON_PURPOSES; CHECK `ck_pair_consent_proposals_consent_purpose_known` mở rộng ở migration `e3b7c1d9a4f2`), hàm thuần `pair_notebook.gu_hai_nguoi` (Go `pairnotebook.GuHaiNguoi`) và trường `taste` của `PairNotebookResponse` (`_pair_taste`, Go `pairsteps.pairTaste`). Golden: `python_pair_notebook*.json` (ca `taste: *`, fuzz có `chia_gu`), `python_pair_steps*.json` (ca `taste_*`); Go replay 0 lệch; tầng Postgres Python 720 xanh.

- `POST /contexts/{context_id}/memories`: Route này không đọc sổ đôi hay gu; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
