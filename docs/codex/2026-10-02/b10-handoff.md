# B10 · Bàn giao

Điểm đọc đầu tiên khi Claude tiếp tục: [bàn giao tổng hợp13 ca và phần còn lại](claude-resume-handoff.md).
Các số/SHA bên dưới là lịch sử kiểm chứng riêng của batch; bản gộp và xung đột production mới ghi ở tài liệu tổng hợp.

## CP06 · Vỏ app và đường lạ — UI-002, UI-009

Commit worktree `dfc985f7`, trên baseline sạch `d97430db`; chưa landing `main`.
Finish reviewer trả **ship** trong phạm vi CP06 sau vòng **fix** về focus và
bằng chứng mở native. Đây là finish cục bộ, **chưa QA_ACCEPTED** và chưa thay
cổng CP09 tại SHA B10 cuối. Checklist chiến dịch do người phụ trách cập nhật.
Kết quả bổ sung ở CP09 bên dưới đã khép concern selection bằng full mobile
1479/0/0 và kiểm cold guest trên release cục bộ; các số đỏ dưới đây là lịch
sử CP06, không phải trạng thái cuối của concern đó.

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
| **BUG_FIX** | UI-008: tiết mục đang chạy có nút skip được đặt tên, khung tĩnh không còn nút vô danh. UI-011: thay native Alert bằng bước xác nhận trong cùng bảng, giữ mô tả đóng băng theo màn/người; chống gửi lặp. UI-012: chip gợi ý có đích bấm tối thiểu 48dp. UI-014: rail danh mục live/demo dừng trước mép Nếp, bỏ negative margin bên phải. Editor/footer thích ứng IME/safe area là cải thiện UX chủ động trong cùng flow. |
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

## CP09 · Kết quả kiểm chứng và cổng còn mở

**UI-009 mở lại sau video release `8ac64443`.** Các lượt dưới đây là bằng
chứng trước sửa native mới: icon mặc định/khung trắng trước JS và Welcome
trượt trên nền trắng vẫn nhìn thấy trong `native-release-startup-frames.png`.
`am start -W` ghi COLD/657ms, không phải phép đo first-brand-frame. Patch
bổ sung splash chính thức SDK57 với chữ hiệu hiện có, hide khi layout có
mặt, bìa index và Welcome không slide khi vào. Source đã commit riêng tại
`6144ff2c`; build/retest native và finish review đã khép ở phạm vi dưới đây.
Clean mobile/native, identity và đối chứng tại SHA mới đã khép: UI-009
là **READY_FOR_QA**, cùng tám ID B10 còn lại. Native300ms, QA độc lập và
các cấu hình còn thiếu vẫn là cổng riêng, không nhận QA_ACCEPTED.

### UI-009 · Pattern native, bằng chứng và retest bổ sung

Bằng chứng đi cùng repo: [ảnh trước](evidence/ui009/startup-before.png),
[ảnh sau](evidence/ui009/startup-after.png), [clip native](evidence/ui009/startup-after.mp4)
và [nguồn/giới hạn](evidence/ui009/README.md). Hai lần quay có launcher khác;
không nhận so thời gian hoặc pixel match. Fresh reviewer kiểm ba copy trùng
byte/source, PNG không metadata, MP4 không audio/payload metadata và mở
đủ **38/38 frame giải mã**; safe đúng ba hash trong
`ui009-committed-evidence-review.md`, ghim riêng trong repo guard.
Đây là review dữ liệu/evidence, không nâng verdict thành chứng nhận hiệu năng.

Ordinary scoped improvement trong hệ notebook: native splash trước JS giữ
chữ hiệu trên indigo; React `OpeningApp` tiếp cùng bìa trong lúc đọc
font/phiên; index vẽ bìa khi quyết định Redirect. `onLayout` hide splash khi
React đã có layout, không timer chờ trang trí. Index/Welcome dùng animation
none/nền cover; CTA → Login và Back giữ handoff hiện hành, tắt khi reduced
motion. Đây vừa là **BUG_FIX** khung Expo/trắng, **UX/VISUAL_UPGRADE** về
continuity nhận diện và **MOTION_UPGRADE** bỏ entry slide; `onLayout` là
capability startup, chưa phong thành luật toàn app. Giữ nguyên `PRODUCT.md`,
`DESIGN.md`, `.impeccable/design.json`; không identity, font hay token mới.

