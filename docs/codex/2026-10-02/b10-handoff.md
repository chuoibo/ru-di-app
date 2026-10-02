# B10 · Bàn giao

## CP06 · Vỏ app và đường lạ — UI-002, UI-009

Commit worktree `dfc985f7`, trên baseline sạch `d97430db`; chưa landing `main`.
Finish reviewer trả **ship** trong phạm vi CP06 sau vòng **fix** về focus và
bằng chứng mở native. Đây là finish cục bộ, **chưa QA_ACCEPTED** và chưa thay
cổng CP09 tại SHA B10 cuối. Checklist chiến dịch do người phụ trách cập nhật.

Giữ thế giới hiện hành của [DESIGN.md](../../../DESIGN.md): giấy và mực trên
trang, bìa indigo, chữ hiệu SVG, Bricolage cho tiêu đề và body system. Màn
đường lạ dùng cảnh `tim-khong-ra`/`EmptyState` có sẵn, một hành động dẫn
«Về Rủ Đi» và Back; không in URL hay token. Bìa chờ dùng `cover`, `coverInk`,
`coverInkSoft`, lời «Đang mở Rủ Đi…» đọc được trước khi font trả lời. Không
thêm motion trang trí; điều hướng vẫn theo cơ chế reduced motion hiện hành.

| Loại | Thay đổi trong CP06 |
|---|---|
| **BUG_FIX** | UI-002: bỏ redirect Welcome vô điều kiện; đợi đọc phiên rồi dùng `manDau(phien)` cho lối về. UI-009: thay vùng trắng khi chờ font/phiên bằng `OpeningApp`. Header đường lạ nhận focus sau khi scene mở khóa. |
| **UX_IMPROVEMENT** | Đường lạ giải thích ngắn và có CTA/Back; trạng thái chờ có tên trợ năng. Navigator giữ mounted, scene chờ không nhận pointer và bị ẩn khỏi accessibility; web thêm `inert`. Sau restore, focus vào heading rồi Tab tới CTA. |
| **VISUAL_UPGRADE** | Bổ sung hai trạng thái bằng primitive và token hiện có: bìa chờ có chữ hiệu/lời chờ, trang giấy có cảnh bản đồ và thứ bậc rõ. Không đổi concept, palette, token, font hay kit toàn cục. |

Source: `apps/mobile/app/_layout.tsx`, `apps/mobile/app/[...legacy].tsx`,
`apps/mobile/src/rudi/ui/OpeningApp.tsx`; hồi quy mới:
`apps/mobile/tests/rudi-opening-web.test.mjs`. Adapter link/lời mời giữ nguyên;
CP06 không sửa auth, domain tiền, backend hay patch B9 của Claude.

| Ca retest | Bằng chứng hiện có / phần CP09 còn cần |
|---|---|
| URL lạ, có phiên và nhóm active | Android tổng hợp: «Về Rủ Đi» → Khám phá, không Welcome/Đăng nhập; mở lại đường lạ rồi Back → Khám phá. `native-recovery.log`. Reviewer đọc log/source, không chạy lại native interaction. |
| URL lạ, có phiên chưa có nhóm | Chrome 390×844, resume trễ 5 giây: bìa phủ đủ màn, scene `inert` + `aria-hidden`; sau restore heading nhận focus, 0 inert, CTA → `/messages`, không Welcome. Regression export và reviewer chạy độc lập đều xanh. |
| URL lạ, không phiên | `manDau` pure test xác nhận đích `/welcome`; **chưa có retest runtime CP06**. CP09 cần mở lạnh URL lạ, chờ đọc phiên rồi CTA/Back tới Welcome. |
| Link thật và lời mời | Source adapter không đổi; contract tests về route, fragment và mã lời mời xanh trong full run. **Chưa có retest runtime CP06**; CP09 cần kiểm cold/warm link thật và lời mời trên web/native, giữ đích và mã. |
| Back có/không history | Native Back đã có log; source dùng `luiVeVe(router, manDau(phien))`. CP09 cần đối chiếu hai trạng thái history trên web và native; không suy cả hai từ một lượt thao tác. |

