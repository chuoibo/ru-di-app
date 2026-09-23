# Bàn giao 23-09: một engine AI, «luôn bật», chất lượng trả lời, dock Nếp

Người viết: phiên `ai-engine-unification`. Viết để một agent khác làm tiếp mà không
phải đọc lại hội thoại. Mọi SHA dưới đây đã có trên `origin`. PR của chính tài liệu này: #643.
Câu chuyện đầy đủ của phiên (giao gì, làm gì, vì sao) ở `lich-su-phien-ai-engine-unification.md` cùng thư mục.

## 1. Leader đã quyết gì (đừng hỏi lại)

- **Một đường gọi AI duy nhất** cho cả app. Người gọi (client) trao ngữ cảnh (transcript đã giải
  mã). Máy chủ chỉ đắp thêm thứ nó vốn sở hữu: roster, gu, ngân sách, catalogue. Máy chủ không
  bao giờ đọc `messages.body`. Chi tiết ở ADR-0034 (nhánh engine).
- **Xoá v1 tận gốc, kể cả Python.** `/chia-bill` gộp vào engine. Làm luôn **não chữ cho Nếp**.
- **Go là mặc định cho mọi xử lý backend.** Python chỉ giữ bước gọi model.
- **«Luôn bật»**: engine AI và feed thay đổi realtime phải bật mặc định. Nguyên văn leader:
  «nó phải luôn bật và AI luôn hoạt động tốt và chạy tốt … cả 2 đều phải tốt».
- **Dock Nếp**: phải đẹp, có câu chuyện, hài hoà với ngôn ngữ hình đã có, và không bao giờ
  che nội dung. Frontend đi qua impeccable-pipeline, không tự chấm.

## 2. Các nhánh và PR, theo thứ tự nên merge

