# GET /people/{person_id}

people · core · trạng thái trong bộ nhớ: không có

## Mục đích

Hồ sơ công khai của một người, cho chính họ, một người bạn hoặc một người cùng nhóm đang hoạt động. Quan hệ được chứng minh từ đồ thị bạn bè và roster **trước** khi đọc hàng `people`, nên mọi id không nhìn được, có tồn tại hay không, là cùng một 403.

## Xác thực và quyền

Thứ tự (`services/api/app/api/routes/people.py:250-266`, `services/api/app/api/service.py:4191-4243`), đo trên stack:

1. Router: `/people/me` là route riêng (khai trước); đuôi `/` → 307; `DELETE`, `HEAD` → 405.
2. `get_actor` → 401 (trước 422 path), 422 `invalid_actor_id` / `invalid_actor_roles`.
3. Path `person_id`: UUID lax → 422 `uuid_parsing` (`owner_path_not_uuid`); dạng không gạch nối của id bịa → 403 như id bịa (`owner_path_hyphenless_unknown`).
4. Quan hệ, theo thứ tự (`service.py:4200-4207`): `person_id == actor.id` → `self`; `are_friends` (`services/api/app/api/repository.py:4077-4097`) → `friend`; `share_active_context` (`repository.py:4099-4113`: cả hai có membership `active` trong cùng context, **pair cũng tính**) → `groupmate`; còn lại không có.
5. `_require_permission("view_person_profile", {"is_visible_person": …})` (`services/api/app/domain/permissions.py:197-201`): **mọi** 403 (thiếu `member`, không quan hệ) đổi thành 403 `person_not_visible` `Không xem được hồ sơ này.` (`service.py:4208-4219`). `X-Actor-Contexts` không được đọc (`owner_reads_stranger_claiming_context`).
6. `get_person`; không có hoặc `deleted_at` có → 404 `person_not_found` `Chưa có hồ sơ cho tài khoản này.` (`service.py:4221-4233`). Chỉ `self` tới được đây: người gọi chưa đăng ký (`owner_reads_self_unregistered`) hoặc đã xoá tài khoản trong dev (`leaver_reads_self_after`).

Ma trận đo được (người đọc → người được đọc):

| Quan hệ | Kết quả |
|---|---|
| chính mình, có `member` | 200 `self` |
| chính mình, không `member` (`''`, `group_admin`) | 403 `person_not_visible` |
| bạn `accepted` (hai chiều) | 200 `friend`, kể cả khi cùng nhóm |
| bạn nhưng người đọc thiếu `member` | 403 |
| lời mời `pending` (hai chiều) | 403 |
| cùng nhóm, cả hai `active` | 200 `groupmate` |
| một bên chỉ `invited` | 403 |
| một bên đã rời nhóm | 403 |
| từng là bạn, đã chặn, còn pair chung | 200 `groupmate` (hai chiều) |
| người lạ, id bịa, id của người đã xoá tài khoản | 403, cùng byte |

## Đầu vào

- Path `person_id`: UUID. Không query, không thân.

## Đầu ra

**200** `PublicPersonResponse` (`services/api/app/api/schemas.py:938-948`), thứ tự khoá: `id`, `display_name`, `bio`, `city`, `created_at`, `relation` (`self` | `friend` | `groupmate`). Không có số đếm, sở thích, `wall_comment_policy`, `discoverable_by_phone`, cách đăng nhập. `bio`/`city` là giá trị đã lưu (null khi trống).

## Tác dụng phụ

