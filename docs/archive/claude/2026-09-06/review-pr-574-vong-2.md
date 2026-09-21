# Review PR #574 — vòng 2, sau commit sửa `ee874ff`

- **Commit được review:** `ee874ff5698b0030ad7ed8a6cfe7d145255c4f72` (head PR). Đọc `git diff 7dd9467..ee874ff`: hai commit — `c961bf9` (doc review vòng 1, không có mã) và `ee874ff` (bản sửa). Mã đổi chỉ ở `services/api` (`git diff --stat 7dd9467..ee874ff -- apps packages scripts tests .github` rỗng); client không đổi.
- **Nhánh → base:** `claude/p0-w-m15-l1-chat` → `claude/p0-w-m15-l0-adr` (`56495e9`).
- **protocol_version:** `v1` (`docs/protocol/v1/01-giao-thuc-thuc-dia.md`), như doc vòng 1.
- **Reviewer:** Claude, context độc lập (không phải tác giả, không phải reviewer vòng 1), ADR-0007. Worktree sạch `/tmp/review2-pr-574` detached tại `ee874ff`, `.env` chép từ cây làm việc; không sửa mã của PR. Mọi kịch bản «tự phá» đặt ngoài cây (scratchpad); đột biến chạy trên một **bản sao** của cây, không phải worktree review.
- **Doc vòng 1:** `docs/archive/claude/2026-09-06/review-pr-574.md` @ `c961bf9` — REQUEST_CHANGES, blocker B1 + S1–S6.
- **Ngày:** 2026-09-06.

## VERDICT: APPROVE

Blocker B1 đóng theo cả ba tiêu chí gỡ chặn; S1–S4 xử lý đúng như trả lời của tác giả (S4 là «không áp dụng», xem dưới); S5/S6 gác lại có lý do. Mọi cổng tôi tự chạy đều xanh với con số khớp lời khai (một lệch do môi trường, đã giải thích ở bảng cổng). Bảy đột biến trên bản sao đều bị test của PR bắt. Tôi thử tự phá theo bốn hướng và **không tìm được** cách làm `GET /contexts/{id}/messages` trả 500 nữa.

**Blocker còn mở: không.**

**Điều kiện merge** (giữ nguyên từ vòng 1, không phải blocker của review này): ADR-0021 vẫn đang ở dòng `🟡 ĐỀ XUẤT` — PR này chỉ merge sau khi Lead đánh ĐÃ CHẤP NHẬN qua #573, rồi đổi base về `main` và chạy lại cổng gốc trên cây gộp (bài học «gộp tự động giữ pin»).

## B1 — đối chiếu từng tiêu chí gỡ chặn với diff

### Tiêu chí 1 — chặn lúc ghi *và* chịu được lúc đọc

| Tầng | Diff `ee874ff` | Đo lại |
|---|---|---|
| Service từ chối tin đã xoá | `service.py` `react_to_message`: sau `_message_in_context`, `if message.kind == "deleted": raise ApiProblem(409, "message_deleted", …)`; `RepositoryConflict("MESSAGE_DELETED")` từ repo cũng map về đúng 409 ấy, `MESSAGE_NOT_FOUND` → 404 cùng câu với `_message_in_context` | Fake (kịch bản vòng 1): `REACT_STATUS 409 {"code":"message_deleted",…}` · `LIST_STATUS 200`. Postgres qua HTTP (ca của PR + probe của tôi): 409 rồi GET 200, `reactions == []` |
| Repository đọc lại `kind` dưới khoá hàng | `repository.py` `add_reaction`: `select(Message).where(id).with_for_update()` → `None` → `MESSAGE_NOT_FOUND`; `DELETED` → `MESSAGE_DELETED`; chỉ sau đó mới `INSERT`. `soft_delete_message` lấy **cùng khoá**, kiểm lại → `MESSAGE_ALREADY_DELETED`; hình dạng hàng sau xoá lấy từ `deleted_shape` | Race thật bằng **hai session** trên Postgres 16 (probe ngoài PR, xem «Tự phá»): `RACE_OUTCOME CONFLICT=MESSAGE_DELETED`, `STRAY_ROWS_IN_TABLE 0`; xoá đôi: `DOUBLE_DELETE_OUTCOME CONFLICT=MESSAGE_ALREADY_DELETED`, `deleted_at` trong DB giữ mốc của lần xoá đầu |
| Đường đọc chịu được hàng lạc | `service.py` `_wire_message`: `reactions=[] if record.kind == "deleted" else (reactions or [])` | Fake: cấy hàng lạc thẳng vào `repository.reactions` → list mặc định / `before=` / `after=` đều 200, `reactions == []`. Postgres: cấy hàng bằng SQL tay cạnh tin đã xoá → `PG_STRAY_LIST_STATUS 200` · `PG_STRAY_WIRE deleted []` · react lại `409 message_deleted` |

