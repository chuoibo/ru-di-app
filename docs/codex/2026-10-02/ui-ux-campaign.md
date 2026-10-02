# Tiếp nối toàn bộ audit UI/UX · 167 ID

Nguồn: PR #663 (`3bd112e5`), plan và bàn giao của session Claude `c97c4813-340b-485b-a30b-32b6828ad498`.
Mục tiêu là xử lý toàn bộ phần còn lại, kèm review thiết kế, runtime native Android và QA retest; B9a không phải kết thúc chiến dịch.

Tài liệu Claude giữ nguyên: 103 READY_FOR_QA, 3 VERIFIED_LOCALLY, 58 PLANNED, 1 phần UI READY_FOR_QA nhưng luật BLOCKED, 2 BLOCKED.
Codex đã xử lý thêm UI-150/151/152/154 ở `88282a87`, bàn giao `b4f4b189`. Tại lúc lập ledger, còn 35 case chưa triển khai trong scope Codex: 26 B9 + 9 B10. 18 case B8 để nguyên cho Claude; UI-096 đã bị thay thế bởi UI-158, không triển khai lại.
READY_FOR_QA là tự kiểm và có hướng dẫn retest; không phải QA_ACCEPTED. Các trạng thái kế thừa chỉ là trạng thái do Claude ghi, không phải Codex xác minh lại.

## Hướng và thứ tự

Giữ giấy, mực, coral và chữ Bricolage. Kỷ niệm để ảnh/câu chuyện dẫn; trình xem recedes; form có một quyết định, lỗi cạnh thao tác; cài đặt dùng trạng thái thật và lời dễ hiểu.
CP00 baseline và bảo vệ công việc; CP01 viewer (094/098/101); CP02 nháp/ảnh (097/102); CP03 tường/album/bình luận (095/100/103/104/105/155/156/158/159); CP04 hành trình/kệ sổ (106/153/157/160/161/162); CP05 cài đặt/Cá nhân (015/107/108/109/110/111); CP06 vỏ app (002/009); CP07 Welcome (016/017/020); CP08 Nếp (008/011/012/014); CP09 consistency/gates/bàn giao.
Không ghi đè worktree chính hay report QA/Claude. File có B8 sửa dở chỉ được patch phần issue mới, trong worktree riêng.

## Checkpoint đang thực hiện

### CP00 · Baseline
- [x] Đối chiếu ledger 167 ID, 35 case mới và 18 case B8 giữ nguyên.
- [x] Worktree riêng `codex/ui-ux-b9a`, baseline `b4f4b189`; manifest hash bảo vệ công việc Claude lưu ngoài repo.
- [x] Backup app-private và cấu hình của AVD đang có; runtime riêng, dữ liệu tổng hợp.

### CP01 · Xem ảnh và story
- [x] Source baseline: viewer đo mỗi chiều rộng nên ảnh web có thể cao 0; counter chỉ nghe momentum; caller unmount ngay khi đóng; vùng story chưa có role/focus xác nhận.
- [x] Brief: ảnh làm trọng tâm, cover indigo làm nền; tên/counter tách rõ khỏi caption. Một cụm nút trước/sau/phóng ảnh phục vụ cả gesture lẫn bàn phím. Không cắt ảnh, không làm đổi quy tắc xem story.
- [x] Motion thesis: ảnh bước vào/ra cùng một fade có lifecycle, exit ngắn hơn enter; đóng nhanh/Back/tap lặp chỉ hoàn tất một lần; zoom giữ giới hạn ảnh và không phóng trang. Reduced motion giữ nguyên thông tin.
- [ ] Implement UI-094/098/101, acceptance: ảnh có chiều cao thật; counter/caption khớp cuộn; enter/exit có khung trung gian; focus vào xác nhận story, trả về nút xoá; mọi thao tác có role và vùng bấm 48dp.
- [ ] Render Android thực; web đối chiếu bug gốc; 320/font lớn/tablet/keyboard/reduced motion theo ngữ cảnh.
- [ ] Inspection tối đa hai vòng; detector một lần, polish và regression phù hợp.
- [ ] Fresh finish reviewer, xử lý disposition, documenter.
- [ ] Commit riêng, kiểm clean SHA và handoff QA; chưa QA_ACCEPTED.