Bó bằng chứng ngoài repo: `/home/lakiet/.local/share/rudi-b9a/campaign/cp06/`.
Ảnh đã được mở nhìn: [Android phone](/home/lakiet/.local/share/rudi-b9a/campaign/cp06/unknown-native-phone.png),
[Android 320dp/font 1.3](/home/lakiet/.local/share/rudi-b9a/campaign/cp06/unknown-native-320-font130.png),
[bìa native](/home/lakiet/.local/share/rudi-b9a/campaign/cp06/opening-native.png) và
[montage native](/home/lakiet/.local/share/rudi-b9a/campaign/cp06/opening-native-frames.png).
Reviewer còn mở `unknown-web.png`, `unknown-desktop.png`, `resume-web.png`.
Video thực: `/home/lakiet/.local/share/rudi-b9a/campaign/cp06/opening-native.mp4`.
Ảnh 320dp/font 1.3 giữ trọn lời và CTA trong khung đã chụp; bìa native có
chữ hiệu, lời chờ và status bar sáng. Video/montage cho thấy chỉ báo xuất hiện
trong Expo devclient; không đo production native ≤300ms từ clip này.

`opening-frame-final.log`: **1 pass / 0 fail / 0 skip**, khung web **5,0ms**
từ yêu cầu resume. Reviewer chạy riêng cùng regression trên export có kiểm
freshness: **1/0/0**, **8,6ms**. Đây là cơ hội paint qua `requestAnimationFrame`
trên Chrome, 390×844, không phải thời gian startup Android.
`focus-evidence.md` ghi heading → Tab → «Về Rủ Đi» sau trễ 5 giây; thử trễ
20 giây cho thấy cover có mặt và scene bị khóa, không phải phép đo ≤300ms.

Typecheck/export đạt. Hai full mobile run ở patch đều **1.472 pass / 1 fail /
0 skip**, lỗi selection «Lịch trình → Hành trình» trong
`rudi-hanh-trinh-web.test.mjs`. Baseline sạch full **1.473/0/0**; ca selection
chạy riêng ở patch **1/0/0** và baseline xanh. **Concern gián đoạn còn mở**:
chạy riêng xanh chưa loại trừ hồi quy patch. Giữ `mobile-test.log`,
`mobile-final-test.log`, `baseline-full.log`, `patch-journey-diagnostic.log`.

Detector rendered URL timeout; kết quả `[]` của lượt này không hợp lệ làm
chứng nhận. Fallback static `[]` chỉ rà source bằng regex, không có geometry
hay computed contrast. Không dùng các kết quả đó để tuyên bố cổng xanh.

CP09 còn điều tra concern selection và chạy full/gates trong cây sạch tại
SHA cuối, cùng canary đỏ đúng dự đoán, identity xanh và ít nhất hai mutant
không tương đương. Chưa chứng minh production native ≤300ms, TalkBack,
iOS, thiết bị vật lý, FPS, tablet/landscape hoặc bảo toàn nội dung nhập của
mọi route qua restore. Finish `ship` không khép các cổng này.

Tham chiếu: `finish-review.md`, `verdict.md`, `opening-frame-final.log`,
`focus-evidence.md` trong bó CP06; [brief/checklist](ui-ux-campaign.md).
Giữ nguyên DESIGN.md và sidecar; context báo metadata stale không mở rộng
phạm vi này, việc sửa metadata cần yêu cầu riêng.

## CP07 · Bìa Welcome — UI-016, UI-017, UI-020

Finish reviewer trả **ship trong phạm vi hai sửa chữa đã chấm**, sau hai vòng
**fix** về handoff và Reduce Motion; cả hai finding gốc đã resolved. Đây là
lát mở rộng/harden/animate của thế giới nhật ký hiện hành, không thêm thế
giới, token hay font. Chưa landing `main`, **chưa QA_ACCEPTED**; clean SHA và
full gates vẫn thuộc CP09.

