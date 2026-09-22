# `codex/p0-w28-chat-go-e2ee`: đóng tại chỗ, **không merge** — và vì sao

**Ngày 22-09-2026.** Câu hỏi hay gặp khi nhìn `git status` ở `/home/lakiet/mobile`:
nhánh này chưa merge, cây thì bẩn, có phải đang chờ Codex tự merge không?

**Không.** Lane Codex đã đóng theo [ADR-0030]; không còn ai để «tự merge». Nhánh
này là **nhánh làm việc của phiên bàn giao**, và nội dung của nó đã vào `main`
theo đường khác.

## Nội dung đã vào main, lịch sử thì không

Các PR #624 và #628–#633 được dựng **từ `main`**, chép file cuối theo phạm vi,
chứ không cherry-pick lịch sử nhánh (các commit cũ trộn UI với backend). Nên:

| | |
|---|---|
| commit của nhánh chưa có trong `main` | 15 |
| commit `main` đi trước nhánh | **41** |

File lõi đều đã có trên `main`: `chatv2/store.go`, `chatv2http/hub.go`,
`chatassist/handler.go`, `chatlegacychange/handler.go`,
`packages/chat-crypto/src/lib.rs`. Chiều ngược lại, `main` có thứ nhánh **không**
có — ví dụ cả bộ E2E Go `services/core/e2e/chat/`, viết sau lúc bàn giao.

## 1022 file chỉ có trên nhánh, và vì sao KHÔNG nên kéo vào

| Nhóm | Số file | Là gì |
|---|---|---|
| `docs/claude` + `docs/codex` | 846 | nhật ký phiên cũ |
| `apps/mobile/.maestro-bs-*` | ~120 | mini-bảng QA dùng một lần |
| `scripts/dot_bien_*`, `scripts/qc/*` | 15 | script đột biến một lần |
| `.resume-codex-*.md` | 4 | file khôi phục phiên của Codex |
| **source thật** | **5** | `useBanNhap.ts`, `viewability.ts`, test của nó, `chat-ui-contract.md`, vài `tools/chat-live-*.mjs` |

Năm file source thật **đều đã nằm trong PR #630**. Merge cả nhánh chỉ để lấy
chúng thì đổi lại là gần một nghìn file rác phiên vào `main`, và repo guard phải
duyệt lại toàn bộ đống đó.

**Quyết định của leader (22-09-2026): đóng nhánh tại chỗ, không merge.** Nhánh
giữ nguyên làm dấu vết lịch sử. Ai cần tra thì đọc nhánh, đừng merge nó.

## Cây `/home/lakiet/mobile` bẩn là **cố ý**

17 file tracked đang sửa, **432 file chưa theo dõi**. Suốt phiên không chạy
`reset`/`clean`/đổi nhánh ở cây ấy: đống untracked có thể là việc chưa commit
của lane khác, và xoá là mất thật. **Quyết định của leader: giữ nguyên.**

Hệ quả phải chấp nhận: `git status` ở cây ấy sẽ luôn bẩn. Đó không phải dấu hiệu
hỏng. Muốn cây sạch để làm việc thì dùng worktree riêng, đừng dọn cây ấy.
