# Trả lời audit native 09/09 — F41–F46 và cổng motion

Ngày: 10/09/2026. Nền: `docs/archive/codex/2026-09-09/audit-native-toan-app.md` trên `main 92d0f241` —
**REQUEST_CHANGES cho nghiệm thu native, APPROVE hướng mỹ thuật**. Thứ tự làm theo §8 của audit:
F42 + F41 → F43 + F44 → F45 → cổng motion → F46. Mỗi mục dưới đây nói **đã đo gì** và **chưa đo gì**;
không có mục nào được đóng bằng «test xanh» thay cho phép đo mà audit đòi.

Đọc audit xong tôi kiểm từng finding trên code trước khi sửa. Cả sáu đều đúng; hai cái **rộng hơn**
audit ghi (F43 có thêm chín chỗ in `BASE_URL`, F42 có thêm ba đường async cùng lớp lỗi). Không có gì
phải đẩy lùi.

---

## PR 1 — F41 · F42 (`claude/p0-w-ui5-chat-phan-hoi-muon`)

### F42 — phản hồi muộn vào nhầm nhóm (P1)

**Nguyên nhân gốc, sâu hơn probe.** `chay` ghép `daGui` vào `tinRef.current` không hỏi phản hồi ấy
thuộc nhóm nào, rồi gọi `napMoi` **đóng trên A** → đọc A bằng cursor của B (`reads [A, B, A]` trong
probe của Codex). Cùng lớp lỗi ở `napDau` (trang đầu A về muộn **đè** trang B), `napMoi`, `napCuHon`,
`xoaTinCuaToi`, `doiPhanUng`, và **lỗi muộn**: `catch` ném lên màn nên thông báo lỗi của A hiện ở B.
Effect reset ở `:105` chỉ xoá hàng chờ, không xoá `tin`, không chặn completion. Route mount không có
`key`, nên state **của màn** (`thongBao`, `dangGuiThan`, chữ đang soạn, tin đang trả lời) cũng sống qua
lần đổi nhóm.

**Sửa — hai lớp, vì hai loại state.**
- Route `app/groups/[id]/chat.tsx`: `key={id}` → đổi nhóm là remount, state màn về không.
- Hook `useTinNhan.ts`: **thế hệ** `theHeRef`. Mọi đường async chụp số trước `await` đầu, so lại sau
  mỗi `await` ở cả `try` lẫn `catch`; lệch → bỏ trọn (không ghép, không đặt lỗi, không poll, không
  ném). Số chỉ tăng ở **một** chỗ: cleanup của effect `[contextId, personId]` — React chạy nó trước
  lượt đọc đầu của cuộc hội thoại kế và khi unmount. `chay` trả `null` = «không phải của mình»; bốn
  call site trong màn không cuộn/không báo khi `null`. Read-mark có thêm `trang.tin === tinRef.current`:
  ở commit đổi nhóm effect còn thấy danh sách **cũ** dưới id **mới**.
- Remount **không đủ một mình**: `void napMoi()` sau một cú gửi muộn vẫn kéo thêm một GET cho A sau
  khi màn đã unmount — đó là việc của thế hệ. Đây là lý do có cả hai lớp, không phải cho chắc.

**Cổng.** `tests/rudi-chat-useTinNhan.test.mjs`: **hook thật dưới React thật** (`react-test-renderer`
ghim `19.2.3` = phiên bản `react`; devDependency mới, có cảnh báo deprecated được lọc **đúng một dòng**
có tên). Transport là `globalThis.fetch` trả tay theo URL — export ESM của `tin-song` bất biến nên không
stub được `docTrangTin`; `expo-router` stub qua `imports` của `package.json` (`#expo-router`), stub
`useFocusEffect` **no-op** mặc định vì RNW không DOM trả `undefined` cho `AppState.addEventListener`. Bảy
ca: (1) gửi A muộn → B không có tin A, GET = `[A, B]`; (2) trang đầu A muộn không đè B; (3) lỗi A muộn →
B không lỗi, không hàng chờ, không ném; (4) unmount → không GET thêm; (5) đổi người; (6) **F32 giữ
nguyên** — retry trong cùng cuộc hội thoại mang cùng `Idempotency-Key`; (7) đổi nhóm khi focus không PUT
read-mark của B bằng id của A.

