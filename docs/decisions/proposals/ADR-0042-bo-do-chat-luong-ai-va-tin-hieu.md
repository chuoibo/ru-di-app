# ADR-0042 — Bộ đo chất lượng AI và tín hiệu phản hồi không mang nội dung

- Ngày: 2026-09-25.
- Trạng thái: **ĐỀ XUẤT — chờ Lead ký; số hiệu cấp lúc vào main.** Nếu main đã có ADR-0042 khác thì văn bản
  này nhận số trống kế tiếp, như ADR-0036 từng đổi số.
- Quyết định sản phẩm: người dùng chốt trong phiên lập kế hoạch 2026-09-25 rằng mục tiêu đợt này là cả hai con AI
  **thật sự thông minh**, và «thông minh» phải đo được. Kế hoạch đã duyệt; ADR này chờ chữ ký Lead.
- Thiết kế chi tiết: `docs/claude/2026-09-25/thiet-ke-ai/06-bo-do-chat-luong.md` (commit gốc `f251db7`).
- Cùng đợt: ADR-0037 engine, ADR-0038 hàng đợi và stream, ADR-0039 nhóm trong luồng, ADR-0040 RAG và nạp dữ
  liệu, ADR-0041 Nếp. Văn bản này chỉ quyết cách đo và tín hiệu phản hồi.
- Sửa và bổ sung một số điều khoản cũ, liệt kê ở mục 5; **không sửa bản lịch sử** của ADR nào.
- Không đổi: ba luật tiền; ADR-0034 §2.6 (mỗi lượt đo thật cần Lead duyệt số lời gọi); ADR-0010 §6.1 (không để
  máy sinh đáp án); câu «Chưa có bằng chứng hành vi nào» của CLAUDE.md (ADR-0006).

## 1. Bối cảnh

- Bộ đo hiện có là một file (`services/api/tests/skills/tra_loi_trong_nhom.py`): 16 ca, gọi brain thật, bộ chấm
  luật có test đỏ khi đáp sai. Chính nó ghi rằng nhiệt độ 0.0 và `--lap` lần «không chặn được gì» (`:27-28`).
  Số hôm nay là 10/16 trên một lần chạy mỗi ca, không có khoảng tin cậy.
- Engine sắp đổi từ một lời gọi mỗi job thành nhiều lời gọi có tool, truy hồi, trí nhớ và stream (ADR-0037).
  Không có phép đo lặp lại được thì không ai nói được bản mới hơn hay kém bản cũ.
- Lời gọi thật tốn tiền và phải được Lead duyệt từng lần; lần gần nhất duyệt 54, dùng đúng 54. Dự toán cần một
  trần cứng để tính, không phải một con số đoán.
- Sáu bản thiết kế của đợt này đề xuất ba binary eval, hai stub, hai geministub, một migration «v4» bị ba bản cùng
  đòi, và một mục tiêu latency (token đầu p50 ≤1,2 s) mà pipeline có guard không đạt được.
- Không có tín hiệu nào từ người dùng thật. Chat v2 là E2EE, máy chủ không đọc nội dung, nên tín hiệu nào sống qua
  cutover cũng phải không mang nội dung.

## 2. Quyết định

1. **Một binary eval, trên đúng seam của worker.** `services/core/cmd/rudi-eval` (build tag `eval`) gọi
   `aiharness.Engine.Run(ctx, Turn, Sink)` với một `Sink` ghi lại, và `Engine.Hieu` cho chế độ thành phần. Ba
   chế độ model: kịch bản (stub `aiharness/llm/stub.go`), cassette (ghi/phát lại), thật. Không vào image
   production, không vào `routes.json`; CI kiểm `go list -deps ./cmd/core` không chứa gói eval. Một stub, một
   `e2e/geministub`; không binary, stub hay registry thứ hai.
