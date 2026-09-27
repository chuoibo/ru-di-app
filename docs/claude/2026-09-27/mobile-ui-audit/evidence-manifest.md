# Evidence manifest

Ảnh bằng chứng đã commit của audit (26 ảnh, tổng 3.71 MiB). Mỗi ảnh được ghim path + sha256 trong `.repo-guard-allowlist.json`. Ảnh gốc PNG, số đo JSON và video nằm ngoài git.

| Ảnh | Mô tả | Byte | sha256 (8 ký tự đầu) |
|---|---|---|---|
| [EV-F00-CREATE-LANH-C1](evidence/EV-F00-CREATE-LANH-C1.jpg) | UI-010: mở lạnh /create trên web, rơi về Khám phá không có khay (C1) | 145881 | `3614f29d` |
| [EV-F00-DT-dalat-0-la](evidence/EV-F00-DT-dalat-0-la.jpg) | UI-002: người đã đăng nhập mở URL lạ thấy bìa Welcome (C1) | 202697 | `749785cd` |
| [EV-F00-DT-la-cta-C1](evidence/EV-F00-DT-la-cta-C1.jpg) | UI-002: bấm «Rủ Đi thôi!» dẫn tới Đăng nhập dù phiên còn (C1) | 135092 | `3397acf4` |
| [EV-F00-KT-back-aria-C1](evidence/EV-F00-KT-back-aria-C1.jpg) | UI-005: sau khi đóng khay tạo bằng Back, phần tử bọc cả màn Khám phá và thanh tab còn aria-hidden=true (C1) | 161634 | `2c21522f` |
| [EV-F00-KT-cham-dup-C1](evidence/EV-F00-KT-cham-dup-C1.jpg) | UI-006: chạm «+» hai lần cách 60 ms, chạm thứ hai mở nhầm «Rủ một người đi chơi» (C1) | 123609 | `1b22e4ab` |
| [EV-F00-KT-mo-C1](evidence/EV-F00-KT-mo-C1.jpg) | Khay tạo mở ở C1 (baseline; UI-008 thứ tự Tab) | 93119 | `02cc5efb` |
| [EV-F00-KT-mo-C2](evidence/EV-F00-KT-mo-C2.jpg) | UI-007: khay tạo ở 320×640 cao 92% (C2) | 64825 | `afa6176c` |
| [EV-F00-KT-mo-C7](evidence/EV-F00-KT-mo-C7.jpg) | Khay tạo trên tablet 1024×1366 tối (C7) | 319227 | `7323ada2` |
| [EV-F00-KT-mo-C8](evidence/EV-F00-KT-mo-C8.jpg) | UI-007: khay tạo ở cửa sổ 390×460 cao 96% (C8) | 48576 | `90f561bc` |
| [EV-F00-M1-lan1-C1](evidence/EV-F00-M1-lan1-C1.jpg) | L04: M1 trong khay tạo, lần mở đầu trong phiên (chưa kết luận được) | 219360 | `105f1c89` |
| [EV-F00-M1-lan2-C1](evidence/EV-F00-M1-lan2-C1.jpg) | L04: M1 trong khay tạo, lần mở thứ hai trong phiên (chưa kết luận được) | 276284 | `695e3dd3` |
| [EV-F00-MO01-push-C1](evidence/EV-F00-MO01-push-C1.jpg) | UI-015 và MO01: khung hình Cá nhân → Cài đặt; web cắt thẳng; Cài đặt nháy «Bạn/B» và công tắc lật (C1) | 150936 | `b647ec00` |
| [EV-F00-MO04-dong-khay-C1](evidence/EV-F00-MO04-dong-khay-C1.jpg) | UI-013: khung hình đóng khay tạo, panel còn lộ ở khung cuối rồi biến mất (C1) | 167885 | `d59c357c` |
| [EV-F00-MO04-mo-khay-C1](evidence/EV-F00-MO04-mo-khay-C1.jpg) | MO03/MO04: khung hình mở khay tạo (C1) | 307867 | `ad9c30ea` |
| [EV-F00-NEP-BANG-C1](evidence/EV-F00-NEP-BANG-C1.jpg) | UI-012: bảng Nếp, chip gợi ý cao 36dp (C1) | 102425 | `b522849c` |
| [EV-F00-NEP-BANG-C8](evidence/EV-F00-NEP-BANG-C8.jpg) | Bảng Nếp ở cửa sổ thấp, ô hỏi vẫn trong màn (C8) | 59850 | `c3b27a24` |
| [EV-F00-NEP-CHE-CHIP-C2](evidence/EV-F00-NEP-CHE-CHIP-C2.jpg) | UI-014: mép Nếp đè chữ chip ở 320×640 (C2) | 94441 | `3ff283c2` |
| [EV-F00-NEP-VE-C1](evidence/EV-F00-NEP-VE-C1.jpg) | UI-011: bảng Nếp sau khi bấm «Vẽ» trên web, không có phản hồi (C1) | 103552 | `2d348a28` |
| [EV-F00-NEP-mep-C1](evidence/EV-F00-NEP-mep-C1.jpg) | Mép Nếp khi nghỉ ở Khám phá, lộ 10dp (C1) | 145881 | `3614f29d` |
| [EV-F00-RAIL-chibao-cat](evidence/EV-F00-RAIL-chibao-cat.jpg) | UI-004: rail ở 768dp, vạch chỉ báo lệch khỏi tab Lên plan / Tin nhắn / Cá nhân | 48302 | `c5fd10a9` |
| [EV-F00-RESUME-CHAM-1800ms-C1](evidence/EV-F00-RESUME-CHAM-1800ms-C1.jpg) | UI-009: /plan lúc 1,8 s khi khôi phục phiên bị trễ 5 s: vùng nội dung trống (C1) | 19726 | `a5dae17a` |
| [EV-F00-TAB-ghep-a](evidence/EV-F00-TAB-ghep-a.jpg) | Baseline thanh tab ở C1, C2, C3, biên 599, biên 600 (Khám phá) | 221550 | `e89d7318` |
| [EV-F00-TAB-ghep-b](evidence/EV-F00-TAB-ghep-b.jpg) | Baseline rail ở C6, biên 839, biên 840, C7 (Khám phá) | 223331 | `9d888e93` |
| [EV-F00.S01-BASE-ghep](evidence/EV-F00.S01-BASE-ghep.jpg) | Baseline Welcome khi chưa đăng nhập ở C1, C2, C3 | 165496 | `73046663` |
| [EV-F01-O-NHAP-44-C1](evidence/EV-F01-O-NHAP-44-C1.jpg) | UI-001: ô số điện thoại cao 44dp ở Đăng nhập (C1) | 143787 | `f614a1ec` |
| [EV-F01.S02-BASE-ghep](evidence/EV-F01.S02-BASE-ghep.jpg) | Baseline Đăng nhập ở C1, C2, C3 | 142565 | `ce3fe306` |
