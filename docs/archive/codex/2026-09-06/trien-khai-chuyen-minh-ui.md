# Triển khai chuyển mình UI Rủ Đi

## Phạm vi được leader giao

Ngày 06/09/2026, leader yêu cầu Codex thực thi kế hoạch chuyển mình UI bằng
Impeccable, gồm frontend và những phần backend cần cho tương tác thật.
Đây là phân công riêng cho chiến dịch này, không sửa quyền sở hữu mặc định,
không phải verdict APPROVE và không cho phép tự merge.

Baseline: `931d9c3`. Giữ nguyên các tài liệu/ảnh audit chưa được theo dõi có sẵn.
Không đổi luật tiền, payment rail, dữ liệu cá nhân hay vùng đóng băng.

## Hướng đã chốt

“Nhật ký chuyến đi sau giờ làm”: giấy ấm, bìa indigo, mực rõ; coral dành cho
hành động, tím trầm cho bản nháp AI, teal cho tiền. Nội dung và thao tác thật
là điểm nhấn; đường đi, washi và dấu chỉ xuất hiện khi có ý nghĩa.
Body 17/24, metadata 13/18; control sáng dùng viền trung tính đủ tương phản.
Grid đo phần nội dung còn lại sau rail; IME chỉ có một chủ sở hữu.

## Các lát triển khai và bằng chứng

- [ ] Nền tảng: token, type, grid hữu hạn, FAB, roster, Create sheet, IME fixture.
- [ ] Timeline: revision nguyên tử, conflict, giữ stop/check-in, kéo và đổi thứ tự.
- [ ] Auth: Google theo cấu hình, giữ OTP và preferences, không provider giả.
- [ ] Media: ảnh có giấy phép/provenance, importer bên ngoài Git, ảnh nhóm riêng tư.
- [ ] Khám phá, địa điểm, tạo kèo và lịch trình.
- [ ] Chat, vote, nhóm, bạn bè, lời mời.
- [ ] Review bill, gán món, kết quả và thu chi.
- [ ] Tường, album/viewer, chia sẻ, check-in thủ công, hồ sơ, tài chính, huy hiệu.
- [ ] Kiểm thử native từng lát; rà độc lập Impeccable và cập nhật DESIGN.

Mỗi lát ghi rõ test đã chạy/chưa chạy. Typecheck và test không thay thế ảnh
native. Android không chứng minh iOS. Không có ảnh thật được nhập khi chưa có
mapping địa điểm–ảnh đã duyệt và nguồn/license hợp lệ; không lấy ảnh gần tọa
độ rồi gọi đó là địa điểm. Google không cấu hình thì không hiện nút.

## Nhật ký

- Bắt đầu: cây làm việc không có thay đổi tracked; tài liệu audit untracked
  được giữ nguyên. Detector HTML/CSS không áp dụng cho native theo Impeccable.
- Checkpoint `c29c54f`: revision lịch trình được khóa và kiểm tra trong cùng
  transaction PostgreSQL; client cũ vẫn tăng revision, bản nháp cũ nhận 409.
  Reviewer độc lập chạy 43 ca PostgreSQL; sau góp ý giữ tham chiếu ORM mạnh,
  tác giả chạy lại 3 ca revision/race/migration thành công. Đây chỉ là review
  backend của 6 file, không phải phê duyệt UI hoặc toàn chiến dịch.
- Nền tảng UI đang ở cây làm việc: palette semantic sáng/tối, chữ 17/24,
  grid đo chiều rộng thật, roster checkbox chỉ mở một món, khay Tạo theo
  nhóm hiện hành; native drag timeline và viewer vuốt/phóng ảnh đã có code.
  Google chỉ hiện khi cấu hình native hợp lệ; chưa kiểm chứng OAuth thật.
- Kiểm thử mobile gần nhất: 667 test đạt và typecheck đạt. Lỗi kiểm tra AST
  về fallback ID trong biểu thức đóng/mở món đã được sửa bằng nhánh rõ nghĩa;
  không sửa scanner để ép qua. Kết quả này không chứng minh tương tác native.
- Android QA dùng riêng `rudi-qa3`, serial `emulator-5560`; không thao tác
  emulator của người khác. Dữ liệu fixture tổng hợp, Metro 8096. Fingerprint
  `codex-ui-wave1-931d9c3` chỉ baseline cộng thay đổi local, không phải commit
  đã phát hành. Ảnh ở `.impeccable/review/wave1/` đang được kiểm tra lại vì
  một số lần đổi cấu hình máy đã chụp nhầm launcher/màn chưa tải; các ảnh đó
  không được tính là bằng chứng. Chưa có bằng chứng iOS.
- Bổ sung `expo-system-ui` đúng dải phiên bản SDK 57 để cấu hình giao diện
  tự động được áp dụng trên Android; đang build lại và xác nhận sáng/tối.
- Kho ảnh Commons và importer đang triển khai riêng, chưa nhập ảnh địa điểm
  thật. Ảnh công khai dùng namespace storage riêng, không đọc kho ảnh nhóm.
