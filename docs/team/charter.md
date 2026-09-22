# Điều lệ làm việc — một engineer fullstack + leader

> Bản gốc là kết quả hội tụ sau 4 vòng debate Claude ↔ Codex, 2026-08-26.
> **Viết lại 2026-09-22 theo `ADR-0032`**: lane Codex đã đóng (`ADR-0030`, 16/09), không còn chia
> nhiệm vụ theo tầng, không còn review chéo, không còn PR bắt buộc.
> Tài liệu này là **nguồn sự thật về quy trình**. Đổi nó cần một ADR trong `docs/decisions/`.

## 1. Vai

| Vai | Ai | Chịu trách nhiệm |
|---|---|---|
| Leader | Chủ sản phẩm | Tuyển nhóm, chỉ định/đóng vai operator, thuê counsel, ngân sách và khuyến khích, giám sát tiền thật giữa participant, xử lý sự cố thực địa, **ký quyết định gate** |
| Engineer | Một vai fullstack | **Toàn bộ sản phẩm**: Go backend · SQL và migration · Python AI · TypeScript frontend · `apps/mobile/` · trang khách · test mọi tầng · giao thức đo, thiết kế thí nghiệm, chính sách dữ liệu, threat model, repo guard, gate packet |
| QA | agy | Kiểm thử sản phẩm: hình ảnh, thăm dò, API, hồi quy — **chạy song song** với việc engineering. **Nộp phát hiện, không nộp diff — không sở hữu file mã nguồn sản phẩm nào** *(ADR-0010)* |

**Một việc là của một người từ đầu đến cuối** *(ADR-0032)*. Không chia đôi theo tầng, không bàn giao nửa chừng, không chờ lane khác đồng ý. Việc lớn thì cắt theo **lát cắt dọc chạy được**, không cắt theo tầng.

**Vẫn chạy hai luồng song song**: task của mình, và kiểm việc đã làm — nhưng luồng thứ hai giờ là *tự kiểm bằng phép đo máy*, không phải chữ ký của người khác. Không xếp hàng, không để việc pending trong lúc đang thảo luận.

**Leader lane là đường găng thật.** Engineer không bù được việc chưa tuyển được nhóm hoặc chưa có operator bằng cách viết thêm code.

Engineer không phủi trách nhiệm kỹ thuật khi công cụ lỗi, kể cả công cụ dùng một lần.

## 2. Nhánh

```
p0-w<N>-<slug>                    ví dụ  p0-w9a-repo-guard
```

Tiền tố chủ sở hữu **không còn bắt buộc** *(ADR-0032)*. `<slug>` mơ hồ kiểu `backend` / `research` vẫn là sai. **Work ID là thứ nối branch ↔ nhật ký ↔ protocol_version.**

### `main` và PR

> **Không còn PR bắt buộc** *(ADR-0032, thay luật merge của ADR-0007)*. Commit thẳng lên `main`.

Mở PR chỉ khi thật sự muốn người khác đọc trước khi vào `main`. Khi đó verdict vẫn đúng ba giá trị `APPROVE` / `REQUEST_CHANGES` / `REJECT`, và `REQUEST_CHANGES` vẫn chặn merge.

**Leader chỉ đọc `main`, và giờ chỉ còn commit message để đọc.** Nên commit message phải nói *cái gì đổi và vì sao*, kèm số đo của cổng đã chạy. Đừng bắt người đọc suy từ diff.

Ghi chép dài (khi cần lập luận hơn một dòng commit) đặt ở `docs/claude/<YYYY-MM-DD>/` và đi cùng commit mà nó nói về.

### Cái gì thay chỗ con mắt thứ hai

Không còn review chéo, nên cổng là phép đo máy chạy được *(ADR-0030 §3, nay áp cho cả frontend và mobile)*:

- cổng chạy lại trong **cây sạch tại đúng SHA** — không phải cây của agent;
- canary đỏ ở đúng chỗ đã dự đoán, identity xanh, cùng SHA harness;
- **ít nhất hai đột biến tự nghĩ**, kiểm tương đương trước, mỗi cái đỏ ở đúng bước đã dự đoán;
- với UI: **mở ảnh chụp ra nhìn**. Bảng xanh không phải bằng chứng hình ảnh;
- **số đo viết thẳng vào commit message**, không để chỉ nằm trong log.

Digest của agent không phải bằng chứng. Người giao việc chạy lại.

## 3. Hai cổng tách biệt

**MERGE-GATE** — được phép merge vào `main`:
- reviewer đã ra verdict, không còn blocker mở
- test/kết quả tái lập được

**FIELD-GATE** — được phép chạm người thật và dữ liệu thật:
- W9a **enforcement đang hoạt động** — không phải chỉ artifact xong. Xem mục 3.1
- W9 (chính sách dữ liệu) xong + **counsel checkpoint đã qua**
- W4a (threat model nghiên cứu) xong
- W0 (protocol + measurement contract) đã đóng băng ở một `protocol_version`
- W6 differential gate xanh — nếu phiên đó có công cụ tính hoặc đề xuất số tiền
- leader lane sẵn sàng: operator đã chỉ định, nơi lưu dữ liệu ngoài repo, kế hoạch sự cố/hoàn trả, đã chạy dry run

Merge được **không** đồng nghĩa được ra thực địa.

### 3.1 `W9a artifact xong` ≠ `W9a enforcement active`

Hai trạng thái khác nhau và **chỉ trạng thái thứ hai mới mở được FIELD-GATE**. *(Blocker B-01 của Codex, 2026-08-26.)*

