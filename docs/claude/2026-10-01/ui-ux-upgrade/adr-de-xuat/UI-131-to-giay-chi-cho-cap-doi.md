# Đề xuất ADR: «tờ giấy chỉ cho cặp đôi» là luật của máy chủ hay chỉ của app (QA UI-131)

- Trạng thái: **đã quyết 2026-10-04 → phương án B**, thành `docs/decisions/ADR-0053-to-giay-la-so-cua-moi-cap.md`. Viết 01/10/2026 trong đợt nâng cấp UI/UX từ audit PR #663.
- Người quyết: chủ sản phẩm.

## Vấn đề

QA đo được: `POST /contexts/{id}/papers/draft` trả 201 cho cặp **chưa** bật «Một đôi». App chỉ cho cặp đôi thấy công cụ
«Tờ giấy» (ADR-0046 §8.4: khay của đám bạn có «Tờ hẹn» mà không có «Tờ giấy»; hàng ghim «Tờ giấy» chỉ khi cả hai bật
«Một đôi»), nhưng máy chủ không gác điều đó.

## Vì sao không sửa thẳng ở đợt này

Hai luật đang nói khác nhau, và đây là quyết định sản phẩm chứ không phải lỗi thực thi:

- ADR-0027 §4 cố ý cho **tờ tạm trước khi lập sổ** (lời rủ trước khi có sổ). ADR-0038 §2.1 đã cân nhắc «từ chối tờ tạm
  (`cycle_not_active` khi chưa lập sổ)» và **bác** phương án đó vì «xoá một tính năng ADR-0027 cố ý giữ».
- ADR-0046 §8.4 (chủ sản phẩm 28/09) chỉ đặt «Tờ giấy» ở lớp cặp đôi trong **giao diện chat**.

Chặn ở máy chủ là xoá tờ tạm cho mọi cặp chưa «Một đôi», tức đảo một quyết định ADR-0038 đã ghi.

## Phương án

| Phương án | Hệ quả |
|---|---|
| A. Máy chủ gác: phác tờ chỉ khi `can_bat_doi` | Khớp ADR-0046 §8.4; xoá tờ tạm của ADR-0027 §4 cho cặp chưa «Một đôi»; sửa Go + Python oracle + goldens + parity; app cũ đang phác cho cặp bạn sẽ nhận 4xx |
| B. Ghi rõ luật là của app, máy chủ giữ tờ tạm | Không đổi mã; ADR mới nói «Tờ giấy» là bề mặt của cặp đôi, máy chủ vẫn nhận tờ tạm cho mọi cặp như ADR-0027 §4 |
| C. Máy chủ gác riêng lối «Rủ … tới đây» (UI-130, UI-085) | Lối này đang phác tờ cho cặp bạn rồi bỏ quán; sửa ở app là đủ (B8), không cần đổi luật |

## Khuyến nghị

B cho máy chủ, C ở app (đã nằm trong B8 của đợt này). Nếu chủ sản phẩm chọn A thì cần một ADR thay ADR-0038 §2.1.
