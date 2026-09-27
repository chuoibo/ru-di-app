# ADR-0043 — Trí nhớ Nếp: Postgres là sổ, Milvus là chỉ mục dẫn xuất, mem0 là thư viện trong sidecar suy luận Python

- Ngày: 2026-09-27.
- Trạng thái: **ĐỀ XUẤT — chờ Lead ký; số hiệu cấp lúc vào main.** Nếu main đã có ADR-0043 khác
  thì văn bản này nhận số trống kế tiếp.
- Thực hiện một phần ADR-0041 §2.5–§2.9 (trí nhớ «lặng lẽ nhưng có công bố»). Cùng đợt: ADR-0037
  engine, ADR-0038 hàng đợi, ADR-0040 RAG.
- Nguồn đã đọc: `scratchpad/research/mem0.md` và `stm-personalization.md` (phần **Kiểm chứng** thắng
  phần thân khi hai bên khác nhau), `research/milvus.md`, thiết kế 05 §6, hợp đồng sidecar trong
  `services/ai-infer` (commit `90a9f7e`); §2.6 thêm `research/qwen-reranker.md` (cùng phần
  Kiểm chứng) và quyết định của chủ sản phẩm ngày 2026-09-27 về reranker.
- **Có lệch khỏi ADR-0031**. Mục 4 nêu tên từng chỗ lệch; ADR này chỉ có hiệu lực khi Lead ký chấp
  nhận các chỗ lệch đó.

## 1. Bối cảnh

- ADR-0041 cho phép Nếp nhớ sở thích đi chơi của **chính người dùng**. Điều kiện: mặc định tắt, công
  bố trước khi bật, xoá thật khi người dùng bảo «quên», không bao giờ lọt sang bot nhóm.
- Nghiên cứu mem0 2.2.1 (đã kiểm chứng lại) tìm ra bốn chỗ làm vỡ «quên = xoá cứng» nếu dùng mem0
  mặc định:
  - SQLite `history`/`messages` giữ nguyên văn ký ức và 10 tin nhắn gần nhất, kể cả sau khi xoá.
  - Telemetry PostHog bật sẵn và gửi MD5(user_id).
  - `get`/`delete` theo id không kiểm chủ sở hữu.
  - `delete_all` có thể dừng giữa chừng mà vẫn báo thành công.
- Redis mặc định vẫn chụp RDB nếu không đặt `save ""`. Một key đã hết TTL vẫn có thể nằm trong RAM
  một lúc.

## 2. Quyết định

### 2.1 Postgres là sổ; Milvus là chỉ mục dẫn xuất

- `internal/nepnho` là writer duy nhất của mọi bảng `nep_*`. Bảng version riêng
  `nep_schema_migrations`, có checksum. `core migrate-chat` chạy nó sau `internal/jobs`.
  `serve`/`work` từ chối chạy khi có khoá trí nhớ mà schema còn cũ.
- Các bảng:
  - `nep_cai_dat`: cờ `nho`, mặc định `false`. Bật phải kèm phiên bản bản công bố và `cong_bo_at`
    (CHECK).
  - `nep_su_that`: một **biên nhận** cho mỗi ký ức mà sidecar giữ. Gồm id của mem0, `person_id`,
    `loai` đóng, nguồn, thời gian hiệu lực, `created_at`, `dang_xoa_at`, `deleted_at`. **Không có
    chữ.**
  - `nep_quen`: HMAC-SHA256 (khoá máy chủ) của chữ đã quên, giữ 365 ngày. Nhờ vậy câu đã quên không
    được ghi lại.
  - `nep_su_kien`: sự kiện app kiểu đóng, chỉ id và enum, giữ 30 ngày.
  - `nep_xoa`: sổ saga xoá kèm biên nhận.
- Postgres quyết định **cái gì tồn tại**:
  - Mọi lần đọc từ sidecar đều lọc lại bằng biên nhận còn sống của đúng người đó. Một hàng sidecar
    trả về mà không có biên nhận thì không bao giờ được nhắc lại.
  - `what_you_remember` vẫn liệt kê hàng đó dưới mục «giữ mà không có sổ», và lần «quên hết» sau sẽ
    xoá nó.
