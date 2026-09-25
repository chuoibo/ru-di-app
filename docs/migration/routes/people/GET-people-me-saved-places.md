# GET /people/me/saved-places

people · core · trạng thái trong bộ nhớ: không có

## Mục đích

Các địa điểm người gọi đã đánh dấu, mới nhất trước, mỗi hàng được đặt tên từ danh mục lúc đọc (M2).

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:96-105`, `services/api/app/api/service.py:4245-4254`), đo trên stack:

1. Router: đuôi `/` → 307; `POST` → 405.
2. `get_actor` → 401, 422 `invalid_actor_id` / `invalid_actor_roles`.
3. `_require_permission("manage_saved_places", {"is_self": True})` (`services/api/app/domain/permissions.py:202`): thiếu `member` → 403 `permission_denied` `role_not_permitted` (`owner_lists_without_roles`).
4. Không tra hàng `people`: chưa đăng ký → 200 `{"saved": []}`.

## Đầu vào

Không path, không thân; query bị bỏ qua (`owner_lists_with_query`).

## Đầu ra

**200** `{"saved": [{"place_id", "name", "category", "saved_at"}, …]}` (`services/api/app/api/schemas.py:951-959`).

- Hàng: `saved_places WHERE person_id = me ORDER BY created_at DESC, id` (`services/api/app/api/repository.py:4265-4271`). Lưu lại một chỗ đã lưu không đổi `created_at` nên không đổi thứ tự (`owner_lists_after_resave`).
- Mỗi hàng tra `place_row(place_id)` (`service.py:1241-1247`, rồi `get_place`, `repository.py:4190-4192`): không còn trong danh mục → hàng bị bỏ khỏi đầu ra, không bị xoá (`service.py:4251-4253`).
- `saved_at` = `saved_places.created_at`.

## Tác dụng phụ

Không. Một câu danh sách, rồi một `places` mỗi hàng.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:144-155` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:4246` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:96-105`
- Service: `services/api/app/api/service.py:4245-4254`, `:4282-4288`, `:1241-1247`
- Repository: `services/api/app/api/repository.py:4265-4271`, `:4190-4192`
- Quyền: `services/api/app/domain/permissions.py:202`

## Test đang phủ

- `services/api/tests/api/test_profile.py`: `test_saved_places_are_idempotent_and_named_from_the_catalogue` (191)
- `services/api/tests/postgres/test_profile_postgres.py`: `test_one_bookmark_per_person_and_place_in_the_database_and_over_http` (262)

## Kịch bản parity

`parity/scenarios/w10/people/GET-people-me-saved-places.yaml`, id `w10/people/get-people-me-saved-places` (27 bước, `dev`):

- Thứ tự: `anonymous_lists`, `actor_id_not_uuid`, `roles_unknown`, `owner_lists_without_roles` (`guest`), `owner_lists_unregistered` (200 rỗng).
- Thứ tự hàng: `owner_lists_three` (mới nhất trước), `owner_saves_first_again` + `owner_lists_after_resave` (không đổi), `owner_unsaves_second`, `owner_lists_as_group_admin_and_member`, `other_lists_own`, `owner_lists_with_query`.
- Xoá tài khoản: `leaver_saves`, `leaver_lists_before`, `leaver_ends_account`, `leaver_lists_after` (rỗng), `other_lists_after_leaver`.
- Framework: `trailing_slash` (307), `post_not_allowed` (405 `allow: GET`).

Đọc lại cũng có trong `crossreplay/PUT-…`, `crossreplay/DELETE-…`, `concurrency/PUT-…`, `concurrency/DELETE-…` và `prod-auth.yaml` (`owner_lists_saved`, `owner_lists_saved_after_stranger`).

Corpus sinh: `generated/w10-422/get-people-me-saved-places.yaml` (5 bước).

## Chưa phủ / lưu ý cho bản Go

- Hàng có `place_id` không còn trong danh mục bị bỏ khỏi đầu ra; chưa phủ vì không có cửa HTTP xoá danh mục.
- Tên và loại đọc lúc trả lời, không lưu cùng dấu.

## Lỗi Python (chỉ báo, không sửa)

- Một truy vấn `places` cho mỗi hàng (N+1). Không ảnh hưởng đầu ra.

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này nằm trong vùng chạm của `check_go_owned_python_touch.py` do đồ thị gọi so theo tên hàm trần (`ApiService.pair_notebook` trùng tên module `pair_notebook`); mã của route không đọc đồng ý của sổ đôi. Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `GET /people/me/saved-places`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `GET /people/me/saved-places`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — `chia_gu`: gu trong sổ đôi, mỗi người tự bật (ADR-0034 §2.1–2.2)

Diff này thêm mục đích đồng ý `chia_gu` (CONSENT_PURPOSES, PER_PERSON_PURPOSES; CHECK `ck_pair_consent_proposals_consent_purpose_known` mở rộng ở migration `e3b7c1d9a4f2`), hàm thuần `pair_notebook.gu_hai_nguoi` (Go `pairnotebook.GuHaiNguoi`) và trường `taste` của `PairNotebookResponse` (`_pair_taste`, Go `pairsteps.pairTaste`). Golden: `python_pair_notebook*.json` (ca `taste: *`, fuzz có `chia_gu`), `python_pair_steps*.json` (ca `taste_*`); Go replay 0 lệch; tầng Postgres Python 720 xanh.

- `GET /people/me/saved-places`: Route này không đọc sổ đôi hay gu; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.
