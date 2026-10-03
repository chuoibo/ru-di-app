# Bàn giao cho Codex: OTP qua email, bỏ lời mời đích danh, dọn nốt máy demo

Ngày 2026-10-04. Người viết: Claude (phiên gỡ bản trải nghiệm). Người nhận: một agent
Codex làm trọn lát cắt theo ADR-0032. Chưa có PR: chủ sản phẩm sẽ quyết sau cách giao (PR
hay commit thẳng `main`). Tài liệu này là toàn bộ đầu bài.

`main` đang ở `4f74b011` (gỡ bản trải nghiệm «Team Đà Lạt», đã push thẳng `main`). Ghi
chép của đợt đó: `docs/claude/2026-10-03/go-ban-trai-nghiem.md` — đọc mục «Giữ lại có chủ
đích» và «Còn mở» trước.

---

## 0. Bốn việc, theo thứ tự

| # | Việc | Loại | Cần ADR trước? |
|---|---|---|---|
| A | Đăng nhập bằng **mã OTP gửi qua email**; gỡ OTP qua SMS | đổi hành vi, đổi danh tính | **Có — ADR-0053** |
| B | **Bỏ hẳn lời mời đích danh** (lời mời đổi lấy phiên) | xoá hành vi | Có (cùng ADR-0053 hoặc ADR riêng) |
| C | Dọn nốt máy demo/seed còn lại («Team Đà Lạt» của harness, script seed) | dọn công cụ dev/CI | Không |
| D | Phủ lại hai test trình duyệt của bản đồ hành trình đã mất | test | Không |

A và B đụng cùng cửa đăng nhập, cùng bảng `account_sessions`/`account_identities`, cùng
bảng Maestro — làm A+B cùng một mạch, nhưng **commit theo lát cắt dọc chạy được**. C và D
độc lập, làm được song song hoặc sau.

## 1. Quyết định của chủ sản phẩm (nguyên văn ý, 2026-10-04)

- «Hiện tại không support cái stack OTP SMS nữa, ta chỉ duyệt qua mail thôi, thế nên gỡ
  phần đó.» Khi được hỏi cụ thể, chủ sản phẩm chọn: **mã OTP 6 số gửi qua email** (không
  phải magic link, không phải chỉ Google Sign-In).
- Về số điện thoại của lời mời và tài khoản cũ: «bỏ luôn cái feature lời mời đi».
- Code gỡ bản trải nghiệm đã push thẳng `main`; bàn giao này đi qua PR.

**Diễn giải cần xác nhận lại với chủ sản phẩm (ghi vào ADR, đừng đoán):** «lời mời» ở đây
được hiểu là **lời mời đích danh dùng để đăng nhập** (mục 3). Lời mời **vào nhóm** và lời
mời **vào buổi đi** (mục 3.3) là đường người dùng gia nhập nhóm — gỡ chúng làm app không còn
cách nào kéo bạn vào nhóm. Đừng xoá chúng nếu ADR chưa được chủ sản phẩm duyệt rõ.

## 2. Việc A — OTP qua email thay SMS

### 2.1 Cái phải quyết trong ADR-0053 trước khi viết code

CLAUDE.md: đổi hành vi thì đọc ADR rồi mở ADR trước. ADR bị thay: **ADR-0016** (phạm vi v1,
đăng nhập OTP điện thoại + Google); liên quan **ADR-0014** (phiên thay header actor),
**ADR-0023** §2.2.2 và §2.5 (gắn danh tính, `discoverable_by_phone`). ADR kế tiếp còn trống:
**0053**.

1. **Danh tính người dùng hiện suy ra từ số điện thoại.**
   `identity.DerivePersonID(canonical phone, key)` (`services/core/internal/identity/identity.go:236`)
   cho tài khoản mới id của nó (`authsteps/otpdoor.go:241`). Cùng phép suy ra ấy chống lưng
   `POST /identity/person-id` — thứ «mời vào nhóm bằng số» và «thêm bạn bằng số» dựa vào:
   người mời tạo sẵn hàng `people` dưới id đó, người được mời đăng nhập OTP là rơi đúng vào
   hàng ấy. Đổi sang email là gãy chuỗi này. Phương án phải chọn: suy id từ email (miền HMAC
   mới, ví dụ `ru-di:otp-email:v1:`), mời/thêm bạn bằng email, hay bỏ tra cứu theo số.
