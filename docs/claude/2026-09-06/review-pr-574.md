# Review PR #574 — L1 «chat nâng cấp»: sticker, trả lời, xoá tin, cài đặt nhóm

- **Commit được review:** `7dd9467b70a92eebc3aa60ae00718f294eaab886` (head PR). Commit L0 `56495e9` (gói ADR-0021…0025) đã được PR #573 review riêng; doc này chỉ đọc `git diff 56495e9..7dd9467`.
- **Nhánh → base:** `claude/p0-w-m15-l1-chat` → `claude/p0-w-m15-l0-adr`.
- **protocol_version:** `v1` (`docs/protocol/v1/01-giao-thuc-thuc-dia.md`).
- **Reviewer:** Claude, context độc lập (không phải tác giả), ADR-0007. Worktree sạch `/tmp/review-pr-574` detached tại `7dd9467`, `.env` chép từ cây làm việc; không sửa mã của PR.
- **Ngày:** 2026-09-06.

## VERDICT: REQUEST_CHANGES

Một blocker (B1) — thuộc loại *vi phạm spec/cổng* và *bảo mật (khả dụng)*: một thành viên bất kỳ (kể cả vô tình, bằng client còn giữ bản cũ) làm `GET /contexts/{id}/messages` trả 500 cho **cả nhóm**, không tự hồi. Mọi thứ khác trong lát này tôi đo được đều khớp lời khai thân PR; các cổng tự chạy đều xanh với con số đúng như PR ghi.

## Blocker còn mở

### B1 — Máy chủ nhận phản ứng lên tin đã xoá, rồi trang danh sách tin 500 cho mọi thành viên

**Loại:** vi phạm spec/cổng (ADR-0021 §2.3.3 «phản ứng của tin đã xoá bị xoá cùng transaction» và validator wire của chính PR `a deleted message keeps no reactions`) · bảo mật/khả dụng (một thành viên làm nhóm không đọc được chat).

**Dẫn chứng (mã):**

- `services/api/app/api/service.py`, `react_to_message` (≈ dòng 3400): `_require_permission("react_to_message")` → `self._message_in_context(context_id, message_id)` → `self.repository.add_reaction(...)`. `_message_in_context` chỉ kiểm tồn tại + đúng nhóm; **không có chỗ nào kiểm `kind == "deleted"`**. Cùng file, `create_chat_expense_draft` thì có (`409 message_deleted`).
- `services/api/app/api/schemas.py`, `MessageResponse._payload_matches_kind_at_the_wire`: `if self.reactions: raise ValueError("a deleted message keeps no reactions")`.
- `list_context_messages` gọi `_wire_message(record, summaries.get(record.id), ...)` cho mọi hàng của trang → một hàng `deleted` có phản ứng làm pydantic ném ngay trong service → handler `Exception` của `main.py` trả 500 cho **cả trang**, với mọi người đọc.

**Dẫn chứng (đo, kịch bản reviewer nằm ngoài PR, chạy trên chính fake `ChatRepositoryWithEdits` và helper `_client` của PR):**

```
REACT_STATUS 201 {"message_id":"<id>","reactions":[{"kind":"heart","count":1,"mine":true}]}
pydantic_core._pydantic_core.ValidationError: 1 validation error for MessageResponse
  Value error, a deleted message keeps no reactions [type=value_error, ...]
app/api/service.py:862: ValidationError
```

Cùng kịch bản trên **PostgreSQL 16 thật** qua HTTP (`tests/postgres/conftest.py` với container dùng-một-lần, `MOBILE_AUTH_MODE=dev`): tác giả POST tin → DELETE 204 → thành viên khác POST reactions → GET danh sách:

```
LIVE_REACT_STATUS 201 {"message_id":"<id>","reactions":[{"kind":"heart","count":1,"mine":true}]}
LIVE_LIST_RAISED ValidationError 1 validation error for MessageResponse
  Value error, a deleted message keeps no reactions
```

Kịch bản đầy đủ ở cuối doc (mục «Cách tái lập B1»).

**Hậu quả:** hàng `message_reactions` lạc nằm lại trong DB; từ đó `GET /contexts/{id}/messages` (trang mặc định 50 tin, mọi cursor chứa hàng ấy) là 500 với mọi thành viên cho tới khi có người xoá hàng phản ứng bằng tay — không ai trong app làm được vì màn chat không render. Không cần ác ý: `apps/mobile/src/rudi/chat/useTinNhan.ts` `napMoi` poll bằng `after=cursorMoiNhat` nên client của người khác **vẫn thấy tin đã xoá là tin sống cho tới khi mở lại màn** (hàng đổi `kind` không có `created_at` mới); nhấn giữ → ❤ là đủ để làm hỏng chat của cả nhóm. Không chạm tiền, không lộ dữ liệu; nhưng «không đọc được chat» là hỏng tính năng cho mọi người trong nhóm.

