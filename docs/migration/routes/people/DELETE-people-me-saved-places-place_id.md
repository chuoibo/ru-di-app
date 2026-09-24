# DELETE /people/me/saved-places/{place_id}

people · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Bỏ đánh dấu một địa điểm. Idempotent: bỏ một dấu không có vẫn là «không có», nên 204 cả hai trường hợp; chỉ một khoá danh mục lạ bị từ chối, vì nó chỉ có thể là lỗi client.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:130-141`, `services/api/app/api/service.py:4262-4268`), đo trên stack:

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 204 lưu và phát lại; chỗ khác cùng khoá → 422 reuse; 404 nhả khoá (`owner_unknown_with_key_two`, `owner_known_with_key_two`).
2. Router: path giải mã phần trăm; đuôi `/` → 307; `GET` → 405.
3. `get_actor` → 401, 422.
4. `_require_permission("manage_saved_places", {"is_self": True})` (`service.py:4266`): thiếu `member` → 403 `role_not_permitted`, trước khi tra chỗ (`owner_unknown_place_without_roles`).
5. `_known_place` → 404 `place_not_found` `Không có địa điểm này trong danh mục.` (`service.py:4270-4279`), đã lưu hay chưa (`owner_unsaves_unknown_place`, `owner_unsaves_uppercase_place`, `ghost_unsaves_unknown_unregistered`).
6. `unsave_place` (`services/api/app/api/repository.py:4299-4309`): SELECT rồi `session.delete`; không có hàng → không làm gì. Kết quả bool bị bỏ qua.

## Đầu vào

- Path `place_id`: khoá danh mục. Không query; thân bị bỏ qua kể cả JSON hỏng (`owner_unsaves_with_json_body`).

## Đầu ra

**204**, thân rỗng, không `content-type` (`people.py:141`). Như nhau cho: đã lưu, chưa bao giờ lưu, vừa bỏ, người gọi chưa đăng ký (`ghost_unsaves_known_unregistered`).

## Tác dụng phụ

DELETE đúng một hàng `saved_places (person_id, place_id)` nếu có. Dấu của người khác cùng chỗ không bị chạm (`other_lists_after`).

## Idempotency

Tự nhiên idempotent. Khoá header: 204 lưu và phát lại với `content-length: 0` và `idempotency-replayed: true`, **không** xoá lần nữa: lưu lại rồi phát lại khoá thì dấu vẫn còn (`owner_saves_second_again`, `owner_replays_key`, `owner_lists_after_replay`). Ba lần bỏ đồng thời: ba 204.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:144-155` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:4266` |
| 404 | `place_not_found` | `Không có địa điểm này trong danh mục.` | `service.py:4270-4279` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-481` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:130-141`
- Service: `services/api/app/api/service.py:4262-4279`
- Repository: `services/api/app/api/repository.py:4299-4309`
- Quyền: `services/api/app/domain/permissions.py:202`

## Test đang phủ

- `services/api/tests/api/test_profile.py`: `test_saved_places_are_idempotent_and_named_from_the_catalogue` (191), `test_a_key_the_catalogue_does_not_know_is_refused_on_put_and_delete` (224)

## Kịch bản parity

`parity/scenarios/w10/people/DELETE-people-me-saved-places-place_id.yaml`, id `w10/people/delete-people-me-saved-places-place_id` (29 bước, `dev`):

- Thứ tự: `anonymous_unsaves`, `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `roles_unknown`, `owner_unknown_place_without_roles` (403), `ghost_unsaves_known_unregistered` (204), `ghost_unsaves_unknown_unregistered` (404).
- `owner_unsaves_never_saved` (204), `owner_unsaves_unknown_place`, `owner_unsaves_uppercase_place` (404).
- `owner_saves_first`, `owner_saves_second`, `other_saves_first`, `owner_unsaves_first` (một hàng xoá), `owner_unsaves_first_again` (204, không delta), `owner_lists_after`, `other_lists_after`.
- Khoá: `owner_unsaves_second_with_key`, `owner_saves_second_again`, `owner_replays_key` (204 phát lại, không xoá), `owner_lists_after_replay` (dấu còn), `owner_key_other_place`, `owner_unknown_with_key_two` (404 nhả khoá), `owner_known_with_key_two` (204).
- `owner_unsaves_with_json_body`, `trailing_slash`, `get_not_allowed` (405 `allow: PUT`).

`crossreplay/DELETE-people-me-saved-places-place_id.yaml` (12 bước), `concurrency/DELETE-people-me-saved-places-place_id.yaml` (6 bước: ba lần bỏ cùng lúc → ba 204, một hàng xoá).

`prod-auth.yaml`: `anonymous_unsaves`, `stranger_unsaves_claiming_owner` (204 trên dấu của chính stranger, dấu của owner còn).

## Chưa phủ / lưu ý cho bản Go

- `session.delete` của SQLAlchemy: nếu hàng đã bị xoá bởi request đồng thời, câu DELETE khớp 0 hàng chỉ sinh cảnh báo, request vẫn 204.
- Corpus sinh: hoãn, `path place_id is str`.

## Lỗi Python (chỉ báo, không sửa)

- Không có lỗi hành vi đo được. Ghi nhận: không phân biệt được «đã bỏ» với «chưa bao giờ lưu» (có chủ ý).

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này nằm trong vùng chạm của `check_go_owned_python_touch.py` do đồ thị gọi so theo tên hàm trần (`ApiService.pair_notebook` trùng tên module `pair_notebook`); mã của route không đọc đồng ý của sổ đôi. Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `DELETE /people/me/saved-places/{place_id}`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
