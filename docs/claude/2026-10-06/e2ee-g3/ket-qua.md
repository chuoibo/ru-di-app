# G3 — chat v2 E2EE chạy thật: kết quả, cổng, còn mở (06/10/2026)

- Nhánh `audit-0510-g3-e2ee`; kế hoạch audit backend 05/10 (G0–G6), giai đoạn 3. ADR: `ADR-0057` (phụ lục ADR-0031).
- Protocol: chat v2 `rudi-chat-v2` (Rust `packages/chat-crypto`, OpenMLS 0.9.0). `protocol_version` v1 không đổi.
- Verdict: không có reviewer thật; **chưa có review crypto độc lập** (ADR-0031 §4 đòi APPROVE trước khi bật production).

## Đã làm, theo lát

| Lát | Nội dung | Commit chính |
|---|---|---|
| E1/E2 | Rust: một danh tính nhiều phòng, KeyPackage, outbox, ops v2 + media; FFI 9 symbol, `--locked` | trước `98f400d3` |
| E4/E7 | Module native Android (Kotlin, Keystore) + iOS (Swift, chưa biên dịch); engine `may-ma-hoa.ts`; làn fail-closed | đến `a9c5fc6d` |
| — | **Sổ phòng niêm trên máy** (RoomLog, AES-GCM, fsync mỗi khung) + **sổ "đã nhận"** Rust: app chết giữa `receive` và ghi sổ không làm kẹt phòng; hàng chờ gửi | `98f400d3` |
| — | SONAME (shim JNI ghi đường dẫn máy dựng → điện thoại không nạp được); thứ tự và giờ tin v2 | `33f792e3` |
| — | Màn «Thiết bị nhắn tin mã hoá» (dấu khoá tính từ khoá trên máy) | `7cea4f36`, `7049630f`, `6599e8f6` |
| G3b | Rủ Đi AI trong phòng E2EE: máy chủ giữ câu trả lời cho máy người hỏi, máy đó niêm `ai_card` vào phòng, mọi máy kiểm bằng biên nhận + digest; `chia_bill` tắt ở phòng E2EE | `7aa37163`, `dddb805b`, `779e5ede` |
| G3c | Danh sách cuộc trò chuyện cho phòng E2EE (`GET /v2/chat/summaries`), đánh dấu đã đọc trên làn v2 | `54c639d6` |
| — | Cổng: test push cô lập schema; fixture CapDoi hết bom hẹn giờ (đỏ cả trên `main`); aigate theo authority tách làn | `d40051a1`, `28eb032f` |

## Bằng chứng đã xem

- Emulator Android (`emulator-5610`, x86_64, bản debug, stack `e2e_slice --keep`): flow `.maestro/49-chat-e2ee.yaml`
  ĐẠT 3 lượt; đầu kia là `tests/drill/ban-doi-tac.mjs` (engine của app, MLS thật qua C ABI). Ảnh đã mở: phòng
  «Mã hoá đầu cuối», hai chiều, sau khi tắt mở app lịch sử còn, thứ tự đúng, giờ «Hôm nay · 23:28». Màn thiết bị:
  «Android · MÁY NÀY · 1/5» với dấu khoá.
- Ba lỗi chỉ emulator bắt được (drill và unit đều xanh): SONAME, thứ tự tin, giờ tin.

## Rà soát bảo mật nền: 10 phát hiện, đều đã vá

Roster nằm trong digest sổ "đã nhận" · bỏ trùng theo (người gửi, logical id) · dấu khoá «MÁY NÀY» từ khoá cục bộ ·
mở lại đúng danh tính khi đổi tài khoản · báo khi máy chủ không liệt kê máy này · `/delivered` buộc đúng logical id
của lời gọi · `/receipt` chỉ sau khi giao · `chia_bill` tắt ở E2EE (máy chủ không kiểm được lời gốc) · kiểm thẻ AI
theo từng tin · (và một lỗi ở lần trước tóm tắt). Mỗi bản vá có canary và đột biến đỏ đúng chỗ (xem commit message).

## Cổng ở `28eb032f`

- gate nhẹ `--strict` 12/12 (guard, ruff, contract, client-routes, server-routes, cors, ownership, python-touch,
  go-vet, go-test, migration) · `go_postgres_tier` 592 ca PASS (chatv2, chatv2http, push, nepnho, chatassist,
  chatlegacychange, aigate, dieuchinh, routes, httpapi, cmd/core, db) · `npm test` mobile 1566/1566 ·
  `scripts/chat_drill.sh` ĐẠT (Go drill + engine 2/2) · `gate.sh crypto` ĐẠT ở `a6969181` (34 canary, SONAME; Rust
  không đổi sau đó).
- **Parity ở `fc0b85cf`** (06/10, khung giờ phiên audit nhường, có bộ canh RAM ≥ 4 GiB): dev 348 kịch bản / 10671
  bước, làn phụ 1/35, prod 23/602 — cả ba **0 khác biệt**; canary «identity» ok ở mọi bảng, mọi chế độ hư hại đều bị
  bắt. (Lần ở `1d565850` có một lần canary lệch dấu thời gian `w1/reports` `f6`/`f0`; Go bỏ phần lẻ khi micro-giây = 0
  đúng như Python, lần chạy sạch này không lặp lại.)
- Đã vào `main` local bằng `ff-only` ở `fc0b85cf` (38 commit từ `811c893f`); chưa push `origin`.

## Sự cố: tải của phiên này làm soak 24 giờ của audit tự huỷ

05/10 18:09:50 UTC RAM khả dụng 1,15 GiB (sàn 1,5 GiB), soak chat lần 2 (`20261005-d8bbd584`) tự huỷ; lúc đó phiên
này chạy song song ba cổng nặng với emulator còn bật, swap đã cạn từ 17:16 UTC. Đã tắt emulator, dừng parity, dọn
stack, báo phiên audit; audit ghi `ABORTED-RESOURCE`, sẽ chạy lại cuối kế hoạch trên máy giữ riêng. Bài học trong
memory `feedback-tai-may-khi-audit-chay`.

## Còn mở

1. Push phía app (`expo-notifications`): cần `google-services.json` (FCM) và khoá APNs của chủ sản phẩm.
2. Chuyển lịch sử sang thiết bị mới, mã khôi phục, mã an toàn so ngoài kênh đầy đủ (ADR-0057 §1.3, §4.2–§4.3).
3. iOS: Swift chưa biên dịch lần nào; cần EAS/macOS.
4. Làn v2 còn hỏi lại mỗi 3 giây; route `/stream` chưa có client (G5, PER-FE-01).
5. Hiệu năng: mỗi lần gửi `UPDATE chat_v2_conversations SET last_sequence=last_sequence+1` trên một hàng mỗi phòng —
   audit nêu là ứng viên gây xếp hàng khi có một lần chậm (cần đo lại trên máy yên, G5).
6. AI trong phòng E2EE chưa đo đầu-cuối với mô hình thật (stack e2e không có khoá AI).
7. Review crypto độc lập và thử nghiệm 5 người (cổng ngoài, ADR-0031).
