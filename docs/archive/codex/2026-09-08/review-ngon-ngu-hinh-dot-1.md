Method: dual-agent (A: /root/visual_review_a · B: /root/evidence_review_b); Codex kiểm lại source và thao tác native độc lập.

# Review đợt 1: giữ hướng «Chừa một chỗ cho nhau», chưa mở rộng full

**VERDICT: REQUEST_CHANGES — áp dụng cho cổng nhân rộng ngôn ngữ hình.**

Ngày 08/09/2026. Đối tượng: [yêu cầu review của Claude](../../claude/2026-09-08/hoi-review-ngon-ngu-hinh.md) và [báo cáo đợt 1](../../claude/2026-09-08/bao-cao-ban-sac-dot-1.md), đối chiếu audit 07/09 §13.1 và §13.3 *(tài liệu local của Codex `docs/archive/codex/2026-09-07/…`, chưa đưa vào cây)*.

Checkout `/home/lakiet/mobile`, nhánh `main`, HEAD đầu/cuối `16ac824dac0b2386a42e30a75026185e4482fbbb`. Lịch sử local cho thấy đây là merge PR #582, gồm bản hỏi review `0971113f`; không còn ở nhánh ghi trong đầu doc. Không kiểm trạng thái GitHub trực tuyến, không đăng verdict PR hay thay đổi merge. `protocol_version: v1` giữ nguyên, không thuộc phạm vi thay đổi của review UI.

## 1. Trả lời câu hỏi của Lead

**Đã tốt hơn, đúng hướng ở nền tảng, nhưng chưa đủ đẹp và đủ kể chuyện để trải bộ hình hiện tại ra toàn app.** Tôi giữ lựa chọn có Nếp nhưng tách lớp, lưới Sở thích, bảng màu, nét Gu và bố cục Khám phá có cặp so sánh. Tôi chưa nghiệm thu bộ diễn xuất Nếp, phần ảnh và Album kể chuyện theo ngày.

Điều còn thiếu là quan hệ nhìn thấy được: ai đang kéo ghế cho ai, ai giữ khung ảnh, ảnh nào thuộc ngày nào. Một nhân vật đứng cạnh một đồ vật mới tạo ra hai thành phần trong tranh. Động tác, điểm tiếp xúc và hướng nhìn mới cho người xem thấy chuyện gì đang xảy ra.

Không yêu cầu quay lại thiết kế từ đầu. Lượt tiếp theo nên đóng bốn nhóm finding dưới đây trên cùng lát cắt; sau đó review một batch xác nhận rồi mở B2/B3/C2. Không vẽ đủ 41 cảnh, tám sticker và toàn bộ huy hiệu trước khi sửa ba cảnh đại diện.

**Phạm vi công bằng:** báo cáo đợt 1 ghi Lead đã giao A1+A2+B1; A3 ngoài đợt là giới hạn được khai báo, không phải đội âm thầm bỏ việc. Nhưng cổng mở rộng ở §13.1 bao gồm A3. `ship` hẹp của finish-reviewer không thể thay điều kiện đó.

## 2. Bằng chứng đã xem và đã kiểm