**Bằng chứng.**
- Trước sửa: ca 1–5 **đỏ đúng lý do** (`a-muon` lọt vào B; `a-cu` đè `b-cu`; lỗi «Máy chủ đang hỏng.»
  ném lên; `3 !== 2` lượt gọi sau unmount; đổi người bị đè). Ca 6 xanh sẵn — nó là hợp đồng F32 phải giữ.
- Sau sửa: 7/7 xanh; `npm test` **783/783, exit 0**; `tsc --noEmit` sạch.
- Đột biến lên cổng: bỏ điều kiện `trang.tin !== tinRef.current` → ca 7 đỏ «read-mark của B mang id a-cu
  của nhóm khác». Điều kiện ấy có tải, không trang trí.
- Probe của Codex `probe-late-context.mjs` chạy trên cây đã sửa: **exit 1** — «The historical defect did
  not reproduce». Tôi không sửa file của họ; kết quả ấy chính là điều họ viết sẵn cho lúc này.
- Lockfile: `npm ci` bằng node 20 (đúng CI) trong thư mục nháp từ `package.json` + `package-lock.json`
  mới → 666 gói, exit 0.

**Chưa đo.** Tái hiện **native** (audit: «rồi tái hiện native synthetic»). Cần stack live có hai nhóm và một
yêu cầu **treo rồi hoàn tất muộn**; gỡ `adb reverse` giữa lúc gửi làm fetch treo (WSL2 nuốt SYN) nhưng lúc
hoàn tất không điều khiển được, và build fixture không đi qua hook này. Tôi để lại flow mẫu trong kế hoạch,
không bịa một lần đo. Chi phí chấp nhận (ghi trong header hook): hàng chờ của A hỏng **sau** khi đã sang
B mất nút thử lại; quay lại A thì trang đầu đọc lại.

### F41 — «Thử lại» tràn khỏi mép trái (P1)

**Nguyên nhân gốc.** `RudiButton` mặc định `full` (`width:"100%"`, `flexShrink:0`); hai nút cùng hàng
trong `khoi` (`maxWidth 82%`, canh phải) là hai lần trọn bề rộng → «Thử lại» bị đẩy khỏi mép trái.
Toàn vỏ chỉ có **đúng cặp này** không truyền `full=` (grep).

**Sửa.** Hai nút `full={false}` (theo nội dung, nhãn không bao giờ bị `numberOfLines={1}` cắt) + hàng
`flexWrap: "wrap"`, canh phải → chữ lớn thì xuống dòng thay vì thu nhãn hay giấu nút. Lỗi vĩnh viễn vẫn
chỉ «Bỏ»; retry vẫn cùng `Attempt`.

**Cổng phải đo bounds, không đo «có trong cây».** Ảnh 16 của Codex có node «Thử lại» trong XML nhưng vẽ
ngoài màn — `assertVisible` xanh. Hai điều `uiautomator dump` làm: **kẹp** bounds về mép màn (nút tràn
hiện `[0,1209]→[195,1335]`, trông như trong khung) và **bỏ node** chữ đã rơi hẳn khỏi màn. Nên
`native-r11/kiem-bounds.mjs` đòi: mỗi nút có **node chữ con** cùng chuỗi, nằm trong nút với lề ≥ 30px hai
bên, nút nằm trong màn, hai nút không chồng. Tự kiểm trên **chính XML 16** của Codex: **ĐỎ** đúng lý do
(«KHÔNG có node chữ trong nút»). Bản đầu của script chỉ so `x ≥ 0` và **xanh trên XML 16** — cổng mù, đã
bỏ.

**Bằng chứng native.** Bảng `.maestro-bs-r11` **XANH** (flow 00 + 76, NEO 2b cắn, canary đỏ đúng
thiết kế). Vòng đo `native-r11/chup-va-do.sh`: **sáu cấu hình 1.0/1.3/2.0 × sáng/tối đều XANH** theo
bounds — «Thử lại» rộng 194/234/293px với node chữ con lề 39–40px hai bên, «Bỏ» cạnh phải, không chồng;
ảnh + XML + bounds.txt ở `native-r11/anh/`. Ở chữ lớn hai nút vẫn đủ chỗ trên một hàng (tổng < 82% bề
rộng), wrap là lưới an toàn chưa cần dùng tới. **Chưa đo**: hàng lỗi trong chat live thật (cần stack
`--otp` và một cú gửi hỏng thật); bàn thử vẽ **đúng component** `HangChoGui` của màn, không phải bản chép.

