# PUT /people/me/saved-places/{place_id}

people · core · trạng thái trong bộ nhớ: không có (không limiter)

## Mục đích

Đánh dấu một địa điểm có trong danh mục. 201 lần đầu, 200 khi đã đánh dấu, cùng thân.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:108-127`, `services/api/app/api/service.py:4256-4260`), đo trên stack:

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): khoá rỗng 422 (`anonymous_empty_idempotency_key`); khoá đã xong phát lại 201 (`owner_replays_key`); chỗ khác cùng khoá → 422 reuse (`owner_key_other_place`).
2. Router: path đã giải mã phần trăm trước khi khớp: `p%2Dtiem…` là `p-tiem…` (`owner_saves_percent_encoded_hyphens`); `%2F` tạo hai đoạn và không khớp route nào → 404 `{"detail":"Not Found"}` (`owner_place_encoded_slash`); đuôi `/` → 307 (`trailing_slash`); đoạn rỗng `/people/me/saved-places/` → 307 về route GET (`empty_place_segment`); `POST` → 405.
3. `get_actor` → 401 (trước 404 của chỗ lạ, `anonymous_unknown_place`); 422 `invalid_actor_id` / `invalid_actor_roles`.
4. `place_id: str` bất kỳ, không validation. Thân không khai báo, bị bỏ qua (`owner_saves_with_json_body`).
5. `_require_permission("manage_saved_places", {"is_self": True})` (`service.py:4257`; `services/api/app/domain/permissions.py:202`): thiếu `member` → 403 `permission_denied` `role_not_permitted`, trước khi tra chỗ (`owner_unknown_place_without_roles`).
6. `_known_place` (`service.py:4270-4279`): `place_row` (`service.py:1241-1247` → `get_place`, `services/api/app/api/repository.py:4190-4192`) không có → 404 `place_not_found` `Không có địa điểm này trong danh mục.` So khớp chính xác: viết hoa, có khoảng trắng, tiếng Việt đều 404 (`owner_unknown_place`, `owner_place_uppercase`, `owner_place_padded`, `owner_place_vietnamese`).
7. `save_place` (`repository.py:4273-4297`): SELECT hàng `(person_id, place_id)`; có → 200 với hàng cũ; không → INSERT `saved_places(person_id, place_id, created_at=now)` trong savepoint → 201. `IntegrityError` → đọc lại hàng thắng → 200. Người gọi chưa có hàng `people` (dev): INSERT vi phạm khoá ngoại, cũng là `IntegrityError`, đọc lại không có gì → `assert` → 500 (`ghost_saves_unregistered`).

## Đầu vào

- Path `place_id`: khoá danh mục (`places.id`), chuỗi, phân biệt hoa thường, sau giải mã phần trăm.
- Không query, không thân.

## Đầu ra

**201** hoặc **200** `SavedPlaceSummary` (`services/api/app/api/schemas.py:951-955`), thứ tự khoá: `place_id`, `name`, `category`, `saved_at`. `name`, `category` từ danh mục; `saved_at` = `created_at` của hàng (lần đầu), không đổi khi lưu lại.

## Tác dụng phụ

INSERT `saved_places` một lần cho mỗi `(person, place)` (`uq_saved_places_person_place`, `services/api/app/db/models.py:2704`). Lưu lại không ghi. Bỏ lưu rồi lưu lại là hàng mới với `saved_at` mới (`owner_saves_first_after_unsave`).

## Idempotency

Tự nhiên idempotent (200 lần sau). Khoá header: 201 được lưu và phát lại là 201 dù hàng đã có (`crossreplay/PUT-people-me-saved-places-place_id.yaml`). Ba lần lưu đồng thời: một 201, hai 200 cùng `saved_at` (`concurrency/…`).

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:144-155` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:4257` |
| 404 | `place_not_found` | `Không có địa điểm này trong danh mục.` | `service.py:4270-4279` |
| 404 | — | `{"detail":"Not Found"}` (`%2F`) | Starlette |
| 500 | — | `Internal Server Error` (người gọi chưa đăng ký) | `repository.py:4288-4295` |
| 422 | `invalid_idempotency_key` / `idempotency_key_reuse` | | `idempotency.py:432-481` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:108-127`
- Service: `services/api/app/api/service.py:4256-4260`, `:4270-4288`, `:1241-1247`
- Repository: `services/api/app/api/repository.py:4273-4297`, `:4190-4192`
- Quyền: `services/api/app/domain/permissions.py:202`

