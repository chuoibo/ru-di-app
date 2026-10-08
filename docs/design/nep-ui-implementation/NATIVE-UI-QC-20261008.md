# Audit native và độ hoàn thiện thị giác — 08/10/2026

**Chưa nghiệm thu toàn frontend.** Theo quyết định người dùng, chỉ thao tác app
Rủ Đi trong Android emulator có cửa sổ. Không dùng web để thay bằng chứng native.
Mỗi màn/layer phải vừa chạy đúng vừa đẹp tương xứng mockup; giữ riêng hai kết quả.
Rule lâu dài ở [AGENTS.md](../../../AGENTS.md#cổng-chất-lượng-thị-giác-ui--quyết-định-người-dùng-08102026).

Đây là ledger native bổ sung cho catalogue 62 cửa/2 layout ở
[ROUTE-COVERAGE.md](./ROUTE-COVERAGE.md). Các ô «bị chặn web cuối» trong catalogue
là checkpoint lịch sử `811c893f`, không mô tả trạng thái quyền hoặc Android hiện tại.
Không đổi nhãn ảnh/capture lịch sử thành source mới.

## Nguồn và giới hạn

Hai emulator riêng `5558`/`5560`, 320×640 portrait, native Rust/JNI thật; tài khoản và
ảnh hoàn toàn tổng hợp. APK build cây sạch `b87a16f1`, SHA256
`34e48965242785651cc982199e4b3d823d45d0b4b30a79a98a622080efd63967`.
Go runtime `54b5dc0e`; JS có fingerprint riêng theo từng lượt. APK/Go không được
gán thành build của SHA JS. Không đụng audit dài đang được bảo vệ.

Evidence nằm ngoài mọi worktree:
`/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/`.
Chỉ receipt, source và tài liệu không chứa credential được bàn giao trong repo.
Ảnh có capability hoặc credential vẫn giữ riêng, không trình bày/commit.

## Các phần đã kiểm và phần còn thiếu

| Nhóm | Chức năng native đã có bằng chứng | Thị giác đối chiếu mockup | Còn phải kiểm/hoàn thiện |
|---|---|---|---|
| A — Welcome/auth | Bốn chương, swipe/reset, Login; font130/200, giảm chuyển động, tối; đăng nhập fixture thật | Welcome `34d57ca9`: 15/16; ba finding nhãn/art/fold resolved; CTA→Login warm `3b71838e` có chuỗi frame được mở | Register/reset/error/keyboard toàn bộ; cold/back/cancel/background/reduced lifecycle; không suy toàn motion |
| B — Gu/onboarding | Có baseline native và bỏ qua onboarding ở login fixture | Chưa review đủ state native đúng source cuối | Save/restore/cancel/validation, keyboard/chữ lớn và review |
| C — Explore/destination | Màn native thật làm điểm vào các flow | Chưa nhận toàn composition/state đạt | Search/filter/save/no-result/ảnh thiếu/credit; provider AI thật chưa cấu hình |
| D — Place/chọn kèo | Source và baseline có trong recovery | Chưa nhận đủ native so mockup | Chọn place/context/kèo, lỗi/empty và mọi layer |
| E — Plan/outing/map | Baseline flow22–29 có bước outing; chưa phải ma trận đầy đủ | Chưa review đủ timeline/map/layer đúng source cuối | Reorder/pin/map provider/permission/conflict/partial creation |
| F — Create/check-in | Tạo hub/tab thật đã được mở trên native | Chưa chấm toàn bộ sheet/check-in | Từng entry, context/audience, keyboard/location/cancel |
| G — Chat/story | MLS hai chiều và media nhận/relaunch hai emulator; poll create/vote/tie/switch/author-close/relaunch; Story thay ảnh/draft/đăng/pause/Back/Keep/DELETE thật; Ngọc bạn đọc/seen/nonauthor | Khung media `2fc1c73f` mở ảnh so board03; Story preview/viewer/question `c9a3f541` 14/16, ship chỉ hai P2 và state đã chụp | Header/history-empty copy; motion/retry/expiry/unauthorized audience/pendingDELETE/TalkBack; poll→tờ hẹn thật; reseal/reverse media |
| H — Groups/friends | Thành viên `3b71838e`: 2 active/0 invited, tên thật, không đổi vai trò | Root mở ảnh inline: không đẩy nhãn/CTA khỏi hàng; chưa chấm cả family | Invite/accept/reject/leave/last-admin, friend pending/block và empty/error |
| I — Hai người | Chưa nhận full native pair workflow | Chưa review native đủ | Hai phiên UI, tờ hẹn/accept/promote, chọn người và các nhánh |
| J — Moments/album/wall | Story không thay bằng chứng khoảnh khắc/album/tường | Chưa review native đủ | Media/crop/draft/retry/audience/view/album/delete, heart/comment đúng quyền |
| K — Ending/diary | Bằng chứng web lịch sử không tính native | Chưa review native đủ | Ending→reader→post, draft/error/layout/privacy và provider thật |
| L — Profile/people | Profile edit/keyboard/cancel; avatar picker/upload/cancel thật; crop native `2c1189d7` cancel/confirm và server640×640 | Root mở profile/editor/after; chưa rubric đủ own/other/pending/blocked | Avatar error/retry/permission, propagation giữa máy; people/achievements; chữ lớn và states |
| M — Bill | Focused native822 nhập/preview/ghi bill200.000đ, phần75.000/125.000; witness thật; runner toàn bảng vẫn đỏ | Có ảnh đã mở; chưa nhận composition/state đầy đủ | Native whole-flow/canary cùng harness, scan camera/OCR/provider, font/keyboard/errors |
| N — Settlement/finance | Focused native822 batch75.000đ, xác nhận nhận và ledger về0; receiver confirmation không là bank evidence | Chưa nghiệm thu toàn state/layer native | Bảng đầy đủ, role/wait/conflict, share handoff và visual |
| O — Guest ngoài Expo | Cổng riêng, không gán emulator Expo thành guest proof | Không mở browser theo yêu cầu hiện tại | Fresh guest/oracle parity; phương thức visual guest phải giữ đúng phạm vi được phép |
| P — Community | Native deeplink `c9a3f541` vào feed thật; chưa kiểm actions | Root mở baseline đầu feed; chưa rubric/reference toàn state | Feed/topic/post/edit/comment/report/moderation/filter/cursor và lỗi |
| Q — Keeps/notifications/search | Chưa nhận đầy đủ native | Chưa review native đủ | Từng route/action/error/empty và related sheets |
| R — Settings/account/privacy | Theme thật Sáng/Tối; device list read thật; login Ngọc lại thật; avatar crop qua settings | Root mở settings light/dark/device/profile, chưa rubric toàn family | Logout phải thu hồi đúng device còn là finding mở; remote session/revoke/privacy/block/delete-account; không xoá mọi máy để ép xanh |
| S — Root/tab/alias | Hai native runtime có ownership/fingerprint; tab/Back/relaunch thực trong các flow đã nêu | Welcome transfer được review có giới hạn | Deep-link/alias/auth-expiry/foreground/reduced lifecycle; keyboard/nav trong toàn matrix |

Không cộng các route đã mở thành số state/action đã đạt. Provider chưa có cấu hình
không thay bằng response giả. iOS, phần cứng thật và review crypto độc lập vẫn là
cổng riêng, không suy từ emulator hoặc focused unit.

## Đối chiếu cụ thể

Rubric gốc tám trục ×2 ở prompt §13; identity, artwork, hierarchy và state phải2,
tổng ít nhất14/16. Điểm phục vụ tìm lỗi, không là chứng nhận khách quan. Mỗi điểm
phải có ảnh/trạng thái đã mở và nhận xét; motion cần clip/chuỗi frame thực.

- Welcome: [review ảnh34d57ca9](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-peer-r1/welcome-independent-verdict-34d57ca9.md);
  [review CTA warm3b71838e](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-peer-r1/welcome-independent-motion-3b71838e.md).
- Story: [review3b71838e](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-3b71838e/story-independent-verdict-3b71838e.md)
  giữ lịch sử **fix**. [Verdict passc9a3f541](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-c9a3f541/story-independent-verdict-c9a3f541.md)
  mở17afterảnh và chốt **ship chỉ hai P2**: hierarchy/header isolation resolved;
  câu hỏi13→14/16, preview/viewer14/16. Chạm header thật/Back/Keep và DELETE native
  đã kiểm; chưa pendingDELETE/TalkBack/motion/retry/expiry/composer matrix.
- Avatar crop `2c1189d7`: editor thật được mở; Back exit0/6,422s giữ nguyên version
  và hash ảnh640×800. Confirm exit0/13,531s đổi version, GET200 ảnh640×640.
  Root mở editor/cancel/after; không suy crop/permission/retry cho mọi media.
- Bạn đọcNgọc `c9a3f541`: accepted friend trong danh sách nativeMinh; viewer không
  có nút xoá, đóng đổi vòng sang đã xem,16,835s exit0; root mở hai ảnh. Avatar cắt
  hiện trên rail và HTTPNgọc GET200 cùng hash640×640; chưa chứng minh người lạ,
  block/revoke/expiry hoặc mọi audience.

## Cổng trước landing

Main chưa nhận batch này. Full mobile sạch gần nhất `2fc1c73f`:1620/1620,
0fail/skip/cancel/todo; không gán cho sửa sau đó. Plugin detector/craft-floor thiếu,
không ghi detector clean. Còn full strict gate32 ở cây sạch SHA cuối,
identity/canary/ít nhất hai mutant độc lập cùng harness, fresh guest parity,
state/native matrix và Impeccable finish phù hợp tập capture cuối. Không mở web
hoặc chạy benchmark/load để thay những phần thiếu.