2. **Sáu tầng, mỗi tầng nói rõ nó không chứng minh gì.**
   - T0 offline (bộ chấm, corpus, thống kê, checker).
   - T1 kịch bản trong CI, trên Postgres dùng một lần: chứng minh đường ống, không chứng minh chất lượng.
   - T2 phát lại cassette ở máy local: chứng minh mã sau model, không chứng minh model hôm nay.
   - T3 model thật, Lead duyệt: số trong trailer.
   - T4 latency e2e trên `geministub`: chi phí hệ thống, không phải latency Gemini.
   - T5 online: M4.
3. **Bất biến trên mọi request và mọi luồng sự kiện.**
   - Request: có `system_instruction`; model đúng hằng flash-lite; «bây giờ» RFC3339 `+07:00` của
     `pairpaper.Local(Turn.Luc)`; tool ⊆ `quyen.golden.json`; không canary trí nhớ Nếp trong request nhóm;
     đầu ra Understand chỉ nằm trong `<du_lieu>` và chỉ gồm enum/slot có kiểu; số lời gọi ≤
     `MaxModelCallsPerTurn`, đếm cả retry.
   - Luồng sự kiện: `trang_thai` đi trước mọi chữ; `lam_lai` không bao giờ sau `delta` đầu; các `delta` nối lại
     là tiền tố của chữ cuối, tức không có «rút lại».
   - Thẻ `tra_loi`: đúng hình hợp đồng; `expense_draft.da_ghi` rỗng khi rời engine vì dấu «Đã ghi vào sổ» chỉ suy
     từ dòng chi tiêu thật.
4. **Thống kê.**
   - k=5 **chỉ** cho 16 ca lõi; bộ khác k=1 hoặc chế độ thành phần.
   - pass@1 theo ca, CI 95% bootstrap lấy lại cả ca, B=10 000, seed 20260925.
   - «Ca vững» = đạt ≥4/5. Chỉ số không dung sai in kèm cận 3/n.
   - So với nền bằng bootstrap ghép cặp, chỉ trên ca cùng `sha_ca`.
   - Lỗi hạ tầng không tính; quá 5% thì lượt vô hiệu. Chạm trần lời gọi mỗi lượt là **trượt**.
5. **Judge phải hiệu chuẩn trước khi gác.**
   - ≥100 mục nhãn người mỗi chiều, 30 mục hai người gán.
   - Chấp nhận khi κ có trọng số ≥0,70, cận dưới ≥0,60 và không kém κ giữa người quá 0,10; đa số 3 lần chấm;
     có kiểm thiên vị độ dài.
   - Chưa đạt thì in «chưa hiệu chuẩn» và không gác gì. Rubric, prompt judge và model version nằm trong vân
     tay: đổi là hiệu chuẩn lại.
6. **Ngân sách có trần cứng.** `--du-toan` = Σ lượt × `MaxModelCallsPerTurn` (+ judge). `--tran-goi` bắt buộc ở
   chế độ thật. Dự toán vượt trần thì không chạy; chạm trần giữa chừng thì dừng, đánh `chua_xong`, không in trailer.
7. **Bằng chứng ở ngoài Git.** Vết, cassette, bảng điểm, manifest ở `~/.cache/rudi-bang-chung/eval/<run_id>/`;
   `--out` trong worktree bị từ chối. Corpus tổng hợp viết tay và kịch bản stub vào Git; tệp hiệu chuẩn chỉ mang
   id, sha256 và nhãn, không chữ.
8. **Trailer eval trong commit.**
   - Các khoá: `Eval-Run`, `Eval-Fingerprint`, `Eval-Moc` và dòng số theo bộ. Vân tay = sha256 của các file làm nên
     hành vi đã sắp (prompt, khai báo tool, bảng quyền, enum Understand, cấu hình model, mẫu guard, rubric) + id model.
   - `check_eval_trailer.py`: commit chạm các file đó phải mang vân tay khớp và số đạt ngưỡng, hoặc
     `Eval-Chua-Do: <lý do>`.
   - `check_eval_release.py`: commit bật `MOBILE_AI_ENGINE_*=go` ở triển khai, và commit gỡ đường brain, phải có
     một lượt T3 đạt cho đúng vân tay ở SHA đó (cổng lát + M2).
