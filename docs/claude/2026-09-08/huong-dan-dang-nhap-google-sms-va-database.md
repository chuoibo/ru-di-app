# Hướng dẫn: đăng nhập Google + SMS OTP và phần database — đối chiếu với cách làm hiện đại

Ngày đo: 2026-09-08 · `main` = a03f563d · người viết: Claude · trạng thái: **tài liệu hướng dẫn, chưa đổi code**.

Đọc trước: ADR-0014 (phiên thay header actor), ADR-0016 (OTP + Google), ADR-0023 (vòng đời tài khoản), `.env.example`, `docs/CHAY-DEMO.md`.

## 0. Đọc trong hai phút

**Điều dễ hiểu nhầm nhất:** OTP và Google **đã được viết xong ở máy chủ và ở app** từ 2026-09-04 (PR #529, #530, #531). Cái còn thiếu không phải code, mà là **ba thứ chỉ leader mới làm được**, và chúng nằm ngoài repo:

| # | Thiếu | Ai | Hậu quả khi chưa có |
|---|---|---|---|
| 1 | Google OAuth client (Android + Web) trên Google Cloud Console | Leader | `POST /auth/google` trả 503 `google_not_configured`; nút Google **ẩn** trên app |
| 2 | Nhà cung cấp SMS + brandname, và một **subclass** sender cho vendor đó | Leader chọn · Claude nối | Ngoài stack demo (mã debug `000000`) không ai nhận được mã |
| 3 | Nơi chạy Postgres + API cho người thật (B3 trong `01-duong-toi-production.md`) | Leader | Mọi thứ ở trên chỉ chạy trên máy dev/emulator |

Ba quyết định cần leader chốt trong tài liệu này: **§3.3** (vendor SMS), **§4.5** (giữ thư viện Google legacy hay đổi sang Credential Manager), **§7** (thứ tự làm).

## 1. Bức tranh tổng: hai cửa, một bảng phiên

```
  [App RuDi]                         [API prod]                      [Postgres]
      │  POST /auth/otp/request {phone}  │                                │
      ├─────────────────────────────────►│ canonical → HMAC digest        │
      │                                  │ kiểm 60s/5 lần/15'             │
      │                                  │ sinh mã 6 số (hoặc debug)      │
      │                                  ├── INSERT otp_challenges ──────►│  (digest số + digest mã, KHÔNG số, KHÔNG mã)
      │                                  │ SmsSender.send_otp(số, mã)     │  → log | gateway thật
      │◄── 202 {challenge_id, 300s, 60s}─┤                                │
      │  POST /auth/otp/verify           │                                │
      │  {challenge_id, phone, code}     │                                │
      ├─────────────────────────────────►│ so digest, đếm attempts        │
      │                                  ├── UPDATE otp_challenges ──────►│  consumed_at
      │                                  ├── account_identities(phone) ──►│  tìm/ghi người
      │                                  ├── INSERT account_sessions ────►│  digest token, 30 ngày, issued_via='otp'
      │◄── 201 SessionResponse{token…} ──┤                                │
      │                                  │                                │
      │  GoogleSignin.signIn() ─► Google trả id_token (JWT, aud = Web client id)
      │  POST /auth/google {id_token}    │                                │
      ├─────────────────────────────────►│ verify chữ ký Google, iss, exp │
      │                                  │ aud ∈ MOBILE_GOOGLE_CLIENT_IDS │
      │                                  ├── account_identities(google) ─►│  sub lần đầu ⇒ people mới (uuid4)
      │                                  ├── INSERT account_sessions ────►│  issued_via='google'
      │◄── 201 SessionResponse ──────────┤                                │
      │  Authorization: Bearer <token> cho mọi route sau đó (ADR-0014)
```

Hai cửa gặp nhau ở **một chỗ duy nhất**: `account_sessions`. Mọi route khác chỉ biết «phiên này là người nào», không biết người đó vào bằng cửa gì trừ cột `issued_via` để audit.

File tương ứng: `services/api/app/api/routes/auth.py` (ba route + limiter theo IP) · `app/api/service.py` từ dòng ~3596 (`request_otp`, `verify_otp`, `login_with_google`) · `app/domain/otp.py` (luật thuần: TTL, attempts, cooldown) · `app/api/sms.py` (SmsSender) · `app/api/google_identity.py` (verifier) · `app/api/person_identity.py` (HMAC số điện thoại) · client: `apps/mobile/src/phien.ts` (`guiOtp`, `xacMinhOtp`, `dangNhapGoogle`, `khoiPhucPhien`, `dangXuat`), `src/rudi/screens/auth/Login.tsx`, `Otp.tsx`, `src/rudi/google.ts`.

## 2. Database — «clear» phần này

### 2.1 Bốn bảng liên quan tới danh tính

| Bảng | Mỗi hàng là | Cột đáng nhớ | Cái KHÔNG bao giờ nằm trong bảng |
|---|---|---|---|
| `people` | một con người | `id` (UUID), `display_name`, `deleted_at` | số điện thoại, email |
| `account_identities` | một **bằng chứng ngoài** trỏ về một người | `provider ∈ {phone, google}`, `subject`, `person_id`, `last_login_at`; **unique (provider, subject)** | số điện thoại thô, email Google |
| `otp_challenges` | một mã đã phát | `phone_digest` (32 byte), `code_digest` (32 byte), `expires_at`, `attempts`, `consumed_at` | số, mã |
| `account_sessions` | một token đang sống | `token_digest` (SHA-256, unique), `expires_at`, `revoked_at`, `issued_via ∈ {invite, otp, google, genesis}` | token thô |

`subject` của `provider='phone'` là **hex của HMAC-SHA256(số chuẩn hoá)** dưới khoá `MOBILE_PERSON_ID_KEY`; của `provider='google'` là `sub` Google (chuỗi số ổn định, không phải email).

### 2.2 `people.id` sinh ra thế nào — và vì sao khoá là chuyện lớn

- **Cửa OTP**: người mới ⇒ `id = HMAC(số) → UUID v8` (`derive_person_id`). Lý do: một thành viên có thể đặt tên bạn mình bằng số điện thoại **trước** khi bạn đó cài app (`PUT /people/{id}`); khi bạn đó đăng nhập OTP lần đầu, id trùng khớp và «tìm lại» đúng hàng đó — tiền đã ghi cho họ không mất.
- **Cửa Google**: người mới ⇒ `uuid4`. Không có gì để «tìm lại» — ADR-0016 **cấm** merge theo email.
- **Người đã xoá tài khoản** (ADR-0023): số cũ quay lại ⇒ `uuid4` mới, không hồi sinh hàng đã ẩn danh hoá.
- **Khoá `MOBILE_PERSON_ID_KEY`** là thứ đứng giữa một `person_id` và một số điện thoại thật (`.env.example` giải thích con số 257.316 ứng viên/giây nếu không có khoá). **Đổi khoá = mọi người đăng nhập vào tài khoản trống mới** — là một cuộc di trú tài khoản, không phải sửa config. Sinh bằng `python3 -c "import secrets; print(secrets.token_urlsafe(48))"`, ≥ 32 ký tự, **một khoá cho production, giữ ở secret manager, sao lưu như sao lưu database**. Mất khoá = mất khả năng nhận diện người cũ qua số điện thoại (họ vẫn vào được nếu còn hàng trong `account_identities`, nhưng người mới đặt tên bằng số sẽ không khớp nữa).

### 2.3 Ba môi trường database đang tồn tại

| Stack | Ai dựng | `MOBILE_AUTH_MODE` | SMS | Dùng cho |
|---|---|---|---|---|
| `docker compose up` (cổng 8099, project `mobile-local`, volume chung cả máy) | `make up` | `dev` (tin `X-Actor-*`) | log | seed cũ, trang khách |
| `scripts/e2e_slice.sh --keep` (cổng ngẫu nhiên, container Postgres dùng-một-lần) | harness | vắng ⇒ `prod` | log + `MOBILE_OTP_DEBUG_CODE=000000` + `MOBILE_OTP_LOG_CODES=1` | Maestro `--otp`, `make demo-rudi` |
| **Production** | **chưa có** | `prod` | gateway thật, **không** debug code | người thật |

Xem dữ liệu trên stack compose:

```bash
docker compose exec postgres psql -U mobile -d mobile -c \
  "select provider, count(*) from account_identities group by 1;"
docker compose exec postgres psql -U mobile -d mobile -c \
  "select issued_via, count(*) filter (where revoked_at is null and expires_at > now()) as song from account_sessions group by 1;"
docker compose exec postgres psql -U mobile -d mobile -c \
  "select count(*), count(*) filter (where consumed_at is null and expires_at > now()) as con_hieu_luc from otp_challenges;"
```

Xoá sạch (chỉ máy dev, xoá volume của **mọi** worktree trên máy): `make clean CONFIRM=mobile-local`.

### 2.4 Nợ database đã thấy khi đọc

- **Không có job dọn** `otp_challenges` hết hạn và `account_sessions` hết hạn/đã thu hồi. Hôm nay là vài trăm hàng; với người thật là hàng triệu hàng digest vô dụng. Cách hiện đại: một lệnh `purge` idempotent chạy theo lịch (repo đã có mẫu `scripts/purge_expired_stories.py`), giữ hàng `consumed` thêm N ngày để audit rồi xoá.
- Cổng Luật 1 tầng Postgres **đỏ sẵn** trên main (memory: `cong-tien-tang-postgres-do-san-tren-main`), không liên quan auth nhưng sẽ hiện ra khi chạy `make test-db` cho việc này.

### 2.5 Cách các hệ hiện đại lưu — so với ta

| | Firebase / Supabase Auth (bị bác ở ADR-0016) | Ta |
|---|---|---|
| Số điện thoại | Lưu **rõ** trong bảng users của họ | Chỉ HMAC digest; số không có ở đâu |
| Email Google | Lưu rõ, thường dùng để **auto-link** tài khoản | Không tới tầng service; không link |
| Token phiên | JWT ký + refresh token | Token ngẫu nhiên, lưu digest, tra DB mỗi request |
| Xoá tài khoản | Xoá user record | Ẩn danh hoá + số quay lại thành người mới |

Ta **chặt hơn** về dữ liệu cá nhân, đúng luật «không cột, không cache, không file» của repo. Giá phải trả: không có «quên số, tìm theo email» — đó là quyết định, không phải thiếu sót.

## 3. Cửa SMS OTP

### 3.1 Luồng chi tiết và từng mã lỗi

`POST /auth/otp/request {phone}` → **202** `{challenge_id, expires_in_seconds: 300, resend_after_seconds: 60}`

| Tình huống | Mã | `code` |
|---|---|---|
| thiếu/không phải chuỗi | 422 | `phone_required` |
| không phải số di động VN (`canonical_mobile`: `+84`/`84`/`0` + 9 số) | 422 | `phone_not_mobile` |
| máy chủ thiếu `MOBILE_PERSON_ID_KEY` | 503 | `identity_key_missing` |
| gửi lại trong 60 s | 429 | `otp_resend_too_soon` (kèm giây) |
| quá 5 mã / 15 phút / số | 429 | `otp_too_many_requests` |
| quá 10 yêu cầu / phút / IP | 429 | `rate_limited` |
| gateway không nhận | 503 | `sms_unavailable` (challenge bị đánh consumed) |

`POST /auth/otp/verify {challenge_id, phone, code}` → **201** `SessionResponse`

| Tình huống | Mã | `code` |
|---|---|---|
| challenge không tồn tại / của số khác / hết hạn / đã dùng / đã cháy | **404** | `otp_challenge_not_found` — cố ý một câu cho cả bốn |
| sai mã, còn lượt | 422 | `otp_code_invalid` «Còn N lần thử» |
| sai lần thứ 5 | 429 | `otp_too_many_attempts` (challenge cháy) |
| quá 30 xác minh / phút / IP | 429 | `rate_limited` |

Mã được băm **kèm `challenge_id`** (`derive_code_digest`) nên một bảng tra 10⁶ mã không dùng được.

### 3.2 Đối chiếu với chuẩn hiện đại (OWASP + các nhà cung cấp OTP lớn)

| Kiểm soát | Chuẩn 2026 | Ta hôm nay | Đánh giá |
|---|---|---|---|
| TTL mã | ngắn, một số vendor khuyên 60–90 s; 5 phút vẫn phổ biến | 300 s | Chấp nhận được; hạ xuống 120–180 s sau khi đo SMS VN tới trong bao lâu |
| Mã dùng một lần, huỷ khi đúng | bắt buộc | có (`consumed_at`) | ✅ |
| Giới hạn lần thử | 3–5 | 5, cháy challenge | ✅ |
| Rate limit theo số | có | 60 s + 5/15' | ✅ |
| Rate limit theo IP | có | 10/phút request, 30/phút verify | ✅ nhưng **trong bộ nhớ một tiến trình** — chạy 2 replica là mỗi replica một bộ đếm |
| Chặn prefix quốc tế/premium (chống SMS pumping) | có | `canonical_mobile` **chỉ nhận số VN** | ✅ đây là lá chắn pumping tốt nhất ta có |
| Trần chi tiêu SMS/ngày + cảnh báo | có | **không** | ❌ thêm khi có gateway thật: đếm challenge/ngày, quá trần thì 503 và báo |
| Gắn mã với thiết bị/phiên yêu cầu | khuyến nghị | gắn với `challenge_id` + số, chưa gắn thiết bị | ⚠️ đủ cho v1 |
| CAPTCHA / attestation (Play Integrity) khi nghi ngờ | khuyến nghị khi bị tấn công | không | ⏳ §6 |
| Không log số / mã | bắt buộc | không log số; mã chỉ khi `MOBILE_OTP_LOG_CODES=1` | ✅ |
| Mã debug không lọt production | bắt buộc | fail-closed: có gateway + có debug ⇒ **không khởi động** | ✅ hơn nhiều hệ |

Kết luận: **thiết kế OTP của ta đã ở mức các hệ tốt**; hai lỗ thật là *bộ đếm trong RAM* (chỉ lộ khi > 1 replica) và *không có trần chi tiêu* (chỉ lộ khi có gateway thật). Cả hai làm cùng lúc với việc nối vendor.

### 3.3 Chọn nhà cung cấp cho Việt Nam — quyết định của leader

Bối cảnh riêng của Việt Nam: SMS tới thuê bao VN với tên người gửi (brandname) **phải đăng ký với từng nhà mạng** (Viettel, VinaPhone, MobiFone); không có brandname thì tin bị chặn hoặc đi từ đầu số lạ, tỉ lệ tới thấp. Đăng ký brandname riêng cần hồ sơ doanh nghiệp và mất từ vài ngày (SpeedSMS nói 3–5 ngày làm việc) tới vài tuần.

| Phương án | Ưu | Nhược | Hợp khi |
|---|---|---|---|
| **A. SpeedSMS, brandname dùng chung «Verify»** | Không cần đăng ký nhà mạng; có ngay; API token đơn giản; rẻ | Tin đến từ tên chung, không phải «RuDi»; phụ thuộc một vendor nội | **Bắt đầu ngay tuần này** để người thật thử |
| **B. eSMS.vn — ZNS OTP (Zalo) → rơi về SMS** | Zalo phủ gần hết người dùng VN, rẻ hơn SMS, hiển thị thương hiệu; API đa kênh tự rơi về SMS | Cần Zalo OA + duyệt template; hai kênh = hai chỗ hỏng | Khi đã có công ty/OA và muốn chi phí thấp lâu dài |
| **C. Twilio Verify / Prelude (OTP-as-a-service quốc tế)** | Họ lo cả sinh mã, retry, chống pumping, đa kênh; chuẩn ngành | ~0,05 USD/lần + phí SMS; **vẫn** phải đăng ký sender ID cho VN; **thay** logic OTP của ta bằng của họ (mất `otp_challenges` tự chủ) | Khi mở rộng ngoài VN |

**Khuyến nghị:** A trước (một buổi), giữ kiến trúc «ta sinh mã, vendor chỉ chuyển tin» đúng ADR-0016; đăng ký brandname «RuDi» song song; B là bước kế khi có OA. C không hợp với quyết định backend-owned trừ khi đi quốc tế.

Việc **leader làm** cho A: mở tài khoản, lấy access token, nạp tiền thử, hỏi vendor đúng hình dạng request cho OTP + brandname Verify, và hỏi **giá/tin và có cắt tin khi hết tiền không** (đó là trần chi tiêu tự nhiên đầu tiên).

### 3.4 Code nối vendor — ĐÃ VIẾT XONG 2026-09-08

`HttpJsonSmsSender` cũ gửi `{"to": số, "body": nội dung}` với `Authorization: Bearer`. **Không vendor VN nào nhận hình dạng đó**, và tệ hơn: cả hai vendor trả **HTTP 200 kèm lỗi nằm trong thân**, nên sender cũ đọc một lượt từ chối thành «đã gửi». Đo được trên máy: cùng một gateway từ chối, sender cũ trả **202**, sender mới trả **503**.

Đã có trên nhánh `claude/p0-w-m1-sms-vendor`:

1. `SpeedSmsSender` và `EsmsSender` đè `payload()`, `headers()` và `verdict()`. `verdict()` đọc thân trả về; thân không đọc được cũng là **thất bại**, vì một lượt gửi không xác nhận được không phải là một lượt gửi.
2. `build_sms_sender` chọn theo `MOBILE_SMS_VENDOR` (`generic` · `speedsms` · `esms`); tên lạ ⇒ từ chối khởi động.
3. **URL `http://` bị từ chối** trừ loopback: mã OTP nằm trong thân request, mà chính tài liệu eSMS in endpoint `http://`.
4. `MOBILE_SMS_TYPE` **không có mặc định**; loại tin quyết định tin có tới máy hay không và chỉ vendor mới biết tài khoản này được loại nào. Loại nào cần brandname thì thiếu `MOBILE_SMS_SENDER_NAME` là từ chối khởi động (SpeedSMS 3 và 5; eSMS 2).
5. `scripts/fake_sms_gateway.py` — gateway giả nói đúng giọng từng vendor, có `--refuse <mã>` để trả lỗi kèm HTTP 200. Nhờ nó mà đo được toàn đường **trước khi có tài khoản**.
6. 13 ca mới ở `tests/api/test_sms_vendors.py`, chạy trên HTTP server thật chứ không mock. Bốn đột biến (bỏ `verdict`, đổi Basic thành Bearer, bỏ chặn plaintext, bỏ `SecretKey`) đều bị bắt đúng ca.

**Còn nợ, PR sau:** trần chi tiêu mỗi ngày (đếm `otp_challenges` 24 h, quá `MOBILE_OTP_DAILY_CAP` ⇒ 503 + WARNING) vì cần một câu đếm trong DB và ca Postgres riêng; và bộ đếm rate limit vẫn nằm trong RAM một tiến trình — chỉ thành vấn đề khi chạy nhiều replica.

### 3.4b Đo thử ngay, không cần tài khoản

```bash
python3 scripts/fake_sms_gateway.py --vendor speedsms --port 8123      # cửa sổ 1
# cửa sổ 2: dựng API prod trỏ vào đó (KHÔNG đặt MOBILE_OTP_DEBUG_CODE)
MOBILE_SMS_VENDOR=speedsms MOBILE_SMS_GATEWAY_URL=http://127.0.0.1:8123/send \
MOBILE_SMS_GATEWAY_TOKEN=bat-ky MOBILE_SMS_TYPE=4 MOBILE_SMS_SENDER_NAME=Verify \
  python3 -m uvicorn app.api.main:app --port 8000
curl -X POST localhost:8000/auth/otp/request -H 'Content-Type: application/json' -d '{"phone":"<số của bạn>"}'
```

Cửa sổ 1 in ra đúng thân request đã gửi. Chạy lại gateway với `--refuse 300` thì cùng lời gọi đó phải ra **503**, không phải 202.

### 3.5 Cấu hình từng bước trên host thật

```bash
# 1. Khoá danh tính — sinh MỘT lần, cất ở secret manager
MOBILE_PERSON_ID_KEY=$(python3 -c "import secrets; print(secrets.token_urlsafe(48))")
# 2. Gateway
MOBILE_SMS_VENDOR=speedsms                # hoặc esms
MOBILE_SMS_GATEWAY_URL=https://api.speedsms.vn/index.php/sms/send        # PHẢI https
MOBILE_SMS_GATEWAY_TOKEN=...              # secret; với esms đây là ApiKey
MOBILE_SMS_GATEWAY_SECRET=...             # chỉ esms: SecretKey
MOBILE_SMS_TYPE=4                         # HỎI VENDOR; không có mặc định
MOBILE_SMS_SENDER_NAME=Verify             # brandname; bắt buộc với loại cần nó
MOBILE_SMS_TEMPLATE='Ma Ru Di cua ban: {code}. Ma het han sau 5 phut.'   # phải KHỚP template đã đăng ký với vendor
# 3. Tuyệt đối KHÔNG đặt
# MOBILE_OTP_DEBUG_CODE=   MOBILE_OTP_LOG_CODES=   MOBILE_AUTH_MODE=dev
```

Kiểm sau khi bật (dùng số của chính bạn, không ghi số vào repo):

```bash
curl -s -X POST "$API/auth/otp/request" -H 'Content-Type: application/json' \
  -d '{"phone":"<số của bạn>"}'                       # mong 202 + challenge_id; điện thoại rung
curl -s -X POST "$API/auth/otp/verify" -H 'Content-Type: application/json' \
  -d '{"challenge_id":"<id>","phone":"<số>","code":"<mã nhận được>"}'   # mong 201 + token
```

Đối chứng âm (phải đỏ): gửi lại ngay ⇒ 429; mã sai 5 lần ⇒ 422 ×4 rồi 429; số nước ngoài ⇒ 422.

## 4. Cửa Google

### 4.1 Luồng chi tiết

1. App chỉ hiện nút khi `EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID` có dạng `*.apps.googleusercontent.com` (`googleConfigured`).
2. Bấm ⇒ `GoogleSignin.configure({webClientId})` → `hasPlayServices()` → `signIn()` ⇒ Google trả **ID token** (JWT ~1 giờ) có `aud = Web client id`. Huỷ ⇒ không gọi API.
3. `POST /auth/google {id_token}` ⇒ máy chủ: 503 nếu chưa cấu hình → 422 nếu thiếu → `google-auth` kiểm chữ ký theo chứng chỉ công khai của Google, `iss`, `exp` (lệch giờ 10 s) → `claims_from` kiểm `aud ∈ MOBILE_GOOGLE_CLIENT_IDS`, có `sub` → **401 một câu** cho mọi lý do sai.
4. `sub` lần đầu ⇒ `people` mới với `display_name` từ Google (chỉ tên, không email) + `account_identities(google, sub)` trong một savepoint (thua race không để orphan) ⇒ phiên `issued_via='google'`, `is_new_person=true` ⇒ app đưa vào màn cá nhân hoá.

### 4.2 Google Cloud Console — leader làm, từng bước

Dùng tài khoản Google của tổ chức (không phải cá nhân, để không mất project khi đổi người).

1. **Tạo project** `rudi-prod` tại console.cloud.google.com (menu Project → New Project).
2. **APIs & Services → OAuth consent screen** (giờ nằm trong «Google Auth Platform»):
   - User type: **External**. App name: `Rủ Đi`. Support email + developer contact: email tổ chức.
   - Scopes: **để mặc định** (`openid`, `profile`, `email` là non-sensitive; ta không đọc email nhưng scope này Google vẫn cấp).
   - **Publishing status**: lúc đầu là *Testing* ⇒ **chỉ tài khoản trong danh sách Test users đăng nhập được** (tối đa 100). Thêm tài khoản Google của bạn và người thử vào đó ngay. Khi mở cho công chúng ⇒ bấm **Publish app**; với scope non-sensitive không cần Google thẩm định.
3. **Credentials → Create credentials → OAuth client ID**, tạo **hai** client:
   - **Android**: Name `RuDi Android debug`; Package name `com.lakiet.rudi`; SHA-1 của keystore debug trong repo (`apps/mobile/android/app/debug.keystore`, alias `androiddebugkey`, mật khẩu `android`):
     `5E:8F:16:06:2E:A3:CD:2C:4A:0D:54:78:76:BA:A6:F3:8C:AB:F6:25`
     (lệnh tạo lại: `keytool -list -v -keystore android/app/debug.keystore -alias androiddebugkey -storepass android | grep SHA1`).
     Sau này thêm client Android thứ hai cho **keystore release** và, nếu phát hành qua Play, SHA-1 của **Play App Signing** (Play Console → Setup → App signing). Không có SHA-1 đúng ⇒ `signIn()` ném `DEVELOPER_ERROR`.
   - **Web application**: Name `RuDi Web (id_token audience)`; **không cần** Authorized origins / redirect URIs cho luồng native. Client ID này là **audience** của mọi ID token app nhận được — nó phải có mặt ở **cả** app lẫn máy chủ.
   - (iOS, sau) client **iOS** với Bundle ID `com.lakiet.rudi` ⇒ `EXPO_PUBLIC_GOOGLE_IOS_CLIENT_ID`; `app.config.ts` tự suy `iosUrlScheme`.
4. Ghi lại: Web client ID, Android client ID. **Không có secret nào** cần giữ cho luồng native (client secret của Web client không dùng — đừng chép vào repo).

### 4.3 Đưa client ID vào app và máy chủ

Client ID là cấu hình công khai (nằm trong APK ai cũng đọc được), **không** phải bí mật — nhưng vẫn đi qua biến môi trường, không hard-code.

```bash
# Máy chủ (cả Web lẫn Android id, phân cách dấu phẩy; Web là bắt buộc vì đó là aud)
MOBILE_GOOGLE_CLIENT_IDS=<web-id>.apps.googleusercontent.com,<android-id>.apps.googleusercontent.com

# App: Metro đọc lúc bundle. Dev client đã có module native Google (PR #527), KHÔNG cần dựng lại APK cho Android.
cd apps/mobile && EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID=<web-id>.apps.googleusercontent.com \
  EXPO_PUBLIC_API_URL=http://10.0.2.2:<cổng> npx expo start --dev-client
```

Emulator: AVD `rudi` là ảnh `google_apis` (có Google Play services, `PlayStore.enabled=no`). Google Sign-In cần **một tài khoản Google đã đăng nhập trên máy**: Settings → Passwords & accounts → Add account → Google, dùng tài khoản đã nằm trong Test users. Chưa kiểm thực tế trên ảnh này — nếu `hasPlayServices` báo thiếu thì phải tạo AVD ảnh `google_apis_playstore`.

### 4.4 Kiểm tra theo bậc — mỗi bậc phải đỏ đúng chỗ trước khi lên bậc sau

```bash
# Bậc 0 — chưa cấu hình: 503 (e2e_slice đã có chặng này)
curl -s -o /dev/null -w '%{http_code}\n' -X POST "$API/auth/google" \
  -H 'Content-Type: application/json' -d '{"id_token":"x"}'          # 503
# Bậc 1 — đã đặt MOBILE_GOOGLE_CLIENT_IDS, token rác: 401 (không phải 500)
#   cùng lệnh                                                          # 401
# Bậc 2 — token thật: bấm nút trên emulator ⇒ 201, app vào «Chưa có nhóm nào»,
#   psql: select provider, count(*) from account_identities ⇒ có hàng google
# Bậc 3 — bấm lại cùng tài khoản ⇒ cùng person_id (không sinh người mới), last_login_at đổi
# Bậc 4 — canary: token của project Google KHÁC ⇒ 401 (aud không thuộc ta)
```

Bảng Maestro: thêm flow «Tiếp tục với Google» **không** tự động hoá được (màn chọn tài khoản là UI hệ thống, tài khoản thật) — bằng chứng là ảnh chụp tay + hàng trong DB; ghi rõ vào cột «không chứng minh».

### 4.5 Đối chiếu với chuẩn hiện đại — quyết định thứ hai của leader

**Sự thật khó chịu:** Google đã **deprecate** Google Sign-In legacy cho Android (`play-services-auth`) và nói sẽ **gỡ khỏi SDK ở một bản tương lai**; hướng thay thế là **Android Credential Manager** (một API cho mật khẩu, passkey, Sign in with Google, có `nonce`). Thư viện ta đang dùng, `@react-native-google-signin/google-signin` **bản miễn phí v16**, vẫn đứng trên SDK legacy («deprecated but remains functional»); bản Credential Manager của họ là **trả phí**. Expo hiện liệt kê `react-native-nitro-google-signin` (Credential Manager, miễn phí) bên cạnh thư viện của ta; cộng đồng còn có `@thoughtbot/react-native-social-auth` và `@ademhatay/expo-google-signin` cùng hướng.

| Phương án | Ưu | Nhược |
|---|---|---|
| **Giữ legacy** (code hiện tại) | Không đổi gì; chạy được hôm nay; máy chủ **không** cần đổi (vẫn nhận ID token) | Sống trên API đã khai tử; không có `nonce`; ngày Google gỡ khỏi SDK thì phải đổi gấp |
| **Đổi sang thư viện Credential Manager miễn phí** | Đúng hướng Google; hỗ trợ passkey sau; có `nonce` chống replay | Dựng lại dev client (một lần); thư viện trẻ hơn, ít người dùng hơn; ~1 ngày kể cả đo trên emulator |

**Khuyến nghị:** làm **§4.2–4.4 với code hiện tại trước** (để có bằng chứng «Google vào được» trong tuần), rồi mở một PR riêng đổi thư viện — vì máy chủ nhận **ID token** nên phía máy chủ không đổi, chỉ đổi cách app lấy token. Khi đổi, thêm `nonce`: app sinh nonce ngẫu nhiên, gửi cho Google, máy chủ kiểm `nonce` trong claims khớp với nonce app gửi kèm ⇒ một token bị chép không dùng lại được.

Phần **máy chủ đã đúng chuẩn**: kiểm chữ ký theo chứng chỉ Google, `iss`, `exp`, `aud` là **tập** client id của ta, không nhận email vào tầng service, 401 một câu, 503 khi chưa cấu hình, rate limit theo IP. Điểm cộng nhỏ nên thêm: lưu **digest của id_token đã dùng** cho tới `exp` của nó, để một token bị chép không đổi được hai phiên (cửa replay ~1 giờ); chỉ cần khi chưa có `nonce`.

## 5. Phiên — đã hiện đại ở mức nào

Token 32 byte ngẫu nhiên (`secrets.token_urlsafe`), lưu **SHA-256** (DB bị lộ ⇒ kẻ cắp có digest, không có token), 30 ngày, thu hồi bằng `revoked_at`, `DELETE /sessions/current` khi đăng xuất, client giữ trong SecureStore. Đây là mô hình «opaque token + tra DB» mà OWASP khuyên cho app first-party; **không** cần JWT (JWT không thu hồi được tức thì, và ta có DB ở mỗi request rồi).

Nên thêm sau, theo thứ tự: (1) **gia hạn trượt** — phiên dùng thường xuyên không đứt sau đúng 30 ngày; (2) **danh sách thiết bị + «đăng xuất mọi nơi»** — đã có `revoked_at`, chỉ thiếu route và màn; (3) **xoay token** khi đổi số/đăng nhập lại. Refresh token tách riêng chỉ cần khi có bên thứ ba cầm access token.

## 6. Chống lạm dụng — thứ tự đúng

1. Có gateway thật ⇒ ngay lập tức **trần chi tiêu/ngày** + cảnh báo (rẻ, chặn được vụ đốt tiền qua đêm).
2. Đo 2–4 tuần: log số challenge/ngày, tỉ lệ verify thành công, IP top.
3. Chỉ khi thấy tấn công ⇒ Play Integrity/App Attest cho **hai route** `/auth/otp/request` và `/auth/google` (theo lộ trình «monitor → warn → enforce»), rồi CAPTCHA làm lối thoát cho máy không có Play services.

Không làm 3 trước 1–2: attestation làm khó người thật trên máy root/không Play, và ta chưa có số liệu để biết mình chặn cái gì.

## 7. Thứ tự làm — checklist

| Bước | Ai | Xong khi |
|---|---|---|
| 1. Tạo Google project + consent screen (Testing) + 2 client (Android debug, Web) — §4.2 | Leader | Có Web client ID + Android client ID gửi cho Claude |
| 2. Chạy stack `e2e_slice --keep` với `MOBILE_GOOGLE_CLIENT_IDS`, Metro với `EXPO_PUBLIC_GOOGLE_WEB_CLIENT_ID`, thêm tài khoản Google vào emulator, bấm nút — §4.3–4.4 | Claude | Bậc 0→4 đúng; ảnh chụp + hàng `account_identities(google)`; ghi vào `docs/CHAY-DEMO.md` |
| 3. Chọn vendor SMS (khuyến nghị A) + mở tài khoản + lấy token thử + tài liệu API — §3.3 | Leader | Có URL, token, tên brandname, giá/tin |
| 4. Subclass sender + `MOBILE_SMS_VENDOR` + trần/ngày + chặng e2e fail-closed — §3.4 | Claude | pytest gốc xanh; e2e có chặng «gateway + debug ⇒ từ chối khởi động»; tin thật tới máy leader |
| 5. Job dọn `otp_challenges`/phiên hết hạn — §2.4 | Claude | script + test + dòng cron trong tài liệu deploy |
| 6. Quyết nơi deploy (B3) + sinh `MOBILE_PERSON_ID_KEY` production + secret manager | Leader | URL công khai HTTPS; `alembic upgrade head` chạy trước khi API nhận traffic |
| 7. PR đổi thư viện Google sang Credential Manager + `nonce` — §4.5 | Claude | Dev client mới; bậc 0→4 lại xanh |
| 8. Đăng ký brandname «RuDi»; Publish consent screen | Leader | Tin tới với tên RuDi; người ngoài Test users đăng nhập được |

Bước 1 và 3 song song, không phụ thuộc nhau. Bước 2 cần 1; bước 4 cần 3.

## 8. Cái tài liệu này KHÔNG chứng minh

- Google Sign-In **chạy được trên ảnh `google_apis` không Play Store** — chưa thử; lối thoát ghi ở §4.3.
- Bất kỳ hình dạng API của vendor SMS — mọi câu về SpeedSMS/eSMS/Twilio ở đây là từ trang công khai của họ ngày 2026-09-08, **phải đọc tài liệu kỹ thuật của vendor** khi nối.
- Người thật hiểu màn OTP không — chưa có bằng chứng hành vi (ADR-0006 vẫn gác).
- Thời điểm Google gỡ legacy SDK — Google chỉ nói «một bản tương lai».

## 9. Nguồn đã đọc

- Google: [Migrate from legacy Google Sign-In](https://developer.android.com/identity/sign-in/legacy-gsi-migration) · [Credential Manager replaces legacy APIs (blog 2024-09)](https://android-developers.googleblog.com/2024/09/streamlining-android-authentication-credential-manager-replaces-legacy-apis.html)
- Thư viện: [react-native-google-signin — Original sign in](https://react-native-google-signin.github.io/docs/original) · [issue #1373 Credential Manager](https://github.com/react-native-google-signin/google-signin/issues/1373) · [Expo — Using Google authentication](https://docs.expo.dev/guides/google-authentication/) · [@thoughtbot/react-native-social-auth](https://thoughtbot.com/blog/sign-in-with-google-for-react-native) · [@ademhatay/expo-google-signin](https://www.npmjs.com/package/@ademhatay/expo-google-signin)
- SMS Việt Nam: [SpeedSMS OTP Service](https://speedsms.vn/sms-otp-service/) · [SpeedSMS Brandname](https://speedsms.vn/sms-brandname-service/) · [eSMS — ZNS OTP](https://esms.vn/Tin-Tuc/tin-cong-nghe/zns-otp) · [eSMS API đa kênh Zalo → SMS](https://developers.esms.vn/esms-api/ham-gui-tin/tin-gencode-tu-dong-multichanel-zalo-greater-than-sms) · [AWS — Vietnam sender ID registration](https://docs.aws.amazon.com/sms-voice/latest/userguide/registrations-vietnam.html) · [Message Central — OTP Vietnam without sender ID](https://www.messagecentral.com/blog/send-otps-vietnam-no-sender-id)
- OTP-as-a-service và chống gian lận: [Twilio Verify pricing (tổng hợp)](https://www.engagelab.com/blog/twilio-verify-pricing) · [Prelude — OTP fraud 2026](https://prelude.so/blog/secure-otp) · [Telnyx Verify — security best practices](https://developers.telnyx.com/docs/identity/verify/security-best-practices) · [Arkesel — OTP expiry & rate limiting](https://arkesel.com/otp-expiration-rate-limiting-best-practices/)
- Attestation: [Play Integrity API overview](https://developer.android.com/google/play/integrity/overview) · [Guardsquare — Play Integrity drawbacks](https://www.guardsquare.com/blog/google-play-integrity-api-app-attestation)
