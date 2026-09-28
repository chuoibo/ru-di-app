# ADR-0050 — Cá nhân hoá chạy nền cho Nếp: học từ lời của chính người dùng, tìm kiếm và tương tác

- Ngày: 2026-09-28.
- Trạng thái: **Chấp nhận — chủ sản phẩm chốt 2026-09-28** trong phiên làm việc (trả lời có ghi lại). Không phải
  chữ ký Lead.
- Thay một phần ADR-0041 §2.6 và ADR-0048 §5 (mục 5). **Không sửa bản lịch sử** của ADR nào.
- Dựa trên: ADR-0041 (Nếp, trí nhớ), ADR-0049 (nhà cung cấp AI v3: agy-proxy, jev qua OpenRouter, Milvus).
- Không đổi: ba luật tiền; chat v2 E2EE (server không giải mã, không đọc); bot nhóm không chạm `nep_*`
  (ADR-0041 §2.9); đồng ý mặc định **tắt**.

## 1. Bối cảnh

- Chủ sản phẩm muốn Nếp hiểu từng người dần theo thời gian: thói quen, sở thích, qua chat, tìm kiếm và tương tác
  trên mạng xã hội trong app, bằng một tác vụ nền không ảnh hưởng đường xử lý request.
- Phần lớn khung đã có (ADR-0041): cờ đồng ý `nep_cai_dat.nho`, sự kiện có kiểu `nep_su_kien`, biên nhận sự thật
  `nep_su_that` (loại khép kín, nguồn, hiệu lực `tu_luc`/`den_luc`), bia `nep_quen`, saga xoá `nep_xoa`, chỗ đưa
  hồ sơ vào lượt `HoSoNep`. Job gộp hằng đêm và job trích xuất đã được vạch ở design 05 §6 nhưng chưa làm.
- ADR-0041 §2.6 và ADR-0048 §5 hiện cấm học từ chat. Tìm kiếm cố ý không lưu theo người (`rag_query_log` không có
  `person_id`).

## 2. Quyết định

1. **Đồng ý**: dùng lại `nep_cai_dat.nho` (mặc định tắt). Bản công bố nâng lên **v2**, nói rõ: Nếp đọc *tin nhắn
   bạn tự viết* trong chat thường (không mã hoá), lịch sử tìm kiếm và tương tác của bạn với bài viết, để hiểu sở
   thích của bạn. Người đã bật ở v1 phải đồng ý lại trước khi job đọc chat/tìm kiếm của họ.
2. **Nguồn** (chỉ khi `nho = true`, chỉ của chính người đó):
   - tin nhắn legacy do **chính người đó** viết (`messages.author_id`); lời người khác không bao giờ vào prompt;
     phòng v2 E2EE không đọc được và không đọc;
   - lịch sử tìm kiếm của họ: bảng mới `nep_tim_kiem`, chỉ ghi khi `nho = true`;
   - tương tác mạng xã hội: xem/đọc hết/thích/bình luận bài của người khác — chủ đề, địa điểm, loại bài của bài
     được tương tác và lời bình luận do chính họ viết; không lấy danh tính tác giả bài;
   - sự kiện app `nep_su_kien` như cũ.
3. **Nhịp**: mỗi đêm (02:00 giờ Việt Nam), chỉ cho người có hoạt động mới kể từ mốc (`nep_hoc_moc`). Mỗi người mỗi
   đêm tối đa **1 lời gọi agy** (trích sự thật ứng viên, `json_schema`) và **1 lời gọi jev** (quyết thêm / cập nhật /
   bỏ qua so với sự thật đang có), đếm qua `llm.Dem`, chặn trên bởi `MOBILE_MODEL_RPM`. Chạy trên lane `memory`,
   một việc một người, khoá tư vấn theo người.
4. **Hồ sơ thay đổi theo thời gian, không xoá theo thời gian**: sự thật có phiên bản. Khi một sự thật đổi, bản cũ
   được đóng `den_luc`, bản mới mở; lịch sử giữ nguyên. Độ tin (`do_tin`) được cập nhật theo bằng chứng mới.
   (Thay ADR-0041 §2.7 «sự thật cũ bị xoá ở lần gộp sau».)
