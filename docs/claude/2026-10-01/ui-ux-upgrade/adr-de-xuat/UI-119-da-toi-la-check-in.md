# Đề xuất ADR: «Tôi đã tới» và «check-in» là một việc hay hai (QA UI-119)

- Trạng thái: **đề xuất**, chưa triển khai. Viết 01/10/2026 trong đợt nâng cấp UI/UX từ audit PR #663.
- Người quyết: chủ sản phẩm.

## Vấn đề

Hai việc cùng mang chữ «tới»/«check-in», ghi vào hai nơi, và không màn nào nói chúng khác nhau.

- «Tôi đã tới» ở một chặng của kèo gọi `POST /outing-stops/{id}/checkins` và ghi vào `outing_stop_checkins`.
- «Check-in» của tường là một kỷ niệm: `POST /contexts/{id}/checkins`.
- Album chỉ đếm kỷ niệm (`internal/domain/album/album.go:115–153`).

QA đo ngay sau một lần «Tôi đã tới», ba màn nói ba điều khác nhau:

- Màn kèo ghi «1 đã tới».
- Album ghi «0 chỗ đã tới · 0 check-in».
- Tường mời «check-in ở chỗ đang ngồi».

## Phương án

| Phương án | Hệ quả |
|---|---|
| A. Album đếm cả lượt tới chặng: «chỗ đã tới» = chặng có ít nhất một «Tôi đã tới» ∪ địa điểm của kỷ niệm check-in. «check-in» vẫn là số kỷ niệm | Đổi luật đếm của album ở máy chủ (Go, và Python oracle nếu route còn `python: live`), cùng goldens và parity |
| B. Sau «Tôi đã tới», app mời «Thêm khoảnh khắc ở đây» (một chạm, mang sẵn quán của chặng); tường và album nhận dấu qua kỷ niệm đó | Chỉ đổi app. Người không đăng gì thì album vẫn ghi 0 chỗ đã tới |
| C. Giữ tách, đổi chữ: kèo nói «đã tới», tường nói «khoảnh khắc ở đây». Album ghi rõ «chỗ đã tới (theo kèo)» và «khoảnh khắc check-in» | Chỉ đổi chữ. Hai số vẫn khác nhau nhưng không còn mâu thuẫn |

## Khuyến nghị

**B + C ở app** (đã nằm trong B3 của đợt này), **A là quyết định của chủ sản phẩm**.

A làm album khớp điều người dùng vừa làm. Nó cũng đổi nghĩa một con số đã có, nên cần ADR chứ không sửa lặng lẽ.

## Tiêu chí xong (theo QA)

Sau «Tôi đã tới» ở một chặng: album của kèo ghi ít nhất «1 chỗ đã tới» (A), hoặc tường có dấu của lần tới đó (B, khi người
dùng nhận lời mời).
