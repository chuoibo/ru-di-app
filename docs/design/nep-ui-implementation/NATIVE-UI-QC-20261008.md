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
ảnh hoàn toàn tổng hợp. Primary5558 đã cài APK build cây sạch `2f20a301`, SHA256
`963bd5daa0be4348606158dd3c06848e7ef604e06ebbdf035136a718cae0d514`;
peer5560 vẫn APK `b87a16f1`, SHA256
`34e48965242785651cc982199e4b3d823d45d0b4b30a79a98a622080efd63967`.
Không gán APK mới hoặc JS9007 của primary cho peer chưa cập nhật.
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
| B — Gu/onboarding | `6f3f5af2`: save/reopen4 mục/ngân sách thật; Settings entry chọn2/3 khóa lưu, Back/cancel giữ server4; restore UI3/vua-phai, GET200 khớp; tách khỏi capture d3 | Root mở14 ảnh thao tác6f; `d3c4a3aa`: ship chỉ sửa nhãn giữa, resolved; reviewer mở27 ảnh (24after/reference/fingerprint/before130), 14/14 tĩnh ở sáng/tối font1.0/1.3/2.0, state3-selected/vua-phai/enabled; motion N/E | Name/new account/keyboard, disabled/loading/error/offline/retry, TalkBack/motion/lifecycle, native390/430/tablet/iOS; chưa toàn Gu/font/state matrix |
| C — Explore/destination | Native `6f3f5af2`: Cafe2, tìm tên1, query lạ0; save/unsave GET200 khớp, câu hỏi provider thật ra2; đổi Hà Nội0 nơi/selected persisted/restore Đà Lạt; gallery sáng/tối có fingerprint đúng JS | Ship P2-01 viewport đầu: 14/14 tĩnh ở sáng/tối font 1.0; tên/fact đầu và artwork rõ trước nav | Ảnh thiếu/credit, offline/retry/double tap; chữ lớn/toàn state/motion và review các state mới chưa đạt |
| D — Place/chọn kèo | Detail/save/cancel6f; JS41 picker chụp đúng hàngQC rồi add19,689s exit0, GET200 own1→2/firststop/check-ins giữ, pairhai stop nguyên | Root mở ảnh so board01-panel3 và picker, chưa review đủ | Nhãn kèo cụ thể candidate sau41; lỗi/empty/layers/font matrix |
| E — Plan/outing/map | Native d3 tạo kèo/check-in; APK2f20 sửa wrapper0×0, ba coldmap/pan/khớp80,472s exit0; JS41 ground→tapmốc selected16,635s exit0; cleanup chặng thêm nhầm GET200 hai records gốc nguyên | Root mở timeline/map/diagnostic; chưa review đủ đúng source cuối hoặc settled camera | Reorder/pin/cluster/map provider/permission/conflict/partial creation; fullgate/TalkBack |
| F — Create/check-in | Native d3 empty→form/blanksubmit/name/budget→create200.000đ/2người/stop/check-in1, GET200 khớp; JS41 thêm địa điểm giữcheck-in, hủy draft không ghi | Root mở11PNG d3 và sheet41; chưa chấm toàn bộ sheet/check-in | Sheet CTA trên IME đang sửa sau41; từng entry/context/audience/location/cancel/font/theme/error |
| G — Chat/story | MLS hai chiều và media nhận/relaunch hai emulator; poll create/vote/tie/switch/author-close/relaunch; Story thay ảnh/draft/đăng/pause/Back/Keep/DELETE thật; Ngọc bạn đọc/seen/nonauthor | Khung media `2fc1c73f` mở ảnh so board03; Story preview/viewer/question `c9a3f541` 14/16, ship chỉ hai P2 và state đã chụp | Header/history-empty copy; motion/retry/expiry/unauthorized audience/pendingDELETE/TalkBack; poll→tờ hẹn thật; reseal/reverse media |
| H — Groups/friends | Thành viên `3b71838e`: 2 active/0 invited, tên thật, không đổi vai trò | Root mở ảnh inline: không đẩy nhãn/CTA khỏi hàng; chưa chấm cả family | Invite/accept/reject/leave/last-admin, friend pending/block và empty/error |
| I — Hai người | Chưa nhận full native pair workflow | Chưa review native đủ | Hai phiên UI, tờ hẹn/accept/promote, chọn người và các nhánh |
| J — Moments/album/wall | Story không thay bằng chứng khoảnh khắc/album/tường | Chưa review native đủ | Media/crop/draft/retry/audience/view/album/delete, heart/comment đúng quyền |
| K — Ending/diary | Bằng chứng web lịch sử không tính native | Chưa review native đủ | Ending→reader→post, draft/error/layout/privacy và provider thật |
| L — Profile/people | Profile edit/keyboard/cancel; avatar picker/upload/cancel thật; crop native `2c1189d7` cancel/confirm và server640×640 | Root mở profile/editor/after; chưa rubric đủ own/other/pending/blocked | Avatar error/retry/permission, propagation giữa máy; people/achievements; chữ lớn và states |
| M — Bill | Focused native822 nhập/preview/ghi bill200.000đ, phần75.000/125.000; witness thật; runner toàn bảng vẫn đỏ | Có ảnh đã mở; chưa nhận composition/state đầy đủ | Native whole-flow/canary cùng harness, scan camera/OCR/provider, font/keyboard/errors |
| N — Settlement/finance | Focused native822 batch75.000đ, xác nhận nhận và ledger về0; receiver confirmation không là bank evidence | Chưa nghiệm thu toàn state/layer native | Bảng đầy đủ, role/wait/conflict, share handoff và visual |
| O — Guest ngoài Expo | Cổng riêng, không gán emulator Expo thành guest proof | Không mở browser theo yêu cầu hiện tại | Fresh guest/oracle parity; phương thức visual guest phải giữ đúng phạm vi được phép |
| P — Community | Native deeplink/feed; gallery `6f3f5af2` sáng 35.859s/tối 19.539s/font 2.0 13.933s, đều exit 0; chưa full actions | Ship chỉ P2-01 viewport đầu và P2-02 «Để sau», resolved; baseline sáng/tối font 1.0 đạt 14/14 tĩnh, motion N/E | Font 2.0 chỉ nhãn/control ở viewport đã cuộn; feed/topic/post/edit/comment/report/moderation/filter/cursor/lỗi và toàn lifecycle còn thiếu |
| Q — Keeps/notifications/search | Lượt 52: consent→nháp→lưu thật, own-list của Ngọc không chứa ID keep của Minh và Keeps empty cuộn/bấm; editor `eba8678a` bấm gọi/lưu trên IME; sáng 2.0 41.118s/tối 1.0 53.484s exit 0, sáng 1.0 exit 1 sau save vì selector sai | Ship chỉ P2-NEP-EDITOR-01 CTA trên IME, resolved; 9→12/14 tĩnh, artwork N/A, motion N/E | P3 font 2.0: caret autoscroll đưa nhãn khỏi viewport/crop dòng đầu; chưa kiểm mọi dòng/offset, mọi route/action/error/empty/sheet, AI quality/privacy |
| R — Settings/account/privacy | Theme thật Sáng/Tối; device list read thật; login Ngọc lại thật; avatar crop qua settings | Root mở settings light/dark/device/profile, chưa rubric toàn family | Logout phải thu hồi đúng device còn là finding mở; remote session/revoke/privacy/block/delete-account; không xoá mọi máy để ép xanh |
| S — Root/tab/alias | Hai native runtime có ownership/fingerprint; tab/Back/relaunch thực trong các flow đã nêu | Welcome transfer được review có giới hạn | Deep-link/alias/auth-expiry/foreground/reduced lifecycle; keyboard/nav trong toàn matrix |

