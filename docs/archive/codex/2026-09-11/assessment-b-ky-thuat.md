# Assessment B độc lập — kỹ thuật PR #597–#600, ngày 11/09/2026

**REQUEST_CHANGES riêng cho việc đóng R2/cổng motion.** Hai lỗi điều khiển shell còn tái hiện: cửa sổ chưa reset vẫn được ghi hợp lệ; scale gốc không đọc được vẫn bị ghi đè. R1 nối Reduce Motion đúng ở nguồn; các regression gate R3/F45/R4/R5 kiểm lại đều qua trong phạm vi bên dưới. Không dùng thiếu iOS, release, máy thật hoặc TalkBack để dựng blocker cho bốn PR này.

Tree kiểm: `5061b610735df34d189eab61af997ac4783fec14`; diff `0c1a4169..5061b610`. Đọc AGENTS.md, trả lời của Claude ngày 11/09, source/test diff và tài liệu kỹ thuật B cũ. **Không đọc Assessment A hoặc ảnh/nhận xét A. Không chạy ADB, Maestro, Metro hoặc emulator thật.** Mọi adb/maestro trong canary là executable giả trong thư mục tạm; PATH của canary độc lập chỉ gồm fake bin + `/usr/bin:/bin`. Không sửa sản phẩm, dependency hoặc backend. Tất cả log của B ở [technical-evidence](technical-evidence/).

## Finding có bằng chứng tái hiện

### B11 — P1 cho cổng R2: reset thất bại hoặc dump thiếu số liệu vẫn được coi là phép đo hợp lệ