- Hai assessment tách biệt: A xem thiết kế/source và hình trước khi B gửi detector; B kiểm source và detector. Codex tổng hợp, mở lại các hình quan trọng và tái hiện riêng trên emulator.
- Đã xem đủ tám PNG được dẫn trong doc Claude: ba cặp trước/sau, A/B native, hai bảng năm cảnh và hai bảng art. Đã xem concept Nếp selected *(file local ngoài cây: `nep-concept-01-selected.png`)* cùng README/spec *(concept local của Codex `docs/archive/codex/2026-09-08/nep-concept/`, chưa đưa vào cây)*, các ảnh native cuối sáng/tối 1.3 mà team cung cấp. Bảng art là preview, không phải mọi pose đã được native nghiệm thu.
- Native mới: `emulator-5554`, Android, `com.lakiet.rudi`, 1080×2400, density 420 (khoảng 411dp ngang). Metro từ `apps/mobile`, cổng riêng 8097, không nạp `.env`, cửa demo bật, API đặt về cổng 18999; không dùng tài khoản/người tham gia thật.
- Ảnh 01 *(ảnh đã kiểm local, không nằm trong PR: `01-welcome-session.png`)* thấy dấu phiên `codex-review-sep08-16ac824d`; Metro có `Android Bundled`. Dấu phiên chứng minh lượt nạp, không phải checksum JS/APK. Dùng APK đã cài; không rebuild, không chứng nhận native binary khớp toàn bộ source.
- Kiểm mới ở sáng/font 1.0: chọn ba gu → Tiếp tục → Khám phá, cuộn cặp so sánh, mở Album bằng route ấm, chạm ảnh. Kiểm sáng/font 2.0: Sở thích đầu/cuối, Khám phá lead/cặp/hàng và tab bar. Font đã khôi phục 1.0, theme sáng giữ nguyên; Metro review và reverse 8097 đã dừng/gỡ.
- Manifest *(ảnh đã kiểm local, không nằm trong PR: `manifest.json`)* ghim checksum 25 artifact PNG/XML. Có 12 PNG bề mặt app và một PNG lỗi môi trường. Ảnh 08 ghi lỗi kết nối Metro sau khi tiến trình dừng, **loại khỏi bằng chứng lỗi UI**; đã mở lại Metro và nạp lại trước khi chụp 09–13.
- Detector B: chạy một lần trên `apps/mobile/src/rudi`, exit 0, JSON `[]`. Không có findings/rule/location. Bộ quét không xác nhận nội dung ảnh, diễn xuất nhân vật, bố cục native hoặc TalkBack; không có browser overlay.
- Phép thử ngày: trích helper và biểu thức đang có từ source bằng TypeScript AST, chạy dữ liệu tổng hợp trong bộ nhớ; kết quả ở §4.3. Không thay product hoặc dựng đáp án giả trong repository.

**Chưa kiểm:** OTP/API E2E của ExploreLive/AlbumLive; no-photo/image-error native; iOS; TalkBack/VoiceOver; máy rộng của bản mới; hiệu năng release/motion. Dark 1.3 dựa trên ảnh team, không phải lượt chạy mới của Codex. Không coi các khoảng chưa kiểm là tính năng đã hỏng.

## 3. Những phần đã đạt hướng và nên giữ

| Bề mặt/lớp | Đánh giá sau khi nhìn hình | Quyết định |
|---|---|---|
| Sở thích | Bớt tầng chữ, tám đồ vật có nét chung, chọn có check và màu. Ở 2.0 một cột, nhãn đầy đủ, cuộn tới ngân sách/nút được | Giữ grid ổn định; không rải hoặc xoay touch target |
| Khám phá | Nhịp lead → hai ứng viên → hàng là thay đổi công năng có giá trị. Chip dùng cùng nét Gu, facts so sánh dễ nhìn ở 1.0 | Giữ cấu trúc, sửa nội dung ảnh và khả năng co giãn |
| Album fixture | Bớt lặp nhóm/ngày/nơi, ảnh có không gian tốt hơn | Giữ phần rút chữ; chưa coi đây là bằng chứng nhịp ngày live |
| Tầng art | Hình học tách màu/render, giữ ID, có lưới và bản nhỏ, dark giữ nét rõ | Giữ kiến trúc; sửa hình và cách đặt nhân vật |
| Tông cảm xúc | Giấy, mực, coral tiết chế hợp cuốn sổ hiện tại; không cần một palette mới | Nâng chất lượng quan hệ trong tranh và biên tập ảnh |

Sở thích sau chọn *(ảnh đã kiểm local, không nằm trong PR: `03-preferences-selected.png`)*, Sở thích font 2.0 đầu *(ảnh đã kiểm local, không nằm trong PR: `09-preferences-font2.png`)*, cuối *(ảnh đã kiểm local, không nằm trong PR: `10-preferences-font2-bottom.png`)*.

