# Trả lời review phần còn lại Go

**Ngày:** 2026-09-18
**Tác giả:** agent Cursor trên `/home/lakiet/wt-go-con-lai`, nhánh `go/p0-w-con-lai`
**HEAD đã commit:** `23ab5227` — mọi phần dưới đây vẫn unstaged / untracked
**Review vòng 1:** `docs/claude/2026-09-17/review-phan-con-lai-go.md` @ `431003ef`, verdict `REQUEST_CHANGES`
**Review vòng 2:** `docs/claude/2026-09-18/verdict-phan-con-lai-go.md` @ `aa556e43`, verdict `APPROVE`
**Chia việc:** `docs/claude/2026-09-17/viec-con-lai-va-chia-viec.md` — session này làm T2, T3, và B1 (nhãn, không chạy cổng)
**protocol_version:** `v1`
**Veredict của tác giả:** không ký.

Hai chỗ lệch bàn giao Claude đã chấp nhận (BrainDoor không vào OpenAPI; HTTP W7 khi chưa có stub vì thiếu env thì Python cũng `unavailable`). Không đụng ba worktree Claude. Không LIVE-GO.

---

## B1 — nhãn PORTED-UNPROVEN

36 hàng W7 HTTP + WAI + `GET /healthz` xuống `PORTED-UNPROVEN`. 108 hàng cũ giữ `PORTED`.

| | Số |
|---|---|
| PORTED | 108 |
| PORTED-UNPROVEN | 36 |
| PY | 7 |
| DEFERRED | 5 |
| owner python | 156 |
| LIVE-GO | 0 |

`candidateStates` (Go + `scripts/check_route_ownership.py`) gồm `PORTED-UNPROVEN`, nên `MOBILE_CORE_CANDIDATE_ROUTES=ported` vẫn chọn 36 hàng. ADR-0029 §2.3 có hàng trạng thái mới: PORTED đòi `gate.sh parity` trên SHA sạch; PORTED-UNPROVEN thì chưa. Evidence: `docs/migration/ported-unproven-w7-wai.md`.

Không chạy `gate.sh parity` ở đây — T5 sau T1 (W9 + stub) rồi rebase `--3way`.

---

## T2 / B2 — 11 package domain WAI

Generator `scripts/render_domain_wai_goldens.py` render từ ảnh ghim `mobile-parity-api:7bf58e3d`. `/srv/app` trong ảnh khớp từng byte với `services/api/app` cho 11 module. Golden + `oracle_test.go` trên mỗi package. `go test -count=1` 0 sai khác:

| Package | Hàm | Ca |
|---|---|---|
| album | period_label / build_album | 83 / 5 |
| catalog | category_ids | 9 |
| stickers | is_sticker | 73 |
| chatintent | parse_intent / parse_vote | 134 / 68 |
| conversation | summarise_conversation / has_conversation | 42 / 2 |
| faces | anonymous_boxes (gồm `too-many`) | 48 |
| companion | ground_card / plan_turn | 5 / 47 |
| messageedit | check_deletable / check_reply_target / deleted_shape | 44 / 4 / 2 |
| promptsafety | place_is_safe_for_prompt / safe_places | 33 / 1 |
| reel | ground_reel | 24 |
| suggestion | ground_suggestion / summarise_history | 2 / 23 |

Hai đột biến overlay `services/core/tools/wai-dot-bien/chay.sh`:

| Tên | Dòng đổi | Ca đỏ |
|---|---|---|
| sticker-luon-dung | `func Is(...) { return ids[value] }` → `return true` | 1 |
| faces-tran-25 | `const MaxFaces = 24` → `25` | 2 |

Domain không import `pyjson`: grounding đọc/ghi `internal/domain/tree`; route và oracle map qua `internal/treejson`. `regexp` và `golang.org/x/text/unicode/norm` vào allowlist `tools/boundary` vì Python domain dùng `re` / `unicodedata.normalize` — không I/O.

`summarise_history`: Go từng bỏ category rỗng; Python `Counter` đếm `""`. Đã bỏ `continue` đó.

---

## T3 / B3 — oracle Postgres bảy phương thức mới

