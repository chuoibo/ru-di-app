# `scripts/archive/` — bảng đột biến đã dùng xong

Mỗi file ở đây là một **bảng đột biến một-lần**: nó sửa mã đã ship đúng một chỗ,
chạy một cổng, khôi phục cây, rồi in ĐỎ/XANH. Chúng là *bằng chứng* rằng một
cổng thật sự cắn ở thời điểm đó — không phải cổng đang chạy.

Không Makefile, workflow hay test nào gọi chúng. Nơi duy nhất còn nhắc tên
chúng là các phán quyết QA trong `docs/archive/`. Để ở `scripts/` thì thư mục
đó trông như 117 công cụ đang sống, trong khi phần lớn đã xong việc.

Giữ lại chứ không xoá vì một bảng đột biến là thứ khó dựng lại: nó ghi **đúng
mutation nào làm cổng đỏ**, và đó là câu trả lời cho "cổng này có mù không".

Ba file cùng loại **không** nằm ở đây vì còn caller thật trong `tests/qa/`:
`scripts/mutation_cong_cua_so_model.py`, `scripts/mutation_rd_do_f22.py`,
`scripts/qc/repro_bill_mo_gemini_bia_mon.py`.