Giữ bìa vải indigo, chữ hiệu SVG cream, Bricolage, washi và con dấu coral.
Bốn câu/bốn mốc vẫn nối hội bạn → chọn nơi → rõ phần tiền → giữ kỷ niệm.
Tiêu đề pager dùng Bricolage ExtraBold 28/33, body system 17/24, counter
caption 13/18; mực trên coral dùng `mauSang.ink` tĩnh. Không đổi lời hứa về
tiền hay quyền AI đọc dữ liệu.

| Loại | Thay đổi trong CP07 |
|---|---|
| **BUG_FIX** | UI-016: `onScroll` và momentum cùng cập nhật copy/mốc/chấm; resize giữ đúng trang. UI-017: snap web `always`, native `disableIntervalMomentum`, dừng một chặng trong các lượt vuốt nhanh đã chạy. Bỏ outgoing 3D/callback chờ trước push Login; chốt một navigation, chỉ mở khóa khi Welcome nhận focus lại. |
| **UX_IMPROVEMENT** | UI-020: bốn control có tên, role button, trạng thái chọn và vùng bấm ≥48dp với gap 8dp; pager nhận focus và Left/Right/Home/End; trang ngoài khung bị ẩn khỏi accessibility, counter live lịch sự. «Tìm hiểu thêm» tiến một trang, trang cuối có «Xem lại từ đầu». |
| **VISUAL_UPGRADE** | Giữ topology của bìa và một câu dẫn mỗi trang. Washi cao theo chữ xuống dòng; cửa sổ ngắn/font ≥1.3 nhường đường trang trí cho chữ và CTA. Welcome → Login dùng một handoff native, reduced cắt thẳng theo navigator hiện hành. |

Source: `apps/mobile/src/rudi/screens/Welcome.tsx`; hồi quy:
`apps/mobile/tests/rudi-welcome-web.test.mjs`. Welcome prefetch Login và push
ngay khi chạm dấu; không còn lớp bìa nghiêng trên tờ trắng rồi slide lần hai.
Stack tiếp tục dùng luật Reduce Motion có sẵn; CP07 không đổi backend/auth,
token toàn cục hoặc patch B8/B9 của Claude.

| Ca retest | Bằng chứng và giới hạn |
|---|---|
| Web vuốt/phím/accessibility/resize | `regression.log` kết hợp Welcome và opening: **2 pass / 0 fail / 0 skip**; Welcome riêng `welcome-web.log`: **1/0/0**. Touch fling 1→2, AX tree không đọc trang 1 ngoài khung, bốn control ≥48px, End/Home/Right, resize 390→768 giữ trang 2, chọn trang 4/reset và CTA reduced tới Login. Đây là lượt CP07, không phải full mobile tại SHA cuối. |
| Native pager và Back | `native.log` ghi hoàn tất từng swipe một chặng, chọn trang 4, reset, CTA và một Back. 320dp/font 1.3 dùng geometry vuốt đã sửa: mọi bước trong `native-narrow.log` hoàn tất nhưng CLI **exit 143**, không gọi là command xanh. AVD mất kết nối rồi được khởi động lại cùng AVD. |
| Normal handoff cuối | `cover-native.mp4` phiên **02:11**, cùng `cover-frames.png`/`cover-later-frames.png`: reviewer xác nhận một handoff, không blank wait/3D/hai bước; `motion-check-normal.txt` ghi double tap Login **True**, single Back Welcome **True**. Packet normal cũ sai phiên bản và montage có ô đen đã được thay, không dùng làm bằng chứng cuối. |
| Reduced lạnh/warm cuối | `cover-reduced-cold.mp4` **02:18** và `cover-reduced-warm.mp4` **02:20**: reviewer giải mã đủ **22/20 khung thật**, entry và Back đổi trực tiếp giữa hai màn. Cold Back frame 14/3.965156s → 15/4.002089s; warm 12/5.031722s → 13/5.211089s. Hai `motion-check-reduced-*.txt` đều True/True; `reduced-os-state.txt` ghi ba scale 0, probe sau reload true/true và đã gỡ. Không suy khoảng timestamp thành FPS hoặc thời lượng transition. |