`unreact_to_message` không đổi và không cần đổi: nó chỉ **xoá** hàng, không tạo; đo trên tin đã xoá: `UNREACT_STATUS 200 {"reactions":[]}`, không hàng mới; với hàng lạc thì chính người đó gỡ được (`STRAY_UNREACT 200`).

### Tiêu chí 2 — ca ở tầng fake và tầng Postgres, đột biến đỏ

Ca thêm đúng như tác giả khai: fake `tests/api/test_chat_sticker_reply_delete.py` +4 (từ chối + list 200 · thua race → cùng 409 · hàng lạc không đổ feed · xoá đôi → 409), Postgres `tests/postgres/test_message_reply_and_delete_postgres.py` +2 (qua HTTP · dưới khoá ở repo), `tests/postgres/test_message_delete_downgrade_postgres.py` +1. Tổng khớp: gốc 3202 → 3206, tier 608 → 611.

Tiêu chí yêu cầu «đột biến bỏ kiểm ở service → ca đỏ, ghi dòng đỏ vào thân PR» — tác giả **không dán** dòng đỏ (chỉ nhắc «7/7 đột biến bị bắt» của agy trên cây `7dd9467`, tức là *trước* sửa, không kiểm được từ git). Tôi tự chạy bảy đột biến trên **bản sao** của cây tại `ee874ff` (`PYTHONDONTWRITEBYTECODE=1`, neo phải xuất hiện đúng một lần mới ghi, khôi phục file từ cây sạch sau mỗi đột biến, `diff -rq` cuối bảng: `no diff`). Nền xanh trước khi đột biến: fake `22 passed` · Postgres reply/delete `8 passed` · downgrade `1 passed` · probe race `3 passed`.

| Đột biến | Ca đỏ (nguyên văn) | Ca vẫn xanh (đúng ý) |
|---|---|---|
| M1 service: bỏ `if message.kind == "deleted"` trước khi ghi | fake `1 failed, 21 passed` — `test_a_reaction_on_a_deleted_message_is_refused_and_the_thread_still_lists` | Postgres reply/delete `8 passed` — tầng khoá ở repo còn đỡ |
| M2 repo `add_reaction`: bỏ đọc lại `kind` dưới khoá | Postgres `1 failed, 7 passed` — `test_the_repository_refuses_under_the_lock_what_the_service_refuses_before_it`; probe hai session của tôi `2 failed` (`RACE_OUTCOME ADDED=True`) | fake `22 passed` — fake không đi qua repo thật |
| M3 `_wire_message`: gắn lại phản ứng cho hàng `deleted` | fake `1 failed, 21 passed` — `test_a_stray_reaction_on_a_deleted_row_never_fails_the_feed` | — |
| M4 repo `soft_delete_message`: bỏ kiểm `ALREADY_DELETED` dưới khoá | Postgres `1 failed, 7 passed` — cùng ca trên; probe `1 failed` (`FLIPPED_AGAIN`) | — |
| M5 migration: `downgrade()` quay về `DELETE FROM messages WHERE kind = 'deleted'` | downgrade `1 failed` — `psycopg.errors.ForeignKeyViolation: update or delete on table "messages" violates foreign key constraint "fk_context_read_marks_message" on table "context_read_marks"` | — |
| M6 service `delete_own_message`: bỏ map `MESSAGE_ALREADY_DELETED` → 409 | fake `1 failed, 21 passed` — `test_the_second_of_two_simultaneous_deletions_is_a_conflict` | — |
| M7 service `react_to_message`: bỏ map `MESSAGE_DELETED` → 409 | fake `1 failed, 21 passed` — `test_a_tap_that_loses_the_race_to_a_deletion_is_the_same_409` | — |

### Tiêu chí 3 — chạy lại cổng, dán dòng kết quả

Xem bảng «Bằng chứng đã xem» cuối doc: pytest gốc và `postgres_tier.sh` đều xanh.

## Tự phá — không tìm được đường 500

Bốn hướng, mỗi hướng ghi rõ đã thử gì và ra gì:

