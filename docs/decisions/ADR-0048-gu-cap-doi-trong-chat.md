# ADR-0048 — Gu của cặp đôi trong chat với Rủ Đi AI, và xin đồng ý lại

**Trạng thái:** 🟢 **Chấp nhận — chủ sản phẩm chốt 2026-09-28** (quyết định của chủ sản phẩm, không phải chữ ký Lead)
**Soạn:** Claude, 2026-09-28 · **Sửa bổ sung:** ADR-0034 §2.1 và §3 (cho việc dùng trong chat), dòng ngoại lệ
trong `CLAUDE.md` («AI … không tự đọc chat/gu/lịch sử»)
**Liên quan:** ADR-0027 (sổ đôi), ADR-0034 (`chia_gu`), ADR-0046 §8.4 (hai class «đám bạn»/«cặp đôi»),
ADR-0019 §2.1 (gu là của riêng từng người), `docs/claude/2026-09-28/ai-chat-hai-nguoi.md`

## 1. Bối cảnh

ADR-0046 §8.4 chia phòng chat thành hai class: **đám bạn** (mọi nhóm, mọi chat hai người thường) và
**cặp đôi** (chat hai người mà cả hai đã bật «Một đôi», `bat_doi`). Cặp đôi được mọi thứ của đám bạn
**cộng** gu đã chia. Gu đã chia là `chia_gu` của ADR-0034: mỗi người tự bật cho riêng mình trong sổ đôi.

Nhưng lời hứa người dùng đã đồng ý khi bật `chia_gu` chỉ là «Cho người ấy thấy gu của mình và cho Nếp
dùng gu mình **khi phác tờ**» (sheet `GuHaiBan`, ADR-0034 §2.1). Chat với Rủ Đi AI là một việc khác:
câu trả lời hiện lên trong luồng hai người, do một người gọi, có thể ngay khi người kia không mở app.
Dùng gu cho chat dựa trên đồng ý cũ là dùng dữ liệu cá nhân vượt phạm vi đã hứa.

Chủ sản phẩm chốt ngày 2026-09-28:

1. Cặp đôi (`Turn.Doi`, `laDoi` = cả hai `bat_doi`) có mọi tính năng của đám bạn **cộng** gu.
2. Gu của một người chỉ được dùng trong chat khi **chính người đó** đã bật `chia_gu` **theo lời mới**. Đồng
   ý `chia_gu` cũ (chỉ hứa «Nếp dùng gu khi phác tờ») **không** phủ chat — phải xin đồng ý lại.
3. Thu hồi có hiệu lực từ lượt sau; một lượt đang chạy **không được đăng** nếu gu nó đã đọc vừa bị thu hồi.
4. AI chỉ đọc gu khi **mô hình tự quyết** gọi công cụ — không heuristic từ khoá.
5. Thẻ trả lời nói rõ đã dùng gu của ai.

## 2. Khảo sát (bước 0, chỉ đọc)

- `chia_gu` được cấp bằng `POST /contexts/{id}/notebook/proposals` (mục đích `chia_gu`): máy chủ tạo một
  đề nghị, người đề nghị tự đồng ý và đề nghị hoàn tất ngay (`_propose_per_person`); hỏi lại khi đang bật
  trả về đúng đề nghị đang hiệu lực. Thu hồi bằng `DELETE /contexts/{id}/notebook/consents/chia_gu`: đặt
  `revoked_at` cho mọi dòng đồng ý `chia_gu` của người đó trong chu kỳ. Cả hai route là **`LIVE-GO`, Python
  `live`** trong `services/core/ownership/routes.json` (parity so byte với Python).
- App gọi `song.xinBac("chia_gu")` và `song.thuHoi("chia_gu")` (`to-giay/SoDoiSong.tsx`), sheet
  `screens/hai-nguoi/GuHaiBan.tsx`.
- Dòng `pair_consents` có `granted_at`; bật lại sau khi thu hồi **tạo dòng mới** với `granted_at` mới
  (không sửa dòng cũ). `pairnotebook.GrantedBy` đọc tính sống (đã cấp, chưa thu hồi, đề nghị chưa hết hạn
  hoặc đã hoàn tất).
- `GET /contexts/{id}/notebook` (LIVE-GO, Python live) không trả `granted_at` cho app.

Kết luận: làm sạch được **không cần** sửa schema bảng sổ đôi (Alembic sở hữu) và **không** đổi wire của
route LIVE nào. Không phải dừng ở bước 0.

