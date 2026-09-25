# Thiết kế 05 — Nếp tại chỗ, trí nhớ lặng lẽ có công bố, nhắc khi được cho phép

- Ngày: 2026-09-25. Commit gốc: `f251db7`. protocol_version: không áp dụng (không phải lượt thí nghiệm).
- Trạng thái: **thiết kế đã được người dùng duyệt, chờ Lead ký ADR**. ADR đi kèm:
  `docs/decisions/proposals/ADR-0041-nep-thay-man-nho-lang-le-nhac-khi-cho-phep.md`.
- Phạm vi: phiếu ngữ cảnh v2, sổ tay app, tool và chip của Nếp, trí nhớ (`nepnho`), nhắc chủ động
  (`nepnhac`), push tối thiểu (`push`), bảng Nếp và animation. Vòng lặp, router, registry tool: thiết
  kế 01. Hàng đợi, SSE, task id: thiết kế 02. Ruột truy hồi: thiết kế 04. Bộ đo: thiết kế 06.
- Khi lệch: bảng «Hợp đồng chung» (`docs/architecture/03-ai-engine-hop-dong.md`) đè lên bản Nếp gốc.
- Đánh dấu: «(sửa theo phản biện)» là chỗ khác bản gốc do phản biện; `[P1-n]` là phát hiện n của phản
  biện «ràng buộc, riêng tư, bảo mật, đúng đắn», `[P2-n]` của phản biện «khả thi, thứ tự, đầy đủ».
  «(theo hợp đồng chung)» là chỗ đổi cho khớp hợp đồng; «(tự phát hiện)» là chỗ tìm ra lúc viết.

## 0. Sự thật đã kiểm tại `f251db7`

| Sự thật | Chỗ |
|---|---|
| Phiếu v1: `man, tieuDe, nhip, loaiSo, soLieu, goiY`; `soLieu` chỉ là số đếm; màn tiền ở một hằng | `apps/mobile/src/rudi/nep/phieu.ts:57-65`, `:30`, `:45` |
| Go kiểm lại phiếu bằng struct đóng, khoá lạ → 400 `boi_canh_sai_dang`; test lệch đọc thẳng `phieu.ts` | `services/core/internal/chatassist/nep.go:70-77`, `:115`; `nep_test.go:189`, `:221` |
| Cổng quét phiếu chỉ quét `src/`; màn plan khai phiếu hai lần | `apps/mobile/tests/nep-phieu-kin.test.mjs:16`; `app/(tabs)/plan.tsx:7-10`, `src/rudi/screens/keo/PlanLive.tsx:164` |
| Câu gợi ý hỏi thứ Nếp không làm được (gu, lịch sử, nhắc, «gần đây») | `plan.tsx:10`, `screens/Profile.tsx:64`, `screens/keo/OutingLive.tsx:187`, `screens/hai-nguoi/KhongGianGiay.tsx:86` |
| Cổng đọc Nếp chỉ quét file của chính gói; ba gốc, ba bảng | `chatassist/nep_khong_doc_test.go:31-34`, `:46` |
| Một lời gọi brain `nep-reply`; trần chữ 2000; `nepXong` đóng job bằng một UPDATE | `nep.go:399`, `:408`, `:51`, `:449`, `:466` |
| 8 lời gọi/phút/người, chung với nhóm; cửa sổ chia sẻ 15 phút; handler cắt 8 s; `Matches` chỉ khớp `/me/nep/ai-invocations` (`/me/nep/media` còn là Python) | `nep.go:306-313`, `:315`; `handler.go:83`, `:72` |
| Đóng bảng là mất câu đang chờ; tờ thứ hai có trong máy trạng thái nhưng chỉ đường QA gửi việc | `nep/useNepHoi.ts:58-68`; `nep/trang-thai.ts:90-91`, `:114`; `nep/NepNoi.tsx:33`; `nep/qa-nep.ts:29-30` |
| «Vẽ» không xét màn tiền; trạng thái chờ là một dòng chữ tĩnh | `nep/NepBang.tsx:184`, `:114` |
| Ảnh Nếp vẽ tải không kèm header; test e2e tự thêm header nên vẫn xanh | `tests/e2e/nep-gen-anh.test.mjs:181`; mẫu đúng `src/rudi/ky-niem/ky-niem.ts:201` + `src/danh-tinh.ts:109` |
| Xoá tài khoản ẩn danh hàng `people`, đặt `deleted_at`, không xoá hàng; danh sách bảng khoá theo bản đồ Python | `services/core/internal/repo/erasure.go:52`, `:220`, `:361`; `services/api/app/domain/account_lifecycle.py:64`; `services/api/app/db/models.py:999` |
| Bảng Go đang có mang cột người; chưa có `notifications`, `notification_devices`, `expo-notifications` | `chatassist/schema.sql:4` (`person_id`), `:32` (`created_by_id`); không có trong `services/`, `apps/mobile/package.json` |
| Tạo kèo đã có đường điền sẵn, nguồn là một tin chat | `screens/keo/CreateOutingLive.tsx:80-94` |
| `Skeleton` đã lặp 1400 ms; bốn bậc chuyển động; giờ Việt Nam lấy từ một hàm | `DESIGN.md:1982`, `:384`; `services/core/internal/domain/pairpaper/pairpaper.go:207` |

## 1. Thành phần và người ghi

| Thành phần | Vai | Người ghi duy nhất của |
|---|---|---|
| `nep/phieu.ts` + `kiemPhieu` (`chatassist/nep.go`) | phiếu v2; registry `BUOC_MAN`, `HANH_DONG`, `DANH_MUC` | — |
| `services/core/internal/huongdan` | sổ tay app `go:embed`, đồ thị màn | — (dữ liệu sản phẩm) |
| `aiharness/tools` (thiết kế 01) | tool của Nếp trong registry chung | — |
| `nep/{lo-trinh,nhap-tam,chu-gon,su-kien,cong-bo,goi-y}.ts`, `useNepMoc` | chip, nháp RAM, markdown-gọn, sự kiện, công bố | — |
| `internal/nepnho` | trí nhớ | `nep_cai_dat`, `nep_su_kien`, `nep_trich`, `nep_su_that`, `nep_quen`, `nep_moc`, `go_xoa_dang_ky` |
| `internal/nepnhac` | chấm và giao lời nhắc | `nep_nhac`, `nep_loi_hen` |
| `internal/push` | push tối thiểu theo ADR-0024 | `notification_devices`, `notifications`, `push_viec` |

- Mỗi gói có bảng version riêng theo mẫu `chatassist/migrate.go:41`, số cấp theo thứ tự lên main;
  `serve`/`work` từ chối chạy khi version < N (theo hợp đồng chung) [P2-1]. `chatassist` không ghi bảng
  `nep_*`: `nepXong` gọi `nepnho.GhiLuot(tx, …)` trong cùng transaction (sửa theo phản biện) [P1-9].
