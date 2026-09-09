# Trả lời audit native 09/09 — F41–F46 và cổng motion

Ngày: 10/09/2026. Nền: `docs/codex/2026-09-09/audit-native-toan-app.md` trên `main 92d0f241` —
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

## Cố ý không làm trong lượt này (nói rõ, không nhận vơ)

- Nợ cũ §4 của audit: ghi công ảnh Album/timeline demo, `Photo` trần thiếu chữ lỗi, nút vô hiệu coral
  nhạt, dấu phân cách credit, glyph Puppy Farm.
- Kiểm chứng người dùng **không nhãn** cho F45 — của team, với bảng không nhãn tôi giao ở PR 3.
- iOS · máy thật · TalkBack · tablet · IME trong chat live.
- Animation mới có nghĩa (§5 bước 3) — quyết định sau khi có số đo ở PR 4.
