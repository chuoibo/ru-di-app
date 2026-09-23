# Hệ huy hiệu hồ sơ

Bộ hình dành cho 13 thành tựu của hành trình cá nhân. Đây là hình tổng hợp bằng trình sinh ảnh tích hợp của Codex từ mô tả tổng hợp; không dùng ảnh, tên hoặc dữ liệu người tham gia. Điều kiện đạt, quyền lợi và tên huy hiệu do Go và chữ native quyết định, không nằm trong ảnh.

## Ba hướng mẫu

| Hướng | Mẫu | Nhận xét |
|---|---|---|
| Dấu mực in trên giấy sợi | [Xem mẫu](../assets/profile-badges/concepts/a-screenprint.png) | Chọn. Viền và nét mực đồng bộ với con dấu trạng thái, rõ khi thu nhỏ, hợp bìa indigo và trang giấy của Rủ Đi. |
| Tem giấy cắt | [Xem mẫu](../assets/profile-badges/concepts/b-cut-paper.png) | Chiều sâu đẹp ở cỡ lớn nhưng mép giấy mất tác dụng ở 48dp. |
| Phù hiệu thêu | [Xem mẫu](../assets/profile-badges/concepts/c-embroidery.png) | Có chất liệu riêng nhưng nặng màu hơn tường nhà và khó phân biệt nét thêu ở cỡ nhỏ. |

Ba mẫu cùng dùng các hình tượng dấu chân đầu tiên, bản đồ mở và hai lối đi gặp nhau. Hướng dấu mực được áp dụng nhất quán cho bốn huy hiệu mở đầu, tám kết tuyến và kết toàn hành trình. [Bảng xem 13 huy hiệu](../assets/profile-badges/contact-sheet.png) đặt từng ảnh ở 96px và 48px trên nền giấy sáng và nền indigo tối.

## Cách dùng và nguồn ảnh

- Ảnh gốc RGBA có alpha nằm ở `docs/assets/profile-badges/source/`; ảnh ứng dụng 384 × 384 nằm ở `apps/mobile/assets/rudi/badges/`. Ảnh ứng dụng được cắt theo alpha rồi thu bằng Lanczos vào khung trong suốt; không đổi màu hay vẽ lại.
- `BadgeArt` ở `apps/mobile/src/rudi/ui/BadgeArt.tsx` ánh xạ 13 ID ổn định. `locked`, `progress`, `unlocked`, `unknown` khác nhau bằng độ hiện và biểu tượng trạng thái; nhãn cho VoiceOver/TalkBack luôn là chữ thật.
- Không có chữ, số, hạng hoặc giá trị thưởng trong PNG. Giao diện hiển thị nhãn, tiến độ và quyền lợi bằng text native để cập nhật nội dung, hỗ trợ cỡ chữ lớn và đọc màn hình. Hình tĩnh; chuyển động đóng dấu khi server trao huy hiệu dùng quy tắc Reduce Motion của `Stamp`.
- [Prompt và quy trình sinh](../assets/profile-badges/prompts.json) ghi từng hình tượng, chất liệu, tham chiếu phong cách và phép thu kích thước. Các PNG cũng nhúng prompt hoặc nguồn dựng bằng `impeccable:prompt`; digest của từng binary được ghim trong `.repo-guard-allowlist.json`.

Mỗi huy hiệu là ảnh tổng hợp không có dữ liệu riêng tư. Bảng xem là bằng chứng kiểm tra hình; chất lượng trên thiết bị thật vẫn cần ảnh chụp native ở màn hồ sơ và thành tựu.