- Milvus (DB `nep_memory`, collection `memories_v<N>`) giữ chữ và vector. **Chữ chỉ nằm ở đó**:
  mục 4 nêu hệ quả.

### 2.2 mem0 là thư viện trong sidecar suy luận Python (`services/ai-infer`)

- Không chạy server REST của mem0. Chỉ Go gọi sidecar: loopback hoặc mạng riêng, Bearer
  `AI_INFER_TOKEN`. Hợp đồng gồm `/v1/memory/{add,search,list,delete,delete_all,purge_user}`:
  - id của người khác → 404, cùng câu trả lời với «không có».
  - Xoá mà còn sót hàng → 500 kèm `remaining`. Sidecar chỉ báo xong khi đếm lại bằng Strong
    consistency ra 0.
- Sidecar đã có các biện pháp sau (`90a9f7e`, có test):
  - `MEM0_TELEMETRY=false` được đặt trước khi import mem0.
  - `SQLiteManager` thay bằng no-op (không SQLite nào, kể cả `:memory:`).
  - Logger của mem0 bị tắt.
  - Không cài spaCy, nên không có entity store.
  - Kiểm chủ sở hữu trên từng id.
  - `delete_all` lặp tới khi đếm ra 0.
  - `purge_user` flush, compact L0 rồi compact mix, rồi mới đếm.
- Sidecar là **writer duy nhất** của collection trí nhớ. Go không ghi Milvus: bản nháp đầu từng có
  đường xoá thẳng qua REST của Milvus, và đã bỏ vì hai writer trên một collection trái luật «mỗi
  module một writer».

### 2.3 Go giữ chính sách (`internal/nepnho`)

- **Nhớ lại**: chỉ khi cờ bật. Tắt thì sidecar không nghe câu hỏi nào. Tối đa 10 ứng viên, vì sidecar
  nhận `top_k` 1..10. Lỗi 422 này bắt được khi chạy đầu-cuối với sidecar thật: bản trước xin 15.
- **Ghi**: chỉ khi được gọi, qua tool `remember_fact`. Lõi agent đã gác bằng ý định của router và cổng
  dữ liệu. Go không đọc chữ để quyết định; nó chỉ kiểm cờ, cấu trúc, tombstone của chữ gửi đi và của
  chữ trích ra, và trần 200 ký ức. Việc trích và phân loại (của chính mình? tiền? nhạy cảm?) là của
  model trích trong sidecar.
- **Quên** là một saga (`nep_xoa`):
  1. Sidecar xoá (`delete` / `delete_all` / `purge_user`).
  2. Sidecar phải trả `remaining` bằng 0.
  3. Go tự liệt kê lại (collection ở Strong consistency) và cái đã xoá phải vắng mặt.
  4. Xoá bộ đệm ngắn hạn (với «quên hết» và xoá tài khoản).
  5. Ghi biên nhận.

  Thiếu bước nào thì ghi mã lỗi đóng, tăng `lan_thu`, đẩy `chay_luc` theo backoff 30 s → 1 h. Trigger
  `nep_xoa_enqueue` xếp lần thử kế tiếp vào **làn `memory`** của outbox (`jobs_them`, chỉ id đi qua
  broker). `core work` tiêu thụ làn này; `nep.xoa` định kỳ là lưới an toàn khi broker vắng. Lặp tới
  khi đếm ra 0.
