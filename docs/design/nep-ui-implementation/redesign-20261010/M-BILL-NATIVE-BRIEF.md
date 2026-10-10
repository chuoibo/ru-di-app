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
