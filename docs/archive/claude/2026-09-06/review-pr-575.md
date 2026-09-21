# Review PR #575 — L2 «nhắn riêng 1:1» là một context hai người (ADR-0021 §2.5)

- **Commit được review:** `bfa3389e69dd5e07cff55a5ebf7689bd1c65e231` (head PR). Lát là **một** commit trên base; đọc `git diff origin/claude/p0-w-m15-l1-chat..bfa3389`: 26 file, +1667/−61.
- **Nhánh → base:** `claude/p0-w-m15-l2-dm` → `claude/p0-w-m15-l1-chat` (`8aa3272`, PR #574 đã APPROVE vòng 2 — `docs/archive/claude/2026-09-06/review-pr-574-vong-2.md`).
- **protocol_version:** `v1` (`docs/protocol/v1/01-giao-thuc-thuc-dia.md`), như hai doc #574.
- **Reviewer:** Claude, context độc lập (không phải tác giả, không phải reviewer của #574), ADR-0007. Worktree sạch `/tmp/review-pr-575` detached tại `bfa3389`, `.env` chép từ cây làm việc; **không sửa mã của PR**. Mọi probe «tự phá» và mọi đột biến chạy trên một **bản sao** của `services/api` (scratchpad) và một worktree đột biến riêng (`/tmp/review-pr-575-mut`, đã xoá sau khi dùng); không file probe nào nằm trong cây PR. Postgres cho probe là container riêng của reviewer (`postgres:16-alpine`, cổng loopback ngẫu nhiên), không phải DB của lane nào.
- **Ngày:** 2026-09-06.

## VERDICT: APPROVE

**Blocker còn mở: không.**

Tám điểm được yêu cầu nhìn kỹ đều đo được và đều khớp lời khai của thân PR, kể cả hai điều thân PR tự nhận là «không chứng minh»: tôi đo **đua hai session thật** trên Postgres 16 (người thua bị khoá 1,01 s rồi đọc lại đúng hàng người thắng) và đo cơ chế `populate_existing` bằng đúng kịch bản «tham chiếu mạnh» của S7 #574 (giờ là `CONFLICT`, trước là `ADDED=True`). Mọi cổng tôi tự chạy đều xanh với con số khớp thân PR (một lệch 1 ca do môi trường, đã đóng — xem bảng cổng). 14 đột biến trên bản sao: 11 bị bắt bởi test của PR; **3 chỉ bị bắt bởi probe của tôi** (hai cửa roster chưa có ca, và nửa «ca đo» của S7) — đây là lỗ trong *test*, không phải trong *mã*, nên là suggestion S1/S3 chứ không phải blocker: hành vi đã đo đúng ở cả sáu cửa trên Postgres thật qua HTTP.

**Điều kiện merge** (giữ nguyên từ thân PR, không phải blocker của review này): #574 merge trước rồi đổi base về `main`; ADR-0021 phải được Lead đánh ĐÃ CHẤP NHẬN (#573); chạy lại cổng gốc trên cây gộp (bài học «gộp tự động giữ pin»). Khi đổi base, S1 là chỗ nên thêm hai ca trước khi bấm merge.

## Tám điểm phải nhìn kỹ — mã đọc gì, probe đo gì

### (1) Mọi lý do từ chối mở DM là MỘT 404 cùng byte

**Mã.** `service.py::open_direct_message`: `unavailable = ApiProblem(404, "person_not_found", "Chưa thể nhắn riêng với người này.")` dựng **một lần**; `are_friends` (SQL: chỉ hàng `state == ACCEPTED` hai chiều; fake: cùng luật) → `_require_permission("open_direct_message", …, {"is_friend"})` 403 → bắt và ném lại `unavailable`; rồi `get_person` → `can_open(is_friend, other_exists)` → cũng `unavailable`. Bạn bè được kiểm **trước** tồn tại, nên id không tồn tại không bao giờ tới `get_person` và không có nhánh nào tự dựng câu riêng. `permissions.py` thêm entry `open_direct_message` với `requires: ("is_friend",)`; `direct.can_open` nói *có/không*, không nói *vì sao*.

**Đo (probe fake, so `r.content` từng byte, không so JSON đã parse).** Chín biến thể: lời mời tôi gửi còn chờ · lời mời họ gửi còn chờ · cùng nhóm nhưng không có lời mời · người lạ · id ngẫu nhiên · lời mời đã bị từ chối · ba chiều ngược (người chờ / người được chờ / người lạ mở với tôi):

```
DISTINCT_STATUS {404} DISTINCT_BODIES 1        (mỗi thân len=85, cùng content-type)
FRIEND 201 SELF 422 {"code":"self_direct_message", …}
số hàng cặp sau chín ca từ chối: 1 (chỉ của cặp bạn thật)
```

Trên Postgres thật qua HTTP (ca của PR `…outsiders_get_the_one_404`): `len({r.text for r in refusals}) == 1` cho ba ca, và không hàng `kind='pair'` nào mọc cho người ngoài. Đột biến M3 (đổi 404 chung thành 403): 2 ca của PR + probe đỏ. M11 (bỏ điều kiện bạn bè): 3 ca đỏ. M14 (bỏ 422 tự nhắn): 2 ca đỏ.

**Ghi nhận, không chặn.** Cuối hàm còn một 404 *khác byte* (`context_not_found`, khi cặp có mà actor không thấy nó trong danh sách của mình) — chỉ tới được khi DB không nhất quán, vì actor luôn là một trong hai id của `pair_key`; không phải oracle về quan hệ. 422 tự nhắn là thông tin actor sẵn có.

### (2) Đua hai người cùng mở: `uq_contexts_pair_key` + `RepositoryConflict("PAIR_EXISTS")`

**Mã.** `repository.py::create_pair_context`: context + hai membership ACTIVE trong một `begin_nested()`; `IntegrityError` có `diag.constraint_name == "uq_contexts_pair_key"` → `RepositoryConflict("PAIR_EXISTS")`, lỗi khác ném tiếp. Service có `get_pair_context` *trước* để trả 200 cho đường thường, nhưng đường tạo không tin nó: thua → đọc lại `get_pair_context(key)`, vẫn không có → 409 `pair_exists` (nhánh phòng thủ). Migration `6d2b8f4e0c53`: `kind` (`String(8)`, default `group`), `pair_key` TEXT UNIQUE, CHECK `kind IN ('group','pair')`, CHECK `(kind='pair') = (pair_key IS NOT NULL)`; `models.py` khai đúng ba ràng buộc (cổng «migration khớp models» nằm trong 3224 ca xanh).

**Đo — hai session THẬT (probe ngoài PR, thread + hai `Session` trên cùng engine).** A INSERT chưa commit → B INSERT bị khoá trên unique index → A commit sau 1 s → B nhận lỗi:

```
RACE_REPO    a=(<id A>, True, '')  b=('CONFLICT', 'PAIR_EXISTS', <id A>)  b_wait=1.01  rows=1  members=[(<a>,'active'),(<b>,'active')]  stored_names=['']
RACE_SERVICE a=(<id A>, True, 'Bình')  b=('OK', <id A>, False, 'An')  b_wait=1.02  rows=1  members=2  stored_names=['']
```

Tức là: người thua đọc lại **đúng hàng người thắng**, DB còn đúng một hàng và hai membership ACTIVE, và qua service người thua nhận `created=False` với cùng id, tên là người kia. Thân PR ghi «chỉ mô phỏng tuần tự» — giờ có bằng chứng song song, nhưng là bằng chứng của reviewer, chưa nằm trong tier (S3 gợi ý đưa vào). Đột biến M12 (chỉ ghi một membership): 2 ca Postgres của PR + 2 probe đua đỏ.

### (3) Cửa roster trên cặp: 409 **sau** bước quyền, cả sáu cửa

**Mã (đọc trọn sáu hàm).** `_require_permission` luôn đứng trước `_require_group_kind`: `invite_context_member`, `leave_context`, `set_context_member_role`, `create_outing_invite` (sau `invite_to_outing`), `accept_context_membership` (sau cả hai nhánh `origin`), `update_context` (kind chỉ khi có `display_name`, sau `edit_context`). `_require_group_kind` đọc `get_context` và chỉ ném khi `is_pair`; `ROSTER_ONLY_DOORS` là tuple đóng 6 tên.

**Đo trên Postgres thật qua HTTP (probe, ba kịch bản).**

```
INVITE_STRANGER 403 is_group_member · STRANGER_DOOR invite/leave/rename 403 is_group_member · role 403 is_group_admin
MEMBER_DOOR invite 409 not_a_group · leave 409 · rename 409 · role 403 is_group_admin (kể cả header khai group_admin ở dev)
OUTING_IN_PAIR 201 (kèo trong cặp vẫn được, §2.5.5) → INVITE_FRIEND 409 not_a_group · INVITE_LINK 409 not_a_group
ACCEPT (INVITED trồng tay lên cặp): người không phải invitee 403 is_invitee · invitee 409 not_a_group · hàng vẫn INVITED
```

Hai điều cần nói thẳng cho người đọc `main`:

- Cửa **đổi vai trò** với thành viên cặp là **403**, không phải 409: không ai là admin của một cặp nên `set_member_role` từ chối ở bước quyền trước khi tới kind. Đúng «kiểm sau bước quyền», nhưng câu «thành viên cặp 409» trong thân PR không áp dụng cho cửa này; test fake của PR ghi nhận đúng (S7).
- Ca của PR chỉ đi **3/6** cửa (invite, leave, rename) ở cả hai tầng, cộng role. **Duyệt** và **mời vào kèo** không có ca nào. Đột biến M1 (bỏ kind ở `accept_context_membership`) và M2 (bỏ kind ở `create_outing_invite`): **toàn bộ test của PR xanh**, chỉ probe của tôi đỏ. M6 (đảo kind lên trước quyền ở `leave_context`, tức người lạ nhận 409 thay vì 403): test PR xanh, probe đỏ. ADR-0021 §6 tự đặt tiêu chí «bỏ `_require_group_kind` ở một cửa roster → ca âm cửa đó đỏ» — với hai cửa, tiêu chí đó hôm nay chưa đạt. Mã đúng, test thiếu → **S1**.

### (4) Tên cặp là dẫn xuất; `display_name` trong DB rỗng; không in id

**Mã.** `direct.display_name_for(kind, stored, counterpart)`: nhóm → tên nhóm; cặp → tên người kia; thiếu → `ANONYMOUS_COUNTERPART = "Thành viên"` (một từ, có test «không có dấu gạch»). Dùng ở `list_person_context_summaries` (SQL: hai truy vấn gộp cho mọi cặp trong danh sách; fake tương ứng), `_context_response` (GET/PATCH `/contexts/{id}`, POST `/contexts` — thay ba chỗ dựng `ContextResponse` bằng tay) và `_context_summary.counterpart`. `create_pair_context` ghi `display_name=""`; `update_context` từ chối đặt tên cho cặp (409). Client: `tenCuocTroChuyen` (cặp → `counterpart.display_name ?? display_name`, trim, rỗng → «Thành viên»), dùng ở `GroupChatLive`, `Conversations` (cả `Avatar`), `Members`.

**Đo.** Ca Postgres của PR kiểm `display_name == ""` trong hàng sau khi PATCH theme; probe của tôi sau đua: `stored_names=['']`. Đột biến M13 (cặp lấy tên lưu): fake 4 đỏ, Postgres 1 đỏ + probe đỏ. Client M9 (tên khuyết in `counterpart.id`): ca «không bao giờ in id» đỏ.

**Một đường đọc còn thô (S2).** `repository.py::_finance_movements` (≈ L5968) SELECT `Context.display_name` thẳng vào `FinanceMovement.context_name`. Đo trên Postgres (chia bill trong cặp bằng `Slice` của `test_person_finance_postgres.py`, publish, xác nhận nhận tiền):

```
PAIR_FINANCE_MOVEMENT repo: context_name='' counterparty_name='An' occasion='Lẩu nấm'
PAIR_IN_LIST [('pair', 'An')]
```

Là chuỗi rỗng, **không phải id**; `apps/mobile/src/screens/ca-nhan/tai-chinh.ts` chỉ khai kiểu, không màn nào render `context_name` (grep 0 chỗ) — nên hôm nay không có gì hiện sai. Nhưng §2.5.6 nói «máy chủ điền tên người kia khi trả về», và đây là một đường trả về không điền.

### (5) `chonNhomMacDinh` không chọn cặp; hồ sơ đếm «nhóm» không đếm cặp

`phien.ts`: `find(my_state === "active" && kind !== "pair")` — đột biến M8 (bỏ điều kiện) → ca client đỏ. `Conversations.moNhom` không gọi `chonNhom` khi là cặp (màn tiền giữ nhóm cũ, chỉ chat mở); `DangBaiScreen` lọc cặp khỏi «Một nhóm». Máy chủ `profile_counts` join `Context.kind == "group"`; fake lọc `kind != "pair"`. Đột biến M5a (SQL) → ca Postgres `…does_not_count_as_a_group…` đỏ; M5b (fake) → ca fake đỏ. Ghi nhận: `outings` trên hồ sơ vẫn đếm kèo trong cặp — docstring nói rõ, và ADR cho phép kèo trong cặp.

### (6) Client: hex, `?? id`, `—`, literal route

- Dòng thêm ở 9 file client: `#[0-9a-fA-F]{3,8}` = 0 · `\?\?\s*…id` = 0 · `—` = 0 (một em dash duy nhất trong toàn diff nằm ở docstring Python của migration, không phải chuỗi UI).
- Route mới có literal: `translatedAsActor(…, \`/people/${personId}/dm\`, …)` trong `nhan-rieng.ts`; `check_server_routes_called.py` → «Không có route mới nào bị bỏ rơi»; `check_api_contract` → «Client và máy chủ khớp hợp đồng».
- Sheet «Cài đặt» ẩn đổi tên/rời khi `kind === "pair"` từ **L1** (`CaiDatNhom.tsx` @ `7dd9467` đã có `kind?: "group" | "pair"` và `laPair`); L2 chỉ truyền `kind` vào — lời khai «sheet chỉ còn theme» đúng nhưng là việc L1 đã làm sẵn.
- `RudiButton` nhận `accessibilityLabel` → «Nhắn tin cho <tên>» (flow 41 bấm bằng nhãn này). `tsc --noEmit` 0 lỗi; `npm test` 697/697; đột biến client M10 (`ghepVaoDanhSach` không thay hàng cũ) đỏ đúng ca.

### (7) Hai việc nhỏ của L1 gộp vào lát này

- **Đếm nhóm bỏ cặp:** đúng, có ca ở cả hai tầng và cả hai đột biến đều bị bắt (xem (5)).
- **`populate_existing=True` cho hai `SELECT … FOR UPDATE` (`add_reaction`, `soft_delete_message`) — S7 của #574:** đúng về cơ chế. Đo theo đúng kịch bản đối chứng của #574 (session A giữ **tham chiếu mạnh** `A.get(Message, id)`, session B xoá + commit, A ghi):

  ```
  HELD_REACT ('CONFLICT', 'MESSAGE_DELETED')   HELD_DELETE ('CONFLICT', 'MESSAGE_ALREADY_DELETED')
  ```

  (#574 vòng 2 đo cùng kịch bản trước sửa: `ADDED=True held.kind=text`.) Nhưng **không có ca đo trong PR** cho nửa này — S7 yêu cầu «thêm vào tier Postgres một ca hai session». Đột biến M4 (bỏ `populate_existing` ở đúng hai `select(Message)…with_for_update()`; neo xuất hiện đúng 2 lần): `tests/postgres/test_message_reply_and_delete_postgres.py` 8 xanh, `test_direct_message_postgres.py` 7 xanh, chỉ probe `HELD_*` của tôi đỏ (`1 failed, 17 passed`). Thân PR khai «S7» như đã làm; đúng nửa mã, thiếu nửa ca → **S3**.

### (8) Harness: `pm clear` đầu bảng `--otp` và chọn nhóm theo tên

- **`pm clear` + `mo_link` + `cho_bundle || true` trước vòng flow** (chỉ khi `OTP=1`): bảng luôn xuất phát từ app sạch. Mọi flow 22–41 đều mở bằng `extendedWaitUntil "Rủ Đi thôi!|Khám phá|…"` rồi `runFlow when visible` nên đường đi không đổi; bảng đo **thêm** một điều kiện (khởi động sạch) chứ không đo thứ khác. Cũng là thuốc đúng cho bệnh đã ghi trong bộ nhớ («phiên sót làm bảng mặc định đỏ ở flow 00; pm clear trước»). Bẫy còn lại: `cho_bundle || true` nuốt lỗi nạp bundle — Metro chậm thì flow 22 đỏ với câu của flow, không phải «bundle không nạp» (S5).
- **Chọn nhóm theo tên «Hoi QA»** trong `kiem_may_chu_sau_27` và `chuan_bi_anh_nhom_cho_38`: lọc `kind != "pair"` thu về đúng đối tượng (phép kiểm nói về *nhóm* của D), `(hoi or act)[0]` giữ đường cũ khi tên không có. Lý do thật, comment nói đúng: các `kiem_may_chu_sau_*` chạy ở **cuối** bảng, sau 41, khi cặp mới nhắn đã đứng đầu danh sách. Không đo thứ khác.
- **Không tái lập được ở đây** (không có emulator/adb): bảng 18 flow, ba ảnh, `kiem_may_chu_sau_41`, hai canary curl. Đối chiếu tĩnh: mọi nhãn flow 41 bấm/chờ đều có trong mã (`Bạn bè, lời mời đã nhận và đã gửi` Profile.tsx · `Nhắn tin cho .*` Friends.tsx · `Ô soạn tin`/`Gửi tin nhắn` GroupChatLive.tsx · `Xem hồ sơ` pill mới · `.*thành viên · xem và mời` chỉ ở nhánh nhóm · `Đã là bạn` Friends.tsx · `Tin nhắn` tab `app/(tabs)/_layout.tsx` · `Nhắn riêng` Conversations.tsx · `Tường cá nhân`/`Nhắn tin` HoSoNguoiScreen.tsx · «Bạn: …» dựng ở `Conversations.tsx:43`). `kiem_may_chu_sau_41` suy người lái từ `da_chay 37`; nếu 37 đã chạy mà app lại ở màn chào lúc 41 (flow tự đăng nhập D), phép kiểm hỏi nhầm C và **đỏ** vì môi trường — không có đường xanh giả. `tests/test_maestro_flows_dev_client.py` đã thêm 41 vào danh sách flow phải dùng `${OTP_PHONE…}`/`${OTP_CODE}` và không chứa dãy số dài.

## Đối chiếu lời khai thân PR ↔ đo lại (cây sạch `/tmp/review-pr-575` tại `bfa3389`)

| Lời khai | Đo lại | Khớp |
|---|---|---|
| pytest gốc **3225 passed, 680 skipped**, 5433 subtests | `3224 passed, 681 skipped, 2 warnings, 5433 subtests passed in 482.44s`; ca lệch là `tests/test_phone_path.py:398 SKIPPED — apps/mobile/node_modules chưa cài`; sau `npm ci`: `41 passed` → tổng khớp 3225/680 | ✓ (lệch môi trường, đã đóng — cùng dạng #574) |
| `postgres_tier.sh -q`: tests/postgres **618 passed** · qa 88 | `618 passed in 103.88s (0:01:43)` · `88 passed, 19 subtests passed in 14.05s` · `ĐẠT tests/postgres` · `ĐẠT ../../tests/qa` · rc 0 | ✓ |
| `tsc --noEmit` 0 lỗi | stdout rỗng, rc 0 | ✓ |
| `MOBILE_REQUIRE_WEB_A11Y=1 npm test` **697/697** | `# tests 697 · # pass 697 · # fail 0` (`npm ci`: `added 659 packages in 12s`; `dist-test` xoá trước). Ghi nhận: cờ `MOBILE_REQUIRE_WEB_A11Y` không có người đọc trong `npm test` (chỉ `scripts/gate.sh` và `tests/qa` đọc), nên có hay không cũng ra cùng 697 | ✓ |
| `ruff_changed.sh origin/main`: 0 finding, 0 file cần format | `ruff over 31 changed Python file(s)` → `All checks passed!` · `31 files already formatted` | ✓ |
| repo guard staged / tree / range PASS | `range <merge-base origin/main> HEAD`: `Repo guard passed commit range: 9386 file scan(s) in 6 commit(s).` | ✓ |
| migration offline ok · một head `6d2b8f4e0c53` | `ok` · `Alembic guard: mot head duy nhat (6d2b8f4e0c53).` | ✓ |
| `check_api_contract` / `check_server_routes_called` / `check_screens_reachable` / `check_actor_headers` / `check_cors_contract` rc=0 · «Không có route mới nào bị bỏ rơi» · 41/41 màn | «Client và máy chủ khớp hợp đồng.» · `96 route, 84 có người gọi, 5 miễn, 7 đang nợ, 0 không ai gọi và chưa ghi nhận` + «Không có route mới nào bị bỏ rơi.» · `41/41 màn có đường render từ cửa vào · 0 pin` · `ĐẠT — 229 lời gọi đều gửi X-Actor-ID.` · «Mọi header và method client gửi đều qua được preflight.» — tất cả rc 0 | ✓ |
| Mọi từ chối DM là một 404 cùng byte; tự nhắn 422 | probe 9 biến thể: `DISTINCT_BODIES 1`, len=85; 422 | ✓ |
| Đua trên `pair_key` giải bằng index rồi đọc lại; «chỉ mô phỏng tuần tự» | đo hai session thật: `CONFLICT PAIR_EXISTS`, `b_wait=1.01`, đọc lại đúng id | ✓ (mạnh hơn lời khai) |
| Sáu cửa roster → 409 sau bước quyền; người lạ 403 | đo đủ sáu trên Postgres qua HTTP; role với thành viên cặp là 403 (không ai là admin của cặp) | ✓ (kèm S7 về câu chữ) |
| `display_name` lưu rỗng; tên người kia dẫn xuất; thiếu → «Thành viên» | DB `''`; 3 đường đọc dùng `display_name_for`; `_finance_movements` còn thô (`context_name=''`) | ✓ (kèm S2) |
| `chonNhomMacDinh` bỏ qua `kind = pair`; hồ sơ không đếm cặp | đột biến M8/M5a/M5b đều đỏ đúng ca | ✓ |
| S7 #574: `populate_existing=True` cho hai `FOR UPDATE` | mã đúng, đo `HELD_* CONFLICT`; **không có ca** trong PR (M4 chỉ probe đỏ) | ½ (S3) |
| `e2e_slice.sh` 10/10 · bảng emulator 18 flow · ảnh · canary 41 | **không chạy** (không có emulator; e2e không nằm trong danh sách cổng được giao) | không kiểm được |

## Suggestion (không chặn)

- **S1 — hai cửa roster chưa có ca âm, và chưa có ca «người lạ trên cửa cặp».** Thêm vào `tests/postgres/test_direct_message_postgres.py` (hoặc fake, nhưng fake không có `get_membership`/`create_outing` nên tầng Postgres là chỗ tự nhiên): (a) kèo tạo trong cặp → `POST /outings/{id}/invites` `friend` và `link` → 409 `not_a_group`, người lạ → 403; (b) trồng một membership INVITED lên cặp → `POST /memberships/{id}/accept` của invitee → 409, của người khác → 403, hàng vẫn INVITED; (c) người lạ trên invite/leave/rename → 403. Ba probe của tôi (`test_zz_probe_dm_doors.py`, in trong phần bằng chứng) là mẫu sẵn; M1/M2/M6 là đột biến để chứng minh ca mới đỏ.
- **S2 — `_finance_movements.context_name`** đi qua `display_name_for` (hoặc trả `null` cho cặp) để §2.5.6 đúng ở mọi đường đọc; thêm một assert `context_name` vào ca movement khi có màn nào render nó.
- **S3 — ca hai session cho `populate_existing`** (nửa còn lại của S7 #574) và, nhân tiện, ca đua hai session cho `pair_key`: probe `HELD_*` và `RACE_*` ở đây là mẫu; chúng cần `postgres_engine` chứ không cần fixture mới.
- **S4 — `open_direct_message` quét toàn bộ `list_person_context_summaries(actor)`** (mỗi hàng một `count_unread_messages`) để lấy đúng một hàng. Đủ dùng hôm nay; sẽ đắt khi danh sách dài. Một `summary_for(context_id, person_id)` là đủ.
- **S5 — `cho_bundle || true`** trong khối `pm clear` mới của `mobile_native.sh`: nên `hong "bundle không nạp sau pm clear"` thay vì im lặng, để lần đỏ kế tiếp nói đúng tên bệnh.
- **S6 — `Friends.tsx::nhanTin`** đổi cả màn sang `pha: "hong"` khi mở DM lỗi (mất danh sách bạn đang xem); `HoSoNguoiScreen` làm đúng hơn (`loiChat` tại chỗ dưới nút).
- **S7 — câu chữ thân PR/ADR:** cửa đổi vai trò với *thành viên cặp* là 403 (không ai là admin của cặp), không phải 409 — nên viết rõ để người đọc `main` không đợi 409 ở đó.

## Bằng chứng đã xem — dòng kết quả nguyên văn

Cổng (worktree sạch, `PYTHONDONTWRITEBYTECODE=1`, `-p no:cacheprovider`, chạy tuần tự để cây không bị đổi giữa cổng):

```
$ python3 -m pytest services/api/tests tests -q                        # gốc worktree
3224 passed, 681 skipped, 2 warnings, 5433 subtests passed in 482.44s (0:08:02)
SKIPPED [1] tests/test_phone_path.py:398: apps/mobile/node_modules chưa cài
$ python3 -m pytest tests/test_phone_path.py -q                         # sau npm ci
41 passed in 0.05s
$ scripts/postgres_tier.sh -q
618 passed in 103.88s (0:01:43)
88 passed, 19 subtests passed in 14.05s
  ĐẠT   tests/postgres
  ĐẠT   ../../tests/qa
$ cd apps/mobile && npm ci --no-audit --no-fund                         → added 659 packages in 12s
$ rm -rf dist-test && npx --no-install tsc --noEmit                     → (rỗng) rc=0
$ MOBILE_REQUIRE_WEB_A11Y=1 npm test                                    → # tests 697 · # pass 697 · # fail 0
$ python3 scripts/check_server_routes_called.py
Máy chủ khai 96 route. 84 có người gọi, 5 miễn (người gọi ở nơi khác), 7 đang nợ (đã ghi nhận là chưa ai gọi), 0 không ai gọi và chưa ghi nhận.
Không có route mới nào bị bỏ rơi.
$ python3 scripts/repo_guard.py range "$(git merge-base origin/main HEAD)" HEAD
Repo guard passed commit range: 9386 file scan(s) in 6 commit(s).
$ scripts/ruff_changed.sh "$(git merge-base origin/main HEAD)"
ruff 0.9.2 (bản ghim) … ruff over 31 changed Python file(s) … All checks passed! … 31 files already formatted
$ (migration offline)                                                   → ok
$ python3 scripts/check_alembic_heads.py                                → Alembic guard: mot head duy nhat (6d2b8f4e0c53).
$ python3 scripts/check_api_contract.py     → Client và máy chủ khớp hợp đồng.            rc=0
$ python3 scripts/check_screens_reachable.py → 41/41 màn có đường render từ cửa vào · 0 pin rc=0
$ python3 scripts/check_actor_headers.py    → ĐẠT — 229 lời gọi đều gửi X-Actor-ID.       rc=0
$ python3 scripts/check_cors_contract.py    → Mọi header và method client gửi đều qua được preflight. rc=0
```

Probe của reviewer (bản sao `services/api` + container Postgres riêng; ba file `tests/*/test_zz_probe_dm_*.py` chỉ tồn tại trên bản sao):

```
test_zz_probe_dm_404.py (fake)             1 passed
  PROBE pending_out/pending_in/groupmate_no_request/stranger/nobody/declined/other_way_pending/other_way_pending_in/stranger_to_me
        → mỗi ca 404 len=85, cùng thân; DISTINCT_STATUS {404} DISTINCT_BODIES 1; FRIEND 201 SELF 422
test_zz_probe_dm_race.py (Postgres 16)     3 passed in 4.57s
  RACE_REPO    b=('CONFLICT','PAIR_EXISTS', <id A>) b_wait=1.01 rows=1 members=2 ACTIVE stored_names=['']
  RACE_SERVICE a=(…, True, 'Bình') b=('OK', <id A>, False, 'An') b_wait=1.02 rows=1
  HELD_REACT ('CONFLICT','MESSAGE_DELETED') HELD_DELETE ('CONFLICT','MESSAGE_ALREADY_DELETED')
test_zz_probe_dm_doors.py (Postgres 16)    3 passed in 3.17s
  OUTING_IN_PAIR 201 · INVITE_FRIEND 409 not_a_group · INVITE_LINK 409 not_a_group · INVITE_STRANGER 403
  ACCEPT_STRANGER 403 is_invitee · ACCEPT_INVITEE 409 not_a_group
  STRANGER_DOOR invite/leave/rename 403 is_group_member · role 403 is_group_admin
  MEMBER_DOOR invite/leave/rename 409 not_a_group · role 403 is_group_admin
test_zz_probe_dm_finance.py (Postgres 16)  1 passed
  PAIR_FINANCE_MOVEMENT repo: context_name='' counterparty_name='An' occasion='Lẩu nấm' · PAIR_IN_LIST [('pair','An')]
```

Đột biến trên bản sao (neo phải xuất hiện đúng số lần khai mới áp; khôi phục từ cây sạch sau mỗi đột biến; `diff -rq` cuối bảng: `no diff` ngoài ba file probe). Nền xanh trước khi đột biến: fake `8 passed` (7 của PR + 1 probe) · Postgres `21 passed` (7 DM của PR + 8 reply/delete của L1 + 6 probe). Client: worktree đột biến riêng tại `bfa3389`, nền `# pass 6 # fail 0`, `git status` sạch sau bảng.

| Đột biến | Ca của PR đỏ | Chỉ probe đỏ | Đọc |
|---|---|---|---|
| M1 bỏ `_require_group_kind` ở `accept_context_membership` | — (fake 8 xanh, pg 20 xanh) | `test_probe_accept_door_on_a_pair` | **lỗ test** → S1 |
| M2 bỏ `_require_group_kind` ở `create_outing_invite` | — | `test_probe_outing_invite_door_on_a_pair` | **lỗ test** → S1 |
| M3 404 chung → 403 | fake `…same_404_with_the_same_body`, pg `…outsiders_get_the_one_404` | + probe 404 | gác |
| M4 bỏ `populate_existing` ở hai `select(Message)…with_for_update()` (neo đúng 2 lần) | — (reply/delete 8 xanh, DM 7 xanh) | `test_probe_held_orm_object_sees_the_other_sessions_delete` | **lỗ test** → S3 |
| M5a `profile_counts` SQL đếm cả cặp | pg `…does_not_count_as_a_group_on_the_profile` | — | gác |
| M5b `profile_counts` fake đếm cả cặp | fake `…is_not_a_group_on_the_profile_counts` | — | gác |
| M6 `leave_context`: kind TRƯỚC quyền (người lạ nhận 409) | — | `test_probe_stranger_gets_403_on_every_closed_door` | **lỗ test** → S1 |
| M11 `is_friend = True` | fake same-404, pg same-404 | + probe 404 | gác |
| M12 chỉ ghi một membership | pg `…two_active_memberships_in_one_savepoint`, `…share_one_pair_over_http…` | + 2 probe đua | gác |
| M13 `display_name_for` trả tên lưu | fake 4 ca, pg `…share_one_pair_over_http…` | + probe đua service | gác |
| M14 bỏ 422 tự nhắn | fake `…oneself_is_a_422…` | + probe 404 | gác |
| M8 client: `chonNhomMacDinh` bỏ lọc pair | `chonNhomMacDinh không bao giờ chọn một cặp…` | — | gác |
| M9 client: tên khuyết in `counterpart.id` | `…thiếu tên thì là một từ, không phải id` | — | gác |
| M10 client: `ghepVaoDanhSach` không thay hàng cũ | `ghepVaoDanhSach đặt cặp mới lên đầu…` | — | gác |

**Không chạy:** `scripts/e2e_slice.sh`, bảng Maestro / adb / emulator (không có máy ảo trong context này; phần đó chỉ có lời khai và ảnh ngoài Git của tác giả).
