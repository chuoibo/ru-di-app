# HANDOFF: RuDi cần gì từ máy vnlocal

- Ngày: 2026-09-27
- Từ: phiên RuDi (repo `chuoibo/ru-di-app`, nhánh `claude/p0-w31-nap-du-lieu-dia-diem`, PR #645), máy dev của RuDi
- Gửi: người vận hành máy vnlocal (repo `chuoibo/vnlocal`)
- Địa chỉ: IP LAN của hai máy không ghi trong repo này (repo guard coi chúng là dãy số dài). Máy vnlocal là `<VNLOCAL_HOST>`, IP ở mục 0 của `HANDOFF-KET-NOI.md`; máy dev là `<RUDI_DEV_HOST>`, chủ hệ thống biết
- Trả lời cho: `HANDOFF-KET-NOI.md` (vnlocal commit `0b0c3451`)
- Phạm vi: **PostgreSQL và MinIO**. agy-proxy nằm ngoài tài liệu này.

Đọc mục 0 rồi làm theo mục 1 → 5. Mục 6 là cách báo lại.

---

## 0. Tóm tắt 60 giây

RuDi hiện chỉ có 12 địa điểm tự bịa. Ta sẽ nạp danh mục thật của vnlocal vào DB của app, rồi xoá dữ liệu dev.

`HANDOFF-KET-NOI.md` ghi đúng: Postgres `vnlocal` và bucket `vnlocal-frames` là **nguồn chỉ đọc**, và «dữ liệu riêng của app để trong DB của app». Nhưng RuDi **chưa có DB của app ở đâu cả**: máy dev của RuDi đã thôi giữ database. Leader chốt đặt DB của app **trên máy vnlocal, là một database riêng**.

Việc cần bên bạn làm:

| # | Việc | Mức |
|---|---|---|
| 1 | Tạo database `rudi` và role `rudi_owner` ghi được, trong cụm Postgres 16 đang chạy | **Bắt buộc**, chặn mọi việc khác |
| 2 | Thêm index `places(synced_at)` ở DB `vnlocal` | Nên có |
| 3 | Tạo bucket `rudi-media` và user MinIO `rudi-core` | Nên có (dùng ở mốc M5) |
| 4 | Sao lưu hằng ngày DB `rudi` | Bắt buộc trước khi có người dùng thật |
| 5 | Đưa bí mật cho chủ hệ thống, **không** qua chat hay git | Bắt buộc |

Không cần mở cổng mới: `5432` và `9000` đã mở cho mạng LAN (`LAN_CIDR` của bạn). Từ máy dev, ta đã thông được `5432`, `9000` và `/minio/health/live` trả 200.

---

## 1. Database của app: `rudi`

### 1.1 Vì sao là database riêng chứ không là schema trong `vnlocal`

- Alembic của RuDi quản lý 68 bảng (cộng 7 bảng nạp dữ liệu của Go), gồm tiền, chat, người dùng. Nó phải là chủ trọn vẹn một nơi, không chen vào schema của pipeline.
- Hai bên sao lưu, khôi phục và cấp quyền độc lập với nhau.
- `rudi_app` giữ nguyên **chỉ đọc**. Đừng cấp thêm quyền ghi cho nó; RuDi dùng một role khác để ghi.

### 1.2 Lệnh tạo (chạy bằng superuser của cụm)

Mật khẩu dùng hex để khỏi phải URL-encode trong DSN:

```bash
openssl rand -hex 24      # → dán vào chỗ <RUDI_OWNER_PASSWORD> bên dưới, rồi lưu vào serve/.env
```

```sql
CREATE ROLE rudi_owner LOGIN PASSWORD '<RUDI_OWNER_PASSWORD>'
  NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION
  CONNECTION LIMIT 40;

CREATE DATABASE rudi OWNER rudi_owner ENCODING 'UTF8' TEMPLATE template0;
REVOKE ALL ON DATABASE rudi FROM PUBLIC;

\c rudi
ALTER SCHEMA public OWNER TO rudi_owner;

-- Không đặt read-only; không để statement_timeout 30 s như rudi_app.
-- Một lượt `apply` ghi khoảng 9.000 dòng trong MỘT giao dịch (cố ý: đợt nạp
-- hoặc vào trọn, hoặc không vào gì).
ALTER ROLE rudi_owner IN DATABASE rudi SET statement_timeout = '10min';
ALTER ROLE rudi_owner IN DATABASE rudi SET idle_in_transaction_session_timeout = '5min';
ALTER ROLE rudi_owner IN DATABASE rudi SET timezone = 'UTC';
```

Ghi chú:

- **Không cần extension nào.** Migration của RuDi chỉ dùng tính năng có sẵn của PG 16 (`gen_random_uuid`, JSONB, partial index, trigger, advisory lock).
- `CONNECTION LIMIT 40` chia cho ba tiến trình: API Go (`core`), API Python cũ và `rudi-ingest`. Cụm của bạn có khoảng 100 kết nối, nên phần còn lại vẫn đủ cho pipeline. Nếu bạn muốn chặt hơn thì báo, ta sẽ hạ pool.
- `rudi_owner` không cần quyền gì trên DB `vnlocal`. Việc đọc nguồn dùng `rudi_app`, như tài liệu của bạn.

### 1.3 Nếu bạn muốn tách hẳn khỏi cụm của pipeline

Có thể chạy một container `postgres:16` thứ hai trong `serve/docker-compose.yml`, ví dụ ở cổng `5433`, volume riêng. Lợi: nâng cấp và khởi động lại độc lập với pipeline. Giá: thêm một dịch vụ phải canh, và phải thêm `5433` vào chain `VNLOCAL-LAN` của `serve/tuong-lua.sh`. **Ta không đòi cách này**; chọn cách nào thì ghi vào mục 6.

---

## 2. DB nguồn `vnlocal`: một index

RuDi sẽ kéo tăng dần đúng như mục 3.4 của tài liệu bạn gợi ý, theo từng trang 500 dòng:

```sql
SELECT place_id, doc, synced_at FROM places
WHERE synced_at > $1 ORDER BY synced_at, place_id LIMIT 500;
```

Mục 3.2 liệt kê index trên `updated_at` nhưng không có trên `synced_at`. Nếu chưa có, xin thêm:

```sql
\c vnlocal
CREATE INDEX CONCURRENTLY IF NOT EXISTS places_synced_at ON vnlocal.places (synced_at, place_id);
```

Nhịp đọc của RuDi: một lượt mỗi 10 phút (khớp nhịp đồng bộ của bạn). Mỗi lượt vài truy vấn có index, dùng một kết nối, đọc xong thì đóng. Lần đầu kéo khoảng 9.300 dòng, chia thành trang nên không câu nào chạm mốc 30 giây.

Ta cần hai điều giữ nguyên, hoặc báo trước khi đổi:

- cột `doc` vẫn là **nguyên bản ghi place.v1**, cùng hình dạng với file NDJSON `export/place-v1/` (ta đã kiểm trên lô 0001 → 0008)
- `place_frames.storage_key` và `co_trong_minio` giữ đúng nghĩa như mục 3.2

---

## 3. MinIO: bucket của app

Hiện RuDi đọc ảnh từ `vnlocal-frames` bằng key `rudi-app` có sẵn, chỉ đọc; phần này đủ cho việc nạp. Bucket riêng dưới đây dành cho mốc **M5** (RuDi chuyển kho ảnh của chính nó từ đĩa sang S3: ảnh người dùng tải lên, ảnh đã sanitize). Tạo sẵn thì ta không phải quay lại xin.

```bash
mc alias set local http://127.0.0.1:9000 <root user> <root password>
mc mb local/rudi-media                     # riêng tư, không bật anonymous
mc version enable local/rudi-media         # để lỡ xoá nhầm còn lấy lại được

cat > /tmp/rudi-core.json <<'JSON'
{
  "Version": "2012-10-17",
  "Statement": [
    { "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:PutObject", "s3:DeleteObject"],
      "Resource": ["arn:aws:s3:::rudi-media/*"] },
    { "Effect": "Allow",
      "Action": ["s3:ListBucket", "s3:GetBucketLocation"],
      "Resource": ["arn:aws:s3:::rudi-media"] },
    { "Effect": "Allow",
      "Action": ["s3:GetObject"],
      "Resource": ["arn:aws:s3:::vnlocal-frames/*"] },
    { "Effect": "Allow",
      "Action": ["s3:ListBucket"],
      "Resource": ["arn:aws:s3:::vnlocal-frames"] }
  ]
}
JSON
mc admin policy create local rudi-core /tmp/rudi-core.json
mc admin user add local rudi-core "$(openssl rand -hex 24)"   # lưu secret vào serve/.env
mc admin policy attach local rudi-core --user rudi-core
rm /tmp/rudi-core.json
```

`rudi-core` ghi được **chỉ** ở `rudi-media` và chỉ đọc ở `vnlocal-frames`. Như vậy backend RuDi dùng một key cho cả hai việc, còn ảnh của pipeline không bao giờ bị app ghi đè.

Điện thoại người dùng không bao giờ chạm MinIO: backend RuDi đọc ảnh rồi phục vụ lại, đúng mục 4.3 của bạn.

---

## 4. Sao lưu DB `rudi`

DB `rudi` sẽ chứa dữ liệu không tính lại được: tài khoản, nhóm, sổ chi tiêu, chat. Danh mục địa điểm thì nạp lại được từ nguồn, nhưng những thứ đó thì không.

- `pg_dump -Fc -d rudi` mỗi ngày lúc 03:00, giữ 7 bản gần nhất, lưu trên **ổ khác** ổ chứa volume Postgres (ví dụ HDD).
- Ghi lệnh khôi phục vào `serve/` để ai cũng làm được: `pg_restore -d rudi --clean --if-exists <file>`.
- Cần có trước khi RuDi có người dùng thật. Trong lúc dev mà chưa kịp thì ghi rõ «chưa có sao lưu» ở mục 6, ta sẽ không đưa dữ liệu thật của ai vào.

---

## 5. Bí mật: cách đưa

Luật giống mục 2 của bạn: không commit, không dán vào chat hay issue, không nhúng vào app mobile.

Gửi cho **chủ hệ thống** qua kênh riêng. Chủ hệ thống tạo file sau trên máy dev `<RUDI_DEV_HOST>`:

```bash
install -m 600 /dev/null ~/.config/rudi/vnlocal.env
```

```bash
# ~/.config/rudi/vnlocal.env — mode 600, không commit, không copy ra ngoài máy
# DB của app (ghi được). Go và Python đều nhận dạng postgresql+psycopg://
MOBILE_DATABASE_URL=postgresql+psycopg://rudi_owner:<RUDI_OWNER_PASSWORD>@<VNLOCAL_HOST>:5432/rudi

# DB nguồn (chỉ đọc)
VNLOCAL_PG_DSN=postgresql://rudi_app:<PG_APP_PASSWORD>@<VNLOCAL_HOST>:5432/vnlocal

# MinIO
VNLOCAL_S3_ENDPOINT=http://<VNLOCAL_HOST>:9000
VNLOCAL_S3_ACCESS_KEY=rudi-core            # hoặc rudi-app nếu chưa làm mục 3
VNLOCAL_S3_SECRET=<secret của key trên>
RUDI_S3_BUCKET=rudi-media
```

Phiên RuDi nạp file này bằng `set -a; . ~/.config/rudi/vnlocal.env; set +a` ngay trong lệnh chạy, không in giá trị ra, không ghi vào log.

Lộ một bí mật thì xoay nó và báo ta. Ta chỉ cần sửa đúng dòng đó trong file.

---

## 6. Báo lại

Chạy các lệnh kiểm dưới đây **từ một máy trong LAN không phải `<VNLOCAL_HOST>`** (để kiểm luôn đường mạng), rồi trả lời bằng bảng cuối mục. Không cần dán output có bí mật.

```bash
set -a; . ~/.config/rudi/vnlocal.env; set +a
DSN=${MOBILE_DATABASE_URL/+psycopg/}

# 1. rudi_owner ghi được ở rudi, và có quyền đổi lược đồ (Alembic cần)
psql "$DSN" -Atc "select current_user, current_database(), current_setting('transaction_read_only'), current_setting('statement_timeout')"
#    → rudi_owner|rudi|off|10min
psql "$DSN" -c "create table probe_rudi(x int); insert into probe_rudi values (1); drop table probe_rudi;"
#    → CREATE TABLE / INSERT 0 1 / DROP TABLE

# 2. rudi_app vẫn chỉ đọc ở vnlocal, và index mới có mặt
psql "$VNLOCAL_PG_DSN" -Atc "select count(*) from places"
psql "$VNLOCAL_PG_DSN" -Atc "select indexname from pg_indexes where tablename='places' and indexdef like '%synced_at%'"
#    → places_synced_at

# 3. rudi-core: ghi được rudi-media, KHÔNG ghi được vnlocal-frames
mc alias set rudi "$VNLOCAL_S3_ENDPOINT" "$VNLOCAL_S3_ACCESS_KEY" "$VNLOCAL_S3_SECRET"
echo probe | mc pipe rudi/rudi-media/probe.txt && mc cat rudi/rudi-media/probe.txt && mc rm rudi/rudi-media/probe.txt
echo probe | mc pipe rudi/vnlocal-frames/probe.txt     # → phải là Access Denied
```

| Mục | Xong? | Ghi chú (cách chọn, lệch gì so với tài liệu) |
|---|---|---|
| 1. DB `rudi` + `rudi_owner` | | cùng cụm `:5432` hay container riêng (cổng nào) |
| 2. Index `places(synced_at)` | | |
| 3. Bucket `rudi-media` + `rudi-core` | | |
| 4. Sao lưu hằng ngày | | giờ chạy, nơi lưu, hay «chưa có» |
| 5. File `~/.config/rudi/vnlocal.env` trên `<RUDI_DEV_HOST>` | | |
| Kết quả ba lệnh kiểm | | |

Có gì không làm được theo đúng tài liệu này (ví dụ muốn đặt tên khác, giới hạn kết nối khác), cứ làm theo cách của bạn và ghi vào cột ghi chú. Ta sẽ theo bên bạn.

---

## 7. Việc RuDi sẽ làm sau khi nhận được (để bên bạn biết tải sẽ tới)

1. `alembic upgrade head` rồi `rudi-ingest migrate` lên DB `rudi` (tạo bảng, 34 tỉnh, 33 điểm đến cấp tỉnh). Một lần, vài giây.
2. Nạp danh mục: `land` (hiện đọc file NDJSON; bước tiếp theo đổi sang đọc `VNLOCAL_PG_DSN` như mục 2), rồi `apply`. Một giao dịch khoảng 9.000 dòng.
3. Ảnh: đọc `place_frames` + `vnlocal-frames`, lọc và chép vào kho ảnh của app. Lần đầu khoảng 2,8 GB đọc từ MinIO, sau đó chỉ phần mới.
4. `rudi-ingest purge-dev`: xoá 12 địa điểm bịa và 15 điểm đến cũ, **chỉ trong DB `rudi`**.
5. Chạy lặp `land` + `apply` mỗi 10 phút theo `synced_at`.

RuDi **không bao giờ**: ghi vào DB `vnlocal` hay bucket `vnlocal-frames`, đọc MongoDB, hay mở kết nối từ điện thoại tới `<VNLOCAL_HOST>`.

Tầng test PostgreSQL của RuDi (`scripts/go_postgres_tier.sh`, `make test-db`) **không chạy trên máy vnlocal**. Chúng tự dựng container Postgres dùng một lần ở máy dev, tạo và xoá schema liên tục, nên không thuộc về một máy dùng chung.

Liên lạc: phiên RuDi ở `/home/lakiet/project/mobile/wt-ingest` trên `<RUDI_DEV_HOST>`, qua chủ hệ thống.