9. **Thang M0–M4.**
   - M0: đường nền thật, 80 lời gọi.
   - M1: T0/T1 xanh trên engine Go.
   - M2: mỗi bề mặt ASR hệ thống 0/≥80, luật tiền 0, lộ trí nhớ chéo người 0, bịa id quán 0.
   - M3: lõi ≥14/16 vững, pass@1 ≥0,85 (cận dưới ≥0,78); định tuyến ≥0,92; recall@10 quán ≥0,90; grounding
     ≥0,98; Nếp đúng bước ≥0,85, bịa UI ≤2%; trí nhớ ≥0,90, quên 100% trong kho; latency như điểm 10.
   - M4: 14 ngày online, 👎 ≤8% số câu được chấm, thử lại ≤5%, tối thiểu 200 câu được chấm.
   - Số khác trong thiết kế chỉ báo cáo cho tới khi Lead ký.
10. **Latency theo hợp đồng chung.** Sự kiện trạng thái đầu p95 ≤300 ms (T4, cả SSE người gọi lẫn frame WS người
    xem); token chữ đầu p50 ≤2,5 s, p95 ≤5 s; `plan` p95 ≤8 s; lời gọi model p95 ≤4 mỗi lượt.
11. **Tín hiệu phản hồi không mang nội dung.**
    - Bảng `chat_ai_tin_hieu`: chỉ enum `thich|khong_thich|mo_dia_diem|tao_keo_tu_the` và lý do enum; không cột
      chữ, không ô chữ trên UI. Hàng thô sống 30 ngày.
    - Bảng `chat_ai_chi_so_ngay`: không id người, không id phòng.
    - `thu_lai`, `hoi_lai_ngay`, `bo_giua_chung` suy ở máy chủ chỉ từ metadata, tính thẳng vào bảng ngày.
      «Bỏ giữa chừng» là huỷ sau token đầu, không phải SSE đóng.
    - Hai route `GO-ONLY` `…/ai-invocations/{id}/tin-hieu`. Gộp và xoá là tác vụ định kỳ của ADR-0038.
      Xoá tài khoản qua trigger Go trên `people.deleted_at` của ADR-0041.
12. **Dữ liệu dùng chung nằm ở testdata của gói Go sở hữu**, Python đọc theo đường dẫn (tiền lệ
    `hoi_thoai_golden.json`); không thư mục gốc mới, tập vàng truy hồi không chép. Bộ 16 ca lõi đóng băng tại
    chỗ làm nền M0; ca mới vào file riêng.

## 3. Hệ quả

- CI thêm một job (T1: Go, Docker Postgres, Python). Commit chạm prompt hay tool tốn thêm một dòng trailer, hoặc
  một lý do viết ra. Đó là ma sát có chủ ý.
- Trần lời gọi theo mốc (với `MaxModelCallsPerTurn` = 8 như ADR-0037 đề xuất): M0 80; lát 9 là 704; M2 là 1 280;
  M3 khoảng 4 300 chia thành nhiều lần duyệt, cộng 600 cho hiệu chuẩn judge. Đây là trần; thật thường khoảng một nửa.
- Checker trailer chứng minh trailer có, đúng hình và khớp mã ở SHA đó. Nó **không** chứng minh lượt đã chạy: điều
  đó do manifest trong kho và phép phát lại ở mục 5.
- Trước khi `check_eval_release.py` có (lát 18), bật cờ ở prod dựa vào kỷ luật trailer.
- Bộ đo cũ sống tới lát 19 làm đường nền brain; bộ chấm chuyển sang `tests/evals/cham/` và được import lại, không chép.
- Migration của bảng tín hiệu lấy số theo thứ tự lên main; nó lên sau trigger xoá của ADR-0041 nên phải đăng ký
  cột người vào sổ xoá của `nepnho` trong cùng commit, và test liệt kê cột `person_id` bắt chỗ quên.
- Xanh ở mọi tầng vẫn chỉ nghĩa là «không thấy lỗi trên corpus tổng hợp do chính người thiết kế viết».

## 4. Cái này KHÔNG cho phép