- Route mới (`/me/nep/su-kien`, `/me/nep/cai-dat`, `/me/nep/viec…`, `/devices…`) có `Matches` riêng của
  từng gói, và vào `ownership/routes.json` loại `GO-ONLY` **cùng commit** với handler (sửa theo phản
  biện) [P2-17].

## 2. Phiếu ngữ cảnh v2 (lát 13)

```ts
interface PhieuNguCanh {           // v2; v1 vẫn được nhận trong lúc chuyển
  ban?: 2; man: string; tieuDe?: string; nhip?: NhipKeo; loaiSo?: LoaiSo;
  soLieu?: Partial<Record<KhoaSoLieu, string | number>>; goiY?: readonly string[];
  buoc?: BuocMan;                  // "<màn>:<trạng thái>", registry đóng
  thay?: { diaDiem?: string[];     // ≤8 id catalogue, không bao giờ tên
           chang?: { gio?: string; diaDiem?: string }[];  // ≤12, "HH:MM"
           diemDen?: string; danhMuc?: DanhMuc };
  hanhDong?: HanhDong[];           // ≤12 id trong HANH_DONG, có mặt NGAY LÚC NÀY
  chon?: `dia-diem:${string}` | `chang:${number}`;
  banBuild?: string;               // 12 hex: băm sổ tay mà bản app này dựng cùng
}
```

- Không bao giờ mang tên người, chữ chat, hay tiền (`_vnd`, ngân sách, giá), kể cả khoá lồng trong `thay`.
- **Bỏ `chang[].nhan` của bản gốc (theo hợp đồng chung: không tên người).** Nhãn chặng tự do («nhà Lan»)
  có thể chứa tên; chặng chưa gắn địa điểm chỉ còn giờ. Câu hỏi mở số 1 của bản gốc được đóng.
- Client gửi id, không gửi tên. Go tra `repo.GetPlace` → `service.PlaceRow` (`service/catalogue.go:120`)
  → `promptsafety.Safe` (`promptsafety.go:98`). Id lạ: 400 `boi_canh_sai_dang`; hàng không an toàn: bỏ.
  Đường này thêm `places` vào allowlist gốc Nếp, nên chỉ vào main **sau** cổng đọc xuyên gói lát 5
  (sửa theo phản biện) [P2-5].
- «Mình đang thấy» mở ra thành danh sách dựng từ chính object sắp gửi (mẫu ADR-0036 §2.5). Registry có
  bản Go; test lệch mở rộng `TestNepDanhSachKhopVoiPhieuTs` bằng helper `doc(…)` sẵn có.

| Route | `buoc` | `thay` | `hanhDong` |
|---|---|---|---|
| plan (chỉ `PlanLive`; bỏ bản trong `app/(tabs)/plan.tsx`) | `plan:co-keo`, `trong`, `khong-nhom` | `soLieu.soMuc` | `tao-keo`, `mo-keo-toi` |
| `outings/[id]` | `outing:lich-trinh`, `hanh-trinh`, `nhap-thu-tu` | `chang`, `chon` | `them-chang`, `gan-dia-diem`, `toi-da-toi`, `luu-thu-tu`, `xep-theo-gio`, `che-do-hanh-trinh`, `xem-cach-di-gon`; không bao giờ `chia-bill` |
| `groups/[id]/to-giay` | `to:<13 trạng thái>` | chỉ giờ chặng (chữ `viec` do hai người cùng viết) | 1:1 từ `nutChoTo()` (`to-giay/to-giay.ts:317`) |
| `groups/[id]/chat` | `chat:thuong`, `co-binh-chon-mo`, `co-to-hen` | `soLieu.soMuc` | `mo-khay`, `tao-binh-chon`, `mo-to-hen`, `chot-binh-chon` |
| explore | `kham-pha:duyet`, `ket-qua` | `diaDiem` đang hiện, `diemDen`, `danhMuc` | `loc-danh-muc`, `tim-ai`, `doi-diem-den` |
| `places/[id]` (mới khai) | `dia-diem:xem` | `diaDiem`, `chon` | `them-vao-keo`, `ru-toi-day`, `luu-dia-diem` |
| `outings/new` (mới khai) | `tao-keo:trong`, `dang-dien`, `tu-to-hen` | `soNgay`, `soNguoi` | `tao-keo-xac-nhan`; không liệt kê chip ngân sách |

**Sửa câu gợi ý.** Gợi ý thành `{chu, yDinh}` trong `nep/goi-y.ts`; `nep-goi-y.test.mjs` đỏ khi màn khai
gợi ý có ý định mà `quyen.golden.json` chưa bật cho Nếp. «Gu của mình đang thế nào?» (`Profile.tsx:64`)
bị thay, vì gu vẫn bị cấm.

## 3. Sổ tay app (lát 13)

- **Một chỗ:** `services/core/internal/huongdan/data/*.md`, nhúng `go:embed` (sửa theo phản biện) [P2-6].
  Core build với `context: ./services/core` và `go:embed` không ra khỏi module, nên `docs/huong-dan-app/`
  (bản gốc) và `apps/mobile/huong-dan/` (RAG) đều bỏ.
- **Định dạng đã chốt khi dựng** (theo bản dựng lát 13, `5c3a3c1`; ghi lại theo review lát 13 phát hiện 6).
  Bản thiết kế ban đầu (mỗi luồng một mục, front matter `{id, man[], buoc[], hanhDong[], loTrinh[], nhanUI[]}`)
  không được dựng. Dựng: **mỗi màn một file**, front matter là JSON giữa dòng `---json` và dòng `---`, đúng
  năm khoá `{man, tieu_de, nhanUI[], di_toi[{nhan, man}], tien}`; khoá lạ bị từ chối, kể cả tên cũ `nut`,
  `buoc`, `hanhDong`; khoá trùng trong một object hay viết sai hoa thường cũng bị từ chối (JSON của Go và JS
  giữ khoá sau cùng và im lặng, Go còn khớp khoá không phân biệt hoa thường; sửa theo review lát 13 vòng 2).
  `man` là route id đúng như `PhieuNguCanh.man`. Mỗi mục `## ` là một việc và một đoạn
  truy hồi, 1–5 bước; id mục = `<tệp>/<slug tiêu đề>`. Luật nạp (`huongdan.nap`, phaiNap panic lúc init):
  mọi `man`/`di_toi[].man` có trong `_rut.json`; không `di_toi` nào về chính màn đó; mọi `di_toi` là một
  cạnh của mã (`_rut.json` `di_toi`, thanh tab, hoặc ngoại lệ có tên `canhNgoaiRut` kèm lý do — hiện chỉ
  `plan → create`, nút «Tạo mới» của `RudiTabBar`); mọi «…», kể cả trong tiêu đề mục và tổng quan, khai trong
  `nhanUI`; `tien` phải đúng theo route (`manTienDau` = `MAN_NEP_LUI`). Màn tiền: một mục duy nhất, tiêu đề
  cố định «Tới màn này và đi tiếp», không chữ số, mọi dòng là bước, mỗi bước trích ít nhất một «cửa» và **chỉ
  trích cửa** (tiêu đề mục in trên màn như «Chi theo nhóm», hay nút trả tiền đứng cạnh một cửa, đều không được
  trích; nhắc tới tiêu đề mục thì viết chữ thường, không «…»); cửa là nhãn của
  một lối vào/ra đã khai mà **là cạnh có nhãn của mã** (`_rut.json` `canh`: nút mang đúng nhãn đó và điều
  hướng tới đúng màn đó), hoặc tiêu đề của một màn không phải màn tiền có lối vào, in trên màn đó. Người
  review viết văn; model soạn nháp chỉ khi Lead duyệt số lời gọi.