Vị trí: [do-motion.sh:87](../../claude/2026-09-10/motion/do-motion.sh#L87), `:92`, `:98–103`; parser `:68–71`.

- Canary `reset-failed` cho duy nhất `adb shell dumpsys gfxinfo … reset` exit 1; pid ổn định, Maestro exit 0 và dump có số. Runner vẫn **exit 0, cả bốn hàng hợp lệ**. Hậu quả: khung cũ/warm-up có thể nằm trong số liệu mang tên thao tác mới, đúng kiểu lỗi cửa sổ mà R2 cần đóng. Exit của dump cũng chưa tham gia điều kiện hợp lệ.
- Canary `dump-malformed` chỉ trả `Total frames rendered: 12`, không histogram, percentile, janky hoặc slow counters. Runner vẫn **exit 0**, in bốn hàng có `khung>150ms = 0`, các percentile rỗng và counters `?`. Giá trị 0 ở đuôi chậm lúc này là do parser không có histogram, không phải quan sát không có khung chậm.
- Canary pid rỗng đỏ đúng. Pid chuỗi `pid-not-known` ổn định lại xanh vì chỉ kiểm không rỗng và bằng nhau. Đây là nhánh kiểm parser yếu bổ sung, không phải lời khẳng định adb bình thường trả dạng ấy.

Bằng chứng: [reset-failed.log](technical-evidence/reset-failed.log), [reset-failed-trace.json](technical-evidence/reset-failed-trace.json), [dump-malformed.log](technical-evidence/dump-malformed.log), [pid-malformed.log](technical-evidence/pid-malformed.log), [canary-summary.json](technical-evidence/canary-summary.json). Script tái hiện: `python3 docs/archive/codex/2026-09-11/technical-evidence/independent-canary.py`; không chạm thiết bị.

**Điều kiện gỡ:** bắt exit của reset/dump; reset lỗi thì không chạy cửa sổ mang nhãn hợp lệ. Kiểm pid theo định dạng đã chọn và ràng buộc dump đúng tiến trình. Các metric được công bố phải parse được; histogram vắng/không hợp lệ phải là invalid, không tự thành 0. Canary reset đỏ, dump chỉ một dòng, pid rỗng/sai định dạng phải đỏ; đối chứng dump thật vẫn xanh. Không cần đo release để chứng minh bản sửa shell này.

### B12 — P2: lỗi đọc/ghi scale chưa được dừng trước thao tác và lỗi restore bị nuốt

Vị trí: [do-motion.sh:43](../../claude/2026-09-10/motion/do-motion.sh#L43), `:45`, `:49–52`, `:58–59`, `:82`.

Canary bắt đầu với ba scale **0.5 / 1.5 / 2**, giả lỗi đọc riêng `animator_duration_scale` ban đầu. Script đặt `that_bai=1` nhưng **vẫn ghi cả ba scale về 0**, chạy warm-up và bốn Maestro. Cuối lượt exit 1, song `animator_duration_scale` còn **0**, vì trap bỏ qua giá trị gốc không hợp lệ. Việc tránh `put ""` chưa giải quyết lỗi: chính `put 0` sau khi không đọc được gốc tạo cấu hình không thể phục hồi.

Hai nhánh khác: `put-failed` từ chối mọi ghi 0, vẫn **exit 0** và nhãn `reduce` dù đọc lại là 0.5 / 1.5 / 2; `restore-failed` từ chối ghi về gốc, runner **exit 0** và để cả ba scale ở 0. Script lưu số đọc lại nhưng không đối chiếu hoặc truyền lỗi ra exit.

Bằng chứng: [original-empty-trace.json](technical-evidence/original-empty-trace.json), [original-empty.log](technical-evidence/original-empty.log), [put-failed.log](technical-evidence/put-failed.log), [restore-failed-trace.json](technical-evidence/restore-failed-trace.json), bảng 13 canary. Dữ liệu hoàn toàn tổng hợp.

**Điều kiện gỡ:** nếu bất kỳ gốc nào chưa đọc/parse chắc chắn thì exit trước mọi `put` và Maestro; kiểm thành công ghi và đọc lại đúng scale yêu cầu trước đo. Restore phải đối chiếu giá trị, báo lỗi và làm lượt thất bại khi không hoàn tất; không hứa phục hồi trong trường hợp adb không hoạt động. Thêm ba canary trên, giữ canary INT và giá trị gốc khác 1.

## Những nhánh thực sự đã sửa đúng

- Warm-up chỉ chạy một lần trước cửa sổ; m1–m4 fixture/live chỉ thao tác, không nhúng `_vao-*`. Helper live có nhánh nhập OTP cho cả hai màn vào.
- Canary tác giả chạy lại: Maestro 42, pid thay đổi, frames 0 đều exit 1; đối chứng dump thật exit 0. Bucket ≥150 tính được 12 = 7 + 4 + 1 trên dump m1 cũ. [author-canary.log](technical-evidence/author-canary.log).
- Canary độc lập: Maestro lỗi, pid đổi/rỗng, frames 0, dump rỗng đều đỏ. Với gốc 0.5 / 1.5 / 2 và adb hoạt động, kết thúc bình thường trả đúng cả ba. **INT giữa lượt exit 130, chỉ hai lần gọi Maestro (warm-up + m1), không đo tiếp m2–m4; trả đúng gốc.** [interrupt-trace.json](technical-evidence/interrupt-trace.json).
- Đếm độc lập tám dump v2 đã lưu: tổng histogram khớp frames ở cả tám; bucket ≥150 bằng 0. Frames thường 271/907/406/273, reduce 45/704/63/60. Đây là kiểm tính nhất quán artifact tác giả, không phải đo native mới và không bị các canary biến thành số liệu sai. [histogram-v2-check.json](technical-evidence/histogram-v2-check.json).

## R1 và regression gates R3/F45/R4/R5

Biên dịch lại `tsc -p tsconfig.test.json`, chạy `node tools/fixup-esm.mjs`: exit 0. `tsc --noEmit`: exit 0. Chạy trực tiếp tám file test trên đầu ra biên dịch mới: **75 ca qua, 0 lỗi, 0 bỏ qua**. Không dùng số `8` của test runner tổng làm số ca: runner tổng chỉ đếm file. [direct-test-summary.json](technical-evidence/direct-test-summary.json) và các `test-*.log` lưu số bên trong từng file.

| Mục | Kiểm lại trên tree hiện tại | Điều chưa được phép suy ra |
|---|---|---|
| R1 | `stackAnimation` đưa right/bottom/fade về `none`; `_layout` đọc bit sống cho screenOptions và sáu Screen override; `useMotion` dùng Always/Never cho timing/spring; ExploreLive dùng cùng bit. Motion 6/6. | Test thuần chưa kiểm subscription hook hoặc chuyển cảnh thực. B không đo native; kết luận native do nhánh điều khiển thiết bị ghi riêng. |
| R3 | `dauCon` bỏ stamp khi có lý do ở lead/row/compare; `chonLyDo` dùng cả từ; ly-do 4/4, khong-mo-coi 2/2, kham-pha 28/28. Chạy lại cổng XML trên bốn XML tác giả 1.0/2.0 × sáng/tối: 4 xanh; XML Codex 05-explore cũ: đỏ đúng. | XML cũ/tác giả không phải ảnh mới của B. Live reason dài và TalkBack không được chứng minh bởi test chuỗi. |
| F45/R4 | Art 15/15, sticker 6/6: bánh xe/ô bầu dục/đuôi phía ngoài và hai cảnh không cung coral được pin cơ chế. Source bỏ `vongHo` khỏi cảnh bộ lọc. | Cơ chế có mặt không chứng minh con người hiểu nghĩa hình. Không ký verdict mỹ thuật hoặc đọc mù bằng tên hàm. |
| R5 | Cổng không Card 3/3; settings domain/API helper 11/11. Source năm màn Cài đặt dùng NhomHang; thẻ AI fixture có nút mở lý do, state expanded và target 48; thẻ AI live đổi typography title. | Các test này không phải live thao tác lưu setting, xóa tài khoản hoặc kiểm focus/đọc TalkBack. Hai màn ngoài phạm vi còn Card là nợ tác giả đã nêu. |

Không phát hiện regression nguồn sản phẩm cần chặn thêm trong diff đã kiểm. Chưa chạy toàn `npm test`/build export hoặc toàn backend vì không cần cho kết luận shell và các regression gate được giao.

## Detector và baseline năm test

Detector chạy **đúng một lần** theo yêu cầu: `node .claude/skills/impeccable/scripts/detect.mjs --json apps/mobile/src`; **exit 0, stdout `[]`, stderr rỗng, count 0, không có tên rule/finding**. [detector-meta.json](technical-evidence/detector-meta.json). Đã đọc Impeccable local và context một lần; native audit playbook ghi detector không áp dụng cho native, nên đây chỉ là kết quả quét mã yêu cầu, không điểm native và không bằng chứng sạch về hình/animation. Không có browser overlay.

Local có `main-api-domain-failed.txt` và `pr593-api-domain-failed.txt` của tác giả: hai danh sách giống hệt nhau, đúng năm ca `test_android_emulator_may_ket.py`. Diff PR #597–#600 không đổi file test này hoặc `scripts/android_emulator.sh`. [baseline-five-check.json](technical-evidence/baseline-five-check.json) chép tên ca và SHA256 của danh sách. **Chỉ xác minh được danh sách local và không đổi source**; chưa xác thực commit của lần chạy gốc hoặc nguyên nhân năm lỗi từ full trace, không gọi đây là năm lỗi vừa tái hiện. Không chạy suite đó vì B không được điều khiển emulator thật và không cần mở rộng phạm vi để phân loại diff này.

## Phán quyết phạm vi B

R1 có wiring nguồn đúng; R3/F45/R4/R5 có gate kỹ thuật qua. **R2 chưa đủ điều kiện gọi fail-closed hoặc đóng hoàn toàn**, do B11/B12 tái hiện ngay trên script HEAD bằng dữ liệu tổng hợp. Sửa hai lỗi và chạy lại canary tương ứng là đường gỡ cụ thể; không buộc làm các hạng mục ngoài phạm vi. Đánh giá nghĩa hình, mỹ thuật và native hiện tại được giữ độc lập ở nhánh A/nhánh chính.
