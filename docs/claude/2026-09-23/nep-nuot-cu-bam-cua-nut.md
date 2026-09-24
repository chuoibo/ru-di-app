# Nếp nuốt cú bấm của nút bên dưới: đo được, không phải suy đoán

**Ngày 23-09-2026.** Bàn giao cho người đang sửa Nếp. Tôi dừng ở đây để không
giẫm chân; dưới đây là bằng chứng và phép đo, chưa sửa mã Nếp.

## Triệu chứng

Bảng native `--otp` trên `main` chết từ flow 25. D không nhận được lời mời vào
nhóm «Hoi QA», nên chín flow sau (26–33) đỏ vì một lý do chẳng liên quan tới
thứ chúng đo, và harness bắt được ở flow 34: «D chưa ở nhóm Hoi QA active».

## Bằng chứng

Tái lập bằng mini-bảng có chụp ảnh **ngay trước** và **ngay sau** cú bấm:

```
Take screenshot 25-d-loi-moi-cho... COMPLETED
Take screenshot nep-truoc-khi-bam... COMPLETED
Tap on "Đồng ý"... COMPLETED
Take screenshot nep-ngay-sau-khi-bam... COMPLETED
Assert that "Khám phá" is visible... FAILED
```

Ảnh `nep-truoc-khi-bam.png` cho thấy **đĩa Nếp nằm đè lên nút «Đồng ý»**, che
mất nửa phải của nút — chữ «Đồng ý» bị cắt ngay trên hình.

## Phép đo

Máy: `wm density` **420** (scale 2.625), `wm size` **1080x2400** → 411×914 dp.

| | dp x | dp y |
|---|---|---|
| nút «Đồng ý» | 325 – 393 | 471 – 507 |
| đĩa Nếp (56dp) | 361 – 402 | 496 – 535 |
| **vùng chạm của dock** (kèm `hitSlop` trái 24, trên 12) | **337** – 402 | **484** – 535 |

Tâm nút là **(359, 489)**. Nó nằm ngoài đĩa, nhưng **nằm TRONG vùng chạm** sau
khi cộng `hitSlop`. Maestro bấm đúng tâm phần tử, và người dùng cũng bấm quanh
đó — nên cú bấm rơi vào Nếp chứ không vào nút.

## Vì sao đây là lỗi của sản phẩm, không phải của máy đo

Người dùng thật gặp y hệt: mở app bằng tài khoản mới, màn đầu tiên có lời mời,
bấm «Đồng ý» → Nếp mở ra thay vì nhận lời mời. Muốn nhận thì phải kéo Nếp đi
chỗ khác trước, mà không có gì nói cho họ biết điều đó.

Ba điều kiện cộng lại:

1. `DOCK_DAU.trangThai = "nghi"` — mặc định là **đĩa nổi**, không phải mép giấy;
2. từ `nghi`, **một** chạm mở thẳng bảng (`trang-thai.ts`, case `cham`);
3. `hitSlop={{top:12, bottom:12, left:24, right:12}}` kéo vùng chạm **vào trong
   nội dung** 24dp, đủ để trùm tâm một nút nằm sát mép phải.

Màn này không thuộc «Luật Nếp Đứng Xa Tiền» nên `luiLai` không đẩy Nếp về mép.

## Cái tôi ĐÃ làm, và cái tôi KHÔNG làm

**Không** sửa mã Nếp — để người đang sửa quyết.

Đã làm, và cố ý làm theo hướng **không giấu lỗi**:

- `EXPO_PUBLIC_QA_TAT_NEP=1` không mount tầng nổi (`qa-nep.ts`), theo khuôn
  `EXPO_PUBLIC_QA_TAT_KAV` đã có;
- `mobile_native.sh --tat-nep` là **cờ phải khai rõ**, KHÔNG tự bật. Ban đầu tôi
  cho `--otp` tự bật nó và điều đó sai: một thứ ship cho người dùng mà bảng
  không chạm tới thì không cổng nào canh nó. Cờ ấy để **chẩn đoán** — chạy bảng
  có/không Nếp để tách lỗi Nếp khỏi lỗi khác — chứ không phải để bảng xanh.

Chạy `--tat-nep` một lượt cho thấy phần còn lại của app ổn: **118 ảnh, 3 flow
đỏ**, và cả ba đều có nguyên nhân riêng đã biết, không liên quan Nếp.