- **Tắt cờ** xoá hết, bằng cùng saga «tất cả». Sự kiện và tombstone đi ngay trong transaction đó.
- **Xoá tài khoản**: trigger `nep_xoa_nguoi` trên `people.deleted_at` chạy dù Go hay Python đặt cột.
  - Trong cùng transaction: xoá sự kiện, tombstone và cài đặt; ẩn mọi biên nhận; xếp một việc
    `tai_khoan`.
  - Làn `memory` gọi `purge_user` tới khi còn 0.
  - Khi xong: mọi hàng `nep_xoa` của người đó bỏ `person_id`, chỉ giữ `nguoi_bam` (HMAC của id).
    Không còn hàng `nep_*` nào gọi tên người đã xoá.
  - Test `-tags postgres` liệt kê mọi cột nhắc tới người trong bảng do Go tạo (đọc từ SQL nhúng, đối
    chiếu catalogue với bảng `nep_*`) và chứng minh các cột của `nepnho` còn 0 hàng.
- **Route GO-ONLY**:
  - `GET`/`PUT /me/nep/tri-nho/cai-dat`
  - `POST /me/nep/su-kien`: sự kiện kiểu đóng; cờ tắt thì trả 409 `nep_tri_nho_tat`.
  - `DELETE /me/nep/tri-nho`

  Người được xác định bằng phiên Bearer, không bao giờ lấy từ thân request. Host không có
  `MOBILE_NEP_MEMORY_KEY` thì trả 503.

### 2.4 Trí nhớ ngắn hạn trên instance `redis-ai` riêng (`internal/aictx`)

- Theo phương án A, tương thích ADR-0036 §4: thiết bị vẫn là nguồn của phiên. Mỗi lượt Nếp có một
  key riêng `…:aictx:nep:<người>:<lượt>`:
  - Ghi lúc lượt bắt đầu; TTL 5 phút đặt trong cùng MULTI (`EXPIRE NX`).
  - `UNLINK` khi lượt kết thúc, không trông vào TTL.
  - Có chỉ mục theo người để «quên hết» và xoá tài khoản `UNLINK` chính xác, không cần `SCAN`.
- Nhóm lane legacy: `…:aictx:grp:<phòng>:<lượt gọi>` `EX 900`, không lượt ghi nào kéo dài cửa sổ.
  Phòng E2EE v2: **không đặt được tên key nào**, `PhienNhom` từ chối.
- Mọi giá trị được niêm phong AES-256-GCM bằng khoá `MOBILE_AICTX_KEY`, AAD là tên key.
- Lúc khởi động, kiểm `CONFIG GET save` = `""` và `appendonly` = `no`:
  - `prod`: sai hoặc không đọc được thì **từ chối chạy**.
  - `dev`: chỉ cảnh báo.
- Cấu hình vận hành đề xuất: `maxmemory-policy volatile-ttl`, `hide-user-data-from-log yes`,
  `slowlog-log-slower-than -1`, ACL chỉ `~rudi:*:aictx:*`, cấm `MONITOR`/`KEYS`/`CONFIG` với user ứng
  dụng, không replica. Ghim `redis:8.10.2` (hoặc `8.8.3`), không dùng `latest`.

### 2.5 Cá nhân hoá

- Mỗi lượt Nếp: tối đa 5 ký ức của chính người hỏi, đặt vào khối `<du_lieu nguon="tri_nho">` của lời
  gọi trả lời (router không thấy khối này). Chỉ khi cờ bật.
- Đường này chưa có reranker (§2.6 chỉ rerank truy hồi địa điểm và sổ tay): rerank ký ức cá nhân
  vẫn là quyết định mở ở §5.
- Nhớ lại lỗi thì lượt vẫn trả lời, chỉ thiếu khối này.

### 2.6 Reranker: Qwen3-Reranker-4B trên GPU, gọi qua HTTP (chủ sản phẩm chốt 2026-09-27)

- **Model sản xuất**: Qwen3-Reranker-4B (Apache-2.0) trên GPU sau vLLM, hợp đồng `/rerank` của vLLM
  (nghiên cứu `scratchpad/research/qwen-reranker.md`, phần **Kiểm chứng** thắng phần thân). Trên
  MMTEB-R, 4B hơn 0.6B +6,4 điểm; 8B gần như bằng 4B. Máy dev đứng thay bằng `llama-server` với
  GGUF 0.6B ở `127.0.0.1:18081` (CPU chậm, đo được ~140–255 token/s: chỉ dùng cho test với hạn rộng).