Ba màn đã khác nhiệm vụ. Không bắt mỗi màn phải thay silhouette so với chính nó trước đây nếu silhouette đó đang giúp thao tác. Cần cùng thế giới, không cần cùng cấu trúc.

## 4. Các finding cần giải quyết trước cổng full

### F01 — P1: ảnh sai ngữ cảnh vẫn chi phối toàn màn Khám phá; A3 chưa đóng

**Bằng chứng:** cặp so sánh native mới *(ảnh đã kiểm local, không nằm trong PR: `05-explore-compare.png`)*; `apps/mobile/src/rudi/fixtures.ts:73–109` gán kiến trúc kính cho Tiệm Nướng, ảnh hai người cho Bánh căn, mặt gỗ cho Lẩu gà. `Discovery.tsx` chuyển nguồn này vào component. Fallback chỉ chạy khi nguồn ảnh vắng, nên bộ art hiện không cứu được các mapping sai.

**Hậu quả:** ảnh dẫn không gợi được đúng món/nơi; người xem phải tự bỏ qua hình để hiểu tên. Con dấu hợp gu và câu lý do khiến hình sai càng có vẻ là bằng chứng của gợi ý. Nhãn demo không khắc phục nhược điểm này trong một vòng nghiệm thu gu.

**Yêu cầu:** gỡ mapping sai khỏi fixture. Nơi thiếu ảnh đúng dùng đồ vật theo loại và facts. Điều chỉnh type/adapters/các consumer fixture để nguồn null là hợp lệ; không ép kiểu hoặc chỉ sửa một màn khiến detail vẫn cần ảnh bắt buộc. Không chuyển sang một bát lẩu stock rồi ngầm gọi đó là ảnh của chính quán.

Gỡ ảnh chỉ hoàn tất tính trung thực. Để nghiệm thu sức gợi của nhánh có ảnh, chuẩn bị bộ demo có nguồn và đúng quan hệ (chính địa điểm / quanh đây / minh họa), hoặc dùng các địa điểm tổng hợp được ghi rõ. Giấy phép hợp lệ và ảnh đúng đối tượng là hai điều kiện riêng. `B cho live` không tự làm bộ demo đang review trở nên tốt.

**Điểm phải nhìn sau khi gỡ:** `PlaceCompare` vẫn dành khung 4:3 cho fallback, `PlaceGlyph` vẫn là glyph trong đĩa nền. Nếu cả catalogue không có ảnh, hai khung biểu tượng lớn có thể chiếm chỗ mà không giúp chọn. Dùng bố cục biên tập gọn theo tên/loại/facts khi không ảnh, đúng §9.4.

**Trạng thái liên quan, mới xác nhận từ source:** `PlaceRow` dùng `Image` trực tiếp ở `HangDiaDiem.tsx:123`, AlbumLive ở dòng 200/224; chưa có `onError` tại các ảnh này. `MediaSlot` báo lỗi tải ảnh nhưng không thay ảnh lỗi bằng fallback. Kiểm source có URI nhưng tải lỗi riêng với source null.

**Gỡ chặn:** ba đối tượng đầu có quan hệ ảnh đúng, manifest nguồn/crop ở các cỡ dùng thật; native nhánh có ảnh/không ảnh/ảnh lỗi đều đọc được và giữ đúng credit, không lấy ảnh nơi khác để lấp. Gợi ý công việc: `/impeccable harden` media và `/impeccable layout` no-photo.

### F02 — P2 về usability, chưa đạt duyệt mỹ thuật: Nếp chưa diễn hành động của câu chuyện

**Bằng chứng:** [năm cảnh sáng](../../claude/2026-09-08/canh/canh-nam-sang.png), [tối](../../claude/2026-09-08/canh/canh-nam-toi.png); `art/canh.ts:85–90` dùng pose `moi` cho bốn trong năm cảnh. `art/nep.ts:71–101` giữ mặt và chân cố định, khác biệt pose chủ yếu ở tay.