1. **Qua HTTP trên fake của PR** (`ChatRepositoryWithEdits`, `_client` của `test_chat_intents`): xoá → người khác react (`409`); tác giả react lên tin đã xoá của chính mình (`OWN_REACT_STATUS 409`); react **trước** rồi xoá (hàng bị dọn, list 200 `[]`); unreact trên tin đã xoá (`200`, không tạo hàng); cấy hàng lạc rồi đọc bằng ba dạng cursor + react lại (409) + unreact (200, dọn sạch). `5 passed` — không kịch bản nào ra 500.
2. **Qua repository thật trên PostgreSQL 16** (container dùng-một-lần của reviewer, schema `repository_it_*` do `tests/postgres/conftest.py` migrate): (a) session A `get_message` (đúng thứ tự service làm trong `_message_in_context`) → session B `soft_delete_message` + commit → A `add_reaction` → `CONFLICT=MESSAGE_DELETED`, 0 hàng lạc, GET 200; (b) A đọc → B xoá + commit → A `soft_delete_message` → `CONFLICT=MESSAGE_ALREADY_DELETED`, `DELETED_AT_IN_DB` giữ mốc của B; (c) đối chứng session mới tinh → `FRESH_SESSION MESSAGE_DELETED`. `3 passed`.
3. **Đường ghi khác vào `message_reactions`:** `grep -rn "MessageReaction(" app/` → chỉ `repository.add_reaction`; `grep -rn "\.add_reaction(" app/` → chỉ `service.react_to_message`. Không có intent, companion hay route nào khác ghi phản ứng; migration L1 không chạm bảng này.
4. **Dữ liệu cũ / SQL tay:** hàng phản ứng cấy bằng `INSERT` thẳng cạnh tin đã xoá trên Postgres → GET qua HTTP `200`, hàng `deleted` có `reactions == []`; react lại `409`. Đây là tầng duy nhất đỡ được hàng đến từ ngoài API, và nó đỡ được.

**Một cơ chế cần nói rõ (S7 dưới):** «đọc lại `kind` dưới khoá» tươi trên đường ship *vì* `get_message` trả về `MessageRecord` và thả object ORM, nên identity map (weak) quên nó và `SELECT … FOR UPDATE` dựng object mới từ hàng vừa khoá. Tôi đo đối chứng: giữ **tham chiếu mạnh** tới object ORM trong cùng session (`held = session.get(Message, id)`) rồi để session khác xoá + commit → `HELD_OBJECT_OUTCOME ADDED=True held.kind=text` — SQLAlchemy 2.0.43 **không** làm mới object đã có trong identity map khi `with_for_update()`, và hàng lạc được ghi. Không đường nào trong PR giữ object ORM kiểu đó (service chỉ cầm record), nên hành vi ship là đúng; nhưng nó đúng nhờ vòng đời object chứ không nhờ câu lệnh, và ca `test_the_repository_refuses_under_the_lock…` của PR là **cùng session, tuần tự** — nó vẫn xanh kể cả khi đọc lại bị mù (object trong bộ nhớ đã bị chính lần xoá đầu sửa), tức là nó chứng minh *có* câu kiểm, không chứng minh câu kiểm *nhìn thấy* commit của người khác. Hàng lạc nếu lọt cũng không làm 500 (tầng 3), nên đây là suggestion, không phải blocker.

## S1–S6 — đối chiếu

