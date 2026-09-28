# Inventory UI: app mobile RuDi

- Cây đo: `7ea1a7c` (trùng `origin/main` lúc bắt đầu audit, 27/09/2026).
- Phạm vi: `apps/mobile` (Expo 57, expo-router, react-native-web cho bản web).
- Nguồn: 55 file route trong `apps/mobile/app/`, màn và kit trong `apps/mobile/src/rudi/`.
  Lập bằng đọc mã (STATIC), rồi đối chiếu runtime web trong từng feature (cột «Runtime web»).
- Vai trò tài liệu: danh sách đầy đủ những gì PHẢI được đối chiếu. Ma trận coverage
  (`coverage-matrix.md`) có một hàng cho mọi screen và layer ở đây; hàng nào chưa chạy thì là
  NOT_TESTED hoặc BLOCKED, không bị bỏ.

## 0. Quy ước

- ID:
  - Feature `F00`–`F11`, E2E `E1`–`E6`.
  - Screen `Fnn.Smm`.
  - Lớp UI (overlay, sheet, tray, panel) `Lnn`.
  - Motion `MOnn`.
- Kiểu trình bày lấy từ `Stack.Screen` trong `app/_layout.tsx`:
  - mặc định là push, `slide_from_right`;
  - «giảm chuyển động» chuyển thành `none`.
- «Live / demo»: phần lớn route chọn màn live khi có phiên, màn demo «Team Đà Lạt» khi chưa đăng nhập.
- Điều kiện hiển thị: phiên, vai trò (người lập nhóm / thành viên), dữ liệu (rỗng / có), nền tảng.

## 1. Feature → Screen

### F00 Vỏ toàn cục

| ID | Route | Màn / file | Trình bày | Vào từ | Ghi chú |
|---|---|---|---|---|---|
| F00.S01 | `/` | redirect theo phiên (`duong-vao.ts`) | — | mở app | không phiên → `/welcome`; có nhóm → `/explore`; không nhóm → `/messages` |
| F00.S02 | `[...legacy]` | redirect `/welcome` | — | URL lạ | |
| F00.S03 | `(tabs)/_layout` | `ui/RudiTabBar.tsx`: 4 tab + nút «+» | tab, `fade` | mọi màn tab | <600dp: thanh dưới; ≥600dp: rail trái |
| F00.S04 | `/create` | `screens/Create.tsx` (khay tạo, 5 vật thể) | `transparentModal` + `fade` | «+» | mở lạnh: thay bằng `/explore` rồi push lại `/create` |
| F00.S05 | toàn cục | `nep/NepNoi.tsx`, `NepDock.tsx`, `NepBang.tsx` | lớp phủ trên Stack | mép phải | ẩn khi có sheet, trên màn tiền chỉ còn mép trơn, ẩn ở màn đăng nhập và bản đồ |

### F01 Vào cửa

| ID | Route | Màn / file | Section / component chính | Trạng thái cần phủ | Hành động |
|---|---|---|---|---|---|
| F01.S01 | `/welcome` | `screens/Welcome.tsx` | bìa chàm, logo, đường 4 mốc, pager 4 trang, «Rủ Đi thôi!», «Tìm hiểu thêm», dấu vân cây | mở bìa, 4 trang pager, giảm chuyển động | vuốt pager, chạm chấm, CTA (lật bìa → `/login`) |
| F01.S02 | `/login` | `screens/auth/Login.tsx` | ô số điện thoại, «Gửi mã», «Tôi có lời mời», Google (ẩn trên web), cửa fixture (chỉ bản dev) | trống, số sai, đang gửi, lỗi máy chủ, 429 | nhập, gửi, sang `/moi` |
| F01.S03 | `/otp` | `screens/auth/Otp.tsx` | 6 ô OTP, đếm ngược gửi lại 60 s, «đổi số» | sai mã, hết hạn, quá lượt, đang xác minh | nhập, dán, gửi lại, đổi số |
| F01.S04 | `/moi` | `screens/LoiMoi.tsx` | ô mã lời mời | mã sai, hết hạn, đúng | nhập, xác nhận |
| F01.S05 | `/personalization` | `screens/Onboarding.tsx` | chip gu, 4 mức chi (có «Trên 500K · Rộng tay»), lưu | người mới, người sửa lại | chọn, bỏ chọn, lưu |