- «Chưa có hội»: tay chỉ về ghế nhưng chưa chạm ghế, chưa thấy động tác kéo/chừa chỗ.
- «Chưa có bạn»: vị trí chân khiến Nếp đọc như đứng trên lưng ghế trái. Không có mặt phẳng/động tác ngồi thuyết phục.
- «Chưa có ảnh»: cùng dáng chìa tay cạnh khung; chưa thấy hành động giữ/chuyền.
- «Tìm không ra»: cùng dáng chìa tay bên bản đồ; mô tả «cầm bản đồ» của doc mạnh hơn hình đang thể hiện.

**Nhận định mỹ thuật:** thân vẫn gần icon tài liệu có mặt và chân que. Bản 48 cần đơn giản, nhưng bản 96/144 còn thiếu trọng tâm thân, bàn tay và hướng mắt đủ để có cá tính. Không đòi giữ grain, 3D hoặc mọi chi tiết của concept raster; cần giữ động tác quan hệ vốn là điểm mạnh của concept.

**Sửa một nhóm ba cảnh:** kéo ghế; xem/giữ bản đồ; giữ/chuyền khung ảnh. Tay chạm vật hợp lý, chân có mặt phẳng, thân nghiêng theo việc làm, hướng mắt cùng chủ thể. Sửa vị trí Nếp ở cảnh bạn. Chưa cần vẽ lại đủ mọi pose.

**Tắt Nếp:** tôi không coi hai cảnh cùng dùng ghế là lỗi tự thân. Một ghế được kéo ra và hai ghế đối diện cùng chiếc bàn có thể là họ hàng có nghĩa. Hãy phân biệt quan hệ và khoảng trống, không cố thêm đồ vật lạ để đủ năm silhouette. Việc tách mảng bằng code chỉ chứng minh tháo lớp được; chưa chứng minh bố cục sau khi tháo đã hay.

**Gỡ chặn art:** xem ba cảnh ở 48/96/144dp, sáng/tối, trong context và có/không Nếp. Reviewer chỉ được điểm tiếp xúc và hành động từ hình, không cần đọc thuyết minh. Native một cảnh nhỏ không lỗi nét/giao nhau. Đây là điều kiện duyệt hướng mỹ thuật do Lead yêu cầu trước nhân rộng; không gán thành P0 hay vi phạm domain. Gợi ý: `/impeccable shape` ba cảnh rồi `/impeccable polish` nét.

### F03 — P2 và khoảng trống bằng chứng: Album chưa chứng minh đúng phần kể chuyện mới, có hai lỗi cụ thể

**Bằng chứng khác renderer:** `Memories.tsx:194–196,279` vẫn ảnh lead + ba thumbnail; code theo ngày/ảnh lẻ rộng ở `AlbumLive.tsx:196–240`. Ảnh fixture trước/sau chứng minh giảm chữ, không chứng minh bố cục live này. B1 nên ghi «fixture/copy đạt phần đã xem; live còn chờ», thay dấu xong phủ cả câu chuyện Album.

**F03a — mất nhãn hai ngày:** `nhomTheoNgay(photos.slice(1))` loại ảnh đầu trước khi `nhieuNgay = theoNgay.length > 1`. Với hai ảnh ở hai ngày, phần còn lại chỉ có một nhóm; mọi nhãn ngày bị ẩn, ảnh lead cũng không mang ngày.

| Ca tổng hợp chạy từ source hiện hành | Số ngày của toàn bộ ảnh | Số nhóm sau lead | Có hiện nhãn ngày theo điều kiện hiện tại? |
|---|---:|---:|---|
| 17/10 12:00 +07; 18/10 12:00 +07 | 2 | 1 | Không |
| 17/10 23:59:59 +07; 18/10 00:00:00 +07 | 2 | 1 | Không |
| Hai ảnh cùng ngày 17/10 | 1 | 1 | Không, hợp lý cho ca này |

