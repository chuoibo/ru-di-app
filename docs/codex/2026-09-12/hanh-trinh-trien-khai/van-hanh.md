# Vận hành Hành trình — ADR-0028

## Nguồn đường đi

API gửi tọa độ và phương tiện tới Valhalla tự vận hành. Không gửi tên hội,
tên chặng, ID thành viên; không dùng dịch vụ routing công khai làm dự phòng.
Nền bản đồ vẫn tải tile/glyph OpenFreeMap như trước; đó là nhà cung cấp bản đồ,
không phải nguồn đề xuất hay vị trí thiết bị. App không lấy GPS.

Engine được ghim bằng digest trong `docker-compose.journey.yml`. Script chuẩn bị
PBF công khai ngoài repo, ghi SHA-256 và graph version vào `graph-manifest.json`.
Dữ liệu đã chạy tại máy kiểm thử: Việt Nam 2026-09-11,
`vietnam-3.8.3-cf11cef89f650c0b`. Đây là bản graph đã kiểm, không có giao thông
trực tiếp. Xe máy dùng costing `motor_scooter`, ô tô `auto`, đi bộ `pedestrian`.

```bash
python3 scripts/prepare_journey_routing.py --data-dir /srv/rudi-routing/vietnam-20260911
export MOBILE_ROUTING_DATA_DIR=/srv/rudi-routing/vietnam-20260911
export MOBILE_ROUTING_GRAPH_VERSION=vietnam-3.8.3-cf11cef89f650c0b
docker compose -f docker-compose.yml -f docker-compose.journey.yml up -d valhalla api
```

Lấy giá trị graph version từ output/manifest nếu đổi PBF. Dựng graph lần đầu
có thể lâu; API trả trạng thái chưa sẵn sàng trong thời gian đó. Container chỉ
expose cổng nội bộ, không publish Valhalla ra Internet. Engine và Docker không
ghi request log. Đặt thư mục graph riêng cho mỗi lần nâng cấp để có thể quay lại
image/graph đã kiểm cùng nhau. Giữ attribution OpenStreetMap/ODbL.

API có timeout mỗi lần gọi 12 giây, tối đa 50 chặng, ma trận có hướng tối đa
2.500 cặp, 2.000 lần đánh giá tìm thứ tự, tối đa bốn preview đồng thời và 30
preview/phút/actor mỗi process. Giới hạn process không thay thế hạn mức toàn
cụm nếu triển khai nhiều worker. Không tuyên bố tìm được tối ưu tuyệt đối.

## Triển khai và quay lại

Chạy `alembic upgrade head` tới `d6a2f93b81e7` trước khi đưa API mới lên.
Head này hợp nhất migration hành trình `27a9b83e10c4` và sổ hai người
`c4f27a90d1e3`; không sửa lại revision đã tồn tại. Migration hành trình chỉ bổ sung
metadata lịch trình; một ngày cũ được backfill, nhiều ngày chưa rõ được giữ
`null`; giờ cũ ghim và thời lượng chưa rõ giữ `null`.

Để tắt đề xuất, đặt `MOBILE_JOURNEY_SUGGESTIONS_ENABLED=0` rồi khởi động lại API.
Lịch trình đã lưu, sửa thủ công và tuyến hiện tại vẫn hoạt động. Khi routing
không sẵn sàng, không biến nét nối vị trí thành ETA đường bộ.

Không hạ schema khi client đang dùng v2. Downgrade loại bỏ ngày/thời lượng/
ghim/điểm hẹn/revision; vì vậy chỉ kiểm lên–xuống–lên trên schema thử hoặc sau
khi có bản sao lưu và kế hoạch dữ liệu cụ thể. Rollback chức năng thông thường
dùng feature flag, giữ schema và phiên bản API đọc được v2.

PUT itinerary yêu cầu `expected_revision` và `Idempotency-Key`. Replay vẫn
kiểm tra phiên và quyền thành viên hiện tại. Preview chỉ đọc, không cache vào
idempotency store. API cũ không được ghi timeline đã nâng cấp v2. Check-in tăng
revision để đề xuất tính trước đó không ghi đè trạng thái mới.

## Chạy kiểm thử độc lập

```bash
cd services/api
MOBILE_TEST_DATABASE_URL=postgresql+psycopg://USER:PASS@HOST/TEST_DB \
MOBILE_REQUIRE_POSTGRES_TESTS=1 \
python3 -m pytest tests/postgres/test_itinerary_postgres.py \
  tests/postgres/test_itinerary_migration_race.py tests/postgres/test_outings_postgres.py \
  tests/api/test_idempotency.py tests/db/test_migration_matches_models.py -q

MOBILE_TEST_VALHALLA_URL=http://PRIVATE_VALHALLA:8002 \
MOBILE_ROUTING_GRAPH_VERSION=VERSION_FROM_MANIFEST \
python3 -m pytest tests/journey tests/test_import_boundary.py -q
```

`scripts/seed_journey_demo.py` chỉ nhận PostgreSQL localhost, database `mobile`,
qua biến `MOBILE_JOURNEY_DEMO_DATABASE_URL`. Nó tạo hội/chuyến/điểm hẹn tổng hợp;
không nhập người dùng thật. Native evidence dùng dữ liệu này phải được gọi rõ
là API thật với dữ liệu thử, không phải thử trên hội thật hay production.
