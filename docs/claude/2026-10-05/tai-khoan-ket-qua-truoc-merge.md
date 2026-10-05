# Tài khoản tự quản (PR #665) — kết quả trước merge

Ngày 2026-10-05. Người làm: Claude, nhận bàn giao
`docs/claude/2026-10-04/handoff-tai-khoan-production-codex.md`. ADR: ADR-0055
(đổi số từ 0053 vì main đã dùng 0053/0054 trong lúc nhánh còn mở).
Verdict: không có reviewer người; đây là ghi chép bằng chứng, không phải chữ ký.

## Cái gì đổi so với bàn giao

1. **Gộp main d8bbd584** (B8/B9/B11, ADR-0053 Tờ giấy, ADR-0054): tám xung đột,
   manifest route đánh lại thứ tự, lockfile ghim lại.
2. **Ba review độc lập + review bảo mật nền** tìm các đường lạm dụng giới hạn
   thử; đã sửa (chi tiết trong ADR-0055 mục «Bổ sung sau review trước merge»):
   login chỉ đếm lần sai và giữ chỗ nguyên tử; trần mã email xuyên lượt gửi
   lại; đổi email không lộ địa chỉ; hạn mức mail kiểm lúc phát mã và dành 1/5
   cho khôi phục; reset kiểm thử thách trước khi băm; phiên genesis nhận 403
   chứ không 401; migration tài khoản có phiên bản (v2: trigger xoá tài khoản,
   username đã xoá không cấp lại, xoá otp_challenges còn HMAC số điện thoại).
3. **Frontend**: phiên bị thu hồi thì về cửa đăng nhập (native trước đây không
   biết), nút Google web theo lượt xác thực, đăng ký giữ form, mã lỗi mới.
4. **Cổng bị nới đã khôi phục**: ba scenario parity của route còn sống
   (chỉ bỏ 5 bước gọi cửa đã nghỉ); bảng Android chạy lại 27 flow sản phẩm
   trên tài khoản tổng hợp thay vì 1 flow đăng nhập. Flow 20/21/_dang-nhap-so
   nghỉ vì cả mục đích là cửa đã gỡ.
5. **Lỗi có sẵn trên main, sửa trên đường đi**: chia-se-web không chạy Node 20
   của CI; gudoi là bom hẹn giờ (đỏ từ chiều 04/10); màn địa điểm Android đỏ
   «Transform origin must have exactly 3 values» (px lẻ).

## Số đo

| Cổng | Kết quả |
|---|---|
| `gate.sh --strict` toàn phần, cây sạch 8430b13c | 30 ĐẠT, 2 HỎNG, 0 bỏ qua. HỎNG: ruff (format một file test — sửa ở ebd9a498), go-milvus (trùng lúc đĩa đầy, xem dưới) |
| mobile-native trong lượt đó | ĐẠT (3253 s): 27 flow + `_43b`, kiểm máy chủ sau flow, canary |
| parity trong lượt đó | ĐẠT (3710 s); agent đo riêng: dev 347/10663/0 khác biệt, prod 22/598/0, canary bắt đủ |
| ai-infer-milvus, e2e, chat-e2e, crypto, go-postgres, go-media, go-broker | ĐẠT |
| `account_auth_mutants.sh` (cây sạch đúng SHA) | identity xanh; canary + 4 mutant (nonce, thu hồi phiên reset, trần mã theo email, khoá theo cặp) đỏ đúng test dự đoán |
| tầng Go-PG `accountauth` | 49 ca PASS, 0 SKIP, sentinel |
| mobile `npm test` | 1540 / 0 / 0 |
| pytest gốc (container) | 19 đỏ giống hệt cây sạch main d8bbd584 (môi trường container), không khác biệt mới |
| ruff chạy lại, cây sạch ebd9a498 | ĐẠT |
| go-milvus chạy lại, cây sạch ebd9a498 (watchdog đĩa) | **Chưa đo được — giới hạn đĩa của máy.** Woodpecker của Milvus chặn ghi khi ổ dùng > 90%; máy dùng chung chỉ còn 55–65 GB/591 GB và chính dữ liệu test đẩy qua ngưỡng. `TestHybridDauCuoiKhongViPham` (lượt trước treo 300 s) lần này PASS khi đĩa còn dưới ngưỡng (60 lượt truy hồi, 514 mục, 0 vi phạm); test sau đó đỏ «channel tsafe stalled» đúng lúc log Milvus ghi `writesBlocked=true`. PR không đổi dòng nào trong hybrid/vectordb/rag/nap/rerank/ai-infer; CI bỏ qua tầng này trên main như trên nhánh. Chạy lại khi máy có ≥ 80 GB trống: `scripts/gate.sh --strict go-milvus`. **Đã chạy lại 05/10 chiều trên `752bbc49`: ĐẠT, xem cuối mục sự cố đĩa.** |