2. **Tài khoản đang có** gắn `account_identities.provider='phone'`. Nói rõ chúng ra sao
   (không còn cửa đăng nhập nào → cần đường gắn email, hay chấp nhận mất). Kiểm trước xem DB
   production có tài khoản thật nào chưa (phiên trước không đọc được DB production —
   xem mục 7).
3. **Bộ gửi mail**: SMTP hay dịch vụ (Resend/SES/…). Biến môi trường mới (đề xuất
   `MOBILE_EMAIL_*`), giữ đúng hình dạng an toàn của `internal/sms`: không có cấu hình thì
   `LogSender` (không gửi gì), mã debug **chỉ** hợp lệ cạnh log sender và máy chủ từ chối
   khởi động nếu có cả hai (`sms.go:157-171`). Bí mật nằm ngoài repo
   (`~/.config/rudi/stack.env`), **không** thêm mã debug vào file đó (chủ sản phẩm đã bắt
   gỡ ngày 2026-09-29).
4. **Chuẩn hoá email** (chữ thường, cắt khoảng trắng; có gộp `+tag`/dấu chấm Gmail không)
   và giới hạn: giữ nguyên máy trạng thái OTP (6 số, TTL 300 s, 5 lần thử, chờ 60 s, 5 thử
   thách/900 s — `internal/domain/otp/otp.go`), chỉ đổi kênh và định danh.
5. **Python twin**: cả hai route OTP là `LIVE-GO` (owner go, python live). Python là oracle
   của parity và đường lùi. Đổi hợp đồng (email thay phone) nghĩa là hoặc đổi cả Python
   twin + parity cùng lúc, hoặc theo ngoại lệ ADR-0036 (bản Go, bản Python, hàng manifest
   đi **cùng một commit**). `scripts/check_go_owned_python_touch.py` sẽ chặn sửa Python sau
   route Go nếu thiếu phía Go và tài liệu bằng chứng.

### 2.2 Bản đồ mã (kiểm kê tại `4f74b011`)

**Go `services/core`**
- Route: `internal/routes/auth.go` (`requestOTP` 16–42 đọc `phone`; `verifyOTP` 44–70),
  đăng ký `routes.go:140-141`. Hỗ trợ: `routes/auth_support.go` (`CanonicalMobile`,
  `PhoneDigest`, `smsDoor` 85–98, `smsSender` 120–125, `wireOtpRequest` 253),
  `routes/auth_store.go` (89–165, `otpOf` 287).
- Luồng: `internal/domain/authsteps/otpdoor.go` (`otpPhone` 26 → lỗi `phone_required`,
  `phone_not_mobile`; `RequestOTP` 57; `VerifyOTP` 154; provider `"phone"` 233, 266).
  Giao diện: `authsteps.go` (`OtpChallenge.PhoneDigest` 172, `Identity.PhoneDigest` 274–280,
  `SMSSender.SendOTP` 303–306, `DeliveryError` 135), view `wire.go:35`.
- Máy trạng thái **giữ**: `internal/domain/otp/otp.go` + goldens `domain/otp/testdata/python_otp*.json`.
- Bộ gửi: `internal/sms/sms.go` (env 22–26, `LogSender`, `HTTPSender`, `FromEnv` 131,
  `resolveDebugCode` 157); nối ở `cmd/core/main.go:232-244`,
  `internal/httpapi/endpoint/endpoint.go:86-87,147-150,195-198,284-288`.
- Giới hạn: `internal/limit/shipped.go:31-32,116-127,155-179`.
- Chuẩn hoá: `internal/identity/identity.go` (`CanonicalMobile` 147, `DerivePhoneDigest` 217,
  `DeriveCodeDigest` 226, `DerivePersonID` 236).
- Repo: `internal/repo/otp_challenges.go`, `account_identities.go`, `account_sessions.go`;
  `erasure.go` và `internal/janitor` dọn thử thách hết hạn.
- Manifest: `ownership/routes.json:1271-1300`; hợp đồng `contract/ir/auth.json`; bằng chứng
  `docs/migration/ported-w9-auth-sessions.md`, `ported-unproven-w9-auth.md`,
  `docs/migration/routes/auth/POST-auth-otp-*.md`.