## 3. Quyết định

### 3.1 Phiên bản đồng ý: mốc hiệu lực, không đổi schema

Một hằng số `gudoi.MocChat` = **0 giờ ngày 29/9/2026 giờ Việt Nam** (17 giờ ngày 28/9 UTC). Một đồng ý `chia_gu`
**phủ chat** khi và chỉ khi nó còn sống (`GrantedBy`) **và** `granted_at ≥ MocChat`. Đồng ý trước mốc vẫn
phủ sổ đôi như cũ (Nếp phác tờ, người kia thấy gu), chỉ không phủ chat.

- Mốc **không được sớm hơn** lúc bản app mang lời mới (§3.5) lên production. Nếu phát hành trễ, dời mốc
  về sau **trong cùng commit phát hành**; dời về sau chỉ làm hẹp phạm vi (đóng an toàn), không bao giờ mở
  rộng ngược.
- Không dùng `terms_version`: đó là hằng số máy chủ Python cũng ghi (`DIEU_KHOAN_HIEN_TAI`), đổi nó là
  đổi byte của route LIVE và đổi cho mọi mục đích, không riêng `chia_gu`.
- **Khoảng hở đã biết:** máy chủ không phân biệt được một đồng ý sau mốc bấm từ **bản app cũ** (vẫn hiện
  lời «khi phác tờ»). Giảm bằng: mốc đặt sau lúc phát hành; bản cũ vẫn hiện sheet cũ nên người dùng chủ
  động bật `chia_gu` lần đầu sau mốc trên bản cũ là trường hợp hẹp. Muốn đóng hẳn cần hoặc cổng phiên bản
  app tối thiểu, hoặc một dấu đồng ý riêng cho chat trong bảng Go sở hữu — mở ADR mới nếu cần.

### 3.2 Xin đồng ý lại

Người có đồng ý cũ bật lại bằng **luồng sẵn có**: tắt (`DELETE …/consents/chia_gu`) rồi bật
(`POST …/proposals` mục đích `chia_gu`). Dòng mới có `granted_at` sau mốc. Không thêm route, không thêm cột.
App gộp hai lời gọi thành một nút «Bật lại cho chat» (§3.5). Hai lời gọi không nguyên tử: nếu lời gọi
thứ hai hỏng, người đó ở trạng thái **tắt** (đóng an toàn) và app báo lỗi để bấm lại.

`GET /contexts/{id}/chat-capabilities` (GO-ONLY) thêm trường gốc `gu_chat`: `null` ngoài cặp đôi; trong
cặp đôi `{"cua_toi": "tat" | "bat" | "can_bat_lai", "nguoi_kia": bool}` — công tắc của chính người gọi
cho chat, và gu người kia có dùng được trong chat không. Hỏi lại mỗi lần đọc.

### 3.3 Đọc gu: công cụ mô hình tự gọi

- Công cụ `gu_hai_ban`: **không đối số**, lớp `doc` (chỉ đọc), phạm vi mới `doi`. Bảng quyền
  (`quyen.golden.json`) cấp cho bot `nhom`; bộ nạp từ chối cấp phạm vi `doi` cho Nếp hay cho `doi`.
  Công cụ chỉ **được khai** trên lượt có `Turn.Doi` (`Quyen.DuocPhepDoi`); lượt đám bạn (nhóm, chat hai
  người thường) không khai, không mời, và gọi thử bị từ chối trước khi chạy. Đây là chỗ duy nhất
  `Turn.Doi` quyết quyền.
- Nguồn dữ liệu (`aidoc.Doc.GuDoi`) chạy trong **một giao dịch READ ONLY**: chu kỳ sổ đôi còn sống của
  phòng → hai người tham gia → dòng đồng ý `bat_doi`/`chia_gu` → `gudoi.NguoiDuocDung` (cặp đôi? rồi lọc
  `chia_gu` còn sống **và** sau mốc) → `person_interests` **chỉ** của những người đó.
- Kết quả cho mô hình: mỗi người một bằng chứng `{nguoi: <nhãn danh bạ của lượt>, thich: <nhãn gu>}`, và
  khi cả hai cùng chia, một bằng chứng `{nguoi: "cả hai", cung_thich: …}`. Không ai đủ điều kiện → danh
  sách rỗng, mô tả công cụ dặn mô hình đọc là «chưa biết», không phải «không thích gì».
