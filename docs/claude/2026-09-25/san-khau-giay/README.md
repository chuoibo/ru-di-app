# UI v3 «Sân khấu giấy»: nhật ký giao việc

- Ngày: 2026-09-24 → 2026-09-25
- Nhánh: `claude/practical-faraday-mswgmv`
- Quyết định gốc:
  - ADR-0037 (Lead ký 24/09);
  - kế hoạch `docs/architecture/04-ui-v3-san-khau-giay.md`;
  - QC gốc `docs/claude/2026-09-24/qc-frontend-chieu-sau-va-cau-chuyen.md` (commit `9036001`).
- protocol_version: không áp dụng (lát frontend, không đụng giao thức v1 hay trang khách).
- Verdict: chưa có reviewer thật. Chưa mở PR; Lead chọn giao một lần.

## Các commit, theo lát

| Lát | Commit | Nội dung |
|---|---|---|
| S0.1 | `ded49ba` | ADR-0037, token sân khấu, mực người, cổng chữ trên giấy tối |
| S0.2 | `be484ec` | Skia 2.6.2, nạp lười, SVG dự phòng |
| S0.3 | `caac00d` | máy sân khấu pop-up |
| S0.4 | `c28a7bb` | Nếp con rối giấy, 9 tiết mục, 8 khoảnh khắc |
| S0.5 | `2385b29` | bộ primitive giấy |
| S1 | `992ae1d` | màn tiền; sửa B9 |
| S2 | `927ee1d` | sổ hai người; sửa B1 phía client |
| S3 | `e6751d4` | cửa tạo mới; M1, M5 |
| S4 | `dbca268` | Khám phá, Lên plan, 15 thành phố; sửa B5 |
| S5 | `763149b` | chat, người, bảng Nếp; sửa B2, B3, B8; mực người qua AvatarNguoi |
| S6 | `733b4e8` | kỷ niệm, thành tích, hộ chiếu; sửa B4 |
| S7 | `ed3d49e` | vào cửa, B7, quét 768/1280/tối/giảm chuyển động, DESIGN v3 |
| S8 | `b359c6a` | Sở thích, Đăng bài, Story, Chi tiết bài, thẻ bình chọn |
| S9 | commit sau S8 | hành trình bản đồ, cài đặt nhóm, hồ sơ người khác, vai quản trị; trả hết nợ chữ trên giấy; đọc mù |

Hai commit merge `origin/main`: `1f61fcd`, `f67c286`. Mỗi commit lát có số đo cổng, đột biến và
số chạy cây sạch trong commit message.

## Bằng chứng đã xem

- Ảnh chụp và khung quay trên stack sống:
  - Postgres 16, API Python :58098, Go core :58099, seed `seed-rudi-world`;
  - web dev qua Chrome headless (SwiftShader), đăng nhập OTP thật;
  - 360/412/768/1280dp, sáng và tối, giảm chuyển động.
- Ảnh nằm ngoài repo (không commit).
- Khoảnh khắc đã quay khung: M1, M3, M4, M5, M6 (hai theme), M7, M8. Cú mở bìa Welcome cũng đã
  quay.
- Lỗi QC B1–B9: B1–B5, B7–B9 đã sửa phía client, mỗi lỗi có test ghim; B6 cần khoá AI để đo lại.
- Lỗi tự tìm thấy trong lúc làm, đã sửa:
  - AvatarNguoi nuốt `personId`;
  - Nếp M5 bị cắt đôi ở mép hộp;
  - dấu «SỔ ĐÃ MỞ» gãy ba dòng;
  - bìa mở biến mất;
  - ô tên kèo nổi lên ở màn rộng;
  - giấy nhớ trên nền `paper` tối.
- Đọc mù (S9, 26/09): một subagent context mới, không đọc mã, xem 11 ảnh không nhãn (Sở thích,
  Đăng bài ×3, Story, Chi tiết bài, thẻ bình chọn, hành trình, cài đặt nhóm, hồ sơ người khác,
  thành viên) và trả lời «màn này làm gì, bấm vào đâu, có gì khó hiểu».
  - Đoán đúng việc chính ở 11/11 màn (tự tin cao 10, vừa 1: hành trình).
  - Một ảnh là khung cũ trước bản sửa nắp phong bì của S8; lời chê «nắp đè tiêu đề» ứng với khung
    đó, khung hiện tại không còn.
  - Sửa ngay trong S9: ô màu đang chọn không nhìn ra (viền cùng màu với ô), câu «Bạn bè» bị cắt
    «…» ngay chỗ chọn người đọc (D8), hồ sơ nói năm tham gia hai lần.
  - Phần còn lại ghi ở `docs/team/hang-doi.md`, mục 2026-09-25, điểm 5.

## Cái gì còn mở

1. **Native (Lead):**
   - dựng lại dev client có Skia;
   - chạy lại các flow Maestro liệt kê ở `docs/team/hang-doi.md` (mục 2026-09-25);
   - đo `dumpsys gfxinfo`;
   - smoke crash Skia (dev client cũ phải rơi về SVG).
2. B1 phía máy chủ (bản phác mồ côi khi sổ chưa lập): cần Lead chốt luật, qua cổng parity.
3. Cài đặt vẫn giữ trơn có chủ ý (chỉ đổi mép `Sheet`). Mọi màn khác đã làm lại: S8 (Sở thích,
   Đăng bài, Story, Chi tiết bài, thẻ bình chọn) và S9 (hành trình bản đồ, cài đặt nhóm, hồ sơ
   người khác, vai quản trị).
4. DESIGN.md: mục v3 là hợp đồng hiện hành; phần v2 bên dưới chưa viết lại toàn bộ.
5. Cỡ chữ lớn (fontScale 1.3/2.0) chỉ đo được trên máy thật; web headless luôn là 1.0.
6. Chữ «bút chì» của tờ sổ đôi đi theo trạng thái tờ, không theo từng dòng. Muốn theo dòng thì
   máy chủ phải đưa `nguon` ra dây.