- **`buoc` và `hanhDong` đi trên phiếu v2, không vào front matter.** Trạng thái màn thuộc về mã màn đó
  (registry đóng `BuocMan` trong `phieu.ts`), không thuộc văn sổ tay. Lát 9 thêm bảng Go `buocMuc`
  (cùng registry tool): mỗi giá trị `BuocMan` → danh sách id mục của đúng màn đó. `TheoMan(man)` giữ chữ ký;
  `explain_screen` lấy `TheoMan(phieu.man)` rồi đưa các mục của `buocMuc[phieu.buoc]` lên trước; `buoc` lạ
  hoặc không có mục thì giữ thứ tự tệp. `hanhDong` không lọc sổ tay: nó chỉ quyết chip nào được ra.
  Test lệch khi đó: mọi `BuocMan` có mục trong `buocMuc` hoặc nằm trong danh sách «không có hướng dẫn
  riêng» kèm lý do; mọi id trong `buocMuc` tồn tại và thuộc đúng màn.
- `apps/mobile/tools/rut-huong-dan.mjs` rút ra `data/_rut.json`: route (cây expo-router), nhãn (literal
  JSX), cạnh điều hướng (`router.push/replace` literal), và cạnh có nhãn `canh` (nhãn và điều hướng nằm trên
  cùng một thứ người ta bấm, và nhãn là tên của đúng cú bấm đó: trên một thẻ JSX, `onPress`/`href` ghép với
  `label`, `accessibilityLabel` hay `title`, `onAction` chỉ ghép với `action`, handler khác không ghép với nhãn
  nào; hoặc một object menu `{title|label, href|onPress}`. Thêm theo review lát 13; vòng 2 bỏ ghép `title` với
  `onAction`, vì `<SectionHeader title="Chi theo nhóm" action="Xem quyết toán" onAction>` là một tiêu đề mang
  một nút, không phải nút «Chi theo nhóm»). Cổng tươi: CI chạy lại, diff phải rỗng. `banBuild`
  = 12 hex đầu sha256 của `_rut.json`, client nhúng qua `nep/huong-dan-ban.ts` (sửa theo phản biện) [P2-6].
- **API (theo bản dựng):** `TheoMan(man)` trả các mục của một màn theo thứ tự tệp (nhận route khai hoặc
  route đi). `Tim(ctx, Hoi{Cau, Man, K})`, K mặc định 4: chỉ đọc 2000 rune đầu của câu (`MaxRuneCau`, bằng
  `maxHoiNep`); âm tiết teencode mà sổ tay không dùng được viết lại theo bảng đóng `teen` (không dịch tiếng
  Anh); BM25 qua `rag/xephang` trên tiêu đề mục + thân; một mục chỉ «khớp» khi chung ít nhất một âm tiết nội
  dung (ngoài `tuDem`). **Luật ghim:** mục của màn đang đứng chỉ lên trước khi điểm ≥ 1/2 điểm cao nhất
  (`tyLeGhim`), phần còn lại giữ thứ tự điểm. Ghim mọi mục khớp (bản đầu) làm câu hỏi từ màn khác có MRR
  0.3526 trên bộ `truy-hoi-man-khac.json`. Tìm trong RAM, ≤50 ms; version hoá trong DB để sau [P2-6].
  `DuongToi(tu, den)` là BFS trên đồ thị màn cho câu «làm sao tới X», ≤5 bước, không đi qua màn đăng nhập,
  và giữa các đường cùng độ dài thì chọn đường đi qua ít màn tiền nhất. Ruột `search_app_manual` thuộc thiết
  kế 04.
- **Luật trả lời «muốn chọn A/B/C thì làm sao»:** ≤5 bước; `nguon[]` liệt kê id mục đã trích; nhãn trong
  «…» chỉ lấy từ `nhanUI` của mục đã trích. Output guard (cửa sổ 48 rune, thiết kế 01 §3.5) quét mọi «…».
  Nhãn lạ trước delta đầu: thử lại một lần. Sau khi đã stream: bỏ «», đếm `nhan_la`, **không rút lại**
  (theo hợp đồng chung).
- **Cổng lệch** (`apps/mobile/tests/huong-dan-khop-ma.test.mjs`, kiểm độc lập với bộ nạp Go): `_rut.json` và
  `huong-dan-ban.ts` tươi; mọi `nhanUI` là literal có thật trong `apps/mobile/{src,app}` và được dùng; mọi
  `di_toi` là cạnh của mã (cùng danh sách ngoại lệ `CANH_NGOAI_RUT`), không về chính màn; luật màn tiền như
  trên, kể cả cửa phải là cạnh có nhãn. Khi phiếu v2 và `buocMuc` có (lát 9): mọi `buoc` có trong registry.

## 4. Tool và chip của Nếp (theo hợp đồng chung)

Một registry trong `aiharness/tools`. Bỏ `nep_cong_cu.go` và các tên riêng của bản gốc (sửa theo phản
biện) [P1-6][P2-4].

| Bản gốc | Tên chốt | Ý định Nếp | Điều kiện |
|---|---|---|---|
| `giai_thich_man` | `explain_screen` | `explain_screen`, `app_help` | không DB |
| `tra_huong_dan` | `search_app_manual` | `app_help`, `explain_screen` | `huongdan.Tim` |
| `tim_dia_diem` | `search_places`, `get_place`, `list_destinations`, `nearest_area` | `find_places`, `plan_help` | id trả về vào ledger |
| — | `suggest_screen` | mọi ý định trừ `smalltalk` | chỉ ra chip |
| — | `propose_places`, `propose_itinerary` | `plan_help` | nháp, không DB; ra chip `dien_nhap` |
| `keo_sap_toi` | `my_upcoming_outings` | `plan_help`, `remind` | chỉ trong lời gọi `hoi` do chính người đó gửi |
| — | `recall_memory` | `find_places`, `plan_help` | `nho` bật; **chỉ từ gốc scope=me** [P1-1] |
| `nho` | `remember_fact` | `remember` | `nho` bật; fact nằm trong `hoi` của lượt này [P1-9] |
| `quen` | `forget_fact` | `forget` | luôn được |
| `hen_nhac` | `set_reminder` | `remind` | `nhac` bật; lớp «ghi của chính mình» [P2-4] |
| `nho_gi` | không phải tool | `what_you_remember` | Go liệt kê, không qua model |

