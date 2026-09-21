# PR #614 — phép đo độc lập và hai lỗ cổng nó tìm ra

**Tree đo:** `40b9050b` (nhánh `claude/p0-w-hn-3-so-hai-nguoi-be`), rồi hai commit sửa
ở trên nó. **Người đo:** agy (Antigravity/Gemini), một lượt uỷ quyền, ~20 phút, không
đọc nhánh nào khác. **Người xác minh lại từng phát hiện:** Claude, trong cùng cây, bằng
tay. **Điều kiện ADR-0016 §2.3 số 4** («không tự review; agy test trước merge») là lý do
lượt đo này tồn tại.

## Số liệu agy trả về, và số liệu tôi chạy lại

| Phép đo | agy | Tôi chạy lại |
|---|---|---|
| `tests/postgres` với hai biến môi trường thật | 682 passed, 154 s | 683 passed, 137 s (sau khi thêm ca mới) |
| `pytest services/api/tests tests` ở gốc | 3528 passed, 745 skipped, 5496 subtests | xem cuối trang |

Hai biến `MOBILE_TEST_DATABASE_URL` và `MOBILE_REQUIRE_POSTGRES_TESTS` là bắt buộc trong
lệnh giao cho agy, vì thiếu chúng thì 682 ca **SKIP** và skip đọc thành pass. Dòng tổng
của agy có hàng trăm `passed` chứ không phải `skipped`, nên tầng ấy đã chạy thật.

## Bốn phép đột biến, và hai cái sống sót

Mỗi phép: sửa một dòng sản phẩm, chạy lại tầng test tương ứng, rồi hoàn nguyên.

| Phép | Đột biến | agy | Tôi xác minh |
|---|---|---|---|
| a | xoá trigger `pair_reject_sent_version_change` khỏi migration | ĐỎ, 3 failed | — (đủ tin, có ca đỏ tên rõ) |
| b | `hieu_luc` trả `chot` thay vì `het_han` khi quá hạn | ĐỎ, 6 failed | — |
| c | bỏ `version_current` khỏi `requires` của `respond_pair_paper` | **SỐNG SÓT** | **tái hiện**: `tests/domain` 1072 xanh, và cả `tests/domain tests/api` 2320 xanh |
| d | dời `pair_notebooks` từ nhánh `keep` sang `delete` | **SỐNG SÓT** | **tái hiện, và tệ hơn báo cáo**: xanh cả 682 ca Postgres |

Ba trigger còn lại chưa ai thử. Đó là **chưa quét**, không phải đã chứng minh.

## Phát hiện c — bảng quyền tự kiểm chính nó

`test_thieu_bat_ky_vi_tu_nao_la_dong` đọc `permissions._TABLE[action]["requires"]` để biết
phải thử thiếu cái gì. Xoá một vị từ khỏi bảng thì vòng lặp **ngắn đi**, và ca vẫn xanh.
Danh sách và thân hàm lái là hai hướng trôi dạt; ở đây chúng là **một** hướng, nên không
ai cãi lại được khi bảng đổi.

**Sửa:** `TestPairNotebookDoors.VI_TU` — tập vị từ của từng cửa, viết tay, độc lập với
`_TABLE`, cộng một ca đối chiếu hai chiều (một cửa `pair` mới mà không ai ghi vào danh
sách cũng đỏ). Ghim **tập**, không ghim thứ tự: thứ tự quyết định câu từ chối nào được kể
khi thiếu hai vị từ cùng lúc, và đó là chuyện câu chữ, có ca riêng.

**Đối chứng:** đúng đột biến c làm ca mới đỏ, và cây sạch xanh lại.

## Phát hiện d — bản đồ xoá không phải thứ điều khiển việc xoá

Đây là chỗ báo cáo của agy còn nhẹ hơn sự thật. Tôi dựng một ca Postgres mới xoá một tài
khoản trong một cuốn sổ thật và đếm từng bảng — **ca ấy vẫn xanh dưới đột biến d**. Lý do:
`erase_person` **không đọc** `ERASURE`. Nó có một danh sách `wipe()` viết tay riêng. Bản đồ
là lời tuyên bố chính sách; đoạn mã là việc thật; và không một câu nào trong cây bắt chúng
khớp nhau.

Hai hướng trôi dạt có hai hậu quả khác nhau:

- một bảng ở nhánh `delete` mà mã không xoá là **lời hứa bị vỡ** — người ta bấm xoá tài
  khoản và dữ liệu ở lại;
- một bảng ở nhánh `keep` mà mã có xoá là **ký ức của người khác bị lấy mất** — cuốn sổ
  hai người là của cả hai.

**Sửa:** `tests/db/test_erasure_map_matches_the_code.py` đọc `SqlAlchemyApiRepository.
erase_person` bằng AST, lấy tên bảng của mọi `wipe(Model, …)`, và so hai chiều với
`ERASURE["delete"]`. Không cần database. Ba chi tiết đáng ghi:

- Nó nhắm **lớp hiện thực**, không phải Protocol: hai hàm cùng tên `erase_person` trong
  `repository.py`, và cái đầu tiên là khai báo `...`. Bản đầu của cổng đọc trúng nó, thấy
  không có `wipe(` nào, và **im lặng đồng ý với mọi bản đồ**.
- Nó đòi tham số đầu của `wipe()` là **tên model viết thẳng**; một bí danh làm cổng mù
  đúng chỗ nó sinh ra để nhìn.
- Ca thứ ba đếm `wipe(` trong nguồn và so với số lời gọi AST đọc được, nên «AST bỏ sót»
  không thể đọc thành «mã đúng».

**Đối chứng hai chiều:** dời một bảng `keep → delete` làm ca thứ nhất đỏ; dời một bảng
`delete → keep` làm ca thứ hai đỏ. Cây sạch xanh cả ba.

## Ca Postgres mới đi kèm

`test_the_notebook_of_two_survives_one_of_them_ending` dựng một cuốn sổ thật — hai tư cách
thành viên, một chu kỳ, hai bậc đồng ý, một tờ giấy đã chốt, một kèo liên kết, một dòng
giữ lại — rồi kết thúc một tài khoản và đếm: **mười bảng lịch sử còn nguyên số hàng, ba
bảng riêng của người ấy về 0, và hàng của người còn lại trong ba bảng ấy không bị chạm.**
Nó không thay thế cổng AST ở trên (nó mù với đột biến d) nhưng nó là câu duy nhất nói được
hậu quả thật, và ba trigger đã chặn ba lần trong lúc tôi viết nó: T4 đòi roster, T3 đòi cả
hai đồng ý `bat_doi`, T2 đòi tờ giấy đang `chot` lúc hàng liên kết được ghi.

## Ghi chú về agy như một người đo

Hai lượt đầu qua `agent_supervisor` (32 phút) ra **một file scaffold và một `verdict.md`
ghi «init»** — supervisor tự gắn cờ «bao THANH CONG nhung KHONG RA SAN PHAM NAO». Lượt
thành công là lượt thứ ba, giao bằng một lệnh **cơ học**: bốn bước đánh số, mỗi bước một
lệnh, và yêu cầu ghi **con số** chứ không phải nhận xét. Bài học lặp lại của ADR-0010: một
digest «GREEN» không phải bằng chứng; thứ có giá trị là con số và phép đột biến, và cách
hỏi quyết định có lấy được chúng hay không.
