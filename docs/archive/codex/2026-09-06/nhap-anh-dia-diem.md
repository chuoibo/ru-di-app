# Nhập ảnh địa điểm có giấy phép từ Wikimedia Commons

CLI `python3 -m app.places.import_photos` nhận mapping do người vận hành duyệt
trực tiếp giữa `place_id` đã tồn tại và một tiêu đề `File:` của Commons.
Không có tìm ảnh tự động, suy đoán theo tên hay ghép địa điểm gần nhất. Việc đánh
dấu đã duyệt là lời xác nhận của người vận hành; chương trình không thể xác minh
người đó đã xem ảnh hoặc ảnh thể hiện đúng địa điểm.

## Chuẩn bị ngoài Git

Mapping thật phải được tạo trong vùng lưu trữ do người vận hành kiểm soát, ngoài
mọi repository/worktree. Không đưa mapping, ảnh tải về hoặc bản metadata thật vào
Git, log CI hay ticket. CLI từ chối mapping và vùng ảnh nếu đường dẫn đã giải
symlink có tổ tiên chứa `.git`. Kiểm tra này không phát hiện được bind mount hay
thư mục đồng bộ; người vận hành vẫn phải kiểm tra những điều đó.

Schema dưới đây chỉ minh họa bằng dữ liệu tổng hợp, không phải mapping để nhập:

```json
{
  "human_reviewed": true,
  "mappings": [
    {
      "place_id": "p-synthetic",
      "file_title": "File:Synthetic courtyard.jpg"
    }
  ]
}
```

Không ghi tên người duyệt hoặc thông tin người tham gia vào mapping. CLI từ chối
field thừa, key JSON lặp, mapping trùng, nhiều tiêu đề trong một field, đường dẫn
tải tùy ý và hơn 50 mục. File mapping tối đa 256 KiB, chỉ nhận file thường.

Thiết lập `MOBILE_DATABASE_URL` và `MOBILE_MEDIA_ROOT` bằng môi trường vận hành;
không tạo `.env` thật trong worktree. Database phải đã migrate đến phiên bản có
bảng ảnh địa điểm và có các `place_id` tương ứng. Ảnh công khai được mã hóa lại,
bỏ metadata nhúng và lưu dưới `MOBILE_MEDIA_ROOT/licensed-places/`, tách khỏi ảnh
riêng của nhóm.

## Chạy thử và áp dụng

Từ `services/api`, thay đường dẫn minh họa bằng mapping bên ngoài Git đã duyệt:

```bash
python3 -m app.places.import_photos --mapping /srv/rudi-media-reviewed/approved.json
python3 -m app.places.import_photos --mapping /srv/rudi-media-reviewed/approved.json --apply
```

Lệnh đầu mặc định `dry-run`; cũng có thể ghi rõ `--dry-run`. Nó đọc database,
tải metadata và ảnh rồi kiểm tra giải mã trong RAM; không ghi database hoặc file
ảnh. Dry-run vẫn gọi mạng, không phải chế độ offline. Lệnh thứ hai ghi và commit
từng ảnh hợp lệ. Một mục lỗi không hủy các mục đã commit; chạy lại bỏ qua cặp
địa điểm/nguồn đã có. Hàm nền khóa địa điểm và unique constraint bảo vệ thao tác
nhập đồng thời. Nếu commit lỗi sau khi ghi file, có thể còn file đã lọc không
được tham chiếu; importer không tự xóa file.

Stdout chỉ gồm tổng số `checked`, `ready`, `stored`, `existing`, `rejected` và
chế độ chạy. Lỗi thiết lập chỉ in `{"status": "failed"}`. Không in tiêu đề File,
địa điểm, tác giả, path, raw response hoặc traceback. Exit code là 1 nếu có mục
bị từ chối/lỗi, 2 nếu tham số sai, 0 nếu hoàn tất không lỗi. Vì có thể đã commit
một phần trước exit code 1, đối chiếu tổng số trước khi chạy lại.

## Giới hạn nguồn và metadata

Metadata chỉ tải qua HTTPS từ `commons.wikimedia.org/w/api.php`; ảnh gốc JPEG/PNG
chỉ tải qua HTTPS từ `upload.wikimedia.org/wikipedia/commons/<hash>/<hash>/...`.
CLI không tải URL giấy phép hay link trong tên tác giả. Nó kiểm tra lại host,
scheme, path ở từng redirect, giới hạn ba redirect, không kế thừa proxy môi
trường, không nhận query/fragment/credentials/port riêng trên URL ảnh.

Mỗi lần tải có deadline 30 giây và timeout socket tối đa 10 giây; lần đọc đang
chờ có thể kéo dài quá deadline tối đa khoảng 10 giây. Không retry tự động.
Metadata tối đa 256 KiB; ảnh tối đa 10 MiB và 50 triệu pixel, kiểm tra lại bằng
decoder thật khi chạy thử hoặc hàm lưu nền khi apply. Không nhận response nén
HTTP, SVG, PDF hoặc ảnh vượt giới hạn. Các giới hạn mạng không phải cam kết thời
gian hoàn tất database/giải mã của cả batch.

Chỉ nhận `CC0-1.0`, `CC-BY-3.0`, `CC-BY-4.0`, `CC-BY-SA-3.0`, `CC-BY-SA-4.0`.
Tên giấy phép ngắn phải khớp URL Creative Commons được liệt kê cụ thể. Không
suy luận giấy phép từ `licensefree`, `Copyrighted` hay nhãn “Public domain”.
Tác giả được chuyển HTML thành text với tập tag nhỏ; không lưu link/attribute.
Metadata mơ hồ, tác giả trống, `NonFree`, hoặc `Attribution` tùy chỉnh đều bị từ
chối để người vận hành chọn ảnh khác phù hợp hợp đồng hiện tại.

Tham số truy vấn tuân theo [MediaWiki Imageinfo](https://www.mediawiki.org/wiki/API:Imageinfo).
[CommonsMetadata](https://www.mediawiki.org/wiki/Extension:CommonsMetadata) mô tả
`Artist` có thể chứa HTML, `Attribution` có thể thay thế ghi công thông thường,
và metadata giấy phép nhiều lựa chọn có thể không đáng tin cậy. Vì vậy duyệt
mapping trực tiếp vẫn là điều kiện vận hành; metadata máy đọc không chứng minh
ảnh đúng địa điểm hay mọi yêu cầu của chủ ảnh đã được thực hiện.

## Bằng chứng kiểm tra

```bash
python3 -m pytest services/api/tests/places/test_import_photos.py -q
ruff check services/api/app/places/import_photos.py services/api/tests/places/test_import_photos.py
ruff format --check services/api/app/places/import_photos.py services/api/tests/places/test_import_photos.py
```

Các ca kiểm tra dùng metadata tổng hợp, HTTP transport giả lập và ảnh sinh trong
RAM. Chúng kiểm tra giới hạn đầu vào/tải, URL và redirect nguy hiểm, giấy phép,
tác giả HTML, lỗi được che, chạy thử không ghi và orchestration commit. Chúng
không chứng minh nhập Commons trực tiếp, nội dung ảnh thật, điều khoản thực tế
của từng ảnh, hay SQL/PostgreSQL. Bằng chứng persistence thuộc suite nền
`tests/postgres/test_place_photos_postgres.py`. Công việc này không chạy nhập ảnh
thật và không tạo mapping địa điểm thật.
