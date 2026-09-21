# ADR-0030 — Lane Codex đóng; Claude sở hữu toàn bộ backend

**Trạng thái:** Chấp nhận · **Quyết định bởi:** leader, 2026-09-16 · **Hiện thực:** Claude
**Liên quan:** ADR-0007 (review sống trên PR), ADR-0010 (QC bằng agy), ADR-0016 §2.3, ADR-0029 (chuyển lõi sang Go)

## 1. Bối cảnh

Từ 2026-08-27 repo chạy hai lane: Claude giữ `app/web/` và `apps/mobile/`, Codex giữ `db/`, `api/`, `domain/` và
test backend. ADR-0007 đòi review sống trên PR và cấm tự review PR của mình; ADR-0010 giao QC hình ảnh và thăm dò cho
agy. Chiến dịch ADR-0029 (150 route từ FastAPI sang Go) chạy trong khuôn đó, và đã kẹt ở hai chỗ: nhóm `outings`
phải chờ lane Codex đồng ý đóng băng, còn agy không chạy được vì ADR-0010 §6.4 cấm cờ mà supervisor đang truyền.

Leader đóng lane Codex ngày 2026-09-16.

## 2. Quyết định

1. **Claude sở hữu toàn bộ backend** (`db/`, `api/`, `domain/`, test backend) từ 2026-09-16. Ranh giới sở hữu chốt
   2026-08-27 hết hiệu lực.
2. **`outings` không còn cần thoả thuận đóng băng với lane khác.** Việc đóng băng do chính người port ghi vào
   `docs/team/hang-doi.md` trước khi ghi mốc kịch bản, đúng luật đóng băng của ADR-0029.
3. **Không còn review chéo.** Cổng bằng chứng để một route đi tới LIVE-GO là, và chỉ là:
   - cổng parity đầy đủ chạy lại trong **cây sạch tại đúng SHA** (không phải cây của agent);
   - canary đỏ hết mọi kiểu phá được áp dụng, identity xanh, cùng SHA harness;
   - probe, và mọi lệch đều nằm trong danh sách ngoại lệ đã ghi;
   - **ít nhất hai đột biến do người gộp tự nghĩ**, khác với đột biến của agent, mỗi đột biến phải đỏ ở đúng bước
     đã dự đoán — và phải kiểm tương đương trước, vì một đột biến không đổi được kết quả thì «đỏ» hay «sống» đều vô
     nghĩa;
   - số đo viết thẳng vào commit message, không để chỉ nằm trong log.
   Digest của agent không phải bằng chứng; người gộp chạy lại.
4. **AGY-PASS gỡ khỏi thang trạng thái của ADR-0029.** 95 route đã PORTED chưa hề đi qua agy, nên giữ nó làm cổng bắt
   buộc lúc này là nói dối về cái đã xảy ra. Muốn bật lại agy thì mở ADR mới cùng cách chạy hợp ADR-0010 §6.4.

## 3. Hệ quả

- Đi nhanh hơn: không còn chờ thoả thuận giữa hai lane, không còn hàng đợi review.
- **Mất lớp người thứ hai.** Người viết code cũng là người gác. Bù lại bằng: đột biến độc lập của người gộp, canary,
  probe, replay chéo, làn DB và làn kho ảnh — đều là phép đo máy chạy được, không phải ý kiến.
- `docs/team/hang-doi.md` giữ tên cũ nhưng từ nay là nhật ký chiến dịch của một lane.

## 4. Cái này KHÔNG chứng minh

Không chứng minh rằng bỏ review chéo là an toàn. Nó chỉ nói rằng khi không còn lane thứ hai, phép đo máy thay chỗ cho
con mắt thứ hai, và phép đo đó phải chạy trong cây sạch thì mới tính. Những gì ADR-0029 §4 đã ghi là không chứng minh
được (dữ liệu thật, tải thật, query plan, Google/SMS thật, Gemini) vẫn nguyên.

## 5. Không đổi

Ba luật tiền (số nguyên đồng, Σ phân bổ bằng tổng, số dư tính lại được từ sổ) — đổi vẫn phải mở ADR trước.
`phase0/` và `docs/protocol/v1/` vẫn đóng băng. Repo guard vẫn fail-closed. ADR-0029 vẫn là luật của chiến dịch.