## Test đang phủ

- `services/api/tests/api/test_profile.py`: `test_saved_places_are_idempotent_and_named_from_the_catalogue` (191), `test_a_key_the_catalogue_does_not_know_is_refused_on_put_and_delete` (224)
- `services/api/tests/postgres/test_profile_postgres.py`: `test_one_bookmark_per_person_and_place_in_the_database_and_over_http` (262)

## Kịch bản parity

`parity/scenarios/w10/people/PUT-people-me-saved-places-place_id.yaml`, id `w10/people/put-people-me-saved-places-place_id` (31 bước, `dev`):

- Thứ tự: `anonymous_saves`, `anonymous_unknown_place` (401 trước 404), `anonymous_empty_idempotency_key`, `actor_id_not_uuid`, `roles_unknown`, `owner_unknown_place_without_roles` (403 trước 404), `ghost_saves_unregistered` (500), `ghost_unknown_place_unregistered` (404).
- Khoá danh mục: `owner_unknown_place`, `owner_place_uppercase`, `owner_place_padded`, `owner_place_encoded_slash` (404 Starlette), `owner_place_vietnamese`.
- Lưu: `owner_saves_first` (201), `owner_saves_first_again`, `owner_saves_percent_encoded_hyphens`, `owner_saves_with_json_body` (200), `owner_saves_second_as_group_admin_and_member` (201).
- Khoá: `owner_saves_third_with_key` (201), `owner_replays_key`, `owner_key_other_place`, `owner_saves_third_without_key` (200).
- `other_saves_same_place` (201), `owner_lists`, `owner_unsaves_then_saves_again`, `owner_saves_first_after_unsave` (201, `saved_at` mới).
- Framework: `trailing_slash`, `empty_place_segment` (307 về `/people/me/saved-places`), `post_not_allowed` (405 `allow: PUT`).

`crossreplay/PUT-people-me-saved-places-place_id.yaml` (11 bước), `concurrency/PUT-people-me-saved-places-place_id.yaml` (4 bước: ba lần lưu cùng lúc → hai 200 và một 201 cùng `saved_at`; ba lần cùng khoá → ba 201, một chèn).

`prod-auth.yaml`: `junk_bearer_unknown_place` (401 trước 404), `owner_saves_place`, `owner_saves_after` (401).

## Chưa phủ / lưu ý cho bản Go

- Tra danh mục theo khoá chính xác, sau giải mã phần trăm của router; `%2F` không bao giờ tới handler.
- Vai trò trước danh mục; danh mục trước hàng `people` (không tra).
- Người gọi chưa có hàng `people` là 500 (`AssertionError`), transaction rollback.
- Corpus sinh: hoãn, `path place_id is str`; hình dạng khoá đã phủ viết tay ở trên.

## Lỗi Python (chỉ báo, không sửa)

- `IntegrityError` của khoá ngoại `person_id` bị đọc như một lần đua, đọc lại không thấy gì rồi `assert` → 500 thay vì 404/409 (`repository.py:4288-4295`). Chỉ tới được khi người gọi không có hàng `people` (dev).

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này nằm trong vùng chạm của `check_go_owned_python_touch.py` do đồ thị gọi so theo tên hàm trần (`ApiService.pair_notebook` trùng tên module `pair_notebook`); mã của route không đọc đồng ý của sổ đôi. Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).

## Đổi 2026-09-23 (b) — `place_id` của tờ giấy là id danh mục; chốt ghi chặng vào kèo (QA cặp đôi)