**Vòng review context mới (`impeccable-finish-reviewer`) → `fix`.** Reviewer đo từ bounds: «Bỏ» theo nội
dung còn **124px = 47,2dp** ngang — dưới sàn 48dp của nút compact, hệ quả trực tiếp của `full={false}`
(trước đó nút chiếm trọn bề rộng nên không ai thấy kit thiếu sàn ngang). Sửa ở **kit**: `RudiButton`
có `minWidth: 48` cạnh `minHeight` (`ui.tsx`), không vá riêng hàng này; cổng bounds thêm điều kiện mỗi
nút ≥ 48dp hai chiều, và bounds **cũ** của «Bỏ» đỏ với sàn mới (124 < 126px) — cổng bắt được lớp lỗi
reviewer tìm ra. Chụp lại sáu cấu hình trên bundle mới: bảng r11 lần 2 XANH (dấu vân mới), sáu cấu hình
lại XANH — «Bỏ» **126px = 48dp** ở 1.0 (140px ở 1.3, 162px ở 2.0), «Thử lại» 194/234/293px, nhãn lề
39–40px, không chồng; ảnh/XML/bounds trong `native-r11/anh/` là của lượt chụp lại này.

Reviewer cũng chỉ ra packet của tôi mô tả sai phạm vi: diff `GroupChatLive.tsx` còn các hunk F42
(narrow `null` ở bốn call site) mà tôi nói «không đổi gì khác». Đúng. Các hunk ấy là **hành vi**, không
phải hình: được gác bằng bảy ca React ở trên và lượt soát thiết kế bảy câu hỏi (thứ tự effect, cửa sổ
poll, `personId`, `key` với `Redirect`, kiểu trả về, harness, race sót) chạy trong context mới trước khi
code — không có ảnh nào chấm được chúng, nên reviewer hình không chấm là đúng.

---

## PR 2 — F43 · F44 (`claude/p0-w-ui5-loi-noi-voi-nguoi`, xếp trên PR 1)

### F43 — câu lỗi nói với lập trình viên (P1)

**Rộng hơn audit ghi.** Ngoài `thongDiepNguoiDoc(0)` và nhánh 404, còn **năm** chỗ dựng thẳng
`ApiError(0, "unreachable", \`Không nối được ${BASE_URL}…\`)` trong `api.ts`, **bốn** câu dự phòng in
`BASE_URL` ở `Bill.tsx`/`Profile.tsx`, và **sáu** chuỗi legacy («Không gọi được ${BASE_URL}», «Địa chỉ
đã thử: ${url}», «Không nối được ${state.url}…»). Quét nguồn tìm ra tất cả; audit thấy hai.

**Sửa.** Một hằng `LOI_KHONG_NOI_DUOC` («Không nối được máy chủ. Kiểm tra mạng rồi thử lại.» — câu nhà
đã dùng ở `dia-diem.ts`), dùng ở mọi chỗ mất kết nối; **không hứa «chưa ghi gì»** vì đây là ca duy nhất
client không biết máy chủ đã ghi hay chưa (đúng lời audit). 404: «Bản app này và máy chủ chưa khớp nhau
nên chưa mở được phần này. Cập nhật app rồi thử lại.» (bản đầu còn «báo cho nhóm kỹ thuật» — reviewer chỉ ra app không có kênh ấy, đã bỏ) Câu Việt của
máy chủ (`invite_not_found`…) vẫn đi qua nguyên — không thay lời từ chối nghiệp vụ bằng câu chung. Địa chỉ
đi kênh dev: một `console.warn` gác `__DEV__` ở `call`, tiếng Anh, không template. Mười chuỗi màn/legacy
bỏ địa chỉ, giữ động từ.

