# ADR-0025 — Thước phim là một file MP4 do máy chủ dựng từ ảnh nhóm; mô hình vẫn chỉ nhận metadata; `/reel` là một lệnh chat; «Khoảnh khắc của tôi» đọc kỷ niệm mình đã đăng

- **Trạng thái:** 🟡 **ĐỀ XUẤT** 2026-09-06 — chờ Lead đánh ĐÃ CHẤP NHẬN. Lát L8 của nhánh `claude/p0-w-m15-social-v1-1` không merge trước khi dòng này đổi. Lead đã duyệt kế hoạch chứa các quyết định này trong phiên 2026-09-06.
- **Quyết định bởi:** Lead (phiên 2026-09-06; ghi lại ở mục 2).
- **Hiện thực:** nhánh `claude/p0-w-m15-social-v1-1`, lát L8 (sau khi L7 đã dựng lại dev client với `expo-video`, `expo-sharing`); kế hoạch `~/.claude/plans/mellow-waddling-lantern.md` mục 5.9.
- **Thêm một bảng, một phụ thuộc Python, một font vào image API, một họ route và một lệnh chat**; không đổi ADR-0011/F37 về việc mô hình không nhận byte ảnh; không đổi ba luật tiền.

## 1. Bối cảnh

F37 «AI Highlight Reel» (M6, #~550) hiện là: mô hình chọn tối đa 6 kỷ niệm **theo metadata** (id, chú thích, địa điểm, thời gian, số tương tác — không bao giờ có byte hay URL ảnh, `reel_gemini.py`), máy chủ gắn lại mọi dữ kiện từ hàng của mình (`app/domain/reel.py`), client hiển thị danh sách pick. Không có video, không xuất được, không chia sẻ được, không gọi được từ chat. `product/feature_list.md` F37 nói «30-second memory video».

Dockerfile của API là `python:3.12-slim` chỉ `COPY app` và `alembic.ini`, không có ffmpeg, không có font; build context là `services/api` nên font đang pin ở `apps/mobile/assets/fonts/` không vào image. Máy chủ không có việc nền nào ngoài cửa `AfterResponse` mở ở ADR-0024.

Backend do Claude làm theo uỷ quyền ADR-0016 §2.3; charter không đổi.

## 2. Quyết định

### 2.1 Máy chủ dựng MP4, ảnh không rời máy chủ
1. ffmpeg đến từ gói pip `imageio-ffmpeg` (binary tĩnh, pin trong `pyproject.toml` và `requirements-dev.txt`), không `apt-get` trong Dockerfile. Chữ (tiêu đề, chú thích, tên địa điểm, tấm kết «Rủ Đi») **vẽ bằng Pillow** (đã pin) với font `BricolageGrotesque-Bold.ttf` chép vào `services/api/app/media/fonts/` (pin thứ hai trong allowlist, cùng sha256 với bản mobile, kèm OFL) — không dùng `drawtext` để không phụ thuộc freetype của binary tĩnh.
2. Kịch bản dựng cố định: ≤ 6 ảnh × 5 giây, 1280×720, zoompan nhẹ, tấm tiêu đề đầu và tấm kết, `libx264 -profile:v baseline -pix_fmt yuv420p -r 24 -movflags +faststart` để emulator và máy thật giải mã được và phát được khi tải dần. Ảnh được `ImageOps.fit` vào khung; không ảnh nào bị kéo méo.
3. **Mô hình vẫn chỉ chọn cảnh bằng metadata** như F37; byte ảnh chỉ đi từ `PhotoStorage` vào ffmpeg trên cùng máy. Không có dịch vụ dựng ngoài.

### 2.2 Dựng là việc sau-response, có trạng thái
1. Bảng `reel_renders(id, context_id, outing_id | person_id, requested_by_id, status ∈ queued|rendering|ready|failed, title, picks JSONB, storage_key, error_code, created_at, ready_at)`, CHECK «đúng một chủ thể» và `(status='ready') = (storage_key IS NOT NULL)`; **một render đang chạy mỗi chủ thể** bằng partial unique.
2. `POST /contexts/{id}/albums/{outing_id}/reel/render` → 202; render chạy qua `AfterResponse.schedule_reel_render` (cửa duy nhất, ADR-0024 §2.3) với session riêng; `GET …/reel/render` trả trạng thái; `GET …/reel/video` trả **cả file** MP4 (không Range) với `Cache-Control: private`, gate `view_trip_album`; chưa `ready` → 404, `failed` giữ `error_code` là tên kiểu lỗi, không lộ đường dẫn.
3. Picks lấy từ `trip_reel` (mô hình) khi có; mô hình im lặng hoặc bị nhịp → dùng 6 kỷ niệm mới nhất có ảnh (`fallback_picks`), và video nói rõ nguồn «Rủ Đi AI chọn cảnh» hay không trên tấm tiêu đề.
4. Limiter riêng `reel_render_limiter` (6 lần/phút/người) vào roster `_KNOWN_DOORS` vì đây là cửa tốn CPU.

### 2.3 `/reel` là lệnh chat, thẻ là của máy chủ
`chat_intent.COMMANDS` thêm `/reel` → kèo gần nhất của nhóm → `trip_reel` → thẻ `ai_card` kind **`reel`** `{outing_id, title, picks[{memory_id, image_url, caption, note}]}` do máy chủ tác giả (`author_id = NULL`); `ground_card` vẫn từ chối kind `reel` từ client như đã từ chối `poll`. Không có kèo, không có ảnh, mô hình không sẵn → `intent_error` tương ứng, tin của người dùng vẫn được lưu.

### 2.4 «Khoảnh khắc của tôi» và thước phim cá nhân
`GET /people/me/memories` trả kỷ niệm **tôi đã đăng** ở những nhóm **tôi còn là thành viên** (rời nhóm là mất quyền đọc ảnh nhóm — nhất quán với gate ảnh); `POST/GET /people/me/reel/render|video` dựng từ 6 ảnh mới nhất của tôi, **không gọi mô hình**. Ảnh gốc mở bằng album của nhóm đó (route có sẵn).

### 2.5 Chia sẻ ra ngoài là hành động của thành viên
Client tải MP4 về cache với header phiên rồi phát bằng `expo-video` và chia sẻ bằng `expo-sharing` (share sheet của hệ điều hành). Video chứa ảnh của nhóm; đưa nó ra ngoài là quyết định của người bấm «Chia sẻ», đúng như chụp màn hình. Máy chủ không có route công khai cho video.

## 3. Hệ quả

- Migration `2d8b4f0e6c19`; `imageio-ffmpeg` thêm ~30 MB vào image (cài trong stage build, Dockerfile không đổi dòng); `gate.sh docker` thêm bước import `imageio_ffmpeg` trong image để bảo đảm binary thật sự có.
- Hai route bytes mới khai vào hai bảng của tier test; client dựng đường dẫn video bằng literal (`duongVideoNhom`) nên không cần ghim `cong-mu`.
- `tests/media/test_reel_render.py` skip khi thiếu binary trừ khi `MOBILE_REQUIRE_FFMPEG_TESTS=1` (skip không phải xanh — cùng luật với tier Postgres).
- `MOBILE_MEDIA_ROOT` tích luỹ MP4 (≈ 5–10 MB mỗi render); dọn bằng script vận hành cùng họ với `purge_expired_stories.py`, không tự động.

## 4. Cái này KHÔNG chứng minh

- Video «đẹp» hay nhạc nền — v1 không có nhạc (bản quyền), không có chuyển cảnh phức tạp.
- Chia sẻ ra app ngoài trên emulator: chỉ thấy chooser hệ thống; không có app đích để nhận.
- Video có **thật sự chạy** trên màn: Maestro chỉ đo chữ; bằng chứng là hai ảnh chụp ở giây 3 và 8 khác nhau hoặc `screenrecord`.
- Hiệu năng render trên máy chủ thật; binary `imageio-ffmpeg` có `drawtext` hay không (không dùng nên không cần).

## 5. Phương án đã bác

| Phương án | Vì sao không |
|---|---|
| Dựng video trên điện thoại | RN không có ffmpeg ổn định; ảnh vẫn phải tải hết về máy; mỗi máy dựng một bản khác |
| Dịch vụ dựng bên ngoài | Ảnh nhóm rời máy chủ — trái nguyên tắc F37/ADR-0011 |
| Render đồng bộ trong request | 6 ảnh × 5 giây có thể quá 30 giây; timeout HTTP và worker bị chiếm |
| `apt-get install ffmpeg` trong Dockerfile | Lớp image lớn, không pin được bản như gói pip; `imageio-ffmpeg` pin đúng bản |
| `drawtext` với font hệ thống trong image | Image slim không có font có dấu Việt; binary tĩnh không chắc có freetype |
| Route video công khai có token | Bề mặt công khai mới cho ảnh nhóm; share sheet của hệ điều hành đủ |
| Cho phép Range request | Phức tạp hơn cần thiết ở 5–10 MB; client tải cả file về cache rồi phát |

## 6. Cách kiểm chứng

- Domain: `reel_card`, `fallback_picks`, `plan_slides` (≤ 6 cảnh, ≤ 80 ký tự chú thích, tổng giây).
- Media: `render_reel` với ảnh PIL sinh trong test (không byte ảnh vào Git) → file > 0 byte, `ftyp` ở byte 4–8, `moov` trước `mdat`, thời lượng qua `imageio_ffmpeg.read_frames` meta ≈ 32–36 giây; ảnh hỏng → `failed`, không 500, `error_code` không chứa đường dẫn.
- API (fake): 202 rồi trạng thái; render đang chạy → cùng id; người ngoài nhóm 403 trước 404; video chưa `ready` → 404; client gửi `card.kind = reel` → 422; `/reel` trong nhóm không có kèo → `intent_error`.
- Postgres: CHECK một chủ thể / `ready_has_file`; unique render đang chạy; `list_person_memories` loại nhóm đã rời.
- Docker: `docker run --rm --entrypoint python <image> -c "import imageio_ffmpeg,subprocess;subprocess.run([imageio_ffmpeg.get_ffmpeg_exe(),'-version'],check=True)"`.
- Emulator: flow 49 (`/reel` → thẻ; «Dựng video» → «Video đã sẵn» → màn phát + «Chia sẻ»; «Khoảnh khắc của tôi») + `kiem_may_chu_sau_49` (trạng thái `ready`, `Content-Type: video/mp4`, `ftyp`; canary: người ngoài nhóm tải video → 404).
- Đột biến phải đỏ: bỏ gate `view_trip_album` ở route video → canary đỏ; cho `ground_card` nhận `reel` → ca 422 đỏ; bỏ partial unique → ca hai render song song đỏ.
