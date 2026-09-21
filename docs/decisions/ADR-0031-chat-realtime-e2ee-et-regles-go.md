# ADR-0031 — Backend Go, chat realtime và E2EE

- Ngày: 2026-09-21.
- Quyết định sản phẩm: Lead chốt trong phiên lập kế hoạch và yêu cầu triển khai.
- Trạng thái triển khai: đang thực hiện; chưa có APPROVE độc lập hoặc gate production.
- Thay phần polling/plaintext và AI đọc chat của ADR-0016/0021/0024 khi cutover
  chat v2; không sửa bản lịch sử của các ADR đó, không sửa protocol v1.

## Quyết định

ADR-0028 đã thuộc hành trình bản đồ ở HEAD `534c0fd1`; số 0028 trong
bản kế hoạch ban đầu là mốc checkout cũ. ADR này dùng số 0031 để không
ghi đè quyết định đã tồn tại. ADR-0029 đã quy định chuyển Go; ở đây kế thừa
code và parity hiện có, không dựng backend song song. Các gate độc lập của
chương trình này theo chỉ thị trực tiếp mới của Lead và AGENTS.md.

1. Toàn bộ backend nghiệp vụ chuyển Go trong cùng chương trình; Python chỉ AI.
   Giữ contract legacy để port auth, nhóm/DM, social, ledger/allocator, bill,
   outing/vote, catalogue, guest web, media, notification, jobs và migration.
2. Go modular monolith, API/worker riêng; PostgreSQL là nguồn sự thật.
   Chat v2 dùng REST mutation và WebSocket authenticated catch-up; event log
   theo conversation sequence và transactional outbox. Redis chỉ fanout,
   presence và rate limiting; mất Redis không được làm mất event đã ACK.
3. Logical send ID, ciphertext, event và sequence commit cùng transaction.
   Replay kiểm quyền hiện tại. Read/delivery watermark chỉ tăng. Mỗi module
   chỉ có một writer. Không dual-write hoặc tự động downgrade plaintext.
4. Chat mới E2EE trên Android/iOS. Hướng tích hợp: MLS/OpenMLS qua native Rust.
   OpenMLS build native trong CI nhưng không test chính thức các target này;
   phải qua spike native và review crypto độc lập trước enable production.
   Go không có khoá chat. Mỗi thiết bị có credential, enrollment tin cậy,
   kiểm roster/epoch trên client, revoke barrier và outbox bền vững.
5. Tối đa 100 người/nhóm, 5 thiết bị/account; kiểm 500 MLS leaves và 1.000
   kết nối. Thành viên mới chỉ đọc từ lúc tham gia; thiết bị mới nhận lịch sử
   qua thiết bị tin cậy hoặc backup mã hoá tự chọn bằng recovery code.
   Restore tạo identity thiết bị mới, không resurrect ratchet cũ.
6. Lịch sử plaintext giữ riêng, chỉ đọc, có nhãn chưa từng E2EE và ACL hiện tại.
   Client cũ phải nâng cấp khi cutover. Không xoá dữ liệu để lách migration.
7. AI chỉ khi gọi. Mặc định lời gọi; trích đoạn thêm cần grant từng tác giả,
   gắn đúng bytes/invocation/purpose/expiry. Kiểm lại trước dispatch/publish.
   Provider đã thấy bytes thì không hứa thu hồi được. Kết quả sealed tới
   requester, có provenance; client đăng card ở epoch hiện hành.
   AI không tự chốt plan hoặc tiền. Việc chuyển sang nghiệp vụ cần xác nhận.
8. Media xử lý/strip EXIF trên thiết bị trước encrypt; storage giữ ciphertext.
   UI giữ giấy–mực/Nếp, dùng Impeccable và các token hiện tại. Bao phủ text,
   emoji/reaction, reply/edit/delete, ảnh, sticker, voice, receipts, typing,
   offline, đa thiết bị, push và slash. Gọi điện/video ngoài phạm vi đợt này.

## Cổng nghiệm thu

- PostgreSQL thật, nhiều connection, crash sau commit trước ACK, replay sau
  revoke, sequence/race/read-mark, migration mới và upgrade dữ liệu legacy.
- Native Android/iOS release build, ba account và đa thiết bị; crypto review
  độc lập, restore, malicious payload, key substitution và revoke offline.
- 1.000 connections: 100 tin/s 30 phút; burst 300 tin/s 60 giây; soak 24 giờ
  20 tin/s; ít nhất hai Go replica, Redis/storage failure và reconnect storm.
- RTT <=100 ms cùng vùng: delivery p95 <=800 ms, p99 <=2 giây; không mất/nhân
  đôi tin đã ACK. UI local-send p95 <=100 ms; jank <3% trên máy thật 60 Hz.
- Một nhóm 5 người thử nội dung tổng hợp; không còn P0/P1; APPROVE độc lập.

## Bất biến và giới hạn

Ba luật tiền không đổi: integer dong, tổng phân bổ chính xác, ledger là nguồn
sự thật. Go dùng integer/big rational. Backend vẫn PostgreSQL, không SQLite.
Thông tin participant, transcript, ảnh thật, secret không được đưa vào Git hay
gửi ra ngoài khi kiểm thử. Test dùng dữ liệu tổng hợp. E2EE không giấu toàn bộ
metadata và không xoá được bản sao đã được người nhận giữ. Không tự nhận
production-ready khi chưa có bằng chứng native/crypto/tải/người dùng.

Nguồn: [MLS](https://www.rfc-editor.org/rfc/rfc9420),
[application access control](https://www.rfc-editor.org/rfc/rfc9750.html#section-3.5),
[OpenMLS native targets](https://latest.openmls.tech/doc/openmls/index.html).