- Test: `routes/auth_test.go`; `domain/authsteps/oracle_test.go`, `oracle_store_test.go`,
  `oracle_live_test.go` (+ `testdata/python_auth_steps*.json`); `domain/otp/oracle*_test.go`;
  `sms/sms_test.go`; `limit/limit_test.go`; `repo/auth_repo_{routes,oracle,world}_postgres_test.go`;
  `identity/oracle_test.go`; `httpapi/problem/testdata/python_problems.json`;
  `httpapi/router/testdata/starlette_decisions.json`.
- Parity: `parity/scenarios/w9/limiter/POST-auth-otp-{request,verify}.yaml`; bộ sinh
  `scripts/render_domain_w9_goldens.py`, `render_limit_goldens.py`, `render_problem_goldens.py`,
  `render_parity_422_scenarios.py`; `scripts/parity_stacks.sh:175,217-218` đặt env OTP.

**Schema** chỉ nằm trong alembic (`services/api/app/db/migrations`; Go không có migration):
`otp_challenges.phone_digest` và `account_identities(provider IN ('phone','google'))` —
migration `c3d4e5f6a7b8`; `account_sessions.issued_via IN ('invite','otp','google','genesis')`
— `b7e2c9a41d05`; `people.discoverable_by_phone` — `9a5e1c7b3f86`. Models
`services/api/app/db/models.py` (2548, 2865, 2902, 992). Cổng: `scripts/check_alembic_heads.py`,
kiểm migration offline trong CLAUDE.md, `tests/db/test_migration_matches_models.py`, và **thêm
ca live** ở `tests/postgres/` + tầng `scripts/go_postgres_tier.sh` (skip ở tầng đó là ĐỎ).

**Python twin** `services/api/app`: `api/sms.py` (nối `main.py:82,128-135`, `deps.py:190-197`);
`api/routes/auth.py` (128, 149); `service.py:3560, 3619`; `domain/otp.py`;
`api/person_identity.py`. Test: `tests/api/test_auth_otp.py`, `test_person_identity.py`,
`test_identity_route.py`, `test_friends_lookup_phone_shapes.py`, `test_prod_session_auth.py`;
`tests/domain/test_otp.py`; `tests/postgres/test_auth_otp_postgres.py`,
`test_person_identity_postgres.py`, `test_identity_postgres.py`, `test_identity_routes_postgres.py`;
gốc `tests/test_stack_carries_identity_key.py`.

**Số điện thoại làm danh tính ở chỗ khác** (quyết theo 2.1.1): `POST /identity/person-id`
(Go `routes/identity.go:20`, Python `routes/identity.py:157`), `POST /friends/lookup`
(Go `identity.go:54-118`, `mobileFromBody` 120; Python `routes/friends.py:204`,
`service.py:6623`; manifest `routes.json:144`), cổng `discoverable_by_phone`
(`routes/friend_writes.go:38-56`, `routes/people.go:82,208,398,664`, `peoplesteps`). Parity
`w2/identity/*`, `w2/friends/*`, `w2/crossreplay/POST-{identity-person-id,friends-lookup}.yaml`,
`w2/limiter/*`.

**Mobile `apps/mobile`**
- `app/login.tsx`, `app/otp.tsx` → `src/rudi/screens/auth/Login.tsx` (ô số 155–167,
  `chuanHoaSo` 105, `guiOtp` 112, nút «Tôi có lời mời» 195 — việc B) và `Otp.tsx` (gửi
  `phone` 100, «Đổi số điện thoại» 141, gửi lại 97/170).
- `src/rudi/otp-dang-cho.ts` (trường `phone` 13, che số `cheSo` 31);
  `src/phien.ts` (`guiOtp` 295, `xacMinhOtp` 444, `LOI_OTP_*` 276/286, `issued_via` 86,
  `discoverable_by_phone` 413/432); `src/screens/vao-cua/danh-tinh.ts` (`chuanHoaSo` 64);
  `src/screens/vao-cua/cong-api.ts:98-110`; `src/rudi/ten-giu-cho.ts`.
- Thêm bạn bằng số: `screens/friends/AddFriend.tsx:53` → `src/screens/ca-nhan/ban-be.ts:182-192`.
  Mời vào nhóm bằng số: `screens/groups/Invite.tsx:61` → `src/rudi/moi-bang-so.ts`.
