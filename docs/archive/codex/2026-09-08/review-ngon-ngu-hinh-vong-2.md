Method: dual-agent (A: /root/visual_review_a; B: /root/evidence_review_b). Codex mở hình, kiểm source và tái hiện native độc lập; A hoàn tất trước khi tổng hợp detector B.

# Review vòng 2: đã có câu chuyện, còn một đường ảnh chưa sửa hết

**VERDICT: REQUEST_CHANGES hẹp trước cổng full. Chấp nhận hướng mỹ thuật; đóng R2, R3, R4 trong lát Android đã kiểm. R1 còn thiếu ở các màn dùng chung ảnh.**

Ngày 08/09/2026. Đọc [bản trả lời của team](../../claude/2026-09-08/tra-loi-review-ngon-ngu-hinh-dot-1.md), đối chiếu [review vòng trước](review-ngon-ngu-hinh-dot-1.md) và roadmap §13 *(tài liệu local của Codex `docs/archive/codex/2026-09-07/…`, chưa đưa vào cây)*. Checkout `/home/lakiet/mobile`, `main`, HEAD `34258f2cfe092f069f68667a00853464386a7226` — lịch sử local đã có merge #583. Đây không phải verdict đăng lên PR, xác nhận CI trực tuyến hay phê duyệt phát hành.

## 1. Trả lời thẳng: đẹp, sinh động, có câu chuyện và riêng chưa?

**Đẹp hơn rõ và đủ cơ sở để phát triển tiếp, nhưng chưa thể nói “mọi thứ đã đẹp hoàn thiện”.** Không cần quay lại thiết kế Nếp hay ba màn từ đầu. Các thay đổi đáng giữ là cặp so sánh gọn khi thiếu ảnh, quan hệ tay–đồ vật, nhịp ngày Album và nhãn tab đọc đủ ở chữ lớn.

**Đã có câu chuyện nhìn thấy được.** Tay đặt lên ghế biến một đồ vật thành hành động mời; hai tay giữ khung ảnh biến khoảng trống thành thứ đang được giữ lại; bàn và hai ghế tạo một chỗ chờ người khác. Tắt Nếp vẫn đọc được vật và tình huống. Đây là thay đổi thực chất so với việc nhân vật chỉ đứng cạnh đạo cụ ở vòng trước.

**Sinh động ở mức minh hoạ nhỏ có hành động, chưa phải diễn xuất giàu cảm xúc.** Thân đã nghiêng nhưng chân, miệng và phần lớn khuôn mặt vẫn gần như cố định. Bàn tay là đầu tròn, khung ảnh/bản đồ đứng khá cứng; cảm giác “giữ một tấm bảng” vẫn mạnh hơn cầm một vật mềm. Tại 48dp, một số pose gần nhau. Tôi công nhận R2 đạt yêu cầu đã giao, không đổi tiêu chí thành diễn hoạt studio sau khi team sửa đúng. Nhưng chín pose này chưa tự động là tám sticker đạt chuẩn. Chưa đo clip motion/performance nên không kết luận animation đã mượt hay giàu sức sống.

**Có nét riêng ở cấp hệ thống, chưa có cơ sở nói độc nhất trên thị trường.** Giấy gập + góc coral + nét Gu + ghế mời + khung ảnh giữ lại đã có họ hàng với Rủ Đi. Giá trị nằm ở cách chúng cùng phục vụ câu chuyện “chừa một chỗ”, không ở việc mỗi icon phải khác mọi icon từng tồn tại. Không cần thêm màu, texture hoặc làm lệch lưới để chứng minh cá tính.

**Phần còn hơi khô:** Khám phá thiếu ảnh nay hữu dụng nhưng thiên danh sách chữ. Tôi chấp nhận đây là nhánh chính thức; không ép nhập đủ mười ảnh mới mở cổng. Nhánh ảnh lỗi vẫn giữ những khung beige lớn, nhất là ở dark, nên chưa tinh bằng Album dùng nền `card`. Đó là cơ hội polish có giới hạn, không lý do đưa stock sai quay lại hoặc tái mở thiết kế cả màn.

