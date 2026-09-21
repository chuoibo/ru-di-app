# Trả lời review delta #584 / #585 — đóng F31 và F32

Ngày: 09/09/2026. Nhánh: `claude/p0-w-ui4-ghi-cong-va-gui-lai`.
Nền: `docs/archive/codex/2026-09-08/review-delta-584-585.md` (APPROVE hướng mỹ thuật, hai finding mở).

Review nói đúng cả hai lần, và cả hai lần nguyên nhân **sâu hơn** chỗ nó chỉ. Dưới đây là
cái gì đã đổi, đo bằng gì, và cái gì **chưa** đo được.

## F31 — ghi công: sửa bằng kiểu, không sửa bằng lời hứa

Probe của Codex chỉ ra hai đường thoát của máy dò. Chạy lại trên cây trước khi sửa: đúng,
cả hai trả 0 finding. Nhưng chúng chỉ là triệu chứng. Nguyên nhân là hợp đồng:

- `MediaSlot` nhận `source` và `attribution?` **rời nhau**, hai quyết định độc lập phải tự
  khớp bằng tay. `PlaceCompare` đã dùng đúng lỗ ấy: đưa `MediaSlot` một địa chỉ trần, câu
  ghi công nằm ở nhánh JSX khác.
- `AnhCoGhiCong.source` là thuộc tính công khai, nên `x.anh.source` hợp lệ ở **mọi** file.
  Không cách nào dạy một biểu thức chính quy hết mọi cách viết lại của nó.
- `khungAnh()` chính là bước nới kiểu bắt buộc thành `attribution?`.
- Ngoài ba khung, còn **ba renderer ảnh nữa** không có mặt ghi công nào: `ui.tsx Photo`,
  `AlbumAnh`, `PhotoViewer`. Máy dò không thấy chúng vì nó khoá theo tên thẻ.

**Đã sửa.** Ảnh catalogue giữ địa chỉ **trong closure** và chỉ trả ra qua `ve()`, thứ luôn
trả **cả** `source` **lẫn** `ghiCong`:

```ts
export type AnhCoGhiCong = { readonly ve: () => AnhDaMo };
export function anhDanhMuc(source: ImageSource, nguon: Attribution): AnhCoGhiCong;
```

Hệ quả trực tiếp: **hai mẫu thoát của Codex nay là lỗi biên dịch**, thứ tsc đọc trên mọi
file mọi lần build, không phải một biểu thức chính quy phải đoán trước cách viết. Và vì
địa chỉ không rút ra được, ảnh catalogue **không thể** tới ba renderer kia nữa.

Một quyết định thuần thay cho hai điều kiện phải tự khớp:

```ts
export function veKhung(nguon: NguonKhung, o: { hong: boolean }):
  { source: ImageSource | null; ghiCong: string | null; canhBao: string | null };
```

`MediaSlot`, `HangChang` và ba renderer của `HangDiaDiem` gọi nó một lần rồi render đúng ba
trường. Ảnh của nhóm đi nhánh `{loai:"nhom"}` và **không bịa giấy phép cho ảnh riêng tư**,
đúng lời review. `anhBiaThe` trả thẳng `AnhCoGhiCong | null`, gộp ba phép kiểm null mà
`ExploreLive` và `PlaceDetailLive` đang chép tay.

**Một lỗi cũ lộ ra khi làm.** Khung khoá trạng thái «chưa tải được» vào *object địa chỉ*,
thứ mọi màn live dựng lại mỗi lần render. Nên `useEffect` reset chạy mỗi khung hình và
trạng thái hỏng **không bao giờ hiện được** trên màn live. Nay khoá bằng `khoaNguon`, một
chuỗi ổn định. Không nằm trong F31; sửa vì cùng dòng code.

**Máy dò thành lớp hai**, cho thứ kiểu không thấy (`as any`, ai đó dựng lại cặp
`{source, nguon}`, hay một khung vẽ ảnh mà bỏ nửa quyết định):

| Trước | Sau |
|---|---|
| Chỉ quét `.tsx` | Quét cả `.ts` lẫn `.tsx` |
| Bỏ qua **nguyên ba file** khung | Không bỏ qua file nào |
| Kiểm chuỗi `cauGhiCong(` có trong file, đọc cả comment | Bất biến trên `veKhung` + ca đột biến |
| Sàn `daQuet > 40` trên 141 file | Sàn `daQuet >= 130` **và** sàn số nút đã duyệt |
| Tự kiểm 9 ca, không có hai mẫu thoát | Tự kiểm có **đúng hai mẫu của Codex** + ca «vẽ ảnh bỏ ghi công» |

Điều kiện đóng của review, chạy lại được:

```
node docs/archive/claude/2026-09-09/kiem-cong-ghi-cong.mjs
```

Probe gốc của Codex dừng ở «biDanh is not defined»: nó liệt kê tên khai báo theo bản cũ,
máy dò mới cần thêm bảng bí danh. Bản trên **cùng phương pháp** (đọc file test như văn bản,
trích khai báo, chạy trong context mới), thêm tên còn thiếu. Kết quả 9/9:

| ca | bắt được | mong đợi |
|---|---|---|
| direct · optional/alias · source-variable | có | có |
| **object-alias** (mẫu 1 của Codex) | **có** | có |
| **destructure** (mẫu 2 của Codex) | **có** | có |
| mở khoá `ve()` ngoài `ghi-cong.ts` | có | có |
| `veKhung` vẽ ảnh mà bỏ ghi công | có | có |
| `veKhung` đủ đôi · asset thường | không | không |