Packet base `8ac64443`, fingerprint literal **`b10-ui009-review`**;
`splash-build-source.json` pin 8 file, root đối chiếu **8/8 hash khớp source
commit `6144ff2c`**. APK dựng trước commit từ source byte-identical, bản
dựng/cài cùng SHA256
`3abcfa68341e6114ee83652cc8af9a652f85f731c98cfd28810689c894928945`.
Release cục bộ bundled, không Metro, debug signing; không profile production
và không tự thay gate clean exact-SHA. Clean mobile riêng tại `6144ff2c` đã kết thúc **1 stage pass / 0 fail / 0 skip**, **1479/0/0** + typecheck và export Android/iOS/web (86s). Lượt đầu đỏ được giữ trong concern bên dưới.

| Phép kiểm mới | Kết quả / phạm vi |
|---|---|
| Native build | `splash-native-build.log`: **BUILD SUCCESSFUL**, 4m33s, **1460 task** (1428 executed, 32 up-to-date); SDK57, assembleRelease x86_64. |
| Cold release guest | `splash-release-cold.log`, **exit0**: URL lạ → recovery → CTA/Quay lại Welcome; lời mời giữ `CP09-TEST-ONLY`, không redeem; icon → Welcome → Login → Back, đúng fingerprint. |
| Narrow dark/reduced | `splash-native-narrow.log`/JSON, Maestro/record **exit0**: **840×1840, density420 (~320dp), font1.3**, night yes, ba animation scale **0/0/0**; Welcome/CTA/Login/Back. |
| Tablet thường | `splash-native-tablet.log`/JSON, Maestro/record **exit0**: **2016×2688, density420, font1.0**, light, scale **1/1/1**; cùng flow. |
| Web hồi quy | `splash-web-regression.log`: **6 pass / 0 fail / 0 skip** (ba Nếp, hai opening, một Welcome); ảnh font settled **390×844/1280×800**. Không suy thành full mobile/native. |

Finish reviewer fresh ghi **`ship`** cho continuity startup trong
[ui009-finish-review.md](/home/lakiet/.local/share/rudi-b9a/campaign/cp09/ui009-finish-review.md).
Documenter mở nhìn [startup phone](/home/lakiet/.local/share/rudi-b9a/campaign/cp09/ui009-review/native-phone-startup-frames.png),
[reduced-motion narrow](/home/lakiet/.local/share/rudi-b9a/campaign/cp09/ui009-review/native-narrow-dark-reduced-transition-dense.png),
[Login/Back tablet](/home/lakiet/.local/share/rudi-b9a/campaign/cp09/ui009-review/native-tablet-normal-transition-dense.png),
[Login font1.3](/home/lakiet/.local/share/rudi-b9a/campaign/cp09/ui009-review/native-narrow-ui009-login.png)
và [Opening web](/home/lakiet/.local/share/rudi-b9a/campaign/cp09/ui009-review/mobile-opening.png).
Các khung phone đã lấy mẫu cho thấy wordmark/indigo → Welcome, không
Expo/trắng như baseline; không tách rõ React Opening thành giai đoạn riêng.
Tablet có khung slide Login/Back; narrow scale0 cắt về Welcome. Mẫu thời gian
không loại được flash giữa hai khung, không đo first-brand ≤300ms, FPS ứng
dụng, frame pacing hay hiệu năng native.

| Controlled artifact | SHA256 được reviewer xác nhận safe |
|---|---|
| `apps/mobile/assets/rudi/wordmark-splash.png` | `6296dc0fcd7ad6ab0857e8210f61533bc2a74f858758b552f257bb897be8b99c` |
| `apps/mobile/package-lock.json` | `e99327fe89cf009b69d1079a853530a7ba7334c9205bb924b061fbd3eef6e36f` |

