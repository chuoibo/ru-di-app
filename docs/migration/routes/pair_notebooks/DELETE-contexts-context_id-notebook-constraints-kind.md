# DELETE /contexts/{context_id}/notebook/constraints/{kind}

pair_notebooks · core · trạng thái trong bộ nhớ: không có

## Mục đích

Xoá dòng ràng buộc của chính mình (`khong_an_duoc` hoặc `dung`) trong chu kỳ sống. «Làm rỗng» một ràng buộc là việc của route này, không phải PUT với chuỗi rỗng.

## Xác thực và quyền

Thứ tự (đọc từ mã, kịch bản đo):

1. Middleware idempotency (`services/api/app/api/idempotency.py:404-583`): 204 được lưu; phát lại đi trước service (`owner_replays_header_key` phát lại 204 dù dòng viết lại ở giữa vẫn còn).
2. `get_actor` → 401 (`anonymous_deletes`), 422.
3. Validation `context_id` → 422.
4. Kind ngoài `CONSTRAINT_KINDS` → 404 `constraint_kind_unknown` `Không có ô này.` **trước context** (`services/api/app/api/service.py:7529-7530`; `stranger_unknown_kind`, `owner_kind_uppercase`).
5. `_pair_context_or_404` → 404 `notebook_not_found` (`stranger_deletes_in_pair`, `owner_deletes_in_group`).
6. `_require_pair_permission("edit_pair_constraint", {"is_self": True})` (`service.py:7532`) → 403 `role_not_permitted` (`owner_deletes_as_advancer`).
7. `_locked_notebook`; không có chu kỳ sống → 204 không làm gì (`owner_deletes_on_fresh_pair`, `owner_deletes_after_close`).

## Đầu vào

- Path `context_id`, `kind`. Không thân.

## Đầu ra

**204**, thân rỗng. Không phân biệt «đã xoá» với «không có gì để xoá» (`owner_deletes_missing`).

## Tác dụng phụ

- Pair chưa có hàng sổ: `pair_notebooks` được chèn và commit (`owner_deletes_on_fresh_pair`).
- Có chu kỳ: `DELETE pair_shared_constraints WHERE (cycle_id, owner_id=actor, kind)` (`repository.py:7959-7967`). Chỉ dòng của người gọi; dòng cùng kind của người kia còn nguyên (`mate_deletes_khong_an_duoc`, `owner_reads_after_mate_delete`).
- Không có lịch sử: PUT sau khi xoá bắt đầu lại `version=1` (`owner_puts_after_delete`, `mate_reads_version_restart`).
- Từ chối rollback.

## Lỗi

| Status | code | detail (nguyên văn) | Nguồn |
|---|---|---|---|
| 401 | `authentication_required` | `Missing X-Actor-ID` (dev) / `Missing bearer session`, bearer rác `Session is not valid` (prod) | `deps.py:142-143` |
| 422 | (validation) | `{"detail":[…]}` | `main.py:318-351` |
| 404 | `constraint_kind_unknown` | `Không có ô này.` | `service.py:7530` |
| 404 | `notebook_not_found` | `Không có sổ này.` | `service.py:7231` |
| 403 | `permission_denied` | `role_not_permitted` | `service.py:504` |

## Mã Python

- Route: `services/api/app/api/routes/pair_notebooks.py:136-148`
- Service: `services/api/app/api/service.py:7526-7537`
- Repository: `services/api/app/api/repository.py:7959-7967`

## Test đang phủ

- `services/api/tests/api/test_pair_notebook.py`: `test_both_may_read_the_two_lines_and_only_their_owner_writes_one` (185), `test_an_unknown_constraint_kind_is_not_a_door` (231)

## Kịch bản parity

`parity/scenarios/w8/pair_notebooks/DELETE-contexts-context_id-notebook-constraints-kind.yaml` (31 bước, `dev`): `anonymous_deletes`, `stranger_unknown_kind`, `owner_deletes_on_fresh_pair`, `stranger_deletes_in_pair`, `owner_deletes_in_group`, `owner_deletes_as_advancer`, `owner_kind_uppercase`, `owner_deletes_missing`, `mate_deletes_khong_an_duoc`, `owner_reads_after_mate_delete`, `owner_deletes_khong_an_duoc`, `owner_puts_after_delete`, `mate_reads_version_restart`, `owner_header_key`, `owner_puts_between`, `owner_replays_header_key`, `owner_reads_after_replay`, `owner_deletes_after_close`.

`crossreplay/DELETE-…-constraints-kind.yaml` (13 bước): 204 lưu ở cửa trước được Python phát lại mà không xoá dòng viết lại; kind khác cùng khoá 422.

`concurrency/DELETE-…-constraints-kind.yaml` (9 bước): ba lần xoá cùng lúc → ba 204.

`pair_notebooks/prod-auth.yaml`: `mate_deletes`, `junk_bearer_deletes_constraint`.

Corpus sinh tự động: hoãn, `path kind is str`.

## Chưa phủ / lưu ý cho bản Go

- Kind trước context, sau actor. 204 cho mọi nhánh thành công, kể cả khi không có dòng hoặc không có chu kỳ.
- Phát lại một DELETE đã lưu không chạm DB.

## Lỗi Python (chỉ báo, không sửa)

- Một DELETE trên pair chưa từng mở sổ để lại một hàng `pair_notebooks` đã commit.
- `version` bắt đầu lại từ 1 sau khi xoá: client giữ `version` cũ để phát hiện màn cũ sẽ nhầm dòng mới với dòng đã biết.
