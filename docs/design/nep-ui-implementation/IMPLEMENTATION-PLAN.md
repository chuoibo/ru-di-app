# Triển khai frontend Nếp — 05/10/2026

Nguồn bàn giao đã đọc: prompt đầy đủ, audit A–S và coverage, pack/manifest/geometry/provenance/QC, Studio/demo source và ảnh composition/10 board, AGENTS/CLAUDE/PRODUCT/DESIGN, ADR/architecture theo lát cắt. Handoff bất biến. Trình duyệt từ chối URL file của Studio; không vòng qua chặn đó. Ảnh reference được mở bằng viewer và source HTML đã đọc; chỉ app HTTP thật được chạy để QC sản phẩm.

Worktree `/home/lakiet/project/mobile/wt-nep-ui-20261005`, branch `nep-ui-20261005`, base `811c893f`. Cây chính và audit đang chạy được giữ nguyên. Runtime riêng nằm dưới `/home/lakiet/.local/share/rudi-nep-ui`; synthetic credentials/world/token ở ngoài mọi worktree. Không push/merge/deploy/land main trong lượt này.

Checkpoint tiếp tục 06/10: execution cũ trở thành chỉ đọc trong sandbox hiện tại. Nguồn đã được sao chép chính xác vào checkout độc lập `/tmp/rudi-nep-ui-final-20261006`, nhánh `nep-ui-final-20261006`, rồi sửa regression và chốt tài liệu ở đó; [migration record](/tmp/rudi-nep-ui-final-evidence/source-migration.json). Đọc [HANDOFF](./HANDOFF.md), [TEST-RESULTS](./TEST-RESULTS.md) và [BLOCKERS](./BLOCKERS.md) để biết kết quả cuối, thay vì suy trạng thái hiện tại từ phân công ban đầu bên dưới. Local feature commit/bundle phục vụ review; không phải landing main hoặc full clean-SHA gate.

## Pipeline

1. Kiểm nguồn và baseline; matrix route/layer ở HEAD, ghi state chưa kiểm.
2. Adapt registry, palette, geometry, lifecycle, host hiện hữu. Pilot Explore, settlement, notifications, invite.
3. Mỗi lát cắt: đọc source/reference → ghi constraints → sửa → chạy action thật → capture → mở ảnh → sửa defect → regression → coverage.
4. Nhân rộng A–S và guest riêng, giữ contract money/privacy/consent/chat.
5. Web light/dark mọi renderer thật; compact/tablet/desktop/font scale/keyboard các template và màn rủi ro. Android emulator riêng; iOS native ghi blocked.
6. Tests/gates pinned và repo guard chính xác. Không benchmark/load test trong soak.
7. Impeccable 4.5 code-led extension: craft floor/native guides → detector web một lượt → fresh finish reviewer → sửa/recapture theo defect → fresh documenter → handoff.

## Công việc đang chạy

- Root: primitives/assets/shell + M/N/O, stack/captures/review/regression và documentation.
- Lát A–F: welcome/auth/gu/explore/place/create/outing/timeline/map/check-in.
- Lát G–I: inbox/archive/story entry/group/invite/friends/members/sổ hai người.
- Lát J/K/L/P/Q/R/S: media/diary/community/utility/profile/settings/aliases.

Phân công là các lát cắt source có thể chạy; root tích hợp và tự chạy/review toàn bộ. Không coi báo cáo của agent là evidence hành vi.

## Các điều giữ nguyên

Integer VND và allocator server; sổ cái là nguồn; expense edit version mới; receiver receipt khác bank proof. Không payment/bank/QR mới. Keeps là AI notes riêng theo contract hiện hành. Kho chat cũ chỉ đọc; chưa có adapter v2/device/key thì ghi blocker, không giả E2EE. Nếp canonical, một nhân vật một lúc, thành công từ response và registry cả phiên. Dark tách paper decor/card mang chữ. Control 48dp, không kicker, text/control ngoài SVG.