### Đối chiếu khi tiếp tục · 2026-10-03
Worktree chính hiện `d97430db`, session Claude đã viết thêm patch B9 tới 21:20 ngày 2026-10-02 (UTC+7), chưa commit/bàn giao. Vì yêu cầu không làm lại việc Claude đã/đang làm, CP02–CP05 và viewer trùng scope được giữ cho Claude. Không copy patch B9 đang dở vào nhánh Codex và không sửa worktree chính.
CP01 Codex đã implement riêng trước khi phát hiện phần trùng: 1.460 test pass, 0 fail/skip trên nhánh cũ; ảnh Android phone/320/font130/dark/tablet và video thực đã xem. Review yêu cầu sửa giới hạn pan theo ảnh contain, trả focus native và chụp lại status bar; source hai sửa đầu đã có, **render cuối chưa xác minh**. Giữ patch này ở `codex/ui-ux-b9a`, IMPLEMENTED_UNVERIFIED; không tự áp đè patch Claude.
Scope tiếp nối độc lập: CP06–CP08, 9 ID B10 (002/009/016/017/020/008/011/012/014); CP09 retest và consistency phần mới. Nhánh mới bắt đầu từ commit main `d97430db`, không mang patch B9 trùng việc.

### CP06 · Vỏ và đường lạ
- [x] Baseline source: root chờ font/session bằng null; unknown route redirect Welcome kể cả có phiên. Main sạch `d97430db`; B9 dirty để Claude.
- [x] Brief: mở app cần biết đang trở lại trải nghiệm; link sai cần lối về thật. Nền bìa và chữ hiệu khi khôi phục; đường lạ dùng trang giấy và một hành động Về Rủ Đi, không in URL/token.
- [x] Acceptance: chỉ báo có ngay trước font; không quyết định auth trước khôi phục; có phiên mở URL lạ không bị đòi OTP; links thật/lời mời giữ nguyên; Back có lối ra. Không thêm motion trang trí; reduced motion giữ thông tin.
- [x] Implement UI-002/009; source và 2 đường recovery đã chạy.
- [x] Android phone/320dp font130, web390/1280, phiên thật tổng hợp; Back/CTA → Khám phá. Proxy web trễ5s/20s khóa scene. Hồi quy export giữ phiên tổng hợp không nhóm → Tin nhắn, frame5.0ms từ resume request. Chưa đo native300ms/TalkBack/iOS.
- [x] Detector URL thất bại timeout (không dùng [] làm bằng chứng); fallback source [] chỉ regex. Polish và review fresh bắt thiếu focus, sửa + CUA xác minh heading rồi Tab → CTA.
- [x] Finish CP06 `ship` theo scope; documenter giữ hệ thống và ghi bàn giao. Clean gates cuối ở CP09.
- [x] Typecheck/export; testopening1/0/0, journey riêng1/0/0.
- [ ] Clean SHA, commit/handoff/tick: fullpatch1472pass1fail0skip (journey selection, 2 lần); fullbaseline sạch1473pass0fail0skip; isolatedpatch/baseline xanh. Giữ phát hiện flaky trong bàn giao, kiểm lại clean SHA cuối B10.

### CP07 · Bìa Welcome
- [x] Baseline: web không phát momentum-end; pager chỉ nghe sự kiện đó, snap cho phép nhảy trang, chấm không role và vùng cuộn không focus. Chữ hiệu, washi và đường giấy đã có bản sắc; giữ chúng.
- [x] Brief: người mới hiểu chuỗi lời rủ → chọn nơi → rõ phần tiền → giữ kỷ niệm. Một câu dẫn mỗi trang, mốc đường/copy/chấm cùng một vị trí. CTA đóng dấu mở bìa hiện có là khoảnh khắc chính; điều khiển trang cần phản hồi rõ và truy cập bằng phím.
- [x] Motion sau review video: dùng một chuyển trang native Welcome → Login, giữ bìa đọc được tới khi trang sau sẵn sàng; bỏ xoay bìa giữ khung trắng rồi chuyển lần hai. Chạm đúp chỉ một navigation; trở lại mới mở khóa. Swipe dừng một chặng, resize giữ trang; reduced motion đổi ngay. Giữ đường giấy vào màn hiện có, không thêm độ trễ.
- [x] Implement 016/017/020; onScroll đồng bộ, snap-stop always web/disableIntervalMomentum native, chấm 48dp có tên/trạng thái; phím trái/phải và Home/End; trang ngoài khung không đọc cùng lúc.
- [x] Review native phone/320-font lớn/tablet và chuyển động thật; web vuốt/keyboard + accessibility; build fingerprint chỉ xuất hiện khi harness yêu cầu.
- [x] Detector rendered một lần: 23/34 finding mobile/desktop còn ghi nhận, không ignore. Axe 0 violation/32 pass/11 incomplete. Fresh review chấm hai sửa motion resolved; `ship` theo scope. Test export Welcome + opening 2/0/0; typecheck/export đạt.
- [x] Documenter giữ DESIGN/sidecar và ghi bàn giao; commit CP07 riêng. Clean SHA/gates và handoff cuối tiếp tục ở CP09.

