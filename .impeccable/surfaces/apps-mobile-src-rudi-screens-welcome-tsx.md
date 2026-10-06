---
version: 1
slug: "apps-mobile-src-rudi-screens-welcome-tsx"
primary_target: "apps/mobile/src/rudi/screens/Welcome.tsx"
related_targets: ["apps/mobile/src/rudi/screens/auth/Login.tsx","apps/mobile/src/rudi/screens/auth/Register.tsx","apps/mobile/src/rudi/screens/Onboarding.tsx","apps/mobile/src/rudi/screens/explore/ExploreLive.tsx","apps/mobile/src/rudi/screens/explore/DiemDenScreen.tsx","apps/mobile/src/rudi/screens/explore/HangDiaDiem.tsx","apps/mobile/src/rudi/screens/explore/PlaceDetailLive.tsx","apps/mobile/src/rudi/screens/Create.tsx","apps/mobile/src/rudi/screens/keo/PlanLive.tsx","apps/mobile/src/rudi/screens/keo/CreateOutingLive.tsx","apps/mobile/src/rudi/screens/keo/PickOutingLive.tsx","apps/mobile/src/rudi/screens/keo/OutingLive.tsx","apps/mobile/src/rudi/hanh-trinh/ManHinhHanhTrinh.tsx","apps/mobile/src/rudi/hanh-trinh/SoHanhTrinh.tsx"]
---

# Surface brief · Cửa vào, khám phá và lời hẹn A–F

Phạm vi: Welcome/auth/OTP/gu → Explore/destinations/địa điểm/chọn kèo → Tạo/Lên plan/tạo kèo/timeline/bản đồ/check-in. Mode: auth/form/picker/map = Operate; Welcome và khám phá có một khoảng minh hoạ của thế giới nhật ký hiện hữu. Dữ liệu hiển thị từ API và storage thật; evidence chỉ dùng dữ liệu tổng hợp. Giữ vocabulary gu/bốn mức chi, session/deep link, revision/Attempt, integer money và thứ tự tác vụ theo entry context.

## Direction contract

**THESIS:** Mở một cuốn sổ để đi từ lời rủ đến một cuộc đi thực có. Mỗi màn đặt tác vụ chính và trạng thái thật trước vật liệu giấy. Không dùng khung icon trống thay ảnh, không bịa địa điểm hay tuyến đường.

**OWN-WORLD:** Bìa chàm, trang giấy theo theme, Bricolage ở tiêu đề, body theo hệ thống; coral cho thao tác và chọn, teal cho tiền/check-in, tím cho AI. Dùng registry asset-pack chung. Minh hoạ địa điểm có nhãn; ảnh API luôn đi với credit. Form dùng material quiet; vé và trang hành trình giữ đường cắt/giấy của hệ thống.

**STORY:** Bốn trang Welcome dẫn đến login. Xác minh email và trả lời gu giữ thông tin khi API lỗi. Khám phá mở thành phố đang xem, search/filter rồi một kết quả dẫn và cặp so sánh. Lưu là thao tác độc lập. Địa điểm thêm vào kèo có thật; tạo kèo giữ chặng đầu và đúng context. Timeline sửa bằng bản nháp/revision, check-in chỉ báo đã tới sau API.

**FIRST VIEWPORT:** Ở 320/390dp, Welcome có wordmark nhỏ, một cảnh 88–190dp, một câu dẫn, pager có target48dp và CTA. Form auth bắt đầu trong viewport đầu, art nhỏ84–96dp. Gu hai cột; budget2×2, chữ lớn chuyển một cột. Explore có destination control, postcard150dp, search + AI, filter; first result có ảnh/nhãn. Plan vé gọn với giờ/tên chặng thật; tạo kèo theo tên/ngày/người/chi. Map là tiles thật và status đường nằm ngoài path.

**FORM:** Extension code-led của world «Sân khấu giấy» đã pin; không chọn world mới, không sửa DESIGN.md global. Kế thừa v3 ADR-0037 và seed v2 `c8e88116`. Signature move: một bưu thiếp/cut giấy dẫn vào thao tác, feedback arrival chỉ settle sau response. Reduced motion và touch targets dùng shared primitives.

**FINISH:** unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Phạm vi cần capture/review

- Welcome: 320/390, trang2–4, swipe/keyboard/resize, reduced motion, CTA.
- Auth: loading/error/Google thiếu config; register/reset initial/challenge/success; OTP paste/backspace/resend; keyboard/font1.3. Login giữ policy xoá password sau request hiện có, giữ username.
- Gu: 320 hai cột và ngân sách2×2, selected có check, font1.3 một cột; skip, lưu lỗi giữ chọn.
- Explore: no-photo lead/compare/rows, ảnh hỏng có credit đúng state; save pending/rollback, filter reset/no-result/query; AI search và dock độc lập.
- Destination: selected postcard + row marker khác nhau theo source; searchempty, storagefail/pending; chỉ selected có stamp «Đang xem».
- Place: hero/facts/footer, savefail, gallery không ảnh/1/nhiều ảnh, viewer close/back/swipe/buttons/loading/error/retry, nhóm riêng đúng quyền.
- Create/Plan: sáu tác vụ/entry context/back; không nhóm, no-outing, upcoming/past, vé có chặng thật; form validation/budget/place/context/partial creation/API error.
- Outing: catalogue loading/empty/error/retry trong sheet, draft/conflict/reconcile, timeline add/reorder, check-in pending/confirmed/error; map no-coordinates/uncomputed/providerfail, edit/pin, two-point calculation requirement, safe area.

## Validation đã chạy

`tsc -p tsconfig.test.json`, `tools/fixup-esm.mjs`: exit0. 12 suite contract/catalogue/credit/OTP/gu/kèo/bản đồ/thứ tự Tạo:132tests pass,0fail,0skip (815ms). `npm run typecheck`: exit0 sau khi các source agents cùng sửa lỗi tích hợp; log `/tmp/nep-af/typecheck.log`. TAP regression lưu `/tmp/nep-af/contracts.tap`. Đây là regression code. Runtime Android dev client trên emulator5630 đã kiểm A–F và một số màn G–S bằng dữ liệu tổng hợp, đã mở ảnh; packet ngoài worktree tại `/home/lakiet/.local/share/rudi-nep-ui/evidence/android/README.md`. Font1.3/2.0, keyboard, Android Back, reduced motion và map native có bằng chứng theo từng trạng thái. Lỗi Fabric khi chọn điểm đến đã tái hiện lạnh và sửa bằng việc không sắp lại children ngay trước pop. Shared calendar Back và keyboard geometry do root sửa, phải đọc trạng thái cuối trong packet; không suy rộng sang mọi màn hay máy vật lý.

## Cổng còn phải chứng minh

Root central export/runtime/capture và fresh finish reviewer. Ảnh native/location/camera, Google provider/realemail, realrouting config, physical device và iOS không được suy ra từ web/test unit. Legacy AI tờ hẹn đọc theo đường source; quyền ghi chat do lát cắt G xử lý. Không sửa backend hay hợp đồng tiền trong A–F.
