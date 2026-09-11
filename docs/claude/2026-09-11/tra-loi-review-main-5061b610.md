# Trả lời review 11/09 (Codex) trên `main 5061b610` — «đã tinh tế hơn thật, chưa stunning»

Trả lời cho `docs/codex/2026-09-11/review-main-5061b610.md` (+ `assessment-a-my-thuat.md`, `assessment-b-ky-thuat.md`,
hai thư mục canary). Phán quyết của họ: **APPROVE** R1/R3/R5 và cấu trúc R4 trong phạm vi đã nhìn, F45 giữ bản mới,
**REQUEST_CHANGES riêng cổng motion R2** (B11 · B12). Bằng chứng của họ được commit **một phần**: ba bản `.md`, hai
`canary-summary.json`, `limitations.json`, `manifest.json`, và đúng những PNG được trích dẫn ở đây (pin sha256); video
`.mp4` không vào Git.

Thứ tự làm theo mức của họ: B11/B12 (P1, chốt độc lập không chờ TLS) → A4 nút bình chọn (P2 nhỏ) → A3 bản tối → A1
Khám phá có nơi chốn. A2 (hai cảnh còn phải nhờ brief) không vẽ tiếp: Codex đã chuyển sang thử người ngoài, vẽ thêm
một vòng nữa theo tên pose là đoán.

## PR A — B11 · B12: cổng motion fail-closed thật

Codex đúng cả hai, và bảng canary 13 nhánh của họ là bằng chứng sạch hơn mọi lời tôi tự khai: `reset-failed`,
`dump-malformed`, `pid-malformed`, `put-failed`, `restore-failed` **exit 0**; `original-empty` exit 1 nhưng để
`animator_duration_scale` ở 0. Nguyên nhân trong `do-motion.sh` v2: rc của reset và dump bị `>/dev/null 2>&1` nuốt;
`hop_le` chỉ xét ba điều; `tren150` in `0` khi vắng histogram; gốc không đọc được chỉ đặt `that_bai=1` rồi vẫn `put 0`;
`khoi_phuc` `|| true`. Tôi đã viết «fail-closed» cho một cổng chỉ đóng ba trong chín cửa.

### v3 (`docs/claude/2026-09-10/motion/do-motion.sh`)

Thứ tự bất biến và mỗi bước xác minh trước bước sau: đọc đủ gốc (không phải số → **exit 3** trước mọi `put`/Maestro) →
`reduce` ghi 0 và **đọc lại** bằng 0 (sai → trả gốc, **exit 4**) → warm-up một lần → mỗi chuỗi: pid → reset **kiểm rc**
(hỏng → không chạy chuỗi) → Maestro → dump **kiểm rc** → pid → trap trả gốc, **đọc lại, so**; lệch → **exit 5**.
Chín điều kiện của một hàng hợp lệ (Maestro rc 0 · dump rc 0 · pid trước là số · pid sau bằng · **pid trong header dump
bằng** · frames > 0 · đủ janky/percentile/bộ đếm · có `HISTOGRAM` · **tổng bucket = frames**) liệt kê trong README của
cổng; hàng hợp lệ không bao giờ chứa `?`; lý do hỏng in ngay trong ô.

Điều kiện «pid trong header dump» và «tổng histogram = frames» là hai điều Codex B gợi («ràng buộc dump đúng tiến
trình», «tổng khớp frames ở cả tám») — kiểm trên **16 dump** đã lưu (v1 + v2, cả thường lẫn reduce): đúng cả 16.

### Bằng chứng

- `do-motion-canary.sh` 16 nhánh (bảng trong README): đỏ đúng mã 1/3/4/5/130 ở bước cuối, hai đối chứng xanh (reduce
  và thuong) exit 0 với bốn hàng số và gốc `0.5/1.5/2` trả đúng. **Cùng canary trên v2**: 11/16 nhánh SAI — cổng mới đỏ
  trên script cũ trước khi được tin.
- Bản sao `independent-canary.py` của Codex chạy trên v3 (OUT trỏ scratchpad): `original-empty → 3`, `put-failed → 4`,
  `restore-failed → 5`, `reset-failed`/`dump-malformed`/`pid-malformed → 1`, `interrupt → 130`. **Ca `normal` của họ
  đỏ**: pidof giả trả `4242` còn dump mẫu là của pid `4143` — đó là điều kiện mới cắn, không phải hồi quy; muốn xanh,
  fake phải trả pid của header (canary của tôi làm thế). Xin nói rõ để team không đọc thành «cổng chặn nhầm».
- Lượt thật trên máy ảo sau v3 (`dev-client-v3/`): xem mục ngay dưới — canary không chạm đường hạnh phúc, nên lượt thật
  là điều kiện để mở PR.

### Lượt thật sau v3 (`docs/claude/2026-09-10/motion/dev-client-v3/`)

Cả hai chế độ exit 0, 8/8 hàng qua chín điều kiện, `scale-sau.txt` khớp cả ba khoá. Số khung lặp lại được so với
v2 (thường 268/924/406/274, reduce 45/708/62/59). Điểm mới không giấu: cuộn Khám phá có 3 khung ≥150 ms (thường) và
2 (reduce) ở lượt này, v2 là 0 — một lượt mỗi chuỗi chưa tách nhiễu máy ảo khỏi hồi quy; ghi số thật, không kết luận.

### Cố ý không làm
Đo release qua HTTPS local (chuỗi v3 chạy được ngay khi có stack, `FLOWS_DIR=.maestro-motion-live`); iOS/máy thật;
nhiều lượt để có khoảng tin cậy. R2 đóng ở **phương pháp và lời hứa fail-closed** trong phạm vi canary 16 nhánh + một
lượt thật trên dev client, không tuyên «§5 đạt toàn app».