**Tiêu chí gỡ chặn:**

1. `react_to_message` (và nên cả `unreact_to_message`) từ chối tin `deleted` bằng một câu 409 (ví dụ tái dùng `message_deleted`), quyết định ở domain/service như `check_deletable`; **và** đường đọc không bao giờ 500 vì một hàng phản ứng lạc (ví dụ `_wire_message` không gắn phản ứng cho hàng `deleted`, hoặc `add_reaction` khoá hàng tin và kiểm lại `kind` trong cùng transaction để không lọt race với `soft_delete_message`). Chặn lúc ghi *và* chịu được lúc đọc — một tầng thôi thì race giữa `add_reaction` và `soft_delete_message` vẫn để lọt.
2. Ca ở tầng fake (`tests/api/test_chat_sticker_reply_delete.py`) và tầng Postgres (`tests/postgres/test_message_reply_and_delete_postgres.py`): xoá → phản ứng của người khác bị từ chối → danh sách vẫn 200 và hàng `deleted` có `reactions == []`. Đột biến bỏ kiểm ở service → ca đỏ (ghi dòng đỏ vào thân PR).
3. Chạy lại `python3 -m pytest services/api/tests tests -q` (gốc) và `scripts/postgres_tier.sh -q`, dán dòng kết quả.

## Suggestion (không chặn)

- **S1 — Downgrade `5c1a7e3d9b42` đỏ khi có dấu đọc trỏ vào tin đã xoá.** Đo trên Postgres 16 dùng-một-lần: seed một tin `deleted` + `context_read_marks.last_read_message_id` trỏ vào nó, rồi `alembic downgrade e1f2a3b4c5d6`:
  ```
  psycopg.errors.ForeignKeyViolation: update or delete on table "messages" violates foreign key constraint "fk_context_read_marks_message" on table "context_read_marks"
  alembic current after failed downgrade: 5c1a7e3d9b42 (head)
  ```
  DDL giao dịch cuộn lại sạch (không kẹt giữa chừng). Gỡ dấu đọc thì xuống rồi lên qua; DB rỗng roundtrip qua. Docstring migration nói «Xuống được» và chính nó nêu lý do giữ hàng là vì read mark trỏ tới — nên `downgrade()` phải dời dấu đọc về tin sống gần nhất cùng nhóm (hoặc xoá dấu đọc) và dọn phản ứng còn sót trước `DELETE`. Thêm một ca roundtrip có dữ liệu vào tier Postgres. Không xếp blocker vì downgrade không nằm trong cổng PR và chưa có dữ liệu thật; nhưng lời khai «xuống được» hiện chỉ đúng với DB rỗng.
- **S2 — Docstring `soft_delete_message` không khớp mã.** Docstring nói lượt thứ hai «finds `deleted` and is answered 409», nhưng sau `with_for_update()` không kiểm lại `kind`: hai cú bấm «Xoá» cùng lúc của chính tác giả → cả hai 204, `deleted_at` bị ghi đè. Vô hại (chỉ tác giả xoá được) nhưng comment sai giữ lỗi sống; ca `test_deleting_twice_is_a_conflict_not_a_second_deletion` là tuần tự (409 đến từ `check_deletable`), không phải race. Thân PR mục «Cái KHÔNG chứng minh» nói race «mô phỏng bằng `SELECT … FOR UPDATE` + 409 trong tier Postgres» — không có ca nào như vậy; nên sửa mã (kiểm sau khoá) hoặc sửa lời khai.
- **S3 — `app/domain/message_edit.deleted_shape` không được service dùng.** `delete_own_message` gọi `check_deletable` rồi để `soft_delete_message` tự viết hình dạng hàng xoá; domain và repository là hai cách viết không đối chiếu. Hoặc service tính shape bằng `deleted_shape` rồi repo ghi, hoặc bỏ hàm.
- **S4 — `FakeRepository.soft_delete_message` (tests/api/conftest.py) không xoá phản ứng** trong khi repo thật có; fake lệch bất biến. (Fake trong `test_chat_sticker_reply_delete.py` thì có xoá.)
- **S5 — «Mọi người sẽ thấy “Tin nhắn đã bị xoá”» chỉ đúng sau khi mở lại màn** (xem `napMoi` ở B1). Là hành vi poll có sẵn từ M3 (phản ứng của người khác cũng không được làm mới), ADR-0024 mới đổi; nhưng nên ghi rõ vào «Cái KHÔNG chứng minh» — flow 37 chỉ đo màn của người xoá.
- **S6 — `MenuTin` dùng `Clipboard` lõi RN (deprecated)** — tác giả đã ghi chú, theo dõi ở ADR-0024 §2.5.