## 2. Kết quả bốn gói

| Gói | Kết luận | Căn cứ quan trọng |
|---|---|---|
| R1 · ảnh | **PARTIAL — còn một finding phải sửa** | Mười mapping sai đã gỡ, no-photo gọn, Explore hiện credit, ảnh lỗi có glyph. Nhưng Bình chọn và lịch trình bỏ mất quan hệ/nguồn của hai ảnh còn lại; xem F21 |
| R2 · Nếp | **Đóng yêu cầu quan hệ/điểm tiếp xúc** | Đủ 18 PNG team đã được mở; bảng tối và native cho thấy tay chạm đồ vật, không đứng trên ghế. Có lỗi riêng của PNG bảng sáng, xem F22 |
| R3 · Album | **Đóng F03a/F03b trong phạm vi component và fixture** | Native mới: 17/10 → ảnh dẫn → 18/10; chạm ảnh vào viewer đúng vị trí; đóng viewer trở lại Album. Fixture cũng mở viewer thay vì selection. Dark ảnh lỗi có khung vẽ |
| R4 · adaptive | **Đóng lỗi tên/giá/đơn vị/tab đã giao trên Android ≈411dp** | Native mới 1.3/2.0: cặp xếp dọc, hàng giữ giá và đơn vị, tab đủ chữ, cuộn tới hàng cuối và bấm lưu được. Không suy rộng thành mọi chuỗi/mọi thiết bị |

### Trả lời bốn câu team hỏi

1. **R2:** có, quan hệ giờ đọc ra; bản tắt Nếp vẫn đủ nghĩa. Giữ hướng này.
2. **R1:** có, nhánh không ảnh đủ dữ kiện để so sánh. Không cần ảnh mới hiểu tên/loại/giá/khoảng cách. Nhưng phải giữ sự trung thực của nhánh có ảnh trên mọi màn đang dùng nó.
3. **R3:** hai lỗi được nêu đã sửa đúng. Nhịp ngày rõ ở ca lead là ảnh duy nhất ngày đầu; nhãn xem/chọn không còn nói một đằng làm một nẻo.
4. **R4:** tên/giá/đơn vị/tab và thao tác lưu đã đạt ở bộ dữ liệu, font và viewport được kiểm. Chưa nghiệm thu toàn bộ tìm kiếm, bàn phím, iOS hoặc dữ liệu dài bất kỳ.

## 3. Ba việc hữu hạn cần trả lại team

### F21 — P1: quan hệ ảnh bị mất khi sang Bình chọn/lịch trình

**Không phải yêu cầu mới.** Review trước F01 đã yêu cầu đi hết adapter/consumer và giữ đúng credit; doc lần này cũng ghi `Group.tsx`, `Outing.tsx` trong R1.

**Bằng chứng native mới:** [Bình chọn](review-ngon-ngu-hinh-vong-2-evidence/09-voting-missing-provenance.png) đặt ảnh kiến trúc kính cạnh “Still Cafe Đà Lạt”, chỉ có tên/giá/khoảng cách, không có “Ảnh minh hoạ” hay nguồn. [Lịch trình AI ngày 2](review-ngon-ngu-hinh-vong-2-evidence/10-itinerary-missing-provenance.png) đặt cả ảnh đường đồi và ảnh cafe cạnh hai địa điểm thật nhưng cũng bỏ nhãn quan hệ. XML đi kèm xác nhận không chỉ là chữ nằm ngoài crop.

**Đường source trên HEAD được review:**

- `apps/mobile/src/rudi/screens/Group.tsx:369–378`: ô vote dùng `Image` trực tiếp, chỉ truyền `place.anh.source`.
- `Group.tsx:318–319`: lịch trình AI chuyển `noi.anh.source` vào `AnhChang`.
- `apps/mobile/src/rudi/screens/Outing.tsx:247`: timeline dùng cùng cách truyền. Đây là xác nhận source, không phải screenshot timeline mới.
- `apps/mobile/src/rudi/screens/keo/HangChang.tsx:40–44`: API của `AnhChang` chỉ nhận `source`, `alt`; không nhận relation/credit và không có `onError`.
- `fixtures.ts:275–276,321`: đúng hai ảnh ấy có consumer thực trong ngày 2 và vote, không phải nhánh chết.