- Không gọi model thật trong CI, test hay parity; không chạy lượt thật khi chưa có trần đã duyệt.
- Không commit vết, cassette, đầu ra model hay mục hiệu chuẩn có chữ.
- Không dùng model để sinh `ky_vong`, kịch bản stub, nhãn truy hồi hay nhãn hiệu chuẩn.
- Không để judge chưa hiệu chuẩn gác bất cứ gì; không đổi rubric mà giữ hiệu chuẩn cũ.
- Không cột chữ tự do trong bảng tín hiệu, không ô chữ trong UI 👎, không lưu nhãn guard con gắn với người.
- Không suy tín hiệu từ nội dung tin nhắn, không đọc bảng tin để suy tín hiệu.
- Không coi SKIP, `bang_lech`, lượt `chua_xong` hay lượt vô hiệu là xanh.
- Không tự đặt hằng của engine (trần lời gọi, trần chữ, enum ý định) trong Python: đọc từ binary.
- Không đo latency khi chạy song song.
- Không xoá bộ đo cũ hay bộ 16 ca lõi trước commit gỡ đường brain.

## 5. Điều khoản bị thay hoặc sửa (không sửa bản lịch sử của chúng)

| Điều khoản | Hiện nói | Sau ADR này |
|---|---|---|
| ADR-0030 §2.3, «chạy lại trong cây sạch tại đúng SHA … người gộp chạy lại» | Người gộp chạy lại cổng | Với lượt model thật, chạy lại = phát lại cassette của lượt đó ở đúng SHA (0 lời gọi, phải ra cùng điểm, cùng vân tay). Lượt thật mới chỉ khi Lead duyệt thêm |
| ADR-0034 §2.6, câu «Một lượt đo chất lượng thật chỉ chạy khi Lead duyệt riêng số lời gọi» | Lead duyệt số lời gọi | Giữ nguyên, nói rõ thêm: Lead duyệt một **trần** do `--du-toan` tính; lượt dừng ở trần; số thật dùng ghi ở `Eval-Run goi=` |
| CLAUDE.md, «số đo viết thẳng vào commit message» | Số đo trong commit message | Với số chất lượng AI: đúng các khoá `Eval-*`, để checker đọc được |
| CLAUDE.md, bảng «Mỗi tầng test chứng minh được gì» | Chưa có hàng AI | Thêm hàng T1, T2, T3, judge, T5, mỗi hàng có cột «Không chứng minh». Sửa khi ADR được ký |
| ADR-0036 §3b, «Đừng xoá một cổng chất lượng dưới danh nghĩa dọn dẹp» | Nguyên tắc | Áp dụng cụ thể: bộ đo cũ sống tới lát 19 |
| ADR-0037 §2.10, «Lời gọi thật chỉ đi qua binary eval» | Chưa nêu tên | Binary là `cmd/rudi-eval`, ba chế độ như §2.1 |

Không đổi bởi văn bản này:
- ADR-0023 §2.1: bản đồ xoá đóng giữ nguyên. Ngoại lệ trigger Go cho bảng chỉ Go có là của ADR-0041; bảng tín
  hiệu chỉ đi theo.
- ADR-0036 §2.5: tín hiệu không gửi nội dung nào nên không đụng lời hứa «Chỉ gửi lời nhờ».

## 6. Cổng nghiệm thu

- Lát 2: T0 thống kê xanh, canary đỏ; M0 với 80 lời gọi Lead duyệt, số vào commit message.
- Lát 6: T1 trong CI xanh, ca canh gác đỏ đúng chỗ, không SKIP; cổng `go list -deps`.
- Lát 9: T3 lõi ≥14/16 vững, ca 01, 03, 13, 16 đạt 5/5 (ADR-0040), với trần đã duyệt.
- Lát 18: T2 phát lại tái lập được một lượt T3; T4 trên `geministub`; judge qua hiệu chuẩn hoặc in «chưa hiệu
  chuẩn»; route tín hiệu từ chối chữ tự do; M2 và M3.
- Mỗi lát: ít nhất hai đột biến tự nghĩ, kiểm tương đương trước, đỏ đúng bước dự đoán; chạy lại trong cây sạch
  đúng SHA; số đo ghi thẳng vào commit message.