| Trạng thái | Nghĩa | Ai làm |
|---|---|---|
| `artifact_complete` | Scanner, hook, workflow file, allowlist, runbook đã tồn tại và test xanh | Engineer |
| `enforcement_active` | Required status check `repo-guard` đã **bật** · PR bắt buộc · **chặn direct push** (hoặc giới hạn bypass rõ ràng) · đã chạy **một PR dry-run âm tính** và nó thực sự bị chặn · bằng chứng cấu hình (không chứa PII) đã lưu vào gate packet | **LEADER** |

Lý do phải tách: hook local bị bỏ qua bằng `--no-verify`. Có workflow file trong repo mà required check chưa bật thì scanner đỏ vẫn merge được — và FIELD-GATE sẽ bị hiểu nhầm là đã mở trong khi hàng rào server chưa hề enforce.

## 4. Quyền chặn

Blocker chỉ hợp lệ nếu thuộc một trong năm loại:

1. vi phạm spec hoặc vi phạm cổng
2. sai tiền
3. quyền riêng tư / bảo mật / consent
4. làm hỏng tính hợp lệ của thí nghiệm
5. kết quả hoặc test không tái lập được

Mọi thứ khác — đặt tên, phong cách, "tôi thích cách kia hơn" — là **suggestion**, không chặn được.

Blocker phải kèm: dẫn chứng · hậu quả · tiêu chí cụ thể để gỡ chặn.

Không còn reviewer thứ hai nên **không còn SLA review** *(ADR-0032)*. Blocker giờ chủ yếu do chính người làm tự đặt lên việc của mình, hoặc do leader đặt khi đọc `main`. Một blocker tự đặt vẫn phải kèm đủ ba thứ trên và vẫn chặn — tự gỡ chặn bằng cách viết lại tiêu chí là gian lận với chính mình.

Vẫn kiểm hai lần, nhưng là hai lần **đo**, không phải hai lần người: **protocol/contract trước khi implement hoặc thu dữ liệu**, và **cổng bằng chứng trước khi vào `main`**. Kiểm sau khi đã thu dữ liệu người thật không sửa được thiết kế thí nghiệm.

Leader phá được thế bế tắc về đánh đổi sản phẩm. Leader **không** phá được bằng cách: miễn consent · chấp nhận sai tiền · đổi ngưỡng sau khi đã thấy kết quả. Đổi protocol thì tăng `protocol_version` và **không gộp dữ liệu cũ**.

## 5. Tài liệu

```
docs/protocol/              giao thức thực địa — có version, snapshot bất biến
docs/decisions/             ADR — mọi thay đổi protocol/gate/phạm vi
docs/team/                  điều lệ + backlog + hàng đợi việc còn mở
docs/claude/<YYYY-MM-DD>/   nhật ký và ghi chép dài của việc đang làm
docs/archive/claude/        nhật ký cũ của lane Claude — đọc được, KHÔNG sửa, KHÔNG di chuyển
docs/archive/codex/         nhật ký cũ của lane Codex — như trên
docs/superpowers/specs/     spec sản phẩm — ĐÓNG BĂNG cho tới sau gate
```

Mỗi `protocol_version` là **snapshot bất biến**. Không sửa `v1` tại chỗ; ADR cho phép tạo `v2` và dữ liệu mới trỏ tới `v2`. ADR phải được duyệt **trước** khi thay đổi có hiệu lực — không hợp thức hoá hậu nghiệm.

Ghi chép dài bắt buộc có: **commit SHA · protocol_version · cái gì còn mở · bằng chứng đã xem** — và `verdict` khi thật sự có reviewer.

`verdict` dùng đúng ba giá trị, không dùng câu tự do: **`APPROVE`** · **`REQUEST_CHANGES`** · **`REJECT`**. *(Suggestion 1 của Codex, 2026-08-26 — để tự động hoá review không phải suy diễn text.)*

`docs/archive/claude/` và `docs/archive/codex/` là **lịch sử đóng băng tại chỗ**: `.repo-guard-allowlist.json` ghim `sha256` theo đúng những đường dẫn đó, nên đổi tên hay di chuyển chúng làm `repo_guard tree HEAD` đỏ cả cây.

Nhật ký là nhật ký, **không phải nguồn quyết định**. Quyết định sống ở `docs/decisions/`.

### Nhịp ghi
- **Hằng ngày** — chỉ trong những ngày thực sự có build work.
- **Hằng tuần** — trong các tuần chạy thực địa với allocation giảm.
- **Theo sự kiện, ngay lập tức** — mỗi thay đổi protocol · mỗi sự cố · mỗi lần mở block tuyển · mỗi interim gate.

## 6. Dữ liệu người tham gia — tuyệt đối

**Không bao giờ** đưa vào Git: ảnh bill · số tài khoản · tên người tham gia · transcript thô · file export.

Dữ liệu thật nằm **ngoài repository và ngoài worktree**, có kiểm soát truy cập. Thư mục bị `.gitignore` là chưa đủ.

Kiểm tra tự động (W9a) là **lớp giảm thiểu, không thay thế quy tắc trên** — không scanner nào nhận ra mọi tên người Việt hay PII nằm trong ảnh nén.

Không ai trong team — leader, operator, engineer — **giữ hoặc chuyển tiền hộ**. Nghĩa vụ giữa participant phải là nghĩa vụ thật, tự họ chuyển trực tiếp cho nhau. Không có nghĩa vụ thật thì rơi vào bẫy "tạo khoản nợ giả" mà mục 13.6 của spec cấm.
