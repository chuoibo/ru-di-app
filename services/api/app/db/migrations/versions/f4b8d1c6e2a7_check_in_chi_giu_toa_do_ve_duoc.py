"""Check-in chỉ giữ toạ độ vẽ được (M7, HANDOFF-GEO §2 của vnlocal).

Toạ độ của một check-in chép từ bảng `places`. Từ khi danh mục là dữ liệu thật
(PR #645), `places.lat/lng` có thể NULL (~2%) hoặc chỉ là tâm phường / tâm tỉnh
/ điểm model đoán (~40%), phân biệt bằng `geo_precision`. Trước bản này:

- check-in ở quán KHÔNG có toạ độ chết ở `payload_matches_kind` (bắt check-in
  phải có lat/lng) -- route trả 500 ở cả hai nửa;
- check-in ở quán chỉ có toạ độ tâm lưu vĩnh viễn một điểm giữa phường/tỉnh,
  rồi lớp «đã tới» và heatmap đọc lại điểm đó như chỗ nhóm đã đứng.

Ràng buộc mới nới đúng một chuyện: check-in được có `lat`/`lng` cùng NULL.
Nửa điểm vẫn bị từ chối; ảnh vẫn không mang toạ độ.

Backfill: check-in đã ghi ở quán mà điểm không vẽ được (precision khác
rooftop/street) được gỡ toạ độ. Dòng seed cũ có precision NULL giữ nguyên --
cùng luật với `app.places.geo_precision.mappable_point`.

Xuống được: điền lại toạ độ từ `places` cho check-in đang NULL. Quán nào vẫn
không có điểm thì không có gì để điền, và ràng buộc cũ không chứa nổi hàng đó;
`downgrade` dừng và nói rõ thay vì xoá kỷ niệm của người ta.
"""

from collections.abc import Sequence

from alembic import op

revision: str = "f4b8d1c6e2a7"
down_revision: str | Sequence[str] | None = "e8c4d2a7f913"
branch_labels: str | Sequence[str] | None = None
depends_on: str | Sequence[str] | None = None

CU = (
    "(kind = 'photo' AND image_url IS NOT NULL AND image_url <> '' "
    "AND ((place_id IS NULL AND place_name IS NULL) "
    "OR (place_id IS NOT NULL AND place_id <> '' "
    "AND place_name IS NOT NULL AND place_name <> '')) "
    "AND lat IS NULL AND lng IS NULL) OR "
    "(kind = 'checkin' AND image_url IS NULL "
    "AND place_id IS NOT NULL AND place_id <> '' "
    "AND place_name IS NOT NULL AND place_name <> '' "
    "AND lat IS NOT NULL AND lng IS NOT NULL)"
)

MOI = (
    "(kind = 'photo' AND image_url IS NOT NULL AND image_url <> '' "
    "AND ((place_id IS NULL AND place_name IS NULL) "
    "OR (place_id IS NOT NULL AND place_id <> '' "
    "AND place_name IS NOT NULL AND place_name <> '')) "
    "AND lat IS NULL AND lng IS NULL) OR "
    "(kind = 'checkin' AND image_url IS NULL "
    "AND place_id IS NOT NULL AND place_id <> '' "
    "AND place_name IS NOT NULL AND place_name <> '' "
    "AND ((lat IS NULL AND lng IS NULL) "
    "OR (lat IS NOT NULL AND lng IS NOT NULL)))"
)


def upgrade() -> None:
    op.drop_constraint("payload_matches_kind", "memories", type_="check")
    op.create_check_constraint("payload_matches_kind", "memories", MOI)
    op.execute(
        "UPDATE memories SET lat = NULL, lng = NULL "
        "WHERE kind = 'checkin' AND lat IS NOT NULL AND place_id IN ("
        "SELECT id FROM places WHERE lat IS NULL "
        "OR (geo_precision IS NOT NULL "
        "AND geo_precision NOT IN ('rooftop', 'street')))"
    )


def downgrade() -> None:
    op.execute(
        "UPDATE memories AS m SET lat = p.lat, lng = p.lng FROM places AS p "
        "WHERE m.kind = 'checkin' AND m.lat IS NULL "
        "AND p.id = m.place_id AND p.lat IS NOT NULL"
    )
    op.execute(
        "DO $$ BEGIN IF EXISTS (SELECT 1 FROM memories "
        "WHERE kind = 'checkin' AND lat IS NULL) THEN RAISE EXCEPTION "
        "'memories: check-in ở quán không có toạ độ, ràng buộc cũ không chứa "
        "được; xử lý tay trước khi hạ phiên bản'; END IF; END $$"
    )
    op.drop_constraint("payload_matches_kind", "memories", type_="check")
    op.create_check_constraint("payload_matches_kind", "memories", CU)