- Cài đặt/hồ sơ: `CaiDatScreen.tsx:58,109,159-167` («Tìm theo số điện thoại»),
  `src/rudi/cai-dat/phien-cai-dat.ts:41-44`, `screens/profile/HoSoSong.tsx:43`.
- Câu «Số điện thoại chỉ dùng để gửi mã…» dưới form đăng nhập (`Login.tsx`) phải đổi theo.
- Test: `tests/rudi-cua-otp.test.mjs`, `phien.test.mjs`, `danh-tinh.test.mjs`, `ma-ban.test.mjs`,
  `ban-be.test.mjs`, `moi-bang-so.test.mjs`, `rudi-cai-dat.test.mjs`, `seed-rudi-world.test.mjs`,
  `chuoi-maestro-con-song.test.mjs` + `tests/fixtures/chuoi-maestro-dong.json`.
- Sổ tay Nếp được rút từ mã: đổi nhãn màn đăng nhập thì chạy `node tools/rut-huong-dan.mjs`
  rồi `go test ./internal/huongdan/` — số MRR ghim có thể dịch; ghi lại trong
  `truy_hoi_test.go` như các lần trước (ADR-0047 §9: mỗi số ≥ bản cũ − 0.01).

**Maestro + harness** — gần như cả bảng đăng nhập bằng số:
- Dùng thẳng `${OTP_PHONE*}`/`${OTP_CODE}`: flow 22, 23, 24 (mời bằng số 70–73), 25 (thêm
  bạn bằng số 47–69), 30, 36, 37; subflow `_dang-nhap-{c,d,f,so}.yaml`;
  `.maestro-motion-live/_nhap-otp.yaml`, `_vao-live.yaml`. Qua subflow: 20, 26–29, 31–35,
  38–47, `_canary-dm-bi-chan`, `_43b-story-het-han`.
- `scripts/mobile_native.sh`: biến 48, 66–82; cờ 92–131; `sinh_so_di_dong` 254,
  `kiem_ma_debug` 263, `dang_nhap_curl` 291 và mọi lời gọi 364–1625; `cho_nhip_otp` 1317;
  `canary_otp` 1797; số theo lượt 2206–2208. `tests/test_maestro_flows_dev_client.py` ghim
  hình dạng này.
- Công cụ đăng nhập qua `/auth/otp`: `apps/mobile/tools/seed-rudi-world{,-lib}.mjs`
  (`soDienThoai` lib:30), `seed-diary-native.mjs`, `chat-live-{e2e,plan,recovery}.mjs`,
  `chat-native-fixture.mjs`, `xem-dock-nep.mjs`, `xem-khoi-boi-canh.mjs`,
  `scripts/chat_e2e_seed.mjs`, `scripts/e2e_bot_nhom.py`, `scripts/do/do-motion.sh`,
  `scripts/e2e_slice.sh` (252–253, 324–347, `login_by_otp` 513), `scripts/chat_e2e_stack.sh:63,76`.
- `.env.example:66-76`.

**Bẫy:** `scripts/repo_guard.py` chặn mọi chuỗi email (`EMAIL_RE` dòng 122, luật `email`).
Fixture/flow cần email phải mang chú thích `repo-guard: allow=email reason=...`, hoặc sinh
email lúc chạy như harness đang sinh số. Luật `vn-phone` giữ nguyên.

## 3. Việc B — bỏ lời mời đích danh

### 3.1 Trong phạm vi (lời mời đổi lấy phiên đăng nhập)

- Go: `POST /sessions` — `routes/sessions_account.go:17-34` (`invite_token`), đăng ký
  `routes.go:136`; `authsteps/sessions.go:5-88` (`BootstrapSessionFromInvite`,
  `issued_via "invite"`); `auth_store.go:168-224` (`GetOutingInviteByDigest`,
  `ConsumeNamedInviteSecret`, `EnsureInvitedMembership(…,"named")`);
  `repo/named_invite_secret.go`; route xoay bí mật `routes/outings.go:313` +
  `outingsteps/invites.go:241-270`; lời mời đích danh mint token `outingsteps/invites.go:73-93`.
  Manifest `routes.json:1219` (`POST /sessions`) và 761 (rotate). **Giữ** `GET /sessions` và
  `DELETE /sessions/*` (1232–1268): quản lý phiên.