**Cổng.** `trang-thai.test.mjs` thêm hai ca: (1) status 0 và 404 không chứa `http`, `localhost`, IP,
cổng, «đang chạy», «API», «địa chỉ», «cuối màn hình»; status 0 phải nói «mạng» và không nói «chưa ghi»;
(2) **quét nguồn** `src/**` (sàn ≥ 180 file, đo 203): không template literal nào trộn chữ Việt với
`${BASE_URL}`/`${state.url}`/`${url}`. Bản đầu của (2) quét mọi `*url` và **vu oan** `dot-thu.ts` — tin
chia sẻ của đợt thu mang **link khách** (`envelope.url`), link là nội dung, không phải rò — nên luật thu
về đúng ba biến giữ địa chỉ máy chủ. Trước sửa: (1) đỏ đúng câu cũ, (2) liệt kê 17 chỗ; sau: xanh.

**Bằng chứng native.** flow 77 (`rudi://destinations` không có máy chủ — đúng cách audit tạo ảnh 22): bảng
`.maestro-bs-r12` XANH ở 1.0 sáng, vòng đo thêm 1.0 tối và 2.0 sáng — câu mới hiện, `assertNotVisible
"http.*"` và `kiem-placeholder.mjs` (từ chối mọi node chữ mang URL) đều xanh. **Chưa đo trên máy**: 404 và
5xx (cần stack trả mã ấy; câu gác bằng unit test); «Retry thể hiện đang xử lý» — `DiemDenScreen` về skeleton
rồi kết quả, chưa chụp giữa chừng.

### F44 — placeholder ô tìm cắt ở 2.0 (P2)

**Nguyên nhân gốc.** Android dàn hint native theo bề rộng view và cho xuống dòng **ngay cả ở ô một dòng**,
rồi cắt dòng hai ở đáy ô; RN không lộ `ellipsize` cho `TextInput`, `numberOfLines` mặc định đã là 1 nên
không phải cách sửa; câu 30 ký tự của Khám phá live trong ô ~230dp cạnh hai nút hỏng từ 1.3.

**Sửa.** Lõi `Field` tách ra `ui/Field.tsx` (không import native → node test render được; `ui.tsx` giữ
`Field`/`SearchField` làm vỏ mỏng gắn `Ionicons`, consumer không đổi). Ô một dòng vẽ placeholder bằng
`Text numberOfLines={1}` phủ đúng ô input khi rỗng và **không** truyền `placeholder` xuống native; ô nhiều
dòng giữ hint native. Tên trợ năng vẫn trọn câu (phương án B của audit: nhãn ngắn hơn về hình, mô tả trợ
năng đầy đủ); lớp phủ ẩn khỏi screen reader. **Hàng tìm thích ứng** ở `chuLon(fontScale)`: ô chiếm cả
hàng, nút xuống dòng canh phải (Khám phá fixture + live). Không tắt font scaling.

**Cổng.** `o-tim-placeholder.test.mjs` (RNW): input **không** có attribute `placeholder`, câu là node chữ
riêng, `aria-label` trọn câu, có chữ đã gõ thì node biến mất — đỏ trước sửa ở «`<input placeholder=…`».
Nó **không** chứng minh xuống dòng; cái đó là `native-r12/kiem-placeholder.mjs` trên hierarchy: node
placeholder phải cao ≤ 1,5 dòng ở cỡ chữ đang đo (hint native không là node chữ nên không đo được — tự
kiểm trên XML 20 của Codex ra ĐỎ vì «không có node»).

**Bằng chứng native.** bảng r12 XANH; vòng đo **14/14** (flow 78 bàn thử và 79 Khám phá × 1.0/1.3/2.0 ×
sáng/tối, flow 77 hai lượt): mọi node placeholder cao **một dòng** (63/70/95px ≤ trần 95/123/189), câu 30
ký tự ở 2.0 cắt «…» trên một dòng, glyph không cắt chân; hàng Khám phá đổi từ 544px (cùng hàng hai nút) ở
1.0 sang 838px (ô chiếm cả hàng, nút xuống dòng) từ 1.3; chữ đã gõ và placeholder cùng mép trái (ảnh bàn
thử, ô 1–2 so ô 3). Ảnh + XML + số đo ở `native-r12/anh/`. **Chưa đo**: Khám phá **live** (cần stack
`--otp`; hàng thích ứng cùng mã với fixture), màn hẹp hơn 411dp, iOS, và **caret trong ô đang focus** (không
ảnh nào chụp lúc focus — reviewer ghi là chưa xác minh, không phải hỏng).

