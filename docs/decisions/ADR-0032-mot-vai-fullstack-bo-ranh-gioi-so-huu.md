# ADR-0032 — Một vai fullstack; bỏ ranh giới sở hữu và bỏ PR bắt buộc

**Trạng thái:** Chấp nhận · **Quyết định bởi:** leader, 2026-09-22 · **Hiện thực:** Claude
**Liên quan:** ADR-0001 (phân công Giai đoạn 0), ADR-0005 (đường đi review doc), ADR-0007 (quy trình PR),
ADR-0010 (agy làm QA), ADR-0029 (lõi backend sang Go), ADR-0030 (đóng lane Codex), ADR-0031 (chat Go E2EE)

## 1. Bối cảnh

Repo được dựng quanh giả định hai engineer làm song song mà không chạm cùng file. Giả định đó sinh ra:
bảng sở hữu hai cột ở `docs/architecture/00-layout-va-so-huu.md` (chốt 2026-08-27), luật nhánh
`<owner>/p0-w<N>-<slug>`, hai thư mục nhật ký `docs/claude/` và `docs/codex/`, một hàng đợi liên lạc
giữa hai lane, và toàn bộ cơ chế review chéo của ADR-0005 + ADR-0007 — trong đó
**«không tự review PR của chính mình»** là cổng trung tâm.

ADR-0030 đã gỡ nửa sau của giả định đó cho backend: lane Codex đóng ngày 2026-09-16, không còn review
chéo, cổng bằng chứng máy thay chỗ con mắt thứ hai. Nhưng ADR-0030 chỉ nói về backend. Frontend,
`apps/mobile/`, trang khách và toàn bộ tài liệu quy trình vẫn viết theo mô hình hai lane — tức là
tài liệu đang mô tả một đội không còn tồn tại.

Leader chốt ngày 2026-09-22: **không còn chia nhiệm vụ. Tất cả là fullstack engineering.**

## 2. Quyết định

1. **Một vai engineer fullstack.** Bảng sở hữu theo người hết hiệu lực trên toàn repo, không chỉ
   backend. Ai nhận một việc thì làm trọn lát cắt của việc đó: Go backend · SQL và migration ·
   Python AI · TypeScript frontend · mobile native · test mọi tầng · tự kiểm bằng chứng.

2. **Một việc là của một người từ đầu đến cuối.** Không chia đôi theo tầng, không bàn giao nửa chừng,
   không chờ lane khác đồng ý đóng băng. Nếu một việc lớn tới mức phải cắt, cắt theo **lát cắt dọc
   chạy được**, không cắt theo tầng.

3. **Bỏ PR bắt buộc.** Commit thẳng lên `main`. PR chỉ mở khi thật sự muốn người khác đọc trước khi
   vào `main`. Điều này thay thế luật merge của ADR-0007 và thay thế câu
   «Không tự review PR của chính mình» — câu đó nhắm vào rủi ro thật, nhưng rủi ro đó không gỡ được
   bằng một luật khi chỉ còn một người viết.

4. **Cổng bằng chứng thay chỗ chữ ký người.** Danh sách của ADR-0030 §3 nay áp cho **mọi tầng**, kể
   cả frontend và mobile:
   - cổng chạy lại trong **cây sạch tại đúng SHA**, không phải cây của agent;
   - canary phải đỏ ở đúng chỗ dự đoán; identity phải xanh; cùng SHA harness;
   - **ít nhất hai đột biến do chính người gộp tự nghĩ**, khác đột biến của agent, mỗi cái phải đỏ ở
     đúng bước đã dự đoán — và phải kiểm tương đương trước, vì một đột biến không đổi được kết quả thì
     «đỏ» hay «sống» đều vô nghĩa;
   - với UI: **mở ảnh chụp ra nhìn**. Bảng xanh không phải bằng chứng hình ảnh;
   - **số đo viết thẳng vào commit message**, không để chỉ nằm trong log.
   Digest của agent không phải bằng chứng.

5. **Commit message là nơi leader đọc.** ADR-0007 đặt gánh nặng giải thích lên mô tả PR. Không còn PR
   bắt buộc thì gánh nặng đó chuyển nguyên vẹn sang commit message: nói **cái gì đổi và vì sao**, kèm
   số đo cổng đã chạy. Đừng bắt người đọc suy từ diff.

6. **Nhánh:** tiền tố chủ sở hữu không còn bắt buộc. Slug vẫn phải là Work ID cụ thể —
   `backend` / `research` vẫn sai, vì Work ID là thứ nối nhánh với nhật ký và với `protocol_version`.

7. **agy không đổi.** agy vẫn là QA/QC chạy song song theo ADR-0010: nộp phát hiện, không nộp diff,
   không sở hữu file mã nguồn sản phẩm nào, không ký verdict, không sinh đáp án tiền. Digest của agy
   không phải bằng chứng; người giao việc chạy lại cổng trong cây sạch.

## 3. Hệ quả

- Không còn ai chờ ai. Không còn hàng đợi liên lạc, không còn thoả thuận đóng băng giữa hai lane.
- **Mất lớp người thứ hai trên toàn repo**, kể cả ở frontend — nơi ADR-0030 chưa chạm tới. Cái bù lại
  là phép đo máy chạy được, và với UI là con mắt người nhìn ảnh chụp thật. Cả hai đều yếu hơn một
  reviewer độc lập. Ghi ra đây để không ai đọc nhầm đây là nâng cấp về chất lượng.
- Nhật ký cũ ở `docs/archive/claude/` và `docs/archive/codex/` giữ nguyên tại chỗ: đó là lịch sử có
  thật, và `.repo-guard-allowlist.json` ghim sha256 theo đúng những đường dẫn đó.
- ADR-0001, ADR-0005, ADR-0007, ADR-0010 và ADR-0030 không bị sửa. ADR là snapshot bất biến; phần nào
  của chúng bị thay thì ADR này nói rõ phần đó.

## 4. Cái này KHÔNG chứng minh

Không chứng minh rằng một người làm fullstack thì chất lượng giữ nguyên. Nó chỉ nói rằng khi không còn
lane thứ hai, phép đo máy và ảnh chụp thật thay chỗ cho con mắt thứ hai, và chúng chỉ tính khi chạy
trong cây sạch. Mọi thứ ADR-0029 §4 và ADR-0030 §4 đã ghi là không chứng minh được — dữ liệu thật, tải
thật, query plan, Google/SMS thật, Gemini — vẫn nguyên.

Nó cũng không chứng minh rằng bỏ PR làm đi nhanh hơn. Nó đổi một cổng người lấy một cổng máy, và cổng
máy chỉ tốt bằng đột biến mà người viết nghĩ ra được.

## 5. Không đổi

Ba luật tiền (số nguyên đồng · `Σ` phân bổ bằng đúng tổng khoản chi · số dư tính lại được từ sổ) —
đổi vẫn phải mở ADR trước. `phase0/` và `docs/protocol/v1/` vẫn đóng băng tại chỗ. Repo guard vẫn
fail-closed. Ranh giới tầng vẫn là luật: `domain/` không import `db`/`api`, template trang khách không
bao giờ tự query, mỗi module có đúng một writer. Quy tắc dữ liệu người thật không đổi một chữ.
ADR-0029 và ADR-0031 vẫn là luật của chiến dịch đang chạy.