### CP08 · Nếp có chỗ đứng và lời xác nhận
- [x] Baseline: NepDien Pressable vô danh vẫn focus được; chip bảng 36dp; Vẽ dùng Alert.alert web không phản hồi; hàng loại Khám phá cuộn sát cạnh nơi mép sổ đứng.
- [x] Brief: Nếp ở bên cạnh thao tác, nói rõ phần sẽ gửi; mỗi lần vẽ là một xác nhận riêng. Một lời nhờ → xem mô tả → xác nhận/cancel → trạng thái đang vẽ. Không gọi AI có phí khi test, không sửa quota/business rules.
- [x] Điều chỉnh từ render Android: keyboard che hoàn toàn ô nhập của sheet cũ. Chỉ Nếp bật tránh keyboard; ô soạn nhiều dòng và hành động giữ ở chân sheet, lời tham khảo và mô tả xác nhận cuộn phía trên. Nút Sửa/Vẽ luôn ở chân sheet. Trần sheet theo vùng còn lại, không cộng padding cố định và không giảm font.
- [x] Implement 008/011/012/014: performance có tên nút bỏ qua khi đang chạy, tĩnh không nhận Tab; chip ≥48dp; xác nhận Vẽ trong ngôn ngữ sheet/giấy, mô tả cố định, đóng/Back hủy; mép dock không che hàng loại ở 320dp.
- [x] Keyboard/font lớn/error/cancel, native motion, web focus/geometry: 6 test web pass/0 fail/0 skip; phone/320 font130/tablet Android + IME/video thực đã mở nhìn. Detector 23/36 findings giữ nguyên; mapping6/20 raster cụ thể, không ignore.
- [x] Finish `fix` → một batch: disclosure cố định cạnh Sửa/Vẽ, thanh cuộn cho mô tả; recapture cùng file và verdict `ship` cho đúng hai sửa.
- [x] Documenter ghi CP08 vào b10-handoff.md, giữ DESIGN/sidecar; commit riêng. Clean SHA/gates thuộc CP09.

### CP09 · Nhất quán và cổng kiểm chứng
- [x] So sánh screen/overlay/state mới với notebook world; không đổi hệ nhận diện, quota, money/API hoặc B8/B9.
- [x] Thêm recovery web anonymous có/không history; test kết hợp6/0/0. Native warm place → Back → unknown → Back → invite điền mã tổng hợp, không redeem.
- [ ] Native anonymous/cold deep links; ghi giới hạn devclient cold launcher.
- [ ] Clean exact-SHA full gate, identity xanh, canary và ≥2 mutant không tương đương đỏ đúng dự đoán cùng harness.
- [ ] Tái kiểm concern journey selection, cập nhật handoff/status thật và khôi phục emulator.

## Theo dõi đủ ID