PNG **828×288 RGBA, 24.764 byte**, chỉ mark kem trên alpha;
IHDR/bKGD/IDAT/IEND, không text/EXIF. Reviewer tái raster độc lập trong bộ
nhớ bằng **CairoSVG2.8.2** từ đúng bốn `GLYPHS` hiện có trong `Wordmark.tsx`,
viewBox đúng vector nguồn, fill `#f7f3ec`: trùng toàn bộ byte/hash, pixel diff
rỗng. Baloo 2 ExtraBold/Ek Type/OFL1.1 đã có ở source. Lock chỉ thêm
`expo-splash-screen ~57.0.9` cùng metadata công khai, không đổi/bỏ package
khác. Safe chỉ cho hai hash trên, không thay dependency security audit.

Wrapper Impeccable cũ không tìm được plugin, **exit127**; lượt thực dùng
launcher hiện đang ship, engine **0.1.9**, rendered frozen Opening:
**5 mobile / 5 desktop findings, exit2**, không ignore. Theo review, ba
`transition: padding` còn **unresolved về detector**: source startup không
thêm animation padding, frame không cho thấy đổi padding. `text-occlusion`
được giải thích bằng status dưới cover bị che có chủ đích, scene inert/ẩn
AX, ảnh cuối chỉ một status đọc được; `monotonous-spacing` non-material cho
mark + status gap24/12. Không gọi detector sạch hoặc so trend CP08 khác
scope/version; không canonize findings thành luật thiết kế.

Concern QA riêng: privacy caption Login narrow/font1.3 bị cắt tại navigation
bar. Login source không đổi, thiếu baseline cùng cấu hình nên chưa quy
nguyên nhân/đóng accessibility hoặc inset. Signed-in release, iOS, máy vật
lý, TalkBack và production API/signing chưa kiểm; Back CLI không chứng minh
predictive-back gesture. Verdict startup không phải QA_ACCEPTED hay duyệt main.

Checkpoint tại lúc bàn giao:

- [x] Build bundled, hash APK/source và asset provenance có bằng chứng.
- [x] Retest guest/Login/Back đúng các cấu hình trên; mở nhìn khung chuyển động native.
- [x] Detector có findings đã triage, finish fresh `ship`, documenter ghi pattern/giới hạn.
- [x] Gate cây sạch tại `6144ff2c`: mobile1479/0/0+typecheck/ba export; native12flow1/0/0; identity trước/sau2/0/0 và canary/hai mutant cùng harness đỏ đúng dự đoán.
- [ ] QA độc lập nhận UI-009; cold có phiên/link hợp lệ, TalkBack, iOS và máy thật.

Retest trên **AVD tổng hợp riêng**, cài đúng APK/fingerprint trên, không gửi
OTP/redeem lời mời. Đặt `QA_AVD_SERIAL` thành serial AVD ấy. Hai YAML là
flow đã chạy, không đủ tự đóng các ô còn trống:

```bash
maestro --device "$QA_AVD_SERIAL" test /home/lakiet/.local/share/rudi-b9a/campaign/cp09/splash-cold.yaml
maestro --device "$QA_AVD_SERIAL" test /home/lakiet/.local/share/rudi-b9a/campaign/cp09/splash-compact.yaml
```

| Retest tiếp | Thao tác / expected |
|---|---|
| Cold guest sáng | Cold YAML; quay từ trước launch và mở khung trước JS/handoff. Cùng mark/indigo đến Welcome, không Expo/trắng/entry slide; recovery CTA/Quay lại giữ đích. |
| Narrow dark/font1.3/reduced | Cấu hình 840×1840/density420/font1.3/night yes/scale0 rồi compact YAML: Welcome/CTA trong safe area, Login/Back không slide. Ghi riêng caption Login cắt, so baseline để điều tra. |
| Tablet normal | Cấu hình 2016×2688/density420/font1.0/light/scale1 rồi compact YAML; mở khung giữa CTA/Login/Back, nền thuộc hai màn hiện hành; ảnh cuối không thay motion proof. |
| Có phiên/link hợp lệ/TalkBack | Fixture tổng hợp có phiên, stop rồi mở icon/link địa điểm hợp lệ: giữ đích/quyền/state; TalkBack không vào controls dưới cover, chỉ status rồi đích đúng. Chưa có runtime evidence cho các ca này. |