Không. Chỉ đọc `friend_requests`, `memberships`, `people`.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` / `Missing bearer session`, `Session is not valid` | `deps.py:93-143` |
| 422 | `invalid_actor_id` / `invalid_actor_roles` | | `deps.py:144-155` |
| 422 | (validation) | `uuid_parsing` `["path","person_id"]` | `main.py:319-351` |
| 403 | `person_not_visible` | `Không xem được hồ sơ này.` | `service.py:4208-4219` |
| 404 | `person_not_found` | `Chưa có hồ sơ cho tài khoản này.` | `service.py:4221-4233` |
| 307 / 405 | — | | Starlette |

## Mã Python

- Route: `services/api/app/api/routes/people.py:250-266`
- Service: `services/api/app/api/service.py:4191-4243`
- Repository: `services/api/app/api/repository.py:4077-4113`, `:2525-2527`
- Quyền: `services/api/app/domain/permissions.py:197-201`

## Test đang phủ

- `services/api/tests/api/test_profile.py`: `test_a_friend_and_a_groupmate_see_the_public_view_with_their_relation` (159), `test_a_stranger_and_a_nonexistent_id_get_the_same_403` (178), `test_profile_routes_need_a_signed_in_person` (235)
- `services/api/tests/api/test_delete_account.py`: `test_the_ended_account_stops_being_a_person_the_product_answers_about` (95)
- `services/api/tests/postgres/test_profile_postgres.py`: `test_a_patch_persists_and_the_public_view_follows_the_relation` (231)

## Kịch bản parity

`parity/scenarios/w10/people/GET-people-person_id.yaml`, id `w10/people/get-people-person_id` (62 bước, `dev`):

- Thứ tự: `anonymous_reads`, `anonymous_non_uuid` (401 trước 422), `actor_id_not_uuid`, `roles_unknown`, `owner_path_not_uuid`, `owner_path_hyphenless_unknown` (403), `owner_reads_self_unregistered` (404).
- Chính mình: `owner_reads_self`, `owner_writes_bio`, `owner_reads_self_with_bio` (`<b>` giữ nguyên), `owner_reads_self_without_roles`, `owner_reads_self_as_group_admin` (403).
- Không quan hệ: `owner_reads_unknown_id`, `owner_reads_stranger`, `owner_reads_stranger_claiming_context`, `owner_reads_pending`, `pending_reads_owner`.
- Bạn: `owner_reads_friend`, `friend_reads_owner`, `friend_reads_owner_without_roles` (403).
- Nhóm: `owner_reads_mate`, `mate_reads_owner`, `mate_reads_friend` (`groupmate`), `owner_reads_friend_also_groupmate` (`friend`), `owner_reads_invitee`, `invitee_reads_owner` (403), `mate_leaves_group`, `owner_reads_mate_after_leave`, `mate_reads_owner_after_leave` (403).
- Pair và chặn: `owner_opens_pair_with_blocker`, `blocker_blocks_owner`, `owner_reads_blocker_after_block`, `blocker_reads_owner_after_block` (200 `groupmate`).
- Xoá tài khoản: `owner_reads_leaver_before` (`friend`), `leaver_ends_account`, `owner_reads_leaver_after` (403, cùng byte với `owner_reads_unknown_id`), `leaver_reads_self_after` (404), `leaver_reads_owner_after` (403).
- Framework: `trailing_slash`, `delete_on_id_not_allowed`, `head_not_allowed` (405 `allow: GET`).

Cũng đọc trong `POST-people-person_id-block.yaml` (`owner_reads_target_profile_after`), `DELETE-people-person_id-block.yaml` (`owner_reads_friend_profile_after_lift`), `DELETE-people-me.yaml` (`owner_reads_own_public_profile_after`), `DELETE-people-me-world.yaml` (`mate_reads_leaver_profile_after`), `prod-auth.yaml` (`anonymous_non_uuid_person` 401, `mate_reads_owner`, `stranger_reads_owner`, `mate_reads_owner_after`).

Corpus sinh: `generated/w10-422/get-people-person_id.yaml` (18 bước).

## Chưa phủ / lưu ý cho bản Go

- Ba truy vấn quan hệ theo thứ tự `self` → `friend` → `groupmate` chạy trước khi đọc hàng; câu SQL chạy cả với id bịa.
- `share_active_context` gồm pair: người đã bị chặn vẫn đọc được hồ sơ qua pair chung.
- 404 chỉ cho `self` trên hàng thiếu hoặc đã xoá.
- Chưa phủ: id viết hoa hay không gạch nối của một người có thật (harness không đổi dạng giá trị đã bind; corpus sinh phủ dạng đó cho id bịa).

## Lỗi Python (chỉ báo, không sửa)

- Chặn không ẩn hồ sơ khi còn nhóm hoặc pair chung (ADR-0023 §2.3.2 chỉ nói bài, story, nhắn riêng).
- Người đã xoá tài khoản (dev) đọc chính mình là 404, còn `GET /people/me` là 200 hàng ẩn danh.

## Đổi 2026-09-25 (lượt 2) — gậy luân phiên, hạn mức tuần, «Một đôi» trên hồ sơ, 4 sticker đôi (ADR-0034 §2.4–2.5)

Diff này: (1) `vai_tuan` nhận `mo_loi_truoc` — người lo quen đã mở lời (tờ đầu tiên gửi trong tuần, `nguoi_mo_loi`) hai tuần liền thì tuần này sang người kia, `cach` = `luot`; tín hiệu tờ thêm `tuan`, `sent_at`. (2) `draft_pair_paper` từ chối 409 `paper_week_quota` khi người gọi đã phác `TO_MOI_NGUOI_MOI_TUAN` (3, cùng số với `packages/shared/nep-nhip.json`) tờ trong tuần. (3) `get_person_profile` trả `relation` = `couple` khi hai người là một «Một đôi» (`same_couple`, hỏi SAU cửa quyền, không phải oracle cho người lạ). (4) Từ vựng sticker thêm `hen-nhe`, `nho-nhau`, `ve-toi-chua`, `om-cai`. Golden pair_notebook (ca baton, 10 shard), pair_steps (`quota_*`, `role_baton_*`), people_steps (`couple_*`), stickers; Go 0 lệch.

- `GET /people/{person_id}`: Chỉ bị cổng nối theo tên hàm kéo vào — hành vi không đổi.
