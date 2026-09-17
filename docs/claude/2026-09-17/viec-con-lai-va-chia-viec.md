# Việc còn lại, chia thành gói giao được

**Ngày:** 2026-09-17 · **Bổ sung cho:** `docs/claude/2026-09-16/ban-giao-chien-dich-go.md`
**Đọc cùng:** `docs/claude/2026-09-17/review-phan-con-lai-go.md` (verdict `REQUEST_CHANGES` cho nhánh `go/p0-w-con-lai`)

Mỗi gói dưới đây độc lập, giao được cho một agent, và có tiêu chí nghiệm thu đo được. Thứ tự là thứ tự phụ thuộc,
không phải thứ tự quan trọng.

---

## T1 — Thu ba worktree đang dở của tôi *(làm trước, công này sẽ mất nếu bỏ)*

Ba agent chết giữa chừng vì hết hạn mức, **việc đã viết ra tệp nhưng chưa commit**:

| Worktree | Base | Tệp chưa commit |
|---|---|---|
| `~/wt-go0-valhalla-stub` | `75fdeff3` | 6 |
| `~/wt-go0-w9-domain` | `75fdeff3` | 20 |
| `~/wt-go0-w9-repo` | `75fdeff3` | 11 |

Với **từng** cây: `git status` trước, drift-check từng tệp so với HEAD (tệp nào HEAD đã đụng thì `git apply --3way`,
**không chép đè**), dựng cây kiểm sạch tại HEAD, chạy cổng rẻ, chạy oracle/tier **có `-v`**, tự nghĩ **hai đột biến**
và chứng minh chúng đỏ, rồi commit kèm số đo trong message. Công thức đầy đủ ở §5 bàn giao 16/09.

Ghi chú từ lúc chúng chết, có thể tiết kiệm thời gian cho người tiếp: agent W9 repository báo đã tìm ra nguyên nhân
một chỗ sai của chính nó — *«identity map của SQLAlchemy là weak, và repository này bỏ mọi entity khi chuyển sang
dataclass, nên `session.get` luôn đọc lại; helper route của tôi mô hình sai thành map có chặn»*. Agent W9 domain
đang truy một đột biến làm tròn hai lần còn sống sót.

**Nghiệm thu:** ba commit trên nhánh chiến dịch, mỗi commit dựng lại được trong cây sạch, mỗi commit có số đo trong
message. Worktree chỉ được xoá **sau khi** đối chiếu tệp của agent khớp HEAD.

---

## T2 — Gỡ blocker B2: bằng chứng vi phân cho 11 package domain WAI *(gói lớn nhất)*

10/11 package mới có **0 tệp test**: `album`, `catalog`, `companion`, `conversation`, `faces`, `messageedit`,
`promptsafety`, `reel`, `stickers`, `suggestion` (chỉ `chatintent` có test). Đây là tầng grounding/chấm điểm/phân
loại ý định — chỗ một bản port lệch mà cổng wire không thấy.

Làm theo đúng khuôn đã có: `scripts/render_domain_w*_goldens.py` của W7/W8/W10 là mẫu. Render ca **từ chính hàm
Python trong ảnh ghim** `mobile-parity-api:7bf58e3d`; trước khi tin câu trả lời nào, kiểm `/srv/app` trong ảnh giống
từng byte với `services/api/app` của cây. Ca biên + một shard mẫu thì commit; fuzz lớn để sau tag `oracle`.

**Nghiệm thu:** mỗi package có số ca và số sai khác in ra bằng `-v`; 112 golden của các sóng cũ dựng lại vẫn giống
từng byte; ít nhất hai đột biến đỏ, mỗi đột biến chứng minh được là rơi vào dòng code **và** quan sát được.

---

## T3 — Gỡ blocker B3: oracle Postgres cho tầng repository mới

`messages.go`, `destinations.go`, `place_photos.go`, `outing_memories.go`, `SetMembershipRole` chưa chạm Postgres
thật. Khuôn có sẵn: `internal/repo/*_repo_oracle_postgres_test.go` + `scripts/render_*_repo_oracle.py`.

Nhớ tính chất đã biết của oracle này: nó ghi **văn bản SQL**, không ghi tham số. Giá trị sai chỉ bị bắt khi nó đổi
kết quả hoặc đổi dump bảng — chỗ nào giá trị quan trọng mà không đổi trạng thái cuối thì phải ghim cách khác.

**Nghiệm thu:** `scripts/go_postgres_tier.sh --image mobile-parity-api:7bf58e3d -- -v ./...` báo số PASS, **0 FAIL,
0 SKIP**, sentinel `TestPostgresTierReachesDatabase` PASS **trong cùng lượt**, oracle mới có số ca/câu lệnh/sai khác,
và mọi oracle sóng cũ vẫn 0 sai khác.

---

## T4 — Stub định tuyến tất định *(gỡ nốt nửa routed của preview)*

Chi tiết đầy đủ ở §4.1 bàn giao 16/09. Một phần đã có trong `~/wt-go0-valhalla-stub` (T1). Sau khi stub lên, nửa
routed của `preview` mới có parity; trước đó nó **vẫn là chưa chứng minh**, kể cả khi HTTP W7 đã PORTED.

---

## T5 — Gỡ blocker B1 rồi chạy cổng một lượt cho tất cả

Quyết nhãn trước: hoặc 36 hàng mới chờ `gate.sh parity` rồi mới lật `PORTED`, hoặc để nhãn riêng và ghi lý do vào
`evidence`. Sau T1–T4, Go phục vụ 144+ route, lúc đó **một** lượt cổng đầy đủ mới đáng tiền:

`flock -o <scratchpad>/stack.lock scripts/gate.sh parity` trên **SHA sạch**, kèm canary, probe, và hai đột biến của
người gộp. Dự kiến ~1,8 giờ sau khi làn kho ảnh đã được làm rẻ (`75fdeff3`). Làn limiter đã ra INFRA ba lần khi máy
bận — nếu còn agent sống, chạy riêng làn limiter và pha prod trên máy rảnh và ghi rõ trong commit.

---

## T6 — Trước bất kỳ `LIVE-GO` nào

Không route ảnh nào được cắt khi Go còn trả 415 ở chỗ Python trả 201: Pillow trong ảnh ghim mở được 40 định dạng
(`avif`, `jpg_2000`, `libtiff`, `webp` đều bật), Go hỗ trợ 6. Hoặc Go giải được, hoặc chuyển đúng request đó cho
Python kèm bộ đếm. Và harness phải sinh AVIF + TIFF ở một làn riêng — corpus hiện sinh đúng những định dạng Go đã
hỗ trợ nên không thể đỏ ở 34 định dạng còn lại.

---

## Ai làm được gì

- **T1, T5** đòi phán đoán về bằng chứng và phải do người gộp làm (đọc diff, tự nghĩ đột biến, quyết nhãn).
- **T2, T3** là việc khuôn mẫu, khối lượng lớn, có mẫu sẵn trong repo — giao được cho agent, rồi người gộp chạy lại
  trong cây sạch.
- **T4** vừa phải, có sẵn một nửa.
- **T6** là quyết định sản phẩm + một mẩu harness.

Luật không đổi cho mọi gói: digest của agent không phải bằng chứng; người gộp chạy lại trong cây sạch tại đúng SHA;
phán quyết đỏ/sống lấy từ **số ca đỏ**, không từ mã thoát; `go test` không `-v` im lặng cả với ca SKIP; `-race` cần
cgo; đừng vứt stderr trong lệnh kiểm.