- Python: `routes/sessions.py:41-55`, `service.py:3445` (bootstrap), `service.py:4417`
  (rotate), `repository.py:1439,3561`, `schemas.py:982`.
- DB: `outing_invites` CHECK `link_carries_digest` (models.py:1408-1421),
  `account_sessions.issued_from_invite_id` + `issued_via='invite'` (2924–2955, `b7e2c9a41d05`),
  `memberships.origin 'named'` (`c5f141903a2b`). Bỏ hành vi thì bỏ cột/ràng buộc bằng
  migration mới; hàng cũ trong DB xử lý theo ADR.
- Mobile: `app/moi.tsx` → `src/rudi/screens/LoiMoi.tsx`; `src/phien.ts:101,210-246`
  (`doiLoiMoiLayPhien`); `src/rudi/loi-moi-den.ts`; `src/rudi/duong-vao.ts:87-117`
  (`maLoiMoi`, `kieu "loi-moi"`) và 187 (`/moi` trong `CUA_VAO`); `app/_layout.tsx:9,92-101,118-126`;
  nút «Tôi có lời mời» `Login.tsx:195`, `groups/Conversations.tsx:155`; nhãn «Lời mời»
  `phien-cai-dat.ts:43`; deep link `rudi://moi/...`.
- Test: mobile `tests/phien.test.mjs`, `rudi-duong-vao.test.mjs`; Python
  `tests/postgres/test_session_bootstrap_postgres.py`, `test_outing_invite_escalation_postgres.py`,
  `test_prod_session_auth.py`; Go `authsteps/oracle*_test.go`, `routes/auth_test.go`,
  `repo/auth_repo_*_postgres_test.go`, `outingsteps/oracle_replay_test.go`; parity
  `w9/sessions/POST-sessions.yaml`, `generated/w9-422/post-sessions.yaml`,
  `w7/outings/POST-…-rotate.yaml`, `generated/w7-422/…-rotate.yaml`; `w9/sessions/{GET,DELETE}`
  dùng `invite_token` để dựng — đổi sang genesis/OTP.
- Native: Maestro `21-dang-nhap-that.yaml`; `mobile_native.sh --dang-nhap` (47, 92,
  112–118, `dung_loi_moi` 1817–1870, link `rudi://moi/` 2033–2042); Makefile
  `mobile-native-dangnhap` (406–411); `e2e_slice.sh` `redeem_a_real_invite` (426–512, gọi 569).
- Xoá hành vi → **ngoại lệ ADR-0036**: bản Go, bản Python và hàng manifest đi cùng **một
  commit**. Bỏ bản Go trước thì cửa trước proxy thẳng sang Python và hành vi sống lại mà mọi
  cổng vẫn xanh.

### 3.2 Giữ

- `scripts/genesis_session.py`: phiên đầu tiên trên host sạch, và `e2e_slice.sh`
  `mint_sessions` (387), parity runner (`runner.go:641-662`), nhiều fixture Go postgres dùng
  nó. Chỉ sửa câu chuyện trong docstring («bước kế là lời mời đích danh»).
- Hàng `outing_invites` nguồn group/friend: chỉ bỏ vai trò token/phiên của chúng.

### 3.3 Ngoài phạm vi trừ khi chủ sản phẩm duyệt rõ trong ADR

- **Lời mời vào nhóm**: `POST /contexts/{id}/members` tạo membership `invited`, người được mời
  `POST /memberships/{id}/accept`. Mobile `groups/Invite.tsx`, `moi-bang-so.ts`, `Members.tsx`.
  Không có token; nhưng danh tính đến từ id suy từ số → bị việc A ảnh hưởng.
- **Lời mời vào buổi đi**: `POST /outings/{id}/invites` (nguồn group/friend/link), link chấp
  nhận bằng `POST /outing-invites/{token}/accept`, thu hồi. Thư viện mobile
  `screens/quan-tri/quan-tri.ts:193-264`, `len-plan/moi-vao-chuyen.ts`, `api.ts:2117`;
  test `loi-moi-buoi-di.test.mjs`.