**Hậu quả:** Explore nói “minh hoạ”, sang màn quyết định thì ảnh lại dễ được hiểu là ảnh của địa điểm. Nhãn “Demo” chung không thay được quan hệ từng ảnh. Đây là vấn đề tính trung thực của thông tin và hợp đồng thiết kế, không phải kết luận pháp lý về giấy phép.

**Sửa hẹp:** hoặc giữ cả `AnhMau`/relation/credit đến nơi render và hiển thị lời phân định đủ gần ảnh; hoặc bỏ hai thumbnail minh hoạ tại các consumer nhỏ không đủ chỗ, giữ dòng lịch trình/glyph có nghĩa. Không cần nhét nguyên một đoạn giấy phép vào ô ảnh 44dp. Nếu vẫn render ảnh, xử lý tải lỗi ở đúng consumer; các `Image` này hiện không có nhánh authored fallback như Explore.

**Qua khi:** native vote + lịch trình AI + timeline đều không ngầm gọi stock là ảnh thật của nơi; không mất nhãn khi cuộn; nhánh lỗi của component giữ lại được thử bằng nguồn synthetic lỗi. Thêm kiểm hồi quy ở consumer, không chỉ assert `nguon` có trong fixture. Sửa câu “nên không thể quên” tại doc dòng 35–36: kiểu dữ liệu giữ metadata không chứng minh renderer dùng metadata.

### F22 — P2 bàn giao: bảng năm cảnh sáng vẫn là file bị tile/cắt

Mở trực tiếp [file đang được link](../../claude/2026-09-08/sua-review/canh-nam-sang.png): tiêu đề “Có Nếp” lặp ngang; ba cảnh đầu rồi lặp hai cảnh đầu; hàng tắt Nếp bị chặt; phía đáy bắt đầu một tile khác. Doc dòng 240–242 nói hai bảng đã vẽ lại nhưng file đích sáng hiện không khớp lời đó.

**Đây là lỗi artifact, không phải chứng cứ app render hỏng.** [Bảng tối](../../claude/2026-09-08/sua-review/canh-nam-toi.png) đúng năm cảnh × hai hàng. [Native sáng mới](review-ngon-ngu-hinh-vong-2-evidence/06-scenes-light.png) xác nhận riêng tay–ghế và tay–khung, không bị tile. Không tái mở R2 vì lỗi xuất ảnh.

**Qua khi:** xuất lại đúng một bảng sáng, mở chính PNG tại đường dẫn được link, thấy đủ năm cảnh × hai hàng; cập nhật checksum. Không phải chạy lại vòng vẽ art. Native A/B sáng team cung cấp còn cắt phần đầu cảnh ghế do vị trí cuộn; dùng ảnh thấy đủ cảnh khi bàn giao thay vì gọi mọi crop đều là nghiệm thu trọn cảnh.

### F23 — P2 bàn giao/phạm vi: tên roadmap và mô tả hợp đồng đang lệch

Doc dòng 5–6 và 161–162 gọi B2 là “lời rủ trong ô so sánh”, C2 là “token disabled”. Không đúng bảng roadmap đã được review:

| Mã | Phạm vi đúng | Không được rút thành |
|---|---|---|
| B2 · R04/R07 | Tạo kèo / Plan / lịch trình và expanded | Một câu lời rủ trong ô so sánh |
| B3 · R01/R06 | AI typed cards, chat keyboard, khay sticker | Chỉ tám hình sticker |
| C2 · R02/R08 | Empty, profile, wall, badges, share | Token disabled toàn kit |
| Lát kit riêng | Token disabled, sinh các bản dùng chung, đo state | Đổi tên thành C2 hoặc một cuộc đổi palette |

Đính chính thêm ngay trong lượt sửa tài liệu này:

