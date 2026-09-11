# Nghĩa hình vòng 2 — F45 còn mở + R4 (tái audit 10/09)

Codex 10/09: sau vòng 1, giấy rách ở lỗi «sửa tốt»; nhưng «Kẹt xe» ở khay thật giống điện thoại/kiosk hơn xe buýt;
«Trả tiền nè» vẫn có thể đọc thành vé/giấy (cần một dấu nhận biết tiền ở nhỏ, trong ranh giới ADR-0021: không
xu/tick/ký hiệu tiền/QR/ngân hàng); cảnh gọi lời: đuôi bong bóng bị Nếp che, đọc như bảng có chấm; `bo-loc-che-het`:
lưới thoáng + vòng hở coral gợi đang tải — Codex đề nghị **bỏ vòng hở**.

Cách làm như vòng 1: ứng viên dựng bằng chính primitives (`scratchpad/ung-vien-vong2.mjs`), render cạnh bản hiện tại
ở 64/120 sáng/tối, **nhìn** rồi chọn, ghi vào nguồn, rồi **reviewer context mới đọc mù** bảng không nhãn trước khi đọc
packet — vòng đọc mù đầu đánh trượt bong bóng và bộ lọc, nửa đạt xe buýt; sửa hình học theo ba gợi ý rồi đọc mù lại (v3).

## Chọn gì, vì sao (đọc hình học, không đọc chữ)

| Hình | Trước (10/09) | Ứng viên đã xem | Chọn |
|---|---|---|---|
| Kẹt xe | khối 27×59 không bánh, dải kính, một vạch cản → kiosk | A xe buýt có bánh (27 rộng) · B xe hơi thấp-rộng · C = A + vạch đèn hậu coral · D người nhỏ hơn (0.72, x0 −7) để xe rộng 34 · E = D + vạch coral → reviewer đọc mù: «xe tải/xe đẩy», thân còn cao hơn rộng · **v3**: người 0.66/x0 −9 (khung xe máy co theo `k`), thân xe **rộng hơn cao** (≈42×38), **hai ô kính**, hai bánh, một vạch đèn hậu coral | **v3**: bánh là dấu phương tiện mạnh nhất ở 64; hai ô kính nói «xe buýt» (một ô nói «màn hình»); **một** vạch đèn hậu nói «đuôi xe» (hai chấm đã bị loại vì thành mặt). Ngoại lệ ngân sách coral có ghi lý do |
| Trả tiền nè | ba tờ xoè, chỉ góc gấp coral | A bầu dục trên cả ba tờ · B tờ trên: bầu dục + khung đôi | **B**: khung đôi + ô chân dung là hai dấu mọi tờ bạc có mà vé không có; không ký hiệu tiền; khung đôi chỉ ở bản 120 (nét mảnh ở 64 là nhiễu), bầu dục tô `bong` ở cả hai cỡ |
| chua-co-tin-nhan | đuôi là đỉnh của cùng đa giác, nằm dưới tờ giấy (x 16…52) → che trọn; chấm coral (91,35) | D1 đuôi cắt từ góc dưới-trái, không chấm · D2 chấm sát gốc · D3 chấm giữa → reviewer đọc mù D1: «bảng có góc vạt» · **v3**: thân bong bóng đặt **cao hơn miệng**, đuôi là tam giác riêng **mọc từ cạnh đáy** (gốc x 64–78), mũi (53,58) ở độ cao miệng | **v3**: đuôi có cạnh bong bóng ở hai bên nên đọc là đuôi, không là góc; dừng ở mép tờ giấy theo quy ước tranh; bong bóng **trống** đúng nghĩa «chưa có lời»; chấm làm nó thành bảng |
| bo-loc-che-het | panel giấy + 6 nét `bong` hở + `vongHo` coral | A panel tô `bong` + lưới 4×4 nét giấy + ô hở viền coral · B lưới mực dày → reviewer đọc mù A: «bảng tính/lịch có ô được chọn» · **v3**: **một tờ có bốn dòng chữ** vẽ trước, **tấm che** `bong` lệch xuống-phải để lộ dải trên-trái của tờ (đầu các dòng chữ), lưới chỉ 2×2, ô hở cho lộ một mẩu dòng chữ, khung coral bốn cạnh | **v3**: «che» cần có **cái bị che**; tấm che lệch để thấy tờ dưới; ô hở lộ chữ = còn một chỗ nhìn qua. Coral = cái khe đáng nhìn, bốn nét thẳng, không cung |

Bảng: `sau-v2-sang.png` / `sau-v2-toi.png` (có nhãn), `khong-nhan-v2-sang.png` / `khong-nhan-v2-toi.png` (**không nhãn** — cho
người chưa đọc brief: 4 sticker 64 + 4 cảnh, trong đó 2 sticker và 2 cảnh **không đổi** làm đối chứng), `tam-sticker-v2-sang.png`
(cả tám ở 120 và 64).

## Cổng (cơ chế, không phải nghĩa)

- `art-duong.test.mjs`: cấm cung tròn coral ở **cả** `chua-doc-duoc` và `bo-loc-che-het`; mọi đỉnh của bong bóng ≥ x 52
  (không phần nào nằm dưới tờ giấy Nếp) và đỉnh trái nhất ở độ cao miệng.
- `rudi-chat-sticker.test.mjs`: `ket-xe` ≥ 4 hình tròn tô (hai bánh Nếp + hai bánh xe trước); `tra-tien-ne` có ô bầu dục
  tô màu giấy và không dùng teal.
- 21/21 test art + sticker qua; `test_sticker_vocabulary_matches_client.py` qua (bảng `HINH` không đổi).

## Trên máy (dev client, ui-lab)

`native/r13-71-khay-tam-o-fs1.0-{sang,toi}.png`: khay sticker 64 thật — «Kẹt xe» đọc ra đuôi xe buýt hai ô kính trên bánh,
«Trả tiền nè» quạt tờ có ô bầu dục; `native/r13-73-canh-ab-fs{1.0,2.0}-{sang,toi}.png`: hai cột cảnh A/B (có/không Nếp).
Flow 71 ở chữ 2.0 đỏ ở bước cuộn tới «Mở khay sticker» (giới hạn của flow bàn thử ở 2.0, có từ r13, không phải của hình)
— khay 2.0 chưa có ảnh lượt này.

## Đọc mù của reviewer context mới (một người đọc, đã thấy từ vựng packet — không thay cho người ngoài)

Vòng 1 (v2): xe → «xe tải/xe đẩy/kiosk» (nửa đạt); tiền → «tờ bạc/tiền mặt» (đạt); bong bóng → «bảng có góc vạt» (trượt);
bộ lọc → «bảng tính/lịch có ô được chọn» (trượt). Vòng 2 (v3): xe → «kẹt sau đuôi xe buýt/van nhỏ, hai ô kính, đèn hậu»;
bong bóng → «bong bóng thoại trống, đuôi trỏ về miệng — đang nói, chưa nói gì»; bộ lọc → «tấm che trên tờ giấy có chữ, một
ô nhìn qua»; tiền giữ. Phán quyết: ship trong phạm vi bốn fix đã chấm.

## Chưa chứng minh

- **Người chưa đọc brief** đọc bảng không nhãn — vẫn là việc của team (Codex 10/09 nói rõ họ đã đọc brief nên không tự
  xưng thử mù; tôi cũng vậy). Reviewer context mới của Impeccable đọc mù trước khi đọc packet — ghi ở trả lời.
- Khay 64 trên máy thật; iOS.
