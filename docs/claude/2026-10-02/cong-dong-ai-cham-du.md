# Cộng đồng: AI chấm đủ mọi ảnh và video — 2026-10-02

- Gốc: `main` @ `f808dbbe`; ghi chép này đi cùng commit của nó trên nhánh
  `claude/congdong-ai-cham-du-anh-video`.
- protocol_version: không đổi (không đụng `docs/protocol/`).
- Verdict: không có reviewer; cổng bằng chứng ở cuối.

## Vì sao

Chủ sản phẩm hỏi vì sao ngưỡng tự duyệt là «900». Đó là 900 phần nghìn (`confidence_milli`
0..1000), tức 90%. Chủ sản phẩm chốt: **giữ 90%, và AI phải đánh giá**, không thêm cờ «duyệt
tay toàn bộ».

Đối chiếu code thì AI chưa chấm hết, có hai lỗ, và cả hai đều đẩy bài sang duyệt tay mà không hỏi AI:

1. **Ảnh.** Một lần đọc gửi tối đa 14 MiB ảnh (thân request agy ≤ 20 MB). Ảnh lưu tới 12 MiB/ảnh,
   một bài 10 ảnh, nhật ký chia sẻ 40 ảnh. Vượt 14 MiB thì không gửi ảnh nào, chỉ gửi chữ,
   nên `media_checked=false` và bài vào duyệt tay. Bài E2E dưới đây có 10 ảnh lưu tổng **36,4 MiB**.
2. **Video.** Video không bao giờ được gửi cho model.

## Đã làm

- `aiharness/congdong/anh.go`: thu nhỏ từng ảnh bằng box filter đọc từng hàng (bộ nhớ chỉ tỉ lệ với
  chiều rộng). Ảnh có nền trong suốt được đặt lên nền trắng rồi mã hoá JPEG q80. Chọn cạnh dài lớn
  nhất trong 1536 → 1024 → 768 → 512 sao cho cả bài vừa 14 MiB. Ảnh gốc giải mã một lần; các nấc
  nhỏ hơn cắt từ bản 1536.
- `media.go` (`community-media-worker`, có ffmpeg): sau khi transcode, cắt **bản cắt duyệt**:
  - mỗi đoạn 45 s (`congdong.GiayDoan`), 1 khung/s, cao ≤ 360 px, tiếng mono 16 kHz;
  - mỗi đoạn một lần encode riêng: muxer `segment` của ffmpeg cắt đoạn đầu dài 97 s thay vì 45 s;
  - key lưu ở `community_media.review_keys` (migration cộng đồng số 4);
  - trigger dọn file đưa cả các đoạn này vào `community_media_gc`.
- `congdong.CacYeuCau`/`Duyet` sinh các yêu cầu đọc như sau:
  - một yêu cầu gồm chữ và mọi ảnh;
  - mỗi đoạn video thêm một yêu cầu riêng, gồm chữ cùng `{"video":{"piece":i,"pieces":n,"starts_at_second":s}}`.

  Kết quả gộp lại:
  - liên quan nếu một lần đọc nói có;
  - an toàn chỉ khi mọi lần đọc nói có;
  - độ tự tin lấy mức thấp nhất;
  - lý do lấy từ lần đọc không an toàn đầu tiên, nếu không có thì từ lần đọc kém tự tin nhất.

  Hễ có một tệp không gửi được thì vẫn giữ luật cũ: chỉ gửi chữ, và bài vào duyệt tay. Một lời gọi hỏng
  thì cả lần đọc hỏng, job chờ rồi đọc lại.
- Thời hạn: mỗi lời gọi 85 s (client agy cắt ở 90 s), lease job 8 phút (2 phút trước đây). Nếp vẫn
  45 s vì người dùng đang chờ.
- `duyet.txt`: dặn model khi dữ liệu có trường `video` thì đó là một đoạn, chạy 1 khung/s kèm tiếng.
  Model phải xét cả hình, chữ trên màn hình và lời nói; độ liên quan xét trên toàn bài.

## Số đo agy thật (2026-10-02, qua LAN)

| Request | Kết quả |
|---|---|
| Video 20 s nguyên khối | 31,8 s, 1.888 token; chỉ đúng ô đỏ giây 12–13 |
| Video 180 s nguyên khối, lần 1 | 170,7 s, 16.439 token; câu hỏi gợi sẵn có ô đỏ, model trả sai giây (30 thay 150) |
| Video 180 s nguyên khối, lần 2 (không gợi ý) | **không trả lời trong 400 s** |
| Hai đoạn 45 s gửi song song, không gợi ý | 29,5 s và 30,6 s, mỗi đoạn 4.161 token; tự thấy ô xanh ở giây 15–17 (thật: 60–62) và ô đỏ ở giây 15–17 (thật: 150–152) |

E2E trên stack local: nhánh core (serve + media worker), Postgres dùng một lần, agy thật. Một người
được tạo bằng genesis; mọi ảnh và video đều nhân tạo.

| Ca | Kết quả | Thời gian đọc |
|---|---|---|
| 10 ảnh 4000×3000 có hạt nhiễu, lưu 36,4 MiB | `approved` | 24 s |
| Video trình chiếu 90 s, sạch (2 đoạn) | `approved` | 28 s |
| Cùng video, chèn quảng cáo vay tiền kèm một số điện thoại giả ở giây 50–62 | `rejected unsafe_content` | 12 s |

Chạy riêng từng đoạn của video quảng cáo qua `congdong.Duyet`:
- đoạn 1: an toàn, 980‰;
- đoạn 2: **không an toàn**, «illegal loan advertising», 980‰.

Vậy kết quả gộp đúng là nhờ đọc tới đoạn thứ hai.

## Cổng đã chạy

- `go test ./internal/aiharness/congdong/`.
- `scripts/go_postgres_tier.sh -tags 'postgres communitymedia' ./internal/community/ ./internal/db/`:
  `VideoReviewCut` cắt video 50 s ra hai đoạn, 45 và 5 khung, cao 360 px. Số đo của cả cổng nằm trong
  commit message.
- Đột biến, mỗi cái đỏ ở đúng chỗ đã dự đoán:
  - **M1** (video không có bản cắt vẫn coi là gửi được): đỏ ở `TestDuyetDocVideoTungDoan`, ca «unsendable video».
  - **M2** (gộp an toàn bằng OR): đỏ ở «joined reading».
  - **M3** (bỏ thang thu nhỏ): đỏ ở «12 photographs», `MediaChecked=false`.
  - **M4** (trigger dọn file bỏ sót `review_keys`): đỏ ở «review pieces left for the sweep: 0».
  - **M5** (cắt mà không `fps=1`): đỏ ở «frames [1125 125]».

## Còn mở

- ~~Test cắt video thật chưa nằm trong cổng nào.~~ Commit sau đã thêm chặng `go-media` vào
  `scripts/gate.sh` và một bước tương ứng vào job `core`.
- Video xử lý trước migration 4 không có bản cắt nên vẫn vào duyệt tay. Đã kiểm 2026-10-02: DB vnlocal
  không có bảng cộng đồng, không compose nào bật cờ, nên hiện không có video nào như vậy và chưa cần
  lệnh cắt bù.
- Video 180 s cần 4 lời gọi nối tiếp. Lúc agy bận, mỗi lời gọi có thể tới 85 s.
- Chất lượng phán đoán vẫn mới chỉ thử bằng bài bịa (`hang-doi.md` mục ADR-0052 số 3).