- `my_upcoming_outings`: kèo đang hoạt động của chính người đó trong ≤14 ngày, trả id, tiêu đề
  (`TextSafe` 60), ngày, giờ chặng, id địa điểm. Đọc `outings`, `outing_stops`, `memberships`; cấm cột
  `budget*`/`_vnd`; không lưu gì; không cần công tắc vì dữ liệu vốn của người đó và đang hiện ở tab plan.
  ADR-0041 đóng mâu thuẫn giữa bảng quyền harness và bản gốc ở đây (sửa theo phản biện) [P2-2].
- **Ý định `remind` chưa có** trong tập ý định Nếp của thiết kế 01 §3.3 (tự phát hiện). Lát 17 thêm nó
  cùng hàng `set_reminder` trong `quyen.golden.json`; tới lúc đó tool này tắt.
- **Chip chỉ đi trong `xong.chips`**, không bao giờ rút ra từ chữ trả lời:
  `{"id":"c1","loai":"mo|chi|mo_to|dien_nhap","nhan":"Mở kèo","dich":{…}}`.
  - `mo`: route trong `nep/lo-trinh.ts`, có bản Go; cổng `lo-trinh` ∩ `MAN_NEP_LUI` = ∅.
  - `chi` / `mo_to`: màn đăng ký mốc bằng `useNepMoc(id, ref, {mo?})`; chỉ nhận id có trong `hanhDong`
    của phiếu **và** khi `banBuild` khớp. `chi`: bảng đóng, phần tử cuộn vào tầm nhìn, một vòng mực vẽ
    quanh nó. `mo_to`: gọi callback `mo` của mốc.
  - `dien_nhap`: nháp trong RAM ở `nep/nhap-tam.ts` (token, 10 phút, không xuống đĩa).
    `/outings/new?nepNhap=<token>` là nguồn thứ ba của `CreateOutingLive`, cạnh `sourceMessageId`.
    Banner «Nếp điền sẵn — xem lại rồi bấm Tạo kèo». Ngân sách không bao giờ được điền sẵn.

## 5. Một lượt Nếp trên engine (lát 6, lát 11)

1. `POST /me/nep/ai-invocations` như hôm nay, trả 202 `{id}`: đó là task id. Client giữ `{id, luot}`
   trong `NepProvider` (RAM); đóng bảng không giết câu đang chờ; xong thì báo bằng tờ thứ hai
   (`xong-viec`), `hienToSau` giấu tờ đó ở màn tiền (thiết kế 02 §5.4) [P2-13].
2. Worker gọi `Engine.Run(Turn{Bot: nep, Lenh: hoi, PhieuNep, LuotNep}, sink)`. Preprocess gắn «bây giờ»
   theo `pairpaper.Local`. Guard bỏ lượt phiên bị gắn cờ, kể cả lượt vai `nep` client gửi lại (sửa theo
   phản biện) [P1-24]. Lượt phiên không bao giờ là nguồn để trích trí nhớ.
3. **Định tuyến tất định trước Understand** (tự phát hiện, cùng hướng [P2-10]): «bạn nhớ gì (về) mình»
   → `what_you_remember`, 0 lời gọi; yêu cầu tiền hoặc màn tiền → từ chối, 0 lời gọi; còn lại → Understand
   (một lời gọi, đầu ra toàn enum và slot, bọc `<du_lieu>`).
4. Ý định: `app_help, find_places, explain_screen, plan_help, remember, forget, what_you_remember,
   smalltalk` (+`remind` từ lát 17). Fast path cho một ý định `find_places | app_help | explain_screen`;
   còn lại agent ≤3 bước; cả hai qua `MaxModelCallsPerTurn` (theo hợp đồng chung).
5. **Sự kiện stream** (theo hợp đồng chung) [P2-3]: `hello`, `trang_thai{cau}`, `delta{p,text}`,
   `lam_lai`, `xong{text,chips,nguon}`, `that_bai{code}`, `huy`, `thu_hoi`, `ket_noi_lai`, `: ping`.
   `the`, `chip`, `loi` của bản gốc bỏ; Nếp v1 không dùng `phan`. Không có câu trạng thái riêng cho trí
   nhớ, để trí nhớ đúng là lặng lẽ.
6. `nepXong` commit kết quả kín; khi `nho` bật, cùng transaction gọi `nepnho.GhiLuot(tx, person,
   invocationID, hoi)`. Chỉ **câu hỏi của người dùng** được giữ, không giữ câu trả lời (sửa theo phản
   biện) [P1-9]. `nguon` hiện thành chú thích nhỏ dưới câu trả lời («Sổ tay app · Danh mục quán · Điều
   Nếp nhớ về bạn»): trí nhớ được nói ra đúng chỗ nó được dùng, không bằng toast.

**Bảng Nếp (lát 11, UI thiết kế bằng `/impeccable`).**
- Mỗi lượt là một tờ giấy dùng token sẵn có. Chữ đi qua `nep/chu-gon.ts`: đoạn, `**đậm**`, danh sách
  `- ` và `1. `; không link, HTML, ảnh, tiêu đề. Chip là `RudiButton compact outline tone="ai"`
  (`theme.ts:11`).
- **«Mực chờ» + icon Nếp «thở giấy».** Nét mực SVG cạnh Nếp: vẽ 300 ms, giữ 550, nhấc 300, nghỉ 100
  (bậc của `DESIGN.md:384`). Tờ giấy dưới icon phồng 1.00→1.02 kèm bóng; toạ độ nét vẽ Nếp không đổi,
  nên `art-duong` giữ nguyên. Chạy từ lúc chạm gửi (tại chỗ, trước mạng) tới `delta` đầu; sau 20 s đứng
  yên. Reduce Motion: một nét tĩnh. Trả `null` khi `!nepDuocHoi`. Không vòng xoay.
- **Không gọi đây là «vòng lặp duy nhất»**: `Skeleton` đã lặp 1400 ms (sửa theo phản biện) [P2-17]. Phần
  sửa `DESIGN.md` («Luật Mực Chờ»: chỉ trong bảng Nếp, tắt dưới Reduce Motion, không ở màn tiền) Lead ký
  riêng, cùng lát 11.

## 6. Trí nhớ `nepnho` (lát 15)

