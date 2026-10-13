---
version: 1
slug: "apps-mobile-src-rudi-community-communityscreen-tsx"
primary_target: "apps/mobile/src/rudi/community/CommunityScreen.tsx"
related_targets: []
---

# Cộng đồng — cập nhật có chỗ riêng, giữ nơi đang đọc

Đề xuất refinement theo `/impeccable-pipeline` → `layout`, trong thế giới Cộng đồng hiện hữu; không thay bản sắc hay tạo raster. Contract này được ghi trước candidate code. Root cần ghi phần được chọn vào surface brief qua wrapper trước khi sửa UI thật.

## Direction contract

**THESIS:** Câu chuyện dẫn mắt; cập nhật chờ người đọc trong một hàng gọn ở đáy cột, không nằm đè ô tìm hoặc bài viết.

**OWN-WORLD:** Giữ giấy/mực/cam, `RudiButton` ghost, `colors.line`, gutter 16 và nhịp 4 hiện có. Không thêm khung trang trí, token, icon mới hoặc asset.

**STORY:** Người đọc tiếp tục ở đúng nơi; “Xem cập nhật” báo có điều mới và gọi đúng refresh hiện tại khi được chạm. Tên truy cập giữ “Bảng tin có cập nhật”.

**FIRST VIEWPORT:** Ô tìm → chế độ → câu chuyện vẫn ở vị trí cũ. Hàng cập nhật đứng sau FlatList, trước điều hướng, nằm trong readingColumn tối đa 560; không đè nội dung và không chèn vào đầu danh sách. Khi hiện, chỉ giảm vùng đọc ở đáy. Khi không có cập nhật, không giữ khoảng trống.

**FORM:** Mở rộng thế giới đã chốt, không roll seed mới. Hàng tự cao theo native metrics, vùng chạm tối thiểu của kit, label ngắn có thể wrap; fade vẫn theo Reduce Motion. Không đổi state/ref/key/data, generation guard, stream hay HTTP.

**FINISH:** unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Quality contract và giới hạn

- Native phone 320×640 và tablet override 768×1024, sáng/tối, font 1/2: cả ô tìm lẫn hàng cập nhật phải đầy đủ, tách vùng chạm, không che card/nav/IME.
- Khi nhận stream lúc đang cuộn: không tự load, không gọi scrollToOffset, không thay danh sách hay input. Top của nội dung đang đọc không đổi; viewport giảm từ đáy là lựa chọn có chủ đích. Chỉ chạm refresh và nhận response hợp lệ mới cuộn lên theo logic cũ.
- Kiểm riêng initial sync, reconnect/feed.changed/post.changed, refresh lỗi/thành công, route topic không có `dau`, focus return và reduced motion. Không dùng ảnh static để khẳng định motion hoặc host test để khẳng định Yoga.
- Đây là proposal private chưa chạy/chụp AFTER, chưa chứng minh chức năng hoặc thẩm mỹ của candidate. Không có verdict toàn app.

Questions skipped: user đã cho phép tự sửa/audit; root là executor duy nhất và sẽ review candidate trước áp dụng.