**Vòng review context mới (`impeccable-finish-reviewer`) → `ship`**, mở đủ 15 ảnh, đo tương phản placeholder
từ pixel: sáng ≈ 5,1:1, tối ≈ 5,6:1 (≥ 4,5). Không có material fix; ba trong bốn gợi ý không chặn đã làm
ngay vì rẻ và đúng: default placeholder của kit «Tìm quán, món...» → «…» (ba chấm ASCII cạnh «…» cắt của kit
sẽ đọc thành «món... …»; bốn flow `tapOn` chuỗi ấy cập nhật theo); câu 404 bỏ «báo cho nhóm kỹ thuật» vì app
không có kênh ấy; trần một dòng của `kiem-placeholder.mjs` đổi từ giả định tuyến tính (biên 1px ở 2.0) sang
số đo thật `max(24dp, 17sp × cỡ) × 1,35`. Gợi ý còn lại — mồ côi «lại.» ở 1.0 — giữ câu nhà đang dùng ở ba
nơi; reviewer cũng ghi không vi phạm.

**Cổng tương phản biên control của backend** (`services/api/tests/web/test_contrast_floor.py`) đọc kit dạng
chữ và lấy **khối đầu tiên** khớp mốc «exported function Field» để đo `borderColor` ≥ 3:1. Tách lõi làm nó
rơi vào vỏ trong `ui.tsx` (không có viền) → 3 ca đỏ ở pytest gốc. Sửa ở phía mình, không đụng test của
Codex: vỏ là `export const Field = …`, gate rơi đúng lõi `ui/Field.tsx` nơi `lineStrong` được khai — comment
trên vỏ nói rõ và **không** lặp chuỗi mốc (bản đầu của comment lặp và lại đỏ). 9/9 ca contrast xanh,
pytest gốc chạy lại trọn bộ.

## PR 3 — F45 · F46 (`claude/p0-w-ui5-nghia-hinh`, xếp trên PR 2)

### F45 — nghĩa hình chưa tự đứng (P2)

Làm đúng lời audit: **giữ khung Nếp, sửa hành động/quan hệ, không thêm chi tiết li ti**, không vẽ thêm
cảnh. Cách chọn như lượt mày Nếp: dựng ứng viên bằng chính primitives đã biên dịch, render cạnh bản cũ,
**nhìn** rồi chọn — không suy từ toạ độ. Bảng ứng viên và bảng trước/sau ở `docs/archive/claude/2026-09-10/nghia-hinh/`.

| Hình | Trước (audit đọc ra) | Ứng viên đã thử | Chọn, và vì sao |
|---|---|---|---|
| «Kẹt xe» | «đi xe»: xe + mặt mệt, không có gì nói *vì sao* không đi | A khối chặn trơn · A2 khối + hai đèn hậu coral · A3 khối + dải kính + cản · B ba thanh (hàng xe) | **A3**: đuôi xe buýt chạm bánh trước, kèm tay chống cằm (`ngoi-xe` đổi tay, mắt chúc xuống). A2 bỏ vì hai chấm + vạch **thành một khuôn mặt**; B đọc ra hàng rào |
| «Trả tiền nè» | «đưa vé»: hai tờ chồng góc coral | A ba tờ xoè · B hoá đơn xé đôi | **A**: xoè ba tờ, ngang hơn cao, là silhouette tiền mặt duy nhất không cần ký hiệu tiền (giữ ranh giới ADR-0021: không xu/tick/₫/QR/ngân hàng). B bỏ vì **đưa hoá đơn** đọc thành đòi tiền — ngược câu |
| `chua-co-tin-nhan` | cùng dáng `ghi-lai` với `chua-co-keo`, chỉ đổi đạo cụ | A tay giơ bong bóng như cờ · B `moi-ly` giơ bong bóng · C bong bóng mọc từ **miệng**, tay khum cạnh miệng, tay xa mở | **C** → pose mới `goi-loi`. A/B đều đọc như cầm **bảng**; lời nói phải mọc từ miệng |
| `chua-doc-duoc` | vòng coral rời dưới tờ rách ≈ spinner đứng yên | D1 bỏ vòng, coral **ở vết rách**, mảnh rách trượt sang | **D1**: «đã rách» thay «đang tải» |