- **Gọi từ orchestrator Go** (`internal/rerank`), không qua Milvus Model Ranker: một request cho mọi
  tài liệu, hạn riêng, lỗi thì không làm hỏng truy hồi. Đường là `POST {URL}/rerank`, **không**
  `/v1/rerank` (vLLM 0.30 báo `/v1/rerank` lỗi thời). Chỉ đọc `results[].index` và
  `results[].relevance_score`; mọi index phải trong `[0, n)`, không trùng, đủ một cho mỗi tài liệu;
  điểm phải hữu hạn. Phần `document` vọng lại không bao giờ được đọc; không body nào vào log.
- **Cấu hình** (đọc một lần lúc khởi động engine Go, `cmd/core` → `aiharness.WithXepLai`):
  - `MOBILE_RERANK_URL` — trống thì không rerank: mọi truy hồi giữ thứ tự RRF và gắn cờ `no_rerank`.
    `http` chỉ tới loopback; ra khỏi máy phải `https` (câu hỏi của người dùng và token đi trên
    request). URL sai thì tiến trình từ chối khởi động.
  - `MOBILE_RERANK_MODEL` — tên model gửi trong request (vLLM `--served-model-name`), mặc định
    `Qwen3-Reranker-4B`.
  - `MOBILE_RERANK_TIMEOUT` — hạn mỗi lời gọi, `(0, 60s]`, mặc định `3s`.
  - `MOBILE_RERANK_TOKEN` — bearer tuỳ chọn (vLLM `--api-key`), ≥ 16 ký tự, không khoảng trắng,
    không bao giờ ghi log.
- **Nối vào engine**: engine giữ reranker; mỗi lượt Nếp bọc nó trong bộ đếm `rerank.Dem` với
  `llm.MaxRerankCallsPerTurn` = 2 và đặt vào ngữ cảnh của lượt (`truyhoi.VoiXepLai`). Retriever
  hybrid dùng chung giữa các lượt (`internal/hybrid`) rerank bằng reranker lượt của nó mang tới, nên
  tool `search_places` của vòng agent được rerank; vòng sửa của đường truy hồi (`crag`) rerank **một
  lần** trên ứng viên đã trộn của mọi truy vấn router, còn retriever thì nhường
  (`truyhoi.HoanXepLai`, không rerank và không gắn cờ). Cả hai đường đếm chung một ngân sách; lời gọi
  thứ ba trong lượt giữ thứ tự RRF và gắn `no_rerank`. `so_xep_lai` của hàng metrics là số lời gọi thật.
- **Ứng viên**: reranker đọc tối đa 30 ứng viên đầu theo thứ tự RRF (`truyhoi.UngVienXepLai`), trả
  `k` của request; đường truy hồi xin 30 ứng viên khi có reranker rồi cắt còn 8 sau rerank.
  Truy vấn reranker chấm là **dạng khôi phục dấu** của router (điểm Qwen3-Reranker tụt mạnh với câu
  không dấu, nghiên cứu §3.4).
- **Hỏng**: quá hạn, lỗi HTTP, trả lời sai hợp đồng, mạch đang mở hay hết ngân sách lượt → giữ nguyên
  thứ tự RRF, gắn `no_rerank`, lượt đi tiếp. **Không thử lại trong lượt**. Cầu dao mở sau 5 lỗi trong
  30 s và mở 60 s: reranker chết tốn một lần quá hạn mỗi cửa sổ, không phải mỗi lượt.
- **Điểm chỉ để xếp**: điểm nằm riêng ở `BangChung.DiemXepLai`, không ghi đè điểm truy hồi và không
  bao giờ là ngưỡng cắt (model nhỏ chấm tài liệu liên quan và bẫy từ vựng cùng gần 1,0). Ràng buộc
  cứng đã lọc trước khi reranker thấy ứng viên.