- **Việc dở của phiên khác trong cây gốc** `/home/lakiet/project/mobile/ru-di-app` (chưa
  commit, lúc 2026-10-03): gói `services/core/internal/loimoi/`,
  `docs/migration/routes/contexts/{GET,DELETE}-contexts-context_id-invitation.md`,
  `apps/mobile/src/rudi/nhom/`, flow `_vao-nhom-vua-lap.yaml`, test `rudi-nhom-nguoi-b8` — một
  đợt lời mời **vào nhóm** bằng link. Đừng đụng cây gốc; hỏi chủ sản phẩm đợt đó còn sống
  không trước khi quyết 3.3. Cây gốc cũng trùng file với `4f74b011` (`Profile.tsx`,
  `Onboarding.tsx`, …) nên việc dở đó phải rebase lên `origin/main`.

## 4. Việc C — dọn nốt máy demo/seed

Production không seed (`deploy/vnlocal/compose.yml` dòng 7–8 ghi rõ) và app không còn import
dữ liệu mẫu (test `rudi-cua-otp.test.mjs` canh). Phần còn lại là công cụ dev/CI:

- `apps/mobile/src/rudi/nhom-demo.ts` — 7 người «Team Đà Lạt» của harness. Dùng bởi
  `tests/e2e/*` (vertical-slice, duong-bill, so-du-cuoi-duong-di, story-het-han),
  `scripts/e2e_demo_people.py` (qua `e2e_slice.sh:389`), `tests/test_demo_identity_matches_seed.py`
  (đọc đúng đường dẫn file), `screens/chat/uuid5.ts`, `rudi-cua-otp.test.mjs`. CI: có.
  Đề xuất: đổi thành bộ người tổng hợp trung tính trong vùng test (không tên nhóm demo), giữ
  phép đối chiếu với seed.
- `scripts/seed_demo_data.py` (compose service `demo`, `make demo`) và các cổng quanh nó:
  `reset_demo_group.py`, `check_demo_data.py`, `cong_persona_demo_sach.py`,
  `check_demo_matches_main.py`, `demo_watch.py`, `check_demo_ai_key.py` (cũng dùng ở
  `mobile_native.sh` và flow 40), `hero_walk.sh`; Makefile `demo-*`, `hero-walk*` (274–372);
  `gate.sh` chặng `demo-watch`/`hero-walk` (75, 1003–1036 — script thiếu là chặng đỏ, gỡ cùng
  lúc). Test gốc chạy trong CI: `test_demo_watch.py`, `test_demo_data_gate.py`,
  `test_reset_demo_group.py`, `test_demo_matches_main_gate.py`, `test_demo_ai_key_gate.py`,
  `test_hero_walk_binds_to_the_tree_it_walked.py`, `test_qa_evidence_runs_on_another_machine.py`,
  `test_agent_supervisor_dong_ho.py`.
- `scripts/seed_journey_demo.py`: không ai dùng (chỉ tài liệu archive) — xoá được.
- `apps/mobile/tools/seed-rudi-world*.mjs` (`make demo-rudi`, `mobile_native.sh --live`,
  `seed-diary-native.mjs`, test) và flow 20 — dựng «Team Đà Lạt» trên stack dev.
- **Không xoá mù** `services/api/app/places/seed_catalog.py`: nó là danh mục cho mọi stack
  dev/test (`docker-compose.yml:108` migrate, `e2e_slice.sh:193`, `parity_stacks.sh:149`,
  `chat_e2e_stack.sh:61`, conftest của `services/api/tests`, parity `wai/places/GET-destinations.yaml`).
  Đổi tên quán bịa thì được; xoá thì phải thay nguồn danh mục cho các stack đó.
- `docs/CHAY-DEMO.md` mô tả thế giới seed — cập nhật theo.

## 5. Việc D — phủ lại bản đồ hành trình trên web

`4f74b011` gỡ `apps/mobile/tests/rudi-hanh-trinh-web.test.mjs` và
`rudi-hanh-trinh-no-webgl.test.mjs`: chúng lái chuyến mẫu trên `/plan` khi chưa đăng nhập.
Cái chúng từng đo và nay không ai đo: Lịch trình → Bản đồ không đổi URL; marker/sheet/khớp/
chọn mốc còn; bản đồ không WebGL2 vẫn dùng được (danh sách chặng thay bản đồ); trình sửa ngày
gõ không trễ. Làm lại **không** bằng dữ liệu mẫu đóng gói: hoặc đăng nhập vào stack QA rồi mở
`/outings/{id}` (kèo có toạ độ), hoặc test thành phần `SoHanhTrinh` bằng react-test-renderer
với props dựng trong test.

