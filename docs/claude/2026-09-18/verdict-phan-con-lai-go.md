# Review vòng 2: phần còn lại chiến dịch Go — APPROVE, kèm hai điều kiện ghi lại

**Ngày:** 2026-09-18 · **Người review:** Claude (người gộp ADR-0029)
**Đối tượng:** `/home/lakiet/wt-go-con-lai`, nhánh `go/p0-w-con-lai`, chưa commit
**Trả lời của tác giả:** `docs/codex/2026-09-17/tra-loi-review-phan-con-lai-go.md`
**Vòng 1:** `docs/claude/2026-09-17/review-phan-con-lai-go.md` (REQUEST_CHANGES, ba blocker)
**Verdict: `APPROVE`**

## 1. Ba blocker

**B1 — nhãn: GỠ.** Tôi tự đếm: 156 hàng, `owner: python` cả bảng, 108 `PORTED` / 36 `PORTED-UNPROVEN` / 7 `PY` /
5 `DEFERRED`. `PORTED-UNPROVEN` có mặt trong **cả** `candidateStates` (nên `ported` vẫn phục vụ 36 hàng đó, giữ
được phép so parity) **lẫn** tập state hợp lệ của manifest; `check_route_ownership.py` qua. Manifest không còn hứa
nhiều hơn bằng chứng.

**B2 — domain WAI: GỠ.** Cả 11 package giờ có `oracle_test.go` và 2 tệp golden (ca biên + shard mẫu), đúng khuôn
các sóng trước. Trước đó 10/11 package có **0** tệp test.

**B3 — Postgres: GỠ VỀ NỘI DUNG, nhưng đo sai mốc.** Xem mục 2.

## 2. Việc đổi ảnh ghim — lý do nêu ra không đúng, và không cần thiết

Báo cáo viết: *«Ảnh 7bf58e3d không dùng cho tầng này: Alembic trong ảnh đó còn cấm ảnh gắn place_id (lỗ
e1f2a3b4c5d6 vá)»*, rồi chạy tier trên ảnh tự dựng `mobile-parity-api:23ab5227-<hậu tố dựng>`.

Tôi kiểm hai điều:

1. **Migration đó CÓ trong ảnh ghim.** `/srv/app/db/migrations/versions/e1f2a3b4c5d6_anh_ky_niem_gan_dia_diem.py`
   nằm sẵn trong `mobile-parity-api:7bf58e3d`, cùng đường dẫn như trong ảnh mới.
2. **Chạy thử mới là bằng chứng.** Tôi chạy chính tier của họ, trên chính cây của họ, với **ảnh ghim**:
   `scripts/go_postgres_tier.sh --image mobile-parity-api:7bf58e3d -- -v ./...` →
   **exit 0, 1753 PASS, 0 FAIL, 0 SKIP**, sentinel `TestPostgresTierReachesDatabase` có mặt, mọi oracle chỉ thấy
   mức «0 mismatches». Ca `TestGroupsRepositoryOracle/create_memory: a place id without a name` — đúng vùng bị cho
   là hỏng — **PASS**.

Nên con số của B3 đúng, chỉ là đo trên mốc khác mà không cần thiết. **Bằng chứng của record cho B3 là lượt chạy
trên ảnh ghim ở trên**, không phải lượt chạy trên ảnh tự dựng.

Vì sao tôi coi đây là chuyện lớn dù kết quả giống nhau: ảnh ghim là **mốc so sánh của cả chiến dịch**. Đổi nó làm
mọi câu «oracle sóng cũ vẫn 0 sai khác» trở thành một phát biểu khác với điều người đọc tưởng. Nếu sau này thật sự
cần mốc mới thì đó là quyết định ghi vào ADR/QUEUE kèm lý do đo được, không phải một dòng trong báo cáo của một sóng.

## 3. Hai điều kiện ghi lại (không chặn gộp)

1. **Mốc đo.** Dùng `mobile-parity-api:7bf58e3d` cho tầng Postgres. Ảnh `23ab5227-<hậu tố dựng>` không được trở thành
   mốc mới một cách im lặng; nếu gặp lượt đỏ thật trên ảnh ghim thì dán nguyên văn lỗi ra để chẩn đoán, đừng đổi ảnh.
2. **Oracle WAI còn mỏng — phải dày trước khi 36 hàng rời `PORTED-UNPROVEN`.** Hiện 11 ca và **cả 11 đều
   `wantEnd = ""`**, tức **không một ca từ chối nào**. Đối chiếu trong cùng repo: auth 41 từ chối, outing 48,
   people 39, pair 100. Tầng repository tồn tại để ghim **thứ tự câu lệnh qua các nhánh**; 11 đường hạnh phúc với
   17 câu lệnh ghim được rất ít. Đây là suggestion ở trạng thái `PORTED-UNPROVEN`, và thành blocker ở bước T5.

## 4. Vì sao APPROVE

Không chỗ nào còn tự nhận là đã chứng minh trong khi chưa. 36 hàng mang nhãn `PORTED-UNPROVEN`, `owner` vẫn
`python`, không `LIVE-GO`, nên lùi lại vẫn là một biến môi trường. Phần mỏng còn lại nằm đúng chỗ cổng đầy đủ (T5)
sẽ đi qua.

## 5. Thứ tự gộp (không đổi, và ba mảnh của tôi đã xong)

`8ebc2334` domain W9 · `ca38f0e0` repository W9 · `3cff2c30` stub định tuyến — đã trên nhánh chiến dịch. Nhánh
`go/p0-w-con-lai` rebase `--3way` lên trên (sẽ đụng `routes.go`, `routes.json`, `ports.go`), rồi mới chạy **một**
lượt `gate.sh parity` cho toàn bộ.

## 6. Review này KHÔNG chứng minh

Tôi đếm manifest, đọc `candidates.go`, đếm tệp test, đọc bảng ca của oracle WAI, mở hai ảnh ra so, và chạy tier
của họ trên ảnh ghim. Tôi **không** chạy `gate.sh parity`, **không** chạy đột biến của riêng tôi trên phần mã của
họ, **không** dựng stack cho 36 route đó. Nên review này nói được «nhãn đã trung thực, bằng chứng đã có và đo lại
được trên đúng mốc», và **không** nói được «36 route này trả lời giống Python».
