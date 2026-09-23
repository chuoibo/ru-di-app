# W7 outings — 11 route, bằng chứng để rời `PORTED-UNPROVEN`

Người gộp: Claude · ADR-0029 (thang trạng thái) + ADR-0030 (cổng khi không còn review chéo)

## 1. Cổng đòi gì

ADR-0030 bỏ AGY-PASS và thay bằng: **một lượt `gate.sh --strict parity` đầy đủ, chạy lại trong worktree SẠCH tại
đúng SHA, cộng canary, cộng probe, cộng HAI đột biến do chính người gộp nghĩ ra** (không dùng lại của tác giả),
và số phải nằm trong commit message chứ không nằm trong scratchpad.

## 2. Hai đột biến của người gộp

Đặc tả và quy trình tái lập: `services/core/tools/w7-dot-bien/dot-bien.json`. Khác với các làn đột biến khác,
hai cái này chạy trên **stack sống** chứ không qua `go test -overlay`, vì thứ chúng phải làm đỏ là bộ so parity
trên dây. Đo tại `41d8a030`, trên `parity/scenarios/w7/outings/` — 11 kịch bản, 545 bước:

| đột biến | kết quả | chữ ký |
|---|---|---|
| mọi từ chối outings → 403 | **9/11 kịch bản, 75 khác biệt** | bước `owner_unknown_outing`, `holder_accepts_junk`: reference 404, candidate 403 |
| `timeline_revision` lệch một | **11/11 kịch bản, 80 khác biệt** | `timeline_revision` 0 ở reference, 1 ở candidate |
| **đối chứng, core gốc** | **11/11 EQUAL, 0 khác biệt** | — |

Bước đối chứng là bắt buộc và không được bỏ: hai lượt đỏ ở trên chỉ có nghĩa khi bản gốc xanh lại trên **đúng
tập kịch bản ấy**. Không có nó thì màu đỏ có thể chỉ là hậu quả của việc tráo nhầm binary.

Cố ý **không** chọn «bỏ `FOR UPDATE`»: kết quả của nó phụ thuộc lịch trình nên ra `RACY` chứ không ra `DIFF`,
mà `RACY` không phải bằng chứng.

## 3. Những gì KHÔNG được chứng minh

- 11 route này **vẫn `owner: python`**. Không một route nào được Go phục vụ trong cấu hình thật; cú chuyển
  (`LIVE-GO`) là bước riêng, chưa làm.
- Kịch bản W7 phủ 11 route qua 545 bước ở làn chính, cộng cross-replay, concurrency và làn limiter. Nó **không**
  phủ dữ liệu thật, tải thật, hay kế hoạch truy vấn.
- Valhalla đi qua `parity/internal/routingstub`, không phải Valhalla thật. Cái được chứng minh là **ánh xạ** của
  Go từ câu trả lời định tuyến sang phản hồi, không phải hành vi của Valhalla.
- `RACY` trong làn concurrency là bất định của **chính bên reference** (thứ tự `pair_key` sắp theo chuỗi), nên
  những kịch bản ấy lượt này không khẳng định gì về Go.

## 4. Lượt cổng — `46e7627d`, worktree sạch, `ĐẠT` (7 281 giây)

`scripts/gate.sh --strict parity`, `HEAD 46e7627d`, chạy lại trong worktree tách rời tại đúng SHA.
Kết quả: `ĐẠT 1 · HỎNG 0 · BỎ QUA 0` — «Tất cả chặng đã chạy đều ĐẠT».

Toàn lượt: **383 EQUAL · 0 DIFF · 0 SHIFT · 0 RETRY · 0 INFRA · 16 RACY**.

| pha | kịch bản | bước | khác biệt | làn DB · làn kho ảnh | tap |
|---|---|---|---|---|---|
| dev, làn chính | 339 | 10 369 | **0** | bật · bật | 9 857 trong Go · 151 route · `unserved=0` |
| dev, làn limiter | 9 | 209 | **0** | bật · bật | 195 trong Go |
| prod, làn chính | 23 | 604 | **0** | bật · bật | 601 trong Go |

- **canary dev**: `identity equal, every exercised damage caught` — 14/14 mode.
- **canary prod**: `identity equal, every exercised damage caught` — 14/14 mode.
- **probe**: `cases=22 differ=10 accepted=10 unexpected=0 stale=0`.

`unserved=0` ở cả ba pha nghĩa là không bước nào rơi ra ngoài tầm quan sát của tap.

**16 RACY** là bất định của **chính bên reference**: `pair_key` sắp hai id theo chuỗi, nên thứ tự là đồng xu và
ba lượt reference không phải lúc nào cũng trùng nhau. Nó không khẳng định gì về Go, và không làm đỏ cổng.

## 5. Ba lỗi của bộ đo phải sửa trước khi lượt này xanh được

Ghi lại vì chúng nói rõ con số trên đáng tin tới đâu — và vì suốt thời gian đó cổng **im lặng chứ không đỏ**:

1. `MOBILE_INTERNAL_TOKEN` chưa bao giờ được truyền cho stack parity (`8c13e1c3`). Từ `9a4a9bc2`, Python từ
   chối khởi động khi thiếu nó, nên harness **không dựng nổi stack nào** suốt bốn commit.
2. umask lệch giữa hai bên (ảnh 022, host 002) làm làn kho ảnh đọc **24 kịch bản khác nhau** trong khi
   `mode`, `size`, `sha256` giống hệt — chỉ quyền thư mục lệch (`8c13e1c3`).
3. Làn limiter vỡ cửa sổ 60 giây vì chi phí chụp bản đổ lớn theo cỡ database, mà làn này chạy sau cùng trên
   database đầy nhất (`86aae829` thử-lại có giới hạn, `46e7627d` dồn bản đổ về cuối kịch bản).

Cộng thêm một ngoại lệ ADR đã hết lý do tồn tại (`1ea03816`) và một lỗi trong chính bộ phân loại `SHIFT` do tôi
viết (`3d76a957`).

## Đổi 2026-09-23 — đồng ý theo cùng một lời đề nghị (QA cặp đôi)

Diff này đổi `pair_notebook.granted_purposes`/`_live` (và Go `pairnotebook.GrantedPurposes`/`live`): «cả hai đồng ý» một bậc của sổ đôi tính theo CÙNG MỘT lời đề nghị, lời đề nghị đã hoàn tất không hết hạn; `_consents_as_dicts` mang thêm `proposal_id`, `proposal_completed_at`. Route này nằm trong vùng chạm của `check_go_owned_python_touch.py` do đồ thị gọi so theo tên hàm trần (`ApiService.pair_notebook` trùng tên module `pair_notebook`); mã của route không đọc đồng ý của sổ đôi. Byte trả lời không đổi với mọi dữ liệu có một lời đề nghị mỗi bậc (mọi kịch bản parity hiện có), và từ 23/09 không còn tạo được hai lời đề nghị cùng bậc song song (`POST …/notebook/proposals` trả 409).