### F02 Khám phá

| ID | Route | Màn / file | Section / component chính | Trạng thái | Hành động |
|---|---|---|---|---|---|
| F02.S01 | `(tabs)/explore` | live `explore/ExploreLive.tsx`; demo `Discovery.tsx` | dòng thành phố, sân khấu thành phố (Skia/SVG), ô tìm + nút AI, hàng chip lọc, «N nơi ở …», danh sách `HangDiaDiem` | catalog rỗng, lọc ra 0, AI chưa cấu hình, ảnh hỏng/chậm, lỗi mạng | tìm, lọc, kéo nghiêng sân khấu, tim, mở quán, đổi thành phố |
| F02.S02 | `/destinations` | `explore/DiemDenScreen.tsx` | lưới bưu thiếp 15 thành phố | 1/2/3 cột theo bề rộng | chọn → `router.back()` |
| F02.S03 | `/places/[id]` | live `explore/PlaceDetailLive.tsx`; demo `Discovery.tsx` | sân khấu quán, ảnh + dòng nguồn, thông tin, chỉ đường, thêm vào kèo, «Rủ <tên> tới đây» (chỉ hiện khi người xem có sổ đôi đang mở, tối đa 3), thêm kỷ niệm | ảnh hỏng, không ảnh, tên dài | chỉ đường (link ngoài), `/outings/chon`, `/groups/[id]/to-giay`, `/moments/new` |
| F02.S04 | `/ai-match` | demo `Discovery.tsx` | match gu cả nhóm | có phiên → redirect `/explore` | |

### F03 Lên plan · Kèo · Hành trình

| ID | Route | Màn / file | Section / component chính | Trạng thái | Hành động |
|---|---|---|---|---|---|
| F03.S01 | `(tabs)/plan` | live `keo/PlanLive.tsx`; demo `Outing.tsx` `TripTimelineScreen` | danh sách kèo theo nhịp, CTA tạo kèo | rỗng, nhiều kèo | mở kèo, tạo kèo |
| F03.S02 | `/outings/new` | live `keo/CreateOutingLive.tsx`; demo `Outing.tsx` | thiệp mời, `CauRu`, lá lịch `ChonNgayLich`, `BanXoay`, chọn người, CTA dính đáy | validate, gửi, lỗi | điền, chọn ngày giờ, tạo |
| F03.S03 | `/outings/[id]` (Lịch trình) | `keo/OutingLive.tsx` | vé kèo, thành viên, các chặng `HangChang` + `ReorderList`, check-in, công tắc «Lịch trình · Bản đồ», chia bill | 0 chặng, 12 chặng, tên dài | kéo đổi thứ tự, thêm chặng, gắn quán, check-in, chia bill |
| F03.S04 | `/outings/[id]` (Bản đồ) | `hanh-trinh/SoHanhTrinh.tsx` + `ManHinhHanhTrinh.tsx` + `BanDo.tsx` | bản đồ maplibre, chip ngày, cách đi, «Tính lại đường», trang ngày gập được, dải chặng đánh số | ngày trống, nhiều chặng, cụm | chọn chặng, sửa ngày, điểm hẹn (chuột phải trên web), về Lịch trình |
| F03.S05 | `/outings/chon` | `keo/PickOutingLive.tsx` | danh sách kèo để thêm quán | rỗng | chọn kèo |
| F03.S06 | `/check-ins/new` | demo `Outing.tsx` | | có phiên → redirect `/plan` | |
| F03.S07 | `/trips/[id]/itinerary` | demo `Group.tsx` (AI itinerary) | | có phiên → redirect `/outings/[id]` | |
| F03.S08 | `/trips/[id]/timeline` | demo `Outing.tsx` `TripTimelineScreen` | | **không** kiểm phiên | |

### F04 Tiền