B10 đã triển khai đủ **9 ID độc lập**: 002/009, 016/017/020,
008/011/012/014. B9a có thêm 150/151/152/154 trong nhánh riêng. Đây không
phải kết luận 167 ID đã hoàn tất: **18 B8 + 26 B9 vẫn RESERVED_CLAUDE**,
109 ID kế thừa chưa được Codex xác nhận QA độc lập, UI-096 được thay bằng
UI-158. Các ADR UI-073/119/131/149 và điều kiện HTTPS UI-136 vẫn giữ giới
hạn nghiệp vụ/hạ tầng; không sửa rule để đổi màu trạng thái.

| Phép kiểm | SHA / kết quả / phạm vi |
|---|---|
| Clean mobile cuối | `6144ff2c`: npm ci đúng lock, typecheck + **1479 pass / 0 fail / 0 skip**, export **Android/iOS/web**, 86s; `mobile-6144-repeat.log`/summary. Lượt đầu1478/1/0 và concern không ổn định giữ phía dưới; không sửa assertion/source Hành trình. Lượt `b46bdcf7` giữ lịch sử. Export iOS không phải chạy iOS. |
| Hướng dẫn Nếp | `dc1bd772`: generator đọc 60 route, chỉ sáu nhãn Welcome đổi; băm frontend/Go `99be676b0199`. Test web hướng dẫn **29/0/0**, Go package đạt. Không đổi route hay cập nhật snapshot bỏ qua review. |
| API trong full gate | `dc1bd772`: **3028 pass / 743 skip**. Không đọc skip thành bằng chứng PostgreSQL. AI inference offline đúng pin/env **65 pass / 3 deselected**, không gọi provider. |
| Identity, canary và mutant | Clean `6144ff2c`: identity trước/sau **2/0/0**; canary bỏ cover đỏ tại chờ first frame; mutant bỏ inert đỏ tại protected scene; mutant CTA luôn Welcome đỏ tại chờ home signed-in. Cùng hash opening/chrome-cdp/freshness, mỗi variant export mới; kiểm không tương đương trước chạy, khôi phục tree sạch. `mutations-6144ff2c/mutations.json`: identity2/0/0 trước/sau; web frame3.8/8.6ms từ resume request. Canary bỏ cover không có paint probe, đỏ trước khi đọc frame; hai mutant đỏ đúng assertion/đích. Lượt `b46bdcf7` giữ lịch sử; các diff/log ngoài repo. |
| Full strict 33 chặng | Clean `dc1bd772` **đã kết thúc: 26 pass / 7 fail / 0 skip**, `gate-final.log` + `gate-final-summary.txt`, pin-drift clean. Đỏ: ruff (nhánh không đổi Python), demo-watch/hero-walk (thiếu demo/8099), mobile/mobile-native (lượt cũ; xanh riêng tại `b46bdcf7`/`c8328b0e` ghi bên dưới), Go PostgreSQL (`internal/nepnho` xóa tài khoản: registry thiếu `community_notifications.actor_id`), crypto (thiếu cargo). Crypto rerun riêng cùng SHA: 48s đạt1/0/0 nhưng bỏ dựng Android; lượt hai `crypto-android-final.log` 16s đạt1/0/0, đúng NDKr27b có compile Android; root kiểm ELF Android và đủ7 symbol, **21 test (4+17)**. Đây là build, chưa runtime E2EE. Giữ nguyên bảy chặng đỏ cũ. Parity dev **351/10758**, limiter **9/209**, production **23/605** scenario/step, đều **0 diff**; PostgreSQL live **702 + 51 pass**. Không sửa `dangky.go` original đang có patch Claude để khép Go PostgreSQL. Chưa có full strict xanh hay gate trên merge-result với main mới; **chưa đủ điều kiện landing main**, không phải gate cuối của UI-009 tại `6144ff2c`. |

Ba sửa harness đều giữ assertion và dữ liệu:

- `65a306e0`: đợi chip Café demo thực sự render trước đo gutter; dock có
  mặt không chứng minh dữ liệu danh mục đã về.
- `4e62573c`: đợi `opening-app` biến mất rồi mới điều khiển Nếp bằng phím;
  scene có DOM dưới bìa không chứng minh scene tương tác được.
- `b46bdcf7`: sau chọn chặng, đợi đúng selected-stop **và** rail
  `aria-pressed=true` rồi mới chuyển sang Hành trình. Tên chặng vốn có trên
  rail chưa chọn nên predicate cũ có thể qua quá sớm. Source Hành trình
  không đổi; concern CP06 được khép bằng lượt full clean **1479/0/0** sau
  assertion mạnh hơn. Lịch sử hai lượt CP06 đỏ và baseline1473 xanh giữ nguyên.

