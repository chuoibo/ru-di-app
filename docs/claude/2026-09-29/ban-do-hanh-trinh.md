# Bản đồ «Hành trình»: đường thật (Valhalla) và bản đồ giấy

PR #662, nhánh `claude/p0-hanh-trinh-duong-that`. Bốn lượt:

| Lượt | Commit | Nội dung |
|---|---|---|
| A | 81625350 | Dữ liệu đường |
| C | 471420af | Bản đồ giấy |
| B | 75a8b30f | Valhalla trong stack vnlocal |
| D | 3a37be6b | Nét mực tự vẽ và các sửa sau review |

## Đồ thị đường đang chạy

- Engine: `ghcr.io/valhalla/valhalla-scripted:3.8.3`, ghim theo digest trong `deploy/vnlocal/compose.yml`, service `valhalla`.
  - Chỉ `expose` cổng 8002, không publish ra ngoài.
  - Logging `none`.
- Dữ liệu OSM: `vietnam-260927.osm.pbf` từ Geofabrik, sha256 `3e3ba54b299577873c38d933a99ea0f50b8e29877e638aec015d14a3469ae51a`.
- Graph version: `vietnam-3.8.3-3e3ba54b29957787`.
- Thư mục dữ liệu: `~/.local/share/rudi-routing/vietnam-260927/`. Symlink `~/.local/share/rudi-routing/current` trỏ tới thư mục này.
  - Compose đọc thư mục qua `RUDI_ROUTING_DATA_DIR`; mặc định là `…/current`.
- Biến môi trường, đặt trong `~/.config/rudi/stack.env` (không commit):
  - `MOBILE_VALHALLA_URL=http://valhalla:8002`
  - `MOBILE_ROUTING_GRAPH_VERSION`
- Số đo lúc build: 143 s, RAM đỉnh 2.6 GiB, 2.5 GB trên đĩa.
- Preview trên core (`POST /outings/{id}/itinerary/preview`) trả `ready` trong khoảng 243 ms.

## Build lại khi có bản OSM mới

1. Chuẩn bị dữ liệu vào một thư mục mới:
   ```
   python scripts/prepare_journey_routing.py --pbf-url <pbf mới> --expect-sha256 <sha> --data-dir ~/.local/share/rudi-routing/vietnam-YYMMDD
   ```
   - Sai sha256 thì script dừng.
   - Script in ra graph version mới.
2. Build tile một lần: chạy container valhalla với thư mục đó và `build_tar=True`. Chờ đến khi `/status` có `tileset_last_modified > 0`.
3. Trỏ symlink `current` sang thư mục mới.
4. Sửa `MOBILE_ROUTING_GRAPH_VERSION` trong `stack.env`.
5. Deploy: `deploy/vnlocal/up.sh up -d --build`.
   - Không dùng `--remove-orphans`.
   - Không đụng milvus.
6. Kiểm: preview trả `source.graph_version` khớp với version mới.

**Rollback:** trỏ `current` về thư mục cũ, đặt lại graph version cũ, rồi deploy. Thư mục cũ vẫn còn nguyên vì mỗi bản OSM có thư mục riêng.

**Khi chuyển app sang máy vnlocal:** chép cả thư mục dữ liệu đã build sang, không cần build lại.

## Kiểm thu (Android và web; iOS CHƯA kiểm)

- Máy kiểm:
  - Android: AVD `rudi-ingest` (emulator-5580, `-gpu swangle`), dùng dev client.
  - Web: Chrome headless, xem `tests/chrome-cdp.mjs`.
- Dữ liệu: kèo QA «Sài Gòn một ngày», 4 quán thật, đi xe máy, tuyến Valhalla thật.
- Bộ ảnh đã mở ra xem nằm ở `.impeccable/review/` (gitignored), chụp lúc 23:16–23:19 ngày 29/09, sau lần sửa cuối:
  - điện thoại 412: sáng, tối, đang chọn mốc, cỡ chữ 1.3, khung giữa lúc vẽ, tem đang chờ;
  - tablet 1066 dp (bố cục rộng);
  - web 390, 412 sáng, 412 tối, 1280.
- Review độc lập bằng impeccable-finish-reviewer, hai vòng:
  - Vòng 1: 8 mục cần sửa.
  - Vòng 2: 5 mục xong, 3 mục xong một phần. Ba lỗi phát sinh do đợt sửa đã được sửa ngay sau đó; phần này tôi tự kiểm bằng ảnh, không có vòng review thứ ba.

## Việc còn mở

- Bản đồ chiếm khoảng 34% màn hình điện thoại ở khung đầu; mục tiêu là từ 45% trở lên.
  - Hướng sửa: gập phần chọn phương tiện vào một nút, hoặc mở trang ở nấc thấp hơn.
- Trên điện thoại, ở mức zoom vừa khung cả tuyến, nhãn phút có lúc bị tem hoặc nhãn KẾT THÚC đè.
- Trên emulator (GPU phần mềm), lớp nét native vẽ chậm hơn đồng hồ vài trăm ms, nên tem có thể «đóng» trước khi nét tới. Cần kiểm trên máy thật.
- Khi chọn một mốc, một mốc khác không liền kề có thể bị mép trang cắt ngang (ADR-0028: một mốc hoặc hiện đủ, hoặc khuất hẳn).
- Một chặng thiếu toạ độ chính xác làm cả ngày thành `missing_location`. Theo chủ sản phẩm, cách sửa là **bổ sung dữ liệu** để mọi địa điểm đều có toạ độ, không đổi logic tính đường.
- ADR-0028 vẫn ghi «chờ review độc lập trước phát hành».
- Dọn dẹp: xoá hai tài khoản QA (chủ kèo và khách). Cần chủ sản phẩm cho phép.