## Triển khai và nghiệm thu thật

Stack nghiệm thu chạy 61fcb34b (api/core); migrate Alembic `d5e1a7c3b902`
và migration tài khoản v2 sau một bản sao lưu mã hoá đã kiểm đọc lại (159 bảng).
Caddy lấy địa chỉ client thật qua Funnel (header giả bị bỏ qua — kiểm bằng log
tạm đã gỡ). Theo dõi một giờ: 121 mẫu, 0 lỗi HTTP ngoài dự kiến, 0 lỗi mail
worker, p95 16,9 ms.

Nghiệm thu thật bằng email thật (Brevo → Gmail, địa chỉ +tag trong hộp thư
của chủ sản phẩm, đọc mã chỉ qua tìm theo tiêu đề/người gửi), qua một lối vào
loopback vào cùng core/DB/SMTP: đăng ký A và B; sai/đúng mật khẩu; tải lại giữ
phiên; cookie HttpOnly, không token trong localStorage; reset bằng mã thật thu
hồi cả phiên web lẫn bearer thiết bị khác; xác thực lại rồi đổi email (mã tới
địa chỉ mới) và đổi mật khẩu, phiên cũ 401; đăng xuất tất cả; tra username
không phân biệt hoa thường, tắt cho tìm thì 404 như không tồn tại; kết bạn
nhận trên UI; mời nhóm — người được mời 403 trước khi nhận, nhận trên UI thì
2 thành viên; nhóm riêng khác của A 403 kể cả tự mời; X-Actor-ID giả không đổi
danh tính; buổi đi thấy được với thành viên, 403 với người ngoài, hiện trên UI.
Bằng chứng chi tiết ngoài repo (`evidence/real-acceptance-release6.json`).

## Sự cố

Lúc ~01:34 UTC ổ đĩa máy đầy 100% trong lượt cổng toàn phần (Milvus test cùng
dung lượng của các phiên khác); PostgreSQL nghiệm thu PANIC «No space left on
device» rồi tự phục hồi trong dưới 1 giây; đã kiểm số hàng tài khoản, phiên
bản migration và Alembic sau phục hồi. Lượt go-milvus chạy lại có watchdog
dừng khi đĩa trống dưới 45 GB. Máy dùng chung cần headroom đĩa trước khi chạy
tầng Milvus — đây là điều kiện vận hành, không phải lỗi mã.

### Cập nhật sau merge (05/10 chiều): go-milvus ĐẠT

- Ổ đầy không phải do dữ liệu test: container `rudi-vnlocal-milvus-1` đã ghi
  192 GB log docker (woodpecker local không bao giờ dọn WAL, 234k segment rỗng
  trên một kênh, auditor in 2 dòng/segment/10 s). Sửa ở `752bbc49` (Milvus
  vnlocal chuyển WAL sang MinIO, log giới hạn); ổ còn ~314 GB trống.
- `scripts/gate.sh --strict go-milvus`, cây sạch `752bbc49` (worktree
  detached, 0 file lệch): **ĐẠT** sau 300 s — 65 ca PASS trên 3 gói
  (`internal/hybrid`, `internal/vectordb`, `internal/vectordb/napkho`),
  18 sentinel có mặt, 0 bỏ qua. Chặng này khép lại; bảng cổng toàn phần của
  PR #665 giờ không còn chặng nào chưa đo.

## Còn mở (không chặn merge mã, phải làm trước khi mở rộng công khai)

- Google trên APK release và thiết bị thật; iOS là cổng riêng.
- Hạ tầng production: tên miền gửi mail + SPF/DKIM/DMARC (Brevo Free 300/ngày
  chỉ cho nghiệm thu), OAuth consent published/verified, hosting thay Funnel trên
  máy cá nhân, backup ngoài máy và đo RPO/RTO, giám sát/cảnh báo.
- Đánh giá bảo mật độc lập (người), log retention.
- Giới hạn đã biết ghi trong ADR-0055: trần theo username/email cho phép tạm
  dừng một người tới 24 giờ nếu kẻ phá có nhiều địa chỉ; PATCH /people/me còn
  trả lại discoverable_by_phone được gửi.
- Chat nhóm thật vẫn nhãn «Chưa mã hoá đầu cuối» (chương trình chat v2 riêng).