- **Làm sạch token**: chuỗi token đặc biệt của họ Qwen (kể cả dạng full-width sau NFKC, dạng có khoảng
  trắng trong ngoặc, các added token `<tool_call>`/`<think>`) và nhãn template bị gỡ khỏi truy vấn và
  tài liệu trước khi gửi. Đây là làm sạch cấu trúc theo từ vựng token của model, không đọc nghĩa.
- **Đây là suy luận AI** (ADR-0031 cho phép), không phải backend nghiệp vụ; client là Go, server là
  vLLM/llama.cpp không trạng thái: «quên» và xoá tài khoản không phải dọn gì ở reranker (nếu bật
  prefix cache ghi ra đĩa của vLLM thì phải tắt).
- Bot nhóm không nhận gì:
  - `Engine.Run` kết thúc lượt nhóm trước mọi lời gọi.
  - Cổng `aigate` `TestGroupNeverReachesMemoryStores` đỏ nếu bất kỳ handler/job nhóm nào với tới gói
    `nepnho`, client sidecar, hay key Nếp của `aictx`.

## 3. Luật và pháp lý (thứ cấp, **chưa phải tư vấn luật** — Lead/pháp chế xác nhận)

- Luật Bảo vệ dữ liệu cá nhân **91/2025/QH15** có hiệu lực từ 2026-01-01.
- **Nghị định 356/2025/NĐ-CP** (ký 2025-12-31, hiệu lực 2026-01-01) hướng dẫn luật và **thay Nghị định
  13/2023/NĐ-CP**. ADR-0041 §1 đang dẫn «Nghị định 13/2023». Khi ADR-0041 được ký, phần dẫn chiếu
  phải là Luật 91/2025/QH15 + NĐ 356/2025/NĐ-CP. Văn bản đề xuất của ADR-0041 không bị sửa ở commit
  này.
- Gửi lời người dùng sang Gemini (máy chủ ngoài Việt Nam) để trích ký ức là **chuyển dữ liệu xuyên
  biên giới**. Cần hồ sơ đánh giá tác động trước khi bật `AI_INFER_GEMINI_MODE=real` cho người thật.
- Rút lại đồng ý phải có hiệu lực bất cứ lúc nào: tắt cờ là xoá hết (§2.3). Thời hạn xoá vật lý
  (mục 5) phải ghi vào bản công bố.

## 4. Chỗ lệch khỏi ADR-0031, nêu tên

1. **Sidecar Python ghi vào một kho** (collection trí nhớ trong Milvus). ADR-0031 chỉ cho Python làm
   suy luận, trích xuất và đánh giá.
   - Ngoại lệ đề xuất: sidecar là writer duy nhất của một **chỉ mục vector**. Chỉ Go ra lệnh cho nó,
     và nó không giữ quyết định nào: đồng ý, tồn tại, xoá đều nằm ở Go/Postgres.
   - Viết lại toàn bộ đường ghi trong Go theo thuật toán hai pha của paper là phương án thay thế
     (research mem0 §8). Nó đúng ADR-0031 hơn, đổi lại mất prompt và đường nâng cấp của mem0.
2. **Chữ của ký ức không nằm trong Postgres** nên không dựng lại được từ sổ. Mất Milvus là mất chữ
   ký ức. Biên nhận vẫn còn, và `what_you_remember` báo «đang giữ, không đọc được chữ» chứ không bịa.
   - Chấp nhận vì trí nhớ là trợ giúp, người dùng nói lại được.
   - Làm vậy thì Postgres không giữ lời người dùng, và một bản sao lưu Postgres không mang ký ức.
3. Go không có client Milvus cho trí nhớ. Mọi thao tác, kể cả kiểm đếm của saga, đi qua sidecar.
   Kiểm chứng phía Go là liệt kê lại qua sidecar, không phải truy vấn Milvus độc lập.

## 5. Quyết định còn mở

