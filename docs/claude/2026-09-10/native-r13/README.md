# Bằng chứng native r13 — F45 nghĩa hình trên máy

Máy: `emulator-5554`, Android 15, 1080×2400 @420dpi, dev client 1.0.0, theme sáng, cỡ chữ 1.0, dữ liệu demo,
không có stack API. Bảng `.maestro-bs-r13` chạy hai lượt (lượt 2 sau khi bỏ vệt khói «Kẹt xe» và thêm flow 77):
**XANH** — flow 00 · 71 · 73 · 77 qua, NEO 2b cắn, canary đỏ đúng thiết kế. Ảnh ở đây là của lượt 2.

| Ảnh | Nội dung |
|---|---|
| `r13-71-tam-sticker-tren.png` · `-duoi.png` | Bàn thử ui-lab: tám sticker ở 120 và 64dp qua component `Sticker` thật — hàng dưới có «Kẹt xe» (đuôi xe buýt, chống cằm) và «Trả tiền nè» (ba tờ xoè) |
| `r13-71-khay-tam-o.png` | Khay sticker thật (`KhaySticker`), tám ô 64dp |
| `r13-71-bubble-ca-phe.png` | Bubble sticker trong chat (tổng hợp) |
| `r13-73-canh-ab.png` · `r13-73-canh-moi.png` | Cảnh có/không Nếp qua component `Canh` thật; `canh-moi` có `chua-co-tin-nhan` (bong bóng mọc từ miệng), `chua-co-loi-moi`, `chua-co-ky-niem`, `bo-loc-che-het` |
| `r13-77-loi-canh-moi.png` | **Màn lỗi thật** «Đi đâu?» không máy chủ: cảnh `chua-doc-duoc` mới — không vòng coral, coral ở vết rách, mảnh rách trượt; câu F43 giữ nguyên |

Không có path nào làm `PathParser` ném lúc mount (app lên ở mọi flow) — rủi ro thật khi đụng tầng art.

Chưa đo: theme tối trên máy (bảng art `nghia-hinh/*-toi.png` phủ); cỡ chữ ≠ 1.0 (hình không phụ thuộc cỡ chữ);
iOS; máy thật; kiểm chứng người dùng không nhãn (team).