- Bằng chứng nguồn `gu_doi` vào sổ cái của lượt (bí danh `d1`, `d2`, …), nên verifier chấm câu nói về gu
  theo đúng bằng chứng đó như mọi nguồn khác.

### 3.4 Kiểm lại lúc đăng

Worker ghi lại những người có gu đã vào sổ cái (`Result.GuDung`). Trong **giao dịch đăng thẻ**, nếu danh
sách không rỗng: khoá chia sẻ dòng `pair_notebooks` của phòng (thu hồi khoá cập nhật dòng này), rồi hỏi
lại `gudoi.NguoiDuocDung` — phòng còn là cặp đôi, mỗi người trong danh sách còn `chia_gu` sau mốc. Sai
một người → không đăng thẻ, việc kết thúc `sharing_unavailable`. Thu hồi hoặc tắt «Một đôi» giữa lượt vì
vậy không bao giờ tới phòng. Lượt không đọc gu không bị kiểm thêm.

### 3.5 Minh bạch

- Thẻ `tra_loi` thêm `payload.doc.gu`: danh sách nhãn danh bạ của những người có gu đã được đọc (tối đa 2),
  chỉ khi có. App hiện ở chân thẻ «· dùng gu của Linh» / «· dùng gu của Linh và Tú».
- Sheet «Gu của hai bạn» đổi lời: bật là cho người ấy thấy gu và cho **Nếp và Rủ Đi AI trong chat của hai
  bạn** dùng gu. Người có đồng ý cũ thấy dòng giải thích và nút «Bật lại cho chat».

### 3.6 Tối thiểu hoá dữ liệu

Chỉ thẻ gu trong từ vựng đóng (`interests`). Công cụ **không** đọc tên hiển thị (nhãn lấy từ danh bạ
lượt mà worker đã dựng), **không** đọc ngân sách, **không** đọc `pair_shared_constraints`, **không** đọc
tin nhắn. Cổng `aigate` `TestGuDoiDocDungCot` ghim: nguồn gu đọc đúng sáu bảng và không câu SQL nào nêu
`pair_shared_constraints`, `display_name`, `budget*`, `body`, `content`, `messages` (có canary đỏ).

### 3.7 Lưu giữ và ghi nhật ký

Gu không được lưu thêm ở đâu: không vào bộ nhớ ngắn hạn, không vào trí nhớ Nếp, không vào `result`.
`ai_turn_metrics` (lên v7) chỉ ghi **tên** công cụ `gu_hai_ban` trong mảng `cong_cu` đóng, không ghi gì nó
trả về (triển khai: `core migrate-chat` lên v7 trước `serve`/`work` bản mới). Không nhật ký, số đo hay
dấu vết nào chứa chữ gu. Thứ còn lại là câu trả lời đã đăng (do người
gọi yêu cầu, verifier đã chấm) và nhãn `doc.gu` trên thẻ.

## 4. Sửa ADR-0034

- §2.1: câu consent đổi thành «Cho người ấy thấy gu của mình, cho Nếp dùng khi phác tờ và cho Rủ Đi AI
  dùng trong chat của hai bạn». Đồng ý trước mốc chỉ phủ phần cũ.
- §3: thêm «Không cho Rủ Đi AI đọc gu của người chưa bật `chia_gu` theo lời mới (sau mốc ADR-0048), kể cả
  khi người kia đã bật; không đọc gu ngoài lượt cặp đôi; không đọc gu khi mô hình không gọi công cụ».

## 5. Cái này KHÔNG cho phép

- Không đọc gu trong phòng đám bạn (nhóm, chat hai người thường), kể cả khi hai người có sổ đôi.
- Không đọc gu bằng heuristic từ khoá, không nạp gu sẵn vào lời nhắc.
- Không suy ra gu từ chat, không ghi gu vào trí nhớ Nếp.
- Không dời `MocChat` về trước.
- Không đọc ràng buộc chung của sổ đôi hay ngân sách qua công cụ này.

## 6. Bằng chứng và còn mở

- Bằng chứng: xem commit của lát P5 (đơn vị `gudoi`, `tools`, `aiharness`; tầng Postgres `aidoc`,
  `chatassist`, `metrics` v7; cổng `aigate`; T1 ca `15-doi-gu-hai-ban`, `16-hai-ban-khong-co-gu`; test app).
- Còn mở: chưa đo với mô hình thật (T3); khoảng hở bản app cũ (§3.1); ảnh chụp sheet và chân thẻ trên máy
  thật; Lead chưa ký (đây là quyết định của chủ sản phẩm).