Bó bằng chứng ngoài repo: `/home/lakiet/.local/share/rudi-b9a/campaign/cp07/`.
Documenter đã mở nhìn [phone](</home/lakiet/.local/share/rudi-b9a/campaign/cp07/after-native-phone.png>),
[320dp/font 1.3](</home/lakiet/.local/share/rudi-b9a/campaign/cp07/after-native-320-font130.png>),
[tablet](</home/lakiet/.local/share/rudi-b9a/campaign/cp07/after-native-tablet.png>),
[web 390×844](</home/lakiet/.local/share/rudi-b9a/campaign/cp07/after-web-phone.png>) và
[web 1280×800](</home/lakiet/.local/share/rudi-b9a/campaign/cp07/after-web-desktop.png>),
cùng ba montage `verdict-timed-cover-{native,reduced-cold,reduced-warm}.png`.
Ảnh narrow giữ đầy đủ chữ/CTA và washi hai dòng. Reviewer còn mở ảnh before
và montage trong `finish-review.md`; ảnh before dùng bundle incumbent cache
đã được nhận diện trước verification, sau đó devclient được mở lạnh lại.

Typecheck và web export đạt trong packet CP07. Axe: **0 violations / 32 passes /
11 color-contrast nodes incomplete** do SVG/vân, không chứng nhận AA toàn màn.
Detector vẫn **23 mobile / 34 desktop findings, exit 2**; không thêm ignore và
không gọi sạch. Triage đã ghi trong `evidence.md`/`finish-review.md`: vân hiện
thật, clip thuộc lớp bounded, route bốn chặng có nghĩa; phép dò DOM đọc xuyên
nền SVG coral. Mẫu ảnh khoảng 4.62:1 web/4.65:1 native không thay phép đo
conformance; cảnh báo mép glyph/transition padding chưa chứng minh jank.

Lần reduced 02:01 còn Back trượt, lần warm cũ thất bại Back và deep link quá
sớm bị bỏ qua vẫn là lịch sử thực nghiệm. Retest lạnh/warm sau app ready giải
quyết finding đã chấm, **không xác lập nguyên nhân** các lần trước. Packet
không có QUALITY BAR card/comp quyết định riêng; finish dùng code và thế giới
incumbent. Không suy emulator thành iOS, thiết bị vật lý, TalkBack hay FPS.
CP09 còn full mobile/gates trong cây sạch tại SHA cuối, canary đỏ đúng dự đoán,
identity xanh cùng harness SHA và ít nhất hai mutant không tương đương.

Giữ nguyên `PRODUCT.md`, `DESIGN.md` và `.impeccable/design.json`. Có drift
đã ghi nhận: mục Welcome/sidecar còn tagline cũ, kích thước wordmark/chấm cũ
và nhấc bìa rồi push, trong khi brief CP07 đã chốt handoff đơn; sidecar vẫn
giữ cấu trúc legacy để tương thích script. Không sửa hoặc biến findings
detector/contrast incomplete thành luật thiết kế chỉ để chứng minh lượt
documenter. Tham chiếu [brief/checklist](ui-ux-campaign.md) và
`finish-review.md`, `regression.log`, `evidence.md` trong bó CP07.

## CP08 · Bảng Nếp, đồng ý vẽ và lề Khám phá — UI-008, UI-011, UI-012, UI-014

Lát mở rộng Operate + Experience của hệ giấy hiện hành: nhập lời nhờ → đọc
đúng mô tả sẽ gửi → Sửa/Vẽ → phản hồi cạnh bản nháp → về màn cũ. Finish
reviewer trả **ship cho hai sửa chữa đã chấm** sau một vòng fix: câu về thời
gian/phạm vi chia sẻ luôn thấy cạnh hành động ở font 1.3, và đối chiếu đủ
6/20 phần tử buried-raster. Đây là finish cục bộ, **chưa QA_ACCEPTED**, chưa
landing `main`; full mobile và clean exact-SHA gates vẫn thuộc CP09.

