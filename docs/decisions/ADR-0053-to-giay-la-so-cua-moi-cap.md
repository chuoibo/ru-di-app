# ADR-0053 — «Tờ giấy» là sổ của mọi cặp; lối tắt trong khay chat là của cặp đôi

- Ngày: 2026-10-04.
- Trạng thái: **Chấp nhận — chủ sản phẩm chốt 2026-10-04**, trả lời đề xuất
  `docs/claude/2026-10-01/ui-ux-upgrade/adr-de-xuat/UI-131-to-giay-chi-cho-cap-doi.md` («làm theo cách bạn khuyến
  nghị nhưng hãy tính đường xa hơn cho production ready»). Không phải chữ ký Lead.
- Gỡ một mâu thuẫn giữa ADR-0027 §4, ADR-0038 §2.1 và ADR-0046 §8.4. Không sửa bản lịch sử của ADR nào.
- Không đổi: ba luật tiền; ADR-0034 và ADR-0048 (gu, đồng ý); luật chặn và rời của sổ hai người (QA UI-120,
  `requirePairIsAlive`).

## 1. Bối cảnh

QA (PR #663, UI-131) đo được `POST /contexts/{id}/papers/draft` trả `201` cho một cặp **chưa** bật «Một đôi», trong khi
khay công cụ của chat chỉ đưa «Tờ giấy» cho cặp đôi. QA đọc đó là máy chủ thiếu một cái gác.

Ba văn bản đang nói ba điều:

- ADR-0027 §4 cố ý giữ **lời rủ tạm**: hai người rủ nhau trước khi lập sổ.
- ADR-0038 §2.1 cân nhắc từ chối tờ tạm (`cycle_not_active` khi chưa lập sổ) và **bác** phương án đó, vì nó xoá một
  tính năng ADR-0027 cố ý giữ.
- ADR-0046 §8.4 (2026-09-28) đặt «Tờ giấy» vào lớp cặp đôi **của giao diện chat**: hàng ghim và công cụ trong khay chỉ hiện
  khi cả hai đã bật «Một đôi».

Sổ hai người có hai loại, «Hội bạn» và «Một đôi» (`LoaiSo`). Mọi cặp có sổ, kể cả cặp bạn, viết tờ giấy trong sổ của
mình. ADR-0046 §8.4 chỉ nói về **lối tắt** từ chat, không nói về sổ.

## 2. Quyết định

1. **«Tờ giấy» (pair papers) là sổ của mọi cặp.**
   - Máy chủ nhận tờ của bất kỳ cặp nào còn hiệu lực: chưa có sổ (lời rủ tạm, ADR-0027 §4), sổ đã khép (tờ tạm), hay
     sổ đang chạy loại «Hội bạn» hoặc «Một đôi».
   - Cái gác giữ nguyên như hiện có: thành viên của cặp, sổ không ở trạng thái chờ, không có tờ đang mở, trần mỗi tuần
     (ADR-0034 §2.5), cặp còn sống (không chặn, không ai rời, không ai xoá tài khoản; QA UI-120).
   - **Không** thêm `can_bat_doi` vào `draft_pair_paper`.
2. **Lối tắt trong chat là của cặp đôi** (ADR-0046 §8.4 giữ nguyên): công cụ «Tờ giấy» trong khay và hàng ghim đầy đủ chỉ
   khi `cap_doi`. Cặp bạn vào sổ của mình từ hàng mời trong chat và từ hồ sơ người kia.
3. **Lối «Rủ X tới đây» không phác tờ cho cặp bạn** (QA UI-085, UI-130, đã làm ở B8). Quán mở một kèo của hai người với
   quán là chặng đầu, hoặc thêm vào kèo đã hẹn. Một tờ phác ra rồi bỏ quán là lỗi của lối đi, không phải của luật.
4. Mọi tính năng riêng của cặp đôi vẫn đòi `CanBatDoi` ở máy chủ, như đã có: gu cho Nếp (`guChoNep`), «Người lo» của
   tuần, `chia_gu`, các bước đồng ý của «Một đôi». Ranh giới «riêng của cặp đôi» là **nội dung**, không phải tờ giấy.

## 3. Hệ quả

- QA UI-131 đóng theo phương án B của đề xuất: máy chủ không đổi; hành vi `201` là cố ý.
- Phần app (phương án C) đã nằm trong B8: «Rủ … tới đây» cho cặp bạn không còn phác một tờ bỏ quán.
- Khi số người dùng tăng, không có hai luật song song cho cùng một bảng: sổ một luật, chat một luật, và test ghim cả hai.

## 4. Bằng chứng ghim cả hai phía

- Máy chủ, Go so với Python oracle trên PostgreSQL thật (`services/core/internal/repo/pair_repo_oracle_postgres_test.go`):
  - «a notebook whose cycle closed» và «no notebook yet»: thành công;
  - ca mới «an active friends' notebook, «Một đôi» not on»: thành công ở cả hai máy chủ. Canary đổi kỳ vọng thành
    `409:paper_wrong_state` đỏ đúng ca: «python ended in ""».
- App: `apps/mobile/tests/ai-chat-hai-nguoi.test.mjs` ghim `onToGiay={capDoi ? …}` (công cụ trong khay chỉ cho cặp đôi)
  và `SoHen` chỉ thêm «Tờ giấy» khi có `onToGiay`; `apps/mobile/tests/to-giay-luat.test.mjs` và
  `rudi-nhom-nguoi-b8.test.mjs` ghim lối «Rủ … tới đây» (B8).
- Maestro `47-to-giay-hai-nguoi.yaml`: thang đồng ý tới cặp đôi, hàng ghim «Tờ giấy» chỉ khi cả hai đã bật «Một đôi».