| ID | Route | Màn / file | Section / component chính | Trạng thái | Hành động |
|---|---|---|---|---|---|
| F04.S01 | `/smart-split/[id]/review` | live `chia-bill/ChiaBillLive.tsx` (5 bước, `LatTrang`); demo `Bill.tsx` | hoá đơn giấy, món, bàn gán món `BanGanMon`, xem lại, chia, ghi sổ | món dài, tiền 8 chữ số, nhiều người, chưa gán hết | nhập món, chọn ảnh bill, kéo món vào ghế hoặc chạm, sang/lùi bước |
| F04.S02 | `/smart-split/[id]/assignment` | demo `Bill.tsx` | | có phiên → redirect review | |
| F04.S03 | `/settlements/[id]` | `Bill.tsx` `SettlementScreen` | mũi tên mực, dòng tiền, xem phụ | tên dài, nhiều người | mở xem phụ, quay lại |
| F04.S04 | `/batches/[id]` | `dot-thu/DotThuLive.tsx` | trang sổ kẻ, tiến độ, chia sẻ | chưa phát, đã phát, đủ | phát, chia sẻ (`Share` trên web) |
| F04.S05 | `/finance` | `Profile.tsx` `FinanceScreen` | tổng tiền, dòng sổ | rỗng, nhiều | mở quyết toán |

### F05 Tin nhắn · Chat

| ID | Route | Màn / file | Section / component chính | Trạng thái | Hành động |
|---|---|---|---|---|---|
| F05.S01 | `(tabs)/messages` | live `groups/Conversations.tsx`; demo `Group.tsx` | dải story, lời mời đến, danh sách nhóm và DM, lập nhóm, thêm bạn, «Tôi có lời mời» | 0 nhóm, nhiều nhóm, lời mời chờ | mở chat, lập nhóm, xem story |
| F05.S02 | `/groups/[id]/chat` nhóm | `chat/GroupChatLive.tsx` | header (gáy nhóm, thành viên, «⋯»), nhãn «Chưa mã hoá đầu cuối», thanh ghim, danh sách tin, thẻ bình chọn, thẻ AI, hàng tờ giấy đôi, composer + «+» + sticker | rỗng, dài, đang gửi, lỗi gửi, bị xoá, trả lời | gửi, gửi ảnh, sticker, nhấn giữ tin, trả lời, xoá, thả cảm xúc, mở khay |
| F05.S03 | `/groups/[id]/chat` DM | như trên, ngữ cảnh đôi | dải `HangToGiaySong` | bị chặn, chưa là bạn | như trên |
| F05.S04 | `/votes/[id]` | demo `Group.tsx` | | có phiên → redirect `/messages` | |

### F06 Nhóm · Người

| ID | Route | Màn / file | Section / component chính | Trạng thái | Hành động |
|---|---|---|---|---|---|
| F06.S01 | `/groups/new` | `groups/New.tsx` | bìa nhóm, tên, chọn bạn | tên dài, không bạn | tạo |
| F06.S02 | `/groups/[id]/members` | `groups/Members.tsx` | danh sách, vai trò, mời, tường, album | 20 người, 1 người | mời, đổi vai trò. Đo ở checkpoint 7: hàng không mở được hồ sơ (UI-076) |
| F06.S03 | `/groups/[id]/invite` | `groups/Invite.tsx` | phong bì, số điện thoại | số sai, đã mời | mời |
| F06.S04 | `/groups/empty` | redirect `/messages` | | | |
| F06.S05 | `/friends` | `friends/Friends.tsx` | 3 phân đoạn (bạn, đã nhận, đã gửi) | rỗng, nhiều | chấp nhận, từ chối, nhắn |
| F06.S06 | `/friends/add` | `friends/AddFriend.tsx` | ô số, kết quả | không thấy, đã là bạn | gửi lời mời |
| F06.S07 | `/people/[id]` người khác | `nguoi/HoSoNguoiScreen.tsx` | hộ chiếu, tường, «Kết bạn», «⋯» | chưa là bạn, là bạn, bị chặn | kết bạn, nhắn, rủ, chặn, báo cáo |
| F06.S08 | `/people/[id]` chính mình | như trên | tường mình, đăng bài | | `/posts/new` |

### F07 Sổ hai người

| ID | Route | Màn / file | Section / component chính | Trạng thái | Hành động |
|---|---|---|---|---|---|
| F07.S01 | `/hai-nguoi/chon-nguoi` | `hai-nguoi/ChonNguoi.tsx` | danh sách bạn | 0 bạn, nhiều | chọn → `to-giay` |
| F07.S02 | `/groups/[id]/to-giay` | `hai-nguoi/KhongGianGiay.tsx` | bìa sổ (lật), giao kèo, tờ chì, tờ tuần, «Chi tiêu chung» | chưa lập sổ, đang chờ (mình hay người kia đề nghị), đã lập, bị từ chối. Đo ở checkpoint 8: lỗi đọc hiện như chưa lập sổ (UI-083); không phiên thì hiện sổ demo không nhãn (UI-082); `?ru=1&cho=` từ «Rủ … tới đây» | đề nghị, đồng ý, sửa, giữ một điều |