```sql
nep_cai_dat(person_id uuid PK, nho bool NOT NULL DEFAULT false, nho_tu timestamptz, cong_bo_ban smallint,
  da_bao_luc timestamptz, nhac bool NOT NULL DEFAULT false, nhac_day bool NOT NULL DEFAULT false,
  gio_yen_tu smallint NOT NULL DEFAULT 1290, gio_yen_den smallint NOT NULL DEFAULT 480);
nep_su_kien(id bigint IDENTITY PK, person_id uuid NOT NULL, loai text NOT NULL CHECK (loai IN (<10 loại>)),
  luc timestamptz NOT NULL, man text, dia_diem_id text, diem_den_id text, danh_muc text, gia_tri text,
  ghi_luc timestamptz NOT NULL DEFAULT now());                                  -- giữ 30 ngày
nep_trich(id uuid PK, person_id uuid NOT NULL, invocation_id uuid UNIQUE,
  hoi text NOT NULL CHECK (char_length(hoi) <= 2000), het_luc timestamptz NOT NULL, -- luc + 48 h
  status, attempts, lease_id, lease_until, available_at, enqueue_seq);          -- thiết kế 02 §4.11
nep_su_that(id uuid PK, person_id uuid NOT NULL, loai text NOT NULL CHECK (loai IN (<9 loại>)),
  khoa text NOT NULL, gia_tri jsonb NOT NULL, cau text NOT NULL CHECK (char_length(cau) <= 160),
  nguon text NOT NULL CHECK (nguon IN ('hanh_vi','noi_ro','hoi_dap')),
  tin_cay smallint NOT NULL CHECK (tin_cay BETWEEN 0 AND 100), so_bang_chung int NOT NULL,
  hieu_luc_tu timestamptz NOT NULL, hieu_luc_den timestamptz, ghi_luc timestamptz NOT NULL DEFAULT now(),
  bang_chung_cuoi timestamptz NOT NULL, thay_boi uuid REFERENCES nep_su_that ON DELETE SET NULL);
CREATE UNIQUE INDEX nep_su_that_hien_hanh ON nep_su_that(person_id, khoa) WHERE thay_boi IS NULL;
nep_quen(person_id uuid, khoa_bam bytea, den timestamptz NOT NULL, PRIMARY KEY (person_id, khoa_bam));
nep_moc(person_id uuid PK, su_kien_den bigint NOT NULL, luc timestamptz NOT NULL);
```

- **Một schema, do `nepnho` sở hữu** (sửa theo phản biện) [P1-2][P2-7]: `forgotten_at`, `superseded_at`,
  `vector(768)` và loại `an_uong` của RAG bỏ; móc của RAG thành tìm từ vựng trên `cau` cộng lọc loại.
- **Hai mốc thời gian, không giữ bản cũ.** `hieu_luc_tu/den` là lúc điều đó đúng; `ghi_luc` là lúc Nếp
  ghi. `thay_boi` chỉ nối tới fact sẽ thay nó **trong tương lai** («từ tháng 10 mình đi xe buýt»). Fact
  bị thay hay hết hạn bị **xoá** ở lần củng cố kế tiếp, không giữ 180 ngày như bản gốc (theo hợp đồng
  chung) [P1-2].
- **Loại fact (đóng):** `thich_danh_muc`, `thich_dia_diem`, `ne_dia_diem`, `diem_den_quen`,
  `khung_gio_hay_di`, `phuong_tien`, `thoi_luong_chang`, `nhip_len_keo`, `dieu_da_dan`. `cau` do Go dựng
  từ `(loai, gia_tri)` bằng mẫu; riêng `dieu_da_dan` là đoạn ≤120 rune cắt nguyên văn lời người dùng.
- **Không bao giờ nhớ:** tiền; người, tên nhóm, quan hệ; chữ chat; toạ độ; đặc điểm nhạy cảm (ADR-0034
  §2.3); sức khoẻ, gồm dị ứng và ăn kiêng, tới khi Lead quyết (mục 12).

**`kiemSuThat()`** chạy trước mọi lần ghi, từ chối im lặng; lý do chỉ đếm bằng enum, không gắn id người
[P1-3]. (1) Giá trị là enum hoặc id catalogue có thật. (2) Đoạn `dieu_da_dan` nằm nguyên trong `hoi` sau
NFC và `Fold`. (3) Slot `chu_the` của Understand là `toi`. (4) Deny-list đã `Fold`: đại từ ngôi ba và từ
quan hệ («anh ấy», «vợ», «bạn mình»…), tiền, nhạy cảm, sức khoẻ. (5) Qua `aiharness/guard/patterns.go`.
(6) Khoá không nằm trong `nep_quen`. «Lan dị ứng tôm» bị loại ở (3) và (4) (sửa theo phản biện) [P1-9].

**Nguồn 1: thao tác trong app, `POST /me/nep/su-kien`.**
- Lô ≤50, `loai` đóng: `mo_dia_diem`, `luu_dia_diem`, `bo_luu`, `them_chang`, `chon_phuong_tien`,
  `chon_thoi_luong`, `loc_danh_muc`, `chon_diem_den`, `tao_keo`, `check_in`. Tham chiếu chỉ là id có kiểu
  hoặc enum.
- Phát từ PlaceDetailLive, OutingLive, SoHanhTrinh, ExploreLive, CreateOutingLive qua `nep/su-kien.ts`;
  hàng chỉ trong RAM, xả mỗi 30 s hoặc khi app vào nền. Máy bỏ sự kiện khi công tắc tắt hoặc
  `nepPhaiLui(man)`; máy chủ trả 204 và không ghi gì khi `nho` tắt.
- `luc` kẹp vào [now−7 ngày, now+5 phút]. 1 lô/10 s mỗi người; vượt thì 429 và máy bỏ lô.

**Nguồn 2: lời người dùng nói với Nếp.**
- `nep_trich` giữ `hoi` ≤48 h. Nó là bảng việc có trigger vào outbox, hàng `memory` (sửa theo phản biện)
  [P2-14]; `available_at = now + 10 phút` để gom một phiên.
- Consumer lấy khoá advisory theo người, đọc lại `nho`, nạp mọi hàng đang chờ (≤20), rồi gọi **một** lần
  `nep.trich` flash-lite qua bộ đếm và bộ giới hạn của `aiharness/llm`. JSON schema `{ops:[{op:add|update|
  supersede|expire, loai, gia_tri, fact_id?, chu_the, tin_cay}]}`.
- Chỉ lời của chính người dùng; không câu trả lời của model, không kết quả tool (sửa theo phản biện)
  [P1-9]. Hàng đã đọc bị xoá; lỗi ba lần cũng xoá; không bao giờ giữ quá `het_luc`.

**Củng cố tất định** bằng `jobs.DinhKy{"nep-cung-co", 03:00 hằng đêm}`, sớm hơn khi đủ 50 sự kiện mới;
không có lease riêng trong `nepnho` (sửa theo phản biện) [P2-14]. Trong một transaction, dưới khoá
`nepnho:<person>`:
1. Nạp sự kiện trong 30 ngày và fact hiện hành (≤200).
2. Gộp tất định. Ví dụ `thich_danh_muc:X` khi ≥3 ngày khác nhau trong 30 ngày, `tin_cay = min(90,
   40+10·ngày)`; đảo chiều 3:1 trong 21 ngày thì thay fact cũ.