Giữ `card`, `ink`, `inkSoft`, `line`/`lineStrong` và màu AI semantic; tiêu đề
«Nhờ Nếp vẽ?» dùng Bricolage H1 28/34, mô tả/giải thích dùng body system
17/24, tiêu đề bảng dùng title 17/23, nhãn gợi ý dùng caption 13/18. Nếp
`dua-giay` 72dp nối lời nhờ với bước xác nhận. Mô tả dài cuộn có chỉ báo;
disclosure và Sửa/Vẽ đứng ở chân bảng. Không thu nhỏ chữ để vừa screenshot,
không thêm world, font, token hoặc QUALITY BAR card/comp mới.

| Loại | Thay đổi trong CP08 |
|---|---|
| **BUG_FIX** | UI-008: editor và hành động ở chân sheet, sheet thích ứng IME/safe area; lỗi không đẩy thao tác khỏi khung. UI-011: thay native Alert bằng bước xác nhận trong cùng bảng, giữ mô tả đóng băng theo màn/người; chống gửi lặp. UI-012: tiết mục đang chạy có nút skip được đặt tên, khung tĩnh không còn nút vô danh. UI-014: rail danh mục live/demo dừng trước mép Nếp, bỏ negative margin bên phải. |
| **UX_IMPROVEMENT** | Back/Escape/browser Back ở bước xác nhận trả lại editor và bản nháp; Sửa không gửi; đóng bảng đóng cả flow. Lỗi 503 và trạng thái chờ nằm cạnh bản nháp, có live region. Lời về phạm vi Nếp nhận luôn đọc được trước Vẽ đi. Gợi ý có đích bấm tối thiểu 48dp và gap 8dp. |
| **VISUAL_UPGRADE** | Hai trạng thái cùng giấy có lỗ gáy, nét mực và thứ bậc hiện hành; Nếp đưa giấy cho bước quyết định. Kẻ tóc phân vùng mô tả/footer, focus editor dùng AI semantic; hai hành động phân biệt outline và solid AI. Tablet giữ cột sheet tối đa 640dp. |
| **MOTION_UPGRADE** | Giữ spring vào/standard ra và reduced motion của Sheet; thao tác hủy trở lại editor có focus, không nhảy body web. Gợi ý dùng PressScale/haptic select; tiết mục M1 vẫn diễn một lần rồi tĩnh, có cách bỏ qua bằng phím. Không thêm độ trễ trang trí hoặc diễn lại Nếp. |
| **DESIGN_SYSTEM_IMPROVEMENT** | Sheet thêm `footer`, `avoidKeyboard`, `onBack` và chỉ báo cuộn opt-in; default callers giữ API/hành vi cũ. Đây là khả năng của primitive, chưa phong thành luật bố cục toàn app; CP09 phải kiểm hồi quy các callers. |

Source: `apps/mobile/src/rudi/nep/NepBang.tsx`, `ui/Sheet.tsx`,
`ui/NepDien.tsx`, `screens/Discovery.tsx`, `screens/explore/ExploreLive.tsx`;
hồi quy mới `apps/mobile/tests/rudi-nep-web.test.mjs`. Không đổi money,
quota, API/E2EE, quyền AI hay patch B8/B9 của Claude. UI-014 là ca gutter mới,
không làm lại batch B7.