### F08 Kỷ niệm · Media

| ID | Route | Màn / file | Section / component chính | Trạng thái | Hành động |
|---|---|---|---|---|---|
| F08.S01 | `/groups/[id]/wall` | live `ky-niem/GroupWallLive.tsx`; demo `Memories.tsx` | ảnh in nghiêng, check-in, tim, bình luận | rỗng, ảnh dọc/ngang | check-in, thêm kỷ niệm |
| F08.S02 | `/groups/[id]/album` | `ky-niem/AlbumLive.tsx` | kệ album, reel | rỗng, nhiều | mở ảnh (PhotoViewer) |
| F08.S03 | `/trips/[id]/album` | `AlbumLive.tsx` / demo `Memories.tsx` | | | |
| F08.S04 | `/moments/new` | live `ky-niem/ShareMomentLive.tsx` | chọn ảnh, chú thích, gửi | chưa chọn ảnh, đang tải lên, lỗi | chọn ảnh (file chooser), gửi, huỷ |
| F08.S05 | `/stories/new` | `story/DangStoryScreen.tsx` | polaroid, chọn ảnh | | đăng |
| F08.S06 | `/stories/[personId]` | `story/XemStoryScreen.tsx` | thanh tiến độ, vùng chạm trước/sau, xoá | hết hạn, của mình | chạm, xoá, đóng |
| F08.S07 | `/posts/new` | `nguoi/DangBaiScreen.tsx` | soạn bài, ảnh | | đăng |
| F08.S08 | `/posts/[id]` | `tuong/BaiChiTietScreen.tsx` | bài, bình luận, cảm xúc, báo cáo | bình luận dài, nhiều | bình luận, báo cáo |
| F08.S09 | `/achievements` | live `ky-niem/AchievementsLive.tsx` | con dấu thành tích | chưa có, có | |

### F09 Hồ sơ · Cài đặt

| ID | Route | Màn / file | Section / component chính | Trạng thái | Hành động |
|---|---|---|---|---|---|
| F09.S01 | `(tabs)/profile` | `Profile.tsx` + `profile/HoSoSong.tsx` | thẻ hộ chiếu, panel trong màn (tài khoản, sửa hồ sơ, đã lưu), lối vào bạn bè, thành tích, tài chính, cài đặt, đăng xuất | | mở panel, sửa, đăng xuất |
| F09.S02 | `/settings` | `cai-dat/CaiDatScreen.tsx` | ảnh đại diện, giao diện Sáng/Tối/Hệ thống, sở thích, phiên, đã chặn, về Rủ Đi, xoá tài khoản | | đổi giao diện, đổi ảnh |
| F09.S03 | `/settings/phien` | phiên đăng nhập | danh sách phiên | 1 phiên, nhiều | thu hồi |
| F09.S04 | `/settings/da-chan` | người đã chặn | | rỗng, có | bỏ chặn |
| F09.S05 | `/settings/ve-rudi` | về Rủ Đi (bản nháp) | | | |
| F09.S06 | `/settings/xoa-tai-khoan` | `XoaTaiKhoan.tsx` | xác nhận hai bước | | xoá (chỉ trên persona dùng một lần) |

### F10 Bảng QA dev (chỉ bản dev với `EXPO_PUBLIC_RUDI_FIXTURE=1`)

| ID | Route | Nội dung | Ghi chú |
|---|---|---|---|
| F10.S01 | `/dev/ui-lab` | renderer, sân khấu, Nếp, primitive giấy, album 7 trạng thái, ảnh hỏng/tên dài, sticker, hàng đợi gửi, ô tìm dài, kéo đổi thứ tự, PhotoViewer | không có lối vào trong app |
| F10.S02 | `/dev/san-khau` | đầu màn sân khấu gập khi cuộn, 18 dòng dài | chỉ vào từ ui-lab |

### F11 Chế độ demo (chưa đăng nhập)

