# ADR-0033 — Nếp ra khỏi trạng thái rỗng: một trợ lý nổi trên mép màn

**Trạng thái:** Chấp nhận · **Quyết định bởi:** leader, 2026-09-22 · **Hiện thực:** Claude
**Liên quan:** ADR-0017 (ảnh không đi mà thiếu xuất xứ), ADR-0024 (thông báo trong app và push),
ADR-0025 (reel dựng từ ảnh của nhóm), ADR-0027 (sổ hai người), ADR-0031 (chat Go E2EE),
ADR-0032 (một vai fullstack)

## 1. Bối cảnh

`DESIGN.md` §1284 viết:

> **Luật Nếp Đứng Xa Tiền.** Nếp chỉ xuất hiện ở trạng thái rỗng và cửa vào; **không bao giờ**
> cạnh số tiền, lỗi, hay xung đột (báo cáo 07/09 §6.4).

và nói thêm, ở đúng mục ấy, rằng muốn đưa Nếp ra ngoài trạng thái rỗng và cửa vào thì **phải mở
một quyết định mới, đừng suy từ mục đó**. Đây là quyết định ấy.

Lý do cần: AI của app hiện chỉ sống trong khung chat của một nhóm (`/plan`, `@Rủ Đi`,
companion-turn). Người dùng đứng ở màn khác thì không có ai để hỏi, và cách duy nhất để hỏi là
đi ngược về một nhóm rồi gõ một lệnh. Thứ thiếu không phải thêm một câu lệnh chat, mà là **một
chỗ để hỏi, ở mọi nơi**.

Cùng lúc, hai ràng buộc không được nới:

- `CLAUDE.md`: *«AI chỉ nhận nội dung được gọi/chia sẻ rõ ràng, không tự đọc chat/gu/lịch sử.»*
  Chat v2 E2EE nên máy chủ cũng không có khoá để mà đọc.
- Chính Luật Nếp Đứng Xa Tiền: lý do nó tồn tại (báo cáo 07/09 §6.4) là một nhân vật vui vẻ đứng
  cạnh một con số nợ thì đọc như chế nhạo.

## 2. Quyết định

1. **Nếp được nổi trên mọi màn, dưới dạng một dock bám mép phải.** Không phải một màn, không phải
   một tab, không phải một mục trong thanh điều hướng. Nó là một lớp nổi trên route đang mở.

2. **Nếp TỰ LUI ở màn tiền, lỗi và xung đột.** Cụ thể: `finance`, `settlements/*`, `batches/*`,
   `smart-split/*`. Ở những màn đó Nếp bị ép về trạng thái `an` và **cấm** hé ra một dòng, kể cả
   khi có việc. Việc vẫn được ghi nhận, chỉ là không nói ra. Danh sách màn nằm ở đúng một hằng số
   `MAN_NEP_LUI` trong `src/rudi/nep/phieu.ts`.

   Đây là cách giữ nguyên tinh thần luật cũ chứ không phải huỷ nó: thứ luật cũ cấm là Nếp **đứng
   cạnh tiền**, và điều đó vẫn bị cấm.

3. **Thứ còn lại ở mép trên màn tiền là một mép giấy 6dp, không mặt, không nhân vật.** Nó là cửa
   quay lại chỗ Nếp, không phải Nếp đang có mặt.

4. **Rời màn tiền thì Nếp trả về đúng lựa chọn của người dùng**, không về mặc định. Ai đã vuốt
   Nếp đi thì vẫn đi.

5. **Nếp không tự đọc gì cả.** Mỗi màn tự khai một `PhieuNguCanh` với danh sách khoá **đóng**:
   tên route, tiêu đề, nhịp kèo, loại sổ, vài **số đếm**, và hai ba câu hỏi gợi sẵn. Không tên
   người, không nội dung chat, và **không tiền dưới bất kỳ khoá nào** — đó là Luật Nếp Đứng Xa
   Tiền áp lên dữ liệu chứ không chỉ lên pixel.

6. **Nếp là của cá nhân, không phải của nhóm.** Endpoint `/me/nep/*`. Việc này giữ nguyên
   `ban-tinh.ts`, nơi sổ loại `hoi` cho Nếp **zero** việc: Nếp vẫn im trong nhóm.

7. **Báo tin khi đang giấu bằng mép giấy, không bằng chấm đỏ.** `DESIGN.md` cấm Nếp làm chrome,
   và một badge đỏ là chrome. Có việc thì mép dày thêm một lớp và ấm lên một nấc, **đúng một
   nhịp**, không lặp.

## 3. Hệ quả

- `tests/nep-phieu-kin.test.mjs` quét nguồn: một khoá ngoài danh sách làm **đỏ CI tại chỗ gọi**.
  Bộ lọc lúc chạy giữ máy chủ sạch nhưng không nói cho người viết biết họ vừa viết sai.
- Máy trạng thái nằm ở `src/rudi/nep/trang-thai.ts`, thuần, chạy dưới node trần. Hai luật nặng
  nhất của ADR này (mục 2 và mục 4) là hai bài kiểm ở đó.
- Bước Maestro `12-nep-dock.yaml` đi qua ẩn/hiện/mở/Back trên máy thật.
- Baseline nghệ thuật không đổi: cổng `art-duong` vẫn băm 680 lớp và vẫn xanh. ADR này không đụng
  một toạ độ nào của Nếp.
- Ảnh Nếp vẽ ra hiện qua `MediaSlot` kèm câu xuất xứ «AI vẽ, có dấu SynthID», theo ADR-0017 §2.5.

## 4. Cái này KHÔNG cho phép

- Không cho Nếp tự đọc hội thoại, gu, hay lịch sử. Muốn thế thì mở quyết định khác.
- Không cho Nếp nói trong nhóm. `ban-tinh.ts` vẫn là chỗ duy nhất quyết việc đó.
- Không cho Nếp thay `Stamp` hay `GuGlyph` trong vai trò thông tin, và không cho Nếp vào thanh
  điều hướng hay biểu tượng app.
- Không cho Nếp chạm ledger. Không tool nào của Nếp ghi vào tiền.