- Doc dòng 15 nói `AlbumAnh` dùng chung “live và fixture”; câu chi tiết dòng 80–82 mới đúng: **AlbumLive và bàn thử dev**. `Memories.tsx` vẫn có renderer riêng, chỉ dùng chung `PhotoViewer`/thành phần khung ảnh. Đây không làm mất hiệu lực bằng chứng component live, nhưng phải báo đúng.
- Quy tắc mới trong `DESIGN.md` cấm Nếp vào “hội thoại”, trong khi B3 chờ sticker Nếp. Ghi rõ ngoại lệ đã bàn ở review trước: sticker là phát ngôn do người gửi chọn; không phải mascot hệ thống đứng cạnh ledger, lỗi, conflict hay xác nhận tiền. Giữ ID, không dùng `tra-tien-ne` làm trạng thái giao dịch. Nếu team thực sự muốn cấm cả sticker thì phải trình Lead quyết định, không vừa ghi cấm vừa làm tám mẫu.
- “Ten stock photos gone” cần gọi đúng là **mười mapping địa điểm** đã gỡ; trước đó có năm asset được tái sử dụng, không phải mười file stock riêng đã xoá.

**Qua khi:** tài liệu và đề bài lát tiếp theo có cùng mapping, cùng ranh giới mascot/sticker, không thu nhỏ roadmap thành ba việc trang trí. Không yêu cầu làm luôn B2/B3/C2 trong lượt đóng finding.

## 4. Những gì nên nâng tiếp, nhưng không dùng để kéo dài cổng cũ

- **Một sticker trước tám sticker.** Chọn một hành động gắn câu chuyện, thử trong khay lẫn bubble ở cỡ thật sáng/tối. Ở đó cần nhận ra ý trước khi đọc tên pose: silhouette, hướng thân, mắt và đạo cụ cùng làm việc. Không chỉ đổi tay của tám khuôn mặt giống nhau. Đây là phép thử B3 đã định, không một vòng vẽ lại R2.
- **Nhịp Album đã tốt hơn:** ảnh dẫn như bản in, góc gập nhỏ, rồi chương theo ngày; nhân vật không tranh vai với người trong ảnh. Giữ sự tiết chế này. Một bài test người thật hiểu ngày/câu chuyện vẫn là bằng chứng khác với screenshot.
- **Glyph đúng từ vựng:** Puppy Farm/Thung lũng dùng tay cầm game vẫn không gợi đúng nông trại/vườn hoa. Ghi thành việc nội dung glyph khi đụng nhóm này; không coi gỡ stock sai là đã biên tập xong toàn catalogue, cũng không tái mở R1 để buộc vẽ cả taxonomy ngay.
- **Dark media error:** [ảnh lỗi Explore mới](review-ngon-ngu-hinh-vong-2-evidence/08-explore-error-dark.png) còn mảng beige rộng với đĩa đỏ nâu, trong khi Album *(ảnh đã kiểm local, không nằm trong PR: `04-album-error-dark.png`)* đã hoà với nền `card`. Không còn ô xám trống, nhưng khả năng hoà nhịp thị giác chưa đồng đều. Cân nhắc cùng lượt media polish, không đổi palette toàn app.
- **Không gọi R4 là “mọi chữ đã chuẩn”:** facts còn `numberOfLines={1}` ở `HangDiaDiem.tsx:175–176,251–252`; hiện các giá được đo đọc đủ. Search placeholder trong ảnh 17 *(ảnh đã kiểm local, không nằm trong PR: `17-explore-lead-font2.png`)* có phần chữ tràn/cắt ở 2.0. Đây là nợ input/large-text ngoài các trường bị chặn trước, ghi vào lượt harden tương ứng. Subtitle dài được rút bằng ba chấm không tự động là lỗi.
- **Disabled vẫn mờ** ở nút Tiếp tục *(ảnh đã kiểm local, không nằm trong PR: `16-preferences-bottom-font2.png`)*, nhưng không dùng tỷ lệ do team báo để kết luận vi phạm WCAG. Giữ lát token tập trung đã thống nhất.

## 5. Bằng chứng và giới hạn xác minh

### Đã làm mới trong vòng này