| ID | Route | Nội dung |
|---|---|---|
| F11.S01 | 4 tab khi chưa đăng nhập | Discovery, TripTimeline, GroupChat demo, Profile demo |
| F11.S02 | route demo | `/finance`, `/settlements/[id]`, `/trips/[id]/timeline`, `/trips/[id]/itinerary`, `/votes/[id]`, `/check-ins/new`, `/ai-match`, `/smart-split/[id]/assignment` |

## 2. Lớp UI (overlay, sheet, tray, panel, bộ chọn)

`Sheet` là `src/rudi/ui/Sheet.tsx`, dùng chung cho các sheet bên dưới. Trên web nó có:
- `role=dialog` và `aria-modal`;
- bẫy focus, các nhánh ngoài được đặt `inert` và `aria-hidden`;
- đóng bằng Esc, nút «Đóng bảng», nền «Đóng», kéo tay cầm >90dp hoặc fling >900dp/s;
- cao tối đa 82% cửa sổ.

Mặc định không né bàn phím.

| ID | Lớp | File | Chủ | Ô nhập | Đóng bằng | Ghi chú |
|---|---|---|---|---|---|---|
| L01 | Khay tạo «Mình làm gì tiếp?» | `screens/Create.tsx` | F00.S04 | không | Sheet | hành động dùng `router.replace`; M1 |
| L02 | Mép Nếp (dock) | `nep/NepDock.tsx` | toàn cục | — | tự thu sau 6 s, vuốt phải | 10dp, nơ đỏ |
| L03 | Bảng Nếp | `nep/NepBang.tsx` | toàn cục | có (đáy) | Sheet | `Alert.alert` «Nhờ Nếp vẽ?» (L03a) |
| L04 | Nếp diễn (M1–M8) | `ui/NepDien.tsx` | nhiều màn | — | chạm để bỏ qua | chiếm chỗ riêng |
| L05 | Thanh tab / rail | `ui/RudiTabBar.tsx` | F00.S03 | — | — | cao theo cỡ chữ |
| L06 | Hành động hồ sơ (chặn, báo cáo, lý do) | `nguoi/HanhDongHoSo.tsx` | F06.S07 | có | Sheet | |
| L07 | Báo cáo bài | `tuong/BaiChiTietScreen.tsx` | F08.S08 | có | Sheet | |
| L08 | Chặng mới | `keo/OutingLive.tsx` | F03.S03 | có + `BanXoay` | Sheet | |
| L09 | Gắn quán | `keo/OutingLive.tsx` | F03.S03 | tìm | Sheet | |
| L10 | Sửa ngày | `hanh-trinh/SoHanhTrinh.tsx` | F03.S04 | 6 ô + 2 công tắc | Sheet | không nằm trong khe overlay: đã đo ở checkpoint 4, nền không phủ đầu màn (UI-041); tiêu đề sheet «Những hẹn quan trọng» |
| L11 | Điểm hẹn | `hanh-trinh/SoHanhTrinh.tsx` | F03.S04 | có | Sheet | như L10; mở bằng chuột phải trên web, giữ ngón tay trên native (`onLongPress`) |
| L12 | Popup cụm bản đồ | `hanh-trinh/BanDo.tsx` (web) | F03.S04 | — | nút đóng | maplibre Popup |
| L13 | Trang ngày gập được | `hanh-trinh/ManHinhHanhTrinh.tsx` | F03.S04 | — | gập/mở | |
| L14 | Lá lịch tháng | `ui/ChonNgayLich.tsx` | F03.S02 | — | chọn ngày | ô ngày 44dp |
| L15 | Mặt quay giờ | `ui/BanXoay.tsx` | F03.S02, L08 | — | kéo | |
| L16 | Báo cáo tin | `chat/GroupChatLive.tsx` | F05.S02 | có | Sheet | trong vùng né bàn phím của chat |
| L17 | Cài đặt nhóm | `chat/CaiDatNhom.tsx` | F05.S02 | tên + 5 màu + Switch | Sheet | có «Rời nhóm» |
| L18 | Menu tin | `chat/MenuTin.tsx` | F05.S02 | — | Sheet | nút cảm xúc 44dp; xác nhận xoá trong sheet; sao chép |
| L19 | Khay sticker | `chat/KhaySticker.tsx` | F05.S02 | — | Sheet | |
| L20 | Khay công cụ | `chat/SoHen.tsx` `CongCuChat` | F05.S02 | có | X, Esc | inline |
| L21 | Khay tờ hẹn chung | `chat/ToHenChungKhay.tsx` | F05.S02 | — | chỉ X | inline. Đo ở checkpoint 6: Esc không đóng (UI-066), Back rời chat (UI-038) |
| L22 | Thẻ thông báo chat «Đã hiểu» | `chat/GroupChatLive.tsx` | F05.S02 | — | «Đã hiểu» | nhận lỗi cảm xúc, lỗi gửi ảnh, lỗi xoá, câu ý định sau khi gửi. Luôn thêm ở cuối chat (UI-069) |
| L23 | ~12 sheet sổ đôi | `hai-nguoi/*.tsx`, `KhongGianGiay.tsx` | F07.S02 | DeNghiSua 5 ô, RangBuoc 2, GiuMotDieu 1 | Sheet | Đo ở checkpoint 8: Cài đặt sổ (vòng đời đủ 7 cách đóng, C9, tablet), Loại sổ, Hai ô ràng buộc, Đóng sổ (mở rồi thôi, không đóng sổ), Lập sổ (hai phía), Sửa bản phác, Đề nghị sửa. Chưa tới: Bật «Một đôi», Giữ lại một điều, Ai lo tuần này, Gu hai bạn, xác nhận bỏ/rút/nghỉ/huỷ. Hàng điều hướng không đóng sheet (UI-087, đã xác nhận) |
| L24 | Check-in | `ky-niem/GroupWallLive.tsx` | F08.S01 | tìm + ô | Sheet | |
| L25 | Xem ảnh | `ui/PhotoViewer.tsx` (RN `Modal` duy nhất) | F08.S02, F08.S03 | — | «Đóng», Android back | pinch, pan, chạm đúp; không vuốt xuống để đóng |
| L26 | Xem story + xác nhận xoá | `story/XemStoryScreen.tsx` | F08.S06 | — | đóng modal | tự chuyển 5 s |
| L27 | Xác nhận xoá tài khoản | `XoaTaiKhoan.tsx` | F09.S06 | — | huỷ | hai bước |
| L28 | Tuỳ chọn chuyến (demo) | `Outing.tsx` | F11 | — | Sheet | |
| L29 | Nắp gấp `NapGiay` | `ui/NapGiay.tsx` | nhiều màn | — | gập | |
| L30 | Pager Welcome | `screens/Welcome.tsx` | F01.S01 | — | vuốt, chấm | carousel |
| L31 | Bộ chọn ảnh (file chooser trên web) | `ky-niem/chon-anh.ts`, `ChiaBillLive.tsx` | F04, F05, F08, F09 | — | huỷ | trên native là picker hệ thống |
| L32 | Chia sẻ (`Share`) | `dot-thu/DotThuLive.tsx` | F04.S04 | — | — | web: `navigator.share` nếu có. Đo ở checkpoint 5: trên web cả ba trường hợp (không có, chia sẻ xong, đóng khay) đều báo lỗi mạng (UI-049) |
| L33 | Link ngoài (chỉ đường) | `explore/PlaceDetailLive.tsx` | F02.S03 | — | — | |
| L34 | 3 route modal trượt từ dưới | `check-ins/new`, `moments/new`, `stories/new` | F03, F08 | có | vuốt xuống (iOS), back | không chặn mất bản nháp |
| L35 | Kit trạng thái: Skeleton, ErrorState, EmptyState | `ui/Skeleton.tsx`, `ErrorState`, `EmptyState` | mọi màn | — | — | lỗi là câu chữ có `aria-live` |
| L36 | Panel trong màn Hồ sơ (tài khoản, sửa, đã lưu) | `Profile.tsx` | F09.S01 | có | nút back trên màn | không phải route |