| Ca QA cần retest | Expected / bằng chứng cục bộ |
|---|---|
| Nhập dài, keyboard mở và font lớn | Editor/Vẽ/Gửi và lỗi nằm trên IME; tham khảo cuộn riêng. Android `keyboard-check-final.txt` và `keyboard-check-320-font130.txt` ghi `mInputShown=true`; ảnh actual IME đã mở nhìn. |
| Vẽ → Back/Escape/browser Back hoặc Sửa | Trở lại đúng draft, không POST; web focus về editor, body top 0. Native phone/narrow chạy Back và Sửa; regression dùng browser history thật. |
| Xác nhận mô tả dài | Đọc trọn lời về lượt vẽ, thời gian và đúng phạm vi chia sẻ cạnh Sửa/Vẽ đi; mô tả có thể cuộn, chỉ báo persistent Android/thin web. Cùng paragraph ở phone, 320dp/font 1.3, tablet và web recapture cuối. |
| Vẽ đi, tap lặp, lỗi rồi thử lại | Chỉ một POST với đúng mô tả đóng băng và tên màn; 503 hiển thị cạnh bản nháp còn nguyên. Đóng bảng từ xác nhận không gửi. Provider bị chặn trước upstream, không tiêu quota thật. |
| Gợi ý, gutter live/demo, tiết mục M1 | Gợi ý ≥48dp; web 320dp rail kết thúc 304, mép Nếp 310. M1 đang chạy là skip có tên và phím Enter bỏ qua; xong là ảnh tĩnh có mô tả, ngoài Tab order. AX tree của lượt Create không có nút vô danh. |

Bó bằng chứng ngoài repo: `/home/lakiet/.local/share/rudi-b9a/campaign/cp08/`.
Android emulator-5600 `rudi-diary-review`, devclient SDK57, Metro 8163 từ B10:
phone **1080×2400/density 420/font 1.0**, narrow **840×1840/density 420/font
1.3**, tablet **2016×2688/density 420/font 1.0**. Web **320×700 và 1280×800
dark**. Dữ liệu tổng hợp; proxy loopback 58395 chặn AI POST, trả 503
`nep_media_chua_cau_hinh`; backend binary B9a, không phải backend B10 mới.
`phone-final2`, `narrow-footer`, `tablet-final`/`tablet-settle` hoàn tất CLI
exit 0; tablet capture cuối ghi `mInputShown=false` sau ổn định.

Documenter mở nhìn [xác nhận 320dp/font 1.3](/home/lakiet/.local/share/rudi-b9a/campaign/cp08/after-native-confirm-320-font130.png),
[desktop dark](/home/lakiet/.local/share/rudi-b9a/campaign/cp08/after-web-confirm-desktop.png)
và [keyboard thật](/home/lakiet/.local/share/rudi-b9a/campaign/cp08/after-native-keyboard-visible-final.png).
Reviewer mở packet 13 ảnh rồi năm recapture consent cuối; người phụ trách mở
montage `panel-keyboard-final-frames.png` và `performance-frames.png` từ video
native thật. M1 cho thấy sheet vào, Nếp đổi tư thế rồi đứng; không đo FPS/frame
time. Baseline `before-native-panel.png`, `before-keyboard-fix.png` giữ riêng;
vòng narrow cũ có đuôi draft nhân đôi không dùng làm cặp trước/sau cuối.

Typecheck và web export đạt. `regression-final.log`: **6 pass / 0 fail / 0
skip**: ba ca CP08, hai ca opening (gồm anonymous recovery mới của CP09), một
ca Welcome. Không đọc con số này thành full mobile hoặc native gate tại SHA
cuối. Concern selection «Lịch trình → Hành trình» từ CP06 vẫn cần CP09 khép.

Detector rendered frozen DOM/CSSOM vẫn **23 mobile / 36 desktop findings,
exit 2**, không ignore. `raster-map.md` và hai `raster-map-*-detail.json` đối
chiếu đủ 6/20 IMG opacity 0: cùng `vai-bia.png`, alt rỗng, cạnh sibling DIV
background-image cùng URI/geometry opacity 1, thuộc nền Explore inert dưới
sheet. Vân ngoài sheet nhìn thấy; không phải 26 asset mới bị giấu. Mapping
không đóng các findings còn lại, không chứng nhận detector sạch hay AA.

Chưa kiểm provider/media job thành công thật, iOS, thiết bị vật lý, TalkBack,
native dark, landscape hoặc FPS. Success incumbent chỉ review source, không
gọi là runtime verified. CP09 còn full regression shared Sheet, clean
exact-SHA gates, identity/canary và hai mutant không tương đương. Tham chiếu
`evidence.md`, `finish-review.md`, `finish-verdict.md`, `regression-final.log`
và [brief/checklist](ui-ux-campaign.md).