Phép kiểm dùng helper và biểu thức trích AST từ source, không phải native E2E. Có thể chạy lại bằng `node docs/archive/codex/2026-09-08/review-ngon-ngu-hinh-evidence/kiem-ngay-album.cjs` từ gốc repo. Sửa số ngày dựa trên toàn bộ photos và gắn ngày lead khi cần. Ngày của chương và khoảng ngày chuyến đi là thông tin khác nhau; luật «nói một lần» không có nghĩa xóa ngày đang cần để hiểu ảnh.

**F03b — hứa mở ảnh nhưng chuyển sang chọn:** ở `Memories.tsx:216–225`, nhãn `Mở ảnh 1` gọi `setSelecting(true)` và `togglePhoto`. Tôi tái hiện native: trước chạm *(ảnh đã kiểm local, không nằm trong PR: `06-album-browse.png`)* → sau chạm *(ảnh đã kiểm local, không nằm trong PR: `07-album-tap-photo.png`)* thành «1 ảnh đã chọn», không mở viewer. XML trước chạm xác nhận nhãn. Không gọi đây là lỗi AlbumLive: live có đường mở viewer riêng.

Sửa để xem và chọn có entry rõ. Ưu tiên dùng viewer đã có cho chạm ảnh khi browsing, checkbox chỉ trong selection mode. Nếu fixture chỉ minh họa chọn thì nhãn phải đúng hành động và tài liệu không dùng nó nghiệm thu khả năng xem ảnh.

**Gỡ chặn:** chụp đúng `TripAlbumLiveScreen` và `ExploreLiveScreen` với dữ liệu tổng hợp: 0/1/2/nhiều ảnh; hai ngày với lead là ảnh duy nhất của ngày đầu; ngày chưa rõ; ảnh chẵn/lẻ; caption dài. Kiểm chạm đúng ảnh → viewer → back giữ context, sáng/tối/font lớn. Nếu OTP chưa sẵn, harness synthetic có thể mount đúng renderer để chứng minh native component; ghi đúng giới hạn, không gọi đó là API E2E. Gợi ý: `/impeccable harden` Album và `/impeccable audit` native lát cắt.

### F04 — P2: Khám phá mất dữ kiện quyết định ở chữ lớn, cả nhãn tab cũng bị cắt

**Đã tái hiện native mới ở font 2.0:** cặp so sánh *(ảnh đã kiểm local, không nằm trong PR: `12-explore-font2-compare.png`)*, các hàng *(ảnh đã kiểm local, không nằm trong PR: `13-explore-font2-rows.png`)*.

- Cặp: «40K - 80K/ngu…», «180K - 260K/n…», khoảng cách cũng cắt. Hai tên dài khiến trục facts không còn cùng hàng.
- Hàng: Still Cafe cắt cả tên và giá; nhiều nơi chỉ còn phần đầu dải facts, người dùng không thấy giá.
- Tab: «Khám …», «Lên pl…», «Tin nh…», «Cá nhâ…». Icon vẫn giúp nhận diện và accessibilityLabel còn đủ; chưa mất hoàn toàn điều hướng nhưng không đạt mục tiêu đọc rõ bằng chữ.

**Source:** `HangDiaDiem.tsx:141` khóa facts hàng một dòng; dòng 225–226 khóa facts cặp một dòng, layout dòng 239–240 luôn hai cột. `ui/RudiTabBar.tsx:88` khóa nhãn một dòng. Ảnh team tối 1.3 đã thấy lỗi giá hàng, nên đây không chỉ là vấn đề ở điều kiện cực đoan mới thêm.

**Yêu cầu:** giữ nguyên giá và đơn vị, cho facts xuống dòng theo cụm; khi hai cột không đủ chiều rộng chữ, chuyển cấu trúc so sánh phù hợp. Tách stamp khỏi phần tên cần đọc, ưu tiên tên/giá trước prose. Tab cần chiều cao/nhãn thích ứng hoặc tên ngắn có chủ ý được duyệt; không chỉ khóa scale để đạt ảnh đẹp.

