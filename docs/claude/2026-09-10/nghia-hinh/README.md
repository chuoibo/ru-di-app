# Nghĩa hình F45 — bảng trước, ứng viên, sau (10/09/2026)

Audit native 09/09 §2 nói ba hình còn cần nhãn để chốt nghĩa: «Kẹt xe» đọc thành «đi xe», «Trả tiền nè»
đọc thành «đưa vé», `chua-co-tin-nhan` cùng động tác với `chua-co-keo`; và vòng coral dưới tờ rách của
`chua-doc-duoc` đọc thành spinner đứng yên. Yêu cầu: giữ khung Nếp, sửa hành động/quan hệ, không thêm chi
tiết li ti, không vẽ thêm cảnh.

Cách làm: dựng ứng viên bằng chính primitives đã biên dịch (`dist-test/rudi/art/*`), render cạnh bản cũ
bằng chromium (puppeteer-core), **nhìn** rồi chọn — như lượt mày Nếp 09/09. Mọi bảng ở đây là **art sheet**
(SVG của app tô bằng token thật), không phải ảnh máy; ảnh máy nằm ở `../native-r13/`.

| File | Nội dung |
|---|---|
| `truoc-f45-sang.png` | Bản **trước** (main `aad96499`): hai cảnh cùng dáng cầm bút, vòng coral dưới tờ rách, xe + mặt mệt, hai tờ chồng góc coral |
| `ung-vien-f45-vong-1-sang.png` | Vòng 1: Kẹt xe A khối trơn / B ba thanh; Trả tiền A ba tờ xoè / B hoá đơn xé đôi; tin nhắn A bong bóng như cờ / B `moi-ly` giơ bong bóng |
| `ung-vien-f45-sang.png` | Vòng 3: Kẹt xe **A3** (đuôi xe buýt: dải kính + cản) phóng to; Trả tiền A phóng to; `chua-doc-duoc` D1; tin nhắn **C** (bong bóng mọc từ miệng) |
| `khong-nhan-sang.png` · `khong-nhan-toi.png` | **Bảng không nhãn** cho team thử với người chưa đọc brief: Kẹt xe 120/64, Trả tiền nè 120/64, `chua-co-tin-nhan`, `chua-co-keo`, `chua-doc-duoc`, `chua-co-loi-moi` |
| `tam-sticker-sang.png` · `tam-sticker-toi.png` | Cả tám sticker ở 120 và 64 sau khi đổi hai hình — kiểm tính **bộ** |
| `canh-truoc.json` | **Mốc** `hinhCanh(id)` có/không Nếp của mười cảnh trên `main aad96499`, dump từ `dist-test` trước khi tái cấu trúc bảng `NEP`; sau tái cấu trúc (chưa đổi pose) so `JSON.stringify` từng cảnh → `refactor thuần: 10/10 cảnh giống hệt`. Đổi pose mới làm nó khác đi — đúng ý |

## Quyết định

- **Kẹt xe → A3.** Khối chặn chạm bánh trước, dải kính `bong` + vạch cản để là xe chứ không phải tường;
  người chống cằm, mắt chúc xuống (`ngoi-xe` đổi tay). A2 (hai đèn hậu coral) bỏ: hai chấm trên một vạch
  **thành khuôn mặt**. B (ba thanh) bỏ: đọc ra hàng rào.
- **Trả tiền nè → A.** Ba tờ xoè, ngang hơn cao — silhouette tiền mặt duy nhất không cần ký hiệu tiền; giữ
  ranh giới ADR-0021 (không xu, tick, ₫, QR, ngân hàng). B (hoá đơn xé đôi) bỏ: **đưa hoá đơn** là đòi tiền,
  ngược câu.
- **`chua-co-tin-nhan` → C, pose mới `goi-loi`.** Tay khum cạnh miệng, tay xa mở ra, bong bóng có đuôi trỏ
  vào miệng và vẽ trước người để mặt che đầu đuôi. A/B bỏ vì đọc như cầm **bảng**.
- **`chua-doc-duoc` → D1.** Bỏ `vongHo`; coral là mép răng cưa của vết rách; mảnh rách trượt sang bên. «Đã
  rách», không «đang tải».

## Review context mới (`impeccable-finish-reviewer`) → ship

Đọc **mù** bảng không nhãn trước khi đọc README: «đứng sau đuôi xe buýt, chán» · «đây, cầm lấy» với xoè giấy
(đọc phụ «vé» còn nhưng yếu hơn hẳn bản cũ) · «gọi mà chưa ai trả lời» · «tài liệu rách, mảnh rơi ra». Không
material fix. Năm ghi chú không chặn — đã làm hai: bỏ **vệt khói** sau bánh của «Kẹt xe» (tín hiệu chuyển động
trong bức tranh nói «không đi»; reviewer bảo giữ tới khi team thử, tôi bỏ luôn vì nó là một chi tiết và
ngược nghĩa); `bo-loc-che-het` vẫn dùng `vongHo` trên lưới — cùng hình vòng hở vừa bị loại ở cảnh lỗi, và
nghĩa «bàn trống một bên» cũng không hợp lưới lọc → **để team quyết**, không đổi trong PR này. Ba ghi chú
còn lại giữ nguyên: dải kính `bong` trên `giay` ở 64dp mờ (trang trí, không mang nghĩa); đuôi bong bóng ở
bản không Nếp trỏ vào khoảng trống (quy ước bong bóng vẫn đọc được); bộ tám vẫn một nét/một ngân sách coral.

## Cổng

`tests/art-duong.test.mjs`: (1) `POSE_CANH` — mười cảnh, mỗi cảnh có Nếp một pose riêng, cảnh lỗi `null`;
đỏ trước khi đổi (`ghi-lai` hai lần). Bảng `NEP` của `canh.ts` thành dữ liệu để pose test thấy là pose màn
vẽ; refactor so byte 10/10 cảnh với mốc trước khi đổi pose nào. (2) Cảnh lỗi không cung tròn coral, coral
phải là nét gấp khúc; đỏ trước sửa. (3) `khongCatNepGap` quét pose mới trên cả dải nghiêng/đậm và trên
sticker/cảnh đã ghép; hai cỡ đọc; từ vựng sticker khớp máy chủ.

Chưa làm: kiểm chứng người dùng không nhãn (team, với hai bảng `khong-nhan-*`); iOS; máy thật.
