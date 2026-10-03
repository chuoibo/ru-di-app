# Gỡ bản trải nghiệm «Team Đà Lạt» — app chỉ còn dữ liệu thật

Ngày 2026-10-03. Chủ sản phẩm: «production hết, xoá Team Đà Lạt, dữ liệu thật sẽ chạy
real, xoá những thứ stale và demo» — trước buổi ra mắt app tối cùng ngày.

## Vì sao Team Đà Lạt còn sau đợt «dữ liệu thật» 27/09

Đợt 27/09 (`6aa0071a`, `532ff254`) nạp **danh mục quán** thật vào DB `rudi` và gỡ danh
mục bịa; stack production không seed. Nhưng câu chuyện Team Đà Lạt không nằm ở DB mà
nằm **trong mã app**: `src/rudi/fixtures.ts` + trạng thái nháp trong `session.tsx`, và
mỗi route có hai nhánh — có phiên thì màn Live, **không phiên thì màn fixture**. Chưa
đăng nhập (và trước 23/09 cả đăng nhập mà chưa có nhóm) là thấy nhóm 8 người, bill Xóm
Lèo, chuyến Đà Lạt 17–19/10.

## Đã gỡ

- Màn fixture: `screens/{Group,Discovery,Outing,Memories}.tsx` (xoá cả file), phần mẫu
  của `Bill.tsx` (giữ `SettlementScreen` live), `Profile.tsx` (hồ sơ/chuyến/tài chính/
  thành tích mẫu), sổ cặp đôi mẫu (`SoDoiProvider`, `fixtures-doi.ts`, chế độ «đóng vai
  người ấy»), cửa «Vào bản trải nghiệm Team Đà Lạt», nhãn/cửa «Demo» trên thanh tiêu đề
  (`DemoBadge`, `CuaDemo`), `cua-fixture.ts` + cờ `EXPO_PUBLIC_RUDI_FIXTURE`, bốn trang
  `app/dev/*`.
- Dữ liệu mẫu: `fixtures.ts`, `places.ts`, `money.ts`, `vote.ts`, `luu-tru.ts`,
  `hanh-trinh/toa-do-mau.ts`, `src/fixtures/*`, năm ảnh stock + mục ghim allowlist.
  `session.tsx` chỉ còn phiên thật; lần mở đầu tiên xoá blob nháp cũ `rudi.phien.v1`.
- Mọi route: chưa đăng nhập → `CuaDangNhap` (đọc xong phiên rồi mới quyết). Album/tường
  nhóm thiếu mã khi ĐÃ đăng nhập trước đây rơi về album mẫu — nay chuyển về Lên plan /
  Tin nhắn.
- Bản đồ trống mở ở TP.HCM (nơi có danh mục thật), không còn tâm Đà Lạt.
- Maestro: bảng fixture mặc định (01–12, 91, canary 09, `_vao-app*`) và `.maestro-motion`
  gỡ. `scripts/mobile_native.sh` không còn chế độ mặc định: chạy không cờ thì thoát 2
  «không đo được» kèm cách chạy `--otp`. Flow 00 (màn chào + dấu vân cây, neo 2b) **giữ**,
  chạy ở mọi bảng trừ `--live`. Canary của `--dang-nhap` dùng `canary_otp`.

## Giữ lại có chủ đích

- `src/rudi/nhom-demo.ts`: danh tính 7 người seed của **harness e2e** trên stack dùng xong
  bỏ (`tests/e2e`, `scripts/e2e_demo_people.py`). Không màn nào import; có test canh
  (`rudi-cua-otp.test.mjs`). `chat/nhom.ts` không còn phụ thuộc nó (`khoiDongNhom` nhận
  chủ nhóm + tên nhóm).
- `scripts/seed_demo_data.py`, `make demo-rudi`, `tools/seed-rudi-world*.mjs`: seed cho
  stack dev/CI, không chạm stack production (stack đó không seed). Gỡ chúng là việc
  riêng, kéo theo `e2e_slice.sh`, parity và ~10 test CI.