5. **Xoá chỉ theo quyền của người dùng**: tắt «Nếp nhớ», «quên điều này», hoặc xoá tài khoản vẫn xoá thật qua saga
   `nep_xoa` như ADR-0041 §2.8 — quyền rút đồng ý và yêu cầu xoá (Luật 91/2025/QH15) không bị văn bản này bỏ.
6. **Lược đồ** (mở rộng, không tạo hệ thứ hai):
   - `nep_su_that`: `nguon` thêm `chat`, `tim_kiem`, `tuong_tac`; `loai` thêm `thich_mon`, `ngan_sach_thuong`,
     `khong_khi_thich`, `di_cung` (enum `ban_be` / `nguoi_yeu` / `gia_dinh` / `mot_minh`, không bao giờ tên người);
     thêm `noi_dung` (≤ 160 ký tự, qua `promptsafety.TextSafe`), `do_tin` (0–1), `thay_the_cho` (phiên bản trước).
   - `nep_tim_kiem`: `person_id`, câu tìm đã qua `promptsafety`, bộ lọc có kiểu, quán đã bấm, `luc`.
   - `nep_hoc_moc`: mốc đã đọc tới của từng người cho chat / tìm kiếm / tương tác / sự kiện, và lần chạy cuối.
   - Vector của sự thật nằm trong collection trí nhớ Milvus của Go (partition key `owner_id`, ADR-0049 §2.8).
7. **Ai đọc hồ sơ**: chỉ **Nếp** (tối đa 5 sự thật mỗi lượt qua `HoSoNep`, như cũ) và **xếp hạng tìm kiếm / khám
   phá của chính người đó**. Bot nhóm, bot cặp đôi, và mọi người khác không bao giờ đọc.
8. **Không bao giờ trích**: sức khoẻ (kể cả dị ứng, ăn kiêng vì bệnh), tôn giáo, chính trị, xu hướng tính dục, dân
   tộc, tiền/số dư/nợ, tên hay thông tin của người khác. Luật nằm trong prompt; jev gắn nhãn `nhay_cam` thì bỏ.

## 3. Hệ quả

- Thêm tải đêm trên pool seat agy và OpenRouter; số lời gọi tỉ lệ với số người có hoạt động mới, không với tổng
  người dùng.
- App mobile phải gửi sự kiện (`/me/nep/su-kien`) và có công tắc «Nếp nhớ» với bản công bố v2 — hiện chưa có.
- `nep_tim_kiem` là dữ liệu cá nhân mới: vào `CotNguoiGo` và saga xoá như mọi cột theo người.

## 4. Cái này KHÔNG cho phép

- Không đọc chat, tìm kiếm hay tương tác của người **chưa bật** «Nếp nhớ» v2.
- Không đưa lời của người khác (kể cả trong cùng phòng) vào prompt.
- Không cho bot nhóm hay bất kỳ ai ngoài Nếp đọc hồ sơ.
- Không mở cho người dùng thật khi chưa có hồ sơ đánh giá chuyển dữ liệu xuyên biên giới (ADR-0049 §4).
- Không ghi `person_id` hay nội dung vào log, metrics, span.

## 5. Điều khoản bị thay hoặc sửa

- ADR-0041 §2.6 (nguồn trí nhớ chỉ là hành vi trong app ≤ 30 ngày và câu `hoi` của chính người đó ≤ 48 giờ, không
  bao giờ chat): **sửa** — thêm tin nhắn legacy do chính người đó viết, tìm kiếm và tương tác mạng xã hội, sau
  đồng ý v2. Vẫn không bao giờ chat nhóm của người khác.
- ADR-0041 §2.7 (sự thật cũ bị xoá khi gộp): **sửa** — đóng phiên bản, giữ lịch sử.
- ADR-0048 §5 («Không suy ra gu từ chat»): **sửa cho Nếp** — Nếp được học sở thích từ lời của chính người đó sau
  đồng ý v2; gu cặp đôi (`chia_gu`) và `person_interests` vẫn chỉ do người dùng tự khai, job không ghi vào đó.