## 6. Cách chạy và cổng

- Chạy tại gốc: `python3 -m pytest services/api/tests tests -q`; mobile `npm ci && npm test`
  (node_modules thật, symlink làm hỏng `build:check`); `scripts/gate.sh` (chặng cần fastapi
  chạy trong image `rudi-ux-pytest-git:dev`); Go toolchain 1.26.8 ở
  `~/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin`, đặt `GOTOOLCHAIN=local`,
  **không** đặt `GOFLAGS=-mod=mod` (nó sửa `go.mod`).
- `scripts/e2e_slice.sh` trên máy này cần venv Python 3.12 (`uv venv -p 3.12` +
  `services/api/requirements-dev.txt`) đứng đầu PATH: python host là 3.14, bản pin không cài được.
- Stack QA tổng hợp (loopback, mã debug chỉ trong container): `~/.local/share/rudi-ux/stack.sh up`
  — script build `core` từ `ROOT=/home/lakiet/project/mobile/ru-di-app` (cây gốc đang bẩn);
  chép script ra và đổi `ROOT` sang worktree của mình. Core ở `127.0.0.1:58299`.
- Maestro trên máy ảo riêng: AVD chỉ đọc cổng 5610, dev client từ debug APK, Metro 8096,
  `JAVA_HOME=/home/lakiet/.local/jdk/jdk-17.0.20.1+1`:
  `ANDROID_SERIAL=emulator-5610 MOBILE_METRO_PORT=8096 scripts/mobile_native.sh --otp --api-port 58299`.
  Không cờ thì harness thoát 2 «không đo được» (bảng fixture đã gỡ). Việc A đổi hẳn chế độ
  `--otp` sang email — đặt tên cờ mới cho đúng.
- **Đỏ có sẵn ở `2b6c9360`/`4f74b011`** (bảng `--otp`, đo A/B 2026-10-03): 29, 30, 41, 42,
  43, 45, 47, 48; 36 chập chờn. pytest gốc: 6 đỏ do môi trường (`test_demo_watch` ×2,
  `test_gate_failure_report_has_evidence` ×2, `test_make_targets`, `test_motion_measurement_gate`).
  So A/B với baseline trước khi kết luận một flow đỏ là do mình.
- Cổng bằng chứng (ADR-0030 §3): chạy lại trong cây sạch đúng SHA; canary đỏ đúng chỗ dự đoán;
  ít nhất hai đột biến tự nghĩ, mỗi cái đỏ đúng bước; UI thì mở ảnh chụp ra nhìn; **số đo vào
  commit message**. Digest của agent không phải bằng chứng.

## 7. Vận hành — không phải việc code, nhưng chặn ra mắt

Lúc ghi (2026-10-03 tối): stack production `rudi-vnlocal` sập — `core` và `rag-indexer` khởi
động lại liên tục vì Postgres :5432 và MinIO :9000 của máy vnlocal từ chối kết nối (LAN theo tên
lẫn Tailscale), agy :20131 vẫn sống. Bộ gửi SMS production chưa từng được kiểm cấu hình; sau
việc A thì bộ gửi **email** production phải được chủ sản phẩm cấp (tài khoản dịch vụ, tên miền
gửi, SPF/DKIM). DB production chưa được kiểm dữ liệu test sót (phiên trước bị chặn quyền đọc).

## 8. Xong là khi

- ADR-0053 được chủ sản phẩm duyệt (danh tính theo email, số phận tài khoản số điện thoại,
  phạm vi «lời mời», bộ gửi mail).
- Không còn đường nào trong app, Go, Python, parity, manifest, Maestro và harness nhận số điện
  thoại để đăng nhập; không còn `POST /sessions` bằng `invite_token`, `/moi`, «Tôi có lời mời».
- Đăng nhập email chạy thật trên stack QA và trên máy ảo (ảnh chụp đã mở ra xem); bảng Maestro
  đăng nhập bằng email, canary mã sai đỏ đúng chỗ, neo 2b cắn.
- `nhom-demo.ts`/seed «Team Đà Lạt» thay bằng dữ liệu tổng hợp trung tính hoặc gỡ cùng các
  cổng phụ thuộc; CI xanh tương đương baseline.
- Bản đồ hành trình có lại phép đo web không dựa vào dữ liệu mẫu.