và hai sự thật cấu trúc mà Codex đo: `bo_qua_nguyen_file: false`,
`kiem_chuoi_cauGhiCong: false`.

`DESIGN.md` đã sửa: câu «ba khung ấy nhận `AnhCoGhiCong` nên không có cách đưa ảnh mà bỏ
ghi công» được thay bằng mức bảo đảm thật, và câu «`MediaSlot` là nơi duy nhất ảnh được
phép xuất hiện» được sửa bằng cách kể tên ba renderer kia.

## F32 — gửi sticker: nguyên nhân không phải thiếu ảnh chụp

Review viết «không chỉ thiếu capture; code thiếu trạng thái». Đúng, và còn hơn thế. Nguyên
nhân gốc là **cái chìa**:

`useTinNhan` gọi `newAttempt()` **bên trong** hành động, trong khi `api.ts:180` đã viết sẵn
câu ngược lại: «Mint an attempt. Call this on the press, never inside a retry.» Nên mỗi lần
gửi lại mint một `Idempotency-Key` mới, và ca nguy hiểm nhất mà review nêu — máy chủ đã
nhận, client mất phản hồi — **ghi hai tin**. Lỗi ấy có ở **cả ba** đường gửi, không riêng
sticker; gửi chữ chỉ giấu nó kỹ hơn vì câu chữ quay về ô soạn và người ta bấm lại.

**Đã sửa.** Hàng chờ giữ chính `Attempt` của lần bấm (`chat/hang-cho.ts`, thuần):

- **Thử lại** gửi lại **đúng chìa cũ**, nên máy chủ phát lại câu trả lời đã lưu
  (`app/api/idempotency.py`, `Replay`) và `gopTin` dedupe theo id máy chủ. Một lần thử lại
  **không thể** thành tin thứ hai.
- **Gửi lại cố ý** cùng một sticker là **chìa mới**, là tin thứ hai. Hai việc khác nhau,
  đúng yêu cầu «phân biệt một lần thử lại với một lần cố ý gửi thêm».
- Gửi chữ và gửi ảnh dùng lại chìa **chỉ khi từng byte giống hệt** (thân và tin trả lời):
  cùng chìa khác thân là `422 idempotency_key_reuse`, một lời từ chối nhắm vào người không
  làm gì sai.

**Trạng thái nằm trên đúng tin**, không phải một thông báo chung: hàng đang đi vẽ **chính
sticker ấy** mờ kèm «Đang gửi...»; hàng hỏng vẽ rõ nét kèm câu của máy chủ, nút **«Thử
lại»** viền và nút «Bỏ». Nhãn lấy đúng của nhà (`ErrorState`), **không** dùng «Gửi lại» vì
chữ ấy đã có nghĩa «gửi cho người này lần nữa» ở đợt thu.

Lỗi **vĩnh viễn không mời thử lại**: `sticker_unknown` (bản app không có hình),
`reply_target_deleted`, `permission_denied`, và ba mã idempotency —
`idempotency_request_in_flight` nói thẳng là đừng bấm nữa. Một nút sẽ hỏng y như cũ là câu
trả lời tệ hơn không có nút.

Hàng chờ **không** vào `chat.tin`: `cursorMoiNhat` poll từ đầu danh sách ấy, nên một cursor
bịa ở đầu sẽ đầu độc mọi lần poll.

Câu quá lời trong doc trước (`tra-loi-sticker-cho-ti.md`, mục «Trạng thái gửi / lỗi / gửi
lại») đã được đính chính tại chỗ: cờ `loading` mà nó viện dẫn là cờ của nút gửi **chữ**.

## Cổng đã chạy

| Cổng | Kết quả |
|---|---|
| `npx tsc --noEmit` | sạch |
| `npm test` (cây sạch, xoá `dist-test`) | 768 pass, 0 fail |
| `python3 -m pytest services/api/tests tests -q` | 3433 passed, 711 skipped, 5465 subtests |
| `python3 scripts/repo_guard.py staged` | pass, từng commit |
| detector Impeccable trên target đã đổi | `[]` |
| probe điều kiện đóng F31 | 9/9 |

## Cái này **chưa** chứng minh

- **Chưa đo hành vi gửi trên chat API thật.** Ba ca review đòi — mạng chậm, từ chối có phản
  hồi, mất phản hồi sau khi máy chủ đã nhận — cần stack `--otp` và các flow tiền đề. Ca thứ
  ba đo bằng gỡ đường hầm `adb reverse` giữa lúc gửi. Chưa dựng được trong lượt này.
- **Chưa chụp native lượt này**: máy ảo đang do lane khác lái. Bàn thử dev đã có mục dựng ba
  trạng thái hàng chờ **qua chính máy trạng thái ấy**, nên khi có máy thì chụp là đủ.
- Detector trả `[]` trên cả `src/rudi`, như mọi lượt trước. Nó **không** là bằng chứng mỹ
  thuật; Codex đã nói đúng điều đó.
- Ba renderer ảnh của nhóm nay **không nhận được** ảnh catalogue, nhưng chúng vẫn chưa có
  câu ghi công riêng cho ảnh mượn của bên thứ ba nếu sau này có. Ghi nhận, chưa cần.