**Cơ chế thay lời hứa.** (1) Pose của cảnh nay là **dữ liệu**: bảng `NEP` trong `canh.ts` là
`{pose, x0, y0?, tiLe, them?, truoc?}` và `POSE_CANH` đọc từ đúng entry `hinhCanh` vẽ — `art-duong.test.mjs`
đòi mười giá trị **không trùng** (đỏ trước khi đổi pose: `ghi-lai` hai lần); refactor bảng là thuần — 10/10 cảnh
so byte với mốc trước khi đổi bất cứ pose nào. (2) Cảnh lỗi: test đòi **không cung tròn coral** và coral phải là
**nét gấp khúc** của vết rách (đỏ trước sửa: `vongHo` là cung). (3) Pose `goi-loi` và tay mới của `ngoi-xe` qua
sweep `khongCatNepGap` (nghiêng −8..13 × đậm) và cổng hai cỡ đọc — tay khum ở (37,58) nằm trái miệng (x ≥ 44)
và cách nếp gấp hơn hai mươi đơn vị theo thiết kế, không phải tình cờ.

**Kiểm chứng người dùng không nhãn** — của team: `nghia-hinh/khong-nhan-sang.png` / `-toi.png` là bảng **không
nhãn** (Kẹt xe · Trả tiền nè ở 120/64, `chua-co-tin-nhan` · `chua-co-keo` · `chua-doc-duoc` · `chua-co-loi-moi`).
Hỏi vài người chưa đọc brief «hình này nói gì», ghi nguyên văn. Tôi **không** làm và không bịa tỉ lệ nhận biết.

**Vòng review context mới → `ship`.** Reviewer đọc **mù** bảng không nhãn (trước khi đọc README) ra đúng bốn
nghĩa: «đứng sau đuôi xe buýt, chán» · «đây, cầm lấy» · «gọi mà chưa ai trả lời» · «tài liệu rách, mảnh rơi
ra». Không material fix. Hai ghi chú không chặn được nhận: bỏ **vệt khói** sau bánh «Kẹt xe» (một lớp; tín hiệu
chuyển động ngược nghĩa), và `bo-loc-che-het` còn dùng `vongHo` trên lưới — cùng hình vừa bị loại ở cảnh lỗi →
**để team quyết**, ngoài phạm vi F45. Reviewer đòi hiện vật của phép so byte: `nghia-hinh/canh-truoc.json` là
mốc, README ghi lệnh so.

**Bằng chứng native** (`native-r13/`): bảng `.maestro-bs-r13` XANH hai lượt (lượt 2 sau khi bỏ vệt khói) — tám
sticker ở 120/64 và khay thật (flow 71), cảnh có/không Nếp gồm `chua-co-tin-nhan` mới (flow 73), và **màn lỗi
thật** «Đi đâu?» với cảnh `chua-doc-duoc` mới (flow 77, cùng đường ảnh 22 của audit). Không path nào làm
`PathParser` ném. **Chưa đo trên máy**: theme tối (bảng art phủ), iOS, máy thật.

### F46 — DESIGN.md drift (P3)