- **S1 — downgrade `5c1a7e3d9b42` (đóng).** `downgrade()` giờ `UPDATE messages SET kind = 'text', body = 'Tin nhắn đã bị xoá', deleted_at = NULL WHERE kind = 'deleted'` (một câu, để CHECK mốc giờ còn đứng không từ chối), rồi `sticker` → `text`, rồi mới hạ CHECK; docstring sửa cho khớp. Ca mới `test_message_delete_downgrade_postgres.py` dựng schema riêng ở L1 với tin đã xoá + dấu đọc trỏ vào, xuống `e1f2a3b4c5d6` rồi lên lại — nằm trong `611 passed` của tier. Tôi thử thêm dữ liệu khó hơn bằng chính fixture `scratch_schema` của PR: một tin **trả lời trích** tin đã xoá, một **hàng phản ứng lạc** trên nó, một tin đã xoá vốn là ảnh: `AFTER_DOWN [('text', 'Tin nhắn đã bị xoá'), ('text', 'Tin nhắn đã bị xoá'), ('text', 'di-thoi'), ('text', 'trả lời tin đã xoá')]`, hàng phản ứng còn nguyên, lên lại đủ 4 hàng — `1 passed`. Đột biến M5 (quay về `DELETE`) đỏ đúng FK `fk_context_read_marks_message`. Ghi nhận có chủ ý: sau khi lên lại, hai hàng giữ chỗ là `text` chứ không còn `kind = 'deleted'` (mất trạng thái xoá, còn chữ giữ chỗ) — docstring nói rõ, chấp nhận.
- **S2 — docstring `soft_delete_message` (đóng).** Giờ mã kiểm lại `kind` sau `with_for_update()` và ném `MESSAGE_ALREADY_DELETED`; service map 409 `message_already_deleted`. Đo hai session: `CONFLICT=MESSAGE_ALREADY_DELETED`. Thân PR mục «Cái KHÔNG chứng minh» («mô phỏng bằng FOR UPDATE + 409 trong tier Postgres») giờ có ca thật ở tier (cùng session) — xem S7 về giới hạn của ca đó.
- **S3 — `deleted_shape` (đóng).** Repository gọi `deleted_shape({"kind": …}, now)` và áp năm trường; `_now()` trả `datetime.now(UTC)` (tz-aware) nên nhánh `NAIVE_DATETIME` của domain không bao giờ nổ ở production; `tests/test_import_boundary.py` trong bộ gốc xanh (repo import domain là hướng cho phép).
- **S4 — fake conftest (không áp dụng).** Tác giả trả lời về `ChatRepositoryWithEdits`; S1 vòng 1 nói về `tests/api/conftest.py::FakeRepository`. File đó **không đổi** ở `ee874ff` — nhưng nó cũng **không có** `add_reaction`/`reactions` (grep = 0), nên không có bất biến nào để lệch; `soft_delete_message` của nó chỉ đổi hình dạng hàng. Đóng như «không áp dụng», không phải «đã sửa».
- **S5 / S6 — gác có lý do.** Poll `after=` thuộc cơ chế realtime ADR-0024 sẽ đổi; `Clipboard` đổi khi rebuild native ở L7. Không đổi ở lát này là hợp lý; thân PR nên (vẫn chưa) ghi S5 vào «Cái KHÔNG chứng minh».

## Suggestion mới (không chặn)

- **S7 — làm «đọc lại dưới khoá» tươi bằng câu lệnh, không nhờ vòng đời object.** Thêm `.execution_options(populate_existing=True)` vào hai `select(Message)…with_for_update()` (`add_reaction`, `soft_delete_message`), và thêm vào tier Postgres một ca **hai session** (đọc ở A → xoá + commit ở B → ghi ở A) cho cả hai đường; ca cùng-session hiện có không phân biệt được «tươi» với «mù». Kịch bản đo ở mục «Tự phá» 2(a)–(b) và đối chứng «tham chiếu mạnh» là mẫu sẵn.
- **S8 — dán dòng đỏ đột biến vào thân PR** như tiêu chí 2 vòng 1 yêu cầu, để người đọc `main` không phải tin lời «7/7» của một lượt agy trên cây khác.

## Bằng chứng đã xem — dòng kết quả nguyên văn (cây sạch `/tmp/review2-pr-574` tại `ee874ff`)

```
$ python3 -m pytest services/api/tests tests -q                     # gốc worktree, không MOBILE_* trong env
3205 passed, 674 skipped, 2 warnings, 5429 subtests passed in 515.41s (0:08:35)
# lệch 1 so với tác giả (3206 / 673 / 5430): worktree chưa có apps/mobile/node_modules →
#   SKIPPED tests/test_phone_path.py:398: apps/mobile/node_modules chưa cài
$ cd apps/mobile && npm ci --no-audit --no-fund                     → added 659 packages in 12s
$ python3 -m pytest tests/test_phone_path.py -q                     → 41 passed in 0.05s   (ca skip ấy giờ pass; tổng khớp 3206)

$ scripts/postgres_tier.sh -q                                      # container dùng-một-lần do script tự dựng
611 passed in 108.99s (0:01:48)                                     # tests/postgres
88 passed, 19 subtests passed in 14.24s                             # ../../tests/qa
  ĐẠT   tests/postgres
  ĐẠT   ../../tests/qa

$ python3 scripts/repo_guard.py range "$(git merge-base origin/main HEAD)" HEAD    # merge-base d094deb
Repo guard passed commit range: 6240 file scan(s) in 4 commit(s).
$ python3 scripts/repo_guard.py tree HEAD
Repo guard passed tracked tree: 1568 file scan(s).

$ scripts/ruff_changed.sh "$(git merge-base origin/main HEAD)"
ruff over 25 changed Python file(s):
All checks passed!
25 files already formatted

$ (cd services/api && alembic offline upgrade head, sql=True)       → ok
$ python3 scripts/check_alembic_heads.py                            → Alembic guard: mot head duy nhat (5c1a7e3d9b42).
```

