# M — chia bill, lượt thiết kế native 11/10/2026

**Candidate chưa nghiệm thu.** Giữ UI3/seed `c8e88116`; code-led trong thế giới
hiện hành. Không có comp M mới được duyệt. Mockup/crop lịch sử chỉ để phản biện.
Chỉ chạy app trong Android emulator thuộc phiên; không mở bản web.

## Hợp đồng trước code

THESIS: Receipt và dòng sổ dẫn việc; minh hoạ ở vùng phụ.

OWN-WORLD: Giữ UI3/seed `c8e88116`, giấy/mực/coral, Bricolage đúng vai, Nếp canonical, teal tiền; Operate.

STORY: Nhập/resume → soát món → chọn người → đọc phần → ghi tường minh. Giữ draft nhóm/kèo, line-id, Back/keep, payer, occasion và server allocations.

FIRST VIEWPORT: Cửa nhập rõ; review quét món hợp lệ; tasklist trước bàn; kết quả/đã ghi dùng toàn cột.

FORM: Target48dp; font100/200/wide đổi cấu trúc, không thu chữ/cắt tiền. Opt-in IME, thấy ô focus và phản hồi.

FINISH: Capture → fresh finish-reviewer → disposition → documenter.

Contract 96 từ đã đóng băng private lúc 21:20:30 UTC ngày 10/10, trước lần
viết UI candidate riêng đầu tiên 21:24:53 UTC. Tài liệu này lưu lại contract
đó trước khi áp dụng candidate vào feature. Không nhận bằng chứng HTML/DOM
cho màn React Native.

## Baseline đã quan sát tại 061d8d7a

M đã chạy native sáng100 với dữ liệu tổng hợp: nhập hai món, Back/resume/giữ,
gán người, kết quả, payer/occasion với IME, ghi một khoản chi. Receipt khi resume
bị ép cạnh Nếp; review lặp toàn bộ editor; tên người bị cắt trong standee;
kết quả và trang đã ghi chưa đủ gọn. Một bill200.000đ tạo một expense v1,
hai allocation125.000/75.000đ, rồi một batch/obligation/receipt75.000đ.
SQL đọc-only và GET hai actor đối chiếu riêng; không dùng PNG làm bằng chứng
sổ cái, không gọi xác nhận người thu là bằng chứng ngân hàng hay completed.

Baseline N đã lưu cũng phát hiện tem “ĐÃ VỀ” và CTA “Xem quyết toán” mất đuôi
khi đọc lại ở sáng100. Chưa xác định cause. Candidate M phải kiểm nhãn/tem
ở initial/reentry; không suy rằng đổi bề ngang đã tự sửa shared primitive.

## Phạm vi triển khai và kiểm tra

- Receipt resume dùng trọn cột; hai cửa ảnh/nhập tay rõ trước Nếp hỗ trợ.
- Review thu dòng hợp lệ, mở dòng mới/trống/lỗi/cần kiểm. Không gấp editor
  trong lúc gõ; expansion review và assignment độc lập.
- Assignment ưu tiên danh sách món/người với tên đầy đủ, trạng thái checkbox
  và vùng chạm48dp; bàn/ghế/drag có cửa mở phụ và thao tác chạm tương đương.
- Result gom phần chia vào một receipt, phân vai phần bạn/tên/tiền/trạng thái;
  chữ lớn đổi cấu trúc. Payer/occasion và lỗi phải đọc được khi IME mở.
- Trang đã ghi dùng dòng tự cao, giữ tên/tiền/xác nhận; Nếp có vùng phụ riêng.

Không đổi allocator, integer đồng, draft scope, line-id, attempt/idempotency,
API/ledger payload hoặc quyền ghi. Shared default ngoài scope. Bản private
đầu chỉ đổi `ChiaBillLive.tsx`; so byte10 block nghiệp vụ là kiểm nguồn,
chưa chứng minh tương tác hoặc số request.

Cổng dự kiến: typecheck/checks tại SHA sạch; actual-TSX interaction harness
với identity/canary/hai mutant không tương đương; detector một lượt đúng
changed target; ảnh native light/dark, chữ100/130/200 và cửa sổ rộng, trạng
thái nhập/resume/review/lỗi/assignment/result/recorded. Ma trận này là kế hoạch,
chưa là coverage. Không ghi thêm expense chỉ để chụp lại theme. Kiểm và ghi
rõ cell chưa đạt; mở ảnh thật rồi fresh-context finish review. Mục tiêu ≥8/10
cho nhóm M được chụp, không critical/major còn mở trong cell đã kiểm; không
quy đổi thành điểm toàn app hay thay TalkBack/motion/hardware/iOS.

GPL/J có verdict riêng; M/N và toàn bộ app vẫn còn cổng. Không merge/push
main hoặc coi cổng kỹ thuật xanh là nghiệm thu vẻ đẹp.