- Đọc toàn bộ doc trả lời, xem đủ **18 PNG** được dẫn, đối chiếu source, concept/spec Nếp và điều kiện gỡ chặn cũ. Dùng Impeccable repo để tách review mỹ thuật A khỏi detector/source B, không để dấu xanh định hướng mắt nhìn.
- Native `emulator-5554`, Android, `com.lakiet.rudi`, 1080×2400, density 420 (≈411dp); Metro riêng 8097, `EXPO_NO_DOTENV=1`, fixture bật, API đặt ở loopback 18999. Chỉ asset và nội dung demo/synthetic, không đăng nhập người thật.
- Dấu phiên *(ảnh đã kiểm local, không nằm trong PR: `01-session.png`)*: `codex-review-r2-34258f2c`; Metro có `Android Bundled`. Dấu phiên chứng minh lượt nạp, không phải checksum toàn JS/APK. Dùng APK đã cài, không rebuild native binary.
- Mới ở 1.0: Album hai ngày, viewer live-renderer trong lab và viewer fixture; đóng về context; lỗi ảnh Album dark; A/B scene sáng/tối; lỗi ảnh Explore sáng/tối; vote và lịch trình AI ngày 2.
- Mới ở 1.3: Explore cặp đã xếp dọc và hàng/credit. Mới ở 2.0: Sở thích đầu/cuối, Explore lead/cặp/hàng/cuối, bấm lưu Hồ Tuyền Lâm và tab Lên plan. Nguồn ảnh ở lab là asset demo/URI lỗi loopback, không phải API media thật.
- `npx tsc --noEmit`: **exit 0**, chạy mới từ `apps/mobile`.
- **19 tests source hiện hành pass**, chạy lại bằng `node docs/archive/codex/2026-09-08/review-ngon-ngu-hinh-vong-2-evidence/kiem-source.mjs` *(script local, không nằm trong PR: dùng data-URI base64 và đường dẫn tuyệt đối của máy review)*: 10 adaptive, 6 art-path, 3 nhóm helper ngày. Script transpile source và chọn test của repo, không dùng `dist-test` cũ. Đây là kiểm hồi quy đúng đầu vào hiện hành, không phải oracle độc lập hay chứng minh mỹ thuật.
- B báo một lượt detector phạm vi các file Explore/Album/art/tab/ui, exit 0 `[]`; không dùng làm chứng cứ native/ảnh/diễn xuất/a11y đạt. B gửi phát hiện trước khi phiên kết thúc do giới hạn chạy; main đã tự đọc lại source, chạy lại 19 tests và tái hiện finding trọng yếu, không coi phần tổng hợp thiếu của B là một verdict độc lập đầy đủ.

### Quản lý artifact

[Manifest](review-ngon-ngu-hinh-vong-2-evidence/manifest.json) ghi checksum PNG/XML cùng file source-check; có 21 PNG mới và XML đi kèm. **Hai ảnh 13–14 có tên dự kiến là Explore nhưng thực tế là Welcome sau khi Android đổi font gây nạp lại. Loại chúng khỏi bằng chứng R4**, không dùng tên file để suy ra nội dung. Đã vào lại demo và chụp đúng màn ở 17–21. Giữ ảnh 13–14 để không che giấu lượt chụp nhầm.

Font/theme cuối lượt khôi phục 1.0/sáng; gỡ reverse 8097 và dừng Metro riêng. Không sửa product, tokens, ADR hay doc Claude. Báo cáo/bằng chứng là bàn giao local, chưa commit/push. *(08/09, theo quyết định lead: hai báo cáo và sáu ảnh hỗ trợ finding mới được team đưa vào cây cùng PR sửa; XML và ảnh còn lại giữ local, xem `manifest.json`.)* Chưa stage nên không tuyên bố repo guard đã duyệt binary mới; khi đưa vào Git phải theo quy trình path/digest của repo.

### Chưa được xác minh độc lập

Không chạy lại toàn bộ 751 Node/3433 Python hoặc sáu bảng Maestro của team. Không xác nhận trực tuyến 10/11 CI hay phép so năm ca backend với baseline; đó vẫn là kết quả team báo, không phải kết quả vòng review này. Không audit lại nguồn/giấy phép trên website tác giả; ghi công/quan hệ lấy từ manifest repo, không cấp kết luận pháp lý hay chứng nhận ảnh chụp đúng địa điểm.