**Gỡ chặn:** compact 1.3 và 2.0 vẫn thấy đủ tên cần phân biệt, dải giá và đơn vị, tab gọi tên rõ; CTA không che nội dung khi cuộn. Không cần mọi subtitle dài đều không cắt, nhưng phải giữ facts quyết định. Gợi ý: `/impeccable adapt` Khám phá và tab bar.

## 5. Trả lời các quyết định team đang chờ

| Câu hỏi | Kết luận review và việc tiếp theo |
|---|---|
| 1. Giữ Nếp A/B/C? | **A: có Nếp nhưng tách lớp**, tiếp tục hướng đã chọn trong README. Chưa duyệt bộ pose hiện tại để nhân rộng; sửa F02. ADR-0020 không đề cập mascot không đồng nghĩa tự động cấm mọi minh họa. Ghi quyết định mở rộng nhận diện và giới hạn vào nơi quản lý hướng trước rollout; không mở lại lựa chọn A/B/C chỉ vì thiếu một dòng trong ADR |
| 2. Motif ở populated? | **Có, chọn một nối kết có nghĩa trước:** góc gấp ở khung ảnh Album và mẩu lời rủ khi làm B2, cùng vị trí/tỷ lệ. Giữ check của Sở thích. Không nối các ứng viên quán bằng đường đi vì chúng đang là phương án thay thế, chưa là lịch trình. RouteLine nhận vai nối chặng; vòng hở Check-in chỉ trang trí và kiểm riêng |
| 3. Ảnh fixture? | **A ngay, B có biên tập để hoàn tất A3**, không C. Cho null đi hết adapter/consumer, kiểm no-photo native. Có bộ mẫu đúng nguồn/quan hệ để nghiệm thu nhánh ảnh dẫn; không chỉ hẹn nhập ảnh live sau này |
| 4. Silhouette Sở thích? | **Giữ lưới.** Tám nhãn và checkbox hợp tác vụ; native 2.0 đã thấy một cột đọc được. Tinh chỉnh art/spacing nếu cần, không rải đồ vật để tạo vẻ mới |
| 5. Tám sticker? | Chốt ba pose đại diện trước; thử **một sticker trong khay và bubble**, rồi mới vẽ đủ tám trong B3. Giữ tám ID hiện hành. `tra-tien-ne` là lời người gửi trong chat; ghi rõ khác với hệ thống xác nhận tiền, không dùng sticker làm trạng thái giao dịch. Không cần mascot ở mọi màn tiền |
| 6. Huy hiệu 6 hay 8? | **Theo tám ID đang dùng ở live**, fixture dùng lại cùng danh mục/semantics. Sửa cách viết «8 từ máy chủ»: `src/screens/thanh-tich/thanh-tich.ts:130–181` tính danh mục phía client từ Finance server; bốn đo được, bốn `chua-do-duoc`. Không âm thầm chuyển cái chưa đo thành đã khóa/chưa đạt. Không dựng thêm sáu hình độc lập chỉ vì tên fixture khác |
| 7. Token disabled và hai nợ khác? | Nên có lượt token disabled tập trung; label cần đọc được và state rõ. Làm qua tokens + script sinh các bản dùng chung. Ưu tiên PlaceRow trong F04. Tên quán lặp do fixture đã chứa nơi thì biên tập fixture, không bỏ rule phụ có ích ở live |
| 8. Live/iOS/TalkBack/release? | Đúng renderer live và trạng thái của lát là cổng chứng minh hướng. iOS, screen reader, release và thiết bị rộng vẫn phải có bằng chứng tương ứng trước tuyên bố sẵn sàng phát hành. Không cần làm toàn 41 màn để được review lại ba màn mẫu |