## Lượt sửa sau ma trận native a21309eb

Đã mở đủ 22 ảnh gốc của màn đã ghi qua bảy profile và lần khôi phục sáng100,
cùng PID app31543. Chữ200 vẫn ép tên thành cột hẹp cạnh tiền; wide giãn cả
receipt và nút720dp; khôi phục100 còn giữ khoảng trống của cỡ lớn. Review và
result cũng có chữ/tem bị cắt. XML có đủ câu không được tính là paint đạt.
Hai probe Dimensions đều dừng mà không có metric; chưa kết luận nguyên nhân.

Đường native a213 đã bấm đúng một lần Next, một lần xem kết quả và một lần
ghi: một khoản chi mới200.000đ, hai phần125.000/75.000đ. Sau ma trận đã ghi,
24 GET giữ nguyên body, SQL giữ nguyên mọi projection ngoài snapshot_at;
đợt thu cũ không đổi. Đây không phải đếm HTTP request hoặc bằng chứng ngân hàng.
Evidence private: `m-a213-native-path-complete-correction-checkpoint.json`,
`m-recorded-hot-root-original-inspection.json` và
`m-recorded-hot-no-ledger-write-proof.json` trong `native-audit-a21309eb`.

Batch sửa dùng cột form cho trang đọc và chữ lớn; chỉ assignment/result ở
wide chữ thường giữ hai cột. Đầu receipt opt-in chiều rộng đầy đủ; tem ghi
opt-in wrap trong vùng có giới hạn; tên khoản chi được ngắt dòng tự nhiên.
Hai primitive shared giữ default hiện hành.

Local Expo module Android đọc resources fontScale hiện tại, phát notification
để hook M đọc lại; subscribe trước fresh read, dedup và chặn callback muộn.
M truyền cùng snapshot vào geometry ô nhập. Module chưa có hoặc lỗi dùng
RN fallback hiện hành; fallback không được nhận credit hot support. Không
đổi route key, draft, line-id, parser, allocator, Attempt hoặc API payload.

Trước commit: typecheck và guide freshness đạt;10 file kiểm bill/receipt/field/
adaptive đạt77/77, guard observer mới đạt27/27, không fail/skip. Đây là kiểm
trong feature tree, chưa thay clean exact-SHA check. APK mới, observer available,
hot/Settings return, giữ input/assignment/payer/occasion, IME/focus, paint và
fresh finish review vẫn pending. Chưa nghiệm thu M hoặc toàn bộ frontend.


## Vòng native 2528 — yêu cầu sửa sau ảnh thật

Review độc lập ở `2528a901` yêu cầu **fix**: chữ 130% cắt tiến độ, câu nguồn
và nhãn thêm món; 200% cắt cả tiêu đề, tên món, số tiền và CTA. Tám ảnh gốc
của vòng 100→130→200→100 đã được mở; quay về 100% phục hồi bố cục. Đổi cỡ
chữ tối đa bằng thanh Font size trong Android Settings rồi quay lại cùng PID
cũng tái hiện lỗi. Vòng này dừng trước Next, không tạo bill hay ghi sổ mới.
Đối chiếu mới: 24 GET và projection SQL chỉ đọc giữ nguyên lịch sử trước đó.
Các ô tối, rộng, focus/IME và downstream của nguồn này chưa nhận credit.

Chẩn đoán hữu hạn: giữ chữ 200%, đổi chiều rộng 320→321 làm chữ nhìn thấy được
vẽ đầy đủ và xuống dòng tự nhiên trong cùng PID, không mở lại route. Kết quả
này hỗ trợ thử đồng bộ lại layout, chưa xác định nguyên nhân cache/callback.

Bản sửa kế tiếp yêu cầu Android đo lại content root hiện có sau thay đổi font
hợp lệ: gộp công việc trên main Handler, lấy Activity hiện hành khi chạy, bỏ
Activity đã hủy và hủy Runnable khi module đóng. Chỉ yêu cầu layout trên
content, con và cháu trực tiếp để phủ root trực tiếp và một lớp bọc Expo hiện
hành; không thay cache, không reflection, không remount hoặc đổi mặc định chữ.
Getter vẫn chỉ đọc. Phạm vi nguồn là Fabric/Expo hiện tại, chưa xác nhận legacy
hoặc lớp bọc sâu hơn. Nhãn thêm món được căn giữa và có padding cân đối.

Đây còn là ứng viên: phải compile APK, kiểm 100→130→200→100 và Settings return
mà không đổi chiều rộng cứu layout, mở đủ ảnh gốc, kiểm tổng sau scroll cùng
raw input/focus/IME, rồi hoàn tất các ô còn thiếu và review lại. Detector nguồn
không chứng minh painted text; chưa đạt ≥8/10 và chưa có ship toàn M/toàn app.