### Concern mới từ gate UI-009

Lượt clean mobile đầu tại `6144ff2c`: **1478 pass / 1 fail / 0 skip**,
ca Hành trình chờ selected-stop/`aria-pressed=true` hết hạn, kết quả cuối
`pressed=false`. Source Hành trình không đổi. Lượt chạy riêng **1/0/0**,
lượt full test lặp **1479/0/0**, rồi lượt có instrumentation cũng **1479/0/0**;
trace thành công cho thấy cover đã đóng khi pointerdown/up/click. Không
quan sát được trace của lần đỏ, nên chưa xác lập nguyên nhân và không
được nhận đã sửa lỗi này. Instrumentation chỉ ở verifier, đã khôi phục
harness nguyên bản trước gate tiếp; giữ `journey-diagnostic.diff`, các log
`mobile-6144-journey-*` và `journey-6144-isolated.log` ngoài repo.
Đây là concern không ổn định của kiểm chứng để QA/theo dõi tích hợp,
không đánh tráo thành bug QA gốc hoặc sửa/tắt assertion để xanh.

Native cuối tại clean `6144ff2c`: **12 flow**, **1 stage pass / 0 fail /
0 skip**, **830s**, `native-6144.log`/summary. Dấu vân sai đỏ đúng assertion;
canary09 tới Tài chính rồi đỏ ở chuỗi không tồn tại bước cuối. Metro riêng
8175 ở verifier không bị mutation; mã nguồn8/8 khớp APK debug dựng/cài.
Debug APK SHA256 `f47d6a4d5a327b10b3ba30dadd7a5e6276c408de8f741e7fc6998740beb444b2`,
`splash-debug-apk-verification.json`. Thư mục Android ignored chỉ nhận
fingerprint sau đối chiếu; không giả build trong verifier. Đã mở contact
22 ảnh (giữ hai file cùng tên từ hai flow thành đường dẫn riêng), ảnh
Welcome/trang2 đầy đủ tại `native-6144-images/`. Đây là default fixture,
không OTP/AI có điều kiện/Expo Go/máy thật hoặc 167 ca QA.
Repo guard tree tại clean `6144ff2c`: **4837 file** đạt;
staged code trước commit **9 file** đạt. Asset evidence docs sẽ có cổng
staged/tree riêng sau commit bàn giao.

### Android, APK và link

Android warm (5600) mở địa điểm → Back → unknown → Back → invite; guest
(AVD riêng5620) unknown → CTA Welcome → invite. Hai CLI hoàn tất exit0,
ảnh đã mở nhìn; mã CP09-TEST-ONLY chỉ điền, không redeem. Web anonymous
có/không browser history nằm trong hai test opening. Lượt debug cold
custom scheme vào devclient launcher trước JS giữ nguyên giới hạn ấy;
không dùng lỗi launcher để suy kết quả release.

APK **release cục bộ** tại `8ac64443` đã được assembleRelease x86_64,
**957 task** (898 executed, 59 up-to-date), build thành công trong 3m27s.
APK cài trên AVD tổng hợp riêng có SHA256 khớp bản dựng:
`8f4ba7957c76b8936e91a86d5bc65a89dca4ffc89f4ea7fb78281e20363c3053`.
Bundle nằm trong APK, không dùng Metro. `native-release-build.log`,
`native-release-apk-verification.json` và `native-release-cold.log` giữ
bằng chứng; CLI **exit0** cho các ca guest:

- Cold URL lạ mở trang recovery; CTA và nút Quay lại trong UI về Welcome.
- Cold `rudi://moi/CP09-TEST-ONLY` giữ đúng mã, không tự redeem.
- Stop rồi mở từ icon về Welcome, đúng fingerprint release tổng hợp.