- `to-giay/so-fixture.ts`: kiểu `RangBuoc` và các phép chuyển trạng thái thuần còn test.

## Sổ tay Nếp

`_rut.json` rút lại: các route không còn «thừa hưởng» nút của màn mẫu (trước đây Nếp có
thể chỉ đường qua nút chỉ có trong demo). Đồ thị đường đi 1810 → 1576 cặp có đường.
`ca-nhan.md` tả trình sửa hồ sơ thật (Tên/Giới thiệu/Thành phố/«Lưu hồ sơ») thay cho
«Bio/Xong» của bản mẫu. Truy hồi: recall@5 không đổi ở mọi nhóm; MRR ghim lại, hai nhóm
giảm hơn 0.01 (xem `truy_hoi_test.go`).

## Số đo (cây làm việc trên 2b6c9360)

- `npm test` (apps/mobile, có `expo export` web): 1488/1488 xanh. Bundle web: 0 lần
  «Team Đà Lạt», «Vào bản trải nghiệm», «Dữ liệu demo», «Bánh căn Lệ», «Xóm Lèo», «Minh
  Anh»; đối chứng dương «Tài chính của tôi» 4, «Chưa có sổ nào để quyết toán» 1.
- pytest `services/api/tests tests` (image `rudi-ux-pytest-git:dev`): 2992 xanh, 6 đỏ —
  cả 6 đỏ y hệt trên cây gốc sạch (demo_watch, gate_failure_report, make_targets,
  motion_measurement: môi trường).
- `go test ./internal/huongdan/` và gate: guard, screens, go-vet, go-test, eval-kich-ban,
  pinned-import, shared, docker ĐẠT; contract, client-routes, server-routes, cors,
  ownership, python-touch, ruff, migration ĐẠT trong container; parity
  `scenarios=351 differences=0`.
- `scripts/e2e_slice.sh` (Postgres thật): 11 xanh, 0 đỏ, 4 skip (đường media Nếp chưa cấu
  hình trên máy chủ dùng một lần).
- Maestro `--otp` trên máy ảo (emulator-5610, dev client, stack QA tổng hợp), A/B cùng
  stack: cây gốc đỏ 29, 30, 41, 42, 43, 45, 47, 48 (8/25); bản này đỏ 29, 30, 36, 42, 43,
  45 (6/26). 36 chạy lại riêng hai lượt: xanh 2/2 (chập chờn — dòng «Sở thích» dưới nếp
  gấp). Bảng mini 00 + 22: neo 2b cắn đúng dòng dấu vân, canary OTP đỏ đúng chỗ, XANH.
  Năm flow đỏ ở cả hai bản là nợ có sẵn (29/42: menu Cá nhân dưới thẻ «Dấu mốc mới» mà
  flow không cuộn; 30 bình chọn; 43 story; 45 hồ sơ).

## Còn mở

- **Stack production đang sập** lúc ghi: `rudi-vnlocal-core-1` khởi động lại liên tục
  («cannot check the chat schema; is the database reachable?»), `rag-indexer` cũng vậy.
  Postgres :5432 và MinIO :9000 của máy vnlocal từ chối kết nối cả qua LAN theo tên lẫn
  Tailscale; agy :20131 vẫn trả 200 — máy sống, dịch vụ DB/MinIO tắt. Phải bật lại trên
  máy vnlocal.
- OTP production chỉ gửi SMS khi có `MOBILE_SMS_GATEWAY_URL`; thiếu thì mã chỉ vào log.
  Chưa kiểm được cấu hình thật (file bí mật ngoài repo).
- Dữ liệu test/E2E còn sót trong DB production chưa được kiểm (đọc DB production bị chặn
  quyền trong phiên này).
- Mất độ phủ: hai test trình duyệt của bản đồ hành trình (`rudi-hanh-trinh-web`,
  `-no-webgl`) lái chuyến mẫu trên `/plan` chưa đăng nhập — gỡ cùng bản mẫu. Logic hành
  trình vẫn có unit test; bản đồ trên web chưa có phép đo thay thế.