## Điều kiện merge (không phải blocker của review này)

Thân PR tự ghi: ADR-0021 phải được Lead đánh ĐÃ CHẤP NHẬN (PR #573) trước, rồi đổi base về `main`. Khi đổi base, chạy lại cổng gốc trên cây gộp (bài học «gộp tự động giữ pin»).

## Tám điểm phải nhìn kỹ

1. **Tin `deleted` không mang nội dung ở ba tầng.** Domain: `deleted_shape` đưa `body/image_url/card` về `None` (nhưng xem S3 — service không đi qua nó; `soft_delete_message` tự đặt cả ba về `None`). CHECK: `payload_matches_kind` nhánh `deleted` + `deleted_state_matches_timestamp` (`tests/postgres/test_message_reply_and_delete_postgres.py` đỏ đúng tên constraint khi giữ chữ / thiếu mốc giờ). Wire: `MessageResponse` validator (`test_deleted_message_wire_shape.py`, 5 kiểu rò rỉ). Cách lách tôi tìm: `MessageCreateRequest.kind` không có `deleted` (test có); không có route sửa tin; `reply_to` của hàng deleted vẫn giữ trích của tin *gốc* (tin cùng nhóm, không phải nội dung đã xoá — hợp lệ). Lỗ duy nhất tìm thấy không phải nội dung mà là **phản ứng** — B1.
2. **Xoá tin người khác → 403; không xoá `ai_card`.** Thứ tự trong `delete_own_message`: thành viên nhóm (403 cho người ngoài, trước khi đọc hàng) → `_message_in_context` (404 chung cho không tồn tại / khác nhóm) → `check_deletable`: `ALREADY_DELETED` 409 · `NOT_AUTHOR` → `_require_permission("delete_own_message", is_author=False)` 403 · `KIND_NOT_DELETABLE` 409. Thẻ không tác giả → 403; thẻ đứng tên mình → 409 (`test_a_poll_card_in_ones_own_name_cannot_be_deleted`). Cả fake lẫn Postgres (`stranger_delete` 403) đều có ca. `kiem_may_chu_sau_37` canary curl cùng ý.
3. **`reply_to_id` chéo nhóm.** DB: FK kép `(reply_to_id, context_id) → messages(id, context_id)` + `uq_messages_id_context`; ca Postgres bắt đúng `fk_messages_reply_to`. Service: `check_reply_target` kiểm `context_id` **trước** kiểm `kind`, ném `REPLY_NOT_FOUND` → `ApiProblem(404, "message_not_found", "Message does not exist")` — đúng từng chữ với `_message_in_context`; ca fake so `absent.json() == cross.json()`. Kiểm mục tiêu trả lời nằm sau kiểm thành viên nên người ngoài nhóm không dùng `reply_to_id` làm oracle.
4. **Từ vựng sticker/theme ba nơi.** `tests/test_sticker_vocabulary_matches_client.py` đọc AST `stickers.py` ↔ `stickers.json` ↔ regex `STICKER_IDS`/khoá `HINH` trong `sticker.ts` (đủ cả «id có hình»). `tests/test_chat_theme_matches_tokens.py`: AST `chat_theme.py` ↔ khoá `tokens.json.chatTheme` ↔ `THEME_CHAT` trong `mau-chat.ts` + cấm hex trong `mau-chat.ts`. `services/api/tests/web/test_chat_theme_tokens.py`: domain ↔ tokens ↔ regex CHECK trong migration, đủ light/dark, `bubbleInk/bubble ≥ 4.5`, `mac-dinh = accent/accentInk`. `test_context_settings.py`/`test_chat_theme.py`: `Literal ChatTheme == THEMES`. CHECK trong `models.py` ↔ migration do `test_migration_matches_models` giữ. Chỗ chưa có test đối chiếu: ba bản regex `^[a-z0-9-]{1,32}$` (migration, models, `STICKER_ID_PATTERN`) — nhỏ, không chặn.
5. **Migration `5c1a7e3d9b42`.** Biên dịch offline `ok`; một head. Chỉ có `op.*` trên `messages` và `contexts`; grep tên bảng tiền (`expense|obligation|allocation|collection_batch|bill|receipt|payment`) trong file = 0. Trên Postgres 16 thật: `upgrade head` → `downgrade e1f2a3b4c5d6` → `upgrade head` qua trên DB rỗng; **đỏ** khi có read mark trỏ vào tin đã xoá (S1).
6. **Client.** Grep các file client đổi (trừ `theme.ts`): không hex (`#[0-9a-fA-F]{3,8}`), không `?? id`, không `—` ở dòng thêm; `theme.ts` chỉ thêm re-export, màu nằm ở `tokens.json`; `bangMauChat` viết fallback dạng câu lệnh (không `?? "mac-dinh"`) như comment nói. `tsc --noEmit` 0 lỗi; `npm test` 691/691 với `MOBILE_REQUIRE_WEB_A11Y=1`.
7. **Route mới có literal client; 204.** `check_server_routes_called`: «Không có route mới nào bị bỏ rơi» — `DELETE /contexts/${contextId}/messages/${messageId}` ở `tin-song.ts` `xoaTin`, `PATCH /contexts/${contextId}` ở `nhom-cai-dat.ts` `doiNhom`. Route DELETE khai `status_code=204` + `responses=ERRORS` ở `routes/messages.py` và pin ở `ROUTES_WITHOUT_RESPONSE_VALIDATION` (`test_money_api_boundary_is_integer.py`); tôi không tìm thấy registry thứ hai nào cho route 204 (grep `sessions/current` chỉ ra một chỗ) và bộ gốc xanh nên không thiếu chỗ khai. `.server-routes-uncalled.json`, `main.py`, `deps.py` không đổi (`git diff --stat` rỗng) — khớp lời khai «không thêm dòng, `_KNOWN_DOORS` không đổi».
8. **Ba sửa sau bảng Maestro.** Dấu vân trong thân PR cho biết bảng chạy trên HEAD L0 + cây chưa commit, nên **không kiểm được từ git** ba diff nào đến sau. Đánh giá từng cái: (a) điều kiện `tin.kind !== "deleted"` trên khối trích trong `GroupChatLive.tsx` chỉ đổi cách vẽ hàng đã xoá; flow 37 sau bước xoá chỉ assert «Tin nhắn đã bị xoá» hiện và «Tra loi QA» không hiện — không assert gì về khối trích → không đổi phép đo nào; nhưng ảnh `37-da-xoa` là **trước** sửa, và không có test nào chạy JSX này (`rudi-chat-tra-loi-xoa.test.mjs` chỉ đo `thayTinDaXoa`) — bằng chứng cho hình dạng hàng đã xoá sau sửa là đọc mã, không phải đo. (b) hai đường `d` viết bằng builder `duong(...)` («an-gi» cái tô, ô «?» của `HINH_KHAC`): test node parse ngữ pháp Java + biên khung qua; chuỗi trước sửa không có trong git nên không so được «cùng chuỗi kết quả»; hình trên máy sau sửa chưa được đo lại. (c) ca Postgres nhích đồng hồ — chỉ test. Kết luận: ba sửa không đổi hành vi mà bảng **đã assert**, nhưng (a) và (b) đổi pixel mà bảng **không assert**; chấp nhận được, ghi rõ ở đây.

## Maestro — phần KHÔNG tái lập được

Tôi không có emulator; không chạy `adb`/Maestro. Chỉ đối chiếu YAML với mã:

- `37-chat-sticker-tra-loi-xoa.yaml` ↔ client: «Gửi sticker» (IconButton), «Khay sticker» (Sheet label), «Đi thôi!» (ô khay), «Sticker: Đi thôi!» (`Sticker` label), «Tin nhắn: Dep qua» (longPress), «Trả lời» (ListRow), «Đang trả lời .*» (dải), «Trích: Dep qua» (label khối trích), «Xoá» → «Xoá tin» (xác nhận trong sheet), «Tin nhắn đã bị xoá» (hàng + `xemTruocTinCuoi` ở danh sách «Bạn: …») — mọi nhãn đều có trong mã đúng nguyên văn.
- `39-cai-dat-nhom.yaml` ↔ client: «Cài đặt nhóm» (pill), «Màu bong bóng», «Ô tên nhóm», «Lưu tên», «Theme Hoàng hôn» (`Theme ${nhanTheme}`), «Hoàng hôn. Cả nhóm thấy cùng một màu.» (một node Text), «Đóng» (scrim), «Rời nhóm» → «Rời nhóm này?.*» → «Rời nhóm» (ở bước xác nhận chỉ còn một nút tên đó) — khớp.
- `scripts/mobile_native.sh` `kiem_may_chu_sau_37`: đọc 50 tin của «Hoi QA» bằng phiên C: ≥1 sticker có body trong VOCAB (8 id đúng với domain), đúng 1 `deleted` và nó sạch (`body/image_url/card` None, có `deleted_at`, không `reactions`), ≥1 tin có `reply_to.id` = id tin «Dep qua»; canary người khác DELETE sticker → 403; PATCH `{"theme":"hong"}` → 422. `kiem_may_chu_sau_39`: danh sách nhóm của người lái không còn «Nhom Da Doi Ten»/«Nhom Cai Dat QA»; theme «Hoi QA» = `mac-dinh`. Hai hàm được nối vào vòng kiểm sau bảng (`da_chay 37/39 && kiem_can_25 …`), 37/39 vào nhóm chỉ chạy khi `--otp`, biến `OTP_PHONE_E` đi qua `-e` cho cả bảng lẫn canary. `tests/test_maestro_flows_dev_client.py` pin hai flow + regex số E.
- Điều tôi **không** chứng minh: flow chạy xanh trên máy thật; sticker vẽ ra hình; theme đổi màu bong bóng; ba sửa sau bảng không đổi pixel (mục 8).

## Đối chiếu lời khai thân PR ↔ đo lại

| Lời khai | Đo lại trên `7dd9467` | Khớp |
|---|---|---|
| pytest gốc 3202 passed, 670 skipped | `3202 passed, 670 skipped, 2 warnings, 5429 subtests passed in 505.70s (0:08:25)` | ✓ |
| tests/postgres 608 · tests/qa 88 | `608 passed in 119.64s (0:01:59)` · `88 passed, 19 subtests passed in 14.70s` | ✓ |
| tsc 0 lỗi · npm test 691/691 | `tsc RC=0` (không dòng lỗi) · `# tests 691 / # pass 691 / # fail 0` | ✓ |
| ruff_changed 0 finding, 0 file cần format | `ruff over 24 changed Python file(s)` · `All checks passed!` · `24 files already formatted` | ✓ |
| repo guard PASS | `Repo guard passed commit range: 3105 file scan(s) in 2 commit(s).` · `Repo guard passed tracked tree: 1566 file scan(s).` | ✓ |
| migration offline ok · một head | `ok` · `Alembic guard: mot head duy nhat (5c1a7e3d9b42).` | ✓ |
| contract · routes called · màn | `Client và máy chủ khớp hợp đồng.` · `Không có route mới nào bị bỏ rơi.` · `41/41 màn có đường render từ cửa vào · 0 pin · 198 file đã đọc` | ✓ |
| «tin deleted không mang nội dung ở wire» | đúng cho `body/image_url/card`; **phản ứng** thì lọt (B1) | ✗ |
| «race hai người cùng xoá: mô phỏng FOR UPDATE + 409» | không có ca nào như vậy; mã không kiểm lại sau khoá (S2) | ✗ |
| «migration xuống được» (docstring) | DB rỗng: qua; có read mark trỏ tin đã xoá: đỏ FK (S1) | một phần |
| `.server-routes-uncalled.json` không thêm dòng · `_KNOWN_DOORS` không đổi | `git diff --stat` rỗng cho hai file | ✓ |
| ROUTES_WITHOUT_RESPONSE_VALIDATION + DELETE messages | có dòng, bộ gốc xanh | ✓ |
| e2e_slice 10/10 | không chạy lại (không nằm trong danh sách cổng được giao; cần stack docker) | — |

## Bằng chứng đã xem — dòng kết quả nguyên văn (cây sạch `/tmp/review-pr-574` tại `7dd9467`)

```
$ python3 -m pytest services/api/tests tests -q            # gốc worktree
3202 passed, 670 skipped, 2 warnings, 5429 subtests passed in 505.70s (0:08:25)

$ scripts/postgres_tier.sh -q
608 passed in 119.64s (0:01:59)                             # tests/postgres
88 passed, 19 subtests passed in 14.70s                     # ../../tests/qa
  ĐẠT   tests/postgres
  ĐẠT   ../../tests/qa

$ cd apps/mobile && npm ci --no-audit --no-fund && rm -rf dist-test && npx --no-install tsc --noEmit && MOBILE_REQUIRE_WEB_A11Y=1 npm test
added 659 packages in 11s
tsc RC=0
# tests 691
# pass 691
# fail 0

$ python3 scripts/check_server_routes_called.py
Máy chủ khai 95 route. 83 có người gọi, 5 miễn (người gọi ở nơi khác), 7 đang nợ (đã ghi nhận là chưa ai gọi), 0 không ai gọi và chưa ghi nhận.
Không có route mới nào bị bỏ rơi.

$ python3 scripts/repo_guard.py range "$(git merge-base origin/main HEAD)" HEAD
Repo guard passed commit range: 3105 file scan(s) in 2 commit(s).
$ python3 scripts/repo_guard.py tree HEAD
Repo guard passed tracked tree: 1566 file scan(s).

$ (alembic offline upgrade head, sql=True)      → ok
$ python3 scripts/check_alembic_heads.py        → Alembic guard: mot head duy nhat (5c1a7e3d9b42).
$ python3 scripts/check_api_contract.py         → Máy chủ có 95 route. Đọc được 102 đường dẫn qua 114 lần gọi trong 22 file, 12 chỗ không phân giải được. / Client và máy chủ khớp hợp đồng.
$ python3 scripts/check_screens_reachable.py    → 41/41 màn có đường render từ cửa vào · 0 pin · 198 file đã đọc
$ scripts/ruff_changed.sh origin/main           → ruff over 24 changed Python file(s) / All checks passed! / 24 files already formatted

# Postgres 16 dùng-một-lần (docker postgres:16-alpine, MOBILE_DATABASE_URL trỏ vào):
$ alembic upgrade head && alembic downgrade e1f2a3b4c5d6 && alembic current && alembic upgrade head && alembic current
Running downgrade 5c1a7e3d9b42 -> e1f2a3b4c5d6 … / e1f2a3b4c5d6 / Running upgrade e1f2a3b4c5d6 -> 5c1a7e3d9b42 … / 5c1a7e3d9b42 (head)
# seed: 1 tin deleted + 1 context_read_marks trỏ vào nó, rồi:
$ alembic downgrade e1f2a3b4c5d6
psycopg.errors.ForeignKeyViolation: update or delete on table "messages" violates foreign key constraint "fk_context_read_marks_message" on table "context_read_marks"
alembic current after failed downgrade: 5c1a7e3d9b42 (head)
```

Không chạy: bảng Maestro (không có emulator), `scripts/e2e_slice.sh`.

## Cách tái lập B1 (kịch bản reviewer, không nằm trong PR)

Fake (đặt file ngoài cây, `PYTHONPATH=services/api`, `MOBILE_AUTH_MODE=dev`, `--rootdir services/api -c services/api/pyproject.toml`):

```python
from tests.api.helpers import actor_headers
from tests.api.test_chat_intents import CONTEXT_ID, MEMBER_ID, _client
from tests.api.test_chat_sticker_reply_delete import OTHER_MEMBER_ID, ChatRepositoryWithEdits, RecordingCompanion, _text

def test_react_to_deleted_then_list(monkeypatch):
    repo = ChatRepositoryWithEdits()
    client = _client(repo, RecordingCompanion(), monkeypatch)
    posted = _text(client, "gửi nhầm").json()
    assert client.delete(f"/contexts/{CONTEXT_ID}/messages/{posted['id']}",
                         headers=actor_headers(actor_id=MEMBER_ID)).status_code == 204
    reacted = client.post(f"/contexts/{CONTEXT_ID}/messages/{posted['id']}/reactions",
                          json={"kind": "heart"}, headers=actor_headers(actor_id=OTHER_MEMBER_ID))
    assert reacted.status_code in (404, 409, 422)   # hiện: 201
    assert client.get(f"/contexts/{CONTEXT_ID}/messages",
                      headers=actor_headers(actor_id=MEMBER_ID)).status_code == 200   # hiện: ValidationError → 500
```

Postgres: cùng bốn bước với `_app/_call/_group/_headers` của `tests/postgres/test_group_recap_postgres.py` và `_join` của `test_message_reply_and_delete_postgres.py`; cần `MOBILE_AUTH_MODE=dev` vì `_app` gọi `create_app()` không tham số.