Năm ảnh trong `release-native/2026-10-03_050224/release-cold/` đã mở nhìn:
[recovery lạnh](/home/lakiet/.local/share/rudi-b9a/campaign/cp09/release-native/2026-10-03_050224/release-cold/takeScreenshot/release-cold-unknown.png),
[CTA về Welcome](/home/lakiet/.local/share/rudi-b9a/campaign/cp09/release-native/2026-10-03_050224/release-cold/takeScreenshot/release-cold-unknown-cta.png),
[Quay lại](/home/lakiet/.local/share/rudi-b9a/campaign/cp09/release-native/2026-10-03_050224/release-cold/takeScreenshot/release-cold-unknown-back.png),
[lời mời](/home/lakiet/.local/share/rudi-b9a/campaign/cp09/release-native/2026-10-03_050224/release-cold/takeScreenshot/release-cold-invite.png)
và [mở từ icon](/home/lakiet/.local/share/rudi-b9a/campaign/cp09/release-native/2026-10-03_050224/release-cold/takeScreenshot/release-cold-launch.png).
Nội dung/action và safe area đúng tại cấu hình đã chạy. Đây là signing
debug mặc định của build release cục bộ, không chứng nhận signing/profile
production. API dùng loopback tổng hợp; chưa kiểm authenticated release
network, cold có phiên, link địa điểm hợp lệ lạnh hoặc startup ≤300ms.
Không gọi đây là QA_ACCEPTED hay bằng chứng iOS.

Emulator cũ đã khôi phục **14 private file**, băm nội dung khớp backup mới
trước lượt này; font/system scales/size/density và reverses trả lại.
`device-restore.json` giữ bằng chứng, không pm-clear thiết bị cũ. AVD tổng
hợp mới `rudi-b10-clean` là thiết bị duy nhất chịu pm-clear/reboot thử nghiệm.

APK debug đã được **prebuild + assembleDebug x86_64** từ B10, **568 task**,
build thành công; cài lại thành công trên AVD mới. Fingerprint native đúng
hai file package/app config (28be9856… / 1465aa33…); APK SHA256 và build log
ở `native-apk-sha256.txt`, `native-prebuild.log`, `native-build.log`. Lượt
full gate trước đó dùng APK kéo từ thiết bị, không có chứng nhận matching;
không xóa giới hạn ấy bằng kết quả APK mới. Fingerprint Metro là cổng riêng.

Native full gate đầu bị ngắt sau mất transport/launcher, chưa chứng minh
regression sản phẩm. Chẩn đoán sau trên APK mới thấy Metro sống nhưng
reverse port biến mất; chưa xác lập tác nhân gây mất binding. Dùng host
emulator 10.0.2.2 đã mở đúng Welcome/dấu vân, nhưng Maestro còn báo
`DeviceServerDiedException`/device offline ở port5620. Khi chuyển AVD riêng
sang port5560 có System UI ANR phủ app; đã lưu ảnh và đóng hộp hệ thống,
không sửa/tắt assertion để bảng xanh.

Sau ANR, cùng các YAML gốc: **flow00 smoke và flow01 Welcome/auth hoàn
tất** trên APK mới, có ảnh Welcome/trang2/Login/Sở thích. Cả bảng rút gọn
vẫn **exit1**: canary dấu vân bắt đầu từ Sở thích sau flow01, đỏ trước
assert dấu vân. Không gọi canary đó là đỏ đúng dự đoán.

`c8328b0e` thêm `MOBILE_METRO_HOST_NATIVE` opt-in vào harness, mặc định
localhost giữ nguyên; host được dùng đồng thời cho URL bundle và packager.
Host dạng URL bị từ chối exit64 trước thao tác thiết bị; bash syntax và
staged guard đạt. Không đổi assertion, app behavior hoặc API host. Bảng
smoke từ app tổng hợp sạch tại `c8328b0e` **exit0**: flow00 đúng dấu vân;
dấu vân sai đỏ đúng assertion; canary09 đi hết flow tới Tài chính rồi đỏ
đúng chuỗi không tồn tại ở bước cuối. `native-tracked-host-smoke.log` giữ
lượt đo. Ảnh Welcome và trang2 trên APK mới đã mở nhìn. Đây là bảng một
flow + hai negative control, không phải toàn bộ native gate.
Clean native gate mặc định tại `c8328b0e` **1 stage pass / 0 fail / 0 skip**,
**12 flow**, 874s; dấu vân sai đỏ đúng assertion, canary09 đỏ đúng bước cuối.
`native-clean-c832.log` và `native-c832-summary.txt` giữ verdict. Đây là
fixture/default gate: chưa chạy Expo Go với host tùy chọn, thiết bị vật lý
hoặc các flow OTP/AI có điều kiện. Không suy 12 flow thành 48 flow hay đủ
167 ID. Verifier kiểm package/app config byte-identical với APK đã dựng/cài,
chỉ sao fingerprint vào thư mục Android ignored, không giả một build mới.
Test meta harness tại SHA này: **8 pass** với toolchain đúng; lượt trước
**6 pass / 2 fail** vì thiếu Node trên PATH giữ riêng, không sửa test để xanh.