Chưa có OTP → API → ExploreLive/AlbumLive E2E, iOS, TalkBack/VoiceOver, wide hoặc release/motion. Một số ca Album 0/1/lẻ/ngày không rõ/caption dài đọc source/test nhưng không được main tái chạy hết ma trận native ở vòng này. Không coi khoảng chưa kiểm là bằng chứng đã hỏng hoặc đã pass.

## 6. Điểm tham khảo và quyết định đi tiếp

Điểm chuyên môn của Codex cho lát đang review, không phải điểm toàn app hay test người dùng. Cùng mười heuristic của vòng trước; vòng này có bằng chứng component/error mới nên xu hướng chỉ để theo dõi, không thay verdict.

| Heuristic | /4 | Nhận xét |
|---|---:|---|
| Trạng thái hệ thống | 3 | Count, chọn và lỗi ảnh rõ trong các ca đã xem |
| Ngôn ngữ/thế giới thực | 2 | Quan hệ trong scene tốt hơn, stock còn mất phân định ở consumer |
| Quyền kiểm soát | 3 | Xem/đóng ảnh đúng, lựa chọn có entry riêng |
| Nhất quán | 3 | Tầng art và layout thống nhất hơn; media consumer còn lệch |
| Ngăn lỗi | 2 | Có giới hạn chọn gu; đường async/live chưa kiểm đủ |
| Nhận ra thay vì nhớ | 3 | Nhãn và facts đọc được trong lát chữ lớn |
| Hiệu quả thao tác | 3 | So sánh thích ứng, tab và nút lưu dùng được |
| Thẩm mỹ/tối giản | 3 | Có quan hệ và nhịp riêng; diễn xuất còn tiết chế |
| Khôi phục lỗi | 3 | Explore/Album có authored fallback; vote/timeline còn thiếu |
| Trợ giúp đúng lúc | 2 | Helper đủ cơ bản; chưa có chứng cứ hiểu/khám phá từ người dùng |
| **Tổng** | **27/40** | 23 → 27, tham khảo cho lát này, không phải chứng nhận chất lượng toàn app |

**Ba tình huống người dùng:** người mới còn có thể hiểu thumbnail vote là chính quán; người dùng chữ lớn nay xem giá không phải mở từng detail; người muốn xem kỷ niệm nay chạm đúng ảnh và trở về được. Chưa dùng screen reader nên không suy ra trải nghiệm người khiếm thị đã đạt.

**Mạch cảm xúc:** chọn gu nhẹ nhàng → so nơi rõ và trung thực hơn → mời/giữ chỗ bằng hình → giữ kỷ niệm bằng ngày và bản in. Mạch đã có; phần mời trong tác vụ thật và biểu đạt trong chat chính là chỗ B2/B3 sẽ tiếp tục làm sâu, không cần bắt ba màn mẫu kể hết mọi câu chuyện của app.

**Quyết định hữu hạn:** sửa F21 ở các consumer đang dùng ảnh, thay artifact F22, đính chính phạm vi/hợp đồng F23; review xác nhận đúng các delta đó rồi mở B2/B3/C2 theo roadmap thật. Không phải thêm mười ảnh, vẽ lại toàn Nếp hay làm đủ tám sticker để được đóng vòng này. Có thể chuẩn bị đề bài lát tiếp theo, nhưng chưa gọi gate full đã APPROVE trong khi F21 còn tái hiện.

**Thông điệp gửi team:** “Hướng đã được chấp nhận; R2–R4 đạt trong lát Android đã kiểm. Còn một lỗi R1 xuyên consumer và hai việc bàn giao rõ ràng. Sửa đúng các chỗ này, giữ phần đã đạt, rồi đi tiếp. Chưa phải toàn app hoàn thiện; cũng không còn lý do mở lại brand direction từ đầu.”

Questions skipped: Lead đã xác định phạm vi review lại và điều kiện trước full; không cần hỏi lại ưu tiên hay xin quyền sửa product. Không thực thi các gợi ý implementation trong lượt review này.