Không cộng các route đã mở thành số state/action đã đạt. Provider one-shot thật đã nối sau52b6ba1d; không thay nhánh chưa kiểm bằng response giả. iOS, phần cứng thật và review crypto độc lập vẫn là
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
- Editor Nếp: [verdict eba8678a](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-eba8678a/nep-editor-independent-verdict-eba8678a.md)
  chốt **ship chỉ P2-NEP-EDITOR-01**, CTA gọi/lưu trên IME resolved ở Android
  320×640 sáng 1.0/2.0 và tối 1.0. Root/reviewer mở toàn bộ 17 ảnh packet,
  reviewer mở thêm ảnh lỗi cuối font 1.0. Lượt sáng 1.0 **69.962s, exit 1**
  đã lưu thật rồi timeout vì tìm title «Ghi chép riêng» thay vì «Điều mình
  muốn giữ»; không ghi runner này xanh. Sáng 2.0/tối 1.0 dùng đích
  `community-keeps`, đều exit 0. Điểm **9→12/14 tĩnh**, artwork/crop N/A,
  motion N/E; P3 autoscroll/crop font 2.0 còn mở. Font đặt trước fresh
  attach, chưa telemetry số RN. Diff `Sheet.tsx`/`PostDetail.tsx` từ eba tới
  `6f3f5af2` rỗng, không gán ảnh eba thành capture JS mới. Chưa toàn editor,
  Keeps, AI quality/privacy, error/retry/motion/TalkBack/font 1.3/tối 2.0.
- Gallery: [verdict 6f3f5af2](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-6f3f5af2/gallery-independent-verdict-6f3f5af2.md)
  chốt **ship chỉ P2-01 composition viewport đầu và P2-02 «Để sau»**,
  cả hai resolved; root/reviewer mở 10/10 PNG, fingerprint đúng JS.
  Community/Explore top, consent sau bài đầu và settings scheme snapshots
  sáng/tối font 1.0 **14/14 tĩnh**, interaction/motion N/E; /14 không thay
  ngưỡng /16. Font 2.0 chỉ nhãn/control tại viewport đã cuộn, không toàn màn
  chữ lớn. Consent dùng RudiButton inline, NBSP ở nhãn nhìn và khoảng trắng
  thường ở accessibilityLabel; source giữ handler/phạm vi consent. Không
  suy TalkBack/target/contrast đo native, lifecycle theme hoặc mọi action.
