# ADR-0035 — Nếp cài trong lề sổ: chỗ nghỉ duy nhất là mép giấy

**Trạng thái:** Chấp nhận · **Quyết định bởi:** leader (luật «không bao giờ che nội dung», 2026-09-23)
· **Hiện thực:** Claude, 2026-09-24
**Bổ sung:** ADR-0033 §2.4, §3, §7. Không sửa bản lịch sử của ADR-0033.

## 1. Bối cảnh

ADR-0033 dựng Nếp thành một dock bám mép phải. Bản đầu nghỉ ở dạng đĩa 57dp và đè chữ: cắt «20|0»
trên thẻ Khám phá, nuốt cú bấm «Đồng ý» của lời mời (flow 25). Leader chốt dock phải đẹp, có câu
chuyện, hài hoà với ngôn ngữ giấy đã có, và **không bao giờ che nội dung**.

Đo trên bản đang chạy: chữ trong app chạm tới lề trang, 16dp tính từ mép màn. Nên thứ gì nằm nghỉ
mà rộng hơn 16dp thì chắc chắn đè chữ ở một vị trí cuộn nào đó. Lề trang là một ngân sách đo được,
không phải lựa chọn thẩm mỹ.

Ba lượt reviewer độc lập (impeccable finish reviewer, 23–24/09) bắt thêm ba chỗ mà ADR-0033 để hở:

- Kéo Nếp ra là bước đầu của hai chạm để mở bảng. Nếu «đã kéo ra» được giữ làm chỗ nghỉ thì ai mở
  bảng một lần cũng mang một tờ 56dp đè lên mọi trang tới hết phiên. Phép đo canary thấy nó đè
  «200.000đ» (lấn 23px) và «22:30» (lấn 14px) trên Khám phá.
- Dòng hé bốn giây rộng 234dp, đè giá và giờ mở cửa của thẻ, và là một nút sống đặt trên chữ.
- Trên màn tiền, chạm vào mép vẫn đưa mặt Nếp ra cạnh con số.

## 2. Quyết định

1. **Chỗ nghỉ duy nhất là `an`: tờ giấy cài trong lề, lộ 10dp.** Có việc thì thêm một tờ thứ hai
   lộ 4dp, tổng 14dp, vẫn trong lề 16dp. Vùng chạm dừng đúng ở lề.
2. **Kéo ra (`nghi`) là một lối đi, không phải chỗ đứng.** Nó chỉ đến từ cú chạm của người dùng.
   Đóng bảng, rời màn, một tờ khác đóng lại, hay **6 giây không chạm lần hai** đều đưa Nếp về
   mép: một cú chạm lỡ vào mép 10dp, sát dải Back của hệ thống, không được để 56dp nằm trên chữ
   trong lúc người ta đọc tiếp. Khi bật trình đọc màn hình thì không tự cất theo giờ (người đó
   di tiêu điểm chậm hơn và không được bị giành), thay vào đó có thao tác «Cất Nếp vào mép».
   «Đã kéo ra» không được lưu xuống đĩa; mỗi lần mở app, Nếp bắt đầu ở trạng thái cài.
   *Thay ADR-0033 §2.4.* Câu «rời màn tiền thì Nếp trả về đúng lựa chọn của người dùng» vẫn đúng,
   vì lựa chọn nghỉ duy nhất còn lại là mép. «Ai đã vuốt Nếp đi thì vẫn đi» vẫn giữ nguyên.
3. **Nếp không bao giờ tự nở rộng.** Trạng thái `he` (dòng hé) bị bỏ. Việc chỉ được báo bằng tờ thứ
   hai, còn việc đó là gì thì nằm trong bảng. Không trạng thái nào rộng hơn 56dp mà đến được khi
   người dùng không chạm.
4. **Trên màn tiền, mép chỉ là cánh cửa, không phải một khuôn mặt.** Chạm hay kéo ở đó đều không
   đưa Nếp ra.
   *Bổ sung ADR-0033 §3.* Bề rộng mép là **10dp**, không phải 6dp như ADR-0033 ghi. 6dp nhỏ hơn
   góc gấp 8dp nên tờ không còn đọc ra là giấy. 10dp vẫn nằm trong lề.
5. **Nhường chỗ thì không vẽ gì.** Khi một tờ khác (khay, bottom sheet, story, bản đồ) nằm trên
   trang, dock không vẽ gì cả, kể cả mép. Câu chuyện đặt Nếp *trong* lề trang và tờ kia *trên*
   trang. Nên một mép còn vẽ đè lên góc khay, cạnh nút ✕, là lỗi xếp lớp chứ không phải chiều sâu.
6. **Tờ thứ hai là giấy ấm hơn một nấc, theo từng theme.** Ở theme sáng dùng `accentSoft`
   (#fff0ea). Ở theme tối dùng `line` (#363b5e), là mặt sáng hơn của họ giấy, chính là mặt mà nếp
   gấp đang lộ ra.
   *Bổ sung ADR-0033 §7.* `accentSoft` của theme tối (#3d1a10) nằm cạnh giấy xanh đậm #2e335c thì
   đọc ra một vệt gỉ sét. Vì đúng lý do đó mà `AlbumAnh.tsx` cũng đã loại màu này.

## 3. Hệ quả

- `luu-dock` lên v3, chỉ lưu vị trí trên ray. Bản v1 (`an`) và v2 (`ra`) không được đọc lại.
- Phiếu ngữ cảnh gắn với **focus của màn** (`useFocusEffect`), không gắn với pathname. Cách cũ là
  xoá phiếu khi pathname đổi, và cách đó thua một cuộc đua thứ tự: expo-router mount màn mới và
  chạy effect của nó *trước khi* provider thấy pathname mới. Kết quả là phiếu vừa khai bị xoá ngay,
  và Nếp không bao giờ biết người dùng đang đứng ở Khám phá (đo 24/09 trên bản web).
- Công cụ đo `apps/mobile/tools/xem-dock-nep.mjs` giữ các luật trên:
  - lúc nghỉ, trên tab đầu và trong hội thoại có tin thật, không che chữ nào;
  - khi khay mở, dock không vẽ gì;
  - mở rồi đóng bảng thì Nếp về mép; kéo ra rồi để yên thì 6 giây sau tự về mép;
  - trên màn tiền chỉ còn mép trơn, và chạm vào không ra mặt Nếp;
  - có việc thì không gì rộng hơn 56dp;
  - canary: kéo Nếp ra thì phép đo **phải** thấy chữ bị che.
- Chưa có ảnh chụp của story và bản đồ lúc nhường chỗ. Hai màn này gọi `useNhuongChoNep(true)`, và
  test reducer đã giữ luật nhường chỗ. Bằng chứng ở đây mới là bằng chứng trên mã.

## 4. Cái này KHÔNG cho phép

- Không cho Nếp nghỉ ở bất kỳ trạng thái nào rộng hơn lề trang.
- Không cho việc đến tự đưa Nếp ra hay tự viết chữ lên trang.
- Không cho khai `setSystemGestureExclusionRects` để giành mép màn: cú vuốt ngang vào trong ở mép
  là nút Back của người dùng. Nếp chỉ dùng chạm và kéo dọc. Đo 23/09: dải cử chỉ rộng 29.7dp và
  chỉ nuốt cú vuốt ngang vào trong; chạm trong dải vẫn tới được app.
