# Nút bình chọn có chữ trong tờ AI (review Codex 11/09 · A4)

Codex A4 (P2): hành động phụ cạnh «Xem lịch trình» chỉ là icon cột biểu đồ (`stats-chart-outline`), người chưa được
giải thích đọc thành «xem thống kê»; nhãn a11y «Mở bình chọn» đúng nhưng không giúp mắt đang nhìn icon.

## Sửa (`apps/mobile/src/rudi/screens/Group.tsx`, tờ AI fixture)

- `IconButton` → `RudiButton variant="outline" tone="ai" label="Bình chọn"`, không icon: hai nút chữ, nút chính `soft`
  kéo hàng, nút phụ `outline` vừa chữ — thứ bậc giữ bằng bề rộng và độ đậm nền, không bằng glyph.
- **Vòng sửa theo finish reviewer (context mới):** bản đầu để viền nút phụ màu `ai`; reviewer đo ở 2.0 hai nút xếp dọc
  thì viền tím (5.8:1) sắc hơn nền tô nhạt của nút chính (1.17:1) nên mắt rơi vào «Bình chọn» trước. Sửa: viền
  `lineStrong` (4.5:1 sáng / 4.1:1 tối), tông chỉ ở chữ. Reviewer chấm **resolved**, phán quyết `ship` cho phạm vi này.
- Ở chữ lớn (`chuLon(fontScale)`, ngưỡng 1.28) hai nút xếp dọc, mỗi nút hết cột — không nhãn nào phải xuống dòng.
- `accessibilityLabel="Mở bình chọn"` giữ: hai flow đang bấm nó (`.maestro/06-vote-an-ket-qua.yaml`,
  `.maestro-bs-r3/65-r3-binh-chon.yaml`) chạy lại rc 0 sau khi đổi.
- Thẻ AI live (`chat/TheAi.tsx`) không có nút → không đổi.
- DESIGN.md: đoạn «Nhịp của tờ AI trong luồng chat» mô tả nút mới; luật mới «Hành động phụ trong tờ mang chữ».

## Bằng chứng (`anh/`, máy ảo Android 15, 1080×2400 @420dpi, fixture)

| ảnh | cấu hình | thấy gì |
|---|---|---|
| `r17-93-chat-ai-binh-chon-fs1.0-sang.png` · `-toi.png` | 1.0 sáng/tối | «Xem lịch trình» soft kéo hàng, «Bình chọn» outline vừa chữ bên phải; «Vì sao phác vậy» và chân ký giữ nhịp |
| `r17-93-chat-ai-binh-chon-fs2.0-sang.png` · `-toi.png` | 2.0 sáng/tối | hai nút xếp dọc hết cột, nhãn một dòng; tờ vẫn đọc là một quyết định |
| `r17-93-man-binh-chon-fs1.0-sang.png` · `-fs2.0-toi.png` | sau khi bấm | màn «Bình chọn» mở («BBQ tối thứ Bảy ở đâu?») — nút dẫn tới đúng nơi |

Flow `apps/mobile/.maestro-bs-r17/93-binh-chon-co-chu.yaml`: assert «Bình chọn» hiện, bấm theo a11y «Mở bình chọn»,
chờ câu hỏi bình chọn. Bốn cấu hình xanh (`scratchpad/chup.sh`). `imp detect --json Group.tsx` → 0 finding;
`tsc --noEmit` exit 0. XML dump ở `anh/*.xml` là của màn bình chọn (chụp sau flow), không phải của tờ AI.

## Chưa chứng minh
Người ngoài phân biệt «Bình chọn» với «xem thống kê» (Codex yêu cầu thử người thật — để team); thẻ AI live không có
nút nên không có gì đo; TalkBack chưa nghe.