3. Suy giảm: hiệu lực = `tin_cay·0.5^(tuổi/bán_rã)`, bán rã 60 ngày (khung giờ), 90 (gu), 120 (phương
   tiện), 365 (điều đã dặn). Dưới 30 thì xoá. Trần 200 fact, bỏ fact yếu nhất. Sự kiện quá 30 ngày xoá.

**Lệnh tường minh** (dưới cùng khoá)
- `remember_fact`: chỉ khi ý định `remember` và `nho` bật; ghi ngay, `tin_cay` 95, `nguon=noi_ro`. Khi
  `nho` tắt thì trả câu cố định kèm chip `mo` tới Cài đặt.
- `forget_fact({khoa?} | {cau_khop})` **xoá cứng** (theo hợp đồng chung) [P1-2]: hàng fact của khoá, sự
  kiện đã góp vào khoá (ánh xạ tất định `loai` → điều kiện cột), mọi `nep_trich` đang chờ của người đó;
  rồi ghi tombstone `nep_quen` giữ 365 ngày. Khớp theo id fact trong `result.nguon` của lời gọi trước của
  chính người đó (nếu còn trong cửa sổ), rồi so `cau` đã `Fold`. Nhiều khớp: Go liệt kê và hỏi lại, chưa
  xoá. Không khớp: «Mình không nhớ điều đó.»
- **Tombstone băm bằng HMAC-SHA256 với khoá máy chủ** (tự phát hiện): không gian khoá nhỏ (enum, id
  catalogue), nên sha256 trần của `person‖khoa` dò ngược được.
- `what_you_remember` do Go dựng, không qua model [P1-2]: mọi `cau` hiện hành kèm ngày (kể cả fact sắp
  hết hiệu lực), số thao tác trong 30 ngày theo loại, số câu hỏi đang chờ đọc lại (xoá sau ≤48 h), số lời
  nhắc đã hẹn, trạng thái công tắc.

**Vào prompt.** `recall_memory` gọi `nepnho.ChoPrompt`: ≤12 fact theo hiệu lực × độ hợp với màn/ý định ×
độ trùng từ, ≤1200 rune, trong `<du_lieu nguon="tri-nho">`, mỗi fact gắn `[f<id>]`. Cùng các fact đó tăng
hạng trong `search_places`, chỉ khi gốc là scope=me; nhóm không bao giờ tới `nep_*` (sửa theo phản
biện) [P1-1].

**Xoá**
- **Tắt công tắc** (`PUT /me/nep/cai-dat`): `nepnho.Xoa(tx, person)` xoá mọi hàng trí nhớ, kể cả
  tombstone, cùng transaction, và gọi `nepnhac.BoTheoTriNho(tx, person)`.
- **Xoá tài khoản: một trigger Go** (sửa theo phản biện) [P1-14][P2-8].
  - `AFTER UPDATE OF deleted_at ON people WHEN (OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL)`
    gọi `go_xoa_nguoi(id)`, hàm đọc `go_xoa_dang_ky(bang, cot, cach CHECK IN ('xoa','bo_ten','giu'),
    ly_do)`. Gói khác đăng ký bảng của mình bằng `nepnho.DangKyXoa(tx, bang, cot, cach, ly_do)` gọi từ
    `Migrate` của gói đó; `nepnho` vẫn là writer duy nhất của `go_xoa_dang_ky`.
  - Mọi cột người của bảng Go phải có câu trả lời: bảng mới của bản này, `claimed_by` của lát 14,
    `chat_ai_tin_hieu` của lát 18, và cột đang có như `chat_ai_invocations.person_id`,
    `chat_plan_promotions.created_by_id`.
  - Chạy ở cả đường xoá Go lẫn Python, vì cả hai đặt `deleted_at`. `repo/erasure.go` và bản đồ `ERASURE`
    của Python **không đổi**. `serve`/`work` từ chối khởi động khi trigger vắng.

**Công bố:** một thẻ một lần trong bảng («Bật» / «Để sau») và một dòng trong `app/settings/index.tsx`.
Câu chữ ở `nep/cong-bo.ts` với `CONG_BO_BAN`; `cong_bo_ban` ghi bản người đó đã đồng ý, làm bằng chứng
đồng ý; đổi câu là tăng bản và hỏi lại. Không trang ký ức, không toast. Văn bản ở ADR-0041 §6.

## 7. Nhắc chủ động `nepnhac` + push (lát 17)

```sql
nep_nhac(id uuid PK, person_id uuid NOT NULL, loai text NOT NULL CHECK (loai IN
  ('loi_hen','keo_mai_chua_chang','keo_sap_bat_dau','goi_y_cuoi_tuan')), chu_de text NOT NULL,
  chips jsonb NOT NULL, diem smallint NOT NULL, ngay_vn date NOT NULL, tao_luc, hien_tu, het_luc,
  da_xem_luc, bo_luc, bam_luc, day_luc, UNIQUE (person_id, loai, chu_de, ngay_vn));
nep_loi_hen(id uuid PK, person_id uuid NOT NULL, outing_id uuid, luc timestamptz NOT NULL,
  cau text NOT NULL CHECK (char_length(cau) <= 120), tao_luc timestamptz NOT NULL, xong_luc timestamptz);
```

- **Câu nhắc là mẫu cố định theo `loai`, không do model viết, không được lưu** (tự phát hiện): `nep_nhac`
  không có cột `cau`, kèo chỉ nêu qua id trong chip. Riêng `nep_loi_hen.cau` là lời chính người đó dặn
  (`TextSafe` 120, nằm trong `hoi`), chỉ hiện trong app.
- **`nepnhac.XetNguoi(person, now)` đọc** kèo đang hoạt động của chính người đó trong 36 h (`outings`,
  `outing_stops`, `memberships`, không cột ngân sách), `nep_loi_hen`, `nep_cai_dat`, và `nepnho.ChoNhac` +
  catalogue cho `goi_y_cuoi_tuan`: lần đọc duy nhất khi người đó không hỏi, chỉ khi họ đã bật `nhac`.
- **Chấm điểm** = ưu tiên × đúng lúc × (1 − mỏi), mỏi = min(1, 0.25 × số lần bỏ qua loại đó trong 14
  ngày). Dưới 30 thì không hiện.

| Ứng viên | Ưu tiên | Điều kiện |
|---|---|---|
| Lời hẹn nhắc (`loi_hen`) | 100 | tới `luc` |
| Kèo ngày mai chưa có chặng | 70 | — |
| Chặng đầu bắt đầu trong 60 phút | 60 | cửa sổ 45–75 phút |
| Gợi ý cuối tuần từ trí nhớ × catalogue | 30 | `nho` bật, ≥2 quán khớp, chỉ thứ Năm–Sáu |