- Foundation ảnh công khai: metadata tác giả/license/source bắt buộc, khóa
  opaque, ràng buộc PostgreSQL, chống nhập trùng cùng nguồn/địa điểm, mã hóa
  lại pixel bỏ EXIF. Hai route đọc metadata/bytes không dùng kho ảnh nhóm.
  Review sửa hai lỗi trước bàn giao: đặt nhầm ranh giới model GuestLink và
  annotation `list` bị phương thức cùng tên che khuất khi import module.
  Người kiểm tra chạy 129 test + 88 subtest và 63 regression PostgreSQL;
  main agent chạy độc lập 36 test + 72 subtest trong database dùng một lần,
  gồm migration/model parity, roundtrip và timeline revision. Tất cả đạt.
  Chưa chứng minh importer Commons hoặc gallery trên app live.
- Bản Android có `expo-system-ui` đã build/cài thành công; xác nhận trực tiếp
  sáng/tối qua ảnh phone assignment và Create, cùng tablet assignment chữ
  1,3×. 667 test mobile và typecheck chạy lại đạt. Một lần emulator bị crash
  khi đổi cấu hình; đã khởi động lại bằng renderer software để tiếp tục thu
  bằng chứng, không quy lỗi hạ tầng đó thành lỗi UI.
- Checkpoint `2fb215f` lưu foundation ảnh công khai; `5d853f4` lưu importer
  Commons theo mapping được người vận hành duyệt. Main chạy lại 56 ca importer
  đạt; HTTP/repository giả lập, không coi đó là bằng chứng Commons live.
- Review Impeccable ban đầu trả `fix`: viewer mất đồng bộ trang khi đổi rộng,
  offset vượt giới hạn khi pinch nhỏ lại, caption tổng bill 4,47:1 và tài liệu
  còn thang chữ/màu cũ. Đã sửa code trong một batch và thêm regression.
  Lần chạy cuối: 669 test mobile đạt, typecheck đạt; 13 test / 165 subtest
  token–contrast đạt. Native xác nhận viewer vẫn đúng ảnh 2/caption sau đổi
  rộng và vuốt về ảnh 1 được. Pinch đa điểm thật vẫn chưa kiểm chứng; test
  hàm clamp không được ghi là test gesture.
- `DESIGN.md` và sidecar đã được documenter đồng bộ với bản thực thi; giữ
  định dạng hiện tại, không đổi schema ngoài phạm vi. Detector HTML/CSS không
  chạy cho React Native. Reviewer đã kiểm delta lockfile là phụ thuộc hợp lệ;
  digest allowlist đổi riêng cho file đó, không nới rule.
- Bộ ảnh bàn giao: [wave1-native-evidence](wave1-native-evidence/README.md),
  gồm thông số và ranh giới bằng chứng từng ảnh. Profile Android user 11 tạo
  riêng để kiểm fixture sạch; không xóa session cũ ở user 0.
- Checkpoint UI `2490e67` đã lưu code, tài liệu và 6 PNG; không push/merge.
  Repo guard staged đạt 50 file; kiểm lại toàn tracked tree đạt 1.550 file.
  Hồi quy `services/api/tests/api` + `services/api/tests/domain` chạy sau
  checkpoint: **1.928 test / 4.657 subtest đạt**, không phải suite PostgreSQL.
  Reviewer đầu hoàn thành lượt phát hiện nhưng lượt chấm lại bị treo; main
  ngắt và gửi cùng danh sách sửa lỗi cho reviewer mới một lần theo fallback
  Impeccable. Không suy luận phê duyệt từ việc code đã commit.
- Reviewer fallback chấm cả bốn finding đã resolved; pinch chỉ resolved ở mức
  code/hàm thuần vì chưa có gesture đa điểm thật. Disposition `SHIP` chỉ áp
  cho bốn finding/checkpoint, không phải `APPROVE` toàn app hay production.
- Handoff chi tiết cho agent kế tiếp nằm tại
  [`HANDOFF-SESSION-CHUYEN-MINH-UI-RUDI.md`](HANDOFF-SESSION-CHUYEN-MINH-UI-RUDI.md).

## Phần còn lại sau checkpoint nền

Chưa tích hợp gallery Commons/cover vào catalogue live, chưa có mapping ảnh
thật đã duyệt; liên kết ảnh nhóm–địa điểm riêng còn phải triển khai. Chưa hoàn
thành các wave bố cục Khám phá, social/chat, vote/bill, hồ sơ/kỷ niệm. Timeline
kéo đã có, nhưng auto-scroll danh sách dài và ca native hai phiên conflict
chưa được chứng minh. OAuth thật, iOS/iPad, TalkBack/VoiceOver, pinch đa điểm,
hiệu năng trên phần cứng thật vẫn là các ô nghiệm thu mở.

Không tự merge hoặc push. Các checkpoint không phải tuyên bố app sẵn sàng ra
thị trường; chỉ lưu phần triển khai và bằng chứng đã thực sự có.
