# ADR-0026 — Hành trình là chế độ xem không gian của lịch trình; bản đồ chiếu toạ độ địa điểm, không toạ độ người

- **Trạng thái:** 🟡 **ĐỀ XUẤT** 2026-09-11 — Lead đánh khi merge. Hiện thực trên nhánh `claude/p0-w-hanh-trinh`.
- **Quyết định bởi:** Lead (yêu cầu 2026-09-11: chế độ xem thứ hai của Timeline, không phải itinerary độc lập).
- **Sửa phạm vi** [ADR-0018](ADR-0018-vi-tri-thiet-bi-va-diem-den.md) §2.3 và [ADR-0016](ADR-0016-pham-vi-v1-va-dang-nhap-otp-google.md) mục «map SDK»: cấm bản đồ cho chỉ đường / theo dõi người vẫn đứng; cho phép một SDK bản đồ để **chiếu lịch trình lên địa lý**.

## 1. Bối cảnh

Timeline trả lời «làm gì, lúc nào». Người dùng cũng cần «đi đâu, ngày chảy trên mặt đất ra sao». Hai câu hỏi cùng một nguồn sự thật: danh sách chặng của ngày / kèo. Tách một store bản đồ riêng sẽ lệch thứ tự với Timeline.

ADR-0018 (đề xuất) cấm SDK bản đồ vì đợt M10 chỉ cần `geo:` cho chỉ đường và GPS một lần cho «Gần tôi». Lệnh này khác: không đọc vị trí thiết bị, không lưu toạ độ người, không chỉ đường turn-by-turn thay app bản đồ của máy.

## 2. Quyết định

### 2.1 Một lịch trình, hai cách nhìn

Màn Timeline hiện có thêm segmented **Lịch trình | Hành trình**. Đổi chế độ không đổi route. Cả hai view đọc:

- `activities` — đúng các chặng Timeline, theo thứ tự thời gian;
- `routeSegments` — chuyển động giữa hai chặng **có toạ độ**, suy ra từ cùng danh sách.

Không có itinerary thứ hai. Sửa thứ tự / xoá chặng ở Timeline thì polyline Hành trình vẽ lại.

### 2.2 Toạ độ là của địa điểm, không của người

Marker lấy `lat`/`lng` từ danh mục địa điểm (live) hoặc toạ độ công khai OSM gắn trên `DemoPlace` (fixture). Chặng không có toạ độ vẫn nằm trên Timeline, không thành marker, không bị bịa một điểm.

Check-in vẫn là «đã tới» (F46). Không GPS, không nút «Vị trí của tôi», không quyền `ACCESS_*_LOCATION` trong đợt này.

### 2.3 MapLibre, không token trả tiền

Web: `maplibre-gl` + style OpenFreeMap Positron. Native: `@maplibre/maplibre-react-native`. Attribution OpenStreetMap bắt buộc trên map.

Đường đi: gọi OSRM công khai (`geometries=geojson`) với timeout 4 giây; hết hạn hoặc lỗi thì polyline geodesic. Không API routing trên máy chủ Rủ Đi. Không persist kết quả routing.

### 2.4 Số trên map phải tính được

Mét là số nguyên (haversine làm tròn). Phút ước lượng từ 25 km/h (xe máy đô thị). «Hiệu suất tuyến» chỉ hiện khi có ≥ 2 chặng có toạ độ: số nguyên `(quãng nearest-neighbor / quãng hiện tại) × 100`. Thiếu dữ liệu thì không hiện số — cùng luật với badge phần trăm giả ở danh mục.

«Tối ưu lộ trình» là nearest-neighbor trên các marker rồi ghi lại thứ tự vào itinerary đang dùng, không gọi TSP server.

## 3. Hệ quả

- Dev client native phải rebuild vì MapLibre native. `expo export --platform web` không cần native binary.
- ADR-0018 §2.3 đọc lại: «không bản đồ cho chỉ đường / theo dõi người». `geo:` vẫn là cửa «Chỉ đường» ở chi tiết địa điểm.
- Không đụng `api/` / `db/` trong lát này.

## 4. Phương án đã bác

| Phương án | Vì sao bác |
|---|---|
| Trang itinerary / map độc lập | Hai nguồn dữ liệu, lệch thứ tự. |
| Mapbox token | Phụ thuộc trả tiền, không cần để chiếu OSM. |
| `react-native-maps` + Google | Khoá API, web kém, trái «không token». |
| GPS / My Location | ADR-0018; không cần để xem hành trình đã lên. |
| Hiệu suất 82% viết cứng | Cùng lớp dối với «AI MATCH 95%» giả. |

## 5. Cách kiểm chứng

- Test thuần: chiếu, bỏ chặng không toạ độ, haversine nguyên mét, fallback geodesic khi OSRM fail, NN reorder.
- Web: segmented không đổi URL; marker + sheet + «Khớp hành trình»; selected sống khi đổi chế độ.
- Native: flow Maestro fixture mở Timeline, bấm Hành trình, thấy «Khớp hành trình» — sau khi cài APK có MapLibre.