Probe của reviewer (ngoài cây, `MOBILE_AUTH_MODE=dev`, `--rootdir services/api -c services/api/pyproject.toml`; Postgres 16 `postgres:16-alpine` dùng-một-lần riêng, không phải container của tier):

```
probe_fake_b1.py        5 passed in 2.31s
  REACT_STATUS 409 {"code":"message_deleted","detail":"Tin nhắn đã bị xoá, không phản ứng được."} · LIST_STATUS 200
  OWN_REACT_STATUS 409 · UNREACT_STATUS 200 {"reactions":[]} · STRAY_LIST_STATUS 200 · STRAY_UNREACT 200
probe_pg_race.py + probe_pg_downgrade.py        4 passed in 3.22s
  RAW_KIND_SEEN_BY_A_AFTER_COMMIT deleted · RACE_OUTCOME CONFLICT=MESSAGE_DELETED · STRAY_ROWS_IN_TABLE 0 · LIST_STATUS 200 · WIRE deleted []
  DOUBLE_DELETE_OUTCOME CONFLICT=MESSAGE_ALREADY_DELETED · DELETED_AT_IN_DB 2030-08-27T12:00:00+00:00
  FRESH_SESSION MESSAGE_DELETED
  AFTER_DOWN [('text', 'Tin nhắn đã bị xoá'), ('text', 'Tin nhắn đã bị xoá'), ('text', 'di-thoi'), ('text', 'trả lời tin đã xoá')]
probe_pg_race.py -k mechanism                   1 passed
  HELD_OBJECT_OUTCOME ADDED=True held.kind=text · HELD_OBJECT_STRAY_ROW True        (đối chứng cho S7)
probe_pg_race.py -k stray_row_planted           1 passed
  PG_STRAY_LIST_STATUS 200 · PG_STRAY_WIRE deleted [] · PG_STRAY_REACT_AGAIN 409 message_deleted
```

**Không chạy:** `npm test` / `tsc` (client không đổi giữa `7dd9467` và `ee874ff`; số 691/691 và tsc 0 lỗi của vòng 1 vẫn là bằng chứng cho cây client này), bảng Maestro / adb / emulator (không có; tác giả cũng nói không chạy lại cho commit này — hợp lý vì chỉ máy chủ đổi và hai flow 37/39 không đi qua nhánh phản-ứng-sau-xoá), `scripts/e2e_slice.sh`.

## Đối chiếu lời khai trả lời của tác giả ↔ đo lại

| Lời khai (comment trên PR) | Đo lại trên `ee874ff` | Khớp |
|---|---|---|
| pytest gốc 3206 passed, 673 skipped, 5430 subtests | 3205 / 674 / 5429 + `test_phone_path.py` 41 passed sau `npm ci` (1 ca skip vì môi trường) | ✓ (lệch do môi trường, đã đóng) |
| `postgres_tier.sh` 611 passed · tests/qa 88 | `611 passed in 108.99s` · `88 passed, 19 subtests passed` | ✓ |
| ruff: All checks passed, 25 files already formatted | y hệt | ✓ |
| repo guard range: 4 commit(s) | `6240 file scan(s) in 4 commit(s)` | ✓ |
| migration offline ok, một head | `ok` · `mot head duy nhat (5c1a7e3d9b42)` | ✓ |
| «Cú bấm thua cuộc đua với lệnh xoá không còn ghi được hàng» | đúng trên đường ship (hai session: `CONFLICT=MESSAGE_DELETED`); cơ chế là vòng đời object, xem S7 | ✓ (kèm S7) |
| S4 «fake của chat đã xoá phản ứng từ trước» | đúng về `ChatRepositoryWithEdits`; S4 vòng 1 nói về `conftest.FakeRepository` — file ấy không có phản ứng để lệch | không áp dụng |
| «7/7 đột biến bị bắt (agy, cây `7dd9467`)» | không kiểm được từ git; tôi chạy 7 đột biến trên bản sao tại `ee874ff`: 7/7 đỏ đúng chỗ | ✓ (bằng chứng của tôi, không phải của agy) |
