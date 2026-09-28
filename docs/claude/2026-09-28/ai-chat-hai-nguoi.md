# Rủ Đi AI trong chat hai người — thiết kế lát

- Ngày: 2026-09-28 · nhánh `claude/peaceful-hopper-32kwjs` (làm lại từ `main` `04a086f9` sau khi PR #654 merge)
- Căn cứ: ADR-0046 §8.1 (chủ sản phẩm chốt 2026-09-27: flow kiểu Meta AI cho 1:1 và nhóm), ADR-0027 §4,
  ADR-0034 §2.1, ADR-0023 (chặn), ADR-0046 §4, §8.3, §9–10.
- Verdict: chưa có (chưa có reviewer thật).

## 1. Hiện trạng (khảo sát chỉ đọc)

- Chat 2 người (`contexts.kind='pair'`, `pair_key`) và nhóm dùng **chung** màn `GroupChatLive` và chung
  lane legacy (không E2EE). Chưa có dòng `chat_v2_conversations` nào ở production cho cặp.
- Máy chủ chặn cặp ở 6 chỗ trên đường gọi AI: `chatassist/handler.go` (preflight, capabilities, create,
  aiStream), `sse.go` (events), `worker.go` (prepare, publish), `nhom_engine.go` (chuanBiNhom).
- App chặn ở 2 chỗ: `GroupChatLive.tsx` (`nhanRieng ? null : timNhacAi`), cộng chữ viết cho «cả nhóm».

> **Bị thay một phần (2026-09-28, sau):** chủ sản phẩm chốt hai class «đám bạn» / «cặp đôi» — xem
> mục cuối «Quyết định 2026-09-28 (sau): hai class» và ADR-0046 §8.4. Các điểm 2.1, 2.5 và hàng S1
> «`plan`/`chia_bill` → 409» dưới đây **không còn đúng**; giữ nguyên để đọc lịch sử. Điểm 2.2, 2.3,
> 2.4 (phần «không đọc gu»), 2.7 vẫn đúng; 2.6 được thay bằng «cặp chỉ chạy trên engine Go».

## 2. Quyết định

1. **[Đã thay] Cặp chỉ có lệnh `hoi`** (gắn `@Rủ Đi` hỏi trong luồng). `plan`, `chia_bill`, bản nháp chung và
   «thành kèo» vẫn chỉ cho nhóm: ADR-0046 §9 buộc `chia_bill` vào nhóm, ngoại lệ đọc `body` (§8.3) không
   mở rộng sang cặp. Capabilities của cặp báo `hoi`+`mention` bật, `plan`/`chia_bill` tắt với lý do
   `group_plan_only`.
2. **Không migration.** `scope='group'` ở `chat_ai_invocations` nghĩa là «phòng chat» (nhóm hoặc cặp);
   CHECK hiện có (`scope='group' AND command IN (plan,chia_bill,hoi)`) và hàng đợi `ai.group` dùng
   nguyên. Loại phòng đọc lại từ `contexts.kind` mỗi lần, không lưu thêm.
3. **Chặn/xoá tài khoản**: cặp mà người kia đã xoá tài khoản hoặc một trong hai đã chặn (friend_requests
   `blocked`) thì từ chối — cùng luật với `chatlegacychange/store.go` `authorize`. Kiểm ở preflight
   **và** lại ở worker (prepare + publish), vì chặn có thể xảy ra khi lượt đang chạy.
4. **AI chỉ đọc gói người gọi kèm** (chip «Kèm {n} tin gần đây · Xem · Chỉ gửi lời nhờ»), không tự đọc
   lịch sử, không đọc gu. `chia_gu` (ADR-0034) **chưa** dùng ở lát này — khi cần gợi ý theo gu chung thì
   làm lát riêng, gác theo công tắc của từng người.
5. **[Đã thay] Công cụ**: bot cặp có bảng quyền riêng `ChoCap` = công cụ chung, **không** có `draft_poll`,
   `group_snapshot`, `list_group_outings`. Golden `quyen.golden.json` thêm bot `cap`.
6. **Engine**: dùng đường nhóm (`MOBILE_AI_ENGINE_GROUP`) với roster 2 người; cờ vẫn mặc định `brain`.
   Đường brain Python cũ: cặp gửi `hoi` như nhóm (không sửa Python — kiểm xem brain có chặn kind không;
   nếu có thì cặp chỉ chạy được khi cờ `go`, ghi rõ).
7. **Mọi cổng aigate giữ nguyên**: không đọc `body` ngoài chỗ ghim `chia_bill`, cặp không chạm trí nhớ
   Nếp, trigger/outbox không có chữ tự do, SSE chỉ qua cửa sổ guard.

## 3. Lát

| Lát | Nội dung | Bằng chứng |
|---|---|---|
| S1 máy chủ | mở 6 cổng cho cặp chỉ với `hoi`; chặn/xoá tài khoản; `ChoCap`; capabilities | test Postgres: cặp gọi `hoi` → 202, stream tới người kia; `plan`/`chia_bill` → 409; chặn → 403 ở preflight và giữa lượt; người ngoài cặp → 403; aigate xanh; ≥2 đột biến |
| M1 app | bỏ 2 cổng `nhanRieng`; chữ cho hai người; lối vào AI trong khay của cặp; sổ tay | test node: cặp hiện chip và gửi lời nhờ; chữ không có «nhóm» ở cặp; `_rut.json` rút lại; Maestro flow cặp (chưa chạy được ở cloud) |

## 4. Còn mở
- Cặp chuyển sang v2 (E2EE) sau này: endpoint sẽ từ chối `encrypted_invocation_required` như nhóm v2.
- Gợi ý theo gu chung (`chia_gu`) — lát riêng.
- Maestro trên máy thật, ảnh chụp.

## Quyết định 2026-09-28 (sau): hai class

Chủ sản phẩm chốt cùng ngày, thay điểm 2.1 và 2.5 (quyết định của chủ sản phẩm, không phải chữ ký
Lead; ghi ở ADR-0046 §8.4):

- **Đám bạn** = mọi chat nhóm **và** mọi chat hai người thường (`kind='pair'`): Rủ Đi AI y hệt nhóm —
  `hoi`, `/plan`, `/chia-bill`, tờ hẹn chung, «thành kèo». Khảo sát: không tính năng nhóm nào đòi ≥3
  người; không cần migration.
- **Cặp đôi** = chat hai người mà cả hai đã bật «Một đôi» (`bat_doi`, `pairnotebook.CanBatDoi`): như
  đám bạn, cộng phần riêng làm sau (gu đã chia xin đồng ý lại theo ADR-0048, prompt xưng hô cặp đôi).
- Hợp đồng: `chat-capabilities` thêm trường gốc `cap_doi` (true chỉ khi `pair` và `CanBatDoi`).

Lát P1 (máy chủ) làm:

- Gỡ `lenhChoPhong` (preflight, create, worker); capabilities của cặp như nhóm; tờ hẹn và «thành kèo»
  nhận cặp qua `phongAi` (sửa, bỏ tờ hẹn cũng kiểm chặn/xoá/rời); `chuDaLuu` đọc cho cặp lane legacy
  (ngoại lệ ADR-0046 §8.3 nay nói rõ gồm chat hai người của đám bạn).
- Gỡ `tools.BotCap`/`ChoCap`, khoá `cap` trong `quyen.golden.json`, `cau.CapKhongChamTien` và nhánh tiền
  riêng của cặp: lượt của cặp đi `ChoNhom` và đường `nhapChiaBill` như nhóm.
- `Turn.Cap` đổi thành `Turn.Doi` (tính bằng `laDoi` khi worker đọc phòng; chưa gì dùng).
- Giữ `capConMo`. Cặp chỉ chạy trên engine Go: brain không có đường cho cặp, nên capabilities báo
  `provider_unavailable`, route từ chối `503`, worker brain làm thất bại cùng mã mà không hỏi brain.