- **Khi chạy:** `jobs.DinhKy{"nep-nhac", 15 phút}` cho người đã bật `nhac` có kèo hoặc lời hẹn trong
  36 h; đồng bộ (≤300 ms) khi `GET /me/nep/viec`. `UNIQUE` làm việc ghi idempotent.
- **Trong app, gác bởi `nhac`** (sửa theo phản biện) [P1-23]: `GET /me/nep/viec` →
  `{viec:[{id,loai,cau,chips,het_luc}]}`, `nhac` tắt thì rỗng; `POST /me/nep/viec/{id}/{da-xem|bo|bam}`.
  `NepProvider` gọi khi ra tiền cảnh và khi đổi route (cách ≥60 s), rồi gửi `bao-viec`/`xong-viec`.
  ADR-0035 không đổi: Nếp không nở rộng, `hienToSau` giấu tờ thứ hai ở màn tiền.
- **Push: bản tối thiểu của ADR-0024 bằng Go, đúng tên và schema ADR-0024** (sửa theo phản biện) [P1-21].
  - `notifications(id, person_id, kind, actor_id, context_id, subject_type, subject_id, created_at,
    read_at, pushed_at)`; CHECK `kind` chỉ liệt kê kind đã có writer, hôm nay là `nep_nhac`;
    `subject_type='nep_nhac'`, `subject_id = nep_nhac.id`.
  - `notification_devices(id, person_id, platform, expo_push_token UNIQUE, installation_id uuid NOT NULL,
    created_at, last_seen_at)`.
  - `push_viec`: bảng việc (hàng `notify`) trỏ tới `notifications.id`, để `notifications` giữ đúng schema.
- **Không chiếm token của máy khác** (sửa theo phản biện) [P1-22]. `POST /devices {platform,
  expo_push_token, installation_id}`; `installation_id` là UUID ngẫu nhiên máy tự sinh, giữ trong kho an
  toàn. Token trùng **cùng** `installation_id` thì đổi chủ được (đăng xuất rồi vào tài khoản khác trên
  cùng máy); khác thì 409 `thiet_bi_khac`. `DELETE /devices/{id}` khi đăng xuất hoặc tắt push.
- **Sender:** `LogSender` chỉ ghi số lượng; `ExpoSender` gọi exp.host, 5 s, lô ≤100, `DeviceNotRegistered`
  thì xoá thiết bị; `MOBILE_PUSH_MODE=log|expo`, giá trị lạ thì từ chối khởi động. Consumer `notify` gửi
  rồi đặt `pushed_at`. **Payload:** tiêu đề «Nếp», một câu đóng theo `loai` («Kèo ngày mai chưa có chặng
  nào.»), `data={kind:"nep_nhac", id, route}`; **không bao giờ** có tiêu đề kèo, tên quán, chữ lời hẹn.
- **Client:** `expo-notifications` cần dựng lại native. Chỉ xin quyền khi bật «Nếp nhắc qua thông báo»;
  lấy token hỏng thì nói «Thông báo đẩy chưa bật trên bản này» (ADR-0024 §2.4). Chạm thì kiểm lại route
  (`lo-trinh`, `nepPhaiLui`); không hiện banner khi đang ở màn tiền.

## 8. Giới hạn và ngân sách

| Mục | Trần |
|---|---|
| Lời gọi model | một lượt: `MaxModelCallsPerTurn` (chung), agent ≤3 bước, hết thì `ai_het_ngan_sach`; `nep.trich`: 1 lời gọi mỗi job, ≤20 câu |
| `what_you_remember`, từ chối tiền, màn tiền | 0 lời gọi; `xong` p95 ≤500 ms |
| Latency (theo hợp đồng chung) [P2-9] | trạng thái đầu p95 ≤300 ms; chữ đầu p50 ≤2.5 s, p95 ≤5 s |
| Tool | `search_places` 3 s; `my_upcoming_outings` 1.5 s; `recall_memory` 1 s; `huongdan.Tim` ≤50 ms |
| Trí nhớ, sự kiện | 200 fact; `cau` ≤160; `dieu_da_dan` ≤120; prompt ≤12 fact, ≤1200 rune; sự kiện ≤50/lô, 1 lô/10 s, giữ 30 ngày |
| Nhắc | trong app ≤3/ngày; push ≤1/ngày, giờ yên 21:30–08:00 giờ VN, không push nếu 14 ngày không mở app; 72 h mỗi `loai` (trừ `loi_hen`), 24 h mỗi `chu_de`; bỏ qua 2 lần liền thì tắt loại đó 14 ngày |
| DB | tool và `nepnhac` dùng pool riêng của worker, semaphore ≤ pool/2 [P1-17][P2-11] |

## 9. Hỏng hóc và cách chịu

| Hỏng | Cách chịu |
|---|---|
| SSE hỏng; đóng bảng giữa chừng | rơi về polling (thiết kế 02); task còn trong `NepProvider`, `xong-viec` báo, giấu ở màn tiền |
| Tắt `nho` lúc đang trích; «quên» đua với củng cố | cùng khoá `nepnho:<person>`; đọc lại `nho` và tombstone trong khoá; tắt thì xoá hàng, Ack |
| Model trích trả op sai | Go bỏ op; không op nào tới DB mà chưa qua `kiemSuThat` |
| Sổ tay lệch UI; `banBuild` lệch | CI đỏ; lúc chạy bỏ «» cho nhãn lạ, không phát `chi`/`mo_to` |
| Expo sập | `push_viec` thử lại có backoff; quá `het_luc` thì bỏ |
| Trigger xoá tài khoản vắng | `serve`/`work` từ chối khởi động |

## 10. Cổng, test, canary, đột biến

**Cổng cấu trúc**
- Node: `nep-phieu-kin` quét cả `src/` lẫn `app/`, kể cả khoá lồng dưới `thay`; `nep-man-khai` (mỗi route
  khai đúng một lần); `nep-lo-trinh`, `nep-goi-y`, `nep-chu-gon`, `nep-luong`, `huong-dan-khop-ma`;
  `cau-chu-goi-ai` có câu cho mã mới (`thiet_bi_khac`, `nep_su_kien_qua_nhanh`).
- Cổng đọc xuyên gói của lát 5, allowlist theo gốc (sửa theo phản biện) [P1-13][P2-5]:
  - gốc Nếp hiện có: `chat_ai_invocations`, `account_sessions`, `people`, `job_outbox`; lát 13 thêm `places`;
  - `my_upcoming_outings`, `nepnhac.XetNguoi`: `outings`, `outing_stops`, `memberships`;
  - `nepnho.*`: chỉ `nep_*`; `push.*`: chỉ ba bảng của nó;
  - cấm mọi gốc: `messages`, bảng tiền của `accountlifecycle`, `chat_v2_%`, `person_interests`,
    `saved_places`, cột `budget*`/`_vnd`.
- `quyen.golden.json`: hàng Nếp bật đúng ở lát thi công (13, 15, 17).
- Test Go: `kiemPhieu` v2, `kiemSuThat`, suy giảm, fact thay trong tương lai; giờ yên lúc 21:29, 21:30,
  07:59, 08:00 giờ VN; mô phỏng nhắc 1.000 người-ngày, tất định trong CI.