Diff này đổi `PaperStopInput.place_id`/`PaperStop.place_id` từ `uuid.UUID` sang `StrictStr` 1..80 (đúng kiểu `OutingStopInput.place_id`: danh mục dùng slug như `p-lau-ga`, trước đây mọi chỗ có thật đều bị 422), `_noi_dung_wire` đọc `str(place_id)` thay vì `uuid.UUID(...)`, và `ApiService._chot` (Go `pairsteps.chot`) đọc `get_place` cho từng chặng có id, đặt tên kèo «<tên quán hoặc việc chặng đầu, ≤190 ký tự> · dd/mm» thay vì «Tờ lời rủ dd/mm», rồi sau `link_paper_outing` gọi `replace_outing_stops(expected_revision=None)` cùng transaction: chặng đã đồng ý thành timeline của kèo; id danh mục không còn thì giữ nhãn, bỏ id. Go: `routes/pair_papers.go` (`optionalStringField`), `pairsteps/wire.go`, `pairsteps/papers.go`, `service/pair_store.go` (`GetPlace`, `ReplaceOutingStops`); golden `python_pair_steps.json` tái sinh (+4 ca `agreed_place_*`), repo oracle thêm ca «a catalogue place names the outing and its stop» và dump `outing_stops`.

- `PUT /people/me/saved-places/{place_id}`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-24 — tờ chưa từng gửi chỉ chủ bản phác thấy, ở mọi trạng thái (lỗ rò riêng tư, QA cặp đôi)

Diff này thêm `_chi_chu_thay` (Go `pairsteps.chiChuThay`): người không phải chủ bản phác chỉ thấy một tờ khi tờ không ở `nhap` **và** có ít nhất một phiên bản đã gửi (`sent_at` khác null). Trước đây luật là «không phải `nhap`», nên bản phác chưa gửi mà chủ bấm «Tuần này nghỉ» (`nghi_tuan`), bỏ (`bo`) hay để hết tuần (`het_han`) hiện ra trong danh sách và chi tiết của người kia, kèm nội dung và lý do riêng — tái hiện trên stack cô lập 24/09 bằng hai phiên thật. Áp ở `list_pair_papers` (lọc) và `_readable_paper_or_404` (`may_view_paper`, 404 `paper_not_found`), nên mọi lệnh đọc/ghi tờ đi qua cửa này. Golden `python_pair_steps.json` thêm `unsent_*`/`sent_then_skipped_as_kia`; bản sao route trong repo oracle Postgres đổi theo.

- `PUT /people/me/saved-places/{place_id}`: Route này không đọc tờ giấy; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — `chia_gu`: gu trong sổ đôi, mỗi người tự bật (ADR-0034 §2.1–2.2)

Diff này thêm mục đích đồng ý `chia_gu` (CONSENT_PURPOSES, PER_PERSON_PURPOSES; CHECK `ck_pair_consent_proposals_consent_purpose_known` mở rộng ở migration `e3b7c1d9a4f2`), hàm thuần `pair_notebook.gu_hai_nguoi` (Go `pairnotebook.GuHaiNguoi`) và trường `taste` của `PairNotebookResponse` (`_pair_taste`, Go `pairsteps.pairTaste`). Golden: `python_pair_notebook*.json` (ca `taste: *`, fuzz có `chia_gu`), `python_pair_steps*.json` (ca `taste_*`); Go replay 0 lệch; tầng Postgres Python 720 xanh.

- `PUT /people/me/saved-places/{place_id}`: Route này không đọc sổ đôi hay gu; cổng `check_go_owned_python_touch.py` nối theo tên hàm nên bị kéo vào — hành vi không đổi.

## Đổi 2026-09-25 — Nếp dùng gu của người đã bật `chia_gu` khi phác tờ (ADR-0034 §2.2)

Diff này tách `_pair_context_or_404` thành `_pair_roster_or_404` (cùng ba lệnh đọc, cùng thứ tự, cùng câu trả lời; chỉ giữ thêm tên hiển thị của hàng thành viên — Go `pairRosterOr404`, `Member.DisplayName`) và thêm vào `draft_pair_paper` bước gu: `_gu_cho_nep` + hàm thuần `pair_paper.gu_cho_nep` / `loai_theo_gu` / `lam_giau_theo_gu` (Go `pairpaper.GuChoNep` / `LoaiTheoGu` / `LamGiauTheoGu`). Golden: `python_pair_paper*.json` (ca `gu_cho_nep`, `lam_giau_theo_gu`, fuzz riêng), `python_pair_steps.json` (7 ca `taste_*` của draft); Go replay 0 lệch.

- `PUT /people/me/saved-places/{place_id}`: Chỉ đi qua `_pair_roster_or_404` (cùng lệnh đọc, cùng câu trả lời) hoặc bị cổng nối theo tên hàm kéo vào — hành vi không đổi.