| ID | Batch kế thừa | Trạng thái Claude | Tiếp nối Codex | Vấn đề/tiêu chí |
|---|---|---|---|---|
| UI-001 | B2 | VERIFIED_LOCALLY | KẾ_THỪA · cần retest cuối | Ô nhập một dòng 44dp (<48); nút cảm xúc, nút story nhỏ; hitSlop bị web bỏ qua · mọi ô nhập một dòng ≥48dp |
| UI-002 | B10 | PLANNED | VERIFIED_LOCALLY · CP06 | Có phiên mà mở URL lạ thì về Welcome rồi bị đòi đăng nhập · URL lạ + phiên → về tab, hoặc 404 có lối ra |
| UI-003 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | `accessibilityState` không tới DOM: tab, radio, checkbox, mở gập không có `aria-*` · mọi control có trạng thái mang `aria-*` khớp; tab đang chọn có `aria-selected` |
| UI-004 | B2 (sửa sớm: rail và tablist) | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Rail (≥600dp): vạch chỉ báo lệch khỏi tab · vạch nằm giữa tab đang chọn ở 600/768/839/840/1024 |
| UI-005 | B1 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Đóng khay «Tạo mới» bằng Back trình duyệt để lại `aria-hidden`/`inert` trên cả màn và thanh tab · 0 vùng inert ≥25% màn sau Back |
| UI-006 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Chạm «+» hai lần nhanh: lần hai rơi vào khay đang mở · chạm đúp = 1 hộp thoại |
| UI-007 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Khay tạo cao 92% (C2) / 96% (C8), vượt trần 82% · panel ≤82% ở C2, C8 |
| UI-008 | B10 | PLANNED | VERIFIED_LOCALLY · CP08 | Điểm dừng Tab đầu tiên là khối Nếp không tên · không còn điểm dừng không tên |
| UI-009 | B10 | PLANNED | VERIFIED_LOCALLY · CP06 web300/native nhìn | Khôi phục phiên chậm: vùng nội dung trống, không chỉ báo · skeleton/chỉ báo ≤300ms |
| UI-010 | B2 (sửa sớm: khay Tạo mới) | READY_FOR_QA | KẾ_THỪA · cần retest cuối | `/create` mở lạnh không mở khay · mở lạnh /create → 1 hộp thoại |
| UI-011 | B10 | PLANNED | VERIFIED_LOCALLY · CP08 | Nút «Vẽ» của bảng Nếp chết trên web (`Alert.alert` rỗng) · có phản hồi thấy được trên web |
| UI-012 | B10 | PLANNED | VERIFIED_LOCALLY · CP08 | Chip gợi ý của bảng Nếp cao 36dp · ≥48dp |
| UI-013 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Sheet đóng: panel còn lộ rồi biến mất đột ngột · khung cuối ra ngoài màn hoặc mờ ≈0 |
| UI-014 | B10 | PLANNED | VERIFIED_LOCALLY · CP08 | Ở 320dp mép Nếp đè chữ hàng chip · không chữ nào bị che ở C2 |
| UI-015 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Cài đặt hiện giá trị giữ chỗ («Bạn», «B», công tắc sai) rồi mới đổi · không khung giữ chỗ sai |
| UI-016 | B10 | PLANNED | VERIFIED_LOCALLY · CP07 | Welcome web: chấm trang và mốc đường đứng yên ở trang 1 khi vuốt · chấm/nhãn khớp trang đang xem |
| UI-017 | B10 | PLANNED | VERIFIED_LOCALLY · CP07 | Welcome: vuốt nhanh nhảy hai trang · vuốt nhanh = 1 trang |
| UI-018 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | «Quay lại» chết khi màn mở thẳng bằng link (TopBar không kiểm `canGoBack`) · nút lui luôn tới một màn |
| UI-019 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Mã lời mời sai báo «Cập nhật app rồi thử lại» · câu theo `code` (mã sai/hết hạn) |
| UI-020 | B10 | PLANNED | VERIFIED_LOCALLY · CP07 | Chấm trang Welcome không đọc được; pager không nhận focus · axe 0 trên /welcome |
| UI-021 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Hàng địa điểm: dòng giá bị cắt ở 8–10/10 hàng · 0 dòng giá bị cắt ở C1–C6 |
| UI-022 | B1 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | «Chỉ đường» trên web không làm gì (`geo:`) · mở bản đồ hoặc nói vì sao |
| UI-023 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Nhãn «Lưu địa điểm» và nút demo bị cắt · nhãn trọn ở C2 |
| UI-024 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Nút ✦ chỉ điền câu mẫu, danh sách báo «0 kết quả» trước khi có câu hỏi · không «0 kết quả» trước câu trả lời |
| UI-025 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Danh sách nhảy 149dp khi sân khấu chen vào · 0dp ở C1, C9 |
| UI-026 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Bỏ lọc làm sân khấu dựng lại từ đầu · không dựng lại |
| UI-027 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Giảm chuyển động: sân khấu trống một lúc; Nếp M5 nhảy tư thế · không khung trống ở C9; M5 một hình |
| UI-028 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Thành phố chưa có quán khuyên bỏ bộ lọc không tồn tại · không «Xóa lọc» khi không có lọc |
| UI-029 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | 503 và mất mạng cùng một câu · 503 → câu máy chủ; mất mạng → câu mạng |
| UI-030 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Mất mạng rồi về tab: danh sách đã tải bị thay bằng màn lỗi · giữ danh sách đã tải |
| UI-031 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Điểm đến luôn 2 cột kể cả expanded · 3 cột ở C7 |
| UI-032 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Kèo nhiều ngày: Bản đồ không hiện chặng nào · tổng ghim = số chặng, hoặc dòng «chưa xếp ngày» |
| UI-033 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Ngày trống: dòng giải thích dưới «Về Lịch trình» còn bị cắt (nút đã đạt) · thấy trọn ở C1–C4, C8 |
| UI-034 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Ô ngân sách trông như đã điền; bỏ trống thì lỗi ngoài màn · lỗi thấy ngay sau khi chạm |
| UI-035 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | `/trips/[id]/timeline` hiện demo cho người đã đăng nhập · có phiên → /outings/[id] |
| UI-036 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Đổi thứ tự chặng chỉ bằng kéo; tay nắm không nhận focus · bàn phím đổi được; ARIA hợp lệ |
| UI-037 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Nếp vắng ở chế độ Lịch trình · Nếp ở Lịch trình, ẩn ở Bản đồ |
| UI-038 | B1 (Sheet) + B6 (khay trong màn chat) | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Back trình duyệt khi sheet mở rời cả màn, mất chữ đang gõ · Back đóng sheet, giữ URL |
| UI-039 | B2 (nhờ Sheet v2) | READY_FOR_QA | KẾ_THỪA · cần retest cuối | «Thêm chặng» là công tắc: chạm đúp mở rồi đóng · chạm đúp → 1 sheet |
| UI-040 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Sheet cao 90–96% ở cửa sổ thấp; nút chính dưới mép · ≤82% ở C8 |
| UI-041 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Sheet «Sửa trang ngày» không làm mờ đầu màn mà chặn chạm · đầu màn mờ; chạm ngoài thì đóng |
| UI-042 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | ARIA sai vai trò ở màn kèo · axe 0 critical ở màn kèo |
| UI-043 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Điều khiển bản đồ tên tiếng Anh; Esc không đóng popup cụm · không tiếng Anh; Esc đóng |
| UI-044 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Dòng thông tin vé kèo bị cắt (mất số chặng) · C2 giữ số chặng |
| UI-045 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Ở 320dp cột tên chặng 53px · ≥120px |
| UI-046 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | `/outings/chon` thiếu `?place` kẹt skeleton · câu + lối ra ≤1s |
| UI-047 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Tablet: đầu màn co vào giữa, lệch cột nội dung · nút lui thẳng mép cột |
| UI-048 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Số tiền món bị cắt «12.3…» · 0 số tiền bị cắt |
| UI-049 | B1 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Web: «Gửi cho <tên>» không gửi được link, báo «Kiểm tra mạng» ở y −2855 · có lối chép link; không báo lỗi khi đóng khay |
| UI-050 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Từ 9 người ghế đè nhau; 20 người chạm ghế này đổi ghế khác · chạm đúng ghế tới n=20 |
| UI-051 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Lý do chặn và lỗi 503 ở đầu trang, ngoài màn · câu trong khung nhìn sau khi chạm |
| UI-052 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Lùi bước 1, Back hay tải lại đều mất bill đang gõ · không mất mà không hỏi |
| UI-053 | B2 | VERIFIED_LOCALLY | KẾ_THỪA · cần retest cuối | Phím Space không đổi checkbox/radio dựng bằng Pressable · Space đổi trạng thái |
| UI-054 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Sơ đồ quyết toán từ 10 người: nhãn đè nhau · 10 người: 0 chỗ đè |
| UI-055 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Nếp M2 bị đẩy ra ngoài màn · trong màn ở C1–C3 |
| UI-056 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Đọc ảnh bill lỗi khuyên nhập tay mà không có nút · có lối nhập tay tại chỗ |
| UI-057 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Mép Nếp hứa «chạm để kéo ra» mà chạm không làm gì · không hứa điều không làm |
| UI-058 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | «Tạo đợt thu từ sổ» vẫn mời khi mọi khoản đã vào đợt · không có nút chắc chắn hỏng |
| UI-059 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Dòng người trả mất «(trả)» · «trả» luôn thấy |
| UI-060 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Tài chính: «Chi theo nhóm» không có hàng · tiêu đề khớp nội dung |
| UI-061 | B4 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Quyết toán ở 320dp: dòng đầu sổ ép thành 9 dòng · ≤5 dòng ở C2 |
| UI-062 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Ô soạn web là textarea 2 hàng không cao lên; chữ lệch 20px · lệch ≤4px; cao dần tới trần |
| UI-063 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Bong bóng có link dài tràn cột; 320dp mất đầu link · bong bóng trong khung |
| UI-064 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Avatar thấp hơn bong bóng 22px; giờ lặp dưới mọi cụm · avatar ±4px; giờ theo khoảng thời gian |
| UI-065 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Thẻ bình chọn chiếm 39–59% màn, «phiếu» lặp 4 lần, trải hết tablet · ≤25% chiều cao C1; «phiếu» ≤1; ≤560 ở C6 |
| UI-066 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Khay «Tờ hẹn chung» không đóng bằng Esc · Esc đóng, focus trả về |
| UI-067 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Sheet báo cáo: lý do chọn chỉ khác màu nền · 5 radio, một đang chọn |
| UI-068 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Một request tin hỏng: câu lỗi chung, không «Thử lại», dù tin vẫn hiện đủ · không câu thừa, hoặc có «Thử lại» |
| UI-069 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Thẻ thông báo lỗi mọc ở cuối chat, mặc áo AI · trong khung nhìn, không kiểu AI |
| UI-070 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Dải ghim sáng trên nền mờ khi sheet mở · dải bị mờ cùng nền |
| UI-071 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Lập nhóm xong về Khám phá, không thấy nhóm vừa lập · vào nhóm mới + lời mời |
| UI-072 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Mời người đã ở trong nhóm: 409 hiện thành câu «lần bấm trước» · câu «đã ở trong nhóm» |
| UI-073 | B8 + ADR | PLANNED | RESERVED_CLAUDE_B8 | Người vào bằng lời mời bỏ qua Sở thích; tên người mời đặt thành tên công khai · Sở thích có ô tên sửa được (luật tra số: ADR đề xuất) |
| UI-074 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Quản trị tự bỏ quyền bằng một chạm · hỏi trước khi tự hạ quyền |
| UI-075 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Mọi quản trị mang nhãn «Người lập nhóm» · chỉ người lập |
| UI-076 | B8 | PLANNED | RESERVED_CLAUDE_B8 | 19 nút cùng tên «Đặt làm quản trị»; hàng không mở hồ sơ · tên nút riêng; hàng mở hồ sơ |
| UI-077 | B8 | PLANNED | RESERVED_CLAUDE_B8 | «Đồng ý» lỗi thì cả danh sách bạn thành màn lỗi · lỗi theo hàng, danh sách giữ |
| UI-078 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Sau khi chặn, tải lại mất dấu «Đã chặn» · «Đã chặn» + «Bỏ chặn» sau tải lại |
| UI-079 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Chat đôi bị chặn: «Đang nối lại» mãi, vẫn mời nhắn · 20s: không còn; người chặn thấy lý do |
| UI-080 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Lời mời vào nhóm không nói ai mời, không từ chối được · tên người mời + hai lựa chọn |
| UI-081 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Tablet: danh sách trải hết, nút cách tên 487–679px · ≤160px |
| UI-082 | B1 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Không phiên: link thật mở sổ demo không nhãn, «Gửi» báo đã gửi · cửa đăng nhập rồi về link; không báo gửi giả |
| UI-083 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Đọc sổ lỗi thì vẽ «Chưa có sổ» và mời lập lại · lỗi + «Thử lại» |
| UI-084 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Sheet «Lập sổ» quay về trạng thái mời khi vừa được đồng ý · 0 khung mời sau khi đồng ý |
| UI-085 | B8 | PLANNED | RESERVED_CLAUDE_B8 | «Rủ … tới đây» phác thêm tờ không mang quán, câu nói ngược · số tờ không đổi + câu đúng |
| UI-086 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Người đề nghị đóng sheet thì không còn thấy «đang chờ» · câu chờ trên màn |
| UI-087 | B1 (sheet đóng khi màn mất focus) | READY_FOR_QA | KẾ_THỪA · cần retest cuối | «Cài đặt sổ» vẫn mở khi quay lại · 0 hộp thoại khi quay lại |
| UI-088 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Nền sheet nhận chạm ngay lúc mở · chạm đúp → 1 sheet |
| UI-089 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Tay cầm `div` có `aria-label` không role · axe 0 |
| UI-090 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Tên trên bìa sổ bị cắt «Chat Tes…» · tên phân biệt được |
| UI-091 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Nút tắt không nói lý do · không nút tắt nào thiếu lý do |
| UI-092 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Lá ngày đang chọn nằm khuất · lá chọn thấy trọn (cả ngày xa) |
| UI-093 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Tablet: tờ giấy, sheet trải hết bề ngang · ≤640 |
| UI-094 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Viewer web: ảnh cao 0, vuốt nhảy hai ảnh, chụm phóng cả trang · ảnh >0; «2/3»; không phóng trang |
| UI-095 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Thả tim lỗi ở cuối tường: câu lỗi ở đầu tường · trong khung nhìn |
| UI-096 | B9 | PLANNED | SUPERSEDED_BY_UI_158 | (đã hết: nút xoá bình luận bị gỡ; lối xoá chuyển thành UI-158) · xem UI-158 |
| UI-097 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Rời «Thả khoảnh khắc»/«Đăng story» mất ảnh và chữ, không hỏi · hỏi hoặc giữ nháp |
| UI-098 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Viewer mờ dần khi mở nhưng biến mất ngay khi đóng · ≥1 khung mờ giữa chừng |
| UI-099 | B2 (Chip `maxWidth`) | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Chip tên quán dài tràn mép sheet check-in · chip trong sheet |
| UI-100 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Bài «Chỉ mình tôi» mở bởi người khác: hai khối lỗi, hai «Thử lại» vô ích · một khối, không «Thử lại» |
| UI-101 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Viewer story: vùng chạm không role; câu hỏi xoá không nhận focus · axe 0; focus vào câu hỏi |
| UI-102 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Ảnh dọc 9:16: xem trước trọn, lên tường bị cắt · cùng vùng thấy |
| UI-103 | B9 (Go) | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Album chỉ ghi năm, không ghi ngày chuyến · có ngày chuyến |
| UI-104 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Ngày viết «28-09», «28/9/2026» lẫn lộn · một định dạng |
| UI-105 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Tablet: ảnh tường 702–894px, cao hơn cửa sổ · cột ≤640 |
| UI-106 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Ở 320px thẻ huy hiệu bẻ đôi chữ «châ/n» · cột chữ ≥120px |
| UI-107 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Lưu công tắc lỗi: câu lỗi ở cuối trang · cạnh control |
| UI-108 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Panel trong Cá nhân trông như màn con, Back rời tab · Back đóng panel |
| UI-109 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | «Đã lưu» chỉ có con số · mở được từng chỗ |
| UI-110 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Xoá tài khoản: «XOA» không dấu; «XOÁ» tắt nút không lý do; Back rời trang · nhận «XOÁ» hoặc nói lý do; Back về bước 1 |
| UI-111 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Câu cuối Cài đặt chỉ sai chỗ đổi tên · chỉ đúng chỗ |
| UI-112 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Sang màn mới focus ở `body` · focus trong màn mới |
| UI-113 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | 320dp: tim «Lưu» của cặp so sánh bị đẩy ra ngoài · cả hai tim trong ô |
| UI-114 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Nút «Lưu» lồng trong nút «Mở …» · không nút lồng nút |
| UI-115 | B5 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Sân khấu kéo nghiêng chặn cuộn dọc · kéo dọc vẫn cuộn |
| UI-116 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Chat demo: bong bóng không xuống dòng, chữ tràn · 0 chữ bị cắt ở C1–C3 |
| UI-117 | B1 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Back khi sheet demo mở: Khám phá bị khoá, chạm rơi vào sheet vô hình · không inert sót; chạm hoạt động |
| UI-118 | B3 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Kèo tạo từ chat không để lại dấu trong chat · chat có kèo + lối mở |
| UI-119 | B3 + ADR | READY_FOR_QA (phần UI) · BLOCKED (luật: chờ quyết ADR UI-119) | KẾ_THỪA · cần retest cuối | «Tôi đã tới» không thành kỷ niệm; album «0 chỗ đã tới» · lối thêm khoảnh khắc ngay sau khi tới; album nói rõ nó đếm gì (luật: ADR đề xuất) |
| UI-120 | B1 (Go+Py) | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Bị chặn vẫn gửi được tờ hẹn tới người đã chặn mình · bên bị chặn gửi → từ chối; bên chặn không nhận gì |
| UI-121 | B1 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Đăng nhập từ link không quay về link đó · tới đúng link gốc |
| UI-122 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Đầu chat giữ số thành viên cũ khi có người mới vào · cập nhật trong vài giây |
| UI-123 | B1 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Web: đổi tab thay mục lịch sử, Back rời app · Back qua lại giữa các tab |
| UI-124 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Chat hai người rỗng ở cửa sổ thấp đẩy ô soạn ra ngoài · nút gửi trong khung ở C2/C4/C8 |
| UI-125 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Hàng ghim tắt/bật mỗi lần focus (nhảy 78dp); không thấy phòng thành cặp đôi · 0 khung mất hàng; cập nhật ≤8s |
| UI-126 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Hàng mời «Một đôi» dẫn tới màn không nhắc lời đề nghị · màn tới nói về lời đề nghị |
| UI-127 | B8 | PLANNED | RESERVED_CLAUDE_B8 | Khoảnh khắc M6 «sổ hai người mở» không diễn · M6 diễn một lần; C9 khung cuối |
| UI-128 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Chat hai người còn chữ của nhóm · không «nhóm/hội» trong chat hai người |
| UI-129 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Sheet «Gu của hai bạn»: hai dòng ngược nhau; lỗi nằm dưới lớp phủ · một câu đúng; lỗi trong sheet |
| UI-130 | B8 | PLANNED | RESERVED_CLAUDE_B8 | «Rủ X tới đây» cho cặp bạn: chỗ vừa chọn bị bỏ · màn tới mang quán, hoặc nút ẩn |
| UI-131 | ADR (`adr-de-xuat/UI-131-…`, chờ chủ sản phẩm) | BLOCKED | KẾ_THỪA · cần retest cuối | Máy chủ vẫn phác tờ cho cặp chưa «Một đôi» · draft cho cặp bạn → 4xx |
| UI-132 | B7 (Go) | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Cộng đồng mới: tab đầu báo «chưa kết nối được», «Thử lại» không bao giờ được · 200 với 0 bài; trạng thái rỗng mời kể chuyện |
| UI-133 | B7 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Sáu chủ đề hay chủ đề một ký tự → câu «lỗi của app», dưới mép màn · câu theo chủ đề, thấy lúc gửi |
| UI-134 | B7 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Stream nối lại làm mất bình luận đang gõ · nháp sống qua nối lại |
| UI-135 | B7 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Mở bài rồi lui: bảng tin về đầu, bài gập lại · giữ vị trí cuộn ±24px và trạng thái mở |
| UI-136 | B1 | VERIFIED_LOCALLY | KẾ_THỪA · cần retest cuối | «Chia sẻ» trên web không làm gì; có Web Share thì gửi chuỗi `rudi://` · có phản hồi; link https |
| UI-137 | B1 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Mở màn trong bằng link khi không phiên: chờ mãi, không lối đăng nhập · cả 4 route có «Đăng nhập» |
| UI-138 | B7 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Gọi Nếp lỗi: câu lỗi nằm sau sheet · lỗi trong sheet + thử lại |
| UI-139 | B7 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Bài ngắn: chạm đầu vào thân không làm gì · chạm mở chi tiết khi không bị cắt |
| UI-140 | B7 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Theo dõi chỉ cập nhật một thẻ của tác giả · mọi thẻ cùng tác giả |
| UI-141 | B7 (Go) | READY_FOR_QA | KẾ_THỪA · cần retest cuối | «Không quan tâm» không hoàn tác; «Xóa lịch sử đề xuất» một chạm · hoàn tác tại chỗ; xem lại bài đã ẩn; hỏi trước khi xoá |
| UI-142 | B7 (Go) | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Sửa bài thì bài rời bảng tin của chính tác giả · tác giả thấy bản đã duyệt + dải chờ duyệt |
| UI-143 | B7 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | 320: ảnh cắt 8px; dải cập nhật đè tab 13px · 0px cắt; 0px đè |
| UI-144 | B7 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Hàng duyệt in «· pending» · nhãn tiếng Việt |
| UI-145 | B7 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Tìm không ra và «Điều mình muốn giữ» không có trạng thái rỗng · có câu rỗng |
| UI-146 | B7 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Trang chủ đề không có «Quay lại» · TopBar lui + theo dõi chủ đề |
| UI-147 | B7 (Go) | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Thông báo không nói ai nhắc · tên người nhắc; lối vào ngoài sheet |
| UI-148 | B7 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Đọc bình luận lỗi không có «Thử lại» · «Thử lại» đọc lại |
| UI-149 | ADR (`adr-de-xuat/UI-149-…`, chờ chủ sản phẩm) | BLOCKED | KẾ_THỪA · cần retest cuối | «Đã chia» tính theo ngày: hai kèo trùng ngày cùng ghi một khoản · chỉ đề xuất ADR (mô hình dữ liệu tiền) |
| UI-150 | B9 | PLANNED | READY_FOR_QA · B9a | Lưu sổ lỗi: câu lỗi ở đầu màn, trên nút hơn 1000px · lỗi thấy ngay sau khi chạm |
| UI-151 | B9 (Go) | PLANNED | READY_FOR_QA · B9a | Kèo chưa tới ngày vẫn có «Khép cuộc đi»; chạm thì 409 · `can_end` false + lý do; không nút |
| UI-152 | B9 | PLANNED | READY_FOR_QA · B9a | Thành viên thấy bộ chọn loại mà không có tác dụng · không bộ chọn với người không tổ chức |
| UI-153 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Kệ trống chỉ có một câu, không hành động · một hành động qua EmptyState |
| UI-154 | B9 (Go) | PLANNED | READY_FOR_QA · B9a | Tên trang tự xếp là «2026-09-29» · không ngày ISO |
| UI-155 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Long poll trả về thì tường cắt về trang đầu · giữ đủ bài sau sự kiện hoặc 30s |
| UI-156 | B9 (Go) | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Bài Cộng đồng có ảnh lên tường không ảnh · thẻ tường có ảnh |
| UI-157 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Lỗi thao tác sổ hành trình nằm ở cuối sổ · thấy ngay sau khi chạm |
| UI-158 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Không còn lối xoá bình luận của mình ở trang viết · xoá có hỏi |
| UI-159 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Câu xác nhận đăng lại ngoài khung · trong khung nhìn |
| UI-160 | B9 (Go) | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | «MỚI MỞ» nhớ theo máy · máy mới không «MỚI MỞ» huy hiệu đã thấy |
| UI-161 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Chạm huy hiệu thứ tư không phản hồi mà vẫn gửi PATCH · không PATCH; nói lý do |
| UI-162 | B9 | PLANNED | RESERVED_CLAUDE_B9 · patch dở, chưa bàn giao | Mục «Thành tích…» mở màn «Hành trình» · tên mục khớp màn tới |
| UI-163 | B1 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Gói bối cảnh mặc định 40 tin, gấp đôi mức 20 của ADR-0046 · chip ≤20; gói = số trên chip |
| UI-164 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Chip nói «gửi như tin thường» mà vẫn hiện «Đang hỏi Rủ Đi AI…» · không khối AI khi AI chưa sẵn sàng |
| UI-165 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Chữ hiện dần, «đang đọc/nghĩ», lỗi Nếp không trong vùng aria-live · vùng live lịch sự, báo một lần |
| UI-166 | B2 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Tấm «Xem» trượt dưới dải ghim · sheet phủ dải; trần 82% cả sheet |
| UI-167 | B6 | READY_FOR_QA | KẾ_THỪA · cần retest cuối | Ở 320 chip gãy dòng, ✦ đứng riêng · ✦ cùng dòng; chip ≤36 cao |