| # | Nhánh (PR) | Base | Nội dung | Trạng thái |
|---|---|---|---|---|
| 1 | `claude/p0-w31-chat-tai-tranh-khoa` (#644) | main | Sửa script phân tích khoá, đo lại tải, sửa dòng tiến độ cũ | Xong. Chỉ có docs và script |
| 2 | `claude/p0-w30-nep-dock-trong-le` (#646) | main | Dock Nếp thiết kế lại | Code xong. **Chưa qua reviewer vòng 2, chưa đo native** (mục 4) |
| 3 | `claude/p0-w30-mot-engine-loi-goi-ai` (#647) | main | ADR-0034, lược đồ v3, gói bối cảnh, khối «Mình đang thấy», promptsafety, cổng không đọc chat | Xong. **Đi sau main 18 commit, cần merge main** |
| 4 | `claude/p0-w30-ai-luon-bat` (#648) | #3 | Cờ mặc định bật ở prod, `core migrate-chat`, service compose `migrate-chat` | Xong, trừ một lượt tầng Postgres chưa chạy lại (mục 5) |
| 5 | `claude/p0-w30-ai-chat-luong` (#649) | #3 | Gu, ngân sách, roster như v1; bộ corpus chấm chất lượng | Xong. **Còn một quyết định leader** (mục 6) |

Hai nhánh #4 và #5 đều rẽ từ #3 và không đụng cùng file. Merge #3 trước, rồi đổi base của #4
và #5 sang main.

**Không được xoá v1 trước khi #4 vào main.** Hiện ở cấu hình mặc định, route `ai-invocations`
và `chat-capabilities` chưa hề tồn tại. Nếu xoá v1 lúc này, production mất sạch đường AI, và
không cổng nào bắt được lỗi đó.

## 3. Việc CHƯA làm trong kế hoạch

Kế hoạch đầy đủ nằm trong ADR-0034 và commit message của nhánh #3. Các phần còn lại:

1. **Xoá v1.** Làm một commit gồm cả Go, Python và manifest.
   - Giữ `/vote`, vì route này đang sống.
   - Giữ `GroundCard`, package `chatintent`/`companion` và oracle parity.
   - Chuyển bất biến «từ chối bằng tiếng Việt» sang chỗ mới TRƯỚC khi xoá
     `apps/mobile/tests/cau-chu-im-lang.test.mjs`.
   - Thứ tự: chỉ làm sau khi #4 đã vào main (xem mục 2).
2. **Gộp `/chia-bill` thành `command=chia_bill` của engine.**
   - Kết quả bọc thành thẻ `kind:"text"`, số liệu có cấu trúc để ở cột `result`.
   - Không thêm `kind` mới: `GroundCard` bị ghim parity với Python.
3. **Não chữ cho Nếp.**
   - Route `/me/nep/ai-invocations` với `scope='me'`. Lược đồ v3 đã có sẵn index
     `chat_ai_me_logical` cho phạm vi này.
   - Kết quả trả kín về người gọi, không đăng vào phòng nào.
   - Action brain mới `nep-reply` trong Python: phần Python DUY NHẤT của cả đợt.
4. Tuỳ chọn, tách riêng được: chuyển `/me/nep/media` sang Go.

## 4. Dock Nếp: đang dở ở đâu

Thiết kế (xem doc comment đầu `apps/mobile/src/rudi/nep/NepDock.tsx`):
- **Mặc định Nếp là tờ giấy cài trong lề phải.** Tờ nhô 10dp. Khi có việc, thêm 4dp của một tờ
  `accentSoft` nằm sau, tổng 14dp, vẫn nhỏ hơn lề trang 16dp.
- Góc trên bên trái gấp lại theo ngữ pháp của ToGiay. Không dùng san hô, không có bóng.
- Vùng chạm dừng đúng ở lề (`slopTrai`).
- Chạm vào tờ thì Nếp ra ngoài. Chạm lần nữa mới mở bảng.
- Nếp đang cài không bao giờ tự hé dòng.
- `ui/Sheet`, khay Tờ hẹn, màn story và bản đồ hành trình khiến Nếp nhường chỗ
  (`useNhuongChoNep`).

Đã đo, trên bản web 390dp, sáng và tối, trước khi máy khởi động lại:
- Lúc nghỉ, 0 chữ bị che: 0/153 ở tab đầu, 0/180 trong hội thoại.
- Khi khay mở, 0/373 chữ bị che, và tờ «có việc» biến mất.
- Test: node 66/66 (các bộ Nếp và cổng env); npm test 976/976 ở commit đầu; tsc sạch.

Chưa làm:
- **Finish reviewer vòng 2 (impeccable) chưa chạy.** Vòng 1 chấm `fix` với 5 lỗi. Cả 5 đã
  được sửa trong hai commit của nhánh. Phải chụp lại rồi gửi reviewer context mới kèm ảnh.
- **Ảnh chụp đã mất** vì máy khởi động lại. Script chụp tuần tự nằm NGOÀI repo:
  `~/.cache/rudi-bang-chung/chup-dock.sh`. Nó dựng stack, dựng ba bản web (`--clear`), chụp,
  rồi dọn; bằng chứng được chép sang `~/.cache/rudi-bang-chung/dock-HHMM/`. Công cụ đo nằm
  trong repo: `apps/mobile/tools/xem-dock-nep.mjs`.
- **Dòng hé chưa chụp được lần nào.** Núm QA `EXPO_PUBLIC_QA_NEP_VIEC=hoi` đã được thêm độ trễ
  1200ms nhưng chưa được thử.
- **Ca canary** (khi Nếp đã ra, phép đo PHẢI thấy chữ bị che) chưa chạy lần nào.
- **Native chưa đo**:
  - Flow 25 không kèm `--tat-nep`.
  - Flow 12 (`.maestro/12-nep-dock.yaml` đã viết lại theo thứ tự mới).
  - Chạm và kéo dọc tờ cài trên máy dùng điều hướng cử chỉ: không được nổ Back. Phiên
    `chat-backend-go-migration` đo được dải cử chỉ Back rộng 29.7dp ở mỗi mép, và app không
    khai vùng loại trừ.
  - Phiên đó cũng bàn giao giữa chừng, không kịp đo. Phần đã chuẩn bị sẵn:
    - Worktree `~/wt-do-dock` ở `656c1235` (detached), CHƯA `npm ci`.
    - Máy ảo `rudi` và adb server đã CHẾT khi WSL khởi động lại lần thứ ba lúc 21:41. Phải
      dựng lại máy ảo; adb dùng cổng 5038.
    - Ba phép đo được ghi nguyên văn kèm kỳ vọng ở mục 1a của
      `docs/claude/2026-09-23/handoff-do-nep-dock-va-flow-30.md` (PR #642).
  - Bẫy khi đo: `adb devices` liệt kê CÙNG một máy hai dòng (`127.0.0.1:5555` và
    `emulator-5554`). Harness tự lấy dòng đầu, nên phải truyền `--serial emulator-5554`.
    `--serial` có tới được Maestro.
  - Đừng kiểm máy ảo bằng `adb devices`: adb tự dựng một server mới rồi im lặng trả danh sách
    rỗng, trông y hệt «không có máy nào» lẫn «adb hỏng». Kiểm bằng
    `ps -eo args | grep qemu-system`.
- **Chưa đo bằng hình học**: badge của Outing (right:12), Memories (right:8), `leadSave` của
  HangDiaDiem. Đọc nguồn thì chúng nằm trong cột nội dung, nhưng chưa có số đo.
- DESIGN.md chưa cập nhật. Theo quy trình, chỉ chạy `impeccable-documenter` sau khi reviewer
  chấm `ship`.

## 5. «Luôn bật» (#4)

Luật cờ nằm trong commit `bbf62d4b` (`resolveChatFeatures`, có test bảng):
- Không đặt + prod → bật.
- `0` → tắt, có log WARN.
- Không đặt + dev → tắt, có log WARN, KHÔNG từ chối khởi động. Lý do: ở dev, bearer đúc được
  bằng header, đã dựng lại được. Ràng buộc prod là bất biến bảo mật thật.

Hệ quả cho vận hành: host rollback bằng `MOBILE_FORCE_PYTHON=all` giờ phải đặt kèm
`MOBILE_CHAT_CHANGES_CANDIDATE=0`, nếu không core từ chối khởi động.

Chưa đo: tầng Postgres sau hai commit sửa test (`e9bf794f` chờ cổng công khai, `d985f732` lấy
hai cổng cùng lúc). Lệnh chạy:
`scripts/go_postgres_tier.sh -- ./internal/db/... ./internal/chatassist/... ./internal/chatlegacychange/... ./cmd/core/...`

## 6. Chất lượng trả lời (#5)

- Lượt thật trên `gemini-2.5-flash`: **9/16 ca qua mọi phép máy chấm**.
- Output model và bảng điểm chi tiết đã mất khi máy khởi động lại. Chạy lại bằng
  `services/api/tests/skills/tra_loi_trong_nhom.py` (cần khoá Gemini). `--cham-lai` chấm lại
  kết quả đã lưu mà không gọi model.
- `services/api/tests/live/test_companion_gemini_live.py`
  (`MOBILE_REQUIRE_GEMINI_TESTS=1`) chưa được báo kết quả.
- **Quyết định cho leader**: roster gửi model dùng BÚT DANH («Mình», «Bạn 1»…), không dùng tên
  hiển thị. Lý do: khối xem trước trên màn đang hứa «Tên tài khoản không đi kèm», và ADR-0034
  §2.5 cấm rút một lời hứa riêng tư đơn phương. Muốn đổi sang tên thật thì chỉ cần sửa một
  dòng trong `roster.go`, nhưng phải đổi câu hứa trên màn trước.
- Việc tiếp theo nên làm: đọc 7 ca trượt, sửa prompt hoặc grounding, rồi chạy lại để so.

## 7. Tải realtime (#1)

Bản sửa tranh khoá đã lên main (PR #633). Burst 300/s trên `62a591fa` đạt: p99 208ms, toàn
vẹn 9 triệu/9 triệu. Còn lại:
- Lượt 30 phút trên main hiện tại, trên máy yên.
- Soak 24 giờ.
- `Mark` vẫn giữ `FOR UPDATE` qua năm vòng mạng. Harness chưa đo đường này.

## 8. Bẫy môi trường gặp trong đợt này

- **WSL khởi động lại ba lần** trong ngày (11:22, 20:05, 21:41), khi chạy song song pytest
  đầy đủ, tầng Postgres của Go, máy ảo, Metro và nhập ảnh. Mỗi lần mất sạch `/tmp` và
  scratchpad, mọi stack và máy ảo đều tắt; cây git sống sót cả ba lần.
  Mọi thứ cần giữ phải nằm trong git. Chạy
  tuần tự, kiểm `free -g` trước, commit sớm, và giữ bằng chứng ngoài `/tmp`.
- `expo export` không kèm `--clear` giữ lại `EXPO_PUBLIC_API_URL` của lần dựng trước. Khi đó
  đăng nhập bắn vào `api.build-check.invalid`. Sau khi dựng, grep bundle để kiểm URL.
- Hai phiên từng nhận cùng một worktree là của mình. Worktree `/home/lakiet/wt-ai-engine`
  thuộc phiên kia, đừng commit ở đó. Cây `/home/lakiet/wt-chat-ui` thuộc phiên
  `chat-backend-go-migration`.
- Không `git add -A` ở gốc repo: `apps/mobile/node_modules` là thư mục thật, chưa được theo dõi.

## 9. Worktree

| Đường dẫn | Nhánh |
|---|---|
| `/home/lakiet/wt-nep-dock` | `claude/p0-w30-nep-dock-trong-le` (có `node_modules` thật) |
| `/home/lakiet/wt-ai-luon-bat` | `claude/p0-w30-ai-luon-bat` |
| `/home/lakiet/wt-ai-chat-luong` | `claude/p0-w30-ai-chat-luong` |
| `/home/lakiet/wt-chat-tai` | `claude/p0-w31-chat-tai-tranh-khoa` |
| `/home/lakiet/wt-ban-giao` | nhánh của chính tài liệu này |

Không còn stack docker hay tiến trình nền nào của phiên này đang chạy.
