# Mục lục `docs/`

Cây này có hai nửa và **nhầm lẫn giữa chúng là lỗi hay gặp nhất**: nửa trên là
tài liệu **đang sống** — đọc nó trước khi đổi hành vi; nửa dưới (`archive/`) là
**nhật ký đóng băng**, ghi lại một lần chạy hoặc một phán quyết đã xong, giữ để
truy nguyên chứ không phải để làm theo.

## Đang sống — đọc trước khi đổi hành vi

| Thư mục | Nội dung | Khi nào đọc |
|---|---|---|
| `decisions/` | **ADR-0001 … ADR-0030** + `proposals/` | **Trước mọi thay đổi hành vi.** Đây là trục của toàn bộ tài liệu; mọi thứ khác dẫn về đây. Đổi ba luật tiền, ranh giới tầng, hay phạm vi sản phẩm đều phải mở ADR **trước**, không sửa mã trước. |
| `architecture/` | Layout + quyền sở hữu, cưỡng chế luật 1 (số nguyên đồng) | Trước khi đổi ranh giới giữa các tầng hoặc giữa hai service |
| `migration/` | 132 route card cho đợt chuyển lõi sang Go (ADR-0029) | Trước khi đụng một route. **Máy đọc thư mục này**: `scripts/check_route_ownership.py` đỏ cổng nếu một hàng Go-owned mất file bằng chứng |
| `team/` | `charter.md` (quy trình), `hang-doi.md` (việc còn nợ), `backlog.md`, `de-xuat-agy.md` | Khi cần biết cái gì còn mở, ai quyết cái gì |
| `security/` | `repo-guard.md` (chính `scripts/repo_guard.py` in ra khi chặn), prompt-injection địa điểm | Khi repo guard chặn commit, hoặc khi đụng dữ liệu từ nguồn ngoài |
| `testing/` | `postgres-repository.md` — tầng test PostgreSQL thật | Khi đổi persistence. Fake repository **không** thay được tầng này |
| `runbooks/` | `pii-git-history.md` | Khi dữ liệu thật lỡ vào Git |
| `superpowers/specs/` | Spec thiết kế tính năng | Khi làm tiếp tính năng đã có spec |
| `assets/` | 11 ảnh/sơ đồ `README.md` dựng | Khi sửa README |
| `CHAY-DEMO.md` | Chạy app để tự bấm thử | Khi cần xem sản phẩm chạy |

## Đóng băng tại chỗ — không sửa, không xoá, không di chuyển

| | |
|---|---|
| `protocol/v1/` | Giao thức nghiên cứu v1. `protocol_version` là **ảnh chụp bất biến**: cần đổi thì ADR cho phép tạo `v2`, không sửa `v1`. Cùng luật với `phase0/` ở gốc repo. |

## `archive/` — nhật ký đóng băng

`archive/claude/<YYYY-MM-DD>/` và `archive/codex/<YYYY-MM-DD>/` là nhật ký ghi
theo ngày của hai lane engineer: review doc, phán quyết QA, báo cáo bàn giao, bó
bằng chứng. Mỗi file viết **một lần rồi thôi**. `archive/qa/`, `archive/qa2/` và
`archive/testing/` là phán quyết QA và hậu kiểm của cùng thời kỳ.

**Đọc chúng để truy nguyên một quyết định, đừng đọc chúng như tài liệu hiện
hành** — phần lớn mô tả một cây mã đã đổi từ lâu. Khi một bài ở đây mâu thuẫn
với `decisions/` hoặc `architecture/`, bài ở đây sai.

**Ảnh chụp màn đã được gỡ khỏi cây** (382 file, 191 MB) ở đợt dọn repo
2026-09-21; bài viết, dump `hierarchy.xml` và mọi file đo dạng text vẫn ở đây.
Lấy lại một tấm ảnh từ lịch sử Git:

```bash
git log --oneline --all -- docs/archive/claude/<ngày>/<chủ đề>/
git show <sha>:docs/claude/<ngày>/<chủ đề>/anh/<tên>.png > /tmp/xem.png
```

(Đường dẫn trong `git show` là đường dẫn **cũ**, vì ở commit đó thư mục chưa
chuyển vào `archive/`.)

## Thứ KHÔNG nằm trong `docs/`

| Ở đâu | Cái gì |
|---|---|
| `README.md` (gốc) | Sản phẩm là gì, chạy thế nào, cây thư mục |
| `CLAUDE.md`, `AGENTS.md` (gốc) | Luật cho agent: ba luật tiền, ranh giới tầng, lệnh hay dùng |
| `DESIGN.md` (gốc) | Hệ thiết kế — ở gốc theo quy ước của plugin Impeccable, và 52 file trong cây trích dẫn nó |
| `PRODUCT.md` (gốc) | Một trang về sản phẩm |
| `services/core/ownership/routes.json` | Route nào do Go sở hữu — **nguồn sự thật**, không phải cây thư mục trong `architecture/` |
