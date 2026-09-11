# Review độc lập — cổng đo motion v4

Ngày: 12/09/2026. Cây review: `/home/lakiet/wt-codex-audit-r4`, nhánh `codex/khep-audit-luot-4`, base `f9f3b2d1`.

**APPROVE đối với ba tệp mã và phần README v4 đã review.** Không tìm thấy lỗi chặn trong phần siết histogram, kiểm mã thoát đọc cấu hình/PID, kiểm scale hai đầu cửa sổ và phục hồi cấu hình. Phán quyết này không duyệt toàn PR hay chất lượng motion native. README v4 đã được đọc lại: mô tả khớp mã, 31 nhánh canary, 14 đối chứng đỏ và giới hạn kiểm scale hai đầu. Chưa có lượt đo native v4 mới được reviewer xác minh tại thời điểm ký; phần dẫn tới hồ sơ chạy thực cần do tác giả hoàn tất trước bàn giao.

## Phạm vi và nhận dạng mã

Reviewer là agent kỹ sư riêng, không viết ba tệp bên dưới, không sửa product, không đổi Git và không dùng thiết bị. Chỉ kiểm diff so với base, script đầy đủ, canary, test wrapper, phần README v4 mới và README motion lịch sử. Không xét phần hình minh họa/native/đọc mù của PR.

| Tệp | SHA-256 đã kiểm |
|---|---|
| `docs/claude/2026-09-10/motion/do-motion.sh` | `dd8a66d50bc100b7f19410288a5d81971818b910b2bc3ce86124c33483a1526c` |
| `docs/claude/2026-09-10/motion/do-motion-canary.sh` | `dbc65af9507c872316dacb672dc9b8296aa445c5ab8110c7af4423ab4ec9865f` |
| `tests/test_motion_measurement_gate.py` | `c0c230925e0cb3cd248b5e9c76f11ed0c81a6988aff33984437d7b8742a82d7e` |
| `docs/claude/2026-09-10/motion/README.md` | `5e62f8775c610b2e80d73c162ae97aded0417d5a1cd4a0528c9267fdffca5641` |

## Bằng chứng tự chạy

- `python3 -m pytest tests/test_motion_measurement_gate.py -q`: **1 passed**, 13,00 giây. Wrapper thực sự chạy canary **31 nhánh**, không chạm emulator.
- `python3 -m ruff check tests/test_motion_measurement_gate.py`: **All checks passed**.
- Đối chứng hồi quy: xác minh `.do-motion-v3-before.sh` bằng đúng bytes của `git show f9f3b2d1:docs/claude/2026-09-10/motion/do-motion.sh`; SHA-256 `5b8053d4558b425479184488a50a93ccf92d796bb1c938f23ea2c954cefb1e17`. Chạy canary mới với `MOTION_RUNNER` trỏ bản đó: **exit 1, 14/31 nhánh sai**. Tám dạng histogram hỏng đều được v3 in thành bốn hàng hợp lệ; các nhánh đọc lỗi và scale lệch cũng lọt. Đây là đối chứng đỏ thực chạy, không suy từ mã tác giả.
- Harness Python riêng, fake `adb`/`maestro` riêng với trạng thái JSON: **8 ca tích hợp đạt**; không dùng fake của tác giả. Dùng đúng runner trên đĩa và dump fixture cũ chỉ chứa metric làm đầu vào tổng hợp.
- Trích riêng hàm parser từ runner, cho thêm **6 histogram hỏng**: rỗng, hậu tố rác sau số đếm, số đếm `+0`, bucket trùng giá trị số (`5ms`, `05ms`), nhãn thập phân (`5.0ms`), token dư `extra=0`: cả sáu trả cờ histogram không hợp lệ.

| Ca harness riêng | Exit | Số lần Maestro | Scale cuối |
|---|---:|---:|---|
| Đối chứng reduce hợp lệ | 0 | 5 | 0.5 / 1.5 / 2 |
| Đọc scale sau flow trả số đúng nhưng rc=23 | 1 | 2 | 0.5 / 1.5 / 2 |
| Thuong: đọc lại lỗi liên tục, kể cả lúc xác minh kết thúc | 5 | 0 | 0.5 / 1.5 / 2; không có put |
| PID trước trả số đúng nhưng rc=23 | 1 | 1 | 0.5 / 1.5 / 2 |
| PID sau trả số đúng nhưng rc=23 | 1 | 5 | 0.5 / 1.5 / 2 |
| Dump đầy đủ nhưng rc=23 | 1 | 5 | 0.5 / 1.5 / 2 |
| TERM trong flow đầu sau warm-up | 130 | 2 | 0.5 / 1.5 / 2 |
| TERM và lệnh phục hồi bị từ chối | 5 | 2 | 0 / 0 / 0 |

Các tệp scratch của reviewer ở ngoài worktree: `/tmp/review-motion-independent.py`, `/tmp/review-motion-independent.log`, `/tmp/review-motion-v3-canary.log`. Chúng là bằng chứng chạy của phiên review, không được coi là artifact bàn giao bền vững. Bộ canary trong repository và test pytest là đường chạy lại bền vững; dùng `MOTION_RUNNER` với bản runner lấy từ base để tái hiện đối chứng đỏ.

## Diễn giải và giới hạn

Parser chỉ nhận một dòng histogram, token nguyên không âm đúng dạng `Nms=M`, nhãn tăng nghiêm ngặt và tổng bằng frame. Canary có đối chứng bucket chậm hợp lệ để tránh đạt bằng cách chặn mọi histogram. Kiểm readrc ngăn trường hợp lệnh lỗi nhưng vẫn in số mong đợi. Kiểm scale trước/sau làm hàng sai khi cấu hình lệch ở biên cửa sổ; lỗi sau flow còn ngăn các flow kế tiếp qua kiểm trước. Restore đọc lại cả mã thoát lẫn giá trị; lỗi phục hồi ưu tiên exit 5 kể cả sau tín hiệu.

Kiểm hai đầu không phát hiện cấu hình đổi rồi trở về giữa flow. Tín hiệu được kiểm bằng tiến trình fake kết thúc hữu hạn; không chứng minh độ trễ hủy của Maestro thật hay khả năng phục hồi sau SIGKILL/mất thiết bị. Canary không chứng minh native mượt, motion dễ hiểu, hiệu năng release/live, hay chất lượng trên điện thoại. README v4 đã giữ giới hạn kiểm scale hai đầu; kết luận PR cần tiếp tục giữ ranh giới bằng chứng native chưa xác minh của review này.

Sau phán quyết độc lập, tác giả đã chạy thêm một spot-check native Android ở
font scale 2.0 cho hàng ghim chuyến đi; kết quả văn bản được ghi ở
`native-check.txt`. Reviewer này không ký thay cho iOS, release, máy thật,
TalkBack/VoiceOver hoặc phép thử người dùng.