**Lưu ý về disabled:** nhãn mờ đáng sửa, nhưng không lấy riêng con số ~2:1 để kết luận vi phạm WCAG 1.4.3: văn bản thuộc control inactive có ngoại lệ trong tiêu chí đó. Tôi chưa đo lại tỷ lệ ~2:1 mà team báo. Nguồn kiểm ngày 08/09: [W3C: Contrast Minimum, Inactive UI Components](https://www.w3.org/WAI/WCAG22/Understanding/contrast-minimum.html#inactive-user-interface-components). Review này đánh giá khả năng đọc native, không cấp chứng nhận tuân thủ WCAG cho app.

**Điểm cần điều chỉnh cả ở đề xuất trước của tôi:** «Nếp đứng xa tiền» cần diễn đạt theo ngữ cảnh: không cạnh số đang kiểm, lỗi, conflict hay như dấu xác nhận hệ thống; sticker có câu về tiền là phát ngôn người gửi đã có ID. Và «ba motif» là ngữ pháp có thể dùng, không quota phải nhét cả ba vào mỗi màn. Đưa một góc gấp có nghĩa vào Album trước sẽ có giá trị hơn trang trí mọi chip.

## 6. Điểm heuristic và câu chuyện người dùng

Điểm của Assessment A từ hình + source, Codex giữ để làm tham khảo cho đúng lát; không tính mức tăng/giảm với audit toàn app cũ. Một số phần async/error chỉ được đọc source, chưa test live. Không dùng tổng điểm thay các điều kiện gỡ chặn.

| Heuristic | /4 | Nhận xét chính |
|---|---:|---|
| Trạng thái hệ thống | 3 | Chọn/check/count rõ; async live chưa khảo sát |
| Ngôn ngữ và thế giới thực | 2 | Copy nhẹ hơn, ảnh sai phá nghĩa |
| Quyền kiểm soát | 2 | Có back/skip, xem/chọn Album lệch |
| Nhất quán | 3 | Theme/Gu có họ hàng, Album khác renderer |
| Ngăn lỗi | 2 | Min gu/đơn vị có, state thật chưa kiểm đủ |
| Nhận ra thay vì nhớ | 3 | Nhãn gu rõ; giá bị cắt buộc tra detail |
| Hiệu quả thao tác | 2 | Có so sánh nhưng chữ lớn mất facts |
| Thẩm mỹ và tối giản | 2 | Bớt lời rõ, art chưa diễn, ảnh chưa đúng |
| Khôi phục lỗi | 2 | No-results có action, lỗi media/live chưa kiểm |
| Trợ giúp đúng lúc | 2 | Helper ngân sách gọn; khả năng khám phá còn giới hạn |
| **Tổng tham khảo** | **23/40** | Đánh giá chuyên môn tạm thời, không phải kết quả test người dùng |

**Độ đặc trưng:** đã có thế giới giấy và nét riêng, nhưng bản sắc quan hệ còn yếu ở động tác và nội dung ảnh. Không thể thay wordmark rồi coi đây hoàn toàn là app khác; cũng chưa đủ để nói bộ hình đã thành nhận diện trưởng thành có thể nhân rộng.

**Tải nhận thức:** Sở thích có tám lựa chọn nhìn thấy, nhưng đây là nhận diện nhãn, không phải nhớ tám dữ kiện trong đầu. Không dùng quy tắc «hơn bốn là sai» để bẻ grid. Khám phá có chủ thể lead và cặp so sánh tốt; khi facts bị cắt người dùng phải mở từng nơi để ghép đủ thông tin. Empty tìm không ra nói tình huống hai lần và có hai action xóa lọc; nên rút còn một cụm chính, mức polish.

**Ba người dùng dễ vấp:** người mới suy ra ảnh mặt gỗ là ảnh của quán; người dùng chữ lớn không thấy dải giá; người dựa vào nhãn trợ năng nghe «Mở ảnh» nhưng rơi vào selection. Chưa chạy TalkBack nên chỉ xác nhận nhãn từ source/XML và hành vi chạm, không khẳng định toàn luồng screen reader.

**Nhịp cảm xúc:** chọn gu đã nhẹ hơn → tìm nơi vẫn đứt vì hình sai → giữ kỷ niệm bớt chữ nhưng chưa chứng minh nhịp ngày. Nếp nên giúp mở lời/giữ chỗ ở những đoạn yên tĩnh; ảnh thật của hội và hành động người dùng phải tiếp quản câu chuyện ở màn đã có nội dung.

## 7. Cổng review lại và phạm vi lượt sửa

| Gói | Đầu ra cụ thể cho review lại | Qua khi |
|---|---|---|
| R1 · A3 | Mapping ảnh đã chỉnh + manifest quan hệ/nguồn/crop + native có ảnh/không ảnh/ảnh lỗi | Không còn ảnh sai đối tượng; composition thiếu ảnh vẫn có thứ để so |
| R2 · A2 | Ba scene hành động đã sửa, spec điểm tiếp xúc/pose, preview 48/96/144 sáng/tối và một native A/B | Nhân vật đang làm được việc được mô tả; không đứng/lơ lửng sai chỗ |
| R3 · B1 Album | Sửa nhãn xem/chọn và nhãn ngày, case hai ngày, ảnh đúng renderer live với synthetic | Xem mở đúng ảnh; hai ngày đọc ra hai ngày; caption và credit giữ đúng nghĩa |
| R4 · B1 adaptive | Ảnh Khám phá compact 1.3/2.0, cặp/hàng/tab đủ nhãn quyết định | Không cắt giá/đơn vị, tab rõ, action còn thao tác được |

Một dấu góc Album có thể làm trong R3 để thử nối câu chuyện populated; không bắt nhân vật xuất hiện ở cả ba màn. Disabled token có thể là lát kit riêng cùng lượt, không thành cuộc đổi màu toàn app. Sau batch sửa, đối chiếu cùng dữ liệu/font/viewport một lần, không kéo dài vòng «đẹp hơn chút nữa» bằng các yêu cầu mới không thuộc bốn gói này.

**Thông điệp gửi team:** giữ hướng hiện tại và giải quyết R1–R4 trước full. Tôi đồng ý với kiến trúc tách lớp Nếp và lưới Sở thích; chưa duyệt bộ hình hiện tại để nhân rộng. Tiêu chí hoàn tất là quan hệ trong tranh đọc ra được, ảnh đúng, nhịp ngày thật và thao tác/nhãn không lệch. Khi các bằng chứng đó đạt, chuyển tiếp B2/B3/C2 theo roadmap, không cần mở lại toàn bộ brand direction.

## 8. Giới hạn bàn giao và kiểm tra

Chỉ thêm báo cáo và bằng chứng local trong `docs/archive/codex/2026-09-08/`; không sửa product, tokens, ADR hay tài liệu của Claude, không commit/push. Ảnh mới là chụp dữ liệu demo/asset có sẵn. Chưa thêm allowlist binary vì chưa stage/commit; nếu đưa ảnh này vào Git cần ghim đúng path+digest qua quy trình repo guard, không bỏ qua scanner.

Kiểm đã làm: source đối chiếu các finding; native mục 2; phép kiểm ngày source hiện hành; detector qua B; kiểm liên kết tương đối và checksum artifact; `git diff --check`. Có thử lệnh test art trên `dist-test` hiện có nhưng chưa rebuild đầu vào ở lượt này, nên không dùng kết quả đó làm bằng chứng source hiện tại pass; không lặp các con số 738/738 hay 929 pass của team như kết quả độc lập.

Questions skipped: Lead đã xác định phạm vi và quy tắc «review kỹ, chưa đạt thì trả finding cho team trước full»; không cần hỏi lại ưu tiên hay xin quyền sửa product trong lượt review này. Đây là áp dụng yêu cầu trực tiếp của Lead thay cho câu hỏi kết thúc mặc định của skill.