Không có trong app (N/A, đã rà mã):
- toast, snackbar, banner;
- dropdown, popover, tooltip;
- action sheet;
- date/time picker native;
- slider;
- camera viewfinder (`CameraView` không được mount);
- màn quyền `EmptyState kind="permission"` (không nơi nào dùng).

## 3. Motion inventory

| ID | Chuyển động | Nguồn | Kích hoạt |
|---|---|---|---|
| MO01 | Push/pop stack `slide_from_right` | `app/_layout.tsx` | điều hướng |
| MO02 | Đổi tab (`fade`) + vạch chỉ báo trượt | `RudiTabBar.tsx` | chạm tab |
| MO03 | Khay tạo: route fade + sheet lò xo + M1 | `Create.tsx` | «+» |
| MO04 | Sheet: nền mờ dần, panel lò xo, kéo, fling | `Sheet.tsx` | mở/đóng sheet |
| MO05 | Route modal `slide_from_bottom` | `app/_layout.tsx` | check-in, kỷ niệm, story |
| MO06 | Story: fade + tiến độ 5 s | `XemStoryScreen.tsx` | mở story |
| MO07 | Phản hồi nhấn (chỉ co giãn) | `ui/PressScale.tsx` | nhấn |
| MO08 | Skeleton lấp lánh (vòng lặp được phép) | `ui/Skeleton.tsx` | đang tải |
| MO09 | Lật bìa Welcome 84° + đường hiện dần | `Welcome.tsx` | CTA |
| MO10 | Lật trang giữa các bước bill | `ui/LatTrang.tsx` | sang bước |
| MO11 | Dock Nếp: rút/thu, kéo dọc, tự thu 6 s | `NepDock.tsx` | chạm mép |
| MO12 | Sân khấu pop-up: bật dựng khi mount, gập theo cuộn, mount lại khi bỏ lọc. Kéo nghiêng (`useThiSaiKeo`) chỉ nối ở bảng dev `ThuSanKhau` và `/dev/san-khau` (F10); nghiêng máy (ADR-0037 D3, chỉ native) chưa nối ở đâu (sửa ở checkpoint 3) | `SanKhau`, `KhungSkia`, `useThiSai` | mở tab Khám phá, bỏ lọc; kéo nghiêng ở F10 |
| MO13 | Nếp diễn M1–M8 | `NepDien.tsx`, `NepRoi` | sự kiện |
| MO14 | Đóng dấu (rơi 130 ms + chạm 60 ms) | `Stamp`, `DauLon`, `useNhipDau` | xác nhận |
| MO15 | Mực tự vẽ (chữ ký, sơ đồ chuyển) | `ChuKy`, `SoDoChuyen` | hiện màn |
| MO16 | Bìa sổ đôi mở | `SoBia`, `KhongGianGiay.tsx` | mở sổ; đo góc từng khung ở C1/C9 (checkpoint 8) |
| MO17 | Kéo món vào ghế | `BanGanMon` | kéo |
| MO18 | Kéo mặt quay giờ | `BanXoay` | kéo |
| MO19 | Nhấn giữ 250 ms rồi kéo đổi thứ tự | `ReorderList` | nhấn giữ |
| MO20 | Cross-fade SVG → Skia | `KhungSkia` | nạp Skia xong |
| MO21 | FadeIn khi xuất hiện | `ChonNgayLich`, `NapGiay`, `ExploreLive` | mở |
| MO22 | Đếm số tiền 200 ms | `Money.tsx` (`countUp`) | hiện số; grep ở checkpoint 5: không màn nào bật `countUp` (N/A) |
| MO23 | Camera bản đồ `fitBounds`/`easeTo` 200 ms | `BanDo.tsx` | chọn chặng |
| MO24 | Zoom ảnh: pinch, pan, chạm đúp | `PhotoViewer.tsx` | xem ảnh |
| MO25 | Pager Welcome | `Welcome.tsx` | vuốt |

Không dùng RN `Animated` hay `LayoutAnimation`; không có hàng vuốt (swipeable) và không có slider.

## 4. Route và lối vào đặc biệt

- Không có lối vào trong app:
  - `/dev/ui-lab`;
  - `/dev/san-khau`, chỉ vào được từ ui-lab.
- Chỉ là redirect: `/`, `[...legacy]`, `/groups/empty`.
- Chỉ demo, redirect khi đã có phiên: `/ai-match`, `/check-ins/new`, `/votes/[id]`,
  `/trips/[id]/itinerary`, `/smart-split/[id]/assignment`.
- Chỉ demo nhưng **không** redirect: `/trips/[id]/timeline`.
- Deep link `rudi://moi/<token>`: lưu mã vào module, rồi thay bằng `/moi`. Trên web, hiệu ứng deep
  link của `_layout` bị bỏ qua; URL trực tiếp vẫn vào được.
- Thông báo đẩy: chưa có mã trong app → N/A.
