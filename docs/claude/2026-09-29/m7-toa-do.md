# M7 «Toạ độ» — phía RuDi sau HANDOFF-GEO của vnlocal

- Ngày: 2026-09-29
- Nhánh: `claude/p0-m7-toa-do` (ba lượt, mỗi lượt một commit)
- Nguồn: `HANDOFF-GEO.md` của repo vnlocal (28/09); mốc M7 trong
  `docs/claude/2026-09-23/ke-hoach-nap-du-lieu-dia-diem.md`
- Verdict: không có reviewer, không có verdict

## Luật

Chỉ `geo_precision` ∈ {`rooftop`, `street`} được dùng làm **điểm**: cắm ghim, đo
khoảng cách, tính đường, lưu lại. Tâm phường (`ward_centroid`), tâm tỉnh
(`province_centroid`) và điểm model đoán (`suy_luan`) chỉ được dùng để lọc theo
vùng. NULL có điểm là dòng seed cũ, được tin.

Luật này giờ nằm ở một chỗ cho mỗi nửa, và bốn chỗ cùng một luật:

| Nửa | Chỗ |
|---|---|
| Nạp | `services/core/internal/ingest/place_v1.go` `Record.MappablePoint` |
| Go | `services/core/internal/repo/places.go` `Place.MappablePoint` |
| Python | `services/api/app/places/geo_precision.py` `mappable_point` |
| App | `apps/mobile/src/screens/kham-pha/places.ts` `veDuocLenBanDo` |

## Đã làm

1. **«Chỉ đường»** tìm theo «tên, địa chỉ». Toạ độ vẽ được chỉ còn để định
   hướng tìm kiếm (HANDOFF-GEO §2: `rooftop` vẫn có thể lệch vài chục mét).
2. **Check-in / Kỷ niệm** ở quán không có toạ độ hết 500. Check-in ở quán chỉ
   có toạ độ tâm không lưu điểm tâm lên tường nhóm. Migration `f4b8d1c6e2a7`
   nới ràng buộc cho check-in không có điểm, và backfill gỡ điểm tâm của các
   check-in cũ.
3. **Bản đồ nhóm, «Điểm hẹn», hành trình** không dùng điểm tâm nữa. Hai lỗi 500
   trên dữ liệu thật tìm thấy trong lúc làm cũng đã sửa:
   - bản đồ bắt ghim phải có `rating`, mà quán nạp từ vnlocal không có;
   - «Điểm hẹn» bắt ứng viên phải có `address`, mà nhiều quán không có.

   Cả hai trường giờ là Optional ở Python, Go và app.

## Câu hỏi đã đóng

«Chạy agy geocode thêm 1.750 dòng tâm tỉnh không?»
(`docs/claude/2026-09-23/ban-giao-nap-du-lieu-dia-diem.md` mục 11): **hết cần**.
Bên vnlocal đã tra toạ độ cho mọi địa điểm (97,6% có toạ độ, chỗ mới tự tra).
18% còn ở mức tâm tỉnh là do trên web không có địa chỉ cụ thể, tra lại cũng
không ra.

## Còn mở — bàn giao cho người giữ RAG (không sửa ở đây)

RAG (`services/core/internal/rag/**`) dùng `CoToaDo` (tức «có toạ độ») ở ba chỗ
lẽ ra phải dùng «điểm vẽ được»:

| Chỗ | Hậu quả với toạ độ tâm |
|---|---|
| `rag/phamvi.go:138` `PhamVi.Chua`, lọc sống ở `rag/retrieve.go:139-154` và `rag/loc.go:75` | Quán cấp tỉnh bị kéo vào (hoặc đẩy ra khỏi) một thành phố chỉ vì tâm tỉnh rơi trong hộp của thành phố |
| `rag/chunk_place.go:87` → `rag/build.go:95` → SQL `rag/retrieve.go:318` | Như trên, ở tầng SQL. Sửa phải sửa cả hai tầng, không thì hai tầng lệch nhau |
| `rag/nap/trung.go:77` (gộp trùng khi cách nhau ≤ 80 m) | Mọi quán cùng một phường chung tâm phường, khoảng cách là 0, nên hai quán khác nhau mà hồ sơ giống nhau bị **gộp làm một**, một quán biến khỏi chỉ mục. `ingest.DuplicateCoordinatesAreSuspect` đã nói chỉ `rooftop` trùng mới đáng ngờ |

Đề xuất: dùng `repo.Place.MappablePoint` (hoặc cùng luật) ở ba chỗ trên, và
thêm ca `ward_centroid` vào `TestTimTrung`.

## Còn mở khác

- iOS không mở `geo:`. Bản dựng hiện chỉ có Android.
- Lọc theo phường: `ward_code` mới phủ 49% (địa chỉ nguồn còn dùng tên phường
  trước sáp nhập 2025). App chưa có tính năng lọc theo phường nên chưa cần.
- Tính đường thật (Valhalla) chưa được dựng trên stack vnlocal. Hành trình vẫn
  vẽ đường chim bay ở app.
- `services/core/contract/ir/*.json` lệch thứ tự route so với cây (có sẵn trên
  `main`, không do nhánh này; IR không chứa schema trả về nên thay đổi ở đây
  không đụng IR).