Đoạn «Mười cảnh» viết lại thì hiện tại với pose thật (`nang-bong` ghi «vẽ 08/09, gỡ 09/09»; `chua-co-loi-moi` =
`dua-hai-tay`); bốn dòng lịch sử ở mục bằng chứng gắn tiền tố «Lịch sử tới 08/09 (`a03f563d`)» + dòng hiện hành
(#587, #588). Không xoá bằng chứng cũ.

## PR 4 — cổng motion hữu hạn (§5)

Đúng phạm vi audit đề nghị: **đo**, không thêm animation. Build release x86_64 (Hermes, bundle nhúng, env
fixture) cài đè dev client trên `emulator-5554`; bốn chuỗi thao tác Maestro (đổi tab · cuộn Khám phá · mở/đóng
sheet «Tạo mới» · vào chi tiết rồi Back), mỗi chuỗi `gfxinfo reset` → chạy → đọc tổng khung, % janky,
p50/p90/p95/p99, khung chậm/đóng băng; lượt hai với ba `*_animation_scale = 0` (Reduce Motion của hệ).
Phương pháp, bảng số và giới hạn ở `docs/archive/claude/2026-09-10/motion/README.md`.

**Tầng 1 — dev client + bundle dev trên fixture (đo xong).** Cửa fixture đòi `__DEV__` theo thiết kế nên release
không vào được bản trải nghiệm (lượt release đầu đỏ ở màn «Chào bạn»); tầng này là **giới hạn trên** của jank.
Lượt thường:

| chuỗi | khung | janky | p50 | p90 | p95 | p99 | slow UI | slow draw | missed vsync | maestro |
|---|---|---|---|---|---|---|---|---|---|---|
| m1-doi-tab | 804 | 12.69% | 26ms | 36ms | 61ms | 150ms | 54 | 87 | 16 | rc=0 |
| m2-cuon-kham-pha | 1482 | 6.34% | 16ms | 32ms | 36ms | 117ms | 30 | 78 | 17 | rc=0 |
| m3-sheet-tao | 933 | 11.79% | 26ms | 36ms | 53ms | 150ms | 32 | 97 | 16 | rc=0 |
| m4-back-chi-tiet | 812 | 12.81% | 28ms | 36ms | 65ms | 150ms | 47 | 87 | 18 | rc=0 |

Lượt Reduce Motion (`*_animation_scale` = 0): mọi chuỗi vẫn `rc=0`, số khung giảm 2–4 lần vì khung chuyển cảnh
không còn, nên **% janky không so được giữa hai lượt** (đổi tab 804 → 226 khung, UI chậm 54 → 44); p99 = 150ms là
trần bucket của histogram, không phải giá trị đo. Bảng đầy đủ + gfxinfo thô + ảnh cuối chuỗi ở
`motion/dev-client/`.

**Tầng 2 — release + stack live (đăng nhập OTP).** Đã thử hai lần, **chưa có số hợp lệ**: (1) release + fixture đỏ ở màn «Chào bạn»
vì cửa fixture đòi `__DEV__` theo thiết kế; (2) release + stack live (`e2e_slice --keep`, `adb reverse`, `healthz`
xanh) đỏ ở «Gửi mã» với chính câu F43 mới «Không nối được máy chủ…» — bản release **chặn HTTP cleartext**
(chỉ manifest debug bật). Bảng lượt ấy mọi chuỗi `rc=1`, không dùng. Đo tiếp cần **quyết định của team**: bật
cleartext tạm/cục bộ cho bản đo (đổi tư thế bảo mật release dù không commit — bộ phân loại chặn, tôi không
vòng qua) hoặc đặt stack sau HTTPS local. Chuỗi đo đã sẵn, chạy được ngay khi có một trong hai
(`motion/README.md` tầng 2, `motion/release-thu/`).

**Nói thẳng**: máy ảo trên WSL2 không phải điện thoại — số này là **đường cơ sở hồi quy của rig**, không phải
phán quyết «mượt». Khung hình giữa chuyển cảnh khi Reduce Motion bật chưa chụp được xác định. «Dấu giấy khi
chốt kèo» (§5 bước 3) là đề xuất cho team sau khi có số — không làm lượt này.

## Cố ý không làm trong lượt này (nói rõ, không nhận vơ)

- Nợ cũ §4 của audit: ghi công ảnh Album/timeline demo, `Photo` trần thiếu chữ lỗi, nút vô hiệu coral
  nhạt, dấu phân cách credit, glyph Puppy Farm.
- Kiểm chứng người dùng **không nhãn** cho F45 — của team, với bảng không nhãn tôi giao ở PR 3.
- `bo-loc-che-het` còn dùng `vongHo` trên lưới lọc (reviewer PR 3 chỉ ra: cùng hình vòng hở vừa bị loại ở
  cảnh lỗi, và nghĩa «bàn trống một bên» không hợp lưới) — quyết định mỹ thuật của team, không phải chỗ thiếu.
- iOS · máy thật · TalkBack · tablet · IME trong chat live.
- Animation mới có nghĩa (§5 bước 3) — quyết định sau khi có số đo ở PR 4.