Video `native-nep-gate-c832.mp4` (99,89s, Android thật) đã được giải mã và
mở nhìn: sheet đi lên cùng scrim, Back đưa panel xuống ngoài khung rồi trở
lại đúng Khám phá; dock cất lại, gesture không làm app sập. Montage
`native-nep-transition-c832-frames.png` và
`native-nep-exit-final-c832-frames.png` có khung trung gian enter/exit;
không suy tần suất lấy mẫu thành FPS. Đầu recording có khung trắng của
devclient trước JS khi stop/launch; **không chứng nhận native startup
≤300ms** hoặc đồng nhất nó với slow-session cover đã đo trên web.

### Review thiết kế và QA cần làm tiếp

Hai assessment độc lập Nếp: UX **28/40**, native kỹ thuật **15/20**.
Ba P2 giữ riêng để iteration kế: độ hẹp disclosure khi font lớn; cue phân
biệt hỏi/vẽ tại editor; lối tiếp sau lỗi chức năng chưa bật. Đây là findings
mới của critique, không gán thành bug QA hoặc bịa trend điểm tăng.
B10 giữ đúng phạm vi quyền/chia sẻ và hai hành động hiện hành; không thêm
mode mới/tắt hành động theo chuỗi lỗi. Hai sửa finish đã có recapture và
verdict ship trong phạm vi đó; chưa có P0/P1 được assessment xác nhận,
không suy thành toàn app sạch.

Detector **23/36** là scan frozen DOM trước batch fix footer. Recapture
cuối chứng minh hai sửa finish; JSON không phải scan exact SHA cuối.
Không inject overlay vì browser evaluate read-only; metadata/design drift
và contrast incomplete giữ nguyên. Video/khung chuyển động native CP06–08
đã mở nhìn; không bịa FPS, frame time hoặc motion từ screenshot tĩnh.

QA retest theo các hàng CP06/07/08 phía trên, ưu tiên:

- 002/009: slow session, guest/có phiên, Back và CTA; cold guest trên
  release cục bộ đã tự kiểm. QA cần cold có phiên/link địa điểm hợp lệ,
  state preservation và startup native ≤300ms trên bản ship.
- 016/017/020: swipe nhanh chỉ một trang; phím/resize/pager state;
  double tap CTA, Back giữa transition và reduced motion lạnh/warm.
- 008/011/012/014: tên/nút skip của performance, chip48dp, gutter320;
  keyboard/font lớn, mô tả dài, Sửa/Back/Escape/đóng không POST;
  confirm một POST, pending/error giữ draft, shared Sheet ở caller khác.
- Provider/media success, TalkBack, iOS, thiết bị vật lý, native dark và
  landscape còn thiếu runtime; không nhận QA_ACCEPTED từ tự kiểm.

Bó CP09: `/home/lakiet/.local/share/rudi-b9a/campaign/cp09/`; liên kết
[checklist 167 ID](ui-ux-campaign.md). Không sửa QA report gốc, session/plan
Claude, PRODUCT/DESIGN/sidecar hoặc patch B8/B9. Main đã tiến tới `f043bea4` trong lane khác. Merge-tree chỉ đọc tại
`6144ff2c`/`f043bea4` exit0, không conflict văn bản; chưa merge hay chứng
minh tích hợp semantic/gates trên main mới.

Lượt cleanup sau UI-009: xác nhận đúng AVD riêng `rudi-b10-clean`/5560,
size/density không override, font1/animator1; dừng AVD, Metro8175 không còn
listener. `native-ui009-cleanup.json` giữ kết quả. Các thiết bị/data Claude
không thao tác trong lượt này.