- Gu: [verdict d3c4a3aa](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-d3c4a3aa/gu-independent-verdict-d3c4a3aa.md)
  chốt **ship chỉ sửa nhãn giữa**, finding resolved: `styles.giua` thêm
  `alignSelf: "stretch"`, giữ chữ giữa/font/ngưỡng tiền/handler. Before130
  chỉ có «Trên»; after130 save-action sáng/tối có đủ «Trên 500K»/«Rộng tay».
  Reviewer tự mở27 PNG trong [packet](/home/lakiet/.local/share/rudi-nep-ui/runtime-private-merged-20261006/native-audit-d3c4a3aa/gu-review-packet.json):
  24after, reference, fingerprint đúng JS và before130. Android320×640,
  sáng/tối × font1.0/1.3/2.0; chỉ state3-selected/vua-phai/enabled,
  **14/14 tĩnh**, motion N/E, không quy thành /16. Sáu receipt after exit0:
  sáng1.0 17.988s, sáng1.3 18.840s, tối1.0 18.038s, tối1.3 19.040s,
  sáng2.0-visible 25.798s, tối2.0-visible 25.648s; `not_full_gate: true`.
  Hai lượt font2.0 đầu đỏ do count ngoài fold vẫn giữ; visible là capture
  cuộn bổ sung, không đổi runner đỏ thành xanh hay gán lỗi UI. Font đặt
  trước fresh attach, chưa telemetry số RN. Reviewer thấy mode0664 ban đầu;
  parent sửa quyền rồi reviewer kiểm lại27/27 mode0600, không gán hồi tố.
  Save/cancel/restore6f là bằng chứng chức năng riêng, chưa chạy lại ở d3;
  chưa name/new-account/disabled/loading/error/offline/TalkBack/motion,
  native390/430/tablet/iOS hoặc toàn Gu/frontend.
- Avatar crop `2c1189d7`: editor thật được mở; Back exit0/6,422s giữ nguyên version
  và hash ảnh640×800. Confirm exit0/13,531s đổi version, GET200 ảnh640×640.
  Root mở editor/cancel/after; không suy crop/permission/retry cho mọi media.
- Bạn đọcNgọc `c9a3f541`: accepted friend trong danh sách nativeMinh; viewer không
  có nút xoá, đóng đổi vòng sang đã xem,16,835s exit0; root mở hai ảnh. Avatar cắt
  hiện trên rail và HTTPNgọc GET200 cùng hash640×640; chưa chứng minh người lạ,
  block/revoke/expiry hoặc mọi audience.

## Cổng trước landing

E/F checkpoint9007 có6/6 flow matrix và6/6 receipt bổ sung exit0; root mở
60+24PNG. Reviewer fresh-context vẫn trả **fix**,9/14 ở bảy trục tĩnh,
motionN/E: hierarchy/fade, overview, form IME, giờ, picker và thu gọn.
Diagnostic thu gọn44,672s exit1 giữ nguyên. Hai lượt đỏ khác là giả định
harness sai selected/checked và cluster/card, không gọi lỗi sản phẩm.
Candidate sửa sáu finding mới qua typecheck và55/55 focused; chưa có
native after/verdict pass. Chi tiết và giới hạn ở RECOVERY-20261008.md,
evidence ngoài repo `native-audit-9007c3bb/finish-review/`.

Checkpoint5569633c sau đó có fingerprint đúng,6/6 matrix và6/6 bổ sung exit0;
root mở60+24PNG. Verdict sáu finding: F1/F3/F5/F6 resolved, F2/F4 partial,
disposition fix. Nhãn / người mất chữ và HH:mm bị cắt với IME; đầu trang gấp
bị cắt dù state đã giữ đúng. Candidate tiếp sửa đúng ba điểm và rút lại hai
file hướng dẫn stale, qua86/86 focused+guide và Go guide; chưa có native
after/verdict cho batch tiếp. Chi tiết receipts/giới hạn ở RECOVERY; không
gộp green thao tác thành green thị giác. Clean Node556 vẫn ghi1602/1605,
ba red được giữ nguyên, không nhận là full npm test hoặc strict gate.

Main chưa nhận batch này. Full mobile sạch gần nhất `2fc1c73f`:1620/1620,
0fail/skip/cancel/todo; không gán cho sửa sau đó. Runtime plugin/detector
không tìm thấy, `imp preflight` exit127; không ghi detector clean.
Craft-floor có và đã được đọc tại
`/home/lakiet/.claude/plugins/marketplaces/impeccable/plugin/skills/impeccable/reference/craft-floor.md`;
đính chính các ghi chú cũ nói reference này thiếu. Còn full strict gate32 ở cây sạch SHA cuối,
identity/canary/ít nhất hai mutant độc lập cùng harness, fresh guest parity,
state/native matrix và Impeccable finish phù hợp tập capture cuối. Không mở web
hoặc chạy benchmark/load để thay những phần thiếu.