- **Milvus GC `dropTolerance`**: xoá trong Milvus là xoá logic. Compaction chỉ chạy tường minh khi xoá
  tài khoản; «quên một» và «quên hết» chờ compaction tự động. File vật lý chỉ mất sau GC.
  - Cần đọc/đặt `dataCoord.gc.dropTolerance` và chu kỳ GC trên bản triển khai, rồi ghi con số «xoá
    vật lý trong ≤ N giờ» vào bản công bố.
  - Có nên compact tường minh sau cả «quên hết» không? Mỗi lần mất 2,7–5,8 s trên máy dev.
- **Backup/snapshot và «quên»**: snapshot Milvus (và WAL/object storage) giữ ký ức đã quên tới khi hết
  hạn. Hai lựa chọn:
  - (a) không backup collection `nep_memory` (theo §4.2, mất là chấp nhận được);
  - (b) backup có hạn giữ ≤ N ngày, ghi vào bản công bố, và sau khôi phục phải chạy lại mọi `nep_xoa`
    đã xong trong khoảng đó.

  Đề xuất (a).
- **Reranker cho cá nhân hoá** (ký ức): truy hồi địa điểm/sổ tay đã chốt 4B trên GPU (§2.6); có
  rerank cả ký ức cá nhân không thì còn mở.
  - Không tốn lời gọi Gemini, nhưng tính vào ngân sách rerank của lượt.
  - Cần đo latency p95 trên GPU thật và độ chính xác trên bộ eval tiếng Việt trước khi bật; golden so
    với `transformers` fp32 là bắt buộc khi đổi image hay model (Kiểm chứng, điều chỉnh 2).
- **Hạn ghi 5 s**: Milvus local thỉnh thoảng đứng khoảng 5 s ở lần chèn (đo đầu-cuối: 107 ms thường,
  5,2 s khi đứng). `remember_fact` đang ghi đồng bộ sau khi câu trả lời qua kiểm, trong trần 8 s của
  lượt.
  - Chuyển ghi sang làn `memory` (bất đồng bộ, idempotent theo lượt), hay nới `ThemHan`?
- **Host không có `MOBILE_NEP_MEMORY_KEY`**: trigger vẫn xếp việc xoá tài khoản. Việc đó nằm ở `cho`
  tới khi có khoá, còn tin trên làn `memory` nằm trong RabbitMQ không ai tiêu thụ. Hai lựa chọn: bắt
  buộc khoá khi `nepnho` đã migrate, hoặc chấp nhận hàng chờ.
- **Cột nhắc người chưa được xoá theo tài khoản**, không thuộc `nepnho`: `chat_ai_invocations`,
  `chat_plan_promotions`, `chat_shared_drafts` (chatassist), `chat_v2_devices`, `chat_v2_events`
  (chatv2). `nepnho.CotNguoiGo` liệt kê chúng là `chua`. Gói sở hữu phải tự làm.
  Sau khi gộp bảng tin cộng đồng của `main`: mười ba cột nhắc người của `community` (trigger
  `community_erase` của chính gói xoá, chưa có test nào đếm lại sau khi xoá tài khoản) và ba cột của
  `diary` (`outing_diaries`, `outing_diary_jobs` do trigger `diary_erase_for_account` xoá, test
  postgres của `diary` đếm; `outing_endings.ended_by` chưa được xoá) cũng được liệt kê là `chua`.

## 6. Bằng chứng (lúc soạn)

Số đo nằm trong commit message của lát này. Tóm tắt:

- Test đơn vị, `-tags postgres`, `-tags broker` của `nepnho`, `aictx`, `aiharness`, `aigate`.
- Test đầu-cuối cục bộ: Go `KhachHTTP` → sidecar thật (mem0, model trích có kịch bản, embed stub) →
  Milvus 3.0.2 thật, trên Postgres thật:
  - ghi, nhớ lại, cá nhân hoá, quên một, quên hết, xoá tài khoản bằng `purge_user`;
  - id của người khác bị từ chối.

Chưa có: người dùng thật; model trích thật trên tiếng Việt (chưa có bộ eval trí nhớ); đo trên bản
triển khai.