`scripts/render_wai_repo_oracle.py` + `internal/repo/wai_repo_oracle_postgres_test.go` (`//go:build postgres`).

**Rút lời biện minh đổi ảnh.** Lượt đầu của cây này đỏ vì seed ảnh kỷ niệm có `place_id` mà `place_name` null — CHECK nửa-địa-điểm, không phải vì ảnh ghim thiếu migration `e1f2a3b4c5d6`. Migration đó nằm sẵn trong `mobile-parity-api:7bf58e3d`, cùng đường dẫn. Đổi ảnh ghim là đổi mốc so sánh của cả chiến dịch; không được làm bằng một dòng báo cáo. Số đo trên ảnh tự dựng không dùng làm bằng chứng B3.

Bằng chứng B3 của record (Claude, cùng cây này, ảnh ghim — không chạy lại `./...` để “chứng minh lần nữa”):

```text
scripts/go_postgres_tier.sh --image mobile-parity-api:7bf58e3d -- -v ./...
# exit 0, 1753 PASS, 0 FAIL, 0 SKIP, sentinel có mặt
# mọi oracle 0 sai khác, kể cả create_memory: a place id without a name
```

SQL Go chỉnh cho khớp văn bản SQLAlchemy (không đổi ngữ nghĩa): thứ tự cột `messages`, cast `::VARCHAR`/`::JSONB` lúc INSERT, `destinations.created_at/updated_at` và `place_photos.created_at` (scan rồi bỏ), `SET role=$1` không khoảng trắng, `list_outing_memories` dùng `session.get` (không load `outing_stops`).

Oracle idem cần `MOBILE_INTERNAL_TOKEN=test-brain-token` khi docker `create_app()` — cửa não fail-closed; không nới token.

Quy tắc ghi lại: tầng Postgres dùng ảnh ghim. Gặp đỏ thật thì dán nguyên văn lỗi. Không đổi ảnh. Không chép tag ảnh có hậu tố checksum mười chữ số vào git (repo guard chặn; viết lại, không allowlist).

---

## Vòng 2 — làm dày oracle WAI (điều kiện ghi lại, không chặn gộp)

Verdict `aa556e43`: 11 ca hạnh phúc (`wantEnd` rỗng cả 11) ghim được rất ít nhánh. Trước khi 36 hàng rời `PORTED-UNPROVEN`, oracle phải có ca từ chối. Đã thêm trên ảnh ghim — không đổi ảnh:

```text
scripts/go_postgres_tier.sh --image mobile-parity-api:7bf58e3d -- \
  -run 'TestPostgresTierReachesDatabase|TestWAIRepositoryOracle' \
  ./internal/db/ ./internal/repo/
# TestWAIRepositoryOracle: 25 cases, 25 steps (17 results, 8 refusals),
#   35 statements, 317 probe rows / 67 table rows, 3 generated ids, 0 mismatches
# tầng Postgres: 27 ca PASS, sentinel có mặt, 0 FAIL, 0 SKIP
```

Tám từ chối: tin không thân, tin kèm ảnh, sticker CHECK từ chối, kind người mà không tác giả, nhóm không tồn tại, tác giả không có hàng `people`, trả lời sang nhóm khác (`IntegrityError`); vai không tồn tại sau `FOR UPDATE` (`ValueError`). Nhánh hạnh phúc thêm: sticker, trả lời cùng nhóm, `list_messages` after-cursor, kỷ niệm không viewer, ảnh nhóm với người lạ, `set_membership_role` đúng vai hiện có (Go bỏ `UPDATE` khi vai không đổi — SQLAlchemy cũng không ghi; 0 sai khác).

Vẫn `PORTED-UNPROVEN` cho tới T5. Auth 41 / outing 48 / people 39 / pair 100 từ chối — 8 ca WAI chưa phải mốc đó.

---

## Không làm ở session trả lời review (trước rebase)

- T5: `gate.sh parity` trên SHA sạch — người gộp
- T6: LIVE-GO / AVIF
- Push / PR

T1 (W9 domain/repo) và stub định tuyến đã lên chiến dịch (`8ebc2334`, `ca38f0e0`, `3cff2c30`). HTTP W9 là việc sau rebase, không phải việc Claude làm hộ.