- Tầng Postgres (`go_postgres_tier.sh`, skip là đỏ): tắt `nho` và xoá tài khoản mỗi cái để lại 0 hàng
  trong mọi bảng đã đăng ký; test liệt kê mọi cột `person_id`/`*_by` của bảng Go (`information_schema`,
  trừ bảng có trong models Python), cột chưa đăng ký là đỏ; index một fact hiện hành; củng cố chạy lại
  idempotent; tombstone chặn trích lại; token khác `installation_id` → 409.

**Canary** (đỏ đúng chỗ dự đoán, ca identity xanh)
1. Khoá phiếu `tenNguoi` đỏ ở cổng TS **và** 400 ở Go; phiếu v2 hợp lệ thì xanh.
2. Fact đã seed không có trong request nào của nhóm (bộ ghi request của thiết kế 01) [P1-1].
3. Op trích `gioi_tinh` bị từ chối; «Lan dị ứng tôm» bị từ chối; «mình thích đi sớm» được ghi.
4. Payload push mang tiêu đề kèo làm test payload đỏ. Push lúc 22:00 bị chặn; lúc 08:00 đi.
5. Sau «quên», tìm chữ đã quên trên mọi bảng `nep_*` ra 0 hàng.

**Đột biến tự nghĩ** (kiểm tương đương trước; mỗi cái đỏ đúng bước dự đoán)

| Lát | Đột biến | Đỏ ở |
|---|---|---|
| 1 | bỏ `!duocHoi` khỏi «Vẽ» | test «Vẽ» ở màn tiền |
| 1 | `nguonAnhNep` bỏ header | ca header của `nep-media` |
| 13 | bỏ `app/` khỏi phạm vi quét | ca phủ, tại `app/(tabs)/explore.tsx` |
| 13 | `chi` không kiểm `hanhDong` ⊆ phiếu | ca chip ngoài màn |
| 15 | bỏ kiểm `nho` ở ingest | ca Postgres tắt công tắc |
| 15 | «quên» dùng UPDATE thay DELETE | canary 5 |
| 15 | cho gốc nhóm gọi `recall_memory` | canary 2 |
| 17 | `>=` thành `>` ở cuối giờ yên | ca 08:00 |
| 17 | bỏ gác `nhac` ở `/me/nep/viec` | ca opt-in |
| 17 | đổi chủ token không kiểm `installation_id` | ca chiếm token |

**Bộ đo:** một binary eval trên `Engine.Run` (thiết kế 06), không có `cmd/nep-eval` riêng (sửa theo phản
biện) [P1-16][P2-17]. Lời gọi thật cần Lead duyệt số lượng từng lần (ADR-0034 §2.6).

| Bộ ca | Cỡ | Mục tiêu |
|---|---|---|
| Hỏi cách làm | 30 | đúng bước ≥0.85, bịa UI ≤2%, chip hợp lệ 100% (M3) |
| Màn tiền | 10 | từ chối 100%, 0 lời gọi |
| Câu có nhãn để trích | 40 | precision ≥0.90, recall ≥0.70 |
| Đối kháng (nhạy cảm, tiền, người khác) | 20 | 0 được nhận |
| Nhớ / quên / «bạn nhớ gì» | 10 | nhớ và thay ≥0.90; quên 100% trong store |
| Mô phỏng nhắc | 1.000 người-ngày | 0 vi phạm trần hay giờ yên |

**Bằng chứng UI:** Maestro `49-nep-hoi-dap.yaml`, `50-nep-viec.yaml`; `tools/xem-dock-nep.mjs` lấy tờ
thứ hai từ `/me/nep/viec` thật, vẫn ≤14 dp; ảnh sáng, tối, Reduce Motion được mở ra nhìn; số đo viết
vào commit message.

## 11. Lát cắt, theo số của kế hoạch

| Lát | Phần Nếp | Phụ thuộc |
|---|---|---|
| 1 | «Vẽ» kiểm `!duocHoi`; `/me/nep/media` nhận `man`, 403 `nep_lui_man_tien` ở màn tiền (ngoại lệ bảo mật có tên cho Python, nói trong commit); `nguonAnhNep` dùng `headerNguoiGoi` | — |
| 5 | allowlist gốc Nếp trong cổng đọc xuyên gói, canary method value | 4 |
| 6 | Nếp qua `Engine.Run`, prompt `nep_agent.txt`, định tuyến tất định, chưa stream | 3, 4, 5 |
| 11 | SSE cho Nếp, task trong `NepProvider`, bảng mới, «Mực chờ», sửa `DESIGN.md` | 10 |
| 13 | phiếu v2 cho 7 màn, sổ tay, chip, `my_upcoming_outings`, sửa gợi ý | 8, 9 |
| 15 | `nepnho`, công tắc, công bố, sự kiện, trích, củng cố, trigger xoá | 10 (hàng `memory`, `DinhKy`), 13 |
| 17 | `nepnhac` trong app, rồi `push` | 15, FCM credentials |
| 18, 19 | bộ ca Nếp trong binary eval chung; rồi gỡ `nep-reply` cùng manifest (ADR-0037) | thiết kế 06 |

## 12. Quyết định còn mở

1. `dieu_da_dan` có giữ dị ứng hay ăn kiêng do chính người dùng nói không? Đề xuất: **không**, tới khi
   Lead chốt, vì đó là dữ liệu sức khoẻ, nhóm nhạy cảm theo Nghị định 13/2023.
2. Parity: bảng `nep_*`, `notifications` và trigger trên `people` dựng ở cả hai stack, hay loại khỏi phép
   so hàng?
3. Giờ yên chỉ áp cho push; trong app là kéo (chỉ hiện khi người đó tự mở app). Cần Lead xác nhận.
4. Cài lại app mà token trùng `installation_id` cũ: đợi `DeviceNotRegistered`, hay nhả hàng sau 60 ngày?
5. Tiêu đề kèo trong phiếu `outings/[id]` (có từ v1) có thể mang tên người do nhóm đặt: giữ, hay chỉ gửi id?
6. FCM credentials, `projectId` (lát 17; iOS ngoài phạm vi); câu công bố nêu tên nhà cung cấp mô hình.
7. Hai sửa cho tài liệu mảng khác: thêm ý định `remind` vào thiết kế 01 §3.3; thêm `nepnhac` vào danh
   sách gói có bảng version riêng trong hợp đồng chung.

## 13. Bằng chứng đã xem

Kế hoạch, quyết định đã chốt, bản Nếp gốc, hai phản biện, thiết kế 01–03, hợp đồng chung, ADR-0024/0033–0036;
mã tại `f251db7` (mọi file:line ở mục 0 đã mở ra đọc). Chưa có bằng chứng hành vi: đây là thiết kế.