Giữ nguyên `PRODUCT.md`, `DESIGN.md`, `.impeccable/design.json`: ordinary
extension, không được yêu cầu sửa hệ. Mục Sheet trong tài liệu vẫn mô tả tay
cầm có label/hint và công thức scrim cũ; sidecar giữ cấu trúc legacy. Không
canonize các mô tả lệch source, số đo contrast incomplete hay findings
detector thành luật mới; capability opt-in của CP08 được ghi ở handoff này.

## CP09 · Kiểm chứng tích hợp đang chạy

Lượt clean `dea5152d`: API **3028 pass / 743 skip**, không suy thành PostgreSQL
đã kiểm; mobile **1478 pass / 1 fail / 0 skip**. Finding thật là bản đồ
hướng dẫn Nếp chưa chứa sáu nhãn mới của Welcome. Generator
`tools/rut-huong-dan.mjs` cập nhật đúng sáu nhãn và băm frontend/Go; diff
đã đọc, không cập nhật snapshot mù hay bỏ assertion. Test hướng dẫn riêng
và Go package đã chạy lại; full gate phải tiếp tục ở SHA chứa sửa này.
Chặng AI offline ban đầu thiếu env đã retest bằng requirements đúng pin,
**1 stage pass / 0 fail / 0 skip**; không gọi provider thật.

Identity/canary/mutants tại `dea5152d` dùng cùng hash của opening/chrome-cdp/
tuoi-ban-dung: identity trước/sau **2/0/0**, bỏ cover đỏ ở chờ first frame,
bỏ inert đỏ ở protected scene, CTA luôn Welcome đỏ ở home signed-in.
Mỗi variant export mới; source variant cuối khôi phục sạch. Cần identity
cuối cho SHA có map mới trước kết luận gate.

Android warm (5600) mở địa điểm → Back → unknown → Back → invite; guest
(AVD riêng5620) unknown → CTA Welcome → invite. Hai CLI hoàn tất exit0,
ảnh mở nhìn; mã CP09-TEST-ONLY chỉ điền, không redeem. Cold custom scheme
vào devclient launcher khi chưa nạp JS; đó không phải evidence release
link hỏng hay pass. Release cold link và cold invite vẫn chưa kiểm.

Emulator cũ đã khôi phục **14 private file**, băm nội dung khớp backup mới
trước lượt này; font/system scales/size/density và reverses trả lại. Gate
pm-clear chỉ dùng AVD5620 tổng hợp mới tạo. APK SDK57 kéo từ devclient
đã cài; không có fingerprint native build trong clean tree để chứng nhận
APK dependency matching. Fingerprint Metro/runtime JS là cổng riêng.

Review hai assessment độc lập Nếp: UX **28/40**, native kỹ thuật **15/20**.
Ba P2 cần retest/iteration kế: độ hẹp khối disclosure khi font lớn; cue
phân biệt hỏi/vẽ tại editor; lối tiếp sau lỗi chức năng chưa bật. Đây là
findings mới của critique, không gán lại thành bug QA. B10 giữ cấu trúc
quyền/chia sẻ và hai hành động hiện hành; không thêm mode mới hoặc tắt
thao tác dựa trên chuỗi lỗi. Ưu tiên kiểm với người dùng/chức năng thật
trước đổi meaning hoặc availability. Không có P0/P1 được hai assessment
xác nhận trong phạm vi này, không suy thành toàn app sạch.

Detector **23/36** là scan frozen DOM trước batch fix footer. Recapture
cuối chứng minh hai sửa finish; JSON không phải scan exact SHA cuối.
Không có overlay inject vì browser evaluate read-only; metadata/design
drift và contrast incomplete giữ nguyên. Bó CP09 ngoài repo chứa hai
assessment, mutation-plan/results, log gates và device-restore.
